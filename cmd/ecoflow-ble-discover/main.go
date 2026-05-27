package main

import (
	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"tinygo.org/x/bluetooth"
)

type outputFormat string

const (
	outputFormatText outputFormat = "text"
	outputFormatJSON outputFormat = "json"
)

type discoveryConfig struct {
	Duration     time.Duration
	Format       outputFormat
	IncludeAll   bool
	Redact       bool
	MinRSSI      int
	NamePrefix   string
	ScanOnly     bool
	Selection    string
	ProbeTimeout time.Duration
}

type ecoFlowDeviceInfo struct {
	Matched      bool   `json:"matched"`
	Prefix       string `json:"prefix,omitempty"`
	Model        string `json:"model,omitempty"`
	PacketFamily string `json:"packet_family,omitempty"`
}

type discoveredBLEDevice struct {
	Address          string            `json:"address"`
	RSSI             int16             `json:"rssi"`
	LocalName        string            `json:"local_name,omitempty"`
	ServiceUUIDs     []string          `json:"service_uuids,omitempty"`
	ManufacturerData map[string]string `json:"manufacturer_data,omitempty"`
	Info             ecoFlowDeviceInfo `json:"ecoflow"`
}

type deviceProbe struct {
	Capabilities probeCapabilities `json:"capabilities"`
	Metrics      []probeMetric     `json:"metrics,omitempty"`
	Readings     []probeReading    `json:"readings,omitempty"`
}

type probeCapabilities struct {
	ServiceCount        int            `json:"service_count"`
	CharacteristicCount int            `json:"characteristic_count"`
	MTUs                []uint16       `json:"mtus,omitempty"`
	Services            []probeService `json:"services,omitempty"`
}

type probeService struct {
	UUID            string   `json:"uuid"`
	Characteristics []string `json:"characteristics,omitempty"`
}

type probeMetric struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Unit   string `json:"unit,omitempty"`
	Source string `json:"source"`
}

type probeReading struct {
	ServiceUUID        string `json:"service_uuid"`
	CharacteristicUUID string `json:"characteristic_uuid"`
	Label              string `json:"label"`
	Bytes              int    `json:"bytes"`
	ValueHex           string `json:"value_hex,omitempty"`
	Text               string `json:"text,omitempty"`
}

type discoveryScanner interface {
	Scan(stop <-chan struct{}, emit func(discoveredBLEDevice)) error
}

type deviceProber interface {
	Probe(ctx context.Context, device discoveredBLEDevice) (deviceProbe, error)
}

type bluetoothAdapter interface {
	Enable() error
}

type bluetoothScanner struct{}

type bluetoothProber struct{}

type blePrefixHint struct {
	Prefix       string
	Model        string
	PacketFamily string
}

// Prefix hints are intentionally lightweight discovery metadata, not protocol
// support guarantees. They mirror the currently useful EcoFlow BLE families
// seen in community integrations while leaving unknown EF-* names discoverable.
var ecoflowBLEPrefixHints = []blePrefixHint{
	{Prefix: "D3UP", Model: "EcoFlow DELTA 3 Ultra Plus", PacketFamily: "v3"},
	{Prefix: "D3N1", Model: "EcoFlow DELTA 3 Classic", PacketFamily: "v3"},
	{Prefix: "D3M1", Model: "EcoFlow DELTA 3 Max", PacketFamily: "v3"},
	{Prefix: "D3E1", Model: "EcoFlow DELTA 3 Max Plus", PacketFamily: "v3"},
	{Prefix: "D3U1", Model: "EcoFlow DELTA 3 Ultra", PacketFamily: "v3"},
	{Prefix: "PR11", Model: "EcoFlow DELTA 3 1000 Air", PacketFamily: "v3"},
	{Prefix: "PR12", Model: "EcoFlow DELTA 3 1000 Air (10ms UPS)", PacketFamily: "v3"},
	{Prefix: "PR1W", Model: "EcoFlow DELTA 3 1000 Air", PacketFamily: "v3"},
	{Prefix: "PR21", Model: "EcoFlow DELTA 3 2000 Air", PacketFamily: "v3"},
	{Prefix: "PR", Model: "EcoFlow DELTA 3 Air", PacketFamily: "v3"},
	{Prefix: "P231", Model: "EcoFlow DELTA 3", PacketFamily: "v3"},
	{Prefix: "D361", Model: "EcoFlow DELTA 3 (1500)", PacketFamily: "v3"},
	{Prefix: "P351", Model: "EcoFlow DELTA 3 Plus", PacketFamily: "v3"},
	{Prefix: "MR51", Model: "EcoFlow DELTA Pro 3", PacketFamily: "v3"},
	{Prefix: "MR53", Model: "EcoFlow DELTA Pro 3E", PacketFamily: "v3"},
	{Prefix: "Y711", Model: "EcoFlow DELTA Pro Ultra", PacketFamily: "v3"},
	{Prefix: "R631", Model: "EcoFlow RIVER 3 Plus", PacketFamily: "v3"},
	{Prefix: "R634", Model: "EcoFlow RIVER 3 Plus (270Wh)", PacketFamily: "v3"},
	{Prefix: "R635", Model: "EcoFlow RIVER 3 Plus (Wireless)", PacketFamily: "v3"},
	{Prefix: "R3PG", Model: "EcoFlow RIVER 3 Plus (270Wh)", PacketFamily: "v3"},
	{Prefix: "R3P", Model: "EcoFlow RIVER 3 Plus", PacketFamily: "v3"},
	{Prefix: "R651", Model: "EcoFlow RIVER 3 (245Wh)", PacketFamily: "v3"},
	{Prefix: "R653", Model: "EcoFlow RIVER 3 (230Wh)", PacketFamily: "v3"},
	{Prefix: "R654", Model: "EcoFlow RIVER 3 UPS (230Wh)", PacketFamily: "v3"},
	{Prefix: "R655", Model: "EcoFlow RIVER 3 UPS (245Wh)", PacketFamily: "v3"},
	{Prefix: "R3", Model: "EcoFlow RIVER 3", PacketFamily: "v3"},
	{Prefix: "YJ", Model: "EcoFlow DELTA Pro Ultra", PacketFamily: "v3"},
	{Prefix: "R331", Model: "EcoFlow DELTA 2", PacketFamily: "v2"},
	{Prefix: "R335", Model: "EcoFlow DELTA 2", PacketFamily: "v2"},
	{Prefix: "R351", Model: "EcoFlow DELTA 2 Max", PacketFamily: "v2"},
	{Prefix: "R354", Model: "EcoFlow DELTA 2 Max", PacketFamily: "v2"},
	{Prefix: "P341", Model: "EcoFlow DELTA 2 Max S", PacketFamily: "v2"},
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := runDiscovery(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr, bluetoothScanner{}, bluetoothProber{}); err != nil {
		fmt.Fprintf(os.Stderr, "ecoflow-ble-discover: %v\n", err)
		os.Exit(1)
	}
}

func runDiscovery(
	parent context.Context,
	args []string,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
	scanner discoveryScanner,
	prober deviceProber,
) error {
	cfg, err := parseDiscoveryConfig(args, stderr)
	if err != nil {
		return err
	}
	if err := validateDiscoveryConfig(cfg); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	if cfg.Duration > 0 {
		var timeoutCancel context.CancelFunc
		ctx, timeoutCancel = context.WithTimeout(ctx, cfg.Duration)
		defer timeoutCancel()
	}

	stopScan := make(chan struct{})
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			close(stopScan)
		case <-done:
		}
	}()
	defer close(done)

	startedAt := time.Now()
	seen := make(map[string]discoveredBLEDevice)
	var order []string
	var writeErrMu sync.Mutex
	var writeErr error
	recordWriteErr := func(err error) {
		if err == nil {
			return
		}
		writeErrMu.Lock()
		defer writeErrMu.Unlock()
		if writeErr == nil {
			writeErr = err
			cancel()
		}
	}
	currentWriteErr := func() error {
		writeErrMu.Lock()
		defer writeErrMu.Unlock()
		return writeErr
	}
	emit := func(device discoveredBLEDevice) {
		if currentWriteErr() != nil {
			return
		}
		device.Info = inferEcoFlowDevice(device.LocalName)
		if !shouldIncludeAdvertisement(device, cfg) {
			return
		}
		key := discoveryKey(device)
		if existing, ok := seen[key]; ok {
			if device.RSSI > existing.RSSI {
				seen[key] = device
			}
			return
		}
		seen[key] = device
		order = append(order, key)
		if !shouldPrintDiscoveryDevice(cfg) {
			return
		}
		if cfg.Format == outputFormatJSON {
			if _, err := fmt.Fprintln(stdout, formatDiscoveryJSON(device, cfg.Redact)); err != nil {
				recordWriteErr(fmt.Errorf("write discovery device: %w", err))
			}
			return
		}
		if _, err := fmt.Fprintln(stdout, formatDiscoveryText(device, cfg.Redact)); err != nil {
			recordWriteErr(fmt.Errorf("write discovery device: %w", err))
		}
	}

	if err := scanner.Scan(stopScan, emit); err != nil {
		return err
	}
	if err := currentWriteErr(); err != nil {
		return err
	}

	summary := discoverySummary(seen)
	devices := orderedDiscoveredDevices(seen, order)
	elapsed := time.Since(startedAt).Round(time.Millisecond)
	if cfg.Format == outputFormatJSON {
		if _, err := fmt.Fprintf(stdout, `{"type":"summary","seen":%d,"ecoflow":%d,"elapsed":%q}`+"\n", summary.Seen, summary.EcoFlow, elapsed.String()); err != nil {
			return fmt.Errorf("write discovery summary: %w", err)
		}
		if !cfg.ScanOnly && cfg.Selection != "" {
			selected, ok, err := selectDiscoveredDevice(devices, cfg.Selection)
			if err != nil {
				return err
			}
			if ok {
				probe, err := probeSelectedDevice(parent, cfg, selected, prober)
				if err != nil {
					return err
				}
				if _, err := fmt.Fprintln(stdout, formatProbeJSON(selected, probe, cfg.Redact)); err != nil {
					return fmt.Errorf("write probe result: %w", err)
				}
			}
		}
		return nil
	}
	if _, err := fmt.Fprintf(stdout, "summary seen=%d ecoflow=%d elapsed=%s\n", summary.Seen, summary.EcoFlow, elapsed); err != nil {
		return fmt.Errorf("write discovery summary: %w", err)
	}
	if cfg.ScanOnly {
		return nil
	}
	if len(devices) == 0 {
		return nil
	}
	if err := printDeviceSelection(stdout, devices, cfg.Redact); err != nil {
		return err
	}
	selected, ok, err := promptForDeviceSelection(stdin, stdout, devices, cfg)
	if err != nil {
		return err
	}
	if !ok {
		if _, err := fmt.Fprintln(stdout, "probe skipped"); err != nil {
			return fmt.Errorf("write probe skipped: %w", err)
		}
		return nil
	}
	probe, err := probeSelectedDevice(parent, cfg, selected, prober)
	if err != nil {
		return err
	}
	if err := printProbeText(stdout, selected, probe, cfg.Redact); err != nil {
		return err
	}
	return nil
}

func parseDiscoveryConfig(args []string, stderr io.Writer) (discoveryConfig, error) {
	cfg := discoveryConfig{
		Duration:     5 * time.Second,
		Format:       outputFormatText,
		Redact:       true,
		MinRSSI:      -100,
		NamePrefix:   "EF-",
		ProbeTimeout: 10 * time.Second,
	}
	fs := flag.NewFlagSet("ecoflow-ble-discover", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.DurationVar(&cfg.Duration, "duration", cfg.Duration, "BLE scan duration; use 0 to scan until interrupted")
	fs.Var((*outputFormatValue)(&cfg.Format), "format", "output format: text or json")
	fs.BoolVar(&cfg.IncludeAll, "all", cfg.IncludeAll, "include non-EcoFlow BLE advertisements")
	fs.BoolVar(&cfg.Redact, "redact", cfg.Redact, "redact BLE addresses and local names in output")
	fs.IntVar(&cfg.MinRSSI, "min-rssi", cfg.MinRSSI, "minimum RSSI to include; use 0 to disable RSSI filtering")
	fs.StringVar(&cfg.NamePrefix, "name-prefix", cfg.NamePrefix, "extra local-name prefix treated as an EcoFlow candidate")
	fs.BoolVar(&cfg.ScanOnly, "scan-only", cfg.ScanOnly, "scan and print advertisements without prompting for a BLE probe")
	fs.StringVar(&cfg.Selection, "select", cfg.Selection, "select a discovered device by menu number, BLE address, local name, or prefix without prompting")
	fs.DurationVar(&cfg.ProbeTimeout, "probe-timeout", cfg.ProbeTimeout, "maximum time to spend probing the selected device")
	if err := fs.Parse(args); err != nil {
		return discoveryConfig{}, err
	}
	return cfg, nil
}

func validateDiscoveryConfig(cfg discoveryConfig) error {
	if cfg.Duration < 0 {
		return errors.New("duration must be zero or positive")
	}
	if cfg.ProbeTimeout <= 0 {
		return errors.New("probe-timeout must be positive")
	}
	switch cfg.Format {
	case outputFormatText, outputFormatJSON:
		return nil
	default:
		return fmt.Errorf("format must be %q or %q", outputFormatText, outputFormatJSON)
	}
}

func shouldPrintDiscoveryDevice(cfg discoveryConfig) bool {
	return cfg.ScanOnly || cfg.Format == outputFormatJSON
}

func orderedDiscoveredDevices(seen map[string]discoveredBLEDevice, order []string) []discoveredBLEDevice {
	devices := make([]discoveredBLEDevice, 0, len(seen))
	for _, key := range order {
		device, ok := seen[key]
		if !ok {
			continue
		}
		devices = append(devices, device)
	}
	return devices
}

func printDeviceSelection(stdout io.Writer, devices []discoveredBLEDevice, redact bool) error {
	if _, err := fmt.Fprintln(stdout, "discovered devices:"); err != nil {
		return fmt.Errorf("write device selection header: %w", err)
	}
	for i, device := range devices {
		line := strings.TrimPrefix(formatDiscoveryText(device, redact), "device ")
		if _, err := fmt.Fprintf(stdout, "%d) %s\n", i+1, line); err != nil {
			return fmt.Errorf("write device selection option: %w", err)
		}
	}
	return nil
}

func promptForDeviceSelection(stdin io.Reader, stdout io.Writer, devices []discoveredBLEDevice, cfg discoveryConfig) (discoveredBLEDevice, bool, error) {
	if cfg.Selection != "" {
		return selectDiscoveredDevice(devices, cfg.Selection)
	}
	if stdin == nil {
		return discoveredBLEDevice{}, false, nil
	}
	if _, err := fmt.Fprintf(stdout, "select device [1-%d, empty to skip]: ", len(devices)); err != nil {
		return discoveredBLEDevice{}, false, fmt.Errorf("write device selection prompt: %w", err)
	}
	line, err := bufio.NewReader(stdin).ReadString('\n')
	if _, writeErr := fmt.Fprintln(stdout); writeErr != nil {
		return discoveredBLEDevice{}, false, fmt.Errorf("write device selection prompt newline: %w", writeErr)
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return discoveredBLEDevice{}, false, fmt.Errorf("read device selection: %w", err)
	}
	return selectDiscoveredDevice(devices, line)
}

func selectDiscoveredDevice(devices []discoveredBLEDevice, selector string) (discoveredBLEDevice, bool, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return discoveredBLEDevice{}, false, nil
	}
	if strings.EqualFold(selector, "first") {
		if len(devices) == 0 {
			return discoveredBLEDevice{}, false, nil
		}
		return devices[0], true, nil
	}
	if index, err := strconv.Atoi(selector); err == nil {
		if index < 1 || index > len(devices) {
			return discoveredBLEDevice{}, false, fmt.Errorf("selection %q out of range 1-%d", selector, len(devices))
		}
		return devices[index-1], true, nil
	}

	var matches []discoveredBLEDevice
	for _, device := range devices {
		info := device.Info
		if !info.Matched {
			info = inferEcoFlowDevice(device.LocalName)
		}
		if strings.EqualFold(selector, device.Address) ||
			strings.EqualFold(selector, device.LocalName) ||
			(info.Prefix != "" && strings.EqualFold(selector, info.Prefix)) {
			matches = append(matches, device)
		}
	}
	switch len(matches) {
	case 0:
		return discoveredBLEDevice{}, false, fmt.Errorf("selection %q did not match a discovered device", selector)
	case 1:
		return matches[0], true, nil
	default:
		return discoveredBLEDevice{}, false, fmt.Errorf("selection %q matched %d devices; use the menu number or BLE address", selector, len(matches))
	}
}

func probeSelectedDevice(parent context.Context, cfg discoveryConfig, device discoveredBLEDevice, prober deviceProber) (deviceProbe, error) {
	if prober == nil {
		return deviceProbe{}, errors.New("probe unavailable")
	}
	ctx, cancel := context.WithTimeout(parent, cfg.ProbeTimeout)
	defer cancel()

	probe, err := prober.Probe(ctx, device)
	if err != nil {
		return deviceProbe{}, fmt.Errorf("probe selected device: %w", err)
	}
	probe.Metrics = append(advertisementProbeMetrics(device), probe.Metrics...)
	return probe, nil
}

func advertisementProbeMetrics(device discoveredBLEDevice) []probeMetric {
	metrics := []probeMetric{
		{Name: "rssi_dbm", Value: strconv.Itoa(int(device.RSSI)), Unit: "dBm", Source: "advertisement"},
		{Name: "advertised_services", Value: strconv.Itoa(len(device.ServiceUUIDs)), Source: "advertisement"},
		{Name: "manufacturer_data_blocks", Value: strconv.Itoa(len(device.ManufacturerData)), Source: "advertisement"},
	}
	if device.Info.Prefix != "" {
		metrics = append(metrics, probeMetric{Name: "prefix", Value: device.Info.Prefix, Source: "advertisement"})
	}
	if device.Info.PacketFamily != "" {
		metrics = append(metrics, probeMetric{Name: "packet_family", Value: device.Info.PacketFamily, Source: "inference"})
	}
	return metrics
}

func printProbeText(stdout io.Writer, device discoveredBLEDevice, probe deviceProbe, redact bool) error {
	display := displayDevice(device, redact)
	parts := []string{
		"probing",
		"address=" + display.Address,
		fmt.Sprintf("name=%q", display.LocalName),
	}
	if display.Info.Model != "" {
		parts = append(parts, fmt.Sprintf("model=%q", display.Info.Model))
	}
	if _, err := fmt.Fprintln(stdout, strings.Join(parts, " ")); err != nil {
		return fmt.Errorf("write probe header: %w", err)
	}

	capabilityParts := []string{
		"capabilities",
		fmt.Sprintf("services=%d", probe.Capabilities.ServiceCount),
		fmt.Sprintf("characteristics=%d", probe.Capabilities.CharacteristicCount),
	}
	if len(probe.Capabilities.MTUs) > 0 {
		capabilityParts = append(capabilityParts, "mtus="+joinUint16s(probe.Capabilities.MTUs))
	}
	if _, err := fmt.Fprintln(stdout, strings.Join(capabilityParts, " ")); err != nil {
		return fmt.Errorf("write probe capabilities: %w", err)
	}
	for _, service := range probe.Capabilities.Services {
		if _, err := fmt.Fprintf(stdout, "service uuid=%s characteristics=%d\n", service.UUID, len(service.Characteristics)); err != nil {
			return fmt.Errorf("write probe service: %w", err)
		}
	}
	for _, metric := range probe.Metrics {
		metricParts := []string{"metric", fmt.Sprintf("%s=%s", metric.Name, formatTokenValue(metric.Value))}
		if metric.Unit != "" {
			metricParts = append(metricParts, fmt.Sprintf("unit=%q", metric.Unit))
		}
		if metric.Source != "" {
			metricParts = append(metricParts, "source="+formatTokenValue(metric.Source))
		}
		if _, err := fmt.Fprintln(stdout, strings.Join(metricParts, " ")); err != nil {
			return fmt.Errorf("write probe metric: %w", err)
		}
	}
	for _, reading := range probe.Readings {
		readingParts := []string{
			"reading",
			"service=" + reading.ServiceUUID,
			"characteristic=" + reading.CharacteristicUUID,
			"label=" + reading.Label,
			fmt.Sprintf("bytes=%d", reading.Bytes),
		}
		if reading.ValueHex != "" {
			readingParts = append(readingParts, "value_hex="+reading.ValueHex)
		}
		if reading.Text != "" {
			readingParts = append(readingParts, fmt.Sprintf("text=%q", reading.Text))
		}
		if _, err := fmt.Fprintln(stdout, strings.Join(readingParts, " ")); err != nil {
			return fmt.Errorf("write probe reading: %w", err)
		}
	}
	return nil
}

func joinUint16s(values []uint16) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.Itoa(int(value)))
	}
	return strings.Join(parts, ",")
}

func formatTokenValue(value string) string {
	if value == "" {
		return `""`
	}
	if strings.ContainsAny(value, " \t\n\"'") {
		return strconv.Quote(value)
	}
	return value
}

func formatProbeJSON(device discoveredBLEDevice, probe deviceProbe, redact bool) string {
	body, err := json.Marshal(struct {
		Type   string              `json:"type"`
		Device discoveredBLEDevice `json:"device"`
		Probe  deviceProbe         `json:"probe"`
	}{
		Type:   "probe",
		Device: displayDevice(device, redact),
		Probe:  probe,
	})
	if err != nil {
		return `{"type":"probe","error":"encode failed"}`
	}
	return string(body)
}

type outputFormatValue outputFormat

func (v *outputFormatValue) String() string {
	if v == nil {
		return string(outputFormatText)
	}
	return string(*v)
}

func (v *outputFormatValue) Set(value string) error {
	format := outputFormat(strings.ToLower(strings.TrimSpace(value)))
	switch format {
	case outputFormatText, outputFormatJSON:
		*v = outputFormatValue(format)
		return nil
	default:
		return fmt.Errorf("unsupported output format %q", value)
	}
}

func (bluetoothScanner) Scan(stop <-chan struct{}, emit func(discoveredBLEDevice)) error {
	adapter := bluetooth.DefaultAdapter
	if err := enableBluetoothAdapter(adapter); err != nil {
		return fmt.Errorf("enable BLE adapter: %w", err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- adapter.Scan(func(_ *bluetooth.Adapter, result bluetooth.ScanResult) {
			emit(discoveredFromScanResult(result))
		})
	}()

	select {
	case <-stop:
		if err := adapter.StopScan(); err != nil {
			select {
			case scanErr := <-errCh:
				return scanErr
			default:
				return fmt.Errorf("stop BLE scan: %w", err)
			}
		}
		if err := <-errCh; err != nil {
			return fmt.Errorf("scan BLE advertisements: %w", err)
		}
		return nil
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("scan BLE advertisements: %w", err)
		}
		return nil
	}
}

func (bluetoothProber) Probe(ctx context.Context, device discoveredBLEDevice) (deviceProbe, error) {
	if strings.TrimSpace(device.Address) == "" {
		return deviceProbe{}, errors.New("selected device has no BLE address")
	}

	adapter := bluetooth.DefaultAdapter
	if err := enableBluetoothAdapter(adapter); err != nil {
		return deviceProbe{}, fmt.Errorf("enable BLE adapter: %w", err)
	}

	var address bluetooth.Address
	address.Set(device.Address)
	connectionTimeout := 10 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining > 0 {
			connectionTimeout = remaining
		}
	}
	remote, err := adapter.Connect(address, bluetooth.ConnectionParams{
		ConnectionTimeout: bluetooth.NewDuration(connectionTimeout),
	})
	if err != nil {
		return deviceProbe{}, fmt.Errorf("connect to %s: %w", device.Address, err)
	}
	defer func() {
		_ = remote.Disconnect()
	}()

	select {
	case <-ctx.Done():
		return deviceProbe{}, ctx.Err()
	default:
	}

	services, err := remote.DiscoverServices(nil)
	if err != nil {
		return deviceProbe{}, fmt.Errorf("discover services: %w", err)
	}

	probe := deviceProbe{
		Capabilities: probeCapabilities{
			ServiceCount: len(services),
			Services:     make([]probeService, 0, len(services)),
		},
	}
	mtus := make(map[uint16]struct{})
	knownReads := knownReadableCharacteristics()

	for _, service := range services {
		select {
		case <-ctx.Done():
			return deviceProbe{}, ctx.Err()
		default:
		}

		serviceUUID := service.UUID().String()
		serviceProbe := probeService{UUID: serviceUUID}
		characteristics, err := service.DiscoverCharacteristics(nil)
		if err != nil {
			return deviceProbe{}, fmt.Errorf("discover characteristics for service %s: %w", serviceUUID, err)
		}
		probe.Capabilities.CharacteristicCount += len(characteristics)
		serviceProbe.Characteristics = make([]string, 0, len(characteristics))
		for _, characteristic := range characteristics {
			characteristicUUID := characteristic.UUID().String()
			serviceProbe.Characteristics = append(serviceProbe.Characteristics, characteristicUUID)
			if mtu, err := characteristic.GetMTU(); err == nil && mtu > 0 {
				mtus[mtu] = struct{}{}
			}
			knownRead, ok := knownReads[strings.ToLower(characteristicUUID)]
			if !ok {
				continue
			}
			reading, metrics, ok := readKnownCharacteristic(serviceUUID, characteristicUUID, characteristic, knownRead)
			if ok {
				probe.Readings = append(probe.Readings, reading)
				probe.Metrics = append(probe.Metrics, metrics...)
			}
		}
		sort.Strings(serviceProbe.Characteristics)
		probe.Capabilities.Services = append(probe.Capabilities.Services, serviceProbe)
	}

	sort.Slice(probe.Capabilities.Services, func(i, j int) bool {
		return probe.Capabilities.Services[i].UUID < probe.Capabilities.Services[j].UUID
	})
	for mtu := range mtus {
		probe.Capabilities.MTUs = append(probe.Capabilities.MTUs, mtu)
	}
	sort.Slice(probe.Capabilities.MTUs, func(i, j int) bool {
		return probe.Capabilities.MTUs[i] < probe.Capabilities.MTUs[j]
	})
	return probe, nil
}

func enableBluetoothAdapter(adapter bluetoothAdapter) error {
	if err := adapter.Enable(); err != nil {
		if strings.Contains(err.Error(), "already calling Enable function") {
			return nil
		}
		return err
	}
	return nil
}

func discoveredFromScanResult(result bluetooth.ScanResult) discoveredBLEDevice {
	device := discoveredBLEDevice{
		Address: result.Address.String(),
		RSSI:    result.RSSI,
	}
	if result.AdvertisementPayload == nil {
		return device
	}
	device.LocalName = strings.TrimSpace(result.LocalName())
	for _, uuid := range result.ServiceUUIDs() {
		device.ServiceUUIDs = append(device.ServiceUUIDs, uuid.String())
	}
	sort.Strings(device.ServiceUUIDs)
	manufacturerData := result.ManufacturerData()
	if len(manufacturerData) > 0 {
		device.ManufacturerData = make(map[string]string, len(manufacturerData))
		for _, element := range manufacturerData {
			device.ManufacturerData[fmt.Sprintf("0x%04x", element.CompanyID)] = hex.EncodeToString(element.Data)
		}
	}
	return device
}

type knownCharacteristicRead struct {
	Label      string
	MetricName string
	Unit       string
	Kind       string
}

func knownReadableCharacteristics() map[string]knownCharacteristicRead {
	return map[string]knownCharacteristicRead{
		strings.ToLower(bluetooth.CharacteristicUUIDBatteryLevel.String()): {
			Label:      "battery_percent",
			MetricName: "battery_percent",
			Unit:       "%",
			Kind:       "battery_percent",
		},
		strings.ToLower(bluetooth.CharacteristicUUIDModelNumberString.String()): {
			Label: "model_number",
			Kind:  "text",
		},
		strings.ToLower(bluetooth.CharacteristicUUIDFirmwareRevisionString.String()): {
			Label: "firmware_revision",
			Kind:  "text",
		},
		strings.ToLower(bluetooth.CharacteristicUUIDManufacturerNameString.String()): {
			Label: "manufacturer_name",
			Kind:  "text",
		},
	}
}

func readKnownCharacteristic(
	serviceUUID string,
	characteristicUUID string,
	characteristic bluetooth.DeviceCharacteristic,
	knownRead knownCharacteristicRead,
) (probeReading, []probeMetric, bool) {
	buf := make([]byte, 128)
	n, err := characteristic.Read(buf)
	if err != nil || n <= 0 {
		return probeReading{}, nil, false
	}
	data := append([]byte(nil), buf[:n]...)
	reading := probeReading{
		ServiceUUID:        serviceUUID,
		CharacteristicUUID: characteristicUUID,
		Label:              knownRead.Label,
		Bytes:              len(data),
		ValueHex:           hex.EncodeToString(data),
	}
	var metrics []probeMetric
	switch knownRead.Kind {
	case "battery_percent":
		value := strconv.Itoa(int(data[0]))
		reading.Text = value + "%"
		metrics = append(metrics, probeMetric{
			Name:   knownRead.MetricName,
			Value:  value,
			Unit:   knownRead.Unit,
			Source: "gatt",
		})
	case "text":
		reading.Text = printableProbeText(data)
	}
	return reading, metrics, true
}

func printableProbeText(data []byte) string {
	data = []byte(strings.TrimSpace(string(data)))
	if len(data) == 0 || !utf8.Valid(data) {
		return ""
	}
	for _, r := range string(data) {
		if r < 32 || r == 127 {
			return ""
		}
	}
	return string(data)
}

func inferEcoFlowDevice(localName string) ecoFlowDeviceInfo {
	name := strings.TrimSpace(localName)
	if name == "" {
		return ecoFlowDeviceInfo{}
	}
	normalized := strings.ToUpper(name)
	normalized = strings.TrimPrefix(normalized, "EF-")

	for _, hint := range sortedEcoFlowPrefixHints() {
		if strings.HasPrefix(normalized, hint.Prefix) {
			return ecoFlowDeviceInfo{
				Matched:      true,
				Prefix:       hint.Prefix,
				Model:        hint.Model,
				PacketFamily: hint.PacketFamily,
			}
		}
	}
	if strings.HasPrefix(strings.ToUpper(name), "EF-") || strings.Contains(strings.ToUpper(name), "ECOFLOW") {
		return ecoFlowDeviceInfo{
			Matched: true,
			Prefix:  bestEffortPrefix(normalized),
			Model:   "EcoFlow device",
		}
	}
	return ecoFlowDeviceInfo{}
}

func sortedEcoFlowPrefixHints() []blePrefixHint {
	hints := append([]blePrefixHint(nil), ecoflowBLEPrefixHints...)
	sort.SliceStable(hints, func(i, j int) bool {
		if len(hints[i].Prefix) == len(hints[j].Prefix) {
			return hints[i].Prefix < hints[j].Prefix
		}
		return len(hints[i].Prefix) > len(hints[j].Prefix)
	})
	return hints
}

func bestEffortPrefix(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 4 {
		return value
	}
	return value[:4]
}

func shouldIncludeAdvertisement(device discoveredBLEDevice, cfg discoveryConfig) bool {
	if cfg.MinRSSI != 0 && device.RSSI != 0 && int(device.RSSI) < cfg.MinRSSI {
		return false
	}
	if cfg.IncludeAll {
		return true
	}
	info := device.Info
	if !info.Matched {
		info = inferEcoFlowDevice(device.LocalName)
	}
	if info.Matched {
		return true
	}
	prefix := strings.TrimSpace(cfg.NamePrefix)
	if prefix == "" {
		return false
	}
	return strings.HasPrefix(strings.ToUpper(strings.TrimSpace(device.LocalName)), strings.ToUpper(prefix))
}

func formatDiscoveryText(device discoveredBLEDevice, redact bool) string {
	display := displayDevice(device, redact)
	parts := []string{
		"device",
		"address=" + display.Address,
		fmt.Sprintf("name=%q", display.LocalName),
		fmt.Sprintf("rssi=%d", display.RSSI),
	}
	if display.Info.Model != "" {
		parts = append(parts, fmt.Sprintf("model=%q", display.Info.Model))
	}
	if display.Info.Prefix != "" {
		parts = append(parts, "prefix="+display.Info.Prefix)
	}
	if display.Info.PacketFamily != "" {
		parts = append(parts, "packets="+display.Info.PacketFamily)
	}
	if len(display.ServiceUUIDs) > 0 {
		parts = append(parts, fmt.Sprintf("services=%d", len(display.ServiceUUIDs)))
	}
	if len(display.ManufacturerData) > 0 {
		parts = append(parts, fmt.Sprintf("manufacturer=%d", len(display.ManufacturerData)))
	}
	return strings.Join(parts, " ")
}

func formatDiscoveryJSON(device discoveredBLEDevice, redact bool) string {
	display := displayDevice(device, redact)
	body, err := json.Marshal(struct {
		Type string `json:"type"`
		discoveredBLEDevice
	}{
		Type:                "device",
		discoveredBLEDevice: display,
	})
	if err != nil {
		return `{"type":"device","error":"encode failed"}`
	}
	return string(body)
}

func displayDevice(device discoveredBLEDevice, redact bool) discoveredBLEDevice {
	if !redact {
		return device
	}
	device.Address = redactIdentifier(device.Address, 4, 4)
	device.LocalName = redactLocalName(device.LocalName)
	return device
}

func redactIdentifier(value string, prefixChars int, suffixChars int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) <= prefixChars+suffixChars+3 {
		return value
	}
	return value[:prefixChars] + "..." + value[len(value)-suffixChars:]
}

func redactLocalName(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(strings.ToUpper(value), "EF-") {
		return redactIdentifier(value, 5, 4)
	}
	return redactIdentifier(value, 4, 4)
}

func discoveryKey(device discoveredBLEDevice) string {
	if address := strings.TrimSpace(device.Address); address != "" {
		return "addr:" + strings.ToUpper(address)
	}
	if name := strings.TrimSpace(device.LocalName); name != "" {
		return "name:" + strings.ToUpper(name)
	}
	return fmt.Sprintf("rssi:%d", device.RSSI)
}

type summaryCounts struct {
	Seen    int
	EcoFlow int
}

func discoverySummary(seen map[string]discoveredBLEDevice) summaryCounts {
	var summary summaryCounts
	for _, device := range seen {
		summary.Seen++
		info := device.Info
		if !info.Matched {
			info = inferEcoFlowDevice(device.LocalName)
		}
		if info.Matched {
			summary.EcoFlow++
		}
	}
	return summary
}

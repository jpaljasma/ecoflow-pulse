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
	"math"
	"os"
	"os/signal"
	"path/filepath"
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
type activeProbeMode string
type bleTransport string

const (
	outputFormatText outputFormat = "text"
	outputFormatJSON outputFormat = "json"

	activeProbeNone       activeProbeMode = "none"
	activeProbeAuto       activeProbeMode = "auto"
	activeProbeECDH       activeProbeMode = "ecdh"
	activeProbeAuthStatus activeProbeMode = "auth-status"

	bleTransportAuto   bleTransport = "auto"
	bleTransportRFCOMM bleTransport = "rfcomm"
	bleTransportAlt    bleTransport = "alt"
	bleTransportBoth   bleTransport = "both"

	defaultRawOutputPath = ".tmp/ecoflow-ble-discover-raw.jsonl"
)

type discoveryConfig struct {
	Duration            time.Duration
	Format              outputFormat
	IncludeAll          bool
	Redact              bool
	MinRSSI             int
	NamePrefix          string
	ScanOnly            bool
	Selection           string
	ProbeTimeout        time.Duration
	ProbeTimeoutSet     bool
	ListenDuration      time.Duration
	ListenDurationSet   bool
	ListenUntilCanceled bool
	RawNotifications    bool
	ActiveProbe         activeProbeMode
	ActiveProbeSet      bool
	BLETransport        bleTransport
	AuthUserID          string
	AuthDeviceSerial    string
	RawOutputPath       string
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
	Capabilities  probeCapabilities   `json:"capabilities"`
	Metrics       []probeMetric       `json:"metrics,omitempty"`
	Readings      []probeReading      `json:"readings,omitempty"`
	Notifications []probeNotification `json:"notifications,omitempty"`
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
	Name    string `json:"name"`
	Value   string `json:"value"`
	Unit    string `json:"unit,omitempty"`
	Source  string `json:"source"`
	Decoder string `json:"decoder,omitempty"`
}

type probeReading struct {
	ServiceUUID        string `json:"service_uuid"`
	CharacteristicUUID string `json:"characteristic_uuid"`
	Label              string `json:"label"`
	Bytes              int    `json:"bytes"`
	ValueHex           string `json:"value_hex,omitempty"`
	Text               string `json:"text,omitempty"`
}

type probeNotification struct {
	Direction          string `json:"direction,omitempty"`
	ServiceUUID        string `json:"service_uuid"`
	CharacteristicUUID string `json:"characteristic_uuid"`
	Role               string `json:"role,omitempty"`
	Step               string `json:"step,omitempty"`
	Bytes              int    `json:"bytes"`
	Frame              string `json:"frame,omitempty"`
	Packet             string `json:"packet,omitempty"`
	Detail             string `json:"detail,omitempty"`
	ValueHex           string `json:"value_hex,omitempty"`
	DecodedHex         string `json:"decoded_hex,omitempty"`
}

type probeOptions struct {
	ListenDuration      time.Duration
	ListenUntilCanceled bool
	RawNotifications    bool
	ActiveProbe         activeProbeMode
	BLETransport        bleTransport
	AuthUserID          string
	AuthDeviceSerial    string
	EventSink           probeEventSink
}

type probeEventSink func(probeEvent) error

type probeEvent struct {
	Capabilities  *probeCapabilities
	Notifications []probeNotification
	Metrics       []probeMetric
	Readings      []probeReading
}

type bleAuthFailureError struct {
	Result string
}

func (e bleAuthFailureError) Error() string {
	return "BLE authentication failed: " + e.Result
}

func probeEventAuthFailure(event probeEvent) error {
	return probeMetricsAuthFailure(event.Metrics)
}

func probeMetricsAuthFailure(metrics []probeMetric) error {
	for _, metric := range metrics {
		if metric.Name != "auth_result" {
			continue
		}
		result := strings.TrimSpace(metric.Value)
		if result == "" {
			result = "empty"
		}
		if strings.EqualFold(result, "ok") {
			return nil
		}
		return bleAuthFailureError{Result: result}
	}
	return nil
}

func isBLEAuthFailure(err error) bool {
	var authErr bleAuthFailureError
	return errors.As(err, &authErr)
}

type probeRawEventLogger struct {
	mu      sync.Mutex
	file    *os.File
	encoder *json.Encoder
}

func newProbeRawEventLogger(path string) (*probeRawEventLogger, error) {
	cleanPath := filepath.Clean(strings.TrimSpace(path))
	if cleanPath == "." || cleanPath == "" {
		return nil, errors.New("raw-output path must not be empty")
	}
	if dir := filepath.Dir(cleanPath); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create raw output directory: %w", err)
		}
	}
	file, err := os.Create(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("create raw output file: %w", err)
	}
	return &probeRawEventLogger{
		file:    file,
		encoder: json.NewEncoder(file),
	}, nil
}

func (l *probeRawEventLogger) Handle(device discoveredBLEDevice, event probeEvent) error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.encoder.Encode(struct {
		Type   string              `json:"type"`
		Time   string              `json:"time"`
		Device discoveredBLEDevice `json:"device"`
		Event  probeEvent          `json:"event"`
	}{
		Type:   "probe_event",
		Time:   time.Now().UTC().Format(time.RFC3339Nano),
		Device: displayDevice(device, false),
		Event:  event,
	}); err != nil {
		return fmt.Errorf("write raw probe event: %w", err)
	}
	return nil
}

func (l *probeRawEventLogger) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	return l.file.Close()
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
				probe, _, err := probeSelectedDevice(parent, cfg, selected, prober, nil)
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
	if isAutoProbeSelection(cfg.Selection) {
		selected := supportedAutoProbeDevices(devices)
		if len(selected) == 0 {
			if _, err := fmt.Fprintln(stdout, "probe skipped no supported devices"); err != nil {
				return fmt.Errorf("write probe skipped: %w", err)
			}
			return nil
		}
		return probeDiscoveredDevices(parent, cfg, selected, prober, stdout)
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
	var eventSink probeEventSink
	if supportsProbeEvents(prober) {
		streamer, err := newProbeTextStreamer(stdout, selected, cfg.Redact)
		if err != nil {
			return err
		}
		eventSink = func(event probeEvent) error {
			if err := streamer.Handle(event); err != nil {
				return err
			}
			return probeEventAuthFailure(event)
		}
	}
	probe, streamed, err := probeSelectedDevice(parent, cfg, selected, prober, eventSink)
	if err != nil {
		return err
	}
	if streamed {
		return nil
	}
	if err := printProbeText(stdout, selected, probe, cfg.Redact); err != nil {
		return err
	}
	return nil
}

func parseDiscoveryConfig(args []string, stderr io.Writer) (discoveryConfig, error) {
	cfg := discoveryConfig{
		Duration:       5 * time.Second,
		Format:         outputFormatText,
		Redact:         true,
		MinRSSI:        -100,
		NamePrefix:     "EF-",
		ProbeTimeout:   10 * time.Second,
		ListenDuration: 5 * time.Second,
		ActiveProbe:    activeProbeNone,
		BLETransport:   bleTransportAuto,
		RawOutputPath:  defaultRawOutputPath,
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
	fs.DurationVar(&cfg.ProbeTimeout, "probe-timeout", cfg.ProbeTimeout, "maximum time to spend probing the selected device; use 0 to run until interrupted")
	fs.DurationVar(&cfg.ListenDuration, "listen-duration", cfg.ListenDuration, "time to listen for EcoFlow BLE notification frames after service discovery; use 0 to disable")
	fs.BoolVar(&cfg.RawNotifications, "raw-notifications", cfg.RawNotifications, "include raw and decoded BLE probe buffer hex in probe output")
	fs.Var((*activeProbeValue)(&cfg.ActiveProbe), "active-probe", "active BLE probe to send after notifications are enabled: none, auto, ecdh, or auth-status")
	fs.Var((*bleTransportValue)(&cfg.BLETransport), "ble-transport", "EcoFlow BLE transport to use for active probes: auto, rfcomm, alt, or both")
	fs.StringVar(&cfg.AuthUserID, "auth-user-id", cfg.AuthUserID, "EcoFlow user ID for BLE authentication; defaults to ECOFLOW_BLE_USER_ID")
	fs.StringVar(&cfg.AuthDeviceSerial, "auth-device-serial", cfg.AuthDeviceSerial, "EcoFlow device serial for BLE authentication; defaults to advertisement serial or ECOFLOW_BLE_DEVICE_SERIAL")
	fs.StringVar(&cfg.RawOutputPath, "raw-output", cfg.RawOutputPath, "JSONL file for raw probe events; overwritten on each run; use empty to disable")
	if err := fs.Parse(args); err != nil {
		return discoveryConfig{}, err
	}
	cfg.ActiveProbeSet = flagWasPassed(args, "active-probe")
	cfg.ProbeTimeoutSet = flagWasPassed(args, "probe-timeout")
	cfg.ListenDurationSet = flagWasPassed(args, "listen-duration")
	if cfg.AuthUserID == "" {
		cfg.AuthUserID = strings.TrimSpace(os.Getenv("ECOFLOW_BLE_USER_ID"))
	}
	if cfg.AuthDeviceSerial == "" {
		cfg.AuthDeviceSerial = strings.TrimSpace(os.Getenv("ECOFLOW_BLE_DEVICE_SERIAL"))
	}
	cfg.RawOutputPath = strings.TrimSpace(cfg.RawOutputPath)
	return cfg, nil
}

func flagWasPassed(args []string, name string) bool {
	prefix := "-" + name
	for _, arg := range args {
		arg = strings.TrimSpace(arg)
		if arg == prefix || strings.HasPrefix(arg, prefix+"=") {
			return true
		}
		if arg == "--"+name || strings.HasPrefix(arg, "--"+name+"=") {
			return true
		}
	}
	return false
}

func validateDiscoveryConfig(cfg discoveryConfig) error {
	if cfg.Duration < 0 {
		return errors.New("duration must be zero or positive")
	}
	if cfg.ProbeTimeout < 0 {
		return errors.New("probe-timeout must be zero or positive")
	}
	if cfg.ListenDuration < 0 {
		return errors.New("listen-duration must be zero or positive")
	}
	switch cfg.Format {
	case outputFormatText, outputFormatJSON:
	default:
		return fmt.Errorf("format must be %q or %q", outputFormatText, outputFormatJSON)
	}
	switch cfg.ActiveProbe {
	case activeProbeNone, activeProbeAuto, activeProbeECDH, activeProbeAuthStatus:
	default:
		return fmt.Errorf("active-probe must be %q, %q, %q, or %q", activeProbeNone, activeProbeAuto, activeProbeECDH, activeProbeAuthStatus)
	}
	switch cfg.BLETransport {
	case bleTransportAuto, bleTransportRFCOMM, bleTransportAlt, bleTransportBoth:
		return nil
	default:
		return fmt.Errorf("ble-transport must be %q, %q, %q, or %q", bleTransportAuto, bleTransportRFCOMM, bleTransportAlt, bleTransportBoth)
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
	sort.SliceStable(devices, func(i, j int) bool {
		return discoveredDeviceSortKey(devices[i]) < discoveredDeviceSortKey(devices[j])
	})
	return devices
}

func discoveredDeviceSortKey(device discoveredBLEDevice) string {
	info := device.Info
	if !info.Matched {
		info = inferEcoFlowDevice(device.LocalName)
	}
	parts := []string{
		strings.ToUpper(device.LocalName),
		strings.ToUpper(info.Model),
		strings.ToUpper(device.Address),
	}
	return strings.Join(parts, "\x00")
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

func isAutoProbeSelection(selector string) bool {
	selector = strings.ToLower(strings.TrimSpace(selector))
	return selector == "" || selector == "auto" || selector == "supported" || selector == "all"
}

func supportedAutoProbeDevices(devices []discoveredBLEDevice) []discoveredBLEDevice {
	selected := make([]discoveredBLEDevice, 0, len(devices))
	for _, device := range devices {
		if isSupportedAutoProbeDevice(device) {
			selected = append(selected, device)
		}
	}
	return selected
}

func isSupportedAutoProbeDevice(device discoveredBLEDevice) bool {
	info := device.Info
	if !info.Matched {
		info = inferEcoFlowDevice(device.LocalName)
	}
	if !info.Matched || info.PacketFamily != "v3" {
		return false
	}
	model := strings.ToUpper(info.Model)
	return strings.Contains(model, "DELTA 3") || strings.Contains(model, "RIVER 3")
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

func supportsProbeEvents(prober deviceProber) bool {
	if prober == nil {
		return false
	}
	_, ok := prober.(interface {
		ProbeWithOptions(context.Context, discoveredBLEDevice, probeOptions) (deviceProbe, error)
	})
	return ok
}

func probeSelectedDevice(
	parent context.Context,
	cfg discoveryConfig,
	device discoveredBLEDevice,
	prober deviceProber,
	eventSink probeEventSink,
) (deviceProbe, bool, error) {
	if prober == nil {
		return deviceProbe{}, false, errors.New("probe unavailable")
	}
	var ctx context.Context
	var cancel context.CancelFunc
	if cfg.ProbeTimeout > 0 {
		ctx, cancel = context.WithTimeout(parent, cfg.ProbeTimeout)
	} else {
		ctx, cancel = context.WithCancel(parent)
	}
	defer cancel()

	advertisementMetrics := advertisementProbeMetrics(device)
	options := probeOptions{
		ListenDuration:      cfg.ListenDuration,
		ListenUntilCanceled: cfg.ListenUntilCanceled,
		RawNotifications:    cfg.RawNotifications,
		ActiveProbe:         cfg.ActiveProbe,
		BLETransport:        cfg.BLETransport,
		AuthUserID:          cfg.AuthUserID,
		AuthDeviceSerial:    cfg.AuthDeviceSerial,
		EventSink:           eventSink,
	}
	if optionProber, ok := prober.(interface {
		ProbeWithOptions(context.Context, discoveredBLEDevice, probeOptions) (deviceProbe, error)
	}); ok {
		if eventSink != nil {
			if err := eventSink(probeEvent{Metrics: advertisementMetrics}); err != nil {
				return deviceProbe{}, true, fmt.Errorf("stream advertisement metrics: %w", err)
			}
		}
		probe, err := optionProber.ProbeWithOptions(ctx, device, options)
		if err != nil {
			return deviceProbe{}, eventSink != nil, fmt.Errorf("probe selected device: %w", err)
		}
		probe.Metrics = append(advertisementMetrics, probe.Metrics...)
		if eventSink == nil {
			if err := probeMetricsAuthFailure(probe.Metrics); err != nil {
				return deviceProbe{}, false, err
			}
		}
		return probe, eventSink != nil, nil
	}

	probe, err := prober.Probe(ctx, device)
	if err != nil {
		return deviceProbe{}, false, fmt.Errorf("probe selected device: %w", err)
	}
	probe.Metrics = append(advertisementMetrics, probe.Metrics...)
	if err := probeMetricsAuthFailure(probe.Metrics); err != nil {
		return deviceProbe{}, false, err
	}
	return probe, false, nil
}

func probeDiscoveredDevices(
	parent context.Context,
	cfg discoveryConfig,
	devices []discoveredBLEDevice,
	prober deviceProber,
	stdout io.Writer,
) error {
	if prober == nil {
		return errors.New("probe unavailable")
	}
	if !cfg.ActiveProbeSet && cfg.ActiveProbe == activeProbeNone {
		cfg.ActiveProbe = activeProbeAuto
	}
	if !cfg.ProbeTimeoutSet {
		cfg.ProbeTimeout = 0
	}
	if !cfg.ListenDurationSet {
		cfg.ListenUntilCanceled = true
	}
	if _, err := fmt.Fprintf(stdout, "auto probing supported devices=%d\n", len(devices)); err != nil {
		return fmt.Errorf("write auto probe summary: %w", err)
	}

	probeCtx, cancelProbes := context.WithCancel(parent)
	defer cancelProbes()

	var rawLogger *probeRawEventLogger
	if cfg.RawOutputPath != "" {
		var err error
		rawLogger, err = newProbeRawEventLogger(cfg.RawOutputPath)
		if err != nil {
			return err
		}
		defer func() {
			_ = rawLogger.Close()
		}()
		cfg.RawNotifications = true
		if _, err := fmt.Fprintf(stdout, "raw_output path=%s\n", cfg.RawOutputPath); err != nil {
			return fmt.Errorf("write raw output path: %w", err)
		}
	}

	for _, device := range devices {
		if err := printProbeHeaderText(stdout, device, cfg.Redact); err != nil {
			return err
		}
	}
	if err := flushWriter(stdout); err != nil {
		return err
	}

	summaryBoard := newProbeSummaryBoard(stdout, devices, cfg.Redact)
	var wg sync.WaitGroup
	errCh := make(chan error, len(devices))
	for _, device := range devices {
		device := device
		wg.Add(1)
		go func() {
			defer wg.Done()
			eventSink := func(event probeEvent) error {
				if rawLogger != nil {
					if err := rawLogger.Handle(device, event); err != nil {
						return err
					}
				}
				if err := summaryBoard.Handle(device, event); err != nil {
					return err
				}
				if err := probeEventAuthFailure(event); err != nil {
					cancelProbes()
					return err
				}
				return nil
			}
			probe, streamed, err := probeSelectedDevice(probeCtx, cfg, device, prober, eventSink)
			if err != nil {
				if isBLEAuthFailure(err) {
					cancelProbes()
				}
				errCh <- fmt.Errorf("probe %s: %w", displayDevice(device, cfg.Redact).LocalName, err)
				return
			}
			if !streamed {
				if err := eventSink(probeEvent{
					Capabilities:  &probe.Capabilities,
					Notifications: probe.Notifications,
					Metrics:       probe.Metrics,
					Readings:      probe.Readings,
				}); err != nil {
					errCh <- err
				}
			}
		}()
	}
	wg.Wait()
	close(errCh)
	var firstErr error
	for err := range errCh {
		if err == nil {
			continue
		}
		if isBLEAuthFailure(err) {
			return err
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
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
	metrics = append(metrics, manufacturerScanRecordMetrics(device)...)
	return metrics
}

func printProbeText(stdout io.Writer, device discoveredBLEDevice, probe deviceProbe, redact bool) error {
	if err := printProbeHeaderText(stdout, device, redact); err != nil {
		return err
	}
	if err := printProbeCapabilitiesText(stdout, probe.Capabilities); err != nil {
		return err
	}
	if err := printProbeNotificationsText(stdout, probe.Notifications); err != nil {
		return err
	}
	if err := printProbeMetricsText(stdout, probe.Metrics); err != nil {
		return err
	}
	if err := printProbeReadingsText(stdout, probe.Readings); err != nil {
		return err
	}
	return nil
}

type probeTextStreamer struct {
	stdout io.Writer
}

func newProbeTextStreamer(stdout io.Writer, device discoveredBLEDevice, redact bool) (*probeTextStreamer, error) {
	streamer := &probeTextStreamer{stdout: stdout}
	if err := printProbeHeaderText(stdout, device, redact); err != nil {
		return nil, err
	}
	if err := flushWriter(stdout); err != nil {
		return nil, err
	}
	return streamer, nil
}

func (s *probeTextStreamer) Handle(event probeEvent) error {
	if event.Capabilities != nil {
		if err := printProbeCapabilitiesText(s.stdout, *event.Capabilities); err != nil {
			return err
		}
	}
	if err := printProbeNotificationsText(s.stdout, event.Notifications); err != nil {
		return err
	}
	if err := printProbeMetricsText(s.stdout, event.Metrics); err != nil {
		return err
	}
	if err := printProbeReadingsText(s.stdout, event.Readings); err != nil {
		return err
	}
	return flushWriter(s.stdout)
}

const (
	summaryMetricColumnWidth = 19
	summaryDeviceColumnWidth = 29
)

type probeSummaryBoard struct {
	stdout  io.Writer
	redact  bool
	devices []discoveredBLEDevice
	states  map[string]*probeSummaryState
	mu      sync.Mutex
}

type probeSummaryState struct {
	device            discoveredBLEDevice
	redact            bool
	capabilities      *probeCapabilities
	metrics           map[string]probeMetric
	refreshCount      int
	notificationCount int
}

type summaryTableRow struct {
	Label  string
	Values []string
}

func newProbeSummaryBoard(stdout io.Writer, devices []discoveredBLEDevice, redact bool) *probeSummaryBoard {
	board := &probeSummaryBoard{
		stdout:  stdout,
		redact:  redact,
		devices: append([]discoveredBLEDevice(nil), devices...),
		states:  make(map[string]*probeSummaryState, len(devices)),
	}
	for _, device := range devices {
		board.states[probeSummaryDeviceKey(device)] = newProbeSummaryState(device, redact)
	}
	return board
}

func newProbeSummaryState(device discoveredBLEDevice, redact bool) *probeSummaryState {
	return &probeSummaryState{
		device:  device,
		redact:  redact,
		metrics: make(map[string]probeMetric),
	}
}

func (b *probeSummaryBoard) Handle(device discoveredBLEDevice, event probeEvent) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	key := probeSummaryDeviceKey(device)
	state := b.states[key]
	if state == nil {
		state = newProbeSummaryState(device, b.redact)
		b.states[key] = state
	}
	if !state.apply(event) {
		return nil
	}
	if _, err := fmt.Fprintln(b.stdout, b.formatSnapshot()); err != nil {
		return fmt.Errorf("write probe refresh summary: %w", err)
	}
	return flushWriter(b.stdout)
}

func (s *probeSummaryState) apply(event probeEvent) bool {
	if event.Capabilities != nil {
		capabilities := *event.Capabilities
		s.capabilities = &capabilities
	}
	for _, notification := range event.Notifications {
		if notification.Packet != "" {
			s.notificationCount++
		}
	}
	for _, metric := range event.Metrics {
		s.metrics[metric.Name] = metric
	}
	if len(event.Metrics) == 0 && event.Capabilities == nil {
		return false
	}
	s.refreshCount++
	return true
}

func (b *probeSummaryBoard) formatSnapshot() string {
	headers := make([]string, 0, len(b.devices))
	for _, device := range b.devices {
		state := b.states[probeSummaryDeviceKey(device)]
		if state == nil {
			state = newProbeSummaryState(device, b.redact)
		}
		headers = append(headers, state.headerSummary())
	}

	rows := []summaryTableRow{
		b.summaryRow("Device", (*probeSummaryState).deviceSummary),
		b.summaryRow("Model", (*probeSummaryState).modelSummary),
		b.summaryRow("Address", (*probeSummaryState).addressSummary),
		b.summaryRow("Update", func(state *probeSummaryState) string {
			return strconv.Itoa(state.refreshCount)
		}),
		b.summaryRow("Packets", func(state *probeSummaryState) string {
			return strconv.Itoa(state.notificationCount)
		}),
		b.summaryRow("Current load", (*probeSummaryState).currentLoadSummary),
		b.summaryRow("Solar in", (*probeSummaryState).solarInSummary),
		b.summaryRow("Battery charge", (*probeSummaryState).batteryChargeSummary),
		b.summaryRow("ETA", (*probeSummaryState).etaSummary),
		b.summaryRow("Services", (*probeSummaryState).serviceCountSummary),
		b.summaryRow("Characteristics", (*probeSummaryState).characteristicCountSummary),
		b.summaryRow("MTUs", (*probeSummaryState).mtuSummary),
		b.summaryRow("RSSI", func(state *probeSummaryState) string {
			return state.metricSummary("rssi_dbm")
		}),
		b.summaryRow("Auth", func(state *probeSummaryState) string {
			return state.metricSummary("auth_result")
		}),
		b.summaryRow("Total input", func(state *probeSummaryState) string {
			return state.metricSummary("input_power_w")
		}),
		b.summaryRow("AC input", func(state *probeSummaryState) string {
			return state.metricSummary("ac_input_power_w")
		}),
		b.summaryRow("AC output", func(state *probeSummaryState) string {
			return state.metricSummary("ac_output_power_w")
		}),
		b.summaryRow("Battery power", func(state *probeSummaryState) string {
			return state.metricSummary("battery_power_w")
		}),
		b.summaryRow("DC charging", func(state *probeSummaryState) string {
			return state.metricSummary("dc_charging_type", "pv_input_state")
		}),
		b.summaryRow("AC charger", func(state *probeSummaryState) string {
			return state.metricSummary("ac_charger_connected", "ac_input_plugged")
		}),
		b.summaryRow("AC output enabled", func(state *probeSummaryState) string {
			return state.metricSummary("ac_output_enabled")
		}),
	}
	return formatFixedSummaryTable(headers, rows)
}

func (b *probeSummaryBoard) summaryRow(label string, value func(*probeSummaryState) string) summaryTableRow {
	row := summaryTableRow{
		Label:  label,
		Values: make([]string, 0, len(b.devices)),
	}
	for _, device := range b.devices {
		state := b.states[probeSummaryDeviceKey(device)]
		if state == nil {
			state = newProbeSummaryState(device, b.redact)
		}
		row.Values = append(row.Values, value(state))
	}
	return row
}

func (s *probeSummaryState) headerSummary() string {
	display := displayDevice(s.device, s.redact)
	if display.LocalName != "" {
		return display.LocalName
	}
	return nonEmptySummaryValue(display.Address)
}

func (s *probeSummaryState) deviceSummary() string {
	return s.headerSummary()
}

func (s *probeSummaryState) modelSummary() string {
	display := displayDevice(s.device, s.redact)
	return nonEmptySummaryValue(display.Info.Model)
}

func (s *probeSummaryState) addressSummary() string {
	display := displayDevice(s.device, s.redact)
	return nonEmptySummaryValue(display.Address)
}

func (s *probeSummaryState) serviceCountSummary() string {
	if s.capabilities == nil {
		return "-"
	}
	return strconv.Itoa(s.capabilities.ServiceCount)
}

func (s *probeSummaryState) characteristicCountSummary() string {
	if s.capabilities == nil {
		return "-"
	}
	return strconv.Itoa(s.capabilities.CharacteristicCount)
}

func (s *probeSummaryState) mtuSummary() string {
	if s.capabilities == nil || len(s.capabilities.MTUs) == 0 {
		return "-"
	}
	return joinUint16s(s.capabilities.MTUs)
}

func (s *probeSummaryState) metricSummary(names ...string) string {
	if value, ok := s.summaryMetricDisplay(names...); ok {
		return value
	}
	return "-"
}

func (s *probeSummaryState) currentLoadSummary() string {
	if value, ok := s.summaryPowerDisplay("output_power_w", false); ok {
		return value
	}
	if value, ok := s.summaryPowerDisplay("ac_output_power_w", true); ok {
		return value
	}
	return "-"
}

func (s *probeSummaryState) solarInSummary() string {
	var total float64
	found := false
	for _, name := range []string{"pv_input_power_w", "pv2_input_power_w"} {
		value, _, ok := s.summaryMetricFloat(name)
		if !ok {
			continue
		}
		total += value
		found = true
	}
	if found {
		return formatSummaryUnitValue(formatSummaryNumber(total), "W")
	}
	if value, ok := s.summaryMetricDisplay("pv_input_power_w", "pv2_input_power_w"); ok {
		return value
	}
	return "-"
}

func (s *probeSummaryState) batteryChargeSummary() string {
	if value, ok := s.summaryMetricDisplay("battery_soc_percent", "main_battery_soc_percent"); ok {
		return value
	}
	return "-"
}

func (s *probeSummaryState) etaSummary() string {
	batteryPower, _, hasBatteryPower := s.summaryMetricFloat("battery_power_w")
	if hasBatteryPower {
		if batteryPower < -0.5 {
			if soc, _, ok := s.summaryMetricFloat("battery_soc_percent", "main_battery_soc_percent"); ok && soc >= 99.5 {
				return "full"
			}
			if value, ok := s.remainingMinutesSummary("charge", "battery_charge_remaining_min"); ok {
				return value
			}
		}
		if batteryPower > 0.5 {
			if value, ok := s.remainingMinutesSummary("discharge", "battery_discharge_remaining_min"); ok {
				return value
			}
		}
	}
	if value, ok := s.remainingMinutesSummary("discharge", "battery_discharge_remaining_min"); ok {
		return value
	}
	if value, ok := s.remainingMinutesSummary("charge", "battery_charge_remaining_min"); ok {
		return value
	}
	return "-"
}

func (s *probeSummaryState) summaryPowerDisplay(name string, absolute bool) (string, bool) {
	value, metric, ok := s.summaryMetricFloat(name)
	if !ok {
		if display, ok := s.summaryMetricDisplay(name); ok {
			return display, true
		}
		return "", false
	}
	if absolute {
		value = math.Abs(value)
	}
	unit := metric.Unit
	if unit == "" {
		unit = "W"
	}
	return formatSummaryUnitValue(formatSummaryNumber(value), unit), true
}

func (s *probeSummaryState) remainingMinutesSummary(label, name string) (string, bool) {
	value, _, ok := s.summaryMetricFloat(name)
	if !ok {
		return "", false
	}
	minutes := int(math.Round(value))
	if minutes < 0 {
		return "", false
	}
	return label + ": " + formatSummaryMinutes(minutes), true
}

func (s *probeSummaryState) summaryMetricDisplay(names ...string) (string, bool) {
	for _, name := range names {
		metric, ok := s.metrics[name]
		if !ok {
			continue
		}
		return formatSummaryUnitValue(metric.Value, metric.Unit), true
	}
	return "", false
}

func (s *probeSummaryState) summaryMetricFloat(names ...string) (float64, probeMetric, bool) {
	for _, name := range names {
		metric, ok := s.metrics[name]
		if !ok {
			continue
		}
		value, err := strconv.ParseFloat(metric.Value, 64)
		if err != nil {
			continue
		}
		return value, metric, true
	}
	return 0, probeMetric{}, false
}

func formatFixedSummaryTable(headers []string, rows []summaryTableRow) string {
	border := fixedSummaryTableBorder(len(headers))
	lines := []string{border}
	lines = append(lines, fixedSummaryTableLine("Metric", headers))
	lines = append(lines, border)
	for _, row := range rows {
		lines = append(lines, fixedSummaryTableLine(row.Label, row.Values))
	}
	lines = append(lines, border)
	return strings.Join(lines, "\n")
}

func fixedSummaryTableBorder(deviceCount int) string {
	var builder strings.Builder
	builder.WriteByte('+')
	builder.WriteString(strings.Repeat("-", summaryMetricColumnWidth+2))
	builder.WriteByte('+')
	for range deviceCount {
		builder.WriteString(strings.Repeat("-", summaryDeviceColumnWidth+2))
		builder.WriteByte('+')
	}
	return builder.String()
}

func fixedSummaryTableLine(label string, values []string) string {
	var builder strings.Builder
	builder.WriteString("| ")
	builder.WriteString(fixedSummaryCell(label, summaryMetricColumnWidth))
	builder.WriteString(" |")
	for _, value := range values {
		builder.WriteByte(' ')
		builder.WriteString(fixedSummaryCell(value, summaryDeviceColumnWidth))
		builder.WriteString(" |")
	}
	return builder.String()
}

func fixedSummaryCell(value string, width int) string {
	value = nonEmptySummaryValue(sanitizeTableCell(value))
	if len(value) > width {
		if width <= 3 {
			return value[:width]
		}
		value = value[:width-3] + "..."
	}
	return value + strings.Repeat(" ", width-len(value))
}

func sanitizeTableCell(value string) string {
	replacer := strings.NewReplacer("\r", " ", "\n", " ", "\t", " ", "|", "/")
	return replacer.Replace(value)
}

func nonEmptySummaryValue(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func probeSummaryDeviceKey(device discoveredBLEDevice) string {
	if device.Address != "" {
		return device.Address
	}
	if device.LocalName != "" {
		return device.LocalName
	}
	if device.Info.Prefix != "" {
		return device.Info.Prefix
	}
	return device.Info.Model
}

func formatSummaryUnitValue(value, unit string) string {
	if value == "" {
		value = "-"
	}
	if unit == "" {
		return value
	}
	if unit == "%" {
		return value + "%"
	}
	return value + " " + unit
}

func formatSummaryNumber(value float64) string {
	if math.Abs(value) < 0.0000001 {
		value = 0
	}
	rounded := math.Round(value)
	if math.Abs(value-rounded) < 0.0000001 {
		return strconv.FormatInt(int64(rounded), 10)
	}
	text := strconv.FormatFloat(value, 'f', 2, 64)
	text = strings.TrimRight(text, "0")
	return strings.TrimRight(text, ".")
}

func formatSummaryMinutes(minutes int) string {
	hours := minutes / 60
	remainder := minutes % 60
	switch {
	case hours == 0:
		return fmt.Sprintf("%dm", remainder)
	case remainder == 0:
		return fmt.Sprintf("%dh", hours)
	default:
		return fmt.Sprintf("%dh %02dm", hours, remainder)
	}
}

func printProbeHeaderText(stdout io.Writer, device discoveredBLEDevice, redact bool) error {
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
	return nil
}

func printProbeCapabilitiesText(stdout io.Writer, capabilities probeCapabilities) error {
	capabilityParts := []string{
		"capabilities",
		fmt.Sprintf("services=%d", capabilities.ServiceCount),
		fmt.Sprintf("characteristics=%d", capabilities.CharacteristicCount),
	}
	if len(capabilities.MTUs) > 0 {
		capabilityParts = append(capabilityParts, "mtus="+joinUint16s(capabilities.MTUs))
	}
	if _, err := fmt.Fprintln(stdout, strings.Join(capabilityParts, " ")); err != nil {
		return fmt.Errorf("write probe capabilities: %w", err)
	}
	for _, service := range capabilities.Services {
		if _, err := fmt.Fprintf(stdout, "service uuid=%s characteristics=%d\n", service.UUID, len(service.Characteristics)); err != nil {
			return fmt.Errorf("write probe service: %w", err)
		}
		for _, characteristicUUID := range service.Characteristics {
			characteristicParts := []string{
				"characteristic",
				"uuid=" + characteristicUUID,
			}
			if role := inferEcoFlowCharacteristicRole(service.UUID, characteristicUUID); role != "" {
				characteristicParts = append(characteristicParts, "role="+role)
			}
			if _, err := fmt.Fprintln(stdout, strings.Join(characteristicParts, " ")); err != nil {
				return fmt.Errorf("write probe characteristic: %w", err)
			}
		}
	}
	return nil
}

func printProbeNotificationsText(stdout io.Writer, notifications []probeNotification) error {
	for _, notification := range notifications {
		notificationParts := []string{
			"notification",
		}
		if notification.Direction != "" {
			notificationParts = append(notificationParts, "direction="+notification.Direction)
		}
		notificationParts = append(notificationParts,
			"service="+notification.ServiceUUID,
			"characteristic="+notification.CharacteristicUUID,
		)
		if notification.Role != "" {
			notificationParts = append(notificationParts, "role="+notification.Role)
		}
		if notification.Step != "" {
			notificationParts = append(notificationParts, "step="+notification.Step)
		}
		notificationParts = append(notificationParts, fmt.Sprintf("bytes=%d", notification.Bytes))
		if notification.Frame != "" {
			notificationParts = append(notificationParts, "frame="+notification.Frame)
		}
		if notification.Packet != "" {
			notificationParts = append(notificationParts, "packet="+notification.Packet)
		}
		if notification.Detail != "" {
			notificationParts = append(notificationParts, fmt.Sprintf("detail=%q", notification.Detail))
		}
		if notification.ValueHex != "" {
			notificationParts = append(notificationParts, "value_hex="+notification.ValueHex)
		}
		if notification.DecodedHex != "" {
			notificationParts = append(notificationParts, "decoded_hex="+notification.DecodedHex)
		}
		if _, err := fmt.Fprintln(stdout, strings.Join(notificationParts, " ")); err != nil {
			return fmt.Errorf("write probe notification: %w", err)
		}
	}
	return nil
}

func printProbeMetricsText(stdout io.Writer, metrics []probeMetric) error {
	for _, metric := range metrics {
		metricParts := []string{"metric", fmt.Sprintf("%s=%s", metric.Name, formatTokenValue(metric.Value))}
		if metric.Unit != "" {
			metricParts = append(metricParts, fmt.Sprintf("unit=%q", metric.Unit))
		}
		if metric.Source != "" {
			metricParts = append(metricParts, "source="+formatTokenValue(metric.Source))
		}
		if metric.Decoder != "" {
			metricParts = append(metricParts, "decoder="+formatTokenValue(metric.Decoder))
		}
		if _, err := fmt.Fprintln(stdout, strings.Join(metricParts, " ")); err != nil {
			return fmt.Errorf("write probe metric: %w", err)
		}
	}
	return nil
}

func printProbeReadingsText(stdout io.Writer, readings []probeReading) error {
	for _, reading := range readings {
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

func flushWriter(writer io.Writer) error {
	type flusher interface {
		Flush() error
	}
	if flushable, ok := writer.(flusher); ok {
		return flushable.Flush()
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

type activeProbeValue activeProbeMode

func (v *activeProbeValue) String() string {
	if v == nil {
		return string(activeProbeNone)
	}
	return string(*v)
}

func (v *activeProbeValue) Set(value string) error {
	mode := activeProbeMode(strings.ToLower(strings.TrimSpace(value)))
	switch mode {
	case activeProbeNone, activeProbeAuto, activeProbeECDH, activeProbeAuthStatus:
		*v = activeProbeValue(mode)
		return nil
	default:
		return fmt.Errorf("unsupported active probe %q", value)
	}
}

type bleTransportValue bleTransport

func (v *bleTransportValue) String() string {
	if v == nil {
		return string(bleTransportAuto)
	}
	return string(*v)
}

func (v *bleTransportValue) Set(value string) error {
	transport := bleTransport(strings.ToLower(strings.TrimSpace(value)))
	switch transport {
	case bleTransportAuto, bleTransportRFCOMM, bleTransportAlt, bleTransportBoth:
		*v = bleTransportValue(transport)
		return nil
	default:
		return fmt.Errorf("unsupported BLE transport %q", value)
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
	return (bluetoothProber{}).ProbeWithOptions(ctx, device, probeOptions{})
}

func (bluetoothProber) ProbeWithOptions(ctx context.Context, device discoveredBLEDevice, options probeOptions) (deviceProbe, error) {
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
	notifications := make(chan rawBLENotification, 64)
	var enabledNotifications []bluetooth.DeviceCharacteristic
	activeTransports := newActiveProbeTransports()
	defer func() {
		for _, characteristic := range enabledNotifications {
			_ = characteristic.EnableNotifications(nil)
		}
	}()

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
			role := inferEcoFlowCharacteristicRole(serviceUUID, characteristicUUID)
			activeTransports.Observe(serviceUUID, characteristicUUID, role, characteristic)
			if (options.ListenDuration > 0 || options.ActiveProbe != activeProbeNone) && isEcoFlowNotifyRole(role) {
				serviceUUIDCopy := serviceUUID
				characteristicUUIDCopy := characteristicUUID
				roleCopy := role
				if err := characteristic.EnableNotifications(func(buf []byte) {
					data := append([]byte(nil), buf...)
					select {
					case notifications <- rawBLENotification{
						ServiceUUID:        serviceUUIDCopy,
						CharacteristicUUID: characteristicUUIDCopy,
						Role:               roleCopy,
						Data:               data,
					}:
					default:
					}
				}); err == nil {
					enabledNotifications = append(enabledNotifications, characteristic)
				} else {
					probe.Metrics = append(probe.Metrics, probeMetric{
						Name:   "notification_enable_error",
						Value:  role + ":" + err.Error(),
						Source: "gatt",
					})
				}
			}
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
	if err := emitProbeEvent(options, probeEvent{
		Capabilities: &probe.Capabilities,
		Metrics:      append([]probeMetric(nil), probe.Metrics...),
		Readings:     append([]probeReading(nil), probe.Readings...),
	}); err != nil {
		return deviceProbe{}, err
	}
	var activeSession *activeProbeSession
	if options.ActiveProbe != activeProbeNone {
		record, _ := ecoFlowScanRecordFromDevice(device)
		var activeNotifications []probeNotification
		activeNotifications, activeSession = sendActiveProbeRequests(activeTransports.Select(options.BLETransport), options, record)
		probe.Notifications = append(probe.Notifications, activeNotifications...)
		if err := emitProbeEvent(options, probeEvent{Notifications: activeNotifications}); err != nil {
			return deviceProbe{}, err
		}
	}
	if (options.ListenDuration > 0 || options.ListenUntilCanceled) && len(enabledNotifications) > 0 {
		listenCtx := ctx
		cancel := func() {}
		if !options.ListenUntilCanceled {
			listenCtx, cancel = context.WithTimeout(ctx, options.ListenDuration)
		}
		defer cancel()
		for {
			select {
			case notification := <-notifications:
				decodedNotification, metrics := probeNotificationFromRaw(notification, options.RawNotifications)
				probe.Notifications = append(probe.Notifications, decodedNotification)
				probe.Metrics = append(probe.Metrics, metrics...)
				if err := emitProbeEvent(options, probeEvent{
					Notifications: []probeNotification{decodedNotification},
					Metrics:       metrics,
				}); err != nil {
					return deviceProbe{}, err
				}
				followUpEvent := activeSession.HandleNotificationEvent(notification, options)
				if len(followUpEvent.Notifications) > 0 || len(followUpEvent.Metrics) > 0 || len(followUpEvent.Readings) > 0 || followUpEvent.Capabilities != nil {
					probe.Notifications = append(probe.Notifications, followUpEvent.Notifications...)
					probe.Metrics = append(probe.Metrics, followUpEvent.Metrics...)
					probe.Readings = append(probe.Readings, followUpEvent.Readings...)
					if err := emitProbeEvent(options, followUpEvent); err != nil {
						return deviceProbe{}, err
					}
				}
			case <-listenCtx.Done():
				return probe, nil
			}
		}
	}
	return probe, nil
}

func emitProbeEvent(options probeOptions, event probeEvent) error {
	if options.EventSink == nil {
		return nil
	}
	return options.EventSink(event)
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

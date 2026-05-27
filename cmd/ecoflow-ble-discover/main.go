package main

import (
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
	"strings"
	"sync"
	"syscall"
	"time"

	"tinygo.org/x/bluetooth"
)

type outputFormat string

const (
	outputFormatText outputFormat = "text"
	outputFormatJSON outputFormat = "json"
)

type discoveryConfig struct {
	Duration   time.Duration
	Format     outputFormat
	IncludeAll bool
	Redact     bool
	MinRSSI    int
	NamePrefix string
}

type ecoFlowDeviceInfo struct {
	Matched      bool   `json:"matched"`
	SerialPrefix string `json:"serial_prefix,omitempty"`
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

type discoveryScanner interface {
	Scan(stop <-chan struct{}, emit func(discoveredBLEDevice)) error
}

type bluetoothScanner struct{}

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
	{Prefix: "PR21", Model: "EcoFlow DELTA 3 2000 Air", PacketFamily: "v3"},
	{Prefix: "P231", Model: "EcoFlow DELTA 3", PacketFamily: "v3"},
	{Prefix: "D361", Model: "EcoFlow DELTA 3 (1500)", PacketFamily: "v3"},
	{Prefix: "P351", Model: "EcoFlow DELTA 3 Plus", PacketFamily: "v3"},
	{Prefix: "MR51", Model: "EcoFlow DELTA Pro 3", PacketFamily: "v3"},
	{Prefix: "MR53", Model: "EcoFlow DELTA Pro 3E", PacketFamily: "v3"},
	{Prefix: "Y711", Model: "EcoFlow DELTA Pro Ultra", PacketFamily: "v3"},
	{Prefix: "R631", Model: "EcoFlow RIVER 3 Plus", PacketFamily: "v3"},
	{Prefix: "R634", Model: "EcoFlow RIVER 3 Plus (270Wh)", PacketFamily: "v3"},
	{Prefix: "R635", Model: "EcoFlow RIVER 3 Plus (Wireless)", PacketFamily: "v3"},
	{Prefix: "R651", Model: "EcoFlow RIVER 3 (245Wh)", PacketFamily: "v3"},
	{Prefix: "R653", Model: "EcoFlow RIVER 3 (230Wh)", PacketFamily: "v3"},
	{Prefix: "R654", Model: "EcoFlow RIVER 3 UPS (230Wh)", PacketFamily: "v3"},
	{Prefix: "R655", Model: "EcoFlow RIVER 3 UPS (245Wh)", PacketFamily: "v3"},
	{Prefix: "R331", Model: "EcoFlow DELTA 2", PacketFamily: "v2"},
	{Prefix: "R335", Model: "EcoFlow DELTA 2", PacketFamily: "v2"},
	{Prefix: "R351", Model: "EcoFlow DELTA 2 Max", PacketFamily: "v2"},
	{Prefix: "R354", Model: "EcoFlow DELTA 2 Max", PacketFamily: "v2"},
	{Prefix: "P341", Model: "EcoFlow DELTA 2 Max S", PacketFamily: "v2"},
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := runDiscovery(ctx, os.Args[1:], os.Stdout, os.Stderr, bluetoothScanner{}); err != nil {
		fmt.Fprintf(os.Stderr, "ecoflow-ble-discover: %v\n", err)
		os.Exit(1)
	}
}

func runDiscovery(
	parent context.Context,
	args []string,
	stdout io.Writer,
	stderr io.Writer,
	scanner discoveryScanner,
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
	elapsed := time.Since(startedAt).Round(time.Millisecond)
	if cfg.Format == outputFormatJSON {
		if _, err := fmt.Fprintf(stdout, `{"type":"summary","seen":%d,"ecoflow":%d,"elapsed":%q}`+"\n", summary.Seen, summary.EcoFlow, elapsed.String()); err != nil {
			return fmt.Errorf("write discovery summary: %w", err)
		}
		return nil
	}
	if _, err := fmt.Fprintf(stdout, "summary seen=%d ecoflow=%d elapsed=%s\n", summary.Seen, summary.EcoFlow, elapsed); err != nil {
		return fmt.Errorf("write discovery summary: %w", err)
	}
	return nil
}

func parseDiscoveryConfig(args []string, stderr io.Writer) (discoveryConfig, error) {
	cfg := discoveryConfig{
		Duration:   15 * time.Second,
		Format:     outputFormatText,
		Redact:     true,
		MinRSSI:    -100,
		NamePrefix: "EF-",
	}
	fs := flag.NewFlagSet("ecoflow-ble-discover", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.DurationVar(&cfg.Duration, "duration", cfg.Duration, "BLE scan duration; use 0 to scan until interrupted")
	fs.Var((*outputFormatValue)(&cfg.Format), "format", "output format: text or json")
	fs.BoolVar(&cfg.IncludeAll, "all", cfg.IncludeAll, "include non-EcoFlow BLE advertisements")
	fs.BoolVar(&cfg.Redact, "redact", cfg.Redact, "redact BLE addresses and local names in output")
	fs.IntVar(&cfg.MinRSSI, "min-rssi", cfg.MinRSSI, "minimum RSSI to include; use 0 to disable RSSI filtering")
	fs.StringVar(&cfg.NamePrefix, "name-prefix", cfg.NamePrefix, "extra local-name prefix treated as an EcoFlow candidate")
	if err := fs.Parse(args); err != nil {
		return discoveryConfig{}, err
	}
	return cfg, nil
}

func validateDiscoveryConfig(cfg discoveryConfig) error {
	if cfg.Duration < 0 {
		return errors.New("duration must be zero or positive")
	}
	switch cfg.Format {
	case outputFormatText, outputFormatJSON:
		return nil
	default:
		return fmt.Errorf("format must be %q or %q", outputFormatText, outputFormatJSON)
	}
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
	if err := adapter.Enable(); err != nil {
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
				SerialPrefix: hint.Prefix,
				Model:        hint.Model,
				PacketFamily: hint.PacketFamily,
			}
		}
	}
	if strings.HasPrefix(strings.ToUpper(name), "EF-") || strings.Contains(strings.ToUpper(name), "ECOFLOW") {
		return ecoFlowDeviceInfo{
			Matched:      true,
			SerialPrefix: bestEffortSerialPrefix(normalized),
			Model:        "EcoFlow device",
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

func bestEffortSerialPrefix(value string) string {
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
	if display.Info.SerialPrefix != "" {
		parts = append(parts, "serial_prefix="+display.Info.SerialPrefix)
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

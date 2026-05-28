package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	defaultConfigPath    = "/etc/pulse-edge/config.yaml"
	defaultRawOutputPath = "/tmp/pulse-edge/ecoflow-ble-raw.jsonl"
	defaultHTTPTimeout   = 10 * time.Second
)

type config struct {
	Profile string                  `yaml:"profile"`
	Targets map[string]targetConfig `yaml:"targets"`
	BLE     bleConfig               `yaml:"ble"`
}

type targetConfig struct {
	BaseURL string `yaml:"base_url"`
}

type bleConfig struct {
	DiscoverBinary string   `yaml:"discover_binary"`
	Args           []string `yaml:"args"`
	RawOutputPath  string   `yaml:"raw_output_path"`
}

type rawProbeRecord struct {
	Type   string `json:"type"`
	Time   string `json:"time"`
	Device struct {
		Address   string `json:"address"`
		RSSI      int16  `json:"rssi"`
		LocalName string `json:"local_name"`
		Info      struct {
			Model        string `json:"model"`
			Prefix       string `json:"prefix"`
			PacketFamily string `json:"packet_family"`
		} `json:"ecoflow"`
	} `json:"device"`
	Event struct {
		Metrics []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
			Unit  string `json:"unit"`
		} `json:"metrics"`
	} `json:"event"`
}

type edgeClient struct {
	baseURL    string
	secret     string
	httpClient *http.Client
}

func main() {
	os.Exit(runMain(os.Args[1:], os.Stdout, os.Stderr))
}

func runMain(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("pulse-edge-collector", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", defaultConfigPath, "collector config path")
	enrollToken := fs.String("enroll-token", "", "setup token to exchange for a collector secret")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	log := slog.New(slog.NewTextHandler(stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg, err := loadConfig(*configPath, os.Getenv)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load config: %v\n", err)
		return 1
	}
	client := edgeClient{
		baseURL:    cfg.targetBaseURL(),
		secret:     strings.TrimSpace(os.Getenv("PULSE_EDGE_COLLECTOR_SECRET")),
		httpClient: &http.Client{Timeout: defaultHTTPTimeout},
	}
	if strings.TrimSpace(*enrollToken) != "" {
		secret, err := client.enroll(context.Background(), *enrollToken, collectorVersion(), hostname())
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "enroll collector: %v\n", err)
			return 1
		}
		_, _ = fmt.Fprintf(stdout, "PULSE_EDGE_COLLECTOR_SECRET=%s\n", secret)
		return 0
	}
	if client.secret == "" {
		_, _ = fmt.Fprintln(stderr, "PULSE_EDGE_COLLECTOR_SECRET is required")
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := runCollector(ctx, log, cfg, client); err != nil && !errors.Is(err, context.Canceled) {
		_, _ = fmt.Fprintf(stderr, "collector stopped: %v\n", err)
		return 1
	}
	return 0
}

func runCollector(ctx context.Context, log *slog.Logger, cfg config, client edgeClient) error {
	parentCtx := ctx
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	if err := client.heartbeat(ctx, collectorVersion(), hostname()); err != nil {
		return fmt.Errorf("initial heartbeat: %w", err)
	}
	heartbeatDone := make(chan struct{})
	go func() {
		defer close(heartbeatDone)
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := client.heartbeat(ctx, collectorVersion(), hostname()); err != nil {
					log.Warn("edge heartbeat failed", slog.String("error", err.Error()))
				}
			}
		}
	}()

	rawPath := cfg.BLE.RawOutputPath
	if rawPath == "" {
		rawPath = defaultRawOutputPath
	}
	if err := os.MkdirAll(filepath.Dir(rawPath), 0o755); err != nil {
		return fmt.Errorf("create raw output dir: %w", err)
	}
	cmd, err := startBLEDiscover(cfg.BLE, rawPath, log)
	if err != nil {
		return err
	}
	waitCh := waitBLEDiscover(cmd)
	tailCh := make(chan error, 1)
	go func() {
		tailCh <- tailRawProbeEvents(ctx, rawPath, func(record rawProbeRecord) error {
			if err := record.authError(); err != nil {
				return err
			}
			if err := client.uploadDiscovery(ctx, record); err != nil {
				log.Warn("edge discovery upload failed; dropping refresh", slog.String("error", err.Error()))
			}
			if metrics := record.metricMap(); len(metrics) > 0 {
				if err := client.uploadTelemetry(ctx, record, metrics); err != nil {
					log.Warn("edge telemetry upload failed; dropping refresh", slog.String("error", err.Error()))
				}
			}
			return nil
		})
	}()

	var tailErr error
	var waitErr error
	select {
	case tailErr = <-tailCh:
		cancel()
		waitErr = stopBLEDiscover(cmd, waitCh)
	case waitErr = <-waitCh:
		cancel()
		tailErr = <-tailCh
	case <-parentCtx.Done():
		cancel()
		waitErr = stopBLEDiscover(cmd, waitCh)
		tailErr = <-tailCh
	}
	<-heartbeatDone
	if tailErr != nil && !errors.Is(tailErr, context.Canceled) {
		return tailErr
	}
	if parentCtx.Err() != nil {
		return parentCtx.Err()
	}
	if waitErr != nil {
		return fmt.Errorf("ble discover exited: %w", waitErr)
	}
	return errors.New("ble discover exited")
}

func startBLEDiscover(cfg bleConfig, rawPath string, log *slog.Logger) (*exec.Cmd, error) {
	binary := strings.TrimSpace(cfg.DiscoverBinary)
	if binary == "" {
		binary = "ecoflow-ble-discover"
	}
	args := append([]string(nil), cfg.Args...)
	args = append(args, "-raw-output="+rawPath)
	cmd := exec.Command(binary, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start ecoflow ble discover: %w", err)
	}
	log.Info("started ecoflow ble discover", "binary", binary, "raw_output", rawPath)
	return cmd, nil
}

func waitBLEDiscover(cmd *exec.Cmd) <-chan error {
	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()
	return waitCh
}

func stopBLEDiscover(cmd *exec.Cmd, waitCh <-chan error) error {
	if cmd.Process != nil {
		_ = cmd.Process.Signal(syscall.SIGTERM)
	}
	select {
	case err := <-waitCh:
		return err
	case <-time.After(5 * time.Second):
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		return <-waitCh
	}
}

func tailRawProbeEvents(ctx context.Context, path string, handle func(rawProbeRecord) error) error {
	var offset int64
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := readNewRawProbeEvents(path, &offset, handle); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func readNewRawProbeEvents(path string, offset *int64, handle func(rawProbeRecord) error) error {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open raw probe output: %w", err)
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat raw probe output: %w", err)
	}
	if info.Size() < *offset {
		*offset = 0
	}
	if _, err := file.Seek(*offset, io.SeekStart); err != nil {
		return fmt.Errorf("seek raw probe output: %w", err)
	}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var record rawProbeRecord
		if err := json.Unmarshal(line, &record); err != nil {
			return fmt.Errorf("decode raw probe event: %w", err)
		}
		if record.Type == "probe_event" {
			if err := handle(record); err != nil {
				return err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan raw probe output: %w", err)
	}
	pos, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return fmt.Errorf("track raw probe offset: %w", err)
	}
	*offset = pos
	return nil
}

func loadConfig(path string, getenv func(string) string) (config, error) {
	cfg := config{
		Profile: "local",
		Targets: map[string]targetConfig{
			"local": {BaseURL: "http://localhost:8081"},
		},
		BLE: bleConfig{
			DiscoverBinary: "ecoflow-ble-discover",
			Args: []string{
				"-duration=20s",
				"-probe-timeout=11m",
				"-listen-duration=0",
				"-active-probe=auto",
				"-ble-transport=rfcomm",
			},
			RawOutputPath: defaultRawOutputPath,
		},
	}
	body, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return config{}, err
	}
	if len(body) > 0 {
		if err := yaml.Unmarshal(body, &cfg); err != nil {
			return config{}, err
		}
	}
	if profile := strings.TrimSpace(getenv("PULSE_EDGE_PROFILE")); profile != "" {
		cfg.Profile = profile
	}
	return cfg, nil
}

func (c config) targetBaseURL() string {
	if target, ok := c.Targets[strings.TrimSpace(c.Profile)]; ok {
		return strings.TrimRight(strings.TrimSpace(target.BaseURL), "/")
	}
	return strings.TrimRight(strings.TrimSpace(c.Targets["local"].BaseURL), "/")
}

func (c edgeClient) enroll(ctx context.Context, setupToken string, version string, hostname string) (string, error) {
	var response struct {
		CollectorSecret string `json:"collectorSecret"`
	}
	err := c.postJSON(ctx, "/api/v1/edge/enroll", map[string]any{
		"setupToken":       setupToken,
		"collectorVersion": version,
		"hostname":         hostname,
	}, &response)
	return response.CollectorSecret, err
}

func (c edgeClient) heartbeat(ctx context.Context, version string, hostname string) error {
	return c.postJSON(ctx, "/api/v1/edge/heartbeat", map[string]any{
		"collectorSecret":  c.secret,
		"collectorVersion": version,
		"hostname":         hostname,
	}, nil)
}

func (c edgeClient) uploadDiscovery(ctx context.Context, record rawProbeRecord) error {
	return c.postJSON(ctx, "/api/v1/edge/discoveries", map[string]any{
		"collectorSecret": c.secret,
		"discoveries": []map[string]any{{
			"provider":         "ecoflow",
			"transport":        "ble",
			"providerDeviceId": record.providerDeviceID(),
			"displayName":      record.Device.LocalName,
			"model":            record.Device.Info.Model,
			"address":          record.Device.Address,
			"rssiDbm":          int(record.Device.RSSI),
			"observedAtUnixMs": record.observedAtUnixMS(),
			"metadata": map[string]any{
				"prefix":        record.Device.Info.Prefix,
				"packet_family": record.Device.Info.PacketFamily,
			},
		}},
	}, nil)
}

func (c edgeClient) uploadTelemetry(ctx context.Context, record rawProbeRecord, metrics map[string]any) error {
	return c.postJSON(ctx, "/api/v1/edge/telemetry", map[string]any{
		"collectorSecret": c.secret,
		"samples": []map[string]any{{
			"provider":         "ecoflow",
			"transport":        "ble",
			"providerDeviceId": record.providerDeviceID(),
			"observedAtUnixMs": record.observedAtUnixMS(),
			"metrics":          metrics,
		}},
	}, nil)
}

func (c edgeClient) postJSON(ctx context.Context, endpoint string, payload any, response any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+endpoint, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
		} else {
			err = decodeHTTPResponse(endpoint, resp, response)
			if err == nil {
				return nil
			}
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 200 * time.Millisecond):
		}
	}
	return lastErr
}

func decodeHTTPResponse(endpoint string, resp *http.Response, response any) error {
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		limited, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("%s returned HTTP %d: %s", endpoint, resp.StatusCode, strings.TrimSpace(string(limited)))
	}
	if response == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(response)
}

func (r rawProbeRecord) providerDeviceID() string {
	if strings.TrimSpace(r.Device.LocalName) != "" {
		return strings.ToUpper(strings.TrimSpace(r.Device.LocalName))
	}
	return strings.ToUpper(strings.TrimSpace(r.Device.Info.Prefix))
}

func (r rawProbeRecord) observedAtUnixMS() int64 {
	parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(r.Time))
	if err != nil {
		return time.Now().UTC().UnixMilli()
	}
	return parsed.UTC().UnixMilli()
}

func (r rawProbeRecord) metricMap() map[string]any {
	out := make(map[string]any, len(r.Event.Metrics))
	for _, metric := range r.Event.Metrics {
		name := strings.TrimSpace(metric.Name)
		if name == "" || name == "auth_result" {
			continue
		}
		value := strings.TrimSpace(metric.Value)
		if value == "" {
			continue
		}
		if parsed, err := strconv.ParseFloat(value, 64); err == nil {
			out[name] = parsed
			continue
		}
		if parsed, err := strconv.ParseBool(value); err == nil {
			out[name] = parsed
			continue
		}
		out[name] = value
	}
	return out
}

func (r rawProbeRecord) authError() error {
	for _, metric := range r.Event.Metrics {
		if metric.Name != "auth_result" {
			continue
		}
		result := strings.TrimSpace(metric.Value)
		if strings.EqualFold(result, "ok") {
			return nil
		}
		if result == "" {
			result = "empty"
		}
		return fmt.Errorf("BLE authentication failed: %s", result)
	}
	return nil
}

func collectorVersion() string {
	return "pulse-edge-collector/dev"
}

func hostname() string {
	name, err := os.Hostname()
	if err != nil {
		return ""
	}
	return name
}

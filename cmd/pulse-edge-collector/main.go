package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"math"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	edgev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/edge/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/edgecollector"
	"github.com/jpaljasma/ecoflow-pulse/internal/edgefiles"
	pulselog "github.com/jpaljasma/ecoflow-pulse/pkg/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/structpb"
	"gopkg.in/yaml.v3"
)

const (
	defaultConfigPath     = "/etc/pulse-edge/config.yaml"
	defaultRawOutputPath  = "/tmp/pulse-edge/ecoflow-ble-raw.jsonl"
	defaultHTTPTimeout    = 10 * time.Second
	rawScannerMaxBytes    = 4 * 1024 * 1024
	bleRestartBackoffMin  = 1 * time.Second
	bleRestartBackoffMax  = 30 * time.Second
	bleAuthExitCode       = 10
	edgePostAttempts      = 3
	edgePostRetryDelay    = 200 * time.Millisecond
	defaultEdgeGRPCAddr   = "127.0.0.1:19090"
	defaultStartupRetry   = 5 * time.Second
	defaultOutboxMaxAge   = 168 * time.Hour
	defaultOutboxMaxBytes = 2 * 1024 * 1024 * 1024
	outboxPruneInterval   = time.Minute
	defaultPi5GOMAXPROCS  = 4
	defaultPi5Memory      = 512 * 1024 * 1024
	defaultPi5GCPercent   = 100
)

var errCorruptOutboxEntry = errors.New("corrupt outbox entry")

type edgeTransport string

const (
	edgeTransportREST edgeTransport = "rest"
	edgeTransportGRPC edgeTransport = "grpc"
)

const (
	edgeOutboxKindDiscovery = "discovery"
	edgeOutboxKindTelemetry = "telemetry"
	edgeProviderEcoFlow     = "ecoflow"
	edgeTransportBLE        = "ble"
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
	transport         edgeTransport
	baseURL           string
	secret            string
	httpClient        *http.Client
	grpcClient        edgev1.EdgeIngestServiceClient
	startupWait       time.Duration
	startupRetryDelay time.Duration
	outbox            *edgeOutbox
}

type edgeEnrollment struct {
	CollectorSecret string
	CollectorEnv    map[string]string
}

type runtimeDefaults struct {
	GOMAXPROCS  int
	MemoryLimit int64
	GCPercent   int
}

type edgeTransportConfig struct {
	transport         edgeTransport
	grpcAddr          string
	startupWait       time.Duration
	startupRetryDelay time.Duration
}

type edgeOutboxConfig struct {
	Dir      string
	MaxAge   time.Duration
	MaxBytes int64
}

type edgeOutbox struct {
	flushMu     sync.Mutex
	mu          sync.Mutex
	dir         string
	maxAge      time.Duration
	maxBytes    int64
	now         func() time.Time
	usedBytes   int64
	usageLoaded bool
	nextPruneAt time.Time
}

type edgeOutboxEntry struct {
	ID              string               `json:"id"`
	Kind            string               `json:"kind"`
	CreatedAtUnixMS int64                `json:"created_at_unix_ms"`
	Discovery       *edgeOutboxDiscovery `json:"discovery,omitempty"`
	Telemetry       *edgeOutboxTelemetry `json:"telemetry,omitempty"`
}

type edgeOutboxDiscovery struct {
	Provider         string         `json:"provider"`
	Transport        string         `json:"transport"`
	ProviderDeviceID string         `json:"provider_device_id"`
	DisplayName      string         `json:"display_name,omitempty"`
	Model            string         `json:"model,omitempty"`
	Address          string         `json:"address,omitempty"`
	RSSIDbm          int32          `json:"rssi_dbm,omitempty"`
	ObservedAtUnixMS int64          `json:"observed_at_unix_ms,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}

type edgeOutboxTelemetry struct {
	Provider         string         `json:"provider"`
	Transport        string         `json:"transport"`
	ProviderDeviceID string         `json:"provider_device_id"`
	ObservedAtUnixMS int64          `json:"observed_at_unix_ms,omitempty"`
	Metrics          map[string]any `json:"metrics"`
	ClientSampleID   string         `json:"client_sample_id"`
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

	logCfg := pulselog.DefaultServiceConfig("pulse-edge-collector")
	logCfg.Out = stderr
	logCfg.Level = pulselog.ParseLevel(os.Getenv("LOG_LEVEL"), slog.LevelInfo)
	log, asyncLogHandler, err := pulselog.BuildServiceLogger(logCfg)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "init logger: %v\n", err)
		return 1
	}
	defer func() {
		if asyncLogHandler != nil {
			asyncLogHandler.Close()
		}
	}()
	appliedRuntime := applyRuntimeDefaults(os.Getenv)
	log.Info("pulse edge runtime configured",
		slog.Int("gomaxprocs", runtime.GOMAXPROCS(0)),
		slog.Int64("memory_limit_bytes", debug.SetMemoryLimit(-1)),
		slog.Int("default_gc_percent", appliedRuntime.GCPercent),
	)
	cfg, err := loadConfig(*configPath, os.Getenv)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load config: %v\n", err)
		return 1
	}
	transportCfg, err := edgeTransportConfigFromEnv(os.Getenv)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load transport config: %v\n", err)
		return 1
	}
	outboxCfg, err := edgeOutboxConfigFromEnv(os.Getenv)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load outbox config: %v\n", err)
		return 1
	}
	client := edgeClient{
		transport:         transportCfg.transport,
		baseURL:           cfg.targetBaseURL(),
		secret:            strings.TrimSpace(os.Getenv("PULSE_EDGE_COLLECTOR_SECRET")),
		httpClient:        newEdgeHTTPClient(),
		startupWait:       transportCfg.startupWait,
		startupRetryDelay: transportCfg.startupRetryDelay,
	}
	if strings.TrimSpace(outboxCfg.Dir) != "" {
		outbox, err := newEdgeOutbox(outboxCfg)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "create edge outbox: %v\n", err)
			return 1
		}
		client.outbox = outbox
	}
	var grpcConn *grpc.ClientConn
	if transportCfg.transport == edgeTransportGRPC {
		grpcConn, err = newEdgeGRPCConn(transportCfg.grpcAddr)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "create edge gRPC client: %v\n", err)
			return 1
		}
		defer func() { _ = grpcConn.Close() }()
		client.grpcClient = edgev1.NewEdgeIngestServiceClient(grpcConn)
	}
	if strings.TrimSpace(*enrollToken) != "" {
		enrollment, err := client.enroll(context.Background(), *enrollToken, collectorVersion(), hostname())
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "enroll collector: %v\n", err)
			return 1
		}
		writeEnrollmentEnv(stdout, enrollment)
		return 0
	}
	if client.secret == "" {
		_, _ = fmt.Fprintln(stderr, "PULSE_EDGE_COLLECTOR_SECRET is required")
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := runCollector(ctx, log, cfg, client); err != nil {
		if errors.Is(err, context.Canceled) {
			return 0
		}
		_, _ = fmt.Fprintf(stderr, "collector stopped: %v\n", err)
		return exitCodeForCollectorError(err)
	}
	return 0
}

func applyRuntimeDefaults(getenv func(string) string) runtimeDefaults {
	defaults := runtimeDefaultsFor(getenv, runtime.NumCPU())
	if defaults.GOMAXPROCS > 0 {
		runtime.GOMAXPROCS(defaults.GOMAXPROCS)
	}
	if defaults.MemoryLimit > 0 {
		debug.SetMemoryLimit(defaults.MemoryLimit)
	}
	if defaults.GCPercent >= 0 {
		debug.SetGCPercent(defaults.GCPercent)
	}
	return defaults
}

func runtimeDefaultsFor(getenv func(string) string, cpuCount int) runtimeDefaults {
	defaults := runtimeDefaults{MemoryLimit: -1, GCPercent: -1}
	if strings.TrimSpace(getenv("GOMAXPROCS")) == "" {
		if cpuCount <= 0 {
			cpuCount = 1
		}
		defaults.GOMAXPROCS = minInt(cpuCount, defaultPi5GOMAXPROCS)
	}
	if strings.TrimSpace(getenv("GOMEMLIMIT")) == "" {
		defaults.MemoryLimit = defaultPi5Memory
	}
	if strings.TrimSpace(getenv("GOGC")) == "" {
		defaults.GCPercent = defaultPi5GCPercent
	}
	return defaults
}

func edgeTransportConfigFromEnv(getenv func(string) string) (edgeTransportConfig, error) {
	transportValue := strings.ToLower(strings.TrimSpace(getenv("PULSE_EDGE_TRANSPORT")))
	if transportValue == "" {
		transportValue = string(edgeTransportREST)
	}
	grpcAddr := strings.TrimSpace(getenv("PULSE_EDGE_GRPC_ADDR"))
	if grpcAddr == "" {
		grpcAddr = defaultEdgeGRPCAddr
	}
	switch edgeTransport(transportValue) {
	case edgeTransportREST:
		cfg := edgeTransportConfig{transport: edgeTransportREST, grpcAddr: grpcAddr}
		if err := edgeStartupWaitConfigFromEnv(getenv, &cfg); err != nil {
			return edgeTransportConfig{}, err
		}
		return cfg, nil
	case edgeTransportGRPC:
		cfg := edgeTransportConfig{transport: edgeTransportGRPC, grpcAddr: grpcAddr}
		if !isLoopbackAddr(cfg.grpcAddr) {
			return edgeTransportConfig{}, fmt.Errorf("PULSE_EDGE_GRPC_ADDR must be loopback when using insecure gRPC transport: %s", cfg.grpcAddr)
		}
		if err := edgeStartupWaitConfigFromEnv(getenv, &cfg); err != nil {
			return edgeTransportConfig{}, err
		}
		return cfg, nil
	default:
		return edgeTransportConfig{}, fmt.Errorf("unsupported PULSE_EDGE_TRANSPORT %q", transportValue)
	}
}

func isLoopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(strings.TrimSpace(addr))
	if err != nil {
		host = strings.TrimSpace(addr)
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func edgeStartupWaitConfigFromEnv(getenv func(string) string, cfg *edgeTransportConfig) error {
	startupWait, err := optionalDurationFromEnv(getenv, "PULSE_EDGE_STARTUP_WAIT")
	if err != nil {
		return err
	}
	startupRetry, err := optionalDurationFromEnv(getenv, "PULSE_EDGE_STARTUP_RETRY_DELAY")
	if err != nil {
		return err
	}
	if startupRetry == 0 {
		startupRetry = defaultStartupRetry
	}
	cfg.startupWait = startupWait
	cfg.startupRetryDelay = startupRetry
	return nil
}

func optionalDurationFromEnv(getenv func(string) string, key string) (time.Duration, error) {
	value := strings.TrimSpace(getenv(key))
	if value == "" {
		return 0, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	if duration < 0 {
		return 0, fmt.Errorf("%s must be non-negative", key)
	}
	return duration, nil
}

func edgeOutboxConfigFromEnv(getenv func(string) string) (edgeOutboxConfig, error) {
	dir := strings.TrimSpace(getenv("PULSE_EDGE_OUTBOX_DIR"))
	if dir == "" {
		return edgeOutboxConfig{}, nil
	}
	maxAge, err := optionalDurationFromEnv(getenv, "PULSE_EDGE_OUTBOX_MAX_AGE")
	if err != nil {
		return edgeOutboxConfig{}, err
	}
	if maxAge == 0 {
		maxAge = defaultOutboxMaxAge
	}
	maxBytes, err := optionalByteSizeFromEnv(getenv, "PULSE_EDGE_OUTBOX_MAX_BYTES")
	if err != nil {
		return edgeOutboxConfig{}, err
	}
	if maxBytes == 0 {
		maxBytes = defaultOutboxMaxBytes
	}
	return edgeOutboxConfig{Dir: dir, MaxAge: maxAge, MaxBytes: maxBytes}, nil
}

func optionalByteSizeFromEnv(getenv func(string) string, key string) (int64, error) {
	value := strings.TrimSpace(getenv(key))
	if value == "" {
		return 0, nil
	}
	size, err := parseByteSize(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	if size < 0 {
		return 0, fmt.Errorf("%s must be non-negative", key)
	}
	return size, nil
}

func parseByteSize(value string) (int64, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return 0, nil
	}
	multiplier := int64(1)
	for _, suffix := range []struct {
		text       string
		multiplier int64
	}{
		{"GiB", 1024 * 1024 * 1024},
		{"MiB", 1024 * 1024},
		{"KiB", 1024},
		{"GB", 1000 * 1000 * 1000},
		{"MB", 1000 * 1000},
		{"KB", 1000},
		{"B", 1},
	} {
		if strings.HasSuffix(normalized, suffix.text) {
			multiplier = suffix.multiplier
			normalized = strings.TrimSpace(strings.TrimSuffix(normalized, suffix.text))
			break
		}
	}
	base, err := strconv.ParseInt(normalized, 10, 64)
	if err != nil {
		return 0, err
	}
	if base > math.MaxInt64/multiplier {
		return 0, errors.New("byte size overflows int64")
	}
	return base * multiplier, nil
}

func newEdgeGRPCConn(addr string) (*grpc.ClientConn, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		addr = defaultEdgeGRPCAddr
	}
	return grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
}

func newEdgeHTTPClient() *http.Client {
	return &http.Client{
		Timeout: defaultHTTPTimeout,
		Transport: &http.Transport{
			Proxy:             http.ProxyFromEnvironment,
			ForceAttemptHTTP2: true,
			DialContext: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:          8,
			MaxIdleConnsPerHost:   4,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   5 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
}

func newEdgeOutbox(cfg edgeOutboxConfig) (*edgeOutbox, error) {
	dir := strings.TrimSpace(cfg.Dir)
	if dir == "" {
		return nil, errors.New("outbox dir is required")
	}
	if cfg.MaxAge <= 0 {
		cfg.MaxAge = defaultOutboxMaxAge
	}
	if cfg.MaxBytes <= 0 {
		cfg.MaxBytes = defaultOutboxMaxBytes
	}
	dir, err := edgefiles.PreparePrivateDirectory(dir)
	if err != nil {
		return nil, fmt.Errorf("prepare edge outbox dir: %w", err)
	}
	return &edgeOutbox{
		dir:      dir,
		maxAge:   cfg.MaxAge,
		maxBytes: cfg.MaxBytes,
		now:      time.Now,
	}, nil
}

func (o *edgeOutbox) enqueueAndFlush(ctx context.Context, entry edgeOutboxEntry, send func(context.Context, edgeOutboxEntry) error) error {
	if o == nil {
		return errors.New("edge outbox is not configured")
	}
	o.mu.Lock()
	queued, err := o.enqueueLocked(entry)
	o.mu.Unlock()
	if err != nil {
		return err
	}
	if err := o.flushPath(ctx, o.pathForID(queued.ID), send); err != nil {
		return fmt.Errorf("flush queued edge outbox entry: %w", err)
	}
	return nil
}

func (o *edgeOutbox) enqueue(entry edgeOutboxEntry) (edgeOutboxEntry, error) {
	if o == nil {
		return edgeOutboxEntry{}, errors.New("edge outbox is not configured")
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.enqueueLocked(entry)
}

func (o *edgeOutbox) enqueueLocked(entry edgeOutboxEntry) (edgeOutboxEntry, error) {
	if strings.TrimSpace(entry.ID) == "" {
		return edgeOutboxEntry{}, errors.New("outbox entry id is required")
	}
	if strings.TrimSpace(entry.Kind) == "" {
		return edgeOutboxEntry{}, errors.New("outbox entry kind is required")
	}
	if entry.CreatedAtUnixMS <= 0 {
		entry.CreatedAtUnixMS = o.now().UTC().UnixMilli()
	}
	if err := o.maybePruneExpiredLocked(); err != nil {
		return edgeOutboxEntry{}, err
	}
	body, err := json.Marshal(entry)
	if err != nil {
		return edgeOutboxEntry{}, fmt.Errorf("marshal outbox entry: %w", err)
	}
	path := o.pathForID(entry.ID)
	replacingBytes, err := fileSize(path)
	if err != nil {
		return edgeOutboxEntry{}, err
	}
	if err := o.ensureCapacityLocked(int64(len(body)), replacingBytes); err != nil {
		return edgeOutboxEntry{}, err
	}
	tmp, err := os.CreateTemp(o.dir, ".tmp-"+sanitizeOutboxID(entry.ID)+"-")
	if err != nil {
		return edgeOutboxEntry{}, fmt.Errorf("create outbox temp file: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		return edgeOutboxEntry{}, fmt.Errorf("write outbox temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return edgeOutboxEntry{}, fmt.Errorf("fsync outbox temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return edgeOutboxEntry{}, fmt.Errorf("close outbox temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return edgeOutboxEntry{}, fmt.Errorf("commit outbox entry: %w", err)
	}
	cleanup = false
	if err := fsyncDir(o.dir); err != nil {
		return edgeOutboxEntry{}, err
	}
	if o.usageLoaded {
		o.usedBytes += int64(len(body)) - replacingBytes
		if o.usedBytes < 0 {
			o.usedBytes = 0
		}
	}
	return entry, nil
}

func (o *edgeOutbox) flush(ctx context.Context, send func(context.Context, edgeOutboxEntry) error) error {
	if o == nil {
		return nil
	}
	o.flushMu.Lock()
	defer o.flushMu.Unlock()

	o.mu.Lock()
	if err := o.pruneExpiredLocked(); err != nil {
		o.mu.Unlock()
		return err
	}
	paths, err := o.pendingPathsLocked()
	o.mu.Unlock()
	if err != nil {
		return err
	}

	var corruptErr error
	for _, path := range paths {
		err := o.flushPathLocked(ctx, path, send)
		if errors.Is(err, errCorruptOutboxEntry) {
			corruptErr = errors.Join(corruptErr, err)
			continue
		}
		if err != nil {
			return err
		}
	}
	return corruptErr
}

func (o *edgeOutbox) flushPath(ctx context.Context, path string, send func(context.Context, edgeOutboxEntry) error) error {
	o.flushMu.Lock()
	defer o.flushMu.Unlock()
	return o.flushPathLocked(ctx, path, send)
}

func (o *edgeOutbox) flushPathLocked(ctx context.Context, path string, send func(context.Context, edgeOutboxEntry) error) error {
	o.mu.Lock()
	entry, body, err := readOutboxEntryBody(path)
	if errors.Is(err, errCorruptOutboxEntry) {
		quarantineErr := o.quarantineCorruptEntryLocked(path)
		o.mu.Unlock()
		if quarantineErr != nil {
			return errors.Join(err, quarantineErr)
		}
		return err
	}
	o.mu.Unlock()
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := send(ctx, entry); err != nil {
		return err
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.removeEntryIfBodyMatchesLocked(path, body)
}

func readOutboxEntry(path string) (edgeOutboxEntry, error) {
	entry, _, err := readOutboxEntryBody(path)
	return entry, err
}

func readOutboxEntryBody(path string) (edgeOutboxEntry, []byte, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return edgeOutboxEntry{}, nil, fmt.Errorf("read outbox entry: %w", err)
	}
	var entry edgeOutboxEntry
	if err := json.Unmarshal(body, &entry); err != nil {
		return edgeOutboxEntry{}, nil, fmt.Errorf("%w %s: %v", errCorruptOutboxEntry, filepath.Base(path), err)
	}
	return entry, body, nil
}

func (o *edgeOutbox) quarantineCorruptEntryLocked(path string) error {
	size, err := fileSize(path)
	if err != nil {
		return err
	}
	candidate := path + ".corrupt"
	if _, err := os.Stat(candidate); err == nil {
		candidate = fmt.Sprintf("%s.corrupt.%d", path, o.now().UTC().UnixNano())
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat corrupt outbox quarantine target: %w", err)
	}
	if err := os.Rename(path, candidate); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("quarantine corrupt outbox entry: %w", err)
	}
	o.accountRemovedBytesLocked(size)
	return fsyncDir(o.dir)
}

func (o *edgeOutbox) pendingPaths() ([]string, error) {
	if o == nil {
		return nil, nil
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.pendingPathsLocked()
}

func (o *edgeOutbox) pendingPathsLocked() ([]string, error) {
	entries, err := os.ReadDir(o.dir)
	if err != nil {
		return nil, fmt.Errorf("read edge outbox dir: %w", err)
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		paths = append(paths, filepath.Join(o.dir, entry.Name()))
	}
	sort.Strings(paths)
	return paths, nil
}

func (o *edgeOutbox) pruneExpiredLocked() error {
	err := o.refreshUsageLocked(true)
	o.nextPruneAt = o.now().UTC().Add(outboxPruneInterval)
	return err
}

func (o *edgeOutbox) maybePruneExpiredLocked() error {
	if o.maxAge <= 0 {
		return nil
	}
	now := o.now().UTC()
	if o.usageLoaded && !o.nextPruneAt.IsZero() && now.Before(o.nextPruneAt) {
		return nil
	}
	if err := o.refreshUsageLocked(true); err != nil {
		return err
	}
	o.nextPruneAt = now.Add(outboxPruneInterval)
	return nil
}

func (o *edgeOutbox) ensureCapacityLocked(newEntryBytes int64, replacingBytes int64) error {
	if o.maxBytes <= 0 {
		return nil
	}
	if !o.usageLoaded {
		if err := o.refreshUsageLocked(true); err != nil {
			return err
		}
	}
	if o.usedBytes+newEntryBytes-replacingBytes > o.maxBytes {
		if err := o.refreshUsageLocked(true); err != nil {
			return err
		}
		o.nextPruneAt = o.now().UTC().Add(outboxPruneInterval)
	}
	if o.usedBytes+newEntryBytes-replacingBytes > o.maxBytes {
		return fmt.Errorf("edge outbox size %d plus new entry %d exceeds max %d", o.usedBytes, newEntryBytes, o.maxBytes)
	}
	return nil
}

func (o *edgeOutbox) refreshUsageLocked(prune bool) error {
	var used int64
	cutoff := int64(0)
	if prune && o.maxAge > 0 {
		cutoff = o.now().UTC().Add(-o.maxAge).UnixMilli()
	}
	removed := false
	paths, err := o.pendingPathsLocked()
	if err != nil {
		return err
	}
	for _, path := range paths {
		size, err := fileSize(path)
		if err != nil {
			return err
		}
		if cutoff > 0 {
			entry, err := readOutboxEntry(path)
			if errors.Is(err, errCorruptOutboxEntry) {
				if quarantineErr := o.quarantineCorruptEntryLocked(path); quarantineErr != nil {
					return errors.Join(err, quarantineErr)
				}
				removed = true
				continue
			}
			if err != nil {
				return err
			}
			if entry.CreatedAtUnixMS > 0 && entry.CreatedAtUnixMS < cutoff {
				if err := o.removeEntryLocked(path); err != nil {
					return err
				}
				removed = true
				continue
			}
		}
		used += size
	}
	o.usedBytes = used
	o.usageLoaded = true
	if removed {
		return fsyncDir(o.dir)
	}
	return nil
}

func (o *edgeOutbox) removeEntryLocked(path string) error {
	size, err := fileSize(path)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove outbox entry: %w", err)
	}
	o.accountRemovedBytesLocked(size)
	return nil
}

func (o *edgeOutbox) removeEntryIfBodyMatchesLocked(path string, sentBody []byte) error {
	currentBody, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read outbox entry before remove: %w", err)
	}
	if !bytes.Equal(currentBody, sentBody) {
		return nil
	}
	if err := o.removeEntryLocked(path); err != nil {
		return err
	}
	return fsyncDir(o.dir)
}

func (o *edgeOutbox) accountRemovedBytesLocked(size int64) {
	if !o.usageLoaded || size <= 0 {
		return
	}
	o.usedBytes -= size
	if o.usedBytes < 0 {
		o.usedBytes = 0
	}
}

func fileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("stat outbox entry: %w", err)
	}
	return info.Size(), nil
}

func (o *edgeOutbox) pathForID(id string) string {
	return filepath.Join(o.dir, sanitizeOutboxID(id)+".json")
}

func sanitizeOutboxID(id string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(id) {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "entry"
	}
	return b.String()
}

func fsyncDir(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("open outbox dir for fsync: %w", err)
	}
	defer func() { _ = f.Close() }()
	if err := f.Sync(); err != nil {
		return fmt.Errorf("fsync outbox dir: %w", err)
	}
	return nil
}

func exitCodeForCollectorError(err error) int {
	if err == nil || errors.Is(err, context.Canceled) {
		return 0
	}
	if isBLEAuthError(err) {
		return bleAuthExitCode
	}
	return 1
}

func runCollector(ctx context.Context, log *slog.Logger, cfg config, client edgeClient) error {
	parentCtx := ctx
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	if err := client.waitForInitialHeartbeat(ctx, log, collectorVersion(), hostname()); err != nil {
		return err
	}
	if err := client.flushOutbox(ctx); err != nil {
		log.Warn("edge outbox replay failed", slog.String("error", err.Error()))
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
					continue
				}
				if err := client.flushOutbox(ctx); err != nil {
					log.Warn("edge outbox replay failed", slog.String("error", err.Error()))
				}
			}
		}
	}()

	rawPath := cfg.BLE.RawOutputPath
	if rawPath == "" {
		rawPath = defaultRawOutputPath
	}
	if err := prepareRawOutputPath(rawPath); err != nil {
		return err
	}
	log.Info("pulse edge collector started", "profile", cfg.Profile, "target", cfg.targetBaseURL(), "raw_output", rawPath)
	err := runBLEProbeLoop(ctx, log, func(runCtx context.Context) error {
		return runBLEProbeOnce(runCtx, log, cfg.BLE, rawPath, client)
	}, exponentialBackoff(bleRestartBackoffMin, bleRestartBackoffMax))
	cancel()
	<-heartbeatDone
	return err
}

func prepareRawOutputPath(rawPath string) error {
	_, err := edgefiles.PreparePrivateOutputPath(rawPath)
	return err
}

func (c edgeClient) flushOutbox(ctx context.Context) error {
	if c.outbox == nil {
		return nil
	}
	return c.outbox.flush(ctx, c.sendOutboxEntry)
}

func (c edgeClient) waitForInitialHeartbeat(ctx context.Context, log *slog.Logger, version string, hostname string) error {
	if c.startupWait <= 0 {
		if err := c.heartbeat(ctx, version, hostname); err != nil {
			return fmt.Errorf("initial heartbeat: %w", err)
		}
		return nil
	}
	retryDelay := c.startupRetryDelay
	if retryDelay <= 0 {
		retryDelay = defaultStartupRetry
	}
	deadlineCtx, cancel := context.WithTimeout(ctx, c.startupWait)
	defer cancel()
	var lastErr error
	for {
		if err := c.heartbeat(deadlineCtx, version, hostname); err != nil {
			lastErr = err
			log.Warn("edge initial heartbeat failed; waiting for edge API", slog.String("error", err.Error()))
		} else {
			return nil
		}
		timer := time.NewTimer(retryDelay)
		select {
		case <-deadlineCtx.Done():
			timer.Stop()
			if lastErr != nil {
				return fmt.Errorf("initial heartbeat after %s: %w", c.startupWait, lastErr)
			}
			return deadlineCtx.Err()
		case <-timer.C:
		}
	}
}

type probeRunner func(context.Context) error

type backoffForAttempt func(int) time.Duration

func runBLEProbeLoop(ctx context.Context, log *slog.Logger, run probeRunner, backoff backoffForAttempt) error {
	if backoff == nil {
		backoff = exponentialBackoff(bleRestartBackoffMin, bleRestartBackoffMax)
	}
	for attempt := 1; ; attempt++ {
		err := run(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if isBLEAuthError(err) {
			log.Error("BLE authentication failed; stopping collector", slog.String("error", err.Error()))
			return err
		}
		delay := backoff(attempt)
		log.Warn("BLE probe stopped; restarting", slog.String("error", errString(err)), slog.Duration("restart_in", delay))
		if sleepErr := sleepContext(ctx, delay); sleepErr != nil {
			return sleepErr
		}
	}
}

func runBLEProbeOnce(ctx context.Context, log *slog.Logger, cfg bleConfig, rawPath string, client edgeClient) error {
	if err := resetRawProbeOutput(rawPath); err != nil {
		return err
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	cmd, err := startBLEDiscover(cfg, rawPath, log)
	if err != nil {
		return err
	}
	waitCh := waitBLEDiscover(cmd)
	tailCh := make(chan error, 1)
	go func() {
		tailCh <- tailRawProbeEvents(runCtx, rawPath, func(record rawProbeRecord) error {
			if err := record.authError(); err != nil {
				return err
			}
			if err := client.uploadDiscovery(runCtx, record); err != nil {
				log.Warn("edge discovery upload failed", slog.String("error", err.Error()))
			}
			if metrics := record.metricMap(); len(metrics) > 0 {
				if err := client.uploadTelemetry(runCtx, record, metrics); err != nil {
					log.Warn("edge telemetry upload failed", slog.String("error", err.Error()))
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
	case <-ctx.Done():
		cancel()
		waitErr = stopBLEDiscover(cmd, waitCh)
		tailErr = <-tailCh
	}
	if tailErr != nil && !errors.Is(tailErr, context.Canceled) {
		return tailErr
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if waitErr != nil {
		return fmt.Errorf("ble discover exited: %w", waitErr)
	}
	return errors.New("ble discover exited")
}

func exponentialBackoff(minDelay time.Duration, maxDelay time.Duration) backoffForAttempt {
	return func(attempt int) time.Duration {
		if attempt <= 1 {
			return nextBackoff(0, minDelay, maxDelay)
		}
		var previous time.Duration
		for i := 0; i < attempt; i++ {
			previous = nextBackoff(previous, minDelay, maxDelay)
			if previous >= maxDelay {
				return previous
			}
		}
		return previous
	}
}

func fixedBackoff(delay time.Duration) backoffForAttempt {
	return func(int) time.Duration {
		return delay
	}
}

func nextBackoff(previous time.Duration, minDelay time.Duration, maxDelay time.Duration) time.Duration {
	if minDelay <= 0 {
		minDelay = bleRestartBackoffMin
	}
	if maxDelay < minDelay {
		maxDelay = minDelay
	}
	if previous <= 0 {
		return minDelay
	}
	next := previous * 2
	if next < previous || next > maxDelay {
		return maxDelay
	}
	return next
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func errString(err error) string {
	if err == nil {
		return "clean exit"
	}
	return err.Error()
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
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
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case err := <-waitCh:
		return err
	case <-timer.C:
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
	reader := bufio.NewReaderSize(file, 64*1024)
	nextOffset := *offset
	for {
		rawLine, complete, err := readRawProbeLine(reader)
		if err != nil {
			return fmt.Errorf("scan raw probe output: %w", err)
		}
		if !complete {
			break
		}
		nextOffset += int64(len(rawLine))
		line := bytes.TrimSpace(rawLine)
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
	*offset = nextOffset
	return nil
}

func readRawProbeLine(reader *bufio.Reader) ([]byte, bool, error) {
	var line []byte
	for {
		chunk, err := reader.ReadSlice('\n')
		if len(chunk) > 0 {
			nextLen := len(line) + len(chunk)
			if nextLen > rawScannerMaxBytes {
				return nil, false, fmt.Errorf("line exceeds %d bytes", rawScannerMaxBytes)
			}
			line = append(line, chunk...)
		}
		switch {
		case err == nil:
			return line, true, nil
		case errors.Is(err, bufio.ErrBufferFull):
			continue
		case errors.Is(err, io.EOF):
			return nil, false, nil
		default:
			return nil, false, err
		}
	}
}

func resetRawProbeOutput(path string) error {
	file, err := edgefiles.OpenPrivateOutputFile(path)
	if err != nil {
		return fmt.Errorf("reset raw probe output: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close raw probe output: %w", err)
	}
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

func (c edgeClient) enroll(ctx context.Context, setupToken string, version string, hostname string) (edgeEnrollment, error) {
	if c.transport == edgeTransportGRPC {
		if c.grpcClient == nil {
			return edgeEnrollment{}, errors.New("edge gRPC client is not configured")
		}
		callCtx, cancel := edgeCallContext(ctx)
		defer cancel()
		resp, err := c.grpcClient.EnrollCollector(callCtx, &edgev1.EnrollCollectorRequest{
			SetupToken:       setupToken,
			CollectorVersion: version,
			Hostname:         hostname,
		})
		if err != nil {
			return edgeEnrollment{}, err
		}
		return edgeEnrollment{
			CollectorSecret: resp.GetCollectorSecret(),
			CollectorEnv:    resp.GetCollectorEnv(),
		}, nil
	}
	var response struct {
		CollectorSecret string            `json:"collectorSecret"`
		CollectorEnv    map[string]string `json:"collectorEnv"`
	}
	err := c.postJSON(ctx, "/api/v1/edge/enroll", map[string]any{
		"setupToken":       setupToken,
		"collectorVersion": version,
		"hostname":         hostname,
	}, &response)
	return edgeEnrollment{
		CollectorSecret: response.CollectorSecret,
		CollectorEnv:    response.CollectorEnv,
	}, err
}

func writeEnrollmentEnv(w io.Writer, enrollment edgeEnrollment) {
	_, _ = fmt.Fprintf(w, "PULSE_EDGE_COLLECTOR_SECRET=%s\n", sanitizeCollectorEnvValue(enrollment.CollectorSecret))
	if value := sanitizeCollectorEnvValue(enrollment.CollectorEnv[edgecollector.EcoFlowBLEUserIDEnvKey]); value != "" {
		_, _ = fmt.Fprintf(w, "%s=%s\n", edgecollector.EcoFlowBLEUserIDEnvKey, value)
	}
}

func sanitizeCollectorEnvValue(value string) string {
	value = strings.TrimSpace(value)
	if strings.ContainsAny(value, "\r\n") {
		return ""
	}
	return value
}

func (c edgeClient) heartbeat(ctx context.Context, version string, hostname string) error {
	if c.transport == edgeTransportGRPC {
		if c.grpcClient == nil {
			return errors.New("edge gRPC client is not configured")
		}
		callCtx, cancel := edgeCallContext(ctx)
		defer cancel()
		_, err := c.grpcClient.Heartbeat(callCtx, &edgev1.HeartbeatRequest{
			CollectorSecret:  c.secret,
			CollectorVersion: version,
			Hostname:         hostname,
		})
		return err
	}
	return c.postJSON(ctx, "/api/v1/edge/heartbeat", map[string]any{
		"collectorSecret":  c.secret,
		"collectorVersion": version,
		"hostname":         hostname,
	}, nil)
}

func (c edgeClient) uploadDiscovery(ctx context.Context, record rawProbeRecord) error {
	entry := discoveryOutboxEntry(record)
	if c.outbox != nil {
		return c.outbox.enqueueAndFlush(ctx, entry, c.sendOutboxEntry)
	}
	return c.sendOutboxEntry(ctx, entry)
}

func (c edgeClient) uploadTelemetry(ctx context.Context, record rawProbeRecord, metrics map[string]any) error {
	entry := telemetryOutboxEntry(record, metrics)
	if c.outbox != nil {
		return c.outbox.enqueueAndFlush(ctx, entry, c.sendOutboxEntry)
	}
	return c.sendOutboxEntry(ctx, entry)
}

func discoveryOutboxEntry(record rawProbeRecord) edgeOutboxEntry {
	discovery := &edgeOutboxDiscovery{
		Provider:         edgeProviderEcoFlow,
		Transport:        edgeTransportBLE,
		ProviderDeviceID: record.providerDeviceID(),
		DisplayName:      record.Device.LocalName,
		Model:            record.Device.Info.Model,
		Address:          record.Device.Address,
		RSSIDbm:          int32(record.Device.RSSI),
		ObservedAtUnixMS: record.observedAtUnixMS(),
		Metadata: map[string]any{
			"prefix":        record.Device.Info.Prefix,
			"packet_family": record.Device.Info.PacketFamily,
		},
	}
	return edgeOutboxEntry{
		ID:              stableDiscoveryID(discovery),
		Kind:            edgeOutboxKindDiscovery,
		CreatedAtUnixMS: time.Now().UTC().UnixMilli(),
		Discovery:       discovery,
	}
}

func telemetryOutboxEntry(record rawProbeRecord, metrics map[string]any) edgeOutboxEntry {
	providerDeviceID := record.providerDeviceID()
	observedAtUnixMS := record.observedAtUnixMS()
	telemetry := &edgeOutboxTelemetry{
		Provider:         edgeProviderEcoFlow,
		Transport:        edgeTransportBLE,
		ProviderDeviceID: providerDeviceID,
		ObservedAtUnixMS: observedAtUnixMS,
		Metrics:          maps.Clone(metrics),
		ClientSampleID:   stableTelemetrySampleID(providerDeviceID, observedAtUnixMS, metrics),
	}
	return edgeOutboxEntry{
		ID:              telemetry.ClientSampleID,
		Kind:            edgeOutboxKindTelemetry,
		CreatedAtUnixMS: time.Now().UTC().UnixMilli(),
		Telemetry:       telemetry,
	}
}

type telemetrySampleFingerprint struct {
	Metrics          map[string]any `json:"metrics"`
	ObservedAtUnixMS int64          `json:"observed_at_unix_ms"`
	Provider         string         `json:"provider"`
	ProviderDeviceID string         `json:"provider_device_id"`
	Transport        string         `json:"transport"`
}

func stableTelemetrySampleID(providerDeviceID string, observedAtUnixMS int64, metrics map[string]any) string {
	payload, _ := json.Marshal(telemetrySampleFingerprint{
		Metrics:          metrics,
		ObservedAtUnixMS: observedAtUnixMS,
		Provider:         edgeProviderEcoFlow,
		ProviderDeviceID: providerDeviceID,
		Transport:        edgeTransportBLE,
	})
	sum := sha256.Sum256(payload)
	return "edge-telemetry-" + hex.EncodeToString(sum[:])
}

func stableDiscoveryID(discovery *edgeOutboxDiscovery) string {
	payload, _ := json.Marshal(discovery)
	sum := sha256.Sum256(payload)
	return "edge-discovery-" + hex.EncodeToString(sum[:])
}

func (c edgeClient) sendOutboxEntry(ctx context.Context, entry edgeOutboxEntry) error {
	switch entry.Kind {
	case edgeOutboxKindDiscovery:
		if entry.Discovery == nil {
			return errors.New("outbox discovery payload is missing")
		}
		return c.sendDiscovery(ctx, *entry.Discovery)
	case edgeOutboxKindTelemetry:
		if entry.Telemetry == nil {
			return errors.New("outbox telemetry payload is missing")
		}
		return c.sendTelemetry(ctx, *entry.Telemetry)
	default:
		return fmt.Errorf("unsupported outbox entry kind %q", entry.Kind)
	}
}

func (c edgeClient) sendDiscovery(ctx context.Context, discovery edgeOutboxDiscovery) error {
	if c.transport == edgeTransportGRPC {
		if c.grpcClient == nil {
			return errors.New("edge gRPC client is not configured")
		}
		metadata, err := structpb.NewStruct(discovery.Metadata)
		if err != nil {
			return err
		}
		callCtx, cancel := edgeCallContext(ctx)
		defer cancel()
		_, err = c.grpcClient.UploadDiscovery(callCtx, &edgev1.UploadDiscoveryRequest{
			CollectorSecret: c.secret,
			Discoveries: []*edgev1.EdgeDiscovery{{
				Provider:         discovery.Provider,
				Transport:        discovery.Transport,
				ProviderDeviceId: discovery.ProviderDeviceID,
				DisplayName:      discovery.DisplayName,
				Model:            discovery.Model,
				Address:          discovery.Address,
				RssiDbm:          discovery.RSSIDbm,
				ObservedAtUnixMs: discovery.ObservedAtUnixMS,
				Metadata:         metadata,
			}},
		})
		return err
	}
	return c.postJSON(ctx, "/api/v1/edge/discoveries", map[string]any{
		"collectorSecret": c.secret,
		"discoveries": []map[string]any{{
			"provider":         discovery.Provider,
			"transport":        discovery.Transport,
			"providerDeviceId": discovery.ProviderDeviceID,
			"displayName":      discovery.DisplayName,
			"model":            discovery.Model,
			"address":          discovery.Address,
			"rssiDbm":          int(discovery.RSSIDbm),
			"observedAtUnixMs": discovery.ObservedAtUnixMS,
			"metadata":         discovery.Metadata,
		}},
	}, nil)
}

func (c edgeClient) sendTelemetry(ctx context.Context, telemetry edgeOutboxTelemetry) error {
	if c.transport == edgeTransportGRPC {
		if c.grpcClient == nil {
			return errors.New("edge gRPC client is not configured")
		}
		metricsStruct, err := structpb.NewStruct(telemetry.Metrics)
		if err != nil {
			return err
		}
		callCtx, cancel := edgeCallContext(ctx)
		defer cancel()
		_, err = c.grpcClient.UploadTelemetryBatch(callCtx, &edgev1.UploadTelemetryBatchRequest{
			CollectorSecret: c.secret,
			Samples: []*edgev1.EdgeTelemetrySample{{
				Provider:         telemetry.Provider,
				Transport:        telemetry.Transport,
				ProviderDeviceId: telemetry.ProviderDeviceID,
				ObservedAtUnixMs: telemetry.ObservedAtUnixMS,
				Metrics:          metricsStruct,
				ClientSampleId:   telemetry.ClientSampleID,
			}},
		})
		return err
	}
	return c.postJSON(ctx, "/api/v1/edge/telemetry", map[string]any{
		"collectorSecret": c.secret,
		"samples": []map[string]any{{
			"provider":         telemetry.Provider,
			"transport":        telemetry.Transport,
			"providerDeviceId": telemetry.ProviderDeviceID,
			"observedAtUnixMs": telemetry.ObservedAtUnixMS,
			"clientSampleId":   telemetry.ClientSampleID,
			"metrics":          telemetry.Metrics,
		}},
	}, nil)
}

func edgeCallContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, defaultHTTPTimeout)
}

func (c edgeClient) postJSON(ctx context.Context, endpoint string, payload any, response any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	var lastErr error
	for attempt := 0; attempt < edgePostAttempts; attempt++ {
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
			if !isRetryablePostError(err) {
				return err
			}
			lastErr = err
		}
		if attempt == edgePostAttempts-1 {
			break
		}
		if err := sleepContext(ctx, time.Duration(attempt+1)*edgePostRetryDelay); err != nil {
			return err
		}
	}
	return lastErr
}

func decodeHTTPResponse(endpoint string, resp *http.Response, response any) error {
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		limited, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return httpStatusError{
			endpoint:   endpoint,
			statusCode: resp.StatusCode,
			body:       strings.TrimSpace(string(limited)),
		}
	}
	if response == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(response)
}

type httpStatusError struct {
	endpoint   string
	statusCode int
	body       string
}

func (e httpStatusError) Error() string {
	return fmt.Sprintf("%s returned HTTP %d: %s", e.endpoint, e.statusCode, e.body)
}

func isRetryablePostError(err error) bool {
	var statusErr httpStatusError
	if !errors.As(err, &statusErr) {
		return true
	}
	return statusErr.statusCode == http.StatusTooManyRequests || statusErr.statusCode >= http.StatusInternalServerError
}

func (r rawProbeRecord) providerDeviceID() string {
	serial := strings.TrimSpace(r.metricValue("manufacturer_serial"))
	if serial != "" {
		return strings.ToUpper(serial)
	}
	localName := strings.TrimSpace(r.Device.LocalName)
	if localName != "" {
		return strings.ToUpper(localName)
	}
	return strings.ToUpper(strings.TrimSpace(r.Device.Info.Prefix))
}

func (r rawProbeRecord) metricValue(name string) string {
	for _, metric := range r.Event.Metrics {
		if strings.TrimSpace(metric.Name) == name {
			return strings.TrimSpace(metric.Value)
		}
	}
	return ""
}

func (r rawProbeRecord) observedAtUnixMS() int64 {
	parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(r.Time))
	if err != nil {
		return time.Now().UTC().UnixMilli()
	}
	return parsed.UTC().UnixMilli()
}

func (r rawProbeRecord) metricMap() map[string]any {
	var out map[string]any
	for _, metric := range r.Event.Metrics {
		name := strings.TrimSpace(metric.Name)
		if name == "" || name == "auth_result" || name == "manufacturer_serial" {
			continue
		}
		value := strings.TrimSpace(metric.Value)
		if value == "" {
			continue
		}
		if out == nil {
			out = make(map[string]any, len(r.Event.Metrics))
		}
		if parsed, err := strconv.ParseFloat(value, 64); err == nil {
			if !math.IsInf(parsed, 0) && !math.IsNaN(parsed) {
				out[name] = parsed
			}
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
		return fmtBLEAuthError(result)
	}
	return nil
}

type bleAuthError struct {
	result string
}

func (e bleAuthError) Error() string {
	return "BLE authentication failed: " + e.result
}

func fmtBLEAuthError(result string) error {
	result = strings.TrimSpace(result)
	if result == "" {
		result = "empty"
	}
	return bleAuthError{result: result}
}

func isBLEAuthError(err error) bool {
	var authErr bleAuthError
	return errors.As(err, &authErr)
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

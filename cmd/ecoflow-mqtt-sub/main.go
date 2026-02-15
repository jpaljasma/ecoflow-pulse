package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/jpaljasma/ecoflow-api-playground/pkg/ecoflow"
	"github.com/jpaljasma/ecoflow-api-playground/pkg/ecoflowmqtt"
)

const (
	defaultDeviceMatch           = "delta pro ultra"
	dcResidualInferenceMinWatts  = 3.0
	dcResidualXT150DeductRatio   = 0.7
	solarLockVoltageMinVolts     = 8.0
	solarLockCurrentMaxAmps      = 0.05
	solarLockInputMaxWatts       = 1.5
	solarPowerEstimateMinWatts   = 0.5
	solarPowerEstimateMaxWatts   = 20000.0
	idleDrawNoiseFloorWatts      = 1.0
	systemStateNetThresholdWatts = 8.0
	appShowFlagACOnMask          = int64(0x4)
	appShowFlagDCOnMask          = int64(0x2)
	defaultMinuteTableRows       = 10
	defaultMinuteHistoryBuckets  = 24 * 60
	defaultPVSmoothingSamples    = 6
	defaultPowerSmoothingSamples = 6
	defaultMQTTQueueCapacity     = 128
	passthroughMinWatts          = 20.0
	passthroughAbsToleranceWatts = 15.0
	passthroughRelTolerance      = 0.12
	solarPassthroughMinOutWatts  = 5.0
	solarPassthroughMinPVWatts   = 5.0
	solarPassthroughMaxACInWatts = 10.0
	solarPassthroughSlackWatts   = 20.0
)

type systemStateKind string

const (
	systemStateUnknown     systemStateKind = "n/a"
	systemStateCharging    systemStateKind = "charging"
	systemStateDischarging systemStateKind = "discharging"
	systemStateIdle        systemStateKind = "idle"
)

var dcOutputSuffixes = []string{
	"outUsb1Pwr", "outUsb2Pwr", "outTypec1Pwr", "outTypec2Pwr", "outAdsPwr",
	"usb1Watts", "usb2Watts", "qcUsb1Watts", "qcUsb2Watts", "typec1Watts", "typec2Watts",
	"carWatts", "wireWatts",
}

var acOutputSuffixes = []string{
	"outAcTtPwr", "outAc5p8Pwr", "outAcL11Pwr", "outAcL12Pwr", "outAcL14Pwr", "outAcL21Pwr", "outAcL22Pwr",
	"outPrPwr", "invOutWatts",
}

var acInputSuffixes = []string{
	"inAc5p8Pwr", "inAcC20Pwr",
}

// telemetryMetric is one extracted metric from MQTT quota payload.
type telemetryMetric struct {
	Key   string
	Value float64
}

// batterySOC is one extracted battery SOC entry.
type batterySOC struct {
	Label string
	SOC   float64
}

// telemetryEnvelope contains common metadata for one MQTT telemetry message.
type telemetryEnvelope struct {
	ModuleType int64
	NeedAck    int64
	ID         int64
	Time       int64
	CmdID      int64
	CmdFunc    int64
	Addr       string
	Version    string
	TypeCode   string
}

// kitInfoWattsEntry is one battery entry under params.watts for typeCode=kitInfo.
type kitInfoWattsEntry struct {
	AppState int64
	CurPower float64
	AppVer   int64
	F32Soc   float64
	Soc      int64
	AvaFlag  int64
	SN       string
	Detail   int64
	Type     int64
	LoadVer  int64
}

// kitInfoStats is an aggregate summary of kitInfo battery telemetry.
type kitInfoStats struct {
	TotalSlots       int
	AvailableSlots   int
	ActiveSlots      int
	ChargingSlots    int
	DischargingSlots int
	IdleSlots        int
	TotalCurPower    float64
	AvgSOC           float64
	MinSOC           float64
	MaxSOC           float64
}

// pdStatusSummary is a typed view of pdStatus payload metrics.
type pdStatusSummary struct {
	Soc         int64
	RemainTime  int64
	ChgDsgState int64
	ErrCode     int64
	SysVer      int64

	WattsInSum  float64
	WattsOutSum float64
	NetWatts    float64

	InvInWatts  float64
	InvOutWatts float64
	CarWatts    float64
	WireWatts   float64

	XT150Watts    map[string]float64
	PVChargeWatts map[string]float64
	PVChargeTypes map[string]int64

	ChargePowerCounters    map[string]float64
	DischargePowerCounters map[string]float64
	USBWatts               map[string]float64
	Temperatures           map[string]float64

	TotalPVInputWatts float64
	TotalXT150Watts   float64

	IcoBytes    []uint64
	BMSKitState []uint64
	Reserved    []uint64
}

type channelStats struct {
	Total         float64
	PositiveTotal float64
	NegativeTotal float64
	PositiveCount int
	NegativeCount int
}

type packSnapshot struct {
	SOC                     float64
	HasSOC                  bool
	ActSOC                  float64
	HasActSOC               bool
	TempC                   float64
	HasTemp                 bool
	PowerW                  float64
	HasPower                bool
	EnergyWh                float64
	HasEnergy               bool
	SOH                     float64
	HasSOH                  bool
	ActSOH                  float64
	HasActSOH               bool
	VoltageV                float64
	HasVoltage              bool
	TargetSOC               float64
	HasTargetSOC            bool
	MinSOC                  float64
	HasMinSOC               bool
	MaxSOC                  float64
	HasMaxSOC               bool
	DiffSOC                 float64
	HasDiffSOC              bool
	RemainCap               float64
	HasRemainCap            bool
	FullCap                 float64
	HasFullCap              bool
	DesignCap               float64
	HasDesignCap            bool
	BoardTempC              float64
	HasBoardTemp            bool
	RemainTimeRaw           int64
	MaxVolDiff              float64
	HasMaxVolDiff           bool
	Serial                  string
	PreconditioningOn       bool
	HasPreconditioning      bool
	PreconditioningStateRaw int64
	HasPreconditioningState bool
	PreconditioningEventRaw int64
	HasPreconditioningEvent bool
	PreconditioningHeatTime int64
	HasPreconditioningHeat  bool
}

type energySnapshot struct {
	DeviceSOC     float64
	HasDeviceSOC  bool
	FullEnergyWh  float64
	HasFullEnergy bool

	WattsIn     float64
	HasWattsIn  bool
	WattsOut    float64
	HasWattsOut bool

	InACWatts        float64
	HasInAC          bool
	InPVWatts        float64
	HasInPV          bool
	InPVLowWatts     float64
	HasInPVLow       bool
	InPVHighWatts    float64
	HasInPVHigh      bool
	XT150Watts       float64
	HasXT150         bool
	OutACWatts       float64
	HasOutAC         bool
	OutACL14Watts    float64
	HasOutACL14      bool
	OutDCWatts       float64
	HasOutDC         bool
	HasOutDCExplicit bool
	ACOn             bool
	HasACOn          bool
	DCOn             bool
	HasDCOn          bool
	USBOn            bool
	HasUSBOn         bool
	DC12VOn          bool
	HasDC12VOn       bool
	EVChargingOn     bool
	HasEVChargingOn  bool
	AppShowFlagRaw   int64
	HasAppShowFlag   bool
	AppShowBatteryNo int
	HasAppShowBpNum  bool
	RemainCombo      float64
	HasRemainCombo   bool
	FullCombo        float64
	HasFullCombo     bool
	C20ChgMaxWatts   float64
	HasC20ChgMax     bool
	ParaChgMaxWatts  float64
	HasParaChgMax    bool
	MaxChargeSOC     float64
	HasMaxChargeSOC  bool
	MinDischargeSOC  float64
	HasMinDischarge  bool

	RemainTimeRaw          int64
	HasRemainTime          bool
	ChargeRemainTimeRaw    int64
	HasChargeRemainTime    bool
	DischargeRemainTimeRaw int64
	HasDischargeRemainTime bool
	ChgDsgStateRaw         int64
	HasChgDsgState         bool
	MQTTQueueDepth         int
	MQTTQueueCapacity      int
	MQTTQueueDroppedOldest uint64

	Packs        map[int]*packSnapshot
	PackSNToNo   map[string]int
	KitSOC       map[string]float64
	Temperatures map[string]float64

	MainBMSVoltageV   float64
	HasMainBMSVoltage bool

	BatteryInWatts  float64
	HasBatteryIn    bool
	BatteryOutWatts float64
	HasBatteryOut   bool
	BatteryAmp      float64
	HasBatteryAmp   bool
	BatteryVolts    float64
	HasBatteryVolts bool

	SolarWeakSourceFlag bool
	HasSolarWeakSource  bool
	SolarLowFlag        bool
	HasSolarLowFlag     bool
	SolarHighFlag       bool
	HasSolarHighFlag    bool
	SolarLVVolts        float64
	HasSolarLVVolts     bool
	SolarLVAmp          float64
	HasSolarLVAmp       bool
	SolarHVVolts        float64
	HasSolarHVVolts     bool
	SolarHVAmp          float64
	HasSolarHVAmp       bool
	PVLowType           int64
	HasPVLowType        bool
	PVHighType          int64
	HasPVHighType       bool
	EMSParaVolMin       float64
	HasEMSParaVolMin    bool
	EMSParaVolMax       float64
	HasEMSParaVolMax    bool

	pvLowSmoother    *rollingAverage
	pvHighSmoother   *rollingAverage
	pvTotalSmoother  *rollingAverage
	acInSmoother     *rollingAverage
	totalInSmoother  *rollingAverage
	totalOutSmoother *rollingAverage
}

type snapshotDerived struct {
	SOCValue                string
	PacksValue              string
	TempsValue              string
	InputValue              string
	OutputValue             string
	NetValue                string
	SystemStateValue        string
	RemainValue             string
	ChargeLeftValue         string
	EstimateChargeValue     string
	EstimateDischargeValue  string
	EstimateActiveValue     string
	EstimatePowerValue      string
	EstimateConfidenceValue string
	InACValue               string
	InPVValue               string
	InPVLowValue            string
	InPVHighValue           string
	XT150InValue            string
	XT150OutValue           string
	OutACValue              string
	OutACL14Value           string
	OutDCValue              string
	BatteryInValue          string
	BatteryOutValue         string
	BatteryNetValue         string
	IdleDrawValue           string
	PVStateValue            string
	PVLowStateValue         string
	PVHighStateValue        string
	PVLowVoltsValue         string
	PVHighVoltsValue        string
	PVLowAmpsValue          string
	PVHighAmpsValue         string
	StatusACValue           string
	StatusDCValue           string
	StatusUSBValue          string
	StatusDC12VValue        string
	StatusEVValue           string
	StatusPassthroughValue  string
	StatusGroundedValue     string
	StatusSolarPassValue    string
	StatusPrecondValue      string
	ChannelsNetValue        string
	ShowFlagValue           string
	BatteryCount            string
	ComboValue              string
	C20LimitValue           string
	ParaLimitValue          string
	EMSWindowValue          string
	SocGuardrail            string
	EffectiveIn             float64
	HasEffectiveIn          bool
	EffectiveOut            float64
	HasEffectiveOut         bool
}

type minuteTableConfig struct {
	Rows            int
	NewestFirst     bool
	HistoryCapacity int
}

type minuteTelemetryBucket struct {
	MinuteStartUnix       int64
	SolarSumWatts         float64
	SolarSamples          int
	ACInSumWatts          float64
	ACInSamples           int
	ACOutSumWatts         float64
	ACOutSamples          int
	DCOutSumWatts         float64
	DCOutSamples          int
	BatteryChargeSumWatts float64
	BatteryChargeSamples  int
}

type minuteTelemetryHistory struct {
	buckets     map[int64]*minuteTelemetryBucket
	maxBuckets  int
	initialized bool
}

type rollingAverage struct {
	window int
	values []float64
	next   int
	count  int
	sum    float64
}

type startupQuotaBootstrap struct {
	QuotaKeys    int
	BatteryCount int
	MappedPacks  int
	MappedSOC    bool
	MappedRemain bool
	MappedIn     bool
	MappedOut    bool
	MappedXT150  bool
	XT150Watts   float64
}

type reconnectAttemptState struct {
	initialBackoff time.Duration
	maxBackoff     time.Duration
	currentBackoff time.Duration
	failureCount   int
}

type mqttQueueStats struct {
	droppedOldest atomic.Uint64
}

func newReconnectAttemptState(initialBackoff, maxBackoff time.Duration) *reconnectAttemptState {
	if initialBackoff <= 0 {
		initialBackoff = 500 * time.Millisecond
	}
	if maxBackoff < initialBackoff {
		maxBackoff = initialBackoff
	}
	return &reconnectAttemptState{
		initialBackoff: initialBackoff,
		maxBackoff:     maxBackoff,
		currentBackoff: initialBackoff,
	}
}

func (s *reconnectAttemptState) currentAttempt() int {
	if s == nil {
		return 1
	}
	return s.failureCount + 1
}

func (s *reconnectAttemptState) registerFailure(jitterFactor float64) (attempt int, wait time.Duration) {
	if s == nil {
		return 1, 0
	}
	attempt = s.currentAttempt()
	wait = applyJitter(s.currentBackoff, jitterFactor)
	s.failureCount++
	if s.currentBackoff < s.maxBackoff {
		s.currentBackoff *= 2
		if s.currentBackoff > s.maxBackoff {
			s.currentBackoff = s.maxBackoff
		}
	}
	return attempt, wait
}

func (s *reconnectAttemptState) reset() {
	if s == nil {
		return
	}
	s.failureCount = 0
	s.currentBackoff = s.initialBackoff
}

type mqttOutputLogger struct {
	mu   sync.Mutex
	file *os.File
}

func newMQTTOutputLogger(path string) (*mqttOutputLogger, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("mqtt log path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create mqtt log directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open mqtt log file: %w", err)
	}
	return &mqttOutputLogger{file: file}, nil
}

func (l *mqttOutputLogger) Close() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return nil
	}
	err := l.file.Close()
	l.file = nil
	return err
}

func (l *mqttOutputLogger) Printf(format string, args ...any) {
	if l == nil {
		return
	}
	line := fmt.Sprintf(format, args...)
	if !strings.HasSuffix(line, "\n") {
		line += "\n"
	}
	ts := time.Now().Format(time.RFC3339Nano)
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return
	}
	_, _ = l.file.WriteString(ts + " " + line)
}

func newEnergySnapshot() *energySnapshot {
	return &energySnapshot{
		Packs:        make(map[int]*packSnapshot),
		PackSNToNo:   make(map[string]int),
		KitSOC:       make(map[string]float64),
		Temperatures: make(map[string]float64),
	}
}

func newRollingAverage(window int) *rollingAverage {
	if window <= 1 {
		return nil
	}
	return &rollingAverage{
		window: window,
		values: make([]float64, window),
	}
}

func (r *rollingAverage) Add(value float64) {
	if r == nil || r.window <= 1 || len(r.values) == 0 {
		return
	}
	if r.count < r.window {
		r.values[r.next] = value
		r.sum += value
		r.count++
		r.next = (r.next + 1) % r.window
		return
	}
	evicted := r.values[r.next]
	r.values[r.next] = value
	r.sum += value - evicted
	r.next = (r.next + 1) % r.window
}

func (r *rollingAverage) Average() (float64, bool) {
	if r == nil || r.count == 0 {
		return 0, false
	}
	return r.sum / float64(r.count), true
}

func (s *energySnapshot) configurePVSmoothing(window int) {
	if s == nil {
		return
	}
	s.pvLowSmoother = newRollingAverage(window)
	s.pvHighSmoother = newRollingAverage(window)
	s.pvTotalSmoother = newRollingAverage(window)
}

func (s *energySnapshot) configurePowerSmoothing(window int) {
	if s == nil {
		return
	}
	s.acInSmoother = newRollingAverage(window)
	s.totalInSmoother = newRollingAverage(window)
	s.totalOutSmoother = newRollingAverage(window)
}

func newMinuteTelemetryHistory(maxBuckets int) *minuteTelemetryHistory {
	if maxBuckets <= 0 {
		maxBuckets = defaultMinuteHistoryBuckets
	}
	return &minuteTelemetryHistory{
		buckets:    make(map[int64]*minuteTelemetryBucket),
		maxBuckets: maxBuckets,
	}
}

func (h *minuteTelemetryHistory) AddSample(at time.Time, snapshot *energySnapshot) {
	if h == nil || snapshot == nil {
		return
	}
	if h.buckets == nil {
		h.buckets = make(map[int64]*minuteTelemetryBucket)
	}
	minute := at.Truncate(time.Minute).Unix()
	bucket := h.buckets[minute]
	if bucket == nil {
		bucket = &minuteTelemetryBucket{MinuteStartUnix: minute}
		h.buckets[minute] = bucket
	}

	if pvInWatts, hasPVIn := snapshot.effectivePVInputWatts(); hasPVIn {
		bucket.SolarSumWatts += pvInWatts
		bucket.SolarSamples++
	}
	if snapshot.HasInAC {
		bucket.ACInSumWatts += snapshot.InACWatts
		bucket.ACInSamples++
	}
	if snapshot.HasOutAC {
		bucket.ACOutSumWatts += snapshot.OutACWatts
		bucket.ACOutSamples++
	}
	if snapshot.HasOutDC {
		bucket.DCOutSumWatts += snapshot.OutDCWatts
		bucket.DCOutSamples++
	}
	if batteryChargeWatts, hasBatteryCharge := snapshot.effectiveBatteryChargeWatts(); hasBatteryCharge {
		bucket.BatteryChargeSumWatts += batteryChargeWatts
		bucket.BatteryChargeSamples++
	}

	h.pruneOldest()
}

func (h *minuteTelemetryHistory) pruneOldest() {
	if h == nil || h.maxBuckets <= 0 || len(h.buckets) <= h.maxBuckets {
		return
	}
	keys := make([]int64, 0, len(h.buckets))
	for minute := range h.buckets {
		keys = append(keys, minute)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	for len(keys) > h.maxBuckets {
		delete(h.buckets, keys[0])
		keys = keys[1:]
	}
}

func (h *minuteTelemetryHistory) SortedBuckets(newestFirst bool, limit int) []minuteTelemetryBucket {
	if h == nil || len(h.buckets) == 0 || limit == 0 {
		return nil
	}
	keys := make([]int64, 0, len(h.buckets))
	for minute := range h.buckets {
		keys = append(keys, minute)
	}
	sort.Slice(keys, func(i, j int) bool {
		if newestFirst {
			return keys[i] > keys[j]
		}
		return keys[i] < keys[j]
	})
	if limit > 0 && len(keys) > limit {
		keys = keys[:limit]
	}
	out := make([]minuteTelemetryBucket, 0, len(keys))
	for _, key := range keys {
		if bucket := h.buckets[key]; bucket != nil {
			out = append(out, *bucket)
		}
	}
	return out
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cfg, err := ecoflow.ConfigFromEnvironment()
	if err != nil {
		fatalf("load config: %v", err)
	}

	logger := cfg.Logging.Logger
	if logger == nil {
		logger = slog.Default()
	}
	mqttLogPath := envOrDefault("ECOFLOW_MQTT_LOG_PATH", "logs/mqtt.log")
	runLog, err := newMQTTOutputLogger(mqttLogPath)
	if err != nil {
		fatalf("init mqtt run log: %v", err)
	}
	defer func() {
		_ = runLog.Close()
	}()
	runLog.Printf("session_start")

	stopQuitListener, err := startQuitKeyListener(cancel, logger, runLog)
	if err != nil {
		logger.Warn("keyboard quit listener disabled", slog.String("error", err.Error()))
	} else {
		defer stopQuitListener()
	}

	httpClient, err := ecoflow.NewClient(cfg)
	if err != nil {
		fatalf("init ecoflow client: %v", err)
	}

	devices, _, err := httpClient.GeneralInfo().ListDevices(ctx)
	if err != nil {
		fatalf("list devices: %v", err)
	}
	if len(devices) == 0 {
		fatalf("no devices returned from EcoFlow API")
	}

	targetSN := strings.TrimSpace(os.Getenv("ECOFLOW_MQTT_SN"))
	deviceMatch := envOrDefault("ECOFLOW_MQTT_DEVICE_MATCH", defaultDeviceMatch)
	targetDevice, err := selectTargetDevice(devices, targetSN, deviceMatch)
	if err != nil {
		fatalf("select target device: %v", err)
	}
	printPayload := parseBoolEnv("ECOFLOW_MQTT_PRINT_PAYLOAD", false)
	tableView := parseBoolEnv("ECOFLOW_MQTT_TABLE_VIEW", true) && !printPayload
	logBootstrapRaw := parseBoolEnv("ECOFLOW_MQTT_LOG_BOOTSTRAP_RAW", true)
	idleReconnectAfter := mustDuration("ECOFLOW_MQTT_IDLE_RECONNECT_AFTER", 30*time.Second)
	minuteTableConfig := minuteTableConfig{
		Rows:            parsePositiveIntEnv("ECOFLOW_MQTT_MINUTE_ROWS", defaultMinuteTableRows),
		NewestFirst:     parseSortNewestFirstEnv("ECOFLOW_MQTT_MINUTE_SORT", true),
		HistoryCapacity: parsePositiveIntEnv("ECOFLOW_MQTT_MINUTE_HISTORY_BUCKETS", defaultMinuteHistoryBuckets),
	}
	snapshot := newEnergySnapshot()
	pvSmoothingSamples := parsePositiveIntEnv("ECOFLOW_MQTT_PV_SMOOTH_SAMPLES", defaultPVSmoothingSamples)
	powerSmoothingSamples := parsePositiveIntEnv("ECOFLOW_MQTT_POWER_SMOOTH_SAMPLES", pvSmoothingSamples)
	if powerSmoothingSamples <= 0 {
		powerSmoothingSamples = defaultPowerSmoothingSamples
	}
	snapshot.configurePVSmoothing(pvSmoothingSamples)
	snapshot.configurePowerSmoothing(powerSmoothingSamples)
	minuteHistory := newMinuteTelemetryHistory(minuteTableConfig.HistoryCapacity)

	bootstrap, err := bootstrapSnapshotFromDeviceQuota(ctx, httpClient.GeneralInfo(), targetDevice.SN, snapshot, runLog, logBootstrapRaw)
	if err != nil {
		logger.Warn("startup device quota bootstrap failed", slog.String("error", err.Error()))
	} else {
		logger.Debug(
			"startup device quota bootstrap applied",
			slog.String("device_sn", targetDevice.SN),
			slog.Int("quota_keys", bootstrap.QuotaKeys),
			slog.Int("battery_count", bootstrap.BatteryCount),
			slog.Int("mapped_packs", bootstrap.MappedPacks),
			slog.Bool("mapped_soc", bootstrap.MappedSOC),
			slog.Bool("mapped_remain", bootstrap.MappedRemain),
			slog.Bool("mapped_in", bootstrap.MappedIn),
			slog.Bool("mapped_out", bootstrap.MappedOut),
			slog.Bool("mapped_xt150", bootstrap.MappedXT150),
			slog.Float64("xt150_watts", bootstrap.XT150Watts),
		)
	}

	cert, _, err := httpClient.GeneralInfo().GetMQTTCertification(ctx)
	if err != nil {
		fatalf("get mqtt certification: %v", err)
	}
	if cert.URL == "" || cert.Port == "" {
		fatalf("mqtt certification missing url/port")
	}

	topic := strings.TrimSpace(os.Getenv("ECOFLOW_MQTT_TOPIC"))
	if topic == "" {
		topic = fmt.Sprintf("/open/%s/%s/quota", cert.CertificateAccount, targetDevice.SN)
	}
	address := fmt.Sprintf("%s:%s", cert.URL, cert.Port)

	subscriber, err := ecoflowmqtt.NewSubscriber(ecoflowmqtt.Config{
		Address:        address,
		Username:       cert.CertificateAccount,
		Password:       cert.CertificatePassword,
		ClientID:       buildClientID(targetDevice.SN),
		KeepAlive:      mustDuration("ECOFLOW_MQTT_KEEPALIVE", 60*time.Second),
		ConnectTimeout: mustDuration("ECOFLOW_MQTT_CONNECT_TIMEOUT", 10*time.Second),
		ReadTimeout:    mustDuration("ECOFLOW_MQTT_READ_TIMEOUT", 30*time.Second),
	})
	if err != nil {
		fatalf("init mqtt subscriber: %v", err)
	}
	defer func() {
		_ = subscriber.Close()
	}()

	if err := subscriber.Connect(ctx); err != nil {
		fatalf("connect mqtt: %v", err)
	}
	if err := subscriber.Subscribe(ctx, topic, 0); err != nil {
		fatalf("subscribe mqtt topic: %v", err)
	}
	go func() {
		<-ctx.Done()
		_ = subscriber.Close()
	}()
	runLog.Printf("mqtt_subscription_started device_sn=%s topic=%s broker=%s", targetDevice.SN, topic, address)

	logger.Debug(
		"ecoflow mqtt subscription started",
		slog.String("device_sn", targetDevice.SN),
		slog.String("device_name", targetDevice.DeviceName),
		slog.String("product_name", targetDevice.ProductName),
		slog.String("broker", address),
		slog.String("topic", topic),
	)
	mqttQueueCapacity := parsePositiveIntEnv("ECOFLOW_MQTT_QUEUE_CAPACITY", defaultMQTTQueueCapacity)
	mqttIngressQueue := make(chan ecoflowmqtt.Message, mqttQueueCapacity)
	mqttIngressStats := &mqttQueueStats{}
	snapshot.MQTTQueueCapacity = cap(mqttIngressQueue)
	snapshot.MQTTQueueDepth = 0
	snapshot.MQTTQueueDroppedOldest = 0
	if bootstrap.QuotaKeys > 0 {
		initialEnvelope := telemetryEnvelope{TypeCode: "quotaBootstrap"}
		minuteHistory.AddSample(time.Now(), snapshot)
		if tableView {
			fmt.Print(renderDashboard(targetDevice, topic, initialEnvelope, snapshot, minuteHistory, minuteTableConfig))
		} else {
			summary := snapshot.String()
			fmt.Printf("energy_summary %s\n", summary)
			runLog.Printf("energy_summary %s", summary)
		}
		runLog.Printf(
			"quota_bootstrap_applied mapped_packs=%d battery_count=%d mapped_xt150=%t xt150_watts=%.1f",
			bootstrap.MappedPacks,
			bootstrap.BatteryCount,
			bootstrap.MappedXT150,
			bootstrap.XT150Watts,
		)
	}

	reconnectState := newReconnectAttemptState(500*time.Millisecond, 15*time.Second)
	readerErrCh := make(chan error, 1)
	go func() {
		defer close(mqttIngressQueue)
		err := readMQTTIntoQueue(
			ctx,
			subscriber,
			topic,
			logger,
			runLog,
			reconnectState,
			mqttIngressQueue,
			mqttIngressStats,
			idleReconnectAfter,
		)
		if err != nil && !errors.Is(err, context.Canceled) {
			select {
			case readerErrCh <- err:
			default:
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			logger.Debug("ecoflow mqtt subscriber stopped")
			runLog.Printf("session_stop reason=context_canceled")
			return
		case err := <-readerErrCh:
			if err == nil {
				continue
			}
			runLog.Printf("fatal_reader_error error=%q", err.Error())
			fatalf("%v", err)
		case msg, ok := <-mqttIngressQueue:
			if !ok {
				select {
				case err := <-readerErrCh:
					if err != nil {
						runLog.Printf("fatal_reader_error error=%q", err.Error())
						fatalf("%v", err)
					}
				default:
				}
				if errors.Is(ctx.Err(), context.Canceled) {
					logger.Debug("ecoflow mqtt subscriber stopped")
					runLog.Printf("session_stop reason=context_canceled")
				} else {
					runLog.Printf("session_stop reason=mqtt_ingress_closed")
				}
				return
			}

			runLog.Printf("payload_raw=%s", string(msg.Payload))

			envelope, quota, err := parseTelemetryPayload(msg.Payload)
			if err != nil {
				logger.Warn(
					"mqtt payload decode failed",
					slog.String("topic", msg.Topic),
					slog.Int("payload_bytes", len(msg.Payload)),
					slog.String("error", err.Error()),
				)
				runLog.Printf("payload_decode_error error=%q payload_bytes=%d", err.Error(), len(msg.Payload))
				if printPayload {
					fmt.Printf("payload_raw=%s\n", string(msg.Payload))
				}
				continue
			}

			socValues := extractBatterySOC(quota)
			pvInputs := extractPVInput(quota)
			kitEntries, hasKit := extractKitInfoWatts(quota)
			pdStatus := pdStatusSummary{}
			hasPDStatus := false
			if isPDStatusEnvelope(envelope) {
				pdStatus, hasPDStatus = extractPDStatus(quota)
			}

			snapshot.Update(envelope, quota, kitEntries, hasKit, pdStatus, hasPDStatus)
			snapshot.MQTTQueueDepth = len(mqttIngressQueue)
			snapshot.MQTTQueueCapacity = cap(mqttIngressQueue)
			snapshot.MQTTQueueDroppedOldest = mqttIngressStats.droppedOldest.Load()
			minuteHistory.AddSample(time.Now(), snapshot)

			logger.Debug(
				"ecoflow quota telemetry",
				slog.String("topic", msg.Topic),
				slog.Int("payload_bytes", len(msg.Payload)),
				slog.String("type_code", envelope.TypeCode),
				slog.String("addr", envelope.Addr),
				slog.Int64("cmd_id", envelope.CmdID),
				slog.Int64("cmd_func", envelope.CmdFunc),
				slog.Int64("message_id", envelope.ID),
				slog.Int64("message_time", envelope.Time),
				slog.Int("quota_keys", len(quota)),
				slog.Int("battery_soc_count", len(socValues)),
				slog.Int("pv_input_count", len(pvInputs)),
				slog.Bool("has_kitinfo_watts", hasKit),
				slog.Bool("has_pd_status", hasPDStatus),
				slog.Int("queue_depth", snapshot.MQTTQueueDepth),
				slog.Int("queue_capacity", snapshot.MQTTQueueCapacity),
				slog.Uint64("queue_dropped_oldest", snapshot.MQTTQueueDroppedOldest),
			)

			if tableView {
				fmt.Print(renderDashboard(targetDevice, topic, envelope, snapshot, minuteHistory, minuteTableConfig))
			} else {
				summary := snapshot.String()
				fmt.Printf("energy_summary %s\n", summary)
				runLog.Printf("energy_summary %s", summary)
			}
			if tableView {
				runLog.Printf("energy_summary %s", snapshot.String())
			}
			if printPayload {
				fmt.Printf("payload_raw=%s\n", string(msg.Payload))
			}
		}
	}
}

func bootstrapSnapshotFromDeviceQuota(
	ctx context.Context,
	service *ecoflow.GeneralInfoService,
	sn string,
	snapshot *energySnapshot,
	runLog *mqttOutputLogger,
	logRaw bool,
) (startupQuotaBootstrap, error) {
	quota, _, err := service.GetDeviceAllQuota(ctx, sn)
	if err != nil {
		return startupQuotaBootstrap{}, err
	}
	if logRaw {
		logDeviceQuotaRaw(runLog, sn, quota)
	}
	return applyDeviceQuotaToSnapshot(snapshot, quota), nil
}

func logDeviceQuotaRaw(runLog *mqttOutputLogger, sn string, quota map[string]string) {
	if runLog == nil {
		return
	}
	runLog.Printf("quota_bootstrap_raw_begin sn=%s keys=%d", sn, len(quota))
	keys := make([]string, 0, len(quota))
	for key := range quota {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		runLog.Printf("quota_raw[%s]=%s", key, sanitizeLogLine(quota[key]))
	}
	runLog.Printf("quota_bootstrap_raw_end sn=%s keys=%d", sn, len(quota))
}

func sanitizeLogLine(value string) string {
	replacer := strings.NewReplacer("\n", "\\n", "\r", "\\r", "\t", "\\t")
	return replacer.Replace(value)
}

func applyDeviceQuotaToSnapshot(snapshot *energySnapshot, quota map[string]string) startupQuotaBootstrap {
	report := startupQuotaBootstrap{QuotaKeys: len(quota)}
	if len(quota) == 0 {
		return report
	}

	parsed := make(map[string]any, len(quota))
	for key, raw := range quota {
		parsed[key] = decodeQuotaValue(raw)
	}

	seed := map[string]any{}
	report.MappedSOC = copyNumberFromQuotaCandidates(seed, "soc", parsed,
		"soc",
		"pd.soc",
		"hs_yj751_pd_appshow_addr.soc",
		"cmsBattSoc",
		"d_addr.cmsBattSoc",
	)
	report.MappedRemain = copyNumberFromQuotaCandidates(seed, "remainTime", parsed,
		"remainTime",
		"pd.remainTime",
		"hs_yj751_pd_appshow_addr.remainTime",
		"dsgRemainTime",
		"chgRemainTime",
	)
	copyNumberFromQuotaCandidates(seed, "chgRemainTime", parsed,
		"chgRemainTime",
		"bms_emsStatus.chgRemainTime",
		"hs_yj751_bms_ems_status_addr.chgRemainTime",
	)
	copyNumberFromQuotaCandidates(seed, "dsgRemainTime", parsed,
		"dsgRemainTime",
		"bms_emsStatus.dsgRemainTime",
		"hs_yj751_bms_ems_status_addr.dsgRemainTime",
	)
	copyNumberFromQuotaCandidates(seed, "chgDsgState", parsed,
		"chgDsgState",
		"pd.chgDsgState",
		"hs_yj751_pd_appshow_addr.chgDsgState",
		"bms_emsStatus.sysChgDsgState",
		"hs_yj751_bms_ems_status_addr.sysChgDsgState",
	)
	report.MappedIn = copyNumberFromQuotaCandidates(seed, "wattsInSum", parsed,
		"wattsInSum",
		"pd.wattsInSum",
		"hs_yj751_pd_appshow_addr.wattsInSum",
		"bmsInputWatts",
		"hs_yj751_pd_backend_addr.bmsInputWatts",
	)
	report.MappedOut = copyNumberFromQuotaCandidates(seed, "wattsOutSum", parsed,
		"wattsOutSum",
		"pd.wattsOutSum",
		"hs_yj751_pd_appshow_addr.wattsOutSum",
		"bmsOutputWatts",
		"hs_yj751_pd_backend_addr.bmsOutputWatts",
	)
	copyNumberFromQuotaCandidates(seed, "bmsInputWatts", parsed,
		"bmsInputWatts",
		"hs_yj751_pd_backend_addr.bmsInputWatts",
	)
	copyNumberFromQuotaCandidates(seed, "bmsOutputWatts", parsed,
		"bmsOutputWatts",
		"hs_yj751_pd_backend_addr.bmsOutputWatts",
	)
	copyNumberFromQuotaCandidates(seed, "batAmp", parsed,
		"batAmp",
		"hs_yj751_pd_backend_addr.batAmp",
	)
	copyNumberFromQuotaCandidates(seed, "batVol", parsed,
		"batVol",
		"hs_yj751_pd_backend_addr.batVol",
	)
	copyNumberFromQuotaCandidates(seed, "inLvMpptVol", parsed,
		"inLvMpptVol",
		"hs_yj751_pd_backend_addr.inLvMpptVol",
	)
	copyNumberFromQuotaCandidates(seed, "inLvMpptAmp", parsed,
		"inLvMpptAmp",
		"hs_yj751_pd_backend_addr.inLvMpptAmp",
	)
	copyNumberFromQuotaCandidates(seed, "inHvMpptVol", parsed,
		"inHvMpptVol",
		"hs_yj751_pd_backend_addr.inHvMpptVol",
	)
	copyNumberFromQuotaCandidates(seed, "inHvMpptAmp", parsed,
		"inHvMpptAmp",
		"hs_yj751_pd_backend_addr.inHvMpptAmp",
	)
	copyNumberFromQuotaCandidates(seed, "plugInInfoPvWeakSourceFlag", parsed,
		"plugInInfoPvWeakSourceFlag",
		"d_addr.plugInInfoPvWeakSourceFlag",
	)
	copyNumberFromQuotaCandidates(seed, "plugInInfoPvLFlag", parsed,
		"plugInInfoPvLFlag",
		"d_addr.plugInInfoPvLFlag",
	)
	copyNumberFromQuotaCandidates(seed, "pv1ChargeType", parsed,
		"pv1ChargeType",
		"pd.pv1ChargeType",
		"plugInInfoPvLType",
		"d_addr.plugInInfoPvLType",
	)
	copyNumberFromQuotaCandidates(seed, "pv2ChargeType", parsed,
		"pv2ChargeType",
		"pd.pv2ChargeType",
		"plugInInfoPvHType",
		"d_addr.plugInInfoPvHType",
	)
	copyNumberFromQuotaCandidates(seed, "cfgAcEnabled", parsed,
		"cfgAcEnabled",
		"inv.cfgAcEnabled",
	)
	copyNumberFromQuotaCandidates(seed, "dcOutState", parsed,
		"dcOutState",
		"pd.dcOutState",
		"hs_yj751_pd_appshow_addr.dcOutState",
	)
	copyNumberFromQuotaCandidates(seed, "carState", parsed,
		"carState",
		"pd.carState",
		"mppt.carState",
		"hs_yj751_pd_appshow_addr.carState",
	)
	copyNumberFromQuotaCandidates(seed, "showFlag", parsed,
		"showFlag",
		"pd.showFlag",
		"hs_yj751_pd_appshow_addr.showFlag",
	)
	copyNumberFromQuotaCandidates(seed, "bpNum", parsed,
		"bpNum",
		"pd.bpNum",
		"hs_yj751_pd_appshow_addr.bpNum",
	)
	copyNumberFromQuotaCandidates(seed, "remainCombo", parsed,
		"remainCombo",
		"pd.remainCombo",
		"hs_yj751_pd_appshow_addr.remainCombo",
	)
	copyNumberFromQuotaCandidates(seed, "fullCombo", parsed,
		"fullCombo",
		"pd.fullCombo",
		"hs_yj751_pd_appshow_addr.fullCombo",
	)
	copyNumberFromQuotaCandidates(seed, "c20ChgMaxWatts", parsed,
		"c20ChgMaxWatts",
		"pd.c20ChgMaxWatts",
		"hs_yj751_pd_appshow_addr.c20ChgMaxWatts",
	)
	copyNumberFromQuotaCandidates(seed, "paraChgMaxWatts", parsed,
		"paraChgMaxWatts",
		"pd.paraChgMaxWatts",
		"hs_yj751_pd_appshow_addr.paraChgMaxWatts",
	)
	copyNumberFromQuotaCandidates(seed, "emsParaVolMin", parsed,
		"emsParaVolMin",
		"hs_yj751_pd_backend_addr.emsParaVolMin",
	)
	copyNumberFromQuotaCandidates(seed, "emsParaVolMax", parsed,
		"emsParaVolMax",
		"hs_yj751_pd_backend_addr.emsParaVolMax",
	)
	copyNumberFromQuotaCandidates(seed, "cmsMaxChgSoc", parsed,
		"cmsMaxChgSoc",
		"d_addr.cmsMaxChgSoc",
	)
	copyNumberFromQuotaCandidates(seed, "cmsMinDsgSoc", parsed,
		"cmsMinDsgSoc",
		"d_addr.cmsMinDsgSoc",
	)
	copyNumberFromQuotaCandidates(seed, "acOutFreq", parsed,
		"acOutFreq",
		"inv.acOutFreq",
		"hs_yj751_pd_backend_addr.acOutFreq",
		"hs_yj751_pd_app_set_info_addr.acOutFreq",
	)
	copyAnyFromQuotaCandidates(seed, "evChgManualCtrl", parsed,
		"evChgManualCtrl",
		"d_addr.evChgManualCtrl",
	)
	copyNumberFromQuotaCandidates(seed, "plugInInfoAcpRunState", parsed,
		"plugInInfoAcpRunState",
		"d_addr.plugInInfoAcpRunState",
	)
	copyBootstrapChannelsFromQuota(seed, parsed)
	if len(seed) > 0 {
		pdSeed, hasPDSeed := extractPDStatus(seed)
		snapshot.Update(telemetryEnvelope{TypeCode: "quotaBootstrap"}, seed, nil, false, pdSeed, hasPDSeed)
		report.MappedXT150 = snapshot.HasXT150
		report.XT150Watts = snapshot.XT150Watts
	}

	// Apply kitInfo battery slots (works for stringified arrays from quota/all).
	if raw, ok := findQuotaValueByCandidates(parsed,
		"bms_kitInfo.watts",
		"hs_yj751_bms_kitInfo_addr.watts",
		"kitInfo.watts",
	); ok {
		entries, hasKit := parseKitInfoWattsEntries(raw)
		if hasKit {
			snapshot.Update(telemetryEnvelope{TypeCode: "kitInfo"}, map[string]any{"watts": raw}, entries, true, pdStatusSummary{}, false)
		}
	}

	// Apply bpInfo array blocks.
	for key, value := range parsed {
		lower := strings.ToLower(strings.TrimSpace(key))
		if lower != "bpinfo" && !strings.HasSuffix(lower, ".bpinfo") {
			continue
		}
		snapshot.Update(telemetryEnvelope{TypeCode: "bpInfo"}, map[string]any{key: value}, nil, false, pdStatusSummary{}, false)
	}

	// Slave battery packs (supports DPU multi-pack layout such as bp1..bp5).
	reSlaveStatusA := regexp.MustCompile(`(?i)^bms_slave_bmsSlaveStatus_(\d+)\.(.+)$`)
	reSlaveStatusB := regexp.MustCompile(`(?i)^hs_yj751_bms_slave_addr\.(\d+)\.(.+)$`)
	slaveGroups := make(map[int]map[string]any)
	for key, value := range parsed {
		switch {
		case reSlaveStatusA.MatchString(key):
			m := reSlaveStatusA.FindStringSubmatch(key)
			packNo, _ := strconv.Atoi(m[1])
			if packNo <= 0 {
				continue
			}
			group := slaveGroups[packNo]
			if group == nil {
				group = make(map[string]any)
				slaveGroups[packNo] = group
			}
			group[m[2]] = value
		case reSlaveStatusB.MatchString(key):
			m := reSlaveStatusB.FindStringSubmatch(key)
			packNo, _ := strconv.Atoi(m[1])
			if packNo <= 0 {
				continue
			}
			group := slaveGroups[packNo]
			if group == nil {
				group = make(map[string]any)
				slaveGroups[packNo] = group
			}
			group[m[2]] = value
		}
	}
	for packNo, group := range slaveGroups {
		snapshot.Update(
			telemetryEnvelope{TypeCode: fmt.Sprintf("bmsSlaveStatus_%d", packNo)},
			group,
			nil,
			false,
			pdStatusSummary{},
			false,
		)
	}

	// Main/internal battery (Delta 2 Max style).
	// Apply after slave groups so canonical bmsStatus values win for bp1.
	if group := collectQuotaByPrefix(parsed, "bms_bmsStatus."); len(group) > 0 {
		snapshot.Update(telemetryEnvelope{TypeCode: "bmsStatus"}, group, nil, false, pdStatusSummary{}, false)
	}

	if rawKitNum, ok := findQuotaValueByCandidates(parsed, "bms_kitInfo.kitNum", "kitNum"); ok {
		if kitNum := int(toInt64(rawKitNum)); kitNum > report.BatteryCount {
			report.BatteryCount = kitNum
		}
	}
	if len(snapshot.Packs) > report.BatteryCount {
		report.BatteryCount = len(snapshot.Packs)
	}
	if report.BatteryCount > 0 {
		for packNo := 1; packNo <= report.BatteryCount; packNo++ {
			snapshot.ensurePack(packNo)
		}
	}
	report.MappedPacks = len(snapshot.Packs)
	return report
}

func decodeQuotaValue(raw string) any {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	first := trimmed[0]
	if first == '[' || first == '{' {
		var decoded any
		if err := json.Unmarshal([]byte(trimmed), &decoded); err == nil {
			return decoded
		}
	}
	return trimmed
}

func copyNumberFromQuotaCandidates(dst map[string]any, dstKey string, quota map[string]any, candidates ...string) bool {
	value, ok := findQuotaValueByCandidates(quota, candidates...)
	if !ok {
		return false
	}
	if _, ok := numberFromAny(value); !ok {
		return false
	}
	dst[dstKey] = value
	return true
}

func copyAnyFromQuotaCandidates(dst map[string]any, dstKey string, quota map[string]any, candidates ...string) bool {
	value, ok := findQuotaValueByCandidates(quota, candidates...)
	if !ok {
		return false
	}
	dst[dstKey] = value
	return true
}

func copyBootstrapChannelsFromQuota(dst map[string]any, quota map[string]any) {
	for key, value := range quota {
		if !shouldSeedBootstrapChannel(key, value) {
			continue
		}
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			continue
		}
		dst[trimmed] = value
	}
}

func shouldSeedBootstrapChannel(key string, value any) bool {
	if _, ok := numberFromAny(value); !ok {
		return false
	}
	lower := strings.ToLower(strings.TrimSpace(key))
	if strings.Contains(lower, "xt150watts") {
		return true
	}
	if isPVInputKey(key) {
		return true
	}
	if keyMatchesAnySuffix(key, acOutputSuffixes...) {
		return true
	}
	if keyMatchesAnySuffix(key, dcOutputSuffixes...) {
		return true
	}
	if keyMatchesAnySuffix(key, acInputSuffixes...) {
		return true
	}
	return false
}

func findQuotaValueByCandidates(quota map[string]any, candidates ...string) (any, bool) {
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if value, ok := quota[candidate]; ok {
			return value, true
		}
	}
	for _, candidate := range candidates {
		suffix := strings.ToLower(strings.TrimSpace(candidate))
		if suffix == "" {
			continue
		}
		for key, value := range quota {
			if strings.HasSuffix(strings.ToLower(strings.TrimSpace(key)), suffix) {
				return value, true
			}
		}
	}
	return nil, false
}

func collectQuotaByPrefix(quota map[string]any, prefix string) map[string]any {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return nil
	}
	out := map[string]any{}
	lowerPrefix := strings.ToLower(prefix)
	for key, value := range quota {
		if strings.HasPrefix(strings.ToLower(key), lowerPrefix) {
			field := key[len(prefix):]
			if strings.TrimSpace(field) == "" {
				continue
			}
			out[field] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func reconnectSubscriber(
	ctx context.Context,
	subscriber *ecoflowmqtt.Subscriber,
	topic string,
	logger *slog.Logger,
	runLog *mqttOutputLogger,
	state *reconnectAttemptState,
) error {
	if state == nil {
		state = newReconnectAttemptState(500*time.Millisecond, 15*time.Second)
	}
	const jitterFactor = 0.25

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		attempt := state.currentAttempt()
		_ = subscriber.Disconnect()

		connectErr := subscriber.Connect(ctx)
		if connectErr == nil {
			subscribeErr := subscriber.Subscribe(ctx, topic, 0)
			if subscribeErr == nil {
				state.reset()
				logger.Info(
					"ecoflow mqtt reconnected",
					slog.Int("attempt", attempt),
					slog.String("topic", topic),
				)
				runLog.Printf("mqtt_reconnected attempt=%d topic=%s", attempt, topic)
				return nil
			}
			connectErr = fmt.Errorf("subscribe: %w", subscribeErr)
		}

		attempt, wait := state.registerFailure(jitterFactor)
		logger.Warn(
			"ecoflow mqtt reconnect attempt failed",
			slog.Int("attempt", attempt),
			slog.String("error", connectErr.Error()),
			slog.Duration("retry_in", wait),
		)
		runLog.Printf("mqtt_reconnect_failed attempt=%d error=%q retry_in=%s", attempt, connectErr.Error(), wait.String())
		if err := sleepContext(ctx, wait); err != nil {
			return err
		}
	}
}

func enqueueMQTTMessageDropOldest(
	ctx context.Context,
	queue chan ecoflowmqtt.Message,
	message ecoflowmqtt.Message,
	stats *mqttQueueStats,
) (enqueued bool, droppedOldest bool) {
	select {
	case <-ctx.Done():
		return false, false
	case queue <- message:
		return true, false
	default:
	}

	select {
	case <-ctx.Done():
		return false, false
	case <-queue:
		droppedOldest = true
		if stats != nil {
			stats.droppedOldest.Add(1)
		}
	default:
		return false, false
	}

	select {
	case <-ctx.Done():
		return false, droppedOldest
	case queue <- message:
		return true, droppedOldest
	default:
		return false, droppedOldest
	}
}

func readMQTTIntoQueue(
	ctx context.Context,
	subscriber *ecoflowmqtt.Subscriber,
	topic string,
	logger *slog.Logger,
	runLog *mqttOutputLogger,
	reconnectState *reconnectAttemptState,
	queue chan ecoflowmqtt.Message,
	queueStats *mqttQueueStats,
	idleReconnectAfter time.Duration,
) error {
	if idleReconnectAfter <= 0 {
		idleReconnectAfter = 30 * time.Second
	}

	for {
		readCtx, readCancel := context.WithTimeout(ctx, idleReconnectAfter)
		msg, err := subscriber.ReadMessage(readCtx)
		readCancel()
		if err != nil {
			if errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
				return ctx.Err()
			}
			if errors.Is(err, context.DeadlineExceeded) {
				logger.Warn(
					"mqtt idle timeout; reconnecting",
					slog.Duration("idle_timeout", idleReconnectAfter),
				)
				runLog.Printf("mqtt_idle_timeout_reconnecting idle_timeout=%s", idleReconnectAfter.String())
				if reconnectErr := reconnectSubscriber(ctx, subscriber, topic, logger, runLog, reconnectState); reconnectErr != nil {
					return fmt.Errorf("reconnect mqtt subscriber: %w", reconnectErr)
				}
				continue
			}
			if !isReconnectableReadError(err) {
				return fmt.Errorf("read mqtt message: %w", err)
			}
			logger.Warn(
				"mqtt read failed; reconnecting",
				slog.String("error", err.Error()),
			)
			runLog.Printf("mqtt_read_error_reconnecting error=%q", err.Error())
			if reconnectErr := reconnectSubscriber(ctx, subscriber, topic, logger, runLog, reconnectState); reconnectErr != nil {
				return fmt.Errorf("reconnect mqtt subscriber: %w", reconnectErr)
			}
			continue
		}

		enqueued, dropped := enqueueMQTTMessageDropOldest(ctx, queue, msg, queueStats)
		if dropped {
			droppedCount := uint64(0)
			if queueStats != nil {
				droppedCount = queueStats.droppedOldest.Load()
			}
			if droppedCount == 1 || droppedCount%25 == 0 {
				logger.Warn(
					"mqtt ingress queue dropped oldest message",
					slog.Uint64("dropped_oldest", droppedCount),
					slog.Int("queue_depth", len(queue)),
					slog.Int("queue_capacity", cap(queue)),
				)
				runLog.Printf(
					"mqtt_queue_drop_oldest dropped_oldest=%d queue_depth=%d queue_capacity=%d",
					droppedCount,
					len(queue),
					cap(queue),
				)
			}
		}
		if !enqueued && ctx.Err() != nil {
			return ctx.Err()
		}
	}
}

func isReconnectableReadError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "eof") ||
		strings.Contains(lower, "broken pipe") ||
		strings.Contains(lower, "connection reset") ||
		strings.Contains(lower, "closed network connection")
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	if duration <= 0 {
		return nil
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func startQuitKeyListener(cancel context.CancelFunc, logger *slog.Logger, runLog *mqttOutputLogger) (func(), error) {
	stdin := os.Stdin
	if stdin == nil {
		return func() {}, errors.New("stdin is nil")
	}
	if !isCharacterDevice(stdin) {
		return func() {}, nil
	}

	restore, err := setupTerminalForSingleKeyInput(stdin)
	if err != nil {
		return func() {}, err
	}

	var stopOnce sync.Once
	stop := func() {
		stopOnce.Do(func() {
			restore()
		})
	}

	go func() {
		buf := make([]byte, 1)
		for {
			n, readErr := stdin.Read(buf)
			if readErr != nil {
				// Expected during shutdown if stdin closes or terminal detaches.
				if !errors.Is(readErr, io.EOF) && !errors.Is(readErr, os.ErrClosed) {
					logger.Debug("keyboard quit listener stopped", slog.String("error", readErr.Error()))
				}
				return
			}
			if n == 0 {
				continue
			}
			switch buf[0] {
			case 'q', 'Q':
				runLog.Printf("session_stop reason=keyboard_q")
				stop()
				cancel()
				return
			}
		}
	}()

	return stop, nil
}

func isCharacterDevice(file *os.File) bool {
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func applyJitter(base time.Duration, factor float64) time.Duration {
	if base <= 0 || factor <= 0 {
		return base
	}
	var randomByte [1]byte
	if _, err := rand.Read(randomByte[:]); err != nil {
		return base
	}
	unit := float64(randomByte[0])/255.0*2 - 1
	delta := time.Duration(float64(base) * factor * unit)
	value := base + delta
	if value < 100*time.Millisecond {
		return 100 * time.Millisecond
	}
	return value
}

func fatalf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func mustDuration(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		fatalf("invalid %s=%q: expected positive duration", key, raw)
	}
	return value
}

func parseBoolEnv(key string, fallback bool) bool {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if raw == "" {
		return fallback
	}
	switch raw {
	case "1", "true", "t", "yes", "y", "on":
		return true
	case "0", "false", "f", "no", "n", "off":
		return false
	default:
		return fallback
	}
}

func parsePositiveIntEnv(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func parseSortNewestFirstEnv(key string, fallback bool) bool {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if raw == "" {
		return fallback
	}
	switch raw {
	case "desc", "descending", "newest", "newest_first", "newest-first":
		return true
	case "asc", "ascending", "oldest", "oldest_first", "oldest-first":
		return false
	default:
		return fallback
	}
}

func buildClientID(sn string) string {
	var random [4]byte
	_, _ = rand.Read(random[:])
	suffix := hex.EncodeToString(random[:])
	cleanSN := strings.NewReplacer(" ", "", "/", "", "\\", "", ":", "").Replace(sn)
	if len(cleanSN) > 16 {
		cleanSN = cleanSN[:16]
	}
	return "ecoflow-sub-" + cleanSN + "-" + suffix
}

// selectTargetDevice chooses one device to subscribe.
// Priority: exact SN override, then case-insensitive name/product match.
func selectTargetDevice(
	devices []ecoflow.GeneralInfoDevice,
	requiredSN string,
	matchTerm string,
) (ecoflow.GeneralInfoDevice, error) {
	requiredSN = strings.TrimSpace(requiredSN)
	if requiredSN != "" {
		for _, device := range devices {
			if device.SN == requiredSN {
				return device, nil
			}
		}
		return ecoflow.GeneralInfoDevice{}, fmt.Errorf("device with sn=%s not found", requiredSN)
	}

	term := strings.ToLower(strings.TrimSpace(matchTerm))
	for _, device := range devices {
		name := strings.ToLower(device.DeviceName)
		product := strings.ToLower(device.ProductName)
		if term != "" && (strings.Contains(name, term) || strings.Contains(product, term)) {
			return device, nil
		}
	}

	if len(devices) == 1 {
		return devices[0], nil
	}

	available := make([]string, 0, len(devices))
	for _, device := range devices {
		available = append(available, fmt.Sprintf("%s(%s)", device.DeviceName, device.SN))
	}
	sort.Strings(available)
	return ecoflow.GeneralInfoDevice{}, fmt.Errorf(
		"no device matched %q; set ECOFLOW_MQTT_SN explicitly; available=%s",
		matchTerm,
		strings.Join(available, ", "),
	)
}

func parseTelemetryPayload(payload []byte) (telemetryEnvelope, map[string]any, error) {
	var root any
	if err := json.Unmarshal(payload, &root); err != nil {
		return telemetryEnvelope{}, nil, err
	}

	object, ok := root.(map[string]any)
	if !ok {
		return telemetryEnvelope{}, nil, errors.New("payload is not a JSON object")
	}

	envelope := telemetryEnvelope{
		ModuleType: toInt64(object["moduleType"]),
		NeedAck:    toInt64(object["needAck"]),
		ID:         toInt64(object["id"]),
		Time:       toInt64(object["time"]),
		CmdID:      toInt64(object["cmdId"]),
		CmdFunc:    toInt64(object["cmdFunc"]),
		Addr:       toString(object["addr"]),
		Version:    toString(object["version"]),
		TypeCode:   toString(object["typeCode"]),
	}

	params := mapFromAny(object["params"])
	param := mapFromAny(object["param"])
	data := mapFromAny(object["data"])
	quota := mapFromAny(object["quota"])

	merged := make(map[string]any, len(params)+len(param)+len(data)+len(quota))
	mergeAnyMap(merged, params)
	mergeAnyMap(merged, param)
	mergeAnyMap(merged, data)
	mergeAnyMap(merged, quota)

	if envelope.Addr != "" {
		baseKeys := make([]string, 0, len(merged))
		for key := range merged {
			baseKeys = append(baseKeys, key)
		}
		for _, key := range baseKeys {
			if strings.HasPrefix(key, envelope.Addr+".") {
				continue
			}
			prefixed := envelope.Addr + "." + key
			if _, exists := merged[prefixed]; !exists {
				merged[prefixed] = merged[key]
			}
		}
	}

	if envelope.TypeCode == "" {
		envelope.TypeCode = inferTypeCode(envelope.Addr, merged)
	}

	if len(merged) == 0 {
		return envelope, map[string]any{}, nil
	}
	return envelope, merged, nil
}

func extractBatterySOC(quota map[string]any) []batterySOC {
	keys := sortedMapKeys(quota)
	out := make([]batterySOC, 0, 8)

	for _, key := range keys {
		if !strings.HasSuffix(strings.ToLower(key), ".bpinfo") {
			continue
		}
		entries, ok := quota[key].([]any)
		if !ok {
			continue
		}
		for i, raw := range entries {
			entry, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			soc, ok := numberFromAny(entry["bpSoc"])
			if !ok {
				continue
			}
			bpNo, _ := numberFromAny(entry["bpNo"])
			label := fmt.Sprintf("%s[%d].bpSoc", key, i)
			if bpNo > 0 {
				label = fmt.Sprintf("%s.bpNo=%d", key, int(bpNo))
			}
			out = append(out, batterySOC{Label: label, SOC: soc})
		}
	}

	for _, key := range keys {
		if !strings.HasSuffix(strings.ToLower(key), ".soc") {
			continue
		}
		if !strings.Contains(strings.ToLower(key), "bms") {
			continue
		}
		soc, ok := numberFromAny(quota[key])
		if !ok {
			continue
		}
		out = append(out, batterySOC{Label: key, SOC: soc})
	}

	for _, key := range keys {
		lower := strings.ToLower(key)
		if lower != "watts" && !strings.HasSuffix(lower, "kitinfo.watts") {
			continue
		}
		entries, ok := parseKitInfoWattsEntries(quota[key])
		if !ok {
			continue
		}
		for i, entry := range entries {
			if entry.AvaFlag == 0 && entry.Soc == 0 && entry.F32Soc == 0 {
				continue
			}
			label := fmt.Sprintf("%s[%d]", key, i)
			if strings.TrimSpace(entry.SN) != "" {
				label = entry.SN
			}
			out = append(out, batterySOC{Label: label, SOC: entrySOC(entry)})
		}
	}

	return out
}

func extractPVInput(quota map[string]any) []telemetryMetric {
	keys := sortedMapKeys(quota)
	out := make([]telemetryMetric, 0, 6)
	for _, key := range keys {
		if !isPVInputKey(key) {
			continue
		}
		value, ok := numberFromAny(quota[key])
		if !ok {
			continue
		}
		out = append(out, telemetryMetric{Key: key, Value: value})
	}
	return out
}

func isPVInputKey(key string) bool {
	lower := strings.ToLower(key)
	if strings.Contains(lower, "mppt.inwatts") {
		return true
	}
	if strings.Contains(lower, "inhvmpptpwr") || strings.Contains(lower, "inlvmpptpwr") {
		return true
	}
	if strings.Contains(lower, "pv") {
		if strings.Contains(lower, "chargewatts") || strings.Contains(lower, "inwatts") || strings.HasSuffix(lower, "pwr") {
			return true
		}
	}
	return false
}

func numberFromAny(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case int32:
		return float64(v), true
	case int16:
		return float64(v), true
	case int8:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint64:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint16:
		return float64(v), true
	case uint8:
		return float64(v), true
	case json.Number:
		f, err := v.Float64()
		if err != nil {
			return 0, false
		}
		return f, true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}

func sortedMapKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func extractPDStatus(quota map[string]any) (pdStatusSummary, bool) {
	out := pdStatusSummary{
		XT150Watts:             make(map[string]float64),
		PVChargeWatts:          make(map[string]float64),
		PVChargeTypes:          make(map[string]int64),
		ChargePowerCounters:    make(map[string]float64),
		DischargePowerCounters: make(map[string]float64),
		USBWatts:               make(map[string]float64),
		Temperatures:           make(map[string]float64),
	}
	found := false

	if value, ok := quota["soc"]; ok {
		out.Soc = toInt64(value)
		found = true
	}
	if value, ok := quota["remainTime"]; ok {
		out.RemainTime = toInt64(value)
		found = true
	}
	if value, ok := quota["chgDsgState"]; ok {
		out.ChgDsgState = toInt64(value)
		found = true
	}
	if value, ok := quota["errCode"]; ok {
		out.ErrCode = toInt64(value)
		found = true
	}
	if value, ok := quota["sysVer"]; ok {
		out.SysVer = toInt64(value)
		found = true
	}

	if watts, ok := numberFromAny(quota["wattsInSum"]); ok {
		out.WattsInSum = watts
		found = true
	}
	if watts, ok := numberFromAny(quota["wattsOutSum"]); ok {
		out.WattsOutSum = watts
		found = true
	}
	out.NetWatts = out.WattsInSum - out.WattsOutSum

	if watts, ok := numberFromAny(quota["invInWatts"]); ok {
		out.InvInWatts = watts
		found = true
	}
	if watts, ok := numberFromAny(quota["invOutWatts"]); ok {
		out.InvOutWatts = watts
		found = true
	}
	if watts, ok := numberFromAny(quota["carWatts"]); ok {
		out.CarWatts = watts
		found = true
	}
	if watts, ok := numberFromAny(quota["wireWatts"]); ok {
		out.WireWatts = watts
		found = true
	}

	for key, value := range quota {
		lowerKey := strings.ToLower(key)
		if strings.Contains(lowerKey, "xt150watts") {
			watts, ok := numberFromAny(value)
			if !ok {
				continue
			}
			out.XT150Watts[key] = watts
			found = true
			continue
		}
		if strings.HasPrefix(lowerKey, "pv") && strings.HasSuffix(lowerKey, "chargewatts") {
			watts, ok := numberFromAny(value)
			if !ok {
				continue
			}
			out.PVChargeWatts[key] = watts
			found = true
			continue
		}
		if strings.HasPrefix(lowerKey, "pv") && strings.HasSuffix(lowerKey, "chargetype") {
			out.PVChargeTypes[key] = toInt64(value)
			found = true
			continue
		}
		if strings.HasPrefix(lowerKey, "chgpower") {
			watts, ok := numberFromAny(value)
			if !ok {
				continue
			}
			out.ChargePowerCounters[key] = watts
			found = true
			continue
		}
		if strings.HasPrefix(lowerKey, "dsgpower") {
			watts, ok := numberFromAny(value)
			if !ok {
				continue
			}
			out.DischargePowerCounters[key] = watts
			found = true
			continue
		}
		if strings.HasSuffix(lowerKey, "watts") &&
			(strings.HasPrefix(lowerKey, "usb") || strings.HasPrefix(lowerKey, "qcusb") || strings.HasPrefix(lowerKey, "typec")) {
			watts, ok := numberFromAny(value)
			if !ok {
				continue
			}
			out.USBWatts[key] = watts
			found = true
			continue
		}
		if strings.HasSuffix(lowerKey, "temp") &&
			(strings.HasPrefix(lowerKey, "typec") || strings.HasPrefix(lowerKey, "car")) {
			temperature, ok := numberFromAny(value)
			if !ok {
				continue
			}
			out.Temperatures[key] = temperature
			found = true
			continue
		}
	}

	if bytes, ok := parseUintArrayFromAny(quota["icoBytes"]); ok {
		out.IcoBytes = bytes
		found = true
	}
	if bytes, ok := parseUintArrayFromAny(quota["bmsKitState"]); ok {
		out.BMSKitState = bytes
		found = true
	}
	if bytes, ok := parseUintArrayFromAny(quota["reserved"]); ok {
		out.Reserved = bytes
		found = true
	}
	out.TotalPVInputWatts = sumInputChannelValues(out.PVChargeWatts)
	out.TotalXT150Watts = computeChannelStats(out.XT150Watts).Total

	return out, found
}

func extractKitInfoWatts(quota map[string]any) ([]kitInfoWattsEntry, bool) {
	keys := []string{"watts", "bms_kitInfo.watts"}
	for _, key := range keys {
		raw, ok := quota[key]
		if !ok {
			continue
		}
		entries, ok := parseKitInfoWattsEntries(raw)
		if !ok {
			continue
		}
		return entries, true
	}
	return nil, false
}

func parseKitInfoWattsEntries(raw any) ([]kitInfoWattsEntry, bool) {
	switch value := raw.(type) {
	case []any:
		return decodeKitInfoEntries(value), true
	case string:
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return nil, false
		}
		var decoded []any
		if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
			return nil, false
		}
		return decodeKitInfoEntries(decoded), true
	default:
		return nil, false
	}
}

func decodeKitInfoEntries(rawEntries []any) []kitInfoWattsEntry {
	out := make([]kitInfoWattsEntry, 0, len(rawEntries))
	for _, raw := range rawEntries {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, kitInfoWattsEntry{
			AppState: toInt64(entry["appState"]),
			CurPower: toFloat64(entry["curPower"]),
			AppVer:   toInt64(entry["appVer"]),
			F32Soc:   toFloat64(entry["f32Soc"]),
			Soc:      toInt64(entry["soc"]),
			AvaFlag:  toInt64(entry["avaFlag"]),
			SN:       toString(entry["sn"]),
			Detail:   toInt64(entry["detail"]),
			Type:     toInt64(entry["type"]),
			LoadVer:  toInt64(entry["loadVer"]),
		})
	}
	return out
}

func summarizeKitInfo(entries []kitInfoWattsEntry) kitInfoStats {
	if len(entries) == 0 {
		return kitInfoStats{}
	}
	stats := kitInfoStats{
		TotalSlots: len(entries),
	}
	socSamples := 0
	for _, entry := range entries {
		if entry.AppState != 0 {
			stats.ActiveSlots++
		}
		if entry.AvaFlag != 0 {
			stats.AvailableSlots++
		}

		switch {
		case entry.CurPower > 0:
			stats.ChargingSlots++
		case entry.CurPower < 0:
			stats.DischargingSlots++
		default:
			stats.IdleSlots++
		}
		stats.TotalCurPower += entry.CurPower

		soc := entrySOC(entry)
		if soc <= 0 && entry.AvaFlag == 0 {
			continue
		}
		if socSamples == 0 {
			stats.MinSOC = soc
			stats.MaxSOC = soc
		} else {
			if soc < stats.MinSOC {
				stats.MinSOC = soc
			}
			if soc > stats.MaxSOC {
				stats.MaxSOC = soc
			}
		}
		stats.AvgSOC += soc
		socSamples++
	}
	if socSamples > 0 {
		stats.AvgSOC = stats.AvgSOC / float64(socSamples)
	}
	return stats
}

func entrySOC(entry kitInfoWattsEntry) float64 {
	if entry.F32Soc > 0 {
		return entry.F32Soc
	}
	return float64(entry.Soc)
}

func toInt64(value any) int64 {
	number, ok := numberFromAny(value)
	if !ok {
		return 0
	}
	return int64(number)
}

func boolFromAny(value any) (bool, bool) {
	switch v := value.(type) {
	case bool:
		return v, true
	case string:
		trimmed := strings.TrimSpace(strings.ToLower(v))
		switch trimmed {
		case "1", "true", "on", "yes":
			return true, true
		case "0", "false", "off", "no":
			return false, true
		default:
			return false, false
		}
	default:
		number, ok := numberFromAny(value)
		if !ok {
			return false, false
		}
		return number > 0, true
	}
}

func toFloat64(value any) float64 {
	number, ok := numberFromAny(value)
	if !ok {
		return 0
	}
	return number
}

func toString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func parseUintArrayFromAny(value any) ([]uint64, bool) {
	switch v := value.(type) {
	case []any:
		out := make([]uint64, 0, len(v))
		for _, raw := range v {
			number, ok := numberFromAny(raw)
			if !ok || number < 0 {
				return nil, false
			}
			out = append(out, uint64(number))
		}
		return out, true
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return nil, false
		}
		var decoded []any
		if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
			return nil, false
		}
		return parseUintArrayFromAny(decoded)
	default:
		return nil, false
	}
}

func parseFloatArrayFromAny(value any) ([]float64, bool) {
	switch v := value.(type) {
	case []any:
		out := make([]float64, 0, len(v))
		for _, raw := range v {
			number, ok := numberFromAny(raw)
			if !ok {
				return nil, false
			}
			out = append(out, number)
		}
		return out, true
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return nil, false
		}
		var decoded []any
		if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
			return nil, false
		}
		return parseFloatArrayFromAny(decoded)
	default:
		return nil, false
	}
}

func (s *energySnapshot) Update(
	envelope telemetryEnvelope,
	quota map[string]any,
	kitEntries []kitInfoWattsEntry,
	hasKit bool,
	pdStatus pdStatusSummary,
	hasPDStatus bool,
) {
	if hasPDStatus {
		if pdStatus.Soc > 0 {
			s.DeviceSOC = float64(pdStatus.Soc)
			s.HasDeviceSOC = true
		}
		if pdStatus.ChgDsgState != 0 || hasQuotaKey(quota, "chgDsgState") {
			s.ChgDsgStateRaw = pdStatus.ChgDsgState
			s.HasChgDsgState = true
		}
		if rawChargeRemain, ok := findQuotaValueByCandidates(quota, "chgRemainTime"); ok {
			s.applyChargeRemain(toInt64(rawChargeRemain))
		}
		if rawDischargeRemain, ok := findQuotaValueByCandidates(quota, "dsgRemainTime"); ok {
			s.applyDischargeRemain(toInt64(rawDischargeRemain))
		}
		if pdStatus.WattsInSum != 0 || (!s.HasWattsIn && hasQuotaKey(quota, "wattsInSum")) {
			s.WattsIn = pdStatus.WattsInSum
			s.HasWattsIn = true
		}
		if pdStatus.WattsOutSum != 0 || (!s.HasWattsOut && hasQuotaKey(quota, "wattsOutSum")) {
			s.WattsOut = pdStatus.WattsOutSum
			s.HasWattsOut = true
		}
		if rawOutAC, ok := findQuotaValueByCandidates(quota, "invOutWatts", "outAcTtPwr", "outPrPwr"); ok {
			if outACWatts, ok := numberFromAny(rawOutAC); ok && outACWatts >= 0 {
				s.OutACWatts = normalizeOutputChannelWatts(outACWatts)
				s.HasOutAC = true
			}
		}
		if rawOutACL14, ok := findQuotaValueByCandidates(quota, "outAcL14Pwr"); ok {
			if outACL14Watts, ok := numberFromAny(rawOutACL14); ok && outACL14Watts >= 0 {
				s.OutACL14Watts = normalizeOutputChannelWatts(outACL14Watts)
				s.HasOutACL14 = true
			}
		}
		if rawACOn, ok := findQuotaValueByCandidates(quota, "cfgAcEnabled", "inv.cfgAcEnabled"); ok {
			if on, ok := boolFromAny(rawACOn); ok {
				s.ACOn = on
				s.HasACOn = true
			}
		}
		// If PV keys are present (even when value is 0), treat them as authoritative.
		pvLow, hasPVLow, pvHigh, hasPVHigh := splitPVChannels(pdStatus.PVChargeWatts)
		if hasPVLow {
			s.InPVLowWatts = pvLow
			s.HasInPVLow = true
		}
		if hasPVHigh {
			s.InPVHighWatts = pvHigh
			s.HasInPVHigh = true
		}
		if !s.refreshPVTotalFromChannels() && len(pdStatus.PVChargeWatts) > 0 {
			s.InPVWatts = pdStatus.TotalPVInputWatts
			s.HasInPV = true
		} else if pdStatus.TotalPVInputWatts > 0 {
			s.InPVWatts = pdStatus.TotalPVInputWatts
			s.HasInPV = true
		}
		pvLowType, hasPVLowType, pvHighType, hasPVHighType := splitPVChannelTypes(pdStatus.PVChargeTypes)
		if hasPVLowType {
			s.PVLowType = pvLowType
			s.HasPVLowType = true
		}
		if hasPVHighType {
			s.PVHighType = pvHighType
			s.HasPVHighType = true
		}
		if pdStatus.TotalXT150Watts != 0 || len(pdStatus.XT150Watts) > 0 {
			s.XT150Watts = pdStatus.TotalXT150Watts
			s.HasXT150 = true
		}
		// Accept explicit zero AC input updates so stale non-zero values do not linger.
		if acIn, ok := sumByKeySuffix(quota, "inAc5p8Pwr", "inAcC20Pwr"); ok {
			s.InACWatts = normalizeInputChannelWatts(acIn)
			s.HasInAC = true
		}
		if dcOut, hasDCChannels := sumByKeySuffix(quota, dcOutputSuffixes...); hasDCChannels {
			s.OutDCWatts = normalizeOutputChannelWatts(dcOut)
			s.HasOutDC = true
			s.HasOutDCExplicit = true
		} else {
			hasDCState, dcStateOn := dcOutStateFromQuota(quota)
			if hasDCState {
				s.DCOn = dcStateOn
				s.HasDCOn = true
			}
			switch {
			case hasDCState:
				// State is authoritative when reported, even if no per-port watts are present.
				if dcStateOn {
					s.OutDCWatts = 0
					s.HasOutDC = true
				} else {
					s.OutDCWatts = 0
					s.HasOutDC = true
				}
				s.HasOutDCExplicit = true
			case shouldInferDCOutputFromResidual(pdStatus, quota) && s.HasWattsOut && s.HasOutAC && (!s.HasOutDC || !s.HasOutDCExplicit):
				// Some firmware streams omit per-port DC watt keys but still provide total and AC output.
				residual := s.WattsOut - s.OutACWatts
				residual = deductXT150BatteryLinkFromResidual(residual, xt150ForResidualInference(pdStatus, s))
				if residual >= dcResidualInferenceMinWatts {
					s.OutDCWatts = normalizeOutputChannelWatts(residual)
					s.HasOutDC = true
				} else {
					s.OutDCWatts = 0
					s.HasOutDC = true
				}
				s.HasOutDCExplicit = false
			}
		}
		if pdStatus.RemainTime > 0 {
			s.applyGlobalRemain(pdStatus.RemainTime, "pdStatus")
			s.applyRemainForCurrentState(pdStatus.RemainTime)
		}
		for key, value := range pdStatus.Temperatures {
			s.Temperatures[normalizeTempKey(key)] = value
		}
	}

	if strings.EqualFold(envelope.TypeCode, "dAddr") || strings.EqualFold(strings.TrimSpace(envelope.Addr), "d_addr") {
		if soc, ok := numberFromAny(quota["cmsBattSoc"]); ok && soc > 0 {
			s.DeviceSOC = soc
			s.HasDeviceSOC = true
		}
		if fullEnergy, ok := numberFromAny(quota["cmsBattFullEnergy"]); ok && fullEnergy > 0 {
			s.FullEnergyWh = fullEnergy
			s.HasFullEnergy = true
		}
	}

	if hasKit {
		for i, entry := range kitEntries {
			if entry.AvaFlag == 0 && entry.Soc == 0 && entry.F32Soc == 0 && entry.CurPower == 0 && strings.TrimSpace(entry.SN) == "" {
				// Delta 2 Max reports slot-0 placeholder in kitInfo; wait for bmsStatus for main battery metrics.
				continue
			}
			packNo := i + 1
			pack := s.ensurePack(packNo)
			if soc := entrySOC(entry); soc > 0 {
				pack.SOC = soc
				pack.HasSOC = true
			}
			if entry.CurPower != 0 {
				pack.PowerW = entry.CurPower
				pack.HasPower = true
			}
			if serial := normalizePackSerial(entry.SN); serial != "" {
				s.bindPackSerial(packNo, serial)
			}

			label := strings.TrimSpace(entry.SN)
			if label == "" {
				label = fmt.Sprintf("slot-%d", packNo)
			}
			s.KitSOC[label] = entrySOC(entry)
		}
	}

	for key, value := range quota {
		if strings.Contains(key, ".") {
			continue
		}
		lower := strings.ToLower(key)
		switch lower {
		case "soc":
			if !isPDStatusEnvelope(envelope) && !strings.EqualFold(envelope.TypeCode, "dAddr") {
				break
			}
			if soc, ok := numberFromAny(value); ok && soc > 0 {
				s.DeviceSOC = soc
				s.HasDeviceSOC = true
			}
		case "cmsbattsoc":
			if soc, ok := numberFromAny(value); ok && soc > 0 {
				s.DeviceSOC = soc
				s.HasDeviceSOC = true
			}
		case "cmsbattfullenergy":
			if fullEnergy, ok := numberFromAny(value); ok && fullEnergy > 0 {
				s.FullEnergyWh = fullEnergy
				s.HasFullEnergy = true
			}
		case "chgdsgstate":
			s.ChgDsgStateRaw = toInt64(value)
			s.HasChgDsgState = true
		case "syschgdsgstate":
			s.ChgDsgStateRaw = toInt64(value)
			s.HasChgDsgState = true
		case "chgremaintime":
			s.applyChargeRemain(toInt64(value))
		case "dsgremaintime":
			s.applyDischargeRemain(toInt64(value))
		case "remaintime":
			// Global remainTime should come from app/pd status, not from per-pack bms frames.
			if isPDStatusEnvelope(envelope) {
				s.applyGlobalRemain(toInt64(value), "pdStatus")
			}
		case "wattsinsum":
			if watts, ok := numberFromAny(value); ok && (watts != 0 || !s.HasWattsIn) {
				s.WattsIn = watts
				s.HasWattsIn = true
			}
		case "wattsoutsum":
			if watts, ok := numberFromAny(value); ok && (watts != 0 || !s.HasWattsOut) {
				s.WattsOut = watts
				s.HasWattsOut = true
			}
		case "bmsinputwatts":
			if watts, ok := numberFromAny(value); ok {
				s.BatteryInWatts = normalizeInputChannelWatts(watts)
				s.HasBatteryIn = true
			}
			if !s.HasWattsIn || s.WattsIn == 0 {
				if watts, ok := numberFromAny(value); ok {
					s.WattsIn = watts
					s.HasWattsIn = true
				}
			}
		case "bmsoutputwatts":
			if watts, ok := numberFromAny(value); ok {
				s.BatteryOutWatts = normalizeOutputChannelWatts(watts)
				s.HasBatteryOut = true
			}
			if !s.HasWattsOut || s.WattsOut == 0 {
				if watts, ok := numberFromAny(value); ok {
					s.WattsOut = watts
					s.HasWattsOut = true
				}
			}
		case "batamp":
			if amp, ok := numberFromAny(value); ok {
				s.BatteryAmp = normalizeCurrentAmps(amp)
				s.HasBatteryAmp = true
			}
		case "batvol":
			if volts, ok := numberFromAny(value); ok {
				s.BatteryVolts = normalizeVoltageVolts(volts)
				s.HasBatteryVolts = true
			}
		case "inlvmpptvol":
			if volts, ok := numberFromAny(value); ok {
				s.SolarLVVolts = normalizeVoltageVolts(volts)
				s.HasSolarLVVolts = true
			}
		case "inlvmpptamp":
			if amps, ok := numberFromAny(value); ok {
				s.SolarLVAmp = normalizeCurrentAmps(amps)
				s.HasSolarLVAmp = true
			}
		case "inlvmpptpwr", "pv1chargewatts":
			if watts, ok := numberFromAny(value); ok {
				s.InPVLowWatts = normalizeInputChannelWatts(watts)
				s.HasInPVLow = true
			}
		case "inhvmpptvol":
			if volts, ok := numberFromAny(value); ok {
				s.SolarHVVolts = normalizeVoltageVolts(volts)
				s.HasSolarHVVolts = true
			}
		case "inhvmpptamp":
			if amps, ok := numberFromAny(value); ok {
				s.SolarHVAmp = normalizeCurrentAmps(amps)
				s.HasSolarHVAmp = true
			}
		case "inhvmpptpwr", "pv2chargewatts":
			if watts, ok := numberFromAny(value); ok {
				s.InPVHighWatts = normalizeInputChannelWatts(watts)
				s.HasInPVHigh = true
			}
		case "pluginfopvweaksourceflag":
			s.SolarWeakSourceFlag = toInt64(value) > 0
			s.HasSolarWeakSource = true
		case "pluginfopvlflag":
			s.SolarLowFlag = toInt64(value) > 0
			s.HasSolarLowFlag = true
		case "pluginfopvhflag":
			s.SolarHighFlag = toInt64(value) > 0
			s.HasSolarHighFlag = true
		case "pv1chargetype", "pluginfopvltype":
			s.PVLowType = toInt64(value)
			s.HasPVLowType = true
		case "pv2chargetype", "pluginfopvhtype":
			s.PVHighType = toInt64(value)
			s.HasPVHighType = true
		case "cfgacenabled":
			if on, ok := boolFromAny(value); ok {
				s.ACOn = on
				s.HasACOn = true
			}
		case "acoutfreq":
			if freq, ok := numberFromAny(value); ok {
				if freq > 0 {
					s.ACOn = true
					s.HasACOn = true
				}
			}
		case "outacttvol", "outacl14vol", "outacl11vol", "outacl12vol", "outacl21vol", "outacl22vol", "outac5p8vol":
			if volts, ok := numberFromAny(value); ok {
				if volts >= 50 {
					s.ACOn = true
					s.HasACOn = true
				}
			}
		case "dcoutstate":
			if on, ok := boolFromAny(value); ok {
				s.DCOn = on
				s.HasDCOn = true
				s.USBOn = on
				s.HasUSBOn = true
			}
		case "carstate":
			if on, ok := boolFromAny(value); ok {
				s.DC12VOn = on
				s.HasDC12VOn = true
			}
		case "showflag":
			if flags, ok := numberFromAny(value); ok {
				s.AppShowFlagRaw = int64(flags)
				s.HasAppShowFlag = true
				decodedACOn, decodedDCOn := decodeAppShowCircuitFlags(int64(flags))
				s.ACOn = decodedACOn
				s.HasACOn = true
				s.DCOn = decodedDCOn
				s.HasDCOn = true
			}
		case "bpnum":
			if bpNo, ok := numberFromAny(value); ok && bpNo > 0 {
				s.AppShowBatteryNo = int(bpNo)
				s.HasAppShowBpNum = true
				for i := 1; i <= int(bpNo); i++ {
					s.ensurePack(i)
				}
			}
		case "remaincombo":
			if combo, ok := numberFromAny(value); ok {
				s.RemainCombo = combo
				s.HasRemainCombo = true
			}
		case "fullcombo":
			if combo, ok := numberFromAny(value); ok {
				s.FullCombo = combo
				s.HasFullCombo = true
			}
		case "c20chgmaxwatts":
			if watts, ok := numberFromAny(value); ok && watts >= 0 {
				s.C20ChgMaxWatts = watts
				s.HasC20ChgMax = true
			}
		case "parachgmaxwatts":
			if watts, ok := numberFromAny(value); ok && watts >= 0 {
				s.ParaChgMaxWatts = watts
				s.HasParaChgMax = true
			}
		case "cmsmaxchgsoc":
			if soc, ok := numberFromAny(value); ok && soc >= 0 {
				s.MaxChargeSOC = soc
				s.HasMaxChargeSOC = true
			}
		case "cmsmindsgsoc":
			if soc, ok := numberFromAny(value); ok && soc >= 0 {
				s.MinDischargeSOC = soc
				s.HasMinDischarge = true
			}
		case "emsparavolmin":
			if volts, ok := numberFromAny(value); ok && volts > 0 {
				s.EMSParaVolMin = normalizeVoltageVolts(volts)
				s.HasEMSParaVolMin = true
			}
		case "emsparavolmax":
			if volts, ok := numberFromAny(value); ok && volts > 0 {
				s.EMSParaVolMax = normalizeVoltageVolts(volts)
				s.HasEMSParaVolMax = true
			}
		case "evchgmanualctrl":
			if on, ok := boolFromAny(value); ok {
				s.EVChargingOn = on
				s.HasEVChargingOn = true
			}
		case "plugininfoacprunstate", "pluginfoacprunstate":
			if runState, ok := numberFromAny(value); ok {
				s.EVChargingOn = runState > 0
				s.HasEVChargingOn = true
			}
		case "inputwatts":
			if !s.HasWattsIn || s.WattsIn == 0 {
				if watts, ok := numberFromAny(value); ok {
					s.WattsIn = watts
					s.HasWattsIn = true
				}
			}
		case "outputwatts":
			if !s.HasWattsOut || s.WattsOut == 0 {
				if watts, ok := numberFromAny(value); ok {
					s.WattsOut = watts
					s.HasWattsOut = true
				}
			}
		}

		if strings.HasSuffix(lower, "temp") {
			if temp, ok := numberFromAny(value); ok {
				s.Temperatures[normalizeTempKey(key)] = temp
				continue
			}
		}
		if strings.HasSuffix(lower, "celltemp") {
			if values, ok := parseFloatArrayFromAny(value); ok && len(values) > 0 {
				_, maxValue := minMax(values)
				s.Temperatures[normalizeTempKey(key)+".max"] = maxValue
			}
		}
	}

	for packNo, pack := range extractBatteryPacks(quota) {
		existing := s.ensurePack(packNo)
		if pack.HasSOC {
			existing.SOC = pack.SOC
			existing.HasSOC = true
		}
		if pack.HasTemp {
			existing.TempC = pack.TempC
			existing.HasTemp = true
			s.Temperatures[fmt.Sprintf("bp%d.temp", packNo)] = pack.TempC
		}
		if pack.HasPower {
			existing.PowerW = pack.PowerW
			existing.HasPower = true
		}
		if pack.HasEnergy {
			existing.EnergyWh = pack.EnergyWh
			existing.HasEnergy = true
		}
		if pack.HasSOH {
			existing.SOH = pack.SOH
			existing.HasSOH = true
		}
		if pack.HasActSOH {
			existing.ActSOH = pack.ActSOH
			existing.HasActSOH = true
		}
		if pack.HasVoltage {
			existing.VoltageV = pack.VoltageV
			existing.HasVoltage = true
		}
		if pack.HasTargetSOC {
			existing.TargetSOC = pack.TargetSOC
			existing.HasTargetSOC = true
		}
		if pack.HasMinSOC {
			existing.MinSOC = pack.MinSOC
			existing.HasMinSOC = true
		}
		if pack.HasMaxSOC {
			existing.MaxSOC = pack.MaxSOC
			existing.HasMaxSOC = true
		}
		if pack.HasDiffSOC {
			existing.DiffSOC = pack.DiffSOC
			existing.HasDiffSOC = true
		}
		if pack.HasRemainCap {
			existing.RemainCap = pack.RemainCap
			existing.HasRemainCap = true
		}
		if pack.HasFullCap {
			existing.FullCap = pack.FullCap
			existing.HasFullCap = true
		}
		if pack.HasDesignCap {
			existing.DesignCap = pack.DesignCap
			existing.HasDesignCap = true
		}
		if pack.HasBoardTemp {
			existing.BoardTempC = pack.BoardTempC
			existing.HasBoardTemp = true
		}
		if pack.RemainTimeRaw > 0 {
			existing.RemainTimeRaw = pack.RemainTimeRaw
			if !s.HasRemainTime && !isLikelyRemainSentinel(pack.RemainTimeRaw) {
				s.applyGlobalRemain(pack.RemainTimeRaw, "packFallback")
			}
		}
		if pack.HasMaxVolDiff {
			existing.MaxVolDiff = pack.MaxVolDiff
			existing.HasMaxVolDiff = true
		}
		if pack.HasPreconditioning {
			existing.PreconditioningOn = pack.PreconditioningOn
			existing.HasPreconditioning = true
		}
		if pack.HasPreconditioningState {
			existing.PreconditioningStateRaw = pack.PreconditioningStateRaw
			existing.HasPreconditioningState = true
		}
		if pack.HasPreconditioningEvent {
			existing.PreconditioningEventRaw = pack.PreconditioningEventRaw
			existing.HasPreconditioningEvent = true
		}
		if pack.HasPreconditioningHeat {
			existing.PreconditioningHeatTime = pack.PreconditioningHeatTime
			existing.HasPreconditioningHeat = true
		}
		if serial := normalizePackSerial(pack.Serial); serial != "" {
			s.bindPackSerial(packNo, serial)
		}
	}

	if packNo, ok := s.resolvePackNoFromEnvelope(envelope, quota); ok {
		pack := s.ensurePack(packNo)
		if soc, ok := firstNumberFromKeys(
			quota,
			"f32ShowSoc",
			envelope.Addr+".f32ShowSoc",
			"soc",
			envelope.Addr+".soc",
			"actSoc",
			envelope.Addr+".actSoc",
		); ok && soc > 0 {
			pack.SOC = soc
			pack.HasSOC = true
		}
		if actSOC, ok := firstNumberFromKeys(quota, "actSoc", envelope.Addr+".actSoc"); ok && actSOC > 0 {
			pack.ActSOC = actSOC
			pack.HasActSOC = true
		}
		if outputWatts, ok := firstNumberFromKeys(quota, "outputWatts", envelope.Addr+".outputWatts"); ok && outputWatts != 0 {
			// Pack output uses positive numbers for discharge in some payloads.
			if outputWatts > 0 {
				pack.PowerW = -outputWatts
			} else {
				pack.PowerW = outputWatts
			}
			pack.HasPower = true
		}
		if inputWatts, ok := firstNumberFromKeys(quota, "inputWatts", envelope.Addr+".inputWatts"); ok && inputWatts > 0 {
			pack.PowerW = inputWatts
			pack.HasPower = true
		}
		packAmp, hasPackAmp := firstNumberFromKeys(quota, "amp", envelope.Addr+".amp")
		packVolts, hasPackVolts := firstNumberFromKeys(quota, "vol", envelope.Addr+".vol")
		if hasPackVolts && packVolts > 0 {
			pack.VoltageV = normalizeVoltageVolts(packVolts)
			pack.HasVoltage = true
		}
		// Delta 2 Max bp1 (main unit battery) frequently omits per-pack input/output watts.
		// Fall back to amp*vol so pack state isn't stuck as unknown.
		if !pack.HasPower && hasPackAmp && hasPackVolts && packVolts > 0 {
			inferredPower := normalizeCurrentAmps(packAmp) * normalizeVoltageVolts(packVolts)
			if math.Abs(inferredPower) >= idleDrawNoiseFloorWatts {
				pack.PowerW = inferredPower
				pack.HasPower = true
			}
		}
		if remain, ok := firstNumberFromKeys(quota, "remainTime", envelope.Addr+".remainTime"); ok && remain > 0 {
			packRemain := int64(remain)
			if !isLikelyRemainSentinel(packRemain) {
				pack.RemainTimeRaw = packRemain
			}
			if !s.HasRemainTime && !isLikelyRemainSentinel(packRemain) {
				s.applyGlobalRemain(packRemain, "packFallback")
			}
		}
		if temp, ok := firstNumberFromKeys(quota, "bpTemp", "temp", envelope.Addr+".bpTemp", envelope.Addr+".temp"); ok {
			pack.TempC = temp
			pack.HasTemp = true
		}
		if maxVolDiff, ok := firstNumberFromKeys(quota, "maxVolDiff", envelope.Addr+".maxVolDiff"); ok && maxVolDiff >= 0 {
			pack.MaxVolDiff = maxVolDiff
			pack.HasMaxVolDiff = true
		}
		if soh, ok := firstNumberFromKeys(quota, "soh", envelope.Addr+".soh"); ok && soh >= 0 {
			pack.SOH = soh
			pack.HasSOH = true
		}
		if actSoh, ok := firstNumberFromKeys(quota, "actSoh", envelope.Addr+".actSoh"); ok && actSoh >= 0 {
			pack.ActSOH = actSoh
			pack.HasActSOH = true
		}
		if targetSoc, ok := firstNumberFromKeys(quota, "targetSoc", envelope.Addr+".targetSoc"); ok && targetSoc >= 0 {
			pack.TargetSOC = targetSoc
			pack.HasTargetSOC = true
		}
		if minSoc, ok := firstNumberFromKeys(quota, "dsgMinSoc", envelope.Addr+".dsgMinSoc"); ok && minSoc >= 0 {
			pack.MinSOC = minSoc
			pack.HasMinSOC = true
		}
		if maxSoc, ok := firstNumberFromKeys(quota, "chgMaxSoc", envelope.Addr+".chgMaxSoc"); ok && maxSoc >= 0 {
			pack.MaxSOC = maxSoc
			pack.HasMaxSOC = true
		}
		if diffSoc, ok := firstNumberFromKeys(quota, "bpDiffSoc", envelope.Addr+".bpDiffSoc", "diffSoc", envelope.Addr+".diffSoc"); ok {
			pack.DiffSOC = diffSoc
			pack.HasDiffSOC = true
		}
		if remainCap, ok := firstNumberFromKeys(quota, "remainCap", envelope.Addr+".remainCap"); ok && remainCap >= 0 {
			pack.RemainCap = remainCap
			pack.HasRemainCap = true
		}
		if fullCap, ok := firstNumberFromKeys(quota, "fullCap", envelope.Addr+".fullCap"); ok && fullCap >= 0 {
			pack.FullCap = fullCap
			pack.HasFullCap = true
		}
		if designCap, ok := firstNumberFromKeys(quota, "designCap", envelope.Addr+".designCap"); ok && designCap >= 0 {
			pack.DesignCap = designCap
			pack.HasDesignCap = true
		}
		if boardTemp, ok := firstNumberFromKeys(quota, "hwBoardTemp", envelope.Addr+".hwBoardTemp"); ok {
			pack.BoardTempC = boardTemp
			pack.HasBoardTemp = true
		}
		if stateRaw, ok := firstNumberFromKeys(quota, "ptcMosState", envelope.Addr+".ptcMosState"); ok {
			state := int64(stateRaw)
			pack.PreconditioningStateRaw = state
			pack.HasPreconditioningState = true
			pack.PreconditioningOn = state > 0
			pack.HasPreconditioning = true
		}
		if eventRaw, ok := firstNumberFromKeys(quota, "ptcHeatingEvent", envelope.Addr+".ptcHeatingEvent"); ok {
			event := int64(eventRaw)
			pack.PreconditioningEventRaw = event
			pack.HasPreconditioningEvent = true
			// On DPU telemetry, ptcHeatingEvent reflects preconditioning workflow state
			// and can remain authoritative even when ptcMosState reports 0.
			if event > 0 {
				pack.PreconditioningOn = true
				pack.HasPreconditioning = true
			}
		}
		if heatRaw, ok := firstNumberFromKeys(quota, "heatTime", envelope.Addr+".heatTime"); ok {
			heat := int64(heatRaw)
			pack.PreconditioningHeatTime = heat
			pack.HasPreconditioningHeat = true
			// Use heatTime as fallback preconditioning state only when explicit PTC MOS state is absent.
			if !pack.HasPreconditioningState {
				pack.PreconditioningOn = heat > 0
				pack.HasPreconditioning = true
			}
		}
	}

	if strings.EqualFold(envelope.TypeCode, "invStatus") {
		if temp, ok := numberFromAny(quota["outTemp"]); ok {
			s.Temperatures["inv.outTemp"] = temp
		}
		if temp, ok := numberFromAny(quota["dcInTemp"]); ok {
			s.Temperatures["inv.dcInTemp"] = temp
		}
		if outputWatts, ok := firstNumberFromKeys(quota, "outputWatts"); ok && outputWatts >= 0 {
			s.OutACWatts = normalizeOutputChannelWatts(outputWatts)
			s.HasOutAC = true
		}
		// invStatus can explicitly report 0W; keep it to avoid stale AC-in lag.
		if inputWatts, ok := firstNumberFromKeys(quota, "inputWatts"); ok && inputWatts >= 0 {
			s.InACWatts = normalizeInputChannelWatts(inputWatts)
			s.HasInAC = true
		}
	}
	if strings.EqualFold(envelope.TypeCode, "mpptStatus") {
		if value, ok := firstNumberFromKeys(quota, "inWatts"); ok {
			s.InPVLowWatts = normalizeInputChannelWatts(value)
			s.HasInPVLow = true
		}
		if value, ok := firstNumberFromKeys(quota, "pv2InWatts"); ok {
			s.InPVHighWatts = normalizeInputChannelWatts(value)
			s.HasInPVHigh = true
		}
		if value, ok := firstNumberFromKeys(quota, "inVol"); ok {
			s.SolarLVVolts = normalizeVoltageVolts(value)
			s.HasSolarLVVolts = true
		}
		if value, ok := firstNumberFromKeys(quota, "inAmp"); ok {
			s.SolarLVAmp = normalizeCurrentAmps(value)
			s.HasSolarLVAmp = true
		}
		if value, ok := firstNumberFromKeys(quota, "pv2InVol"); ok {
			s.SolarHVVolts = normalizeVoltageVolts(value)
			s.HasSolarHVVolts = true
		}
		if value, ok := firstNumberFromKeys(quota, "pv2InAmp"); ok {
			s.SolarHVAmp = normalizeCurrentAmps(value)
			s.HasSolarHVAmp = true
		}
		s.refreshPVTotalFromChannels()
	}
	if pvLow, hasPVLow, pvHigh, hasPVHigh := sumPVInputChannelsFromQuota(quota); hasPVLow || hasPVHigh {
		if hasPVLow {
			s.InPVLowWatts = pvLow
			s.HasInPVLow = true
		}
		if hasPVHigh {
			s.InPVHighWatts = pvHigh
			s.HasInPVHigh = true
		}
		s.refreshPVTotalFromChannels()
	} else if inPV, ok := sumPVInputFromQuota(quota); ok {
		s.InPVWatts = inPV
		s.HasInPV = true
	}
	if xt150, ok := sumXT150FromQuota(quota); ok {
		s.XT150Watts = xt150
		s.HasXT150 = true
	}
	if outAC, ok := sumByKeySuffix(quota, acOutputSuffixes...); ok {
		s.OutACWatts = normalizeOutputChannelWatts(outAC)
		s.HasOutAC = true
	}
	if outACL14, ok := sumByKeySuffix(quota, "outAcL14Pwr"); ok {
		s.OutACL14Watts = normalizeOutputChannelWatts(outACL14)
		s.HasOutACL14 = true
	}
	if outDC, ok := sumByKeySuffix(quota, dcOutputSuffixes...); ok {
		s.OutDCWatts = normalizeOutputChannelWatts(outDC)
		s.HasOutDC = true
		s.HasOutDCExplicit = true
	}
	if inAC, ok := sumByKeySuffix(quota, acInputSuffixes...); ok {
		s.InACWatts = normalizeInputChannelWatts(inAC)
		s.HasInAC = true
	}

	// applyPrimaryPackFallbacks(s, quota)

	// Fill missing channel views from totals when possible.
	if s.HasWattsIn {
		if !s.HasInPV && s.HasInAC && !(s.HasXT150 && s.XT150Watts > 0) {
			remaining := s.WattsIn - s.InACWatts
			if remaining > 0 {
				s.InPVWatts = remaining
				s.HasInPV = true
			}
		}
		if !s.HasInAC && s.HasInPV {
			remaining := s.WattsIn - s.InPVWatts
			if remaining > 0 {
				s.InACWatts = remaining
				s.HasInAC = true
			}
		}
	}
	s.pushPVSmoothingSample()
	s.pushPowerSmoothingSample()
	if !s.HasDeviceSOC {
		if avgSOC, ok := averagePackSOC(s.Packs); ok {
			s.DeviceSOC = avgSOC
			s.HasDeviceSOC = true
		}
	}
}

func (s *energySnapshot) String() string {
	derived := s.derived()

	return fmt.Sprintf(
		"soc=%s packs=%s temps=%s in=%s out=%s net=%s state=%s in_ac=%s in_pv=%s in_pv_low=%s in_pv_high=%s xt150_in=%s out_ac=%s out_ac_l14=%s out_dc=%s xt150_out=%s batt_in=%s batt_out=%s batt_net=%s idle_draw=%s pv_state=%s remain=%s charge_left=%s",
		derived.SOCValue,
		derived.PacksValue,
		derived.TempsValue,
		derived.InputValue,
		derived.OutputValue,
		derived.NetValue,
		derived.SystemStateValue,
		derived.InACValue,
		derived.InPVValue,
		derived.InPVLowValue,
		derived.InPVHighValue,
		derived.XT150InValue,
		derived.OutACValue,
		derived.OutACL14Value,
		derived.OutDCValue,
		derived.XT150OutValue,
		derived.BatteryInValue,
		derived.BatteryOutValue,
		derived.BatteryNetValue,
		derived.IdleDrawValue,
		derived.PVStateValue,
		derived.RemainValue,
		derived.ChargeLeftValue,
	)
}

func (s *energySnapshot) derived() snapshotDerived {
	derived := snapshotDerived{
		PacksValue: formatPackSummary(s.Packs, s.KitSOC),
		TempsValue: formatTemperatureSummary(s.Temperatures, 8),
	}
	if soc, ok := s.displaySOC(); ok {
		derived.SOCValue = fmt.Sprintf("%.2f%%", soc)
	} else {
		derived.SOCValue = "n/a"
	}

	packChargeW, packDischargeW := packPowerTotals(s.Packs)

	derived.EffectiveIn, derived.HasEffectiveIn, derived.EffectiveOut, derived.HasEffectiveOut =
		s.effectiveTotalsForDisplayWithPackTotals(packChargeW, packDischargeW)

	derived.InputValue = "n/a"
	if derived.HasEffectiveIn {
		derived.InputValue = formatWatts(derived.EffectiveIn)
	}
	derived.OutputValue = "n/a"
	if derived.HasEffectiveOut {
		derived.OutputValue = formatWatts(derived.EffectiveOut)
	}
	derived.NetValue = "n/a"
	if derived.HasEffectiveIn && derived.HasEffectiveOut {
		derived.NetValue = formatWatts(derived.EffectiveIn - derived.EffectiveOut)
	}
	systemState := s.detectSystemState(derived.EffectiveIn, derived.HasEffectiveIn, derived.EffectiveOut, derived.HasEffectiveOut, packChargeW, packDischargeW)
	derived.SystemStateValue = string(systemState)
	derived.InACValue = formatOptionalWatts(s.HasInAC, s.InACWatts)
	pvTotalWatts, hasPVTotal, pvLowWatts, hasPVLowWatts, pvHighWatts, hasPVHighWatts := s.effectivePVInputChannels()
	derived.InPVValue = formatOptionalWatts(hasPVTotal, pvTotalWatts)
	derived.InPVLowValue = formatOptionalWatts(hasPVLowWatts, pvLowWatts)
	derived.InPVHighValue = formatOptionalWatts(hasPVHighWatts, pvHighWatts)
	derived.PVLowVoltsValue = formatOptionalVolts(s.HasSolarLVVolts, s.SolarLVVolts)
	derived.PVHighVoltsValue = formatOptionalVolts(s.HasSolarHVVolts, s.SolarHVVolts)
	derived.PVLowAmpsValue = formatOptionalAmps(s.HasSolarLVAmp, s.SolarLVAmp)
	derived.PVHighAmpsValue = formatOptionalAmps(s.HasSolarHVAmp, s.SolarHVAmp)
	derived.OutACValue = formatOptionalWatts(s.HasOutAC, s.OutACWatts)
	derived.OutACL14Value = formatOptionalWatts(s.HasOutACL14, s.OutACL14Watts)
	derived.OutDCValue = formatOptionalWatts(s.HasOutDC, s.OutDCWatts)
	derived.XT150InValue, derived.XT150OutValue = formatXT150DirectionalValues(s.HasXT150, s.XT150Watts)
	if s.HasInAC && hasPVTotal && s.HasOutAC && s.HasOutDC {
		derived.ChannelsNetValue = formatWatts((s.InACWatts + pvTotalWatts) - (s.OutACWatts + s.OutDCWatts))
	} else {
		derived.ChannelsNetValue = "n/a"
	}

	acOn := s.ACOn
	if s.HasOutAC && s.OutACWatts > 0 {
		acOn = true
	}
	if s.HasOutACL14 && s.OutACL14Watts > 0 {
		acOn = true
	}
	if !s.HasACOn && !s.HasOutAC && !s.HasOutACL14 {
		acOn = false
	}
	dcOn := s.DCOn
	if s.HasOutDC && s.OutDCWatts > 0 {
		dcOn = true
	}
	if !s.HasDCOn && !s.HasOutDC {
		dcOn = false
	}
	usbOn := s.USBOn
	if !s.HasUSBOn {
		usbOn = dcOn
	}
	dc12VOn := s.DC12VOn
	evOn := s.EVChargingOn
	passthroughOn := isLikelyACPassthrough(s.HasInAC, s.InACWatts, s.HasOutAC, s.OutACWatts)
	derived.StatusACValue = checkboxStatus(acOn)
	derived.StatusDCValue = checkboxStatus(dcOn)
	derived.StatusUSBValue = checkboxStatus(usbOn)
	derived.StatusDC12VValue = checkboxStatus(dc12VOn)
	derived.StatusEVValue = checkboxStatus(evOn)
	derived.StatusPassthroughValue = checkboxStatus(passthroughOn)
	// Grounded estimate is inferred from AC passthrough behavior.
	derived.StatusGroundedValue = checkboxStatus(passthroughOn)
	derived.StatusSolarPassValue = "[ ]"
	knownPreconditioning, anyPreconditioningOn := overallPreconditioningStatus(s.Packs)
	if knownPreconditioning {
		derived.StatusPrecondValue = checkboxStatus(anyPreconditioningOn)
	} else {
		derived.StatusPrecondValue = "[ ]"
	}
	derived.ShowFlagValue = "n/a"
	if s.HasAppShowFlag {
		derived.ShowFlagValue = fmt.Sprintf("%d", s.AppShowFlagRaw)
	}
	derived.BatteryCount = "n/a"
	if s.HasAppShowBpNum && s.AppShowBatteryNo > 0 {
		derived.BatteryCount = fmt.Sprintf("%d", s.AppShowBatteryNo)
	} else if len(s.Packs) > 0 {
		derived.BatteryCount = fmt.Sprintf("%d", len(s.Packs))
	}
	derived.ComboValue = "n/a"
	if s.HasRemainCombo || s.HasFullCombo {
		remain := "n/a"
		full := "n/a"
		if s.HasRemainCombo {
			remain = fmt.Sprintf("%.0f", s.RemainCombo)
		}
		if s.HasFullCombo {
			full = fmt.Sprintf("%.0f", s.FullCombo)
		}
		derived.ComboValue = fmt.Sprintf("%s/%s", remain, full)
	}
	derived.C20LimitValue = "n/a"
	if s.HasC20ChgMax {
		derived.C20LimitValue = formatWatts(s.C20ChgMaxWatts)
	}
	derived.ParaLimitValue = "n/a"
	if s.HasParaChgMax {
		derived.ParaLimitValue = formatWatts(s.ParaChgMaxWatts)
	}
	derived.EMSWindowValue = "n/a"
	if s.HasEMSParaVolMin || s.HasEMSParaVolMax {
		minV := "n/a"
		maxV := "n/a"
		if s.HasEMSParaVolMin {
			minV = fmt.Sprintf("%.1fV", s.EMSParaVolMin)
		}
		if s.HasEMSParaVolMax {
			maxV = fmt.Sprintf("%.1fV", s.EMSParaVolMax)
		}
		derived.EMSWindowValue = minV + " .. " + maxV
	}
	derived.SocGuardrail = "n/a"
	if s.HasMinDischarge || s.HasMaxChargeSOC {
		minSoc := "n/a"
		maxSoc := "n/a"
		if s.HasMinDischarge {
			minSoc = fmt.Sprintf("%.0f%%", s.MinDischargeSOC)
		}
		if s.HasMaxChargeSOC {
			maxSoc = fmt.Sprintf("%.0f%%", s.MaxChargeSOC)
		}
		derived.SocGuardrail = minSoc + " .. " + maxSoc
	}

	batteryInWatts := 0.0
	hasBatteryIn := false
	batteryFlowFromNet := false
	if s.HasBatteryIn {
		batteryInWatts = s.BatteryInWatts
		hasBatteryIn = true
	}
	batteryOutWatts := 0.0
	hasBatteryOut := false
	if s.HasBatteryOut {
		batteryOutWatts = s.BatteryOutWatts
		hasBatteryOut = true
	}
	if (!hasBatteryIn || !hasBatteryOut) && s.HasBatteryAmp && s.HasBatteryVolts {
		busWatts := normalizeCurrentAmps(s.BatteryAmp) * normalizeVoltageVolts(s.BatteryVolts)
		if math.Abs(busWatts) >= idleDrawNoiseFloorWatts {
			if busWatts > 0 && !hasBatteryIn {
				batteryInWatts = busWatts
				hasBatteryIn = true
			}
			if busWatts < 0 && !hasBatteryOut {
				batteryOutWatts = -busWatts
				hasBatteryOut = true
			}
		}
	}
	if !hasBatteryIn && packChargeW > idleDrawNoiseFloorWatts {
		batteryInWatts = packChargeW
		hasBatteryIn = true
	}
	if !hasBatteryOut && packDischargeW > idleDrawNoiseFloorWatts {
		batteryOutWatts = packDischargeW
		hasBatteryOut = true
	}
	if derived.HasEffectiveIn && derived.HasEffectiveOut {
		netWatts := derived.EffectiveIn - derived.EffectiveOut
		switch {
		case netWatts > systemStateNetThresholdWatts:
			// Total in/out is the most reliable battery direction signal.
			batteryInWatts = netWatts
			hasBatteryIn = true
			batteryOutWatts = 0
			hasBatteryOut = true
			batteryFlowFromNet = true
		case netWatts < -systemStateNetThresholdWatts:
			batteryOutWatts = -netWatts
			hasBatteryOut = true
			batteryInWatts = 0
			hasBatteryIn = true
			batteryFlowFromNet = true
		}
	}
	if !batteryFlowFromNet {
		if hasBatteryIn {
			if sanitized, ok := s.sanitizeBatteryFlowHintWatts(batteryInWatts); ok {
				batteryInWatts = sanitized
			} else {
				hasBatteryIn = false
			}
		}
		if hasBatteryOut {
			if sanitized, ok := s.sanitizeBatteryFlowHintWatts(batteryOutWatts); ok {
				batteryOutWatts = sanitized
			} else {
				hasBatteryOut = false
			}
		}
	}
	if hasBatteryIn && !hasBatteryOut {
		batteryOutWatts = 0
		hasBatteryOut = true
	}
	if hasBatteryOut && !hasBatteryIn {
		batteryInWatts = 0
		hasBatteryIn = true
	}
	derived.BatteryInValue = formatOptionalWatts(hasBatteryIn, batteryInWatts)
	derived.BatteryOutValue = formatOptionalWatts(hasBatteryOut, batteryOutWatts)
	derived.BatteryNetValue = "n/a"
	if hasBatteryIn || hasBatteryOut {
		derived.BatteryNetValue = formatWatts(batteryOutWatts - batteryInWatts)
	}
	solarPassthroughOn := isLikelySolarPassthrough(s, batteryInWatts, hasBatteryIn, batteryOutWatts, hasBatteryOut)
	derived.StatusSolarPassValue = checkboxStatus(solarPassthroughOn)

	derived.IdleDrawValue = "n/a"
	if hasBatteryOut && (!hasBatteryIn || batteryInWatts <= idleDrawNoiseFloorWatts) {
		externalOutWatts, hasExternalOut := 0.0, false
		if s.HasOutAC {
			externalOutWatts += s.OutACWatts
			hasExternalOut = true
		}
		if s.HasOutDC {
			externalOutWatts += s.OutDCWatts
			hasExternalOut = true
		}
		if !hasExternalOut && s.HasWattsOut {
			externalOutWatts = s.WattsOut
			hasExternalOut = true
		}
		if hasExternalOut {
			idleDrawWatts := batteryOutWatts - externalOutWatts
			if math.Abs(idleDrawWatts) <= idleDrawNoiseFloorWatts {
				idleDrawWatts = 0
			}
			if idleDrawWatts >= 0 {
				derived.IdleDrawValue = formatWatts(idleDrawWatts)
			}
		}
	}

	lowLockedByFlag := (s.HasSolarWeakSource && s.SolarWeakSourceFlag) || (s.HasSolarLowFlag && s.SolarLowFlag)
	derived.PVLowStateValue = derivePVInputState(
		hasPVLowWatts,
		pvLowWatts,
		s.HasSolarLVVolts,
		s.SolarLVVolts,
		s.HasSolarLVAmp,
		s.SolarLVAmp,
		lowLockedByFlag,
	)
	derived.PVHighStateValue = derivePVInputState(
		hasPVHighWatts,
		pvHighWatts,
		s.HasSolarHVVolts,
		s.SolarHVVolts,
		s.HasSolarHVAmp,
		s.SolarHVAmp,
		s.HasSolarHighFlag && s.SolarHighFlag,
	)

	switch {
	case derived.PVLowStateValue != "n/a" && derived.PVHighStateValue != "n/a":
		derived.PVStateValue = fmt.Sprintf("low=%s high=%s", derived.PVLowStateValue, derived.PVHighStateValue)
	case derived.PVLowStateValue != "n/a":
		derived.PVStateValue = derived.PVLowStateValue
	case derived.PVHighStateValue != "n/a":
		derived.PVStateValue = derived.PVHighStateValue
	default:
		derived.PVStateValue = "n/a"
	}

	derived.RemainValue = "n/a"
	if remainMinutes, remainSource, ok := s.selectRemainForState(systemState); ok {
		derived.RemainValue = formatRemainValueWithState(remainMinutes, systemState, remainSource)
	}

	leftWh := estimatedWhLeft(s.Packs)
	if leftWh <= 0 && s.HasFullEnergy && s.HasDeviceSOC {
		leftWh = s.FullEnergyWh * (s.DeviceSOC / 100.0)
	}
	derived.ChargeLeftValue = "n/a"
	if leftWh > 0 {
		derived.ChargeLeftValue = formatEnergyWh(leftWh)
	}
	etaEstimates := s.estimateBatteryETAs(
		systemState,
		batteryInWatts,
		hasBatteryIn,
		batteryOutWatts,
		hasBatteryOut,
		derived.EffectiveIn,
		derived.HasEffectiveIn,
		derived.EffectiveOut,
		derived.HasEffectiveOut,
	)
	derived.EstimateChargeValue = etaEstimates.ChargeValue
	derived.EstimateDischargeValue = etaEstimates.DischargeValue
	derived.EstimateActiveValue = etaEstimates.ActiveValue
	derived.EstimatePowerValue = etaEstimates.PowerValue
	derived.EstimateConfidenceValue = etaEstimates.ConfidenceValue

	return derived
}

func renderDashboard(
	device ecoflow.GeneralInfoDevice,
	topic string,
	envelope telemetryEnvelope,
	snapshot *energySnapshot,
	minuteHistory *minuteTelemetryHistory,
	minuteCfg minuteTableConfig,
) string {
	derived := snapshot.derived()
	pvTotalRaw, hasPVTotalRaw, pvLowRaw, hasPVLowRaw, pvHighRaw, hasPVHighRaw := snapshot.effectivePVInputChannels()
	pvTotalSmooth, hasPVTotalSmooth, pvLowSmooth, hasPVLowSmooth, pvHighSmooth, hasPVHighSmooth := snapshot.smoothedPVChannels()
	acInSmooth, hasACInSmooth := snapshot.smoothedACInput()
	totalInSmooth, hasTotalInSmooth, totalOutSmooth, hasTotalOutSmooth := snapshot.smoothedTotalChannels()
	pvTotalDisplay := formatSmoothedWattsValue(derived.InPVValue, hasPVTotalRaw, pvTotalRaw, hasPVTotalSmooth, pvTotalSmooth)
	pvLowDisplay := formatSmoothedWattsValue(derived.InPVLowValue, hasPVLowRaw, pvLowRaw, hasPVLowSmooth, pvLowSmooth)
	pvHighDisplay := formatSmoothedWattsValue(derived.InPVHighValue, hasPVHighRaw, pvHighRaw, hasPVHighSmooth, pvHighSmooth)
	acInDisplay := formatSmoothedWattsValue(derived.InACValue, snapshot.HasInAC, snapshot.InACWatts, hasACInSmooth, acInSmooth)
	totalOutDisplay := formatSmoothedWattsValue(derived.OutputValue, derived.HasEffectiveOut, derived.EffectiveOut, hasTotalOutSmooth, totalOutSmooth)
	totalNetDisplay := derived.NetValue
	hasRawNet := derived.HasEffectiveIn && derived.HasEffectiveOut
	rawNet := derived.EffectiveIn - derived.EffectiveOut
	hasSmoothNet := hasTotalInSmooth && hasTotalOutSmooth
	smoothNet := totalInSmooth - totalOutSmooth
	if hasRawNet {
		totalNetDisplay = formatSmoothedWattsValue(derived.NetValue, true, rawNet, hasSmoothNet, smoothNet)
	}
	mqttQueueValue := "n/a"
	mqttDropsValue := "n/a"
	if snapshot.MQTTQueueCapacity > 0 {
		mqttQueueValue = fmt.Sprintf("%d/%d", snapshot.MQTTQueueDepth, snapshot.MQTTQueueCapacity)
		mqttDropsValue = fmt.Sprintf("drop-oldest: %d", snapshot.MQTTQueueDroppedOldest)
	}
	pvLowLabel := formatPVInputRowLabel("low", device, snapshot)
	pvHighLabel := formatPVInputRowLabel("high", device, snapshot)

	updatedAt := time.Now().Format("2006-01-02 15:04:05")
	deviceHeaders := []string{"Device Name", "SOC", "AC In", "Solar Generated", "Out", "Net", "State", "Updated"}
	summaryHeaders := []string{
		"Details",
		"In",
		"Out",
		"Net",
		"Remain",
	}
	lastTypeState := formatTypeWithState(firstNonEmpty(envelope.TypeCode, "n/a"), derived.SystemStateValue)
	lastMQTTMeta := formatLastMQTTMeta(envelope)
	mlEstimates := estimateBatteryETAsML(snapshot, minuteHistory, systemStateKind(derived.SystemStateValue))
	primaryEstimateCharge := preferEstimateValue(mlEstimates.ChargeValue, derived.EstimateChargeValue)
	primaryEstimateDischarge := preferEstimateValue(mlEstimates.DischargeValue, derived.EstimateDischargeValue)
	primaryEstimateActive := preferEstimateValue(mlEstimates.ActiveValue, derived.EstimateActiveValue)
	primaryEstimatePower := preferEstimateValue(mlEstimates.PowerValue, derived.EstimatePowerValue)
	primaryEstimateConfidence := preferEstimateValue(mlEstimates.ConfidenceValue, derived.EstimateConfidenceValue)
	topStateValue := selectTopStateValue(
		derived.RemainValue,
		systemStateKind(derived.SystemStateValue),
		mlEstimates,
		batteryETAEstimates{
			ActiveValue:     derived.EstimateActiveValue,
			ConfidenceValue: derived.EstimateConfidenceValue,
		},
	)
	deviceRows := [][]string{{
		chooseDeviceLabel(device),
		derived.SOCValue,
		acInDisplay,
		pvTotalDisplay,
		totalOutDisplay,
		totalNetDisplay,
		topStateValue,
		updatedAt,
	}}
	summaryRows := [][]string{
		{
			"channels",
			fmt.Sprintf("ac: %s pv_total: %s xt150_in: %s", derived.InACValue, pvTotalDisplay, derived.XT150InValue),
			fmt.Sprintf("ac: %s (l14: %s) dc: %s xt150_out: %s", derived.OutACValue, derived.OutACL14Value, derived.OutDCValue, derived.XT150OutValue),
			derived.ChannelsNetValue,
			"-",
		},
		{
			"meta",
			fmt.Sprintf("packs: %s showFlag: %s", derived.BatteryCount, derived.ShowFlagValue),
			fmt.Sprintf("combo: %s c20: %s para: %s", derived.ComboValue, derived.C20LimitValue, derived.ParaLimitValue),
			fmt.Sprintf("socWindow: %s", derived.SocGuardrail),
			"-",
		},
		{
			pvLowLabel,
			fmt.Sprintf("in: %s", pvLowDisplay),
			fmt.Sprintf("volts: %s amps: %s", derived.PVLowVoltsValue, derived.PVLowAmpsValue),
			fmt.Sprintf("state: %s", derived.PVLowStateValue),
			"-",
		},
		{
			pvHighLabel,
			fmt.Sprintf("in: %s", pvHighDisplay),
			fmt.Sprintf("volts: %s amps: %s", derived.PVHighVoltsValue, derived.PVHighAmpsValue),
			fmt.Sprintf("state: %s", derived.PVHighStateValue),
			"-",
		},
		{
			"battery",
			fmt.Sprintf("in: %s", derived.BatteryInValue),
			fmt.Sprintf("out: %s idle: %s", derived.BatteryOutValue, derived.IdleDrawValue),
			derived.BatteryNetValue,
			"-",
		},
		{
			"estimates",
			fmt.Sprintf("charge: %s", primaryEstimateCharge),
			fmt.Sprintf("discharge: %s", primaryEstimateDischarge),
			primaryEstimatePower,
			fmt.Sprintf("active: %s conf: %s", primaryEstimateActive, primaryEstimateConfidence),
		},
		{
			"heuristic",
			fmt.Sprintf("charge: %s", derived.EstimateChargeValue),
			fmt.Sprintf("discharge: %s", derived.EstimateDischargeValue),
			derived.EstimatePowerValue,
			fmt.Sprintf("active: %s conf: %s", derived.EstimateActiveValue, derived.EstimateConfidenceValue),
		},
		{
			"mqtt",
			fmt.Sprintf("queue: %s", mqttQueueValue),
			mqttDropsValue,
			fmt.Sprintf("last: %s", lastTypeState),
			lastMQTTMeta,
		},
	}

	packHeaders := []string{"Pack", "SOC", "Temp", "Power", "Remain", "MaxΔmV", "State"}
	packRows := make([][]string, 0, len(snapshot.Packs))
	ids := make([]int, 0, len(snapshot.Packs))
	for id := range snapshot.Packs {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		pack := snapshot.Packs[id]
		soc := "n/a"
		if pack.HasSOC {
			soc = fmt.Sprintf("%.2f%%", pack.SOC)
		}
		temp := "n/a"
		if pack.HasTemp {
			temp = fmt.Sprintf("%.1fC", pack.TempC)
		}
		power := "n/a"
		if pack.HasPower {
			power = formatWatts(pack.PowerW)
		}
		remain := "n/a"
		if pack.RemainTimeRaw > 0 && !isLikelyRemainSentinel(pack.RemainTimeRaw) {
			remain = fmt.Sprintf("%dmin (~%s)", pack.RemainTimeRaw, formatMinutesHuman(pack.RemainTimeRaw))
		}
		maxVolDiff := "n/a"
		if pack.HasMaxVolDiff {
			maxVolDiff = fmt.Sprintf("%.0f", pack.MaxVolDiff)
		}
		packRows = append(packRows, []string{
			fmt.Sprintf("bp%d", id),
			soc,
			temp,
			power,
			remain,
			maxVolDiff,
			packStateLabel(pack),
		})
	}
	if len(packRows) == 0 {
		packRows = append(packRows, []string{"n/a", "n/a", "n/a", "n/a", "n/a", "n/a", "waiting"})
	}

	packDiagHeaders := []string{"Pack", "Serial", "Energy", "SOH", "Voltage", "SOC(target)", "Limits", "ΔSOC", "Cap(rem/full)", "Board"}
	packDiagRows := make([][]string, 0, len(snapshot.Packs))
	for _, id := range ids {
		pack := snapshot.Packs[id]
		packDiagRows = append(packDiagRows, []string{
			fmt.Sprintf("bp%d", id),
			formatPackSerial(pack),
			formatPackEnergy(pack),
			formatPackSoh(pack),
			formatPackVoltage(pack),
			formatPackTarget(pack),
			formatPackLimits(pack),
			formatPackDiffSOC(pack),
			formatPackCapacity(pack),
			formatPackBoardTemp(pack),
		})
	}
	if len(packDiagRows) == 0 {
		packDiagRows = append(packDiagRows, []string{"n/a", "n/a", "n/a", "n/a", "n/a", "n/a", "n/a", "n/a", "n/a", "n/a"})
	}

	minuteHeaders := []string{
		"Time",
		"Solar Generated (Wh)",
		"AC Input (Wh)",
		"AC Output (Wh)",
		"DC Output (Wh)",
		"Battery Charge (Wh)",
		"Total Input (Wh)",
		"Total Output (Wh)",
		"Net (Wh)",
	}
	minuteRows := buildMinuteTelemetryRows(minuteHistory, minuteCfg)
	if len(minuteRows) == 0 {
		minuteRows = append(minuteRows, []string{"n/a", "n/a", "n/a", "n/a", "n/a", "n/a", "n/a", "n/a", "n/a"})
	}

	var builder strings.Builder
	showSeparateUSBAndDC := shouldShowSeparateUSBAndDC(device, snapshot)
	showEVStatus := shouldShowEVStatus(device, snapshot)
	showPreconditioningStatus := shouldShowPreconditioningStatus(device, snapshot)
	builder.WriteString("\033[H\033[2J")
	builder.WriteString("EcoFlow Live Telemetry\n")
	builder.WriteString("Topic: " + topic + "\n\n")
	builder.WriteString(renderASCIITable(deviceHeaders, deviceRows))
	builder.WriteString("\n")
	builder.WriteString(fmt.Sprintf("%s AC On\n", derived.StatusACValue))
	if showSeparateUSBAndDC {
		builder.WriteString(fmt.Sprintf("%s USB On\n", derived.StatusUSBValue))
		builder.WriteString(fmt.Sprintf("%s 12V DC On\n", derived.StatusDC12VValue))
	} else {
		builder.WriteString(fmt.Sprintf("%s DC/USB On\n", derived.StatusDCValue))
	}
	if showEVStatus {
		builder.WriteString(fmt.Sprintf("%s EV Charging On\n", derived.StatusEVValue))
	}
	builder.WriteString(fmt.Sprintf("%s UPS Passthrough\n", derived.StatusPassthroughValue))
	builder.WriteString(fmt.Sprintf("%s Solar Passthrough\n", derived.StatusSolarPassValue))
	builder.WriteString(fmt.Sprintf("%s Grounded (Estimated)\n", derived.StatusGroundedValue))
	if showPreconditioningStatus {
		builder.WriteString(fmt.Sprintf("%s Battery Preconditioning On\n\n", derived.StatusPrecondValue))
	} else {
		builder.WriteString("\n")
	}
	builder.WriteString(renderASCIITable(summaryHeaders, summaryRows))
	builder.WriteString("\n")
	builder.WriteString(renderASCIITable(packHeaders, packRows))
	builder.WriteString("\n\n")
	builder.WriteString(renderASCIITable(packDiagHeaders, packDiagRows))
	builder.WriteString("\n\n")
	builder.WriteString(renderASCIITable(minuteHeaders, minuteRows))
	builder.WriteString("\n")
	return builder.String()
}

func chooseDeviceLabel(device ecoflow.GeneralInfoDevice) string {
	name := strings.TrimSpace(device.DeviceName)
	if name == "" {
		return device.SN
	}
	return name
}

func firstNonEmpty(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func preferEstimateValue(primary, fallback string) string {
	primary = strings.TrimSpace(primary)
	if primary == "" || primary == "n/a" {
		return fallback
	}
	return primary
}

func selectTopStateValue(
	deviceReported string,
	state systemStateKind,
	ml batteryETAEstimates,
	heuristic batteryETAEstimates,
) string {
	deviceReported = strings.TrimSpace(deviceReported)
	if deviceReported == "" {
		deviceReported = "n/a"
	}
	if !isMLEstimateReady(ml) {
		return deviceReported
	}

	mlLow := isLowConfidence(ml.ConfidenceValue)
	heuristicLow := isLowConfidence(heuristic.ConfidenceValue)
	if mlLow && heuristicLow {
		return deviceReported
	}

	if confidenceTierFromValue(ml.ConfidenceValue) == "high" {
		if value := formatStateETAForDisplay(state, ml.ActiveValue); value != "n/a" {
			return value
		}
	}

	if value := formatStateETAForDisplay(state, heuristic.ActiveValue); value != "n/a" {
		return value
	}
	if value := formatStateETAForDisplay(state, ml.ActiveValue); value != "n/a" {
		return value
	}
	return deviceReported
}

func formatStateETAForDisplay(state systemStateKind, activeETA string) string {
	activeETA = strings.TrimSpace(activeETA)
	if activeETA == "" || activeETA == "n/a" {
		return "n/a"
	}
	prefix := ""
	switch state {
	case systemStateCharging:
		prefix = "charging"
	case systemStateDischarging:
		prefix = "discharging"
	case systemStateIdle:
		prefix = "idle"
	}
	if prefix == "" {
		return activeETA
	}
	return fmt.Sprintf("%s: %s", prefix, activeETA)
}

func isLowConfidence(value string) bool {
	return confidenceTierFromValue(value) == "low"
}

func isMLEstimateReady(ml batteryETAEstimates) bool {
	active := strings.TrimSpace(ml.ActiveValue)
	if active == "" || active == "n/a" {
		return false
	}
	return strings.Contains(strings.ToLower(strings.TrimSpace(ml.PowerValue)), "ewma+trend")
}

func confidenceTierFromValue(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" || value == "n/a" {
		return "low"
	}
	switch {
	case strings.Contains(value, "(high)"):
		return "high"
	case strings.Contains(value, "(medium)"):
		return "medium"
	case strings.Contains(value, "(low)"):
		return "low"
	default:
		return "low"
	}
}

func formatTypeWithState(typeCode string, systemState string) string {
	typeCode = strings.TrimSpace(typeCode)
	systemState = strings.TrimSpace(systemState)
	if typeCode == "" {
		typeCode = "n/a"
	}
	if systemState == "" || systemState == string(systemStateUnknown) {
		return typeCode
	}
	return typeCode + "/" + systemState
}

func formatPVInputRowLabel(channel string, device ecoflow.GeneralInfoDevice, snapshot *energySnapshot) string {
	channel = strings.ToLower(strings.TrimSpace(channel))
	if channel != "high" {
		channel = "low"
	}
	maxWatts, ok := estimatePVInputMaxWatts(channel, device, snapshot)
	if !ok {
		if channel == "high" {
			return "pv high"
		}
		return "pv low"
	}
	capLabel := formatPVCapacityWatts(maxWatts)
	otherChannel := "high"
	if channel == "high" {
		otherChannel = "low"
	}
	if otherWatts, otherOK := estimatePVInputMaxWatts(otherChannel, device, snapshot); otherOK && math.Abs(otherWatts-maxWatts) <= 0.5 {
		if channel == "high" {
			return fmt.Sprintf("solar #2 [%s]", capLabel)
		}
		return fmt.Sprintf("solar #1 [%s]", capLabel)
	}
	return fmt.Sprintf("solar [%s]", capLabel)
}

func estimatePVInputMaxWatts(channel string, device ecoflow.GeneralInfoDevice, snapshot *energySnapshot) (float64, bool) {
	channel = strings.ToLower(strings.TrimSpace(channel))
	if channel != "high" {
		channel = "low"
	}
	deviceName := strings.ToLower(strings.TrimSpace(device.DeviceName + " " + device.ProductName))
	isD2M := strings.Contains(deviceName, "delta 2 max")
	isDPU := strings.Contains(deviceName, "delta pro ultra") || strings.Contains(deviceName, "dpu")

	if isD2M {
		return 500, true
	}
	if isDPU {
		if channel == "high" {
			return 4000, true
		}
		return 1600, true
	}

	if snapshot != nil && snapshot.HasPVLowType && snapshot.HasPVHighType {
		if snapshot.PVLowType == 2 && snapshot.PVHighType == 2 {
			return 500, true
		}
		if snapshot.PVLowType == 2 && snapshot.PVHighType == 0 {
			if channel == "high" {
				return 4000, true
			}
			return 1600, true
		}
	}
	if snapshot != nil {
		var typeCode int64
		var hasType bool
		if channel == "high" {
			typeCode, hasType = snapshot.PVHighType, snapshot.HasPVHighType
		} else {
			typeCode, hasType = snapshot.PVLowType, snapshot.HasPVLowType
		}
		if hasType {
			if typeCode == 2 {
				return 500, true
			}
			if channel == "high" && typeCode == 0 {
				return 4000, true
			}
		}
	}
	return 0, false
}

func formatPVCapacityWatts(watts float64) string {
	if watts >= 1000 {
		kw := watts / 1000.0
		if math.Abs(kw-math.Round(kw)) <= 0.05 {
			return fmt.Sprintf("%.0fkW", math.Round(kw))
		}
		return fmt.Sprintf("%.1fkW", kw)
	}
	return fmt.Sprintf("%.0fW", watts)
}

func formatLastMQTTMeta(envelope telemetryEnvelope) string {
	parts := make([]string, 0, 4)
	if envelope.ID != 0 {
		parts = append(parts, fmt.Sprintf("id: %d", envelope.ID))
	}
	if envelope.CmdID != 0 || envelope.CmdFunc != 0 {
		parts = append(parts, fmt.Sprintf("cmd: %d/%d", envelope.CmdID, envelope.CmdFunc))
	}
	if addr := strings.TrimSpace(envelope.Addr); addr != "" {
		parts = append(parts, "addr: "+addr)
	}
	if envelope.Time != 0 {
		parts = append(parts, fmt.Sprintf("t: %d", envelope.Time))
	}
	if len(parts) == 0 {
		return "meta: n/a"
	}
	return strings.Join(parts, " ")
}

func packStateLabel(pack *packSnapshot) string {
	if !pack.HasPower {
		return "unknown"
	}
	if pack.PowerW > 0 {
		return "charging"
	}
	if pack.PowerW < 0 {
		return "discharging"
	}
	return "idle"
}

func formatPackPreconditioning(pack *packSnapshot) string {
	stateKnown := pack.HasPreconditioning
	on := pack.PreconditioningOn
	if !stateKnown && pack.HasPreconditioningState {
		stateKnown = true
		on = pack.PreconditioningStateRaw > 0
	}
	if !stateKnown && pack.HasPreconditioningHeat && pack.PreconditioningHeatTime > 0 {
		stateKnown = true
		on = true
	}
	if !stateKnown {
		return "n/a"
	}
	return checkboxStatus(on)
}

func overallPreconditioningStatus(packs map[int]*packSnapshot) (known bool, on bool) {
	if len(packs) == 0 {
		return false, false
	}
	for _, pack := range packs {
		if pack == nil {
			continue
		}
		packKnown := pack.HasPreconditioning || pack.HasPreconditioningState || pack.HasPreconditioningHeat
		if !packKnown {
			continue
		}
		known = true
		packOn := pack.PreconditioningOn
		if !pack.HasPreconditioning && pack.HasPreconditioningState {
			packOn = pack.PreconditioningStateRaw > 0
		}
		if !pack.HasPreconditioning && !pack.HasPreconditioningState && pack.HasPreconditioningHeat {
			packOn = pack.PreconditioningHeatTime > 0
		}
		if packOn {
			return true, true
		}
	}
	return known, false
}

func formatPackSerial(pack *packSnapshot) string {
	if pack == nil {
		return "n/a"
	}
	serial := strings.TrimSpace(pack.Serial)
	if serial == "" {
		return "n/a"
	}
	return serial
}

func formatPackEnergy(pack *packSnapshot) string {
	if pack == nil || !pack.HasEnergy {
		return "n/a"
	}
	return formatEnergyWh(pack.EnergyWh)
}

func formatPackSoh(pack *packSnapshot) string {
	if pack == nil {
		return "n/a"
	}
	if pack.HasActSOH && pack.HasSOH {
		return fmt.Sprintf("%.2f/%.1f%%", pack.ActSOH, pack.SOH)
	}
	if pack.HasActSOH {
		return fmt.Sprintf("%.2f%%", pack.ActSOH)
	}
	if pack.HasSOH {
		return fmt.Sprintf("%.1f%%", pack.SOH)
	}
	return "n/a"
}

func formatPackVoltage(pack *packSnapshot) string {
	if pack == nil || !pack.HasVoltage {
		return "n/a"
	}
	return fmt.Sprintf("%.3fV", pack.VoltageV)
}

func formatPackTarget(pack *packSnapshot) string {
	if pack == nil {
		return "n/a"
	}
	if !pack.HasTargetSOC {
		if pack.HasSOC {
			return fmt.Sprintf("%.2f%%", pack.SOC)
		}
		return "n/a"
	}
	return fmt.Sprintf("%.2f%%", pack.TargetSOC)
}

func formatPackLimits(pack *packSnapshot) string {
	if pack == nil {
		return "n/a"
	}
	if pack.HasMinSOC && pack.HasMaxSOC {
		return fmt.Sprintf("%.0f-%.0f%%", pack.MinSOC, pack.MaxSOC)
	}
	if pack.HasMinSOC {
		return fmt.Sprintf("min %.0f%%", pack.MinSOC)
	}
	if pack.HasMaxSOC {
		return fmt.Sprintf("max %.0f%%", pack.MaxSOC)
	}
	return "n/a"
}

func formatPackDiffSOC(pack *packSnapshot) string {
	if pack == nil || !pack.HasDiffSOC {
		return "n/a"
	}
	return fmt.Sprintf("%.2f%%", pack.DiffSOC)
}

func formatPackCapacity(pack *packSnapshot) string {
	if pack == nil {
		return "n/a"
	}
	switch {
	case pack.HasRemainCap && pack.HasFullCap:
		return fmt.Sprintf("%.0f/%.0f", pack.RemainCap, pack.FullCap)
	case pack.HasRemainCap:
		return fmt.Sprintf("%.0f", pack.RemainCap)
	case pack.HasFullCap:
		return fmt.Sprintf("n/a/%.0f", pack.FullCap)
	default:
		return "n/a"
	}
}

func formatPackBoardTemp(pack *packSnapshot) string {
	if pack == nil || !pack.HasBoardTemp {
		return "n/a"
	}
	return fmt.Sprintf("%.1fC", pack.BoardTempC)
}

func renderASCIITable(headers []string, rows [][]string) string {
	widths := make([]int, len(headers))
	for i, header := range headers {
		widths[i] = len(header)
	}
	for _, row := range rows {
		for i := 0; i < len(headers) && i < len(row); i++ {
			if len(row[i]) > widths[i] {
				widths[i] = len(row[i])
			}
		}
	}

	var builder strings.Builder
	builder.WriteString(renderTableBorder(widths))
	builder.WriteByte('\n')
	builder.WriteString(renderTableRow(headers, widths))
	builder.WriteByte('\n')
	builder.WriteString(renderTableBorder(widths))
	for _, row := range rows {
		builder.WriteByte('\n')
		builder.WriteString(renderTableRow(row, widths))
	}
	builder.WriteByte('\n')
	builder.WriteString(renderTableBorder(widths))
	return builder.String()
}

func renderTableBorder(widths []int) string {
	var builder strings.Builder
	builder.WriteByte('+')
	for _, width := range widths {
		builder.WriteString(strings.Repeat("-", width+2))
		builder.WriteByte('+')
	}
	return builder.String()
}

func renderTableRow(cells []string, widths []int) string {
	var builder strings.Builder
	builder.WriteByte('|')
	for i, width := range widths {
		cell := ""
		if i < len(cells) {
			cell = cells[i]
		}
		builder.WriteByte(' ')
		builder.WriteString(cell)
		if pad := width - len(cell); pad > 0 {
			builder.WriteString(strings.Repeat(" ", pad))
		}
		builder.WriteByte(' ')
		builder.WriteByte('|')
	}
	return builder.String()
}

func buildMinuteTelemetryRows(history *minuteTelemetryHistory, cfg minuteTableConfig) [][]string {
	rows := cfg.Rows
	if rows <= 0 {
		rows = defaultMinuteTableRows
	}
	if history == nil {
		return nil
	}
	buckets := history.SortedBuckets(cfg.NewestFirst, rows)
	if len(buckets) == 0 {
		return nil
	}
	out := make([][]string, 0, len(buckets))
	for _, bucket := range buckets {
		solarWh, hasSolarWh := averageWh(bucket.SolarSumWatts, bucket.SolarSamples)
		acInWh, hasACInWh := averageWh(bucket.ACInSumWatts, bucket.ACInSamples)
		acOutWh, hasACOutWh := averageWh(bucket.ACOutSumWatts, bucket.ACOutSamples)
		dcOutWh, hasDCOutWh := averageWh(bucket.DCOutSumWatts, bucket.DCOutSamples)
		batteryChargeWh, hasBatteryChargeWh := averageWh(bucket.BatteryChargeSumWatts, bucket.BatteryChargeSamples)

		totalInWh := 0.0
		hasTotalInWh := false
		if hasSolarWh {
			totalInWh += solarWh
			hasTotalInWh = true
		}
		if hasACInWh {
			totalInWh += acInWh
			hasTotalInWh = true
		}

		totalOutWh := 0.0
		hasTotalOutWh := false
		if hasACOutWh {
			totalOutWh += acOutWh
			hasTotalOutWh = true
		}
		if hasDCOutWh {
			totalOutWh += dcOutWh
			hasTotalOutWh = true
		}
		netWh := totalInWh - totalOutWh
		hasNetWh := hasTotalInWh || hasTotalOutWh

		out = append(out, []string{
			time.Unix(bucket.MinuteStartUnix, 0).Local().Format("2006-01-02 15:04"),
			formatAverageWhNoUnit(bucket.SolarSumWatts, bucket.SolarSamples),
			formatAverageWhNoUnit(bucket.ACInSumWatts, bucket.ACInSamples),
			formatAverageWhNoUnit(bucket.ACOutSumWatts, bucket.ACOutSamples),
			formatAverageWhNoUnit(bucket.DCOutSumWatts, bucket.DCOutSamples),
			formatWhNoUnit(batteryChargeWh, hasBatteryChargeWh),
			formatWhNoUnit(totalInWh, hasTotalInWh),
			formatWhNoUnit(totalOutWh, hasTotalOutWh),
			formatWhNoUnit(netWh, hasNetWh),
		})
	}
	return out
}

func (s *energySnapshot) effectiveBatteryChargeWatts() (float64, bool) {
	if s == nil {
		return 0, false
	}

	bestChargeWatts := 0.0
	hasSignal := false

	// Start with explicit system totals when both are present.
	if s.HasWattsIn && s.HasWattsOut {
		hasSignal = true
		netChargeWatts := s.WattsIn - s.WattsOut
		if netChargeWatts > idleDrawNoiseFloorWatts {
			bestChargeWatts = netChargeWatts
		}
	}

	// Fall back to channel totals (input sources minus external outputs).
	totalInputWatts := 0.0
	hasInput := false
	if s.HasInAC {
		totalInputWatts += s.InACWatts
		hasInput = true
	}
	if pvInputWatts, hasPVInput := s.effectivePVInputWatts(); hasPVInput {
		totalInputWatts += pvInputWatts
		hasInput = true
	}

	totalExternalOutputWatts := 0.0
	hasExternalOutput := false
	if s.HasOutAC {
		totalExternalOutputWatts += s.OutACWatts
		hasExternalOutput = true
	}
	if s.HasOutDC {
		totalExternalOutputWatts += s.OutDCWatts
		hasExternalOutput = true
	}

	if hasInput && hasExternalOutput {
		hasSignal = true
		netChargeWatts := totalInputWatts - totalExternalOutputWatts
		if netChargeWatts > idleDrawNoiseFloorWatts && netChargeWatts > bestChargeWatts {
			bestChargeWatts = netChargeWatts
		}
	}

	// Pack totals can be more complete than system net on Delta 2 style payloads,
	// where internal XT150 transfer may appear in wattsOutSum.
	packChargeWatts, _ := packPowerTotals(s.Packs)
	if packChargeWatts > idleDrawNoiseFloorWatts {
		if sanitized, ok := s.sanitizeBatteryFlowHintWatts(packChargeWatts); ok {
			hasSignal = true
			if sanitized > bestChargeWatts {
				bestChargeWatts = sanitized
			}
		}
	}

	// Last-resort fallback to direct battery hints.
	if s.HasBatteryIn && s.BatteryInWatts > idleDrawNoiseFloorWatts {
		if sanitized, ok := s.sanitizeBatteryFlowHintWatts(s.BatteryInWatts); ok {
			hasSignal = true
			if sanitized > bestChargeWatts {
				bestChargeWatts = sanitized
			}
		}
	}
	if s.HasXT150 && s.XT150Watts > idleDrawNoiseFloorWatts {
		if sanitized, ok := s.sanitizeBatteryFlowHintWatts(s.XT150Watts); ok {
			hasSignal = true
			if sanitized > bestChargeWatts {
				bestChargeWatts = sanitized
			}
		}
	}
	if !hasSignal {
		return 0, false
	}
	if bestChargeWatts <= idleDrawNoiseFloorWatts {
		return 0, true
	}
	return bestChargeWatts, true
}

func formatAverageWhNoUnit(sum float64, samples int) string {
	wh, ok := averageWh(sum, samples)
	if !ok {
		return "n/a"
	}
	return fmt.Sprintf("%.1f", wh)
}

func averageWh(sum float64, samples int) (float64, bool) {
	if samples <= 0 {
		return 0, false
	}
	avgWatts := sum / float64(samples)
	return avgWatts / 60.0, true
}

func formatWhNoUnit(value float64, ok bool) string {
	if !ok {
		return "n/a"
	}
	return fmt.Sprintf("%.1f", value)
}

func extractBatteryPacks(quota map[string]any) map[int]packSnapshot {
	out := make(map[int]packSnapshot)
	for key, raw := range quota {
		lower := strings.ToLower(key)
		if lower != "bpinfo" && !strings.HasSuffix(lower, ".bpinfo") {
			continue
		}
		entries, ok := parseAnyArrayFromAny(raw)
		if !ok {
			continue
		}
		for i, entryRaw := range entries {
			entry, ok := entryRaw.(map[string]any)
			if !ok {
				continue
			}
			bpNo := int(toInt64(entry["bpNo"]))
			if bpNo <= 0 {
				bpNo = i + 1
			}
			pack := out[bpNo]
			if soc, ok := numberFromAny(entry["bpSoc"]); ok {
				pack.SOC = soc
				pack.HasSOC = true
			}
			if temp, ok := numberFromAny(entry["bpTemp"]); ok {
				pack.TempC = temp
				pack.HasTemp = true
			}
			if pwr, ok := numberFromAny(entry["bpPwr"]); ok {
				// Ignore zero-only bpInfo updates because they frequently represent sparse snapshots
				// and can overwrite useful non-zero power from other telemetry frames.
				if pwr != 0 {
					pack.PowerW = pwr
					pack.HasPower = true
				}
			}
			if energy, ok := numberFromAny(entry["bpEnergy"]); ok {
				pack.EnergyWh = energy
				pack.HasEnergy = true
			}
			if maxSoc, ok := numberFromAny(entry["bpSocMax"]); ok && maxSoc >= 0 {
				pack.MaxSOC = maxSoc
				pack.HasMaxSOC = true
			}
			if minSoc, ok := numberFromAny(entry["bpSocMin"]); ok && minSoc >= 0 {
				pack.MinSOC = minSoc
				pack.HasMinSOC = true
			}
			if maxVolDiff, ok := numberFromAny(entry["maxVolDiff"]); ok && maxVolDiff >= 0 {
				pack.MaxVolDiff = maxVolDiff
				pack.HasMaxVolDiff = true
			}
			if heat := toInt64(entry["heatTime"]); heat >= 0 {
				pack.PreconditioningHeatTime = heat
				pack.HasPreconditioningHeat = true
				// Some bpInfo payloads only expose heatTime; use it as fallback ON/OFF state.
				if !pack.HasPreconditioningState {
					pack.PreconditioningOn = heat > 0
					pack.HasPreconditioning = true
				}
			}
			if stateRaw, ok := numberFromAny(entry["ptcMosState"]); ok {
				state := int64(stateRaw)
				pack.PreconditioningStateRaw = state
				pack.HasPreconditioningState = true
				pack.PreconditioningOn = state > 0
				pack.HasPreconditioning = true
			}
			if eventRaw, ok := numberFromAny(entry["ptcHeatingEvent"]); ok {
				event := int64(eventRaw)
				pack.PreconditioningEventRaw = event
				pack.HasPreconditioningEvent = true
				if event > 0 {
					pack.PreconditioningOn = true
					pack.HasPreconditioning = true
				}
			}
			if remain := toInt64(entry["remainTime"]); remain > 0 && !isLikelyRemainSentinel(remain) {
				pack.RemainTimeRaw = remain
			}
			out[bpNo] = pack
		}
	}
	return out
}

func parseAnyArrayFromAny(value any) ([]any, bool) {
	switch v := value.(type) {
	case []any:
		return v, true
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return nil, false
		}
		var decoded []any
		if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
			return nil, false
		}
		return decoded, true
	default:
		return nil, false
	}
}

func formatPackSummary(packs map[int]*packSnapshot, kitSOC map[string]float64) string {
	if len(packs) > 0 {
		ids := make([]int, 0, len(packs))
		for id := range packs {
			ids = append(ids, id)
		}
		sort.Ints(ids)
		parts := make([]string, 0, len(ids))
		for _, id := range ids {
			pack := packs[id]
			label := fmt.Sprintf("bp%d", id)
			if pack.HasSOC {
				label += fmt.Sprintf("=%.1f%%", pack.SOC)
			}
			if pack.HasTemp {
				label += fmt.Sprintf("(%.1fC)", pack.TempC)
			}
			if pack.HasPower {
				label += fmt.Sprintf("[%s]", formatWatts(pack.PowerW))
			}
			parts = append(parts, label)
		}
		return strings.Join(parts, ",")
	}

	if len(kitSOC) > 0 {
		keys := make([]string, 0, len(kitSOC))
		for key := range kitSOC {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, key := range keys {
			parts = append(parts, fmt.Sprintf("%s=%.1f%%", key, kitSOC[key]))
		}
		return strings.Join(parts, ",")
	}

	return "n/a"
}

func formatTemperatureSummary(temps map[string]float64, limit int) string {
	if len(temps) == 0 {
		return "n/a"
	}
	keys := make([]string, 0, len(temps))
	for key := range temps {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if limit > 0 && len(keys) > limit {
		keys = keys[:limit]
	}
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%.1fC", key, temps[key]))
	}
	return strings.Join(parts, ",")
}

func estimatedWhLeft(packs map[int]*packSnapshot) float64 {
	total := 0.0
	for _, pack := range packs {
		if !pack.HasEnergy || !pack.HasSOC {
			continue
		}
		total += pack.EnergyWh * (pack.SOC / 100.0)
	}
	return total
}

type batteryETAEstimates struct {
	ChargeValue     string
	DischargeValue  string
	ActiveValue     string
	PowerValue      string
	ConfidenceValue string
}

func (s *energySnapshot) estimateBatteryETAs(
	state systemStateKind,
	batteryInWatts float64,
	hasBatteryIn bool,
	batteryOutWatts float64,
	hasBatteryOut bool,
	effectiveIn float64,
	hasEffectiveIn bool,
	effectiveOut float64,
	hasEffectiveOut bool,
) batteryETAEstimates {
	estimates := batteryETAEstimates{
		ChargeValue:     "n/a",
		DischargeValue:  "n/a",
		ActiveValue:     "n/a",
		PowerValue:      "power: n/a",
		ConfidenceValue: "n/a",
	}

	energyToChargeWh, energyToDischargeWh, ok := s.energyToTargetsWh()
	if !ok {
		return estimates
	}

	// Prefer net system power when available; raw battery amp/vol telemetry can be scaled
	// inconsistently on some payloads and produce impossible ETA rates.
	chargePowerW := 0.0
	hasChargePower := false
	chargePowerSource := ""
	if hasEffectiveIn && hasEffectiveOut {
		netPowerW := effectiveIn - effectiveOut
		if netPowerW > systemStateNetThresholdWatts {
			chargePowerW = netPowerW
			hasChargePower = true
			chargePowerSource = "net"
		}
	}
	if packChargeW, _ := packPowerTotals(s.Packs); packChargeW > idleDrawNoiseFloorWatts {
		if sanitized, ok := s.sanitizeBatteryFlowHintWatts(packChargeW); ok {
			if !hasChargePower || sanitized > chargePowerW {
				chargePowerW = sanitized
				hasChargePower = true
				chargePowerSource = "pack"
			}
		}
	}
	if !hasChargePower && hasBatteryIn && batteryInWatts > idleDrawNoiseFloorWatts {
		if sanitized, ok := s.sanitizeBatteryFlowHintWatts(batteryInWatts); ok {
			chargePowerW = sanitized
			hasChargePower = true
			chargePowerSource = "hint"
		}
	}

	dischargePowerW := 0.0
	hasDischargePower := false
	dischargePowerSource := ""
	if hasEffectiveIn && hasEffectiveOut {
		netPowerW := effectiveOut - effectiveIn
		if netPowerW > systemStateNetThresholdWatts {
			dischargePowerW = netPowerW
			hasDischargePower = true
			dischargePowerSource = "net"
		}
	}
	if _, packDischargeW := packPowerTotals(s.Packs); packDischargeW > idleDrawNoiseFloorWatts {
		if sanitized, ok := s.sanitizeBatteryFlowHintWatts(packDischargeW); ok {
			if !hasDischargePower || sanitized > dischargePowerW {
				dischargePowerW = sanitized
				hasDischargePower = true
				dischargePowerSource = "pack"
			}
		}
	}
	if !hasDischargePower && hasBatteryOut && batteryOutWatts > idleDrawNoiseFloorWatts {
		if sanitized, ok := s.sanitizeBatteryFlowHintWatts(batteryOutWatts); ok {
			dischargePowerW = sanitized
			hasDischargePower = true
			dischargePowerSource = "hint"
		}
	}

	if hasChargePower {
		if energyToChargeWh <= 0 {
			estimates.ChargeValue = "0min (~0m)"
		} else {
			etaChargeMin := energyToChargeWh * 60.0 / chargePowerW
			estimates.ChargeValue = formatETAMinutes(etaChargeMin)
		}
	}
	if hasDischargePower {
		if energyToDischargeWh <= 0 {
			estimates.DischargeValue = "0min (~0m)"
		} else {
			etaDischargeMin := energyToDischargeWh * 60.0 / dischargePowerW
			estimates.DischargeValue = formatETAMinutes(etaDischargeMin)
		}
	}

	switch {
	case hasChargePower && hasDischargePower:
		estimates.PowerValue = fmt.Sprintf("power: chg@%s dsg@%s", formatWatts(chargePowerW), formatWatts(dischargePowerW))
	case hasChargePower:
		estimates.PowerValue = fmt.Sprintf("power: chg@%s", formatWatts(chargePowerW))
	case hasDischargePower:
		estimates.PowerValue = fmt.Sprintf("power: dsg@%s", formatWatts(dischargePowerW))
	}

	switch state {
	case systemStateCharging:
		estimates.ActiveValue = estimates.ChargeValue
	case systemStateDischarging:
		estimates.ActiveValue = estimates.DischargeValue
	default:
		estimates.ActiveValue = "n/a"
	}

	confidenceScore := 0.0
	hasConfidence := false
	switch state {
	case systemStateCharging:
		if hasChargePower {
			confidenceScore = etaConfidenceScoreForSource(chargePowerSource)
			hasConfidence = true
			if hasEffectiveIn && hasEffectiveOut {
				confidenceScore += 0.08
			}
			if chargePowerW <= 20 {
				confidenceScore -= 0.08
			}
		}
	case systemStateDischarging:
		if hasDischargePower {
			confidenceScore = etaConfidenceScoreForSource(dischargePowerSource)
			hasConfidence = true
			if hasEffectiveIn && hasEffectiveOut {
				confidenceScore += 0.08
			}
			if dischargePowerW <= 20 {
				confidenceScore -= 0.08
			}
		}
	default:
		// When state is unknown, confidence is inherently lower even if we have power.
		if hasChargePower || hasDischargePower {
			confidenceScore = 0.45
			hasConfidence = true
		}
	}
	estimates.ConfidenceValue = formatConfidenceValue(confidenceScore, hasConfidence)

	return estimates
}

func (s *energySnapshot) energyToTargetsWh() (energyToChargeWh, energyToDischargeWh float64, ok bool) {
	if s == nil {
		return 0, 0, false
	}
	capacityWh, hasCapacity := s.estimatedTotalCapacityWh()
	if !hasCapacity || capacityWh <= 0 {
		return 0, 0, false
	}
	remainingWh, hasRemaining := s.estimatedRemainingEnergyWh()
	if !hasRemaining || remainingWh < 0 {
		return 0, 0, false
	}

	minSOC, maxSOC := 0.0, 100.0
	if s.HasMinDischarge {
		minSOC = clampPercent(s.MinDischargeSOC)
	}
	if s.HasMaxChargeSOC {
		maxSOC = clampPercent(s.MaxChargeSOC)
	}
	if maxSOC < minSOC {
		minSOC, maxSOC = 0, 100
	}

	targetChargeWh := capacityWh * (maxSOC / 100.0)
	targetDischargeWh := capacityWh * (minSOC / 100.0)

	energyToChargeWh = targetChargeWh - remainingWh
	if energyToChargeWh < 0 {
		energyToChargeWh = 0
	}
	energyToDischargeWh = remainingWh - targetDischargeWh
	if energyToDischargeWh < 0 {
		energyToDischargeWh = 0
	}
	return energyToChargeWh, energyToDischargeWh, true
}

func estimateBatteryETAsML(snapshot *energySnapshot, history *minuteTelemetryHistory, state systemStateKind) batteryETAEstimates {
	estimates := batteryETAEstimates{
		ChargeValue:     "n/a",
		DischargeValue:  "n/a",
		ActiveValue:     "n/a",
		PowerValue:      "power: n/a",
		ConfidenceValue: "n/a",
	}
	if snapshot == nil {
		return estimates
	}
	energyToChargeWh, energyToDischargeWh, ok := snapshot.energyToTargetsWh()
	if !ok {
		return estimates
	}

	samples := netPowerSamplesFromMinuteHistory(history, 24)
	if len(samples) < 3 {
		return estimates
	}
	predNetW, meanNetW, stdNetW, ok := predictNetPowerEWMATrend(samples)
	if !ok {
		return estimates
	}

	chargePowerW := 0.0
	hasChargePower := false
	if predNetW > systemStateNetThresholdWatts {
		chargePowerW = predNetW
		hasChargePower = true
	} else if state == systemStateCharging && meanNetW > systemStateNetThresholdWatts {
		chargePowerW = meanNetW
		hasChargePower = true
	}
	if hasChargePower {
		if sanitized, ok := snapshot.sanitizeBatteryFlowHintWatts(chargePowerW); ok {
			chargePowerW = sanitized
		} else {
			hasChargePower = false
		}
	}

	dischargePowerW := 0.0
	hasDischargePower := false
	if predNetW < -systemStateNetThresholdWatts {
		dischargePowerW = -predNetW
		hasDischargePower = true
	} else if state == systemStateDischarging && meanNetW < -systemStateNetThresholdWatts {
		dischargePowerW = -meanNetW
		hasDischargePower = true
	}
	if hasDischargePower {
		if sanitized, ok := snapshot.sanitizeBatteryFlowHintWatts(dischargePowerW); ok {
			dischargePowerW = sanitized
		} else {
			hasDischargePower = false
		}
	}

	if hasChargePower {
		if energyToChargeWh <= 0 {
			estimates.ChargeValue = "0min (~0m)"
		} else {
			etaChargeMin := energyToChargeWh * 60.0 / chargePowerW
			estimates.ChargeValue = formatETAMinutes(etaChargeMin)
		}
	}
	if hasDischargePower {
		if energyToDischargeWh <= 0 {
			estimates.DischargeValue = "0min (~0m)"
		} else {
			etaDischargeMin := energyToDischargeWh * 60.0 / dischargePowerW
			estimates.DischargeValue = formatETAMinutes(etaDischargeMin)
		}
	}

	switch {
	case hasChargePower && hasDischargePower:
		estimates.PowerValue = fmt.Sprintf("power: chg@%s dsg@%s (ewma+trend)", formatWatts(chargePowerW), formatWatts(dischargePowerW))
	case hasChargePower:
		estimates.PowerValue = fmt.Sprintf("power: chg@%s (ewma+trend)", formatWatts(chargePowerW))
	case hasDischargePower:
		estimates.PowerValue = fmt.Sprintf("power: dsg@%s (ewma+trend)", formatWatts(dischargePowerW))
	}

	switch state {
	case systemStateCharging:
		estimates.ActiveValue = estimates.ChargeValue
	case systemStateDischarging:
		estimates.ActiveValue = estimates.DischargeValue
	default:
		estimates.ActiveValue = "n/a"
	}

	signMatchRatio := 0.0
	switch state {
	case systemStateCharging:
		signMatchRatio = signDirectionMatchRatio(samples, true)
	case systemStateDischarging:
		signMatchRatio = signDirectionMatchRatio(samples, false)
	default:
		signMatchRatio = 0.5
	}
	sampleScore := math.Min(float64(len(samples))/12.0, 1.0) * 0.35
	stabilityScore := 0.0
	if math.Abs(meanNetW) > systemStateNetThresholdWatts {
		cv := stdNetW / math.Abs(meanNetW)
		if cv < 0 {
			cv = 0
		}
		if cv > 1 {
			cv = 1
		}
		stabilityScore = (1 - cv) * 0.3
	}
	confidenceScore := 0.2 + sampleScore + (signMatchRatio * 0.25) + stabilityScore
	if !hasChargePower && !hasDischargePower {
		confidenceScore -= 0.2
	}
	if state == systemStateUnknown {
		confidenceScore -= 0.1
	}
	estimates.ConfidenceValue = formatConfidenceValue(confidenceScore, true)

	return estimates
}

func netPowerSamplesFromMinuteHistory(history *minuteTelemetryHistory, limit int) []float64 {
	if history == nil {
		return nil
	}
	buckets := history.SortedBuckets(false, limit)
	if len(buckets) == 0 {
		return nil
	}
	out := make([]float64, 0, len(buckets))
	for _, bucket := range buckets {
		inW := 0.0
		hasIn := false
		if bucket.SolarSamples > 0 {
			inW += bucket.SolarSumWatts / float64(bucket.SolarSamples)
			hasIn = true
		}
		if bucket.ACInSamples > 0 {
			inW += bucket.ACInSumWatts / float64(bucket.ACInSamples)
			hasIn = true
		}

		outW := 0.0
		hasOut := false
		if bucket.ACOutSamples > 0 {
			outW += bucket.ACOutSumWatts / float64(bucket.ACOutSamples)
			hasOut = true
		}
		if bucket.DCOutSamples > 0 {
			outW += bucket.DCOutSumWatts / float64(bucket.DCOutSamples)
			hasOut = true
		}
		if !hasIn && !hasOut {
			continue
		}
		out = append(out, inW-outW)
	}
	return out
}

func predictNetPowerEWMATrend(samples []float64) (pred float64, mean float64, std float64, ok bool) {
	if len(samples) < 2 {
		return 0, 0, 0, false
	}
	const alpha = 0.35
	ewma := samples[0]
	for i := 1; i < len(samples); i++ {
		ewma = (alpha * samples[i]) + ((1 - alpha) * ewma)
	}

	n := float64(len(samples))
	xMean := (n - 1) / 2
	yMean := 0.0
	for _, sample := range samples {
		yMean += sample
	}
	yMean /= n

	num := 0.0
	den := 0.0
	for i, sample := range samples {
		x := float64(i) - xMean
		y := sample - yMean
		num += x * y
		den += x * x
	}
	slope := 0.0
	if den > 0 {
		slope = num / den
	}
	pred = ewma + slope
	if math.Abs(pred) < systemStateNetThresholdWatts && math.Abs(ewma) >= systemStateNetThresholdWatts {
		pred = ewma
	}

	variance := 0.0
	for _, sample := range samples {
		delta := sample - yMean
		variance += delta * delta
	}
	variance /= n
	std = math.Sqrt(variance)
	return pred, yMean, std, true
}

func signDirectionMatchRatio(samples []float64, charging bool) float64 {
	if len(samples) == 0 {
		return 0
	}
	match := 0
	for _, sample := range samples {
		if charging {
			if sample > systemStateNetThresholdWatts {
				match++
			}
		} else if sample < -systemStateNetThresholdWatts {
			match++
		}
	}
	return float64(match) / float64(len(samples))
}

func (s *energySnapshot) estimatedTotalCapacityWh() (float64, bool) {
	if s == nil {
		return 0, false
	}
	if s.HasFullEnergy && s.FullEnergyWh > 0 {
		return s.FullEnergyWh, true
	}
	total := 0.0
	count := 0
	for _, pack := range s.Packs {
		if packWh, ok := estimatedPackCapacityWh(pack); ok {
			total += packWh
			count++
		}
	}
	if count == 0 || total <= 0 {
		return 0, false
	}
	return total, true
}

func (s *energySnapshot) estimatedRemainingEnergyWh() (float64, bool) {
	if s == nil {
		return 0, false
	}
	total := 0.0
	count := 0
	for _, pack := range s.Packs {
		if packWh, ok := estimatedPackRemainingWh(pack); ok {
			total += packWh
			count++
		}
	}
	if count > 0 && total >= 0 {
		return total, true
	}

	capacityWh, hasCapacity := s.estimatedTotalCapacityWh()
	if !hasCapacity || capacityWh <= 0 {
		return 0, false
	}
	if soc, ok := s.displaySOC(); ok {
		return capacityWh * (clampPercent(soc) / 100.0), true
	}
	return 0, false
}

func estimatedPackCapacityWh(pack *packSnapshot) (float64, bool) {
	if pack == nil {
		return 0, false
	}
	if pack.HasEnergy && pack.EnergyWh > 0 {
		return pack.EnergyWh, true
	}
	if wh, ok := capacityToWh(pack.FullCap, pack.HasFullCap, pack.VoltageV, pack.HasVoltage); ok {
		return wh, true
	}
	if wh, ok := capacityToWh(pack.DesignCap, pack.HasDesignCap, pack.VoltageV, pack.HasVoltage); ok {
		return wh, true
	}
	return 0, false
}

func estimatedPackRemainingWh(pack *packSnapshot) (float64, bool) {
	if pack == nil {
		return 0, false
	}
	if wh, ok := capacityToWh(pack.RemainCap, pack.HasRemainCap, pack.VoltageV, pack.HasVoltage); ok {
		return wh, true
	}
	if capWh, ok := estimatedPackCapacityWh(pack); ok && pack.HasSOC {
		return capWh * (clampPercent(pack.SOC) / 100.0), true
	}
	return 0, false
}

func capacityToWh(capacity float64, hasCapacity bool, voltage float64, hasVoltage bool) (float64, bool) {
	if !hasCapacity || !hasVoltage || capacity <= 0 || voltage <= 0 {
		return 0, false
	}
	ampHours := capacity
	// EcoFlow capacities are typically published in mAh; convert when value is clearly in that scale.
	if ampHours > 500 {
		ampHours = ampHours / 1000.0
	}
	wh := ampHours * voltage
	if wh <= 0 || wh > 100000 {
		return 0, false
	}
	return wh, true
}

func (s *energySnapshot) maxReasonableBatteryFlowHintWatts() (float64, bool) {
	if s == nil {
		return 0, false
	}
	maxWatts := 0.0
	if s.HasParaChgMax && s.ParaChgMaxWatts > 0 {
		maxWatts = math.Max(maxWatts, s.ParaChgMaxWatts*1.15)
	}
	if s.HasC20ChgMax && s.C20ChgMaxWatts > 0 {
		// Dual charge on D2M can exceed C20 AC-only limit.
		maxWatts = math.Max(maxWatts, s.C20ChgMaxWatts*1.6)
	}
	if s.HasInAC || s.HasInPV {
		totalIn := 0.0
		if s.HasInAC {
			totalIn += s.InACWatts
		}
		if pvWatts, ok := s.effectivePVInputWatts(); ok {
			totalIn += pvWatts
		}
		if totalIn > 0 {
			maxWatts = math.Max(maxWatts, totalIn*1.6)
		}
	}
	if s.HasOutAC || s.HasOutDC {
		totalOut := 0.0
		if s.HasOutAC {
			totalOut += s.OutACWatts
		}
		if s.HasOutDC {
			totalOut += s.OutDCWatts
		}
		if totalOut > 0 {
			maxWatts = math.Max(maxWatts, totalOut*2.0)
		}
	}
	if maxWatts <= 0 {
		return 0, false
	}
	return maxWatts, true
}

func (s *energySnapshot) sanitizeBatteryFlowHintWatts(value float64) (float64, bool) {
	if value <= idleDrawNoiseFloorWatts || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	if maxWatts, ok := s.maxReasonableBatteryFlowHintWatts(); ok && maxWatts > 0 {
		if value > maxWatts {
			return 0, false
		}
	}
	return value, true
}

func etaConfidenceScoreForSource(source string) float64 {
	switch source {
	case "net":
		return 0.88
	case "pack":
		return 0.8
	case "hint":
		return 0.65
	default:
		return 0
	}
}

func formatConfidenceValue(score float64, ok bool) string {
	if !ok || math.IsNaN(score) || math.IsInf(score, 0) {
		return "n/a"
	}
	if score < 0 {
		score = 0
	}
	if score > 0.99 {
		score = 0.99
	}
	return fmt.Sprintf("%.2f (%s)", score, confidenceTier(score))
}

func confidenceTier(score float64) string {
	switch {
	case score >= 0.8:
		return "high"
	case score >= 0.6:
		return "medium"
	default:
		return "low"
	}
}

func clampPercent(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func formatETAMinutes(minutes float64) string {
	if minutes <= 0 || math.IsNaN(minutes) || math.IsInf(minutes, 0) {
		return "n/a"
	}
	rounded := int64(math.Round(minutes))
	if rounded < 1 {
		rounded = 1
	}
	return fmt.Sprintf("%dmin (~%s)", rounded, formatMinutesHuman(rounded))
}

func packPowerTotals(packs map[int]*packSnapshot) (chargeW float64, dischargeW float64) {
	for _, pack := range packs {
		if !pack.HasPower {
			continue
		}
		if pack.PowerW > 0 {
			chargeW += pack.PowerW
			continue
		}
		if pack.PowerW < 0 {
			dischargeW += -pack.PowerW
		}
	}
	return chargeW, dischargeW
}

func (s *energySnapshot) effectiveTotalsForDisplay() (effectiveIn float64, hasEffectiveIn bool, effectiveOut float64, hasEffectiveOut bool) {
	packChargeW, packDischargeW := packPowerTotals(s.Packs)
	return s.effectiveTotalsForDisplayWithPackTotals(packChargeW, packDischargeW)
}

func (s *energySnapshot) effectiveTotalsForDisplayWithPackTotals(packChargeW, packDischargeW float64) (effectiveIn float64, hasEffectiveIn bool, effectiveOut float64, hasEffectiveOut bool) {
	effectiveIn = s.WattsIn
	hasEffectiveIn = s.HasWattsIn
	if (!hasEffectiveIn || effectiveIn == 0) && packChargeW > 0 {
		effectiveIn = packChargeW
		hasEffectiveIn = true
	}

	effectiveOut = s.WattsOut
	hasEffectiveOut = s.HasWattsOut
	if (!hasEffectiveOut || effectiveOut == 0) && packDischargeW > 0 {
		effectiveOut = packDischargeW
		hasEffectiveOut = true
	}
	return effectiveIn, hasEffectiveIn, effectiveOut, hasEffectiveOut
}

func (s *energySnapshot) detectSystemState(
	effectiveIn float64,
	hasEffectiveIn bool,
	effectiveOut float64,
	hasEffectiveOut bool,
	packChargeW float64,
	packDischargeW float64,
) systemStateKind {
	if hasEffectiveIn && hasEffectiveOut {
		net := effectiveIn - effectiveOut
		switch {
		case net > systemStateNetThresholdWatts:
			return systemStateCharging
		case net < -systemStateNetThresholdWatts:
			return systemStateDischarging
		case effectiveIn <= systemStateNetThresholdWatts && effectiveOut <= systemStateNetThresholdWatts:
			return systemStateIdle
		}
	} else if hasEffectiveIn {
		if effectiveIn > systemStateNetThresholdWatts {
			return systemStateCharging
		}
	} else if hasEffectiveOut {
		if effectiveOut > systemStateNetThresholdWatts {
			return systemStateDischarging
		}
	}

	hasBatteryIn := s.HasBatteryIn && s.BatteryInWatts > 0
	hasBatteryOut := s.HasBatteryOut && s.BatteryOutWatts > 0
	switch {
	case hasBatteryIn && (!hasBatteryOut || s.BatteryInWatts > s.BatteryOutWatts+systemStateNetThresholdWatts):
		return systemStateCharging
	case hasBatteryOut && (!hasBatteryIn || s.BatteryOutWatts > s.BatteryInWatts+systemStateNetThresholdWatts):
		return systemStateDischarging
	}

	switch {
	case packChargeW > packDischargeW+systemStateNetThresholdWatts:
		return systemStateCharging
	case packDischargeW > packChargeW+systemStateNetThresholdWatts:
		return systemStateDischarging
	case packChargeW <= systemStateNetThresholdWatts && packDischargeW <= systemStateNetThresholdWatts && (packChargeW > 0 || packDischargeW > 0):
		return systemStateIdle
	}

	if inferred := inferStateFromChgDsgState(s.ChgDsgStateRaw, s.HasChgDsgState, s.HasChargeRemainTime, s.HasDischargeRemainTime); inferred != systemStateUnknown {
		return inferred
	}
	return systemStateUnknown
}

func inferStateFromChgDsgState(raw int64, hasRaw bool, hasChargeRemain bool, hasDischargeRemain bool) systemStateKind {
	if !hasRaw {
		return systemStateUnknown
	}
	switch raw {
	case 0:
		return systemStateIdle
	case 1:
		if hasDischargeRemain && !hasChargeRemain {
			return systemStateDischarging
		}
		if hasChargeRemain && !hasDischargeRemain {
			return systemStateCharging
		}
	case 2:
		if hasChargeRemain && !hasDischargeRemain {
			return systemStateCharging
		}
		if hasDischargeRemain && !hasChargeRemain {
			return systemStateDischarging
		}
	}
	return systemStateUnknown
}

func (s *energySnapshot) selectRemainForState(state systemStateKind) (minutes int64, source string, ok bool) {
	hasGlobal := s.HasRemainTime && s.RemainTimeRaw > 0 && !isLikelyRemainSentinel(s.RemainTimeRaw)
	hasCharge := s.HasChargeRemainTime && s.ChargeRemainTimeRaw > 0 && !isLikelyRemainSentinel(s.ChargeRemainTimeRaw)
	hasDischarge := s.HasDischargeRemainTime && s.DischargeRemainTimeRaw > 0 && !isLikelyRemainSentinel(s.DischargeRemainTimeRaw)

	switch state {
	case systemStateCharging:
		if hasCharge {
			return s.ChargeRemainTimeRaw, "charge", true
		}
		if hasGlobal {
			return s.RemainTimeRaw, "global", true
		}
	case systemStateDischarging:
		if hasDischarge {
			return s.DischargeRemainTimeRaw, "discharge", true
		}
		if hasGlobal {
			return s.RemainTimeRaw, "global", true
		}
	case systemStateIdle:
		if hasGlobal {
			return s.RemainTimeRaw, "global", true
		}
	default:
		if hasGlobal {
			return s.RemainTimeRaw, "global", true
		}
	}

	// Fallback when state is unavailable but one directional remain exists.
	if hasCharge && !hasDischarge {
		return s.ChargeRemainTimeRaw, "charge", true
	}
	if hasDischarge && !hasCharge {
		return s.DischargeRemainTimeRaw, "discharge", true
	}
	return 0, "", false
}

func formatRemainValueWithState(minutes int64, state systemStateKind, source string) string {
	if minutes <= 0 {
		return "n/a"
	}
	prefix := ""
	switch state {
	case systemStateCharging:
		prefix = "charging"
	case systemStateDischarging:
		prefix = "discharging"
	case systemStateIdle:
		prefix = "idle"
	default:
		switch source {
		case "charge":
			prefix = "charging"
		case "discharge":
			prefix = "discharging"
		}
	}
	if prefix != "" {
		return fmt.Sprintf("%s: %dmin (~%s)", prefix, minutes, formatMinutesHuman(minutes))
	}
	return fmt.Sprintf("%dmin (~%s)", minutes, formatMinutesHuman(minutes))
}

func normalizeTempKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return "temp"
	}
	return strings.ReplaceAll(key, " ", "")
}

func formatOptionalWatts(has bool, value float64) string {
	if !has {
		return "n/a"
	}
	return formatWatts(value)
}

func formatOptionalVolts(has bool, value float64) string {
	if !has {
		return "n/a"
	}
	return fmt.Sprintf("%.1fV", value)
}

func formatOptionalAmps(has bool, value float64) string {
	if !has {
		return "n/a"
	}
	return fmt.Sprintf("%.2fA", value)
}

func formatWatts(value float64) string {
	if math.Abs(value) > 1000 {
		return fmt.Sprintf("%.2fkW", value/1000.0)
	}
	return fmt.Sprintf("%.1fW", value)
}

func formatEnergyWh(value float64) string {
	if math.Abs(value) > 1000 {
		return fmt.Sprintf("%.2fkWh", value/1000.0)
	}
	return fmt.Sprintf("%.1fWh", value)
}

func effectivePVInputWatts(
	hasInputWatts bool,
	inputWatts float64,
	hasVolts bool,
	volts float64,
	hasAmps bool,
	amps float64,
) (float64, bool) {
	effectiveInputWatts := normalizeInputChannelWatts(inputWatts)
	effectiveHasInputWatts := hasInputWatts
	if hasVolts && hasAmps {
		estimatedInputWatts := math.Abs(volts * amps)
		if estimatedInputWatts > solarPowerEstimateMaxWatts {
			estimatedInputWatts = 0
		}
		// Guard against scale mismatches (for example raw mV/mA published as volts/amps).
		if effectiveHasInputWatts && effectiveInputWatts > 0 && estimatedInputWatts > effectiveInputWatts*5 {
			estimatedInputWatts = 0
		}
		if estimatedInputWatts >= solarPowerEstimateMinWatts && (!effectiveHasInputWatts || effectiveInputWatts < estimatedInputWatts) {
			effectiveInputWatts = estimatedInputWatts
			effectiveHasInputWatts = true
		}
	}
	if effectiveHasInputWatts && math.Abs(effectiveInputWatts) < solarPowerEstimateMinWatts {
		effectiveInputWatts = 0
	}
	return effectiveInputWatts, effectiveHasInputWatts
}

func (s *energySnapshot) effectivePVInputChannels() (total float64, hasTotal bool, low float64, hasLow bool, high float64, hasHigh bool) {
	if s == nil {
		return 0, false, 0, false, 0, false
	}
	low, hasLow = effectivePVInputWatts(
		s.HasInPVLow,
		s.InPVLowWatts,
		s.HasSolarLVVolts,
		s.SolarLVVolts,
		s.HasSolarLVAmp,
		s.SolarLVAmp,
	)
	high, hasHigh = effectivePVInputWatts(
		s.HasInPVHigh,
		s.InPVHighWatts,
		s.HasSolarHVVolts,
		s.SolarHVVolts,
		s.HasSolarHVAmp,
		s.SolarHVAmp,
	)

	total, hasTotal = s.InPVWatts, s.HasInPV
	if hasLow || hasHigh {
		channelTotal := 0.0
		if hasLow {
			channelTotal += low
		}
		if hasHigh {
			channelTotal += high
		}
		if !hasTotal || total < channelTotal {
			total = channelTotal
			hasTotal = true
		}
	} else if !hasTotal && (s.HasInPVLow || s.HasInPVHigh) && s.refreshPVTotalFromChannels() {
		total, hasTotal = s.InPVWatts, s.HasInPV
	}
	return total, hasTotal, low, hasLow, high, hasHigh
}

func (s *energySnapshot) effectivePVInputWatts() (float64, bool) {
	total, hasTotal, _, _, _, _ := s.effectivePVInputChannels()
	return total, hasTotal
}

func (s *energySnapshot) pushPVSmoothingSample() {
	if s == nil {
		return
	}
	total, hasTotal, low, hasLow, high, hasHigh := s.effectivePVInputChannels()
	if hasLow && s.pvLowSmoother != nil {
		s.pvLowSmoother.Add(low)
	}
	if hasHigh && s.pvHighSmoother != nil {
		s.pvHighSmoother.Add(high)
	}
	if hasTotal && s.pvTotalSmoother != nil {
		s.pvTotalSmoother.Add(total)
	}
}

func (s *energySnapshot) smoothedPVChannels() (total float64, hasTotal bool, low float64, hasLow bool, high float64, hasHigh bool) {
	if s == nil {
		return 0, false, 0, false, 0, false
	}
	if s.pvTotalSmoother != nil {
		if value, ok := s.pvTotalSmoother.Average(); ok {
			total, hasTotal = value, true
		}
	}
	if s.pvLowSmoother != nil {
		if value, ok := s.pvLowSmoother.Average(); ok {
			low, hasLow = value, true
		}
	}
	if s.pvHighSmoother != nil {
		if value, ok := s.pvHighSmoother.Average(); ok {
			high, hasHigh = value, true
		}
	}
	return total, hasTotal, low, hasLow, high, hasHigh
}

func (s *energySnapshot) pushPowerSmoothingSample() {
	if s == nil {
		return
	}
	if s.HasInAC && s.acInSmoother != nil {
		s.acInSmoother.Add(s.InACWatts)
	}
	effectiveIn, hasIn, effectiveOut, hasOut := s.effectiveTotalsForDisplay()
	if hasIn && s.totalInSmoother != nil {
		s.totalInSmoother.Add(effectiveIn)
	}
	if hasOut && s.totalOutSmoother != nil {
		s.totalOutSmoother.Add(effectiveOut)
	}
}

func (s *energySnapshot) smoothedACInput() (in float64, hasIn bool) {
	if s == nil {
		return 0, false
	}
	if s.acInSmoother != nil {
		if value, ok := s.acInSmoother.Average(); ok {
			in, hasIn = value, true
		}
	}
	return in, hasIn
}

func (s *energySnapshot) smoothedTotalChannels() (in float64, hasIn bool, out float64, hasOut bool) {
	if s == nil {
		return 0, false, 0, false
	}
	if s.totalInSmoother != nil {
		if value, ok := s.totalInSmoother.Average(); ok {
			in, hasIn = value, true
		}
	}
	if s.totalOutSmoother != nil {
		if value, ok := s.totalOutSmoother.Average(); ok {
			out, hasOut = value, true
		}
	}
	return in, hasIn, out, hasOut
}

func formatSmoothedWattsValue(rawValue string, hasRaw bool, rawWatts float64, hasSmooth bool, smoothWatts float64) string {
	if !hasSmooth || !hasRaw {
		return rawValue
	}
	if math.Abs(rawWatts-smoothWatts) < 0.05 {
		return rawValue
	}
	return fmt.Sprintf("%s (~%s avg)", formatWatts(rawWatts), formatWatts(smoothWatts))
}

func derivePVInputState(
	hasInputWatts bool,
	inputWatts float64,
	hasVolts bool,
	volts float64,
	hasAmps bool,
	amps float64,
	lockedByFlag bool,
) string {
	effectiveInputWatts := inputWatts
	effectiveHasInputWatts := hasInputWatts
	if !effectiveHasInputWatts && hasVolts && hasAmps {
		effectiveInputWatts = math.Abs(volts * amps)
		effectiveHasInputWatts = true
	}

	hasPVSignals := lockedByFlag || hasVolts || hasAmps || effectiveHasInputWatts
	if !hasPVSignals {
		return "n/a"
	}

	lowCurrentWithVoltage := hasVolts && volts >= solarLockVoltageMinVolts
	if hasAmps {
		lowCurrentWithVoltage = lowCurrentWithVoltage && math.Abs(amps) <= solarLockCurrentMaxAmps
	}
	lowInput := !effectiveHasInputWatts || effectiveInputWatts <= solarLockInputMaxWatts
	switch {
	case lockedByFlag && lowInput:
		if hasVolts {
			return fmt.Sprintf("locked(%.1fV)", volts)
		}
		return "locked"
	case lowCurrentWithVoltage && lowInput:
		return fmt.Sprintf("locked(%.1fV)", volts)
	case effectiveHasInputWatts && effectiveInputWatts > solarLockInputMaxWatts:
		return fmt.Sprintf("active(%s)", formatWatts(effectiveInputWatts))
	default:
		return "idle"
	}
}

func checkboxStatus(on bool) string {
	if on {
		return "[x]"
	}
	return "[ ]"
}

func decodeAppShowCircuitFlags(showFlag int64) (acOn bool, dcOn bool) {
	acOn = (showFlag & appShowFlagACOnMask) != 0
	dcOn = (showFlag & appShowFlagDCOnMask) != 0
	return acOn, dcOn
}

func shouldShowSeparateUSBAndDC(device ecoflow.GeneralInfoDevice, snapshot *energySnapshot) bool {
	name := strings.ToLower(strings.TrimSpace(device.DeviceName + " " + device.ProductName))
	if strings.Contains(name, "delta 2") {
		return true
	}
	if strings.Contains(name, "delta pro ultra") || strings.Contains(name, "dpu") {
		return false
	}
	if snapshot == nil {
		return false
	}
	// Fallback for unknown products: only split when we have explicit independent signals.
	return snapshot.HasUSBOn || snapshot.HasDC12VOn
}

func shouldShowEVStatus(device ecoflow.GeneralInfoDevice, snapshot *energySnapshot) bool {
	if snapshot != nil && snapshot.HasEVChargingOn {
		return true
	}
	name := strings.ToLower(strings.TrimSpace(device.DeviceName + " " + device.ProductName))
	return strings.Contains(name, "delta pro ultra") || strings.Contains(name, "dpu")
}

func shouldShowPreconditioningStatus(device ecoflow.GeneralInfoDevice, snapshot *energySnapshot) bool {
	name := strings.ToLower(strings.TrimSpace(device.DeviceName + " " + device.ProductName))
	if strings.Contains(name, "delta pro ultra") || strings.Contains(name, "dpu") {
		return true
	}
	if strings.Contains(name, "delta 2 max") {
		return false
	}
	if snapshot == nil {
		return false
	}
	for _, pack := range snapshot.Packs {
		if pack == nil {
			continue
		}
		if pack.HasPreconditioning || pack.HasPreconditioningState || pack.HasPreconditioningHeat || pack.HasPreconditioningEvent {
			return true
		}
	}
	return false
}

func formatXT150DirectionalValues(has bool, totalWatts float64) (inValue string, outValue string) {
	if !has {
		return "n/a", "n/a"
	}
	// XT150 is directional: negative means battery->inverter (input), positive means inverter->battery (output).
	if totalWatts < 0 {
		return formatWatts(-totalWatts), formatWatts(0)
	}
	if totalWatts > 0 {
		return formatWatts(0), formatWatts(totalWatts)
	}
	return formatWatts(0), formatWatts(0)
}

func normalizeInputChannelWatts(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}

func normalizeOutputChannelWatts(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}

func normalizeVoltageVolts(value float64) float64 {
	if value > 1000 {
		return value / 1000.0
	}
	return value
}

func normalizeCurrentAmps(value float64) float64 {
	abs := math.Abs(value)
	if abs > 200 {
		return value / 1000.0
	}
	return value
}

func isLikelyACPassthrough(hasIn bool, inWatts float64, hasOut bool, outWatts float64) bool {
	if !hasIn || !hasOut {
		return false
	}
	inAbs := math.Abs(inWatts)
	outAbs := math.Abs(outWatts)
	if inAbs < passthroughMinWatts || outAbs < passthroughMinWatts {
		return false
	}
	diff := math.Abs(inAbs - outAbs)
	tolerance := passthroughAbsToleranceWatts
	relativeTolerance := math.Max(inAbs, outAbs) * passthroughRelTolerance
	if relativeTolerance > tolerance {
		tolerance = relativeTolerance
	}
	return diff <= tolerance
}

func isLikelySolarPassthrough(
	s *energySnapshot,
	batteryInWatts float64,
	hasBatteryIn bool,
	batteryOutWatts float64,
	hasBatteryOut bool,
) bool {
	if s == nil {
		return false
	}
	outACWatts := 0.0
	hasOutAC := false
	if s.HasOutAC {
		outACWatts = math.Max(outACWatts, math.Abs(s.OutACWatts))
		hasOutAC = true
	}
	if s.HasOutACL14 {
		outACWatts = math.Max(outACWatts, math.Abs(s.OutACL14Watts))
		hasOutAC = true
	}
	if !hasOutAC || outACWatts < solarPassthroughMinOutWatts {
		return false
	}
	if s.HasInAC && math.Abs(s.InACWatts) > solarPassthroughMaxACInWatts {
		return false
	}
	pvInputWatts, hasPVInput := s.effectivePVInputWatts()
	if !hasPVInput || pvInputWatts < solarPassthroughMinPVWatts {
		return false
	}
	if hasBatteryOut && batteryOutWatts > systemStateNetThresholdWatts {
		return false
	}
	if hasBatteryIn && batteryInWatts > idleDrawNoiseFloorWatts {
		return true
	}
	return pvInputWatts+solarPassthroughSlackWatts >= outACWatts
}

func extractDCOutputFromPDStatus(pdStatus pdStatusSummary) float64 {
	total := 0.0
	for _, value := range pdStatus.USBWatts {
		total += normalizeOutputChannelWatts(value)
	}
	if pdStatus.CarWatts != 0 {
		total += normalizeOutputChannelWatts(pdStatus.CarWatts)
	}
	if pdStatus.WireWatts != 0 {
		total += normalizeOutputChannelWatts(pdStatus.WireWatts)
	}
	return total
}

func dcOutStateFromQuota(quota map[string]any) (present bool, on bool) {
	raw, ok := quota["dcOutState"]
	if !ok {
		return false, false
	}
	return true, toInt64(raw) > 0
}

func shouldInferDCOutputFromResidual(pdStatus pdStatusSummary, quota map[string]any) bool {
	// Prefer direct same-message totals when present:
	// out_dc ~= wattsOutSum - invOutWatts.
	if _, hasWattsOut := findQuotaValueByCandidates(quota, "wattsOutSum"); hasWattsOut {
		if _, hasInvOut := findQuotaValueByCandidates(quota, "invOutWatts"); hasInvOut {
			return true
		}
	}

	// Some devices expose AC output under appshow keys instead of invOutWatts.
	if _, hasWattsOut := findQuotaValueByCandidates(quota, "wattsOutSum"); hasWattsOut {
		if _, hasAppshowAC := findQuotaValueByCandidates(quota, "outAcTtPwr", "outPrPwr"); hasAppshowAC {
			return true
		}
	}

	if hasDCHintInIcoBytes(pdStatus.IcoBytes) {
		return true
	}
	if _, ok := quota["dsgPowerDC"]; ok {
		return true
	}
	return false
}

func xt150ForResidualInference(pdStatus pdStatusSummary, snapshot *energySnapshot) float64 {
	if pdStatus.TotalXT150Watts != 0 || len(pdStatus.XT150Watts) > 0 {
		return pdStatus.TotalXT150Watts
	}
	if snapshot != nil && snapshot.HasXT150 {
		return snapshot.XT150Watts
	}
	return 0
}

func deductXT150BatteryLinkFromResidual(residual float64, xt150Watts float64) float64 {
	if residual <= 0 || xt150Watts <= 0 {
		return residual
	}
	// Positive XT150 represents battery-link charging on Delta 2 style payloads.
	if residual >= xt150Watts*dcResidualXT150DeductRatio {
		residual -= xt150Watts
		if residual < 0 {
			return 0
		}
	}
	return residual
}

func hasDCHintInIcoBytes(bytes []uint64) bool {
	// Observed Delta 2 Max stream sets bit-7 of icoBytes[3] when DC path is actively represented.
	if len(bytes) <= 3 {
		return false
	}
	return (bytes[3] & 0x80) != 0
}

func (s *energySnapshot) refreshPVTotalFromChannels() bool {
	if s == nil {
		return false
	}
	if !s.HasInPVLow && !s.HasInPVHigh {
		return false
	}
	total := 0.0
	if s.HasInPVLow {
		total += s.InPVLowWatts
	}
	if s.HasInPVHigh {
		total += s.InPVHighWatts
	}
	s.InPVWatts = total
	s.HasInPV = true
	return true
}

func splitPVChannels(values map[string]float64) (low float64, hasLow bool, high float64, hasHigh bool) {
	for key, value := range values {
		switch classifyPVInputChannelKey(key) {
		case "low":
			low += normalizeInputChannelWatts(value)
			hasLow = true
		case "high":
			high += normalizeInputChannelWatts(value)
			hasHigh = true
		}
	}
	return low, hasLow, high, hasHigh
}

func classifyPVInputChannelKey(key string) string {
	lower := strings.ToLower(strings.TrimSpace(key))
	switch {
	case lower == "":
		return ""
	case strings.Contains(lower, "xt150watts"):
		return ""
	case strings.Contains(lower, "inhvmppt"), strings.Contains(lower, "pv2chargewatts"), strings.Contains(lower, "pv2inwatts"), strings.Contains(lower, "powgetpvh"), strings.Contains(lower, "pv2chargetype"), strings.Contains(lower, "pluginfopvhtype"):
		return "high"
	case strings.Contains(lower, "inlvmppt"), strings.Contains(lower, "pv1chargewatts"), strings.Contains(lower, "powgetpvl"), lower == "inwatts", strings.Contains(lower, "mppt.inwatts"), strings.Contains(lower, "pv1chargetype"), strings.Contains(lower, "pluginfopvltype"):
		return "low"
	default:
		return ""
	}
}

func splitPVChannelTypes(values map[string]int64) (low int64, hasLow bool, high int64, hasHigh bool) {
	for key, value := range values {
		switch classifyPVInputChannelKey(key) {
		case "low":
			low = value
			hasLow = true
		case "high":
			high = value
			hasHigh = true
		}
	}
	return low, hasLow, high, hasHigh
}

func sumPVInputChannelsFromQuota(quota map[string]any) (low float64, hasLow bool, high float64, hasHigh bool) {
	for key, raw := range quota {
		if !isPVInputKey(key) {
			continue
		}
		value, ok := numberFromAny(raw)
		if !ok {
			continue
		}
		switch classifyPVInputChannelKey(key) {
		case "low":
			low += normalizeInputChannelWatts(value)
			hasLow = true
		case "high":
			high += normalizeInputChannelWatts(value)
			hasHigh = true
		}
	}
	return low, hasLow, high, hasHigh
}

func sumPVInputFromQuota(quota map[string]any) (float64, bool) {
	total := 0.0
	found := false
	for key, raw := range quota {
		if !isPVInputKey(key) {
			continue
		}
		value, ok := numberFromAny(raw)
		if !ok {
			continue
		}
		total += normalizeInputChannelWatts(value)
		found = true
	}
	return total, found
}

func sumXT150FromQuota(quota map[string]any) (float64, bool) {
	total := 0.0
	found := false
	for key, raw := range quota {
		if !strings.Contains(strings.ToLower(strings.TrimSpace(key)), "xt150watts") {
			continue
		}
		value, ok := numberFromAny(raw)
		if !ok {
			continue
		}
		total += value
		found = true
	}
	return total, found
}

func sumByKeySuffix(quota map[string]any, suffixes ...string) (float64, bool) {
	total := 0.0
	found := false
	for key, raw := range quota {
		if !keyMatchesAnySuffix(key, suffixes...) {
			continue
		}
		value, ok := numberFromAny(raw)
		if !ok {
			continue
		}
		total += normalizeOutputChannelWatts(value)
		found = true
	}
	return total, found
}

func keyMatchesAnySuffix(key string, suffixes ...string) bool {
	lowerKey := strings.ToLower(strings.TrimSpace(key))
	if lowerKey == "" {
		return false
	}
	for _, suffix := range suffixes {
		lowerSuffix := strings.ToLower(strings.TrimSpace(suffix))
		if lowerSuffix == "" {
			continue
		}
		if strings.HasSuffix(lowerKey, lowerSuffix) {
			return true
		}
	}
	return false
}

func (s *energySnapshot) ensurePack(packNo int) *packSnapshot {
	pack, ok := s.Packs[packNo]
	if ok {
		return pack
	}
	pack = &packSnapshot{}
	s.Packs[packNo] = pack
	return pack
}

func (s *energySnapshot) applyGlobalRemain(remain int64, source string) {
	if remain <= 0 || isLikelyRemainSentinel(remain) {
		return
	}
	if s.HasRemainTime && !isLikelyRemainSentinel(s.RemainTimeRaw) {
		// Guard against known transient remainTime spikes in live appshow traffic
		// (for example 447 -> 45 -> 23 -> 186 -> 447).
		if strings.EqualFold(source, "pdStatus") && s.RemainTimeRaw >= 240 && remain < s.RemainTimeRaw/2 {
			return
		}
	}
	s.RemainTimeRaw = remain
	s.HasRemainTime = true
}

func (s *energySnapshot) applyChargeRemain(remain int64) {
	if remain <= 0 || isLikelyRemainSentinel(remain) {
		return
	}
	s.ChargeRemainTimeRaw = remain
	s.HasChargeRemainTime = true
}

func (s *energySnapshot) applyDischargeRemain(remain int64) {
	if remain <= 0 || isLikelyRemainSentinel(remain) {
		return
	}
	s.DischargeRemainTimeRaw = remain
	s.HasDischargeRemainTime = true
}

func (s *energySnapshot) applyRemainForCurrentState(remain int64) {
	if remain <= 0 || isLikelyRemainSentinel(remain) {
		return
	}
	packChargeW, packDischargeW := packPowerTotals(s.Packs)
	state := s.detectSystemState(s.WattsIn, s.HasWattsIn, s.WattsOut, s.HasWattsOut, packChargeW, packDischargeW)
	switch state {
	case systemStateCharging:
		s.applyChargeRemain(remain)
	case systemStateDischarging:
		s.applyDischargeRemain(remain)
	}
}

func isLikelyRemainSentinel(remain int64) bool {
	return remain >= 120000
}

func normalizePackSerial(raw string) string {
	serial := strings.ToUpper(strings.TrimSpace(raw))
	if serial == "" {
		return ""
	}
	if strings.Trim(serial, "0") == "" {
		return ""
	}
	return serial
}

func extractPackSerialFromQuota(quota map[string]any) string {
	if raw, ok := quota["packSn"]; ok {
		return normalizePackSerial(toString(raw))
	}
	if raw, ok := quota["sn"]; ok {
		return normalizePackSerial(toString(raw))
	}
	return ""
}

func (s *energySnapshot) bindPackSerial(packNo int, serial string) {
	serial = normalizePackSerial(serial)
	if packNo <= 0 || serial == "" {
		return
	}
	if s.PackSNToNo == nil {
		s.PackSNToNo = make(map[string]int)
	}
	s.PackSNToNo[serial] = packNo
	pack := s.ensurePack(packNo)
	pack.Serial = serial
}

func (s *energySnapshot) resolvePackNoFromEnvelope(envelope telemetryEnvelope, quota map[string]any) (int, bool) {
	packNo, hasPackNo := inferPackNoFromEnvelope(envelope)
	serial := extractPackSerialFromQuota(quota)
	if serial == "" {
		return packNo, hasPackNo
	}

	if mappedPackNo, ok := s.PackSNToNo[serial]; ok && mappedPackNo > 0 {
		s.bindPackSerial(mappedPackNo, serial)
		return mappedPackNo, true
	}

	if hasPackNo && packNo > 0 {
		s.bindPackSerial(packNo, serial)
		return packNo, true
	}

	ids := make([]int, 0, len(s.Packs))
	for id := range s.Packs {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		if normalizePackSerial(s.Packs[id].Serial) == "" {
			s.bindPackSerial(id, serial)
			return id, true
		}
	}

	nextPackNo := len(s.Packs) + 1
	if nextPackNo > 0 {
		s.bindPackSerial(nextPackNo, serial)
		return nextPackNo, true
	}
	return 0, false
}

func hasQuotaKey(quota map[string]any, key string) bool {
	_, ok := quota[key]
	return ok
}

func firstNumberFromKeys(quota map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		value, ok := quota[key]
		if !ok {
			continue
		}
		number, ok := numberFromAny(value)
		if !ok {
			continue
		}
		return number, true
	}
	return 0, false
}

func inferPackNoFromEnvelope(envelope telemetryEnvelope) (int, bool) {
	lowerType := strings.ToLower(strings.TrimSpace(envelope.TypeCode))
	if lowerType == "bmsstatus" {
		return 1, true
	}
	lowerAddr := strings.ToLower(strings.TrimSpace(envelope.Addr))
	if strings.Contains(lowerAddr, "bms_addr") && !strings.Contains(lowerAddr, "bms_slave") {
		return 1, true
	}

	if packNo, ok := trailingIntegerAfterUnderscore(envelope.TypeCode); ok {
		if strings.Contains(lowerType, "bmsslavestatus") ||
			strings.Contains(lowerType, "bmsslaveinfo") ||
			strings.Contains(lowerType, "bms_slave") {
			return packNo, true
		}
	}
	if packNo, ok := trailingIntegerAfterUnderscore(envelope.Addr); ok {
		if strings.Contains(lowerAddr, "bms_slave_addr") {
			return packNo, true
		}
	}
	return 0, false
}

func trailingIntegerAfterUnderscore(value string) (int, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	idx := strings.LastIndex(value, "_")
	if idx < 0 || idx+1 >= len(value) {
		return 0, false
	}
	number, err := strconv.Atoi(value[idx+1:])
	if err != nil || number <= 0 {
		return 0, false
	}
	return number, true
}

func averagePackSOC(packs map[int]*packSnapshot) (float64, bool) {
	if len(packs) == 0 {
		return 0, false
	}
	total := 0.0
	count := 0
	for _, pack := range packs {
		if !pack.HasSOC {
			continue
		}
		total += pack.SOC
		count++
	}
	if count == 0 {
		return 0, false
	}
	return total / float64(count), true
}

func (s *energySnapshot) displaySOC() (float64, bool) {
	if weightedSOC, ok := weightedAveragePackSOC(s.Packs); ok {
		return weightedSOC, true
	}
	if avgSOC, ok := averagePackSOC(s.Packs); ok {
		return avgSOC, true
	}
	if s.HasDeviceSOC {
		return s.DeviceSOC, true
	}
	return 0, false
}

func weightedAveragePackSOC(packs map[int]*packSnapshot) (float64, bool) {
	if len(packs) == 0 {
		return 0, false
	}
	weightedTotal := 0.0
	weightSum := 0.0
	for _, pack := range packs {
		if pack == nil || !pack.HasSOC {
			continue
		}
		weight, ok := packSOCWeight(pack)
		if !ok {
			continue
		}
		weightedTotal += pack.SOC * weight
		weightSum += weight
	}
	if weightSum <= 0 {
		return 0, false
	}
	return weightedTotal / weightSum, true
}

func packSOCWeight(pack *packSnapshot) (float64, bool) {
	if pack == nil {
		return 0, false
	}
	switch {
	case pack.HasDesignCap && pack.DesignCap > 0:
		return pack.DesignCap, true
	case pack.HasFullCap && pack.FullCap > 0:
		return pack.FullCap, true
	case pack.HasEnergy && pack.EnergyWh > 0:
		return pack.EnergyWh, true
	default:
		return 0, false
	}
}

func minMax(values []float64) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
	}
	minValue := values[0]
	maxValue := values[0]
	for _, value := range values[1:] {
		if value < minValue {
			minValue = value
		}
		if value > maxValue {
			maxValue = value
		}
	}
	return minValue, maxValue
}

func formatMinutesHuman(totalMinutes int64) string {
	if totalMinutes <= 0 {
		return "0m"
	}
	const minutesPerHour = int64(60)
	const minutesPerDay = int64(24) * minutesPerHour

	days := totalMinutes / minutesPerDay
	remaining := totalMinutes % minutesPerDay
	hours := remaining / minutesPerHour
	minutes := remaining % minutesPerHour

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

func sumValues(values map[string]float64) float64 {
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total
}

func sumInputChannelValues(values map[string]float64) float64 {
	total := 0.0
	for _, value := range values {
		total += normalizeInputChannelWatts(value)
	}
	return total
}

func mapFromAny(value any) map[string]any {
	typed, ok := value.(map[string]any)
	if !ok || len(typed) == 0 {
		return nil
	}
	return typed
}

func mergeAnyMap(dst map[string]any, src map[string]any) {
	if len(src) == 0 {
		return
	}
	for key, value := range src {
		dst[key] = value
	}
}

func inferTypeCode(addr string, quota map[string]any) string {
	lowerAddr := strings.ToLower(strings.TrimSpace(addr))
	switch {
	case strings.Contains(lowerAddr, "pd_appshow_addr"):
		return "pdStatus"
	case strings.Contains(lowerAddr, "pd_backend_addr"):
		return "pdBackend"
	case strings.Contains(lowerAddr, "pd_bp_addr"):
		return "bpInfo"
	case strings.Contains(lowerAddr, "bms_slave_addr"):
		return "bmsSlave"
	case strings.Contains(lowerAddr, "kitinfo"):
		return "kitInfo"
	case lowerAddr == "d_addr":
		return "dAddr"
	}

	if _, ok := quota["watts"]; ok {
		return "kitInfo"
	}
	if _, ok := quota["bpInfo"]; ok {
		return "bpInfo"
	}
	if _, ok := quota["wattsInSum"]; ok {
		return "pdStatus"
	}
	return ""
}

func isPDStatusEnvelope(envelope telemetryEnvelope) bool {
	if strings.EqualFold(envelope.TypeCode, "pdStatus") {
		return true
	}
	lowerAddr := strings.ToLower(strings.TrimSpace(envelope.Addr))
	return strings.Contains(lowerAddr, "pd_appshow_addr")
}

func computeChannelStats(values map[string]float64) channelStats {
	stats := channelStats{}
	for _, value := range values {
		stats.Total += value
		if value > 0 {
			stats.PositiveTotal += value
			stats.PositiveCount++
		}
		if value < 0 {
			stats.NegativeTotal += value
			stats.NegativeCount++
		}
	}
	return stats
}

func printKeyFloatMap(prefix string, values map[string]float64) {
	if len(values) == 0 {
		return
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for i, key := range keys {
		fmt.Printf("%s[%d] %s=%.3f\n", prefix, i, key, values[key])
	}
}

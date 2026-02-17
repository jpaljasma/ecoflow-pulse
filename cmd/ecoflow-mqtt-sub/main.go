package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"os/signal"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflow"
	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflowmqtt"
	"github.com/jpaljasma/ecoflow-pulse/pkg/panelselect"
)

const (
	defaultDeviceMatch            = "delta pro ultra"
	dcResidualInferenceMinWatts   = 3.0
	dcResidualXT150DeductRatio    = 0.7
	solarLockVoltageMinVolts      = 8.0
	solarLockCurrentMaxAmps       = 0.05
	solarLockInputMaxWatts        = 1.5
	solarPowerEstimateMinWatts    = 0.5
	solarPowerEstimateMaxWatts    = 20000.0
	idleDrawNoiseFloorWatts       = 1.0
	systemStateNetThresholdWatts  = 8.0
	systemStateSmoothThreshold    = 10.0
	appShowFlagACOnMask           = int64(0x4)
	appShowFlagDCOnMask           = int64(0x2)
	defaultMinuteTableRows        = 3
	defaultMinuteHistoryBuckets   = 24 * 60
	defaultMinuteHistoryPath      = "logs/telemetry_history.jsonl"
	defaultTrainingCSVPath        = "logs/telemetry_training.csv"
	defaultRawPayloadLogPath      = "logs/mqtt_payload_raw.log"
	defaultPanelDBIndexPath       = "data/solar_panels/solar_panel_specs_v13.index.json"
	defaultTrainingCSVInterval    = 10 * time.Second
	defaultTrainingCSVJitter      = 0.2
	defaultHistoryQueueCapacity   = 1024
	defaultTrainingQueueCapacity  = 4096
	defaultHistoryLoadWindowMins  = 180
	defaultMLBucketSeconds        = 10
	defaultMLHistoryBuckets       = 180
	defaultPVSmoothingSamples     = 6
	defaultPowerSmoothingSamples  = 6
	defaultStateSmoothingSamples  = 6
	defaultMQTTQueueCapacity      = 64
	defaultMQTTLogQueueCapacity   = 2048
	defaultPanelModelQueueCap     = 64
	defaultPanelModelResultCap    = 64
	defaultMQTTAuthRejectThresh   = 3
	defaultMQTTFallbackPollEvery  = 15 * time.Second
	defaultMQTTFallbackPollTO     = 12 * time.Second
	defaultMQTTReconcileEvery     = 1 * time.Minute
	defaultMQTTReconcileTO        = 12 * time.Second
	defaultMQTTStaleAfter         = 30 * time.Second
	defaultMQTTLivenessPollAfter  = 90 * time.Second
	defaultMQTTLivenessCheckEvery = 10 * time.Second
	defaultMQTTLivenessPollTO     = 12 * time.Second
	defaultMQTTLivenessMinIntvl   = 60 * time.Second
	defaultUIRefreshEvery         = 1 * time.Second
	defaultUIQueueCapacity        = 8
	passthroughMinWatts           = 20.0
	passthroughAbsToleranceWatts  = 15.0
	passthroughRelTolerance       = 0.12
	solarPassthroughMinOutWatts   = 5.0
	solarPassthroughMinPVWatts    = 5.0
	solarPassthroughMaxACInWatts  = 10.0
	solarPassthroughSlackWatts    = 20.0
	solarChargePVMinWattsD2M      = 5.0
	solarChargePVMinWattsDPU      = 56.0
	solarChargePVHoldWattsD2M     = 5.0
	solarChargePVHoldWattsDPU     = 54.0
	solarChargeBatteryMinWatts    = 8.0
	acInputStaleClearToleranceW   = 12.0
	acInputStaleClearAfter        = 12 * time.Second
	wattsInReconcileToleranceW    = 25.0
	packPowerStaleAfter           = 75 * time.Second
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
	PowerUpdatedAt          time.Time
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
	ChargeStateRaw          int64
	HasChargeState          bool
	PreconditioningOn       bool
	HasPreconditioning      bool
	PreconditioningStateRaw int64
	HasPreconditioningState bool
	PreconditioningEventRaw int64
	HasPreconditioningEvent bool
	PreconditioningHeatTime int64
	HasPreconditioningHeat  bool
}

type panelDBCandidate struct {
	Label               string
	Status              string
	PanelWatts          float64
	VocV                float64
	VmpV                float64
	ImpA                float64
	IscA                float64
	ModuleEfficiencyPct float64
	ModuleEfficiencySrc string
	MinSeries           int
	MaxSeries           int
	Bifacial            bool
}

type energySnapshot struct {
	DeviceSOC     float64
	HasDeviceSOC  bool
	FullEnergyWh  float64
	HasFullEnergy bool

	WattsIn      float64
	HasWattsIn   bool
	WattsOut     float64
	HasWattsOut  bool
	WattsInAt    time.Time
	HasWattsInAt bool

	InACWatts        float64
	HasInAC          bool
	InACAt           time.Time
	HasInACAt        bool
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
	FanOn            bool
	HasFanOn         bool
	FanLevelRaw      int64
	HasFanLevel      bool
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
	MQTTConnected          bool
	MQTTDegraded           bool
	MQTTDegradedReason     string
	MQTTAuthRejectStreak   int
	MQTTFallbackActive     bool
	MQTTFallbackPollCount  uint64
	MQTTLastError          string
	MQTTLastMessageAt      time.Time
	HasMQTTLastMessage     bool
	MQTTConnectedSince     time.Time
	HasMQTTConnectedSince  bool
	MQTTStaleAfter         time.Duration
	DataUpdatedAt          time.Time
	HasDataUpdatedAt       bool

	sensorUpdates   []sensorUpdateEvent
	sensorStateLast map[string]bool
	sensorStateSeen map[string]bool

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
	PVLowChgStateRaw    int64
	HasPVLowChgState    bool
	PVHighChgStateRaw   int64
	HasPVHighChgState   bool
	PVLowType           int64
	HasPVLowType        bool
	PVHighType          int64
	HasPVHighType       bool

	PVLowPanelSetup          string
	PVLowPanelConfidence     float64
	PVLowPanelSamples        int
	PVLowPanelCount          int
	HasPVLowPanelCount       bool
	PVLowPanelNominalWatts   float64
	HasPVLowPanelNominal     bool
	PVLowPanelStatus         string
	HasPVLowPanelPrediction  bool
	PVHighPanelSetup         string
	PVHighPanelConfidence    float64
	PVHighPanelSamples       int
	PVHighPanelCount         int
	HasPVHighPanelCount      bool
	PVHighPanelNominalWatts  float64
	HasPVHighPanelNominal    bool
	PVHighPanelStatus        string
	HasPVHighPanelPrediction bool

	PVLowBestPanelLabel      string
	HasPVLowBestPanelLabel   bool
	PVLowBestPanelWatts      float64
	HasPVLowBestPanelWatts   bool
	PVLowBestPanelVocV       float64
	HasPVLowBestPanelVocV    bool
	PVLowBestPanelVmpV       float64
	HasPVLowBestPanelVmpV    bool
	PVLowBestPanelImpA       float64
	HasPVLowBestPanelImpA    bool
	PVLowBestPanelIscA       float64
	HasPVLowBestPanelIscA    bool
	PVLowBestPanelEffPct     float64
	HasPVLowBestPanelEffPct  bool
	PVLowBestPanelMaxSeries  int
	HasPVLowBestPanelSeries  bool
	PVLowBestPanelBifacial   bool
	HasPVLowBestPanelType    bool
	PVLowBestPanelEffSource  string
	HasPVLowBestPanelEffSrc  bool
	PVLowAltPanelLabel       string
	HasPVLowAltPanelLabel    bool
	PVLowAltPanelWatts       float64
	HasPVLowAltPanelWatts    bool
	PVLowAltPanelVocV        float64
	HasPVLowAltPanelVocV     bool
	PVLowAltPanelVmpV        float64
	HasPVLowAltPanelVmpV     bool
	PVLowAltPanelImpA        float64
	HasPVLowAltPanelImpA     bool
	PVLowAltPanelIscA        float64
	HasPVLowAltPanelIscA     bool
	PVLowAltPanelEffPct      float64
	HasPVLowAltPanelEffPct   bool
	PVLowAltPanelMaxSeries   int
	HasPVLowAltPanelSeries   bool
	PVLowAltPanelBifacial    bool
	HasPVLowAltPanelType     bool
	PVLowAltPanelEffSource   string
	HasPVLowAltPanelEffSrc   bool
	PVHighBestPanelLabel     string
	HasPVHighBestPanelLabel  bool
	PVHighBestPanelWatts     float64
	HasPVHighBestPanelWatts  bool
	PVHighBestPanelVocV      float64
	HasPVHighBestPanelVocV   bool
	PVHighBestPanelVmpV      float64
	HasPVHighBestPanelVmpV   bool
	PVHighBestPanelImpA      float64
	HasPVHighBestPanelImpA   bool
	PVHighBestPanelIscA      float64
	HasPVHighBestPanelIscA   bool
	PVHighBestPanelEffPct    float64
	HasPVHighBestPanelEffPct bool
	PVHighBestPanelMaxSeries int
	HasPVHighBestPanelSeries bool
	PVHighBestPanelBifacial  bool
	HasPVHighBestPanelType   bool
	PVHighBestPanelEffSource string
	HasPVHighBestPanelEffSrc bool
	PVHighAltPanelLabel      string
	HasPVHighAltPanelLabel   bool
	PVHighAltPanelWatts      float64
	HasPVHighAltPanelWatts   bool
	PVHighAltPanelVocV       float64
	HasPVHighAltPanelVocV    bool
	PVHighAltPanelVmpV       float64
	HasPVHighAltPanelVmpV    bool
	PVHighAltPanelImpA       float64
	HasPVHighAltPanelImpA    bool
	PVHighAltPanelIscA       float64
	HasPVHighAltPanelIscA    bool
	PVHighAltPanelEffPct     float64
	HasPVHighAltPanelEffPct  bool
	PVHighAltPanelMaxSeries  int
	HasPVHighAltPanelSeries  bool
	PVHighAltPanelBifacial   bool
	HasPVHighAltPanelType    bool
	PVHighAltPanelEffSource  string
	HasPVHighAltPanelEffSrc  bool
	PVLowDBCandidates        []panelDBCandidate
	HasPVLowDBCandidates     bool
	PVHighDBCandidates       []panelDBCandidate
	HasPVHighDBCandidates    bool

	solarChargingSticky    bool
	hasSolarChargingSticky bool
	EMSParaVolMin          float64
	HasEMSParaVolMin       bool
	EMSParaVolMax          float64
	HasEMSParaVolMax       bool

	pvLowSmoother             *rollingAverage
	pvHighSmoother            *rollingAverage
	pvTotalSmoother           *rollingAverage
	acInSmoother              *rollingAverage
	totalInSmoother           *rollingAverage
	totalOutSmoother          *rollingAverage
	stateNetSmoother          *rollingAverage
	mlFastHistory             *powerTelemetryHistory
	mlConfidenceEWMA          float64
	hasMLConfEWMA             bool
	mlTopStateUse             bool
	mlProfileChargeBias       float64
	hasMLProfileChargeBias    bool
	mlProfileDischargeBias    float64
	hasMLProfileDischargeBias bool
}

type sensorUpdateEvent struct {
	At     time.Time
	Sensor string
	Status string
}

type snapshotDerived struct {
	SOCValue                 string
	PacksValue               string
	TempsValue               string
	InputValue               string
	OutputValue              string
	NetValue                 string
	SystemStateValue         string
	RemainValue              string
	ChargeLeftValue          string
	EstimateChargeValue      string
	EstimateDischargeValue   string
	EstimateActiveValue      string
	EstimatePowerValue       string
	EstimateConfidenceValue  string
	InACValue                string
	InPVValue                string
	InPVLowValue             string
	InPVHighValue            string
	XT150InValue             string
	XT150OutValue            string
	OutACValue               string
	OutACL14Value            string
	OutDCValue               string
	BatteryInValue           string
	BatteryOutValue          string
	BatteryNetValue          string
	IdleDrawValue            string
	PVStateValue             string
	PVLowStateValue          string
	PVHighStateValue         string
	PVLowVoltsValue          string
	PVHighVoltsValue         string
	PVLowAmpsValue           string
	PVHighAmpsValue          string
	StatusACValue            string
	StatusDCValue            string
	StatusUSBValue           string
	StatusDC12VValue         string
	StatusEVValue            string
	StatusFanValue           string
	StatusPassthroughValue   string
	StatusGroundedValue      string
	StatusSolarPassValue     string
	StatusSolarChargingValue string
	StatusPrecondValue       string
	ChannelsNetValue         string
	ShowFlagValue            string
	BatteryCount             string
	ComboValue               string
	C20LimitValue            string
	ParaLimitValue           string
	EMSWindowValue           string
	SocGuardrail             string
	EffectiveIn              float64
	HasEffectiveIn           bool
	EffectiveOut             float64
	HasEffectiveOut          bool
}

type minuteTableConfig struct {
	Rows                int
	NewestFirst         bool
	HistoryCapacity     int
	ShowSolarCandidates bool
}

type minuteTelemetryBucket struct {
	MinuteStartUnix       int64
	SOCSumPercent         float64
	SOCSamples            int
	SOCLastPercent        float64
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
	BatteryNetSumWatts    float64
	BatteryNetSamples     int
}

type minuteTelemetryHistory struct {
	buckets     map[int64]*minuteTelemetryBucket
	maxBuckets  int
	initialized bool
}

type powerTelemetryBucket struct {
	BucketStartUnix       int64
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
	BatteryNetSumWatts    float64
	BatteryNetSamples     int
	PackChargeSumWatts    float64
	PackChargeSamples     int
	PackDischargeSumWatts float64
	PackDischargeSamples  int
}

type powerTelemetryHistory struct {
	buckets      map[int64]*powerTelemetryBucket
	maxBuckets   int
	bucketWindow time.Duration
}

type minuteTelemetryRecord struct {
	Version               int     `json:"version"`
	DeviceSN              string  `json:"device_sn"`
	MinuteStartUnix       int64   `json:"minute_start_unix"`
	SOCSumPercent         float64 `json:"soc_sum_percent"`
	SOCSamples            int     `json:"soc_samples"`
	SolarSumWatts         float64 `json:"solar_sum_watts"`
	SolarSamples          int     `json:"solar_samples"`
	ACInSumWatts          float64 `json:"ac_in_sum_watts"`
	ACInSamples           int     `json:"ac_in_samples"`
	ACOutSumWatts         float64 `json:"ac_out_sum_watts"`
	ACOutSamples          int     `json:"ac_out_samples"`
	DCOutSumWatts         float64 `json:"dc_out_sum_watts"`
	DCOutSamples          int     `json:"dc_out_samples"`
	BatteryChargeSumWatts float64 `json:"battery_charge_sum_watts"`
	BatteryChargeSamples  int     `json:"battery_charge_samples"`
	BatteryNetSumWatts    float64 `json:"battery_net_sum_watts"`
	BatteryNetSamples     int     `json:"battery_net_samples"`
}

type minuteTelemetryStore struct {
	path  string
	sink  *fileAppendSink
	queue *asyncChunkQueue
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

type mqttConnectRetryEvent struct {
	Connected    bool
	AuthRejected bool
	Attempt      int
	RetryIn      time.Duration
	Error        error
	Topic        string
	Broker       string
}

type mqttSessionEventKind int

const (
	mqttSessionEventConnectFailure mqttSessionEventKind = iota + 1
	mqttSessionEventConnected
	mqttSessionEventDisconnected
	mqttSessionEventFatal
)

type mqttSessionEvent struct {
	Kind         mqttSessionEventKind
	AuthRejected bool
	Attempt      int
	RetryIn      time.Duration
	Error        error
	Topic        string
	Broker       string
}

type mqttQueueStats struct {
	droppedOldest atomic.Uint64
}

type panelModelWorkerStats struct {
	droppedInput  atomic.Uint64
	droppedOutput atomic.Uint64
}

type mqttOutputLogger struct {
	mu      sync.RWMutex
	sink    appendChunkSink
	queue   chan []byte
	wg      sync.WaitGroup
	closed  bool
	dropped atomic.Uint64
}

type appendChunkSink interface {
	Path() string
	WriteChunk([]byte) error
	Close() error
}

func newMQTTOutputLogger(path string, queueCapacity int) (*mqttOutputLogger, error) {
	sink, err := newFileAppendSink(path, defaultAppendChunkBytes)
	if err != nil {
		return nil, fmt.Errorf("init mqtt log append sink: %w", err)
	}
	return newMQTTOutputLoggerWithSink(sink, queueCapacity)
}

func newMQTTOutputLoggerWithSink(sink appendChunkSink, queueCapacity int) (*mqttOutputLogger, error) {
	if sink == nil {
		return nil, errors.New("mqtt output logger sink is nil")
	}
	if queueCapacity <= 0 {
		queueCapacity = defaultMQTTLogQueueCapacity
	}
	logger := &mqttOutputLogger{
		sink:  sink,
		queue: make(chan []byte, queueCapacity),
	}
	logger.wg.Add(1)
	go logger.runWriter()
	return logger, nil
}

func (l *mqttOutputLogger) runWriter() {
	defer l.wg.Done()
	for payload := range l.queue {
		if l.sink == nil || len(payload) == 0 {
			continue
		}
		_ = l.sink.WriteChunk(payload)
	}
}

func (l *mqttOutputLogger) Close() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return nil
	}
	l.closed = true
	queue := l.queue
	l.mu.Unlock()

	if queue != nil {
		close(queue)
	}
	l.wg.Wait()

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.sink == nil {
		return nil
	}
	err := l.sink.Close()
	l.sink = nil
	l.queue = nil
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
	payload := []byte(ts + " " + line)

	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.closed || l.queue == nil {
		return
	}
	_, dropped := enqueueLoggerChunkDropOldest(l.queue, payload, &l.dropped)
	if dropped {
		// Intentionally silent to avoid recursive logging on logger pressure.
	}
}

func (l *mqttOutputLogger) DroppedCount() uint64 {
	if l == nil {
		return 0
	}
	return l.dropped.Load()
}

func enqueueLoggerChunkDropOldest(queue chan []byte, payload []byte, counter *atomic.Uint64) (enqueued bool, droppedOldest bool) {
	if queue == nil {
		return false, false
	}
	select {
	case queue <- payload:
		return true, false
	default:
	}
	select {
	case <-queue:
		droppedOldest = true
		if counter != nil {
			counter.Add(1)
		}
	default:
	}
	select {
	case queue <- payload:
		return true, droppedOldest
	default:
		// If we still can't enqueue under contention, keep the old queue content.
		return false, droppedOldest
	}
}

func enqueuePanelObservationDropOldest(queue chan panelObservation, sample panelObservation, counter *atomic.Uint64) (enqueued bool, droppedOldest bool) {
	if queue == nil {
		return false, false
	}
	select {
	case queue <- sample:
		return true, false
	default:
	}
	select {
	case <-queue:
		droppedOldest = true
		if counter != nil {
			counter.Add(1)
		}
	default:
	}
	select {
	case queue <- sample:
		return true, droppedOldest
	default:
		return false, droppedOldest
	}
}

func enqueuePanelObservationResultDropOldest(queue chan panelObservationResult, result panelObservationResult, counter *atomic.Uint64) (enqueued bool, droppedOldest bool) {
	if queue == nil {
		return false, false
	}
	select {
	case queue <- result:
		return true, false
	default:
	}
	select {
	case <-queue:
		droppedOldest = true
		if counter != nil {
			counter.Add(1)
		}
	default:
	}
	select {
	case queue <- result:
		return true, droppedOldest
	default:
		return false, droppedOldest
	}
}

func runPanelModelWorker(
	ctx context.Context,
	model *panelRuntimeModel,
	input <-chan panelObservation,
	output chan panelObservationResult,
	stats *panelModelWorkerStats,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case sample, ok := <-input:
			if !ok {
				return
			}
			if model == nil {
				continue
			}
			result := model.ObserveSample(sample)
			if !result.HasUpdates() {
				continue
			}
			var droppedOutputCounter *atomic.Uint64
			if stats != nil {
				droppedOutputCounter = &stats.droppedOutput
			}
			_, _ = enqueuePanelObservationResultDropOldest(output, result, droppedOutputCounter)
		}
	}
}

func newEnergySnapshot() *energySnapshot {
	return &energySnapshot{
		Packs:           make(map[int]*packSnapshot),
		PackSNToNo:      make(map[string]int),
		KitSOC:          make(map[string]float64),
		Temperatures:    make(map[string]float64),
		sensorStateLast: make(map[string]bool),
		sensorStateSeen: make(map[string]bool),
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

func (s *energySnapshot) configureStateSmoothing(window int) {
	if s == nil {
		return
	}
	s.stateNetSmoother = newRollingAverage(window)
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

func newPowerTelemetryHistory(bucketWindow time.Duration, maxBuckets int) *powerTelemetryHistory {
	if bucketWindow <= 0 {
		bucketWindow = time.Duration(defaultMLBucketSeconds) * time.Second
	}
	if maxBuckets <= 0 {
		maxBuckets = defaultMLHistoryBuckets
	}
	return &powerTelemetryHistory{
		buckets:      make(map[int64]*powerTelemetryBucket),
		maxBuckets:   maxBuckets,
		bucketWindow: bucketWindow,
	}
}

func newMinuteTelemetryStore(path string) (*minuteTelemetryStore, error) {
	sink, err := newFileAppendSink(path, defaultAppendChunkBytes)
	if err != nil {
		return nil, fmt.Errorf("init minute telemetry append sink: %w", err)
	}
	queueCapacity := parsePositiveIntEnv("ECOFLOW_MQTT_HISTORY_QUEUE_CAPACITY", defaultHistoryQueueCapacity)
	queue := newAsyncChunkQueue(queueCapacity, sink.WriteChunk)
	return &minuteTelemetryStore{
		path:  sink.Path(),
		sink:  sink,
		queue: queue,
	}, nil
}

func (s *minuteTelemetryStore) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

func (s *minuteTelemetryStore) Close() error {
	if s == nil {
		return nil
	}
	var queueErr error
	if s.queue != nil {
		queueErr = s.queue.Close()
		s.queue = nil
	}
	if s.sink == nil {
		if queueErr != nil {
			return queueErr
		}
		return nil
	}
	err := s.sink.Close()
	s.sink = nil
	if queueErr != nil {
		if err != nil {
			return fmt.Errorf("%v; %w", queueErr, err)
		}
		return queueErr
	}
	return err
}

func (s *minuteTelemetryStore) Flush() error {
	if s == nil || s.queue == nil {
		return nil
	}
	return s.queue.Flush()
}

func (s *minuteTelemetryStore) DroppedCount() uint64 {
	if s == nil || s.queue == nil {
		return 0
	}
	return s.queue.DroppedCount()
}

func (s *minuteTelemetryStore) AppendBucket(deviceSN string, bucket minuteTelemetryBucket) error {
	if s == nil {
		return nil
	}
	if bucket.MinuteStartUnix <= 0 {
		return nil
	}
	record := minuteTelemetryRecord{
		Version:               1,
		DeviceSN:              strings.TrimSpace(deviceSN),
		MinuteStartUnix:       bucket.MinuteStartUnix,
		SOCSumPercent:         bucket.SOCSumPercent,
		SOCSamples:            bucket.SOCSamples,
		SolarSumWatts:         bucket.SolarSumWatts,
		SolarSamples:          bucket.SolarSamples,
		ACInSumWatts:          bucket.ACInSumWatts,
		ACInSamples:           bucket.ACInSamples,
		ACOutSumWatts:         bucket.ACOutSumWatts,
		ACOutSamples:          bucket.ACOutSamples,
		DCOutSumWatts:         bucket.DCOutSumWatts,
		DCOutSamples:          bucket.DCOutSamples,
		BatteryChargeSumWatts: bucket.BatteryChargeSumWatts,
		BatteryChargeSamples:  bucket.BatteryChargeSamples,
		BatteryNetSumWatts:    bucket.BatteryNetSumWatts,
		BatteryNetSamples:     bucket.BatteryNetSamples,
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal minute telemetry record: %w", err)
	}
	payload = append(payload, '\n')

	if s.sink == nil || s.queue == nil {
		return errors.New("minute telemetry history file is closed")
	}
	enqueued, _ := s.queue.EnqueueDropOldest(payload)
	if !enqueued {
		return errors.New("append minute telemetry record: queue saturated")
	}
	return nil
}

func (s *minuteTelemetryStore) LoadInto(deviceSN string, history *minuteTelemetryHistory) (int, error) {
	return s.LoadIntoWindow(deviceSN, history, 0)
}

func (s *minuteTelemetryStore) LoadIntoWindow(deviceSN string, history *minuteTelemetryHistory, notBeforeMinuteStartUnix int64) (int, error) {
	if s == nil || history == nil {
		return 0, nil
	}
	if err := s.Flush(); err != nil {
		return 0, fmt.Errorf("flush minute telemetry queue before read: %w", err)
	}
	deviceSN = strings.TrimSpace(deviceSN)

	file, err := os.Open(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, fmt.Errorf("open minute telemetry history for read: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	seenMinutes := make(map[int64]struct{})
	scanner := bufio.NewScanner(file)
	buffer := make([]byte, 0, 64*1024)
	scanner.Buffer(buffer, 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var record minuteTelemetryRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			continue
		}
		if record.MinuteStartUnix <= 0 {
			continue
		}
		if notBeforeMinuteStartUnix > 0 && record.MinuteStartUnix < notBeforeMinuteStartUnix {
			continue
		}
		if deviceSN != "" && strings.TrimSpace(record.DeviceSN) != deviceSN {
			continue
		}
		history.UpsertBucket(minuteTelemetryBucket{
			MinuteStartUnix:       record.MinuteStartUnix,
			SOCSumPercent:         record.SOCSumPercent,
			SOCSamples:            record.SOCSamples,
			SolarSumWatts:         record.SolarSumWatts,
			SolarSamples:          record.SolarSamples,
			ACInSumWatts:          record.ACInSumWatts,
			ACInSamples:           record.ACInSamples,
			ACOutSumWatts:         record.ACOutSumWatts,
			ACOutSamples:          record.ACOutSamples,
			DCOutSumWatts:         record.DCOutSumWatts,
			DCOutSamples:          record.DCOutSamples,
			BatteryChargeSumWatts: record.BatteryChargeSumWatts,
			BatteryChargeSamples:  record.BatteryChargeSamples,
			BatteryNetSumWatts:    record.BatteryNetSumWatts,
			BatteryNetSamples:     record.BatteryNetSamples,
		})
		seenMinutes[record.MinuteStartUnix] = struct{}{}
	}
	if err := scanner.Err(); err != nil {
		return len(seenMinutes), fmt.Errorf("scan minute telemetry history: %w", err)
	}
	return len(seenMinutes), nil
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

	if soc, ok := snapshot.displaySOC(); ok {
		bucket.SOCSumPercent += soc
		bucket.SOCSamples++
		bucket.SOCLastPercent = soc
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
	packChargeW, packDischargeW := packPowerTotals(snapshot.Packs)
	effectiveIn, hasEffectiveIn, effectiveOut, hasEffectiveOut :=
		snapshot.effectiveTotalsForDisplayWithPackTotals(packChargeW, packDischargeW)
	batteryFlow := snapshot.batteryFlowForDisplay(
		effectiveIn,
		hasEffectiveIn,
		effectiveOut,
		hasEffectiveOut,
		packChargeW,
		packDischargeW,
	)
	if batteryFlow.hasNet {
		bucket.BatteryNetSumWatts += batteryFlow.netWatts
		bucket.BatteryNetSamples++
		chargeWatts := 0.0
		if batteryFlow.netWatts > 0 {
			chargeWatts = batteryFlow.netWatts
		}
		bucket.BatteryChargeSumWatts += chargeWatts
		bucket.BatteryChargeSamples++
	}
	h.pruneOldest()
}

func (h *powerTelemetryHistory) AddSample(at time.Time, snapshot *energySnapshot) {
	if h == nil || snapshot == nil {
		return
	}
	if h.buckets == nil {
		h.buckets = make(map[int64]*powerTelemetryBucket)
	}
	window := h.bucketWindow
	if window <= 0 {
		window = time.Duration(defaultMLBucketSeconds) * time.Second
	}
	bucketStart := at.Truncate(window).Unix()
	bucket := h.buckets[bucketStart]
	if bucket == nil {
		bucket = &powerTelemetryBucket{BucketStartUnix: bucketStart}
		h.buckets[bucketStart] = bucket
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
	packChargeW, packDischargeW := packPowerTotals(snapshot.Packs)
	effectiveIn, hasEffectiveIn, effectiveOut, hasEffectiveOut :=
		snapshot.effectiveTotalsForDisplayWithPackTotals(packChargeW, packDischargeW)
	batteryFlow := snapshot.batteryFlowForDisplay(
		effectiveIn,
		hasEffectiveIn,
		effectiveOut,
		hasEffectiveOut,
		packChargeW,
		packDischargeW,
	)
	if batteryFlow.hasNet {
		bucket.BatteryNetSumWatts += batteryFlow.netWatts
		bucket.BatteryNetSamples++
		chargeWatts := 0.0
		if batteryFlow.netWatts > 0 {
			chargeWatts = batteryFlow.netWatts
		}
		bucket.BatteryChargeSumWatts += chargeWatts
		bucket.BatteryChargeSamples++
	}
	if len(snapshot.Packs) > 0 || packChargeW > 0 || packDischargeW > 0 {
		bucket.PackChargeSumWatts += packChargeW
		bucket.PackChargeSamples++
		bucket.PackDischargeSumWatts += packDischargeW
		bucket.PackDischargeSamples++
	}

	h.pruneOldest()
}

func (h *minuteTelemetryHistory) UpsertBucket(bucket minuteTelemetryBucket) {
	if h == nil || bucket.MinuteStartUnix <= 0 {
		return
	}
	if h.buckets == nil {
		h.buckets = make(map[int64]*minuteTelemetryBucket)
	}
	copyBucket := bucket
	h.buckets[bucket.MinuteStartUnix] = &copyBucket
	h.pruneOldest()
}

func (h *minuteTelemetryHistory) Bucket(minuteStartUnix int64) (minuteTelemetryBucket, bool) {
	if h == nil || minuteStartUnix <= 0 {
		return minuteTelemetryBucket{}, false
	}
	bucket, ok := h.buckets[minuteStartUnix]
	if !ok || bucket == nil {
		return minuteTelemetryBucket{}, false
	}
	return *bucket, true
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

func (h *powerTelemetryHistory) pruneOldest() {
	if h == nil || h.maxBuckets <= 0 || len(h.buckets) <= h.maxBuckets {
		return
	}
	keys := make([]int64, 0, len(h.buckets))
	for start := range h.buckets {
		keys = append(keys, start)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	for len(keys) > h.maxBuckets {
		delete(h.buckets, keys[0])
		keys = keys[1:]
	}
}

func (h *powerTelemetryHistory) SortedBuckets(newestFirst bool, limit int) []powerTelemetryBucket {
	if h == nil || len(h.buckets) == 0 || limit == 0 {
		return nil
	}
	keys := make([]int64, 0, len(h.buckets))
	for start := range h.buckets {
		keys = append(keys, start)
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
	out := make([]powerTelemetryBucket, 0, len(keys))
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
	mqttLogQueueCapacity := parsePositiveIntEnv("ECOFLOW_MQTT_LOG_QUEUE_CAPACITY", defaultMQTTLogQueueCapacity)
	mqttLogPath := envOrDefault("ECOFLOW_MQTT_LOG_PATH", "logs/mqtt.log")
	runLog, err := newMQTTOutputLogger(mqttLogPath, mqttLogQueueCapacity)
	if err != nil {
		fatalf("init mqtt run log: %v", err)
	}
	defer func() {
		dropped := runLog.DroppedCount()
		if dropped > 0 {
			logger.Warn("mqtt run log dropped queued chunks", slog.Uint64("dropped_chunks", dropped))
		}
		if closeErr := runLog.Close(); closeErr != nil {
			logger.Warn("flush/close mqtt run log failed", slog.String("error", closeErr.Error()))
		}
	}()
	runLog.Printf("session_start")

	rawPayloadLogEnabled := parseBoolEnv("ECOFLOW_MQTT_RAW_PAYLOAD_LOG", true)
	rawPayloadLogPath := envOrDefault("ECOFLOW_MQTT_RAW_PAYLOAD_LOG_PATH", defaultRawPayloadLogPath)
	rawPayloadLogQueueCapacity := parsePositiveIntEnv("ECOFLOW_MQTT_RAW_PAYLOAD_LOG_QUEUE_CAPACITY", mqttLogQueueCapacity)
	var rawPayloadLog *mqttOutputLogger
	if rawPayloadLogEnabled {
		rotatingSink, sinkErr := newDailyRotatingAppendSink(rawPayloadLogPath, defaultAppendChunkBytes)
		if sinkErr != nil {
			logger.Warn("init raw mqtt payload log sink failed", slog.String("error", sinkErr.Error()), slog.String("path", rawPayloadLogPath))
			runLog.Printf("raw_payload_log_init_error path=%s error=%q", rawPayloadLogPath, sinkErr.Error())
		} else {
			rawPayloadLog, err = newMQTTOutputLoggerWithSink(rotatingSink, rawPayloadLogQueueCapacity)
			if err != nil {
				logger.Warn("init raw mqtt payload logger failed", slog.String("error", err.Error()), slog.String("path", rawPayloadLogPath))
				runLog.Printf("raw_payload_log_init_error path=%s error=%q", rawPayloadLogPath, err.Error())
				_ = rotatingSink.Close()
			} else {
				runLog.Printf("raw_payload_log_enabled path=%s rotate=daily queue_capacity=%d", rawPayloadLogPath, rawPayloadLogQueueCapacity)
				defer func() {
					dropped := rawPayloadLog.DroppedCount()
					if dropped > 0 {
						logger.Warn("raw mqtt payload log dropped queued chunks", slog.Uint64("dropped_chunks", dropped))
					}
					if closeErr := rawPayloadLog.Close(); closeErr != nil {
						logger.Warn("flush/close raw mqtt payload log failed", slog.String("error", closeErr.Error()))
					}
				}()
			}
		}
	} else {
		runLog.Printf("raw_payload_log_disabled path=%s", rawPayloadLogPath)
	}

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
	var loadedPanelDB *solarPanelIndex
	panelDBEnabled := parseBoolEnv("ECOFLOW_MQTT_PANEL_DB_ENABLED", true)
	panelDBPath := envOrDefault("ECOFLOW_MQTT_PANEL_DB_PATH", defaultPanelDBIndexPath)
	if panelDBEnabled {
		panelDB, loadErr := loadSolarPanelIndex(panelDBPath)
		if loadErr != nil {
			logger.Warn(
				"solar panel database load failed",
				slog.String("path", panelDBPath),
				slog.String("error", loadErr.Error()),
			)
			runLog.Printf("solar_panel_db_load_error path=%s error=%q", panelDBPath, loadErr.Error())
		} else {
			loadedPanelDB = panelDB
			deviceTags := inferPanelDeviceTags(targetDevice)
			candidatePanels := panelDB.candidatePanelsForDeviceTags(deviceTags)
			tagList := strings.Join(deviceTags, ",")
			if strings.TrimSpace(tagList) == "" {
				tagList = "n/a"
			}
			logger.Info(
				"solar panel database loaded",
				slog.String("path", panelDBPath),
				slog.Int("panel_count", panelDB.PanelCount),
				slog.Int("device_tag_count", len(deviceTags)),
				slog.Int("candidate_panel_count", len(candidatePanels)),
			)
			runLog.Printf(
				"solar_panel_db_loaded path=%s panel_count=%d device_tags=%s candidate_panels=%d tag_labels=%q",
				panelDBPath,
				panelDB.PanelCount,
				tagList,
				len(candidatePanels),
				panelTagLabels(panelDB, deviceTags),
			)
		}
	} else {
		runLog.Printf("solar_panel_db_disabled path=%s", panelDBPath)
	}
	panelModelEnabled := parseBoolEnv("ECOFLOW_MQTT_PANEL_MODEL_ENABLED", true)
	panelModelPath := envOrDefault("ECOFLOW_MQTT_PANEL_MODEL_PATH", defaultPanelModelPath)
	panelModelWindow := parsePositiveIntEnv("ECOFLOW_MQTT_PANEL_MODEL_WINDOW", panelselect.DefaultTrackerLimit)
	panelModelMinSamples := parsePositiveIntEnv("ECOFLOW_MQTT_PANEL_MODEL_MIN_SAMPLES", defaultPanelModelMinSamples)
	var runtimePanelModel *panelRuntimeModel
	if panelModelEnabled {
		panelModel, loadErr := panelselect.Load(panelModelPath)
		if loadErr != nil {
			logger.Warn(
				"solar panel select model load failed",
				slog.String("path", panelModelPath),
				slog.String("error", loadErr.Error()),
			)
			runLog.Printf("solar_panel_model_load_error path=%s error=%q", panelModelPath, loadErr.Error())
		} else {
			runtimePanelModel = newPanelRuntimeModel(panelModel, targetDevice.ProductName, panelModelWindow, panelModelMinSamples)
			logger.Info(
				"solar panel select model loaded",
				slog.String("path", panelModelPath),
				slog.Int("classes", len(panelModel.Classes)),
				slog.Int("window", panelModelWindow),
				slog.Int("min_samples", panelModelMinSamples),
			)
			runLog.Printf(
				"solar_panel_model_loaded path=%s classes=%d profile=%s window=%d min_samples=%d",
				panelModelPath,
				len(panelModel.Classes),
				panelselect.NormalizeProfile(targetDevice.ProductName),
				panelModelWindow,
				panelModelMinSamples,
			)
		}
	} else {
		runLog.Printf("solar_panel_model_disabled path=%s", panelModelPath)
	}
	lockDir := envOrDefault("ECOFLOW_MQTT_LOCK_DIR", defaultDeviceLockDir)
	if err := cleanupStaleDeviceLocks(lockDir); err != nil {
		logger.Warn("stale device lock cleanup skipped", slog.String("error", err.Error()))
	}
	instanceLock, err := acquireDeviceInstanceLock(lockDir, targetDevice.SN, targetDevice.ProductName)
	if err != nil {
		fatalf("acquire device serial lock: %v", err)
	}
	defer func() {
		if closeErr := instanceLock.Close(); closeErr != nil {
			logger.Warn("release device instance lock failed", slog.String("error", closeErr.Error()))
		}
	}()
	runLog.Printf("instance_lock_acquired sn=%s path=%s", targetDevice.SN, instanceLock.Path())
	logger.Info(
		"device instance lock acquired",
		slog.String("device_sn", targetDevice.SN),
		slog.String("path", instanceLock.Path()),
	)
	printPayload := parseBoolEnv("ECOFLOW_MQTT_PRINT_PAYLOAD", false)
	tableView := parseBoolEnv("ECOFLOW_MQTT_TABLE_VIEW", true) && !printPayload
	var uiWriter *asyncUIWriter
	if tableView {
		uiQueueCapacity := parsePositiveIntEnv("ECOFLOW_MQTT_UI_QUEUE_CAPACITY", defaultUIQueueCapacity)
		uiWriter = newAsyncUIWriter(os.Stdout, uiQueueCapacity)
		defer func() {
			if uiWriter == nil {
				return
			}
			dropped := uiWriter.DroppedCount()
			if dropped > 0 {
				logger.Warn("ui frames dropped queued updates", slog.Uint64("dropped_frames", dropped))
			}
			if closeErr := uiWriter.Close(); closeErr != nil {
				logger.Warn("flush/close ui writer failed", slog.String("error", closeErr.Error()))
			}
		}()
	}
	logBootstrapRaw := parseBoolEnv("ECOFLOW_MQTT_LOG_BOOTSTRAP_RAW", true)
	idleReconnectAfter := durationAllowZero("ECOFLOW_MQTT_IDLE_RECONNECT_AFTER", 120*time.Second)
	mqttStaleAfter := durationAllowZero("ECOFLOW_MQTT_STALE_AFTER", defaultMQTTStaleAfter)
	mqttLivenessPollAfter := durationAllowZero("ECOFLOW_MQTT_LIVENESS_POLL_AFTER", defaultMQTTLivenessPollAfter)
	mqttLivenessCheckEvery := durationAllowZero("ECOFLOW_MQTT_LIVENESS_CHECK_INTERVAL", defaultMQTTLivenessCheckEvery)
	mqttLivenessPollTimeout := mustDuration("ECOFLOW_MQTT_LIVENESS_POLL_TIMEOUT", defaultMQTTLivenessPollTO)
	mqttLivenessPollMinInterval := mustDuration("ECOFLOW_MQTT_LIVENESS_POLL_MIN_INTERVAL", defaultMQTTLivenessMinIntvl)
	minuteTableConfig := minuteTableConfig{
		Rows:                parsePositiveIntEnv("ECOFLOW_MQTT_MINUTE_ROWS", defaultMinuteTableRows),
		NewestFirst:         parseSortNewestFirstEnv("ECOFLOW_MQTT_MINUTE_SORT", true),
		HistoryCapacity:     parsePositiveIntEnv("ECOFLOW_MQTT_MINUTE_HISTORY_BUCKETS", defaultMinuteHistoryBuckets),
		ShowSolarCandidates: parseBoolEnv("ECOFLOW_MQTT_SHOW_SOLAR_CANDIDATES", false),
	}
	minuteHistoryPath := envOrDefault("ECOFLOW_MQTT_HISTORY_PATH", defaultMinuteHistoryPath)
	trainingCSVPath := envOrDefault("ECOFLOW_MQTT_TRAINING_CSV_PATH", defaultTrainingCSVPath)
	trainingCSVEnabled := parseBoolEnv("ECOFLOW_MQTT_TRAINING_CSV", true)
	trainingCSVInterval := durationAllowZero("ECOFLOW_MQTT_TRAINING_CSV_INTERVAL", defaultTrainingCSVInterval)
	trainingCSVJitter := parseNonNegativeFloatEnv("ECOFLOW_MQTT_TRAINING_CSV_JITTER", defaultTrainingCSVJitter)
	historyLoadWindowMins := parsePositiveIntEnv("ECOFLOW_MQTT_HISTORY_LOAD_WINDOW_MINUTES", defaultHistoryLoadWindowMins)
	mlBucketSeconds := parsePositiveIntEnv("ECOFLOW_ML_BUCKET_SECONDS", defaultMLBucketSeconds)
	mlHistoryBuckets := parsePositiveIntEnv("ECOFLOW_ML_HISTORY_BUCKETS", defaultMLHistoryBuckets)
	snapshot := newEnergySnapshot()
	if loadedPanelDB != nil {
		applyPanelDBPortMetadata(snapshot, loadedPanelDB, targetDevice)
	}
	pvSmoothingSamples := parsePositiveIntEnv("ECOFLOW_MQTT_PV_SMOOTH_SAMPLES", defaultPVSmoothingSamples)
	powerSmoothingSamples := parsePositiveIntEnv("ECOFLOW_MQTT_POWER_SMOOTH_SAMPLES", pvSmoothingSamples)
	if powerSmoothingSamples <= 0 {
		powerSmoothingSamples = defaultPowerSmoothingSamples
	}
	stateSmoothingSamples := parsePositiveIntEnv("ECOFLOW_MQTT_STATE_SMOOTH_SAMPLES", powerSmoothingSamples)
	if stateSmoothingSamples <= 0 {
		stateSmoothingSamples = defaultStateSmoothingSamples
	}
	snapshot.configurePVSmoothing(pvSmoothingSamples)
	snapshot.configurePowerSmoothing(powerSmoothingSamples)
	snapshot.configureStateSmoothing(stateSmoothingSamples)
	minuteHistory := newMinuteTelemetryHistory(minuteTableConfig.HistoryCapacity)
	mlFastHistory := newPowerTelemetryHistory(time.Duration(mlBucketSeconds)*time.Second, mlHistoryBuckets)
	snapshot.mlFastHistory = mlFastHistory
	var panelModelInputCh chan panelObservation
	var panelModelOutputCh chan panelObservationResult
	panelModelStats := &panelModelWorkerStats{}
	enqueuePanelObservation := func() {
		if runtimePanelModel == nil || panelModelInputCh == nil {
			return
		}
		sample := buildPanelObservation(snapshot)
		_, _ = enqueuePanelObservationDropOldest(panelModelInputCh, sample, &panelModelStats.droppedInput)
	}
	if runtimePanelModel != nil {
		panelModelQueueCap := parsePositiveIntEnv("ECOFLOW_MQTT_PANEL_MODEL_QUEUE_CAPACITY", defaultPanelModelQueueCap)
		panelModelResultCap := parsePositiveIntEnv("ECOFLOW_MQTT_PANEL_MODEL_RESULT_QUEUE_CAPACITY", defaultPanelModelResultCap)
		panelModelInputCh = make(chan panelObservation, panelModelQueueCap)
		panelModelOutputCh = make(chan panelObservationResult, panelModelResultCap)
		go runPanelModelWorker(ctx, runtimePanelModel, panelModelInputCh, panelModelOutputCh, panelModelStats)
		defer func() {
			droppedInput := panelModelStats.droppedInput.Load()
			droppedOutput := panelModelStats.droppedOutput.Load()
			if droppedInput > 0 || droppedOutput > 0 {
				logger.Warn(
					"panel model worker dropped queued items",
					slog.Uint64("dropped_input", droppedInput),
					slog.Uint64("dropped_output", droppedOutput),
				)
				runLog.Printf(
					"panel_model_worker_dropped dropped_input=%d dropped_output=%d",
					droppedInput,
					droppedOutput,
				)
			}
		}()
		runLog.Printf(
			"panel_model_worker_started queue_capacity=%d result_capacity=%d",
			panelModelQueueCap,
			panelModelResultCap,
		)
	}
	minuteHistoryStore, err := newMinuteTelemetryStore(minuteHistoryPath)
	if err != nil {
		fatalf("init minute telemetry history store: %v", err)
	}
	defer func() {
		dropped := minuteHistoryStore.DroppedCount()
		if dropped > 0 {
			logger.Warn("minute telemetry history dropped queued records", slog.Uint64("dropped_records", dropped))
		}
		if closeErr := minuteHistoryStore.Close(); closeErr != nil {
			logger.Warn("flush/close minute telemetry history store failed", slog.String("error", closeErr.Error()))
		}
	}()
	var trainingCSVStore *trainingTelemetryCSVStore
	if trainingCSVEnabled {
		trainingCSVStore, err = newTrainingTelemetryCSVStore(trainingCSVPath)
		if err != nil {
			logger.Warn("init training telemetry csv store failed", slog.String("error", err.Error()))
			runLog.Printf("training_csv_init_error path=%s error=%q", trainingCSVPath, err.Error())
		} else {
			defer func() {
				dropped := trainingCSVStore.DroppedCount()
				if dropped > 0 {
					logger.Warn("training telemetry csv dropped queued rows", slog.Uint64("dropped_rows", dropped))
				}
				if closeErr := trainingCSVStore.Close(); closeErr != nil {
					logger.Warn("flush/close training telemetry csv store failed", slog.String("error", closeErr.Error()))
				}
			}()
			runLog.Printf(
				"training_csv_enabled path=%s interval=%s jitter=%.3f",
				trainingCSVStore.Path(),
				trainingCSVInterval.String(),
				trainingCSVJitter,
			)
		}
	} else {
		runLog.Printf("training_csv_disabled")
	}
	trainingCaptureScheduler := newTelemetryCaptureScheduler(trainingCSVInterval, trainingCSVJitter)
	loadNotBeforeUnix := int64(0)
	if historyLoadWindowMins > 0 {
		loadNotBeforeUnix = time.Now().Add(-time.Duration(historyLoadWindowMins) * time.Minute).Truncate(time.Minute).Unix()
	}
	loadedMinuteBuckets, err := minuteHistoryStore.LoadIntoWindow(targetDevice.SN, minuteHistory, loadNotBeforeUnix)
	if err != nil {
		logger.Warn(
			"load minute telemetry history failed",
			slog.String("path", minuteHistoryPath),
			slog.String("device_sn", targetDevice.SN),
			slog.String("error", err.Error()),
		)
		runLog.Printf(
			"minute_history_load_error path=%s device_sn=%s window_minutes=%d not_before_unix=%d error=%q",
			minuteHistoryPath,
			targetDevice.SN,
			historyLoadWindowMins,
			loadNotBeforeUnix,
			err.Error(),
		)
	} else {
		logger.Debug(
			"minute telemetry history loaded",
			slog.String("path", minuteHistoryPath),
			slog.String("device_sn", targetDevice.SN),
			slog.Int("window_minutes", historyLoadWindowMins),
			slog.Int("loaded_buckets", loadedMinuteBuckets),
		)
		runLog.Printf(
			"minute_history_loaded path=%s device_sn=%s window_minutes=%d not_before_unix=%d loaded_buckets=%d",
			minuteHistoryPath,
			targetDevice.SN,
			historyLoadWindowMins,
			loadNotBeforeUnix,
			loadedMinuteBuckets,
		)
	}

	lastSampleMinute := int64(-1)
	persistMinuteBucket := func(minuteStartUnix int64) {
		if minuteStartUnix <= 0 {
			return
		}
		bucket, ok := minuteHistory.Bucket(minuteStartUnix)
		if !ok {
			return
		}
		if err := minuteHistoryStore.AppendBucket(targetDevice.SN, bucket); err != nil {
			logger.Warn(
				"persist minute telemetry history failed",
				slog.String("path", minuteHistoryPath),
				slog.String("device_sn", targetDevice.SN),
				slog.Int64("minute_start_unix", minuteStartUnix),
				slog.String("error", err.Error()),
			)
			runLog.Printf(
				"minute_history_append_error path=%s device_sn=%s minute_start_unix=%d error=%q",
				minuteHistoryPath,
				targetDevice.SN,
				minuteStartUnix,
				err.Error(),
			)
		}
	}
	recordMinuteSample := func(at time.Time) {
		minuteHistory.AddSample(at, snapshot)
		mlFastHistory.AddSample(at, snapshot)
		minuteStartUnix := at.Truncate(time.Minute).Unix()
		if lastSampleMinute < 0 {
			lastSampleMinute = minuteStartUnix
			return
		}
		if minuteStartUnix != lastSampleMinute {
			persistMinuteBucket(lastSampleMinute)
			lastSampleMinute = minuteStartUnix
		}
	}
	defer func() {
		persistMinuteBucket(lastSampleMinute)
	}()

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

	topicOverride := strings.TrimSpace(os.Getenv("ECOFLOW_MQTT_TOPIC"))
	keepAlive := mustDuration("ECOFLOW_MQTT_KEEPALIVE", 60*time.Second)
	connectTimeout := mustDuration("ECOFLOW_MQTT_CONNECT_TIMEOUT", 10*time.Second)
	readTimeout := mustDuration("ECOFLOW_MQTT_READ_TIMEOUT", 30*time.Second)
	writeTimeout := mustDuration("ECOFLOW_MQTT_WRITE_TIMEOUT", 15*time.Second)
	fallbackPollEvery := mustDuration("ECOFLOW_MQTT_FALLBACK_POLL_INTERVAL", defaultMQTTFallbackPollEvery)
	fallbackPollTimeout := mustDuration("ECOFLOW_MQTT_FALLBACK_POLL_TIMEOUT", defaultMQTTFallbackPollTO)
	reconcileEvery := durationAllowZero("ECOFLOW_MQTT_RECONCILE_INTERVAL", defaultMQTTReconcileEvery)
	reconcileTimeout := mustDuration("ECOFLOW_MQTT_RECONCILE_TIMEOUT", defaultMQTTReconcileTO)
	uiRefreshEvery := durationAllowZero("ECOFLOW_MQTT_UI_REFRESH_INTERVAL", defaultUIRefreshEvery)
	authRejectThreshold := parsePositiveIntEnv("ECOFLOW_MQTT_AUTH_REJECT_THRESHOLD", defaultMQTTAuthRejectThresh)
	currentTopic := topicOverride
	if currentTopic == "" {
		currentTopic = fmt.Sprintf("/open/<account>/%s/quota", targetDevice.SN)
	}

	mqttQueueCapacity := parsePositiveIntEnv("ECOFLOW_MQTT_QUEUE_CAPACITY", defaultMQTTQueueCapacity)
	mqttIngressQueue := make(chan ecoflowmqtt.Message, mqttQueueCapacity)
	mqttIngressStats := &mqttQueueStats{}
	mqttEventCh := make(chan mqttSessionEvent, 64)
	snapshot.MQTTQueueCapacity = cap(mqttIngressQueue)
	snapshot.MQTTQueueDepth = 0
	snapshot.MQTTQueueDroppedOldest = 0
	snapshot.MQTTConnected = false
	snapshot.MQTTDegraded = false
	snapshot.MQTTDegradedReason = ""
	snapshot.MQTTAuthRejectStreak = 0
	snapshot.MQTTFallbackActive = false
	snapshot.MQTTFallbackPollCount = 0
	snapshot.MQTTLastError = ""
	snapshot.MQTTStaleAfter = mqttStaleAfter

	lastEnvelope := telemetryEnvelope{TypeCode: "n/a"}
	emitDashboard := func(envelope telemetryEnvelope) {
		if !tableView {
			return
		}
		frame := renderDashboard(targetDevice, currentTopic, envelope, snapshot, minuteHistory, minuteTableConfig)
		if uiWriter != nil {
			uiWriter.Enqueue(frame)
			return
		}
		fmt.Print(frame)
	}
	debugTelemetry := logger.Enabled(ctx, slog.LevelDebug)
	captureTrainingTelemetry := func(at time.Time, topic string, envelope telemetryEnvelope, quota map[string]any) {
		if trainingCSVStore == nil {
			return
		}
		shouldCaptureNow := isPowerRelatedTelemetry(envelope, quota)
		if !shouldCaptureNow && trainingCaptureScheduler != nil && !trainingCaptureScheduler.ShouldCapture(at) {
			return
		}
		row := buildTrainingTelemetryCSVRow(at, targetDevice, topic, envelope, snapshot)
		if err := trainingCSVStore.AppendRow(row); err != nil {
			logger.Warn(
				"append training telemetry csv row failed",
				slog.String("path", trainingCSVStore.Path()),
				slog.String("error", err.Error()),
			)
			runLog.Printf(
				"training_csv_append_error path=%s error=%q",
				trainingCSVStore.Path(),
				err.Error(),
			)
		}
	}
	if bootstrap.QuotaKeys > 0 {
		lastEnvelope = telemetryEnvelope{TypeCode: "quotaBootstrap"}
		enqueuePanelObservation()
		now := time.Now()
		recordMinuteSample(now)
		captureTrainingTelemetry(now, currentTopic, lastEnvelope, nil)
		if tableView {
			emitDashboard(lastEnvelope)
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

	go func() {
		runMQTTSessionLoop(
			ctx,
			httpClient.GeneralInfo(),
			targetDevice.SN,
			topicOverride,
			keepAlive,
			connectTimeout,
			readTimeout,
			writeTimeout,
			idleReconnectAfter,
			logger,
			runLog,
			mqttIngressQueue,
			mqttIngressStats,
			mqttEventCh,
		)
	}()
	quotaPollReqCh := make(chan quotaPollRequest, 1)
	quotaPollResCh := make(chan quotaPollResult, 1)
	go runQuotaPollWorker(ctx, httpClient.GeneralInfo(), targetDevice.SN, quotaPollReqCh, quotaPollResCh)
	quotaPollInFlight := false
	scheduleQuotaPoll := func(kind quotaPollKind, timeout time.Duration, silenceAge time.Duration) bool {
		if quotaPollInFlight {
			return false
		}
		req := quotaPollRequest{
			Kind:       kind,
			Timeout:    timeout,
			Requested:  time.Now(),
			SilenceAge: silenceAge,
		}
		select {
		case quotaPollReqCh <- req:
			quotaPollInFlight = true
			return true
		default:
			return false
		}
	}
	fallbackTicker := time.NewTicker(fallbackPollEvery)
	defer fallbackTicker.Stop()
	var livenessTicker *time.Ticker
	var livenessCh <-chan time.Time
	if mqttLivenessCheckEvery > 0 && mqttLivenessPollAfter > 0 {
		livenessTicker = time.NewTicker(mqttLivenessCheckEvery)
		livenessCh = livenessTicker.C
		defer livenessTicker.Stop()
	}
	lastLivenessPollAt := time.Time{}
	var reconcileTicker *time.Ticker
	var reconcileCh <-chan time.Time
	if reconcileEvery > 0 {
		reconcileTicker = time.NewTicker(reconcileEvery)
		reconcileCh = reconcileTicker.C
		defer reconcileTicker.Stop()
	}
	var uiRefreshTicker *time.Ticker
	var uiRefreshCh <-chan time.Time
	if tableView && uiRefreshEvery > 0 {
		uiRefreshTicker = time.NewTicker(uiRefreshEvery)
		uiRefreshCh = uiRefreshTicker.C
		defer uiRefreshTicker.Stop()
	}
	authRejectStreak := 0

	for {
		select {
		case <-ctx.Done():
			logger.Debug("ecoflow mqtt subscriber stopped")
			runLog.Printf("session_stop reason=context_canceled")
			return
		case event := <-mqttEventCh:
			if event.Topic != "" {
				currentTopic = event.Topic
			}
			switch event.Kind {
			case mqttSessionEventConnectFailure:
				snapshot.MQTTConnected = false
				snapshot.MQTTLastError = formatErrorString(event.Error)
				if event.AuthRejected {
					authRejectStreak++
					snapshot.MQTTAuthRejectStreak = authRejectStreak
					if authRejectStreak >= authRejectThreshold {
						if !snapshot.MQTTFallbackActive {
							runLog.Printf(
								"mqtt_fallback_enabled reason=auth_rejected streak=%d threshold=%d poll_every=%s",
								authRejectStreak,
								authRejectThreshold,
								fallbackPollEvery.String(),
							)
						}
						snapshot.MQTTDegraded = true
						snapshot.MQTTDegradedReason = "MQTT auth degraded (broker reject code 5)"
						snapshot.MQTTFallbackActive = true
					}
				} else {
					authRejectStreak = 0
					snapshot.MQTTAuthRejectStreak = 0
				}
			case mqttSessionEventConnected:
				authRejectStreak = 0
				snapshot.MQTTConnected = true
				snapshot.MQTTConnectedSince = time.Now()
				snapshot.HasMQTTConnectedSince = true
				snapshot.MQTTDegraded = false
				snapshot.MQTTDegradedReason = ""
				snapshot.MQTTAuthRejectStreak = 0
				snapshot.MQTTFallbackActive = false
				snapshot.MQTTLastError = ""
				runLog.Printf("mqtt_subscription_started device_sn=%s topic=%s broker=%s", targetDevice.SN, currentTopic, event.Broker)
				logger.Debug(
					"ecoflow mqtt subscription started",
					slog.String("device_sn", targetDevice.SN),
					slog.String("device_name", targetDevice.DeviceName),
					slog.String("product_name", targetDevice.ProductName),
					slog.String("broker", event.Broker),
					slog.String("topic", currentTopic),
				)
			case mqttSessionEventDisconnected:
				snapshot.MQTTConnected = false
				snapshot.HasMQTTConnectedSince = false
				snapshot.MQTTLastError = formatErrorString(event.Error)
				if event.AuthRejected {
					authRejectStreak++
					snapshot.MQTTAuthRejectStreak = authRejectStreak
					if authRejectStreak >= authRejectThreshold {
						if !snapshot.MQTTFallbackActive {
							runLog.Printf(
								"mqtt_fallback_enabled reason=auth_rejected_after_disconnect streak=%d threshold=%d poll_every=%s",
								authRejectStreak,
								authRejectThreshold,
								fallbackPollEvery.String(),
							)
						}
						snapshot.MQTTDegraded = true
						snapshot.MQTTDegradedReason = "MQTT auth degraded (broker reject code 5)"
						snapshot.MQTTFallbackActive = true
					}
				}
			case mqttSessionEventFatal:
				if event.Error != nil {
					runLog.Printf("fatal_reader_error error=%q", event.Error.Error())
					fatalf("%v", event.Error)
				}
			}
			if tableView {
				emitDashboard(lastEnvelope)
			}
		case <-fallbackTicker.C:
			if !snapshot.MQTTFallbackActive {
				continue
			}
			if scheduled := scheduleQuotaPoll(quotaPollKindFallback, fallbackPollTimeout, 0); !scheduled {
				logger.Debug("fallback poll skipped; quota poll already in flight")
			}
		case <-livenessCh:
			if snapshot.MQTTFallbackActive || !snapshot.MQTTConnected || !snapshot.HasMQTTLastMessage {
				continue
			}
			if snapshot.MQTTLastMessageAt.IsZero() {
				continue
			}
			silenceAge := time.Since(snapshot.MQTTLastMessageAt)
			if silenceAge < mqttLivenessPollAfter {
				continue
			}
			if !lastLivenessPollAt.IsZero() && time.Since(lastLivenessPollAt) < mqttLivenessPollMinInterval {
				continue
			}
			if scheduleQuotaPoll(quotaPollKindLiveness, mqttLivenessPollTimeout, silenceAge) {
				lastLivenessPollAt = time.Now()
			} else {
				logger.Debug("liveness poll skipped; quota poll already in flight")
			}
		case <-reconcileCh:
			if snapshot.MQTTFallbackActive {
				continue
			}
			if scheduled := scheduleQuotaPoll(quotaPollKindReconcile, reconcileTimeout, 0); !scheduled {
				logger.Debug("reconcile poll skipped; quota poll already in flight")
			}
		case panelResult := <-panelModelOutputCh:
			applyPanelObservationResult(snapshot, panelResult)
			if tableView {
				emitDashboard(lastEnvelope)
			}
		case pollResult := <-quotaPollResCh:
			quotaPollInFlight = false
			if pollResult.Error != nil {
				snapshot.MQTTLastError = pollResult.Error.Error()
				switch pollResult.Request.Kind {
				case quotaPollKindFallback:
					logger.Warn(
						"fallback GetDeviceAllQuota poll failed",
						slog.String("device_sn", targetDevice.SN),
						slog.String("error", pollResult.Error.Error()),
					)
					runLog.Printf("fallback_quota_poll_failed error=%q", pollResult.Error.Error())
				case quotaPollKindLiveness:
					logger.Warn(
						"mqtt liveness poll failed",
						slog.String("device_sn", targetDevice.SN),
						slog.String("error", pollResult.Error.Error()),
						slog.Duration("silence_age", pollResult.Request.SilenceAge.Round(time.Second)),
					)
					runLog.Printf(
						"mqtt_liveness_poll_failed silence_age=%s error=%q",
						pollResult.Request.SilenceAge.Round(time.Second).String(),
						pollResult.Error.Error(),
					)
				case quotaPollKindReconcile:
					logger.Debug(
						"periodic GetDeviceAllQuota reconcile failed",
						slog.String("device_sn", targetDevice.SN),
						slog.String("error", pollResult.Error.Error()),
					)
					runLog.Printf("quota_reconcile_failed error=%q", pollResult.Error.Error())
				}
				continue
			}
			snapshot.MQTTLastError = ""
			report := applyDeviceQuotaToSnapshot(snapshot, pollResult.Quota)
			enqueuePanelObservation()
			now := time.Now()
			switch pollResult.Request.Kind {
			case quotaPollKindFallback:
				snapshot.MQTTFallbackPollCount++
				recordMinuteSample(now)
				captureTrainingTelemetry(now, currentTopic, telemetryEnvelope{TypeCode: "fallbackQuotaPoll"}, nil)
				runLog.Printf(
					"fallback_quota_poll_ok polls=%d quota_keys=%d mapped_packs=%d",
					snapshot.MQTTFallbackPollCount,
					report.QuotaKeys,
					report.MappedPacks,
				)
				if tableView {
					emitDashboard(lastEnvelope)
				} else {
					summary := snapshot.String()
					fmt.Printf("energy_summary %s\n", summary)
					runLog.Printf("energy_summary %s", summary)
				}
			case quotaPollKindLiveness:
				recordMinuteSample(now)
				captureTrainingTelemetry(now, currentTopic, telemetryEnvelope{TypeCode: "livenessQuotaPoll"}, nil)
				runLog.Printf(
					"mqtt_liveness_poll_ok silence_age=%s quota_keys=%d mapped_packs=%d",
					pollResult.Request.SilenceAge.Round(time.Second).String(),
					report.QuotaKeys,
					report.MappedPacks,
				)
				if tableView {
					emitDashboard(lastEnvelope)
				}
			case quotaPollKindReconcile:
				recordMinuteSample(now)
				captureTrainingTelemetry(now, currentTopic, telemetryEnvelope{TypeCode: "quotaReconcile"}, nil)
				runLog.Printf(
					"quota_reconcile_ok interval=%s quota_keys=%d mapped_packs=%d",
					reconcileEvery.String(),
					report.QuotaKeys,
					report.MappedPacks,
				)
				if tableView {
					emitDashboard(lastEnvelope)
				}
			}
		case <-uiRefreshCh:
			if tableView {
				emitDashboard(lastEnvelope)
			}
		case msg := <-mqttIngressQueue:
			if len(msg.Payload) == 0 {
				if ctx.Err() != nil {
					return
				}
				continue
			}

			rawPayload := sanitizeLogLine(string(msg.Payload))
			runLog.Printf("payload_raw=%s", rawPayload)
			if rawPayloadLog != nil {
				rawPayloadLog.Printf("topic=%s payload_raw=%s", sanitizeLogLine(msg.Topic), rawPayload)
			}

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

			batterySOCCount := 0
			pvInputCount := 0
			if debugTelemetry {
				batterySOCCount = len(extractBatterySOC(quota))
				pvInputCount = len(extractPVInput(quota))
			}
			kitEntries, hasKit := extractKitInfoWatts(quota)
			pdStatus := pdStatusSummary{}
			hasPDStatus := false
			if isPDStatusEnvelope(envelope) {
				pdStatus, hasPDStatus = extractPDStatus(quota)
			}

			snapshot.Update(envelope, quota, kitEntries, hasKit, pdStatus, hasPDStatus)
			enqueuePanelObservation()
			snapshot.MQTTQueueDepth = len(mqttIngressQueue)
			snapshot.MQTTQueueCapacity = cap(mqttIngressQueue)
			snapshot.MQTTQueueDroppedOldest = mqttIngressStats.droppedOldest.Load()
			snapshot.MQTTConnected = true
			snapshot.MQTTLastMessageAt = time.Now()
			snapshot.HasMQTTLastMessage = true
			now := time.Now()
			recordMinuteSample(now)
			lastEnvelope = envelope
			captureTrainingTelemetry(now, msg.Topic, envelope, quota)

			if debugTelemetry {
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
					slog.Int("battery_soc_count", batterySOCCount),
					slog.Int("pv_input_count", pvInputCount),
					slog.Bool("has_kitinfo_watts", hasKit),
					slog.Bool("has_pd_status", hasPDStatus),
					slog.Int("queue_depth", snapshot.MQTTQueueDepth),
					slog.Int("queue_capacity", snapshot.MQTTQueueCapacity),
					slog.Uint64("queue_dropped_oldest", snapshot.MQTTQueueDroppedOldest),
				)
			}

			if tableView {
				emitDashboard(lastEnvelope)
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
	if runLog != nil {
		runLog.Printf("quota_bootstrap_fetch_start sn=%s", sn)
	}
	quota, _, err := service.GetDeviceAllQuota(ctx, sn)
	if err != nil {
		if runLog != nil {
			runLog.Printf("quota_bootstrap_fetch_error sn=%s error=%q", sn, err.Error())
		}
		return startupQuotaBootstrap{}, err
	}
	if runLog != nil {
		runLog.Printf("quota_bootstrap_fetch_ok sn=%s keys=%d", sn, len(quota))
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
	copyNumberFromQuotaCandidates(seed, "fanState", parsed,
		"fanState",
		"inv.fanState",
		"hs_yj751_inv_addr.fanState",
	)
	copyNumberFromQuotaCandidates(seed, "fanLevel", parsed,
		"fanLevel",
		"bms_emsStatus.fanLevel",
		"hs_yj751_bms_ems_status_addr.fanLevel",
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

var updateQuotaLookupCandidateSuffixes = []string{
	"chgremaintime",
	"dsgremaintime",
	"invoutwatts",
	"outacttpwr",
	"outprpwr",
	"outacl14pwr",
	"cfgacenabled",
	"inv.cfgacenabled",
	"invinwatts",
	"wattsoutsum",
	"wattsinsum",
	"dcoutstate",
}

var updateQuotaLookupSumSuffixes = []string{
	"outacttpwr",
	"outac5p8pwr",
	"outacl11pwr",
	"outacl12pwr",
	"outacl14pwr",
	"outacl21pwr",
	"outacl22pwr",
	"outprpwr",
	"invoutwatts",
	"outusb1pwr",
	"outusb2pwr",
	"outtypec1pwr",
	"outtypec2pwr",
	"outadspwr",
	"usb1watts",
	"usb2watts",
	"qcusb1watts",
	"qcusb2watts",
	"typec1watts",
	"typec2watts",
	"carwatts",
	"wirewatts",
	"inac5p8pwr",
	"inacc20pwr",
}

type updateQuotaLookup struct {
	byLower       map[string]any
	firstBySuffix map[string]any
	sumBySuffix   map[string]float64
	hasBySuffix   map[string]bool
	hasXT150      bool
	xt150Total    float64
}

func newUpdateQuotaLookup(quota map[string]any) *updateQuotaLookup {
	lookup := &updateQuotaLookup{
		byLower:       make(map[string]any, len(quota)),
		firstBySuffix: make(map[string]any, len(updateQuotaLookupCandidateSuffixes)),
		sumBySuffix:   make(map[string]float64, len(updateQuotaLookupSumSuffixes)),
		hasBySuffix:   make(map[string]bool, len(updateQuotaLookupSumSuffixes)),
	}
	for key, value := range quota {
		lowerKey := strings.ToLower(strings.TrimSpace(key))
		if lowerKey == "" {
			continue
		}
		if _, ok := lookup.byLower[lowerKey]; !ok {
			lookup.byLower[lowerKey] = value
		}
		for _, suffix := range updateQuotaLookupCandidateSuffixes {
			if strings.HasSuffix(lowerKey, suffix) {
				if _, exists := lookup.firstBySuffix[suffix]; !exists {
					lookup.firstBySuffix[suffix] = value
				}
			}
		}
		if number, ok := numberFromAny(value); ok {
			normalized := normalizeOutputChannelWatts(number)
			for _, suffix := range updateQuotaLookupSumSuffixes {
				if strings.HasSuffix(lowerKey, suffix) {
					lookup.sumBySuffix[suffix] += normalized
					lookup.hasBySuffix[suffix] = true
				}
			}
			if strings.Contains(lowerKey, "xt150watts") {
				lookup.hasXT150 = true
				lookup.xt150Total += number
			}
		}
	}
	return lookup
}

func (q *updateQuotaLookup) findAny(candidates ...string) (any, bool) {
	if q == nil {
		return nil, false
	}
	for _, candidate := range candidates {
		lowerCandidate := strings.ToLower(strings.TrimSpace(candidate))
		if lowerCandidate == "" {
			continue
		}
		if value, ok := q.byLower[lowerCandidate]; ok {
			return value, true
		}
	}
	for _, candidate := range candidates {
		lowerCandidate := strings.ToLower(strings.TrimSpace(candidate))
		if lowerCandidate == "" {
			continue
		}
		if value, ok := q.firstBySuffix[lowerCandidate]; ok {
			return value, true
		}
	}
	return nil, false
}

func (q *updateQuotaLookup) hasAny(candidates ...string) bool {
	_, ok := q.findAny(candidates...)
	return ok
}

func (q *updateQuotaLookup) sumAbsBySuffixes(suffixes ...string) (float64, bool) {
	if q == nil {
		return 0, false
	}
	total := 0.0
	found := false
	for _, suffix := range suffixes {
		lowerSuffix := strings.ToLower(strings.TrimSpace(suffix))
		if lowerSuffix == "" {
			continue
		}
		if !q.hasBySuffix[lowerSuffix] {
			continue
		}
		total += q.sumBySuffix[lowerSuffix]
		found = true
	}
	return total, found
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

func fatalf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func formatErrorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
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

func durationAllowZero(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value < 0 {
		fatalf("invalid %s=%q: expected non-negative duration", key, raw)
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

func parseNonNegativeFloatEnv(key string, fallback float64) float64 {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
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
	var root telemetryPayloadWire
	if err := json.Unmarshal(payload, &root); err != nil {
		return telemetryEnvelope{}, nil, err
	}

	envelope := telemetryEnvelope{
		ModuleType: root.ModuleType,
		NeedAck:    root.NeedAck,
		ID:         root.ID,
		Time:       root.Time,
		CmdID:      root.CmdID,
		CmdFunc:    root.CmdFunc,
		Addr:       strings.TrimSpace(root.Addr),
		Version:    strings.TrimSpace(root.Version),
		TypeCode:   strings.TrimSpace(root.TypeCode),
	}

	merged := mergeTelemetryMaps(root.Params, root.Param, root.Data, root.Quota)

	if envelope.Addr != "" {
		prefix := envelope.Addr + "."
		for key, value := range merged {
			if strings.HasPrefix(key, prefix) {
				continue
			}
			prefixed := prefix + key
			if _, exists := merged[prefixed]; !exists {
				merged[prefixed] = value
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

type telemetryPayloadWire struct {
	ModuleType int64          `json:"moduleType"`
	NeedAck    int64          `json:"needAck"`
	ID         int64          `json:"id"`
	Time       int64          `json:"time"`
	CmdID      int64          `json:"cmdId"`
	CmdFunc    int64          `json:"cmdFunc"`
	Addr       string         `json:"addr"`
	Version    string         `json:"version"`
	TypeCode   string         `json:"typeCode"`
	Params     map[string]any `json:"params"`
	Param      map[string]any `json:"param"`
	Data       map[string]any `json:"data"`
	Quota      map[string]any `json:"quota"`
}

func mergeTelemetryMaps(maps ...map[string]any) map[string]any {
	total := 0
	var single map[string]any
	singleCount := 0
	for _, entry := range maps {
		if len(entry) == 0 {
			continue
		}
		total += len(entry)
		single = entry
		singleCount++
	}
	switch singleCount {
	case 0:
		return map[string]any{}
	case 1:
		return single
	}

	merged := make(map[string]any, total)
	for _, entry := range maps {
		if len(entry) == 0 {
			continue
		}
		for key, value := range entry {
			merged[key] = value
		}
	}
	return merged
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
	updateNow := time.Now()
	scan := newUpdateQuotaLookup(quota)
	acInputUpdated := false
	wattsInUpdated := false

	if hasPDStatus {
		if pdStatus.Soc > 0 {
			s.DeviceSOC = float64(pdStatus.Soc)
			s.HasDeviceSOC = true
		}
		if pdStatus.ChgDsgState != 0 || hasQuotaKey(quota, "chgDsgState") {
			s.ChgDsgStateRaw = pdStatus.ChgDsgState
			s.HasChgDsgState = true
		}
		if rawChargeRemain, ok := scan.findAny("chgRemainTime"); ok {
			s.applyChargeRemain(toInt64(rawChargeRemain))
		}
		if rawDischargeRemain, ok := scan.findAny("dsgRemainTime"); ok {
			s.applyDischargeRemain(toInt64(rawDischargeRemain))
		}
		if scan.hasAny("wattsInSum") || pdStatus.WattsInSum != 0 {
			s.WattsIn = pdStatus.WattsInSum
			s.HasWattsIn = true
			wattsInUpdated = true
		}
		if scan.hasAny("wattsOutSum") || pdStatus.WattsOutSum != 0 {
			s.WattsOut = pdStatus.WattsOutSum
			s.HasWattsOut = true
		}
		if rawOutAC, ok := scan.findAny("invOutWatts", "outAcTtPwr", "outPrPwr"); ok {
			if outACWatts, ok := numberFromAny(rawOutAC); ok && outACWatts >= 0 {
				s.OutACWatts = normalizeOutputChannelWatts(outACWatts)
				s.HasOutAC = true
			}
		}
		if rawOutACL14, ok := scan.findAny("outAcL14Pwr"); ok {
			if outACL14Watts, ok := numberFromAny(rawOutACL14); ok && outACL14Watts >= 0 {
				s.OutACL14Watts = normalizeOutputChannelWatts(outACL14Watts)
				s.HasOutACL14 = true
			}
		}
		if rawACOn, ok := scan.findAny("cfgAcEnabled", "inv.cfgAcEnabled"); ok {
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
		if acIn, ok := scan.sumAbsBySuffixes("inAc5p8Pwr", "inAcC20Pwr"); ok {
			s.InACWatts = normalizeInputChannelWatts(acIn)
			s.HasInAC = true
			acInputUpdated = true
		} else if rawInvIn, ok := scan.findAny("invInWatts"); ok {
			if invInWatts, ok := numberFromAny(rawInvIn); ok && invInWatts >= 0 {
				s.InACWatts = normalizeInputChannelWatts(invInWatts)
				s.HasInAC = true
				acInputUpdated = true
			}
		}
		if dcOut, hasDCChannels := scan.sumAbsBySuffixes(dcOutputSuffixes...); hasDCChannels {
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
			case shouldInferDCOutputFromResidual(pdStatus, quota, scan) && s.HasWattsOut && s.HasOutAC && (!s.HasOutDC || !s.HasOutDCExplicit):
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
			setPackPower(pack, entry.CurPower)
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

	var fanStateValue int64
	hasFanStateInQuota := false
	var fanLevelValue int64
	hasFanLevelInQuota := false

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
			if watts, ok := numberFromAny(value); ok {
				s.WattsIn = watts
				s.HasWattsIn = true
				wattsInUpdated = true
			}
		case "wattsoutsum":
			if watts, ok := numberFromAny(value); ok {
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
		case "inlvmpptpwr", "pv1chargewatts", "powgetpvl":
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
		case "inhvmpptpwr", "pv2chargewatts", "powgetpvh":
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
		case "fanstate":
			if state, ok := numberFromAny(value); ok {
				fanStateValue = int64(state)
				hasFanStateInQuota = true
			}
		case "fanlevel":
			if level, ok := numberFromAny(value); ok {
				fanLevelValue = int64(level)
				hasFanLevelInQuota = true
			}
		case "inputwatts":
			if !s.HasWattsIn || s.WattsIn == 0 {
				if watts, ok := numberFromAny(value); ok {
					s.WattsIn = watts
					s.HasWattsIn = true
					wattsInUpdated = true
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

	// Resolve fan fields deterministically. If both keys exist in one payload,
	// explicit fanState is authoritative and must not be overridden by fanLevel.
	if hasFanLevelInQuota {
		s.FanLevelRaw = fanLevelValue
		s.HasFanLevel = true
	}
	if hasFanStateInQuota {
		s.FanOn = fanStateValue > 0
		s.HasFanOn = true
	} else if hasFanLevelInQuota {
		// Use fanLevel as fallback only when fanState is absent in this update.
		s.FanOn = fanLevelValue > 0
		s.HasFanOn = true
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
			setPackPower(existing, pack.PowerW)
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
		if pack.HasChargeState {
			existing.ChargeStateRaw = pack.ChargeStateRaw
			existing.HasChargeState = true
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
				setPackPower(pack, -outputWatts)
			} else {
				setPackPower(pack, outputWatts)
			}
		}
		if inputWatts, ok := firstNumberFromKeys(quota, "inputWatts", envelope.Addr+".inputWatts"); ok && inputWatts > 0 {
			setPackPower(pack, inputWatts)
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
				setPackPower(pack, inferredPower)
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
		if chargeStateRaw, ok := firstNumberFromKeys(quota, "bpChgSta", envelope.Addr+".bpChgSta"); ok {
			pack.ChargeStateRaw = int64(chargeStateRaw)
			pack.HasChargeState = true
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
			acInputUpdated = true
		}
	}
	if isMPPTStatusEnvelope(envelope) {
		s.applyMPPTStatusQuota(quota)
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
	if scan.hasXT150 {
		xt150 := scan.xt150Total
		s.XT150Watts = xt150
		s.HasXT150 = true
	}
	if outAC, ok := scan.sumAbsBySuffixes(acOutputSuffixes...); ok {
		s.OutACWatts = normalizeOutputChannelWatts(outAC)
		s.HasOutAC = true
	}
	if outACL14, ok := scan.sumAbsBySuffixes("outAcL14Pwr"); ok {
		s.OutACL14Watts = normalizeOutputChannelWatts(outACL14)
		s.HasOutACL14 = true
	}
	if outDC, ok := scan.sumAbsBySuffixes(dcOutputSuffixes...); ok {
		s.OutDCWatts = normalizeOutputChannelWatts(outDC)
		s.HasOutDC = true
		s.HasOutDCExplicit = true
	}
	if inAC, ok := scan.sumAbsBySuffixes(acInputSuffixes...); ok {
		s.InACWatts = normalizeInputChannelWatts(inAC)
		s.HasInAC = true
		acInputUpdated = true
	}
	if acInputUpdated {
		s.InACAt = updateNow
		s.HasInACAt = true
	}
	if wattsInUpdated {
		s.WattsInAt = updateNow
		s.HasWattsInAt = true
	}

	// AC input can become stale when streams omit explicit AC-in keys.
	// If current totals are fully explained by non-AC inputs (PV + XT150-in) or near zero,
	// clear lingering AC-in so UI/state reflects reality faster.
	if !acInputUpdated && s.HasInAC && s.InACWatts > 0 && s.HasWattsIn {
		inTotal := math.Abs(s.WattsIn)
		nonACIn := 0.0
		hasNonACSignal := false
		if pvIn, ok := s.effectivePVInputWatts(); ok {
			nonACIn += math.Abs(pvIn)
			hasNonACSignal = true
		}
		if s.HasXT150 && s.XT150Watts < 0 {
			nonACIn += math.Abs(s.XT150Watts)
			hasNonACSignal = true
		}
		tolerance := math.Max(acInputStaleClearToleranceW, inTotal*0.08)
		if inTotal <= tolerance || (hasNonACSignal && inTotal <= nonACIn+tolerance) {
			s.InACWatts = 0
			s.HasInAC = true
		}
	}
	// Fallback: when both AC-input and total-input signals are stale, clear lingering AC input.
	// This addresses sparse streams that only emit status bytes while power keys are absent.
	if !acInputUpdated && s.HasInAC && s.InACWatts > 0 {
		acStale := s.HasInACAt && updateNow.Sub(s.InACAt) >= acInputStaleClearAfter
		totalStale := !s.HasWattsInAt || updateNow.Sub(s.WattsInAt) >= acInputStaleClearAfter
		if acStale && totalStale {
			s.InACWatts = 0
			s.HasInAC = true
		}
	}

	// Keep total input aligned with channel truth when AC toggles off but wattsInSum lags behind.
	if !wattsInUpdated && s.HasWattsIn {
		if inferredIn, hasInferredIn := s.estimatedInputFromChannels(); hasInferredIn {
			mismatch := math.Abs(s.WattsIn - inferredIn)
			tolerance := math.Max(wattsInReconcileToleranceW, math.Max(math.Abs(s.WattsIn), inferredIn)*0.18)
			forceFromACToggle := acInputUpdated && s.HasInAC && s.InACWatts <= acInputStaleClearToleranceW
			totalStale := s.HasWattsInAt && updateNow.Sub(s.WattsInAt) >= acInputStaleClearAfter/2
			if mismatch > tolerance && (forceFromACToggle || totalStale) {
				s.WattsIn = inferredIn
				s.HasWattsIn = true
				s.WattsInAt = updateNow
				s.HasWattsInAt = true
				wattsInUpdated = true
			}
		}
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
	s.pushStateSmoothingSample()
	if !s.HasDeviceSOC {
		if avgSOC, ok := averagePackSOC(s.Packs); ok {
			s.DeviceSOC = avgSOC
			s.HasDeviceSOC = true
		}
	}
	s.recordSensorStatusTransitions(updateNow)
	s.DataUpdatedAt = updateNow
	s.HasDataUpdatedAt = true
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
	derived.StatusFanValue = "[ ]"
	if s.HasFanOn {
		derived.StatusFanValue = checkboxStatus(s.FanOn)
	}
	derived.StatusPassthroughValue = checkboxStatus(passthroughOn)
	// Grounded estimate is inferred from AC passthrough behavior.
	derived.StatusGroundedValue = checkboxStatus(passthroughOn)
	derived.StatusSolarPassValue = "[ ]"
	derived.StatusSolarChargingValue = "[ ]"
	solarChargingKnown, solarChargingOn := s.solarChargingStatus()
	if solarChargingKnown {
		derived.StatusSolarChargingValue = checkboxStatus(solarChargingOn)
	}
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

	flow := s.batteryFlowForDisplay(
		derived.EffectiveIn,
		derived.HasEffectiveIn,
		derived.EffectiveOut,
		derived.HasEffectiveOut,
		packChargeW,
		packDischargeW,
	)
	batteryInWatts := flow.inWatts
	batteryOutWatts := flow.outWatts
	hasBatteryIn := flow.hasIn
	hasBatteryOut := flow.hasOut
	derived.BatteryInValue = formatOptionalWatts(hasBatteryIn, batteryInWatts)
	derived.BatteryOutValue = formatOptionalWatts(hasBatteryOut, batteryOutWatts)
	derived.BatteryNetValue = "n/a"
	if flow.hasNet {
		// Positive when charging, negative when discharging.
		derived.BatteryNetValue = formatWatts(flow.netWatts)
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

func (s *energySnapshot) estimatedInputFromChannels() (float64, bool) {
	if s == nil {
		return 0, false
	}
	total := 0.0
	hasTotal := false
	if s.HasInAC {
		total += math.Abs(s.InACWatts)
		hasTotal = true
	}
	if pvIn, hasPVIn := s.effectivePVInputWatts(); hasPVIn {
		total += math.Abs(pvIn)
		hasTotal = true
	}
	if !hasTotal {
		return 0, false
	}
	return total, true
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
	currentMinuteStart := time.Now().Truncate(time.Minute).Unix()
	for _, bucket := range buckets {
		socPercent, hasSOCPercent := averageValue(bucket.SOCSumPercent, bucket.SOCSamples)
		if bucket.MinuteStartUnix == currentMinuteStart && bucket.SOCSamples > 0 {
			socPercent = bucket.SOCLastPercent
			hasSOCPercent = true
		}
		solarWh, hasSolarWh := averageWh(bucket.SolarSumWatts, bucket.SolarSamples)
		acInWh, hasACInWh := averageWh(bucket.ACInSumWatts, bucket.ACInSamples)
		acOutWh, hasACOutWh := averageWh(bucket.ACOutSumWatts, bucket.ACOutSamples)
		dcOutWh, hasDCOutWh := averageWh(bucket.DCOutSumWatts, bucket.DCOutSamples)
		batteryChargeWh, hasBatteryChargeWh := averageWh(bucket.BatteryChargeSumWatts, bucket.BatteryChargeSamples)
		batteryNetWh, hasBatteryNetWh := averageWh(bucket.BatteryNetSumWatts, bucket.BatteryNetSamples)

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
		netWh := batteryNetWh
		hasNetWh := hasBatteryNetWh

		out = append(out, []string{
			time.Unix(bucket.MinuteStartUnix, 0).Local().Format("2006-01-02 15:04"),
			formatPercentNoUnit(socPercent, hasSOCPercent),
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
	rawState := s.detectSystemStateRaw(effectiveIn, hasEffectiveIn, effectiveOut, hasEffectiveOut, packChargeW, packDischargeW)
	smoothedState, smoothedNet, hasSmoothed := s.smoothedSystemState()
	if !hasSmoothed || smoothedState == systemStateUnknown {
		return rawState
	}
	rawNet := 0.0
	hasRawNet := false
	if hasEffectiveIn && hasEffectiveOut {
		rawNet = effectiveIn - effectiveOut
		hasRawNet = true
	}

	// No strong instantaneous signal yet; rely on smoothed direction.
	if rawState == systemStateUnknown || rawState == systemStateIdle {
		return smoothedState
	}

	// Allow strong, fresh pack direction to break ties when aggregate channels lag.
	// Require a high margin to avoid overreacting to sparse/noisy pack updates.
	packOverrideMargin := systemStateNetThresholdWatts * 4.0
	strongBatteryChargeHint := s.HasBatteryIn && (!s.HasBatteryOut || s.BatteryInWatts > s.BatteryOutWatts+packOverrideMargin)
	strongBatteryDischargeHint := s.HasBatteryOut && (!s.HasBatteryIn || s.BatteryOutWatts > s.BatteryInWatts+packOverrideMargin)
	if rawState == systemStateDischarging && packChargeW > packDischargeW+packOverrideMargin && !strongBatteryDischargeHint {
		return systemStateCharging
	}
	if rawState == systemStateCharging && packDischargeW > packChargeW+packOverrideMargin && !strongBatteryChargeHint {
		return systemStateDischarging
	}
	if rawState == smoothedState {
		return rawState
	}

	// Delta 2 style trickle windows can disagree between stale smoothed trend and
	// fresh raw samples. If raw aggregate net and battery-flow net agree, trust
	// that direction to avoid charging<->discharging flap.
	if s.HasXT150 && hasRawNet {
		flow := s.batteryFlowForDisplay(
			effectiveIn,
			hasEffectiveIn,
			effectiveOut,
			hasEffectiveOut,
			packChargeW,
			packDischargeW,
		)
		agreeCharging := rawState == systemStateCharging && rawNet > systemStateNetThresholdWatts && flow.hasNet && flow.netWatts > systemStateNetThresholdWatts
		agreeDischarging := rawState == systemStateDischarging && rawNet < -systemStateNetThresholdWatts && flow.hasNet && flow.netWatts < -systemStateNetThresholdWatts
		if agreeCharging || agreeDischarging {
			return rawState
		}
	}

	// Preserve explicit pack direction only when aggregate in/out is missing.
	if (!hasEffectiveIn || !hasEffectiveOut) && rawState == systemStateDischarging && packDischargeW > packChargeW+systemStateNetThresholdWatts {
		return rawState
	}
	if (!hasEffectiveIn || !hasEffectiveOut) && rawState == systemStateCharging && packChargeW > packDischargeW+systemStateNetThresholdWatts {
		return rawState
	}

	// When both packs are discharging, avoid temporary flips to charging from stale total in/out counters.
	if smoothedState == systemStateDischarging && packDischargeW > idleDrawNoiseFloorWatts && packChargeW <= idleDrawNoiseFloorWatts {
		return systemStateDischarging
	}
	if smoothedState == systemStateCharging && packChargeW > idleDrawNoiseFloorWatts && packDischargeW <= idleDrawNoiseFloorWatts {
		return systemStateCharging
	}

	// For remaining conflicts, trust smoothed trend if it is directional enough.
	if math.Abs(smoothedNet) >= systemStateNetThresholdWatts {
		return smoothedState
	}
	return rawState
}

func (s *energySnapshot) detectSystemStateRaw(
	effectiveIn float64,
	hasEffectiveIn bool,
	effectiveOut float64,
	hasEffectiveOut bool,
	packChargeW float64,
	packDischargeW float64,
) systemStateKind {
	// Prefer aggregate channels first. Pack-level telemetry can arrive slower and
	// should be treated as fallback direction hints.
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

	packNet := packChargeW - packDischargeW
	if packNet > systemStateNetThresholdWatts {
		return systemStateCharging
	}
	if packNet < -systemStateNetThresholdWatts {
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

func (s *energySnapshot) pushStateSmoothingSample() {
	if s == nil || s.stateNetSmoother == nil {
		return
	}
	netWatts, ok := s.stateNetForSmoothingSample()
	if !ok {
		return
	}
	s.stateNetSmoother.Add(netWatts)
}

func (s *energySnapshot) stateNetForSmoothingSample() (float64, bool) {
	if s == nil {
		return 0, false
	}
	packChargeW, packDischargeW := packPowerTotals(s.Packs)
	effectiveIn, hasIn, effectiveOut, hasOut := s.effectiveTotalsForDisplayWithPackTotals(packChargeW, packDischargeW)
	switch {
	case hasIn && hasOut:
		return effectiveIn - effectiveOut, true
	case hasIn:
		return effectiveIn, true
	case hasOut:
		return -effectiveOut, true
	}
	// Fallback to pack direction when aggregate channels are unavailable.
	if packChargeW > idleDrawNoiseFloorWatts || packDischargeW > idleDrawNoiseFloorWatts {
		return packChargeW - packDischargeW, true
	}
	if s.HasBatteryIn || s.HasBatteryOut {
		batteryIn := 0.0
		if s.HasBatteryIn {
			batteryIn = s.BatteryInWatts
		}
		batteryOut := 0.0
		if s.HasBatteryOut {
			batteryOut = s.BatteryOutWatts
		}
		if batteryIn > idleDrawNoiseFloorWatts || batteryOut > idleDrawNoiseFloorWatts {
			return batteryIn - batteryOut, true
		}
	}
	return 0, false
}

func (s *energySnapshot) smoothedSystemState() (state systemStateKind, netWatts float64, ok bool) {
	if s == nil || s.stateNetSmoother == nil {
		return systemStateUnknown, 0, false
	}
	netWatts, ok = s.stateNetSmoother.Average()
	if !ok {
		return systemStateUnknown, 0, false
	}
	switch {
	case netWatts > systemStateSmoothThreshold:
		return systemStateCharging, netWatts, true
	case netWatts < -systemStateSmoothThreshold:
		return systemStateDischarging, netWatts, true
	case math.Abs(netWatts) <= idleDrawNoiseFloorWatts:
		return systemStateIdle, netWatts, true
	default:
		return systemStateUnknown, netWatts, true
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

func shouldShowFanStatus(device ecoflow.GeneralInfoDevice, snapshot *energySnapshot) bool {
	if snapshot != nil && (snapshot.HasFanOn || snapshot.HasFanLevel) {
		return true
	}
	name := strings.ToLower(strings.TrimSpace(device.DeviceName + " " + device.ProductName))
	return strings.Contains(name, "delta 2 max")
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

func shouldShowXT150Channels(device ecoflow.GeneralInfoDevice, snapshot *energySnapshot, derived snapshotDerived) bool {
	name := strings.ToLower(strings.TrimSpace(device.DeviceName + " " + device.ProductName))
	if strings.Contains(name, "delta pro ultra") || strings.Contains(name, "dpu") {
		return false
	}
	if strings.Contains(name, "delta 2 max") {
		return true
	}
	if snapshot != nil && snapshot.HasXT150 {
		return true
	}
	if strings.TrimSpace(derived.XT150InValue) != "" && !strings.EqualFold(strings.TrimSpace(derived.XT150InValue), "n/a") {
		return true
	}
	if strings.TrimSpace(derived.XT150OutValue) != "" && !strings.EqualFold(strings.TrimSpace(derived.XT150OutValue), "n/a") {
		return true
	}
	return false
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

func shouldInferDCOutputFromResidual(pdStatus pdStatusSummary, quota map[string]any, lookup *updateQuotaLookup) bool {
	hasAny := func(candidates ...string) bool {
		if lookup != nil {
			return lookup.hasAny(candidates...)
		}
		_, ok := findQuotaValueByCandidates(quota, candidates...)
		return ok
	}

	// Prefer direct same-message totals when present:
	// out_dc ~= wattsOutSum - invOutWatts.
	if hasAny("wattsOutSum") {
		if hasAny("invOutWatts") {
			return true
		}
	}

	// Some devices expose AC output under appshow keys instead of invOutWatts.
	if hasAny("wattsOutSum") {
		if hasAny("outAcTtPwr", "outPrPwr") {
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

func (s *energySnapshot) solarChargingStatus() (known bool, on bool) {
	if s == nil {
		return false, false
	}
	profile := s.solarChargingProfile()
	returnWithMemory := func(k bool, value bool) (bool, bool) {
		if k {
			s.solarChargingSticky = value
			s.hasSolarChargingSticky = true
		}
		return k, value
	}

	pvThreshold := profile.PVActiveMinWatts
	if s.hasSolarChargingSticky && s.solarChargingSticky && profile.PVHoldMinWatts > 0 {
		pvThreshold = profile.PVHoldMinWatts
	}

	anyPackCharging := false
	for _, pack := range s.Packs {
		if pack == nil || !pack.HasChargeState {
			continue
		}
		known = true
		if pack.ChargeStateRaw > 0 {
			anyPackCharging = true
		}
	}
	// Restrict to solar-driven charging (not AC-only charging).
	pvActive := false
	if s.HasPVLowChgState && isMPPTChargeStateActive(s.PVLowChgStateRaw) {
		pvActive = true
	}
	if s.HasPVHighChgState && isMPPTChargeStateActive(s.PVHighChgStateRaw) {
		pvActive = true
	}
	if !pvActive {
		// Prefer direct PV power channels when present, because V*I inferred power can
		// stay non-zero from stale volt/amp updates after appshow/d_addr reports 0W.
		directPVKnown := false
		directPVActive := false
		if s.HasInPVLow {
			directPVKnown = true
			if s.InPVLowWatts > pvThreshold {
				directPVActive = true
			}
		}
		if s.HasInPVHigh {
			directPVKnown = true
			if s.InPVHighWatts > pvThreshold {
				directPVActive = true
			}
		}
		if !directPVKnown && s.HasInPV {
			directPVKnown = true
			if s.InPVWatts > pvThreshold {
				directPVActive = true
			}
		}
		if directPVKnown {
			pvActive = directPVActive
		} else if pvWatts, hasPV := s.effectivePVInputWatts(); hasPV && pvWatts > pvThreshold {
			pvActive = true
		}
	}

	// Preferred signal path: explicit pack charge-state flags for models where
	// bpChgSta is known to be reliable.
	if known && profile.PreferPackChargeState {
		return returnWithMemory(true, anyPackCharging && pvActive)
	}
	if known && anyPackCharging {
		return returnWithMemory(true, pvActive)
	}

	if !profile.AllowFallbackInference {
		if known {
			return returnWithMemory(true, false)
		}
		return returnWithMemory(false, false)
	}

	if profile.PreferAggregateNetForFallback {
		if batteryNetWatts, hasBatteryNet := s.aggregateBatteryNetWatts(); hasBatteryNet {
			isCharging := batteryNetWatts > profile.BatteryChargeMinWatts
			return returnWithMemory(true, pvActive && isCharging)
		}
	}

	// Fallback path for models like Delta 2 Max where bpChgSta is often absent:
	// infer direction from effective battery net.
	if batteryNetWatts, hasBatteryNet := s.effectiveBatteryNetWatts(); hasBatteryNet {
		isCharging := batteryNetWatts > profile.BatteryChargeMinWatts
		return returnWithMemory(true, pvActive && isCharging)
	}
	if batteryChargeWatts, hasBatteryCharge := s.effectiveBatteryChargeWatts(); hasBatteryCharge {
		isCharging := batteryChargeWatts > profile.BatteryChargeMinWatts
		return returnWithMemory(true, pvActive && isCharging)
	}

	return returnWithMemory(false, false)
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

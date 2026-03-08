package inference

import (
	"context"
	"time"
)

type Kind string

const (
	KindBatteryExpansion Kind = "battery_expansion"
	KindSolarAddOn       Kind = "solar_add_on"
	KindSolarUpgrade     Kind = "solar_upgrade"
	KindEnergyShift      Kind = "energy_shift"
	KindMaintenance      Kind = "maintenance"
)

type Status string

const (
	StatusPending     Status = "pending"
	StatusReady       Status = "ready"
	StatusStale       Status = "stale"
	StatusUnavailable Status = "unavailable"
)

type ActionKind string

const (
	ActionKindInternalRoute ActionKind = "internal_route"
	ActionKindExternalURL   ActionKind = "external_url"
	ActionKindLearnMore     ActionKind = "learn_more"
	ActionKindDismiss       ActionKind = "dismiss"
)

type EvidenceSource string

const (
	EvidenceSourceLiveSnapshot       EvidenceSource = "live_snapshot"
	EvidenceSourceRollupHistory      EvidenceSource = "rollup_history"
	EvidenceSourceDeviceCapabilities EvidenceSource = "device_capabilities"
	EvidenceSourceProviderMetadata   EvidenceSource = "provider_metadata"
	EvidenceSourceModelOutput        EvidenceSource = "model_output"
	EvidenceSourceRuleEngine         EvidenceSource = "rule_engine"
	EvidenceSourceUserContext        EvidenceSource = "user_context"
)

type Filter struct {
	Kinds    []Kind
	MaxItems int
}

type Action struct {
	Kind   ActionKind
	Label  string
	Target string
	Params map[string]any
}

type Evidence struct {
	Source  EvidenceSource
	Summary string
	Metrics map[string]any
}

type Insight struct {
	ID           string
	DeviceID     string
	Kind         Kind
	Title        string
	Summary      string
	Score        float64
	Rank         uint32
	ModelKey     string
	ModelVersion string
	GeneratedAt  time.Time
	ExpiresAt    time.Time
	Tags         []string
	Evidence     []Evidence
	Actions      []Action
	Attributes   map[string]any
}

type DeviceInsights struct {
	DeviceID     string
	Status       Status
	StatusDetail string
	RefreshedAt  time.Time
	Insights     []Insight
}

type Reader interface {
	GetDeviceInsights(ctx context.Context, deviceID string, filter Filter) (DeviceInsights, error)
	ListFleetInsights(ctx context.Context, deviceIDs []string, filter Filter) ([]DeviceInsights, error)
}

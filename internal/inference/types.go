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
	Kinds    []Kind `json:"kinds,omitempty"`
	MaxItems int    `json:"max_items,omitempty"`
}

type Action struct {
	Kind   ActionKind     `json:"kind"`
	Label  string         `json:"label"`
	Target string         `json:"target"`
	Params map[string]any `json:"params,omitempty"`
}

type Evidence struct {
	Source  EvidenceSource `json:"source"`
	Summary string         `json:"summary"`
	Metrics map[string]any `json:"metrics,omitempty"`
}

type Insight struct {
	ID           string         `json:"id"`
	DeviceID     string         `json:"device_id"`
	Kind         Kind           `json:"kind"`
	Title        string         `json:"title"`
	Summary      string         `json:"summary"`
	Score        float64        `json:"score"`
	Rank         uint32         `json:"rank"`
	ModelKey     string         `json:"model_key,omitempty"`
	ModelVersion string         `json:"model_version,omitempty"`
	GeneratedAt  time.Time      `json:"generated_at,omitempty"`
	ExpiresAt    time.Time      `json:"expires_at,omitempty"`
	Tags         []string       `json:"tags,omitempty"`
	Evidence     []Evidence     `json:"evidence,omitempty"`
	Actions      []Action       `json:"actions,omitempty"`
	Attributes   map[string]any `json:"attributes,omitempty"`
}

type DeviceInsights struct {
	DeviceID     string    `json:"device_id"`
	Status       Status    `json:"status"`
	StatusDetail string    `json:"status_detail,omitempty"`
	RefreshedAt  time.Time `json:"refreshed_at,omitempty"`
	Insights     []Insight `json:"insights,omitempty"`
}

type Reader interface {
	GetDeviceInsights(ctx context.Context, deviceID string, filter Filter) (DeviceInsights, error)
	ListFleetInsights(ctx context.Context, deviceIDs []string, filter Filter) ([]DeviceInsights, error)
}

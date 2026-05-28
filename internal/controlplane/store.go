package controlplane

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"
)

const (
	ProviderEcoFlow    = "ecoflow"
	ProviderPulseMQTT  = "pulsemqtt"
	ProviderPecron     = "pecron"
	ProviderAnkerSolix = "anker_solix"
)

var (
	ErrUserNotFound             = errors.New("user not found")
	ErrCredentialNotFound       = errors.New("provider credential not found")
	ErrCredentialAlreadyExists  = errors.New("provider credential access key already exists")
	ErrDeviceNotFound           = errors.New("device not found")
	ErrPermissionDenied         = errors.New("permission denied")
	ErrVerifiedEmailNotFound    = errors.New("verified user email not found")
	ErrUserSubjectConflict      = errors.New("target keycloak subject already belongs to another user")
	ErrEdgeCollectorNotFound    = errors.New("edge collector not found")
	ErrEdgeDeviceSourceNotFound = errors.New("edge device source not found")
)

type ProviderCredential struct {
	ID       string
	UserID   string
	Provider string
	// AccessKeyMask is the only key material field exposed in user-facing APIs.
	AccessKeyMask string
	// AccessKey and SecretKey are internal-only credential material.
	// They must never be exposed through user-facing API responses.
	AccessKey string
	SecretKey string
	Config    map[string]any
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ProviderDevice struct {
	ID                 string
	DeviceID           string
	Provider           string
	ProviderDeviceID   string
	CredentialID       string
	CanonicalSN        string
	ProductName        string
	Model              string
	Capabilities       map[string]any
	Metadata           map[string]any
	IsActive           bool
	IngestDesiredState string
}

type UserDevice struct {
	DeviceID    string
	EcoflowSN   string
	ProductName string
	Model       string
	Role        string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type EdgeCollector struct {
	ID                  string
	UserID              string
	DisplayName         string
	SetupTokenHash      string
	CollectorSecretHash string
	IsActive            bool
	CollectorVersion    string
	Hostname            string
	LastHeartbeatAt     time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type EdgeDeviceSource struct {
	ID               string
	CollectorID      string
	UserID           string
	Provider         string
	Transport        string
	ProviderDeviceID string
	DisplayName      string
	Model            string
	AddressHash      string
	RSSIDBm          int32
	Metadata         map[string]any
	Status           string
	LinkedDeviceID   string
	LastSeenAt       time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type CurrentUser struct {
	ID                     string
	KeycloakSubject        string
	Email                  string
	EmailVerified          bool
	DisplayName            string
	DisplayNameSource      string
	AvatarURL              string
	GivenName              string
	FamilyName             string
	Locale                 string
	Timezone               string
	WeatherLocationEnabled bool
	WeatherLocationSource  string
	WeatherLocationLabel   string
	WeatherLatitude        float64
	WeatherLongitude       float64
	HasWeatherLocation     bool
	LastLoginAt            time.Time
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

// IngestAssignment is an internal worker-facing projection for distributed
// MQTT session orchestration. It includes credential material and desired
// ingest state and must never be returned by user-facing APIs.
type IngestAssignment struct {
	Provider           string
	ProviderDeviceID   string
	DeviceID           string
	CredentialID       string
	ProductName        string
	Model              string
	AccessKey          string
	SecretKey          string
	CredentialConfig   map[string]any
	DeviceIsActive     bool
	CredentialIsActive bool
	IngestDesiredState string
}

type CreateProviderCredentialInput struct {
	UserSubject string
	Provider    string
	AccessKey   string
	SecretKey   string
	Config      map[string]any
	IsActive    bool
}

type ListProviderCredentialsInput struct {
	UserSubject string
	Provider    string
}

type SetProviderCredentialActiveInput struct {
	UserSubject  string
	CredentialID string
	IsActive     bool
}

type UpdateProviderCredentialInput struct {
	UserSubject  string
	CredentialID string
	AccessKey    string
	SecretKey    string
	Config       map[string]any
	IsActive     bool
}

type ListProviderDevicesInput struct {
	UserSubject string
	Provider    string
	ActiveOnly  bool
}

type CreateDeviceInput struct {
	UserSubject string
	EcoflowSN   string
	ProductName string
	Model       string
}

type LinkDeviceInput struct {
	UserSubject       string
	TargetUserSubject string
	DeviceID          string
	Role              string
}

type ListUserDevicesInput struct {
	UserSubject string
}

type GetOrProvisionCurrentUserInput struct {
	UserSubject   string
	Email         string
	EmailVerified bool
	DisplayName   string
	AvatarURL     string
	GivenName     string
	FamilyName    string
	Locale        string
}

type UpdateCurrentUserProfileInput struct {
	UserSubject             string
	DisplayName             string
	Timezone                string
	WeatherLocationEnabled  bool
	WeatherLocationSource   string
	WeatherLocationLabel    string
	WeatherLatitude         float64
	WeatherLongitude        float64
	HasWeatherLocationValue bool
}

type ReconcileUserSubjectByEmailInput struct {
	Email       string
	UserSubject string
}

type UpsertProviderDeviceInput struct {
	DeviceID           string
	Provider           string
	ProviderDeviceID   string
	CredentialID       string
	ProductName        string
	Model              string
	Capabilities       map[string]any
	Metadata           map[string]any
	IsActive           bool
	IngestDesiredState string
}

type ImportProviderDeviceInput struct {
	UserSubject        string
	Provider           string
	ProviderDeviceID   string
	CredentialID       string
	CanonicalSN        string
	ProductName        string
	Model              string
	Capabilities       map[string]any
	Metadata           map[string]any
	IsActive           bool
	IngestDesiredState string
}

type ImportedProviderDevice struct {
	ProviderDevice ProviderDevice
	UserDevice     UserDevice
}

type ListIngestAssignmentsInput struct {
	Provider   string
	ActiveOnly bool
}

type AdminLogFilterOption struct {
	Kind           string
	ID             string
	Label          string
	SecondaryLabel string
	DeviceIDs      []string
	Provider       string
}

type SearchAdminLogFiltersInput struct {
	Query       string
	Kind        string
	Limit       int
	Provider    string
	DeviceIDs   []string
	UserSubject string
	GlobalAdmin bool
}

type CreateEdgeCollectorInput struct {
	UserSubject    string
	DisplayName    string
	SetupTokenHash string
}

type EnrollEdgeCollectorInput struct {
	SetupTokenHash      string
	CollectorSecretHash string
	CollectorVersion    string
	Hostname            string
}

type AuthenticateEdgeCollectorInput struct {
	CollectorSecretHash string
}

type UpdateEdgeCollectorHeartbeatInput struct {
	CollectorID      string
	CollectorVersion string
	Hostname         string
}

type UpsertEdgeDeviceSourceInput struct {
	CollectorID      string
	Provider         string
	Transport        string
	ProviderDeviceID string
	DisplayName      string
	Model            string
	AddressHash      string
	RSSIDBm          int32
	Metadata         map[string]any
	ObservedAt       time.Time
}

type ListEdgeCollectorsInput struct {
	UserSubject string
}

type ListEdgeDeviceSourcesInput struct {
	UserSubject string
	CollectorID string
	Status      string
}

type ApproveEdgeDeviceSourceInput struct {
	UserSubject string
	SourceID    string
	DeviceID    string
	ProductName string
	Model       string
}

type ApprovedEdgeDeviceSource struct {
	Source EdgeDeviceSource
	Device UserDevice
}

type GetLinkedEdgeDeviceSourceInput struct {
	CollectorID      string
	Provider         string
	Transport        string
	ProviderDeviceID string
}

type Store interface {
	CreateProviderCredential(ctx context.Context, in CreateProviderCredentialInput) (ProviderCredential, error)
	ListProviderCredentials(ctx context.Context, in ListProviderCredentialsInput) ([]ProviderCredential, error)
	SetProviderCredentialActive(ctx context.Context, in SetProviderCredentialActiveInput) (ProviderCredential, error)
	UpdateProviderCredential(ctx context.Context, in UpdateProviderCredentialInput) (ProviderCredential, error)
	GetProviderCredential(ctx context.Context, userSubject string, credentialID string) (ProviderCredential, error)
	CreateDevice(ctx context.Context, in CreateDeviceInput) (UserDevice, error)
	LinkDevice(ctx context.Context, in LinkDeviceInput) (UserDevice, error)
	ListUserDevices(ctx context.Context, in ListUserDevicesInput) ([]UserDevice, error)
	GetOrProvisionCurrentUser(ctx context.Context, in GetOrProvisionCurrentUserInput) (CurrentUser, error)
	UpdateCurrentUserProfile(ctx context.Context, in UpdateCurrentUserProfileInput) (CurrentUser, error)
	ReconcileUserSubjectByEmail(ctx context.Context, in ReconcileUserSubjectByEmailInput) (CurrentUser, error)
	UpsertProviderDevice(ctx context.Context, in UpsertProviderDeviceInput) (ProviderDevice, error)
	ImportProviderDevice(ctx context.Context, in ImportProviderDeviceInput) (ImportedProviderDevice, error)
	ListProviderDevices(ctx context.Context, in ListProviderDevicesInput) ([]ProviderDevice, error)
	GetProviderDeviceByDeviceID(ctx context.Context, deviceID string) (ProviderDevice, error)
	ListIngestAssignments(ctx context.Context, in ListIngestAssignmentsInput) ([]IngestAssignment, error)
	SearchAdminLogFilters(ctx context.Context, in SearchAdminLogFiltersInput) ([]AdminLogFilterOption, error)
}

func normalizeAdminLogFilterKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "device":
		return "device"
	case "serial":
		return "serial"
	case "user":
		return "user"
	default:
		return ""
	}
}

func normalizeAdminLogFilterLimit(limit int) int {
	switch {
	case limit <= 0:
		return 12
	case limit > 50:
		return 50
	default:
		return limit
	}
}

func appendAdminLogOptions(out []AdminLogFilterOption, options []AdminLogFilterOption, limit int) []AdminLogFilterOption {
	for _, option := range options {
		if len(out) >= limit {
			return out
		}
		out = append(out, option)
	}
	return out
}

func normalizeAdminLogDeviceIDs(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		deviceID := strings.TrimSpace(value)
		if deviceID == "" {
			continue
		}
		if _, ok := seen[deviceID]; ok {
			continue
		}
		seen[deviceID] = struct{}{}
		out = append(out, deviceID)
	}
	sort.Strings(out)
	return out
}

func adminLogDeviceIDSet(values []string) map[string]struct{} {
	normalized := normalizeAdminLogDeviceIDs(values)
	if len(normalized) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(normalized))
	for _, value := range normalized {
		out[value] = struct{}{}
	}
	return out
}

func matchesAdminLogQuery(query string, values ...string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return true
	}
	for _, value := range values {
		if strings.Contains(strings.ToLower(strings.TrimSpace(value)), query) {
			return true
		}
	}
	return false
}

func adminLogFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func shortID(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 8 {
		return value
	}
	return value[:8]
}

package controlplane

import (
	"context"
	"errors"
	"time"
)

const (
	ProviderEcoFlow = "ecoflow"
)

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrCredentialNotFound = errors.New("provider credential not found")
	ErrDeviceNotFound     = errors.New("device not found")
	ErrPermissionDenied   = errors.New("permission denied")
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
	DeviceIsActive     bool
	CredentialIsActive bool
	IngestDesiredState string
}

type CreateProviderCredentialInput struct {
	UserSubject string
	Provider    string
	AccessKey   string
	SecretKey   string
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

type ListIngestAssignmentsInput struct {
	Provider   string
	ActiveOnly bool
}

type Store interface {
	CreateProviderCredential(ctx context.Context, in CreateProviderCredentialInput) (ProviderCredential, error)
	ListProviderCredentials(ctx context.Context, in ListProviderCredentialsInput) ([]ProviderCredential, error)
	SetProviderCredentialActive(ctx context.Context, in SetProviderCredentialActiveInput) (ProviderCredential, error)
	GetProviderCredential(ctx context.Context, userSubject string, credentialID string) (ProviderCredential, error)
	CreateDevice(ctx context.Context, in CreateDeviceInput) (UserDevice, error)
	LinkDevice(ctx context.Context, in LinkDeviceInput) (UserDevice, error)
	ListUserDevices(ctx context.Context, in ListUserDevicesInput) ([]UserDevice, error)
	GetOrProvisionCurrentUser(ctx context.Context, in GetOrProvisionCurrentUserInput) (CurrentUser, error)
	UpdateCurrentUserProfile(ctx context.Context, in UpdateCurrentUserProfileInput) (CurrentUser, error)
	UpsertProviderDevice(ctx context.Context, in UpsertProviderDeviceInput) (ProviderDevice, error)
	ListProviderDevices(ctx context.Context, in ListProviderDevicesInput) ([]ProviderDevice, error)
	GetProviderDeviceByDeviceID(ctx context.Context, deviceID string) (ProviderDevice, error)
	ListIngestAssignments(ctx context.Context, in ListIngestAssignmentsInput) ([]IngestAssignment, error)
}

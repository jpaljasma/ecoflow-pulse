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
)

type ProviderCredential struct {
	ID            string
	UserID        string
	Provider      string
	AccessKeyMask string
	IsActive      bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
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
	IsActive           bool
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

type Store interface {
	CreateProviderCredential(ctx context.Context, in CreateProviderCredentialInput) (ProviderCredential, error)
	ListProviderCredentials(ctx context.Context, in ListProviderCredentialsInput) ([]ProviderCredential, error)
	SetProviderCredentialActive(ctx context.Context, in SetProviderCredentialActiveInput) (ProviderCredential, error)
	GetProviderCredential(ctx context.Context, userSubject string, credentialID string) (ProviderCredential, error)
	ListProviderDevices(ctx context.Context, in ListProviderDevicesInput) ([]ProviderDevice, error)
}

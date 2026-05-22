package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	controlplanev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/controlplane/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/dbpool"
	"github.com/jpaljasma/ecoflow-pulse/internal/grpcmw"
	"github.com/jpaljasma/ecoflow-pulse/internal/provideradapter"
	"github.com/jpaljasma/ecoflow-pulse/pkg/ankersolix"
	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflow"
	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflowmqtt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	defaultMQTTProbeTimeout        = 12 * time.Second
	defaultMQTTProbeKeepAlive      = 90 * time.Second
	defaultMQTTProbeConnectTimeout = 10 * time.Second
	defaultMQTTProbeReadTimeout    = 10 * time.Second
	defaultMQTTProbeWriteTimeout   = 10 * time.Second
)

type mqttProbeSubscriber interface {
	Connect(ctx context.Context) error
	Subscribe(ctx context.Context, topic string, qos byte) error
	ReadMessage(ctx context.Context) (ecoflowmqtt.Message, error)
	Close() error
}

type mqttProbeSubscriberFactory func(cfg ecoflowmqtt.Config) (mqttProbeSubscriber, error)

type mqttCertificationResolver interface {
	GetMQTTCertification(ctx context.Context, credential controlplane.ProviderCredential, providerDeviceID string) (ecoflow.GeneralInfoMQTTCertification, error)
}

type providerMQTTProber interface {
	ProbeMQTT(ctx context.Context, credential controlplane.ProviderCredential, providerDeviceID string, timeout time.Duration) (provideradapter.MQTTProbeResult, error)
}

type mqttResolverTLSConfigProvider interface {
	MQTTTLSConfig() *tls.Config
}

type ControlPlaneService struct {
	controlplanev1.UnimplementedControlPlaneServiceServer

	log      *slog.Logger
	store    controlplane.Store
	adapters *provideradapter.Registry

	newMQTTSubscriber mqttProbeSubscriberFactory
	mqttProbeTimeout  time.Duration
}

func NewControlPlaneService(log *slog.Logger, store controlplane.Store, adapters *provideradapter.Registry) *ControlPlaneService {
	if adapters == nil {
		adapters = provideradapter.NewRegistry()
	}
	return &ControlPlaneService{
		log:      log,
		store:    store,
		adapters: adapters,
		newMQTTSubscriber: func(cfg ecoflowmqtt.Config) (mqttProbeSubscriber, error) {
			return ecoflowmqtt.NewSubscriber(cfg)
		},
		mqttProbeTimeout: defaultMQTTProbeTimeout,
	}
}

func (s *ControlPlaneService) RegisterProvider(provider string) {
	if s == nil || s.adapters == nil {
		return
	}
	s.adapters.RegisterProvider(provider)
}

func (s *ControlPlaneService) RegisterDiscoverer(provider string, discoverer provideradapter.Discoverer) {
	if s == nil || s.adapters == nil {
		return
	}
	s.adapters.RegisterDiscoverer(provider, discoverer)
}

func (s *ControlPlaneService) supportsProvider(provider string) bool {
	provider = controlplane.NormalizeProvider(provider)
	if provider == "" {
		return false
	}
	if s != nil && s.adapters != nil && s.adapters.Supports(provider) {
		return true
	}
	return controlplane.IsSupportedProvider(provider)
}

func controlPlaneStoreError(operation string, err error) error {
	if dbpool.IsTransientReadError(err) {
		return status.Error(codes.Unavailable, operation+" temporarily unavailable")
	}
	return status.Errorf(codes.Internal, "%s: %v", operation, err)
}

func (s *ControlPlaneService) GetCurrentUser(ctx context.Context, req *controlplanev1.GetCurrentUserRequest) (*controlplanev1.GetCurrentUserResponse, error) {
	userSubject, err := resolveUserSubject(ctx, req.GetUserSubject())
	if err != nil {
		return nil, err
	}
	claims, _ := grpcmw.ClaimsFromContext(ctx)
	user, err := s.store.GetOrProvisionCurrentUser(ctx, controlplane.GetOrProvisionCurrentUserInput{
		UserSubject:   userSubject,
		Email:         claims.Email,
		EmailVerified: claims.EmailVerified,
		DisplayName:   claims.DisplayName,
		AvatarURL:     claims.AvatarURL,
		GivenName:     claims.GivenName,
		FamilyName:    claims.FamilyName,
		Locale:        claims.Locale,
	})
	if err != nil {
		return nil, controlPlaneStoreError("get current user", err)
	}
	devices, err := s.store.ListUserDevices(ctx, controlplane.ListUserDevicesInput{UserSubject: userSubject})
	if err != nil {
		return nil, controlPlaneStoreError("list current user devices", err)
	}
	return &controlplanev1.GetCurrentUserResponse{
		User: currentUserToProto(user, claims.AuthMethod),
		Authorization: &controlplanev1.AuthorizationSummary{
			TokenRoles:  claims.Roles,
			DeviceCount: uint32(len(devices)),
		},
	}, nil
}

func (s *ControlPlaneService) UpdateCurrentUser(ctx context.Context, req *controlplanev1.UpdateCurrentUserRequest) (*controlplanev1.UpdateCurrentUserResponse, error) {
	userSubject, err := resolveUserSubject(ctx, req.GetUserSubject())
	if err != nil {
		return nil, err
	}
	timezone := strings.TrimSpace(req.GetTimezone())
	if timezone == "" {
		return nil, status.Error(codes.InvalidArgument, "timezone required")
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return nil, status.Error(codes.InvalidArgument, "timezone must be a valid IANA timezone")
	}
	weatherSource := strings.TrimSpace(req.GetWeatherLocationSource())
	if req.GetWeatherLocationEnabled() && req.GetHasWeatherLocation() && weatherSource != "" && weatherSource != "auto" && weatherSource != "none" {
		return nil, status.Error(codes.InvalidArgument, "weather_location_source must be none or auto")
	}
	user, err := s.store.UpdateCurrentUserProfile(ctx, controlplane.UpdateCurrentUserProfileInput{
		UserSubject:             userSubject,
		DisplayName:             req.GetDisplayName(),
		Timezone:                timezone,
		WeatherLocationEnabled:  req.GetWeatherLocationEnabled(),
		WeatherLocationSource:   weatherSource,
		WeatherLocationLabel:    req.GetWeatherLocationLabel(),
		WeatherLatitude:         req.GetWeatherLatitude(),
		WeatherLongitude:        req.GetWeatherLongitude(),
		HasWeatherLocationValue: req.GetHasWeatherLocation(),
	})
	if err != nil {
		if errors.Is(err, controlplane.ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Errorf(codes.Internal, "update current user: %v", err)
	}
	claims, _ := grpcmw.ClaimsFromContext(ctx)
	return &controlplanev1.UpdateCurrentUserResponse{User: currentUserToProto(user, claims.AuthMethod)}, nil
}

func (s *ControlPlaneService) RefreshCurrentUserIdentity(ctx context.Context, req *controlplanev1.RefreshCurrentUserIdentityRequest) (*controlplanev1.RefreshCurrentUserIdentityResponse, error) {
	userSubject, err := resolveUserSubject(ctx, req.GetUserSubject())
	if err != nil {
		return nil, err
	}
	claims, _ := grpcmw.ClaimsFromContext(ctx)
	user, err := s.store.GetOrProvisionCurrentUser(ctx, controlplane.GetOrProvisionCurrentUserInput{
		UserSubject:   userSubject,
		Email:         firstNonEmpty(req.GetEmail(), claims.Email),
		EmailVerified: req.GetEmailVerified() || claims.EmailVerified,
		DisplayName:   firstNonEmpty(req.GetDisplayName(), claims.DisplayName),
		AvatarURL:     firstNonEmpty(req.GetAvatarUrl(), claims.AvatarURL),
		GivenName:     firstNonEmpty(req.GetGivenName(), claims.GivenName),
		FamilyName:    firstNonEmpty(req.GetFamilyName(), claims.FamilyName),
		Locale:        firstNonEmpty(req.GetLocale(), claims.Locale),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "refresh current user identity: %v", err)
	}
	return &controlplanev1.RefreshCurrentUserIdentityResponse{
		User: currentUserToProto(user, claims.AuthMethod),
	}, nil
}

func (s *ControlPlaneService) CreateProviderCredential(ctx context.Context, req *controlplanev1.CreateProviderCredentialRequest) (*controlplanev1.CreateProviderCredentialResponse, error) {
	userSubject, err := resolveUserSubject(ctx, req.GetUserSubject())
	if err != nil {
		return nil, err
	}
	provider := controlplane.NormalizeProvider(req.GetProvider())
	if !s.supportsProvider(provider) {
		return nil, status.Error(codes.InvalidArgument, "unsupported provider")
	}
	config, err := normalizeProviderCredentialConfig(provider, structToMap(req.GetConfig()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if strings.TrimSpace(req.GetAccessKey()) == "" {
		return nil, status.Error(codes.InvalidArgument, "access_key required")
	}
	if strings.TrimSpace(req.GetSecretKey()) == "" {
		return nil, status.Error(codes.InvalidArgument, "secret_key required")
	}
	if req.GetIsActive() {
		if err := s.validateProviderCredentialActivation(ctx, userSubject, controlplane.ProviderCredential{
			Provider:  provider,
			AccessKey: req.GetAccessKey(),
			SecretKey: req.GetSecretKey(),
			Config:    config,
			IsActive:  true,
		}); err != nil {
			return nil, err
		}
	}
	out, err := s.store.CreateProviderCredential(ctx, controlplane.CreateProviderCredentialInput{
		UserSubject: userSubject,
		Provider:    provider,
		AccessKey:   req.GetAccessKey(),
		SecretKey:   req.GetSecretKey(),
		Config:      config,
		IsActive:    req.GetIsActive(),
	})
	if err != nil {
		if errors.Is(err, controlplane.ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		if errors.Is(err, controlplane.ErrCredentialAlreadyExists) {
			return nil, status.Error(codes.AlreadyExists, err.Error())
		}
		return nil, status.Errorf(codes.Internal, "create provider credential: %v", err)
	}
	return &controlplanev1.CreateProviderCredentialResponse{
		Credential: providerCredentialToProto(out),
	}, nil
}

func (s *ControlPlaneService) ListProviderCredentials(ctx context.Context, req *controlplanev1.ListProviderCredentialsRequest) (*controlplanev1.ListProviderCredentialsResponse, error) {
	userSubject, err := resolveUserSubject(ctx, req.GetUserSubject())
	if err != nil {
		return nil, err
	}
	provider := controlplane.NormalizeProvider(req.GetProvider())
	if provider != "" && !s.supportsProvider(provider) {
		return nil, status.Error(codes.InvalidArgument, "unsupported provider")
	}
	rows, err := s.store.ListProviderCredentials(ctx, controlplane.ListProviderCredentialsInput{
		UserSubject: userSubject,
		Provider:    provider,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list provider credentials: %v", err)
	}
	credentials := make([]*controlplanev1.ProviderCredential, 0, len(rows))
	for i := range rows {
		credentials = append(credentials, providerCredentialToProto(rows[i]))
	}
	return &controlplanev1.ListProviderCredentialsResponse{Credentials: credentials}, nil
}

func (s *ControlPlaneService) SetProviderCredentialActive(ctx context.Context, req *controlplanev1.SetProviderCredentialActiveRequest) (*controlplanev1.SetProviderCredentialActiveResponse, error) {
	userSubject, err := resolveUserSubject(ctx, req.GetUserSubject())
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.GetCredentialId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "credential_id required")
	}
	if req.GetIsActive() {
		cred, _, err := s.getProviderCredentialForUser(ctx, userSubject, "", req.GetCredentialId())
		if err != nil {
			return nil, err
		}
		cred.IsActive = true
		if err := s.validateProviderCredentialActivation(ctx, userSubject, cred); err != nil {
			return nil, err
		}
	}
	row, err := s.store.SetProviderCredentialActive(ctx, controlplane.SetProviderCredentialActiveInput{
		UserSubject:  userSubject,
		CredentialID: req.GetCredentialId(),
		IsActive:     req.GetIsActive(),
	})
	if err != nil {
		if errors.Is(err, controlplane.ErrCredentialNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Errorf(codes.Internal, "set provider credential active: %v", err)
	}
	return &controlplanev1.SetProviderCredentialActiveResponse{
		Credential: providerCredentialToProto(row),
	}, nil
}

func (s *ControlPlaneService) UpdateProviderCredential(ctx context.Context, req *controlplanev1.UpdateProviderCredentialRequest) (*controlplanev1.UpdateProviderCredentialResponse, error) {
	userSubject, err := resolveUserSubject(ctx, req.GetUserSubject())
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.GetCredentialId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "credential_id required")
	}
	if strings.TrimSpace(req.GetAccessKey()) == "" {
		return nil, status.Error(codes.InvalidArgument, "access_key required")
	}
	if strings.TrimSpace(req.GetSecretKey()) == "" {
		return nil, status.Error(codes.InvalidArgument, "secret_key required")
	}
	existing, _, err := s.getProviderCredentialForUser(ctx, userSubject, "", req.GetCredentialId())
	if err != nil {
		return nil, err
	}
	config := cloneMap(existing.Config)
	if req.GetConfig() != nil {
		incomingConfig := structToMap(req.GetConfig())
		if controlplane.NormalizeProvider(existing.Provider) == controlplane.ProviderAnkerSolix {
			config = mergeProviderCredentialConfig(config, incomingConfig)
		} else {
			config = incomingConfig
		}
	}
	config, err = normalizeProviderCredentialConfig(existing.Provider, config)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if req.GetIsActive() {
		if err := s.validateProviderCredentialActivation(ctx, userSubject, controlplane.ProviderCredential{
			ID:        existing.ID,
			UserID:    existing.UserID,
			Provider:  existing.Provider,
			AccessKey: req.GetAccessKey(),
			SecretKey: req.GetSecretKey(),
			Config:    config,
			IsActive:  true,
		}); err != nil {
			return nil, err
		}
	}
	row, err := s.store.UpdateProviderCredential(ctx, controlplane.UpdateProviderCredentialInput{
		UserSubject:  userSubject,
		CredentialID: req.GetCredentialId(),
		AccessKey:    req.GetAccessKey(),
		SecretKey:    req.GetSecretKey(),
		Config:       config,
		IsActive:     req.GetIsActive(),
	})
	if err != nil {
		if errors.Is(err, controlplane.ErrCredentialNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		if errors.Is(err, controlplane.ErrCredentialAlreadyExists) {
			return nil, status.Error(codes.AlreadyExists, err.Error())
		}
		return nil, status.Errorf(codes.Internal, "update provider credential: %v", err)
	}
	return &controlplanev1.UpdateProviderCredentialResponse{
		Credential: providerCredentialToProto(row),
	}, nil
}

func (s *ControlPlaneService) validateProviderCredentialActivation(
	ctx context.Context,
	userSubject string,
	cred controlplane.ProviderCredential,
) error {
	provider := controlplane.NormalizeProvider(cred.Provider)
	if provider == "" {
		return status.Error(codes.InvalidArgument, "provider required for credential activation")
	}
	discoverer, ok := s.adapters.Discoverer(provider)
	if !ok {
		return status.Error(codes.Unimplemented, "provider discoverer not configured")
	}
	discovered, err := discoverer.DiscoverDevices(ctx, cred)
	if err != nil {
		switch {
		case errors.Is(err, provideradapter.ErrInactiveCredential), errors.Is(err, provideradapter.ErrMissingCredentialMaterial):
			return status.Error(codes.FailedPrecondition, err.Error())
		default:
			return status.Errorf(codes.Internal, "validate provider credential discovery: %v", err)
		}
	}
	enabled, err := s.store.ListProviderDevices(ctx, controlplane.ListProviderDevicesInput{
		UserSubject: userSubject,
		Provider:    provider,
		ActiveOnly:  true,
	})
	if err != nil {
		return status.Errorf(codes.Internal, "list enabled provider devices for validation: %v", err)
	}
	if len(enabled) == 0 {
		return nil
	}
	discoveredByID := make(map[string]struct{}, len(discovered))
	for i := range discovered {
		discoveredByID[strings.ToUpper(strings.TrimSpace(discovered[i].ProviderDeviceID))] = struct{}{}
	}
	for i := range enabled {
		targetID := strings.ToUpper(strings.TrimSpace(enabled[i].ProviderDeviceID))
		if _, ok := discoveredByID[targetID]; !ok {
			return status.Errorf(codes.FailedPrecondition, "provider connection does not expose enabled device %s", targetID)
		}
		probe, err := s.probeProviderDeviceMQTT(ctx, provider, cred, enabled[i].ProviderDeviceID)
		if err != nil {
			return status.Errorf(codes.FailedPrecondition, "mqtt validation failed for enabled device %s: %v", targetID, err)
		}
		if !probe.GetSuccess() {
			return status.Errorf(codes.FailedPrecondition, "mqtt validation failed for enabled device %s: %s", targetID, probe.GetStatus())
		}
	}
	return nil
}

func (s *ControlPlaneService) CreateDevice(ctx context.Context, req *controlplanev1.CreateDeviceRequest) (*controlplanev1.CreateDeviceResponse, error) {
	userSubject, err := resolveUserSubject(ctx, req.GetUserSubject())
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.GetEcoflowSn()) == "" {
		return nil, status.Error(codes.InvalidArgument, "ecoflow_sn required")
	}
	row, err := s.store.CreateDevice(ctx, controlplane.CreateDeviceInput{
		UserSubject: userSubject,
		EcoflowSN:   strings.ToUpper(strings.TrimSpace(req.GetEcoflowSn())),
		ProductName: req.GetProductName(),
		Model:       req.GetModel(),
	})
	if err != nil {
		if errors.Is(err, controlplane.ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Errorf(codes.Internal, "create device: %v", err)
	}
	return &controlplanev1.CreateDeviceResponse{Device: userDeviceToProto(row)}, nil
}

func (s *ControlPlaneService) LinkDevice(ctx context.Context, req *controlplanev1.LinkDeviceRequest) (*controlplanev1.LinkDeviceResponse, error) {
	userSubject, err := resolveUserSubject(ctx, req.GetUserSubject())
	if err != nil {
		return nil, err
	}
	role, err := normalizeDeviceRole(req.GetRole())
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.GetDeviceId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id required")
	}
	row, err := s.store.LinkDevice(ctx, controlplane.LinkDeviceInput{
		UserSubject:       userSubject,
		TargetUserSubject: strings.TrimSpace(req.GetTargetUserSubject()),
		DeviceID:          strings.TrimSpace(req.GetDeviceId()),
		Role:              role,
	})
	if err != nil {
		switch {
		case errors.Is(err, controlplane.ErrUserNotFound):
			return nil, status.Error(codes.NotFound, err.Error())
		case errors.Is(err, controlplane.ErrDeviceNotFound):
			return nil, status.Error(codes.NotFound, err.Error())
		case errors.Is(err, controlplane.ErrPermissionDenied):
			return nil, status.Error(codes.PermissionDenied, err.Error())
		default:
			return nil, status.Errorf(codes.Internal, "link device: %v", err)
		}
	}
	return &controlplanev1.LinkDeviceResponse{Device: userDeviceToProto(row)}, nil
}

func (s *ControlPlaneService) ListUserDevices(ctx context.Context, req *controlplanev1.ListUserDevicesRequest) (*controlplanev1.ListUserDevicesResponse, error) {
	userSubject, err := resolveUserSubject(ctx, req.GetUserSubject())
	if err != nil {
		return nil, err
	}
	rows, err := s.store.ListUserDevices(ctx, controlplane.ListUserDevicesInput{
		UserSubject: userSubject,
	})
	if err != nil {
		return nil, controlPlaneStoreError("list user devices", err)
	}
	devices := make([]*controlplanev1.UserDevice, 0, len(rows))
	for i := range rows {
		devices = append(devices, userDeviceToProto(rows[i]))
	}
	return &controlplanev1.ListUserDevicesResponse{Devices: devices}, nil
}

func (s *ControlPlaneService) ListDevices(ctx context.Context, req *controlplanev1.ListDevicesRequest) (*controlplanev1.ListDevicesResponse, error) {
	userSubject, err := resolveUserSubject(ctx, req.GetUserSubject())
	if err != nil {
		return nil, err
	}
	provider := controlplane.NormalizeProvider(req.GetProvider())
	if provider != "" && !s.supportsProvider(provider) {
		return nil, status.Error(codes.InvalidArgument, "unsupported provider")
	}
	rows, err := s.store.ListProviderDevices(ctx, controlplane.ListProviderDevicesInput{
		UserSubject: userSubject,
		Provider:    provider,
		ActiveOnly:  req.GetActiveOnly(),
	})
	if err != nil {
		return nil, controlPlaneStoreError("list provider devices", err)
	}
	groupByProvider := map[string]*controlplanev1.ProviderDeviceGroup{}
	order := make([]string, 0, 2)
	for i := range rows {
		row := rows[i]
		group, ok := groupByProvider[row.Provider]
		if !ok {
			group = &controlplanev1.ProviderDeviceGroup{
				Provider: row.Provider,
				Devices:  make([]*controlplanev1.ProviderDevice, 0, 4),
			}
			groupByProvider[row.Provider] = group
			order = append(order, row.Provider)
		}
		group.Devices = append(group.Devices, providerDeviceToProto(row))
	}
	groups := make([]*controlplanev1.ProviderDeviceGroup, 0, len(order))
	for _, key := range order {
		groups = append(groups, groupByProvider[key])
	}
	return &controlplanev1.ListDevicesResponse{Groups: groups}, nil
}

func (s *ControlPlaneService) SearchAdminLogFilters(ctx context.Context, req *controlplanev1.SearchAdminLogFiltersRequest) (*controlplanev1.SearchAdminLogFiltersResponse, error) {
	userSubject, err := resolveUserSubject(ctx, req.GetUserSubject())
	if err != nil {
		return nil, err
	}
	claims, _ := grpcmw.ClaimsFromContext(ctx)
	isAdmin := hasGlobalRole(claims.Roles, "admin")
	kind := strings.ToLower(strings.TrimSpace(req.GetKind()))
	if kind != "" && kind != "device" && kind != "serial" && kind != "user" {
		return nil, status.Error(codes.InvalidArgument, "kind must be device, serial, or user")
	}
	if !isAdmin && kind == "user" {
		return nil, status.Error(codes.PermissionDenied, "admin role required")
	}
	rows, err := s.store.SearchAdminLogFilters(ctx, controlplane.SearchAdminLogFiltersInput{
		Query:       req.GetQuery(),
		Kind:        kind,
		Limit:       int(req.GetLimit()),
		Provider:    req.GetProvider(),
		UserSubject: userSubject,
		GlobalAdmin: isAdmin,
	})
	if err != nil {
		return nil, controlPlaneStoreError("search admin log filters", err)
	}
	out := make([]*controlplanev1.AdminLogFilterOption, 0, len(rows))
	for i := range rows {
		out = append(out, adminLogFilterOptionToProto(rows[i]))
	}
	return &controlplanev1.SearchAdminLogFiltersResponse{Options: out}, nil
}

func (s *ControlPlaneService) DiscoverDevices(ctx context.Context, req *controlplanev1.DiscoverDevicesRequest) (*controlplanev1.DiscoverDevicesResponse, error) {
	userSubject, err := resolveUserSubject(ctx, req.GetUserSubject())
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.GetCredentialId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "credential_id required")
	}
	cred, err := s.store.GetProviderCredential(ctx, userSubject, req.GetCredentialId())
	if err != nil {
		if errors.Is(err, controlplane.ErrCredentialNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Errorf(codes.Internal, "get provider credential: %v", err)
	}
	provider := controlplane.NormalizeProvider(req.GetProvider())
	if provider == "" {
		provider = cred.Provider
	}
	if provider != cred.Provider {
		return nil, status.Error(codes.InvalidArgument, "provider does not match credential provider")
	}
	discoverer, ok := s.adapters.Discoverer(provider)
	if !ok {
		return &controlplanev1.DiscoverDevicesResponse{
			Accepted:        false,
			Status:          "discoverer_not_configured",
			DiscoveredCount: 0,
		}, nil
	}
	devices, err := discoverer.DiscoverDevices(ctx, cred)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "discover devices: %v", err)
	}
	out := make([]*controlplanev1.ProviderDevice, 0, len(devices))
	for i := range devices {
		discovered := devices[i]
		created, err := s.store.CreateDevice(ctx, controlplane.CreateDeviceInput{
			UserSubject: userSubject,
			EcoflowSN:   discovered.CanonicalSN,
			ProductName: discovered.ProductName,
			Model:       discovered.Model,
		})
		if err != nil {
			return nil, status.Errorf(codes.Internal, "create discovered device: %v", err)
		}
		persisted, err := s.store.UpsertProviderDevice(ctx, controlplane.UpsertProviderDeviceInput{
			DeviceID:           created.DeviceID,
			Provider:           provider,
			ProviderDeviceID:   discovered.ProviderDeviceID,
			CredentialID:       cred.ID,
			ProductName:        discovered.ProductName,
			Model:              discovered.Model,
			IsActive:           discovered.IsActive,
			IngestDesiredState: discovered.IngestDesiredState,
		})
		if err != nil {
			return nil, status.Errorf(codes.Internal, "persist discovered provider device: %v", err)
		}
		persisted.CanonicalSN = created.EcoflowSN
		if persisted.ProductName == "" {
			persisted.ProductName = created.ProductName
		}
		if persisted.Model == "" {
			persisted.Model = created.Model
		}
		out = append(out, providerDeviceToProto(persisted))
	}
	return &controlplanev1.DiscoverDevicesResponse{
		Accepted:        true,
		Status:          "ok",
		DiscoveredCount: uint32(len(out)),
		Devices:         out,
	}, nil
}

func (s *ControlPlaneService) ListAvailableProviderDevices(ctx context.Context, req *controlplanev1.ListAvailableProviderDevicesRequest) (*controlplanev1.ListAvailableProviderDevicesResponse, error) {
	userSubject, err := resolveUserSubject(ctx, req.GetUserSubject())
	if err != nil {
		return nil, err
	}
	provider := controlplane.NormalizeProvider(req.GetProvider())
	if provider != "" && !s.supportsProvider(provider) {
		return nil, status.Error(codes.InvalidArgument, "unsupported provider")
	}
	devices, hasActiveCredentials, err := s.listAvailableProviderDevices(ctx, userSubject, provider)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list available provider devices: %v", err)
	}
	out := make([]*controlplanev1.AvailableProviderDevice, 0, len(devices))
	for i := range devices {
		out = append(out, availableProviderDeviceToProto(devices[i]))
	}
	return &controlplanev1.ListAvailableProviderDevicesResponse{
		Devices:              out,
		HasActiveCredentials: hasActiveCredentials,
	}, nil
}

func (s *ControlPlaneService) TestProviderDeviceMQTT(ctx context.Context, req *controlplanev1.TestProviderDeviceMQTTRequest) (*controlplanev1.TestProviderDeviceMQTTResponse, error) {
	userSubject, err := resolveUserSubject(ctx, req.GetUserSubject())
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.GetCredentialId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "credential_id required")
	}
	if strings.TrimSpace(req.GetProviderDeviceId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "provider_device_id required")
	}
	cred, provider, err := s.getProviderCredentialForUser(ctx, userSubject, req.GetProvider(), req.GetCredentialId())
	if err != nil {
		return nil, err
	}
	discovered, err := s.discoverProviderDeviceForCredential(ctx, provider, cred, req.GetProviderDeviceId())
	if err != nil {
		return nil, err
	}
	if err := validateProviderDeviceImportable(provider, discovered); err != nil {
		return nil, err
	}
	probe, err := s.probeProviderDeviceMQTT(ctx, provider, cred, discovered.ProviderDeviceID)
	if err != nil {
		return nil, err
	}
	if !probe.GetSuccess() {
		return probe, nil
	}
	persisted, err := s.persistImportedProviderDevice(ctx, userSubject, provider, cred, discovered, true, "active")
	if err != nil {
		return nil, err
	}
	probe.DeviceId = persisted.UserDevice.DeviceID
	return probe, nil
}

func (s *ControlPlaneService) probeProviderDeviceMQTT(
	ctx context.Context,
	provider string,
	cred controlplane.ProviderCredential,
	providerDeviceID string,
) (*controlplanev1.TestProviderDeviceMQTTResponse, error) {
	if discoverer, ok := s.adapters.Discoverer(provider); ok {
		if prober, ok := discoverer.(providerMQTTProber); ok {
			result, err := prober.ProbeMQTT(ctx, cred, providerDeviceID, s.mqttProbeTimeout)
			if err != nil {
				switch {
				case errors.Is(err, provideradapter.ErrProviderDeviceNotFound):
					return nil, status.Error(codes.NotFound, err.Error())
				case errors.Is(err, provideradapter.ErrInactiveCredential), errors.Is(err, provideradapter.ErrMissingCredentialMaterial):
					return nil, status.Error(codes.FailedPrecondition, err.Error())
				default:
					return nil, status.Errorf(codes.Internal, "probe provider mqtt: %v", err)
				}
			}
			return mqttProbeResultToProto(result), nil
		}
	}
	resolver, tlsConfig, err := s.mqttResolver(provider)
	if err != nil {
		return nil, err
	}
	probeCtx, cancel := context.WithTimeout(ctx, s.mqttProbeTimeout)
	defer cancel()

	cert, err := resolver.GetMQTTCertification(probeCtx, cred, providerDeviceID)
	if err != nil {
		switch {
		case errors.Is(err, provideradapter.ErrProviderDeviceNotFound):
			return nil, status.Error(codes.NotFound, err.Error())
		case errors.Is(err, provideradapter.ErrInactiveCredential), errors.Is(err, provideradapter.ErrMissingCredentialMaterial):
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		default:
			return nil, status.Errorf(codes.Internal, "resolve mqtt certification: %v", err)
		}
	}
	address, topic, err := provideradapter.BuildMQTTAddressAndTopic(cert, providerDeviceID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "build mqtt probe topic: %v", err)
	}
	subscriber, err := s.newMQTTSubscriber(ecoflowmqtt.Config{
		Address:        address,
		Username:       strings.TrimSpace(cert.CertificateAccount),
		Password:       strings.TrimSpace(cert.CertificatePassword),
		ClientID:       buildMQTTProbeClientID(providerDeviceID),
		KeepAlive:      defaultMQTTProbeKeepAlive,
		ConnectTimeout: defaultMQTTProbeConnectTimeout,
		ReadTimeout:    defaultMQTTProbeReadTimeout,
		WriteTimeout:   defaultMQTTProbeWriteTimeout,
		TLSConfig:      tlsConfig,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "init mqtt probe subscriber: %v", err)
	}
	defer func() { _ = subscriber.Close() }()

	if err := subscriber.Connect(probeCtx); err != nil {
		return &controlplanev1.TestProviderDeviceMQTTResponse{
			Success: false,
			Status:  mqttProbeStatusFromError(err, "connect_failed"),
		}, nil
	}
	if err := subscriber.Subscribe(probeCtx, topic, 0); err != nil {
		return &controlplanev1.TestProviderDeviceMQTTResponse{
			Success: false,
			Status:  mqttProbeStatusFromError(err, "subscribe_failed"),
		}, nil
	}
	msg, err := subscriber.ReadMessage(probeCtx)
	if err != nil {
		return &controlplanev1.TestProviderDeviceMQTTResponse{
			Success: false,
			Status:  mqttProbeStatusFromError(err, "no_messages"),
		}, nil
	}
	return &controlplanev1.TestProviderDeviceMQTTResponse{
		Success:          true,
		Status:           "ok",
		SampleTopic:      redactMQTTSampleTopic(msg.Topic),
		PayloadBytes:     int64(len(msg.Payload)),
		ObservedAtUnixMs: time.Now().UTC().UnixMilli(),
	}, nil
}

func (s *ControlPlaneService) EnableProviderDevice(ctx context.Context, req *controlplanev1.EnableProviderDeviceRequest) (*controlplanev1.EnableProviderDeviceResponse, error) {
	userSubject, err := resolveUserSubject(ctx, req.GetUserSubject())
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.GetCredentialId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "credential_id required")
	}
	if strings.TrimSpace(req.GetProviderDeviceId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "provider_device_id required")
	}
	cred, provider, err := s.getProviderCredentialForUser(ctx, userSubject, req.GetProvider(), req.GetCredentialId())
	if err != nil {
		return nil, err
	}
	discovered, err := s.discoverProviderDeviceForCredential(ctx, provider, cred, req.GetProviderDeviceId())
	if err != nil {
		return nil, err
	}
	if err := validateProviderDeviceImportable(provider, discovered); err != nil {
		return nil, err
	}
	probe, err := s.probeProviderDeviceMQTT(ctx, provider, cred, req.GetProviderDeviceId())
	if err != nil {
		return nil, err
	}
	if !probe.GetSuccess() {
		return nil, status.Errorf(codes.FailedPrecondition, "successful mqtt probe required before enablement: %s", probe.GetStatus())
	}
	persisted, err := s.persistImportedProviderDevice(ctx, userSubject, provider, cred, discovered, true, "active")
	if err != nil {
		return nil, err
	}
	return &controlplanev1.EnableProviderDeviceResponse{
		ProviderDevice: providerDeviceToProto(persisted.ProviderDevice),
		UserDevice:     userDeviceToProto(persisted.UserDevice),
	}, nil
}

func (s *ControlPlaneService) ImportProviderDevice(ctx context.Context, req *controlplanev1.ImportProviderDeviceRequest) (*controlplanev1.ImportProviderDeviceResponse, error) {
	userSubject, err := resolveUserSubject(ctx, req.GetUserSubject())
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.GetCredentialId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "credential_id required")
	}
	if strings.TrimSpace(req.GetProviderDeviceId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "provider_device_id required")
	}
	cred, provider, err := s.getProviderCredentialForUser(ctx, userSubject, req.GetProvider(), req.GetCredentialId())
	if err != nil {
		return nil, err
	}
	ingestDesiredState, err := normalizeImportedIngestState(req.GetIngestDesiredState(), req.GetIsActive())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	discovered, err := s.discoverProviderDeviceForCredential(ctx, provider, cred, req.GetProviderDeviceId())
	if err != nil {
		return nil, err
	}
	if err := validateProviderDeviceImportable(provider, discovered); err != nil {
		return nil, err
	}
	if req.GetIsActive() {
		probe, err := s.probeProviderDeviceMQTT(ctx, provider, cred, req.GetProviderDeviceId())
		if err != nil {
			return nil, err
		}
		if !probe.GetSuccess() {
			return nil, status.Errorf(codes.FailedPrecondition, "successful mqtt probe required before activation: %s", probe.GetStatus())
		}
	}
	persisted, err := s.persistImportedProviderDevice(ctx, userSubject, provider, cred, discovered, req.GetIsActive(), ingestDesiredState)
	if err != nil {
		return nil, err
	}
	return &controlplanev1.ImportProviderDeviceResponse{
		ProviderDevice: providerDeviceToProto(persisted.ProviderDevice),
		UserDevice:     userDeviceToProto(persisted.UserDevice),
	}, nil
}

func normalizeImportedIngestState(raw string, isActive bool) (string, error) {
	state := strings.ToLower(strings.TrimSpace(raw))
	if state == "" {
		if isActive {
			return "active", nil
		}
		return "paused", nil
	}
	switch state {
	case "active", "draining", "paused":
	default:
		return "", fmt.Errorf("ingest_desired_state must be active, draining, or paused")
	}
	if isActive && state != "active" {
		return "", fmt.Errorf("active provider devices must use ingest_desired_state=active")
	}
	if !isActive && state == "active" {
		return "", fmt.Errorf("inactive provider devices cannot use ingest_desired_state=active")
	}
	return state, nil
}

type persistedImportedProviderDevice struct {
	ProviderDevice controlplane.ProviderDevice
	UserDevice     controlplane.UserDevice
}

func (s *ControlPlaneService) persistImportedProviderDevice(
	ctx context.Context,
	userSubject string,
	provider string,
	cred controlplane.ProviderCredential,
	discovered controlplane.ProviderDevice,
	isActive bool,
	ingestDesiredState string,
) (persistedImportedProviderDevice, error) {
	persisted, err := s.store.ImportProviderDevice(ctx, controlplane.ImportProviderDeviceInput{
		UserSubject:        userSubject,
		Provider:           provider,
		ProviderDeviceID:   discovered.ProviderDeviceID,
		CredentialID:       cred.ID,
		CanonicalSN:        discovered.CanonicalSN,
		ProductName:        discovered.ProductName,
		Model:              discovered.Model,
		Capabilities:       discovered.Capabilities,
		Metadata:           discovered.Metadata,
		IsActive:           isActive,
		IngestDesiredState: ingestDesiredState,
	})
	if err != nil {
		if errors.Is(err, controlplane.ErrUserNotFound) {
			return persistedImportedProviderDevice{}, status.Error(codes.NotFound, err.Error())
		}
		return persistedImportedProviderDevice{}, status.Errorf(codes.Internal, "persist imported provider device: %v", err)
	}
	return persistedImportedProviderDevice{
		ProviderDevice: persisted.ProviderDevice,
		UserDevice:     persisted.UserDevice,
	}, nil
}

func (s *ControlPlaneService) listAvailableProviderDevices(ctx context.Context, userSubject, provider string) ([]controlplane.ProviderDevice, bool, error) {
	credentials, err := s.store.ListProviderCredentials(ctx, controlplane.ListProviderCredentialsInput{
		UserSubject: userSubject,
		Provider:    provider,
	})
	if err != nil {
		return nil, false, err
	}
	activeCreds := make([]controlplane.ProviderCredential, 0, len(credentials))
	for i := range credentials {
		if credentials[i].IsActive {
			activeCreds = append(activeCreds, credentials[i])
		}
	}
	if len(activeCreds) == 0 {
		return nil, false, nil
	}
	existing, err := s.store.ListProviderDevices(ctx, controlplane.ListProviderDevicesInput{
		UserSubject: userSubject,
		Provider:    provider,
		ActiveOnly:  true,
	})
	if err != nil {
		return nil, true, err
	}
	existingKeys := make(map[string]struct{}, len(existing))
	for i := range existing {
		existingKeys[availableProviderDeviceKey(existing[i].Provider, existing[i].ProviderDeviceID)] = struct{}{}
	}
	seen := make(map[string]struct{})
	out := make([]controlplane.ProviderDevice, 0, len(activeCreds)*2)
	for i := range activeCreds {
		cred, err := s.store.GetProviderCredential(ctx, userSubject, activeCreds[i].ID)
		if err != nil {
			if errors.Is(err, controlplane.ErrCredentialNotFound) {
				continue
			}
			return nil, true, err
		}
		discoverer, ok := s.adapters.Discoverer(cred.Provider)
		if !ok {
			continue
		}
		discovered, err := discoverer.DiscoverDevices(ctx, cred)
		if err != nil {
			switch {
			case errors.Is(err, provideradapter.ErrInactiveCredential),
				errors.Is(err, provideradapter.ErrMissingCredentialMaterial),
				errors.Is(err, provideradapter.ErrUnsupportedProvider):
				continue
			default:
				return nil, true, err
			}
		}
		for j := range discovered {
			device := discovered[j]
			device.Provider = cred.Provider
			if device.CredentialID == "" {
				device.CredentialID = cred.ID
			}
			key := availableProviderDeviceKey(device.Provider, device.ProviderDeviceID)
			if _, ok := existingKeys[key]; ok {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, device)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Provider == out[j].Provider {
			return out[i].ProviderDeviceID < out[j].ProviderDeviceID
		}
		return out[i].Provider < out[j].Provider
	})
	return out, true, nil
}

func (s *ControlPlaneService) getProviderCredentialForUser(ctx context.Context, userSubject, requestedProvider, credentialID string) (controlplane.ProviderCredential, string, error) {
	cred, err := s.store.GetProviderCredential(ctx, userSubject, credentialID)
	if err != nil {
		if errors.Is(err, controlplane.ErrCredentialNotFound) {
			return controlplane.ProviderCredential{}, "", status.Error(codes.NotFound, err.Error())
		}
		return controlplane.ProviderCredential{}, "", status.Errorf(codes.Internal, "get provider credential: %v", err)
	}
	provider := controlplane.NormalizeProvider(requestedProvider)
	if provider == "" {
		provider = cred.Provider
	}
	if provider != cred.Provider {
		return controlplane.ProviderCredential{}, "", status.Error(codes.InvalidArgument, "provider does not match credential provider")
	}
	return cred, provider, nil
}

func (s *ControlPlaneService) discoverProviderDeviceForCredential(ctx context.Context, provider string, cred controlplane.ProviderCredential, providerDeviceID string) (controlplane.ProviderDevice, error) {
	discoverer, ok := s.adapters.Discoverer(provider)
	if !ok {
		return controlplane.ProviderDevice{}, status.Error(codes.Unimplemented, "provider discoverer not configured")
	}
	devices, err := discoverer.DiscoverDevices(ctx, cred)
	if err != nil {
		return controlplane.ProviderDevice{}, status.Errorf(codes.Internal, "discover provider devices: %v", err)
	}
	target := strings.ToUpper(strings.TrimSpace(providerDeviceID))
	for i := range devices {
		device := devices[i]
		if strings.EqualFold(strings.TrimSpace(device.ProviderDeviceID), target) {
			device.Provider = provider
			device.CredentialID = cred.ID
			return device, nil
		}
	}
	return controlplane.ProviderDevice{}, status.Error(codes.NotFound, "provider device not found")
}

func (s *ControlPlaneService) mqttResolver(provider string) (mqttCertificationResolver, *tls.Config, error) {
	discoverer, ok := s.adapters.Discoverer(provider)
	if !ok {
		return nil, nil, status.Error(codes.Unimplemented, "provider mqtt probe not configured")
	}
	resolver, ok := discoverer.(mqttCertificationResolver)
	if !ok {
		return nil, nil, status.Error(codes.Unimplemented, "provider mqtt probe not configured")
	}
	var tlsConfig *tls.Config
	if provider, ok := discoverer.(mqttResolverTLSConfigProvider); ok {
		tlsConfig = provider.MQTTTLSConfig()
	}
	return resolver, tlsConfig, nil
}

func availableProviderDeviceKey(provider, providerDeviceID string) string {
	return controlplane.NormalizeProvider(provider) + "|" + strings.ToUpper(strings.TrimSpace(providerDeviceID))
}

func mqttProbeStatusFromError(err error, fallback string) string {
	if err == nil {
		return "ok"
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return "timeout"
	}
	lower := strings.ToLower(err.Error())
	switch {
	case strings.Contains(lower, "connect rejected"), strings.Contains(lower, "not authorized"), strings.Contains(lower, "return code=5"):
		return "connect_rejected"
	case strings.Contains(lower, "subscription rejected"):
		return "subscribe_rejected"
	case strings.Contains(lower, "eof"), strings.Contains(lower, "read"):
		return "no_messages"
	default:
		return fallback
	}
}

func buildMQTTProbeClientID(providerDeviceID string) string {
	seed := strings.ToUpper(strings.TrimSpace(providerDeviceID))
	if seed == "" {
		seed = "mqtt-probe"
	}
	return ecoflowmqtt.BuildClientIDFromSN(fmt.Sprintf("probe:%s:%d", seed, time.Now().UTC().UnixNano()))
}

func resolveUserSubject(ctx context.Context, requested string) (string, error) {
	requested = strings.TrimSpace(requested)
	if claims, ok := grpcmw.ClaimsFromContext(ctx); ok {
		subject := strings.TrimSpace(claims.Subject)
		if subject != "" {
			if requested != "" && requested != subject {
				return "", status.Error(codes.PermissionDenied, "user_subject does not match token subject")
			}
			return subject, nil
		}
	}
	if requested == "" {
		return "", status.Error(codes.InvalidArgument, "user_subject required")
	}
	return requested, nil
}

func providerCredentialToProto(in controlplane.ProviderCredential) *controlplanev1.ProviderCredential {
	return &controlplanev1.ProviderCredential{
		Id:              in.ID,
		Provider:        in.Provider,
		AccessKeyMask:   in.AccessKeyMask,
		IsActive:        in.IsActive,
		CreatedAtUnixMs: in.CreatedAt.UnixMilli(),
		UpdatedAtUnixMs: in.UpdatedAt.UnixMilli(),
		Config:          mapToStructProto(in.Config),
	}
}

func providerDeviceToProto(in controlplane.ProviderDevice) *controlplanev1.ProviderDevice {
	return &controlplanev1.ProviderDevice{
		Id:                 in.ID,
		DeviceId:           in.DeviceID,
		Provider:           in.Provider,
		ProviderDeviceId:   in.ProviderDeviceID,
		CredentialId:       in.CredentialID,
		CanonicalSn:        in.CanonicalSN,
		ProductName:        in.ProductName,
		Model:              in.Model,
		IsActive:           in.IsActive,
		IngestDesiredState: in.IngestDesiredState,
		Capabilities:       mapToStructProto(in.Capabilities),
		Metadata:           mapToStructProto(in.Metadata),
	}
}

func availableProviderDeviceToProto(in controlplane.ProviderDevice) *controlplanev1.AvailableProviderDevice {
	return &controlplanev1.AvailableProviderDevice{
		Provider:         in.Provider,
		ProviderDeviceId: in.ProviderDeviceID,
		CredentialId:     in.CredentialID,
		CanonicalSn:      in.CanonicalSN,
		ProductName:      in.ProductName,
		Model:            in.Model,
		Capabilities:     mapToStructProto(in.Capabilities),
		Metadata:         mapToStructProto(in.Metadata),
	}
}

func userDeviceToProto(in controlplane.UserDevice) *controlplanev1.UserDevice {
	return &controlplanev1.UserDevice{
		DeviceId:        in.DeviceID,
		EcoflowSn:       in.EcoflowSN,
		ProductName:     in.ProductName,
		Model:           in.Model,
		Role:            in.Role,
		CreatedAtUnixMs: in.CreatedAt.UnixMilli(),
		UpdatedAtUnixMs: in.UpdatedAt.UnixMilli(),
	}
}

func adminLogFilterOptionToProto(in controlplane.AdminLogFilterOption) *controlplanev1.AdminLogFilterOption {
	return &controlplanev1.AdminLogFilterOption{
		Kind:           in.Kind,
		Id:             in.ID,
		Label:          in.Label,
		SecondaryLabel: in.SecondaryLabel,
		DeviceIds:      append([]string(nil), in.DeviceIDs...),
		Provider:       in.Provider,
	}
}

func currentUserToProto(in controlplane.CurrentUser, authMethod string) *controlplanev1.CurrentUser {
	out := &controlplanev1.CurrentUser{
		Id:                     in.ID,
		KeycloakSubject:        in.KeycloakSubject,
		Email:                  in.Email,
		EmailVerified:          in.EmailVerified,
		DisplayName:            in.DisplayName,
		DisplayNameSource:      in.DisplayNameSource,
		AvatarUrl:              in.AvatarURL,
		GivenName:              in.GivenName,
		FamilyName:             in.FamilyName,
		Locale:                 in.Locale,
		Timezone:               in.Timezone,
		WeatherLocationEnabled: in.WeatherLocationEnabled,
		WeatherLocationSource:  in.WeatherLocationSource,
		WeatherLocationLabel:   in.WeatherLocationLabel,
		HasWeatherLocation:     in.HasWeatherLocation,
		CreatedAtUnixMs:        in.CreatedAt.UnixMilli(),
		UpdatedAtUnixMs:        in.UpdatedAt.UnixMilli(),
		AuthMethod:             strings.TrimSpace(authMethod),
	}
	if in.HasWeatherLocation {
		out.WeatherLatitude = in.WeatherLatitude
		out.WeatherLongitude = in.WeatherLongitude
	}
	if !in.LastLoginAt.IsZero() {
		out.LastLoginAtUnixMs = in.LastLoginAt.UnixMilli()
	}
	return out
}

func normalizeDeviceRole(in string) (string, error) {
	role := strings.ToLower(strings.TrimSpace(in))
	if role == "" {
		role = "viewer"
	}
	switch role {
	case "viewer", "admin":
		return role, nil
	default:
		return "", status.Error(codes.InvalidArgument, "role must be viewer or admin")
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func hasGlobalRole(roles []string, want string) bool {
	want = strings.ToLower(strings.TrimSpace(want))
	for _, role := range roles {
		if strings.ToLower(strings.TrimSpace(role)) == want {
			return true
		}
	}
	return false
}

func mapToStructProto(in map[string]any) *structpb.Struct {
	if len(in) == 0 {
		return nil
	}
	out, err := structpb.NewStruct(in)
	if err != nil {
		return nil
	}
	return out
}

func structToMap(in *structpb.Struct) map[string]any {
	if in == nil || len(in.GetFields()) == 0 {
		return nil
	}
	out := in.AsMap()
	if len(out) == 0 {
		return nil
	}
	return out
}

func cloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func mergeProviderCredentialConfig(base map[string]any, patch map[string]any) map[string]any {
	if len(base) == 0 && len(patch) == 0 {
		return nil
	}
	out := cloneMap(base)
	if out == nil {
		out = make(map[string]any, len(patch))
	}
	for key, value := range patch {
		out[key] = value
	}
	return out
}

func normalizeProviderCredentialConfig(provider string, config map[string]any) (map[string]any, error) {
	if controlplane.NormalizeProvider(provider) != controlplane.ProviderAnkerSolix {
		return cloneMap(config), nil
	}
	server, err := configString(config, "server")
	if err != nil {
		return nil, err
	}
	country, err := configString(config, "country")
	if err != nil {
		return nil, err
	}
	cfg, err := ankersolix.ResolveConfig(server, country)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"server":  string(cfg.Server),
		"country": cfg.Country,
	}, nil
}

func configString(config map[string]any, key string) (string, error) {
	if len(config) == 0 {
		return "", nil
	}
	value, ok := config[key]
	if !ok {
		return "", nil
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("anker solix config %s must be a string", key)
	}
	return strings.TrimSpace(text), nil
}

func validateProviderDeviceImportable(provider string, device controlplane.ProviderDevice) error {
	if controlplane.NormalizeProvider(provider) != controlplane.ProviderAnkerSolix {
		return nil
	}
	if ankerSolixProviderDeviceEnableable(device) {
		return nil
	}
	return status.Errorf(
		codes.FailedPrecondition,
		"anker solix device is not enableable for mqtt ingest: %s",
		ankerSolixSupportStatus(device),
	)
}

func ankerSolixProviderDeviceEnableable(device controlplane.ProviderDevice) bool {
	if device.Capabilities["mqtt_supported"] == true {
		return true
	}
	return strings.EqualFold(ankerSolixSupportStatus(device), string(ankersolix.SupportEnabled))
}

func ankerSolixSupportStatus(device controlplane.ProviderDevice) string {
	for _, record := range []map[string]any{device.Metadata, device.Capabilities} {
		if len(record) == 0 {
			continue
		}
		for _, key := range []string{"support_status", "supportStatus", "mqtt_support_status", "mqttSupportStatus"} {
			if value, ok := record[key]; ok {
				if text := strings.TrimSpace(fmt.Sprint(value)); text != "" {
					return text
				}
			}
		}
	}
	return "unknown"
}

func redactMQTTSampleTopic(topic string) string {
	if strings.TrimSpace(topic) == "" {
		return ""
	}
	return "redacted"
}

func mqttProbeResultToProto(in provideradapter.MQTTProbeResult) *controlplanev1.TestProviderDeviceMQTTResponse {
	return &controlplanev1.TestProviderDeviceMQTTResponse{
		Success:          in.Success,
		Status:           in.Status,
		SampleTopic:      redactMQTTSampleTopic(in.SampleTopic),
		PayloadBytes:     in.PayloadBytes,
		ObservedAtUnixMs: in.ObservedAtUnixMS,
	}
}

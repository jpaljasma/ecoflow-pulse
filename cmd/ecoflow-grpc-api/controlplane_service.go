package main

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	controlplanev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/controlplane/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/grpcmw"
	"github.com/jpaljasma/ecoflow-pulse/internal/provideradapter"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

type ControlPlaneService struct {
	controlplanev1.UnimplementedControlPlaneServiceServer

	log      *slog.Logger
	store    controlplane.Store
	adapters *provideradapter.Registry
}

func NewControlPlaneService(log *slog.Logger, store controlplane.Store, adapters *provideradapter.Registry) *ControlPlaneService {
	if adapters == nil {
		adapters = provideradapter.NewRegistry()
	}
	return &ControlPlaneService{
		log:      log,
		store:    store,
		adapters: adapters,
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
		return nil, status.Errorf(codes.Internal, "get current user: %v", err)
	}
	devices, err := s.store.ListUserDevices(ctx, controlplane.ListUserDevicesInput{UserSubject: userSubject})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list current user devices: %v", err)
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
	if strings.TrimSpace(req.GetAccessKey()) == "" {
		return nil, status.Error(codes.InvalidArgument, "access_key required")
	}
	if strings.TrimSpace(req.GetSecretKey()) == "" {
		return nil, status.Error(codes.InvalidArgument, "secret_key required")
	}
	out, err := s.store.CreateProviderCredential(ctx, controlplane.CreateProviderCredentialInput{
		UserSubject: userSubject,
		Provider:    provider,
		AccessKey:   req.GetAccessKey(),
		SecretKey:   req.GetSecretKey(),
		IsActive:    req.GetIsActive(),
	})
	if err != nil {
		if errors.Is(err, controlplane.ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
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
		return nil, status.Errorf(codes.Internal, "list user devices: %v", err)
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
		return nil, status.Errorf(codes.Internal, "list provider devices: %v", err)
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

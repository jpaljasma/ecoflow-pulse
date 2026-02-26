package main

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	controlplanev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/controlplane/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/grpcmw"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ProviderDiscoverer interface {
	DiscoverDevices(ctx context.Context, credential controlplane.ProviderCredential) ([]controlplane.ProviderDevice, error)
}

type ControlPlaneService struct {
	controlplanev1.UnimplementedControlPlaneServiceServer

	log         *slog.Logger
	store       controlplane.Store
	discoverers map[string]ProviderDiscoverer
}

func NewControlPlaneService(log *slog.Logger, store controlplane.Store) *ControlPlaneService {
	return &ControlPlaneService{
		log:         log,
		store:       store,
		discoverers: map[string]ProviderDiscoverer{},
	}
}

func (s *ControlPlaneService) RegisterDiscoverer(provider string, discoverer ProviderDiscoverer) {
	if s == nil || discoverer == nil {
		return
	}
	if s.discoverers == nil {
		s.discoverers = map[string]ProviderDiscoverer{}
	}
	s.discoverers[controlplane.NormalizeProvider(provider)] = discoverer
}

func (s *ControlPlaneService) CreateProviderCredential(ctx context.Context, req *controlplanev1.CreateProviderCredentialRequest) (*controlplanev1.CreateProviderCredentialResponse, error) {
	userSubject, err := resolveUserSubject(ctx, req.GetUserSubject())
	if err != nil {
		return nil, err
	}
	provider := controlplane.NormalizeProvider(req.GetProvider())
	if !controlplane.IsSupportedProvider(provider) {
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
	if provider != "" && !controlplane.IsSupportedProvider(provider) {
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

func (s *ControlPlaneService) ListDevices(ctx context.Context, req *controlplanev1.ListDevicesRequest) (*controlplanev1.ListDevicesResponse, error) {
	userSubject, err := resolveUserSubject(ctx, req.GetUserSubject())
	if err != nil {
		return nil, err
	}
	provider := controlplane.NormalizeProvider(req.GetProvider())
	if provider != "" && !controlplane.IsSupportedProvider(provider) {
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
	discoverer, ok := s.discoverers[provider]
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
		out = append(out, providerDeviceToProto(devices[i]))
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
	}
}

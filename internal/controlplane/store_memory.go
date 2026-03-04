package controlplane

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type memoryDevice struct {
	ID          string
	EcoflowSN   string
	ProductName string
	Model       string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type MemoryStore struct {
	mu sync.RWMutex

	usersBySubject  map[string]string
	credentials     map[string]ProviderCredential
	providerDevices map[string]ProviderDevice

	devicesByID map[string]memoryDevice
	deviceBySN  map[string]string
	userDevices map[string]map[string]string // userID -> deviceID -> role

	idCounter uint64
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		usersBySubject: map[string]string{
			"dev-user": "dev-user-id-1",
		},
		credentials:     map[string]ProviderCredential{},
		providerDevices: map[string]ProviderDevice{},
		devicesByID:     map[string]memoryDevice{},
		deviceBySN:      map[string]string{},
		userDevices:     map[string]map[string]string{},
	}
}

func (s *MemoryStore) EnsureUser(userSubject string) string {
	subject := strings.TrimSpace(userSubject)
	if subject == "" {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if id, ok := s.usersBySubject[subject]; ok {
		return id
	}
	id := s.nextID("usr")
	s.usersBySubject[subject] = id
	return id
}

func (s *MemoryStore) PutProviderDevice(device ProviderDevice) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(device.ID) == "" {
		device.ID = s.nextID("pdev")
	}
	if strings.TrimSpace(device.Provider) == "" {
		device.Provider = ProviderEcoFlow
	}
	device.Provider = NormalizeProvider(device.Provider)
	device.ProviderDeviceID = strings.ToUpper(strings.TrimSpace(device.ProviderDeviceID))
	device.Capabilities = cloneAnyMap(device.Capabilities)
	device.Metadata = cloneAnyMap(device.Metadata)
	s.providerDevices[device.ID] = device
}

func (s *MemoryStore) CreateProviderCredential(_ context.Context, in CreateProviderCredentialInput) (ProviderCredential, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	userID, ok := s.usersBySubject[strings.TrimSpace(in.UserSubject)]
	if !ok {
		return ProviderCredential{}, ErrUserNotFound
	}
	now := utcNow()
	row := ProviderCredential{
		ID:            s.nextID("cred"),
		UserID:        userID,
		Provider:      NormalizeProvider(in.Provider),
		AccessKeyMask: MaskAccessKey(in.AccessKey),
		AccessKey:     in.AccessKey,
		SecretKey:     in.SecretKey,
		IsActive:      in.IsActive,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	s.credentials[row.ID] = row
	return row, nil
}

func (s *MemoryStore) ListProviderCredentials(_ context.Context, in ListProviderCredentialsInput) ([]ProviderCredential, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	userID, ok := s.usersBySubject[strings.TrimSpace(in.UserSubject)]
	if !ok {
		return []ProviderCredential{}, nil
	}
	provider := NormalizeProvider(in.Provider)
	out := make([]ProviderCredential, 0, len(s.credentials))
	for _, row := range s.credentials {
		if row.UserID != userID {
			continue
		}
		if provider != "" && row.Provider != provider {
			continue
		}
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID > out[j].ID
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

func (s *MemoryStore) SetProviderCredentialActive(_ context.Context, in SetProviderCredentialActiveInput) (ProviderCredential, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	userID, ok := s.usersBySubject[strings.TrimSpace(in.UserSubject)]
	if !ok {
		return ProviderCredential{}, ErrCredentialNotFound
	}
	row, ok := s.credentials[in.CredentialID]
	if !ok || row.UserID != userID {
		return ProviderCredential{}, ErrCredentialNotFound
	}
	row.IsActive = in.IsActive
	row.UpdatedAt = utcNow()
	s.credentials[row.ID] = row
	return row, nil
}

func (s *MemoryStore) GetProviderCredential(_ context.Context, userSubject string, credentialID string) (ProviderCredential, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	userID, ok := s.usersBySubject[strings.TrimSpace(userSubject)]
	if !ok {
		return ProviderCredential{}, ErrCredentialNotFound
	}
	row, ok := s.credentials[credentialID]
	if !ok || row.UserID != userID {
		return ProviderCredential{}, ErrCredentialNotFound
	}
	return row, nil
}

func (s *MemoryStore) CreateDevice(_ context.Context, in CreateDeviceInput) (UserDevice, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.TrimSpace(in.EcoflowSN) == "" {
		return UserDevice{}, ErrDeviceNotFound
	}
	userID, ok := s.usersBySubject[strings.TrimSpace(in.UserSubject)]
	if !ok {
		return UserDevice{}, ErrUserNotFound
	}
	now := utcNow()
	dev, created := s.ensureDeviceLocked(in.EcoflowSN, in.ProductName, in.Model, now)
	if !created {
		dev.UpdatedAt = now
		s.devicesByID[dev.ID] = dev
	}
	s.ensureUserDeviceRoleLocked(userID, dev.ID, "admin")
	return UserDevice{
		DeviceID:    dev.ID,
		EcoflowSN:   dev.EcoflowSN,
		ProductName: dev.ProductName,
		Model:       dev.Model,
		Role:        "admin",
		CreatedAt:   dev.CreatedAt,
		UpdatedAt:   dev.UpdatedAt,
	}, nil
}

func (s *MemoryStore) LinkDevice(_ context.Context, in LinkDeviceInput) (UserDevice, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	requesterID, ok := s.usersBySubject[strings.TrimSpace(in.UserSubject)]
	if !ok {
		return UserDevice{}, ErrUserNotFound
	}
	targetSubject := strings.TrimSpace(in.TargetUserSubject)
	if targetSubject == "" {
		targetSubject = strings.TrimSpace(in.UserSubject)
	}
	targetUserID, ok := s.usersBySubject[targetSubject]
	if !ok {
		return UserDevice{}, ErrUserNotFound
	}

	deviceID := strings.TrimSpace(in.DeviceID)
	dev, ok := s.devicesByID[deviceID]
	if !ok {
		return UserDevice{}, ErrDeviceNotFound
	}

	requesterRoles, ok := s.userDevices[requesterID]
	if !ok || requesterRoles[deviceID] != "admin" {
		return UserDevice{}, ErrPermissionDenied
	}
	s.ensureUserDeviceRoleLocked(targetUserID, deviceID, in.Role)
	return UserDevice{
		DeviceID:    dev.ID,
		EcoflowSN:   dev.EcoflowSN,
		ProductName: dev.ProductName,
		Model:       dev.Model,
		Role:        in.Role,
		CreatedAt:   dev.CreatedAt,
		UpdatedAt:   dev.UpdatedAt,
	}, nil
}

func (s *MemoryStore) ListUserDevices(_ context.Context, in ListUserDevicesInput) ([]UserDevice, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	userID, ok := s.usersBySubject[strings.TrimSpace(in.UserSubject)]
	if !ok {
		return []UserDevice{}, nil
	}
	roles := s.userDevices[userID]
	out := make([]UserDevice, 0, len(roles))
	for deviceID, role := range roles {
		dev, ok := s.devicesByID[deviceID]
		if !ok {
			continue
		}
		out = append(out, UserDevice{
			DeviceID:    dev.ID,
			EcoflowSN:   dev.EcoflowSN,
			ProductName: dev.ProductName,
			Model:       dev.Model,
			Role:        role,
			CreatedAt:   dev.CreatedAt,
			UpdatedAt:   dev.UpdatedAt,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ProductName == out[j].ProductName {
			return out[i].EcoflowSN < out[j].EcoflowSN
		}
		return out[i].ProductName < out[j].ProductName
	})
	return out, nil
}

func (s *MemoryStore) UpsertProviderDevice(_ context.Context, in UpsertProviderDeviceInput) (ProviderDevice, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	provider := NormalizeProvider(in.Provider)
	providerDeviceID := strings.ToUpper(strings.TrimSpace(in.ProviderDeviceID))
	var existingID string
	for id, row := range s.providerDevices {
		if row.Provider == provider && row.ProviderDeviceID == providerDeviceID {
			existingID = id
			break
		}
	}
	if existingID == "" {
		existingID = s.nextID("pdev")
	}
	existing := s.providerDevices[existingID]
	capabilities := cloneAnyMap(existing.Capabilities)
	if in.Capabilities != nil {
		capabilities = cloneAnyMap(in.Capabilities)
	}
	metadata := cloneAnyMap(existing.Metadata)
	if in.Metadata != nil {
		metadata = cloneAnyMap(in.Metadata)
	}
	row := ProviderDevice{
		ID:                 existingID,
		DeviceID:           strings.TrimSpace(in.DeviceID),
		Provider:           provider,
		ProviderDeviceID:   providerDeviceID,
		CredentialID:       strings.TrimSpace(in.CredentialID),
		CanonicalSN:        providerDeviceID,
		ProductName:        strings.TrimSpace(in.ProductName),
		Model:              strings.TrimSpace(in.Model),
		Capabilities:       capabilities,
		Metadata:           metadata,
		IsActive:           in.IsActive,
		IngestDesiredState: strings.ToLower(strings.TrimSpace(in.IngestDesiredState)),
	}
	s.providerDevices[row.ID] = row
	return cloneProviderDevice(row), nil
}

func (s *MemoryStore) ListProviderDevices(_ context.Context, in ListProviderDevicesInput) ([]ProviderDevice, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	provider := NormalizeProvider(in.Provider)
	out := make([]ProviderDevice, 0, len(s.providerDevices))
	for _, row := range s.providerDevices {
		if provider != "" && row.Provider != provider {
			continue
		}
		if in.ActiveOnly && !row.IsActive {
			continue
		}
		out = append(out, cloneProviderDevice(row))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Provider == out[j].Provider {
			return out[i].CanonicalSN < out[j].CanonicalSN
		}
		return out[i].Provider < out[j].Provider
	})
	return out, nil
}

func (s *MemoryStore) ListIngestAssignments(_ context.Context, in ListIngestAssignmentsInput) ([]IngestAssignment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	provider := NormalizeProvider(in.Provider)
	credsByID := make(map[string]ProviderCredential, len(s.credentials))
	for _, cred := range s.credentials {
		credsByID[cred.ID] = cred
	}

	out := make([]IngestAssignment, 0, len(s.providerDevices))
	for _, dev := range s.providerDevices {
		if provider != "" && dev.Provider != provider {
			continue
		}
		if in.ActiveOnly {
			if !dev.IsActive {
				continue
			}
			if strings.TrimSpace(strings.ToLower(dev.IngestDesiredState)) != "active" {
				continue
			}
		}
		cred, ok := credsByID[dev.CredentialID]
		if !ok {
			continue
		}
		out = append(out, IngestAssignment{
			Provider:           dev.Provider,
			ProviderDeviceID:   dev.ProviderDeviceID,
			DeviceID:           dev.DeviceID,
			CredentialID:       dev.CredentialID,
			ProductName:        dev.ProductName,
			Model:              dev.Model,
			AccessKey:          cred.AccessKey,
			SecretKey:          cred.SecretKey,
			DeviceIsActive:     dev.IsActive,
			CredentialIsActive: cred.IsActive,
			IngestDesiredState: dev.IngestDesiredState,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Provider == out[j].Provider {
			return out[i].ProviderDeviceID < out[j].ProviderDeviceID
		}
		return out[i].Provider < out[j].Provider
	})
	return out, nil
}

func (s *MemoryStore) ensureDeviceLocked(sn string, productName string, model string, now time.Time) (memoryDevice, bool) {
	canonicalSN := strings.ToUpper(strings.TrimSpace(sn))
	if canonicalSN == "" {
		return memoryDevice{}, false
	}
	if existingID, ok := s.deviceBySN[canonicalSN]; ok {
		dev := s.devicesByID[existingID]
		if strings.TrimSpace(dev.ProductName) == "" {
			dev.ProductName = strings.TrimSpace(productName)
		}
		if strings.TrimSpace(dev.Model) == "" {
			dev.Model = strings.TrimSpace(model)
		}
		dev.UpdatedAt = now
		s.devicesByID[dev.ID] = dev
		return dev, false
	}
	dev := memoryDevice{
		ID:          s.nextID("dev"),
		EcoflowSN:   canonicalSN,
		ProductName: strings.TrimSpace(productName),
		Model:       strings.TrimSpace(model),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	s.deviceBySN[canonicalSN] = dev.ID
	s.devicesByID[dev.ID] = dev
	return dev, true
}

func (s *MemoryStore) ensureUserDeviceRoleLocked(userID string, deviceID string, role string) {
	if _, ok := s.userDevices[userID]; !ok {
		s.userDevices[userID] = map[string]string{}
	}
	s.userDevices[userID][deviceID] = strings.TrimSpace(role)
}

func (s *MemoryStore) nextID(prefix string) string {
	seq := atomic.AddUint64(&s.idCounter, 1)
	return fmt.Sprintf("%s-%d", prefix, seq)
}

func cloneProviderDevice(in ProviderDevice) ProviderDevice {
	out := in
	out.Capabilities = cloneAnyMap(in.Capabilities)
	out.Metadata = cloneAnyMap(in.Metadata)
	return out
}

func cloneAnyMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

package controlplane

import (
	"bytes"
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

type memoryUser struct {
	CurrentUser
}

type MemoryStore struct {
	mu sync.RWMutex

	usersBySubject  map[string]string
	usersByID       map[string]memoryUser
	credentials     map[string]ProviderCredential
	providerDevices map[string]ProviderDevice
	edgeCollectors  map[string]EdgeCollector
	edgeSources     map[string]EdgeDeviceSource

	devicesByID map[string]memoryDevice
	deviceBySN  map[string]string
	userDevices map[string]map[string]string // userID -> deviceID -> role

	idCounter uint64
	now       func() time.Time
}

func NewMemoryStore() *MemoryStore {
	now := utcNow()
	return &MemoryStore{
		usersBySubject: map[string]string{
			"dev-user": "dev-user-id-1",
		},
		usersByID: map[string]memoryUser{
			"dev-user-id-1": {
				CurrentUser: CurrentUser{
					ID:                "dev-user-id-1",
					KeycloakSubject:   "dev-user",
					DisplayNameSource: "provider",
					CreatedAt:         now,
					UpdatedAt:         now,
				},
			},
		},
		credentials:     map[string]ProviderCredential{},
		providerDevices: map[string]ProviderDevice{},
		edgeCollectors:  map[string]EdgeCollector{},
		edgeSources:     map[string]EdgeDeviceSource{},
		devicesByID:     map[string]memoryDevice{},
		deviceBySN:      map[string]string{},
		userDevices:     map[string]map[string]string{},
		now:             utcNow,
	}
}

func (s *MemoryStore) ProviderCredentialSecretsEncrypted() bool {
	return true
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
	now := normalizeWriteTime(s.now())
	s.usersByID[id] = memoryUser{
		CurrentUser: CurrentUser{
			ID:                id,
			KeycloakSubject:   subject,
			DisplayNameSource: "provider",
			CreatedAt:         now,
			UpdatedAt:         now,
		},
	}
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
	now := normalizeWriteTime(s.now())
	row := ProviderCredential{
		ID:            s.nextID("cred"),
		UserID:        userID,
		Provider:      NormalizeProvider(in.Provider),
		AccessKeyMask: MaskAccessKey(in.AccessKey),
		AccessKey:     in.AccessKey,
		SecretKey:     in.SecretKey,
		Config:        cloneAnyMap(in.Config),
		IsActive:      in.IsActive,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if s.providerAccessKeyExistsLocked(row.Provider, HashAccessKey(in.AccessKey), "") {
		return ProviderCredential{}, ErrCredentialAlreadyExists
	}
	if row.IsActive {
		s.setExclusiveCredentialActiveLocked(userID, row.Provider, row.ID, now)
	}
	s.credentials[row.ID] = row
	if row.IsActive {
		s.rebindProviderDevicesLocked(userID, row.Provider, row.ID, now)
	}
	return cloneProviderCredential(row), nil
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
		row.AccessKey = ""
		row.SecretKey = ""
		row.Config = cloneAnyMap(row.Config)
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
	now := normalizeWriteTime(s.now())
	row.UpdatedAt = now
	if in.IsActive {
		s.setExclusiveCredentialActiveLocked(userID, row.Provider, row.ID, now)
	}
	s.credentials[row.ID] = row
	if in.IsActive {
		s.rebindProviderDevicesLocked(userID, row.Provider, row.ID, now)
	}
	return cloneProviderCredential(row), nil
}

func (s *MemoryStore) UpdateProviderCredential(_ context.Context, in UpdateProviderCredentialInput) (ProviderCredential, error) {
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
	now := normalizeWriteTime(s.now())
	row.AccessKey = in.AccessKey
	row.SecretKey = in.SecretKey
	row.Config = cloneAnyMap(in.Config)
	row.AccessKeyMask = MaskAccessKey(in.AccessKey)
	row.IsActive = in.IsActive
	row.UpdatedAt = now
	if s.providerAccessKeyExistsLocked(row.Provider, HashAccessKey(in.AccessKey), row.ID) {
		return ProviderCredential{}, ErrCredentialAlreadyExists
	}
	if row.IsActive {
		s.setExclusiveCredentialActiveLocked(userID, row.Provider, row.ID, now)
	}
	s.credentials[row.ID] = row
	if row.IsActive {
		s.rebindProviderDevicesLocked(userID, row.Provider, row.ID, now)
	}
	return cloneProviderCredential(row), nil
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
	return cloneProviderCredential(row), nil
}

func (s *MemoryStore) GetActiveProviderCredentialByUserID(_ context.Context, userID string, provider string) (ProviderCredential, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	userID = strings.TrimSpace(userID)
	provider = NormalizeProvider(provider)
	for _, row := range s.credentials {
		if row.UserID == userID && row.Provider == provider && row.IsActive {
			return cloneProviderCredential(row), nil
		}
	}
	return ProviderCredential{}, ErrCredentialNotFound
}

func (s *MemoryStore) setExclusiveCredentialActiveLocked(userID, provider, credentialID string, now time.Time) {
	for id, cred := range s.credentials {
		if cred.UserID != userID || cred.Provider != provider {
			continue
		}
		shouldBeActive := id == credentialID
		if cred.IsActive == shouldBeActive {
			continue
		}
		cred.IsActive = shouldBeActive
		cred.UpdatedAt = now
		s.credentials[id] = cred
	}
}

func (s *MemoryStore) rebindProviderDevicesLocked(userID, provider, credentialID string, now time.Time) {
	for id, device := range s.providerDevices {
		if NormalizeProvider(device.Provider) != provider {
			continue
		}
		credential, ok := s.credentials[device.CredentialID]
		if !ok || credential.UserID != userID || credential.Provider != provider {
			continue
		}
		if device.CredentialID == credentialID {
			continue
		}
		device.CredentialID = credentialID
		s.providerDevices[id] = device
	}
}

func (s *MemoryStore) providerAccessKeyExistsLocked(provider string, accessKeyHash []byte, excludeCredentialID string) bool {
	for id, cred := range s.credentials {
		if id == excludeCredentialID {
			continue
		}
		if cred.Provider != provider {
			continue
		}
		if bytes.Equal(HashAccessKey(cred.AccessKey), accessKeyHash) {
			return true
		}
	}
	return false
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
	now := normalizeWriteTime(s.now())
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

func (s *MemoryStore) GetOrProvisionCurrentUser(_ context.Context, in GetOrProvisionCurrentUserInput) (CurrentUser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	user := s.ensureUserLocked(strings.TrimSpace(in.UserSubject))
	if user.ID == "" {
		return CurrentUser{}, ErrUserNotFound
	}
	now := normalizeWriteTime(s.now())
	user.KeycloakSubject = strings.TrimSpace(in.UserSubject)
	user.Email = strings.TrimSpace(in.Email)
	user.EmailVerified = in.EmailVerified
	user.AvatarURL = strings.TrimSpace(in.AvatarURL)
	user.GivenName = strings.TrimSpace(in.GivenName)
	user.FamilyName = strings.TrimSpace(in.FamilyName)
	user.Locale = strings.TrimSpace(in.Locale)
	user.LastLoginAt = now
	if user.DisplayNameSource != "pulse" || strings.TrimSpace(user.DisplayName) == "" {
		if displayName := PreferredProviderDisplayName(in.DisplayName, in.GivenName, in.FamilyName, in.Email); displayName != "" {
			user.DisplayName = displayName
		}
		if user.DisplayNameSource == "" {
			user.DisplayNameSource = "provider"
		}
		if user.DisplayNameSource == "pulse" && strings.TrimSpace(user.DisplayName) != "" {
			user.DisplayNameSource = "provider"
		}
	}
	user.UpdatedAt = now
	s.usersByID[user.ID] = memoryUser{CurrentUser: user}
	return cloneCurrentUser(user), nil
}

func (s *MemoryStore) UpdateCurrentUserProfile(_ context.Context, in UpdateCurrentUserProfileInput) (CurrentUser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	userID, ok := s.usersBySubject[strings.TrimSpace(in.UserSubject)]
	if !ok {
		return CurrentUser{}, ErrUserNotFound
	}
	entry, ok := s.usersByID[userID]
	if !ok {
		return CurrentUser{}, ErrUserNotFound
	}
	user := entry.CurrentUser
	now := normalizeWriteTime(s.now())
	displayName := strings.TrimSpace(in.DisplayName)
	if displayName != "" {
		user.DisplayName = displayName
		user.DisplayNameSource = "pulse"
	}
	user.Timezone = strings.TrimSpace(in.Timezone)
	user.WeatherLocationEnabled = in.WeatherLocationEnabled
	if !in.WeatherLocationEnabled {
		user.WeatherLocationSource = "none"
		user.WeatherLocationLabel = ""
		user.WeatherLatitude = 0
		user.WeatherLongitude = 0
		user.HasWeatherLocation = false
	} else {
		source := strings.TrimSpace(in.WeatherLocationSource)
		if source == "" {
			source = "auto"
		}
		user.WeatherLocationSource = source
		user.WeatherLocationLabel = strings.TrimSpace(in.WeatherLocationLabel)
		user.HasWeatherLocation = in.HasWeatherLocationValue
		if in.HasWeatherLocationValue {
			user.WeatherLatitude = in.WeatherLatitude
			user.WeatherLongitude = in.WeatherLongitude
		} else {
			user.WeatherLatitude = 0
			user.WeatherLongitude = 0
		}
	}
	user.UpdatedAt = now
	s.usersByID[userID] = memoryUser{CurrentUser: user}
	return cloneCurrentUser(user), nil
}

func (s *MemoryStore) ReconcileUserSubjectByEmail(_ context.Context, in ReconcileUserSubjectByEmailInput) (CurrentUser, error) {
	email := strings.TrimSpace(in.Email)
	userSubject := strings.TrimSpace(in.UserSubject)
	if email == "" {
		return CurrentUser{}, ErrVerifiedEmailNotFound
	}
	if userSubject == "" {
		return CurrentUser{}, ErrUserNotFound
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if existingID, ok := s.usersBySubject[userSubject]; ok {
		existing, ok := s.usersByID[existingID]
		if !ok {
			return CurrentUser{}, ErrUserNotFound
		}
		if strings.EqualFold(strings.TrimSpace(existing.Email), email) {
			return cloneCurrentUser(existing.CurrentUser), nil
		}
		return CurrentUser{}, ErrUserSubjectConflict
	}

	for userID, entry := range s.usersByID {
		user := entry.CurrentUser
		if !user.EmailVerified {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(user.Email), email) {
			continue
		}
		delete(s.usersBySubject, strings.TrimSpace(user.KeycloakSubject))
		user.KeycloakSubject = userSubject
		user.UpdatedAt = normalizeWriteTime(s.now())
		s.usersByID[userID] = memoryUser{CurrentUser: user}
		s.usersBySubject[userSubject] = userID
		return cloneCurrentUser(user), nil
	}

	return CurrentUser{}, ErrVerifiedEmailNotFound
}

func (s *MemoryStore) UpsertProviderDevice(_ context.Context, in UpsertProviderDeviceInput) (ProviderDevice, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	provider := NormalizeProvider(in.Provider)
	providerDeviceID := strings.ToUpper(strings.TrimSpace(in.ProviderDeviceID))
	now := normalizeWriteTime(s.now())
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
	deviceID := strings.TrimSpace(in.DeviceID)
	if deviceID == "" {
		deviceID = strings.TrimSpace(existing.DeviceID)
	}
	credentialID := strings.TrimSpace(in.CredentialID)
	if credentialID == "" {
		credentialID = strings.TrimSpace(existing.CredentialID)
	}
	productName := strings.TrimSpace(existing.ProductName)
	if refreshed := strings.TrimSpace(in.ProductName); refreshed != "" {
		productName = refreshed
	}
	model := strings.TrimSpace(existing.Model)
	if refreshed := strings.TrimSpace(in.Model); refreshed != "" {
		model = refreshed
	}
	canonicalSN := providerDeviceID
	if strings.TrimSpace(existing.CanonicalSN) != "" {
		canonicalSN = strings.TrimSpace(existing.CanonicalSN)
	}
	if dev, ok := s.devicesByID[deviceID]; ok {
		if productName != "" {
			dev.ProductName = productName
		}
		if model != "" {
			dev.Model = model
		}
		dev.UpdatedAt = now
		s.devicesByID[dev.ID] = dev
		canonicalSN = dev.EcoflowSN
	}
	row := ProviderDevice{
		ID:                 existingID,
		DeviceID:           deviceID,
		Provider:           provider,
		ProviderDeviceID:   providerDeviceID,
		CredentialID:       credentialID,
		CanonicalSN:        canonicalSN,
		ProductName:        productName,
		Model:              model,
		Capabilities:       capabilities,
		Metadata:           metadata,
		IsActive:           in.IsActive,
		IngestDesiredState: strings.ToLower(strings.TrimSpace(in.IngestDesiredState)),
	}
	s.providerDevices[row.ID] = row
	return cloneProviderDevice(row), nil
}

func (s *MemoryStore) ImportProviderDevice(_ context.Context, in ImportProviderDeviceInput) (ImportedProviderDevice, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	userID, ok := s.usersBySubject[strings.TrimSpace(in.UserSubject)]
	if !ok {
		return ImportedProviderDevice{}, ErrUserNotFound
	}
	providerDeviceID := strings.TrimSpace(in.ProviderDeviceID)
	canonicalSN := strings.TrimSpace(in.CanonicalSN)
	if canonicalSN == "" {
		canonicalSN = providerDeviceID
	}
	if canonicalSN == "" {
		return ImportedProviderDevice{}, ErrDeviceNotFound
	}

	now := normalizeWriteTime(s.now())
	dev, created := s.ensureDeviceLocked(canonicalSN, in.ProductName, in.Model, now)
	if !created {
		dev.UpdatedAt = now
		s.devicesByID[dev.ID] = dev
	}
	s.ensureUserDeviceRoleLocked(userID, dev.ID, "admin")

	provider := NormalizeProvider(in.Provider)
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
	productName := strings.TrimSpace(existing.ProductName)
	if refreshed := strings.TrimSpace(in.ProductName); refreshed != "" {
		productName = refreshed
	}
	model := strings.TrimSpace(existing.Model)
	if refreshed := strings.TrimSpace(in.Model); refreshed != "" {
		model = refreshed
	}
	providerDevice := ProviderDevice{
		ID:                 existingID,
		DeviceID:           dev.ID,
		Provider:           provider,
		ProviderDeviceID:   providerDeviceID,
		CredentialID:       strings.TrimSpace(in.CredentialID),
		CanonicalSN:        dev.EcoflowSN,
		ProductName:        productName,
		Model:              model,
		Capabilities:       capabilities,
		Metadata:           metadata,
		IsActive:           in.IsActive,
		IngestDesiredState: strings.ToLower(strings.TrimSpace(in.IngestDesiredState)),
	}
	s.providerDevices[providerDevice.ID] = providerDevice

	userDevice := UserDevice{
		DeviceID:    dev.ID,
		EcoflowSN:   dev.EcoflowSN,
		ProductName: dev.ProductName,
		Model:       dev.Model,
		Role:        "admin",
		CreatedAt:   dev.CreatedAt,
		UpdatedAt:   dev.UpdatedAt,
	}
	return ImportedProviderDevice{
		ProviderDevice: cloneProviderDevice(providerDevice),
		UserDevice:     userDevice,
	}, nil
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

func (s *MemoryStore) GetProviderDeviceByDeviceID(_ context.Context, deviceID string) (ProviderDevice, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return ProviderDevice{}, ErrDeviceNotFound
	}

	var (
		best     ProviderDevice
		found    bool
		bestRank = -1
	)
	for _, row := range s.providerDevices {
		if strings.TrimSpace(row.DeviceID) != deviceID {
			continue
		}
		rank := 0
		if row.IsActive {
			rank += 2
		}
		if strings.TrimSpace(strings.ToLower(row.IngestDesiredState)) == "active" {
			rank++
		}
		if !found || rank > bestRank || (rank == bestRank && row.ProviderDeviceID < best.ProviderDeviceID) {
			best = cloneProviderDevice(row)
			bestRank = rank
			found = true
		}
	}
	if !found {
		return ProviderDevice{}, ErrDeviceNotFound
	}
	return best, nil
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
			CredentialConfig:   cloneAnyMap(cred.Config),
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

func (s *MemoryStore) SearchAdminLogFilters(_ context.Context, in SearchAdminLogFiltersInput) ([]AdminLogFilterOption, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	kind := normalizeAdminLogFilterKind(in.Kind)
	query := strings.ToLower(strings.TrimSpace(in.Query))
	limit := normalizeAdminLogFilterLimit(in.Limit)
	provider := NormalizeProvider(in.Provider)
	requestedDeviceIDs := adminLogDeviceIDSet(in.DeviceIDs)
	out := make([]AdminLogFilterOption, 0, limit)
	var scopedDeviceIDs map[string]struct{}
	if !in.GlobalAdmin {
		userID, ok := s.usersBySubject[strings.TrimSpace(in.UserSubject)]
		if !ok {
			return out, nil
		}
		scopedDeviceIDs = make(map[string]struct{}, len(s.userDevices[userID]))
		for deviceID := range s.userDevices[userID] {
			scopedDeviceIDs[deviceID] = struct{}{}
		}
	}

	if kind == "" || kind == "device" || kind == "serial" {
		devices := make([]memoryDevice, 0, len(s.devicesByID))
		for _, device := range s.devicesByID {
			if scopedDeviceIDs != nil {
				if _, ok := scopedDeviceIDs[device.ID]; !ok {
					continue
				}
			}
			if requestedDeviceIDs != nil {
				if _, ok := requestedDeviceIDs[device.ID]; !ok {
					continue
				}
			}
			if provider != "" && !s.memoryDeviceMatchesProviderLocked(device.ID, provider) {
				continue
			}
			devices = append(devices, device)
		}
		sort.Slice(devices, func(i, j int) bool {
			left := adminLogFirstNonEmpty(devices[i].ProductName, devices[i].Model, devices[i].EcoflowSN, devices[i].ID)
			right := adminLogFirstNonEmpty(devices[j].ProductName, devices[j].Model, devices[j].EcoflowSN, devices[j].ID)
			return strings.ToLower(left) < strings.ToLower(right)
		})
		for _, device := range devices {
			if len(out) >= limit {
				break
			}
			if query != "" && !matchesAdminLogQuery(query, device.ID, device.EcoflowSN, device.ProductName, device.Model) {
				continue
			}
			if kind == "" || kind == "device" {
				out = append(out, AdminLogFilterOption{
					Kind:           "device",
					ID:             device.ID,
					Label:          adminLogFirstNonEmpty(device.ProductName, device.Model, "Device "+shortID(device.ID)),
					SecondaryLabel: adminLogFirstNonEmpty(device.Model, "UUID "+shortID(device.ID)),
					DeviceIDs:      []string{device.ID},
					Provider:       provider,
				})
			}
			if len(out) >= limit {
				break
			}
			if kind == "" || kind == "serial" {
				out = append(out, AdminLogFilterOption{
					Kind:           "serial",
					ID:             device.ID,
					Label:          device.EcoflowSN,
					SecondaryLabel: adminLogFirstNonEmpty(device.ProductName, device.Model, "Device "+shortID(device.ID)),
					DeviceIDs:      []string{device.ID},
					Provider:       provider,
				})
			}
		}
	}

	if in.GlobalAdmin && (kind == "" || kind == "user") {
		users := make([]memoryUser, 0, len(s.usersByID))
		for _, user := range s.usersByID {
			users = append(users, user)
		}
		sort.Slice(users, func(i, j int) bool {
			return strings.ToLower(adminLogFirstNonEmpty(users[i].Email, users[i].DisplayName, users[i].ID)) <
				strings.ToLower(adminLogFirstNonEmpty(users[j].Email, users[j].DisplayName, users[j].ID))
		})
		for _, user := range users {
			if len(out) >= limit {
				break
			}
			if strings.TrimSpace(user.Email) == "" {
				continue
			}
			if query != "" && !matchesAdminLogQuery(query, user.Email, user.DisplayName, user.KeycloakSubject) {
				continue
			}
			deviceIDs := make([]string, 0, len(s.userDevices[user.ID]))
			for deviceID := range s.userDevices[user.ID] {
				if requestedDeviceIDs != nil {
					if _, ok := requestedDeviceIDs[deviceID]; !ok {
						continue
					}
				}
				deviceIDs = append(deviceIDs, deviceID)
			}
			if requestedDeviceIDs != nil && len(deviceIDs) == 0 {
				continue
			}
			sort.Strings(deviceIDs)
			out = append(out, AdminLogFilterOption{
				Kind:           "user",
				ID:             user.ID,
				Label:          user.Email,
				SecondaryLabel: adminLogFirstNonEmpty(user.DisplayName, fmt.Sprintf("%d devices", len(deviceIDs))),
				DeviceIDs:      deviceIDs,
			})
		}
	}
	return out, nil
}

func (s *MemoryStore) CreateEdgeCollector(_ context.Context, in CreateEdgeCollectorInput) (EdgeCollector, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	userID, ok := s.usersBySubject[strings.TrimSpace(in.UserSubject)]
	if !ok {
		return EdgeCollector{}, ErrUserNotFound
	}
	setupTokenHash := strings.TrimSpace(in.SetupTokenHash)
	if setupTokenHash == "" {
		return EdgeCollector{}, ErrEdgeCollectorNotFound
	}
	now := normalizeWriteTime(s.now())
	displayName := strings.TrimSpace(in.DisplayName)
	if displayName == "" {
		displayName = "Pulse Edge Collector"
	}
	row := EdgeCollector{
		ID:             s.nextID("edgecol"),
		UserID:         userID,
		DisplayName:    displayName,
		SetupTokenHash: setupTokenHash,
		IsActive:       false,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	s.edgeCollectors[row.ID] = row
	return cloneEdgeCollector(row), nil
}

func (s *MemoryStore) ListEdgeCollectors(_ context.Context, in ListEdgeCollectorsInput) ([]EdgeCollector, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	userID, ok := s.usersBySubject[strings.TrimSpace(in.UserSubject)]
	if !ok {
		return []EdgeCollector{}, nil
	}
	out := make([]EdgeCollector, 0, len(s.edgeCollectors))
	for _, row := range s.edgeCollectors {
		if row.UserID == userID {
			out = append(out, cloneEdgeCollector(row))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID > out[j].ID
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

func (s *MemoryStore) EnrollEdgeCollector(_ context.Context, in EnrollEdgeCollectorInput) (EdgeCollector, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	setupTokenHash := strings.TrimSpace(in.SetupTokenHash)
	collectorSecretHash := strings.TrimSpace(in.CollectorSecretHash)
	if setupTokenHash == "" || collectorSecretHash == "" {
		return EdgeCollector{}, ErrEdgeCollectorNotFound
	}
	now := normalizeWriteTime(s.now())
	for id, row := range s.edgeCollectors {
		if row.SetupTokenHash != setupTokenHash {
			continue
		}
		row.SetupTokenHash = ""
		row.CollectorSecretHash = collectorSecretHash
		row.CollectorVersion = strings.TrimSpace(in.CollectorVersion)
		row.Hostname = strings.TrimSpace(in.Hostname)
		row.IsActive = true
		row.LastHeartbeatAt = now
		row.UpdatedAt = now
		s.edgeCollectors[id] = row
		return cloneEdgeCollector(row), nil
	}
	return EdgeCollector{}, ErrEdgeCollectorNotFound
}

func (s *MemoryStore) GetEdgeCollectorBySetupTokenHash(_ context.Context, setupTokenHash string) (EdgeCollector, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	setupTokenHash = strings.TrimSpace(setupTokenHash)
	if setupTokenHash == "" {
		return EdgeCollector{}, ErrEdgeCollectorNotFound
	}
	for _, row := range s.edgeCollectors {
		if row.SetupTokenHash == setupTokenHash && !row.IsActive {
			return cloneEdgeCollector(row), nil
		}
	}
	return EdgeCollector{}, ErrEdgeCollectorNotFound
}

func (s *MemoryStore) AuthenticateEdgeCollector(_ context.Context, in AuthenticateEdgeCollectorInput) (EdgeCollector, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	collectorSecretHash := strings.TrimSpace(in.CollectorSecretHash)
	if collectorSecretHash == "" {
		return EdgeCollector{}, ErrEdgeCollectorNotFound
	}
	for _, row := range s.edgeCollectors {
		if row.CollectorSecretHash == collectorSecretHash && row.IsActive {
			return cloneEdgeCollector(row), nil
		}
	}
	return EdgeCollector{}, ErrEdgeCollectorNotFound
}

func (s *MemoryStore) UpdateEdgeCollectorHeartbeat(_ context.Context, in UpdateEdgeCollectorHeartbeatInput) (EdgeCollector, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	row, ok := s.edgeCollectors[strings.TrimSpace(in.CollectorID)]
	if !ok || !row.IsActive {
		return EdgeCollector{}, ErrEdgeCollectorNotFound
	}
	now := normalizeWriteTime(s.now())
	row.LastHeartbeatAt = now
	row.UpdatedAt = now
	if version := strings.TrimSpace(in.CollectorVersion); version != "" {
		row.CollectorVersion = version
	}
	if hostname := strings.TrimSpace(in.Hostname); hostname != "" {
		row.Hostname = hostname
	}
	s.edgeCollectors[row.ID] = row
	return cloneEdgeCollector(row), nil
}

func (s *MemoryStore) UpsertEdgeDeviceSource(_ context.Context, in UpsertEdgeDeviceSourceInput) (EdgeDeviceSource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	collector, ok := s.edgeCollectors[strings.TrimSpace(in.CollectorID)]
	if !ok || !collector.IsActive {
		return EdgeDeviceSource{}, ErrEdgeCollectorNotFound
	}
	provider := NormalizeProvider(in.Provider)
	transport := normalizeEdgeTransport(in.Transport)
	providerDeviceID := strings.ToUpper(strings.TrimSpace(in.ProviderDeviceID))
	if provider == "" || transport == "" || providerDeviceID == "" {
		return EdgeDeviceSource{}, ErrEdgeDeviceSourceNotFound
	}
	var existingID string
	for id, row := range s.edgeSources {
		if row.CollectorID == collector.ID &&
			row.Provider == provider &&
			row.Transport == transport &&
			row.ProviderDeviceID == providerDeviceID {
			existingID = id
			break
		}
	}
	now := normalizeWriteTime(s.now())
	observedAt := normalizeWriteTime(in.ObservedAt)
	if observedAt.IsZero() {
		observedAt = now
	}
	if existingID == "" {
		existingID = s.nextID("edgesrc")
	}
	row := s.edgeSources[existingID]
	if row.ID == "" {
		row.ID = existingID
		row.CollectorID = collector.ID
		row.UserID = collector.UserID
		row.Provider = provider
		row.Transport = transport
		row.ProviderDeviceID = providerDeviceID
		row.Status = "pending"
		row.CreatedAt = now
	}
	row.DisplayName = strings.TrimSpace(in.DisplayName)
	row.Model = strings.TrimSpace(in.Model)
	row.AddressHash = strings.TrimSpace(in.AddressHash)
	row.RSSIDBm = in.RSSIDBm
	row.Metadata = cloneAnyMap(in.Metadata)
	row.LastSeenAt = observedAt
	row.UpdatedAt = now
	s.edgeSources[row.ID] = row
	return cloneEdgeDeviceSource(row), nil
}

func (s *MemoryStore) ListEdgeDeviceSources(_ context.Context, in ListEdgeDeviceSourcesInput) ([]EdgeDeviceSource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	userID, ok := s.usersBySubject[strings.TrimSpace(in.UserSubject)]
	if !ok {
		return []EdgeDeviceSource{}, nil
	}
	collectorID := strings.TrimSpace(in.CollectorID)
	status := strings.ToLower(strings.TrimSpace(in.Status))
	out := make([]EdgeDeviceSource, 0, len(s.edgeSources))
	for _, row := range s.edgeSources {
		if row.UserID != userID {
			continue
		}
		if collectorID != "" && row.CollectorID != collectorID {
			continue
		}
		if status != "" && row.Status != status {
			continue
		}
		out = append(out, cloneEdgeDeviceSource(row))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].LastSeenAt.Equal(out[j].LastSeenAt) {
			return out[i].ProviderDeviceID < out[j].ProviderDeviceID
		}
		return out[i].LastSeenAt.After(out[j].LastSeenAt)
	})
	return out, nil
}

func (s *MemoryStore) ApproveEdgeDeviceSource(_ context.Context, in ApproveEdgeDeviceSourceInput) (ApprovedEdgeDeviceSource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	userID, ok := s.usersBySubject[strings.TrimSpace(in.UserSubject)]
	if !ok {
		return ApprovedEdgeDeviceSource{}, ErrUserNotFound
	}
	source, ok := s.edgeSources[strings.TrimSpace(in.SourceID)]
	if !ok || source.UserID != userID {
		return ApprovedEdgeDeviceSource{}, ErrEdgeDeviceSourceNotFound
	}
	now := normalizeWriteTime(s.now())
	var dev memoryDevice
	deviceID := strings.TrimSpace(in.DeviceID)
	if deviceID != "" {
		existing, ok := s.devicesByID[deviceID]
		if !ok {
			return ApprovedEdgeDeviceSource{}, ErrDeviceNotFound
		}
		role := s.userDevices[userID][deviceID]
		if role != "admin" {
			return ApprovedEdgeDeviceSource{}, ErrPermissionDenied
		}
		dev = existing
	} else {
		productName := strings.TrimSpace(in.ProductName)
		if productName == "" {
			productName = source.DisplayName
		}
		model := strings.TrimSpace(in.Model)
		if model == "" {
			model = source.Model
		}
		var created bool
		dev, created = s.ensureDeviceLocked(source.ProviderDeviceID, productName, model, now)
		if !created {
			dev.UpdatedAt = now
			s.devicesByID[dev.ID] = dev
		}
	}
	s.ensureUserDeviceRoleLocked(userID, dev.ID, "admin")
	source.Status = "linked"
	source.LinkedDeviceID = dev.ID
	source.UpdatedAt = now
	s.edgeSources[source.ID] = source

	return ApprovedEdgeDeviceSource{
		Source: cloneEdgeDeviceSource(source),
		Device: UserDevice{
			DeviceID:    dev.ID,
			EcoflowSN:   dev.EcoflowSN,
			ProductName: dev.ProductName,
			Model:       dev.Model,
			Role:        "admin",
			CreatedAt:   dev.CreatedAt,
			UpdatedAt:   dev.UpdatedAt,
		},
	}, nil
}

func (s *MemoryStore) GetLinkedEdgeDeviceSource(_ context.Context, in GetLinkedEdgeDeviceSourceInput) (EdgeDeviceSource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	provider := NormalizeProvider(in.Provider)
	transport := normalizeEdgeTransport(in.Transport)
	providerDeviceID := strings.ToUpper(strings.TrimSpace(in.ProviderDeviceID))
	for _, row := range s.edgeSources {
		if row.CollectorID == strings.TrimSpace(in.CollectorID) &&
			row.Provider == provider &&
			row.Transport == transport &&
			row.ProviderDeviceID == providerDeviceID &&
			row.Status == "linked" &&
			strings.TrimSpace(row.LinkedDeviceID) != "" {
			return cloneEdgeDeviceSource(row), nil
		}
	}
	return EdgeDeviceSource{}, ErrEdgeDeviceSourceNotFound
}

func (s *MemoryStore) memoryDeviceMatchesProviderLocked(deviceID string, provider string) bool {
	for _, row := range s.providerDevices {
		if row.DeviceID == deviceID && row.Provider == provider {
			return true
		}
	}
	return false
}

func (s *MemoryStore) ensureDeviceLocked(sn string, productName string, model string, now time.Time) (memoryDevice, bool) {
	canonicalSN := strings.ToUpper(strings.TrimSpace(sn))
	if canonicalSN == "" {
		return memoryDevice{}, false
	}
	if existingID, ok := s.deviceBySN[canonicalSN]; ok {
		dev := s.devicesByID[existingID]
		if refreshed := strings.TrimSpace(productName); refreshed != "" {
			dev.ProductName = refreshed
		}
		if refreshed := strings.TrimSpace(model); refreshed != "" {
			dev.Model = refreshed
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

func (s *MemoryStore) ensureUserLocked(subject string) CurrentUser {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return CurrentUser{}
	}
	if id, ok := s.usersBySubject[subject]; ok {
		if entry, found := s.usersByID[id]; found {
			return entry.CurrentUser
		}
	}
	id := s.nextID("usr")
	now := normalizeWriteTime(s.now())
	user := CurrentUser{
		ID:                id,
		KeycloakSubject:   subject,
		DisplayNameSource: "provider",
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	s.usersBySubject[subject] = id
	s.usersByID[id] = memoryUser{CurrentUser: user}
	return user
}

func cloneProviderDevice(in ProviderDevice) ProviderDevice {
	out := in
	out.Capabilities = cloneAnyMap(in.Capabilities)
	out.Metadata = cloneAnyMap(in.Metadata)
	return out
}

func cloneProviderCredential(in ProviderCredential) ProviderCredential {
	out := in
	out.Config = cloneAnyMap(in.Config)
	return out
}

func cloneCurrentUser(in CurrentUser) CurrentUser {
	return in
}

func cloneEdgeCollector(in EdgeCollector) EdgeCollector {
	return in
}

func cloneEdgeDeviceSource(in EdgeDeviceSource) EdgeDeviceSource {
	out := in
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

func normalizeEdgeTransport(value string) string {
	transport := strings.ToLower(strings.TrimSpace(value))
	if transport == "" {
		return "ble"
	}
	return transport
}

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

type memoryUser struct {
	CurrentUser
}

type MemoryStore struct {
	mu sync.RWMutex

	usersBySubject  map[string]string
	usersByID       map[string]memoryUser
	credentials     map[string]ProviderCredential
	providerDevices map[string]ProviderDevice

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
		devicesByID:     map[string]memoryDevice{},
		deviceBySN:      map[string]string{},
		userDevices:     map[string]map[string]string{},
		now:             utcNow,
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
	row.UpdatedAt = normalizeWriteTime(s.now())
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

func cloneCurrentUser(in CurrentUser) CurrentUser {
	return in
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

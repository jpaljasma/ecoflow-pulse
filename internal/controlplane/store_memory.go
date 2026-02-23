package controlplane

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

type MemoryStore struct {
	mu sync.RWMutex

	usersBySubject map[string]string
	credentials    map[string]ProviderCredential
	devices        map[string]ProviderDevice
	idCounter      uint64
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		usersBySubject: map[string]string{
			"dev-user": "dev-user-id-1",
		},
		credentials: map[string]ProviderCredential{},
		devices:     map[string]ProviderDevice{},
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
	s.devices[device.ID] = device
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

func (s *MemoryStore) ListProviderDevices(_ context.Context, in ListProviderDevicesInput) ([]ProviderDevice, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	provider := NormalizeProvider(in.Provider)
	out := make([]ProviderDevice, 0, len(s.devices))
	for _, row := range s.devices {
		if provider != "" && row.Provider != provider {
			continue
		}
		if in.ActiveOnly && !row.IsActive {
			continue
		}
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Provider == out[j].Provider {
			return out[i].CanonicalSN < out[j].CanonicalSN
		}
		return out[i].Provider < out[j].Provider
	})
	return out, nil
}

func (s *MemoryStore) nextID(prefix string) string {
	seq := atomic.AddUint64(&s.idCounter, 1)
	return fmt.Sprintf("%s-%d", prefix, seq)
}

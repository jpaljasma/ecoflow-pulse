package energydashboard

import (
	"fmt"
	"slices"
	"strings"
)

const (
	ScopeModeAll    = "all"
	ScopeModeSingle = "single"
)

type Scope struct {
	Mode              string
	DeviceID          string
	ResolvedDeviceIDs []string
}

func ResolveScope(requested string, visibleDeviceIDs []string) (Scope, error) {
	resolved := uniqueSortedDeviceIDs(visibleDeviceIDs)
	if len(resolved) == 0 {
		return Scope{Mode: ScopeModeAll, ResolvedDeviceIDs: []string{}}, nil
	}

	requested = strings.TrimSpace(requested)
	if requested == "" || strings.EqualFold(requested, ScopeModeAll) {
		return Scope{
			Mode:              ScopeModeAll,
			ResolvedDeviceIDs: resolved,
		}, nil
	}

	if !slices.Contains(resolved, requested) {
		return Scope{}, fmt.Errorf("device not visible in scope: %s", requested)
	}

	return Scope{
		Mode:              ScopeModeSingle,
		DeviceID:          requested,
		ResolvedDeviceIDs: []string{requested},
	}, nil
}

func uniqueSortedDeviceIDs(deviceIDs []string) []string {
	seen := make(map[string]struct{}, len(deviceIDs))
	out := make([]string, 0, len(deviceIDs))
	for _, raw := range deviceIDs {
		deviceID := strings.TrimSpace(raw)
		if deviceID == "" {
			continue
		}
		if _, ok := seen[deviceID]; ok {
			continue
		}
		seen[deviceID] = struct{}{}
		out = append(out, deviceID)
	}
	slices.Sort(out)
	return out
}

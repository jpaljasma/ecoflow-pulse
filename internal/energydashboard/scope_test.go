package energydashboard

import "testing"

func TestResolveScopeDefaultsToAllVisibleDevices(t *testing.T) {
	t.Parallel()

	scope, err := ResolveScope("", []string{"b", "a", "a"})
	if err != nil {
		t.Fatalf("ResolveScope failed: %v", err)
	}
	if scope.Mode != ScopeModeAll {
		t.Fatalf("mode mismatch: got=%s want=%s", scope.Mode, ScopeModeAll)
	}
	if len(scope.ResolvedDeviceIDs) != 2 || scope.ResolvedDeviceIDs[0] != "a" || scope.ResolvedDeviceIDs[1] != "b" {
		t.Fatalf("resolved device ids mismatch: got=%v", scope.ResolvedDeviceIDs)
	}
}

func TestResolveScopeSupportsExplicitAll(t *testing.T) {
	t.Parallel()

	scope, err := ResolveScope("all", []string{"device-1"})
	if err != nil {
		t.Fatalf("ResolveScope failed: %v", err)
	}
	if scope.Mode != ScopeModeAll {
		t.Fatalf("mode mismatch: got=%s want=%s", scope.Mode, ScopeModeAll)
	}
}

func TestResolveScopeSupportsSingleVisibleDevice(t *testing.T) {
	t.Parallel()

	scope, err := ResolveScope("device-2", []string{"device-1", "device-2"})
	if err != nil {
		t.Fatalf("ResolveScope failed: %v", err)
	}
	if scope.Mode != ScopeModeSingle {
		t.Fatalf("mode mismatch: got=%s want=%s", scope.Mode, ScopeModeSingle)
	}
	if scope.DeviceID != "device-2" {
		t.Fatalf("device id mismatch: got=%s want=device-2", scope.DeviceID)
	}
}

func TestResolveScopeRejectsInvisibleDevice(t *testing.T) {
	t.Parallel()

	if _, err := ResolveScope("device-3", []string{"device-1", "device-2"}); err == nil {
		t.Fatalf("expected ResolveScope to reject invisible device")
	}
}

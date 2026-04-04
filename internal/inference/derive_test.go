package inference

import "testing"

func TestMaxBatteryPacksForModelUsesUltraXCeiling(t *testing.T) {
	t.Parallel()

	if got := maxBatteryPacksForModel("DELTA Pro Ultra X"); got != 10 {
		t.Fatalf("maxBatteryPacksForModel() = %d, want 10", got)
	}
	if got := maxBatteryPacksForModel("DELTA Pro Ultra"); got != 5 {
		t.Fatalf("maxBatteryPacksForModel() regular DPU = %d, want 5", got)
	}
}

func TestBatteryCapacityKWhUsesUltraXPackSize(t *testing.T) {
	t.Parallel()

	got, ok := batteryCapacityKWh("DELTA Pro Ultra X", 4)
	if !ok {
		t.Fatal("batteryCapacityKWh() returned ok=false for DPU-X")
	}
	if got != 24.576 {
		t.Fatalf("batteryCapacityKWh() = %.3f, want 24.576", got)
	}
}

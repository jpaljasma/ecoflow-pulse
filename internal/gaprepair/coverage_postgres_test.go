package gaprepair

import "testing"

func TestNewPostgresCoverageStoreRejectsBlankDSN(t *testing.T) {
	t.Parallel()

	store, err := NewPostgresCoverageStore("   ")
	if err == nil {
		t.Fatalf("expected error for blank dsn")
	}
	if store != nil {
		t.Fatalf("expected nil store on error")
	}
}

func TestNormalizeProviderDeviceIDs(t *testing.T) {
	t.Parallel()

	got := normalizeProviderDeviceIDs([]string{" r1 ", "R1", "", "y2", "Y2", " z3 "})
	want := []string{"R1", "Y2", "Z3"}
	if len(got) != len(want) {
		t.Fatalf("normalizeProviderDeviceIDs length mismatch: got=%v want=%v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("normalizeProviderDeviceIDs mismatch at %d: got=%v want=%v", i, got, want)
		}
	}
}

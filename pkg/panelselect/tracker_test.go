package panelselect

import "testing"

func TestTrackerFeatureVector(t *testing.T) {
	t.Parallel()

	tracker := NewTracker(50)
	for i := 0; i < 20; i++ {
		state := "idle"
		w := 0.0
		v := 0.0
		a := 0.0
		if i >= 5 {
			state = "charging"
			w = 40 + float64(i%5)
			v = 33 + float64(i%3)
			a = 1.0 + float64(i%4)*0.1
		}
		tracker.Observe(w, v, a, state)
	}

	features, ok := tracker.FeatureVector()
	if !ok {
		t.Fatalf("expected feature vector")
	}
	if got, want := len(features), len(FeatureNames); got != want {
		t.Fatalf("feature width mismatch: got=%d want=%d", got, want)
	}
	if features[0] <= 0 {
		t.Fatalf("median active watts should be positive, got=%.4f", features[0])
	}
	if features[2] <= 0 {
		t.Fatalf("median active volts should be positive, got=%.4f", features[2])
	}
	if features[5] <= 0 || features[5] > 1 {
		t.Fatalf("active ratio out of range: %.4f", features[5])
	}
}

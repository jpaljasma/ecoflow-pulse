package ecoflowserver

import "testing"

func TestDefaultConfigValidate(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

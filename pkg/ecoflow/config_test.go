package ecoflow

import "testing"

func TestParseEnvironment(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		want  Environment
		ok    bool
	}{
		{input: "dev", want: EnvironmentDev, ok: true},
		{input: "staging", want: EnvironmentStaging, ok: true},
		{input: "prod", want: EnvironmentProd, ok: true},
		{input: "PROD", want: EnvironmentProd, ok: true},
		{input: "qa", ok: false},
	}

	for _, tc := range cases {
		got, err := ParseEnvironment(tc.input)
		if tc.ok && err != nil {
			t.Fatalf("ParseEnvironment(%q) unexpected error: %v", tc.input, err)
		}
		if !tc.ok && err == nil {
			t.Fatalf("ParseEnvironment(%q) expected error, got nil", tc.input)
		}
		if tc.ok && got != tc.want {
			t.Fatalf("ParseEnvironment(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestDefaultConfig_DevelopmentDebugByDefault(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	if cfg.Environment != EnvironmentDev {
		t.Fatalf("default environment mismatch: got %q", cfg.Environment)
	}
	if !cfg.Logging.Debug {
		t.Fatal("expected debug logging enabled by default for dev environment")
	}
	if cfg.Logging.AdvancedDebugTelemetry {
		t.Fatal("expected advanced debug telemetry disabled by default")
	}
	if cfg.Logging.DebugLogHeaders {
		t.Fatal("expected debug header logging disabled by default")
	}
	if cfg.Logging.Logger == nil {
		t.Fatal("expected default logger to be configured")
	}
	if cfg.Compression.RequestCompressionMinBytes != 0 {
		t.Fatalf("default request compression min bytes mismatch: got %d", cfg.Compression.RequestCompressionMinBytes)
	}
}

func TestConfigFromEnvironment_DebugOverride(t *testing.T) {
	t.Setenv("ECOFLOW_ENV", "dev")
	t.Setenv("ECOFLOW_DEV_ACCESS_KEY", "dev-ak")
	t.Setenv("ECOFLOW_DEV_SECRET_KEY", "dev-sk")
	t.Setenv("ECOFLOW_DEBUG", "false")
	t.Setenv("ECOFLOW_ADVANCED_DEBUG_TELEMETRY", "true")
	t.Setenv("ECOFLOW_DEBUG_LOG_HEADERS", "true")

	cfg, err := ConfigFromEnvironment()
	if err != nil {
		t.Fatalf("ConfigFromEnvironment() error = %v", err)
	}
	if cfg.Logging.Debug {
		t.Fatal("expected debug logging disabled when ECOFLOW_DEBUG=false")
	}
	if cfg.Logging.Logger == nil {
		t.Fatal("expected logger to be configured")
	}
	if !cfg.Logging.AdvancedDebugTelemetry {
		t.Fatal("expected advanced debug telemetry enabled by env override")
	}
	if !cfg.Logging.DebugLogHeaders {
		t.Fatal("expected debug header logging enabled by env override")
	}
}

func TestConfigFromEnvironment_CompressionOverrides(t *testing.T) {
	t.Setenv("ECOFLOW_ENV", "dev")
	t.Setenv("ECOFLOW_DEV_ACCESS_KEY", "dev-ak")
	t.Setenv("ECOFLOW_DEV_SECRET_KEY", "dev-sk")
	t.Setenv("ECOFLOW_REQUEST_COMPRESSION", "true")
	t.Setenv("ECOFLOW_RESPONSE_COMPRESSION", "true")
	t.Setenv("ECOFLOW_REQUEST_COMPRESSION_ALGORITHM", "deflate")
	t.Setenv("ECOFLOW_REQUEST_COMPRESSION_MIN_BYTES", "2048")
	t.Setenv("ECOFLOW_ACCEPT_ENCODINGS", "gzip, deflate")

	cfg, err := ConfigFromEnvironment()
	if err != nil {
		t.Fatalf("ConfigFromEnvironment() error = %v", err)
	}
	if !cfg.Compression.EnableRequestCompression {
		t.Fatal("expected request compression enabled")
	}
	if !cfg.Compression.EnableResponseCompression {
		t.Fatal("expected response compression enabled")
	}
	if cfg.Compression.RequestCompressionAlgorithm != "deflate" {
		t.Fatalf("request compression algorithm mismatch: got %q", cfg.Compression.RequestCompressionAlgorithm)
	}
	if cfg.Compression.RequestCompressionMinBytes != 2048 {
		t.Fatalf("request compression min bytes mismatch: got %d", cfg.Compression.RequestCompressionMinBytes)
	}
	if len(cfg.Compression.ResponseAcceptedEncodings) != 2 {
		t.Fatalf("accepted encodings length mismatch: got %d", len(cfg.Compression.ResponseAcceptedEncodings))
	}
}

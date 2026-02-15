package ecoflow

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultBaseURL = "https://api.ecoflow.com"
)

// Environment identifies which credential/config profile to use.
type Environment string

const (
	// EnvironmentDev is the local/developer environment profile.
	EnvironmentDev Environment = "dev"
	// EnvironmentStaging is the pre-production environment profile.
	EnvironmentStaging Environment = "staging"
	// EnvironmentProd is the production environment profile.
	EnvironmentProd Environment = "prod"
)

// ParseEnvironment validates and normalizes a string into an Environment.
func ParseEnvironment(value string) (Environment, error) {
	switch Environment(strings.ToLower(strings.TrimSpace(value))) {
	case EnvironmentDev:
		return EnvironmentDev, nil
	case EnvironmentStaging:
		return EnvironmentStaging, nil
	case EnvironmentProd:
		return EnvironmentProd, nil
	default:
		return "", fmt.Errorf("unsupported environment %q", value)
	}
}

// Config controls Client behavior, including signing, transport, retries,
// credentials, logging, and observability.
type Config struct {
	Environment Environment
	BaseURL     string
	UserAgent   string
	Logging     LoggingOptions
	// Compression controls request-body and response transfer encodings.
	Compression CompressionOptions

	HTTPClient      *http.Client
	Transport       *http.Transport
	RequestTimeout  time.Duration
	TransportTuning TransportTuning

	RetryPolicy RetryPolicy
	Signer      Signer

	CredentialsProvider CredentialsProvider
	Observability       ObservabilityOptions
}

// DefaultConfig returns production-safe client defaults.
//
// By design, the default profile is developer-friendly (`dev`) so local
// debugging is enabled unless overridden.
func DefaultConfig() Config {
	logging := defaultLoggingOptions(EnvironmentDev)
	return Config{
		Environment:     EnvironmentDev,
		BaseURL:         defaultBaseURL,
		UserAgent:       "ecoflow-go-client/0.1.0",
		Logging:         logging,
		Compression:     DefaultCompressionOptions(),
		RequestTimeout:  30 * time.Second,
		TransportTuning: DefaultTransportTuning(),
		RetryPolicy:     DefaultRetryPolicy(),
		Signer:          NewHMACSHA256Signer(),
	}
}

// ConfigFromEnvironment builds Config from process environment variables.
//
// It attempts to load a dotenv file before reading env vars:
//   - ECOFLOW_DOTENV_PATH, when set
//   - .env, otherwise
//
// Existing non-empty process variables take precedence over dotenv values.
func ConfigFromEnvironment() (Config, error) {
	cfg := DefaultConfig()

	dotEnvPath := os.Getenv("ECOFLOW_DOTENV_PATH")
	if dotEnvPath == "" {
		dotEnvPath = ".env"
	}
	if err := loadDotEnvFile(dotEnvPath, false); err != nil {
		return Config{}, err
	}

	if envRaw := os.Getenv("ECOFLOW_ENV"); envRaw != "" {
		env, err := ParseEnvironment(envRaw)
		if err != nil {
			return Config{}, err
		}
		cfg.Environment = env
	}
	cfg.Logging = defaultLoggingOptions(cfg.Environment)

	if v := os.Getenv("ECOFLOW_BASE_URL"); v != "" {
		cfg.BaseURL = v
	}
	if v := os.Getenv("ECOFLOW_USER_AGENT"); v != "" {
		cfg.UserAgent = v
	}
	if v := os.Getenv("ECOFLOW_REQUEST_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid ECOFLOW_REQUEST_TIMEOUT: %w", err)
		}
		cfg.RequestTimeout = d
	}
	if enabled, found, err := parseBoolEnvironment("ECOFLOW_REQUEST_COMPRESSION"); err != nil {
		return Config{}, err
	} else if found {
		cfg.Compression.EnableRequestCompression = enabled
	}
	if enabled, found, err := parseBoolEnvironment("ECOFLOW_RESPONSE_COMPRESSION"); err != nil {
		return Config{}, err
	} else if found {
		cfg.Compression.EnableResponseCompression = enabled
	}
	if v := strings.TrimSpace(os.Getenv("ECOFLOW_REQUEST_COMPRESSION_ALGORITHM")); v != "" {
		cfg.Compression.RequestCompressionAlgorithm = strings.ToLower(v)
	}
	if v := strings.TrimSpace(os.Getenv("ECOFLOW_REQUEST_COMPRESSION_MIN_BYTES")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid ECOFLOW_REQUEST_COMPRESSION_MIN_BYTES: %w", err)
		}
		cfg.Compression.RequestCompressionMinBytes = n
	}
	if v := strings.TrimSpace(os.Getenv("ECOFLOW_ACCEPT_ENCODINGS")); v != "" {
		parts := strings.Split(v, ",")
		cfg.Compression.ResponseAcceptedEncodings = cfg.Compression.ResponseAcceptedEncodings[:0]
		for _, part := range parts {
			if encoding := strings.TrimSpace(strings.ToLower(part)); encoding != "" {
				cfg.Compression.ResponseAcceptedEncodings = append(cfg.Compression.ResponseAcceptedEncodings, encoding)
			}
		}
	}
	if debug, found, err := parseBoolEnvironment("ECOFLOW_DEBUG"); err != nil {
		return Config{}, err
	} else if found {
		cfg.Logging.Debug = debug
		if debug {
			cfg.Logging.Logger = defaultLoggingOptions(EnvironmentDev).Logger
		} else {
			cfg.Logging.Logger = defaultLoggingOptions(EnvironmentProd).Logger
		}
	}
	if advanced, found, err := parseBoolEnvironment("ECOFLOW_ADVANCED_DEBUG_TELEMETRY"); err != nil {
		return Config{}, err
	} else if found {
		cfg.Logging.AdvancedDebugTelemetry = advanced
	}
	if includeHeaders, found, err := parseBoolEnvironment("ECOFLOW_DEBUG_LOG_HEADERS"); err != nil {
		return Config{}, err
	} else if found {
		cfg.Logging.DebugLogHeaders = includeHeaders
	}

	cfg.CredentialsProvider = NewEnvironmentCredentialsProvider(cfg.Environment)

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// Validate performs structural validation for the configuration.
func (c Config) Validate() error {
	if _, err := ParseEnvironment(string(c.Environment)); err != nil {
		return err
	}
	if _, err := url.ParseRequestURI(c.BaseURL); err != nil {
		return fmt.Errorf("invalid base URL %q: %w", c.BaseURL, err)
	}
	if c.CredentialsProvider == nil {
		return errors.New("credentials provider is required")
	}
	if c.Signer == nil {
		return errors.New("signer is required")
	}
	if c.RetryPolicy.MaxAttempts <= 0 {
		return errors.New("retry max attempts must be > 0")
	}
	if c.RetryPolicy.BaseDelay < 0 || c.RetryPolicy.MaxDelay < 0 {
		return errors.New("retry delays must be >= 0")
	}
	if c.RequestTimeout <= 0 && c.HTTPClient == nil {
		return errors.New("request timeout must be > 0 when HTTP client is not supplied")
	}
	if c.Compression.RequestCompressionMinBytes < 0 {
		return errors.New("request compression min bytes must be >= 0")
	}
	return nil
}

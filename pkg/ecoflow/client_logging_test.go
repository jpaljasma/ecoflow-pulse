package ecoflow

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/jpaljasma/ecoflow-pulse/pkg/logger"
)

func TestClient_Do_LogsOutgoingRequestWhenDebugEnabled(t *testing.T) {
	t.Parallel()

	credentialsProvider, err := NewStaticCredentialsProvider("ak-test-1234", "sk-test-1234")
	if err != nil {
		t.Fatalf("NewStaticCredentialsProvider() error = %v", err)
	}

	var logs bytes.Buffer
	cfg := DefaultConfig()
	cfg.BaseURL = "https://api.ecoflow.test"
	cfg.CredentialsProvider = credentialsProvider
	cfg.HTTPClient = &http.Client{
		Transport: roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
			}, nil
		}),
	}
	cfg.Logging = LoggingOptions{
		Logger:                 logger.NewDevelopmentJSON(&logs),
		Debug:                  true,
		AdvancedDebugTelemetry: true,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	_, err = client.Do(context.Background(), Request{
		Method: http.MethodGet,
		Path:   "/test",
	})
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}

	logOutput := logs.String()
	if !strings.Contains(logOutput, "ecoflow outgoing http request") {
		t.Fatalf("expected outgoing request log, got: %s", logOutput)
	}
	requiredFields := []string{
		"round_trip_latency",
		"request_bytes_sent_estimate",
		"response_bytes_received",
		"request_compression",
		"response_compression",
		"mem_alloc_bytes",
		"cpu_load_pct",
	}
	for _, field := range requiredFields {
		if !strings.Contains(logOutput, field) {
			t.Fatalf("expected field %q in debug logs, got: %s", field, logOutput)
		}
	}
}

func TestClient_Do_DoesNotLogAdvancedFieldsWhenTelemetryDisabled(t *testing.T) {
	t.Parallel()

	credentialsProvider, err := NewStaticCredentialsProvider("ak-test-1234", "sk-test-1234")
	if err != nil {
		t.Fatalf("NewStaticCredentialsProvider() error = %v", err)
	}

	var logs bytes.Buffer
	cfg := DefaultConfig()
	cfg.BaseURL = "https://api.ecoflow.test"
	cfg.CredentialsProvider = credentialsProvider
	cfg.HTTPClient = &http.Client{
		Transport: roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
			}, nil
		}),
	}
	cfg.Logging = LoggingOptions{
		Logger:                 logger.NewDevelopmentJSON(&logs),
		Debug:                  true,
		AdvancedDebugTelemetry: false,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	_, err = client.Do(context.Background(), Request{
		Method: http.MethodGet,
		Path:   "/test",
	})
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}

	logOutput := logs.String()
	disallowedFields := []string{
		"round_trip_latency",
		"request_bytes_sent_estimate",
		"response_bytes_received",
		"request_compression",
		"response_compression",
		"cpu_load_pct",
		"mem_alloc_bytes",
	}
	for _, field := range disallowedFields {
		if strings.Contains(logOutput, field) {
			t.Fatalf("did not expect advanced telemetry field %q in debug logs: %s", field, logOutput)
		}
	}
}

func TestClient_Do_LogsRequestAndResponseHeadersWhenEnabled(t *testing.T) {
	t.Parallel()

	credentialsProvider, err := NewStaticCredentialsProvider("ak-test-1234", "sk-test-1234")
	if err != nil {
		t.Fatalf("NewStaticCredentialsProvider() error = %v", err)
	}

	var logs bytes.Buffer
	cfg := DefaultConfig()
	cfg.BaseURL = "https://api.ecoflow.test"
	cfg.CredentialsProvider = credentialsProvider
	cfg.HTTPClient = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if got := req.Header.Get("X-Custom-Request"); got != "foo" {
				t.Fatalf("X-Custom-Request mismatch: got %q", got)
			}
			header := make(http.Header)
			header.Set("X-Custom-Response", "bar")
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     header,
				Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
			}, nil
		}),
	}
	cfg.Logging = LoggingOptions{
		Logger:                 logger.NewDevelopmentJSON(&logs),
		Debug:                  true,
		AdvancedDebugTelemetry: false,
		DebugLogHeaders:        true,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	_, err = client.Do(context.Background(), Request{
		Method: http.MethodGet,
		Path:   "/test",
		Headers: http.Header{
			"X-Custom-Request": []string{"foo"},
		},
	})
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}

	logOutput := logs.String()
	for _, field := range []string{
		"request_headers",
		"response_headers",
		"X-Custom-Request",
		"X-Custom-Response",
	} {
		if !strings.Contains(logOutput, field) {
			t.Fatalf("expected %q in debug logs, got: %s", field, logOutput)
		}
	}
}

func TestClient_Do_DoesNotLogHeadersWhenDisabled(t *testing.T) {
	t.Parallel()

	credentialsProvider, err := NewStaticCredentialsProvider("ak-test-1234", "sk-test-1234")
	if err != nil {
		t.Fatalf("NewStaticCredentialsProvider() error = %v", err)
	}

	var logs bytes.Buffer
	cfg := DefaultConfig()
	cfg.BaseURL = "https://api.ecoflow.test"
	cfg.CredentialsProvider = credentialsProvider
	cfg.HTTPClient = &http.Client{
		Transport: roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
			header := make(http.Header)
			header.Set("X-Custom-Response", "bar")
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     header,
				Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
			}, nil
		}),
	}
	cfg.Logging = LoggingOptions{
		Logger:                 logger.NewDevelopmentJSON(&logs),
		Debug:                  true,
		AdvancedDebugTelemetry: false,
		DebugLogHeaders:        false,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	_, err = client.Do(context.Background(), Request{
		Method: http.MethodGet,
		Path:   "/test",
		Headers: http.Header{
			"X-Custom-Request": []string{"foo"},
		},
	})
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}

	logOutput := logs.String()
	for _, field := range []string{"request_headers", "response_headers", "X-Custom-Request", "X-Custom-Response"} {
		if strings.Contains(logOutput, field) {
			t.Fatalf("did not expect %q in debug logs: %s", field, logOutput)
		}
	}
}

type roundTripperFunc func(req *http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

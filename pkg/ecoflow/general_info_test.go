package ecoflow

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestGeneralInfoService_ListDevices_DecodesTypedResponse(t *testing.T) {
	t.Parallel()

	credentialsProvider, err := NewStaticCredentialsProvider("ak-test-1234", "sk-test-1234")
	if err != nil {
		t.Fatalf("NewStaticCredentialsProvider() error = %v", err)
	}

	cfg := DefaultConfig()
	cfg.BaseURL = "https://api.ecoflow.test"
	cfg.CredentialsProvider = credentialsProvider
	cfg.HTTPClient = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodGet {
				t.Fatalf("request method mismatch: got %q", req.Method)
			}
			if req.URL.Path != deviceListPath {
				t.Fatalf("request path mismatch: got %q", req.URL.Path)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"code":"0",
					"message":"Success",
					"data":[
						{"sn":"PR12ZA1CDHAW0498","deviceName":"TV Delta 1000 Air","online":1},
						{"sn":"R351ZABAPH331057","deviceName":"Kitchen Delta 2 Max","online":1,"productName":"DELTA 2 Max"}
					]
				}`)),
			}, nil
		}),
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	devices, response, err := client.GeneralInfo().ListDevices(context.Background())
	if err != nil {
		t.Fatalf("ListDevices() error = %v", err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status code mismatch: got %d", response.StatusCode)
	}
	if len(devices) != 2 {
		t.Fatalf("device count mismatch: got %d", len(devices))
	}
	if devices[0].SN != "PR12ZA1CDHAW0498" {
		t.Fatalf("first SN mismatch: got %q", devices[0].SN)
	}
	if devices[0].DeviceName != "TV Delta 1000 Air" {
		t.Fatalf("first device name mismatch: got %q", devices[0].DeviceName)
	}
	if devices[0].Online != 1 {
		t.Fatalf("first online mismatch: got %d", devices[0].Online)
	}
	if devices[1].ProductName != "DELTA 2 Max" {
		t.Fatalf("second product name mismatch: got %q", devices[1].ProductName)
	}
}

func TestGeneralInfoService_GetMQTTCertification_DecodesTypedResponse(t *testing.T) {
	t.Parallel()

	credentialsProvider, err := NewStaticCredentialsProvider("ak-test-1234", "sk-test-1234")
	if err != nil {
		t.Fatalf("NewStaticCredentialsProvider() error = %v", err)
	}

	cfg := DefaultConfig()
	cfg.BaseURL = "https://api.ecoflow.test"
	cfg.CredentialsProvider = credentialsProvider
	cfg.HTTPClient = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodGet {
				t.Fatalf("request method mismatch: got %q", req.Method)
			}
			if req.URL.Path != mqttCertificationPath {
				t.Fatalf("request path mismatch: got %q", req.URL.Path)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"code":"0",
					"message":"Success",
					"data":{
						"certificateAccount":"open-57c134518b5abc",
						"certificatePassword":"959253cc103a4008abc",
						"url":"mqtt.ecoflow.com",
						"port":"8883",
						"protocol":"mqtts"
					}
				}`)),
			}, nil
		}),
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	certification, response, err := client.GeneralInfo().GetMQTTCertification(context.Background())
	if err != nil {
		t.Fatalf("GetMQTTCertification() error = %v", err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status code mismatch: got %d", response.StatusCode)
	}
	if certification.CertificateAccount != "open-57c134518b5abc" {
		t.Fatalf("certificate account mismatch: got %q", certification.CertificateAccount)
	}
	if certification.CertificatePassword != "959253cc103a4008abc" {
		t.Fatalf("certificate password mismatch: got %q", certification.CertificatePassword)
	}
	if certification.URL != "mqtt.ecoflow.com" {
		t.Fatalf("url mismatch: got %q", certification.URL)
	}
	if certification.Port != "8883" {
		t.Fatalf("port mismatch: got %q", certification.Port)
	}
	if certification.Protocol != "mqtts" {
		t.Fatalf("protocol mismatch: got %q", certification.Protocol)
	}
}

func TestGeneralInfoService_GetMQTTCertification_BusinessErrorCodeFails(t *testing.T) {
	t.Parallel()

	credentialsProvider, err := NewStaticCredentialsProvider("ak-test-1234", "sk-test-1234")
	if err != nil {
		t.Fatalf("NewStaticCredentialsProvider() error = %v", err)
	}

	cfg := DefaultConfig()
	cfg.BaseURL = "https://api.ecoflow.test"
	cfg.CredentialsProvider = credentialsProvider
	cfg.HTTPClient = &http.Client{
		Transport: roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"code":"1001",
					"message":"Signature invalid",
					"data":{}
				}`)),
			}, nil
		}),
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	_, _, err = client.GeneralInfo().GetMQTTCertification(context.Background())
	if err == nil {
		t.Fatal("expected business error for non-zero code, got nil")
	}
	if !strings.Contains(err.Error(), "code=1001") {
		t.Fatalf("expected code in error, got: %v", err)
	}
}

func TestGeneralInfoService_GetDeviceAllQuota_DecodesMapResponse(t *testing.T) {
	t.Parallel()

	credentialsProvider, err := NewStaticCredentialsProvider("ak-test-1234", "sk-test-1234")
	if err != nil {
		t.Fatalf("NewStaticCredentialsProvider() error = %v", err)
	}

	cfg := DefaultConfig()
	cfg.BaseURL = "https://api.ecoflow.test"
	cfg.CredentialsProvider = credentialsProvider
	cfg.HTTPClient = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodGet {
				t.Fatalf("request method mismatch: got %q", req.Method)
			}
			if req.URL.Path != deviceQuotaAllPath {
				t.Fatalf("request path mismatch: got %q", req.URL.Path)
			}
			if got := req.URL.Query().Get("sn"); got != "Y711ZABA9H2P0294" {
				t.Fatalf("sn query mismatch: got %q", got)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
						"code":"0",
						"message":"Success",
						"data":{
							"bmsMaster.soc":100,
							"bmsMaster.temp":34,
							"inv.cfgAcEnabled":"0"
						}
					}`)),
			}, nil
		}),
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	quota, response, err := client.GeneralInfo().GetDeviceAllQuota(context.Background(), "Y711ZABA9H2P0294")
	if err != nil {
		t.Fatalf("GetDeviceAllQuota() error = %v", err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status code mismatch: got %d", response.StatusCode)
	}
	if len(quota) != 3 {
		t.Fatalf("quota map length mismatch: got %d", len(quota))
	}
	if quota["bmsMaster.soc"] != "100" {
		t.Fatalf("quota bmsMaster.soc mismatch: got %q", quota["bmsMaster.soc"])
	}
	if quota["bmsMaster.temp"] != "34" {
		t.Fatalf("quota bmsMaster.temp mismatch: got %q", quota["bmsMaster.temp"])
	}
}

func TestGeneralInfoService_GetDeviceAllQuota_RequiresSN(t *testing.T) {
	t.Parallel()

	credentialsProvider, err := NewStaticCredentialsProvider("ak-test-1234", "sk-test-1234")
	if err != nil {
		t.Fatalf("NewStaticCredentialsProvider() error = %v", err)
	}

	cfg := DefaultConfig()
	cfg.BaseURL = "https://api.ecoflow.test"
	cfg.CredentialsProvider = credentialsProvider
	cfg.HTTPClient = &http.Client{
		Transport: roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
			t.Fatal("transport should not be called when sn is empty")
			return nil, nil
		}),
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	_, _, err = client.GeneralInfo().GetDeviceAllQuota(context.Background(), "   ")
	if err == nil {
		t.Fatal("expected error for empty sn, got nil")
	}
	if !strings.Contains(err.Error(), "sn is required") {
		t.Fatalf("expected sn required error, got: %v", err)
	}
}

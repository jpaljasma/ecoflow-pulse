package pecron

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientLoginFallsBackToJWTClaimsForMQTTIdentity(t *testing.T) {
	t.Parallel()

	expiresAt := time.Date(2026, time.May, 8, 18, 30, 0, 0, time.UTC)
	token := testJWT(map[string]any{
		"uid": "user-from-jwt",
		"exp": expiresAt.Unix(),
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/enduser/enduserapi/emailPwdLogin" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Fatalf("content type = %q", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("appId") != appID || r.Header.Get("appSystemType") != appSystemType {
			t.Fatalf("missing Pecron app headers: %#v", r.Header)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 200,
			"data": map[string]any{
				"accessToken": map[string]any{"token": token},
				"refreshToken": map[string]any{
					"token": "refresh-token",
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(RegionConfig{BaseURL: server.URL, UserDomainSecret: "secret"}, server.Client())
	session, err := client.Login(context.Background(), "owner@example.test", "battery-staple")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if session.UserID != "user-from-jwt" {
		t.Fatalf("user id = %q, want JWT uid", session.UserID)
	}
	if !session.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("expires at = %s, want %s", session.ExpiresAt, expiresAt)
	}
}

func TestClientProductTSLParsesTSLJSON(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/binding/enduserapi/productTSL" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if r.URL.Query().Get("pk") != ProductKeyE1000LFP {
			t.Fatalf("pk query = %q", r.URL.Query().Get("pk"))
		}
		if r.Header.Get("Authorization") != "token" {
			t.Fatalf("authorization header = %q", r.Header.Get("Authorization"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 200,
			"data": map[string]any{
				"tslJson": `{"properties":[{"code":"battery_percentage","name":"Battery","dataType":{"type":"int"},"accessMode":"R"},{"code":"ac_switch_hm","name":"AC Switch","dataType":"bool","accessMode":"RW"}]}`,
			},
		})
	}))
	defer server.Close()

	client := NewClient(RegionConfig{BaseURL: server.URL}, server.Client())
	properties, err := client.ProductTSL(context.Background(), Session{AccessToken: "token"}, ProductKeyE1000LFP)
	if err != nil {
		t.Fatalf("ProductTSL() error = %v", err)
	}
	if len(properties) != 2 {
		t.Fatalf("properties = %d, want 2", len(properties))
	}
	if properties[0].Code != "battery_percentage" || properties[0].Writable || properties[0].DataType != "int" {
		t.Fatalf("first property = %+v", properties[0])
	}
	if properties[1].Code != "ac_switch_hm" || !properties[1].Writable || properties[1].DataType != "bool" {
		t.Fatalf("second property = %+v", properties[1])
	}
}

func TestClientDeviceKVParsesCurrentAttributes(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/binding/enduserapi/getDeviceBusinessAttributes" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if r.URL.Query().Get("pk") != ProductKeyE1000LFP || r.URL.Query().Get("dk") != "device-key" {
			t.Fatalf("query = %s", r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 200,
			"data": map[string]any{
				"customizeTslInfo": []map[string]any{
					{"resourceCode": "battery_percentage", "resourceValce": "66", "dataType": "int"},
					{"resourceCode": "dc_data_input_hm", "resourceValce": `{"gx16mf1_input_power":144}`, "dataType": "struct"},
					{"resourceCode": "ac_switch_hm", "resourceValce": "true", "dataType": "bool"},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(RegionConfig{BaseURL: server.URL}, server.Client())
	kv, err := client.DeviceKV(context.Background(), Session{AccessToken: "token"}, DeviceRef{
		ProductKey: ProductKeyE1000LFP,
		DeviceKey:  "device-key",
	})
	if err != nil {
		t.Fatalf("DeviceKV() error = %v", err)
	}
	if kv["battery_percentage"] != float64(66) {
		t.Fatalf("battery percentage = %#v", kv["battery_percentage"])
	}
	if nested := asMap(kv["dc_data_input_hm"]); nested["gx16mf1_input_power"] != float64(144) {
		t.Fatalf("nested DC input = %#v", nested)
	}
	if kv["ac_switch_hm"] != true {
		t.Fatalf("ac switch = %#v", kv["ac_switch_hm"])
	}
}

func TestClientDeviceKVHandlesCloudAttributeShapeDrift(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 200,
			"data": map[string]any{
				"customizeTslInfo": []map[string]any{
					{"resourceCode": "empty_value", "resourceValce": "", "dataType": "STRING"},
					{"resourceCode": "battery_percentage", "value": "91", "dataType": map[string]any{"type": "INT"}},
					{"resourceCode": "host_packet_voltage", "resourceValce": "", "resourceValue": "53.2", "dataType": "FLOAT"},
					{"resourceCode": "dc_data_input_hm", "resourceValue": `{"dc_input_power":"0","gx16mf1_input_power":"41"}`, "dataType": "STRUCT"},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(RegionConfig{BaseURL: server.URL}, server.Client())
	kv, err := client.DeviceKV(context.Background(), Session{AccessToken: "token"}, DeviceRef{
		ProductKey: ProductKeyE1000LFP,
		DeviceKey:  "device-key",
	})
	if err != nil {
		t.Fatalf("DeviceKV() error = %v", err)
	}
	if _, ok := kv["empty_value"]; ok {
		t.Fatalf("empty resource values should be skipped: %#v", kv)
	}
	if got := kv["battery_percentage"]; got != float64(91) {
		t.Fatalf("battery percentage = %#v, want 91", got)
	}
	if got := kv["host_packet_voltage"]; got != 53.2 {
		t.Fatalf("host packet voltage = %#v, want 53.2", got)
	}
	dcInput := asMap(kv["dc_data_input_hm"])
	if dcInput["dc_input_power"] != "0" || dcInput["gx16mf1_input_power"] != "41" {
		t.Fatalf("dc input struct = %#v", dcInput)
	}
}

func TestClientSurfacesPecronRateLimitAsTypedAPIError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"type": "BUSI-ERROR",
			"code": 4026,
			"msg":  "Insufficient resources in the manufacturer's account. Please contact the device manufacturer.",
		})
	}))
	defer server.Close()

	client := NewClient(RegionConfig{BaseURL: server.URL}, server.Client())
	_, err := client.DeviceKV(context.Background(), Session{AccessToken: "token"}, DeviceRef{
		ProductKey: ProductKeyE1000LFP,
		DeviceKey:  "device-key",
	})
	if err == nil {
		t.Fatal("expected Pecron API error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error %T %[1]v should wrap APIError", err)
	}
	if apiErr.Code != "4026" || !apiErr.RateLimited() {
		t.Fatalf("api error = %+v, want code 4026 rate-limited", apiErr)
	}
}

func TestSessionRefreshPolicyUsesReloginWhenExpired(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.May, 8, 12, 0, 0, 0, time.UTC)
	if (Session{AccessToken: "token", ExpiresAt: now.Add(10 * time.Minute)}).NeedsRefresh(now, 5*time.Minute) {
		t.Fatalf("valid token should not need refresh")
	}
	if !(Session{AccessToken: "token", ExpiresAt: now.Add(2 * time.Minute)}).NeedsRefresh(now, 5*time.Minute) {
		t.Fatalf("near-expiry token should need refresh")
	}
	if !(Session{}).NeedsRefresh(now, 5*time.Minute) {
		t.Fatalf("empty token should need refresh")
	}
}

func testJWT(claims map[string]any) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		panic(err)
	}
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	return strings.Join([]string{header, payload, "signature"}, ".")
}

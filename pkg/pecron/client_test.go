package pecron

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

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
	if properties[0].Code != "battery_percentage" || properties[0].Writable {
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

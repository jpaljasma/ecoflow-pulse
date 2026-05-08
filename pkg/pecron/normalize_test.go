package pecron

import "testing"

func TestNormalizeE1000LFPTelemetryMapsCloudKVToCanonicalParams(t *testing.T) {
	t.Parallel()

	out := NormalizeTelemetry(Device{
		ProductKey:  ProductKeyE1000LFP,
		DeviceKey:   "aabbccddeeff",
		ProductName: "E1000LFP",
	}, map[string]any{
		"battery_percentage":    67,
		"total_input_power":     "184",
		"total_output_power":    float64(92),
		"remain_time":           245,
		"remain_charging_time":  180,
		"ac_switch_hm":          true,
		"dc_switch_hm":          false,
		"ups_status_hm":         true,
		"host_packet_data_jdb":  map[string]any{"host_packet_voltage": 51.8, "host_packet_current": -1.7, "host_packet_temp": 28},
		"ac_data_input_hm":      map[string]any{"ac_input_power": 0},
		"ac_data_output_hm":     map[string]any{"ac_output_power": 88, "ac_output_voltage": 120.1},
		"dc_data_input_hm":      map[string]any{"gx16mf1_input_voltage": 31.2, "gx16mf1_input_current": 5.9, "gx16mf1_input_power": 184},
		"dc_data_output_hm":     map[string]any{"dc_output_power": 4},
		"device_status_hm":      1,
		"high_frequency_ignore": "left alone",
	})

	if got := out.Params["soc"]; got != float64(67) {
		t.Fatalf("soc = %#v, want 67", got)
	}
	if got := out.Params["wattsInSum"]; got != float64(184) {
		t.Fatalf("wattsInSum = %#v, want 184", got)
	}
	if got := out.Params["wattsOutSum"]; got != float64(92) {
		t.Fatalf("wattsOutSum = %#v, want 92", got)
	}
	if got := out.Params["batVol"]; got != float64(51.8) {
		t.Fatalf("batVol = %#v, want 51.8", got)
	}
	if got := out.Params["batAmp"]; got != float64(-1.7) {
		t.Fatalf("batAmp = %#v, want -1.7", got)
	}
	if got := out.Params["pv1ChargeWatts"]; got != float64(184) {
		t.Fatalf("pv1ChargeWatts = %#v, want 184", got)
	}
	if got := out.Params["pv1InVol"]; got != float64(31.2) {
		t.Fatalf("pv1InVol = %#v, want 31.2", got)
	}
	if got := out.Params["pv1InAmp"]; got != float64(5.9) {
		t.Fatalf("pv1InAmp = %#v, want 5.9", got)
	}
	if got := out.Capabilities["pv_input_count"]; got != 1 {
		t.Fatalf("pv_input_count = %#v, want 1", got)
	}
	if got := out.Capabilities["battery_capacity_wh"]; got != 1024 {
		t.Fatalf("battery_capacity_wh = %#v, want 1024", got)
	}
	if got := out.Metadata["product_key"]; got != ProductKeyE1000LFP {
		t.Fatalf("product_key metadata = %#v", got)
	}
}

func TestNormalizeTelemetryHandlesKnownCloudQuirks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		kv         map[string]any
		wantParams map[string]any
		absent     []string
	}{
		{
			name: "host packet SOC refreshes standalone E1000 when top-level SOC is stale",
			kv: map[string]any{
				"battery_percentage":   100,
				"host_packet_data_jdb": map[string]any{"host_packet_electric_percentage": 82},
			},
			wantParams: map[string]any{"soc": float64(82), "f32ShowSoc": float64(82)},
		},
		{
			name: "total input and output fall back to component sums when top-level totals are absent",
			kv: map[string]any{
				"ac_data_input_hm":  map[string]any{"ac_input_power": 150},
				"dc_data_input_hm":  map[string]any{"dc_input_power": 20},
				"ac_data_output_hm": map[string]any{"ac_output_power": 100},
				"dc_data_output_hm": map[string]any{"dc_output_power": 30},
			},
			wantParams: map[string]any{"wattsInSum": float64(170), "wattsOutSum": float64(130)},
		},
		{
			name: "component zero totals are emitted when idle components are observed",
			kv: map[string]any{
				"ac_data_input_hm": map[string]any{"ac_input_power": 0},
				"dc_data_input_hm": map[string]any{"dc_input_power": 0},
			},
			wantParams: map[string]any{"wattsInSum": float64(0)},
			absent:     []string{"wattsOutSum"},
		},
		{
			name: "missing totals and components do not invent idle zeros",
			kv: map[string]any{
				"host_packet_data_jdb": map[string]any{"host_packet_temp": 24},
			},
			absent: []string{"wattsInSum", "wattsOutSum"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out := NormalizeTelemetry(Device{
				ProductKey:  ProductKeyE1000LFP,
				DeviceKey:   "aabbccddeeff",
				ProductName: "E1000LFP",
			}, tt.kv)
			for key, want := range tt.wantParams {
				if got := out.Params[key]; got != want {
					t.Fatalf("%s = %#v, want %#v (params=%#v)", key, got, want, out.Params)
				}
			}
			for _, key := range tt.absent {
				if _, ok := out.Params[key]; ok {
					t.Fatalf("%s should be absent from params %#v", key, out.Params)
				}
			}
		})
	}
}

func TestMergeMQTTKVAccumulatesPartialPackets(t *testing.T) {
	t.Parallel()

	state := map[string]any{
		"battery_percentage": 61,
	}
	merged := MergeKV(state, map[string]any{
		"total_input_power": 88,
		"dc_data_input_hm":  map[string]any{"gx16mf1_input_power": 88},
	})

	if got := merged["battery_percentage"]; got != 61 {
		t.Fatalf("battery percentage was not retained: %#v", merged)
	}
	if got := merged["total_input_power"]; got != 88 {
		t.Fatalf("total input not merged: %#v", merged)
	}
	nested := merged["dc_data_input_hm"].(map[string]any)
	if got := nested["gx16mf1_input_power"]; got != 88 {
		t.Fatalf("nested input not merged: %#v", nested)
	}
}

func TestMergeMQTTKVPreservesLastGoodVoltageAcrossPlaceholderPackets(t *testing.T) {
	t.Parallel()

	state := map[string]any{
		"host_packet_data_jdb": map[string]any{
			"host_packet_voltage": 53.1,
			"host_packet_temp":    28,
		},
	}
	merged := MergeKV(state, map[string]any{
		"host_packet_data_jdb": map[string]any{
			"host_packet_voltage": 0,
			"host_packet_temp":    29,
		},
	})

	host := asMap(merged["host_packet_data_jdb"])
	if got := host["host_packet_voltage"]; got != 53.1 {
		t.Fatalf("host_packet_voltage = %#v, want last-good 53.1", got)
	}
	if got := host["host_packet_temp"]; got != 29 {
		t.Fatalf("host_packet_temp = %#v, want fresh temp 29", got)
	}
}

func TestDecodeMQTTBusPayloadExtractsKV(t *testing.T) {
	t.Parallel()

	msg, err := DecodeMQTTBusPayload([]byte(`{"deviceKey":"dk1","data":{"kv":{"battery_percentage":72}}}`))
	if err != nil {
		t.Fatalf("DecodeMQTTBusPayload() error = %v", err)
	}
	if msg.DeviceKey != "dk1" {
		t.Fatalf("device key = %q", msg.DeviceKey)
	}
	if got := msg.KV["battery_percentage"]; got != float64(72) {
		t.Fatalf("kv battery = %#v", got)
	}
}

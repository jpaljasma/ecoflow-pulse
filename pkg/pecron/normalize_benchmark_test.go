package pecron

import "testing"

func BenchmarkDecodeMQTTBusPayload(b *testing.B) {
	payload := benchmarkPecronPayload()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		msg, err := DecodeMQTTBusPayload(payload)
		if err != nil {
			b.Fatalf("DecodeMQTTBusPayload() error = %v", err)
		}
		if len(msg.KV) == 0 {
			b.Fatal("empty KV")
		}
	}
}

func BenchmarkMergeKV(b *testing.B) {
	base := benchmarkPecronBaseKV()
	next := benchmarkPecronNextKV()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		merged := MergeKV(base, next)
		if len(merged) == 0 {
			b.Fatal("empty merge")
		}
	}
}

func BenchmarkNormalizeTelemetry(b *testing.B) {
	device := benchmarkPecronDevice()
	kv := benchmarkPecronBaseKV()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		normalized := NormalizeTelemetry(device, kv)
		if len(normalized.Params) == 0 {
			b.Fatal("empty params")
		}
	}
}

func BenchmarkDecodeMergeNormalizeTelemetry(b *testing.B) {
	device := benchmarkPecronDevice()
	payload := benchmarkPecronPayload()
	state := map[string]any{}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		msg, err := DecodeMQTTBusPayload(payload)
		if err != nil {
			b.Fatalf("DecodeMQTTBusPayload() error = %v", err)
		}
		state = MergeKV(state, msg.KV)
		normalized := NormalizeTelemetry(device, state)
		if len(normalized.Params) == 0 {
			b.Fatal("empty params")
		}
	}
}

func benchmarkPecronDevice() Device {
	return Device{
		ProductKey:  ProductKeyE1000LFP,
		DeviceKey:   "aabbccddeeff",
		ProductName: "E1000LFP",
	}
}

func benchmarkPecronBaseKV() map[string]any {
	return map[string]any{
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
		"dc_data_input_hm":      map[string]any{"dc_input_power": 184, "gx16mf1_input_voltage": 31.2, "gx16mf1_input_current": 5.9},
		"dc_data_output_hm":     map[string]any{"dc_output_power": 4},
		"device_status_hm":      1,
		"high_frequency_ignore": "left alone",
	}
}

func benchmarkPecronNextKV() map[string]any {
	return map[string]any{
		"battery_percentage": 68,
		"total_input_power":  "190",
		"host_packet_data_jdb": map[string]any{
			"host_packet_voltage": 52.1,
			"host_packet_current": -1.4,
			"host_packet_temp":    29,
		},
		"dc_data_input_hm": map[string]any{
			"dc_input_power":        190,
			"gx16mf1_input_voltage": 32.0,
			"gx16mf1_input_current": 5.95,
		},
	}
}

func benchmarkPecronPayload() []byte {
	return []byte(`{
		"deviceKey":"aabbccddeeff",
		"data":{"kv":{
			"battery_percentage":68,
			"total_input_power":"190",
			"total_output_power":93,
			"remain_time":244,
			"remain_charging_time":179,
			"ac_switch_hm":true,
			"dc_switch_hm":false,
			"ups_status_hm":true,
			"host_packet_data_jdb":{"host_packet_voltage":52.1,"host_packet_current":-1.4,"host_packet_temp":29},
			"ac_data_input_hm":{"ac_input_power":0},
			"ac_data_output_hm":{"ac_output_power":89,"ac_output_voltage":120.2},
			"dc_data_input_hm":{"dc_input_power":190,"gx16mf1_input_voltage":32.0,"gx16mf1_input_current":5.95},
			"dc_data_output_hm":{"dc_output_power":4},
			"device_status_hm":1,
			"high_frequency_ignore":"left alone"
		}}
	}`)
}

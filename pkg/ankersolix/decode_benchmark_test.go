package ankersolix

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func BenchmarkDecodeMQTTWrapper(b *testing.B) {
	frame := binaryFrame(
		"0421",
		tlvString("a2", "SN-C2000"),
		tlvNumber("a5", typeUI, 87),
		tlvNumber("a6", typeSILE, 612),
		tlvNumber("a7", typeSILE, 250),
		tlvNumber("b2", typeSILE, 52),
	)
	wrapped := benchmarkAnkerMQTTWrapper(b, "A1783", "SN-C2000", "data", frame)
	topic := "dt/anker_power/A1783/SN-C2000/param_info"

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		msg, err := DecodeMQTTWrapper(topic, wrapped)
		if err != nil {
			b.Fatalf("DecodeMQTTWrapper() error = %v", err)
		}
		if len(msg.Values) == 0 {
			b.Fatal("empty values")
		}
	}
}

func BenchmarkDecodeNormalizeBinaryPayload(b *testing.B) {
	frame := binaryFrame(
		"0421",
		tlvString("a2", "SN-C2000"),
		tlvNumber("a5", typeUI, 87),
		tlvNumber("a6", typeSILE, 612),
		tlvNumber("a7", typeSILE, 250),
		tlvNumber("b2", typeSILE, 52),
	)
	ref := DeviceRef{ProductCode: "A1783", DeviceSN: "SN-C2000"}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		decoded, err := DecodeBinaryPayload(ref.ProductCode, frame)
		if err != nil {
			b.Fatalf("DecodeBinaryPayload() error = %v", err)
		}
		normalized := NormalizeTelemetry(ref, decoded.Values)
		if len(normalized.Params) == 0 {
			b.Fatal("empty params")
		}
	}
}

func BenchmarkDecodeNormalizeJSONPayload(b *testing.B) {
	payload := []byte(`{"soc":62,"pp":650,"gp":3705,"g2lp":1409,"lp":1409,"bds":[{"sn":"module-1","soc":61,"power":-1148},{"sn":"module-2","soc":63,"power":-1148}]}`)
	ref := DeviceRef{ProductCode: "A5101", DeviceSN: "SN-X1"}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		decoded, err := DecodeJSONPayload(ref.ProductCode, payload)
		if err != nil {
			b.Fatalf("DecodeJSONPayload() error = %v", err)
		}
		normalized := NormalizeTelemetry(ref, decoded.Values)
		if len(normalized.Params) == 0 {
			b.Fatal("empty params")
		}
	}
}

func benchmarkAnkerMQTTWrapper(tb testing.TB, pn, sn, field string, payload []byte) []byte {
	tb.Helper()
	inner := map[string]any{
		"pn":  pn,
		"sn":  sn,
		field: base64.StdEncoding.EncodeToString(payload),
	}
	innerJSON, err := json.Marshal(inner)
	if err != nil {
		tb.Fatal(err)
	}
	outer := map[string]any{
		"head": map[string]any{
			"timestamp": 1700000000,
			"device_pn": pn,
			"device_sn": sn,
		},
		"payload": string(innerJSON),
	}
	raw, err := json.Marshal(outer)
	if err != nil {
		tb.Fatal(err)
	}
	return raw
}

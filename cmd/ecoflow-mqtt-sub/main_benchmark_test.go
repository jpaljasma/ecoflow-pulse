package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

const (
	replayLogPath         = "logs/mqtt.log"
	replayPayloadLinePref = "payload_raw="
	replayPayloadMaxLines = 4096
)

var (
	replayPayloadsOnce sync.Once
	replayPayloads     [][]byte
)

func BenchmarkTelemetryReplayLegacy(b *testing.B) {
	benchmarkTelemetryReplay(b, parseTelemetryPayloadLegacy, true)
}

func BenchmarkTelemetryReplayOptimized(b *testing.B) {
	benchmarkTelemetryReplay(b, parseTelemetryPayload, false)
}

func BenchmarkParseTelemetryPayloadLegacy(b *testing.B) {
	benchmarkTelemetryParseOnly(b, parseTelemetryPayloadLegacy)
}

func BenchmarkParseTelemetryPayloadOptimized(b *testing.B) {
	benchmarkTelemetryParseOnly(b, parseTelemetryPayload)
}

func BenchmarkTelemetryUpdateOnlyOptimized(b *testing.B) {
	payloads := loadReplayPayloads(b)
	prepared := make([]preparedTelemetry, 0, len(payloads))
	for i, payload := range payloads {
		envelope, quota, err := parseTelemetryPayload(payload)
		if err != nil {
			b.Fatalf("prepare parse payload at %d: %v", i, err)
		}
		kitEntries, hasKit := extractKitInfoWatts(quota)
		pdStatus := pdStatusSummary{}
		hasPDStatus := false
		if isPDStatusEnvelope(envelope) {
			pdStatus, hasPDStatus = extractPDStatus(quota)
		}
		prepared = append(prepared, preparedTelemetry{
			envelope:   envelope,
			quota:      quota,
			kitEntries: kitEntries,
			hasKit:     hasKit,
			pdStatus:   pdStatus,
			hasPD:      hasPDStatus,
		})
	}
	if len(prepared) == 0 {
		b.Fatalf("no prepared telemetry payloads")
	}

	snapshot := newEnergySnapshot()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		item := prepared[i%len(prepared)]
		snapshot.Update(item.envelope, item.quota, item.kitEntries, item.hasKit, item.pdStatus, item.hasPD)
	}
}

func benchmarkTelemetryParseOnly(
	b *testing.B,
	parseFn func([]byte) (telemetryEnvelope, map[string]any, error),
) {
	payloads := loadReplayPayloads(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		payload := payloads[i%len(payloads)]
		if _, _, err := parseFn(payload); err != nil {
			b.Fatalf("parse payload at %d: %v", i, err)
		}
	}
}

func benchmarkTelemetryReplay(
	b *testing.B,
	parseFn func([]byte) (telemetryEnvelope, map[string]any, error),
	legacy bool,
) {
	payloads := loadReplayPayloads(b)
	snapshot := newEnergySnapshot()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		payload := payloads[i%len(payloads)]
		envelope, quota, err := parseFn(payload)
		if err != nil {
			b.Fatalf("parse payload at %d: %v", i, err)
		}

		if legacy {
			_ = extractBatterySOC(quota)
			_ = extractPVInput(quota)
		}

		kitEntries, hasKit := extractKitInfoWatts(quota)
		pdStatus := pdStatusSummary{}
		hasPDStatus := false
		if isPDStatusEnvelope(envelope) {
			pdStatus, hasPDStatus = extractPDStatus(quota)
		}

		snapshot.Update(envelope, quota, kitEntries, hasKit, pdStatus, hasPDStatus)
	}
}

type preparedTelemetry struct {
	envelope   telemetryEnvelope
	quota      map[string]any
	kitEntries []kitInfoWattsEntry
	hasKit     bool
	pdStatus   pdStatusSummary
	hasPD      bool
}

func parseTelemetryPayloadLegacy(payload []byte) (telemetryEnvelope, map[string]any, error) {
	var root any
	if err := json.Unmarshal(payload, &root); err != nil {
		return telemetryEnvelope{}, nil, err
	}

	object, ok := root.(map[string]any)
	if !ok {
		return telemetryEnvelope{}, nil, errors.New("payload is not a JSON object")
	}

	envelope := telemetryEnvelope{
		ModuleType: toInt64(object["moduleType"]),
		NeedAck:    toInt64(object["needAck"]),
		ID:         toInt64(object["id"]),
		Time:       toInt64(object["time"]),
		CmdID:      toInt64(object["cmdId"]),
		CmdFunc:    toInt64(object["cmdFunc"]),
		Addr:       toString(object["addr"]),
		Version:    toString(object["version"]),
		TypeCode:   toString(object["typeCode"]),
	}

	params := mapFromAny(object["params"])
	param := mapFromAny(object["param"])
	data := mapFromAny(object["data"])
	quota := mapFromAny(object["quota"])

	merged := make(map[string]any, len(params)+len(param)+len(data)+len(quota))
	mergeAnyMap(merged, params)
	mergeAnyMap(merged, param)
	mergeAnyMap(merged, data)
	mergeAnyMap(merged, quota)

	if envelope.Addr != "" {
		baseKeys := make([]string, 0, len(merged))
		for key := range merged {
			baseKeys = append(baseKeys, key)
		}
		for _, key := range baseKeys {
			if strings.HasPrefix(key, envelope.Addr+".") {
				continue
			}
			prefixed := envelope.Addr + "." + key
			if _, exists := merged[prefixed]; !exists {
				merged[prefixed] = merged[key]
			}
		}
	}

	if envelope.TypeCode == "" {
		envelope.TypeCode = inferTypeCode(envelope.Addr, merged)
	}

	if len(merged) == 0 {
		return envelope, map[string]any{}, nil
	}
	return envelope, merged, nil
}

func loadReplayPayloads(tb testing.TB) [][]byte {
	tb.Helper()

	replayPayloadsOnce.Do(func() {
		if payloads, err := readReplayPayloadsFromLog(replayLogPath, replayPayloadMaxLines); err == nil && len(payloads) > 0 {
			replayPayloads = payloads
			return
		}
		replayPayloads = replayFallbackPayloads()
	})

	if len(replayPayloads) == 0 {
		tb.Fatalf("no replay payloads available")
	}
	return replayPayloads
}

func readReplayPayloadsFromLog(path string, limit int) ([][]byte, error) {
	path = filepath.Clean(path)
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), 1024*1024)

	out := make([][]byte, 0, 256)
	for scanner.Scan() {
		line := scanner.Text()
		idx := strings.Index(line, replayPayloadLinePref)
		if idx < 0 {
			continue
		}
		raw := strings.TrimSpace(line[idx+len(replayPayloadLinePref):])
		if raw == "" {
			continue
		}
		if !strings.HasPrefix(raw, "{") || !strings.HasSuffix(raw, "}") {
			continue
		}
		out = append(out, []byte(raw))
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func replayFallbackPayloads() [][]byte {
	return [][]byte{
		[]byte(`{"moduleType":1,"needAck":0,"id":8222869,"time":17105352,"params":{"wattsOutSum":136,"remainTime":444,"invOutWatts":136,"icoBytes":[0,8,8,0,128,0,0,0,0,0,0,0,0,0]},"version":"1.0","typeCode":"pdStatus"}`),
		[]byte(`{"moduleType":2,"needAck":1,"id":38553491,"time":17106192,"params":{"watts":[{"appState":0,"curPower":0,"appVer":0,"f32Soc":0,"soc":0,"avaFlag":0,"sn":"","detail":0,"type":0,"loadVer":0},{"appState":1,"curPower":-100,"appVer":33620275,"f32Soc":26.25,"soc":26,"avaFlag":1,"sn":"R361Z1BAPH2K1398","detail":4,"type":81,"loadVer":33619974}]},"version":"1.0","typeCode":"kitInfo"}`),
		[]byte(`{"cmdId":2,"cmdFunc":2,"addr":"hs_yj751_pd_backend_addr","params":{"fanState":0,"mpptHvTemp":25.0,"mpptLvTemp":25.0,"pcsAcTemp":26.0,"pcsDcTemp":25.0,"pdTemp":24,"inLvMpptVol":33.306152,"inLvMpptAmp":1.1429396,"bmsOutputWatts":84.0},"version":"1.0"}`),
	}
}

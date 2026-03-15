package energydashboard

import (
	"encoding/json"
	"math"
	"strings"
	"time"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	"google.golang.org/protobuf/proto"
)

const quotaPayloadType = "ecoflow.quota.normalized"

type PVPortHistory struct {
	DeviceID          string
	PortID            string
	PortLabel         string
	MaxObservedVolts  float64
	MaxObservedAmps   float64
	MaxObservedWatts  float64
	LastObservedVolts float64
	LastObservedAmps  float64
	LastObservedWatts float64
	LastObservedAt    time.Time
	SampleCount       int
}

func SummarizePVPortHistory(envelopes []*envelopev1.TelemetryEnvelope) []PVPortHistory {
	ports := map[string]PVPortHistory{}
	for _, env := range envelopes {
		observations, observedAt, ok := extractPVPortObservations(env)
		if !ok {
			continue
		}
		mergePVPortObservations(ports, observations, observedAt, 1)
	}
	return rowsFromPVPorts(ports)
}

func SummarizePVPortHistoryFrames(frames [][]byte, keep func(*envelopev1.TelemetryEnvelope) bool) ([]PVPortHistory, error) {
	ports := map[string]PVPortHistory{}
	for _, frame := range frames {
		var env envelopev1.TelemetryEnvelope
		if err := proto.Unmarshal(frame, &env); err != nil {
			return nil, err
		}
		if keep != nil && !keep(&env) {
			continue
		}
		observations, observedAt, ok := extractPVPortObservations(&env)
		if !ok {
			continue
		}
		mergePVPortObservations(ports, observations, observedAt, 1)
	}
	return rowsFromPVPorts(ports), nil
}

func MergePVPortHistorySets(sets ...[]PVPortHistory) []PVPortHistory {
	ports := map[string]PVPortHistory{}
	for _, rows := range sets {
		for _, row := range rows {
			mergePVPortRow(ports, row, row.LastObservedAt, row.SampleCount)
		}
	}
	return rowsFromPVPorts(ports)
}

func mergePVPortObservations(ports map[string]PVPortHistory, observations []PVPortHistory, observedAt time.Time, sampleCount int) {
	for _, observation := range observations {
		mergePVPortRow(ports, observation, observedAt, sampleCount)
	}
}

func mergePVPortRow(ports map[string]PVPortHistory, observation PVPortHistory, observedAt time.Time, sampleCount int) {
	key := observation.DeviceID + "|" + observation.PortID
	current, found := ports[key]
	if !found {
		current = observation
		current.LastObservedAt = observedAt
		current.SampleCount = 0
	}
	current.MaxObservedVolts = math.Max(current.MaxObservedVolts, observation.LastObservedVolts)
	current.MaxObservedAmps = math.Max(current.MaxObservedAmps, observation.LastObservedAmps)
	current.MaxObservedWatts = math.Max(current.MaxObservedWatts, observation.LastObservedWatts)
	if current.LastObservedAt.IsZero() || observedAt.After(current.LastObservedAt) || observedAt.Equal(current.LastObservedAt) {
		current.LastObservedVolts = observation.LastObservedVolts
		current.LastObservedAmps = observation.LastObservedAmps
		current.LastObservedWatts = observation.LastObservedWatts
		current.LastObservedAt = observedAt
	}
	current.SampleCount += sampleCount
	ports[key] = current
}

func rowsFromPVPorts(ports map[string]PVPortHistory) []PVPortHistory {
	out := make([]PVPortHistory, 0, len(ports))
	for _, row := range ports {
		out = append(out, row)
	}
	return out
}

func extractPVPortObservations(env *envelopev1.TelemetryEnvelope) ([]PVPortHistory, time.Time, bool) {
	if env == nil || env.GetPayloadType() != quotaPayloadType || len(env.GetPayload()) == 0 {
		return nil, time.Time{}, false
	}

	var payload struct {
		Params map[string]any `json:"params"`
	}
	if err := json.Unmarshal(env.GetPayload(), &payload); err != nil || len(payload.Params) == 0 {
		return nil, time.Time{}, false
	}

	observedAt := envelopeObservedAt(env)
	deviceID := strings.TrimSpace(env.GetDeviceId())
	if deviceID == "" {
		return nil, time.Time{}, false
	}
	observations := make([]PVPortHistory, 0, 2)
	if low, ok := extractPVPortObservation(deviceID, "pv-low", "PV Low", payload.Params, "inLvMpptVol", "inLvMpptAmp", "pv1ChargeWatts", "inLvMpptPwr"); ok {
		observations = append(observations, low)
	}
	if high, ok := extractPVPortObservation(deviceID, "pv-high", "PV High", payload.Params, "inHvMpptVol", "inHvMpptAmp", "pv2ChargeWatts", "inHvMpptPwr"); ok {
		observations = append(observations, high)
	}
	if len(observations) == 0 {
		return nil, time.Time{}, false
	}
	return observations, observedAt, true
}

func extractPVPortObservation(deviceID, portID, label string, params map[string]any, voltsKey, ampsKey, wattsKey, fallbackWattsKey string) (PVPortHistory, bool) {
	volts, hasVolts := numberFromMap(params, voltsKey)
	amps, hasAmps := numberFromMap(params, ampsKey)
	watts, hasWatts := numberFromMap(params, wattsKey)
	if !hasWatts {
		watts, hasWatts = numberFromMap(params, fallbackWattsKey)
	}
	if !hasWatts && hasVolts && hasAmps {
		watts = volts * amps
		hasWatts = true
	}
	if !hasVolts && !hasAmps && !hasWatts {
		return PVPortHistory{}, false
	}
	return PVPortHistory{
		DeviceID:          deviceID,
		PortID:            portID,
		PortLabel:         label,
		LastObservedVolts: clampFinite(volts),
		LastObservedAmps:  clampFinite(amps),
		LastObservedWatts: clampFinite(watts),
	}, true
}

func numberFromMap(values map[string]any, key string) (float64, bool) {
	raw, ok := values[strings.TrimSpace(key)]
	if !ok {
		return 0, false
	}
	value, ok := raw.(float64)
	if !ok || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	return value, true
}

func clampFinite(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return 0
	}
	return value
}

func envelopeObservedAt(env *envelopev1.TelemetryEnvelope) time.Time {
	if env == nil {
		return time.Time{}
	}
	if ts := env.GetObservedTimeUnixMs(); ts > 0 {
		return time.UnixMilli(ts).UTC()
	}
	if ts := env.GetIngestedTimeUnixMs(); ts > 0 {
		return time.UnixMilli(ts).UTC()
	}
	if ts := env.GetDeviceTimeUnixMs(); ts > 0 {
		return time.UnixMilli(ts).UTC()
	}
	return time.Time{}
}

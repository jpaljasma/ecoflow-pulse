package ecoflow

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// BatteryPackInfo describes one battery-pack entry present in some
// device quota payloads under "*.bpInfo".
type BatteryPackInfo struct {
	BPChgSta     int `json:"bpChgSta"`
	BPEnergy     int `json:"bpEnergy"`
	BPErrCode    int `json:"bpErrCode"`
	BPNo         int `json:"bpNo"`
	BPPwr        int `json:"bpPwr"`
	BPSoc        int `json:"bpSoc"`
	BPSocMax     int `json:"bpSocMax"`
	BPSocMin     int `json:"bpSocMin"`
	BPSunnovaBan int `json:"bpSunnovaBan"`
	BPTemp       int `json:"bpTemp"`
	HeatTime     int `json:"heatTime"`
	RemainTime   int `json:"remainTime"`
}

// ParseBatteryPackInfo parses a bpInfo JSON array value into typed battery-pack
// records.
func ParseBatteryPackInfo(value string) ([]BatteryPackInfo, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return []BatteryPackInfo{}, nil
	}

	var out []BatteryPackInfo
	if err := json.Unmarshal([]byte(trimmed), &out); err != nil {
		return nil, fmt.Errorf("parse bpInfo: %w", err)
	}
	return out, nil
}

// ParseQuotaBatteryPackInfo extracts and parses one quota value by key as
// bpInfo JSON.
func ParseQuotaBatteryPackInfo(
	quota map[string]string,
	key string,
) ([]BatteryPackInfo, bool, error) {
	if len(quota) == 0 {
		return nil, false, nil
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, false, nil
	}

	value, ok := quota[key]
	if !ok {
		return nil, false, nil
	}

	packs, err := ParseBatteryPackInfo(value)
	if err != nil {
		return nil, true, err
	}
	return packs, true, nil
}

// KitInfoWattsEntry describes one entry in bms_kitInfo.watts.
type KitInfoWattsEntry struct {
	AppState int     `json:"appState"`
	AppVer   int64   `json:"appVer"`
	AvaFlag  int     `json:"avaFlag"`
	CurPower int     `json:"curPower"`
	Detail   int     `json:"detail"`
	F32Soc   float64 `json:"f32Soc"`
	LoadVer  int64   `json:"loadVer"`
	SN       string  `json:"sn"`
	Soc      int     `json:"soc"`
	Type     int     `json:"type"`
}

// ParseKitInfoWatts parses a bms_kitInfo.watts JSON array value into typed
// records.
func ParseKitInfoWatts(value string) ([]KitInfoWattsEntry, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return []KitInfoWattsEntry{}, nil
	}

	var out []KitInfoWattsEntry
	if err := json.Unmarshal([]byte(trimmed), &out); err != nil {
		return nil, fmt.Errorf("parse kitInfo.watts: %w", err)
	}
	return out, nil
}

// ParseQuotaKitInfoWatts extracts and parses one quota value by key as
// kitInfo.watts JSON.
func ParseQuotaKitInfoWatts(
	quota map[string]string,
	key string,
) ([]KitInfoWattsEntry, bool, error) {
	if len(quota) == 0 {
		return nil, false, nil
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, false, nil
	}

	value, ok := quota[key]
	if !ok {
		return nil, false, nil
	}

	entries, err := ParseKitInfoWatts(value)
	if err != nil {
		return nil, true, err
	}
	return entries, true, nil
}

// ParseUnsignedIntArray parses JSON array payloads like "[0,9,11]" into
// unsigned integer slices.
func ParseUnsignedIntArray(value string) ([]uint64, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return []uint64{}, nil
	}

	var decoded []any
	if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
		return nil, fmt.Errorf("parse unsigned int array: %w", err)
	}

	out := make([]uint64, 0, len(decoded))
	for i, item := range decoded {
		switch v := item.(type) {
		case float64:
			if v < 0 || math.Trunc(v) != v {
				return nil, fmt.Errorf("parse unsigned int array: index %d has non-uint value %v", i, v)
			}
			out = append(out, uint64(v))
		default:
			return nil, fmt.Errorf("parse unsigned int array: index %d has unsupported type %T", i, item)
		}
	}
	return out, nil
}

// ParseQuotaUnsignedIntArray extracts and parses one quota value by key as an
// unsigned integer array.
func ParseQuotaUnsignedIntArray(
	quota map[string]string,
	key string,
) ([]uint64, bool, error) {
	if len(quota) == 0 {
		return nil, false, nil
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, false, nil
	}

	value, ok := quota[key]
	if !ok {
		return nil, false, nil
	}

	out, err := ParseUnsignedIntArray(value)
	if err != nil {
		return nil, true, err
	}
	return out, true, nil
}

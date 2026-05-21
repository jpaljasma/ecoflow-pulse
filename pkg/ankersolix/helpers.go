package ankersolix

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func asMap(value any) map[string]any {
	switch v := value.(type) {
	case map[string]any:
		return v
	case map[string]string:
		out := make(map[string]any, len(v))
		for key, value := range v {
			out[key] = value
		}
		return out
	case string:
		clean := strings.TrimSpace(v)
		if len(clean) < 2 || clean[0] != '{' {
			return nil
		}
		var out map[string]any
		if err := json.Unmarshal([]byte(clean), &out); err == nil {
			return out
		}
	}
	return nil
}

func asString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

func toFloat(value any) (float64, bool) {
	switch v := value.(type) {
	case nil:
		return 0, false
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int8:
		return float64(v), true
	case int16:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint8:
		return float64(v), true
	case uint16:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	case string:
		clean := strings.TrimSpace(v)
		if clean == "" {
			return 0, false
		}
		f, err := strconv.ParseFloat(clean, 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func toBool(value any) (bool, bool) {
	switch v := value.(type) {
	case bool:
		return v, true
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "1", "on", "yes", "enabled":
			return true, true
		case "false", "0", "off", "no", "disabled":
			return false, true
		default:
			return false, false
		}
	default:
		if f, ok := toFloat(value); ok {
			return f != 0, true
		}
		return false, false
	}
}

func cloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		if nested := asMap(value); len(nested) > 0 {
			out[key] = cloneMap(nested)
			continue
		}
		out[key] = value
	}
	return out
}

func collapseEmptyMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	return in
}

func firstText(record map[string]any, keys ...string) string {
	for _, key := range keys {
		if text := asString(record[key]); text != "" {
			return text
		}
	}
	return ""
}

func rawList(raw any) []any {
	switch v := raw.(type) {
	case []any:
		return v
	case []map[string]any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, item)
		}
		return out
	case map[string]any:
		for _, key := range []string{"device_list", "devices", "list", "data"} {
			if list := rawList(v[key]); len(list) > 0 {
				return list
			}
		}
	}
	return nil
}

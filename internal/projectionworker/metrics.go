package projectionworker

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
)

func extractNumericMetrics(payload []byte) map[string]float64 {
	if len(payload) == 0 || !gjson.ValidBytes(payload) {
		return nil
	}
	root := gjson.ParseBytes(payload)
	out := make(map[string]float64, 32)
	walkMetricValue("", root, out)
	if len(out) == 0 {
		return nil
	}
	return out
}

func walkMetricValue(path string, value gjson.Result, out map[string]float64) {
	switch value.Type {
	case gjson.Number:
		if path != "" {
			out[path] = value.Num
		}
	case gjson.True:
		if path != "" {
			out[path] = 1
		}
	case gjson.False:
		if path != "" {
			out[path] = 0
		}
	case gjson.JSON:
		if value.IsArray() {
			index := 0
			value.ForEach(func(_, child gjson.Result) bool {
				next := joinMetricPath(path, strconv.Itoa(index))
				walkMetricValue(next, child, out)
				index++
				return true
			})
			return
		}
		if value.IsObject() {
			value.ForEach(func(key, child gjson.Result) bool {
				next := joinMetricPath(path, key.String())
				walkMetricValue(next, child, out)
				return true
			})
		}
	}
}

func joinMetricPath(parent, child string) string {
	clean := sanitizeMetricPathSegment(child)
	if parent == "" {
		return clean
	}
	if clean == "" {
		return parent
	}
	return fmt.Sprintf("%s.%s", parent, clean)
}

func sanitizeMetricPathSegment(in string) string {
	clean := strings.TrimSpace(in)
	if clean == "" {
		return ""
	}
	clean = strings.ReplaceAll(clean, "{", "_")
	clean = strings.ReplaceAll(clean, "}", "_")
	clean = strings.ReplaceAll(clean, " ", "_")
	return clean
}

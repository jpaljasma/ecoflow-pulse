package projectionworker

import "github.com/jpaljasma/ecoflow-pulse/internal/currenttelemetry"

func extractNumericMetrics(payload []byte) map[string]float64 {
	return currenttelemetry.ExtractNumericMetrics(payload)
}

package rollupworker

import (
	"strings"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/envelopededup"
)

func rollupDedupKey(env *envelopev1.TelemetryEnvelope) string {
	return strings.TrimSpace(envelopededup.Key(env))
}

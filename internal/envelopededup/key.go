package envelopededup

import (
	"fmt"
	"strings"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
)

func Key(env *envelopev1.TelemetryEnvelope) string {
	if env == nil {
		return ""
	}
	if envelopeID := strings.TrimSpace(env.GetEnvelopeId()); envelopeID != "" {
		return "env:" + envelopeID
	}
	deviceID := strings.TrimSpace(env.GetDeviceId())
	messageID := strings.TrimSpace(env.GetMessageId())
	if deviceID != "" && messageID != "" {
		return fmt.Sprintf("msg:%s:%s", deviceID, messageID)
	}
	return ""
}

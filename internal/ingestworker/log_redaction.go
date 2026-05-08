package ingestworker

import (
	"strings"

	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/logredact"
	"github.com/jpaljasma/ecoflow-pulse/pkg/ankersolix"
	"github.com/jpaljasma/ecoflow-pulse/pkg/pecron"
)

func providerDeviceLogRef(provider string, providerDeviceID string) string {
	if strings.TrimSpace(providerDeviceID) == "" {
		return "unknown"
	}
	switch controlplane.NormalizeProvider(provider) {
	case controlplane.ProviderAnkerSolix:
		ref, err := ankersolix.ParseProviderDeviceID(providerDeviceID)
		if err == nil && ref.ProductCode != "" {
			return ref.ProductCode + ":redacted"
		}
	case controlplane.ProviderPecron:
		ref, err := pecron.ParseProviderDeviceID(providerDeviceID)
		if err == nil && ref.ProductKey != "" {
			return ref.ProductKey + ":redacted"
		}
	}
	return logredact.Identifier(providerDeviceID)
}

func mqttTopicLogRef(topic string) string {
	return logredact.Topic(topic)
}

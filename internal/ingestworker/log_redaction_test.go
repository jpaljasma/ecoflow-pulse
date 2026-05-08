package ingestworker

import (
	"strings"
	"testing"

	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
)

func TestProviderDeviceLogRefRedactsRawIdentifiers(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		raw      string
		want     string
	}{
		{name: "ecoflow", provider: controlplane.ProviderEcoFlow, raw: "DEMOD2M00001057", want: "redacted"},
		{name: "pecron", provider: controlplane.ProviderPecron, raw: "p11vxg:aabbccddeeff", want: "p11vxg:redacted"},
		{name: "anker", provider: controlplane.ProviderAnkerSolix, raw: "a1783:SN-C2000", want: "A1783:redacted"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := providerDeviceLogRef(tt.provider, tt.raw)
			if got != tt.want {
				t.Fatalf("providerDeviceLogRef() = %q, want %q", got, tt.want)
			}
			if strings.Contains(got, "DEMOD") || strings.Contains(got, "aabbccddeeff") || strings.Contains(got, "SN-C2000") {
				t.Fatalf("providerDeviceLogRef leaked raw identifier: %q", got)
			}
		})
	}
}

func TestMQTTTopicLogRefRedactsTopics(t *testing.T) {
	if got := mqttTopicLogRef("/open/open-account/DEMOD2M00001057/quota"); got != "redacted" {
		t.Fatalf("mqttTopicLogRef() = %q, want redacted", got)
	}
	if got := mqttTopicLogRef(" "); got != "" {
		t.Fatalf("empty mqttTopicLogRef() = %q, want empty", got)
	}
}

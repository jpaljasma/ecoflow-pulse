package telemetrypayload

const (
	LegacyEcoFlowQuotaPayloadType = "ecoflow.quota.normalized"
	ProviderNormalizedPayloadType = "provider.params.normalized"
)

func IsNormalizedParamsPayloadType(payloadType string) bool {
	switch payloadType {
	case LegacyEcoFlowQuotaPayloadType, ProviderNormalizedPayloadType:
		return true
	default:
		return false
	}
}

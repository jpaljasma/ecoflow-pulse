package main

// solarChargingProfile controls how solar charging state is inferred for a
// device family.
type solarChargingProfile struct {
	PVActiveMinWatts              float64
	PVHoldMinWatts                float64
	BatteryChargeMinWatts         float64
	AllowFallbackInference        bool
	PreferPackChargeState         bool
	PreferAggregateNetForFallback bool
}

type solarChargingProfileRule struct {
	match   func(*energySnapshot) bool
	profile solarChargingProfile
}

var solarChargingDefaultProfile = solarChargingProfile{
	PVActiveMinWatts:              solarChargePVMinWattsD2M,
	PVHoldMinWatts:                solarChargePVHoldWattsD2M,
	BatteryChargeMinWatts:         solarChargeBatteryMinWatts,
	AllowFallbackInference:        true,
	PreferPackChargeState:         true,
	PreferAggregateNetForFallback: false,
}

var solarChargingProfileRules = []solarChargingProfileRule{
	{
		// Delta Pro Ultra family: pack-based system without XT150 battery-link
		// telemetry. In practice, charging acceptance appears around ~60W PV and
		// fallback inference from aggregate counters is noisy, so this profile is
		// intentionally stricter.
		match: func(s *energySnapshot) bool {
			return s != nil && len(s.Packs) > 0 && !s.HasXT150
		},
		profile: solarChargingProfile{
			PVActiveMinWatts:              solarChargePVMinWattsDPU,
			PVHoldMinWatts:                solarChargePVHoldWattsDPU,
			BatteryChargeMinWatts:         solarChargeBatteryMinWatts,
			AllowFallbackInference:        true,
			PreferPackChargeState:         false,
			PreferAggregateNetForFallback: true,
		},
	},
}

func (s *energySnapshot) solarChargingProfile() solarChargingProfile {
	for _, rule := range solarChargingProfileRules {
		if rule.match(s) {
			return rule.profile
		}
	}
	return solarChargingDefaultProfile
}

package weatherd

func TemperatureCToF(v float64) float64 {
	return (v * 9.0 / 5.0) + 32.0
}

func WindSpeedKmHToMPH(v float64) float64 {
	return v * 0.621371
}

func MillimetersToInches(v float64) float64 {
	return v / 25.4
}

func MetersToMiles(v float64) float64 {
	return v / 1609.344
}

func ForecastValues(in ForecastValueSet, unitSystem UnitSystem) ForecastValueSet {
	if unitSystem != UnitSystemImperial {
		return in
	}
	out := in
	if out.Temperature != nil {
		v := TemperatureCToF(*out.Temperature)
		out.Temperature = &v
	}
	if out.WindSpeed != nil {
		v := WindSpeedKmHToMPH(*out.WindSpeed)
		out.WindSpeed = &v
	}
	if out.Precipitation != nil {
		v := MillimetersToInches(*out.Precipitation)
		out.Precipitation = &v
	}
	if out.Visibility != nil {
		v := MetersToMiles(*out.Visibility)
		out.Visibility = &v
	}
	return out
}

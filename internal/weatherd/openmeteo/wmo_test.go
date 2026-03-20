package openmeteo

import "testing"

func TestWeatherCodeText(t *testing.T) {
	tests := []struct {
		code int32
		want string
	}{
		{code: 0, want: "Clear sky"},
		{code: 63, want: "Rain"},
		{code: 99, want: "Thunderstorm with hail"},
		{code: 404, want: "Unknown"},
	}

	for _, tt := range tests {
		if got := WeatherCodeText(tt.code); got != tt.want {
			t.Fatalf("WeatherCodeText(%d) = %q, want %q", tt.code, got, tt.want)
		}
	}
}

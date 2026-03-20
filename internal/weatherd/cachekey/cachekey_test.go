package cachekey

import "testing"

func TestBuildUsesReturnedGridCellAndBucketsAngles(t *testing.T) {
	tilt := 44.6
	azimuth := 182.0
	key := Build(CanonicalLocation{
		Latitude:            42.61591,
		Longitude:           -77.40144,
		Elevation:           289.84,
		PanelTiltDegrees:    TiltBucket(&tilt),
		PanelAzimuthDegrees: AzimuthBucket(&azimuth),
	})

	want := "lat:42.616|lon:-77.401|elev:289.8|tilt:45|az:180"
	if key != want {
		t.Fatalf("Build() = %q, want %q", key, want)
	}
}

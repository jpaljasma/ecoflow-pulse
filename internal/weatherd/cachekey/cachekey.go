package cachekey

import (
	"fmt"
	"math"
	"strings"
)

type CanonicalLocation struct {
	Latitude            float64
	Longitude           float64
	Elevation           float64
	PanelTiltDegrees    *float64
	PanelAzimuthDegrees *float64
}

func Build(in CanonicalLocation) string {
	parts := []string{
		fmt.Sprintf("lat:%.3f", in.Latitude),
		fmt.Sprintf("lon:%.3f", in.Longitude),
		fmt.Sprintf("elev:%.1f", in.Elevation),
		fmt.Sprintf("tilt:%s", bucketPointer(in.PanelTiltDegrees, 1)),
		fmt.Sprintf("az:%s", bucketPointer(in.PanelAzimuthDegrees, 5)),
	}
	return strings.Join(parts, "|")
}

func TiltBucket(v *float64) *float64 {
	return bucket(v, 1)
}

func AzimuthBucket(v *float64) *float64 {
	return bucket(v, 5)
}

func bucketPointer(v *float64, step float64) string {
	out := bucket(v, step)
	if out == nil {
		return "none"
	}
	if step >= 1 {
		return fmt.Sprintf("%.0f", *out)
	}
	return fmt.Sprintf("%.3f", *out)
}

func bucket(v *float64, step float64) *float64 {
	if v == nil || step <= 0 {
		return nil
	}
	b := math.Round(*v/step) * step
	return &b
}

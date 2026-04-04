package pulsemqttemulator

import (
	"fmt"
	"strings"
	"time"
)

// TopicName returns the EcoFlow-compatible quota topic for the given account
// and provider device identifier.
func TopicName(account string, deviceSN string) string {
	return fmt.Sprintf("/open/%s/%s/quota", strings.TrimSpace(account), strings.ToUpper(strings.TrimSpace(deviceSN)))
}

// TickForTime maps a wall-clock instant onto the emulator's publish cadence so
// generated historical frames match the same waveform logic as live ticks.
func TickForTime(at time.Time, publishInterval time.Duration) int {
	if publishInterval <= 0 {
		publishInterval = defaultPublishInterval
	}
	return int(at.UTC().UnixNano() / publishInterval.Nanoseconds())
}

// MQTTFramesAt returns the emulator's MQTT quota frames for the provided
// instant using the standard waveform tick derived from the publish cadence.
func MQTTFramesAt(at time.Time, publishInterval time.Duration) [][]byte {
	return buildMQTTFrames(snapshotForTime(TickForTime(at, publishInterval), at))
}

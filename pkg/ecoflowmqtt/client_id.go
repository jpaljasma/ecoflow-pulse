package ecoflowmqtt

import (
	"fmt"
	"hash/crc32"
	"strings"
)

// BuildClientIDFromSN derives deterministic MQTT client ID from provider serial number.
func BuildClientIDFromSN(sn string) string {
	cleanSN := strings.TrimSpace(sn)
	checksum := crc32.ChecksumIEEE([]byte(cleanSN))
	return fmt.Sprintf("ecoflow-pulse-%08x", checksum)
}

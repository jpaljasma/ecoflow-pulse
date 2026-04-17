package ecoflowmqtt

import (
	"fmt"
	"hash/crc32"
	"strings"
)

// BuildClientIDFromSN derives deterministic MQTT client ID from provider serial number.
func BuildClientIDFromSN(sn string) string {
	return BuildClientIDWithNamespace("", sn)
}

// BuildClientIDWithNamespace derives deterministic MQTT client ID from an
// optional environment namespace plus provider serial number. When namespace is
// blank, it preserves the legacy seed-only client ID behavior.
func BuildClientIDWithNamespace(namespace, seed string) string {
	cleanSeed := strings.TrimSpace(seed)
	cleanNamespace := strings.TrimSpace(namespace)
	checksumInput := cleanSeed
	if cleanNamespace != "" {
		checksumInput = strings.ToLower(cleanNamespace) + "|" + cleanSeed
	}
	checksum := crc32.ChecksumIEEE([]byte(checksumInput))
	return fmt.Sprintf("ecoflow-pulse-%08x", checksum)
}

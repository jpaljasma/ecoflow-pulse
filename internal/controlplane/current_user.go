package controlplane

import "strings"

func PreferredProviderDisplayName(displayName string, givenName string, familyName string, email string) string {
	if value := strings.TrimSpace(displayName); value != "" {
		return value
	}
	fullName := strings.TrimSpace(strings.Join([]string{
		strings.TrimSpace(givenName),
		strings.TrimSpace(familyName),
	}, " "))
	if fullName != "" {
		return fullName
	}
	return strings.TrimSpace(email)
}

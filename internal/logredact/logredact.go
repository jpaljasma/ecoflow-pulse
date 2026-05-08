package logredact

import "strings"

func Identifier(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return "redacted"
}

func Email(value string) string {
	return Identifier(value)
}

func Topic(value string) string {
	return Identifier(value)
}

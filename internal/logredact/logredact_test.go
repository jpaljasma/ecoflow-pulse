package logredact

import "testing"

func TestRedactorsHideSensitiveValues(t *testing.T) {
	for name, redact := range map[string]func(string) string{
		"identifier": Identifier,
		"email":      Email,
		"topic":      Topic,
	} {
		t.Run(name, func(t *testing.T) {
			if got := redact("secret-value"); got != "redacted" {
				t.Fatalf("redacted value = %q, want redacted", got)
			}
			if got := redact(" "); got != "" {
				t.Fatalf("blank value = %q, want empty", got)
			}
		})
	}
}

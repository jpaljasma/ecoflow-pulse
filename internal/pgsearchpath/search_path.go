package pgsearchpath

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

const GlobalEnvKey = "DB_SCHEMA_SEARCH_PATH"

func ResolveEnv(overrideEnvKey string) string {
	if overrideEnvKey != "" {
		if value := strings.TrimSpace(os.Getenv(overrideEnvKey)); value != "" {
			return value
		}
	}
	return strings.TrimSpace(os.Getenv(GlobalEnvKey))
}

func Apply(dsn string, searchPath string) (string, error) {
	dsn = strings.TrimSpace(dsn)
	searchPath = strings.TrimSpace(searchPath)
	if dsn == "" || searchPath == "" {
		return dsn, nil
	}
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		parsed, err := url.Parse(dsn)
		if err != nil {
			return "", fmt.Errorf("parse postgres url: %w", err)
		}
		values := parsed.Query()
		values.Set("search_path", searchPath)
		parsed.RawQuery = values.Encode()
		return parsed.String(), nil
	}
	parts := strings.Fields(dsn)
	filtered := make([]string, 0, len(parts)+1)
	for _, part := range parts {
		if strings.HasPrefix(strings.ToLower(part), "search_path=") {
			continue
		}
		filtered = append(filtered, part)
	}
	filtered = append(filtered, "search_path="+searchPath)
	return strings.Join(filtered, " "), nil
}

func ApplyFromEnv(dsn string, overrideEnvKey string) (string, error) {
	return Apply(dsn, ResolveEnv(overrideEnvKey))
}

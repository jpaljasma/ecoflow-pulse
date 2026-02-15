package ecoflow

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"
)

func loadDotEnvFile(path string, override bool) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read dotenv file %q: %w", path, err)
	}

	lines := strings.Split(string(data), "\n")
	for i, raw := range lines {
		key, value, ok, parseErr := parseDotEnvLine(raw)
		if parseErr != nil {
			return fmt.Errorf("parse dotenv %q line %d: %w", path, i+1, parseErr)
		}
		if !ok {
			continue
		}
		if !override {
			if existing, exists := os.LookupEnv(key); exists && strings.TrimSpace(existing) != "" {
				continue
			}
		}
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("set environment variable %q from dotenv: %w", key, err)
		}
	}
	return nil
}

func parseDotEnvLine(line string) (key string, value string, ok bool, err error) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false, nil
	}

	if strings.HasPrefix(line, "export ") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
	}

	idx := strings.IndexByte(line, '=')
	if idx <= 0 {
		return "", "", false, fmt.Errorf("invalid line: expected KEY=VALUE")
	}

	key = strings.TrimSpace(line[:idx])
	if !isValidDotEnvKey(key) {
		return "", "", false, fmt.Errorf("invalid key %q", key)
	}

	valuePart := strings.TrimSpace(line[idx+1:])
	valuePart = stripInlineComment(valuePart)
	value, err = parseDotEnvValue(valuePart)
	if err != nil {
		return "", "", false, err
	}

	return key, value, true, nil
}

func parseDotEnvValue(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}

	if strings.HasPrefix(raw, "\"") {
		unquoted, err := strconv.Unquote(raw)
		if err != nil {
			return "", fmt.Errorf("invalid quoted value: %w", err)
		}
		return unquoted, nil
	}

	if strings.HasPrefix(raw, "'") {
		if !strings.HasSuffix(raw, "'") || len(raw) < 2 {
			return "", fmt.Errorf("invalid single-quoted value")
		}
		return raw[1 : len(raw)-1], nil
	}

	return strings.TrimSpace(raw), nil
}

func isValidDotEnvKey(key string) bool {
	if key == "" {
		return false
	}
	for i, r := range key {
		if i == 0 {
			if !(unicode.IsLetter(r) || r == '_') {
				return false
			}
			continue
		}
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_') {
			return false
		}
	}
	return true
}

func stripInlineComment(value string) string {
	inSingle := false
	inDouble := false
	escaped := false
	last := rune(0)

	for i, r := range value {
		if escaped {
			escaped = false
			last = r
			continue
		}
		if r == '\\' && inDouble {
			escaped = true
			last = r
			continue
		}
		switch r {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '#':
			if !inSingle && !inDouble && unicode.IsSpace(last) {
				return strings.TrimSpace(value[:i])
			}
		}
		last = r
	}
	return strings.TrimSpace(value)
}

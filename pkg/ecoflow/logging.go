package ecoflow

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/jpaljasma/ecoflow-pulse/pkg/logger"
)

// LoggingOptions controls client-side structured logging behavior.
type LoggingOptions struct {
	Logger                 *slog.Logger
	Debug                  bool
	AdvancedDebugTelemetry bool
	DebugLogHeaders        bool
}

func defaultLoggingOptions(env Environment) LoggingOptions {
	debug := env == EnvironmentDev
	if debug {
		return LoggingOptions{
			Logger:                 logger.NewDevelopmentJSON(os.Stderr),
			Debug:                  true,
			AdvancedDebugTelemetry: false,
			DebugLogHeaders:        false,
		}
	}
	return LoggingOptions{
		Logger:                 logger.NewProductionJSON(os.Stderr),
		Debug:                  false,
		AdvancedDebugTelemetry: false,
		DebugLogHeaders:        false,
	}
}

func parseBoolEnvironment(name string) (bool, bool, error) {
	raw, ok := os.LookupEnv(name)
	if !ok {
		return false, false, nil
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false, false, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, true, fmt.Errorf("invalid %s value %q: %w", name, raw, err)
	}
	return value, true, nil
}

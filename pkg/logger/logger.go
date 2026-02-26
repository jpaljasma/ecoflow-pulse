package logger

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// NewProductionJSON builds a JSON logger tuned for production verbosity.
func NewProductionJSON(out io.Writer) *slog.Logger {
	return NewJSON(out, slog.LevelInfo)
}

// NewDevelopmentJSON builds a JSON logger with debug verbosity.
func NewDevelopmentJSON(out io.Writer) *slog.Logger {
	return NewJSON(out, slog.LevelDebug)
}

// NewJSON builds a JSON slog logger with normalized level/timestamp fields.
func NewJSON(out io.Writer, level slog.Level) *slog.Logger {
	return slog.New(NewJSONHandler(out, level))
}

// NewJSONHandler builds a JSON slog handler with normalized level/timestamp fields.
func NewJSONHandler(out io.Writer, level slog.Level) slog.Handler {
	if out == nil {
		out = os.Stderr
	}

	levelVar := new(slog.LevelVar)
	levelVar.Set(level)

	return slog.NewJSONHandler(out, &slog.HandlerOptions{
		Level:     levelVar,
		AddSource: false,
		ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
			switch attr.Key {
			case slog.TimeKey:
				if attr.Value.Kind() == slog.KindTime {
					return slog.Int64("ts_unix_ms", attr.Value.Time().UnixMilli())
				}
				return attr
			case slog.LevelKey:
				return slog.String("level", strings.ToLower(attr.Value.String()))
			default:
				return attr
			}
		},
	})
}

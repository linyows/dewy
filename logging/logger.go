package logging

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// Logger wraps slog.Logger with format information.
type Logger struct {
	*slog.Logger
	format string
}

// Format returns the logger format (json or text).
func (l *Logger) Format() string {
	return l.format
}

// Slog returns the underlying *slog.Logger. Prefer this over reaching for
// the embedded field directly so call sites read as deliberate hand-offs to
// downstream packages whose APIs accept *slog.Logger.
func (l *Logger) Slog() *slog.Logger {
	return l.Logger
}

// SetupLogger creates and configures a structured logger.
func SetupLogger(level, format string, output io.Writer) *Logger {
	var slogLevel slog.Level
	switch strings.ToUpper(level) {
	case "DEBUG":
		slogLevel = slog.LevelDebug
	case "INFO":
		slogLevel = slog.LevelInfo
	case "WARN":
		slogLevel = slog.LevelWarn
	case "ERROR":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	if output == nil {
		output = os.Stderr
	}

	opts := &slog.HandlerOptions{
		Level: slogLevel,
	}

	var handler slog.Handler
	switch strings.ToLower(format) {
	case "json":
		handler = slog.NewJSONHandler(output, opts)
	case "text":
		fallthrough
	default:
		handler = slog.NewTextHandler(output, opts)
	}

	return &Logger{
		Logger: slog.New(handler),
		format: strings.ToLower(format),
	}
}

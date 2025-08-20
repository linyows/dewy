package dewy

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// SetupLogger creates and configures a structured logger
func SetupLogger(level, format string, output io.Writer) *slog.Logger {
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

	return slog.New(handler)
}

package dewy

import (
	"github.com/linyows/dewy/logging"
	"io"
)

// SetupLogger creates and configures a structured logger
func SetupLogger(level, format string, output io.Writer) *logging.Logger {
	return logging.SetupLogger(level, format, output)
}

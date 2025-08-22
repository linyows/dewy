package dewy

import (
	"io"
	"github.com/linyows/dewy/logging"
)

// SetupLogger creates and configures a structured logger
func SetupLogger(level, format string, output io.Writer) *logging.Logger {
	return logging.SetupLogger(level, format, output)
}

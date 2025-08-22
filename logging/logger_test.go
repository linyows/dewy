package logging

import (
	"bytes"
	"strings"
	"testing"
)

func TestSetupLogger(t *testing.T) {
	tests := []struct {
		name        string
		level       string
		format      string
		expectJSON  bool
		expectLevel string
	}{
		{
			name:        "JSON format logger",
			level:       "INFO",
			format:      "json",
			expectJSON:  true,
			expectLevel: "INFO",
		},
		{
			name:        "Text format logger",
			level:       "DEBUG",
			format:      "text",
			expectJSON:  false,
			expectLevel: "DEBUG",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := SetupLogger(tt.level, tt.format, &buf)

			// Test that Format() returns the expected format
			if logger.Format() != strings.ToLower(tt.format) {
				t.Errorf("expected format %s, got %s", strings.ToLower(tt.format), logger.Format())
			}

			// Test logging output
			logger.Info("test message", "key", "value")
			output := buf.String()

			if tt.expectJSON {
				if !strings.Contains(output, `"msg":"test message"`) {
					t.Errorf("expected JSON output, got: %s", output)
				}
			} else {
				if !strings.Contains(output, "test message") {
					t.Errorf("expected text output to contain message, got: %s", output)
				}
			}
		})
	}
}

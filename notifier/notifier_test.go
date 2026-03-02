package notifier

import (
	"context"
	"testing"
	"time"
)

// mockSender records all messages sent to it.
type mockSender struct {
	messages    []string
	hookResults []*hookResultCall
}

type hookResultCall struct {
	hookType string
	result   *HookResult
}

func (m *mockSender) Send(ctx context.Context, message string) {
	m.messages = append(m.messages, message)
}

func (m *mockSender) SendHookResult(ctx context.Context, hookType string, result *HookResult) {
	m.hookResults = append(m.hookResults, &hookResultCall{hookType: hookType, result: result})
}

func newTestErrorLimitingSender(quiet bool) (*ErrorLimitingSender, *mockSender) {
	mock := &mockSender{}
	return &ErrorLimitingSender{
		underlying: mock,
		quiet:      quiet,
		logger:     testLogger(),
	}, mock
}

func TestErrorLimitingSender_QuietModeSuppressesSend(t *testing.T) {
	sender, mock := newTestErrorLimitingSender(true)
	ctx := context.Background()

	sender.Send(ctx, "should be suppressed")

	if len(mock.messages) != 0 {
		t.Errorf("expected 0 messages in quiet mode, got %d: %v", len(mock.messages), mock.messages)
	}
}

func TestErrorLimitingSender_NonQuietModeAllowsSend(t *testing.T) {
	sender, mock := newTestErrorLimitingSender(false)
	ctx := context.Background()

	sender.Send(ctx, "should be sent")

	if len(mock.messages) != 1 {
		t.Errorf("expected 1 message in non-quiet mode, got %d", len(mock.messages))
	}
}

func TestErrorLimitingSender_QuietModeAllowsSendImportant(t *testing.T) {
	sender, mock := newTestErrorLimitingSender(true)
	ctx := context.Background()

	sender.SendImportant(ctx, "important message")

	if len(mock.messages) != 1 {
		t.Errorf("expected 1 message for SendImportant in quiet mode, got %d", len(mock.messages))
	}
	if len(mock.messages) > 0 && mock.messages[0] != "important message" {
		t.Errorf("expected message 'important message', got '%s'", mock.messages[0])
	}
}

func TestErrorLimitingSender_QuietModeAllowsErrors(t *testing.T) {
	sender, mock := newTestErrorLimitingSender(true)
	ctx := context.Background()

	testErr := &testError{msg: "test error"}
	sender.SendError(ctx, testErr)

	if len(mock.messages) != 1 {
		t.Errorf("expected 1 message for SendError in quiet mode, got %d", len(mock.messages))
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestErrorLimitingSender_QuietModeSuppressesSuccessfulHookResult(t *testing.T) {
	sender, mock := newTestErrorLimitingSender(true)
	ctx := context.Background()

	result := &HookResult{
		Command:  "echo hello",
		Stdout:   "hello",
		ExitCode: 0,
		Duration: time.Second,
		Success:  true,
	}
	sender.SendHookResult(ctx, "Pre-deploy", result)

	if len(mock.messages) != 0 {
		t.Errorf("expected 0 messages for successful hook in quiet mode, got %d: %v", len(mock.messages), mock.messages)
	}
	if len(mock.hookResults) != 0 {
		t.Errorf("expected 0 hook results for successful hook in quiet mode, got %d", len(mock.hookResults))
	}
}

func TestErrorLimitingSender_QuietModeAllowsFailedHookResult(t *testing.T) {
	sender, mock := newTestErrorLimitingSender(true)
	ctx := context.Background()

	result := &HookResult{
		Command:  "exit 1",
		Stderr:   "error",
		ExitCode: 1,
		Duration: time.Second,
		Success:  false,
	}
	sender.SendHookResult(ctx, "Pre-deploy", result)

	if len(mock.hookResults) != 1 {
		// Fallback to regular message if underlying doesn't implement AttachmentSender
		if len(mock.messages) != 1 {
			t.Errorf("expected 1 message for failed hook in quiet mode, got messages=%d hookResults=%d", len(mock.messages), len(mock.hookResults))
		}
	}
}

func TestErrorLimitingSender_SendImportantRespectsErrorCount(t *testing.T) {
	sender, mock := newTestErrorLimitingSender(true)
	ctx := context.Background()

	// Trigger an error to increment error count
	sender.SendError(ctx, &testError{msg: "error"})
	mock.messages = nil // clear the error message

	sender.SendImportant(ctx, "should be suppressed due to error count")

	if len(mock.messages) != 0 {
		t.Errorf("expected 0 messages when error count > 0, got %d", len(mock.messages))
	}
}

func TestParseQuietFlag(t *testing.T) {
	tests := []struct {
		name     string
		rawPart  string
		expected bool
	}{
		{
			name:     "quiet=true",
			rawPart:  "/general?title=myapp&quiet=true",
			expected: true,
		},
		{
			name:     "quiet=1",
			rawPart:  "/general?title=myapp&quiet=1",
			expected: true,
		},
		{
			name:     "quiet=false",
			rawPart:  "/general?title=myapp&quiet=false",
			expected: false,
		},
		{
			name:     "no quiet parameter",
			rawPart:  "/general?title=myapp",
			expected: false,
		},
		{
			name:     "empty string",
			rawPart:  "",
			expected: false,
		},
		{
			name:     "quiet=T",
			rawPart:  "/general?quiet=T",
			expected: true,
		},
		{
			name:     "quiet=TRUE",
			rawPart:  "/general?quiet=TRUE",
			expected: true,
		},
		{
			name:     "quiet=0",
			rawPart:  "/general?quiet=0",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseQuietFlag(tt.rawPart)
			if got != tt.expected {
				t.Errorf("parseQuietFlag(%q) = %v, want %v", tt.rawPart, got, tt.expected)
			}
		})
	}
}

package notifier

import (
	"context"
	"fmt"
	"strings"
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

// Only error notifications are limited by the error count. The count is
// reset after a successful tick returns, so progress notifications sent during
// that tick (download, hook results, deploy success) must not depend on it.
func TestErrorLimitingSender_ErrorCountDoesNotSuppressOtherNotifications(t *testing.T) {
	failed := &HookResult{Command: "exit 1", ExitCode: 1, Success: false}
	succeeded := &HookResult{Command: "true", ExitCode: 0, Success: true}

	tests := []struct {
		name         string
		quiet        bool
		send         func(context.Context, *ErrorLimitingSender)
		wantMessages int
		wantHooks    int
	}{
		{"Send", false, func(ctx context.Context, s *ErrorLimitingSender) { s.Send(ctx, "downloaded") }, 1, 0},
		{"SendImportant", false, func(ctx context.Context, s *ErrorLimitingSender) { s.SendImportant(ctx, "deployed") }, 1, 0},
		{"SendImportant in quiet mode", true, func(ctx context.Context, s *ErrorLimitingSender) { s.SendImportant(ctx, "deployed") }, 1, 0},
		{"SendHookResult", false, func(ctx context.Context, s *ErrorLimitingSender) { s.SendHookResult(ctx, "after", succeeded) }, 0, 1},
		{"failed SendHookResult in quiet mode", true, func(ctx context.Context, s *ErrorLimitingSender) { s.SendHookResult(ctx, "after", failed) }, 0, 1},
	}

	for _, tt := range tests {
		for _, errors := range []int{1, maxNotifyErrors, maxNotifyErrors + 5} {
			t.Run(fmt.Sprintf("%s after %d errors", tt.name, errors), func(t *testing.T) {
				sender, mock := newTestErrorLimitingSender(tt.quiet)
				ctx := context.Background()
				for range errors {
					sender.SendError(ctx, &testError{msg: "registry down"})
				}
				mock.messages = nil

				tt.send(ctx, sender)

				if len(mock.messages) != tt.wantMessages || len(mock.hookResults) != tt.wantHooks {
					t.Errorf("got messages=%d hookResults=%d, want messages=%d hookResults=%d",
						len(mock.messages), len(mock.hookResults), tt.wantMessages, tt.wantHooks)
				}
			})
		}
	}
}

// After the error count is reset, error notifications start over from 1 and
// are limited again.
func TestErrorLimitingSender_ResetRestartsErrorNotifications(t *testing.T) {
	sender, mock := newTestErrorLimitingSender(false)
	ctx := context.Background()

	for range maxNotifyErrors + 2 {
		sender.SendError(ctx, &testError{msg: "registry down"})
	}
	if len(mock.messages) != maxNotifyErrors {
		t.Fatalf("error notifications before reset = %d, want %d", len(mock.messages), maxNotifyErrors)
	}

	sender.ResetErrorCount()
	mock.messages = nil

	for range maxNotifyErrors + 2 {
		sender.SendError(ctx, &testError{msg: "registry down again"})
	}
	if len(mock.messages) != maxNotifyErrors {
		t.Fatalf("error notifications after reset = %d, want %d", len(mock.messages), maxNotifyErrors)
	}
	if !strings.Contains(mock.messages[0], "count: 1") {
		t.Errorf("first error notification after reset = %q, want count 1", mock.messages[0])
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

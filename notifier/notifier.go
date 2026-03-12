package notifier

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/user"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Sender interface for basic message sending.
type Sender interface {
	Send(ctx context.Context, message string)
}

// HookResult represents the result of executing a deploy hook.
type HookResult struct {
	Command  string
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
	Success  bool
}

// AttachmentSender interface for sending messages with attachments.
type AttachmentSender interface {
	SendHookResult(ctx context.Context, hookType string, result *HookResult)
}

// BroadcastSender interface for sending messages with broadcast (thread + channel).
type BroadcastSender interface {
	SendBroadcast(ctx context.Context, message string)
}

// Notifier interface extends Sender with error handling and hook result notifications.
type Notifier interface {
	Sender
	AttachmentSender
	SendImportant(ctx context.Context, message string)
	SendError(ctx context.Context, err error)
	ResetErrorCount()
	SetThreadTS(ts string)
}

const (
	maxNotifyErrors = 3
	maxErrorCount   = 1000 // Prevent integer overflow
)

// ErrorLimitingSender wraps a Sender implementation with error limiting functionality.
type ErrorLimitingSender struct {
	underlying Sender
	errorCount int
	quiet      bool
	mu         sync.RWMutex
	logger     *slog.Logger
}

// Send sends a message only if error count is 0 and quiet mode is off.
func (e *ErrorLimitingSender) Send(ctx context.Context, message string) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.errorCount == 0 && !e.quiet {
		e.underlying.Send(ctx, message)
	}
}

// SendImportant sends a message regardless of quiet mode (but still respects error count).
// If the underlying sender supports BroadcastSender, it uses broadcast (thread + channel).
func (e *ErrorLimitingSender) SendImportant(ctx context.Context, message string) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.errorCount == 0 {
		if bs, ok := e.underlying.(BroadcastSender); ok {
			bs.SendBroadcast(ctx, message)
		} else {
			e.underlying.Send(ctx, message)
		}
	}
}

// SendError handles error notifications with count limiting.
func (e *ErrorLimitingSender) SendError(ctx context.Context, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Prevent integer overflow by capping the error count
	if e.errorCount < maxErrorCount {
		e.errorCount++
	}

	// Send notification if error count is within the limit
	// Error notifications use broadcast so they appear in the main channel feed
	sendMsg := func(msg string) {
		if bs, ok := e.underlying.(BroadcastSender); ok {
			bs.SendBroadcast(ctx, msg)
		} else {
			e.underlying.Send(ctx, msg)
		}
	}

	if e.errorCount < maxNotifyErrors {
		msg := fmt.Sprintf("Error occurred (count: %d): %v", e.errorCount, err)
		sendMsg(msg)
	} else if e.errorCount == maxNotifyErrors {
		msg := fmt.Sprintf("⚠️ No more error notifications will be sent until errors are resolved.\n\nError occurred (count: %d): %v", e.errorCount, err)
		sendMsg(msg)
	}

	// Log all errors regardless of notification count
	e.logger.Error("Error count", slog.Int("count", e.errorCount), slog.String("error", err.Error()))
}

// ResetErrorCount resets error count when operation succeeds.
func (e *ErrorLimitingSender) ResetErrorCount() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.errorCount > 0 {
		e.logger.Info("Error count reset", slog.Int("from", e.errorCount), slog.Int("to", 0))
		e.errorCount = 0
	}
}

// SetThreadTS delegates to the underlying sender if it supports SetThreadTS.
func (e *ErrorLimitingSender) SetThreadTS(ts string) {
	if setter, ok := e.underlying.(interface{ SetThreadTS(string) }); ok {
		setter.SetThreadTS(ts)
	}
}

// SendHookResult sends hook result notification.
// In quiet mode, only failed hook results are sent.
func (e *ErrorLimitingSender) SendHookResult(ctx context.Context, hookType string, result *HookResult) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.errorCount == 0 {
		if e.quiet && result.Success {
			return
		}
		if attachmentSender, ok := e.underlying.(AttachmentSender); ok {
			attachmentSender.SendHookResult(ctx, hookType, result)
		} else {
			// Fallback to regular message for notifiers that don't support attachments
			statusIcon := "✓"
			if !result.Success {
				statusIcon = "✗"
			}
			msg := fmt.Sprintf("%s %s Hook Result\n**Command:** `%s`\n**Exit Code:** %d\n**Duration:** %s",
				statusIcon, hookType, result.Command, result.ExitCode, result.Duration)

			if result.Stdout != "" {
				msg += fmt.Sprintf("\n**Stdout:**\n```\n%s\n```", result.Stdout)
			}
			if result.Stderr != "" {
				msg += fmt.Sprintf("\n**Stderr:**\n```\n%s\n```", result.Stderr)
			}

			e.underlying.Send(ctx, msg)
		}
	}
}

// New returns Notifier.
func New(ctx context.Context, url string, logger *slog.Logger) (Notifier, error) {
	splitted := strings.SplitN(url, "://", 2)

	var underlying Sender
	switch splitted[0] {
	case "":
		underlying = &Null{}
	case "slack":
		sl, err := NewSlack(splitted[1], logger)
		if err != nil {
			logger.Error("Notification error", slog.String("error", err.Error()))
			underlying = &Null{}
		} else {
			underlying = sl
		}
	case "mail", "smtp":
		ml, err := NewMail(splitted[1], logger)
		if err != nil {
			logger.Error("Notification error", slog.String("error", err.Error()))
			underlying = &Null{}
		} else {
			underlying = ml
		}
	default:
		return nil, fmt.Errorf("unsupported notify: %s", url)
	}

	var quiet bool
	if len(splitted) > 1 {
		quiet = parseQuietFlag(splitted[1])
	}

	return &ErrorLimitingSender{
		underlying: underlying,
		errorCount: 0,
		quiet:      quiet,
		logger:     logger,
	}, nil
}

// parseQuietFlag parses the quiet parameter from a URL query string.
func parseQuietFlag(rawPart string) bool {
	u, err := url.Parse(rawPart)
	if err != nil {
		return false
	}
	v := u.Query().Get("quiet")
	b, _ := strconv.ParseBool(v)
	return b
}

func hostname() string {
	n, err := os.Hostname()
	if err != nil {
		return fmt.Sprintf("%#v", err)
	}
	return n
}

func cwd() string {
	c, err := os.Getwd()
	if err != nil {
		return fmt.Sprintf("%#v", err)
	}
	return c
}

func username() string {
	u, err := user.Current()
	if err != nil {
		return fmt.Sprintf("%#v", err)
	}
	return u.Name
}

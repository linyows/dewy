package notify

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/user"
	"strings"
	"sync"
)

// Sender interface for basic message sending.
type Sender interface {
	Send(ctx context.Context, message string)
}

// Notifier interface extends Sender with error handling
type Notifier interface {
	Sender
	SendError(ctx context.Context, err error)
	ResetErrorCount()
}

const (
	maxNotifyErrors = 3
	maxErrorCount   = 1000 // Prevent integer overflow
)

// ErrorLimitingSender wraps a Sender implementation with error limiting functionality
type ErrorLimitingSender struct {
	underlying Sender
	errorCount int
	mu         sync.RWMutex
}

// Send sends a message only if error count is 0
func (e *ErrorLimitingSender) Send(ctx context.Context, message string) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.errorCount == 0 {
		e.underlying.Send(ctx, message)
	}
}

// SendError handles error notifications with count limiting
func (e *ErrorLimitingSender) SendError(ctx context.Context, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Prevent integer overflow by capping the error count
	if e.errorCount < maxErrorCount {
		e.errorCount++
	}

	// Send notification if error count is within the limit
	if e.errorCount < maxNotifyErrors {
		msg := fmt.Sprintf("Error occurred (count: %d): %v", e.errorCount, err)
		e.underlying.Send(ctx, msg)
	} else if e.errorCount == maxNotifyErrors {
		msg := fmt.Sprintf("⚠️ No more error notifications will be sent until errors are resolved.\n\nError occurred (count: %d): %v", e.errorCount, err)
		e.underlying.Send(ctx, msg)
	}

	// Log all errors regardless of notification count
	log.Printf("[ERROR] Error count: %d, %v", e.errorCount, err)
}

// ResetErrorCount resets error count when operation succeeds
func (e *ErrorLimitingSender) ResetErrorCount() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.errorCount > 0 {
		log.Printf("[INFO] Error count reset from %d to 0", e.errorCount)
		e.errorCount = 0
	}
}

// New returns Notifier.
func New(ctx context.Context, url string) (Notifier, error) {
	splitted := strings.SplitN(url, "://", 2)

	var underlying Sender
	switch splitted[0] {
	case "":
		underlying = &Null{}
	case "slack":
		sl, err := NewSlack(splitted[1])
		if err != nil {
			log.Printf("[ERROR] %s", err)
			underlying = &Null{}
		} else {
			underlying = sl
		}
	case "mail", "smtp":
		ml, err := NewMail(splitted[1])
		if err != nil {
			log.Printf("[ERROR] %s", err)
			underlying = &Null{}
		} else {
			underlying = ml
		}
	default:
		return nil, fmt.Errorf("unsupported notify: %s", url)
	}

	return &ErrorLimitingSender{
		underlying: underlying,
		errorCount: 0,
	}, nil
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

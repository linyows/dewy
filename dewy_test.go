package dewy

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/linyows/dewy/kvs"
	"github.com/linyows/dewy/logging"
	"github.com/linyows/dewy/notifier"
	"github.com/linyows/dewy/registry"
)

// testLogger creates a logger that discards output for testing
func testLogger() *logging.Logger {
	return logging.SetupLogger("INFO", "text", io.Discard)
}

func TestNew(t *testing.T) {
	reg := "ghr://linyows/dewy?pre-release=true"
	c := DefaultConfig()
	c.Registry = reg
	dewy, err := New(c, testLogger())
	if err != nil {
		t.Fatal(err)
	}
	wd, _ := os.Getwd()

	expect := &Dewy{
		config: Config{
			Registry: reg,
			Cache: CacheConfig{
				Type:       FILE,
				Expiration: 10,
			},
			Starter: nil,
		},
		cache:           dewy.cache,
		isServerRunning: false,
		root:            wd,
	}

	opts := []cmp.Option{
		cmp.AllowUnexported(Dewy{}, kvs.File{}),
		cmpopts.IgnoreFields(Dewy{}, "RWMutex", "logger"),
		cmpopts.IgnoreFields(kvs.File{}, "mutex", "logger"),
	}
	if diff := cmp.Diff(dewy, expect, opts...); diff != "" {
		t.Error(diff)
	}
}

type mockRegistry struct {
	url         string
	currentFunc func(context.Context) (*registry.CurrentResponse, error)
	reportFunc  func(context.Context, *registry.ReportRequest) error
}

func (r *mockRegistry) Current(ctx context.Context) (*registry.CurrentResponse, error) {
	if r.currentFunc != nil {
		return r.currentFunc(ctx)
	}
	return &registry.CurrentResponse{
		ID:          "id",
		Tag:         "tag",
		ArtifactURL: r.url,
	}, nil
}

func (r *mockRegistry) Report(ctx context.Context, req *registry.ReportRequest) error {
	if r.reportFunc != nil {
		return r.reportFunc(ctx, req)
	}
	return nil
}

type mockArtifact struct {
	binary string
	url    string
}

func (a *mockArtifact) Download(ctx context.Context, w io.Writer) error {
	zw := zip.NewWriter(w)
	defer zw.Close()

	fInZip, err := zw.Create(a.binary)
	if err != nil {
		return fmt.Errorf("failed to create file in zip: %w", err)
	}

	_, err = io.Copy(fInZip, bytes.NewBufferString(a.url))
	if err != nil {
		return fmt.Errorf("failed to write content to file in zip: %w", err)
	}

	return nil
}

// mockNotify is a mock notify for testing error notification limiting
type mockNotify struct {
	messages   []string
	errorCount int
	mu         sync.Mutex
}

func (n *mockNotify) Send(ctx context.Context, msg string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.errorCount == 0 {
		n.messages = append(n.messages, msg)
	}
}

func (n *mockNotify) SendError(ctx context.Context, err error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Prevent integer overflow by capping the error count
	if n.errorCount < 1000 {
		n.errorCount++
	}

	// Send notification if error count is within the limit
	if n.errorCount < 3 {
		msg := fmt.Sprintf("Error occurred (count: %d): %v", n.errorCount, err)
		n.messages = append(n.messages, msg)
	} else if n.errorCount == 3 {
		msg := fmt.Sprintf("⚠️ No more error notifications will be sent until errors are resolved.\n\nError occurred (count: %d): %v", n.errorCount, err)
		n.messages = append(n.messages, msg)
	}
}

func (n *mockNotify) ResetErrorCount() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.errorCount = 0
}

func (n *mockNotify) GetMessages() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]string(nil), n.messages...)
}

func TestRun(t *testing.T) {
	binary := "dewy"
	artifact := "ghr://linyows/dewy/tag/v1.2.3/artifact.zip"

	root := t.TempDir()
	c := DefaultConfig()
	c.Command = ASSETS
	c.Registry = "ghr://linyows/dewy"
	c.Cache = CacheConfig{
		Type:       FILE,
		Expiration: 10,
	}
	dewy, err := New(c, testLogger())
	if err != nil {
		t.Fatal(err)
	}
	dewy.root = root

	dewy.registry = &mockRegistry{
		url: artifact,
	}
	dewy.artifact = &mockArtifact{
		binary: binary,
		url:    artifact,
	}
	notifyInstance, err := notifier.New(context.Background(), "", testLogger().Logger)
	if err != nil {
		t.Fatal(err)
	}
	dewy.notifier = notifyInstance

	if err := dewy.Run(); err != nil {
		t.Error(err)
	}

	if fi, err := os.Stat(filepath.Join(root, "current")); err != nil || !fi.IsDir() {
		t.Errorf("current directory is not found: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, "current", binary)); err != nil {
		t.Errorf("current dewy binary is not found: %v", err)
	}

	if fi, err := os.Stat(filepath.Join(root, "releases")); err != nil || !fi.IsDir() {
		t.Errorf("releases directory is not found: %v", err)
	}
}

func TestDeployHook(t *testing.T) {
	artifact := "ghr://linyows/dewy/tag/v1.2.3/artifact.zip"
	registry := "ghr://linyows/dewy"

	tests := []struct {
		name               string
		beforeHook         string
		afterHook          string
		executedBeforeHook bool
		executedAfterHook  bool
	}{
		{"execute a hook before run", "touch before", "", true, false},
		{"execute a hook after run", "", "touch after", false, true},
		{"execute both the before hook and after hook", "touch before", "touch after", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			c := DefaultConfig()
			c.Command = ASSETS
			if tt.beforeHook != "" {
				c.BeforeDeployHook = tt.beforeHook
			}
			if tt.afterHook != "" {
				c.AfterDeployHook = tt.afterHook
			}
			c.Registry = registry
			c.Cache = CacheConfig{
				Type:       FILE,
				Expiration: 10,
			}
			dewy, err := New(c, testLogger())
			if err != nil {
				t.Fatal(err)
			}
			dewy.registry = &mockRegistry{
				url: artifact,
			}
			dewy.artifact = &mockArtifact{
				binary: "dewy",
				url:    artifact,
			}
			dewy.notifier, err = notifier.New(context.Background(), "", testLogger().Logger)
			if err != nil {
				t.Fatal(err)
			}
			dewy.root = root
			_ = dewy.Run()
			if _, err := os.Stat(filepath.Join(root, "before")); err != nil {
				if tt.executedBeforeHook {
					t.Errorf("before hook is not executed: %v", err)
				}
			} else {
				if !tt.executedBeforeHook {
					t.Error("before hook is executed")
				}
			}
			if _, err := os.Stat(filepath.Join(root, "after")); err != nil {
				if tt.executedAfterHook {
					t.Errorf("after hook is not executed: %v", err)
				}
			} else {
				if !tt.executedAfterHook {
					t.Error("after hook is executed")
				}
			}
		})
	}
}

func TestHandleError(t *testing.T) {
	ctx := context.Background()
	c := DefaultConfig()
	c.Registry = "ghr://linyows/dewy"

	dewy, err := New(c, testLogger())
	if err != nil {
		t.Fatal(err)
	}

	mockNotify := &mockNotify{messages: []string{}}
	dewy.notifier = mockNotify

	// Test error notification limiting
	testErr := fmt.Errorf("test error")

	// First 2 errors should send normal notifications
	for i := 1; i < 3; i++ {
		dewy.notifier.SendError(ctx, testErr)
		messages := mockNotify.GetMessages()
		if len(messages) != i {
			t.Errorf("Expected %d messages, got %d", i, len(messages))
		}
		if mockNotify.errorCount != i {
			t.Errorf("Expected error count %d, got %d", i, mockNotify.errorCount)
		}
	}

	// The 3rd error should send final notification with warning
	dewy.notifier.SendError(ctx, testErr)
	messages := mockNotify.GetMessages()
	if len(messages) != 3 {
		t.Errorf("Expected %d messages (including final notification), got %d", 3, len(messages))
	}
	if mockNotify.errorCount != 3 {
		t.Errorf("Expected error count %d, got %d", 3, mockNotify.errorCount)
	}

	// Check that final notification contains the warning message
	finalMessage := messages[len(messages)-1]
	expectedWarning := "⚠️ No more error notifications will be sent until errors are resolved."
	if !strings.Contains(finalMessage, expectedWarning) {
		t.Errorf("Final notification should contain warning message, got: %s", finalMessage)
	}

	// Further errors should not send more notifications
	for i := 4; i <= 6; i++ {
		dewy.notifier.SendError(ctx, testErr)
		messages := mockNotify.GetMessages()
		if len(messages) != 3 {
			t.Errorf("Expected %d messages (no new notifications), got %d", 3, len(messages))
		}
		if mockNotify.errorCount != i {
			t.Errorf("Expected error count %d, got %d", i, mockNotify.errorCount)
		}
	}
}

func TestResetErrorCount(t *testing.T) {
	ctx := context.Background()
	c := DefaultConfig()
	c.Registry = "ghr://linyows/dewy"

	dewy, err := New(c, testLogger())
	if err != nil {
		t.Fatal(err)
	}

	mockNotify := &mockNotify{messages: []string{}}
	dewy.notifier = mockNotify

	// Set error count to 5
	testErr := fmt.Errorf("test error")
	for i := 0; i < 5; i++ {
		dewy.notifier.SendError(ctx, testErr)
	}

	if mockNotify.errorCount != 5 {
		t.Errorf("Expected error count 5, got %d", mockNotify.errorCount)
	}

	// Reset error count
	dewy.notifier.ResetErrorCount()

	if mockNotify.errorCount != 0 {
		t.Errorf("Expected error count 0 after reset, got %d", mockNotify.errorCount)
	}

	// Test that after reset, notifications work again
	dewy.notifier.SendError(ctx, testErr)
	messages := mockNotify.GetMessages()
	expectedMessages := 3 + 1 // 3 from before + 1 new notification after reset
	if len(messages) != expectedMessages {
		t.Errorf("Expected %d messages after reset, got %d", expectedMessages, len(messages))
	}
}

func TestErrorCountOverflow(t *testing.T) {
	ctx := context.Background()
	c := DefaultConfig()
	c.Registry = "ghr://linyows/dewy"

	dewy, err := New(c, testLogger())
	if err != nil {
		t.Fatal(err)
	}

	mockNotify := &mockNotify{messages: []string{}}
	dewy.notifier = mockNotify

	// Manually set error count to near the limit
	mockNotify.errorCount = 999

	testErr := fmt.Errorf("test error")

	// This should increment to 1000 (max limit)
	dewy.notifier.SendError(ctx, testErr)
	if mockNotify.errorCount != 1000 {
		t.Errorf("Expected error count 1000, got %d", mockNotify.errorCount)
	}

	// This should NOT increment beyond 1000
	dewy.notifier.SendError(ctx, testErr)
	if mockNotify.errorCount != 1000 {
		t.Errorf("Expected error count to remain at 1000, got %d", mockNotify.errorCount)
	}
}

func TestDewy_Run_ArtifactNotFoundGracePeriod(t *testing.T) {
	tests := []struct {
		name                  string
		releaseTime           *time.Time
		artifactName          string
		expectError           bool
		expectErrorSuppressed bool
		description           string
	}{
		{
			name:                  "artifact not found within 30min grace period - should suppress error",
			releaseTime:           func() *time.Time { t := time.Now().Add(-15 * time.Minute); return &t }(),
			artifactName:          "test-artifact",
			expectError:           true,
			expectErrorSuppressed: true,
			description:           "When artifact is not found but release is recent, error should be suppressed",
		},
		{
			name:                  "artifact not found outside 30min grace period - should return error",
			releaseTime:           func() *time.Time { t := time.Now().Add(-45 * time.Minute); return &t }(),
			artifactName:          "test-artifact",
			expectError:           true,
			expectErrorSuppressed: false,
			description:           "When artifact is not found and release is old, error should be returned",
		},
		{
			name:                  "artifact not found with nil release time - should return error",
			releaseTime:           nil,
			artifactName:          "test-artifact",
			expectError:           true,
			expectErrorSuppressed: false,
			description:           "When release time is unknown, error should be returned",
		},
		{
			name:                  "non-artifact-not-found error - should return error",
			releaseTime:           nil,
			artifactName:          "",
			expectError:           true,
			expectErrorSuppressed: false,
			description:           "Generic registry errors should not be suppressed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()

			mockReg := &mockRegistry{
				currentFunc: func(ctx context.Context) (*registry.CurrentResponse, error) {
					if tt.artifactName != "" {
						// Return ArtifactNotFoundError
						return nil, &registry.ArtifactNotFoundError{
							ArtifactName: tt.artifactName,
							ReleaseTime:  tt.releaseTime,
							Message:      "artifact not found: " + tt.artifactName,
						}
					}
					// Return generic error
					return nil, errors.New("generic registry error")
				},
			}

			config := Config{
				Command:  ASSETS,
				Registry: "ghr://test/test",
				Cache: CacheConfig{
					Type:       FILE,
					Expiration: 10,
				},
			}

			dewy, err := New(config, testLogger())
			if err != nil {
				t.Fatal(err)
			}
			dewy.root = root
			dewy.registry = mockReg

			// Set up notifier
			notifyInstance, err := notifier.New(context.Background(), "", testLogger().Logger)
			if err != nil {
				t.Fatal(err)
			}
			dewy.notifier = notifyInstance

			err = dewy.Run()

			if tt.expectError {
				if tt.expectErrorSuppressed {
					// Error should be suppressed (nil returned)
					if err != nil {
						t.Errorf("%s: Expected error to be suppressed (nil), but got: %v", tt.description, err)
					}
				} else {
					// Error should be returned
					if err == nil {
						t.Errorf("%s: Expected error to be returned, but got nil", tt.description)
					}
				}
			} else {
				if err != nil {
					t.Errorf("%s: Expected no error, but got: %v", tt.description, err)
				}
			}
		})
	}
}

func TestArtifactNotFoundError_TypeChecking(t *testing.T) {
	// Test that our custom error can be detected using errors.As
	releaseTime := time.Now()
	originalErr := &registry.ArtifactNotFoundError{
		ArtifactName: "test-artifact",
		ReleaseTime:  &releaseTime,
		Message:      "artifact not found: test-artifact",
	}

	// Test direct error type checking
	var artifactNotFoundErr *registry.ArtifactNotFoundError
	if !errors.As(originalErr, &artifactNotFoundErr) {
		t.Errorf("errors.As should detect ArtifactNotFoundError")
	}

	// Test that error implements error interface correctly
	if originalErr.Error() != "artifact not found: test-artifact" {
		t.Errorf("Error() method returned unexpected message: %s", originalErr.Error())
	}

	// Test IsWithinGracePeriod method
	if !artifactNotFoundErr.IsWithinGracePeriod(1 * time.Hour) {
		t.Errorf("Should be within grace period of 1 hour")
	}

	if artifactNotFoundErr.IsWithinGracePeriod(0) {
		t.Errorf("Should not be within grace period when grace period is 0")
	}

	// Test with old release time
	oldTime := time.Now().Add(-2 * time.Hour)
	oldErr := &registry.ArtifactNotFoundError{
		ArtifactName: "test-artifact",
		ReleaseTime:  &oldTime,
		Message:      "artifact not found: test-artifact",
	}

	if oldErr.IsWithinGracePeriod(1 * time.Hour) {
		t.Errorf("Should not be within grace period when release is 2 hours old")
	}
}

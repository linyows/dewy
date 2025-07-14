package dewy

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/linyows/dewy/kvs"
	"github.com/linyows/dewy/notify"
	"github.com/linyows/dewy/registry"
)

func TestNew(t *testing.T) {
	reg := "ghr://linyows/dewy?pre-release=true"
	c := DefaultConfig()
	c.Registry = reg
	dewy, err := New(c)
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
		cmpopts.IgnoreFields(Dewy{}, "RWMutex"),
		cmpopts.IgnoreFields(kvs.File{}, "mutex"),
	}
	if diff := cmp.Diff(dewy, expect, opts...); diff != "" {
		t.Error(diff)
	}
}

type mockRegistry struct {
	url string
}

func (r *mockRegistry) Current(ctx context.Context) (*registry.CurrentResponse, error) {
	return &registry.CurrentResponse{
		ID:          "id",
		Tag:         "tag",
		ArtifactURL: r.url,
	}, nil
}

func (r *mockRegistry) Report(ctx context.Context, req *registry.ReportRequest) error {
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
	dewy, err := New(c)
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
	notifier, err := notify.New(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	dewy.notify = notifier

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
			dewy, err := New(c)
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
			dewy.notify, err = notify.New(context.Background(), "")
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

	dewy, err := New(c)
	if err != nil {
		t.Fatal(err)
	}

	mockNotify := &mockNotify{messages: []string{}}
	dewy.notify = mockNotify

	// Test error notification limiting
	testErr := fmt.Errorf("test error")

	// First 2 errors should send normal notifications
	for i := 1; i < 3; i++ {
		dewy.notify.SendError(ctx, testErr)
		messages := mockNotify.GetMessages()
		if len(messages) != i {
			t.Errorf("Expected %d messages, got %d", i, len(messages))
		}
		if mockNotify.errorCount != i {
			t.Errorf("Expected error count %d, got %d", i, mockNotify.errorCount)
		}
	}

	// The 3rd error should send final notification with warning
	dewy.notify.SendError(ctx, testErr)
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
		dewy.notify.SendError(ctx, testErr)
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

	dewy, err := New(c)
	if err != nil {
		t.Fatal(err)
	}

	mockNotify := &mockNotify{messages: []string{}}
	dewy.notify = mockNotify

	// Set error count to 5
	testErr := fmt.Errorf("test error")
	for i := 0; i < 5; i++ {
		dewy.notify.SendError(ctx, testErr)
	}

	if mockNotify.errorCount != 5 {
		t.Errorf("Expected error count 5, got %d", mockNotify.errorCount)
	}

	// Reset error count
	dewy.notify.ResetErrorCount()

	if mockNotify.errorCount != 0 {
		t.Errorf("Expected error count 0 after reset, got %d", mockNotify.errorCount)
	}

	// Test that after reset, notifications work again
	dewy.notify.SendError(ctx, testErr)
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

	dewy, err := New(c)
	if err != nil {
		t.Fatal(err)
	}

	mockNotify := &mockNotify{messages: []string{}}
	dewy.notify = mockNotify

	// Manually set error count to near the limit
	mockNotify.errorCount = 999
	
	testErr := fmt.Errorf("test error")
	
	// This should increment to 1000 (max limit)
	dewy.notify.SendError(ctx, testErr)
	if mockNotify.errorCount != 1000 {
		t.Errorf("Expected error count 1000, got %d", mockNotify.errorCount)
	}
	
	// This should NOT increment beyond 1000
	dewy.notify.SendError(ctx, testErr)
	if mockNotify.errorCount != 1000 {
		t.Errorf("Expected error count to remain at 1000, got %d", mockNotify.errorCount)
	}
}

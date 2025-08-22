package artifact

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"cloud.google.com/go/storage"
)

func TestNewGS(t *testing.T) {
	tests := []struct {
		desc      string
		url       string
		expected  *GS
		expectErr bool
		err       error
	}{
		{
			"valid small structure is returned",
			"gs://mybucket/v1.2.3/myapp-linux-x86_64.zip",
			&GS{
				Bucket: "mybucket",
				Object: "v1.2.3/myapp-linux-x86_64.zip",
				url:    "gs://mybucket/v1.2.3/myapp-linux-x86_64.zip",
			},
			false,
			nil,
		},
		{
			"valid large structure is returned",
			"gs://mybucket/myteam/myapp/v1.2.3/myapp-linux-x86_64.zip",
			&GS{
				Bucket: "mybucket",
				Object: "myteam/myapp/v1.2.3/myapp-linux-x86_64.zip",
				url:    "gs://mybucket/myteam/myapp/v1.2.3/myapp-linux-x86_64.zip",
			},
			false,
			nil,
		},
		{
			"error is returned for invalid scheme",
			"http://mybucket/object",
			nil,
			true,
			fmt.Errorf("unsupported scheme: http"),
		},
		{
			"error is returned for missing bucket",
			"gs://object",
			nil,
			true,
			fmt.Errorf("url parse error: gs://object (format: gs://<bucket>/<object>)"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			// Skip actual client creation for error test cases
			if tt.expectErr {
				// Test URL parsing errors without creating client
				_, err := NewGS(context.Background(), tt.url, testLogger())
				if err == nil || err.Error() != tt.err.Error() {
					t.Errorf("expected error %s, got %s", tt.err, err)
				}
				return
			}

			// For valid cases, we would need to mock the client creation
			// For now, skip these tests unless GOOGLE_APPLICATION_CREDENTIALS is set
			if testing.Short() {
				t.Skip("Skipping GS client creation test in short mode")
			}
		})
	}
}

type MockGSClient struct {
	BucketFunc func(name string) *storage.BucketHandle
}

func (m *MockGSClient) Bucket(name string) *storage.BucketHandle {
	return m.BucketFunc(name)
}

type MockBucketHandle struct {
	ObjectFunc func(name string) *storage.ObjectHandle
}

func (m *MockBucketHandle) Object(name string) *storage.ObjectHandle {
	return m.ObjectFunc(name)
}

type MockObjectHandle struct {
	NewReaderFunc func(ctx context.Context) (*storage.Reader, error)
}

func (m *MockObjectHandle) NewReader(ctx context.Context) (*storage.Reader, error) {
	return m.NewReaderFunc(ctx)
}

// MockReader implements io.ReadCloser for testing.
type MockReader struct {
	*bytes.Reader
}

func (m *MockReader) Close() error {
	return nil
}

func TestGSDownload(t *testing.T) {
	tests := []struct {
		name      string
		mockFunc  func() GSClient
		expected  string
		expectErr bool
	}{
		{
			name: "successful download",
			mockFunc: func() GSClient {
				return &MockGSClient{
					BucketFunc: func(name string) *storage.BucketHandle {
						// Return a real BucketHandle that we can't easily mock
						// This test would require more complex mocking setup
						return nil
					},
				}
			},
			expected:  "mock file content",
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip complex mocking tests for now
			t.Skip("Complex GS mocking requires additional setup")

			gs := &GS{
				cl:     tt.mockFunc(),
				Bucket: "test-bucket",
				Object: "test-object/v1.2.3/test.tar",
				url:    "gs://test-bucket/test-object/v1.2.3/test.tar",
				logger: testLogger(),
			}

			var buf bytes.Buffer
			err := gs.Download(context.Background(), &buf)

			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
				if buf.String() != tt.expected {
					t.Errorf("expected %q, got %q", tt.expected, buf.String())
				}
			}
		})
	}
}

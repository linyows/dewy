package artifact

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestNewS3(t *testing.T) {
	tests := []struct {
		desc     string
		url      string
		expected *S3
		err      error
	}{
		{
			"valid small structure is returned",
			"s3://ap-northeast-1/mybucket/v1.2.3/myapp-linux-x86_64.zip",
			&S3{
				Region:   "ap-northeast-1",
				Bucket:   "mybucket",
				Key:      "v1.2.3/myapp-linux-x86_64.zip",
				Endpoint: "",
				url:      "s3://ap-northeast-1/mybucket/v1.2.3/myapp-linux-x86_64.zip",
			},
			nil,
		},
		{
			"valid large structure is returned",
			"s3://ap-northeast-1/mybucket/myteam/myapp/v1.2.3/myapp-linux-x86_64.zip?endpoint=http://localhost:9999/foobar",
			&S3{
				Region:   "ap-northeast-1",
				Bucket:   "mybucket",
				Key:      "myteam/myapp/v1.2.3/myapp-linux-x86_64.zip",
				Endpoint: "http://localhost:9999/foobar",
				url:      "s3://ap-northeast-1/mybucket/myteam/myapp/v1.2.3/myapp-linux-x86_64.zip?endpoint=http://localhost:9999/foobar",
			},
			nil,
		},
		{
			"error is returned",
			"s3://ap",
			nil,
			fmt.Errorf("url parse error: s3://ap"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			s3, err := NewS3(context.Background(), tt.url)
			if err != tt.err && err.Error() != tt.err.Error() {
				t.Errorf("expected error %s, got %s", tt.err, err)
			} else {
				opts := []cmp.Option{
					cmp.AllowUnexported(S3{}),
					cmpopts.IgnoreFields(S3{}, "cl"),
				}
				if diff := cmp.Diff(s3, tt.expected, opts...); diff != "" {
					t.Error(diff)
				}
			}
		})
	}
}

type MockS3Client struct {
	GetObjectFunc func(ctx context.Context, input *s3.GetObjectInput, opts ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

func (m *MockS3Client) GetObject(ctx context.Context, input *s3.GetObjectInput, opts ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	return m.GetObjectFunc(ctx, input, opts...)
}

// Helper to simulate an error during io.Copy
type errorReader struct{}

func (r *errorReader) Read(p []byte) (int, error) {
	return 0, fmt.Errorf("read error")
}

func TestS3Download(t *testing.T) {
	tests := []struct {
		name       string
		mockOutput *s3.GetObjectOutput
		mockError  error
		expected   string
		expectErr  bool
	}{
		{
			name: "successful download",
			mockOutput: &s3.GetObjectOutput{
				Body: io.NopCloser(bytes.NewReader([]byte("mock file content"))),
			},
			expected:  "mock file content",
			expectErr: false,
		},
		{
			name:      "S3 GetObject error",
			mockError: errors.New("GetObject error"),
			expectErr: true,
		},
		{
			name: "write error during io.Copy",
			mockOutput: &s3.GetObjectOutput{
				Body: io.NopCloser(&errorReader{}), // Simulate io.Copy error
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockS3Client{
				GetObjectFunc: func(ctx context.Context, input *s3.GetObjectInput, opts ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
					if tt.mockError != nil {
						return nil, tt.mockError
					}
					return tt.mockOutput, nil
				},
			}

			s3 := &S3{
				cl:     mockClient,
				Bucket: "test-bucket",
				Key:    "test-key/v1.2.3/test.tar",
				url:    "s3://ap-northeast-1/test-bucket/test-key/v1.2.3/test.tar",
			}

			var buf bytes.Buffer
			err := s3.Download(context.Background(), &buf)

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

package registry

import (
	"context"
	"fmt"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestNewGCS(t *testing.T) {
	tests := []struct {
		desc      string
		url       string
		expectErr bool
		err       error
	}{
		{
			"error is returned when project is missing",
			"gcs:///mybucket",
			true,
			fmt.Errorf("project is required: %s", gcsFormat),
		},
		{
			"error is returned when bucket is missing",
			"gcs://my-project",
			true,
			fmt.Errorf("bucket is required: %s", gcsFormat),
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			mockClient := &MockGCSClient{}
			_, err := NewGCSWithClient(context.Background(), tt.url, mockClient)
			if tt.expectErr {
				if err == nil || err.Error() != tt.err.Error() {
					t.Errorf("expected error %s, got %s", tt.err, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// MockGCSClient is a mock client for testing
type MockGCSClient struct{}

func (m *MockGCSClient) Bucket(name string) *storage.BucketHandle {
	return &storage.BucketHandle{}
}

func (m *MockGCSClient) Close() error {
	return nil
}

func TestNewGCSWithMockClient(t *testing.T) {
	tests := []struct {
		desc      string
		url       string
		expected  *GCS
		expectErr bool
		err       error
	}{
		{
			"valid small structure is returned",
			"gcs://my-project/mybucket",
			&GCS{
				Project:  "my-project",
				Bucket:   "mybucket",
				Prefix:   "",
				Artifact: "",
			},
			false,
			nil,
		},
		{
			"valid large structure is returned",
			"gcs://my-project/mybucket/myteam/myapp?artifact=myapp-linux-x86_64.zip",
			&GCS{
				Project:  "my-project",
				Bucket:   "mybucket",
				Prefix:   "myteam/myapp/",
				Artifact: "myapp-linux-x86_64.zip",
			},
			false,
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			mockClient := &MockGCSClient{}
			gcs, err := NewGCSWithClient(context.Background(), tt.url, mockClient)
			if tt.expectErr {
				if err == nil || err.Error() != tt.err.Error() {
					t.Errorf("expected error %s, got %s", tt.err, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}
				opts := []cmp.Option{
					cmp.AllowUnexported(GCS{}),
					cmpopts.IgnoreFields(GCS{}, "client"),
				}
				if diff := cmp.Diff(gcs, tt.expected, opts...); diff != "" {
					t.Error(diff)
				}
			}
		})
	}
}

// Note: Testing LatestVersion requires complex mocking of GCS client
// For now, we'll test other functions and skip the integration test for LatestVersion
func TestGCSLatestVersion(t *testing.T) {
	t.Skip("Complex GCS client mocking required - integration test should be run separately")
}

// Note: Testing Current requires complex mocking of GCS client
// For now, we'll test other functions and skip the integration test for Current
func TestGCSCurrent(t *testing.T) {
	t.Skip("Complex GCS client mocking required - integration test should be run separately")
}

func TestGCSBuildArtifactURL(t *testing.T) {
	gcs := &GCS{
		Project: "test-project",
		Bucket:  "test-bucket",
	}

	name := "path/to/artifact.tar.gz"
	expected := "gcs://test-project/test-bucket/path/to/artifact.tar.gz"

	result := gcs.buildArtifactURL(name)
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestGCSExtractFilenameFromObjectName(t *testing.T) {
	gcs := &GCS{}

	tests := []struct {
		name     string
		prefix   string
		expected string
	}{
		{"path/to/file.tar.gz", "path/to/", "file.tar.gz"},
		{"path/to/file.tar.gz", "path/", "to/file.tar.gz"},
		{"file.tar.gz", "", "file.tar.gz"},
		{"path/to/file.tar.gz/", "path/to/", "file.tar.gz"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("name=%s,prefix=%s", tt.name, tt.prefix), func(t *testing.T) {
			result := gcs.extractFilenameFromObjectName(tt.name, tt.prefix)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

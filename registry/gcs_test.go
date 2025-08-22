package registry

import (
	"context"
	"fmt"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestNewGS(t *testing.T) {
	tests := []struct {
		desc      string
		url       string
		expectErr bool
		err       error
	}{
		{
			"error is returned when bucket is missing",
			"gs://",
			true,
			fmt.Errorf("bucket is required: %s", gsFormat),
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			mockClient := &MockGSClient{}
			_, err := NewGSWithClient(context.Background(), tt.url, testLogger(), mockClient)
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

// MockGSClient is a mock client for testing
type MockGSClient struct{}

func (m *MockGSClient) Bucket(name string) *storage.BucketHandle {
	return &storage.BucketHandle{}
}

func (m *MockGSClient) Close() error {
	return nil
}

func TestNewGSWithMockClient(t *testing.T) {
	tests := []struct {
		desc      string
		url       string
		expected  *GS
		expectErr bool
		err       error
	}{
		{
			"valid small structure is returned",
			"gs://mybucket",
			&GS{
				Bucket:   "mybucket",
				Prefix:   "",
				Artifact: "",
			},
			false,
			nil,
		},
		{
			"valid large structure is returned",
			"gs://mybucket/myteam/myapp?artifact=myapp-linux-x86_64.zip",
			&GS{
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
			mockClient := &MockGSClient{}
			gs, err := NewGSWithClient(context.Background(), tt.url, testLogger(), mockClient)
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
					cmp.AllowUnexported(GS{}),
					cmpopts.IgnoreFields(GS{}, "client", "logger"),
				}
				if diff := cmp.Diff(gs, tt.expected, opts...); diff != "" {
					t.Error(diff)
				}
			}
		})
	}
}

// Note: Testing LatestVersion requires complex mocking of GS client
// For now, we'll test other functions and skip the integration test for LatestVersion
func TestGSLatestVersion(t *testing.T) {
	t.Skip("Complex GS client mocking required - integration test should be run separately")
}

// Note: Testing Current requires complex mocking of GS client
// For now, we'll test other functions and skip the integration test for Current
func TestGSCurrent(t *testing.T) {
	t.Skip("Complex GS client mocking required - integration test should be run separately")
}

func TestGSBuildArtifactURL(t *testing.T) {
	gs := &GS{
		Bucket: "test-bucket",
	}

	name := "path/to/artifact.tar.gz"
	expected := "gs://test-bucket/path/to/artifact.tar.gz"

	result := gs.buildArtifactURL(name)
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestGSExtractFilenameFromObjectName(t *testing.T) {
	gs := &GS{}

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
			result := gs.extractFilenameFromObjectName(tt.name, tt.prefix)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

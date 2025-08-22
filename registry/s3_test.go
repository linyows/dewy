package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	awslogging "github.com/aws/smithy-go/logging"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/linyows/dewy/logging"
)

func TestNewS3(t *testing.T) {
	tests := []struct {
		desc      string
		url       string
		expected  *S3
		expectErr bool
		err       error
	}{
		{
			"valid small structure is returned",
			"s3://ap-northeast-1/mybucket",
			&S3{
				Region:   "ap-northeast-1",
				Bucket:   "mybucket",
				Prefix:   "",
				Endpoint: "",
				Artifact: "",
			},
			false,
			nil,
		},
		{
			"valid large structure is returned",
			"s3://ap-northeast-1/mybucket/myteam/myapp?endpoint=http://localhost:9999/foobar&artifact=myapp-linux-x86_64.zip",
			&S3{
				Region:   "ap-northeast-1",
				Bucket:   "mybucket",
				Prefix:   "myteam/myapp/",
				Endpoint: "http://localhost:9999/foobar",
				Artifact: "myapp-linux-x86_64.zip",
			},
			false,
			nil,
		},
		{
			"error is returned",
			"s3://ap",
			nil,
			true,
			fmt.Errorf("bucket is required: %s", s3Format),
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			s3, err := NewS3(context.Background(), tt.url, testLogger())
			if tt.expectErr {
				if err == nil || err.Error() != tt.err.Error() {
					t.Errorf("expected error %s, got %s", tt.err, err)
				}
			} else {
				opts := []cmp.Option{
					cmp.AllowUnexported(S3{}),
					cmpopts.IgnoreFields(S3{}, "cl", "logger"),
				}
				if diff := cmp.Diff(s3, tt.expected, opts...); diff != "" {
					t.Error(diff)
				}
			}
		})
	}
}

type MockS3Client struct {
	// For LatestVersion test
	CommonPrefixes [][]types.CommonPrefix
	PageIndex      int
}

func (m *MockS3Client) PutObject(ctx context.Context, input *s3.PutObjectInput, opts ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	return &s3.PutObjectOutput{}, nil
}

func (m *MockS3Client) ListObjectsV2(ctx context.Context, input *s3.ListObjectsV2Input, opts ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	// Return empty result for getVersionDirectoryCreatedAt calls
	return &s3.ListObjectsV2Output{
		Contents: []types.Object{},
	}, nil
}

type MockListObjectsV2Pager struct {
	// Pages   [][]types.Object
	// Pages   []*s3.ListObjectsV2Output
	Pages     [][]types.CommonPrefix
	PageIndex int
}

func (m *MockListObjectsV2Pager) HasMorePages() bool {
	return m.PageIndex < len(m.Pages)
}

func (m *MockListObjectsV2Pager) NextPage(ctx context.Context, opts ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	if !m.HasMorePages() {
		return &s3.ListObjectsV2Output{}, nil
	}
	page := m.Pages[m.PageIndex]
	m.PageIndex++
	return &s3.ListObjectsV2Output{
		// Contents: page,
		CommonPrefixes: page,
	}, nil
}

func TestS3LatestVersion(t *testing.T) {
	data := [][]types.CommonPrefix{
		{
			{Prefix: aws.String("your/path/v1.0.0/")},
			{Prefix: aws.String("your/path/v1.2.0/")},
			{Prefix: aws.String("your/path/v3.2.1-rc.1/")},
			{Prefix: aws.String("your/path/v3.2.2-beta.10/")},
			{Prefix: aws.String("your/path/v0.0.1/")},
		},
		{
			{Prefix: aws.String("your/path/v1.2.3/")},
			{Prefix: aws.String("your/path/v1.1.0/")},
			{Prefix: aws.String("your/path/3.2.1/")},
			{Prefix: aws.String("your/path/foobar.tar.gz")},
		},
	}

	tests := []struct {
		desc           string
		pre            bool
		expectedPrefix string
		expectedVer    string
	}{
		{"pre-release is enabled", true, "your/path/v3.2.2-beta.10/", "v3.2.2-beta.10"},
		{"pre-release is disabled", false, "your/path/3.2.1/", "3.2.1"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {

			s3 := &S3{
				Bucket:     "foobar",
				Prefix:     "your/path/",
				PreRelease: tt.pre,
				// If you create a mocking object outside of iteration,
				// the pageindex will be updated and the page will become 0 from the second time onwards, so create it during iteration.
				pager: &MockListObjectsV2Pager{Pages: data},
				cl:    &MockS3Client{},
			}

			gotPrefix, gotVer, err := s3.LatestVersion(context.Background())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotPrefix != tt.expectedPrefix {
				t.Errorf("expected latest version key %s, got %s", tt.expectedPrefix, gotPrefix)
			}
			if gotVer.String() != tt.expectedVer {
				t.Errorf("expected latest version key %s, got %s", tt.expectedVer, gotVer)
			}
		})
	}
}

func TestCustomLogger(t *testing.T) {
	tests := []struct {
		name           string
		classification awslogging.Classification
		message        string
		expectedLevel  string
		checkTime      bool
	}{
		{
			name:           "Warn level with AWS SDK message",
			classification: awslogging.Warn,
			message:        "Response has no supported checksum. Not validating response payload.",
			expectedLevel:  "WARN",
			checkTime:      true,
		},
		{
			name:           "Debug level",
			classification: awslogging.Debug,
			message:        "test message",
			expectedLevel:  "DEBUG",
			checkTime:      false,
		},
		{
			name:           "Info level (default)",
			classification: awslogging.Classification("unknown"),
			message:        "test message",
			expectedLevel:  "INFO",
			checkTime:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			log := logging.SetupLogger("DEBUG", "json", &buf)
			awsLogger := &customLogger{Logger: log}

			awsLogger.Logf(tt.classification, "%s", tt.message)

			output := buf.String()
			var logEntry map[string]interface{}
			if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &logEntry); err != nil {
				t.Errorf("Failed to parse JSON log output: %v", err)
			}

			// Verify log level
			if logEntry["level"] != tt.expectedLevel {
				t.Errorf("Expected level %s, got %v", tt.expectedLevel, logEntry["level"])
			}

			// Verify message
			if logEntry["msg"] != tt.message {
				t.Errorf("Expected msg '%s', got %v", tt.message, logEntry["msg"])
			}

			// Verify time field is present (for the main test case)
			if tt.checkTime && logEntry["time"] == nil {
				t.Error("Expected time to be present")
			}
		})
	}
}

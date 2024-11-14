package registry

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

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

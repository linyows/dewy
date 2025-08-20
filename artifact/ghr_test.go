package artifact

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/go-github/v73/github"
	"github.com/linyows/dewy/client"
	"github.com/migueleliasweb/go-github-mock/src/mock"
)

func TestNewGHR(t *testing.T) {
	if os.Getenv("GITHUB_TOKEN") == "" {
		t.Skip("GITHUB_TOKEN is not set")
	}

	tests := []struct {
		desc      string
		url       string
		expected  *GHR
		expectErr bool
		err       error
	}{
		{
			"valid structure is returned",
			"ghr://linyows/dewy/tag/v1.2.3/myapp-linux-x86_64.zip",
			&GHR{
				owner:    "linyows",
				repo:     "dewy",
				tag:      "v1.2.3",
				artifact: "myapp-linux-x86_64.zip",
				url:      "ghr://linyows/dewy/tag/v1.2.3/myapp-linux-x86_64.zip",
			},
			false,
			nil,
		},
		{
			"error is returned",
			"ghr://foo",
			nil,
			true,
			fmt.Errorf("invalid artifact url: ghr://foo, []string{\"foo\"}"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			s3, err := NewGHR(context.Background(), tt.url, testLogger())
			if tt.expectErr {
				if err == nil || err.Error() != tt.err.Error() {
					t.Errorf("expected error %s, got %s", tt.err, err)
				}
			} else {
				opts := []cmp.Option{
					cmp.AllowUnexported(GHR{}),
					cmpopts.IgnoreFields(GHR{}, "cl", "logger"),
				}
				if diff := cmp.Diff(s3, tt.expected, opts...); diff != "" {
					t.Error(diff)
				}
			}
		})
	}
}

func TestGHRDownload(t *testing.T) {
	tests := []struct {
		name           string
		mockClient     *http.Client
		expectedOutput string
		expectErr      bool
	}{
		{
			name: "successful download",
			mockClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetReposReleasesByOwnerByRepo,
					[]github.RepositoryRelease{
						{
							TagName: github.String("v1.0.0"),
							Assets: []*github.ReleaseAsset{
								{
									ID:   github.Int64(12345),
									Name: github.String("artifact.zip"),
								},
							},
						},
					},
				),
				mock.WithRequestMatch(
					mock.GetReposReleasesAssetsByOwnerByRepoByAssetId,
					[]byte("mock content"),
				),
			),
			expectedOutput: "mock content",
			expectErr:      false,
		},
		{
			name: "artifact not found",
			mockClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetReposReleasesByOwnerByRepo,
					[]github.RepositoryRelease{
						{
							TagName: github.String("v1.0.0"),
							Assets: []*github.ReleaseAsset{
								{
									ID:   github.Int64(12345),
									Name: github.String("other-artifact.zip"),
								},
							},
						},
					},
				),
			),
			expectedOutput: "",
			expectErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cl := client.NewMockGitHub(tt.mockClient)

			ghr := &GHR{
				owner:    "test-owner",
				repo:     "test-repo",
				tag:      "v1.0.0",
				artifact: "artifact.zip",
				cl:       cl,
				logger:   testLogger(),
			}

			var buf bytes.Buffer
			err := ghr.Download(context.Background(), &buf)

			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("expected no error but got %v", err)
				}
				if buf.String() != tt.expectedOutput {
					t.Errorf("expected %q but got %q", tt.expectedOutput, buf.String())
				}
			}
		})
	}
}

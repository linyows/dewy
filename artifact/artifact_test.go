package artifact

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// testLogger creates a logger that discards output for testing
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestNew(t *testing.T) {
	if os.Getenv("GITHUB_TOKEN") == "" {
		t.Skip("GITHUB_TOKEN is not set")
	}

	tests := []struct {
		desc string
		url  string
		want Artifact
	}{
		{
			"use Github Releases",
			"ghr://linyows/dewy/tag/v1.2.3/dewy-linux-x86_64.tar.gz",
			&GHR{
				owner:    "linyows",
				repo:     "dewy",
				tag:      "v1.2.3",
				artifact: "dewy-linux-x86_64.tar.gz",
				url:      "ghr://linyows/dewy/tag/v1.2.3/dewy-linux-x86_64.tar.gz",
			},
		},
		{
			"use AWS S3",
			"s3://ap-northeast-1/mybucket/myapp/v1.2.3/dewy-linux-x86_64.tar.gz?endpoint=http://localhost:9000/api",
			&S3{
				Region:   "ap-northeast-1",
				Bucket:   "mybucket",
				Key:      "myapp/v1.2.3/dewy-linux-x86_64.tar.gz",
				Endpoint: "http://localhost:9000/api",
				url:      "s3://ap-northeast-1/mybucket/myapp/v1.2.3/dewy-linux-x86_64.tar.gz?endpoint=http://localhost:9000/api",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got, err := New(context.Background(), tt.url, testLogger())
			if err != nil {
				t.Fatal(err)
			} else {
				opts := []cmp.Option{
					cmp.AllowUnexported(GHR{}, S3{}),
					cmpopts.IgnoreFields(GHR{}, "cl", "logger"),
					cmpopts.IgnoreFields(S3{}, "cl", "logger"),
				}
				if diff := cmp.Diff(got, tt.want, opts...); diff != "" {
					t.Error(diff)
				}
			}
		})
	}
}

package registry

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/k1LoW/grpcstub"
)

// testLogger creates a logger that discards output for testing
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}


func TestNew(t *testing.T) {
	if os.Getenv("GITHUB_TOKEN") == "" {
		t.Skip("GITHUB_TOKEN is not set")
	}

	ts := grpcstub.NewServer(t, "dewy.proto")
	t.Cleanup(func() {
		ts.Close()
	})
	tests := []struct {
		urlstr  string
		want    Registry
		wantErr bool
	}{
		{
			"ghr://linyows/dewy",
			func(t *testing.T) Registry {
				return &GHR{
					Owner: "linyows",
					Repo:  "dewy",
				}
			}(t),
			false,
		},
		{
			"ghr://linyows/dewy?artifact=dewy_linux_amd64",
			func(t *testing.T) Registry {
				return &GHR{
					Owner:    "linyows",
					Repo:     "dewy",
					Artifact: "dewy_linux_amd64",
				}
			}(t),
			false,
		},
		{
			"ghr://linyows/dewy?artifact=dewy_linux_amd64&pre-release=true",
			func(t *testing.T) Registry {
				return &GHR{
					Owner:      "linyows",
					Repo:       "dewy",
					Artifact:   "dewy_linux_amd64",
					PreRelease: true,
				}
			}(t),
			false,
		},
		{
			"s3://ap-northeast-3/dewy/foo/bar/baz?pre-release=true",
			func(t *testing.T) Registry {
				return &S3{
					Bucket:     "dewy",
					Prefix:     "foo/bar/baz/",
					Region:     "ap-northeast-3",
					PreRelease: true,
				}
			}(t),
			false,
		},
		{
			"s3://ap-northeast-1/dewy/",
			func(t *testing.T) Registry {
				return &S3{
					Bucket:     "dewy",
					Prefix:     "",
					Region:     "ap-northeast-1",
					PreRelease: false,
				}
			}(t),
			false,
		},
		{
			fmt.Sprintf("grpc://%s?no-tls=true", ts.Addr()),
			func(t *testing.T) Registry {
				return &GRPC{
					Target: ts.Addr(),
					NoTLS:  true,
				}
			}(t),
			false,
		},
		{
			"invalid://linyows/dewy",
			nil,
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.urlstr, func(t *testing.T) {
			ctx := context.Background()
			got, err := New(ctx, tt.urlstr, testLogger())
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			opts := []cmp.Option{
				cmp.AllowUnexported(GHR{}, GRPC{}, S3{}),
				cmpopts.IgnoreFields(GHR{}, "cl", "logger"),
				cmpopts.IgnoreFields(S3{}, "cl", "logger"),
				cmpopts.IgnoreFields(GRPC{}, "cl"),
			}
			if diff := cmp.Diff(got, tt.want, opts...); diff != "" {
				t.Error(diff)
			}
		})
	}
}

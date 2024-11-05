package registry

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/k1LoW/grpcstub"
)

func TestNew(t *testing.T) {
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
				return &GithubRelease{
					Owner: "linyows",
					Repo:  "dewy",
				}
			}(t),
			false,
		},
		{
			"ghr://linyows/dewy?artifact=dewy_linux_amd64",
			func(t *testing.T) Registry {
				return &GithubRelease{
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
				return &GithubRelease{
					Owner:      "linyows",
					Repo:       "dewy",
					Artifact:   "dewy_linux_amd64",
					PreRelease: true,
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
			got, err := New(tt.urlstr)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			opts := []cmp.Option{
				cmp.AllowUnexported(GithubRelease{}, GRPC{}),
				cmpopts.IgnoreFields(GithubRelease{}, "cl"),
				cmpopts.IgnoreFields(GRPC{}, "cl"),
			}
			if diff := cmp.Diff(got, tt.want, opts...); diff != "" {
				t.Error(diff)
			}
		})
	}
}

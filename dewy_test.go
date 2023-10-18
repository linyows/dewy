package dewy

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/k1LoW/grpcstub"
	"github.com/linyows/dewy/kvs"
	"github.com/linyows/dewy/registry"
	ghrelease "github.com/linyows/dewy/registry/github_release"
	"github.com/linyows/dewy/registry/grpc"
)

func TestNewRegistry(t *testing.T) {
	ts := grpcstub.NewServer(t, "registry/grpc/proto/dewy.proto")
	t.Cleanup(func() {
		ts.Close()
	})
	tests := []struct {
		urlstr  string
		want    registry.Registry
		wantErr bool
	}{
		{
			"github_release://linyows/dewy",
			func(t *testing.T) registry.Registry {
				r, err := ghrelease.New(ghrelease.Config{
					Owner:      "linyows",
					Repo:       "dewy",
					Artifact:   "",
					PreRelease: false,
				})
				if err != nil {
					t.Fatal(err)
				}
				return r
			}(t),
			false,
		},
		{
			"github_release://linyows/dewy?artifact=dewy_linux_amd64",
			func(t *testing.T) registry.Registry {
				r, err := ghrelease.New(ghrelease.Config{
					Owner:      "linyows",
					Repo:       "dewy",
					Artifact:   "dewy_linux_amd64",
					PreRelease: false,
				})
				if err != nil {
					t.Fatal(err)
				}
				return r
			}(t),
			false,
		},
		{
			"github_release://linyows/dewy?artifact=dewy_linux_amd64&pre-release=true",
			func(t *testing.T) registry.Registry {
				r, err := ghrelease.New(ghrelease.Config{
					Owner:      "linyows",
					Repo:       "dewy",
					Artifact:   "dewy_linux_amd64",
					PreRelease: true,
				})
				if err != nil {
					t.Fatal(err)
				}
				return r
			}(t),
			false,
		},
		{
			fmt.Sprintf("grpc://%s?no-tls=true", ts.Addr()),
			func(t *testing.T) registry.Registry {
				r, err := grpc.New(grpc.Config{
					Target: ts.Addr(),
					NoTLS:  true,
				})
				if err != nil {
					t.Fatal(err)
				}
				return r
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
			got, err := newRegistry(tt.urlstr)
			if (err != nil) != tt.wantErr {
				t.Errorf("newRegistry() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			opts := []cmp.Option{
				cmp.AllowUnexported(ghrelease.GithubRelease{}, grpc.Client{}),
				cmpopts.IgnoreFields(ghrelease.GithubRelease{}, "cl"),
				cmpopts.IgnoreFields(grpc.Client{}, "cl"),
			}
			if diff := cmp.Diff(got, tt.want, opts...); diff != "" {
				t.Error(diff)
			}
		})
	}
}

func TestNew(t *testing.T) {
	regiurl := "github_release://linyows/dewy"
	c := DefaultConfig()
	c.Registry = regiurl
	c.PreRelease = true
	dewy, err := New(c)
	if err != nil {
		t.Fatal(err)
	}
	wd, _ := os.Getwd()
	r, err := newRegistry(regiurl + "?pre-release=true")
	if err != nil {
		t.Fatal(err)
	}
	expect := &Dewy{
		config: Config{
			Registry:   regiurl + "?pre-release=true",
			PreRelease: true,
			Cache: CacheConfig{
				Type:       FILE,
				Expiration: 10,
			},
			Starter: nil,
		},
		registry:        r,
		cache:           dewy.cache,
		isServerRunning: false,
		root:            wd,
	}

	opts := []cmp.Option{
		cmp.AllowUnexported(Dewy{}, ghrelease.GithubRelease{}, kvs.File{}),
		cmpopts.IgnoreFields(Dewy{}, "notice"),
		cmpopts.IgnoreFields(Dewy{}, "RWMutex"),
		cmpopts.IgnoreFields(ghrelease.GithubRelease{}, "cl"),
		cmpopts.IgnoreFields(kvs.File{}, "mutex"),
	}
	if diff := cmp.Diff(dewy, expect, opts...); diff != "" {
		t.Error(diff)
	}
}

func TestRun(t *testing.T) {
	if os.Getenv("GITHUB_TOKEN") == "" {
		t.Skip("GITHUB_TOKEN is not set")
	}
	root := t.TempDir()
	c := DefaultConfig()
	c.Command = ASSETS
	c.Registry = "github_release://linyows/dewy"
	c.Cache = CacheConfig{
		Type:       FILE,
		Expiration: 10,
	}
	dewy, err := New(c)
	if err != nil {
		t.Fatal(err)
	}
	dewy.root = root
	dewy.disableReport = true
	if err := dewy.Run(); err != nil {
		t.Error(err)
	}

	if fi, err := os.Stat(filepath.Join(root, "current")); err != nil || !fi.IsDir() {
		t.Errorf("current directory is not found: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, "current", "dewy")); err != nil {
		t.Errorf("current dewy binary is not found: %v", err)
	}

	if fi, err := os.Stat(filepath.Join(root, "releases")); err != nil || !fi.IsDir() {
		t.Errorf("releases directory is not found: %v", err)
	}
}

func TestDeployHook(t *testing.T) {
	if os.Getenv("GITHUB_TOKEN") == "" {
		t.Skip("GITHUB_TOKEN is not set")
	}
	tests := []struct {
		registry           string
		beforeHook         string
		executedBeforeHook bool
		executedAfterHook  bool
	}{
		{"github_release://linyows/dewy", "touch before", true, true},
		{"github_release://linyows/invalid", "touch before", false, false},
		{"github_release://linyows/dewy", "touch before && invalid command", true, false},
	}
	for _, tt := range tests {
		t.Run(tt.registry, func(t *testing.T) {
			root := t.TempDir()
			c := DefaultConfig()
			c.Command = ASSETS
			c.BeforeDeployHook = tt.beforeHook
			c.AfterDeployHook = "touch after"
			c.Registry = tt.registry
			c.Cache = CacheConfig{
				Type:       FILE,
				Expiration: 10,
			}
			dewy, err := New(c)
			if err != nil {
				t.Fatal(err)
			}
			dewy.root = root
			dewy.disableReport = true
			_ = dewy.Run()
			if _, err := os.Stat(filepath.Join(root, "before")); err != nil {
				if tt.executedBeforeHook {
					t.Errorf("before hook is not executed: %v", err)
				}
			} else {
				if !tt.executedBeforeHook {
					t.Error("before hook is executed")
				}
			}
			if _, err := os.Stat(filepath.Join(root, "after")); err != nil {
				if tt.executedAfterHook {
					t.Errorf("after hook is not executed: %v", err)
				}
			} else {
				if !tt.executedAfterHook {
					t.Error("after hook is executed")
				}
			}
		})
	}
}

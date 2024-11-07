package dewy

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/linyows/dewy/kvs"
	"github.com/linyows/dewy/registry"
)

func TestNew(t *testing.T) {
	if os.Getenv("GITHUB_TOKEN") == "" {
		t.Skip("GITHUB_TOKEN is not set")
	}

	reg := "ghr://linyows/dewy?pre-release=true"
	c := DefaultConfig()
	c.Registry = reg
	c.PreRelease = true
	dewy, err := New(c)
	if err != nil {
		t.Fatal(err)
	}
	wd, _ := os.Getwd()
	ctx := context.Background()
	r, err := registry.New(ctx, reg)
	if err != nil {
		t.Fatal(err)
	}

	expect := &Dewy{
		config: Config{
			Registry:   reg,
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
		cmp.AllowUnexported(Dewy{}, registry.GHR{}, kvs.File{}),
		cmpopts.IgnoreFields(Dewy{}, "notify"),
		cmpopts.IgnoreFields(Dewy{}, "RWMutex"),
		cmpopts.IgnoreFields(registry.GHR{}, "cl"),
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
	c.Registry = "ghr://linyows/dewy"
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
		{"ghr://linyows/dewy", "touch before", true, true},
		{"ghr://linyows/invalid", "touch before", false, false},
		{"ghr://linyows/dewy", "touch before && invalid command", true, false},
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

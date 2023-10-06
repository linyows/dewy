package dewy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/linyows/dewy/kvs"
	"github.com/linyows/dewy/repo"
)

func TestNew(t *testing.T) {
	dewy, err := New(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	wd, _ := os.Getwd()
	c := Config{
		Repository: repo.Config{},
		Cache: CacheConfig{
			Type:       FILE,
			Expiration: 10,
		},
		Starter: nil,
	}
	r, err := repo.NewGithubRelease(c.Repository)
	if err != nil {
		t.Fatal(err)
	}
	expect := &Dewy{
		config:          c,
		registory:       r,
		cache:           dewy.cache,
		isServerRunning: false,
		root:            wd,
	}

	opts := []cmp.Option{
		cmp.AllowUnexported(Dewy{}, repo.GithubRelease{}, kvs.File{}),
		cmpopts.IgnoreFields(Dewy{}, "notice"),
		cmpopts.IgnoreFields(Dewy{}, "RWMutex"),
		cmpopts.IgnoreFields(repo.GithubRelease{}, "cl"),
		cmpopts.IgnoreFields(repo.GithubRelease{}, "baseURL"),
		cmpopts.IgnoreFields(repo.GithubRelease{}, "uploadURL"),
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
	c.Repository = repo.Config{
		Owner:                 "linyows",
		Repo:                  "dewy",
		DisableRecordShipping: true,
	}
	c.Cache = CacheConfig{
		Type:       FILE,
		Expiration: 10,
	}
	dewy, err := New(c)
	if err != nil {
		t.Fatal(err)
	}
	dewy.root = root
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

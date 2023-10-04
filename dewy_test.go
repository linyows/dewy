package dewy

import (
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

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
	r, err := repo.New(c.Repository, dewy.cache)
	if err != nil {
		t.Fatal(err)
	}
	expect := &Dewy{
		config:          c,
		repo:            r,
		cache:           dewy.cache,
		isServerRunning: false,
		RWMutex:         sync.RWMutex{},
		root:            wd,
	}

	if !reflect.DeepEqual(dewy, expect) {
		t.Errorf("new return is incorrect\nexpected: \n%#v\ngot: \n%#v\n", expect, dewy)
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
		Provider:              repo.GITHUB,
		Owner:                 "linyows",
		Name:                  "dewy",
		Token:                 os.Getenv("GITHUB_TOKEN"),
		Artifact:              "dewy_darwin_x86_64.tar.gz",
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

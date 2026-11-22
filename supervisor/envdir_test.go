package supervisor

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func writeEnvFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestReadEnvDir(t *testing.T) {
	dir := t.TempDir()
	writeEnvFile(t, dir, "PLAIN", "value")
	writeEnvFile(t, dir, "FIRST_LINE", "one\ntwo\n")
	writeEnvFile(t, dir, "TRAILING", "value \t\n")
	writeEnvFile(t, dir, "NUL", "a\x00b")
	writeEnvFile(t, dir, "EMPTY", "")
	writeEnvFile(t, dir, "BLANK_LINE", "\n")
	writeEnvFile(t, dir, ".hidden", "skipped")
	if err := os.Mkdir(filepath.Join(dir, "SUBDIR"), 0700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(target, []byte("linked"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "LINKED")); err != nil {
		t.Fatal(err)
	}

	got, err := readEnvDir(dir)
	if err != nil {
		t.Fatalf("readEnvDir() error = %v", err)
	}
	want := map[string]envDirEntry{
		"PLAIN":      {value: "value"},
		"FIRST_LINE": {value: "one"},
		"TRAILING":   {value: "value"},
		"NUL":        {value: "a\nb"},
		"EMPTY":      {unset: true},
		"BLANK_LINE": {value: ""},
		"LINKED":     {value: "linked"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("readEnvDir() = %+v, want %+v", got, want)
	}
}

func TestReadEnvDirErrors(t *testing.T) {
	t.Run("empty dir disables it", func(t *testing.T) {
		if got, err := readEnvDir(""); got != nil || err != nil {
			t.Errorf("readEnvDir(\"\") = %v, %v, want nil, nil", got, err)
		}
	})
	t.Run("missing directory", func(t *testing.T) {
		if _, err := readEnvDir(filepath.Join(t.TempDir(), "absent")); err == nil {
			t.Error("readEnvDir() error = nil, want an error for a missing directory")
		}
	})
	t.Run("value too long", func(t *testing.T) {
		dir := t.TempDir()
		writeEnvFile(t, dir, "BIG", strings.Repeat("x", maxEnvValueBytes+1))
		if _, err := readEnvDir(dir); err == nil {
			t.Error("readEnvDir() error = nil, want an error for a value over the limit")
		}
	})
	t.Run("missing directory fails New", func(t *testing.T) {
		if _, err := New(Config{Command: "true", EnvDir: filepath.Join(t.TempDir(), "absent")}); err == nil {
			t.Error("New() error = nil, want an error for a missing envdir")
		}
	})
}

func TestMergeEnv(t *testing.T) {
	base := []string{"A=1", "B=2", "C=3", "A=4"}
	vars := map[string]envDirEntry{
		"B":         {value: "envdir"},
		"C":         {unset: true},
		"NEW":       {value: "new"},
		"DEWY_PORT": {value: "spoofed"},
	}
	proto := []string{"DEWY_PORT=8080=3"}

	got := mergeEnv(base, vars, proto)
	want := []string{"A=4", "B=envdir", "DEWY_PORT=8080=3", "NEW=new"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("mergeEnv() = %q, want %q", got, want)
	}
}

func TestEnvDirIsReloadedOnRestart(t *testing.T) {
	dir := t.TempDir()
	writeEnvFile(t, dir, "APP_COLOR", "blue")
	writeEnvFile(t, dir, "APP_GONE", "here")

	h := newHarness(t, "serve")
	cfg := h.config("127.0.0.1:0")
	cfg.Env = append(cfg.Env, "APP_GONE=from-env", "APP_BASE=base")
	cfg.EnvDir = dir
	s := h.start(cfg)
	addr := s.Addrs()[0]

	for name, want := range map[string]string{"APP_COLOR": "blue", "APP_GONE": "here", "APP_BASE": "base"} {
		if got := getPath(t, addr, "/env/"+name); got != want {
			t.Errorf("first worker %s = %q, want %q", name, got, want)
		}
	}

	writeEnvFile(t, dir, "APP_COLOR", "green")
	writeEnvFile(t, dir, "APP_GONE", "")
	restart := func() {
		t.Helper()
		if err := s.Restart(context.Background()); err != nil {
			t.Fatalf("Restart() error = %v", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.WaitRetired(ctx); err != nil {
			t.Fatalf("WaitRetired() error = %v", err)
		}
	}
	restart()

	for name, want := range map[string]string{"APP_COLOR": "green", "APP_GONE": "<unset>", "APP_BASE": "base"} {
		if got := getPath(t, addr, "/env/"+name); got != want {
			t.Errorf("after Restart %s = %q, want %q", name, got, want)
		}
	}

	// A file removed from the envdir no longer applies, so the value from
	// Env comes back.
	if err := os.Remove(filepath.Join(dir, "APP_GONE")); err != nil {
		t.Fatal(err)
	}
	restart()
	if got := getPath(t, addr, "/env/APP_GONE"); got != "from-env" {
		t.Errorf("after removing the file APP_GONE = %q, want %q", got, "from-env")
	}
}

func TestRestartKeepsWorkerWhenEnvDirIsUnreadable(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "env")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	h := newHarness(t, "serve")
	cfg := h.config("127.0.0.1:0")
	cfg.EnvDir = dir
	s := h.start(cfg)
	before := get(t, s.Addrs()[0])

	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if err := s.Restart(context.Background()); err == nil {
		t.Fatal("Restart() error = nil, want an error when the envdir is gone")
	}
	if after := get(t, s.Addrs()[0]); after != before {
		t.Errorf("after the failed Restart got %q, want the current worker %q", after, before)
	}
}

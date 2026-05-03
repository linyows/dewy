package dewy

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	"github.com/linyows/dewy/cache"
	starter "github.com/linyows/server-starter"
)

// deploy extracts the cached artifact into a new release directory and
// atomically swaps the "current" symlink to point at it. Before- and after-
// deploy hooks are wrapped around the extract step.
func (d *Dewy) deploy(key string) (err error) {
	ctx := context.Background()

	beforeResult, beforeErr := d.execHook(d.config.BeforeDeployHook)
	if beforeResult != nil {
		d.notifier.SendHookResult(ctx, "Before Deploy", beforeResult)
	}
	if beforeErr != nil {
		d.logger.Error("Before deploy hook failure", slog.String("error", beforeErr.Error()))
		// Continue with deploy even if before hook fails
	}

	defer func() {
		if err != nil {
			return
		}
		// When deploy is success, run after deploy hook
		afterResult, afterErr := d.execHook(d.config.AfterDeployHook)
		if afterResult != nil {
			d.notifier.SendHookResult(ctx, "After Deploy", afterResult)
		}
		if afterErr != nil {
			d.logger.Error("After deploy hook failure", slog.String("error", afterErr.Error()))
		}
	}()
	p := filepath.Join(d.cache.GetDir(), key)
	linkFrom, err := d.preserve(p)
	if err != nil {
		d.logger.Error("Preserve failure", slog.String("error", err.Error()))
		return err
	}
	d.logger.Info("Extract archive", slog.String("path", linkFrom))

	d.notifier.OnDeploy(linkFrom)

	linkTo := filepath.Join(d.root, symlinkDir)

	// Atomic symlink replacement: create temp symlink, then rename
	tmpLink := linkTo + ".tmp"
	os.Remove(tmpLink) // Ensure no stale temp link exists
	if err := os.Symlink(linkFrom, tmpLink); err != nil {
		return err
	}

	d.logger.Info("Create symlink",
		slog.String("from", linkFrom),
		slog.String("to", linkTo))
	if err := os.Rename(tmpLink, linkTo); err != nil {
		os.Remove(tmpLink) // Cleanup on failure
		return err
	}

	return nil
}

// preserve materializes the cached artifact into a timestamp-named release
// directory under d.root and returns its path.
func (d *Dewy) preserve(p string) (string, error) {
	dst := filepath.Join(d.root, releasesDir, time.Now().UTC().Format(releaseDir))
	if err := os.MkdirAll(dst, 0755); err != nil {
		return "", err
	}

	if err := cache.ExtractArchive(p, dst); err != nil {
		return "", err
	}

	return dst, nil
}

// restartServer signals the running dewy process to restart its child
// (server-starter handles the SIGHUP-driven graceful swap).
func (d *Dewy) restartServer() error {
	d.Lock()
	defer d.Unlock()

	pid := os.Getpid()
	p, _ := os.FindProcess(pid)
	err := p.Signal(syscall.SIGHUP)
	if err != nil {
		return err
	}
	d.logger.Info("Send SIGHUP for server restart", slog.String("version", d.cVer), slog.Int("pid", pid))

	return nil
}

// startServer launches the managed application via server-starter and marks
// d.isServerRunning. Errors during NewStarter are returned synchronously;
// errors from the actual Run() goroutine flip isServerRunning back to false.
func (d *Dewy) startServer() error {
	d.Lock()
	defer d.Unlock()

	d.logger.Info("Start server", slog.String("version", d.cVer))

	// Try to create starter first (synchronous validation)
	s, err := starter.NewStarter(d.config.Starter)
	if err != nil {
		d.logger.Error("Starter failure", slog.String("error", err.Error()))
		return err
	}

	// Start server in background
	go func() {
		err := s.Run()
		if err != nil {
			d.logger.Error("Server run failure", slog.String("error", err.Error()))
			d.Lock()
			d.isServerRunning = false
			d.Unlock()
		}
	}()

	d.isServerRunning = true
	return nil
}

// keepReleases prunes old release directories under d.root, retaining the
// keep most recent by modification time.
func (d *Dewy) keepReleases() error {
	dir := filepath.Join(d.root, releasesDir)
	files, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, name := range selectStaleReleases(files, keepReleases) {
		if err := os.RemoveAll(filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	return nil
}

// selectStaleReleases returns the names of release directories that should
// be removed to retain at most `keep` entries by modification time (newest
// first).
//
// Entries whose Info() errors (e.g. concurrent deletion, transient stat
// failure) are excluded from both the keep set and the stale set: keeping
// the in-memory comparator strictly ordered avoids the strict-weak-ordering
// violation that the previous sort.Slice comparator would hit when Info()
// returned an error mid-sort. Unstattable entries are simply left alone so
// a transient FS hiccup does not remove the wrong directory.
func selectStaleReleases(files []fs.DirEntry, keep int) []string {
	type entry struct {
		name    string
		modTime time.Time
	}
	stat := make([]entry, 0, len(files))
	for _, f := range files {
		info, err := f.Info()
		if err != nil {
			continue
		}
		stat = append(stat, entry{name: f.Name(), modTime: info.ModTime()})
	}

	// Newest first; tie-break on name so equal mtimes yield a deterministic
	// order even across runs.
	sort.Slice(stat, func(i, j int) bool {
		if !stat[i].modTime.Equal(stat[j].modTime) {
			return stat[i].modTime.After(stat[j].modTime)
		}
		return stat[i].name < stat[j].name
	})

	if len(stat) <= keep {
		return nil
	}
	stale := make([]string, 0, len(stat)-keep)
	for _, e := range stat[keep:] {
		stale = append(stale, e.name)
	}
	return stale
}

// cleanupOldImages removes old container images, keeping only the most recent ones.
func (d *Dewy) cleanupOldImages(ctx context.Context, imageRef string) error {
	if d.containerRuntime == nil {
		return fmt.Errorf("container runtime not initialized")
	}

	return d.containerRuntime.CleanupOldImages(ctx, imageRef, keepReleases)
}

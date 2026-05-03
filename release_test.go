package dewy

import (
	"errors"
	"io/fs"
	"slices"
	"testing"
	"time"
)

// fakeDirEntry implements os.DirEntry for retention tests, with controllable
// Info() success / failure independent of the filesystem.
type fakeDirEntry struct {
	name    string
	modTime time.Time
	infoErr error
}

func (f fakeDirEntry) Name() string                { return f.name }
func (f fakeDirEntry) IsDir() bool                 { return true }
func (f fakeDirEntry) Type() fs.FileMode           { return fs.ModeDir }
func (f fakeDirEntry) Info() (fs.FileInfo, error) {
	if f.infoErr != nil {
		return nil, f.infoErr
	}
	return fakeFileInfo{name: f.name, modTime: f.modTime}, nil
}

type fakeFileInfo struct {
	name    string
	modTime time.Time
}

func (f fakeFileInfo) Name() string       { return f.name }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() fs.FileMode  { return fs.ModeDir }
func (f fakeFileInfo) ModTime() time.Time { return f.modTime }
func (f fakeFileInfo) IsDir() bool        { return true }
func (f fakeFileInfo) Sys() any           { return nil }

// Sanity test: with all entries stattable, the oldest beyond `keep` are stale.
func TestSelectStaleReleases_NewestKept(t *testing.T) {
	now := time.Now()
	files := []fs.DirEntry{
		fakeDirEntry{name: "old1", modTime: now.Add(-3 * time.Hour)},
		fakeDirEntry{name: "newest", modTime: now},
		fakeDirEntry{name: "old2", modTime: now.Add(-2 * time.Hour)},
		fakeDirEntry{name: "mid", modTime: now.Add(-1 * time.Hour)},
	}
	got := selectStaleReleases(files, 2)
	slices.Sort(got)
	want := []string{"old1", "old2"}
	if !slices.Equal(got, want) {
		t.Errorf("stale = %v, want %v", got, want)
	}
}

// The retained set must be the same regardless of input ordering. The old
// sort.Slice comparator returned `false`/`true` on Info() errors, breaking
// strict weak ordering and producing non-deterministic results when an
// unstattable entry sat between stattable entries.
func TestSelectStaleReleases_DeterministicWithInfoErrors(t *testing.T) {
	now := time.Now()
	infoErr := errors.New("stat: file vanished")

	good := []fakeDirEntry{
		{name: "a", modTime: now.Add(-3 * time.Hour)},
		{name: "b", modTime: now.Add(-2 * time.Hour)},
		{name: "c", modTime: now.Add(-1 * time.Hour)},
		{name: "d", modTime: now},
	}
	bad := fakeDirEntry{name: "z-bad", infoErr: infoErr}

	// Two permutations that the broken comparator would sort inconsistently.
	perm1 := []fs.DirEntry{good[0], bad, good[1], good[2], good[3]}
	perm2 := []fs.DirEntry{bad, good[3], good[2], good[1], good[0]}

	stale1 := selectStaleReleases(perm1, 2)
	stale2 := selectStaleReleases(perm2, 2)
	slices.Sort(stale1)
	slices.Sort(stale2)
	if !slices.Equal(stale1, stale2) {
		t.Errorf("non-deterministic: perm1=%v perm2=%v", stale1, stale2)
	}

	// The two newest stattable entries (c, d) must be retained.
	for _, kept := range []string{"c", "d"} {
		if slices.Contains(stale1, kept) {
			t.Errorf("kept set should include %q, got stale=%v", kept, stale1)
		}
	}
}

// Unstattable entries must not push out stattable ones from the keep window.
// (Old behavior could place an unstattable entry "newer" than real entries
// and silently shorten the retention.)
func TestSelectStaleReleases_UnstattableExcludedFromKeep(t *testing.T) {
	now := time.Now()
	files := []fs.DirEntry{
		fakeDirEntry{name: "real-newest", modTime: now},
		fakeDirEntry{name: "real-oldish", modTime: now.Add(-time.Hour)},
		fakeDirEntry{name: "vanished", infoErr: errors.New("gone")},
	}
	stale := selectStaleReleases(files, 2)
	for _, kept := range []string{"real-newest", "real-oldish"} {
		if slices.Contains(stale, kept) {
			t.Errorf("real entry %q should be kept (unstattable must not steal a slot), got stale=%v", kept, stale)
		}
	}
}

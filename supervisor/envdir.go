package supervisor

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// maxEnvValueBytes bounds how much of an envdir file is read.
const maxEnvValueBytes = 128 * 1024

// envDirEntry is one variable read from an envdir: a value to set, or, for
// an empty file, a variable to remove.
type envDirEntry struct {
	value string
	unset bool
}

// readEnvDir reads dir the way daemontools envdir(8) does. Each regular file
// names a variable. Its value is the first line of the file, with trailing
// spaces and tabs removed and each NUL turned into a newline. An empty file
// removes the variable instead of setting it.
//
// Files whose names start with "." are skipped, as are directories and
// other non-regular files. A symbolic link is followed, so the files of a
// mounted secret or config map are read through their links. An empty dir
// disables the envdir and returns nil.
func readEnvDir(dir string) (map[string]envDirEntry, error) {
	if dir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read envdir: %w", err)
	}

	vars := make(map[string]envDirEntry, len(entries))
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		path := filepath.Join(dir, name)
		fi, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("read envdir: %w", err)
		}
		if !fi.Mode().IsRegular() {
			continue
		}
		if strings.Contains(name, "=") {
			return nil, fmt.Errorf("read envdir: %s: a variable name must not contain \"=\"", path)
		}
		v, err := readEnvValue(path)
		if err != nil {
			return nil, fmt.Errorf("read envdir: %w", err)
		}
		vars[name] = v
	}
	return vars, nil
}

// readEnvValue reads the variable held by the envdir file at path.
func readEnvValue(path string) (envDirEntry, error) {
	f, err := os.Open(path) //nolint:gosec // G304: the path is a file in the operator's own envdir
	if err != nil {
		return envDirEntry{}, err
	}
	defer func() { _ = f.Close() }()

	data, err := io.ReadAll(io.LimitReader(f, maxEnvValueBytes+1))
	if err != nil {
		return envDirEntry{}, fmt.Errorf("%s: %w", path, err)
	}
	if len(data) == 0 {
		return envDirEntry{unset: true}, nil
	}
	line, _, found := bytes.Cut(data, []byte("\n"))
	if !found && len(data) > maxEnvValueBytes {
		return envDirEntry{}, fmt.Errorf("%s: the first line is longer than %d bytes", path, maxEnvValueBytes)
	}
	line = bytes.TrimRight(line, " \t")
	line = bytes.ReplaceAll(line, []byte{0}, []byte("\n"))
	return envDirEntry{value: string(line)}, nil
}

// mergeEnv returns base with the envdir vars applied and then the protocol
// variables in proto, each "KEY=VALUE". A later definition of a key replaces
// an earlier one in place, so every key appears once and keeps the position
// of its first definition. The protocol variables come last, so an envdir
// cannot override them.
func mergeEnv(base []string, vars map[string]envDirEntry, proto []string) []string {
	out := make([]string, 0, len(base)+len(vars)+len(proto))
	index := make(map[string]int, cap(out))
	set := func(kv string) {
		k, _, _ := strings.Cut(kv, "=")
		if i, ok := index[k]; ok {
			out[i] = kv
			return
		}
		index[k] = len(out)
		out = append(out, kv)
	}

	for _, kv := range base {
		set(kv)
	}
	unset := make(map[string]bool)
	names := make([]string, 0, len(vars))
	for name := range vars {
		names = append(names, name)
	}
	// Sorted, so a new variable is appended in the same position on every
	// start.
	slices.Sort(names)
	for _, name := range names {
		if vars[name].unset {
			unset[name] = true
			continue
		}
		set(name + "=" + vars[name].value)
	}
	for _, kv := range proto {
		k, _, _ := strings.Cut(kv, "=")
		delete(unset, k)
		set(kv)
	}

	if len(unset) == 0 {
		return out
	}
	kept := out[:0]
	for _, kv := range out {
		k, _, _ := strings.Cut(kv, "=")
		if !unset[k] {
			kept = append(kept, kv)
		}
	}
	return kept
}

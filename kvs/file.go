package kvs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mholt/archives"
)

var (
	// DefaultCacheDir creates persistent cache dir.
	DefaultCacheDir = createPersistentCacheDir()
	// DefaultMaxSize for data size.
	DefaultMaxSize int64 = 64 * 1024 * 1024
)

func createPersistentCacheDir() string {
	var dir string

	// 1. Use DEWY_CACHEDIR if set
	if cacheDir := os.Getenv("DEWY_CACHEDIR"); cacheDir != "" {
		dir = cacheDir
	} else {
		// 2. Default: current working directory
		if pwd, err := os.Getwd(); err == nil {
			dir = filepath.Join(pwd, ".dewy", "cache")
		} else {
			// Fallback if PWD is not available
			dir = filepath.Join(".", ".dewy", "cache")
		}
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(dir, 0755); err != nil { //nolint:gosec // G703
		// If creation fails, fall back to temp directory
		tempDir, _ := os.MkdirTemp("", "dewy-")
		return tempDir
	}

	return dir
}

// File struct.
type File struct {
	items    map[string]*item //nolint
	dir      string
	mutex    sync.Mutex //nolint
	MaxItems int
	MaxSize  int64
	logger   *slog.Logger
}

// GetDir returns dir.
func (f *File) GetDir() string {
	return f.dir
}

// SetDir sets the cache directory.
func (f *File) SetDir(dir string) {
	f.dir = dir
}

// Default sets to struct.
func (f *File) Default() {
	f.dir = DefaultCacheDir
	f.MaxSize = DefaultMaxSize
}

// SetLogger sets the logger for the File instance.
func (f *File) SetLogger(logger *slog.Logger) {
	f.logger = logger
}

// validateKeyPath validates that a key resolves to a path within the base directory.
func validateKeyPath(base, key string) (string, error) {
	p := filepath.Join(base, key)
	cleanBase := filepath.Clean(base) + string(os.PathSeparator)
	if !strings.HasPrefix(filepath.Clean(p)+string(os.PathSeparator), cleanBase) && filepath.Clean(p) != filepath.Clean(base) {
		return "", fmt.Errorf("illegal key (path traversal detected): %s", key)
	}
	return p, nil
}

// Read data by key from file.
func (f *File) Read(key string) ([]byte, error) {
	p, err := validateKeyPath(f.dir, key)
	if err != nil {
		return nil, err
	}
	if !IsFileExist(p) {
		return nil, fmt.Errorf("File not found: %s", p)
	}

	content, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}

	return content, nil
}

// Write data to file.
func (f *File) Write(key string, data []byte) error {
	dirstat, err := os.Stat(f.dir)
	if err != nil {
		return err
	}

	if !dirstat.Mode().IsDir() {
		return errors.New("file.dir is not dir")
	}
	if dirstat.Size() > f.MaxSize {
		return errors.New("max size has been reached")
	}

	p, err := validateKeyPath(f.dir, key)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	defer file.Close()
	_, err = file.Write(data)
	if err != nil {
		return err
	}

	if f.logger != nil {
		f.logger.Info("Write file", slog.String("path", p))
	}

	return nil
}

// Delete data on file.
func (f *File) Delete(key string) error {
	p, err := validateKeyPath(f.dir, key)
	if err != nil {
		return err
	}
	if !IsFileExist(p) {
		return fmt.Errorf("File not found: %s", p)
	}

	if err := os.Remove(p); err != nil {
		return err
	}

	return nil
}

// List returns keys from file.
func (f *File) List() ([]string, error) {
	files, err := os.ReadDir(f.dir)
	if err != nil {
		return nil, err
	}

	var list []string
	for _, file := range files {
		list = append(list, file.Name())
	}

	return list, nil
}

// ExtractArchive extracts by archive.
func ExtractArchive(src, dst string) error {
	if !IsFileExist(src) {
		return fmt.Errorf("File not found: %s", src)
	}

	// Open the source file
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Determine the format based on file extension
	var format archives.Extractor
	switch {
	case strings.HasSuffix(strings.ToLower(src), ".tar.gz") || strings.HasSuffix(strings.ToLower(src), ".tgz"):
		format = archives.CompressedArchive{
			Compression: archives.Gz{},
			Extraction:  archives.Tar{},
		}
	case strings.HasSuffix(strings.ToLower(src), ".tar.bz2") || strings.HasSuffix(strings.ToLower(src), ".tbz2"):
		format = archives.CompressedArchive{
			Compression: archives.Bz2{},
			Extraction:  archives.Tar{},
		}
	case strings.HasSuffix(strings.ToLower(src), ".tar.xz") || strings.HasSuffix(strings.ToLower(src), ".txz"):
		format = archives.CompressedArchive{
			Compression: archives.Xz{},
			Extraction:  archives.Tar{},
		}
	case strings.HasSuffix(strings.ToLower(src), ".tar"):
		format = archives.Tar{}
	case strings.HasSuffix(strings.ToLower(src), ".zip"):
		format = archives.Zip{}
	default:
		return fmt.Errorf("unsupported archive format: %s", src)
	}

	// Extract the archive
	cleanDst := filepath.Clean(dst) + string(os.PathSeparator)

	return format.Extract(context.Background(), srcFile, func(ctx context.Context, f archives.FileInfo) error {
		// Construct the destination path and validate against path traversal (Zip Slip)
		destPath := filepath.Join(dst, f.NameInArchive)
		if !strings.HasPrefix(filepath.Clean(destPath)+string(os.PathSeparator), cleanDst) && filepath.Clean(destPath) != filepath.Clean(dst) {
			return fmt.Errorf("illegal file path in archive (path traversal detected): %s", f.NameInArchive)
		}

		// Reject symlinks to prevent symlink-based path traversal attacks
		if f.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlinks are not allowed in archive: %s", f.NameInArchive)
		}

		// Sanitize file permissions: strip setuid/setgid/sticky bits
		sanitizedMode := f.Mode().Perm() &^ (os.ModeSetuid | os.ModeSetgid | os.ModeSticky)

		// Handle directories
		if f.IsDir() {
			return os.MkdirAll(destPath, sanitizedMode)
		}

		// Create parent directories if needed
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}

		// Create and write the file with sanitized permissions
		outFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, sanitizedMode)
		if err != nil {
			return err
		}
		defer outFile.Close()

		// Copy the file content
		reader, err := f.Open()
		if err != nil {
			return err
		}
		defer reader.Close()

		_, err = outFile.ReadFrom(reader)
		return err
	})
}

// IsFileExist checks file exists.
func IsFileExist(p string) bool {
	_, err := os.Stat(p)

	return !os.IsNotExist(err)
}

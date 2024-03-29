package kvs

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/mholt/archiver/v3"
)

var (
	// DefaultTempDir creates temp dir.
	DefaultTempDir = createTempDir()
	// DefaultMaxSize for data size.
	DefaultMaxSize int64 = 64 * 1024 * 1024
)

func createTempDir() string {
	dir, _ := os.MkdirTemp("", "dewy-")
	return dir
}

// File struct.
type File struct {
	items    map[string]*item //nolint
	dir      string
	mutex    sync.Mutex //nolint
	MaxItems int
	MaxSize  int64
}

// GetDir returns dir.
func (f *File) GetDir() string {
	return f.dir
}

// Default sets to struct.
func (f *File) Default() {
	f.dir = DefaultTempDir
	f.MaxSize = DefaultMaxSize
}

// Read data by key from file.
func (f *File) Read(key string) ([]byte, error) {
	p := filepath.Join(f.dir, key)
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
		return errors.New("File.dir is not dir")
	}
	if dirstat.Size() > f.MaxSize {
		return errors.New("Max size has been reached")
	}

	p := filepath.Join(f.dir, key)
	file, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	defer file.Close()
	_, err = file.Write(data)
	if err != nil {
		return err
	}

	log.Printf("[INFO] Write file to %s", p)

	return nil
}

// Delete data on file.
func (f *File) Delete(key string) error {
	p := filepath.Join(f.dir, key)
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

	return archiver.Unarchive(src, dst)
}

// IsFileExist checks file exists.
func IsFileExist(p string) bool {
	_, err := os.Stat(p)

	return !os.IsNotExist(err)
}

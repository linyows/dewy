package kvs

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestIsFileExist(t *testing.T) {
	if IsFileExist("/tmp") != true {
		t.Error("expects return true")
	}
	if IsFileExist("/tmpfoo") != false {
		t.Error("expects return false")
	}
}

func TestFileDefault(t *testing.T) {
	f := &File{}
	f.Default()
	if IsFileExist(f.dir) != true {
		t.Error("file dir expects not setted")
	}
}

func TestFileRead(t *testing.T) {
	f := &File{}
	f.Default()
	data := []byte("this is data for test")
	err := f.Write("testread", data)
	if err != nil {
		t.Error(err.Error())
	}
	p := filepath.Join(f.dir, "testread")
	if IsFileExist(p) != true {
		t.Error("file not created")
	}
	content, err := f.Read("testread")
	if err != nil {
		t.Error(err.Error())
	}
	if !reflect.DeepEqual(content, data) {
		t.Error("return is not correct")
	}
}

func TestFileWrite(t *testing.T) {
	f := &File{}
	f.Default()
	data := []byte("this is data for test")
	err := f.Write("test", data)
	if err != nil {
		t.Error(err.Error())
	}
	if IsFileExist(filepath.Join(f.dir, "test")) != true {
		t.Error("file not found for cache")
	}
	content, err := f.Read("test")
	if !reflect.DeepEqual(content, data) {
		t.Errorf("writing is not correct: %s", err)
	}

	// Override
	data2 := []byte("hello gophers")
	err = f.Write("test", data2)
	if err != nil {
		t.Error(err.Error())
	}
	content2, err := f.Read("test")
	if !reflect.DeepEqual(content2, data2) {
		t.Errorf("writing is not correct when override: %s, %s", content2, err)
	}
}

func TestFileDelete(t *testing.T) {
	f := &File{}
	f.Default()
	data := []byte("this is data for test")
	err := f.Write("testdelete", data)
	if err != nil {
		t.Error(err.Error())
	}
	p := filepath.Join(f.dir, "testdelete")
	if IsFileExist(p) != true {
		t.Error("file not created")
	}
	err = f.Delete("testdelete")
	if err != nil {
		t.Error(err.Error())
	}
	if IsFileExist(p) != false {
		t.Error("file not deleted")
	}
}

func TestFileList(t *testing.T) {
	f := &File{}
	f.Default()
	data := []byte("this is data for test")
	err := f.Write("testlist", data)
	if err != nil {
		t.Error(err.Error())
	}
	list, err := f.List()
	if err != nil {
		t.Error(err.Error())
	}

	found := false
	for _, v := range list {
		if v == "testlist" {
			found = true
		}
	}
	if !found {
		t.Error("file not found in list")
	}
}

func TestExtractArchivePreservesPermissions(t *testing.T) {
	// Create a temporary directory for test
	tempDir, err := os.MkdirTemp("", "extract-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test tar.gz archive with files having different permissions
	archivePath := filepath.Join(tempDir, "test.tar.gz")
	if err := createTestArchive(archivePath); err != nil {
		t.Fatal(err)
	}

	// Extract the archive
	extractDir := filepath.Join(tempDir, "extracted")
	if err := ExtractArchive(archivePath, extractDir); err != nil {
		t.Fatal(err)
	}

	// Test regular file permissions (0644)
	regularFile := filepath.Join(extractDir, "regular.txt")
	if info, err := os.Stat(regularFile); err != nil {
		t.Fatal(err)
	} else if info.Mode().Perm() != 0644 {
		t.Errorf("Regular file has incorrect permissions: got %o, want %o", info.Mode().Perm(), 0644)
	}

	// Test executable file permissions (0755)
	execFile := filepath.Join(extractDir, "executable")
	if info, err := os.Stat(execFile); err != nil {
		t.Fatal(err)
	} else if info.Mode().Perm() != 0755 {
		t.Errorf("Executable file has incorrect permissions: got %o, want %o", info.Mode().Perm(), 0755)
	}

	// Test directory permissions (0755)
	dir := filepath.Join(extractDir, "subdir")
	if info, err := os.Stat(dir); err != nil {
		t.Fatal(err)
	} else if info.Mode().Perm() != 0755 {
		t.Errorf("Directory has incorrect permissions: got %o, want %o", info.Mode().Perm(), 0755)
	}
}

// createTestArchive creates a tar.gz archive with files having different permissions.
func createTestArchive(archivePath string) error {
	file, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gw := gzip.NewWriter(file)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	// Add a regular file with 0644 permissions
	if err := addFileToArchive(tw, "regular.txt", "This is a regular file", 0644); err != nil {
		return err
	}

	// Add an executable file with 0755 permissions
	if err := addFileToArchive(tw, "executable", "#!/bin/bash\necho 'Hello, World!'", 0755); err != nil {
		return err
	}

	// Add a directory with 0755 permissions
	if err := addDirToArchive(tw, "subdir/", 0755); err != nil {
		return err
	}

	// Add a file in subdirectory
	if err := addFileToArchive(tw, "subdir/nested.txt", "Nested file content", 0644); err != nil {
		return err
	}

	return nil
}

// addFileToArchive adds a file to tar archive with specified permissions.
func addFileToArchive(tw *tar.Writer, name, content string, mode os.FileMode) error {
	header := &tar.Header{
		Name: name,
		Mode: int64(mode),
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	_, err := tw.Write([]byte(content))
	return err
}

// addDirToArchive adds a directory to tar archive with specified permissions.
func addDirToArchive(tw *tar.Writer, name string, mode os.FileMode) error {
	header := &tar.Header{
		Name:     name,
		Mode:     int64(mode),
		Typeflag: tar.TypeDir,
	}
	return tw.WriteHeader(header)
}

func TestExtractArchiveZipSlip(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "zipslip-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create a malicious tar.gz archive with path traversal
	archivePath := filepath.Join(tempDir, "malicious.tar.gz")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	gw := gzip.NewWriter(file)
	tw := tar.NewWriter(gw)

	// Add a file with path traversal
	header := &tar.Header{
		Name: "../../etc/evil.txt",
		Mode: int64(0644),
		Size: int64(len("pwned")),
	}
	if err := tw.WriteHeader(header); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte("pwned")); err != nil {
		t.Fatal(err)
	}
	tw.Close()
	gw.Close()
	file.Close()

	// Extract should fail with path traversal error
	extractDir := filepath.Join(tempDir, "extracted")
	err = ExtractArchive(archivePath, extractDir)
	if err == nil {
		t.Fatal("Expected error for path traversal, got nil")
	}
	if !strings.Contains(err.Error(), "path traversal detected") {
		t.Errorf("Expected path traversal error, got: %v", err)
	}
}

func TestExtractArchiveSymlink(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "symlink-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create a tar.gz archive with a symlink entry
	archivePath := filepath.Join(tempDir, "symlink.tar.gz")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	gw := gzip.NewWriter(file)
	tw := tar.NewWriter(gw)

	// Add a symlink entry
	header := &tar.Header{
		Name:     "evil-link",
		Typeflag: tar.TypeSymlink,
		Linkname: "/etc/passwd",
		Mode:     int64(0777),
	}
	if err := tw.WriteHeader(header); err != nil {
		t.Fatal(err)
	}
	tw.Close()
	gw.Close()
	file.Close()

	// Extract should fail with symlink error
	extractDir := filepath.Join(tempDir, "extracted")
	err = ExtractArchive(archivePath, extractDir)
	if err == nil {
		t.Fatal("Expected error for symlink, got nil")
	}
	if !strings.Contains(err.Error(), "symlinks are not allowed") {
		t.Errorf("Expected symlink error, got: %v", err)
	}
}

func TestExtractArchiveSetuidStripped(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "setuid-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create a tar.gz archive with setuid bit
	archivePath := filepath.Join(tempDir, "setuid.tar.gz")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	gw := gzip.NewWriter(file)
	tw := tar.NewWriter(gw)

	// Add a file with setuid bit (04755)
	content := "#!/bin/bash\necho hello"
	header := &tar.Header{
		Name: "setuid-binary",
		Mode: 04755,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(header); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	tw.Close()
	gw.Close()
	file.Close()

	// Extract should succeed but strip setuid bit
	extractDir := filepath.Join(tempDir, "extracted")
	err = ExtractArchive(archivePath, extractDir)
	if err != nil {
		t.Fatalf("Expected extraction to succeed, got: %v", err)
	}

	// Verify setuid bit is stripped, but executable bit remains
	info, err := os.Stat(filepath.Join(extractDir, "setuid-binary"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSetuid != 0 {
		t.Error("Expected setuid bit to be stripped")
	}
	if info.Mode().Perm() != 0755 {
		t.Errorf("Expected permissions 0755, got %o", info.Mode().Perm())
	}
}

func TestCacheKeyPathTraversal(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cachekey-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	f := &File{}
	f.dir = tempDir
	f.MaxSize = DefaultMaxSize

	// Write valid key should succeed
	err = f.Write("valid-key", []byte("data"))
	if err != nil {
		t.Fatalf("Expected valid key to succeed, got: %v", err)
	}

	// Read with path traversal should fail
	_, err = f.Read("../../../etc/passwd")
	if err == nil {
		t.Fatal("Expected error for path traversal in Read, got nil")
	}
	if !strings.Contains(err.Error(), "path traversal detected") {
		t.Errorf("Expected path traversal error, got: %v", err)
	}

	// Write with path traversal should fail
	err = f.Write("../../../tmp/evil", []byte("data"))
	if err == nil {
		t.Fatal("Expected error for path traversal in Write, got nil")
	}
	if !strings.Contains(err.Error(), "path traversal detected") {
		t.Errorf("Expected path traversal error, got: %v", err)
	}

	// Delete with path traversal should fail
	err = f.Delete("../../../tmp/important")
	if err == nil {
		t.Fatal("Expected error for path traversal in Delete, got nil")
	}
	if !strings.Contains(err.Error(), "path traversal detected") {
		t.Errorf("Expected path traversal error, got: %v", err)
	}
}

func TestCreatePersistentCacheDir(t *testing.T) {
	tests := []struct {
		name          string
		dewyCacheDir  string
		expectedPath  func() string
		shouldContain string
	}{
		{
			name:         "DEWY_CACHEDIR priority",
			dewyCacheDir: "/tmp/custom-dewy-cache",
			expectedPath: func() string { return "/tmp/custom-dewy-cache" },
		},
		{
			name:          "Default PWD cache",
			dewyCacheDir:  "",
			shouldContain: ".dewy/cache",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original environment
			originalDewyCacheDir := os.Getenv("DEWY_CACHEDIR")

			// Set test environment
			if tt.dewyCacheDir != "" {
				os.Setenv("DEWY_CACHEDIR", tt.dewyCacheDir)
			} else {
				os.Unsetenv("DEWY_CACHEDIR")
			}

			// Test the function
			result := createPersistentCacheDir()

			// Verify result
			if tt.expectedPath != nil {
				expected := tt.expectedPath()
				if result != expected {
					t.Errorf("createPersistentCacheDir() = %v, want %v", result, expected)
				}
			} else if tt.shouldContain != "" {
				if !strings.Contains(result, tt.shouldContain) {
					t.Errorf("createPersistentCacheDir() = %v, should contain %v", result, tt.shouldContain)
				}
			}

			// Verify directory exists
			if _, err := os.Stat(result); os.IsNotExist(err) {
				t.Errorf("Created directory does not exist: %v", result)
			}

			// Clean up created directory for custom cache dir test
			if tt.dewyCacheDir != "" && strings.HasPrefix(tt.dewyCacheDir, "/tmp/") {
				os.RemoveAll(tt.dewyCacheDir)
			}

			// Restore original environment
			if originalDewyCacheDir != "" {
				os.Setenv("DEWY_CACHEDIR", originalDewyCacheDir)
			} else {
				os.Unsetenv("DEWY_CACHEDIR")
			}
		})
	}
}

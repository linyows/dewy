package kvs

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"reflect"
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

// createTestArchive creates a tar.gz archive with files having different permissions
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

// addFileToArchive adds a file to tar archive with specified permissions
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

// addDirToArchive adds a directory to tar archive with specified permissions
func addDirToArchive(tw *tar.Writer, name string, mode os.FileMode) error {
	header := &tar.Header{
		Name:     name,
		Mode:     int64(mode),
		Typeflag: tar.TypeDir,
	}
	return tw.WriteHeader(header)
}

package kvs

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
)

type File struct {
	items    map[string]*item
	dir      string
	mutex    sync.Mutex
	MaxItems int
	MaxSize  int64
}

func (f *File) Default() error {
	dir, err := ioutil.TempDir("", "dewy")
	if err != nil {
		return err
	}

	f.dir = dir
	f.MaxSize = 64 * 1024 * 1024

	return nil
}

func (f *File) Read(key string) ([]byte, error) {
	p := filepath.Join(f.dir, key)
	if !isFileExist(p) {
		return nil, errors.New(fmt.Sprintf("File not found: %s", p))
	}

	content, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, err
	}

	return content, nil
}

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
	if isFileExist(p) {
		return errors.New(fmt.Sprintf("File already exists: %s", p))
	}

	file, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	defer file.Close()
	file.Write(data)

	return nil
}

func (f *File) Delete(key string) error {
	p := filepath.Join(f.dir, key)
	if !isFileExist(p) {
		return errors.New(fmt.Sprintf("File not found: %s", p))
	}

	if err := os.Remove(p); err != nil {
		return err
	}

	return nil
}

func (f *File) List() ([]string, error) {
	files, err := ioutil.ReadDir(f.dir)
	if err != nil {
		return nil, err
	}

	var list []string
	for _, file := range files {
		list = append(list, file.Name())
	}

	return list, nil
}

func isFileExist(p string) bool {
	_, err := os.Stat(p)

	return !os.IsNotExist(err)
}

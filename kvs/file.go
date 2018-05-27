package kvs

import (
	"sync"
)

type File struct {
	items    map[string]*item
	dir      string
	mutex    sync.Mutex
	MaxItems int
	MaxSize  int64
}

func (f *File) Read(key string) string {
	return ""
}

func (f *File) Write(data string) bool {
	return true
}

func (f *File) Delete(key string) bool {
	return true
}

func (f *File) List() []string {
	return []string{""}
}

func isFileExist(p string) bool {
	_, err := os.Stat(p)
	return !os.IsNotExist(err)
}

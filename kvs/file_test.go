package kvs

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestIsFileExist(t *testing.T) {
	if isFileExist("/tmp") != true {
		t.Error("expects return true")
	}
	if isFileExist("/tmpfoo") != false {
		t.Error("expects return false")
	}
}

func TestFileDefault(t *testing.T) {
	f := &File{}
	f.Default()
	if isFileExist(f.dir) != true {
		t.Error("file dir expects not setted")
	}
	defer os.RemoveAll(f.dir)
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
	if isFileExist(p) != true {
		t.Error("file not created")
	}
	content, err := f.Read("testread")
	if !reflect.DeepEqual(content, data) {
		t.Error("return is not correct")
	}
	defer os.RemoveAll(f.dir)
}

func TestFileWrite(t *testing.T) {
	f := &File{}
	f.Default()
	data := []byte("this is data for test")
	err := f.Write("test", data)
	if err != nil {
		t.Error(err.Error())
	}
	if isFileExist(filepath.Join(f.dir, "test")) != true {
		t.Error("file not found for cache")
	}
	defer os.RemoveAll(f.dir)
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
	if isFileExist(p) != true {
		t.Error("file not created")
	}
	err = f.Delete("testdelete")
	if err != nil {
		t.Error(err.Error())
	}
	if isFileExist(p) != false {
		t.Error("file not deleted")
	}
	defer os.RemoveAll(f.dir)
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
	if fmt.Sprintf("%#v", list) != "[]string{\"testlist\"}" {
		t.Error("return is not correct")
	}
	defer os.RemoveAll(f.dir)
}

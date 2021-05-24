package kvs

import (
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

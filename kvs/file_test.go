package kvs

import (
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
}

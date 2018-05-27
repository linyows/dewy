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

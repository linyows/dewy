package dewy

import (
	"os"
	"reflect"
	"sync"
	"testing"

	"github.com/linyows/dewy/repo"
)

func TestNew(t *testing.T) {
	dewy := New(DefaultConfig())
	r, _ := os.Getwd()
	c := Config{
		Repository: repo.Config{},
		Cache: CacheConfig{
			Type:       FILE,
			Expiration: 10,
		},
		Starter: nil,
	}

	expect := &Dewy{
		config:          c,
		repo:            repo.New(c.Repository, dewy.cache),
		cache:           dewy.cache,
		isServerRunning: false,
		RWMutex:         sync.RWMutex{},
		root:            r,
	}

	if !reflect.DeepEqual(dewy, expect) {
		t.Errorf("new return is incorrect\nexpected: \n%#v\ngot: \n%#v\n", expect, dewy)
	}
}

package kvs

import (
	"sync"
	"time"
)

type KVS interface {
	Read(key string) (*item, error)
	Write(key string, data []byte) error
	Delete(key string) error
	List() ([]string, error)
}

type Config struct {
}

func New(t string, c Config) KVS {
	switch t {
	case "file":
		return &File{}
	default:
		panic("no provider")
	}
}

type item struct {
	content    []byte
	lock       sync.Mutex
	expiration time.Time
	size       uint64
}

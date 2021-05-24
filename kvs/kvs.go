package kvs

import (
	"sync"
	"time"
)

// KVS interface
type KVS interface {
	Read(key string) ([]byte, error)
	Write(key string, data []byte) error
	Delete(key string) error
	List() ([]string, error)
	GetDir() string
}

// Config struct
type Config struct {
}

// New returns KVS
func New(t string, c Config) KVS {
	switch t {
	case "file":
		return &File{}
	default:
		panic("no provider")
	}
}

//nolint
type item struct {
	content    []byte
	lock       sync.Mutex
	expiration time.Time
	size       uint64
}

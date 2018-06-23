package dewy

import (
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	starter "github.com/lestrrat-go/server-starter"
)

type StarterConfig struct {
	args       []string
	command    string
	dir        string
	interval   int
	pidfile    string
	ports      []string
	paths      []string
	sigonhup   string
	sigonterm  string
	statusfile string
}

func (c StarterConfig) Args() []string          { return c.args }
func (c StarterConfig) Command() string         { return c.command }
func (c StarterConfig) Dir() string             { return c.dir }
func (c StarterConfig) Interval() time.Duration { return time.Duration(c.interval) * time.Second }
func (c StarterConfig) PidFile() string         { return c.pidfile }
func (c StarterConfig) Ports() []string         { return c.ports }
func (c StarterConfig) Paths() []string         { return c.paths }
func (c StarterConfig) SignalOnHUP() os.Signal  { return starter.SigFromName(c.sigonhup) }
func (c StarterConfig) SignalOnTERM() os.Signal { return starter.SigFromName(c.sigonterm) }
func (c StarterConfig) StatusFile() string      { return c.statusfile }

func TestNew(t *testing.T) {
	conf := DefaultConfig()
	dewy := New(conf)

	root, _ := os.Getwd()
	config := Config{
		Repository: RepositoryConfig{},
		Cache: CacheConfig{
			Type:       FILE,
			Expiration: 10,
		},
		Starter: nil,
	}

	expect := &Dewy{
		config:          config,
		repository:      nil,
		cache:           dewy.cache,
		isServerRunning: false,
		RWMutex:         sync.RWMutex{},
		root:            root,
	}

	if !reflect.DeepEqual(dewy, expect) {
		t.Errorf("new return is incorrect\nexpected: \n%#v\ngot: \n%#v\n", expect, dewy)
	}
}

package fake

import (
	"sync"

	"github.com/linyows/dewy/internal/sysdeps"
)

// Env is a map-backed sysdeps.Env for tests.
type Env struct {
	mu       sync.RWMutex
	vars     map[string]string
	hostname string
	hostErr  error
	pid      int
}

// NewEnv returns an empty Env with hostname set to "test-host" and pid 1.
func NewEnv() *Env {
	return &Env{
		vars:     map[string]string{},
		hostname: "test-host",
		pid:      1,
	}
}

// Set sets an environment variable for subsequent Get calls.
func (e *Env) Set(key, value string) *Env {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.vars[key] = value
	return e
}

// SetHostname overrides the value returned by Hostname.
func (e *Env) SetHostname(name string) *Env {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.hostname = name
	e.hostErr = nil
	return e
}

// SetHostnameError makes Hostname return the given error.
func (e *Env) SetHostnameError(err error) *Env {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.hostErr = err
	return e
}

// SetPid overrides the value returned by Pid.
func (e *Env) SetPid(pid int) *Env {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.pid = pid
	return e
}

func (e *Env) Get(key string) string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.vars[key]
}

func (e *Env) Hostname() (string, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.hostname, e.hostErr
}

func (e *Env) Pid() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.pid
}

// Compile-time check.
var _ sysdeps.Env = (*Env)(nil)

package sysdeps

import "os"

// Env abstracts process-level environmental queries (env vars, hostname, pid)
// so callers can be tested without depending on the real process state.
type Env interface {
	Get(key string) string
	Hostname() (string, error)
	Pid() int
}

// RealEnv returns an Env backed by the os package.
func RealEnv() Env { return realEnv{} }

type realEnv struct{}

func (realEnv) Get(key string) string     { return os.Getenv(key) }
func (realEnv) Hostname() (string, error) { return os.Hostname() }
func (realEnv) Pid() int                  { return os.Getpid() }

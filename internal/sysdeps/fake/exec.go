package fake

import (
	"context"
	"errors"
	"sync"

	"github.com/linyows/dewy/internal/sysdeps"
)

// Call records one invocation against a fake CommandRunner.
type Call struct {
	Name string
	Args []string
}

// CommandRunner is a programmable sysdeps.CommandRunner. Tests register
// responses by command name; unregistered commands return an error.
type CommandRunner struct {
	mu        sync.Mutex
	calls     []Call
	responses map[string]commandResponse
	paths     map[string]string
}

// commandResponse produces the result for one invocation. Taking the args
// lets a single command name answer differently per subcommand.
type commandResponse func(args []string) ([]byte, error)

// NewCommandRunner returns an empty fake runner.
func NewCommandRunner() *CommandRunner {
	return &CommandRunner{
		responses: map[string]commandResponse{},
		paths:     map[string]string{},
	}
}

// SetOutput configures the bytes (and nil error) returned for the given command,
// regardless of its arguments.
func (c *CommandRunner) SetOutput(name string, output []byte) *CommandRunner {
	return c.SetOutputFunc(name, func([]string) ([]byte, error) { return output, nil })
}

// SetError configures the error returned for the given command, regardless of
// its arguments.
func (c *CommandRunner) SetError(name string, err error) *CommandRunner {
	return c.SetOutputFunc(name, func([]string) ([]byte, error) { return nil, err })
}

// SetOutputFunc configures a handler invoked with the command's arguments. Use
// this when one command name must answer differently per subcommand — a
// container runtime, for example, serves both `docker ps` and `docker inspect`
// under the name "docker".
func (c *CommandRunner) SetOutputFunc(name string, fn func(args []string) ([]byte, error)) *CommandRunner {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.responses[name] = fn
	return c
}

// SetPath configures what LookPath returns for the given command name.
func (c *CommandRunner) SetPath(name, path string) *CommandRunner {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.paths[name] = path
	return c
}

// Calls returns a snapshot of the recorded invocations.
func (c *CommandRunner) Calls() []Call {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Call, len(c.calls))
	copy(out, c.calls)
	return out
}

func (c *CommandRunner) record(name string, args []string) {
	c.mu.Lock()
	c.calls = append(c.calls, Call{Name: name, Args: append([]string(nil), args...)})
	c.mu.Unlock()
}

func (c *CommandRunner) respond(name string, args []string) ([]byte, error) {
	c.mu.Lock()
	fn, ok := c.responses[name]
	c.mu.Unlock()
	if !ok {
		return nil, errors.New("fake: unregistered command: " + name)
	}
	return fn(args)
}

func (c *CommandRunner) Run(ctx context.Context, name string, args ...string) error {
	c.record(name, args)
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err := c.respond(name, args)
	return err
}

func (c *CommandRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	c.record(name, args)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return c.respond(name, args)
}

func (c *CommandRunner) LookPath(name string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	p, ok := c.paths[name]
	if !ok {
		return "", errors.New("fake: command not found: " + name)
	}
	return p, nil
}

// Compile-time check.
var _ sysdeps.CommandRunner = (*CommandRunner)(nil)

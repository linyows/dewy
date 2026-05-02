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

type commandResponse struct {
	output []byte
	err    error
}

// NewCommandRunner returns an empty fake runner.
func NewCommandRunner() *CommandRunner {
	return &CommandRunner{
		responses: map[string]commandResponse{},
		paths:     map[string]string{},
	}
}

// SetOutput configures the bytes (and nil error) returned for the given command.
func (c *CommandRunner) SetOutput(name string, output []byte) *CommandRunner {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.responses[name] = commandResponse{output: output}
	return c
}

// SetError configures the error returned for the given command.
func (c *CommandRunner) SetError(name string, err error) *CommandRunner {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.responses[name] = commandResponse{err: err}
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

func (c *CommandRunner) response(name string) commandResponse {
	c.mu.Lock()
	defer c.mu.Unlock()
	r, ok := c.responses[name]
	if !ok {
		return commandResponse{err: errors.New("fake: unregistered command: " + name)}
	}
	return r
}

func (c *CommandRunner) Run(ctx context.Context, name string, args ...string) error {
	c.record(name, args)
	if err := ctx.Err(); err != nil {
		return err
	}
	return c.response(name).err
}

func (c *CommandRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	c.record(name, args)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r := c.response(name)
	return r.output, r.err
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

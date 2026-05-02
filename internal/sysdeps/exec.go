package sysdeps

import (
	"context"
	"os/exec"
)

// CommandRunner abstracts subprocess execution. The interface is intentionally
// minimal — callers that need streaming I/O should add methods as required.
//
// Real production code is wired up gradually (introduced in the sysdeps PR;
// individual call sites move over in subsequent PRs).
type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) error
	Output(ctx context.Context, name string, args ...string) ([]byte, error)
	LookPath(name string) (string, error)
}

// RealCommandRunner returns a CommandRunner backed by os/exec.
func RealCommandRunner() CommandRunner { return realCommandRunner{} }

type realCommandRunner struct{}

func (realCommandRunner) Run(ctx context.Context, name string, args ...string) error {
	return exec.CommandContext(ctx, name, args...).Run()
}

func (realCommandRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}

func (realCommandRunner) LookPath(name string) (string, error) {
	return exec.LookPath(name)
}

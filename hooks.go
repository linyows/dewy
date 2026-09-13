package dewy

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/cli/safeexec"
	"github.com/linyows/dewy/notifier"
)

// deployHook identifies one of the two deploy hooks. The notifier label and
// the log wording are carried together rather than derived from each other:
// they differ in capitalization, and both are user-visible.
type deployHook struct {
	label  string
	logMsg string
}

var (
	beforeDeployHook = deployHook{label: "Before Deploy", logMsg: "Before deploy hook failure"}
	afterDeployHook  = deployHook{label: "After Deploy", logMsg: "After deploy hook failure"}
)

// runDeployHook executes one deploy hook and reports it: the result is sent
// to the notifier whether or not the hook succeeded, and a failure is logged
// before the error is returned.
//
// Whether a failure aborts the deploy is the caller's decision, not this
// function's. The before-deploy hook is a gate and its callers return the
// error; the after-deploy hook runs once the deploy is already final, so its
// callers discard it (the failure is logged here either way).
//
// A blank cmd is a no-op, so callers can pass the configured hook
// unconditionally.
func (d *Dewy) runDeployHook(ctx context.Context, h deployHook, cmd string) error {
	result, err := d.execHook(cmd)
	if result != nil {
		d.notifier.SendHookResult(ctx, h.label, result)
	}
	if err != nil {
		d.logger.Error(h.logMsg, slog.String("error", err.Error()))
		return err
	}
	return nil
}

// execHook runs cmd as a shell command in d.root and returns a HookResult
// describing the run. A blank cmd is a no-op (returns nil, nil) so callers
// can pass d.config.BeforeDeployHook / AfterDeployHook unconditionally.
func (d *Dewy) execHook(cmd string) (*notifier.HookResult, error) {
	if cmd == "" {
		return nil, nil
	}

	start := time.Now()
	sh, err := safeexec.LookPath("sh")
	if err != nil {
		return nil, err
	}

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	c := exec.Command(sh, "-c", cmd)
	c.Dir = d.root
	c.Env = os.Environ()
	c.Stdout = stdout
	c.Stderr = stderr

	result := &notifier.HookResult{
		Command: cmd,
	}

	if err := c.Run(); err != nil {
		result.Duration = time.Since(start)
		result.Stdout = strings.TrimSpace(stdout.String())
		result.Stderr = strings.TrimSpace(stderr.String())
		result.Success = false

		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			result.ExitCode = exitError.ExitCode()
		} else {
			result.ExitCode = 1
		}

		d.logger.Info("Execute hook failed",
			slog.String("command", cmd),
			slog.String("stdout", result.Stdout),
			slog.String("stderr", result.Stderr),
			slog.Int("exit_code", result.ExitCode),
			slog.Duration("duration", result.Duration))

		return result, err
	}

	result.Duration = time.Since(start)
	result.Stdout = strings.TrimSpace(stdout.String())
	result.Stderr = strings.TrimSpace(stderr.String())
	result.Success = true
	result.ExitCode = 0

	d.logger.Info("Execute hook",
		slog.String("command", cmd),
		slog.String("stdout", result.Stdout),
		slog.String("stderr", result.Stderr),
		slog.Duration("duration", result.Duration))

	return result, nil
}

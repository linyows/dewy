package dewy

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/cli/safeexec"
	"github.com/linyows/dewy/notifier"
)

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

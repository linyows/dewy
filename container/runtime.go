package container

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/linyows/dewy/internal/sysdeps"
)

// Runtime implements Runtime interface using a container CLI (docker or podman).
type Runtime struct {
	cmd                string
	logger             *slog.Logger
	drainTime          time.Duration
	loggedInRegistries map[string]bool
	runner             sysdeps.CommandRunner
}

// Option customizes a Runtime at construction time.
type Option func(*Runtime)

// WithCommandRunner overrides the subprocess runner. Production code uses the
// default (sysdeps.RealCommandRunner); tests inject a fake to exercise runtime
// logic without a real container CLI.
func WithCommandRunner(r sysdeps.CommandRunner) Option {
	return func(rt *Runtime) { rt.runner = r }
}

// forbiddenLongOptions are long-form flags that conflict with Dewy management
// or pose security risks. --label-file is included because its file contents
// would bypass the reservedLabelPrefix check.
var forbiddenLongOptions = []string{
	"--detach",
	"--interactive",
	"--tty",
	"--publish",
	"--label-file",
	"--privileged",
	"--pid",
	"--cap-add",
	"--security-opt",
	"--device",
	"--userns",
	"--cgroupns",
}

// forbiddenShortFlagChars are single-character short flags that conflict with
// Dewy management. A short-flag arg is rejected when its first character is in
// this set, which catches three forms in one check:
//   - standalone: -d
//   - bundled boolean shorts: -dit (= -d -i -t)
//   - value-attached: -p8080:80 (= -p 8080:80)
//
// pflag-style parsing determines a short flag's type by its first character,
// so checking only arg[1] handles all three. Bundles led by an allowed boolean
// short (e.g., -qd) are not caught, but extra args are user-supplied so that
// blind spot does not cross a privilege boundary.
var forbiddenShortFlagChars = []byte{'d', 'i', 't', 'p'}

// reservedLabelPrefix is the label namespace Dewy uses to track managed
// containers. Users cannot set labels under this prefix because doing so
// would interfere with container discovery (FindContainersByLabel).
const reservedLabelPrefix = "dewy."

// newCLIRuntime creates a new Runtime with the specified command name.
func newCLIRuntime(cmd string, logger *slog.Logger, drainTime time.Duration, opts ...Option) (*Runtime, error) {
	r := &Runtime{
		cmd:                cmd,
		logger:             logger,
		drainTime:          drainTime,
		loggedInRegistries: make(map[string]bool),
		runner:             sysdeps.RealCommandRunner(),
	}
	for _, opt := range opts {
		opt(r)
	}

	if _, err := r.runner.LookPath(cmd); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrRuntimeNotFound, err)
	}

	return r, nil
}

// extractLabelValue returns the label value if args[i] is a label flag.
// Supports --label foo, --label=foo, -l foo, -l=foo, and -lfoo concatenated form.
// Returns ("", false) if args[i] is not a label flag.
func extractLabelValue(args []string, i int) (string, bool) {
	arg := args[i]
	switch {
	case arg == "--label" || arg == "-l":
		if i+1 < len(args) {
			return args[i+1], true
		}
		return "", false
	case strings.HasPrefix(arg, "--label="):
		return arg[len("--label="):], true
	case strings.HasPrefix(arg, "-l="):
		return arg[len("-l="):], true
	case strings.HasPrefix(arg, "-l") && len(arg) > 2 && arg[2] != '-':
		return arg[2:], true
	}
	return "", false
}

// matchForbiddenShortFlag reports whether arg is a short-flag form (standalone,
// bundled, or value-attached) led by a character in forbiddenShortFlagChars.
func matchForbiddenShortFlag(arg string) (byte, bool) {
	if len(arg) < 2 || arg[0] != '-' || arg[1] == '-' {
		return 0, false
	}
	for _, c := range forbiddenShortFlagChars {
		if arg[1] == c {
			return c, true
		}
	}
	return 0, false
}

// validateExtraArgs checks if any forbidden options are present in extra args.
func validateExtraArgs(args []string) error {
	for i, arg := range args {
		for _, forbidden := range forbiddenLongOptions {
			if arg == forbidden || strings.HasPrefix(arg, forbidden+"=") {
				return fmt.Errorf("option %s conflicts with Dewy management and cannot be used", forbidden)
			}
		}

		if c, ok := matchForbiddenShortFlag(arg); ok {
			return fmt.Errorf("option -%c conflicts with Dewy management and cannot be used", c)
		}

		if value, ok := extractLabelValue(args, i); ok && strings.HasPrefix(value, reservedLabelPrefix) {
			return fmt.Errorf("label %q uses reserved prefix %q and cannot be used", value, reservedLabelPrefix)
		}
	}
	return nil
}

// hasUserOption checks if --user or -u option is present in args.
func hasUserOption(args []string) bool {
	for _, arg := range args {
		if arg == "--user" || strings.HasPrefix(arg, "--user=") {
			return true
		}
		if arg == "-u" || strings.HasPrefix(arg, "-u=") {
			return true
		}
		// Handle combined short options like -u1000 (without =)
		if len(arg) > 2 && arg[0] == '-' && arg[1] == 'u' && arg[2] != '-' {
			return true
		}
	}
	return false
}

// extractNameOption extracts --name option from args and returns the name and filtered args.
func extractNameOption(args []string) (string, []string) {
	var name string
	filtered := []string{}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--name" && i+1 < len(args) {
			name = args[i+1]
			i++ // Skip next argument
		} else if strings.HasPrefix(arg, "--name=") {
			name = arg[7:] // Skip "--name="
		} else {
			filtered = append(filtered, arg)
		}
	}

	return name, filtered
}

// execCommand executes a command, discarding its output on success.
func (r *Runtime) execCommand(ctx context.Context, args ...string) error {
	_, err := r.execCommandOutput(ctx, args...)
	return err
}

// execCommandOutput executes a command and returns its trimmed stdout. On
// failure the returned error carries the subprocess stderr when the runner
// exposes it via *exec.ExitError.
func (r *Runtime) execCommandOutput(ctx context.Context, args ...string) (string, error) {
	r.logger.Debug("Executing command",
		slog.String("cmd", r.cmd),
		slog.Any("args", args))

	output, err := r.runner.Output(ctx, r.cmd, args...)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			r.logger.Error("Command failed",
				slog.String("cmd", r.cmd),
				slog.Any("args", args),
				slog.String("stderr", string(exitErr.Stderr)),
				slog.String("error", err.Error()))
			return "", fmt.Errorf("%s %s failed: %w: %s",
				r.cmd, strings.Join(args, " "), err, string(exitErr.Stderr))
		}
		r.logger.Error("Command failed",
			slog.String("cmd", r.cmd),
			slog.Any("args", args),
			slog.String("error", err.Error()))
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// Run starts a new container and returns the container ID.
func (r *Runtime) Run(ctx context.Context, opts RunOptions) (string, error) {
	// Validate extra args first
	if err := validateExtraArgs(opts.ExtraArgs); err != nil {
		return "", err
	}

	// Extract --name from extra args
	userName, filteredArgs := extractNameOption(opts.ExtraArgs)

	// Determine container name
	baseName := opts.AppName
	if userName != "" {
		baseName = userName
	}

	// Generate container name with timestamp and replica index
	timestamp := time.Now().Unix()
	containerName := fmt.Sprintf("%s-%d-%d", baseName, timestamp, opts.ReplicaIndex)

	// Build run command
	args := []string{"run"}

	if opts.Detach {
		args = append(args, "-d")
	}

	args = append(args, "--name", containerName)

	for key, value := range opts.Labels {
		args = append(args, "--label", fmt.Sprintf("%s=%s", key, value))
	}

	for _, port := range opts.Ports {
		args = append(args, "-p", port)
	}

	args = append(args, filteredArgs...)

	if !hasUserOption(filteredArgs) {
		args = append(args, "--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()))
	}

	args = append(args, opts.Image)
	args = append(args, opts.Command...)

	r.logger.Info("Starting container",
		slog.String("name", containerName),
		slog.String("image", opts.Image))

	containerID, err := r.execCommandOutput(ctx, args...)
	if err != nil {
		return "", err
	}

	return containerID, nil
}

// Stop stops a running container gracefully.
func (r *Runtime) Stop(ctx context.Context, containerID string, timeout time.Duration) error {
	r.logger.Info("Stopping container gracefully",
		slog.String("container", containerID),
		slog.Duration("timeout", timeout))

	timeoutSec := int(timeout.Seconds())
	return r.execCommand(ctx, "stop", fmt.Sprintf("--time=%d", timeoutSec), containerID)
}

// Remove removes a container.
func (r *Runtime) Remove(ctx context.Context, containerID string) error {
	r.logger.Info("Removing container", slog.String("container", containerID))
	return r.execCommand(ctx, "rm", containerID)
}

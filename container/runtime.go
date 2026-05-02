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
)

// Runtime implements Runtime interface using a container CLI (docker or podman).
type Runtime struct {
	cmd                string
	logger             *slog.Logger
	drainTime          time.Duration
	loggedInRegistries map[string]bool
}

// Forbidden options that conflict with Dewy management or pose security risks.
var forbiddenOptions = []string{
	"-d", "--detach",
	"-it",
	"-i", "--interactive",
	"-t", "--tty",
	"-l", "--label",
	"-p", "--publish",
	"--privileged",
	"--pid",
	"--cap-add",
	"--security-opt",
	"--device",
	"--userns",
	"--cgroupns",
}

// newCLIRuntime creates a new Runtime with the specified command name.
func newCLIRuntime(cmd string, logger *slog.Logger, drainTime time.Duration) (*Runtime, error) {
	if _, err := exec.LookPath(cmd); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrRuntimeNotFound, err)
	}

	return &Runtime{
		cmd:                cmd,
		logger:             logger,
		drainTime:          drainTime,
		loggedInRegistries: make(map[string]bool),
	}, nil
}

// validateExtraArgs checks if any forbidden options are present in extra args.
func validateExtraArgs(args []string) error {
	for _, arg := range args {
		for _, forbidden := range forbiddenOptions {
			if arg == forbidden || strings.HasPrefix(arg, forbidden+"=") {
				return fmt.Errorf("option %s conflicts with Dewy management and cannot be used", forbidden)
			}
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

// execCommand executes a command without returning output.
func (r *Runtime) execCommand(ctx context.Context, args ...string) error {
	// #nosec G204 - args are constructed internally from validated inputs
	cmd := exec.CommandContext(ctx, r.cmd, args...)
	r.logger.Debug("Executing command",
		slog.String("cmd", r.cmd),
		slog.Any("args", args))

	output, err := cmd.CombinedOutput()
	if err != nil {
		r.logger.Error("Command failed",
			slog.String("cmd", r.cmd),
			slog.Any("args", args),
			slog.String("output", string(output)),
			slog.String("error", err.Error()))
		return fmt.Errorf("%s %s failed: %w: %s",
			r.cmd, strings.Join(args, " "), err, string(output))
	}

	return nil
}

// execCommandOutput executes a command and returns the output.
func (r *Runtime) execCommandOutput(ctx context.Context, args ...string) (string, error) {
	// #nosec G204 - args are constructed internally from validated inputs
	cmd := exec.CommandContext(ctx, r.cmd, args...)
	r.logger.Debug("Executing command",
		slog.String("cmd", r.cmd),
		slog.Any("args", args))

	output, err := cmd.Output()
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

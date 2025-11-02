package container

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// HealthChecker provides health check functionality for containers.
type HealthChecker struct {
	logger  *slog.Logger
	timeout time.Duration
	retries int
	delay   time.Duration
}

// NewHealthChecker creates a new HealthChecker.
func NewHealthChecker(logger *slog.Logger, timeout time.Duration, retries int) *HealthChecker {
	return &HealthChecker{
		logger:  logger,
		timeout: timeout,
		retries: retries,
		delay:   2 * time.Second,
	}
}

// CheckHTTP performs HTTP health check.
func (h *HealthChecker) CheckHTTP(ctx context.Context, url string) error {
	client := &http.Client{Timeout: h.timeout}

	for i := 0; i < h.retries; i++ {
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return err
		}

		resp, err := client.Do(req)
		if err != nil {
			h.logger.Warn("Health check attempt failed",
				slog.Int("attempt", i+1),
				slog.String("url", url),
				slog.String("error", err.Error()))

			if i < h.retries-1 {
				time.Sleep(h.delay)
				continue
			}
			return fmt.Errorf("health check failed after %d attempts: %w", h.retries, err)
		}

		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			h.logger.Info("Health check passed",
				slog.String("url", url),
				slog.Int("status", resp.StatusCode))
			return nil
		}

		h.logger.Warn("Health check returned unhealthy status",
			slog.Int("attempt", i+1),
			slog.String("url", url),
			slog.Int("status", resp.StatusCode))

		if i < h.retries-1 {
			time.Sleep(h.delay)
		}
	}

	return fmt.Errorf("health check failed: unhealthy status after %d attempts", h.retries)
}

// CheckTCP performs TCP health check.
func (h *HealthChecker) CheckTCP(ctx context.Context, host string, port int) error {
	address := fmt.Sprintf("%s:%d", host, port)

	for i := 0; i < h.retries; i++ {
		dialer := net.Dialer{Timeout: h.timeout}
		conn, err := dialer.DialContext(ctx, "tcp", address)
		if err != nil {
			h.logger.Warn("TCP health check attempt failed",
				slog.Int("attempt", i+1),
				slog.String("address", address),
				slog.String("error", err.Error()))

			if i < h.retries-1 {
				time.Sleep(h.delay)
				continue
			}
			return fmt.Errorf("TCP health check failed after %d attempts: %w", h.retries, err)
		}

		conn.Close()
		h.logger.Info("TCP health check passed", slog.String("address", address))
		return nil
	}

	return fmt.Errorf("TCP health check failed after %d attempts", h.retries)
}

// WaitForHTTP creates a health check function for HTTP endpoints.
func WaitForHTTP(logger *slog.Logger, url string, timeout time.Duration, retries int) HealthCheckFunc {
	return func(ctx context.Context, containerID string) error {
		checker := NewHealthChecker(logger, timeout, retries)
		return checker.CheckHTTP(ctx, url)
	}
}

// WaitForTCP creates a health check function for TCP endpoints.
func WaitForTCP(logger *slog.Logger, host string, port int, timeout time.Duration, retries int) HealthCheckFunc {
	return func(ctx context.Context, containerID string) error {
		checker := NewHealthChecker(logger, timeout, retries)
		return checker.CheckTCP(ctx, host, port)
	}
}

// WaitForHTTPviaNetwork creates a health check function that accesses the container via Docker network.
// This is useful for Blue-Green deployments where the container may not have port mappings.
// Note: On macOS, this uses docker exec since network IP access from host doesn't work.
func WaitForHTTPviaNetwork(logger *slog.Logger, runtime *Docker, network, path string, port int, timeout time.Duration, retries int) HealthCheckFunc {
	return func(ctx context.Context, containerID string) error {
		// Use docker exec to check health from inside the container
		// This works on all platforms (Linux, macOS, Windows)
		url := fmt.Sprintf("http://localhost:%d%s", port, path)
		logger.Debug("Using docker exec for health check",
			slog.String("url", url),
			slog.String("container", containerID))

		checker := NewHealthChecker(logger, timeout, retries)
		for i := 0; i < retries; i++ {
			// Use wget inside the container to check the health endpoint
			args := []string{"exec", containerID, "wget", "-q", "-O-", url}
			err := runtime.execCommand(ctx, args...)
			if err == nil {
				logger.Info("Health check passed via docker exec",
					slog.String("url", url),
					slog.String("container", containerID))
				return nil
			}

			logger.Warn("Health check attempt failed via docker exec",
				slog.Int("attempt", i+1),
				slog.String("url", url),
				slog.String("error", err.Error()))

			if i < retries-1 {
				time.Sleep(checker.delay)
			}
		}

		return fmt.Errorf("health check failed after %d attempts", retries)
	}
}

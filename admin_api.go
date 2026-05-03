package dewy

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/linyows/dewy/container"
)

// startAdminAPI starts the admin API server on TCP localhost.
func (d *Dewy) startAdminAPI(ctx context.Context) error {
	// Default admin port is 17539 (DEWY: D=4, E=5, W=23, Y=25 -> 4+5+2+3+2+5=21, but 17539 is more unique)
	adminPort := d.config.AdminPort
	if adminPort == 0 {
		adminPort = 17539
	}

	// Try to bind to the port, increment if already in use
	var listener net.Listener
	var err error
	maxAttempts := 10

	for i := range maxAttempts {
		currentPort := adminPort + i
		addr := fmt.Sprintf("localhost:%d", currentPort)
		listener, err = net.Listen("tcp", addr)
		if err == nil {
			// Successfully bound to port
			adminPort = currentPort
			d.logger.Info("Admin API port bound successfully",
				slog.Int("port", adminPort))
			break
		}
		d.logger.Debug("Admin API port in use, trying next",
			slog.Int("port", currentPort),
			slog.String("error", err.Error()))
	}

	if listener == nil {
		return fmt.Errorf("failed to bind admin API after %d attempts: %w", maxAttempts, err)
	}

	// Create HTTP mux for admin API
	mux := http.NewServeMux()
	mux.HandleFunc("/api/containers", d.handleGetContainers)
	mux.HandleFunc("/api/status", d.handleGetStatus)

	// Add Prometheus metrics endpoint if telemetry is enabled
	if d.telemetry != nil && d.telemetry.Enabled() {
		mux.Handle("/metrics", d.telemetry.PrometheusHandler())
		d.logger.Info("Prometheus metrics endpoint enabled", slog.String("path", "/metrics"))
	}

	d.adminServer = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: defaultAdminReadHeaderTimeout,
	}

	// Start server in background
	go func() {
		d.logger.Info("Starting admin API server",
			slog.Int("port", adminPort))

		if err := d.adminServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			d.logger.Error("Admin API server error", slog.String("error", err.Error()))
		}
	}()

	d.logger.Info("Admin API server started",
		slog.Int("port", adminPort),
		slog.String("address", fmt.Sprintf("http://localhost:%d", adminPort)))

	return nil
}

// stopAdminAPI stops the admin API server.
func (d *Dewy) stopAdminAPI(ctx context.Context) error {
	if d.adminServer == nil {
		return nil
	}

	d.logger.Info("Stopping admin API server")

	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := d.adminServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("failed to shutdown admin API: %w", err)
	}

	d.logger.Info("Admin API server stopped")
	return nil
}

// containerListLabels returns the label set the deploy path uses to mark
// managed containers, so admin queries match the deploy reality even when
// --name is omitted (in which case appName falls back to the registry-
// derived repository name).
func (d *Dewy) containerListLabels() map[string]string {
	return map[string]string{
		"dewy.managed": "true",
		"dewy.app":     d.appName(),
	}
}

// handleGetContainers handles GET /api/containers endpoint.
func (d *Dewy) handleGetContainers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	var containers []*container.Info

	// containerRuntime is wired up by the first RunContainer tick. The admin
	// API listens earlier (in Start()), so a request that lands during the
	// startup window has nothing to query — return an empty list rather
	// than nil-deref.
	if d.config.Command == CONTAINER && d.containerRuntime != nil {
		// Use first port mapping for listing containers (0 = auto-detect / not specified)
		containerPort := 0
		if len(d.config.Container.PortMappings) > 0 {
			containerPort = d.config.Container.PortMappings[0].ContainerPort
		}
		var err error
		containers, err = d.containerRuntime.ListContainersByLabels(ctx, d.containerListLabels(), containerPort)
		if err != nil {
			d.logger.Error("Failed to list containers",
				slog.String("error", err.Error()))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"containers": containers,
	}); err != nil {
		d.logger.Error("Failed to encode response",
			slog.String("error", err.Error()))
	}
}

// handleGetStatus handles GET /api/status endpoint.
func (d *Dewy) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	d.RLock()
	defer d.RUnlock()

	// Count total backends across all proxies
	totalBackends := d.totalProxyBackends()

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"name":            d.appName(),
		"command":         d.config.Command,
		"current_version": d.cVer,
		"proxy_backends":  totalBackends,
		"is_running":      d.isServerRunning,
	}); err != nil {
		d.logger.Error("Failed to encode response",
			slog.String("error", err.Error()))
	}
}

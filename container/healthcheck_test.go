package container

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestHealthChecker_CheckHTTP_Success(t *testing.T) {
	// Create a test HTTP server that returns 200 OK
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "OK")
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	checker := NewHealthChecker(logger, 5*time.Second, 3)

	ctx := context.Background()
	url := server.URL + "/health"

	err := checker.CheckHTTP(ctx, url)
	if err != nil {
		t.Fatalf("Expected health check to pass, got error: %v", err)
	}
}

func TestHealthChecker_CheckHTTP_Failure_BadStatus(t *testing.T) {
	// Create a test HTTP server that returns 503 Service Unavailable
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprint(w, "UNHEALTHY")
	}))
	defer server.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	checker := NewHealthChecker(logger, 1*time.Second, 2) // Quick retries for testing

	ctx := context.Background()
	url := server.URL + "/health"

	err := checker.CheckHTTP(ctx, url)
	if err == nil {
		t.Fatal("Expected health check to fail, but it passed")
	}

	expectedErr := "health check failed: unhealthy status after 2 attempts"
	if err.Error() != expectedErr {
		t.Errorf("Expected error message '%s', got '%s'", expectedErr, err.Error())
	}
}

func TestHealthChecker_CheckHTTP_Failure_Timeout(t *testing.T) {
	// Create a test HTTP server that times out
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second) // Sleep longer than timeout
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	checker := NewHealthChecker(logger, 500*time.Millisecond, 2) // Short timeout

	ctx := context.Background()
	url := server.URL + "/health"

	err := checker.CheckHTTP(ctx, url)
	if err == nil {
		t.Fatal("Expected health check to fail due to timeout, but it passed")
	}
}

func TestHealthChecker_CheckHTTP_Retry(t *testing.T) {
	// Create a test HTTP server that fails first, then succeeds
	attemptCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		if attemptCount < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	checker := NewHealthChecker(logger, 1*time.Second, 3)

	ctx := context.Background()
	url := server.URL + "/health"

	err := checker.CheckHTTP(ctx, url)
	if err != nil {
		t.Fatalf("Expected health check to pass after retry, got error: %v", err)
	}

	if attemptCount != 2 {
		t.Errorf("Expected 2 attempts, got %d", attemptCount)
	}
}

func TestHealthChecker_CheckHTTP_ContextCancellation(t *testing.T) {
	// Create a test HTTP server that takes time
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	checker := NewHealthChecker(logger, 5*time.Second, 3)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	url := server.URL + "/health"

	err := checker.CheckHTTP(ctx, url)
	if err == nil {
		t.Fatal("Expected health check to fail due to context cancellation, but it passed")
	}
}

func TestHealthChecker_CheckTCP_Success(t *testing.T) {
	// Create a test TCP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Extract host and port from server URL
	// URL format: http://127.0.0.1:12345
	url := server.URL[7:] // Remove "http://"
	parts := strings.Split(url, ":")
	host := parts[0]
	var port int
	if _, err := fmt.Sscanf(parts[1], "%d", &port); err != nil {
		t.Fatalf("Failed to parse port: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	checker := NewHealthChecker(logger, 2*time.Second, 3)

	ctx := context.Background()
	err := checker.CheckTCP(ctx, host, port)
	if err != nil {
		t.Fatalf("Expected TCP health check to pass, got error: %v", err)
	}
}

func TestHealthChecker_CheckTCP_Failure(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	checker := NewHealthChecker(logger, 500*time.Millisecond, 2)

	ctx := context.Background()
	// Use a port that's not listening
	err := checker.CheckTCP(ctx, "127.0.0.1", 9999)
	if err == nil {
		t.Fatal("Expected TCP health check to fail, but it passed")
	}
}

func TestWaitForHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	healthCheck := WaitForHTTP(logger, server.URL, 2*time.Second, 3)

	ctx := context.Background()
	err := healthCheck(ctx, "dummy-container-id")
	if err != nil {
		t.Fatalf("Expected health check function to pass, got error: %v", err)
	}
}

func TestWaitForTCP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Extract host and port
	url := server.URL[7:]
	parts := strings.Split(url, ":")
	host := parts[0]
	var port int
	if _, err := fmt.Sscanf(parts[1], "%d", &port); err != nil {
		t.Fatalf("Failed to parse port: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	healthCheck := WaitForTCP(logger, host, port, 2*time.Second, 3)

	ctx := context.Background()
	err := healthCheck(ctx, "dummy-container-id")
	if err != nil {
		t.Fatalf("Expected health check function to pass, got error: %v", err)
	}
}

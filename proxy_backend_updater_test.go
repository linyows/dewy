package dewy

import (
	"strings"
	"testing"

	"github.com/linyows/dewy/container"
)

// proxyBackendUpdater must satisfy container.BackendUpdater so it can be
// passed straight to runtime.Deploy.
func TestProxyBackendUpdaterImplementsInterface(t *testing.T) {
	d := newPhaseTestDewy(t)
	var _ container.BackendUpdater = (*proxyBackendUpdater)(d)
}

// AddBackend / RemoveBackend forward to addProxyBackend / removeProxyBackend.
// Without a real tcpProxy registered for the port, both methods surface an
// error path so we can assert the forwarding happened and the wiring is alive.
func TestProxyBackendUpdaterForwardsToDewy(t *testing.T) {
	d := newPhaseTestDewy(t)
	d.tcpProxies = nil // explicit empty
	u := (*proxyBackendUpdater)(d)

	err := u.AddBackend("localhost", 9001, 8080)
	if err == nil {
		t.Fatal("expected error when no proxy is configured for the port, got nil")
	}
	if !strings.Contains(err.Error(), "8080") {
		t.Errorf("error %q should mention the missing proxy port 8080", err)
	}

	err = u.RemoveBackend("localhost", 9001, 8080)
	if err == nil {
		t.Fatal("expected error when no proxy is configured for the port, got nil")
	}
	if !strings.Contains(err.Error(), "8080") {
		t.Errorf("error %q should mention the missing proxy port 8080", err)
	}
}

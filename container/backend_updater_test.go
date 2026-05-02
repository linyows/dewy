package container

import (
	"errors"
	"sync"
	"testing"
)

// mockBackendUpdater records every Add/Remove call and optionally returns an
// error from AddBackend (for rollback-path coverage in deeper tests).
type mockBackendUpdater struct {
	mu      sync.Mutex
	adds    []backendCall
	removes []backendCall
	addErr  error
}

type backendCall struct {
	Host       string
	MappedPort int
	ProxyPort  int
}

func (m *mockBackendUpdater) AddBackend(host string, mappedPort, proxyPort int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.adds = append(m.adds, backendCall{host, mappedPort, proxyPort})
	return m.addErr
}

func (m *mockBackendUpdater) RemoveBackend(host string, mappedPort, proxyPort int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.removes = append(m.removes, backendCall{host, mappedPort, proxyPort})
	return nil
}

// Compile-time check that mockBackendUpdater satisfies the public interface.
var _ BackendUpdater = (*mockBackendUpdater)(nil)

func TestNoopBackendUpdaterReturnsNil(t *testing.T) {
	// noopBackendUpdater is the substitute Deploy uses when callers pass nil.
	// The contract is "never panic, always return nil".
	u := noopBackendUpdater{}
	if err := u.AddBackend("h", 1, 2); err != nil {
		t.Errorf("AddBackend returned %v, want nil", err)
	}
	if err := u.RemoveBackend("h", 1, 2); err != nil {
		t.Errorf("RemoveBackend returned %v, want nil", err)
	}
}

func TestNoopBackendUpdaterImplementsInterface(t *testing.T) {
	var _ BackendUpdater = noopBackendUpdater{}
}

func TestMockBackendUpdaterRecordsCalls(t *testing.T) {
	// Sanity-check the test mock: future PRs that exercise Deploy behavior
	// will rely on Calls() ordering and the addErr injection.
	m := &mockBackendUpdater{}
	if err := m.AddBackend("a", 1, 100); err != nil {
		t.Fatal(err)
	}
	if err := m.RemoveBackend("b", 2, 200); err != nil {
		t.Fatal(err)
	}
	if len(m.adds) != 1 || m.adds[0] != (backendCall{"a", 1, 100}) {
		t.Errorf("adds = %+v", m.adds)
	}
	if len(m.removes) != 1 || m.removes[0] != (backendCall{"b", 2, 200}) {
		t.Errorf("removes = %+v", m.removes)
	}

	m.addErr = errors.New("boom")
	if err := m.AddBackend("c", 3, 300); err == nil {
		t.Error("expected addErr to be returned")
	}
}

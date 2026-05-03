package dewy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newAdminTestDewy returns a Dewy minimally configured for admin-handler
// tests. Container.Name is left empty so the admin path must fall back to
// the registry-derived appName.
func newAdminTestDewy(t *testing.T) *Dewy {
	t.Helper()
	c := DefaultConfig()
	c.Command = CONTAINER
	c.Registry = "img://ghcr.io/owner/myapp"
	c.Cache = CacheConfig{Type: FILE, Expiration: 10, URL: "file://" + t.TempDir()}
	c.Container = &ContainerConfig{Name: ""}
	d, err := New(c, testLogger())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return d
}

// Bug #2: when --name is omitted, the deploy path labels containers with the
// derived app name; the admin API used to filter by an empty value and
// return 0 containers. The label-building helper must agree with appName().
func TestContainerListLabels_DerivesAppName(t *testing.T) {
	d := newAdminTestDewy(t)
	got := d.containerListLabels()
	if got["dewy.app"] != "myapp" {
		t.Errorf(`labels["dewy.app"] = %q, want "myapp" (derived from registry)`, got["dewy.app"])
	}
	if got["dewy.managed"] != "true" {
		t.Errorf(`labels["dewy.managed"] = %q, want "true"`, got["dewy.managed"])
	}
}

// Bug #1: startAdminAPI runs in Start() before the first RunContainer tick
// initializes d.containerRuntime. Hitting /api/containers during that window
// used to nil-deref. The handler must respond cleanly without panicking.
func TestHandleGetContainers_NilRuntimeDoesNotPanic(t *testing.T) {
	d := newAdminTestDewy(t)
	if d.containerRuntime != nil {
		t.Fatal("test setup invariant violated: containerRuntime should be nil before the first tick")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/containers", nil)
	w := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("handleGetContainers panicked with nil runtime: %v", r)
		}
	}()

	d.handleGetContainers(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var body struct {
		Containers []any `json:"containers"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(body.Containers) != 0 {
		t.Errorf("containers = %v, want empty list", body.Containers)
	}
}

// Bug #3: /api/status used to report Config.Container.Name verbatim, which
// is empty when --name is omitted. The reported name should match the
// appName the deploy path actually uses.
func TestHandleGetStatus_ReportsDerivedAppName(t *testing.T) {
	d := newAdminTestDewy(t)

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	w := httptest.NewRecorder()
	d.handleGetStatus(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if name, _ := body["name"].(string); name != "myapp" {
		t.Errorf(`status.name = %q, want "myapp" (derived from registry); body=%s`, name, w.Body.String())
	}
}

// MethodNotAllowed remains. (Sanity check that the new code paths haven't
// regressed the method gate.)
func TestHandleGetContainers_RejectsNonGet(t *testing.T) {
	d := newAdminTestDewy(t)
	req := httptest.NewRequest(http.MethodPost, "/api/containers", strings.NewReader(""))
	w := httptest.NewRecorder()
	d.handleGetContainers(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /api/containers: status = %d, want 405", w.Code)
	}
}

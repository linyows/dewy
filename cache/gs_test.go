package cache

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"cloud.google.com/go/storage"
	"google.golang.org/api/googleapi"
)

type mockGSClient struct {
	objects     map[string][]byte
	generations map[string]int64
	nextGen     int64
	getErr      error
}

func newMockGSClient() *mockGSClient {
	return &mockGSClient{
		objects:     map[string][]byte{},
		generations: map[string]int64{},
	}
}

func (m *mockGSClient) GetObject(ctx context.Context, bucket, name string) ([]byte, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	data, ok := m.objects[name]
	if !ok {
		return nil, storage.ErrObjectNotExist
	}
	return data, nil
}

func (m *mockGSClient) PutObject(ctx context.Context, bucket, name string, data []byte) error {
	m.objects[name] = data
	m.nextGen++
	m.generations[name] = m.nextGen
	return nil
}

func (m *mockGSClient) ReadWithGeneration(ctx context.Context, bucket, name string) ([]byte, int64, error) {
	if m.getErr != nil {
		return nil, 0, m.getErr
	}
	data, ok := m.objects[name]
	if !ok {
		return nil, 0, storage.ErrObjectNotExist
	}
	return data, m.generations[name], nil
}

func (m *mockGSClient) WriteIfGeneration(ctx context.Context, bucket, name string, data []byte, expectedGeneration int64) (int64, error) {
	current := m.generations[name]
	if current != expectedGeneration {
		return 0, &googleapi.Error{Code: 412, Message: "precondition failed"}
	}
	m.objects[name] = data
	m.nextGen++
	m.generations[name] = m.nextGen
	return m.nextGen, nil
}

func (m *mockGSClient) DeleteObject(ctx context.Context, bucket, name string) error {
	if _, ok := m.objects[name]; !ok {
		return storage.ErrObjectNotExist
	}
	delete(m.objects, name)
	return nil
}

func (m *mockGSClient) ListObjects(ctx context.Context, bucket, prefix string) ([]string, error) {
	var names []string
	for k := range m.objects {
		if strings.HasPrefix(k, prefix) {
			names = append(names, k)
		}
	}
	return names, nil
}

func newTestGS(t *testing.T) (*GS, *mockGSClient) {
	t.Helper()
	dir := t.TempDir()
	mock := newMockGSClient()
	g := &GS{
		Bucket:  "testbucket",
		Prefix:  "team/app/",
		cl:      mock,
		ctx:     context.Background(),
		dir:     dir,
		MaxSize: DefaultMaxSize,
	}
	return g, mock
}

func TestGSWriteAndRead(t *testing.T) {
	g, mock := newTestGS(t)
	data := []byte("hello gcs")

	if err := g.Write("artifact.tar.gz", data); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if got, ok := mock.objects["team/app/artifact.tar.gz"]; !ok {
		t.Fatal("expected object stored under prefixed name")
	} else if !bytes.Equal(got, data) {
		t.Errorf("stored bytes mismatch: got %q want %q", got, data)
	}

	localPath := filepath.Join(g.dir, "artifact.tar.gz")
	if got, err := os.ReadFile(localPath); err != nil {
		t.Fatalf("expected local stage file: %v", err)
	} else if !bytes.Equal(got, data) {
		t.Errorf("staged bytes mismatch: got %q want %q", got, data)
	}

	got, err := g.Read("artifact.tar.gz")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Errorf("Read bytes mismatch: got %q want %q", got, data)
	}
}

func TestGSReadFromCloudStagesLocally(t *testing.T) {
	g, mock := newTestGS(t)
	mock.objects["team/app/x"] = []byte("from-cloud")

	got, err := g.Read("x")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(got) != "from-cloud" {
		t.Errorf("got %q", got)
	}

	mock.getErr = errors.New("cloud unavailable")
	got2, err := g.Read("x")
	if err != nil {
		t.Fatalf("Read after stage: %v", err)
	}
	if string(got2) != "from-cloud" {
		t.Errorf("staged read mismatch: %q", got2)
	}
}

func TestGSReadNotFound(t *testing.T) {
	g, _ := newTestGS(t)
	_, err := g.Read("missing")
	if err == nil {
		t.Fatal("expected error for missing key")
	}
	if !IsNotFound(err) {
		t.Errorf("expected IsNotFound, got %v", err)
	}
}

func TestGSList(t *testing.T) {
	g, mock := newTestGS(t)
	mock.objects["team/app/a"] = []byte("1")
	mock.objects["team/app/b"] = []byte("2")
	mock.objects["other/c"] = []byte("3")

	keys, err := g.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	sort.Strings(keys)
	if len(keys) != 2 || keys[0] != "a" || keys[1] != "b" {
		t.Errorf("unexpected keys: %v", keys)
	}
}

func TestGSDelete(t *testing.T) {
	g, mock := newTestGS(t)
	if err := g.Write("k", []byte("v")); err != nil {
		t.Fatal(err)
	}

	if err := g.Delete("k"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok := mock.objects["team/app/k"]; ok {
		t.Error("expected object removed from cloud")
	}
	if IsFileExist(filepath.Join(g.dir, "k")) {
		t.Error("expected local stage removed")
	}
}

func TestGSDeleteNotExistIgnored(t *testing.T) {
	g, _ := newTestGS(t)
	// No local file, no cloud object — Delete should not return an error.
	if err := g.Delete("absent"); err != nil {
		t.Errorf("expected nil for absent key, got %v", err)
	}
}

func TestGSPathTraversal(t *testing.T) {
	g, _ := newTestGS(t)
	if err := g.Write("../evil", []byte("x")); err == nil {
		t.Error("expected path traversal error on Write")
	}
	if _, err := g.Read("../evil"); err == nil {
		t.Error("expected path traversal error on Read")
	}
}

func TestNewGSURLParse(t *testing.T) {
	tests := []struct {
		desc      string
		url       string
		bucket    string
		prefix    string
		expectErr bool
	}{
		{"basic", "gs://mybucket", "mybucket", "", false},
		{"with prefix", "gs://mybucket/team/app", "mybucket", "team/app/", false},
		{"missing bucket", "gs:///team/app", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			g, err := NewGSWithClient(context.Background(), tt.url, nil, newMockGSClient())
			if tt.expectErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if g.Bucket != tt.bucket {
				t.Errorf("bucket: got %q want %q", g.Bucket, tt.bucket)
			}
			if g.Prefix != tt.prefix {
				t.Errorf("prefix: got %q want %q", g.Prefix, tt.prefix)
			}
		})
	}
}

// Compile-time check that *GS satisfies Cache and AtomicCache.
var (
	_ Cache       = (*GS)(nil)
	_ AtomicCache = (*GS)(nil)
)

func TestGSAtomicWriteIfAbsent(t *testing.T) {
	g, mock := newTestGS(t)

	v1, err := g.WriteIfMatch("k", "", []byte("first"))
	if err != nil {
		t.Fatalf("first WriteIfMatch: %v", err)
	}
	if v1 == "" || v1 == "0" {
		t.Errorf("expected non-empty version, got %q", v1)
	}

	if _, err := g.WriteIfMatch("k", "", []byte("second")); err == nil {
		t.Fatal("expected conflict on second write-if-absent")
	} else if !IsConflict(err) {
		t.Errorf("expected IsConflict, got %v", err)
	}

	if got := mock.objects["team/app/k"]; string(got) != "first" {
		t.Errorf("object overwritten unexpectedly: %q", got)
	}
}

func TestGSAtomicReadWriteIfMatch(t *testing.T) {
	g, _ := newTestGS(t)

	v1, err := g.WriteIfMatch("k", "", []byte("v1"))
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	data, version, err := g.ReadWithVersion("k")
	if err != nil {
		t.Fatalf("ReadWithVersion: %v", err)
	}
	if string(data) != "v1" || version != v1 {
		t.Errorf("unexpected read: data=%q version=%q want %q", data, version, v1)
	}

	v2, err := g.WriteIfMatch("k", version, []byte("v2"))
	if err != nil {
		t.Fatalf("CAS with current version: %v", err)
	}
	if v2 == version {
		t.Error("expected new version different from previous")
	}

	if _, err := g.WriteIfMatch("k", version, []byte("v3")); err == nil {
		t.Fatal("expected conflict on stale CAS")
	} else if !IsConflict(err) {
		t.Errorf("expected IsConflict, got %v", err)
	}

	if _, err := g.WriteIfMatch("k", v2, []byte("v3")); err != nil {
		t.Fatalf("CAS after stale fail: %v", err)
	}
}

func TestGSAtomicReadWithVersionNotFound(t *testing.T) {
	g, _ := newTestGS(t)
	_, _, err := g.ReadWithVersion("missing")
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsNotFound(err) {
		t.Errorf("expected IsNotFound, got %v", err)
	}
}

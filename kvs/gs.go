package kvs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

const gsFormat = "gs://<bucket>/<prefix>"

// GSClient is the storage operation surface used by GS backend, for testability.
type GSClient interface {
	GetObject(ctx context.Context, bucket, name string) ([]byte, error)
	PutObject(ctx context.Context, bucket, name string, data []byte) error
	DeleteObject(ctx context.Context, bucket, name string) error
	ListObjects(ctx context.Context, bucket, prefix string) ([]string, error)
}

// GS is a Google Cloud Storage backed cache with local filesystem staging.
type GS struct {
	Bucket string
	Prefix string

	cl  GSClient
	ctx context.Context

	dir     string
	MaxSize int64
	logger  *slog.Logger
}

// NewGS returns a GS cache backend configured from a URL.
func NewGS(ctx context.Context, u string, log *slog.Logger) (*GS, error) {
	return NewGSWithClient(ctx, u, log, nil)
}

// NewGSWithClient is like NewGS but lets callers inject a custom client (for testing).
func NewGSWithClient(ctx context.Context, u string, log *slog.Logger, client GSClient) (*GS, error) {
	ur, err := url.Parse(u)
	if err != nil {
		return nil, err
	}

	bucket := ur.Host
	prefix := normalizePrefix(strings.TrimPrefix(ur.Path, "/"))

	if bucket == "" {
		return nil, fmt.Errorf("bucket is required: %s", gsFormat)
	}

	g := &GS{
		Bucket:  bucket,
		Prefix:  prefix,
		ctx:     ctx,
		dir:     DefaultCacheDir,
		MaxSize: DefaultMaxSize,
		logger:  log,
	}

	if client != nil {
		g.cl = client
		return g, nil
	}

	sc, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	g.cl = &gsStorageClient{client: sc}
	return g, nil
}

// SetLogger sets the logger.
func (g *GS) SetLogger(logger *slog.Logger) { g.logger = logger }

// SetDir sets the local staging directory.
func (g *GS) SetDir(dir string) { g.dir = dir }

// GetDir returns the local staging directory.
func (g *GS) GetDir() string { return g.dir }

func (g *GS) objectName(key string) string { return g.Prefix + key }

// Read returns cache data for key, fetching from GCS and staging locally on miss.
func (g *GS) Read(key string) ([]byte, error) {
	localPath, err := validateKeyPath(g.dir, key)
	if err != nil {
		return nil, err
	}

	if data, err := os.ReadFile(localPath); err == nil {
		return data, nil
	}

	data, err := g.cl.GetObject(g.ctx, g.Bucket, g.objectName(key))
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			return nil, fmt.Errorf("%w: %s", errNotFound, key)
		}
		return nil, fmt.Errorf("failed to get object: %w", err)
	}

	if err := g.stageLocal(localPath, data); err != nil && g.logger != nil {
		g.logger.Warn("Failed to stage GCS object locally",
			slog.String("path", localPath), slog.String("error", err.Error()))
	}
	return data, nil
}

// Write stores data both locally and on GCS.
func (g *GS) Write(key string, data []byte) error {
	localPath, err := validateKeyPath(g.dir, key)
	if err != nil {
		return err
	}

	if err := g.stageLocal(localPath, data); err != nil {
		return err
	}

	if err := g.cl.PutObject(g.ctx, g.Bucket, g.objectName(key), data); err != nil {
		return fmt.Errorf("failed to put object: %w", err)
	}

	if g.logger != nil {
		g.logger.Info("Write GCS object",
			slog.String("bucket", g.Bucket),
			slog.String("name", g.objectName(key)))
	}
	return nil
}

// Delete removes the entry from both local staging and GCS.
func (g *GS) Delete(key string) error {
	localPath, err := validateKeyPath(g.dir, key)
	if err != nil {
		return err
	}

	if IsFileExist(localPath) {
		if err := os.Remove(localPath); err != nil {
			return err
		}
	}

	if err := g.cl.DeleteObject(g.ctx, g.Bucket, g.objectName(key)); err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			return nil
		}
		return fmt.Errorf("failed to delete object: %w", err)
	}
	return nil
}

// List returns cache keys present in GCS under the configured prefix.
func (g *GS) List() ([]string, error) {
	names, err := g.cl.ListObjects(g.ctx, g.Bucket, g.Prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list objects: %w", err)
	}

	keys := make([]string, 0, len(names))
	for _, n := range names {
		k := strings.TrimPrefix(n, g.Prefix)
		if k == "" {
			continue
		}
		keys = append(keys, k)
	}
	return keys, nil
}

func (g *GS) stageLocal(p string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}

// gsStorageClient is the default GSClient backed by *storage.Client.
type gsStorageClient struct {
	client *storage.Client
}

func (c *gsStorageClient) GetObject(ctx context.Context, bucket, name string) ([]byte, error) {
	r, err := c.client.Bucket(bucket).Object(name).NewReader(ctx)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

func (c *gsStorageClient) PutObject(ctx context.Context, bucket, name string, data []byte) error {
	w := c.client.Bucket(bucket).Object(name).NewWriter(ctx)
	if _, err := w.Write(data); err != nil {
		_ = w.Close()
		return err
	}
	return w.Close()
}

func (c *gsStorageClient) DeleteObject(ctx context.Context, bucket, name string) error {
	return c.client.Bucket(bucket).Object(name).Delete(ctx)
}

func (c *gsStorageClient) ListObjects(ctx context.Context, bucket, prefix string) ([]string, error) {
	it := c.client.Bucket(bucket).Objects(ctx, &storage.Query{Prefix: prefix})
	var names []string
	for {
		attrs, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, err
		}
		if attrs.Name != "" {
			names = append(names, attrs.Name)
		}
	}
	return names, nil
}

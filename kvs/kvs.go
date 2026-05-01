package kvs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"
)

// KVS interface.
type KVS interface {
	Read(key string) ([]byte, error)
	Write(key string, data []byte) error
	Delete(key string) error
	List() ([]string, error)
	GetDir() string
}

// Config struct.
type Config struct {
}

// New returns KVS backend by URL scheme.
//
// Supported schemes:
//   - "" or "file": local filesystem cache (default).
//   - "s3://<region>/<bucket>/<prefix>": Amazon S3 backed cache with local staging.
//   - "gs://<bucket>/<prefix>": Google Cloud Storage backed cache with local staging.
//
// An empty urlStr returns the default file backend.
func New(ctx context.Context, urlStr string, log *slog.Logger) (KVS, error) {
	if urlStr == "" {
		f := &File{}
		f.Default()
		f.SetLogger(log)
		return f, nil
	}

	scheme := schemeOf(urlStr)
	switch scheme {
	case "file":
		f := &File{}
		f.Default()
		f.SetLogger(log)
		// Allow file:///custom/path to override the default dir.
		if p := strings.TrimPrefix(urlStr, "file://"); p != "" && p != urlStr {
			f.SetDir(p)
		}
		return f, nil
	case "s3":
		return NewS3(ctx, urlStr, log)
	case "gs":
		return NewGS(ctx, urlStr, log)
	default:
		return nil, fmt.Errorf("unsupported cache scheme: %s", scheme)
	}
}

func schemeOf(urlStr string) string {
	if i := strings.Index(urlStr, "://"); i >= 0 {
		return urlStr[:i]
	}
	u, err := url.Parse(urlStr)
	if err != nil {
		return ""
	}
	return u.Scheme
}

// errNotFound indicates a missing cache entry.
var errNotFound = errors.New("not found")

// IsNotFound reports whether err indicates a missing cache entry.
func IsNotFound(err error) bool {
	return errors.Is(err, errNotFound)
}

// nolint
type item struct {
	content    []byte
	lock       sync.Mutex
	expiration time.Time
	size       uint64
}

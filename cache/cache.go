package cache

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

// Cache interface.
type Cache interface {
	Read(key string) ([]byte, error)
	Write(key string, data []byte) error
	Delete(key string) error
	List() ([]string, error)
	GetDir() string
	// RegistryTTL returns the TTL configured for caching upstream registry
	// responses against this backend. A non-zero value opts the backend
	// in to acting as a shared registry-result cache. Backends parse this
	// from the "registry-ttl" URL query parameter at construction time.
	RegistryTTL() time.Duration
}

// New returns a Cache backend by URL scheme.
//
// Supported schemes:
//   - "" or "file": local filesystem cache (default).
//   - "s3://<region>/<bucket>/<prefix>": Amazon S3 backed cache with local staging.
//   - "gs://<bucket>/<prefix>": Google Cloud Storage backed cache with local staging.
//
// An empty urlStr returns the default file backend.
func New(ctx context.Context, urlStr string, log *slog.Logger) (Cache, error) {
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
		u, err := url.Parse(urlStr)
		if err != nil {
			return nil, err
		}
		// Allow file:///custom/path to override the default dir.
		if p := u.Path; p != "" {
			f.SetDir(p)
		}
		ttl, err := parseRegistryTTL(u.Query())
		if err != nil {
			return nil, err
		}
		f.SetRegistryTTL(ttl)
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

// ErrNotFound indicates a missing cache entry. Backends wrap this to
// surface a not-found condition; callers detect it via IsNotFound.
var ErrNotFound = errors.New("not found")

// IsNotFound reports whether err indicates a missing cache entry.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// ErrConflict indicates that a conditional write's precondition did not
// match. Backends wrap this to surface a precondition mismatch; callers
// detect it via IsConflict.
var ErrConflict = errors.New("precondition failed")

// IsConflict reports whether err indicates a conditional-write precondition mismatch.
func IsConflict(err error) bool {
	return errors.Is(err, ErrConflict)
}

// AtomicCache is an optional capability for cache backends that support
// conditional writes. Cloud backends (S3, GCS) implement it so that callers
// can coordinate writes across instances without an external lock service.
//
// version is an opaque token returned by ReadWithVersion. Pass it back in
// WriteIfMatch to perform a "write only if the entry has not changed" update.
// Pass version="" to perform a "write only if no entry exists" update.
//
// On precondition mismatch WriteIfMatch returns an error for which IsConflict
// returns true; the caller is expected to re-read and retry as appropriate.
type AtomicCache interface {
	Cache
	ReadWithVersion(key string) (data []byte, version string, err error)
	WriteIfMatch(key string, version string, data []byte) (newVersion string, err error)
}

// parseRegistryTTL parses the "registry-ttl" query parameter from a cache URL.
// Empty or unset means 0 (no registry-result caching).
func parseRegistryTTL(values url.Values) (time.Duration, error) {
	v := values.Get("registry-ttl")
	if v == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("invalid registry-ttl %q: %w", v, err)
	}
	if d < 0 {
		return 0, fmt.Errorf("registry-ttl must be non-negative, got %s", d)
	}
	return d, nil
}

// nolint
type item struct {
	content    []byte
	lock       sync.Mutex
	expiration time.Time
	size       uint64
}

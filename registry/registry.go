package registry

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gorilla/schema"
	"github.com/linyows/dewy/internal/scheme"
	"github.com/linyows/dewy/logging"
)

var decoder = schema.NewDecoder()

// factoryFn constructs a Registry from a parsed URL. logger is optional
// (NewGRPC ignores it) but kept in the signature so all registries plug into
// the same dispatch table.
type factoryFn func(ctx context.Context, url string, log *logging.Logger) (Registry, error)

var factories = map[string]factoryFn{
	scheme.GHR: func(ctx context.Context, url string, log *logging.Logger) (Registry, error) {
		return NewGHR(ctx, url, log)
	},
	scheme.S3: func(ctx context.Context, url string, log *logging.Logger) (Registry, error) {
		return NewS3(ctx, url, log)
	},
	scheme.GS: func(ctx context.Context, url string, log *logging.Logger) (Registry, error) {
		return NewGS(ctx, url, log)
	},
	scheme.GRPC: func(ctx context.Context, url string, _ *logging.Logger) (Registry, error) {
		return NewGRPC(ctx, url)
	},
	scheme.OCI: func(ctx context.Context, url string, log *logging.Logger) (Registry, error) {
		return NewOCI(ctx, url, log)
	},
}

type Registry interface {
	// Current returns the current artifact.
	Current(context.Context) (*CurrentResponse, error)
	// Report reports the result of deploying the artifact.
	Report(context.Context, *ReportRequest) error
}

// CurrentResponse is the response to get the current artifact.
type CurrentResponse struct {
	// ID uniquely identifies the response.
	ID string
	// Tag uniquely identifies the artifact concerned.
	Tag string
	// ArtifactURL is the URL to download the artifact.
	// The URL is not only "https://"
	ArtifactURL string
	// CreatedAt is the creation time of the release
	CreatedAt *time.Time
	// Slot is the deployment slot extracted from build metadata (e.g., "blue", "green").
	// This is used for blue/green deployment support.
	Slot string
}

// ReportRequest is the request to report the result of deploying the artifact.
type ReportRequest struct {
	// ID is the ID of the response.
	ID string
	// Tag is the current tag of deployed artifact.
	Tag string
	// Command is the command that was used for deployment (server or assets).
	Command string
	// Err is the error that occurred during deployment. If Err is nil, the deployment is considered successful.
	Err error
}

func New(ctx context.Context, url string, log *logging.Logger) (Registry, error) {
	splitted := strings.SplitN(url, "://", 2)
	if f, ok := factories[splitted[0]]; ok {
		return f(ctx, url, log)
	}
	return nil, fmt.Errorf("unsupported registry: %s", url)
}

// extractSlot extracts the deployment slot (build metadata) from a version tag.
// If calverFormat is non-empty, it tries CalVer parsing first, then falls back to SemVer.
// If calverFormat is empty, it uses SemVer parsing.
func extractSlot(tag, calverFormat string) string {
	if calverFormat != "" {
		if f, err := NewCalVerFormat(calverFormat); err == nil {
			if cv := f.Parse(tag); cv != nil {
				return cv.BuildMetadata
			}
		}
	}
	if sv := ParseSemVer(tag); sv != nil {
		return sv.BuildMetadata
	}
	return ""
}

func addTrailingSlash(path string) string {
	if strings.HasSuffix(path, "/") {
		return path
	}
	return path + "/"
}

func removeTrailingSlash(path string) string {
	return strings.TrimSuffix(path, "/")
}

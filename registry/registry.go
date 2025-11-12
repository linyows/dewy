package registry

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gorilla/schema"
	"github.com/linyows/dewy/logging"
)

var (
	decoder      = schema.NewDecoder()
	s3Scheme     = "s3"
	ghrScheme    = "ghr"
	grpcScheme   = "grpc"
	gsScheme     = "gs"
	dockerScheme = "docker"
	ociScheme    = "oci"
	imgScheme    = "img"
)

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

	switch splitted[0] {
	case ghrScheme:
		return NewGHR(ctx, url, log)

	case s3Scheme:
		return NewS3(ctx, url, log)

	case gsScheme:
		return NewGS(ctx, url, log)

	case grpcScheme:
		return NewGRPC(ctx, url)

	case dockerScheme, ociScheme, imgScheme:
		return NewOCI(ctx, url, log)
	}

	return nil, fmt.Errorf("unsupported registry: %s", url)
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

package registry

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/gorilla/schema"
)

var (
	decoder    = schema.NewDecoder()
	s3Scheme   = "s3"
	ghrScheme  = "ghr"
	grpcScheme = "grpc"
)

type Registry interface {
	// Current returns the current artifact.
	Current(context.Context, *CurrentRequest) (*CurrentResponse, error)
	// Report reports the result of deploying the artifact.
	Report(context.Context, *ReportRequest) error
}

// CurrentRequest is the request to get the current artifact.
type CurrentRequest struct {
	// Arch is the CPU architecture of deployment environment.
	Arch string
	// OS is the operating system of deployment environment.
	OS string
	// ArtifactName is the name of the artifact to fetch.
	// FIXME: If possible, ArtifactName should be optional.
	ArtifactName string
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
}

// ReportRequest is the request to report the result of deploying the artifact.
type ReportRequest struct {
	// ID is the ID of the response.
	ID string
	// Tag is the current tag of deployed artifact.
	Tag string
	// Err is the error that occurred during deployment. If Err is nil, the deployment is considered successful.
	Err error
}

func New(strUrl string) (Registry, error) {
	splitted := strings.SplitN(strUrl, "://", 2)

	switch splitted[0] {
	case ghrScheme:
		u, err := url.Parse(splitted[1])
		if err != nil {
			return nil, err
		}

		ownerrepo := strings.SplitN(u.Path, "/", 2)
		gr, err := NewGHR(ownerrepo[0], ownerrepo[1])
		if err != nil {
			return nil, err
		}
		if err := decoder.Decode(gr, u.Query()); err != nil {
			return nil, err
		}

		return gr, nil

	case s3Scheme:
		s3, err := NewS3(splitted[1])
		if err != nil {
			return nil, err
		}
		return s3, nil

	case grpcScheme:
		u, err := url.Parse(strUrl)
		if err != nil {
			return nil, err
		}

		var gr GRPC
		if err := decoder.Decode(&gr, u.Query()); err != nil {
			return nil, err
		}
		if err := gr.Dial(u.Host); err != nil {
			return nil, err
		}

		return &gr, nil
	}

	return nil, fmt.Errorf("unsupported registry: %s", strUrl)
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

package registory

type Registory interface {
	// Current returns the current artifact.
	Current(*CurrentRequest) (*CurrentResponse, error)
	// Report reports the result of deploying the artifact.
	Report(*ReportRequest) error
}

// CurrentRequest is the request to get the current artifact.
type CurrentRequest struct {
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

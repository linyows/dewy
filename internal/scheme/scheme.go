// Package scheme defines the URL scheme strings shared between the registry
// and artifact factories. Centralizing the names prevents drift between the
// two factories (which historically duplicated the same set of constants).
package scheme

const (
	GHR  = "ghr"  // GitHub Releases
	S3   = "s3"   // Amazon S3
	GS   = "gs"   // Google Cloud Storage
	GRPC = "grpc" // gRPC registry endpoint
	OCI  = "img"  // OCI / container image registry
)

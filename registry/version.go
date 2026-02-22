package registry

// Version is a common interface for version types (SemVer, CalVer).
type Version interface {
	String() string
	GetBuildMetadata() string
}

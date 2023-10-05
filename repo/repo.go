package repo

// Config struct.
type Config struct {
	Owner                 string
	Repo                  string
	Artifact              string
	PreRelease            bool
	DisableRecordShipping bool // FIXME: For testing. Remove this.
}

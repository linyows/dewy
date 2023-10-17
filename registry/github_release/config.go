package ghrelease

// Config struct.
type Config struct {
	Owner                 string `schema:"-"`
	Repo                  string `schema:"-"`
	Artifact              string `schema:"artifact"`
	PreRelease            bool   `schema:"pre-release"`
	DisableRecordShipping bool   // FIXME: For testing. Remove this.
}

package storage

import "io"

// Fetcher is the interface that wraps the Fetch method.
type Fetcher interface {
	// Fetch fetches the artifact from the storage.
	Fetch(url string, w io.Writer) error
}

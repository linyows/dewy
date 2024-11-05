package artifact

import (
	"io"
	"log"

	"github.com/k1LoW/remote"
)

type GCS struct{}

func NewGCS() (*GCS, error) {
	return &GCS{}, nil
}

func (s *GCS) Fetch(url string, w io.Writer) error {
	f, err := remote.Open(url)
	if err != nil {
		return err
	}
	defer f.Close()
	log.Printf("[INFO] Downloaded from %s", url)
	if _, err := io.Copy(w, f); err != nil {
		return err
	}
	return nil
}

package gcs

import (
	"io"
	"log"

	"github.com/k1LoW/remote"
)

const (
	Scheme      = "gcs"
	SchemeShort = "gs"
)

type GCS struct{}

func New() (*GCS, error) {
	return &GCS{}, nil
}

func (s *GCS) Fetch(urlstr string, w io.Writer) error {
	f, err := remote.Open(urlstr)
	if err != nil {
		return err
	}
	defer f.Close()
	log.Printf("[INFO] Downloaded from %s", urlstr)
	if _, err := io.Copy(w, f); err != nil {
		return err
	}
	return nil
}

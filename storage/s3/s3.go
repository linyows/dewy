package s3

import (
	"io"
	"log"

	"github.com/k1LoW/remote"
)

const Scheme = "s3"

type S3 struct{}

func New() (*S3, error) {
	return &S3{}, nil
}

func (s *S3) Fetch(urlstr string, w io.Writer) error {
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

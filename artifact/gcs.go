package artifact

import (
	"context"
	"io"
	"log"

	"github.com/k1LoW/remote"
)

type GCS struct {
	url string
}

func NewGCS(ctx context.Context, u string) (*GCS, error) {
	return &GCS{url: u}, nil
}

func (g *GCS) Download(ctx context.Context, w io.Writer) error {
	f, err := remote.Open(g.url)
	if err != nil {
		return err
	}
	defer f.Close()
	log.Printf("[INFO] Downloaded from %s", g.url)
	if _, err := io.Copy(w, f); err != nil {
		return err
	}
	return nil
}

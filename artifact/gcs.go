package artifact

import (
	"context"
	"io"
	"log/slog"

	"github.com/k1LoW/remote"
)

type GCS struct {
	url    string
	logger *slog.Logger
}

func NewGCS(ctx context.Context, u string, logger *slog.Logger) (*GCS, error) {
	return &GCS{url: u, logger: logger}, nil
}

func (g *GCS) Download(ctx context.Context, w io.Writer) error {
	f, err := remote.Open(g.url)
	if err != nil {
		return err
	}
	defer f.Close()
	g.logger.Info("Downloaded from GCS", slog.String("url", g.url))
	if _, err := io.Copy(w, f); err != nil {
		return err
	}
	return nil
}

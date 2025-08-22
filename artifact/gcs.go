package artifact

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"strings"

	"cloud.google.com/go/storage"
)

type GS struct {
	Bucket string `schema:"-"`
	Object string `schema:"-"`
	url    string
	cl     GSClient
	logger *slog.Logger
}

// gs://<bucket>/<object>
func NewGS(ctx context.Context, strUrl string, logger *slog.Logger) (*GS, error) {
	u, err := url.Parse(strUrl)
	if err != nil {
		return nil, err
	}

	if u.Scheme != gsScheme {
		return nil, fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}

	bucket := u.Host
	pathParts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")

	if bucket == "" || len(pathParts) < 2 {
		return nil, fmt.Errorf("url parse error: %s (format: gs://<bucket>/<object>)", strUrl)
	}

	object := strings.Join(pathParts[0:], "/")

	if object == "" {
		return nil, fmt.Errorf("url parse error: %s (format: gs://<bucket>/<object>)", strUrl)
	}

	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create GS client: %w", err)
	}

	g := &GS{
		Bucket: bucket,
		Object: object,
		url:    strUrl,
		cl:     client,
		logger: logger,
	}

	return g, nil
}

func (g *GS) Download(ctx context.Context, w io.Writer) error {
	reader, err := g.cl.Bucket(g.Bucket).Object(g.Object).NewReader(ctx)
	if err != nil {
		return fmt.Errorf("failed to download artifact from Google Cloud Storage: %w", err)
	}
	defer reader.Close()

	g.logger.Info("Downloaded from GS", slog.String("url", g.url))
	_, err = io.Copy(w, reader)
	if err != nil {
		return fmt.Errorf("failed to write artifact to writer: %w", err)
	}

	return nil
}

type GSClient interface {
	Bucket(name string) *storage.BucketHandle
}

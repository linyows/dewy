package artifact

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gorilla/schema"
)

var decoder = schema.NewDecoder()

type S3 struct {
	Region   string `schema:"-"`
	Bucket   string `schema:"-"`
	Key      string `schema:"-"`
	Endpoint string `schema:"endpoint"`
	url      string
	cl       S3Client
	logger   *slog.Logger
}

// s3://<region>/<bucket>/<key>?endpoint=bbb"
func NewS3(ctx context.Context, strUrl string, logger *slog.Logger) (*S3, error) {
	u, err := url.Parse(strUrl)
	if err != nil {
		return nil, err
	}

	splitted := strings.SplitN(strings.TrimPrefix(u.Path, "/"), "/", 2)

	if len(splitted) < 2 {
		return nil, fmt.Errorf("url parse error: %s", strUrl)
	}

	s := &S3{
		Region: u.Host,
		Bucket: splitted[0],
		Key:    splitted[1],
		url:    strUrl,
		logger: logger,
	}
	if err = decoder.Decode(s, u.Query()); err != nil {
		return nil, err
	}

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(s.Region))
	if err != nil {
		return nil, err
	}

	if s.Endpoint != "" {
		s.cl = s3.NewFromConfig(cfg, func(o *s3.Options) {
			o.UsePathStyle = true
			o.BaseEndpoint = aws.String(s.Endpoint)
		})
	} else if e := os.Getenv("AWS_ENDPOINT_URL"); e != "" {
		s.cl = s3.NewFromConfig(cfg, func(o *s3.Options) {
			o.UsePathStyle = true
		})
	} else {
		s.cl = s3.NewFromConfig(cfg)
	}

	return s, nil
}

func (s *S3) Download(ctx context.Context, w io.Writer) error {
	res, err := s.cl.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(s.Key),
	})
	if err != nil {
		return fmt.Errorf("failed to download artifact from S3: %w", err)
	}
	defer res.Body.Close()

	s.logger.Info("Downloaded from S3", slog.String("url", s.url))
	_, err = io.Copy(w, res.Body)
	if err != nil {
		return fmt.Errorf("failed to write artifact to writer: %w", err)
	}

	return nil
}

type S3Client interface {
	GetObject(ctx context.Context, input *s3.GetObjectInput, opts ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

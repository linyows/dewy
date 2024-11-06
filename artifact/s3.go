package artifact

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/gorilla/schema"
)

var decoder = schema.NewDecoder()

type S3 struct {
	Bucket   string `schema:"-"`
	Key      string `schema:"-"`
	Region   string `schema:"region"`
	Endpoint string `schema:"endpoint"`
	cl       S3Client
}

// s3://<bucket>/<key>?region=aaa&endpoint=bbb"
func NewS3(path string) (*S3, error) {
	u, err := url.Parse(path)
	if err != nil {
		return nil, err
	}

	splitted := strings.SplitN(u.Path, "/", 2)

	s := &S3{
		Bucket: splitted[0],
		Key:    splitted[1],
	}
	if err = decoder.Decode(s, u.Query()); err != nil {
		return nil, err
	}

	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(s.Region))
	if err != nil {
		return nil, err
	}

	if s.Endpoint != "" {
		s.cl = s3.NewFromConfig(cfg, func(o *s3.Options) {
			o.EndpointResolver = s3.EndpointResolverFromURL(s.Endpoint)
		})
	} else {
		s.cl = s3.NewFromConfig(cfg)
	}

	return s, nil
}

func (s *S3) Fetch(url string, w io.Writer) error {
	res, err := s.cl.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(s.Key),
	})
	if err != nil {
		return fmt.Errorf("failed to download artifact from S3: %w", err)
	}
	defer res.Body.Close()

	log.Printf("[INFO] Downloaded from %s", url)
	_, err = io.Copy(w, res.Body)
	if err != nil {
		return fmt.Errorf("failed to write artifact to writer: %w", err)
	}

	return nil
}

type S3Client interface {
	GetObject(ctx context.Context, input *s3.GetObjectInput, opts ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

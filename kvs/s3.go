package kvs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

const s3Format = "s3://<region>/<bucket>/<prefix>"

// S3Client is the subset of *s3.Client used by S3 backend, for testability.
type S3Client interface {
	GetObject(ctx context.Context, in *s3.GetObjectInput, opts ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	PutObject(ctx context.Context, in *s3.PutObjectInput, opts ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	DeleteObject(ctx context.Context, in *s3.DeleteObjectInput, opts ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	ListObjectsV2(ctx context.Context, in *s3.ListObjectsV2Input, opts ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
}

// S3 is an S3-backed cache with local filesystem staging.
//
// S3 is the source of truth for cross-node sharing; writes go to both the
// local staging directory and S3, and reads fall back from local to S3.
type S3 struct {
	Bucket   string
	Prefix   string
	Region   string
	Endpoint string

	cl  S3Client
	ctx context.Context

	dir     string
	MaxSize int64
	logger  *slog.Logger
}

// NewS3 returns an S3 cache backend configured from a URL.
func NewS3(ctx context.Context, u string, log *slog.Logger) (*S3, error) {
	ur, err := url.Parse(u)
	if err != nil {
		return nil, err
	}

	after, _ := strings.CutPrefix(ur.Path, "/")
	parts := strings.SplitN(after, "/", 2)
	bucket := ""
	prefix := ""
	if len(parts) > 0 {
		bucket = parts[0]
	}
	if len(parts) > 1 {
		prefix = parts[1]
	}
	prefix = normalizePrefix(prefix)

	s := &S3{
		Region:   ur.Host,
		Bucket:   bucket,
		Prefix:   prefix,
		Endpoint: ur.Query().Get("endpoint"),
		ctx:      ctx,
		dir:      DefaultCacheDir,
		MaxSize:  DefaultMaxSize,
		logger:   log,
	}

	if s.Region == "" {
		return nil, fmt.Errorf("region is required: %s", s3Format)
	}
	if s.Bucket == "" {
		return nil, fmt.Errorf("bucket is required: %s", s3Format)
	}

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(s.Region))
	if err != nil {
		return nil, err
	}

	switch {
	case s.Endpoint != "":
		s.cl = s3.NewFromConfig(cfg, func(o *s3.Options) {
			o.UsePathStyle = true
			o.BaseEndpoint = aws.String(s.Endpoint)
		})
	case os.Getenv("AWS_ENDPOINT_URL") != "":
		s.cl = s3.NewFromConfig(cfg, func(o *s3.Options) {
			o.UsePathStyle = true
		})
	default:
		s.cl = s3.NewFromConfig(cfg)
	}

	return s, nil
}

// SetLogger sets the logger.
func (s *S3) SetLogger(logger *slog.Logger) { s.logger = logger }

// SetDir sets the local staging directory.
func (s *S3) SetDir(dir string) { s.dir = dir }

// GetDir returns the local staging directory.
func (s *S3) GetDir() string { return s.dir }

// objectKey returns the full S3 object key for a cache key.
func (s *S3) objectKey(key string) string { return s.Prefix + key }

// Read returns cache data for key, fetching from S3 and staging locally on miss.
func (s *S3) Read(key string) ([]byte, error) {
	localPath, err := validateKeyPath(s.dir, key)
	if err != nil {
		return nil, err
	}

	if data, err := os.ReadFile(localPath); err == nil {
		return data, nil
	}

	out, err := s.cl.GetObject(s.ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(s.objectKey(key)),
	})
	if err != nil {
		var nsk *s3types.NoSuchKey
		var nf *s3types.NotFound
		if errors.As(err, &nsk) || errors.As(err, &nf) {
			return nil, fmt.Errorf("%w: %s", errNotFound, key)
		}
		return nil, fmt.Errorf("failed to get object: %w", err)
	}
	defer out.Body.Close()

	data, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read object body: %w", err)
	}

	if err := s.stageLocal(localPath, data); err != nil && s.logger != nil {
		s.logger.Warn("Failed to stage S3 object locally",
			slog.String("path", localPath), slog.String("error", err.Error()))
	}
	return data, nil
}

// Write stores data both locally and on S3.
func (s *S3) Write(key string, data []byte) error {
	localPath, err := validateKeyPath(s.dir, key)
	if err != nil {
		return err
	}

	if err := s.stageLocal(localPath, data); err != nil {
		return err
	}

	_, err = s.cl.PutObject(s.ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(s.objectKey(key)),
		Body:   bytes.NewReader(data),
	})
	if err != nil {
		return fmt.Errorf("failed to put object: %w", err)
	}

	if s.logger != nil {
		s.logger.Info("Write S3 object",
			slog.String("bucket", s.Bucket),
			slog.String("key", s.objectKey(key)))
	}
	return nil
}

// Delete removes the entry from both local staging and S3.
func (s *S3) Delete(key string) error {
	localPath, err := validateKeyPath(s.dir, key)
	if err != nil {
		return err
	}

	if IsFileExist(localPath) {
		if err := os.Remove(localPath); err != nil {
			return err
		}
	}

	_, err = s.cl.DeleteObject(s.ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(s.objectKey(key)),
	})
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}
	return nil
}

// List returns cache keys present in S3 under the configured prefix.
func (s *S3) List() ([]string, error) {
	var keys []string
	var token *string

	for {
		out, err := s.cl.ListObjectsV2(s.ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(s.Bucket),
			Prefix:            aws.String(s.Prefix),
			ContinuationToken: token,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}
		for _, obj := range out.Contents {
			if obj.Key == nil {
				continue
			}
			name := strings.TrimPrefix(*obj.Key, s.Prefix)
			if name == "" {
				continue
			}
			keys = append(keys, name)
		}
		if out.IsTruncated == nil || !*out.IsTruncated {
			break
		}
		token = out.NextContinuationToken
	}

	return keys, nil
}

func (s *S3) stageLocal(p string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	return os.WriteFile(p, data, 0644)
}

// normalizePrefix ensures the prefix has a trailing slash when non-empty.
func normalizePrefix(p string) string {
	p = strings.TrimPrefix(p, "/")
	if p == "" {
		return ""
	}
	if !strings.HasSuffix(p, "/") {
		p += "/"
	}
	return p
}

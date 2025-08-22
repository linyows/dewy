package registry

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	awslogging "github.com/aws/smithy-go/logging"
	"github.com/linyows/dewy/logging"
)

const (
	s3Format string = "s3://<region>/<bucket>/<prefix>"
)

// S3 struct.
type S3 struct {
	Bucket     string `schema:"-"`
	Prefix     string `schema:"-"`
	Region     string `schema:"region"`
	Endpoint   string `schema:"endpoint"`
	Artifact   string `schema:"artifact"`
	PreRelease bool   `schema:"pre-release"`
	cl         S3Client
	pager      ListObjectsV2Pager
	logger     *logging.Logger
}

// NewS3 returns S3.
func NewS3(ctx context.Context, u string, log *logging.Logger) (*S3, error) {
	ur, err := url.Parse(u)
	if err != nil {
		return nil, err
	}

	after, _ := strings.CutPrefix(ur.Path, "/")
	splitted := strings.SplitN(after, "/", 2)
	bucket := ""
	prefix := ""
	if len(splitted) > 0 {
		bucket = splitted[0]
	}
	if len(splitted) > 1 {
		prefix = strings.TrimPrefix(addTrailingSlash(splitted[1]), "/")
	}

	s := &S3{
		Region: ur.Host,
		Bucket: bucket,
		Prefix: prefix,
	}
	if err = decoder.Decode(s, ur.Query()); err != nil {
		return nil, err
	}

	if s.Region == "" {
		return nil, fmt.Errorf("region is required: %s", s3Format)
	}
	if s.Bucket == "" {
		return nil, fmt.Errorf("bucket is required: %s", s3Format)
	}

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(s.Region),
		config.WithLogger(&customLogger{Logger: log}),
		config.WithClientLogMode(aws.LogResponse),
	)
	if err != nil {
		return nil, err
	}

	if s.Endpoint != "" {
		s.cl = s3.NewFromConfig(cfg, func(o *s3.Options) {
			// path-style: https://s3.region.amazonaws.com/<bucket>/<key>
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

	s.logger = log
	return s, nil
}

// Current returns current artifact.
func (s *S3) Current(ctx context.Context) (*CurrentResponse, error) {
	prefix, version, err := s.LatestVersion(ctx)
	if err != nil {
		return nil, err
	}

	objects, err := s.ListObjects(ctx, prefix)
	if err != nil {
		return nil, err
	}

	var artifactName string
	var createdAt *time.Time
	found := false

	if s.Artifact != "" {
		artifactName = s.Artifact
		for _, v := range objects {
			name := s.extractFilenameFromObjectKey(*v.Key, prefix)
			if name == artifactName {
				found = true
				createdAt = v.LastModified
				s.logger.Debug("Fetched S3 version", slog.Any("version", v))
				break
			}
		}

	} else {
		// Extract object names
		var objectNames []string
		var objectMap = make(map[string]*types.Object)
		for _, v := range objects {
			name := s.extractFilenameFromObjectKey(*v.Key, prefix)
			objectNames = append(objectNames, name)
			objectMap[name] = &v
		}

		// Use common pattern matching
		var matchedName string
		matchedName, found = MatchArtifactByPlatform(objectNames)
		if found {
			artifactName = matchedName
			if obj, exists := objectMap[matchedName]; exists {
				createdAt = obj.LastModified
				s.logger.Debug("Fetched S3 object", slog.Any("object", obj))
			}
		}
	}

	if !found {
		// Only get the creation time when artifact is not found
		artifactCreatedAt, _ := s.getVersionDirectoryCreatedAt(ctx, prefix)
		return nil, &ArtifactNotFoundError{
			ArtifactName: prefix + artifactName,
			ReleaseTime:  artifactCreatedAt,
			Message:      fmt.Sprintf("artifact not found: %s%s", prefix, artifactName),
		}
	}

	return &CurrentResponse{
		ID:          time.Now().Format(ISO8601),
		Tag:         version.String(),
		ArtifactURL: s.buildArtifactURL(prefix + artifactName),
		CreatedAt:   createdAt,
	}, nil
}

func (s *S3) buildArtifactURL(key string) string {
	var q []string
	var qstr string

	if s.Endpoint != "" {
		q = append(q, "endpoint="+s.Endpoint)
	}
	if len(q) > 0 {
		qstr = "?" + strings.Join(q, "&")
	}

	return fmt.Sprintf("%s://%s/%s/%s%s", s3Scheme, s.Region, s.Bucket, key, qstr)
}

// Report report shipping.
func (s *S3) Report(ctx context.Context, req *ReportRequest) error {
	if req.Err != nil {
		return req.Err
	}

	now := time.Now().UTC().Format(ISO8601)
	hostname, _ := os.Hostname()
	info := fmt.Sprintf("shipped to %s at %s", strings.ToLower(hostname), now)
	filename := fmt.Sprintf("%s.txt", strings.Replace(info, " ", "_", -1))
	key := fmt.Sprintf("%s%s/%s", s.Prefix, req.Tag, filename)
	err := s.PutTextObject(ctx, key, "")

	return err
}

type S3Client interface {
	PutObject(ctx context.Context, input *s3.PutObjectInput, opts ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	ListObjectsV2(context.Context, *s3.ListObjectsV2Input, ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
}

func (s *S3) PutTextObject(ctx context.Context, key, content string) error {
	_, err := s.cl.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.Bucket),
		Key:         aws.String(key),
		Body:        strings.NewReader(content),
		ContentType: aws.String("text/plain"),
	})
	if err != nil {
		return fmt.Errorf("failed to upload text to S3: %w", err)
	}

	return nil
}

type ListObjectsV2Pager interface {
	HasMorePages() bool
	NextPage(context.Context, ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
}

func (s *S3) ListObjects(ctx context.Context, prefix string) ([]types.Object, error) {
	pager := s.pager
	if pager == nil {
		pager = s3.NewListObjectsV2Paginator(s.cl, &s3.ListObjectsV2Input{
			Bucket: aws.String(s.Bucket),
			Prefix: aws.String(prefix),
		})
	}

	var objects []types.Object

	for pager.HasMorePages() {
		output, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}
		objects = append(objects, output.Contents...)
	}

	return objects, nil
}

func (s *S3) extractFilenameFromObjectKey(key, prefix string) string {
	return strings.TrimPrefix(removeTrailingSlash(key), prefix)
}

// getVersionDirectoryCreatedAt gets the creation time of the first object in a version directory
func (s *S3) getVersionDirectoryCreatedAt(ctx context.Context, prefix string) (*time.Time, error) {
	pager := s3.NewListObjectsV2Paginator(s.cl, &s3.ListObjectsV2Input{
		Bucket:  aws.String(s.Bucket),
		Prefix:  aws.String(prefix),
		MaxKeys: aws.Int32(1), // We only need the first object
	})

	if pager.HasMorePages() {
		output, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list objects for version directory: %w", err)
		}
		if len(output.Contents) > 0 {
			return output.Contents[0].LastModified, nil
		}
	}

	return nil, fmt.Errorf("no objects found in version directory: %s", prefix)
}

func (s *S3) LatestVersion(ctx context.Context) (string, *SemVer, error) {
	pager := s.pager
	if pager == nil {
		pager = s3.NewListObjectsV2Paginator(s.cl, &s3.ListObjectsV2Input{
			Bucket:    aws.String(s.Bucket),
			Prefix:    aws.String(s.Prefix),
			Delimiter: aws.String("/"),
		})
	}

	// Collect all version directory names
	var versionNames []string
	var objectMap = make(map[string]*types.CommonPrefix)

	for pager.HasMorePages() {
		output, err := pager.NextPage(ctx)
		if err != nil {
			return "", nil, fmt.Errorf("failed to list objects: %w", err)
		}
		// Use output.CommonPrefixes instead of output.Contents to process only directories under prefix.
		for i, obj := range output.CommonPrefixes {
			name := s.extractFilenameFromObjectKey(*obj.Prefix, s.Prefix)
			versionNames = append(versionNames, name)
			objectMap[name] = &output.CommonPrefixes[i]
		}
	}

	// Use common latest version finding logic
	latestVersion, latestName, err := FindLatestSemVer(versionNames, s.PreRelease)
	if err != nil {
		return "", nil, err
	}

	latestObject := objectMap[latestName]
	return *latestObject.Prefix, latestVersion, nil
}

type customLogger struct {
	*logging.Logger
}

func (l *customLogger) Logf(classification awslogging.Classification, format string, v ...any) {
	if l.Format() == "json" {
		// For JSON format, use structured logging
		switch classification {
		case awslogging.Warn:
			l.Warn("aws", "message", fmt.Sprintf(format, v...))
		case awslogging.Debug:
			l.Debug("aws", "message", fmt.Sprintf(format, v...))
		default:
			l.Info("aws", "message", fmt.Sprintf(format, v...))
		}
	} else {
		// For text format, use simple message
		s := fmt.Sprintf(format, v...)
		switch classification {
		case awslogging.Warn:
			l.Warn(s)
		case awslogging.Debug:
			l.Debug(s)
		default:
			l.Info(s)
		}
	}
}

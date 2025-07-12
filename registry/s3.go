package registry

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
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
}

// NewS3 returns S3.
func NewS3(ctx context.Context, u string) (*S3, error) {
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

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(s.Region))
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
	found := false

	if s.Artifact != "" {
		artifactName = s.Artifact
		for _, v := range objects {
			name := s.extractFilenameFromObjectKey(*v.Key, prefix)
			if name == artifactName {
				found = true
				log.Printf("[DEBUG] Fetched: %+v", v)
				break
			}
		}

	} else {
		arch := getArch()
		os := getOS()
		archMatchs := []string{arch}
		if arch == "amd64" {
			archMatchs = append(archMatchs, "x86_64")
		}
		osMatchs := []string{os}
		if os == "darwin" {
			osMatchs = append(osMatchs, "macos")
		}

		for _, v := range objects {
			name := s.extractFilenameFromObjectKey(*v.Key, prefix)
			n := strings.ToLower(name)
			for _, arch := range archMatchs {
				if strings.Contains(n, arch) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
			found = false
			for _, os := range osMatchs {
				if strings.Contains(n, os) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
			artifactName = name
			log.Printf("[DEBUG] Fetched: %+v", v)
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("artifact not found: %s%s", prefix, artifactName)
	}

	return &CurrentResponse{
		ID:          time.Now().Format(ISO8601),
		Tag:         version.String(),
		ArtifactURL: s.buildArtifactURL(prefix + artifactName),
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

func (s *S3) LatestVersion(ctx context.Context) (string, *SemVer, error) {
	pager := s.pager
	if pager == nil {
		pager = s3.NewListObjectsV2Paginator(s.cl, &s3.ListObjectsV2Input{
			Bucket:    aws.String(s.Bucket),
			Prefix:    aws.String(s.Prefix),
			Delimiter: aws.String("/"),
		})
	}

	var latestObject *types.CommonPrefix
	var latestVersion *SemVer

	matched := func(str string, pre bool) bool {
		if pre {
			return SemVerRegex.MatchString(str)
		} else {
			return SemVerRegexWithoutPreRelease.MatchString(str)
		}
	}

	for pager.HasMorePages() {
		output, err := pager.NextPage(ctx)
		if err != nil {
			return "", nil, fmt.Errorf("failed to list objects: %w", err)
		}
		// Use output.CommonPrefixes instead of output.Contents to process only directories under prefix.
		for i, obj := range output.CommonPrefixes {
			name := s.extractFilenameFromObjectKey(*obj.Prefix, s.Prefix)
			if matched(name, s.PreRelease) {
				ver := ParseSemVer(name)
				if ver != nil {
					if latestVersion == nil || ver.Compare(latestVersion) > 0 {
						latestVersion = ver
						latestObject = &output.CommonPrefixes[i]
					}
				}
			}
		}
	}

	if latestObject == nil {
		return "", nil, fmt.Errorf("no valid versioned object found")
	}

	return *latestObject.Prefix, latestVersion, nil
}

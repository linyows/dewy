package registry

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

var (
	verRegex        = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)$`)
	verWithPreRegex = regexp.MustCompile(`^(v)?(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z.-]+))?$`)
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

	s := &S3{
		Bucket: ur.Host,
		Prefix: strings.TrimPrefix(addTrailingSlash(ur.Path), "/"),
	}
	if err = decoder.Decode(s, ur.Query()); err != nil {
		return nil, err
	}

	if s.Region == "" {
		s.Region = "ap-northeast-1"
	}
	if s.Bucket == "" {
		return nil, fmt.Errorf("s3 bucket is required: %s", "s3://<bucket>/<prefix>")
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
func (s *S3) Current(ctx context.Context, req *CurrentRequest) (*CurrentResponse, error) {
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

	if req.ArtifactName != "" {
		artifactName = req.ArtifactName
		for _, v := range objects {
			name := s.extractFilenameFromObjectKey(*v.Key, prefix)
			if name == artifactName {
				found = true
				log.Printf("[DEBUG] Fetched: %+v", v)
				break
			}
		}

	} else {
		archMatchs := []string{req.Arch}
		if req.Arch == "amd64" {
			archMatchs = append(archMatchs, "x86_64")
		}
		osMatchs := []string{req.OS}
		if req.OS == "darwin" {
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

	if s.Region != "" {
		q = append(q, "region="+s.Region)
	}
	if s.Endpoint != "" {
		q = append(q, "endpoint="+s.Endpoint)
	}
	if len(q) > 0 {
		qstr = "?" + strings.Join(q, "&")
	}

	return fmt.Sprintf("%s://%s/%s%s", s3Scheme, s.Bucket, key, qstr)
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
			return verWithPreRegex.MatchString(str)
		} else {
			return verRegex.MatchString(str)
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
				ver := parseSemVer(name)
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

type SemVer struct {
	V          string
	Major      int
	Minor      int
	Patch      int
	PreRelease string
}

func parseSemVer(version string) *SemVer {
	match := verWithPreRegex.FindStringSubmatch(version)
	if match == nil {
		return nil
	}

	v := match[1]
	major, _ := strconv.Atoi(match[2])
	minor, _ := strconv.Atoi(match[3])
	patch, _ := strconv.Atoi(match[4])
	preRelease := match[5]

	return &SemVer{
		V:          v,
		Major:      major,
		Minor:      minor,
		Patch:      patch,
		PreRelease: preRelease,
	}
}

func (v *SemVer) Compare(other *SemVer) int {
	if v.Major != other.Major {
		return v.Major - other.Major
	}
	if v.Minor != other.Minor {
		return v.Minor - other.Minor
	}
	if v.Patch != other.Patch {
		return v.Patch - other.Patch
	}
	if v.PreRelease == "" && other.PreRelease != "" {
		return 1
	}
	if v.PreRelease != "" && other.PreRelease == "" {
		return -1
	}
	return strings.Compare(v.PreRelease, other.PreRelease)
}

func (v *SemVer) String() string {
	var pre string
	if v.PreRelease != "" {
		pre = fmt.Sprintf("-%s", v.PreRelease)
	}
	return fmt.Sprintf("%s%d.%d.%d%s", v.V, v.Major, v.Minor, v.Patch, pre)
}

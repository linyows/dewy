package registry

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

const (
	gcsFormat string = "gcs://<project>/<bucket>/<prefix>"
)

// GCS struct.
type GCS struct {
	Project    string `schema:"project"`
	Bucket     string `schema:"-"`
	Prefix     string `schema:"-"`
	Artifact   string `schema:"artifact"`
	PreRelease bool   `schema:"pre-release"`
	client     GCSClient
}

// NewGCS returns GCS.
func NewGCS(ctx context.Context, u string) (*GCS, error) {
	return NewGCSWithClient(ctx, u, nil)
}

// NewGCSWithClient returns GCS with custom client (for testing).
func NewGCSWithClient(ctx context.Context, u string, client GCSClient) (*GCS, error) {
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

	g := &GCS{
		Project: ur.Host,
		Bucket:  bucket,
		Prefix:  prefix,
	}
	if err = decoder.Decode(g, ur.Query()); err != nil {
		return nil, err
	}

	if g.Project == "" {
		return nil, fmt.Errorf("project is required: %s", gcsFormat)
	}
	if g.Bucket == "" {
		return nil, fmt.Errorf("bucket is required: %s", gcsFormat)
	}

	if client == nil {
		client, err := storage.NewClient(ctx)
		if err != nil {
			return nil, err
		}
		g.client = client
	} else {
		g.client = client
	}

	return g, nil
}

// Current returns current artifact.
func (g *GCS) Current(ctx context.Context) (*CurrentResponse, error) {
	prefix, version, err := g.LatestVersion(ctx)
	if err != nil {
		return nil, err
	}

	objects, err := g.listObjects(ctx, prefix)
	if err != nil {
		return nil, err
	}

	var artifactName string
	var createdAt *time.Time
	found := false

	if g.Artifact != "" {
		artifactName = g.Artifact
		for _, obj := range objects {
			name := g.extractFilenameFromObjectName(obj.Name, prefix)
			if name == artifactName {
				found = true
				createdAt = &obj.Created
				log.Printf("[DEBUG] Fetched: %+v", obj)
				break
			}
		}
	} else {
		// Extract object names
		var objectNames []string
		var objectMap = make(map[string]*storage.ObjectAttrs)
		for _, obj := range objects {
			name := g.extractFilenameFromObjectName(obj.Name, prefix)
			objectNames = append(objectNames, name)
			objectMap[name] = obj
		}

		// Use common pattern matching
		matchedName, found := MatchArtifactByPlatform(objectNames)
		if found {
			artifactName = matchedName
			if obj, exists := objectMap[matchedName]; exists {
				createdAt = &obj.Created
				log.Printf("[DEBUG] Fetched: %+v", obj)
			}
		}
	}

	if !found {
		// Only get the creation time when artifact is not found
		artifactCreatedAt, _ := g.getVersionDirectoryCreatedAt(ctx, prefix)
		return nil, &ArtifactNotFoundError{
			ArtifactName: prefix + artifactName,
			ReleaseTime:  artifactCreatedAt,
			Message:      fmt.Sprintf("artifact not found: %s%s", prefix, artifactName),
		}
	}

	return &CurrentResponse{
		ID:          time.Now().Format(ISO8601),
		Tag:         version.String(),
		ArtifactURL: g.buildArtifactURL(prefix + artifactName),
		CreatedAt:   createdAt,
	}, nil
}

func (g *GCS) buildArtifactURL(name string) string {
	return fmt.Sprintf("%s://%s/%s/%s", gcsScheme, g.Project, g.Bucket, name)
}

// Report report shipping.
func (g *GCS) Report(ctx context.Context, req *ReportRequest) error {
	if req.Err != nil {
		return req.Err
	}

	now := time.Now().UTC().Format(ISO8601)
	hostname, _ := os.Hostname()
	info := fmt.Sprintf("shipped to %s at %s", strings.ToLower(hostname), now)
	filename := fmt.Sprintf("%s.txt", strings.Replace(info, " ", "_", -1))
	key := fmt.Sprintf("%s%s/%s", g.Prefix, req.Tag, filename)
	err := g.putTextObject(ctx, key, "")

	return err
}

type GCSClient interface {
	Bucket(name string) *storage.BucketHandle
	Close() error
}

func (g *GCS) putTextObject(ctx context.Context, name, content string) error {
	bucket := g.client.Bucket(g.Bucket)
	obj := bucket.Object(name)
	w := obj.NewWriter(ctx)
	w.ContentType = "text/plain"
	defer w.Close()

	if _, err := w.Write([]byte(content)); err != nil {
		return fmt.Errorf("failed to write text to GCS: %w", err)
	}

	return nil
}

func (g *GCS) listObjects(ctx context.Context, prefix string) ([]*storage.ObjectAttrs, error) {
	bucket := g.client.Bucket(g.Bucket)
	query := &storage.Query{
		Prefix: prefix,
	}

	var objects []*storage.ObjectAttrs
	it := bucket.Objects(ctx, query)
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}
		objects = append(objects, attrs)
	}

	return objects, nil
}

func (g *GCS) extractFilenameFromObjectName(name, prefix string) string {
	return strings.TrimPrefix(removeTrailingSlash(name), prefix)
}

// getVersionDirectoryCreatedAt gets the creation time of the first object in a version directory
func (g *GCS) getVersionDirectoryCreatedAt(ctx context.Context, prefix string) (*time.Time, error) {
	bucket := g.client.Bucket(g.Bucket)
	query := &storage.Query{
		Prefix: prefix,
	}

	it := bucket.Objects(ctx, query)
	attrs, err := it.Next()
	if err == iterator.Done {
		return nil, fmt.Errorf("no objects found in version directory: %s", prefix)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list objects for version directory: %w", err)
	}

	return &attrs.Created, nil
}

func (g *GCS) LatestVersion(ctx context.Context) (string, *SemVer, error) {
	bucket := g.client.Bucket(g.Bucket)
	query := &storage.Query{
		Prefix:    g.Prefix,
		Delimiter: "/",
	}

	var latestObjectName string
	var latestVersion *SemVer

	matched := func(str string, pre bool) bool {
		if pre {
			return SemVerRegex.MatchString(str)
		} else {
			return SemVerRegexWithoutPreRelease.MatchString(str)
		}
	}

	it := bucket.Objects(ctx, query)
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return "", nil, fmt.Errorf("failed to list objects: %w", err)
		}

		// Skip regular files, only process directories (prefixes)
		if attrs.Prefix == "" {
			continue
		}

		name := g.extractFilenameFromObjectName(attrs.Prefix, g.Prefix)
		if matched(name, g.PreRelease) {
			ver := ParseSemVer(name)
			if ver != nil {
				if latestVersion == nil || ver.Compare(latestVersion) > 0 {
					latestVersion = ver
					latestObjectName = attrs.Prefix
				}
			}
		}
	}

	if latestObjectName == "" {
		return "", nil, fmt.Errorf("no valid versioned object found")
	}

	return latestObjectName, latestVersion, nil
}

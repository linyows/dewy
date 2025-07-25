package registry

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/google/go-github/v73/github"
	"github.com/google/go-querystring/query"
	"github.com/linyows/dewy/client"
)

// ArtifactNotFoundError wraps artifact not found errors with release information
type ArtifactNotFoundError struct {
	ArtifactName string
	ReleaseTime  *time.Time
	Message      string
}

func (e *ArtifactNotFoundError) Error() string {
	return e.Message
}

// IsWithinGracePeriod checks if the error occurred within the grace period
func (e *ArtifactNotFoundError) IsWithinGracePeriod(gracePeriod time.Duration) bool {
	if e.ReleaseTime == nil || gracePeriod == 0 {
		return false
	}
	return time.Since(*e.ReleaseTime) < gracePeriod
}

const (
	// ISO8601 for time format.
	ISO8601 = "20060102T150405Z0700"
)

// GHR struct.
type GHR struct {
	Owner                 string `schema:"-"`
	Repo                  string `schema:"-"`
	Artifact              string `schema:"artifact"`
	PreRelease            bool   `schema:"pre-release"`
	DisableRecordShipping bool   // FIXME: For testing. Remove this.
	cl                    *github.Client
}

// New returns GHR.
func NewGHR(ctx context.Context, u string) (*GHR, error) {
	ur, err := url.Parse(u)
	if err != nil {
		return nil, err
	}

	ghr := &GHR{
		Owner: ur.Host,
		Repo:  strings.TrimPrefix(removeTrailingSlash(ur.Path), "/"),
	}

	if err := decoder.Decode(ghr, ur.Query()); err != nil {
		return nil, err
	}

	// Support GITHUB_ARTIFACT environment variable for backward compatibility
	if ghr.Artifact == "" {
		if artifact := os.Getenv("GITHUB_ARTIFACT"); artifact != "" {
			ghr.Artifact = artifact
		}
	}

	ghr.cl, err = client.NewGitHub()
	if err != nil {
		return nil, err
	}

	return ghr, nil
}

// String to string.
func (g *GHR) String() string {
	return g.host()
}

func (g *GHR) host() string {
	h := g.cl.BaseURL.Host
	if h != "api.github.com" {
		return h
	}
	return "github.com"
}

// Current returns current artifact.
func (g *GHR) Current(ctx context.Context) (*CurrentResponse, error) {
	release, err := g.latest(ctx)
	if err != nil {
		return nil, err
	}
	var artifactName string

	if g.Artifact != "" {
		artifactName = g.Artifact
		found := false
		for _, v := range release.Assets {
			if v.GetName() == artifactName {
				found = true
				log.Printf("[DEBUG] Fetched: %+v", v)
				break
			}
		}
		if !found {
			return nil, &ArtifactNotFoundError{
				ArtifactName: artifactName,
				ReleaseTime:  release.CreatedAt.GetTime(),
				Message:      fmt.Sprintf("artifact not found: %s", artifactName),
			}
		}
	} else {
		// Extract asset names
		var assetNames []string
		for _, v := range release.Assets {
			assetNames = append(assetNames, v.GetName())
		}
		
		// Use common pattern matching
		matchedName, found := MatchArtifactByPlatform(assetNames)
		if !found {
			return nil, &ArtifactNotFoundError{
				ArtifactName: artifactName,
				ReleaseTime:  release.CreatedAt.GetTime(),
				Message:      fmt.Sprintf("artifact not found: %s", artifactName),
			}
		}
		
		artifactName = matchedName
		log.Printf("[DEBUG] Fetched: %s", artifactName)
	}

	au := fmt.Sprintf("%s://%s/%s/tag/%s/%s", ghrScheme, g.Owner, g.Repo, release.GetTagName(), artifactName)

	return &CurrentResponse{
		ID:          time.Now().Format(ISO8601),
		Tag:         release.GetTagName(),
		ArtifactURL: au,
		CreatedAt:   release.CreatedAt.GetTime(),
	}, nil
}

func (g *GHR) latest(ctx context.Context) (*github.RepositoryRelease, error) {
	var r *github.RepositoryRelease
	if g.PreRelease {
		opt := &github.ListOptions{Page: 1}
		rr, _, err := g.cl.Repositories.ListReleases(ctx, g.Owner, g.Repo, opt)
		if err != nil {
			return nil, fmt.Errorf("failed github.Repositories.ListReleases: %w", err)
		}
		for _, v := range rr {
			if *v.Draft {
				continue
			}
			return r, nil
		}
	}
	r, _, err := g.cl.Repositories.GetLatestRelease(ctx, g.Owner, g.Repo)
	if err != nil {
		return nil, fmt.Errorf("failed github.Repositories.GetLatestRelease: %w", err)
	}
	return r, nil
}

// Report report shipping.
func (g *GHR) Report(ctx context.Context, req *ReportRequest) error {
	if req.Err != nil {
		return req.Err
	}
	now := time.Now().UTC().Format(ISO8601)
	hostname, _ := os.Hostname()
	info := fmt.Sprintf("shipped to %s at %s", strings.ToLower(hostname), now)

	page := 1
	for {
		releases, res, err := g.cl.Repositories.ListReleases(ctx, g.Owner, g.Repo, &github.ListOptions{
			Page:    page,
			PerPage: 100,
		})
		if err != nil {
			return err
		}
		for _, r := range releases {
			if r.GetTagName() == req.Tag {
				s := fmt.Sprintf("repos/%s/%s/releases/%d/assets", g.Owner, g.Repo, r.GetID())
				opt := &github.UploadOptions{Name: strings.Replace(info, " ", "_", -1) + ".txt"}

				u, err := url.Parse(s)
				if err != nil {
					return err
				}
				qs, err := query.Values(opt)
				if err != nil {
					return err
				}
				u.RawQuery = qs.Encode()
				b := []byte(info)
				r := bytes.NewReader(b)
				req, err := g.cl.NewUploadRequest(u.String(), r, int64(len(b)), "text/plain")
				if err != nil {
					return err
				}

				asset := new(github.ReleaseAsset)
				if _, err := g.cl.Do(ctx, req, asset); err != nil {
					return err
				}
				return nil
			}
		}
		if res.NextPage == 0 {
			break
		}
		page = res.NextPage
	}

	return fmt.Errorf("release not found: %s", req.Tag)
}

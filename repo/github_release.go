package repo

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/google/go-github/v55/github"
	"github.com/google/go-querystring/query"
	"github.com/k1LoW/go-github-client/v55/factory"
	"github.com/linyows/dewy/registory"
)

const (
	GitHubReleaseScheme = "github_release"
	// ISO8601 for time format.
	ISO8601 = "20060102T150405Z0700"
)

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}

var _ Repo = (*GithubRelease)(nil)

// GithubRelease struct.
type GithubRelease struct {
	baseURL               string
	uploadURL             string
	owner                 string
	repo                  string
	downloadURL           string
	prerelease            bool
	disableRecordShipping bool // FIXME: For testing. Remove this.
	cl                    *github.Client
	updatedAt             github.Timestamp
}

// NewGithubRelease returns GithubRelease.
func NewGithubRelease(c Config) (*GithubRelease, error) {
	cl, err := factory.NewGithubClient()
	if err != nil {
		return nil, err
	}
	g := &GithubRelease{
		owner:                 c.Owner,
		repo:                  c.Repo,
		prerelease:            c.PreRelease,
		disableRecordShipping: c.DisableRecordShipping,
		cl:                    cl,
	}
	_, v3ep, v3upload, _ := factory.GetTokenAndEndpoints()
	g.baseURL = v3ep
	g.uploadURL = v3upload
	return g, nil
}

// String to string.
func (g *GithubRelease) String() string {
	return g.host()
}

func (g *GithubRelease) host() string {
	h := g.cl.BaseURL.Host
	if h != "api.github.com" {
		return h
	}
	return "github.com"
}

// OwnerURL returns owner URL.
func (g *GithubRelease) OwnerURL() string {
	return fmt.Sprintf("https://%s/%s", g, g.owner)
}

// OwnerIconURL returns owner icon URL.
func (g *GithubRelease) OwnerIconURL() string {
	return fmt.Sprintf("%s.png?size=200", g.OwnerURL())
}

// URL returns repository URL.
func (g *GithubRelease) URL() string {
	return fmt.Sprintf("%s/%s", g.OwnerURL(), g.repo)
}

// Current returns current artifact.
func (g *GithubRelease) Current(req *registory.CurrentRequest) (*registory.CurrentResponse, error) {
	release, err := g.latest()
	if err != nil {
		return nil, err
	}

	found := false
	for _, v := range release.Assets {
		if v.GetName() == req.ArtifactName {
			found = true
			log.Printf("[DEBUG] Fetched: %+v", v)
			g.downloadURL = v.GetBrowserDownloadURL()
			g.updatedAt = v.GetUpdatedAt()
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("artifact not found: %s", req.ArtifactName)
	}

	au := fmt.Sprintf("github_release://%s/%s/tag/%s/%s", g.owner, g.repo, release.GetTagName(), req.ArtifactName)

	return &registory.CurrentResponse{
		ID:          time.Now().Format(ISO8601),
		Tag:         release.GetTagName(),
		ArtifactURL: au,
	}, nil
}

func (g *GithubRelease) latest() (*github.RepositoryRelease, error) {
	ctx := context.Background()
	var r *github.RepositoryRelease
	if g.prerelease {
		opt := &github.ListOptions{Page: 1}
		rr, _, err := g.cl.Repositories.ListReleases(ctx, g.owner, g.repo, opt)
		if err != nil {
			return nil, err
		}
		for _, v := range rr {
			if *v.Draft {
				continue
			}
			return r, nil
		}
	}
	r, _, err := g.cl.Repositories.GetLatestRelease(ctx, g.owner, g.repo)
	if err != nil {
		return nil, err
	}
	return r, nil
}

// Fetch fetch artifact.
func (g *GithubRelease) Fetch(url string, w io.Writer) error {
	ctx := context.Background()
	// github_release://owner/repo/tag/v1.0.0/artifact.zip
	// github_release://owner/repo/latest/artifact.zip
	splitted := strings.Split(strings.TrimPrefix(url, fmt.Sprintf("%s://", GitHubReleaseScheme)), "/")
	if len(splitted) != 4 && len(splitted) != 5 {
		return fmt.Errorf("invalid url: %s", url)
	}
	owner := splitted[0]
	name := splitted[1]
	if len(splitted) == 4 {
		// latest
		// FIXME: not implemented
		return fmt.Errorf("not implemented")
	}
	tag := splitted[3]
	artifactName := splitted[4]
	page := 1
	var assetID int64
L:
	for {
		releases, res, err := g.cl.Repositories.ListReleases(ctx, g.owner, g.repo, &github.ListOptions{
			Page:    page,
			PerPage: 100,
		})
		if err != nil {
			return err
		}
		for _, r := range releases {
			if r.GetTagName() != tag {
				continue
			}
			for _, a := range r.Assets {
				if a.GetName() != artifactName {
					continue
				}
				assetID = a.GetID()
				break L
			}
		}
		if res.NextPage == 0 {
			break
		}
		page = res.NextPage
	}

	reader, url, err := g.cl.Repositories.DownloadReleaseAsset(ctx, owner, name, assetID, httpClient)
	if err != nil {
		return err
	}
	if url != "" {
		res, err := httpClient.Get(url)
		if err != nil {
			return err
		}
		reader = res.Body
	}

	log.Printf("[INFO] Downloaded from %s", g.downloadURL)
	_, err = io.Copy(w, reader)
	if err != nil {
		return err
	}

	return nil
}

// Report report shipping.
func (g *GithubRelease) Report(req *registory.ReportRequest) error {
	if g.disableRecordShipping {
		return nil
	}
	if req.Err != nil {
		return req.Err
	}
	ctx := context.Background()
	now := time.Now().UTC().Format(ISO8601)
	hostname, _ := os.Hostname()
	info := fmt.Sprintf("shipped to %s at %s", strings.ToLower(hostname), now)

	page := 1
	for {
		releases, res, err := g.cl.Repositories.ListReleases(ctx, g.owner, g.repo, &github.ListOptions{
			Page:    page,
			PerPage: 100,
		})
		if err != nil {
			return err
		}
		for _, r := range releases {
			if r.GetTagName() == req.Tag {
				s := fmt.Sprintf("repos/%s/%s/releases/%d/assets", g.owner, g.repo, r.GetID())
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

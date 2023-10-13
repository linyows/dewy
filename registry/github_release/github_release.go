package ghrelease

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/google/go-github/v55/github"
	"github.com/google/go-querystring/query"
	"github.com/k1LoW/go-github-client/v55/factory"
	"github.com/linyows/dewy/registry"
	ghrelease "github.com/linyows/dewy/storage/github_release"
)

const (
	// ISO8601 for time format.
	ISO8601 = "20060102T150405Z0700"
	Scheme  = "github_release"
)

// GithubRelease struct.
type GithubRelease struct {
	owner      string
	repo       string
	prerelease bool
	cl         *github.Client
}

var _ registry.Registry = (*GithubRelease)(nil)

// New returns GithubRelease.
func New(c Config) (*GithubRelease, error) {
	cl, err := factory.NewGithubClient()
	if err != nil {
		return nil, err
	}
	g := &GithubRelease{
		owner:      c.Owner,
		repo:       c.Repo,
		prerelease: c.PreRelease,
		cl:         cl,
	}
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

// Owner returns owner.
func (g *GithubRelease) Owner() string {
	return g.owner
}

// Repo returns repository.
func (g *GithubRelease) Repo() string {
	return g.repo
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
func (g *GithubRelease) Current(ctx context.Context, req *registry.CurrentRequest) (*registry.CurrentResponse, error) {
	release, err := g.latest(ctx)
	if err != nil {
		return nil, err
	}
	var artifactName string

	if req.ArtifactName != "" {
		artifactName = req.ArtifactName
		found := false
		for _, v := range release.Assets {
			if v.GetName() == artifactName {
				found = true
				log.Printf("[DEBUG] Fetched: %+v", v)
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("artifact not found: %s", artifactName)
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
		found := false
		for _, v := range release.Assets {
			n := strings.ToLower(v.GetName())
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
			artifactName = v.GetName()
			log.Printf("[DEBUG] Fetched: %+v", v)
			break
		}
		if !found {
			return nil, fmt.Errorf("artifact not found: %s", artifactName)
		}
	}

	au := fmt.Sprintf("%s://%s/%s/tag/%s/%s", ghrelease.Scheme, g.owner, g.repo, release.GetTagName(), artifactName)

	return &registry.CurrentResponse{
		ID:          time.Now().Format(ISO8601),
		Tag:         release.GetTagName(),
		ArtifactURL: au,
	}, nil
}

func (g *GithubRelease) latest(ctx context.Context) (*github.RepositoryRelease, error) {
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

// Report report shipping.
func (g *GithubRelease) Report(ctx context.Context, req *registry.ReportRequest) error {
	if req.Err != nil {
		return req.Err
	}
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

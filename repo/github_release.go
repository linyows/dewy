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
)

const (
	// ISO8601 for time format
	ISO8601 = "20060102T150405Z0700"
)

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}

// GithubRelease struct
type GithubRelease struct {
	baseURL               string
	uploadURL             string
	owner                 string
	name                  string
	artifact              string
	downloadURL           string
	releaseID             int64
	assetID               int64
	releaseURL            string
	releaseTag            string
	prerelease            bool
	disableRecordShipping bool // FIXME: For testing. Remove this.
	cl                    *github.Client
	updatedAt             github.Timestamp
}

// NewGithubRelease returns GithubRelease
func NewGithubRelease(c Config) (*GithubRelease, error) {
	cl, err := factory.NewGithubClient()
	if err != nil {
		return nil, err
	}
	g := &GithubRelease{
		owner:                 c.Owner,
		name:                  c.Name,
		artifact:              c.Artifact,
		prerelease:            c.PreRelease,
		disableRecordShipping: c.DisableRecordShipping,
		cl:                    cl,
	}
	_, v3ep, v3upload, _ := factory.GetTokenAndEndpoints()
	g.baseURL = v3ep
	g.uploadURL = v3upload
	return g, nil
}

// String to string
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

// OwnerURL returns owner URL
func (g *GithubRelease) OwnerURL() string {
	return fmt.Sprintf("https://%s/%s", g, g.owner)
}

// OwnerIconURL returns owner icon URL
func (g *GithubRelease) OwnerIconURL() string {
	return fmt.Sprintf("%s.png?size=200", g.OwnerURL())
}

// URL returns repository URL
func (g *GithubRelease) URL() string {
	return fmt.Sprintf("%s/%s", g.OwnerURL(), g.name)
}

// ReleaseTag returns tag
func (g *GithubRelease) ReleaseTag() string {
	return g.releaseTag
}

// ReleaseURL returns release URL
func (g *GithubRelease) ReleaseURL() string {
	return g.releaseURL
}

func (g *GithubRelease) Current(req *CurrentRequest) (*CurrentResponse, error) {
	release, err := g.latest()
	if err != nil {
		return nil, err
	}

	g.releaseID = *release.ID
	g.releaseURL = *release.HTMLURL

	found := false
	for _, v := range release.Assets {
		if v.GetName() == req.ArtifactName {
			found = true
			log.Printf("[DEBUG] Fetched: %+v", v)
			g.downloadURL = v.GetBrowserDownloadURL()
			g.releaseTag = release.GetTagName()
			g.assetID = v.GetID()
			g.updatedAt = v.GetUpdatedAt()
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("artifact not found: %s", req.ArtifactName)
	}

	au := fmt.Sprintf("github_release://%s/%s/tag/%s/%s", g.owner, g.name, release.GetTagName(), req.ArtifactName)

	return &CurrentResponse{
		Tag:         release.GetTagName(),
		ArtifactURL: au,
	}, nil
}

func (g *GithubRelease) latest() (*github.RepositoryRelease, error) {
	ctx := context.Background()
	var r *github.RepositoryRelease
	if g.prerelease {
		opt := &github.ListOptions{Page: 1}
		rr, _, err := g.cl.Repositories.ListReleases(ctx, g.owner, g.name, opt)
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
	r, _, err := g.cl.Repositories.GetLatestRelease(ctx, g.owner, g.name)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (g *GithubRelease) Download(w io.Writer) error {
	ctx := context.Background()
	reader, url, err := g.cl.Repositories.DownloadReleaseAsset(ctx, g.owner, g.name, g.assetID, httpClient)
	if err != nil {
		return err
	}
	if url != "" {
		res, err := http.Get(url)
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

// RecordShipping save shipping to github
func (g *GithubRelease) RecordShipping() error {
	if g.disableRecordShipping {
		return nil
	}
	ctx := context.Background()
	now := time.Now().UTC().Format(ISO8601)
	hostname, _ := os.Hostname()
	info := fmt.Sprintf("shipped to %s at %s", strings.ToLower(hostname), now)

	s := fmt.Sprintf("repos/%s/%s/releases/%d/assets", g.owner, g.name, g.releaseID)
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

	byteData := []byte(info)
	r := bytes.NewReader(byteData)
	req, err := g.cl.NewUploadRequest(u.String(), r, int64(len(byteData)), "text/plain")
	if err != nil {
		return err
	}

	asset := new(github.ReleaseAsset)
	_, err = g.cl.Do(ctx, req, asset)

	return err
}

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

	"github.com/google/go-github/github"
	"github.com/google/go-querystring/query"
	"github.com/linyows/dewy/kvs"
	"golang.org/x/oauth2"
)

const (
	// ISO8601 for time format
	ISO8601 = "20060102T150405Z0700"
)

// GithubRelease struct
type GithubRelease struct {
	token       string
	baseURL     string
	uploadURL   string
	owner       string
	name        string
	artifact    string
	downloadURL string
	cacheKey    string
	cache       kvs.KVS
	releaseID   int64
	assetID     int64
	releaseURL  string
	releaseTag  string
	prerelease  bool
	cl          *github.Client
	updatedAt   github.Timestamp
}

// NewGithubRelease returns GithubRelease
func NewGithubRelease(c Config, d kvs.KVS) *GithubRelease {
	g := &GithubRelease{
		token:      c.Token,
		owner:      c.Owner,
		name:       c.Name,
		artifact:   c.Artifact,
		cache:      d,
		prerelease: c.PreRelease,
	}
	if c.Endpoint != "" {
		if !strings.HasSuffix(c.Endpoint, "/") {
			c.Endpoint += "/"
		}
		g.baseURL = c.Endpoint
		g.uploadURL = c.Endpoint + "../uploads/"
	}
	return g
}

// String to string
func (g *GithubRelease) String() string {
	return g.host()
}

func (g *GithubRelease) host() string {
	ctx := context.Background()
	c, err := g.client(ctx)
	if err != nil {
		return err.Error()
	}
	h := c.BaseURL.Host
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

// Fetch to latest github release
func (g *GithubRelease) Fetch() error {
	release, err := g.latest()
	if err != nil {
		return err
	}

	g.releaseID = *release.ID
	g.releaseURL = *release.HTMLURL

	for _, v := range release.Assets {
		if *v.Name == g.artifact {
			log.Printf("[DEBUG] Fetched: %+v", v)
			g.downloadURL = *v.BrowserDownloadURL
			g.releaseTag = *release.TagName
			g.assetID = *v.ID
			g.updatedAt = *v.UpdatedAt
			break
		}
	}

	if err := g.setCacheKey(); err != nil {
		return err
	}

	return nil
}

func (g *GithubRelease) latest() (*github.RepositoryRelease, error) {
	ctx := context.Background()
	c, err := g.client(ctx)
	if err != nil {
		return nil, err
	}

	var r *github.RepositoryRelease
	if g.prerelease {
		opt := &github.ListOptions{Page: 1}
		rr, _, err := c.Repositories.ListReleases(ctx, g.owner, g.name, opt)
		if err != nil {
			return nil, err
		}
		r = rr[0]
	} else {
		r, _, err = c.Repositories.GetLatestRelease(ctx, g.owner, g.name)
	}
	if err != nil {
		return nil, err
	}

	return r, nil
}

func (g *GithubRelease) setCacheKey() error {
	u, err := url.Parse(g.downloadURL)
	if err != nil {
		return err
	}
	g.cacheKey = strings.Replace(fmt.Sprintf("%s--%d-%s", u.Host, g.updatedAt.Unix(), u.RequestURI()), "/", "-", -1)

	return nil
}

// GetDeploySourceKey returns cache key
func (g *GithubRelease) GetDeploySourceKey() (string, error) {
	currentKey := "current.txt"
	currentSourceKey, _ := g.cache.Read(currentKey)
	found := false

	list, err := g.cache.List()
	if err != nil {
		return "", err
	}

	for _, key := range list {
		if string(currentSourceKey) == g.cacheKey && key == g.cacheKey {
			return "", fmt.Errorf("No need to deploy")
		}

		if key == g.cacheKey {
			found = true
			break
		}
	}

	if !found {
		if err := g.download(); err != nil {
			return "", err
		}
	}

	if err := g.cache.Write(currentKey, []byte(g.cacheKey)); err != nil {
		return "", err
	}

	return g.cacheKey, nil
}

func (g *GithubRelease) download() error {
	ctx := context.Background()
	c, err := g.client(ctx)
	if err != nil {
		return err
	}

	reader, url, err := c.Repositories.DownloadReleaseAsset(ctx, g.owner, g.name, g.assetID)
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
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, reader)
	if err != nil {
		return err
	}

	if err := g.cache.Write(g.cacheKey, buf.Bytes()); err != nil {
		return err
	}
	log.Printf("[INFO] Cached as %s", g.cacheKey)

	return nil
}

func (g *GithubRelease) client(ctx context.Context) (*github.Client, error) {
	if g.cl != nil {
		return g.cl, nil
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: g.token},
	)
	tc := oauth2.NewClient(ctx, ts)

	if g.baseURL == "" {
		g.cl = github.NewClient(tc)
	} else {
		var err error
		g.cl, err = github.NewEnterpriseClient(g.baseURL, g.uploadURL, tc)
		if err != nil {
			return nil, err
		}
	}

	return g.cl, nil
}

// RecordShipping save shipping to github
func (g *GithubRelease) RecordShipping() error {
	ctx := context.Background()
	c, err := g.client(ctx)
	if err != nil {
		return err
	}

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
	req, err := c.NewUploadRequest(u.String(), r, int64(len(byteData)), "text/plain")
	if err != nil {
		return err
	}

	asset := new(github.ReleaseAsset)
	_, err = c.Do(ctx, req, asset)

	return err
}

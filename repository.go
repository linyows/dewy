package dewy

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

type Repository interface {
	Fetch() error
	Download() (string, error)
	IsDownloadNecessary() bool
	Record() error
}

type GithubReleaseRepository struct {
	token       string
	endpoint    string
	owner       string
	name        string
	artifact    string
	tag         string
	downloadURL string
	cacheKey    string
	cache       kvs.KVS
	releaseID   int64
	cl          *github.Client
}

func NewRepository(c RepositoryConfig, d kvs.KVS) Repository {
	switch c.Provider {
	case GITHUB:
		return &GithubReleaseRepository{
			token:    c.Token,
			endpoint: c.Endpoint,
			owner:    c.Owner,
			name:     c.Name,
			artifact: c.Artifact,
			cache:    d,
		}
	default:
		panic("no repository provider")
	}
}

func (g *GithubReleaseRepository) Fetch() error {
	ctx := context.Background()
	c, err := g.client(ctx)
	if err != nil {
		return err
	}
	release, _, err := c.Repositories.GetLatestRelease(ctx, g.owner, g.name)

	if err != nil {
		return err
	}
	g.releaseID = *release.ID

	for _, v := range release.Assets {
		if *v.Name == g.artifact {
			log.Printf("[DEBUG] Fetched: %+v", v)
			g.downloadURL = *v.BrowserDownloadURL
			g.tag = *release.TagName
			break
		}
	}

	if err := g.setCacheKey(); err != nil {
		return err
	}

	return nil
}

func (g *GithubReleaseRepository) setCacheKey() error {
	u, err := url.Parse(g.downloadURL)
	if err != nil {
		return err
	}
	g.cacheKey = strings.Replace(fmt.Sprintf("%s%s", u.Host, u.RequestURI()), "/", "-", -1)

	return nil
}

func (g *GithubReleaseRepository) IsDownloadNecessary() bool {
	list, err := g.cache.List()
	if err != nil {
		return false
	}

	for _, key := range list {
		if key == g.cacheKey {
			return false
		}
	}

	return true
}

func (g *GithubReleaseRepository) Download() (string, error) {
	res, err := http.Get(g.downloadURL)
	if err != nil {
		return "", err
	}
	log.Printf("[INFO] Downloaded from %s", g.downloadURL)

	buf := new(bytes.Buffer)
	io.Copy(buf, res.Body)
	body := buf.Bytes()

	if err := g.cache.Write(g.cacheKey, body); err != nil {
		return "", err
	}
	log.Printf("[INFO] Cached as %s", g.cacheKey)

	return g.cacheKey, nil
}

func (g *GithubReleaseRepository) client(ctx context.Context) (*github.Client, error) {
	if g.cl != nil {
		return g.cl, nil
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: g.token},
	)
	tc := oauth2.NewClient(ctx, ts)
	g.cl = github.NewClient(tc)

	if g.endpoint != "" {
		url, err := url.Parse(g.endpoint)
		if err != nil {
			return nil, err
		}
		g.cl.BaseURL = url
	}

	return g.cl, nil
}

func (g *GithubReleaseRepository) Record() error {
	ctx := context.Background()
	c, err := g.client(ctx)
	if err != nil {
		return err
	}

	ISO8601 := "20060102T150405Z0700"
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

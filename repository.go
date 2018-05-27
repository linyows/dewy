package dewy

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

type Repository interface {
	Fetch() error
	Download() error
}

type GithubReleaseRepository struct {
	token       string
	endpoint    string
	owner       string
	name        string
	artifact    string
	tag         string
	downloadURL string
}

func NewRepository(c RepositoryConfig) Repository {
	switch c.Provider {
	case GITHUB:
		return &GithubReleaseRepository{
			token:    c.Token,
			endpoint: c.Endpoint,
			owner:    c.Owner,
			name:     c.Name,
			artifact: c.Artifact,
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
	for _, v := range release.Assets {
		fmt.Printf("%s -- Size: %d, Download: %d <%s>\n", *v.Name, *v.Size, *v.DownloadCount, *v.BrowserDownloadURL)
		if *v.Name == g.artifact {
			g.downloadURL = *v.BrowserDownloadURL
			g.tag = *release.TagName
			break
		}
	}
	return nil
}

func (g *GithubReleaseRepository) Download() error {
	res, err := http.Get(g.downloadURL)
	if err != nil {
		return err
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	dir := ""

	_, filename := path.Split(g.downloadURL)
	filePath := filepath.Join(dir, filename)
	fmt.Printf("Download to %s\n", filePath)

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	file.Write(body)

	binPath, err := g.unzip(filePath, dir)
	if err != nil {
		return err
	}
	fmt.Printf("Unzip to %s\n", binPath)

	if err := os.Rename(binPath, binPath+"."+g.tag); err != nil {
		return err
	}

	return nil
}

func (g *GithubReleaseRepository) client(ctx context.Context) (*github.Client, error) {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: g.token},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	if g.endpoint != "" {
		url, err := url.Parse(g.endpoint)
		if err != nil {
			return nil, err
		}
		client.BaseURL = url
	}

	return client, nil
}

func (g *GithubReleaseRepository) unzip(src, dstDir string) (string, error) {
	r, err := zip.OpenReader(src)
	if err != nil {
		return "", err
	}
	defer r.Close()
	var dst string

	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		defer rc.Close()

		if f.FileInfo().IsDir() {
			dst = filepath.Join(dstDir, f.Name)
			os.MkdirAll(dst, f.Mode())
		} else {
			buf := make([]byte, f.UncompressedSize)
			_, err = io.ReadFull(rc, buf)
			if err != nil {
				return "", err
			}

			dst = filepath.Join(dstDir, f.Name)
			if err = ioutil.WriteFile(dst, buf, f.Mode()); err != nil {
				return "", err
			}
		}
	}

	return dst, nil
}

func isExists(f string) bool {
	_, err := os.Stat(f)
	return err == nil
}

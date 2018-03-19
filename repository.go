package dewy

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

type Repository struct {
	provider    RepositoryProvider
	token       string
	endpoint    string
	owner       string
	name        string
	artifact    string
	downloadURL string
}

func NewRepository(c RepositoryConfig) *Repository {
	return &Repository{
		provider: c.Provider,
		token:    c.Token,
		endpoint: c.Endpoint,
		owner:    c.Owner,
		name:     c.Name,
		artifact: c.Artifact,
	}
}

func (r *Repository) Fetch() error {
	ctx := context.Background()
	c, err := r.client(ctx)
	if err != nil {
		return err
	}
	release, _, err := c.Repositories.GetLatestRelease(ctx, r.owner, r.name)
	if err != nil {
		return err
	}
	for _, v := range release.Assets {
		if *v.Name == r.artifact {
			r.downloadURL = *v.BrowserDownloadURL
			break
		}
		//fmt.Printf("%s -- Size: %d, Download: %d <%s>\n", *v.Name, *v.Size, *v.DownloadCount, *v.BrowserDownloadURL)
	}
	return nil
}

func (r *Repository) Download() error {
	res, err := http.Get(r.downloadURL)
	if err != nil {
		return err
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	_, filename := path.Split(r.downloadURL)
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	file.Write(body)

	return nil
}

func (r *Repository) client(ctx context.Context) (*github.Client, error) {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: r.token},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	if r.endpoint != "" {
		url, err := url.Parse(r.endpoint)
		if err != nil {
			return nil, err
		}
		client.BaseURL = url
	}

	return client, nil
}

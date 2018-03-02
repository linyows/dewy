package dewy

import (
	"context"
	"fmt"
	"net/url"

	"github.com/google/go-github/github"
	"github.com/lestrrat-go/server-starter"
	"golang.org/x/oauth2"
)

type Repository struct {
	provider RepositoryProvider
	token    string
	endpoint string
	owner    string
	name     string
}

func NewRepository(c RepositoryConfig) *Repository {
	return &Repository{
		provider: c.Provider,
		token:    c.Token,
		endpoint: c.Endpoint,
		owner:    c.Owner,
		name:     c.Name,
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
	fmt.Printf("Assets: %d\n", len(release.Assets))
	for _, v := range release.Assets {
		fmt.Printf("%s -- Size: %d, Download: %d <%s>\n",
			*v.Name, *v.Size, *v.DownloadCount, *v.BrowserDownloadURL)
	}
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

type Cache struct {
}

type Dewy struct {
	config     Config
	repository Repository
	cache      Cache
	starter    starter.Config
}

func New(c Config) *Dewy {
	return &Dewy{
		config: c,
	}
}

func (d *Dewy) Run() error {
	r := NewRepository(d.config.Repository)
	if err := r.Fetch(); err != nil {
		return err
	}
	return nil
}

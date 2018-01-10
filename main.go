package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

func main() {
	//go polling()
	Release()
}

func polling() {
	t := time.NewTicker(3 * time.Second)
	for {
		select {
		case <-t.C:
			Release()
		}
	}
	t.Stop()
}

func Client(ctx context.Context, token string, endpoint string) *github.Client {
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	if endpoint != "" {
		url, err := url.Parse(endpoint)
		if err != nil {
			panic(err)
		}
		client.BaseURL = url
	}

	return client
}

func Release() {
	ctx := context.Background()
	owner := "linyows"
	repo := "octopass"
	c := Client(ctx, "", "")
	release, _, err := c.Repositories.GetLatestRelease(ctx, owner, repo)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%#v\n", release)
}

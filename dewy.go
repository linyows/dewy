//package dewy
package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"runtime"

	"github.com/carlescere/scheduler"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

func main() {
	job := release()
	scheduler.Every(10).Seconds().NotImmediately().Run(job)
	runtime.Goexit()
}

func client(ctx context.Context, token string, endpoint string) *github.Client {
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

func release() func() {
	return func() {
		ctx := context.Background()
		owner := "linyows"
		repo := "octopass"
		c := client(ctx, "", "")
		release, _, err := c.Repositories.GetLatestRelease(ctx, owner, repo)
		if err != nil {
			panic(err)
		}
		fmt.Printf("Assets: %d\n", len(release.Assets))
		for _, v := range release.Assets {
			fmt.Printf("%s -- Size: %d, Download: %d <%s>\n",
				*v.Name, *v.Size, *v.DownloadCount, *v.BrowserDownloadURL)
		}
	}
}

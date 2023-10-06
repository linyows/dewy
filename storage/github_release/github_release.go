package ghrelease

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/google/go-github/v55/github"
	"github.com/k1LoW/go-github-client/v55/factory"
)

const Scheme = "github_release"

type GithubRelease struct {
	cl *github.Client
}

func New() (*GithubRelease, error) {
	cl, err := factory.NewGithubClient()
	if err != nil {
		return nil, err
	}
	return &GithubRelease{
		cl: cl,
	}, nil
}

// Fetch fetch artifact.
func (r *GithubRelease) Fetch(urlstr string, w io.Writer) error {
	ctx := context.Background()
	// github_release://owner/repo/tag/v1.0.0/artifact.zip
	// github_release://owner/repo/latest/artifact.zip
	splitted := strings.Split(strings.TrimPrefix(urlstr, fmt.Sprintf("%s://", Scheme)), "/")
	if len(splitted) != 4 && len(splitted) != 5 {
		return fmt.Errorf("invalid url: %s", urlstr)
	}
	owner := splitted[0]
	repo := splitted[1]
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
		releases, res, err := r.cl.Repositories.ListReleases(ctx, owner, repo, &github.ListOptions{
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

	reader, url, err := r.cl.Repositories.DownloadReleaseAsset(ctx, owner, repo, assetID, r.cl.Client())
	if err != nil {
		return err
	}
	if url != "" {
		res, err := r.cl.Client().Get(url)
		if err != nil {
			return err
		}
		defer res.Body.Close()
		reader = res.Body
	}

	log.Printf("[INFO] Downloaded from %s", urlstr)
	if _, err := io.Copy(w, reader); err != nil {
		return err
	}

	return nil
}

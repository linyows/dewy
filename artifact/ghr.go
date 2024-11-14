package artifact

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/google/go-github/v55/github"
	"github.com/k1LoW/go-github-client/v55/factory"
)

type GHR struct {
	owner    string
	repo     string
	tag      string
	artifact string
	url      string
	cl       *github.Client
}

func NewGHR(ctx context.Context, url string) (*GHR, error) {
	// ghr://owner/repo/tag/v1.0.0/artifact.zip
	// ghr://owner/repo/tag/latest/artifact.zip
	splitted := strings.Split(strings.TrimPrefix(url, fmt.Sprintf("%s://", ghrScheme)), "/")
	if len(splitted) != 4 && len(splitted) != 5 {
		return nil, fmt.Errorf("invalid artifact url: %s, %#v", url, splitted)
	}

	cl, err := factory.NewGithubClient()
	if err != nil {
		return nil, err
	}

	return &GHR{
		owner:    splitted[0],
		repo:     splitted[1],
		tag:      splitted[3],
		artifact: splitted[4],
		url:      url,
		cl:       cl,
	}, nil
}

// Download download artifact.
func (r *GHR) Download(ctx context.Context, w io.Writer) error {
	page := 1
	var assetID int64
L:
	for {
		releases, res, err := r.cl.Repositories.ListReleases(ctx, r.owner, r.repo, &github.ListOptions{
			Page:    page,
			PerPage: 100,
		})
		if err != nil {
			return err
		}
		for _, v := range releases {
			if v.GetTagName() != r.tag {
				continue
			}
			for _, a := range v.Assets {
				if a.GetName() != r.artifact {
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

	reader, url, err := r.cl.Repositories.DownloadReleaseAsset(ctx, r.owner, r.repo, assetID, r.cl.Client())
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

	log.Printf("[INFO] Downloaded from %s", url)
	if _, err := io.Copy(w, reader); err != nil {
		return err
	}

	return nil
}

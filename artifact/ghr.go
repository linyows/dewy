package artifact

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v73/github"
	"github.com/linyows/dewy/client"
)

var (
	firstDownloadOnce sync.Once
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
	splitted := strings.Split(strings.TrimPrefix(url, fmt.Sprintf("%s://", ghrScheme)), "/")
	if len(splitted) != 5 {
		return nil, fmt.Errorf("invalid artifact url: %s, %#v", url, splitted)
	}

	cl, err := client.NewGitHub()
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
	// Wait 1 second on first download to allow CDN authentication to stabilize
	firstDownloadOnce.Do(func() {
		log.Printf("[INFO] First download attempt, waiting 1 second for CDN auth stabilization...")
		time.Sleep(1 * time.Second)
	})

	page := 1
	var assetID int64
L:
	for {
		releases, res, err := r.cl.Repositories.ListReleases(ctx, r.owner, r.repo, &github.ListOptions{
			Page:    page,
			PerPage: 100,
		})
		if err != nil {
			return fmt.Errorf("failed github.Repositories.ListReleases: %w", err)
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

	// Note:
	// This method downloads assets with application/octet-stream of accept header.
	// Do download by browser_download_url when returns json.
	reader, redirectURL, err := r.cl.Repositories.DownloadReleaseAsset(ctx, r.owner, r.repo, assetID, r.cl.Client())
	if err != nil {
		return fmt.Errorf("failed github.Repositories.DownloadReleaseAsset: %w", err)
	}
	if redirectURL != "" {
		log.Printf("[INFO] Following redirect to: %s", redirectURL)
		res, err := r.cl.Client().Get(redirectURL)
		if err != nil {
			return err
		}
		reader = res.Body
	}

	defer reader.Close()
	if _, err := io.Copy(w, reader); err != nil {
		return fmt.Errorf("failed io.Copy: %w", err)
	}

	log.Printf("[INFO] Artifact downloaded")

	return nil
}

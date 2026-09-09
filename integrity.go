package dewy

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"path"
	"strings"

	"github.com/linyows/dewy/artifact"
	"github.com/linyows/dewy/checksum"
	"github.com/linyows/dewy/registry"
)

// ChecksumMode selects how a downloaded artifact is checked against the
// SHA-256 checksum file published next to it.
type ChecksumMode int

const (
	// ChecksumAuto verifies the artifact when the registry found a checksum
	// file, and deploys without verification when it did not. It is the
	// default so that a release with checksums is protected without
	// configuration, and a release without them keeps working.
	ChecksumAuto ChecksumMode = iota
	// ChecksumOff never fetches a checksum file.
	ChecksumOff
	// ChecksumRequired fails the deployment when no checksum file is
	// published, in addition to failing on a mismatch.
	ChecksumRequired
)

// String to string for ChecksumMode.
func (m ChecksumMode) String() string {
	switch m {
	case ChecksumAuto:
		return "auto"
	case ChecksumOff:
		return "off"
	case ChecksumRequired:
		return "required"
	default:
		return "unknown"
	}
}

// ParseChecksumMode converts the --verify-checksum argument. An empty string
// selects the default.
func ParseChecksumMode(s string) (ChecksumMode, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "auto":
		return ChecksumAuto, nil
	case "off":
		return ChecksumOff, nil
	case "required":
		return ChecksumRequired, nil
	default:
		return ChecksumAuto, fmt.Errorf("invalid checksum mode %q: expected off, auto or required", s)
	}
}

// verifyChecksum checks the downloaded bytes against the checksum file the
// registry located next to the artifact. The caller must not write data to
// the cache before this returns without an error.
//
// Only a fresh download reaches this. An artifact already staged in the cache
// was verified when it was first downloaded, by this instance or by another
// one sharing the same cache backend.
func (d *Dewy) verifyChecksum(ctx context.Context, res *registry.CurrentResponse, data []byte) error {
	if d.config.ChecksumMode == ChecksumOff {
		return nil
	}

	artifactName := artifactNameFromURL(res.ArtifactURL)

	if res.ChecksumURL == "" {
		if d.config.ChecksumMode == ChecksumRequired {
			return fmt.Errorf("no checksum file published for %s and --verify-checksum=required", artifactName)
		}
		d.logger.Debug("No checksum file published; skipping verification",
			slog.String("artifact", artifactName))
		return nil
	}

	content, err := d.fetchChecksumFile(ctx, res.ChecksumURL)
	if err != nil {
		return fmt.Errorf("failed to fetch the checksum file %s: %w", res.ChecksumURL, err)
	}

	want, err := compareChecksum(content, artifactName, data)
	if err != nil {
		return fmt.Errorf("%s: %w", res.ChecksumURL, err)
	}

	d.logger.Info("Verified artifact checksum",
		slog.String("artifact", artifactName),
		slog.String("sha256", want))
	return nil
}

// compareChecksum reads the digest recorded for artifactName in the content
// of a checksum file and compares it with data, returning the digest that
// matched.
func compareChecksum(content []byte, artifactName string, data []byte) (string, error) {
	want, err := checksum.Parse(content, artifactName)
	if err != nil {
		return "", err
	}
	if err := checksum.Verify(data, want); err != nil {
		return "", fmt.Errorf("checksum verification failed for %s: %w", artifactName, err)
	}
	return want, nil
}

// fetchChecksumFile downloads the checksum file, which uses the same scheme
// and credentials as the artifact itself.
func (d *Dewy) fetchChecksumFile(ctx context.Context, url string) ([]byte, error) {
	a, err := artifact.New(ctx, url, d.logger.Slog())
	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	if err := a.Download(ctx, &limitedWriter{W: buf, N: checksum.MaxFileSize}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// artifactNameFromURL returns the file name an artifact URL ends with, with
// any query string removed.
func artifactNameFromURL(url string) string {
	u, _, _ := strings.Cut(url, "?")
	return path.Base(u)
}

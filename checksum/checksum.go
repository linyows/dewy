// Package checksum locates and applies SHA-256 checksum files published
// alongside an artifact.
//
// Two shapes are in common use. A per-artifact file holds the digest of one
// artifact and is named after it (app_linux_amd64.tar.gz.sha256). An
// aggregate file holds one line per artifact in a release (checksums.txt,
// SHA256SUMS) in the format sha256sum writes:
//
//	<64 hex characters>  <filename>
//
// Both are recognized. Only SHA-256 is supported: it is what GoReleaser,
// sha256sum and the GitHub release tooling emit by default.
package checksum

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"strings"
)

// hexLen is the length of a SHA-256 digest in hexadecimal.
const hexLen = sha256.Size * 2

// MaxFileSize caps how much of a checksum file is read. An aggregate file for
// a large release stays well under this; anything larger is not a checksum
// file.
const MaxFileSize int64 = 1 << 20

// perArtifactSuffixes are the extensions appended to an artifact name to name
// its own checksum file, in the order they are looked for.
var perArtifactSuffixes = []string{".sha256", ".sha256sum", ".sha256.txt", ".sha256sums"}

// aggregateNames are the file names, matched case-insensitively, that hold
// the digests of every artifact in a release.
var aggregateNames = []string{"sha256sums", "sha256sums.txt", "checksums.txt", "checksums.sha256", "checksums_sha256.txt"}

// aggregateSuffixes cover the tool-generated aggregate names that carry a
// project or version prefix, such as myapp_1.2.3_checksums.txt.
var aggregateSuffixes = []string{"_checksums.txt", "-checksums.txt", "_sha256sums.txt", "-sha256sums.txt"}

// FindFile returns the name of the checksum file covering artifactName from
// the names available next to it, or an empty string when there is none.
// A file named after the artifact wins over an aggregate file.
func FindFile(artifactName string, names []string) string {
	index := make(map[string]string, len(names))
	for _, n := range names {
		index[strings.ToLower(n)] = n
	}

	for _, suffix := range perArtifactSuffixes {
		if name, ok := index[strings.ToLower(artifactName)+suffix]; ok {
			return name
		}
	}

	for _, candidate := range aggregateNames {
		if name, ok := index[candidate]; ok {
			return name
		}
	}

	for _, n := range names {
		lower := strings.ToLower(n)
		for _, suffix := range aggregateSuffixes {
			if strings.HasSuffix(lower, suffix) {
				return n
			}
		}
	}

	return ""
}

// Parse returns the SHA-256 digest recorded for artifactName in the content
// of a checksum file, as lowercase hexadecimal.
//
// A file holding a single digest and no file name is accepted as covering the
// artifact, which is what "sha256sum < file" and several release pipelines
// produce. Otherwise the line whose file name matches artifactName is used;
// the comparison is on the base name, since aggregate files sometimes record
// a path.
func Parse(content []byte, artifactName string) (string, error) {
	var (
		lines      int
		loneDigest string
	)

	for raw := range strings.SplitSeq(string(content), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lines++

		fields := strings.Fields(line)
		digest := strings.ToLower(fields[0])
		if !isHexDigest(digest) {
			continue
		}

		if len(fields) == 1 {
			loneDigest = digest
			continue
		}

		// sha256sum marks binary mode with a "*" before the file name.
		name := strings.TrimPrefix(fields[len(fields)-1], "*")
		if path.Base(name) == path.Base(artifactName) {
			return digest, nil
		}
	}

	if loneDigest != "" && lines == 1 {
		return loneDigest, nil
	}

	return "", fmt.Errorf("no sha256 digest for %q in the checksum file", artifactName)
}

// Verify reports whether data hashes to want. want is a lowercase
// hexadecimal SHA-256 digest as returned by Parse.
func Verify(data []byte, want string) error {
	sum := sha256.Sum256(data)
	got := hex.EncodeToString(sum[:])
	if got != want {
		return fmt.Errorf("sha256 mismatch: artifact is %s, checksum file says %s", got, want)
	}
	return nil
}

func isHexDigest(s string) bool {
	if len(s) != hexLen {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

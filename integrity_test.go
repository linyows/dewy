package dewy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/linyows/dewy/registry"
)

func TestParseChecksumMode(t *testing.T) {
	tests := []struct {
		in      string
		want    ChecksumMode
		wantErr bool
	}{
		{"", ChecksumAuto, false},
		{"auto", ChecksumAuto, false},
		{"AUTO", ChecksumAuto, false},
		{" off ", ChecksumOff, false},
		{"required", ChecksumRequired, false},
		{"yes", ChecksumAuto, true},
	}

	for _, tt := range tests {
		got, err := ParseChecksumMode(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParseChecksumMode(%q) = %v, want an error", tt.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseChecksumMode(%q) error: %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseChecksumMode(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestChecksumModeString(t *testing.T) {
	tests := map[ChecksumMode]string{
		ChecksumAuto:     "auto",
		ChecksumOff:      "off",
		ChecksumRequired: "required",
		ChecksumMode(9):  "unknown",
	}
	for mode, want := range tests {
		if got := mode.String(); got != want {
			t.Errorf("ChecksumMode(%d).String() = %q, want %q", mode, got, want)
		}
	}
}

func TestArtifactNameFromURL(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"ghr://linyows/dewy/tag/v1.2.3/app_linux_amd64.tar.gz", "app_linux_amd64.tar.gz"},
		{"s3://ap-northeast-1/bucket/app/v1.2.3/app.zip?endpoint=http://localhost:9000", "app.zip"},
		{"gs://bucket/app/v1.2.3/app.tar.gz", "app.tar.gz"},
	}
	for _, tt := range tests {
		if got := artifactNameFromURL(tt.url); got != tt.want {
			t.Errorf("artifactNameFromURL(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestCompareChecksum(t *testing.T) {
	data := []byte("artifact contents")
	sum := sha256.Sum256(data)
	digest := hex.EncodeToString(sum[:])

	t.Run("matching digest", func(t *testing.T) {
		content := []byte(digest + "  app_linux_amd64.tar.gz\n")
		got, err := compareChecksum(content, "app_linux_amd64.tar.gz", data)
		if err != nil {
			t.Fatalf("compareChecksum() error: %v", err)
		}
		if got != digest {
			t.Errorf("compareChecksum() = %q, want %q", got, digest)
		}
	})

	t.Run("mismatched digest", func(t *testing.T) {
		content := []byte(digest + "  app_linux_amd64.tar.gz\n")
		_, err := compareChecksum(content, "app_linux_amd64.tar.gz", []byte("tampered"))
		if err == nil {
			t.Fatal("compareChecksum() = nil, want an error for altered contents")
		}
		if !strings.Contains(err.Error(), "app_linux_amd64.tar.gz") {
			t.Errorf("error %q does not name the artifact", err)
		}
	})

	t.Run("artifact absent from the checksum file", func(t *testing.T) {
		content := []byte(digest + "  other.tar.gz\n")
		if _, err := compareChecksum(content, "app_linux_amd64.tar.gz", data); err == nil {
			t.Fatal("compareChecksum() = nil, want an error when the artifact is not listed")
		}
	})
}

func TestVerifyChecksum_WithoutFetching(t *testing.T) {
	res := &registry.CurrentResponse{
		Tag:         "v1.2.3",
		ArtifactURL: "ghr://linyows/dewy/tag/v1.2.3/app_linux_amd64.tar.gz",
	}

	t.Run("off skips verification", func(t *testing.T) {
		d := newPhaseTestDewy(t)
		d.config.ChecksumMode = ChecksumOff
		withChecksum := *res
		withChecksum.ChecksumURL = "ghr://linyows/dewy/tag/v1.2.3/checksums.txt"
		if err := d.verifyChecksum(context.Background(), &withChecksum, []byte("data")); err != nil {
			t.Errorf("verifyChecksum() = %v, want nil", err)
		}
	})

	t.Run("auto deploys a release that publishes no checksum", func(t *testing.T) {
		d := newPhaseTestDewy(t)
		d.config.ChecksumMode = ChecksumAuto
		if err := d.verifyChecksum(context.Background(), res, []byte("data")); err != nil {
			t.Errorf("verifyChecksum() = %v, want nil", err)
		}
	})

	t.Run("required rejects a release that publishes no checksum", func(t *testing.T) {
		d := newPhaseTestDewy(t)
		d.config.ChecksumMode = ChecksumRequired
		err := d.verifyChecksum(context.Background(), res, []byte("data"))
		if err == nil {
			t.Fatal("verifyChecksum() = nil, want an error under --verify-checksum=required")
		}
		if !strings.Contains(err.Error(), "app_linux_amd64.tar.gz") {
			t.Errorf("error %q does not name the artifact", err)
		}
	})
}

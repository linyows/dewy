package registry

import (
	"testing"
	"time"
)

func TestArtifactNotFoundError_Error(t *testing.T) {
	err := &ArtifactNotFoundError{
		ArtifactName: "test-artifact",
		Message:      "artifact not found: test-artifact",
	}

	expected := "artifact not found: test-artifact"
	if err.Error() != expected {
		t.Errorf("Error() = %v, want %v", err.Error(), expected)
	}
}

func TestArtifactNotFoundError_IsWithinGracePeriod(t *testing.T) {
	tests := []struct {
		name        string
		releaseTime *time.Time
		gracePeriod time.Duration
		want        bool
	}{
		{
			name:        "within grace period",
			releaseTime: func() *time.Time { t := time.Now().Add(-5 * time.Minute); return &t }(),
			gracePeriod: 10 * time.Minute,
			want:        true,
		},
		{
			name:        "outside grace period",
			releaseTime: func() *time.Time { t := time.Now().Add(-15 * time.Minute); return &t }(),
			gracePeriod: 10 * time.Minute,
			want:        false,
		},
		{
			name:        "no release time",
			releaseTime: nil,
			gracePeriod: 10 * time.Minute,
			want:        false,
		},
		{
			name:        "zero grace period",
			releaseTime: func() *time.Time { t := time.Now().Add(-5 * time.Minute); return &t }(),
			gracePeriod: 0,
			want:        false,
		},
		{
			name:        "exactly at grace period boundary",
			releaseTime: func() *time.Time { t := time.Now().Add(-10 * time.Minute); return &t }(),
			gracePeriod: 10 * time.Minute,
			want:        false, // Should be false as time.Since will be slightly > 10m
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &ArtifactNotFoundError{
				ArtifactName: "test",
				ReleaseTime:  tt.releaseTime,
				Message:      "test message",
			}
			if got := err.IsWithinGracePeriod(tt.gracePeriod); got != tt.want {
				t.Errorf("IsWithinGracePeriod() = %v, want %v", got, tt.want)
			}
		})
	}
}

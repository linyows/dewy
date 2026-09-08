package registry

import "testing"

func TestChannelOf(t *testing.T) {
	tests := []struct {
		preRelease string
		want       string
	}{
		{"", "stable"},
		{"canary", "canary"},
		{"canary.1", "canary"},
		{"rc.1", "rc"},
		{"beta.10.2", "beta"},
	}
	for _, tt := range tests {
		if got := ChannelOf(tt.preRelease); got != tt.want {
			t.Errorf("ChannelOf(%q) = %q, want %q", tt.preRelease, got, tt.want)
		}
	}
}

func TestChannelMatcher(t *testing.T) {
	tests := []struct {
		name       string
		expected   string
		preRelease string
		want       bool
	}{
		{"an unset matcher takes a final release", "", "", true},
		{"an unset matcher takes a pre-release", "", "canary.1", true},
		{"stable takes a final release", "stable", "", true},
		{"stable rejects a pre-release", "stable", "canary.1", false},
		{"canary takes its own channel", "canary", "canary.1", true},
		{"canary takes the bare identifier", "canary", "canary", true},
		{"canary rejects another channel", "canary", "beta.1", false},
		{"canary rejects a final release", "canary", "", false},
		{"names are compared case-insensitively", "Canary", "canary.1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := ChannelMatcher{Expected: tt.expected}
			if got := m.Matches(tt.preRelease); got != tt.want {
				t.Errorf("ChannelMatcher{%q}.Matches(%q) = %v, want %v", tt.expected, tt.preRelease, got, tt.want)
			}
		})
	}
}

func TestVersionFilterAllowPreRelease(t *testing.T) {
	tests := []struct {
		name   string
		filter VersionFilter
		want   bool
	}{
		{"no channel, pre-release off", VersionFilter{}, false},
		{"no channel, pre-release on", VersionFilter{PreRelease: true}, true},
		{"a channel admits pre-releases on its own", VersionFilter{Channel: "canary"}, true},
		{"stable excludes pre-releases even with the flag", VersionFilter{Channel: "stable", PreRelease: true}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.filter.allowPreRelease(); got != tt.want {
				t.Errorf("allowPreRelease() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFindLatestSemVerWithChannel(t *testing.T) {
	versions := []string{
		"v1.0.0",
		"v1.1.0",
		"v1.2.0-canary.1",
		"v1.2.0-canary.2",
		"v1.2.0-beta.1",
		"v2.0.0-canary.1",
	}

	tests := []struct {
		name   string
		filter VersionFilter
		want   string
	}{
		{
			name:   "canary takes the newest canary tag",
			filter: VersionFilter{Channel: "canary"},
			want:   "v2.0.0-canary.1",
		},
		{
			name:   "beta takes the newest beta tag",
			filter: VersionFilter{Channel: "beta"},
			want:   "v1.2.0-beta.1",
		},
		{
			name:   "stable takes the newest final release",
			filter: VersionFilter{Channel: "stable"},
			want:   "v1.1.0",
		},
		{
			name:   "stable ignores the pre-release flag",
			filter: VersionFilter{Channel: "stable", PreRelease: true},
			want:   "v1.1.0",
		},
		{
			name:   "no channel and no pre-release matches the previous behavior",
			filter: VersionFilter{},
			want:   "v1.1.0",
		},
		{
			name:   "no channel with pre-release takes any newest tag",
			filter: VersionFilter{PreRelease: true},
			want:   "v2.0.0-canary.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, name, err := FindLatestSemVerWith(versions, tt.filter)
			if err != nil {
				t.Fatalf("FindLatestSemVerWith() error: %v", err)
			}
			if name != tt.want {
				t.Errorf("FindLatestSemVerWith() = %q, want %q", name, tt.want)
			}
		})
	}
}

func TestFindLatestSemVerWithChannelAndSlot(t *testing.T) {
	versions := []string{
		"v1.2.0-canary.1+blue",
		"v1.2.0-canary.2+green",
		"v1.3.0-canary.1+green",
		"v1.4.0+blue",
	}

	_, name, err := FindLatestSemVerWith(versions, VersionFilter{Channel: "canary", Slot: "blue"})
	if err != nil {
		t.Fatalf("FindLatestSemVerWith() error: %v", err)
	}
	if name != "v1.2.0-canary.1+blue" {
		t.Errorf("FindLatestSemVerWith() = %q, want v1.2.0-canary.1+blue", name)
	}
}

func TestFindLatestSemVerWithChannelNoMatch(t *testing.T) {
	versions := []string{"v1.0.0", "v1.1.0"}

	if _, name, err := FindLatestSemVerWith(versions, VersionFilter{Channel: "canary"}); err == nil {
		t.Errorf("FindLatestSemVerWith() = %q, want an error when the channel has no versions", name)
	}
}

func TestFindLatestCalVerWithChannel(t *testing.T) {
	versions := []string{
		"2024.06.0",
		"2024.06.1",
		"2024.07.0-canary.1",
		"2024.08.0-beta.1",
	}
	const format = "YYYY.0M.MICRO"

	tests := []struct {
		name   string
		filter VersionFilter
		want   string
	}{
		{"canary", VersionFilter{Channel: "canary"}, "2024.07.0-canary.1"},
		{"stable", VersionFilter{Channel: "stable"}, "2024.06.1"},
		{"no channel with pre-release", VersionFilter{PreRelease: true}, "2024.08.0-beta.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, name, err := FindLatestCalVerWith(versions, format, tt.filter)
			if err != nil {
				t.Fatalf("FindLatestCalVerWith() error: %v", err)
			}
			if name != tt.want {
				t.Errorf("FindLatestCalVerWith() = %q, want %q", name, tt.want)
			}
		})
	}
}

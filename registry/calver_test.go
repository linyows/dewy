package registry

import (
	"testing"
)

func TestNewCalVerFormat(t *testing.T) {
	tests := []struct {
		desc      string
		format    string
		expectErr bool
	}{
		{"YYYY.0M.MICRO", "YYYY.0M.MICRO", false},
		{"YYYY.MM.DD", "YYYY.MM.DD", false},
		{"YY.MM.MICRO", "YY.MM.MICRO", false},
		{"0Y.0M.0D", "0Y.0M.0D", false},
		{"YYYY.0W.MICRO", "YYYY.0W.MICRO", false},
		{"empty format", "", true},
		{"no valid specifiers", "yyyy.MICR", true},
		{"mixed valid and invalid", "foo.bar", true},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			f, err := NewCalVerFormat(tt.format)
			if tt.expectErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if f.Format != tt.format {
				t.Errorf("expected format %s, got %s", tt.format, f.Format)
			}
		})
	}
}

func TestCalVerFormatParse(t *testing.T) {
	tests := []struct {
		desc     string
		format   string
		version  string
		expected *CalVer
	}{
		{
			"YYYY.0M.MICRO basic",
			"YYYY.0M.MICRO",
			"2024.01.0",
			&CalVer{Segments: []int{2024, 1, 0}, Original: "2024.01.0"},
		},
		{
			"YYYY.0M.MICRO with v prefix",
			"YYYY.0M.MICRO",
			"v2024.01.42",
			&CalVer{V: "v", Segments: []int{2024, 1, 42}, Original: "v2024.01.42"},
		},
		{
			"YYYY.0M.MICRO with build metadata",
			"YYYY.0M.MICRO",
			"2024.06.3+blue",
			&CalVer{Segments: []int{2024, 6, 3}, BuildMetadata: "blue", Original: "2024.06.3+blue"},
		},
		{
			"YYYY.MM.DD",
			"YYYY.MM.DD",
			"2024.1.9",
			&CalVer{Segments: []int{2024, 1, 9}, Original: "2024.1.9"},
		},
		{
			"YYYY.MM.DD with double digit",
			"YYYY.MM.DD",
			"2024.11.31",
			&CalVer{Segments: []int{2024, 11, 31}, Original: "2024.11.31"},
		},
		{
			"YY.MM.MICRO",
			"YY.MM.MICRO",
			"24.1.5",
			&CalVer{Segments: []int{24, 1, 5}, Original: "24.1.5"},
		},
		{
			"YY.MM.MICRO with 3-digit year",
			"YY.MM.MICRO",
			"106.1.5",
			&CalVer{Segments: []int{106, 1, 5}, Original: "106.1.5"},
		},
		{
			"0Y.0M.0D",
			"0Y.0M.0D",
			"24.01.09",
			&CalVer{Segments: []int{24, 1, 9}, Original: "24.01.09"},
		},
		{
			"0Y.0M.0D with 3-digit year",
			"0Y.0M.0D",
			"106.01.09",
			&CalVer{Segments: []int{106, 1, 9}, Original: "106.01.09"},
		},
		{
			"YYYY.0W.MICRO",
			"YYYY.0W.MICRO",
			"2024.01.3",
			&CalVer{Segments: []int{2024, 1, 3}, Original: "2024.01.3"},
		},
		{
			"YYYY.0M.MICRO with pre-release",
			"YYYY.0M.MICRO",
			"2024.06.3-rc.1",
			&CalVer{Segments: []int{2024, 6, 3}, PreRelease: "rc.1", Original: "2024.06.3-rc.1"},
		},
		{
			"YYYY.0M.MICRO with pre-release and build metadata",
			"YYYY.0M.MICRO",
			"v2024.06.3-beta.2+blue",
			&CalVer{V: "v", Segments: []int{2024, 6, 3}, PreRelease: "beta.2", BuildMetadata: "blue", Original: "v2024.06.3-beta.2+blue"},
		},
		{
			"non-matching version returns nil",
			"YYYY.0M.MICRO",
			"not-a-version",
			nil,
		},
		{
			"semver does not match calver format",
			"YYYY.0M.MICRO",
			"v1.2.3",
			nil,
		},
		{
			"invalid month for 0M format",
			"YYYY.0M.MICRO",
			"2024.13.0",
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			f, err := NewCalVerFormat(tt.format)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := f.Parse(tt.version)
			if tt.expected == nil {
				if got != nil {
					t.Errorf("expected nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected %+v, got nil", tt.expected)
			}
			if got.V != tt.expected.V {
				t.Errorf("V: expected %q, got %q", tt.expected.V, got.V)
			}
			if len(got.Segments) != len(tt.expected.Segments) {
				t.Fatalf("Segments length: expected %d, got %d", len(tt.expected.Segments), len(got.Segments))
			}
			for i, v := range got.Segments {
				if v != tt.expected.Segments[i] {
					t.Errorf("Segments[%d]: expected %d, got %d", i, tt.expected.Segments[i], v)
				}
			}
			if got.PreRelease != tt.expected.PreRelease {
				t.Errorf("PreRelease: expected %q, got %q", tt.expected.PreRelease, got.PreRelease)
			}
			if got.BuildMetadata != tt.expected.BuildMetadata {
				t.Errorf("BuildMetadata: expected %q, got %q", tt.expected.BuildMetadata, got.BuildMetadata)
			}
			if got.Original != tt.expected.Original {
				t.Errorf("Original: expected %q, got %q", tt.expected.Original, got.Original)
			}
		})
	}
}

func TestCalVerCompare(t *testing.T) {
	tests := []struct {
		desc     string
		a, b     *CalVer
		expected int
	}{
		{
			"equal versions",
			&CalVer{Segments: []int{2024, 1, 0}},
			&CalVer{Segments: []int{2024, 1, 0}},
			0,
		},
		{
			"first segment greater",
			&CalVer{Segments: []int{2025, 1, 0}},
			&CalVer{Segments: []int{2024, 1, 0}},
			1,
		},
		{
			"first segment less",
			&CalVer{Segments: []int{2023, 1, 0}},
			&CalVer{Segments: []int{2024, 1, 0}},
			-1,
		},
		{
			"second segment greater",
			&CalVer{Segments: []int{2024, 6, 0}},
			&CalVer{Segments: []int{2024, 1, 0}},
			1,
		},
		{
			"third segment greater",
			&CalVer{Segments: []int{2024, 1, 5}},
			&CalVer{Segments: []int{2024, 1, 3}},
			1,
		},
		{
			"stable beats pre-release",
			&CalVer{Segments: []int{2024, 1, 0}, PreRelease: ""},
			&CalVer{Segments: []int{2024, 1, 0}, PreRelease: "rc.1"},
			1,
		},
		{
			"pre-release loses to stable",
			&CalVer{Segments: []int{2024, 1, 0}, PreRelease: "beta"},
			&CalVer{Segments: []int{2024, 1, 0}, PreRelease: ""},
			-1,
		},
		{
			"pre-release lexicographic comparison",
			&CalVer{Segments: []int{2024, 1, 0}, PreRelease: "rc.1"},
			&CalVer{Segments: []int{2024, 1, 0}, PreRelease: "beta.1"},
			1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := tt.a.Compare(tt.b)
			if (tt.expected > 0 && got <= 0) || (tt.expected < 0 && got >= 0) || (tt.expected == 0 && got != 0) {
				t.Errorf("Compare: expected sign %d, got %d", tt.expected, got)
			}
		})
	}
}

func TestCalVerGetBuildMetadata(t *testing.T) {
	cv := &CalVer{
		Segments:      []int{2024, 1, 0},
		BuildMetadata: "blue",
		Original:      "2024.01.0+blue",
	}
	if got := cv.GetBuildMetadata(); got != "blue" {
		t.Errorf("expected 'blue', got %q", got)
	}
}

func TestCalVerString(t *testing.T) {
	cv := &CalVer{
		Segments: []int{2024, 1, 0},
		Original: "v2024.01.0",
	}
	if got := cv.String(); got != "v2024.01.0" {
		t.Errorf("expected 'v2024.01.0', got %q", got)
	}
}

func TestFindLatestCalVer(t *testing.T) {
	tests := []struct {
		desc            string
		versions        []string
		format          string
		allowPreRelease bool
		expectedVer     string
		expectedTag     string
		expectErr       bool
	}{
		{
			"basic YYYY.0M.MICRO",
			[]string{"2024.01.0", "2024.01.1", "2024.06.0", "2023.12.5"},
			"YYYY.0M.MICRO",
			false,
			"2024.06.0",
			"2024.06.0",
			false,
		},
		{
			"with v prefix",
			[]string{"v2024.01.0", "v2024.06.3", "v2024.03.1"},
			"YYYY.0M.MICRO",
			false,
			"v2024.06.3",
			"v2024.06.3",
			false,
		},
		{
			"mixed with non-matching",
			[]string{"v1.2.3", "2024.01.0", "foobar", "2024.06.1"},
			"YYYY.0M.MICRO",
			false,
			"2024.06.1",
			"2024.06.1",
			false,
		},
		{
			"YYYY.MM.DD format",
			[]string{"2024.1.15", "2024.2.1", "2024.1.31"},
			"YYYY.MM.DD",
			false,
			"2024.2.1",
			"2024.2.1",
			false,
		},
		{
			"no matching versions",
			[]string{"v1.2.3", "foobar"},
			"YYYY.0M.MICRO",
			false,
			"",
			"",
			true,
		},
		{
			"pre-release excluded by default",
			[]string{"2024.01.0", "2024.06.0-rc.1", "2024.03.1"},
			"YYYY.0M.MICRO",
			false,
			"2024.03.1",
			"2024.03.1",
			false,
		},
		{
			"pre-release included when allowed",
			[]string{"2024.01.0", "2024.06.0-rc.1", "2024.03.1"},
			"YYYY.0M.MICRO",
			true,
			"2024.06.0-rc.1",
			"2024.06.0-rc.1",
			false,
		},
		{
			"stable preferred over pre-release at same segments",
			[]string{"2024.06.0-beta.1", "2024.06.0"},
			"YYYY.0M.MICRO",
			true,
			"2024.06.0",
			"2024.06.0",
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ver, tag, err := FindLatestCalVer(tt.versions, tt.format, tt.allowPreRelease)
			if tt.expectErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ver.String() != tt.expectedVer {
				t.Errorf("expected version %q, got %q", tt.expectedVer, ver.String())
			}
			if tag != tt.expectedTag {
				t.Errorf("expected tag %q, got %q", tt.expectedTag, tag)
			}
		})
	}
}

func TestFindLatestCalVerWithSlot(t *testing.T) {
	tests := []struct {
		desc            string
		versions        []string
		format          string
		slot            string
		allowPreRelease bool
		expectedVer     string
		expectedTag     string
		expectErr       bool
	}{
		{
			"slot filter blue",
			[]string{"2024.01.0+blue", "2024.06.0+green", "2024.06.1+blue", "2024.12.0+green"},
			"YYYY.0M.MICRO",
			"blue",
			false,
			"2024.06.1+blue",
			"2024.06.1+blue",
			false,
		},
		{
			"slot filter green",
			[]string{"2024.01.0+blue", "2024.06.0+green", "2024.06.1+blue", "2024.12.0+green"},
			"YYYY.0M.MICRO",
			"green",
			false,
			"2024.12.0+green",
			"2024.12.0+green",
			false,
		},
		{
			"empty slot matches all",
			[]string{"2024.01.0+blue", "2024.06.0+green", "2024.12.0"},
			"YYYY.0M.MICRO",
			"",
			false,
			"2024.12.0",
			"2024.12.0",
			false,
		},
		{
			"no matching slot",
			[]string{"2024.01.0+blue", "2024.06.0+green"},
			"YYYY.0M.MICRO",
			"red",
			false,
			"",
			"",
			true,
		},
		{
			"pre-release with slot",
			[]string{"2024.01.0-rc.1+blue", "2024.06.0+blue", "2024.06.1-beta+blue"},
			"YYYY.0M.MICRO",
			"blue",
			true,
			"2024.06.1-beta+blue",
			"2024.06.1-beta+blue",
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ver, tag, err := FindLatestCalVerWithSlot(tt.versions, tt.format, tt.slot, tt.allowPreRelease)
			if tt.expectErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ver.String() != tt.expectedVer {
				t.Errorf("expected version %q, got %q", tt.expectedVer, ver.String())
			}
			if tag != tt.expectedTag {
				t.Errorf("expected tag %q, got %q", tt.expectedTag, tag)
			}
		})
	}
}

func TestCalVerVersionInterface(t *testing.T) {
	cv := &CalVer{
		V:             "v",
		Segments:      []int{2024, 6, 3},
		BuildMetadata: "blue",
		Original:      "v2024.06.3+blue",
	}

	// Verify CalVer implements Version interface
	var v Version = cv
	if v.String() != "v2024.06.3+blue" {
		t.Errorf("String(): expected %q, got %q", "v2024.06.3+blue", v.String())
	}
	if v.GetBuildMetadata() != "blue" {
		t.Errorf("GetBuildMetadata(): expected %q, got %q", "blue", v.GetBuildMetadata())
	}
}

func TestSemVerVersionInterface(t *testing.T) {
	sv := &SemVer{
		V:             "v",
		Major:         1,
		Minor:         2,
		Patch:         3,
		BuildMetadata: "green",
	}

	// Verify SemVer implements Version interface
	var v Version = sv
	if v.String() != "v1.2.3+green" {
		t.Errorf("String(): expected %q, got %q", "v1.2.3+green", v.String())
	}
	if v.GetBuildMetadata() != "green" {
		t.Errorf("GetBuildMetadata(): expected %q, got %q", "green", v.GetBuildMetadata())
	}
}

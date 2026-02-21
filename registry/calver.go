package registry

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// CalVerFormat represents a parsed CalVer format string.
// It builds a regular expression from the format to parse version strings.
type CalVerFormat struct {
	Format   string
	Regex    *regexp.Regexp
	segments []string // segment specifiers in order (e.g., ["YYYY", "0M", "MICRO"])
}

// CalVer represents a parsed calendar version.
type CalVer struct {
	V             string // "v" prefix if present
	Segments      []int
	PreRelease    string
	BuildMetadata string
	Original      string
}

// calverSpecifiers maps format specifiers to their regex patterns.
var calverSpecifiers = map[string]string{
	"YYYY":  `(\d{4})`,
	"YY":    `(\d{1,3})`,
	"0Y":    `(\d{2,3})`,
	"MM":    `([1-9]|1[0-2])`,
	"0M":    `(0[1-9]|1[0-2])`,
	"WW":    `([1-9]|[1-4]\d|5[0-3])`,
	"0W":    `(0[1-9]|[1-4]\d|5[0-3])`,
	"DD":    `([1-9]|[12]\d|3[01])`,
	"0D":    `(0[1-9]|[12]\d|3[01])`,
	"MICRO": `(\d+)`,
}

// calverSpecifierOrder defines the order to try matching specifiers (longest first to avoid partial matches).
var calverSpecifierOrder = []string{
	"YYYY", "0Y", "YY",
	"0M", "MM",
	"0W", "WW",
	"0D", "DD",
	"MICRO",
}

// NewCalVerFormat creates a CalVerFormat from a format string like "YYYY.0M.MICRO".
func NewCalVerFormat(format string) (*CalVerFormat, error) {
	if format == "" {
		return nil, fmt.Errorf("calver format is empty")
	}

	remaining := format
	var regexParts []string
	var segments []string

	for remaining != "" {
		matched := false
		for _, spec := range calverSpecifierOrder {
			if strings.HasPrefix(remaining, spec) {
				regexParts = append(regexParts, calverSpecifiers[spec])
				segments = append(segments, spec)
				remaining = remaining[len(spec):]
				matched = true
				break
			}
		}
		if !matched {
			// Literal character (e.g., ".")
			regexParts = append(regexParts, regexp.QuoteMeta(string(remaining[0])))
			remaining = remaining[1:]
		}
	}

	// Build full regex: optional "v" prefix, the format pattern, optional pre-release, optional build metadata
	pattern := `^(v)?` + strings.Join(regexParts, "") + `(?:-([0-9A-Za-z.-]+))?(?:\+([0-9A-Za-z.-]+))?$`
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to compile calver regex: %w", err)
	}

	return &CalVerFormat{
		Format:   format,
		Regex:    re,
		segments: segments,
	}, nil
}

// Parse parses a version string according to this CalVer format.
// Returns nil if the version string doesn't match the format.
func (f *CalVerFormat) Parse(version string) *CalVer {
	match := f.Regex.FindStringSubmatch(version)
	if match == nil {
		return nil
	}

	v := match[1] // "v" prefix
	var segs []int

	// match[0] is the full match, match[1] is "v" prefix
	// match[2..N] are the segment groups
	// match[N+1] is pre-release, match[N+2] is build metadata
	segStart := 2
	for i := 0; i < len(f.segments); i++ {
		val, err := strconv.Atoi(match[segStart+i])
		if err != nil {
			return nil
		}
		segs = append(segs, val)
	}

	preRelease := match[len(match)-2]
	buildMetadata := match[len(match)-1]

	return &CalVer{
		V:             v,
		Segments:      segs,
		PreRelease:    preRelease,
		BuildMetadata: buildMetadata,
		Original:      version,
	}
}

// Compare compares two CalVer versions segment by segment.
// Returns positive if v > other, negative if v < other, 0 if equal.
// Pre-release versions are considered lower than stable versions (same as SemVer).
func (v *CalVer) Compare(other *CalVer) int {
	maxLen := len(v.Segments)
	if len(other.Segments) > maxLen {
		maxLen = len(other.Segments)
	}

	for i := 0; i < maxLen; i++ {
		var a, b int
		if i < len(v.Segments) {
			a = v.Segments[i]
		}
		if i < len(other.Segments) {
			b = other.Segments[i]
		}
		if a != b {
			return a - b
		}
	}

	// Pre-release handling: stable (no pre-release) > pre-release
	if v.PreRelease == "" && other.PreRelease != "" {
		return 1
	}
	if v.PreRelease != "" && other.PreRelease == "" {
		return -1
	}
	return strings.Compare(v.PreRelease, other.PreRelease)
}

// String returns the original version string.
func (v *CalVer) String() string {
	return v.Original
}

// GetBuildMetadata returns the build metadata of the CalVer version.
func (v *CalVer) GetBuildMetadata() string {
	return v.BuildMetadata
}

// FindLatestCalVer finds the latest calendar version from a list of version strings.
func FindLatestCalVer(versionNames []string, format string, allowPreRelease bool) (*CalVer, string, error) {
	return FindLatestCalVerWithSlot(versionNames, format, "", allowPreRelease)
}

// FindLatestCalVerWithSlot finds the latest calendar version that matches the specified slot.
// If slot is empty, it matches versions without build metadata or any build metadata.
// If allowPreRelease is false, versions with pre-release identifiers are excluded.
func FindLatestCalVerWithSlot(versionNames []string, format, slot string, allowPreRelease bool) (*CalVer, string, error) {
	f, err := NewCalVerFormat(format)
	if err != nil {
		return nil, "", err
	}

	var latestVersion *CalVer
	var latestName string

	for _, name := range versionNames {
		ver := f.Parse(name)
		if ver == nil {
			continue
		}

		// Pre-release filtering
		if !allowPreRelease && ver.PreRelease != "" {
			continue
		}

		// Slot filtering: if slot is specified, only match versions with that build metadata
		if slot != "" && ver.BuildMetadata != slot {
			continue
		}

		if latestVersion == nil || ver.Compare(latestVersion) > 0 {
			latestVersion = ver
			latestName = name
		}
	}

	if latestVersion == nil {
		return nil, "", fmt.Errorf("no valid versioned object found")
	}

	return latestVersion, latestName, nil
}

package registry

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	SemVerRegexWithoutPreRelease = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)$`)
	SemVerRegex                  = regexp.MustCompile(`^(v)?(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z.-]+))?$`)
)

func ParseSemVer(version string) *SemVer {
	match := SemVerRegex.FindStringSubmatch(version)
	if match == nil {
		return nil
	}

	v := match[1]
	major, _ := strconv.Atoi(match[2])
	minor, _ := strconv.Atoi(match[3])
	patch, _ := strconv.Atoi(match[4])
	preRelease := match[5]

	return &SemVer{
		V:          v,
		Major:      major,
		Minor:      minor,
		Patch:      patch,
		PreRelease: preRelease,
	}
}

type SemVer struct {
	V          string
	Major      int
	Minor      int
	Patch      int
	PreRelease string
}

func (v *SemVer) Compare(other *SemVer) int {
	if v.Major != other.Major {
		return v.Major - other.Major
	}
	if v.Minor != other.Minor {
		return v.Minor - other.Minor
	}
	if v.Patch != other.Patch {
		return v.Patch - other.Patch
	}
	if v.PreRelease == "" && other.PreRelease != "" {
		return 1
	}
	if v.PreRelease != "" && other.PreRelease == "" {
		return -1
	}
	return strings.Compare(v.PreRelease, other.PreRelease)
}

func (v *SemVer) String() string {
	var pre string
	if v.PreRelease != "" {
		pre = fmt.Sprintf("-%s", v.PreRelease)
	}
	return fmt.Sprintf("%s%d.%d.%d%s", v.V, v.Major, v.Minor, v.Patch, pre)
}

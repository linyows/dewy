package registry

import (
	"strconv"
	"strings"
)

// Version is a common interface for version types (SemVer, CalVer).
type Version interface {
	String() string
	GetBuildMetadata() string
}

// comparePreRelease compares two pre-release strings according to SemVer v2 spec (item 11).
// Both a and b should be non-empty pre-release strings (the caller handles stable vs pre-release).
// Identifiers are split by "." and compared left to right:
//   - Numeric identifiers are compared as integers.
//   - Alphanumeric identifiers are compared lexically.
//   - Numeric identifiers always have lower precedence than alphanumeric.
//   - A shorter set of identifiers has lower precedence when all preceding identifiers are equal.
func comparePreRelease(a, b string) int {
	if a == b {
		return 0
	}

	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")

	maxLen := len(partsA)
	if len(partsB) > maxLen {
		maxLen = len(partsB)
	}

	for i := 0; i < maxLen; i++ {
		if i >= len(partsA) {
			return -1
		}
		if i >= len(partsB) {
			return 1
		}

		numA, errA := strconv.Atoi(partsA[i])
		numB, errB := strconv.Atoi(partsB[i])

		switch {
		case errA == nil && errB == nil:
			if numA != numB {
				return numA - numB
			}
		case errA == nil:
			return -1
		case errB == nil:
			return 1
		default:
			if cmp := strings.Compare(partsA[i], partsB[i]); cmp != 0 {
				return cmp
			}
		}
	}

	return 0
}

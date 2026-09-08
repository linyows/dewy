package registry

import "strings"

// StableChannel is the channel name of a version with no pre-release
// identifier, such as v1.2.3.
const StableChannel = "stable"

// ChannelOf returns the channel a pre-release identifier belongs to: its
// leading dot-separated component, or "stable" when there is no pre-release
// identifier at all.
//
//	""          -> "stable"
//	"canary"    -> "canary"
//	"canary.4"  -> "canary"
//	"rc.1"      -> "rc"
func ChannelOf(preRelease string) string {
	if preRelease == "" {
		return StableChannel
	}
	head, _, _ := strings.Cut(preRelease, ".")
	return head
}

// ChannelMatcher decides whether a version belongs to the channel a Dewy
// instance tracks. An empty Expected matches every version, which is the
// behavior of instances that do not use channels.
//
// Slot and channel answer different questions and are independent. A slot is
// build metadata (v1.2.3+blue) naming which of two parallel environments a
// build is for; a channel is the pre-release identifier (v1.2.3-canary.1)
// naming how far a build has been rolled out.
type ChannelMatcher struct {
	Expected string
}

// Enabled reports whether the matcher narrows anything.
func (m ChannelMatcher) Enabled() bool {
	return m.Expected != ""
}

// Stable reports whether the matcher tracks final releases only.
func (m ChannelMatcher) Stable() bool {
	return strings.EqualFold(m.Expected, StableChannel)
}

// Matches reports whether a version with the given pre-release identifier
// belongs to the channel. Names are compared case-insensitively.
func (m ChannelMatcher) Matches(preRelease string) bool {
	if !m.Enabled() {
		return true
	}
	return strings.EqualFold(ChannelOf(preRelease), m.Expected)
}

// VersionFilter narrows the candidate versions considered when picking the
// latest one.
type VersionFilter struct {
	// Slot keeps only versions whose build metadata matches. Empty accepts
	// any build metadata.
	Slot string
	// Channel keeps only versions in that channel. Empty accepts any, subject
	// to PreRelease.
	Channel string
	// PreRelease admits pre-release versions when no channel is set. A
	// channel other than "stable" names pre-release versions by definition
	// and admits them on its own.
	PreRelease bool
}

// channel returns the matcher for this filter.
func (f VersionFilter) channel() ChannelMatcher {
	return ChannelMatcher{Expected: f.Channel}
}

// allowPreRelease reports whether pre-release versions are candidates at all.
func (f VersionFilter) allowPreRelease() bool {
	c := f.channel()
	if c.Enabled() {
		return !c.Stable()
	}
	return f.PreRelease
}

// accepts reports whether a version with the given pre-release identifier and
// build metadata passes the filter.
func (f VersionFilter) accepts(preRelease, buildMetadata string) bool {
	if !f.allowPreRelease() && preRelease != "" {
		return false
	}
	if !f.channel().Matches(preRelease) {
		return false
	}
	if f.Slot != "" && buildMetadata != f.Slot {
		return false
	}
	return true
}

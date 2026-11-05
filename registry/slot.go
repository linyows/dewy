package registry

import "fmt"

// SlotNotFoundError is returned when a slot is configured and no version
// carries that slot in its build metadata. Callers treat it as "nothing to
// deploy yet" rather than a failure: one slot of a blue/green pair commonly
// has no release while the other is being rolled out.
type SlotNotFoundError struct {
	Slot string
}

func (e *SlotNotFoundError) Error() string {
	return fmt.Sprintf("no version found for slot %q", e.Slot)
}

// SlotMatcher decides whether a registry response's slot satisfies a Dewy
// instance's configured slot constraint. An empty Expected matches anything;
// otherwise the actual slot must match exactly.
//
// Centralizing this check keeps blue/green slot semantics in one place even
// though the comparison itself is trivial.
type SlotMatcher struct {
	Expected string
}

// Matches reports whether actual satisfies the matcher.
func (m SlotMatcher) Matches(actual string) bool {
	return m.Expected == "" || m.Expected == actual
}

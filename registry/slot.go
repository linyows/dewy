package registry

// SlotMatcher decides whether a registry response's slot satisfies a Dewy
// instance's configured slot constraint. An empty Expected matches anything;
// otherwise the actual slot must match exactly.
//
// Centralising this check keeps blue/green slot semantics in one place even
// though the comparison itself is trivial.
type SlotMatcher struct {
	Expected string
}

// Matches reports whether actual satisfies the matcher.
func (m SlotMatcher) Matches(actual string) bool {
	return m.Expected == "" || m.Expected == actual
}

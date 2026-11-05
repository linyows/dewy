package registry

import (
	"errors"
	"testing"
)

func TestFindLatestWithSlotErrors(t *testing.T) {
	find := map[string]func(names []string, slot string) error{
		"semver": func(names []string, slot string) error {
			_, _, err := FindLatestSemVerWithSlot(names, slot, false)
			return err
		},
		"calver": func(names []string, slot string) error {
			_, _, err := FindLatestCalVerWithSlot(names, "YYYY.0M.MICRO", slot, false)
			return err
		},
	}
	names := map[string][]string{
		"semver": {"v1.0.0+blue", "v1.1.0+blue"},
		"calver": {"2024.06.0+blue", "2024.06.1+blue"},
	}

	for kind, fn := range find {
		t.Run(kind+": other slots only", func(t *testing.T) {
			err := fn(names[kind], "green")
			var slotErr *SlotNotFoundError
			if !errors.As(err, &slotErr) {
				t.Fatalf("expected SlotNotFoundError, got %v", err)
			}
			if slotErr.Slot != "green" {
				t.Errorf("slot = %s, want green", slotErr.Slot)
			}
		})
		t.Run(kind+": no versions", func(t *testing.T) {
			err := fn([]string{"README.md"}, "green")
			if err == nil {
				t.Fatal("expected an error")
			}
			var slotErr *SlotNotFoundError
			if errors.As(err, &slotErr) {
				t.Errorf("no versions must stay a plain error, got %v", err)
			}
		})
		t.Run(kind+": no slot configured", func(t *testing.T) {
			if err := fn(names[kind], ""); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestSlotMatcherMatches(t *testing.T) {
	tests := []struct {
		expected string
		actual   string
		want     bool
	}{
		{"", "", true},
		{"", "blue", true},
		{"blue", "blue", true},
		{"blue", "green", false},
		{"blue", "", false},
	}
	for _, tt := range tests {
		m := SlotMatcher{Expected: tt.expected}
		if got := m.Matches(tt.actual); got != tt.want {
			t.Errorf("SlotMatcher{%q}.Matches(%q) = %v, want %v", tt.expected, tt.actual, got, tt.want)
		}
	}
}

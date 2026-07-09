package project

import (
	"regexp"
	"strings"
	"testing"
)

// TestRandomWorktreeName proves the generator honors the exists predicate:
// free names come out as adjective-noun, and persistent collisions fall back
// to a numeric suffix instead of looping forever.
func TestRandomWorktreeName(t *testing.T) {
	pair := regexp.MustCompile(`^[a-z]+-[a-z]+$`)
	suffixed := regexp.MustCompile(`^[a-z]+-[a-z]+-\d+$`)

	tests := []struct {
		name   string
		exists func(string) bool
		want   *regexp.Regexp
	}{
		{"all free", func(string) bool { return false }, pair},
		{"all taken", func(n string) bool { return !strings.HasSuffix(n, "-2") }, suffixed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := randomWorktreeName(tt.exists)
			if !tt.want.MatchString(got) {
				t.Errorf("randomWorktreeName() = %q, want match %v", got, tt.want)
			}
			if tt.exists(got) {
				t.Errorf("randomWorktreeName() = %q, but exists(%q) is true", got, got)
			}
		})
	}
}

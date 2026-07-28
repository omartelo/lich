package shquote

import "testing"

// TestQuote proves quoting survives the shell round trip's edge cases — above
// all a single quote, which must not break out of the quoting, and the
// metacharacters a path or a binary name could otherwise smuggle in.
func TestQuote(t *testing.T) {
	tests := []struct{ in, want string }{
		{"plain", "'plain'"},
		{"with space", "'with space'"},
		{"it's", `'it'\''s'`},
		{"a'b.txt", `'a'\''b.txt'`},
		{"", "''"},
		{"$HOME `id` ;rm", "'$HOME `id` ;rm'"},
	}
	for _, tt := range tests {
		if got := Quote(tt.in); got != tt.want {
			t.Errorf("Quote(%q) = %s, want %s", tt.in, got, tt.want)
		}
	}
}

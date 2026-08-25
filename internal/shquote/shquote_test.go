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

// TestQuotePwsh proves the PowerShell rule against the same edge cases, plus
// the three cmd.exe could not express at all: a double quote, a percent-wrapped
// variable name and a trailing backslash all survive as literals here, which is
// why the Windows caller no longer needs a refusal path.
func TestQuotePwsh(t *testing.T) {
	tests := []struct{ in, want string }{
		{"plain", "'plain'"},
		{"with space", "'with space'"},
		{"it's", "'it''s'"},
		{`C:\Users\O'Brien\a.txt`, `'C:\Users\O''Brien\a.txt'`},
		{"", "''"},
		{"$HOME `id` ;rm", "'$HOME `id` ;rm'"},
		{`C:\repo\a"b.txt`, `'C:\repo\a"b.txt'`},
		{`C:\repo\a%PATH%.txt`, `'C:\repo\a%PATH%.txt'`},
		{`C:\repo\dir\`, `'C:\repo\dir\'`},
	}
	for _, tt := range tests {
		if got := QuotePwsh(tt.in); got != tt.want {
			t.Errorf("QuotePwsh(%q) = %s, want %s", tt.in, got, tt.want)
		}
	}
}

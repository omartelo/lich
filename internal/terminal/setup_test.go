package terminal

import (
	"strings"
	"testing"
)

// TestWrapSetup proves the wrap decision table: no script or Windows leaves
// the spec untouched, otherwise the spawn becomes sh -c with the script in a
// subshell and the original argv exec'd after it, quoted against spaces and
// embedded quotes.
func TestWrapSetup(t *testing.T) {
	base := ptySpec{
		bin:  "/opt/claude bin/claude",
		args: []string{"--resume", "it's-an-id"},
		dir:  "/wt",
		env:  []string{"HOME=/home/user"},
		cols: 80,
		rows: 24,
	}

	if got := wrapSetup(base, "", "linux"); got.bin != base.bin || len(got.args) != 2 {
		t.Errorf("empty script rewrote the spec: %+v", got)
	}
	if got := wrapSetup(base, "pnpm i", "windows"); got.bin != base.bin {
		t.Errorf("windows rewrote the spec: %+v", got)
	}

	got := wrapSetup(base, "pnpm i", "linux")
	if got.bin != "sh" || len(got.args) != 2 || got.args[0] != "-c" {
		t.Fatalf("wrapped spec = %+v, want sh -c", got)
	}
	cmd := got.args[1]
	for _, want := range []string{
		"pnpm i",
		"exec '/opt/claude bin/claude' '--resume' 'it'\\''s-an-id'",
		"[lich] worktree setup failed",
	} {
		if !strings.Contains(cmd, want) {
			t.Errorf("wrapped command missing %q:\n%s", want, cmd)
		}
	}
	if got.dir != base.dir || got.cols != base.cols || got.rows != base.rows {
		t.Errorf("wrap changed dir/size: %+v", got)
	}
}

// TestShQuote proves quoting survives the shell round trip's edge cases.
func TestShQuote(t *testing.T) {
	tests := []struct{ in, want string }{
		{"plain", "'plain'"},
		{"with space", "'with space'"},
		{"it's", `'it'\''s'`},
		{"", "''"},
		{"$HOME `id` ;rm", "'$HOME `id` ;rm'"},
	}
	for _, tt := range tests {
		if got := shQuote(tt.in); got != tt.want {
			t.Errorf("shQuote(%q) = %s, want %s", tt.in, got, tt.want)
		}
	}
}

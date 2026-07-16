package terminal

import (
	"slices"
	"testing"
)

// TestWrapArgv proves the Windows launch decision: npm's claude.cmd (and any
// .bat) gets the `cmd.exe /c` prefix CreateProcess needs — case-insensitively —
// while native executables and extensionless binaries pass through untouched.
// This is the pure slice of commandLine; the LookPath/ComposeCommandLine
// boundaries around it stay in pty_windows.go and run only under Windows CI.
func TestWrapArgv(t *testing.T) {
	cases := []struct {
		name string
		path string
		args []string
		want []string
	}{
		{"cmd script wraps", `C:\n\claude.cmd`, []string{"--resume", "x"},
			[]string{"cmd.exe", "/c", `C:\n\claude.cmd`, "--resume", "x"}},
		{"bat script wraps case-insensitively", `C:\n\run.BAT`, nil,
			[]string{"cmd.exe", "/c", `C:\n\run.BAT`}},
		{"exe passes through", `C:\n\claude.exe`, []string{"--resume", "x"},
			[]string{`C:\n\claude.exe`, "--resume", "x"}},
		{"extensionless passes through", "/usr/bin/claude", nil,
			[]string{"/usr/bin/claude"}},
	}
	for _, tc := range cases {
		if got := wrapArgv(tc.path, tc.args); !slices.Equal(got, tc.want) {
			t.Errorf("%s: wrapArgv(%q, %v) = %v, want %v",
				tc.name, tc.path, tc.args, got, tc.want)
		}
	}
}

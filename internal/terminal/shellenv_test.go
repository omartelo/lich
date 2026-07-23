package terminal

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

func TestParseShellEnvDump(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want []string
	}{
		{
			name: "no sentinel keeps launch env",
			out:  "FOO=bar\nBAZ=qux\n",
			want: nil,
		},
		{
			name: "chatter before sentinel is dropped",
			out:  "Welcome to zsh!\nbash: no job control in this shell\n" + shellEnvSentinel + "\nTOKEN=secret\nPATH=/x\n",
			want: []string{"TOKEN=secret", "PATH=/x"},
		},
		{
			name: "last sentinel wins when rc echoes it",
			out:  shellEnvSentinel + "\nSTALE=1\n" + shellEnvSentinel + "\nREAL=2\n",
			want: []string{"REAL=2"},
		},
		{
			name: "non-key lines are skipped",
			out:  shellEnvSentinel + "\nGOOD=1\n  continuation of a value\nALSO_GOOD=2\n",
			want: []string{"GOOD=1", "ALSO_GOOD=2"},
		},
		{
			name: "carriage returns are trimmed",
			out:  shellEnvSentinel + "\r\nWIN=1\r\n",
			want: []string{"WIN=1"},
		},
		{
			name: "empty value is preserved",
			out:  shellEnvSentinel + "\nEMPTY=\n",
			want: []string{"EMPTY="},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseShellEnvDump(shellEnvSentinel, tt.out)
			if !slices.Equal(got, tt.want) {
				t.Errorf("parseShellEnvDump() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidEnvKey(t *testing.T) {
	valid := []string{"PATH", "_", "_X", "A1", "MY_VAR_2"}
	invalid := []string{"", "1ABC", "MY-VAR", "MY VAR", "="}
	for _, k := range valid {
		if !validEnvKey(k) {
			t.Errorf("validEnvKey(%q) = false, want true", k)
		}
	}
	for _, k := range invalid {
		if validEnvKey(k) {
			t.Errorf("validEnvKey(%q) = true, want false", k)
		}
	}
}

func TestMergeEnv(t *testing.T) {
	base := []string{"PATH=/orig", "BASE_ONLY=1"}
	extra := []string{"PATH=/shell/bin", "TOKEN=secret"}
	got := mergeEnv(base, extra)
	want := []string{"PATH=/shell/bin", "BASE_ONLY=1", "TOKEN=secret"}
	if !slices.Equal(got, want) {
		t.Fatalf("mergeEnv() = %v, want %v", got, want)
	}
}

func TestMergeEnvBaseUntouchedWithoutExtra(t *testing.T) {
	base := []string{"A=1", "B=2"}
	if got := mergeEnv(base, nil); !slices.Equal(got, base) {
		t.Fatalf("mergeEnv(base, nil) = %v, want %v", got, base)
	}
}

// TestResolveShellEnv drives the real spawn through a fake $SHELL. It prints the
// dump lines directly instead of shelling out to `env`, whose resolution would
// depend on the very PATH the test rewrites; INHERITED echoes a base var back to
// prove cmd.Env reached the child.
func TestResolveShellEnv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake POSIX $SHELL script is not runnable on Windows")
	}
	dir := t.TempDir()
	fake := filepath.Join(dir, "fakeshell")
	script := "#!/bin/sh\n" +
		"echo " + shellEnvSentinel + "\n" +
		"echo FROM_SHELL=yes\n" +
		"echo PATH=/shell/bin\n" +
		"echo INHERITED=$LAUNCH_ONLY\n"
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHELL", fake)

	got := ResolveShellEnv([]string{"LAUNCH_ONLY=1", "PATH=/orig"})

	if !slices.Contains(got, "FROM_SHELL=yes") {
		t.Errorf("shell-exported var missing: %v", got)
	}
	if !slices.Contains(got, "INHERITED=1") {
		t.Errorf("launch env not passed to child shell: %v", got)
	}
	if !slices.Contains(got, "LAUNCH_ONLY=1") {
		t.Errorf("launch-only var dropped: %v", got)
	}
	if !slices.Contains(got, "PATH=/shell/bin") || slices.Contains(got, "PATH=/orig") {
		t.Errorf("shell PATH did not override launch PATH: %v", got)
	}
}

func TestResolveShellEnvNoShell(t *testing.T) {
	t.Setenv("SHELL", "")
	base := []string{"A=1"}
	if got := ResolveShellEnv(base); !slices.Equal(got, base) {
		t.Fatalf("ResolveShellEnv without SHELL = %v, want %v", got, base)
	}
}

func TestResolveShellEnvBrokenShell(t *testing.T) {
	t.Setenv("SHELL", filepath.Join(t.TempDir(), "does-not-exist"))
	base := []string{"A=1"}
	if got := ResolveShellEnv(base); !slices.Equal(got, base) {
		t.Fatalf("ResolveShellEnv with broken shell = %v, want %v", got, base)
	}
}

package providers

import (
	"io/fs"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

// absolute builds a path the running OS agrees is absolute. A POSIX literal is
// not one on Windows: "/opt/claude" there names a location on whatever drive the
// process sits on, which is the same per-session ambiguity a relative path has —
// and which Verify reports as CheckRelative. So the two forms are spelled out
// rather than assumed to be one.
func absolute(parts ...string) string {
	if runtime.GOOS == "windows" {
		return `C:\` + strings.Join(parts, `\`)
	}
	return "/" + strings.Join(parts, "/")
}

// verifier resolves the given names and fails every other lookup the way
// exec.LookPath does, so Verify is driven without touching the machine.
func verifier(found map[string]string, denied map[string]bool) *Service {
	return &Service{
		lookPath: func(name string) (string, error) {
			if denied[name] {
				return "", &exec.Error{Name: name, Err: fs.ErrPermission}
			}
			if path, ok := found[name]; ok {
				return path, nil
			}
			return "", &exec.Error{Name: name, Err: exec.ErrNotFound}
		},
	}
}

func TestVerifyResolvesLikeTheSpawn(t *testing.T) {
	onPath := absolute("home", "dev", ".local", "bin", "claude")
	installed := absolute("opt", "claude", "claude")
	svc := verifier(map[string]string{
		"claude":  onPath,
		installed: installed,
	}, nil)

	// A bare name is a $PATH lookup, and the answer is the resolved path.
	if got := svc.Verify("claude"); got.Status != CheckOK || got.Path != onPath {
		t.Errorf("Verify(bare name) = %+v, want ok at %s", got, onPath)
	}
	// An absolute path is taken as written.
	if got := svc.Verify(installed); got.Status != CheckOK || got.Path != installed {
		t.Errorf("Verify(absolute) = %+v, want ok", got)
	}
	// Surrounding whitespace is a paste artefact, not part of the path.
	if got := svc.Verify("  claude \n"); got.Status != CheckOK {
		t.Errorf("Verify(padded) = %+v, want ok", got)
	}
}

func TestVerifyNamesEachSilentMistake(t *testing.T) {
	gone := absolute("opt", "gone", "claude")
	notExecutable := absolute("tmp", "claude.sh")
	directory := absolute("etc")
	svc := verifier(
		map[string]string{"claude": absolute("usr", "bin", "claude")},
		map[string]bool{directory: true, notExecutable: true},
	)
	tests := []struct {
		name string
		bin  string
		want string
	}{
		{name: "nothing configured", bin: "", want: CheckEmpty},
		{name: "only whitespace", bin: "   ", want: CheckEmpty},
		{name: "not installed", bin: "codex", want: CheckNotFound},
		{name: "absolute path that is not there", bin: gone, want: CheckNotFound},
		{name: "a file without the execute bit", bin: notExecutable, want: CheckNotExecutable},
		{name: "a directory", bin: directory, want: CheckNotExecutable},
		// The spawn is exec, not a shell: nothing expands these two, and both
		// fail long after this screen accepted them.
		{name: "a home shortcut", bin: "~/dev/claude", want: CheckHomeShortcut},
		{name: "bare tilde", bin: "~", want: CheckHomeShortcut},
		{name: "a relative path", bin: "bin/claude", want: CheckRelative},
		{name: "an explicitly relative path", bin: "./claude", want: CheckRelative},
		{name: "a parent-relative path", bin: "../claude", want: CheckRelative},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.Verify(tt.bin)
			if got.Status != tt.want {
				t.Errorf("Verify(%q) = %q, want %q", tt.bin, got.Status, tt.want)
			}
			if got.Path != "" {
				t.Errorf("Verify(%q) reported path %q for a failing check", tt.bin, got.Path)
			}
		})
	}
}

// The relative check must not swallow a bare command name: that is a $PATH
// lookup, which resolves the same for every session and has to be verified.
func TestIsRelativePath(t *testing.T) {
	// True everywhere: a bare command name is a $PATH lookup, and the three
	// relative forms name a location without rooting it.
	tests := map[string]bool{
		"claude":     false,
		"claude-2.1": false,
		"bin/claude": true,
		"./claude":   true,
		"../claude":  true,
	}
	for bin, want := range tests {
		if got := isRelativePath(bin); got != want {
			t.Errorf("isRelativePath(%q) = %v, want %v", bin, got, want)
		}
	}

	// What counts as rooted is the one thing that is not portable. On Windows a
	// path is absolute only with its volume: the leading-slash form resolves
	// against whatever drive the process is on, which is the same per-session
	// ambiguity a relative path has, so it is reported as one.
	if runtime.GOOS == "windows" {
		if isRelativePath(`C:\tools\claude.exe`) {
			t.Error(`isRelativePath("C:\tools\claude.exe") = true, want false`)
		}
		for _, bin := range []string{`\tools\claude.exe`, "/usr/bin/claude"} {
			if !isRelativePath(bin) {
				t.Errorf("isRelativePath(%q) = false, want true on Windows", bin)
			}
		}
		return
	}
	if isRelativePath("/usr/bin/claude") {
		t.Error(`isRelativePath("/usr/bin/claude") = true, want false`)
	}
}

package providers

import (
	"io/fs"
	"os/exec"
	"path/filepath"
	"testing"
)

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
	svc := verifier(map[string]string{
		"claude":             "/home/dev/.local/bin/claude",
		"/opt/claude/claude": "/opt/claude/claude",
	}, nil)

	// A bare name is a $PATH lookup, and the answer is the resolved path.
	if got := svc.Verify("claude"); got.Status != CheckOK || got.Path != "/home/dev/.local/bin/claude" {
		t.Errorf("Verify(bare name) = %+v, want ok at the resolved path", got)
	}
	// An absolute path is taken as written.
	if got := svc.Verify("/opt/claude/claude"); got.Status != CheckOK || got.Path != "/opt/claude/claude" {
		t.Errorf("Verify(absolute) = %+v, want ok", got)
	}
	// Surrounding whitespace is a paste artefact, not part of the path.
	if got := svc.Verify("  claude \n"); got.Status != CheckOK {
		t.Errorf("Verify(padded) = %+v, want ok", got)
	}
}

func TestVerifyNamesEachSilentMistake(t *testing.T) {
	svc := verifier(
		map[string]string{"claude": "/usr/bin/claude"},
		map[string]bool{"/etc": true, "/tmp/claude.sh": true},
	)
	tests := []struct {
		name string
		bin  string
		want string
	}{
		{name: "nothing configured", bin: "", want: CheckEmpty},
		{name: "only whitespace", bin: "   ", want: CheckEmpty},
		{name: "not installed", bin: "codex", want: CheckNotFound},
		{name: "absolute path that is not there", bin: "/opt/gone/claude", want: CheckNotFound},
		{name: "a file without the execute bit", bin: "/tmp/claude.sh", want: CheckNotExecutable},
		{name: "a directory", bin: "/etc", want: CheckNotExecutable},
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
	tests := map[string]bool{
		"claude":          false,
		"claude-2.1":      false,
		"/usr/bin/claude": false,
		"bin/claude":      true,
		"./claude":        true,
		"../claude":       true,
	}
	for bin, want := range tests {
		if got := isRelativePath(bin); got != want {
			t.Errorf("isRelativePath(%q) = %v, want %v", bin, got, want)
		}
	}
	// A Windows path is absolute only with its volume; the rooted form depends
	// on the current drive, which is the same ambiguity a relative path has.
	if filepath.Separator == '\\' {
		if !isRelativePath(`\tools\claude.exe`) {
			t.Error(`isRelativePath("\tools\claude.exe") = false, want true on Windows`)
		}
		if isRelativePath(`C:\tools\claude.exe`) {
			t.Error(`isRelativePath("C:\tools\claude.exe") = true, want false`)
		}
	}
}

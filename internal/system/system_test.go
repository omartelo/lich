package system

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateExternalURL(t *testing.T) {
	valid := []string{"http://example.com", "https://github.com/omartelo/lich/pull/1"}
	for _, u := range valid {
		if err := ValidateExternalURL(u); err != nil {
			t.Fatalf("want %q accepted: %v", u, err)
		}
	}
	invalid := []string{
		"file:///etc/passwd",
		"javascript:alert(1)",
		"vscode://open",
		"https://",
		"not a url at all\x00",
		"",
	}
	for _, u := range invalid {
		if err := ValidateExternalURL(u); err == nil {
			t.Fatalf("want %q rejected", u)
		}
	}
}

func TestOpenExternalGatesBeforeLaunching(t *testing.T) {
	launched := ""
	s := &Service{run: func(_ string, args ...string) error {
		if len(args) > 0 {
			launched = args[0]
		}
		return nil
	}}
	if err := s.OpenExternal("file:///etc/passwd"); err == nil || launched != "" {
		t.Fatalf("invalid url launched: %q (%v)", launched, err)
	}
	if err := s.OpenExternal("https://example.com"); err != nil || launched != "https://example.com" {
		t.Fatalf("valid url not launched: %q (%v)", launched, err)
	}
}

// captureRun records the last command the service launched.
func captureRun(name *string, args *[]string) func(string, ...string) error {
	return func(n string, a ...string) error {
		*name, *args = n, a
		return nil
	}
}

func TestOpenInEditorTerminalReturnsCommand(t *testing.T) {
	launched := false
	s := &Service{env: []string{"EDITOR=nvim"}, run: func(string, ...string) error {
		launched = true
		return nil
	}}

	cmd, err := s.OpenInEditor("/repo", "src/main.go")
	if err != nil {
		t.Fatalf("OpenInEditor: %v", err)
	}
	if want := "nvim '" + filepath.Join("/repo", "src/main.go") + "'"; cmd != want {
		t.Errorf("cmd = %q, want %q", cmd, want)
	}
	if launched {
		t.Error("a terminal editor was launched detached instead of returned")
	}
}

func TestOpenInEditorPrefersVisual(t *testing.T) {
	// VISUAL wins over EDITOR: a terminal EDITOR is ignored when VISUAL is GUI.
	var name string
	var args []string
	s := &Service{env: []string{"EDITOR=nvim", "VISUAL=zed"}, run: captureRun(&name, &args)}

	cmd, err := s.OpenInEditor("/repo", "a.txt")
	if err != nil {
		t.Fatalf("OpenInEditor: %v", err)
	}
	if cmd != "" {
		t.Errorf("cmd = %q, want empty (GUI editor launched)", cmd)
	}
	if want := filepath.Join("/repo", "a.txt"); name != "zed" || len(args) != 1 || args[0] != want {
		t.Errorf("launched %q %v, want zed [%s]", name, args, want)
	}
}

func TestOpenInEditorGUISplitsFlags(t *testing.T) {
	var name string
	var args []string
	s := &Service{env: []string{"EDITOR=code --wait"}, run: captureRun(&name, &args)}

	if _, err := s.OpenInEditor("/repo", "a.txt"); err != nil {
		t.Fatalf("OpenInEditor: %v", err)
	}
	want := filepath.Join("/repo", "a.txt")
	if name != "code" || len(args) != 2 || args[0] != "--wait" || args[1] != want {
		t.Errorf("got %q %v, want code [--wait %s]", name, args, want)
	}
}

func TestOpenInEditorRejectsTraversal(t *testing.T) {
	launched := false
	s := &Service{env: []string{"EDITOR=nvim"}, run: func(string, ...string) error {
		launched = true
		return nil
	}}
	for _, rel := range []string{"../escape", "/etc/passwd", "a/../../b"} {
		if cmd, err := s.OpenInEditor("/repo", rel); err == nil || cmd != "" {
			t.Errorf("rel %q: want error, got cmd %q err %v", rel, cmd, err)
		}
	}
	if launched {
		t.Error("a traversal path reached the launcher")
	}
}

// TestOpenInEditorFallsBackToDefault proves that with no editor set the file is
// handed to the default opener (launched, empty command). The opener command is
// per-OS, so only the file argument (always last) is portable to assert.
func TestOpenInEditorFallsBackToDefault(t *testing.T) {
	var name string
	var args []string
	s := &Service{run: captureRun(&name, &args)}

	cmd, err := s.OpenInEditor("/repo", "a.txt")
	if err != nil {
		t.Fatalf("OpenInEditor: %v", err)
	}
	if cmd != "" {
		t.Errorf("cmd = %q, want empty (default opener launched)", cmd)
	}
	if want := filepath.Join("/repo", "a.txt"); len(args) == 0 || args[len(args)-1] != want {
		t.Errorf("args = %v, want file %s as last arg", args, want)
	}
}

// TestValidateRelRejectsRootedPaths pins the guard itself: a path that starts
// at a root is never a work-tree path. "/etc/passwd" is the case Windows used to
// wave through — filepath.IsAbs wants a volume name there, so a leading
// separator alone read as relative.
func TestValidateRelRejectsRootedPaths(t *testing.T) {
	rooted := []string{"/etc/passwd", "/"}
	if os.IsPathSeparator('\\') {
		rooted = append(rooted, `\Windows\System32\config`, `C:\Windows`)
	}
	for _, rel := range rooted {
		if err := validateRel(rel); err == nil {
			t.Errorf("validateRel(%q): want error, got nil", rel)
		}
	}
	for _, rel := range []string{"a.txt", "src/main.go", "a/b/../c.txt"} {
		if err := validateRel(rel); err != nil {
			t.Errorf("validateRel(%q): want nil, got %v", rel, err)
		}
	}
}

func TestShellQuoteEscapesSingleQuote(t *testing.T) {
	// A path with a single quote must not break out of the quoting.
	if got, want := shellQuote("a'b.txt"), `'a'\''b.txt'`; got != want {
		t.Errorf("shellQuote = %q, want %q", got, want)
	}
}

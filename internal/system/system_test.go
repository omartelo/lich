package system

import (
	"path/filepath"
	"runtime"
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

// TestOpenExternalGatesBeforeLaunching proves the scheme gate runs before
// anything is launched, and that an accepted URL reaches the launcher. The
// opener command is per-OS, so only the URL (always the last argument) is
// portable to assert.
func TestOpenExternalGatesBeforeLaunching(t *testing.T) {
	launched := ""
	s := &Service{run: func(_ string, args ...string) error {
		if len(args) > 0 {
			launched = args[len(args)-1]
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

// TestOpenExternalPassesURLAsOneArgument proves the URL is handed to the
// launcher whole, as a single argument. On Windows that is what keeps a `&` in
// a query string out of a shell's hands — the opener there must not route
// through cmd, whose parser would end the command at the ampersand.
func TestOpenExternalPassesURLAsOneArgument(t *testing.T) {
	const raw = "https://example.com/x?a=1&b=2"
	var name string
	var args []string
	s := &Service{run: captureRun(&name, &args)}

	if err := s.OpenExternal(raw); err != nil {
		t.Fatalf("OpenExternal: %v", err)
	}
	if name == "cmd" {
		t.Errorf("launcher is %q: a URL must not go through the shell", name)
	}
	if len(args) == 0 || args[len(args)-1] != raw {
		t.Errorf("args = %v, want the whole url %q as the last argument", args, raw)
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
	// The quoting belongs to the session's shell: sh everywhere, cmd.exe on
	// Windows, and the two spell a quoted argument differently.
	full := filepath.Join("/repo", "src/main.go")
	want := "nvim '" + full + "'"
	if runtime.GOOS == "windows" {
		want = `nvim "` + full + `"`
	}
	if cmd != want {
		t.Errorf("cmd = %q, want %q", cmd, want)
	}
	if launched {
		t.Error("a terminal editor was launched detached instead of returned")
	}
}

// TestQuoteCmdPath proves the cmd.exe rule: the separators a shell would act on
// are covered by the quotes, and the three characters cmd cannot express are
// refused outright rather than handed over half-escaped.
func TestQuoteCmdPath(t *testing.T) {
	quoted := []string{
		`C:\repo\a.txt`,
		`C:\Users\First Last\repo\a.txt`,
		`C:\repo\a&calc.txt`,
		`C:\repo\a|b>c.txt`,
		`C:\repo\a^b(c).txt`,
	}
	for _, full := range quoted {
		got, ok := quoteCmdPath(full)
		if !ok || got != `"`+full+`"` {
			t.Errorf("quoteCmdPath(%q) = (%q, %v), want the path in quotes", full, got, ok)
		}
	}

	refused := []string{
		`C:\repo\a"b.txt`,     // no way to put a quote back inside a quoted run
		`C:\repo\a%PATH%.txt`, // expanded even inside quotes
		`C:\repo\dir\`,        // would escape the closing quote
	}
	for _, full := range refused {
		if got, ok := quoteCmdPath(full); ok {
			t.Errorf("quoteCmdPath(%q) = (%q, true), want refused", full, got)
		}
	}
}

// TestOpenInEditorFallsBackWhenTheShellCannotQuote proves an unquotable path
// opens with the default handler instead of being pasted into a shell. Only
// cmd.exe refuses anything, so only Windows has the branch to exercise.
func TestOpenInEditorFallsBackWhenTheShellCannotQuote(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("every path is expressible in a POSIX shell")
	}
	var name string
	var args []string
	s := &Service{env: []string{"EDITOR=nvim"}, run: captureRun(&name, &args)}

	cmd, err := s.OpenInEditor(`C:\repo`, `a"b.txt`)
	if err != nil {
		t.Fatalf("OpenInEditor: %v", err)
	}
	if cmd != "" {
		t.Errorf("cmd = %q, want empty: an unquotable path must not reach a shell", cmd)
	}
	if want := filepath.Join(`C:\repo`, `a"b.txt`); len(args) == 0 || args[len(args)-1] != want {
		t.Errorf("args = %v, want the file %s handed to the default opener", args, want)
	}
}

// TestOpenDefaultKeepsThePathOutOfAShell proves the default opener passes the
// file as its own argument. On Windows that is what stops a checked-out file
// named "a&calc.txt" from ending the command and running calc.
func TestOpenDefaultKeepsThePathOutOfAShell(t *testing.T) {
	const rel = "a&calc.txt"
	var name string
	var args []string
	s := &Service{run: captureRun(&name, &args)}

	if _, err := s.OpenInEditor("/repo", rel); err != nil {
		t.Fatalf("OpenInEditor: %v", err)
	}
	if name == "cmd" || name == "sh" || name == "bash" {
		t.Errorf("launcher is %q: the default opener must not route through a shell", name)
	}
	if want := filepath.Join("/repo", rel); len(args) == 0 || args[len(args)-1] != want {
		t.Errorf("args = %v, want the whole path %q as one argument", args, want)
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

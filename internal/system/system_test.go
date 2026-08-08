package system

import (
	"path/filepath"
	"runtime"
	"strings"
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

// TestRevealLogOpensTheDirectory proves the reveal hands the log's *directory*
// to the opener, so the rotated generation is in view beside the live file.
func TestRevealLogOpensTheDirectory(t *testing.T) {
	var name string
	var args []string
	dir := filepath.Join("/config", "lich")
	s := &Service{logPath: filepath.Join(dir, "lich.log"), run: captureRun(&name, &args)}

	if err := s.RevealLog(); err != nil {
		t.Fatalf("RevealLog: %v", err)
	}
	if name == "cmd" || name == "sh" || name == "bash" {
		t.Errorf("launcher is %q: the path must not route through a shell", name)
	}
	if len(args) == 0 || args[len(args)-1] != dir {
		t.Errorf("args = %v, want the directory %q as the last argument", args, dir)
	}
}

// TestRevealLogWithoutAFileLaunchesNothing proves that a run with no log file
// (Init fell back to stderr) reports it instead of opening filepath.Dir("") —
// the working directory, which has nothing to do with the report.
func TestRevealLogWithoutAFileLaunchesNothing(t *testing.T) {
	launched := false
	s := &Service{run: func(string, ...string) error {
		launched = true
		return nil
	}}
	if err := s.RevealLog(); err == nil {
		t.Error("want an error when there is no log file")
	}
	if launched {
		t.Error("something was opened for a log file that does not exist")
	}
}

func TestDiagnosticsReportsTheRunningBuild(t *testing.T) {
	s := New(nil, "/config/lich/lich.log", "0.23.0")
	got := s.Diagnostics()
	if got.Version != "0.23.0" || got.LogPath != "/config/lich/lich.log" {
		t.Errorf("Diagnostics() = %+v, want the version and log path it was built with", got)
	}
	if want := runtime.GOOS + "/" + runtime.GOARCH; got.Platform != want {
		t.Errorf("Platform = %q, want %q", got.Platform, want)
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

// captureNotify records the one notification a test raises.
func captureNotify(text, title *string) func(string, string) error {
	return func(gotText, gotTitle string) error {
		*text, *title = gotText, gotTitle
		return nil
	}
}

// TestNotifyBuildsTwoLineText proves the detail becomes the text's second line
// rather than a separate field: Linux takes the first line as the summary, so
// the split has to live in the text itself.
func TestNotifyBuildsTwoLineText(t *testing.T) {
	var text, title string
	s := &Service{notify: captureNotify(&text, &title)}

	if err := s.Notify("api needs your input", "lich"); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if want := "api needs your input\nlich"; text != want {
		t.Errorf("text = %q, want %q", text, want)
	}
	if title != "lich" {
		t.Errorf("title = %q, want lich", title)
	}

	if err := s.Notify("api needs your input", ""); err != nil {
		t.Fatalf("Notify without detail: %v", err)
	}
	if strings.Contains(text, "\n") {
		t.Errorf("text = %q, want no second line when detail is empty", text)
	}
}

// TestNotifyTruncatesEachLine pins the 120-rune cap on both lines, counted in
// runes: a byte cap would split a multi-byte character and hand the notifier
// invalid UTF-8.
func TestNotifyTruncatesEachLine(t *testing.T) {
	var text, title string
	s := &Service{notify: captureNotify(&text, &title)}

	exact := strings.Repeat("é", 120)
	if err := s.Notify(exact, exact); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if text != exact+"\n"+exact {
		t.Errorf("120 runes were altered: %q", text)
	}

	over := strings.Repeat("é", 121)
	if err := s.Notify(over, over); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	want := exact + "…\n" + exact + "…"
	if text != want {
		t.Errorf("text = %q, want each line cut to 120 runes plus an ellipsis", text)
	}
}

// TestNotifyRejectsEmpty proves a blank notification never reaches the desktop:
// a card with no label must not raise an empty popup.
func TestNotifyRejectsEmpty(t *testing.T) {
	raised := false
	s := &Service{notify: func(string, string) error {
		raised = true
		return nil
	}}
	for _, summary := range []string{"", "   ", "\n"} {
		if err := s.Notify(summary, ""); err == nil {
			t.Errorf("summary %q: want error", summary)
		}
	}
	if raised {
		t.Error("an empty notification reached the desktop")
	}
}

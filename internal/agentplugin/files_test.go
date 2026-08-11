package agentplugin

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/omartelo/lich/internal/providers"
)

// The harnesses lich installs by writing files rather than by calling a CLI.
// What is asserted here is what the harness will read back: the bytes the
// release published, the marker that names the version, and — for Crush — that
// every line the user wrote survives the write.

const testVersion = "0.8.0"

// fileServer serves the plugin repository's files at a tag and records the
// paths asked for. A path with no body 404s, which is how a test says "this
// release does not carry that file".
func fileServer(t *testing.T, files map[string]string) (*Service, func() []string) {
	t.Helper()
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Path)
		body, ok := files[r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)

	release := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"tag_name":"v`+testVersion+`"}`)
	}))
	t.Cleanup(release.Close)

	return &Service{
		http:      srv.Client(),
		latestURL: release.URL,
		rawBase:   srv.URL,
		bins:      stubBins{},
		lookPath:  func(name string) (string, error) { return "/usr/bin/" + name, nil },
	}, func() []string { return asked }
}

// tagged is the URL path a release file is served at.
func tagged(path string) string { return "/v" + testVersion + "/" + path }

// ---------------------------------------------------------------- opencode --

func TestOpencodeInstallWritesTheModule(t *testing.T) {
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	s, asked := fileServer(t, map[string]string{
		tagged(opencodeSource): "export const LichPlugin = async () => ({})\n",
	})

	if err := s.Install(providers.OpenCode); err != nil {
		t.Fatalf("Install: %v", err)
	}

	path := filepath.Join(config, "opencode", "plugin", opencodeFile)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the installed module: %v", err)
	}
	body := string(data)
	if !strings.Contains(body, "export const LichPlugin") {
		t.Errorf("installed module lost the release's body:\n%s", body)
	}
	if !strings.HasPrefix(body, jsComment+" "+markerName+" v"+testVersion+" ") {
		t.Errorf("installed module has no version marker on its first line:\n%s", body)
	}
	if got := asked(); len(got) != 1 || got[0] != tagged(opencodeSource) {
		t.Errorf("fetched %v, want just %q", got, tagged(opencodeSource))
	}
}

// The version has to survive the round trip: Status reads back what Install
// wrote, and a marker either side spells differently reports nothing installed.
func TestOpencodeInstalledVersionRoundTrips(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	s, _ := fileServer(t, map[string]string{tagged(opencodeSource): "export const P = async () => ({})\n"})

	if err := s.Install(providers.OpenCode); err != nil {
		t.Fatalf("Install: %v", err)
	}
	got, ok := s.installedVersion(providers.OpenCode)
	if !ok || got != testVersion {
		t.Fatalf("installedVersion = (%q,%v), want (%q,true)", got, ok, testVersion)
	}
}

// A module somebody installed by hand carries no marker. Reporting a version
// for it would offer an update over a file lich never wrote.
func TestOpencodeInstalledVersionIgnoresAnUnmarkedModule(t *testing.T) {
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	dir := filepath.Join(config, "opencode", "plugin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, opencodeFile), []byte("export const P = async () => ({})\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, _ := fileServer(t, nil)

	if got, ok := s.installedVersion(providers.OpenCode); ok || got != "" {
		t.Fatalf("installedVersion = (%q,%v), want empty/false for a module lich did not write", got, ok)
	}
}

func TestOpencodeInstalledVersionMissingModule(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	s, _ := fileServer(t, nil)
	if got, ok := s.installedVersion(providers.OpenCode); ok || got != "" {
		t.Fatalf("installedVersion = (%q,%v), want empty/false", got, ok)
	}
}

// An update replaces the module wholesale, so the older version must not
// survive anywhere in the file — Status would otherwise read the first marker
// it finds and report the version the user just left.
func TestOpencodeUpdateReplacesTheModule(t *testing.T) {
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	s, _ := fileServer(t, map[string]string{tagged(opencodeSource): "export const P = async () => ({ event: async () => {} })\n"})

	dir := filepath.Join(config, "opencode", "plugin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	old := jsComment + " " + markerName + " v0.1.0 — installed by lich\nexport const P = async () => ({})\n"
	if err := os.WriteFile(filepath.Join(dir, opencodeFile), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := s.Update(providers.OpenCode); err != nil {
		t.Fatalf("Update: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, opencodeFile))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "v0.1.0") {
		t.Errorf("update left the old version behind:\n%s", data)
	}
	if got, ok := s.installedVersion(providers.OpenCode); !ok || got != testVersion {
		t.Errorf("installedVersion = (%q,%v), want (%q,true)", got, ok, testVersion)
	}
}

// A release that does not carry the module must leave nothing behind: a
// half-written plugin directory is worse than an install the user can retry.
func TestOpencodeInstallWritesNothingWhenTheFetchFails(t *testing.T) {
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	s, _ := fileServer(t, nil) // every path 404s

	if err := s.Install(providers.OpenCode); err == nil {
		t.Fatal("Install: want an error when the release has no module, got nil")
	}
	if _, err := os.Stat(filepath.Join(config, "opencode", "plugin", opencodeFile)); !os.IsNotExist(err) {
		t.Errorf("a failed install left a module behind (stat err = %v)", err)
	}
}

// ------------------------------------------------------------------- crush --

// crushCLI points a Service at the fake CLI, answering `crush dirs` with
// configDir and `crush --version` with version.
func crushCLI(t *testing.T, s *Service, configDir, version string) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve test binary: %v", err)
	}
	t.Setenv(fakeCLIGuard, "1")
	t.Setenv(fakeCLIDirs, configDir)
	t.Setenv(fakeCLIVersion, version)
	s.bins = stubBin(self)
}

// lichConfigHome redirects os.UserConfigDir — where lich keeps the scripts it
// installs for Crush — into a temporary directory, and returns what it will
// answer.
//
// Setting XDG_CONFIG_HOME alone is a Linux-only isolation: os.UserConfigDir
// reads it there, but $HOME/Library/Application Support on macOS and %AppData%
// on Windows. A test that sets only XDG therefore looks for the scripts in its
// temporary directory while the install writes them into the real user's config
// directory — failing on those two, and leaving files behind on a machine it was
// never supposed to touch. All three variables are set so the redirect holds
// wherever the suite runs, and the answer is read back from the standard library
// rather than rebuilt here, because the per-OS layout is its rule, not lich's.
func lichConfigHome(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	t.Setenv("AppData", root)
	t.Setenv("HOME", root)
	dir, err := os.UserConfigDir()
	if err != nil {
		t.Fatalf("resolve config directory: %v", err)
	}
	return dir
}

// installCrush runs an install against temporary directories and returns the
// crushrc it wrote and the directory the scripts went to.
func installCrush(t *testing.T, s *Service, existingRC string) (string, string) {
	t.Helper()
	configDir := t.TempDir()
	lichConfig := lichConfigHome(t)
	crushCLI(t, s, configDir, "0.88.1")

	rc := filepath.Join(configDir, "crushrc")
	if existingRC != "" {
		if err := os.WriteFile(rc, []byte(existingRC), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Install(providers.Crush); err != nil {
		t.Fatalf("Install: %v", err)
	}
	data, err := os.ReadFile(rc)
	if err != nil {
		t.Fatalf("read crushrc: %v", err)
	}
	return string(data), filepath.Join(lichConfig, "lich", "plugin", "hooks")
}

func crushFiles() map[string]string {
	files := map[string]string{}
	for _, hook := range crushHooks {
		files[tagged(hook.script)] = "#!/bin/sh\n# " + hook.name + "\nexit 0\n"
	}
	return files
}

func TestCrushInstallWritesScriptsAndHooks(t *testing.T) {
	s, _ := fileServer(t, crushFiles())
	rc, scriptDir := installCrush(t, s, "")

	for _, hook := range crushHooks {
		path := filepath.Join(scriptDir, filepath.Base(hook.script))
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		// Crush dispatches a shebang'd script through os/exec, which needs the bit.
		// Windows has no POSIX mode to carry it — a file written 0o755 stats as
		// -rw-rw-rw- there — so the bit is asserted where it decides anything.
		if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
			t.Errorf("%s is not executable (mode %v)", path, info.Mode().Perm())
		}
		if !strings.Contains(rc, hook.name) {
			t.Errorf("crushrc does not register %q:\n%s", hook.name, rc)
		}
		if !strings.Contains(rc, filepath.Base(hook.script)) {
			t.Errorf("crushrc does not name %q:\n%s", hook.script, rc)
		}
	}
	if strings.Count(rc, "hook add "+crushHookEvent) != len(crushHooks) {
		t.Errorf("crushrc has the wrong number of hook lines:\n%s", rc)
	}
	// Only the tools that write: a git-status refresh per read costs more than
	// the poll it front-runs.
	if !strings.Contains(rc, crushWriteTools) {
		t.Errorf("crushrc registers the touched hook without its matcher:\n%s", rc)
	}
}

// The block carries the provider argument through, which is what puts Crush's
// icon on the card rather than Claude's.
func TestCrushInstallNamesTheProvider(t *testing.T) {
	s, _ := fileServer(t, crushFiles())
	rc, _ := installCrush(t, s, "")

	command, ok := commandOf(rc, "lich-session-id")
	if !ok {
		t.Fatalf("no session-start hook in crushrc:\n%s", rc)
	}
	if !strings.HasSuffix(command, " "+providers.Crush) {
		t.Errorf("session-start command does not end in the provider: %q", command)
	}
}

// The whole reason for crushrc over crush.json: what the user wrote stays.
func TestCrushInstallKeepsTheUsersLines(t *testing.T) {
	existing := "# my own notes\nprovider anthropic --api-key $ANTHROPIC_API_KEY\n"
	s, _ := fileServer(t, crushFiles())
	rc, _ := installCrush(t, s, existing)

	if !strings.HasPrefix(rc, existing) {
		t.Errorf("install did not keep the user's lines at the top:\n%s", rc)
	}
}

func TestCrushInstalledVersionRoundTrips(t *testing.T) {
	s, _ := fileServer(t, crushFiles())
	installCrush(t, s, "")

	got, ok := s.installedVersion(providers.Crush)
	if !ok || got != testVersion {
		t.Fatalf("installedVersion = (%q,%v), want (%q,true)", got, ok, testVersion)
	}
}

func TestCrushInstalledVersionWithoutABlock(t *testing.T) {
	s, _ := fileServer(t, nil)
	configDir := t.TempDir()
	crushCLI(t, s, configDir, "0.88.1")
	if err := os.WriteFile(filepath.Join(configDir, "crushrc"), []byte("# just the user's own\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, ok := s.installedVersion(providers.Crush); ok || got != "" {
		t.Fatalf("installedVersion = (%q,%v), want empty/false", got, ok)
	}
}

// A Crush too old reads the file and ignores the lines, so the install would
// look done and report nothing forever. Nothing may be written.
func TestCrushInstallRefusesAnOldCrush(t *testing.T) {
	s, asked := fileServer(t, crushFiles())
	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	crushCLI(t, s, configDir, "0.77.0")

	err := s.Install(providers.Crush)
	if err == nil {
		t.Fatal("Install: want an error on a Crush without crushrc hooks, got nil")
	}
	for _, want := range []string{"0.77.0", crushMinVersion} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
	if _, statErr := os.Stat(filepath.Join(configDir, "crushrc")); !os.IsNotExist(statErr) {
		t.Errorf("a refused install wrote a crushrc anyway (stat err = %v)", statErr)
	}
	if got := asked(); len(got) > 0 {
		t.Errorf("a refused install fetched %v, want nothing", got)
	}
}

// A version lich cannot read is not a version it may refuse on: guessing would
// block an install that would have worked.
func TestCrushInstallProceedsOnAnUnreadableVersion(t *testing.T) {
	s, _ := fileServer(t, crushFiles())
	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	crushCLI(t, s, configDir, "")

	if err := s.Install(providers.Crush); err != nil {
		t.Fatalf("Install with an unreadable crush version: %v", err)
	}
}

func TestParseCrushVersion(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"crush version v0.88.1\n", "0.88.1"},
		{"crush version 0.88.1", "0.88.1"},
		{"v1.0.0", "1.0.0"},
		{"", ""},
		{"command not found", ""},
		{"crush version unknown", ""},
	}
	for _, tc := range tests {
		if got := parseCrushVersion(tc.in); got != tc.want {
			t.Errorf("parseCrushVersion(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// --------------------------------------------------------------- the block --

func TestReplaceBlock(t *testing.T) {
	block := fmt.Sprintf(blockOpen+"\nhook add X\n"+blockClose+"\n", "2.0.0")
	old := fmt.Sprintf(blockOpen+"\nhook add OLD\n"+blockClose+"\n", "1.0.0")

	tests := []struct {
		name     string
		existing string
		want     string
	}{
		{"empty file takes the block alone", "", block},
		{"whitespace only is still empty", "\n\n  \n", block},
		{"appends after the user's lines", "mine\n", "mine\n\n" + block},
		{"replaces an older block in place", old, block},
		{"replaces between the user's lines", "before\n\n" + old + "after\n", "before\n\n" + block + "after\n"},
		{"an unterminated block is appended past, never eaten",
			"before\n" + fmt.Sprintf(blockOpen, "1.0.0") + "\nhook add OLD\n",
			"before\n" + fmt.Sprintf(blockOpen, "1.0.0") + "\nhook add OLD\n\n" + block},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := replaceBlock(tc.existing, block); got != tc.want {
				t.Errorf("replaceBlock(%q) =\n%q\nwant\n%q", tc.existing, got, tc.want)
			}
		})
	}
}

// Reinstalling must converge: the second write has to equal the first, or a
// crushrc grows a blank line — or a whole block — on every update.
func TestReplaceBlockIsIdempotent(t *testing.T) {
	block := fmt.Sprintf(blockOpen+"\nhook add X\n"+blockClose+"\n", "2.0.0")
	once := replaceBlock("mine\n", block)
	twice := replaceBlock(once, block)
	if once != twice {
		t.Errorf("a second install changed the file:\n%q\n%q", once, twice)
	}
}

// commandOf returns the command a named hook line carries, as Crush reads it:
// the value of --command with the crushrc line's own quoting removed. Its
// bounds are the two flags around it, which is what makes the outer quoting
// visible rather than guessed at.
func commandOf(block, name string) (string, bool) {
	for line := range strings.Lines(block) {
		if !strings.Contains(line, "--name "+name+" ") {
			continue
		}
		_, rest, ok := strings.Cut(line, "--command ")
		if !ok {
			return "", false
		}
		quoted, _, ok := strings.Cut(rest, " --timeout ")
		if !ok {
			return "", false
		}
		return shUnquote(quoted), true
	}
	return "", false
}

// shUnquote undoes one layer of single-quoting, the shape shquote.Quote writes.
func shUnquote(s string) string {
	s = strings.TrimPrefix(s, "'")
	s = strings.TrimSuffix(s, "'")
	return strings.ReplaceAll(s, `'\''`, "'")
}

// The command lands inside a shell word that Crush then runs through a shell,
// so both layers of quoting have to hold. A directory with a space is the case
// that catches a missing one, and running the command is the only way to know
// it does — a path split in two would run a different file, or none.
func TestCrushrcBlockSurvivesAPathWithSpaces(t *testing.T) {
	if runtime.GOOS == "windows" {
		// The assertion is that a POSIX shell resolves the command to one file;
		// Crush runs its own embedded one there, which this test cannot borrow.
		t.Skip("no /bin/sh to run the command through")
	}
	dir := filepath.Join(t.TempDir(), "some one")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	seen := filepath.Join(t.TempDir(), "args")
	for _, hook := range crushHooks {
		script := "#!/bin/sh\nprintf '%s|%s\\n' \"$0\" \"$*\" >> " + seen + "\n"
		if err := os.WriteFile(filepath.Join(dir, filepath.Base(hook.script)), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	block := crushrcBlock("1.2.3", dir, "/opt/lich/lich")
	for _, hook := range crushHooks {
		command, ok := commandOf(block, hook.name)
		if !ok {
			t.Fatalf("no %q hook in the block:\n%s", hook.name, block)
		}
		// Exactly what Crush does with the value: run it through a POSIX shell.
		out, err := exec.Command("/bin/sh", "-c", command).CombinedOutput()
		if err != nil {
			t.Fatalf("running %q: %v: %s", command, err, out)
		}
	}

	got, err := os.ReadFile(seen)
	if err != nil {
		t.Fatalf("no hook ran from a path with a space: %v", err)
	}
	for _, hook := range crushHooks {
		want := filepath.Join(dir, filepath.Base(hook.script)) + "|" + hook.arg
		if !strings.Contains(string(got), want+"\n") {
			t.Errorf("hook %q ran as something else:\n%s\nwant a line %q", hook.name, got, want)
		}
	}
}

func TestVersionOf(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		prefix string
		want   string
	}{
		{"js marker", "// lich-plugin v0.8.0 — installed by lich\ncode\n", jsComment, "0.8.0"},
		{"crushrc marker", "mine\n# >>> lich-plugin v1.2.3 >>>\nhook add X\n", shComment + " >>>", "1.2.3"},
		{"no marker", "just code\n", jsComment, ""},
		{"marker without a version", "// lich-plugin v\ncode\n", jsComment, ""},
		{"another comment first", "// unrelated\n// lich-plugin v0.9.0 x\n", jsComment, "0.9.0"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := versionOf([]byte(tc.body), tc.prefix)
			if got != tc.want || ok != (tc.want != "") {
				t.Errorf("versionOf = (%q,%v), want %q", got, ok, tc.want)
			}
		})
	}
}

// The tools half of the Crush install. Its hooks tell lich what a session is
// doing; this line is what lets that session act on the ones beside it, and it
// is the only way Crush can be given them — it takes no flag at spawn the way
// Claude Code and Codex do.

func TestCrushrcRegistersLichsMCPServer(t *testing.T) {
	block := crushrcBlock("1.2.3", "/plugins/lich", "/opt/lich/lich")

	line, ok := lineWith(block, "mcp add")
	if !ok {
		t.Fatalf("the block registers no MCP server:\n%s", block)
	}
	// Pinned as the literal Crush reads: stdio, the binary, and the subcommand
	// that serves the tools. A registration Crush cannot parse is one that fails
	// in silence — its whole reason for being checked here.
	// Quoted like every other path lich writes here: crushrc is read by a shell,
	// and a path is not the place to find out which characters it minds.
	want := `mcp add lich --type stdio --command '/opt/lich/lich' --args mcp --timeout 10`
	if line != want {
		t.Errorf("registration line:\n  %s\nwant:\n  %s", line, want)
	}
}

// A path with a space is the case that decides whether the line survives the
// shell that reads crushrc.
func TestCrushrcQuotesTheBinaryPath(t *testing.T) {
	block := crushrcBlock("1.2.3", "/plugins/lich", "/home/some one/lich")

	line, ok := lineWith(block, "mcp add")
	if !ok {
		t.Fatalf("no registration in:\n%s", block)
	}
	if !strings.Contains(line, `'/home/some one/lich'`) {
		t.Errorf("the binary path is not quoted for the shell: %s", line)
	}
}

// An unresolvable executable must not write a command that cannot run: the
// hooks are what make the install worth having, and they do not need it.
func TestCrushrcWithoutABinaryStillRegistersTheHooks(t *testing.T) {
	block := crushrcBlock("1.2.3", "/plugins/lich", "")

	if _, ok := lineWith(block, "mcp add"); ok {
		t.Errorf("registered a server with no binary to run:\n%s", block)
	}
	if _, ok := lineWith(block, "hook add"); !ok {
		t.Errorf("the hooks went with it:\n%s", block)
	}
}

// The registration lives inside the markers, so an update replaces it and an
// uninstall takes it away — the same rule every other line here answers to.
func TestTheMCPRegistrationIsInsideLichsBlock(t *testing.T) {
	block := crushrcBlock("1.2.3", "/plugins/lich", "/opt/lich/lich")
	existing := "# mine\nmodel add something\n"

	updated := replaceBlock(existing, block)
	if !strings.Contains(updated, "mcp add lich") {
		t.Fatalf("the registration did not land:\n%s", updated)
	}
	// Replacing the block with an empty one is what an uninstall does.
	cleaned := replaceBlock(updated, "")
	if strings.Contains(cleaned, "mcp add") {
		t.Errorf("the registration outlived lich's block:\n%s", cleaned)
	}
	if !strings.Contains(cleaned, "model add something") {
		t.Errorf("a line the user wrote went with it:\n%s", cleaned)
	}
}

// lineWith returns the block's first line containing needle.
func lineWith(block, needle string) (string, bool) {
	for _, line := range strings.Split(block, "\n") {
		if strings.Contains(line, needle) {
			return strings.TrimSpace(line), true
		}
	}
	return "", false
}

package agentplugin

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/omartelo/lich/internal/providers"
	"github.com/omartelo/lich/internal/relay"
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
		lichBin:   lichBinary,
	}, func() []string { return asked }
}

// tagged is the URL path a release file is served at.
func tagged(path string) string { return "/v" + testVersion + "/" + path }

// A body lich cannot trust is refused rather than trimmed to fit: what it
// fetches is written where a harness loads and runs it, under a marker line
// saying which release it is — and a truncated file carries that line too.
func TestFetchFileBodyLimit(t *testing.T) {
	tests := []struct {
		name    string
		size    int
		wantErr bool
	}{
		{name: "an empty file is no release file", size: 0, wantErr: true},
		{name: "a file at the cap is whole", size: fileBodyLimit},
		{name: "one byte past it is refused", size: fileBodyLimit + 1, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := fileServer(t, map[string]string{
				tagged(opencodeSource): strings.Repeat("x", tc.size),
			})

			data, err := s.fetchFile(testVersion, opencodeSource)
			if (err != nil) != tc.wantErr {
				t.Fatalf("fetchFile with a %d byte body: err = %v, want an error: %v", tc.size, err, tc.wantErr)
			}
			if err == nil && len(data) != tc.size {
				t.Errorf("fetchFile returned %d bytes, want the %d the release served", len(data), tc.size)
			}
		})
	}
}

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

// --------------------------------------------------------------------- omp --

// ompHome points an install at a temporary agent directory and returns it. The
// explicit override is used rather than a redirected home, so the isolation
// holds on every OS without depending on which variable that OS reads a home
// from — and the profile variable is cleared, because a machine that has one set
// would otherwise send the install somewhere else entirely.
func ompHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("OMP_PROFILE", "")
	t.Setenv("PI_CODING_AGENT_DIR", dir)
	return dir
}

// The profile beats the explicit override, which is the order `omp config path`
// applies and the opposite of how the two read. Asserted through a real install
// rather than on the resolver alone: getting it backwards writes the extension
// into a directory omp never scans, and every report goes missing in silence.
// (internal/terminal resolves the same rule for resume, and pins it separately.)
func TestOMPInstallFollowsTheProfileOverTheOverride(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	// Asserted, not assumed: a platform whose home this redirect misses would
	// send the install into the real user's directory and still pass.
	if dir, err := os.UserHomeDir(); err != nil || dir != home {
		t.Fatalf("home redirect missed: %q (err %v)", dir, err)
	}
	t.Setenv("PI_CODING_AGENT_DIR", t.TempDir())
	t.Setenv("OMP_PROFILE", "work")
	s, _ := fileServer(t, map[string]string{tagged(ompSource): "export default () => {}\n"})

	if err := s.Install(providers.OMP); err != nil {
		t.Fatalf("Install: %v", err)
	}

	profile := filepath.Join(home, ".omp", "profiles", "work", "agent")
	if _, err := os.Stat(filepath.Join(profile, ompExtensionsDir, ompFile)); err != nil {
		t.Fatalf("the extension is not in the profile's directory: %v", err)
	}
	if override := os.Getenv("PI_CODING_AGENT_DIR"); override != "" {
		if _, err := os.Stat(filepath.Join(override, ompExtensionsDir, ompFile)); !os.IsNotExist(err) {
			t.Errorf("the override was written too (stat err = %v)", err)
		}
	}
	// And the version reads back from the same place it was written to.
	if got, ok := s.installedVersion(providers.OMP); !ok || got != testVersion {
		t.Errorf("installedVersion = (%q,%v), want (%q,true)", got, ok, testVersion)
	}
}

func TestOMPInstallWritesTheExtensionAndRegistersTheServer(t *testing.T) {
	agent := ompHome(t)
	s, asked := fileServer(t, map[string]string{
		tagged(ompSource): "export default function lichPlugin(pi) {}\n",
	})

	if err := s.Install(providers.OMP); err != nil {
		t.Fatalf("Install: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(agent, ompExtensionsDir, ompFile))
	if err != nil {
		t.Fatalf("read the installed extension: %v", err)
	}
	body := string(data)
	if !strings.Contains(body, "export default function lichPlugin") {
		t.Errorf("installed extension lost the release's body:\n%s", body)
	}
	if !strings.HasPrefix(body, jsComment+" "+markerName+" v"+testVersion+" ") {
		t.Errorf("installed extension has no version marker on its first line:\n%s", body)
	}
	if got := asked(); len(got) != 1 || got[0] != tagged(ompSource) {
		t.Errorf("fetched %v, want just %q", got, tagged(ompSource))
	}

	// omp takes no --mcp-config flag, so this document is the only way a session
	// of its own reaches the other sessions.
	servers := readOMPServers(t, agent)
	lich, ok := servers[relay.MCPServerName].(map[string]any)
	if !ok {
		t.Fatalf("no %q server in %v", relay.MCPServerName, servers)
	}
	if lich["command"] == "" || lich["command"] == nil {
		t.Errorf("registered server has no command: %v", lich)
	}
	if args, _ := lich["args"].([]any); len(args) != 1 || args[0] != relay.MCPSubcommand {
		t.Errorf("registered server args = %v, want [%q]", lich["args"], relay.MCPSubcommand)
	}

	if got, ok := s.installedVersion(providers.OMP); !ok || got != testVersion {
		t.Errorf("installedVersion = (%q,%v), want (%q,true)", got, ok, testVersion)
	}
}

// The document belongs to the user: lich rewrites it, so everything it did not
// come for has to survive the round trip — other servers, and the keys lich
// knows nothing about.
func TestOMPInstallKeepsEveryOtherServer(t *testing.T) {
	agent := ompHome(t)
	existing := `{
  "mcpServers": {
    "notes": {"command": "notes-server", "args": ["--stdio"]}
  },
  "somethingElse": {"kept": true}
}`
	if err := os.WriteFile(filepath.Join(agent, ompMCPFile), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	s, _ := fileServer(t, map[string]string{tagged(ompSource): "export default () => {}\n"})

	if err := s.Install(providers.OMP); err != nil {
		t.Fatalf("Install: %v", err)
	}

	servers := readOMPServers(t, agent)
	if _, ok := servers["notes"]; !ok {
		t.Errorf("the user's own server is gone: %v", servers)
	}
	if _, ok := servers[relay.MCPServerName]; !ok {
		t.Errorf("lich did not register itself: %v", servers)
	}
	raw, err := os.ReadFile(filepath.Join(agent, ompMCPFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "somethingElse") {
		t.Errorf("a key lich does not know about was dropped:\n%s", raw)
	}
}

// A document lich cannot parse is refused rather than replaced: overwriting it
// would delete servers lich never got to see.
func TestOMPInstallRefusesAnUnreadableDocument(t *testing.T) {
	agent := ompHome(t)
	const garbage = "{ not json"
	path := filepath.Join(agent, ompMCPFile)
	if err := os.WriteFile(path, []byte(garbage), 0o644); err != nil {
		t.Fatal(err)
	}
	s, _ := fileServer(t, map[string]string{tagged(ompSource): "export default () => {}\n"})

	if err := s.Install(providers.OMP); err == nil {
		t.Fatal("Install: want an error for a document lich cannot merge into, got nil")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != garbage {
		t.Errorf("the document was rewritten anyway: %q (err %v)", data, err)
	}
	// And nothing else was written either: an extension reporting into a session
	// that never got the tools is the half-install this ordering exists to avoid.
	if _, err := os.Stat(filepath.Join(agent, ompExtensionsDir, ompFile)); !os.IsNotExist(err) {
		t.Errorf("the refused install wrote the extension anyway (stat err = %v)", err)
	}
}

// A release that does not carry the extension must leave nothing behind — no
// module, and no registration pointing at reports that will never come.
func TestOMPInstallWritesNothingWhenTheFetchFails(t *testing.T) {
	agent := ompHome(t)
	s, _ := fileServer(t, nil) // every path 404s

	if err := s.Install(providers.OMP); err == nil {
		t.Fatal("Install: want an error when the release has no extension, got nil")
	}
	if _, err := os.Stat(filepath.Join(agent, ompExtensionsDir, ompFile)); !os.IsNotExist(err) {
		t.Errorf("a failed install left an extension behind (stat err = %v)", err)
	}
	if _, err := os.Stat(filepath.Join(agent, ompMCPFile)); !os.IsNotExist(err) {
		t.Errorf("a failed install registered a server anyway (stat err = %v)", err)
	}
}

// readOMPServers returns the mcpServers table lich wrote.
func readOMPServers(t *testing.T, agent string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(agent, ompMCPFile))
	if err != nil {
		t.Fatalf("read the mcp document: %v", err)
	}
	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("the mcp document is not JSON: %v\n%s", err, data)
	}
	servers, ok := config[mcpServersKey].(map[string]any)
	if !ok {
		t.Fatalf("no %s table in:\n%s", mcpServersKey, data)
	}
	return servers
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
	// Asserted, not assumed: a redirect that misses a platform's variable leaves
	// this answering the real config directory — which the install then writes
	// into, agreeing with the assertions and staying green while leaking files
	// onto the machine. The failure has to be the redirect, not the leak.
	if !strings.HasPrefix(dir, root) {
		t.Fatalf("config dir %q is outside the test's temp dir %q", dir, root)
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

// A `crush dirs` that prints nothing leaves lich without the directory it was
// asked for. Joining "crushrc" onto an empty one would write the file into
// whatever the process's working directory happens to be.
func TestCrushrcPathWithoutAConfigDir(t *testing.T) {
	s, _ := fileServer(t, nil)
	crushCLI(t, s, "", "0.88.1")

	if path, err := s.crushrcPath(); err == nil {
		t.Fatalf("crushrcPath = %q, want an error when crush dirs prints no directory", path)
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
	for line := range strings.SplitSeq(block, "\n") {
		if strings.Contains(line, needle) {
			return strings.TrimSpace(line), true
		}
	}
	return "", false
}

// ------------------------------------------------------------- antigravity --

// antigravityHome redirects the home Antigravity discovers global
// customizations under, and returns the plugin directory an install will write
// into. Both variables are set for the reason lichConfigHome sets three:
// os.UserHomeDir reads HOME on Unix and USERPROFILE on Windows, and a redirect
// that misses one leaves the install writing into the real user's home while
// the assertions read the temporary one.
func antigravityHome(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("USERPROFILE", root)
	dir, err := antigravityPluginDir()
	if err != nil {
		t.Fatalf("resolve the plugin directory: %v", err)
	}
	if !strings.HasPrefix(dir, root) {
		t.Fatalf("plugin dir %q is outside the test's temp dir %q", dir, root)
	}
	return dir
}

// antigravityCLI points the Service's shell-out — the MCP registration, and
// nothing else here — at the test binary's fake CLI, and returns a reader of the
// calls it received.
func antigravityCLI(t *testing.T, s *Service) func() []string {
	t.Helper()
	log := filepath.Join(t.TempDir(), "calls.log")
	t.Setenv(fakeCLIGuard, "1")
	t.Setenv(fakeCLILog, log)
	s.bins = stubBin(mustExecutable(t))
	return func() []string {
		data, err := os.ReadFile(log)
		if err != nil {
			return nil
		}
		return strings.Split(strings.TrimSpace(string(data)), "\n")
	}
}

func mustExecutable(t *testing.T) string {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve test binary: %v", err)
	}
	return self
}

// antigravityFiles is the release as this install reads it: the registration at
// the repository root and every script it names, with the relative commands
// Antigravity resolves against the directory holding hooks.json.
func antigravityFiles() map[string]string {
	files := map[string]string{
		tagged(antigravityHooksFile): `{"lich":{"PreInvocation":[{"type":"command",` +
			`"command":"hooks/report-session-start.sh antigravity; printf '{}'"}]}}`,
	}
	for _, script := range antigravityScripts {
		files[tagged(script)] = "#!/bin/sh\n# " + filepath.Base(script) + "\nexit 0\n"
	}
	return files
}

func TestAntigravityInstallWritesTheCustomization(t *testing.T) {
	dir := antigravityHome(t)
	s, _ := fileServer(t, antigravityFiles())
	antigravityCLI(t, s)

	if err := s.Install(providers.Antigravity); err != nil {
		t.Fatalf("Install: %v", err)
	}

	for _, script := range antigravityScripts {
		path := filepath.Join(dir, antigravityScriptDir, filepath.Base(script))
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		// Antigravity runs a hook command through `sh -c`, which needs the bit on
		// a shebang'd script. Windows carries no POSIX mode, so the assertion is
		// made where it decides anything.
		if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
			t.Errorf("%s is not executable (mode %v)", path, info.Mode().Perm())
		}
	}

	hooks, err := os.ReadFile(filepath.Join(dir, antigravityHooksFile))
	if err != nil {
		t.Fatalf("read the registration: %v", err)
	}
	body := string(hooks)
	// The registration is written through untouched, and its commands are
	// relative — Antigravity runs a hook with the working directory set to the
	// folder holding this file, which is where the scripts just went. A rewrite
	// that resolved them would only be another way to get that wrong.
	if !strings.Contains(body, antigravityScriptDir+"/report-session-start.sh") {
		t.Errorf("the registration does not name the installed script:\n%s", body)
	}
	// The provider argument is what puts Antigravity's icon on the card rather
	// than Claude Code's, and what decides which CLI is asked to resume it.
	if !strings.Contains(body, providers.Antigravity) {
		t.Errorf("the registration does not name the provider:\n%s", body)
	}
}

// The manifest is what marks the directory as a plugin at all, and the version
// in it is this install's only record of what it wrote.
func TestAntigravityInstalledVersionRoundTrips(t *testing.T) {
	dir := antigravityHome(t)
	s, _ := fileServer(t, antigravityFiles())
	antigravityCLI(t, s)

	if err := s.Install(providers.Antigravity); err != nil {
		t.Fatalf("Install: %v", err)
	}
	var manifest struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	data, err := os.ReadFile(filepath.Join(dir, antigravityManifest))
	if err != nil {
		t.Fatalf("read the manifest: %v", err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("the manifest is not JSON: %v\n%s", err, data)
	}
	if manifest.Name != pluginName {
		t.Errorf("manifest names %q, want %q", manifest.Name, pluginName)
	}
	got, ok := s.installedVersion(providers.Antigravity)
	if !ok || got != testVersion {
		t.Fatalf("installedVersion = (%q,%v), want (%q,true)", got, ok, testVersion)
	}
}

// A copy installed through `agy plugin install` carries a manifest with no
// version. Reporting one for it would claim an install lich never wrote and
// cannot update.
func TestAntigravityInstalledVersionIgnoresAnUnversionedManifest(t *testing.T) {
	dir := antigravityHome(t)
	s, _ := fileServer(t, nil)

	if _, ok := s.installedVersion(providers.Antigravity); ok {
		t.Error("reported a version with nothing installed")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, antigravityManifest), []byte(`{"name":"lich"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, ok := s.installedVersion(providers.Antigravity); ok {
		t.Errorf("installedVersion = (%q,true) for a manifest lich did not write, want false", got)
	}
}

// The registration is what gives an Antigravity session the operations for
// driving the others: it takes no MCP flag at spawn, so this is the only place
// it can be told.
func TestAntigravityInstallRegistersLichsMCPServer(t *testing.T) {
	antigravityHome(t)
	s, _ := fileServer(t, antigravityFiles())
	calls := antigravityCLI(t, s)
	s.lichBin = func() string { return "/opt/lich/lich" }

	if err := s.Install(providers.Antigravity); err != nil {
		t.Fatalf("Install: %v", err)
	}
	want := "mcp add " + relay.MCPServerName + " /opt/lich/lich " + relay.MCPSubcommand
	if got := calls(); !slices.Contains(got, want) {
		t.Errorf("calls = %v, want one of them to be %q", got, want)
	}
}

// A lich that cannot name its own binary registers nothing rather than a command
// that cannot run — the reports are the half the session needs, and they do not
// depend on it.
func TestAntigravityInstallWithoutABinaryStillWritesTheHooks(t *testing.T) {
	dir := antigravityHome(t)
	s, _ := fileServer(t, antigravityFiles())
	calls := antigravityCLI(t, s)
	s.lichBin = func() string { return "" }

	if err := s.Install(providers.Antigravity); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, antigravityHooksFile)); err != nil {
		t.Errorf("the registration was not written: %v", err)
	}
	if got := calls(); len(got) > 0 {
		t.Errorf("calls = %v, want none: there is no binary to register", got)
	}
}

// Every file is fetched before anything is written, so a release missing one
// leaves no half-installed customization behind — a hooks.json naming a script
// that is not there is a session reporting nothing on every turn.
func TestAntigravityInstallWritesNothingWhenTheFetchFails(t *testing.T) {
	dir := antigravityHome(t)
	files := antigravityFiles()
	delete(files, tagged(antigravityScripts[len(antigravityScripts)-1]))
	s, _ := fileServer(t, files)
	antigravityCLI(t, s)

	if err := s.Install(providers.Antigravity); err == nil {
		t.Fatal("Install: want an error when the release is missing a script, got nil")
	}
	if _, err := os.Stat(filepath.Join(dir, antigravityHooksFile)); !os.IsNotExist(err) {
		t.Errorf("a failed install left a registration behind (stat err = %v)", err)
	}
	if _, err := os.Stat(filepath.Join(dir, antigravityManifest)); !os.IsNotExist(err) {
		t.Errorf("a failed install left a manifest behind (stat err = %v)", err)
	}
}

// A refused `agy mcp add` must not leave an install that claims to be finished.
// The manifest is what Installed, the update check and HasTools all read, and
// HasTools promising lich's own tools is what an agent meets as an error at its
// prompt — so the registration has to succeed before the version is written
// down.
func TestAntigravityInstallClaimsNothingWhenTheRegistrationFails(t *testing.T) {
	dir := antigravityHome(t)
	s, _ := fileServer(t, antigravityFiles())
	antigravityCLI(t, s)
	t.Setenv(fakeCLIFail, "mcp add")
	s.lichBin = func() string { return "/opt/lich/lich" }

	if err := s.Install(providers.Antigravity); err == nil {
		t.Fatal("Install: want an error when the registration is refused, got nil")
	}
	// The hooks stay: they are written and they work, and the next install
	// overwrites them. It is the claim that must not survive.
	if _, err := os.Stat(filepath.Join(dir, antigravityHooksFile)); err != nil {
		t.Errorf("the registration's own scripts were rolled back: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, antigravityManifest)); !os.IsNotExist(err) {
		t.Errorf("a refused registration left a manifest behind (stat err = %v)", err)
	}
	if got, ok := s.installedVersion(providers.Antigravity); ok {
		t.Errorf("installedVersion = (%q,true) after the registration failed, want false", got)
	}
	if s.Installed(providers.Antigravity) {
		t.Error("Installed = true after the registration failed")
	}
	// HasTools is not asserted here, and the omission is the point: it reads the
	// same manifest, so the three checks above already decide it — and at this
	// fixture's version (below toolsMinVersion) it answers false either way,
	// which would be an assertion that cannot fail.
}

// cursorHome points both halves of a Cursor install at temporary directories:
// the home it writes `~/.cursor/mcp.json` under, and the Claude Code state that
// decides whether there is anything to register tools beside.
func cursorHome(t *testing.T, claudeState string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("CLAUDE_CONFIG_DIR", writeClaudeState(t, claudeState))
	return home
}

func readCursorServers(t *testing.T, home string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(home, ".cursor", cursorMCPFile))
	if err != nil {
		t.Fatalf("read cursor's MCP document: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("cursor's MCP document is not JSON: %v", err)
	}
	servers, _ := doc[mcpServersKey].(map[string]any)
	return servers
}

const claudeStateWithPlugin = `{"plugins":{"lich@lich-plugin":[{"scope":"user","version":"0.9.0"}]}}`

// Cursor takes no MCP server on its command line and gets none from the Claude
// Code plugin it does run, so this document is the only way a session of its own
// reaches the sessions beside it.
func TestCursorInstallRegistersTheServer(t *testing.T) {
	home := cursorHome(t, claudeStateWithPlugin)
	s := New(stubBins{})
	s.lichBin = func() string { return "/usr/bin/lich" }

	if err := s.Install(providers.Cursor); err != nil {
		t.Fatalf("Install: %v", err)
	}

	lich, ok := readCursorServers(t, home)[relay.MCPServerName].(map[string]any)
	if !ok {
		t.Fatalf("no %q server registered", relay.MCPServerName)
	}
	if lich["command"] != "/usr/bin/lich" {
		t.Errorf("registered command = %v, want the resolved lich binary", lich["command"])
	}
	if args, _ := lich["args"].([]any); len(args) != 1 || args[0] != relay.MCPSubcommand {
		t.Errorf("registered args = %v, want [%q]", lich["args"], relay.MCPSubcommand)
	}
}

// The version reported for Cursor is Claude Code's, because that is the install
// its reports actually come from — and it is reported only once the tools are
// registered too, since a session needs both halves.
func TestCursorInstalledVersionIsClaudeCodes(t *testing.T) {
	cursorHome(t, claudeStateWithPlugin)
	s := New(stubBins{})
	s.lichBin = func() string { return "/usr/bin/lich" }

	if got, ok := s.installedVersion(providers.Cursor); ok {
		t.Errorf("installedVersion = (%q,%v) before the server was registered, want not installed", got, ok)
	}
	if err := s.Install(providers.Cursor); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if got, ok := s.installedVersion(providers.Cursor); !ok || got != "0.9.0" {
		t.Errorf("installedVersion = (%q,%v), want (\"0.9.0\",true)", got, ok)
	}
}

// Registering the tools while the reports have nowhere to come from is a card
// that answers with lich's own operations and never says it is working, so the
// install refuses and names the step that is missing.
func TestCursorInstallRefusesWithoutTheClaudeCodeInstall(t *testing.T) {
	home := cursorHome(t, `{"plugins":{}}`)
	s := New(stubBins{})
	s.lichBin = func() string { return "/usr/bin/lich" }

	err := s.Install(providers.Cursor)
	if err == nil {
		t.Fatal("Install = nil with no plugin in Claude Code, want an error naming it")
	}
	if !strings.Contains(err.Error(), "Claude Code") {
		t.Errorf("error does not name what to install: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".cursor", cursorMCPFile)); !os.IsNotExist(err) {
		t.Error("a refused install still wrote the MCP document")
	}
}

// The document belongs to the user: lich rewrites it, so every server and every
// key it did not come for survives the round trip.
func TestCursorInstallKeepsEveryOtherServer(t *testing.T) {
	home := cursorHome(t, claudeStateWithPlugin)
	dir := filepath.Join(home, ".cursor")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	existing := `{
  "mcpServers": {"notes": {"command": "notes-server", "args": ["--stdio"]}},
  "somethingElse": {"kept": true}
}`
	if err := os.WriteFile(filepath.Join(dir, cursorMCPFile), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	s := New(stubBins{})
	s.lichBin = func() string { return "/usr/bin/lich" }

	if err := s.Install(providers.Cursor); err != nil {
		t.Fatalf("Install: %v", err)
	}

	servers := readCursorServers(t, home)
	if _, ok := servers["notes"]; !ok {
		t.Errorf("the user's own server is gone: %v", servers)
	}
	if _, ok := servers[relay.MCPServerName]; !ok {
		t.Errorf("lich did not register itself: %v", servers)
	}
	raw, err := os.ReadFile(filepath.Join(dir, cursorMCPFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "somethingElse") {
		t.Errorf("a key lich does not know about was dropped:\n%s", raw)
	}
}

// A document lich cannot parse is refused rather than replaced: overwriting it
// would delete servers lich never got to see.
func TestCursorInstallRefusesAnUnreadableDocument(t *testing.T) {
	home := cursorHome(t, claudeStateWithPlugin)
	dir := filepath.Join(home, ".cursor")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	const garbage = "{ not json"
	path := filepath.Join(dir, cursorMCPFile)
	if err := os.WriteFile(path, []byte(garbage), 0o644); err != nil {
		t.Fatal(err)
	}
	s := New(stubBins{})
	s.lichBin = func() string { return "/usr/bin/lich" }

	if err := s.Install(providers.Cursor); err == nil {
		t.Fatal("Install = nil over a document lich cannot merge into")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != garbage {
		t.Errorf("the user's document was rewritten: %q", data)
	}
}

// A lich that cannot name its own binary registers nothing rather than a command
// that cannot run — the session still has the `lich` command line.
func TestCursorInstallWritesNothingWithoutABinary(t *testing.T) {
	home := cursorHome(t, claudeStateWithPlugin)
	s := New(stubBins{})
	s.lichBin = func() string { return "" }

	if err := s.Install(providers.Cursor); err == nil {
		t.Fatal("Install = nil with no lich binary to register")
	}
	if _, err := os.Stat(filepath.Join(home, ".cursor", cursorMCPFile)); !os.IsNotExist(err) {
		t.Error("a refused install still wrote the MCP document")
	}
}

// Cursor's tools come from the document the install writes, and its version from
// Claude Code's install — so it can answer with a tool only once both halves are
// there, and never when lich cannot name the binary to register.
func TestHasToolsForCursor(t *testing.T) {
	cursorHome(t, claudeStateWithPlugin)
	s := New(stubBins{})
	s.lookPath = func(string) (string, error) { return "/usr/bin/cursor-agent", nil }
	s.lichBin = func() string { return "/usr/bin/lich" }

	if s.HasTools(providers.Cursor) {
		t.Error("HasTools(cursor) = true before lich's server was registered")
	}
	if err := s.Install(providers.Cursor); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !s.HasTools(providers.Cursor) {
		t.Error("HasTools(cursor) = false with the server registered and the plugin in Claude Code")
	}

	s.lichBin = func() string { return "" }
	if s.HasTools(providers.Cursor) {
		t.Error("HasTools(cursor) = true, but no server could be registered for it")
	}
}

// TestUnderGoBuildCache pins the shape that keeps a `go run` binary out of a
// harness's config. Writing one produces a registration that works until that
// run ends and then fails silently forever, which is what a `task dev` install
// left in a real ~/.cursor/mcp.json. The paths are composed for the running OS:
// what the cache looks like is a separator question.
func TestUnderGoBuildCache(t *testing.T) {
	tmp := os.TempDir()
	cases := []struct {
		name string
		path string
		want bool
	}{
		{"the binary go run builds", filepath.Join(tmp, "go-build1790994521", "b001", "exe", "lich"), true},
		{"and a deeper cache root", filepath.Join(tmp, "x", "go-build42", "b001", "exe", "lich"), true},
		{"an installed lich", filepath.Join(string(filepath.Separator), "usr", "bin", "lich"), false},
		{"one built into the repo", filepath.Join(string(filepath.Separator), "home", "u", "src", "bin", "lich"), false},
		// `exe` alone is not the cache, and the cache without it is not the
		// binary: both halves have to be there.
		{"an exe directory of the user's own", filepath.Join(string(filepath.Separator), "opt", "exe", "lich"), false},
		{"the cache without the exe directory", filepath.Join(tmp, "go-build42", "b001", "lich"), false},
		{"nothing at all", "", false},
	}
	for _, tc := range cases {
		if got := underGoBuildCache(tc.path); got != tc.want {
			t.Errorf("%s: underGoBuildCache(%q) = %v, want %v", tc.name, tc.path, got, tc.want)
		}
	}
}

// TestLichBinaryFallsBackToPath proves a dev build registers the installed lich
// rather than nothing. `task dev` runs the app with `go run`, which is where this
// project is worked on, and an install that refuses there is one nobody can test.
// The registration is only the transport — a session reaches the lich its PTY's
// coordinates name — so any lich that starts is the right one to write.
func TestLichBinaryFallsBackToPath(t *testing.T) {
	const onPath = "/usr/bin/lich"
	found := func(string) (string, error) { return onPath, nil }
	missing := func(string) (string, error) { return "", exec.ErrNotFound }
	goRun := filepath.Join(os.TempDir(), "go-build1790994521", "b001", "exe", "lich")
	installed := filepath.Join(string(filepath.Separator), "opt", "lich", "lich")

	cases := []struct {
		name     string
		exe      string
		exeErr   error
		lookPath func(string) (string, error)
		want     string
	}{
		{"an ordinary build registers itself", installed, nil, missing, installed},
		{"a go run build registers the lich on PATH", goRun, nil, found, onPath},
		{"and nothing when there is none", goRun, nil, missing, ""},
		{"an unresolvable executable falls back too", "", exec.ErrNotFound, found, onPath},
	}
	for _, tc := range cases {
		if got := resolveLichBinary(tc.exe, tc.exeErr, tc.lookPath); got != tc.want {
			t.Errorf("%s: resolveLichBinary(%q) = %q, want %q", tc.name, tc.exe, got, tc.want)
		}
	}
}

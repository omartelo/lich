package agentplugin

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/omartelo/lich/internal/providers"
)

func TestParseInstalledVersion(t *testing.T) {
	tests := []struct {
		name   string
		json   string
		want   string
		wantOK bool
	}{
		{
			name:   "user scope",
			json:   `{"plugins":{"lich@lich-plugin":[{"scope":"user","version":"0.0.1"}]}}`,
			want:   "0.0.1",
			wantOK: true,
		},
		{
			name:   "prefers user over project",
			json:   `{"plugins":{"lich@lich-plugin":[{"scope":"project","version":"9.9.9"},{"scope":"user","version":"0.0.1"}]}}`,
			want:   "0.0.1",
			wantOK: true,
		},
		{
			name:   "falls back to any scope",
			json:   `{"plugins":{"lich@lich-plugin":[{"scope":"project","version":"1.2.3"}]}}`,
			want:   "1.2.3",
			wantOK: true,
		},
		{
			name:   "missing key",
			json:   `{"plugins":{"other@mkt":[{"scope":"user","version":"1.0.0"}]}}`,
			want:   "",
			wantOK: false,
		},
		{
			name:   "empty version ignored",
			json:   `{"plugins":{"lich@lich-plugin":[{"scope":"user","version":""}]}}`,
			want:   "",
			wantOK: false,
		},
		{name: "malformed", json: `{`, want: "", wantOK: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseInstalledVersion([]byte(tc.json), "lich@lich-plugin")
			if got != tc.want || ok != tc.wantOK {
				t.Fatalf("got (%q,%v), want (%q,%v)", got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

// TestParseCodexPluginList covers Codex's own listing, which answers the same
// question from a different shape — including the banner its CLI prints ahead
// of the JSON on some setups.
func TestParseCodexPluginList(t *testing.T) {
	tests := []struct {
		name   string
		json   string
		want   string
		wantOK bool
	}{
		{
			name:   "installed",
			json:   `{"installed":[{"pluginId":"lich@lich-plugin","version":"0.5.0","installed":true}],"available":[]}`,
			want:   "0.5.0",
			wantOK: true,
		},
		{
			name:   "skips a banner before the json",
			json:   "WARNING: proceeding\n{\"installed\":[{\"pluginId\":\"lich@lich-plugin\",\"version\":\"0.5.0\",\"installed\":true}]}",
			want:   "0.5.0",
			wantOK: true,
		},
		{
			name:   "another plugin only",
			json:   `{"installed":[{"pluginId":"other@mkt","version":"1.0.0","installed":true}]}`,
			want:   "",
			wantOK: false,
		},
		{
			name:   "listed but not installed",
			json:   `{"installed":[{"pluginId":"lich@lich-plugin","version":"0.5.0","installed":false}]}`,
			want:   "",
			wantOK: false,
		},
		{name: "no json at all", json: "command not found", want: "", wantOK: false},
		{name: "malformed", json: `{`, want: "", wantOK: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseCodexPluginList([]byte(tc.json), "lich@lich-plugin")
			if got != tc.want || ok != tc.wantOK {
				t.Fatalf("got (%q,%v), want (%q,%v)", got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func TestComputeStatus(t *testing.T) {
	tests := []struct {
		name           string
		installed      bool
		installedVer   string
		latestVer      string
		wantUpdate     bool
		wantCompatible bool
	}{
		{"not installed", false, "", "0.12.2", false, true},
		{"installed, no latest known", true, "0.12.1", "", false, true},
		{"update available", true, "0.12.1", "0.12.2", true, true},
		{"already latest", true, "0.12.2", "0.12.2", false, true},
		{"installed newer than latest, still compatible", true, "0.13.0", "0.12.2", false, true},
		{"pre-release install sees the stable release", true, "0.12.0-rc.3", "0.12.0", true, true},
		// Past the ceiling: the compatible release is offered even though it is
		// older, because it is the one this lich speaks.
		{"installed past the ceiling", true, PluginVersionCeiling, "0.13.1", true, false},
		{"installed below the floor", true, "0.2.9", "0.13.1", true, false},
		{"incompatible, no compatible release known", true, "0.2.9", "", false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := computeStatus(providers.Codex, true, tc.installed, tc.installedVer, tc.latestVer)
			if got.UpdateAvailable != tc.wantUpdate {
				t.Fatalf("UpdateAvailable = %v, want %v", got.UpdateAvailable, tc.wantUpdate)
			}
			if got.Compatible != tc.wantCompatible {
				t.Fatalf("Compatible = %v, want %v", got.Compatible, tc.wantCompatible)
			}
			if got.Installed != tc.installed || got.InstalledVersion != tc.installedVer || got.LatestVersion != tc.latestVer {
				t.Fatalf("status = %+v, mismatch on passthrough fields", got)
			}
			if got.Provider != providers.Codex || got.Name != "Codex" {
				t.Fatalf("status = %+v, want it to name the provider it is about", got)
			}
		})
	}
}

// Cursor's version is Claude Code's install, so an incompatible one is reported
// on its row but the fix is offered on Claude Code's.
func TestComputeStatusCursorNeverOffersTheFix(t *testing.T) {
	got := computeStatus(providers.Cursor, true, true, PluginVersionCeiling, "0.13.1")
	if got.Compatible || got.UpdateAvailable {
		t.Fatalf("status = %+v, want incompatible with no update of its own", got)
	}
}

func TestClaudeInstalledVersionReadsConfigDir(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", writeClaudeState(t, `{"plugins":{"lich@lich-plugin":[{"scope":"user","version":"0.3.1"}]}}`))

	ver, ok := claudeInstalledVersion()
	if !ok || ver != "0.3.1" {
		t.Fatalf("got (%q,%v), want (0.3.1,true)", ver, ok)
	}
}

func TestClaudeInstalledVersionMissingFile(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	if ver, ok := claudeInstalledVersion(); ok || ver != "" {
		t.Fatalf("got (%q,%v), want empty/false for missing file", ver, ok)
	}
}

func TestClaudeInstalledVersionNoConfigDir(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	if ver, ok := claudeInstalledVersion(); ok || ver != "" {
		t.Fatalf("got (%q,%v), want empty/false without a config dir", ver, ok)
	}
}

// writeClaudeState lays out a Claude Code config dir holding body as its
// installed-plugin state, and returns the dir.
func writeClaudeState(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	pluginsDir := filepath.Join(dir, "plugins")
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginsDir, "installed_plugins.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

type stubBins struct{ bin string }

func (s stubBins) ProviderBin(string, string) string { return s.bin }

func TestNewDefaults(t *testing.T) {
	s := New(stubBins{bin: "claude"})
	if s.releasesURL != releasesURL {
		t.Errorf("releasesURL = %q, want %q", s.releasesURL, releasesURL)
	}
	if s.http == nil || s.http.Timeout != httpTimeout {
		t.Errorf("http client = %+v, want one with a %v timeout", s.http, httpTimeout)
	}
	if s.lookPath == nil {
		t.Error("lookPath = nil, want the real PATH lookup")
	}
	if s.bins == nil {
		t.Error("bins = nil, want the resolver passed to New")
	}
}

func TestClaudeConfigDir(t *testing.T) {
	t.Run("env override wins", func(t *testing.T) {
		t.Setenv("CLAUDE_CONFIG_DIR", "/custom/claude")
		if got := claudeConfigDir(); got != "/custom/claude" {
			t.Fatalf("claudeConfigDir() = %q, want %q", got, "/custom/claude")
		}
	})

	t.Run("falls back to home", func(t *testing.T) {
		t.Setenv("CLAUDE_CONFIG_DIR", "")
		// os.UserHomeDir reads HOME on Unix and USERPROFILE on Windows.
		t.Setenv("HOME", "/home/someone")
		t.Setenv("USERPROFILE", "/home/someone")
		if got := claudeConfigDir(); got != filepath.Join("/home/someone", ".claude") {
			t.Fatalf("claudeConfigDir() = %q, want %q", got, "/home/someone/.claude")
		}
	})

	t.Run("empty when home is unresolvable", func(t *testing.T) {
		t.Setenv("CLAUDE_CONFIG_DIR", "")
		t.Setenv("HOME", "")
		t.Setenv("USERPROFILE", "")
		if got := claudeConfigDir(); got != "" {
			t.Fatalf("claudeConfigDir() = %q, want %q", got, "")
		}
	})
}

// serveBody starts a test server returning status/body and returns a Service
// pointed at it, with every provider CLI present.
func serveBody(t *testing.T, status int, body string) *Service {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return &Service{
		http:        srv.Client(),
		releasesURL: srv.URL,
		bins:        stubBins{},
		lookPath:    func(name string) (string, error) { return "/usr/bin/" + name, nil },
		lichBin:     lichBinary,
	}
}

// statusOf picks one provider's entry out of the reported list.
func statusOf(t *testing.T, list []Status, provider string) Status {
	t.Helper()
	for _, s := range list {
		if s.Provider == provider {
			return s
		}
	}
	t.Fatalf("Status() = %+v, missing an entry for %q", list, provider)
	return Status{}
}

func TestStatus(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", writeClaudeState(t, `{"plugins":{"lich@lich-plugin":[{"scope":"user","version":"0.12.0-rc.3"}]}}`))

	s := serveBody(t, http.StatusOK, `[{"tag_name":"v0.12.0"}]`)
	// Codex's version comes from its CLI, which is not spawned here; only the
	// Claude Code entry is asserted on.
	s.lookPath = func(name string) (string, error) {
		if name == "claude" {
			return "/usr/bin/claude", nil
		}
		return "", errors.New("not found")
	}

	want := Status{
		Provider: providers.Claude, Name: "Claude Code", Available: true,
		Installed: true, InstalledVersion: "0.12.0-rc.3", LatestVersion: "0.12.0", UpdateAvailable: true,
		Compatible: true,
	}
	if got := statusOf(t, s.Status(), providers.Claude); got != want {
		t.Fatalf("Status() = %+v, want %+v", got, want)
	}
}

// TestStatusListsEveryHarness proves the report covers the providers that can
// run the plugin and nothing else: an entry for one that cannot would put an
// install button on a CLI with nothing to install.
func TestStatusListsEveryHarness(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	s := serveBody(t, http.StatusOK, `[{"tag_name":"v0.12.0"}]`)
	s.lookPath = func(string) (string, error) { return "", errors.New("not found") }

	var got []string
	for _, entry := range s.Status() {
		got = append(got, entry.Provider)
	}
	want := []string{
		providers.Claude, providers.Codex, providers.Antigravity,
		providers.OpenCode, providers.OMP, providers.Crush, providers.Cursor,
		providers.Kiro,
	}
	if !slices.Equal(got, want) {
		t.Fatalf("Status() covers %v, want %v", got, want)
	}
}

// TestStatusWithoutTheCLI proves a provider the machine does not have is
// reported as unavailable rather than omitted — and that its plugin state is
// never read, since there is no CLI to have installed anything.
func TestStatusWithoutTheCLI(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", writeClaudeState(t, `{"plugins":{"lich@lich-plugin":[{"scope":"user","version":"0.5.0"}]}}`))
	s := serveBody(t, http.StatusOK, `[{"tag_name":"v0.5.0"}]`)
	s.lookPath = func(string) (string, error) { return "", errors.New("not found") }

	got := statusOf(t, s.Status(), providers.Claude)
	if got.Available || got.Installed || got.InstalledVersion != "" {
		t.Fatalf("Status() = %+v, want an unavailable provider with no install", got)
	}
}

func TestStatusNotInstalled(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	s := serveBody(t, http.StatusOK, `[{"tag_name":"v0.12.0"}]`)

	got := statusOf(t, s.Status(), providers.Claude)
	if got.Installed || got.UpdateAvailable {
		t.Fatalf("Status() = %+v, want not installed and no update", got)
	}
	if got.LatestVersion != "0.12.0" {
		t.Fatalf("LatestVersion = %q, want %q", got.LatestVersion, "0.12.0")
	}
}

// TestUnknownProviderIsRefused proves a kind with no plugin never reaches a CLI
// or a file: a shell session has no marketplace to add and no plugin directory
// to write into, so spawning it with the plugin's arguments would be nonsense.
func TestUnknownProviderIsRefused(t *testing.T) {
	s, calls := fakeCLI(t, "")
	for _, run := range []func(string) error{s.Install, s.Update} {
		if err := run("shell"); err == nil {
			t.Error("want an error for a kind with no plugin, got nil")
		}
	}
	if got := calls(); len(got) > 0 {
		t.Errorf("calls = %v, want none", got)
	}
}

// The plugin CLI calls are the supported interface for every mutation this
// package makes, so nothing below can be asserted without spawning something.
// The test binary doubles as that something: TestMain hands control to a fake
// CLI when the guard variable is set, which is how the child tells its two roles
// apart. Portable by construction — no shell script, no per-OS fixture.
const (
	fakeCLIGuard = "LICH_TEST_FAKE_CLI"
	fakeCLILog   = "LICH_TEST_FAKE_CLI_LOG"
	fakeCLIFail  = "LICH_TEST_FAKE_CLI_FAIL"
	// The two questions lich asks Crush rather than tells it, answered from the
	// environment so a test decides what the machine's Crush says.
	fakeCLIDirs    = "LICH_TEST_FAKE_CLI_DIRS"
	fakeCLIVersion = "LICH_TEST_FAKE_CLI_VERSION"
)

func TestMain(m *testing.M) {
	if os.Getenv(fakeCLIGuard) == "" {
		os.Exit(m.Run())
	}
	// Recording before the exit code: a call that fails still has to show which
	// call it was.
	if log := os.Getenv(fakeCLILog); log != "" {
		line := strings.Join(os.Args[1:], " ") + "\n"
		f, err := os.OpenFile(log, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			os.Exit(2)
		}
		_, _ = f.WriteString(line)
		_ = f.Close()
	}
	if failOn := os.Getenv(fakeCLIFail); failOn != "" && strings.Contains(strings.Join(os.Args[1:], " "), failOn) {
		fmt.Fprintln(os.Stderr, "fake cli: refusing "+failOn)
		os.Exit(1)
	}
	switch strings.Join(os.Args[1:], " ") {
	case "dirs":
		if dirs := os.Getenv(fakeCLIDirs); dirs != "" {
			fmt.Println(dirs)
		}
	case "--version":
		if v := os.Getenv(fakeCLIVersion); v != "" {
			fmt.Println("crush version v" + v)
		}
	}
	os.Exit(0)
}

// stubBin is a BinResolver naming a fixed binary for every provider, standing in
// for the store.
type stubBin string

func (b stubBin) ProviderBin(string, string) string { return string(b) }

// fakeCLI points a Service's shell-out at the test binary's fake CLI and returns
// a reader of the calls it received. failOn, when non-empty, makes the fake fail
// any call whose arguments contain it.
func fakeCLI(t *testing.T, failOn string) (*Service, func() []string) {
	t.Helper()
	log := filepath.Join(t.TempDir(), "calls.log")
	t.Setenv(fakeCLIGuard, "1")
	t.Setenv(fakeCLILog, log)
	t.Setenv(fakeCLIFail, failOn)

	self, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve test binary: %v", err)
	}
	calls := func() []string {
		data, err := os.ReadFile(log)
		if err != nil {
			return nil
		}
		return strings.Split(strings.TrimSpace(string(data)), "\n")
	}
	release := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `[{"tag_name":"v`+PluginVersionCeiling+`"},{"tag_name":"v`+testVersion+`"}]`)
	}))
	t.Cleanup(release.Close)
	return &Service{
		bins: stubBin(self), lichBin: lichBinary,
		http: release.Client(), releasesURL: release.URL,
	}, calls
}

// pinnedCalls is what an install or an update runs through a harness's plugin
// CLI: whatever marketplace was declared goes, the marketplace comes back at the
// tag of the newest compatible release (testVersion; fakeCLI's release list also
// carries one past the ceiling), and the plugin is installed from it under the
// key that harness stores it under.
var pinnedCalls = map[string][]string{
	providers.Claude: {
		"plugin marketplace remove " + marketplaceName,
		"plugin marketplace add " + gitURL + "#v" + testVersion,
		"plugin install " + pluginKey,
	},
	providers.Codex: {
		"plugin marketplace remove " + marketplaceName,
		"plugin marketplace add " + marketplaceRepo + " --ref v" + testVersion,
		"plugin add " + pluginKey,
	},
}

// TestInstallPinsTheMarketplace pins the order and the exact targets per
// harness, for Install and Update alike: an unpinned marketplace is one the
// harness's own refresh can move past the contract this lich speaks.
func TestInstallPinsTheMarketplace(t *testing.T) {
	for provider, want := range pinnedCalls {
		for name, call := range map[string]func(*Service, string) error{
			"install": (*Service).Install,
			"update":  (*Service).Update,
		} {
			t.Run(provider+" "+name, func(t *testing.T) {
				s, calls := fakeCLI(t, "")
				if err := call(s, provider); err != nil {
					t.Fatalf("%s: %v", name, err)
				}
				if got := calls(); !slices.Equal(got, want) {
					t.Errorf("calls = %v, want %v", got, want)
				}
			})
		}
	}
}

// TestInstallSurvivesANothingToRemove proves the first install: a remove with no
// marketplace declared fails, and that failure must not cost the install.
func TestInstallSurvivesANothingToRemove(t *testing.T) {
	for provider, want := range pinnedCalls {
		t.Run(provider, func(t *testing.T) {
			s, calls := fakeCLI(t, "marketplace remove")
			if err := s.Install(provider); err != nil {
				t.Fatalf("Install with nothing to remove: %v", err)
			}
			if got := calls(); !slices.Equal(got, want) {
				t.Errorf("calls = %v, want %v", got, want)
			}
		})
	}
}

// TestInstallReportsTheInstallFailure proves the other half: when the install
// itself fails, the error carries both the call and what the CLI said, which is
// all the settings screen has to show.
func TestInstallReportsTheInstallFailure(t *testing.T) {
	s, _ := fakeCLI(t, "plugin install")

	err := s.Install(providers.Claude)
	if err == nil {
		t.Fatal("Install: want an error, got nil")
	}
	for _, want := range []string{"plugin install " + pluginKey, "refusing"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q missing %q", err, want)
		}
	}
}

// TestMarketplaceAddFailureStopsTheRun is the regression this package exists to
// prevent: a marketplace that could not be declared at the pinned tag must
// surface as an error, never as a plugin call that reads whatever clone is left
// and exits 0 as a successful install.
func TestMarketplaceAddFailureStopsTheRun(t *testing.T) {
	for provider := range pinnedCalls {
		t.Run(provider, func(t *testing.T) {
			s, calls := fakeCLI(t, "marketplace add")

			err := s.Install(provider)
			if err == nil || !strings.Contains(err.Error(), "plugin marketplace add") {
				t.Fatalf("Install = %v, want the error to name the marketplace add", err)
			}
			for _, call := range calls() {
				if strings.HasPrefix(call, "plugin install") || strings.HasPrefix(call, "plugin add") {
					t.Fatalf("calls = %v, want no plugin call after a failed add", calls())
				}
			}
		})
	}
}

// TestPinnedInstallNeedsARelease proves an install that cannot name a
// compatible release touches nothing: removing the marketplace first would
// uninstall the plugin with nothing to put back.
func TestPinnedInstallNeedsARelease(t *testing.T) {
	for provider := range pinnedCalls {
		t.Run(provider, func(t *testing.T) {
			s, calls := fakeCLI(t, "")
			s.releasesURL = "http://127.0.0.1:0/unreachable"
			if err := s.Install(provider); err == nil {
				t.Fatal("Install: want an error without a release, got nil")
			}
			if got := calls(); len(got) != 0 {
				t.Fatalf("calls = %v, want none", got)
			}
		})
	}
}

// TestRunFallsBackToPath proves an unset binary override spawns the provider's
// default name rather than an empty command — the store answers "" for every
// project that configured no path.
func TestRunFallsBackToPath(t *testing.T) {
	s := &Service{bins: stubBin("")}
	err := s.run(providers.Codex, "plugin", "list", "--json")
	if err == nil {
		return // a machine with Codex installed: the call really ran
	}
	if !strings.Contains(err.Error(), "codex plugin list") {
		t.Errorf("error %q does not name the codex call", err)
	}
}

// Whether a session can answer with a tool is two different facts. Claude Code
// and Codex are told about lich's server on their own command line at spawn, so
// they always can; opencode and Crush only get the operations with the plugin,
// and only from the release that carries them.

func TestHasToolsIsAlwaysTrueForTheHarnessesToldAtSpawn(t *testing.T) {
	svc := New(stubBins{})

	for _, id := range []string{providers.Claude, providers.Codex} {
		if !svc.HasTools(id) {
			t.Errorf("%s cannot answer with a tool, but lich registers its server at spawn", id)
		}
	}
}

// Which providers depend on a registration lich writes with its own path: the
// ones whose installs leave that registration out — or refuse outright — when
// the path cannot be resolved (crushrcBlock, mcpDocument,
// antigravityRegisterMCP). Pinned as a list rather than
// derived, because a provider added to one side and not the other is exactly the
// drift that promises tools nobody registered.
func TestRegistersServerAtInstall(t *testing.T) {
	for _, provider := range []string{
		providers.Crush, providers.OMP, providers.Antigravity, providers.Cursor,
	} {
		if !registersServerAtInstall(provider) {
			t.Errorf("%s writes lich's server at install, but is not treated as doing so", provider)
		}
	}
	for _, provider := range []string{providers.Claude, providers.Codex, providers.OpenCode} {
		if registersServerAtInstall(provider) {
			t.Errorf("%s does not get its tools from a registration lich writes", provider)
		}
	}
}

// And the decision that rule feeds: an installed oh-my-pi whose lich cannot name
// its own binary has the plugin's reports and no tool list, so a relayed message
// must name the shell command instead of a tool that is not there.
func TestHasToolsIsFalseWhenTheServerCouldNotBeRegistered(t *testing.T) {
	agent := ompHome(t)
	writeMarkedModule(t, filepath.Join(agent, ompExtensionsDir, ompFile), toolsMinVersion)
	s := New(stubBins{})
	s.lookPath = func(string) (string, error) { return "/usr/bin/omp", nil }

	if !s.HasTools(providers.OMP) {
		t.Fatal("HasTools(omp) = false with the plugin installed and a binary to register")
	}

	s.lichBin = func() string { return "" }
	if s.HasTools(providers.OMP) {
		t.Error("HasTools(omp) = true, but no server could be registered for it")
	}
}

// opencode is the other side of that rule: its plugin defines the tools itself,
// so there is no binary to name and an unresolvable one changes nothing.
func TestHasToolsIgnoresTheBinaryForOpencode(t *testing.T) {
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	writeMarkedModule(t, filepath.Join(config, "opencode", "plugin", opencodeFile), toolsMinVersion)
	s := New(stubBins{})
	s.lookPath = func(string) (string, error) { return "/usr/bin/opencode", nil }
	s.lichBin = func() string { return "" }

	if !s.HasTools(providers.OpenCode) {
		t.Error("HasTools(opencode) = false, but its tools come from the plugin, not from a server")
	}
}

// writeMarkedModule lays down what an install of that version leaves behind: the
// module with lich's marker line, which is where the version is read back from.
func writeMarkedModule(t *testing.T, path, version string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	marker := jsComment + " " + markerName + " v" + version + " — installed by lich\n"
	if err := os.WriteFile(path, []byte(marker), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestHasToolsFollowsThePluginVersionElsewhere(t *testing.T) {
	for _, tt := range []struct {
		name      string
		installed string
		want      bool
	}{
		// Pinned as literals rather than read off toolsMinVersion: the number is
		// a contract with a released plugin, and a test that moved with the
		// constant would follow it anywhere.
		{name: "the release that brought them", installed: "0.9.0", want: true},
		{name: "a newer one", installed: "0.10.0", want: true},
		{name: "the release before", installed: "0.8.0"},
		{name: "nothing installed"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			config := t.TempDir()
			t.Setenv("XDG_CONFIG_HOME", config)
			if tt.installed != "" {
				// What an install of that version leaves behind: the module with
				// lich's marker line, which is where the version is read from.
				dir := filepath.Join(config, "opencode", "plugin")
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				marker := jsComment + " " + markerName + " v" + tt.installed + " — installed by lich\n"
				if err := os.WriteFile(filepath.Join(dir, opencodeFile), []byte(marker), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			svc := New(stubBins{})
			svc.lookPath = func(string) (string, error) { return "/usr/bin/opencode", nil }

			if got := svc.HasTools(providers.OpenCode); got != tt.want {
				t.Errorf("HasTools with %q installed = %v, want %v", tt.installed, got, tt.want)
			}
		})
	}
}

// Installed is the question the relay asks before reading a session's silence as
// an answer (relay.reportsState), so each way of answering "no" has to be its
// own: a provider with no plugin at all, a CLI this machine does not have, and
// an installed harness whose plugin was never written.
func TestInstalledAnswersForEachWayThereIsNoPlugin(t *testing.T) {
	agent := ompHome(t)
	s := New(stubBins{})
	s.lookPath = func(string) (string, error) { return "/usr/bin/omp", nil }

	if s.Installed(providers.OMP) {
		t.Error("Installed(omp) = true with nothing in its extensions directory")
	}

	writeMarkedModule(t, filepath.Join(agent, ompExtensionsDir, ompFile), "0.1.0")
	if !s.Installed(providers.OMP) {
		t.Error("Installed(omp) = false with the plugin's module in place")
	}
	// Written older than toolsMinVersion on purpose: reporting state and carrying
	// lich's operations are two questions, and only the second weighs a version.
	if s.HasTools(providers.OMP) {
		t.Error("HasTools(omp) = true for a plugin older than the release that carries the tools")
	}

	s.lookPath = func(string) (string, error) { return "", errors.New("not found") }
	if s.Installed(providers.OMP) {
		t.Error("Installed(omp) = true on a machine with no omp CLI to run it")
	}

	if s.Installed("shell") {
		t.Error(`Installed("shell") = true, but a session kind that is no provider has no plugin`)
	}
}

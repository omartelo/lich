package claudeplugin

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
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

func TestComputeStatus(t *testing.T) {
	tests := []struct {
		name         string
		installed    bool
		installedVer string
		latestVer    string
		wantUpdate   bool
	}{
		{"not installed", false, "", "0.0.2", false},
		{"installed, no latest known", true, "0.0.1", "", false},
		{"update available", true, "0.0.1", "0.0.2", true},
		{"already latest", true, "0.0.2", "0.0.2", false},
		{"installed newer than latest", true, "0.1.0", "0.0.2", false},
		{"pre-release install sees the stable release", true, "0.2.0-rc.3", "0.2.0", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := computeStatus(tc.installed, tc.installedVer, tc.latestVer)
			if got.UpdateAvailable != tc.wantUpdate {
				t.Fatalf("UpdateAvailable = %v, want %v", got.UpdateAvailable, tc.wantUpdate)
			}
			if got.Installed != tc.installed || got.InstalledVersion != tc.installedVer || got.LatestVersion != tc.latestVer {
				t.Fatalf("status = %+v, mismatch on passthrough fields", got)
			}
		})
	}
}

func TestInstalledVersionReadsConfigDir(t *testing.T) {
	dir := t.TempDir()
	pluginsDir := filepath.Join(dir, "plugins")
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"plugins":{"lich@lich-plugin":[{"scope":"user","version":"0.3.1"}]}}`
	if err := os.WriteFile(filepath.Join(pluginsDir, "installed_plugins.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &Service{configDir: dir}
	ver, ok := s.installedVersion()
	if !ok || ver != "0.3.1" {
		t.Fatalf("got (%q,%v), want (0.3.1,true)", ver, ok)
	}
}

func TestInstalledVersionMissingFile(t *testing.T) {
	s := &Service{configDir: t.TempDir()}
	if ver, ok := s.installedVersion(); ok || ver != "" {
		t.Fatalf("got (%q,%v), want empty/false for missing file", ver, ok)
	}
}

func TestInstalledVersionNoConfigDir(t *testing.T) {
	s := &Service{configDir: ""}
	if ver, ok := s.installedVersion(); ok || ver != "" {
		t.Fatalf("got (%q,%v), want empty/false without a config dir", ver, ok)
	}
}

type stubBins struct{ bin string }

func (s stubBins) ClaudeBin(string) string { return s.bin }

func TestNewDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", dir)

	s := New(stubBins{bin: "claude"})
	if s.latestURL != latestReleaseURL {
		t.Errorf("latestURL = %q, want %q", s.latestURL, latestReleaseURL)
	}
	if s.http == nil || s.http.Timeout != httpTimeout {
		t.Errorf("http client = %+v, want one with a %v timeout", s.http, httpTimeout)
	}
	if s.configDir != dir {
		t.Errorf("configDir = %q, want %q", s.configDir, dir)
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
// pointed at it.
func serveBody(t *testing.T, status int, body string) *Service {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return &Service{http: srv.Client(), latestURL: srv.URL}
}

func TestStatus(t *testing.T) {
	dir := t.TempDir()
	pluginsDir := filepath.Join(dir, "plugins")
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"plugins":{"lich@lich-plugin":[{"scope":"user","version":"0.2.0-rc.3"}]}}`
	if err := os.WriteFile(filepath.Join(pluginsDir, "installed_plugins.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	s := serveBody(t, http.StatusOK, `{"tag_name":"v0.2.0"}`)
	s.configDir = dir

	want := Status{Installed: true, InstalledVersion: "0.2.0-rc.3", LatestVersion: "0.2.0", UpdateAvailable: true}
	if got := s.Status(); got != want {
		t.Fatalf("Status() = %+v, want %+v", got, want)
	}
}

func TestStatusNotInstalled(t *testing.T) {
	s := serveBody(t, http.StatusOK, `{"tag_name":"v0.2.0"}`)
	s.configDir = t.TempDir()

	got := s.Status()
	if got.Installed || got.UpdateAvailable {
		t.Fatalf("Status() = %+v, want not installed and no update", got)
	}
	if got.LatestVersion != "0.2.0" {
		t.Fatalf("LatestVersion = %q, want %q", got.LatestVersion, "0.2.0")
	}
}

// The `claude plugin` calls are the supported interface for every mutation this
// package makes, so nothing below can be asserted without spawning something.
// The test binary doubles as that something: TestMain hands control to a fake
// CLI when the guard variable is set, which is how the child tells its two roles
// apart. Portable by construction — no shell script, no per-OS fixture.
const (
	fakeClaudeGuard = "LICH_TEST_FAKE_CLAUDE"
	fakeClaudeLog   = "LICH_TEST_FAKE_CLAUDE_LOG"
	fakeClaudeFail  = "LICH_TEST_FAKE_CLAUDE_FAIL"
)

func TestMain(m *testing.M) {
	if os.Getenv(fakeClaudeGuard) == "" {
		os.Exit(m.Run())
	}
	// Recording before the exit code: a call that fails still has to show which
	// call it was.
	if log := os.Getenv(fakeClaudeLog); log != "" {
		line := strings.Join(os.Args[1:], " ") + "\n"
		f, err := os.OpenFile(log, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			os.Exit(2)
		}
		_, _ = f.WriteString(line)
		_ = f.Close()
	}
	if failOn := os.Getenv(fakeClaudeFail); failOn != "" && strings.Contains(strings.Join(os.Args[1:], " "), failOn) {
		fmt.Fprintln(os.Stderr, "fake claude: refusing "+failOn)
		os.Exit(1)
	}
	os.Exit(0)
}

// stubBin is a BinResolver naming a fixed binary, standing in for the store.
type stubBin string

func (b stubBin) ClaudeBin(string) string { return string(b) }

// fakeClaude points a Service's shell-out at the test binary's fake CLI and
// returns a reader of the calls it received. failOn, when non-empty, makes the
// fake fail any call whose arguments contain it.
func fakeClaude(t *testing.T, failOn string) (*Service, func() []string) {
	t.Helper()
	log := filepath.Join(t.TempDir(), "calls.log")
	t.Setenv(fakeClaudeGuard, "1")
	t.Setenv(fakeClaudeLog, log)
	t.Setenv(fakeClaudeFail, failOn)

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
	return &Service{bins: stubBin(self)}, calls
}

// TestInstallAddsMarketplaceThenPlugin pins the order and the exact targets: the
// marketplace has to be added before the plugin can resolve, and both name the
// keys Claude Code stores the plugin under.
func TestInstallAddsMarketplaceThenPlugin(t *testing.T) {
	s, calls := fakeClaude(t, "")

	if err := s.Install(); err != nil {
		t.Fatalf("Install: %v", err)
	}
	want := []string{
		"plugin marketplace add " + marketplaceRepo,
		"plugin install " + pluginKey,
	}
	if got := calls(); !slices.Equal(got, want) {
		t.Errorf("calls = %v, want %v", got, want)
	}
}

// TestInstallSurvivesARepeatMarketplaceAdd proves the documented tolerance: a
// marketplace already present makes `marketplace add` fail, and that failure
// must not cost the install that follows it — it only redirects the refresh to
// the existing clone.
func TestInstallSurvivesARepeatMarketplaceAdd(t *testing.T) {
	s, calls := fakeClaude(t, "marketplace add")

	if err := s.Install(); err != nil {
		t.Fatalf("Install after a repeat marketplace add: %v", err)
	}
	want := []string{
		"plugin marketplace add " + marketplaceRepo,
		"plugin marketplace update " + marketplaceName,
		"plugin install " + pluginKey,
	}
	if got := calls(); !slices.Equal(got, want) {
		t.Errorf("calls = %v, want %v", got, want)
	}
}

// TestInstallReportsTheInstallFailure proves the other half: when the install
// itself fails, the error carries both the call and what the CLI said, which is
// all the settings screen has to show.
func TestInstallReportsTheInstallFailure(t *testing.T) {
	s, _ := fakeClaude(t, "plugin install")

	err := s.Install()
	if err == nil {
		t.Fatal("Install: want an error, got nil")
	}
	for _, want := range []string{"plugin install " + pluginKey, "refusing"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q missing %q", err, want)
		}
	}
}

// TestUpdateTargetsThePluginKey proves Update names the same key Status reads
// the installed version under — a mismatch would silently update nothing.
func TestUpdateTargetsThePluginKey(t *testing.T) {
	s, calls := fakeClaude(t, "")

	if err := s.Update(); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got := calls(); got[len(got)-1] != "plugin update "+pluginKey {
		t.Errorf("calls = %v, want the last one to be the plugin update", got)
	}
}

// TestUpdateRefreshesTheMarketplaceFirst pins the order on the real-world path,
// where the marketplace is already present: the clone is refreshed before the
// plugin update reads a version off it. Reversed, the update reads the stale
// clone and reports the version it already has as the newest one.
func TestUpdateRefreshesTheMarketplaceFirst(t *testing.T) {
	s, calls := fakeClaude(t, "marketplace add")

	if err := s.Update(); err != nil {
		t.Fatalf("Update: %v", err)
	}
	want := []string{
		"plugin marketplace add " + marketplaceRepo,
		"plugin marketplace update " + marketplaceName,
		"plugin update " + pluginKey,
	}
	if got := calls(); !slices.Equal(got, want) {
		t.Errorf("calls = %v, want %v", got, want)
	}
}

// TestMarketplaceRefreshFailureStopsTheRun is the regression this package exists
// to prevent: an unrefreshable marketplace must surface as an error, never as a
// plugin call that exits 0 on the stale clone and reads as a successful update.
func TestMarketplaceRefreshFailureStopsTheRun(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func(*Service) error
	}{
		{"install", (*Service).Install},
		{"update", (*Service).Update},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// "marketplace" fails both the add and the update, leaving the clone
			// as it was; "plugin install"/"plugin update" still match nothing.
			s, calls := fakeClaude(t, "marketplace")

			err := tc.run(s)
			if err == nil {
				t.Fatal("want an error when the marketplace cannot be refreshed, got nil")
			}
			if !strings.Contains(err.Error(), "plugin marketplace update "+marketplaceName) {
				t.Errorf("error %q does not name the refresh that failed", err)
			}
			for _, call := range calls() {
				if strings.HasPrefix(call, "plugin install") || strings.HasPrefix(call, "plugin update") {
					t.Errorf("calls = %v, want no plugin call after a failed refresh", calls())
					break
				}
			}
		})
	}
}

// TestRunClaudeFallsBackToPath proves an unset binary override spawns plain
// "claude" rather than an empty command — the store answers "" for every
// project that configured no path.
func TestRunClaudeFallsBackToPath(t *testing.T) {
	s := &Service{bins: stubBin("")}
	err := s.runClaude("plugin", "update", pluginKey)
	if err == nil {
		return // a machine with Claude Code installed: the call really ran
	}
	if !strings.Contains(err.Error(), "claude plugin update") {
		t.Errorf("error %q does not name the claude call", err)
	}
}

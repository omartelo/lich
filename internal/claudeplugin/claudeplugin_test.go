package claudeplugin

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

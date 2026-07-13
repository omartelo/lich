package claudeplugin

import (
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

func TestParseLatestTag(t *testing.T) {
	tests := []struct {
		name string
		json string
		want string
	}{
		{"v prefix", `{"tag_name":"v0.2.0"}`, "0.2.0"},
		{"no prefix", `{"tag_name":"1.4.2"}`, "1.4.2"},
		{"missing", `{"name":"x"}`, ""},
		{"malformed", `not json`, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseLatestTag([]byte(tc.json)); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSemverLess(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"0.0.1", "0.0.2", true},
		{"0.0.1", "0.1.0", true},
		{"0.9.9", "1.0.0", true},
		{"1.0.0", "1.0.0", false},
		{"1.2.0", "1.1.9", false},
		{"v1.0.0", "v1.0.1", true},
		{"1.0.0-rc1", "1.0.0", false},
		{"1.0", "1.0.1", true},
	}
	for _, tc := range tests {
		if got := semverLess(tc.a, tc.b); got != tc.want {
			t.Errorf("semverLess(%q,%q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
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

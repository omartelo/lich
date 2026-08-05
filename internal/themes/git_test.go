package themes

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	// The repository is a fixture, so it must not inherit the developer's
	// identity or commit signing: a machine with gpgsign on would fail here.
	base := []string{
		"-c", "user.email=tests@lich.invalid",
		"-c", "user.name=lich tests",
		"-c", "commit.gpgsign=false",
		"-c", "init.defaultBranch=main",
	}
	cmd := exec.Command("git", append(base, args...)...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
}

func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func manifestJSON(t *testing.T, name, version string) string {
	t.Helper()
	data, err := json.Marshal(Manifest{Name: name, Version: version})
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	return string(data)
}

// packDir writes a theme repository without a git history — enough for the
// readPack rules, which never touch git.
func packDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		writeFile(t, dir, name, body)
	}
	return dir
}

// themeRepo turns a set of files into a committed git repository and returns
// its path, which doubles as the clone URL.
func themeRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	requireGit(t)
	dir := packDir(t, files)
	git(t, dir, "init")
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-m", "themes")
	return dir
}

func commitAll(t *testing.T, dir, message string) {
	t.Helper()
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-m", message)
}

func TestInstallFromGitInstallsEveryThemeInThePack(t *testing.T) {
	dir := t.TempDir()
	s := NewInDir(dir)
	repo := themeRepo(t, map[string]string{
		manifestName:       manifestJSON(t, "Sample pack", "1.2.0"),
		"tokyo-night.json": rawTheme(t, customTheme("tokyo-night")),
		"gruvbox.json":     rawTheme(t, customTheme("gruvbox")),
	})

	result, err := s.InstallFromGit(repo, false)
	if err != nil {
		t.Fatalf("InstallFromGit: %v", err)
	}
	if result.Pack != "Sample pack" || result.Version != "1.2.0" {
		t.Fatalf("result = %#v", result)
	}
	if len(result.Themes) != 2 || len(result.Conflicts) != 0 {
		t.Fatalf("installed %d themes, %d conflicts", len(result.Themes), len(result.Conflicts))
	}
	for _, theme := range result.Themes {
		if theme.Source == nil || theme.Source.URL != repo || theme.Source.Version != "1.2.0" {
			t.Fatalf("theme %q source = %#v", theme.ID, theme.Source)
		}
	}
	listed, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	// Two bundled themes plus the pack.
	if len(listed) != 4 {
		t.Fatalf("got %d themes, want 4", len(listed))
	}
	stored, err := s.read("gruvbox")
	if err != nil {
		t.Fatalf("read stored theme: %v", err)
	}
	if stored.Source == nil || stored.Source.Version != "1.2.0" {
		t.Fatalf("stored source = %#v", stored.Source)
	}
}

func TestInstallFromGitRejectsPackWithAnInvalidTheme(t *testing.T) {
	dir := t.TempDir()
	s := NewInDir(dir)
	broken := customTheme("broken")
	delete(broken.App, "background")
	repo := themeRepo(t, map[string]string{
		manifestName:  manifestJSON(t, "Sample pack", "1.0.0"),
		"good.json":   rawTheme(t, customTheme("good")),
		"broken.json": rawTheme(t, broken),
	})

	_, err := s.InstallFromGit(repo, false)
	if err == nil {
		t.Fatal("InstallFromGit accepted a pack holding an invalid theme")
	}
	if !strings.Contains(err.Error(), "broken.json") {
		t.Fatalf("error does not name the file: %v", err)
	}
	// All-or-nothing: the valid sibling must not have been written either.
	if _, err := os.Stat(filepath.Join(dir, "good.json")); !os.IsNotExist(err) {
		t.Fatalf("valid theme was installed from a rejected pack: %v", err)
	}
}

func TestInstallFromGitReportsConflictsBeforeWriting(t *testing.T) {
	dir := t.TempDir()
	s := NewInDir(dir)
	existing := customTheme("tokyo-night")
	existing.Name = "Mine"
	if _, err := importTheme(t, s, existing); err != nil {
		t.Fatalf("seed import: %v", err)
	}
	repo := themeRepo(t, map[string]string{
		manifestName:       manifestJSON(t, "Sample pack", "1.0.0"),
		"tokyo-night.json": rawTheme(t, customTheme("tokyo-night")),
	})

	result, err := s.InstallFromGit(repo, false)
	if err != nil {
		t.Fatalf("InstallFromGit: %v", err)
	}
	if len(result.Conflicts) != 1 || result.Conflicts[0] != "tokyo-night" {
		t.Fatalf("conflicts = %#v", result.Conflicts)
	}
	if len(result.Themes) != 0 {
		t.Fatalf("themes were installed despite the conflict: %#v", result.Themes)
	}
	kept, err := s.read("tokyo-night")
	if err != nil {
		t.Fatalf("read kept theme: %v", err)
	}
	if kept.Name != "Mine" || kept.Source != nil {
		t.Fatalf("existing theme was replaced: %#v", kept)
	}

	replaced, err := s.InstallFromGit(repo, true)
	if err != nil {
		t.Fatalf("InstallFromGit overwrite: %v", err)
	}
	if len(replaced.Themes) != 1 {
		t.Fatalf("overwrite installed %d themes", len(replaced.Themes))
	}
	updated, err := s.read("tokyo-night")
	if err != nil {
		t.Fatalf("read replaced theme: %v", err)
	}
	if updated.Source == nil || updated.Source.URL != repo {
		t.Fatalf("replaced theme source = %#v", updated.Source)
	}
}

func TestUpdateFromGitInstallsANewerManifest(t *testing.T) {
	s := NewInDir(t.TempDir())
	repo := themeRepo(t, map[string]string{
		manifestName:       manifestJSON(t, "Sample pack", "1.0.0"),
		"tokyo-night.json": rawTheme(t, customTheme("tokyo-night")),
	})
	if _, err := s.InstallFromGit(repo, false); err != nil {
		t.Fatalf("InstallFromGit: %v", err)
	}

	renamed := customTheme("tokyo-night")
	renamed.Name = "Tokyo Night II"
	writeFile(t, repo, manifestName, manifestJSON(t, "Sample pack", "1.1.0"))
	writeFile(t, repo, "tokyo-night.json", rawTheme(t, renamed))
	commitAll(t, repo, "bump")

	result, err := s.UpdateFromGit("tokyo-night")
	if err != nil {
		t.Fatalf("UpdateFromGit: %v", err)
	}
	if result.UpToDate || result.Version != "1.1.0" || len(result.Themes) != 1 {
		t.Fatalf("result = %#v", result)
	}
	stored, err := s.read("tokyo-night")
	if err != nil {
		t.Fatalf("read updated theme: %v", err)
	}
	if stored.Name != "Tokyo Night II" || stored.Source.Version != "1.1.0" {
		t.Fatalf("stored theme = %#v", stored)
	}
}

func TestUpdateFromGitKeepsAnUpToDatePack(t *testing.T) {
	s := NewInDir(t.TempDir())
	repo := themeRepo(t, map[string]string{
		manifestName:       manifestJSON(t, "Sample pack", "1.0.0"),
		"tokyo-night.json": rawTheme(t, customTheme("tokyo-night")),
	})
	if _, err := s.InstallFromGit(repo, false); err != nil {
		t.Fatalf("InstallFromGit: %v", err)
	}

	result, err := s.UpdateFromGit("tokyo-night")
	if err != nil {
		t.Fatalf("UpdateFromGit: %v", err)
	}
	if !result.UpToDate || len(result.Themes) != 0 {
		t.Fatalf("result = %#v", result)
	}
}

func TestUpdateFromGitRejectsAFileImportedTheme(t *testing.T) {
	s := NewInDir(t.TempDir())
	if _, err := importTheme(t, s, customTheme("standalone")); err != nil {
		t.Fatalf("seed import: %v", err)
	}
	_, err := s.UpdateFromGit("standalone")
	if err == nil {
		t.Fatal("UpdateFromGit accepted a theme with no repository")
	}
	if !strings.Contains(err.Error(), "single file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestImportDropsAClaimedSource(t *testing.T) {
	s := NewInDir(t.TempDir())
	claimed := customTheme("claimed")
	claimed.Source = &Source{URL: "https://example.invalid/themes.git", Version: "9.9.9"}

	result, err := importTheme(t, s, claimed)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if result.Theme.Source != nil {
		t.Fatalf("import kept a claimed source: %#v", result.Theme.Source)
	}
	stored, err := s.read("claimed")
	if err != nil {
		t.Fatalf("read stored theme: %v", err)
	}
	if stored.Source != nil {
		t.Fatalf("stored theme kept a claimed source: %#v", stored.Source)
	}
}

func TestValidateRemoteAllowsOnlyKnownTransports(t *testing.T) {
	absolute := filepath.Join(t.TempDir(), "themes")
	cases := []struct {
		name  string
		url   string
		valid bool
	}{
		{"https", "https://github.com/omartelo/lich-themes.git", true},
		{"ssh", "ssh://git@github.com/omartelo/lich-themes.git", true},
		{"scp shorthand", "git@github.com:omartelo/lich-themes.git", true},
		{"file", "file://" + absolute, true},
		{"absolute path", absolute, true},
		{"remote helper", "ext::sh -c 'touch /tmp/pwned'", false},
		{"flag", "--upload-pack=touch /tmp/pwned", false},
		{"unencrypted git", "git://github.com/omartelo/lich-themes.git", false},
		{"relative path", "../themes", false},
		{"empty", "   ", false},
		{"overlong", "https://example.invalid/" + strings.Repeat("a", remoteMaxLength), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := validateRemote(tc.url)
			if tc.valid && err != nil {
				t.Fatalf("validateRemote(%q) = %v, want accepted", tc.url, err)
			}
			if !tc.valid && err == nil {
				t.Fatalf("validateRemote(%q) accepted a rejected transport", tc.url)
			}
		})
	}
}

func TestInstallFromGitRejectsAnUnsupportedRemote(t *testing.T) {
	s := NewInDir(t.TempDir())
	if _, err := s.InstallFromGit("ext::sh -c 'touch /tmp/pwned'", false); err == nil {
		t.Fatal("InstallFromGit accepted a remote-helper URL")
	}
}

func TestReadPackRejectsMalformedRepositories(t *testing.T) {
	valid := rawTheme(t, customTheme("one"))
	duplicate := rawTheme(t, customTheme("dup"))
	crowded := map[string]string{manifestName: manifestJSON(t, "Crowded", "1.0.0")}
	for i := 0; i <= maxThemesPerPack; i++ {
		crowded[string(rune('a'+i%26))+string(rune('a'+i/26))+".json"] = "{}"
	}
	cases := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{
			name:  "no manifest",
			files: map[string]string{"one.json": valid},
			want:  manifestName,
		},
		{
			name:  "malformed manifest",
			files: map[string]string{manifestName: "{", "one.json": valid},
			want:  "parse",
		},
		{
			name:  "manifest without a release version",
			files: map[string]string{manifestName: manifestJSON(t, "Pack", "1.2"), "one.json": valid},
			want:  "MAJOR.MINOR.PATCH",
		},
		{
			name:  "manifest without a name",
			files: map[string]string{manifestName: manifestJSON(t, "  ", "1.0.0"), "one.json": valid},
			want:  "name is required",
		},
		{
			name:  "no themes",
			files: map[string]string{manifestName: manifestJSON(t, "Pack", "1.0.0")},
			want:  "no theme JSON",
		},
		{
			name: "duplicate id",
			files: map[string]string{
				manifestName: manifestJSON(t, "Pack", "1.0.0"),
				"a.json":     duplicate,
				"b.json":     duplicate,
			},
			want: "more than one file",
		},
		{
			name:  "too many themes",
			files: crowded,
			want:  "more than",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := readPack(packDir(t, tc.files), "https://example.invalid/themes.git")
			if err == nil {
				t.Fatalf("readPack accepted %s", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

func TestReadPackIgnoresNonThemeEntries(t *testing.T) {
	files := map[string]string{
		manifestName: manifestJSON(t, "Pack", "2.0.0"),
		"one.json":   rawTheme(t, customTheme("one")),
		"README.md":  "# themes",
	}
	dir := packDir(t, files)
	if err := os.Mkdir(filepath.Join(dir, "docs"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	pack, err := readPack(dir, "https://example.invalid/themes.git")
	if err != nil {
		t.Fatalf("readPack: %v", err)
	}
	if len(pack.themes) != 1 || pack.themes[0].ID != "one" {
		t.Fatalf("pack = %#v", pack.themes)
	}
}

func TestListKeepsTheSourceOfAnInstalledTheme(t *testing.T) {
	dir := t.TempDir()
	s := NewInDir(dir)
	repo := themeRepo(t, map[string]string{
		manifestName: manifestJSON(t, "Sample pack", "3.1.4"),
		"one.json":   rawTheme(t, customTheme("one")),
	})
	if _, err := s.InstallFromGit(repo, false); err != nil {
		t.Fatalf("InstallFromGit: %v", err)
	}

	listed, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, theme := range listed {
		if theme.ID != "one" {
			continue
		}
		if theme.Source == nil || theme.Source.Version != "3.1.4" || theme.Source.URL != repo {
			t.Fatalf("listed source = %#v", theme.Source)
		}
		return
	}
	t.Fatal("installed theme is missing from List")
}

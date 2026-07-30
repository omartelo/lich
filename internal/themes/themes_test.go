package themes

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func customTheme(id string) Theme {
	theme := cloneTheme(bundledThemes[0])
	theme.ID = id
	theme.Name = "Custom"
	theme.Origin = OriginCustom
	return theme
}

func rawTheme(t *testing.T, theme Theme) string {
	t.Helper()
	data, err := json.Marshal(theme)
	if err != nil {
		t.Fatalf("marshal theme: %v", err)
	}
	return string(data)
}

func TestListIncludesBundledThemes(t *testing.T) {
	s := NewInDir(t.TempDir())
	themes, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(themes) != 2 {
		t.Fatalf("got %d themes, want 2", len(themes))
	}
	if themes[0].ID != "light" || themes[0].Origin != OriginBundled {
		t.Fatalf("first bundled theme = %#v", themes[0])
	}
	if themes[1].ID != "dark" || themes[1].Origin != OriginBundled {
		t.Fatalf("second bundled theme = %#v", themes[1])
	}
}

func TestImportPersistsCustomTheme(t *testing.T) {
	dir := t.TempDir()
	s := NewInDir(dir)
	imported, err := s.Import(rawTheme(t, customTheme("solar")))
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if imported.ID != "solar" || imported.Origin != OriginCustom {
		t.Fatalf("imported = %#v", imported)
	}
	if _, err := os.Stat(filepath.Join(dir, "solar.json")); err != nil {
		t.Fatalf("stat saved theme: %v", err)
	}
	listed, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 3 || listed[2].ID != "solar" {
		t.Fatalf("listed themes = %#v", listed)
	}
}

func TestImportRejectsReservedID(t *testing.T) {
	s := NewInDir(t.TempDir())
	_, err := s.Import(rawTheme(t, customTheme("dark")))
	if err == nil {
		t.Fatal("Import reserved id succeeded")
	}
}

func TestImportRejectsMalformedJSON(t *testing.T) {
	s := NewInDir(t.TempDir())
	if _, err := s.Import("{"); err == nil {
		t.Fatal("Import malformed JSON succeeded")
	}
}

func TestImportRejectsInvalidID(t *testing.T) {
	tests := []struct {
		name string
		id   string
	}{
		{name: "empty", id: ""},
		{name: "uppercase", id: "Bad"},
		{name: "leading dash", id: "-bad"},
		{name: "too long", id: strings.Repeat("a", 65)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewInDir(t.TempDir())
			if _, err := s.Import(rawTheme(t, customTheme(tt.id))); err == nil {
				t.Fatal("Import invalid id succeeded")
			}
		})
	}
}

func TestImportRejectsInvalidMetadata(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Theme)
	}{
		{
			name: "missing name",
			mutate: func(theme *Theme) {
				theme.Name = " "
			},
		},
		{
			name: "invalid scheme",
			mutate: func(theme *Theme) {
				theme.Scheme = "dim"
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			theme := customTheme("invalid-metadata")
			tt.mutate(&theme)
			s := NewInDir(t.TempDir())
			if _, err := s.Import(rawTheme(t, theme)); err == nil {
				t.Fatal("Import invalid metadata succeeded")
			}
		})
	}
}

func TestImportRejectsMissingToken(t *testing.T) {
	theme := customTheme("missing-token")
	delete(theme.App, "background")
	s := NewInDir(t.TempDir())
	_, err := s.Import(rawTheme(t, theme))
	if err == nil {
		t.Fatal("Import without required token succeeded")
	}
}

func TestImportRejectsInvalidColorValues(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Theme)
	}{
		{
			name: "unknown app color",
			mutate: func(theme *Theme) {
				theme.App["unknown"] = "#fff"
			},
		},
		{
			name: "empty app color",
			mutate: func(theme *Theme) {
				theme.App["background"] = " "
			},
		},
		{
			name: "unsafe app color",
			mutate: func(theme *Theme) {
				theme.App["foreground"] = "red; color: blue"
			},
		},
		{
			name: "unknown terminal color",
			mutate: func(theme *Theme) {
				theme.Terminal["unknown"] = "#fff"
			},
		},
		{
			name: "empty terminal color",
			mutate: func(theme *Theme) {
				theme.Terminal["cursor"] = " "
			},
		},
		{
			name: "unsafe terminal color",
			mutate: func(theme *Theme) {
				theme.Terminal["cursor"] = "red{}"
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			theme := customTheme("invalid-color")
			tt.mutate(&theme)
			s := NewInDir(t.TempDir())
			if _, err := s.Import(rawTheme(t, theme)); err == nil {
				t.Fatal("Import invalid color succeeded")
			}
		})
	}
}

func TestImportRejectsMissingTerminalRequiredColors(t *testing.T) {
	for _, key := range []string{"background", "foreground"} {
		t.Run(key, func(t *testing.T) {
			theme := customTheme("missing-terminal")
			delete(theme.Terminal, key)
			s := NewInDir(t.TempDir())
			if _, err := s.Import(rawTheme(t, theme)); err == nil {
				t.Fatal("Import without required terminal color succeeded")
			}
		})
	}
}

func TestImportReturnsDirectoryError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "themes")
	if err := os.WriteFile(path, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("write dir blocker: %v", err)
	}
	s := NewInDir(path)
	if _, err := s.Import(rawTheme(t, customTheme("dir-error"))); err == nil {
		t.Fatal("Import with file as themes dir succeeded")
	}
}

func TestRemoveDeletesOnlyCustomTheme(t *testing.T) {
	dir := t.TempDir()
	s := NewInDir(dir)
	if _, err := s.Import(rawTheme(t, customTheme("removable"))); err != nil {
		t.Fatalf("Import: %v", err)
	}
	if err := s.Remove("removable"); err != nil {
		t.Fatalf("Remove custom: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "removable.json")); !os.IsNotExist(err) {
		t.Fatalf("saved theme still exists: %v", err)
	}
	if err := s.Remove("light"); err == nil {
		t.Fatal("Remove bundled theme succeeded")
	}
}

func TestListSortsCustomThemesByName(t *testing.T) {
	dir := t.TempDir()
	s := NewInDir(dir)
	zeta := customTheme("zeta")
	zeta.Name = "Zeta"
	alpha := customTheme("alpha")
	alpha.Name = "alpha"
	if _, err := s.Import(rawTheme(t, zeta)); err != nil {
		t.Fatalf("Import zeta: %v", err)
	}
	if _, err := s.Import(rawTheme(t, alpha)); err != nil {
		t.Fatalf("Import alpha: %v", err)
	}
	themes, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	got := []string{themes[0].ID, themes[1].ID, themes[2].ID, themes[3].ID}
	want := []string{"light", "dark", "alpha", "zeta"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("theme order = %v, want %v", got, want)
	}
}

func TestListSkipsInvalidCustomTheme(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bad.json"), []byte("{"), 0o600); err != nil {
		t.Fatalf("write bad theme: %v", err)
	}
	s := NewInDir(dir)
	themes, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(themes) != 2 {
		t.Fatalf("got %d themes, want bundled only", len(themes))
	}
}

func TestListSkipsFilenameMismatch(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "wrong.json"), []byte(rawTheme(t, customTheme("right"))), 0o600); err != nil {
		t.Fatalf("write mismatched theme: %v", err)
	}
	s := NewInDir(dir)
	themes, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(themes) != 2 {
		t.Fatalf("got %d themes, want bundled only", len(themes))
	}
}

func TestListReturnsDirectoryError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "themes")
	if err := os.WriteFile(path, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("write dir blocker: %v", err)
	}
	s := NewInDir(path)
	if _, err := s.List(); err == nil {
		t.Fatal("List with file as themes dir succeeded")
	}
}

func TestBundledThemesAreCloned(t *testing.T) {
	s := NewInDir(t.TempDir())
	first, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	first[0].App["background"] = "changed"
	first[0].Terminal["foreground"] = "changed"

	second, err := s.List()
	if err != nil {
		t.Fatalf("List again: %v", err)
	}
	if second[0].App["background"] == "changed" || second[0].Terminal["foreground"] == "changed" {
		t.Fatalf("bundled theme was mutated: %#v", second[0])
	}
}

func TestBundledThemesMatchFrontendJSON(t *testing.T) {
	for _, theme := range bundledThemes {
		t.Run(theme.ID, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "..", "frontend", "src", "themes", theme.ID+".json"))
			if err != nil {
				t.Fatalf("read frontend theme: %v", err)
			}
			var frontend Theme
			if err := json.Unmarshal(data, &frontend); err != nil {
				t.Fatalf("parse frontend theme: %v", err)
			}
			if !reflect.DeepEqual(theme, frontend) {
				t.Fatalf("backend bundled theme differs from frontend JSON\nbackend: %#v\nfrontend: %#v", theme, frontend)
			}
			if !reflect.DeepEqual(keySet(theme.App), appTokens) {
				t.Fatalf("app tokens for %s = %#v, want %#v", theme.ID, keySet(theme.App), appTokens)
			}
		})
	}
}

package themes

import (
	"encoding/json"
	"os"
	"path/filepath"
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

func TestImportRejectsMissingToken(t *testing.T) {
	theme := customTheme("missing-token")
	delete(theme.App, "background")
	s := NewInDir(t.TempDir())
	_, err := s.Import(rawTheme(t, theme))
	if err == nil {
		t.Fatal("Import without required token succeeded")
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

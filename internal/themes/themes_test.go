package themes

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	themeassets "github.com/omartelo/lich/themes"
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

func rawThemeFile(t *testing.T, raw string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "theme.json")
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatalf("write source theme: %v", err)
	}
	return path
}

func importTheme(t *testing.T, s *Service, theme Theme) (ImportResult, error) {
	t.Helper()
	return s.Import(rawThemeFile(t, rawTheme(t, theme)), false)
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
	result, err := importTheme(t, s, customTheme("solar"))
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if result.Theme.ID != "solar" || result.Theme.Origin != OriginCustom || result.NeedsOverwrite {
		t.Fatalf("import result = %#v", result)
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
	_, err := importTheme(t, s, customTheme("dark"))
	if err == nil {
		t.Fatal("Import reserved id succeeded")
	}
}

func TestImportRejectsMalformedJSON(t *testing.T) {
	s := NewInDir(t.TempDir())
	if _, err := s.Import(rawThemeFile(t, "{"), false); err == nil {
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
			if _, err := importTheme(t, s, customTheme(tt.id)); err == nil {
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
		{
			name: "name too long",
			mutate: func(theme *Theme) {
				theme.Name = strings.Repeat("a", themeNameMaxLength+1)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			theme := customTheme("invalid-metadata")
			tt.mutate(&theme)
			s := NewInDir(t.TempDir())
			if _, err := importTheme(t, s, theme); err == nil {
				t.Fatal("Import invalid metadata succeeded")
			}
		})
	}
}

func TestImportRejectsMissingToken(t *testing.T) {
	theme := customTheme("missing-token")
	delete(theme.App, "background")
	s := NewInDir(t.TempDir())
	_, err := importTheme(t, s, theme)
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
			name: "app color naming a resource",
			mutate: func(theme *Theme) {
				theme.App["foreground"] = "url(http://example.test/pixel.png)"
			},
		},
		{
			name: "app color deferring a value",
			mutate: func(theme *Theme) {
				theme.App["foreground"] = "var(--leaked)"
			},
		},
		{
			name: "non-hex terminal color",
			mutate: func(theme *Theme) {
				theme.Terminal["cursor"] = "oklch(0.7 0.1 280)"
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
			if _, err := importTheme(t, s, theme); err == nil {
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
			if _, err := importTheme(t, s, theme); err == nil {
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
	if _, err := importTheme(t, s, customTheme("dir-error")); err == nil {
		t.Fatal("Import with file as themes dir succeeded")
	}
}

func TestImportRequiresOverwriteConfirmation(t *testing.T) {
	dir := t.TempDir()
	s := NewInDir(dir)
	original := customTheme("replace-me")
	original.Name = "Original"
	if _, err := importTheme(t, s, original); err != nil {
		t.Fatalf("import original: %v", err)
	}

	replacement := customTheme("replace-me")
	replacement.Name = "Replacement"
	source := rawThemeFile(t, rawTheme(t, replacement))
	result, err := s.Import(source, false)
	if err != nil {
		t.Fatalf("probe replacement: %v", err)
	}
	if !result.NeedsOverwrite || result.Theme.Name != "Replacement" {
		t.Fatalf("probe result = %#v", result)
	}
	listed, _ := s.List()
	if listed[2].Name != "Original" {
		t.Fatalf("probe overwrote original: %#v", listed[2])
	}

	result, err = s.Import(source, true)
	if err != nil || result.NeedsOverwrite {
		t.Fatalf("confirmed replacement = %#v, %v", result, err)
	}
	listed, _ = s.List()
	if listed[2].Name != "Replacement" {
		t.Fatalf("confirmed replacement not installed: %#v", listed[2])
	}
}

func TestImportRejectsInvalidSourceFiles(t *testing.T) {
	s := NewInDir(t.TempDir())
	if _, err := s.Import(t.TempDir(), false); err == nil {
		t.Fatal("Import directory succeeded")
	}
	large := rawThemeFile(t, strings.Repeat("x", maxThemeFileSize+1))
	if _, err := s.Import(large, false); err == nil {
		t.Fatal("Import oversized file succeeded")
	}
}

func TestServiceInitErrorPropagates(t *testing.T) {
	want := os.ErrPermission
	s := &Service{initErr: want}
	if _, err := s.List(); err != want {
		t.Fatalf("List init error = %v", err)
	}
	if _, err := s.Import("ignored", false); err != want {
		t.Fatalf("Import init error = %v", err)
	}
	if err := s.Remove("custom"); err != want {
		t.Fatalf("Remove init error = %v", err)
	}
}

func TestDefaultDirSeparatesDevThemes(t *testing.T) {
	t.Setenv("LICH_DEV", "")
	production, err := defaultDir()
	if err != nil {
		t.Fatalf("defaultDir production: %v", err)
	}
	if filepath.Base(production) != "themes" || filepath.Base(filepath.Dir(production)) != "lich" {
		t.Fatalf("production dir = %q", production)
	}

	t.Setenv("LICH_DEV", "1")
	development, err := defaultDir()
	if err != nil {
		t.Fatalf("defaultDir development: %v", err)
	}
	if filepath.Base(development) != "themes-dev" || filepath.Base(filepath.Dir(development)) != "lich" {
		t.Fatalf("development dir = %q", development)
	}
	if got := New().dir; got != development {
		t.Fatalf("New dir = %q, want %q", got, development)
	}
}

func TestRemoveDeletesOnlyCustomTheme(t *testing.T) {
	dir := t.TempDir()
	s := NewInDir(dir)
	if _, err := importTheme(t, s, customTheme("removable")); err != nil {
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
	if err := s.Remove("Bad ID"); err == nil {
		t.Fatal("Remove invalid id succeeded")
	}
	if err := s.Remove("not-there"); err != nil {
		t.Fatalf("Remove missing theme: %v", err)
	}
}

func TestListSortsCustomThemesByName(t *testing.T) {
	dir := t.TempDir()
	s := NewInDir(dir)
	zeta := customTheme("zeta")
	zeta.Name = "Zeta"
	alpha := customTheme("alpha")
	alpha.Name = "alpha"
	if _, err := importTheme(t, s, zeta); err != nil {
		t.Fatalf("Import zeta: %v", err)
	}
	if _, err := importTheme(t, s, alpha); err != nil {
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

func TestListSkipsMissingTerminalAndNonThemeEntries(t *testing.T) {
	dir := t.TempDir()
	missingTerminal := customTheme("missing-terminal")
	missingTerminal.Terminal = nil
	if err := os.WriteFile(
		filepath.Join(dir, "missing-terminal.json"),
		[]byte(rawTheme(t, missingTerminal)),
		0o600,
	); err != nil {
		t.Fatalf("write missing-terminal theme: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignored"), 0o600); err != nil {
		t.Fatalf("write non-theme file: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "nested.json"), 0o700); err != nil {
		t.Fatalf("make directory entry: %v", err)
	}
	themes, err := NewInDir(dir).List()
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

func TestBootCSSMatchesBundledThemes(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "frontend", "src", "index.css"))
	if err != nil {
		t.Fatalf("read index.css: %v", err)
	}
	css := string(data)
	selectors := []string{":root", ".dark"}
	for i, theme := range bundledThemes {
		t.Run(theme.ID, func(t *testing.T) {
			got := cssVariables(t, css, selectors[i])
			delete(got, "radius")
			if !reflect.DeepEqual(got, theme.App) {
				t.Fatalf("%s boot variables differ from bundled JSON\nCSS: %#v\nJSON: %#v", selectors[i], got, theme.App)
			}
		})
	}
}

func cssVariables(t *testing.T, css, selector string) map[string]string {
	t.Helper()
	blockPattern := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(selector) + `\s*\{([^}]*)\}`)
	match := blockPattern.FindStringSubmatch(css)
	if match == nil {
		t.Fatalf("CSS selector %q not found", selector)
	}
	variables := make(map[string]string)
	propertyPattern := regexp.MustCompile(`(?m)^\s*--([a-z0-9-]+):\s*([^;]+);`)
	for _, property := range propertyPattern.FindAllStringSubmatch(match[1], -1) {
		variables[property[1]] = strings.TrimSpace(property[2])
	}
	return variables
}

func TestBundledThemesPassValidation(t *testing.T) {
	for _, theme := range bundledThemes {
		t.Run(theme.ID, func(t *testing.T) {
			if err := validateTheme(theme); err != nil {
				t.Fatalf("bundled theme %q is invalid: %v", theme.ID, err)
			}
		})
	}
}

// The bundled assets carry an "origin" for the frontend, which imports the same
// JSON. Go must not read it back: a typo there would ship a theme the UI sorts
// and filters as neither bundled nor custom.
func TestBundledThemeOriginIgnoresAsset(t *testing.T) {
	for name, field := range map[string]string{
		"typo":    `"origin": "bundlde",`,
		"custom":  `"origin": "custom",`,
		"missing": "",
	} {
		t.Run(name, func(t *testing.T) {
			raw := `{"id": "light", "name": "Light", "scheme": "light", ` + field +
				`"app": {}, "terminal": {}}`
			if got := mustBundledTheme([]byte(raw)).Origin; got != OriginBundled {
				t.Fatalf("origin = %q, want %q", got, OriginBundled)
			}
		})
	}
}

// The template ships as the answer to "what shape does a theme have", so it has
// to survive the same validator an imported file meets — not just parse.
func TestBundledTemplateIsImportable(t *testing.T) {
	var template Theme
	if err := json.Unmarshal(themeassets.Template, &template); err != nil {
		t.Fatalf("parse template: %v", err)
	}
	if template.Origin != "" {
		t.Fatalf("template ships an origin: %q", template.Origin)
	}
	template.Origin = OriginCustom
	if err := validateCustom(template); err != nil {
		t.Fatalf("template is not importable: %v", err)
	}
	if !reflect.DeepEqual(keySet(template.App), appTokens) {
		t.Fatalf("template app tokens = %#v, want %#v", keySet(template.App), appTokens)
	}
	if !reflect.DeepEqual(keySet(template.Terminal), terminalTokens) {
		t.Fatalf("template terminal tokens = %#v, want every supported one", keySet(template.Terminal))
	}
}

func TestSaveTemplateWritesEveryColor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "template.json")
	if err := NewInDir(t.TempDir()).SaveTemplate(path); err != nil {
		t.Fatalf("SaveTemplate: %v", err)
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved template: %v", err)
	}
	if !reflect.DeepEqual(written, themeassets.Template) {
		t.Fatal("saved template differs from the bundled asset")
	}
}

func TestSaveTemplateRejectsEmptyDestination(t *testing.T) {
	if err := NewInDir(t.TempDir()).SaveTemplate("  "); err == nil {
		t.Fatal("SaveTemplate with no destination succeeded")
	}
}

func TestSaveTemplateReportsWriteFailure(t *testing.T) {
	dir := t.TempDir()
	if err := NewInDir(dir).SaveTemplate(filepath.Join(dir, "missing", "template.json")); err == nil {
		t.Fatal("SaveTemplate into a missing directory succeeded")
	}
}

func TestImportRejectsWindowsDeviceNames(t *testing.T) {
	for _, id := range []string{"con", "nul", "aux", "com1", "lpt9"} {
		t.Run(id, func(t *testing.T) {
			if _, err := importTheme(t, NewInDir(t.TempDir()), customTheme(id)); err == nil {
				t.Fatalf("Import of Windows device name %q succeeded", id)
			}
		})
	}
}

// \r? because the repo pins no .gitattributes, so a Windows checkout hands this
// test CRLF and a bare \n matches nothing there.
var docJSONBlock = regexp.MustCompile("(?s)```json\r?\n(.*?)```")

func TestDocJSONBlockReadsBothLineEndings(t *testing.T) {
	for _, tt := range []struct{ name, newline string }{
		{name: "lf", newline: "\n"},
		{name: "crlf", newline: "\r\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			doc := "text" + tt.newline + "```json" + tt.newline + "{}" + tt.newline + "```"
			block := docJSONBlock.FindStringSubmatch(doc)
			if block == nil {
				t.Fatalf("no JSON block found in a %s document", tt.name)
			}
			if strings.TrimSpace(block[1]) != "{}" {
				t.Fatalf("block = %q, want the JSON body", block[1])
			}
		})
	}
}

// The documented example is the shape users copy from, so it answers to the
// validator like any other imported theme.
func TestDocumentedExampleIsImportable(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "themes.md"))
	if err != nil {
		t.Fatalf("read docs/themes.md: %v", err)
	}
	block := docJSONBlock.FindSubmatch(data)
	if block == nil {
		t.Fatal("docs/themes.md has no JSON example")
	}
	var example Theme
	if err := json.Unmarshal(block[1], &example); err != nil {
		t.Fatalf("parse documented example: %v", err)
	}
	example.Origin = OriginCustom
	if err := validateCustom(example); err != nil {
		t.Fatalf("documented example is not importable: %v", err)
	}
	if !reflect.DeepEqual(keySet(example.App), appTokens) {
		t.Fatalf("documented app tokens = %#v, want %#v", keySet(example.App), appTokens)
	}
}

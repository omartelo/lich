// Package themes stores user-imported color themes and exposes them to the UI.
package themes

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	themeassets "github.com/omartelo/lich/themes"
)

const (
	OriginBundled       = "bundled"
	OriginCustom        = "custom"
	SchemeLight         = "light"
	SchemeDark          = "dark"
	themeIDMaxLength    = 64
	themeNameMaxLength  = 128
	colorValueMaxLength = 128
	maxThemeFileSize    = 1 << 20
	// App colors are set as CSS custom properties, so the allowlist is what
	// both a CSS color context and xterm's parser accept — anything able to
	// name a resource (url(), image-set()) or defer a value (var(), attr())
	// is out by construction, not by blocklist.
	appColorPattern = `^(#[0-9a-fA-F]{3,8}|[a-zA-Z]{3,20}|(rgb|rgba|hsl|hsla|oklch|oklab|lab|lch|color)\([0-9a-zA-Z.,%/ +-]*\))$`
	// Terminal colors are stricter: xterm parses hex directly, and everything
	// else goes through a canvas round-trip that throws on alpha and is caught
	// into a silent fallback — a theme that half-applies with no error.
	terminalColorPattern = `^#[0-9a-fA-F]{3,8}$`
)

var (
	// The first character is spelled separately, so the repeat counts the rest.
	idPattern          = regexp.MustCompile(fmt.Sprintf(`^[a-z0-9][a-z0-9._-]{0,%d}$`, themeIDMaxLength-1))
	appColorValue      = regexp.MustCompile(appColorPattern)
	terminalColorValue = regexp.MustCompile(terminalColorPattern)
	reserved           = map[string]struct{}{
		"light":  {},
		"dark":   {},
		"system": {},
		"match":  {},
	}
	// A theme id names a file, and on Windows these still address a device
	// even with an extension — con.json opens the console, not a file.
	windowsDeviceNames = set(
		"con", "prn", "aux", "nul",
		"com1", "com2", "com3", "com4", "com5", "com6", "com7", "com8", "com9",
		"lpt1", "lpt2", "lpt3", "lpt4", "lpt5", "lpt6", "lpt7", "lpt8", "lpt9",
	)
)

// Theme describes every color value the frontend needs to paint the app and
// the xterm terminal.
type Theme struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Scheme   string            `json:"scheme"`
	Origin   string            `json:"origin"`
	Source   *Source           `json:"source,omitempty"`
	App      map[string]string `json:"app"`
	Terminal map[string]string `json:"terminal"`
}

// Source records the repository a theme was installed from and the manifest
// version it came in at, so lich can clone it again to look for a newer one. It
// is written by lich, never by a picked file: a standalone import has no
// source and stays unversioned.
type Source struct {
	URL     string `json:"url"`
	Version string `json:"version"`
}

// ImportResult either reports the installed theme or asks the UI to confirm
// replacing the existing custom theme with the same id.
type ImportResult struct {
	Theme          Theme `json:"theme"`
	NeedsOverwrite bool  `json:"needsOverwrite"`
}

// Service reads bundled themes plus user-imported themes.
type Service struct {
	dir     string
	initErr error
}

// New returns a theme service rooted under lich's config directory.
func New() *Service {
	dir, err := defaultDir()
	return &Service{dir: dir, initErr: err}
}

// NewInDir returns a theme service rooted at dir. It is used by tests.
func NewInDir(dir string) *Service {
	return &Service{dir: dir}
}

func defaultDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve config directory: %w", err)
	}
	name := "themes"
	if os.Getenv("LICH_DEV") != "" {
		name = "themes-dev"
	}
	return filepath.Join(dir, "lich", name), nil
}

// List returns bundled themes followed by custom themes sorted by name.
func (s *Service) List() ([]Theme, error) {
	if s.initErr != nil {
		return nil, s.initErr
	}
	custom, err := s.customThemes()
	if err != nil {
		return nil, err
	}
	out := make([]Theme, 0, len(bundledThemes)+len(custom))
	for _, theme := range bundledThemes {
		out = append(out, cloneTheme(theme))
	}
	out = append(out, custom...)
	return out, nil
}

// Import reads a picked JSON file and installs it as a custom theme. An
// existing id is reported without changing it unless overwrite is true.
func (s *Service) Import(path string, overwrite bool) (ImportResult, error) {
	if s.initErr != nil {
		return ImportResult{}, s.initErr
	}
	raw, err := readThemeFile(path)
	if err != nil {
		return ImportResult{}, err
	}
	var theme Theme
	if err := json.Unmarshal(raw, &theme); err != nil {
		return ImportResult{}, fmt.Errorf("parse theme JSON: %w", err)
	}
	theme.Origin = OriginCustom
	// A picked file names no repository: whatever it claims as its source would
	// point a later update check at a URL lich never cloned.
	theme.Source = nil
	if err := validateCustom(theme); err != nil {
		return ImportResult{}, err
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return ImportResult{}, fmt.Errorf("create themes dir: %w", err)
	}
	installed, err := s.install(theme, overwrite)
	if err != nil {
		return ImportResult{}, err
	}
	if !installed {
		return ImportResult{Theme: theme, NeedsOverwrite: true}, nil
	}
	return ImportResult{Theme: theme}, nil
}

// install writes theme to its file. Without overwrite an existing file is left
// untouched and install reports false, which the callers turn into a question
// for the user.
func (s *Service) install(theme Theme, overwrite bool) (bool, error) {
	data, err := encodeTheme(theme)
	if err != nil {
		return false, err
	}
	destination := s.path(theme.ID)
	if !overwrite {
		return writeNewTheme(destination, data)
	}
	tmp := destination + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return false, fmt.Errorf("write theme: %w", err)
	}
	if err := os.Rename(tmp, destination); err != nil {
		_ = os.Remove(tmp)
		return false, fmt.Errorf("install theme: %w", err)
	}
	return true, nil
}

// read loads one installed theme by id.
func (s *Service) read(id string) (Theme, error) {
	if err := validateID(id); err != nil {
		return Theme{}, err
	}
	data, err := os.ReadFile(s.path(id))
	if err != nil {
		return Theme{}, fmt.Errorf("read theme %q: %w", id, err)
	}
	theme, err := decodeStored(data)
	if err != nil {
		return Theme{}, fmt.Errorf("theme %q: %w", id, err)
	}
	return theme, nil
}

// decodeStored reads a theme that is already installed. Unlike an import it
// tolerates an app token the file predates: a release that adds one would
// otherwise invalidate every theme installed before it, and the user would see
// their theme simply disappear from Appearance with nothing but a log line.
func decodeStored(data []byte) (Theme, error) {
	var theme Theme
	if err := json.Unmarshal(data, &theme); err != nil {
		return Theme{}, fmt.Errorf("parse theme JSON: %w", err)
	}
	theme.Origin = OriginCustom
	fillMissingAppTokens(&theme)
	if err := validateCustom(theme); err != nil {
		return Theme{}, err
	}
	return theme, nil
}

// fillMissingAppTokens defaults from the bundled theme of the same scheme, the
// closest thing to the value the author would have written. A theme with no app
// block at all is left alone: that is a malformed file, not an aged one.
func fillMissingAppTokens(theme *Theme) {
	if theme.App == nil {
		return
	}
	defaults := bundledThemes[0]
	if theme.Scheme == SchemeDark {
		defaults = bundledThemes[1]
	}
	for token := range appTokens {
		if _, ok := theme.App[token]; !ok {
			theme.App[token] = defaults.App[token]
		}
	}
}

// SaveTemplate writes the bundled starter theme to a destination the user
// picked. The save dialog already confirmed replacing an existing file.
func (s *Service) SaveTemplate(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("no destination was chosen")
	}
	if err := os.WriteFile(path, themeassets.Template, 0o600); err != nil {
		return fmt.Errorf("write theme template: %w", err)
	}
	return nil
}

func readThemeFile(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("read theme file: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() > maxThemeFileSize {
		return nil, fmt.Errorf("theme file must be a regular file no larger than %d bytes", maxThemeFileSize)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read theme file: %w", err)
	}
	return data, nil
}

func encodeTheme(theme Theme) ([]byte, error) {
	data, err := json.MarshalIndent(theme, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode theme: %w", err)
	}
	return append(data, '\n'), nil
}

func writeNewTheme(path string, data []byte) (bool, error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if os.IsExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("create theme: %w", err)
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return false, fmt.Errorf("write theme: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return false, fmt.Errorf("close theme: %w", err)
	}
	return true, nil
}

// Remove deletes a custom theme. Bundled and reserved ids cannot be removed.
func (s *Service) Remove(id string) error {
	if s.initErr != nil {
		return s.initErr
	}
	if err := validateID(id); err != nil {
		return err
	}
	if _, ok := reserved[id]; ok {
		return fmt.Errorf("theme %q is bundled or reserved", id)
	}
	if err := os.Remove(s.path(id)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove theme: %w", err)
	}
	return nil
}

func (s *Service) customThemes() ([]Theme, error) {
	// Stat before reading: a file sitting where the themes directory belongs
	// makes ReadDir report not-exist on Windows, which the "nothing imported
	// yet" branch below would swallow into an empty list on that OS and an
	// error everywhere else.
	info, err := os.Stat(s.dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read themes dir: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("themes path is not a directory: %s", s.dir)
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("read themes dir: %w", err)
	}
	themes := make([]Theme, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, entry.Name()))
		if err != nil {
			slog.Warn("themes: read custom theme", "file", entry.Name(), "err", err)
			continue
		}
		theme, err := decodeStored(data)
		if err != nil {
			slog.Warn("themes: invalid custom theme", "file", entry.Name(), "err", err)
			continue
		}
		if theme.ID+".json" != entry.Name() {
			slog.Warn("themes: theme filename mismatch", "file", entry.Name(), "id", theme.ID)
			continue
		}
		themes = append(themes, theme)
	}
	sort.Slice(themes, func(i, j int) bool {
		return strings.ToLower(themes[i].Name) < strings.ToLower(themes[j].Name)
	})
	return themes, nil
}

func (s *Service) path(id string) string {
	return filepath.Join(s.dir, id+".json")
}

func validateCustom(theme Theme) error {
	if err := validateID(theme.ID); err != nil {
		return err
	}
	if _, ok := reserved[theme.ID]; ok {
		return fmt.Errorf("theme id %q is bundled or reserved", theme.ID)
	}
	return validateTheme(theme)
}

func validateTheme(theme Theme) error {
	if strings.TrimSpace(theme.Name) == "" {
		return fmt.Errorf("theme name is required")
	}
	if utf8.RuneCountInString(theme.Name) > themeNameMaxLength {
		return fmt.Errorf("theme name cannot exceed %d characters", themeNameMaxLength)
	}
	if theme.Scheme != SchemeLight && theme.Scheme != SchemeDark {
		return fmt.Errorf("theme scheme must be %q or %q", SchemeLight, SchemeDark)
	}
	if err := validateSource(theme.Source); err != nil {
		return err
	}
	if err := validateColors("app", theme.App, appTokens, appColorValue, true); err != nil {
		return err
	}
	if err := validateColors("terminal", theme.Terminal, terminalTokens, terminalColorValue, false); err != nil {
		return err
	}
	for _, key := range []string{"background", "foreground"} {
		if strings.TrimSpace(theme.Terminal[key]) == "" {
			return fmt.Errorf("terminal.%s is required", key)
		}
	}
	return nil
}

func validateID(id string) error {
	if len(id) > themeIDMaxLength || !idPattern.MatchString(id) {
		return fmt.Errorf("theme id %q must be lowercase letters, digits, dots, underscores or dashes", id)
	}
	if _, ok := windowsDeviceNames[id]; ok {
		return fmt.Errorf("theme id %q is a reserved Windows device name", id)
	}
	return nil
}

func validateColors(
	group string,
	colors map[string]string,
	allowed map[string]struct{},
	value *regexp.Regexp,
	requireAll bool,
) error {
	if colors == nil {
		return fmt.Errorf("%s colors are required", group)
	}
	for key, color := range colors {
		if _, ok := allowed[key]; !ok {
			return fmt.Errorf("unknown %s color %q", group, key)
		}
		if err := validateColorValue(group, key, color, value); err != nil {
			return err
		}
	}
	if requireAll {
		for key := range allowed {
			if _, ok := colors[key]; !ok {
				return fmt.Errorf("%s.%s is required", group, key)
			}
		}
	}
	return nil
}

func validateColorValue(group, key, value string, pattern *regexp.Regexp) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("%s.%s cannot be empty", group, key)
	}
	if len(trimmed) > colorValueMaxLength || !pattern.MatchString(trimmed) {
		return fmt.Errorf("%s.%s is not an accepted color value: %q", group, key, value)
	}
	return nil
}

func cloneTheme(theme Theme) Theme {
	out := Theme{
		ID:       theme.ID,
		Name:     theme.Name,
		Scheme:   theme.Scheme,
		Origin:   theme.Origin,
		App:      maps.Clone(theme.App),
		Terminal: maps.Clone(theme.Terminal),
	}
	if theme.Source != nil {
		source := *theme.Source
		out.Source = &source
	}
	return out
}

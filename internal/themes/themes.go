// Package themes stores user-imported color themes and exposes them to the UI.
package themes

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
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
	themeIDPattern      = `^[a-z0-9][a-z0-9._-]{0,63}$`
)

var (
	idPattern = regexp.MustCompile(themeIDPattern)
	reserved  = map[string]struct{}{
		"light":  {},
		"dark":   {},
		"system": {},
		"match":  {},
	}
)

// Theme describes every color value the frontend needs to paint the app and
// the xterm terminal.
type Theme struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Scheme   string            `json:"scheme"`
	Origin   string            `json:"origin"`
	App      map[string]string `json:"app"`
	Terminal map[string]string `json:"terminal"`
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
	if err := validateCustom(theme); err != nil {
		return ImportResult{}, err
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return ImportResult{}, fmt.Errorf("create themes dir: %w", err)
	}
	data, err := encodeTheme(theme)
	if err != nil {
		return ImportResult{}, err
	}
	destination := s.path(theme.ID)
	if !overwrite {
		created, err := writeNewTheme(destination, data)
		if err != nil {
			return ImportResult{}, err
		}
		if !created {
			return ImportResult{Theme: theme, NeedsOverwrite: true}, nil
		}
		return ImportResult{Theme: theme}, nil
	}
	tmp := destination + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return ImportResult{}, fmt.Errorf("write theme: %w", err)
	}
	if err := os.Rename(tmp, destination); err != nil {
		_ = os.Remove(tmp)
		return ImportResult{}, fmt.Errorf("install theme: %w", err)
	}
	return ImportResult{Theme: theme}, nil
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
	entries, err := os.ReadDir(s.dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
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
		var theme Theme
		if err := json.Unmarshal(data, &theme); err != nil {
			slog.Warn("themes: parse custom theme", "file", entry.Name(), "err", err)
			continue
		}
		theme.Origin = OriginCustom
		if err := validateCustom(theme); err != nil {
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
	if err := validateColors("app", theme.App, appTokens, true); err != nil {
		return err
	}
	if err := validateColors("terminal", theme.Terminal, terminalTokens, false); err != nil {
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
	return nil
}

func validateColors(group string, colors map[string]string, allowed map[string]struct{}, requireAll bool) error {
	if colors == nil {
		return fmt.Errorf("%s colors are required", group)
	}
	for key, value := range colors {
		if _, ok := allowed[key]; !ok {
			return fmt.Errorf("unknown %s color %q", group, key)
		}
		if err := validateColorValue(group, key, value); err != nil {
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

func validateColorValue(group, key, value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("%s.%s cannot be empty", group, key)
	}
	if len(trimmed) > colorValueMaxLength || strings.ContainsAny(trimmed, ";{}") {
		return fmt.Errorf("%s.%s is not a safe CSS color value", group, key)
	}
	return nil
}

func cloneTheme(theme Theme) Theme {
	return Theme{
		ID:       theme.ID,
		Name:     theme.Name,
		Scheme:   theme.Scheme,
		Origin:   theme.Origin,
		App:      cloneMap(theme.App),
		Terminal: cloneMap(theme.Terminal),
	}
}

func cloneMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

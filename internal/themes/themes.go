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
)

const (
	OriginBundled = "bundled"
	OriginCustom  = "custom"
	SchemeLight   = "light"
	SchemeDark    = "dark"
)

var (
	idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)
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

// Import validates raw JSON and persists it as a custom theme.
func (s *Service) Import(raw string) (Theme, error) {
	if s.initErr != nil {
		return Theme{}, s.initErr
	}
	var theme Theme
	if err := json.Unmarshal([]byte(raw), &theme); err != nil {
		return Theme{}, fmt.Errorf("parse theme JSON: %w", err)
	}
	theme.Origin = OriginCustom
	if err := validateCustom(theme); err != nil {
		return Theme{}, err
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return Theme{}, fmt.Errorf("create themes dir: %w", err)
	}
	data, err := json.MarshalIndent(theme, "", "  ")
	if err != nil {
		return Theme{}, fmt.Errorf("encode theme: %w", err)
	}
	data = append(data, '\n')
	path := s.path(theme.ID)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return Theme{}, fmt.Errorf("write theme: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return Theme{}, fmt.Errorf("install theme: %w", err)
	}
	return theme, nil
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
	if !idPattern.MatchString(id) {
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
	if len(trimmed) > 128 || strings.ContainsAny(trimmed, ";{}") {
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

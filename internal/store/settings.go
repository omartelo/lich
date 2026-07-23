package store

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/omartelo/lich/internal/providers"
)

// claudeBinKey is the settings key holding the Claude Code binary path. Claude
// keeps this legacy key (rather than the "provider.<id>.bin" scheme the others
// use) so overrides configured before the providers feature keep resolving.
const claudeBinKey = "claude.bin"

// globalScope is the sentinel project_id for settings that apply to every
// project. A concrete project id scopes the setting to that project only.
const globalScope = ""

// GetSetting returns a setting's value for the given scope. An empty projectID
// reads the global value. A missing setting returns "" and no error.
func (s *Service) GetSetting(key, projectID string) (string, error) {
	var value string
	err := s.db.QueryRow(
		`SELECT value FROM settings WHERE key = ? AND project_id = ?`,
		key, projectID,
	).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get setting %q: %w", key, err)
	}
	return value, nil
}

// SetSetting writes a setting's value for the given scope. An empty projectID
// sets the global value.
func (s *Service) SetSetting(key, projectID, value string) error {
	_, err := s.db.Exec(
		`INSERT INTO settings (key, project_id, value) VALUES (?, ?, ?)
		 ON CONFLICT(key, project_id) DO UPDATE SET value = excluded.value`,
		key, projectID, value,
	)
	if err != nil {
		return fmt.Errorf("set setting %q: %w", key, err)
	}
	return nil
}

// binKey is the settings key holding a provider's custom binary path. Claude
// uses the legacy "claude.bin"; every other provider is namespaced by id.
func binKey(providerID string) string {
	if providerID == providers.Claude {
		return claudeBinKey
	}
	return "provider." + providerID + ".bin"
}

// ProviderBin resolves a provider's binary path for a project: the project
// override wins, then the global value, then "" (letting the terminal fall back
// to the provider's default). It is the single call the terminal service makes
// when spawning a session's PTY.
func (s *Service) ProviderBin(providerID, projectID string) string {
	key := binKey(providerID)
	if projectID != globalScope {
		if bin, err := s.GetSetting(key, projectID); err == nil && bin != "" {
			return bin
		}
	}
	bin, err := s.GetSetting(key, globalScope)
	if err != nil {
		return ""
	}
	return bin
}

// ClaudeBin is ProviderBin for Claude Code, kept for the plugin service that
// resolves the same binary outside the terminal's spawn path.
func (s *Service) ClaudeBin(projectID string) string {
	return s.ProviderBin(providers.Claude, projectID)
}

package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// claudeBinKey is the settings key holding the Claude Code binary path.
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

// ClaudeBin resolves the Claude Code binary path for a project: the project
// override wins, then the global value, then "" (letting the terminal fall back
// to its default). It is the single call the terminal service makes when
// spawning a session's PTY.
func (s *Service) ClaudeBin(projectID string) string {
	if projectID != globalScope {
		if bin, err := s.GetSetting(claudeBinKey, projectID); err == nil && bin != "" {
			return bin
		}
	}
	bin, err := s.GetSetting(claudeBinKey, globalScope)
	if err != nil {
		return ""
	}
	return bin
}

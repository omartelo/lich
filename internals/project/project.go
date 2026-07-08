// Package project opens project directories through the OS file picker. Project
// metadata (list, active) lives in the frontend; this service only turns a
// picked directory into a stable identity the terminal sessions can key on.
package project

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Project identifies an opened project directory.
type Project struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

// Service opens project directories via the native file picker.
type Service struct{}

// New returns a ready-to-use project service.
func New() *Service {
	return &Service{}
}

// Open shows the native directory picker and returns the chosen project, or nil
// if the user cancels the dialog.
func (s *Service) Open() (*Project, error) {
	path, err := application.Get().Dialog.OpenFile().
		CanChooseDirectories(true).
		CanChooseFiles(false).
		SetTitle("Open Project").
		PromptForSingleSelection()
	if err != nil {
		return nil, fmt.Errorf("open dialog failed: %w", err)
	}
	if path == "" {
		return nil, nil // cancelled
	}
	return &Project{ID: projectID(path), Name: filepath.Base(path), Path: path}, nil
}

// projectID derives a stable, URL- and event-safe ID from the absolute path, so
// the same directory always maps to the same project.
func projectID(path string) string {
	sum := sha256.Sum256([]byte(path))
	return hex.EncodeToString(sum[:6])
}

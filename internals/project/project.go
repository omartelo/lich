// Package project opens project directories through the OS file picker. Project
// metadata (list, active, sessions) lives in the frontend; this service only
// turns a picked directory into a stable identity the frontend can group its
// terminal sessions under.
package project

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

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

// Branch returns the current git branch of the project directory, or "" when the
// path is not a git work tree or HEAD is detached (no branch to name). It reads
// the current state on call, so a checkout made after opening is not reflected
// until the branch is resolved again.
func (s *Service) Branch(path string) string {
	out, err := exec.Command("git", "-C", path, "symbolic-ref", "--short", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// PickFile shows the native file picker and returns the chosen file path, or ""
// if the user cancels the dialog.
func (s *Service) PickFile() (string, error) {
	path, err := application.Get().Dialog.OpenFile().
		CanChooseFiles(true).
		CanChooseDirectories(false).
		SetTitle("Attach File").
		PromptForSingleSelection()
	if err != nil {
		return "", fmt.Errorf("open dialog failed: %w", err)
	}
	return path, nil
}

// DiffStats summarizes the uncommitted changes of a work tree.
type DiffStats struct {
	Files   int `json:"files"`
	Added   int `json:"added"`
	Deleted int `json:"deleted"`
}

// Diff returns the dirty-file count (modified + untracked) and the added/deleted
// line totals against HEAD. A non-repository path yields the zero value, matching
// Branch's contract.
func (s *Service) Diff(path string) DiffStats {
	var stats DiffStats
	if out, err := exec.Command("git", "-C", path, "status", "--porcelain").Output(); err == nil {
		stats.Files = countLines(out)
	}
	out, err := exec.Command("git", "-C", path, "diff", "--numstat", "HEAD").Output()
	if err != nil {
		return stats
	}
	for line := range strings.Lines(string(out)) {
		cols := strings.Fields(line)
		if len(cols) < 3 {
			continue
		}
		// Binary files report "-" for both counts; Atoi fails and adds zero.
		added, _ := strconv.Atoi(cols[0])
		deleted, _ := strconv.Atoi(cols[1])
		stats.Added += added
		stats.Deleted += deleted
	}
	// Untracked files are invisible to `git diff`; count their lines as
	// additions, the way Warp and forge diff views present a fresh file.
	if out, err := exec.Command("git", "-C", path, "ls-files", "--others", "--exclude-standard", "-z").Output(); err == nil {
		for _, rel := range strings.Split(string(out), "\x00") {
			if rel != "" {
				stats.Added += countFileLines(filepath.Join(path, rel))
			}
		}
	}
	return stats
}

// countFileLines returns the line count of a text file, or 0 for binaries
// (NUL byte in the first 8000 bytes, git's own heuristic) and unreadable files.
// ponytail: reads the whole file with a 10MB cap — untracked source files are
// small; stream in chunks if that assumption ever breaks.
func countFileLines(name string) int {
	const maxSize = 10 << 20
	if info, err := os.Stat(name); err != nil || !info.Mode().IsRegular() || info.Size() > maxSize {
		return 0
	}
	data, err := os.ReadFile(name)
	if err != nil || len(data) == 0 {
		return 0
	}
	if bytes.IndexByte(data[:min(len(data), 8000)], 0) >= 0 {
		return 0
	}
	n := bytes.Count(data, []byte{'\n'})
	if data[len(data)-1] != '\n' {
		n++ // last line without trailing newline still counts
	}
	return n
}

func countLines(out []byte) int {
	n := 0
	for line := range strings.Lines(string(out)) {
		if strings.TrimSpace(line) != "" {
			n++
		}
	}
	return n
}

// projectID derives a stable, URL- and event-safe ID from the absolute path, so
// the same directory always maps to the same project.
func projectID(path string) string {
	sum := sha256.Sum256([]byte(path))
	return hex.EncodeToString(sum[:6])
}

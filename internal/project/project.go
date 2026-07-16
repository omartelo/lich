// Package project opens project directories through the OS file picker. Project
// metadata (list, active, sessions) lives in the frontend; this service only
// turns a picked directory into a stable identity the frontend can group its
// terminal sessions under.
package project

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Project identifies an opened project directory.
type Project struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

// Picker shows native file/directory choosers — zenity in production
// (picker.go), fakes in tests. Returns "" on user cancel.
type Picker interface {
	PickDirectory(title string) (string, error)
	PickFile(title string) (string, error)
}

// Service opens project directories via the native file picker.
type Service struct {
	picker Picker
}

// New returns a project service using the given picker.
func New(picker Picker) *Service {
	return &Service{picker: picker}
}

// Open shows the native directory picker and returns the chosen project, or nil
// if the user cancels the dialog.
func (s *Service) Open() (*Project, error) {
	path, err := s.picker.PickDirectory("Open Project")
	if err != nil {
		return nil, fmt.Errorf("open dialog failed: %w", err)
	}
	if path == "" {
		return nil, nil // cancelled
	}
	return newProject(path), nil
}

// newProject builds a project's stable identity from a chosen directory path.
func newProject(path string) *Project {
	return &Project{ID: projectID(path), Name: filepath.Base(path), Path: path}
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

// PullRequest identifies the open GitHub pull request for a work tree's current
// branch, as reported by the gh CLI. State is one of gh's OPEN, CLOSED, MERGED;
// only OPEN reaches the frontend (see parsePullRequest).
type PullRequest struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
	State  string `json:"state"`
}

// prLookupTimeout caps the gh network call so a slow forge or hung auth prompt
// never stalls the footer poll.
const prLookupTimeout = 5 * time.Second

// PullRequest returns the open pull request for the path's current branch, or nil
// when there is none, gh is missing/unauthenticated, or the path is not a GitHub
// repo. It shells out to `gh pr view`, which resolves the PR from the checked-out
// branch. Any failure yields nil, matching Branch's "hide the segment" contract.
func (s *Service) PullRequest(path string) *PullRequest {
	ctx, cancel := context.WithTimeout(context.Background(), prLookupTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "gh", "pr", "view", "--json", "number,url,state")
	cmd.Dir = path
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	return parsePullRequest(out)
}

// parsePullRequest decodes gh's `pr view --json` output. It returns nil for
// malformed JSON, a zero PR number (gh emits `{}` in some no-PR states), or a
// non-OPEN state — gh reports the branch's PR even after it is merged or closed,
// so without the state gate a merged PR would keep showing the badge.
func parsePullRequest(out []byte) *PullRequest {
	var pr PullRequest
	if err := json.Unmarshal(out, &pr); err != nil {
		return nil
	}
	if pr.Number == 0 || pr.URL == "" || pr.State != "OPEN" {
		return nil
	}
	return &pr
}

// PickFile shows the native file picker and returns the chosen file path, or ""
// if the user cancels the dialog.
func (s *Service) PickFile() (string, error) {
	path, err := s.picker.PickFile("Attach File")
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
	// A repository without commits has no HEAD; diff against git's empty tree,
	// same as DiffText. Errors here must not skip the untracked block below.
	base := "HEAD"
	if _, err := runGit(path, "rev-parse", "--verify", "HEAD"); err != nil {
		base = emptyTreeHash
	}
	if out, err := exec.Command("git", "-C", path, "diff", "--numstat", base).Output(); err == nil {
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
// It reads the whole file with a 10MB cap — untracked source files are small;
// stream in chunks if that assumption ever breaks.
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

// projectIDBytes is the number of SHA-256 bytes kept for a project ID (12 hex
// chars) — enough to make collisions a non-issue for a handful of open projects.
const projectIDBytes = 6

// projectID derives a stable, URL- and event-safe ID from the absolute path, so
// the same directory always maps to the same project.
func projectID(path string) string {
	sum := sha256.Sum256([]byte(path))
	return hex.EncodeToString(sum[:projectIDBytes])
}

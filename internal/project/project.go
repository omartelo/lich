// Package project opens project directories through the OS file picker. Project
// metadata (list, active, sessions) lives in the frontend; this service only
// turns a picked directory into a stable identity the frontend can group its
// terminal sessions under.
package project

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
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
	PickSaveFile(title, defaultName string) (string, error)
}

// Service opens project directories via the native file picker.
type Service struct {
	picker Picker
	// runner runs the GitHub CLI for the pull request flows (pr.go); a seam so
	// they are testable without a GitHub remote. Call it through s.gh, which
	// resolves the account the checkout runs as.
	runner ghRunner
	// accounts resolves the gh account a checkout's calls run as; nil (the
	// default, and every test that does not wire it) means gh's own active
	// account. Wired to the store in main (ghaccount.go).
	accounts accountLookup
	// bases memoises the base-branch readout and throttles the fetch behind it
	// (basestatus.go). Zero value ready; it holds a mutex, so a Service is
	// passed by pointer and never copied.
	bases baseCache
}

// New returns a project service using the given picker.
func New(picker Picker) *Service {
	return &Service{picker: picker, runner: runGH}
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

// Home returns a project rooted at the user's home directory, without a picker.
// The update flow opens an install terminal there when no project is in view, so
// clicking Install never dead-ends on the Home screen.
func (s *Service) Home() (*Project, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home dir: %w", err)
	}
	return newProject(home), nil
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
	out, _ := gitQuiet(path, "symbolic-ref", "--short", "HEAD")
	return strings.TrimSpace(out)
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
	out, err := s.gh(prLookupTimeout, path, "pr", "view", "--json", "number,url,state")
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

// Exists reports whether path is still a directory on disk. A stored project
// outlives the folder it points at — the picker guarantees the directory exists
// when a project is first opened, nothing does when one is reopened from a row.
func (s *Service) Exists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// PickFile shows the native file picker and returns the chosen file path, or ""
// if the user cancels the dialog.
func (s *Service) PickFile(title string) (string, error) {
	path, err := s.picker.PickFile(title)
	if err != nil {
		return "", fmt.Errorf("open dialog failed: %w", err)
	}
	return path, nil
}

// PickSaveFile shows the native save dialog seeded with defaultName and returns
// the chosen destination, or "" if the user cancels.
func (s *Service) PickSaveFile(title, defaultName string) (string, error) {
	path, err := s.picker.PickSaveFile(title, defaultName)
	if err != nil {
		return "", fmt.Errorf("save dialog failed: %w", err)
	}
	return path, nil
}

// DiffStats summarizes the uncommitted changes of a work tree. Head is the
// commit those changes sit on — the frontend watches it to notice a commit the
// way it watches Files/Added/Deleted to notice an edit.
type DiffStats struct {
	Files   int    `json:"files"`
	Added   int    `json:"added"`
	Deleted int    `json:"deleted"`
	Head    string `json:"head"`
}

// Diff returns the dirty-file count (modified + untracked), the added/deleted
// line totals against HEAD, and the HEAD commit itself. A non-repository path
// yields the zero value, matching Branch's contract.
func (s *Service) Diff(path string) DiffStats {
	var stats DiffStats
	// --untracked-files=all, because the default collapses a new directory into
	// one "?? dir/" line: an agent writing 25 files into fresh packages would
	// count as one changed file.
	if out, ok := gitQuiet(path, "status", "--porcelain", "--untracked-files=all"); ok {
		stats.Files = countLines(out)
	}
	head, base := diffBase(path)
	stats.Head = head
	if out, ok := gitQuiet(path, "diff", "--numstat", base); ok {
		stats.Added, stats.Deleted = numstatTotals(out)
	}
	// Untracked files are invisible to `git diff`; count their lines as
	// additions, the way Warp and forge diff views present a fresh file.
	for _, rel := range untrackedFiles(path) {
		stats.Added += countFileLines(filepath.Join(path, rel))
	}
	return stats
}

// diffBase resolves what uncommitted changes are diffed against: HEAD and its
// commit, or git's empty tree with no commit at all when the repository has no
// HEAD yet. A repository without commits is an ordinary state here, not a
// failure — hence the quiet probe.
func diffBase(path string) (head, base string) {
	out, ok := gitQuiet(path, "rev-parse", "--verify", "HEAD")
	if !ok {
		return "", emptyTreeHash
	}
	return strings.TrimSpace(out), "HEAD"
}

// numstatTotals sums the added and deleted line counts of `git diff --numstat`.
func numstatTotals(out string) (added, deleted int) {
	for line := range strings.Lines(out) {
		cols := strings.Fields(line)
		if len(cols) < 3 {
			continue
		}
		// Binary files report "-" for both counts; Atoi fails and adds zero.
		a, _ := strconv.Atoi(cols[0])
		d, _ := strconv.Atoi(cols[1])
		added += a
		deleted += d
	}
	return added, deleted
}

// countFileLines returns the line count of a text file, or 0 for binaries and
// unreadable files.
func countFileLines(name string) int {
	if info, err := os.Stat(name); err != nil || !info.Mode().IsRegular() || info.Size() > maxTextFileBytes {
		return 0
	}
	data, err := os.ReadFile(name)
	if err != nil || len(data) == 0 {
		return 0
	}
	if isBinary(data) {
		return 0
	}
	n := bytes.Count(data, []byte{'\n'})
	if data[len(data)-1] != '\n' {
		n++ // last line without trailing newline still counts
	}
	return n
}

func countLines(out string) int {
	n := 0
	for line := range strings.Lines(out) {
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

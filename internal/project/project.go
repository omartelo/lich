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
	// projects names the project a directory already belongs to, so Relocate can
	// refuse one the workspace is holding twice. Wired to the store in main;
	// nil answers "nobody owns it", which is what a picked directory is until
	// there is a workspace to ask.
	projects projectLookup
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

// projectLookup names the project already rooted at a directory — its id and
// its name — or two empty strings when no project is.
type projectLookup func(path string) (id, name string)

// SetProjects wires the workspace lookup Relocate validates against. Left
// unset, any picked directory is taken as free.
func (s *Service) SetProjects(lookup projectLookup) {
	s.projects = lookup
}

// Relocate points a stored project at a directory chosen from the picker,
// keeping id rather than deriving a new one from the path. The id is the
// project's identity in the store — what its sessions hang off and what names
// its worktree directory — so a checkout that was moved or renamed keeps both.
// The name follows the new directory. nil when the user cancels.
//
// A directory another project already sits on is refused: the two rows would
// hold the same path under different ids, and every path-addressed lookup the
// app makes — which project a checkout belongs to, which account its gh calls
// run as — answers with whichever row the query reached first.
func (s *Service) Relocate(id string) (*Project, error) {
	path, err := s.picker.PickDirectory("Relocate Project")
	if err != nil {
		return nil, fmt.Errorf("open dialog failed: %w", err)
	}
	if path == "" {
		return nil, nil // cancelled
	}
	if s.projects != nil {
		if owner, name := s.projects(path); owner != "" && owner != id {
			return nil, fmt.Errorf("That directory is already the project %q. Open that one instead.", name)
		}
	}
	project := newProject(path)
	project.ID = id
	return project, nil
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

// Missing returns the subset of paths whose directory is no longer there. The
// lists that offer closed projects ask about all their rows at once, so a row
// that cannot be opened as it stands says so before it is clicked.
func (s *Service) Missing(paths []string) []string {
	gone := []string{}
	for _, path := range paths {
		if !s.Exists(path) {
			gone = append(gone, path)
		}
	}
	return gone
}

// BranchesOf answers Branch for several checkouts at once, keyed by path; a path
// that names no branch — gone, not a repository, a detached HEAD — is left out
// rather than mapped to "", so a caller reads "no branch" from the absence.
//
// The batch exists because the list that asks is a list of parked sessions: it
// wants each row's branch once, when it is drawn, not the per-path poll a live
// card subscribes to (frontend/src/lib/git/use-git-status.ts). Three RPCs per
// row per second is what that poll costs, and a history list is long.
func (s *Service) BranchesOf(paths []string) map[string]string {
	branches := map[string]string{}
	for _, path := range paths {
		if branch := s.Branch(path); branch != "" {
			branches[path] = branch
		}
	}
	return branches
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

// DiffStats is what one status read says about a work tree: its uncommitted
// changes, and the two things they sit on. Head is the commit — the frontend
// watches it to notice a commit the way it watches Files/Added/Deleted to
// notice an edit — and Branch is the branch, carried here because git answers
// both in the same call the counts come out of.
type DiffStats struct {
	Files   int    `json:"files"`
	Added   int    `json:"added"`
	Deleted int    `json:"deleted"`
	Head    string `json:"head"`
	Branch  string `json:"branch"`
}

// Diff returns the dirty-file count (modified + untracked), the added/deleted
// line totals against HEAD, the HEAD commit itself and the branch it is on. A
// non-repository path yields the zero value, matching Branch's contract.
//
// One git child on a clean checkout, two on a dirty one: the status read
// answers the branch, the head and everything dirty (status.go), and the line
// totals are asked for only when there is something to total.
func (s *Service) Diff(path string) DiffStats {
	status := readWorkTree(path)
	stats := DiffStats{Files: status.files, Head: status.head, Branch: status.branch}
	// A tree the status read found nothing dirty in has nothing to total, so the
	// second child would spend a process to be told so — on a call the frontend
	// runs per second per checkout on screen, most of which nobody is editing.
	// The implication runs one way only: untracked files are dirty and produce
	// no numstat lines, so this over-asks rather than under-counts.
	if status.files > 0 {
		if out, ok := gitQuiet(path, "diff", "--numstat", status.base); ok {
			stats.Added, stats.Deleted = numstatTotals(out)
		}
	}
	// Untracked files are invisible to `git diff`; count their lines as
	// additions, the way Warp and forge diff views present a fresh file.
	for _, rel := range status.untracked {
		stats.Added += countFileLines(filepath.Join(path, rel))
	}
	return stats
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

// projectIDBytes is the number of SHA-256 bytes kept for a project ID (12 hex
// chars) — enough to make collisions a non-issue for a handful of open projects.
const projectIDBytes = 6

// projectID derives a stable, URL- and event-safe ID from the absolute path, so
// the same directory always maps to the same project.
func projectID(path string) string {
	sum := sha256.Sum256([]byte(path))
	return hex.EncodeToString(sum[:projectIDBytes])
}

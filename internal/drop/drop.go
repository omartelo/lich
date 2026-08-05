// Package drop turns a file dropped on the terminal into a path the session
// can paste.
//
// The shell is a Chromium window, not a native terminal emulator, so a drop
// arrives through the page's DataTransfer — which carries the bytes and the
// metadata (name, size, mtime) but never the file's path: Chromium exposes
// neither `text/uri-list` nor `File.path` for a local drop. Two ways back to a
// path, in order:
//
//   - Resolve: find the item by that metadata — under the session's directory
//     first, then under home. The path is then the real one, so an edit lands
//     on the user's file.
//   - Upload: for anything neither tree holds, keep a copy under the config dir
//     and paste the copy's path. Reading it works; editing it edits the copy —
//     which is why Resolve is tried first. The copies expire (see Prune).
//
// A dropped directory only ever takes the first path: the page can walk one,
// but copying a tree to paste a path to the copy is not what the drop meant.
package drop

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// maxUpload bounds one dropped file. Screenshots and logs are the payload; a
// bigger drop is a mistake the terminal should refuse rather than copy.
const maxUpload = 32 << 20

// maxEntries bounds one searched tree, so a drop over a home directory or a
// vendored monorepo cannot stall the window. Past it the search gives up and
// the item falls through to the upload copy.
const maxEntries = 50_000

// homeDir is os.UserHomeDir, replaced in tests: the search must never reach
// the machine's real home, on any OS.
var homeDir = os.UserHomeDir

// mtimeSlackMs absorbs the rounding between the browser's millisecond
// File.lastModified and the filesystem's timestamp.
const mtimeSlackMs = 2_000

// keepDropped is how long a copy outlives its drop. Long enough that a path
// pasted into a prompt still resolves after a weekend of it sitting unsent,
// and short enough that the directory cannot grow without bound, which is the
// only reason it is deleted at all.
const keepDropped = 3 * 24 * time.Hour

// skipDirs are trees a dropped file is never meaningfully found in, and the
// ones that make the search expensive.
var skipDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
	"target":       true,
	"dist":         true,
	".venv":        true,
}

// Item is one dropped entry as the page sees it — the whole of what Chromium
// hands over about a local file. Mtime is File.lastModified: milliseconds.
type Item struct {
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	Mtime int64  `json:"mtime"`
	Dir   bool   `json:"dir"`
}

type Service struct {
	dir string
}

// New keeps uploaded copies under <configDir>/lich/dropped.
func New(configDir string) *Service {
	return &Service{dir: filepath.Join(configDir, "lich", "dropped")}
}

// Resolve maps each item to its absolute path, or "" when neither the
// session's tree nor the home directory holds a match for it.
//
// The session's directory is searched first, then home — a drop is most often
// the session's own file, but it is just as often a screenshot or a directory
// from somewhere else entirely, and a directory has no copy to fall back to.
func (s *Service) Resolve(root string, items []Item) []string {
	paths := make([]string, len(items))
	pending := make(map[int]bool, len(items))
	for i := range items {
		pending[i] = true
	}
	home, err := homeDir()
	if err != nil {
		slog.Warn("drop: no home directory to search", "err", err)
	}
	for _, search := range []struct {
		root string
		// Hidden directories are the session tree's own (.github, .config of a
		// dotfiles checkout) but home's are its caches — tens of thousands of
		// files that would spend the whole budget before the search reaches
		// anything a person drags.
		skipHidden bool
	}{{root: root}, {root: home, skipHidden: true}} {
		if len(pending) == 0 || search.root == "" || (search.root == root && search.skipHidden) {
			continue
		}
		find(search.root, search.skipHidden, items, pending, paths)
	}
	return paths
}

// find walks root a level at a time, resolving each pending item to the
// shallowest match it has. Breadth first, because depth first spends its
// budget on whichever subtree sorts first — the caches under a home directory,
// which is how a directory sitting right beside the search root goes missing.
//
// Deciding at the end of a level is what keeps the ambiguity rule meaningful:
// every twin at the same depth is seen together, and two of them resolve to
// nothing rather than to the wrong one.
func find(root string, skipHidden bool, items []Item, pending map[int]bool, paths []string) {
	wanted := make(map[string][]int, len(items))
	for i, item := range items {
		wanted[item.Name] = append(wanted[item.Name], i)
	}
	budget := maxEntries
	for level := []string{root}; len(level) > 0 && len(pending) > 0 && budget > 0; {
		hits := make(map[int][]string)
		var next []string
		for _, dir := range level {
			entries, err := os.ReadDir(dir)
			if err != nil {
				// An unreadable directory costs its items their real path,
				// nothing more.
				continue
			}
			for _, entry := range entries {
				budget--
				if budget <= 0 {
					break
				}
				path := filepath.Join(dir, entry.Name())
				if entry.IsDir() && !skipDirs[entry.Name()] &&
					!(skipHidden && strings.HasPrefix(entry.Name(), ".")) {
					next = append(next, path)
				}
				for _, i := range wanted[entry.Name()] {
					if pending[i] && matches(items[i], entry) {
						hits[i] = append(hits[i], path)
					}
				}
			}
		}
		for i, candidates := range hits {
			if len(candidates) == 1 {
				paths[i] = candidates[0]
			}
			// Ambiguous at this depth is answered here and not deeper: a match
			// further down is not the one the shallow twins were competing for.
			delete(pending, i)
		}
		level = next
	}
}

// matches reports whether entry is the dropped item. A directory can only be
// matched by name — the page reports a placeholder size for one — so the
// uniqueness rule in Resolve is what keeps that honest.
func matches(item Item, entry fs.DirEntry) bool {
	if entry.IsDir() != item.Dir {
		return false
	}
	if item.Dir {
		return true
	}
	info, err := entry.Info()
	if err != nil || info.Size() != item.Size {
		return false
	}
	delta := info.ModTime().UnixMilli() - item.Mtime
	return delta > -mtimeSlackMs && delta < mtimeSlackMs
}

// Upload stores one dropped file's bytes and answers with the path it landed
// at. The name rides the query because the body is the file itself.
func (s *Service) Upload(w http.ResponseWriter, r *http.Request) {
	// CORS like the RPC's (internal/rpc): in dev the page's origin is the Vite
	// server, not this listener, and the token is the actual auth. The preflight
	// is not optional here — the body carries the file's own content type, which
	// is not one a simple request may have.
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	name := filepath.Base(filepath.FromSlash(r.URL.Query().Get("name")))
	if name == "" || name == "." || name == string(filepath.Separator) || name == ".." {
		http.Error(w, "missing name", http.StatusBadRequest)
		return
	}
	path, err := s.Save(name, http.MaxBytesReader(w, r.Body, maxUpload))
	if err != nil {
		slog.Warn("drop: upload failed", "name", name, "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"path": path}); err != nil {
		slog.Warn("drop: encode response", "path", path, "err", err)
	}
}

// Save writes one dropped file under the copies directory, without ever
// overwriting an earlier drop — its path may still be sitting in a prompt.
func (s *Service) Save(name string, body io.Reader) (string, error) {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return "", fmt.Errorf("dropped-files dir: %w", err)
	}
	path := uniquePath(s.dir, name)
	file, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("create %s: %w", path, err)
	}
	defer file.Close()
	if _, err := io.Copy(file, body); err != nil {
		// A body that stops mid-write — an aborted request, a full disk —
		// otherwise leaves a truncated copy nobody asked for and nobody pasted,
		// under a name the next drop of the same file will then avoid. Windows
		// refuses to remove a file that still has an open handle, so the close
		// happens here rather than in the deferred one below it.
		file.Close()
		if err := os.Remove(path); err != nil {
			slog.Warn("drop: could not remove a half-written copy", "path", path, "err", err)
		}
		return "", fmt.Errorf("write %s: %w", path, err)
	}
	s.Prune()
	return path, nil
}

// Prune deletes copies older than keepDropped. Called after each new copy —
// which is the only thing that grows the directory — and once at startup, so
// the last of them is cleared even by a lich that is never dropped on again.
//
// Age is the rule because a copy's whole purpose is the prompt it was pasted
// into: once that conversation is days behind, the path in it is scrollback.
// A failure here is never the caller's problem — the copy it just wrote is
// good, and the stale ones get another chance on the next drop.
func (s *Service) Prune() {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		// Nothing dropped yet is the common case, not a fault.
		if !errors.Is(err, fs.ErrNotExist) {
			slog.Warn("drop: prune could not read the copies dir", "dir", s.dir, "err", err)
		}
		return
	}
	deadline := time.Now().Add(-keepDropped)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil || info.ModTime().After(deadline) {
			continue
		}
		path := filepath.Join(s.dir, entry.Name())
		if err := os.Remove(path); err != nil {
			slog.Warn("drop: prune failed", "path", path, "err", err)
		}
	}
}

// uniquePath is name, or name-2, name-3… up to a bound past which a caller is
// looping on something other than collisions.
func uniquePath(dir, name string) string {
	path := filepath.Join(dir, name)
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for i := 2; i < 1_000; i++ {
		if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
			break
		}
		path = filepath.Join(dir, fmt.Sprintf("%s-%d%s", stem, i, ext))
	}
	return path
}

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
// A confined session takes the second way far more often, and has to: its home
// is an empty private one (internal/sandbox), so a path found under the real
// home is a path only lich can open. Resolve is told which sessions those are
// and does not search home for them at all — the copy is what reaches the
// session, because the sandbox binds the copies directory and nothing else of
// the home.
//
// The copies are kept one directory per session, and that is the unit they are
// deleted in: a confined session sees its own directory mounted and not the one
// beside it, and every copy a session was dropped goes when its row does
// (Purge).
//
// A dropped directory only ever takes the first path: the page can walk one,
// but copying a tree to paste a path to the copy is not what the drop meant.
//
// The footer's attach button lands here too (Attach): a file chosen from the
// native picker already has its path, so there is nothing to look up — but a
// confined session cannot open one outside its checkout either, and the same
// copy is the answer.
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

	"github.com/omartelo/lich/internal/rpc"

	"github.com/omartelo/lich/internal/sandbox"
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

// sandboxAvailable is sandbox.Available, replaced in tests: whether a machine
// can confine anything is the host's answer — bubblewrap installed, macOS —
// and a test about the search must not take it.
var sandboxAvailable = sandbox.Available

// mtimeSlackMs absorbs the rounding between the browser's millisecond
// File.lastModified and the filesystem's timestamp.
const mtimeSlackMs = 2_000

// maxCopies bounds how many copies of one name live side by side (the name
// itself plus its -2… suffixes). Past it a caller is looping on something other
// than collisions, and the drop is refused rather than overwriting.
const maxCopies = 999

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
	// pick opens the host's file chooser. See SetPicker.
	pick func(title string) (string, error)
}

// SetPicker wires the host's file picker (internal/project), which is what the
// footer's attach button reaches through Attach. Startup wiring, called before
// anything serves.
func (s *Service) SetPicker(pick func(title string) (string, error)) {
	s.pick = pick
}

// Dir is where a lich with this config directory keeps the copies. It is the
// directory the sandbox binds into a confined session, so it is named here
// rather than spelled out at the two callers (main.go wires it through to
// internal/terminal).
func Dir(configDir string) string {
	return filepath.Join(configDir, "lich", "dropped")
}

// New keeps uploaded copies under Dir(configDir), and creates that directory
// now rather than on the first drop. A confined session binds its own copies
// directory when its PTY spawns (internal/terminal/sandbox.go), and a bind
// whose source is not there yet is skipped: the first file dropped into a
// session that opened before the directory existed would land where that
// session cannot read it.
func New(configDir string) *Service {
	dir := Dir(configDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		slog.Warn("drop: could not create the copies dir", "dir", dir, "err", err)
	}
	return &Service{dir: dir}
}

// Resolve maps each item to its absolute path, or "" when neither the
// session's tree nor the home directory holds a match for it.
//
// The session's directory is searched first, then home — a drop is most often
// the session's own file, but it is just as often a screenshot or a directory
// from somewhere else entirely, and a directory has no copy to fall back to.
//
// confined is the verdict recorded for the calling session (internal/sandbox):
// its home is an empty private one, so a match under the real home is a path
// the session cannot open, and answering with it hands the prompt a file that
// is not there. Those sessions search their checkout and nothing else, and
// everything outside it falls through to the copy — which the sandbox does
// bind. A machine with no sandbox backend confines nothing, so the flag alone
// is not enough to stop searching: Windows, which has no backend at all, keeps
// the unconfined behaviour whatever the row says.
func (s *Service) Resolve(root string, items []Item, confined bool) []string {
	paths := make([]string, len(items))
	pending := make(map[int]bool, len(items))
	for i := range items {
		pending[i] = true
	}
	home := searchableHome(confined)
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

// searchableHome is the home directory Resolve falls back to, or "" when the
// caller is a confined session on a machine that can actually confine one — see
// Resolve for why that home is worse than no home at all.
func searchableHome(confined bool) string {
	if confined && sandboxAvailable() {
		return ""
	}
	home, err := homeDir()
	if err != nil {
		slog.Warn("drop: no home directory to search", "err", err)
	}
	return home
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
	scan := &levelScan{
		items:      items,
		wanted:     wanted,
		pending:    pending,
		skipHidden: skipHidden,
		budget:     maxEntries,
	}
	for dirs := []string{root}; len(dirs) > 0 && len(pending) > 0 && scan.budget > 0; {
		scan.hits = make(map[int][]string)
		scan.next = nil
		for _, dir := range dirs {
			scan.read(dir)
		}
		// A level the budget cut short was never seen whole, so a lone candidate
		// in it may still have a twin among the entries never read: deciding
		// here is how the ambiguity rule hands back the wrong file instead of
		// nothing.
		if scan.budget > 0 {
			for i, candidates := range scan.hits {
				if len(candidates) == 1 {
					paths[i] = candidates[0]
				}
				// Ambiguous at this depth is answered here and not deeper: a
				// match further down is not the one the shallow twins were
				// competing for.
				delete(pending, i)
			}
		}
		dirs = scan.next
	}
}

// levelScan is one breadth-first level in progress: what is still being looked
// for, what this level has hit, the directories the next level will read, and
// what is left of the search budget.
type levelScan struct {
	items      []Item
	wanted     map[string][]int
	pending    map[int]bool
	skipHidden bool

	hits   map[int][]string
	next   []string
	budget int
}

// read walks one directory into the level, stopping the moment the budget runs
// out — which leaves the level undecided rather than half-decided.
func (l *levelScan) read(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		// An unreadable directory costs its items their real path, nothing more.
		return
	}
	for _, entry := range entries {
		l.budget--
		if l.budget <= 0 {
			return
		}
		path := filepath.Join(dir, entry.Name())
		if entry.IsDir() && !skipDirs[entry.Name()] &&
			!(l.skipHidden && strings.HasPrefix(entry.Name(), ".")) {
			l.next = append(l.next, path)
		}
		for _, i := range l.wanted[entry.Name()] {
			if l.pending[i] && matches(l.items[i], entry) {
				l.hits[i] = append(l.hits[i], path)
			}
		}
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
// at. The name and the session ride the query because the body is the file
// itself.
func (s *Service) Upload(w http.ResponseWriter, r *http.Request) {
	// The preflight is not optional here — the body carries the file's own
	// content type, which is not one a simple request may have.
	if rpc.Preflight(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	name := element(r.URL.Query().Get("name"))
	if name == "" {
		http.Error(w, "missing name", http.StatusBadRequest)
		return
	}
	// No session, no copy: the id is what decides which directory the file
	// lands in, which is both what a confined session can read and what its
	// close deletes. A copy dropped outside either would live on with nothing
	// to end it.
	session := element(r.URL.Query().Get("session"))
	if session == "" {
		http.Error(w, "missing session", http.StatusBadRequest)
		return
	}
	path, err := s.Save(session, name, http.MaxBytesReader(w, r.Body, maxUpload))
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

// attachTitle is the file chooser's own title, which is all the dialog says
// about why it opened.
const attachTitle = "Attach File"

// Attachment is what the picker produced: the path to write at the prompt, and
// whether that path is a copy's — the caller says so, because nothing about the
// path itself does. Both empty for a cancelled dialog.
type Attachment struct {
	Path   string `json:"path"`
	Copied bool   `json:"copied"`
}

// Attach opens the file chooser and answers with a path the session can
// actually open: the file's own where the session reaches it, a copy's inside
// the session's dropped-files directory otherwise. It is the footer's attach
// button, and the counterpart of a drop — the same problem arrives through a
// dialog instead of a drag.
//
// The picker runs here rather than in the caller, and that is the security
// property of this method: every session carries LICH_TOKEN (see
// internal/terminal), so a confined agent can call any RPC on the loopback
// listener. A method that copied a *path it was handed* would let that agent
// name ~/.ssh/id_rsa and read the copy from inside the sandbox — the sandbox
// undone by its own harness. Here the path can only come from a dialog a human
// answers on screen.
//
// root is the session's checkout, the one tree a confined session sees; a file
// under it keeps its own path. Anything else is copied, including files under
// directories the sandbox happens to bind (a toolchain, ~/.config): copying one
// of those costs a few bytes, and deciding it does not need copying means
// reproducing the mount list here, where it would go stale in silence.
func (s *Service) Attach(sessionID, root string, confined bool) (Attachment, error) {
	if s.pick == nil {
		return Attachment{}, errors.New("no file picker wired")
	}
	path, err := s.pick(attachTitle)
	if err != nil {
		return Attachment{}, fmt.Errorf("open dialog failed: %w", err)
	}
	// A cancelled dialog is not a failure, and neither is a session that reaches
	// the file where it is.
	if path == "" || !confined || !sandboxAvailable() || under(root, path) {
		return Attachment{Path: path}, nil
	}
	copied, err := s.copyIn(sessionID, path)
	if err != nil {
		return Attachment{}, err
	}
	return Attachment{Path: copied, Copied: true}, nil
}

// under reports whether path is inside root. Both are cleaned but neither is
// resolved through symlinks: a link answering "outside" costs a copy, and a
// wrong "inside" costs the session the file altogether.
func under(root, path string) bool {
	if root == "" {
		return false
	}
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// copyIn puts one of the user's own files in a session's copies directory, so a
// confined session can read it. The ceiling is the upload's, for the same
// reason: past it the copy is a mistake rather than an attachment.
func (s *Service) copyIn(sessionID, path string) (string, error) {
	// Without a session there is no directory the sandbox binds, so a copy would
	// land where the session it was made for cannot read it.
	session, name := element(sessionID), element(filepath.Base(path))
	if session == "" || name == "" {
		return "", fmt.Errorf("cannot attach %q to session %q", path, sessionID)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s is a folder — a sandboxed session takes files, not trees", path)
	}
	if info.Size() > maxUpload {
		return "", fmt.Errorf(
			"%s is over the %dMB a sandboxed session can be handed", filepath.Base(path), maxUpload>>20,
		)
	}
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()
	return s.Save(session, name, file)
}

// element is one path element taken from untrusted input — a dropped file's
// name, a session id off the query — reduced to its last element so it can only
// ever name something inside the copies directory. Empty when nothing usable is
// left, which the callers refuse.
func element(raw string) string {
	name := filepath.Base(filepath.FromSlash(raw))
	if name == "." || name == ".." || name == string(filepath.Separator) {
		return ""
	}
	return name
}

// Save writes one dropped file under the session's own copies directory,
// without ever overwriting an earlier drop — its path may still be sitting in
// a prompt. sessionID must already be one path element (element).
func (s *Service) Save(sessionID, name string, body io.Reader) (string, error) {
	dir := filepath.Join(s.dir, sessionID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("dropped-files dir: %w", err)
	}
	path, err := uniquePath(dir, name)
	if err != nil {
		return "", err
	}
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

// Purge deletes every copy dropped into one session. It is what makes a copy
// die with the session it was dropped into rather than with the clock: the
// paths pasted from it only ever meant anything inside that conversation, and
// the session's own row is gone by the time this runs (store.SetSessionGone).
//
// Best effort, like Prune: a failure leaves copies that the age rule still
// clears.
func (s *Service) Purge(sessionID string) {
	name := element(sessionID)
	if name == "" {
		return
	}
	dir := filepath.Join(s.dir, name)
	if err := os.RemoveAll(dir); err != nil {
		slog.Warn("drop: purge failed", "dir", dir, "err", err)
	}
}

// Prune deletes copies older than keepDropped. Called after each new copy —
// which is the only thing that grows the directory — so a lich left running
// for weeks still clears what earlier drops left behind.
//
// Age is the backstop, not the rule: a session that ends takes its copies with
// it (Purge), and what this catches is the session lich never saw end — a
// crash, a kill, a machine that went down.
//
// A failure here is never the caller's problem — the copy it just wrote is
// good, and the stale ones get another chance on the next drop.
func (s *Service) Prune() {
	s.prune(false)
}

// PruneStale is Prune plus the session directories left with nothing in them.
// Startup only, and that is the whole reason it is a second method: a confined
// session binds its own copies directory when it spawns, and removing that
// directory under a live mount leaves every later copy of that session
// invisible inside it — the mount would still point at the deleted one. At
// startup no session has spawned yet, so there is no mount to strand.
func (s *Service) PruneStale() {
	s.prune(true)
}

func (s *Service) prune(empty bool) {
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
		path := filepath.Join(s.dir, entry.Name())
		if !entry.IsDir() {
			pruneExpired(path, entry, deadline)
			continue
		}
		s.pruneSession(path, deadline, empty)
	}
}

// pruneSession clears the expired copies of one session, and the directory
// itself when the caller allows it and nothing is left in it.
func (s *Service) pruneSession(dir string, deadline time.Time, empty bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		slog.Warn("drop: prune could not read a session's copies", "dir", dir, "err", err)
		return
	}
	left := 0
	for _, entry := range entries {
		if entry.IsDir() || !pruneExpired(filepath.Join(dir, entry.Name()), entry, deadline) {
			left++
		}
	}
	if !empty || left > 0 {
		return
	}
	if err := os.Remove(dir); err != nil {
		slog.Warn("drop: prune failed", "path", dir, "err", err)
	}
}

// pruneExpired deletes one copy when it is past the deadline, and reports
// whether it is gone.
func pruneExpired(path string, entry fs.DirEntry, deadline time.Time) bool {
	info, err := entry.Info()
	if err != nil || info.ModTime().After(deadline) {
		return false
	}
	if err := os.Remove(path); err != nil {
		slog.Warn("drop: prune failed", "path", path, "err", err)
		return false
	}
	return true
}

// uniquePath is name, or name-2, name-3… up to maxCopies of them. Past that the
// namespace is exhausted and there is no path left to give: answering with the
// last one would overwrite a copy whose path may still be sitting in a prompt,
// which is the one thing Save promises never to do.
func uniquePath(dir, name string) (string, error) {
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for i := 1; i <= maxCopies; i++ {
		path := filepath.Join(dir, name)
		if i > 1 {
			path = filepath.Join(dir, fmt.Sprintf("%s-%d%s", stem, i, ext))
		}
		if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
			return path, nil
		}
	}
	return "", fmt.Errorf("%s: %d copies already, no free name left", name, maxCopies)
}

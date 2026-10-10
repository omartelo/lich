package spawn

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/omartelo/lich/internal/relay"
	"github.com/omartelo/lich/internal/store"
)

// FiledEventName carries sessions whose folder changed outside the window, so
// their cards move into or out of the folder's block without a reload. One
// event for a card filed and for a folder renamed, because the window's answer
// to both is the same: set these cards' folder (sessions.ts, setSessionsFolder).
const FiledEventName = "sessions-filed"

// FiledEvent is FiledEventName's payload. Folder is "" when the cards were taken
// out of the one they were in.
type FiledEvent struct {
	ProjectID string   `json:"projectId"`
	IDs       []string `json:"ids"`
	Folder    string   `json:"folder"`
}

// ColoredEventName carries sessions painted outside the window, so their cards
// take the colour without a reload (sessions.ts, setSessionsColor).
const ColoredEventName = "sessions-colored"

// ColoredEvent is ColoredEventName's payload. Color is "" when the cards went
// back to the theme.
type ColoredEvent struct {
	ProjectID string   `json:"projectId"`
	IDs       []string `json:"ids"`
	Color     string   `json:"color"`
}

// cardColors are the names a card can be painted with, in the order the
// window's menu offers them: CARD_COLORS in frontend/src/lib/session/card-color.ts,
// which owns the values and draws any other name as no colour at all.
var cardColors = []string{"red", "orange", "amber", "green", "teal", "blue", "violet", "pink"}

// Folder is one of a project's folders as a caller outside the window sees it:
// its name, which is its whole identity (store.SetSessionFolder), and the
// sessions filed under it, by label.
type Folder struct {
	Name     string   `json:"name"`
	Sessions []string `json:"sessions"`
}

// Filed is what a caller is told about the session it filed. Previous is the
// folder it left, "" when it was in none.
type Filed struct {
	ID       string `json:"id"`
	Project  string `json:"project"`
	Label    string `json:"label"`
	Folder   string `json:"folder"`
	Previous string `json:"previous"`
}

// Refiled is what a caller is told about a folder it renamed or took apart:
// every session that moved, which is the part a merge into an existing folder
// would otherwise hide.
type Refiled struct {
	Project  string   `json:"project"`
	From     string   `json:"from"`
	To       string   `json:"to"`
	Sessions []string `json:"sessions"`
}

// FoldersOptions is one Folders call, an object for the reason CloseOptions is
// one.
type FoldersOptions struct {
	From    string `json:"from"`
	Project string `json:"project"`
}

// Folders lists a project's folders. Project names it; empty takes the caller's
// own, exactly as Worktrees does.
func (s *Service) Folders(opts FoldersOptions) ([]Folder, error) {
	fromID, projectName := opts.From, opts.Project
	projects, err := s.sessions.LoadState()
	if err != nil {
		return nil, fmt.Errorf("read the workspace: %w", err)
	}
	target, err := resolveProject(projects, fromID, projectName)
	if err != nil {
		return nil, err
	}
	return foldersOf(target), nil
}

// foldersOf names a project's folders with their sessions, each folder where its
// first card sits in the stored list: the order the sidebar draws their blocks
// in (sessions.ts, foldersOf).
func foldersOf(p store.Project) []Folder {
	folders := []Folder{}
	at := map[string]int{}
	for _, sess := range p.Sessions {
		if sess.Folder == "" {
			continue
		}
		i, seen := at[sess.Folder]
		if !seen {
			i = len(folders)
			at[sess.Folder] = i
			folders = append(folders, Folder{Name: sess.Folder})
		}
		folders[i].Sessions = append(folders[i].Sessions, sess.Label)
	}
	return folders
}

// FileOptions is one File call, an object for the reason CloseOptions is one.
type FileOptions struct {
	From    string `json:"from"`
	Target  string `json:"target"`
	Project string `json:"project"`
	Folder  string `json:"folder"`
}

// File files a session under a folder, the window's "Move to folder" from
// outside it; an empty folder takes the session out of the one it is in. A
// name no session carries yet is a folder that starts existing with this one in
// it, as it is in the window.
//
// target is the session to file, by either of the names it answers to; empty
// files the caller's own, as a rename does.
func (s *Service) File(opts FileOptions) (Filed, error) {
	fromID, target, projectName := opts.From, opts.Target, opts.Project
	folder := strings.TrimSpace(opts.Folder)

	projects, err := s.sessions.LoadState()
	if err != nil {
		return Filed{}, fmt.Errorf("read the workspace: %w", err)
	}
	found, err := targetOrOwn(projects, s.term.AgentName, fromID, target, projectName, "file")
	if err != nil {
		return Filed{}, err
	}

	if err := s.sessions.SetSessionFolder(found.session.ID, folder); err != nil {
		return Filed{}, err
	}
	if s.events != nil {
		s.events.Emit(FiledEventName, FiledEvent{
			ProjectID: found.project.ID, IDs: []string{found.session.ID}, Folder: folder,
		})
	}
	return Filed{
		ID:       found.session.ID,
		Project:  found.project.Name,
		Label:    found.session.Label,
		Folder:   folder,
		Previous: found.session.Folder,
	}, nil
}

// RenameFolderOptions is one RenameFolder call, an object for the reason
// CloseOptions is one. Folder is the name to change and To the new one.
type RenameFolderOptions struct {
	From    string `json:"from"`
	Project string `json:"project"`
	Folder  string `json:"folder"`
	To      string `json:"to"`
}

// RenameFolder renames one of a project's folders, or takes it apart when To is
// empty: the window's "Rename folder" and "Ungroup" from outside it. Project
// names it; empty takes the caller's own.
//
// The name is matched exactly, as the store matches it. One no session carries
// is refused rather than written: the store would match no row and answer
// success, and a caller told its rename landed would go on to address a folder
// that is not there. Renaming onto a name the project already holds merges the
// two, as it does in the window (docs/ceilings.md), which is why the answer
// names every session that moved. What moved is what the store's write reports,
// not this call's own read: a card filed under the old name in between is
// rewritten too. Parked sessions move with the rest but have no card, so the
// answer names the open ones.
func (s *Service) RenameFolder(opts RenameFolderOptions) (Refiled, error) {
	fromID, projectName := opts.From, opts.Project
	from, to := strings.TrimSpace(opts.Folder), strings.TrimSpace(opts.To)
	if from == "" {
		return Refiled{}, errors.New("no folder was named to rename")
	}

	target, err := s.projectHolding(fromID, projectName, from)
	if err != nil {
		return Refiled{}, err
	}

	moved, err := s.sessions.RenameFolder(target.ID, from, to)
	if err != nil {
		return Refiled{}, err
	}
	if s.events != nil {
		s.events.Emit(FiledEventName, FiledEvent{ProjectID: target.ID, IDs: moved, Folder: to})
	}
	return Refiled{Project: target.Name, From: from, To: to, Sessions: labelsOf(target, moved)}, nil
}

// Colored is what a caller is told about a folder it painted: the sessions that
// took the colour, by label.
type Colored struct {
	Project  string   `json:"project"`
	Folder   string   `json:"folder"`
	Color    string   `json:"color"`
	Sessions []string `json:"sessions"`
}

// ColorFolderOptions is one ColorFolder call, an object for the reason
// CloseOptions is one.
type ColorFolderOptions struct {
	From    string `json:"from"`
	Project string `json:"project"`
	Folder  string `json:"folder"`
	Color   string `json:"color"`
}

// ColorFolder paints every session filed under one of a project's folders, the
// window's folder "Color" from outside it; an empty colour hands them back to
// the theme. Project names it; empty takes the caller's own.
//
// The folder is matched exactly and refused when no session carries it, for the
// reason RenameFolder gives. The colour is one of the window's palette names,
// in any case; a name outside it is refused, since the window would draw it as
// no colour and the caller would be told otherwise.
func (s *Service) ColorFolder(opts ColorFolderOptions) (Colored, error) {
	fromID, projectName := opts.From, opts.Project
	folder, color := strings.TrimSpace(opts.Folder), strings.ToLower(strings.TrimSpace(opts.Color))
	if folder == "" {
		return Colored{}, errors.New("no folder was named to color")
	}
	if color != "" && !slices.Contains(cardColors, color) {
		return Colored{}, fmt.Errorf(
			"%q is not a card color: pick one of %s, or an empty one to clear it",
			color, relay.QuotedList(cardColors),
		)
	}

	target, err := s.projectHolding(fromID, projectName, folder)
	if err != nil {
		return Colored{}, err
	}

	painted, err := s.sessions.ColorFolder(target.ID, folder, color)
	if err != nil {
		return Colored{}, err
	}
	if s.events != nil {
		s.events.Emit(ColoredEventName, ColoredEvent{ProjectID: target.ID, IDs: painted, Color: color})
	}
	return Colored{Project: target.Name, Folder: folder, Color: color, Sessions: labelsOf(target, painted)}, nil
}

// projectHolding resolves the project a folder-wide write is aimed at (project
// names it; empty takes the caller's own) and refuses a folder no session in it
// carries: the store would match no row and answer success.
func (s *Service) projectHolding(fromID, projectName, folder string) (store.Project, error) {
	projects, err := s.sessions.LoadState()
	if err != nil {
		return store.Project{}, fmt.Errorf("read the workspace: %w", err)
	}
	target, err := resolveProject(projects, fromID, projectName)
	if err != nil {
		return store.Project{}, err
	}
	if !slices.ContainsFunc(target.Sessions, func(sess store.Session) bool { return sess.Folder == folder }) {
		return store.Project{}, fmt.Errorf("no folder named %q in %s. %s", folder, target.Name, knownFolders(target))
	}
	return target, nil
}

// labelsOf names the open sessions among ids a folder-wide write reported.
// Parked ones are rewritten too but have no card, so they go unnamed.
func labelsOf(p store.Project, ids []string) []string {
	labels := []string{}
	for _, sess := range p.Sessions {
		if slices.Contains(ids, sess.ID) {
			labels = append(labels, sess.Label)
		}
	}
	return labels
}

func knownFolders(p store.Project) string {
	folders := foldersOf(p)
	if len(folders) == 0 {
		return "It has no folders."
	}
	names := make([]string, 0, len(folders))
	for _, f := range folders {
		names = append(names, f.Name)
	}
	return "Its folders: " + relay.QuotedList(names) + "."
}

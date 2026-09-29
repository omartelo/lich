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

// Folders lists a project's folders. project names it; empty takes the caller's
// own, exactly as Worktrees does.
func (s *Service) Folders(fromID, projectName string) ([]Folder, error) {
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

// File files a session under a folder, the window's "Move to folder" from
// outside it; an empty folder takes the session out of the one it is in. A
// name no session carries yet is a folder that starts existing with this one in
// it, as it is in the window.
//
// target is the session to file, by either of the names it answers to; empty
// files the caller's own, as a rename does.
func (s *Service) File(fromID, target, projectName, folder string) (Filed, error) {
	folder = strings.TrimSpace(folder)

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

// RenameFolder renames one of a project's folders, or takes it apart when to is
// empty: the window's "Rename folder" and "Ungroup" from outside it. project
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
func (s *Service) RenameFolder(fromID, projectName, from, to string) (Refiled, error) {
	from, to = strings.TrimSpace(from), strings.TrimSpace(to)
	if from == "" {
		return Refiled{}, errors.New("no folder was named to rename")
	}

	projects, err := s.sessions.LoadState()
	if err != nil {
		return Refiled{}, fmt.Errorf("read the workspace: %w", err)
	}
	target, err := resolveProject(projects, fromID, projectName)
	if err != nil {
		return Refiled{}, err
	}
	if !slices.ContainsFunc(target.Sessions, func(sess store.Session) bool { return sess.Folder == from }) {
		return Refiled{}, fmt.Errorf("no folder named %q in %s. %s", from, target.Name, knownFolders(target))
	}

	moved, err := s.sessions.RenameFolder(target.ID, from, to)
	if err != nil {
		return Refiled{}, err
	}
	if s.events != nil {
		s.events.Emit(FiledEventName, FiledEvent{ProjectID: target.ID, IDs: moved, Folder: to})
	}
	labels := []string{}
	for _, sess := range target.Sessions {
		if slices.Contains(moved, sess.ID) {
			labels = append(labels, sess.Label)
		}
	}
	return Refiled{Project: target.Name, From: from, To: to, Sessions: labels}, nil
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

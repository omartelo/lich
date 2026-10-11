package main

import (
	"context"
	"encoding/json"
	"log"
	"path/filepath"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/omartelo/lich/native/lichclient"
)

// gitPoll is how often each shown path's branch and changes are re-read. The
// backend pushes no git events (the web page polls project.Diff at 1-3s too).
const gitPoll = 3 * time.Second

// model is the backend state the window draws, fed by LoadState, the /events
// stream and a git poll. Every method is safe from any goroutine.
type model struct {
	client     *lichclient.Client
	invalidate func()

	mu       sync.Mutex
	projects []lichclient.Project
	status   map[string]string               // session id -> last session-status state
	cwd      map[string]sessionCwd           // session id -> last session-cwd report
	git      map[string]lichclient.DiffStats // shown path -> its branch and changes
	active   string                          // session id on screen
	lost     bool                            // /events ended under an open window
}

// sessionCwd is a session-cwd payload: where the session's shell is, or the
// host (tmux, ssh, ...) it went into that the backend cannot see through.
type sessionCwd struct {
	ID   string `json:"id"`
	Cwd  string `json:"cwd"`
	Host string `json:"host"`
}

func newModel(client *lichclient.Client, invalidate func()) *model {
	return &model{client: client, invalidate: invalidate, status: map[string]string{}, cwd: map[string]sessionCwd{}, git: map[string]lichclient.DiffStats{}}
}

func (m *model) reload(ctx context.Context) error {
	ps, err := m.client.LoadState(ctx)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.projects = ps
	if m.active == "" {
		for _, p := range ps {
			if p.ActiveSessionID != "" {
				m.active = p.ActiveSessionID
				break
			}
		}
	}
	m.mu.Unlock()
	m.invalidate()
	return nil
}

// follow applies /events until ctx ends. The backend replays nothing, so a
// session's status is unknown until its next session-status event, and its
// cwd until the next cd. When the stream ends first, the backend dropped this
// window: /events serves one client and a newer one replaces it, or lich quit.
// follow marks the model lost rather than reconnecting, which would take the
// socket back from the window that just took it.
func (m *model) follow(ctx context.Context, events <-chan lichclient.Event) {
	for ev := range events {
		switch ev.Name {
		case "session-status":
			var s struct {
				ID    string `json:"id"`
				State string `json:"state"`
			}
			if json.Unmarshal(ev.Data, &s) == nil {
				m.mu.Lock()
				m.status[s.ID] = s.State
				m.mu.Unlock()
			}
		case "session-cwd":
			var c sessionCwd
			if json.Unmarshal(ev.Data, &c) == nil {
				m.mu.Lock()
				m.cwd[c.ID] = c
				m.mu.Unlock()
			}
		case "session-title":
			var s struct {
				ID    string `json:"id"`
				Label string `json:"label"`
			}
			if json.Unmarshal(ev.Data, &s) == nil {
				m.rename(s.ID, s.Label)
			}
		case "session-opened", "session-closed", "project-opened", "sessions-filed", "sessions-colored":
			if err := m.reload(ctx); err != nil {
				log.Printf("reload after %s: %v", ev.Name, err)
			}
		default:
			continue
		}
		m.invalidate()
	}
	if ctx.Err() != nil {
		return
	}
	m.mu.Lock()
	m.lost = true
	m.mu.Unlock()
	m.invalidate()
}

// connectionLost reports whether the backend dropped this window.
func (m *model) connectionLost() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lost
}

func (m *model) rename(id, label string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for pi := range m.projects {
		for si := range m.projects[pi].Sessions {
			if m.projects[pi].Sessions[si].ID == id {
				m.projects[pi].Sessions[si].Label = label
			}
		}
	}
}

func (m *model) pollGit(ctx context.Context) {
	tick := time.NewTicker(gitPoll)
	defer tick.Stop()
	for {
		m.refreshGit(ctx)
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

// refreshGit re-reads every shown path once. A path that is not a git
// checkout fails and keeps no entry, so its card shows no branch.
func (m *model) refreshGit(ctx context.Context) {
	for _, path := range m.shownPaths() {
		d, err := m.client.Diff(ctx, path)
		if err != nil {
			continue
		}
		m.mu.Lock()
		changed := m.git[path] != d
		m.git[path] = d
		m.mu.Unlock()
		if changed {
			m.invalidate()
		}
	}
}

func (m *model) shownPaths() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	seen := map[string]bool{}
	var paths []string
	for _, p := range m.projects {
		for _, s := range p.Sessions {
			path := m.shownPath(p, s)
			if !seen[path] {
				seen[path] = true
				paths = append(paths, path)
			}
		}
	}
	return paths
}

// shownPath is where the card says a session is: its live cwd, else its
// checkout. Callers hold m.mu.
func (m *model) shownPath(p lichclient.Project, s lichclient.Session) string {
	if c := m.cwd[s.ID]; c.Cwd != "" {
		return c.Cwd
	}
	return sessionPath(p, s)
}

func sessionPath(p lichclient.Project, s lichclient.Session) string {
	if s.Path != "" {
		return s.Path
	}
	return p.Path
}

// group is one sidebar block: the sessions of one checkout.
type group struct {
	name     string
	path     string
	sessions []sessionView
}

// sessionView is a session with what the card shows about it. path is the
// checkout it was spawned in, which groups it; shown is where its shell is now,
// and git describes shown.
type sessionView struct {
	lichclient.Session
	projectID string
	path      string
	shown     string
	host      string
	status    string
	git       lichclient.DiffStats
}

// groups returns the sidebar blocks of the first project: the project's own
// checkout first, then each worktree by name.
func (m *model) groups() []group {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.projects) == 0 {
		return nil
	}
	p := m.projects[0]
	byPath := map[string]*group{}
	var order []string
	for _, s := range p.Sessions {
		path := sessionPath(p, s)
		g, ok := byPath[path]
		if !ok {
			g = &group{name: filepath.Base(path), path: path}
			byPath[path] = g
			order = append(order, path)
		}
		shown := m.shownPath(p, s)
		g.sessions = append(g.sessions, sessionView{
			Session: s, projectID: p.ID, path: path, shown: shown, host: m.cwd[s.ID].Host,
			status: m.status[s.ID], git: m.git[shown],
		})
	}
	sort.SliceStable(order, func(i, j int) bool { return order[i] == p.Path && order[j] != p.Path })
	out := make([]group, 0, len(order))
	for _, path := range order {
		out = append(out, *byPath[path])
	}
	return out
}

func (m *model) activeSession() (sessionView, bool) {
	m.mu.Lock()
	id := m.active
	m.mu.Unlock()
	for _, g := range m.groups() {
		for _, s := range g.sessions {
			if s.ID == id {
				return s, true
			}
		}
	}
	return sessionView{}, false
}

func (m *model) setActive(id string) {
	m.mu.Lock()
	m.active = id
	m.mu.Unlock()
}

func (m *model) project() (lichclient.Project, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.projects) == 0 {
		return lichclient.Project{}, false
	}
	return m.projects[0], true
}

// closeSession drops a session from the first project and, when it was on
// screen, puts its neighbor there: the session that fills its slot, else the
// one before it (sessions.ts neighborId). It returns the project and the
// session now active, and false when no open session has that id.
func (m *model) closeSession(id string) (projectID, active string, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.projects) == 0 {
		return "", "", false
	}
	p := &m.projects[0]
	i := slices.IndexFunc(p.Sessions, func(s lichclient.Session) bool { return s.ID == id })
	if i < 0 {
		return "", "", false
	}
	p.Sessions = slices.Delete(p.Sessions, i, i+1)
	if m.active == id {
		m.active = ""
		switch {
		case i < len(p.Sessions):
			m.active = p.Sessions[i].ID
		case i > 0:
			m.active = p.Sessions[i-1].ID
		}
	}
	return p.ID, m.active, true
}

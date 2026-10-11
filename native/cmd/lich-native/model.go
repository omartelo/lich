package main

import (
	"context"
	"encoding/json"
	"log"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/omartelo/lich/native/lichclient"
)

// gitPoll is how often each shown path's branch and changes are re-read. The
// backend pushes no git events (the web page polls project.Diff at 1-3s too).
const gitPoll = 3 * time.Second

// lastScreenKey is the global setting the web window saves its route under
// (frontend/src/lib/last-screen.ts), the one record of which project the user
// was on.
const lastScreenKey = "window.lastScreen"

// projectScreen is last-screen.ts's PROJECT_SCREEN.
var projectScreen = regexp.MustCompile(`^/projects/([^/]+)(?:/.*)?$`)

// model is the backend state the window draws, fed by LoadState, the /events
// stream and a git poll. Every method is safe from any goroutine.
type model struct {
	client     *lichclient.Client
	invalidate func()

	mu            sync.Mutex
	projects      []lichclient.Project // open projects, in the user's order
	status        *statusStore
	cwd           map[string]sessionCwd             // session id -> last session-cwd report
	agent         map[string]string                 // session id -> provider CLI live in its PTY
	sandbox       map[string]sandboxEvent           // session id -> how its spawn was confined
	relay         map[string]sessionRelay           // session id -> request open with another
	git           map[string]lichclient.DiffStats   // shown path -> its branch and changes
	base          map[string]*lichclient.BaseStatus // shown path -> where it stands against its base
	prs           map[string]prLookup               // shown path -> its open pull request
	activeProject string                            // project id on screen
	active        string                            // session id on screen
	lost          bool                              // /events ended under an open window
	focused       bool                              // the window has the user's focus
	restored      bool                              // unread marks taken from the first LoadState
}

// sessionCwd is a session-cwd payload: where the session's shell is, or the
// host (tmux, ssh, ...) it went into that the backend cannot see through.
type sessionCwd struct {
	ID   string `json:"id"`
	Cwd  string `json:"cwd"`
	Host string `json:"host"`
}

func newModel(client *lichclient.Client, invalidate func()) *model {
	return &model{client: client, invalidate: invalidate, status: newStatusStore(), cwd: map[string]sessionCwd{},
		agent: map[string]string{}, sandbox: map[string]sandboxEvent{}, relay: map[string]sessionRelay{}, git: map[string]lichclient.DiffStats{},
		base: map[string]*lichclient.BaseStatus{}, prs: map[string]prLookup{}}
}

func (m *model) reload(ctx context.Context) error {
	ps, err := m.client.LoadState(ctx)
	if err != nil {
		return err
	}
	m.mu.Lock()
	choose := m.activeProject == "" && len(ps) > 0
	m.mu.Unlock()
	launch := ""
	if choose {
		if launch, err = m.launchProject(ctx, ps); err != nil {
			return err
		}
	}
	m.mu.Lock()
	m.projects = ps
	if !m.restored {
		m.status.restoreUnread(unreadSessions(ps))
		m.restored = true
	}
	if launch != "" {
		m.activeProject = launch
	}
	if p := m.shownProject(); p != nil && m.active == "" {
		m.active = p.ActiveSessionID
	}
	m.mu.Unlock()
	m.invalidate()
	return nil
}

// launchProject is the project the window opens on: the one the last screen
// was on while it is still open (projects.tsx resumableScreen). Where the web
// then lands on its Home screen, this window has none and takes the first.
func (m *model) launchProject(ctx context.Context, ps []lichclient.Project) (string, error) {
	saved, err := m.client.GetSetting(ctx, lastScreenKey, "")
	if err != nil {
		return "", err
	}
	if match := projectScreen.FindStringSubmatch(saved); match != nil {
		if slices.ContainsFunc(ps, func(p lichclient.Project) bool { return p.ID == match[1] }) {
			return match[1], nil
		}
	}
	return ps[0].ID, nil
}

// shownProject is the active project, else the first when the active one is
// not open. Callers hold m.mu.
func (m *model) shownProject() *lichclient.Project {
	if i := slices.IndexFunc(m.projects, func(p lichclient.Project) bool { return p.ID == m.activeProject }); i >= 0 {
		return &m.projects[i]
	}
	if len(m.projects) == 0 {
		return nil
	}
	return &m.projects[0]
}

// setActiveProject puts a project on screen with the session it last showed,
// and reports false when no open project has that id.
func (m *model) setActiveProject(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	i := slices.IndexFunc(m.projects, func(p lichclient.Project) bool { return p.ID == id })
	if i < 0 {
		return false
	}
	m.activeProject = id
	m.active = m.projects[i].ActiveSessionID
	return true
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
		case statusEventName:
			if err := m.applyStatus(ctx, ev); err != nil {
				log.Printf("%s: %v", ev.Name, err)
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
			live, err := m.applyLive(ev)
			if err != nil {
				log.Printf("%s: %v", ev.Name, err)
			}
			if !live {
				continue
			}
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
		m.refreshPath(ctx, path)
	}
}

func (m *model) shownPaths() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	p := m.shownProject()
	if p == nil {
		return nil
	}
	seen := map[string]bool{}
	var paths []string
	for _, s := range p.Sessions {
		path := m.shownPath(*p, s)
		if !seen[path] {
			seen[path] = true
			paths = append(paths, path)
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
	unread    bool          // a done turn nobody has read
	age       time.Duration // how long status has lasted, while aged
	aged      bool          // status has a clock (busy, compacting, waiting)
	agent     string        // provider CLI live in the PTY, "" for none
	sandbox   sandboxEvent
	relay     sessionRelay // Direction "" for none
	tool      sessionTool  // Name "" outside a tool call
	reason    string       // what a waiting session is blocked on, "" for unsaid
	git       lichclient.DiffStats
	base      *lichclient.BaseStatus  // nil without a base to stand against
	pr        *lichclient.PullRequest // open PR of the shown path's branch, nil for none
}

// groups returns the sidebar blocks of the active project: the project's own
// checkout first, then each worktree by name.
func (m *model) groups() []group {
	m.mu.Lock()
	defer m.mu.Unlock()
	shown := m.shownProject()
	if shown == nil {
		return nil
	}
	p := *shown
	now := time.Now()
	byPath := map[string]*group{}
	var order []string
	for _, s := range p.Sessions {
		path := sessionPath(p, s)
		g, ok := byPath[path]
		if !ok {
			g = &group{name: checkoutLabel(s.Path, p.Path, p.ID), path: path}
			byPath[path] = g
			order = append(order, path)
		}
		shown := m.shownPath(p, s)
		age, aged := m.status.age(s.ID, now)
		tool, _ := m.status.tool(s.ID)
		g.sessions = append(g.sessions, sessionView{
			Session: s, projectID: p.ID, path: path, shown: shown, host: m.cwd[s.ID].Host,
			status: m.status.status(s.ID), unread: m.status.unread(s.ID), age: age, aged: aged,
			agent: m.agent[s.ID], sandbox: m.sandbox[s.ID], git: m.git[shown],
			base: m.base[shown], pr: m.prs[shown].pr,
			relay: m.relay[s.ID], tool: tool, reason: m.status.reason(s.ID),
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

// setActive puts a session of the active project on screen, and records it
// as the session that project shows when it is put back on screen.
//
// Leaving a card reads its turn, so one that finished while it was on screen
// does not stay news; arriving reads the new one while the window has focus
// (project-events.tsx).
func (m *model) setActive(ctx context.Context, id string) {
	m.mu.Lock()
	left := m.active
	m.active = id
	if p := m.shownProject(); p != nil {
		p.ActiveSessionID = id
	}
	readLeft := left != id && m.status.markSeen(left)
	readNew := m.focused && m.status.markSeen(id)
	m.mu.Unlock()
	if readLeft {
		m.markRead(ctx, left)
	}
	if readNew {
		m.markRead(ctx, id)
	}
}

// project returns the active project.
func (m *model) project() (lichclient.Project, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p := m.shownProject()
	if p == nil {
		return lichclient.Project{}, false
	}
	return *p, true
}

// closeSession drops a session from its project and, when it was on screen,
// puts its neighbor there: the session that fills its slot, else the one
// before it (sessions.ts neighborId). It returns the project and the session
// now active, and false when no open session has that id.
func (m *model) closeSession(id string) (projectID, active string, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, i := m.sessionAt(id)
	if p == nil {
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
		p.ActiveSessionID = m.active
	}
	return p.ID, m.active, true
}

// sessionAt finds the project holding a session and its slot there. Callers
// hold m.mu.
func (m *model) sessionAt(id string) (*lichclient.Project, int) {
	for pi := range m.projects {
		if i := slices.IndexFunc(m.projects[pi].Sessions, func(s lichclient.Session) bool { return s.ID == id }); i >= 0 {
			return &m.projects[pi], i
		}
	}
	return nil, -1
}

// checkoutLabel is frontend/src/lib/git/checkout-label.ts: a group is titled
// with the project folder for the root checkout (an empty path), else with
// the name the worktree was made with, read past <worktrees-root>/<projectID>/
// because a branch name may hold a slash. A checkout outside that root keeps
// its last segment.
func checkoutLabel(path, projectPath, projectID string) string {
	if path == "" {
		return baseName(projectPath)
	}
	// git spells a Windows checkout with forward slashes, lich with backslashes.
	unix := strings.ReplaceAll(path, `\`, "/")
	root := "/" + projectID + "/"
	// The first match: a worktree may itself be named after the project id.
	at := strings.Index(unix, root)
	if at < 0 {
		return baseName(path)
	}
	return unix[at+len(root):]
}

// baseName is frontend/src/lib/paths.ts's: the last segment under either
// separator, ignoring a trailing one.
func baseName(path string) string {
	segments := strings.FieldsFunc(path, func(r rune) bool { return r == '/' || r == '\\' })
	if len(segments) == 0 {
		return ""
	}
	return segments[len(segments)-1]
}

// unreadSessions is every session the workspace database says came back with
// a finished turn nobody read.
func unreadSessions(ps []lichclient.Project) []string {
	var ids []string
	for _, p := range ps {
		for _, s := range p.Sessions {
			if s.Unread {
				ids = append(ids, s.ID)
			}
		}
	}
	return ids
}

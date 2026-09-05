package spawn

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/omartelo/lich/internal/project"
	"github.com/omartelo/lich/internal/store"
)

// added is one session row the fake store was asked to write.
type added struct {
	projectID   string
	sessionID   string
	label       string
	kind        string
	path        string
	nextSeq     int
	originID    string
	originLabel string
}

type fakeSessions struct {
	projects []store.Project
	loadErr  error
	// closed is the workspace's history: projects with is_open = 0, which
	// RecentProjects offers back and AddProject reopens with their sessions
	// intact, exactly as the store does.
	closed     []store.Project
	recentErr  error
	addProjErr error
	rows       []added
	addErr     error
	// models records the model written on each session row, keyed by session id.
	models   map[string]string
	modelErr error
	// deleted/parked/purged record what a close asked the store to do, and
	// against which active session it left the project.
	deleted []closedRow
	parked  []closedRow
	purged  []string
	// renamed records the label each rename wrote, keyed by session id, and
	// renameErr is the write refusing.
	renamed   map[string]string
	renameErr error
}

// closedRow is one session the store was asked to take out of the workspace.
type closedRow struct {
	projectID string
	sessionID string
	activeID  string
}

func (f *fakeSessions) DeleteSession(projectID, sessionID, activeID string) error {
	f.deleted = append(f.deleted, closedRow{projectID, sessionID, activeID})
	return nil
}

func (f *fakeSessions) CloseSession(projectID, sessionID, activeID string) error {
	f.parked = append(f.parked, closedRow{projectID, sessionID, activeID})
	return nil
}

func (f *fakeSessions) RenameSession(sessionID, label string) error {
	if f.renameErr != nil {
		return f.renameErr
	}
	if f.renamed == nil {
		f.renamed = map[string]string{}
	}
	f.renamed[sessionID] = label
	return nil
}

func (f *fakeSessions) PurgeWorktreeSessions(_, path string) error {
	f.purged = append(f.purged, path)
	return nil
}

func (f *fakeSessions) LoadState() ([]store.Project, error) {
	return f.projects, f.loadErr
}

func (f *fakeSessions) RecentProjects() ([]store.Recent, error) {
	if f.recentErr != nil {
		return nil, f.recentErr
	}
	recents := make([]store.Recent, 0, len(f.closed))
	for _, p := range f.closed {
		recents = append(recents, store.Recent{ID: p.ID, Name: p.Name, Path: p.Path})
	}
	return recents, nil
}

// AddProject mirrors the store's upsert: a closed project's row is reopened as
// it stands — its sessions, its label counter and its name all come back — and
// an id nothing holds opens as a new project.
func (f *fakeSessions) AddProject(id, name, path string) error {
	if f.addProjErr != nil {
		return f.addProjErr
	}
	for i, p := range f.projects {
		if p.ID == id {
			f.projects[i].Name, f.projects[i].Path = name, path
			return nil
		}
	}
	for i, p := range f.closed {
		if p.ID == id {
			p.Name, p.Path = name, path
			f.closed = append(f.closed[:i], f.closed[i+1:]...)
			f.projects = append(f.projects, p)
			return nil
		}
	}
	f.projects = append(f.projects, store.Project{ID: id, Name: name, Path: path, NextSeq: 1})
	return nil
}

func (f *fakeSessions) AddSessionFrom(
	projectID, sessionID, label, kind, path string, nextSeq int, originID, originLabel string,
) error {
	if f.addErr != nil {
		return f.addErr
	}
	f.rows = append(f.rows, added{
		projectID, sessionID, label, kind, path, nextSeq, originID, originLabel,
	})
	// The counter moves with the write, as it does in the store: a fake that kept
	// answering the old number would hide two sessions taking one label.
	for i := range f.projects {
		if f.projects[i].ID == projectID {
			f.projects[i].NextSeq = nextSeq
		}
	}
	return nil
}

func (f *fakeSessions) SetSessionModel(sessionID, model string) error {
	if f.modelErr != nil {
		return f.modelErr
	}
	if f.models == nil {
		f.models = map[string]string{}
	}
	f.models[sessionID] = model
	return nil
}

// createdWorktree is one CreateWorktree call, so a test can assert the base the
// service resolved rather than only the checkout it got back.
type createdWorktree struct {
	projectPath string
	projectID   string
	name        string
	base        string
	remote      bool
}

type fakeWorktrees struct {
	branch    string
	branches  project.Branches
	listErr   error
	created   []createdWorktree
	createErr error
	// checkouts is what ListCheckouts answers, dirty which of them hold
	// uncommitted work, and removed what RemoveWorktree was asked to delete.
	checkouts []project.Worktree
	dirty     map[string]bool
	removed   []removedWorktree
	removeErr error
	// adopted names the checkouts the fixture says lich did not create.
	adopted map[string]bool
}

type removedWorktree struct {
	path  string
	force bool
}

func (f *fakeWorktrees) ListCheckouts(string) ([]project.Worktree, error) {
	return f.checkouts, f.listErr
}

func (f *fakeWorktrees) WorktreeDirty(path string) (bool, error) {
	return f.dirty[path], nil
}

func (f *fakeWorktrees) WorktreeAdopted(path string) bool { return f.adopted[path] }

func (f *fakeWorktrees) RemoveWorktree(_, path string, force bool) error {
	f.removed = append(f.removed, removedWorktree{path, force})
	return f.removeErr
}

func (f *fakeWorktrees) Branch(string) string { return f.branch }

func (f *fakeWorktrees) ListBranches(string) (project.Branches, error) {
	return f.branches, f.listErr
}

func (f *fakeWorktrees) CreateWorktree(
	projectPath, projectID, name, base string, baseIsRemote bool,
) (*project.Worktree, error) {
	f.created = append(f.created, createdWorktree{projectPath, projectID, name, base, baseIsRemote})
	if f.createErr != nil {
		return nil, f.createErr
	}
	return &project.Worktree{Name: name, Path: "/wt/" + name}, nil
}

// started is one PTY spawn the fake terminal was asked for.
type started struct {
	id        string
	projectID string
	cwd       string
	kind      string
	name      string
	setup     bool
	cols      int
	rows      int
}

type fakeTerminal struct {
	spawns   []started
	err      error
	closed   []string
	closeErr error
}

func (f *fakeTerminal) Close(id string) error {
	f.closed = append(f.closed, id)
	return f.closeErr
}

func (f *fakeTerminal) Start(
	id, projectID, cwd, kind, _, name string, setup bool, cols, rows int,
) error {
	f.spawns = append(f.spawns, started{id, projectID, cwd, kind, name, setup, cols, rows})
	return f.err
}

type emitted struct {
	name string
	data any
}

type fakeEvents struct{ events []emitted }

func (f *fakeEvents) Emit(name string, data any) {
	f.events = append(f.events, emitted{name, data})
}

// workspace is the state most tests open a session into: one project holding
// the caller's own session, and a second one to name explicitly.
func workspace() []store.Project {
	return []store.Project{
		{ID: "p1", Name: "lich", Path: "/src/lich", NextSeq: 4, Sessions: []store.Session{
			{ID: "s1", Label: "Session 3", Kind: "codex"},
		}},
		{ID: "p2", Name: "revu", Path: "/src/revu", NextSeq: 1},
	}
}

func newService(t *testing.T) (*Service, *fakeSessions, *fakeWorktrees, *fakeTerminal, *fakeEvents) {
	t.Helper()
	sessions := &fakeSessions{projects: workspace()}
	worktrees := &fakeWorktrees{branch: "main"}
	term := &fakeTerminal{}
	events := &fakeEvents{}
	return New(sessions, worktrees, term, events), sessions, worktrees, term, events
}

func TestOpenLandsInTheCallersProject(t *testing.T) {
	svc, sessions, _, term, events := newService(t)

	opened, err := svc.Open("s1", "", "", "", "", "")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	if opened.ProjectID != "p1" || opened.Project != "lich" {
		t.Errorf("opened in %q (%q), want the caller's own project", opened.Project, opened.ProjectID)
	}
	// The label continues the project's own counter, exactly as the window's
	// reducer does — a session that repeats a live label is one `lich send`
	// cannot address without being told which is which.
	if opened.Label != "Session 4" || opened.NextSeq != 5 {
		t.Errorf("label %q, next %d; want Session 4 and 5", opened.Label, opened.NextSeq)
	}
	if len(sessions.rows) != 1 {
		t.Fatalf("wrote %d rows, want 1", len(sessions.rows))
	}
	row := sessions.rows[0]
	if row != (added{"p1", opened.ID, "Session 4", "codex", "", 5, "s1", "Session 3"}) {
		t.Errorf("row = %+v", row)
	}
	if len(term.spawns) != 1 {
		t.Fatalf("started %d terminals, want 1", len(term.spawns))
	}
	spawn := term.spawns[0]
	if spawn.id != opened.ID || spawn.cwd != "/src/lich" || spawn.kind != "codex" {
		t.Errorf("spawn = %+v, want the new session in the project directory", spawn)
	}
	if spawn.setup {
		t.Error("ran the worktree setup script for a session that opened no worktree")
	}
	// No size of its own — pinned as the literal zero rather than read off the
	// constants, because the zero is the contract: it is what makes the terminal
	// service start this session at the size the window is actually showing,
	// instead of a shape chosen here that the first view would have to repaint.
	if spawn.cols != 0 || spawn.rows != 0 {
		t.Errorf("spawn sized %dx%d, want no size of its own", spawn.cols, spawn.rows)
	}
	if spawn.name != opened.Name {
		t.Errorf("spawned as %q but reported %q — the roster name has to be one name", spawn.name, opened.Name)
	}
	if len(events.events) != 1 || events.events[0].name != OpenedEventName {
		t.Fatalf("events = %+v, want one %s", events.events, OpenedEventName)
	}
	if events.events[0].data != any(opened) {
		t.Errorf("the window was told %+v, the caller %+v", events.events[0].data, opened)
	}
}

// The kind is what an agent inherits without asking: a Codex session opening a
// worker gets another Codex, not whatever lich defaults to.
func TestOpenInheritsTheCallersKind(t *testing.T) {
	svc, _, _, term, _ := newService(t)

	if _, err := svc.Open("s1", "", "", "", "", ""); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if got := term.spawns[0].kind; got != "codex" {
		t.Errorf("kind = %q, want the caller's codex", got)
	}
}

func TestOpenTakesANamedProjectAndKind(t *testing.T) {
	svc, sessions, _, term, _ := newService(t)

	opened, err := svc.Open("s1", "revu", "shell", "", "", "")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if opened.ProjectID != "p2" {
		t.Errorf("opened in %q, want the named project", opened.ProjectID)
	}
	if opened.Label != "Session 1" {
		t.Errorf("label = %q, want the named project's own counter", opened.Label)
	}
	if sessions.rows[0].kind != "shell" || term.spawns[0].kind != "shell" {
		t.Errorf("row %q / spawn %q, want shell", sessions.rows[0].kind, term.spawns[0].kind)
	}
	if term.spawns[0].cwd != "/src/revu" {
		t.Errorf("cwd = %q, want the named project's directory", term.spawns[0].cwd)
	}
}

func TestOpenWithoutASessionOrAProjectSaysWhatToDo(t *testing.T) {
	svc, _, _, term, _ := newService(t)

	_, err := svc.Open("", "", "", "", "", "")
	if err == nil {
		t.Fatal("opened a session with nothing to open it in")
	}
	// The command line outside a session inherits nothing, so the error has to
	// name what it could have been given instead of only refusing.
	if !strings.Contains(err.Error(), "lich") || !strings.Contains(err.Error(), "revu") {
		t.Errorf("error = %q, want the open projects named", err)
	}
	if len(term.spawns) != 0 {
		t.Error("started a terminal for a session that was never created")
	}
}

func TestOpenRefusesAnUnknownProject(t *testing.T) {
	svc, sessions, _, _, _ := newService(t)

	if _, err := svc.Open("s1", "nope", "", "", "", ""); err == nil {
		t.Fatal("opened a session in a project that is not open")
	}
	if len(sessions.rows) != 0 {
		t.Error("wrote a row for a project that is not open")
	}
}

func TestOpenRefusesAnUnknownKind(t *testing.T) {
	svc, sessions, _, _, _ := newService(t)

	_, err := svc.Open("s1", "", "gemini", "", "", "")
	if err == nil {
		t.Fatal("opened a session running a provider lich does not know")
	}
	if !strings.Contains(err.Error(), "claude") {
		t.Errorf("error = %q, want the known kinds named", err)
	}
	if len(sessions.rows) != 0 {
		t.Error("wrote a row for an unknown kind")
	}
}

// The model is written on the row rather than only handed to the spawn: the
// terminal service reads it back on every later spawn of this session, so a
// model that never reached the row is a reload away from the default.
func TestOpenRecordsTheModelOnTheRow(t *testing.T) {
	svc, sessions, _, _, _ := newService(t)

	opened, err := svc.Open("s1", "", "claude", "", "", "opus")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if got := sessions.models[opened.ID]; got != "opus" {
		t.Errorf("row model = %q, want opus", got)
	}
}

// The model is written after the card is announced, and this is why: a row with
// no card is a session the user cannot reach to see what went wrong, so the one
// write that can fail after the row exists must not be what hides it. The model
// is an override — losing it costs the provider's default and nothing more,
// which means the terminal still starts: a card with no PTY behind it is
// invisible to `lich sessions` and unreachable by `lich send`.
func TestOpenAnnouncesTheCardEvenWhenTheModelCannotBeRecorded(t *testing.T) {
	svc, sessions, _, term, events := newService(t)
	sessions.modelErr = errors.New("database is locked")

	if _, err := svc.Open("s1", "", "claude", "", "", "opus"); err == nil {
		t.Fatal("Open: want the write error reported, got nil")
	}
	if len(sessions.rows) != 1 {
		t.Fatalf("wrote %d rows, want the session's own", len(sessions.rows))
	}
	if len(events.events) != 1 || events.events[0].name != OpenedEventName {
		t.Errorf("events = %+v, want the card announced for the row that exists", events.events)
	}
	if len(term.spawns) != 1 {
		t.Fatalf("spawned %d terminals, want the session started on the provider's default",
			len(term.spawns))
	}
	if term.spawns[0].id != sessions.rows[0].sessionID {
		t.Errorf("spawned %q, want the session that was written (%q)",
			term.spawns[0].id, sessions.rows[0].sessionID)
	}
}

// A session opened with no model leaves the column alone: "" is what every
// session the window opens carries, and what leaves the provider on its default.
func TestOpenWithoutAModelWritesNone(t *testing.T) {
	svc, sessions, _, _, _ := newService(t)

	if _, err := svc.Open("s1", "", "claude", "", "", ""); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if len(sessions.models) != 0 {
		t.Errorf("wrote %v, want the model column left alone", sessions.models)
	}
}

// The origin is the caller, not the project: a session opened into another
// project still came from the card that asked for it, and the label written
// beside the id is the one that caller answers to right now.
func TestOpenRecordsTheCallerAsTheOrigin(t *testing.T) {
	svc, sessions, _, _, events := newService(t)

	opened, err := svc.Open("s1", "revu", "shell", "", "", "")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	row := sessions.rows[0]
	if row.originID != "s1" || row.originLabel != "Session 3" {
		t.Errorf("row origin = (%q, %q), want (s1, Session 3)", row.originID, row.originLabel)
	}
	// The window draws the rung off the event, so an origin only on the row is
	// one that appears a reload late.
	if opened.OriginSessionID != "s1" || opened.OriginLabel != "Session 3" {
		t.Errorf("announced origin = (%q, %q), want (s1, Session 3)",
			opened.OriginSessionID, opened.OriginLabel)
	}
	if events.events[0].data != any(opened) {
		t.Errorf("the window was told %+v, the caller %+v", events.events[0].data, opened)
	}
}

// No caller, no origin: `lich open` from a plain shell opens a session nobody
// delegated, and a card that claims a parent it never had is worse than a quiet
// one. A fromID that names no session in the workspace is the same case — there
// is nothing left to name.
func TestOpenWithoutACallerRecordsNoOrigin(t *testing.T) {
	for _, from := range []string{"", "gone"} {
		t.Run("from="+from, func(t *testing.T) {
			svc, sessions, _, _, _ := newService(t)

			opened, err := svc.Open(from, "lich", "", "", "", "")
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			row := sessions.rows[0]
			if row.originID != "" || row.originLabel != "" {
				t.Errorf("row origin = (%q, %q), want none", row.originID, row.originLabel)
			}
			if opened.OriginSessionID != "" || opened.OriginLabel != "" {
				t.Errorf("announced origin = %+v, want none", opened)
			}
		})
	}
}

// Crush and a shell cannot be told which model to run when lich starts them, so
// a caller that names one is told — silently dropping it would hand back a
// session that looks like the one asked for and runs a different model.
func TestOpenRefusesAModelTheProviderCannotBeTold(t *testing.T) {
	for _, kind := range []string{"crush", "shell"} {
		t.Run(kind, func(t *testing.T) {
			svc, sessions, _, term, _ := newService(t)

			_, err := svc.Open("s1", "", kind, "", "", "opus")
			if err == nil {
				t.Fatal("opened a session on a model its provider never sees")
			}
			if !strings.Contains(err.Error(), "model") {
				t.Errorf("error = %q, want it to name the model", err)
			}
			if len(sessions.rows) != 0 || len(term.spawns) != 0 {
				t.Error("the refusal still created something")
			}
		})
	}
}

func TestOpenOnAWorktreeBranchesOffTheCurrentBranch(t *testing.T) {
	svc, sessions, worktrees, term, _ := newService(t)

	opened, err := svc.Open("s1", "", "", "auth-fix", "", "")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	if len(worktrees.created) != 1 {
		t.Fatalf("created %d worktrees, want 1", len(worktrees.created))
	}
	if got := worktrees.created[0]; got != (createdWorktree{"/src/lich", "p1", "auth-fix", "main", false}) {
		t.Errorf("created = %+v, want it off the project's current branch", got)
	}
	// The card is named after the checkout, not numbered — the same thing the
	// window's worktree flow does, and what makes the session addressable by the
	// branch whoever asked for it is thinking in.
	if opened.Label != "auth-fix" {
		t.Errorf("label = %q, want the worktree's name", opened.Label)
	}
	if opened.Path != "/wt/auth-fix" || sessions.rows[0].path != "/wt/auth-fix" {
		t.Errorf("path %q / row %q, want the checkout", opened.Path, sessions.rows[0].path)
	}
	if term.spawns[0].cwd != "/wt/auth-fix" {
		t.Errorf("spawned in %q, want the checkout", term.spawns[0].cwd)
	}
	if !term.spawns[0].setup {
		t.Error("skipped the worktree setup script on a fresh checkout")
	}
}

func TestOpenTracksARemoteBase(t *testing.T) {
	svc, _, worktrees, _, _ := newService(t)
	worktrees.branches = project.Branches{
		Local:  []string{"main"},
		Remote: []string{"origin/release"},
	}

	if _, err := svc.Open("s1", "", "", "hotfix", "origin/release", ""); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if got := worktrees.created[0]; !got.remote || got.base != "origin/release" {
		t.Errorf("created = %+v, want the remote base fetched and tracked", got)
	}
}

// A branch another worktree holds is reported among the checkouts rather than
// among the local branches, and is still a base a new worktree can branch off.
func TestOpenBranchesOffABranchAnotherWorktreeHolds(t *testing.T) {
	svc, _, worktrees, _, _ := newService(t)
	worktrees.branches = project.Branches{
		Local:     []string{"main"},
		Worktrees: []project.Worktree{{Name: "amber-otter", Path: "/wt/amber-otter"}},
	}

	if _, err := svc.Open("s1", "", "", "follow-up", "amber-otter", ""); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if got := worktrees.created[0]; got.base != "amber-otter" || got.remote {
		t.Errorf("created = %+v, want a local base", got)
	}
}

func TestOpenRefusesABaseThatIsNoBranch(t *testing.T) {
	svc, sessions, worktrees, _, _ := newService(t)
	worktrees.branches = project.Branches{Local: []string{"main"}}

	// git would branch off any revision this happened to resolve to, leaving a
	// checkout nobody asked for; a name that is not a branch is a typo.
	_, err := svc.Open("s1", "", "", "hotfix", "mian", "")
	if err == nil {
		t.Fatal("created a worktree off a base that is not a branch")
	}
	if len(worktrees.created) != 0 || len(sessions.rows) != 0 {
		t.Error("wrote something for a base that was refused")
	}
}

func TestOpenLeavesNoSessionWhenTheWorktreeFails(t *testing.T) {
	svc, sessions, worktrees, term, events := newService(t)
	worktrees.createErr = errors.New("branch already exists")

	if _, err := svc.Open("s1", "", "", "auth-fix", "", ""); err == nil {
		t.Fatal("opened a session on a worktree that was never created")
	}
	if len(sessions.rows) != 0 || len(term.spawns) != 0 || len(events.events) != 0 {
		t.Error("a failed checkout left a row, a terminal or a card behind")
	}
}

// A spawn that fails is still a session: the row is written and the card has to
// appear, or the user cannot reach what went wrong.
func TestOpenKeepsTheCardWhenTheTerminalFails(t *testing.T) {
	svc, sessions, _, term, events := newService(t)
	term.err = errors.New("claude: not found")

	_, err := svc.Open("s1", "", "", "", "", "")
	if err == nil {
		t.Fatal("reported success for a terminal that never started")
	}
	if !strings.Contains(err.Error(), "Session 4") {
		t.Errorf("error = %q, want the session it left behind named", err)
	}
	if len(sessions.rows) != 1 {
		t.Errorf("wrote %d rows, want the session kept", len(sessions.rows))
	}
	if len(events.events) != 1 {
		t.Errorf("emitted %d events, want the card announced anyway", len(events.events))
	}
}

func TestOpenWithoutAnEventsSinkStillOpens(t *testing.T) {
	sessions := &fakeSessions{projects: workspace()}
	svc := New(sessions, &fakeWorktrees{branch: "main"}, &fakeTerminal{}, nil)

	if _, err := svc.Open("s1", "", "", "", "", ""); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if len(sessions.rows) != 1 {
		t.Errorf("wrote %d rows, want 1", len(sessions.rows))
	}
}

func TestOpenRefusesADetachedHeadWithoutABase(t *testing.T) {
	svc, _, worktrees, _, _ := newService(t)
	worktrees.branch = ""

	_, err := svc.Open("s1", "", "", "auth-fix", "", "")
	if err == nil {
		t.Fatal("created a worktree off nothing")
	}
	if len(worktrees.created) != 0 {
		t.Error("reached git with an empty base")
	}
}

// A checkout is opened rather than created whatever case its branch is typed in:
// the name lookups around it all fold case (resolveBase, labelTaken), and a
// second branch beside the one asked for is the worst way to answer.
func TestOpenReusesACheckoutWhoseNameIsSpeltInAnotherCase(t *testing.T) {
	svc, sessions, worktrees, term, _ := newService(t)
	worktrees.checkouts = []project.Worktree{{Name: "auth-fix", Path: "/wt/auth-fix"}}

	opened, err := svc.Open("s1", "", "", "Auth-Fix", "", "")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if len(worktrees.created) != 0 {
		t.Errorf("created %+v, want the checkout that is already there", worktrees.created)
	}
	if opened.Path != "/wt/auth-fix" || sessions.rows[0].path != "/wt/auth-fix" {
		t.Errorf("path %q / row %q, want the existing checkout", opened.Path, sessions.rows[0].path)
	}
	if term.spawns[0].setup {
		t.Error("ran the setup script again in a checkout that already ran it")
	}
}

// A base is what a worktree branches off, so one without a worktree describes
// nothing. Accepting it silently drops it, and the session lands on a branch
// whoever asked never named.
func TestOpenRefusesABaseWithoutAWorktree(t *testing.T) {
	svc, sessions, worktrees, term, _ := newService(t)

	_, err := svc.Open("s1", "", "", "", "main", "")
	if err == nil {
		t.Fatal("accepted a base with no worktree to start")
	}
	if !strings.Contains(err.Error(), "main") {
		t.Errorf("error = %q, want the base it refused named", err)
	}
	if len(sessions.rows) != 0 || len(worktrees.created) != 0 || len(term.spawns) != 0 {
		t.Error("a refused open still created something")
	}
}

// Every RPC call runs on its own goroutine, and the label counter is read from
// the workspace and written back — two opens at once must not walk away with the
// same number.
func TestConcurrentOpensDoNotShareALabel(t *testing.T) {
	svc, sessions, _, _, _ := newService(t)

	var wg sync.WaitGroup
	for range 2 {
		wg.Go(func() {
			if _, err := svc.Open("s1", "", "", "", "", ""); err != nil {
				t.Errorf("Open: %v", err)
			}
		})
	}
	wg.Wait()

	if len(sessions.rows) != 2 {
		t.Fatalf("wrote %d rows, want 2", len(sessions.rows))
	}
	if sessions.rows[0].label == sessions.rows[1].label {
		t.Errorf("both sessions took the label %q — `lich send` cannot tell them apart", sessions.rows[0].label)
	}
}

func TestOpenReportsAnUnreadableWorkspace(t *testing.T) {
	svc, sessions, _, term, _ := newService(t)
	sessions.loadErr = errors.New("database is locked")

	_, err := svc.Open("s1", "", "", "", "", "")
	if err == nil {
		t.Fatal("opened a session against a workspace it could not read")
	}
	if !strings.Contains(err.Error(), "database is locked") {
		t.Errorf("error = %q, want the store's own reason kept", err)
	}
	if len(term.spawns) != 0 {
		t.Error("started a terminal for a session that was never created")
	}
}

// A row that was refused is a session that does not exist: no PTY, and no card
// announcing one.
func TestOpenStartsNothingWhenTheRowIsRefused(t *testing.T) {
	svc, sessions, _, term, events := newService(t)
	sessions.addErr = errors.New("disk full")

	if _, err := svc.Open("s1", "", "", "", "", ""); err == nil {
		t.Fatal("opened a session whose row was never written")
	}
	if len(term.spawns) != 0 || len(events.events) != 0 {
		t.Error("a refused row left a terminal or a card behind")
	}
}

func TestOpenReportsAnUnreadableBranchList(t *testing.T) {
	svc, sessions, worktrees, _, _ := newService(t)
	worktrees.listErr = errors.New("not a git repository")

	_, err := svc.Open("s1", "", "", "hotfix", "main", "")
	if err == nil {
		t.Fatal("created a worktree off a base it could not check")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("error = %q, want git's own reason kept", err)
	}
	if len(worktrees.created) != 0 || len(sessions.rows) != 0 {
		t.Error("wrote something for a base that was never resolved")
	}
}

// Two projects can carry one name — the same directory opened twice, or two
// checkouts of one repository — and a name that addresses both addresses
// neither.
func TestOpenRefusesAProjectNameThatNamesTwo(t *testing.T) {
	sessions := &fakeSessions{projects: []store.Project{
		{ID: "p1", Name: "lich", Path: "/src/lich", NextSeq: 1},
		{ID: "p2", Name: "lich", Path: "/other/lich", NextSeq: 1},
	}}
	svc := New(sessions, &fakeWorktrees{branch: "main"}, &fakeTerminal{}, &fakeEvents{})

	_, err := svc.Open("", "lich", "", "", "", "")
	if err == nil {
		t.Fatal("opened a session in one of two projects that answer to the same name")
	}
	if !strings.Contains(err.Error(), "told apart") {
		t.Errorf("error = %q, want it to say the two cannot be told apart", err)
	}
	if len(sessions.rows) != 0 {
		t.Error("wrote a row for an ambiguous project")
	}
}

func TestWorktreesReportsAnUnreadableWorkspace(t *testing.T) {
	svc, sessions, _, _, _ := newService(t)
	sessions.loadErr = errors.New("database is locked")

	if _, err := svc.Worktrees("s1", ""); err == nil {
		t.Fatal("listed the checkouts of a workspace it could not read")
	}
}

func TestWorktreesReportsAnUnreadableRepository(t *testing.T) {
	svc, _, worktrees, _, _ := newService(t)
	worktrees.listErr = errors.New("not a git repository")

	if _, err := svc.Worktrees("s1", ""); err == nil {
		t.Fatal("listed the checkouts of a repository git refused to read")
	}
}

func TestSessionIDsDoNotRepeat(t *testing.T) {
	svc, sessions, _, _, _ := newService(t)

	for range 2 {
		if _, err := svc.Open("s1", "", "", "", "", ""); err != nil {
			t.Fatalf("Open: %v", err)
		}
	}
	if sessions.rows[0].sessionID == sessions.rows[1].sessionID {
		t.Fatal("two sessions took the same id — the second would overwrite the first's row")
	}
}

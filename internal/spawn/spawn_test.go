package spawn

import (
	"errors"
	"strings"
	"testing"

	"github.com/omartelo/lich/internal/project"
	"github.com/omartelo/lich/internal/store"
)

// added is one session row the fake store was asked to write.
type added struct {
	projectID string
	sessionID string
	label     string
	kind      string
	path      string
	nextSeq   int
}

type fakeSessions struct {
	projects []store.Project
	loadErr  error
	rows     []added
	addErr   error
	// models records the model written on each session row, keyed by session id.
	models   map[string]string
	modelErr error
	// deleted/parked/purged record what a close asked the store to do, and
	// against which active session it left the project.
	deleted []closedRow
	parked  []closedRow
	purged  []string
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

func (f *fakeSessions) PurgeWorktreeSessions(_, path string) error {
	f.purged = append(f.purged, path)
	return nil
}

func (f *fakeSessions) LoadState() ([]store.Project, error) {
	return f.projects, f.loadErr
}

func (f *fakeSessions) AddSession(projectID, sessionID, label, kind, path string, nextSeq int) error {
	if f.addErr != nil {
		return f.addErr
	}
	f.rows = append(f.rows, added{projectID, sessionID, label, kind, path, nextSeq})
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
	spawns []started
	err    error
	closed []string
}

func (f *fakeTerminal) Close(id string) error {
	f.closed = append(f.closed, id)
	return nil
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
	if row != (added{"p1", opened.ID, "Session 4", "codex", "", 5}) {
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
// is an override — losing it costs the provider's default and nothing more.
func TestOpenAnnouncesTheCardEvenWhenTheModelCannotBeRecorded(t *testing.T) {
	svc, sessions, _, _, events := newService(t)
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

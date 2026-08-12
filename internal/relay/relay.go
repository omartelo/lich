// Package relay lets one lich session hand a prompt to another and get an
// answer back, for the providers whose own CLI has no cross-session channel of
// its own. Claude Code has one; Codex, OpenCode and Crush do not, and this
// works the same for all four.
//
// The delivery is deliberately dumb: lich types the message at the target's
// prompt and submits it, exactly as the user would. What it never does is read
// the answer off the terminal — a TUI's output is boxes, spinners and ANSI, and
// parsing it would mean a parser per provider, each hostage to a release.
// Instead the message it types asks the agent to report back by running
// `lich reply <ticket>`, so the answer is *written by the agent that produced
// it*. That is what keeps this provider-agnostic: anything that can run a shell
// command can answer.
package relay

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/omartelo/lich/internal/store"
)

const (
	// DefaultWait is how long Send blocks before handing the caller a ticket to
	// come back with. It sits under the 120s an agent's shell tool typically
	// allows a command, so a long errand ends in an answer this side chose —
	// "still working, wait on this ticket" — instead of a killed process and no
	// way back to the conversation.
	DefaultWait = 100 * time.Second
	// MaxWait bounds an explicit timeout. Past this the caller should be
	// polling with Wait, not holding a socket.
	MaxWait = 30 * time.Minute
	// ticketTTL is how long an unanswered ticket stays waitable. A target that
	// never replies would otherwise leak one entry per attempt for the life of
	// the process.
	ticketTTL = time.Hour
	// defaultReceiptWindow is how long a task has to be picked up by the agent it
	// was typed at. A provider that read it reports UserPromptSubmit within a
	// second or two; this is generous against a machine under load, and short
	// enough that the sender learns in one tool call rather than at the ticket's
	// expiry an hour later.
	defaultReceiptWindow = 30 * time.Second
	// promptLimit bounds one relayed prompt. The message is typed into a TUI a
	// character at a time; a megabyte of it is a hang, not a prompt.
	promptLimit = 8192
	// answerLimit bounds one reply. Generous — an answer is a summary, and the
	// caller reads it as command output.
	answerLimit = 64 * 1024
	// readyPoll is how often awaitReady asks whether a target's agent is up. A
	// setup script runs for tens of seconds at least, so this only has to be
	// quick against a human's patience, not against the machine.
	readyPoll = 250 * time.Millisecond
)

// How lich's own MCP server (internal/cli) is registered and what it calls its
// tools. They live here rather than beside the server because two other places
// need them and neither may depend on that one: the message this package
// composes has to name the reply tool, and the spawn path (internal/terminal)
// has to name the command to register. The server itself already depends on
// this package, so this is the only direction that closes no cycle.
const (
	// MCPServerName namespaces the tools in a client's list
	// (`mcp__lich__send_to_session` in Claude Code) and is the key lich
	// registers itself under.
	MCPServerName = "lich"
	// MCPSubcommand is the `lich` subcommand that serves them.
	MCPSubcommand = "mcp"
	// ToolReply answers a relayed message. It is the one tool named in prose,
	// because the receiving agent is told what to call.
	ToolReply = "reply_to_session"
)

// Status values a Result carries.
const (
	// StatusAnswered means the target's agent replied and Answer holds it.
	StatusAnswered = "answered"
	// StatusPending means the message was delivered and the wait ran out first.
	// The ticket is still live: Wait picks the same answer up later.
	StatusPending = "pending"
	// StatusUnread means the task was typed at the target's prompt and nothing
	// there read it: the session never started working. See watchReceipt.
	StatusUnread = "unread"
	// StatusUnanswered means the target worked through the request and ended its
	// turn without replying here. Its answer, if it wrote one, is in that
	// session's own terminal and nowhere lich can read it — which is what
	// happens when an agent answers over a channel of its provider's own.
	StatusUnanswered = "unanswered"
)

// Session states the relay watches, spelled as the hook contract reports them
// (docs/hooks/session-state.md).
const (
	stateBusy    = "busy"
	stateDone    = "done"
	stateIdle    = "idle"
	stateWaiting = "waiting"
)

// RelayEventName carries which session is waiting on which, so the sidebar can
// say it. Global rather than per-session because its consumer — one store keyed
// by id — outlives any one card, exactly as the status event's does.
const RelayEventName = "session-relay"

// Directions a RelayEvent carries. An empty direction clears the mark: the
// request it named is over, answered or expired.
const (
	DirectionOut = "out"
	DirectionIn  = "in"
)

// StalledEventName carries a request whose target ended its turn without
// answering through lich. The window turns it into a toast that opens the
// target's card, because the answer — if there is one — is on that screen and
// the person who asked has no other way to know where it went.
const StalledEventName = "session-relay-stalled"

// StalledEvent is the payload of StalledEventName: who asked ("" when the
// request came from the command line rather than a session), and the session
// that has whatever was produced.
type StalledEvent struct {
	ID       string `json:"id"`
	TargetID string `json:"targetId"`
	Target   string `json:"target"`
}

// RelayEvent is the payload of RelayEventName: the session whose mark changed,
// the label at the other end, and which way the request runs. Peer is empty
// when the other end is not a session at all — the CLI run from a script or a
// shell — which the card words its own way.
type RelayEvent struct {
	ID        string `json:"id"`
	Peer      string `json:"peer"`
	Direction string `json:"direction"`
}

// Sessions is the persistence the relay reads: every open session, so a label
// can be resolved to the session it names and the caller can be told what it
// may address. The store implements it.
type Sessions interface {
	LoadState() ([]store.Project, error)
}

// Events is where the relay announces a request in flight. The app's event hub
// implements it; a nil one leaves the feature working and silent, which is the
// state a test that only exercises delivery is in.
type Events interface {
	Emit(name string, data any)
}

// Terminal is the PTY side: whether a session has a process running right now,
// whether what runs in it is the agent rather than the checkout's setup script,
// and how to type at it. The terminal service implements it.
type Terminal interface {
	Live(id string) bool
	Ready(id string) bool
	Write(id, data string) error
}

// Peer is one session a caller may address: the label it is addressed by, the
// name it answers to in Claude Code's peer roster, the project it belongs to
// (labels are unique within a project, not across them), and what is running in
// it.
//
// Both names are published because both reach this session and an agent sees
// them in different places — the label on the card, the roster name in
// `/list-agents` and in what a mention writes at a prompt. Showing one and
// accepting the other is what made an agent treat a single session as two.
type Peer struct {
	Label   string `json:"label"`
	Name    string `json:"name"`
	Project string `json:"project"`
	Kind    string `json:"kind"`
}

// Result is what a caller gets back from Send or Wait. Answer is empty unless
// Status is StatusAnswered.
type Result struct {
	Ticket string `json:"ticket"`
	Target string `json:"target"`
	Status string `json:"status"`
	Answer string `json:"answer"`
}

// ticket is one outstanding errand. answer is written once, before done is
// closed, so every waiter reads it safely after the close.
//
// It carries both ends' ids and labels because the marks raised when it opens
// have to be taken down when it closes, and by then the roster may have moved
// on: a session renamed, or closed altogether, would otherwise leave a mark
// nothing can clear.
type ticket struct {
	fromID   string
	sender   string
	targetID string
	target   string
	created  time.Time
	// delivered is when the message actually reached the target's PTY — zero
	// while the ticket is still waiting out the target's setup script. Turn
	// accounting reads it twice over: a turn observed on the target says nothing
	// about an undelivered message, and when one turn ends with several errands
	// open, the oldest delivery is the one that turn belongs to.
	delivered time.Time
	done      chan struct{}
	answer    string

	// attended is how many callers are blocked on this ticket right now. An
	// answer that lands while nobody is waiting has nowhere to be returned, so
	// it is typed at the sender's prompt instead — the same way the request
	// reached the target.
	attended int
	// answered records that the answer is in, for the waiter that gave up in the
	// same instant it arrived: it leaves last and has to notice it was the one
	// holding the ticket.
	answered bool

	// stalled closes when the target finished a turn without replying here. See
	// Observe for how a turn is told apart from the one already running when the
	// message arrived.
	stalled chan struct{}
	// unread closes when the target never reacted to the task at all. See
	// watchReceipt.
	unread chan struct{}
	// sawBusy is whether the target has been working since this ticket's turn
	// began; a turn that ends without it never started here.
	sawBusy bool
	// skipTurns is how many turn endings belong to work that was already running
	// when the message was delivered. Every provider queues typed input, so a
	// message handed to a busy session is answered a turn later — and the ending
	// of the turn in progress says nothing about this request.
	skipTurns int
}

// Service relays prompts between sessions. Tickets live in memory only: one
// exists for as long as its errand does, and a lich that restarted has no PTY
// left to answer into anyway.
type Service struct {
	mu      sync.Mutex
	tickets map[string]*ticket
	// state is the last thing each session reported, so a delivery knows whether
	// it is landing in the middle of a turn. Only sessions the relay has heard
	// about appear; an unknown one is treated as not working, which is what a
	// session with no hooks installed looks like.
	state map[string]string

	sessions Sessions
	term     Terminal
	events   Events
	now      func() time.Time
	// submitDelay separates the paste from the Enter that sends it (see
	// defaultSubmitDelay). A field so the suite can drop it to zero: the fakes
	// have no TUI to settle, and paying it per test would buy nothing.
	submitDelay time.Duration
	// receiptWindow is how long a delivered task has to be picked up before it
	// is called unread (see watchReceipt). A field for the same reason.
	receiptWindow time.Duration
	// plugins answers what a provider's sessions can do, which depends on state
	// this package has none of: whether the companion plugin is installed there,
	// and whether it is new enough to carry lich's own operations. Nil — the
	// state a test that does not care is in — reads as "nothing is installed":
	// no delivery is checked, and a relayed message names the command line.
	plugins Plugins
}

// Plugins is what the relay needs to know about the companion plugin
// (internal/agentplugin implements it). Both questions are about a provider's
// sessions rather than about one session, because that is the grain the plugin
// is installed at.
type Plugins interface {
	// Installed is whether those sessions report their state to lich at all,
	// which is what makes a missing report mean something (see watchReceipt).
	Installed(kind string) bool
	// HasTools is whether they can call lich's own operations, which decides
	// whether a relayed message names a tool or the shell command.
	HasTools(kind string) bool
}

// New returns a relay reading its roster from sessions, typing through term and
// announcing what is in flight on events.
func New(sessions Sessions, term Terminal, events Events) *Service {
	return &Service{
		tickets:       make(map[string]*ticket),
		state:         make(map[string]string),
		sessions:      sessions,
		term:          term,
		events:        events,
		now:           time.Now,
		submitDelay:   defaultSubmitDelay,
		receiptWindow: defaultReceiptWindow,
	}
}

// SetPlugins wires what the relay can ask about the companion plugin. Without
// it a target that never reacts is indistinguishable from one lich cannot hear,
// and every relayed message names the command line. Called at startup, before
// any errand exists.
func (s *Service) SetPlugins(plugins Plugins) {
	s.plugins = plugins
}

// Peers lists the live sessions fromID may address, in the order the sidebar
// shows them. A session with no PTY running is left out: there is nothing there
// to type at, and offering it would only produce a message nobody ever reads.
func (s *Service) Peers(fromID string) ([]Peer, error) {
	found, err := s.roster(fromID)
	if err != nil {
		return nil, err
	}
	peers := make([]Peer, 0, len(found))
	for _, c := range found {
		peers = append(peers, c.Peer)
	}
	return peers, nil
}

// Send types prompt at the prompt of the session labelled target and waits for
// its agent to answer. project narrows the search when the same label exists in
// more than one project; empty searches them all. waitSeconds bounds the wait —
// 0 uses DefaultWait.
//
// The wait running out is not a failure: the message was delivered, and the
// returned ticket is what picks the answer up later.
func (s *Service) Send(fromID, target, project, prompt string, waitSeconds int) (Result, error) {
	prompt = sanitize(prompt)
	if strings.TrimSpace(prompt) == "" {
		return Result{}, fmt.Errorf("nothing to send: the prompt is empty")
	}
	if len(prompt) > promptLimit {
		return Result{}, fmt.Errorf("prompt is %d bytes, over the %d limit", len(prompt), promptLimit)
	}

	dest, err := s.resolve(fromID, target, project)
	if err != nil {
		return Result{}, err
	}
	sender := s.labelOf(fromID)

	id, err := newTicketID()
	if err != nil {
		return Result{}, err
	}
	t := &ticket{
		fromID:   fromID,
		sender:   sender,
		targetID: dest.ID,
		target:   dest.Peer.Label,
		created:  s.now(),
		done:     make(chan struct{}),
		stalled:  make(chan struct{}),
		unread:   make(chan struct{}),
	}
	s.mu.Lock()
	expired := s.sweep()
	busy := s.state[dest.ID] == stateBusy
	if busy {
		t.skipTurns = 1
	}
	s.tickets[id] = t
	s.mu.Unlock()
	s.clearAll(expired)

	// One deadline covers the whole call: the setup wait and the answer wait
	// spend the same budget. Each taking a full waitFor of its own would let a
	// Send block for twice what the caller asked — past the HTTP client's own
	// budget (internal/cli, waitBudget), which would report a timeout on a
	// message that was in fact delivered.
	deadline := s.now().Add(waitFor(waitSeconds))
	if err := s.awaitReady(dest, deadline); err != nil {
		s.mu.Lock()
		delete(s.tickets, id)
		s.mu.Unlock()
		return Result{}, err
	}
	message := compose(sender, id, prompt, s.offersTools(dest.Peer.Kind))
	if err := s.deliver(dest.ID, message); err != nil {
		s.mu.Lock()
		delete(s.tickets, id)
		s.mu.Unlock()
		return Result{}, fmt.Errorf("deliver to %q: %w", dest.Peer.Label, err)
	}
	s.mu.Lock()
	t.delivered = s.now()
	s.mu.Unlock()
	// Announced only once the message is actually in the PTY: a mark raised
	// before the write would survive a delivery that never happened.
	s.announce(t.targetID, t.sender, DirectionIn)
	s.announce(t.fromID, t.target, DirectionOut)
	// A target that was already working is not checked: it will read this at the
	// end of the turn it is in, whenever that is, and its provider is busy the
	// whole time — there is nothing here to tell apart.
	if !busy && s.reportsState(dest.Peer.Kind) {
		go s.watchReceipt(id, t)
	}
	return s.await(id, t, deadline.Sub(s.now())), nil
}

// deliver puts message at a session's prompt and sends it — two writes, a beat
// apart, because the Enter has to arrive after the prompt has taken the paste
// (see defaultSubmitDelay).
func (s *Service) deliver(sessionID, message string) error {
	if err := s.term.Write(sessionID, paste(message)); err != nil {
		return err
	}
	time.Sleep(s.submitDelay)
	return s.term.Write(sessionID, submit)
}

// awaitReady blocks until a target's agent is the program reading its PTY.
//
// A session opened on a fresh worktree runs the project's setup script in that
// PTY first, and the script can take minutes. It is live the whole time, and a
// message typed into it reaches `pnpm install`, which reads and discards it —
// so the request never existed, and the sender waits out a ticket nobody was
// asked to answer. Seen on the first real run of a worktree session.
//
// Waiting is bounded by the caller's own deadline — the same one the answer is
// waited on, so setup and answer spend one budget between them: there is no
// point holding a message for a setup that outlasts the errand. Running out is
// an error rather than a delivery, because a message nobody can be given is not
// one that was sent.
func (s *Service) awaitReady(dest candidate, deadline time.Time) error {
	if s.term.Ready(dest.ID) {
		return nil
	}
	for {
		time.Sleep(readyPoll)
		if s.term.Ready(dest.ID) {
			return nil
		}
		if !s.term.Live(dest.ID) {
			return fmt.Errorf("%q stopped before its agent started", dest.Peer.Label)
		}
		if s.now().After(deadline) {
			// Which of the two it is, lich cannot see from here, and naming only
			// the setup script sent whoever read this hunting a script the
			// project may not even have. What is certain is on that session's own
			// screen, so the message says to go and look.
			return fmt.Errorf(
				"%q has not stopped drawing, so lich cannot tell what is reading its "+
					"terminal: the project's worktree setup script may still be running in "+
					"it, or its provider may still be starting up. Nothing was sent — open "+
					"that session to see what is on its screen, and try again once it is at "+
					"a prompt",
				dest.Peer.Label,
			)
		}
	}
}

// reportsState is whether a provider tells lich what it is doing, which is what
// makes its silence mean something (docs/hooks/session-state.md).
func (s *Service) reportsState(kind string) bool {
	return s.plugins != nil && s.plugins.Installed(kind)
}

// offersTools is whether the target can answer with a tool rather than with the
// shell command. Naming a tool a session does not have is worse than naming the
// command, which works everywhere — so an unknown answer is no.
func (s *Service) offersTools(kind string) bool {
	return s.plugins != nil && s.plugins.HasTools(kind)
}

// watchReceipt closes a ticket whose task nobody ever picked up.
//
// Delivery has always been provable — the bytes reached the PTY — and receipt
// never was. Between the two sits everything that can be on a terminal instead
// of a prompt: Claude Code asking whether a new directory is trusted, a
// provider still taking over the tty, a dialog left open by whoever was here
// last. The task is typed into that, and it is gone. Nothing fails: the sender
// waits out a ticket nobody was asked to answer, and finds out an hour later
// when it expires, which is how this feature's own first users found it.
//
// An agent that reads a prompt reports UserPromptSubmit within a second or two,
// so the absence of that report inside the window is the answer. It is only
// asked of providers that report at all — the rest have always been silent, and
// silence has to mean something to be read as anything.
func (s *Service) watchReceipt(id string, t *ticket) {
	timer := time.NewTimer(s.receiptWindow)
	defer timer.Stop()
	select {
	case <-t.done:
		return
	case <-t.stalled:
		return
	case <-timer.C:
	}

	s.mu.Lock()
	current, live := s.tickets[id]
	if !live || current != t || t.sawBusy {
		s.mu.Unlock()
		return
	}
	delete(s.tickets, id)
	close(t.unread)
	unattended := t.attended == 0
	s.mu.Unlock()

	s.clear(t)
	if unattended {
		s.tellSender(t, unreadNotice(t.target))
	}
}

// Wait blocks on an already-delivered ticket for another round, so a caller
// whose first wait ran out can come back without sending the message twice.
func (s *Service) Wait(ticketID string, waitSeconds int) (Result, error) {
	s.mu.Lock()
	expired := s.sweep()
	t, ok := s.tickets[ticketID]
	s.mu.Unlock()
	s.clearAll(expired)
	if !ok {
		return Result{}, fmt.Errorf("unknown ticket %q — it was answered long ago, or expired", ticketID)
	}
	return s.await(ticketID, t, waitFor(waitSeconds)), nil
}

// Reply hands an answer back to whoever is waiting on ticketID. It is what the
// message composed by Send asks the receiving agent to run.
func (s *Service) Reply(ticketID, answer string) error {
	answer = sanitize(answer)
	if len(answer) > answerLimit {
		// The cut is in bytes and can land inside a rune; the tail is typed into
		// a PTY, and half a rune there is garbage keystrokes, not text.
		answer = strings.ToValidUTF8(answer[:answerLimit], "")
	}

	s.mu.Lock()
	t, ok := s.tickets[ticketID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("unknown ticket %q — it was answered already, or expired", ticketID)
	}
	select {
	case <-t.done:
		s.mu.Unlock()
		return fmt.Errorf("ticket %q was already answered", ticketID)
	default:
	}
	t.answer = answer
	t.answered = true
	close(t.done)
	// The errand is over the moment the answer lands, whether or not anyone is
	// still waiting on it: a sender whose wait ran out has moved on, and the
	// mark on both cards has to go with the ticket rather than with the waiter.
	delete(s.tickets, ticketID)
	unattended := t.attended == 0
	s.mu.Unlock()

	s.clear(t)
	if unattended {
		s.returnAnswer(t)
	}
	return nil
}

// returnAnswer types the answer at the sender's own prompt, the same way the
// request reached the target. It runs when nobody is blocked on the ticket
// anymore, which is the normal case for anything that took longer than a tool
// call may sit still: the answer arrives when it arrives, in the session that
// asked, instead of being lost because the asker stopped listening.
//
// A sender that is not a session at all — the `lich` command from a script —
// has no prompt to type at. Such a caller waits or does without.
func (s *Service) returnAnswer(t *ticket) {
	if t.answer == "" {
		return
	}
	s.tellSender(t, fmt.Sprintf(
		"[lich] Answer from session %q, to the request you sent it:\n\n%s",
		t.target, t.answer,
	))
}

// tellSender types one line of news at the prompt of whoever asked, the same
// way their request reached the target. A sender that is not a session at all —
// the `lich` command from a script — has no prompt to type at, and hears this
// only if it is still waiting.
func (s *Service) tellSender(t *ticket, message string) {
	if t.fromID == "" {
		return
	}
	if err := s.deliver(t.fromID, message); err != nil {
		slog.Warn("relay: news not delivered", "session", t.fromID, "err", err)
	}
}

// await blocks on one ticket until it is answered or the wait runs out. It only
// reads: the ticket is dropped by whoever closes it (Reply, or sweep), because
// a wait that expired leaves an errand that is still open and still has to be
// waitable, and an answer has to reach a caller who stopped waiting for it too.
func (s *Service) await(id string, t *ticket, wait time.Duration) Result {
	timer := time.NewTimer(wait)
	defer timer.Stop()

	s.mu.Lock()
	t.attended++
	s.mu.Unlock()

	// An answer that is already in outranks everything: a target may reply and
	// end its turn in the same breath, and a select over two ready channels
	// picks between them at random.
	select {
	case <-t.done:
		s.leave(t)
		return Result{Ticket: id, Target: t.target, Status: StatusAnswered, Answer: t.answer}
	default:
	}

	select {
	case <-t.done:
		s.leave(t)
		return Result{Ticket: id, Target: t.target, Status: StatusAnswered, Answer: t.answer}
	case <-t.stalled:
		s.leave(t)
		return Result{Ticket: id, Target: t.target, Status: StatusUnanswered}
	case <-t.unread:
		s.leave(t)
		return Result{Ticket: id, Target: t.target, Status: StatusUnread}
	case <-timer.C:
		return Result{Ticket: id, Target: t.target, Status: s.giveUp(t)}
	}
}

// leave drops this caller's claim on a ticket it is carrying the answer out of.
func (s *Service) leave(t *ticket) {
	s.mu.Lock()
	t.attended--
	s.mu.Unlock()
}

// giveUp drops the claim of a caller whose wait ran out and returns the status
// that caller should hear. It catches the races that would lose what became of
// the errand, all of the same shape: the ticket closing in the same instant,
// seeing this caller still attending, and leaving the news for it — after it
// had already stopped listening. Whoever leaves last owns what is left behind.
//
// A reply is typed at the sender's prompt, because an answer outlives the wait
// it missed. The receipt window closing the ticket unread — and a turn ending
// with no answer — are told to the caller instead: it is still here to hear
// them, and the alternative is reporting the errand as still in progress, on a
// ticket already out of the map.
func (s *Service) giveUp(t *ticket) string {
	s.mu.Lock()
	t.attended--
	last := t.attended == 0
	orphaned := t.answered && last
	unread, stalled := false, false
	if last && !t.answered {
		select {
		case <-t.unread:
			unread = true
		default:
		}
		select {
		case <-t.stalled:
			stalled = true
		default:
		}
	}
	s.mu.Unlock()
	if orphaned {
		s.returnAnswer(t)
	}
	if unread {
		return StatusUnread
	}
	if stalled {
		return StatusUnanswered
	}
	return StatusPending
}

// Observe takes one session-state report from the hooks the provider already
// runs (docs/hooks/session-state.md). It exists for one case: a target that
// works through a relayed request and then answers somewhere lich cannot read —
// its provider's own peer channel, or simply out loud to the person watching.
// Nothing would ever close that ticket, and the sender would sit out the full
// wait learning nothing.
//
// A turn ending is only this request's turn when the target has been working
// since the request arrived. Every provider queues typed input, so a message
// handed to a busy session is answered a turn later; the ending of the turn
// already in progress is skipped rather than mistaken for an answer that never
// came. Getting that backwards would report "answered elsewhere" about a
// request the target had not read yet, which is worse than saying nothing.
func (s *Service) Observe(sessionID, state string) {
	s.mu.Lock()
	// waiting keeps the previous state on record. Mid-turn it means a permission
	// prompt — the turn is still open, and a delivery now queues behind it, so it
	// has to keep reading as busy: read as idle it would arm the receipt check
	// against a target that cannot pick anything up until a human answers, and
	// the errand would be reported unread with its message still queued. After
	// done it is the provider's "your turn" nudge, and done is what must survive.
	// idle is SessionEnd: an ended session reports nothing more, and keeping a
	// row for it would grow the map by one dead entry per session for the life
	// of the process — absent reads as "not working", which is what idle means.
	switch state {
	case stateWaiting:
	case stateIdle:
		delete(s.state, sessionID)
	default:
		s.state[sessionID] = state
	}

	var ended []*ticket
	switch state {
	case stateBusy:
		for _, t := range s.tickets {
			// A ticket whose message is still held back by the target's setup
			// has nothing in that PTY yet; whatever runs there is not about it.
			if t.targetID == sessionID && !t.delivered.IsZero() {
				t.sawBusy = true
			}
		}
	case stateIdle:
		// SessionEnd needs no turn to have run: the CLI has left the PTY and
		// nothing there can answer anymore. An undelivered ticket is left for
		// awaitReady, which sees the session die and fails the Send itself.
		for id, t := range s.tickets {
			if t.targetID == sessionID && !t.delivered.IsZero() {
				delete(s.tickets, id)
				close(t.stalled)
				ended = append(ended, t)
			}
		}
	case stateDone:
		if id, t := s.turnErrand(sessionID); t != nil {
			delete(s.tickets, id)
			close(t.stalled)
			ended = append(ended, t)
		}
	}
	// A waiter still holding the line carries the news out through its own
	// select; an errand nobody is attending has to reach the sender's prompt
	// instead, the way an answer or an unread notice would. Without it the
	// promise a pending result makes — "the answer will arrive at your prompt"
	// — is silently never kept.
	var quiet []*ticket
	for _, t := range ended {
		if t.attended == 0 {
			quiet = append(quiet, t)
		}
	}
	s.mu.Unlock()

	s.clearAll(ended)
	for _, t := range ended {
		if s.events != nil {
			s.events.Emit(StalledEventName, StalledEvent{
				ID: t.fromID, TargetID: t.targetID, Target: t.target,
			})
		}
	}
	for _, t := range quiet {
		s.tellSender(t, stalledNotice(t.target))
	}
}

// turnErrand decides which of a target's errands the turn that just ended
// belongs to, and returns it — nil when the turn was nobody's. A turn answers
// for at most one errand: every provider queues typed input, so two messages
// delivered to one session run as two turns, and a single done closing both
// would report "answered elsewhere" about a request the target had not read
// yet. The oldest delivery owns the turn; the rest wait for their own, their
// busy marks reset so the next turn starts clean. Called under s.mu.
func (s *Service) turnErrand(sessionID string) (string, *ticket) {
	var oldestID string
	var oldest *ticket
	for id, t := range s.tickets {
		if t.targetID != sessionID || t.delivered.IsZero() {
			continue
		}
		// The turn that was already running when the message was delivered. Its
		// own busy reports say nothing about this request either.
		if t.skipTurns > 0 {
			t.skipTurns--
			t.sawBusy = false
			continue
		}
		if !t.sawBusy {
			continue
		}
		if oldest == nil || t.delivered.Before(oldest.delivered) {
			oldestID, oldest = id, t
		}
	}
	for _, t := range s.tickets {
		if t.targetID == sessionID && t != oldest {
			t.sawBusy = false
		}
	}
	return oldestID, oldest
}

// announce raises or clears one session's mark. Called outside s.mu on every
// path: Emit blocks on a stalled /events client, and holding the relay's lock
// across it would stall every other errand behind one unread window.
func (s *Service) announce(sessionID, peer, direction string) {
	if s.events == nil || sessionID == "" {
		return
	}
	s.events.Emit(RelayEventName, RelayEvent{ID: sessionID, Peer: peer, Direction: direction})
}

// clear takes down both ends' marks for a ticket that is over.
func (s *Service) clear(t *ticket) {
	s.announce(t.targetID, "", "")
	s.announce(t.fromID, "", "")
}

func (s *Service) clearAll(tickets []*ticket) {
	for _, t := range tickets {
		s.clear(t)
	}
}

// waitFor clamps a caller's requested wait into the supported range.
func waitFor(seconds int) time.Duration {
	if seconds <= 0 {
		return DefaultWait
	}
	if d := time.Duration(seconds) * time.Second; d < MaxWait {
		return d
	}
	return MaxWait
}

// sweep drops tickets nobody answered in time and returns them, so the caller
// can take their marks down after releasing s.mu. Called under the lock on the
// paths that already hold it, which is often enough for a map that grows one
// entry per errand.
func (s *Service) sweep() []*ticket {
	var expired []*ticket
	cutoff := s.now().Add(-ticketTTL)
	for id, t := range s.tickets {
		if t.created.Before(cutoff) {
			delete(s.tickets, id)
			expired = append(expired, t)
		}
	}
	return expired
}

// ticketIDBytes is the length of a ticket id before hex encoding. Short enough
// to read back off a terminal, wide enough that two live errands never collide.
const ticketIDBytes = 4

func newTicketID() (string, error) {
	raw := make([]byte, ticketIDBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate ticket id: %w", err)
	}
	return hex.EncodeToString(raw), nil
}

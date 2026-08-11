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
	"strconv"
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
	stateBusy = "busy"
	stateDone = "done"
	stateIdle = "idle"
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
	done     chan struct{}
	answer   string

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
		return Result{}, fmt.Errorf("prompt is %d characters, over the %d limit", len(prompt), promptLimit)
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

	if err := s.awaitReady(dest, waitFor(waitSeconds)); err != nil {
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
	return s.await(id, t, waitSeconds), nil
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
// Waiting is bounded by the caller's own wait: there is no point holding a
// message for a setup that outlasts the errand. Running out is an error rather
// than a delivery, because a message nobody can be given is not one that was
// sent.
func (s *Service) awaitReady(dest candidate, wait time.Duration) error {
	if s.term.Ready(dest.ID) {
		return nil
	}
	deadline := s.now().Add(wait)
	for {
		time.Sleep(readyPoll)
		if s.term.Ready(dest.ID) {
			return nil
		}
		if !s.term.Live(dest.ID) {
			return fmt.Errorf("%q stopped before its agent started", dest.Peer.Label)
		}
		if s.now().After(deadline) {
			return fmt.Errorf(
				"%q is still preparing its checkout — the project's worktree setup script "+
					"is running in it and its agent has not started yet. Nothing was sent. "+
					"Try again once it is up; lich sessions lists it as soon as it can take work",
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
	return s.await(ticketID, t, waitSeconds), nil
}

// Reply hands an answer back to whoever is waiting on ticketID. It is what the
// message composed by Send asks the receiving agent to run.
func (s *Service) Reply(ticketID, answer string) error {
	answer = sanitize(answer)
	if len(answer) > answerLimit {
		answer = answer[:answerLimit]
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
func (s *Service) await(id string, t *ticket, waitSeconds int) Result {
	timer := time.NewTimer(waitFor(waitSeconds))
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
// that caller should hear. It catches the two races that would lose what became
// of the errand, both of the same shape: the ticket closing in the same instant,
// seeing this caller still attending, and leaving the news for it — after it had
// already stopped listening. Whoever leaves last owns what is left behind.
//
// A reply is typed at the sender's prompt, because an answer outlives the wait
// it missed. The receipt window closing the ticket unread is told to the caller
// instead: it is still here to hear it, and the alternative is reporting a task
// nobody read as still in progress, on a ticket already out of the map.
func (s *Service) giveUp(t *ticket) string {
	s.mu.Lock()
	t.attended--
	last := t.attended == 0
	orphaned := t.answered && last
	unread := false
	if last && !t.answered {
		select {
		case <-t.unread:
			unread = true
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
	s.state[sessionID] = state

	var ended []*ticket
	for id, t := range s.tickets {
		if t.targetID != sessionID {
			continue
		}
		switch state {
		case stateBusy:
			t.sawBusy = true
		case stateDone, stateIdle:
			if t.skipTurns > 0 && state == stateDone {
				// The turn that was already running. Its own busy reports say
				// nothing about this request either.
				t.skipTurns--
				t.sawBusy = false
				continue
			}
			// SessionEnd needs no turn to have run: the CLI has left the PTY and
			// nothing there can answer anymore.
			if !t.sawBusy && state != stateIdle {
				continue
			}
			close(t.stalled)
			delete(s.tickets, id)
			ended = append(ended, t)
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

// candidate is a resolved peer together with the session id behind it, which
// callers never see: they address a session by the label on its card.
type candidate struct {
	ID   string
	Peer Peer
}

// roster returns every live session except the caller's own.
func (s *Service) roster(fromID string) ([]candidate, error) {
	projects, err := s.sessions.LoadState()
	if err != nil {
		return nil, fmt.Errorf("read sessions: %w", err)
	}
	var found []candidate
	for _, p := range projects {
		for _, sess := range p.Sessions {
			if sess.ID == fromID || !s.term.Live(sess.ID) {
				continue
			}
			cwd := sess.Path
			if cwd == "" {
				cwd = p.Path
			}
			found = append(found, candidate{
				ID: sess.ID,
				Peer: Peer{
					Label:   sess.Label,
					Name:    RosterName(cwd, sess.ID),
					Project: p.Name,
					Kind:    sess.Kind,
				},
			})
		}
	}
	return found, nil
}

// resolve finds the single live session named by target, optionally narrowed to
// one project. An ambiguous label is an error naming every match, because
// guessing which session a prompt lands in is the one mistake this feature must
// not make.
func (s *Service) resolve(fromID, target, project string) (candidate, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return candidate{}, fmt.Errorf("no target session given")
	}
	found, err := s.roster(fromID)
	if err != nil {
		return candidate{}, err
	}

	// The label first, then the roster name. Both address this session, and an
	// agent may have been handed either — a mention writes the roster name at
	// the prompt, list_sessions prints both. The label wins a tie because it is
	// the name the user chose and the one every message and error quotes.
	matches := matching(found, target, project, func(c candidate) string { return c.Peer.Label })
	if len(matches) == 0 {
		matches = matching(found, target, project, func(c candidate) string { return c.Peer.Name })
	}

	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return candidate{}, fmt.Errorf(
			"no live session named %q%s. %s",
			target, inProject(project), knownLabels(found),
		)
	default:
		return candidate{}, fmt.Errorf(
			"%q names %d live sessions (%s) — narrow it with the project",
			target, len(matches), projectsOf(matches),
		)
	}
}

// labelOf names the sending session for the message it is about to deliver.
// An empty id is a caller with no session at all — the CLI run from a plain
// shell or a script — and stays empty, which compose words differently. A
// caller lich has no record of still gets to send: the receiving agent is told
// the sender is unknown rather than told nothing at all.
func (s *Service) labelOf(id string) string {
	if id == "" {
		return ""
	}
	projects, err := s.sessions.LoadState()
	if err != nil {
		return "unknown"
	}
	for _, p := range projects {
		for _, sess := range p.Sessions {
			if sess.ID == id {
				return sess.Label
			}
		}
	}
	return "unknown"
}

// matching returns the candidates whose name — read out by naming — is target,
// narrowed to one project when one was given.
func matching(found []candidate, target, project string, naming func(candidate) string) []candidate {
	var matches []candidate
	for _, c := range found {
		if !strings.EqualFold(naming(c), target) {
			continue
		}
		if project != "" && !strings.EqualFold(c.Peer.Project, project) {
			continue
		}
		matches = append(matches, c)
	}
	return matches
}

// knownLabels names what is actually reachable, under both names, so a wrong
// name is one step from a right one rather than a guess.
func knownLabels(found []candidate) string {
	if len(found) == 0 {
		return "No other session is live."
	}
	names := make([]string, 0, len(found))
	for _, c := range found {
		names = append(names, fmt.Sprintf("%s (%s)", strconv.Quote(c.Peer.Label), c.Peer.Name))
	}
	return "Live sessions: " + strings.Join(names, ", ") + "."
}

func inProject(project string) string {
	if project == "" {
		return ""
	}
	return fmt.Sprintf(" in project %q", project)
}

func projectsOf(matches []candidate) string {
	names := make([]string, 0, len(matches))
	for _, c := range matches {
		names = append(names, c.Peer.Project)
	}
	return strings.Join(names, ", ")
}

// compose is the message typed at the target's prompt. It names where the
// request came from so the receiving agent knows this did not come from the
// person in front of it, and carries the exact command that sends an answer
// home — the reply path only exists because this text describes it, so the
// agent needs no prior knowledge of the feature.
func compose(sender, ticketID, prompt string, hasTools bool) string {
	return fmt.Sprintf(
		"[lich] %s, not from your own prompt.\n\n%s\n\n%s",
		origin(sender), prompt, replyInstruction(hasTools, ticketID),
	)
}

// replyInstruction tells the receiving agent how to answer. Every agent has a
// shell, so the command is always named; a provider lich registers its MCP
// server with is offered the tool first, because a session that withholds shell
// access would otherwise have no way to answer at all.
//
// The exclusivity is load-bearing, not emphasis. A Claude Code session has a
// peer channel of its own and this message names another session, so answering
// through that channel is the obvious move — and it reaches a sender that is
// blocked on a ticket instead. That happened on the first real run: the target
// replied over its own socket and the errand timed out with the answer already
// written. The ticket is the only route home, and the message has to say so.
func replyInstruction(hasTools bool, ticketID string) string {
	command := fmt.Sprintf("  \"$LICH_BIN\" reply %s \"<your answer>\"", ticketID)
	route := "When you have an answer, send it back by running:\n" + command
	if hasTools {
		route = fmt.Sprintf(
			"When you have an answer, send it back with the lich tool `%s` (ticket %s), or by running:\n%s",
			ToolReply, ticketID, command,
		)
	}
	return route + "\n\nThat ticket is the only way back: whoever asked is blocked on it and " +
		"is reading nothing else. Do not answer by messaging a peer session — an answer " +
		"sent any other way is lost."
}

// unreadNotice is what a sender who stopped waiting is told, typed at its own
// prompt the way an answer would be. It has to be actionable on its own: the
// sender cannot see the other screen, and what is usually on it is a question
// only a person can answer.
func unreadNotice(target string) string {
	return fmt.Sprintf(
		"[lich] The %q session never picked up the task you sent it. It was typed at "+
			"that prompt and nothing read it, so something else has that terminal — a "+
			"provider still starting, or a question of its own on screen. Nothing is "+
			"queued and nothing was answered.",
		target,
	)
}

// origin describes the sender in the message's first line. An empty sender is
// the lich CLI run outside any session — a script, a scheduled job, the user's
// own shell — which is a different thing to be told than "another agent".
func origin(sender string) string {
	if sender == "" {
		return "Message relayed by the lich command line"
	}
	return fmt.Sprintf("Message from session %q", sender)
}

// paste wraps text in bracketed paste, which is how a multi-line message
// reaches a TUI prompt as one prompt instead of as one submission per newline.
// Every provider lich spawns runs a TUI that enables bracketed paste; one that
// did not would read the newlines as submissions.
//
// It does not submit. See submitDelay.
func paste(text string) string {
	return "\x1b[200~" + text + "\x1b[201~"
}

// submit is the Enter that sends what paste put at the prompt.
const submit = "\r"

// defaultSubmitDelay is how long the relay waits between the paste and the
// Enter that sends it.
//
// Everything else lich pastes into a prompt is left for the user to send, so
// this is the only place that presses Enter itself — and a carriage return
// riding in the same write as the paste is swallowed. Claude Code collapses a
// multi-line paste into a "[Pasted text #2 +7 lines]" placeholder, and the
// Enter arriving inside that same burst goes into building the placeholder
// rather than sending it: the message sat unsent at the target's prompt, seen
// only when someone opened that session by hand. Nothing here can read the
// target's screen to know when it has settled, so the delay is the instrument,
// and it is generous on purpose — a tenth of a second nobody notices against a
// message that otherwise never arrives.
const defaultSubmitDelay = 150 * time.Millisecond

// sanitize strips the control characters that would either break out of the
// bracketed paste framing or drive the target's terminal, keeping the
// whitespace a prompt legitimately contains. ESC is the one that matters: text
// carrying "\x1b[201~" would end the paste early and leave the rest of itself
// running as keystrokes.
func sanitize(text string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, text)
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

// Package relay lets one lich session hand a prompt to another and get an
// answer back, for the providers whose own CLI has no cross-session channel of
// its own. Claude Code has one; Codex, opencode, oh-my-pi and Crush do not, and
// this works the same for all five.
//
// The delivery is deliberately dumb: lich hands the message to the target's
// mod where one runs (Claude Code with lich-plugin), and otherwise types it at
// the target's prompt and submits it, exactly as the user would. What it never
// does is read the answer off the terminal — a TUI's output is boxes, spinners
// and ANSI, and parsing it would mean a parser per provider, each hostage to a
// release. Instead the message it delivers asks the agent to report back by
// running `lich reply <ticket>`, so the answer is *written by the agent that
// produced it*. That is what keeps this provider-agnostic: anything that can
// run a shell command can answer.
package relay

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
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
	// ticketTTL is how long an unanswered ticket stays waitable after anyone
	// last wanted it — its creation, or a caller that stopped waiting on it —
	// and how long an errand that ended without an answer still takes a late
	// one. A target that never replies would otherwise leak one entry per
	// attempt for the life of the process.
	ticketTTL = time.Hour
	// defaultReceiptWindow is how long a task has to be picked up by the agent it
	// was typed at. A provider that read it reports UserPromptSubmit within a
	// second or two; this is generous against a machine under load, and short
	// enough that the sender learns in one tool call rather than at the ticket's
	// expiry an hour later.
	defaultReceiptWindow = 30 * time.Second
	// defaultDeliveryLimit is how long a queued task waits for its target to
	// reach a free prompt before the errand is reported undelivered (see
	// queueDelivery and awaitFree). A worktree setup script that installs
	// dependencies and warms a build runs minutes on a cold cache, so the number
	// has to be generous; past it the checkout is broken or waiting on a person,
	// and neither ends by itself. It is well inside ticketTTL on purpose: the
	// sender hears a failure it can act on rather than watching a ticket expire
	// an hour later with nothing said.
	defaultDeliveryLimit = 5 * time.Minute
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
	// defaultSettleLimit caps how long a delivery waits for the target to finish
	// taking a paste in before it presses Enter anyway (see awaitSettled). It
	// only has to outlast a drain — the whole message is already in the PTY —
	// and it sits well inside the receipt window, so a target that never goes
	// quiet still gets its Enter in time to report reading it.
	defaultSettleLimit = 5 * time.Second
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
	// ToolCollect drains the results waiting for a sender. Named in prose too:
	// the nudge typed at a sender's prompt is what tells it the tool exists.
	ToolCollect = "wait_for_answer"
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
	// StatusUndelivered means the task never reached a prompt: it was held back
	// for a session that was not at one, and that session died or stayed busy
	// past defaultDeliveryLimit. Nothing is queued anymore and nothing was read.
	StatusUndelivered = "undelivered"
	// StatusStopped means a subagent worker was closed before it answered
	// (SessionClosed). A caller that closed it itself hears it only while
	// holding the line, or when it is itself a worker somebody waits on; a
	// worker anyone else closed has it filed in its caller's inbox.
	StatusStopped = "stopped"
	// StatusExpired means nobody answered within ticketTTL and the ticket was
	// dropped (sweep). Only a sender that is itself a worker somebody waits on
	// hears it, filed in its inbox; any other sender finds the ticket unknown.
	StatusExpired = "expired"
)

// Session states the relay watches, spelled as the hook contract reports them
// (docs/hooks/session-state.md).
const (
	stateBusy    = "busy"
	stateDone    = "done"
	stateIdle    = "idle"
	stateWaiting = "waiting"
	// stateCompacting brackets a compaction without moving the turn; the report
	// closing it restates the session's state (internal/terminal).
	stateCompacting = "compacting"
	// stateInterrupted is not a hook report: lich raises it for a turn the user
	// stopped at the PTY (internal/terminal, noteInterrupt).
	stateInterrupted = "interrupted"
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
// the label at the other end, which way the request runs, and the ticket the
// two ends share. Peer is empty when the other end is not a session at all —
// the CLI run from a script or a shell — which the card words its own way.
//
// The ticket rides along because it is otherwise written down in exactly one
// place: the message typed at the target's prompt. An agent whose context was
// compacted past that message has no way back to it, and neither had the person
// watching — the window knew a request was open and could not say which one.
// Empty when the mark is being cleared, along with the direction.
type RelayEvent struct {
	ID        string `json:"id"`
	Peer      string `json:"peer"`
	Direction string `json:"direction"`
	Ticket    string `json:"ticket"`
}

// Sessions is the persistence the relay reads: every open session, so a label
// can be resolved to the session it names and the caller can be told what it
// may address. The store implements it.
type Sessions interface {
	LoadState() ([]store.Project, error)
	// SetSessionSchedule clears a scheduled prompt once it has been typed at its
	// session (see deliverDue), and parks or drops the continuation of a turn a
	// usage limit ended (resume.go). Every other prompt is parked by the window.
	SetSessionSchedule(sessionID string, at int64, prompt string) error
	// SessionBranch is the branch of the checkout a session runs in, "" when
	// git cannot name one. A subagent's report names it (subagentReport).
	SessionBranch(sessionID string) string
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
	// QuietFor is how long that session's PTY has produced nothing, which is
	// how a delivery knows the target has finished taking the paste in before
	// it presses Enter behind it (see awaitSettled).
	QuietFor(id string) time.Duration
	// AgentName is the name that session's agent answers to in its provider's
	// peer roster, read out of the provider's own record. Empty when there is
	// none, which is what leaves the roster on the derived name.
	AgentName(id string) string
	// HoldInput keeps what the person at that session types out of the
	// submission this delivery is making, and hands it back at the prompt
	// afterwards. The returned release is called once the Enter is through
	// (see typeIn).
	HoldInput(id string) func()
	// SubmitPrompt hands text to the session's Claude Code mod as a prompt of
	// its own, submitted as a notification when note is not nil; ErrNoMod when
	// no mod polls from that session.
	SubmitPrompt(id, text string, note *Notification) (HandedPrompt, error)
	// ModAttached is whether a Claude Code mod polls from that session, which
	// takes a prompt without touching the line its user is typing.
	ModAttached(id string) bool
	// ModAnswers is whether that session's mod answers a subagent errand with
	// its turn's final message (docs/hooks/mod-answer.md).
	ModAnswers(id string) bool
}

// Peer is one session a caller may address: the label it is addressed by, the
// name it answers to in Claude Code's peer roster, the project it belongs to
// (labels are unique within a project, not across them), what is running in it,
// and what that session last reported it was doing.
//
// Both names are published because both reach this session and an agent sees
// them in different places — the label on the card, the roster name in
// `/list-agents` and in what a mention writes at a prompt. Showing one and
// accepting the other is what made an agent treat a single session as two.
//
// State is stateBusy, stateWaiting or stateDone, and empty when the session has
// reported nothing — which is a provider whose plugin does not report state as
// much as a session that has not had a turn yet. Empty is "not known", never
// "idle": guessing the second is how a caller ends up sending work into a
// session that cannot take it.
type Peer struct {
	Label   string `json:"label"`
	Name    string `json:"name"`
	Project string `json:"project"`
	Kind    string `json:"kind"`
	State   string `json:"state"`
	// ID is the session's lich id, the LICH_SESSION_ID its process carries,
	// so a reader holding a recorded id can find its card. Last, because
	// fields are appended to this shape, never inserted.
	ID string `json:"id"`
}

// Result is what a caller gets back from Send or Wait. Answer is empty unless
// Status is StatusAnswered.
type Result struct {
	Ticket string `json:"ticket"`
	Target string `json:"target"`
	Status string `json:"status"`
	Answer string `json:"answer"`
	// Private is set on the outcome of a ticket sent with SendPrivate, which
	// only that ticket collects: no note will announce it.
	Private bool `json:"private,omitempty"`
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
	// delivered is when the message started going into the target's PTY — zero
	// while the ticket is still waiting out the target's setup script. Turn
	// accounting reads it to tell an undelivered message apart from a live one;
	// which of two live ones came first is deliverySeq's question, never this
	// timestamp's.
	delivered time.Time
	// deliverySeq is the order this ticket was handed off in, counting from one.
	// Two errands can share a delivered timestamp — Windows' clock only moves
	// every ~15.6ms, and two messages land inside one tick easily — and "the
	// oldest delivery" then fell to Go's randomised map iteration, closing the
	// wrong errand about one time in ten. A counter is the order itself rather
	// than a reading of it, so no clock's resolution can flatten it.
	deliverySeq uint64
	// asked is the opening of the prompt this errand carried, on one line. It is
	// what names an errand to the agent working it: a session with more than one
	// open request has to be shown which is which before it can pick the ticket
	// its answer belongs to (see errandOfLocked). The whole prompt is not kept —
	// it runs to promptLimit, and what a reader needs is the line they recognise.
	asked string
	// prompt is the task itself, composed into the message when it is handed
	// over (handOff): whether the target's mod answers for it is known only
	// then.
	prompt string
	done   chan struct{}
	answer string

	// attended is how many callers are blocked on this ticket right now. An
	// answer that lands while nobody is waiting has nowhere to be returned, so
	// it is typed at the sender's prompt instead — the same way the request
	// reached the target.
	attended int
	// private is a ticket sent by a caller the sender session's prompt does not
	// speak for: a subagent or workflow step inside it, which lich cannot tell
	// apart from the session itself. Its outcome is held for the ticket alone,
	// never drained by a no-ticket collect, counted or nudged (SendPrivate).
	private bool
	// subagent is a ticket sent with SendSubagent: its worker is the sender's
	// subagent, and its errand lives as long as the worker does.
	subagent bool
	// modAnswers is a subagent errand handed to a worker whose mod answers it
	// (WorkerAnswered). A turn ending without that answer handed work to the
	// background, so it does not end the errand (turnCandidates).
	modAnswers bool
	// blockNoted is whether the sender was already told its subagent waits on a
	// permission in the block running now. A busy or done report ends the block.
	blockNoted bool
	// answered records that the answer is in, for the waiter that gave up in the
	// same instant it arrived: it leaves last and has to notice it was the one
	// holding the ticket.
	answered bool

	// stalled closes when the target finished a turn without replying here. See
	// Observe for how a turn is told apart from the one already running when the
	// message arrived.
	stalled chan struct{}
	// stopped is set before stalled closes when the errand ended because its
	// worker was closed (SessionClosed), which a waiter hears as stopped rather
	// than unanswered.
	stopped bool
	// unread closes when the target never reacted to the task at all. See
	// watchReceipt.
	unread chan struct{}
	// redelivered is whether the task was already typed in a second time, which
	// is what bounds watchReceipt's retry at one.
	redelivered bool
	// submitted is whether the Enter that sends the message went in. delivered
	// is stamped before the paste, for turn accounting; until the Enter the
	// agent has not seen the message, so an answer that names no ticket cannot
	// be about it (errandOfLocked).
	submitted bool
	// waited is when a caller last stopped waiting on this ticket without an
	// outcome. The ticket's hour runs from there as well as from its creation:
	// that caller was just told the errand is still open.
	waited time.Time
	// lapsed is when the errand ended without an answer. Its ticket stays
	// answerable for ticketTTL after that, in Service.lapsed.
	lapsed time.Time
	// lapsedAs is the status the errand ended with: unanswered or unread.
	lapsedAs string
	// why is what an errand that ended unanswered says about it, when its
	// worker's mod named the reason (WorkerUnanswered); empty otherwise.
	why string
	// undelivered closes when the message never got into the target's PTY at
	// all. See queueDelivery.
	undelivered chan struct{}
	// sawBusy is whether the target has been working since this ticket's turn
	// began; a turn that ends without it never started here.
	sawBusy bool
	// skipTurns is how many turn endings belong to work that was already running
	// when the message was delivered. Every provider queues typed input, so a
	// message handed to a busy session is answered a turn later — and the ending
	// of the turn in progress says nothing about this request.
	skipTurns int
	// collected reports whether a poll carried the message to the target's mod;
	// nil for a typed one. Until then nothing in that session has it, so a turn
	// ending there is not its turn (turnCandidates).
	collected func() bool
}

// Service relays prompts between sessions. Tickets live in memory only: one
// exists for as long as its errand does, and a lich that restarted has no PTY
// left to answer into anyway.
type Service struct {
	mu sync.Mutex
	// announceMu orders the inbox announcements, which are counted and emitted
	// outside s.mu. See announceInbox.
	announceMu sync.Mutex
	tickets    map[string]*ticket
	// deliveries counts hand-offs, and stamps each ticket's deliverySeq.
	deliveries uint64
	// state is the last thing each session reported, so a delivery knows whether
	// it is landing in the middle of a turn. Only sessions the relay has heard
	// about appear; an unknown one is treated as not working, which is what a
	// session with no hooks installed looks like.
	state map[string]string
	// reported is what each session last said out loud, which is what the roster
	// publishes. It is not s.state: that one answers "is a turn running", and
	// there a waiting mid-turn has to keep reading as busy (see Observe). Here
	// waiting has to read as waiting — it is the one state that means a caller
	// must not send work in at all.
	reported map[string]string
	// ready is the inbox: finished errands waiting to be collected, keyed by
	// ticket so a Wait on the original ticket still finds its outcome.
	ready map[string]*inboxEntry
	// held is where the outcomes of private tickets wait, apart from ready, so
	// nothing that reads the inbox for a sender can reach them: only a Wait on
	// their ticket does. A subagent's report waits here too once it was handed
	// to its sender whole (takeNewsLocked).
	held map[string]*inboxEntry
	// lapsed is the errands that ended without an answer — a turn over with no
	// reply, or a task nobody read — kept answerable by their ticket. A worker
	// that hands its work to the background ends its turn before the work is
	// done, and its answer arrives later on the same ticket.
	lapsed map[string]*ticket
	// collectors is who is blocked in Collect right now, per sender. A result
	// stashed while one is registered wakes it instead of arming a nudge.
	collectors map[string][]chan struct{}
	// nudgeTimer is the armed debounce per sender, so a burst of results costs
	// one nudge rather than one per result.
	nudgeTimer map[string]*time.Timer
	// nudging serializes flushNudge per sender: it marks an entry nudged before
	// attempting delivery and only unmarks it after a failed attempt, so a second
	// flush racing that window — the debounce timer and Observe's end-of-turn
	// call both reach the same sender — must see the outcome of the first before
	// deciding, or it finds the entry still marked and gives up wrongly silent.
	nudging map[string]*sync.Mutex

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
	// deliveryLimit is how long a queued task waits for a prompt to reach
	// (defaultDeliveryLimit). A field for the same reason.
	deliveryLimit time.Duration
	// nudgeDelay is the debounce before a nudge is typed (defaultNudgeDelay).
	// A field for the same reason.
	nudgeDelay time.Duration
	// settleLimit caps the wait for a target to finish taking a paste in
	// (defaultSettleLimit). A field for the same reason.
	settleLimit time.Duration
	// modAckWait bounds the wait on a mod's ack for a delivery that carries no
	// ticket (defaultModAckWait). A field for the same reason.
	modAckWait time.Duration
	// plugins answers what a provider's sessions can do, which depends on state
	// this package has none of: whether the companion plugin is installed there,
	// and whether it is new enough to carry lich's own operations. Nil — the
	// state a test that does not care is in — reads as "nothing is installed":
	// no delivery is checked, and a relayed message names the command line.
	plugins Plugins
	// reportedWorkers is the sessions that answered a subagent errand in the
	// turn running now; the turn ending finishes them (finishedWorkerLocked).
	reportedWorkers map[string]bool
	// resumes is the sessions holding a continuation lich parked after a usage
	// limit (resume.go), so a turn starting there can drop it.
	resumes map[string]bool
	// workerFinished is told about a worker that finished. Nil leaves every
	// worker running, the state a test that does not care is in.
	workerFinished func(workerID string) error
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
		tickets:         make(map[string]*ticket),
		state:           make(map[string]string),
		reported:        make(map[string]string),
		ready:           make(map[string]*inboxEntry),
		held:            make(map[string]*inboxEntry),
		lapsed:          make(map[string]*ticket),
		reportedWorkers: make(map[string]bool),
		resumes:         make(map[string]bool),
		collectors:      make(map[string][]chan struct{}),
		nudgeTimer:      make(map[string]*time.Timer),
		nudging:         make(map[string]*sync.Mutex),
		sessions:        sessions,
		term:            term,
		events:          events,
		now:             time.Now,
		submitDelay:     defaultSubmitDelay,
		receiptWindow:   defaultReceiptWindow,
		deliveryLimit:   defaultDeliveryLimit,
		nudgeDelay:      defaultNudgeDelay,
		settleLimit:     defaultSettleLimit,
		modAckWait:      defaultModAckWait,
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
// The wait running out is not a failure: the errand is open, and the returned
// ticket is what picks its outcome up later. A target that is not at a prompt
// yet is not a failure either — the task is queued and delivered when it is
// (see queueDelivery). ctx is the caller's: one that hangs up mid-wait hears
// nothing, and the errand's outcome goes to its inbox as if its wait had run out.
func (s *Service) Send(ctx context.Context, fromID, target, project, prompt string, waitSeconds int) (Result, error) {
	return s.send(ctx, fromID, target, project, prompt, waitSeconds, errandShared)
}

// SendPrivate is Send for a caller that runs inside the sender session without
// being its agent: a subagent or a workflow step. Every call from that session
// reaches lich as the session, so this is how the caller keeps its errand to
// itself — the outcome is collected by its ticket alone, and nothing is typed at
// the session's prompt or counted on its card about it.
func (s *Service) SendPrivate(
	ctx context.Context, fromID, target, project, prompt string, waitSeconds int,
) (Result, error) {
	return s.send(ctx, fromID, target, project, prompt, waitSeconds, errandPrivate)
}

// SendSubagent is Send for a worker opened as the sender's subagent (`lich open
// --subagent`), so the errand behaves like the subagent it replaces: the report
// reaches the sender whole where its mod takes it (flushNudge), the errand is not
// dropped by ticketTTL while the worker lives (sweep), and the sender hears once
// when the worker blocks on a permission (Observe). It needs a sending session:
// there is nobody else to report to.
func (s *Service) SendSubagent(
	ctx context.Context, fromID, target, project, prompt string, waitSeconds int,
) (Result, error) {
	if fromID == "" {
		return Result{}, fmt.Errorf("a subagent reports to the session that asked for it, and this call came from no session")
	}
	return s.send(ctx, fromID, target, project, prompt, waitSeconds, errandSubagent)
}

// errandMode is who an errand's outcome is for: the sending session (Send), the
// one caller holding its ticket (SendPrivate), or the sending session as its
// subagent's (SendSubagent).
type errandMode int

const (
	errandShared errandMode = iota
	errandPrivate
	errandSubagent
)

func (s *Service) send(
	ctx context.Context, fromID, target, project, prompt string, waitSeconds int, mode errandMode,
) (Result, error) {
	prompt = sanitize(prompt)
	if strings.TrimSpace(prompt) == "" {
		return Result{}, fmt.Errorf("nothing to send: the prompt is empty")
	}
	if len(prompt) > promptLimit {
		return Result{}, fmt.Errorf("prompt is %d bytes, over the %d limit: "+
			"name the paths to read instead of pasting their contents", len(prompt), promptLimit)
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
		fromID:      fromID,
		sender:      sender,
		targetID:    dest.ID,
		target:      dest.Peer.Label,
		created:     s.now(),
		asked:       askedExcerpt(prompt),
		prompt:      prompt,
		private:     mode == errandPrivate,
		subagent:    mode == errandSubagent,
		done:        make(chan struct{}),
		stalled:     make(chan struct{}),
		unread:      make(chan struct{}),
		undelivered: make(chan struct{}),
	}
	s.mu.Lock()
	expired, senders := s.sweep()
	// The caller attends from the moment the ticket exists: it always goes on
	// to await, and an answer landing before then is its to carry out.
	t.attended = 1
	s.tickets[id] = t
	s.mu.Unlock()
	s.clearAll(expired)
	s.announceInboxAll(senders)

	if s.takesDelivery(dest.ID) {
		// A target that takes the message now is written to on the caller's own
		// goroutine, so a PTY that refuses the write is the error this call
		// returns rather than an outcome mailed to the sender later.
		if err := s.handOff(id, t, dest.Peer.Kind); err != nil {
			s.mu.Lock()
			delete(s.tickets, id)
			s.mu.Unlock()
			return Result{}, err
		}
	} else {
		go s.queueDelivery(id, t, dest)
	}
	// The caller's own wait bounds this call and nothing else: the errand
	// outlives it either way, and blocking past what was asked would run past
	// the HTTP client's own budget (internal/cli, waitBudget) and report a
	// timeout on an errand that is running perfectly well.
	result := s.await(ctx, id, t, waitFor(waitSeconds))
	result.Private = t.private
	return result, nil
}

// handOff composes a ticket's message, hands it to the target's mod, or types
// it where no mod takes it, and starts everything that watches what becomes of
// it. Called either on the caller's goroutine or, for a target that was not at
// a prompt yet, on the one holding the message back; and once more from
// watchReceipt, when the first write reached a terminal nothing was reading.
func (s *Service) handOff(id string, t *ticket, kind string) error {
	message := s.taskMessage(id, t, kind)
	// Stamped before the mod is handed it too: its session's busy report can
	// arrive milliseconds after the prompt is queued.
	busy := s.stampDelivery(t)
	handed, err := s.term.SubmitPrompt(t.targetID, message, nil)
	if errors.Is(err, ErrNoMod) {
		return s.typeTask(id, t, kind, message, busy)
	}
	if err != nil {
		return fmt.Errorf("deliver to %q: %w", t.target, err)
	}
	s.mu.Lock()
	t.collected = handed.Collected
	s.mu.Unlock()
	// Queued for the mod is submitted the way an Enter typed into a busy
	// session is: the agent reads it once its prompt is free.
	s.markSubmitted(id, t)
	go s.watchModReceipt(id, t, kind, message, busy, handed.Await)
	return nil
}

// stampDelivery stamps the ticket as delivered now and reports whether the
// target was busy, which decides the turn this message lands in.
func (s *Service) stampDelivery(t *ticket) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Read here rather than at Send: a queued message can wait out a whole setup
	// script, and what the target was doing back then decides nothing about the
	// turn this message lands in.
	busy := s.state[t.targetID] == stateBusy
	if busy {
		t.skipTurns = 1
	}
	// Stamped before the write, not after: a delivery is two writes a beat apart
	// (see typeIn), and turn accounting ignores an undelivered ticket. A target
	// reporting inside that beat would be lost — its busy report, and the ticket
	// is closed unread with the message queued and the reply that follows landing
	// on nothing; its done, and the skip meant for the turn already running is
	// spent on the turn that carries the answer.
	t.delivered = s.now()
	s.deliveries++
	t.deliverySeq = s.deliveries
	return busy
}

// markSubmitted records that the agent has the message, and raises both cards'
// marks.
func (s *Service) markSubmitted(id string, t *ticket) {
	s.mu.Lock()
	t.submitted = true
	s.mu.Unlock()
	// Announced only once the message is actually in: a mark raised before the
	// write would survive a delivery that never happened.
	s.announce(t.targetID, t.sender, DirectionIn, id)
	s.announce(t.fromID, t.target, DirectionOut, id)
}

// queueDelivery holds a task back until its target is at a prompt, then hands
// it over.
//
// A session opened on a fresh worktree runs the project's setup script in that
// PTY first, and the script routinely outlasts the budget of the call that
// sent the task — that is the main path of a fan-out, where every worker is a
// checkout that has never been installed. Failing there loses the task
// outright and leaves the sender guessing when to try again, so the wait moved
// off the caller: it gets its ticket, and the message goes in when there is
// something there to read it.
//
// A wait that can never end is reported rather than left open. The sender
// hears it the way it hears any other outcome — through the inbox, or on the
// ticket it is still holding — because a promise of news at your prompt has to
// be kept by the failures too.
func (s *Service) queueDelivery(id string, t *ticket, dest candidate) {
	err := s.awaitReady(dest, s.now().Add(s.deliveryLimit))
	if err == nil {
		err = s.handOff(id, t, dest.Peer.Kind)
	}
	if err != nil {
		s.failDelivery(id, t, err)
	}
}

// failDelivery closes an errand whose message never reached a prompt, and tells
// the sender: the one still holding the ticket hears it from its own wait,
// anyone who has moved on finds it in the inbox. The ticket is checked to be
// the live one first — a delivery that failed after the message went in races
// Observe, which may have closed the same errand from the other side.
func (s *Service) failDelivery(id string, t *ticket, cause error) {
	slog.Warn("relay: task never reached a prompt", "target", t.target, "err", cause)
	s.mu.Lock()
	current, live := s.tickets[id]
	if !live || current != t {
		s.mu.Unlock()
		return
	}
	delete(s.tickets, id)
	close(t.undelivered)
	unattended := t.attended == 0
	if unattended {
		s.stashLocked(id, t, StatusUndelivered, "")
	}
	s.mu.Unlock()

	s.clear(t)
	if unattended {
		s.announceInbox(t.fromID)
	}
}

// MaxWaitSeconds is MaxWait in the unit every caller states a wait in. Exported
// because the clamp has to happen before the multiplication that would overflow
// it, which means every surface taking a number of seconds needs the bound
// itself rather than the Duration (internal/cli, waitBudget).
const MaxWaitSeconds = int(MaxWait / time.Second)

// waitFor clamps a caller's requested wait into the supported range.
//
// The seconds are clamped before they become a Duration, not after: a Duration
// is int64 nanoseconds, so a number past about 9.2e9 overflows into a negative
// one — which reads as shorter than MaxWait and hands back a timer that has
// already fired, answering a caller that asked to wait longer by not waiting at
// all.
func waitFor(seconds int) time.Duration {
	if seconds <= 0 {
		return DefaultWait
	}
	if seconds > MaxWaitSeconds {
		return MaxWait
	}
	return time.Duration(seconds) * time.Second
}

// lastActive is the latest moment a ticket was known to be wanted: when it was
// created, or when a caller last stopped waiting on it.
func lastActive(t *ticket) time.Time {
	if t.waited.After(t.created) {
		return t.waited
	}
	return t.created
}

// sweep drops tickets nobody answered in time and returns them, so the caller
// can take their marks down after releasing s.mu. Inbox entries nobody
// collected age out on the same TTL — a sender that never drains would
// otherwise grow the inbox by one entry per errand for the life of the
// process. Called under the lock on the paths that already hold it, which is
// often enough for a map that grows one entry per errand. A ticket a caller is
// blocked on is never dropped: that caller would be told the errand is still
// open on a ticket that is already gone. A sender that is itself a worker
// somebody waits on has its report held by the errand (awaitsOutcomeLocked),
// so the expiry is filed in its inbox, where it resumes it, as SessionClosed
// files a stop.
func (s *Service) sweep() ([]*ticket, []string) {
	var expired []*ticket
	touched := map[string]bool{}
	cutoff := s.now().Add(-ticketTTL)
	for id, t := range s.tickets {
		if t.attended == 0 && lastActive(t).Before(cutoff) && !s.workerHolds(t.subagent, t.targetID) {
			delete(s.tickets, id)
			expired = append(expired, t)
			if !t.private && s.owesSubagentAnswerLocked(t.fromID) {
				s.stashLocked(id, t, StatusExpired, "")
				touched[t.fromID] = true
			}
		}
	}
	for id, e := range s.ready {
		if e.ready.Before(cutoff) && !s.workerHolds(e.subagent, e.targetID) {
			delete(s.ready, id)
			if e.fromID != "" {
				touched[e.fromID] = true
			}
		}
	}
	for id, t := range s.lapsed {
		if t.lapsed.Before(cutoff) && !s.workerHolds(t.subagent, t.targetID) {
			delete(s.lapsed, id)
		}
	}
	// A held outcome expires on the same clock, and silently: nobody was ever
	// told it was there.
	for id, e := range s.held {
		if e.ready.Before(cutoff) {
			delete(s.held, id)
		}
	}
	senders := make([]string, 0, len(touched))
	for fromID := range touched {
		senders = append(senders, fromID)
	}
	return expired, senders
}

// expireTickets runs sweep on the relay's own clock, so an errand ages out
// even while nobody calls in: an outcome filed for it (sweep) has to reach its
// sender without waiting for that sender's next call.
func (s *Service) expireTickets() {
	s.mu.Lock()
	expired, senders := s.sweep()
	s.mu.Unlock()
	s.clearAll(expired)
	s.announceInboxAll(senders)
}

// workerHolds is whether a subagent's worker is still running, which keeps its
// errand off ticketTTL: a worker that runs longer than the hour is the case a
// subagent exists for, and its report has to find its way home. Closing the
// worker ends the errand (SessionClosed), and the TTL runs from there. Called
// under s.mu.
func (s *Service) workerHolds(subagent bool, targetID string) bool {
	return subagent && s.term.Live(targetID)
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

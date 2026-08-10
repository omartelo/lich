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
	"strings"
	"sync"
	"time"

	"github.com/omartelo/lich/internal/providers"
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
	// promptLimit bounds one relayed prompt. The message is typed into a TUI a
	// character at a time; a megabyte of it is a hang, not a prompt.
	promptLimit = 8192
	// answerLimit bounds one reply. Generous — an answer is a summary, and the
	// caller reads it as command output.
	answerLimit = 64 * 1024
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
)

// Sessions is the persistence the relay reads: every open session, so a label
// can be resolved to the session it names and the caller can be told what it
// may address. The store implements it.
type Sessions interface {
	LoadState() ([]store.Project, error)
}

// Terminal is the PTY side: whether a session has a process running right now,
// and how to type at it. The terminal service implements it.
type Terminal interface {
	Live(id string) bool
	Write(id, data string) error
}

// Peer is one session a caller may address: the label it is addressed by, the
// project it belongs to (labels are unique within a project, not across them),
// and what is running in it.
type Peer struct {
	Label   string `json:"label"`
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
type ticket struct {
	target  string
	created time.Time
	done    chan struct{}
	answer  string
}

// Service relays prompts between sessions. Tickets live in memory only: one
// exists for as long as its errand does, and a lich that restarted has no PTY
// left to answer into anyway.
type Service struct {
	mu      sync.Mutex
	tickets map[string]*ticket

	sessions Sessions
	term     Terminal
	now      func() time.Time
}

// New returns a relay reading its roster from sessions and typing through term.
func New(sessions Sessions, term Terminal) *Service {
	return &Service{
		tickets:  make(map[string]*ticket),
		sessions: sessions,
		term:     term,
		now:      time.Now,
	}
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
	t := &ticket{target: dest.Peer.Label, created: s.now(), done: make(chan struct{})}
	s.mu.Lock()
	s.sweep()
	s.tickets[id] = t
	s.mu.Unlock()

	if err := s.term.Write(dest.ID, paste(compose(sender, id, prompt, dest.Peer.Kind))); err != nil {
		s.mu.Lock()
		delete(s.tickets, id)
		s.mu.Unlock()
		return Result{}, fmt.Errorf("deliver to %q: %w", dest.Peer.Label, err)
	}
	return s.await(id, t, waitSeconds), nil
}

// Wait blocks on an already-delivered ticket for another round, so a caller
// whose first wait ran out can come back without sending the message twice.
func (s *Service) Wait(ticketID string, waitSeconds int) (Result, error) {
	s.mu.Lock()
	s.sweep()
	t, ok := s.tickets[ticketID]
	s.mu.Unlock()
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
	defer s.mu.Unlock()
	t, ok := s.tickets[ticketID]
	if !ok {
		return fmt.Errorf("unknown ticket %q — it was answered already, or expired", ticketID)
	}
	select {
	case <-t.done:
		return fmt.Errorf("ticket %q was already answered", ticketID)
	default:
	}
	t.answer = answer
	close(t.done)
	return nil
}

// await blocks on one ticket until it is answered or the wait runs out. An
// answered ticket is dropped here rather than in Reply: the answer has to
// outlive the reply long enough for the waiter to read it, and a caller whose
// wait expired needs the ticket to still be there.
func (s *Service) await(id string, t *ticket, waitSeconds int) Result {
	timer := time.NewTimer(waitFor(waitSeconds))
	defer timer.Stop()

	select {
	case <-t.done:
		s.mu.Lock()
		delete(s.tickets, id)
		s.mu.Unlock()
		return Result{Ticket: id, Target: t.target, Status: StatusAnswered, Answer: t.answer}
	case <-timer.C:
		return Result{Ticket: id, Target: t.target, Status: StatusPending}
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

// sweep drops tickets nobody answered in time. Called under s.mu on the paths
// that already take it, which is often enough for a map that grows one entry
// per errand.
func (s *Service) sweep() {
	cutoff := s.now().Add(-ticketTTL)
	for id, t := range s.tickets {
		if t.created.Before(cutoff) {
			delete(s.tickets, id)
		}
	}
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
			found = append(found, candidate{
				ID:   sess.ID,
				Peer: Peer{Label: sess.Label, Project: p.Name, Kind: sess.Kind},
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

	var matches []candidate
	for _, c := range found {
		if !strings.EqualFold(c.Peer.Label, target) {
			continue
		}
		if project != "" && !strings.EqualFold(c.Peer.Project, project) {
			continue
		}
		matches = append(matches, c)
	}

	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return candidate{}, fmt.Errorf("no live session named %q%s", target, inProject(project))
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
func compose(sender, ticketID, prompt, targetKind string) string {
	return fmt.Sprintf(
		"[lich] %s, not from your own prompt.\n\n%s\n\n%s",
		origin(sender), prompt, replyInstruction(targetKind, ticketID),
	)
}

// replyInstruction tells the receiving agent how to answer. Every agent has a
// shell, so the command is always named; a provider lich registers its MCP
// server with is offered the tool first, because a session that withholds shell
// access would otherwise have no way to answer at all.
func replyInstruction(kind, ticketID string) string {
	command := fmt.Sprintf(
		"  \"$LICH_BIN\" reply %s \"<your answer>\"\nWhoever asked is blocked waiting on it.",
		ticketID,
	)
	if !providers.AcceptsMCPServer(kind) {
		return "When you have an answer, send it back by running:\n" + command
	}
	return fmt.Sprintf(
		"When you have an answer, send it back with the lich tool `%s` (ticket %s), or by running:\n%s",
		ToolReply, ticketID, command,
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

// paste wraps text in bracketed paste and submits it, which is how a multi-line
// message reaches a TUI prompt as one prompt instead of as one submission per
// newline. Every provider lich spawns runs a TUI that enables bracketed paste;
// one that did not would read the newlines as submissions.
func paste(text string) string {
	return "\x1b[200~" + text + "\x1b[201~\r"
}

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

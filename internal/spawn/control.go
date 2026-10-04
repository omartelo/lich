package spawn

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/omartelo/lich/internal/providers"
	"github.com/omartelo/lich/internal/store"
	"github.com/omartelo/lich/internal/terminal"
)

// A few seconds cover an abort, a model or an effort, which the mod applies the
// moment it collects them, and a prompt to an idle session; past that the
// session is busy and the answer is minutes away. A slash command runs only once
// the session is idle and then takes as long as it takes: the longer wait is for
// /compact on an idle session. Both stay under the 90 seconds an MCP call may
// block (cli.mcpMaxWait).
const (
	ackWait        = 10 * time.Second
	commandAckWait = 60 * time.Second
	// ControlWaitLimit is the longest Control waits for a session to confirm.
	ControlWaitLimit = commandAckWait
)

// How far a command Control handed a session got by the time it answered.
const (
	ControlDone      = "done"
	ControlDelivered = terminal.ModDelivered
	ControlEnded     = terminal.ModEnded
)

// Controlled is what a caller is told about a command it gave a session. Value
// is the prompt, model, effort or slash command's name the action carried, and
// CommandID the id the session's mod acks it under.
type Controlled struct {
	ID        string `json:"id"`
	Project   string `json:"project"`
	Label     string `json:"label"`
	Action    string `json:"action"`
	Value     string `json:"value,omitempty"`
	CommandID string `json:"command_id"`
	State     string `json:"state"`
}

// Control gives a running Claude Code session one command through its mod
// (docs/hooks/mod-control.md) and waits a bounded time for the mod to confirm
// it. action is one of the mod's kinds, value what that kind carries, and args
// what follows a slash command's name. A wait that ends after the session took
// the command is not an error: State says it was delivered and still goes
// through, or that the session ended. A command the session never took is
// withdrawn and is the error, and so is an ack that says the command failed,
// each naming the session.
func (s *Service) Control(
	ctx context.Context, fromID, target, projectName, action, value, args string,
) (Controlled, error) {
	cmd, err := modCommandFor(action, value, args)
	if err != nil {
		return Controlled{}, err
	}
	if strings.TrimSpace(target) == "" {
		return Controlled{}, errors.New("name the session to control")
	}
	projects, err := s.sessions.LoadState()
	if err != nil {
		return Controlled{}, fmt.Errorf("read the workspace: %w", err)
	}
	found, err := findSession(projects, s.term.AgentName, target, projectName)
	if err != nil {
		return Controlled{}, err
	}
	if err := controllable(found.session, fromID); err != nil {
		return Controlled{}, err
	}

	label, bound := found.session.Label, ackWaitFor(action)
	wait, cancel := context.WithTimeout(ctx, bound)
	defer cancel()
	out, err := s.term.RunModCommand(wait, found.session.ID, cmd)
	if err != nil {
		return Controlled{}, fmt.Errorf("%q: %w", label, err)
	}
	if out.State == terminal.ModWithdrawn {
		return Controlled{}, fmt.Errorf(
			"%q did not take the %s within %d seconds, so lich withdrew it and nothing ran: its mod stopped "+
				"polling (Claude Code frozen, or the mod reloading). It is safe to send again",
			label, action, int(bound/time.Second))
	}
	if out.State == terminal.ModAcked && !out.OK {
		return Controlled{}, fmt.Errorf("%q refused the %s: %s", label, action, out.Error)
	}
	state := out.State
	if state == terminal.ModAcked {
		state = ControlDone
	}
	return Controlled{
		ID: found.session.ID, Project: found.project.Name, Label: found.session.Label,
		Action: action, Value: value, CommandID: out.ID, State: state,
	}, nil
}

func modCommandFor(action, value, args string) (terminal.ModCommand, error) {
	cmd := terminal.ModCommand{Kind: action}
	switch action {
	case terminal.ModPrompt:
		if strings.TrimSpace(value) == "" {
			return terminal.ModCommand{}, errors.New("a prompt needs its text, and none was given")
		}
		cmd.Text = value
	case terminal.ModAbort:
		if value != "" {
			return terminal.ModCommand{}, errors.New("an abort takes nothing after it")
		}
	case terminal.ModModel:
		cmd.Model = value
	case terminal.ModEffort:
		cmd.Effort = value
	case terminal.ModRunCommand:
		cmd.Name, cmd.Args = value, args
	default:
		return terminal.ModCommand{}, fmt.Errorf(
			"%q is not something lich control does: the actions are prompt, abort, model, effort and command",
			action)
	}
	if args != "" && action != terminal.ModRunCommand {
		return terminal.ModCommand{}, fmt.Errorf("only a command takes arguments, and %s does not", action)
	}
	return cmd, nil
}

func controllable(sess store.Session, fromID string) error {
	if sess.Kind != providers.Claude {
		return fmt.Errorf(
			"%q runs %s, and only a Claude Code session can be controlled: lich drives one "+
				"through a Claude Code mod, which no other CLI has", sess.Label, kindName(sess.Kind))
	}
	// Refused for model and effort too, which would work: nobody asked for
	// them, and allowing them would need a target that defaults to the caller,
	// since list_sessions never shows an agent its own label.
	if sess.ID == fromID {
		return fmt.Errorf(
			"%q is this session, and a session cannot control itself: an abort would end the "+
				"turn asking for it, and a prompt or a slash command would only run once that "+
				"turn is over", sess.Label)
	}
	return nil
}

func ackWaitFor(action string) time.Duration {
	if action == terminal.ModRunCommand {
		return commandAckWait
	}
	return ackWait
}

package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/omartelo/lich/internal/spawn"
)

// controlCall bounds `lich control`: the longest wait spawn.Control holds, plus
// the trip back.
const controlCall = spawn.ControlWaitLimit + callSlack

// control takes the session, the action, and up to two words more: the value
// the action carries, and a slash command's arguments.
func (c *client) control(args []string) error {
	flags := newFlagSet("control")
	project := flags.String(
		"project", "",
		"narrow the target to one project, by name or by directory path, when the label is ambiguous",
	)
	asJSON := flags.Bool("json", false, "print the result as JSON")
	if err := c.parse(flags, args); err != nil {
		return err
	}
	if flags.NArg() < 2 || flags.NArg() > 4 {
		return usageError("control")
	}

	var out spawn.Controlled
	call := []any{c.sessionID(), flags.Arg(0), *project, flags.Arg(1), flags.Arg(2), flags.Arg(3)}
	if err := c.call(context.Background(), "spawn.Control", call, controlCall, &out); err != nil {
		return err
	}
	if *asJSON {
		if err := c.emit(out); err != nil {
			return err
		}
	} else {
		fmt.Fprintln(c.stdout, controlledText(out))
	}
	return controlOutcome(out.State)
}

func controlOutcome(state string) error {
	switch state {
	case spawn.ControlDone:
		return nil
	case spawn.ControlEnded:
		return exitStatus(ExitNoAnswer)
	default:
		return exitStatus(ExitPending)
	}
}

// controlledText words a control outcome for the command line and for an agent
// alike. A delivered command says it goes through, so neither reads it as a
// failure and sends it again.
func controlledText(out spawn.Controlled) string {
	switch out.State {
	case spawn.ControlDone:
		return doneText(out)
	case spawn.ControlEnded:
		return fmt.Sprintf("%q ended before confirming the command (%s, id %s).", out.Label, out.Action, out.CommandID)
	default:
		return fmt.Sprintf("%q took the command (%s, id %s) and has not confirmed it yet: a prompt or "+
			"a slash command runs only once the session is idle. It still goes through.",
			out.Label, out.Action, out.CommandID)
	}
}

func doneText(out spawn.Controlled) string {
	switch {
	case out.Action == "prompt":
		return fmt.Sprintf("%q started a turn on the prompt.", out.Label)
	case out.Action == "abort":
		return fmt.Sprintf("%q stopped its turn.", out.Label)
	case out.Action == "model" && out.Value == "":
		return fmt.Sprintf("%q is back on its own model.", out.Label)
	case out.Action == "model":
		return fmt.Sprintf("%q uses %s from its next request on.", out.Label, out.Value)
	case out.Action == "effort" && out.Value == "":
		return fmt.Sprintf("%q is back on its own effort level.", out.Label)
	case out.Action == "effort":
		return fmt.Sprintf("%q runs at %s effort from its next request on.", out.Label, out.Value)
	default:
		return fmt.Sprintf("%q ran /%s.", out.Label, strings.TrimPrefix(strings.TrimSpace(out.Value), "/"))
	}
}

// controlTools is `lich control` as an MCP tool, appended to mcpTools.
var controlTools = []mcpTool{
	{
		Name: "control_session",
		Description: "Drive another running Claude Code session in lich: type a prompt into it, stop the " +
			"turn it is running, set the model or reasoning effort its next requests use, or run one " +
			"of its slash commands, such as compact or clear. It waits up to 10 seconds (60 for a " +
			"slash command) for the session to confirm. A result saying delivered, with an id, is " +
			"not a failure: the command still goes through. One the session never took fails, and " +
			"nothing ran. Claude Code sessions only, and " +
			"never your own. model and effort change that session only; the slash commands /model " +
			"and /effort are refused because Claude Code would save them as the user's default for " +
			"every new session. " +
			"Experimental: may change outside semver.",
		Schema: schema(map[string]any{
			"session": property("string",
				"The session to drive, by the label on its card, the name it answers to or its lich id."),
			"action": property("string", "One of prompt, abort, model, effort, command."),
			"value": property("string",
				"prompt: the text (required). model: the model name, or omit to go back to the "+
					"session's own. effort: low, medium, high, xhigh or max, or omit likewise. "+
					"command: the slash command's name, with or without the slash (required). "+
					"abort: omit."),
			"args": property("string", "command only: what follows the command's name, as typed."),
			"project": property("string",
				"Project to narrow to, by name or by directory path, when the same label "+
					"exists in more than one."),
		}, "session", "action"),
		Run: func(ctx context.Context, c *client, args mcpArgs) (string, error) {
			var out spawn.Controlled
			call := []any{
				c.sessionID(), args.text("session"), args.text("project"),
				args.text("action"), args.text("value"), args.text("args"),
			}
			if err := c.call(ctx, "spawn.Control", call, controlCall, &out); err != nil {
				return "", err
			}
			return controlledText(out), nil
		},
	},
}

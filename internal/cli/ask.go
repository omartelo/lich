package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/omartelo/lich/internal/spawn"
)

// askCall bounds `lich ask`: the longest wait spawn.Ask holds, plus the trip
// back.
const askCall = spawn.AskWaitLimit + callSlack

// ask takes the session and the question, whose words need no quoting.
func (c *client) ask(args []string) error {
	flags := newFlagSet("ask")
	project := flags.String(
		"project", "",
		"narrow the target to one project, by name or by directory path, when the label is ambiguous",
	)
	asJSON := flags.Bool("json", false, "print the result as JSON")
	if err := c.parse(flags, args); err != nil {
		return err
	}
	if flags.NArg() < 2 {
		return usageError("ask")
	}

	var out spawn.Answered
	question := strings.Join(flags.Args()[1:], " ")
	call := []any{c.sessionID(), flags.Arg(0), *project, question}
	if err := c.call(context.Background(), "spawn.Ask", call, askCall, &out); err != nil {
		return err
	}
	if *asJSON {
		return c.emit(out)
	}
	fmt.Fprintln(c.stdout, out.Answer)
	return nil
}

// askTools is `lich ask` as an MCP tool, appended to mcpTools.
var askTools = []mcpTool{
	{
		Name: "ask_session",
		Description: "Ask another running Claude Code session in lich a side question and get its " +
			"answer, without interrupting it: it answers from its own conversation while its turn " +
			"goes on, and neither the question nor the answer enters that conversation. It sees " +
			"the conversation up to its last request to the model, not the step it is taking right " +
			"now, and cannot use tools to find out more. Waits up to 90 seconds; ask for a brief " +
			"answer. Claude Code sessions only, and never your own.",
		Schema: schema(map[string]any{
			"session": property("string",
				"The session to ask, by the label on its card or the name it answers to."),
			"question": property("string", "The question."),
			"project": property("string",
				"Project to narrow to, by name or by directory path, when the same label "+
					"exists in more than one."),
		}, "session", "question"),
		Run: func(ctx context.Context, c *client, args mcpArgs) (string, error) {
			var out spawn.Answered
			call := []any{c.sessionID(), args.text("session"), args.text("project"), args.text("question")}
			if err := c.call(ctx, "spawn.Ask", call, askCall, &out); err != nil {
				return "", err
			}
			return out.Answer, nil
		},
	},
}

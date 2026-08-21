package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"
)

// command is one subcommand as the help describes it. The spelling lives here
// and nowhere else: the general help, the per-command help and the "usage: …"
// line a malformed call is answered with all read this table, so a flag that
// moves is wrong in one place rather than right on one surface and stale on the
// other two.
type command struct {
	name string
	// args is everything after the name. Written on as many lines as it reads
	// well on; whitespace is collapsed wherever it has to be one line.
	args string
	// about is what the command does, in the words the general help lists it
	// with.
	about string
}

var commands = []command{
	{
		name:  "sessions",
		args:  "[--json]",
		about: "List the live sessions that can be reached.",
	},
	{
		name: "send",
		args: "[--project <name>] [--timeout <seconds>] [--json] <session> <prompt>",
		about: "Put <prompt> at <session>'s prompt and wait for its agent to answer.\n" +
			"Prints the answer. If the wait runs out first it prints a ticket to\n" +
			"pick the answer up with.",
	},
	{
		name: "wait",
		args: "[--timeout <seconds>] [--json] [<ticket>]",
		about: "With a ticket, wait again on that errand. Without one, collect every\n" +
			"result that is ready — and when none is, wait for the next.",
	},
	{
		name: "reply",
		args: "<ticket> <answer>",
		about: "Send <answer> back to whoever is waiting on <ticket>. This is what a\n" +
			"relayed message asks you to run when you are done.",
	},
	{
		name: "open",
		args: "[--project <name>] [--kind <provider>] [--worktree <branch>]\n" +
			"            [--base <branch>] [--model <model>] [--prompt <task>] [--json]",
		about: "Open a new session and start it. --worktree creates a git worktree of\n" +
			"that branch name first and roots the session in it. --model runs the\n" +
			"provider on that model, in the provider's own spelling. --prompt hands\n" +
			"the new session that task as soon as its agent is up, so opening a\n" +
			"worker for a task is one command rather than two. Prints the name the\n" +
			"new session is addressed by.",
	},
	{
		name: "close",
		args: "[--project <name>] [--worktree keep|remove] [--force] [--json] <session>",
		about: "Close a session. Closing the last one in a worktree needs --worktree to\n" +
			"say whether the checkout stays; removing a dirty one needs --force.",
	},
	{
		name: "worktrees",
		args: "[--project <name>] [--json]",
		about: "List a project's git worktrees: what is uncommitted in each and which\n" +
			"sessions are open in it.",
	},
	{
		name: "mcp",
		args: "",
		about: "Serve the commands above as MCP tools over stdio. lich registers this\n" +
			"itself for the providers that support it; you only run it by hand to\n" +
			"point another MCP client at lich.",
	},
	{
		name: "rage",
		args: "[--output <path>]",
		about: "Collect a bug report — versions, browser, providers, plugin state and\n" +
			"the logs, with secrets masked — into one .tar.gz to attach to an issue.\n" +
			"Nothing is uploaded, and it works with no lich running.",
	},
	{
		name: "doctor",
		args: "",
		about: "Walk the boot lich walks — config dir, log, the pinned port, the\n" +
			"workspace database, the browser, the providers — and say whether it\n" +
			"would start here. Exits non-zero when something would stop it.",
	},
}

const helpHeader = "lich talks to the sessions open in the running lich window.\n\n"

const helpFooter = `
  lich version
      Print the running build's version.

Every command takes --help for its own flags.

Run inside a lich session these address the sessions beside it. Run anywhere
else on the machine they find the running lich on their own, and what they
relay is attributed to the command line rather than to a session.
`

// usage is the general help, built from the table so it cannot drift from the
// flags the commands actually parse.
var usage = buildUsage()

func buildUsage() string {
	var b strings.Builder
	b.WriteString(helpHeader)
	for _, cmd := range commands {
		b.WriteString("  " + cmd.spelling() + "\n")
		for line := range strings.SplitSeq(cmd.about, "\n") {
			b.WriteString("      " + line + "\n")
		}
		b.WriteString("\n")
	}
	b.WriteString(strings.TrimPrefix(helpFooter, "\n"))
	return b.String()
}

// spelling is the command as the general help lists it, keeping the line breaks
// the table wrote into a long argument list.
func (c command) spelling() string {
	if c.args == "" {
		return "lich " + c.name
	}
	return "lich " + c.name + " " + c.args
}

// line is the command on one line, which is what an error has room for.
func (c command) line() string {
	return strings.Join(strings.Fields(c.spelling()), " ")
}

// lookup finds a command by name. A miss is impossible from the call sites here
// — every one passes a name the dispatch table already matched — so the zero
// value is enough of an answer.
func lookup(name string) command {
	for _, cmd := range commands {
		if cmd.name == name {
			return cmd
		}
	}
	return command{name: name}
}

// usageError is what a call with the wrong arguments is answered with. It is
// the same line the command's own help prints, so the two cannot disagree.
func usageError(name string) error {
	return fmt.Errorf("usage: %s", lookup(name).line())
}

// printHelp writes one command's own help: what it does, how it is spelled, and
// every flag it takes with the description it was declared with.
func printHelp(w io.Writer, flags *flag.FlagSet) {
	cmd := lookup(flags.Name())
	if cmd.about != "" {
		fmt.Fprintf(w, "%s\n\n", cmd.about)
	}
	fmt.Fprintf(w, "usage: %s\n", cmd.line())
	// Not flag.PrintDefaults: it spells every flag with one dash, and lich's
	// documented spelling — the one the plugin's tool calls use — is two.
	flags.VisitAll(func(f *flag.Flag) {
		kind, description := flag.UnquoteUsage(f)
		name := "  --" + f.Name
		if kind != "" {
			name += " " + kind
		}
		fmt.Fprintf(w, "%s\n        %s\n", name, description)
	})
}

// nearest is the command a word that named none was probably meant to be, or
// empty when nothing is close enough to be worth guessing at. Three edits apart
// is a different word, not a typo.
func nearest(word string) string {
	best, bestDistance := "", 4
	for _, cmd := range commands {
		if d := distance(word, cmd.name); d < bestDistance {
			best, bestDistance = cmd.name, d
		}
	}
	return best
}

// distance is the Levenshtein distance between two words, over two rows rather
// than the full matrix — the words here are one short command name each.
func distance(a, b string) int {
	previous := make([]int, len(b)+1)
	current := make([]int, len(b)+1)
	for j := range previous {
		previous[j] = j
	}
	for i := 1; i <= len(a); i++ {
		current[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			current[j] = min(previous[j]+1, current[j-1]+1, previous[j-1]+cost)
		}
		previous, current = current, previous
	}
	return previous[len(b)]
}

// Package cli is the `lich` command line — the surface a provider CLI running
// inside a lich session uses to reach the sessions beside it.
//
// It exists because an agent has a shell and nothing else: no lich window, no
// RPC client, no knowledge of this app. Every command here reads the loopback
// coordinates lich already exports into each PTY (LICH_PORT / LICH_TOKEN /
// LICH_SESSION_ID, see docs/hooks/README.md) and talks to the running lich over
// the same listener the window uses.
//
// It also works from any other shell on the machine — a script, a scheduled
// job, the user's own terminal. With no coordinates in the environment it reads
// them from the running instance's runtime file (internal/singleton), the same
// one install.sh uses to reach a running lich for /restart. Such a caller has
// no session of its own, and the message it relays says so.
//
// One command is not about the other sessions at all: `lich rage` collects a
// bug report from this machine, and answers to the failure where there is no
// window to ask through (internal/rage). It is here because it is part of the
// same command line, and because it must work when nothing else does.
//
// The contract is docs/cli.md; the lich-plugin slash commands are written
// against it, so a flag or an output line that moves here breaks a repo this
// one cannot see.
package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/omartelo/lich/internal/doctor"
	"github.com/omartelo/lich/internal/rage"
	"github.com/omartelo/lich/internal/relay"
	"github.com/omartelo/lich/internal/singleton"
	"github.com/omartelo/lich/internal/spawn"
)

// NotACommand is returned when the arguments name no lich subcommand at all,
// which is how main tells "the user ran a command" apart from "the user opened
// the app" (and from `lich -- <chromium flags>`).
const NotACommand = -1

// callSlack is how much longer than the wait it asked for the client gives the
// request. The relay answers a pending errand on its own deadline; this only
// has to outlast that answer's trip back.
const callSlack = 30 * time.Second

// shortCall bounds the commands that do not wait on another session.
const shortCall = 10 * time.Second

// openCall bounds opening a session. It is not a wait on anyone — it is git:
// a worktree off a remote branch fetches first, and a cold fetch of a large
// repository is not a ten-second operation. It stays under the 90 seconds an
// MCP tool call may block for (see mcpMaxWait), so the same budget serves both
// surfaces.
const openCall = 60 * time.Second

// deliverWait bounds holding the line when a task is handed to a session that
// was opened a moment ago.
//
// It is not a wait for an answer, and no longer a wait for the session to come
// up either: a task sent to a session that is still running its worktree setup
// script is queued and delivered when its agent is there (relay.queueDelivery).
// The worker was created seconds ago and its task is minutes of work, so a
// ticket is the expected outcome and the sender carries on.
//
// The number is what is left of an MCP call's budget, and the command line
// shares it rather than growing a timeout of its own: opening can spend openCall
// (60s) before this starts, and the client detaches anything still running at
// 120s (see mcpMaxWait) — so the delivery gets 20, plus the callSlack the HTTP
// timeout adds for the trip back, and the worst case lands at 110 rather than
// on the line.
const deliverWait = 20

// Run executes one subcommand and returns the process exit code, or
// NotACommand when args name none. env reads the process environment, and
// version is the running build's, which only the bug report needs.
func Run(args []string, version string, env func(string) string, stdout, stderr io.Writer) int {
	return dispatch(args, &client{
		env: env, version: version, stdin: os.Stdin,
		stdout: stdout, stderr: stderr, running: runningLich,
	})
}

// dispatch is Run with the client already built. It is the seam the tests use:
// a client built here can be pointed at a fake instance, where one built by Run
// would find the lich actually running on the machine and deliver into it.
func dispatch(args []string, c *client) int {
	if len(args) == 0 {
		return NotACommand
	}
	switch args[0] {
	case "sessions":
		return c.run(c.sessions, args[1:])
	case "send":
		return c.run(c.send, args[1:])
	case "wait":
		return c.run(c.wait, args[1:])
	case "reply":
		return c.run(c.reply, args[1:])
	case "open":
		return c.run(c.open, args[1:])
	case "close":
		return c.run(c.close, args[1:])
	case "worktrees":
		return c.run(c.worktrees, args[1:])
	case "mcp":
		return c.run(c.serveMCP, args[1:])
	case "rage":
		return c.run(c.rage, args[1:])
	case "doctor":
		// Not through run: this command's failure is the report it just
		// printed, and a second "lich: …" line on stderr would only say it
		// again, worse.
		return c.doctor(args[1:])
	case "help", "--help", "-h":
		// `lich help <command>` is the command's own help, which is the flags
		// and not just the paragraph the list above shows.
		if len(args) > 1 {
			return dispatch([]string{args[1], "--help"}, c)
		}
		fmt.Fprint(c.stdout, usage)
		return 0
	case "version", "--version", "-v":
		fmt.Fprintf(c.stdout, "lich %s\n", c.version)
		return 0
	}
	// A word that names no subcommand is a typo, and opening the whole app for
	// one is an answer nobody reads: the window it puts on screen says nothing
	// about the command that was run. A leading dash is not a typo — that is
	// `lich --` and the Chromium flags behind it, which the app still takes.
	if strings.HasPrefix(args[0], "-") {
		return NotACommand
	}
	fmt.Fprintf(c.stderr, "lich: unknown command %q", args[0])
	if near := nearest(args[0]); near != "" {
		fmt.Fprintf(c.stderr, " — did you mean %q?", near)
	}
	fmt.Fprint(c.stderr, "\nRun `lich help` for the list.\n")
	return 1
}

type client struct {
	env func(string) string
	// version is the running build's, reported by the bug report and by nothing
	// else: every other command talks to a lich that knows its own.
	version string
	// stdin carries the MCP server's incoming messages; nil for every other
	// command, none of which reads anything.
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
	// running finds the lich instance to talk to when the environment carries
	// no coordinates, i.e. when this is not running inside a lich PTY.
	running func() (*singleton.Info, error)
	// bundle writes the bug report archive; nil takes the real collector. A
	// test overrides it because collecting scans PATH, runs the provider CLIs
	// and asks GitHub for the plugin's latest release.
	bundle func(w io.Writer, root string) (rage.Report, error)
	// diagnose walks the boot; nil takes the real one. A test overrides it for
	// the same reason bundle exists — the real walk binds the pinned port and
	// opens the workspace database.
	diagnose func() ([]doctor.Check, error)
}

// errHelpShown ends a subcommand that was asked for its help rather than for
// its work. It travels the error path because that is the only way out of a
// parse, and is the one error run does not report: the help is already printed,
// and asking for it succeeded.
var errHelpShown = errors.New("help printed")

// run reports a subcommand's failure the way a command line does — one line on
// stderr, a non-zero exit — so an agent reading the output is told what went
// wrong instead of being handed an empty answer.
func (c *client) run(fn func([]string) error, args []string) int {
	err := fn(args)
	if err == nil || errors.Is(err, errHelpShown) {
		return 0
	}
	fmt.Fprintf(c.stderr, "lich: %v\n", err)
	return 1
}

// parse reads a subcommand's flags, answering -h/--help with that command's own
// help. Without this the flag package's "flag: help requested" is what a user
// asking how to run something is handed, on stderr, as a failure.
func (c *client) parse(flags *flag.FlagSet, args []string) error {
	err := flags.Parse(args)
	if errors.Is(err, flag.ErrHelp) {
		printHelp(c.stdout, flags)
		return errHelpShown
	}
	return err
}

func (c *client) sessions(args []string) error {
	flags := newFlagSet("sessions")
	asJSON := flags.Bool("json", false, "print the result as JSON")
	if err := c.parse(flags, args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return usageError("sessions")
	}

	var peers []relay.Peer
	if err := c.call("relay.Peers", []any{c.sessionID()}, shortCall, &peers); err != nil {
		return err
	}
	peers = asList(peers)
	if *asJSON {
		return c.emit(peers)
	}
	if len(peers) == 0 {
		fmt.Fprintln(c.stdout, "No other live sessions.")
		return nil
	}
	// New columns are appended, never inserted, so a script reading the first
	// ones keeps working. The roster name has to be here at all: both names
	// reach a session, and a surface that shows only one is what once made an
	// agent treat a single session as two.
	fmt.Fprintln(c.stdout, "session\tproject\tprovider\tname\tstate")
	for _, p := range peers {
		fmt.Fprintf(c.stdout, "%s\t%s\t%s\t%s\t%s\n", p.Label, p.Project, p.Kind, p.Name, sessionState(p.State))
	}
	return nil
}

// sessionState words a peer's state for the table. A session that has reported
// nothing gets a dash rather than a blank cell: the column is never empty by
// accident, and "not reported" is a different thing from a session sitting
// idle — only the providers whose plugin reports state have one at all.
func sessionState(state string) string {
	if state == "" {
		return "-"
	}
	return state
}

func (c *client) send(args []string) error {
	flags := newFlagSet("send")
	project := flags.String("project", "", "narrow the target to one project when the label is ambiguous")
	timeout := flags.Int("timeout", 0, "seconds to wait for an answer before handing back a ticket")
	asJSON := flags.Bool("json", false, "print the result as JSON")
	if err := c.parse(flags, args); err != nil {
		return err
	}
	if flags.NArg() != 2 {
		return usageError("send")
	}

	var result relay.Result
	call := []any{c.sessionID(), flags.Arg(0), *project, flags.Arg(1), *timeout}
	if err := c.call("relay.Send", call, waitBudget(*timeout), &result); err != nil {
		return err
	}
	return c.report(result, *asJSON)
}

func (c *client) wait(args []string) error {
	flags := newFlagSet("wait")
	timeout := flags.Int("timeout", 0, "seconds to wait before handing the ticket back again")
	asJSON := flags.Bool("json", false, "print the result as JSON")
	if err := c.parse(flags, args); err != nil {
		return err
	}
	if flags.NArg() > 1 {
		return usageError("wait")
	}

	if flags.NArg() == 0 {
		var collected relay.Collected
		if err := c.call("relay.Collect", []any{c.sessionID(), *timeout}, waitBudget(*timeout), &collected); err != nil {
			return err
		}
		if *asJSON {
			return c.emit(collected)
		}
		fmt.Fprintln(c.stdout, collectedText(collected))
		return nil
	}

	var result relay.Result
	if err := c.call("relay.Wait", []any{flags.Arg(0), *timeout}, waitBudget(*timeout), &result); err != nil {
		return err
	}
	return c.report(result, *asJSON)
}

func (c *client) reply(args []string) error {
	flags := newFlagSet("reply")
	if err := c.parse(flags, args); err != nil {
		return err
	}
	if flags.NArg() != 2 {
		return usageError("reply")
	}
	if err := c.call("relay.Reply", []any{flags.Arg(0), flags.Arg(1)}, shortCall, nil); err != nil {
		return err
	}
	fmt.Fprintln(c.stdout, "Answer sent.")
	return nil
}

func (c *client) open(args []string) error {
	flags := newFlagSet("open")
	project := flags.String("project", "", "project to open the session in; defaults to the caller's own")
	kind := flags.String("kind", "", "what the session runs; defaults to the caller's own provider")
	worktree := flags.String("worktree", "", "branch name of a new git worktree to root the session in")
	base := flags.String("base", "", "branch the worktree starts from; defaults to the project's current branch")
	model := flags.String("model", "", "model the provider runs, in the provider's own spelling")
	prompt := flags.String("prompt", "", "task to hand the new session as soon as its agent is up")
	asJSON := flags.Bool("json", false, "print the result as JSON")
	if err := c.parse(flags, args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return usageError("open")
	}

	var opened spawn.Session
	call := []any{c.sessionID(), *project, *kind, *worktree, *base, *model}
	if err := c.call("spawn.Open", call, openCall, &opened); err != nil {
		return err
	}
	delivered, failure := c.handOff(opened, *prompt)
	if *asJSON {
		if err := c.emit(opening{Session: opened, Delivery: delivered}); err != nil {
			return err
		}
		return failure
	}
	fmt.Fprint(c.stdout, openedText(opened))
	if failure != nil {
		return failure
	}
	if delivered == nil {
		return nil
	}
	return c.report(*delivered, false)
}

// opening is what `lich open --json` prints. The session's fields stay at the
// top level, where every reader of this command has always found them, and the
// hand-off rides beside them under one key — absent unless --prompt asked for
// one. Two objects, or two lines, would make a script tell the flags apart
// before it could read the output; --json promises one line and one shape.
//
// A hand-off that failed leaves the key absent and exits non-zero: the session
// is real and printing it is the point, but a script that read exit 0 would
// believe the task landed.
type opening struct {
	spawn.Session
	Delivery *relay.Result `json:"delivery,omitempty"`
}

// handOff gives a just-opened session its first task, and nothing at all when
// no task came with it. The failure it returns names the session, because the
// session outlives it: what went wrong is one send, not the open.
func (c *client) handOff(opened spawn.Session, prompt string) (*relay.Result, error) {
	if prompt == "" {
		return nil, nil
	}
	result, err := c.deliver(opened, prompt)
	if err != nil {
		return nil, fmt.Errorf(
			"the session is open, but the task did not reach it: %w — hand it the task with "+
				"`lich send %q '<task>'` once whatever that says is dealt with",
			err, opened.Label,
		)
	}
	return &result, nil
}

// deliver hands a just-opened session its first task, on both surfaces: the
// command line's --prompt and the open_session tool's, which differ in how they
// word the outcome and in nothing else.
func (c *client) deliver(opened spawn.Session, prompt string) (relay.Result, error) {
	var result relay.Result
	call := []any{c.sessionID(), opened.Label, opened.Project, prompt, deliverWait}
	err := c.call("relay.Send", call, waitBudget(deliverWait), &result)
	return result, err
}

// openedText words a new session for whoever asked for one. It names both of
// its names, because the caller's next move is to address it and either one
// reaches it.
//
// What it no longer does is tell the caller to wait. A session on a fresh
// worktree runs the project's setup script before its provider, which can take
// minutes, and no instruction to "give it a moment" survives that. Sending is
// what waits now (relay.awaitReady), so the honest thing to say is that the
// message will be held.
func openedText(opened spawn.Session) string {
	where := fmt.Sprintf("project %q", opened.Project)
	if opened.Path != "" {
		where = fmt.Sprintf("%s, in worktree %s", where, opened.Path)
	}
	return fmt.Sprintf(
		"Opened session %q (%s) in %s.\n"+
			"It answers to %q and to %q. Its agent may still be starting — a fresh "+
			"worktree runs the project's setup script first — so a task you send it "+
			"is held until the agent is up rather than lost.\n",
		opened.Label, opened.Kind, where, opened.Label, opened.Name,
	)
}

// rage writes the bug report bundle to a file and says what it wrote. It talks
// to no lich: the report it collects is most needed exactly when there is none
// running to ask.
func (c *client) rage(args []string) error {
	flags := newFlagSet("rage")
	output := flags.String("output", "", "write the bundle here instead of ./lich-rage-<timestamp>.tar.gz")
	if err := c.parse(flags, args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return usageError("rage")
	}

	path := *output
	if path == "" {
		path = rage.DefaultBase(time.Now()) + ".tar.gz"
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create the bundle: %w", err)
	}

	collect := c.bundle
	if collect == nil {
		collect = c.collectRage
	}
	report, err := collect(file, archiveRoot(path))
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		// A half-written archive is worse than none: it opens, it looks like a
		// report, and it is missing the part that mattered.
		_ = os.Remove(path)
		return err
	}

	fmt.Fprintf(c.stdout, "Wrote %s\n", path)
	fmt.Fprintf(c.stdout, "lich %s on %s, %s.\n", report.Version, report.Platform, report.Instance)
	fmt.Fprintln(c.stdout, "Read it before you attach it: it carries absolute paths, project and branch names.")
	return nil
}

// doctor prints the boot report and returns the process exit code directly: a
// launch-stopping check is a non-zero exit, which is what a script reads.
func (c *client) doctor(args []string) int {
	flags := newFlagSet("doctor")
	if err := c.parse(flags, args); err != nil {
		if errors.Is(err, errHelpShown) {
			return 0
		}
		fmt.Fprintf(c.stderr, "lich: %v\n", err)
		return 1
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(c.stderr, "lich: %v\n", usageError("doctor"))
		return 1
	}

	run := c.diagnose
	if run == nil {
		run = c.runDoctor
	}
	checks, err := run()
	if err != nil {
		fmt.Fprintf(c.stderr, "lich: %v\n", err)
		return 1
	}
	doctor.Render(c.stdout, c.version, checks)
	if doctor.Failed(checks) {
		return 1
	}
	return 0
}

// runDoctor is the real boot walk, reached when no seam replaced it.
func (c *client) runDoctor() ([]doctor.Check, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("resolve config directory: %w", err)
	}
	return doctor.New(dir, c.env).Run(), nil
}

// collectRage is the real collector, reached when no seam replaced it.
func (c *client) collectRage(w io.Writer, root string) (rage.Report, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return rage.Report{}, fmt.Errorf("resolve config directory: %w", err)
	}
	return rage.New(dir, c.version, os.Environ()).Bundle(w, root)
}

// archiveRoot is the directory the bundle's entries sit under: the archive's
// own name, so two extracted side by side stay apart.
func archiveRoot(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, ".gz")
	return strings.TrimSuffix(base, ".tar")
}

func (c *client) close(args []string) error {
	flags := newFlagSet("close")
	project := flags.String("project", "", "narrow the target to one project when the label is ambiguous")
	worktree := flags.String("worktree", "",
		"what to do with the checkout when this is its last session: keep or remove")
	force := flags.Bool("force", false, "remove a checkout that still has uncommitted work")
	asJSON := flags.Bool("json", false, "print the result as JSON")
	if err := c.parse(flags, args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return usageError("close")
	}

	var closed spawn.Closed
	call := []any{c.sessionID(), flags.Arg(0), *project, *worktree, *force}
	if err := c.call("spawn.Close", call, openCall, &closed); err != nil {
		return err
	}
	if *asJSON {
		return c.emit(closed)
	}
	fmt.Fprint(c.stdout, closedText(closed))
	return nil
}

// closedText says what is gone and what is not. A checkout that was kept is the
// one outcome with something left to come back to, so it says how.
func closedText(closed spawn.Closed) string {
	switch {
	case closed.Removed:
		return fmt.Sprintf("Closed %q and removed its worktree %s.\n", closed.Label, closed.Worktree)
	case closed.Kept:
		return fmt.Sprintf(
			"Closed %q. Its worktree %s is still there, and the session is parked: opening a "+
				"session on that branch again picks its conversation back up.\n",
			closed.Label, closed.Worktree,
		)
	default:
		return fmt.Sprintf("Closed %q.\n", closed.Label)
	}
}

func (c *client) worktrees(args []string) error {
	flags := newFlagSet("worktrees")
	project := flags.String("project", "", "project to list; defaults to the caller's own")
	asJSON := flags.Bool("json", false, "print the result as JSON")
	if err := c.parse(flags, args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return usageError("worktrees")
	}

	var checkouts []spawn.Checkout
	if err := c.call("spawn.Worktrees", []any{c.sessionID(), *project}, shortCall, &checkouts); err != nil {
		return err
	}
	checkouts = asList(checkouts)
	if *asJSON {
		return c.emit(checkouts)
	}
	if len(checkouts) == 0 {
		fmt.Fprintln(c.stdout, "No worktrees.")
		return nil
	}
	fmt.Fprintln(c.stdout, "worktree\tstate\tsessions")
	for _, wt := range checkouts {
		state := "clean"
		if wt.Dirty {
			state = "uncommitted"
		}
		sessions := "-"
		if len(wt.Sessions) > 0 {
			sessions = strings.Join(wt.Sessions, ", ")
		}
		fmt.Fprintf(c.stdout, "%s\t%s\t%s\n", wt.Name, state, sessions)
	}
	return nil
}

// report prints a Send or Wait outcome. An answer goes to stdout on its own so
// it can be read as the command's output; a wait that ran out says what to run
// next, because an agent handed a bare ticket would have to guess.
func (c *client) report(result relay.Result, asJSON bool) error {
	if asJSON {
		return c.emit(result)
	}
	if result.Status == relay.StatusAnswered {
		fmt.Fprintln(c.stdout, result.Answer)
		return nil
	}
	if result.Status == relay.StatusUnanswered {
		fmt.Fprintln(c.stdout, unansweredText(result.Target))
		return nil
	}
	if result.Status == relay.StatusUnread {
		fmt.Fprintln(c.stdout, unreadText(result.Target))
		return nil
	}
	if result.Status == relay.StatusUndelivered {
		fmt.Fprintln(c.stdout, undeliveredText(result.Target))
		return nil
	}
	fmt.Fprintf(c.stdout,
		"%s is still working. The errand is open — a message that session was not ready "+
			"for is held until it is — and a note will be typed at the sending session's "+
			"prompt when its result is ready. To hold the line for it instead:\n"+
			"  lich wait %s\n",
		result.Target, result.Ticket,
	)
	return nil
}

// asList is a decoded list as a caller should be handed it. Decoding a JSON
// null leaves the slice nil, and neither a script reading --json nor an agent
// reading a tool result should have to tell that apart from an empty one.
func asList[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}

// emit writes one JSON line — the shape a script reads instead of the prose
// above, which is what makes these commands usable from an automation.
func (c *client) emit(payload any) error {
	if err := json.NewEncoder(c.stdout).Encode(payload); err != nil {
		return fmt.Errorf("encode the result: %w", err)
	}
	return nil
}

// call posts one RPC to the lich this session belongs to.
func (c *client) call(method string, args []any, timeout time.Duration, out any) error {
	port, token, err := c.coordinates()
	if err != nil {
		return err
	}
	body, err := json.Marshal(args)
	if err != nil {
		return fmt.Errorf("encode arguments: %w", err)
	}

	httpClient := &http.Client{Timeout: timeout}
	status, payload, err := post(httpClient, endpoint(port, token, method), body)
	if err != nil {
		return err
	}
	// A refused token is not this caller's mistake — the coordinates it was given
	// can have gone stale under it (see reissued). Retried once, on the same port,
	// with the token the running instance recorded.
	if status == http.StatusForbidden || status == http.StatusUnauthorized {
		token, err = c.reissued(port, token)
		if err != nil {
			return err
		}
		if status, payload, err = post(httpClient, endpoint(port, token, method), body); err != nil {
			return err
		}
	}
	if status != http.StatusOK {
		return fmt.Errorf("%s", failureOf(payload, status))
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(payload, out); err != nil {
		return fmt.Errorf("decode the reply: %w", err)
	}
	return nil
}

// post sends one RPC and reads the whole reply. The body is read here rather
// than returned open because a refused call is sent a second time, and the
// first response has to be finished with before the second one starts.
func post(httpClient *http.Client, endpoint string, body []byte) (int, []byte, error) {
	resp, err := httpClient.Post(endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return 0, nil, fmt.Errorf("reach lich: %w", err)
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, fmt.Errorf("read the reply: %w", err)
	}
	return resp.StatusCode, payload, nil
}

// endpoint builds the RPC URL for one call.
func endpoint(port, token, method string) string {
	return fmt.Sprintf(
		"http://127.0.0.1:%s/rpc/%s?token=%s",
		url.PathEscape(port), method, url.QueryEscape(token),
	)
}

// reissued answers a listener that refused this call's token, with the token the
// lich on that same port recorded — or with why there is none to try.
//
// The connect token is minted per launch and lives only in memory, so a lich
// closed and opened again answers on the pinned port with a token no older
// process can know. The coordinates in a PTY's environment are that older
// process's, and a caller that outlives the session it was started in keeps
// them: a background agent the harness parks in its own daemon, a nohup, a
// detached pane. Every call it makes 403s from then on, forever, with nothing
// on screen saying why — measured on a Claude Code background agent whose
// `lich mcp` outlived a restart of the lich that spawned it.
//
// Only the same port is retried. The runtime file names one instance, and a
// machine running a daily driver beside a `task dev` build has two: answering to
// the other one would put this message in a window the caller cannot see, which
// is the rule coordinates() follows too.
func (c *client) reissued(port, refused string) (string, error) {
	stale := fmt.Sprintf(
		"lich refused this token on port %s: the coordinates in this environment "+
			"were exported by an earlier lich, and the one running now cannot be "+
			"reached with them", port,
	)
	if c.running == nil {
		return "", fmt.Errorf("%s. Run this from a session of the running lich", stale)
	}
	info, err := c.running()
	if err != nil {
		return "", fmt.Errorf("%s, and the running instance could not be read: %w", stale, err)
	}
	if info == nil || info.Token == "" {
		return "", fmt.Errorf("%s, and no running instance is recorded to ask instead", stale)
	}
	if strconv.Itoa(info.Port) != port {
		return "", fmt.Errorf(
			"%s. The lich recorded on this machine listens on port %d, and a message "+
				"sent there would land in a window this caller cannot see — start it "+
				"again from a session of that lich",
			stale, info.Port,
		)
	}
	if info.Token == refused {
		return "", fmt.Errorf(
			"lich refused this token on port %s, and it is the token the running "+
				"instance recorded — so something other than lich is answering there",
			port,
		)
	}
	return info.Token, nil
}

// coordinates finds the lich to talk to: the one that spawned this PTY when
// there is one, otherwise the instance running on this machine.
//
// The environment comes first on purpose. A session belongs to the lich that
// spawned it, and on a machine running a daily driver beside a `task dev`
// build, the runtime file names only one of them — answering to the wrong lich
// would put a message in a window the caller cannot see.
func (c *client) coordinates() (string, string, error) {
	if port, token := c.env("LICH_PORT"), c.env("LICH_TOKEN"); port != "" && token != "" {
		return port, token, nil
	}
	if c.running == nil {
		return "", "", fmt.Errorf("no lich is running")
	}
	info, err := c.running()
	if err != nil {
		return "", "", err
	}
	if info == nil || info.Port == 0 || info.Token == "" {
		return "", "", fmt.Errorf("no lich is running — open lich, or run this inside one of its sessions")
	}
	return strconv.Itoa(info.Port), info.Token, nil
}

// runningLich reads the loopback coordinates of the lich running on this
// machine. It is the same runtime file install.sh reads to reach a running lich
// from outside a session, and LICH_DEV selects the dev instance's own file just
// as it does everywhere else.
func runningLich() (*singleton.Info, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("resolve config directory: %w", err)
	}
	return singleton.Read(dir)
}

// sessionID is which card this command runs in, empty for a caller that has no
// session — a script, a scheduled job, a plain shell. The relay words the
// message it delivers differently for those, so an empty sender is a fact to
// pass on rather than an error.
func (c *client) sessionID() string {
	return c.env("LICH_SESSION_ID")
}

// failureOf unwraps the RPC's {"error": "..."} envelope, falling back to the
// HTTP status for a body that is not one.
func failureOf(payload []byte, status int) string {
	var envelope struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(payload, &envelope); err == nil && envelope.Error != "" {
		return envelope.Error
	}
	return fmt.Sprintf("%d %s", status, http.StatusText(status))
}

// waitBudget is how long the client gives a call that blocks on another
// session: the wait it asked for, plus room for the answer's trip back. A zero
// timeout means the relay's own default.
func waitBudget(seconds int) time.Duration {
	if seconds <= 0 {
		return relay.DefaultWait + callSlack
	}
	return time.Duration(seconds)*time.Second + callSlack
}

// newFlagSet builds a subcommand's flags. Its own reporting is silenced: a
// parse error comes back as an error and is printed once, by run, on the
// stderr the caller passed rather than on the process's.
func newFlagSet(name string) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.Usage = func() {}
	return flags
}

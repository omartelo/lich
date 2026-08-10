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

	"github.com/omartelo/lich/internal/rage"
	"github.com/omartelo/lich/internal/relay"
	"github.com/omartelo/lich/internal/singleton"
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

const usage = `lich talks to the sessions open in the running lich window.

  lich sessions [--json]
      List the live sessions that can be reached.

  lich send [--project <name>] [--timeout <seconds>] [--json] <session> <prompt>
      Put <prompt> at <session>'s prompt and wait for its agent to answer.
      Prints the answer. If the wait runs out first it prints a ticket to
      pick the answer up with.

  lich wait [--timeout <seconds>] [--json] <ticket>
      Wait again on a ticket a previous send handed back.

  lich reply <ticket> <answer>
      Send <answer> back to whoever is waiting on <ticket>. This is what a
      relayed message asks you to run when you are done.

  lich mcp
      Serve the commands above as MCP tools over stdio. lich registers this
      itself for the providers that support it; you only run it by hand to
      point another MCP client at lich.

  lich rage [--output <path>]
      Collect a bug report — versions, browser, providers, plugin state and
      the logs, with secrets masked — into one .tar.gz to attach to an issue.
      Nothing is uploaded, and it works with no lich running.

Run inside a lich session these address the sessions beside it. Run anywhere
else on the machine they find the running lich on their own, and what they
relay is attributed to the command line rather than to a session.
`

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
	case "mcp":
		return c.run(c.serveMCP, args[1:])
	case "rage":
		return c.run(c.rage, args[1:])
	case "help", "--help", "-h":
		fmt.Fprint(c.stdout, usage)
		return 0
	}
	return NotACommand
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
}

// run reports a subcommand's failure the way a command line does — one line on
// stderr, a non-zero exit — so an agent reading the output is told what went
// wrong instead of being handed an empty answer.
func (c *client) run(fn func([]string) error, args []string) int {
	if err := fn(args); err != nil {
		fmt.Fprintf(c.stderr, "lich: %v\n", err)
		return 1
	}
	return 0
}

func (c *client) sessions(args []string) error {
	flags := newFlagSet("sessions")
	asJSON := flags.Bool("json", false, "print the result as JSON")
	if err := flags.Parse(args); err != nil {
		return err
	}

	var peers []relay.Peer
	if err := c.call("relay.Peers", []any{c.sessionID()}, shortCall, &peers); err != nil {
		return err
	}
	if peers == nil {
		// Decoding a JSON null leaves the slice nil, and a script reading --json
		// should never have to tell that apart from an empty roster.
		peers = []relay.Peer{}
	}
	if *asJSON {
		return c.emit(peers)
	}
	if len(peers) == 0 {
		fmt.Fprintln(c.stdout, "No other live sessions.")
		return nil
	}
	fmt.Fprintln(c.stdout, "session\tproject\tprovider")
	for _, p := range peers {
		fmt.Fprintf(c.stdout, "%s\t%s\t%s\n", p.Label, p.Project, p.Kind)
	}
	return nil
}

func (c *client) send(args []string) error {
	flags := newFlagSet("send")
	project := flags.String("project", "", "narrow the target to one project when the label is ambiguous")
	timeout := flags.Int("timeout", 0, "seconds to wait for an answer before handing back a ticket")
	asJSON := flags.Bool("json", false, "print the result as JSON")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 2 {
		return fmt.Errorf("usage: lich send [--project <name>] [--timeout <seconds>] [--json] <session> <prompt>")
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
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return fmt.Errorf("usage: lich wait [--timeout <seconds>] [--json] <ticket>")
	}

	var result relay.Result
	if err := c.call("relay.Wait", []any{flags.Arg(0), *timeout}, waitBudget(*timeout), &result); err != nil {
		return err
	}
	return c.report(result, *asJSON)
}

func (c *client) reply(args []string) error {
	flags := newFlagSet("reply")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 2 {
		return fmt.Errorf("usage: lich reply <ticket> <answer>")
	}
	if err := c.call("relay.Reply", []any{flags.Arg(0), flags.Arg(1)}, shortCall, nil); err != nil {
		return err
	}
	fmt.Fprintln(c.stdout, "Answer sent.")
	return nil
}

// rage writes the bug report bundle to a file and says what it wrote. It talks
// to no lich: the report it collects is most needed exactly when there is none
// running to ask.
func (c *client) rage(args []string) error {
	flags := newFlagSet("rage")
	output := flags.String("output", "", "write the bundle here instead of ./lich-rage-<timestamp>.tar.gz")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("usage: lich rage [--output <path>]")
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
	fmt.Fprintf(c.stdout,
		"%s is still working. The message was delivered; its answer will be typed at the "+
			"sending session's prompt when it arrives. To hold the line for it instead:\n"+
			"  lich wait %s\n",
		result.Target, result.Ticket,
	)
	return nil
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
	endpoint, err := c.endpoint(method)
	if err != nil {
		return err
	}
	body, err := json.Marshal(args)
	if err != nil {
		return fmt.Errorf("encode arguments: %w", err)
	}

	httpClient := &http.Client{Timeout: timeout}
	resp, err := httpClient.Post(endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("reach lich: %w", err)
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read the reply: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s", failureOf(payload, resp.Status))
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(payload, out); err != nil {
		return fmt.Errorf("decode the reply: %w", err)
	}
	return nil
}

// endpoint builds the RPC URL for one call.
func (c *client) endpoint(method string) (string, error) {
	port, token, err := c.coordinates()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"http://127.0.0.1:%s/rpc/%s?token=%s",
		url.PathEscape(port), method, url.QueryEscape(token),
	), nil
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
func failureOf(payload []byte, status string) string {
	var envelope struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(payload, &envelope); err == nil && envelope.Error != "" {
		return envelope.Error
	}
	return status
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

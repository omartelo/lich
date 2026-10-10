package main

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/ncruces/zenity"
	"github.com/omartelo/lich/internal/agentplugin"
	"github.com/omartelo/lich/internal/appupdate"
	"github.com/omartelo/lich/internal/chromium"
	"github.com/omartelo/lich/internal/cli"
	"github.com/omartelo/lich/internal/drop"
	"github.com/omartelo/lich/internal/events"
	"github.com/omartelo/lich/internal/fonts"
	"github.com/omartelo/lich/internal/logging"
	"github.com/omartelo/lich/internal/patchnotes"
	"github.com/omartelo/lich/internal/project"
	"github.com/omartelo/lich/internal/providers"
	"github.com/omartelo/lich/internal/quota"
	"github.com/omartelo/lich/internal/relay"
	"github.com/omartelo/lich/internal/restart"
	"github.com/omartelo/lich/internal/rpc"
	"github.com/omartelo/lich/internal/singleton"
	"github.com/omartelo/lich/internal/spawn"
	"github.com/omartelo/lich/internal/store"
	"github.com/omartelo/lich/internal/system"
	"github.com/omartelo/lich/internal/terminal"
	"github.com/omartelo/lich/internal/themes"
	"github.com/omartelo/lich/internal/tray"
)

// The frontend is embedded into the binary and served over the loopback
// listener to the Chromium --app window (docs/chromium-shell.md).

//go:embed all:frontend/dist
var assets embed.FS

// changelog is embedded so the app can show a "what's new" popup after an
// update, parsed for the running version's section (internal/patchnotes).
//
//go:embed CHANGELOG.md
var changelog string

// The tray icon, in the encoding each platform's tray takes (internal/tray).
var (
	//go:embed build/appicon-256.png
	trayPNG []byte
	//go:embed build/windows/lich.ico
	trayICO []byte
)

// version is the running build's version, injected at build time via
// -ldflags "-X main.version=<git tag>" (see Taskfile.yml). Unset in dev builds
// ("dev"), which the update check treats as "not a release".
var version = "dev"

func main() {
	// `lich <subcommand>` is the CLI a session's agent calls to reach the
	// sessions beside it (internal/cli). It answers before anything else here:
	// it must not open the database, take the log file or race the singleton
	// bind of the lich it is talking to. Anything that is not a subcommand —
	// including `lich -- <chromium flags>` — falls through and opens the app.
	if code := cli.Run(os.Args[1:], version, os.Getenv, os.Stdout, os.Stderr); code != cli.NotACommand {
		os.Exit(code)
	}

	// Everything after `--` is the window's.
	pinnedShell, chromiumArgs := chromium.ParseFlags(os.Args[1:])
	pinShellFlag(pinnedShell)

	configDir, err := os.UserConfigDir()
	if err != nil {
		slog.Error("resolve config dir", "err", err)
		os.Exit(1)
	}
	// File logging before anything that can fail: every startup failure must be
	// readable after the fact — on Windows the console may not exist at all, and
	// a GUI launch (Finder, .desktop) has no stderr on any of them. The login
	// shell resolution below is why the order matters: it is the one failure
	// that leaves lich running on the launcher's bare PATH, and a warning about
	// it written before this line goes nowhere anybody can read.
	logDir := filepath.Join(configDir, "lich")
	logPath := logging.Path(logDir)
	if closer, err := logging.Init(logDir); err != nil {
		slog.Warn("file log unavailable, stderr only", "err", err)
		// Nothing to reveal or attach to a bug report; the Help section says so
		// rather than pointing at a file that was never written.
		logPath = ""
	} else {
		defer closer.Close()
	}

	launchEnv, env := resolveEnv()

	db, err := store.New()
	if err != nil {
		slog.Error("open store", "err", err)
		// Before the listener and the window, so the dialog is the only surface:
		// a launcher start would otherwise end in silence (handleBindFailure).
		_ = zenity.Error(fmt.Sprintf("lich %s could not open its workspace database.\n\n%v\n\nLog: %s",
			version, err, logPath), zenity.Title("lich"))
		os.Exit(1)
	}
	defer db.Close()

	// App events ride the /events socket; no client connected means no
	// listener yet (the window is still starting) and the event is dropped.
	hub := events.New()
	term := terminal.New(db, env, hub)
	// After db's own defer, so it runs before the database closes: what every
	// session was worked in this run is still in memory until something writes
	// it (internal/terminal.handsOn).
	defer term.FlushHandsOn()
	coord := newCoordinator(chromiumArgs)
	window := newWindow(term.Transport(), configDir, chromiumArgs, term.LiveCount, coord.Quit)
	registerServices(db, term, hub, configDir, logPath, env, launchEnv, window, coord)

	term.SetRestart(coord.Do)

	runChromium(term, configDir, coord, window)
}

// pinShellFlag carries --shell in the environment rather than passing it down:
// the restart successor inherits it (it is re-executed with the window's
// switches alone, chromium.RelaunchArgs), and
// `lich doctor` then reports the window a launch here would really open.
func pinShellFlag(pinnedShell string) {
	if pinnedShell == "" {
		return
	}
	if err := os.Setenv(chromium.OverrideEnv, pinnedShell); err != nil {
		slog.Error("set "+chromium.OverrideEnv, "err", err)
		os.Exit(1)
	}
}

// resolveEnv returns the environment lich was launched in and the one its
// children get, and pins the listener port into both.
//
// The launch environment is snapshotted before any tweak, minus the restart
// marker that is this process's alone (restart.WithoutMarker), because spawned
// terminal sessions must inherit what the user launched lich with (see
// terminal.childEnv) and because a re-check resolves from it again, exactly as
// this does (providers.Service.RefreshPath). ResolveShellEnv recovers the
// rc-exported vars a GUI launch misses (see its doc); its slice is what
// children inherit, and exec.LookPath reads the process PATH instead, so the
// resolved one has to land there too (see PinPath).
//
// The port is pinned rather than only resolved so the restart successor and
// every spawned session agree on the origin the page's localStorage is keyed by
// (singleton.DefaultPort).
func resolveEnv() (launchEnv, env []string) {
	launchEnv = restart.WithoutMarker(os.Environ())
	env = terminal.ResolveShellEnv(launchEnv)
	terminal.PinPath(env)
	if os.Getenv("LICH_LISTEN_PORT") == "" {
		if err := os.Setenv("LICH_LISTEN_PORT", strconv.Itoa(singleton.DefaultPort)); err != nil {
			slog.Error("set LICH_LISTEN_PORT", "err", err)
			os.Exit(1)
		}
	}
	return launchEnv, env
}

// registerServices wires every service the window reaches over the loopback RPC,
// in the order main built them: a service that hands another one a callback has
// to exist first. It is lifted out of main only for its length; nothing here
// runs anywhere else.
func registerServices(db *store.Service, term *terminal.Service, hub *events.Hub,
	configDir, logPath string, env, launchEnv []string, window *restart.Window, coord *restart.Coordinator) {
	proj := project.New(project.ZenityPicker{})
	// gh has one active account per host; a project can name a different one.
	proj.SetAccounts(db.GHAccountForPath)
	// Relocating a project is the one flow that can point two rows at the same
	// directory, so it validates the picked one against the workspace.
	proj.SetProjects(db.ProjectAt)
	// Parking a session records the branch its checkout was on, so the history
	// search can match a branch the row is showing (store.SetBranchOf), and a
	// copy of what was said in it, capped, so the same search reaches the
	// conversation and not only the names around it.
	db.SetBranchOf(proj.Branch)
	db.SetTranscriptOf(terminal.TranscriptText)
	// The history offers conversations started outside lich in a project's
	// checkouts (store.ExternalSessions): the providers' stores list them, git
	// says which directories are the project's.
	db.SetConversationsOf(terminal.Conversations)
	db.SetCheckoutsOf(func(path string) ([]string, error) {
		// Asked for every project on each listing: one that is not a repository,
		// or whose directory is gone, would log a git failure every time.
		if _, err := os.Stat(filepath.Join(path, ".git")); err != nil {
			return nil, err
		}
		checkouts, err := proj.ListCheckouts(path)
		if err != nil {
			return nil, err
		}
		paths := make([]string, len(checkouts))
		for i, c := range checkouts {
			paths[i] = c.Path
		}
		return paths, nil
	})

	// Every service the frontend uses goes through the loopback RPC
	// (internal/rpc). store.Close manages the DB lifecycle and stays Go-only.
	dispatcher := rpc.New()
	// db.SessionExists is what keeps the age rule off a live session's copies:
	// they are deleted by the session's row going away, not by the clock.
	drops := drop.New(configDir, db.SessionExists)
	// The other prune runs after each new copy; this one is what clears the
	// last of them for a lich that is never dropped on again — and the only one
	// that may remove a session's directory outright, no session having spawned
	// around it yet.
	drops.PruneStale()
	// A copy belongs to the session it was dropped into: that session reads it
	// through a bind mount when it is confined, and deleting the session's row
	// takes the copies with it.
	term.SetDropDir(drop.Dir(configDir))
	db.SetSessionGone(drops.Purge)
	// The footer's attach button opens the picker through the drop service, so
	// the file it copies for a confined session is one a human chose in a dialog
	// (drop.Attach says why that matters).
	drops.SetPicker(proj.PickFile)
	dispatcher.Register("terminal", term)
	dispatcher.Register("drop", drops)
	dispatcher.Register("fonts", fonts.New())
	dispatcher.Register("project", proj)
	plugins := agentplugin.New(db)
	dispatcher.Register("agentplugin", plugins)
	go plugins.RepairRegistrations()
	dispatcher.Register("appupdate", appupdate.New(version, coord.Install, hub.Emit))
	dispatcher.Register("patchnotes", patchnotes.New(version, changelog))
	dispatcher.Register("store", db)
	// Read before runChromium writes this run's runtime file over the previous
	// run's: a file still there is the only trace a bad exit leaves, and the
	// restored workspace looks exactly like one closed on purpose.
	uncleanExit := singleton.UncleanExit(configDir, os.Getenv(restart.WaitEnv))
	sys := system.New(env, logPath, version, uncleanExit)
	// A second launch of lich asks for the window here, and `lich quit` is the
	// one way to end a lich whose window is closed.
	sys.SetShowWindow(window.Show)
	sys.SetQuit(coord.Quit)
	sys.SetCloseWindow(window.Dismiss)
	dispatcher.Register("system", sys)
	// Deleting a session for good takes any prompt parked on it, and the row is
	// the only copy of what the user wrote. Both channels are used, and neither
	// is the other's fallback: the toast is what the user reading the card sees,
	// and the desktop notification is what reaches a forfeit nobody was at the
	// window for — an agent closing a session while the page is reloading, where
	// an event with no client connected is dropped.
	db.SetScheduleForfeited(func(lost store.ForfeitedSchedule) {
		hub.Emit(store.ScheduleForfeitEventName, lost)
		summary, detail := lost.Notice()
		if err := sys.Notify(summary, detail); err != nil {
			slog.Warn("notify forfeited schedule", "session", lost.Label, "err", err)
		}
	})
	providerSvc := providers.New()
	// One resolution, three readers: the process PATH exec.LookPath resolves a
	// binary through, what a new session inherits, and the editor lookup. They
	// are replaced together or not at all — a failed re-read leaves every one of
	// them on the pin lich booted with, which is what the surface reports.
	providerSvc.SetPathRefresh(func() error {
		next, err := terminal.ReresolveShellEnv(launchEnv)
		if err != nil {
			return err
		}
		terminal.PinPath(next)
		term.SetEnv(next)
		sys.SetEnv(next)
		return nil
	})
	// Detection resolves a provider the way the spawn does, so an agent reached
	// only through the binary setting counts as installed — otherwise a machine
	// with one configured agent and nothing on PATH reads as bare, and every
	// implicit new session opens a terminal instead.
	providerSvc.SetConfiguredBin(func(id string) string { return db.ProviderBin(id, "") })
	dispatcher.Register("providers", providerSvc)
	// The quota reading is per session, not per machine: a session spawned from
	// a binary the user configured can spend another account entirely, and the
	// terminal is what knows the environment that binary set up.
	plans := quota.New()
	plans.SetSessions(func(sessionID string) quota.Account {
		env, read := term.SessionAccount(sessionID)
		return quota.Account{Env: env, Read: read}
	})
	term.SetRateLimitReports(plans.ReportClaude)
	dispatcher.Register("quota", plans)
	// The relay is the only service whose caller is not the window: the `lich`
	// CLI running inside a session reaches it over the same listener. It watches
	// the hooks' state reports too, to notice a target that ends a turn without
	// answering the request it was given.
	rl := relay.New(db, term, hub)
	term.SetSessionState(rl.Observe)
	// A worker's mod reports its answer, and a closed worker's errand ends
	// without a word to its caller.
	term.SetWorkerAnswer(rl.WorkerAnswered)
	term.SetWorkerUnanswered(rl.WorkerUnanswered)
	term.SetSessionClosed(rl.SessionClosed)
	term.SetErrandStatus(rl.Status)
	// Both questions the relay asks about a provider — whether its sessions
	// report at all, and whether they can answer with a tool — are about what
	// the plugin put there.
	rl.SetPlugins(plugins)
	// `lich focus` opens a card for someone outside the window, who then needs
	// the window in front, or open again: what a second launch of lich asks for.
	rl.SetRaiseWindow(window.ShowSession)
	// The language of the text lich types into sessions is read live from the
	// settings table, by the relay at every message, by the terminal at
	// every spawn and by drop at every copy notice.
	rl.SetPromptLanguage(db.PromptLanguage)
	drops.SetPromptLanguage(db.PromptLanguage)
	term.SetPromptLanguage(db.PromptLanguage)
	// A session whose provider runs lich's hooks is held until its session-start
	// report, which comes only from the agent's own prompt, past a trust
	// question that a quiet screen cannot be told apart from.
	term.SetStartReports(plugins.Installed)
	// The scheduled prompts are the relay's other clock: nobody calls in for
	// them, they come due.
	go rl.RunSchedules()
	// A turn a usage limit ended is picked up again once it resets, through
	// the same scheduled prompt. Codex reports no end for that turn, so the
	// terminal watches its open turns for one.
	term.SetUsageLimit(rl.ParkResume)
	go term.RunLimitWatch()
	dispatcher.Register("relay", rl)
	// Its caller is not the window either: opening a session for an agent starts
	// the PTY here rather than waiting for someone to click the card.
	spawner := spawn.New(db, proj, term, hub)
	// A subagent worker in its caller's checkout closes once it reported and
	// its turn ended, the way a native subagent ends with its result.
	rl.SetWorkerFinished(spawner.CloseFinishedWorker)
	dispatcher.Register("spawn", spawner)
	dispatcher.Register("themes", themes.New(version))
	// The tray is what a lich with its window closed shows of itself; it comes
	// up once the page hands it its words (tray.SetLabels).
	dispatcher.Register("tray", tray.New(tray.Icons{PNG: trayPNG, ICO: trayICO},
		window.Show, coord.Quit, term.LiveCount))
	denyInternal(dispatcher)
	term.Mount("/rpc/", dispatcher)
	term.Mount("/drop", http.HandlerFunc(drops.Upload))
	term.Mount("/blob", http.HandlerFunc(proj.ServeBlob))
	term.Mount("/events", hub)
}

// denyInternal hides the methods that exist for Go callers inside this process
// but must never answer a page POST. Registration exposes every exported method
// of a service (internal/rpc), so each of these is one /rpc/ path away from the
// window:
//
//   - store.Close ends the database the whole app is still using.
//   - drop.Upload and drop.Save take the dropped file's bytes as the request
//     body, so the upload is its own endpoint — the RPC envelope is a JSON
//     argument array with a 1MB bound.
//   - relay.Observe is the hooks' session-state stream, which arrives over
//     /hook: forging a SessionEnd here closes another session's errands.
//   - relay.WorkerAnswered is a worker's answer, which arrives over
//     /mod/answer: called here it answers another session's errand.
//     relay.WorkerUnanswered arrives the same way and ends one unanswered.
//   - relay.SessionClosed is what the terminal tells the relay as it closes a
//     session: called here it ends a running worker's errand, in silence when
//     the closer named is the worker's caller.
//   - terminal.CloseBy is a close on behalf of a session, which `lich close`
//     reaches through spawn.Close: called here with the worker's caller as the
//     closer, it ends that caller's errand without telling it. The window
//     closes with terminal.Close.
//   - drop.Purge deletes every copy dropped into a session, by id: the page
//     closes sessions through the store, which is what reports one gone.
//   - drop.SetPicker is startup wiring like the ones below, and nilling it
//     would leave the footer's attach button with no dialog to open.
//   - terminal.SessionAccount hands back the environment of the process a
//     session runs, credentials and all, so the quota reader can tell which
//     account that session spends.
//   - agentplugin.RepairRegistrations rewrites provider config files and
//     shells out to their CLIs; it is a launch step, not something a page asks.
//   - relay.RunSchedules is the scheduled-prompt loop, started once at launch
//     and never returning: called over /rpc/ it holds that request open for the
//     life of the process and starts a second loop racing the first for every
//     due prompt. terminal.RunLimitWatch is the same kind of loop, for the
//     Codex turns a usage limit ended.
//   - relay.ParkResume parks a prompt that types itself at a session; the
//     terminal calls it for a turn a usage limit ended, with the reset it read.
//   - spawn.CloseFinishedWorker closes a session with none of spawn.Close's
//     checks; the relay calls it for a worker that reported back.
//   - relay.SetPlugins, relay.SetWorkerFinished, relay.SetPromptLanguage, drop.SetPromptLanguage, project.SetAccounts, project.SetProjects,
//     system.SetShowWindow, system.SetQuit, system.SetCloseWindow,
//     quota.SetSessions, store.SetSessionGone, store.SetScheduleForfeited,
//     store.SetBranchOf, store.SetTranscriptOf, store.SetConversationsOf,
//     store.SetCheckoutsOf, terminal.SetDropDir,
//     terminal.SetRateLimitReports, terminal.SetUsageLimit, terminal.SetWorkerAnswer,
//     terminal.SetWorkerUnanswered, terminal.SetStartReports, terminal.SetPromptLanguage and terminal.SetSessionClosed are startup wiring. Called with [null] they silently
//     nil what they wired (encoding/json leaves a func or pointer alone on
//     null), and the write races the readers already serving — nilling
//     SetProjects also disarms the guard that keeps two projects off the same
//     directory, and SetDropDir points the sandbox's read-only bind wherever
//     the caller likes.
//   - quota.ReportClaude is a session's own reading of its plan, which arrives
//     over /mod/usage: called here it would put any numbers on the gauge.
//   - terminal.EnqueueModCommand and terminal.RunModCommand queue a command for
//     a session's mod with none of spawn.Control's checks (Claude Code only,
//     never the caller's own session); `lich control` and control_session
//     reach them through spawn.Control, `lich ask` and ask_session through
//     spawn.Ask.
//   - terminal.SubmitPrompt queues a prompt for a session's mod with none of the
//     relay's ticket accounting; the relay is its one caller.
func denyInternal(d *rpc.Handler) {
	for _, method := range []string{
		"store.Close",
		"system.SetShowWindow",
		"system.SetQuit",
		"system.SetCloseWindow",
		"store.SetSessionGone",
		"store.SetScheduleForfeited",
		"store.SetBranchOf",
		"store.SetTranscriptOf",
		"store.SetConversationsOf",
		"store.SetCheckoutsOf",
		"drop.Upload",
		"drop.Save",
		"drop.Purge",
		"drop.SetPicker",
		"drop.SetPromptLanguage",
		"relay.Observe",
		"relay.WorkerAnswered",
		"relay.WorkerUnanswered",
		"relay.SessionClosed",
		"terminal.CloseBy",
		"relay.RunSchedules",
		"relay.ParkResume",
		"agentplugin.RepairRegistrations",
		"relay.SetPlugins",
		"relay.SetWorkerFinished",
		"relay.SetPromptLanguage",
		"spawn.CloseFinishedWorker",
		"project.SetAccounts",
		"project.SetProjects",
		"quota.SetSessions",
		"quota.ReportClaude",
		"terminal.SessionAccount",
		"terminal.SetDropDir",
		"terminal.SetRateLimitReports",
		"terminal.SetUsageLimit",
		"terminal.RunLimitWatch",
		"terminal.SetWorkerAnswer",
		"terminal.SetWorkerUnanswered",
		"terminal.SetSessionClosed",
		"terminal.SetStartReports",
		"terminal.SetPromptLanguage",
		"terminal.EnqueueModCommand",
		"terminal.RunModCommand",
		"terminal.SubmitPrompt",
	} {
		d.Deny(method)
	}
}

// runChromium serves the embedded frontend on the loopback listener, opens it
// in a Chromium --app window (lich's own on Linux, the system browser
// elsewhere, internal/chromium) and serves until lich is told to quit. The
// window is not the app's lifecycle: closing it leaves every session running,
// and a second launch opens a new one on them (handleBindFailure).
func runChromium(term *terminal.Service, configDir string, coord *restart.Coordinator, window *restart.Window) {
	info := term.Transport()
	if info.Port == 0 {
		handleBindFailure(configDir, term.TransportError()) // never returns
	}

	// The runtime file lets install.sh reach a running lich for /restart when it
	// runs outside a lich terminal (no LICH_PORT/LICH_TOKEN in the env), and lets
	// a second launch find this instance to show its window instead of dying (see
	// handleBindFailure). Removed on the clean exit; a stale file from a crash is
	// harmless (the token check rejects a mismatched or dead listener).
	if path, err := singleton.Write(configDir, info.Port, info.Token); err != nil {
		slog.Warn("runtime file", "err", err)
	} else {
		defer func() { _ = os.Remove(path) }()
	}

	dist, err := fs.Sub(assets, "frontend/dist")
	if err != nil {
		slog.Error("embedded frontend", "err", err)
		os.Exit(1)
	}
	term.MountPublic("/", http.FileServerFS(dist))

	serve(coord, window)
}

// serve opens the window and returns when lich is told to quit (`lich quit`,
// a restart, SIGINT or SIGTERM), so the caller's defers still run. The window
// is closed on the way out rather than left to die with the process, so
// Chromium flushes its profile first.
func serve(coord *restart.Coordinator, window *restart.Window) {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stop)
	coord.SetStop(func() {
		// A second quit while the first is unwinding has nothing left to do.
		select {
		case stop <- os.Interrupt:
		default:
		}
	})
	window.Show()
	<-stop
	slog.Info("quitting")
	window.Close()
}

// windowTarget is what every window this lich opens points at.
//
// LICH_DEV_URL points the window at the Vite dev server instead of the
// embedded frontend (see `task dev`); the token and the backend port ride the
// query string so the page can find the RPC listener across the origin split.
type windowTarget struct {
	// url carries the token; addr is the same page without it, for the logs.
	url, addr  string
	profileDir string
	class      string
}

// pageOn is the page lich's own window opens, on a session's card when focus
// names one (the page reads it off its focus parameter at load). shell=1 says
// the page is in lich's window, whose close it can ask about; a plain tab of the
// macOS fallback cannot answer it, and would show the browser's own dialog.
func (t windowTarget) pageOn(focus string) string {
	page := t.url + "&shell=1"
	if focus != "" {
		page += "&focus=" + url.QueryEscape(focus)
	}
	return page
}

func newWindowTarget(info terminal.TransportInfo, configDir string) windowTarget {
	// The token stays out of the logs on purpose: the log file persists
	// across sessions, the token must not.
	addr := fmt.Sprintf("http://127.0.0.1:%d/", info.Port)
	target := windowTarget{
		url:        addr + "?token=" + info.Token,
		addr:       addr,
		profileDir: filepath.Join(configDir, "lich", "chromium-profile"),
		class:      "lich",
	}
	if dev := os.Getenv("LICH_DEV_URL"); dev != "" {
		target.addr = dev + "/"
		target.url = fmt.Sprintf("%s/?token=%s&backend=%d", dev, info.Token, info.Port)
		target.profileDir = filepath.Join(configDir, "lich", "chromium-profile-dev")
		// Own WM_CLASS: compositor rules for the daily driver must not
		// capture the dev window.
		target.class = "lichdev"
	}
	return target
}

// newCoordinator is the in-place restart: the update flow (install.sh) POSTs
// /restart after replacing the binary. os.Environ() here carries the pinned
// LICH_LISTEN_PORT so the successor rebinds the same port, and the window's
// switches (`lich -- <flags>`) go to it as arguments, so its window opens with
// them too. A missing executable path only disables restart; the app still runs.
func newCoordinator(windowFlags []string) *restart.Coordinator {
	exe, err := os.Executable()
	if err != nil {
		slog.Warn("resolve executable — restart disabled", "err", err)
		exe = ""
	}
	return restart.New(exe, os.Environ(), chromium.RelaunchArgs(windowFlags))
}

// newWindow builds the keeper of this lich's window. Extra CLI args after `--`
// pass through to Chromium (e.g. `lich -- --ozone-platform=wayland`), on every
// window it opens. live counts the sessions with a process, and quit ends lich:
// a window closed with none running leaves nothing to keep it for.
func newWindow(info terminal.TransportInfo, configDir string, extra []string,
	live func() int, quit func() error) *restart.Window {
	target := newWindowTarget(info, configDir)
	return restart.NewWindow(
		func(focus string, onStart func(close func() error)) error {
			slog.Info("chromium shell opening", "addr", target.addr)
			return chromium.Run(target.pageOn(focus), target.profileDir, target.class, extra, onStart)
		},
		func() { focusRunning(configDir, &singleton.Info{Port: info.Port, Token: info.Token}) },
		func(end restart.WindowEnd) {
			windowEnded(end, target, configDir)
			quitIfIdle(end, live, quit)
		},
	)
}

// quitIfIdle ends lich when the user closed its window with no session running:
// the backend outlives the window to keep sessions alive, and there are none.
func quitIfIdle(end restart.WindowEnd, live func() int, quit func() error) {
	if chromium.EndingOf(end.Err, end.First, end.Uptime) != chromium.WindowClosed || live() > 0 {
		return
	}
	slog.Info("window closed with no session running, quitting")
	if err := quit(); err != nil {
		slog.Warn("quit after the window closed", "err", err)
	}
}

// windowEnded performs what chromium.EndingOf says a window's end means.
func windowEnded(end restart.WindowEnd, target windowTarget, configDir string) {
	switch chromium.EndingOf(end.Err, end.First, end.Uptime) {
	case chromium.WindowClosed:
		slog.Info("window closed, lich keeps running")
	case chromium.WindowTabInstead:
		openTab(target, configDir)
	case chromium.WindowFailed:
		slog.Error("chromium shell", "err", end.Err)
		reportWindowFailure(end.Err, configDir,
			"lich is still running with its sessions: launch it again to reopen the window, "+
				"or run `lich quit` to end it.")
	case chromium.WindowNeverOpened:
		slog.Error("chromium shell", "err", end.Err)
		reportWindowFailure(end.Err, configDir, "")
		os.Exit(1)
	}
}

// reportWindowFailure is the dialog for a window that would not open or died.
// A launcher start has no terminal, so without it a browser that will not
// launch reads as lich doing nothing (#409). Best effort — where no dialog
// backend answers, the log line the caller wrote still has the story.
func reportWindowFailure(err error, configDir, after string) {
	text := fmt.Sprintf("lich could not open its window.\n\n%s\n\n", chromium.Why(err))
	if after != "" {
		text += after + "\n\n"
	}
	text += "Log: " + logging.Path(filepath.Join(configDir, "lich"))
	_ = zenity.Error(text, zenity.Title("lich"))
}

// openTab is macOS's last rung (chromium.TabFallback), the one that keeps a
// product on a Mac whose window is missing or died at startup: lich hands its
// URL to the default browser and goes on serving it, as it does after any
// window closes. A tab lich did not spawn cannot be closed on quit, nor told
// apart from a closed one, so a second launch opens another tab.
func openTab(target windowTarget, configDir string) {
	slog.Warn("no window of its own, opening a plain tab", "addr", target.addr)
	// stdout rather than the log: the URL carries the session token, and the
	// log file outlives this run (see windowTarget). A terminal launch reads
	// it here; a launcher launch reads the notification below.
	fmt.Println("lich is running at", target.url)

	if err := system.OpenURL(target.url); err != nil {
		slog.Error("no browser to open at all", "err", err)
		_ = zenity.Error(fmt.Sprintf(
			"lich is running, but found no browser to show it in.\n\n"+
				"Open this URL in any browser:\n%s\n\nLog: %s",
			target.url, logging.Path(filepath.Join(configDir, "lich"))),
			zenity.Title("lich"))
		return
	}
	_ = zenity.Notify(
		"lich is running at "+target.addr+" — opened in your default browser. "+
			"This Mac has no lich window of its own: closing the tab leaves lich running.",
		zenity.Title("lich"))
}

// handleBindFailure runs when the pinned listener would not bind, and never
// returns. It gathers the two inputs the decision needs, asks
// singleton.BindFailureVerdict what they mean, and performs the effects. cause
// is the bind error, carried into the log for the launches that end here.
func handleBindFailure(configDir string, cause error) {
	port := os.Getenv("LICH_LISTEN_PORT")
	restartWait := os.Getenv(restart.WaitEnv)
	// A restart successor never probes: the verdict is already decided, and the
	// probe would only cost it a timeout on a port it is racing for.
	var running *singleton.Info
	if restartWait == "" {
		want, _ := strconv.Atoi(port)
		running, _ = singleton.Detect(configDir, want, singleton.Ping)
	}
	if singleton.BindFailureVerdict(restartWait, running) == singleton.BindFailureIsDuplicate {
		slog.Info("lich already running, showing its window",
			"pid", running.PID, "port", running.Port)
		showRunning(configDir, running)
		os.Exit(0)
	}
	// No "is the port free?" here: on Windows the answer is regularly yes and
	// the bind still fails, so the OS error is the message.
	slog.Error("loopback listener failed to start", "port", port, "err", cause)
	// The window never opens on this path, so a double-click launch dies in
	// silence with only the log to explain — and nothing on screen says a log
	// exists. The dialog is the one surface left. Best effort: where no dialog
	// backend answers, the log line above still has the story.
	_ = zenity.Error(fmt.Sprintf(
		"lich could not listen on 127.0.0.1:%s.\n\n%v\n\n"+
			"If another program is holding the port, set the LICH_LISTEN_PORT "+
			"environment variable to a free one and launch lich again.\n\n"+
			"Log: %s",
		port, cause, logging.Path(filepath.Join(configDir, "lich"))),
		zenity.Title("lich"))
	os.Exit(1)
}

// showRunning asks the running lich for its window: it opens one when the user
// had closed it, and brings the open one forward otherwise. The running lich
// owns that window, so it is tied to that backend and not to this launch,
// which exits. A lich from before the backend outlived its window has no such
// call, and always has its window open: focusing it through the profile lock
// is what reaches it.
func showRunning(configDir string, running *singleton.Info) {
	if err := singleton.Show(running); err != nil {
		slog.Warn("show the running lich's window, focusing it instead", "err", err)
		focusRunning(configDir, running)
	}
}

// focusRunning brings the already-running lich's window to the front by handing
// its URL to Chromium against the shared profile: Chromium's profile-lock IPC
// forwards the command to the running browser (the same lock that stops a second
// window spawning its own process — see chromium.Args) instead of opening a new
// one. Best effort — a failure only means the user raises the window by hand.
//
// Skipped for the dev shell (its own profile/port). The window raises itself
// on the forward (the kurogane fork's relaunch hook). A per-platform raise from
// here is no alternative: Wayland forbids it for an external process, so
// Chromium's IPC is the portable lever we have.
func focusRunning(configDir string, running *singleton.Info) {
	if os.Getenv("LICH_DEV_URL") != "" {
		return
	}
	profileDir := filepath.Join(configDir, "lich", "chromium-profile")
	url := fmt.Sprintf("http://127.0.0.1:%d/?token=%s", running.Port, running.Token)
	if err := chromium.Focus(url, profileDir, "lich"); err != nil {
		slog.Warn("focus existing window", "err", err)
		// The running lich has no window of its own either (it is serving a
		// tab); the honest "focus" is another tab pointed at it.
		if errors.Is(err, chromium.ErrNoShell) {
			_ = system.OpenURL(url)
		}
	}
}

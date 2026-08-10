package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

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
	"github.com/omartelo/lich/internal/relay"
	"github.com/omartelo/lich/internal/restart"
	"github.com/omartelo/lich/internal/rpc"
	"github.com/omartelo/lich/internal/singleton"
	"github.com/omartelo/lich/internal/store"
	"github.com/omartelo/lich/internal/system"
	"github.com/omartelo/lich/internal/terminal"
	"github.com/omartelo/lich/internal/themes"
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

// defaultListenPort pins the loopback listener so the page origin — and with
// it the frontend's localStorage (lich.* settings) — survives restarts.
// LICH_LISTEN_PORT overrides (not LICH_PORT, the per-session hook variable).
const defaultListenPort = "47821"

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

	// Snapshot before any env tweaks: spawned terminal sessions must inherit
	// what the user launched lich with (see terminal.childEnv). ResolveShellEnv
	// recovers the rc-exported vars a GUI launch misses (see its doc).
	env := terminal.ResolveShellEnv(os.Environ())

	configDir, err := os.UserConfigDir()
	if err != nil {
		slog.Error("resolve config dir", "err", err)
		os.Exit(1)
	}
	// File logging as early as possible: every startup failure below must be
	// readable after the fact — on Windows the console may not exist at all.
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

	if os.Getenv("LICH_LISTEN_PORT") == "" {
		if err := os.Setenv("LICH_LISTEN_PORT", defaultListenPort); err != nil {
			slog.Error("set LICH_LISTEN_PORT", "err", err)
			os.Exit(1)
		}
	}

	db, err := store.New()
	if err != nil {
		slog.Error("open store", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	// App events ride the /events socket; no client connected means no
	// listener yet (the window is still starting) and the event is dropped.
	hub := events.New()
	term := terminal.New(db, env, hub)
	proj := project.New(project.ZenityPicker{})
	// gh has one active account per host; a project can name a different one.
	proj.SetAccounts(db.GHAccountForPath)

	// Every service the frontend uses goes through the loopback RPC
	// (internal/rpc). store.Close manages the DB lifecycle and stays Go-only.
	dispatcher := rpc.New()
	drops := drop.New(configDir)
	// The other prune runs after each new copy; this one is what clears the
	// last of them for a lich that is never dropped on again.
	drops.Prune()
	dispatcher.Register("terminal", term)
	dispatcher.Register("drop", drops)
	dispatcher.Register("fonts", fonts.New())
	dispatcher.Register("project", proj)
	dispatcher.Register("agentplugin", agentplugin.New(db))
	dispatcher.Register("appupdate", appupdate.New(version))
	dispatcher.Register("patchnotes", patchnotes.New(version, changelog))
	dispatcher.Register("store", db)
	dispatcher.Register("system", system.New(env, logPath, version))
	dispatcher.Register("providers", providers.New())
	// The relay is the only service whose caller is not the window: the `lich`
	// CLI running inside a session reaches it over the same listener. It watches
	// the hooks' state reports too, to notice a target that ends a turn without
	// answering the request it was given.
	rl := relay.New(db, term, hub)
	term.SetSessionState(rl.Observe)
	dispatcher.Register("relay", rl)
	dispatcher.Register("themes", themes.New())
	dispatcher.Deny("store.Close")
	// The dropped file's bytes are the request body, so the upload is its own
	// endpoint: the RPC envelope is a JSON argument array with a 1MB bound.
	dispatcher.Deny("drop.Upload")
	dispatcher.Deny("drop.Save")
	term.Mount("/rpc/", dispatcher)
	term.Mount("/drop", http.HandlerFunc(drops.Upload))
	term.Mount("/events", hub)

	// In-place restart: the update flow (install.sh) POSTs /restart after
	// replacing the binary. os.Environ() here carries the pinned LICH_LISTEN_PORT
	// so the successor rebinds the same port. A missing executable path only
	// disables restart; the app still runs.
	exe, err := os.Executable()
	if err != nil {
		slog.Warn("resolve executable — restart disabled", "err", err)
		exe = ""
	}
	coord := restart.New(exe, os.Environ())
	term.SetRestart(coord.Do)

	runChromium(term, configDir, coord)
}

// runChromium serves the embedded frontend on the loopback listener and opens
// it in the system Chromium's --app mode; the browser process exiting is the
// app lifecycle. Extra CLI args after `--` pass through to Chromium
// (e.g. `lich -- --ozone-platform=wayland`).
//
// LICH_DEV_URL points the window at the Vite dev server instead of the
// embedded frontend (see `task dev`); the token and the backend port ride the
// query string so the page can find the RPC listener across the origin split.
func runChromium(term *terminal.Service, configDir string, coord *restart.Coordinator) {
	info := term.Transport()
	if info.Port == 0 {
		handleBindFailure(configDir) // never returns
	}

	// The runtime file lets install.sh reach a running lich for /restart when it
	// runs outside a lich terminal (no LICH_PORT/LICH_TOKEN in the env), and lets
	// a second launch find this instance to focus it instead of dying (see
	// handleBindFailure). Removed on the clean window-close exit; a stale file
	// from a crash is harmless (the token check rejects a mismatched or dead
	// listener).
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

	profileDir := filepath.Join(configDir, "lich", "chromium-profile")

	// The token stays out of the logs on purpose: the log file persists
	// across sessions, the token must not.
	addr := fmt.Sprintf("http://127.0.0.1:%d/", info.Port)
	url := addr + "?token=" + info.Token
	class := "lich"
	if dev := os.Getenv("LICH_DEV_URL"); dev != "" {
		addr = dev + "/"
		url = fmt.Sprintf("%s/?token=%s&backend=%d", dev, info.Token, info.Port)
		profileDir = filepath.Join(configDir, "lich", "chromium-profile-dev")
		// Own WM_CLASS: compositor rules for the daily driver must not
		// capture the dev window.
		class = "lichdev"
	}
	slog.Info("chromium shell opening", "addr", addr)

	var extra []string
	if args := os.Args[1:]; len(args) > 1 && args[0] == "--" {
		extra = args[1:]
	}
	if err := chromium.Run(url, profileDir, class, extra, coord.SetWindow); err != nil {
		slog.Error("chromium shell", "err", err)
		os.Exit(1)
	}
	slog.Info("window closed, exiting")
}

// handleBindFailure runs when the pinned listener would not bind, and never
// returns. It gathers the two inputs the decision needs, asks
// singleton.BindFailureVerdict what they mean, and performs the effects.
func handleBindFailure(configDir string) {
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
		slog.Info("lich already running, focusing existing window",
			"pid", running.PID, "port", running.Port)
		focusRunning(configDir, running)
		os.Exit(0)
	}
	slog.Error("loopback listener failed to start — is the port free?", "port", port)
	os.Exit(1)
}

// focusRunning brings the already-running lich's window to the front by handing
// its URL to Chromium against the shared profile: Chromium's profile-lock IPC
// forwards the command to the running browser (the same lock that stops a second
// window spawning its own process — see chromium.Args) instead of opening a new
// one. Best effort — a failure only means the user raises the window by hand.
//
// Skipped for the dev shell (its own profile/port). On some Chromium builds a
// forwarded --app may open a second app window rather than focus; the dup-free
// fix is a per-platform window raise, which Wayland forbids for an external
// process, so Chromium's IPC is the portable lever we have.
func focusRunning(configDir string, running *singleton.Info) {
	if os.Getenv("LICH_DEV_URL") != "" {
		return
	}
	profileDir := filepath.Join(configDir, "lich", "chromium-profile")
	url := fmt.Sprintf("http://127.0.0.1:%d/?token=%s", running.Port, running.Token)
	if err := chromium.Run(url, profileDir, "lich", nil, nil); err != nil {
		slog.Warn("focus existing window", "err", err)
	}
}

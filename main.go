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

	"github.com/omartelo/lich/internal/appupdate"
	"github.com/omartelo/lich/internal/chromium"
	"github.com/omartelo/lich/internal/claudeplugin"
	"github.com/omartelo/lich/internal/events"
	"github.com/omartelo/lich/internal/fonts"
	"github.com/omartelo/lich/internal/logging"
	"github.com/omartelo/lich/internal/project"
	"github.com/omartelo/lich/internal/providers"
	"github.com/omartelo/lich/internal/restart"
	"github.com/omartelo/lich/internal/rpc"
	"github.com/omartelo/lich/internal/singleton"
	"github.com/omartelo/lich/internal/store"
	"github.com/omartelo/lich/internal/system"
	"github.com/omartelo/lich/internal/terminal"
)

// The frontend is embedded into the binary and served over the loopback
// listener to the Chromium --app window (docs/chromium-shell.md).

//go:embed all:frontend/dist
var assets embed.FS

// defaultListenPort pins the loopback listener so the page origin — and with
// it the frontend's localStorage (lich.* settings) — survives restarts.
// LICH_LISTEN_PORT overrides (not LICH_PORT, the per-session hook variable).
const defaultListenPort = "47821"

// version is the running build's version, injected at build time via
// -ldflags "-X main.version=<git tag>" (see Taskfile.yml). Unset in dev builds
// ("dev"), which the update check treats as "not a release".
var version = "dev"

func main() {
	// Snapshot before any env tweaks: spawned terminal sessions must inherit
	// what the user launched lich with (see terminal.childEnv).
	env := os.Environ()

	configDir, err := os.UserConfigDir()
	if err != nil {
		slog.Error("resolve config dir", "err", err)
		os.Exit(1)
	}
	// File logging as early as possible: every startup failure below must be
	// readable after the fact — on Windows the console may not exist at all.
	if closer, err := logging.Init(filepath.Join(configDir, "lich")); err != nil {
		slog.Warn("file log unavailable, stderr only", "err", err)
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

	// Every service the frontend uses goes through the loopback RPC
	// (internal/rpc). store.Close manages the DB lifecycle and stays Go-only.
	dispatcher := rpc.New()
	dispatcher.Register("terminal", term)
	dispatcher.Register("fonts", fonts.New())
	dispatcher.Register("project", proj)
	dispatcher.Register("claudeplugin", claudeplugin.New(db))
	dispatcher.Register("appupdate", appupdate.New(version))
	dispatcher.Register("store", db)
	dispatcher.Register("system", system.New())
	dispatcher.Register("providers", providers.New())
	dispatcher.Deny("store.Close")
	term.Mount("/rpc/", dispatcher)
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
// returns. A fresh launch whose port is held by another live lich is a duplicate
// launch: focus that window and exit 0 — the user re-launching an app they
// already have open should get the window, not an error. Anything else (a
// restart successor that never got the port back, or a non-lich process sitting
// on the port) is a real failure: log it and exit 1.
func handleBindFailure(configDir string) {
	port := os.Getenv("LICH_LISTEN_PORT")
	// A restart successor legitimately expects the port to be busy for a moment
	// (it retries the bind); a failure there is real, not a duplicate launch.
	if os.Getenv(restart.WaitEnv) == "" {
		want, _ := strconv.Atoi(port)
		if running, _ := singleton.Detect(configDir, want, singleton.Ping); running != nil {
			slog.Info("lich already running, focusing existing window",
				"pid", running.PID, "port", running.Port)
			focusRunning(configDir, running)
			os.Exit(0)
		}
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

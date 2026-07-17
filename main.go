package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

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
		slog.Error("loopback listener failed to start — is the port free?",
			"port", os.Getenv("LICH_LISTEN_PORT"))
		os.Exit(1)
	}

	// The runtime file lets install.sh reach a running lich for /restart when it
	// runs outside a lich terminal (no LICH_PORT/LICH_TOKEN in the env). Removed
	// on the clean window-close exit; a stale file from a crash is harmless (the
	// token check rejects a mismatched or dead listener).
	if path, err := writeRuntimeFile(configDir, info.Port, info.Token); err != nil {
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

// writeRuntimeFile records this process's pid and loopback coordinates so a
// script (install.sh) can reach lich for /restart without inheriting the
// per-session hook env. Mode 0600: the token is a loopback credential.
func writeRuntimeFile(configDir string, port int, token string) (string, error) {
	path := filepath.Join(configDir, "lich", "runtime.json")
	data, err := json.Marshal(struct {
		PID   int    `json:"pid"`
		Port  int    `json:"port"`
		Token string `json:"token"`
	}{os.Getpid(), port, token})
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

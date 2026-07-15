package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/omartelo/lich/internal/chromium"
	"github.com/omartelo/lich/internal/claudeplugin"
	"github.com/omartelo/lich/internal/events"
	"github.com/omartelo/lich/internal/fonts"
	"github.com/omartelo/lich/internal/project"
	"github.com/omartelo/lich/internal/rpc"
	"github.com/omartelo/lich/internal/store"
	"github.com/omartelo/lich/internal/system"
	"github.com/omartelo/lich/internal/terminal"
)

// The frontend is embedded into the binary and served to whichever shell is
// running: handed to the Wails asset handler, or mounted on the loopback
// listener for the Chromium shell.

//go:embed all:frontend/dist
var assets embed.FS

// chromiumDefaultPort pins the loopback listener in the Chromium shell so the
// page origin — and with it the frontend's localStorage (lich.* settings) —
// survives restarts. LICH_LISTEN_PORT overrides (not LICH_PORT, which is the
// per-session hook-contract variable).
const chromiumDefaultPort = "47821"

// main wires the shared core (store, event hub, services, RPC) and hands off
// to the selected shell: the Wails webview (default) or the system Chromium
// in --app mode (LICH_SHELL=chromium; see docs/chromium-shell.md).
func main() {
	// Snapshot the environment before any tweaks below: spawned terminal
	// sessions must inherit what the user launched lich with, not our GTK
	// workarounds (see terminal.childEnv).
	env := os.Environ()

	// The Wails webview is the default shell where this build carries it;
	// LICH_SHELL=chromium opts into the Chromium shell, and a nowails build
	// (no Wails/GTK linkage at all) has only the Chromium shell.
	chromiumShell := os.Getenv("LICH_SHELL") == "chromium" || !wailsShellAvailable

	if chromiumShell {
		// Stable origin for localStorage; set before terminal.New starts the
		// listener. An explicit LICH_LISTEN_PORT wins.
		if os.Getenv("LICH_LISTEN_PORT") == "" {
			if err := os.Setenv("LICH_LISTEN_PORT", chromiumDefaultPort); err != nil {
				log.Fatalf("failed to set LICH_LISTEN_PORT: %v", err)
			}
		}
	} else if runtime.GOOS == "linux" && os.Getenv("GDK_BACKEND") == "" {
		// WebKitGTK under Wayland fractional scaling renders every damage
		// frame at 2x and downsamples it on the CPU — typing in a full-size
		// window cost ~40ms/frame of engine time regardless of raster backend
		// (measured 2026-07-10; GPU policy, DMABUF, Skia threads and canvas
		// alpha all made no difference). Under X11/Xwayland the app sees an
		// integer scale and the same workload runs stall-free at full frame
		// rate. Respect an explicit GDK_BACKEND so this stays overridable.
		// Chromium brings its own compositor, so the tweak is Wails-only.
		if err := os.Setenv("GDK_BACKEND", "x11"); err != nil {
			log.Printf("failed to set GDK_BACKEND: %v", err)
		}
	}

	db, err := store.New()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// App events route through the hub: the /events socket when a shell is
	// connected to it, the Wails event bridge otherwise. The Chromium shell
	// has no bridge — events wait for the socket.
	var hub *events.Hub
	if chromiumShell {
		hub = events.New(nil)
	} else {
		hub = events.New(eventFallback())
	}
	term := terminal.New(db, env, hub)

	// Native pickers: the Wails dialog needs the Wails app; the Chromium
	// shell asks through zenity.
	proj := project.New(shellPicker(chromiumShell))

	// Every service is also reachable over loopback HTTP (see internal/rpc),
	// so the frontend runs identically in both shells. store.Close manages
	// the DB lifecycle and stays Go-only.
	fontSvc := fonts.New()
	plugin := claudeplugin.New(db)
	dispatcher := rpc.New()
	dispatcher.Register("terminal", term)
	dispatcher.Register("fonts", fontSvc)
	dispatcher.Register("project", proj)
	dispatcher.Register("claudeplugin", plugin)
	dispatcher.Register("store", db)
	dispatcher.Register("system", system.New())
	dispatcher.Deny("store.Close")
	term.Mount("/rpc/", dispatcher)
	term.Mount("/events", hub)

	if chromiumShell {
		runChromium(term)
		return
	}
	runWails(term, proj, fontSvc, plugin, db)
}

// runChromium serves the embedded frontend on the loopback listener and opens
// it in the system Chromium's --app mode; the browser process exiting is the
// app lifecycle. Extra CLI args pass through to Chromium
// (e.g. `lich -- --ozone-platform=wayland`).
func runChromium(term *terminal.Service) {
	info := term.Transport()
	if info.Port == 0 {
		log.Fatalf("loopback listener failed to start — is port %s (LICH_LISTEN_PORT) free?", os.Getenv("LICH_LISTEN_PORT"))
	}

	dist, err := fs.Sub(assets, "frontend/dist")
	if err != nil {
		log.Fatalf("embedded frontend: %v", err)
	}
	term.MountPublic("/", http.FileServerFS(dist))

	configDir, err := os.UserConfigDir()
	if err != nil {
		log.Fatalf("resolve config dir: %v", err)
	}
	profileDir := filepath.Join(configDir, "lich", "chromium-profile")

	url := fmt.Sprintf("http://127.0.0.1:%d/?token=%s", info.Port, info.Token)
	log.Printf("[lich] chromium shell on %s", url)

	var extra []string
	if args := os.Args[1:]; len(args) > 1 && args[0] == "--" {
		extra = args[1:]
	}
	if err := chromium.Run(url, profileDir, extra); err != nil {
		log.Fatal(err)
	}
}

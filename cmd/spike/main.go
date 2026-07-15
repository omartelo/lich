// Spike for docs/chromium-shell.md option 1: serve the built frontend over
// loopback HTTP and open it in the system Chromium's --app mode, with one
// PTY bridged over a WebSocket per connection. It exists to measure whether
// Chromium's compositor kills the paint jank WebKitGTK shows on the same
// machine — run scenarios in the spike window, read the stats overlay.
//
// Run from the repo root (after `cd frontend && pnpm build`):
//
//	go run ./cmd/spike                       # picks chromium/chrome from PATH
//	go run ./cmd/spike -no-browser           # just prints the URL
//	go run ./cmd/spike -- --ozone-platform=wayland   # extra Chromium flags
//
// Disposable: delete cmd/spike, frontend/spike.html and frontend/src/spike
// once the shell decision lands.
package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/coder/websocket"
	"github.com/creack/pty"

	"github.com/omartelo/lich/internal/chromium"
)

func main() {
	distDir := flag.String("dist", "frontend/dist", "built frontend directory")
	noBrowser := flag.Bool("no-browser", false, "only print the URL, do not launch a browser")
	flag.Parse()

	if _, err := os.Stat(*distDir + "/spike.html"); err != nil {
		log.Fatalf("%s/spike.html not found — run `cd frontend && pnpm build` first", *distDir)
	}

	token, err := randomToken()
	if err != nil {
		log.Fatalf("token: %v", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(*distDir)))
	mux.HandleFunc("/ws", wsHandler(token))
	go func() {
		if err := http.Serve(listener, mux); err != nil {
			log.Fatalf("serve: %v", err)
		}
	}()

	url := fmt.Sprintf("http://%s/spike.html?token=%s", listener.Addr(), token)
	log.Printf("[spike] serving %s", url)

	if *noBrowser {
		waitForSignal()
		return
	}
	if err := runBrowser(url, flag.Args()); err != nil {
		log.Fatalf("browser: %v", err)
	}
}

func randomToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func waitForSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	<-ch
}

// runBrowser launches the --app window on a throwaway profile and blocks
// until the user closes it; the browser process exiting is the app lifecycle.
func runBrowser(url string, extraArgs []string) error {
	dataDir, err := os.MkdirTemp("", "lich-spike-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dataDir)
	return chromium.Run(url, dataDir, extraArgs)
}

// wsHandler bridges one PTY per connection: binary frames carry PTY output
// out, JSON text frames carry input/resize in (see protocol.go). No output
// coalescing on purpose — the spike measures Chromium under the waveterm-style
// firehose, one send per PTY read.
func wsHandler(token string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("token")), []byte(token)) != 1 {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			log.Printf("[spike] ws accept: %v", err)
			return
		}
		defer conn.CloseNow()

		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/bash"
		}
		cmd := exec.Command(shell)
		cmd.Env = append(os.Environ(), "TERM=xterm-256color")
		ptmx, err := pty.Start(cmd)
		if err != nil {
			log.Printf("[spike] pty: %v", err)
			return
		}
		defer func() {
			_ = ptmx.Close()
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}()

		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		// PTY → WS.
		go func() {
			defer cancel()
			buf := make([]byte, 4096)
			for {
				n, err := ptmx.Read(buf)
				if n > 0 {
					if werr := conn.Write(ctx, websocket.MessageBinary, buf[:n]); werr != nil {
						return
					}
				}
				if err != nil {
					return
				}
			}
		}()

		// WS → PTY. Blocks the handler; returning tears everything down.
		for {
			typ, data, err := conn.Read(ctx)
			if err != nil {
				return
			}
			if typ != websocket.MessageText {
				continue
			}
			msg, err := parseControl(data)
			if err != nil {
				log.Printf("[spike] %v", err)
				continue
			}
			switch msg.T {
			case "in":
				if _, err := ptmx.WriteString(msg.D); err != nil {
					return
				}
			case "rs":
				if err := pty.Setsize(ptmx, &pty.Winsize{Cols: msg.Cols, Rows: msg.Rows}); err != nil {
					log.Printf("[spike] resize: %v", err)
				}
			}
		}
	}
}

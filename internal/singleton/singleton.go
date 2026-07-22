// Package singleton keeps a second lich launch from racing the pinned listener
// port and dying with a bare error. It owns runtime.json — the file recording
// the running instance's loopback coordinates ({pid,port,token}) — and, when a
// fresh launch cannot bind, uses it to tell "another live lich already holds my
// port" (a duplicate launch, exit cleanly and focus it) apart from "the port is
// taken by something else" (a real error). The pinned port bind is itself the
// lock; this package is only the detection and the read/write of the file.
package singleton

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Info is the running lich recorded in runtime.json. It lets install.sh reach a
// running lich for /restart when it runs outside a lich terminal, and lets a
// second launch find the instance already holding the pinned port.
type Info struct {
	PID   int    `json:"pid"`
	Port  int    `json:"port"`
	Token string `json:"token"`
}

// pingTimeout bounds the liveness probe: a loopback GET answers in microseconds,
// so a slow one means the recorded instance is gone and the file is stale.
const pingTimeout = time.Second

func path(configDir string) string {
	return filepath.Join(configDir, "lich", "runtime.json")
}

// Write records this process as the running instance. Mode 0600: the token is a
// loopback credential and the file persists across sessions. Returns the path so
// the caller can remove it on the clean window-close exit.
func Write(configDir string, port int, token string) (string, error) {
	p := path(configDir)
	data, err := json.Marshal(Info{PID: os.Getpid(), Port: port, Token: token})
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(p, data, 0o600); err != nil {
		return "", err
	}
	return p, nil
}

// Read loads runtime.json. A missing file returns (nil, nil): no instance
// recorded, not an error.
func Read(configDir string) (*Info, error) {
	data, err := os.ReadFile(path(configDir))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read runtime file: %w", err)
	}
	var info Info
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("parse runtime file: %w", err)
	}
	return &info, nil
}

// Detect reports the live lich holding wantPort, or nil if there is none. It
// returns the recorded instance only when runtime.json names wantPort (so a dev
// instance on another port is never mistaken for this one) and its token pings
// back (proving the recorded process is both alive and actually lich, not a
// stray process that grabbed the port). ping is injectable for tests; production
// passes Ping. A nil result with a nil error means "not a duplicate launch" —
// the caller should treat the bind failure as the real error it is.
func Detect(configDir string, wantPort int, ping func(port int, token string) bool) (*Info, error) {
	info, err := Read(configDir)
	if err != nil || info == nil {
		return nil, err
	}
	if info.Port != wantPort || info.Token == "" || !ping(info.Port, info.Token) {
		return nil, nil
	}
	return info, nil
}

// Ping reports whether a token-gated lich listener answers on port. It is the
// production probe passed to Detect: only lich serves /ping behind the token, so
// a 204 proves the recorded instance is alive and is lich.
func Ping(port int, token string) bool {
	client := http.Client{Timeout: pingTimeout}
	url := fmt.Sprintf("http://127.0.0.1:%d/ping?token=%s", port, token)
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusNoContent
}

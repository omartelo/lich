package singleton

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// writeConfig lays out the <configDir>/lich directory Write and Read expect and
// returns the config dir.
func writeConfig(t *testing.T) string {
	t.Helper()
	configDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(configDir, "lich"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	return configDir
}

func TestWriteThenRead(t *testing.T) {
	configDir := writeConfig(t)
	if _, err := Write(configDir, 47821, "tok"); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := Read(configDir)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got == nil || got.Port != 47821 || got.Token != "tok" || got.PID != os.Getpid() {
		t.Fatalf("round trip = %+v, want pid=%d port=47821 tok", got, os.Getpid())
	}
}

func TestReadMissingIsNotAnError(t *testing.T) {
	got, err := Read(t.TempDir())
	if err != nil {
		t.Fatalf("Read missing: %v", err)
	}
	if got != nil {
		t.Fatalf("Read missing = %+v, want nil", got)
	}
}

func TestReadRejectsGarbage(t *testing.T) {
	configDir := writeConfig(t)
	if err := os.WriteFile(filepath.Join(configDir, "lich", "runtime.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := Read(configDir); err == nil {
		t.Fatal("Read garbage: want error, got nil")
	}
}

// TestUncleanExit pins the three cases the launch notice is decided from: the
// file the clean exit removed, the file it never got to remove, and the file a
// restart successor finds because its predecessor is still holding it.
func TestUncleanExit(t *testing.T) {
	t.Run("no runtime file is a clean exit", func(t *testing.T) {
		if UncleanExit(writeConfig(t), "") {
			t.Fatal("UncleanExit with no runtime file = true, want false")
		}
	})

	t.Run("a leftover runtime file is an unclean exit", func(t *testing.T) {
		configDir := writeConfig(t)
		if _, err := Write(configDir, 47821, "tok"); err != nil {
			t.Fatalf("Write: %v", err)
		}
		if !UncleanExit(configDir, "") {
			t.Fatal("UncleanExit with a leftover runtime file = false, want true")
		}
	})

	t.Run("a restart successor inherits the file, not a crash", func(t *testing.T) {
		configDir := writeConfig(t)
		if _, err := Write(configDir, 47821, "tok"); err != nil {
			t.Fatalf("Write: %v", err)
		}
		if UncleanExit(configDir, "1") {
			t.Fatal("UncleanExit for a restart successor = true, want false")
		}
	})
}

// TestDefaultPortIsPinned spells the number out rather than reading the
// constant: the page's localStorage (lich.* settings) is keyed by the origin
// this port forms, so moving it wipes every user's preferences. A test that
// followed the constant would let that move through green.
func TestDefaultPortIsPinned(t *testing.T) {
	if DefaultPort != 47821 {
		t.Fatalf("DefaultPort = %d, want 47821 — moving it wipes the page's localStorage", DefaultPort)
	}
}

func TestPortReadsTheOverride(t *testing.T) {
	cases := map[string]int{
		"":           DefaultPort,
		"0":          DefaultPort,
		"-1":         DefaultPort,
		"not-a-port": DefaultPort,
		"1234":       1234,
	}
	for value, want := range cases {
		got := Port(func(key string) string {
			if key != "LICH_LISTEN_PORT" {
				t.Errorf("Port read %q, not the listener variable", key)
			}
			return value
		})
		if got != want {
			t.Errorf("Port(LICH_LISTEN_PORT=%q) = %d, want %d", value, got, want)
		}
	}
}

func TestDetect(t *testing.T) {
	const want = 47821
	seed := func(t *testing.T, port int, token string) string {
		configDir := writeConfig(t)
		if _, err := Write(configDir, port, token); err != nil {
			t.Fatalf("Write: %v", err)
		}
		return configDir
	}
	alive := func(int, string) bool { return true }
	dead := func(int, string) bool { return false }

	t.Run("live instance on the wanted port is a duplicate", func(t *testing.T) {
		got, err := Detect(seed(t, want, "tok"), want, alive)
		if err != nil {
			t.Fatalf("Detect: %v", err)
		}
		if got == nil || got.Port != want {
			t.Fatalf("Detect = %+v, want the recorded instance", got)
		}
	})

	t.Run("recorded instance on another port is not this one", func(t *testing.T) {
		got, err := Detect(seed(t, 40000, "tok"), want, alive)
		if err != nil {
			t.Fatalf("Detect: %v", err)
		}
		if got != nil {
			t.Fatalf("Detect other-port = %+v, want nil", got)
		}
	})

	t.Run("dead instance is not a duplicate", func(t *testing.T) {
		got, err := Detect(seed(t, want, "tok"), want, dead)
		if err != nil {
			t.Fatalf("Detect: %v", err)
		}
		if got != nil {
			t.Fatalf("Detect dead = %+v, want nil", got)
		}
	})

	t.Run("no runtime file means no instance", func(t *testing.T) {
		got, err := Detect(t.TempDir(), want, alive)
		if err != nil {
			t.Fatalf("Detect: %v", err)
		}
		if got != nil {
			t.Fatalf("Detect missing = %+v, want nil", got)
		}
	})
}

// TestDevSplit proves a `task dev` shell records itself in its own file: it
// neither overwrites the daily driver's runtime.json nor, when main.go removes
// the path it was handed on exit, takes that file down with it.
func TestDevSplit(t *testing.T) {
	if got := fileName(false); got != "runtime.json" {
		t.Errorf("fileName(false) = %q, want runtime.json", got)
	}
	if got := fileName(true); got != "runtime-dev.json" {
		t.Errorf("fileName(true) = %q, want runtime-dev.json", got)
	}

	configDir := writeConfig(t)
	t.Setenv("LICH_DEV", "")
	if _, err := Write(configDir, 47821, "daily"); err != nil {
		t.Fatalf("Write daily: %v", err)
	}

	t.Setenv("LICH_DEV", "1")
	devPath, err := Write(configDir, 47822, "dev")
	if err != nil {
		t.Fatalf("Write dev: %v", err)
	}
	if filepath.Base(devPath) != "runtime-dev.json" {
		t.Errorf("dev path = %q, want .../runtime-dev.json", devPath)
	}
	if got, err := Read(configDir); err != nil || got == nil || got.Token != "dev" {
		t.Fatalf("dev Read = %+v (err %v), want the dev record", got, err)
	}
	if err := os.Remove(devPath); err != nil {
		t.Fatalf("remove dev path: %v", err)
	}

	t.Setenv("LICH_DEV", "")
	got, err := Read(configDir)
	if err != nil {
		t.Fatalf("Read daily: %v", err)
	}
	if got == nil || got.Port != 47821 || got.Token != "daily" {
		t.Fatalf("daily record = %+v, want port=47821 tok=daily", got)
	}
}

// TestPing exercises the production probe against a listener that mimics the
// transport's token-gated /ping: 204 with the token, 401 without.
func TestPing(t *testing.T) {
	const token = "sekret"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ping" || r.URL.Query().Get("token") != token {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	port := serverPort(t, srv)

	if !Ping(port, token) {
		t.Fatal("Ping with the right token = false, want true")
	}
	if Ping(port, "wrong") {
		t.Fatal("Ping with a bad token = true, want false")
	}
	// A port nothing listens on must read as dead, not hang.
	if Ping(1, token) {
		t.Fatal("Ping unreachable port = true, want false")
	}
}

func TestBindFailureVerdict(t *testing.T) {
	live := &Info{PID: 1, Port: 47821, Token: "t"}
	tests := []struct {
		name        string
		restartWait string
		running     *Info
		want        BindVerdict
	}{
		{"restart successor, nothing detected", "1", nil, BindFailureIsReal},
		// A successor races for a busy port on purpose; whatever still holds it
		// is not a window to hand over to.
		{"restart successor, lich still holding the port", "1", live, BindFailureIsReal},
		{"fresh launch, port held by something else", "", nil, BindFailureIsReal},
		{"fresh launch, another live lich", "", live, BindFailureIsDuplicate},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := BindFailureVerdict(tt.restartWait, tt.running); got != tt.want {
				t.Errorf("BindFailureVerdict(%q, %+v) = %d, want %d",
					tt.restartWait, tt.running, got, tt.want)
			}
		})
	}
}

func serverPort(t *testing.T, srv *httptest.Server) int {
	t.Helper()
	_, portStr, ok := strings.Cut(strings.TrimPrefix(srv.URL, "http://"), ":")
	if !ok {
		t.Fatalf("unexpected server URL %q", srv.URL)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}
	return port
}

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

package cli

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// quittingLich is a running lich that answers system.Quit with answer and
// then, when it quits, lets go of its port the way the real process does on
// exit. It records the call's body so the options object can be checked.
func quittingLich(t *testing.T, answer func(w http.ResponseWriter) (quits bool)) (*httptest.Server, *[]string) {
	t.Helper()
	var bodies []string
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rpc/system.Quit" {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(body))
		if answer(w) {
			// Close waits for this handler, so it cannot run inside it.
			go srv.Close()
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &bodies
}

func runQuit(t *testing.T, srv *httptest.Server) (int, string, string) {
	t.Helper()
	port := strconv.Itoa(srv.Listener.Addr().(*net.TCPAddr).Port)
	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"quit"}, &client{
		env: func(key string) string {
			return map[string]string{"LICH_PORT": port, "LICH_TOKEN": "tok"}[key]
		},
		stdout: &stdout, stderr: &stderr, running: noInstance,
	})
	return code, stdout.String(), stderr.String()
}

func TestQuitReturnsOnceLichIsGone(t *testing.T) {
	srv, bodies := quittingLich(t, func(w http.ResponseWriter) bool {
		_, _ = io.WriteString(w, "null")
		return true
	})
	code, stdout, stderr := runQuit(t, srv)
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(stdout, "lich has quit") {
		t.Errorf("stdout = %q", stdout)
	}
	if len(*bodies) != 1 || (*bodies)[0] != "[{}]" {
		t.Errorf("bodies = %q, want one call with one options object", *bodies)
	}
	if listening(strconv.Itoa(srv.Listener.Addr().(*net.TCPAddr).Port)) {
		t.Error("quit returned while lich still listened")
	}
}

// The process can exit before its reply leaves: the call fails, and lich is
// gone, which is what was asked.
func TestQuitWhoseReplyIsLostToTheExitSucceeds(t *testing.T) {
	srv, _ := quittingLich(t, func(w http.ResponseWriter) bool {
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Errorf("hijack: %v", err)
			return false
		}
		_ = conn.Close()
		return true
	})
	if code, _, stderr := runQuit(t, srv); code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
}

func TestQuitRefusedByARunningLichFails(t *testing.T) {
	srv, _ := quittingLich(t, func(w http.ResponseWriter) bool {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"error":"quit: lich is still starting"}`)
		return false
	})
	code, _, stderr := runQuit(t, srv)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr, "lich is still starting") {
		t.Errorf("stderr = %q, want the refusal", stderr)
	}
}

func TestQuitTakesNoArguments(t *testing.T) {
	f := newFakeLich(t, `null`)
	code, _, stderr := run(t, f, "quit", "now")
	if code != 1 || !strings.Contains(stderr, "usage: lich quit") {
		t.Fatalf("exit = %d, stderr = %q, want the usage line", code, stderr)
	}
	if len(f.calls) != 0 {
		t.Errorf("a malformed quit reached lich: %+v", f.calls)
	}
}

func TestGoneGivesUpOnAPortStillListening(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	if gone(strconv.Itoa(srv.Listener.Addr().(*net.TCPAddr).Port), 3*quitPoll) {
		t.Fatal("gone = true for a port still accepting")
	}
}

package terminal

import (
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/omartelo/lich/internal/restart"
)

func newRestartTransport(t *testing.T) *transport {
	t.Helper()
	tr, err := newTransport(func(string, []byte) {}, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("newTransport: %v", err)
	}
	return tr
}

func TestRestartTriggersCallback(t *testing.T) {
	tr := newRestartTransport(t)
	fired := make(chan struct{}, 1)
	tr.setRestart(func() error { fired <- struct{}{}; return nil })

	url := fmt.Sprintf("http://127.0.0.1:%d/restart?token=%s", tr.port, tr.token)
	resp, err := http.Post(url, "application/json", nil)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
	select {
	case <-fired:
	case <-time.After(5 * time.Second):
		t.Fatal("restart callback never fired")
	}
}

func TestRestartRejectsBadToken(t *testing.T) {
	tr := newRestartTransport(t)
	fired := make(chan struct{}, 1)
	tr.setRestart(func() error { fired <- struct{}{}; return nil })

	url := fmt.Sprintf("http://127.0.0.1:%d/restart?token=wrong", tr.port)
	resp, err := http.Post(url, "application/json", nil)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
	select {
	case <-fired:
		t.Fatal("restart fired despite a bad token")
	case <-time.After(200 * time.Millisecond):
	}
}

func TestRestartRejectsGet(t *testing.T) {
	tr := newRestartTransport(t)
	tr.setRestart(func() error { return nil })

	url := fmt.Sprintf("http://127.0.0.1:%d/restart?token=%s", tr.port, tr.token)
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", resp.StatusCode)
	}
}

func TestRestartUnavailableWhenUnset(t *testing.T) {
	tr := newRestartTransport(t) // restart left nil

	url := fmt.Sprintf("http://127.0.0.1:%d/restart?token=%s", tr.port, tr.token)
	resp, err := http.Post(url, "application/json", nil)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", resp.StatusCode)
	}
}

func TestListenGivesUpAfterBindTimeoutWithoutWaitMarker(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("occupy port: %v", err)
	}
	defer func() { _ = busy.Close() }()

	// No WaitEnv: still retries (a plain relaunch can race the exiting lich's
	// child processes too — see CHANGELOG), but on the shorter bindTimeout
	// budget, and gives up once that runs out.
	start := time.Now()
	if l, err := listen(busy.Addr().String()); err == nil {
		_ = l.Close()
		t.Fatal("listen() = nil error on a port that stayed busy, want failure once bindTimeout elapses")
	}
	if elapsed := time.Since(start); elapsed < bindTimeout {
		t.Fatalf("listen() gave up after %s, want it to retry for at least bindTimeout (%s)", elapsed, bindTimeout)
	}
}

func TestListenBailsOutFastOnUnretryableError(t *testing.T) {
	// An out-of-range port can never free itself; retrying it only delays the
	// real error (seen in the field as LICH_LISTEN_PORT=321654 burning the
	// whole bindTimeout before reporting "invalid port").
	start := time.Now()
	l, err := listen("127.0.0.1:321654")
	if err == nil {
		_ = l.Close()
		t.Fatal("listen() = nil error on an invalid port, want immediate failure")
	}
	if elapsed := time.Since(start); elapsed >= bindRetryInterval {
		t.Fatalf("listen() took %s on an unretryable error, want a bail-out before the first retry (%s)", elapsed, bindRetryInterval)
	}
}

func TestListenRetriesUntilPortFreesWithoutWaitMarker(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("occupy port: %v", err)
	}
	addr := busy.Addr().String()

	type result struct {
		l   net.Listener
		err error
	}
	done := make(chan result, 1)
	go func() {
		l, err := listen(addr)
		done <- result{l, err}
	}()

	// Free the port well inside the bindTimeout budget; listen must then bind
	// without LICH_RESTART_WAIT set — this is the ordinary relaunch path.
	time.Sleep(2 * bindRetryInterval)
	_ = busy.Close()

	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("listen() = %v, want a successful bind after the port freed", r.err)
		}
		_ = r.l.Close()
	case <-time.After(bindTimeout):
		t.Fatal("listen() never bound after the port was freed")
	}
}

func TestListenRetriesUntilPortFreesWithWaitMarker(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("occupy port: %v", err)
	}
	addr := busy.Addr().String()
	t.Setenv(restart.WaitEnv, "1")

	type result struct {
		l   net.Listener
		err error
	}
	done := make(chan result, 1)
	go func() {
		l, err := listen(addr)
		done <- result{l, err}
	}()

	// Free the port after a couple of retry intervals; listen must then bind.
	time.Sleep(2 * bindRetryInterval)
	_ = busy.Close()

	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("listen() = %v, want a successful bind after the port freed", r.err)
		}
		_ = r.l.Close()
	case <-time.After(restartBindTimeout):
		t.Fatal("listen() never bound after the port was freed")
	}
}

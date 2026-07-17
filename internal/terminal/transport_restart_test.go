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

func TestListenBindsOnceWithoutWaitMarker(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("occupy port: %v", err)
	}
	defer func() { _ = busy.Close() }()

	// No WaitEnv: a busy address fails immediately rather than retrying.
	if l, err := listen(busy.Addr().String()); err == nil {
		_ = l.Close()
		t.Fatal("listen() = nil error on a busy port, want immediate failure")
	}
}

func TestListenRetriesUntilPortFrees(t *testing.T) {
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
	time.Sleep(2 * restartBindInterval)
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

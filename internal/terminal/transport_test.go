package terminal

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestFrameRoundTrip(t *testing.T) {
	frame, err := encodeFrame("sess-1", []byte("hello"))
	if err != nil {
		t.Fatalf("encodeFrame: %v", err)
	}
	id, payload, err := decodeFrame(frame)
	if err != nil {
		t.Fatalf("decodeFrame: %v", err)
	}
	if id != "sess-1" || !bytes.Equal(payload, []byte("hello")) {
		t.Fatalf("got id=%q payload=%q", id, payload)
	}
}

func TestFrameEmptyPayload(t *testing.T) {
	frame, err := encodeFrame("s", nil)
	if err != nil {
		t.Fatalf("encodeFrame: %v", err)
	}
	id, payload, err := decodeFrame(frame)
	if err != nil {
		t.Fatalf("decodeFrame: %v", err)
	}
	if id != "s" || len(payload) != 0 {
		t.Fatalf("got id=%q payload=%q", id, payload)
	}
}

func TestFrameErrors(t *testing.T) {
	if _, err := encodeFrame("", []byte("x")); err == nil {
		t.Fatal("expected error for empty id")
	}
	long := make([]byte, 256)
	if _, err := encodeFrame(string(long), nil); err == nil {
		t.Fatal("expected error for oversized id")
	}
	if _, _, err := decodeFrame(nil); err == nil {
		t.Fatal("expected error for empty frame")
	}
	if _, _, err := decodeFrame([]byte{10, 'a'}); err == nil {
		t.Fatal("expected error for truncated frame")
	}
	if _, _, err := decodeFrame([]byte{0, 'a'}); err == nil {
		t.Fatal("expected error for zero id length")
	}
}

// dial connects a test client to the transport with the given token.
func dial(t *testing.T, tr *transport, token string) (*websocket.Conn, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, resp, err := websocket.Dial(ctx, fmt.Sprintf("ws://127.0.0.1:%d/ws?token=%s", tr.port, token), nil)
	if resp != nil && resp.Body != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	return conn, err
}

func TestTransportRejectsBadToken(t *testing.T) {
	tr, err := newTransport(func(string, []byte) {}, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("newTransport: %v", err)
	}
	if _, err := dial(t, tr, "wrong"); err == nil {
		t.Fatal("expected dial to fail with a bad token")
	}
}

func TestTransportInputReachesService(t *testing.T) {
	got := make(chan string, 1)
	tr, err := newTransport(func(id string, data []byte) {
		got <- id + ":" + string(data)
	}, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("newTransport: %v", err)
	}
	conn, err := dial(t, tr, tr.token)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	frame, err := encodeFrame("sess", []byte("ls\r"))
	if err != nil {
		t.Fatalf("encodeFrame: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.Write(ctx, websocket.MessageBinary, frame); err != nil {
		t.Fatalf("write: %v", err)
	}
	select {
	case v := <-got:
		if v != "sess:ls\r" {
			t.Fatalf("got %q", v)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("input frame never reached the service")
	}
}

func TestTransportSendAndFallback(t *testing.T) {
	tr, err := newTransport(func(string, []byte) {}, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("newTransport: %v", err)
	}
	if tr.send("sess", []byte("x")) {
		t.Fatal("send must report false with no client connected")
	}

	conn, err := dial(t, tr, tr.token)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()
	// The server registers the client during the handshake, so send is ready
	// as soon as Dial returns.
	if !tr.send("sess", []byte("output")) {
		t.Fatal("send must succeed with a connected client")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	kind, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if kind != websocket.MessageBinary {
		t.Fatalf("got message type %v", kind)
	}
	id, payload, err := decodeFrame(data)
	if err != nil {
		t.Fatalf("decodeFrame: %v", err)
	}
	if id != "sess" || string(payload) != "output" {
		t.Fatalf("got id=%q payload=%q", id, payload)
	}
}

func TestParseHookRequest(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantID    string
		wantState string
		wantErr   bool
	}{
		{"busy", `{"session_id":"s1","state":"busy"}`, "s1", "busy", false},
		{"done", `{"session_id":"s2","state":"done"}`, "s2", "done", false},
		{"waiting", `{"session_id":"s3","state":"waiting"}`, "s3", "waiting", false},
		{"missing id", `{"state":"busy"}`, "", "", true},
		{"unknown state", `{"session_id":"s1","state":"cooking"}`, "", "", true},
		{"empty state", `{"session_id":"s1"}`, "", "", true},
		{"bad json", `{`, "", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			id, state, err := parseHookRequest([]byte(tc.body))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tc.body)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if id != tc.wantID || state != tc.wantState {
				t.Fatalf("got (%q,%q), want (%q,%q)", id, state, tc.wantID, tc.wantState)
			}
		})
	}
}

func TestHookForwardsStatus(t *testing.T) {
	got := make(chan string, 1)
	tr, err := newTransport(func(string, []byte) {}, func(id, state string) {
		got <- id + ":" + state
	}, nil, nil, nil)
	if err != nil {
		t.Fatalf("newTransport: %v", err)
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/hook?token=%s", tr.port, tr.token)
	resp, err := http.Post(url, "application/json", strings.NewReader(`{"session_id":"sess","state":"busy"}`))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
	select {
	case v := <-got:
		if v != "sess:busy" {
			t.Fatalf("got %q", v)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("status never forwarded to the callback")
	}
}

func TestHookRejectsBadToken(t *testing.T) {
	fired := make(chan struct{}, 1)
	tr, err := newTransport(func(string, []byte) {}, func(string, string) { fired <- struct{}{} }, nil, nil, nil)
	if err != nil {
		t.Fatalf("newTransport: %v", err)
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/hook?token=wrong", tr.port)
	resp, err := http.Post(url, "application/json", strings.NewReader(`{"session_id":"s","state":"busy"}`))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
	select {
	case <-fired:
		t.Fatal("status callback fired despite a bad token")
	case <-time.After(200 * time.Millisecond):
	}
}

func TestParseSessionStart(t *testing.T) {
	tests := []struct {
		name         string
		body         string
		wantID       string
		wantClaudeID string
		wantErr      bool
	}{
		{"ok", `{"session_id":"s1","claude_session_id":"uuid-1"}`, "s1", "uuid-1", false},
		{"missing session id", `{"claude_session_id":"uuid-1"}`, "", "", true},
		{"missing claude id", `{"session_id":"s1"}`, "", "", true},
		{"empty claude id", `{"session_id":"s1","claude_session_id":""}`, "", "", true},
		{"bad json", `{`, "", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			id, claudeID, err := parseSessionStart([]byte(tc.body))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tc.body)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if id != tc.wantID || claudeID != tc.wantClaudeID {
				t.Fatalf("got (%q,%q), want (%q,%q)", id, claudeID, tc.wantID, tc.wantClaudeID)
			}
		})
	}
}

func TestSessionStartLinksSession(t *testing.T) {
	got := make(chan string, 1)
	tr, err := newTransport(func(string, []byte) {}, nil, func(id, claudeID string) error {
		got <- id + ":" + claudeID
		return nil
	}, nil, nil)
	if err != nil {
		t.Fatalf("newTransport: %v", err)
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/session-start?token=%s", tr.port, tr.token)
	resp, err := http.Post(url, "application/json",
		strings.NewReader(`{"session_id":"sess","claude_session_id":"uuid-9"}`))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
	select {
	case v := <-got:
		if v != "sess:uuid-9" {
			t.Fatalf("got %q", v)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("session id never reached the callback")
	}
}

func TestSessionStartRejectsBadToken(t *testing.T) {
	fired := make(chan struct{}, 1)
	tr, err := newTransport(func(string, []byte) {}, nil, func(string, string) error {
		fired <- struct{}{}
		return nil
	}, nil, nil)
	if err != nil {
		t.Fatalf("newTransport: %v", err)
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/session-start?token=wrong", tr.port)
	resp, err := http.Post(url, "application/json",
		strings.NewReader(`{"session_id":"s","claude_session_id":"u"}`))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
	select {
	case <-fired:
		t.Fatal("link callback fired despite a bad token")
	case <-time.After(200 * time.Millisecond):
	}
}

func TestParseSessionTitle(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantID    string
		wantTitle string
		wantErr   bool
	}{
		{"ok", `{"session_id":"s1","title":"Fixing auth"}`, "s1", "Fixing auth", false},
		{"trims", `{"session_id":"s1","title":"  Fixing auth\n"}`, "s1", "Fixing auth", false},
		{"missing session id", `{"title":"x"}`, "", "", true},
		{"missing title", `{"session_id":"s1"}`, "", "", true},
		{"blank title", `{"session_id":"s1","title":"   "}`, "", "", true},
		{"bad json", `{`, "", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			id, title, err := parseSessionTitle([]byte(tc.body))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tc.body)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if id != tc.wantID || title != tc.wantTitle {
				t.Fatalf("got (%q,%q), want (%q,%q)", id, title, tc.wantID, tc.wantTitle)
			}
		})
	}
}

func TestSessionTitleApplies(t *testing.T) {
	got := make(chan string, 1)
	tr, err := newTransport(func(string, []byte) {}, nil, nil, func(id, title string) error {
		got <- id + ":" + title
		return nil
	}, nil)
	if err != nil {
		t.Fatalf("newTransport: %v", err)
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/session-title?token=%s", tr.port, tr.token)
	resp, err := http.Post(url, "application/json",
		strings.NewReader(`{"session_id":"sess","title":"Fixing auth"}`))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
	select {
	case v := <-got:
		if v != "sess:Fixing auth" {
			t.Fatalf("got %q", v)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("title never reached the callback")
	}
}

func TestSessionTitleRejectsBadToken(t *testing.T) {
	fired := make(chan struct{}, 1)
	tr, err := newTransport(func(string, []byte) {}, nil, nil, func(string, string) error {
		fired <- struct{}{}
		return nil
	}, nil)
	if err != nil {
		t.Fatalf("newTransport: %v", err)
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/session-title?token=wrong", tr.port)
	resp, err := http.Post(url, "application/json",
		strings.NewReader(`{"session_id":"s","title":"x"}`))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
	select {
	case <-fired:
		t.Fatal("title callback fired despite a bad token")
	case <-time.After(200 * time.Millisecond):
	}
}

func TestParseSessionTouched(t *testing.T) {
	if id, err := parseSessionTouched([]byte(`{"session_id":"s1"}`)); err != nil || id != "s1" {
		t.Fatalf("got (%q,%v), want (s1,nil)", id, err)
	}
	if _, err := parseSessionTouched([]byte(`{}`)); err == nil {
		t.Fatal("expected error for missing session_id")
	}
	if _, err := parseSessionTouched([]byte(`{`)); err == nil {
		t.Fatal("expected error for bad json")
	}
}

func TestSessionTouchedForwards(t *testing.T) {
	got := make(chan string, 1)
	tr, err := newTransport(func(string, []byte) {}, nil, nil, nil, func(id string) {
		got <- id
	})
	if err != nil {
		t.Fatalf("newTransport: %v", err)
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/session-touched?token=%s", tr.port, tr.token)
	resp, err := http.Post(url, "application/json", strings.NewReader(`{"session_id":"sess"}`))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
	select {
	case v := <-got:
		if v != "sess" {
			t.Fatalf("got %q", v)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("touched id never reached the callback")
	}
}

func TestSessionTouchedRejectsBadToken(t *testing.T) {
	fired := make(chan struct{}, 1)
	tr, err := newTransport(func(string, []byte) {}, nil, nil, nil, func(string) {
		fired <- struct{}{}
	})
	if err != nil {
		t.Fatalf("newTransport: %v", err)
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/session-touched?token=wrong", tr.port)
	resp, err := http.Post(url, "application/json", strings.NewReader(`{"session_id":"s"}`))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
	select {
	case <-fired:
		t.Fatal("touched callback fired despite a bad token")
	case <-time.After(200 * time.Millisecond):
	}
}

func TestTransportInfoZeroWithoutTransport(t *testing.T) {
	s := &Service{}
	if info := s.Transport(); info.Port != 0 || info.Token != "" {
		t.Fatalf("expected zero TransportInfo, got %+v", info)
	}
}

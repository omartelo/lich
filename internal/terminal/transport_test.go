package terminal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/omartelo/lich/internal/events"
)

// TestMain drops LICH_LISTEN_PORT so every transport in this package binds a
// random port: a test run spawned from inside a lich terminal inherits the
// pinned port of the running instance and would collide with it.
func TestMain(m *testing.M) {
	os.Unsetenv("LICH_LISTEN_PORT")
	os.Exit(m.Run())
}

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

// sendWhenRegistered retries send until it reports a connected client, and says
// whether it ever did.
//
// Dial returning only means the client read the handshake response: the server
// registers the connection after websocket.Accept returns, in its own handler
// goroutine, so send can still find no client for a moment. Production wants
// exactly that — an unregistered client makes send report false and the output
// falls back to the /events bridge — which leaves the window unobservable from
// out here, and asserting on the first send is a coin flip. Frames dropped
// before registration are the same fallback, so only the send that lands is
// delivered to the client.
func sendWhenRegistered(t *testing.T, tr *transport, id string, data []byte) bool {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if tr.send(id, data) {
			return true
		}
		time.Sleep(time.Millisecond)
	}
	return false
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
	if !sendWhenRegistered(t, tr, "sess", []byte("output")) {
		t.Fatal("send never saw the connected client")
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
		{"idle", `{"session_id":"s4","state":"idle"}`, "s4", "idle", false},
		{"missing id", `{"state":"busy"}`, "", "", true},
		{"unknown state", `{"session_id":"s1","state":"cooking"}`, "", "", true},
		{"empty state", `{"session_id":"s1"}`, "", "", true},
		{"bad json", `{`, "", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, err := parseHookRequest([]byte(tc.body))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tc.body)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if req.SessionID != tc.wantID || req.State != tc.wantState {
				t.Fatalf("got (%q,%q), want (%q,%q)", req.SessionID, req.State, tc.wantID, tc.wantState)
			}
		})
	}
}

// The cap is pinned as a literal rather than read off hookTextLimit: a test
// that derives its input from the constant passes whatever the constant becomes,
// which is exactly the mutation it exists to catch.
func TestParseHookRequestCapsToolText(t *testing.T) {
	tests := []struct {
		name       string
		tool       string
		wantLength int
	}{
		{"at the cap", strings.Repeat("x", 120), 120},
		{"one past the cap", strings.Repeat("x", 121), 120},
		{"far past the cap", strings.Repeat("x", 5000), 120},
		// Runes, not bytes: a cut between the two bytes of "é" would produce a
		// name the page renders as a replacement character.
		{"multi-byte runes", strings.Repeat("é", 200), 120},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body, err := json.Marshal(hookRequest{
				SessionID: "s1",
				State:     "busy",
				Tool:      tc.tool,
				Detail:    tc.tool,
			})
			if err != nil {
				t.Fatalf("marshal body: %v", err)
			}
			req, err := parseHookRequest(body)
			if err != nil {
				t.Fatalf("parseHookRequest: %v", err)
			}
			if got := len([]rune(req.Tool)); got != tc.wantLength {
				t.Errorf("tool is %d runes, want %d", got, tc.wantLength)
			}
			if got := len([]rune(req.Detail)); got != tc.wantLength {
				t.Errorf("detail is %d runes, want %d", got, tc.wantLength)
			}
		})
	}
}

func TestPingAnswersWithToken(t *testing.T) {
	tr, err := newTransport(func(string, []byte) {}, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("newTransport: %v", err)
	}
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/ping?token=%s", tr.port, tr.token))
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
}

func TestPingRejectsBadToken(t *testing.T) {
	tr, err := newTransport(func(string, []byte) {}, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("newTransport: %v", err)
	}
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/ping?token=wrong", tr.port))
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestGoroutineDumpAnswersWithTheStacks(t *testing.T) {
	tr, err := newTransport(func(string, []byte) {}, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("newTransport: %v", err)
	}
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/debug/goroutines?token=%s", tr.port, tr.token))
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	// The dump this very request is served by is in there, stack and all: the
	// hang it exists for is read off the frames, not off the count.
	if !strings.Contains(string(body), "terminal.(*transport).goroutines") ||
		!strings.Contains(string(body), "TestGoroutineDumpAnswersWithTheStacks") {
		t.Errorf("dump carries no stacks: %q", string(body))
	}
	// The leak profile follows under its heading. A healthy process leaks
	// nothing, so this pins that the section is written at all — the count it
	// reports is the finding, and an empty one is the answer here.
	_, leaks, found := strings.CutLast(string(body), leakHeading)
	if !found {
		t.Fatalf("dump carries no leak section: %q", string(body))
	}
	if !strings.HasPrefix(leaks, "goroutineleak profile: total ") {
		t.Errorf("leak section = %q, want a goroutineleak profile", leaks)
	}
}

func TestGoroutineDumpRejectsBadToken(t *testing.T) {
	tr, err := newTransport(func(string, []byte) {}, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("newTransport: %v", err)
	}
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/debug/goroutines?token=wrong", tr.port))
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestHookForwardsStatus(t *testing.T) {
	got := make(chan hookRequest, 1)
	tr, err := newTransport(func(string, []byte) {}, func(req hookRequest) {
		got <- req
	}, nil, nil, nil)
	if err != nil {
		t.Fatalf("newTransport: %v", err)
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/hook?token=%s", tr.port, tr.token)
	body := `{"session_id":"sess","state":"busy","tool":"Bash","detail":"pnpm test"}`
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
	select {
	case req := <-got:
		want := hookRequest{SessionID: "sess", State: "busy", Tool: "Bash", Detail: "pnpm test"}
		if req != want {
			t.Fatalf("got %#v, want %#v", req, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("status never forwarded to the callback")
	}
}

func TestHookRejectsBadToken(t *testing.T) {
	fired := make(chan struct{}, 1)
	tr, err := newTransport(func(string, []byte) {}, func(hookRequest) { fired <- struct{}{} }, nil, nil, nil)
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
		name           string
		body           string
		wantID         string
		wantProviderID string
		wantProvider   string
		wantErr        bool
	}{
		{"ok", `{"session_id":"s1","provider_session_id":"uuid-1"}`, "s1", "uuid-1", "claude", false},
		{"legacy claude field", `{"session_id":"s1","claude_session_id":"uuid-2"}`, "s1", "uuid-2", "claude", false},
		{"new field wins over legacy",
			`{"session_id":"s1","provider_session_id":"uuid-1","claude_session_id":"uuid-2"}`,
			"s1", "uuid-1", "claude", false},
		{"reported provider",
			`{"session_id":"s1","provider_session_id":"uuid-1","provider":"codex"}`,
			"s1", "uuid-1", "codex", false},
		{"missing session id", `{"provider_session_id":"uuid-1"}`, "", "", "", true},
		{"missing provider id", `{"session_id":"s1"}`, "", "", "", true},
		{"empty provider id", `{"session_id":"s1","provider_session_id":""}`, "", "", "", true},
		{"unknown provider",
			`{"session_id":"s1","provider_session_id":"uuid-1","provider":"gemini"}`,
			"", "", "", true},
		{"bad json", `{`, "", "", "", true},
		// A provider session id is pasted into a filesystem glob to find that
		// conversation's transcript, so an id that is not an opaque token is
		// refused here rather than reaching it: a separator climbs out of the
		// harness directory, and a metacharacter widens the pattern onto
		// somebody else's conversation.
		{"traversing provider id",
			`{"session_id":"s1","provider_session_id":"../../../../etc/passwd"}`,
			"", "", "", true},
		{"windows traversing provider id",
			`{"session_id":"s1","provider_session_id":"..\\..\\secrets"}`,
			"", "", "", true},
		{"wildcard provider id", `{"session_id":"s1","provider_session_id":"*"}`, "", "", "", true},
		{"character class provider id",
			`{"session_id":"s1","provider_session_id":"uuid-[ab]"}`,
			"", "", "", true},
		{"question mark provider id",
			`{"session_id":"s1","provider_session_id":"uuid-?"}`,
			"", "", "", true},
		{"legacy field is checked too",
			`{"session_id":"s1","claude_session_id":"../../etc/passwd"}`,
			"", "", "", true},
		{"a dot inside an id is not a traversal",
			`{"session_id":"s1","provider_session_id":"ses.2026-08-17.a1b2"}`,
			"s1", "ses.2026-08-17.a1b2", "claude", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, err := parseSessionStart([]byte(tc.body))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tc.body)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if req.SessionID != tc.wantID || req.ProviderSessionID != tc.wantProviderID ||
				req.Provider != tc.wantProvider {
				t.Fatalf("got (%q,%q,%q), want (%q,%q,%q)",
					req.SessionID, req.ProviderSessionID, req.Provider,
					tc.wantID, tc.wantProviderID, tc.wantProvider)
			}
		})
	}
}

func TestSessionStartLinksSession(t *testing.T) {
	got := make(chan string, 1)
	tr, err := newTransport(func(string, []byte) {}, nil, func(id, providerID, provider string) error {
		got <- id + ":" + providerID + ":" + provider
		return nil
	}, nil, nil)
	if err != nil {
		t.Fatalf("newTransport: %v", err)
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/session-start?token=%s", tr.port, tr.token)
	resp, err := http.Post(url, "application/json",
		strings.NewReader(`{"session_id":"sess","provider_session_id":"uuid-9","provider":"codex"}`))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
	select {
	case v := <-got:
		if v != "sess:uuid-9:codex" {
			t.Fatalf("got %q", v)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("session id never reached the callback")
	}
}

func TestSessionStartRejectsBadToken(t *testing.T) {
	fired := make(chan struct{}, 1)
	tr, err := newTransport(func(string, []byte) {}, nil, func(string, string, string) error {
		fired <- struct{}{}
		return nil
	}, nil, nil)
	if err != nil {
		t.Fatalf("newTransport: %v", err)
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/session-start?token=wrong", tr.port)
	resp, err := http.Post(url, "application/json",
		strings.NewReader(`{"session_id":"s","provider_session_id":"u"}`))
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
			req, err := parseSessionTitle([]byte(tc.body))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tc.body)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if req.SessionID != tc.wantID || req.Title != tc.wantTitle {
				t.Fatalf("got (%q,%q), want (%q,%q)", req.SessionID, req.Title, tc.wantID, tc.wantTitle)
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
	if req, err := parseSessionTouched([]byte(`{"session_id":"s1"}`)); err != nil || req.SessionID != "s1" {
		t.Fatalf("got (%q,%v), want (s1,nil)", req.SessionID, err)
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

// hookEndpoints is every hook endpoint paired with a body that parses cleanly.
// They all share servePost, so the failure modes it owns are proved once across
// the whole set instead of once per handler.
var hookEndpoints = []struct {
	path string
	body string
}{
	{"/hook", `{"session_id":"s","state":"busy"}`},
	{"/session-start", `{"session_id":"s","provider_session_id":"u"}`},
	{"/session-title", `{"session_id":"s","title":"t"}`},
	{"/session-touched", `{"session_id":"s"}`},
}

// newNilTransport starts a transport with every hook callback unset, the state
// the app is in when a POST lands before the callbacks matter.
func newNilTransport(t *testing.T) *transport {
	t.Helper()
	tr, err := newTransport(func(string, []byte) {}, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("newTransport: %v", err)
	}
	return tr
}

func TestHookEndpointsRejectNonPost(t *testing.T) {
	tr := newNilTransport(t)
	for _, e := range hookEndpoints {
		t.Run(e.path, func(t *testing.T) {
			resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d%s?token=%s", tr.port, e.path, tr.token))
			if err != nil {
				t.Fatalf("get: %v", err)
			}
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want 405", resp.StatusCode)
			}
		})
	}
}

func TestHookEndpointsRejectInvalidJSON(t *testing.T) {
	tr := newNilTransport(t)
	for _, e := range hookEndpoints {
		t.Run(e.path, func(t *testing.T) {
			url := fmt.Sprintf("http://127.0.0.1:%d%s?token=%s", tr.port, e.path, tr.token)
			resp, err := http.Post(url, "application/json", strings.NewReader(`{`))
			if err != nil {
				t.Fatalf("post: %v", err)
			}
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", resp.StatusCode)
			}
		})
	}
}

// TestHookEndpointsWithNilCallbacks proves a valid POST is still accepted when
// nothing is wired to receive it — the hook must never see an error for a
// report lich simply has no use for.
func TestHookEndpointsWithNilCallbacks(t *testing.T) {
	tr := newNilTransport(t)
	for _, e := range hookEndpoints {
		t.Run(e.path, func(t *testing.T) {
			url := fmt.Sprintf("http://127.0.0.1:%d%s?token=%s", tr.port, e.path, tr.token)
			resp, err := http.Post(url, "application/json", strings.NewReader(e.body))
			if err != nil {
				t.Fatalf("post: %v", err)
			}
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusNoContent {
				t.Fatalf("status = %d, want 204", resp.StatusCode)
			}
		})
	}
}

// TestStoreFailuresReport500 proves a persistence error reaches the response
// rather than being swallowed. These are the only two hooks that write, and the
// 500 is the sole signal that the write was lost.
func TestStoreFailuresReport500(t *testing.T) {
	boom := errors.New("store is down")
	tests := []struct {
		name string
		path string
		body string
		tr   func() (*transport, error)
	}{
		{
			name: "session-start link failure",
			path: "/session-start",
			body: `{"session_id":"s","provider_session_id":"u"}`,
			tr: func() (*transport, error) {
				return newTransport(func(string, []byte) {}, nil,
					func(string, string, string) error { return boom }, nil, nil)
			},
		},
		{
			name: "session-title store failure",
			path: "/session-title",
			body: `{"session_id":"s","title":"t"}`,
			tr: func() (*transport, error) {
				return newTransport(func(string, []byte) {}, nil, nil,
					func(string, string) error { return boom }, nil)
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tr, err := tc.tr()
			if err != nil {
				t.Fatalf("newTransport: %v", err)
			}
			url := fmt.Sprintf("http://127.0.0.1:%d%s?token=%s", tr.port, tc.path, tr.token)
			resp, err := http.Post(url, "application/json", strings.NewReader(tc.body))
			if err != nil {
				t.Fatalf("post: %v", err)
			}
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusInternalServerError {
				t.Fatalf("status = %d, want 500", resp.StatusCode)
			}
		})
	}
}

// TestHookBodyLimit proves an oversized body is truncated rather than read into
// memory: the JSON is valid but padded past hookBodyLimit, so the cut lands
// mid-document and the parse fails. Without the limit this would be a 204.
func TestHookBodyLimit(t *testing.T) {
	fired := make(chan struct{}, 1)
	tr, err := newTransport(func(string, []byte) {}, func(hookRequest) { fired <- struct{}{} }, nil, nil, nil)
	if err != nil {
		t.Fatalf("newTransport: %v", err)
	}
	padded := fmt.Sprintf(`{"pad":%q,"session_id":"s","state":"busy"}`, strings.Repeat("x", hookBodyLimit))
	url := fmt.Sprintf("http://127.0.0.1:%d/hook?token=%s", tr.port, tr.token)
	resp, err := http.Post(url, "application/json", strings.NewReader(padded))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body should be cut mid-JSON)", resp.StatusCode)
	}
	select {
	case <-fired:
		t.Fatal("status callback fired on an oversized body")
	case <-time.After(200 * time.Millisecond):
	}
}

func TestTransportInfoZeroWithoutTransport(t *testing.T) {
	s := &Service{}
	if info := s.Transport(); info.Port != 0 || info.Token != "" {
		t.Fatalf("expected zero TransportInfo, got %+v", info)
	}
}

// A launch that cannot bind the pinned port has the OS error as its only
// diagnosis, so New must keep it rather than only reporting the absent port.
func TestTransportErrorKeepsBindFailure(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("occupy a port: %v", err)
	}
	defer busy.Close()
	t.Setenv("LICH_LISTEN_PORT", strconv.Itoa(busy.Addr().(*net.TCPAddr).Port))

	// A nil store, not the suite's stub: that one is Unix-only, and this test
	// earns its keep on Windows. Nothing here spawns a session, so the store is
	// never reached.
	s := New(nil, nil, events.New())
	if info := s.Transport(); info.Port != 0 {
		t.Fatalf("transport bound %d, want the busy port to refuse it", info.Port)
	}
	if s.TransportError() == nil {
		t.Fatal("TransportError is nil after a failed bind")
	}
}

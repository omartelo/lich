package lichclient

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

// fakeBackend answers /rpc/ the way internal/rpc does: positional JSON args
// in, the result or {"error"} out.
func fakeBackend(t *testing.T, handle func(method string, args []json.RawMessage) (any, int)) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("token") != "tok" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		var args []json.RawMessage
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &args); err != nil {
			t.Errorf("body %q is not a JSON array: %v", body, err)
		}
		result, status := handle(strings.TrimPrefix(r.URL.Path, "/rpc/"), args)
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(result)
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	return New(Runtime{Port: port, Token: "tok"})
}

func TestCallSendsPositionalArgsAndDecodes(t *testing.T) {
	c := fakeBackend(t, func(method string, args []json.RawMessage) (any, int) {
		if method != "project.Diff" || len(args) != 1 || string(args[0]) != `"/repo"` {
			t.Errorf("got %s %s", method, args)
		}
		return DiffStats{Files: 2, Branch: "main"}, http.StatusOK
	})
	d, err := c.Diff(context.Background(), "/repo")
	if err != nil || d.Files != 2 || d.Branch != "main" {
		t.Fatalf("got %+v, %v", d, err)
	}
}

func TestCallWithNoArgsSendsEmptyArray(t *testing.T) {
	c := fakeBackend(t, func(_ string, args []json.RawMessage) (any, int) {
		if args == nil {
			t.Error("args decoded as null, want []")
		}
		return nil, http.StatusOK
	})
	if _, err := c.LoadState(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestCallReturnsBackendError(t *testing.T) {
	c := fakeBackend(t, func(string, []json.RawMessage) (any, int) {
		return map[string]string{"error": "want 2 arguments, got 1"}, http.StatusBadRequest
	})
	err := c.Call(context.Background(), "store.RenameSession", nil, "id")
	if err == nil || !strings.Contains(err.Error(), "want 2 arguments, got 1") {
		t.Fatalf("got %v", err)
	}
}

func TestFrameRoundTrip(t *testing.T) {
	frame, err := encodeFrame("abc", []byte("ls\r"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(frame, []byte("\x03abcls\r")) {
		t.Fatalf("encoded %q", frame)
	}
	id, payload, err := decodeFrame(frame)
	if err != nil || id != "abc" || string(payload) != "ls\r" {
		t.Fatalf("decoded %q %q %v", id, payload, err)
	}
	for _, bad := range [][]byte{nil, {0}, {5, 'a'}} {
		if _, _, err := decodeFrame(bad); err == nil {
			t.Errorf("decodeFrame(%q) accepted", bad)
		}
	}
	if _, err := encodeFrame("", nil); err == nil {
		t.Error("empty id accepted")
	}
}

func TestRevertLinesSendsTheLinesAsTheBackendNamesThem(t *testing.T) {
	c := fakeBackend(t, func(method string, args []json.RawMessage) (any, int) {
		want := `[{"side":"old","line":4,"text":"b"},{"side":"new","line":4,"text":"B"}]`
		if method != "project.RevertLines" || len(args) != 3 || string(args[1]) != `"main.go"` || string(args[2]) != want {
			t.Errorf("got %s %s", method, args)
		}
		return RevertResult{Patch: "p"}, http.StatusOK
	})
	r, err := c.RevertLines(context.Background(), "/repo", "main.go", []RevertLine{{"old", 4, "b"}, {"new", 4, "B"}})
	if err != nil || r.Patch != "p" {
		t.Fatalf("got %+v, %v", r, err)
	}
}

func TestFileLinesSendsRefAndRange(t *testing.T) {
	c := fakeBackend(t, func(method string, args []json.RawMessage) (any, int) {
		if method != "project.FileLines" || len(args) != 5 || string(args[2]) != `""` || string(args[3]) != "3" || string(args[4]) != "4" {
			t.Errorf("got %s %s", method, args)
		}
		return []string{"x", "y"}, http.StatusOK
	})
	lines, err := c.FileLines(context.Background(), "/repo", "main.go", "", 3, 4)
	if err != nil || len(lines) != 2 {
		t.Fatalf("got %q, %v", lines, err)
	}
}

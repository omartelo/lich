package ghrelease

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseTag(t *testing.T) {
	tests := []struct {
		name, json, want string
	}{
		{"v prefix", `{"tag_name":"v0.2.0"}`, "0.2.0"},
		{"no prefix", `{"tag_name":"1.4.2"}`, "1.4.2"},
		{"pre-release", `{"tag_name":"v0.2.0-rc.3"}`, "0.2.0-rc.3"},
		{"missing", `{"name":"x"}`, ""},
		{"malformed", `not json`, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseTag([]byte(tc.json)); got != tc.want {
				t.Fatalf("parseTag() = %q, want %q", got, tc.want)
			}
		})
	}
}

// serveBody starts a test server returning status/body.
func serveBody(t *testing.T, status int, body string) (*http.Client, string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv.Client(), srv.URL
}

func TestLatestTag(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body, want string
	}{
		{"tag with v prefix", http.StatusOK, `{"tag_name":"v0.2.0"}`, "0.2.0"},
		{"pre-release tag", http.StatusOK, `{"tag_name":"v0.2.0-rc.3"}`, "0.2.0-rc.3"},
		{"malformed json", http.StatusOK, `{"tag_name":`, ""},
		{"not found", http.StatusNotFound, `{"message":"Not Found"}`, ""},
		{"server error", http.StatusInternalServerError, ``, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client, url := serveBody(t, tc.status, tc.body)
			if got := LatestTag(client, url); got != tc.want {
				t.Fatalf("LatestTag() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestLatestTagNetworkFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	client := srv.Client()
	url := srv.URL
	srv.Close()

	if got := LatestTag(client, url); got != "" {
		t.Fatalf("LatestTag() = %q, want %q for an unreachable server", got, "")
	}
}

func TestLatestTagCapsBody(t *testing.T) {
	// The tag is valid but padded past bodyLimit: the cap truncates the body
	// mid-JSON, so a parse failure here proves the read is bounded.
	body := `{"tag_name":"v9.9.9","body":"` + strings.Repeat("x", bodyLimit*2) + `"}`
	client, url := serveBody(t, http.StatusOK, body)
	if got := LatestTag(client, url); got != "" {
		t.Fatalf("LatestTag() = %q, want %q — body read past bodyLimit", got, "")
	}
}

func TestLatestTagBadURL(t *testing.T) {
	if got := LatestTag(&http.Client{}, "://not a url"); got != "" {
		t.Fatalf("LatestTag() = %q, want %q for an unbuildable request", got, "")
	}
}

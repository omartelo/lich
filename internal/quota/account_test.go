package quota

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// credsDir writes a Claude credentials file into a fresh directory and returns
// the directory — the shape a session's own CLAUDE_CONFIG_DIR points at.
func credsDir(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".credentials.json"), []byte(body), 0o600); err != nil {
		t.Fatalf("write creds: %v", err)
	}
	return dir
}

// probeHeaders is what a message response carries for an account 5% into its
// session window and 1% into its weekly one.
var probeHeaders = map[string]string{
	"anthropic-ratelimit-unified-5h-utilization": "0.05",
	"anthropic-ratelimit-unified-5h-reset":       "1789318316",
	"anthropic-ratelimit-unified-7d-utilization": "0.01",
	"anthropic-ratelimit-unified-7d-reset":       "1789318316",
}

// probeCall is the request the probe made, filled in by serveProbe's handler.
type probeCall struct {
	header http.Header
	method string
	body   string
}

// serveProbe answers with the given rate-limit headers and an empty message,
// recording the request the probe made.
func serveProbe(t *testing.T, headers map[string]string) (url string, call *probeCall) {
	t.Helper()
	made := &probeCall{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		made.body, made.header, made.method = string(raw), r.Header.Clone(), r.Method
		for k, v := range headers {
			w.Header().Set(k, v)
		}
		_, _ = w.Write([]byte(`{"id":"msg_x"}`))
	}))
	t.Cleanup(server.Close)
	return server.URL, made
}

func TestASessionConfigDirWinsOverLichOwn(t *testing.T) {
	writeCreds(t, claudeCredsJSON, "")
	session := credsDir(t, `{"claudeAiOauth":{"accessToken":"tok-session","subscriptionType":"pro"}}`)
	var auth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(claudeLimitsBody))
	}))
	defer server.Close()

	got := newService(server.URL, "", time.Now()).claudePlan(Account{
		Env:  map[string]string{claudeDirVar: session},
		Read: true,
	})

	if auth != "Bearer tok-session" {
		t.Errorf("Authorization = %q, want the session's own login", auth)
	}
	if got.Plan != "Pro" {
		t.Errorf("plan = %q, want the session account's plan", got.Plan)
	}
}

func TestACodexHomeInTheSessionWinsOverLichOwn(t *testing.T) {
	writeCreds(t, "", codexCredsJSON)
	session := t.TempDir()
	if err := os.WriteFile(filepath.Join(session, "auth.json"),
		[]byte(`{"tokens":{"access_token":"tok-session"}}`), 0o600); err != nil {
		t.Fatalf("write creds: %v", err)
	}
	var auth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"plan_type":"pro","rate_limit":{"primary_window":
			{"used_percent":9,"limit_window_seconds":18000}}}`))
	}))
	defer server.Close()

	newService("", server.URL, time.Now()).codexPlan(Account{
		Env:  map[string]string{codexHomeVar: session},
		Read: true,
	})

	if auth != "Bearer tok-session" {
		t.Errorf("Authorization = %q, want the session's own login", auth)
	}
}

func TestATokenOnlySessionIsMeasuredByTheProbe(t *testing.T) {
	writeCreds(t, claudeCredsJSON, "")
	url, call := serveProbe(t, probeHeaders)

	got := newService(url, "", time.Now()).claudePlan(Account{
		Env:  map[string]string{claudeTokenVar: "oat-token"},
		Read: true,
	})

	if got.Status != StatusOK {
		t.Fatalf("status = %q, want %q", got.Status, StatusOK)
	}
	want := []Window{
		{Label: "Session", Seconds: 18000, Percent: 5, ResetsAt: "2026-09-13T16:51:56Z"},
		{Label: "Weekly", Seconds: 604800, Percent: 1, ResetsAt: "2026-09-13T16:51:56Z"},
	}
	if len(got.Windows) != 2 || got.Windows[0] != want[0] || got.Windows[1] != want[1] {
		t.Errorf("windows = %+v, want %+v", got.Windows, want)
	}
	// The credentials file on disk belongs to another account and must not be
	// what was measured.
	if auth := call.header.Get("Authorization"); auth != "Bearer oat-token" {
		t.Errorf("Authorization = %q, want the session's own token", auth)
	}
	if call.method != http.MethodPost {
		t.Errorf("method = %q, want POST", call.method)
	}
	// The API rejects an OAuth token that arrives without Claude Code's system
	// prompt, so the probe is not a request anyone may trim.
	if call.body != claudeProbeBody {
		t.Errorf("body = %s, want the probe request verbatim", call.body)
	}
	for header, want := range map[string]string{
		"anthropic-beta":    claudeBetaHeader,
		"anthropic-version": claudeAPIVersion,
		"content-type":      "application/json",
	} {
		if got := call.header.Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
}

func TestTheProbeReadsWhicheverWindowsTheHeadersCarry(t *testing.T) {
	writeCreds(t, claudeCredsJSON, "")
	url, _ := serveProbe(t, map[string]string{
		"anthropic-ratelimit-unified-5h-utilization": "0.4",
	})

	got := newService(url, "", time.Now()).claudePlan(Account{
		Env:  map[string]string{claudeTokenVar: "oat-token"},
		Read: true,
	})

	want := Window{Label: "Session", Seconds: 18000, Percent: 40}
	if len(got.Windows) != 1 || got.Windows[0] != want {
		t.Errorf("windows = %+v, want only %+v", got.Windows, want)
	}
}

func TestAProbeWithoutRateLimitHeadersFails(t *testing.T) {
	writeCreds(t, claudeCredsJSON, "")
	url, _ := serveProbe(t, nil)

	got := newService(url, "", time.Now()).claudePlan(Account{
		Env:  map[string]string{claudeTokenVar: "oat-token"},
		Read: true,
	})

	if got.Status != StatusError {
		t.Errorf("status = %q, want %q", got.Status, StatusError)
	}
}

func TestARejectedProbeTokenReadsAsSignedOut(t *testing.T) {
	writeCreds(t, claudeCredsJSON, "")
	url, _ := serve(t, http.StatusUnauthorized, `{}`)

	got := newService(url, "", time.Now()).claudePlan(Account{
		Env:  map[string]string{claudeTokenVar: "expired"},
		Read: true,
	})

	if got.Status != StatusSignedOut {
		t.Errorf("status = %q, want %q", got.Status, StatusSignedOut)
	}
}

func TestACustomBinaryWithNoReadableEnvironmentIsUnknown(t *testing.T) {
	writeCreds(t, claudeCredsJSON, codexCredsJSON)
	claudeURL, claudeCalls := serve(t, http.StatusOK, claudeLimitsBody)
	codexURL, codexCalls := serve(t, http.StatusOK, `{"plan_type":"pro"}`)
	s := newService(claudeURL, codexURL, time.Now())
	s.SetSessions(func(string) Account { return Account{Custom: true} })

	for _, got := range s.Plans("session-1") {
		if got.Status != StatusUnknown {
			t.Errorf("%s status = %q, want %q", got.Provider, got.Status, StatusUnknown)
		}
		if len(got.Windows) != 0 {
			t.Errorf("%s windows = %+v, want none", got.Provider, got.Windows)
		}
	}
	if *claudeCalls != 0 || *codexCalls != 0 {
		t.Errorf("calls = %d/%d, want none: there is no account to ask about", *claudeCalls, *codexCalls)
	}
}

func TestAReadableSessionWithoutOverridesReadsTheDefaultLogin(t *testing.T) {
	writeCreds(t, claudeCredsJSON, "")
	url, calls := serve(t, http.StatusOK, claudeLimitsBody)
	s := newService(url, "", time.Now())
	// A wrapper that changes nothing about the login — a PATH tweak, a
	// profiler — leaves the session spending exactly what lich reads.
	s.SetSessions(func(string) Account {
		return Account{Env: map[string]string{"PATH": "/opt/bin"}, Custom: true, Read: true}
	})

	got := s.Plans("session-1")[0]

	if got.Status != StatusOK || *calls != 1 {
		t.Errorf("plan = %+v after %d calls, want the default login read", got, *calls)
	}
}

func TestASessionPointedSomewhereElseIsUnknown(t *testing.T) {
	for _, name := range []string{"ANTHROPIC_BASE_URL", "ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN"} {
		t.Run(name, func(t *testing.T) {
			writeCreds(t, claudeCredsJSON, "")
			url, calls := serve(t, http.StatusOK, claudeLimitsBody)

			got := newService(url, "", time.Now()).claudePlan(Account{
				Env:  map[string]string{name: "somewhere-else"},
				Read: true,
			})

			if got.Status != StatusUnknown {
				t.Errorf("status = %q, want %q", got.Status, StatusUnknown)
			}
			if *calls != 0 {
				t.Errorf("calls = %d, want none: that session spends no plan lich meters", *calls)
			}
		})
	}
}

func TestEachAccountGetsItsOwnCachedReading(t *testing.T) {
	writeCreds(t, claudeCredsJSON, "")
	url, calls := serve(t, http.StatusOK, claudeLimitsBody)
	s := newService(url, "", time.Now())
	s.SetSessions(func(sessionID string) Account {
		return Account{Env: map[string]string{claudeTokenVar: "token-" + sessionID}, Read: true}
	})

	s.Plans("a")
	s.Plans("b")
	if *calls != 2 {
		t.Fatalf("calls = %d, want one per account: a cached reading is not another login's", *calls)
	}

	// The same account is served from the one reading, inside the TTL.
	s.Plans("a")
	if *calls != 2 {
		t.Errorf("calls = %d, want the second session's reading reused", *calls)
	}
}

func TestPlansWithoutASessionReadsLichOwnEnvironment(t *testing.T) {
	writeCreds(t, claudeCredsJSON, "")
	url, calls := serve(t, http.StatusOK, claudeLimitsBody)
	s := newService(url, "", time.Now())
	s.SetSessions(func(string) Account {
		t.Error("the machine-wide reading must not ask about a session")
		return Account{}
	})

	// The plan name is the pin: whatever withholds a reading from an account
	// lich does not bill must not reach the machine that has nothing to hide.
	if got := s.Plans("")[0]; got.Status != "ok" || got.Plan != "Max 5x" || *calls != 1 {
		t.Errorf("plan = %+v after %d calls, want the default login read", got, *calls)
	}
}

func TestAnApiKeyInLichOwnEnvironmentWithholdsTheReading(t *testing.T) {
	for _, name := range []string{"ANTHROPIC_BASE_URL", "ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN"} {
		t.Run(name, func(t *testing.T) {
			writeCreds(t, claudeCredsJSON, "")
			t.Setenv(name, "sk-ant-whatever")
			url, calls := serve(t, http.StatusOK, claudeLimitsBody)

			got := newService(url, "", time.Now()).Plans("")[0]

			if got.Status != "unknown" {
				t.Errorf("status = %q, want %q", got.Status, "unknown")
			}
			if got.Plan != "" {
				t.Errorf("plan = %q, want none: the login on disk is not what this machine bills", got.Plan)
			}
			if *calls != 0 {
				t.Errorf("calls = %d, want none: there is no subscription being spent", *calls)
			}
		})
	}
}

func TestASessionWithNoEnvironmentOfItsOwnSharesTheMachineReading(t *testing.T) {
	writeCreds(t, claudeCredsJSON, "")
	url, calls := serve(t, http.StatusOK, claudeLimitsBody)
	s := newService(url, "", time.Now())
	// macOS and Windows read no session environment at all, so every session
	// there arrives shaped like this one — and spends what Settings reads.
	s.SetSessions(func(string) Account { return Account{Read: true} })

	s.Plans("")
	s.Plans("session-1")

	if *calls != 1 {
		t.Errorf("calls = %d, want 1: both readings are the same account", *calls)
	}
}

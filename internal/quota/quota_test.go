package quota

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// claudeCredsJSON is the shape Claude Code writes, trimmed to what lich reads.
const claudeCredsJSON = `{"claudeAiOauth":{
	"accessToken":"tok-claude",
	"refreshToken":"never-read",
	"subscriptionType":"max",
	"rateLimitTier":"default_claude_max_5x"
}}`

const codexCredsJSON = `{"tokens":{"access_token":"tok-codex","refresh_token":"never-read"}}`

// writeCreds points both providers' state directories at t.TempDir and writes
// the given credential files there. An empty body means "no file at all".
func writeCreds(t *testing.T, claude, codex string) {
	t.Helper()
	claudeDir := filepath.Join(t.TempDir(), "claude")
	codexDir := filepath.Join(t.TempDir(), "codex")
	for dir, body := range map[string]string{claudeDir: claude, codexDir: codex} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if body == "" {
			continue
		}
		name := ".credentials.json"
		if dir == codexDir {
			name = "auth.json"
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatalf("write creds: %v", err)
		}
	}
	t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)
	t.Setenv("CODEX_HOME", codexDir)
}

// serve answers every request with status and body, and counts the calls.
func serve(t *testing.T, status int, body string) (url string, calls *int) {
	t.Helper()
	n := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server.URL, &n
}

// newService builds a Service pointed at the given endpoints with a fixed clock.
func newService(claudeURL, codexURL string, now time.Time) *Service {
	return &Service{
		http:      &http.Client{Timeout: httpTimeout},
		claudeURL: claudeURL,
		codexURL:  codexURL,
		now:       func() time.Time { return now },
	}
}

const claudeLimitsBody = `{
	"five_hour": {"utilization": 2.0, "resets_at": "2026-08-17T23:20:00.153182+00:00"},
	"seven_day": {"utilization": 0.0, "resets_at": "2026-08-24T15:00:00+00:00"},
	"limits": [
		{"kind":"session","percent":2,"resets_at":"2026-08-17T23:20:00.153182+00:00"},
		{"kind":"weekly_all","percent":41,"resets_at":"2026-08-24T15:00:00+00:00"},
		{"kind":"weekly_scoped","percent":11,"resets_at":null,
		 "scope":{"model":{"display_name":"Fable"}}}
	]
}`

func TestClaudePlanReadsLimits(t *testing.T) {
	writeCreds(t, claudeCredsJSON, "")
	url, calls := serve(t, http.StatusOK, claudeLimitsBody)
	got := newService(url, "", time.Now()).claudePlan()

	if got.Status != StatusOK {
		t.Fatalf("status = %q, want %q", got.Status, StatusOK)
	}
	if got.Provider != "claude" || got.Name != "Claude Code" {
		t.Errorf("identity = %q/%q, want claude/Claude Code", got.Provider, got.Name)
	}
	if got.Plan != "Max 5x" {
		t.Errorf("plan = %q, want %q", got.Plan, "Max 5x")
	}
	want := []Window{
		{Label: "Session", Seconds: 18000, Percent: 2, ResetsAt: "2026-08-17T23:20:00.153182+00:00"},
		{Label: "Weekly", Seconds: 604800, Percent: 41, ResetsAt: "2026-08-24T15:00:00+00:00"},
		{Label: "Fable", Seconds: 604800, Percent: 11},
	}
	if len(got.Windows) != len(want) {
		t.Fatalf("windows = %+v, want %d", got.Windows, len(want))
	}
	for i, w := range want {
		if got.Windows[i] != w {
			t.Errorf("window %d = %+v, want %+v", i, got.Windows[i], w)
		}
	}
	if *calls != 1 {
		t.Errorf("calls = %d, want 1", *calls)
	}
}

func TestClaudeRequestCarriesTheHeadersTheEndpointDemands(t *testing.T) {
	writeCreds(t, claudeCredsJSON, "")
	var got http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		_, _ = w.Write([]byte(claudeLimitsBody))
	}))
	defer server.Close()

	newService(server.URL, "", time.Now()).claudePlan()

	if auth := got.Get("Authorization"); auth != "Bearer tok-claude" {
		t.Errorf("Authorization = %q, want %q", auth, "Bearer tok-claude")
	}
	if beta := got.Get("anthropic-beta"); beta != "oauth-2025-04-20" {
		t.Errorf("anthropic-beta = %q, want %q", beta, "oauth-2025-04-20")
	}
	// Sent as anything else the endpoint rate-limits within a few requests.
	if agent := got.Get("User-Agent"); agent[:len("claude-code/")] != "claude-code/" {
		t.Errorf("User-Agent = %q, want a claude-code/… build", agent)
	}
}

func TestClaudeFallsBackToTheOriginalWindowPair(t *testing.T) {
	writeCreds(t, claudeCredsJSON, "")
	body := `{"five_hour":{"utilization":63,"resets_at":"2026-08-17T23:20:00Z"},
		"seven_day":{"utilization":7,"resets_at":"2026-08-24T15:00:00Z"},"limits":[]}`
	url, _ := serve(t, http.StatusOK, body)

	got := newService(url, "", time.Now()).claudePlan()

	want := []Window{
		{Label: "Session", Seconds: 18000, Percent: 63, ResetsAt: "2026-08-17T23:20:00Z"},
		{Label: "Weekly", Seconds: 604800, Percent: 7, ResetsAt: "2026-08-24T15:00:00Z"},
	}
	if len(got.Windows) != 2 || got.Windows[0] != want[0] || got.Windows[1] != want[1] {
		t.Errorf("windows = %+v, want %+v", got.Windows, want)
	}
}

func TestClaudeSkipsEntriesItCannotDraw(t *testing.T) {
	writeCreds(t, claudeCredsJSON, "")
	body := `{"limits":[
		{"kind":"session","percent":5,"resets_at":"2026-08-17T23:20:00Z"},
		{"kind":"opus_promotional","percent":80},
		{"kind":"weekly_all","percent":null},
		{"kind":"weekly_scoped","percent":3,"scope":{"model":{"display_name":""}}},
		{"kind":"weekly_scoped","percent":4,"scope":null}
	]}`
	url, _ := serve(t, http.StatusOK, body)

	got := newService(url, "", time.Now()).claudePlan()

	if len(got.Windows) != 1 || got.Windows[0].Label != "Session" {
		t.Fatalf("windows = %+v, want only the session window", got.Windows)
	}
}

func TestClaudeDropsAnUnparseableResetTime(t *testing.T) {
	writeCreds(t, claudeCredsJSON, "")
	body := `{"limits":[{"kind":"session","percent":5,"resets_at":"whenever"}]}`
	url, _ := serve(t, http.StatusOK, body)

	got := newService(url, "", time.Now()).claudePlan()

	if len(got.Windows) != 1 || got.Windows[0].ResetsAt != "" {
		t.Errorf("windows = %+v, want the window with no reset time", got.Windows)
	}
}

func TestSignedOutStates(t *testing.T) {
	tests := []struct {
		name   string
		creds  string
		status int
	}{
		{name: "no credentials file", creds: "", status: http.StatusOK},
		{name: "no token in the file", creds: `{"claudeAiOauth":{"accessToken":""}}`, status: http.StatusOK},
		{name: "token rejected", creds: claudeCredsJSON, status: http.StatusUnauthorized},
		{name: "token forbidden", creds: claudeCredsJSON, status: http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writeCreds(t, tt.creds, "")
			url, _ := serve(t, tt.status, `{}`)

			got := newService(url, "", time.Now()).claudePlan()

			if got.Status != StatusSignedOut {
				t.Errorf("status = %q, want %q", got.Status, StatusSignedOut)
			}
			if len(got.Windows) != 0 {
				t.Errorf("windows = %+v, want none", got.Windows)
			}
		})
	}
}

func TestFailedStates(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{name: "server error", status: http.StatusInternalServerError, body: `{}`},
		{name: "unparseable body", status: http.StatusOK, body: `not json`},
		{name: "no window in the payload", status: http.StatusOK, body: `{"limits":[]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writeCreds(t, claudeCredsJSON, "")
			url, _ := serve(t, tt.status, tt.body)

			got := newService(url, "", time.Now()).claudePlan()

			if got.Status != StatusError {
				t.Errorf("status = %q, want %q", got.Status, StatusError)
			}
		})
	}
}

func TestCodexPlanNamesWindowsByDuration(t *testing.T) {
	writeCreds(t, "", codexCredsJSON)
	// The free tier reports a single 30-day window where a paid plan reports
	// five-hour and weekly ones — in the same primary_window slot.
	body := `{"plan_type":"free","rate_limit":{"primary_window":{
		"used_percent":26,"limit_window_seconds":2592000,"reset_at":1789318316}}}`
	url, _ := serve(t, http.StatusOK, body)

	got := newService("", url, time.Now()).codexPlan()

	if got.Status != StatusOK || got.Plan != "Free" {
		t.Fatalf("plan = %+v, want an ok Free plan", got)
	}
	want := Window{Label: "Monthly", Seconds: 2592000, Percent: 26, ResetsAt: "2026-09-13T16:51:56Z"}
	if len(got.Windows) != 1 || got.Windows[0] != want {
		t.Errorf("windows = %+v, want %+v", got.Windows, want)
	}
}

func TestCodexReadsBothWindows(t *testing.T) {
	writeCreds(t, "", codexCredsJSON)
	body := `{"plan_type":"pro","rate_limit":{
		"primary_window":{"used_percent":12.4,"limit_window_seconds":18000,"reset_at":0},
		"secondary_window":{"used_percent":88.6,"limit_window_seconds":604800,"reset_at":1789318316}}}`
	url, _ := serve(t, http.StatusOK, body)

	got := newService("", url, time.Now()).codexPlan()

	want := []Window{
		{Label: "Session", Seconds: 18000, Percent: 12},
		{Label: "Weekly", Seconds: 604800, Percent: 89, ResetsAt: "2026-09-13T16:51:56Z"},
	}
	if len(got.Windows) != 2 || got.Windows[0] != want[0] || got.Windows[1] != want[1] {
		t.Errorf("windows = %+v, want %+v", got.Windows, want)
	}
}

func TestCodexWithoutRateLimitBlockFails(t *testing.T) {
	writeCreds(t, "", codexCredsJSON)
	url, _ := serve(t, http.StatusOK, `{"plan_type":"plus"}`)

	if got := newService("", url, time.Now()).codexPlan(); got.Status != StatusError {
		t.Errorf("status = %q, want %q", got.Status, StatusError)
	}
}

func TestCodexLabel(t *testing.T) {
	tests := []struct {
		seconds int
		want    string
	}{
		{seconds: 0, want: "Usage"},
		{seconds: -1, want: "Usage"},
		{seconds: 18000, want: "Session"},
		{seconds: 86399, want: "Session"},
		{seconds: 86400, want: "Weekly"},
		{seconds: 604800, want: "Weekly"},
		{seconds: 604801, want: "Monthly"},
		{seconds: 2592000, want: "Monthly"},
	}
	for _, tt := range tests {
		if got := codexLabel(tt.seconds); got != tt.want {
			t.Errorf("codexLabel(%d) = %q, want %q", tt.seconds, got, tt.want)
		}
	}
}

func TestPercentClampsToTheGauge(t *testing.T) {
	tests := []struct {
		in   float64
		want int
	}{
		{in: -4, want: 0},
		{in: 0, want: 0},
		{in: 12.4, want: 12},
		{in: 12.5, want: 13},
		{in: 99.9, want: 100},
		{in: 103, want: 100},
	}
	for _, tt := range tests {
		if got := percent(tt.in); got != tt.want {
			t.Errorf("percent(%v) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestPlanLabel(t *testing.T) {
	tests := []struct {
		subscription string
		tier         string
		want         string
	}{
		{subscription: "max", tier: "default_claude_max_5x", want: "Max 5x"},
		{subscription: "max", tier: "default_claude_max_20x", want: "Max 20x"},
		{subscription: "pro", tier: "default_claude_pro", want: "Pro"},
		{subscription: "", tier: "default_claude_max_5x", want: ""},
	}
	for _, tt := range tests {
		var creds claudeCredentials
		creds.OAuth.SubscriptionType = tt.subscription
		creds.OAuth.RateLimitTier = tt.tier
		if got := creds.planLabel(); got != tt.want {
			t.Errorf("planLabel(%q, %q) = %q, want %q", tt.subscription, tt.tier, got, tt.want)
		}
	}
}

func TestPlansServesEveryMeteredProviderFromOneReading(t *testing.T) {
	writeCreds(t, claudeCredsJSON, codexCredsJSON)
	claudeURL, claudeCalls := serve(t, http.StatusOK, claudeLimitsBody)
	codexURL, codexCalls := serve(t, http.StatusOK,
		`{"plan_type":"plus","rate_limit":{"primary_window":{"used_percent":9,"limit_window_seconds":18000}}}`)
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	s := newService(claudeURL, codexURL, now)

	plans := s.Plans()
	if len(plans) != 2 {
		t.Fatalf("plans = %+v, want one per metered provider", plans)
	}
	if plans[0].Provider != "claude" || plans[1].Provider != "codex" {
		t.Errorf("order = %q/%q, want claude/codex", plans[0].Provider, plans[1].Provider)
	}

	// Inside the TTL the endpoints are not asked again.
	s.now = func() time.Time { return now.Add(4*time.Minute + 59*time.Second) }
	s.Plans()
	if *claudeCalls != 1 || *codexCalls != 1 {
		t.Fatalf("calls = %d/%d after a cached read, want 1/1", *claudeCalls, *codexCalls)
	}

	// Past it they are.
	s.now = func() time.Time { return now.Add(5*time.Minute + time.Second) }
	s.Plans()
	if *claudeCalls != 2 || *codexCalls != 2 {
		t.Errorf("calls = %d/%d after the TTL, want 2/2", *claudeCalls, *codexCalls)
	}
}

func TestNewPointsAtTheLiveEndpoints(t *testing.T) {
	s := New()
	if s.claudeURL != "https://api.anthropic.com/api/oauth/usage" {
		t.Errorf("claudeURL = %q", s.claudeURL)
	}
	if s.codexURL != "https://chatgpt.com/backend-api/wham/usage" {
		t.Errorf("codexURL = %q", s.codexURL)
	}
	if s.now == nil || s.http == nil {
		t.Error("New must wire a clock and an HTTP client")
	}
}

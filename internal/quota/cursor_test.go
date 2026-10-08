package quota

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// cursorNow is the clock every Cursor test reads tokens against.
var cursorNow = time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)

// cursorToken builds a WorkOS access token with the given subject and expiry.
// Only its payload is ever read.
func cursorToken(sub string, exp time.Time) string {
	payload := fmt.Sprintf(`{"sub":%q,"exp":%d}`, sub, exp.Unix())
	return "eyJhbGciOiJIUzI1NiJ9." + base64.RawURLEncoding.EncodeToString([]byte(payload)) + ".sig"
}

// cursorSummary is the usage summary of a Pro account partway through its
// cycle, in the shape measured on a live free account on 2026-10-08.
const cursorSummary = `{
	"billingCycleStart":"2026-09-25T15:55:17.063Z","billingCycleEnd":"2026-10-25T15:55:17.063Z",
	"membershipType":"pro","limitType":"user","isUnlimited":false,
	"individualUsage":{
		"plan":{"enabled":true,"used":800,"limit":2000,"remaining":1200,
			"autoPercentUsed":12,"apiPercentUsed":55,"totalPercentUsed":40},
		"onDemand":{"enabled":false,"used":0,"limit":null,"remaining":null}},
	"teamUsage":{}}`

// writeCursorLogin writes auth.json and cli-config.json the way cursor-agent
// does into a fresh config dir, and points the account at it.
func writeCursorLogin(t *testing.T, token string) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"auth.json":       fmt.Sprintf(`{"accessToken":%q,"refreshToken":"never-read"}`, token),
		"cli-config.json": `{"version":1,"authInfo":{"email":"dev@example.com","displayName":"Dev"}}`,
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv(cursorConfigDirVar, dir)
	return dir
}

func cursorService(url string) *Service {
	s := newService("", "", cursorNow)
	s.cursorURL = url
	return s
}

func TestCursorReadsTheBillingCycle(t *testing.T) {
	token := cursorToken("auth0|user_01", cursorNow.Add(time.Hour))
	writeCursorLogin(t, token)
	var header http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header = r.Header.Clone()
		_, _ = w.Write([]byte(cursorSummary))
	}))
	t.Cleanup(server.Close)

	got := cursorService(server.URL).cursorPlan(lichEnv())

	if got.Status != StatusOK || got.Plan != "Pro" || got.Account != "dev@example.com" {
		t.Fatalf("plan = %+v, want an ok Pro reading for dev@example.com", got)
	}
	month := 30 * dailyWindow
	reset := "2026-10-25T15:55:17Z"
	want := []Window{
		{Label: "Monthly", Seconds: month, Percent: 40, ResetsAt: reset},
		{Label: "Cursor models", Seconds: month, Percent: 12, ResetsAt: reset},
		{Label: "Other models", Seconds: month, Percent: 55, ResetsAt: reset},
	}
	if !slices.Equal(got.Windows, want) {
		t.Errorf("windows = %+v, want %+v", got.Windows, want)
	}
	if c := header.Get("Cookie"); c != "WorkosCursorSessionToken=auth0%7Cuser_01%3A%3A"+token {
		t.Errorf("cookie = %q, want the dashboard's subject::token session", c)
	}
	if header.Get("Origin") != "https://cursor.com" || header.Get("Referer") != "https://cursor.com/dashboard" {
		t.Errorf("origin/referer = %q/%q, want the dashboard's own", header.Get("Origin"), header.Get("Referer"))
	}
	if header.Get("Authorization") != "" {
		t.Errorf("authorization = %q, want the cookie alone", header.Get("Authorization"))
	}
}

func TestCursorPrefersTheKeychainToken(t *testing.T) {
	writeCursorLogin(t, cursorToken("auth0|file", cursorNow.Add(time.Hour)))
	keychain := cursorToken("auth0|keychain", cursorNow.Add(time.Hour))
	if _, sub, _ := cursorLogin(keychain, os.Getenv(cursorConfigDirVar), cursorNow); sub != "auth0|keychain" {
		t.Errorf("subject = %q, want the Keychain's", sub)
	}
	// An expired Keychain token gives way to a live file one.
	expired := cursorToken("auth0|keychain", cursorNow.Add(-time.Hour))
	if _, sub, _ := cursorLogin(expired, os.Getenv(cursorConfigDirVar), cursorNow); sub != "auth0|file" {
		t.Errorf("subject = %q, want the file's", sub)
	}
}

func TestCursorAsksTheKeychainItWasWiredTo(t *testing.T) {
	writeCursorLogin(t, "")
	url, calls := serve(t, http.StatusOK, cursorSummary)
	s := cursorService(url)
	s.cursorKeychain = func() string { return cursorToken("auth0|keychain", cursorNow.Add(time.Hour)) }
	if got := s.cursorPlan(lichEnv()); got.Status != StatusOK || *calls != 1 {
		t.Errorf("plan = %+v after %d calls, want the Keychain login read", got, *calls)
	}
}

func TestCursorSignedOut(t *testing.T) {
	for name, token := range map[string]string{
		"no token":      "",
		"expired token": cursorToken("auth0|user_01", cursorNow.Add(-time.Minute)),
		"not a JWT":     "opaque",
		"no subject":    cursorToken("", cursorNow.Add(time.Hour)),
	} {
		t.Run(name, func(t *testing.T) {
			writeCursorLogin(t, token)
			url, calls := serve(t, http.StatusOK, cursorSummary)
			if got := cursorService(url).cursorPlan(lichEnv()); got.Status != StatusSignedOut {
				t.Errorf("status = %q, want signed out", got.Status)
			}
			if *calls != 0 {
				t.Errorf("calls = %d, want none: there is no live token to send", *calls)
			}
		})
	}
	t.Run("rejected", func(t *testing.T) {
		writeCursorLogin(t, cursorToken("auth0|user_01", cursorNow.Add(time.Hour)))
		url, _ := serve(t, http.StatusUnauthorized, `{"error":"not_authenticated"}`)
		if got := cursorService(url).cursorPlan(lichEnv()); got.Status != StatusSignedOut {
			t.Errorf("status = %q, want signed out", got.Status)
		}
	})
}

func TestCursorFailures(t *testing.T) {
	for name, tc := range map[string]struct {
		status int
		body   string
	}{
		"server error": {http.StatusInternalServerError, "oops"},
		"not JSON":     {http.StatusOK, "<html>"},
	} {
		writeCursorLogin(t, cursorToken("auth0|user_01", cursorNow.Add(time.Hour)))
		url, _ := serve(t, tc.status, tc.body)
		if got := cursorService(url).cursorPlan(lichEnv()); got.Status != StatusError {
			t.Errorf("%s: status = %q, want error", name, got.Status)
		}
	}
	if got := cursorService("").cursorPlan(Account{Read: true, Env: map[string]string{}}); got.Status != StatusError {
		t.Errorf("no home: status = %q, want error", got.Status)
	}
	if got := cursorService("").cursorPlan(Account{}); got.Status != StatusUnknown {
		t.Errorf("hidden: status = %q, want unknown", got.Status)
	}
}

func TestCursorWindows(t *testing.T) {
	yes, no := true, false
	pct := func(v float64) *float64 { return &v }
	cycle := cursorUsage{BillingCycleStart: "2026-09-25T00:00:00Z", BillingCycleEnd: "2026-10-25T00:00:00Z"}
	for name, tc := range map[string]struct {
		usage cursorUsage
		want  []string
	}{
		"unlimited": {cursorUsage{IsUnlimited: true}, nil},
		"team billed": {withPools(cycle, &cursorPool{Enabled: &no, TotalPercentUsed: pct(0)}, nil),
			nil},
		"used over limit": {withPools(cycle, &cursorPool{Used: pct(50), Limit: pct(200)}, nil),
			[]string{"Monthly 25"}},
		"on-demand": {withPools(cycle, nil, &cursorPool{Enabled: &yes, Used: pct(3), Limit: pct(10)}),
			[]string{"On-demand 30"}},
		"on-demand off": {withPools(cycle, nil, &cursorPool{Enabled: &no, TotalPercentUsed: pct(9)}),
			nil},
		"no limit": {withPools(cycle, &cursorPool{Used: pct(5), Limit: pct(0)}, nil), nil},
	} {
		var got []string
		for _, w := range tc.usage.windows() {
			got = append(got, fmt.Sprintf("%s %d", w.Label, w.Percent))
		}
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s: windows = %q, want %q", name, got, tc.want)
		}
	}
}

func withPools(u cursorUsage, plan, onDemand *cursorPool) cursorUsage {
	u.IndividualUsage = &struct {
		Plan     *cursorPool `json:"plan"`
		OnDemand *cursorPool `json:"onDemand"`
	}{plan, onDemand}
	return u
}

func TestCursorCycle(t *testing.T) {
	for _, tc := range []struct {
		start, end string
		seconds    int
		reset      string
	}{
		{"2026-09-25T00:00:00Z", "2026-10-25T00:00:00Z", 30 * dailyWindow, "2026-10-25T00:00:00Z"},
		{"", "2026-10-25T00:00:00Z", 0, "2026-10-25T00:00:00Z"},
		{"2026-10-26T00:00:00Z", "2026-10-25T00:00:00Z", 0, "2026-10-25T00:00:00Z"},
		{"2026-09-25T00:00:00Z", "soon", 0, ""},
	} {
		seconds, reset := cursorUsage{BillingCycleStart: tc.start, BillingCycleEnd: tc.end}.cycle()
		if seconds != tc.seconds || reset != tc.reset {
			t.Errorf("cycle(%q, %q) = %d/%q, want %d/%q", tc.start, tc.end, seconds, reset, tc.seconds, tc.reset)
		}
	}
}

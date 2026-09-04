package quota

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// claudeProfileBody is what the OAuth profile route answers, trimmed to the one
// field lich reads out of it.
const claudeProfileBody = `{"account":{"uuid":"acc-1","email":"dev@example.com"},
	"organization":{"uuid":"org-1"}}`

// idToken mints an unsigned id token carrying the given claims. The signature
// is the literal "sig" because nothing reads it — which is the property under
// test as much as a convenience.
func idToken(t *testing.T, claims map[string]any) string {
	t.Helper()
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	head := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256"}`))
	return head + "." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}

// codexUsageBody is a paid plan's usage, enough to leave a reading with windows
// while the account name is what is under test.
const codexUsageBody = `{"plan_type":"pro","rate_limit":{
	"primary_window":{"used_percent":12,"limit_window_seconds":18000,"reset_at":0}}}`

// codexCredsWith is the auth.json shape with an id token beside the access one.
func codexCredsWith(idTok string) string {
	return `{"tokens":{"access_token":"tok-codex","id_token":"` + idTok + `"}}`
}

// serveRoutes answers each path with its body, so one server can stand in for
// both the usage and the profile route.
func serveRoutes(t *testing.T, bodies map[string]string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := bodies[r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server
}

func TestClaudeNamesTheAccountItsLoginSpends(t *testing.T) {
	writeCreds(t, claudeCredsJSON, "")
	server := serveRoutes(t, map[string]string{
		"/usage":   claudeLimitsBody,
		"/profile": claudeProfileBody,
	})
	svc := newService(server.URL+"/usage", "", time.Now())
	svc.profileURL = server.URL + "/profile"

	got := svc.claudePlan(lichEnv())

	if got.Account != "dev@example.com" {
		t.Errorf("account = %q, want the email the profile route reports", got.Account)
	}
	if got.Status != StatusOK || len(got.Windows) == 0 {
		t.Errorf("reading = %+v, want the windows unaffected", got)
	}
}

func TestClaudeSendsTheProfileRouteTheHeadersTheUsageRouteNeeds(t *testing.T) {
	writeCreds(t, claudeCredsJSON, "")
	var got http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/profile") {
			got = r.Header.Clone()
			_, _ = w.Write([]byte(claudeProfileBody))
			return
		}
		_, _ = w.Write([]byte(claudeLimitsBody))
	}))
	defer server.Close()
	svc := newService(server.URL+"/usage", "", time.Now())
	svc.profileURL = server.URL + "/profile"

	svc.claudePlan(lichEnv())

	if auth := got.Get("Authorization"); auth != "Bearer tok-claude" {
		t.Errorf("Authorization = %q, want the credentials token", auth)
	}
	if beta := got.Get("anthropic-beta"); beta != claudeBetaHeader {
		t.Errorf("anthropic-beta = %q, want %q", beta, claudeBetaHeader)
	}
	if agent := got.Get("User-Agent"); agent != claudeUserAgent {
		t.Errorf("User-Agent = %q, want %q", agent, claudeUserAgent)
	}
}

// A refused or broken profile route must cost the name and nothing else: the
// gauge is what was asked for, the account is the label on it.
func TestClaudeKeepsItsGaugeWhenTheProfileRouteWillNotAnswer(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{name: "scope refused", status: http.StatusForbidden, body: `{}`},
		{name: "route gone", status: http.StatusNotFound, body: `{}`},
		{name: "not the json it should be", status: http.StatusOK, body: `<html>`},
		{name: "no email in the payload", status: http.StatusOK, body: `{"account":{"uuid":"a"}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writeCreds(t, claudeCredsJSON, "")
			usage := serveRoutes(t, map[string]string{"/usage": claudeLimitsBody})
			profile := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer profile.Close()
			svc := newService(usage.URL+"/usage", "", time.Now())
			svc.profileURL = profile.URL

			got := svc.claudePlan(lichEnv())

			if got.Account != "" {
				t.Errorf("account = %q, want none", got.Account)
			}
			if got.Status != StatusOK {
				t.Errorf("status = %q, want the reading to survive", got.Status)
			}
			if len(got.Windows) != 3 {
				t.Errorf("windows = %+v, want the three the usage route reported", got.Windows)
			}
		})
	}
}

// A token-only login is never asked: the profile route refuses `user:inference`
// exactly as the usage route does, so the probe path spends no request on it.
func TestClaudeProbeAsksTheProfileRouteNothing(t *testing.T) {
	url, _ := serveProbe(t, probeHeaders)
	asked := false
	profile := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		asked = true
		_, _ = w.Write([]byte(claudeProfileBody))
	}))
	defer profile.Close()
	svc := newService("", "", time.Now())
	svc.probeURL, svc.profileURL = url, profile.URL

	got := svc.claudePlan(Account{Env: map[string]string{claudeTokenVar: "tok-long-lived"}, Read: true})

	if asked {
		t.Error("the probe path asked the profile route, which cannot answer that scope")
	}
	if got.Account != "" {
		t.Errorf("account = %q, want none", got.Account)
	}
	if len(got.Windows) != 2 {
		t.Errorf("windows = %+v, want the two the headers carry", got.Windows)
	}
}

func TestCodexNamesTheAccountItsIDTokenCarries(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	writeCreds(t, "", codexCredsWith(idToken(t, map[string]any{
		"email": "dev@example.com",
		"exp":   now.Unix() + 3600,
	})))
	url, _ := serve(t, http.StatusOK, codexUsageBody)

	got := newService("", url, now).codexPlan(lichEnv())

	if got.Account != "dev@example.com" {
		t.Errorf("account = %q, want the email claim", got.Account)
	}
	if got.Status != StatusOK || len(got.Windows) == 0 {
		t.Errorf("reading = %+v, want the windows unaffected", got)
	}
}

// The id token can go stale while the CLI rotates the access token beside it,
// and naming an account the user has since left is worse than naming none.
func TestCodexRefusesToNameAnAccountFromAnExpiredIDToken(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	writeCreds(t, "", codexCredsWith(idToken(t, map[string]any{
		"email": "stale@example.com",
		"exp":   now.Unix() - 1,
	})))
	url, _ := serve(t, http.StatusOK, codexUsageBody)

	got := newService("", url, now).codexPlan(lichEnv())

	if got.Account != "" {
		t.Errorf("account = %q, want none from an expired token", got.Account)
	}
	if got.Status != StatusOK || len(got.Windows) == 0 {
		t.Errorf("reading = %+v, want the gauge to survive the missing name", got)
	}
}

func TestJWTEmailReadsOnlyWhatItCanTrust(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	fresh := map[string]any{"email": "dev@example.com", "exp": now.Unix() + 60}
	for _, tc := range []struct {
		name  string
		token string
		want  string
	}{
		{name: "fresh", token: idToken(t, fresh), want: "dev@example.com"},
		{name: "expiring next second", token: idToken(t, map[string]any{
			"email": "dev@example.com", "exp": now.Unix() + 1,
		}), want: "dev@example.com"},
		{name: "expiring this second", token: idToken(t, map[string]any{
			"email": "dev@example.com", "exp": now.Unix(),
		}), want: ""},
		{name: "no expiry claim", token: idToken(t, map[string]any{"email": "dev@example.com"}), want: ""},
		{name: "no email claim", token: idToken(t, map[string]any{"exp": now.Unix() + 60}), want: ""},
		{name: "absent", token: "", want: ""},
		{name: "not three segments", token: "header.payload", want: ""},
		{name: "payload not base64url", token: "h.!!!.sig", want: ""},
		{name: "payload not json", token: "h." + base64.RawURLEncoding.EncodeToString([]byte("nope")) + ".s", want: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := jwtEmail(tc.token, now); got != tc.want {
				t.Errorf("jwtEmail = %q, want %q", got, tc.want)
			}
		})
	}
}

// Padded base64url is not what a JWT is spelled in, but an encoder that pads is
// a name lost for no reason.
func TestJWTEmailReadsAPaddedPayload(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	payload, err := json.Marshal(map[string]any{"email": "dev@example.com", "exp": now.Unix() + 60})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	token := "h." + base64.URLEncoding.EncodeToString(payload) + ".sig"

	if got := jwtEmail(token, now); got != "dev@example.com" {
		t.Errorf("jwtEmail = %q, want the claim read through the padding", got)
	}
}

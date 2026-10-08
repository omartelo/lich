package quota

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// opencodeUsageBody is the Go usage route's answer in the shape the opencode
// console writes it (routes/zen/go/v1/usage.ts).
const opencodeUsageBody = `{"usage":{
	"rolling":{"status":"ok","percent":30,"resetsAt":"2026-10-08T17:00:00.000Z"},
	"weekly":{"status":"ok","percent":51.4,"resetsAt":"2026-10-12T00:00:00.000Z"},
	"monthly":{"status":"rate-limited","percent":100,"resetsAt":"2026-11-01T00:00:00.000Z"}}}`

// writeOpencodeAuth writes opencode's auth.json under a fresh XDG_DATA_HOME.
func writeOpencodeAuth(t *testing.T, body string) {
	t.Helper()
	data := t.TempDir()
	dir := filepath.Join(data, "opencode")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "auth.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(dataHomeVar, data)
}

const opencodeGoAuth = `{"opencode":{"type":"api","key":"zen-key"},"opencode-go":{"type":"api","key":"go-key"}}`

func opencodeService(url string) *Service {
	s := newService("", "", time.Now())
	s.opencodeURL = url
	return s
}

func TestOpencodeReadsTheGoSubscription(t *testing.T) {
	writeOpencodeAuth(t, opencodeGoAuth)
	var auth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(opencodeUsageBody))
	}))
	t.Cleanup(server.Close)

	got := opencodeService(server.URL).opencodePlan(lichEnv())

	if got.Status != StatusOK || got.Provider != "opencode" || got.Plan != "Go" {
		t.Fatalf("plan = %+v, want an ok Go reading", got)
	}
	want := []Window{
		{Label: "Session", Seconds: sessionWindow, Percent: 30, ResetsAt: "2026-10-08T17:00:00.000Z"},
		{Label: "Weekly", Seconds: weeklyWindow, Percent: 51, ResetsAt: "2026-10-12T00:00:00.000Z"},
		{Label: "Monthly", Seconds: monthlyWindow, Percent: 100, ResetsAt: "2026-11-01T00:00:00.000Z"},
	}
	if !slices.Equal(got.Windows, want) {
		t.Errorf("windows = %+v, want %+v", got.Windows, want)
	}
	if auth != "Bearer go-key" {
		t.Errorf("authorization = %q, want the Go key, never the Zen one", auth)
	}
}

func TestOpencodeWithoutAGoKeyIsLeftOut(t *testing.T) {
	for name, body := range map[string]string{
		"zen key only": `{"opencode":{"type":"api","key":"zen-key"}}`,
		"oauth entry":  `{"opencode-go":{"type":"oauth","refresh":"r"}}`,
		"empty key":    `{"opencode-go":{"type":"api","key":""}}`,
		"not JSON":     `nope`,
	} {
		t.Run(name, func(t *testing.T) {
			writeOpencodeAuth(t, body)
			url, calls := serve(t, http.StatusOK, opencodeUsageBody)
			if got := opencodeService(url).opencodePlan(lichEnv()); got.Provider != "" {
				t.Errorf("plan = %+v, want none", got)
			}
			if *calls != 0 {
				t.Errorf("calls = %d, want none", *calls)
			}
		})
	}
}

func TestOpencodeStatuses(t *testing.T) {
	for name, tc := range map[string]struct {
		status int
		body   string
		want   string
	}{
		// Both bodies were measured against the live route on 2026-10-08.
		"no Go subscription": {http.StatusForbidden,
			`{"type":"error","error":{"type":"EntitlementError","message":"OpenCode Go subscription required."}}`,
			StatusSignedOut},
		"unknown key": {http.StatusUnauthorized,
			`{"type":"error","error":{"type":"AuthError","message":"Unauthorized"}}`, StatusSignedOut},
		"server error":   {http.StatusBadGateway, "", StatusError},
		"not JSON":       {http.StatusOK, "<html>", StatusError},
		"no usage":       {http.StatusOK, `{}`, StatusError},
		"weekly missing": {http.StatusOK, `{"usage":{"rolling":{"percent":3}}}`, StatusError},
	} {
		writeOpencodeAuth(t, opencodeGoAuth)
		url, _ := serve(t, tc.status, tc.body)
		if got := opencodeService(url).opencodePlan(lichEnv()); got.Status != tc.want {
			t.Errorf("%s: status = %q, want %q", name, got.Status, tc.want)
		}
	}
}

func TestOpencodeKeepsAPlanWithoutAMonthlyWindow(t *testing.T) {
	writeOpencodeAuth(t, opencodeGoAuth)
	url, _ := serve(t, http.StatusOK, `{"usage":{"rolling":{"percent":1},"weekly":{"percent":2}}}`)
	if got := opencodeService(url).opencodePlan(lichEnv()); len(got.Windows) != 2 {
		t.Errorf("windows = %+v, want the session and weekly ones", got.Windows)
	}
}

func TestOpencodeReadsTheSessionsDataHome(t *testing.T) {
	writeOpencodeAuth(t, opencodeGoAuth)
	url, calls := serve(t, http.StatusOK, opencodeUsageBody)
	s := opencodeService(url)

	elsewhere := Account{Read: true, Env: map[string]string{dataHomeVar: t.TempDir()}}
	if got := s.opencodePlan(elsewhere); got.Provider != "" || *calls != 0 {
		t.Errorf("plan = %+v, want none: the session's data home holds no Go key", got)
	}
	if got := s.opencodePlan(Account{Read: true, Env: map[string]string{}}); got.Status != StatusError {
		t.Errorf("no home: status = %q, want error", got.Status)
	}
	if got := s.opencodePlan(Account{}); got.Status != StatusUnknown {
		t.Errorf("hidden: status = %q, want unknown", got.Status)
	}
}

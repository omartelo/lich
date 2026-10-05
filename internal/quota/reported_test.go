package quota

import (
	"net/http"
	"testing"
	"time"
)

var reportClock = time.Date(2026, 10, 4, 20, 0, 0, 0, time.UTC)

var reportedLimits = []ReportedLimit{
	{Kind: "five_hour", PercentUsed: 14, ResetsAt: "2026-10-05T00:30:00.000Z"},
	{Kind: "seven_day", PercentUsed: 79.6, ResetsAt: "2026-10-05T15:00:00.000Z"},
	{Kind: "spend_limit", PercentUsed: 3},
}

// reportingService is a Service whose one session spends account, with the
// probe and both usage routes answered by a server that counts its requests.
func reportingService(t *testing.T, account Account) (*Service, *int) {
	t.Helper()
	writeCreds(t, claudeCredsJSON, "")
	url, calls := serve(t, http.StatusInternalServerError, "")
	svc := newService(url, url, reportClock)
	svc.SetSessions(func(string) Account { return account })
	return svc, calls
}

func tokenAccount() Account {
	return Account{Env: map[string]string{claudeTokenVar: "oat-token"}, Read: true}
}

func claudeOf(plans []Plan) Plan {
	for _, p := range plans {
		if p.Provider == "claude" {
			return p
		}
	}
	return Plan{}
}

// TestAReportStandsInForTheProbe is why the contract exists: a token login is
// otherwise measured by a request against the plan it measures, and a session
// that reported its rate limits already said what that request would read.
func TestAReportStandsInForTheProbe(t *testing.T) {
	svc, calls := reportingService(t, tokenAccount())

	svc.ReportClaude("s1", reportedLimits)
	got := claudeOf(svc.Plans("s1"))

	want := []Window{
		{Label: "Session", Seconds: sessionWindow, Percent: 14, ResetsAt: "2026-10-05T00:30:00.000Z"},
		{Label: "Weekly", Seconds: weeklyWindow, Percent: 80, ResetsAt: "2026-10-05T15:00:00.000Z"},
	}
	if got.Status != StatusOK || got.NoAccount != NoAccountTokenLogin {
		t.Fatalf("plan = %+v, want an ok token-login reading", got)
	}
	if len(got.Windows) != 2 || got.Windows[0] != want[0] || got.Windows[1] != want[1] {
		t.Errorf("windows = %+v, want %+v (spend_limit left out)", got.Windows, want)
	}
	if *calls != 0 {
		t.Errorf("requests = %d, want none: the probe must not run", *calls)
	}
}

// TestAReportUpdatesTheCachedReading keeps the gauge from waiting out the cache
// for a figure it already has.
func TestAReportUpdatesTheCachedReading(t *testing.T) {
	svc, calls := reportingService(t, tokenAccount())
	if got := claudeOf(svc.Plans("s1")); got.Status != StatusError {
		t.Fatalf("first reading = %q, want the failed probe", got.Status)
	}

	svc.ReportClaude("s1", reportedLimits)
	got := claudeOf(svc.Plans("s1"))

	if got.Status != StatusOK || len(got.Windows) != 2 {
		t.Errorf("plan = %+v, want the reported windows", got)
	}
	if *calls != 1 {
		t.Errorf("requests = %d, want the first reading's probe and no more", *calls)
	}
}

// TestAReportPastItsResetIsMeasuredAgain pins the staleness rule: a window that
// reset since the report no longer reads what the report says.
func TestAReportPastItsResetIsMeasuredAgain(t *testing.T) {
	svc, calls := reportingService(t, tokenAccount())
	svc.ReportClaude("s1", []ReportedLimit{{Kind: "five_hour", PercentUsed: 90, ResetsAt: "2026-10-04T19:00:00Z"}})

	got := claudeOf(svc.Plans("s1"))

	if got.Status != StatusError || *calls != 1 {
		t.Errorf("plan = %q after %d requests, want the probe's failure: the report is stale", got.Status, *calls)
	}
}

// TestAReportWithoutAResetLastsOneCacheWindow covers a window that names no
// reset: nothing says when it stops being true, so it is held as long as a
// fetched reading would be.
func TestAReportWithoutAResetLastsOneCacheWindow(t *testing.T) {
	report := claudeReport{windows: []Window{{Label: "Session", Percent: 3}}, at: reportClock}

	if report.stale(reportClock.Add(cacheTTL - time.Second)) {
		t.Error("stale inside the cache window")
	}
	if !report.stale(reportClock.Add(cacheTTL)) {
		t.Error("still fresh past the cache window")
	}
}

// TestACredentialsLoginKeepsItsUsageRoute: that route names the plan, the
// account and model-scoped caps a report never carries, and costs no quota.
func TestACredentialsLoginKeepsItsUsageRoute(t *testing.T) {
	svc, calls := reportingService(t, lichEnv())

	svc.ReportClaude("s1", reportedLimits)
	got := claudeOf(svc.Plans("s1"))

	if got.Status != StatusError || *calls != 1 {
		t.Errorf("plan = %q after %d requests, want the usage route asked as before", got.Status, *calls)
	}
}

// TestAReportWithNoDrawableWindowIsIgnored: a gateway's spend limit alone
// meters no plan, so it must not stand in for one.
func TestAReportWithNoDrawableWindowIsIgnored(t *testing.T) {
	svc, calls := reportingService(t, tokenAccount())

	svc.ReportClaude("s1", []ReportedLimit{{Kind: "spend_limit", PercentUsed: 104.5}})
	got := claudeOf(svc.Plans("s1"))

	if got.Status != StatusError || *calls != 1 {
		t.Errorf("plan = %q after %d requests, want the probe", got.Status, *calls)
	}
}

// TestAReportWithoutSessionsWiredIsIgnored: the machine-wide reader has no
// session to tie a report to an account.
func TestAReportWithoutSessionsWiredIsIgnored(t *testing.T) {
	svc := newService("", "", reportClock)

	svc.ReportClaude("s1", reportedLimits)

	if len(svc.reports) != 0 {
		t.Errorf("reports = %v, want none kept", svc.reports)
	}
}

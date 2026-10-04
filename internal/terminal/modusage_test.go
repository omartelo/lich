// Reuses stubBins from the Unix-tagged terminal_test.go, so it carries the same
// tag. Nothing here is platform-specific.
//go:build !windows

package terminal

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/omartelo/lich/internal/events"
	"github.com/omartelo/lich/internal/quota"
)

func reportOf(conversation string, window int) usageReport {
	return usageReport{SessionID: "s1", ConversationID: conversation, Context: reportedContext{Window: window}}
}

func intOf(v int) *int { return &v }

func usdOf(v float64) *float64 { return &v }

// TestAReportedCostReplacesTheTranscriptScan is the point of the contract: the
// scan undercounts (requests no transcript records), so a conversation Claude
// Code has priced itself is shown at its own figure, sub-agents included, and a
// second read neither adds the scan back nor the sub-agents twice.
func TestAReportedCostReplacesTheTranscriptScan(t *testing.T) {
	writeTranscript(t, "uuid-cost", costLineJSON("m1", modelOpus, 100, 0, 0, 0)+"\n")
	writeSubagentTranscript(t, "uuid-cost", "agent-one", costLineJSON("m2", modelOpus, 300, 0, 0, 0)+"\n")
	store := newCostStore("uuid-cost")
	svc := New(store, nil, events.New())
	svc.prices = testRate
	if _, ok := svc.sessionUsage("s1"); !ok {
		t.Fatal("sessionUsage before the report: want ok")
	}

	report := reportOf("uuid-cost", 200_000)
	report.CostUSD = usdOf(2.5)
	svc.usageReports.set(report)

	for range 2 {
		got, ok := svc.sessionUsage("s1")
		if !ok {
			t.Fatal("sessionUsage: want ok")
		}
		if got.CostUSD == nil || !nearly(*got.CostUSD, 2.5) {
			t.Errorf("CostUSD = %v, want the reported 2.5 alone", got.CostUSD)
		}
	}
}

// TestAReportAboutAnotherConversationIsNotApplied pins the conversation match: a
// report raced by a /clear names the conversation that just ended, and its cost
// and window belong to that one, not to the one now running.
func TestAReportAboutAnotherConversationIsNotApplied(t *testing.T) {
	writeTranscript(t, "uuid-cost", costLineJSON("m1", modelOpus, 100, 0, 0, 0)+"\n")
	svc := New(newCostStore("uuid-cost"), nil, events.New())
	svc.prices = testRate
	report := reportOf("uuid-before-clear", 200_000)
	report.CostUSD = usdOf(9)
	svc.usageReports.set(report)

	got, ok := svc.sessionUsage("s1")

	if !ok {
		t.Fatal("sessionUsage: want ok")
	}
	if got.CostUSD == nil || !nearly(*got.CostUSD, 0.1) {
		t.Errorf("CostUSD = %v, want the scan's 0.1", got.CostUSD)
	}
	if got.Window != 1_000_000 {
		t.Errorf("Window = %d, want the transcript's 1000000", got.Window)
	}
}

// TestTheReportedWindowReplacesTheGuess is the ceiling a report closes: the
// transcript names the model but not its window, and an Opus run at 200k read
// as 1M. While the tokens agree, the percentage is Claude Code's own rounding.
func TestTheReportedWindowReplacesTheGuess(t *testing.T) {
	writeTranscript(t, "uuid-abc", assistantLine(modelOpus, 2, 43590, 0)+"\n")
	svc := New(stubBins{providerSession: "uuid-abc"}, nil, events.New())
	report := reportOf("uuid-abc", 200_000)
	report.Context.Tokens, report.Context.Percent = intOf(43592), intOf(22)
	svc.usageReports.set(report)

	got, ok := svc.sessionUsage("s1")

	if !ok {
		t.Fatal("sessionUsage: want ok")
	}
	if got.Window != 200_000 || got.Tokens != 43592 || got.Percent != 22 {
		t.Errorf("usage = %d/%d at %d%%, want 43592/200000 at Claude Code's 22%%",
			got.Tokens, got.Window, got.Percent)
	}
}

// TestTranscriptTokensNewerThanTheReportKeepMoving covers the turn between two
// reports: the transcript has grown past what the last report counted, so its
// tokens are read against the reported window rather than frozen at the report.
func TestTranscriptTokensNewerThanTheReportKeepMoving(t *testing.T) {
	writeTranscript(t, "uuid-abc", assistantLine(modelOpus, 2, 99998, 0)+"\n")
	svc := New(stubBins{providerSession: "uuid-abc"}, nil, events.New())
	report := reportOf("uuid-abc", 200_000)
	report.Context.Tokens, report.Context.Percent = intOf(43592), intOf(22)
	svc.usageReports.set(report)

	got, _ := svc.sessionUsage("s1")

	if got.Tokens != 100_000 || got.Percent != 50 {
		t.Errorf("usage = %d at %d%%, want the transcript's 100000 at 50%%", got.Tokens, got.Percent)
	}
}

// TestModUsageEndpointHandsRateLimitsToTheGauge drives the wire: a report
// POSTed to /mod/usage is kept for its session and its rate limits reach the
// plan gauge's reader.
func TestModUsageEndpointHandsRateLimitsToTheGauge(t *testing.T) {
	svc := New(stubBins{}, nil, events.New())
	if svc.ws == nil {
		t.Fatalf("transport: %v", svc.wsErr)
	}
	got := make(chan []quota.ReportedLimit, 1)
	svc.SetRateLimitReports(func(sessionID string, limits []quota.ReportedLimit) {
		if sessionID == "s1" {
			got <- limits
		}
	})
	body := `{"session_id":"s1","conversation_id":"c1","context":{"window":200000},` +
		`"rate_limits":[{"kind":"five_hour","percent_used":14,"resets_at":"2026-10-05T00:30:00.000Z"}]}`
	url := fmt.Sprintf("http://127.0.0.1:%d/mod/usage?token=%s", svc.ws.port, svc.ws.token)

	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
	select {
	case limits := <-got:
		want := quota.ReportedLimit{Kind: "five_hour", PercentUsed: 14, ResetsAt: "2026-10-05T00:30:00.000Z"}
		if len(limits) != 1 || limits[0] != want {
			t.Errorf("limits = %+v, want [%+v]", limits, want)
		}
	case <-time.After(time.Second):
		t.Fatal("the gauge's reader never got the rate limits")
	}
	if _, ok := svc.usageReports.of("s1", "c1"); !ok {
		t.Error("the report was not kept for its session")
	}
}

// TestAClosedSessionDropsItsReport keeps a closed card's report from outliving
// it: the id is never reused, and the map would only grow.
func TestAClosedSessionDropsItsReport(t *testing.T) {
	svc := New(stubBins{}, nil, events.New())
	svc.usageReports.set(reportOf("c1", 200_000))

	if err := svc.Close("s1"); err != nil {
		t.Fatal(err)
	}

	if _, ok := svc.usageReports.of("s1", "c1"); ok {
		t.Error("the report outlived its session")
	}
}

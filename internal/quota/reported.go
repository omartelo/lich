package quota

import (
	"slices"
	"time"

	"github.com/omartelo/lich/internal/providers"
)

// ReportedLimit is one rate-limit window as a Claude Code session measured it
// for itself (docs/hooks/mod-usage.md): its kind, the share of it spent (0–100,
// past 100 on an exceeded spend limit) and when it resets, RFC 3339.
type ReportedLimit struct {
	Kind        string
	PercentUsed float64
	ResetsAt    string
}

// reportedKinds names the windows a report carries that the gauge draws, the
// same two the probe reads off the response headers. A gateway's spend_limit
// meters no plan and is left out.
var reportedKinds = map[string]struct {
	label   string
	seconds int
}{
	"five_hour": {"Session", sessionWindow},
	"seven_day": {"Weekly", weeklyWindow},
}

// claudeReport is the last reading a session on one account reported.
type claudeReport struct {
	windows []Window
	at      time.Time
}

// stale reports a reading that no longer says how much is spent: one of its
// windows has reset since, or, for a window that names no reset, it is older
// than a fetched reading is ever served.
func (r claudeReport) stale(now time.Time) bool {
	for _, w := range r.windows {
		if w.ResetsAt == "" {
			if now.Sub(r.at) >= cacheTTL {
				return true
			}
			continue
		}
		reset, err := time.Parse(time.RFC3339, w.ResetsAt)
		if err != nil || !now.Before(reset) {
			return true
		}
	}
	return false
}

// ReportClaude takes the rate limits a Claude Code session reported for the
// account it spends. Only a long-lived token login uses them: its other reading
// costs a request against the plan it measures, while a credentials login's
// usage route also names the plan, the account and the model-scoped caps. A
// cached reading for that account is updated at once, so the gauge does not
// wait out the cache for a figure it already has.
func (s *Service) ReportClaude(sessionID string, limits []ReportedLimit) {
	if s.sessions == nil {
		return
	}
	account := s.sessions(sessionID)
	if account.hidden() || account.elsewhere() || account.lookup(claudeTokenVar) == "" {
		return
	}
	windows := reportedWindows(limits)
	if len(windows) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := cacheKey(account)
	now := s.now()
	if s.reports == nil {
		s.reports = make(map[string]claudeReport)
	}
	s.reports[key] = claudeReport{windows: windows, at: now}
	cached, ok := s.cache[key]
	if !ok {
		return
	}
	// A copy, never the cached slice: a reader may still be encoding it.
	plans := slices.Clone(cached.plans)
	for i, p := range plans {
		if p.Provider == providers.Claude {
			plans[i] = reportedPlan(slices.Clone(windows))
			pace(plans[i:i+1], now)
		}
	}
	s.cache[key] = reading{plans: plans, at: cached.at}
}

// reportedReading is the reading a session reported for the account a, while it
// still holds. Called with s.mu held.
func (s *Service) reportedReading(a Account) (Plan, bool) {
	report, ok := s.reports[cacheKey(a)]
	if !ok || report.stale(s.now()) {
		return Plan{}, false
	}
	return reportedPlan(slices.Clone(report.windows)), true
}

// reportedPlan is a token login's reading built from a report: the windows and
// nothing about the account, exactly what the probe would have read.
func reportedPlan(windows []Window) Plan {
	p := plan(providers.Claude)
	p.Windows = windows
	p.NoAccount = NoAccountTokenLogin
	return p
}

func reportedWindows(limits []ReportedLimit) []Window {
	out := make([]Window, 0, len(limits))
	for _, l := range limits {
		kind, ok := reportedKinds[l.Kind]
		if !ok {
			continue
		}
		out = append(out, Window{
			Label:    kind.label,
			Seconds:  kind.seconds,
			Percent:  percent(l.PercentUsed),
			ResetsAt: timestamp(l.ResetsAt),
		})
	}
	return out
}

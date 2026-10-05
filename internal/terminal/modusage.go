package terminal

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/omartelo/lich/internal/quota"
)

// usageReport is what a Claude Code session measured about itself
// (docs/hooks/mod-usage.md). Every figure belongs to ConversationID and is
// applied to no other conversation.
type usageReport struct {
	SessionID      string              `json:"session_id"`
	ConversationID string              `json:"conversation_id"`
	Context        reportedContext     `json:"context"`
	RateLimits     []reportedRateLimit `json:"rate_limits"`
	CostUSD        *float64            `json:"cost_usd,omitempty"`
}

type reportedContext struct {
	Window  int  `json:"window"`
	Tokens  *int `json:"tokens,omitempty"`
	Percent *int `json:"percent,omitempty"`
}

type reportedRateLimit struct {
	Kind        string  `json:"kind"`
	PercentUsed float64 `json:"percent_used"`
	ResetsAt    string  `json:"resets_at,omitempty"`
}

func (r usageReport) session() string { return r.SessionID }

func parseModUsage(body []byte) (usageReport, error) {
	var req usageReport
	if err := json.Unmarshal(body, &req); err != nil {
		return usageReport{}, fmt.Errorf("invalid mod usage body: %w", err)
	}
	if req.SessionID == "" {
		return usageReport{}, errors.New("mod usage missing session_id")
	}
	if req.ConversationID == "" {
		return usageReport{}, errors.New("mod usage missing conversation_id")
	}
	if err := req.Context.validate(); err != nil {
		return usageReport{}, err
	}
	for _, limit := range req.RateLimits {
		if err := limit.validate(); err != nil {
			return usageReport{}, err
		}
	}
	if req.CostUSD != nil && *req.CostUSD < 0 {
		return usageReport{}, errors.New("mod usage cost_usd is negative")
	}
	return req, nil
}

func (c reportedContext) validate() error {
	if c.Window <= 0 {
		return fmt.Errorf("mod usage context.window must be positive, got %d", c.Window)
	}
	if c.Tokens != nil && *c.Tokens < 0 {
		return errors.New("mod usage context.tokens is negative")
	}
	if c.Percent != nil && (*c.Percent < 0 || *c.Percent > 100) {
		return fmt.Errorf("mod usage context.percent must be 0 to 100, got %d", *c.Percent)
	}
	return nil
}

func (l reportedRateLimit) validate() error {
	if l.Kind == "" {
		return errors.New("mod usage rate limit missing kind")
	}
	if l.PercentUsed < 0 {
		return fmt.Errorf("mod usage rate limit %s percent_used is negative", l.Kind)
	}
	if l.ResetsAt != "" {
		if _, err := time.Parse(time.RFC3339, l.ResetsAt); err != nil {
			return fmt.Errorf("mod usage rate limit %s resets_at is not RFC 3339: %w", l.Kind, err)
		}
	}
	return nil
}

// modUsage receives a session's own measurements and hands them to the service.
func (t *transport) modUsage(w http.ResponseWriter, r *http.Request) {
	servePost(t, w, r, parseModUsage, func(req usageReport) error {
		t.mu.Lock()
		fn := t.usageReported
		t.mu.Unlock()
		if fn != nil {
			fn(req)
		}
		return nil
	})
}

// setUsageReported wires the callback a usage report runs. Wired after
// construction like setModAborted.
func (t *transport) setUsageReported(fn func(usageReport)) {
	t.mu.Lock()
	t.usageReported = fn
	t.mu.Unlock()
}

// usageReports is the latest report of each session. A session keeps one: a
// report carries the conversation's whole figures, never a delta.
type usageReports struct {
	mu   sync.Mutex
	byID map[string]usageReport
}

func (u *usageReports) set(r usageReport) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.byID == nil {
		u.byID = map[string]usageReport{}
	}
	u.byID[r.SessionID] = r
}

// of is session id's latest report, if it is about conversation.
func (u *usageReports) of(id, conversation string) (usageReport, bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	r, ok := u.byID[id]
	return r, ok && r.ConversationID == conversation
}

func (u *usageReports) forget(id string) {
	u.mu.Lock()
	delete(u.byID, id)
	u.mu.Unlock()
}

// SetRateLimitReports wires where the rate limits a session reports go: the
// plan gauge's reader. Startup wiring, called once before any session reports.
func (s *Service) SetRateLimitReports(fn func(sessionID string, limits []quota.ReportedLimit)) {
	s.rateLimitsMu.Lock()
	s.rateLimitReports = fn
	s.rateLimitsMu.Unlock()
}

// onUsageReport keeps a session's report, passes its rate limits on to the
// gauge and pushes the footer's readout again, now that it reads differently.
func (s *Service) onUsageReport(r usageReport) {
	s.usageReports.set(r)
	s.rateLimitsMu.Lock()
	fn := s.rateLimitReports
	s.rateLimitsMu.Unlock()
	if fn != nil && len(r.RateLimits) > 0 {
		limits := make([]quota.ReportedLimit, 0, len(r.RateLimits))
		for _, l := range r.RateLimits {
			limits = append(limits, quota.ReportedLimit{Kind: l.Kind, PercentUsed: l.PercentUsed, ResetsAt: l.ResetsAt})
		}
		fn(r.SessionID, limits)
	}
	go s.emitUsage(r.SessionID)
}

// reportedContextUsage refines what the transcript says about a conversation's
// context with what Claude Code reported about it. The window is the reported
// one, which no transcript records. The tokens stay the transcript's, which
// moves mid-turn and after a compaction where no report fires; while the two
// agree, the percentage is Claude Code's own rounding of them.
func reportedContextUsage(u contextUsage, c reportedContext) contextUsage {
	u.window = c.Window
	u.percent = min(u.tokens*100/u.window, 100)
	if c.Tokens != nil && *c.Tokens == u.tokens && c.Percent != nil {
		u.percent = *c.Percent
	}
	return u
}

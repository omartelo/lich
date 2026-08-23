package quota

import (
	"time"

	"github.com/omartelo/lich/internal/providers"
)

const (
	codexUsageURL = "https://chatgpt.com/backend-api/wham/usage"
	// codexUserAgent identifies the CLI this token belongs to, as the Anthropic
	// route's does.
	codexUserAgent = "codex_cli_rs/0.56.0"
	// codexHomeVar is the environment variable a session's own process names
	// its Codex state directory with.
	codexHomeVar = "CODEX_HOME"
)

// codexCredentials is the part of ~/.codex/auth.json lich reads. As with
// Claude, the refresh token beside it is left alone.
type codexCredentials struct {
	Tokens struct {
		AccessToken string `json:"access_token"`
	} `json:"tokens"`
}

// codexUsage is the response of the wham usage route.
type codexUsage struct {
	PlanType  string `json:"plan_type"`
	RateLimit *struct {
		Primary   *codexWindow `json:"primary_window"`
		Secondary *codexWindow `json:"secondary_window"`
	} `json:"rate_limit"`
}

type codexWindow struct {
	UsedPercent float64 `json:"used_percent"`
	Seconds     int     `json:"limit_window_seconds"`
	// ResetAt is Unix seconds, absent on older CLIs.
	ResetAt int64 `json:"reset_at"`
}

// codexPlan reads Codex's quota off the login its CLI wrote, under the state
// directory the session's own environment points at.
func (s *Service) codexPlan(a Account) Plan {
	p := plan(providers.Codex)
	if a.hidden() {
		return unknown(p)
	}
	path, ok := harnessFile(a, codexHomeVar, ".codex", "auth.json")
	if !ok {
		return failed(p)
	}
	var creds codexCredentials
	if !readCredentials(path, &creds) || creds.Tokens.AccessToken == "" {
		return signedOut(p)
	}

	var usage codexUsage
	authOK, err := s.readJSON(s.codexURL, creds.Tokens.AccessToken, map[string]string{
		"User-Agent": codexUserAgent,
	}, &usage)
	if !authOK {
		return signedOut(p)
	}
	if err != nil {
		return failed(p)
	}
	p.Plan = title(usage.PlanType)
	p.Windows = usage.windows()
	if len(p.Windows) == 0 {
		return failed(p)
	}
	return p
}

// windows projects both reported windows onto the neutral shape.
func (u codexUsage) windows() []Window {
	if u.RateLimit == nil {
		return nil
	}
	out := make([]Window, 0, 2)
	for _, w := range []*codexWindow{u.RateLimit.Primary, u.RateLimit.Secondary} {
		if w == nil {
			continue
		}
		out = append(out, Window{
			Label:    codexLabel(w.Seconds),
			Seconds:  w.Seconds,
			Percent:  percent(w.UsedPercent),
			ResetsAt: unixTimestamp(w.ResetAt),
		})
	}
	return out
}

// codexLabel names a window by how long it runs, never by where it sat in the
// payload: the first window is the five-hour one on a paid plan and a monthly
// one on the free tier, and a plan change must not silently relabel a gauge.
func codexLabel(seconds int) string {
	switch {
	case seconds <= 0:
		return "Usage"
	case seconds < 24*60*60:
		return "Session"
	case seconds <= weeklyWindow:
		return "Weekly"
	default:
		return "Monthly"
	}
}

// unixTimestamp renders a Unix-seconds reset as the RFC 3339 the frontend
// parses. Empty for the absent value, which older CLIs write as 0.
func unixTimestamp(sec int64) string {
	if sec <= 0 {
		return ""
	}
	return time.Unix(sec, 0).UTC().Format(time.RFC3339)
}

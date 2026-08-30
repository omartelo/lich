package quota

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/omartelo/lich/internal/providers"
)

const (
	claudeUsageURL = "https://api.anthropic.com/api/oauth/usage"
	// claudeBetaHeader opts the OAuth token into the usage route.
	claudeBetaHeader = "oauth-2025-04-20"
	// claudeUserAgent is what the endpoint expects from the client holding this
	// token. Sent as anything else it rate-limits within a few requests; the
	// patch version is not checked, so a recent one stays good.
	claudeUserAgent = "claude-code/2.1.183"
	// claudeAPIVersion is the dated API contract the probe request is written
	// against; the usage route above needs none.
	claudeAPIVersion = "2023-06-01"
)

// The environment variables a session's own process names its Claude login
// with: a long-lived OAuth token, which wins over everything on disk, and the
// config directory holding the credentials file when there is no token.
const (
	claudeTokenVar = "CLAUDE_CODE_OAUTH_TOKEN"
	claudeDirVar   = "CLAUDE_CONFIG_DIR"
)

// The quota probe: what lich sends when a session's login is a long-lived
// OAuth token rather than a credentials file. Such a token carries the
// `user:inference` scope alone — the usage route above answers it 403, since
// that one wants `user:profile` — so the account is measured the way Claude
// Code measures it for itself: send the smallest possible message and read the
// rate-limit headers off the response.
const (
	claudeProbeURL = "https://api.anthropic.com/v1/messages"
	// claudeProbeBody is one token of output on the cheapest model. The system
	// prompt is not decoration: the API rejects an OAuth token that arrives
	// without Claude Code's own, which is the same coupling as the user agent
	// above.
	claudeProbeBody = `{"model":"claude-haiku-4-5-20251001","max_tokens":1,` +
		`"messages":[{"role":"user","content":"quota"}],` +
		`"system":[{"type":"text","text":"You are Claude Code, Anthropic's official CLI for Claude."}]}`
	// claudeClaimPrefix is what every unified rate-limit header starts with.
	claudeClaimPrefix = "anthropic-ratelimit-unified-"
)

// claudeCredentials is the part of ~/.claude/.credentials.json lich reads. The
// refresh token in the same file is deliberately not among these fields: lich
// has no business spending it (see the package doc).
type claudeCredentials struct {
	OAuth struct {
		AccessToken      string `json:"accessToken"`
		SubscriptionType string `json:"subscriptionType"`
		RateLimitTier    string `json:"rateLimitTier"`
	} `json:"claudeAiOauth"`
}

// planLabel is the subscription as a person names it: the tier from the
// credentials, plus the multiplier the rate-limit tier spells out ("Max 5x").
func (c claudeCredentials) planLabel() string {
	name := title(c.OAuth.SubscriptionType)
	if name == "" {
		return ""
	}
	for _, multiplier := range []string{"20x", "5x"} {
		if strings.Contains(c.OAuth.RateLimitTier, multiplier) {
			return name + " " + multiplier
		}
	}
	return name
}

// claudeUsage is the response of the OAuth usage route. Every field is optional:
// the route is undocumented, its shape varies by plan, and it has already grown
// one representation of the same windows (limits) beside the original pair.
type claudeUsage struct {
	FiveHour *claudeWindow `json:"five_hour"`
	SevenDay *claudeWindow `json:"seven_day"`
	Limits   []claudeLimit `json:"limits"`
}

type claudeWindow struct {
	Utilization  float64 `json:"utilization"`
	ResetsAt     string  `json:"resets_at"`
	LockedReason string  `json:"locked_reason"`
}

type claudeLimit struct {
	Kind     string   `json:"kind"`
	Percent  *float64 `json:"percent"`
	ResetsAt string   `json:"resets_at"`
	IsActive bool     `json:"is_active"`
	Scope    *struct {
		Model *struct {
			DisplayName string `json:"display_name"`
		} `json:"model"`
	} `json:"scope"`
}

// claudePlan reads Claude Code's quota for the account a session spends: the
// token its own environment names, else the login its CLI wrote under the
// config directory that environment points at.
func (s *Service) claudePlan(a Account) Plan {
	p := plan(providers.Claude)
	if a.hidden() || a.elsewhere() {
		return unknown(p)
	}
	if token := a.Env[claudeTokenVar]; token != "" {
		return s.claudeProbe(p, token)
	}
	path, ok := harnessFile(a, claudeDirVar, ".claude", ".credentials.json")
	if !ok {
		return failed(p)
	}
	var creds claudeCredentials
	if !readCredentials(path, &creds) || creds.OAuth.AccessToken == "" {
		return signedOut(p)
	}
	p.Plan = creds.planLabel()

	var usage claudeUsage
	authOK, err := s.readJSON(s.claudeURL, creds.OAuth.AccessToken, map[string]string{
		"anthropic-beta": claudeBetaHeader,
		"User-Agent":     claudeUserAgent,
	}, &usage)
	if !authOK {
		return signedOut(p)
	}
	if err != nil {
		return failed(p)
	}
	p.Windows = usage.windows()
	if len(p.Windows) == 0 {
		return failed(p)
	}
	return p
}

// claudeProbe measures a token that can only infer: it sends the probe request
// and reads the windows off the response headers. The plan name stays empty —
// the headers carry the spend, never the subscription it is spent against.
func (s *Service) claudeProbe(p Plan, token string) Plan {
	resp, authOK, err := s.send(http.MethodPost, s.probeURL, token, claudeProbeBody, map[string]string{
		"anthropic-beta":    claudeBetaHeader,
		"anthropic-version": claudeAPIVersion,
		"content-type":      "application/json",
		"User-Agent":        claudeUserAgent,
	})
	if !authOK {
		return signedOut(p)
	}
	if err != nil {
		return failed(p)
	}
	defer func() { _ = resp.Body.Close() }()
	// The answer is in the headers; draining the body is what lets the
	// connection be reused for the next reading.
	_, _ = io.Copy(io.Discard, resp.Body)
	p.Windows = probeWindows(resp.Header)
	if len(p.Windows) == 0 {
		return failed(p)
	}
	return p
}

// probeWindows reads the unified rate-limit headers every message response
// carries: the share of each window this account has spent (0–1), and the Unix
// second it turns over. A claim the response does not report is skipped, which
// is how an account metered on one window alone reads right.
func probeWindows(h http.Header) []Window {
	out := make([]Window, 0, 2)
	for _, claim := range []struct {
		abbrev  string
		label   string
		seconds int
	}{
		{"5h", "Session", sessionWindow},
		{"7d", "Weekly", weeklyWindow},
	} {
		used, err := strconv.ParseFloat(h.Get(claudeClaimPrefix+claim.abbrev+"-utilization"), 64)
		if err != nil {
			continue
		}
		reset, _ := strconv.ParseInt(h.Get(claudeClaimPrefix+claim.abbrev+"-reset"), 10, 64)
		out = append(out, Window{
			Label:    claim.label,
			Seconds:  claim.seconds,
			Percent:  percent(used * 100),
			ResetsAt: unixTimestamp(reset),
		})
	}
	return out
}

// windows projects the response onto the neutral shape, preferring the limits
// array: it is the representation that carries model-scoped weekly caps, which
// have no field of their own. A payload without it — an older account, or the
// array dropped again — falls back to the original pair so the two windows
// everyone has keep reporting.
func (u claudeUsage) windows() []Window {
	out := make([]Window, 0, len(u.Limits))
	for _, limit := range u.Limits {
		if limit.Percent == nil {
			continue
		}
		label, seconds := limit.window()
		if label == "" {
			continue
		}
		out = append(out, Window{
			Label:        label,
			Seconds:      seconds,
			Percent:      percent(*limit.Percent),
			ResetsAt:     timestamp(limit.ResetsAt),
			Active:       limit.IsActive,
			LockedReason: u.lockedReason(limit.Kind),
		})
	}
	if len(out) > 0 {
		return out
	}
	for _, w := range []struct {
		window  *claudeWindow
		label   string
		seconds int
	}{
		{u.FiveHour, "Session", sessionWindow},
		{u.SevenDay, "Weekly", weeklyWindow},
	} {
		if w.window == nil {
			continue
		}
		out = append(out, Window{
			Label:        w.label,
			Seconds:      w.seconds,
			Percent:      percent(w.window.Utilization),
			ResetsAt:     timestamp(w.window.ResetsAt),
			LockedReason: w.window.LockedReason,
		})
	}
	return out
}

// lockedReason answers the top-level pair for a limits[] entry's kind:
// locked_reason lives only on five_hour/seven_day, never on limits[] itself —
// session mirrors five_hour and weekly_all mirrors seven_day. A model-scoped
// weekly cap has no pair of its own and never reports a lock.
func (u claudeUsage) lockedReason(kind string) string {
	switch kind {
	case "session":
		if u.FiveHour != nil {
			return u.FiveHour.LockedReason
		}
	case "weekly_all":
		if u.SevenDay != nil {
			return u.SevenDay.LockedReason
		}
	}
	return ""
}

// window names one limit entry and gives its length. An empty label is an entry
// lich does not draw: a kind it has no name for, or a model-scoped cap the API
// did not name. A new kind is therefore silently skipped rather than rendered
// as its wire spelling.
func (l claudeLimit) window() (string, int) {
	switch l.Kind {
	case "session":
		return "Session", sessionWindow
	case "weekly_all":
		return "Weekly", weeklyWindow
	case "weekly_scoped":
		if l.Scope == nil || l.Scope.Model == nil {
			return "", 0
		}
		return l.Scope.Model.DisplayName, weeklyWindow
	default:
		return "", 0
	}
}

// timestamp passes through a reset time the frontend can parse, and drops
// anything else: a gauge with no reset reads better than one counting down to
// an unparseable date.
func timestamp(s string) string {
	if s == "" {
		return ""
	}
	if _, err := time.Parse(time.RFC3339, s); err != nil {
		return ""
	}
	return s
}

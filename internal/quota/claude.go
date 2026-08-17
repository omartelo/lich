package quota

import (
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
	Utilization float64 `json:"utilization"`
	ResetsAt    string  `json:"resets_at"`
}

type claudeLimit struct {
	Kind     string   `json:"kind"`
	Percent  *float64 `json:"percent"`
	ResetsAt string   `json:"resets_at"`
	Scope    *struct {
		Model *struct {
			DisplayName string `json:"display_name"`
		} `json:"model"`
	} `json:"scope"`
}

// claudePlan reads Claude Code's quota off the login its CLI wrote.
func (s *Service) claudePlan() Plan {
	p := plan(providers.Claude)
	path, ok := harnessFile("CLAUDE_CONFIG_DIR", ".claude", ".credentials.json")
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
			Label:    label,
			Seconds:  seconds,
			Percent:  percent(*limit.Percent),
			ResetsAt: timestamp(limit.ResetsAt),
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
			Label:    w.label,
			Seconds:  w.seconds,
			Percent:  percent(w.window.Utilization),
			ResetsAt: timestamp(w.window.ResetsAt),
		})
	}
	return out
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

package quota

import (
	"net/url"
	"path/filepath"
	"time"

	"github.com/omartelo/lich/internal/providers"
)

const (
	// cursorOrigin is the dashboard the usage route belongs to. Its /api routes
	// answer 403 to a session cookie that arrives without the dashboard's own
	// Origin and Referer, as CSRF defence, so the request carries both.
	cursorOrigin   = "https://cursor.com"
	cursorUsageURL = cursorOrigin + "/api/usage-summary"
	// cursorConfigDirVar is the variable Cursor CLI's config dir answers to
	// first (providers.CursorConfigDir).
	cursorConfigDirVar = "CURSOR_CONFIG_DIR"
)

// cursorCredentials is the part of auth.json `cursor-agent login` writes that
// lich reads. The refresh token beside it is left alone.
type cursorCredentials struct {
	AccessToken string `json:"accessToken"`
}

// cursorIdentity is cli-config.json's signed-in identity; it carries no token.
type cursorIdentity struct {
	AuthInfo struct {
		Email string `json:"email"`
	} `json:"authInfo"`
}

// cursorUsage is the dashboard's usage summary. Timestamps are ISO strings and
// the pools' percentages whole numbers, both measured on 2026-10-08.
type cursorUsage struct {
	BillingCycleStart string `json:"billingCycleStart"`
	BillingCycleEnd   string `json:"billingCycleEnd"`
	MembershipType    string `json:"membershipType"`
	IsUnlimited       bool   `json:"isUnlimited"`
	IndividualUsage   *struct {
		Plan     *cursorPool `json:"plan"`
		OnDemand *cursorPool `json:"onDemand"`
	} `json:"individualUsage"`
}

// cursorPool is one allowance. The plan's is spent two ways, on Cursor's own
// models (auto) and on the others (API), each with its own percentage.
type cursorPool struct {
	Enabled          *bool    `json:"enabled"`
	Used             *float64 `json:"used"`
	Limit            *float64 `json:"limit"`
	TotalPercentUsed *float64 `json:"totalPercentUsed"`
	AutoPercentUsed  *float64 `json:"autoPercentUsed"`
	APIPercentUsed   *float64 `json:"apiPercentUsed"`
}

// cursorPlan reads Cursor's monthly allowance off the login Cursor CLI wrote.
// Only the CLI's own login is read: the Cursor IDE's session lives in another
// app's database, and the gauge answers for the agent lich runs.
func (s *Service) cursorPlan(a Account) Plan {
	p := plan(providers.Cursor)
	if a.hidden() {
		return unknown(p)
	}
	home := a.lookup(accountHomeVar)
	if home == "" {
		return failed(p)
	}
	dir := providers.CursorConfigDirIn(a.lookup, home)
	keychain := ""
	if s.cursorKeychain != nil {
		keychain = s.cursorKeychain()
	}
	token, subject, ok := cursorLogin(keychain, dir, s.now())
	if !ok {
		return signedOut(p)
	}
	var usage cursorUsage
	authOK, err := s.readJSON(s.cursorURL, "", map[string]string{
		"Cookie":  "WorkosCursorSessionToken=" + url.QueryEscape(subject) + "%3A%3A" + token,
		"Accept":  "application/json",
		"Origin":  cursorOrigin,
		"Referer": cursorOrigin + "/dashboard",
	}, &usage)
	if !authOK {
		return signedOut(p)
	}
	if err != nil {
		return failed(p)
	}
	p.Plan = title(usage.MembershipType)
	var identity cursorIdentity
	if readCredentials(filepath.Join(dir, "cli-config.json"), &identity) {
		p.Account = identity.AuthInfo.Email
	}
	p.Windows = usage.windows()
	return p
}

// cursorLogin is the first live token the CLI holds: the macOS Keychain, where
// cursor-agent keeps it since 2026.06, then auth.json. An expired one is passed
// over rather than sent to answer 401: the CLI refreshes it on its next use.
// subject is the token's `sub`, the first half of the dashboard's cookie.
func cursorLogin(keychain, dir string, now time.Time) (token, subject string, ok bool) {
	var creds cursorCredentials
	readCredentials(filepath.Join(dir, "auth.json"), &creds)
	for _, candidate := range []string{keychain, creds.AccessToken} {
		var claims struct {
			Sub string `json:"sub"`
			Exp int64  `json:"exp"`
		}
		if !jwtClaims(candidate, &claims) || claims.Sub == "" {
			continue
		}
		if claims.Exp > 0 && now.Unix() >= claims.Exp {
			continue
		}
		return candidate, claims.Sub, true
	}
	return "", "", false
}

// windows draws the plan's allowance and the two ways it is spent, then
// on-demand spend when it is switched on. All four reset with the billing
// cycle. A plan the account does not own (team billing) reports zeros it does
// not spend, and an unlimited one has no ceiling: neither is drawn.
func (u cursorUsage) windows() []Window {
	if u.IsUnlimited || u.IndividualUsage == nil {
		return nil
	}
	seconds, resetsAt := u.cycle()
	window := func(label string, used float64) Window {
		return Window{Label: label, Seconds: seconds, Percent: percent(used), ResetsAt: resetsAt}
	}
	var out []Window
	if pool := u.IndividualUsage.Plan; pool != nil && (pool.Enabled == nil || *pool.Enabled) {
		if used, ok := pool.spent(pool.TotalPercentUsed); ok {
			out = append(out, window("Monthly", used))
		}
		if pool.AutoPercentUsed != nil {
			out = append(out, window("Cursor models", *pool.AutoPercentUsed))
		}
		if pool.APIPercentUsed != nil {
			out = append(out, window("Other models", *pool.APIPercentUsed))
		}
	}
	if pool := u.IndividualUsage.OnDemand; pool != nil && pool.Enabled != nil && *pool.Enabled {
		if used, ok := pool.spent(pool.TotalPercentUsed); ok {
			out = append(out, window("On-demand", used))
		}
	}
	return out
}

// spent is the pool's share used: the reported percentage when there is one,
// else used over limit.
func (p cursorPool) spent(reported *float64) (float64, bool) {
	if reported != nil {
		return *reported, true
	}
	if p.Used != nil && p.Limit != nil && *p.Limit > 0 {
		return *p.Used / *p.Limit * 100, true
	}
	return 0, false
}

// cycle is the billing cycle's length and its end, which is when every pool
// resets. No length when either end is missing or unparseable.
func (u cursorUsage) cycle() (seconds int, resetsAt string) {
	end, err := time.Parse(time.RFC3339, u.BillingCycleEnd)
	if err != nil {
		return 0, ""
	}
	resetsAt = end.UTC().Format(time.RFC3339)
	start, err := time.Parse(time.RFC3339, u.BillingCycleStart)
	if err != nil || !end.After(start) {
		return 0, resetsAt
	}
	return int(end.Sub(start).Seconds()), resetsAt
}

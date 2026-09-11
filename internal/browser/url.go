package browser

import (
	"fmt"
	"net/url"
	"strings"
)

// allowURL is the only URL the agent browser will load. http, https and
// about:blank are in; file, javascript, data, chrome and everything else are
// out — a file: URL would hand the agent the machine, and javascript: is just
// evaluate by another name.
func allowURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.EqualFold(raw, "about:blank") {
		return "about:blank", nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid url: %w", err)
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		if u.Host == "" {
			return "", fmt.Errorf("url %q has no host", raw)
		}
		return u.String(), nil
	default:
		if u.Scheme == "" {
			return "", fmt.Errorf("url %q is missing a scheme", raw)
		}
		return "", fmt.Errorf("blocked url scheme %q", u.Scheme)
	}
}

// Package system holds the few OS integrations the frontend needs that are
// not tied to any domain service — today only opening a URL in the user's
// default browser (replacing the Wails Browser runtime, so it works in any
// shell; see docs/chromium-shell.md).
package system

import (
	"fmt"
	"net/url"
	"os/exec"
)

type Service struct {
	// open launches the URL; injectable for tests, xdg-open in production.
	open func(target string) error
}

func New() *Service {
	return &Service{open: func(target string) error {
		return exec.Command("xdg-open", target).Start()
	}}
}

// OpenExternal opens an http(s) URL in the default browser. Scheme-gated so a
// crafted terminal escape can never turn a click into a file:// or custom
// scheme launch — the same policy the Wails Browser runtime applied.
func (s *Service) OpenExternal(rawURL string) error {
	if err := ValidateExternalURL(rawURL); err != nil {
		return err
	}
	return s.open(rawURL)
}

// ValidateExternalURL accepts absolute http/https URLs only.
func ValidateExternalURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("refusing to open scheme %q", parsed.Scheme)
	}
	if parsed.Host == "" {
		return fmt.Errorf("refusing to open url without host")
	}
	return nil
}

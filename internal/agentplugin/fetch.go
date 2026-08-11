package agentplugin

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Fetching plugin files directly, for the harnesses that have no CLI to install
// them: opencode loads a module and Crush runs scripts, and neither ships a
// command that can put one on disk. Both read the release tag, so a file lich
// writes is the same file that release published — the version a card reports
// stays the plugin's, never lich's.

const (
	// fileBodyLimit caps a fetched plugin file. The largest is a few KiB of
	// JavaScript; anything past this is malformed or hostile, and lich is about
	// to write it somewhere a harness executes.
	fileBodyLimit = 1 << 20
	// fetchTimeout bounds one file. It belongs to an install the user asked for
	// and is watching, not to the startup status check the shared client's
	// timeout is sized for.
	fetchTimeout = 30 * time.Second
)

// fetchFile GETs one path from the plugin repository at a released version.
func (s *Service) fetchFile(version, path string) ([]byte, error) {
	url := fmt.Sprintf("%s/v%s/%s", strings.TrimSuffix(s.rawBase, "/"), version, path)
	ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("request %s: %w", url, err)
	}
	req.Header.Set("User-Agent", "lich")
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: %s", url, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, fileBodyLimit))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", url, err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("fetch %s: empty file", url)
	}
	return data, nil
}

// releaseVersion is the version an install writes, or an error naming why it
// could not be known. The install of a file-shipped harness cannot fall back to
// "whatever is on main": the version is what a card reports and what the next
// update compares against, so an unknown one has to stop the install rather
// than write a file lich cannot describe.
func (s *Service) releaseVersion() (string, error) {
	if v := s.latestVersion(); v != "" {
		return v, nil
	}
	return "", fmt.Errorf("cannot reach the %s releases — are you online?", marketplaceRepo)
}

// versionOf pulls the version out of the marker line lich writes above what it
// installs, given the comment prefix that file's syntax uses.
func versionOf(data []byte, prefix string) (string, bool) {
	marker := prefix + " " + markerName + " v"
	for line := range strings.Lines(string(data)) {
		rest, ok := strings.CutPrefix(strings.TrimSpace(line), marker)
		if !ok {
			continue
		}
		if version, _, _ := strings.Cut(rest, " "); version != "" {
			return version, true
		}
	}
	return "", false
}

// Package ghrelease reads the latest published release of a GitHub repository —
// the one piece lich's two update checks (the app in internal/appupdate and the
// Claude plugin in internal/claudeplugin) share. It reports the release tag as a
// bare version and never errors: a failed lookup yields "", which every caller
// treats as "no release known" so a network blip never blocks or breaks startup.
package ghrelease

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// bodyLimit caps the response read; a releases/latest payload is a few KiB, so
// anything larger is malformed or hostile.
const bodyLimit = 1 << 20

// LatestTag GETs a GitHub releases/latest URL and returns its tag as a bare
// semver (leading "v" stripped), or "" on any failure.
func LatestTag(client *http.Client, url string) string {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "lich")
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, bodyLimit))
	if err != nil {
		return ""
	}
	return parseTag(data)
}

// parseTag pulls tag_name out of a release JSON and normalizes it to a bare
// version.
func parseTag(data []byte) string {
	var doc struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return ""
	}
	return strings.TrimPrefix(doc.TagName, "v")
}

// Package ghrelease reads the published releases of a GitHub repository —
// the one piece lich's two update checks (the app in internal/appupdate and the
// lich plugin in internal/agentplugin) share. It reports release tags as bare
// versions and never errors: a failed lookup yields nothing, which every caller
// treats as "no release known" so a network blip never blocks or breaks startup.
package ghrelease

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// BodyLimit caps a metadata response read; a releases/latest payload — or a
// release's checksums.txt — is a few KiB and a page of the releases list tens of
// KiB, so anything larger is malformed or hostile.
const BodyLimit = 1 << 20

// Get issues a GET carrying lich's identifying headers. Every GitHub read lich
// makes goes through here so the request shape is stated once.
func Get(client *http.Client, url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "lich")
	return client.Do(req)
}

// LatestTag GETs a GitHub releases/latest URL and returns its tag as a bare
// semver (leading "v" stripped), or "" on any failure.
func LatestTag(client *http.Client, url string) string {
	data := read(client, url)
	if data == nil {
		return ""
	}
	return parseTag(data)
}

// ReleaseTags GETs a GitHub releases list URL and returns the tags of its
// published releases as bare versions, newest first as GitHub lists them. Drafts
// and pre-releases are left out: neither is something lich installs. Any
// failure yields nil.
func ReleaseTags(client *http.Client, url string) []string {
	data := read(client, url)
	if data == nil {
		return nil
	}
	var docs []struct {
		TagName    string `json:"tag_name"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
	}
	if err := json.Unmarshal(data, &docs); err != nil {
		return nil
	}
	tags := make([]string, 0, len(docs))
	for _, d := range docs {
		if !d.Draft && !d.Prerelease && d.TagName != "" {
			tags = append(tags, strings.TrimPrefix(d.TagName, "v"))
		}
	}
	return tags
}

// read GETs url and returns its body, capped at BodyLimit, or nil on any
// failure or non-200 answer.
func read(client *http.Client, url string) []byte {
	resp, err := Get(client, url)
	if err != nil {
		return nil
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, BodyLimit))
	if err != nil {
		return nil
	}
	return data
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

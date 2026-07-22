// Package patchnotes serves the changelog section for the running build so the
// app can show a "what's new" popup after an update. CHANGELOG.md is embedded
// at the module root (see main.go) and parsed into groups here — the frontend
// stays a dumb renderer.
package patchnotes

import "strings"

// Group is one "### Added / Changed / Fixed" block of a release's notes.
type Group struct {
	Label string `json:"label"`
	// Items keep their markdown bold/code markers; the frontend renders them.
	Items []string `json:"items"`
}

// Notes is a release's changelog section, ready for the frontend to render.
type Notes struct {
	Version string  `json:"version"`
	Groups  []Group `json:"groups"`
}

// Service serves the notes for one fixed version — the running build.
type Service struct {
	version   string
	changelog string
}

// New returns a service that parses changelog for version's section on demand.
func New(version, changelog string) *Service {
	return &Service{version: version, changelog: changelog}
}

// Current returns the running build's changelog section. Groups is empty when
// no section matches (a dev build, or a version not yet in the changelog), and
// the frontend then shows nothing.
func (s *Service) Current() Notes {
	return Notes{Version: normalize(s.version), Groups: Section(s.changelog, s.version)}
}

// normalize drops a leading "v" so "v0.11.0" and "0.11.0" compare equal.
func normalize(version string) string {
	return strings.TrimPrefix(version, "v")
}

// Section returns the groups under the "## [<version>]" heading, or nil when
// that heading is absent. The heading may carry a trailing date
// ("## [0.11.0] - 2026-07-21"); only the bracketed version is matched.
func Section(changelog, version string) []Group {
	body, ok := sliceSection(changelog, normalize(version))
	if !ok {
		return nil
	}
	return parseGroups(body)
}

// sliceSection returns the lines between the version's "## [..]" heading and the
// next "## " heading (end of file if none). ok is false when no heading matches.
func sliceSection(changelog, version string) ([]string, bool) {
	header := "## [" + version + "]"
	lines := strings.Split(changelog, "\n")
	start := -1
	for i, line := range lines {
		if strings.HasPrefix(line, header) {
			start = i + 1
			break
		}
	}
	if start == -1 {
		return nil, false
	}
	for i := start; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "## ") {
			return lines[start:i], true
		}
	}
	return lines[start:], true
}

// parseGroups turns the section lines into groups. A "### Label" opens a group,
// a "- " opens an item, wrapped continuation lines fold into it with a space,
// and a blank line ends an item. Groups that gather no items are dropped.
func parseGroups(lines []string) []Group {
	var groups []Group
	var item strings.Builder
	flush := func() {
		if item.Len() == 0 || len(groups) == 0 {
			item.Reset()
			return
		}
		last := len(groups) - 1
		groups[last].Items = append(groups[last].Items, item.String())
		item.Reset()
	}
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "### "):
			flush()
			groups = append(groups, Group{Label: strings.TrimSpace(line[4:])})
		case strings.HasPrefix(line, "- "):
			flush()
			item.WriteString(strings.TrimSpace(line[2:]))
		case strings.TrimSpace(line) == "":
			flush()
		default:
			if item.Len() > 0 {
				item.WriteByte(' ')
				item.WriteString(strings.TrimSpace(line))
			}
		}
	}
	flush()

	out := groups[:0]
	for _, g := range groups {
		if len(g.Items) > 0 {
			out = append(out, g)
		}
	}
	return out
}

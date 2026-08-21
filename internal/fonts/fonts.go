// Package fonts enumerates the operating system's installed font families so the
// frontend can offer them as terminal fonts. Chromium resolves fonts through the
// same database this package reads — fontconfig on Linux and macOS, the Fonts
// registry key on Windows — so any family listed here is renderable by name in
// the terminal without bundling it.
package fonts

import (
	"sort"
	"strings"
)

// Service lists installed font families.
type Service struct{}

// New returns a ready-to-use fonts service.
func New() *Service {
	return &Service{}
}

// List returns the sorted, de-duplicated set of installed font families,
// enumerated the way this OS records them (see the list_* files).
func (s *Service) List() ([]string, error) {
	return listFamilies()
}

// parseFamilies extracts the sorted, de-duplicated family names from fc-list's
// output.
func parseFamilies(out string) []string {
	seen := make(map[string]struct{})
	for line := range strings.SplitSeq(out, "\n") {
		// A line may hold comma-separated localized aliases; the first is the
		// canonical family name.
		name := strings.TrimSpace(strings.SplitN(line, ",", 2)[0])
		if name == "" {
			continue
		}
		seen[name] = struct{}{}
	}
	return sorted(seen)
}

// parseRegistryFamilies extracts the sorted, de-duplicated family names from the
// value names under the Windows Fonts registry key. A name reads
// "<family>[ <face>] (<format>)" — "Arial Bold Italic (TrueType)" — and one
// entry may register several families joined by " & ".
func parseRegistryFamilies(names []string) []string {
	seen := make(map[string]struct{})
	for _, name := range names {
		for part := range strings.SplitSeq(stripFontFormat(name), "&") {
			family := trimFontFaces(strings.TrimSpace(part))
			if family == "" {
				continue
			}
			seen[family] = struct{}{}
		}
	}
	return sorted(seen)
}

// stripFontFormat drops the trailing format parenthetical every Fonts registry
// value carries ("(TrueType)", "(OpenType)", "(VGA res)"). A name that opens with
// one keeps it: the parenthetical is a suffix, never the whole name.
func stripFontFormat(name string) string {
	if i := strings.LastIndex(name, "("); i > 0 {
		return name[:i]
	}
	return name
}

// fontFaceWords are the trailing words a registry entry appends to name one
// installed face, so every weight of Arial collapses onto "Arial" in the picker.
// The list is short on purpose: a word belongs here only when no real family ends
// in it, which is why "Black", "Light" and "Semibold" are absent — "Arial Black"
// and "Segoe UI Light" are families in their own right.
//
// Ceiling: a family shipping no regular face is offered under its trimmed name,
// which Chromium may then fail to resolve; the terminal falls back to the
// default font when that happens.
var fontFaceWords = map[string]bool{
	"bold": true, "italic": true, "oblique": true, "regular": true,
}

// trimFontFaces drops the face words trailing a registry family name. The last
// word is never dropped: a name that is nothing but a face word is a family
// called that.
func trimFontFaces(name string) string {
	fields := strings.Fields(name)
	for len(fields) > 1 && fontFaceWords[strings.ToLower(fields[len(fields)-1])] {
		fields = fields[:len(fields)-1]
	}
	return strings.Join(fields, " ")
}

// sorted returns the set's members in order.
func sorted(seen map[string]struct{}) []string {
	families := make([]string, 0, len(seen))
	for family := range seen {
		families = append(families, family)
	}
	sort.Strings(families)
	return families
}

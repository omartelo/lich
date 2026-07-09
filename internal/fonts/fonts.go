// Package fonts enumerates the operating system's installed font families so the
// frontend can offer them as terminal fonts. The WebKit webview resolves fonts
// through the same fontconfig database on Linux, so any family listed here is
// renderable by name in the canvas terminal without bundling it.
package fonts

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

// Service lists installed font families.
type Service struct{}

// New returns a ready-to-use fonts service.
func New() *Service {
	return &Service{}
}

// List returns the sorted, de-duplicated set of installed font families. It
// enumerates via fontconfig (fc-list), covering Linux/macOS; Windows would need
// a registry / DirectWrite path.
func (s *Service) List() ([]string, error) {
	out, err := exec.Command("fc-list", ":", "family").Output()
	if err != nil {
		return nil, fmt.Errorf("fc-list failed: %w", err)
	}

	seen := make(map[string]struct{})
	for _, line := range strings.Split(string(out), "\n") {
		// A line may hold comma-separated localized aliases; the first is the
		// canonical family name.
		name := strings.TrimSpace(strings.SplitN(line, ",", 2)[0])
		if name == "" {
			continue
		}
		seen[name] = struct{}{}
	}

	families := make([]string, 0, len(seen))
	for family := range seen {
		families = append(families, family)
	}
	sort.Strings(families)
	return families, nil
}

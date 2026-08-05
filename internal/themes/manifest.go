package themes

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/omartelo/lich/internal/semver"
)

// manifestName is what marks a repository as a lich theme pack. Every other
// *.json next to it is read as a theme.
const manifestName = "lich-theme.json"

// Manifest is the versioning stamp of a theme repository. It deliberately does
// not list the themes: id and name already live inside each theme file, and a
// second copy here would be a list to keep in sync for no gain.
type Manifest struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func (m Manifest) validate() error {
	if strings.TrimSpace(m.Name) == "" {
		return fmt.Errorf("manifest name is required")
	}
	if utf8.RuneCountInString(m.Name) > themeNameMaxLength {
		return fmt.Errorf("manifest name cannot exceed %d characters", themeNameMaxLength)
	}
	// Update compares versions with semver.Less, which orders no two distinct
	// pre-releases — a pack that versioned itself "1.2.0-rc1" would never be
	// seen as newer than "1.2.0-rc2".
	if !semver.IsRelease(m.Version) {
		return fmt.Errorf("manifest version %q must be MAJOR.MINOR.PATCH", m.Version)
	}
	return nil
}

// version is the manifest version normalized for storage and display.
func (m Manifest) version() string {
	return strings.TrimPrefix(strings.TrimSpace(m.Version), "v")
}

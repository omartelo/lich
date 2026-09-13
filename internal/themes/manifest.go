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
	FormatVersion int    `json:"formatVersion,omitempty"`
	Name          string `json:"name"`
	Version       string `json:"version"`
	// MinLichVersion is the oldest lich release the pack's themes are written
	// for, so an older lich refuses the pack by name before it trips on a token.
	MinLichVersion string `json:"minLichVersion,omitempty"`
}

// validate checks the manifest against the running lich. A lichVersion that is
// not a release ("dev", or git describe between tags) cannot be ordered, so it
// skips the minimum-version check rather than refuse every pack.
func (m Manifest) validate(lichVersion string) error {
	if err := validateFormatVersion(m.FormatVersion); err != nil {
		return fmt.Errorf("manifest: %w", err)
	}
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
	return checkMinLichVersion(m.MinLichVersion, lichVersion)
}

func checkMinLichVersion(minimum, lichVersion string) error {
	if minimum == "" {
		return nil
	}
	if !semver.IsRelease(minimum) {
		return fmt.Errorf("manifest minLichVersion %q must be MAJOR.MINOR.PATCH", minimum)
	}
	if semver.IsRelease(lichVersion) && semver.Less(lichVersion, minimum) {
		return fmt.Errorf("theme pack needs lich %s or newer (this is %s); update lich",
			strings.TrimPrefix(minimum, "v"), strings.TrimPrefix(lichVersion, "v"))
	}
	return nil
}

// version is the manifest version normalized for storage and display.
func (m Manifest) version() string {
	return strings.TrimPrefix(strings.TrimSpace(m.Version), "v")
}

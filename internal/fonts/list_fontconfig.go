//go:build !windows

package fonts

import (
	"fmt"
	"os/exec"
)

// listFamilies enumerates installed fonts through fontconfig, the database
// Chromium itself resolves against on Linux and macOS.
func listFamilies() ([]string, error) {
	out, err := exec.Command("fc-list", ":", "family").Output()
	if err != nil {
		return nil, fmt.Errorf("fc-list failed: %w", err)
	}
	return parseFamilies(string(out)), nil
}

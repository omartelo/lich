package agentplugin

import (
	"github.com/omartelo/lich/internal/semver"
)

// The plugin releases this lich speaks the hook contract with
// (docs/hooks/README.md, Versioning). The plugin bumps its minor for a contract
// change (its major, from 1.0) and its patch for anything else, so every release
// below the ceiling sends only what this lich accepts.
const (
	// PluginVersionFloor is the oldest plugin release this lich supports: the
	// first to name itself in the X-Lich-Plugin header, so every supported
	// install says which release a report comes from.
	PluginVersionFloor = "0.13.0"
	// PluginVersionCeiling is the first release past the newest contract this
	// lich implements (0.13), and the first one it cannot vouch for.
	PluginVersionCeiling = "0.14.0"
)

// Compatible reports whether a plugin release speaks the contract this lich
// implements. An unreadable version sorts as 0.0.0 and is not compatible.
func Compatible(version string) bool {
	return !semver.Less(version, PluginVersionFloor) && semver.Less(version, PluginVersionCeiling)
}

// newestCompatible picks the newest release in tags this lich is compatible
// with, or "" when none is.
func newestCompatible(tags []string) string {
	best := ""
	for _, tag := range tags {
		if Compatible(tag) && (best == "" || semver.Less(best, tag)) {
			best = tag
		}
	}
	return best
}

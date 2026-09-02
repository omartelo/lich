//go:build !windows && !darwin

package chromium

// browserCandidates lists candidate binaries in preference order — the shared
// chromiumNames list (resolve.go), since every Linux/BSD install lands on PATH
// under one of those names. Any Chromium gives the same compositor; the
// preference only picks the most conventional install. macOS keeps browsers in
// .app bundles off PATH and Windows under versioned install roots, so both have
// their own list.
func browserCandidates() []string {
	return chromiumNames
}

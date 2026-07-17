// Package semver compares release versions well enough for lich's update
// checks: MAJOR.MINOR.PATCH plus the SemVer rule that a pre-release precedes
// the release it qualifies. It does not order two distinct pre-releases.
package semver

import (
	"strconv"
	"strings"
)

// Release ranks order a pre-release below the release of the same version.
const (
	preReleaseRank = 0
	releaseRank    = 1
)

// Less reports whether version a is strictly older than b. Missing or
// non-numeric components sort as 0, and a pre-release sorts before the release
// it qualifies (SemVer §11: 1.0.0-rc1 < 1.0.0).
//
// Two different pre-releases of the same version compare equal (rc.1 vs rc.2 is
// not ordered): that would need identifier-wise comparison, and the update
// check only ever weighs a build against GitHub's "latest" release, which never
// resolves to a pre-release.
func Less(a, b string) bool {
	pa, pb := parse(a), parse(b)
	for i := range pa {
		if pa[i] != pb[i] {
			return pa[i] < pb[i]
		}
	}
	return false
}

// IsRelease reports whether v is a plain MAJOR.MINOR.PATCH release version —
// no pre-release or build-metadata suffix. A dev build ("dev", or the
// "-N-gSHA" that git describe emits between tags) is not a release, so the
// update check must not treat it as one.
func IsRelease(v string) bool {
	v = strings.TrimPrefix(v, "v")
	if strings.ContainsAny(v, "-+") {
		return false
	}
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
		if _, err := strconv.Atoi(p); err != nil {
			return false
		}
	}
	return true
}

// parse splits v into a comparable tuple: MAJOR, MINOR, PATCH, and a release
// rank that keeps a pre-release below the release of the same version.
func parse(v string) [4]int {
	v = strings.TrimPrefix(v, "v")
	// Build metadata carries no precedence (SemVer §10), so drop it before the
	// pre-release check — "+" may legally precede nothing else.
	if i := strings.IndexByte(v, '+'); i >= 0 {
		v = v[:i]
	}
	rank := releaseRank
	if i := strings.IndexByte(v, '-'); i >= 0 {
		v, rank = v[:i], preReleaseRank
	}
	out := [4]int{3: rank}
	for i, part := range strings.SplitN(v, ".", 3) {
		out[i], _ = strconv.Atoi(part)
	}
	return out
}

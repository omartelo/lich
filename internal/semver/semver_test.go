package semver

import "testing"

func TestLess(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"0.0.1", "0.0.2", true},
		{"0.0.1", "0.1.0", true},
		{"0.9.9", "1.0.0", true},
		{"1.0.0", "1.0.0", false},
		{"1.2.0", "1.1.9", false},
		{"v1.0.0", "v1.0.1", true},
		{"1.0", "1.0.1", true},

		// SemVer §11: a pre-release precedes the release it qualifies.
		{"1.0.0-rc1", "1.0.0", true},
		{"0.2.0-rc.3", "0.2.0", true},
		{"v0.2.0-rc.3", "v0.2.0", true},
		{"1.0.0", "1.0.0-rc1", false},
		{"1.0.0-rc1", "1.0.1", true},
		{"1.0.0-rc1", "0.9.9", false},
		{"1.0.0-rc1", "1.0.0-rc1", false},

		// Ordering between two distinct pre-releases is not resolved; they
		// compare equal, so neither is "less" than the other.
		{"1.0.0-rc1", "1.0.0-rc2", false},
		{"1.0.0-rc2", "1.0.0-rc1", false},

		// Build metadata carries no precedence (SemVer §10).
		{"1.0.0+build.5", "1.0.0", false},
		{"1.0.0", "1.0.0+build.5", false},
		{"1.0.0-rc1+build.5", "1.0.0", true},

		// Garbage a git tag never produces, kept sane: a separator at index 0
		// strips to empty numeric components rather than mis-parsing the digits
		// after it (pins the IndexByte boundary — the '+' and '-' at position 0).
		{"+5", "0.0.1", true},    // "+5" → build meta stripped → 0.0.0 < 0.0.1
		{"-5", "0.0.0-x", false}, // "-5" → leading pre-release marker → 0.0.0-pre, equal
	}
	for _, tc := range tests {
		if got := Less(tc.a, tc.b); got != tc.want {
			t.Errorf("Less(%q,%q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestIsRelease(t *testing.T) {
	tests := []struct {
		v    string
		want bool
	}{
		{"0.7.0", true},
		{"v0.7.0", true},
		{"1.2.3", true},
		{"dev", false},
		{"", false},
		{"1.0", false},             // not three components
		{"1.0.0.0", false},         // too many components
		{"0.7.0-rc.1", false},      // pre-release
		{"0.7.0-5-gabc123", false}, // git describe between tags
		{"1.0.0+build.5", false},   // build metadata
		{"1.x.0", false},           // non-numeric
	}
	for _, tc := range tests {
		if got := IsRelease(tc.v); got != tc.want {
			t.Errorf("IsRelease(%q) = %v, want %v", tc.v, got, tc.want)
		}
	}
}

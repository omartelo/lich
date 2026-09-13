package agentplugin

import "testing"

// The range is pinned in literals rather than read back from the constants, so
// moving a bound is a visible edit here too.
func TestCompatible(t *testing.T) {
	tests := map[string]bool{
		"0.12.9":       false,
		"0.13.0-rc.1":  false,
		"0.13.0":       true,
		"0.13.9":       true,
		"0.14.0-rc.1":  true,
		"0.14.0":       false,
		"1.0.0":        false,
		"":             false,
		"not-a-semver": false,
	}
	for version, want := range tests {
		if got := Compatible(version); got != want {
			t.Errorf("Compatible(%q) = %v, want %v", version, got, want)
		}
	}
}

func TestNewestCompatible(t *testing.T) {
	tests := []struct {
		name string
		tags []string
		want string
	}{
		{"newest in range wins over a newer one past it", []string{"0.14.0", "0.13.2", "0.13.10", "0.12.9"}, "0.13.10"},
		{"order does not matter", []string{"0.13.0", "0.13.1"}, "0.13.1"},
		{"nothing in range", []string{"0.14.0", "0.12.0"}, ""},
		{"no releases", nil, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := newestCompatible(tc.tags); got != tc.want {
				t.Fatalf("newestCompatible(%v) = %q, want %q", tc.tags, got, tc.want)
			}
		})
	}
}

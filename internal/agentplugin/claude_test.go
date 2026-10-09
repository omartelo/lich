package agentplugin

import "testing"

// A worker opens cards only under a release that tells it a worker by its
// depth, since an older one would stop answering for it.
func TestNestsSubagentCardsFromTheFirstNestingRelease(t *testing.T) {
	cases := map[string]bool{
		"0.18.4":       false,
		"0.19.0-rc.1":  false,
		"0.19.0":       true,
		"0.19.2":       true,
		"not-a-semver": false,
	}
	for version, want := range cases {
		if got := nestsSubagentCards(version); got != want {
			t.Errorf("nestsSubagentCards(%q) = %v, want %v", version, got, want)
		}
	}
}

// The briefing points Claude Code at its own Agent tool only once the
// installed plugin is a release whose mod turns subagents into lich cards.
func TestRunsSubagentCardsFromTheFirstCardRelease(t *testing.T) {
	cases := map[string]bool{
		"0.14.0":       false,
		"0.15.0-rc.1":  false,
		"0.15.0":       true,
		"0.15.3":       true,
		"1.0.0":        true,
		"not-a-semver": false,
	}
	for version, want := range cases {
		if got := runsSubagentCards(version); got != want {
			t.Errorf("runsSubagentCards(%q) = %v, want %v", version, got, want)
		}
	}
}

package project

import "testing"

// TestIsFailureConclusion pins every conclusion gh reports, both halves of the
// list. It is an allow-list of failures, so a literal dropped from it does not
// break a build — it turns a broken check green on the panel, and a merge
// affordance opens on a red pull request.
func TestIsFailureConclusion(t *testing.T) {
	tests := map[string]bool{
		"FAILURE":         true,
		"TIMED_OUT":       true,
		"CANCELLED":       true,
		"ACTION_REQUIRED": true,
		"STARTUP_FAILURE": true,
		"STALE":           true,
		"SUCCESS":         false,
		"NEUTRAL":         false,
		"SKIPPED":         false,
		// gh shouts its conclusions; a lowercase one is not one of them, and
		// matching it would be matching something gh never sends.
		"failure": false,
		"":        false,
	}
	for conclusion, want := range tests {
		t.Run(conclusion, func(t *testing.T) {
			if got := isFailureConclusion(conclusion); got != want {
				t.Errorf("isFailureConclusion(%q) = %v, want %v", conclusion, got, want)
			}
		})
	}
}

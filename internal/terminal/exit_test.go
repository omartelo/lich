package terminal

import "testing"

// TestExitPayloadReportsOnlyAnObservedStatus pins the one thing the payload must
// never do: turn "the child left no exit status" into a clean 0. The window
// reads a code as the reason a session ended, so an invented one would explain a
// crash as a successful exit.
func TestExitPayloadReportsOnlyAnObservedStatus(t *testing.T) {
	tests := []struct {
		name string
		code int
		want *int
	}{
		{"clean exit", 0, ptr(0)},
		{"failure", 3, ptr(3)},
		{"no status", -1, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := exitPayload(tt.code).Code
			if tt.want == nil {
				if got != nil {
					t.Fatalf("Code = %d, want absent", *got)
				}
				return
			}
			if got == nil {
				t.Fatalf("Code absent, want %d", *tt.want)
			}
			if *got != *tt.want {
				t.Errorf("Code = %d, want %d", *got, *tt.want)
			}
		})
	}
}

func ptr(v int) *int { return &v }

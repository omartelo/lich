package relay

import "testing"

// These cases are the Go half of frontend/src/lib/session/peer-name.test.ts,
// deliberately the same inputs and the same expected strings. The two
// implementations address the same sessions, so a divergence here is a message
// delivered to the wrong terminal — the one failure this feature must not have.
func TestRosterNameOfMatchesTheFrontend(t *testing.T) {
	tests := []struct {
		name string
		cwd  string
		id   string
		want string
	}{
		{
			name: "directory and id tail",
			cwd:  "/home/me/code/lich",
			id:   "4f2a1b3c-0000-4000-8000-000000000000",
			want: "lich-4f2a",
		},
		{
			name: "trailing separator ignored",
			cwd:  "/home/me/code/lich/",
			id:   "4f2a1b3c",
			want: "lich-4f2a",
		},
		{
			name: "windows path",
			cwd:  `C:\Users\me\code\lich`,
			id:   "4f2a1b3c",
			want: "lich-4f2a",
		},
		{
			name: "no directory to name",
			cwd:  "",
			id:   "4f2a1b3c",
			want: "lich-4f2a",
		},
		{
			name: "root is not a name",
			cwd:  "/",
			id:   "4f2a1b3c",
			want: "lich-4f2a",
		},
		{
			name: "id carries nothing usable",
			cwd:  "/home/me/code/lich",
			id:   "---",
			want: "lich",
		},
		{
			name: "punctuation inside the id is skipped, not counted",
			cwd:  "/home/me/code/lich",
			id:   "4-f-2-a-1",
			want: "lich-4f2a",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RosterName(tt.cwd, tt.id); got != tt.want {
				t.Errorf("RosterName(%q, %q) = %q, want %q", tt.cwd, tt.id, got, tt.want)
			}
		})
	}
}

func TestRosterNameSeparatesSessionsSharingACheckout(t *testing.T) {
	const cwd = "/home/me/code/lich"
	first := RosterName(cwd, "4f2a1b3c-0000-4000-8000-000000000000")
	second := RosterName(cwd, "9d8e7f6a-0000-4000-8000-000000000000")

	if first == second {
		t.Fatalf("two sessions in one checkout share the name %q", first)
	}
}

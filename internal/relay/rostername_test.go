package relay

import "testing"

// TestRosterNameOf pins the choice between the two names a session can be
// addressed by: what its agent has on record beats what lich can derive, every
// time. The derivation is only ever the answer when there is nothing to read —
// and a name that is only whitespace is nothing to read.
func TestRosterNameOf(t *testing.T) {
	tests := []struct {
		name     string
		recorded string
		want     string
	}{
		{name: "the recorded name wins", recorded: "reviewer", want: "reviewer"},
		{name: "trimmed on the way through", recorded: "  reviewer\n", want: "reviewer"},
		{name: "nothing recorded falls back", recorded: "", want: "lich-4f2a"},
		{name: "whitespace is nothing recorded", recorded: "   ", want: "lich-4f2a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RosterNameOf(tt.recorded, "/home/me/code/lich", "4f2a1b3c")
			if got != tt.want {
				t.Errorf("RosterNameOf(%q, …) = %q, want %q", tt.recorded, got, tt.want)
			}
		})
	}
}

// TestRosterNameDerivesTheBirthName pins the string lich gives a session that
// has nothing on record yet — the one it is spawned under, and the one the
// roster falls back to.
func TestRosterNameDerivesTheBirthName(t *testing.T) {
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
			name: "here is not a name either",
			cwd:  ".",
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

package browser

import "testing"

func TestKeySequenceAcceptsNamedKeysAndAliases(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"Enter", "\r"},
		{"return", "\r"},
		{"Tab", "\t"},
		{"Esc", "\u001b"},
		{"Escape", "\u001b"},
		{"Arrow-Down", "\u0301"},
		{"arrow_up", "\u0304"},
		{"Space", " "},
		{"  ENTER  ", "\r"},
	}
	for _, c := range cases {
		got, err := keySequence(c.in)
		if err != nil {
			t.Fatalf("%q: %v", c.in, err)
		}
		if got != c.want {
			t.Fatalf("%q → %q, want %q", c.in, got, c.want)
		}
	}
}

func TestKeySequenceRejectsUnknownAndEmpty(t *testing.T) {
	if _, err := keySequence(""); err == nil {
		t.Fatal("empty key was allowed")
	}
	if _, err := keySequence("Ctrl+C"); err == nil {
		t.Fatal("chord was allowed")
	}
	if _, err := keySequence("hello"); err == nil {
		t.Fatal("text was allowed through press")
	}
}

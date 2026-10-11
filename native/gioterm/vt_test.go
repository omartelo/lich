package gioterm

import (
	"testing"
)

func TestEncodeCtrlLetter(t *testing.T) {
	v, err := New(80, 24, func([]byte) {})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name  string
		setup string
		want  string
	}{
		{"legacy", "", "\x04"},
		// nvim turns on kitty "disambiguate" (CSI > 1 u) once the terminal answers its query.
		{"kitty disambiguate", "\x1b[>1u", "\x1b[100;5u"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.setup != "" {
				v.Write([]byte(c.setup))
			}
			got := string(v.encodeKey(keyA+3, modCtrl, 'd', "d"))
			if got != c.want {
				t.Fatalf("Ctrl+D encoded as %q, want %q", got, c.want)
			}
		})
	}
}

func TestEncodeSpecialKeysUnderKitty(t *testing.T) {
	v, err := New(80, 24, func([]byte) {})
	if err != nil {
		t.Fatal(err)
	}
	v.Write([]byte("\x1b[>1u"))
	cases := []struct {
		name string
		key  int
		want string
	}{
		{"escape", keyEscape, "\x1b[27u"},
		{"enter", keyEnter, "\r"},
		{"backspace", keyBackspace, "\x7f"},
		{"up", keyUp, "\x1b[A"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := string(v.encodeKey(c.key, 0, 0, "")); got != c.want {
				t.Fatalf("encoded as %q, want %q", got, c.want)
			}
		})
	}
}

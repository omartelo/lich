package gioterm

import (
	"testing"

	"gioui.org/io/key"
)

func TestEncodeKeyEvents(t *testing.T) {
	v, err := New(80, 24, func([]byte) {})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		ev   key.Event
		want string
	}{
		{"ctrl+d", key.Event{Name: "D", Modifiers: key.ModCtrl}, "\x04"},
		{"ctrl+u", key.Event{Name: "U", Modifiers: key.ModCtrl}, "\x15"},
		{"ctrl+backspace", key.Event{Name: key.NameDeleteBackward, Modifiers: key.ModCtrl}, "\x08"},
		{"alt+backspace", key.Event{Name: key.NameDeleteBackward, Modifiers: key.ModAlt}, "\x1b\x7f"},
		{"alt+a", key.Event{Name: "A", Modifiers: key.ModAlt}, "\x1ba"},
		{"alt+shift+a", key.Event{Name: "A", Modifiers: key.ModAlt | key.ModShift}, "\x1bA"},
		{"alt+.", key.Event{Name: ".", Modifiers: key.ModAlt}, "\x1b."},
		// Ghostty, like kitty, leaves Ctrl+[ out of the C0 table (key_encode.zig ctrlSeq).
		{"ctrl+[", key.Event{Name: "[", Modifiers: key.ModCtrl}, "\x1b[91;5u"},
		{"ctrl+/", key.Event{Name: "/", Modifiers: key.ModCtrl}, "\x1f"},
		{"ctrl+space", key.Event{Name: key.NameSpace, Modifiers: key.ModCtrl}, "\x00"},
		{"f5", key.Event{Name: key.NameF5}, "\x1b[15~"},
		{"shift+tab", key.Event{Name: key.NameTab, Modifiers: key.ModShift}, "\x1b[Z"},
		// Plain text arrives as an EditEvent; encoding it here too would type it twice.
		{"plain a", key.Event{Name: "A"}, ""},
		{"plain space", key.Event{Name: key.NameSpace}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := string(v.HandleKey(c.ev)); got != c.want {
				t.Fatalf("encoded as %q, want %q", got, c.want)
			}
		})
	}
}

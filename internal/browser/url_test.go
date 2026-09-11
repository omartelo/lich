package browser

import "testing"

func TestAllowURL(t *testing.T) {
	cases := []struct {
		in, want, err string
	}{
		{"", "about:blank", ""},
		{"about:blank", "about:blank", ""},
		{"  ABOUT:BLANK  ", "about:blank", ""},
		{"https://example.com/path", "https://example.com/path", ""},
		{"http://localhost:3000", "http://localhost:3000", ""},
		{"file:///etc/passwd", "", `blocked url scheme "file"`},
		{"javascript:alert(1)", "", `blocked url scheme "javascript"`},
		{"data:text/html,hi", "", `blocked url scheme "data"`},
		{"chrome://settings", "", `blocked url scheme "chrome"`},
		{"example.com", "", `url "example.com" is missing a scheme`},
		{"https://", "", `url "https://" has no host`},
	}
	for _, c := range cases {
		got, err := allowURL(c.in)
		if c.err != "" {
			if err == nil || err.Error() != c.err {
				t.Errorf("allowURL(%q) err = %v, want %s", c.in, err, c.err)
			}
			continue
		}
		if err != nil {
			t.Errorf("allowURL(%q): %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("allowURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

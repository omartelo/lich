package system

import "testing"

func TestValidateExternalURL(t *testing.T) {
	valid := []string{"http://example.com", "https://github.com/omartelo/lich/pull/1"}
	for _, u := range valid {
		if err := ValidateExternalURL(u); err != nil {
			t.Fatalf("want %q accepted: %v", u, err)
		}
	}
	invalid := []string{
		"file:///etc/passwd",
		"javascript:alert(1)",
		"vscode://open",
		"https://",
		"not a url at all\x00",
		"",
	}
	for _, u := range invalid {
		if err := ValidateExternalURL(u); err == nil {
			t.Fatalf("want %q rejected", u)
		}
	}
}

func TestOpenExternalGatesBeforeLaunching(t *testing.T) {
	launched := ""
	s := &Service{open: func(target string) error {
		launched = target
		return nil
	}}
	if err := s.OpenExternal("file:///etc/passwd"); err == nil || launched != "" {
		t.Fatalf("invalid url launched: %q (%v)", launched, err)
	}
	if err := s.OpenExternal("https://example.com"); err != nil || launched != "https://example.com" {
		t.Fatalf("valid url not launched: %q (%v)", launched, err)
	}
}

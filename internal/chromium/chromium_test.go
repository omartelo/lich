package chromium

import (
	"slices"
	"testing"
)

// TestArgs pins the argv contract with shell/src/main.rs: the page, the
// profile, the class, the stdin switch, and the user's own switches last.
func TestArgs(t *testing.T) {
	args := Args("http://127.0.0.1:47821/?token=x", "/home/u/.config/lich/chromium-profile", "lichdev", []string{"--ozone-platform=wayland"})
	want := []string{
		"--url=http://127.0.0.1:47821/?token=x",
		"--user-data-dir=/home/u/.config/lich/chromium-profile",
		"--class=lichdev",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-features=Translate",
		"--exit-on-stdin-eof",
		"--ozone-platform=wayland",
	}
	if !slices.Equal(args, want) {
		t.Fatalf("Args = %v, want %v", args, want)
	}
}

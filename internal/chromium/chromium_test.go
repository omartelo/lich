package chromium

import (
	"errors"
	"slices"
	"testing"
)

func TestFindBrowserPicksFirstHit(t *testing.T) {
	lookPath := func(name string) (string, error) {
		if name == "google-chrome-stable" || name == "brave" {
			return "/usr/bin/" + name, nil
		}
		return "", errors.New("not found")
	}
	got, err := FindBrowser(lookPath)
	if err != nil {
		t.Fatalf("FindBrowser: %v", err)
	}
	if got != "/usr/bin/google-chrome-stable" {
		t.Fatalf("want first candidate in preference order, got %q", got)
	}
}

func TestFindBrowserErrorsWhenNoneInstalled(t *testing.T) {
	lookPath := func(string) (string, error) { return "", errors.New("not found") }
	if _, err := FindBrowser(lookPath); err == nil {
		t.Fatal("want error when no browser is on PATH")
	}
}

func TestArgs(t *testing.T) {
	args := Args("http://127.0.0.1:47821/?token=x", "/home/u/.config/lich/chromium-profile", "lichdev", []string{"--ozone-platform=wayland"})
	for _, want := range []string{
		"--app=http://127.0.0.1:47821/?token=x",
		"--user-data-dir=/home/u/.config/lich/chromium-profile",
		"--class=lichdev",
		"--ozone-platform=wayland",
	} {
		if !slices.Contains(args, want) {
			t.Fatalf("missing %q in %v", want, args)
		}
	}
	if args[len(args)-1] != "--ozone-platform=wayland" {
		t.Fatalf("extra args must come last: %v", args)
	}
}

package main

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
	got, err := findBrowser(lookPath)
	if err != nil {
		t.Fatalf("findBrowser: %v", err)
	}
	if got != "/usr/bin/google-chrome-stable" {
		t.Fatalf("want first candidate in preference order, got %q", got)
	}
}

func TestFindBrowserErrorsWhenNoneInstalled(t *testing.T) {
	lookPath := func(string) (string, error) { return "", errors.New("not found") }
	if _, err := findBrowser(lookPath); err == nil {
		t.Fatal("want error when no browser is on PATH")
	}
}

func TestBrowserArgs(t *testing.T) {
	args := browserArgs("http://127.0.0.1:1234/spike.html?token=x", "/tmp/prof", []string{"--ozone-platform=wayland"})
	for _, want := range []string{
		"--app=http://127.0.0.1:1234/spike.html?token=x",
		"--user-data-dir=/tmp/prof",
		"--class=lich-spike",
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

func TestParseControl(t *testing.T) {
	msg, err := parseControl([]byte(`{"t":"in","d":"ls\r"}`))
	if err != nil || msg.T != "in" || msg.D != "ls\r" {
		t.Fatalf("input frame: %+v, %v", msg, err)
	}
	msg, err = parseControl([]byte(`{"t":"rs","c":120,"r":40}`))
	if err != nil || msg.Cols != 120 || msg.Rows != 40 {
		t.Fatalf("resize frame: %+v, %v", msg, err)
	}
	if _, err := parseControl([]byte(`{"t":"nope"}`)); err == nil {
		t.Fatal("want error on unknown type")
	}
	if _, err := parseControl([]byte(`garbage`)); err == nil {
		t.Fatal("want error on invalid json")
	}
}

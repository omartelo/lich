package main

import (
	"errors"
	"fmt"
)

// Chromium-family binaries, in preference order. Any of them gives the same
// compositor the spike exists to measure.
var browserCandidates = []string{
	"chromium",
	"chromium-browser",
	"google-chrome-stable",
	"google-chrome",
	"brave",
}

// findBrowser returns the first Chromium-family binary on PATH. lookPath is
// injectable for tests (production passes exec.LookPath).
func findBrowser(lookPath func(name string) (string, error)) (string, error) {
	for _, name := range browserCandidates {
		if path, err := lookPath(name); err == nil {
			return path, nil
		}
	}
	return "", errors.New("no chromium-family browser found on PATH (tried " +
		fmt.Sprint(browserCandidates) + "); install chromium or run with -no-browser")
}

// browserArgs builds the --app invocation. The isolated user-data-dir is
// load-bearing: without it Chromium adopts the window into an already-running
// instance and the spawned process exits immediately, breaking lifecycle.
func browserArgs(url, dataDir string, extra []string) []string {
	args := []string{
		"--app=" + url,
		"--user-data-dir=" + dataDir,
		"--class=lich-spike",
		"--no-first-run",
		"--no-default-browser-check",
	}
	return append(args, extra...)
}

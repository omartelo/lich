package providers

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestRefreshPathLetsDetectSeeANewDirectory is the trap the re-check exists to
// close: an agent installed into a directory the launch PATH never carried is
// invisible to every scan until the pin moves. Detection here runs the real
// exec.LookPath, so what it proves is what the app does.
func TestRefreshPathLetsDetectSeeANewDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a bare `claude` file is not an executable on Windows")
	}
	late := t.TempDir()
	t.Setenv("PATH", t.TempDir())
	svc := New()
	// Stands in for main.go's resolution: the login shell hands back a PATH
	// carrying the new directory, and it is pinned the same way.
	svc.SetPathRefresh(func() error { return os.Setenv("PATH", late) })

	if installed(t, svc, Claude) {
		t.Fatal("claude found before it was installed")
	}
	writeExecutable(t, filepath.Join(late, "claude"))
	if installed(t, svc, Claude) {
		t.Fatal("claude found before the PATH was re-read")
	}

	if err := svc.RefreshPath(); err != nil {
		t.Fatalf("RefreshPath: %v", err)
	}

	if !installed(t, svc, Claude) {
		t.Fatal("claude still missing after the PATH was re-read")
	}
	if check := svc.Verify("claude"); check.Status != CheckOK {
		t.Fatalf("Verify after the re-read = %q, want %q", check.Status, CheckOK)
	}
}

// TestRefreshPathFailureKeepsThePin: a login shell that will not answer must
// leave every scan resolving through the PATH lich booted with, and say so —
// silently re-scanning the old one reports a stale absence as a fresh one.
func TestRefreshPathFailureKeepsThePin(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a bare `claude` file is not an executable on Windows")
	}
	boot := t.TempDir()
	writeExecutable(t, filepath.Join(boot, "claude"))
	t.Setenv("PATH", boot)
	svc := New()
	svc.SetPathRefresh(func() error { return errors.New("shell never answered") })

	if err := svc.RefreshPath(); err == nil {
		t.Fatal("RefreshPath: want the resolution's failure")
	}
	if os.Getenv("PATH") != boot {
		t.Fatalf("PATH = %q, want it untouched", os.Getenv("PATH"))
	}
	if !installed(t, svc, Claude) {
		t.Fatal("the pinned PATH stopped resolving after a failed re-read")
	}
}

// TestRefreshPathWithoutRefresher covers the Service `lich doctor` builds: it
// reads the machine rather than changing it, so nothing is wired and the call
// is the scan of the same PATH it always was.
func TestRefreshPathWithoutRefresher(t *testing.T) {
	if err := New().RefreshPath(); err != nil {
		t.Fatalf("RefreshPath with nothing wired = %v, want nil", err)
	}
}

func installed(t *testing.T, svc *Service, id string) bool {
	t.Helper()
	found, err := svc.Detect()
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	for _, d := range found {
		if d.ID == id {
			return d.Installed
		}
	}
	t.Fatalf("Detect returned no %s row", id)
	return false
}

func writeExecutable(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

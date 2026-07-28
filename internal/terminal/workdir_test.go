package terminal

import (
	"os"
	"path/filepath"
	"testing"
)

// TestWorkdirMissing pins the asymmetry the doc promises: only a provable
// absence answers true, because a false answer costs a spawn that reports its
// own failure while a true one closes the user's session.
func TestWorkdirMissing(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	svc := &Service{}
	tests := []struct {
		name string
		cwd  string
		want bool
	}{
		{"existing directory", dir, false},
		{"empty cwd resolves to home", "", false},
		{"deleted checkout", filepath.Join(dir, "gone"), true},
		{"path is a file", file, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := svc.WorkdirMissing(tt.cwd); got != tt.want {
				t.Errorf("WorkdirMissing(%q) = %v, want %v", tt.cwd, got, tt.want)
			}
		})
	}
}

// The suite spawns a real PTY and reuses stubBins from terminal_test.go, which
// is Unix-tagged for the same reason; this file carries the tag so the package
// still builds on Windows.
//go:build !windows

package terminal

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/omartelo/lich/internal/events"
	"github.com/omartelo/lich/internal/providers"
)

// The spawn is what resolves the list, so a server registered while lich was
// already open reaches the next session started after it — which is the whole
// reason this is not read once at startup.
func TestSpawnRecordsTheMCPServersItCanReach(t *testing.T) {
	t.Setenv("SHELL", "sh")
	t.Setenv("OPENCODE_CONFIG_DIR", t.TempDir())
	cwd := t.TempDir()
	config := `{"mcp":{"registered-after-launch":{}}}`
	if err := os.WriteFile(filepath.Join(cwd, "opencode.json"), []byte(config), 0o644); err != nil {
		t.Fatalf("write opencode.json: %v", err)
	}
	recorded := map[string][]string{}
	svc := New(stubBins{bin: "sh", mcpServers: recorded}, nil, events.New())
	t.Cleanup(func() { _ = svc.Close("s1") })

	if err := svc.Start("s1", "p1", cwd, providers.OpenCode, "", "", false, false, 80, 24); err != nil {
		t.Fatalf("Start = %v, want nil", err)
	}

	want := []string{"lich", "registered-after-launch"}
	if got := recorded["s1"]; !slices.Equal(got, want) {
		t.Errorf("recorded %v, want %v", got, want)
	}
}

package sandbox

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/omartelo/lich/internal/providers"
)

// clearHarnessEnv blanks every variable a provider uses to move its state
// directory, so a test answers for the fallback paths rather than for whatever
// the machine running it has exported.
func clearHarnessEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"CLAUDE_CONFIG_DIR", "CODEX_HOME", "OMP_PROFILE", "PI_CODING_AGENT_DIR",
		"XDG_CONFIG_HOME", "XDG_DATA_HOME",
	} {
		t.Setenv(name, "")
	}
}

func TestStateDirsCoversEveryRegisteredProvider(t *testing.T) {
	clearHarnessEnv(t)
	for _, p := range providers.Registry {
		if got := stateDirs(p.ID, "/home/u"); len(got) == 0 {
			t.Errorf("provider %q has no state directory: a confined session of it cannot authenticate", p.ID)
		}
	}
}

func TestStateDirsPerProvider(t *testing.T) {
	clearHarnessEnv(t)
	tests := []struct {
		provider string
		want     []string
	}{
		{providers.Claude, []string{"/home/u/.claude", "/home/u/.claude.json"}},
		{providers.Codex, []string{"/home/u/.codex"}},
		{providers.OMP, []string{"/home/u/.omp/agent"}},
		{providers.OpenCode, []string{"/home/u/.config/opencode", "/home/u/.local/share/opencode"}},
		{providers.Crush, []string{"/home/u/.config/crush", "/home/u/.local/share/crush"}},
	}
	for _, tt := range tests {
		if got := stateDirs(tt.provider, "/home/u"); !slices.Equal(got, tt.want) {
			t.Errorf("stateDirs(%q) = %v, want %v", tt.provider, got, tt.want)
		}
	}
	if got := stateDirs("shell", "/home/u"); got != nil {
		t.Errorf("stateDirs for a shell session = %v, want none", got)
	}
}

func TestStateDirsFollowsHarnessEnvironment(t *testing.T) {
	clearHarnessEnv(t)
	t.Setenv("CLAUDE_CONFIG_DIR", "/srv/claude")
	t.Setenv("CODEX_HOME", "/srv/codex")
	t.Setenv("XDG_CONFIG_HOME", "/srv/cfg")

	if got := stateDirs(providers.Claude, "/home/u"); got[0] != "/srv/claude" {
		t.Errorf("CLAUDE_CONFIG_DIR ignored: got %v", got)
	}
	if got := stateDirs(providers.Codex, "/home/u"); got[0] != "/srv/codex" {
		t.Errorf("CODEX_HOME ignored: got %v", got)
	}
	if got := stateDirs(providers.Crush, "/home/u"); got[0] != "/srv/cfg/crush" {
		t.Errorf("XDG_CONFIG_HOME ignored: got %v", got)
	}
}

// A named omp profile wins over the explicit directory override, which is the
// order `omp config path` applies.
func TestOMPProfileBeatsDirectoryOverride(t *testing.T) {
	clearHarnessEnv(t)
	t.Setenv("PI_CODING_AGENT_DIR", "/srv/omp")
	if got := ompAgentDir("/home/u"); got != "/srv/omp" {
		t.Errorf("PI_CODING_AGENT_DIR ignored: got %q", got)
	}
	t.Setenv("OMP_PROFILE", "work")
	want := "/home/u/.omp/profiles/work/agent"
	if got := ompAgentDir("/home/u"); got != want {
		t.Errorf("ompAgentDir with a profile = %q, want %q", got, want)
	}
}

// A relative override is not a bind mount source: it resolves against a working
// directory this package does not own, so the fallback stands.
func TestEnvDirRejectsRelativeOverride(t *testing.T) {
	t.Setenv("CODEX_HOME", "codex")
	if got := envDir("CODEX_HOME", "/home/u/.codex"); got != "/home/u/.codex" {
		t.Errorf("relative override honoured: got %q", got)
	}
}

func TestDescribeDropsMissingPathsAndDuplicates(t *testing.T) {
	clearHarnessEnv(t)
	home := t.TempDir()
	cwd := filepath.Join(home, "checkout")
	for _, dir := range []string{".claude", ".config", ".cache", "checkout"} {
		if err := os.MkdirAll(filepath.Join(home, dir), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	// Listed twice on purpose: a caller naming the provider's own state
	// directory as an extra read must not turn it into two mounts.
	spec := Describe(providers.Claude, home, cwd, "", []string{
		filepath.Join(home, ".config"),
		filepath.Join(home, "nonexistent"),
	})

	if !slices.Contains(spec.Write, filepath.Join(home, ".claude")) {
		t.Errorf("provider state missing from Write: %v", spec.Write)
	}
	if slices.Contains(spec.Write, filepath.Join(home, ".claude.json")) {
		t.Errorf("a state path that is not on disk was kept: %v", spec.Write)
	}
	if !slices.Contains(spec.Write, filepath.Join(home, ".cache")) {
		t.Errorf("build cache missing from Write: %v", spec.Write)
	}
	for _, path := range spec.Read {
		if path == filepath.Join(home, "nonexistent") {
			t.Errorf("a path that is not on disk was kept: %v", spec.Read)
		}
	}
	if n := count(spec.Read, filepath.Join(home, ".config")); n != 1 {
		t.Errorf("~/.config mounted %d times, want 1: %v", n, spec.Read)
	}
	if spec.Home != home || spec.Cwd != cwd {
		t.Errorf("Describe = home %q cwd %q, want %q and %q", spec.Home, spec.Cwd, home, cwd)
	}
}

// The material the sandbox exists to keep away from an unattended agent is
// named nowhere in the tables: not writable, not readable, not there.
func TestSecretsAreInNoTable(t *testing.T) {
	for _, secret := range []string{".ssh", ".aws", ".gnupg", ".kube", ".netrc", ".docker"} {
		if slices.Contains(writableCaches, secret) {
			t.Errorf("%q is writable inside the sandbox", secret)
		}
		if slices.Contains(readableToolchain, secret) {
			t.Errorf("%q is readable inside the sandbox", secret)
		}
	}
}

func count(haystack []string, needle string) int {
	n := 0
	for _, item := range haystack {
		if item == needle {
			n++
		}
	}
	return n
}

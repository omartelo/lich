package terminal

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/omartelo/lich/internal/providers"
)

// The Kiro CLI side of a spawn: which agent it is told to run, and the shape of
// the command line that carries it. The readout tests are in usage_kiro_test.go.

// kiroHome redirects the home Kiro's directories hang off — it answers to no
// environment variable of its own — and returns it. The resolved path is
// asserted to be under the redirect before anything is written: a variable this
// misses on some platform would leave the test writing an agent into the real
// user's ~/.kiro and passing while it did.
func kiroHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	got, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("resolve home directory: %v", err)
	}
	if got != home {
		t.Fatalf("home resolved to %q, outside the test's temp dir %q", got, home)
	}
	return home
}

// TestKiroPluginAgentNamesTheAgentOnlyOnceInstalled proves the spawn asks for
// `--agent lich` exactly when there is one to ask for. Both directions cost
// something real: without the agent every report goes missing, and naming one
// Kiro cannot find puts a fallback warning on the first line of every session a
// user who never installed the plugin opens.
func TestKiroPluginAgentNamesTheAgentOnlyOnceInstalled(t *testing.T) {
	home := kiroHome(t)

	if got := kiroPluginAgent(providers.Kiro); got != "" {
		t.Errorf("kiroPluginAgent with nothing installed = %q, want \"\"", got)
	}

	dir := filepath.Join(home, ".kiro", "agents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "lich.json"), []byte(`{"name":"lich"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := kiroPluginAgent(providers.Kiro); got != providers.KiroAgentName {
		t.Errorf("kiroPluginAgent with the plugin installed = %q, want %q", got, providers.KiroAgentName)
	}
}

// TestKiroPluginAgentIsNeverAskedForByAnotherProvider proves the flag stays
// Kiro's. `--agent` means something else to Antigravity and nothing at all to a
// shell, so a lookup that answered on kind alone would hand one provider
// another's flag — the failure skipPermissionFlags is pinned against.
func TestKiroPluginAgentIsNeverAskedForByAnotherProvider(t *testing.T) {
	home := kiroHome(t)
	dir := filepath.Join(home, ".kiro", "agents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "lich.json"), []byte(`{"name":"lich"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{providers.Claude, providers.Antigravity, providers.Cursor, KindShell} {
		if got := kiroPluginAgent(kind); got != "" {
			t.Errorf("kiroPluginAgent(%q) = %q, want \"\"", kind, got)
		}
	}
}

// TestAgentArgsIsKiroOnly is the spawn side of the same rule, and pins that an
// unusable name is dropped rather than passed on: a value starting with a dash
// would be read by Kiro as another flag.
func TestAgentArgsIsKiroOnly(t *testing.T) {
	tests := []struct {
		name, kind, agent string
		want              []string
	}{
		{"kiro with the plugin", providers.Kiro, "lich", []string{"--agent", "lich"}},
		{"kiro without it", providers.Kiro, "", nil},
		{"a name that reads as a flag", providers.Kiro, "--dangerous", nil},
		{"claude never gets one", providers.Claude, "lich", nil},
		{"antigravity never gets one", providers.Antigravity, "lich", nil},
		{"a shell never gets one", KindShell, "lich", nil},
	}
	for _, tt := range tests {
		if got := agentArgs(tt.kind, tt.agent); !slices.Equal(got, tt.want) {
			t.Errorf("%s: agentArgs(%q, %q) = %v, want %v", tt.name, tt.kind, tt.agent, got, tt.want)
		}
	}
}

// TestKiroSpawnsTheChatSubcommandBeforeEveryFlag pins the split that a table of
// flags alone cannot see: `kiro-cli`'s root parser takes --agent and
// --resume-id, but --model and --trust-all-tools belong to its `chat`
// subcommand, and either one handed to the root exits 2 with `unexpected
// argument` before the session exists (measured on 2.21.0). So the subcommand is
// not conditional on which settings a session happens to carry — it is argv[0]
// on every Kiro spawn, ahead of all of them.
func TestKiroSpawnsTheChatSubcommandBeforeEveryFlag(t *testing.T) {
	tests := []struct {
		name            string
		agent           string
		resume          string
		model           string
		skipPermissions bool
		want            []string
	}{
		{name: "a bare session", want: []string{"chat"}},
		{name: "with the plugin installed", agent: "lich", want: []string{"chat", "--agent", "lich"}},
		{name: "resuming a conversation", agent: "lich", resume: "s1",
			want: []string{"chat", "--agent", "lich", "--resume-id", "s1"}},
		// The two that only `chat` accepts, each on its own: either one is a
		// dead spawn when the subcommand goes missing.
		{name: "on a chosen model", model: "auto", want: []string{"chat", "--model", "auto"}},
		{name: "with permissions skipped", skipPermissions: true,
			want: []string{"chat", "--trust-all-tools"}},
		{name: "everything at once", agent: "lich", resume: "s1", model: "auto", skipPermissions: true,
			want: []string{"chat", "--agent", "lich", "--resume-id", "s1", "--trust-all-tools", "--model", "auto"}},
	}
	for _, tt := range tests {
		got := providerArgs(
			providers.Kiro, "", tt.resume, tt.model, "/usr/bin/lich", tt.agent,
			false, tt.skipPermissions,
		)
		if !slices.Equal(got, tt.want) {
			t.Errorf("%s: args = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// TestSubcommandArgsIsKiroOnly proves no other provider grew one. A word at
// argv[0] that the harness has no subcommand for is a spawn that dies before the
// session exists, which is the same failure from the other direction.
func TestSubcommandArgsIsKiroOnly(t *testing.T) {
	for _, kind := range []string{
		providers.Claude, providers.Codex, providers.Antigravity, providers.OpenCode,
		providers.OMP, providers.Crush, providers.Cursor, KindShell,
	} {
		if got := subcommandArgs(kind); got != nil {
			t.Errorf("subcommandArgs(%q) = %v, want nil", kind, got)
		}
	}
}

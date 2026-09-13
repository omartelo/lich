package agentplugin

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/omartelo/lich/internal/providers"
	"github.com/omartelo/lich/internal/relay"
)

const (
	staleLich = "/gone/lich"
	freshLich = "/opt/new/lich"
)

// repairHome isolates every document the repair reads — omp's, Cursor's, Kiro's
// and Antigravity's all hang off the home — and routes each CLI call to the
// fake one, so neither a real config nor a real provider binary is reached.
func repairHome(t *testing.T) (*Service, string, func() []string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("OMP_PROFILE", "")
	t.Setenv("PI_CODING_AGENT_DIR", "")
	t.Setenv(devEnv, "")
	if dir, err := os.UserHomeDir(); err != nil || dir != home {
		t.Fatalf("home redirect missed: %q (err %v)", dir, err)
	}
	s := &Service{lookPath: func(name string) (string, error) { return name, nil }}
	calls := antigravityCLI(t, s)
	s.lichBin = func() string { return freshLich }
	return s, home, calls
}

func writeDoc(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func lichCommandIn(t *testing.T, path string) string {
	t.Helper()
	var doc struct {
		Servers map[string]struct {
			Command string `json:"command"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(readFile(t, path)), &doc); err != nil {
		t.Fatalf("%s is not JSON: %v", path, err)
	}
	return doc.Servers[relay.MCPServerName].Command
}

func staleDoc(command string) string {
	return `{"mcpServers":{
		"notes":{"command":"notes-server","args":["--stdio"]},
		"lich":{"command":"` + command + `","args":["mcp"]}}}`
}

// The two documents lich merges itself are rewritten in place, and every server
// that is not lich's survives the rewrite.
func TestRepairRepointsTheDocumentsLichWrites(t *testing.T) {
	s, home, _ := repairHome(t)
	docs := map[string]string{
		providers.OMP:    filepath.Join(home, ".omp", "agent", ompMCPFile),
		providers.Cursor: filepath.Join(home, ".cursor", cursorMCPFile),
	}
	for _, path := range docs {
		writeDoc(t, path, staleDoc(staleLich))
	}

	s.RepairRegistrations()

	for provider, path := range docs {
		if got := lichCommandIn(t, path); got != freshLich {
			t.Errorf("%s: lich command = %q, want %q", provider, got, freshLich)
		}
		if !strings.Contains(readFile(t, path), "notes-server") {
			t.Errorf("%s: the user's other server was dropped:\n%s", provider, readFile(t, path))
		}
	}
}

// Kiro and Antigravity own their documents through their CLIs, so the repair is
// the install's `mcp add`, run once each and naming this binary.
func TestRepairRepointsThroughTheCLIs(t *testing.T) {
	s, home, calls := repairHome(t)
	writeDoc(t, filepath.Join(home, ".kiro", "agents", providers.KiroAgentName+".json"), staleDoc(staleLich))
	writeDoc(t, filepath.Join(home, ".gemini", "config", "mcp_config.json"), staleDoc(staleLich))

	s.RepairRegistrations()

	got := calls()
	for _, want := range []string{
		"mcp add --name lich --command " + freshLich + " --args mcp --agent lich",
		"mcp add lich " + freshLich + " mcp",
	} {
		if !slices.Contains(got, want) {
			t.Errorf("calls = %q, want one to be %q", got, want)
		}
	}
}

// A registration the user deleted, one they disabled, and one already naming this
// binary are all left exactly as they are — and nothing is created where there
// was no document at all.
func TestRepairLeavesTheRestAlone(t *testing.T) {
	s, home, calls := repairHome(t)
	removed := filepath.Join(home, ".cursor", cursorMCPFile)
	writeDoc(t, removed, `{"mcpServers":{"notes":{"command":"notes-server"}}}`)
	disabled := filepath.Join(home, ".gemini", "config", "mcp_config.json")
	writeDoc(t, disabled, `{"mcpServers":{"lich":{"command":"`+staleLich+`","args":["mcp"],"disabled":true}}}`)
	current := filepath.Join(home, ".kiro", "agents", providers.KiroAgentName+".json")
	writeDoc(t, current, staleDoc(freshLich))
	before := map[string]string{}
	for _, p := range []string{removed, disabled, current} {
		before[p] = readFile(t, p)
	}

	s.RepairRegistrations()

	for p, body := range before {
		if got := readFile(t, p); got != body {
			t.Errorf("%s was rewritten:\n%s", p, got)
		}
	}
	if _, err := os.Stat(filepath.Join(home, ".omp", "agent", ompMCPFile)); !os.IsNotExist(err) {
		t.Errorf("omp's document was created (stat err = %v)", err)
	}
	if got := calls(); slices.ContainsFunc(got, func(c string) bool { return strings.HasPrefix(c, "mcp add") }) {
		t.Errorf("calls = %q, want no mcp add", got)
	}
}

// A registration naming a symlink to this binary is current: swapping the link
// for its target would pin the path a package manager moves on every update.
func TestRepairKeepsASymlinkToThisBinary(t *testing.T) {
	s, home, _ := repairHome(t)
	target := filepath.Join(t.TempDir(), "lich")
	writeDoc(t, target, "binary")
	link := filepath.Join(t.TempDir(), "lich")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable here: %v", err)
	}
	s.lichBin = func() string { return target }
	path := filepath.Join(home, ".cursor", cursorMCPFile)
	writeDoc(t, path, staleDoc(filepath.ToSlash(link)))
	before := readFile(t, path)

	s.RepairRegistrations()

	if got := readFile(t, path); got != before {
		t.Errorf("the symlinked registration was rewritten:\n%s", got)
	}
}

// A lich that cannot name itself has nothing to repoint to, and must not erase
// the registration it cannot replace.
func TestRepairWithoutABinaryTouchesNothing(t *testing.T) {
	s, home, _ := repairHome(t)
	s.lichBin = func() string { return "" }
	path := filepath.Join(home, ".cursor", cursorMCPFile)
	writeDoc(t, path, staleDoc(staleLich))

	s.RepairRegistrations()

	if got := lichCommandIn(t, path); got != staleLich {
		t.Errorf("lich command = %q, want it left at %q", got, staleLich)
	}
}

// Crush's registration is a line in lich's crushrc block: the repair rewrites
// that block at the version it already carries, and the user's own lines and a
// path the shell had to quote both survive.
func TestRepairRepointsCrushsBlock(t *testing.T) {
	s, _, _ := repairHome(t)
	configDir := t.TempDir()
	scripts := filepath.Join(lichConfigHome(t), "lich", "plugin", "hooks", providers.Crush)
	crushCLI(t, s, configDir, "0.88.1")
	old := "/opt/it's here/lich"
	rc := filepath.Join(configDir, "crushrc")
	writeDoc(t, rc, "set theme dark\n\n"+crushrcBlock(testVersion, scripts, old))

	if got, ok := s.crushRegisteredCommand(); !ok || got != old {
		t.Fatalf("crushRegisteredCommand = (%q, %v), want (%q, true)", got, ok, old)
	}
	s.RepairRegistrations()

	want := "set theme dark\n\n" + crushrcBlock(testVersion, scripts, freshLich)
	if got := readFile(t, rc); got != want {
		t.Errorf("crushrc =\n%s\nwant\n%s", got, want)
	}
}

// A crushrc block with no `mcp add` line is one whose registration was left out
// or taken out, and the repair does not put it back.
func TestRepairLeavesACrushBlockWithoutTheServer(t *testing.T) {
	s, _, _ := repairHome(t)
	configDir := t.TempDir()
	scripts := filepath.Join(lichConfigHome(t), "lich", "plugin", "hooks", providers.Crush)
	crushCLI(t, s, configDir, "0.88.1")
	rc := filepath.Join(configDir, "crushrc")
	body := crushrcBlock(testVersion, scripts, "")
	writeDoc(t, rc, body)

	s.RepairRegistrations()

	if got := readFile(t, rc); got != body {
		t.Errorf("crushrc was rewritten:\n%s", got)
	}
}

// A dev rig shares the installed lich's home, so it must not repoint the user's
// registrations at itself.
func TestRepairSkipsADevInstance(t *testing.T) {
	s, home, _ := repairHome(t)
	t.Setenv(devEnv, "1")
	path := filepath.Join(home, ".cursor", cursorMCPFile)
	writeDoc(t, path, staleDoc(staleLich))

	s.RepairRegistrations()

	if got := lichCommandIn(t, path); got != staleLich {
		t.Errorf("lich command = %q, want it left at %q", got, staleLich)
	}
}

func TestDevInstance(t *testing.T) {
	goRun := filepath.Join(os.TempDir(), "go-build123", "b001", "exe", "lich")
	tests := []struct {
		name   string
		flag   string
		exe    string
		exeErr error
		want   bool
	}{
		{"installed", "", "/usr/bin/lich", nil, false},
		{"LICH_DEV set", "1", "/usr/bin/lich", nil, true},
		{"go run binary", "", goRun, nil, true},
		{"unresolvable executable", "", "", errors.New("no exe"), false},
	}
	for _, tt := range tests {
		if got := devInstance(tt.flag, tt.exe, tt.exeErr); got != tt.want {
			t.Errorf("%s: devInstance = %v, want %v", tt.name, got, tt.want)
		}
	}
}

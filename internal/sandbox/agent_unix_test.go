//go:build unix

package sandbox

import (
	"net"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/omartelo/lich/internal/providers"
)

// listenUnix puts a real socket on disk and returns its path: what the ssh
// agent leaves behind, and the only thing AgentSocket is willing to mount.
//
// The directory is its own, deliberately not the caller's t.TempDir(): a unix
// address is capped at 104 bytes on macOS and 108 on Linux (sun_path), and
// TempDir spends most of that before the file name — /var/folders/<32 chars>/T/
// plus the test's own name is already over the line on macOS, where the bind
// fails with a bare "invalid argument". Nothing reads the socket's directory:
// AgentSocket takes the path from SSH_AUTH_SOCK, so the socket has no reason to
// sit beside the home a test builds.
func listenUnix(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "l")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	path := filepath.Join(dir, "a.sock")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("listen on %s (%d bytes): %v", path, len(path), err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	return path
}

// SSH_AUTH_SOCK is an environment variable, which means it can name anything at
// all. Everything that is not a socket on disk is refused rather than mounted:
// a bind of a missing source fails the whole spawn, and a relative path is one
// this package would resolve against a working directory it does not own.
func TestAgentSocketRefusesWhatIsNotASocket(t *testing.T) {
	dir := t.TempDir()
	regular := filepath.Join(dir, "regular")
	if err := os.WriteFile(regular, nil, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	for name, value := range map[string]string{
		"unset":    "",
		"relative": "agent.sock",
		"missing":  filepath.Join(dir, "gone.sock"),
		"a file":   regular,
		"a dir":    dir,
	} {
		t.Setenv("SSH_AUTH_SOCK", value)
		if got := AgentSocket(); got != "" {
			t.Errorf("%s SSH_AUTH_SOCK yielded %q, want no socket", name, got)
		}
	}

	socket := listenUnix(t)
	t.Setenv("SSH_AUTH_SOCK", socket)
	if got := AgentSocket(); got != socket {
		t.Errorf("AgentSocket = %q, want %q", got, socket)
	}
}

// The agent is handed over only when asked for. This is the whole contract of
// the setting: the default sandbox has no credentials in it.
func TestDescribeMountsTheAgentOnlyWhenAsked(t *testing.T) {
	home := t.TempDir()
	cwd := filepath.Join(home, "repo")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	socket := listenUnix(t)
	t.Setenv("SSH_AUTH_SOCK", socket)
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatalf("mkdir .ssh: %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, ".ssh", "known_hosts"), []byte("github.com ssh-ed25519 AAAA\n"), 0o600); err != nil {
		t.Fatalf("write known_hosts: %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, ".ssh", "id_ed25519"), []byte("private\n"), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	if spec := Describe(providers.Claude, home, cwd, "", nil, false); slices.Contains(spec.Read, socket) {
		t.Errorf("the agent socket was mounted without being asked for: %v", spec.Read)
	}
	// Neither half of the grant travels without the grant.
	if spec := Describe(providers.Claude, home, cwd, "", nil, false); slices.Contains(spec.Read, filepath.Join(home, ".ssh", "known_hosts")) {
		t.Errorf("known_hosts was mounted without the grant: %v", spec.Read)
	}
	spec := Describe(providers.Claude, home, cwd, "", nil, true)
	if !slices.Contains(spec.Read, socket) {
		t.Errorf("the agent socket is missing from Read: %v", spec.Read)
	}
	// The socket without the host keys is a grant that cannot do the one thing
	// it exists for: ssh fails to verify github.com before it offers the key.
	if !slices.Contains(spec.Read, filepath.Join(home, ".ssh", "known_hosts")) {
		t.Errorf("known_hosts is missing from Read: %v", spec.Read)
	}
	// The directory holding it is what the sandbox exists to keep out.
	if slices.Contains(spec.Read, filepath.Join(home, ".ssh")) {
		t.Errorf("~/.ssh itself was mounted: %v", spec.Read)
	}
	// Read-only, never writable: connecting to a socket is not a write to the
	// mount, and a writable bind would hand over the directory holding it.
	if slices.Contains(spec.Write, socket) {
		t.Errorf("the agent socket is writable: %v", spec.Write)
	}
}

// An agent that is not there is not an error: the session opens without it, and
// git says so itself when a push needs a key.
func TestDescribeSurvivesAMissingAgent(t *testing.T) {
	home := t.TempDir()
	cwd := filepath.Join(home, "repo")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Setenv("SSH_AUTH_SOCK", "")
	if spec := Describe(providers.Claude, home, cwd, "", nil, true); slices.Contains(spec.Read, "") {
		t.Errorf("an absent agent left an empty mount behind: %v", spec.Read)
	}
}

// The grant is the socket and the host keys, and nothing else under ~/.ssh. A
// private key inside the sandbox is the thing the agent exists to avoid.
func TestDescribeNeverMountsAPrivateKey(t *testing.T) {
	home := t.TempDir()
	cwd := filepath.Join(home, "repo")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatalf("mkdir .ssh: %v", err)
	}
	key := filepath.Join(home, ".ssh", "id_ed25519")
	if err := os.WriteFile(key, []byte("private\n"), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	t.Setenv("SSH_AUTH_SOCK", listenUnix(t))

	spec := Describe(providers.Claude, home, cwd, "", nil, true)
	for _, path := range append(append([]string{}, spec.Read...), spec.Write...) {
		if path == key || path == filepath.Join(home, ".ssh") {
			t.Fatalf("a confined session was given %q", path)
		}
	}
}

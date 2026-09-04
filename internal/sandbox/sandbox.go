// Package sandbox confines a session's process tree to the work it was opened
// for: a private empty home, the rest of the machine read-only, and write
// access only inside the checkout and the provider's own state directory.
//
// It is the counterweight to skipping permission prompts. An agent running
// unattended edits files and runs commands with the user's whole account behind
// it — every other repository on the disk, every credential in the home
// directory, every dotfile. Confining it costs the session nothing it needs and
// takes the rest of the machine off the table.
//
// It is not a boundary against hostile code. The backends here use namespaces
// and mounts (Linux) or a path policy (macOS) and nothing else: no seccomp
// filter, no Landlock ruleset, no kernel-bug mitigation. It protects a machine
// from an agent working unsupervised, not from something written to break out.
// Running genuinely hostile code still wants a disposable VM.
//
// The backend is selected by build tag behind Available and Wrap, the seam the
// PTY uses (internal/terminal): Linux runs bubblewrap, macOS runs sandbox-exec,
// and every other platform answers that it cannot confine anything.
package sandbox

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/omartelo/lich/internal/providers"
)

// Spec is one confined spawn: what stays writable, what is merely readable, and
// what is not there at all. Everything is an absolute host path, and the paths
// are what the backends translate into their own vocabulary.
//
// Home is the user's real home directory. Inside the sandbox that path exists
// and is empty — the point of the whole exercise — with Write and Read entries
// under it mounted back in one by one.
type Spec struct {
	// Home is the user's home directory, replaced by an empty private one.
	Home string
	// Cwd is the checkout the session works in, writable.
	Cwd string
	// GitCommon is the repository's common git directory when Cwd is a linked
	// worktree, writable, and empty when Cwd is a repository of its own. Without
	// it every git command in a worktree session fails: the worktree's `.git` is
	// a file pointing at a directory that would not be there.
	GitCommon string
	// Write is every other path bound read-write: the provider's state, and the
	// build caches a toolchain cannot work without.
	Write []string
	// Read is every path bound read-only: configuration, installed toolchains,
	// and the binaries the session runs.
	Read []string
}

// stateDirs is where each provider keeps the state a confined session still
// needs: its credentials, so it can authenticate at all, and its transcript, so
// lich goes on reading the cost and context readouts off it. Bound read-write —
// the session writes its own conversation there.
//
// Each entry is the directory that provider's own docs and CLI name, in the
// same spelling internal/agentplugin and internal/terminal/transcript.go use.
// Each of the three resolves its own harness paths rather than sharing one
// table: they are read at different times, and a package that cannot see a
// provider's env var would answer for it anyway.
//
// A provider missing from here confines to a home with no credentials in it,
// which is a session that opens and cannot log in. The registry is the
// checklist, so all eight are listed.
func stateDirs(providerID, home string) []string {
	switch providerID {
	case providers.Claude:
		// The JSON file beside the directory holds the account, so a session
		// without it starts at the login prompt.
		return []string{
			envDir("CLAUDE_CONFIG_DIR", filepath.Join(home, ".claude")),
			filepath.Join(home, ".claude.json"),
		}
	case providers.Codex:
		return []string{envDir("CODEX_HOME", filepath.Join(home, ".codex"))}
	case providers.Antigravity:
		// One directory covers all three things a confined session needs: the
		// OAuth credentials it authenticates with sit at the root (shared with
		// Gemini CLI), its customizations under `config/`, and its conversations
		// under `antigravity-cli/`. No environment variable moves it — 1.1.19
		// falls back to a hardcoded ".gemini" when it cannot resolve the home.
		return []string{filepath.Join(home, ".gemini")}
	case providers.OMP:
		return []string{ompAgentDir(home)}
	case providers.OpenCode:
		// opencode is xdg-basedir on every platform, and splits config from the
		// session store it writes conversations into.
		return []string{
			filepath.Join(configHome(home), "opencode"),
			filepath.Join(dataHome(home), "opencode"),
		}
	case providers.Crush:
		return []string{
			filepath.Join(configHome(home), "crush"),
			filepath.Join(dataHome(home), "crush"),
		}
	case providers.Cursor:
		// Two, because Cursor splits its state and reaches the second half
		// through the home directly rather than through its config dir: the
		// credentials (`auth.json`, `cli-config.json`) and the chats are under
		// the config dir (providers.CursorConfigDir, which is not xdg-basedir
		// and says why), while `~/.cursor` holds the `mcp.json` it reads, its
		// per-project transcripts and its CLI state. With no XDG_CONFIG_HOME
		// the two are the same directory, which is the usual case.
		dirs := []string{providers.CursorConfigDir(home)}
		if underHome := filepath.Join(home, ".cursor"); underHome != dirs[0] {
			dirs = append(dirs, underHome)
		}
		return dirs
	case providers.Kiro:
		// Two, because Kiro splits credentials from everything else and only the
		// first half is xdg-basedir: the login token lives in the SQLite store
		// under the data home (`kiro-cli/data.sqlite3`, key
		// `kirocli:social:token`, and XDG_DATA_HOME is honoured — both measured
		// on 2.21.0), while `~/.kiro` holds the agents lich installs into, the
		// settings and the conversations usage.go reads. A session with only the
		// second opens at the login prompt.
		return []string{
			filepath.Join(dataHome(home), "kiro-cli"),
			filepath.Join(home, ".kiro"),
		}
	}
	return nil
}

// ompAgentDir resolves oh-my-pi's agent directory. A named profile wins outright
// over the explicit override — the order `omp config path` was measured to
// apply, and the same rule lives in internal/agentplugin/omp.go and
// internal/terminal/transcript.go.
func ompAgentDir(home string) string {
	if profile := os.Getenv("OMP_PROFILE"); profile != "" {
		return filepath.Join(home, ".omp", "profiles", profile, "agent")
	}
	return envDir("PI_CODING_AGENT_DIR", filepath.Join(home, ".omp", "agent"))
}

// envDir returns the directory named by an environment variable, or fallback
// when it is unset. A relative value is ignored: a bind mount needs an absolute
// path, and a provider pointed at a relative one resolves it against a working
// directory this package does not own.
func envDir(name, fallback string) string {
	if dir := os.Getenv(name); filepath.IsAbs(dir) {
		return dir
	}
	return fallback
}

// configHome and dataHome are the XDG directories, which every provider here
// resolves the same way on Linux and macOS alike.
func configHome(home string) string {
	return envDir("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
}

func dataHome(home string) string {
	return envDir("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
}

// writableCaches are the directories a toolchain writes into on an ordinary
// build, listed by what breaks without them rather than by which tool owns
// them. They hold downloaded dependencies and compiler output — no credentials,
// nothing the user typed — which is why they are the one part of the home a
// confined session may still write outside its own state.
//
// A missing entry is not a warning, it is `go build` failing to write its cache
// in a session that worked yesterday, so the list errs towards including a
// cache rather than towards a smaller sandbox.
var writableCaches = []string{
	".cache",             // XDG, and where go-build, pnpm and pip land
	"go/pkg",             // module downloads; ~/go/bin is read-only below
	".cargo/registry",    // crate downloads
	".npm",               // npm's own cache, which is not XDG
	".bun/install/cache", // bun's package cache
}

// readableToolchain are the directories a session reads to find the language
// versions and tools the project runs on, plus the configuration that makes git
// and the shell behave the way they do outside the sandbox. Read-only: a
// session may use the toolchain it was given and may not install into it.
//
// ~/.config is here as one entry rather than a list of the tools under it,
// because a project uses whichever ones it uses. That is the deliberate hole in
// this profile: a token stored under ~/.config (gh's, for one) is readable
// inside the sandbox. What is not there at all is the material that hole would
// be worth closing for — ~/.ssh, ~/.aws, ~/.gnupg, ~/.kube and every other
// repository on the machine — and none of them is in this list.
var readableToolchain = []string{
	".config",
	".local/bin",
	".local/share",
	".local/state",
	".gitconfig",
	".gitignore_global",
	".asdf",
	".nvm",
	".rustup",
	".cargo/bin",
	".bun/bin",
	".deno",
	"go/bin",
	".pyenv",
	".rbenv",
	".sdkman",
}

// Describe builds the Spec for one confined session: the provider's state and
// the build caches writable, the toolchain and configuration readable, the
// checkout writable, and nothing else in the home there at all.
//
// The display server's socket is in there too, which is the one thing mounted
// back that is not about running the project: without it the agent's own copy
// and its clipboard image paste fail, because both shell out to a tool that has
// to reach the compositor (displaySockets).
//
// extraRead is what the caller knows and this package does not — where the
// binaries the spawn actually runs live. The provider's own executable and the
// lich binary (which the session calls back through, and which serves its MCP
// server) are installed wherever the user put them, which is regularly inside
// the home this call is about to empty. Pass directories, from BinaryDirs.
//
// Paths that do not exist are dropped: a bind mount of a missing source fails
// the whole spawn, and every entry here is optional by nature — a machine
// without Rust has no ~/.cargo, and that is not an error.
func Describe(providerID, home, cwd, gitCommon string, extraRead []string, sshAgent bool) Spec {
	spec := Spec{Home: home, Cwd: cwd, GitCommon: gitCommon}
	spec.Write = existing(append(stateDirs(providerID, home), under(home, writableCaches)...))
	read := append(under(home, readableToolchain), displaySockets(home)...)
	if sshAgent {
		read = append(read, AgentSocket(), knownHosts(home))
	}
	spec.Read = existing(append(read, extraRead...))
	return spec
}

// Backend names what confines a session on this machine, or "" when nothing can.
// It is Available's answer with the name attached: a confined session behaves
// differently on each backend, and the name is the first thing a report about
// one needs.
func Backend() string {
	if !Available() {
		return ""
	}
	return backendName
}

// AgentSocket is the ssh agent's socket, read from the environment lich itself
// was launched in, or "" when there is no agent to hand over. Mounting it is
// what makes `git push` work in a confined session: the private home takes
// ~/.ssh with it, and the agent is the way to authenticate without ever putting
// a private key inside the sandbox — the agent signs, and the key never moves.
//
// The cost is the honest one, and it is why this is behind a setting rather
// than always on: an agent holds every identity loaded into it, and a session
// handed the socket can sign with all of them, for any host it can reach, for
// as long as it runs. OpenSSH can pin a key to one destination (`ssh-add -h`),
// but only at the moment the key is added on the host — lich is given a socket
// that is already populated and can only pass it on whole.
//
// Exported because the window shows what is in the agent beside the setting
// that hands it over (internal/system): a choice made without that list is a
// choice about "my GitHub key" that actually covers every key.
func AgentSocket() string {
	path := os.Getenv("SSH_AUTH_SOCK")
	if !filepath.IsAbs(path) || !isSocket(path) {
		return ""
	}
	return path
}

// knownHosts is the user's ssh host-key file, mounted read-only alongside the
// agent socket because without it the agent is useless: the private home takes
// ~/.ssh with it, so ssh has no record of github.com's host key, cannot verify
// it, and — with no tty and no askpass — cannot ask. It fails with "Host key
// verification failed" before the key it was just handed is ever offered.
//
// The file rather than the directory it sits in, which is the whole point:
// ~/.ssh holds the private keys this sandbox exists to keep away from an
// unattended agent, and known_hosts holds none of them. Its contents are public
// host keys; what a confined session learns from it is the list of machines the
// user connects to, not a credential.
//
// Read-only, so ssh may verify a host and may not record a new one. A host the
// user has never connected to outside the sandbox therefore still fails inside
// it — blind trust-on-first-use is not something to grant a session running
// unattended, and the fix is to connect to it once on the machine.
//
// The system-wide file (/etc/ssh/ssh_known_hosts) needs nothing here: /etc is
// already mounted read-only.
func knownHosts(home string) string {
	return filepath.Join(home, ".ssh", "known_hosts")
}

// isSocket reports whether path is a unix socket on disk, without following a
// link to get there — the answer decides what gets mounted into the sandbox.
func isSocket(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode()&os.ModeSocket != 0
}

// symlinkHops caps the chain BinaryDirs walks, so a link that points at itself
// cannot spin the spawn.
const symlinkHops = 16

// BinaryDirs returns the directories a confined session needs mounted to run
// the executable named by name — a bare name resolved on PATH, or a path. Empty
// when nothing on PATH answers to it, which is a spawn that would have failed
// outside the sandbox too.
//
// It returns *directories*, never the executable itself, and that is the whole
// point. Agents are installed as a symlink into a versioned store
// (`~/.local/bin/claude` → `~/.local/share/claude/versions/2.1.234`), and
// binding the symlink is what breaks: bubblewrap resolves a mount destination
// through symlinks, so it tries to create the mount point at a target that is
// not in the sandbox yet and fails the whole spawn with "Can't create file at
// …: No such file or directory". Binding the directories that hold each hop
// leaves the link intact and puts its target where the link already points.
//
// The chain is walked hop by hop rather than resolved in one step because it
// can leave the home and come back — a version manager's shim into a store
// elsewhere — and every directory it passes through has to be there.
func BinaryDirs(name string) []string {
	path, err := exec.LookPath(name)
	if err != nil {
		return nil
	}
	var dirs []string
	for range symlinkHops {
		dirs = append(dirs, filepath.Dir(path))
		target, err := os.Readlink(path)
		if err != nil {
			break
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(path), target)
		}
		path = target
	}
	return dirs
}

// under joins each relative name to base, so the tables above read as the
// dotfiles they are.
func under(base string, names []string) []string {
	out := make([]string, 0, len(names))
	for _, name := range names {
		out = append(out, filepath.Join(base, name))
	}
	return out
}

// existing drops duplicates, paths that are not on disk, and symlinks. Order is
// kept: a backend that mounts in order needs the parent before the child.
//
// Symlinks are refused rather than followed, for two reasons that point the same
// way. A link under a directory this list already mounts is a mount destination
// bubblewrap resolves *through* — it then tries to create the mount point at a
// target that is not in the sandbox yet, and fails the whole spawn with "Can't
// create file at …: No such file or directory". And a link is an instruction
// written inside the home to reach somewhere outside it, which is the one thing
// a private home exists to stop; following it would widen the sandbox by
// whatever the user's dotfile manager happens to point at.
//
// The cost is real and deliberate: a `~/.gitconfig` symlinked out of a dotfiles
// repository is not in a confined session, and nothing on screen says so. The
// binaries the session runs are the exception, and they are handled by mounting
// directories instead (BinaryDirs).
func existing(paths []string) []string {
	seen := make(map[string]bool, len(paths))
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		out = append(out, path)
	}
	return out
}

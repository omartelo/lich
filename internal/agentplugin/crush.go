package agentplugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/omartelo/lich/internal/providers"
	"github.com/omartelo/lich/internal/relay"
	"github.com/omartelo/lich/internal/semver"
	"github.com/omartelo/lich/internal/shquote"
)

// The Crush side. Crush has no plugin system at all: its hooks are entries in
// the user's own config, and the scripts they name have to already be somewhere
// on disk. So an install here is two writes — the scripts into lich's own
// directory, and a block of `hook add` lines into Crush's `crushrc`.
//
// `crushrc` rather than `crush.json` because it is a script, not a document:
// lich appends its own lines and removes them by their markers, leaving every
// other line — and every comment — exactly as the user wrote it. Rewriting the
// JSON would reformat a file lich does not own.
//
// Only two of the four reports are registered, because `PreToolUse` is the only
// event Crush has (docs/hooks/).

const (
	// crushMinVersion is the first Crush release whose crushrc understands
	// `hook add` (v0.88.0). Older ones read the file, ignore the lines and say
	// nothing — the install would look done and report nothing forever — so the
	// version is checked before anything is written.
	crushMinVersion = "0.88.0"

	// crushHookEvent is Crush's single event; every hook here rides it.
	crushHookEvent = "PreToolUse"
	// crushWriteTools matches the tools that change files, anchored because
	// Crush's tool names are whole lower-case words.
	crushWriteTools = "^(edit|write|multiedit|bash)$"

	// shComment is how the marker lines hide from the shell that runs crushrc.
	shComment = "#"
	// blockOpen and blockClose delimit lich's lines inside crushrc, so an update
	// replaces exactly what a previous install wrote.
	blockOpen  = shComment + " >>> " + markerName + " v%s >>>"
	blockClose = shComment + " <<< " + markerName + " <<<"
)

// crushHooks is what lich registers, one entry per report Crush can honour:
// the script from the plugin repository, the argument it takes, the name the
// hook is stored under, and the tools it matches ("" for every tool).
var crushHooks = []struct {
	script  string
	arg     string
	name    string
	matcher string
}{
	{script: "hooks/report-session-start.sh", arg: providers.Crush, name: "lich-session-id"},
	{script: "hooks/report-touched.sh", name: "lich-touched", matcher: crushWriteTools},
}

func (s *Service) crushInstall() error {
	if err := s.crushSupportsHooks(); err != nil {
		return err
	}
	version, err := s.releaseVersion()
	if err != nil {
		return err
	}
	dir, err := crushScriptDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	for _, hook := range crushHooks {
		data, err := s.fetchFile(version, hook.script)
		if err != nil {
			return err
		}
		// Executable: Crush dispatches a shebang'd script through os/exec, which
		// needs the bit the way any other shell would.
		path := filepath.Join(dir, filepath.Base(hook.script))
		if err := os.WriteFile(path, data, 0o755); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	return s.writeCrushrc(version, dir, lichBinary())
}

// writeCrushrc replaces lich's block in Crush's crushrc, creating the file when
// it is not there yet. Everything outside the markers is carried over verbatim.
func (s *Service) writeCrushrc(version, scriptDir, lichBin string) error {
	path, err := s.crushrcPath()
	if err != nil {
		return err
	}
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}
	body := replaceBlock(string(existing), crushrcBlock(version, scriptDir, lichBin))
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// crushrcBlock is the block lich owns: its markers, a line saying who wrote it,
// one `hook add` per report, and the `mcp add` that gives the session the tools
// to drive the others.
func crushrcBlock(version, scriptDir, lichBin string) string {
	var b strings.Builder
	fmt.Fprintf(&b, blockOpen+"\n", version)
	fmt.Fprintf(&b, "%s Managed by lich. Edit through Settings, not by hand.\n", shComment)
	for _, hook := range crushHooks {
		// Two layers of quoting, and both are load-bearing: the inner one makes
		// the script path survive as one word inside the command Crush runs, the
		// outer one makes that whole command survive as one argument to the
		// crushrc builtin reading this line.
		command := shquote.Quote(filepath.Join(scriptDir, filepath.Base(hook.script)))
		if hook.arg != "" {
			command += " " + hook.arg
		}
		fmt.Fprintf(&b, "hook add %s --name %s", crushHookEvent, hook.name)
		if hook.matcher != "" {
			fmt.Fprintf(&b, " --matcher %s", shquote.Quote(hook.matcher))
		}
		fmt.Fprintf(&b, " --command %s --timeout 5\n", shquote.Quote(command))
	}
	if lichBin != "" {
		// The operations Claude Code and Codex are handed on their own command
		// line at spawn (docs/cli.md, Registration). Crush takes no such flag,
		// and this is the only other place it reads a server from — the same
		// file its hooks come from, under the same markers, removed by the same
		// uninstall.
		//
		// The path is this binary's, resolved now: the registration is a line in
		// a file, so it cannot name $LICH_BIN and have Crush expand it per
		// session. It is only the transport — which lich a session talks to is
		// decided by the coordinates in its PTY, not by which binary runs.
		fmt.Fprintf(&b, "mcp add %s --type stdio --command %s --args %s --timeout 10\n",
			relay.MCPServerName, shquote.Quote(lichBin), relay.MCPSubcommand)
	}
	b.WriteString(blockClose + "\n")
	return b.String()
}

// lichBinary is the lich this install writes into Crush's config. An
// unresolvable executable leaves the registration out rather than writing a
// command that cannot run: the hooks are the part Crush needs to be useful, and
// they do not depend on it.
func lichBinary() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return exe
}

// replaceBlock returns existing with lich's block replaced by block — appended
// when there is none, and left where it was when there is. A file whose opening
// marker lost its close is treated as having no block: appending is recoverable,
// eating the rest of the file is not.
func replaceBlock(existing, block string) string {
	start := indexOfOpenMarker(existing)
	if start < 0 {
		return appendBlock(existing, block)
	}
	end := strings.Index(existing[start:], blockClose)
	if end < 0 {
		return appendBlock(existing, block)
	}
	end += start + len(blockClose)
	// The block always ends in a newline, so the one that followed the old close
	// would otherwise become a blank line on every reinstall.
	rest := strings.TrimPrefix(existing[end:], "\n")
	return existing[:start] + block + rest
}

// indexOfOpenMarker finds lich's opening marker whatever version it names.
func indexOfOpenMarker(existing string) int {
	prefix, _, _ := strings.Cut(blockOpen, "%s")
	return strings.Index(existing, prefix)
}

// appendBlock puts the block at the end, on its own line and after a blank one
// when the file already has content.
func appendBlock(existing, block string) string {
	if strings.TrimSpace(existing) == "" {
		return block
	}
	return strings.TrimRight(existing, "\n") + "\n\n" + block
}

// crushInstalledVersion reads the version off the block lich wrote into
// crushrc, or ("", false) when there is none.
func (s *Service) crushInstalledVersion() (string, bool) {
	path, err := s.crushrcPath()
	if err != nil {
		return "", false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return versionOf(data, shComment+" >>>")
}

// crushSupportsHooks refuses an install onto a Crush too old to read the lines
// it would write. A version lich cannot read at all is allowed through: a
// refusal there would be lich guessing, and the failure it prevents is one the
// user can see and undo.
func (s *Service) crushSupportsHooks() error {
	out, err := s.read(providers.Crush, "--version")
	if err != nil {
		return fmt.Errorf("crush --version: %w: %s", err, strings.TrimSpace(out))
	}
	version := parseCrushVersion(out)
	if version == "" || !semver.Less(version, crushMinVersion) {
		return nil
	}
	return fmt.Errorf(
		"Crush %s cannot load hooks from crushrc — update to %s or newer",
		version, crushMinVersion)
}

// parseCrushVersion pulls the bare version out of `crush --version`, whose
// output is "crush version v0.88.1". Empty when the shape is not that.
func parseCrushVersion(out string) string {
	fields := strings.Fields(strings.TrimSpace(out))
	if len(fields) == 0 {
		return ""
	}
	version := strings.TrimPrefix(fields[len(fields)-1], "v")
	if version == "" || version[0] < '0' || version[0] > '9' {
		return ""
	}
	return version
}

// crushrcPath resolves the global crushrc: the sibling of Crush's global
// config, in the directory `crush dirs` prints first. Asking the CLI rather
// than rebuilding its path rule keeps lich right when that rule differs per OS
// or moves.
func (s *Service) crushrcPath() (string, error) {
	out, err := s.read(providers.Crush, "dirs")
	if err != nil {
		return "", fmt.Errorf("crush dirs: %w: %s", err, strings.TrimSpace(out))
	}
	dir, _, _ := strings.Cut(strings.TrimSpace(out), "\n")
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return "", fmt.Errorf("crush dirs printed no config directory")
	}
	return filepath.Join(dir, "crushrc"), nil
}

// crushScriptDir is where lich keeps the hook scripts it installs: its own
// config directory, not Crush's. They are lich's copy of another repository's
// files, and a harness that never installed them should not be carrying them.
func crushScriptDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve config directory: %w", err)
	}
	return filepath.Join(dir, "lich", "plugin", "hooks"), nil
}

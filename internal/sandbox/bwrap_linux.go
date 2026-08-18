//go:build linux

package sandbox

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// bwrapBin is bubblewrap's executable. It is what confines a session on Linux:
// an unprivileged user namespace, a mount namespace built from the arguments
// below, and no setuid helper of lich's own.
const bwrapBin = "bwrap"

// systemDirs are the top-level directories that make the sandbox an operating
// system rather than an empty namespace, all read-only. /nix and /opt are here
// for the machines that have them and cost nothing on the machines that do not:
// a missing one is dropped before it reaches bwrap.
//
// /var is deliberately absent. Binding it brought in whatever was mounted
// underneath — a running container's overlay, measured read-write inside a
// confined session, because bubblewrap's read-only remount does not always
// reach a submount it inherited. What it bought was the package database, which
// a coding agent reads at most to answer a question it can ask another way.
var systemDirs = []string{"/usr", "/etc", "/opt", "/nix"}

// mergedDirs are the paths a modern distribution turns into symlinks into /usr
// and an older one keeps as real directories. Reproduced as whatever the host
// has: a --ro-bind over a host symlink would resolve to a path the sandbox does
// not have, and a --symlink where the host has a directory hides half of it.
var mergedDirs = []string{"/bin", "/sbin", "/lib", "/lib32", "/lib64", "/libx32"}

// Available reports whether this machine can confine a session. Linux answers
// for bubblewrap being installed and nothing else — whether the kernel will
// actually grant the namespace is a question only a spawn can ask, and the
// distributions that refuse (Ubuntu and Debian ship an AppArmor policy denying
// unprivileged user namespaces) say so in bwrap's own error, which is the one
// the user can act on.
func Available() bool {
	_, err := exec.LookPath(bwrapBin)
	return err == nil
}

// Wrap returns the binary and arguments that run bin confined. It falls back to
// the unconfined spawn when bubblewrap is not installed, which is the same
// answer Available gives — a session must open either way, and a card that
// never appears is worse than one that is not confined.
func Wrap(spec Spec, bin string, args []string) (string, []string) {
	path, err := exec.LookPath(bwrapBin)
	if err != nil {
		return bin, args
	}
	argv := append(bwrapArgs(spec, hostRoots()), "--", bin)
	return path, append(argv, args...)
}

// root is one entry of the read-only base filesystem: either a bind of a host
// directory or a symlink reproduced from the host.
type root struct {
	// target is the symlink's destination, empty for a bind mount.
	target string
	path   string
}

// hostRoots reads the base filesystem this machine actually has. Split from
// bwrapArgs so the argument order — the part that decides whether a sandbox is
// a sandbox — is a pure function with a test, on a machine with any layout.
func hostRoots() []root {
	roots := make([]root, 0, len(systemDirs)+len(mergedDirs))
	for _, dir := range systemDirs {
		if _, err := os.Stat(dir); err == nil {
			roots = append(roots, root{path: dir})
		}
	}
	for _, dir := range mergedDirs {
		info, err := os.Lstat(dir)
		if err != nil {
			continue
		}
		if info.Mode()&os.ModeSymlink == 0 {
			roots = append(roots, root{path: dir})
			continue
		}
		target, err := os.Readlink(dir)
		if err != nil {
			continue
		}
		roots = append(roots, root{target: target, path: dir})
	}
	return roots
}

// bwrapArgs is the sandbox itself: the arguments that build it, in the order
// bubblewrap applies them. Order is the whole contract — every mount lands on
// top of what came before it, so the base filesystem is laid down first, the
// private home empties whatever the base left under it, and the paths the
// session is allowed to keep are mounted back on top of that emptiness. Moving
// the home above the paths mounted into it silently unmounts every one of them.
func bwrapArgs(spec Spec, roots []root) []string {
	var args []string
	for _, r := range roots {
		if r.target != "" {
			args = append(args, "--symlink", r.target, r.path)
			continue
		}
		args = append(args, "--ro-bind", r.path, r.path)
	}
	// /etc/resolv.conf is a symlink into /run on a systemd-resolved machine, and
	// /run is not mounted: without its target the sandbox has no DNS, which
	// looks like the agent's API being down.
	if target := resolvedLink("/etc/resolv.conf"); target != "" {
		args = append(args, "--ro-bind", target, target)
	}
	args = append(args,
		"--proc", "/proc",
		"--dev", "/dev",
		"--tmpfs", "/tmp",
		"--tmpfs", spec.Home,
	)
	// The host's runtime directory holds the session bus, the keyring and the
	// display sockets. An empty one of its own answers the programs that expect
	// the variable to name a real directory, and hands over none of that.
	if dir := os.Getenv("XDG_RUNTIME_DIR"); filepath.IsAbs(dir) {
		args = append(args, "--tmpfs", dir)
	}
	for _, path := range spec.Read {
		args = append(args, "--ro-bind", path, path)
	}
	for _, path := range spec.Write {
		args = append(args, "--bind", path, path)
	}
	if spec.GitCommon != "" {
		args = append(args, "--bind", spec.GitCommon, spec.GitCommon)
	}
	// Last of the mounts: the checkout is the one place the session is here to
	// change, and nothing may land on top of it.
	args = append(args, "--bind", spec.Cwd, spec.Cwd)
	return append(args,
		"--setenv", "HOME", spec.Home,
		"--chdir", spec.Cwd,
		// No --unshare-net: the agent needs its own API, and the lich plugin's
		// hooks report this session's status back over loopback. A sandbox that
		// cuts the network is a card that opens and cannot work.
		"--unshare-pid",
		"--unshare-uts",
		"--unshare-ipc",
		"--hostname", "lich-sandbox",
		// The sandbox dies with the PTY rather than outliving it as an orphan
		// nothing on screen accounts for. No --new-session: it calls setsid,
		// which would take the session's controlling terminal away from the PTY
		// lich is reading.
		"--die-with-parent",
	)
}

// resolvedLink returns the destination of path when it is a symlink pointing
// outside the directories already mounted, and "" otherwise — including for a
// destination that no longer exists, which would fail the spawn rather than
// leave one file missing.
func resolvedLink(path string) string {
	target, err := filepath.EvalSymlinks(path)
	if err != nil || target == path {
		return ""
	}
	for _, dir := range systemDirs {
		if target == dir || strings.HasPrefix(target, dir+string(filepath.Separator)) {
			return ""
		}
	}
	if _, err := os.Stat(target); err != nil {
		return ""
	}
	return target
}

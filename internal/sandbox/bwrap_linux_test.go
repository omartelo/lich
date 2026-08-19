//go:build linux

package sandbox

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// fakeRoots is a base filesystem with one of each shape: a plain directory and
// a merged-usr symlink.
var fakeRoots = []root{
	{path: "/usr"},
	{path: "/etc"},
	{target: "usr/bin", path: "/bin"},
}

func testSpec() Spec {
	return Spec{
		Home:      "/home/u",
		Cwd:       "/home/u/work/repo",
		GitCommon: "/home/u/work/main/.git",
		Write:     []string{"/home/u/.claude", "/home/u/.cache"},
		Read:      []string{"/home/u/.config", "/home/u/.local/bin"},
	}
}

// indexOf is the position of the first argument equal to value, or -1.
func indexOf(args []string, value string) int {
	return slices.Index(args, value)
}

// pairIndex is the position of a two-argument mount ("--bind", path) whose
// source is path, or -1. It is how the order assertions below name a mount
// without depending on what surrounds it.
func pairIndex(args []string, flag, path string) int {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag && args[i+1] == path {
			return i
		}
	}
	return -1
}

// The private home unmounts everything already under it, so it has to be laid
// down before the paths that are mounted back in. This is the assertion that
// fails if somebody reorders the builder and quietly ships a sandbox where the
// provider has no credentials.
func TestBwrapArgsMountHomeBeforeWhatGoesIntoIt(t *testing.T) {
	args := bwrapArgs(testSpec(), fakeRoots)

	home := pairIndex(args, "--tmpfs", "/home/u")
	if home < 0 {
		t.Fatalf("no private home in %v", args)
	}
	for _, path := range []string{"/home/u/.claude", "/home/u/.cache"} {
		if at := pairIndex(args, "--bind", path); at < home {
			t.Errorf("%s is mounted at %d, before the private home at %d", path, at, home)
		}
	}
	for _, path := range []string{"/home/u/.config", "/home/u/.local/bin"} {
		if at := pairIndex(args, "--ro-bind", path); at < home {
			t.Errorf("%s is mounted at %d, before the private home at %d", path, at, home)
		}
	}
	if base := pairIndex(args, "--ro-bind", "/usr"); base > home {
		t.Errorf("the base filesystem at %d lands after the private home at %d", base, home)
	}
}

// The checkout is the one place the session exists to change: nothing may be
// mounted over it.
func TestBwrapArgsMountCheckoutLast(t *testing.T) {
	args := bwrapArgs(testSpec(), fakeRoots)
	cwd := pairIndex(args, "--bind", "/home/u/work/repo")
	if cwd < 0 {
		t.Fatalf("the checkout is not mounted: %v", args)
	}
	for i := cwd + 2; i+1 < len(args); i++ {
		switch args[i] {
		case "--bind", "--ro-bind", "--tmpfs", "--symlink":
			t.Errorf("%s %s is mounted over the checkout", args[i], args[i+1])
		}
	}
}

// A worktree's .git is a file pointing at the main repository's metadata. Without
// that directory every git command in the session fails.
func TestBwrapArgsBindGitCommonDirWritable(t *testing.T) {
	args := bwrapArgs(testSpec(), fakeRoots)
	if pairIndex(args, "--bind", "/home/u/work/main/.git") < 0 {
		t.Errorf("the git common directory is not writable: %v", args)
	}

	own := testSpec()
	own.GitCommon = ""
	if got := bwrapArgs(own, fakeRoots); indexOf(got, "/home/u/work/main/.git") >= 0 {
		t.Errorf("a checkout that is its own repository got a common-dir mount: %v", got)
	}
}

// The network is the one capability a coding session cannot lose: the agent
// calls its own API, and the lich plugin's hooks report back over loopback.
func TestBwrapArgsKeepTheNetwork(t *testing.T) {
	args := bwrapArgs(testSpec(), fakeRoots)
	if indexOf(args, "--unshare-net") >= 0 {
		t.Errorf("the sandbox cuts the network: %v", args)
	}
	for _, want := range []string{"--unshare-pid", "--unshare-uts", "--unshare-ipc", "--die-with-parent"} {
		if indexOf(args, want) < 0 {
			t.Errorf("%s missing from %v", want, args)
		}
	}
}

// --new-session calls setsid, which would take the controlling terminal away
// from the PTY lich reads.
func TestBwrapArgsKeepTheControllingTerminal(t *testing.T) {
	if args := bwrapArgs(testSpec(), fakeRoots); indexOf(args, "--new-session") >= 0 {
		t.Errorf("the sandbox detaches the session's terminal: %v", args)
	}
}

func TestBwrapArgsReproduceMergedUsrSymlinks(t *testing.T) {
	args := bwrapArgs(testSpec(), fakeRoots)
	at := pairIndex(args, "--symlink", "usr/bin")
	if at < 0 || args[at+2] != "/bin" {
		t.Errorf("merged-usr symlink not reproduced: %v", args)
	}
	if pairIndex(args, "--ro-bind", "/bin") >= 0 {
		t.Errorf("a host symlink was bound as a directory: %v", args)
	}
}

func TestBwrapArgsSetHomeAndStartDirectory(t *testing.T) {
	args := bwrapArgs(testSpec(), fakeRoots)
	at := indexOf(args, "--setenv")
	if at < 0 || args[at+1] != "HOME" || args[at+2] != "/home/u" {
		t.Errorf("HOME is not set to the private home: %v", args)
	}
	chdir := indexOf(args, "--chdir")
	if chdir < 0 || args[chdir+1] != "/home/u/work/repo" {
		t.Errorf("the sandbox does not start in the checkout: %v", args)
	}
}

// Wrap has to keep the provider's own arguments after the separator, in order:
// a flag that lands before "--" is read by bubblewrap instead of the agent.
func TestWrapSeparatesBubblewrapFromTheCommand(t *testing.T) {
	if !Available() {
		t.Skip("bubblewrap is not installed")
	}
	bin, args := Wrap(testSpec(), "claude", []string{"--resume", "abc", "--model", "opus"})
	if !strings.HasSuffix(bin, "bwrap") {
		t.Fatalf("Wrap ran %q, want bubblewrap", bin)
	}
	at := indexOf(args, "--")
	if at < 0 {
		t.Fatalf("no separator in %v", args)
	}
	want := []string{"claude", "--resume", "abc", "--model", "opus"}
	if got := args[at+1:]; !slices.Equal(got, want) {
		t.Errorf("command after the separator = %v, want %v", got, want)
	}
}

// /etc/resolv.conf is a symlink into /run on a systemd-resolved machine, and
// /run is not mounted. Missing its target, the sandbox has no DNS — which looks
// like the agent's API being down rather than like a mount that was skipped.
func TestResolvedLinkMountsOnlyWhatTheBaseDoesNotCover(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real.conf")
	if err := os.WriteFile(target, []byte("nameserver 127.0.0.53\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	link := filepath.Join(dir, "link.conf")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	if got := resolvedLink(link); got != target {
		t.Errorf("resolvedLink(symlink) = %q, want %q", got, target)
	}
	// A plain file is already covered by the mount of the directory it sits in.
	if got := resolvedLink(target); got != "" {
		t.Errorf("resolvedLink(plain file) = %q, want \"\"", got)
	}
	// A destination inside the read-only base is mounted twice for nothing.
	inside := filepath.Join(dir, "usr.conf")
	if err := os.Symlink("/usr/lib", inside); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	if got := resolvedLink(inside); got != "" {
		t.Errorf("resolvedLink(into /usr) = %q, want \"\"", got)
	}
	// A dangling link would fail the whole spawn rather than leave one file out.
	dangling := filepath.Join(dir, "gone.conf")
	if err := os.Symlink(filepath.Join(dir, "nothing"), dangling); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	if got := resolvedLink(dangling); got != "" {
		t.Errorf("resolvedLink(dangling) = %q, want \"\"", got)
	}
	if got := resolvedLink(filepath.Join(dir, "absent")); got != "" {
		t.Errorf("resolvedLink(missing) = %q, want \"\"", got)
	}
}

// A blank root is an empty tmpfs over a directory that came in with the base
// filesystem, and it has to land after the bind that brought it in. It is what
// keeps ssh working inside the sandbox: the drop-in configs arrive owned by the
// overflow uid and ssh refuses to read them, taking `git fetch` and `git push`
// down with them.
func TestBwrapArgsBlankRootIsAnEmptyTmpfsOverTheBind(t *testing.T) {
	roots := append(slices.Clone(fakeRoots), root{blank: true, path: sshDropInDir})
	args := bwrapArgs(testSpec(), roots)

	blank := pairIndex(args, "--tmpfs", sshDropInDir)
	if blank < 0 {
		t.Fatalf("%s is not blanked: %v", sshDropInDir, args)
	}
	if etc := pairIndex(args, "--ro-bind", "/etc"); etc > blank {
		t.Errorf("/etc is bound at %d, after the tmpfs blanking %s at %d", etc, sshDropInDir, blank)
	}
	if at := pairIndex(args, "--ro-bind", sshDropInDir); at >= 0 {
		t.Errorf("%s is also bound read-only at %d, which defeats the tmpfs", sshDropInDir, at)
	}
}

// The blank entry is read off the host like every other root: a machine that
// has the directory gets it emptied, one that does not gets no mount at all
// (bubblewrap fails a spawn whose mount point it cannot create under a
// read-only /etc).
func TestHostRootsBlanksTheSSHDropInsWhenThePlatformHasThem(t *testing.T) {
	_, err := os.Stat(sshDropInDir)
	blanked := slices.ContainsFunc(hostRoots(), func(r root) bool {
		return r.blank && r.path == sshDropInDir
	})
	if want := err == nil; blanked != want {
		t.Errorf("hostRoots blanks %s = %v, want %v", sshDropInDir, blanked, want)
	}
}

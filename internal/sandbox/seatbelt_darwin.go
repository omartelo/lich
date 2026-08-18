//go:build darwin

package sandbox

import (
	"os"
	"strings"
)

// seatbeltBin is Apple's sandbox interface. It is deprecated and has been for
// years, and it is still the only path-level confinement a macOS process can
// apply to a child without shipping a signed helper.
const seatbeltBin = "/usr/bin/sandbox-exec"

// Available reports whether this machine can confine a session. macOS answers
// for sandbox-exec being present, which on a stock system it is.
func Available() bool {
	info, err := os.Stat(seatbeltBin)
	return err == nil && !info.IsDir() && info.Mode()&0o111 != 0
}

// Wrap returns the binary and arguments that run bin confined, falling back to
// the unconfined spawn when sandbox-exec is missing: a session must open either
// way (bwrap_linux.go's reason).
//
// The profile goes in on the command line rather than through a file, so
// nothing has to be written to disk for a spawn and nothing has to be cleaned
// up after one.
func Wrap(spec Spec, bin string, args []string) (string, []string) {
	if !Available() {
		return bin, args
	}
	argv := []string{"-p", seatbeltProfile(spec), bin}
	return seatbeltBin, append(argv, args...)
}

// seatbeltProfile is the sandbox itself, in Apple's own policy language. Order
// is the contract, and it is the opposite of bubblewrap's: the last rule that
// matches an operation wins, so the profile opens permissive, closes the two
// doors that matter, and then reopens exactly the paths the session works in.
//
// What this cannot reproduce is the private home Linux gets. A tmpfs over the
// home directory needs privileges a user process does not have, so the home is
// denied for reading instead: the paths are still there, and a session that
// asks about one is refused rather than told it does not exist. The material
// this exists to keep away from an unattended agent — ssh keys, cloud
// credentials, every other repository — is unreachable either way, and the
// difference shows up only in the error a program gets when it looks.
func seatbeltProfile(spec Spec) string {
	var b strings.Builder
	b.WriteString("(version 1)\n(allow default)\n")
	// The two denials, widest first: nothing outside the checkout is writable,
	// and nothing in the home is even readable.
	b.WriteString("(deny file-write*)\n")
	if spec.Home != "" {
		b.WriteString("(deny file-read* " + subpath(spec.Home) + ")\n")
	}
	// Then the exceptions, narrowest last. Readable before writable, so a path
	// listed in both ends up writable.
	writeRule(&b, "(allow file-read*", spec.Read)
	writable := append([]string{spec.Cwd}, spec.Write...)
	if spec.GitCommon != "" {
		writable = append(writable, spec.GitCommon)
	}
	writeRule(&b, "(allow file-read* file-write*", writable)
	return b.String()
}

// writeRule appends one rule covering every path, or nothing at all when there
// are none — an operation list with no subpaths is a syntax error, and a
// profile that fails to parse takes the session with it.
func writeRule(b *strings.Builder, head string, paths []string) {
	if len(paths) == 0 {
		return
	}
	b.WriteString(head)
	for _, path := range paths {
		b.WriteString(" " + subpath(path))
	}
	b.WriteString(")\n")
}

// subpath renders one path as an SBPL subpath term. The quoting is the security
// boundary of this file: a path holding a quote or a backslash would otherwise
// close the string and continue the policy as code, and a directory name is
// something the user types.
func subpath(path string) string {
	escaped := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(path)
	return `(subpath "` + escaped + `")`
}

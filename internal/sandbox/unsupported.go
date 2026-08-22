//go:build !linux && !darwin

package sandbox

// Available reports whether this machine can confine a session. Windows has no
// backend here: its isolation primitives (AppContainer, a Job object with a
// restricted token) confine an application built for them rather than an
// arbitrary child process tree, and the port is experimental with no hardware
// to prove one on. The control is absent rather than present and inert, so the
// answer is one word instead of a setting that saves and does nothing.
func Available() bool { return false }

// Wrap returns the spawn unchanged. It exists so the caller has one seam on
// every platform rather than a build tag of its own.
func Wrap(_ Spec, bin string, args []string) (string, []string) {
	return bin, args
}

// backendName is unused here: Backend answers "" before it is read.
const backendName = ""

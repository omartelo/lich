package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// probeTimeout bounds the confined spawn. A kernel that refuses the namespace
// answers at once, but a machine where the request blocks must not take the
// diagnosis with it: `lich doctor` prints how long every check took, so no
// check of its own may be the one that never returns.
const probeTimeout = 2 * time.Second

// The markers the probe reads back from inside the sandbox. They are written to
// two files the backend is supposed to treat differently, and the answer is
// which of them came out:
//
//   - probeReachable sits in the checkout, which every backend keeps readable.
//     It coming back proves the confined child ran at all.
//   - probeUnreachable sits in the home, which every backend takes away. It
//     coming back proves the backend started and confined nothing.
//
// Asking for both is what makes the answer honest. A probe that only checked
// whether the launcher starts would pass on a machine whose ruleset silently
// applies nothing, which is a session running unconfined under a shield saying
// otherwise.
const (
	probeReachable   = "lich-probe-checkout"
	probeUnreachable = "lich-probe-home"
)

// Probe answers whether a confined session on this machine would actually be
// confined, by opening one and looking. It returns the backend's name and why
// it confines nothing, or a nil error when it does; an empty name means this
// platform has no backend at all and there is nothing to report.
//
// Available cannot answer this and does not try: it resolves the launcher and
// stops, because only a spawn can ask the kernel. On Ubuntu and Debian an
// AppArmor policy denies unprivileged user namespaces, so bubblewrap is
// installed, Available says yes, and every confined session dies on bwrap's own
// error. This is the diagnosis of that machine, not a second gate in front of
// it: nothing here decides whether a session is confined.
func Probe() (string, error) {
	if !Available() {
		return "", nil
	}
	// cat is what reads the two markers, one process and no shell. It comes
	// from the host's PATH and is bound at the same path inside, which is how
	// the session's own binaries get there at all.
	reader, err := exec.LookPath("cat")
	if err != nil {
		return backendName, fmt.Errorf("cat is not on PATH, so nothing can be read from inside a sandbox: %w", err)
	}
	layout, err := layOutProbe()
	if err != nil {
		return backendName, err
	}
	defer func() { _ = os.RemoveAll(layout.dir) }()

	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	// The spec is a real one, so what is probed is the sandbox a session gets
	// rather than a second ruleset written to pass.
	bin, args := Wrap(layout.spec, reader, []string{layout.reachable, layout.unreachable})
	cmd := exec.CommandContext(ctx, bin, args...)
	// The deadline kills the launcher, and WaitDelay bounds the wait for what it
	// left behind: a namespace request that hangs must not outlive the check.
	cmd.WaitDelay = probeTimeout
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	runErr := cmd.Run()
	return backendName, verdict(stdout.String(), stderr.String(), ctx.Err(), runErr)
}

// probeLayout is one probe run laid out on disk: the spec that decides what the
// sandbox may see, and the two files read through it.
type probeLayout struct {
	spec        Spec
	reachable   string
	unreachable string
	// dir holds both and is the caller's to remove.
	dir string
}

// layOutProbe writes the markers and the spec that separates them. Split from
// Probe because the whole answer rests on which file lands where, and that half
// has a test on any machine: a layout with both markers on the same side of the
// sandbox would report every machine broken, or every machine fine, and the
// spawn would not notice either way.
func layOutProbe() (probeLayout, error) {
	dir, err := os.MkdirTemp("", "lich-sandbox-probe")
	if err != nil {
		return probeLayout{}, fmt.Errorf("the probe has nowhere to write its files: %w", err)
	}
	// macOS hands out a temporary directory behind /var, a symlink to
	// /private/var, and the seatbelt profile matches the resolved path: an
	// unresolved one is denied by no rule and read straight back, which would
	// read as a backend confining nothing.
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}

	layout := probeLayout{
		spec:        Spec{Home: filepath.Join(dir, "home"), Cwd: filepath.Join(dir, "cwd")},
		reachable:   filepath.Join(dir, "cwd", "checkout-file"),
		unreachable: filepath.Join(dir, "home", "home-file"),
		dir:         dir,
	}
	for path, marker := range map[string]string{
		layout.reachable:   probeReachable,
		layout.unreachable: probeUnreachable,
	} {
		if err := plant(path, marker); err != nil {
			_ = os.RemoveAll(dir)
			return probeLayout{}, fmt.Errorf("the probe could not be laid out: %w", err)
		}
	}
	return layout, nil
}

// plant writes one marker file and the directory holding it.
func plant(path, marker string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(marker), 0o600)
}

// verdict reads what came back, and carries its own consequence: the two ways
// to fail cost different things, and a caller appending one sentence to both
// would tell half of them the wrong thing. It is a pure function so the
// decision has a test on any machine, including one with no backend to run the
// spawn half on.
//
// Order is the contract: a sandbox that leaked the home also handed over the
// checkout, so the leak is answered first and a probe is only believed confined
// when it proved it ran.
func verdict(stdout, stderr string, timeout, runErr error) error {
	if strings.Contains(stdout, probeUnreachable) {
		// This one still opens a session. It is the worse failure of the two:
		// the card draws its shield and the agent is on the machine.
		return fmt.Errorf("%s starts and confines nothing: a file under the home it replaces was read from inside the sandbox, so a session marked confined would run on the machine itself", backendName)
	}
	if strings.Contains(stdout, probeReachable) {
		return nil
	}
	return fmt.Errorf("%s, so a session opened with the sandbox on will not start", refusal(stderr, timeout, runErr))
}

// refusal is why the confined child never ran, in the order the answers are
// worth having: the deadline first, because a killed spawn leaves whatever it
// had written, then the backend's own complaint, which is the sentence the
// reader can act on. bubblewrap names the namespace the kernel refused, and a
// distribution denying them is the whole reason this check exists.
func refusal(stderr string, timeout, runErr error) string {
	switch {
	case timeout != nil:
		return fmt.Sprintf("%s did not answer within %s", backendName, probeTimeout)
	case firstLine(stderr) != "":
		return firstLine(stderr)
	case runErr != nil:
		return fmt.Sprintf("%s could not open a confined session: %v", backendName, runErr)
	}
	return backendName + " opened a session that read nothing back"
}

// firstLine is the head of a backend's stderr, trimmed. What follows it is a
// second failure caused by the first, and a diagnosis reads better ending at
// the cause.
func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(line)
}

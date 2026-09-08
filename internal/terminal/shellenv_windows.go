//go:build windows

package terminal

import (
	"context"
	"os/exec"
)

// runShellDump runs cmdStr under shell -l -i with pipe stdio. SHELL is
// normally unset on Windows — cmd.exe has no rc file to guard on a tty, so
// ResolveShellEnv returns before this ever runs — and no shell shipped here
// is known to gate its rc on `[ -t 0 ]`, so wiring up ConPTY buys nothing a
// pipe doesn't already give; ctx's cancellation kills the process (see
// exec.CommandContext).
func runShellDump(ctx context.Context, shell, cmdStr string, env []string) (string, <-chan struct{}, error) {
	cmd := exec.CommandContext(ctx, shell, "-l", "-i", "-c", cmdStr)
	cmd.Env = env
	out, err := cmd.Output() // stderr, carrying rc warnings, is discarded
	// Nothing outlives this call: the read is over pipes the process's exit
	// closes, so there is never a reader left for the next resolution to wait
	// on — hence the nil channel.
	return string(out), nil, err
}

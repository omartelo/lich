//go:build darwin

package terminal

import "golang.org/x/sys/unix"

const envReadable = true

func readEnv(pid int) map[string]string {
	if pid <= 0 {
		return nil
	}
	// SysctlRaw resolves kern.procargs2 to {CTL_KERN, KERN_PROCARGS2, pid}.
	data, err := unix.SysctlRaw("kern.procargs2", pid)
	if err != nil {
		return nil
	}
	return parseProcArgsEnv(data)
}

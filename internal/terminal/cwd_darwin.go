package terminal

import (
	"bytes"
	"syscall"
	"unsafe"
	_ "unsafe" // for go:linkname
)

const cwdTracked = true

// proc_pidinfo is not exposed by x/sys/unix, so this file binds it from
// libSystem the way x/sys itself binds libc calls: a cgo_import_dynamic symbol
// reached through an assembly trampoline (cwd_darwin.s) and the runtime's
// libSystem-aware syscall6 — no CGO involved, the binary stays pure Go.

//go:linkname syscall_syscall6 syscall.syscall6
func syscall_syscall6(fn, a1, a2, a3, a4, a5, a6 uintptr) (r1, r2 uintptr, err syscall.Errno)

var libc_proc_pidinfo_trampoline_addr uintptr

//go:cgo_import_dynamic libc_proc_pidinfo proc_pidinfo "/usr/lib/libSystem.B.dylib"

// procPidVnodePathInfo is the proc_pidinfo flavor returning a process's
// current and root directories (PROC_PIDVNODEPATHINFO, sys/proc_info.h).
const procPidVnodePathInfo = 9

// vnodeInfoPath mirrors struct vnode_info_path: an opaque struct vnode_info
// (vinfo_stat 136 bytes + vi_type 4 + vi_pad 4 + vi_fsid 8) followed by the
// path (MAXPATHLEN). Only the path is read, so the stat half stays opaque.
type vnodeInfoPath struct {
	_    [152]byte
	Path [1024]byte
}

// procVnodePathInfo mirrors struct proc_vnodepathinfo: the current directory,
// then the root directory.
type procVnodePathInfo struct {
	Cdir vnodeInfoPath
	Rdir vnodeInfoPath
}

// processCwd returns pid's current working directory, or "" when it cannot be
// read (the process exited, or was never ours to inspect). proc_pidinfo
// returns the number of bytes filled; anything short of the full struct is a
// failure.
func processCwd(pid int) string {
	var info procVnodePathInfo
	n, _, _ := syscall_syscall6(
		libc_proc_pidinfo_trampoline_addr,
		uintptr(pid),
		procPidVnodePathInfo,
		0,
		uintptr(unsafe.Pointer(&info)),
		unsafe.Sizeof(info),
		0,
	)
	if n != unsafe.Sizeof(info) {
		return ""
	}
	path := info.Cdir.Path[:]
	if i := bytes.IndexByte(path, 0); i >= 0 {
		path = path[:i]
	}
	return string(path)
}

package terminal

import (
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

const cwdTracked = true

// processCwd returns pid's current working directory, and a second value the
// unix readers use to name a foreground process hosting a shell they cannot
// follow (tmux, ssh, a container). Windows has no foreground process group to
// read that from — a console hands its input to whatever holds it, and nothing
// records which process that is — so this side never has a host to report and
// the readout can still name a directory the user has left. Kept as is
// deliberately: the whole detection would have to be rebuilt on a different
// mechanism, and there is no Windows hardware here to build it against.
func processCwd(pid int) (string, string) {
	return readCwd(pid), ""
}

// readCwd returns pid's current working directory, or "" when it cannot be
// read (the process exited, or was never ours to inspect). Windows keeps a
// process's cwd in its PEB (ProcessParameters.CurrentDirectory), so the read
// is a pointer walk through the child's memory: NtQueryInformationProcess for
// the PEB address, then ReadProcessMemory for the PEB, the process parameters
// and finally the path buffer. Assumes the child matches our architecture (a
// 32-bit child's PEB has a different layout) — there the walk dereferences
// garbage and fails, degrading to "" like any dead process.
func readCwd(pid int) string {
	h, err := windows.OpenProcess(
		windows.PROCESS_QUERY_INFORMATION|windows.PROCESS_VM_READ,
		false,
		uint32(pid),
	)
	if err != nil {
		return ""
	}
	defer func() { _ = windows.CloseHandle(h) }()

	var pbi windows.PROCESS_BASIC_INFORMATION
	var retLen uint32
	err = windows.NtQueryInformationProcess(h, windows.ProcessBasicInformation,
		unsafe.Pointer(&pbi), uint32(unsafe.Sizeof(pbi)), &retLen)
	if err != nil || pbi.PebBaseAddress == nil {
		return ""
	}

	var peb windows.PEB
	if err := readMemory(h, uintptr(unsafe.Pointer(pbi.PebBaseAddress)),
		unsafe.Pointer(&peb), unsafe.Sizeof(peb)); err != nil {
		return ""
	}
	if peb.ProcessParameters == nil {
		return ""
	}

	var params windows.RTL_USER_PROCESS_PARAMETERS
	if err := readMemory(h, uintptr(unsafe.Pointer(peb.ProcessParameters)),
		unsafe.Pointer(&params), unsafe.Sizeof(params)); err != nil {
		return ""
	}

	dos := params.CurrentDirectory.DosPath
	runes := dosPathRunes(dos.Length)
	if runes == 0 || dos.Buffer == nil {
		return ""
	}
	buf := make([]uint16, runes)
	if err := readMemory(h, uintptr(unsafe.Pointer(dos.Buffer)),
		unsafe.Pointer(&buf[0]), uintptr(dos.Length)); err != nil {
		return ""
	}
	// DosPath carries a trailing backslash ("C:\Users\x\"); Clean drops it
	// while keeping a bare drive root intact.
	return filepath.Clean(windows.UTF16ToString(buf))
}

// readMemory reads size bytes of the process behind h at base into out.
func readMemory(h windows.Handle, base uintptr, out unsafe.Pointer, size uintptr) error {
	return windows.ReadProcessMemory(h, base, (*byte)(out), size, nil)
}

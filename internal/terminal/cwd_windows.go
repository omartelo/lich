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
// read (the process exited, was never ours to inspect, or does not share our
// pointer width — see readProcessParameters). Windows keeps a process's cwd in
// its PEB (ProcessParameters.CurrentDirectory), so the read is a pointer walk
// through the child's memory: NtQueryInformationProcess for the PEB address,
// then ReadProcessMemory for the PEB, the process parameters and finally the
// path buffer.
func readCwd(pid int) string {
	h, err := openForRead(pid)
	if err != nil {
		return ""
	}
	defer func() { _ = windows.CloseHandle(h) }()

	params := readProcessParameters(h)
	if params == nil {
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

// openForRead opens pid for the PEB walk both readers below start from: the
// query right NtQueryInformationProcess needs, and the VM read right every
// pointer they follow needs. The caller closes the handle.
func openForRead(pid int) (windows.Handle, error) {
	return windows.OpenProcess(
		windows.PROCESS_QUERY_INFORMATION|windows.PROCESS_VM_READ,
		false,
		uint32(pid),
	)
}

// readMemory reads size bytes of the process behind h at base into out.
func readMemory(h windows.Handle, base uintptr, out unsafe.Pointer, size uintptr) error {
	return windows.ReadProcessMemory(h, base, (*byte)(out), size, nil)
}

// readProcessParameters walks the PEB behind h down to the process parameters
// the cwd and the environment both hang off, and returns nil when any hop
// fails. Shared by readCwd and readEnv (env_windows.go), so the rule below is
// one rule rather than two.
//
// A child whose pointer width is not ours is refused rather than walked. A
// WOW64 process has a 32-bit PEB, and reading that through our native layout
// is not a read that fails — it is a read that succeeds on the wrong offsets
// and hands back a directory or an environment that looks real. This is
// unmeasured: there is no Windows hardware on this project to establish which
// of the two it actually does, and a false negative is the safe half of that
// uncertainty while a believed-wrong value is not.
func readProcessParameters(h windows.Handle) *windows.RTL_USER_PROCESS_PARAMETERS {
	var childWOW64, selfWOW64 bool
	if windows.IsWow64Process(h, &childWOW64) != nil ||
		windows.IsWow64Process(windows.CurrentProcess(), &selfWOW64) != nil ||
		childWOW64 != selfWOW64 {
		return nil
	}

	var pbi windows.PROCESS_BASIC_INFORMATION
	var retLen uint32
	err := windows.NtQueryInformationProcess(h, windows.ProcessBasicInformation,
		unsafe.Pointer(&pbi), uint32(unsafe.Sizeof(pbi)), &retLen)
	if err != nil || pbi.PebBaseAddress == nil {
		return nil
	}

	var peb windows.PEB
	if err := readMemory(h, uintptr(unsafe.Pointer(pbi.PebBaseAddress)),
		unsafe.Pointer(&peb), unsafe.Sizeof(peb)); err != nil {
		return nil
	}
	if peb.ProcessParameters == nil {
		return nil
	}

	var params windows.RTL_USER_PROCESS_PARAMETERS
	if err := readMemory(h, uintptr(unsafe.Pointer(peb.ProcessParameters)),
		unsafe.Pointer(&params), unsafe.Sizeof(params)); err != nil {
		return nil
	}

	return &params
}

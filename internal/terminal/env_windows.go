//go:build windows

package terminal

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

const envReadable = true

// Bound allocations from a remote pointer; an implausible block is unknown.
const maxProcessEnvBytes = 1 << 20

func readEnv(pid int) map[string]string {
	if pid <= 0 {
		return nil
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_INFORMATION|windows.PROCESS_VM_READ, false, uint32(pid))
	if err != nil {
		return nil
	}
	defer func() { _ = windows.CloseHandle(h) }()
	// A WOW64 PEB has 32-bit pointers. Do not interpret it as our native layout.
	var childWOW64, selfWOW64 bool
	if windows.IsWow64Process(h, &childWOW64) != nil ||
		windows.IsWow64Process(windows.CurrentProcess(), &selfWOW64) != nil || childWOW64 != selfWOW64 {
		return nil
	}
	params := readProcessParameters(h)
	if params == nil || params.Environment == nil || params.EnvironmentSize < 4 ||
		params.EnvironmentSize > maxProcessEnvBytes || params.EnvironmentSize%2 != 0 {
		return nil
	}
	data := make([]byte, params.EnvironmentSize)
	if err := readMemory(h, uintptr(params.Environment), unsafe.Pointer(&data[0]), uintptr(len(data))); err != nil {
		return nil
	}
	return parseUTF16Env(data)
}

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
	h, err := openForRead(pid)
	if err != nil {
		return nil
	}
	defer func() { _ = windows.CloseHandle(h) }()
	params := readProcessParameters(h)
	// Under 4 bytes because EnvironmentSize 0 would size data to nothing and
	// take &data[0] straight into a panic, on the bare goroutine serving an
	// RPC; 2 is a lone UTF-16 terminator, which no variable fits inside.
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

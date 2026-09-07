package awake

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The power request API (PowerCreateRequest and friends) is not in x/sys, so
// it is bound here by hand. Two request types are set on the one handle:
// SystemRequired resets the idle timer that leads to S3 sleep, the assertion
// every media player holds; ExecutionRequired is the one Modern Standby
// laptops answer to, there the screen going off is the entry into standby,
// and the Desktop Activity Moderator then freezes every desktop process that
// does not hold it. Without the second, a locked laptop pauses the agents
// exactly as before.
const (
	powerRequestContextVersion      = 0
	powerRequestContextSimpleString = 1
	powerRequestSystemRequired      = 1
	powerRequestExecutionRequired   = 3
)

// reasonContext mirrors REASON_CONTEXT with a simple string, which is what
// `powercfg /requests` prints beside lich.exe.
type reasonContext struct {
	version uint32
	flags   uint32
	reason  *uint16
}

// powerRequestKinds is every request type set on the handle, and cleared from it.
var powerRequestKinds = []uintptr{powerRequestSystemRequired, powerRequestExecutionRequired}

var (
	kernel32          = windows.NewLazySystemDLL("kernel32.dll")
	powerCreateReq    = kernel32.NewProc("PowerCreateRequest")
	powerSetRequest   = kernel32.NewProc("PowerSetRequest")
	powerClearRequest = kernel32.NewProc("PowerClearRequest")
)

func hold() (func(), error) {
	reason, err := windows.UTF16PtrFromString("A lich session is working")
	if err != nil {
		return nil, err
	}
	ctx := reasonContext{version: powerRequestContextVersion, flags: powerRequestContextSimpleString, reason: reason}
	h, _, callErr := powerCreateReq.Call(uintptr(unsafe.Pointer(&ctx)))
	if windows.Handle(h) == windows.InvalidHandle {
		return nil, fmt.Errorf("PowerCreateRequest: %w", callErr)
	}
	handle := windows.Handle(h)
	for _, kind := range powerRequestKinds {
		if ok, _, callErr := powerSetRequest.Call(uintptr(handle), kind); ok == 0 {
			_ = windows.CloseHandle(handle)
			return nil, fmt.Errorf("PowerSetRequest(%d): %w", kind, callErr)
		}
	}
	return func() {
		for _, kind := range powerRequestKinds {
			_, _, _ = powerClearRequest.Call(uintptr(handle), kind)
		}
		_ = windows.CloseHandle(handle)
	}, nil
}

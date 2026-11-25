package engine

import (
	"os"
	"syscall"
)

var sysSetStdHandle = sysKernel32.NewProc("SetStdHandle")

// a GUI subsystem process without redirection has no standard handles
func sysStderrHandleOK() bool {
	h, err := syscall.GetStdHandle(syscall.STD_ERROR_HANDLE)
	return err == nil && h != 0 && h != syscall.InvalidHandle
}

// The runtime looks the handle up on every write, so panics follow it.
func sysReplaceStderr(f *os.File) error {
	which := int32(syscall.STD_ERROR_HANDLE) // a negative constant passed as a DWORD
	r, _, err := sysSetStdHandle.Call(uintptr(uint32(which)), f.Fd())
	if r == 0 {
		return err
	}
	os.Stderr = f
	return nil
}

//go:build windows

package sdl

import (
	"syscall"
	"unsafe"
)

const libName = "SDL3.dll"

// LoadLibraryEx flags: resolve dependencies from the DLL's own directory and the
// default system directories, never from the current directory or PATH.
const (
	loadLibrarySearchDllLoadDir  = 0x00000100
	loadLibrarySearchDefaultDirs = 0x00001000
)

var procLoadLibraryExW = syscall.NewLazyDLL("kernel32.dll").NewProc("LoadLibraryExW")

func open(path string) (uintptr, error) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	h, _, e := procLoadLibraryExW.Call(uintptr(unsafe.Pointer(p)), 0, loadLibrarySearchDllLoadDir|loadLibrarySearchDefaultDirs)
	if h == 0 {
		return 0, e
	}
	return h, nil
}

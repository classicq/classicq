package engine

import (
	"os"
	"syscall"
)

func sysStderrHandleOK() bool {
	return true
}

func sysReplaceStderr(f *os.File) error {
	return syscall.Dup3(int(f.Fd()), 2, 0)
}

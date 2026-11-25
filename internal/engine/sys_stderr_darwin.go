package engine

import (
	"os"
	"syscall"
)

func sysStderrHandleOK() bool {
	return true
}

func sysReplaceStderr(f *os.File) error {
	return syscall.Dup2(int(f.Fd()), 2)
}

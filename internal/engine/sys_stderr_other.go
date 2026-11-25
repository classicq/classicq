//go:build !windows && !linux && !darwin

package engine

import (
	"errors"
	"os"
)

func sysStderrHandleOK() bool {
	return true
}

func sysReplaceStderr(f *os.File) error {
	return errors.New("stderr redirection not supported")
}

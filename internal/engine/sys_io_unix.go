//go:build linux || darwin

package engine

import "syscall"

func Sys_IO_Path_Writable(path string) bool {
	return syscall.Access(path, 2) == nil // W_OK
}

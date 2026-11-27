//go:build windows

package engine

import "os"

// The read-only attribute, which os.Stat reports as cleared write bits.
func Sys_IO_Path_Writable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().Perm()&0200 != 0
}

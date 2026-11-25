//go:build !windows

package engine

import "time"

var sys_start = time.Now()

// Seconds since process start on a monotonic clock; C counts from system
// boot, only differences are ever used.
func Sys_DoubleTime() float64 {
	return time.Since(sys_start).Seconds()
}

func Sys_IntTime() uint64 {
	return uint64(time.Since(sys_start).Microseconds())
}

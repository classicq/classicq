package engine

import (
	"syscall"
	"unsafe"
)

// sys_sdl.c reads QueryPerformanceCounter on Windows. The Go monotonic clock
// there steps with the system timer (about 0.5 ms), too coarse for the
// connection thread and the sleep granularity measurement.

var (
	sysKernel32 = syscall.NewLazyDLL("kernel32.dll")
	sysQPCAddr  = sysKernel32.NewProc("QueryPerformanceCounter").Addr()
	sysQPCFreq  = func() uint64 {
		var freq int64
		sysKernel32.NewProc("QueryPerformanceFrequency").Call(uintptr(unsafe.Pointer(&freq)))
		return uint64(freq)
	}()
)

// syscall.SyscallN keeps the pointer on the stack; (*LazyProc).Call would
// move it to the heap on every call, and the frame limiter spins on this.
func sysPerformanceCounter() uint64 {
	var now int64
	syscall.SyscallN(sysQPCAddr, uintptr(unsafe.Pointer(&now)))
	return uint64(now)
}

func Sys_DoubleTime() float64 {
	return float64(sysPerformanceCounter()) / float64(sysQPCFreq)
}

// now * 1000000 / freq, split so it cannot overflow
func Sys_IntTime() uint64 {
	now := sysPerformanceCounter()
	return now/sysQPCFreq*1000000 + now%sysQPCFreq*1000000/sysQPCFreq
}

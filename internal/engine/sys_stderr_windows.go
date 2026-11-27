// Copyright (C) 2026 classicQ contributors
//
// This program is free software; you can redistribute it and/or
// modify it under the terms of the GNU General Public License
// as published by the Free Software Foundation; either version 2
// of the License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.
//
// See the GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program; if not, write to the Free Software
// Foundation, Inc., 59 Temple Place - Suite 330, Boston, MA  02111-1307, USA.

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

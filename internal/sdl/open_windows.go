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

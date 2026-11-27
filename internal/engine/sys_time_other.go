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

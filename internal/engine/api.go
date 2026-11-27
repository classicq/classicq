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

// Entry points for the cmd programs.

func HostInit(argv []string) {
	Host_Init(argv)
}

func HostFrame(dt float64) {
	Host_Frame(dt)
}

func ConsoleInput(line string) {
	Cbuf_AddText(line + "\n")
}

// DumpActive reports whether -dump was given; DumpFrameTime is the fixed
// step the dump loop runs at, and DemoPlaying tells when it is over.
func DumpActive() bool       { return Dump_Active() }
func DumpFrameTime() float64 { return Dump_FrameTime() }
func DemoPlaying() bool      { return cls.demoplayback }

// DumpMain runs the fixed clock loop of -dump and quits.
func DumpMain() { Dump_Main() }

// SysDoubleTime is the wall clock the main loop feeds to HostFrame.
func SysDoubleTime() float64 { return Sys_DoubleTime() }

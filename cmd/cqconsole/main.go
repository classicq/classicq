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

// Command cqconsole runs the engine without display and sound: each line
// from stdin goes to the command buffer and one frame runs. With -dump
// <file> and +playdemo it plays the demo on a fixed clock and writes the
// state dump instead. Run it from a directory holding id1/ with
// CLASSICQ_USERDATA pointing at a scratch folder.
package main

import (
	"bufio"
	"fmt"
	"os"

	"github.com/classicq/classicq/internal/engine"
)

func main() {
	engine.HostInit(append(os.Args, "-nodisplay", "-nosound"))

	if engine.DumpActive() {
		dt := engine.DumpFrameTime()
		engine.HostFrame(dt)
		for frames := 0; engine.DemoPlaying() && frames < 1000000; frames++ {
			engine.HostFrame(dt)
		}
		return
	}

	sc := bufio.NewScanner(os.Stdin)
	fmt.Print("] ")
	for sc.Scan() {
		line := sc.Text()
		if line == "quit" {
			break
		}
		engine.ConsoleInput(line)
		engine.HostFrame(0.016)
		fmt.Print("] ")
	}
}

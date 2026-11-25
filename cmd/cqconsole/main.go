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

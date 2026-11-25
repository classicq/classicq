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

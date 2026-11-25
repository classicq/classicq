// Command classicq is the game client. SDL3 is loaded from the shared
// library next to the executable, or from CLASSICQ_SDL3_PATH.
package main

import (
	"os"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/classicq/classicq/internal/engine"
	"github.com/classicq/classicq/internal/sdl"
)

func init() {
	// SDL must be driven from the thread that started the process.
	runtime.LockOSThread()
}

func main() {
	stderrpath := engine.Sys_CaptureStderr()

	path, err := sdl.DefaultPath()
	if err == nil {
		err = sdl.Load(path)
	}
	if err != nil {
		engine.Sys_StartupError("%v\n\nclassicQ needs the SDL3 library next to the executable; CLASSICQ_SDL3_PATH points elsewhere.", err)
	}

	sdl.SetMainReady()

	if !sdl.Init(0) {
		engine.Sys_StartupError("SDL_Init failed: %s", sdl.GetError())
	}

	startProfiles(os.Args)
	engine.HostInit(os.Args)

	if stderrpath != "" {
		engine.Com_Printf("stderr goes to %s\n", stderrpath)
	}

	if engine.DumpActive() {
		engine.DumpMain()
	}

	oldtime := engine.SysDoubleTime()
	for {
		newtime := engine.SysDoubleTime()
		engine.HostFrame(newtime - oldtime)
		oldtime = newtime
	}
}

// Developer flags. -memprofile <file> rewrites the allocation profile every
// few seconds, so the run can be killed at any point and still leave data
// behind. -cpuprofile <file> samples seconds 10 to 40 of the run, skipping
// the loading phase, and must finish before the process is killed.
func startProfiles(args []string) {
	for i, a := range args {
		if i+1 >= len(args) {
			break
		}
		path := args[i+1]
		switch a {
		case "-memprofile":
			go func() {
				for {
					time.Sleep(10 * time.Second)
					f, err := os.Create(path)
					if err != nil {
						return
					}
					pprof.Lookup("allocs").WriteTo(f, 0)
					f.Close()
				}
			}()
		case "-cpuprofile":
			go func() {
				time.Sleep(10 * time.Second)
				f, err := os.Create(path)
				if err != nil {
					return
				}
				pprof.StartCPUProfile(f)
				time.Sleep(30 * time.Second)
				pprof.StopCPUProfile()
				f.Close()
			}()
		}
	}
}

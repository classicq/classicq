package engine

import (
	crand "crypto/rand"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/classicq/classicq/internal/sdl"
)

// sys_sdl.c, the parts that need no SDL

func Sys_Printf(format string, a ...any) {
	fmt.Fprintf(os.Stdout, format, a...)
}

var sys_in_error bool

func Sys_Error(format string, a ...any) {
	text := fmt.Sprintf(format, a...)
	fmt.Fprintf(os.Stderr, "FATAL: %s\n", text)
	if !sys_in_error {
		sys_in_error = true
		Host_Shutdown()
		if sdl.ShowSimpleMessageBox != nil {
			sdl.ShowSimpleMessageBox(sdl.MESSAGEBOX_ERROR, "classicQ fatal error", text, nil)
		}
	}
	os.Exit(1)
}

// Sys_StartupError reports a failure before SDL is up. A GUI process has
// no console, so Windows gets a native message box instead.
func Sys_StartupError(format string, a ...any) {
	text := fmt.Sprintf(format, a...)
	fmt.Fprintf(os.Stderr, "FATAL: %s\n", text)
	sysMessageBox("classicQ", text)
	os.Exit(1)
}

func Sys_Quit() {
	Host_Shutdown()
	os.Exit(0)
}

func Sys_Init() {
}

func Sys_CvarInit() {
}

func Sys_MicroSleep(microseconds uint) {
	// C rounds up to whole milliseconds for Sleep(). Rounding down instead
	// keeps the measured sleep granularity at zero: time.Sleep on Windows
	// is too uneven for the granularity probe, and this way the limiter
	// never sleeps past its deadline and spins the sub-millisecond rest.
	if runtime.GOOS == "windows" {
		time.Sleep(time.Duration(microseconds/1000) * time.Millisecond)
		return
	}
	time.Sleep(time.Duration(microseconds) * time.Microsecond)
}

func Sys_SleepTime(usec uint) {
	Sys_MicroSleep(usec)
}

func Sys_ConsoleInput() string {
	return ""
}

func Sys_MakeCodeWriteable(startaddr, length uint) {
}

func Sys_RandomBytes(target []byte) {
	crand.Read(target)
}

func Sys_LowFPPrecision()  {}
func Sys_HighFPPrecision() {}
func Sys_SetFPCW()         {}

var USERDATA_SUBDIR = map[string]string{
	"windows": "\\classicQ",
	"darwin":  "/Library/Application Support/classicQ",
}[runtime.GOOS]

func init() {
	if USERDATA_SUBDIR == "" {
		USERDATA_SUBDIR = "/.classicq"
	}
}

// The directory holding the executable, "" where C returns NULL.
func Sys_GetRODataPath() string {
	path, err := os.Executable()
	if err != nil {
		return ""
	}
	sep := byte('/')
	if runtime.GOOS == "windows" {
		sep = '\\'
	}
	p := strings.LastIndexByte(path, sep)
	if p < 0 {
		return ""
	}
	path = path[:p]
	if runtime.GOOS == "darwin" {
		const bundle_suffix = "/Contents/MacOS"
		if len(path) > len(bundle_suffix) && strings.HasSuffix(path, bundle_suffix) {
			path = path[:len(path)-len(bundle_suffix)]
			if slash := strings.LastIndexByte(path, '/'); slash >= 0 {
				path = path[:slash]
			}
		}
	}
	return path
}

func Sys_GetUserDataPath() string {
	if env := os.Getenv("CLASSICQ_USERDATA"); env != "" {
		return env
	}
	if runtime.GOOS == "windows" {
		base, err := os.UserConfigDir()
		if err != nil {
			return ""
		}
		return base + USERDATA_SUBDIR
	}
	home := os.Getenv("HOME")
	if home == "" {
		return ""
	}
	return home + USERDATA_SUBDIR
}

func Sys_GetLegacyDataPath() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return wd
}

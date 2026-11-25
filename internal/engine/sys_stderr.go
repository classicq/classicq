package engine

import (
	"fmt"
	"os"
	"time"
)

// Sys_CaptureStderr sends stderr to classicq/stderr.log in the user data
// directory when the process has no usable stderr (a Windows GUI process, a
// launcher pointing it at the null device), so Go panics and Sys_Error
// leave a trace. Returns the file path, "" when stderr is left alone.
func Sys_CaptureStderr() string {
	if sysStderrUsable() {
		return ""
	}

	dir := Sys_GetUserDataPath()
	if dir == "" {
		return ""
	}

	dir += "/classicq"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ""
	}

	path := dir + "/stderr.log"
	f, err := os.Create(path)
	if err != nil {
		return ""
	}

	if err := sysReplaceStderr(f); err != nil {
		f.Close()
		return ""
	}

	fmt.Fprintf(os.Stderr, "classicQ %s started %s\n", VersionString(), time.Now().Format("2006-01-02 15:04:05"))

	return path
}

func sysStderrUsable() bool {
	if !sysStderrHandleOK() {
		return false
	}

	fi, err := os.Stderr.Stat()
	if err != nil {
		return false
	}

	if nullfi, err := os.Stat(os.DevNull); err == nil && os.SameFile(fi, nullfi) {
		return false
	}

	return true
}

package engine

import (
	"runtime"
	"runtime/debug"
)

// version.h
const (
	QW_VERSION       = 2.40
	CLASSICQ_VERSION = "3.6.0"
)

var QW_PLATFORM = map[string]string{
	"windows": "Windows",
	"linux":   "Linux",
	"darwin":  "macOS",
}[runtime.GOOS]

// There is no compile time in the build, the commit time stands in.
func buildTime() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if s.Key == "vcs.time" {
				return s.Value
			}
		}
	}
	return "unknown"
}

func CL_Version_f() {
	Com_Printf("classicQ v%s\n", VersionString())
	Com_Printf("Exe: %s\n", buildTime())
}

func VersionString() string {
	return CLASSICQ_VERSION
}

var QW_ARCH = archName()

func archName() string {
	switch runtime.GOARCH {
	case "amd64":
		return "amd64"
	case "arm64":
		return "arm64"
	case "386":
		return "i386"
	case "arm":
		return "arm"
	}
	return "unknown"
}

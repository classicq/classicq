// 	version.c
//
// 	Build number and version strings
//
// 	Copyright (C) 1996-1997 Id Software, Inc.
//
// 	This program is free software; you can redistribute it and/or
// 	modify it under the terms of the GNU General Public License
// 	as published by the Free Software Foundation; either version 2
// 	of the License, or (at your option) any later version.
//
// 	This program is distributed in the hope that it will be useful,
// 	but WITHOUT ANY WARRANTY; without even the implied warranty of
// 	MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.
//
// 	See the GNU General Public License for more details.
//
// 	You should have received a copy of the GNU General Public License
// 	along with this program; if not, write to:
//
// 		Free Software Foundation, Inc.
// 		59 Temple Place - Suite 330
// 		Boston, MA  02111-1307, USA

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

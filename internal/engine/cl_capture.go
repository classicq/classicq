// Copyright (C) 2001-2002       A Nourai
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
// See the included (GNU.txt) GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program; if not, write to the Free Software
// Foundation, Inc., 59 Temple Place - Suite 330, Boston, MA  02111-1307, USA.

package engine

import (
	"runtime"
	"time"
)

// cl_capture.c: demo capture as a numbered series of screenshots

var (
	movie_fps       = cvar_t{name: "demo_capture_fps", string: "30.0"}
	movie_dir       = cvar_t{name: "demo_capture_dir", string: "capture", OnChange: OnChange_movie_dir}
	movie_steadycam = cvar_t{name: "demo_capture_steadycam", string: "0"}
)

var (
	movie_is_capturing, movie_first_frame bool
	movie_start_time, movie_len           float64
	movie_frame_count                     int
	image_ext                             string // 3 chars

	movie_start_date time.Time
)

func Movie_IsCapturing() bool {
	return cls.demoplayback && !cls.timedemo && movie_is_capturing
}

func Movie_Start(_time float64) {
	movie_start_date = time.Now()
	movie_first_frame, movie_is_capturing = true, true
	movie_len = _time
	movie_start_time = cls.realtime

	movie_frame_count = 0
	image_ext = "tga"
}

func Movie_Stop() {
	movie_is_capturing = false
	Com_Printf("Captured %d frames (%.2fs).\n", movie_frame_count, float32(cls.realtime-movie_start_time))
}

func Movie_Demo_Capture_f() {
	error_ := va("Usage: %s <start time | stop>\n", Cmd_Argv(0))
	argc := Cmd_Argc()
	if argc != 2 && argc != 3 {
		Com_Printf("%s", error_)
		return
	}
	if argc == 2 {
		if Q_strncasecmp("stop", Cmd_Argv(1), 4) != 0 {
			Com_Printf("%s", error_)
		} else if Movie_IsCapturing() {
			Movie_Stop()
		} else {
			Com_Printf("%s : Not capturing\n", Cmd_Argv(0))
		}
		return
	}
	if Q_strncasecmp("start", Cmd_Argv(1), 5) != 0 {
		Com_Printf("%s", error_)
		return
	} else if Movie_IsCapturing() {
		Com_Printf("%s : Already capturing\n", Cmd_Argv(0))
		return
	}
	if !cls.demoplayback || cls.timedemo {
		Com_Printf("%s : Must be playing a demo to capture\n", Cmd_Argv(0))
		return
	}
	time_ := float64(Q_atof(Cmd_Argv(2)))
	if time_ <= 0 {
		Com_Printf("%s : Time argument must be positive\n", Cmd_Argv(0))
		return
	}
	Movie_Start(time_)
}

func Movie_CvarInit() {
	Cvar_SetCurrentGroup(CVAR_GROUP_DEMO)
	Cvar_Register(&movie_fps)
	Cvar_Register(&movie_dir)
	Cvar_Register(&movie_steadycam)

	Cvar_ResetCurrentGroup()

	Cmd_AddCommand("demo_capture", Movie_Demo_Capture_f)
}

func Movie_StartFrame() float64 {
	time_ := 1 / 30.0
	if movie_fps.value > 0 {
		time_ = 1.0 / float64(movie_fps.value)
	}
	return bound(1.0/1000, time_, 1.0)
}

func Movie_FinishFrame() {
	if !Movie_IsCapturing() {
		return
	}

	// latch here, cls.realtime is not valid yet when capture starts from the command line
	if movie_first_frame {
		movie_start_time = cls.realtime
		movie_first_frame = false
	}

	// Windows prints the calendar date, the others print the raw struct tm
	// fields: month from 0 and years since 1900
	day, month, year := movie_start_date.Day(), int(movie_start_date.Month()), movie_start_date.Year()
	if runtime.GOOS != "windows" {
		month--
		year -= 1900
	}
	fname := snprintf(128, "%s/capture_%02d-%02d-%04d_%02d-%02d-%02d/shot-%06d.%s",
		movie_dir.string, day, month, year,
		movie_start_date.Hour(), movie_start_date.Minute(), movie_start_date.Second(), movie_frame_count, image_ext)
	SCR_Screenshot(fname)
	movie_frame_count++
	// movie_len is demo time; the wall clock runs slower than the demo while writing frames
	if float64(movie_frame_count)*Movie_StartFrame() >= movie_len {
		Movie_Stop()
	}
}

func OnChange_movie_dir(v *cvar_t, str string) bool {
	if Movie_IsCapturing() {
		Com_Printf("Cannot change demo_capture_dir whilst capturing.  Use 'demo_capture stop' to cease capturing first.\n")
		return true
	} else if len(str) > 31 {
		Com_Printf("demo_capture_dir can only contain a maximum of 31 characters\n")
		return true
	}

	// C cleans the incoming string in place; here the cleaned value goes
	// in through a nested Cvar_Set, which skips OnChange, and the original
	// is vetoed.
	processed := Util_Process_Filename(str)
	if !Util_Is_Valid_Filename(processed) {
		Com_Printf("%s", Util_Invalid_Filename_Msg("demo_capture_dir"))
		return true
	}

	if processed != str {
		Cvar_Set(v, processed)
		return true
	}

	return false
}

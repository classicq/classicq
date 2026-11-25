package engine

import (
	"bufio"
	"os"
)

// cl_logging.c: console logging to a file, by hand and per match

var (
	log_dir      = cvar_t{name: "log_dir", string: "", OnChange: OnChange_log_dir}
	log_readable = cvar_t{name: "log_readable", string: "0"}
)

const LOG_FILENAME_MAXSIZE = MAX_OSPATH * 2

var (
	logfile     *os.File
	logwriter   *bufio.Writer // stdio buffering
	logfilename string

	autologging = false
)

func Log_IsLogging() bool {
	return logfile != nil
}

func Log_LogDirectory() string {
	if log_dir.string != "" {
		return strlcpy(va("%s/%s", com_basedir, log_dir.string), LOG_FILENAME_MAXSIZE)
	}
	return strlcpy(cls.gamedir, LOG_FILENAME_MAXSIZE)
}

func Log_Stop() {
	if !Log_IsLogging() {
		return
	}

	logwriter.Flush()
	logfile.Close()
	logfile = nil
	logwriter = nil
}

func OnChange_log_dir(v *cvar_t, str string) bool {
	if str == "" {
		return false
	}

	// C cleans the incoming string in place; here the cleaned value goes
	// in through a nested Cvar_Set, which skips OnChange, and the original
	// is vetoed.
	processed := Util_Process_Filename(str)
	if !Util_Is_Valid_Filename(processed) {
		Com_Printf("%s", Util_Invalid_Filename_Msg(v.name))

		return true
	}

	if processed != str {
		Cvar_Set(v, processed)
		return true
	}

	return false
}

func logOpen(fullname string) (*os.File, error) {
	templog, err := os.Create(fullname)
	if err != nil {
		FS_CreatePath(fullname)
		templog, err = os.Create(fullname)
	}
	return templog, err
}

func Log_log_f() {
	switch Cmd_Argc() {
	case 1:
		if autologging {
			Com_Printf("Auto console logging is in progress\n")
		} else if Log_IsLogging() {
			Com_Printf("Logging to %s\n", logfilename)
		} else {
			Com_Printf("Not logging\n")
		}

		return
	case 2:
		if Q_strcasecmp(Cmd_Argv(1), "stop") == 0 {
			if autologging {
				Log_AutoLogging_StopMatch()
			} else {
				if Log_IsLogging() {
					Log_Stop()
					Com_Printf("Stopped logging to %s\n", logfilename)
				} else {
					Com_Printf("Not logging\n")
				}
			}

			return
		}

		if autologging {
			Com_Printf("Auto console logging must be stopped first!\n")

			return
		}

		if Log_IsLogging() {
			Log_Stop()
			Com_Printf("Stopped logging to %s\n", logfilename)
		}

		logfilename = strlcpy(Cmd_Argv(1), LOG_FILENAME_MAXSIZE-4)
		logfilename = Util_Process_Filename(logfilename)
		if !Util_Is_Valid_Filename(logfilename) {
			Com_Printf("%s", Util_Invalid_Filename_Msg("filename"))

			return
		}

		logfilename = COM_ForceExtension(logfilename, ".log")
		fulllogname := va("%s/%s", Log_LogDirectory(), logfilename)
		templog, err := logOpen(fulllogname)
		if err != nil {
			Com_Printf("Error: Couldn't open %s\n", logfilename)

			return
		}

		Com_Printf("Logging to %s\n", logfilename)
		logfile = templog
		logwriter = bufio.NewWriter(templog)
	default:
		Com_Printf("Usage: %s [filename | stop]\n", Cmd_Argv(0))

		return
	}
}

func Log_CvarInit() {
	Cvar_SetCurrentGroup(CVAR_GROUP_CONSOLE)
	Cvar_Register(&log_dir)
	Cvar_Register(&log_readable)

	Cvar_ResetCurrentGroup()

	Cmd_AddCommand("log", Log_log_f)
}

func Log_Shutdown() {
	if Log_IsLogging() {
		Log_Stop()
	}
}

func Log_Write(s string) {
	if !Log_IsLogging() {
		return
	}

	logwriter.WriteString(s)
}

//=============================================================================
//							AUTO CONSOLE LOGGING
//=============================================================================

var (
	log_auto_matchname string
	temp_log_ready     = false
	log_auto_starttime float32
)

const TEMP_LOG_NAME = "_!_temp_!_.log"

func Log_AutoLogging_StopMatch() {
	if !autologging {
		return
	}

	autologging = false
	Log_Stop()
	temp_log_ready = true

	if match_auto_logconsole.value == 2 {
		Log_AutoLogging_SaveMatch()
	} else {
		Com_Printf("Auto console logging completed\n")
	}
}

func Log_AutoLogging_CancelMatch() {
	if !autologging {
		return
	}

	autologging = false
	Log_Stop()
	temp_log_ready = true

	if match_auto_logconsole.value == 2 {
		if cls.realtime-float64(log_auto_starttime) > float64(match_auto_minlength.value) {
			Log_AutoLogging_SaveMatch()
		} else {
			Com_Printf("Auto console logging cancelled\n")
		}
	} else {
		Com_Printf("Auto console logging completed\n")
	}
}

func Log_AutoLogging_StartMatch(logname string) {
	temp_log_ready = false

	if match_auto_logconsole.value == 0 {
		return
	}

	if Log_IsLogging() {
		if autologging {
			autologging = false
			Log_Stop()
		} else {
			Com_Printf("Auto console logging skipped (already logging)\n")
			return
		}
	}

	log_auto_matchname = strlcpy(logname, 2*MAX_OSPATH)

	extendedname := COM_ForceExtension(strlcpy(TEMP_LOG_NAME, MAX_OSPATH*2), ".log")
	fullname := va("%s/%s", MT_TempDirectory(), extendedname)

	templog, err := logOpen(fullname)
	if err != nil {
		Com_Printf("Error: Couldn't open %s\n", fullname)
		return
	}

	Com_Printf("Auto console logging commenced\n")

	logfile = templog
	logwriter = bufio.NewWriter(templog)
	autologging = true
	log_auto_starttime = float32(cls.realtime)
}

func Log_AutoLogging_Status() int {
	if temp_log_ready {
		return 2
	}
	if autologging {
		return 1
	}
	return 0
}

func Log_AutoLogging_SaveMatch() {
	exts := []string{"log"}

	if !temp_log_ready {
		return
	}

	temp_log_ready = false

	dir := Log_LogDirectory()
	tempname := va("%s/%s", MT_TempDirectory(), TEMP_LOG_NAME)

	fullsavedname := va("%s/%s", dir, log_auto_matchname)
	num := Util_Extend_Filename(fullsavedname, exts)
	if num == -1 {
		Com_Printf("Error: no available filenames\n")
		return
	}
	savedname := snprintf(2*MAX_OSPATH, "%s_%03d.log", log_auto_matchname, num)

	fullsavedname = va("%s/%s", dir, savedname)

	f, err := os.Open(tempname)
	if err != nil {
		return
	}

	f.Close()

	if err = Sys_rename(tempname, fullsavedname); err != nil {
		FS_CreatePath(fullsavedname)
		err = Sys_rename(tempname, fullsavedname)
	}

	if err == nil {
		Com_Printf("Match console log saved to %s\n", savedname)
	}
}

package engine

import "time"

// host.c -- this is the client only build, so dedicated is a constant.

const dedicated = false

var curtime float64

var host_initialized bool // true if into command execution

// hostAbort is what longjmp(host_abort) carries in C.
type hostAbort struct{}

func Host_Abort() {
	panic(hostAbort{})
}

func Host_EndGame() {
	CL_Disconnect()

	// clear disconnect messages from loopback
	NET_ClearLoopback()
}

var host_inerror = false

// This shuts down both the client and server
func Host_Error(format string, a ...any) {
	if host_inerror {
		Sys_Error("Host_Error: recursively entered")
	}
	host_inerror = true

	str := strlcpy(va(format, a...), 1024)

	Com_Printf("\n===========================\n")
	Com_Printf("Host_Error: %s\n", str)
	Com_Printf("===========================\n\n")

	CL_Disconnect()

	if !host_initialized {
		Sys_Error("Host_Error: %s", str)
	}

	host_inerror = false

	Host_Abort()
}

// The hunk and zone are the Go heap; nothing to set up.
func Host_InitMemory() {
}

func Host_ShutdownMemory() {
}

func Host_Frame(time float64) {
	defer func() {
		// something bad happened, or the server disconnected
		if r := recover(); r != nil {
			if _, ok := r.(hostAbort); !ok {
				panic(r)
			}
		}
	}()

	rand()

	curtime += time

	CL_Frame(time) // will also call SV_Frame
}

var cvarsregged int

func Host_Init(argv []string) {
	COM_InitArgv(argv)

	Dump_Init()
	if Dump_Active() {
		srand(uint32(dump_seed))
	} else {
		srand(uint32(time.Now().Unix()))
	}

	Host_InitMemory()

	ReadableChars_Init()

	Cbuf_Init()
	Cmd_Init()
	Cvar_Init()
	COM_Init()
	FS_Init()

	VID_CvarInit()
	S_CvarInit()
	R_CommonCvarInit()
	Image_CvarInit()
	R_CvarInit()
	Con_CvarInit()
	SCR_CvarInit()
	if !dedicated {
		CL_CvarInit()
		CL_CvarInitInput()
		CL_CvarInitPrediction()
		CL_CvarInitCam()
		CL_CvarDemoInit()
	}
	Mouse_CvarInit()
	ConfigManager_CvarInit()
	Movie_CvarInit()
	MT_CvarInit()
	Sbar_CvarInit()
	TP_CvarInit()
	Ignore_CvarInit()
	V_CvarInit()
	Log_CvarInit()
	Netchan_CvarInit()
	Stats_CvarInit()
	Draw_CvarInit()
	Key_CvarInit()
	M_CvarInit()
	FMod_CvarInit()
	FChecks_CvarInit()
	Sys_CvarInit()
	SB_CvarInit()
	Lua_CvarInit()
	Ruleset_CvarInit()

	cvarsregged = 1

	Con_Init()

	FS_InitFilesystem()

	if !dedicated && (FS_FileExists("gfx/palette.lmp") == 0 || FS_FileExists("gfx/pop.lmp") == 0) {
		Sys_Error("Missing id1/pak0.pak or id1/pak1.pak")
	}

	COM_CheckRegistered()

	Con_Suppress()

	if dedicated {
		Cbuf_AddText("exec server.cfg\n")
		Cmd_StuffCmds_f() // process command line arguments
		Cbuf_Execute()
	} else {
		Cbuf_AddText("exec default.cfg\n")
		if FS_FileExists("config.cfg") != 0 {
			Cbuf_AddText("exec config.cfg\n")
		}
		if FS_FileExists("autoexec.cfg") != 0 {
			Cbuf_AddText("exec autoexec.cfg\n")
		}
		Cbuf_AddText("cfg_load default.cfg\n")
		Cbuf_Execute()
		Cmd_StuffCmds_f() // process command line arguments
		Cbuf_AddText("cl_warncmd 1\n")
	}

	Cmd_ParseLegacyCmdLineCmds()

	Cbuf_Execute()

	Con_Unsuppress()

	if !Mouse_Init() {
		Host_Error("Unable to initialise mouse input\n")
	}

	NET_Init()
	QLib_Init()
	Sys_Init()
	PM_Init()
	Mod_Init()
	Lua_Init()
	SB_Init()

	CL_Init()

	if !dedicated {
		CL_SaveArgv(argv)
	}

	host_initialized = true

	Cmd_EnableFunctionExecution()

	Com_Printf("Exe: %s\n", buildTime())
	Com_Printf("\nclassicQ v%s\n\n", VersionString())

	if dedicated {
		// if a map wasn't specified on the command line, spawn start map
		if !com_serveractive {
			Cmd_ExecuteString("map start")
		}
		if !com_serveractive {
			Host_Error("Couldn't spawn a server")
		}
	} else {
		VID_Open()
	}
}

var host_isdown = false

// FIXME: this is a callback from Sys_Quit and Sys_Error. It would be better
// to run quit through here before the final handoff to the sys code.
func Host_Shutdown() {
	if host_isdown {
		Sys_Printf("recursive shutdown\n")
		return
	}
	host_isdown = true

	SB_Quit()
	Lua_Shutdown()
	QLib_Shutdown()
	CL_Shutdown()
	Mod_Shutdown()
	FS_ShutdownFilesystem()
	NET_Shutdown()
	Mouse_Shutdown()
	Con_Shutdown()
	COM_Shutdown()
	Cvar_Shutdown()
	Cmd_Shutdown()
	Cbuf_Shutdown()
	Host_ShutdownMemory()
}

func Host_Quit() {
	Host_Shutdown()
	Sys_Quit()
}

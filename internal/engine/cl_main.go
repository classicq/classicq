package engine

import (
	"encoding/binary"
	"fmt"
	"os"
	"runtime"
	"strings"
)

// cl_main.c -- client main loop

var movementkey int

var rcon_password = cvar_t{name: "rcon_password", string: ""}
var rcon_address = cvar_t{name: "rcon_address", string: ""}

var con_clearnotify = cvar_t{name: "con_clearnotify", string: "1"}

var cl_timeout = cvar_t{name: "cl_timeout", string: "60"}

var cl_shownet = cvar_t{name: "cl_shownet", string: "0"} // can be 0, 1, or 2

var cl_sbar = cvar_t{name: "cl_sbar", string: "0", flags: CVAR_ARCHIVE}
var cl_hudswap = cvar_t{name: "cl_hudswap", string: "0", flags: CVAR_ARCHIVE}
var cl_maxfps = cvar_t{name: "cl_maxfps", string: "616", flags: CVAR_ARCHIVE}

var cl_predictPlayers = cvar_t{name: "cl_predictPlayers", string: "1"}
var cl_solidPlayers = cvar_t{name: "cl_solidPlayers", string: "1"}

var localid = cvar_t{name: "localid", string: ""}

var allowremotecmd = true

var cl_deadbodyfilter = cvar_t{name: "cl_deadbodyFilter", string: "0"}
var cl_gibfilter = cvar_t{name: "cl_gibFilter", string: "0"}
var cl_muzzleflash = cvar_t{name: "cl_muzzleflash", string: "1"}
var cl_rocket2grenade = cvar_t{name: "cl_r2g", string: "1"}
var cl_demospeed = cvar_t{name: "cl_demospeed", string: "1"}
var cl_staticsounds = cvar_t{name: "cl_staticSounds", string: "1"}
var cl_trueLightning = cvar_t{name: "cl_trueLightning", string: "1"}
var cl_parseWhiteText = cvar_t{name: "cl_parseWhiteText", string: "1"}
var cl_filterdrawviewmodel = cvar_t{name: "cl_filterdrawviewmodel", string: "0"}
var cl_oldPL = cvar_t{name: "cl_oldPL", string: "0"}
var cl_demoPingInterval = cvar_t{name: "cl_demoPingInterval", string: "5"}
var cl_chatsound = cvar_t{name: "cl_chatsound", string: "1"}
var qtv_chatprefix = cvar_t{name: "qtv_chatprefix", string: "$[QTV$] "}
var qtv_gamechatprefix = cvar_t{name: "qtv_gamechatprefix", string: "$[QTV>game$] "}
var qtv_skipchained = cvar_t{name: "qtv_skipchained", string: "1"}
var cl_confirmquit = cvar_t{name: "cl_confirmquit", string: "0"}
var default_fov = cvar_t{name: "default_fov", string: "0"}
var qizmo_dir = cvar_t{name: "qizmo_dir", string: "qizmo"}

var cl_floodprot = cvar_t{name: "cl_floodprot", string: "0"}
var cl_fp_messages = cvar_t{name: "cl_fp_messages", string: "4"}
var cl_fp_persecond = cvar_t{name: "cl_fp_persecond", string: "4"}
var cl_cmdline = cvar_t{name: "cl_cmdline", string: "", flags: CVAR_ROM}
var cl_useproxy = cvar_t{name: "cl_useproxy", string: "0"}

var net_maxfps = cvar_t{name: "net_maxfps", string: "0", OnChange: net_maxfps_callback}
var net_lag = cvar_t{name: "net_lag", string: "0", OnChange: net_lag_callback}
var net_lag_ezcheat = cvar_t{name: "net_lag_ezcheat", string: "0", OnChange: net_lag_ezcheat_callback}

var cl_imitate_client = cvar_t{name: "cl_imitate_client", string: "none", OnChange: cl_imitate_client_callback}
var cl_imitate_os = cvar_t{name: "cl_imitate_os", string: "none", OnChange: cl_imitate_os_callback}

var cl_model_bobbing = cvar_t{name: "cl_model_bobbing", string: "0"}
var cl_nolerp = cvar_t{name: "cl_nolerp", string: "0"}

var r_rocketlight = cvar_t{name: "r_rocketLight", string: "1"}
var r_rocketlightcolor = cvar_t{name: "r_rocketLightColor", string: "0"}
var r_explosionlightcolor = cvar_t{name: "r_explosionLightColor", string: "0"}
var r_explosionlight = cvar_t{name: "r_explosionLight", string: "1"}
var r_explosiontype = cvar_t{name: "r_explosionType", string: "8"}
var r_flagcolor = cvar_t{name: "r_flagColor", string: "0"}
var r_lightflicker = cvar_t{name: "r_lightflicker", string: "0"}
var r_rockettrail = cvar_t{name: "r_rocketTrail", string: "3"}
var r_grenadetrail = cvar_t{name: "r_grenadeTrail", string: "1"}
var r_powerupglow = cvar_t{name: "r_powerupGlow", string: "1"}

var r_drawflat_enable = cvar_t{name: "r_drawflat_enable", string: "0", OnChange: r_drawflat_enable_callback}
var r_drawflat_walls = cvar_t{name: "r_drawflat_walls", string: "off", OnChange: r_drawflat_walls_callback}
var r_drawflat_floors_ceilings = cvar_t{name: "r_drawflat_floors_ceilings", string: "off", OnChange: r_drawflat_floors_ceilings_callback}
var r_drawflat_slopes = cvar_t{name: "r_drawflat_slopes", string: "off", OnChange: r_drawflat_slopes_callback}

// info mirrors
var password = cvar_t{name: "password", string: "", flags: CVAR_USERINFO}
var spectator = cvar_t{name: "spectator", string: "0", flags: CVAR_USERINFO}
var name = cvar_t{name: "name", string: "unnamed", flags: CVAR_ARCHIVE | CVAR_USERINFO}
var team = cvar_t{name: "team", string: "icq", flags: CVAR_ARCHIVE | CVAR_USERINFO}
var topcolor = cvar_t{name: "topcolor", string: "1", flags: CVAR_ARCHIVE | CVAR_USERINFO}
var bottomcolor = cvar_t{name: "bottomcolor", string: "6", flags: CVAR_ARCHIVE | CVAR_USERINFO}
var skin = cvar_t{name: "skin", string: "", flags: CVAR_ARCHIVE | CVAR_USERINFO}
var rate = cvar_t{name: "rate", string: "50000", flags: CVAR_ARCHIVE | CVAR_USERINFO}
var msg = cvar_t{name: "msg", string: "1", flags: CVAR_ARCHIVE | CVAR_USERINFO}
var noaim = cvar_t{name: "noaim", string: "0", flags: CVAR_ARCHIVE | CVAR_USERINFO}
var w_switch = cvar_t{name: "w_switch", string: "", flags: CVAR_ARCHIVE | CVAR_USERINFO}
var b_switch = cvar_t{name: "b_switch", string: "", flags: CVAR_ARCHIVE | CVAR_USERINFO}

var imitatedclientnum int
var imitatedosnum int

// draw flat globals
var draw_flat [3][3]float32

var host_skipframe bool // used in demo playback

var fps_count int

var cl_vidinitialised bool

var cl_net_message_buffer [MAX_MSGLEN * 2]byte

func net_maxfps_callback(v *cvar_t, str string) bool {
	value := Q_atof(str)
	maxfps := cl.maxfps
	if maxfps == 0 {
		maxfps = 72
	}
	if value != 0 && value < maxfps {
		maxfps = value
	}
	if maxfps < 30 {
		maxfps = 30
	}
	if cls.netqw != nil {
		NetQW_SetFPS(cls.netqw, uint(maxfps))
	}
	return false
}

func net_lag_callback(v *cvar_t, str string) bool {
	value := Q_atof(str)
	if cls.netqw != nil {
		NetQW_SetLag(cls.netqw, uint(value*1000))
	}
	return false
}

func net_lag_ezcheat_callback(v *cvar_t, str string) bool {
	value := Q_atof(str)
	if cls.netqw != nil {
		NetQW_SetLagEzcheat(cls.netqw, b2i(value != 0))
	}
	return false
}

const (
	CLIENT_CLASSICQ = iota
	CLIENT_EZQUAKE_1144
	CLIENT_EZQUAKE_1517
	CLIENT_EZQUAKE_1_8_2
	CLIENT_FUHQUAKE_0_31_675
)

var validclientnames = []string{
	"none",
	"ezquake-1144",
	"ezquake-1517",
	"ezquake-1.8.2",
	"FuhQuake-0.31-675",
}

var fversion_clientnames = []string{
	"classicQ v" + CLASSICQ_VERSION,
	"ezQuake version 1144",
	"ezQuake version 1517",
	"ezQuake 1.8.2 stable (build 2029)",
	"FuhQuake version 0.31 (build 675)",
}

var validosnames = []string{
	"none",
	"Cygwin",
	"Linux",
	"MorphOS",
	"Win32",
}

var fversion_osnames = []string{
	QW_PLATFORM,
	"Cygwin",
	"Linux",
	"MorphOS",
	"Win32",
}

func cl_imitate_client_callback(v *cvar_t, str string) bool {
	i := 0
	for ; i < len(validclientnames); i++ {
		if Q_strcasecmp(validclientnames[i], str) == 0 {
			break
		}
	}

	if i == len(validclientnames) {
		Com_Printf("Client name \"%s\" is invalid. Valid client names are:\n", str)
		for i := 0; i < len(validclientnames); i++ {
			Com_Printf(" - %s\n", validclientnames[i])
		}
		return true
	}

	if Q_strcasecmp(str, validclientnames[imitatedclientnum]) == 0 {
		return false
	}

	imitatedclientnum = i

	CL_InitClientVersionInfo()

	Com_Printf("Client information will be updated after (re)connecting to a server.\n")

	return false
}

func cl_imitate_os_callback(v *cvar_t, str string) bool {
	i := 0
	for ; i < len(validosnames); i++ {
		if Q_strcasecmp(validosnames[i], str) == 0 {
			break
		}
	}

	if i == len(validosnames) {
		Com_Printf("OS name \"%s\" is invalid. Valid OS names are:\n", str)
		for i := 0; i < len(validosnames); i++ {
			Com_Printf(" - %s\n", validosnames[i])
		}
		return true
	}

	imitatedosnum = i

	return false
}

var emodel_name = "emodel"
var pmodel_name = "pmodel"

//============================================================================

func CL_Macro_ConnectionType() string {
	var s string
	if cls.state < ca_connected {
		s = "disconnected"
	} else if cl.spectator != 0 {
		s = "spectator"
	} else {
		s = "player"
	}
	return strlcpy(s, 16)
}

func CL_Macro_Demoplayback() string {
	var s string
	if cls.mvdplayback {
		s = "mvdplayback"
	} else if cls.demoplayback {
		s = "qwdplayback"
	} else {
		s = "0"
	}
	return strlcpy(s, 16)
}

func CL_Macro_Serverstatus() string {
	var s string
	if cls.state < ca_connected {
		s = "disconnected"
	} else if cl.standby {
		s = "standby"
	} else {
		s = "normal"
	}
	return strlcpy(s, 16)
}

func CL_ClientState() int {
	return int(cls.state)
}

func CL_MakeActive() {
	cls.state = ca_active
	if cls.demoplayback {
		host_skipframe = true
		demostarttime = cls.demotime
	}

	if !cls.demoplayback {
		VID_SetCaption(va("classicQ: %s", cls.servername))
	}

	Con_ClearNotify()
	TP_ExecTrigger("f_spawn")
	Ignore_PostNewMap()

	if key_dest == key_game {
		VID_SetMouseGrab(1)
	}
}

// Cvar system calls this when a CVAR_USERINFO cvar changes
func CL_UserinfoChanged(key, str string) {
	s := TP_ParseFunChars(str, false)

	if s != Info_ValueForKey(cls.userinfo, key) {
		cls.userinfo = Info_SetValueForKey(cls.userinfo, key, s, MAX_INFO_STRING)

		if cls.state >= ca_connected {
			if cls.netqw != nil {
				buf := fmt.Sprintf("%csetinfo \"%s\" \"%s\"\n", clc_stringcmd, key, s)
				if len(buf) < 1400 {
					NetQW_AppendReliableBuffer(cls.netqw, append([]byte(buf), 0))
				}
			}
		}
	}
}

func CL_BeginServerConnect() {
	Ruleset_Activate()

	cls.netqw = NetQW_Create(cls.servername, cls.userinfo, uint16(rand()&0xffff), FTEX_SUPPORTED)
	if cls.netqw != nil {
		Com_Printf("Connecting to %s...\n", cls.servername)
		NetQW_SetLag(cls.netqw, uint(net_lag.value*1000))
		NetQW_SetLagEzcheat(cls.netqw, b2i(net_lag_ezcheat.value != 0))
	} else {
		Com_Printf("Unable to create connection\n")
	}

	// This is currently needed for /packet to work.
	if !NET_StringToAdr(nil, cls.servername, &cls.server_adr) {
		Com_Printf("Bad server address\n")
		return
	}

	if !NET_OpenSocket(NS_CLIENT, cls.server_adr.type_) {
		Com_Printf("Unable to create socket\n")
		return
	}
}

func CL_Connect_f() {
	if Cmd_Argc() != 2 {
		Com_Printf("Usage: %s <server>\n", Cmd_Argv(0))
		return
	}

	proxy := cl_useproxy.value != 0 && CL_ConnectedToProxy()

	if proxy {
		Cbuf_AddText(va("say ,connect %s", Cmd_Argv(1)))
	} else {
		Host_EndGame()
		cls.servername = strlcpy(Cmd_Argv(1), MAX_OSPATH)
		CL_BeginServerConnect()
	}
}

func CL_ConnectedToProxy() bool {
	qizmo_aliases := []string{"ezcomp", "ezcomp2", "ezcomp3", "f_sens", "f_fps", "f_tj", "f_ta"}

	if cls.state < ca_active {
		return false
	}

	for _, s := range qizmo_aliases {
		alias := Cmd_FindAlias(s)
		if alias == nil || alias.flags&ALIAS_SERVER == 0 {
			return false
		}
	}

	return true
}

func CL_Join_f() {
	if cls.demoplayback {
		Com_Printf("Playing a demo, can't join.\n")
		return
	}

	proxy := cl_useproxy.value != 0 && CL_ConnectedToProxy()

	if Cmd_Argc() > 2 {
		Com_Printf("Usage: %s [server]\n", Cmd_Argv(0))
		return
	}

	Cvar_Set(&spectator, "")

	if Cmd_Argc() == 2 {
		if proxy {
			Cbuf_AddText(va("say ,connect %s\n", Cmd_Argv(1)))
		} else {
			Cbuf_AddText(va("connect %s\n", Cmd_Argv(1)))
		}
	} else if cl.z_ext&Z_EXT_JOIN_OBSERVE != 0 {
		Cbuf_AddText("cmd join\n")
	} else if proxy {
		Cbuf_AddText("say ,reconnect\n")
	} else {
		Cbuf_AddText("reconnect\n")
	}
}

func CL_Observe_f() {
	if cls.demoplayback {
		Com_Printf("Playing a demo, can't observe.\n")
		return
	}

	proxy := cl_useproxy.value != 0 && CL_ConnectedToProxy()

	if Cmd_Argc() > 2 {
		Com_Printf("Usage: %s [server]\n", Cmd_Argv(0))
		return
	}

	if spectator.string == "" || spectator.string == "0" {
		Cvar_SetValue(&spectator, 1)
	}

	if Cmd_Argc() == 2 {
		if proxy {
			Cbuf_AddText(va("say ,connect %s\n", Cmd_Argv(1)))
		} else {
			Cbuf_AddText(va("connect %s\n", Cmd_Argv(1)))
		}
	} else if cl.z_ext&Z_EXT_JOIN_OBSERVE != 0 {
		Cbuf_AddText("cmd observe\n")
	} else if proxy {
		Cbuf_AddText("say ,reconnect\n")
	} else {
		Cbuf_AddText("reconnect\n")
	}
}

func CL_QTVPlay_f() {
	if Cmd_Argc() != 2 {
		Com_Printf("Usage: %s <stream>\n", Cmd_Argv(0))
		return
	}

	buf := strlcpy(fmt.Sprintf("alias f_qtv \"say .qtv %s\"; connect $sb_qtv_proxy\n", Cmd_Argv(1)), 512)
	Cbuf_AddText(buf)
}

func CL_DNS_f() {
	var addr netaddr

	if Cmd_Argc() != 2 {
		Com_Printf("Usage: %s <address>\n", Cmd_Argv(0))
		return
	}

	address := strlcpy(Cmd_Argv(1), 128)

	if !NET_StringToAdr(nil, address, &addr) {
		Com_Printf("Couldn't look up \"%s\"\n", address)
	} else {
		Com_Printf("IP: %s\n", NET_BaseAdrToString(&addr))
		if cname, ok := NET_GetHostnameForAddress(&addr); ok {
			Com_Printf("Hostname: %s\n", cname)
		}
	}
}

func CL_ClearState() {
	S_StopAllSounds(true)

	Com_DPrintf("Clearing memory\n")

	CL_CleanupActiveDownload(false)
	cls.downloadtype = dl_none
	cls.downloadnumber = 0

	CL_ClearTEnts()
	CL_ClearScene()
	CL_FreeStatics()

	// wipe the entire cl structure
	cl = clientState_t{}

	SZ_Clear(&cls.netchan.message)

	// clear other arrays
	cl_efrags = [MAX_EFRAGS]efrag_t{}
	cl_dlight_active = [MAX_DLIGHTS / 32]uint32{}
	cl_lightstyle = [MAX_LIGHTSTYLES]lightstyle_t{}
	cl_entities = [CL_MAX_EDICTS]centity_t{}

	// make sure no centerprint messages are left from previous level
	scr_centertime_off = 0

	// allocate the efrags and chain together into a free list
	cl.free_efrags = &cl_efrags[0]
	for i := 0; i < MAX_EFRAGS-1; i++ {
		cl_efrags[i].entnext = &cl_efrags[i+1]
	}
	cl_efrags[MAX_EFRAGS-1].entnext = nil
}

// Sends a disconnect message to the server
// This is also called on Host_Error, so it shouldn't cause any errors
func CL_Disconnect() {
	cl.teamfortress = false
	cls.ftexsupported = 0
	cls.fte2supported = 0
	cls.mvdprotocolextensions1 = 0

	VID_SetCaption("classicQ")

	// stop sounds (especially looping!)
	S_StopAllSounds(true)

	MT_Disconnect()

	if cls.demorecording && cls.state != ca_disconnected {
		CL_Stop_f()
	}

	if cls.demoplayback {
		CL_StopPlayback()
	} else if cls.state != ca_disconnected {
		final := []byte{clc_stringcmd, 'd', 'r', 'o', 'p', 0}
		Netchan_Transmit(&cls.netchan, final)
		Netchan_Transmit(&cls.netchan, final)
		Netchan_Transmit(&cls.netchan, final)
	}

	cls.netchan = netchan_t{}
	cls.server_adr = netaddr{}
	cls.state = ca_disconnected

	VID_SetMouseGrab(0)

	if cls.netqw != nil {
		NetQW_Delete(cls.netqw)
		cls.netqw = nil
	}

	Cam_Reset()

	CL_CleanupActiveDownload(false)
	cls.downloadtype = dl_none
	cls.downloadnumber = 0

	CL_StopUpload()
	DeleteServerAliases()

	// Restore the rate if it has been overridden by the server
	if Info_ValueForKey(cls.userinfo, "rate") != rate.string {
		CL_UserinfoChanged("rate", rate.string)
	}
}

func CL_Disconnect_f() {
	cl.intermission = 0
	Host_EndGame()
}

// The server is changing levels
func CL_Reconnect_f() {
	if cls.download != nil { // don't change when downloading
		return
	}

	S_StopAllSounds(true)

	if cls.state == ca_connected {
		Com_Printf("reconnecting...\n")
		if cls.netqw != nil {
			NetQW_AppendReliableBuffer(cls.netqw, []byte{clc_stringcmd, 'n', 'e', 'w', 0})
		}
		return
	}

	if cls.servername == "" {
		Com_Printf("No server to reconnect to.\n")
		return
	}

	Host_EndGame()
	CL_BeginServerConnect()
}

// the leading int of the current network message, as C reads it in place
func cl_net_message_id() int32 {
	return int32(binary.LittleEndian.Uint32(cl_net_message.data[:4]))
}

// Responses to broadcasts, etc
func CL_ConnectionlessPacket() {
	MSG_BeginReading(&cl_net_message)
	MSG_ReadLong() // skip the -1

	c := MSG_ReadByte()
	if msg_badread {
		return // runt packet
	}

	switch c {
	case A2C_CLIENT_COMMAND: // remote command from gui front end
		Com_Printf("%s: client command\n", NET_AdrToString(&cl_net_from))

		if !NET_IsLocalAddress(&cl_net_from) {
			Com_Printf("Command packet from remote host.  Ignored.\n")
			return
		}

		cmdtext := strlcpy(MSG_ReadString(), 2048)

		s := strings.Trim(MSG_ReadString(), " \t\n\v\f\r")

		if !allowremotecmd && (localid.string == "" || localid.string != s) {
			if localid.string == "" {
				Com_Printf("===========================\n")
				Com_Printf("Command packet received from local host, but no " +
					"localid has been set.  You may need to upgrade your server " +
					"browser.\n")
				Com_Printf("===========================\n")
			} else {
				Com_Printf("===========================\n")
				Com_Printf("Invalid localid on command packet received from local host. "+
					"\n|%s| != |%s|\n"+
					"You may need to reload your server browser and FuhQuake.\n",
					s, localid.string)
				Com_Printf("===========================\n")
				Cvar_Set(&localid, "")
			}
		} else {
			Cbuf_AddText(cmdtext)
			Cbuf_AddText("\n")
			allowremotecmd = false
		}

	case A2C_PRINT: // print command from somewhere
		remaining := cl_net_message.cursize - msg_readcount
		if remaining >= 6 && string(cl_net_message.data[msg_readcount:msg_readcount+6]) == "\\chunk" {
			msg_readcount += 6
			CL_ParseChunkedDownloadOOB()
			break
		}
		Com_Printf("%s: print\n", NET_AdrToString(&cl_net_from))
		Com_Printf("%s", MSG_ReadString())

	case svc_disconnect:
		if cls.demoplayback {
			Com_Printf("\n======== End of demo ========\n\n")
			Host_EndGame()
			Host_Abort()
		}
	}
}

func CL_DoNetQWStuff() {
	var startframe, numframes uint

	NetQW_GenerateFrames(cls.netqw)
	NetQW_CopyFrames(cls.netqw, &cl.frames, &cls.netchan.outgoing_sequence, &startframe, &numframes)

	for numframes != 0 {
		Cam_FinishMove(&cl.frames[startframe].cmd)

		if cls.demorecording {
			CL_WriteDemoCmd(&cl.frames[startframe].cmd)
		}

		startframe++
		startframe &= UPDATE_MASK
		numframes--
	}
}

// Handles playback of demos, on top of NET_ code
func CL_GetMessage() bool {
	if runtime.GOOS == "windows" {
		CL_CheckQizmoCompletion()
	}

	if cls.demoplayback {
		return CL_GetDemoMessage()
	}

	if cls.netqw != nil {
		if !cls.extensionsqueried {
			if NetQW_GetExtensions(cls.netqw, &cls.ftexsupported) {
				cls.extensionsqueried = true
			}
		}

		size := NetQW_GetPacketLength(cls.netqw)
		if size != 0 {
			if size > uint(len(cl_net_message_buffer)) {
				size = uint(len(cl_net_message_buffer))
			}

			p := NetQW_GetPacketData(cls.netqw)
			copy(cl_net_message_buffer[:], p[:size])
			NetQW_FreePacket(cls.netqw)

			SZ_Init(&cl_net_message, cl_net_message_buffer[:], len(cl_net_message_buffer))
			cl_net_message.cursize = int(size)

			if cls.state == ca_disconnected {
				cls.state = ca_connected
			}

			CL_DoNetQWStuff()

			return true
		}
	}

	return false
}

func CL_ReadPackets() {
	// Check connectionless messages (such as rcon) over the non-game socket
	for NET_GetPacket(NS_CLIENT, &cl_net_message, &cl_net_from) {
		if cl_net_message_id() == -1 {
			CL_ConnectionlessPacket()
		}
	}

	for CL_GetMessage() {
		// remote command packet
		if cl_net_message_id() == -1 {
			CL_ConnectionlessPacket()
			continue
		}

		if cl_net_message.cursize < 8 && !cls.mvdplayback {
			Com_DPrintf("%s: Runt packet\n", NET_AdrToString(&cl_net_from))
			continue
		}

		if cls.mvdplayback {
			MSG_BeginReading(&cl_net_message)
		} else {
			MSG_BeginReading(&cl_net_message)
			cls.netchan.incoming_sequence = MSG_ReadLong() & 0x7fffffff
			cls.netchan.incoming_acknowledged = MSG_ReadLong() & 0x7fffffff
		}

		CL_ParseServerMessage()
	}
}

func ToggleConsole_f() {
	Key_ClearTyping()

	if key_dest == key_console {
		if !SCR_NEED_CONSOLE_BACKGROUND() {
			key_dest = key_game
			VID_SetMouseGrab(1)
		}
	} else {
		key_dest = key_console
		VID_SetMouseGrab(0)
	}

	if con_clearnotify.value != 0 {
		Con_ClearNotify()
	}
}

//=============================================================================

func CL_SaveArgv(argv []string) {
	saved_args := ""
	total_length := 0
	first := true

	for _, arg := range argv {
		if arg == "" {
			continue
		}

		if !first && total_length+1 < 512 {
			saved_args += " "
			total_length++
		}
		first = false

		length := len(arg)
		if total_length+length < 512 {
			saved_args += arg
			total_length += length
		}
	}

	Cvar_ForceSet(&cl_cmdline, saved_args)
}

func CL_CvarInit() {
	Cvar_SetCurrentGroup(CVAR_GROUP_CHAT)
	Cvar_Register(&cl_parseWhiteText)
	Cvar_Register(&cl_chatsound)
	Cvar_Register(&qtv_chatprefix)
	Cvar_Register(&qtv_gamechatprefix)
	Cvar_Register(&qtv_skipchained)
	Cvar_Register(&cl_floodprot)
	Cvar_Register(&cl_fp_messages)
	Cvar_Register(&cl_fp_persecond)

	Cvar_SetCurrentGroup(CVAR_GROUP_SCREEN)
	Cvar_Register(&cl_shownet)

	Cvar_SetCurrentGroup(CVAR_GROUP_SBAR)
	Cvar_Register(&cl_sbar)
	Cvar_Register(&cl_hudswap)

	Cvar_SetCurrentGroup(CVAR_GROUP_VIEWMODEL)
	Cvar_Register(&cl_filterdrawviewmodel)

	Cvar_SetCurrentGroup(CVAR_GROUP_EYECANDY)
	Cvar_Register(&cl_model_bobbing)
	Cvar_Register(&cl_nolerp)
	Cvar_Register(&cl_maxfps)
	Cvar_Register(&cl_deadbodyfilter)
	Cvar_Register(&cl_gibfilter)
	Cvar_Register(&cl_muzzleflash)
	Cvar_Register(&cl_rocket2grenade)
	Cvar_Register(&r_explosiontype)
	Cvar_Register(&r_lightflicker)
	Cvar_Register(&r_rockettrail)
	Cvar_Register(&r_grenadetrail)
	Cvar_Register(&r_powerupglow)
	Cvar_Register(&r_rocketlight)
	Cvar_Register(&r_explosionlight)
	Cvar_Register(&r_rocketlightcolor)
	Cvar_Register(&r_explosionlightcolor)
	Cvar_Register(&r_flagcolor)
	Cvar_Register(&cl_trueLightning)

	Cvar_SetCurrentGroup(CVAR_GROUP_DEMO)
	Cvar_Register(&cl_demospeed)
	Cvar_Register(&cl_demoPingInterval)
	Cvar_Register(&qizmo_dir)

	Cvar_SetCurrentGroup(CVAR_GROUP_SOUND)
	Cvar_Register(&cl_staticsounds)

	Cvar_SetCurrentGroup(CVAR_GROUP_USERINFO)
	Cvar_Register(&team)
	Cvar_Register(&spectator)
	Cvar_Register(&skin)
	Cvar_Register(&rate)
	Cvar_Register(&noaim)
	Cvar_Register(&name)
	Cvar_Register(&msg)
	Cvar_Register(&topcolor)
	Cvar_Register(&bottomcolor)
	Cvar_Register(&w_switch)
	Cvar_Register(&b_switch)

	Cvar_SetCurrentGroup(CVAR_GROUP_VIEW)
	Cvar_Register(&default_fov)

	Cvar_SetCurrentGroup(CVAR_GROUP_NETWORK)
	Cvar_Register(&cl_predictPlayers)
	Cvar_Register(&cl_solidPlayers)
	Cvar_Register(&cl_oldPL)
	Cvar_Register(&cl_timeout)
	Cvar_Register(&cl_useproxy)
	Cvar_Register(&net_maxfps)
	Cvar_Register(&net_lag)
	Cvar_Register(&net_lag_ezcheat)

	Cvar_SetCurrentGroup(CVAR_GROUP_CONSOLE)
	Cvar_Register(&con_clearnotify)

	Cvar_SetCurrentGroup(CVAR_GROUP_NO_GROUP)
	Cvar_Register(&password)
	Cvar_Register(&rcon_password)
	Cvar_Register(&rcon_address)
	Cvar_Register(&localid)
	Cvar_Register(&cl_warncmd)
	Cvar_Register(&cl_cmdline)
	Cvar_ResetCurrentGroup()

	Cvar_Register(&cl_confirmquit)

	Cvar_Register(&cl_imitate_client)
	Cvar_Register(&cl_imitate_os)

	Cvar_Register(&r_drawflat_enable)
	Cvar_Register(&r_drawflat_floors_ceilings)
	Cvar_Register(&r_drawflat_slopes)
	Cvar_Register(&r_drawflat_walls)

	Cmd_AddLegacyCommand("demotimescale", "cl_demospeed")
	Cmd_AddLegacyCommand("cl_fakeshaft", "cl_trueLightning")

	CL_InitCommands()

	Cmd_AddCommand("r_drawflat", R_DrawFlat_f)
	Cmd_AddCommand("r_drawflat_shoot", R_DrawFlatShoot_f)
	Cmd_AddCommand("r_drawflat_shoot_unset", R_DrawFlatShootUnset_f)
	Cmd_AddCommand("r_drawflat_set", R_DrawFlat_Set_f)
	Cmd_AddCommand("r_drawflat_unset", R_DrawFlat_Unset_f)
	Cmd_AddCommand("r_drawflat_writeconfig", R_DrawFlat_Write_Config)
	Cmd_AddCommand("disconnect", CL_Disconnect_f)
	Cmd_AddCommand("connect", CL_Connect_f)
	Cmd_AddCommand("join", CL_Join_f)
	Cmd_AddCommand("observe", CL_Observe_f)
	Cmd_AddCommand("qtvplay", CL_QTVPlay_f)
	Cmd_AddCommand("dns", CL_DNS_f)
	Cmd_AddCommand("reconnect", CL_Reconnect_f)
	Cmd_AddMacro("connectiontype", CL_Macro_ConnectionType)
	Cmd_AddMacro("demoplayback", CL_Macro_Demoplayback)
	Cmd_AddMacro("matchstatus", CL_Macro_Serverstatus)
	Cmd_AddCommand("toggleconsole", ToggleConsole_f)
}

func CL_InitClientVersionInfo() {
	cls.userinfo = Info_RemoveKey(cls.userinfo, "*FuhQuake")
	cls.userinfo = Info_RemoveKey(cls.userinfo, "*client")
	cls.userinfo = Info_RemoveKey(cls.userinfo, "*Fodquake")
	cls.userinfo = Info_RemoveKey(cls.userinfo, "*classicQ")

	if imitatedclientnum == CLIENT_EZQUAKE_1144 {
		cls.userinfo = Info_SetValueForStarKey(cls.userinfo, "*client", "ezQuake 1144", MAX_INFO_STRING)
	} else if imitatedclientnum == CLIENT_EZQUAKE_1517 {
		cls.userinfo = Info_SetValueForStarKey(cls.userinfo, "*client", "ezQuake 1517", MAX_INFO_STRING)
	} else if imitatedclientnum == CLIENT_EZQUAKE_1_8_2 {
		cls.userinfo = Info_SetValueForStarKey(cls.userinfo, "*client", "ezQuake 2029", MAX_INFO_STRING)
	} else if imitatedclientnum == CLIENT_FUHQUAKE_0_31_675 {
		cls.userinfo = Info_SetValueForStarKey(cls.userinfo, "*FuhQuake", "0.31", MAX_INFO_STRING)
	} else {
		cls.userinfo = Info_SetValueForStarKey(cls.userinfo, "*classicQ", CLASSICQ_VERSION, MAX_INFO_STRING)
	}
}

func CL_Init() {
	if dedicated {
		return
	}

	SZ_Init(&cl_net_message, cl_net_message_buffer[:], len(cl_net_message_buffer))

	cls.state = ca_disconnected
	cls.gamedirfile = com_gamedirfile
	cls.gamedir = com_gamedir

	W_LoadWadFile("gfx.wad")

	FChecks_Init()

	host_basepal = FS_LoadMallocFile("gfx/palette.lmp")
	if host_basepal == nil {
		Sys_Error("Couldn't load gfx/palette.lmp")
	}
	FMod_CheckModel("gfx/palette.lmp", host_basepal, com_filesize)
	host_colormap = FS_LoadMallocFile("gfx/colormap.lmp")
	if host_colormap == nil {
		Sys_Error("Couldn't load gfx/colormap.lmp")
	}
	FMod_CheckModel("gfx/colormap.lmp", host_colormap, com_filesize)

	Sys_IO_Create_Directory(va("%s/qw", com_basedir))
	Sys_IO_Create_Directory(va("%s/classicq", com_basedir))

	Key_Init()
	V_Init()

	VID_Init(host_basepal)
	cl_vidinitialised = true
	Image_Init()
	S_Init()
	CDAudio_Init()

	cls.userinfo = Info_SetValueForStarKey(cls.userinfo, "*z_ext", va("%d", SUPPORTED_EXTENSIONS), MAX_SERVERINFO_STRING)

	CL_InitClientVersionInfo()

	if !CL_InitEnts() {
		Sys_Error("CL_InitEnts() failed\n")
	}
	CL_InitTEnts()

	MT_Init()
	CL_Demo_Init()
	Ignore_Init()
	Stats_Init()
	Sleep_Init()
	M_Init()
}

//============================================================================

func CL_BeginLocalConnection() {
	S_StopAllSounds(true)

	// make sure we're not connected to an external server,
	// and demo playback is stopped
	if !com_serveractive {
		CL_Disconnect()
	}

	cl.worldmodel = nil

	if cls.state == ca_active {
		cls.state = ca_connected
	}
}

func CL_MinFrameTime() float64 {
	var fps float64

	if cls.timedemo || Movie_IsCapturing() {
		return 0
	}

	if cls.demoplayback {
		if cl_maxfps.value == 0 {
			return 0
		}
		fps = max(30.0, float64(cl_maxfps.value))
	} else {
		if cl_maxfps.value == 0 {
			return 0
		}
		fps = max(30.0, float64(cl_maxfps.value))
	}

	return 1 / fps
}

var cl_extratime = 0.001

func CL_Frame(time float64) {
	var key keynum_t
	var down bool

	cl_extratime += time

	minframetime := CL_MinFrameTime()

	if cl_extratime < minframetime {
		Sleep_Sleep(uint((minframetime - cl_extratime) * 1000000))
		return
	}

	cls.trueframetime = minframetime
	cl_extratime -= minframetime
	if cl_extratime > minframetime {
		cls.trueframetime += cl_extratime - minframetime
		cl_extratime = minframetime
	}

	cls.framedev = cl_extratime

	// Ugliest hack ever
	movementkey = rand()

	if Movie_IsCapturing() {
		cls.frametime = Movie_StartFrame()
	} else {
		cls.frametime = min(0.2, cls.trueframetime)
	}

	if cls.demoplayback {
		if cl.paused&PAUSED_DEMO != 0 {
			cls.frametime = 0
		} else if !cls.timedemo {
			cls.frametime *= float64(bound(float32(0), cl_demospeed.value, float32(20)))
		}
		if !host_skipframe {
			cls.demotime += cls.frametime
		}
		host_skipframe = false
	}

	// a -dump run uses the frame clock, not the wall clock
	if Dump_Active() {
		cls.realtime += cls.frametime
	} else {
		cls.realtime = Sys_DoubleTime()
	}

	if cl.paused == 0 {
		cl.time += cls.frametime
		cl.servertime += cls.frametime
		cl.stats[STAT_TIME] = int(cl.servertime * 1000)
		cl.gametime += cls.frametime
	}

	if cls.demoplayback {
		cl.gametime = cls.realactualdemotime - cls.demotimeoffset
	}

	focuschanged := VID_FocusChanged()

	M_PerFramePreRender()

	for VID_GetKeyEvent(&key, &down) {
		Key_Event(int(key), down)
	}

	if focuschanged {
		Key_ClearStates()
	}

	// process console commands
	Cbuf_Execute()

	// fetch results from server
	CL_ReadPackets()

	if cls.mvdplayback {
		MVD_Interpolate()
	}

	// process stuffed commands
	Cbuf_ExecuteEx(cbuf_svc)

	if !cls.demoplayback || cls.mvdplayback {
		Mouse_GetViewAngles(&cl.viewangles)
	}

	if cls.netqw != nil {
		CL_DoNetQWStuff()
		CL_SendChunkedDownloadRequests()
		if cl.spectator != 0 {
			var cmd usercmd_t
			Cam_Track(&cmd)
		}
	} else if cls.mvdplayback {
		frame := &cl.frames[cls.netchan.outgoing_sequence&UPDATE_MASK]
		cmd := &frame.cmd
		frame.senttime = cls.realtime
		frame.receivedtime = -1

		CL_BaseMove(cmd)

		// if we are spectator, try autocam
		if cl.spectator != 0 {
			Cam_Track(cmd)
		}

		CL_FinishMove(cmd)
		Cam_FinishMove(cmd)

		cls.netchan.outgoing_sequence++
	}

	TP_Frame()

	if cls.state >= ca_onserver { // !!! Tonik
		Cam_SetViewPlayer()

		// Set up prediction for other players
		CL_SetUpPlayerPrediction(false)

		// do client side motion prediction
		CL_PredictMove()

		// Set up prediction for other players
		CL_SetUpPlayerPrediction(true)

		// build a refresh entity list
		CL_EmitEntities()
	}

	Dump_Frame()

	// update video
	SCR_UpdateScreen()

	CL_DecayLights()

	// update audio
	if cls.state == ca_active {
		S_Update(r_origin, vpn, vright, vup)
	} else {
		S_Update(vec3_origin, vec3_origin, vec3_origin, vec3_origin)
	}

	CDAudio_Update()

	MT_Frame()

	Lua_Frame()

	if Movie_IsCapturing() {
		Movie_FinishFrame()
	}

	cls.framecount++

	fps_count++
}

//============================================================================

func CL_Shutdown() {
	M_Shutdown()
	CL_Disconnect()
	CL_WriteConfiguration()
	CL_ShutdownEnts()
	Skin_Shutdown()
	CDAudio_Shutdown()
	S_Shutdown()
	TP_Shutdown()
	Log_Shutdown()
	if cl_vidinitialised {
		VID_Shutdown()
	}
	host_basepal = nil
	host_colormap = nil
	W_UnloadWadFile()
}

// ---- r_drawflat ----

func Point_On_Surface(points []vec3_t, count int, point vec3_t) int {
	var temp3, temp4, p, q vec3_t

	VectorSubtract(points[count-1], point, &temp3)
	VectorSubtract(points[0], point, &temp4)

	CrossProduct(temp3, temp4, &p)

	for i := 0; i < count-1; i++ {
		VectorSubtract(points[i], point, &temp3)
		VectorSubtract(points[i+1], point, &temp4)
		CrossProduct(temp3, temp4, &q)
		if DotProduct(p, q) < 0 {
			return 0
		}
	}

	return 1
}

func r_drawflat_enable_callback(v *cvar_t, str string) bool {
	return false
}

func r_drawflat_walls_callback(v *cvar_t, str string) bool {
	ts := Tokenize_String(str)
	if ts == nil {
		return true
	}

	if ts.count != 1 && ts.count != 3 {
		Com_Printf("usage: %s [r] [g] [b] or off, colors should be in the range of 0 to 1.\n", v.name)
		return true
	}

	if ts.count == 1 {
		if ts.tokens[0] != "off" {
			Com_Printf("usage: %s [r] [g] [b] or off, colors should be in the range of 0 to 1.\n", v.name)
			return true
		}
		R_Draw_Flat(-1, 1, -1, 1, 0, 0, 0, 0, 0, true)
		return false
	}

	if ts.count == 3 {
		r := float32(bound(0, atof(ts.tokens[0]), 1))
		g := float32(bound(0, atof(ts.tokens[1]), 1))
		b := float32(bound(0, atof(ts.tokens[2]), 1))

		draw_flat[0][0] = r
		draw_flat[0][1] = g
		draw_flat[0][2] = b

		R_Draw_Flat(-1, 1, -1, 1, 0, 0, r, g, b, false)
		return false
	}

	return true
}

func r_drawflat_floors_ceilings_callback(v *cvar_t, str string) bool {
	ts := Tokenize_String(str)
	if ts == nil {
		return true
	}

	if ts.count != 1 && ts.count != 3 {
		Com_Printf("usage: %s [r] [g] [b] or off, colors should be in the range of 0 to 1.\n", v.name)
		return true
	}

	if ts.count == 1 {
		if ts.tokens[0] != "off" {
			Com_Printf("usage: %s [r] [g] [b] or off, colors should be in the range of 0 to 1.\n", v.name)
			return true
		}
		R_Draw_Flat(-1, 1, -1, 1, -1, -1, 0, 0, 0, true)
		R_Draw_Flat(-1, 1, -1, 1, 1, 1, 0, 0, 0, true)
		return false
	}

	if ts.count == 3 {
		r := float32(bound(0, atof(ts.tokens[0]), 1))
		g := float32(bound(0, atof(ts.tokens[1]), 1))
		b := float32(bound(0, atof(ts.tokens[2]), 1))

		draw_flat[1][0] = r
		draw_flat[1][1] = g
		draw_flat[1][2] = b

		R_Draw_Flat(-1, 1, -1, 1, -1, -1, r, g, b, false)
		R_Draw_Flat(-1, 1, -1, 1, 1, 1, r, g, b, false)
		return false
	}

	return true
}

func r_drawflat_slopes_callback(v *cvar_t, str string) bool {
	ts := Tokenize_String(str)
	if ts == nil {
		return true
	}

	if ts.count != 1 && ts.count != 3 {
		Com_Printf("usage: %s [r] [g] [b] or off, colors should be in the range of 0 to 1.\n", v.name)
		return true
	}

	if ts.count == 1 {
		if ts.tokens[0] != "off" {
			Com_Printf("usage: %s [r] [g] [b] or off, colors should be in the range of 0 to 1.\n", v.name)
			return true
		}
		R_Draw_Flat(-1, 1, -1, 1, -0.99999, -0.00001, 0, 0, 0, true)
		R_Draw_Flat(-1, 1, -1, 1, 0.00001, 0.99999, 0, 0, 0, true)
		return false
	}

	if ts.count == 3 {
		r := float32(bound(0, atof(ts.tokens[0]), 1))
		g := float32(bound(0, atof(ts.tokens[1]), 1))
		b := float32(bound(0, atof(ts.tokens[2]), 1))

		draw_flat[2][0] = r
		draw_flat[2][1] = g
		draw_flat[2][2] = b

		R_Draw_Flat(-1, 1, -1, 1, -0.99999, -0.00001, r, g, b, false)
		R_Draw_Flat(-1, 1, -1, 1, 0.00001, 0.99999, r, g, b, false)
		return false
	}

	return true
}

func R_DrawFlat_UpdateSurface(model *model_t, surface *msurface_t) {
	if model.vertcolours != nil {
		if surface.polys != nil {
			for i := surface.polys.firstindex; i < surface.polys.firstindex+surface.polys.numverts; i++ {
				model.vertcolours[i*3+0] = surface.color[0]
				model.vertcolours[i*3+1] = surface.color[1]
				model.vertcolours[i*3+2] = surface.color[2]
			}

			model.surface_colours_dirty = true
		}
	}
}

// the surface polygon the view ray ends on, as in the two shoot commands
func drawflatShotSurfaces(model *model_t, trace *pmtrace_t, hit func(*msurface_t)) {
	leaf := Mod_PointInLeaf(trace.endpos, model)

	if leaf == nil {
		Com_Printf("no leaf returned\n")
		return
	}

	marksurface := leaf.firstmarksurfacenum
	for i := 0; i < leaf.nummarksurfaces; i, marksurface = i+1, marksurface+1 {
		surface := &model.surfaces[model.marksurfaces[marksurface]]

		points := make([]vec3_t, surface.numedges)
		for x := 0; x < surface.numedges; x++ {
			e := model.surfedges[surface.firstedge+x]
			if ae := e; (ae < 0 && -ae >= model.numedges) || ae >= model.numedges {
				return
			}

			var vertex *mvertex_t
			if e >= 0 {
				vertex = &model.vertexes[model.edges[e].v[0]]
			} else {
				vertex = &model.vertexes[model.edges[-e].v[1]]
			}
			points[x] = vertex.position
		}

		if Point_On_Surface(points, surface.numedges, trace.endpos) != 0 && trace.draw_plane == surface.plane {
			hit(surface)
		}
	}
}

func R_DrawFlatShoot_f() {
	var vec, vec1 vec3_t
	var color [3]float32

	if Cmd_Argc() != 4 {
		Com_Printf("Usage: r_drawflat_shoot [r] [g] [b], colors should be in range of 0 to 1\n")
		return
	}

	if cl.worldmodel == nil {
		Com_Printf("No map loaded, can't set surface color\n")
		return
	}

	color[0] = float32(bound(0, atof(Cmd_Argv(1)), 1))
	color[1] = float32(bound(0, atof(Cmd_Argv(2)), 1))
	color[2] = float32(bound(0, atof(Cmd_Argv(3)), 1))

	model := cl.worldmodel
	AngleVectors(r_refdef.viewangles, &vec1, nil, nil)

	VectorMA(r_refdef.vieworg, 10000.0, vec1, &vec)

	trace := PM_TraceLine(r_refdef.vieworg, vec)

	drawflatShotSurfaces(model, &trace, func(surface *msurface_t) {
		surface.is_drawflat = 1
		surface.color[0] = color[0]
		surface.color[1] = color[1]
		surface.color[2] = color[2]

		R_DrawFlat_UpdateSurface(model, surface)
	})
}

func R_DrawFlatShootUnset_f() {
	var vec, vec1 vec3_t

	if cl.worldmodel == nil {
		Com_Printf("No map loaded, can't set surface color\n")
		return
	}

	model := cl.worldmodel
	AngleVectors(r_refdef.viewangles, &vec1, nil, nil)

	VectorMA(r_refdef.vieworg, 10000.0, vec1, &vec)

	trace := PM_TraceLine(r_refdef.vieworg, vec)

	drawflatShotSurfaces(model, &trace, func(surface *msurface_t) {
		surface.is_drawflat = 0

		R_DrawFlat_UpdateSurface(model, surface)
	})
}

func R_Draw_Flat(x_lower_limit, x_upper_limit, y_lower_limit, y_upper_limit, z_lower_limit, z_upper_limit, r, g, b float32, unset bool) {
	if cl.worldmodel == nil {
		return
	}

	model := cl.worldmodel

	for i := 0; i < model.numsurfaces; i++ {
		vec := model.surfaces[i].plane.normal
		VectorNormalize(&vec)
		if vec[2] <= z_upper_limit && vec[2] >= z_lower_limit &&
			vec[1] <= y_upper_limit && vec[1] >= y_lower_limit &&
			vec[0] <= x_upper_limit && vec[0] >= x_lower_limit {
			if unset {
				model.surfaces[i].is_drawflat = 0
			} else {
				model.surfaces[i].is_drawflat = 1
				model.surfaces[i].color[0] = r
				model.surfaces[i].color[1] = g
				model.surfaces[i].color[2] = b
			}

			R_DrawFlat_UpdateSurface(model, &model.surfaces[i])
		}
	}
}

func R_DrawFlat_f() {
	var limit [6]float32
	var color [3]float32

	if Cmd_Argc() != 10 {
		Com_Printf("Usage: r_drawflat [x_lower_limit] [x_upper_limit] [y_lower_limit] [y_upper_limit] [z_lower_limit] [z_upper_limit] [r] [g] [b], limits go from -1 to 1, colors should be in the range of 0 to 1.\n")
		return
	}

	limit[0] = float32(atof(Cmd_Argv(1)))
	limit[1] = float32(atof(Cmd_Argv(2)))
	limit[2] = float32(atof(Cmd_Argv(3)))
	limit[3] = float32(atof(Cmd_Argv(4)))
	limit[4] = float32(atof(Cmd_Argv(5)))
	limit[5] = float32(atof(Cmd_Argv(6)))
	color[0] = float32(bound(0, atof(Cmd_Argv(7)), 1))
	color[1] = float32(bound(0, atof(Cmd_Argv(8)), 1))
	color[2] = float32(bound(0, atof(Cmd_Argv(9)), 1))

	R_Draw_Flat(limit[0], limit[1], limit[2], limit[3], limit[4], limit[5], color[0], color[1], color[2], true)
}

func R_DrawFlat_Set_f() {
	if Cmd_Argc() != 5 {
		Com_Printf("Usage: r_drawflat_set [surface] [r] [g] [b], colors should be in range of 0 to 1\n")
		return
	}

	if cl.worldmodel == nil {
		Com_Printf("No map loaded, can't set surface color\n")
		return
	}

	model := cl.worldmodel

	surface := atoi(Cmd_Argv(1))
	r := float32(bound(0, atof(Cmd_Argv(2)), 1))
	g := float32(bound(0, atof(Cmd_Argv(3)), 1))
	b := float32(bound(0, atof(Cmd_Argv(4)), 1))

	if surface < 0 || surface >= model.numsurfaces {
		Com_Printf("Surface out of range.\n")
		return
	}

	model.surfaces[surface].is_drawflat = 1
	model.surfaces[surface].color[0] = r
	model.surfaces[surface].color[1] = g
	model.surfaces[surface].color[2] = b

	R_DrawFlat_UpdateSurface(model, &model.surfaces[surface])
}

func R_DrawFlat_Unset_f() {
	if cl.worldmodel == nil {
		return
	}

	if Cmd_Argc() != 2 {
		Com_Printf("Usage: r_drawflat_unset [surface number]\n")
		return
	}

	model := cl.worldmodel

	surface := atoi(Cmd_Argv(1))

	if surface < 0 || surface >= model.numsurfaces {
		Com_Printf("Surface out of range.\n")
		return
	}

	model.surfaces[surface].is_drawflat = 0
}

func R_DrawFlat_Write_Config() {
	if cl.worldmodel == nil {
		return
	}

	model := cl.worldmodel

	file := va("%s/qw/%s.dfcfg", com_basedir, mapname.string)

	if f, err := os.Open(file); err == nil {
		f.Close()
		if Cmd_Argc() != 2 {
			Com_Printf("File \"%s\" exists. Please use \"r_drawflat_writeconfig overwrite\" to overwrite the old file.\n", mapname.string)
			return
		} else {
			if Cmd_Argv(1) != "overwrite" {
				Com_Printf("If you really want to overwrite please spell overwrite right.\n")
				return
			}
		}
	}

	f, err := os.Create(file)
	if err != nil {
		return
	}

	for i := 0; i < model.numsurfaces; i++ {
		if model.surfaces[i].is_drawflat == 1 {
			fmt.Fprintf(f, "r_drawflat_set %d %f %f %f\n", i, model.surfaces[i].color[0], model.surfaces[i].color[1], model.surfaces[i].color[2])
		}
	}
	f.Close()
}

func R_DrawFlat_NewMap() {
	if r_drawflat_enable.value != 1 {
		return
	}

	if r_drawflat_walls.string != "off" {
		R_Draw_Flat(-1, 1, -1, 1, 0, 0, draw_flat[0][0], draw_flat[0][1], draw_flat[0][2], false)
	}

	if r_drawflat_floors_ceilings.string != "off" {
		R_Draw_Flat(-1, 1, -1, 1, -1, -1, draw_flat[1][0], draw_flat[1][1], draw_flat[1][2], false)
		R_Draw_Flat(-1, 1, -1, 1, 1, 1, draw_flat[1][0], draw_flat[1][1], draw_flat[1][2], false)
	}

	if r_drawflat_slopes.string != "off" {
		R_Draw_Flat(-1, 1, -1, 1, -0.99999, -0.00001, draw_flat[2][0], draw_flat[2][1], draw_flat[2][2], false)
		R_Draw_Flat(-1, 1, -1, 1, 0.00001, 0.99999, draw_flat[2][0], draw_flat[2][1], draw_flat[2][2], false)
	}

	Cbuf_AddText(va("exec %s.dfcfg\n", mapname.string))
}

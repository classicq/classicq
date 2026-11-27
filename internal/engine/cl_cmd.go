// Copyright (C) 1996-1997 Id Software, Inc.
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
// See the GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program; if not, write to the Free Software
// Foundation, Inc., 59 Temple Place - Suite 330, Boston, MA  02111-1307, USA.

package engine

import (
	"fmt"
	"os"
	"strings"
)

// ASCII tolower per byte, as the C loops do it in place
func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}

// adds the current command line as a clc_stringcmd to the client message.
// things like kill, say, etc, are commands directed to the server,
// so when they are typed in at the console, they will need to be forwarded.
func Cmd_ForwardToServer() {
	if cls.state == ca_disconnected || cls.netqw == nil {
		Com_Printf("Can't \"%s\", not connected\n", Cmd_Argv(0))
		return
	}

	// lowercase command
	cmd_argv[0] = asciiLower(cmd_argv[0])

	var buf string
	if Cmd_Argc() > 1 {
		buf = fmt.Sprintf("%c%s %s", clc_stringcmd, Cmd_Argv(0), Cmd_Args())
	} else {
		buf = fmt.Sprintf("%c%s", clc_stringcmd, Cmd_Argv(0))
	}

	if len(buf) < 1500 {
		NetQW_AppendReliableBuffer(cls.netqw, append([]byte(buf), 0))
	}
}

// don't forward the first argument
func CL_ForwardToServer_f() {
	if cls.state == ca_disconnected || cls.netqw == nil {
		Com_Printf("Can't \"%s\", not connected\n", Cmd_Argv(0))
		return
	}

	if Q_strcasecmp(Cmd_Argv(1), "snap") == 0 {
		SCR_RSShot_f()
		return
	}

	if cls.demoplayback {
		return // not really connected
	}

	if Cmd_Argc() > 1 {
		buf := fmt.Sprintf("%c%s", clc_stringcmd, Cmd_Args())
		if len(buf) < 1500 {
			NetQW_AppendReliableBuffer(cls.netqw, append([]byte(buf), 0))
		}
	}
}

// Handles both say and say_team
func CL_Say_f() {
	qizmo := false

	if Cmd_Argc() < 2 {
		return
	}

	if cls.state == ca_disconnected || cls.netqw == nil {
		Com_Printf("Can't \"%s\", not connected\n", Cmd_Argv(0))
		return
	}

	// lowercase command
	cmd_argv[0] = asciiLower(cmd_argv[0])

	if CL_ConnectedToProxy() {
		s := strings.TrimLeft(Cmd_Argv(1), " ")
		if strings.HasPrefix(s, ".stuff") || strings.HasPrefix(s, ",stuff") || strings.Contains(s, ":stuff") {
			return
		}
		qizmo = strings.HasPrefix(s, "proxy:") || at(s, 0) == ',' || at(s, 0) == '.'
	}

	if !qizmo && cl_floodprot.value != 0 && cl_fp_messages.value > 0 && cl_fp_persecond.value > 0 {
		tmp := cl.whensaidhead - int(min(cl_fp_messages.value, 10)) + 1
		if tmp < 0 {
			tmp += 10
		}
		if cl.whensaid[tmp] != 0 && (cls.realtime-cl.whensaid[tmp]) < 1.02*float64(cl_fp_persecond.value) {
			Com_Printf("Flood Protection\n")
			return
		}
	}

	s := TP_ParseMacroString(Cmd_Args())
	s = TP_ParseFunChars(s, true)

	var buf string
	if at(s, 0) != 0 && at(s, 0) < 32 {
		buf = fmt.Sprintf("%c%s \"%s\"", clc_stringcmd, Cmd_Argv(0), s)
	} else {
		buf = fmt.Sprintf("%c%s %s", clc_stringcmd, Cmd_Argv(0), s)
	}
	if len(buf) < 1500 {
		NetQW_AppendReliableBuffer(cls.netqw, append([]byte(buf), 0))
	}

	if !qizmo {
		cl.whensaidhead++
		if cl.whensaidhead > 9 {
			cl.whensaidhead = 0
		}
		cl.whensaid[cl.whensaidhead] = cls.realtime
	}
}

func CL_Pause_f() {
	if cls.demoplayback {
		cl.paused ^= PAUSED_DEMO
	} else {
		Cmd_ForwardToServer()
	}
}

// packet <destination> <contents>
// Contents allows \n escape character
func CL_Packet_f() {
	var adr netaddr

	if Cmd_Argc() != 3 {
		Com_Printf("packet <destination> <contents>\n")
		return
	}

	if cbuf_current != nil && cbuf_current != cbuf_svc && !Ruleset_AllowPacketCmd() {
		Com_Printf("Packet commands is disabled during match\n")
		return
	}

	if !NET_StringToAdr(nil, Cmd_Argv(1), &adr) {
		Com_Printf("Bad address\n")
		return
	}

	if adr.type_ == NA_IPV4 && adr.ipv4.port == 0 {
		adr.ipv4.port = uint16(BigShort(PORT_SERVER))
	} else if adr.type_ == NA_IPV6 && adr.ipv6.port == 0 {
		// C tests the ipv4 half of the union here
		adr.ipv6.port = uint16(BigShort(PORT_SERVER))
	}

	send := make([]byte, 4, 2048)
	send[0], send[1], send[2], send[3] = 0xFF, 0xFF, 0xFF, 0xFF

	in := Cmd_Argv(2)
	i := 0
	for at(in, i) != 0 && len(send) < 2048-2 {
		if in[i] == '\\' && at(in, i+1) != 0 {
			switch in[i+1] {
			case 'n':
				send = append(send, '\n')
			case 't':
				send = append(send, '\t')
			case '\\':
				send = append(send, '\\')
			default:
				send = append(send, in[i], in[i+1])
			}
			i += 2
		} else {
			send = append(send, in[i])
			i++
		}
	}

	NET_SendPacket(NS_CLIENT, send, &adr)
}

// Send the rest of the command line over as an unconnected command.
func CL_Rcon_f() {
	var to netaddr

	message := "\xff\xff\xff\xff"
	message = strlcat(message, "rcon ", 1024)

	if rcon_password.string != "" {
		message = strlcat(message, rcon_password.string, 1024)
		message = strlcat(message, " ", 1024)
	}

	for i := 1; i < Cmd_Argc(); i++ {
		message = strlcat(message, Cmd_Argv(i), 1024)
		message = strlcat(message, " ", 1024)
	}

	if cls.state >= ca_connected {
		to = cls.server_adr
	} else {
		if rcon_address.string == "" {
			Com_Printf("You must either be connected or set 'rcon_address' to issue rcon commands\n")
			return
		}
		NET_StringToAdr(nil, rcon_address.string, &to)
		if to.type_ == NA_IPV4 && to.ipv4.port == 0 {
			to.ipv4.port = uint16(BigShort(PORT_SERVER))
		} else if to.type_ == NA_IPV6 && to.ipv6.port == 0 {
			to.ipv6.port = uint16(BigShort(PORT_SERVER))
		}
	}

	NET_SendPacket(NS_CLIENT, append([]byte(message), 0), &to)
}

func CL_Download_f() {
	if cls.state == ca_disconnected || cls.netqw == nil {
		Com_Printf("Must be connected.\n")
		return
	}

	if Cmd_Argc() != 2 {
		Com_Printf("Usage: %s <datafile>\n", Cmd_Argv(0))
		return
	}

	cls.downloadname = strlcpy(Cmd_Argv(1), MAX_OSPATH)
	for p := 0; ; {
		q := strings.IndexByte(cls.downloadname[p:], '/')
		if q < 0 {
			break
		}
		Sys_IO_Create_Directory(cls.downloadname[:p+q])
		p += q + 1
	}
	cls.downloadtempname = cls.downloadname
	cls.downloadtype = dl_single

	buf := fmt.Sprintf("%cdownload %s", clc_stringcmd, Cmd_Argv(1))
	if len(buf) < 1500 {
		NetQW_AppendReliableBuffer(cls.netqw, append([]byte(buf), 0))
	}
}

func CL_User_f() {
	if Cmd_Argc() != 2 {
		Com_Printf("Usage: %s <username / userid>\n", Cmd_Argv(0))
		return
	}

	uid := atoi(Cmd_Argv(1))

	for i := 0; i < MAX_CLIENTS; i++ {
		if cl.players[i].name == "" {
			continue
		}
		if cl.players[i].userid == uid || cl.players[i].name == Cmd_Argv(1) {
			Info_Print(cl.players[i].userinfo)
			return
		}
	}
	Com_Printf("User not in server.\n")
}

func CL_Users_f() {
	c := 0
	Com_Printf("userid frags name\n")
	Com_Printf("------ ----- ----\n")
	for i := 0; i < MAX_CLIENTS; i++ {
		if cl.players[i].name != "" {
			Com_Printf("%6d %4d %s\n", cl.players[i].userid, cl.players[i].frags, cl.players[i].name)
			c++
		}
	}

	Com_Printf("%d total users\n", c)
}

func CL_Color_f() {
	var top, bottom int

	if Cmd_Argc() == 1 {
		Com_Printf("\"color\" is \"%s %s\"\n",
			Info_ValueForKey(cls.userinfo, "topcolor"),
			Info_ValueForKey(cls.userinfo, "bottomcolor"))
		Com_Printf("color <0-13> [0-13]\n")
		return
	}

	if Cmd_Argc() == 2 {
		top = atoi(Cmd_Argv(1))
		bottom = top
	} else {
		top = atoi(Cmd_Argv(1))
		bottom = atoi(Cmd_Argv(2))
	}

	top &= 15
	top = min(top, 13)
	bottom &= 15
	bottom = min(bottom, 13)

	Cvar_SetValue(&topcolor, float32(top))
	Cvar_SetValue(&bottomcolor, float32(bottom))

	cl.players[cl.playernum].real_topcolor = byte(top)
	cl.players[cl.playernum].real_bottomcolor = byte(bottom)

	TP_CalculateColoursForPlayer(uint(cl.playernum))
}

// usage: fullinfo \name\unnamed\topcolor\0\bottomcolor\1, etc
func CL_FullInfo_f() {
	if Cmd_Argc() != 2 {
		Com_Printf("fullinfo <complete info string>\n")
		return
	}

	s := Cmd_Argv(1)
	i := 0
	if at(s, i) == '\\' {
		i++
	}
	for at(s, i) != 0 {
		start := i
		for at(s, i) != 0 && s[i] != '\\' {
			i++
		}
		key := s[start:i]

		if at(s, i) == 0 {
			Com_Printf("MISSING VALUE\n")
			return
		}

		i++
		start = i
		for at(s, i) != 0 && s[i] != '\\' {
			i++
		}
		value := s[start:i]

		if at(s, i) != 0 {
			i++
		}

		if Q_strcasecmp(key, pmodel_name) == 0 || Q_strcasecmp(key, emodel_name) == 0 {
			continue
		}

		cls.userinfo = Info_SetValueForKey(cls.userinfo, key, value, MAX_INFO_STRING)
	}
}

// Allow clients to change userinfo
func CL_SetInfo_f() {
	if Cmd_Argc() == 1 {
		Info_Print(cls.userinfo)
		return
	}
	if Cmd_Argc() != 3 {
		Com_Printf("Usage: %s [ <key> <value> ]\n", Cmd_Argv(0))
		return
	}
	if Q_strcasecmp(Cmd_Argv(1), pmodel_name) == 0 || Cmd_Argv(1) == emodel_name {
		return
	}

	cls.userinfo = Info_SetValueForKey(cls.userinfo, Cmd_Argv(1), Cmd_Argv(2), MAX_INFO_STRING)
	if cls.state >= ca_connected {
		Cmd_ForwardToServer()
	}
}

func CL_UserInfo_f() {
	if Cmd_Argc() != 1 {
		Com_Printf("%s : no arguments expected\n", Cmd_Argv(0))
		return
	}
	Info_Print(cls.userinfo)
}

func CL_Quit_f() {
	if cl_confirmquit.value != 0 {
		M_Menu_Quit_f()
	} else {
		Host_Quit()
	}
}

func CL_Serverinfo_f() {
	if cls.state >= ca_onserver {
		Info_Print(cl.serverinfo)
	} else {
		Com_Printf("Can't \"%s\", not connected\n", Cmd_Argv(0))
	}
}

//============================================================================

func CL_WriteConfig(name string) {
	f, err := os.Create(va("%s/%s", cls.gamedir, name))
	if err != nil {
		Com_Printf("Couldn't write %s.\n", name)
		return
	}

	fmt.Fprintf(f, "// Generated by classicQ\n")
	fmt.Fprintf(f, "\n// Key bindings\n")
	Key_WriteBindings(f)
	fmt.Fprintf(f, "\n// Variables\n")
	Cvar_WriteVariables(f)
	fmt.Fprintf(f, "\n// Aliases\n")
	Cmd_WriteAliases(f)
	f.Close()
}

// Writes key bindings and archived cvars to config.cfg
func CL_WriteConfiguration() {
	if host_initialized && cfg_legacy_write.value != 0 {
		CL_WriteConfig("config.cfg")
	}
}

// Writes key bindings and archived cvars to a custom config file
func CL_WriteConfig_f() {
	if Cmd_Argc() != 2 {
		Com_Printf("Usage: %s <filename>\n", Cmd_Argv(0))
		return
	}

	name := strlcpy(Cmd_Argv(1), MAX_OSPATH)
	name = COM_ForceExtension(name, ".cfg")
	Com_Printf("Writing %s\n", name)
	CL_WriteConfig(name)
}

func Skin_Skins_f() {
	if cls.state == ca_onserver && cbuf_current != cbuf_main { // only download when connecting
		if cls.netqw != nil {
			buf := fmt.Sprintf("%cbegin %d", clc_stringcmd, cl.servercount)
			if len(buf) < 128 {
				NetQW_AppendReliableBuffer(cls.netqw, append([]byte(buf), 0))
			}
		}
	} else {
		Skin_FreeAll()
	}
}

func CL_InitCommands() {
	// general commands
	Cmd_AddCommand("cmd", CL_ForwardToServer_f)
	Cmd_AddCommand("download", CL_Download_f)
	Cmd_AddCommand("packet", CL_Packet_f)
	Cmd_AddCommand("pause", CL_Pause_f)
	Cmd_AddCommand("quit", CL_Quit_f)
	Cmd_AddCommand("rcon", CL_Rcon_f)
	Cmd_AddCommand("say", CL_Say_f)
	Cmd_AddCommand("say_team", CL_Say_f)
	Cmd_AddCommand("serverinfo", CL_Serverinfo_f)
	Cmd_AddCommand("skins", Skin_Skins_f)
	Cmd_AddCommand("user", CL_User_f)
	Cmd_AddCommand("users", CL_Users_f)
	Cmd_AddCommand("version", CL_Version_f)
	Cmd_AddCommand("writeconfig", CL_WriteConfig_f)

	// client info setting
	Cmd_AddCommand("color", CL_Color_f)
	Cmd_AddCommand("fullinfo", CL_FullInfo_f)
	Cmd_AddCommand("setinfo", CL_SetInfo_f)
	Cmd_AddCommand("userinfo", CL_UserInfo_f)

	// forward to server commands
	Cmd_AddCommand("kill", nil)
	Cmd_AddCommand("god", nil)
	Cmd_AddCommand("give", nil)
	Cmd_AddCommand("noclip", nil)
	Cmd_AddCommand("fly", nil)
}

/*
==============================================================================
SERVER COMMANDS

Server commands are commands stuffed by server into client's cbuf
We use a separate command buffer for them -- there are several
reasons for that:
1. So that partially stuffed commands are always executed properly
2. Not to let players cheat in TF (v_cshift etc don't work in console)
3. To hide some commands the user doesn't need to know about, like
changing, fullserverinfo, nextul, stopul
==============================================================================
*/

// Just sent as a hint to the client that they should drop to full console
func CL_Changing_f() {
	cl.intermission = 0

	if cls.download != nil { // don't change when downloading
		return
	}

	S_StopAllSounds(true)
	cls.state = ca_connected // not active anymore, but not disconnected

	Ignore_PreNewMap()

	if cls.netqw != nil {
		NetQW_SetDeltaPoint(cls.netqw, -1)
	}

	Com_Printf("\nChanging map...\n")
}

// Sent by server when serverinfo changes
func CL_FullServerinfo_f() {
	if Cmd_Argc() != 2 {
		return
	}

	cl.serverinfo = strlcpy(Cmd_Argv(1), MAX_SERVERINFO_STRING)

	if Info_ValueForKey(cl.serverinfo, "*cheats") != "" {
		Com_Printf("== Cheats are enabled ==\n")
	}

	CL_ProcessServerInfo()
}

func CL_Fov_f() {
	if Cmd_Argc() == 1 {
		Com_Printf("\"fov\" is \"%s\"\n", scr_fov.string)
		return
	}
	if Q_atof(Cmd_Argv(1)) == 90.0 && default_fov.value != 0 {
		Cvar_SetValue(&scr_fov, default_fov.value)
	} else {
		Cvar_Set(&scr_fov, Cmd_Argv(1))
	}
}

func CL_R_DrawViewModel_f() {
	if cl_filterdrawviewmodel.value != 0 {
		return
	}
	Cvar_Command()
}

func InterceptServerRateSet() {
	if Cmd_Argc() == 2 {
		CL_UserinfoChanged("rate", Cmd_Argv(1))
	}
}

type svcmd_t struct {
	name     string
	function func()
}

var svcmds = []svcmd_t{
	{"changing", CL_Changing_f},
	{"fullserverinfo", CL_FullServerinfo_f},
	{"nextul", CL_NextUpload},
	{"stopul", CL_StopUpload},
	{"fov", CL_Fov_f},
	{"r_drawviewmodel", CL_R_DrawViewModel_f},
	{"rate", InterceptServerRateSet},
}

// Called by Cmd_ExecuteString if cbuf_current == cbuf_svc
func CL_CheckServerCommand() bool {
	s := Cmd_Argv(0)
	for i := range svcmds {
		cmd := &svcmds[i]
		if s == cmd.name {
			if cmd.function != nil {
				cmd.function()
			}
			return true
		}
	}
	return false
}

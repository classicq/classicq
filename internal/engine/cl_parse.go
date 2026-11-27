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
	"io"
	"os"
	"strings"
)

var svc_strings = []string{
	"svc_bad",
	"svc_nop",
	"svc_disconnect",
	"svc_updatestat",
	"svc_version",      // [long] server version
	"svc_setview",      // [short] entity number
	"svc_sound",        // <see code>
	"svc_time",         // [float] server time
	"svc_print",        // [string] null terminated string
	"svc_stufftext",    // [string] stuffed into client's console buffer
	"svc_setangle",     // [vec3] set the view angle to this absolute value
	"svc_serverdata",   // [long] version ...
	"svc_lightstyle",   // [byte] [string]
	"svc_updatename",   // [byte] [string]
	"svc_updatefrags",  // [byte] [short]
	"svc_clientdata",   // <shortbits + data>
	"svc_stopsound",    // <see code>
	"svc_updatecolors", // [byte] [byte]
	"svc_particle",     // [vec3] <variable>
	"svc_damage",       // [byte] impact [byte] blood [vec3] from
	"svc_spawnstatic",
	"OBSOLETE svc_spawnbinary",
	"svc_spawnbaseline",
	"svc_temp_entity", // <variable>
	"svc_setpause",
	"svc_signonnum",
	"svc_centerprint",
	"svc_killedmonster",
	"svc_foundsecret",
	"svc_spawnstaticsound",
	"svc_intermission",
	"svc_finale",
	"svc_cdtrack",
	"svc_sellscreen",
	"svc_smallkick",
	"svc_bigkick",
	"svc_updateping",
	"svc_updateentertime",
	"svc_updatestatlong",
	"svc_muzzleflash",
	"svc_updateuserinfo",
	"svc_download",
	"svc_playerinfo",
	"svc_nails",
	"svc_choke",
	"svc_modellist",
	"svc_soundlist",
	"svc_packetentities",
	"svc_deltapacketentities",
	"svc_maxspeed",
	"svc_entgravity",
	"svc_setinfo",
	"svc_serverinfo",
	"svc_updatepl",
	"svc_nails2",
	"NEW PROTOCOL",
	"NEW PROTOCOL",
	"NEW PROTOCOL",
	"NEW PROTOCOL",
	"NEW PROTOCOL",
	"NEW PROTOCOL",
	"NEW PROTOCOL",
	"NEW PROTOCOL",
	"NEW PROTOCOL",
	"NEW PROTOCOL",
	"NEW PROTOCOL",
	"NEW PROTOCOL",
}

var oldparsecountmod int
var parsecountmod int
var parsecounttime float64

//=============================================================================

var packet_latency [NET_TIMINGS]int

func CL_CalcNet() int {
	for i := cls.netchan.outgoing_sequence - UPDATE_BACKUP + 1; i <= cls.netchan.outgoing_sequence; i++ {
		frame := &cl.frames[i&UPDATE_MASK]
		if frame.receivedtime == -1 {
			packet_latency[i&NET_TIMINGSMASK] = 9999 // dropped
		} else if frame.receivedtime == -2 {
			packet_latency[i&NET_TIMINGSMASK] = 10000 // choked
		} else if frame.receivedtime == -3 {
			packet_latency[i&NET_TIMINGSMASK] = -1 // choked by c2spps
		} else if frame.invalid {
			packet_latency[i&NET_TIMINGSMASK] = 9998 // invalid delta
		} else {
			packet_latency[i&NET_TIMINGSMASK] = int((frame.receivedtime - frame.senttime) * 20)
		}
	}

	lost, packetcount := 0, 0
	for a := 0; a < NET_TIMINGS; a++ {
		if cl_oldPL.value == 0 && a < UPDATE_BACKUP && (cls.realtime-
			cl.frames[(cls.netchan.outgoing_sequence-a)&UPDATE_MASK].senttime) < float64(cls.latency) {
			continue
		}
		i := (cls.netchan.outgoing_sequence - a) & NET_TIMINGSMASK
		if packet_latency[i] == 9999 {
			lost++
		}
		if packet_latency[i] != -1 { // don't count packets choked by c2spps
			packetcount++
		}
	}
	if packetcount != 0 {
		return lost * 100 / packetcount
	}
	return 100
}

//=============================================================================

// Returns true if the file exists, otherwise it attempts to start a download
// from the server.
func CL_CheckOrDownloadFile(filename string) bool {
	if strings.Contains(filename, "..") {
		Com_Printf("Refusing to download a path with ..\n")
		return true
	}

	if FS_FileExists(filename) != 0 {
		// it exists, no need to download
		return true
	}

	// ZOID - can't download when recording
	if cls.demorecording {
		Com_Printf("Unable to download %s in record mode.\n", cls.downloadname)
		return true
	}
	// ZOID - can't download when playback
	if cls.demoplayback {
		return true
	}

	cls.downloadname = filename
	Com_Printf("Downloading %s...\n", cls.downloadname)
	cls.downloadrate = 0
	cls.downloadstarttime = Sys_DoubleTime()

	// download to a temp name, and only rename to the real name when done,
	// so if interrupted a runt file wont be left
	cls.downloadtempname = COM_CopyAndStripExtension(cls.downloadname, MAX_OSPATH) + ".tmp"

	buf := fmt.Sprintf("%cdownload %s", clc_stringcmd, cls.downloadname)
	if len(buf) < 1500 {
		NetQW_AppendReliableBuffer(cls.netqw, append([]byte(buf), 0))
		cls.downloadnumber++
		return false
	}
	return true
}

func Model_NextDownload() {
	if cls.downloadnumber == 0 {
		Com_Printf("Checking models...\n")
		cls.downloadnumber = 1
	}

	cls.downloadtype = dl_model
	for ; cls.downloadnumber < MAX_MODELS && cl.model_name[cls.downloadnumber] != ""; cls.downloadnumber++ {
		if strings.Contains(cl.model_name[cls.downloadnumber], "*") {
			continue // inline brush model
		}
		if !CL_CheckOrDownloadFile(cl.model_name[cls.downloadnumber]) {
			return // started a download
		}
	}

	if !com_serveractive {
		mapnamestr := COM_CopyAndStripExtension(COM_SkipPath(cl.model_name[1]), MAX_QPATH)
		Cvar_ForceSet(&mapname, mapnamestr)
		R_PreMapLoad()
	}

	for i := 1; i < MAX_MODELS; i++ {
		if cl.model_name[i] == "" {
			break
		}

		cl.model_precache[i] = Mod_ForName(cl.model_name[i], false)

		if strings.HasPrefix(cl.model_name[i], "progs/v_") {
			switch cl.model_name[i][8:] {
			case "axe.mdl":
				cl.weapon_to_model_index[0] = i
			case "shot.mdl":
				cl.weapon_to_model_index[1] = i
			case "shot2.mdl":
				cl.weapon_to_model_index[2] = i
			case "nail.mdl":
				cl.weapon_to_model_index[3] = i
			case "nail2.mdl":
				cl.weapon_to_model_index[4] = i
			case "rock.mdl":
				cl.weapon_to_model_index[5] = i
			case "rock2.mdl":
				cl.weapon_to_model_index[6] = i
			case "light.mdl":
				cl.weapon_to_model_index[7] = i
			}
		}

		if cl.model_precache[i] == nil {
			Com_Printf("\nThe required model file '%s' could not be found or downloaded.\n\n", cl.model_name[i])
			Host_EndGame()
			return
		}
	}

	// all done
	cl.worldmodel = cl.model_precache[1]
	if cl.worldmodel == nil {
		Host_Error("Model_NextDownload: NULL worldmodel")
	}

	// Clear out leaf efrag stuff
	for i := 1; i < cl.worldmodel.numleafs+1; i++ {
		cl.worldmodel.leafs[i].efrags = nil
	}

	R_NewMap()
	TP_NewMap()
	MT_NewMap()
	Stats_NewMap()
	R_DrawFlat_NewMap()

	if cls.netqw != nil {
		// done with modellist, request first of static signon messages
		buf := fmt.Sprintf("%cprespawn %d 0 %d", clc_stringcmd, cl.servercount, int32(cl.worldmodel.checksum2))
		if len(buf) < 128 {
			NetQW_AppendReliableBuffer(cls.netqw, append([]byte(buf), 0))
		}
	}
}

func Sound_NextDownload() {
	if cls.downloadnumber == 0 {
		Com_Printf("Checking sounds...\n")
		cls.downloadnumber = 1
	}

	cls.downloadtype = dl_sound
	for ; cls.downloadnumber < MAX_SOUNDS && cl.sound_name[cls.downloadnumber] != ""; cls.downloadnumber++ {
		s := cl.sound_name[cls.downloadnumber]
		if !CL_CheckOrDownloadFile(va("sound/%s", s)) {
			return // started a download
		}
	}

	for i := 1; i < MAX_SOUNDS; i++ {
		if cl.sound_name[i] == "" {
			break
		}
		cl.sound_precache[i] = S_PrecacheSound(cl.sound_name[i])
	}

	// done with sounds, request models now
	cl.model_precache = [MAX_MODELS]*model_t{}
	for i := range cl_modelindices {
		cl_modelindices[i] = -1
	}

	if cls.netqw != nil {
		buf := fmt.Sprintf("%cmodellist %d %d", clc_stringcmd, cl.servercount, 0)
		if len(buf) < 128 {
			NetQW_AppendReliableBuffer(cls.netqw, append([]byte(buf), 0))
		}
	}
}

func CL_RequestNextDownload() {
	switch cls.downloadtype {
	case dl_single:
	case dl_model:
		Model_NextDownload()
	case dl_sound:
		Sound_NextDownload()
	default:
		Com_DPrintf("Unknown download type.\n")
	}
}

func cl_downloadPath(name string) string {
	if !strings.HasPrefix(name, "skins/") {
		return cls.gamedir + "/" + name
	}
	return "qw/" + name
}

func CL_OpenDownloadFile() bool {
	name := strlcpy(cl_downloadPath(cls.downloadtempname), 1024)
	FS_CreatePath(name)
	f, err := os.Create(name)
	if err == nil {
		cls.download = f
		return true
	}
	Com_ErrorPrintf("Failed to open output file \"%s\" for writing.\n", name)
	return false
}

// Close download: rename .tmp on completion, else delete.
func CL_CleanupActiveDownload(completed bool) {
	if cls.download == nil {
		return
	}
	cls.download.Close()
	cls.download = nil

	oldn := strlcpy(cl_downloadPath(cls.downloadtempname), MAX_OSPATH)

	if completed {
		if cls.downloadtempname != cls.downloadname {
			newn := strlcpy(cl_downloadPath(cls.downloadname), MAX_OSPATH)
			if Sys_rename(oldn, newn) != nil {
				Com_Printf("failed to rename.\n")
			}
		}
	} else {
		os.Remove(oldn)
	}
	cls.downloadpercent = 0
}

func CL_FinishDownloadFile() {
	CL_CleanupActiveDownload(true)
	CL_RequestNextDownload()
}

const (
	FTECHUNKSIZE = 1024
	FTEMAXBLOCKS = 1024 // must be a power of two
)

var chunked_download_number int // bumped per file header, never reset
var chunked_size int
var chunked_received_bytes int
var chunked_received [FTEMAXBLOCKS]int
var chunked_first_block int
var chunked_block_cycle int

func CL_NextChunkToRequest() int {
	total_blocks := (chunked_size + FTECHUNKSIZE - 1) / FTECHUNKSIZE
	for i := 0; i < FTEMAXBLOCKS; i++ {
		chunked_block_cycle++
		b := (chunked_block_cycle & (FTEMAXBLOCKS - 1)) + chunked_first_block
		if b >= total_blocks {
			continue
		}
		if chunked_received[b&(FTEMAXBLOCKS-1)] == 0 {
			return b
		}
	}
	return -1
}

func CL_SendChunkRequest(chunknum int) {
	if cls.netqw == nil {
		return
	}
	buf := fmt.Sprintf("%cnextdl %d %d %d", clc_stringcmd, chunknum, cls.downloadpercent, chunked_download_number)
	if len(buf) < 64 {
		if chunknum < 0 {
			NetQW_AppendReliableBuffer(cls.netqw, append([]byte(buf), 0))
		} else {
			NetQW_AppendUnreliableBuffer(cls.netqw, append([]byte(buf), 0))
		}
	}
}

var chunked_last_push_time float64

func CL_SendChunkedDownloadRequests() {
	const chunks_per_tick = 30 // matches mvdsv sv_downloadchunksperframe cap

	if cls.download == nil {
		return
	}
	if cls.ftexsupported&FTEX_CHUNKEDDOWNLOADS == 0 {
		return
	}

	if cls.netqw != nil {
		now := Sys_DoubleTime()
		interval := float64(NetQW_GetFrameTime(cls.netqw)) / 1000000.0
		if now-chunked_last_push_time < interval {
			return
		}
		chunked_last_push_time = now
	}

	for j := 0; j < chunks_per_tick; j++ {
		i := CL_NextChunkToRequest()
		if i < 0 {
			CL_SendChunkRequest(-1)
			cls.downloadpercent = 100
			CL_FinishDownloadFile()
			return
		}
		CL_SendChunkRequest(i)
	}
}

func CL_ChunkedDownloadHeader(filesize int, filename string) {
	if cls.download != nil {
		if filesize != -3 {
			Com_Printf("cls.download shouldn't have been set\n")
		}
		cls.download.Close()
		cls.download = nil
		cls.downloadpercent = 0
	}

	if cls.demoplayback {
		return
	}

	if filesize < 0 {
		switch filesize {
		case -3:
			Com_DPrintf("Server cancelled download of \"%s\"\n", filename)
		case -2:
			Com_Printf("Server refused upload of \"%s\"\n", filename)
		default:
			Com_Printf("\"%s\" not found on server\n", filename)
		}
		CL_RequestNextDownload()
		return
	}

	if !CL_OpenDownloadFile() {
		CL_RequestNextDownload()
		return
	}

	cls.downloadpercent = 0
	chunked_download_number++
	chunked_size = filesize
	chunked_received_bytes = 0
	chunked_first_block = 0
	chunked_block_cycle = -1 // first request asks for block 0
	chunked_received = [FTEMAXBLOCKS]int{}
}

func CL_ChunkedDownloadData(chunknum int) {
	var data [FTECHUNKSIZE]byte

	MSG_ReadData(data[:])

	if cls.download == nil || cls.demoplayback {
		return
	}
	if chunknum < chunked_first_block {
		return
	}
	if chunknum-chunked_first_block >= FTEMAXBLOCKS {
		return
	}
	if int64(chunknum)*FTECHUNKSIZE >= int64(chunked_size) {
		return
	}
	if chunked_received[chunknum&(FTEMAXBLOCKS-1)] != 0 {
		return
	}

	chunked_received_bytes += FTECHUNKSIZE
	chunked_received[chunknum&(FTEMAXBLOCKS-1)] = 1
	for chunked_received[chunked_first_block&(FTEMAXBLOCKS-1)] != 0 {
		chunked_received[chunked_first_block&(FTEMAXBLOCKS-1)] = 0
		chunked_first_block++
	}

	cls.download.Seek(int64(chunknum)*FTECHUNKSIZE, io.SeekStart)
	if chunked_size-chunknum*FTECHUNKSIZE < FTECHUNKSIZE {
		cls.download.Write(data[:chunked_size-chunknum*FTECHUNKSIZE])
	} else {
		cls.download.Write(data[:])
	}

	if chunked_size > 0 {
		// 64-bit math to avoid int32 wrap on files >21 MB
		cls.downloadpercent = int((int64(chunked_received_bytes) * 100) / int64(chunked_size))
	}
	elapsed := Sys_DoubleTime() - cls.downloadstarttime
	if elapsed > 0 {
		cls.downloadrate = int(float64(chunked_received_bytes) / 1024.0 / elapsed)
	}
}

func CL_ParseFTEChunkedDownload() {
	chunknum := MSG_ReadLong()
	if chunknum < 0 {
		filesize := MSG_ReadLong()
		filename := MSG_ReadString()
		CL_ChunkedDownloadHeader(filesize, filename)
		return
	}
	CL_ChunkedDownloadData(chunknum)
}

// OOB chunks (mvdsv burst format)
func CL_ParseChunkedDownloadOOB() {
	if msg_readcount+4 > cl_net_message.cursize {
		return
	}
	dlnum := MSG_ReadLong()
	if dlnum != chunked_download_number {
		Com_DPrintf("Dropping stale OOB chunk (dlnum %d vs %d)\n", dlnum, chunked_download_number)
		return
	}
	if msg_readcount+1 > cl_net_message.cursize {
		return
	}
	if MSG_ReadByte() != svc_download {
		return
	}
	CL_ParseFTEChunkedDownload()
}

func CL_ParseQWDownload() {
	// read the data
	size := MSG_ReadShort()
	percent := MSG_ReadByte()

	if cls.demoplayback {
		if size > 0 {
			msg_readcount += size
		}
		return // not in demo playback
	}

	if size == -1 {
		Com_Printf("File not found.\n")
		if cls.download != nil {
			Com_Printf("cls.download shouldn't have been set\n")
			cls.download.Close()
			cls.download = nil
		}
		CL_RequestNextDownload()
		return
	}

	// open the file if not opened yet
	if cls.download == nil {
		if !CL_OpenDownloadFile() {
			msg_readcount += size
			Com_Printf("Failed to open %s\n", cls.downloadtempname)
			CL_RequestNextDownload()
			return
		}
	}

	cls.download.Write(cl_net_message.data[msg_readcount : msg_readcount+size])
	msg_readcount += size

	elapsed := Sys_DoubleTime() - cls.downloadstarttime
	bytes, _ := cls.download.Seek(0, io.SeekCurrent)
	if elapsed > 0 && bytes > 0 {
		cls.downloadrate = int(float64(bytes) / 1024.0 / elapsed)
	}

	if percent != 100 {
		// request next block
		cls.downloadpercent = percent
		buf := fmt.Sprintf("%cnextdl", clc_stringcmd)
		if len(buf) < 128 {
			NetQW_AppendReliableBuffer(cls.netqw, append([]byte(buf), 0))
		}
	} else {
		CL_FinishDownloadFile()
	}
}

func CL_ParseDownload() {
	if cls.ftexsupported&FTEX_CHUNKEDDOWNLOADS != 0 {
		CL_ParseFTEChunkedDownload()
	} else {
		CL_ParseQWDownload()
	}
}

var upload_data []byte
var upload_pos int
var upload_size int

func CL_NextUpload() {
	if upload_data == nil {
		return
	}

	r := min(upload_size-upload_pos, 768)
	buffer := upload_data[upload_pos : upload_pos+r]

	MSG_WriteByte(&cls.netchan.message, clc_upload)
	MSG_WriteShort(&cls.netchan.message, r)

	upload_pos += r
	size := upload_size
	if size == 0 {
		size = 1
	}
	percent := upload_pos * 100 / size
	MSG_WriteByte(&cls.netchan.message, percent)
	SZ_Write(&cls.netchan.message, buffer)

	Com_DPrintf("UPLOAD: %6d: %d written\n", upload_pos-r, r)

	if upload_pos != upload_size {
		return
	}

	Com_Printf("Upload completed\n")

	upload_data = nil
	upload_pos = 0
	upload_size = 0
}

func CL_IsUploading() bool {
	return upload_data != nil
}

func CL_StopUpload() {
	upload_data = nil
}

func CL_StartUpload(filename string) {
	if cls.state < ca_onserver {
		return // gotta be connected
	}

	if CL_IsUploading() {
		CL_StopUpload()
	}

	f, size := FS_FileOpenRead(va("%s/%s", com_basedir, filename))
	if size == -1 {
		return
	}

	Com_DPrintf("Upload starting of %d...\n", size)

	upload_data = make([]byte, size)
	if n, _ := io.ReadFull(f, upload_data); n != size {
		f.Close()
		CL_StopUpload()
		return
	}
	f.Close()

	upload_size = size
	upload_pos = 0

	CL_NextUpload()
}

/*
=====================================================================
  SERVER CONNECTING MESSAGES
=====================================================================
*/

func CL_ParseServerData() {
	cflag := false

	Com_DPrintf("Serverdata packet received.\n")

	// wipe the clientState_t struct
	CL_ClearState()

	cshift_empty = cshift_t{} // Tonik

	// parse protocol version number
	// allow 2.2 and 2.29 demos to play
	var protover int
	for {
		protover = MSG_ReadLong()
		if protover == QW_PROTOEXT_FTEX {
			cls.ftexsupported = uint32(MSG_ReadLong()) & FTEX_SUPPORTED
		} else if protover == QW_PROTOEXT_FTE2 {
			cls.fte2supported = uint32(MSG_ReadLong()) & FTE2_SUPPORTED
		} else if protover == QW_PROTOEXT_MVD1 {
			cls.mvdprotocolextensions1 = uint32(MSG_ReadLong()) & MVD1_SUPPORTED
		} else {
			break
		}
	}

	if cls.ftexsupported&FTEX_TRANS != 0 {
		Com_Printf("Protocol: modern (FTE_PEXT2)\n")
	} else if cls.ftexsupported != 0 {
		Com_Printf("Protocol: extended QW\n")
	} else {
		Com_Printf("Protocol: vanilla 2.40\n")
	}

	if protover != PROTOCOL_VERSION &&
		!(cls.demoplayback && (protover == 26 || protover == 27 || protover == 28)) {
		Host_Error("Server returned version %d (0x%08x), not %d\nYou probably need to upgrade.\nCheck https://classicq.github.io/", protover, uint32(protover), PROTOCOL_VERSION)
	}

	cl.protoversion = protover

	cl.servercount = MSG_ReadLong()

	// game directory
	str := MSG_ReadString()

	cl.teamfortress = Q_strcasecmp(str, "fortress") == 0
	if cl.teamfortress {
		cbuf_current = cbuf_svc // hack
		Cvar_SetValue(&v_iyaw_cycle, 2)
		Cvar_SetValue(&v_iroll_cycle, 0.5)
		Cvar_SetValue(&v_ipitch_cycle, 1)
		Cvar_SetValue(&v_iyaw_level, 0.3)
		Cvar_SetValue(&v_iroll_level, 0.1)
		Cvar_SetValue(&v_ipitch_level, 0.3)
		Cvar_SetValue(&v_idlescale, 0)
	}

	if Q_strcasecmp(cls.gamedirfile, str) != 0 {
		// save current config
		CL_WriteConfiguration()
		cls.gamedirfile = strlcpy(str, MAX_QPATH)
		cls.gamedir = strlcpy(com_basedir+"/"+cls.gamedirfile, MAX_OSPATH)
		cflag = true
	}

	if !com_serveractive {
		FS_SetGamedir(str)
	}

	// run config.cfg and frontend.cfg in the gamedir if they exist
	if cfg_legacy_exec.value != 0 && (cflag || cfg_legacy_exec.value >= 2) {
		fn := strlcpy(cls.gamedir+"/config.cfg", MAX_OSPATH)
		Cbuf_AddText("cl_warncmd 0\n")
		if f, err := os.Open(fn); err == nil {
			f.Close()
			if cls.gamedirfile == com_gamedirfile {
				Cbuf_AddText("exec config.cfg\n")
			} else {
				Cbuf_AddText(va("exec ../%s/config.cfg\n", cls.gamedirfile))
			}
		} else if cfg_legacy_exec.value == 3 && cls.gamedir != "qw" {
			if f, err := os.Open("qw/config.cfg"); err == nil {
				f.Close()
				Cbuf_AddText("exec config.cfg\n")
			}
		}
		fn = strlcpy(cls.gamedir+"/frontend.cfg", MAX_OSPATH)
		if f, err := os.Open(fn); err == nil {
			f.Close()
			if cls.gamedirfile == com_gamedirfile {
				Cbuf_AddText("exec frontend.cfg\n")
			} else {
				Cbuf_AddText(va("exec ../%s/frontend.cfg\n", cls.gamedirfile))
			}
		} else if cfg_legacy_exec.value == 3 && cls.gamedir != "qw" {
			if f, err := os.Open("qw/frontend.cfg"); err == nil {
				f.Close()
				Cbuf_AddText("exec frontend.cfg\n")
			}
		}
		Cbuf_AddText("cl_warncmd 1\n")
	}

	// parse player slot, high bit means spectator
	if cls.mvdplayback {
		olddemotime = MSG_ReadFloat()
		nextdemotime = olddemotime
		cls.netchan.last_received = olddemotime
		cl.playernum = MAX_CLIENTS - 1
		cl.spectator = 1
		for i := 0; i < UPDATE_BACKUP; i++ {
			cl.frames[i].playerstate[cl.playernum].pm_type = int(PM_SPECTATOR)
		}
	} else {
		cl.playernum = MSG_ReadByte()
		if cl.playernum&128 != 0 {
			cl.spectator = 1
			cl.playernum &^= 128
		}
	}

	// get the full level name
	str = MSG_ReadString()
	cl.levelname = strlcpy(str, 40)

	// get the movevars
	movevars.gravity = MSG_ReadFloat()
	movevars.stopspeed = MSG_ReadFloat()
	cl.maxspeed = MSG_ReadFloat()
	movevars.spectatormaxspeed = MSG_ReadFloat()
	movevars.accelerate = MSG_ReadFloat()
	movevars.airaccelerate = MSG_ReadFloat()
	movevars.wateraccelerate = MSG_ReadFloat()
	movevars.friction = MSG_ReadFloat()
	movevars.waterfriction = MSG_ReadFloat()
	cl.entgravity = MSG_ReadFloat()

	// separate the printfs so the server message can have a color
	Com_Printf("%s", "\n\n\x1d"+strings.Repeat("\x1e", 36)+"\x1f\n\n")
	Com_Printf("%c%s\n", 2, str)

	if cls.netqw != nil {
		// ask for the sound list next
		cl.sound_name = [MAX_SOUNDS]string{}
		buf := fmt.Sprintf("%csoundlist %d %d", clc_stringcmd, cl.servercount, 0)
		if len(buf) < 128 {
			NetQW_AppendReliableBuffer(cls.netqw, append([]byte(buf), 0))
		}
	}

	// now waiting for downloads, etc
	cls.state = ca_onserver
}

func CL_ParseSoundlist() {
	// precache sounds
	numsounds := MSG_ReadByte()

	for {
		str := MSG_ReadString()
		if str == "" {
			break
		}
		numsounds++
		if numsounds == MAX_SOUNDS {
			Host_Error("Server sent too many sound_precache")
		}
		cl.sound_name[numsounds] = strlcpy(str, MAX_QPATH)
	}

	n := MSG_ReadByte()

	if n != 0 {
		if cls.netqw != nil {
			buf := fmt.Sprintf("%csoundlist %d %d", clc_stringcmd, cl.servercount, n)
			if len(buf) < 128 {
				NetQW_AppendReliableBuffer(cls.netqw, append([]byte(buf), 0))
			}
		}
		return
	}

	cls.downloadnumber = 0
	cls.downloadtype = dl_sound
	Sound_NextDownload()
}

func CL_ParseModellist(extended int) {
	// precache models and note certain default indexes
	var nummodels int
	if extended != 0 {
		nummodels = int(uint16(MSG_ReadShort()))
	} else {
		nummodels = MSG_ReadByte()
	}

	for {
		str := MSG_ReadString()
		if str == "" {
			break
		}
		nummodels++
		if nummodels >= MAX_MODELS {
			Host_Error("Server sent too many model_precache")
		}
		if str[0] == '*' {
			if nummodels == 1 {
				Host_Error("Server sent submodel too soon")
			}
			cl.model_name[nummodels] = strlcpy(cl.model_name[1]+str, MAX_QPATH)
		} else {
			cl.model_name[nummodels] = strlcpy(str, MAX_QPATH)
			for i := 0; i < int(cl_num_modelindices); i++ {
				if cl_modelnames[i] == cl.model_name[nummodels] {
					cl_modelindices[i] = nummodels
					break
				}
			}
		}
	}

	if nummodels+1 < MAX_MODELS {
		cl.model_name[nummodels+1] = ""
	}

	if n := MSG_ReadByte(); n != 0 {
		// preserve high byte for MODELDBL resume past 255
		next_index := (nummodels & 0xff00) + n
		if cls.netqw != nil {
			buf := fmt.Sprintf("%cmodellist %d %d", clc_stringcmd, cl.servercount, next_index)
			if len(buf) < 128 {
				NetQW_AppendReliableBuffer(cls.netqw, append([]byte(buf), 0))
			}
		}
		return
	}

	cls.downloadnumber = 0
	cls.downloadtype = dl_model
	Model_NextDownload()
}

func CL_ParseBaseline(es *entity_state_t) {
	es.modelindex = MSG_ReadByte()
	es.frame = MSG_ReadByte()
	es.colormap = MSG_ReadByte()
	es.skinnum = MSG_ReadByte()
	for i := 0; i < 3; i++ {
		es.origin[i] = MSG_ReadCoord()
		es.angles[i] = MSG_ReadAngle()
	}
}

func CL_FreeStatics() {
	cl.first_static = nil
	cl.last_static = nil
}

func cl_linkStatic(sent *static_entity) {
	if cl.first_static == nil {
		cl.first_static = sent
		cl.last_static = sent
	} else {
		cl.last_static.next = sent
		cl.last_static = sent
	}
}

// Static entities are non-interactive world objects like torches
func CL_ParseStatic() bool {
	var es entity_state_t

	sent := &static_entity{}

	CL_ParseBaseline(&es)

	ent := &sent.ent

	// copy it to the current state
	ent.model = cl.model_precache[es.modelindex]
	ent.frame = es.frame
	ent.skinnum = es.skinnum

	ent.origin = es.origin
	ent.angles = es.angles

	R_AddEfrags(ent)

	cl_linkStatic(sent)

	return true
}

func CL_ParseStaticSound() {
	var org vec3_t

	for i := 0; i < 3; i++ {
		org[i] = MSG_ReadCoord()
	}
	sound_num := MSG_ReadByte()
	vol := MSG_ReadByte()
	atten := MSG_ReadByte()

	if cl_staticsounds.value != 0 {
		S_StaticSound(cl.sound_precache[sound_num], org, float32(vol), float32(atten))
	}
}

/*
=====================================================================
ACTION MESSAGES
=====================================================================
*/

func CL_ParseStartSoundPacket() {
	var pos vec3_t

	channel := MSG_ReadShort()

	volume := DEFAULT_SOUND_PACKET_VOLUME
	if channel&SND_VOLUME != 0 {
		volume = MSG_ReadByte()
	}

	var attenuation float32 = DEFAULT_SOUND_PACKET_ATTENUATION
	if channel&SND_ATTENUATION != 0 {
		attenuation = float32(float64(MSG_ReadByte()) / 64.0)
	}

	sound_num := MSG_ReadByte()

	for i := 0; i < 3; i++ {
		pos[i] = MSG_ReadCoord()
	}

	ent := (channel >> 3) & 1023
	channel &= 7

	if ent > CL_MAX_EDICTS {
		Host_Error("CL_ParseStartSoundPacket: ent = %d", ent)
	}

	if cls.mvdplayback {
		tracknum := Cam_TrackNum()
		if cl.spectator != 0 && tracknum != -1 && ent == tracknum+1 {
			ent = cl.playernum + 1
		}
	}

	S_StartSound(ent, channel, cl.sound_precache[sound_num], pos, float32(float64(volume)/255.0), attenuation)

	if ent == cl.playernum+1 {
		TP_CheckPickupSound(cl.sound_name[sound_num], pos)
	}
}

// Server information pertaining to this client only, sent every frame
func CL_ParseClientdata() {
	// calculate simulated time of message
	oldparsecountmod = parsecountmod

	newparsecount := cls.netchan.incoming_acknowledged
	if cls.mvdplayback {
		cl.oldparsecount = newparsecount - 1
	} else {
		cl.oldparsecount = cl.parsecount
	}
	cl.parsecount = newparsecount
	parsecountmod = cl.parsecount & UPDATE_MASK
	frame := &cl.frames[parsecountmod]

	if cls.mvdplayback {
		frame.senttime = cls.realtime - cls.frametime
	}

	parsecounttime = cl.frames[parsecountmod].senttime

	frame.receivedtime = cls.realtime

	// calculate latency
	latency := float32(frame.receivedtime - frame.senttime)

	if latency >= 0 && latency <= 1 {
		// drift the average latency towards the observed latency
		if latency < cls.latency {
			cls.latency = latency
		} else {
			cls.latency = float32(float64(cls.latency) + 0.001) // drift up, so correction is needed
		}
	}
}

// allkeys stands for the NULL key of C: everything changed
func CL_ProcessUserInfo(slot int, player *player_info_t, key string, allkeys bool) {
	player.name = strlcpy(Info_ValueForKey(player.userinfo, "name"), MAX_SCOREBOARDNAME)

	if player.name == "" && player.userid != 0 && len(player.userinfo) >= MAX_INFO_STRING-17 {
		// somebody's trying to hide himself by overloading userinfo
		player.name = " "
	}

	player.real_topcolor = byte(atoi(Info_ValueForKey(player.userinfo, "topcolor")))
	player.real_bottomcolor = byte(atoi(Info_ValueForKey(player.userinfo, "bottomcolor")))
	player.team = Info_ValueForKey(player.userinfo, "team")

	if Info_ValueForKey(player.userinfo, "*spectator") != "" {
		player.spectator = 1
	} else {
		player.spectator = 0
	}

	if slot == cl.playernum && player.name != "" {
		cl.spectator = int(player.spectator)
		if cl.spectator == 0 && cls.netqw != nil {
			NetQW_UnlockMovement(cls.netqw)
		}
	}

	Sbar_Changed()

	colourschanged := allkeys || key == "topcolor" || key == "bottomcolor"
	teamchanged := allkeys || key == "team"

	update_skin := allkeys ||
		(player.spectator == 0 && (key == "skin" || colourschanged || teamchanged))

	if (cl.spectator != 0 && Cam_TrackNum() == slot) || cl.playernum == -1 || slot == cl.playernum {
		if teamchanged {
			TP_RecalculateSkins()
			TP_RecalculateColours()
		}
	} else {
		if update_skin {
			TP_CalculateSkinForPlayer(uint(slot))
		}
		if colourschanged || teamchanged {
			TP_CalculateColoursForPlayer(uint(slot))
		}
	}
}

func cl_playerSlot(player *player_info_t) int {
	for i := range cl.players {
		if &cl.players[i] == player {
			return i
		}
	}
	return -1
}

func CL_PlayerEnterSlot(player *player_info_t) {
	player.ignored = false
	slot := cl_playerSlot(player)
	oldplayerstates[slot] = player_state_t{}
	Stats_EnterSlot(slot)
}

func CL_PlayerLeaveSlot(player *player_info_t) {
}

func CL_UpdateUserinfo() {
	slot := MSG_ReadByte()
	if slot >= MAX_CLIENTS {
		Host_Error("CL_ParseServerMessage: svc_updateuserinfo > MAX_CLIENTS")
	}

	player := &cl.players[slot]

	was_empty_slot := player.name == ""

	player.userid = MSG_ReadLong()
	player.userinfo = strlcpy(MSG_ReadString(), MAX_INFO_STRING)

	CL_ProcessUserInfo(slot, player, "", true)

	if player.name != "" && was_empty_slot {
		CL_PlayerEnterSlot(player)
	} else if player.name == "" && !was_empty_slot {
		CL_PlayerLeaveSlot(player)
	}
}

func CL_SetInfo() {
	slot := MSG_ReadByte()
	if slot >= MAX_CLIENTS {
		Host_Error("CL_ParseServerMessage: svc_setinfo > MAX_CLIENTS")
	}

	player := &cl.players[slot]

	key := strlcpy(MSG_ReadString(), MAX_INFO_STRING)
	value := strlcpy(MSG_ReadString(), MAX_INFO_STRING)

	if !cl.teamfortress { // don't allow cheating in TF
		Com_DPrintf("SETINFO %s: %s=%s\n", player.name, key, value)
	}

	player.userinfo = Info_SetValueForStarKey(player.userinfo, key, value, MAX_INFO_STRING)

	CL_ProcessUserInfo(slot, player, key, false)
}

// Called by CL_FullServerinfo_f and CL_ParseServerInfoChange
func CL_ProcessServerInfo() {
	var fpd int

	// game type (sbar code checks it) (GAME_DEATHMATCH default)
	if p := Info_ValueForKey(cl.serverinfo, "deathmatch"); p != "" {
		if atoi(p) != 0 {
			cl.gametype = GAME_DEATHMATCH
		} else {
			cl.gametype = GAME_COOP
		}
	} else {
		cl.gametype = GAME_DEATHMATCH
	}

	// server side fps restriction
	cl.maxfps = Q_atof(Info_ValueForKey(cl.serverinfo, "maxfps"))
	maxfps := cl.maxfps
	if maxfps == 0 {
		maxfps = 72
	}
	if net_maxfps.value != 0 && net_maxfps.value < maxfps {
		maxfps = net_maxfps.value
	}
	if maxfps < 30 {
		maxfps = 30
	}
	if cls.netqw != nil {
		NetQW_SetFPS(cls.netqw, uint(maxfps))
	}

	if cls.demoplayback {
		cl.watervis = true
		cl.allow_lumas = true
		cl.fbskins = 1
		cl.truelightning = 1
		fpd = 0
	} else if cl.spectator != 0 {
		cl.watervis = atoi(Info_ValueForKey(cl.serverinfo, "watervis")) != 0
		cl.allow_lumas = true
		cl.fbskins = 1
		cl.truelightning = 1
		fpd = atoi(Info_ValueForKey(cl.serverinfo, "fpd"))
	} else {
		cl.watervis = atoi(Info_ValueForKey(cl.serverinfo, "watervis")) != 0
		cl.allow_lumas = Info_ValueForKey(cl.serverinfo, "24bit_fbs") == "1"
		if fbskins := Info_ValueForKey(cl.serverinfo, "fbskins"); fbskins != "" {
			cl.fbskins = bound(float32(0), Q_atof(fbskins), float32(1))
		} else if cl.teamfortress {
			cl.fbskins = 0
		} else {
			cl.fbskins = 1
		}
		truelightning := Info_ValueForKey(cl.serverinfo, "fakeshaft")
		if truelightning == "" {
			truelightning = Info_ValueForKey(cl.serverinfo, "truelightning")
		}
		if truelightning != "" {
			cl.truelightning = bound(float32(0), Q_atof(truelightning), float32(1))
		} else {
			cl.truelightning = 1
		}
		fpd = atoi(Info_ValueForKey(cl.serverinfo, "fpd"))
	}

	p := Info_ValueForKey(cl.serverinfo, "status")
	standby := Q_strcasecmp(p, "standby") == 0
	countdown := Q_strcasecmp(p, "countdown") == 0

	if (cl.standby || cl.countdown) && !(standby || countdown) {
		cl.gametime = 0
		cls.demotimeoffset = cls.realactualdemotime
	}

	cl.standby = standby
	cl.countdown = countdown

	if minlight := Info_ValueForKey(cl.serverinfo, "minlight"); minlight != "" {
		cl.minlight = bound(0, Q_atoi(minlight), 255)
	} else {
		cl.minlight = 4
	}

	// ZQuake extension bits
	cl.z_ext = atoi(Info_ValueForKey(cl.serverinfo, "*z_ext"))

	// movement vars for prediction
	cl.bunnyspeedcap = Q_atof(Info_ValueForKey(cl.serverinfo, "pm_bunnyspeedcap"))
	movevars.slidefix = Q_atoi(Info_ValueForKey(cl.serverinfo, "pm_slidefix"))
	movevars.airstep = Q_atoi(Info_ValueForKey(cl.serverinfo, "pm_airstep"))
	if p := Info_ValueForKey(cl.serverinfo, "pm_ktjump"); p != "" {
		movevars.ktjump = Q_atof(p)
	} else if cl.teamfortress {
		movevars.ktjump = 0
	} else {
		movevars.ktjump = 0.5
	}

	// deathmatch and teamplay
	cl.deathmatch = atoi(Info_ValueForKey(cl.serverinfo, "deathmatch"))
	teamplay := atoi(Info_ValueForKey(cl.serverinfo, "teamplay"))

	// update skins if needed
	skin_refresh := (teamplay == 0) != (cl.teamplay == 0) || ((fpd^cl.fpd)&(FPD_NO_FORCE_COLOR|FPD_NO_FORCE_SKIN)) != 0
	cl.teamplay = teamplay
	cl.fpd = fpd
	if skin_refresh {
		TP_RecalculateColours()
		TP_RecalculateSkins()
	}
}

func CL_ParseServerInfoChange() {
	key := strlcpy(MSG_ReadString(), MAX_INFO_STRING)
	value := strlcpy(MSG_ReadString(), MAX_INFO_STRING)

	Com_DPrintf("SERVERINFO: %s=%s\n", key, value)

	cl.serverinfo = Info_SetValueForKey(cl.serverinfo, key, value, MAX_SERVERINFO_STRING)

	CL_ProcessServerInfo()
}

// for CL_ParsePrint
func FlushString(s string, level int, team bool, offset int) {
	if level == PRINT_CHAT {
		Com_Printf("%s", TP_ParseWhiteText(s, team, offset))
	} else {
		Com_Printf("%s", s)
	}

	if level >= 4 {
		return
	}

	if team {
		level = 4
	}

	TP_SearchForMsgTriggers(s[offset:], level)
	Stats_ParsePrint(s, level)
}

// Returns the text from the innermost proxy name on, or false when there is
// no QTV prefix.
func SkipQTVLeadingProxies(s string) (string, bool) {
	last := ""
	found := false
	i := 0
	for at(s, i) != 0 {
		if s[i] == '#' && at(s, i+1) >= '0' && at(s, i+1) <= '9' {
			i++
			for at(s, i) >= '0' && at(s, i) <= '9' {
				i++
			}
			if at(s, i) == ':' {
				i++
				name := i
				if at(s, i) != ' ' {
					for at(s, i) != 0 && s[i] != ':' {
						i++
					}
					if at(s, i) == ':' {
						last = s[name:]
						found = true
						if qtv_skipchained.value == 0 {
							break
						}
						i++
						if at(s, i) == ' ' {
							i++
						}
						continue
					}
					break
				}
				break
			}
			break
		}
		break
	}
	return last, found
}

func CL_ParsePrint() {
	flags := 0
	offset := 0

	level := MSG_ReadByte()
	s := MSG_ReadString()

	if qtvtmp, ok := SkipQTVLeadingProxies(s); ok {
		name := ""
		column := qtvtmp
		if col := strings.IndexByte(qtvtmp, ':'); col >= 0 {
			column = qtvtmp[col:]
			name = strlcpy(qtvtmp, bound(1, col+1, 1024))
		}

		var qtvstr string
		if strings.HasPrefix(s, "#0:qtv_say_game:") {
			qtvstr = fmt.Sprintf("%s%s%s\n", TP_ParseFunChars(qtv_gamechatprefix.string, false), name, column)
		} else if strings.HasPrefix(s, "#0:qtv_say_team_game:") {
			qtvstr = fmt.Sprintf("%s(%s)%s\n", TP_ParseFunChars(qtv_gamechatprefix.string, false), name, column)
		} else {
			qtvstr = fmt.Sprintf("%s%s%s\n", TP_ParseFunChars(qtv_chatprefix.string, false), name, column)
		}
		s = strlcpy(qtvstr, 2048)
	}

	Lua_MessageFunctions(level, s)

	if level == PRINT_CHAT {
		if TP_SuppressMessage(&s) {
			return
		}

		flags = TP_CategorizeMessage(s, &offset)

		FChecks_CheckRequest(s)

		if Ignore_Message(s, flags, offset) {
			return
		}

		if flags == 2 {
			tail, ok := TP_FilterMessage(s[offset:])
			s = s[:offset] + tail
			if !ok {
				return
			}
		}

		check_flood := Ignore_Check_Flood(s, flags, offset)
		if check_flood == IGNORE_NO_ADD {
			return
		} else if check_flood == NO_IGNORE_ADD {
			Ignore_Flood_Add(s)
		}

		suppress_talksound := false
		if flags == 2 {
			var tail string
			tail, suppress_talksound = TP_CheckSoundTrigger(s[offset:])
			s = s[:offset] + tail
		}

		if cl_chatsound.value == 0 || // no sound at all
			(cl_chatsound.value == 2 && flags != 2) { // only play sound in mm2
			suppress_talksound = true
		}

		if !suppress_talksound {
			S_LocalSound("misc/talk.wav")
		}

		if cl_nofake.value == 1 || (cl_nofake.value == 2 && flags != 2) {
			b := []byte(s)
			for p := range b {
				if b[p] == 0x0D || (b[p] == 0x0A && p+1 < len(b)) {
					b[p] = ' '
				}
			}
			s = string(b)
		}
	}

	if cl.sprint_buf != "" && (level != cl.sprint_level || at(s, 0) == 1 || at(s, 0) == 2) {
		FlushString(cl.sprint_buf, cl.sprint_level, false, 0)
		cl.sprint_buf = ""
	}

	if at(s, 0) == 1 || at(s, 0) == 2 {
		FlushString(s, level, flags == 2, offset)
		return
	}

	cl.sprint_buf = strlcat(cl.sprint_buf, s, 2048)
	cl.sprint_level = level

	if p := strings.LastIndexByte(cl.sprint_buf, '\n'); p >= 0 {
		str := cl.sprint_buf[:p+1]
		cl.sprint_buf = cl.sprint_buf[p+1:]
		FlushString(str, level, flags == 2, offset)
	}
}

func CL_ParseStufftext() {
	s := MSG_ReadString()

	Com_DPrintf("stufftext: %s\n", s)

	if strings.HasPrefix(s, "alias _cs") {
		Cbuf_AddTextEx(cbuf_svc, "alias _cs \"wait;+attack;wait;wait;-attack;wait\"\n")
	} else if strings.HasPrefix(s, "alias _y") {
		Cbuf_AddTextEx(cbuf_svc, "alias _y \"wait;-attack;wait;wait;+attack;wait;wait;-attack;wait\"\n")
	} else if s == "cmd snap" || (strings.HasPrefix(s, "r_skyname ") && !strings.Contains(s, "\n")) {
		Cbuf_AddTextEx(cbuf_svc, va("%s\n", s))
	} else {
		Cbuf_AddTextEx(cbuf_svc, s)
	}

	// Execute stuffed commands immediately when starting a demo
	if cls.demoplayback && cls.state != ca_active {
		Cbuf_ExecuteEx(cbuf_svc) // FIXME: execute cbuf_main too?
	}
}

func CL_SetStat(stat int, value int) {
	if stat < 0 || stat >= MAX_CL_STATS {
		Host_Error("CL_SetStat: %d is invalid", stat)
	}

	if cls.mvdplayback {
		// C indexes the players array with lastto unchecked
		if cls.lastto >= 0 && cls.lastto < MAX_CLIENTS {
			cl.players[cls.lastto].stats[stat] = value
		}
		if Cam_TrackNum() != cls.lastto {
			return
		}
	}

	Sbar_Changed()

	if stat == STAT_ITEMS {
		// set flash times
		Sbar_Changed()
		for j := 0; j < 32; j++ {
			if value&(1<<j) != 0 && cl.stats[stat]&(1<<j) == 0 {
				cl.item_gettime[j] = float32(cl.time)
			}
		}
	}

	cl.stats[stat] = value

	if stat == STAT_TIME && cl.z_ext&Z_EXT_SERVERTIME != 0 {
		cl.servertime_works = true
		cl.servertime = float64(cl.stats[STAT_TIME]) * 0.001
	}

	TP_StatChanged(stat, value)
}

func CL_MuzzleFlash() {
	var forward vec3_t

	i := MSG_ReadShort()

	if cl_muzzleflash.value == 0 {
		return
	}

	if cl.validsequence == 0 {
		return
	}

	if uint32(i-1) >= MAX_CLIENTS {
		// a monster firing
		num_ent := cl.frames[cl.validsequence&UPDATE_MASK].packet_entities.num_entities
		for j := 0; j < num_ent; j++ {
			ent := &cl.frames[cl.validsequence&UPDATE_MASK].packet_entities.entities[j]
			if ent.number == i {
				dl := CL_AllocDlight(-i)
				AngleVectors(ent.angles, &forward, nil, nil)
				VectorMA(ent.origin, 18, forward, &dl.origin)
				dl.radius = float32(200 + (rand() & 31))
				dl.minlight = 32
				dl.die = float32(cl.time + 0.1)
				dl.type_ = int(lt_muzzleflash)
				break
			}
		}
		return
	}

	if cl_muzzleflash.value == 2 && i-1 == cl.viewplayernum {
		return
	}

	dl := CL_AllocDlight(-i)
	mod := parsecountmod
	if cls.mvdplayback {
		mod = oldparsecountmod
	}
	state := &cl.frames[mod].playerstate[i-1]
	AngleVectors(state.viewangles, &forward, nil, nil)
	VectorMA(state.origin, 18, forward, &dl.origin)
	dl.radius = float32(200 + (rand() & 31))
	dl.minlight = 32
	dl.die = float32(cl.time + 0.1)
	dl.type_ = int(lt_muzzleflash)
}

func CL_ParseFTESpawnBaseLine2() {
	bits := uint16(MSG_ReadShort())
	// entity number is low 9 bits (mask 511)
	CL_ParseDelta(&nullentitystate, &cl_entities[bits&511].baseline, int(bits))
}

func CL_ParseFTESpawnStatic2() {
	bits := uint16(MSG_ReadShort())
	es := nullentitystate
	CL_ParseDelta(&nullentitystate, &es, int(bits))

	sent := &static_entity{}
	ent := &sent.ent
	ent.model = cl.model_precache[es.modelindex]
	ent.frame = es.frame
	ent.skinnum = es.skinnum
	ent.origin = es.origin
	ent.angles = es.angles
	R_AddEfrags(ent)

	cl_linkStatic(sent)
}

func CL_ParseQizmoVoice() {
	// read the two-byte header
	MSG_ReadByte() // 10-bit block sequence number, strictly increasing
	MSG_ReadByte() // 2-bit sample number, 4 unknown bits

	// 32 bytes of voice data follow
	for i := 0; i < 32; i++ {
		MSG_ReadByte()
	}
}

func SHOWNET(x string) {
	if cl_shownet.value == 2 {
		Com_Printf("%3d:%s\n", msg_readcount-1, x)
	}
}

func CL_ParseServerMessage() {
	var i, j int

	prevcmd := -1

	if cl_shownet.value == 1 {
		Com_Printf("%d ", cl_net_message.cursize)
	} else if cl_shownet.value == 2 {
		Com_Printf("------------------\n")
	}

	cls.demomessage.cursize = 0

	Dump_Message()

	CL_ParseClientdata()
	CL_ClearProjectiles()

	// parse the message
	for {
		if msg_badread {
			Host_Error("CL_ParseServerMessage: Bad server message (while parsing %d)", prevcmd)
			break
		}

		msg_svc_start := msg_readcount
		cmd := MSG_ReadByte()

		if cmd == -1 {
			msg_readcount++ // so the EOM showner has the right value
			SHOWNET("END OF MESSAGE")
			break
		}

		if cmd == svc_qizmovoice {
			SHOWNET("svc_qizmovoice")
		} else if cmd < len(svc_strings) {
			SHOWNET(svc_strings[cmd])
		}

		Dump_Svc(cmd)

		// other commands
		switch cmd {
		default:
			Host_Error("CL_ParseServerMessage: Illegible server message %d\n", cmd)

		case svc_nop:

		case svc_disconnect:
			if cls.state == ca_connected {
				Host_Error("Server disconnected\n" +
					"Server version may not be compatible")
			} else {
				Com_DPrintf("Server disconnected\n")
				Host_EndGame() // the server will be killed if it tries to kick the local player
				Host_Abort()
			}

		case svc_time:
			MSG_ReadFloat()

		case svc_print:
			CL_ParsePrint()

		case svc_centerprint:
			SCR_CenterPrint(MSG_ReadString())

		case svc_stufftext:
			CL_ParseStufftext()

		case svc_damage:
			V_ParseDamage()

		case svc_serverdata:
			Cbuf_ExecuteEx(cbuf_svc) // make sure any stuffed commands are done
			CL_ParseServerData()
			vid.recalc_refdef = 1 // leave full screen intermission

		case svc_setangle:
			if cls.mvdplayback {
				j = MSG_ReadByte()
				mvd_fixangle |= 1 << uint(j)
				if j != Cam_TrackNum() {
					for i = 0; i < 3; i++ {
						MSG_ReadAngle()
					}
				}
			}
			if !cls.mvdplayback || (cls.mvdplayback && j == Cam_TrackNum()) {
				for i = 0; i < 3; i++ {
					cl.viewangles[i] = MSG_ReadAngle()
				}
				Mouse_SetViewAngles(cl.viewangles)
			}

		case svc_lightstyle:
			i = MSG_ReadByte()
			if i >= MAX_LIGHTSTYLES {
				Host_Error("svc_lightstyle > MAX_LIGHTSTYLES")
			}
			cl_lightstyle[i].map_ = strlcpy(MSG_ReadString(), MAX_STYLESTRING)
			cl_lightstyle[i].length = len(cl_lightstyle[i].map_)

		case svc_sound:
			CL_ParseStartSoundPacket()

		case svc_stopsound:
			i = MSG_ReadShort()
			S_StopSound(i>>3, i&7)

		case svc_updatefrags:
			Sbar_Changed()
			i = MSG_ReadByte()
			if i >= MAX_CLIENTS {
				Host_Error("CL_ParseServerMessage: svc_updatefrags > MAX_CLIENTS")
			}
			cl.players[i].frags = MSG_ReadShort()

		case svc_updateping:
			i = MSG_ReadByte()
			if i >= MAX_CLIENTS {
				Host_Error("CL_ParseServerMessage: svc_updateping > MAX_CLIENTS")
			}
			cl.players[i].ping = MSG_ReadShort()

		case svc_updatepl:
			i = MSG_ReadByte()
			if i >= MAX_CLIENTS {
				Host_Error("CL_ParseServerMessage: svc_updatepl > MAX_CLIENTS")
			}
			cl.players[i].pl = byte(MSG_ReadByte())

		case svc_updateentertime:
			// time is sent over as seconds ago
			i = MSG_ReadByte()
			if i >= MAX_CLIENTS {
				Host_Error("CL_ParseServerMessage: svc_updateentertime > MAX_CLIENTS")
			}
			now := cls.realtime
			if cls.demoplayback {
				now = cls.demotime
			}
			cl.players[i].entertime = float32(now - float64(MSG_ReadFloat()))

		case svc_spawnbaseline:
			i = MSG_ReadShort()
			if i < 0 || i >= CL_MAX_EDICTS {
				Host_Error("CL_ParseServerMessage: svc_spawnbaseline entity %d out of range", i)
			}
			CL_ParseBaseline(&cl_entities[i].baseline)

		case svc_spawnstatic:
			if !CL_ParseStatic() {
				Host_Error("CL_ParseServerMessage: CL_ParseStatic() failed")
			}

		case svc_temp_entity:
			CL_ParseTEnt()

		case svc_killedmonster:
			cl.stats[STAT_MONSTERS]++

		case svc_foundsecret:
			cl.stats[STAT_SECRETS]++

		case svc_updatestat:
			i = MSG_ReadByte()
			j = MSG_ReadByte()
			CL_SetStat(i, j)

		case svc_updatestatlong:
			i = MSG_ReadByte()
			j = MSG_ReadLong()
			CL_SetStat(i, j)

		case svc_spawnstaticsound:
			CL_ParseStaticSound()

		case svc_cdtrack:
			cl.cdtrack = MSG_ReadByte()
			CDAudio_Play(byte(cl.cdtrack), true)

		case svc_intermission:
			cl.intermission = 1
			if cls.demoplayback {
				cl.completed_time = int(cls.demotime)
			} else {
				cl.completed_time = int(cls.realtime)
			}
			cl.solo_completed_time = int(cl.servertime)
			vid.recalc_refdef = 1 // go to full screen
			for i = 0; i < 3; i++ {
				cl.simorg[i] = MSG_ReadCoord()
			}
			for i = 0; i < 3; i++ {
				cl.simangles[i] = MSG_ReadAngle()
			}
			cl.simvel = vec3_t{}
			TP_ExecTrigger("f_mapend")

		case svc_finale:
			cl.intermission = 2
			if cls.demoplayback {
				cl.completed_time = int(cls.demotime)
			} else {
				cl.completed_time = int(cls.realtime)
			}
			cl.solo_completed_time = int(cl.servertime)
			vid.recalc_refdef = 1 // go to full screen
			SCR_CenterPrint(MSG_ReadString())

		case svc_sellscreen:
			Cmd_ExecuteString("help")

		case svc_smallkick:
			cl.ideal_punchangle = -2

		case svc_bigkick:
			cl.ideal_punchangle = -4

		case svc_muzzleflash:
			CL_MuzzleFlash()

		case svc_updateuserinfo:
			CL_UpdateUserinfo()

		case svc_setinfo:
			CL_SetInfo()

		case svc_serverinfo:
			CL_ParseServerInfoChange()

		case svc_download:
			CL_ParseDownload()

		case svc_playerinfo:
			CL_ParsePlayerinfo()

		case svc_nails:
			CL_ParseProjectiles(false)

		case svc_nails2:
			if !cls.mvdplayback {
				Host_Error("CL_ParseServerMessage: svc_nails2 without cls.mvdplayback")
			}
			CL_ParseProjectiles(true)

		case svc_chokecount: // some preceding packets were choked
			i = MSG_ReadByte()
			for j = cls.netchan.incoming_acknowledged - 1; i > 0 && j > cls.netchan.outgoing_sequence-UPDATE_BACKUP; j-- {
				if cl.frames[j&UPDATE_MASK].receivedtime != -3 {
					cl.frames[j&UPDATE_MASK].receivedtime = -2
					i--
				}
			}

		case svc_modellist:
			CL_ParseModellist(0)

		case svcfte_modellistshort:
			if cls.ftexsupported&FTEX_MODELDOUBLE == 0 {
				Host_Error("Got unexpected svcfte_modellistshort")
			}
			CL_ParseModellist(1)

		case svc_soundlist:
			CL_ParseSoundlist()

		case svc_packetentities:
			CL_ParsePacketEntities(false)

		case svc_deltapacketentities:
			CL_ParsePacketEntities(true)

		case svc_maxspeed:
			cl.maxspeed = MSG_ReadFloat()

		case svc_entgravity:
			cl.entgravity = MSG_ReadFloat()

		case svc_setpause:
			if MSG_ReadByte() != 0 {
				cl.paused |= PAUSED_SERVER
			} else {
				cl.paused &^= PAUSED_SERVER
			}
			if cl.paused != 0 {
				CDAudio_Pause()
			} else {
				CDAudio_Resume()
			}

		case svcfte_lightstylecol:
			if cls.ftexsupported&FTEX_LIGHTSTYLECOLOUR == 0 {
				Host_Error("Got unexpected svcfte_lightstylecol")
			}
			i = MSG_ReadByte()
			if i >= MAX_LIGHTSTYLES {
				Host_Error("svc_lightstyle > MAX_LIGHTSTYLES")
			}
			MSG_ReadByte() // Discard the colour
			cl_lightstyle[i].map_ = strlcpy(MSG_ReadString(), MAX_STYLESTRING)
			cl_lightstyle[i].length = len(cl_lightstyle[i].map_)

		case svcfte_spawnstatic2:
			if cls.ftexsupported&FTEX_SPAWNBASELINE2 == 0 {
				Host_Error("Got unexpected svcfte_spawnstatic2")
			}
			CL_ParseFTESpawnStatic2()

		case svcfte_spawnbaseline2:
			if cls.ftexsupported&FTEX_SPAWNBASELINE2 == 0 {
				Host_Error("Got unexpected svcfte_spawnbaseline2")
			}
			CL_ParseFTESpawnBaseLine2()

		case svcfte_updatestatfloat:
			if cls.ftexsupported&FTEX_CSQC == 0 {
				Host_Error("Got unexpected svcfte_updatestatfloat")
			}
			i = MSG_ReadByte()
			f := MSG_ReadFloat()
			CL_SetStat(i, int(f))

		case svc_qizmovoice:
			CL_ParseQizmoVoice()
		}

		// C never updates prevcmd, so the error above always says -1

		if cls.demorecording {
			if cls.demomessage.cursize == 0 {
				SZ_Init(&cls.demomessage, cls.demomessage_data[:], len(cls.demomessage_data))
				SZ_Write(&cls.demomessage, cl_net_message.data[:8])
			}
			if cmd == svc_deltapacketentities {
				CL_WriteDemoEntities()
			} else {
				SZ_Write(&cls.demomessage, cl_net_message.data[msg_svc_start:msg_readcount])
			}
		}
	}

	if cls.demorecording {
		if cls.demomessage.cursize != 0 {
			CL_WriteDemoMessage(&cls.demomessage)
		}
	}

	CL_SetSolidEntities()
}

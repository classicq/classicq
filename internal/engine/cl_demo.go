// Copyright (C) 1996-1997 Id Software, Inc.
// Copyright (C) 2005-2011 Mark Olsen
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
	"encoding/binary"
	"io"
	"math"
	"os"
	"runtime"
	"strings"
)

var olddemotime, nextdemotime float32

var td_lastframe float32 // to meter out one message a frame
var td_startframe int    // cls.framecount at start
var td_starttime float32 // realtime at second frame of timedemo

// .qwz playback through qizmo.exe is not supported; the flags keep the
// control flow.
var qwz_unpacking = false
var qwz_playback = false

var demo_dir = cvar_t{name: "demo_dir", string: "qw/demos", OnChange: OnChange_demo_dir}

// little endian scalars as the demo files store them
func f32le(f float32) []byte {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], math.Float32bits(f))
	return b[:]
}

func i32le(i int32) []byte {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], uint32(i))
	return b[:]
}

func lef32(b []byte) float32 { return math.Float32frombits(binary.LittleEndian.Uint32(b)) }
func lei32(b []byte) int32   { return int32(binary.LittleEndian.Uint32(b)) }

// usercmd_t as C lays it out in memory: msec, three bytes of padding,
// angles, the three shorts, buttons and impulse. Demos store it raw.
const sizeof_usercmd_t = 24

func usercmdBytes(cmd *usercmd_t) []byte {
	b := make([]byte, sizeof_usercmd_t)
	b[0] = cmd.msec
	for i := 0; i < 3; i++ {
		binary.LittleEndian.PutUint32(b[4+4*i:], math.Float32bits(cmd.angles[i]))
	}
	binary.LittleEndian.PutUint16(b[16:], uint16(cmd.forwardmove))
	binary.LittleEndian.PutUint16(b[18:], uint16(cmd.sidemove))
	binary.LittleEndian.PutUint16(b[20:], uint16(cmd.upmove))
	b[22] = cmd.buttons
	b[23] = cmd.impulse
	return b
}

func usercmdFromBytes(b []byte, cmd *usercmd_t) {
	cmd.msec = b[0]
	for i := 0; i < 3; i++ {
		cmd.angles[i] = lef32(b[4+4*i:])
	}
	cmd.forwardmove = int16(binary.LittleEndian.Uint16(b[16:]))
	cmd.sidemove = int16(binary.LittleEndian.Uint16(b[18:]))
	cmd.upmove = int16(binary.LittleEndian.Uint16(b[20:]))
	cmd.buttons = b[22]
	cmd.impulse = b[23]
}

//=============================================================================
//								DEMO WRITING
//=============================================================================

var recordfile *os.File

var playback_recordtime float32

func DEMORECORDTIME() float32 {
	if cls.demoplayback {
		return playback_recordtime
	}
	return float32(cls.realtime)
}

const (
	DEMOCACHE_MINSIZE   = 2 * 1024 * 1024
	DEMOCACHE_FLUSHSIZE = 1024 * 1024
)

var democache sizebuf_t
var democache_available = false

func CL_Demo_Open(name string) bool {
	if democache_available {
		SZ_Clear(&democache)
	}
	f, err := os.Create(name)
	if err != nil {
		return false
	}
	recordfile = f
	return true
}

func CL_Demo_Close() {
	if democache_available {
		recordfile.Write(democache.data[:democache.cursize])
	}
	recordfile.Close()
	recordfile = nil
}

func CL_Demo_Write(data []byte) {
	size := len(data)
	if democache_available {
		if size > democache.maxsize {
			Sys_Error("CL_Demo_Write: size > democache.maxsize")
		} else if size > democache.maxsize-democache.cursize {
			Com_Printf("Warning: democache overflow...flushing\n")
			overflow := size - (democache.maxsize - democache.cursize)
			if overflow <= DEMOCACHE_FLUSHSIZE {
				overflow = min(DEMOCACHE_FLUSHSIZE, democache.cursize)
			}
			recordfile.Write(democache.data[:overflow])
			copy(democache.data, democache.data[overflow:democache.cursize])
			democache.cursize -= overflow
			SZ_Write(&democache, data)
		} else {
			SZ_Write(&democache, data)
		}
	} else {
		recordfile.Write(data)
	}
}

// os.File is unbuffered, so fflush has nothing to do here.
func CL_Demo_Flush() {
}

// Writes the current user cmd
func CL_WriteDemoCmd(pcmd *usercmd_t) {
	CL_Demo_Write(f32le(DEMORECORDTIME()))
	CL_Demo_Write([]byte{dem_cmd})

	CL_Demo_Write(usercmdBytes(pcmd))

	var t [12]byte
	for i := 0; i < 3; i++ {
		binary.LittleEndian.PutUint32(t[4*i:], math.Float32bits(cl.viewangles[i]))
	}
	CL_Demo_Write(t[:])
	CL_Demo_Flush()
}

// Dumps the current net message, prefixed by the length and view angles
func CL_WriteDemoMessage(msg *sizebuf_t) {
	CL_Demo_Write(f32le(DEMORECORDTIME()))
	CL_Demo_Write([]byte{dem_read})
	CL_Demo_Write(i32le(int32(msg.cursize)))
	CL_Demo_Write(msg.data[:msg.cursize])
	CL_Demo_Flush()
}

func CL_WriteDemoEntities() {
	MSG_WriteByte(&cls.demomessage, svc_packetentities)

	pe := &cl.frames[cls.netchan.incoming_sequence&UPDATE_MASK].packet_entities
	for ent_index := 0; ent_index < pe.num_entities; ent_index++ {
		ent := &pe.entities[ent_index]
		MSG_WriteDeltaEntity(&cl_entities[ent.number].baseline, ent, &cls.demomessage, true)
	}

	MSG_WriteShort(&cls.demomessage, 0) // end of packetentities
}

func CL_WriteStartupDemoMessage(msg *sizebuf_t, seq int) {
	if !cls.demorecording {
		return
	}

	CL_Demo_Write(f32le(DEMORECORDTIME()))
	CL_Demo_Write([]byte{dem_read})
	CL_Demo_Write(i32le(int32(msg.cursize + 8)))
	CL_Demo_Write(i32le(int32(seq)))
	CL_Demo_Write(i32le(int32(seq)))
	CL_Demo_Write(msg.data[:msg.cursize])
	CL_Demo_Flush()
}

func CL_WriteSetDemoMessage() {
	if !cls.demorecording {
		return
	}

	CL_Demo_Write(f32le(DEMORECORDTIME()))
	CL_Demo_Write([]byte{dem_set})
	CL_Demo_Write(i32le(int32(cls.netchan.outgoing_sequence)))
	CL_Demo_Write(i32le(int32(cls.netchan.incoming_sequence)))
	CL_Demo_Flush()
}

// write startup data to demo (called when demo started and cls.state == ca_active)
func CL_WriteStartupData() {
	var buf sizebuf_t
	var buf_data [MAX_MSGLEN * 2]byte
	seq := 1

	// send the serverdata
	SZ_Init(&buf, buf_data[:], len(buf_data))

	MSG_WriteByte(&buf, svc_serverdata)
	// Unlike C, keep the negotiated extensions: without them playback reads
	// playerinfo and coords in the vanilla layout and the demo is illegible.
	if cls.ftexsupported != 0 {
		MSG_WriteLong(&buf, QW_PROTOEXT_FTEX)
		MSG_WriteLong(&buf, int(cls.ftexsupported))
	}
	if cls.fte2supported != 0 {
		MSG_WriteLong(&buf, QW_PROTOEXT_FTE2)
		MSG_WriteLong(&buf, int(cls.fte2supported))
	}
	if cls.mvdprotocolextensions1 != 0 {
		MSG_WriteLong(&buf, QW_PROTOEXT_MVD1)
		MSG_WriteLong(&buf, int(cls.mvdprotocolextensions1))
	}
	MSG_WriteLong(&buf, PROTOCOL_VERSION)
	MSG_WriteLong(&buf, cl.servercount)
	MSG_WriteString(&buf, cls.gamedirfile)

	if cl.spectator != 0 {
		MSG_WriteByte(&buf, cl.playernum|128)
	} else {
		MSG_WriteByte(&buf, cl.playernum)
	}

	// send full levelname
	MSG_WriteString(&buf, cl.levelname)

	// send the movevars
	MSG_WriteFloat(&buf, movevars.gravity)
	MSG_WriteFloat(&buf, movevars.stopspeed)
	MSG_WriteFloat(&buf, cl.maxspeed)
	MSG_WriteFloat(&buf, movevars.spectatormaxspeed)
	MSG_WriteFloat(&buf, movevars.accelerate)
	MSG_WriteFloat(&buf, movevars.airaccelerate)
	MSG_WriteFloat(&buf, movevars.wateraccelerate)
	MSG_WriteFloat(&buf, movevars.friction)
	MSG_WriteFloat(&buf, movevars.waterfriction)
	MSG_WriteFloat(&buf, cl.entgravity)

	// send music
	MSG_WriteByte(&buf, svc_cdtrack)
	MSG_WriteByte(&buf, 0) // none in demos

	// send server info string
	MSG_WriteByte(&buf, svc_stufftext)
	MSG_WriteString(&buf, va("fullserverinfo \"%s\"\n", cl.serverinfo))

	// flush packet
	CL_WriteStartupDemoMessage(&buf, seq)
	seq++
	SZ_Clear(&buf)

	// soundlist
	MSG_WriteByte(&buf, svc_soundlist)
	MSG_WriteByte(&buf, 0)

	n := 0
	for n+1 < MAX_SOUNDS && cl.sound_name[n+1] != "" {
		MSG_WriteString(&buf, cl.sound_name[n+1])
		if buf.cursize > MAX_MSGLEN/2 {
			MSG_WriteByte(&buf, 0)
			MSG_WriteByte(&buf, n)
			CL_WriteStartupDemoMessage(&buf, seq)
			seq++
			SZ_Clear(&buf)
			MSG_WriteByte(&buf, svc_soundlist)
			MSG_WriteByte(&buf, n+1)
		}
		n++
	}

	if buf.cursize != 0 {
		MSG_WriteByte(&buf, 0)
		MSG_WriteByte(&buf, 0)
		CL_WriteStartupDemoMessage(&buf, seq)
		seq++
		SZ_Clear(&buf)
	}

	// modellist
	MSG_WriteByte(&buf, svc_modellist)
	MSG_WriteByte(&buf, 0)

	n = 0
	for n+1 < MAX_MODELS && cl.model_name[n+1] != "" {
		s := cl.model_name[n+1]
		// To convert Fodquake's internal "foo.bsp*1" representation back into "*1"
		if s2 := strings.LastIndexByte(s, '*'); s2 >= 0 {
			s = s[s2:]
		}
		MSG_WriteString(&buf, s)
		if buf.cursize > MAX_MSGLEN/2 {
			MSG_WriteByte(&buf, 0)
			MSG_WriteByte(&buf, n)
			CL_WriteStartupDemoMessage(&buf, seq)
			seq++
			SZ_Clear(&buf)
			MSG_WriteByte(&buf, svc_modellist)
			MSG_WriteByte(&buf, n+1)
		}
		n++
	}

	if buf.cursize != 0 {
		MSG_WriteByte(&buf, 0)
		MSG_WriteByte(&buf, 0)
		CL_WriteStartupDemoMessage(&buf, seq)
		seq++
		SZ_Clear(&buf)
	}

	// spawnstatic
	for sent := cl.first_static; sent != nil; sent = sent.next {
		ent := &sent.ent

		MSG_WriteByte(&buf, svc_spawnstatic)

		j := 1
		for ; j < MAX_MODELS; j++ {
			if ent.model == cl.model_precache[j] {
				break
			}
		}
		if j == MAX_MODELS {
			MSG_WriteByte(&buf, 0)
		} else {
			MSG_WriteByte(&buf, j)
		}

		MSG_WriteByte(&buf, ent.frame)
		MSG_WriteByte(&buf, 0)
		MSG_WriteByte(&buf, ent.skinnum)
		for j := 0; j < 3; j++ {
			MSG_WriteCoord(&buf, ent.origin[j])
			MSG_WriteAngle(&buf, ent.angles[j])
		}

		if buf.cursize > MAX_MSGLEN/2 {
			CL_WriteStartupDemoMessage(&buf, seq)
			seq++
			SZ_Clear(&buf)
		}
	}

	// static sounds are skipped in demos, life is hard

	// baselines
	var blankes entity_state_t
	for i := 0; i < CL_MAX_EDICTS; i++ {
		es := &cl_entities[i].baseline

		if *es != blankes {
			MSG_WriteByte(&buf, svc_spawnbaseline)
			MSG_WriteShort(&buf, i)

			MSG_WriteByte(&buf, es.modelindex)
			MSG_WriteByte(&buf, es.frame)
			MSG_WriteByte(&buf, es.colormap)
			MSG_WriteByte(&buf, es.skinnum)
			for j := 0; j < 3; j++ {
				MSG_WriteCoord(&buf, es.origin[j])
				MSG_WriteAngle(&buf, es.angles[j])
			}

			if buf.cursize > MAX_MSGLEN/2 {
				CL_WriteStartupDemoMessage(&buf, seq)
				seq++
				SZ_Clear(&buf)
			}
		}
	}

	MSG_WriteByte(&buf, svc_stufftext)
	MSG_WriteString(&buf, va("cmd spawn %d 0\n", cl.servercount))

	if buf.cursize != 0 {
		CL_WriteStartupDemoMessage(&buf, seq)
		seq++
		SZ_Clear(&buf)
	}

	// send current status of all other players
	for i := 0; i < MAX_CLIENTS; i++ {
		player := &cl.players[i]

		MSG_WriteByte(&buf, svc_updatefrags)
		MSG_WriteByte(&buf, i)
		MSG_WriteShort(&buf, player.frags)

		MSG_WriteByte(&buf, svc_updateping)
		MSG_WriteByte(&buf, i)
		MSG_WriteShort(&buf, player.ping)

		MSG_WriteByte(&buf, svc_updatepl)
		MSG_WriteByte(&buf, i)
		MSG_WriteByte(&buf, int(player.pl))

		MSG_WriteByte(&buf, svc_updateentertime)
		MSG_WriteByte(&buf, i)
		MSG_WriteFloat(&buf, float32(cls.realtime-float64(player.entertime)))

		MSG_WriteByte(&buf, svc_updateuserinfo)
		MSG_WriteByte(&buf, i)
		MSG_WriteLong(&buf, player.userid)
		MSG_WriteString(&buf, player.userinfo)

		if buf.cursize > MAX_MSGLEN/2 {
			CL_WriteStartupDemoMessage(&buf, seq)
			seq++
			SZ_Clear(&buf)
		}
	}

	// send all current light styles
	for i := 0; i < MAX_LIGHTSTYLES; i++ {
		if cl_lightstyle[i].length == 0 {
			continue // don't send empty lightstyle strings
		}
		MSG_WriteByte(&buf, svc_lightstyle)
		MSG_WriteByte(&buf, i)
		MSG_WriteString(&buf, cl_lightstyle[i].map_)
	}

	for i := 0; i < MAX_CL_STATS; i++ {
		if cl.stats[i] == 0 {
			continue // no need to send zero values
		}
		if cl.stats[i] >= 0 && cl.stats[i] <= 255 {
			MSG_WriteByte(&buf, svc_updatestat)
			MSG_WriteByte(&buf, i)
			MSG_WriteByte(&buf, cl.stats[i])
		} else {
			MSG_WriteByte(&buf, svc_updatestatlong)
			MSG_WriteByte(&buf, i)
			MSG_WriteLong(&buf, cl.stats[i])
		}
		if buf.cursize > MAX_MSGLEN/2 {
			CL_WriteStartupDemoMessage(&buf, seq)
			seq++
			SZ_Clear(&buf)
		}
	}

	// get the client to check and download skins
	// when that is completed, a begin command will be issued
	MSG_WriteByte(&buf, svc_stufftext)
	MSG_WriteString(&buf, "skins\n")

	CL_WriteStartupDemoMessage(&buf, seq)

	CL_WriteSetDemoMessage()
}

//=============================================================================
//								DEMO READING
//=============================================================================

var playbackfile *os.File

func CL_Demo_Read(buf []byte) {
	if _, err := io.ReadFull(playbackfile, buf); err != nil {
		Host_Error("Unexpected end of demo")
	}
}

var demo_prevtime float32

// When a demo is playing back, all NET_SendMessages are skipped, and
// NET_GetMessages are read from the demo file. Whenever cl.time gets past
// the last received message, another message is read from the demo file.
func CL_GetDemoMessage() bool {
	var demotime float32
	var b4 [4]byte
	var b1 [1]byte

	if qwz_unpacking {
		return false
	}

	if cl.paused&PAUSED_DEMO != 0 {
		return false
	}

	if cls.mvdplayback {
		if demo_prevtime < nextdemotime {
			demo_prevtime = nextdemotime
		}
		if cls.demotime+1.0 < float64(nextdemotime) {
			cls.demotime = float64(nextdemotime) - 1.0
		}
	}

	for {
		// read the time from the packet
		sizeofDemotime := int64(4)
		if cls.mvdplayback {
			sizeofDemotime = 1
			CL_Demo_Read(b1[:])
			newtime := b1[0]
			demotime = float32(float64(demo_prevtime) + float64(newtime)*0.001)
			if cls.demotime-float64(nextdemotime) > 0.0001 && nextdemotime != demotime {
				olddemotime = nextdemotime
				cls.netchan.incoming_sequence++
				cls.netchan.incoming_acknowledged++
				cls.netchan.frame_latency = 0
				cls.netchan.last_received = float32(cls.demotime) // just to happy timeout check
				nextdemotime = demotime
			}
		} else {
			CL_Demo_Read(b4[:])
			demotime = lef32(b4[:])
		}

		if cls.realactualfirstdemotimestamp == -1 {
			cls.realactualfirstdemotimestamp = float64(demotime)
		}

		cls.realactualdemotime = float64(demotime) - cls.realactualfirstdemotimestamp

		playback_recordtime = demotime

		// decide if it is time to grab the next message
		if cls.timedemo {
			if td_lastframe < 0 {
				td_lastframe = demotime
			} else if demotime > td_lastframe {
				td_lastframe = demotime
				playbackfile.Seek(-sizeofDemotime, io.SeekCurrent) // rewind back to time
				return false                                       // already read this frame's message
			}
			if td_starttime == 0 && cls.state == ca_active {
				td_starttime = float32(Sys_DoubleTime())
				td_startframe = cls.framecount
			}
			cls.demotime = float64(demotime) // warp
		} else if cl.paused&PAUSED_SERVER == 0 && cls.state == ca_active { // always grab until fully connected
			if cls.mvdplayback {
				if nextdemotime < demotime {
					playbackfile.Seek(-sizeofDemotime, io.SeekCurrent) // rewind back to time
					return false                                       // don't need another message yet
				}
			} else {
				if cls.demotime < float64(demotime) {
					if cls.demotime+1.0 < float64(demotime) {
						cls.demotime = float64(demotime) - 1.0 // too far back
					}
					playbackfile.Seek(-sizeofDemotime, io.SeekCurrent) // rewind back to time
					return false                                       // don't need another message yet
				}
			}
		} else {
			cls.demotime = float64(demotime) // we're warping
		}

		if cls.mvdplayback {
			demo_prevtime = demotime
		}

		CL_Demo_Read(b1[:]) // get the msg type
		c := b1[0]

		readit := false
		switch c & 7 {
		case dem_cmd:
			// user sent input
			i := cls.netchan.outgoing_sequence & UPDATE_MASK
			pcmd := &cl.frames[i].cmd
			var raw [sizeof_usercmd_t]byte
			CL_Demo_Read(raw[:])
			usercmdFromBytes(raw[:], pcmd)
			cl.frames[i].senttime = cls.realtime
			cl.frames[i].receivedtime = -1 // we haven't gotten a reply yet
			cls.netchan.outgoing_sequence++
			var angles [12]byte
			CL_Demo_Read(angles[:])
			for j := 0; j < 3; j++ {
				cl.viewangles[j] = lef32(angles[4*j:])
			}
			if cl.spectator != 0 {
				Cam_TryLock()
			}
			if cls.demorecording {
				CL_WriteDemoCmd(pcmd)
			}
			continue

		case dem_read:
			readit = true

		case dem_set:
			CL_Demo_Read(b4[:])
			cls.netchan.outgoing_sequence = int(lei32(b4[:]))
			CL_Demo_Read(b4[:])
			cls.netchan.incoming_sequence = int(lei32(b4[:]))
			if cls.mvdplayback {
				cls.netchan.incoming_acknowledged = cls.netchan.incoming_sequence
			}
			continue

		case dem_multiple:
			CL_Demo_Read(b4[:])
			cls.lastto = int(lei32(b4[:]))
			cls.lasttype = dem_multiple
			readit = true

		case dem_single:
			cls.lastto = int(c >> 3)
			cls.lasttype = dem_single
			readit = true

		case dem_stats:
			cls.lastto = int(c >> 3)
			cls.lasttype = dem_stats
			readit = true

		case dem_all:
			cls.lastto = 0
			cls.lasttype = dem_all
			readit = true

		default:
			Host_Error("Corrupted demo")
			return false
		}

		if readit {
			// get the next message
			CL_Demo_Read(b4[:])
			cl_net_message.cursize = int(lei32(b4[:]))
			if cl_net_message.cursize > cl_net_message.maxsize {
				Com_DPrintf("CL_GetDemoMessage: cl_net_message.cursize > cl_net_message.maxsize")
				Host_EndGame()
				Host_Abort()
			}
			if cl_net_message.cursize < 0 {
				Host_Error("Unexpected end of demo")
			}
			if cl_net_message.cursize == 0 {
				continue
			}
			CL_Demo_Read(cl_net_message.data[:cl_net_message.cursize])

			if cls.mvdplayback {
				switch cls.lasttype {
				case dem_multiple:
					tracknum := Cam_TrackNum()
					if tracknum == -1 || cls.lastto&(1<<uint(tracknum)) == 0 {
						continue
					}
				case dem_single:
					tracknum := Cam_TrackNum()
					if tracknum == -1 || cls.lastto != spec_track {
						continue
					}
				}
			}
			return true
		}
	}
}

//=============================================================================
//								DEMO RECORDING
//=============================================================================

var demoname string
var autorecording = false

func OnChange_demo_dir(v *cvar_t, str string) bool {
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

// stop recording a demo
func CL_StopRecording() {
	if !cls.demorecording {
		return
	}

	// write a disconnect message to the demo file
	SZ_Clear(&cl_net_message)
	MSG_WriteLong(&cl_net_message, -1) // -1 sequence means out of band
	MSG_WriteByte(&cl_net_message, svc_disconnect)
	MSG_WriteString(&cl_net_message, "EndOfDemo")
	CL_WriteDemoMessage(&cl_net_message)

	// finish up
	CL_Demo_Close()
	cls.demorecording = false
}

func CL_Stop_f() {
	if !cls.demorecording {
		Com_Printf("Not recording a demo\n")
		return
	}
	if autorecording {
		CL_AutoRecord_StopMatch()
	} else {
		CL_StopRecording()
		Com_Printf("Completed demo\n")
	}
}

func CL_DemoDirectory() string {
	if demo_dir.string != "" {
		return strlcpy(va("%s/%s", com_basedir, demo_dir.string), MAX_OSPATH*2)
	}
	return strlcpy(cls.gamedir, MAX_OSPATH*2)
}

func CL_Record_f() {
	switch Cmd_Argc() {
	case 1:
		if autorecording {
			Com_Printf("Auto demo recording is in progress\n")
		} else if cls.demorecording {
			Com_Printf("Recording to %s\n", demoname)
		} else {
			Com_Printf("Not recording\n")
		}
	case 2:
		if cls.mvdplayback {
			Com_Printf("Cannot record during mvd playback\n")
			return
		}
		if cls.state != ca_active && cls.state != ca_disconnected {
			Com_Printf("Cannot record whilst connecting\n")
			return
		}
		if autorecording {
			Com_Printf("Auto demo recording must be stopped first!\n")
			return
		}
		if cls.demorecording {
			CL_Stop_f()
		}
		if !Util_Is_Valid_Filename(Cmd_Argv(1)) {
			Com_Printf("%s", Util_Invalid_Filename_Msg(Cmd_Argv(1)))
			return
		}

		// open the demo file
		nameext := strlcpy(Cmd_Argv(1), MAX_OSPATH*2)
		nameext = COM_ForceExtension(nameext, ".qwd")
		name := strlcpy(cls.gamedir+"/"+nameext, MAX_OSPATH*2)
		if !CL_Demo_Open(name) {
			Com_Printf("Error: Couldn't record to %s. Make sure path exists.\n", name)
			return
		}
		cls.demorecording = true
		if cls.state == ca_active {
			CL_WriteStartupData()
		}
		demoname = strlcpy(nameext, 2*MAX_OSPATH)
		Com_Printf("Recording to %s\n", nameext)
	default:
		Com_Printf("Usage: %s [demoname]\n", Cmd_Argv(0))
	}
}

func CL_RecordDemo(dir, name string, autorecord bool) bool {
	exts := []string{"qwd", "qwz"}

	if cls.state != ca_active {
		Com_Printf("You must be connected before using easyrecord\n")
		return false
	}
	if cls.mvdplayback {
		Com_Printf("Cannot record during mvd playback\n")
		return false
	}
	if autorecording {
		Com_Printf("Auto demo recording must be stopped first!\n")
		return false
	}
	if cls.demorecording {
		CL_Stop_f()
	}
	if !Util_Is_Valid_Filename(name) {
		Com_Printf("%s", Util_Invalid_Filename_Msg(name))
		return false
	}

	name = COM_ForceExtension(name, ".qwd")

	var extendedname string
	if autorecord {
		extendedname = strlcpy(name, MAX_OSPATH*2)
	} else {
		strippedname := COM_CopyAndStripExtension(name, MAX_OSPATH*2)
		fullname := va("%s/%s", dir, strippedname)
		num := Util_Extend_Filename(fullname, exts)
		if num == -1 {
			Com_Printf("Error: no available filenames\n")
			return false
		}
		extendedname = strlcpy(va("%s_%03d.qwd", strippedname, num), MAX_OSPATH*2)
	}

	fullname := va("%s/%s", dir, extendedname)

	// open the demo file
	if !CL_Demo_Open(fullname) {
		FS_CreatePath(fullname)
		if !CL_Demo_Open(fullname) {
			Com_Printf("Error: Couldn't open %s\n", fullname)
			return false
		}
	}

	cls.demorecording = true
	CL_WriteStartupData()

	if !autorecord {
		Com_Printf("Recording to %s\n", extendedname)
		demoname = strlcpy(extendedname, 2*MAX_OSPATH)
	}

	return true
}

func CL_EasyRecord_f() {
	var name string

	switch Cmd_Argc() {
	case 1:
		name = MT_MatchName()
	case 2:
		name = Cmd_Argv(1)
	default:
		Com_Printf("Usage: %s [demoname]\n", Cmd_Argv(0))
		return
	}

	CL_RecordDemo(CL_DemoDirectory(), name, false)
}

//=============================================================================
//							DEMO AUTO RECORDING
//=============================================================================

var temp_demo_ready = false
var demo_auto_matchname string // cl_screen.c has its own auto_matchname
var auto_starttime float32

const TEMP_DEMO_NAME = "_!_temp_!_.qwd"

func CL_AutoRecord_StopMatch() {
	if !autorecording {
		return
	}

	autorecording = false
	CL_StopRecording()
	temp_demo_ready = true

	if match_auto_record.value == 2 {
		CL_AutoRecord_SaveMatch()
	} else {
		Com_Printf("Auto demo recording completed\n")
	}
}

func CL_AutoRecord_CancelMatch() {
	if !autorecording {
		return
	}

	autorecording = false
	CL_StopRecording()
	temp_demo_ready = true

	if match_auto_record.value == 2 {
		if float32(cls.realtime)-auto_starttime > match_auto_minlength.value {
			CL_AutoRecord_SaveMatch()
		} else {
			Com_Printf("Auto demo recording cancelled\n")
		}
	} else {
		Com_Printf("Auto demo recording completed\n")
	}
}

func CL_AutoRecord_StartMatch(demoname string) {
	temp_demo_ready = false

	if match_auto_record.value == 0 {
		return
	}

	if cls.demorecording {
		if autorecording {
			autorecording = false
			CL_StopRecording()
		} else {
			Com_Printf("Auto demo recording skipped (already recording)\n")
			return
		}
	}

	demo_auto_matchname = strlcpy(demoname, 2*MAX_OSPATH)

	if !CL_RecordDemo(MT_TempDirectory(), TEMP_DEMO_NAME, true) {
		Com_Printf("Auto demo recording failed to start!\n")
		return
	}

	autorecording = true
	auto_starttime = float32(cls.realtime)
	Com_Printf("Auto demo recording commenced\n")
}

func CL_AutoRecord_Status() int {
	if temp_demo_ready {
		return 2
	}
	if autorecording {
		return 1
	}
	return 0
}

func CL_AutoRecord_SaveMatch() {
	exts := []string{"qwd", "qwz"}

	if !temp_demo_ready {
		return
	}

	temp_demo_ready = false

	dir := CL_DemoDirectory()
	tempname := va("%s/%s", MT_TempDirectory(), TEMP_DEMO_NAME)
	fullsavedname := va("%s/%s", dir, demo_auto_matchname)
	num := Util_Extend_Filename(fullsavedname, exts)
	if num == -1 {
		Com_Printf("Error: no available filenames\n")
		return
	}
	savedname := strlcpy(va("%s_%03d.qwd", demo_auto_matchname, num), 2*MAX_OSPATH)
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
		Com_Printf("Match demo saved to %s\n", savedname)
	}
}

//=============================================================================
//							QIZMO COMPRESSION
//=============================================================================

func CL_CheckQizmoCompletion() {
}

func PlayQWZDemo() {
	Com_Printf("Error: .qwz playback through qizmo.exe is not available yet\n")
}

//=============================================================================
//							DEMO PLAYBACK
//=============================================================================

var demostarttime float64

func CL_StopPlayback() {
	if !cls.demoplayback {
		return
	}

	if Movie_IsCapturing() {
		Movie_Stop()
	}

	if playbackfile != nil {
		playbackfile.Close()
	}
	playbackfile = nil
	cls.mvdplayback = false
	cls.demoplayback = false
	cl.paused &^= PAUSED_DEMO

	Dump_End()

	if cls.timedemo {
		cls.timedemo = false

		frames := cls.framecount - td_startframe - 1
		time := float32(Sys_DoubleTime()) - td_starttime
		if time <= 0 {
			time = 1
		}
		Com_Printf("%d frames %5.1f seconds %5.1f fps\n", frames, time, float32(frames)/time)
	}
}

func CL_Play_f() {
	ext := []string{".qwd", ".mvd"}

	if Cmd_Argc() != 2 {
		Com_Printf("Usage: %s <demoname>\n", Cmd_Argv(0))
		return
	}

	Host_EndGame()

	name := strlcpy(Cmd_Argv(1), 2*MAX_OSPATH-4)
	if runtime.GOOS == "windows" && len(name) > 4 && Q_strcasecmp(name[len(name)-4:], ".qwz") == 0 {
		PlayQWZDemo()
		if playbackfile == nil && !qwz_playback {
			return
		}
	} else {
		for _, e := range ext {
			if playbackfile != nil {
				break
			}
			name = strlcpy(Cmd_Argv(1), 2*MAX_OSPATH-4)
			name = COM_DefaultExtension(name, e, 2*MAX_OSPATH)
			if strings.HasPrefix(name, "../") || strings.HasPrefix(name, "..\\") {
				playbackfile, _ = os.Open(va("%s/%s", com_basedir, name[3:]))
				if playbackfile == nil && ro_data_path != "" && ro_data_path != com_basedir {
					playbackfile, _ = os.Open(va("%s/%s", ro_data_path, name[3:]))
				}
			} else {
				playbackfile, _ = FS_FOpenFile(name)
			}
		}

		if playbackfile == nil {
			Com_Printf("Error: Couldn't open %s\n", Cmd_Argv(1))
			return
		}

		Com_Printf("Playing demo from %s\n", COM_SkipPath(name))
	}

	Dump_Header(COM_SkipPath(name))

	cls.demoplayback = true
	cls.mvdplayback = Q_strcasecmp(name[len(name)-3:], "mvd") == 0
	cls.state = ca_demostart
	Netchan_Setup(NS_CLIENT, &cls.netchan, cl_net_from, 0)
	cls.demotime = 0
	demostarttime = -1.0
	cls.realactualfirstdemotimestamp = -1
	cls.realactualdemotime = 0
	olddemotime = 0
	nextdemotime = 0
	cls.findtrack = true
	cls.lastto = 0
	cls.lasttype = 0
	CL_ClearPredict()

	if cls.mvdplayback && cls.demorecording {
		CL_Stop_f()
	}
}

func CL_TimeDemo_f() {
	if Cmd_Argc() != 2 {
		Com_Printf("timedemo <demoname> : gets demo speeds\n")
		return
	}

	CL_Play_f()

	if cls.state != ca_demostart {
		return
	}

	// td_starttime will be grabbed at the second frame of the demo, so all
	// the loading time doesn't get counted
	cls.timedemo = true
	td_starttime = 0
	td_startframe = cls.framecount
	td_lastframe = -1 // get a new message this frame
	cls.realactualfirstdemotimestamp = -1
	cls.realactualdemotime = 0
}

//=============================================================================
//								DEMO TOOLS
//=============================================================================

func CL_Demo_SetSpeed_f() {
	if Cmd_Argc() != 2 {
		Com_Printf("Usage: %s [speed %%]\n", Cmd_Argv(0))
		return
	}
	Cvar_SetValue(&cl_demospeed, float32(atof(Cmd_Argv(1))/100.0))
}

func CL_Demo_Jump_f() {
	const usage_message = "Usage: %s [+|-][m:]<s> (seconds)\n"
	seconds := 0
	relative := 0

	if !cls.demoplayback {
		Com_Printf("Error: not playing a demo\n")
		return
	}

	if cls.state < ca_active {
		Com_Printf("Error: demo must be active first\n")
		return
	}

	if Cmd_Argc() != 2 {
		Com_Printf(usage_message, Cmd_Argv(0))
		return
	}

	text := Cmd_Argv(1)
	if at(text, 0) == '-' {
		text = text[1:]
		relative = -1
	} else if at(text, 0) == '+' {
		text = text[1:]
		relative = 1
	} else if !isdigit(at(text, 0)) {
		Com_Printf(usage_message, Cmd_Argv(0))
		return
	}

	seen_col := 0
	for i := 0; i < len(text); i++ {
		if text[i] == ':' {
			seen_col++
		} else if !isdigit(text[i]) {
			Com_Printf(usage_message, Cmd_Argv(0))
			return
		}
		if seen_col >= 2 {
			Com_Printf(usage_message, Cmd_Argv(0))
			return
		}
	}

	if col := strings.IndexByte(text, ':'); col >= 0 {
		seconds += 60 * atoi(text)
		text = text[col+1:]
	}
	seconds += atoi(text)

	var newdemotime float64
	if relative != 0 {
		newdemotime = cls.demotime + float64(relative*seconds)
	} else {
		newdemotime = demostarttime + float64(seconds)
	}

	if newdemotime < cls.demotime {
		Com_Printf("Error: cannot demo_jump backwards\n")
		return
	}

	cls.demotime = newdemotime
}

func CL_CvarDemoInit() {
	Cmd_AddCommand("record", CL_Record_f)
	Cmd_AddCommand("stop", CL_Stop_f)
	Cmd_AddCommand("playdemo", CL_Play_f)
	Cmd_AddCommand("timedemo", CL_TimeDemo_f)
	Cmd_AddCommand("easyrecord", CL_EasyRecord_f)
	Cmd_AddCommand("demo_setspeed", CL_Demo_SetSpeed_f)
	Cmd_AddCommand("demo_jump", CL_Demo_Jump_f)

	Cvar_SetCurrentGroup(CVAR_GROUP_DEMO)
	Cvar_Register(&demo_dir)
	Cvar_ResetCurrentGroup()
}

func CL_Demo_Init() {
	democache_available = false
	if parm := COM_CheckParm("-democache"); parm != 0 && parm+1 < com_argc {
		democache_size := Q_atoi(com_argv[parm+1]) * 1024
		democache_size = max(democache_size, DEMOCACHE_MINSIZE)
		Com_Printf("Democache initialized (%.1fMB)\n", float32(democache_size)/(1024*1024))
		SZ_Init(&democache, make([]byte, democache_size), democache_size)
		democache_available = true
	}
}

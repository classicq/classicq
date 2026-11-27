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

// protocol.h -- communications protocols

const (
	PROTOCOL_VERSION = 28
	QW_CHECK_HASH    = 0x5157
)

const (
	PORT_CLIENT = 27001
	PORT_MASTER = 27000
	PORT_SERVER = 27500
)

// out of band message id bytes
// M = master, S = server, C = client, A = any
const (
	S2C_CHALLENGE      = 'c'
	S2C_CONNECTION     = 'j'
	A2A_PING           = 'k' // respond with an A2A_ACK
	A2A_ACK            = 'l' // general acknowledgement without info
	A2A_NACK           = 'm' // [+ comment] general failure
	A2A_ECHO           = 'e' // for echoing
	A2C_PRINT          = 'n' // print a message on client
	S2M_HEARTBEAT      = 'a' // + serverinfo + userlist + fraglist
	A2C_CLIENT_COMMAND = 'B' // + command line
	S2M_SHUTDOWN       = 'C'
)

// server to client
const (
	svc_bad                 = 0
	svc_nop                 = 1
	svc_disconnect          = 2
	svc_updatestat          = 3  // [byte] [byte]
	svc_setview             = 5  // [short] entity number
	svc_sound               = 6  // <see code>
	svc_time                = 7  // [float] server time
	svc_print               = 8  // [byte] id [string] null terminated string
	svc_stufftext           = 9  // [string] stuffed into client's console buffer
	svc_setangle            = 10 // [angle3] set the view angle to this absolute value
	svc_serverdata          = 11 // [long] protocol ...
	svc_lightstyle          = 12 // [byte] [string]
	svc_updatefrags         = 14 // [byte] [short]
	svc_stopsound           = 16 // <see code>
	svc_damage              = 19
	svc_spawnstatic         = 20
	svc_spawnbaseline       = 22
	svc_temp_entity         = 23 // variable
	svc_setpause            = 24 // [byte] on / off
	svc_centerprint         = 26 // [string] to put in center of the screen
	svc_killedmonster       = 27
	svc_foundsecret         = 28
	svc_spawnstaticsound    = 29 // [coord3] [byte] samp [byte] vol [byte] aten
	svc_intermission        = 30 // [vec3_t] origin [vec3_t] angle
	svc_finale              = 31 // [string] text
	svc_cdtrack             = 32 // [byte] track
	svc_sellscreen          = 33
	svc_smallkick           = 34 // set client punchangle to 2
	svc_bigkick             = 35 // set client punchangle to 4
	svc_updateping          = 36 // [byte] [short]
	svc_updateentertime     = 37 // [byte] [float]
	svc_updatestatlong      = 38 // [byte] [long]
	svc_muzzleflash         = 39 // [short] entity
	svc_updateuserinfo      = 40 // [byte] slot [long] uid [string] userinfo
	svc_download            = 41 // [short] size [size bytes]
	svc_playerinfo          = 42 // variable
	svc_nails               = 43 // [byte] num [48 bits] xyzpy 12 12 12 4 8
	svc_chokecount          = 44 // [byte] packets choked
	svc_modellist           = 45 // [strings]
	svc_soundlist           = 46 // [strings]
	svc_packetentities      = 47 // [...]
	svc_deltapacketentities = 48 // [...]
	svc_maxspeed            = 49 // maxspeed change, for prediction
	svc_entgravity          = 50 // gravity change, for prediction
	svc_setinfo             = 51 // setinfo on a client
	svc_serverinfo          = 52 // serverinfo
	svc_updatepl            = 53 // [byte] [byte]
	svc_nails2              = 54
)

// FTE extensions
const (
	svcfte_spawnstatic2    = 21 // FTEX_SPAWNBASELINE2 (FTE_PEXT_SPAWNSTATIC2)
	svcfte_lightstylecol   = 57 // FTEX_LIGHTSTYLECOLOUR
	svcfte_modellistshort  = 60 // FTEX_MODELDOUBLE
	svcfte_spawnbaseline2  = 66 // FTEX_SPAWNSTATIC2 || FTEX_REPLACEMENTDELTAS
	svcfte_updatestatfloat = 79 // FTEX_CSQC
)

// Qizmo extensions
const svc_qizmovoice = 83

// client to server
const (
	clc_bad       = 0
	clc_nop       = 1
	clc_move      = 3 // [[usercmd_t]
	clc_stringcmd = 4 // [string] message
	clc_delta     = 5 // [byte] sequence number, requests delta compression of message
	clc_tmove     = 6 // teleport request, spectator only
	clc_upload    = 7 // teleport request, spectator only
)

// playerinfo flags from server
// playerinfo always sends: playernum, flags, origin[] and framenumber
const (
	PF_MSEC        = 1 << 0
	PF_COMMAND     = 1 << 1
	PF_VELOCITY1   = 1 << 2
	PF_VELOCITY2   = 1 << 3
	PF_VELOCITY3   = 1 << 4
	PF_MODEL       = 1 << 5
	PF_SKINNUM     = 1 << 6
	PF_EFFECTS     = 1 << 7
	PF_WEAPONFRAME = 1 << 8  // only sent for view player
	PF_DEAD        = 1 << 9  // don't block movement any more
	PF_GIB         = 1 << 10 // offset the view height differently

	// bits 11..13 are player move type bits
	PF_PMC_SHIFT = 11
	PF_PMC_MASK  = 7

	PF_EXTRA_PFS = 1 << 15
	PF_TRANS_Z   = 1 << 17
)

// player move types
const (
	PMC_NORMAL           = 0 // normal ground movement
	PMC_NORMAL_JUMP_HELD = 1 // a fake move type for jump debouncing
	PMC_OLD_SPECTATOR    = 2 // fly through walls (QW compatibility mode)
	PMC_SPECTATOR        = 3 // fly through walls
	PMC_FLY              = 4 // fly, bump into walls
)

// if the high bit of the client to server byte is set, the low bits are
// client move cmd bits
// ms and angle2 are always sent, the others are optional
const (
	CM_ANGLE1  = 1 << 0
	CM_ANGLE3  = 1 << 1
	CM_FORWARD = 1 << 2
	CM_SIDE    = 1 << 3
	CM_UP      = 1 << 4
	CM_BUTTONS = 1 << 5
	CM_IMPULSE = 1 << 6
	CM_ANGLE2  = 1 << 7
)

const (
	DF_ORIGIN      = 1
	DF_ANGLES      = 1 << 3
	DF_EFFECTS     = 1 << 6
	DF_SKINNUM     = 1 << 7
	DF_DEAD        = 1 << 8
	DF_GIB         = 1 << 9
	DF_WEAPONFRAME = 1 << 10
	DF_MODEL       = 1 << 11
)

// the first 16 bits of a packetentities update holds 9 bits of entity number
// and 7 bits of flags
const (
	U_ORIGIN1  = 1 << 9
	U_ORIGIN2  = 1 << 10
	U_ORIGIN3  = 1 << 11
	U_ANGLE2   = 1 << 12
	U_FRAME    = 1 << 13
	U_REMOVE   = 1 << 14 // REMOVE this entity, don't add it
	U_MOREBITS = 1 << 15

	// if MOREBITS is set, these additional flags are read in next
	U_ANGLE1   = 1 << 0
	U_ANGLE3   = 1 << 1
	U_MODEL    = 1 << 2
	U_COLORMAP = 1 << 3
	U_SKIN     = 1 << 4
	U_EFFECTS  = 1 << 5
	U_SOLID    = 1 << 6 // the entity should be solid for prediction
)

// FTE extensions
const (
	FTE_U_EVENMORE = 1 << 7

	FTE_U2_SCALE      = 1 << 0
	FTE_U2_TRANS      = 1 << 1
	FTE_U2_FATNESS    = 1 << 2
	FTE_U2_MODELDBL   = 1 << 3
	FTE_U2_UNUSED1    = 1 << 4
	FTE_U2_ENTITYDBL  = 1 << 5
	FTE_U2_ENTITYDBL2 = 1 << 6
	FTE_U2_YETMORE    = 1 << 7
	FTE_U2_DRAWFLAGS  = 1 << 8
	FTE_U2_ABSLIGHT   = 1 << 9
	FTE_U2_COLOURMOD  = 1 << 10
	FTE_U2_DPFLAGS    = 1 << 11
	FTE_U2_TAGINFO    = 1 << 12
	FTE_U2_LIGHT      = 1 << 13
	FTE_U2_EFFECT16   = 1 << 14
	FTE_U2_FARMORE    = 1 << 15
)

// a sound with no channel is a local only sound
// the sound field has bits 0-2: channel, 3-12: entity
const (
	SND_VOLUME      = 1 << 15 // a byte
	SND_ATTENUATION = 1 << 14 // a byte

	DEFAULT_SOUND_PACKET_VOLUME      = 255
	DEFAULT_SOUND_PACKET_ATTENUATION = 1.0
)

// temp entity events
const (
	TE_SPIKE          = 0
	TE_SUPERSPIKE     = 1
	TE_GUNSHOT        = 2
	TE_EXPLOSION      = 3
	TE_TAREXPLOSION   = 4
	TE_LIGHTNING1     = 5
	TE_LIGHTNING2     = 6
	TE_WIZSPIKE       = 7
	TE_KNIGHTSPIKE    = 8
	TE_LIGHTNING3     = 9
	TE_LAVASPLASH     = 10
	TE_TELEPORT       = 11
	TE_BLOOD          = 12
	TE_LIGHTNINGBLOOD = 13
)

// ZQuake protocol extensions (*z_ext serverinfo key)
const (
	Z_EXT_PM_TYPE        = 1 << 0
	Z_EXT_PM_TYPE_NEW    = 1 << 1
	Z_EXT_SERVERTIME     = 1 << 3
	Z_EXT_JOIN_OBSERVE   = 1 << 5
	SUPPORTED_EXTENSIONS = Z_EXT_PM_TYPE | Z_EXT_PM_TYPE_NEW | Z_EXT_SERVERTIME | Z_EXT_JOIN_OBSERVE
)

const (
	QW_PROTOEXT_HUFF = 0x46465548
	QW_PROTOEXT_FTEX = 0x58455446
	QW_PROTOEXT_FTE2 = 0x32455446
	QW_PROTOEXT_MVD1 = 0x3144564D // 'MVD1' little-endian, mvdsv 2017+
)

// MVD-specific extensions (QW_PROTOEXT_MVD1)
const (
	MVD_PEXT1_FLOATCOORDS       = 1 << 0
	MVD_PEXT1_HIGHLAGTELEPORT   = 1 << 1
	MVD_PEXT1_SERVERSIDEWEAPON  = 1 << 2
	MVD_PEXT1_DEBUG_ANTILAG     = 1 << 3
	MVD_PEXT1_DEBUG_WEAPON      = 1 << 4
	MVD_PEXT1_HIDDEN_MESSAGES   = 1 << 5
	MVD_PEXT1_SERVERSIDEWEAPON2 = 1 << 6
	MVD_PEXT1_ANTILAG_CLIENTPOS = 1 << 7
)

const (
	FTEX_SCALE            = 0x00000002
	FTEX_LIGHTSTYLECOLOUR = 0x00000004
	FTEX_TRANS            = 0x00000008
	FTEX_FATNESS          = 0x00000100
	FTEX_MODELDOUBLE      = 0x00001000
	FTEX_ENTITYDBL        = 0x00002000
	FTEX_ENTITYDBL2       = 0x00004000
	FTEX_FLOATCOORDS      = 0x00008000
	FTEX_COLOURMOD        = 0x00080000
	FTEX_SPAWNBASELINE2   = 0x00400000
	FTEX_CHUNKEDDOWNLOADS = 0x20000000
	FTEX_CSQC             = 0x40000000

	FTEX_SUPPORTED = FTEX_FLOATCOORDS |
		FTEX_CHUNKEDDOWNLOADS |
		FTEX_MODELDOUBLE |
		FTEX_SPAWNBASELINE2 |
		FTEX_TRANS |
		FTEX_COLOURMOD |
		FTEX_SCALE |
		FTEX_FATNESS |
		FTEX_LIGHTSTYLECOLOUR |
		FTEX_ENTITYDBL |
		FTEX_ENTITYDBL2

	FTE2_SUPPORTED = 0
	MVD1_SUPPORTED = 0
)

/*
==========================================================
  ELEMENTS COMMUNICATED ACROSS THE NET
==========================================================
*/

const (
	MAX_CLIENTS   = 32
	UPDATE_BACKUP = 64 // copies of entity_state_t to keep buffered (must be power of two)
	UPDATE_MASK   = UPDATE_BACKUP - 1
)

// entity_state_t is the information conveyed from the server in an update
// message
type entity_state_t struct {
	number     int // edict index
	flags      int // nolerp, etc
	origin     vec3_t
	angles     vec3_t
	modelindex int
	frame      int
	colormap   int
	skinnum    int
	effects    int
}

const (
	MAX_MVD_PACKET_ENTITIES = 300
	MAX_PACKET_ENTITIES     = 64
)

type packet_entities_t struct {
	num_entities int
	entities     [MAX_MVD_PACKET_ENTITIES]entity_state_t
}

type usercmd_t struct {
	msec                          byte
	angles                        vec3_t
	forwardmove, sidemove, upmove int16
	buttons                       byte
	impulse                       byte
}

// usercmd button bits
const (
	BUTTON_ATTACK = 1 << 0
	BUTTON_JUMP   = 1 << 1
	BUTTON_USE    = 1 << 2
)

const (
	dem_cmd      = 0
	dem_read     = 1
	dem_set      = 2
	dem_multiple = 3
	dem_single   = 4
	dem_stats    = 5
	dem_all      = 6
)

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
	"math"
	"math/bits"
	"strings"
	"sync"
)

// common.h
const (
	MAX_QPATH  = 64  // max length of a quake game pathname
	MAX_OSPATH = 128 // max length of a filesystem pathname

	ON_EPSILON = 0.1 // point on plane side epsilon

	MAX_MSGLEN   = 1450 // max length of a reliable message
	MAX_DATAGRAM = 1450 // max length of unreliable message

	// per-level limits
	MAX_LIGHTSTYLES = 64
	MAX_MODELS      = 2048 // >256 requires FTEX_MODELDOUBLE on the wire
	MAX_SOUNDS      = 256  // sound index is still a byte everywhere

	SAVEGAME_COMMENT_LENGTH = 39

	MAX_STYLESTRING = 64

	// stats are integers communicated to the client by the server
	MAX_CL_STATS       = 32
	STAT_HEALTH        = 0
	STAT_WEAPON        = 2
	STAT_AMMO          = 3
	STAT_ARMOR         = 4
	STAT_SHELLS        = 6
	STAT_NAILS         = 7
	STAT_ROCKETS       = 8
	STAT_CELLS         = 9
	STAT_ACTIVEWEAPON  = 10
	STAT_TOTALSECRETS  = 11
	STAT_TOTALMONSTERS = 12
	STAT_SECRETS       = 13 // bumped on client side by svc_foundsecret
	STAT_MONSTERS      = 14 // bumped by svc_killedmonster
	STAT_ITEMS         = 15
	STAT_VIEWHEIGHT    = 16 // Z_EXT_VIEWHEIGHT protocol extension
	STAT_TIME          = 17 // Z_EXT_SERVERTIME protocol extension

	// item flags
	IT_SHOTGUN          = 1
	IT_SUPER_SHOTGUN    = 2
	IT_NAILGUN          = 4
	IT_SUPER_NAILGUN    = 8
	IT_GRENADE_LAUNCHER = 16
	IT_ROCKET_LAUNCHER  = 32
	IT_LIGHTNING        = 64
	IT_SUPER_LIGHTNING  = 128
	IT_SHELLS           = 256
	IT_NAILS            = 512
	IT_ROCKETS          = 1024
	IT_CELLS            = 2048
	IT_AXE              = 4096
	IT_ARMOR1           = 8192
	IT_ARMOR2           = 16384
	IT_ARMOR3           = 32768
	IT_SUPERHEALTH      = 65536
	IT_KEY1             = 131072
	IT_KEY2             = 262144
	IT_INVISIBILITY     = 524288
	IT_INVULNERABILITY  = 1048576
	IT_SUIT             = 2097152
	IT_QUAD             = 4194304
	IT_SIGIL1           = 1 << 28
	IT_SIGIL2           = 1 << 29
	IT_SIGIL3           = 1 << 30
	IT_SIGIL4           = 1 << 31

	// print flags
	PRINT_LOW    = 0 // pickup messages
	PRINT_MEDIUM = 1 // death messages
	PRINT_HIGH   = 2 // critical messages
	PRINT_CHAT   = 3 // chat messages

	// game types sent by serverinfo
	GAME_COOP       = 0
	GAME_DEATHMATCH = 1

	MAX_INFO_STRING       = 196
	MAX_SERVERINFO_STRING = 512
	MAX_LOCALINFO_STRING  = 32768
)

const MAX_NUM_ARGVS = 50

var com_mutex sync.Mutex

var nullcmd usercmd_t // guaranteed to be zero

var largv [MAX_NUM_ARGVS + 1]string

var (
	developer  = cvar_t{name: "developer", string: "0"}
	registered = cvar_t{name: "registered", string: "0"}
	mapname    = cvar_t{name: "mapname", string: "", flags: CVAR_ROM}
)

var com_serveractive = false

var com_gamedirfile string

/*
All of Quake's data access is through a hierarchic file system, but the
contents of the file system can be transparently merged from several sources.

The "base directory" is the path to the directory holding the quake.exe and
all game directories. The "game directory" is the first tree on the search
path and directory that all generated files (savegames, screenshots, demos,
config files) will be saved to.
*/

/*
============================================================================
					LIBRARY REPLACEMENT FUNCTIONS
============================================================================
*/

func Q_atoi(str string) int {
	var val int32
	var sign int32
	i := 0

	if at(str, i) == '-' {
		sign = -1
		i++
	} else {
		sign = 1
	}

	// check for hex
	if at(str, i) == '0' && (at(str, i+1) == 'x' || at(str, i+1) == 'X') {
		i += 2
		for {
			c := at(str, i)
			i++
			if c >= '0' && c <= '9' {
				val = (val << 4) + int32(c) - '0'
			} else if c >= 'a' && c <= 'f' {
				val = (val << 4) + int32(c) - 'a' + 10
			} else if c >= 'A' && c <= 'F' {
				val = (val << 4) + int32(c) - 'A' + 10
			} else {
				return int(val * sign)
			}
		}
	}

	// check for character
	if at(str, i) == '\'' {
		return int(sign * int32(at(str, i+1)))
	}

	// assume decimal
	for {
		c := at(str, i)
		i++
		if c < '0' || c > '9' {
			return int(val * sign)
		}
		val = val*10 + int32(c) - '0'
	}
}

func Q_atof(str string) float32 {
	var val float64
	var sign int
	i := 0

	if at(str, i) == '-' {
		sign = -1
		i++
	} else {
		sign = 1
	}

	// check for hex
	if at(str, i) == '0' && (at(str, i+1) == 'x' || at(str, i+1) == 'X') {
		i += 2
		for {
			c := at(str, i)
			i++
			if c >= '0' && c <= '9' {
				val = (val * 16) + float64(c) - '0'
			} else if c >= 'a' && c <= 'f' {
				val = (val * 16) + float64(c) - 'a' + 10
			} else if c >= 'A' && c <= 'F' {
				val = (val * 16) + float64(c) - 'A' + 10
			} else {
				return float32(val * float64(sign))
			}
		}
	}

	// check for character
	if at(str, i) == '\'' {
		return float32(sign * int(at(str, i+1)))
	}

	// assume decimal
	decimal := -1
	total := 0
	for {
		c := at(str, i)
		i++
		if c == '.' {
			decimal = total
			continue
		}
		if c < '0' || c > '9' {
			break
		}
		val = val*10 + float64(c) - '0'
		total++
	}

	if decimal == -1 {
		return float32(val * float64(sign))
	}
	for total > decimal {
		val /= 10
		total--
	}

	return float32(val * float64(sign))
}

func Com_HashKey(name string) int {
	var v uint32
	for i := 0; i < len(name); i++ {
		v += uint32(name[i]) &^ 32 // make it case insensitive
	}
	return int(v % 32)
}

/*
============================================================================
					BYTE ORDER FUNCTIONS
============================================================================
*/

func ShortSwap(l int16) int16 {
	return int16(bits.ReverseBytes16(uint16(l)))
}

func LongSwap(l int32) int32 {
	return int32(bits.ReverseBytes32(uint32(l)))
}

func FloatSwap(f float32) float32 {
	return math.Float32frombits(bits.ReverseBytes32(math.Float32bits(f)))
}

// The targets are all little endian.
func BigShort(x int16) int16     { return ShortSwap(x) }
func BigLong(x int32) int32      { return LongSwap(x) }
func BigFloat(x float32) float32 { return FloatSwap(x) }

func LittleShort(x int16) int16     { return x }
func LittleLong(x int32) int32      { return x }
func LittleFloat(x float32) float32 { return x }

//===========================================================================

type sizebuf_t struct {
	allowoverflow bool // if false, do a Sys_Error
	overflowed    bool // set to true if the buffer size failed
	data          []byte
	maxsize       int
	cursize       int
}

func SZ_Init(buf *sizebuf_t, data []byte, length int) {
	*buf = sizebuf_t{data: data, maxsize: length}
}

func SZ_Clear(buf *sizebuf_t) {
	buf.cursize = 0
	buf.overflowed = false
}

func SZ_GetSpace(buf *sizebuf_t, length int) []byte {
	if buf.cursize+length > buf.maxsize {
		if !buf.allowoverflow {
			Sys_Error("SZ_GetSpace: overflow without allowoverflow set (%d)", buf.maxsize)
		}
		if length > buf.maxsize {
			Sys_Error("SZ_GetSpace: %d is > full buffer size", length)
		}
		Sys_Printf("SZ_GetSpace: overflow\n") // because Com_Printf may be redirected
		SZ_Clear(buf)
		buf.overflowed = true
	}

	data := buf.data[buf.cursize : buf.cursize+length]
	buf.cursize += length
	return data
}

func SZ_Write(buf *sizebuf_t, data []byte) {
	copy(SZ_GetSpace(buf, len(data)), data)
}

// strcats onto the sizebuf, keeping a single trailing 0
func SZ_Print(buf *sizebuf_t, data string) {
	n := len(data) + 1
	var space []byte
	if buf.cursize == 0 || buf.data[buf.cursize-1] != 0 {
		space = SZ_GetSpace(buf, n) // no trailing 0
	} else {
		buf.cursize--
		space = SZ_GetSpace(buf, n) // write over trailing 0
	}
	copy(space, data)
	space[n-1] = 0
}

//============================================================================

func COM_SkipPath(pathname string) string {
	last := 0
	for i := 0; i < len(pathname); i++ {
		if pathname[i] == '/' {
			last = i + 1
		}
	}
	return pathname[last:]
}

func COM_StripExtension(s string) string {
	if dot := strings.LastIndexByte(s, '.'); dot >= 0 {
		return s[:dot]
	}
	return s
}

func COM_CopyAndStripExtension(in string, maxlength int) string {
	if dot := strings.LastIndexByte(in, '.'); dot >= 0 && dot+1 < maxlength {
		maxlength = dot + 1
	}
	return strlcpy(in, maxlength)
}

func COM_FileExtension(in string) string {
	dot := strings.LastIndexByte(in, '.')
	if dot < 0 {
		return ""
	}
	ext := in[dot+1:]
	if len(ext) > 7 {
		ext = ext[:7]
	}
	return ext
}

// If path doesn't have a .EXT, append extension (extension should include the .)
func COM_DefaultExtension(path, extension string, path_size int) string {
	for i := len(path) - 1; i > 0 && path[i] != '/'; i-- {
		if path[i] == '.' {
			return path // it has an extension
		}
	}
	return strlcat(path, extension, path_size)
}

// If path doesn't have an extension or has a different extension, append(!)
// the specified extension (should include the .)
func COM_ForceExtension(path, extension string) string {
	if strings.HasSuffix(path, extension) {
		return path
	}
	return strlcat(path, extension, MAX_OSPATH)
}

//============================================================================

const MAX_COM_TOKEN = 1024

var com_token string

var (
	com_argc int
	com_argv []string
)

// Parse a token out of a string. The second result is false where C
// returns NULL: end of data or a nil input.
func COM_Parse(data string) (string, bool) {
	var c byte
	i := 0
	buf := make([]byte, 0, 64)

	com_token = ""

skipwhite:
	for {
		c = at(data, i)
		if !(c == ' ' || c == '\t' || c == '\r' || c == '\n') {
			break
		}
		i++
	}

	if c == 0 {
		return "", false // end of file;
	}

	// skip // comments
	if c == '/' && at(data, i+1) == '/' {
		for at(data, i) != 0 && at(data, i) != '\n' {
			i++
		}
		goto skipwhite
	}

	// handle quoted strings specially
	if c == '"' {
		i++
		for len(buf) < MAX_COM_TOKEN-1 {
			c = at(data, i)
			i++
			if c == '"' || c == 0 {
				if c == 0 {
					i--
				}
				com_token = string(buf)
				return data[i:], true
			}
			buf = append(buf, c)
		}
	}

	if len(buf) < MAX_COM_TOKEN-1 {
		// parse a regular word
		for {
			buf = append(buf, c)
			i++
			if len(buf) >= MAX_COM_TOKEN-1 {
				break
			}
			c = at(data, i)
			if !(c != 0 && c != ' ' && c != '\t' && c != '\n' && c != '\r') {
				break
			}
		}
	}

	com_token = string(buf)
	return data[i:], true
}

func COM_CheckRegistered() {
	if FS_FileExists("gfx/pop.lmp") != 0 {
		Cvar_Set(&registered, "1")
	}
}

// Adds the given string at the end of the current argument list
func COM_AddParm(parm string) {
	if com_argc >= MAX_NUM_ARGVS {
		return
	}
	largv[com_argc] = parm
	com_argc++
}

// Returns the position (1 to argc-1) in the program's argument list
// where the given parameter appears, or 0 if not present
func COM_CheckParm(parm string) int {
	for i := 1; i < com_argc; i++ {
		if parm == com_argv[i] {
			return i
		}
	}
	return 0
}

func COM_Argc() int {
	return com_argc
}

func COM_Argv(arg int) string {
	if arg < 0 || arg >= com_argc {
		return ""
	}
	return com_argv[arg]
}

func COM_ClearArgv(arg int) {
	if arg < 0 || arg >= com_argc {
		return
	}
	com_argv[arg] = ""
}

func COM_InitArgv(argv []string) {
	for com_argc = 0; com_argc < MAX_NUM_ARGVS && com_argc < len(argv); com_argc++ {
		largv[com_argc] = argv[com_argc]
	}
	largv[com_argc] = ""
	com_argv = largv[:]
}

func COM_Init() int {
	Cvar_SetCurrentGroup(CVAR_GROUP_NO_GROUP)
	Cvar_Register(&developer)
	Cvar_Register(&registered)
	Cvar_Register(&mapname)
	Cvar_ResetCurrentGroup()
	return 1
}

func COM_Shutdown() {
}

// does a varargs printf into a temp buffer, so I don't need to have varargs
// versions of all text functions.
func va(format string, a ...any) string {
	return fmt.Sprintf(format, a...)
}

/*
=============================================================================
QUAKE FILESYSTEM
=============================================================================
*/

var (
	com_filesize int
	com_netpath  string
)

const MAX_FILES_IN_PACK = 2048

var (
	com_gamedir string
	com_basedir string
)

/*
=====================================================================
  INFO STRINGS
=====================================================================
*/

// Searches the string for the given key and returns the associated value,
// or an empty string.
func Info_ValueForKey(s, key string) string {
	i := 0
	if at(s, i) == '\\' {
		i++
	}
	for {
		start := i
		for at(s, i) != '\\' {
			if at(s, i) == 0 {
				return ""
			}
			i++
		}
		pkey := s[start:i]
		i++

		start = i
		for at(s, i) != '\\' && at(s, i) != 0 {
			i++
		}
		value := s[start:i]

		if key == pkey {
			return value
		}

		if at(s, i) == 0 {
			return ""
		}
		i++
	}
}

func Info_RemoveKey(s, key string) string {
	if strings.Contains(key, "\\") {
		Com_Printf("Can't use a key with a \\\n")
		return s
	}

	i := 0
	for {
		start := i
		if at(s, i) == '\\' {
			i++
		}
		kstart := i
		for at(s, i) != '\\' {
			if at(s, i) == 0 {
				return s
			}
			i++
		}
		pkey := s[kstart:i]
		i++

		for at(s, i) != '\\' && at(s, i) != 0 {
			i++
		}

		if key == pkey {
			return s[:start] + s[i:]
		}

		if at(s, i) == 0 {
			return s
		}
	}
}

func Info_RemovePrefixedKeys(start string, prefix byte) string {
	s := start
	i := 0
	for {
		if at(s, i) == '\\' {
			i++
		}
		kstart := i
		for at(s, i) != '\\' {
			if at(s, i) == 0 {
				return s
			}
			i++
		}
		pkey := s[kstart:i]
		i++

		for at(s, i) != '\\' && at(s, i) != 0 {
			i++
		}

		if at(pkey, 0) == prefix {
			s = Info_RemoveKey(s, pkey)
			i = 0
		}

		if at(s, i) == 0 {
			return s
		}
	}
}

func Info_SetValueForStarKey(s, key, value string, maxsize int) string {
	if strings.Contains(key, "\\") || strings.Contains(value, "\\") {
		Com_Printf("Can't use keys or values with a \\\n")
		return s
	}

	if strings.Contains(key, "\"") || strings.Contains(value, "\"") {
		Com_Printf("Can't use keys or values with a \"\n")
		return s
	}

	if len(key) > 63 || len(value) > 63 {
		Com_Printf("Keys and values must be < 64 characters.\n")
		return s
	}

	if v := Info_ValueForKey(s, key); v != "" {
		// key exists, make sure we have enough room for new value, if we
		// don't, don't change it!
		if len(value)-len(v)+len(s) >= maxsize {
			Com_Printf("Info string length exceeded\n")
			return s
		}
	}

	s = Info_RemoveKey(s, key)
	if value == "" {
		return s
	}

	pair := "\\" + key + "\\" + value

	if len(pair)+len(s) >= maxsize {
		Com_Printf("Info string length exceeded\n")
		return s
	}

	// only copy ascii values
	out := []byte(s)
	for i := 0; i < len(pair); i++ {
		if pair[i] > 13 {
			out = append(out, pair[i])
		}
	}
	return string(out)
}

func Info_SetValueForKey(s, key, value string, maxsize int) string {
	if at(key, 0) == '*' {
		Com_Printf("Can't set * keys\n")
		return s
	}
	return Info_SetValueForStarKey(s, key, value, maxsize)
}

const INFO_PRINT_FIRST_COLUMN_WIDTH = 20

func Info_Print(s string) {
	i := 0
	if at(s, i) == '\\' {
		i++
	}
	for at(s, i) != 0 {
		start := i
		for at(s, i) != 0 && at(s, i) != '\\' {
			i++
		}
		key := s[start:i]
		if len(key) < INFO_PRINT_FIRST_COLUMN_WIDTH {
			key += strings.Repeat(" ", INFO_PRINT_FIRST_COLUMN_WIDTH-len(key))
		}
		Com_Printf("%s", key)

		if at(s, i) == 0 {
			Com_Printf("MISSING VALUE\n")
			return
		}

		i++
		start = i
		for at(s, i) != 0 && at(s, i) != '\\' {
			i++
		}
		value := s[start:i]
		if at(s, i) != 0 {
			i++
		}
		Com_Printf("%s\n", value)
	}
}

//============================================================================

// Only the first 512 bytes of the table are initialized, the rest is just
// zeros. This is an idiocy in QW but we can't change this, or checksums will
// not match.
var chktbl = [1024]byte{
	0x78, 0xd2, 0x94, 0xe3, 0x41, 0xec, 0xd6, 0xd5, 0xcb, 0xfc, 0xdb, 0x8a, 0x4b, 0xcc, 0x85, 0x01,
	0x23, 0xd2, 0xe5, 0xf2, 0x29, 0xa7, 0x45, 0x94, 0x4a, 0x62, 0xe3, 0xa5, 0x6f, 0x3f, 0xe1, 0x7a,
	0x64, 0xed, 0x5c, 0x99, 0x29, 0x87, 0xa8, 0x78, 0x59, 0x0d, 0xaa, 0x0f, 0x25, 0x0a, 0x5c, 0x58,
	0xfb, 0x00, 0xa7, 0xa8, 0x8a, 0x1d, 0x86, 0x80, 0xc5, 0x1f, 0xd2, 0x28, 0x69, 0x71, 0x58, 0xc3,
	0x51, 0x90, 0xe1, 0xf8, 0x6a, 0xf3, 0x8f, 0xb0, 0x68, 0xdf, 0x95, 0x40, 0x5c, 0xe4, 0x24, 0x6b,
	0x29, 0x19, 0x71, 0x3f, 0x42, 0x63, 0x6c, 0x48, 0xe7, 0xad, 0xa8, 0x4b, 0x91, 0x8f, 0x42, 0x36,
	0x34, 0xe7, 0x32, 0x55, 0x59, 0x2d, 0x36, 0x38, 0x38, 0x59, 0x9b, 0x08, 0x16, 0x4d, 0x8d, 0xf8,
	0x0a, 0xa4, 0x52, 0x01, 0xbb, 0x52, 0xa9, 0xfd, 0x40, 0x18, 0x97, 0x37, 0xff, 0xc9, 0x82, 0x27,
	0xb2, 0x64, 0x60, 0xce, 0x00, 0xd9, 0x04, 0xf0, 0x9e, 0x99, 0xbd, 0xce, 0x8f, 0x90, 0x4a, 0xdd,
	0xe1, 0xec, 0x19, 0x14, 0xb1, 0xfb, 0xca, 0x1e, 0x98, 0x0f, 0xd4, 0xcb, 0x80, 0xd6, 0x05, 0x63,
	0xfd, 0xa0, 0x74, 0xa6, 0x86, 0xf6, 0x19, 0x98, 0x76, 0x27, 0x68, 0xf7, 0xe9, 0x09, 0x9a, 0xf2,
	0x2e, 0x42, 0xe1, 0xbe, 0x64, 0x48, 0x2a, 0x74, 0x30, 0xbb, 0x07, 0xcc, 0x1f, 0xd4, 0x91, 0x9d,
	0xac, 0x55, 0x53, 0x25, 0xb9, 0x64, 0xf7, 0x58, 0x4c, 0x34, 0x16, 0xbc, 0xf6, 0x12, 0x2b, 0x65,
	0x68, 0x25, 0x2e, 0x29, 0x1f, 0xbb, 0xb9, 0xee, 0x6d, 0x0c, 0x8e, 0xbb, 0xd2, 0x5f, 0x1d, 0x8f,
	0xc1, 0x39, 0xf9, 0x8d, 0xc0, 0x39, 0x75, 0xcf, 0x25, 0x17, 0xbe, 0x96, 0xaf, 0x98, 0x9f, 0x5f,
	0x65, 0x15, 0xc4, 0x62, 0xf8, 0x55, 0xfc, 0xab, 0x54, 0xcf, 0xdc, 0x14, 0x06, 0xc8, 0xfc, 0x42,
	0xd3, 0xf0, 0xad, 0x10, 0x08, 0xcd, 0xd4, 0x11, 0xbb, 0xca, 0x67, 0xc6, 0x48, 0x5f, 0x9d, 0x59,
	0xe3, 0xe8, 0x53, 0x67, 0x27, 0x2d, 0x34, 0x9e, 0x9e, 0x24, 0x29, 0xdb, 0x69, 0x99, 0x86, 0xf9,
	0x20, 0xb5, 0xbb, 0x5b, 0xb0, 0xf9, 0xc3, 0x67, 0xad, 0x1c, 0x9c, 0xf7, 0xcc, 0xef, 0xce, 0x69,
	0xe0, 0x26, 0x8f, 0x79, 0xbd, 0xca, 0x10, 0x17, 0xda, 0xa9, 0x88, 0x57, 0x9b, 0x15, 0x24, 0xba,
	0x84, 0xd0, 0xeb, 0x4d, 0x14, 0xf5, 0xfc, 0xe6, 0x51, 0x6c, 0x6f, 0x64, 0x6b, 0x73, 0xec, 0x85,
	0xf1, 0x6f, 0xe1, 0x67, 0x25, 0x10, 0x77, 0x32, 0x9e, 0x85, 0x6e, 0x69, 0xb1, 0x83, 0x00, 0xe4,
	0x13, 0xa4, 0x45, 0x34, 0x3b, 0x40, 0xff, 0x41, 0x82, 0x89, 0x79, 0x57, 0xfd, 0xd2, 0x8e, 0xe8,
	0xfc, 0x1d, 0x19, 0x21, 0x12, 0x00, 0xd7, 0x66, 0xe5, 0xc7, 0x10, 0x1d, 0xcb, 0x75, 0xe8, 0xfa,
	0xb6, 0xee, 0x7b, 0x2f, 0x1a, 0x25, 0x24, 0xb9, 0x9f, 0x1d, 0x78, 0xfb, 0x84, 0xd0, 0x17, 0x05,
	0x71, 0xb3, 0xc8, 0x18, 0xff, 0x62, 0xee, 0xed, 0x53, 0xab, 0x78, 0xd3, 0x65, 0x2d, 0xbb, 0xc7,
	0xc1, 0xe7, 0x70, 0xa2, 0x43, 0x2c, 0x7c, 0xc7, 0x16, 0x04, 0xd2, 0x45, 0xd5, 0x6b, 0x6c, 0x7a,
	0x5e, 0xa1, 0x50, 0x2e, 0x31, 0x5b, 0xcc, 0xe8, 0x65, 0x8b, 0x16, 0x85, 0xbf, 0x82, 0x83, 0xfb,
	0xde, 0x9f, 0x36, 0x48, 0x32, 0x79, 0xd6, 0x9b, 0xfb, 0x52, 0x45, 0xbf, 0x43, 0xf7, 0x0b, 0x0b,
	0x19, 0x19, 0x31, 0xc3, 0x85, 0xec, 0x1d, 0x8c, 0x20, 0xf0, 0x3a, 0xfa, 0x80, 0x4d, 0x2c, 0x7d,
	0xac, 0x60, 0x09, 0xc0, 0x40, 0xee, 0xb9, 0xeb, 0x13, 0x5b, 0xe8, 0x2b, 0xb1, 0x20, 0xf0, 0xce,
	0x4c, 0xbd, 0xc6, 0x04, 0x86, 0x70, 0xc6, 0x33, 0xc3, 0x15, 0x0f, 0x65, 0x19, 0xfd, 0xc2, 0xd3,
}

// For proxy protecting
func COM_BlockSequenceCRCByte(base []byte, length int, sequence int) byte {
	p := chktbl[uint32(sequence)%uint32(len(chktbl)-4):]

	if length > 60 {
		length = 60
	}
	var chkb [60 + 4]byte
	copy(chkb[:], base[:length])

	chkb[length] = byte(sequence&0xff) ^ p[0]
	chkb[length+1] = p[1]
	chkb[length+2] = byte((sequence>>8)&0xff) ^ p[2]
	chkb[length+3] = p[3]

	length += 4

	crc := CRC_Block(chkb[:length])
	return byte(crc & 0xff)
}

//=====================================================================

const MAXPRINTMSG = 4096

var rd_print func(string)

// Only the server ever redirects; the client build keeps these for parity.
func Com_BeginRedirect(RedirectedPrint func(string)) {
	com_mutex.Lock()
	rd_print = RedirectedPrint
	com_mutex.Unlock()
}

func Com_EndRedirect() {
	com_mutex.Lock()
	rd_print = nil
	com_mutex.Unlock()
}

// All console printing must go through this in order to be logged to disk
func Com_Printf(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	if len(msg) > MAXPRINTMSG-1 {
		msg = msg[:MAXPRINTMSG-1]
	}

	com_mutex.Lock()
	defer com_mutex.Unlock()

	if rd_print != nil {
		// add to redirected message
		rd_print(msg)
		return
	}

	// also echo to debugging console
	Sys_Printf("%s", msg)

	// write it to the scrollable buffer
	Con_Print(msg)
}

func Com_ErrorPrintf(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	if len(msg) > MAXPRINTMSG-1 {
		msg = msg[:MAXPRINTMSG-1]
	}

	com_mutex.Lock()
	defer com_mutex.Unlock()

	if rd_print != nil {
		rd_print(msg)
		return
	}

	Sys_Printf("Error: %s", msg)

	Con_Print("&cf00Error: ")
	Con_Print(msg)
}

// A Com_Printf that only shows up if the "developer" cvar is set
func Com_DPrintf(format string, a ...any) {
	if developer.value == 0 {
		return // don't confuse non-developers with techie stuff...
	}
	Com_Printf("%s", fmt.Sprintf(format, a...))
}

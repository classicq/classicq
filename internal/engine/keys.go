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
	"strings"
)

// keys.h

type keynum_t int

// these are the key numbers that should be passed to Key_Event
const (
	K_TAB    keynum_t = 9
	K_ENTER  keynum_t = 13
	K_ESCAPE keynum_t = 27
	K_SPACE  keynum_t = 32
)

// normal keys should be passed as lowercased ascii
const (
	K_BACKSPACE keynum_t = 127 + iota
	K_CAPSLOCK
	K_PRINTSCR
	K_SCRLCK
	K_PAUSE
	K_UPARROW
	K_DOWNARROW
	K_LEFTARROW
	K_RIGHTARROW

	K_ALT
	K_LALT
	K_RALT
	K_CTRL
	K_LCTRL
	K_RCTRL
	K_SHIFT
	K_LSHIFT
	K_RSHIFT
	K_F1
	K_F2
	K_F3
	K_F4
	K_F5
	K_F6
	K_F7
	K_F8
	K_F9
	K_F10
	K_F11
	K_F12
	K_INS
	K_DEL
	K_PGDN
	K_PGUP
	K_HOME
	K_END
	K_WIN
	K_LWIN
	K_RWIN
	K_MENU

	// Keypad stuff..
	KP_NUMLOCK
	KP_SLASH
	KP_STAR
	KP_HOME
	KP_UPARROW
	KP_PGUP
	KP_MINUS
	KP_LEFTARROW
	KP_5
	KP_RIGHTARROW
	KP_PLUS
	KP_END
	KP_DOWNARROW
	KP_PGDN
	KP_INS
	KP_DEL
	KP_ENTER
)

// mouse buttons generate virtual keys
const (
	K_MOUSE1 keynum_t = 200 + iota
	K_MOUSE2
	K_MOUSE3
	K_MOUSE4
	K_MOUSE5
	K_MOUSE6
	K_MOUSE7
	K_MOUSE8
	K_MOUSE9
	K_MOUSE10

	// joystick buttons
	K_JOY1
	K_JOY2
	K_JOY3
	K_JOY4

	// aux keys are for multi-buttoned joysticks to generate so they can use
	// the normal binding process
	K_AUX1
	K_AUX2
	K_AUX3
	K_AUX4
	K_AUX5
	K_AUX6
	K_AUX7
	K_AUX8
	K_AUX9
	K_AUX10
	K_AUX11
	K_AUX12
	K_AUX13
	K_AUX14
	K_AUX15
	K_AUX16
	K_AUX17
	K_AUX18
	K_AUX19
	K_AUX20
	K_AUX21
	K_AUX22
	K_AUX23
	K_AUX24
	K_AUX25
	K_AUX26
	K_AUX27
	K_AUX28
	K_AUX29
	K_AUX30
	K_AUX31
	K_AUX32

	// JACK: Intellimouse(c) Mouse Wheel Support
	K_MWHEELUP
	K_MWHEELDOWN

	// OS code detected a copy or paste key combination
	K_COPY
	K_PASTE
)

type keydest_t int

const (
	key_game keydest_t = iota
	key_console
	key_message
	key_menu
	key_serverbrowser
	key_lua
)

// keys.c

// key up events are sent even if in console mode

var cl_chatmode = cvar_t{name: "cl_chatmode", string: "2"}
var m_wheel_factor = cvar_t{name: "m_wheel_factor", string: "0"}

var key_lines [32][MAXCMDLINE]byte
var key_linepos int

var edit_line int
var history_line int

var key_dest keydest_t

var keybindings [256]*string // nil is unbound, an empty binding is legal
var consolekeys [256]bool    // if true, can't be rebound while in console
var menubound [256]bool      // if true, can't be rebound while in menu
var keyshift [256]int        // key to map to if shift held down in console
var key_repeats [256]int     // if > 1, it is autorepeating
var keydown [256]bool
var keyactive [256]bool

type keyname_t struct {
	name   string
	keynum int
}

var keynames = []keyname_t{
	{"TAB", int(K_TAB)},
	{"ENTER", int(K_ENTER)},
	{"ESCAPE", int(K_ESCAPE)},
	{"SPACE", int(K_SPACE)},
	{"BACKSPACE", int(K_BACKSPACE)},

	{"CAPSLOCK", int(K_CAPSLOCK)},
	{"PRINTSCR", int(K_PRINTSCR)},
	{"SCRLCK", int(K_SCRLCK)},
	{"SCROLLOCK", int(K_SCRLCK)}, // FIXME
	{"PAUSE", int(K_PAUSE)},

	{"UPARROW", int(K_UPARROW)},
	{"DOWNARROW", int(K_DOWNARROW)},
	{"LEFTARROW", int(K_LEFTARROW)},
	{"RIGHTARROW", int(K_RIGHTARROW)},

	{"ALT", int(K_ALT)},
	{"LALT", int(K_LALT)},
	{"RALT", int(K_RALT)},
	{"CTRL", int(K_CTRL)},
	{"LCTRL", int(K_LCTRL)},
	{"RCTRL", int(K_RCTRL)},
	{"SHIFT", int(K_SHIFT)},
	{"LSHIFT", int(K_LSHIFT)},
	{"RSHIFT", int(K_RSHIFT)},

	{"WINKEY", int(K_WIN)},
	{"LWINKEY", int(K_LWIN)},
	{"RWINKEY", int(K_RWIN)},
	{"POPUPMENU", int(K_MENU)},

	// Keypad stuff..

	{"NUMLOCK", int(KP_NUMLOCK)},
	{"KP_NUMLCK", int(KP_NUMLOCK)},
	{"KP_NUMLOCK", int(KP_NUMLOCK)},
	{"KP_SLASH", int(KP_SLASH)},
	{"KP_DIVIDE", int(KP_SLASH)},
	{"KP_STAR", int(KP_STAR)},
	{"KP_MULTIPLY", int(KP_STAR)},

	{"KP_MINUS", int(KP_MINUS)},

	{"KP_HOME", int(KP_HOME)},
	{"KP_7", int(KP_HOME)},
	{"KP_UPARROW", int(KP_UPARROW)},
	{"KP_8", int(KP_UPARROW)},
	{"KP_PGUP", int(KP_PGUP)},
	{"KP_9", int(KP_PGUP)},
	{"KP_PLUS", int(KP_PLUS)},

	{"KP_LEFTARROW", int(KP_LEFTARROW)},
	{"KP_4", int(KP_LEFTARROW)},
	{"KP_5", int(KP_5)},
	{"KP_RIGHTARROW", int(KP_RIGHTARROW)},
	{"KP_6", int(KP_RIGHTARROW)},

	{"KP_END", int(KP_END)},
	{"KP_1", int(KP_END)},
	{"KP_DOWNARROW", int(KP_DOWNARROW)},
	{"KP_2", int(KP_DOWNARROW)},
	{"KP_PGDN", int(KP_PGDN)},
	{"KP_3", int(KP_PGDN)},

	{"KP_INS", int(KP_INS)},
	{"KP_0", int(KP_INS)},
	{"KP_DEL", int(KP_DEL)},
	{"KP_ENTER", int(KP_ENTER)},

	{"F1", int(K_F1)},
	{"F2", int(K_F2)},
	{"F3", int(K_F3)},
	{"F4", int(K_F4)},
	{"F5", int(K_F5)},
	{"F6", int(K_F6)},
	{"F7", int(K_F7)},
	{"F8", int(K_F8)},
	{"F9", int(K_F9)},
	{"F10", int(K_F10)},
	{"F11", int(K_F11)},
	{"F12", int(K_F12)},

	{"INS", int(K_INS)},
	{"DEL", int(K_DEL)},
	{"PGDN", int(K_PGDN)},
	{"PGUP", int(K_PGUP)},
	{"HOME", int(K_HOME)},
	{"END", int(K_END)},

	{"MOUSE1", int(K_MOUSE1)},
	{"MOUSE2", int(K_MOUSE2)},
	{"MOUSE3", int(K_MOUSE3)},
	{"MOUSE4", int(K_MOUSE4)},
	{"MOUSE5", int(K_MOUSE5)},
	{"MOUSE6", int(K_MOUSE6)},
	{"MOUSE7", int(K_MOUSE7)},
	{"MOUSE8", int(K_MOUSE8)},
	{"MOUSE9", int(K_MOUSE9)},
	{"MOUSE10", int(K_MOUSE10)},

	{"JOY1", int(K_JOY1)},
	{"JOY2", int(K_JOY2)},
	{"JOY3", int(K_JOY3)},
	{"JOY4", int(K_JOY4)},

	{"AUX1", int(K_AUX1)},
	{"AUX2", int(K_AUX2)},
	{"AUX3", int(K_AUX3)},
	{"AUX4", int(K_AUX4)},
	{"AUX5", int(K_AUX5)},
	{"AUX6", int(K_AUX6)},
	{"AUX7", int(K_AUX7)},
	{"AUX8", int(K_AUX8)},
	{"AUX9", int(K_AUX9)},
	{"AUX10", int(K_AUX10)},
	{"AUX11", int(K_AUX11)},
	{"AUX12", int(K_AUX12)},
	{"AUX13", int(K_AUX13)},
	{"AUX14", int(K_AUX14)},
	{"AUX15", int(K_AUX15)},
	{"AUX16", int(K_AUX16)},
	{"AUX17", int(K_AUX17)},
	{"AUX18", int(K_AUX18)},
	{"AUX19", int(K_AUX19)},
	{"AUX20", int(K_AUX20)},
	{"AUX21", int(K_AUX21)},
	{"AUX22", int(K_AUX22)},
	{"AUX23", int(K_AUX23)},
	{"AUX24", int(K_AUX24)},
	{"AUX25", int(K_AUX25)},
	{"AUX26", int(K_AUX26)},
	{"AUX27", int(K_AUX27)},
	{"AUX28", int(K_AUX28)},
	{"AUX29", int(K_AUX29)},
	{"AUX30", int(K_AUX30)},
	{"AUX31", int(K_AUX31)},
	{"AUX32", int(K_AUX32)},

	{"MWHEELUP", int(K_MWHEELUP)},
	{"MWHEELDOWN", int(K_MWHEELDOWN)},

	{"SEMICOLON", ';'}, // because a raw semicolon separates commands
}

// the edit line as a C string
func editLine() string {
	return cstr(key_lines[edit_line][:])
}

// Q_strncpyz into the edit line at pos with a buffer of size bytes left
func editLinePut(pos int, s string, size int) {
	if size <= 0 || pos >= MAXCMDLINE {
		return
	}
	s = strlcpy(s, size)
	line := key_lines[edit_line][pos:]
	n := copy(line, s)
	if n < len(line) {
		line[n] = 0
	}
}

/*
==============================================================================
			LINE TYPING INTO THE CONSOLE
==============================================================================
*/

func CheckForCommand() bool {
	command := strlcpy(editLine()[1:], 256)
	s := 0
	for s < len(command) && int8(command[s]) > ' ' {
		s++
	}
	command = command[:s]

	return Cvar_FindVar(command) != nil || Cmd_FindCommand(command) != nil || Cmd_FindAlias(command) != nil || Cmd_IsLegacyCommand(command)
}

//===================================================================
//  Advanced command completion

var compl_common string
var compl_clen int
var compl_len int

func FindCommonSubString(s string) {
	if compl_clen == 0 {
		compl_common = strlcpy(s, 64)
		compl_clen = len(compl_common)
	} else {
		for compl_clen > compl_len && Q_strncasecmp(s, compl_common, uint(compl_clen)) != 0 {
			compl_clen--
		}
	}
}

func CompleteCommand() {
	var cmd string
	line := &key_lines[edit_line]

	if key_linepos < 2 || isspace(line[key_linepos-1]) {
		return
	}

	start := key_linepos - 1
	for start >= 1 && !isspace(line[start]) {
		start--
	}
	if start == 0 {
		start = 1
	}
	if isspace(line[start]) {
		start++
	}
	end := key_linepos - 1

	size := min(end-start+1, MAXCMDLINE-1)
	token := string(line[start : start+size])

	s := token
	if at(s, 0) == '\\' || at(s, 0) == '/' {
		s = s[1:]
		start++
	}
	if start > end {
		return
	}

	compl_len = len(s)
	compl_clen = 0

	c := Cmd_CompleteCountPossible(s)
	a := Cmd_AliasCompleteCountPossible(s)
	v := Cvar_CompleteCountPossible(s)

	if c+a+v > 1 {
		Com_Printf("\n")

		if c != 0 {
			Com_Printf("\x02" + "Commands:\n")
			tp := TablePrint_Begin(1)
			for cmd := cmd_functions; cmd != nil; cmd = cmd.next {
				if Q_strncasecmp(s, cmd.name, uint(compl_len)) == 0 {
					TablePrint_AddItem(tp, cmd.name)
					FindCommonSubString(cmd.name)
				}
			}

			TablePrint_End(tp)
		}

		if v != 0 {
			Com_Printf("\x02" + "Variables:\n")
			tp := TablePrint_Begin(1)
			for v := cvar_vars; v != nil; v = v.next {
				if Q_strncasecmp(s, v.name, uint(compl_len)) == 0 {
					TablePrint_AddItem(tp, v.name)
					FindCommonSubString(v.name)
				}
			}

			TablePrint_End(tp)
		}

		if a != 0 {
			Com_Printf("\x02" + "Aliases:\n")
			tp := TablePrint_Begin(1)
			for alias := cmd_alias; alias != nil; alias = alias.next {
				if Q_strncasecmp(s, alias.name, uint(compl_len)) == 0 {
					TablePrint_AddItem(tp, alias.name)
					FindCommonSubString(alias.name)
				}
			}

			TablePrint_End(tp)
		}
	}

	if c+a+v == 1 {
		cmd = Cmd_CompleteCommand(s)
		if cmd == "" {
			cmd = Cvar_CompleteVariable(s)
		}
		if cmd == "" {
			return // this should never happen
		}
	} else if compl_clen != 0 {
		cmd = compl_common[:compl_clen]
	} else {
		CompleteName()
		return
	}
	diff_len := len(cmd) - (end - start + 1)
	temp := strlcpy(cstr(line[end+1:]), MAXCMDLINE)
	editLinePut(end+1+diff_len, temp, MAXCMDLINE-(end+1+diff_len))
	for i := 0; start+i < MAXCMDLINE && i < len(cmd); i++ {
		line[start+i] = cmd[i]
	}
	key_linepos += diff_len
	line[min(key_linepos+len(temp), MAXCMDLINE-1)] = 0
	if start == 1 && key_linepos+len(temp) < MAXCMDLINE-1 {
		for i := key_linepos + len(temp); i > 0; i-- {
			line[i+1] = line[i]
		}
		line[1] = '/'
		key_linepos++
	}
	if c+a+v == 1 && line[key_linepos] == 0 && key_linepos < MAXCMDLINE-1 {
		line[key_linepos] = ' '
		key_linepos++
		line[key_linepos] = 0
	}
}

const disallowed = "\n\f\\/\" ;"

func RemoveColors(name string) string {
	if name == "" {
		return name
	}

	b := []byte(name)
	for i := range b {
		b[i] = readablechars[b[i]] & 127
		if strings.IndexByte(disallowed, b[i]) >= 0 {
			b[i] = '_'
		}
	}
	// get rid of whitespace
	s := strings.TrimLeft(string(b), "_")
	s = strings.TrimRight(s, "_")

	if s == "" {
		s = "_"
	}
	return s
}

func FindBestNick(s string) int {
	bestplayer, best := -1, -1

	for i := 0; i < MAX_CLIENTS; i++ {
		if cl.players[i].name != "" {
			name := strlcpy(cl.players[i].name, MAX_SCOREBOARDNAME)
			name = RemoveColors(name)
			b := []byte(name)
			for j := range b {
				b[j] = tolower(b[j])
			}
			name = string(b)
			if match := strings.Index(name, s); match >= 0 && (best == -1 || match < best) {
				best = match
				bestplayer = i
			}
		}
	}
	return bestplayer
}

func CompleteName() {
	line := &key_lines[edit_line]

	q := key_linepos
	p := q
	for {
		p--
		if p < 1 {
			break
		}
		c := line[p]
		if !((c >= 32 && c <= 127) && strings.IndexByte(disallowed, c) < 0) {
			break
		}
	}
	p++
	if q-p <= 0 {
		return
	}

	s := string(line[p:q])

	best := FindBestNick(s)
	if best >= 0 {
		t := strlcpy(cl.players[best].name, MAXCMDLINE)

		for i := 0; i < len(t); i++ {
			if (127 & t[i]) == ' ' {
				if k := len(t); k < MAXCMDLINE-2 {
					t = "\"" + t + "\""
				}
				break
			}
		}
		diff := len(t) - len(s)

		tail := cstr(line[q:])
		editLinePut(q+diff, tail, MAXCMDLINE-(q+diff))
		copy(line[p:], t)
		key_linepos += diff
		if line[key_linepos] == 0 && key_linepos < MAXCMDLINE-1 {
			line[key_linepos] = ' '
			key_linepos++
			line[key_linepos] = 0
		}
	}
}

//===================================================================

func AdjustConsoleHeight(delta int) {
	if cls.state != ca_active && cl.intermission == 0 {
		return
	}
	height := int((scr_consize.value*float32(vid.conheight) + float32(delta) + 5) / 10)
	height *= 10
	if delta < 0 && height < 30 {
		height = 30
	}
	if delta > 0 && height > int(vid.conheight)-10 {
		height = int(vid.conheight) - 10
	}
	Cvar_SetValue(&scr_consize, float32(height)/float32(vid.conheight))
}

// Interactive line editing and console scrollback
func Key_Console(key int) {
	line := &key_lines[edit_line]
	lineLen := func() int { return len(cstr(line[:])) }

	switch keynum_t(key) {
	case K_ENTER:
		lf := true
		// backslash text are commands
		if line[1] == '/' && line[2] == '/' {
			lf = false
		} else if (keydown[K_CTRL] || keydown[K_SHIFT]) && cls.state >= ca_connected {
			if keydown[K_CTRL] {
				Cbuf_AddText("say_team ")
			} else {
				Cbuf_AddText("say ")
			}
			Cbuf_AddText(cstr(line[1:]))
		} else if line[1] == '\\' || line[1] == '/' {
			Cbuf_AddText(cstr(line[2:])) // skip the ]/
		} else if cl_chatmode.value != 1 && CheckForCommand() {
			Cbuf_AddText(cstr(line[1:])) // valid command
		} else if (cls.state >= ca_connected && cl_chatmode.value == 2) || cl_chatmode.value == 1 {
			if cls.state < ca_connected { // can happen if cl_chatmode is 1
				lf = false // drop the whole line
			} else {
				Cbuf_AddText("say ")
				Cbuf_AddText(cstr(line[1:]))
			}
		} else {
			Cbuf_AddText(cstr(line[1:])) // skip the ]
		}

		if lf {
			Cbuf_AddText("\n")
		}
		Com_Printf("%s\n", cstr(line[:]))
		edit_line = (edit_line + 1) & 31
		history_line = edit_line
		key_lines[edit_line][0] = ']'
		key_lines[edit_line][1] = 0
		key_linepos = 1
		if cls.state == ca_disconnected {
			SCR_UpdateScreen() // force an update, because the command
			// may take some time
		}
		return

	case K_TAB:
		// command completion
		if keydown[K_CTRL] {
			CompleteName()
		} else {
			CompleteCommand()
		}
		return

	case K_BACKSPACE:
		if key_linepos > 1 {
			copy(line[key_linepos-1:], line[key_linepos:])
			line[MAXCMDLINE-1] = 0
			key_linepos--
		}
		return

	case K_DEL:
		if key_linepos < lineLen() {
			copy(line[key_linepos:], line[key_linepos+1:])
			line[MAXCMDLINE-1] = 0
		}
		return

	case K_RIGHTARROW:
		if keydown[K_CTRL] {
			// word right
			i := lineLen()
			for key_linepos < i && line[key_linepos] != ' ' {
				key_linepos++
			}
			for key_linepos < i && line[key_linepos] == ' ' {
				key_linepos++
			}
			return
		}
		if key_linepos < lineLen() {
			key_linepos++
		}
		return

	case K_LEFTARROW:
		if keydown[K_CTRL] {
			// word left
			for key_linepos > 1 && line[key_linepos-1] == ' ' {
				key_linepos--
			}
			for key_linepos > 1 && line[key_linepos-1] != ' ' {
				key_linepos--
			}
			return
		}
		if key_linepos > 1 {
			key_linepos--
		}
		return

	case K_UPARROW:
		if keydown[K_CTRL] {
			AdjustConsoleHeight(-10)
			return
		}
		for {
			history_line = (history_line - 1) & 31
			if !(history_line != edit_line && key_lines[history_line][1] == 0) {
				break
			}
		}
		if history_line == edit_line {
			history_line = (edit_line + 1) & 31
		}
		key_lines[edit_line] = key_lines[history_line]
		key_linepos = lineLen()
		return

	case K_DOWNARROW:
		if keydown[K_CTRL] {
			AdjustConsoleHeight(10)
			return
		}
		if history_line == edit_line {
			return
		}
		for {
			history_line = (history_line + 1) & 31
			if !(history_line != edit_line && key_lines[history_line][1] == 0) {
				break
			}
		}

		if history_line == edit_line {
			line[0] = ']'
			line[1] = 0
			key_linepos = 1
		} else {
			key_lines[edit_line] = key_lines[history_line]
			key_linepos = lineLen()
		}
		return

	case K_PGUP, K_MWHEELUP:
		if keydown[K_CTRL] && keynum_t(key) == K_PGUP {
			Con_ScrollUp(uint32((int(scr_conlines) - 40) >> 3))
		} else {
			Con_ScrollUp(2)
		}
		return

	case K_MWHEELDOWN, K_PGDN:
		if keydown[K_CTRL] && keynum_t(key) == K_PGDN {
			Con_ScrollDown(uint32((int(scr_conlines) - 40) >> 3))
		} else {
			Con_ScrollDown(2)
		}
		return

	case K_HOME:
		if keydown[K_CTRL] {
			Con_Home()
		} else {
			key_linepos = 1
		}
		return

	case K_END:
		if keydown[K_CTRL] {
			Con_End()
		} else {
			key_linepos = lineLen()
		}
		return
	}
	if ((key == 'V' || key == 'v') && keydown[K_CTRL]) || keynum_t(key) == K_PASTE {
		if clipText := VID_GetClipboardText(); clipText != "" {
			length := len(clipText)
			if length+lineLen() > MAXCMDLINE-1 {
				length = MAXCMDLINE - 1 - lineLen()
			}
			if length > 0 { // insert the string
				tail := cstr(line[key_linepos:])
				copy(line[key_linepos:], clipText[:length])
				editLinePut(key_linepos+length, tail, MAXCMDLINE-(key_linepos+length))
				key_linepos += length
			}

			VID_FreeClipboardText(clipText)
		}
		return
	}

	if key == 'u' && keydown[K_CTRL] {
		if key_linepos > 1 {
			tail := cstr(line[key_linepos:])
			editLinePut(1, tail, MAXCMDLINE-1)
			key_linepos = 1
		}
		return
	}

	if key < 32 || key > 127 {
		return // non printable
	}

	if keydown[K_CTRL] {
		if key >= '0' && key <= '9' {
			key = key - '0' + 0x12 // yellow number
		} else {
			switch key {
			case '[':
				key = 0x10
			case ']':
				key = 0x11
			case 'g':
				key = 0x86
			case 'r':
				key = 0x87
			case 'y':
				key = 0x88
			case 'b':
				key = 0x89
			case '(':
				key = 0x80
			case '=':
				key = 0x81
			case ')':
				key = 0x82
			case 'a':
				key = 0x83
			case '<':
				key = 0x1d
			case '-':
				key = 0x1e
			case '>':
				key = 0x1f
			case ',':
				key = 0x1c
			case '.':
				key = 0x9c
			case 'B':
				key = 0x8b
			case 'C':
				key = 0x8d
			}
		}
	}

	if keydown[K_ALT] {
		key |= 128 // red char
	}

	i := lineLen()
	if i >= MAXCMDLINE-1 {
		return
	}

	// This also moves the ending \0
	copy(line[key_linepos+1:], line[key_linepos:i+1])
	line[key_linepos] = byte(key)
	key_linepos++
}

//============================================================================

var chat_team bool
var chat_buffer [MAXCMDLINE]byte
var chat_linepos int

func Key_Message(key int) {
	chatLen := func() int { return len(cstr(chat_buffer[:])) }

	switch keynum_t(key) {
	case K_ENTER:
		if chat_buffer[0] != 0 {
			if chat_team {
				Cbuf_AddText("say_team \"")
			} else {
				Cbuf_AddText("say \"")
			}
			Cbuf_AddText(cstr(chat_buffer[:]))
			Cbuf_AddText("\"\n")
		}
		key_dest = key_game
		chat_linepos = 0
		chat_buffer[0] = 0
		return

	case K_ESCAPE:
		key_dest = key_game
		chat_buffer[0] = 0
		chat_linepos = 0
		return

	case K_HOME:
		chat_linepos = 0
		return

	case K_END:
		chat_linepos = chatLen()
		return

	case K_LEFTARROW:
		if chat_linepos > 0 {
			chat_linepos--
		}
		return

	case K_RIGHTARROW:
		if chat_linepos < chatLen() {
			chat_linepos++
		}
		return

	case K_BACKSPACE:
		if chat_linepos > 0 {
			copy(chat_buffer[chat_linepos-1:], chat_buffer[chat_linepos:])
			chat_buffer[MAXCMDLINE-1] = 0
			chat_linepos--
		}
		return

	case K_DEL:
		if chat_buffer[chat_linepos] != 0 {
			copy(chat_buffer[chat_linepos:], chat_buffer[chat_linepos+1:])
			chat_buffer[MAXCMDLINE-1] = 0
		}
		return
	}

	// pastes into the console line, as the C code does
	if (key == 'V' || key == 'v') && keydown[K_CTRL] {
		if clipText := VID_GetClipboardText(); clipText != "" {
			line := &key_lines[edit_line]
			lineLen := len(cstr(line[:]))
			length := len(clipText)
			if length+lineLen > MAXCMDLINE-1 {
				length = MAXCMDLINE - 1 - lineLen
			}
			if length > 0 { // insert the string
				tail := cstr(line[key_linepos:])
				copy(line[key_linepos:], clipText[:length])
				editLinePut(key_linepos+length, tail, MAXCMDLINE-(key_linepos+length))
				key_linepos += length
			}

			VID_FreeClipboardText(clipText)
		}
		return
	}

	if key < 32 || key > 127 {
		return // non printable
	}

	length := chatLen()

	if length >= MAXCMDLINE-1 {
		return // all full
	}

	// This also moves the ending \0
	copy(chat_buffer[chat_linepos+1:], chat_buffer[chat_linepos:length+1])
	chat_buffer[chat_linepos] = byte(key)
	chat_linepos++
}

//============================================================================

// Returns a key number to be used to index keybindings[] by looking at the given string.
// Single ascii characters return themselves, while the K_* names are matched up.
func Key_StringToKeynum(str string) int {
	if str == "" {
		return -1
	}
	if len(str) == 1 {
		return int(str[0])
	}

	for i := range keynames {
		if Q_strcasecmp(str, keynames[i].name) == 0 {
			return keynames[i].keynum
		}
	}
	return -1
}

// Returns a string (either a single ascii char, or a K_* name) for the given keynum.
// FIXME: handle quote special (general escape sequence?)
func Key_KeynumToString(keynum int) string {
	if keynum == -1 {
		return "<KEY NOT FOUND>"
	}
	if keynum > 32 && keynum < 127 { // printable ascii
		return string([]byte{byte(keynum)})
	}

	for i := range keynames {
		if keynum == keynames[i].keynum {
			return keynames[i].name
		}
	}

	return "<UNKNOWN KEYNUM>"
}

func Key_SetBinding(keynum int, binding string) {
	if keynum == -1 {
		return
	}

	if keynum == int(K_CTRL) || keynum == int(K_ALT) || keynum == int(K_SHIFT) || keynum == int(K_WIN) {
		Key_SetBinding(keynum+1, binding)
		Key_SetBinding(keynum+2, binding)
		return
	}

	b := binding
	keybindings[keynum] = &b
}

func Key_Unbind(keynum int) {
	if keynum == -1 {
		return
	}

	if keynum == int(K_CTRL) || keynum == int(K_ALT) || keynum == int(K_SHIFT) || keynum == int(K_WIN) {
		Key_Unbind(keynum + 1)
		Key_Unbind(keynum + 2)
		return
	}

	keybindings[keynum] = nil
}

func Key_Unbind_f() {
	if Cmd_Argc() != 2 {
		Com_Printf("Usage:  %s <key> : remove commands from a key\n", Cmd_Argv(0))
		return
	}

	b := Key_StringToKeynum(Cmd_Argv(1))
	if b == -1 {
		Com_Printf("\"%s\" isn't a valid key\n", Cmd_Argv(1))
		return
	}

	Key_Unbind(b)
}

func Key_Unbindall_f() {
	for i := 0; i < 256; i++ {
		if keybindings[i] != nil {
			Key_Unbind(i)
		}
	}
}

func Key_PrintBindInfo(keynum int, keyname string) {
	if keyname == "" {
		keyname = Key_KeynumToString(keynum)
	}

	if keynum == -1 {
		Com_Printf("\"%s\" isn't a valid key\n", keyname)
		return
	}

	if keybindings[keynum] != nil {
		Com_Printf("\"%s\" = \"%s\"\n", keyname, *keybindings[keynum])
	} else {
		Com_Printf("\"%s\" is not bound\n", keyname)
	}
}

// checks if LCTRL and RCTRL are both bound and bound to the same thing
func Key_IsLeftRightSameBind(b int) bool {
	if b < 0 || b >= 256 {
		return false
	}

	return (b == int(K_CTRL) || b == int(K_ALT) || b == int(K_SHIFT) || b == int(K_WIN)) &&
		(keybindings[b+1] != nil && keybindings[b+2] != nil && *keybindings[b+1] == *keybindings[b+2])
}

func Key_Bind_f() {
	c := Cmd_Argc()

	if c < 2 {
		Com_Printf("Usage: %s <key> [command] : attach a command to a key\n", Cmd_Argv(0))
		return
	}
	b := Key_StringToKeynum(Cmd_Argv(1))
	if b == -1 {
		Com_Printf("\"%s\" isn't a valid key\n", Cmd_Argv(1))
		return
	}
	if c == 2 {
		if (b == int(K_CTRL) || b == int(K_ALT) || b == int(K_SHIFT) || b == int(K_WIN)) && (keybindings[b+1] != nil || keybindings[b+2] != nil) {
			if keybindings[b+1] != nil && keybindings[b+2] != nil && *keybindings[b+1] == *keybindings[b+2] {
				Com_Printf("\"%s\" = \"%s\"\n", Cmd_Argv(1), *keybindings[b+1])
			} else {
				Key_PrintBindInfo(b+1, "")
				Key_PrintBindInfo(b+2, "")
			}
		} else {
			//		and the following should print "ctrl (etc) is not bound" since K_CTRL cannot be bound
			Key_PrintBindInfo(b, Cmd_Argv(1))
		}
		return
	}

	// copy the rest of the command line
	Key_SetBinding(b, Cmd_MakeArgs(2))
}

func Key_BindList_f() {
	for i := 0; i < 256; i++ {
		if Key_IsLeftRightSameBind(i) {
			Com_Printf("%s \"%s\"\n", Key_KeynumToString(i), *keybindings[i+1])
			i += 2
		} else {
			if keybindings[i] != nil {
				Com_Printf("%s \"%s\"\n", Key_KeynumToString(i), *keybindings[i])
			}
		}
	}
}

// Writes lines containing "bind key value"
func Key_WriteBindings(f io.Writer) {
	for i := 0; i < 256; i++ {
		leftright := Key_IsLeftRightSameBind(i)
		if leftright || keybindings[i] != nil {
			if i == ';' {
				fmt.Fprintf(f, "bind \";\" \"%s\"\n", *keybindings[i])
			} else {
				b := keybindings[i]
				if leftright {
					b = keybindings[i+1]
				}
				fmt.Fprintf(f, "bind %s \"%s\"\n", Key_KeynumToString(i), *b)
			}

			if leftright {
				i += 2
			}
		}
	}
}

func Key_CvarInit() {
	// register our functions
	Cmd_AddCommand("bindlist", Key_BindList_f)
	Cmd_AddCommand("bind", Key_Bind_f)
	Cmd_AddCommand("unbind", Key_Unbind_f)
	Cmd_AddCommand("unbindall", Key_Unbindall_f)

	Cvar_SetCurrentGroup(CVAR_GROUP_CONSOLE)
	Cvar_Register(&cl_chatmode)

	Cvar_SetCurrentGroup(CVAR_GROUP_INPUT_MOUSE)
	Cvar_Register(&m_wheel_factor)

	Cvar_ResetCurrentGroup()
}

func Key_Init() {
	for i := 0; i < 32; i++ {
		key_lines[i][0] = ']'
		key_lines[i][1] = 0
	}
	key_linepos = 1

	// init ascii characters in console mode
	for i := 32; i < 128; i++ {
		consolekeys[i] = true
	}
	for _, k := range []keynum_t{K_ENTER, K_TAB, K_LEFTARROW, K_RIGHTARROW, K_UPARROW, K_DOWNARROW, K_BACKSPACE,
		K_INS, K_DEL, K_HOME, K_END, K_PGUP, K_PGDN, K_ALT, K_LALT, K_RALT, K_CTRL, K_LCTRL, K_RCTRL,
		K_SHIFT, K_LSHIFT, K_RSHIFT, K_MWHEELUP, K_MWHEELDOWN, K_PASTE} {
		consolekeys[k] = true
	}
	consolekeys['`'] = false
	consolekeys['~'] = false

	for i := 0; i < 256; i++ {
		keyshift[i] = i
	}
	for i := 'a'; i <= 'z'; i++ {
		keyshift[i] = int(i - 'a' + 'A')
	}
	keyshift['1'] = '!'
	keyshift['2'] = '@'
	keyshift['3'] = '#'
	keyshift['4'] = '$'
	keyshift['5'] = '%'
	keyshift['6'] = '^'
	keyshift['7'] = '&'
	keyshift['8'] = '*'
	keyshift['9'] = '('
	keyshift['0'] = ')'
	keyshift['-'] = '_'
	keyshift['='] = '+'
	keyshift[','] = '<'
	keyshift['.'] = '>'
	keyshift['/'] = '?'
	keyshift[';'] = ':'
	keyshift['\''] = '"'
	keyshift['['] = '{'
	keyshift[']'] = '}'
	keyshift['`'] = '~'
	keyshift['\\'] = '|'

	menubound[K_ESCAPE] = true
	for i := 0; i < 12; i++ {
		menubound[int(K_F1)+i] = true
	}
}

func ismovementcommand(s string) bool {
	/* World's ugliest hack */

	if at(s, 0) != '+' && at(s, 0) != '-' {
		return false
	}

	return s[1:] == "forward" || s[1:] == "back" || s[1:] == "moveleft" || s[1:] == "moveright"
}

var mwheelstate int

// Called by the system between frames for both key up and key down events
func Key_Event(key int, down bool) {
	if key == int(K_MWHEELDOWN) || key == int(K_MWHEELUP) {
		mwheelfactor := int(m_wheel_factor.value)
		if mwheelfactor > 0 {
			if key == int(K_MWHEELDOWN) {
				if down {
					mwheelstate--
				}

				if mwheelstate > -mwheelfactor {
					return
				}
			}

			if key == int(K_MWHEELUP) {
				if down {
					mwheelstate++
				}

				if mwheelstate < mwheelfactor {
					return
				}
			}

			if !down {
				mwheelstate = 0
			}
		}
	}

	if key == int(K_LALT) || key == int(K_RALT) {
		Key_Event(int(K_ALT), down)
	} else if key == int(K_LCTRL) || key == int(K_RCTRL) {
		Key_Event(int(K_CTRL), down)
	} else if key == int(K_LSHIFT) || key == int(K_RSHIFT) {
		Key_Event(int(K_SHIFT), down)
	} else if key == int(K_LWIN) || key == int(K_RWIN) {
		Key_Event(int(K_WIN), down)
	}

	keydown[key] = down

	if !down {
		key_repeats[key] = 0
	}

	// update auto-repeat status
	if down {
		key_repeats[key]++
		if key_repeats[key] > 1 {
			if (key != int(K_BACKSPACE) && key != int(K_DEL) &&
				key != int(K_LEFTARROW) && key != int(K_RIGHTARROW) &&
				key != int(K_UPARROW) && key != int(K_DOWNARROW) &&
				key != int(K_PGUP) && key != int(K_PGDN) && (key < 32 || key > 126 || key == '`')) ||
				(key_dest == key_game && cls.state == ca_active) {
				return // ignore most autorepeats
			}
		}
	}

	// handle escape specialy, so the user can never unbind it
	if key == int(K_ESCAPE) {
		if !down {
			return
		}

		switch key_dest {
		case key_message:
			Key_Message(key)

		case key_menu:
			M_Keydown(key)

		case key_game:
			M_ToggleMenu_f()

		case key_console:
			if !SCR_NEED_CONSOLE_BACKGROUND() {
				ToggleConsole_f()
			} else {
				M_ToggleMenu_f()
			}

		case key_serverbrowser:
			SB_Key(key)

		case key_lua:
			Lua_Key(key)
		}

		return
	}

	// key up events only generate commands if the game key binding is a button command (leading + sign).
	// These will occur even in console mode, to keep the character from continuing an action started before a
	// console switch.  Button commands include the kenum as a parameter, so multiple downs can be matched with ups
	if !down {
		kb := keybindings[key]
		if kb != nil && at(*kb, 0) == '+' && keyactive[key] {
			var cmd string
			if ismovementcommand(*kb) {
				cmd = fmt.Sprintf("-%s %d %d\n", (*kb)[1:], key, movementkey)
			} else {
				cmd = fmt.Sprintf("-%s %d\n", (*kb)[1:], key)
			}

			Cbuf_AddText(strlcpy(cmd, 1024))
			keyactive[key] = false
		}
		if keyshift[key] != key {
			kb = keybindings[keyshift[key]]
			if kb != nil && at(*kb, 0) == '+' && keyactive[keyshift[key]] {
				var cmd string
				if ismovementcommand(*kb) {
					cmd = fmt.Sprintf("-%s %d %d\n", (*kb)[1:], key, movementkey)
				} else {
					cmd = fmt.Sprintf("-%s %d\n", (*kb)[1:], key)
				}

				Cbuf_AddText(strlcpy(cmd, 1024))
				keyactive[keyshift[key]] = false
			}
		}

		return
	}

	// if not a consolekey, send to the interpreter no matter what mode is
	if (key_dest == key_menu && menubound[key]) ||
		((key_dest == key_console || key_dest == key_message) && !consolekeys[key]) ||
		(key_dest == key_game && (cls.state == ca_active || !consolekeys[key])) {
		kb := keybindings[key]
		if kb != nil {
			if at(*kb, 0) == '+' {
				// button commands add keynum as a parm
				var cmd string
				if ismovementcommand(*kb) {
					cmd = fmt.Sprintf("%s %d %d\n", *kb, key, movementkey)
				} else {
					cmd = fmt.Sprintf("%s %d\n", *kb, key)
				}

				Cbuf_AddText(strlcpy(cmd, 1024))
				keyactive[key] = true
			} else {
				Cbuf_AddText(*kb)
				Cbuf_AddText("\n")
			}
		}

		return
	}

	if !down {
		return // other systems only care about key down events
	}

	if keydown[K_SHIFT] {
		key = keyshift[key]
	}

	switch key_dest {
	case key_message:
		Key_Message(key)

	case key_menu:
		M_Keydown(key)

	case key_game, key_console:
		Key_Console(key)

	case key_serverbrowser:
		SB_Key(key)

	case key_lua:
		Lua_Key(key)
	}
}

func Key_ClearStates() {
	for i := 0; i < 256; i++ {
		if keydown[i] {
			Key_Event(i, false)
		}

		key_repeats[i] = 0
	}
}

func Key_ClearTyping() {
	key_lines[edit_line][1] = 0 // clear any typing
	key_linepos = 1
}

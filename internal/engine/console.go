package engine

import (
	"fmt"
	"strings"
	"unsafe"
)

var con_notifylines = cvar_t{name: "con_notifylines", string: "9"}
var con_notifytime = cvar_t{name: "con_notifytime", string: "120"}
var con_parsecolors = cvar_t{name: "con_parsecolors", string: "1"}

// The callback reads con_parsecolors through Con_Relayout, which Go rejects
// as an initialization cycle in the literal above.
func init() {
	con_parsecolors.OnChange = con_parsecolors_callback
}

// The console is a ring of NUL-terminated lines; all indices are unsigned
// 32 bit like in C, so wrap-around arithmetic matches.
var (
	conbuf           []byte
	contail          uint32
	consize          uint32
	partiallinestart uint32

	lines            []uint32
	linestartcolours []uint16
	maxlines         uint32 // Must be a power of 2
	firstline        uint32
	lastline         uint32
	displayline      uint32
	textcolumns      uint32
	scrollupmarker   string
	editlinepos      uint32
	stitchbuffer     []byte // linear copy of the line that crosses the end of the console buffer, if any
	con_cursorspeed  = float32(4)
	suppressed       bool
)

const MAXNOTIFYLINES = 16

var notifytimes [MAXNOTIFYLINES]uint64
var notifystart uint32

const MAXCMDLINE = 256

func Con_CopyToBuffer(buf []byte) {
	size := uint32(len(buf))
	if contail+size <= consize {
		copy(conbuf[contail:], buf)
		contail += size
		if contail == consize {
			contail = 0
		}
	} else {
		copy(conbuf[contail:], buf[:consize-contail])
		copy(conbuf, buf[consize-contail:])
		contail = size - (consize - contail)
	}
}

func Con_BufferStringLength(offset uint32) uint32 {
	i := offset
	for i < consize && conbuf[i] != 0 {
		i++
	}
	n := i - offset
	if i == consize {
		i = 0
		for i < offset && conbuf[i] != 0 {
			i++
		}
		n += i
	}
	return n
}

// The stitched line as a C string.
func conStitched() string {
	return cstr(stitchbuffer)
}

func Con_BufferColouredStringLength(offset uint32) uint32 {
	n := Con_BufferStringLength(offset)
	if offset+n > consize {
		if stitchbuffer != nil {
			s := conStitched()
			return uint32(Colored_String_Length(s[uint32(len(s))-n:]))
		}
		return 0
	}
	return uint32(Colored_String_Length(string(conbuf[offset : offset+n])))
}

func Con_BufferColouredStringLengthOffset(offset uint32, maxlen uint32, lastcolour *uint16) uint32 {
	n := Con_BufferStringLength(offset)
	if offset+n > consize {
		if stitchbuffer != nil {
			s := conStitched()
			return uint32(Colored_String_Offset(s[uint32(len(s))-n:], uint(maxlen), lastcolour))
		}
		return n
	}
	return uint32(Colored_String_Offset(string(conbuf[offset:offset+n]), uint(maxlen), lastcolour))
}

func Con_BufferFindLinebreak(offset uint32, max uint32, lastcolour *uint16) uint32 {
	var i uint32

	if con_parsecolors.value != 0 {
		max = Con_BufferColouredStringLength(offset)
		if max <= textcolumns {
			return Con_BufferStringLength(offset)
		}
		i = Con_BufferColouredStringLengthOffset(offset, textcolumns, lastcolour)
	} else {
		if max <= textcolumns {
			return max
		}
		i = textcolumns
		*lastcolour = 0x0fff
	}

	for i > 0 && conbuf[(offset+i)%consize] != ' ' && conbuf[(offset+i)%consize] != (' '|0x80) {
		i--
	}

	if i == 0 {
		return textcolumns
	}

	for conbuf[(offset+i)%consize] == ' ' || conbuf[(offset+i)%consize] == (' '|0x80) {
		i++
	}

	return i
}

func Con_ExpandMaxLines() bool {
	newlines := make([]uint32, maxlines*2)
	newlinestartcolours := make([]uint16, maxlines*2)

	i := uint32(0)
	j := firstline
	for j != (lastline+1)%maxlines {
		newlines[i] = lines[j]
		newlinestartcolours[i] = linestartcolours[j]
		i++
		j++
		j %= maxlines
	}

	displayline = (displayline - firstline) % maxlines
	lastline = (lastline - firstline) % maxlines
	firstline = 0

	lines = newlines
	linestartcolours = newlinestartcolours
	maxlines *= 2

	return true
}

func Con_LayoutLine(offset uint32) {
	var lastcolour uint16
	var linestartcolour uint16 = 0x0fff

	i := Con_BufferStringLength(offset)

	for {
		j := Con_BufferFindLinebreak(offset, i, &lastcolour)

		if (lastline+2)%maxlines == firstline {
			if !Con_ExpandMaxLines() {
				firstline++
				firstline = firstline % maxlines
			}
		}

		if displayline == lastline {
			displayline++
			displayline %= maxlines
		}

		lastline++
		lastline %= maxlines

		lines[lastline] = offset
		linestartcolours[lastline] = linestartcolour

		notifytimes[notifystart] = Sys_IntTime()
		notifystart++
		notifystart %= MAXNOTIFYLINES

		offset += j
		offset %= consize

		i -= j

		linestartcolour = lastcolour

		if i == 0 {
			break
		}
	}
}

func Con_Relayout() {
	if firstline != (lastline+1)%maxlines {
		i := lines[firstline]

		firstline = 0
		lastline = maxlines - 1
		displayline = lastline

		for i != contail {
			Con_LayoutLine(i)
			i += Con_BufferStringLength(i) + 1
			i %= consize
		}
	}
}

func Con_Clear() {
	contail = 0
	partiallinestart = 0
	firstline = 0
	lastline = maxlines - 1
	displayline = lastline
}

func Con_Init() {
	consize = 1024 * 256
	conbuf = make([]byte, consize)
	lines = make([]uint32, 512)
	linestartcolours = make([]uint16, 512)
	maxlines = 512
	lastline = maxlines - 1
	displayline = lastline
	textcolumns = 65536
}

func Con_Shutdown() {
	conbuf = nil
	lines = nil
	scrollupmarker = ""
}

func Con_CvarInit() {
	Cvar_SetCurrentGroup(CVAR_GROUP_CONSOLE)
	Cvar_Register(&con_notifylines)
	Cvar_Register(&con_notifytime)
	Cvar_Register(&con_parsecolors)
	Cvar_ResetCurrentGroup()

	Cmd_AddCommand("clear", Con_Clear)
}

func con_parsecolors_callback(cvar *cvar_t, value string) bool {
	cvar.value = float32(atof(value))
	Con_Relayout()
	return false
}

func Con_CheckResize(pixelwidth uint32) {
	newtextcolumns := pixelwidth / 8
	if newtextcolumns > 2 {
		newtextcolumns -= 2
	} else {
		newtextcolumns = 1
	}

	if newtextcolumns != textcolumns {
		textcolumns = newtextcolumns

		marker := make([]byte, textcolumns)
		for i := range marker {
			if i%4 == 0 {
				marker[i] = '^'
			} else {
				marker[i] = ' '
			}
		}
		scrollupmarker = string(marker)

		Con_Relayout()
	}
}

func Con_DrawTextLines(y uint32, maxlinestodraw uint32, lastlinetodraw uint32) {
	if firstline == (lastline+1)%maxlines {
		return
	}

	if maxlinestodraw == 0 {
		return
	}

	if con_parsecolors.value == 0 {
		Draw_BeginTextRendering()
	}

	y += maxlinestodraw * 8

	j := lastlinetodraw
	if firstline > lastlinetodraw {
		j += maxlines
	}

	var i uint32
	if firstline+maxlinestodraw-1 <= j {
		i = j - maxlinestodraw + 1
	} else {
		i = firstline
		maxlinestodraw = j - firstline + 1
	}

	i %= maxlines

	y -= maxlinestodraw * 8

	for i != (lastline+1)%maxlines && maxlinestodraw != 0 {
		if maxlinestodraw == 1 && i != lastline {
			Draw_String(8, int(y), scrollupmarker)
			break
		}

		nextline := (i + 1) % maxlines

		linelength := Con_BufferStringLength(lines[i])
		if nextline != (lastline+1)%maxlines {
			if lines[nextline] < lines[i] {
				j = consize - lines[i] + lines[nextline]
			} else {
				j = lines[nextline] - lines[i]
			}

			if j < linelength {
				linelength = j
			}
		}

		if lines[i]+linelength > consize {
			if stitchbuffer != nil {
				if con_parsecolors.value != 0 {
					Draw_ColoredString_Length(8, int(y), conStitched(), 0, int(linelength), linestartcolours[i])
				} else {
					Draw_String_Length(8, int(y), conStitched(), int(linelength))
				}
			}
		} else {
			// read-only view, the draw functions keep nothing of it
			line := unsafe.String(&conbuf[lines[i]], linelength)
			if con_parsecolors.value != 0 {
				Draw_ColoredString_Length(8, int(y), line, 0, int(linelength), linestartcolours[i])
			} else {
				Draw_String_Length(8, int(y), line, int(linelength))
			}
		}

		y += 8
		i = nextline
		maxlinestodraw--
	}

	if con_parsecolors.value == 0 {
		Draw_EndTextRendering()
	}
}

func Con_DrawConsole(pixellines int) {
	Draw_ConsoleBackground(pixellines)

	Con_DrawTextLines(uint32(pixellines+2)%8, uint32(pixellines-22)/8, displayline)

	if key_linepos != 0 && uint32(key_linepos-1) < editlinepos {
		editlinepos = uint32(key_linepos - 1)
	} else if uint32(key_linepos) >= editlinepos+textcolumns {
		editlinepos = uint32(key_linepos) - textcolumns + 1
	}

	edit := cstr(key_lines[edit_line][editlinepos:])
	linelength := uint32(len(edit))
	if linelength > textcolumns {
		linelength = textcolumns
	}
	if linelength > 2048-1 {
		linelength = 2048 - 1
	}
	tmpline := make([]byte, linelength+1)
	copy(tmpline, edit)

	if int(Sys_DoubleTime()*float64(con_cursorspeed))&1 != 0 {
		pos := uint32(key_linepos) - editlinepos
		if tmpline[pos] == 0 {
			linelength++
		}
		tmpline[pos] = 11
	}

	Draw_String_Length(8, pixellines-22, string(tmpline[:linelength]), int(linelength))
}

func Con_DrawNotify() uint32 {
	now := Sys_IntTime()

	notifytime := uint64(float64(con_notifytime.value) * 1000000)
	maxnotifylines := uint32(con_notifylines.value)
	if maxnotifylines > MAXNOTIFYLINES {
		maxnotifylines = MAXNOTIFYLINES
	}

	i := (notifystart + (MAXNOTIFYLINES - maxnotifylines)) % MAXNOTIFYLINES
	for maxnotifylines != 0 && notifytimes[i]+notifytime < now {
		i++
		i %= MAXNOTIFYLINES
		maxnotifylines--
	}

	Con_DrawTextLines(0, maxnotifylines, lastline)

	return maxnotifylines
}

func Con_ClearNotify() {
	notifytimes = [MAXNOTIFYLINES]uint64{}
}

func Con_Suppress() {
	suppressed = true
}

func Con_Unsuppress() {
	suppressed = false
}

func Con_GetColumns() uint32 {
	return textcolumns
}

func Con_ClearBufferSpace(size uint32) {
	for firstline != (lastline+1)%maxlines && ((lines[firstline] >= contail && lines[firstline] < contail+size) || (contail+size >= consize && lines[firstline] < (contail+size)%consize)) {
		if displayline == firstline {
			displayline++
			displayline %= maxlines
		}
		lines[firstline] = 0
		firstline++
		firstline %= maxlines
	}
}

func Con_Print(txt string) {
	Log_Write(txt)

	if suppressed {
		return
	}

	if lines == nil {
		fmt.Printf("%s\n", txt)
		return
	}

	if uint32(len(txt)) >= consize {
		return
	}

	if at(txt, 0) == 1 || at(txt, 0) == 2 {
		// OMGWTFBBQ
		b := []byte(txt[1:])
		for k := range b {
			if b[k] != '\n' {
				b[k] |= 128
			}
		}
		txt = string(b)
	}

	for {
		newline := strings.IndexByte(txt, '\n')
		if newline < 0 {
			break
		}

		if cr := strings.IndexByte(txt, '\r'); cr >= 0 && cr < newline {
			txt = txt[cr+1:]
			newline -= cr + 1
		}

		i := uint32(newline + 1)

		if i+partiallinestart < consize {
			Con_ClearBufferSpace(i)

			var linebegin uint32
			if partiallinestart != 0 {
				linebegin = partiallinestart
			} else {
				linebegin = contail
			}

			Con_CopyToBuffer([]byte(txt[:i]))

			var j uint32
			if contail == 0 {
				j = consize - 1
			} else {
				j = contail - 1
			}
			conbuf[j] = 0

			if contail < linebegin {
				j = consize - linebegin + contail
				stitchbuffer = make([]byte, j)
				copy(stitchbuffer, conbuf[linebegin:consize])
				copy(stitchbuffer[consize-linebegin:], conbuf[:contail])
			}

			Con_LayoutLine(linebegin)
		}

		partiallinestart = 0
		txt = txt[i:]
	}

	if txt != "" {
		i := uint32(len(txt))
		if i+partiallinestart < consize {
			if partiallinestart == 0 {
				partiallinestart = contail
			}
			Con_ClearBufferSpace(i + 1)
			Con_CopyToBuffer([]byte(txt))
			conbuf[contail] = 0
		}
	}
}

func Con_ScrollUp(numlines uint32) {
	distance := int32(displayline - firstline)
	if distance < 0 {
		distance += int32(maxlines)
	}
	if numlines > uint32(distance) {
		numlines = uint32(distance)
	}
	displayline -= numlines
	displayline %= maxlines
}

func Con_ScrollDown(numlines uint32) {
	distance := int32(lastline - displayline)
	if distance < 0 {
		distance += int32(maxlines)
	}
	if numlines > uint32(distance) {
		numlines = uint32(distance)
	}
	displayline += numlines
	displayline %= maxlines
}

func Con_Home() {
	displayline = firstline
}

func Con_End() {
	displayline = lastline
}

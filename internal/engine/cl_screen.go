package engine

import (
	"fmt"
	"math"
	"os"
	"strings"
	"time"
	"unsafe"
)

var glwidth, glheight int

const DEFAULT_SSHOT_FORMAT = "png"

var scr_con_current float32
var scr_conlines float32 // lines of console to display

var scr_con_cursorspeed float32 = 4

var scr_clearnotifylines uint

var oldscreensize, oldfov, oldsbar float32
var scr_viewsize = cvar_t{name: "viewsize", string: "100", flags: CVAR_ARCHIVE}
var scr_fov = cvar_t{name: "fov", string: "115", flags: CVAR_ARCHIVE} // 10 - 140
var scr_consize = cvar_t{name: "scr_consize", string: "0.6"}
var scr_conspeed = cvar_t{name: "scr_conspeed", string: "999999999"}
var scr_centertime = cvar_t{name: "scr_centertime", string: "2"}
var scr_showram = cvar_t{name: "showram", string: "1"}
var scr_showturtle = cvar_t{name: "showturtle", string: "0"}
var scr_showpause = cvar_t{name: "showpause", string: "1"}
var scr_printspeed = cvar_t{name: "scr_printspeed", string: "8"}
var scr_allowsnap = cvar_t{name: "scr_allowsnap", string: "0", OnChange: OnChange_scr_allowsnap}

var scr_clock = cvar_t{name: "cl_clock", string: "2"}
var scr_clock_x = cvar_t{name: "cl_clock_x", string: "-1"}
var scr_clock_y = cvar_t{name: "cl_clock_y", string: "0"}

var scr_gameclock = cvar_t{name: "cl_gameclock", string: "3"}
var scr_gameclock_x = cvar_t{name: "cl_gameclock_x", string: "0"}
var scr_gameclock_y = cvar_t{name: "cl_gameclock_y", string: "-3"}

var scr_democlock = cvar_t{name: "cl_democlock", string: "0"}
var scr_democlock_x = cvar_t{name: "cl_democlock_x", string: "0"}
var scr_democlock_y = cvar_t{name: "cl_democlock_y", string: "-2"}

var show_speed = cvar_t{name: "show_speed", string: "1"}
var show_speed_x = cvar_t{name: "show_speed_x", string: "-1"}
var show_speed_y = cvar_t{name: "show_speed_y", string: "2"}

var show_fps = cvar_t{name: "show_fps", string: "1"}
var show_fps_x = cvar_t{name: "show_fps_x", string: "-1"}
var show_fps_y = cvar_t{name: "show_fps_y", string: "1"}

var show_framestddev = cvar_t{name: "show_framestddev", string: "0"}
var show_framestddev_x = cvar_t{name: "show_framestddev_x", string: "-5"}
var show_framestddev_y = cvar_t{name: "show_framestddev_y", string: "-2"}

var scr_sshot_format = cvar_t{name: "sshot_format", string: DEFAULT_SSHOT_FORMAT}
var scr_sshot_dir = cvar_t{name: "sshot_dir", string: ""}

var gl_triplebuffer = cvar_t{name: "gl_triplebuffer", string: "1", flags: CVAR_ARCHIVE}

var scr_autoid = cvar_t{name: "scr_autoid", string: "0"}
var scr_coloredText = cvar_t{name: "scr_coloredText", string: "0"}

var scr_initialized bool // ready to draw

var pausepic *Picture
var loadingpic *Picture

var rampic *Picture
var netpic *Picture
var turtlepic *Picture

var clearconsole int
var clearnotify int

var scr_vrect vrect_t

var scr_skipupdate bool

var scr_drawloading bool
var scr_disabled_for_loading bool
var scr_disabled_time float32

var block_drawing bool

var scr_autosshot_countdown int
var auto_matchname string

func SCR_NEED_CONSOLE_BACKGROUND() bool {
	return cls.state < ca_active && cl.intermission == 0
}

func OnChange_scr_allowsnap(v *cvar_t, s string) bool {
	return cls.state >= ca_connected && cbuf_current == cbuf_svc
}

/**************************** CENTER PRINTING ********************************/

var scr_centerstring string
var scr_centertime_start float32 // for slow victory printing
var scr_centertime_off float32
var scr_center_lines int
var scr_erase_lines int
var scr_erase_center int

// Called for important messages that should stay in the center of the screen for a few moments
func SCR_CenterPrint(str string) {
	scr_centerstring = strlcpy(str, 1024)
	scr_centertime_off = scr_centertime.value
	scr_centertime_start = float32(cl.time)

	// count the number of lines for centering
	scr_center_lines = 1
	for i := 0; i < len(str); i++ {
		if str[i] == '\n' {
			scr_center_lines++
		}
	}
}

func SCR_DrawCenterString() {
	var remaining int

	// the finale prints the characters one at a time
	if cl.intermission != 0 {
		remaining = int(float64(scr_printspeed.value) * (cl.time - float64(scr_centertime_start)))
	} else {
		remaining = 9999
	}

	scr_erase_center = 0
	start := scr_centerstring

	var y int
	if scr_center_lines <= 4 {
		y = int(float64(vid.conheight) * 0.35)
	} else {
		y = 48
	}

	for {
		// scan the width of the line
		l := 0
		for l = 0; l < 40; l++ {
			if at(start, l) == '\n' || at(start, l) == 0 {
				break
			}
		}

		x := (int(vid.conwidth) - l*8) / 2

		for j := 0; j < l; j, x = j+1, x+8 {
			Draw_Character(x, y, start[j])
			if remaining == 0 {
				return
			}
			remaining--
		}

		y += 8

		p := 0
		for p < len(start) && start[p] != '\n' {
			p++
		}

		if p >= len(start) {
			break
		}

		start = start[p+1:] // skip the \n
	}
}

func SCR_CheckDrawCenterString() {
	if scr_center_lines > scr_erase_lines {
		scr_erase_lines = scr_center_lines
	}

	scr_centertime_off = float32(float64(scr_centertime_off) - cls.frametime)

	if scr_centertime_off <= 0 && cl.intermission == 0 {
		return
	}
	if key_dest != key_game {
		return
	}

	SCR_DrawCenterString()
}

func SCR_EraseCenterString() {
	if scr_erase_center > vid.numpages {
		scr_erase_center++
		scr_erase_lines = 0
		return
	}
	scr_erase_center++

	var y int
	if scr_center_lines <= 4 {
		y = int(float64(vid.conheight) * 0.35)
	} else {
		y = 48
	}

	Draw_TileClear(0, y, int(vid.conwidth), min(8*scr_erase_lines, int(vid.conheight)-y-1))
}

/************************************ FOV ************************************/

func CalcFov(fov float32, fov_x, fov_y *float32, width, height float32) {
	var t, fovx, fovy float32

	if fov < 10 {
		fov = 10
	} else if fov > 140 {
		fov = 140
	}

	if width/4 < height/3 {
		fovx = fov
		t = float32(float64(width) / math.Tan(float64(fovx/360)*math.Pi))
		fovy = float32(math.Atan(float64(height/t)) * 360 / math.Pi)
	} else {
		fovx = fov
		t = float32(4.0 / math.Tan(float64(fovx/360)*math.Pi))
		fovy = float32(math.Atan(3.0/float64(t)) * 360 / math.Pi)
		t = float32(float64(height) / math.Tan(float64(fovy/360)*math.Pi))
		fovx = float32(math.Atan(float64(width/t)) * 360 / math.Pi)
	}

	if fovx < 10 || fovx > 140 {
		if fovx < 10 {
			fovx = 10
		} else if fovx > 140 {
			fovx = 140
		}

		t = float32(float64(width) / math.Tan(float64(fovx/360)*math.Pi))
		fovy = float32(math.Atan(float64(height/t)) * 360 / math.Pi)
	}

	if fovy < 10 || fovy > 140 {
		if fovy < 10 {
			fovy = 10
		} else if fovy > 140 {
			fovy = 140
		}

		t = float32(float64(height) / math.Tan(float64(fovy/360)*math.Pi))
		fovx = float32(math.Atan(float64(width/t)) * 360 / math.Pi)
	}

	if fovx < 1 || fovx > 179 || fovy < 1 || fovy > 179 {
		Sys_Error("CalcFov: Bad fov (%f, %f)", fovx, fovy)
	}

	*fov_x = fovx
	*fov_y = fovy
}

// Must be called whenever vid changes
func SCR_CalcRefdef() {
	var size float32
	var h int
	full := false

	vid.recalc_refdef = 0

	// force the status bar to redraw
	Sbar_Changed()

	// bound viewsize
	if scr_viewsize.value < 30 {
		Cvar_Set(&scr_viewsize, "30")
	}
	if scr_viewsize.value > 120 {
		Cvar_Set(&scr_viewsize, "120")
	}

	// bound field of view
	if scr_fov.value < 10 {
		Cvar_Set(&scr_fov, "10")
	}
	if scr_fov.value > 140 {
		Cvar_Set(&scr_fov, "140")
	}

	// intermission is always full screen
	if cl.intermission != 0 {
		size = 120
	} else {
		size = scr_viewsize.value
	}

	if size >= 120 {
		sb_lines = 0 // no status bar at all
	} else if size >= 110 {
		sb_lines = 24 // no inventory
	} else {
		sb_lines = 24 + 16 + 8
	}

	if scr_viewsize.value >= 100.0 {
		full = true
		size = 100.0
	} else {
		size = scr_viewsize.value
	}

	if cl.intermission != 0 {
		full = true
		size = 100.0
		sb_lines = 0
	}
	size = float32(float64(size) / 100.0)

	if cl_sbar.value == 0 && full {
		h = int(vid.conheight)
	} else {
		h = int(vid.conheight) - sb_lines
	}

	r_refdef.vrect.width = int(float32(vid.conwidth) * size)
	if r_refdef.vrect.width < 96 {
		size = float32(96.0 / float64(r_refdef.vrect.width))
		r_refdef.vrect.width = 96 // min for icons
	}

	r_refdef.vrect.height = int(float32(vid.conheight) * size)
	if cl_sbar.value != 0 || !full {
		if r_refdef.vrect.height > int(vid.conheight)-sb_lines {
			r_refdef.vrect.height = int(vid.conheight) - sb_lines
		}
	} else if r_refdef.vrect.height > int(vid.conheight) {
		r_refdef.vrect.height = int(vid.conheight)
	}
	r_refdef.vrect.x = (int(vid.conwidth) - r_refdef.vrect.width) / 2
	if full {
		r_refdef.vrect.y = 0
	} else {
		r_refdef.vrect.y = (h - r_refdef.vrect.height) / 2
	}

	CalcFov(scr_fov.value, &r_refdef.fov_x, &r_refdef.fov_y, float32(r_refdef.vrect.width), float32(r_refdef.vrect.height))

	scr_vrect = r_refdef.vrect

	Draw_SizeChanged()
}

// Keybinding command
func SCR_SizeUp_f() {
	Cvar_SetValue(&scr_viewsize, scr_viewsize.value+10)
	vid.recalc_refdef = 1
}

// Keybinding command
func SCR_SizeDown_f() {
	Cvar_SetValue(&scr_viewsize, scr_viewsize.value-10)
	vid.recalc_refdef = 1
}

/********************************** ELEMENTS **********************************/

func SCR_DrawRam() {
	if scr_showram.value == 0 {
		return
	}

	if !r_cache_thrash {
		return
	}

	Draw_DrawPicture(rampic, scr_vrect.x+32, scr_vrect.y, 32, 32)
}

var turtle_count int

func SCR_DrawTurtle() {
	if scr_showturtle.value == 0 {
		return
	}

	if cls.frametime < 0.1 {
		turtle_count = 0
		return
	}

	turtle_count++
	if turtle_count < 3 {
		return
	}

	Draw_DrawPicture(turtlepic, scr_vrect.x, scr_vrect.y, 32, 32)
}

func SCR_DrawNet() {
	if cls.demoplayback {
		return
	}

	if cls.netqw == nil {
		return
	}

	if NetQW_GetTimeSinceLastPacketFromServer(cls.netqw) < 500000 {
		return
	}

	Draw_DrawPicture(netpic, scr_vrect.x+64, scr_vrect.y, 32, 32)
}

// ELEMENT_X_COORD and ELEMENT_Y_COORD from cl_screen.c
func elementX(x *cvar_t, str string) int {
	if x.value < 0 {
		return int(float32(int(vid.conwidth)-len(str)*8) + 8*x.value)
	}
	return int(8 * x.value)
}

func elementY(y *cvar_t) int {
	if y.value < 0 {
		return int(float32(int(vid.conheight)-sb_lines) + 8*y.value)
	}
	return int(8 * y.value)
}

var (
	fps_lastfps        float32
	fps_lastframetime  float64
	fps_last_fps_count int
)

// The HUD draws these every frame while the values change once a second or
// less, so the text is kept, not formatted anew.
var (
	fps_text      string
	fps_textfps   float32
	fps_textmode  float32
	speed_text    [16]byte
	clock_text    string
	clock_textsec int64
)

func SCR_DrawFPS() {
	if show_fps.value == 0 {
		return
	}

	t := Sys_DoubleTime()
	if t-fps_lastframetime >= 1.0 {
		fps_lastfps = float32(float64(fps_count-fps_last_fps_count) / (t - fps_lastframetime))
		fps_last_fps_count = fps_count
		fps_lastframetime = t
	}

	if fps_text == "" || fps_lastfps != fps_textfps || show_fps.value != fps_textmode {
		suffix := ""
		if show_fps.value == 2 {
			suffix = " FPS"
		}
		fps_text = fmt.Sprintf("%3.1f%s", float64(fps_lastfps)+0.05, suffix)
		fps_textfps, fps_textmode = fps_lastfps, show_fps.value
	}
	str := fps_text
	x := elementX(&show_fps_x, str)
	y := elementY(&show_fps_y)
	Draw_String(x, y, str)
}

var (
	stddev_lastframestddev float64
	stddev_framedevcum     float64
	stddev_lastframetime   float64
	stddev_last_fps_count  int
)

func SCR_DrawFrameStdDev() {
	if show_framestddev.value == 0 {
		return
	}

	stddev_framedevcum += (cls.framedev * 1000) * (cls.framedev * 1000)

	t := Sys_DoubleTime()
	if t-stddev_lastframetime >= 1.0 {
		stddev_lastframestddev = math.Sqrt(stddev_framedevcum / float64(fps_count-stddev_last_fps_count))
		stddev_last_fps_count = fps_count
		stddev_framedevcum = 0
		stddev_lastframetime = t
	}

	suffix := ""
	if show_framestddev.value == 2 {
		suffix = " frame stddev"
	}
	str := fmt.Sprintf("%3.1f%s", stddev_lastframestddev, suffix)
	x := elementX(&show_framestddev_x, str)
	y := elementY(&show_framestddev_y)
	Draw_String(x, y, str)
}

var (
	speed_maxspeed      float32
	speed_display_speed float32 = -1
	speed_lastframetime float64
	speed_lastmynum     = -1
)

func SCR_DrawSpeed() {
	var vel vec3_t
	var mynum int

	if show_speed.value == 0 {
		return
	}

	t := Sys_DoubleTime()

	if cl.spectator == 0 {
		mynum = cl.playernum
	} else if mynum = Cam_TrackNum(); mynum == -1 {
		mynum = cl.playernum
	}

	if mynum != speed_lastmynum {
		speed_lastmynum = mynum
		speed_lastframetime = t
		speed_display_speed = -1
		speed_maxspeed = 0
	}

	if cl.spectator == 0 || cls.demoplayback || mynum == cl.playernum {
		vel = cl.simvel
	} else {
		vel = cl.frames[cl.validsequence&UPDATE_MASK].playerstate[mynum].velocity
	}

	vel[2] = 0
	speed := VectorLength(vel)

	speed_maxspeed = max(speed_maxspeed, speed)

	if speed_display_speed >= 0 {
		suffix := ""
		if show_speed.value == 2 {
			suffix = " SPD"
		}
		b := appendPadInt(speed_text[:0], int64(int(speed_display_speed)), 3)
		b = append(b, suffix...)
		str := unsafe.String(&b[0], len(b))
		x := elementX(&show_speed_x, str)
		y := elementY(&show_speed_y)
		Draw_String(x, y, str)
	}

	if t-speed_lastframetime >= 0.1 {
		speed_lastframetime = t
		speed_display_speed = speed_maxspeed
		speed_maxspeed = 0
	}
}

func SCR_DrawClock() {
	var str string

	if scr_clock.value == 0 {
		return
	}

	if scr_clock.value == 2 {
		now := time.Now()
		if s := now.Unix(); clock_text == "" || s != clock_textsec {
			clock_text = now.Format("15:04:05")
			clock_textsec = s
		}
		str = clock_text
	} else {
		var t float32
		if cl.servertime_works {
			t = float32(cl.servertime)
		} else {
			t = float32(cls.realtime)
		}
		str = SecondsToHourString(int(t))
	}

	x := elementX(&scr_clock_x, str)
	y := elementY(&scr_clock_y)
	Draw_String(x, y, str)
}

func SCR_DrawGameClock() {
	var str, s string
	var timelimit float32

	if scr_gameclock.value == 0 {
		return
	}

	if scr_gameclock.value == 2 || scr_gameclock.value == 4 {
		timelimit = 60 * Q_atof(Info_ValueForKey(cl.serverinfo, "timelimit"))
	} else {
		timelimit = 0
	}

	if cl.countdown || cl.standby {
		str = SecondsToHourString(int(timelimit))
	} else {
		// abs() takes an int in C, so the difference is truncated first
		d := int(float64(timelimit) - cl.gametime)
		if d < 0 {
			d = -d
		}
		str = SecondsToHourString(d)
	}

	if colon := strings.IndexByte(str, ':'); (scr_gameclock.value == 3 || scr_gameclock.value == 4) && colon >= 0 {
		s = str[colon+1:] // or just use SecondsToMinutesString() ...
	} else {
		s = str
	}

	x := elementX(&scr_gameclock_x, s)
	y := elementY(&scr_gameclock_y)
	Draw_String(x, y, s)
}

func SCR_DrawDemoClock() {
	var str string

	if !cls.demoplayback || scr_democlock.value == 0 {
		return
	}

	if scr_democlock.value == 2 {
		str = SecondsToHourString(int(cls.demotime))
	} else {
		str = SecondsToHourString(int(cls.demotime - demostarttime))
	}

	x := elementX(&scr_democlock_x, str)
	y := elementY(&scr_democlock_y)
	Draw_String(x, y, str)
}

func SCR_DrawPause() {
	if scr_showpause.value == 0 { // turn off for screenshots
		return
	}

	if cl.paused == 0 {
		return
	}

	if pausepic == nil {
		pausepic = Draw_LoadPicture("gfx/pause.lmp", DRAW_LOADPICTURE_DUMMYFALLBACK)
	}

	Draw_DrawPicture(pausepic, (int(vid.conwidth)-128)/2, (int(vid.conheight)-48-24)/2, 128, 24)
}

func SCR_DrawLoading() {
	if !scr_drawloading {
		return
	}

	if loadingpic == nil {
		loadingpic = Draw_LoadPicture("gfx/loading.lmp", DRAW_LOADPICTURE_DUMMYFALLBACK)
	}

	Draw_DrawPicture(loadingpic, (int(vid.conwidth)-144)/2, (int(vid.conheight)-48-24)/2, 144, 24)
}

func SCR_BeginLoadingPlaque() {
	if cls.state != ca_active {
		return
	}

	if key_dest == key_console {
		return
	}

	// redraw with no console and the loading plaque
	Sbar_Changed()
	scr_drawloading = true
	SCR_UpdateScreen()
	scr_drawloading = false

	scr_disabled_for_loading = true
	scr_disabled_time = float32(cls.realtime)
}

func SCR_EndLoadingPlaque() {
	if !scr_disabled_for_loading {
		return
	}

	scr_disabled_for_loading = false
}

/********************************** CONSOLE **********************************/

func SCR_SetUpToDrawConsole() {
	Con_CheckResize(vid.conwidth)

	// decide on the height of the console
	if SCR_NEED_CONSOLE_BACKGROUND() {
		scr_conlines = float32(vid.conheight) // full screen
		scr_con_current = scr_conlines
	} else if key_dest == key_console {
		scr_conlines = float32(vid.conheight) * scr_consize.value
		scr_conlines = bound(float32(30), scr_conlines, float32(vid.conheight))
	} else {
		scr_conlines = 0 // none visible
	}

	if scr_conlines < scr_con_current {
		scr_con_current = float32(float64(scr_con_current) - float64(scr_conspeed.value)*cls.trueframetime*float64(vid.conheight)/320)
		scr_con_current = max(scr_con_current, scr_conlines)
	} else if scr_conlines > scr_con_current {
		scr_con_current = float32(float64(scr_con_current) + float64(scr_conspeed.value)*cls.trueframetime*float64(vid.conheight)/320)
		scr_con_current = min(scr_con_current, scr_conlines)
	}

	if clearconsole < vid.numpages {
		clearconsole++
		Sbar_Changed()
	} else {
		clearconsole++
		if clearnotify < vid.numpages {
			clearnotify++
		} else {
			clearnotify++
			scr_clearnotifylines = 0
		}
	}
}

func SCR_DrawDownload(vislines uint) {
	var dlbar []byte
	var dotlength, barlength int

	linewidth := int(vid.conwidth>>3) - 2

	// draw the download bar
	// figure out width
	text := cls.downloadname
	if slash := strings.LastIndexByte(text, '/'); slash >= 0 {
		text = text[slash+1:]
	}

	x := linewidth - ((linewidth * 7) / 40)
	i := linewidth / 3

	if x-i < 20 {
		return
	}

	textlength := len(text)

	if len(text) > i {
		barlength = x - i - 20
		dotlength = 3
		textlength = i
	} else {
		barlength = x - len(text) - 17
		dotlength = 0
	}

	if textlength+dotlength+2+1+barlength+1+14+1 > 1024 {
		return
	}

	dlbar = append(dlbar, text[:textlength]...)
	if dotlength != 0 {
		dlbar = append(dlbar, "..."...)
	}

	dlbar = append(dlbar, ": \x80"...)

	// where's the dot go?
	n := barlength * cls.downloadpercent / 100

	for j := 0; j < barlength; j++ {
		if j == n {
			dlbar = append(dlbar, '\x83')
		} else {
			dlbar = append(dlbar, '\x81')
		}
	}
	dlbar = append(dlbar, '\x82')

	if cls.ftexsupported&FTEX_CHUNKEDDOWNLOADS != 0 {
		dlbar = append(dlbar, fmt.Sprintf(" %4d KB/s", cls.downloadrate)...)
	} else {
		dlbar = append(dlbar, fmt.Sprintf(" %02d%%\x87", cls.downloadpercent)...)
	}

	// draw it
	Draw_String(8, int(vislines)-22+8, string(dlbar))
}

func SCR_DrawMessageMode(y uint) {
	var skip int

	clearnotify = 0

	if chat_team {
		Draw_String(8, int(y), "say_team:")
		skip = 11
	} else {
		Draw_String(8, int(y), "say:")
		skip = 5
	}

	// FIXME: clean this up
	temp := []byte(cstr(chat_buffer[:]))

	// add the cursor frame
	if int(curtime*float64(scr_con_cursorspeed))&1 != 0 {
		if chat_linepos == len(temp) {
			temp = append(temp, 0)
		}
		temp[chat_linepos] = 11
	}
	s := string(temp)

	// prestep if horizontally scrolling
	if chat_linepos+skip >= int(vid.conwidth>>3) {
		s = s[min(1+chat_linepos+skip-int(vid.conwidth>>3), len(s)):]
	}

	length := len(s)
	if length+skip > int(vid.conwidth>>3) {
		length = int(vid.conwidth>>3) - skip
	}

	if length > 0 {
		Draw_String_Length(skip<<3, int(y), s, length)
	}
}

func SCR_DrawConsole() {
	if scr_con_current != 0 {
		Con_DrawConsole(int(scr_con_current))
		clearconsole = 0

		if cls.download != nil {
			SCR_DrawDownload(uint(scr_con_current))
		}
	} else {
		if key_dest == key_game || key_dest == key_message {
			lines := uint(Con_DrawNotify()) // only draw notify in game

			if key_dest == key_message {
				SCR_DrawMessageMode(lines * 8)
				lines++
			}

			if lines != 0 {
				clearnotify = 0

				if lines > scr_clearnotifylines {
					scr_clearnotifylines = lines
				}
			}
		}
	}
}

func SCR_BeginChat(is_chat_team bool) {
	if cls.state != ca_active {
		return
	}

	chat_team = is_chat_team
	key_dest = key_message
	chat_buffer[0] = 0
	chat_linepos = 0
}

func Con_MessageMode_f() {
	SCR_BeginChat(false)
}

func Con_MessageMode2_f() {
	SCR_BeginChat(true)
}

/*********************************** AUTOID ***********************************/

func qglProject(objx, objy, objz float32, model, proj []float32, view []int, winx, winy, winz *float32) int {
	var in, out [4]float32

	in[0] = objx
	in[1] = objy
	in[2] = objz
	in[3] = 1.0

	for i := 0; i < 4; i++ {
		out[i] = in[0]*model[0*4+i] + in[1]*model[1*4+i] + in[2]*model[2*4+i] + in[3]*model[3*4+i]
	}

	for i := 0; i < 4; i++ {
		in[i] = out[0]*proj[0*4+i] + out[1]*proj[1*4+i] + out[2]*proj[2*4+i] + out[3]*proj[3*4+i]
	}

	if in[3] == 0 {
		return 0
	}

	// VectorScale only touches the first three
	scale := 1 / in[3]
	in[0] *= scale
	in[1] *= scale
	in[2] *= scale

	*winx = float32(view[0]) + (1+in[0])*float32(view[2])/2
	*winy = float32(view[1]) + (1+in[1])*float32(view[3])/2
	*winz = (1 + in[2]) / 2

	return 1
}

type autoid_player_t struct {
	x, y   float32
	player *player_info_t
}

var autoids [MAX_CLIENTS]autoid_player_t
var autoid_count int

func SCR_SetupAutoID() {
	var view [4]int
	var model, project [16]float32
	var winz float32
	var visitem item_vis_t
	tracknum := -1

	autoid_count = 0

	if scr_autoid.value == 0 {
		return
	}

	if cls.state != ca_active || cl.validsequence == 0 {
		return
	}

	if !cls.demoplayback && cl.spectator == 0 {
		return
	}

	if GPU_GetSceneMatrices(model[:], project[:], view[:]) == 0 {
		return
	}

	if cl.spectator != 0 {
		tracknum = Cam_TrackNum()
	}

	visitem.forward = vpn
	visitem.right = vright
	visitem.up = vup
	visitem.vieworg = r_origin

	for j := 0; j < MAX_CLIENTS; j++ {
		state := &cl.frames[cl.parsecount&UPDATE_MASK].playerstate[j]
		info := &cl.players[j]

		if state.messagenum != cl.parsecount || j == cl.playernum || j == tracknum || info.spectator != 0 {
			continue
		}

		if (state.modelindex == cl_modelindices[mi_player] && ISDEAD(state.frame)) ||
			state.modelindex == cl_modelindices[mi_h_player] {
			continue
		}

		if R_CullSphere(state.origin, 0) {
			continue
		}

		visitem.entorg = state.origin
		visitem.entorg[2] += 27
		VectorSubtract(visitem.entorg, visitem.vieworg, &visitem.dir)
		visitem.dist = DotProduct(visitem.dir, visitem.forward)
		visitem.radius = 25

		if !TP_IsItemVisible(&visitem) {
			continue
		}

		id := &autoids[autoid_count]
		id.player = info
		origin := state.origin
		if qglProject(origin[0], origin[1], origin[2]+28, model[:], project[:], view[:], &id.x, &id.y, &winz) != 0 {
			autoid_count++
		}
	}
}

func SCR_DrawAutoID() {
	if scr_autoid.value == 0 || (!cls.demoplayback && cl.spectator == 0) {
		return
	}

	for i := 0; i < autoid_count; i++ {
		x := int(autoids[i].x * float32(vid.conwidth) / float32(glwidth))
		y := int((float32(glheight) - autoids[i].y) * float32(vid.conheight) / float32(glheight))
		Draw_String(x-len(autoids[i].player.name)*4, y-8, autoids[i].player.name)
	}
}

/********************************* TILE CLEAR *********************************/

func SCR_TileClear() {
	if cls.state != ca_active && cl.intermission != 0 {
		Draw_TileClear(0, 0, int(vid.conwidth), int(vid.conheight))
		return
	}

	if r_refdef.vrect.x > 0 {
		// left
		Draw_TileClear(0, 0, r_refdef.vrect.x, int(vid.conheight)-sb_lines)
		// right
		Draw_TileClear(r_refdef.vrect.x+r_refdef.vrect.width, 0,
			int(vid.conwidth)-(r_refdef.vrect.x+r_refdef.vrect.width), int(vid.conheight)-sb_lines)
	}
	if r_refdef.vrect.y > 0 {
		// top
		Draw_TileClear(r_refdef.vrect.x, 0, r_refdef.vrect.width, r_refdef.vrect.y)
	}
	if r_refdef.vrect.y+r_refdef.vrect.height < int(vid.conheight)-sb_lines {
		// bottom
		Draw_TileClear(r_refdef.vrect.x,
			r_refdef.vrect.y+r_refdef.vrect.height,
			r_refdef.vrect.width,
			int(vid.conheight)-sb_lines-(r_refdef.vrect.height+r_refdef.vrect.y))
	}
}

func SCR_DrawElements() {
	if scr_drawloading {
		SCR_DrawLoading()
		Sbar_Draw()
	} else {
		if cl.intermission == 1 {
			Sbar_IntermissionOverlay()
			Con_ClearNotify()
		} else if cl.intermission == 2 {
			Sbar_FinaleOverlay()
			SCR_CheckDrawCenterString()
			Con_ClearNotify()
		}

		if cls.state == ca_active {
			SCR_DrawRam()
			SCR_DrawNet()
			SCR_DrawTurtle()
			SCR_DrawPause()
			SCR_DrawAutoID()
			if cl.intermission == 0 {
				if key_dest != key_menu {
					Draw_Crosshair()
				}
				SCR_CheckDrawCenterString()
				SCR_DrawSpeed()
				SCR_DrawClock()
				SCR_DrawGameClock()
				SCR_DrawDemoClock()
				SCR_DrawFPS()
				SCR_DrawFrameStdDev()
				Sbar_Draw()
			}
		}

		if scr_autosshot_countdown == 0 {
			SCR_DrawConsole()
			M_Draw()
		}
	}
}

/******************************* UPDATE SCREEN *******************************/

// This is called every frame, and can also be called explicitly to flush text to the screen.
func SCR_UpdateScreen() {
	if !scr_initialized {
		return // not initialized yet
	}

	if scr_skipupdate || block_drawing {
		return
	}

	if scr_disabled_for_loading {
		if cls.realtime-float64(scr_disabled_time) > 20 {
			scr_disabled_for_loading = false
		} else {
			return
		}
	}

	vid.numpages = 2 + int(gl_triplebuffer.value)

	VID_CheckResize()

	// check for vid changes
	if oldfov != scr_fov.value {
		oldfov = scr_fov.value
		vid.recalc_refdef = 1
	}

	if oldscreensize != scr_viewsize.value {
		oldscreensize = scr_viewsize.value
		vid.recalc_refdef = 1
	}

	if oldsbar != cl_sbar.value {
		oldsbar = cl_sbar.value
		vid.recalc_refdef = 1
	}

	if vid.recalc_refdef != 0 {
		SCR_CalcRefdef()
	}

	if (v_contrast.value > 1 && !VID_HWGammaSupported()) || gl_clear.value != 0 {
		Sbar_Changed()
	}

	// do 3D refresh drawing, and then update the screen
	VID_BeginFrame()

	glwidth = int(VID_GetWidth())
	glheight = int(VID_GetHeight())

	soft_gamma := V_SoftGammaActive() && R_FBO_Init(glwidth, glheight)
	if soft_gamma {
		R_FBO_Bind()
	}

	SCR_SetUpToDrawConsole()

	V_RenderView()

	SCR_SetupAutoID()

	R_Set2D()

	R_PolyBlend()

	// draw any areas not covered by the refresh
	SCR_TileClear()

	if r_netgraph.value != 0 {
		R_NetGraph()
	}

	SCR_DrawElements()

	SB_Frame()

	Lua_Frame_2D()

	R_BrightenScreen()

	V_UpdatePalette(false)

	SCR_CheckAutoScreenshot()

	if soft_gamma {
		R_FBO_Unbind()
		R_PostProcess_Draw(R_FBO_GetColorTexture(), v_gamma.value, v_contrast.value, v_blend[:])
	}

	VID_Update(nil)
}

func SCR_UpdateWholeScreen() {
	SCR_UpdateScreen()
}

/******************************** SCREENSHOTS ********************************/

const (
	SSHOT_FAILED       = -1
	SSHOT_FAILED_QUIET = -2 // failed but don't print an error message
	SSHOT_SUCCESS      = 0
)

type image_format_t int

const (
	IMAGE_PCX image_format_t = iota
	IMAGE_TGA
	IMAGE_JPEG
	IMAGE_PNG
)

func SShot_ExtForFormat(format image_format_t) string {
	switch format {
	case IMAGE_PCX:
		return ".pcx"
	case IMAGE_TGA:
		return ".tga"
	case IMAGE_JPEG:
		return ".jpg"
	case IMAGE_PNG:
		return ".png"
	}
	Sys_Error("SShot_ExtForFormat: unknown format")
	return "err"
}

// JPEG is not supported (USE_JPEG=0), so it never matches here
func SShot_FormatForName(name string) image_format_t {
	ext := COM_FileExtension(name)

	if Q_strcasecmp(ext, "tga") == 0 {
		return IMAGE_TGA
	} else if Q_strcasecmp(ext, "png") == 0 {
		return IMAGE_PNG
	} else if Q_strcasecmp(scr_sshot_format.string, "tga") == 0 {
		return IMAGE_TGA
	} else if Q_strcasecmp(scr_sshot_format.string, "png") == 0 {
		return IMAGE_PNG
	}
	return IMAGE_PNG
}

// CPU mirror of post-process shader for screenshots
func applySoftGammaToBuffer(buffer []byte, size int) {
	g := v_gamma.value * R_PostProcess_CrtGamma()
	ct := v_contrast.value
	ba := v_blend[3]
	br, bgv, bb := v_blend[0], v_blend[1], v_blend[2]

	powf := func(x, y float32) float32 { return float32(math.Pow(float64(x), float64(y))) }

	for i := 0; i < size; i += 3 {
		r := float32(buffer[i+0]) / 255.0
		gv := float32(buffer[i+1]) / 255.0
		b := float32(buffer[i+2]) / 255.0

		r = r*(1.0-ba) + br*ba
		gv = gv*(1.0-ba) + bgv*ba
		b = b*(1.0-ba) + bb*ba

		r = powf(max(r*ct, 0.0), g)
		gv = powf(max(gv*ct, 0.0), g)
		b = powf(max(b*ct, 0.0), g)

		if r > 1.0 {
			buffer[i+0] = 255
		} else {
			buffer[i+0] = byte(r * 255.0)
		}
		if gv > 1.0 {
			buffer[i+1] = 255
		} else {
			buffer[i+1] = byte(gv * 255.0)
		}
		if b > 1.0 {
			buffer[i+2] = 255
		} else {
			buffer[i+2] = byte(b * 255.0)
		}
	}
}

// applies hwgamma (or its soft-mode equivalent) to RGB data
func applyHWGamma(buffer []byte, size int) {
	if V_SoftGammaActive() {
		applySoftGammaToBuffer(buffer, size)
		return
	}

	if VID_HWGammaSupported() {
		for i := 0; i < size; i += 3 {
			buffer[i+0] = byte(ramps[0][buffer[i+0]] >> 8)
			buffer[i+1] = byte(ramps[1][buffer[i+1]] >> 8)
			buffer[i+2] = byte(ramps[2][buffer[i+2]] >> 8)
		}
	}
}

func SCR_Screenshot(name string) int {
	success := SSHOT_FAILED

	if at(name, 0) == '/' {
		name = name[1:]
	}
	format := SShot_FormatForName(name)
	name = COM_ForceExtension(name, SShot_ExtForFormat(format))
	buffersize := glwidth * glheight * 3

	buffer := make([]byte, buffersize)

	GPU_ReadPixels(buffer, uint32(glwidth), uint32(glheight))

	if format == IMAGE_PNG {
		applyHWGamma(buffer, buffersize)
		if Image_WritePNG(name, int(image_png_compression_level.value), buffer, -glwidth, glheight) {
			success = SSHOT_SUCCESS
		} else {
			success = SSHOT_FAILED
		}
	}

	if format == IMAGE_TGA {
		// swap rgb to bgr
		for i := 0; i < buffersize; i += 3 {
			buffer[i], buffer[i+2] = buffer[i+2], buffer[i]
		}
		applyHWGamma(buffer, buffersize)
		if Image_WriteTGA(name, buffer, glwidth, glheight) {
			success = SSHOT_SUCCESS
		} else {
			success = SSHOT_FAILED
		}
	}

	return success
}

func SCR_ScreenShot_f() {
	var name, ext string

	sshot_dir := scr_sshot_dir.string
	if sshot_dir == "" {
		sshot_dir = cls.gamedirfile
	}

	if Cmd_Argc() == 2 {
		name = strlcpy(Cmd_Argv(1), MAX_OSPATH)
	} else if Cmd_Argc() == 1 {
		// find a file name to save it to

		if Q_strcasecmp(scr_sshot_format.string, "tga") == 0 {
			ext = "tga"
		}

		if Q_strcasecmp(scr_sshot_format.string, "png") == 0 {
			ext = "png"
		}
		if ext == "" {
			ext = DEFAULT_SSHOT_FORMAT
		}

		i := 0
		for i = 0; i < 999; i++ {
			name = fmt.Sprintf("classicq%03d.%s", i, ext)
			f, err := os.Open(va("%s/%s/%s", com_basedir, sshot_dir, name))
			if err != nil {
				break // file doesn't exist
			}
			f.Close()
		}
		if i == 1000 {
			Com_Printf("Error: Cannot create more than 1000 screenshots\n")
			return
		}
	} else {
		Com_Printf("Usage: %s [filename]\n", Cmd_Argv(0))
		return
	}

	filename := name
	for len(filename) > 0 && (filename[0] == '/' || filename[0] == '\\') {
		filename = filename[1:]
	}

	success := SCR_Screenshot(va("%s/%s", sshot_dir, filename))
	if success != SSHOT_FAILED_QUIET {
		verb := "Couldn't write"
		if success == SSHOT_SUCCESS {
			verb = "Wrote"
		}
		Com_Printf("%s %s\n", verb, name)
	}
}

func SCR_RSShot_f() {
	success := SSHOT_FAILED

	if CL_IsUploading() {
		return // already one pending
	}

	if cls.state < ca_onserver {
		return // gotta be connected
	}

	if scr_allowsnap.value == 0 {
		MSG_WriteByte(&cls.netchan.message, clc_stringcmd)
		SZ_Print(&cls.netchan.message, "snap\n")
		Com_Printf("Refusing remote screen shot request.\n")
		return
	}

	Com_Printf("Remote screenshot requested.\n")

	filename := "classicq/temp/__rsshot__"

	width, height := 400, 300
	base := make([]byte, glwidth*glheight*3)
	pixels := make([]byte, width*height*3)

	GPU_ReadPixels(base, uint32(glwidth), uint32(glheight))
	Image_Resample(base, glwidth, glheight, pixels, width, height, 3, 0)
	if Image_WritePNG(filename, int(image_png_compression_level.value), pixels, -width, height) {
		success = SSHOT_SUCCESS
	} else {
		success = SSHOT_FAILED
	}

	if success == SSHOT_SUCCESS {
		Com_Printf("Sending screenshot to server...\n")
		CL_StartUpload(filename)
	}

	os.Remove(va("%s/%s", com_basedir, filename))
}

func SCR_CheckAutoScreenshot() {
	exts := []string{"pcx", "tga", "png", "jpg"}

	if scr_autosshot_countdown == 0 {
		return
	}
	scr_autosshot_countdown--
	if scr_autosshot_countdown != 0 {
		return
	}

	if cl.intermission == 0 {
		return
	}

	filename := auto_matchname
	for len(filename) > 0 && (filename[0] == '/' || filename[0] == '\\') {
		filename = filename[1:]
	}

	sshot_dir := scr_sshot_dir.string
	if sshot_dir == "" {
		sshot_dir = cls.gamedirfile
	}
	ext := SShot_ExtForFormat(SShot_FormatForName(filename))

	fullsavedname := va("%s/%s", sshot_dir, auto_matchname)
	num := Util_Extend_Filename(fullsavedname, exts)
	if num == -1 {
		Com_Printf("Error: no available filenames\n")
		return
	}

	savedname := strlcpy(fmt.Sprintf("%s_%03d%s", auto_matchname, num, ext), 2*MAX_OSPATH)
	fullsavedname = va("%s/%s", sshot_dir, savedname)

	if SCR_Screenshot(fullsavedname) == SSHOT_SUCCESS {
		Com_Printf("Match scoreboard saved to %s\n", savedname)
	}
}

func SCR_AutoScreenshot(matchname string) {
	if cl.intermission == 1 {
		scr_autosshot_countdown = vid.numpages
		auto_matchname = strlcpy(matchname, 2*MAX_OSPATH)
	}
}

/************************************ INIT ************************************/

func SCR_CvarInit() {
	Cvar_SetCurrentGroup(CVAR_GROUP_VIEW)
	Cvar_Register(&scr_fov)
	Cvar_Register(&scr_viewsize)

	Cvar_SetCurrentGroup(CVAR_GROUP_CONSOLE)
	Cvar_Register(&scr_consize)
	Cvar_Register(&scr_conspeed)
	Cvar_Register(&scr_printspeed)

	Cvar_SetCurrentGroup(CVAR_GROUP_OPENGL)
	Cvar_Register(&gl_triplebuffer)

	Cvar_SetCurrentGroup(CVAR_GROUP_SCREEN)
	Cvar_Register(&scr_showram)
	Cvar_Register(&scr_showturtle)
	Cvar_Register(&scr_showpause)
	Cvar_Register(&scr_centertime)

	Cvar_Register(&scr_clock_x)
	Cvar_Register(&scr_clock_y)
	Cvar_Register(&scr_clock)

	Cvar_Register(&scr_gameclock_x)
	Cvar_Register(&scr_gameclock_y)
	Cvar_Register(&scr_gameclock)

	Cvar_Register(&scr_democlock_x)
	Cvar_Register(&scr_democlock_y)
	Cvar_Register(&scr_democlock)

	Cvar_Register(&show_speed)
	Cvar_Register(&show_speed_x)
	Cvar_Register(&show_speed_y)

	Cvar_Register(&show_fps)
	Cvar_Register(&show_fps_x)
	Cvar_Register(&show_fps_y)

	Cvar_Register(&show_framestddev)
	Cvar_Register(&show_framestddev_x)
	Cvar_Register(&show_framestddev_y)

	Cvar_Register(&scr_autoid)
	Cvar_Register(&scr_coloredText)

	Cvar_SetCurrentGroup(CVAR_GROUP_SCREENSHOTS)
	Cvar_Register(&scr_allowsnap)
	Cvar_Register(&scr_sshot_format)
	Cvar_Register(&scr_sshot_dir)

	Cvar_ResetCurrentGroup()

	Cmd_AddCommand("screenshot", SCR_ScreenShot_f)
	Cmd_AddCommand("sizeup", SCR_SizeUp_f)
	Cmd_AddCommand("sizedown", SCR_SizeDown_f)
	Cmd_AddCommand("messagemode", Con_MessageMode_f)
	Cmd_AddCommand("messagemode2", Con_MessageMode2_f)
}

func SCR_Init() {
	rampic = Draw_LoadPicture("wad:ram", DRAW_LOADPICTURE_DUMMYFALLBACK)
	netpic = Draw_LoadPicture("wad:net", DRAW_LOADPICTURE_DUMMYFALLBACK)
	turtlepic = Draw_LoadPicture("wad:turtle", DRAW_LOADPICTURE_DUMMYFALLBACK)

	scr_initialized = true
}

func SCR_Shutdown() {
	if pausepic != nil {
		Draw_FreePicture(pausepic)
		pausepic = nil
	}

	if loadingpic != nil {
		Draw_FreePicture(loadingpic)
		loadingpic = nil
	}

	Draw_FreePicture(turtlepic)
	Draw_FreePicture(netpic)
	Draw_FreePicture(rampic)

	scr_initialized = false
}

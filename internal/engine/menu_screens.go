package engine

// menu.c, second half: single player, multiplayer, demos, setup, quit
// screen, the dispatchers and init. The server-side screens (load, save,
// game options) are outside CLIENTONLY and left out.

import (
	"fmt"
	"math"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"
)

//=============================================================================
/* SINGLE PLAYER MENU */

func M_Menu_SinglePlayer_f() {
	M_EnterMenu(m_singleplayer)
}

func M_SinglePlayer_Draw() {
	M_DrawPic(qplaquepic, 16, 4, 32, 144)
	M_DrawPic(ttl_sglpic, (320-128)/2, 4, 128, 24)
	//	M_DrawPic(sp_menupic, 72, 32, 232, 64);

	M_DrawTextBox(60, 10*8, 23, 4)
	M_PrintWhite(100, 12*8, "classicQ is for")
	M_PrintWhite(88, 13*8, "Internet play only")
}

func M_SinglePlayer_Key(key int) {
	switch key {
	case int(K_BACKSPACE), int(K_ESCAPE), int(K_ENTER):
		if key == int(K_BACKSPACE) {
			m_topmenu = m_none
		}
		M_LeaveMenu(m_main)
	}
}

//=============================================================================
/* MULTIPLAYER MENU */

func M_Menu_MultiPlayer_f() {
	M_EnterMenu(m_multiplayer)
}

func M_MultiPlayer_ServerBrowser() {
	key_dest = key_game
	m_state = m_none
	VID_SetMouseGrab(1)
	SB_Activate_f()
}

func M_MultiPlayer_Draw() {
	M_DrawPic(qplaquepic, 16, 4, 32, 144)
	M_DrawPic(p_multipic, (320-216)/2, 4, 216, 24)

	Menu_Draw(multiplayermenu)
}

//=============================================================================
/* DEMO MENU */

const MAX_DEMO_NAME = MAX_OSPATH
const MAX_DEMO_FILES = 2048
const DEMO_MAXLINES = 17

type direntry_type_t int

const (
	dt_file direntry_type_t = iota
	dt_dir
	dt_up
	dt_msg
)

type demo_sort_t int

const (
	ds_name demo_sort_t = iota
	ds_size
	ds_time
)

type direntry_t struct {
	type_ direntry_type_t
	name  string // "" is the NULL of C
	size  int
	time  time.Time
}

var demolist_data [MAX_DEMO_FILES]direntry_t
var demolist [MAX_DEMO_FILES]*direntry_t
var demolist_count int
var demo_currentdir string
var demo_prevdemo string

var last_demo_time float32 = 0

var demo_cursor = 0
var demo_base = 0

var demo_sorttype = ds_name
var demo_reversesort = false

func Demo_SortCompare(d1, d2 *direntry_t) int {
	var retval int
	var sign int

	if retval = int(d2.type_ - d1.type_); retval != 0 || d1.type_ > dt_dir {
		return retval
	}

	if d1.type_ == dt_dir {
		return Q_strcasecmp(d1.name, d2.name)
	}

	if demo_reversesort {
		sign = -1
	} else {
		sign = 1
	}

	switch demo_sorttype {
	case ds_name:
		return sign * Q_strcasecmp(d1.name, d2.name)
	case ds_size:
		return sign * (d1.size - d2.size)
	case ds_time:
		return -sign * d1.time.Compare(d2.time)
	default:
		Sys_Error("Demo_SortCompare: unknown demo_sorttype (%d)", demo_sorttype)
		return 0
	}
}

func Demo_SortDemos() {
	var i int

	last_demo_time = 0

	for i = 0; i < demolist_count; i++ {
		demolist[i] = &demolist_data[i]
	}

	sort.Slice(demolist[:demolist_count], func(a, b int) bool {
		return Demo_SortCompare(demolist[a], demolist[b]) < 0
	})
}

func Demo_PositionCursor() {
	var i int

	last_demo_time = 0
	demo_base = 0
	demo_cursor = 0

	if demo_prevdemo != "" {
		for i = 0; i < demolist_count; i++ {
			if demolist[i].name == demo_prevdemo {
				demo_cursor = i
				if demo_cursor >= DEMO_MAXLINES {
					demo_base += demo_cursor - (DEMO_MAXLINES - 1)
					demo_cursor = DEMO_MAXLINES - 1
				}
				break
			}
		}
	}
	demo_prevdemo = ""
}

// the Win32 scan also lists qwz and reads nFileSizeLow; the POSIX scan uses stat
func demoExtension(ext string) bool {
	if Q_strcasecmp(ext, ".qwd") == 0 || Q_strcasecmp(ext, ".mvd") == 0 {
		return true
	}
	return runtime.GOOS == "windows" && Q_strcasecmp(ext, ".qwz") == 0
}

func demoFileSize(size int64) int {
	if runtime.GOOS == "windows" {
		return int(uint32(size))
	}
	return int(size)
}

// one directory scan of Demo_ReadDirectory; the shipped data path dedupes by name
func demoScanDirectory(base string, dedupe bool) {
	var i, size int
	var type_ direntry_type_t
	var t time.Time
	var name string

	entries, err := os.ReadDir(base + demo_currentdir)
	if err != nil {
		return
	}

	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}

		if info.IsDir() {
			type_ = dt_dir
			size = 0
			t = time.Time{}
		} else {
			i = len(e.Name())
			if i < 5 || !demoExtension(e.Name()[i-4:]) {
				continue
			}
			type_ = dt_file
			size = demoFileSize(info.Size())
			t = info.ModTime()
		}

		name = strlcpy(e.Name(), MAX_DEMO_NAME)

		if dedupe {
			dup := false
			for j := 0; j < demolist_count; j++ {
				if demolist_data[j].name != "" && demolist_data[j].name == name {
					dup = true
					break
				}
			}
			if dup {
				continue
			}
		}

		if demolist_count == MAX_DEMO_FILES {
			break
		}

		demolist_data[demolist_count].name = name
		demolist_data[demolist_count].type_ = type_
		demolist_data[demolist_count].size = size
		demolist_data[demolist_count].time = t
		demolist_count++
	}
}

func Demo_ReadDirectory() {
	var i int

	demolist_count = 0
	demo_base = 0
	demo_cursor = 0

	for i = 0; i < MAX_DEMO_FILES; i++ {
		demolist_data[i].name = ""
	}

	if demo_currentdir != "" {
		demolist_data[0].name = ".."
		demolist_data[0].type_ = dt_up
		demolist_count = 1
	}

	demoScanDirectory(com_basedir, false)

	// scan ro_data_path for shipped demos, dedupe by name
	if ro_data_path != "" && ro_data_path != com_basedir {
		demoScanDirectory(ro_data_path, true)
	}

	if demolist_count == 0 {
		demolist_data[0].name = "[ no files ]"
		demolist_data[0].type_ = dt_msg
		demolist_count = 1
	}

	Demo_SortDemos()
	Demo_PositionCursor()
}

var demo_currentdir_init = false

func M_Menu_Demos_f() {
	var s string

	M_EnterMenu(m_demos)

	if !demo_currentdir_init {
		demo_currentdir_init = true
		if demo_dir.string != "" {
			s = demo_dir.string
			for s != "" && (s[0] == '/' || s[0] == '\\') {
				s = s[1:]
			}
			if s != "" {
				demo_currentdir = "/"
				demo_currentdir = strlcat(demo_currentdir, s, MAX_OSPATH)

				for demo_currentdir[len(demo_currentdir)-1] == '/' || demo_currentdir[len(demo_currentdir)-1] == '\\' {
					demo_currentdir = demo_currentdir[:len(demo_currentdir)-1]
				}
			}
		} else {
			demo_currentdir = "/qw"
		}
	}

	Demo_ReadDirectory()
}

func Demo_FormatSize(t string) string {
	b := []byte(t)

	for i := range b {
		if b[i] >= '0' && b[i] <= '9' {
			b[i] = b[i] - '0' + 18
		} else {
			b[i] |= 128
		}
	}
	return string(b)
}

const DEMOLIST_NAME_WIDTH = 29

var last_demo_name string
var last_demo_index = 0
var last_demo_length = 0

func M_Demos_Draw() {
	var i, y int
	var d *direntry_t
	var demoname, demosize string

	var demoname_scroll string
	var demoindex, scroll_index int
	var frac, time_, elapsed float32

	M_Print(140, 8, "DEMOS")
	if demo_currentdir != "" {
		demoname_scroll = strlcpy(demo_currentdir, 38+1)
	} else {
		demoname_scroll = "/"
	}
	M_PrintWhite(16, 16, demoname_scroll)
	M_Print(8, 24, "\x1d\x1e\x1e\x1e\x1e\x1e\x1e\x1e\x1e\x1e\x1e\x1e\x1e\x1e\x1e\x1e\x1e\x1e\x1e\x1e\x1e\x1e\x1e\x1e\x1e\x1e\x1e\x1e\x1e\x1f \x1d\x1e\x1e\x1e\x1e\x1e\x1f")

	for i = 0; i < demolist_count-demo_base && i < DEMO_MAXLINES; i++ {
		d = demolist[demo_base+i]
		y = 32 + 8*i
		demoname = strlcpy(d.name, DEMOLIST_NAME_WIDTH)

		switch d.type_ {
		case dt_file:
			M_Print(24, y, demoname)
			if d.size > 99999*1024 {
				demosize = snprintf(36-DEMOLIST_NAME_WIDTH, "%5dM", d.size>>20)
			} else {
				demosize = snprintf(36-DEMOLIST_NAME_WIDTH, "%5dK", d.size>>10)
			}
			demosize = Demo_FormatSize(demosize)
			M_PrintWhite(24+8*DEMOLIST_NAME_WIDTH, y, demosize)
		case dt_dir:
			M_PrintWhite(24, y, demoname)
			M_PrintWhite(24+8*DEMOLIST_NAME_WIDTH, y, "folder")
		case dt_up:
			M_PrintWhite(24, y, demoname)
			M_PrintWhite(24+8*DEMOLIST_NAME_WIDTH, y, "    up")
		case dt_msg:
			M_Print(24, y, demoname)
		default:
			Sys_Error("M_Demos_Draw: unknown d->type (%d)", d.type_)
		}
	}

	M_DrawCharacter(8, 32+demo_cursor*8, 12+(int(curtime*4)&1))

	demoindex = demo_base + demo_cursor
	if demolist[demoindex].type_ == dt_file {
		time_ = float32(Sys_DoubleTime())
		if last_demo_time == 0 || last_demo_index != demoindex {
			last_demo_index = demoindex
			last_demo_time = time_
			frac = 0
			scroll_index = 0
			last_demo_name = snprintf(MAX_DEMO_NAME+7, "%s  ***  ", demolist[demoindex].name)
			last_demo_length = len(last_demo_name)
		} else {

			elapsed = float32(3.5 * math.Max(float64(time_-last_demo_time)-0.75, 0))
			scroll_index = int(elapsed)
			frac = bound(0, elapsed-float32(scroll_index), 1)
			scroll_index = scroll_index % last_demo_length
		}

		if last_demo_length <= 38+7 {
			demoname_scroll = strlcpy(demolist[demoindex].name, 38+1)
			M_PrintWhite(160-len(demoname_scroll)*4, 40+8*DEMO_MAXLINES, demoname_scroll)
		} else {
			b := make([]byte, 38)
			for i = 0; i < 38; i++ {
				b[i] = last_demo_name[(scroll_index+i)%last_demo_length]
			}
			demoname_scroll = string(b)
			M_PrintWhite(12-int(8*frac), 40+8*DEMO_MAXLINES, demoname_scroll)
		}
	} else {
		last_demo_time = 0
	}

}

func M_Demos_Key(key int) {
	var sort_target demo_sort_t

	switch key {
	case int(K_BACKSPACE), int(K_ESCAPE):
		if key == int(K_BACKSPACE) {
			m_topmenu = m_none
		}
		demo_prevdemo = strlcpy(demolist[demo_cursor+demo_base].name, MAX_DEMO_NAME)
		M_LeaveMenu(m_multiplayer)

	case int(K_UPARROW):
		S_LocalSound("misc/menu1.wav")
		if demo_cursor > 0 {
			demo_cursor--
		} else if demo_base > 0 {
			demo_base--
		}

	case int(K_DOWNARROW):
		S_LocalSound("misc/menu1.wav")
		if demo_cursor+demo_base < demolist_count-1 {
			if demo_cursor < DEMO_MAXLINES-1 {
				demo_cursor++
			} else {
				demo_base++
			}
		}

	case int(K_HOME):
		S_LocalSound("misc/menu1.wav")
		demo_cursor = 0
		demo_base = 0

	case int(K_END):
		S_LocalSound("misc/menu1.wav")
		if demolist_count > DEMO_MAXLINES {
			demo_cursor = DEMO_MAXLINES - 1
			demo_base = demolist_count - demo_cursor - 1
		} else {
			demo_base = 0
			demo_cursor = demolist_count - 1
		}

	case int(K_PGUP):
		S_LocalSound("misc/menu1.wav")
		demo_cursor -= DEMO_MAXLINES - 1
		if demo_cursor < 0 {
			demo_base += demo_cursor
			if demo_base < 0 {
				demo_base = 0
			}
			demo_cursor = 0
		}

	case int(K_PGDN):
		S_LocalSound("misc/menu1.wav")
		demo_cursor += DEMO_MAXLINES - 1
		if demo_base+demo_cursor >= demolist_count {
			demo_cursor = demolist_count - demo_base - 1
		}
		if demo_cursor >= DEMO_MAXLINES {
			demo_base += demo_cursor - (DEMO_MAXLINES - 1)
			demo_cursor = DEMO_MAXLINES - 1
			if demo_base+demo_cursor >= demolist_count {
				demo_base = demolist_count - demo_cursor - 1
			}
		}

	case int(K_ENTER):
		if demolist_count == 0 || demolist[demo_base+demo_cursor].type_ == dt_msg {
			break
		}

		if demolist[demo_base+demo_cursor].type_ != dt_file {
			if demolist[demo_base+demo_cursor].type_ == dt_up {
				if p := strings.LastIndexByte(demo_currentdir, '/'); p != -1 {
					demo_prevdemo = strlcpy(demo_currentdir[p+1:], MAX_DEMO_NAME)
					demo_currentdir = demo_currentdir[:p]
				}
			} else {
				demo_currentdir = strlcat(demo_currentdir, "/", MAX_OSPATH)
				demo_currentdir = strlcat(demo_currentdir, demolist[demo_base+demo_cursor].name, MAX_OSPATH)
			}
			demo_cursor = 0
			Demo_ReadDirectory()
		} else {
			key_dest = key_game
			m_state = m_none
			VID_SetMouseGrab(1)
			if keydown[K_CTRL] {
				Cbuf_AddText(va("timedemo \"..%s/%s\"\n", demo_currentdir, demolist[demo_cursor+demo_base].name))
			} else {
				Cbuf_AddText(va("playdemo \"..%s/%s\"\n", demo_currentdir, demolist[demo_cursor+demo_base].name))
			}
			demo_prevdemo = strlcpy(demolist[demo_base+demo_cursor].name, MAX_DEMO_NAME)
		}

	case 'n', 's', 't':
		if !keydown[K_CTRL] {
			break
		}

		if key == 'n' {
			sort_target = ds_name
		} else if key == 's' {
			sort_target = ds_size
		} else {
			sort_target = ds_time
		}
		if demo_sorttype == sort_target {
			demo_reversesort = !demo_reversesort
		} else {
			demo_sorttype = sort_target
			demo_reversesort = false
		}
		demo_prevdemo = strlcpy(demolist[demo_cursor+demo_base].name, MAX_DEMO_NAME)
		Demo_SortDemos()
		Demo_PositionCursor()

	case int(K_SPACE):
		demo_prevdemo = strlcpy(demolist[demo_cursor+demo_base].name, MAX_DEMO_NAME)
		Demo_ReadDirectory()
	}
}

//=============================================================================
/* SETUP MENU */

var setup_cursor = 0
var setup_cursor_table = []int{40, 64, 88, 112, 136, 172}

var setup_name string // char[16]
var setup_team string
var setup_name_short string
var setup_top int
var setup_bottom int

const NUM_SETUP_CMDS = 6

func M_Menu_Setup_f() {
	M_EnterMenu(m_setup)
	setup_name = strlcpy(name.string, 16)
	setup_team = strlcpy(team.string, 16)
	{
		src := name_short.string
		for src != "" && src[0] == ' ' {
			src = src[1:]
		}
		setup_name_short = strlcpy(src, 16)
		if len(setup_name_short) > 3 {
			setup_name_short = setup_name_short[:3]
		}
	}
	setup_top = int(topcolor.value)
	setup_bottom = int(bottomcolor.value)
}

func M_Setup_Draw() {
	M_DrawPic(qplaquepic, 16, 4, 32, 144)
	M_DrawPic(p_multipic, (320-216)/2, 4, 216, 24)

	M_Print(64, 40, "Your name")
	M_DrawTextBox(160, 32, 16, 1)
	M_PrintWhite(168, 40, setup_name)

	M_Print(64, 64, "Your team")
	M_DrawTextBox(160, 56, 16, 1)
	M_PrintWhite(168, 64, setup_team)

	M_Print(64, 88, "Short name")
	M_DrawTextBox(160, 80, 16, 1)
	M_PrintWhite(168, 88, setup_name_short)

	M_Print(64, 112, "Shirt color")
	M_Print(64, 136, "Pants color")

	M_DrawTextBox(64, 172-8, 14, 1)
	M_Print(72, 172, "Accept Changes")

	M_DrawPic(bigboxpic, 160, 96, 72, 72)

	{
		plyr := Draw_BuildTranslatedMenuplyr(setup_top*16, setup_bottom*16)

		if plyr != nil {
			M_DrawPic(plyr, 174, 106, 44, 52)
		} else {
			// no menuplyr.lmp - fall back to colour swatches
			M_DrawFill(172, 104, 48, 28, 16*setup_top+8)
			M_DrawFill(172, 132, 48, 28, 16*setup_bottom+8)
		}
	}

	M_DrawCharacter(56, setup_cursor_table[setup_cursor], 12+(int(curtime*4)&1))

	if setup_cursor == 0 {
		M_DrawCharacter(168+8*len(setup_name), setup_cursor_table[setup_cursor], 10+(int(curtime*4)&1))
	}

	if setup_cursor == 1 {
		M_DrawCharacter(168+8*len(setup_team), setup_cursor_table[setup_cursor], 10+(int(curtime*4)&1))
	}

	if setup_cursor == 2 {
		M_DrawCharacter(168+8*len(setup_name_short), setup_cursor_table[setup_cursor], 10+(int(curtime*4)&1))
	}
}

func M_Setup_Key(k int) {
	var l int

	switch k {
	case int(K_ESCAPE):
		M_LeaveMenu(m_multiplayer)

	case int(K_UPARROW):
		S_LocalSound("misc/menu1.wav")
		setup_cursor--
		if setup_cursor < 0 {
			setup_cursor = NUM_SETUP_CMDS - 1
		}

	case int(K_DOWNARROW):
		S_LocalSound("misc/menu1.wav")
		setup_cursor++
		if setup_cursor >= NUM_SETUP_CMDS {
			setup_cursor = 0
		}

	case int(K_HOME):
		S_LocalSound("misc/menu1.wav")
		setup_cursor = 0

	case int(K_END):
		S_LocalSound("misc/menu1.wav")
		setup_cursor = NUM_SETUP_CMDS - 1

	case int(K_LEFTARROW):
		if setup_cursor < 3 || setup_cursor > 4 {
			return
		}
		S_LocalSound("misc/menu3.wav")
		if setup_cursor == 3 {
			setup_top = setup_top - 1
		}
		if setup_cursor == 4 {
			setup_bottom = setup_bottom - 1
		}
	case int(K_RIGHTARROW):
		if setup_cursor < 3 || setup_cursor > 4 {
			return
		}
		S_LocalSound("misc/menu3.wav")
		if setup_cursor == 3 {
			setup_top = setup_top + 1
		}
		if setup_cursor == 4 {
			setup_bottom = setup_bottom + 1
		}

	case int(K_ENTER):
		Cvar_Set(&name, setup_name)
		Cvar_Set(&team, setup_team)
		Cvar_Set(&name_short, va("%3s", setup_name_short))
		Cvar_Set(&topcolor, va("%d", setup_top))
		Cvar_Set(&bottomcolor, va("%d", setup_bottom))
		m_entersound = true
		M_Menu_MultiPlayer_f()

	case int(K_BACKSPACE):
		if setup_cursor == 0 {
			if len(setup_name) != 0 {
				setup_name = setup_name[:len(setup_name)-1]
			}
		} else if setup_cursor == 1 {
			if len(setup_team) != 0 {
				setup_team = setup_team[:len(setup_team)-1]
			}
		} else if setup_cursor == 2 {
			if len(setup_name_short) != 0 {
				setup_name_short = setup_name_short[:len(setup_name_short)-1]
			}
		} else {
			m_topmenu = m_none
			M_LeaveMenu(m_multiplayer)
		}

	default:
		if k < 32 || k > 127 {
			break
		}
		if setup_cursor == 0 {
			l = len(setup_name)
			if l < 15 {
				setup_name += string(byte(k))
			}
		}
		if setup_cursor == 1 {
			l = len(setup_team)
			if l < 4 {
				setup_team += string(byte(k))
			}
		}
		if setup_cursor == 2 {
			l = len(setup_name_short)
			if l < 3 {
				setup_name_short += string(byte(k))
			}
		}
	}

	if setup_top > 13 {
		setup_top = 0
	}
	if setup_top < 0 {
		setup_top = 13
	}
	if setup_bottom > 13 {
		setup_bottom = 0
	}
	if setup_bottom < 0 {
		setup_bottom = 13
	}
}

var quitmsg = []string{
	"0classicQ v" + CLASSICQ_VERSION,
	"0",
	"1Programming:",
	"0David Walton",
	"0Florian Zwoch",
	"0Jacek Piszczek",
	"0Jonny Tornbom",
	"0Juergen Legler",
	"0Mark Olsen",
	"0Morten Bojsen-Hansen",
	"0",
	"1Artwork:",
	"0Marcin Kornas",
	"0",
	"1Based on:",
	"0FuhQuake by A Nourai",
	"0ZQuake by Anton Gavrilov",
	"0Quakeworld by Id Software",
	"0",
	"1Press 'y' to exit classicQ",
}

func M_Quit_Draw() {
	var x, y int

	M_DrawTextBox(0, 11, 38, 21)
	y = 24
	for _, p := range quitmsg {
		x = 18 + (36-len(p[1:]))*4
		if p[0] == '0' {
			M_PrintWhite(x, y, p[1:])
		} else {
			M_Print(x, y, p[1:])
		}
		y += 8
	}
}

//=============================================================================
/* Menu Subsystem */

func M_CvarInit() {
	Cvar_SetCurrentGroup(CVAR_GROUP_SCREEN)
	Cvar_Register(&scr_centerMenu)
	Cvar_Register(&scr_scaleMenu)

	Cvar_ResetCurrentGroup()

	Cmd_AddCommand("togglemenu", M_ToggleMenu_f)

	Cmd_AddCommand("menu_main", M_Menu_Main_f)
	Cmd_AddCommand("menu_multiplayer", M_Menu_MultiPlayer_f)
	Cmd_AddCommand("menu_setup", M_Menu_Setup_f)
	Cmd_AddCommand("menu_demos", M_Menu_Demos_f)
	Cmd_AddCommand("menu_options", M_Menu_Options_f)
	Cmd_AddCommand("menu_keys", M_Menu_Keys_f)
	Cmd_AddCommand("menu_fps", M_Menu_Fps_f)
	Cmd_AddCommand("menu_video", M_Menu_Video_f)
	Cmd_AddCommand("help", M_Menu_Help_f)
	Cmd_AddCommand("menu_help", M_Menu_Help_f)
	Cmd_AddCommand("menu_quit", M_Menu_Quit_f)
}

func M_Draw() {
	var oldconwidth int
	var oldconheight int

	if m_state == m_none || key_dest != key_menu {
		return
	}

	if !m_recursiveDraw {
		if SCR_NEED_CONSOLE_BACKGROUND() {
			Draw_ConsoleBackground(int(scr_con_current))
		} else {
			Draw_FadeScreen()
		}
	} else {
		m_recursiveDraw = false
	}

	if scr_scaleMenu.value != 0 {
		oldconwidth = int(vid.conwidth)
		oldconheight = int(vid.conheight)

		if float64(vid.conwidth)/320 > float64(vid.conheight)/200 {
			menuwidth = int(float64(vid.conwidth) * (200.0 / float64(vid.conheight)))
			menuheight = 200
		} else {
			menuwidth = 320
			menuheight = int(float64(vid.conheight) * (320.0 / float64(vid.conwidth)))
		}

		vid.conwidth = uint32(menuwidth)
		vid.conheight = uint32(menuheight)

		Draw_SetSize(uint(menuwidth), uint(menuheight))
	} else {
		menuwidth = int(vid.conwidth)
		menuheight = int(vid.conheight)
	}

	if scr_centerMenu.value != 0 {
		m_yofs = (menuheight-200)/2 - 16
	} else {
		m_yofs = -16
	}

	switch m_state {
	case m_none:

	case m_main:
		M_Main_Draw()

	case m_singleplayer:
		M_SinglePlayer_Draw()

	case m_multiplayer:
		M_MultiPlayer_Draw()

	case m_setup:
		M_Setup_Draw()

	case m_options:
		M_Options_Draw()

	case m_keys:
		M_Keys_Draw()

	case m_teamcomms:
		M_TeamComms_Draw()

	case m_fps:
		M_Fps_Draw()

	case m_video:
		M_Video_Draw()

	case m_video_verify:
		M_Video_Verify_Draw()

	case m_help:
		M_Help_Draw()

	case m_quit:
		M_Quit_Draw()

	case m_demos:
		M_Demos_Draw()
	}

	if scr_scaleMenu.value != 0 {
		vid.conwidth = uint32(oldconwidth)
		vid.conheight = uint32(oldconheight)

		Draw_SetSize(uint(vid.conwidth), uint(vid.conheight))
	}

	if m_entersound {
		S_LocalSound("misc/menu2.wav")
		m_entersound = false
	}
}

func M_Keydown(key int) {
	switch m_state {
	case m_none:
		return

	case m_main:
		M_Main_Key(key)
		return

	case m_singleplayer:
		M_SinglePlayer_Key(key)
		return

	case m_multiplayer:
		Menu_HandleKey(multiplayermenu, key)
		return

	case m_setup:
		M_Setup_Key(key)
		return

	case m_options:
		Menu_HandleKey(optionsmenu, key)
		return

	case m_keys:
		M_Keys_Key(key)
		return

	case m_teamcomms:
		M_TeamComms_Key(key)
		return

	case m_fps:
		Menu_HandleKey(fpsmenu, key)
		return

	case m_video:
		M_Video_Key(key)
		return

	case m_video_verify:
		M_Video_Verify_Key(key)
		return

	case m_help:
		M_Help_Key(key)
		return

	case m_quit:
		M_Quit_Key(key)
		return

	case m_demos:
		M_Demos_Key(key)
	}
}

func M_PerFramePreRender() {
	if m_state == m_video_verify {
		if curtime >= video_verify_fail_time {
			M_Menu_Video_Verify_Revert()
			M_Menu_Video_Verify_Cleanup()
			M_LeaveMenu(m_video)
		}
	}
}

var maxfps_presets = []int{77, 154, 231, 308, 385, 462, 616, 770, 924, 1001}

func M_Init() {
	optionsmenu = Menu_Create(1, m_main)
	if optionsmenu != nil {
		Menu_AddItem(optionsmenu, MenuItemButton_Create("Customize controls", 0, M_Menu_Keys_f))
		Menu_AddItem(optionsmenu, MenuItemButton_Create("Customize teamplay", 0, M_Menu_TeamComms_f))
		Menu_AddItem(optionsmenu, MenuItemButton_Create("Go to console", 0, M_Options_GoToConsole))
		Menu_AddItem(optionsmenu, MenuItemSpacer_Create())
		Menu_AddItem(optionsmenu, MenuItemButton_Create("Reset to defaults", 0, M_Options_ResetToDefaults))
		Menu_AddItem(optionsmenu, MenuItemButton_Create("Save configuration", 1, M_Options_SaveConfiguration))
		Menu_AddItem(optionsmenu, MenuItemSpacer_Create())
		Menu_AddItem(optionsmenu, MenuItemCvarRange_Create("Sound volume", 0, 0.5, 0.05, &s_volume))
		Menu_AddItem(optionsmenu, MenuItemSpacer_Create())
		Menu_AddItem(optionsmenu, MenuItemCvarPosNegBoolean_Create("Invert mouse", &m_pitch))
		Menu_AddItem(optionsmenu, MenuItemCvarRange_Create("Mouse speed", 1, 11, 0.5, &sensitivity))
		Menu_AddItem(optionsmenu, MenuItemSpacer_Create())
		Menu_AddItem(optionsmenu, MenuItemCvarRange_Create("In-game view size", 30, 100, 10, &scr_viewsize))
		Menu_AddItem(optionsmenu, MenuItemCvarRange_Create("Gamma", 0.5, 1, -0.05, &v_gamma))
		Menu_AddItem(optionsmenu, MenuItemCvarRange_Create("Contrast", 1, 2, 0.1, &v_contrast))
		Menu_AddItem(optionsmenu, MenuItemCvarValuePreset_Create("fps limit", &cl_maxfps, maxfps_presets, uint(len(maxfps_presets))))
		Menu_AddItem(optionsmenu, MenuItemCvarBoolean_Create("CRT phosphor", &r_crt_phosphor, 0))
		Menu_AddItem(optionsmenu, MenuItemCvarBoolean_Create("CRT dot & bloom", &r_crt_dotbloom, 0))
		Menu_AddItem(optionsmenu, MenuItemButton_Create("Video modes", 0, M_Menu_Video_f))

		optionsmenu_usemouse = MenuItemCvarBoolean_Create("Use mouse", &in_grab_windowed_mouse, 0)
		Menu_AddItem(optionsmenu, optionsmenu_usemouse)

		Menu_Layout(optionsmenu)
	}

	/* FPS menu */
	fpsmenu = Menu_Create(1, m_options)
	if fpsmenu != nil {
		Menu_AddItem(fpsmenu, MenuItemCvarMultiSelect_Create("Explosions", &r_explosiontype, explosiontypevalues))
		Menu_AddItem(fpsmenu, MenuItemCvarMultiSelect_Create("Muzzleflashes", &cl_muzzleflash, muzzleflashvalues))
		Menu_AddItem(fpsmenu, MenuItemCvarBoolean_Create("Gib filter", &cl_gibfilter, 0))
		Menu_AddItem(fpsmenu, MenuItemCvarMultiSelect_Create("Dead body filter", &cl_deadbodyfilter, deadbodyfiltervalues))
		Menu_AddItem(fpsmenu, MenuItemCvarMultiSelect_Create("Rocket model", &cl_rocket2grenade, rocket2grenadevalues))
		Menu_AddItem(fpsmenu, MenuItemCvarMultiSelect_Create("Rocket trail", &r_rockettrail, rockettrailvalues))
		Menu_AddItem(fpsmenu, MenuItemCvarBoolean_Create("Rocket light", &r_rocketlight, 0))
		Menu_AddItem(fpsmenu, MenuItemCvarBoolean_Create("Damage filter", &v_damagecshift, 1))
		Menu_AddItem(fpsmenu, MenuItemCvarBoolean_Create("Pickup flashes", &v_bonusflash, 0))
		Menu_AddItem(fpsmenu, MenuItemCvarMultiSelect_Create("Powerup glow", &r_powerupglow, powerupglowvalues))
		Menu_AddItem(fpsmenu, MenuItemCvarBoolean_Create("Draw torches", &r_drawflame, 0))
		Menu_AddItem(fpsmenu, MenuItemCvarBoolean_Create("Fast sky", &r_fastsky, 0))
		Menu_AddItem(fpsmenu, MenuItemCvarPosNegBoolean_Create("Fast lights", &gl_flashblend))

		Menu_AddItem(fpsmenu, MenuItemSpacer_Create())

		Menu_AddItem(fpsmenu, MenuItemButton_Create("Fast mode", 1, M_Fps_FastMode))
		Menu_AddItem(fpsmenu, MenuItemButton_Create("High quality", 1, M_Fps_HighQuality))

		Menu_Layout(fpsmenu)
	}

	/* Multiplayer menu */
	multiplayermenu = Menu_Create(0, m_main)
	if multiplayermenu != nil {
		Menu_AddItem(multiplayermenu, MenuItemButton_Create("Server Browser", 0, M_MultiPlayer_ServerBrowser))
		Menu_AddItem(multiplayermenu, MenuItemButton_Create("Player Setup", 0, M_Menu_Setup_f))
		Menu_AddItem(multiplayermenu, MenuItemButton_Create("Demos", 0, M_Menu_Demos_f))

		Menu_Layout(multiplayermenu)
	}
}

func M_Shutdown() {
	Menu_Delete(optionsmenu)
	optionsmenu = nil

	Menu_Delete(fpsmenu)
	fpsmenu = nil

	Menu_Delete(multiplayermenu)
	multiplayermenu = nil
}

func M_VidInit() {
	var i uint
	var buf string

	qplaquepic = Draw_LoadPicture("gfx/qplaque.lmp", DRAW_LOADPICTURE_DUMMYFALLBACK)
	ttl_mainpic = Draw_LoadPicture("gfx/ttl_main.lmp", DRAW_LOADPICTURE_DUMMYFALLBACK)
	mainmenupic = Draw_LoadPicture("gfx/mainmenu.lmp", DRAW_LOADPICTURE_DUMMYFALLBACK)

	for i = 0; i < 6; i++ {
		buf = fmt.Sprintf("gfx/menudot%d.lmp", i+1)
		menudotpic[i] = Draw_LoadPicture(buf, DRAW_LOADPICTURE_DUMMYFALLBACK)
	}

	p_optionpic = Draw_LoadPicture("gfx/p_option.lmp", DRAW_LOADPICTURE_DUMMYFALLBACK)

	ttl_cstmpic = Draw_LoadPicture("gfx/ttl_cstm.lmp", DRAW_LOADPICTURE_DUMMYFALLBACK)

	vidmodespic = Draw_LoadPicture("gfx/vidmodes.lmp", DRAW_LOADPICTURE_DUMMYFALLBACK)

	for i = 0; i < NUM_HELP_PAGES; i++ {
		buf = fmt.Sprintf("gfx/help%d.lmp", i)
		helppic[i] = Draw_LoadPicture(buf, DRAW_LOADPICTURE_DUMMYFALLBACK)
	}

	ttl_sglpic = Draw_LoadPicture("gfx/ttl_sgl.lmp", DRAW_LOADPICTURE_DUMMYFALLBACK)

	sp_menupic = Draw_LoadPicture("gfx/sp_menu.lmp", DRAW_LOADPICTURE_DUMMYFALLBACK)

	p_loadpic = Draw_LoadPicture("gfx/p_load.lmp", DRAW_LOADPICTURE_DUMMYFALLBACK)

	p_savepic = Draw_LoadPicture("gfx/p_save.lmp", DRAW_LOADPICTURE_DUMMYFALLBACK)

	p_multipic = Draw_LoadPicture("gfx/p_multi.lmp", DRAW_LOADPICTURE_DUMMYFALLBACK)

	bigboxpic = Draw_LoadPicture("gfx/bigbox.lmp", DRAW_LOADPICTURE_DUMMYFALLBACK)

	menuplyrpic = Draw_LoadPicture("gfx/menuplyr.lmp", DRAW_LOADPICTURE_DUMMYFALLBACK)
}

func M_VidShutdown() {
	var i uint

	Draw_FreePicture(menuplyrpic)

	Draw_FreePicture(bigboxpic)

	Draw_FreePicture(p_multipic)

	Draw_FreePicture(p_savepic)

	Draw_FreePicture(p_loadpic)

	Draw_FreePicture(sp_menupic)

	Draw_FreePicture(ttl_sglpic)

	for i = 0; i < NUM_HELP_PAGES; i++ {
		Draw_FreePicture(helppic[i])
	}

	Draw_FreePicture(vidmodespic)

	Draw_FreePicture(ttl_cstmpic)

	Draw_FreePicture(p_optionpic)

	for i = 0; i < 6; i++ {
		Draw_FreePicture(menudotpic[i])
	}

	Draw_FreePicture(mainmenupic)
	Draw_FreePicture(ttl_mainpic)
	Draw_FreePicture(qplaquepic)
}

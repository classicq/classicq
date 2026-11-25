package engine

import (
	"fmt"
	"sync"
)

// vid.h
const (
	VID_CBITS  = 6
	VID_GRADES = 1 << VID_CBITS
)

type vrect_t struct {
	x, y, width, height int
}

type viddef_t struct {
	colormap      []byte // 256 * VID_GRADES size
	displaywidth  uint32
	displayheight uint32
	aspect        float32 // width / height -- < 0 is taller than wide
	numpages      int
	recalc_refdef int // if true, recalc vid-based stuff
	conwidth      uint32
	conheight     uint32
}

var vid viddef_t // global video state

// vid.c

var display_mutex sync.Mutex

var display *sdldisplay

var windowtitle string

var vid_restarted int

var mouse_grab_wanted int
var mouse_grabbed int

var conwidth_user_set bool
var conheight_user_set bool

func vid_conwidth_callback(v *cvar_t, value string) bool {
	v.value = Q_atof(value)

	conwidth_user_set = v.value > 0.5

	set_up_conwidth_conheight()

	return false
}

func vid_conheight_callback(v *cvar_t, value string) bool {
	v.value = Q_atof(value)

	conheight_user_set = v.value > 0.5

	set_up_conwidth_conheight()

	return false
}

func in_grab_windowed_mouse_callback(v *cvar_t, value string) bool {
	v.value = float32(atof(value))

	refresh_mouse_grab_state()

	return false
}

var vid_ref = cvar_t{name: "vid_ref", string: "gl", flags: CVAR_ROM}

var vid_fullscreen = cvar_t{name: "vid_fullscreen", string: "1", flags: CVAR_ARCHIVE}
var vid_width = cvar_t{name: "vid_width", string: "640", flags: CVAR_ARCHIVE}
var vid_height = cvar_t{name: "vid_height", string: "480", flags: CVAR_ARCHIVE}
var vid_mode = cvar_t{name: "vid_mode", string: "", flags: CVAR_ARCHIVE}

var vid_conwidth = cvar_t{name: "vid_conwidth", string: "0", flags: CVAR_ARCHIVE}
var vid_conheight = cvar_t{name: "vid_conheight", string: "0", flags: CVAR_ARCHIVE}

var in_grab_windowed_mouse = cvar_t{name: "in_grab_windowed_mouse", string: "0", flags: CVAR_ARCHIVE}

// the callbacks read their own cvars, which Go rejects as an
// initialization cycle in the literals above
func init() {
	vid_conwidth.OnChange = vid_conwidth_callback
	vid_conheight.OnChange = vid_conheight_callback
	in_grab_windowed_mouse.OnChange = in_grab_windowed_mouse_callback
}

var vid_pal [768]byte

func set_up_conwidth_conheight() {
	var N int
	var auto_w, auto_h int

	if display != nil {
		vid.displaywidth = uint32(Sys_Video_GetWidth(display))
		vid.displayheight = uint32(Sys_Video_GetHeight(display))
	} else {
		vid.displaywidth = 320
		vid.displayheight = 240
	}

	// integer multiplier; >=720p uses N >= 2, lower keeps native raster
	if vid.displayheight >= 720 {
		N = (int(vid.displayheight) + 180) / 360
		if N < 2 {
			N = 2
		}
		auto_w = int(vid.displaywidth) / N
		auto_h = int(vid.displayheight) / N
	} else {
		auto_w = int(vid.displaywidth)
		auto_h = int(vid.displayheight)
	}

	if conwidth_user_set {
		conwidth := int(vid_conwidth.value)
		conwidth &^= 7
		if conwidth < 320 {
			conwidth = 320
		}
		vid.conwidth = uint32(conwidth)
	} else {
		vid.conwidth = uint32(auto_w)
	}

	if conheight_user_set {
		conheight := int(vid_conheight.value)
		if conheight < 200 {
			conheight = 200
		}
		vid.conheight = uint32(conheight)
	} else if conwidth_user_set {
		vid.conheight = uint32(int(vid.conwidth) * int(vid.displayheight) / int(vid.displaywidth))
	} else {
		vid.conheight = uint32(auto_h)
	}

	if int(vid.conwidth) > int(vid.displaywidth) {
		vid.conwidth = vid.displaywidth
	}

	if int(vid.conheight) > int(vid.displayheight) {
		vid.conheight = vid.displayheight
	}

	// store applied auto values as cvar defaultvalue so cfg_save skips them
	if !conwidth_user_set {
		buf := fmt.Sprintf("%d", vid.conwidth)
		if vid_conwidth.string != buf {
			vid_conwidth.string = buf
			vid_conwidth.value = float32(vid.conwidth)
		}
		if vid_conwidth.defaultvalue != buf {
			vid_conwidth.defaultvalue = buf
		}
	}
	if !conheight_user_set {
		buf := fmt.Sprintf("%d", vid.conheight)
		if vid_conheight.string != buf {
			vid_conheight.string = buf
			vid_conheight.value = float32(vid.conheight)
		}
		if vid_conheight.defaultvalue != buf {
			vid_conheight.defaultvalue = buf
		}
	}

	vid.recalc_refdef = 1
}

// The console size and the refdef were only computed when the window
// opened (vid.c carries a #warning there), so a resized window stretched the
// picture to its new proportions. Called once per frame.
func VID_CheckResize() {
	if display == nil {
		return
	}

	w := uint32(Sys_Video_GetWidth(display))
	h := uint32(Sys_Video_GetHeight(display))
	if w == 0 || h == 0 || (w == vid.displaywidth && h == vid.displayheight) {
		return
	}

	set_up_conwidth_conheight()
}

func refresh_mouse_grab_state() {
	if display == nil {
		return
	}

	newstate := 0

	if Sys_Video_GetFullscreen(display) || (mouse_grab_wanted != 0 && in_grab_windowed_mouse.value != 0) {
		newstate = 1
	}

	if newstate != mouse_grabbed {
		mouse_grabbed = newstate
		Sys_Video_GrabMouse(display, mouse_grabbed)
	}
}

func VID_Init(palette []byte) {
	copy(vid_pal[:], palette)

	if Sys_Video_Init() == 0 {
		Sys_Error("Sys_Video_Init() failed")
	}
}

func VID_Shutdown() {
	VID_Close()

	Sys_Video_Shutdown()

	windowtitle = ""
}

func VID_Restart() {
	if display == nil {
		return
	}

	VID_Close()
	VID_Open()

	for i := 1; i < MAX_MODELS; i++ {
		if cl.model_name[i] == "" {
			break
		}

		cl.model_precache[i] = Mod_ForName(cl.model_name[i], false)
		if cl.model_precache[i] == nil {
			Com_Printf("Unable to reload model '%s'.\n", cl.model_name[i])
			Host_EndGame()
			return
		}
	}

	if cl.model_precache[1] != nil {
		cl.worldmodel = cl.model_precache[1]
		R_NewMap()
		R_DrawFlat_NewMap()
		CL_SetSolidEntities() // pmove.physents still point at the models Mod_ClearAll freed
	}

	CL_ClearTEnts() /* Not the prettiest, but the safest for now... */

	vid_restarted = 1
}

func VID_CvarInit() {
	Cvar_SetCurrentGroup(CVAR_GROUP_VIDEO)
	Cvar_Register(&vid_ref)
	Cvar_Register(&vid_fullscreen)
	Cvar_Register(&vid_width)
	Cvar_Register(&vid_height)
	Cvar_Register(&vid_mode)
	Cvar_Register(&vid_conwidth)
	Cvar_Register(&vid_conheight)

	Cvar_SetCurrentGroup(CVAR_GROUP_INPUT_MOUSE)
	Cvar_Register(&in_grab_windowed_mouse)
	Cmd_AddLegacyCommand("_windowed_mouse", "in_grab_windowed_mouse")
	Cvar_ResetCurrentGroup()

	Cmd_AddCommand("vid_restart", VID_Restart)

	Sys_Video_CvarInit()
}

func VID_Open() {
	fullscreen := int(vid_fullscreen.value)
	width := int(vid_width.value)
	height := int(vid_height.value)

	vid.colormap = host_colormap
	vid.aspect = float32(float64(float32(height)/float32(width)) * (320.0 / 240.0))

	display_mutex.Lock()
	display = Sys_Video_Open(vid_mode.string, uint(width), uint(height), fullscreen, host_basepal)
	display_mutex.Unlock()
	if display != nil {
		vid.numpages = int(Sys_Video_GetNumBuffers(display))

		set_up_conwidth_conheight()

		if windowtitle != "" {
			Sys_Video_SetWindowTitle(display, windowtitle)
		}

		mouse_grabbed = 2
		refresh_mouse_grab_state()

		R_Init()

		V_UpdatePalette(true)
		Check_Gamma(host_basepal)
		VID_SetPalette(host_basepal)

		vid.recalc_refdef = 1 // force a surface cache flush

		R_InitGL()
		R_Particles_TextureInit()

		Draw_Init()
		M_VidInit()
		Sbar_Init()
		SCR_Init()
		Skin_Init()

		return
	}

	Sys_Error("VID: Unable to open a display\n")
}

func VID_Close() {
	display_mutex.Lock()

	Skin_Shutdown()
	Mod_ClearAll()
	SCR_Shutdown()
	Sbar_Shutdown()
	M_VidShutdown()
	Draw_Shutdown()

	if display != nil {
		R_Shutdown()
		Sys_Video_Close(display)

		display = nil
	}

	display_mutex.Unlock()
}

func VID_BeginFrame() {
	Sys_Video_BeginFrame(display)
}

func VID_Update(rects *vrect_t) {
	Sys_Video_Update(display, rects)
}

func VID_GetKeyEvent(key *keynum_t, down *bool) bool {
	return Sys_Video_GetKeyEvent(display, key, down) != 0
}

func VID_GetMouseMovement(mousex, mousey *int) {
	display_mutex.Lock()

	if display != nil {
		Sys_Video_GetMouseMovement(display, mousex, mousey)
	} else {
		*mousex = 0
		*mousey = 0
	}

	display_mutex.Unlock()
}

func VID_SetMouseGrab(on int) {
	mouse_grab_wanted = b2i(on != 0)

	refresh_mouse_grab_state()
}

func VID_SetDeviceGammaRamp(ramps []uint16) {
	Sys_Video_SetGamma(display, ramps)
}

func VID_HWGammaSupported() bool {
	return Sys_Video_HWGammaSupported(display)
}

func VID_SetCaption(text string) {
	if display != nil {
		windowtitle = text

		Sys_Video_SetWindowTitle(display, text)
	}
}

// The clipboard text is copied out of SDL right away, so there is nothing
// to free afterwards.
func VID_GetClipboardText() string {
	return Sys_Video_GetClipboardText(display)
}

func VID_FreeClipboardText(text string) {
	Sys_Video_FreeClipboardText(display, text)
}

func VID_SetClipboardText(text string) {
	Sys_Video_SetClipboardText(display, text)
}

func VID_GetWidth() uint {
	return Sys_Video_GetWidth(display)
}

func VID_GetHeight() uint {
	return Sys_Video_GetHeight(display)
}

func VID_GetFullscreen() bool {
	return Sys_Video_GetFullscreen(display)
}

func VID_GetMode() string {
	if Sys_Video_GetFullscreen(display) {
		return Sys_Video_GetMode(display)
	}

	return ""
}

func VID_FocusChanged() bool {
	if vid_restarted != 0 {
		vid_restarted = 0
		return true
	}

	return Sys_Video_FocusChanged(display) != 0
}

func VID_LockBuffer() {
}

func VID_UnlockBuffer() {
}

// Copyright (C) 2026 classicQ contributors
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
	"runtime"
	"unsafe"

	"github.com/classicq/classicq/internal/sdl"
)

// vid_sdl.c, the SDL3 side of the video layer. With -nodisplay no window
// is opened and SDL is never called (cqconsole runs headless); the GPU
// layer then has no device and skips its work.

const KEYQ_SIZE = 256

func OnChange_vid_vsync(v *cvar_t, value string) bool {
	v.value = Q_atof(value)
	GPU_SetVsync(int(v.value))
	return false
}

var vid_vsync = cvar_t{name: "vid_vsync", string: "0", flags: CVAR_ARCHIVE, OnChange: OnChange_vid_vsync}

var vid_nodisplay bool

type keyqent struct {
	key  keynum_t
	down bool
}

type sdldisplay struct {
	window *sdl.Window

	width      uint
	height     uint
	dpi_scale  float32
	fullscreen int
	mode       string

	mouse_dx float32
	mouse_dy float32

	keyq      [KEYQ_SIZE]keyqent
	keyq_head uint
	keyq_tail uint

	focus_changed int
}

func sdl_keycode_to_quake(sym sdl.Keycode) keynum_t {
	switch sym {
	case sdl.SDLK_TAB:
		return K_TAB
	case sdl.SDLK_RETURN:
		return K_ENTER
	case sdl.SDLK_ESCAPE:
		return K_ESCAPE
	case sdl.SDLK_SPACE:
		return K_SPACE
	case sdl.SDLK_BACKSPACE:
		return K_BACKSPACE

	case sdl.SDLK_CAPSLOCK:
		return K_CAPSLOCK
	case sdl.SDLK_PRINTSCREEN:
		return K_PRINTSCR
	case sdl.SDLK_SCROLLLOCK:
		return K_SCRLCK
	case sdl.SDLK_PAUSE:
		return K_PAUSE

	case sdl.SDLK_UP:
		return K_UPARROW
	case sdl.SDLK_DOWN:
		return K_DOWNARROW
	case sdl.SDLK_LEFT:
		return K_LEFTARROW
	case sdl.SDLK_RIGHT:
		return K_RIGHTARROW

	case sdl.SDLK_LALT:
		return K_LALT
	case sdl.SDLK_RALT:
		return K_RALT
	case sdl.SDLK_LCTRL:
		return K_LCTRL
	case sdl.SDLK_RCTRL:
		return K_RCTRL
	case sdl.SDLK_LSHIFT:
		return K_LSHIFT
	case sdl.SDLK_RSHIFT:
		return K_RSHIFT

	case sdl.SDLK_F1:
		return K_F1
	case sdl.SDLK_F2:
		return K_F2
	case sdl.SDLK_F3:
		return K_F3
	case sdl.SDLK_F4:
		return K_F4
	case sdl.SDLK_F5:
		return K_F5
	case sdl.SDLK_F6:
		return K_F6
	case sdl.SDLK_F7:
		return K_F7
	case sdl.SDLK_F8:
		return K_F8
	case sdl.SDLK_F9:
		return K_F9
	case sdl.SDLK_F10:
		return K_F10
	case sdl.SDLK_F11:
		return K_F11
	case sdl.SDLK_F12:
		return K_F12

	case sdl.SDLK_INSERT:
		return K_INS
	case sdl.SDLK_DELETE:
		return K_DEL
	case sdl.SDLK_PAGEUP:
		return K_PGUP
	case sdl.SDLK_PAGEDOWN:
		return K_PGDN
	case sdl.SDLK_HOME:
		return K_HOME
	case sdl.SDLK_END:
		return K_END

	case sdl.SDLK_LGUI:
		return K_LWIN
	case sdl.SDLK_RGUI:
		return K_RWIN
	case sdl.SDLK_MENU:
		return K_MENU

	case sdl.SDLK_NUMLOCKCLEAR:
		return KP_NUMLOCK
	case sdl.SDLK_KP_DIVIDE:
		return KP_SLASH
	case sdl.SDLK_KP_MULTIPLY:
		return KP_STAR
	case sdl.SDLK_KP_MINUS:
		return KP_MINUS
	case sdl.SDLK_KP_PLUS:
		return KP_PLUS
	case sdl.SDLK_KP_ENTER:
		return KP_ENTER
	case sdl.SDLK_KP_PERIOD:
		return KP_DEL
	case sdl.SDLK_KP_0:
		return KP_INS
	case sdl.SDLK_KP_1:
		return KP_END
	case sdl.SDLK_KP_2:
		return KP_DOWNARROW
	case sdl.SDLK_KP_3:
		return KP_PGDN
	case sdl.SDLK_KP_4:
		return KP_LEFTARROW
	case sdl.SDLK_KP_5:
		return KP_5
	case sdl.SDLK_KP_6:
		return KP_RIGHTARROW
	case sdl.SDLK_KP_7:
		return KP_HOME
	case sdl.SDLK_KP_8:
		return KP_UPARROW
	case sdl.SDLK_KP_9:
		return KP_PGUP

	default:
		if sym >= 32 && sym < 127 {
			return keynum_t(sym)
		}
		return 0
	}
}

func push_key(d *sdldisplay, k keynum_t, down bool) {
	next := (d.keyq_head + 1) % KEYQ_SIZE
	if next == d.keyq_tail {
		return
	}
	d.keyq[d.keyq_head].key = k
	d.keyq[d.keyq_head].down = down
	d.keyq_head = next
}

func pump_events(d *sdldisplay) {
	var ev sdl.Event

	if d.window == nil {
		return
	}

	for sdl.PollEvent(&ev) {
		switch ev.Type() {
		case sdl.EVENT_QUIT:
			Sys_Quit()

		case sdl.EVENT_KEY_DOWN, sdl.EVENT_KEY_UP:
			var k keynum_t
			if ev.Key().Scancode == sdl.SCANCODE_GRAVE {
				k = '`'
			} else {
				k = sdl_keycode_to_quake(ev.Key().Key)
			}
			if k != 0 {
				push_key(d, k, ev.Type() == sdl.EVENT_KEY_DOWN)
			}

		case sdl.EVENT_MOUSE_BUTTON_DOWN, sdl.EVENT_MOUSE_BUTTON_UP:
			var k keynum_t
			switch int(ev.Button().Button) {
			case sdl.BUTTON_LEFT:
				k = K_MOUSE1
			case sdl.BUTTON_RIGHT:
				k = K_MOUSE2
			case sdl.BUTTON_MIDDLE:
				k = K_MOUSE3
			case sdl.BUTTON_X1:
				k = K_MOUSE4
			case sdl.BUTTON_X2:
				k = K_MOUSE5
			}
			if k != 0 {
				push_key(d, k, ev.Type() == sdl.EVENT_MOUSE_BUTTON_DOWN)
			}

		case sdl.EVENT_MOUSE_WHEEL:
			// integer_y folds fractional touchpad deltas into whole ticks
			ticks := int(ev.Wheel().IntegerY)
			k := K_MWHEELDOWN
			if ticks > 0 {
				k = K_MWHEELUP
			}
			if ticks < 0 {
				ticks = -ticks
			}
			for ; ticks > 0; ticks-- {
				push_key(d, k, true)
				push_key(d, k, false)
			}

		case sdl.EVENT_MOUSE_MOTION:
			d.mouse_dx += ev.Motion().Xrel
			d.mouse_dy += ev.Motion().Yrel

		case sdl.EVENT_WINDOW_FOCUS_GAINED, sdl.EVENT_WINDOW_FOCUS_LOST:
			d.focus_changed = 1

		case sdl.EVENT_WINDOW_RESIZED, sdl.EVENT_WINDOW_PIXEL_SIZE_CHANGED:
			var dw, dh int32
			sdl.GetWindowSizeInPixels(d.window, &dw, &dh)
			d.width = uint(dw)
			d.height = uint(dh)
		}
	}
}

func Sys_Video_CvarInit() {
	Cvar_Register(&vid_vsync)
}

func Sys_Video_Init() int {
	vid_nodisplay = COM_CheckParm("-nodisplay") != 0
	if vid_nodisplay {
		return 1
	}

	if sdl.WasInit(sdl.INIT_VIDEO) == 0 {
		if !sdl.InitSubSystem(sdl.INIT_VIDEO) {
			Com_Printf("SDL_InitSubSystem(VIDEO) failed: %s\n", sdl.GetError())
			return 0
		}
	}
	return 1
}

func Sys_Video_Shutdown() {
	GPU_ShutdownAll()

	if vid_nodisplay {
		return
	}

	if sdl.WasInit(sdl.INIT_VIDEO) != 0 {
		sdl.QuitSubSystem(sdl.INIT_VIDEO)
	}
}

func Sys_Video_Open(mode string, width, height uint, fullscreen int, palette []byte) *sdldisplay {
	d := &sdldisplay{}

	if vid_nodisplay {
		d.width = width
		d.height = height
		d.fullscreen = b2i(fullscreen != 0)
		d.dpi_scale = 1
		d.mode = fmt.Sprintf("%dx%d", d.width, d.height)
		return d
	}

	if width == 0 || height == 0 {
		dm := sdl.GetCurrentDisplayMode(sdl.GetPrimaryDisplay())
		if dm != nil {
			if width == 0 {
				width = uint(dm.W)
			}
			if height == 0 {
				height = uint(dm.H)
			}
		} else {
			if width == 0 {
				width = 640
			}
			if height == 0 {
				height = 480
			}
		}
	}

	flags := sdl.WINDOW_HIGH_PIXEL_DENSITY | sdl.WINDOW_HIDDEN
	if fullscreen != 0 {
		flags |= sdl.WINDOW_FULLSCREEN
	} else {
		flags |= sdl.WINDOW_RESIZABLE
	}

	d.window = sdl.CreateWindow("classicQ", int32(width), int32(height), flags)
	if d.window == nil {
		Com_Printf("SDL_CreateWindow failed: %s\n", sdl.GetError())
		return nil
	}
	sdl.SetWindowPosition(d.window, sdl.WINDOWPOS_CENTERED, sdl.WINDOWPOS_CENTERED)
	sdl.ShowWindow(d.window)

	if runtime.GOOS != "windows" {
		// Windows takes the icon from the exe resource; other platforms get the PNG
		var icon_w, icon_h uint
		icon_pixels := Image_LoadPNG(nil, "icons/256.png", 0, 0, &icon_w, &icon_h)
		if icon_pixels != nil {
			icon := sdl.CreateSurfaceFrom(int32(icon_w), int32(icon_h), sdl.PIXELFORMAT_RGBA32,
				unsafe.Pointer(&icon_pixels[0]), int32(icon_w*4))
			if icon != nil {
				sdl.SetWindowIcon(d.window, icon)
				sdl.DestroySurface(icon)
			}
			runtime.KeepAlive(icon_pixels)
		}
	}

	if GPU_Init(d.window, int(vid_vsync.value)) == 0 {
		sdl.DestroyWindow(d.window)
		return nil
	}

	w, h := int32(width), int32(height)
	sdl.GetWindowSizeInPixels(d.window, &w, &h)
	d.width = uint(w)
	d.height = uint(h)
	d.fullscreen = b2i(fullscreen != 0)

	{
		scale := sdl.GetWindowDisplayScale(d.window)
		density := sdl.GetWindowPixelDensity(d.window)

		if scale <= 0.0 {
			scale = 1.0
		}
		if density <= 0.0 {
			density = 1.0
		}

		if scale > density {
			d.dpi_scale = scale
		} else {
			d.dpi_scale = density
		}
		if d.dpi_scale < 1.0 {
			d.dpi_scale = 1.0
		}
	}

	if mode != "" {
		d.mode = strlcpy(mode, 64)
	} else {
		d.mode = fmt.Sprintf("%dx%d", d.width, d.height)
	}

	return d
}

func Sys_Video_Close(display *sdldisplay) {
	d := display
	if d == nil {
		return
	}

	GPU_Shutdown()
	if d.window != nil {
		sdl.DestroyWindow(d.window)
	}
}

func Sys_Video_GetNumBuffers(display *sdldisplay) uint {
	return 2
}

func Sys_Video_Update(display *sdldisplay, rects *vrect_t) {
	GPU_EndFrame()
}

func Sys_Video_GetKeyEvent(display *sdldisplay, keynum *keynum_t, down *bool) int {
	d := display
	if d == nil {
		return 0
	}

	if d.keyq_head == d.keyq_tail {
		pump_events(d)
	}

	if d.keyq_head == d.keyq_tail {
		return 0
	}

	*keynum = d.keyq[d.keyq_tail].key
	*down = d.keyq[d.keyq_tail].down
	d.keyq_tail = (d.keyq_tail + 1) % KEYQ_SIZE
	return 1
}

func Sys_Video_GetMouseMovement(display *sdldisplay, mousex, mousey *int) {
	d := display
	if d == nil {
		if mousex != nil {
			*mousex = 0
		}
		if mousey != nil {
			*mousey = 0
		}
		return
	}
	// report whole pixels, keep fractional remainder
	dx := int(d.mouse_dx)
	dy := int(d.mouse_dy)
	if mousex != nil {
		*mousex = dx
	}
	if mousey != nil {
		*mousey = dy
	}
	d.mouse_dx -= float32(dx)
	d.mouse_dy -= float32(dy)
}

func Sys_Video_GrabMouse(display *sdldisplay, dograb int) {
	d := display
	if d != nil && d.window != nil {
		sdl.SetWindowRelativeMouseMode(d.window, dograb != 0)
	}
}

func Sys_Video_SetWindowTitle(display *sdldisplay, text string) {
	d := display
	if d != nil && d.window != nil {
		sdl.SetWindowTitle(d.window, text)
	}
}

func Sys_Video_GetWidth(display *sdldisplay) uint {
	if display != nil {
		return display.width
	}
	return 0
}

func Sys_Video_GetHeight(display *sdldisplay) uint {
	if display != nil {
		return display.height
	}
	return 0
}

func Sys_Video_GetDPIScale(display *sdldisplay) float32 {
	if display != nil {
		return display.dpi_scale
	}
	return 1.0
}

func Sys_Video_GetFullscreen(display *sdldisplay) bool {
	return display != nil && display.fullscreen != 0
}

func Sys_Video_GetMode(display *sdldisplay) string {
	if display != nil {
		return display.mode
	}
	return ""
}

func Sys_Video_FocusChanged(display *sdldisplay) int {
	d := display
	if d == nil {
		return 0
	}
	v := d.focus_changed
	d.focus_changed = 0
	return v
}

func Sys_Video_BeginFrame(display *sdldisplay) {
	d := display
	if d != nil && d.window != nil {
		GPU_BeginFrame(uint32(d.width), uint32(d.height))
	}
}

func Sys_Video_SetGamma(display *sdldisplay, ramps []uint16) {
	// SDL3 dropped gamma ramps; shader gamma path covers it
}

func Sys_Video_HWGammaSupported(display *sdldisplay) bool {
	return false
}

// a NUL terminated string owned by SDL
func cStringFromSDL(p *byte) string {
	if p == nil {
		return ""
	}
	n := 0
	for *(*byte)(unsafe.Add(unsafe.Pointer(p), n)) != 0 {
		n++
	}
	return string(unsafe.Slice(p, n))
}

func Sys_Video_GetClipboardText(display *sdldisplay) string {
	if vid_nodisplay {
		return ""
	}
	p := sdl.GetClipboardText()
	text := cStringFromSDL(p)
	sdl.Free(unsafe.Pointer(p))
	return text
}

func Sys_Video_FreeClipboardText(display *sdldisplay, text string) {
}

func Sys_Video_SetClipboardText(display *sdldisplay, text string) {
	if vid_nodisplay {
		return
	}
	sdl.SetClipboardText(text)
}

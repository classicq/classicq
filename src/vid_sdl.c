/*
Copyright (C) 2026 classicQ contributors

This program is free software; you can redistribute it and/or
modify it under the terms of the GNU General Public License
as published by the Free Software Foundation; either version 2
of the License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.

See the GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program; if not, write to the Free Software
Foundation, Inc., 59 Temple Place - Suite 330, Boston, MA  02111-1307, USA.
*/

#include <stdlib.h>
#include <string.h>
#include <stdio.h>

#include <SDL3/SDL.h>

#include "quakedef.h"
#include "render.h"
#include "keys.h"
#include "sys_video.h"
#include "image.h"
#include "gpu_local.h"

#define KEYQ_SIZE 256

static cvar_t vid_vsync = { "vid_vsync", "0", CVAR_ARCHIVE };

struct sdldisplay
{
	SDL_Window *window;

	unsigned int width;
	unsigned int height;
	float dpi_scale;
	int fullscreen;
	char mode[64];

	float mouse_dx;
	float mouse_dy;

	struct keyqent
	{
		keynum_t key;
		qboolean down;
	} keyq[KEYQ_SIZE];
	unsigned int keyq_head;
	unsigned int keyq_tail;

	int focus_changed;
};

static keynum_t sdl_keycode_to_quake(SDL_Keycode sym)
{
	switch (sym)
	{
		case SDLK_TAB:          return K_TAB;
		case SDLK_RETURN:       return K_ENTER;
		case SDLK_ESCAPE:       return K_ESCAPE;
		case SDLK_SPACE:        return K_SPACE;
		case SDLK_BACKSPACE:    return K_BACKSPACE;

		case SDLK_CAPSLOCK:     return K_CAPSLOCK;
		case SDLK_PRINTSCREEN:  return K_PRINTSCR;
		case SDLK_SCROLLLOCK:   return K_SCRLCK;
		case SDLK_PAUSE:        return K_PAUSE;

		case SDLK_UP:           return K_UPARROW;
		case SDLK_DOWN:         return K_DOWNARROW;
		case SDLK_LEFT:         return K_LEFTARROW;
		case SDLK_RIGHT:        return K_RIGHTARROW;

		case SDLK_LALT:         return K_LALT;
		case SDLK_RALT:         return K_RALT;
		case SDLK_LCTRL:        return K_LCTRL;
		case SDLK_RCTRL:        return K_RCTRL;
		case SDLK_LSHIFT:       return K_LSHIFT;
		case SDLK_RSHIFT:       return K_RSHIFT;

		case SDLK_F1:           return K_F1;
		case SDLK_F2:           return K_F2;
		case SDLK_F3:           return K_F3;
		case SDLK_F4:           return K_F4;
		case SDLK_F5:           return K_F5;
		case SDLK_F6:           return K_F6;
		case SDLK_F7:           return K_F7;
		case SDLK_F8:           return K_F8;
		case SDLK_F9:           return K_F9;
		case SDLK_F10:          return K_F10;
		case SDLK_F11:          return K_F11;
		case SDLK_F12:          return K_F12;

		case SDLK_INSERT:       return K_INS;
		case SDLK_DELETE:       return K_DEL;
		case SDLK_PAGEUP:       return K_PGUP;
		case SDLK_PAGEDOWN:     return K_PGDN;
		case SDLK_HOME:         return K_HOME;
		case SDLK_END:          return K_END;

		case SDLK_LGUI:         return K_LWIN;
		case SDLK_RGUI:         return K_RWIN;
		case SDLK_MENU:         return K_MENU;

		case SDLK_NUMLOCKCLEAR: return KP_NUMLOCK;
		case SDLK_KP_DIVIDE:    return KP_SLASH;
		case SDLK_KP_MULTIPLY:  return KP_STAR;
		case SDLK_KP_MINUS:     return KP_MINUS;
		case SDLK_KP_PLUS:      return KP_PLUS;
		case SDLK_KP_ENTER:     return KP_ENTER;
		case SDLK_KP_PERIOD:    return KP_DEL;
		case SDLK_KP_0:         return KP_INS;
		case SDLK_KP_1:         return KP_END;
		case SDLK_KP_2:         return KP_DOWNARROW;
		case SDLK_KP_3:         return KP_PGDN;
		case SDLK_KP_4:         return KP_LEFTARROW;
		case SDLK_KP_5:         return KP_5;
		case SDLK_KP_6:         return KP_RIGHTARROW;
		case SDLK_KP_7:         return KP_HOME;
		case SDLK_KP_8:         return KP_UPARROW;
		case SDLK_KP_9:         return KP_PGUP;

		default:
			if (sym >= 32 && sym < 127)
				return (keynum_t)sym;
			return (keynum_t)0;
	}
}

static void push_key(struct sdldisplay *d, keynum_t k, qboolean down)
{
	unsigned int next = (d->keyq_head + 1) % KEYQ_SIZE;
	if (next == d->keyq_tail)
		return;
	d->keyq[d->keyq_head].key = k;
	d->keyq[d->keyq_head].down = down;
	d->keyq_head = next;
}

static void pump_events(struct sdldisplay *d)
{
	SDL_Event ev;

	while (SDL_PollEvent(&ev))
	{
		switch (ev.type)
		{
			case SDL_EVENT_QUIT:
				Sys_Quit();
				break;

			case SDL_EVENT_KEY_DOWN:
			case SDL_EVENT_KEY_UP:
			{
				keynum_t k;
				if (ev.key.scancode == SDL_SCANCODE_GRAVE)
					k = '`';
				else
					k = sdl_keycode_to_quake(ev.key.key);
				if (k)
					push_key(d, k, ev.type == SDL_EVENT_KEY_DOWN);
				break;
			}

			case SDL_EVENT_MOUSE_BUTTON_DOWN:
			case SDL_EVENT_MOUSE_BUTTON_UP:
			{
				keynum_t k = 0;
				switch (ev.button.button)
				{
				case SDL_BUTTON_LEFT:   k = K_MOUSE1; break;
				case SDL_BUTTON_RIGHT:  k = K_MOUSE2; break;
				case SDL_BUTTON_MIDDLE: k = K_MOUSE3; break;
				case SDL_BUTTON_X1:     k = K_MOUSE4; break;
				case SDL_BUTTON_X2:     k = K_MOUSE5; break;
				}
				if (k)
					push_key(d, k, ev.type == SDL_EVENT_MOUSE_BUTTON_DOWN);
				break;
			}

			case SDL_EVENT_MOUSE_WHEEL:
			{
				// integer_y folds fractional touchpad deltas into whole ticks
				int ticks = ev.wheel.integer_y;
				keynum_t k = (ticks > 0) ? K_MWHEELUP : K_MWHEELDOWN;
				if (ticks < 0)
					ticks = -ticks;
				while (ticks-- > 0)
				{
					push_key(d, k, true);
					push_key(d, k, false);
				}
				break;
			}

			case SDL_EVENT_MOUSE_MOTION:
				d->mouse_dx += ev.motion.xrel;
				d->mouse_dy += ev.motion.yrel;
				break;

			case SDL_EVENT_WINDOW_FOCUS_GAINED:
			case SDL_EVENT_WINDOW_FOCUS_LOST:
				d->focus_changed = 1;
				break;

			case SDL_EVENT_WINDOW_RESIZED:
			case SDL_EVENT_WINDOW_PIXEL_SIZE_CHANGED:
			{
				int dw = 0, dh = 0;
				SDL_GetWindowSizeInPixels(d->window, &dw, &dh);
				d->width  = (unsigned int)dw;
				d->height = (unsigned int)dh;
				break;
			}
		}
	}
}

void Sys_Video_CvarInit(void)
{
	Cvar_Register(&vid_vsync);
}

int Sys_Video_Init(void)
{
	if (SDL_WasInit(SDL_INIT_VIDEO) == 0)
	{
		if (!SDL_InitSubSystem(SDL_INIT_VIDEO))
		{
			Com_Printf("SDL_InitSubSystem(VIDEO) failed: %s\n", SDL_GetError());
			return 0;
		}
	}
	return 1;
}

void Sys_Video_Shutdown(void)
{
	if (SDL_WasInit(SDL_INIT_VIDEO))
		SDL_QuitSubSystem(SDL_INIT_VIDEO);
}

void *Sys_Video_Open(const char *mode, unsigned int width, unsigned int height, int fullscreen, unsigned char *palette)
{
	(void)palette;

	struct sdldisplay *d = calloc(1, sizeof(*d));
	if (!d)
		return NULL;

	if (width == 0 || height == 0)
	{
		const SDL_DisplayMode *dm = SDL_GetCurrentDisplayMode(SDL_GetPrimaryDisplay());
		if (dm)
		{
			if (width == 0)
				width = (unsigned int)dm->w;
			if (height == 0)
				height = (unsigned int)dm->h;
		}
		else
		{
			if (width == 0)  width = 640;
			if (height == 0) height = 480;
		}
	}

	SDL_WindowFlags flags = SDL_WINDOW_HIGH_PIXEL_DENSITY | SDL_WINDOW_HIDDEN;
	if (fullscreen)
		flags |= SDL_WINDOW_FULLSCREEN;
	else
		flags |= SDL_WINDOW_RESIZABLE;

	d->window = SDL_CreateWindow("classicQ", (int)width, (int)height, flags);
	if (!d->window)
	{
		Com_Printf("SDL_CreateWindow failed: %s\n", SDL_GetError());
		free(d);
		return NULL;
	}
	SDL_SetWindowPosition(d->window, SDL_WINDOWPOS_CENTERED, SDL_WINDOWPOS_CENTERED);
	SDL_ShowWindow(d->window);

#ifndef _WIN32
	// Windows .ico embedded via RC; other platforms get PNG fallback
	{
		byte *icon_pixels;
		unsigned int icon_w, icon_h;
		icon_pixels = Image_LoadPNG(NULL, "icons/256.png", 0, 0, &icon_w, &icon_h);
		if (icon_pixels)
		{
			SDL_Surface *icon = SDL_CreateSurfaceFrom(
				(int)icon_w, (int)icon_h, SDL_PIXELFORMAT_RGBA32,
				icon_pixels, (int)icon_w * 4);
			if (icon)
			{
				SDL_SetWindowIcon(d->window, icon);
				SDL_DestroySurface(icon);
			}
			free(icon_pixels);
		}
	}
#endif

	if (!GPU_Init(d->window, (int)vid_vsync.value))
	{
		SDL_DestroyWindow(d->window);
		free(d);
		return NULL;
	}

	int w = (int)width, h = (int)height;
	SDL_GetWindowSizeInPixels(d->window, &w, &h);
	d->width  = (unsigned int)w;
	d->height = (unsigned int)h;
	d->fullscreen = fullscreen ? 1 : 0;

	{
		float scale = SDL_GetWindowDisplayScale(d->window);
		float density = SDL_GetWindowPixelDensity(d->window);

		if (scale <= 0.0f)
			scale = 1.0f;
		if (density <= 0.0f)
			density = 1.0f;

		d->dpi_scale = (scale > density) ? scale : density;
		if (d->dpi_scale < 1.0f)
			d->dpi_scale = 1.0f;
	}

	if (mode && *mode)
	{
		strncpy(d->mode, mode, sizeof(d->mode) - 1);
		d->mode[sizeof(d->mode) - 1] = 0;
	}
	else
	{
		snprintf(d->mode, sizeof(d->mode), "%ux%u", d->width, d->height);
	}

	return d;
}

void Sys_Video_Close(void *display)
{
	struct sdldisplay *d = display;
	if (!d)
		return;

	GPU_Shutdown();
	if (d->window)
		SDL_DestroyWindow(d->window);

	free(d);
}

unsigned int Sys_Video_GetNumBuffers(void *display)
{
	(void)display;
	return 2;
}

void Sys_Video_Update(void *display, vrect_t *rects)
{
	(void)display;
	(void)rects;
	GPU_EndFrame();
}

int Sys_Video_GetKeyEvent(void *display, keynum_t *keynum, qboolean *down)
{
	struct sdldisplay *d = display;
	if (!d)
		return 0;

	if (d->keyq_head == d->keyq_tail)
		pump_events(d);

	if (d->keyq_head == d->keyq_tail)
		return 0;

	*keynum = d->keyq[d->keyq_tail].key;
	*down   = d->keyq[d->keyq_tail].down;
	d->keyq_tail = (d->keyq_tail + 1) % KEYQ_SIZE;
	return 1;
}

void Sys_Video_GetMouseMovement(void *display, int *mousex, int *mousey)
{
	struct sdldisplay *d = display;
	int dx, dy;
	if (!d)
	{
		if (mousex) *mousex = 0;
		if (mousey) *mousey = 0;
		return;
	}
	// report whole pixels, keep fractional remainder
	dx = (int)d->mouse_dx;
	dy = (int)d->mouse_dy;
	if (mousex) *mousex = dx;
	if (mousey) *mousey = dy;
	d->mouse_dx -= (float)dx;
	d->mouse_dy -= (float)dy;
}

void Sys_Video_GrabMouse(void *display, int dograb)
{
	struct sdldisplay *d = display;
	if (d && d->window)
		SDL_SetWindowRelativeMouseMode(d->window, dograb ? true : false);
}

void Sys_Video_SetWindowTitle(void *display, const char *text)
{
	struct sdldisplay *d = display;
	if (d && d->window && text)
		SDL_SetWindowTitle(d->window, text);
}

unsigned int Sys_Video_GetWidth(void *display)
{
	struct sdldisplay *d = display;
	return d ? d->width : 0;
}

unsigned int Sys_Video_GetHeight(void *display)
{
	struct sdldisplay *d = display;
	return d ? d->height : 0;
}

float Sys_Video_GetDPIScale(void *display)
{
	struct sdldisplay *d = display;
	return d ? d->dpi_scale : 1.0f;
}

qboolean Sys_Video_GetFullscreen(void *display)
{
	struct sdldisplay *d = display;
	return d ? d->fullscreen : 0;
}

const char *Sys_Video_GetMode(void *display)
{
	struct sdldisplay *d = display;
	return d ? d->mode : "";
}

int Sys_Video_FocusChanged(void *display)
{
	struct sdldisplay *d = display;
	int v;
	if (!d)
		return 0;
	v = d->focus_changed;
	d->focus_changed = 0;
	return v;
}

void Sys_Video_BeginFrame(void *display)
{
	struct sdldisplay *d = display;
	if (d && d->window)
		GPU_BeginFrame(d->width, d->height);
}

void Sys_Video_SetGamma(void *display, unsigned short *ramps)
{
	// SDL3 dropped gamma ramps; shader gamma path covers it
	(void)display;
	(void)ramps;
}

qboolean Sys_Video_HWGammaSupported(void *display)
{
	(void)display;
	return 0;
}

void *Sys_Video_GetProcAddress(void *display, const char *name)
{
	(void)display;
	(void)name;
	return NULL;
}

const char *Sys_Video_GetClipboardText(void *display)
{
	(void)display;
	return SDL_GetClipboardText();
}

void Sys_Video_FreeClipboardText(void *display, const char *text)
{
	(void)display;
	if (text)
		SDL_free((void *)text);
}

void Sys_Video_SetClipboardText(void *display, const char *text)
{
	(void)display;
	if (text)
		SDL_SetClipboardText(text);
}

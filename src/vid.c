/*
Copyright (C) 2006-2007 Mark Olsen

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

#include "quakedef.h"
#include "common.h"
#include "r_local.h"
#include "r_texture.h"

#include "menu.h"
#include "skin.h"

#include "sys_thread.h"

#include "sbar.h"

static struct SysMutex *display_mutex;

static void *display;

static char *windowtitle;

static void set_up_conwidth_conheight(void);
static void refresh_mouse_grab_state(void);

static int vid_restarted;

static int mouse_grab_wanted;
static int mouse_grabbed;

static qboolean conwidth_user_set;
static qboolean conheight_user_set;

static qboolean vid_conwidth_callback(cvar_t *var, char *value)
{
	var->value = Q_atof(value);

	conwidth_user_set = (var->value > 0.5f);

	set_up_conwidth_conheight();

	return false;
}

static qboolean vid_conheight_callback(cvar_t *var, char *value)
{
	var->value = Q_atof(value);

	conheight_user_set = (var->value > 0.5f);

	set_up_conwidth_conheight();

	return false;
}

static qboolean in_grab_windowed_mouse_callback(cvar_t *var, char *value)
{
	var->value = atof(value);

	refresh_mouse_grab_state();

	return false;
}

#warning Fixme
static cvar_t vid_ref = { "vid_ref", "gl", CVAR_ROM };

cvar_t vid_fullscreen = { "vid_fullscreen", "1", CVAR_ARCHIVE };
cvar_t vid_width = { "vid_width", "640", CVAR_ARCHIVE };
cvar_t vid_height = { "vid_height", "480", CVAR_ARCHIVE };
cvar_t vid_mode = { "vid_mode", "", CVAR_ARCHIVE };

cvar_t vid_conwidth = { "vid_conwidth", "0", CVAR_ARCHIVE, vid_conwidth_callback };
cvar_t vid_conheight = { "vid_conheight", "0", CVAR_ARCHIVE, vid_conheight_callback };

cvar_t in_grab_windowed_mouse = { "in_grab_windowed_mouse", "0", CVAR_ARCHIVE, in_grab_windowed_mouse_callback };

static unsigned char pal[768];


static void set_up_conwidth_conheight()
{
	int N;
	int auto_w, auto_h;
	char buf[32];

	if (display)
	{
		vid.displaywidth = Sys_Video_GetWidth(display);
		vid.displayheight = Sys_Video_GetHeight(display);
	}
	else
	{
		vid.displaywidth = 320;
		vid.displayheight = 240;
	}

	// integer multiplier; >=720p uses N >= 2, lower keeps native raster
	if (vid.displayheight >= 720)
	{
		N = ((int)vid.displayheight + 180) / 360;
		if (N < 2)
			N = 2;
		auto_w = (int)vid.displaywidth / N;
		auto_h = (int)vid.displayheight / N;
	}
	else
	{
		auto_w = (int)vid.displaywidth;
		auto_h = (int)vid.displayheight;
	}

	if (conwidth_user_set)
	{
		vid.conwidth = (int)vid_conwidth.value;
		vid.conwidth &= ~7;
		if (vid.conwidth < 320)
			vid.conwidth = 320;
	}
	else
	{
		vid.conwidth = auto_w;
	}

	if (conheight_user_set)
	{
		vid.conheight = (int)vid_conheight.value;
		if (vid.conheight < 200)
			vid.conheight = 200;
	}
	else if (conwidth_user_set)
	{
		vid.conheight = vid.conwidth * (int)vid.displayheight / (int)vid.displaywidth;
	}
	else
	{
		vid.conheight = auto_h;
	}

	if (vid.conwidth > (int)vid.displaywidth)
		vid.conwidth = (int)vid.displaywidth;

	if (vid.conheight > (int)vid.displayheight)
		vid.conheight = (int)vid.displayheight;

	// store applied auto values as cvar defaultvalue so cfg_save skips them
	if (!conwidth_user_set)
	{
		snprintf(buf, sizeof(buf), "%d", vid.conwidth);
		if (strcmp(vid_conwidth.string, buf) != 0)
		{
			Z_Free(vid_conwidth.string);
			vid_conwidth.string = CopyString(buf);
			vid_conwidth.value = (float)vid.conwidth;
		}
		if (strcmp(vid_conwidth.defaultvalue, buf) != 0)
		{
			Z_Free(vid_conwidth.defaultvalue);
			vid_conwidth.defaultvalue = CopyString(buf);
		}
	}
	if (!conheight_user_set)
	{
		snprintf(buf, sizeof(buf), "%d", vid.conheight);
		if (strcmp(vid_conheight.string, buf) != 0)
		{
			Z_Free(vid_conheight.string);
			vid_conheight.string = CopyString(buf);
			vid_conheight.value = (float)vid.conheight;
		}
		if (strcmp(vid_conheight.defaultvalue, buf) != 0)
		{
			Z_Free(vid_conheight.defaultvalue);
			vid_conheight.defaultvalue = CopyString(buf);
		}
	}

	vid.recalc_refdef = 1;
}

static void refresh_mouse_grab_state()
{
	int newstate;

	if (!display)
		return;

	newstate = 0;

	if (Sys_Video_GetFullscreen(display) || (mouse_grab_wanted && in_grab_windowed_mouse.value))
		newstate = 1;

	if (newstate != mouse_grabbed)
	{
		mouse_grabbed = newstate;
		Sys_Video_GrabMouse(display, mouse_grabbed);
	}
}

void VID_Init(unsigned char *palette)
{
	memcpy(pal, palette, sizeof(pal));

	display_mutex = Sys_Thread_CreateMutex();
	if (!display_mutex)
	{
		Sys_Error("Failed to create display mutex");
	}

	if (!Sys_Video_Init())
	{
		Sys_Error("Sys_Video_Init() failed");
	}
}

void VID_Shutdown()
{
	VID_Close();

	Sys_Video_Shutdown();

	Sys_Thread_DeleteMutex(display_mutex);

	free(windowtitle);
	windowtitle = 0;
}

void VID_Restart(void)
{
	int i;

	if (!display)
		return;

	VID_Close();
	VID_Open();

	for(i=1;i < MAX_MODELS;i++)
	{
		if (cl.model_name[i][0] == 0)
			break;

		cl.model_precache[i] = Mod_ForName(cl.model_name[i], false);
		if (!cl.model_precache[i])
		{
			Com_Printf("Unable to reload model '%s'.\n", cl.model_name[i]);
			Host_EndGame();
			return;
		}
	}

	if (cl.model_precache[1])
	{
		cl.worldmodel = cl.model_precache[1];
		R_NewMap();
		R_DrawFlat_NewMap();
	}

	CL_ClearTEnts(); /* Not the prettiest, but the safest for now... */

	vid_restarted = 1;
}

void VID_CvarInit()
{
	Cvar_SetCurrentGroup(CVAR_GROUP_VIDEO);
	Cvar_Register(&vid_ref);
	Cvar_Register(&vid_fullscreen);
	Cvar_Register(&vid_width);
	Cvar_Register(&vid_height);
	Cvar_Register(&vid_mode);
	Cvar_Register(&vid_conwidth);
	Cvar_Register(&vid_conheight);

	Cvar_SetCurrentGroup(CVAR_GROUP_INPUT_MOUSE);
	Cvar_Register(&in_grab_windowed_mouse);
	Cmd_AddLegacyCommand("_windowed_mouse", "in_grab_windowed_mouse");
	Cvar_ResetCurrentGroup();

	Cmd_AddCommand("vid_restart", VID_Restart);

	Sys_Video_CvarInit();
}


void VID_Open()
{
	int width, height, fullscreen;

	fullscreen = vid_fullscreen.value;
	width = vid_width.value;
	height = vid_height.value;

#warning Fix this.

	vid.colormap = host_colormap;
	vid.aspect = ((float)height / (float)width) * (320.0 / 240.0);


	Sys_Thread_LockMutex(display_mutex);
	display = Sys_Video_Open(vid_mode.string, width, height, fullscreen, host_basepal);
	Sys_Thread_UnlockMutex(display_mutex);
	if (display)
	{
		width = Sys_Video_GetWidth(display);
		height = Sys_Video_GetHeight(display);


		{
			vid.numpages = Sys_Video_GetNumBuffers(display);

			set_up_conwidth_conheight();


			if (windowtitle)
				Sys_Video_SetWindowTitle(display, windowtitle);

			mouse_grabbed = 2;
			refresh_mouse_grab_state();

			R_Init();

			V_UpdatePalette(true);
			Check_Gamma(host_basepal);
			VID_SetPalette(host_basepal);

			vid.recalc_refdef = 1;				// force a surface cache flush

			R_InitGL();
			GL_Particles_TextureInit();

			Draw_Init();
			M_VidInit();
			Sbar_Init();
			SCR_Init();
			Skin_Init();

			return;
		}

		Sys_Thread_LockMutex(display_mutex);

		Sys_Video_Close(display);

		display = 0;

		Sys_Thread_UnlockMutex(display_mutex);
	}

	Sys_Error("VID: Unable to open a display\n");
}

void VID_Close()
{
	Sys_Thread_LockMutex(display_mutex);


	Skin_Shutdown();
	Mod_ClearAll();
	SCR_Shutdown();
	Sbar_Shutdown();
	M_VidShutdown();
	Draw_Shutdown();

	if (display)
	{
		R_Shutdown();
		Sys_Video_Close(display);

		display = 0;
	}

	Sys_Thread_UnlockMutex(display_mutex);
}

void VID_BeginFrame()
{
	Sys_Video_BeginFrame(display);
}

void VID_Update(vrect_t *rects)
{
	Sys_Video_Update(display, rects);

}



int VID_GetKeyEvent(keynum_t *key, qboolean *down)
{
	return Sys_Video_GetKeyEvent(display, key, down);
}

void VID_GetMouseMovement(int *mousex, int *mousey)
{
	Sys_Thread_LockMutex(display_mutex);

	if (display)
		Sys_Video_GetMouseMovement(display, mousex, mousey);
	else
	{
		*mousex = 0;
		*mousey = 0;
	}

	Sys_Thread_UnlockMutex(display_mutex);
}

void VID_SetMouseGrab(int on)
{
	mouse_grab_wanted = !!on;

	refresh_mouse_grab_state();
}

void VID_SetDeviceGammaRamp(unsigned short *ramps)
{
	Sys_Video_SetGamma(display, ramps);
}

qboolean VID_HWGammaSupported()
{
	return Sys_Video_HWGammaSupported(display);
}

void VID_SetCaption(const char *text)
{
	char *newwindowtitle;

	if (display)
	{
		newwindowtitle = malloc(strlen(text)+1);
		if (newwindowtitle)
		{
			strcpy(newwindowtitle, text);
			free(windowtitle);
			windowtitle = newwindowtitle;
		}

		Sys_Video_SetWindowTitle(display, text);
	}
}

const char *VID_GetClipboardText()
{
	return Sys_Video_GetClipboardText(display);
}

void VID_FreeClipboardText(const char *text)
{
	Sys_Video_FreeClipboardText(display, text);
}

void VID_SetClipboardText(const char *text)
{
	Sys_Video_SetClipboardText(display, text);
}

unsigned int VID_GetWidth()
{
	return Sys_Video_GetWidth(display);
}

unsigned int VID_GetHeight()
{
	return Sys_Video_GetHeight(display);
}

qboolean VID_GetFullscreen()
{
	return Sys_Video_GetFullscreen(display);
}

const char *VID_GetMode()
{
	if (Sys_Video_GetFullscreen(display))
		return Sys_Video_GetMode(display);

	return "";
}

int VID_FocusChanged()
{
	if (vid_restarted)
	{
		vid_restarted = 0;
		return 1;
	}

	return Sys_Video_FocusChanged(display);
}

void VID_LockBuffer(void)
{
}

void VID_UnlockBuffer(void)
{
}


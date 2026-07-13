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

// renderer cvars, shared globals and no-op entry points kept off the gpu_* modules

#include <string.h>
#include <math.h>

#include "quakedef.h"
#include "r_local.h"
#include "r_framebuffer.h"
#include "r_post_process.h"
#include "gpu_local.h"

// some cvars are registered only so old configs load without warnings

cvar_t	r_drawentities = {"r_drawentities", "1"};
cvar_t	r_lerpframes = {"r_lerpframes", "1"};
cvar_t	r_lerpmuzzlehack = {"r_lerpmuzzlehack", "1"};
cvar_t	r_drawflame = {"r_drawflame", "1"};
cvar_t	r_speeds = {"r_speeds", "0"};
cvar_t	r_fullbright = {"r_fullbright", "1"};
cvar_t	r_lightmap = {"r_lightmap", "1"};
cvar_t	gl_shaftlight = {"gl_shaftlight", "1"};
cvar_t	r_wateralpha = {"r_wateralpha", "1"};
cvar_t	r_fastturb = {"r_fastturb", "0"};
cvar_t	r_dynamic = {"r_dynamic", "1"};
cvar_t	r_novis = {"r_novis", "0"};
cvar_t	r_netgraph = {"r_netgraph", "0"};
cvar_t	r_fullbrightSkins = {"r_fullbrightSkins", "0"};
cvar_t	r_fastsky = {"r_fastsky", "0"};
cvar_t	r_skycolor = {"r_skycolor", "4"};
cvar_t	gl_colorlights = {"gl_colorlights", "1"};
cvar_t	r_farclip = {"r_farclip", "8192"};
cvar_t	gl_detail = {"gl_detail", "0"};
cvar_t	gl_caustics = {"gl_caustics", "1"};
cvar_t	gl_subdivide_size = {"gl_subdivide_size", "128", CVAR_ARCHIVE};
cvar_t	gl_clear = {"gl_clear", "1"};
cvar_t	gl_clearColor = {"gl_clearColor", "0 0 0"};
cvar_t	gl_cull = {"gl_cull", "1"};
cvar_t	gl_smoothmodels = {"gl_smoothmodels", "0"};
cvar_t	gl_polyblend = {"gl_polyblend", "1"};
cvar_t	gl_flashblend = {"gl_flashblend", "0"};
cvar_t	gl_playermip = {"gl_playermip", "0"};
cvar_t	gl_finish = {"gl_finish", "0"};
cvar_t	gl_fb_bmodels = {"gl_fb_bmodels", "1"};
cvar_t	gl_fb_models = {"gl_fb_models", "1"};
cvar_t	gl_lightmode = {"gl_lightmode", "1"};
cvar_t	gl_loadlitfiles = {"gl_loadlitfiles", "1"};
extern cvar_t r_skyname;	// gpu_world.c, has the skybox OnChange
extern cvar_t r_crt_phosphor;	// gpu_vid.c
extern cvar_t r_crt_dotbloom;
extern cvar_t vid_framesinflight;
cvar_t	gl_water_program = {"gl_water_program", "1"};
cvar_t	gl_part_explosions = {"gl_part_explosions", "1"};
cvar_t	gl_part_trails = {"gl_part_trails", "0"};
cvar_t	gl_part_spikes = {"gl_part_spikes", "0"};
cvar_t	gl_part_gunshots = {"gl_part_gunshots", "2"};
cvar_t	gl_part_blood = {"gl_part_blood", "0"};
cvar_t	gl_part_telesplash = {"gl_part_telesplash", "0"};
cvar_t	gl_part_blobs = {"gl_part_blobs", "0"};
cvar_t	gl_part_lavasplash = {"gl_part_lavasplash", "1"};
cvar_t	gl_part_inferno = {"gl_part_inferno", "1"};
cvar_t	gl_max_size = {"gl_max_size", "512"};
cvar_t	gl_miptexLevel = {"gl_miptexLevel", "0"};
cvar_t	gl_scaleModelTextures = {"gl_scaleModelTextures", "0"};
cvar_t	gl_scaleTurbTextures = {"gl_scaleTurbTextures", "1"};
cvar_t	gl_externalTextures_world = {"gl_externalTextures_world", "1"};
cvar_t	gl_externalTextures_bmodels = {"gl_externalTextures_bmodels", "1"};

static cvar_t *stub_cvars[] = {
	&r_drawentities, &r_lerpframes, &r_lerpmuzzlehack, &r_drawflame, &r_speeds,
	&r_fullbright, &r_lightmap, &gl_shaftlight, &r_wateralpha, &r_fastturb,
	&r_dynamic, &r_novis, &r_netgraph, &r_fullbrightSkins, &r_fastsky,
	&r_skycolor, &gl_colorlights, &r_farclip, &gl_detail, &gl_caustics,
	&gl_subdivide_size, &gl_clear, &gl_clearColor, &gl_cull, &gl_smoothmodels,
	&gl_polyblend, &gl_flashblend, &gl_playermip, &gl_finish, &gl_fb_bmodels,
	&gl_fb_models, &gl_lightmode, &gl_loadlitfiles, &r_skyname, &gl_water_program,
	&gl_part_explosions, &gl_part_trails, &gl_part_spikes, &gl_part_gunshots,
	&gl_part_blood, &gl_part_telesplash, &gl_part_blobs, &gl_part_lavasplash,
	&gl_part_inferno, &gl_max_size,
	&gl_miptexLevel, &gl_scaleModelTextures, &gl_scaleTurbTextures,
	&gl_externalTextures_world, &gl_externalTextures_bmodels,
	&r_crt_phosphor, &r_crt_dotbloom, &vid_framesinflight,
};

// ---- shared globals ----

refdef_t r_refdef;
vec3_t r_origin, vpn, vright, vup;
entity_t r_worldentity;
texture_t *r_notexture_mip;
qboolean r_cache_thrash;
int d_lightstylevalue[256];
mplane_t frustum[4];
int r_visframecount, r_framecount;
int c_brush_polys, c_alias_polys;
vec3_t modelorg, r_entorigin;
entity_t *currententity;
mleaf_t *r_viewleaf, *r_oldviewleaf;
mleaf_t *r_viewleaf2, *r_oldviewleaf2;
qboolean r_skyboxloaded;

int lightmode = 2;
int particletexture, netgraphtexture, skyboxtextures;
int underwatertexture, detailtexture;

int texture_extension_number = 1;
int currenttexture = -1;
int gl_max_size_default = 2048;
int gl_filter_max;

qboolean gl_fbo = true;

unsigned d_8to24table[256];
unsigned d_8to24table2[256];
float vid_gamma = 1.0f;
byte vid_gamma_table[256];

// ---- palette ----

void Check_Gamma(unsigned char *pal)
{
	float inf;
	unsigned char palette[768];
	int i;

	if ((i = COM_CheckParm("-gamma")) != 0 && i + 1 < com_argc)
	{
		vid_gamma = bound(0.3, Q_atof(com_argv[i + 1]), 1);
		Cvar_SetDefault(&v_gamma, vid_gamma);
	}
	else
	{
		vid_gamma = 1;
	}

	if (vid_gamma != 1)
	{
		for (i = 0; i < 256; i++)
		{
			inf = 255 * pow((i + 0.5) / 255.5, vid_gamma) + 0.5;
			if (inf > 255)
				inf = 255;

			vid_gamma_table[i] = inf;
		}
	}
	else
	{
		for (i = 0; i < 256; i++)
			vid_gamma_table[i] = i;
	}

	for (i = 0; i < 768; i++)
		palette[i] = vid_gamma_table[pal[i]];

	memcpy(pal, palette, sizeof(palette));
}

void VID_SetPalette(unsigned char *palette)
{
	int i;
	byte *pal;
	unsigned r, g, b, v, *table;

	// 8 8 8 encoding
	pal = palette;
	table = d_8to24table;
	for (i = 0; i < 256; i++)
	{
		r = pal[0];
		g = pal[1];
		b = pal[2];
		pal += 3;

		v = (255 << 24) + (b << 16) + (g << 8) + (r << 0);
		*table++ = LittleLong(v);
	}
	d_8to24table[255] = 0;	// 255 is transparent

	// Tonik: create a brighter palette for bmodel textures
	pal = palette;
	table = d_8to24table2;

	for (i = 0; i < 256; i++)
	{
		r = pal[0] * (2.0 / 1.5); if (r > 255) r = 255;
		g = pal[1] * (2.0 / 1.5); if (g > 255) g = 255;
		b = pal[2] * (2.0 / 1.5); if (b > 255) b = 255;
		pal += 3;
		v = (255 << 24) + (b << 16) + (g << 8) + (r << 0);
		*table++ = LittleLong(v);
	}

	d_8to24table2[255] = 0;	// 255 is transparent
}

// ---- lifecycle ----

void R_CvarInit(void)
{
	unsigned int i;

	for (i = 0; i < sizeof(stub_cvars) / sizeof(*stub_cvars); i++)
		Cvar_Register(stub_cvars[i]);

	R_Particles_CvarInit();

	Cmd_AddCommand("loadsky", R_LoadSky_f);
	Cmd_AddCommand("timerefresh", R_TimeRefresh_f);
}

void R_CommonCvarInit(void) {}
void R_Set2D(void) {}

void Draw_SetSize(unsigned int width, unsigned int height)
{
	(void)width; (void)height;
}

// ---- FBO / post-process shims, the real pass lives in gpu_vid.c ----

qboolean R_FBO_Init(int width, int height)
{
	(void)width; (void)height;
	return true;
}

void R_FBO_Bind(void) {}
void R_FBO_Unbind(void) {}

unsigned int R_FBO_GetColorTexture(void)
{
	return 0;
}

qboolean R_PostProcess_IsReady(void)
{
	return true;
}

void R_PostProcess_Draw(unsigned int color_tex, float gamma, float contrast, const float blend[4])
{
	(void)color_tex;
	GPU_SetPostParams(gamma, contrast, blend);
}

float R_PostProcess_CrtGamma(void)
{
	return GPU_CrtCompGamma();
}

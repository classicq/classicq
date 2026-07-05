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

// temporary no-op renderer while SDL_GPU modules replace the GL ones
// pieces move out of here milestone by milestone, then this file dies

#include <stdlib.h>
#include <string.h>
#include <math.h>

#include "quakedef.h"
#include "gl_local.h"
#include "gl_skinimp.h"
#include "gl_framebuffer.h"
#include "gl_post_process.h"
#include "gl_state.h"
#include "particles.h"
#include "gpu_local.h"

// ---- cvars (defaults copied from the GL renderer) ----

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
cvar_t	r_skyname = {"r_skyname", "purple_chaos"};
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
cvar_t	gl_clipparticles = {"gl_clipparticles", "1"};
cvar_t	gl_bounceparticles = {"gl_bounceparticles", "1"};
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
	&gl_part_inferno, &gl_clipparticles, &gl_bounceparticles, &gl_max_size,
	&gl_miptexLevel, &gl_scaleModelTextures, &gl_scaleTurbTextures,
	&gl_externalTextures_world, &gl_externalTextures_bmodels,
};

// ---- shared globals owned by the dropped GL files ----

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
int particletexture, netgraphtexture, playertextures, skyboxtextures;
int playerfbtextures[MAX_CLIENTS];
int underwatertexture, detailtexture;

int texture_extension_number = 1;
int currenttexture = -1;
int vbo_number = 1;
int gl_lightmap_format, gl_solid_format = 3, gl_alpha_format = 4;
int gl_max_size_default = 2048;
int gl_filter_max;

const char *gl_vendor = "classicQ", *gl_renderer = "SDL_GPU";
const char *gl_version = "3", *gl_extensions = "";

float gldepthmin, gldepthmax;
byte color_white[4] = {255, 255, 255, 255}, color_black[4] = {0, 0, 0, 255};
qboolean gl_mtexable = true;
int gl_textureunits = 4;
qboolean gl_combine, gl_add_ext, gl_npot = true, gl_vbo = false;
qboolean gl_vs, gl_fs, gl_fbo = true;

qboolean qmb_initialized;

unsigned d_8to24table[256];
unsigned d_8to24table2[256];
unsigned short d_8to16table[256];
float vid_gamma = 1.0f;
byte vid_gamma_table[256];

void (APIENTRY *qglBindBufferARB)(GLenum, GLuint);
void (APIENTRY *qglBufferDataARB)(GLenum, GLsizeiptrARB, const GLvoid *, GLenum);
void (APIENTRY *qglBufferSubDataARB)(GLenum, GLintptrARB, GLsizeiptrARB, const GLvoid *);

// ---- palette (real, engine reads these tables) ----

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
}

int R_Init(void)
{
	return 1;
}

void R_InitGL(void) {}
void R_Shutdown(void) {}
void R_PreMapLoad(void) {}
void R_NewMap(void) {}
void R_InitEfrags(void) {}
void GL_CvarInit(void) {}
void GL_Init(void) {}

// ---- frame ----

void R_PushDlights(void) {}
void R_RenderView(void) {}
void GL_Set2D(void) {}
void R_PolyBlend(void) {}
void R_BrightenScreen(void) {}
void R_NetGraph(void) {}
void Draw_SizeChanged(void) {}

qboolean R_CullBox(vec3_t mins, vec3_t maxs)
{
	(void)mins; (void)maxs;
	return false;
}

qboolean R_CullSphere(vec3_t centre, float radius)
{
	(void)centre; (void)radius;
	return false;
}

// ---- loader hooks (gl_model.c stays compiled) ----

void R_InitSky(void *texturedata)
{
	(void)texturedata;
}

void GL_SubdivideSurface(model_t *model, msurface_t *fa)
{
	(void)model; (void)fa;
}

// ---- FBO / post-process (real work happens in gpu_vid.c) ----

qboolean GL_FBO_Init(int width, int height)
{
	(void)width; (void)height;
	return true;
}

void GL_FBO_Shutdown(void) {}

qboolean GL_FBO_Resize(int width, int height)
{
	(void)width; (void)height;
	return true;
}

void GL_FBO_Bind(void) {}
void GL_FBO_Unbind(void) {}

unsigned int GL_FBO_GetColorTexture(void)
{
	return 0;
}

unsigned int GL_FBO_GetID(void)
{
	return 0;
}

int GL_FBO_GetWidth(void)
{
	return vid.displaywidth;
}

int GL_FBO_GetHeight(void)
{
	return vid.displayheight;
}

qboolean GL_PostProcess_Init(void)
{
	return true;
}

void GL_PostProcess_Shutdown(void) {}

qboolean GL_PostProcess_IsReady(void)
{
	return true;
}

void GL_PostProcess_Draw(unsigned int color_tex, float gamma, float contrast, const float blend[4])
{
	(void)color_tex;
	GPU_SetPostParams(gamma, contrast, blend);
}

// ---- 2D ----

struct Picture
{
	unsigned int width;
	unsigned int height;
};

void DrawImp_CvarInit(void) {}
void DrawImp_Init(void) {}
void DrawImp_Shutdown(void) {}

void DrawImp_Character(int x, int y, unsigned char num)
{
	(void)x; (void)y; (void)num;
}

void DrawImp_SetTextColor(int r, int g, int b)
{
	(void)r; (void)g; (void)b;
}

void Draw_BeginTextRendering(void) {}
void Draw_EndTextRendering(void) {}
void Draw_BeginColoredTextRendering(void) {}
void Draw_EndColoredTextRendering(void) {}

void Draw_Fill(int x, int y, int w, int h, int c)
{
	(void)x; (void)y; (void)w; (void)h; (void)c;
}

void Draw_AlphaFill(int x, int y, int w, int h, int c, float alpha)
{
	(void)x; (void)y; (void)w; (void)h; (void)c; (void)alpha;
}

void Draw_AlphaFillRGB(int x, int y, int w, int h, float r, float g, float b, float alpha)
{
	(void)x; (void)y; (void)w; (void)h; (void)r; (void)g; (void)b; (void)alpha;
}

void Draw_Line(int x1, int y1, int x2, int y2, float width, float r, float g, float b, float alpha)
{
	(void)x1; (void)y1; (void)x2; (void)y2; (void)width; (void)r; (void)g; (void)b; (void)alpha;
}

void Draw_FadeScreen(void) {}
void Draw_Crosshair(void) {}
void Draw_RecalcCrosshair(void) {}

void Draw_SetSize(unsigned int width, unsigned int height)
{
	(void)width; (void)height;
}

struct Picture *Draw_LoadPicture(const char *name, enum Draw_LoadPicture_Fallback fallback)
{
	struct Picture *pic;

	(void)name; (void)fallback;

	pic = calloc(1, sizeof(*pic));
	if (pic)
	{
		pic->width = 64;
		pic->height = 64;
	}
	return pic;
}

struct Picture *Draw_BuildTranslatedMenuplyr(int top, int bottom)
{
	(void)top; (void)bottom;
	return Draw_LoadPicture("", DRAW_LOADPICTURE_DUMMYFALLBACK);
}

void Draw_FreePicture(struct Picture *pic)
{
	free(pic);
}

unsigned int Draw_GetPictureWidth(struct Picture *pic)
{
	return pic ? pic->width : 0;
}

unsigned int Draw_GetPictureHeight(struct Picture *pic)
{
	return pic ? pic->height : 0;
}

void Draw_DrawPicture(struct Picture *pic, int x, int y, unsigned int width, unsigned int height)
{
	(void)pic; (void)x; (void)y; (void)width; (void)height;
}

void Draw_DrawPictureModulated(struct Picture *pic, int x, int y, unsigned int width, unsigned int height, float r, float g, float b, float alpha)
{
	(void)pic; (void)x; (void)y; (void)width; (void)height; (void)r; (void)g; (void)b; (void)alpha;
}

void Draw_DrawSubPicture(struct Picture *pic, float sx, float sy, float swidth, float sheight, int x, int y, unsigned int width, unsigned int height)
{
	(void)pic; (void)sx; (void)sy; (void)swidth; (void)sheight; (void)x; (void)y; (void)width; (void)height;
}

// ---- textures ----

void GL_Bind(int texnum)
{
	currenttexture = texnum;
}

void GL_SelectTexture(GLenum target) { (void)target; }
void GL_DisableMultitexture(void) {}
void GL_EnableMultitexture(void) {}
void GL_EnableTMU(GLenum target) { (void)target; }
void GL_DisableTMU(GLenum target) { (void)target; }

void GL_Upload8(byte *data, int width, int height, int mode)
{
	(void)data; (void)width; (void)height; (void)mode;
}

void GL_Upload32(unsigned *data, int width, int height, int mode)
{
	(void)data; (void)width; (void)height; (void)mode;
}

int GL_LoadTexture(char *identifier, int width, int height, byte *data, int mode, int bpp)
{
	(void)identifier; (void)width; (void)height; (void)data; (void)mode; (void)bpp;
	return texture_extension_number++;
}

byte *GL_LoadImagePixels(char *filename, int matchwidth, int matchheight, unsigned int *imagewidth, unsigned int *imageheight, int mode)
{
	(void)filename; (void)matchwidth; (void)matchheight; (void)imagewidth; (void)imageheight; (void)mode;
	return NULL;
}

int GL_LoadTexturePixels(byte *data, char *identifier, int width, int height, int mode)
{
	(void)data; (void)identifier; (void)width; (void)height; (void)mode;
	return texture_extension_number++;
}

int GL_LoadTextureImage(char *filename, char *identifier, int matchwidth, int matchheight, int mode)
{
	(void)filename; (void)identifier; (void)matchwidth; (void)matchheight; (void)mode;
	return texture_extension_number++;
}

int GL_LoadCharsetImage(char *filename, char *identifier)
{
	(void)filename; (void)identifier;
	return texture_extension_number++;
}

void GL_Texture_CvarInit(void) {}
void GL_Texture_Init(void) {}
void GL_Texture_Shutdown(void) {}

// ---- skins ----

struct SkinImp *SkinImp_CreateSolidColour(float *colours)
{
	(void)colours;
	return calloc(1, sizeof(struct SkinImp));
}

struct SkinImp *SkinImp_CreateTexturePaletted(void *data, unsigned int width, unsigned int height, unsigned int modulo)
{
	(void)data; (void)width; (void)height; (void)modulo;
	return calloc(1, sizeof(struct SkinImp));
}

struct SkinImp *SkinImp_CreateTextureTruecolour(void *data, unsigned int width, unsigned int height)
{
	(void)data; (void)width; (void)height;
	return calloc(1, sizeof(struct SkinImp));
}

void SkinImp_Destroy(struct SkinImp *skin)
{
	free(skin);
}

// ---- particles ----

void GL_Particles_CvarInit(void) {}
void GL_Particles_TextureInit(void) {}

void GL_DrawParticleInit(void) {}
void GL_DrawParticleBegin(void) {}
void GL_DrawParticleEnd(void) {}

void GL_DrawParticle(particle_t *p)
{
	(void)p;
}

int QMB_InitParticles(void)
{
	return 0;
}

void QMB_ShutdownParticles(void) {}
void QMB_ClearParticles(void) {}
void QMB_DrawParticles(void) {}

void QMB_RunParticleEffect(const vec3_t org, const vec3_t dir, int color, int count)
{
	(void)org; (void)dir; (void)color; (void)count;
}

void QMB_ParticleTrail(vec3_t start, vec3_t end, vec3_t *trail_origin, trail_type_t type)
{
	(void)start; (void)end; (void)trail_origin; (void)type;
}

void QMB_BlobExplosion(vec3_t org) { (void)org; }
void QMB_ParticleExplosion(vec3_t org) { (void)org; }
void QMB_LavaSplash(vec3_t org) { (void)org; }
void QMB_TeleportSplash(vec3_t org) { (void)org; }
void QMB_DetpackExplosion(vec3_t org) { (void)org; }
void QMB_InfernoFlame(vec3_t org) { (void)org; }

void QMB_StaticBubble(entity_t *ent)
{
	(void)ent;
}

// ---- misc gl-side entry points still referenced ----

void R_DrawEntitiesOnList(visentlist_t *vislist)
{
	(void)vislist;
}

void R_AnimateLight(void) {}
void R_RenderDlights(void) {}

void R_MarkLights(model_t *model, dlight_t *light, int bit, unsigned int nodenum)
{
	(void)model; (void)light; (void)bit; (void)nodenum;
}

int R_LightPoint(vec3_t p)
{
	(void)p;
	return 255;
}

void R_InitOtherTextures(void) {}

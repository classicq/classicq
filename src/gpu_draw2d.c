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
#include <ctype.h>

#include "quakedef.h"
#include "gl_local.h"
#include "sbar.h"
#include "wad.h"
#include "image.h"
#include "utils.h"
#include "config.h"
#include "draw.h"
#include "gpu_local.h"

#define MAX_BATCHES 4096

static ui_vert_t verts[GPU_UI_MAX_VERTS];
static unsigned int numverts;

static ui_batch_t batches[MAX_BATCHES];
static unsigned int numbatches;

static int warned_overflow;

void Draw2D_FrameReset(void)
{
	numverts = 0;
	numbatches = 0;
	warned_overflow = 0;
}

void Draw2D_Quad(int texnum, int alphatest,
	float x0, float y0, float x1, float y1,
	float s0, float t0, float s1, float t1,
	const unsigned char rgba[4])
{
	ui_batch_t *b;
	ui_vert_t *v;
	float ow = (float)vid.conwidth;
	float oh = (float)vid.conheight;

	if (numverts + 6 > GPU_UI_MAX_VERTS || numbatches >= MAX_BATCHES)
	{
		if (!warned_overflow)
		{
			Com_Printf("Draw2D: batch overflow\n");
			warned_overflow = 1;
		}
		return;
	}

	b = numbatches ? &batches[numbatches - 1] : NULL;
	if (!b || b->texnum != texnum || b->alphatest != alphatest
		|| b->ortho_w != ow || b->ortho_h != oh)
	{
		b = &batches[numbatches++];
		b->texnum = texnum;
		b->alphatest = alphatest;
		b->firstvert = numverts;
		b->numverts = 0;
		b->ortho_w = ow;
		b->ortho_h = oh;
	}

	v = &verts[numverts];

	v[0].x = x0; v[0].y = y0; v[0].u = s0; v[0].v = t0;
	v[1].x = x1; v[1].y = y0; v[1].u = s1; v[1].v = t0;
	v[2].x = x1; v[2].y = y1; v[2].u = s1; v[2].v = t1;
	v[3].x = x0; v[3].y = y0; v[3].u = s0; v[3].v = t0;
	v[4].x = x1; v[4].y = y1; v[4].u = s1; v[4].v = t1;
	v[5].x = x0; v[5].y = y1; v[5].u = s0; v[5].v = t1;

	memcpy(v[0].rgba, rgba, 4);
	memcpy(v[1].rgba, rgba, 4);
	memcpy(v[2].rgba, rgba, 4);
	memcpy(v[3].rgba, rgba, 4);
	memcpy(v[4].rgba, rgba, 4);
	memcpy(v[5].rgba, rgba, 4);

	numverts += 6;
	b->numverts += 6;
}

const ui_vert_t *Draw2D_GetVerts(unsigned int *out_numverts)
{
	*out_numverts = numverts;
	return verts;
}

const ui_batch_t *Draw2D_GetBatches(unsigned int *out_numbatches)
{
	*out_numbatches = numbatches;
	return batches;
}

// ---- 2D drawing layer, ported from gl_draw.c ----

struct Picture
{
	int texnum;
	unsigned int width;
	unsigned int height;
	float glwidthscale;
	float glheightscale;

	float texcoords[4*2];
};

static unsigned char drawgl_inited;

static const unsigned char colour_white[4] = {255, 255, 255, 255};

extern cvar_t scr_menualpha;

extern cvar_t crosshair, cl_crossx, cl_crossy, crosshaircolor, crosshairsize;

static void PostChange_crosshairstuff(cvar_t *);

qboolean OnChange_gl_crosshairimage(cvar_t *, char *);
cvar_t	gl_crosshairimage   = {"crosshairimage", "", 0, OnChange_gl_crosshairimage};

qboolean OnChange_gl_consolefont (cvar_t *, char *);
cvar_t	gl_consolefont		= {"gl_consolefont", "classicq", 0, OnChange_gl_consolefont};

cvar_t	gl_crosshairalpha	= {"crosshairalpha", "1", 0, 0, PostChange_crosshairstuff};

qboolean OnChange_gl_smoothfont (cvar_t *var, char *string);
cvar_t gl_smoothfont = {"gl_smoothfont", "0", 0, OnChange_gl_smoothfont};

byte			*draw_chars;						// 8*8 graphic characters

static int		char_texture;


#define		NUMCROSSHAIRS 6
int			crosshairtextures[NUMCROSSHAIRS];
int			crosshairtexture_txt;
struct Picture *crosshairpic;

static byte customcrosshairdata[64];

#define CROSSHAIR_NONE	0
#define CROSSHAIR_TXT	1
#define CROSSHAIR_IMAGE	2
static int customcrosshair_loaded = CROSSHAIR_NONE;


static byte crosshairdata[NUMCROSSHAIRS][64] =
{
	{
		0xff, 0xff, 0xff, 0xfe, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xfe, 0xff, 0xff, 0xff, 0xff,
		0xfe, 0xff, 0xfe, 0xff, 0xfe, 0xff, 0xfe, 0xff,
		0xff, 0xff, 0xff, 0xfe, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xfe, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	},

	{
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xfe, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xfe, 0xfe, 0xfe, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xfe, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	},

	{
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xfe, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	},

	{
		0xfe, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xfe,
		0xff, 0xfe, 0xff, 0xff, 0xff, 0xff, 0xfe, 0xff,
		0xff, 0xff, 0xfe, 0xff, 0xff, 0xfe, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xfe, 0xff, 0xff, 0xfe, 0xff, 0xff,
		0xff, 0xfe, 0xff, 0xff, 0xff, 0xff, 0xfe, 0xff,
		0xfe, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xfe,
	},

	{
		0xff, 0xff, 0xfe, 0xfe, 0xfe, 0xff, 0xff, 0xff,
		0xff, 0xfe, 0xff, 0xfe, 0xff, 0xfe, 0xff, 0xff,
		0xfe, 0xfe, 0xff, 0xfe, 0xff, 0xfe, 0xfe, 0xff,
		0xfe, 0xfe, 0xfe, 0xfe, 0xfe, 0xfe, 0xfe, 0xff,
		0xfe, 0xff, 0xfe, 0xfe, 0xfe, 0xff, 0xfe, 0xff,
		0xff, 0xfe, 0xff, 0xff, 0xff, 0xfe, 0xff, 0xff,
		0xff, 0xff, 0xfe, 0xfe, 0xfe, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	},

	{
		0xff, 0xff, 0xfe, 0xfe, 0xfe, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xfe, 0xff, 0xff, 0xff, 0xff, 0xff, 0xfe, 0xff,
		0xfe, 0xff, 0xff, 0xfe, 0xff, 0xff, 0xfe, 0xff,
		0xfe, 0xff, 0xff, 0xff, 0xff, 0xff, 0xfe, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xfe, 0xfe, 0xfe, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	},
};

static void SetupFontSmoothing(void)
{
	if (!char_texture)
		return;

	GPU_Texture_SetPrefs(char_texture, gl_smoothfont.value ? GPU_TEXPREF_LINEAR : 0);
}

qboolean OnChange_gl_smoothfont (cvar_t *var, char *string)
{
	if (drawgl_inited)
	{
		var->value = Q_atof(string);

		SetupFontSmoothing();
	}

	return false;
}

static void Crosshair_LoadImage(const char *s)
{
	struct Picture *pic;

	if (crosshairpic)
	{
		Draw_FreePicture(crosshairpic);
		crosshairpic = 0;
	}

	customcrosshair_loaded &= ~CROSSHAIR_IMAGE;

	if (*s)
	{
		pic = Draw_LoadPicture(va("crosshairs/%s", s), DRAW_LOADPICTURE_NOFALLBACK);
		if (pic)
		{
			// force NEAREST, keep clamp
			GPU_Texture_SetPrefs(pic->texnum, GPU_TEXPREF_CLAMP);
			crosshairpic = pic;
			customcrosshair_loaded |= CROSSHAIR_IMAGE;
		}
		else
			Com_Printf("Couldn't load image %s\n", s);
	}

	Draw_RecalcCrosshair();
}

qboolean OnChange_gl_crosshairimage(cvar_t *v, char *s)
{
	if (drawgl_inited)
		Crosshair_LoadImage(s);

	return false;
}

void customCrosshair_Init(void)
{
	FILE *f;
	int i = 0, c;

	customcrosshair_loaded = CROSSHAIR_NONE;

	Crosshair_LoadImage(gl_crosshairimage.string);

	if (FS_FOpenFile("crosshairs/crosshair.txt", &f) == -1)
		return;

	while (i < 64)
	{
		c = fgetc(f);
		if (c == EOF)
		{
			Com_Printf("Invalid format in crosshair.txt (Need 64 X's and O's)\n");
			fclose(f);
			return;
		}
		if (isspace(c))
			continue;
		if (tolower(c) != 'x' && tolower(c) != 'o')
		{
			Com_Printf("Invalid format in crosshair.txt (Only X's and O's and whitespace permitted)\n");
			fclose(f);
			return;
		}
		customcrosshairdata[i++] = (c == 'x' || c  == 'X') ? 0xfe : 0xff;
	}
	fclose(f);
	crosshairtexture_txt = GL_LoadTexture ("", 8, 8, customcrosshairdata, TEX_ALPHA, 1);
	GPU_Texture_SetPrefs(crosshairtexture_txt, 0);
	customcrosshair_loaded |= CROSSHAIR_TXT;
}

void Draw_SizeChanged(void)
{
	Draw_RecalcCrosshair();
}

static int Draw_LoadCharset(char *name)
{
	int texnum;

	if (!Q_strcasecmp(name, "original"))
	{
		int i;
		byte buf[128 * 256], *src, *dest;

		// blank rows between character rows keep LINEAR from bleeding
		memset (buf, 255, sizeof(buf));
		src = draw_chars;
		dest = buf;
		for (i = 0; i < 16; i++)
		{
			memcpy (dest, src, 128 * 8);
			src += 128 * 8;
			dest += 128 * 8 * 2;
		}
		char_texture = GL_LoadTexture ("pic:charset", 128, 256, buf, TEX_ALPHA, 1);
	}
	else if ((texnum = GL_LoadCharsetImage (va("textures/charsets/%s", name), "pic:charset")))
	{
		char_texture = texnum;
	}
	else
	{
		Com_Printf ("Couldn't load charset \"%s\"\n", name);
		return 1;
	}

	SetupFontSmoothing();

	return 0;
}

qboolean OnChange_gl_consolefont(cvar_t *var, char *string)
{
	if (drawgl_inited)
		return Draw_LoadCharset(string);

	return 0;
}

void Draw_LoadCharset_f (void)
{
	switch (Cmd_Argc())
	{
		case 1:
			Com_Printf("Current charset is \"%s\"\n", gl_consolefont.string);
			break;
		case 2:
			Cvar_Set(&gl_consolefont, Cmd_Argv(1));
			break;
		default:
			Com_Printf("Usage: %s <charset>\n", Cmd_Argv(0));
			break;
	}
}

void Draw_InitCharset(void)
{
	int i;

	draw_chars = W_GetLumpName ("conchars");
	for (i = 0; i < 256 * 64; i++)
	{
		if (draw_chars[i] == 0)
			draw_chars[i] = 255;
	}

	Draw_LoadCharset(gl_consolefont.string);

	if (!char_texture)
	{
		Cvar_Set(&gl_consolefont, "original");
		Draw_LoadCharset(gl_consolefont.string);
	}

	if (!char_texture)
		Sys_Error("Draw_InitCharset: Couldn't load charset");
}

void DrawImp_CvarInit(void)
{
	Cmd_AddCommand("loadcharset", Draw_LoadCharset_f);

	Cvar_SetCurrentGroup(CVAR_GROUP_CONSOLE);
	Cvar_Register (&gl_smoothfont);
	Cvar_Register (&gl_consolefont);

	Cvar_SetCurrentGroup(CVAR_GROUP_CROSSHAIR);
	Cvar_Register (&gl_crosshairimage);
	Cvar_Register (&gl_crosshairalpha);

	Cvar_ResetCurrentGroup();

	GL_Texture_CvarInit();
}

static struct Picture *dummypicture;

static const unsigned char dummyfallbackdata[] =
{
	0xff, 0xff, 0xff, 0xff,
	0x00, 0x00, 0x00, 0xff,
	0x00, 0x00, 0x00, 0xff,
	0xff, 0xff, 0xff, 0xff,
};

static void Draw_CreateDummyPicture(void)
{
	SDL_GPUTexture *tex;

	dummypicture = malloc(sizeof(*dummypicture));
	if (dummypicture)
	{
		dummypicture->texnum = texture_extension_number++;

		dummypicture->width = 2;
		dummypicture->height = 2;

		dummypicture->glwidthscale = 1;
		dummypicture->glheightscale = 1;

		dummypicture->texcoords[0] = 0;
		dummypicture->texcoords[1] = 0;
		dummypicture->texcoords[2] = 1;
		dummypicture->texcoords[3] = 0;
		dummypicture->texcoords[4] = 1;
		dummypicture->texcoords[5] = 1;
		dummypicture->texcoords[6] = 0;
		dummypicture->texcoords[7] = 1;

		tex = GPU_CreateTextureRGBA(dummyfallbackdata, 2, 2, 0);
		GPU_Texture_Set(dummypicture->texnum, tex, 2, 2, GPU_TEXPREF_CLAMP);
	}
}

static void Draw_DeleteDummyPicture(void)
{
	free(dummypicture);
	dummypicture = 0;
}

void DrawImp_Init(void)
{
	int i;

	Draw_InitCharset ();

	for (i = 0; i < NUMCROSSHAIRS; i++)
	{
		crosshairtextures[i] = GL_LoadTexture ("", 8, 8, crosshairdata[i], TEX_ALPHA, 1);
		GPU_Texture_SetPrefs(crosshairtextures[i], 0);
	}
	customCrosshair_Init();

	Draw_RecalcCrosshair();

	Draw_CreateDummyPicture();

	drawgl_inited = 1;
}

void DrawImp_Shutdown(void)
{
	if (crosshairpic)
	{
		Draw_FreePicture(crosshairpic);
		crosshairpic = 0;
	}

	Draw_DeleteDummyPicture();

	drawgl_inited = 0;
}

static int textrenderingenabled;
static int colouredtextrendering;

static unsigned char fontcolour[4] = { 255, 255, 255, 255 };

void DrawImp_SetTextColor(int r, int g, int b)
{
	fontcolour[0] = r|(r<<4);
	fontcolour[1] = g|(g<<4);
	fontcolour[2] = b|(b<<4);
}

//Draws one 8*8 graphics character with 0 being transparent.
//It can be clipped to the top of the screen to allow the console to be smoothly scrolled off.
void DrawImp_Character(int x, int y, unsigned char num)
{
	float frow, fcol;

	frow = (num >> 4) * 0.0625;
	fcol = (num & 15) * 0.0625;

	Draw2D_Quad(char_texture, 1,
		x, y, x + 8, y + 8,
		fcol, frow, fcol + 0.0625, frow + 0.03125,
		colouredtextrendering ? fontcolour : colour_white);
}

void Draw_BeginTextRendering(void)
{
	textrenderingenabled++;
}

void Draw_EndTextRendering(void)
{
	textrenderingenabled--;

	if (!textrenderingenabled)
		colouredtextrendering = 0;
}

void Draw_BeginColoredTextRendering(void)
{
	if (!textrenderingenabled)
		colouredtextrendering = 1;

	textrenderingenabled++;
}

void Draw_EndColoredTextRendering(void)
{
	Draw_EndTextRendering();
	fontcolour[0] = 255;
	fontcolour[1] = 255;
	fontcolour[2] = 255;
	fontcolour[3] = 255;
}

static int crosshairtexnum;
static float crosshair_x0, crosshair_y0, crosshair_x1, crosshair_y1;
static unsigned char crosshaircolour[4];

void Draw_RecalcCrosshair(void)
{
	float x, y;
	float ofs1;
	float ofs2;
	unsigned char *c;
	extern vrect_t scr_vrect;

	x = scr_vrect.x + scr_vrect.width / 2 + cl_crossx.value;
	y = scr_vrect.y + scr_vrect.height / 2 + cl_crossy.value;

	c = StringToRGB(crosshaircolor.string);

	crosshaircolour[0] = c[0];
	crosshaircolour[1] = c[1];
	crosshaircolour[2] = c[2];
	crosshaircolour[3] = bound(0, gl_crosshairalpha.value, 1) * 255;

	if (customcrosshair_loaded & CROSSHAIR_IMAGE)
	{
		crosshairtexnum = crosshairpic->texnum;
		ofs1 = 4 - 4.0 / 16;
		ofs2 = 4 + 4.0 / 16;
	}
	else
	{
		crosshairtexnum = (crosshair.value >= 2) ? crosshairtextures[(int) crosshair.value - 2] : crosshairtexture_txt;
		ofs1 = 3.5;
		ofs2 = 4.5;
	}
	ofs1 *= (vid.conwidth / 320) * bound(0, crosshairsize.value, 20);
	ofs2 *= (vid.conwidth / 320) * bound(0, crosshairsize.value, 20);

	crosshair_x0 = x - ofs1;
	crosshair_y0 = y - ofs1;
	crosshair_x1 = x + ofs2;
	crosshair_y1 = y + ofs2;
}

static void PostChange_crosshairstuff(cvar_t *v)
{
	Draw_RecalcCrosshair();
}

void Draw_Crosshair(void)
{
	extern vrect_t scr_vrect;

	if ((crosshair.value >= 2 && crosshair.value <= NUMCROSSHAIRS + 1)
	 || ((customcrosshair_loaded & CROSSHAIR_TXT) && crosshair.value == 1)
	 || (customcrosshair_loaded & CROSSHAIR_IMAGE))
	{
		if (!gl_crosshairalpha.value)
			return;

		Draw2D_Quad(crosshairtexnum, 0,
			crosshair_x0, crosshair_y0, crosshair_x1, crosshair_y1,
			0, 0, 1, 1,
			crosshaircolour);
	}
	else if (crosshair.value)
	{
		Draw_Character(scr_vrect.x + scr_vrect.width / 2 - 4 + cl_crossx.value, scr_vrect.y + scr_vrect.height / 2 - 4 + cl_crossy.value, '+');
	}
}

void Draw_AlphaFillRGB(int x, int y, int w, int h, float r, float g, float b, float alpha)
{
	unsigned char col[4];

	alpha = bound(0, alpha, 1);

	if (!alpha)
		return;

	col[0] = r * 255;
	col[1] = g * 255;
	col[2] = b * 255;
	col[3] = alpha * 255;

	Draw2D_Quad(GPU_Texture_White(), 0, x, y, x + w, y + h, 0, 0, 1, 1, col);
}

void Draw_AlphaFill(int x, int y, int w, int h, int c, float alpha)
{
	float r;
	float g;
	float b;

	r = host_basepal[c * 3 + 0] / 255.0;
	g = host_basepal[c * 3 + 1] / 255.0;
	b = host_basepal[c * 3 + 2] / 255.0;

	Draw_AlphaFillRGB(x, y, w, h, r, g, b, alpha);
}

void Draw_Fill(int x, int y, int w, int h, int c)
{
	Draw_AlphaFill(x, y, w, h, c, 1);
}

void Draw_Line(int x1, int y1, int x2, int y2, float width, float r, float g, float b, float a)
{
	// no compiled caller
	(void)x1; (void)y1; (void)x2; (void)y2; (void)width; (void)r; (void)g; (void)b; (void)a;
}

//=============================================================================

void Draw_FadeScreen(void)
{
	float alpha;
	unsigned char col[4];

	alpha = bound(0, scr_menualpha.value, 1);
	if (!alpha)
		return;

	col[0] = 0;
	col[1] = 0;
	col[2] = 0;
	col[3] = alpha * 255;

	Draw2D_Quad(GPU_Texture_White(), 0, 0, 0, vid.conwidth, vid.conheight, 0, 0, 1, 1, col);

	Sbar_Changed();
}

//=============================================================================

struct WadHeader
{
	unsigned int width;
	unsigned short height;
};

struct LmpHeader
{
	unsigned int width;
	unsigned int height;
};

static void *Draw_LoadWadPicture(const char *name, unsigned int *rwidth, unsigned int *rheight)
{
	struct WadHeader *header;
	unsigned int width;
	unsigned int height;
	void *data;
	void *newdata;

	data = W_GetLumpName(name);

	if (data) /* Always true, Quake sucks. */
	{
		header = data;

		width = header->width;
		height = header->height;

		if (width < 32768 && height < 32768)
		{
			newdata = malloc(width * height);
			if (newdata)
			{
				memcpy(newdata, (unsigned char *)data + 8, width * height);
				*rwidth = width;
				*rheight = height;
				return newdata;
			}
		}
	}

	return 0;
}

static void *Draw_LoadLmpPicture(FILE *fh, unsigned int *rwidth, unsigned int *rheight)
{
	struct LmpHeader header;
	void *data;
	int r;
	unsigned int width;
	unsigned int height;
	unsigned int size;

	r = fread(&header, 1, sizeof(header), fh);
	if (r == sizeof(header))
	{
		width = LittleLong(header.width);
		height = LittleLong(header.height);

		if (width < 32768 && height < 32768)
		{
			size = width * height;

			data = malloc(size);
			if (data)
			{
				r = fread(data, 1, size, fh);
				if (r == size)
				{
					*rwidth = width;
					*rheight = height;

					return data;
				}

				free(data);
			}
		}
	}

	return 0;
}

static void *Draw_8to32(unsigned char *source, unsigned int width, unsigned int height)
{
	unsigned int *dst;
	unsigned int i;
	unsigned int n;

	if (width == 0 || height == 0 || width >= 32768 || height >= 32768)
		return 0;

	dst = malloc(width * height * sizeof(*dst));
	if (dst)
	{
		n = width * height;
		for (i = 0; i < n; i++)
			dst[i] = d_8to24table[source[i]];
	}

	return dst;
}

struct Picture *Draw_BuildTranslatedMenuplyr(int top, int bottom)
{
	static struct Picture pic;
	static byte src[8192];
	static unsigned int src_w, src_h;
	static int src_state;	/* 0 = untried, 1 = loaded, -1 = missing */
	static int top_cache = -1;
	static int bottom_cache = -1;

	byte trans[256];
	byte rgba[8192 * 4];
	unsigned int i, n;
	SDL_GPUTexture *tex;

	if (src_state == 0)
	{
		FILE *fh;
		void *data;
		unsigned int w = 0, h = 0;

		if (FS_FOpenFile("gfx/menuplyr.lmp", &fh) != -1)
		{
			data = Draw_LoadLmpPicture(fh, &w, &h);
			fclose(fh);

			if (data && w > 0 && h > 0 && (unsigned int)(w * h) <= sizeof(src))
			{
				memcpy(src, data, w * h);
				src_w = w;
				src_h = h;
				src_state = 1;
			}
			else
			{
				src_state = -1;
			}

			if (data)
				free(data);
		}
		else
		{
			src_state = -1;
		}
	}

	if (src_state < 0)
		return 0;

	// rebuild texture if the GPU device was torn down by vid_restart
	if (pic.texnum != 0 && GPU_Texture_Lookup(pic.texnum, NULL) == NULL)
	{
		pic.texnum = 0;
		top_cache = -1;
		bottom_cache = -1;
	}

	if (pic.texnum != 0 && top == top_cache && bottom == bottom_cache)
		return &pic;

	for (i = 0; i < 256; i++)
		trans[i] = (byte)i;

	if (top < 128)
	{
		for (i = 0; i < 16; i++)
			trans[16 + i] = (byte)(top + i);
	}
	else
	{
		// id palette skin ranges run backwards; mirror for slider direction
		for (i = 0; i < 16; i++)
			trans[16 + i] = (byte)(top + 15 - i);
	}

	if (bottom < 128)
	{
		for (i = 0; i < 16; i++)
			trans[96 + i] = (byte)(bottom + i);
	}
	else
	{
		for (i = 0; i < 16; i++)
			trans[96 + i] = (byte)(bottom + 15 - i);
	}

	n = src_w * src_h;
	for (i = 0; i < n; i++)
	{
		byte t = trans[src[i]];

		if (t == 255)
		{
			rgba[i * 4 + 0] = 0;
			rgba[i * 4 + 1] = 0;
			rgba[i * 4 + 2] = 0;
			rgba[i * 4 + 3] = 0;
		}
		else
		{
			rgba[i * 4 + 0] = host_basepal[t * 3 + 0];
			rgba[i * 4 + 1] = host_basepal[t * 3 + 1];
			rgba[i * 4 + 2] = host_basepal[t * 3 + 2];
			rgba[i * 4 + 3] = 255;
		}
	}

	if (pic.texnum == 0)
		pic.texnum = texture_extension_number++;

	tex = GPU_CreateTextureRGBA(rgba, src_w, src_h, 0);
	GPU_Texture_Set(pic.texnum, tex, src_w, src_h, GPU_TEXPREF_CLAMP);

	pic.width = src_w;
	pic.height = src_h;
	pic.glwidthscale = 1.0f;
	pic.glheightscale = 1.0f;
	pic.texcoords[0] = 0; pic.texcoords[1] = 0;
	pic.texcoords[2] = 1; pic.texcoords[3] = 0;
	pic.texcoords[4] = 1; pic.texcoords[5] = 1;
	pic.texcoords[6] = 0; pic.texcoords[7] = 1;

	top_cache = top;
	bottom_cache = bottom;

	return &pic;
}

struct Picture *Draw_LoadPicture(const char *name, enum Draw_LoadPicture_Fallback fallback)
{
	char *newname;
	char *newnameextension;
	FILE *fh;
	unsigned int namelen;
	struct Picture *picture;
	unsigned int width;
	unsigned int height;
	void *data;
	void *newdata;
	SDL_GPUTexture *tex;

	data = 0;
	newdata = 0;
	picture = 0;
	width = 0;
	height = 0;

	if (strncmp(name, "wad:", 4) == 0)
	{
		namelen = strlen(name + 4);
		newname = malloc(namelen + strlen("textures/wad/") + 1);
		if (newname)
		{
			memcpy(newname, "textures/wad/", strlen("textures/wad/"));
			memcpy(newname + strlen("textures/wad/"), name + 4, namelen);
			newname[strlen("textures/wad/") + namelen] = 0;
			picture = Draw_LoadPicture(newname, DRAW_LOADPICTURE_NOFALLBACK);
			free(newname);
			if (picture)
				return picture;
		}
		newname = malloc(namelen + strlen("gfx/") + 1);
		if (newname)
		{
			memcpy(newname, "gfx/", strlen("gfx/"));
			memcpy(newname + strlen("gfx/"), name + 4, namelen);
			newname[strlen("gfx/") + namelen] = 0;
			picture = Draw_LoadPicture(newname, DRAW_LOADPICTURE_NOFALLBACK);
			free(newname);
			if (picture)
				return picture;
		}
		data = Draw_LoadWadPicture(name + 4, &width, &height);
	}
	else
	{
		namelen = strlen(name);

		newname = malloc(namelen + 4 + 1);
		if (newname)
		{
			COM_CopyAndStripExtension(name, newname, namelen + 1);

			newnameextension = newname + strlen(newname);

			strcpy(newnameextension, ".tga");
			newdata = Image_LoadTGA(0, newname, 0, 0, &width, &height);
#if USE_PNG
			if (!newdata)
			{
				strcpy(newnameextension, ".png");
				newdata = Image_LoadPNG(0, newname, 0, 0, &width, &height);
			}
#endif

			if (!newdata)
			{
				strcpy(newnameextension, ".pcx");
				data = Image_LoadPCX(0, newname, 0, 0, &width, &height);
			}

			free(newname);
		}

		if (!newdata && !data)
		{
			if (namelen > 4 && strcmp(name + namelen - 4, ".lmp") == 0)
			{
				FS_FOpenFile(name, &fh);
				if (fh)
				{
					data = Draw_LoadLmpPicture(fh, &width, &height);

					fclose(fh);
				}
			}
		}
	}

	if (data)
	{
		newdata = Draw_8to32(data, width, height);

		free(data);
	}

	if (newdata)
	{
		picture = malloc(sizeof(*picture));
		if (picture)
		{
			picture->texnum = texture_extension_number++;
			picture->width = width;
			picture->height = height;
			picture->glwidthscale = 1;
			picture->glheightscale = 1;

			tex = GPU_CreateTextureRGBA(newdata, width, height, 0);
			GPU_Texture_Set(picture->texnum, tex, width, height, GPU_TEXPREF_LINEAR | GPU_TEXPREF_CLAMP);

			picture->texcoords[0] = 0;
			picture->texcoords[1] = 0;

			picture->texcoords[2] = 1;
			picture->texcoords[3] = 0;

			picture->texcoords[4] = 1;
			picture->texcoords[5] = 1;

			picture->texcoords[6] = 0;
			picture->texcoords[7] = 1;
		}

		free(newdata);
	}

	if (picture)
		return picture;

	if (fallback == DRAW_LOADPICTURE_DUMMYFALLBACK)
		return dummypicture;

	return 0;
}

void Draw_FreePicture(struct Picture *picture)
{
	if (picture != dummypicture)
	{
		GPU_Texture_Set(picture->texnum, NULL, 0, 0, 0);

		free(picture);
	}
}

unsigned int Draw_GetPictureWidth(struct Picture *picture)
{
	return picture->width;
}

unsigned int Draw_GetPictureHeight(struct Picture *picture)
{
	return picture->height;
}

void Draw_DrawPicture(struct Picture *picture, int x, int y, unsigned int width, unsigned int height)
{
	Draw2D_Quad(picture->texnum, 1,
		x, y, x + width, y + height,
		picture->texcoords[0], picture->texcoords[1],
		picture->texcoords[4], picture->texcoords[5],
		colour_white);
}

void Draw_DrawPictureModulated(struct Picture *picture, int x, int y, unsigned int width, unsigned int height, float r, float g, float b, float alpha)
{
	unsigned char col[4];

	col[0] = bound(0, r, 1) * 255;
	col[1] = bound(0, g, 1) * 255;
	col[2] = bound(0, b, 1) * 255;
	col[3] = bound(0, alpha, 1) * 255;

	Draw2D_Quad(picture->texnum, 0,
		x, y, x + width, y + height,
		picture->texcoords[0], picture->texcoords[1],
		picture->texcoords[4], picture->texcoords[5],
		col);
}

void Draw_DrawSubPicture(struct Picture *picture, float sx, float sy, float swidth, float sheight, int x, int y, unsigned int width, unsigned int height)
{
	Draw2D_Quad(picture->texnum, 1,
		x, y, x + width, y + height,
		sx * picture->glwidthscale, sy * picture->glheightscale,
		(sx + swidth) * picture->glwidthscale, (sy + sheight) * picture->glheightscale,
		colour_white);
}

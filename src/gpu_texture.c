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

#include "quakedef.h"
#include "r_local.h"
#include "image.h"
#include "crc.h"
#include "gpu_local.h"

extern unsigned d_8to24table2[256];
extern float vid_gamma;
extern byte vid_gamma_table[256];

#define MAX_GPUTEXTURES 4096

struct texentry
{
	SDL_GPUTexture *tex;
	unsigned int width;
	unsigned int height;
	int prefs;
};

static struct texentry textable[MAX_GPUTEXTURES];
static int white_texnum;

// identifier cache, semantics from gl_texture.c
typedef struct gltexture_s
{
	int texnum;
	char identifier[64];
	int width, height;
	int scaled_width, scaled_height;
	int texmode;
	unsigned int crc;
	int bpp;
} gltexture_t;

static gltexture_t gltextures[MAX_GLTEXTURES];
static int numgltextures;

cvar_t gl_picmip = {"gl_picmip", "0"};
cvar_t gl_lerpimages = {"gl_lerpimages", "1"};
cvar_t gl_texturemode = {"gl_texturemode", "gl_nearest"};

static unsigned int mip_levels(unsigned int w, unsigned int h)
{
	unsigned int levels = 1;
	unsigned int m = w > h ? w : h;

	while (m > 1)
	{
		m >>= 1;
		levels++;
	}
	return levels;
}

SDL_GPUTexture *GPU_CreateTextureRGBA(const unsigned char *rgba, unsigned int width, unsigned int height, int mipmap)
{
	SDL_GPUDevice *device = GPU_GetDevice();
	SDL_GPUTexture *tex;
	SDL_GPUTextureCreateInfo ci;
	SDL_GPUTransferBufferCreateInfo tci;
	SDL_GPUTransferBuffer *tbuf;
	SDL_GPUCommandBuffer *cmdbuf;
	SDL_GPUCopyPass *copy;
	SDL_GPUTextureTransferInfo transfer;
	SDL_GPUTextureRegion region;
	void *mapped;

	if (!device || !rgba || !width || !height)
		return NULL;

	memset(&ci, 0, sizeof(ci));
	ci.type = SDL_GPU_TEXTURETYPE_2D;
	ci.format = SDL_GPU_TEXTUREFORMAT_R8G8B8A8_UNORM;
	ci.usage = SDL_GPU_TEXTUREUSAGE_SAMPLER;
	ci.width = width;
	ci.height = height;
	ci.layer_count_or_depth = 1;
	ci.num_levels = 1;
	ci.sample_count = SDL_GPU_SAMPLECOUNT_1;

	if (mipmap)
	{
		// mip chain generated on GPU, needs color target usage
		ci.usage |= SDL_GPU_TEXTUREUSAGE_COLOR_TARGET;
		ci.num_levels = mip_levels(width, height);
	}

	tex = SDL_CreateGPUTexture(device, &ci);
	if (!tex)
	{
		Com_Printf("GPU: texture create failed: %s\n", SDL_GetError());
		return NULL;
	}

	memset(&tci, 0, sizeof(tci));
	tci.usage = SDL_GPU_TRANSFERBUFFERUSAGE_UPLOAD;
	tci.size = width * height * 4;
	tbuf = SDL_CreateGPUTransferBuffer(device, &tci);
	if (!tbuf)
	{
		SDL_ReleaseGPUTexture(device, tex);
		return NULL;
	}

	mapped = SDL_MapGPUTransferBuffer(device, tbuf, false);
	if (!mapped)
	{
		SDL_ReleaseGPUTransferBuffer(device, tbuf);
		SDL_ReleaseGPUTexture(device, tex);
		return NULL;
	}
	memcpy(mapped, rgba, width * height * 4);
	SDL_UnmapGPUTransferBuffer(device, tbuf);

	cmdbuf = SDL_AcquireGPUCommandBuffer(device);
	if (!cmdbuf)
	{
		SDL_ReleaseGPUTransferBuffer(device, tbuf);
		SDL_ReleaseGPUTexture(device, tex);
		return NULL;
	}

	copy = SDL_BeginGPUCopyPass(cmdbuf);

	memset(&transfer, 0, sizeof(transfer));
	transfer.transfer_buffer = tbuf;

	memset(&region, 0, sizeof(region));
	region.texture = tex;
	region.w = width;
	region.h = height;
	region.d = 1;

	SDL_UploadToGPUTexture(copy, &transfer, &region, false);
	SDL_EndGPUCopyPass(copy);

	if (mipmap && ci.num_levels > 1)
		SDL_GenerateMipmapsForGPUTexture(cmdbuf, tex);

	SDL_SubmitGPUCommandBuffer(cmdbuf);
	SDL_ReleaseGPUTransferBuffer(device, tbuf);

	return tex;
}

void GPU_Texture_Set(int texnum, SDL_GPUTexture *tex, unsigned int width, unsigned int height, int prefs)
{
	SDL_GPUDevice *device = GPU_GetDevice();

	if (texnum < 0 || texnum >= MAX_GPUTEXTURES)
	{
		Com_Printf("GPU: texnum %d out of range\n", texnum);
		if (tex && device)
			SDL_ReleaseGPUTexture(device, tex);
		return;
	}

	if (textable[texnum].tex && device)
		SDL_ReleaseGPUTexture(device, textable[texnum].tex);

	textable[texnum].tex = tex;
	textable[texnum].width = width;
	textable[texnum].height = height;
	textable[texnum].prefs = prefs;
}

void GPU_Texture_SetPrefs(int texnum, int prefs)
{
	if (texnum >= 0 && texnum < MAX_GPUTEXTURES)
		textable[texnum].prefs = prefs;
}

void GPU_UpdateTextureRGBA(int texnum, const unsigned char *rgba, unsigned int width, unsigned int height, int prefs)
{
	SDL_GPUDevice *device = GPU_GetDevice();
	struct texentry *e;
	SDL_GPUTransferBufferCreateInfo tci;
	SDL_GPUTransferBuffer *tbuf;
	SDL_GPUCommandBuffer *cmdbuf;
	SDL_GPUCopyPass *copy;
	SDL_GPUTextureTransferInfo transfer;
	SDL_GPUTextureRegion region;
	void *mapped;

	if (!device || texnum < 0 || texnum >= MAX_GPUTEXTURES)
		return;

	e = &textable[texnum];
	if (!e->tex || e->width != width || e->height != height)
	{
		GPU_Texture_Set(texnum, GPU_CreateTextureRGBA(rgba, width, height, 0), width, height, prefs);
		return;
	}

	e->prefs = prefs;

	memset(&tci, 0, sizeof(tci));
	tci.usage = SDL_GPU_TRANSFERBUFFERUSAGE_UPLOAD;
	tci.size = width * height * 4;
	tbuf = SDL_CreateGPUTransferBuffer(device, &tci);
	if (!tbuf)
		return;

	mapped = SDL_MapGPUTransferBuffer(device, tbuf, false);
	if (!mapped)
	{
		SDL_ReleaseGPUTransferBuffer(device, tbuf);
		return;
	}
	memcpy(mapped, rgba, width * height * 4);
	SDL_UnmapGPUTransferBuffer(device, tbuf);

	cmdbuf = SDL_AcquireGPUCommandBuffer(device);
	if (!cmdbuf)
	{
		SDL_ReleaseGPUTransferBuffer(device, tbuf);
		return;
	}

	copy = SDL_BeginGPUCopyPass(cmdbuf);

	memset(&transfer, 0, sizeof(transfer));
	transfer.transfer_buffer = tbuf;

	memset(&region, 0, sizeof(region));
	region.texture = e->tex;
	region.w = width;
	region.h = height;
	region.d = 1;

	// cycle avoids stalling on the previous frame still sampling it
	SDL_UploadToGPUTexture(copy, &transfer, &region, true);
	SDL_EndGPUCopyPass(copy);
	SDL_SubmitGPUCommandBuffer(cmdbuf);
	SDL_ReleaseGPUTransferBuffer(device, tbuf);
}

SDL_GPUTexture *GPU_Texture_Lookup(int texnum, int *prefs)
{
	if (texnum < 0 || texnum >= MAX_GPUTEXTURES || !textable[texnum].tex)
		return NULL;
	if (prefs)
		*prefs = textable[texnum].prefs;
	return textable[texnum].tex;
}

int GPU_Texture_White(void)
{
	return white_texnum;
}

void GPU_Texture_InitTable(void)
{
	static const unsigned char white[4] = {255, 255, 255, 255};
	SDL_GPUTexture *tex;

	memset(textable, 0, sizeof(textable));

	if (!white_texnum)
		white_texnum = texture_extension_number++;
	tex = GPU_CreateTextureRGBA(white, 1, 1, 0);
	GPU_Texture_Set(white_texnum, tex, 1, 1, 0);
}

void GPU_Texture_ShutdownTable(void)
{
	SDL_GPUDevice *device = GPU_GetDevice();
	int i;

	for (i = 0; i < MAX_GPUTEXTURES; i++)
	{
		if (textable[i].tex && device)
			SDL_ReleaseGPUTexture(device, textable[i].tex);
		textable[i].tex = NULL;
	}
}

// ---- engine-facing texture API (gl_texture.c semantics) ----

void R_Bind(int texnum)
{
	currenttexture = texnum;
}

static void scale_dimensions(int width, int height, int *scaled_width, int *scaled_height, int mode)
{
	int w = width, h = height;

	// NPOT is native now, only picmip and the size cap remain
	if ((mode & TEX_MIPMAP) && !(mode & TEX_NOSCALE))
	{
		int shift = (int)bound(0, gl_picmip.value, 16);
		w >>= shift;
		h >>= shift;
		if (w > gl_max_size.value) w = gl_max_size.value;
		if (h > gl_max_size.value) h = gl_max_size.value;
	}
	else
	{
		if (w > gl_max_size_default) w = gl_max_size_default;
		if (h > gl_max_size_default) h = gl_max_size_default;
	}

	*scaled_width = max(1, w);
	*scaled_height = max(1, h);
}

// mag filter depends only on the gl_nearest/gl_linear prefix
int GPU_PrefsForTexturemode(void)
{
	int nearest;

	nearest = Q_strcasecmp(gl_texturemode.string, "gl_nearest") == 0
		|| Q_strncasecmp(gl_texturemode.string, "gl_nearest_", 11) == 0;

	return nearest ? 0 : GPU_TEXPREF_LINEAR;
}

static int prefs_for_mode(int mode)
{
	(void)mode;
	return GPU_PrefsForTexturemode();
}

void R_Upload32(unsigned *data, int width, int height, int mode)
{
	int scaled_width, scaled_height;
	unsigned char *scaled = NULL;
	const unsigned char *pixels = (const unsigned char *)data;
	SDL_GPUTexture *tex;

	scale_dimensions(width, height, &scaled_width, &scaled_height, mode);

	if (scaled_width != width || scaled_height != height)
	{
		scaled = malloc(scaled_width * scaled_height * 4);
		if (!scaled)
			return;
		Image_Resample(data, width, height, scaled, scaled_width, scaled_height, 4, gl_lerpimages.value != 0);
		pixels = scaled;
	}

	tex = GPU_CreateTextureRGBA(pixels, scaled_width, scaled_height, (mode & TEX_MIPMAP) != 0);
	GPU_Texture_Set(currenttexture, tex, scaled_width, scaled_height, prefs_for_mode(mode));

	free(scaled);
}

void R_Upload8(byte *data, int width, int height, int mode)
{
	unsigned int *buf;
	unsigned int *table;
	int i, size, has_alpha = 0;

	size = width * height;
	buf = malloc(size * 4);
	if (!buf)
		return;

	table = (mode & TEX_BRIGHTEN) ? d_8to24table2 : d_8to24table;

	if (mode & TEX_FULLBRIGHT)
	{
		mode |= TEX_ALPHA;
		for (i = 0; i < size; i++)
		{
			if (data[i] < 224)
				buf[i] = table[data[i]] & COLOURMASK_RGBA;	// zero alpha
			else
				buf[i] = table[data[i]];
		}
	}
	else
	{
		for (i = 0; i < size; i++)
		{
			buf[i] = table[data[i]];
			if (data[i] == 255)
				has_alpha = 1;
		}
		if ((mode & TEX_ALPHA) && !has_alpha)
			mode &= ~TEX_ALPHA;
	}

	R_Upload32(buf, width, height, mode);
	free(buf);
}

static gltexture_t *find_texture(const char *identifier)
{
	int i;

	for (i = 0; i < numgltextures; i++)
	{
		if (!strncmp(identifier, gltextures[i].identifier, sizeof(gltextures[i].identifier) - 1))
			return &gltextures[i];
	}
	return NULL;
}

int R_LoadTexture(char *identifier, int width, int height, byte *data, int mode, int bpp)
{
	gltexture_t *glt = NULL;
	int scaled_width, scaled_height;
	unsigned int crc = 0;

	scale_dimensions(width, height, &scaled_width, &scaled_height, mode);

	if (identifier && identifier[0])
	{
		crc = CRC_Block(data, width * height * bpp);
		glt = find_texture(identifier);
		if (glt)
		{
			if (glt->width == width && glt->height == height
				&& glt->scaled_width == scaled_width && glt->scaled_height == scaled_height
				&& glt->crc == crc && glt->bpp == bpp
				&& (glt->texmode & ~(TEX_COMPLAIN | TEX_NOSCALE)) == (mode & ~(TEX_COMPLAIN | TEX_NOSCALE)))
			{
				R_Bind(glt->texnum);
				return glt->texnum;
			}
			// same name, different content: re-upload into the same slot
		}
	}

	if (!glt)
	{
		if (numgltextures >= MAX_GLTEXTURES)
			Sys_Error("R_LoadTexture: cache full");
		glt = &gltextures[numgltextures++];
		memset(glt, 0, sizeof(*glt));
		if (identifier)
			Q_strncpyz(glt->identifier, identifier, sizeof(glt->identifier));
		glt->texnum = texture_extension_number++;
	}

	glt->width = width;
	glt->height = height;
	glt->scaled_width = scaled_width;
	glt->scaled_height = scaled_height;
	glt->texmode = mode;
	glt->crc = crc;
	glt->bpp = bpp;

	R_Bind(glt->texnum);
	if (bpp == 1)
		R_Upload8(data, width, height, mode);
	else if (bpp == 4)
		R_Upload32((unsigned *)data, width, height, mode);
	else
		Sys_Error("R_LoadTexture: unsupported bpp %d", bpp);

	return glt->texnum;
}

byte *R_LoadImagePixels(char *filename, int matchwidth, int matchheight, unsigned int *imagewidth, unsigned int *imageheight, int mode)
{
	char basename[MAX_QPATH], name[MAX_QPATH];
	byte *pixels;
	char *c;

	COM_CopyAndStripExtension(filename, basename, sizeof(basename));
	for (c = basename; *c; c++)
	{
		if (*c == '*')
			*c = '#';
	}

	snprintf(name, sizeof(name), "%s.tga", basename);
	pixels = Image_LoadTGA(NULL, name, matchwidth, matchheight, imagewidth, imageheight);
	if (pixels)
		return pixels;

#if USE_PNG
	snprintf(name, sizeof(name), "%s.png", basename);
	pixels = Image_LoadPNG(NULL, name, matchwidth, matchheight, imagewidth, imageheight);
	if (pixels)
		return pixels;
#endif

	if (mode & TEX_COMPLAIN)
		Com_Printf("Couldn't load %s image\n", COM_SkipPath(filename));

	return NULL;
}

int R_LoadTexturePixels(byte *data, char *identifier, int width, int height, int mode)
{
	int i, j, image_size;

	image_size = width * height;

	if (!(mode & TEX_LUMA) && vid_gamma != 1)
	{
		for (i = 0; i < image_size; i++)
		{
			data[4 * i + 0] = vid_gamma_table[data[4 * i + 0]];
			data[4 * i + 1] = vid_gamma_table[data[4 * i + 1]];
			data[4 * i + 2] = vid_gamma_table[data[4 * i + 2]];
		}
	}

	if (mode & TEX_ALPHA)
	{
		mode &= ~TEX_ALPHA;
		for (j = 0; j < image_size; j++)
		{
			if (data[4 * j + 3] < 255)
			{
				mode |= TEX_ALPHA;
				break;
			}
		}
	}

	return R_LoadTexture(identifier, width, height, data, mode, 4);
}

int R_LoadTextureImage(char *filename, char *identifier, int matchwidth, int matchheight, int mode)
{
	byte *pixels;
	unsigned int w, h;
	int texnum;

	if (!identifier)
		identifier = filename;

	pixels = R_LoadImagePixels(filename, matchwidth, matchheight, &w, &h, mode);
	if (!pixels)
		return 0;

	texnum = R_LoadTexturePixels(pixels, identifier, w, h, mode);
	free(pixels);
	return texnum;
}

int R_LoadCharsetImage(char *filename, char *identifier)
{
	byte *pixels, *buf, *src, *dest;
	unsigned int w, h;
	int i, image_size, texnum;

	pixels = R_LoadImagePixels(filename, 0, 0, &w, &h, 0);
	if (!pixels)
		return 0;

	if (w >= 32768 || h >= 32768 || w * h >= (1u << 28))
	{
		free(pixels);
		return 0;
	}

	if (!identifier)
		identifier = filename;

	image_size = w * h;

	// blank rows between character rows keep LINEAR from bleeding
	buf = calloc(image_size * 2, 4);
	if (!buf)
	{
		free(pixels);
		return 0;
	}

	src = pixels;
	dest = buf;
	for (i = 0; i < 16; i++)
	{
		memcpy(dest, src, image_size >> 2);
		src += image_size >> 2;
		dest += image_size >> 1;
	}

	texnum = R_LoadTexture(identifier, w, h * 2, buf, TEX_ALPHA | TEX_NOCOMPRESS, 4);

	free(buf);
	free(pixels);
	return texnum;
}

void R_Texture_CvarInit(void)
{
	Cvar_SetCurrentGroup(CVAR_GROUP_TEXTURES);
	Cvar_Register(&gl_picmip);
	Cvar_Register(&gl_lerpimages);
	Cvar_Register(&gl_texturemode);
	Cvar_ResetCurrentGroup();
}

void R_Texture_Init(void)
{
	numgltextures = 0;
}

void R_Texture_Shutdown(void)
{
	numgltextures = 0;
}

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
#include "gpu_local.h"

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

extern int texture_extension_number;

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

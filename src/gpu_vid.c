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

static SDL_GPUDevice *gpu_device;
static SDL_Window *gpu_window;

static SDL_GPUTexture *scene_color;
static SDL_GPUTexture *scene_depth;
static SDL_GPUTextureFormat scene_depth_format;
static unsigned int scene_width, scene_height;

static SDL_GPUCommandBuffer *frame_cmdbuf;
static int frame_scene_cleared;

static SDL_GPUTransferBuffer *readback_buffer;
static unsigned int readback_size;

static void destroy_scene_targets(void)
{
	if (scene_color)
	{
		SDL_ReleaseGPUTexture(gpu_device, scene_color);
		scene_color = NULL;
	}
	if (scene_depth)
	{
		SDL_ReleaseGPUTexture(gpu_device, scene_depth);
		scene_depth = NULL;
	}
	scene_width = 0;
	scene_height = 0;
}

static int create_scene_targets(unsigned int width, unsigned int height)
{
	SDL_GPUTextureCreateInfo info;

	destroy_scene_targets();

	memset(&info, 0, sizeof(info));
	info.type = SDL_GPU_TEXTURETYPE_2D;
	info.format = SDL_GPU_TEXTUREFORMAT_R8G8B8A8_UNORM;
	info.usage = SDL_GPU_TEXTUREUSAGE_COLOR_TARGET | SDL_GPU_TEXTUREUSAGE_SAMPLER;
	info.width = width;
	info.height = height;
	info.layer_count_or_depth = 1;
	info.num_levels = 1;
	info.sample_count = SDL_GPU_SAMPLECOUNT_1;

	scene_color = SDL_CreateGPUTexture(gpu_device, &info);
	if (!scene_color)
	{
		Com_Printf("GPU: scene color target failed: %s\n", SDL_GetError());
		return 0;
	}

	info.format = scene_depth_format;
	info.usage = SDL_GPU_TEXTUREUSAGE_DEPTH_STENCIL_TARGET;

	scene_depth = SDL_CreateGPUTexture(gpu_device, &info);
	if (!scene_depth)
	{
		Com_Printf("GPU: scene depth target failed: %s\n", SDL_GetError());
		destroy_scene_targets();
		return 0;
	}

	scene_width = width;
	scene_height = height;
	return 1;
}

void GPU_SetVsync(int vsync)
{
	SDL_GPUPresentMode mode = SDL_GPU_PRESENTMODE_VSYNC;

	if (!gpu_device || !gpu_window)
		return;

	if (vsync == 0 && SDL_WindowSupportsGPUPresentMode(gpu_device, gpu_window, SDL_GPU_PRESENTMODE_IMMEDIATE))
		mode = SDL_GPU_PRESENTMODE_IMMEDIATE;
	else if (vsync == -1 && SDL_WindowSupportsGPUPresentMode(gpu_device, gpu_window, SDL_GPU_PRESENTMODE_MAILBOX))
		mode = SDL_GPU_PRESENTMODE_MAILBOX;

	SDL_SetGPUSwapchainParameters(gpu_device, gpu_window, SDL_GPU_SWAPCHAINCOMPOSITION_SDR, mode);
}

int GPU_Init(SDL_Window *window, int vsync)
{
	gpu_device = SDL_CreateGPUDevice(
		SDL_GPU_SHADERFORMAT_SPIRV | SDL_GPU_SHADERFORMAT_DXIL | SDL_GPU_SHADERFORMAT_MSL,
		COM_CheckParm("-gpudebug") != 0,
		NULL);
	if (!gpu_device)
	{
		Com_Printf("SDL_CreateGPUDevice failed: %s\n", SDL_GetError());
		return 0;
	}

	if (!SDL_ClaimWindowForGPUDevice(gpu_device, window))
	{
		Com_Printf("SDL_ClaimWindowForGPUDevice failed: %s\n", SDL_GetError());
		SDL_DestroyGPUDevice(gpu_device);
		gpu_device = NULL;
		return 0;
	}

	gpu_window = window;

	if (SDL_GPUTextureSupportsFormat(gpu_device, SDL_GPU_TEXTUREFORMAT_D24_UNORM,
		SDL_GPU_TEXTURETYPE_2D, SDL_GPU_TEXTUREUSAGE_DEPTH_STENCIL_TARGET))
		scene_depth_format = SDL_GPU_TEXTUREFORMAT_D24_UNORM;
	else
		scene_depth_format = SDL_GPU_TEXTUREFORMAT_D32_FLOAT;

	GPU_SetVsync(vsync);

	Com_Printf("GPU driver: %s\n", SDL_GetGPUDeviceDriver(gpu_device));

	return 1;
}

void GPU_Shutdown(void)
{
	if (!gpu_device)
		return;

	if (frame_cmdbuf)
	{
		SDL_SubmitGPUCommandBuffer(frame_cmdbuf);
		frame_cmdbuf = NULL;
	}

	SDL_WaitForGPUIdle(gpu_device);

	if (readback_buffer)
	{
		SDL_ReleaseGPUTransferBuffer(gpu_device, readback_buffer);
		readback_buffer = NULL;
		readback_size = 0;
	}
	destroy_scene_targets();

	if (gpu_window)
		SDL_ReleaseWindowFromGPUDevice(gpu_device, gpu_window);
	SDL_DestroyGPUDevice(gpu_device);
	gpu_device = NULL;
	gpu_window = NULL;
}

void GPU_BeginFrame(unsigned int width, unsigned int height)
{
	if (!gpu_device || frame_cmdbuf)
		return;

	if (width == 0 || height == 0)
		return;

	if (width != scene_width || height != scene_height)
	{
		SDL_WaitForGPUIdle(gpu_device);
		if (!create_scene_targets(width, height))
			return;
	}

	frame_cmdbuf = SDL_AcquireGPUCommandBuffer(gpu_device);
	if (!frame_cmdbuf)
	{
		Com_Printf("GPU: command buffer acquire failed: %s\n", SDL_GetError());
		return;
	}
	frame_scene_cleared = 0;
}

// M1: scene pass only clears; draw modules will hook in here later
static void ensure_scene_cleared(void)
{
	SDL_GPUColorTargetInfo ct;
	SDL_GPUDepthStencilTargetInfo ds;
	SDL_GPURenderPass *pass;

	if (frame_scene_cleared || !frame_cmdbuf || !scene_color)
		return;

	memset(&ct, 0, sizeof(ct));
	ct.texture = scene_color;
	ct.clear_color.r = 0.15f;
	ct.clear_color.g = 0.15f;
	ct.clear_color.b = 0.15f;
	ct.clear_color.a = 1.0f;
	ct.load_op = SDL_GPU_LOADOP_CLEAR;
	ct.store_op = SDL_GPU_STOREOP_STORE;
	ct.cycle = true;

	memset(&ds, 0, sizeof(ds));
	ds.texture = scene_depth;
	ds.clear_depth = 1.0f;
	ds.load_op = SDL_GPU_LOADOP_CLEAR;
	ds.store_op = SDL_GPU_STOREOP_DONT_CARE;
	ds.stencil_load_op = SDL_GPU_LOADOP_DONT_CARE;
	ds.stencil_store_op = SDL_GPU_STOREOP_DONT_CARE;
	ds.cycle = true;

	pass = SDL_BeginGPURenderPass(frame_cmdbuf, &ct, 1, &ds);
	if (pass)
		SDL_EndGPURenderPass(pass);

	frame_scene_cleared = 1;
}

void GPU_EndFrame(void)
{
	SDL_GPUTexture *swap_tex;
	Uint32 swap_w, swap_h;

	if (!gpu_device || !frame_cmdbuf)
		return;

	ensure_scene_cleared();

	if (!SDL_WaitAndAcquireGPUSwapchainTexture(frame_cmdbuf, gpu_window, &swap_tex, &swap_w, &swap_h))
		swap_tex = NULL;

	if (swap_tex)
	{
		SDL_GPUBlitInfo blit;

		memset(&blit, 0, sizeof(blit));
		blit.source.texture = scene_color;
		blit.source.w = scene_width;
		blit.source.h = scene_height;
		blit.destination.texture = swap_tex;
		blit.destination.w = swap_w;
		blit.destination.h = swap_h;
		blit.load_op = SDL_GPU_LOADOP_DONT_CARE;
		blit.filter = SDL_GPU_FILTER_NEAREST;

		SDL_BlitGPUTexture(frame_cmdbuf, &blit);
	}

	SDL_SubmitGPUCommandBuffer(frame_cmdbuf);
	frame_cmdbuf = NULL;

	// TEMP M1 test hook: -autoshot takes a screenshot at frame 60, quits at 70
	{
		static int frames, enabled = -1;
		if (enabled == -1)
			enabled = COM_CheckParm("-autoshot") != 0;
		if (enabled)
		{
			frames++;
			if (frames == 60)
				Cbuf_AddText("screenshot autoshot\n");
			if (frames == 70)
				Host_Quit();
		}
	}
}

SDL_GPUDevice *GPU_GetDevice(void)
{
	return gpu_device;
}

SDL_GPUCommandBuffer *GPU_GetCommandBuffer(void)
{
	return frame_cmdbuf;
}

SDL_GPUTexture *GPU_GetSceneColor(void)
{
	return scene_color;
}

SDL_GPUTexture *GPU_GetSceneDepth(void)
{
	return scene_depth;
}

SDL_GPUTextureFormat GPU_GetSceneDepthFormat(void)
{
	return scene_depth_format;
}

int GPU_GetSceneMatrices(float *modelview, float *projection, int *viewport)
{
	// filled in once the 3D scene renders through gpu modules
	(void)modelview;
	(void)projection;
	(void)viewport;
	return 0;
}

int GPU_ReadPixels(unsigned char *rgb, unsigned int width, unsigned int height)
{
	SDL_GPUCommandBuffer *cmdbuf;
	SDL_GPUCopyPass *copy;
	SDL_GPUTextureRegion region;
	SDL_GPUTextureTransferInfo transfer;
	SDL_GPUFence *fence;
	unsigned char *mapped;
	unsigned int needed, x, y;

	if (!gpu_device || !scene_color || width != scene_width || height != scene_height)
		return 0;

	needed = width * height * 4;
	if (readback_buffer && readback_size < needed)
	{
		SDL_ReleaseGPUTransferBuffer(gpu_device, readback_buffer);
		readback_buffer = NULL;
	}
	if (!readback_buffer)
	{
		SDL_GPUTransferBufferCreateInfo info;
		memset(&info, 0, sizeof(info));
		info.usage = SDL_GPU_TRANSFERBUFFERUSAGE_DOWNLOAD;
		info.size = needed;
		readback_buffer = SDL_CreateGPUTransferBuffer(gpu_device, &info);
		if (!readback_buffer)
			return 0;
		readback_size = needed;
	}

	// mid-frame: append download to the open frame cmdbuf and flush it
	// (that frame skips presentation); between frames: own cmdbuf
	if (frame_cmdbuf)
	{
		ensure_scene_cleared();
		cmdbuf = frame_cmdbuf;
		frame_cmdbuf = NULL;
	}
	else
	{
		cmdbuf = SDL_AcquireGPUCommandBuffer(gpu_device);
		if (!cmdbuf)
			return 0;
	}

	copy = SDL_BeginGPUCopyPass(cmdbuf);

	memset(&region, 0, sizeof(region));
	region.texture = scene_color;
	region.w = width;
	region.h = height;
	region.d = 1;

	memset(&transfer, 0, sizeof(transfer));
	transfer.transfer_buffer = readback_buffer;

	SDL_DownloadFromGPUTexture(copy, &region, &transfer);
	SDL_EndGPUCopyPass(copy);

	fence = SDL_SubmitGPUCommandBufferAndAcquireFence(cmdbuf);
	if (!fence)
		return 0;
	SDL_WaitForGPUFences(gpu_device, true, &fence, 1);
	SDL_ReleaseGPUFence(gpu_device, fence);

	mapped = SDL_MapGPUTransferBuffer(gpu_device, readback_buffer, false);
	if (!mapped)
		return 0;

	// flip to bottom-up and drop alpha, glReadPixels convention
	for (y = 0; y < height; y++)
	{
		const unsigned char *src = mapped + (height - 1 - y) * width * 4;
		unsigned char *dst = rgb + y * width * 3;
		for (x = 0; x < width; x++)
		{
			dst[x * 3 + 0] = src[x * 4 + 0];
			dst[x * 3 + 1] = src[x * 4 + 1];
			dst[x * 3 + 2] = src[x * 4 + 2];
		}
	}

	SDL_UnmapGPUTransferBuffer(gpu_device, readback_buffer);
	return 1;
}

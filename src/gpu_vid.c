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

#include "shaders_gen.h"	// regenerate via src/shaders/compile.ps1 after edits

#define UI_MAX_VERTS GPU_UI_MAX_VERTS

static SDL_GPUDevice *gpu_device;
static SDL_Window *gpu_window;

static SDL_GPUTexture *scene_color;
static SDL_GPUTexture *scene_depth;
static SDL_GPUTextureFormat scene_depth_format;
static SDL_GPUTextureFormat swapchain_format;
static unsigned int scene_width, scene_height;

static SDL_GPUCommandBuffer *frame_cmdbuf;

static SDL_GPUGraphicsPipeline *pipe_ui;
static SDL_GPUGraphicsPipeline *pipe_ui_alphatest;
static SDL_GPUGraphicsPipeline *pipe_post;
static SDL_GPUGraphicsPipeline *pipe_scene[SCENE_PIPE_COUNT];

static SDL_GPUBuffer *scene_ibuf;
static SDL_GPUTransferBuffer *scene_itbuf;
static unsigned int scene_indices[GPU_SCENE_MAX_INDICES];
static unsigned int scene_numindices;
static SDL_GPUBuffer *scene_dynvbuf;
static SDL_GPUTransferBuffer *scene_dyntbuf;
static scene_vert_t scene_dynverts[GPU_SCENE_MAX_DYNVERTS];
static unsigned int scene_numdynverts;
static scene_batch_t scene_batches[GPU_SCENE_MAX_BATCHES];
static unsigned int scene_numbatches;
static float scene_viewport[4];
static int scene_has_viewport;

static void (*scene_uploader)(SDL_GPUCopyPass *copy);

static float autoid_modelview[16];
static float autoid_projection[16];
static int autoid_viewport[4];
static int autoid_valid;

static SDL_GPUSampler *samp_nearest;
static SDL_GPUSampler *samp_linear;
static SDL_GPUSampler *samp_nearest_clamp;
static SDL_GPUSampler *samp_linear_clamp;

static SDL_GPUBuffer *ui_vbuf;
static SDL_GPUTransferBuffer *ui_tbuf;

static SDL_GPUTransferBuffer *readback_buffer;
static unsigned int readback_size;

static float post_gamma = 1.0f;
static float post_contrast = 1.0f;
static float post_blend[4];

#define SHADER_ARGS(n) n##_spv, sizeof(n##_spv), n##_dxil, sizeof(n##_dxil), n##_msl, sizeof(n##_msl)

static SDL_GPUShader *load_shader(SDL_GPUShaderStage stage,
	const unsigned char *spv, unsigned int spv_len,
	const unsigned char *dxil, unsigned int dxil_len,
	const unsigned char *msl, unsigned int msl_len,
	Uint32 num_samplers, Uint32 num_uniform_buffers)
{
	SDL_GPUShaderCreateInfo ci;
	SDL_GPUShaderFormat formats = SDL_GetGPUShaderFormats(gpu_device);

	memset(&ci, 0, sizeof(ci));
	if (formats & SDL_GPU_SHADERFORMAT_SPIRV)
	{
		ci.format = SDL_GPU_SHADERFORMAT_SPIRV;
		ci.code = spv;
		ci.code_size = spv_len;
		ci.entrypoint = "main";
	}
	else if (formats & SDL_GPU_SHADERFORMAT_DXIL)
	{
		ci.format = SDL_GPU_SHADERFORMAT_DXIL;
		ci.code = dxil;
		ci.code_size = dxil_len;
		ci.entrypoint = "main";
	}
	else if (formats & SDL_GPU_SHADERFORMAT_MSL)
	{
		ci.format = SDL_GPU_SHADERFORMAT_MSL;
		ci.code = msl;
		ci.code_size = msl_len;
		ci.entrypoint = "main0";
	}
	else
	{
		Com_Printf("GPU: no supported shader format\n");
		return NULL;
	}

	ci.stage = stage;
	ci.num_samplers = num_samplers;
	ci.num_uniform_buffers = num_uniform_buffers;

	return SDL_CreateGPUShader(gpu_device, &ci);
}

static SDL_GPUGraphicsPipeline *make_ui_pipeline(SDL_GPUShader *vs, SDL_GPUShader *fs)
{
	SDL_GPUGraphicsPipelineCreateInfo ci;
	SDL_GPUVertexBufferDescription vbd;
	SDL_GPUVertexAttribute attrs[3];
	SDL_GPUColorTargetDescription ct;

	memset(&ci, 0, sizeof(ci));
	memset(&vbd, 0, sizeof(vbd));
	memset(&attrs, 0, sizeof(attrs));
	memset(&ct, 0, sizeof(ct));

	vbd.slot = 0;
	vbd.pitch = sizeof(ui_vert_t);
	vbd.input_rate = SDL_GPU_VERTEXINPUTRATE_VERTEX;

	attrs[0].location = 0;
	attrs[0].format = SDL_GPU_VERTEXELEMENTFORMAT_FLOAT2;
	attrs[0].offset = 0;
	attrs[1].location = 1;
	attrs[1].format = SDL_GPU_VERTEXELEMENTFORMAT_FLOAT2;
	attrs[1].offset = 8;
	attrs[2].location = 2;
	attrs[2].format = SDL_GPU_VERTEXELEMENTFORMAT_UBYTE4_NORM;
	attrs[2].offset = 16;

	ct.format = SDL_GPU_TEXTUREFORMAT_R8G8B8A8_UNORM;
	ct.blend_state.enable_blend = true;
	ct.blend_state.color_blend_op = SDL_GPU_BLENDOP_ADD;
	ct.blend_state.alpha_blend_op = SDL_GPU_BLENDOP_ADD;
	ct.blend_state.src_color_blendfactor = SDL_GPU_BLENDFACTOR_SRC_ALPHA;
	ct.blend_state.dst_color_blendfactor = SDL_GPU_BLENDFACTOR_ONE_MINUS_SRC_ALPHA;
	ct.blend_state.src_alpha_blendfactor = SDL_GPU_BLENDFACTOR_ONE;
	ct.blend_state.dst_alpha_blendfactor = SDL_GPU_BLENDFACTOR_ONE_MINUS_SRC_ALPHA;

	ci.vertex_shader = vs;
	ci.fragment_shader = fs;
	ci.vertex_input_state.vertex_buffer_descriptions = &vbd;
	ci.vertex_input_state.num_vertex_buffers = 1;
	ci.vertex_input_state.vertex_attributes = attrs;
	ci.vertex_input_state.num_vertex_attributes = 3;
	ci.primitive_type = SDL_GPU_PRIMITIVETYPE_TRIANGLELIST;
	ci.rasterizer_state.cull_mode = SDL_GPU_CULLMODE_NONE;
	ci.rasterizer_state.fill_mode = SDL_GPU_FILLMODE_FILL;
	ci.target_info.color_target_descriptions = &ct;
	ci.target_info.num_color_targets = 1;
	ci.target_info.depth_stencil_format = scene_depth_format;
	ci.target_info.has_depth_stencil_target = true;

	return SDL_CreateGPUGraphicsPipeline(gpu_device, &ci);
}

static SDL_GPUGraphicsPipeline *make_post_pipeline(SDL_GPUShader *vs, SDL_GPUShader *fs)
{
	SDL_GPUGraphicsPipelineCreateInfo ci;
	SDL_GPUColorTargetDescription ct;

	memset(&ci, 0, sizeof(ci));
	memset(&ct, 0, sizeof(ct));

	ct.format = swapchain_format;

	ci.vertex_shader = vs;
	ci.fragment_shader = fs;
	ci.primitive_type = SDL_GPU_PRIMITIVETYPE_TRIANGLELIST;
	ci.rasterizer_state.cull_mode = SDL_GPU_CULLMODE_NONE;
	ci.rasterizer_state.fill_mode = SDL_GPU_FILLMODE_FILL;
	ci.target_info.color_target_descriptions = &ct;
	ci.target_info.num_color_targets = 1;

	return SDL_CreateGPUGraphicsPipeline(gpu_device, &ci);
}

enum { SCENE_BLEND_NONE, SCENE_BLEND_ADD, SCENE_BLEND_ALPHA, SCENE_BLEND_ADDALPHA, SCENE_BLEND_INVSRCCOLOR, SCENE_BLEND_MOD };

static SDL_GPUGraphicsPipeline *make_scene_pipeline(SDL_GPUShader *vs, SDL_GPUShader *fs,
	int blend_mode, int depth_write, int color_write)
{
	SDL_GPUGraphicsPipelineCreateInfo ci;
	SDL_GPUVertexBufferDescription vbd;
	SDL_GPUVertexAttribute attrs[4];
	SDL_GPUColorTargetDescription ct;

	memset(&ci, 0, sizeof(ci));
	memset(&vbd, 0, sizeof(vbd));
	memset(&attrs, 0, sizeof(attrs));
	memset(&ct, 0, sizeof(ct));

	vbd.slot = 0;
	vbd.pitch = sizeof(scene_vert_t);
	vbd.input_rate = SDL_GPU_VERTEXINPUTRATE_VERTEX;

	attrs[0].location = 0;
	attrs[0].format = SDL_GPU_VERTEXELEMENTFORMAT_FLOAT3;
	attrs[0].offset = 0;
	attrs[1].location = 1;
	attrs[1].format = SDL_GPU_VERTEXELEMENTFORMAT_FLOAT2;
	attrs[1].offset = 12;
	attrs[2].location = 2;
	attrs[2].format = SDL_GPU_VERTEXELEMENTFORMAT_FLOAT2;
	attrs[2].offset = 20;
	attrs[3].location = 3;
	attrs[3].format = SDL_GPU_VERTEXELEMENTFORMAT_UBYTE4_NORM;
	attrs[3].offset = 28;

	ct.format = SDL_GPU_TEXTUREFORMAT_R8G8B8A8_UNORM;
	if (blend_mode == SCENE_BLEND_ADD)
	{
		ct.blend_state.enable_blend = true;
		ct.blend_state.color_blend_op = SDL_GPU_BLENDOP_ADD;
		ct.blend_state.alpha_blend_op = SDL_GPU_BLENDOP_ADD;
		ct.blend_state.src_color_blendfactor = SDL_GPU_BLENDFACTOR_ONE;
		ct.blend_state.dst_color_blendfactor = SDL_GPU_BLENDFACTOR_ONE;
		ct.blend_state.src_alpha_blendfactor = SDL_GPU_BLENDFACTOR_ONE;
		ct.blend_state.dst_alpha_blendfactor = SDL_GPU_BLENDFACTOR_ONE;
	}
	else if (blend_mode == SCENE_BLEND_ALPHA)
	{
		ct.blend_state.enable_blend = true;
		ct.blend_state.color_blend_op = SDL_GPU_BLENDOP_ADD;
		ct.blend_state.alpha_blend_op = SDL_GPU_BLENDOP_ADD;
		ct.blend_state.src_color_blendfactor = SDL_GPU_BLENDFACTOR_SRC_ALPHA;
		ct.blend_state.dst_color_blendfactor = SDL_GPU_BLENDFACTOR_ONE_MINUS_SRC_ALPHA;
		ct.blend_state.src_alpha_blendfactor = SDL_GPU_BLENDFACTOR_ONE;
		ct.blend_state.dst_alpha_blendfactor = SDL_GPU_BLENDFACTOR_ONE_MINUS_SRC_ALPHA;
	}
	else if (blend_mode == SCENE_BLEND_ADDALPHA)
	{
		ct.blend_state.enable_blend = true;
		ct.blend_state.color_blend_op = SDL_GPU_BLENDOP_ADD;
		ct.blend_state.alpha_blend_op = SDL_GPU_BLENDOP_ADD;
		ct.blend_state.src_color_blendfactor = SDL_GPU_BLENDFACTOR_SRC_ALPHA;
		ct.blend_state.dst_color_blendfactor = SDL_GPU_BLENDFACTOR_ONE;
		ct.blend_state.src_alpha_blendfactor = SDL_GPU_BLENDFACTOR_SRC_ALPHA;
		ct.blend_state.dst_alpha_blendfactor = SDL_GPU_BLENDFACTOR_ONE;
	}
	else if (blend_mode == SCENE_BLEND_INVSRCCOLOR)
	{
		ct.blend_state.enable_blend = true;
		ct.blend_state.color_blend_op = SDL_GPU_BLENDOP_ADD;
		ct.blend_state.alpha_blend_op = SDL_GPU_BLENDOP_ADD;
		ct.blend_state.src_color_blendfactor = SDL_GPU_BLENDFACTOR_ZERO;
		ct.blend_state.dst_color_blendfactor = SDL_GPU_BLENDFACTOR_ONE_MINUS_SRC_COLOR;
		ct.blend_state.src_alpha_blendfactor = SDL_GPU_BLENDFACTOR_ZERO;
		ct.blend_state.dst_alpha_blendfactor = SDL_GPU_BLENDFACTOR_ONE;
	}
	else if (blend_mode == SCENE_BLEND_MOD)
	{
		// dst*src*2, caustics and detail decals
		ct.blend_state.enable_blend = true;
		ct.blend_state.color_blend_op = SDL_GPU_BLENDOP_ADD;
		ct.blend_state.alpha_blend_op = SDL_GPU_BLENDOP_ADD;
		ct.blend_state.src_color_blendfactor = SDL_GPU_BLENDFACTOR_DST_COLOR;
		ct.blend_state.dst_color_blendfactor = SDL_GPU_BLENDFACTOR_SRC_COLOR;
		ct.blend_state.src_alpha_blendfactor = SDL_GPU_BLENDFACTOR_ZERO;
		ct.blend_state.dst_alpha_blendfactor = SDL_GPU_BLENDFACTOR_ONE;
	}
	if (!color_write)
		ct.blend_state.enable_color_write_mask = true;	// mask defaults to 0

	ci.vertex_shader = vs;
	ci.fragment_shader = fs;
	ci.vertex_input_state.vertex_buffer_descriptions = &vbd;
	ci.vertex_input_state.num_vertex_buffers = 1;
	ci.vertex_input_state.vertex_attributes = attrs;
	ci.vertex_input_state.num_vertex_attributes = 4;
	ci.primitive_type = SDL_GPU_PRIMITIVETYPE_TRIANGLELIST;
	ci.rasterizer_state.cull_mode = SDL_GPU_CULLMODE_FRONT;
	ci.rasterizer_state.front_face = SDL_GPU_FRONTFACE_COUNTER_CLOCKWISE;
	ci.rasterizer_state.fill_mode = SDL_GPU_FILLMODE_FILL;
	ci.depth_stencil_state.enable_depth_test = true;
	ci.depth_stencil_state.enable_depth_write = depth_write != 0;
	ci.depth_stencil_state.compare_op = SDL_GPU_COMPAREOP_LESS_OR_EQUAL;
	ci.target_info.color_target_descriptions = &ct;
	ci.target_info.num_color_targets = 1;
	ci.target_info.depth_stencil_format = scene_depth_format;
	ci.target_info.has_depth_stencil_target = true;

	return SDL_CreateGPUGraphicsPipeline(gpu_device, &ci);
}

static SDL_GPUSampler *make_sampler(SDL_GPUFilter filter, SDL_GPUSamplerAddressMode address)
{
	SDL_GPUSamplerCreateInfo ci;

	memset(&ci, 0, sizeof(ci));
	ci.min_filter = filter;
	ci.mag_filter = filter;
	ci.mipmap_mode = (filter == SDL_GPU_FILTER_LINEAR) ?
		SDL_GPU_SAMPLERMIPMAPMODE_LINEAR : SDL_GPU_SAMPLERMIPMAPMODE_NEAREST;
	ci.address_mode_u = address;
	ci.address_mode_v = address;
	ci.address_mode_w = address;
	ci.max_lod = 1000.0f;

	return SDL_CreateGPUSampler(gpu_device, &ci);
}

static int create_pipelines(void)
{
	SDL_GPUShader *ui_vs, *ui_fs, *ui_at_fs, *post_vs, *post_fs;
	SDL_GPUShader *world_vs, *world_fs, *world_at_fs, *scene_fs, *scene_at_fs;
	SDL_GPUShader *sky_vs, *sky_fs, *water_fs, *alias_fb_fs, *mod_fs;
	int i, ok;

	ui_vs = load_shader(SDL_GPU_SHADERSTAGE_VERTEX, SHADER_ARGS(ui_vert), 0, 1);
	ui_fs = load_shader(SDL_GPU_SHADERSTAGE_FRAGMENT, SHADER_ARGS(ui_frag), 1, 0);
	ui_at_fs = load_shader(SDL_GPU_SHADERSTAGE_FRAGMENT, SHADER_ARGS(ui_alphatest_frag), 1, 0);
	post_vs = load_shader(SDL_GPU_SHADERSTAGE_VERTEX, SHADER_ARGS(post_vert), 0, 0);
	post_fs = load_shader(SDL_GPU_SHADERSTAGE_FRAGMENT, SHADER_ARGS(post_frag), 1, 1);
	world_vs = load_shader(SDL_GPU_SHADERSTAGE_VERTEX, SHADER_ARGS(world_vert), 0, 1);
	world_fs = load_shader(SDL_GPU_SHADERSTAGE_FRAGMENT, SHADER_ARGS(world_frag), 2, 0);
	world_at_fs = load_shader(SDL_GPU_SHADERSTAGE_FRAGMENT, SHADER_ARGS(world_alphatest_frag), 2, 0);
	scene_fs = load_shader(SDL_GPU_SHADERSTAGE_FRAGMENT, SHADER_ARGS(scene_frag), 1, 0);
	scene_at_fs = load_shader(SDL_GPU_SHADERSTAGE_FRAGMENT, SHADER_ARGS(scene_alphatest_frag), 1, 0);
	sky_vs = load_shader(SDL_GPU_SHADERSTAGE_VERTEX, SHADER_ARGS(sky_vert), 0, 1);
	sky_fs = load_shader(SDL_GPU_SHADERSTAGE_FRAGMENT, SHADER_ARGS(sky_frag), 2, 0);
	water_fs = load_shader(SDL_GPU_SHADERSTAGE_FRAGMENT, SHADER_ARGS(water_frag), 1, 1);
	alias_fb_fs = load_shader(SDL_GPU_SHADERSTAGE_FRAGMENT, SHADER_ARGS(alias_fb_frag), 2, 0);
	mod_fs = load_shader(SDL_GPU_SHADERSTAGE_FRAGMENT, SHADER_ARGS(mod_frag), 1, 0);

	if (!ui_vs || !ui_fs || !ui_at_fs || !post_vs || !post_fs
		|| !world_vs || !world_fs || !world_at_fs || !scene_fs || !scene_at_fs
		|| !sky_vs || !sky_fs || !water_fs || !alias_fb_fs || !mod_fs)
	{
		Com_Printf("GPU: shader creation failed: %s\n", SDL_GetError());
		return 0;
	}

	pipe_ui = make_ui_pipeline(ui_vs, ui_fs);
	pipe_ui_alphatest = make_ui_pipeline(ui_vs, ui_at_fs);
	pipe_post = make_post_pipeline(post_vs, post_fs);

	pipe_scene[SCENE_PIPE_WORLD] = make_scene_pipeline(world_vs, world_fs, 0, 1, 1);
	pipe_scene[SCENE_PIPE_WORLD_ALPHATEST] = make_scene_pipeline(world_vs, world_at_fs, 0, 1, 1);
	pipe_scene[SCENE_PIPE_TEX] = make_scene_pipeline(world_vs, scene_fs, 0, 1, 1);
	pipe_scene[SCENE_PIPE_TEX_ALPHATEST_NODEPTHWRITE] = make_scene_pipeline(world_vs, scene_at_fs, 0, 0, 1);
	pipe_scene[SCENE_PIPE_ADD_NODEPTHWRITE] = make_scene_pipeline(world_vs, scene_fs, SCENE_BLEND_ADD, 0, 1);
	pipe_scene[SCENE_PIPE_SKY] = make_scene_pipeline(sky_vs, sky_fs, 0, 1, 1);
	pipe_scene[SCENE_PIPE_DEPTHFILL] = make_scene_pipeline(world_vs, scene_fs, 0, 1, 0);
	pipe_scene[SCENE_PIPE_WATER] = make_scene_pipeline(world_vs, water_fs, 0, 1, 1);
	pipe_scene[SCENE_PIPE_TEX_BLEND] = make_scene_pipeline(world_vs, scene_fs, SCENE_BLEND_ALPHA, 1, 1);
	pipe_scene[SCENE_PIPE_TEX_ALPHATEST] = make_scene_pipeline(world_vs, scene_at_fs, 0, 1, 1);
	pipe_scene[SCENE_PIPE_ALIAS_FB] = make_scene_pipeline(world_vs, alias_fb_fs, SCENE_BLEND_ALPHA, 1, 1);
	pipe_scene[SCENE_PIPE_TEX_BLEND_NODEPTHWRITE] = make_scene_pipeline(world_vs, scene_fs, SCENE_BLEND_ALPHA, 0, 1);
	pipe_scene[SCENE_PIPE_ADDALPHA_NODEPTHWRITE] = make_scene_pipeline(world_vs, scene_fs, SCENE_BLEND_ADDALPHA, 0, 1);
	pipe_scene[SCENE_PIPE_INVSRCCOLOR_NODEPTHWRITE] = make_scene_pipeline(world_vs, scene_fs, SCENE_BLEND_INVSRCCOLOR, 0, 1);
	pipe_scene[SCENE_PIPE_MOD_NODEPTHWRITE] = make_scene_pipeline(world_vs, mod_fs, SCENE_BLEND_MOD, 0, 1);

	SDL_ReleaseGPUShader(gpu_device, ui_vs);
	SDL_ReleaseGPUShader(gpu_device, ui_fs);
	SDL_ReleaseGPUShader(gpu_device, ui_at_fs);
	SDL_ReleaseGPUShader(gpu_device, post_vs);
	SDL_ReleaseGPUShader(gpu_device, post_fs);
	SDL_ReleaseGPUShader(gpu_device, world_vs);
	SDL_ReleaseGPUShader(gpu_device, world_fs);
	SDL_ReleaseGPUShader(gpu_device, world_at_fs);
	SDL_ReleaseGPUShader(gpu_device, scene_fs);
	SDL_ReleaseGPUShader(gpu_device, scene_at_fs);
	SDL_ReleaseGPUShader(gpu_device, sky_vs);
	SDL_ReleaseGPUShader(gpu_device, sky_fs);
	SDL_ReleaseGPUShader(gpu_device, water_fs);
	SDL_ReleaseGPUShader(gpu_device, alias_fb_fs);
	SDL_ReleaseGPUShader(gpu_device, mod_fs);

	ok = pipe_ui && pipe_ui_alphatest && pipe_post;
	if (!ok)
		Com_Printf("GPU: ui/post pipeline creation failed\n");
	for (i = 0; i < SCENE_PIPE_COUNT; i++)
	{
		if (!pipe_scene[i])
		{
			Com_Printf("GPU: scene pipeline %d creation failed\n", i);
			ok = 0;
		}
	}

	if (!ok)
	{
		Com_Printf("GPU: pipeline creation failed: %s\n", SDL_GetError());
		return 0;
	}

	return 1;
}

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
	SDL_GPUBufferCreateInfo bci;
	SDL_GPUTransferBufferCreateInfo tci;

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
	swapchain_format = SDL_GetGPUSwapchainTextureFormat(gpu_device, gpu_window);

	if (SDL_GPUTextureSupportsFormat(gpu_device, SDL_GPU_TEXTUREFORMAT_D24_UNORM,
		SDL_GPU_TEXTURETYPE_2D, SDL_GPU_TEXTUREUSAGE_DEPTH_STENCIL_TARGET))
		scene_depth_format = SDL_GPU_TEXTUREFORMAT_D24_UNORM;
	else
		scene_depth_format = SDL_GPU_TEXTUREFORMAT_D32_FLOAT;

	samp_nearest = make_sampler(SDL_GPU_FILTER_NEAREST, SDL_GPU_SAMPLERADDRESSMODE_REPEAT);
	samp_linear = make_sampler(SDL_GPU_FILTER_LINEAR, SDL_GPU_SAMPLERADDRESSMODE_REPEAT);
	samp_nearest_clamp = make_sampler(SDL_GPU_FILTER_NEAREST, SDL_GPU_SAMPLERADDRESSMODE_CLAMP_TO_EDGE);
	samp_linear_clamp = make_sampler(SDL_GPU_FILTER_LINEAR, SDL_GPU_SAMPLERADDRESSMODE_CLAMP_TO_EDGE);

	memset(&bci, 0, sizeof(bci));
	bci.usage = SDL_GPU_BUFFERUSAGE_VERTEX;
	bci.size = UI_MAX_VERTS * sizeof(ui_vert_t);
	ui_vbuf = SDL_CreateGPUBuffer(gpu_device, &bci);

	memset(&tci, 0, sizeof(tci));
	tci.usage = SDL_GPU_TRANSFERBUFFERUSAGE_UPLOAD;
	tci.size = UI_MAX_VERTS * sizeof(ui_vert_t);
	ui_tbuf = SDL_CreateGPUTransferBuffer(gpu_device, &tci);

	memset(&bci, 0, sizeof(bci));
	bci.usage = SDL_GPU_BUFFERUSAGE_INDEX;
	bci.size = GPU_SCENE_MAX_INDICES * sizeof(unsigned int);
	scene_ibuf = SDL_CreateGPUBuffer(gpu_device, &bci);

	memset(&tci, 0, sizeof(tci));
	tci.usage = SDL_GPU_TRANSFERBUFFERUSAGE_UPLOAD;
	tci.size = GPU_SCENE_MAX_INDICES * sizeof(unsigned int);
	scene_itbuf = SDL_CreateGPUTransferBuffer(gpu_device, &tci);

	memset(&bci, 0, sizeof(bci));
	bci.usage = SDL_GPU_BUFFERUSAGE_VERTEX;
	bci.size = GPU_SCENE_MAX_DYNVERTS * sizeof(scene_vert_t);
	scene_dynvbuf = SDL_CreateGPUBuffer(gpu_device, &bci);

	memset(&tci, 0, sizeof(tci));
	tci.usage = SDL_GPU_TRANSFERBUFFERUSAGE_UPLOAD;
	tci.size = GPU_SCENE_MAX_DYNVERTS * sizeof(scene_vert_t);
	scene_dyntbuf = SDL_CreateGPUTransferBuffer(gpu_device, &tci);

	if (!scene_ibuf || !scene_itbuf || !scene_dynvbuf || !scene_dyntbuf)
	{
		Com_Printf("GPU: scene buffer creation failed: %s\n", SDL_GetError());
		GPU_Shutdown();
		return 0;
	}

	if (!samp_nearest || !samp_linear || !samp_nearest_clamp || !samp_linear_clamp || !ui_vbuf || !ui_tbuf)
	{
		Com_Printf("GPU: resource creation failed: %s\n", SDL_GetError());
		GPU_Shutdown();
		return 0;
	}

	if (!create_pipelines())
	{
		GPU_Shutdown();
		return 0;
	}

	GPU_Texture_InitTable();
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

	GPU_Texture_ShutdownTable();

	if (pipe_ui)
		SDL_ReleaseGPUGraphicsPipeline(gpu_device, pipe_ui);
	if (pipe_ui_alphatest)
		SDL_ReleaseGPUGraphicsPipeline(gpu_device, pipe_ui_alphatest);
	if (pipe_post)
		SDL_ReleaseGPUGraphicsPipeline(gpu_device, pipe_post);
	pipe_ui = pipe_ui_alphatest = pipe_post = NULL;

	{
		int i;
		for (i = 0; i < SCENE_PIPE_COUNT; i++)
		{
			if (pipe_scene[i])
				SDL_ReleaseGPUGraphicsPipeline(gpu_device, pipe_scene[i]);
			pipe_scene[i] = NULL;
		}
	}

	if (scene_ibuf)
		SDL_ReleaseGPUBuffer(gpu_device, scene_ibuf);
	if (scene_itbuf)
		SDL_ReleaseGPUTransferBuffer(gpu_device, scene_itbuf);
	scene_ibuf = NULL;
	scene_itbuf = NULL;
	if (scene_dynvbuf)
		SDL_ReleaseGPUBuffer(gpu_device, scene_dynvbuf);
	if (scene_dyntbuf)
		SDL_ReleaseGPUTransferBuffer(gpu_device, scene_dyntbuf);
	scene_dynvbuf = NULL;
	scene_dyntbuf = NULL;
	scene_uploader = NULL;

	if (samp_nearest)
		SDL_ReleaseGPUSampler(gpu_device, samp_nearest);
	if (samp_linear)
		SDL_ReleaseGPUSampler(gpu_device, samp_linear);
	if (samp_nearest_clamp)
		SDL_ReleaseGPUSampler(gpu_device, samp_nearest_clamp);
	if (samp_linear_clamp)
		SDL_ReleaseGPUSampler(gpu_device, samp_linear_clamp);
	samp_nearest = samp_linear = samp_nearest_clamp = samp_linear_clamp = NULL;

	if (ui_vbuf)
		SDL_ReleaseGPUBuffer(gpu_device, ui_vbuf);
	if (ui_tbuf)
		SDL_ReleaseGPUTransferBuffer(gpu_device, ui_tbuf);
	ui_vbuf = NULL;
	ui_tbuf = NULL;

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

	Draw2D_FrameReset();
	Scene_FrameReset();
}

void GPU_SetPostParams(float gamma, float contrast, const float blend[4])
{
	post_gamma = gamma;
	post_contrast = contrast;
	if (blend)
		memcpy(post_blend, blend, sizeof(post_blend));
}

static SDL_GPUSampler *sampler_for_prefs(int prefs)
{
	if (prefs & GPU_TEXPREF_LINEAR)
		return (prefs & GPU_TEXPREF_CLAMP) ? samp_linear_clamp : samp_linear;
	return (prefs & GPU_TEXPREF_CLAMP) ? samp_nearest_clamp : samp_nearest;
}

// ---- 3D scene recording ----

SDL_GPUBuffer *GPU_CreateStaticVertexBuffer(const scene_vert_t *verts, unsigned int count)
{
	SDL_GPUBufferCreateInfo bci;
	SDL_GPUTransferBufferCreateInfo tci;
	SDL_GPUBuffer *buf;
	SDL_GPUTransferBuffer *tbuf;
	SDL_GPUCommandBuffer *cmdbuf;
	SDL_GPUCopyPass *copy;
	SDL_GPUTransferBufferLocation src;
	SDL_GPUBufferRegion dst;
	void *mapped;

	if (!gpu_device || !verts || !count)
		return NULL;

	memset(&bci, 0, sizeof(bci));
	bci.usage = SDL_GPU_BUFFERUSAGE_VERTEX;
	bci.size = count * sizeof(scene_vert_t);
	buf = SDL_CreateGPUBuffer(gpu_device, &bci);
	if (!buf)
		return NULL;

	memset(&tci, 0, sizeof(tci));
	tci.usage = SDL_GPU_TRANSFERBUFFERUSAGE_UPLOAD;
	tci.size = bci.size;
	tbuf = SDL_CreateGPUTransferBuffer(gpu_device, &tci);
	if (!tbuf)
	{
		SDL_ReleaseGPUBuffer(gpu_device, buf);
		return NULL;
	}

	mapped = SDL_MapGPUTransferBuffer(gpu_device, tbuf, false);
	if (mapped)
	{
		memcpy(mapped, verts, bci.size);
		SDL_UnmapGPUTransferBuffer(gpu_device, tbuf);
	}

	cmdbuf = SDL_AcquireGPUCommandBuffer(gpu_device);
	if (cmdbuf)
	{
		copy = SDL_BeginGPUCopyPass(cmdbuf);
		memset(&src, 0, sizeof(src));
		src.transfer_buffer = tbuf;
		memset(&dst, 0, sizeof(dst));
		dst.buffer = buf;
		dst.size = bci.size;
		SDL_UploadToGPUBuffer(copy, &src, &dst, false);
		SDL_EndGPUCopyPass(copy);
		SDL_SubmitGPUCommandBuffer(cmdbuf);
	}

	SDL_ReleaseGPUTransferBuffer(gpu_device, tbuf);
	return buf;
}

void GPU_ReleaseBuffer(SDL_GPUBuffer *buf)
{
	if (gpu_device && buf)
	{
		SDL_WaitForGPUIdle(gpu_device);
		SDL_ReleaseGPUBuffer(gpu_device, buf);
	}
}

SDL_GPUTexture *GPU_CreateDynamicTexture(unsigned int width, unsigned int height)
{
	SDL_GPUTextureCreateInfo ci;

	if (!gpu_device)
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

	return SDL_CreateGPUTexture(gpu_device, &ci);
}

void GPU_ReleaseTexture(SDL_GPUTexture *tex)
{
	if (gpu_device && tex)
	{
		SDL_WaitForGPUIdle(gpu_device);
		SDL_ReleaseGPUTexture(gpu_device, tex);
	}
}

void Scene_FrameReset(void)
{
	scene_numindices = 0;
	scene_numbatches = 0;
	scene_numdynverts = 0;
	scene_has_viewport = 0;
	autoid_valid = 0;
}

unsigned int *Scene_AllocIndices(unsigned int count, unsigned int *firstindex)
{
	if (scene_numindices + count > GPU_SCENE_MAX_INDICES)
		return NULL;
	*firstindex = scene_numindices;
	scene_numindices += count;
	return &scene_indices[*firstindex];
}

scene_vert_t *Scene_AllocVerts(unsigned int count, unsigned int *firstvert)
{
	if (scene_numdynverts + count > GPU_SCENE_MAX_DYNVERTS)
		return NULL;
	*firstvert = scene_numdynverts;
	scene_numdynverts += count;
	return &scene_dynverts[*firstvert];
}

SDL_GPUBuffer *GPU_GetDynamicSceneVB(void)
{
	return scene_dynvbuf;
}

scene_batch_t *Scene_AddBatch(int pipe, int texnum, SDL_GPUTexture *tex2, SDL_GPUBuffer *vbuf,
	unsigned int firstindex, unsigned int numindices, const float *mvp)
{
	scene_batch_t *b;

	if (scene_numbatches >= GPU_SCENE_MAX_BATCHES || !numindices)
		return NULL;

	b = &scene_batches[scene_numbatches++];
	memset(b, 0, sizeof(*b));
	b->pipe = pipe;
	b->texnum = texnum;
	b->tex2 = tex2;
	b->vbuf = vbuf;
	b->firstindex = firstindex;
	b->numindices = numindices;
	b->depth_min = 0.0f;
	b->depth_max = 1.0f;
	memcpy(b->mvp, mvp, sizeof(b->mvp));
	return b;
}

void Scene_SetViewport(float x, float y, float w, float h)
{
	scene_viewport[0] = x;
	scene_viewport[1] = y;
	scene_viewport[2] = w;
	scene_viewport[3] = h;
	scene_has_viewport = 1;
}

void GPU_SetSceneUploader(void (*fn)(SDL_GPUCopyPass *copy))
{
	scene_uploader = fn;
}

void GPU_SetSceneMatrices(const float *modelview, const float *projection, const int *viewport)
{
	memcpy(autoid_modelview, modelview, sizeof(autoid_modelview));
	memcpy(autoid_projection, projection, sizeof(autoid_projection));
	memcpy(autoid_viewport, viewport, sizeof(autoid_viewport));
	autoid_valid = 1;
}

// scene render pass: clear + all recorded 2D batches
static void record_scene_pass(void)
{
	SDL_GPUColorTargetInfo ct;
	SDL_GPUDepthStencilTargetInfo ds;
	SDL_GPURenderPass *pass;
	const ui_vert_t *verts;
	const ui_batch_t *batches;
	unsigned int numverts, numbatches, i;

	verts = Draw2D_GetVerts(&numverts);
	batches = Draw2D_GetBatches(&numbatches);

	if (numverts || scene_numindices || scene_uploader)
	{
		SDL_GPUCopyPass *copy;
		SDL_GPUTransferBufferLocation src;
		SDL_GPUBufferRegion dst;
		void *mapped;

		if (numverts)
		{
			mapped = SDL_MapGPUTransferBuffer(gpu_device, ui_tbuf, true);
			if (mapped)
			{
				memcpy(mapped, verts, numverts * sizeof(ui_vert_t));
				SDL_UnmapGPUTransferBuffer(gpu_device, ui_tbuf);
			}
		}

		if (scene_numindices)
		{
			mapped = SDL_MapGPUTransferBuffer(gpu_device, scene_itbuf, true);
			if (mapped)
			{
				memcpy(mapped, scene_indices, scene_numindices * sizeof(unsigned int));
				SDL_UnmapGPUTransferBuffer(gpu_device, scene_itbuf);
			}
		}

		if (scene_numdynverts)
		{
			mapped = SDL_MapGPUTransferBuffer(gpu_device, scene_dyntbuf, true);
			if (mapped)
			{
				memcpy(mapped, scene_dynverts, scene_numdynverts * sizeof(scene_vert_t));
				SDL_UnmapGPUTransferBuffer(gpu_device, scene_dyntbuf);
			}
		}

		copy = SDL_BeginGPUCopyPass(frame_cmdbuf);

		if (numverts)
		{
			memset(&src, 0, sizeof(src));
			src.transfer_buffer = ui_tbuf;
			memset(&dst, 0, sizeof(dst));
			dst.buffer = ui_vbuf;
			dst.size = numverts * sizeof(ui_vert_t);
			SDL_UploadToGPUBuffer(copy, &src, &dst, true);
		}

		if (scene_numindices)
		{
			memset(&src, 0, sizeof(src));
			src.transfer_buffer = scene_itbuf;
			memset(&dst, 0, sizeof(dst));
			dst.buffer = scene_ibuf;
			dst.size = scene_numindices * sizeof(unsigned int);
			SDL_UploadToGPUBuffer(copy, &src, &dst, true);
		}

		if (scene_numdynverts)
		{
			memset(&src, 0, sizeof(src));
			src.transfer_buffer = scene_dyntbuf;
			memset(&dst, 0, sizeof(dst));
			dst.buffer = scene_dynvbuf;
			dst.size = scene_numdynverts * sizeof(scene_vert_t);
			SDL_UploadToGPUBuffer(copy, &src, &dst, true);
		}

		if (scene_uploader)
			scene_uploader(copy);

		SDL_EndGPUCopyPass(copy);
	}

	memset(&ct, 0, sizeof(ct));
	ct.texture = scene_color;
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
	if (!pass)
		return;

	if (scene_numbatches)
	{
		SDL_GPUBufferBinding ib;
		SDL_GPUBuffer *bound_vbuf = NULL;
		float cur_depth_min = 0.0f, cur_depth_max = 1.0f;
		unsigned int i;

		if (scene_has_viewport)
		{
			SDL_GPUViewport vp;
			vp.x = scene_viewport[0];
			vp.y = scene_viewport[1];
			vp.w = scene_viewport[2];
			vp.h = scene_viewport[3];
			vp.min_depth = 0.0f;
			vp.max_depth = 1.0f;
			SDL_SetGPUViewport(pass, &vp);
		}

		memset(&ib, 0, sizeof(ib));
		ib.buffer = scene_ibuf;
		SDL_BindGPUIndexBuffer(pass, &ib, SDL_GPU_INDEXELEMENTSIZE_32BIT);

		for (i = 0; i < scene_numbatches; i++)
		{
			scene_batch_t *b = &scene_batches[i];
			SDL_GPUTextureSamplerBinding tsb[2];
			SDL_GPUTexture *tex;
			int prefs = 0;
			int numtex;

			tex = GPU_Texture_Lookup(b->texnum, &prefs);
			if (!tex || !b->vbuf)
				continue;

			if (b->depth_min != cur_depth_min || b->depth_max != cur_depth_max)
			{
				SDL_GPUViewport vp;
				vp.x = scene_has_viewport ? scene_viewport[0] : 0.0f;
				vp.y = scene_has_viewport ? scene_viewport[1] : 0.0f;
				vp.w = scene_has_viewport ? scene_viewport[2] : (float)scene_width;
				vp.h = scene_has_viewport ? scene_viewport[3] : (float)scene_height;
				vp.min_depth = b->depth_min;
				vp.max_depth = b->depth_max;
				SDL_SetGPUViewport(pass, &vp);
				cur_depth_min = b->depth_min;
				cur_depth_max = b->depth_max;
			}

			if (b->vbuf != bound_vbuf)
			{
				SDL_GPUBufferBinding vb;
				memset(&vb, 0, sizeof(vb));
				vb.buffer = b->vbuf;
				SDL_BindGPUVertexBuffers(pass, 0, &vb, 1);
				bound_vbuf = b->vbuf;
			}

			SDL_BindGPUGraphicsPipeline(pass, pipe_scene[b->pipe]);

			if (b->pipe == SCENE_PIPE_SKY)
			{
				struct
				{
					float mvp[16];
					float params[8];
				} ubo;
				memcpy(ubo.mvp, b->mvp, sizeof(ubo.mvp));
				memcpy(ubo.params, b->params, sizeof(ubo.params));
				SDL_PushGPUVertexUniformData(frame_cmdbuf, 0, &ubo, sizeof(ubo));
			}
			else
			{
				SDL_PushGPUVertexUniformData(frame_cmdbuf, 0, b->mvp, sizeof(b->mvp));
			}

			if (b->pipe == SCENE_PIPE_WATER)
				SDL_PushGPUFragmentUniformData(frame_cmdbuf, 0, b->params, 4 * sizeof(float));

			memset(tsb, 0, sizeof(tsb));
			tsb[0].texture = tex;
			tsb[0].sampler = sampler_for_prefs(prefs);
			numtex = 1;
			if (b->pipe == SCENE_PIPE_WORLD || b->pipe == SCENE_PIPE_WORLD_ALPHATEST
				|| b->pipe == SCENE_PIPE_SKY || b->pipe == SCENE_PIPE_ALIAS_FB)
			{
				tsb[1].texture = b->tex2 ? b->tex2 : tex;
				tsb[1].sampler = samp_linear;
				numtex = 2;
			}
			SDL_BindGPUFragmentSamplers(pass, 0, tsb, numtex);

			SDL_DrawGPUIndexedPrimitives(pass, b->numindices, 1, b->firstindex, 0, 0);
		}

		if (scene_has_viewport)
		{
			SDL_GPUViewport vp;
			vp.x = 0.0f;
			vp.y = 0.0f;
			vp.w = (float)scene_width;
			vp.h = (float)scene_height;
			vp.min_depth = 0.0f;
			vp.max_depth = 1.0f;
			SDL_SetGPUViewport(pass, &vp);
		}
	}

	if (numbatches)
	{
		SDL_GPUBufferBinding vb;

		memset(&vb, 0, sizeof(vb));
		vb.buffer = ui_vbuf;
		SDL_BindGPUVertexBuffers(pass, 0, &vb, 1);

		for (i = 0; i < numbatches; i++)
		{
			const ui_batch_t *b = &batches[i];
			SDL_GPUTextureSamplerBinding tsb;
			SDL_GPUTexture *tex;
			int prefs = 0;
			float ortho[16];

			tex = GPU_Texture_Lookup(b->texnum, &prefs);
			if (!tex)
				continue;

			SDL_BindGPUGraphicsPipeline(pass, b->alphatest ? pipe_ui_alphatest : pipe_ui);

			memset(ortho, 0, sizeof(ortho));
			ortho[0] = 2.0f / b->ortho_w;
			ortho[5] = -2.0f / b->ortho_h;
			ortho[10] = 1.0f;
			ortho[12] = -1.0f;
			ortho[13] = 1.0f;
			ortho[15] = 1.0f;
			SDL_PushGPUVertexUniformData(frame_cmdbuf, 0, ortho, sizeof(ortho));

			memset(&tsb, 0, sizeof(tsb));
			tsb.texture = tex;
			tsb.sampler = sampler_for_prefs(prefs);
			SDL_BindGPUFragmentSamplers(pass, 0, &tsb, 1);

			SDL_DrawGPUPrimitives(pass, b->numverts, 1, b->firstvert, 0);
		}
	}

	SDL_EndGPURenderPass(pass);
}

static void record_post_pass(SDL_GPUTexture *swap_tex, Uint32 swap_w, Uint32 swap_h)
{
	SDL_GPUColorTargetInfo ct;
	SDL_GPURenderPass *pass;
	SDL_GPUTextureSamplerBinding tsb;
	struct
	{
		float blend[4];
		float gamma;
		float contrast;
		float pad[2];
	} ubo;

	memset(&ct, 0, sizeof(ct));
	ct.texture = swap_tex;
	ct.load_op = SDL_GPU_LOADOP_DONT_CARE;
	ct.store_op = SDL_GPU_STOREOP_STORE;

	pass = SDL_BeginGPURenderPass(frame_cmdbuf, &ct, 1, NULL);
	if (!pass)
		return;

	(void)swap_w;
	(void)swap_h;

	SDL_BindGPUGraphicsPipeline(pass, pipe_post);

	memcpy(ubo.blend, post_blend, sizeof(ubo.blend));
	ubo.gamma = post_gamma;
	ubo.contrast = post_contrast;
	ubo.pad[0] = ubo.pad[1] = 0.0f;
	SDL_PushGPUFragmentUniformData(frame_cmdbuf, 0, &ubo, sizeof(ubo));

	memset(&tsb, 0, sizeof(tsb));
	tsb.texture = scene_color;
	tsb.sampler = samp_nearest_clamp;
	SDL_BindGPUFragmentSamplers(pass, 0, &tsb, 1);

	SDL_DrawGPUPrimitives(pass, 3, 1, 0, 0);

	SDL_EndGPURenderPass(pass);
}

void GPU_EndFrame(void)
{
	SDL_GPUTexture *swap_tex;
	Uint32 swap_w, swap_h;

	if (!gpu_device || !frame_cmdbuf)
		return;

	record_scene_pass();

	if (!SDL_WaitAndAcquireGPUSwapchainTexture(frame_cmdbuf, gpu_window, &swap_tex, &swap_w, &swap_h))
		swap_tex = NULL;

	if (swap_tex)
		record_post_pass(swap_tex, swap_w, swap_h);

	SDL_SubmitGPUCommandBuffer(frame_cmdbuf);
	frame_cmdbuf = NULL;

	// TEMP test hook: -autoshot [frame] screenshots at frame N (default 60), quits at N+10
	{
		static int frames, shotframe = -1;
		if (shotframe == -1)
		{
			int parm = COM_CheckParm("-autoshot");
			shotframe = 0;
			if (parm)
			{
				shotframe = 60;
				if (parm + 1 < com_argc && com_argv[parm + 1][0] != '-' && com_argv[parm + 1][0] != '+')
					shotframe = max(10, Q_atoi(com_argv[parm + 1]));
			}
		}
		if (shotframe)
		{
			frames++;
			if (frames == shotframe)
				Cbuf_AddText("screenshot autoshot\n");
			if (frames == shotframe + 10)
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
	if (!autoid_valid)
		return 0;
	memcpy(modelview, autoid_modelview, sizeof(autoid_modelview));
	memcpy(projection, autoid_projection, sizeof(autoid_projection));
	memcpy(viewport, autoid_viewport, sizeof(autoid_viewport));
	return 1;
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

	// mid-frame: record pending 2D into the open cmdbuf so the shot matches
	// what the engine drew this frame, then flush it (frame skips presentation)
	if (frame_cmdbuf)
	{
		record_scene_pass();
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

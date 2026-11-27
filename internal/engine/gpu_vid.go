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
	"runtime"
	"unsafe"

	"github.com/classicq/classicq"
	"github.com/classicq/classicq/internal/sdl"
)

// gpu_local.h

// textures (gpu_texture.c)
const (
	GPU_TEXPREF_LINEAR      = 1
	GPU_TEXPREF_CLAMP       = 2
	GPU_TEXPREF_MIP_NEAREST = 4
	GPU_TEXPREF_MIP_LINEAR  = 8
)

// 3D scene (recorded during R_RenderView, executed in GPU_EndFrame)
type scene_vert_t struct {
	pos  [3]float32
	st   [2]float32
	lm   [2]float32
	rgba [4]uint8
}

const (
	SCENE_PIPE_WORLD                      = iota // tex * lightmap, opaque
	SCENE_PIPE_WORLD_ALPHATEST                   // fence/alpha chain
	SCENE_PIPE_TEX                               // tex * color, opaque (skybox, flat fills)
	SCENE_PIPE_TEX_ALPHATEST_NODEPTHWRITE        // fullbright second pass
	SCENE_PIPE_ADD_NODEPTHWRITE                  // luma pass, blend one/one
	SCENE_PIPE_SKY                               // two scrolling layers
	SCENE_PIPE_DEPTHFILL                         // skybox z fill, no color writes
	SCENE_PIPE_WATER                             // fs warp
	SCENE_PIPE_TEX_BLEND                         // alias models, alpha blend (opaque at a=255)
	SCENE_PIPE_TEX_ALPHATEST                     // sprites, depth write on
	SCENE_PIPE_ALIAS_FB                          // alias base + fullbright decal, 2 textures
	SCENE_PIPE_TEX_BLEND_NODEPTHWRITE            // QMB lavasplash/blood3/bubble
	SCENE_PIPE_ADDALPHA_NODEPTHWRITE             // QMB sparks/fire/smoke, srcalpha/one
	SCENE_PIPE_INVSRCCOLOR_NODEPTHWRITE          // QMB blood1/blood2, zero/1-srccolor
	SCENE_PIPE_MOD_NODEPTHWRITE                  // caustics/detail decal, dstcolor/srccolor
	SCENE_PIPE_COUNT
)

const SCENE_MAX_PARAMS = 8

type scene_batch_t struct {
	pipe       int
	texnum     int             // main texture via texture table
	tex2       *sdl.GPUTexture // lightmap page or sky alpha layer, owned by caller
	vbuf       *sdl.GPUBuffer  // static vertex buffer the indices refer to
	firstindex uint32
	numindices uint32
	depth_min  float32 // viewport depth range, viewmodel squashes to 0..0.3
	depth_max  float32
	mvp        [16]float32
	params     [SCENE_MAX_PARAMS]float32 // sky: origin xyz + speed1, speed2; water: cltime
}

const (
	GPU_SCENE_MAX_INDICES  = 1 << 20
	GPU_SCENE_MAX_BATCHES  = 8192
	GPU_SCENE_MAX_DYNVERTS = 65536
)

// 2D batcher (gpu_draw2d.c)
const GPU_UI_MAX_VERTS = 65536

type ui_batch_t struct {
	texnum    int
	alphatest int
	firstvert uint32
	numverts  uint32
	ortho_w   float32
	ortho_h   float32
}

type ui_vert_t struct {
	x, y float32
	u, v float32
	rgba [4]uint8
}

// gpu_vid.c

const UI_MAX_VERTS = GPU_UI_MAX_VERTS

var gpu_device *sdl.GPUDevice
var gpu_window *sdl.Window

var scene_color *sdl.GPUTexture
var scene_depth *sdl.GPUTexture
var crt_tex [2]*sdl.GPUTexture
var scene_depth_format sdl.GPUTextureFormat
var swapchain_format sdl.GPUTextureFormat
var scene_width, scene_height uint32

var frame_cmdbuf *sdl.GPUCommandBuffer

var pipe_ui *sdl.GPUGraphicsPipeline
var pipe_ui_alphatest *sdl.GPUGraphicsPipeline
var pipe_post *sdl.GPUGraphicsPipeline
var pipe_crt_bloom *sdl.GPUGraphicsPipeline
var pipe_crt_phosphor *sdl.GPUGraphicsPipeline
var pipe_scene [SCENE_PIPE_COUNT]*sdl.GPUGraphicsPipeline

var scene_ibuf *sdl.GPUBuffer
var scene_itbuf *sdl.GPUTransferBuffer
var scene_indices [GPU_SCENE_MAX_INDICES]uint32
var scene_numindices uint32
var scene_dynvbuf *sdl.GPUBuffer
var scene_dyntbuf *sdl.GPUTransferBuffer
var scene_dynverts [GPU_SCENE_MAX_DYNVERTS]scene_vert_t
var scene_numdynverts uint32
var scene_batches [GPU_SCENE_MAX_BATCHES]scene_batch_t
var scene_numbatches uint32
var scene_viewport [4]float32
var scene_has_viewport int

var scene_uploader func(copy *sdl.GPUCopyPass)

var autoid_modelview [16]float32
var autoid_projection [16]float32
var autoid_viewport [4]int
var autoid_valid int

var samp_nearest *sdl.GPUSampler
var samp_linear *sdl.GPUSampler
var samp_nearest_clamp *sdl.GPUSampler
var samp_linear_clamp *sdl.GPUSampler
var samp_tex [12]*sdl.GPUSampler

var ui_vbuf *sdl.GPUBuffer
var ui_tbuf *sdl.GPUTransferBuffer

var readback_buffer *sdl.GPUTransferBuffer
var readback_size uint32

var post_gamma float32 = 1.0
var post_contrast float32 = 1.0
var post_blend [4]float32
var crt_comp_gamma float32 = 1.0

var r_crt_phosphor = cvar_t{name: "r_crt_phosphor", string: "0"}
var r_crt_dotbloom = cvar_t{name: "r_crt_dotbloom", string: "0"}
var vid_framesinflight = cvar_t{name: "vid_framesinflight", string: "1", flags: CVAR_ARCHIVE}

const (
	sizeof_ui_vert_t    = uint32(unsafe.Sizeof(ui_vert_t{}))
	sizeof_scene_vert_t = uint32(unsafe.Sizeof(scene_vert_t{}))
)

// The blobs come from the embedded src/shaders/compiled; name is the file
// base, like "ui.vert".
func load_shader(stage sdl.GPUShaderStage, name string, num_samplers, num_uniform_buffers uint32) *sdl.GPUShader {
	var ci sdl.GPUShaderCreateInfo
	formats := sdl.GetGPUShaderFormats(gpu_device)

	var ext, entry string
	if formats&sdl.GPU_SHADERFORMAT_SPIRV != 0 {
		ci.Format = sdl.GPU_SHADERFORMAT_SPIRV
		ext, entry = "spv", "main"
	} else if formats&sdl.GPU_SHADERFORMAT_DXIL != 0 {
		ci.Format = sdl.GPU_SHADERFORMAT_DXIL
		ext, entry = "dxil", "main"
	} else if formats&sdl.GPU_SHADERFORMAT_MSL != 0 {
		ci.Format = sdl.GPU_SHADERFORMAT_MSL
		ext, entry = "msl", "main0"
	} else {
		Com_Printf("GPU: no supported shader format\n")
		return nil
	}

	code, err := classicq.Shaders.ReadFile("src/shaders/compiled/" + name + "." + ext)
	if err != nil || len(code) == 0 {
		Com_Printf("GPU: shader %s.%s missing from the build\n", name, ext)
		return nil
	}
	entrypoint := append([]byte(entry), 0)

	ci.Code = &code[0]
	ci.CodeSize = uintptr(len(code))
	ci.Entrypoint = &entrypoint[0]
	ci.Stage = stage
	ci.NumSamplers = num_samplers
	ci.NumUniformBuffers = num_uniform_buffers

	shader := sdl.CreateGPUShader(gpu_device, &ci)
	runtime.KeepAlive(code)
	runtime.KeepAlive(entrypoint)
	return shader
}

func make_ui_pipeline(vs, fs *sdl.GPUShader) *sdl.GPUGraphicsPipeline {
	var ci sdl.GPUGraphicsPipelineCreateInfo
	var vbd sdl.GPUVertexBufferDescription
	var attrs [3]sdl.GPUVertexAttribute
	var ct sdl.GPUColorTargetDescription

	vbd.Slot = 0
	vbd.Pitch = sizeof_ui_vert_t
	vbd.InputRate = sdl.GPU_VERTEXINPUTRATE_VERTEX

	attrs[0].Location = 0
	attrs[0].Format = sdl.GPU_VERTEXELEMENTFORMAT_FLOAT2
	attrs[0].Offset = 0
	attrs[1].Location = 1
	attrs[1].Format = sdl.GPU_VERTEXELEMENTFORMAT_FLOAT2
	attrs[1].Offset = 8
	attrs[2].Location = 2
	attrs[2].Format = sdl.GPU_VERTEXELEMENTFORMAT_UBYTE4_NORM
	attrs[2].Offset = 16

	ct.Format = sdl.GPU_TEXTUREFORMAT_R8G8B8A8_UNORM
	ct.BlendState.EnableBlend = true
	ct.BlendState.ColorBlendOp = sdl.GPU_BLENDOP_ADD
	ct.BlendState.AlphaBlendOp = sdl.GPU_BLENDOP_ADD
	ct.BlendState.SrcColorBlendfactor = sdl.GPU_BLENDFACTOR_SRC_ALPHA
	ct.BlendState.DstColorBlendfactor = sdl.GPU_BLENDFACTOR_ONE_MINUS_SRC_ALPHA
	ct.BlendState.SrcAlphaBlendfactor = sdl.GPU_BLENDFACTOR_ONE
	ct.BlendState.DstAlphaBlendfactor = sdl.GPU_BLENDFACTOR_ONE_MINUS_SRC_ALPHA

	ci.VertexShader = vs
	ci.FragmentShader = fs
	ci.VertexInputState.VertexBufferDescriptions = &vbd
	ci.VertexInputState.NumVertexBuffers = 1
	ci.VertexInputState.VertexAttributes = &attrs[0]
	ci.VertexInputState.NumVertexAttributes = 3
	ci.PrimitiveType = sdl.GPU_PRIMITIVETYPE_TRIANGLELIST
	ci.RasterizerState.CullMode = sdl.GPU_CULLMODE_NONE
	ci.RasterizerState.FillMode = sdl.GPU_FILLMODE_FILL
	ci.TargetInfo.ColorTargetDescriptions = &ct
	ci.TargetInfo.NumColorTargets = 1
	ci.TargetInfo.DepthStencilFormat = scene_depth_format
	ci.TargetInfo.HasDepthStencilTarget = true

	p := sdl.CreateGPUGraphicsPipeline(gpu_device, &ci)
	runtime.KeepAlive(&vbd)
	runtime.KeepAlive(&attrs)
	runtime.KeepAlive(&ct)
	return p
}

func make_post_pipeline(vs, fs *sdl.GPUShader, format sdl.GPUTextureFormat) *sdl.GPUGraphicsPipeline {
	var ci sdl.GPUGraphicsPipelineCreateInfo
	var ct sdl.GPUColorTargetDescription

	ct.Format = format

	ci.VertexShader = vs
	ci.FragmentShader = fs
	ci.PrimitiveType = sdl.GPU_PRIMITIVETYPE_TRIANGLELIST
	ci.RasterizerState.CullMode = sdl.GPU_CULLMODE_NONE
	ci.RasterizerState.FillMode = sdl.GPU_FILLMODE_FILL
	ci.TargetInfo.ColorTargetDescriptions = &ct
	ci.TargetInfo.NumColorTargets = 1

	p := sdl.CreateGPUGraphicsPipeline(gpu_device, &ci)
	runtime.KeepAlive(&ct)
	return p
}

const (
	SCENE_BLEND_NONE = iota
	SCENE_BLEND_ADD
	SCENE_BLEND_ALPHA
	SCENE_BLEND_ADDALPHA
	SCENE_BLEND_INVSRCCOLOR
	SCENE_BLEND_MOD
)

func make_scene_pipeline(vs, fs *sdl.GPUShader, blend_mode, depth_write, color_write int) *sdl.GPUGraphicsPipeline {
	var ci sdl.GPUGraphicsPipelineCreateInfo
	var vbd sdl.GPUVertexBufferDescription
	var attrs [4]sdl.GPUVertexAttribute
	var ct sdl.GPUColorTargetDescription

	vbd.Slot = 0
	vbd.Pitch = sizeof_scene_vert_t
	vbd.InputRate = sdl.GPU_VERTEXINPUTRATE_VERTEX

	attrs[0].Location = 0
	attrs[0].Format = sdl.GPU_VERTEXELEMENTFORMAT_FLOAT3
	attrs[0].Offset = 0
	attrs[1].Location = 1
	attrs[1].Format = sdl.GPU_VERTEXELEMENTFORMAT_FLOAT2
	attrs[1].Offset = 12
	attrs[2].Location = 2
	attrs[2].Format = sdl.GPU_VERTEXELEMENTFORMAT_FLOAT2
	attrs[2].Offset = 20
	attrs[3].Location = 3
	attrs[3].Format = sdl.GPU_VERTEXELEMENTFORMAT_UBYTE4_NORM
	attrs[3].Offset = 28

	ct.Format = sdl.GPU_TEXTUREFORMAT_R8G8B8A8_UNORM
	if blend_mode == SCENE_BLEND_ADD {
		ct.BlendState.EnableBlend = true
		ct.BlendState.ColorBlendOp = sdl.GPU_BLENDOP_ADD
		ct.BlendState.AlphaBlendOp = sdl.GPU_BLENDOP_ADD
		ct.BlendState.SrcColorBlendfactor = sdl.GPU_BLENDFACTOR_ONE
		ct.BlendState.DstColorBlendfactor = sdl.GPU_BLENDFACTOR_ONE
		ct.BlendState.SrcAlphaBlendfactor = sdl.GPU_BLENDFACTOR_ONE
		ct.BlendState.DstAlphaBlendfactor = sdl.GPU_BLENDFACTOR_ONE
	} else if blend_mode == SCENE_BLEND_ALPHA {
		ct.BlendState.EnableBlend = true
		ct.BlendState.ColorBlendOp = sdl.GPU_BLENDOP_ADD
		ct.BlendState.AlphaBlendOp = sdl.GPU_BLENDOP_ADD
		ct.BlendState.SrcColorBlendfactor = sdl.GPU_BLENDFACTOR_SRC_ALPHA
		ct.BlendState.DstColorBlendfactor = sdl.GPU_BLENDFACTOR_ONE_MINUS_SRC_ALPHA
		ct.BlendState.SrcAlphaBlendfactor = sdl.GPU_BLENDFACTOR_ONE
		ct.BlendState.DstAlphaBlendfactor = sdl.GPU_BLENDFACTOR_ONE_MINUS_SRC_ALPHA
	} else if blend_mode == SCENE_BLEND_ADDALPHA {
		ct.BlendState.EnableBlend = true
		ct.BlendState.ColorBlendOp = sdl.GPU_BLENDOP_ADD
		ct.BlendState.AlphaBlendOp = sdl.GPU_BLENDOP_ADD
		ct.BlendState.SrcColorBlendfactor = sdl.GPU_BLENDFACTOR_SRC_ALPHA
		ct.BlendState.DstColorBlendfactor = sdl.GPU_BLENDFACTOR_ONE
		ct.BlendState.SrcAlphaBlendfactor = sdl.GPU_BLENDFACTOR_SRC_ALPHA
		ct.BlendState.DstAlphaBlendfactor = sdl.GPU_BLENDFACTOR_ONE
	} else if blend_mode == SCENE_BLEND_INVSRCCOLOR {
		ct.BlendState.EnableBlend = true
		ct.BlendState.ColorBlendOp = sdl.GPU_BLENDOP_ADD
		ct.BlendState.AlphaBlendOp = sdl.GPU_BLENDOP_ADD
		ct.BlendState.SrcColorBlendfactor = sdl.GPU_BLENDFACTOR_ZERO
		ct.BlendState.DstColorBlendfactor = sdl.GPU_BLENDFACTOR_ONE_MINUS_SRC_COLOR
		ct.BlendState.SrcAlphaBlendfactor = sdl.GPU_BLENDFACTOR_ZERO
		ct.BlendState.DstAlphaBlendfactor = sdl.GPU_BLENDFACTOR_ONE
	} else if blend_mode == SCENE_BLEND_MOD {
		// dst*src*2, caustics and detail decals
		ct.BlendState.EnableBlend = true
		ct.BlendState.ColorBlendOp = sdl.GPU_BLENDOP_ADD
		ct.BlendState.AlphaBlendOp = sdl.GPU_BLENDOP_ADD
		ct.BlendState.SrcColorBlendfactor = sdl.GPU_BLENDFACTOR_DST_COLOR
		ct.BlendState.DstColorBlendfactor = sdl.GPU_BLENDFACTOR_SRC_COLOR
		ct.BlendState.SrcAlphaBlendfactor = sdl.GPU_BLENDFACTOR_ZERO
		ct.BlendState.DstAlphaBlendfactor = sdl.GPU_BLENDFACTOR_ONE
	}
	if color_write == 0 {
		ct.BlendState.EnableColorWriteMask = true // mask defaults to 0
	}

	ci.VertexShader = vs
	ci.FragmentShader = fs
	ci.VertexInputState.VertexBufferDescriptions = &vbd
	ci.VertexInputState.NumVertexBuffers = 1
	ci.VertexInputState.VertexAttributes = &attrs[0]
	ci.VertexInputState.NumVertexAttributes = 4
	ci.PrimitiveType = sdl.GPU_PRIMITIVETYPE_TRIANGLELIST
	ci.RasterizerState.CullMode = sdl.GPU_CULLMODE_FRONT
	ci.RasterizerState.FrontFace = sdl.GPU_FRONTFACE_COUNTER_CLOCKWISE
	ci.RasterizerState.FillMode = sdl.GPU_FILLMODE_FILL
	ci.DepthStencilState.EnableDepthTest = true
	ci.DepthStencilState.EnableDepthWrite = depth_write != 0
	ci.DepthStencilState.CompareOp = sdl.GPU_COMPAREOP_LESS_OR_EQUAL
	ci.TargetInfo.ColorTargetDescriptions = &ct
	ci.TargetInfo.NumColorTargets = 1
	ci.TargetInfo.DepthStencilFormat = scene_depth_format
	ci.TargetInfo.HasDepthStencilTarget = true

	p := sdl.CreateGPUGraphicsPipeline(gpu_device, &ci)
	runtime.KeepAlive(&vbd)
	runtime.KeepAlive(&attrs)
	runtime.KeepAlive(&ct)
	return p
}

func make_sampler(filter sdl.GPUFilter, address sdl.GPUSamplerAddressMode) *sdl.GPUSampler {
	var ci sdl.GPUSamplerCreateInfo

	ci.MinFilter = filter
	ci.MagFilter = filter
	if filter == sdl.GPU_FILTER_LINEAR {
		ci.MipmapMode = sdl.GPU_SAMPLERMIPMAPMODE_LINEAR
	} else {
		ci.MipmapMode = sdl.GPU_SAMPLERMIPMAPMODE_NEAREST
	}
	ci.AddressModeU = address
	ci.AddressModeV = address
	ci.AddressModeW = address
	ci.MaxLod = 1000.0

	return sdl.CreateGPUSampler(gpu_device, &ci)
}

// gl_texturemode semantics: no mipmap suffix locks sampling to the base level
func make_tex_sampler(prefs int) *sdl.GPUSampler {
	var ci sdl.GPUSamplerCreateInfo

	if prefs&GPU_TEXPREF_LINEAR != 0 {
		ci.MinFilter = sdl.GPU_FILTER_LINEAR
	} else {
		ci.MinFilter = sdl.GPU_FILTER_NEAREST
	}
	ci.MagFilter = ci.MinFilter
	if prefs&GPU_TEXPREF_MIP_LINEAR != 0 {
		ci.MipmapMode = sdl.GPU_SAMPLERMIPMAPMODE_LINEAR
	} else {
		ci.MipmapMode = sdl.GPU_SAMPLERMIPMAPMODE_NEAREST
	}
	if prefs&(GPU_TEXPREF_MIP_NEAREST|GPU_TEXPREF_MIP_LINEAR) != 0 {
		ci.MaxLod = 1000.0
	} else {
		ci.MaxLod = 0.0
	}
	if prefs&GPU_TEXPREF_CLAMP != 0 {
		ci.AddressModeU = sdl.GPU_SAMPLERADDRESSMODE_CLAMP_TO_EDGE
	} else {
		ci.AddressModeU = sdl.GPU_SAMPLERADDRESSMODE_REPEAT
	}
	ci.AddressModeV = ci.AddressModeU
	ci.AddressModeW = ci.AddressModeU

	return sdl.CreateGPUSampler(gpu_device, &ci)
}

func tex_sampler_index(prefs int) int {
	mip := 0
	if prefs&GPU_TEXPREF_MIP_NEAREST != 0 {
		mip = 1
	} else if prefs&GPU_TEXPREF_MIP_LINEAR != 0 {
		mip = 2
	}

	linear := 0
	if prefs&GPU_TEXPREF_LINEAR != 0 {
		linear = 6
	}
	return linear + mip*2 + b2i(prefs&GPU_TEXPREF_CLAMP != 0)
}

func create_pipelines() int {
	ui_vs := load_shader(sdl.GPU_SHADERSTAGE_VERTEX, "ui.vert", 0, 1)
	ui_fs := load_shader(sdl.GPU_SHADERSTAGE_FRAGMENT, "ui.frag", 1, 0)
	ui_at_fs := load_shader(sdl.GPU_SHADERSTAGE_FRAGMENT, "ui_alphatest.frag", 1, 0)
	post_vs := load_shader(sdl.GPU_SHADERSTAGE_VERTEX, "post.vert", 0, 0)
	post_fs := load_shader(sdl.GPU_SHADERSTAGE_FRAGMENT, "post.frag", 1, 1)
	world_vs := load_shader(sdl.GPU_SHADERSTAGE_VERTEX, "world.vert", 0, 1)
	world_fs := load_shader(sdl.GPU_SHADERSTAGE_FRAGMENT, "world.frag", 2, 0)
	world_at_fs := load_shader(sdl.GPU_SHADERSTAGE_FRAGMENT, "world_alphatest.frag", 2, 0)
	scene_fs := load_shader(sdl.GPU_SHADERSTAGE_FRAGMENT, "scene.frag", 1, 0)
	scene_at_fs := load_shader(sdl.GPU_SHADERSTAGE_FRAGMENT, "scene_alphatest.frag", 1, 0)
	sky_vs := load_shader(sdl.GPU_SHADERSTAGE_VERTEX, "sky.vert", 0, 1)
	sky_fs := load_shader(sdl.GPU_SHADERSTAGE_FRAGMENT, "sky.frag", 2, 0)
	water_fs := load_shader(sdl.GPU_SHADERSTAGE_FRAGMENT, "water.frag", 1, 1)
	alias_fb_fs := load_shader(sdl.GPU_SHADERSTAGE_FRAGMENT, "alias_fb.frag", 2, 0)
	mod_fs := load_shader(sdl.GPU_SHADERSTAGE_FRAGMENT, "mod.frag", 1, 0)
	crt_bloom_fs := load_shader(sdl.GPU_SHADERSTAGE_FRAGMENT, "crt_bloom.frag", 1, 1)
	crt_phosphor_fs := load_shader(sdl.GPU_SHADERSTAGE_FRAGMENT, "crt_phosphor.frag", 1, 1)

	if ui_vs == nil || ui_fs == nil || ui_at_fs == nil || post_vs == nil || post_fs == nil ||
		world_vs == nil || world_fs == nil || world_at_fs == nil || scene_fs == nil || scene_at_fs == nil ||
		sky_vs == nil || sky_fs == nil || water_fs == nil || alias_fb_fs == nil || mod_fs == nil ||
		crt_bloom_fs == nil || crt_phosphor_fs == nil {
		Com_Printf("GPU: shader creation failed: %s\n", sdl.GetError())
		return 0
	}

	pipe_ui = make_ui_pipeline(ui_vs, ui_fs)
	pipe_ui_alphatest = make_ui_pipeline(ui_vs, ui_at_fs)
	pipe_post = make_post_pipeline(post_vs, post_fs, swapchain_format)
	pipe_crt_bloom = make_post_pipeline(post_vs, crt_bloom_fs, sdl.GPU_TEXTUREFORMAT_R8G8B8A8_UNORM)
	pipe_crt_phosphor = make_post_pipeline(post_vs, crt_phosphor_fs, sdl.GPU_TEXTUREFORMAT_R8G8B8A8_UNORM)

	pipe_scene[SCENE_PIPE_WORLD] = make_scene_pipeline(world_vs, world_fs, 0, 1, 1)
	pipe_scene[SCENE_PIPE_WORLD_ALPHATEST] = make_scene_pipeline(world_vs, world_at_fs, 0, 1, 1)
	pipe_scene[SCENE_PIPE_TEX] = make_scene_pipeline(world_vs, scene_fs, 0, 1, 1)
	pipe_scene[SCENE_PIPE_TEX_ALPHATEST_NODEPTHWRITE] = make_scene_pipeline(world_vs, scene_at_fs, 0, 0, 1)
	pipe_scene[SCENE_PIPE_ADD_NODEPTHWRITE] = make_scene_pipeline(world_vs, scene_fs, SCENE_BLEND_ADD, 0, 1)
	pipe_scene[SCENE_PIPE_SKY] = make_scene_pipeline(sky_vs, sky_fs, 0, 1, 1)
	pipe_scene[SCENE_PIPE_DEPTHFILL] = make_scene_pipeline(world_vs, scene_fs, 0, 1, 0)
	pipe_scene[SCENE_PIPE_WATER] = make_scene_pipeline(world_vs, water_fs, 0, 1, 1)
	pipe_scene[SCENE_PIPE_TEX_BLEND] = make_scene_pipeline(world_vs, scene_fs, SCENE_BLEND_ALPHA, 1, 1)
	pipe_scene[SCENE_PIPE_TEX_ALPHATEST] = make_scene_pipeline(world_vs, scene_at_fs, 0, 1, 1)
	pipe_scene[SCENE_PIPE_ALIAS_FB] = make_scene_pipeline(world_vs, alias_fb_fs, SCENE_BLEND_ALPHA, 1, 1)
	pipe_scene[SCENE_PIPE_TEX_BLEND_NODEPTHWRITE] = make_scene_pipeline(world_vs, scene_fs, SCENE_BLEND_ALPHA, 0, 1)
	pipe_scene[SCENE_PIPE_ADDALPHA_NODEPTHWRITE] = make_scene_pipeline(world_vs, scene_fs, SCENE_BLEND_ADDALPHA, 0, 1)
	pipe_scene[SCENE_PIPE_INVSRCCOLOR_NODEPTHWRITE] = make_scene_pipeline(world_vs, scene_fs, SCENE_BLEND_INVSRCCOLOR, 0, 1)
	pipe_scene[SCENE_PIPE_MOD_NODEPTHWRITE] = make_scene_pipeline(world_vs, mod_fs, SCENE_BLEND_MOD, 0, 1)

	for _, s := range []*sdl.GPUShader{ui_vs, ui_fs, ui_at_fs, post_vs, post_fs, world_vs, world_fs, world_at_fs,
		scene_fs, scene_at_fs, sky_vs, sky_fs, water_fs, alias_fb_fs, mod_fs, crt_bloom_fs, crt_phosphor_fs} {
		sdl.ReleaseGPUShader(gpu_device, s)
	}

	ok := pipe_ui != nil && pipe_ui_alphatest != nil && pipe_post != nil && pipe_crt_bloom != nil && pipe_crt_phosphor != nil
	if !ok {
		Com_Printf("GPU: ui/post pipeline creation failed\n")
	}
	for i := 0; i < SCENE_PIPE_COUNT; i++ {
		if pipe_scene[i] == nil {
			Com_Printf("GPU: scene pipeline %d creation failed\n", i)
			ok = false
		}
	}

	if !ok {
		Com_Printf("GPU: pipeline creation failed: %s\n", sdl.GetError())
		return 0
	}

	return 1
}

func destroy_scene_targets() {
	if scene_color != nil {
		sdl.ReleaseGPUTexture(gpu_device, scene_color)
		scene_color = nil
	}
	if scene_depth != nil {
		sdl.ReleaseGPUTexture(gpu_device, scene_depth)
		scene_depth = nil
	}
	for i := 0; i < 2; i++ {
		if crt_tex[i] != nil {
			sdl.ReleaseGPUTexture(gpu_device, crt_tex[i])
			crt_tex[i] = nil
		}
	}
	scene_width = 0
	scene_height = 0
}

func create_scene_targets(width, height uint32) int {
	var info sdl.GPUTextureCreateInfo

	destroy_scene_targets()

	info.Type = sdl.GPU_TEXTURETYPE_2D
	info.Format = sdl.GPU_TEXTUREFORMAT_R8G8B8A8_UNORM
	info.Usage = sdl.GPU_TEXTUREUSAGE_COLOR_TARGET | sdl.GPU_TEXTUREUSAGE_SAMPLER
	info.Width = width
	info.Height = height
	info.LayerCountOrDepth = 1
	info.NumLevels = 1
	info.SampleCount = sdl.GPU_SAMPLECOUNT_1

	scene_color = sdl.CreateGPUTexture(gpu_device, &info)
	if scene_color == nil {
		Com_Printf("GPU: scene color target failed: %s\n", sdl.GetError())
		return 0
	}

	info.Format = scene_depth_format
	info.Usage = sdl.GPU_TEXTUREUSAGE_DEPTH_STENCIL_TARGET

	scene_depth = sdl.CreateGPUTexture(gpu_device, &info)
	if scene_depth == nil {
		Com_Printf("GPU: scene depth target failed: %s\n", sdl.GetError())
		destroy_scene_targets()
		return 0
	}

	scene_width = width
	scene_height = height
	return 1
}

func GPU_SetVsync(vsync int) {
	mode := sdl.GPU_PRESENTMODE_VSYNC

	if gpu_device == nil || gpu_window == nil {
		return
	}

	if vsync == 0 && sdl.WindowSupportsGPUPresentMode(gpu_device, gpu_window, sdl.GPU_PRESENTMODE_IMMEDIATE) {
		mode = sdl.GPU_PRESENTMODE_IMMEDIATE
	} else if vsync == -1 && sdl.WindowSupportsGPUPresentMode(gpu_device, gpu_window, sdl.GPU_PRESENTMODE_MAILBOX) {
		mode = sdl.GPU_PRESENTMODE_MAILBOX
	}

	sdl.SetGPUSwapchainParameters(gpu_device, gpu_window, sdl.GPU_SWAPCHAINCOMPOSITION_SDR, mode)
}

// post pipeline targets the swapchain format, which can change with the window
func rebuild_post_pipeline() int {
	if pipe_post != nil {
		sdl.ReleaseGPUGraphicsPipeline(gpu_device, pipe_post)
	}

	post_vs := load_shader(sdl.GPU_SHADERSTAGE_VERTEX, "post.vert", 0, 0)
	post_fs := load_shader(sdl.GPU_SHADERSTAGE_FRAGMENT, "post.frag", 1, 1)
	if post_vs != nil && post_fs != nil {
		pipe_post = make_post_pipeline(post_vs, post_fs, swapchain_format)
	} else {
		pipe_post = nil
	}
	if post_vs != nil {
		sdl.ReleaseGPUShader(gpu_device, post_vs)
	}
	if post_fs != nil {
		sdl.ReleaseGPUShader(gpu_device, post_fs)
	}

	return b2i(pipe_post != nil)
}

// device, pipelines and shared buffers live for the whole process;
// vid_restart only cycles the window, the swapchain and the texture table
func GPU_Init(window *sdl.Window, vsync int) int {
	var bci sdl.GPUBufferCreateInfo
	var tci sdl.GPUTransferBufferCreateInfo
	created := false

	if gpu_device == nil {
		gpu_device = sdl.CreateGPUDevice(
			sdl.GPU_SHADERFORMAT_SPIRV|sdl.GPU_SHADERFORMAT_DXIL|sdl.GPU_SHADERFORMAT_MSL,
			COM_CheckParm("-gpudebug") != 0,
			nil)
		if gpu_device == nil {
			Com_Printf("SDL_CreateGPUDevice failed: %s\n", sdl.GetError())
			return 0
		}
		created = true
	}

	if !sdl.ClaimWindowForGPUDevice(gpu_device, window) {
		Com_Printf("SDL_ClaimWindowForGPUDevice failed: %s\n", sdl.GetError())
		if created {
			GPU_ShutdownAll()
		}
		return 0
	}

	gpu_window = window
	newformat := sdl.GetGPUSwapchainTextureFormat(gpu_device, gpu_window)

	if created {
		swapchain_format = newformat

		if sdl.GPUTextureSupportsFormat(gpu_device, sdl.GPU_TEXTUREFORMAT_D24_UNORM,
			sdl.GPU_TEXTURETYPE_2D, sdl.GPU_TEXTUREUSAGE_DEPTH_STENCIL_TARGET) {
			scene_depth_format = sdl.GPU_TEXTUREFORMAT_D24_UNORM
		} else {
			scene_depth_format = sdl.GPU_TEXTUREFORMAT_D32_FLOAT
		}

		samp_nearest = make_sampler(sdl.GPU_FILTER_NEAREST, sdl.GPU_SAMPLERADDRESSMODE_REPEAT)
		samp_linear = make_sampler(sdl.GPU_FILTER_LINEAR, sdl.GPU_SAMPLERADDRESSMODE_REPEAT)
		samp_nearest_clamp = make_sampler(sdl.GPU_FILTER_NEAREST, sdl.GPU_SAMPLERADDRESSMODE_CLAMP_TO_EDGE)
		samp_linear_clamp = make_sampler(sdl.GPU_FILTER_LINEAR, sdl.GPU_SAMPLERADDRESSMODE_CLAMP_TO_EDGE)

		{
			ok := true
			for p := 0; p < 16; p++ {
				if p&GPU_TEXPREF_MIP_NEAREST != 0 && p&GPU_TEXPREF_MIP_LINEAR != 0 {
					continue
				}
				samp_tex[tex_sampler_index(p)] = make_tex_sampler(p)
				if samp_tex[tex_sampler_index(p)] == nil {
					ok = false
				}
			}
			if !ok {
				Com_Printf("GPU: sampler creation failed: %s\n", sdl.GetError())
				GPU_ShutdownAll()
				return 0
			}
		}

		bci = sdl.GPUBufferCreateInfo{}
		bci.Usage = sdl.GPU_BUFFERUSAGE_VERTEX
		bci.Size = UI_MAX_VERTS * sizeof_ui_vert_t
		ui_vbuf = sdl.CreateGPUBuffer(gpu_device, &bci)

		tci = sdl.GPUTransferBufferCreateInfo{}
		tci.Usage = sdl.GPU_TRANSFERBUFFERUSAGE_UPLOAD
		tci.Size = UI_MAX_VERTS * sizeof_ui_vert_t
		ui_tbuf = sdl.CreateGPUTransferBuffer(gpu_device, &tci)

		bci = sdl.GPUBufferCreateInfo{}
		bci.Usage = sdl.GPU_BUFFERUSAGE_INDEX
		bci.Size = GPU_SCENE_MAX_INDICES * 4
		scene_ibuf = sdl.CreateGPUBuffer(gpu_device, &bci)

		tci = sdl.GPUTransferBufferCreateInfo{}
		tci.Usage = sdl.GPU_TRANSFERBUFFERUSAGE_UPLOAD
		tci.Size = GPU_SCENE_MAX_INDICES * 4
		scene_itbuf = sdl.CreateGPUTransferBuffer(gpu_device, &tci)

		bci = sdl.GPUBufferCreateInfo{}
		bci.Usage = sdl.GPU_BUFFERUSAGE_VERTEX
		bci.Size = GPU_SCENE_MAX_DYNVERTS * sizeof_scene_vert_t
		scene_dynvbuf = sdl.CreateGPUBuffer(gpu_device, &bci)

		tci = sdl.GPUTransferBufferCreateInfo{}
		tci.Usage = sdl.GPU_TRANSFERBUFFERUSAGE_UPLOAD
		tci.Size = GPU_SCENE_MAX_DYNVERTS * sizeof_scene_vert_t
		scene_dyntbuf = sdl.CreateGPUTransferBuffer(gpu_device, &tci)

		if scene_ibuf == nil || scene_itbuf == nil || scene_dynvbuf == nil || scene_dyntbuf == nil ||
			samp_nearest == nil || samp_linear == nil || samp_nearest_clamp == nil || samp_linear_clamp == nil ||
			ui_vbuf == nil || ui_tbuf == nil {
			Com_Printf("GPU: resource creation failed: %s\n", sdl.GetError())
			GPU_ShutdownAll()
			return 0
		}

		if create_pipelines() == 0 {
			GPU_ShutdownAll()
			return 0
		}

		Com_Printf("GPU driver: %s\n", sdl.GetGPUDeviceDriver(gpu_device))
	} else if newformat != swapchain_format {
		swapchain_format = newformat
		if rebuild_post_pipeline() == 0 {
			Com_Printf("GPU: post pipeline rebuild failed: %s\n", sdl.GetError())
			return 0
		}
	}

	{
		frames := int(vid_framesinflight.value)
		if frames < 1 {
			frames = 1
		}
		if frames > 3 {
			frames = 3
		}
		sdl.SetGPUAllowedFramesInFlight(gpu_device, uint32(frames))
	}

	GPU_Texture_InitTable()
	GPU_SetVsync(vsync)

	return 1
}

// releases everything tied to the current window, keeps the device alive
func GPU_Shutdown() {
	if gpu_device == nil {
		return
	}

	if frame_cmdbuf != nil {
		sdl.SubmitGPUCommandBuffer(frame_cmdbuf)
		frame_cmdbuf = nil
	}

	sdl.WaitForGPUIdle(gpu_device)

	GPU_Texture_ShutdownTable()

	if readback_buffer != nil {
		sdl.ReleaseGPUTransferBuffer(gpu_device, readback_buffer)
		readback_buffer = nil
		readback_size = 0
	}
	destroy_scene_targets()

	if gpu_window != nil {
		sdl.ReleaseWindowFromGPUDevice(gpu_device, gpu_window)
		gpu_window = nil
	}
}

func GPU_ShutdownAll() {
	if gpu_device == nil {
		return
	}

	GPU_Shutdown()

	for _, p := range []*sdl.GPUGraphicsPipeline{pipe_ui, pipe_ui_alphatest, pipe_post, pipe_crt_bloom, pipe_crt_phosphor} {
		if p != nil {
			sdl.ReleaseGPUGraphicsPipeline(gpu_device, p)
		}
	}
	pipe_ui, pipe_ui_alphatest, pipe_post, pipe_crt_bloom, pipe_crt_phosphor = nil, nil, nil, nil, nil

	for i := 0; i < SCENE_PIPE_COUNT; i++ {
		if pipe_scene[i] != nil {
			sdl.ReleaseGPUGraphicsPipeline(gpu_device, pipe_scene[i])
		}
		pipe_scene[i] = nil
	}

	if scene_ibuf != nil {
		sdl.ReleaseGPUBuffer(gpu_device, scene_ibuf)
	}
	if scene_itbuf != nil {
		sdl.ReleaseGPUTransferBuffer(gpu_device, scene_itbuf)
	}
	scene_ibuf = nil
	scene_itbuf = nil
	if scene_dynvbuf != nil {
		sdl.ReleaseGPUBuffer(gpu_device, scene_dynvbuf)
	}
	if scene_dyntbuf != nil {
		sdl.ReleaseGPUTransferBuffer(gpu_device, scene_dyntbuf)
	}
	scene_dynvbuf = nil
	scene_dyntbuf = nil
	scene_uploader = nil

	for _, s := range []*sdl.GPUSampler{samp_nearest, samp_linear, samp_nearest_clamp, samp_linear_clamp} {
		if s != nil {
			sdl.ReleaseGPUSampler(gpu_device, s)
		}
	}
	samp_nearest, samp_linear, samp_nearest_clamp, samp_linear_clamp = nil, nil, nil, nil

	for i := 0; i < 12; i++ {
		if samp_tex[i] != nil {
			sdl.ReleaseGPUSampler(gpu_device, samp_tex[i])
		}
		samp_tex[i] = nil
	}

	if ui_vbuf != nil {
		sdl.ReleaseGPUBuffer(gpu_device, ui_vbuf)
	}
	if ui_tbuf != nil {
		sdl.ReleaseGPUTransferBuffer(gpu_device, ui_tbuf)
	}
	ui_vbuf = nil
	ui_tbuf = nil

	sdl.DestroyGPUDevice(gpu_device)
	gpu_device = nil
}

func GPU_BeginFrame(width, height uint32) {
	if gpu_device == nil || frame_cmdbuf != nil {
		return
	}

	if width == 0 || height == 0 {
		return
	}

	if width != scene_width || height != scene_height {
		sdl.WaitForGPUIdle(gpu_device)
		if create_scene_targets(width, height) == 0 {
			return
		}
	}

	frame_cmdbuf = sdl.AcquireGPUCommandBuffer(gpu_device)
	if frame_cmdbuf == nil {
		Com_Printf("GPU: command buffer acquire failed: %s\n", sdl.GetError())
		return
	}

	Draw2D_FrameReset()
	Scene_FrameReset()
}

func GPU_SetPostParams(gamma, contrast float32, blend []float32) {
	post_gamma = gamma
	post_contrast = contrast
	if blend != nil {
		copy(post_blend[:], blend[:4])
	}
}

var upload_inflight_bytes uint32

func GPU_TrackUploadBytes(bytes uint32) {
	upload_inflight_bytes += bytes
	if upload_inflight_bytes >= (32<<20) && gpu_device != nil {
		sdl.WaitForGPUIdle(gpu_device)
		upload_inflight_bytes = 0
	}
}

func sampler_for_prefs(prefs int) *sdl.GPUSampler {
	s := samp_tex[tex_sampler_index(prefs)]

	if s != nil {
		return s
	}
	if prefs&GPU_TEXPREF_LINEAR != 0 {
		if prefs&GPU_TEXPREF_CLAMP != 0 {
			return samp_linear_clamp
		}
		return samp_linear
	}
	if prefs&GPU_TEXPREF_CLAMP != 0 {
		return samp_nearest_clamp
	}
	return samp_nearest
}

// ---- 3D scene recording ----

func GPU_CreateStaticVertexBuffer(verts []scene_vert_t) *sdl.GPUBuffer {
	var bci sdl.GPUBufferCreateInfo
	var tci sdl.GPUTransferBufferCreateInfo
	var src sdl.GPUTransferBufferLocation
	var dst sdl.GPUBufferRegion

	count := uint32(len(verts))
	if gpu_device == nil || verts == nil || count == 0 {
		return nil
	}

	bci.Usage = sdl.GPU_BUFFERUSAGE_VERTEX
	bci.Size = count * sizeof_scene_vert_t
	buf := sdl.CreateGPUBuffer(gpu_device, &bci)
	if buf == nil {
		return nil
	}

	tci.Usage = sdl.GPU_TRANSFERBUFFERUSAGE_UPLOAD
	tci.Size = bci.Size
	tbuf := sdl.CreateGPUTransferBuffer(gpu_device, &tci)
	if tbuf == nil {
		sdl.ReleaseGPUBuffer(gpu_device, buf)
		return nil
	}

	mapped := sdl.MapGPUTransferBuffer(gpu_device, tbuf, false)
	if mapped != nil {
		copy(unsafe.Slice((*scene_vert_t)(mapped), count), verts)
		sdl.UnmapGPUTransferBuffer(gpu_device, tbuf)
	}

	cmdbuf := sdl.AcquireGPUCommandBuffer(gpu_device)
	if cmdbuf != nil {
		copyPass := sdl.BeginGPUCopyPass(cmdbuf)
		src.TransferBuffer = tbuf
		dst.Buffer = buf
		dst.Size = bci.Size
		sdl.UploadToGPUBuffer(copyPass, &src, &dst, false)
		sdl.EndGPUCopyPass(copyPass)
		sdl.SubmitGPUCommandBuffer(cmdbuf)
	}

	sdl.ReleaseGPUTransferBuffer(gpu_device, tbuf)
	GPU_TrackUploadBytes(bci.Size)
	return buf
}

func GPU_ReleaseBuffer(buf *sdl.GPUBuffer) {
	if gpu_device != nil && buf != nil {
		sdl.WaitForGPUIdle(gpu_device)
		sdl.ReleaseGPUBuffer(gpu_device, buf)
	}
}

func GPU_CreateDynamicTexture(width, height uint32) *sdl.GPUTexture {
	var ci sdl.GPUTextureCreateInfo

	if gpu_device == nil {
		return nil
	}

	ci.Type = sdl.GPU_TEXTURETYPE_2D
	ci.Format = sdl.GPU_TEXTUREFORMAT_R8G8B8A8_UNORM
	ci.Usage = sdl.GPU_TEXTUREUSAGE_SAMPLER
	ci.Width = width
	ci.Height = height
	ci.LayerCountOrDepth = 1
	ci.NumLevels = 1
	ci.SampleCount = sdl.GPU_SAMPLECOUNT_1

	return sdl.CreateGPUTexture(gpu_device, &ci)
}

func GPU_ReleaseTexture(tex *sdl.GPUTexture) {
	if gpu_device != nil && tex != nil {
		sdl.WaitForGPUIdle(gpu_device)
		sdl.ReleaseGPUTexture(gpu_device, tex)
	}
}

func Scene_FrameReset() {
	scene_numindices = 0
	scene_numbatches = 0
	scene_numdynverts = 0
	scene_has_viewport = 0
	autoid_valid = 0
}

// Returns the slice to fill, nil when the frame is full.
func Scene_AllocIndices(count uint32, firstindex *uint32) []uint32 {
	if scene_numindices+count > GPU_SCENE_MAX_INDICES {
		return nil
	}
	*firstindex = scene_numindices
	scene_numindices += count
	return scene_indices[*firstindex : *firstindex+count]
}

func Scene_AllocVerts(count uint32, firstvert *uint32) []scene_vert_t {
	if scene_numdynverts+count > GPU_SCENE_MAX_DYNVERTS {
		return nil
	}
	*firstvert = scene_numdynverts
	scene_numdynverts += count
	return scene_dynverts[*firstvert : *firstvert+count]
}

func GPU_GetDynamicSceneVB() *sdl.GPUBuffer {
	return scene_dynvbuf
}

func Scene_AddBatch(pipe, texnum int, tex2 *sdl.GPUTexture, vbuf *sdl.GPUBuffer,
	firstindex, numindices uint32, mvp []float32) *scene_batch_t {
	if scene_numbatches >= GPU_SCENE_MAX_BATCHES || numindices == 0 {
		return nil
	}

	b := &scene_batches[scene_numbatches]
	scene_numbatches++
	*b = scene_batch_t{}
	b.pipe = pipe
	b.texnum = texnum
	b.tex2 = tex2
	b.vbuf = vbuf
	b.firstindex = firstindex
	b.numindices = numindices
	b.depth_min = 0.0
	b.depth_max = 1.0
	copy(b.mvp[:], mvp[:16])
	return b
}

func Scene_SetViewport(x, y, w, h float32) {
	scene_viewport[0] = x
	scene_viewport[1] = y
	scene_viewport[2] = w
	scene_viewport[3] = h
	scene_has_viewport = 1
}

func GPU_SetSceneUploader(fn func(copy *sdl.GPUCopyPass)) {
	scene_uploader = fn
}

func GPU_SetSceneMatrices(modelview, projection []float32, viewport []int) {
	copy(autoid_modelview[:], modelview[:16])
	copy(autoid_projection[:], projection[:16])
	copy(autoid_viewport[:], viewport[:4])
	autoid_valid = 1
}

// scene render pass: clear + all recorded 2D batches
func record_scene_pass() {
	var ct sdl.GPUColorTargetInfo
	var ds sdl.GPUDepthStencilTargetInfo

	verts := Draw2D_GetVerts()
	batches := Draw2D_GetBatches()
	numverts := uint32(len(verts))
	numbatches := uint32(len(batches))

	if numverts != 0 || scene_numindices != 0 || scene_uploader != nil {
		var src sdl.GPUTransferBufferLocation
		var dst sdl.GPUBufferRegion

		if numverts != 0 {
			mapped := sdl.MapGPUTransferBuffer(gpu_device, ui_tbuf, true)
			if mapped != nil {
				copy(unsafe.Slice((*ui_vert_t)(mapped), numverts), verts)
				sdl.UnmapGPUTransferBuffer(gpu_device, ui_tbuf)
			}
		}

		if scene_numindices != 0 {
			mapped := sdl.MapGPUTransferBuffer(gpu_device, scene_itbuf, true)
			if mapped != nil {
				copy(unsafe.Slice((*uint32)(mapped), scene_numindices), scene_indices[:scene_numindices])
				sdl.UnmapGPUTransferBuffer(gpu_device, scene_itbuf)
			}
		}

		if scene_numdynverts != 0 {
			mapped := sdl.MapGPUTransferBuffer(gpu_device, scene_dyntbuf, true)
			if mapped != nil {
				copy(unsafe.Slice((*scene_vert_t)(mapped), scene_numdynverts), scene_dynverts[:scene_numdynverts])
				sdl.UnmapGPUTransferBuffer(gpu_device, scene_dyntbuf)
			}
		}

		copyPass := sdl.BeginGPUCopyPass(frame_cmdbuf)

		if numverts != 0 {
			src = sdl.GPUTransferBufferLocation{TransferBuffer: ui_tbuf}
			dst = sdl.GPUBufferRegion{Buffer: ui_vbuf, Size: numverts * sizeof_ui_vert_t}
			sdl.UploadToGPUBuffer(copyPass, &src, &dst, true)
		}

		if scene_numindices != 0 {
			src = sdl.GPUTransferBufferLocation{TransferBuffer: scene_itbuf}
			dst = sdl.GPUBufferRegion{Buffer: scene_ibuf, Size: scene_numindices * 4}
			sdl.UploadToGPUBuffer(copyPass, &src, &dst, true)
		}

		if scene_numdynverts != 0 {
			src = sdl.GPUTransferBufferLocation{TransferBuffer: scene_dyntbuf}
			dst = sdl.GPUBufferRegion{Buffer: scene_dynvbuf, Size: scene_numdynverts * sizeof_scene_vert_t}
			sdl.UploadToGPUBuffer(copyPass, &src, &dst, true)
		}

		if scene_uploader != nil {
			scene_uploader(copyPass)
		}

		sdl.EndGPUCopyPass(copyPass)
	}

	ct.Texture = scene_color
	ct.ClearColor.A = 1.0
	ct.LoadOp = sdl.GPU_LOADOP_CLEAR
	ct.StoreOp = sdl.GPU_STOREOP_STORE
	ct.Cycle = true

	ds.Texture = scene_depth
	ds.ClearDepth = 1.0
	ds.LoadOp = sdl.GPU_LOADOP_CLEAR
	ds.StoreOp = sdl.GPU_STOREOP_DONT_CARE
	ds.StencilLoadOp = sdl.GPU_LOADOP_DONT_CARE
	ds.StencilStoreOp = sdl.GPU_STOREOP_DONT_CARE
	ds.Cycle = true

	pass := sdl.BeginGPURenderPass(frame_cmdbuf, &ct, 1, &ds)
	if pass == nil {
		return
	}

	if scene_numbatches != 0 {
		var ib sdl.GPUBufferBinding
		var bound_vbuf *sdl.GPUBuffer
		var cur_depth_min, cur_depth_max float32 = 0.0, 1.0

		if scene_has_viewport != 0 {
			vp := sdl.GPUViewport{X: scene_viewport[0], Y: scene_viewport[1], W: scene_viewport[2], H: scene_viewport[3], MinDepth: 0.0, MaxDepth: 1.0}
			sdl.SetGPUViewport(pass, &vp)
		}

		ib.Buffer = scene_ibuf
		sdl.BindGPUIndexBuffer(pass, &ib, sdl.GPU_INDEXELEMENTSIZE_32BIT)

		for i := uint32(0); i < scene_numbatches; i++ {
			b := &scene_batches[i]
			var tsb [2]sdl.GPUTextureSamplerBinding
			prefs := 0

			tex := GPU_Texture_Lookup(b.texnum, &prefs)
			if tex == nil || b.vbuf == nil {
				continue
			}

			if b.depth_min != cur_depth_min || b.depth_max != cur_depth_max {
				var vp sdl.GPUViewport
				if scene_has_viewport != 0 {
					vp.X, vp.Y, vp.W, vp.H = scene_viewport[0], scene_viewport[1], scene_viewport[2], scene_viewport[3]
				} else {
					vp.X, vp.Y, vp.W, vp.H = 0.0, 0.0, float32(scene_width), float32(scene_height)
				}
				vp.MinDepth = b.depth_min
				vp.MaxDepth = b.depth_max
				sdl.SetGPUViewport(pass, &vp)
				cur_depth_min = b.depth_min
				cur_depth_max = b.depth_max
			}

			if b.vbuf != bound_vbuf {
				vb := sdl.GPUBufferBinding{Buffer: b.vbuf}
				sdl.BindGPUVertexBuffers(pass, 0, &vb, 1)
				bound_vbuf = b.vbuf
			}

			sdl.BindGPUGraphicsPipeline(pass, pipe_scene[b.pipe])

			if b.pipe == SCENE_PIPE_SKY {
				var ubo struct {
					mvp    [16]float32
					params [8]float32
				}
				ubo.mvp = b.mvp
				ubo.params = b.params
				sdl.PushGPUVertexUniformData(frame_cmdbuf, 0, unsafe.Pointer(&ubo), uint32(unsafe.Sizeof(ubo)))
			} else {
				sdl.PushGPUVertexUniformData(frame_cmdbuf, 0, unsafe.Pointer(&b.mvp[0]), uint32(unsafe.Sizeof(b.mvp)))
			}

			if b.pipe == SCENE_PIPE_WATER {
				sdl.PushGPUFragmentUniformData(frame_cmdbuf, 0, unsafe.Pointer(&b.params[0]), 4*4)
			}

			tsb[0].Texture = tex
			tsb[0].Sampler = sampler_for_prefs(prefs)
			numtex := uint32(1)
			if b.pipe == SCENE_PIPE_WORLD || b.pipe == SCENE_PIPE_WORLD_ALPHATEST ||
				b.pipe == SCENE_PIPE_SKY || b.pipe == SCENE_PIPE_ALIAS_FB {
				if b.tex2 != nil {
					tsb[1].Texture = b.tex2
				} else {
					tsb[1].Texture = tex
				}
				tsb[1].Sampler = samp_linear
				numtex = 2
			}
			sdl.BindGPUFragmentSamplers(pass, 0, &tsb[0], numtex)

			sdl.DrawGPUIndexedPrimitives(pass, b.numindices, 1, b.firstindex, 0, 0)
		}

		if scene_has_viewport != 0 {
			vp := sdl.GPUViewport{X: 0.0, Y: 0.0, W: float32(scene_width), H: float32(scene_height), MinDepth: 0.0, MaxDepth: 1.0}
			sdl.SetGPUViewport(pass, &vp)
		}
	}

	if numbatches != 0 {
		vb := sdl.GPUBufferBinding{Buffer: ui_vbuf}
		sdl.BindGPUVertexBuffers(pass, 0, &vb, 1)

		for i := uint32(0); i < numbatches; i++ {
			b := &batches[i]
			var tsb sdl.GPUTextureSamplerBinding
			var ortho [16]float32
			prefs := 0

			tex := GPU_Texture_Lookup(b.texnum, &prefs)
			if tex == nil {
				continue
			}

			if b.alphatest != 0 {
				sdl.BindGPUGraphicsPipeline(pass, pipe_ui_alphatest)
			} else {
				sdl.BindGPUGraphicsPipeline(pass, pipe_ui)
			}

			ortho[0] = 2.0 / b.ortho_w
			ortho[5] = -2.0 / b.ortho_h
			ortho[10] = 1.0
			ortho[12] = -1.0
			ortho[13] = 1.0
			ortho[15] = 1.0
			sdl.PushGPUVertexUniformData(frame_cmdbuf, 0, unsafe.Pointer(&ortho[0]), uint32(unsafe.Sizeof(ortho)))

			tsb.Texture = tex
			tsb.Sampler = sampler_for_prefs(prefs)
			sdl.BindGPUFragmentSamplers(pass, 0, &tsb, 1)

			sdl.DrawGPUPrimitives(pass, b.numverts, 1, b.firstvert, 0)
		}
	}

	sdl.EndGPURenderPass(pass)
}

func record_post_pass(swap_tex *sdl.GPUTexture, swap_w, swap_h uint32, src *sdl.GPUTexture) {
	var ct sdl.GPUColorTargetInfo
	var tsb sdl.GPUTextureSamplerBinding
	var ubo struct {
		blend    [4]float32
		gamma    float32
		contrast float32
		pad      [2]float32
	}

	ct.Texture = swap_tex
	ct.LoadOp = sdl.GPU_LOADOP_DONT_CARE
	ct.StoreOp = sdl.GPU_STOREOP_STORE

	pass := sdl.BeginGPURenderPass(frame_cmdbuf, &ct, 1, nil)
	if pass == nil {
		return
	}

	sdl.BindGPUGraphicsPipeline(pass, pipe_post)

	ubo.blend = post_blend
	ubo.gamma = post_gamma * crt_comp_gamma
	ubo.contrast = post_contrast
	sdl.PushGPUFragmentUniformData(frame_cmdbuf, 0, unsafe.Pointer(&ubo), uint32(unsafe.Sizeof(ubo)))

	tsb.Texture = src
	tsb.Sampler = samp_nearest_clamp
	sdl.BindGPUFragmentSamplers(pass, 0, &tsb, 1)

	sdl.DrawGPUPrimitives(pass, 3, 1, 0, 0)

	sdl.EndGPURenderPass(pass)
}

func ensure_crt_targets() int {
	if crt_tex[0] != nil && crt_tex[1] != nil {
		return 1
	}

	// &info goes to SDL, so it lives on the heap: not before the early return
	var info sdl.GPUTextureCreateInfo
	info.Type = sdl.GPU_TEXTURETYPE_2D
	info.Format = sdl.GPU_TEXTUREFORMAT_R8G8B8A8_UNORM
	info.Usage = sdl.GPU_TEXTUREUSAGE_COLOR_TARGET | sdl.GPU_TEXTUREUSAGE_SAMPLER
	info.Width = scene_width
	info.Height = scene_height
	info.LayerCountOrDepth = 1
	info.NumLevels = 1
	info.SampleCount = sdl.GPU_SAMPLECOUNT_1

	for i := 0; i < 2; i++ {
		if crt_tex[i] == nil {
			crt_tex[i] = sdl.CreateGPUTexture(gpu_device, &info)
		}
		if crt_tex[i] == nil {
			return 0
		}
	}

	return 1
}

func record_crt_pass(cb *sdl.GPUCommandBuffer, pipe *sdl.GPUGraphicsPipeline, src, dst *sdl.GPUTexture) {
	var ct sdl.GPUColorTargetInfo
	var tsb sdl.GPUTextureSamplerBinding
	var ubo struct {
		res [2]float32
		pad [2]float32
	}

	ct.Texture = dst
	ct.LoadOp = sdl.GPU_LOADOP_DONT_CARE
	ct.StoreOp = sdl.GPU_STOREOP_STORE
	ct.Cycle = true

	pass := sdl.BeginGPURenderPass(cb, &ct, 1, nil)
	if pass == nil {
		return
	}

	// mask density tracks ~480 logical pixels across, not the physical resolution;
	// scale 1 would alias the mask away, so small windows get a coarser grid instead
	scale := int(float32(scene_width)/480.0 + 0.5)
	if scale < 2 {
		scale = 2
	}
	ubo.res[0] = float32(scene_width) / float32(scale)
	ubo.res[1] = float32(scene_height) / float32(scale)

	sdl.BindGPUGraphicsPipeline(pass, pipe)
	sdl.PushGPUFragmentUniformData(cb, 0, unsafe.Pointer(&ubo), uint32(unsafe.Sizeof(ubo)))

	tsb.Texture = src
	tsb.Sampler = samp_linear_clamp
	sdl.BindGPUFragmentSamplers(pass, 0, &tsb, 1)

	sdl.DrawGPUPrimitives(pass, 3, 1, 0, 0)

	sdl.EndGPURenderPass(pass)
}

func record_crt_chain(cb *sdl.GPUCommandBuffer) *sdl.GPUTexture {
	src := scene_color

	crt_comp_gamma = 1.0
	if (r_crt_dotbloom.value == 0 && r_crt_phosphor.value == 0) || ensure_crt_targets() == 0 {
		return src
	}

	if r_crt_dotbloom.value != 0 {
		record_crt_pass(cb, pipe_crt_bloom, src, crt_tex[0])
		src = crt_tex[0]
	}
	if r_crt_phosphor.value != 0 {
		dst := crt_tex[0]
		if src == crt_tex[0] {
			dst = crt_tex[1]
		}
		record_crt_pass(cb, pipe_crt_phosphor, src, dst)
		src = dst
	}

	// gamma lift compensating perceived brightness lost to the masks
	if r_crt_dotbloom.value != 0 && r_crt_phosphor.value != 0 {
		crt_comp_gamma = 0.82
	} else {
		crt_comp_gamma = 0.94
	}
	return src
}

func GPU_CrtCompGamma() float32 {
	return crt_comp_gamma
}

var autoshot_frames int
var autoshot_shotframe = -1

func GPU_EndFrame() {
	var swap_tex *sdl.GPUTexture
	var swap_w, swap_h uint32

	if gpu_device == nil || frame_cmdbuf == nil {
		return
	}

	record_scene_pass()

	post_src := record_crt_chain(frame_cmdbuf)

	if !sdl.WaitAndAcquireGPUSwapchainTexture(frame_cmdbuf, gpu_window, &swap_tex, &swap_w, &swap_h) {
		swap_tex = nil
	}

	if swap_tex != nil {
		record_post_pass(swap_tex, swap_w, swap_h, post_src)
	}

	sdl.SubmitGPUCommandBuffer(frame_cmdbuf)
	frame_cmdbuf = nil

	// presenting submit triggers SDL's own deferred cleanup
	if swap_tex != nil {
		upload_inflight_bytes = 0
	}

	// test hook: -autoshot [frame] screenshots at frame N (default 60), quits at N+10
	if autoshot_shotframe == -1 {
		parm := COM_CheckParm("-autoshot")
		autoshot_shotframe = 0
		if parm != 0 {
			autoshot_shotframe = 60
			if parm+1 < com_argc && at(com_argv[parm+1], 0) != '-' && at(com_argv[parm+1], 0) != '+' {
				autoshot_shotframe = max(10, Q_atoi(com_argv[parm+1]))
			}
		}
	}
	if autoshot_shotframe != 0 {
		autoshot_frames++
		// -testrestart exercises vid_restart mid-run
		if COM_CheckParm("-testrestart") != 0 {
			if autoshot_frames == 150 {
				Cbuf_AddText("menu_video\n")
			}
			if autoshot_frames == 200 {
				Cbuf_AddText("vid_fullscreen 0\nvid_width 1280\nvid_height 720\nvid_restart\n")
			}
			if autoshot_frames == 300 {
				Cbuf_AddText("vid_fullscreen 1\nvid_mode \"\"\nvid_restart\n")
			}
			if autoshot_frames == 380 {
				Cbuf_AddText("vid_fullscreen 0\nvid_width 1600\nvid_height 900\nvid_restart\n")
			}
		}
		// -autoshotstep K: from shotframe on, screenshot every K frames until shotframe+10K
		if autoshot_frames >= autoshot_shotframe {
			step := 0
			if parm := COM_CheckParm("-autoshotstep"); parm != 0 && parm+1 < com_argc {
				step = max(1, Q_atoi(com_argv[parm+1]))
			}
			if step != 0 {
				if (autoshot_frames-autoshot_shotframe)%step == 0 && autoshot_frames < autoshot_shotframe+10*step {
					Cbuf_AddText(va("screenshot autoshot_%d\n", autoshot_frames))
				}
				if autoshot_frames == autoshot_shotframe+10*step {
					Host_Quit()
				}
			} else {
				if autoshot_frames == autoshot_shotframe {
					Cbuf_AddText("screenshot autoshot\n")
				}
				if autoshot_frames == autoshot_shotframe+10 {
					Host_Quit()
				}
			}
		}
	}
}

func GPU_GetDevice() *sdl.GPUDevice {
	return gpu_device
}

func GPU_GetSceneMatrices(modelview, projection []float32, viewport []int) int {
	if autoid_valid == 0 {
		return 0
	}
	copy(modelview, autoid_modelview[:])
	copy(projection, autoid_projection[:])
	copy(viewport, autoid_viewport[:])
	return 1
}

// rgb, 3 bytes/px, rows bottom-up (glReadPixels convention)
func GPU_ReadPixels(rgb []byte, width, height uint32) int {
	var cmdbuf *sdl.GPUCommandBuffer
	var region sdl.GPUTextureRegion
	var transfer sdl.GPUTextureTransferInfo

	if gpu_device == nil || scene_color == nil || width != scene_width || height != scene_height {
		return 0
	}

	needed := width * height * 4
	if readback_buffer != nil && readback_size < needed {
		sdl.ReleaseGPUTransferBuffer(gpu_device, readback_buffer)
		readback_buffer = nil
	}
	if readback_buffer == nil {
		var info sdl.GPUTransferBufferCreateInfo
		info.Usage = sdl.GPU_TRANSFERBUFFERUSAGE_DOWNLOAD
		info.Size = needed
		readback_buffer = sdl.CreateGPUTransferBuffer(gpu_device, &info)
		if readback_buffer == nil {
			return 0
		}
		readback_size = needed
	}

	// mid-frame: flush the open cmdbuf so the shot has this frame, skipping its presentation
	if frame_cmdbuf != nil {
		record_scene_pass()
		cmdbuf = frame_cmdbuf
		frame_cmdbuf = nil
	} else {
		cmdbuf = sdl.AcquireGPUCommandBuffer(gpu_device)
		if cmdbuf == nil {
			return 0
		}
	}

	read_tex := record_crt_chain(cmdbuf)

	copyPass := sdl.BeginGPUCopyPass(cmdbuf)

	region.Texture = read_tex
	region.W = width
	region.H = height
	region.D = 1

	transfer.TransferBuffer = readback_buffer

	sdl.DownloadFromGPUTexture(copyPass, &region, &transfer)
	sdl.EndGPUCopyPass(copyPass)

	fence := sdl.SubmitGPUCommandBufferAndAcquireFence(cmdbuf)
	if fence == nil {
		return 0
	}
	sdl.WaitForGPUFences(gpu_device, true, &fence, 1)
	sdl.ReleaseGPUFence(gpu_device, fence)

	mapped := sdl.MapGPUTransferBuffer(gpu_device, readback_buffer, false)
	if mapped == nil {
		return 0
	}
	pixels := unsafe.Slice((*byte)(mapped), needed)

	// flip to bottom-up and drop alpha, glReadPixels convention
	for y := uint32(0); y < height; y++ {
		src := pixels[(height-1-y)*width*4:]
		dst := rgb[y*width*3:]
		for x := uint32(0); x < width; x++ {
			dst[x*3+0] = src[x*4+0]
			dst[x*3+1] = src[x*4+1]
			dst[x*3+2] = src[x*4+2]
		}
	}

	sdl.UnmapGPUTransferBuffer(gpu_device, readback_buffer)
	return 1
}

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
	"unsafe"

	"github.com/classicq/classicq/internal/sdl"
)

// r_texture.h
const (
	TEX_COMPLAIN   = 1
	TEX_MIPMAP     = 2
	TEX_ALPHA      = 4
	TEX_LUMA       = 8
	TEX_FULLBRIGHT = 16
	TEX_NOSCALE    = 32
	TEX_BRIGHTEN   = 64
	TEX_NOCOMPRESS = 128

	MAX_GLTEXTURES = 16384
)

// r_local.h, little endian
const COLOURMASK_RGBA = 0x00ffffff

// gpu_texture.c

const MAX_GPUTEXTURES = 16384

type texentry struct {
	tex    *sdl.GPUTexture
	width  uint32
	height uint32
	prefs  int
}

var textable [MAX_GPUTEXTURES]texentry
var white_texnum int

// identifier cache, semantics from gl_texture.c
type gltexture_t struct {
	texnum                      int
	identifier                  string // 64 bytes in C
	width, height               int
	scaled_width, scaled_height int
	texmode                     int
	crc                         uint32
	bpp                         int
}

var gltextures [MAX_GLTEXTURES]gltexture_t
var numgltextures int

var gl_picmip = cvar_t{name: "gl_picmip", string: "0"}
var gl_lerpimages = cvar_t{name: "gl_lerpimages", string: "1"}
var gl_texturemode = cvar_t{name: "gl_texturemode", string: "gl_nearest"}

func mip_levels(w, h uint32) uint32 {
	levels := uint32(1)
	m := w
	if h > w {
		m = h
	}

	for m > 1 {
		m >>= 1
		levels++
	}
	return levels
}

func GPU_CreateTextureRGBA(rgba []byte, width, height uint32, mipmap int) *sdl.GPUTexture {
	device := GPU_GetDevice()
	var ci sdl.GPUTextureCreateInfo
	var tci sdl.GPUTransferBufferCreateInfo
	var transfer sdl.GPUTextureTransferInfo
	var region sdl.GPUTextureRegion

	if device == nil || rgba == nil || width == 0 || height == 0 {
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

	if mipmap != 0 {
		// mip chain generated on GPU, needs color target usage
		ci.Usage |= sdl.GPU_TEXTUREUSAGE_COLOR_TARGET
		ci.NumLevels = mip_levels(width, height)
	}

	tex := sdl.CreateGPUTexture(device, &ci)
	if tex == nil {
		Com_Printf("GPU: texture create failed: %s\n", sdl.GetError())
		return nil
	}

	tci.Usage = sdl.GPU_TRANSFERBUFFERUSAGE_UPLOAD
	tci.Size = width * height * 4
	tbuf := sdl.CreateGPUTransferBuffer(device, &tci)
	if tbuf == nil {
		sdl.ReleaseGPUTexture(device, tex)
		return nil
	}

	mapped := sdl.MapGPUTransferBuffer(device, tbuf, false)
	if mapped == nil {
		sdl.ReleaseGPUTransferBuffer(device, tbuf)
		sdl.ReleaseGPUTexture(device, tex)
		return nil
	}
	copy(unsafe.Slice((*byte)(mapped), width*height*4), rgba)
	sdl.UnmapGPUTransferBuffer(device, tbuf)

	cmdbuf := sdl.AcquireGPUCommandBuffer(device)
	if cmdbuf == nil {
		sdl.ReleaseGPUTransferBuffer(device, tbuf)
		sdl.ReleaseGPUTexture(device, tex)
		return nil
	}

	copyPass := sdl.BeginGPUCopyPass(cmdbuf)

	transfer.TransferBuffer = tbuf

	region.Texture = tex
	region.W = width
	region.H = height
	region.D = 1

	sdl.UploadToGPUTexture(copyPass, &transfer, &region, false)
	sdl.EndGPUCopyPass(copyPass)

	if mipmap != 0 && ci.NumLevels > 1 {
		sdl.GenerateMipmapsForGPUTexture(cmdbuf, tex)
	}

	sdl.SubmitGPUCommandBuffer(cmdbuf)
	sdl.ReleaseGPUTransferBuffer(device, tbuf)
	GPU_TrackUploadBytes(width * height * 4)

	return tex
}

func GPU_Texture_Set(texnum int, tex *sdl.GPUTexture, width, height uint32, prefs int) {
	device := GPU_GetDevice()

	if texnum < 0 || texnum >= MAX_GPUTEXTURES {
		Com_Printf("GPU: texnum %d out of range\n", texnum)
		if tex != nil && device != nil {
			sdl.ReleaseGPUTexture(device, tex)
		}
		return
	}

	if textable[texnum].tex != nil && device != nil {
		sdl.ReleaseGPUTexture(device, textable[texnum].tex)
	}

	textable[texnum].tex = tex
	textable[texnum].width = width
	textable[texnum].height = height
	textable[texnum].prefs = prefs
}

func GPU_Texture_SetPrefs(texnum int, prefs int) {
	if texnum >= 0 && texnum < MAX_GPUTEXTURES {
		textable[texnum].prefs = prefs
	}
}

// per-frame refresh (netgraph); reuses the texture when dimensions match
func GPU_UpdateTextureRGBA(texnum int, rgba []byte, width, height uint32, prefs int) {
	device := GPU_GetDevice()
	var tci sdl.GPUTransferBufferCreateInfo
	var transfer sdl.GPUTextureTransferInfo
	var region sdl.GPUTextureRegion

	if device == nil || texnum < 0 || texnum >= MAX_GPUTEXTURES {
		return
	}

	e := &textable[texnum]
	if e.tex == nil || e.width != width || e.height != height {
		GPU_Texture_Set(texnum, GPU_CreateTextureRGBA(rgba, width, height, 0), width, height, prefs)
		return
	}

	e.prefs = prefs

	tci.Usage = sdl.GPU_TRANSFERBUFFERUSAGE_UPLOAD
	tci.Size = width * height * 4
	tbuf := sdl.CreateGPUTransferBuffer(device, &tci)
	if tbuf == nil {
		return
	}

	mapped := sdl.MapGPUTransferBuffer(device, tbuf, false)
	if mapped == nil {
		sdl.ReleaseGPUTransferBuffer(device, tbuf)
		return
	}
	copy(unsafe.Slice((*byte)(mapped), width*height*4), rgba)
	sdl.UnmapGPUTransferBuffer(device, tbuf)

	cmdbuf := sdl.AcquireGPUCommandBuffer(device)
	if cmdbuf == nil {
		sdl.ReleaseGPUTransferBuffer(device, tbuf)
		return
	}

	copyPass := sdl.BeginGPUCopyPass(cmdbuf)

	transfer.TransferBuffer = tbuf

	region.Texture = e.tex
	region.W = width
	region.H = height
	region.D = 1

	// cycle avoids stalling on the previous frame still sampling it
	sdl.UploadToGPUTexture(copyPass, &transfer, &region, true)
	sdl.EndGPUCopyPass(copyPass)
	sdl.SubmitGPUCommandBuffer(cmdbuf)
	sdl.ReleaseGPUTransferBuffer(device, tbuf)
	GPU_TrackUploadBytes(width * height * 4)
}

func GPU_Texture_Lookup(texnum int, prefs *int) *sdl.GPUTexture {
	if texnum < 0 || texnum >= MAX_GPUTEXTURES || textable[texnum].tex == nil {
		return nil
	}
	if prefs != nil {
		*prefs = textable[texnum].prefs
	}
	return textable[texnum].tex
}

func GPU_Texture_White() int {
	return white_texnum
}

func GPU_Texture_InitTable() {
	white := []byte{255, 255, 255, 255}

	textable = [MAX_GPUTEXTURES]texentry{}

	if white_texnum == 0 {
		white_texnum = texture_extension_number
		texture_extension_number++
	}
	tex := GPU_CreateTextureRGBA(white, 1, 1, 0)
	GPU_Texture_Set(white_texnum, tex, 1, 1, 0)
}

func GPU_Texture_ShutdownTable() {
	device := GPU_GetDevice()

	for i := 0; i < MAX_GPUTEXTURES; i++ {
		if textable[i].tex != nil && device != nil {
			sdl.ReleaseGPUTexture(device, textable[i].tex)
		}
		textable[i].tex = nil
	}
}

// ---- engine-facing texture API (gl_texture.c semantics) ----

func R_Bind(texnum int) {
	currenttexture = texnum
}

func scale_dimensions(width, height int, scaled_width, scaled_height *int, mode int) {
	w, h := width, height

	// NPOT is native now, only picmip and the size cap remain
	if mode&TEX_MIPMAP != 0 && mode&TEX_NOSCALE == 0 {
		shift := int(bound(float32(0), gl_picmip.value, float32(16)))
		w >>= shift
		h >>= shift
		if float32(w) > gl_max_size.value {
			w = int(gl_max_size.value)
		}
		if float32(h) > gl_max_size.value {
			h = int(gl_max_size.value)
		}
	} else {
		if w > gl_max_size_default {
			w = gl_max_size_default
		}
		if h > gl_max_size_default {
			h = gl_max_size_default
		}
	}

	*scaled_width = max(1, w)
	*scaled_height = max(1, h)
}

func GPU_PrefsForTexturemode() int {
	s := gl_texturemode.string
	prefs := 0

	if Q_strcasecmp(s, "gl_nearest") != 0 && Q_strncasecmp(s, "gl_nearest_", 11) != 0 {
		prefs |= GPU_TEXPREF_LINEAR
	}

	if Q_strcasecmp(s, "gl_nearest_mipmap_nearest") == 0 || Q_strcasecmp(s, "gl_linear_mipmap_nearest") == 0 {
		prefs |= GPU_TEXPREF_MIP_NEAREST
	} else if Q_strcasecmp(s, "gl_nearest_mipmap_linear") == 0 || Q_strcasecmp(s, "gl_linear_mipmap_linear") == 0 {
		prefs |= GPU_TEXPREF_MIP_LINEAR
	}

	return prefs
}

func prefs_for_mode(mode int) int {
	return GPU_PrefsForTexturemode()
}

// data is RGBA, 4 bytes per pixel
func R_Upload32(data []byte, width, height int, mode int) {
	var scaled_width, scaled_height int

	scale_dimensions(width, height, &scaled_width, &scaled_height, mode)

	pixels := data
	if scaled_width != width || scaled_height != height {
		scaled := make([]byte, scaled_width*scaled_height*4)
		Image_Resample(data, width, height, scaled, scaled_width, scaled_height, 4, b2i(gl_lerpimages.value != 0))
		pixels = scaled
	}

	tex := GPU_CreateTextureRGBA(pixels, uint32(scaled_width), uint32(scaled_height), b2i(mode&TEX_MIPMAP != 0))
	GPU_Texture_Set(currenttexture, tex, uint32(scaled_width), uint32(scaled_height), prefs_for_mode(mode))
}

func R_Upload8(data []byte, width, height int, mode int) {
	has_alpha := 0

	size := width * height
	buf := make([]byte, size*4)

	table := &d_8to24table
	if mode&TEX_BRIGHTEN != 0 {
		table = &d_8to24table2
	}

	put := func(i int, v uint32) {
		buf[i*4+0] = byte(v)
		buf[i*4+1] = byte(v >> 8)
		buf[i*4+2] = byte(v >> 16)
		buf[i*4+3] = byte(v >> 24)
	}

	if mode&TEX_FULLBRIGHT != 0 {
		mode |= TEX_ALPHA
		for i := 0; i < size; i++ {
			if data[i] < 224 {
				put(i, table[data[i]]&COLOURMASK_RGBA) // zero alpha
			} else {
				put(i, table[data[i]])
			}
		}
	} else {
		for i := 0; i < size; i++ {
			put(i, table[data[i]])
			if data[i] == 255 {
				has_alpha = 1
			}
		}
		if mode&TEX_ALPHA != 0 && has_alpha == 0 {
			mode &^= TEX_ALPHA
		}
	}

	R_Upload32(buf, width, height, mode)
}

func find_texture(identifier string) *gltexture_t {
	for i := 0; i < numgltextures; i++ {
		if strlcpy(identifier, 64) == gltextures[i].identifier {
			return &gltextures[i]
		}
	}
	return nil
}

func R_LoadTexture(identifier string, width, height int, data []byte, mode int, bpp int) int {
	var glt *gltexture_t
	var scaled_width, scaled_height int
	var crc uint32

	scale_dimensions(width, height, &scaled_width, &scaled_height, mode)

	if identifier != "" {
		crc = uint32(CRC_Block(data[:width*height*bpp]))
		glt = find_texture(identifier)
		if glt != nil {
			if glt.width == width && glt.height == height &&
				glt.scaled_width == scaled_width && glt.scaled_height == scaled_height &&
				glt.crc == crc && glt.bpp == bpp &&
				(glt.texmode&^(TEX_COMPLAIN|TEX_NOSCALE)) == (mode&^(TEX_COMPLAIN|TEX_NOSCALE)) {
				R_Bind(glt.texnum)
				return glt.texnum
			}
			// same name, different content: re-upload into the same slot
		}
	}

	if glt == nil {
		if numgltextures >= MAX_GLTEXTURES {
			// cache only dedups re-uploads, dropping it costs re-upload not correctness
			Com_Printf("R_LoadTexture: identifier cache full, resetting\n")
			numgltextures = 0
		}
		glt = &gltextures[numgltextures]
		numgltextures++
		*glt = gltexture_t{}
		glt.identifier = strlcpy(identifier, 64)
		glt.texnum = texture_extension_number
		texture_extension_number++
	}

	glt.width = width
	glt.height = height
	glt.scaled_width = scaled_width
	glt.scaled_height = scaled_height
	glt.texmode = mode
	glt.crc = crc
	glt.bpp = bpp

	R_Bind(glt.texnum)
	if bpp == 1 {
		R_Upload8(data, width, height, mode)
	} else if bpp == 4 {
		R_Upload32(data, width, height, mode)
	} else {
		Sys_Error("R_LoadTexture: unsupported bpp %d", bpp)
	}

	return glt.texnum
}

func R_LoadImagePixels(filename string, matchwidth, matchheight int, imagewidth, imageheight *uint, mode int) []byte {
	basename := []byte(COM_CopyAndStripExtension(filename, MAX_QPATH))
	for i := range basename {
		if basename[i] == '*' {
			basename[i] = '#'
		}
	}

	name := strlcpy(string(basename)+".tga", MAX_QPATH)
	pixels := Image_LoadTGA(nil, name, matchwidth, matchheight, imagewidth, imageheight)
	if pixels != nil {
		return pixels
	}

	name = strlcpy(string(basename)+".png", MAX_QPATH)
	pixels = Image_LoadPNG(nil, name, matchwidth, matchheight, imagewidth, imageheight)
	if pixels != nil {
		return pixels
	}

	if mode&TEX_COMPLAIN != 0 {
		Com_Printf("Couldn't load %s image\n", COM_SkipPath(filename))
	}

	return nil
}

func R_LoadTexturePixels(data []byte, identifier string, width, height int, mode int) int {
	image_size := width * height

	if mode&TEX_LUMA == 0 && vid_gamma != 1 {
		for i := 0; i < image_size; i++ {
			data[4*i+0] = vid_gamma_table[data[4*i+0]]
			data[4*i+1] = vid_gamma_table[data[4*i+1]]
			data[4*i+2] = vid_gamma_table[data[4*i+2]]
		}
	}

	if mode&TEX_ALPHA != 0 {
		mode &^= TEX_ALPHA
		for j := 0; j < image_size; j++ {
			if data[4*j+3] < 255 {
				mode |= TEX_ALPHA
				break
			}
		}
	}

	return R_LoadTexture(identifier, width, height, data, mode, 4)
}

func R_LoadTextureImage(filename, identifier string, matchwidth, matchheight, mode int) int {
	var w, h uint

	if identifier == "" {
		identifier = filename
	}

	pixels := R_LoadImagePixels(filename, matchwidth, matchheight, &w, &h, mode)
	if pixels == nil {
		return 0
	}

	return R_LoadTexturePixels(pixels, identifier, int(w), int(h), mode)
}

func R_LoadCharsetImage(filename, identifier string) int {
	var w, h uint

	pixels := R_LoadImagePixels(filename, 0, 0, &w, &h, 0)
	if pixels == nil {
		return 0
	}

	if w >= 32768 || h >= 32768 || w*h >= (1<<28) {
		return 0
	}

	if identifier == "" {
		identifier = filename
	}

	image_size := int(w * h)

	// blank rows between character rows keep LINEAR from bleeding
	buf := make([]byte, image_size*2*4)

	src := 0
	dest := 0
	for i := 0; i < 16; i++ {
		copy(buf[dest:dest+image_size>>2], pixels[src:src+image_size>>2])
		src += image_size >> 2
		dest += image_size >> 1
	}

	return R_LoadTexture(identifier, int(w), int(h*2), buf, TEX_ALPHA|TEX_NOCOMPRESS, 4)
}

func R_Texture_CvarInit() {
	Cvar_SetCurrentGroup(CVAR_GROUP_TEXTURES)
	Cvar_Register(&gl_picmip)
	Cvar_Register(&gl_lerpimages)
	Cvar_Register(&gl_texturemode)
	Cvar_ResetCurrentGroup()
}

func R_Texture_Init() {
	numgltextures = 0
}

func R_Texture_Shutdown() {
	numgltextures = 0
}

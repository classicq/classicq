package engine

import (
	"math"
	"strings"
	"unsafe"

	"github.com/classicq/classicq/internal/sdl"
)

// gpu_world.c: world, brush model, lightmap, sky and water rendering for the SDL_GPU path
// ported from gl_rsurf.c and gl_warp.c

const (
	BLOCK_WIDTH       = 128
	BLOCK_HEIGHT      = 128
	MAX_LIGHTMAP_SIZE = 32 * 32
	MAX_LIGHTMAPS     = 512

	BACKFACE_EPSILON = 0.01
)

var blocklights [MAX_LIGHTMAP_SIZE * 3]uint32

type glRect_t struct {
	l, t, w, h byte
}

var lightmap_modified [MAX_LIGHTMAPS]bool
var lightmap_rectchange [MAX_LIGHTMAPS]glRect_t
var lightmap_pages [MAX_LIGHTMAPS]*sdl.GPUTexture
var lightmap_transfer *sdl.GPUTransferBuffer
var allocated [MAX_LIGHTMAPS][BLOCK_WIDTH]int

// CPU copy, RGBA so uploads match the page format
var lightmaps [4 * MAX_LIGHTMAPS * BLOCK_WIDTH * BLOCK_HEIGHT]byte

var skychain *msurface_t
var skychain_tail = &skychain
var waterchain *msurface_t
var waterchain_tail = &waterchain
var alphachain *msurface_t
var drawflatchain *msurface_t
var drawflatchain_tail = &drawflatchain

// CHAIN_SURF_F2B and CHAIN_SURF_B2F from the C source
func chainSurfF2B(surf *msurface_t, chain_tail ***msurface_t) {
	**chain_tail = surf
	*chain_tail = &surf.texturechain
	surf.texturechain = nil
}

func chainSurfB2F(surf *msurface_t, chain **msurface_t) {
	surf.texturechain = *chain
	*chain = surf
}

var fullbright_polys [MAX_GLTEXTURES]*glpoly_t
var fullbright_polys_used [(MAX_GLTEXTURES + 31) / 32]uint32
var luma_polys [MAX_GLTEXTURES]*glpoly_t
var luma_polys_used [(MAX_GLTEXTURES + 31) / 32]uint32
var drawfullbrights, drawlumas bool

// ---- per-model static vertex buffers ----

type worldvb_t struct {
	surfaces *msurface_t // key, shared with inline '*' submodels
	buf      *sdl.GPUBuffer
	verts    []scene_vert_t // CPU copy kept for drawflat colour rebuilds
	numverts uint32
	litverts uint32
}

var worldvbs [MAX_MODELS]worldvb_t
var num_worldvbs int

func surfacesKey(model *model_t) *msurface_t {
	if len(model.surfaces) == 0 {
		return nil
	}
	return &model.surfaces[0]
}

func World_FindVB(model *model_t) *worldvb_t {
	key := surfacesKey(model)
	for i := 0; i < num_worldvbs; i++ {
		if worldvbs[i].surfaces == key {
			return &worldvbs[i]
		}
	}
	return nil
}

// mvp and vertex buffer of the entity whose chains are being drawn
var brush_mvp [16]float32
var current_mvp = r_viewproj[:]
var current_vb *worldvb_t

// drops all world GPU objects; also runs on vid_restart while the old device is alive
func World_Shutdown() {
	for i := 0; i < num_worldvbs; i++ {
		GPU_ReleaseBuffer(worldvbs[i].buf)
	}
	worldvbs = [MAX_MODELS]worldvb_t{}
	num_worldvbs = 0
	current_vb = nil

	for i := 0; i < MAX_LIGHTMAPS; i++ {
		if lightmap_pages[i] != nil {
			GPU_ReleaseTexture(lightmap_pages[i])
			lightmap_pages[i] = nil
		}
		lightmap_modified[i] = false
	}
}

// triangle fan over a contiguous vertex range, returns index count
func Fan_AllocIndices(firstvert uint32, numverts int, firstindex *uint32) uint32 {
	if numverts < 3 {
		return 0
	}
	n := uint32(numverts-2) * 3
	idx := Scene_AllocIndices(n, firstindex)
	if idx == nil {
		return 0
	}
	for k := uint32(0); k < uint32(numverts)-2; k++ {
		idx[k*3+0] = firstvert
		idx[k*3+1] = firstvert + k + 1
		idx[k*3+2] = firstvert + k + 2
	}
	return n
}

// streamed flat-coloured fan from a surface's fastpolys (fastsky, fastturb)
func EmitFastPolyColoured(fa *msurface_t, rgb []byte) {
	var firstvert, firstindex uint32

	if fa.fastpolys == nil || fa.numedges < 3 {
		return
	}
	v := Scene_AllocVerts(uint32(fa.numedges), &firstvert)
	if v == nil {
		return
	}
	for i := 0; i < fa.numedges; i++ {
		v[i].pos[0] = fa.fastpolys[i*3+0]
		v[i].pos[1] = fa.fastpolys[i*3+1]
		v[i].pos[2] = fa.fastpolys[i*3+2]
		v[i].st[0], v[i].st[1] = 0, 0
		v[i].lm[0], v[i].lm[1] = 0, 0
		v[i].rgba[0] = rgb[0]
		v[i].rgba[1] = rgb[1]
		v[i].rgba[2] = rgb[2]
		v[i].rgba[3] = 255
	}
	n := Fan_AllocIndices(firstvert, fa.numedges, &firstindex)
	if n != 0 {
		Scene_AddBatch(SCENE_PIPE_TEX, GPU_Texture_White(), nil, GPU_GetDynamicSceneVB(), firstindex, n, current_mvp)
	}
}

// ---- dynamic lights ----

type dlightinfo_t struct {
	local    [2]int
	rad      int
	minlight int // rad - minlight
	type_    int
}

var dlightlist [MAX_DLIGHTS]dlightinfo_t

func R_BuildDlightList(surf *msurface_t) int {
	var impact vec3_t
	var local [2]int
	var tdmin, sdmin, distmin int

	numdlights := 0

	smax := (int(surf.extents[0]) >> 4) + 1
	tmax := (int(surf.extents[1]) >> 4) + 1
	tex := surf.texinfo

	dlightbits := uint32(surf.dlightbits)

	for lnum := 0; lnum < MAX_DLIGHTS && dlightbits != 0; lnum++ {
		if dlightbits&(uint32(1)<<uint(lnum)) == 0 {
			continue // not lit by this light
		}

		dlightbits &^= uint32(1) << uint(lnum)

		dist := PlaneDiff(cl_dlights[lnum].origin, surf.plane)
		irad := int((float64(cl_dlights[lnum].radius) - math.Abs(float64(dist))) * 256)
		iminlight := int(cl_dlights[lnum].minlight * 256)
		if irad < iminlight {
			continue
		}

		iminlight = irad - iminlight

		for i := 0; i < 3; i++ {
			impact[i] = cl_dlights[lnum].origin[i] - surf.plane.normal[i]*dist
		}

		local[0] = int(DotProduct(impact, texVec3(&tex.vecs[0])) + tex.vecs[0][3] - float32(surf.texturemins[0]))
		local[1] = int(DotProduct(impact, texVec3(&tex.vecs[1])) + tex.vecs[1][3] - float32(surf.texturemins[1]))

		// check if this dlight will touch the surface
		if local[1] > 0 {
			tdmin = local[1] - (tmax << 4)
			if tdmin < 0 {
				tdmin = 0
			}
		} else {
			tdmin = -local[1]
		}

		if local[0] > 0 {
			sdmin = local[0] - (smax << 4)
			if sdmin < 0 {
				sdmin = 0
			}
		} else {
			sdmin = -local[0]
		}

		if sdmin > tdmin {
			distmin = (sdmin << 8) + (tdmin << 7)
		} else {
			distmin = (tdmin << 8) + (sdmin << 7)
		}

		if distmin < iminlight {
			// save dlight info
			light := &dlightlist[numdlights]
			light.minlight = iminlight
			light.rad = irad
			light.local[0] = local[0]
			light.local[1] = local[1]
			light.type_ = cl_dlights[lnum].type_
			numdlights++
		}
	}

	return numdlights
}

var dlightcolor = [NUM_DLIGHTTYPES][3]int{
	{100, 90, 80},   // dimlight or brightlight
	{100, 50, 10},   // muzzleflash
	{100, 50, 10},   // explosion
	{90, 60, 7},     // rocket
	{128, 0, 0},     // red
	{0, 0, 128},     // blue
	{128, 0, 128},   // red + blue
	{0, 128, 0},     // green
	{128, 128, 128}, // white
}

// R_BuildDlightList must be called first!
func R_AddDynamicLights(surf *msurface_t, numdlights int) {
	var color [3]int
	var idist int

	smax := (int(surf.extents[0]) >> 4) + 1
	tmax := (int(surf.extents[1]) >> 4) + 1

	for i := 0; i < numdlights; i++ {
		light := &dlightlist[i]
		if gl_colorlights.value != 0 {
			color = dlightcolor[light.type_]
		} else {
			color = [3]int{128, 128, 128}
		}

		irad := light.rad
		iminlight := light.minlight

		_td := light.local[1]
		dest := 0
		for t := 0; t < tmax; t++ {
			td := _td
			if td < 0 {
				td = -td
			}
			_td -= 16
			_sd := light.local[0]

			for s := 0; s < smax; s++ {
				sd := _sd
				if _sd < 0 {
					sd = -_sd
				}
				_sd -= 16
				if sd > td {
					idist = (sd << 8) + (td << 7)
				} else {
					idist = (td << 8) + (sd << 7)
				}
				if idist < iminlight {
					tmp := (irad - idist) >> 7
					blocklights[dest+0] += uint32(tmp * color[0])
					blocklights[dest+1] += uint32(tmp * color[1])
					blocklights[dest+2] += uint32(tmp * color[2])
				}
				dest += 3
			}
		}
	}
}

func AddAllLightMaps(lightmap []byte, surf *msurface_t, blocksize int) {
	// add all the lightmaps
	if lightmap != nil {
		for maps := 0; maps < MAXLIGHTMAPS && surf.styles[maps] != 255; maps++ {
			scale := uint32(d_lightstylevalue[surf.styles[maps]])
			surf.cached_light[maps] = int(scale) // 8.8 fraction
			for i := 0; i < blocksize; i++ {
				blocklights[i] += uint32(lightmap[i]) * scale
			}
			lightmap = lightmap[blocksize:] // skip to next lightmap
		}
	}
}

func lightmapstore_mode2(stride, smax, tmax int, dest []byte) {
	bl := 0
	d := 0
	for i := 0; i < tmax; i++ {
		for j := smax; j != 0; j-- {
			t := int(blocklights[bl+0])
			t = (t >> 8) + (t >> 9)
			if t > 255 {
				t = 255
			}
			dest[d+0] = byte(t)
			t = int(blocklights[bl+1])
			t = (t >> 8) + (t >> 9)
			if t > 255 {
				t = 255
			}
			dest[d+1] = byte(t)
			t = int(blocklights[bl+2])
			t = (t >> 8) + (t >> 9)
			if t > 255 {
				t = 255
			}
			dest[d+2] = byte(t)
			dest[d+3] = 255
			bl += 3
			d += 4
		}
		d += stride
	}
}

func lightmapstore_mode0(stride, smax, tmax int, dest []byte) {
	bl := 0
	d := 0
	for i := 0; i < tmax; i++ {
		for j := smax; j != 0; j-- {
			t := int(blocklights[bl+0])
			t = t >> 7
			if t > 255 {
				t = 255
			}
			dest[d+0] = byte(t)
			t = int(blocklights[bl+1])
			t = t >> 7
			if t > 255 {
				t = 255
			}
			dest[d+1] = byte(t)
			t = int(blocklights[bl+2])
			t = t >> 7
			if t > 255 {
				t = 255
			}
			dest[d+2] = byte(t)
			dest[d+3] = 255
			bl += 3
			d += 4
		}
		d += stride
	}
}

func StoreLightMap(stride, smax, tmax int, dest []byte) {
	if lightmode == 2 {
		lightmapstore_mode2(stride, smax, tmax, dest)
	} else {
		lightmapstore_mode0(stride, smax, tmax, dest)
	}
}

// Combine and scale multiple lightmaps into the 8.8 format in blocklights
func R_BuildLightMap(surf *msurface_t, dest []byte, stride int, numdlights int) {
	surf.cached_dlight = numdlights != 0

	smax := (int(surf.extents[0]) >> 4) + 1
	tmax := (int(surf.extents[1]) >> 4) + 1
	size := smax * tmax
	stride -= smax * 4
	blocksize := size * 3
	lightmap := surf.samples

	// set to full bright if no light data
	if cl.worldmodel.lightdata == nil {
		for i := 0; i < blocksize; i++ {
			blocklights[i] = 255 << 8
		}
	} else {
		// clear to no light
		for i := 0; i < blocksize; i++ {
			blocklights[i] = 0
		}

		AddAllLightMaps(lightmap, surf, blocksize)

		// add all the dynamic lights
		if numdlights != 0 {
			R_AddDynamicLights(surf, numdlights)
		}
	}

	// bound and shift
	StoreLightMap(stride, smax, tmax, dest)
}

// Returns the proper texture for a given time and base texture
func R_TextureAnimation(base *texture_t) *texture_t {
	if currententity.frame != 0 {
		if base.alternate_anims != nil {
			base = base.alternate_anims
		}
	}

	if base.anim_total == 0 {
		return base
	}

	relative := int(cl.time*10) % base.anim_total

	count := 0
	for base.anim_min > relative || base.anim_max <= relative {
		base = base.anim_next
		if base == nil {
			Host_Error("R_TextureAnimation: broken cycle")
		}
		count++
		if count > 100 {
			Host_Error("R_TextureAnimation: infinite cycle")
		}
	}

	return base
}

func R_RenderDynamicLightmaps(fa *msurface_t) {
	lightstyle_modified := false
	var numdlights int

	c_brush_polys++

	if r_dynamic.value == 0 && !fa.cached_dlight {
		return
	}

	// check for lightmap modification
	for maps := 0; maps < MAXLIGHTMAPS && fa.styles[maps] != 255; maps++ {
		if d_lightstylevalue[fa.styles[maps]] != fa.cached_light[maps] {
			lightstyle_modified = true
			break
		}
	}

	if r_dynamic.value != 0 {
		if fa.dlightframe == r_framecount {
			numdlights = R_BuildDlightList(fa)
		} else {
			numdlights = 0
		}

		if numdlights == 0 && !fa.cached_dlight && !lightstyle_modified {
			return
		}
	} else {
		numdlights = 0
	}

	lightmap_modified[fa.lightmaptexturenum] = true
	theRect := &lightmap_rectchange[fa.lightmaptexturenum]
	if fa.light_t < int(theRect.t) {
		if theRect.h != 0 {
			theRect.h = byte(int(theRect.h) + int(theRect.t) - fa.light_t)
		}
		theRect.t = byte(fa.light_t)
	}
	if fa.light_s < int(theRect.l) {
		if theRect.w != 0 {
			theRect.w = byte(int(theRect.w) + int(theRect.l) - fa.light_s)
		}
		theRect.l = byte(fa.light_s)
	}
	smax := (int(fa.extents[0]) >> 4) + 1
	tmax := (int(fa.extents[1]) >> 4) + 1
	if int(theRect.w)+int(theRect.l) < fa.light_s+smax {
		theRect.w = byte(fa.light_s - int(theRect.l) + smax)
	}
	if int(theRect.h)+int(theRect.t) < fa.light_t+tmax {
		theRect.h = byte(fa.light_t - int(theRect.t) + tmax)
	}
	base := fa.lightmaptexturenum * BLOCK_WIDTH * BLOCK_HEIGHT * 4
	base += (fa.light_t*BLOCK_WIDTH + fa.light_s) * 4
	R_BuildLightMap(fa, lightmaps[base:], BLOCK_WIDTH*4, numdlights)
}

// uploads happen later in World_UploadLightmaps, only the CPU copy is touched here
func R_RenderAllDynamicLightmaps(model *model_t) {
	for i := 0; i < model.numtextures; i++ {
		if model.textures[i] == nil || (model.textures[i].texturechain[0] == nil && model.textures[i].texturechain[1] == nil) {
			continue
		}

		for waterline := 0; waterline < 2; waterline++ {
			for s := model.textures[i].texturechain[waterline]; s != nil; s = s.texturechain {
				R_RenderDynamicLightmaps(s)
			}
		}
	}

	for s := drawflatchain; s != nil; s = s.texturechain {
		R_RenderDynamicLightmaps(s)
	}
}

// copy-pass hook, sends dirty page regions to the GPU
func World_UploadLightmaps(copyPass *sdl.GPUCopyPass) {
	var transfer sdl.GPUTextureTransferInfo
	var region sdl.GPUTextureRegion

	device := GPU_GetDevice()
	if device == nil || copyPass == nil {
		return
	}

	for i := 0; i < MAX_LIGHTMAPS; i++ {
		if lightmap_pages[i] == nil || !lightmap_modified[i] {
			continue
		}
		rect := &lightmap_rectchange[i]
		if rect.h == 0 {
			continue
		}

		if lightmap_transfer == nil {
			var tci sdl.GPUTransferBufferCreateInfo
			tci.Usage = sdl.GPU_TRANSFERBUFFERUSAGE_UPLOAD
			tci.Size = BLOCK_WIDTH * BLOCK_HEIGHT * 4
			lightmap_transfer = sdl.CreateGPUTransferBuffer(device, &tci)
			if lightmap_transfer == nil {
				return
			}
		}

		// full-width rows of the dirty rect, buffer cycled per page
		mapped := sdl.MapGPUTransferBuffer(device, lightmap_transfer, true)
		if mapped == nil {
			return
		}
		n := int(rect.h) * BLOCK_WIDTH * 4
		src := ((i * BLOCK_HEIGHT) + int(rect.t)) * BLOCK_WIDTH * 4
		copy(unsafe.Slice((*byte)(mapped), n), lightmaps[src:src+n])
		sdl.UnmapGPUTransferBuffer(device, lightmap_transfer)

		transfer = sdl.GPUTextureTransferInfo{}
		transfer.TransferBuffer = lightmap_transfer
		transfer.PixelsPerRow = BLOCK_WIDTH

		region = sdl.GPUTextureRegion{}
		region.Texture = lightmap_pages[i]
		region.Y = uint32(rect.t)
		region.W = BLOCK_WIDTH
		region.H = uint32(rect.h)
		region.D = 1

		sdl.UploadToGPUTexture(copyPass, &transfer, &region, false)

		lightmap_modified[i] = false
		rect.l = BLOCK_WIDTH
		rect.t = BLOCK_HEIGHT
		rect.w = 0
		rect.h = 0
	}
}

// ---- fullbright and luma passes ----

func R_RenderPolyBuckets(polys []*glpoly_t, used []uint32, pipe int, next func(*glpoly_t) *glpoly_t) {
	var firstindex uint32

	for j := 0; j < (MAX_GLTEXTURES+31)/32; j++ {
		if used[j] == 0 {
			continue
		}
		for i := 0; i < 32; i++ {
			if used[j]&(1<<uint(i)) == 0 {
				continue
			}
			runcount := uint32(0)
			runfirst := uint32(0)
			for p := polys[j*32+i]; p != nil; p = next(p) {
				n := Fan_AllocIndices(uint32(p.firstindex), p.numverts, &firstindex)
				if n == 0 {
					continue
				}
				if runcount == 0 {
					runfirst = firstindex
				}
				runcount += n
			}
			if runcount != 0 {
				Scene_AddBatch(pipe, j*32+i, nil, current_vb.buf, runfirst, runcount, current_mvp)
			}
		}
		used[j] = 0
	}
}

func next_fb(p *glpoly_t) *glpoly_t   { return p.fb_chain }
func next_luma(p *glpoly_t) *glpoly_t { return p.luma_chain }

func R_RenderFullbrights() {
	if !drawfullbrights {
		return
	}
	R_RenderPolyBuckets(fullbright_polys[:], fullbright_polys_used[:], SCENE_PIPE_TEX_ALPHATEST_NODEPTHWRITE, next_fb)
	drawfullbrights = false
}

func R_RenderLumas() {
	if !drawlumas {
		return
	}
	R_RenderPolyBuckets(luma_polys[:], luma_polys_used[:], SCENE_PIPE_ADD_NODEPTHWRITE, next_luma)
	drawlumas = false
}

// ---- caustics and detail decal passes ----

var caustics_polys *glpoly_t
var detail_polys *glpoly_t

const TURBSINSIZE = 128

var turbsin = [TURBSINSIZE]byte{
	127, 133, 139, 146, 152, 158, 164, 170, 176, 182, 187, 193, 198, 203, 208, 213,
	217, 221, 226, 229, 233, 236, 239, 242, 245, 247, 249, 251, 252, 253, 254, 254,
	255, 254, 254, 253, 252, 251, 249, 247, 245, 242, 239, 236, 233, 229, 226, 221,
	217, 213, 208, 203, 198, 193, 187, 182, 176, 170, 164, 158, 152, 146, 139, 133,
	127, 121, 115, 108, 102, 96, 90, 84, 78, 72, 67, 61, 56, 51, 46, 41,
	37, 33, 28, 25, 21, 18, 15, 12, 9, 7, 5, 3, 2, 1, 0, 0,
	0, 0, 0, 1, 2, 3, 5, 7, 9, 12, 15, 18, 21, 25, 28, 33,
	37, 41, 46, 51, 56, 61, 67, 72, 78, 84, 90, 96, 102, 108, 115, 121,
}

func SINTABLE_APPROX(time float32) float32 {
	// TURBSCALE is a double in C, (float)TURBSINSIZE / (2 * M_PI)
	sinlerpf := float32(float64(time) * (float64(TURBSINSIZE) / (2 * float64(math.Pi))))
	sinlerp1 := int(math.Floor(float64(sinlerpf)))
	sinlerp2 := sinlerp1 + 1
	lerptime := sinlerpf - float32(sinlerp1)

	lerp := float32(turbsin[sinlerp1&(TURBSINSIZE-1)])*(1-lerptime) +
		float32(turbsin[sinlerp2&(TURBSINSIZE-1)])*lerptime
	return float32(-8 + float64(16*lerp)/255.0)
}

func CalcCausticTexCoords(v []float32, s, t *float32) {
	os := v[3]
	ot := v[4]

	*s = os + SINTABLE_APPROX(float32(0.465*(cl.time+float64(ot))))
	*s = float32(float64(*s) * (-3 * (0.5 / 64)))

	*t = ot + SINTABLE_APPROX(float32(0.465*(cl.time+float64(os))))
	*t = float32(float64(*t) * (-3 * (0.5 / 64)))
}

// emits one decal batch over the chained polys with per-frame CPU texcoords
func EmitDecalPolys(chain *glpoly_t, is_caustics int, texnum int) {
	var firstvert, firstindex uint32
	var batchfirst, total uint32
	var s, t float32

	for p := chain; p != nil; {
		if p.numverts < 3 {
			if is_caustics != 0 {
				p = p.caustics_chain
			} else {
				p = p.detail_chain
			}
			continue
		}
		out := Scene_AllocVerts(uint32(p.numverts), &firstvert)
		idx := Scene_AllocIndices(uint32(p.numverts-2)*3, &firstindex)
		if out == nil || idx == nil {
			break
		}
		for i := 0; i < p.numverts; i++ {
			v := p.verts[i][:]
			if is_caustics != 0 {
				CalcCausticTexCoords(v, &s, &t)
			} else {
				s = v[7] * 18
				t = v[8] * 18
			}
			out[i].pos[0] = v[0]
			out[i].pos[1] = v[1]
			out[i].pos[2] = v[2]
			out[i].st[0] = s
			out[i].st[1] = t
			out[i].lm[0] = 0
			out[i].lm[1] = 0
			out[i].rgba[0] = 255
			out[i].rgba[1] = 255
			out[i].rgba[2] = 255
			out[i].rgba[3] = 255
		}
		for j := 0; j < p.numverts-2; j++ {
			idx[j*3+0] = firstvert
			idx[j*3+1] = firstvert + uint32(j) + 1
			idx[j*3+2] = firstvert + uint32(j) + 2
		}
		if total == 0 {
			batchfirst = firstindex
		}
		total += uint32(p.numverts-2) * 3

		if is_caustics != 0 {
			p = p.caustics_chain
		} else {
			p = p.detail_chain
		}
	}

	if total != 0 {
		Scene_AddBatch(SCENE_PIPE_MOD_NODEPTHWRITE, texnum, nil,
			GPU_GetDynamicSceneVB(), batchfirst, total, current_mvp)
	}
}

func EmitCausticsPolys() {
	if underwatertexture != 0 && caustics_polys != nil {
		EmitDecalPolys(caustics_polys, 1, underwatertexture)
	}
	caustics_polys = nil
}

func EmitDetailPolys() {
	if detailtexture != 0 && detail_polys != nil {
		EmitDecalPolys(detail_polys, 0, detailtexture)
	}
	detail_polys = nil
}

func R_InitOtherTextures() {
	const flags = TEX_MIPMAP | TEX_ALPHA | TEX_COMPLAIN

	underwatertexture = R_LoadTextureImage("textures/water_caustic", "", 0, 0, flags)
	detailtexture = R_LoadTextureImage("textures/detail", "", 256, 256, flags)
}

// ---- texture chains ----

func R_ClearTextureChains(clmodel *model_t) {
	fullbright_polys_used = [(MAX_GLTEXTURES + 31) / 32]uint32{}
	luma_polys_used = [(MAX_GLTEXTURES + 31) / 32]uint32{}

	for i := 0; i < clmodel.numtextures; i++ {
		if texture := clmodel.textures[i]; texture != nil {
			texture.texturechain[0] = nil
			texture.texturechain[1] = nil
			texture.texturechain_tail[0] = &texture.texturechain[0]
			texture.texturechain_tail[1] = &texture.texturechain[1]
		}
	}

	if r_notexture_mip != nil {
		r_notexture_mip.texturechain[0] = nil
		r_notexture_mip.texturechain_tail[0] = &r_notexture_mip.texturechain[0]
		r_notexture_mip.texturechain[1] = nil
		r_notexture_mip.texturechain_tail[1] = &r_notexture_mip.texturechain[1]
	}

	skychain = nil
	skychain_tail = &skychain
	if clmodel == cl.worldmodel {
		waterchain = nil
		waterchain_tail = &waterchain
	}
	alphachain = nil
	drawflatchain = nil
	drawflatchain_tail = &drawflatchain
}

func DrawTextureChains(model *model_t) {
	var firstindex uint32

	drawLumasGlowing := (com_serveractive || cl.allow_lumas) && gl_fb_bmodels.value != 0
	draw_caustics := underwatertexture != 0 && gl_caustics.value != 0
	draw_details := detailtexture != 0 && gl_detail.value != 0

	if current_vb == nil {
		return
	}

	for i := 0; i < model.numtextures; i++ {
		if model.textures[i] == nil || (model.textures[i].texturechain[0] == nil && model.textures[i].texturechain[1] == nil) {
			continue
		}

		t := R_TextureAnimation(model.textures[i])

		draw_fbs := t.isLumaTexture != 0 || gl_fb_bmodels.value != 0

		runcount := uint32(0)
		runfirst := uint32(0)
		runpage := -1

		for waterline := 0; waterline < 2; waterline++ {
			for s := model.textures[i].texturechain[waterline]; s != nil; s = s.texturechain {
				if s.polys == nil {
					continue
				}

				// batch breaks on lightmap page change
				if runcount != 0 && s.lightmaptexturenum != runpage {
					Scene_AddBatch(SCENE_PIPE_WORLD, t.gl_texturenum, lightmap_pages[runpage],
						current_vb.buf, runfirst, runcount, current_mvp)
					runcount = 0
				}

				n := Fan_AllocIndices(uint32(s.polys.firstindex), s.polys.numverts, &firstindex)
				if n != 0 {
					if runcount == 0 {
						runfirst = firstindex
						runpage = s.lightmaptexturenum
					}
					runcount += n
				}

				if waterline != 0 && draw_caustics {
					s.polys.caustics_chain = caustics_polys
					caustics_polys = s.polys
				}

				if waterline == 0 && draw_details {
					s.polys.detail_chain = detail_polys
					detail_polys = s.polys
				}

				if t.fb_texturenum > 0 && t.fb_texturenum < MAX_GLTEXTURES && draw_fbs {
					bit := uint32(1) << uint(t.fb_texturenum%32)
					if t.isLumaTexture != 0 {
						if luma_polys_used[t.fb_texturenum/32]&bit != 0 {
							s.polys.luma_chain = luma_polys[t.fb_texturenum]
						} else {
							s.polys.luma_chain = nil
						}
						luma_polys[t.fb_texturenum] = s.polys
						if luma_polys_used[t.fb_texturenum/32]&bit == 0 {
							luma_polys_used[t.fb_texturenum/32] |= bit
							drawlumas = true
						}
					} else {
						if fullbright_polys_used[t.fb_texturenum/32]&bit != 0 {
							s.polys.fb_chain = fullbright_polys[t.fb_texturenum]
						} else {
							s.polys.fb_chain = nil
						}
						fullbright_polys[t.fb_texturenum] = s.polys
						if fullbright_polys_used[t.fb_texturenum/32]&bit == 0 {
							fullbright_polys_used[t.fb_texturenum/32] |= bit
							drawfullbrights = true
						}
					}
				}
			}
		}

		if runcount != 0 {
			Scene_AddBatch(SCENE_PIPE_WORLD, t.gl_texturenum, lightmap_pages[runpage],
				current_vb.buf, runfirst, runcount, current_mvp)
		}
	}

	if drawLumasGlowing {
		R_RenderFullbrights()
		R_RenderLumas()
	} else {
		R_RenderLumas()
		R_RenderFullbrights()
	}

	EmitCausticsPolys()
	EmitDetailPolys()
}

func R_UpdateFlatColours(model *model_t) {
	if !model.surface_colours_dirty || model.vertcolours == nil {
		return
	}

	vb := World_FindVB(model)
	if vb == nil || vb.verts == nil {
		return
	}

	for i := uint32(0); i < vb.litverts; i++ {
		vb.verts[i].rgba[0] = byte(bound(float32(0), model.vertcolours[i*3+0], 1) * 255)
		vb.verts[i].rgba[1] = byte(bound(float32(0), model.vertcolours[i*3+1], 1) * 255)
		vb.verts[i].rgba[2] = byte(bound(float32(0), model.vertcolours[i*3+2], 1) * 255)
		vb.verts[i].rgba[3] = 255
	}

	// rare, a full rebuild keeps the buffer static
	newbuf := GPU_CreateStaticVertexBuffer(vb.verts)
	if newbuf != nil {
		GPU_ReleaseBuffer(vb.buf)
		vb.buf = newbuf
	}

	model.surface_colours_dirty = false
}

func R_DrawFlat(model *model_t) {
	var firstindex uint32

	if r_drawflat_enable.value == 0 {
		return
	}

	if current_vb == nil {
		return
	}

	runcount := uint32(0)
	runfirst := uint32(0)
	runpage := -1

	for s := drawflatchain; s != nil; s = s.texturechain {
		if s.polys == nil {
			continue
		}

		if runcount != 0 && s.lightmaptexturenum != runpage {
			Scene_AddBatch(SCENE_PIPE_WORLD, GPU_Texture_White(), lightmap_pages[runpage],
				current_vb.buf, runfirst, runcount, current_mvp)
			runcount = 0
		}

		n := Fan_AllocIndices(uint32(s.polys.firstindex), s.polys.numverts, &firstindex)
		if n != 0 {
			if runcount == 0 {
				runfirst = firstindex
				runpage = s.lightmaptexturenum
			}
			runcount += n
		}
	}

	if runcount != 0 {
		Scene_AddBatch(SCENE_PIPE_WORLD, GPU_Texture_White(), lightmap_pages[runpage],
			current_vb.buf, runfirst, runcount, current_mvp)
	}
}

// draws transparent textures for HL world and nonworld models
func R_DrawAlphaChain() {
	var firstindex uint32

	if alphachain == nil {
		return
	}

	// back to front, one batch per surface preserves the order
	for s := alphachain; s != nil; s = s.texturechain {
		t := s.texinfo.texture
		R_RenderDynamicLightmaps(s)

		if s.polys == nil || current_vb == nil {
			continue
		}
		n := Fan_AllocIndices(uint32(s.polys.firstindex), s.polys.numverts, &firstindex)
		if n != 0 {
			Scene_AddBatch(SCENE_PIPE_WORLD_ALPHATEST, t.gl_texturenum, lightmap_pages[s.lightmaptexturenum],
				current_vb.buf, firstindex, n, current_mvp)
		}
	}

	alphachain = nil
}

// ---- water ----

// Emits a water batch from the pre-built fastpoly range, shader does the warp
func EmitWaterPolys(model *model_t, fa *msurface_t) {
	var firstindex uint32

	if r_fastturb.value != 0 {
		c := fa.texinfo.texture.colour
		EmitFastPolyColoured(fa, []byte{byte(c), byte(c >> 8), byte(c >> 16), byte(c >> 24)})
		return
	}

	vb := World_FindVB(model)
	if vb == nil || fa.fastpolys == nil {
		return
	}

	n := Fan_AllocIndices(fa.fastpolyfirstindex, fa.numedges, &firstindex)
	if n == 0 {
		return
	}

	b := Scene_AddBatch(SCENE_PIPE_WATER, fa.texinfo.texture.gl_texturenum, nil, vb.buf, firstindex, n, current_mvp)
	if b != nil {
		b.params[0] = float32(cl.time * (20.0 / 64.0))
	}
}

func R_DrawWaterSurfaces() {
	if waterchain == nil {
		return
	}

	// always opaque, r_wateralpha is not honoured by the SDL_GPU path
	current_mvp = r_viewproj[:]
	for s := waterchain; s != nil; s = s.texturechain {
		EmitWaterPolys(cl.worldmodel, s)
	}

	waterchain = nil
	waterchain_tail = &waterchain
}

// ---- surface subdivision (load time, called from gl_model.c) ----

func BoundPoly(numverts int, verts []float32, mins, maxs *vec3_t) {
	mins[0], mins[1], mins[2] = 9999, 9999, 9999
	maxs[0], maxs[1], maxs[2] = -9999, -9999, -9999
	v := 0
	for i := 0; i < numverts; i++ {
		for j := 0; j < 3; j++ {
			if verts[v] < mins[j] {
				mins[j] = verts[v]
			}
			if verts[v] > maxs[j] {
				maxs[j] = verts[v]
			}
			v++
		}
	}
}

// verts is a flat xyz list with room for one wrap-around vertex past numverts
func SubdividePolygon(warpface *msurface_t, numverts int, verts []float32) {
	var mins, maxs vec3_t
	var front, back [64 * 3]float32
	var dist [64]float32

	if numverts > 60 {
		Sys_Error("numverts = %d", numverts)
	}

	subdivide_size := max(float32(1), gl_subdivide_size.value)

	BoundPoly(numverts, verts, &mins, &maxs)

	for i := 0; i < 3; i++ {
		m := (mins[i] + maxs[i]) * 0.5
		m = float32(float64(subdivide_size) * math.Floor(float64(m/subdivide_size)+0.5))
		if maxs[i]-m < 8 {
			continue
		}
		if m-mins[i] < 8 {
			continue
		}

		// cut it
		var j int
		for j = 0; j < numverts; j++ {
			dist[j] = verts[j*3+i] - m
		}

		// wrap cases
		dist[j] = dist[0]
		copy(verts[numverts*3:numverts*3+3], verts[0:3])

		f, b := 0, 0
		for j = 0; j < numverts; j++ {
			v := verts[j*3:]
			if dist[j] >= 0 {
				copy(front[f*3:f*3+3], v[0:3])
				f++
			}
			if dist[j] <= 0 {
				copy(back[b*3:b*3+3], v[0:3])
				b++
			}
			if dist[j] == 0 || dist[j+1] == 0 {
				continue
			}
			if (dist[j] > 0) != (dist[j+1] > 0) {
				// clip point
				frac := dist[j] / (dist[j] - dist[j+1])
				for k := 0; k < 3; k++ {
					e := v[k] + frac*(v[3+k]-v[k])
					front[f*3+k] = e
					back[b*3+k] = e
				}
				f++
				b++
			}
		}

		SubdividePolygon(warpface, f, front[:])
		SubdividePolygon(warpface, b, back[:])
		return
	}

	poly := &glwarppoly{numverts: numverts, verts: make([][WARPVERTEXSIZE]float32, numverts)}
	poly.next = warpface.warppolys
	warpface.warppolys = poly
	for i := 0; i < numverts; i++ {
		v := vec3_t{verts[i*3+0], verts[i*3+1], verts[i*3+2]}
		poly.verts[i][0] = v[0]
		poly.verts[i][1] = v[1]
		poly.verts[i][2] = v[2]
		s := DotProduct(v, texVec3(&warpface.texinfo.vecs[0]))
		t := DotProduct(v, texVec3(&warpface.texinfo.vecs[1]))
		poly.verts[i][3] = s
		poly.verts[i][4] = t
	}
}

// Breaks a polygon up along axial 64 unit boundaries so that turbulent and sky warps can be done reasonably.
func R_SubdivideSurface(model *model_t, fa *msurface_t) {
	var verts [64 * 3]float32

	// Build simple verts for fastturb/fastsky
	fa.fastpolys = make([]float32, fa.numedges*3)
	fa.shadertexcoords = make([]float32, fa.numedges*2)

	// convert edges back to a normal polygon
	for i := 0; i < fa.numedges && i < 64; i++ {
		lindex := model.surfedges[fa.firstedge+i]

		var vec vec3_t
		if lindex > 0 {
			vec = model.vertexes[model.edges[lindex].v[0]].position
		} else {
			vec = model.vertexes[model.edges[-lindex].v[1]].position
		}
		verts[i*3+0] = vec[0]
		verts[i*3+1] = vec[1]
		verts[i*3+2] = vec[2]

		fa.fastpolys[i*3+0] = vec[0]
		fa.fastpolys[i*3+1] = vec[1]
		fa.fastpolys[i*3+2] = vec[2]

		s := DotProduct(vec, texVec3(&fa.texinfo.vecs[0]))
		t := DotProduct(vec, texVec3(&fa.texinfo.vecs[1]))

		fa.shadertexcoords[i*2+0] = s / 64
		fa.shadertexcoords[i*2+1] = t / 64
	}

	SubdividePolygon(fa, fa.numedges, verts[:])
}

// ---- sky ----

var sky_initialised bool
var solidskytexture, alphaskytexture int
var skybox_ext = [6]string{"rt", "bk", "lf", "ft", "up", "dn"}

func R_SetSky(skyname string) int {
	var data [6][]byte
	var imagewidth, imageheight uint

	for i := 0; i < 6; i++ {
		data[i] = R_LoadImagePixels(va("env/%s%s", skyname, skybox_ext[i]), 0, 0, &imagewidth, &imageheight, 0)
		if data[i] == nil {
			data[i] = R_LoadImagePixels(va("gfx/env/%s%s", skyname, skybox_ext[i]), 0, 0, &imagewidth, &imageheight, 0)
		}
		if data[i] == nil {
			data[i] = R_LoadImagePixels(va("env/%s_%s", skyname, skybox_ext[i]), 0, 0, &imagewidth, &imageheight, 0)
		}
		if data[i] == nil {
			data[i] = R_LoadImagePixels(va("gfx/env/%s_%s", skyname, skybox_ext[i]), 0, 0, &imagewidth, &imageheight, 0)
		}
		if data[i] == nil {
			Com_Printf("Couldn't load skybox \"%s\"\n", skyname)
			return 1
		}
	}

	// the GL renderer reserved these at startup, here they are claimed on first use
	if skyboxtextures == 0 {
		skyboxtextures = texture_extension_number
		texture_extension_number += 6
	}

	for i := 0; i < 6; i++ {
		R_Bind(skyboxtextures + i)
		R_Upload32(data[i], int(imagewidth), int(imageheight), TEX_NOCOMPRESS)
		GPU_Texture_SetPrefs(skyboxtextures+i, GPU_TEXPREF_LINEAR|GPU_TEXPREF_CLAMP)
	}

	r_skyboxloaded = true

	return 0
}

func R_SkyNameChanged(skyname string) {
	if !sky_initialised {
		return
	}

	if skyname == "" {
		r_skyboxloaded = false
		return
	}

	R_SetSky(skyname)
}

func OnChange_r_skyname(v *cvar_t, skyname string) bool {
	if !sky_initialised {
		return false
	}

	if skyname == "" {
		r_skyboxloaded = false
		return false
	}

	return R_SetSky(skyname) != 0
}

var r_skyname = cvar_t{name: "r_skyname", string: "purple_chaos"}

func init() {
	r_skyname.OnChange = OnChange_r_skyname
}

func R_LoadSky_f() {
	switch Cmd_Argc() {
	case 1:
		if r_skyboxloaded {
			Com_Printf("Current skybox is \"%s\"\n", r_skyname.string)
		} else {
			Com_Printf("No skybox has been set\n")
		}
	case 2:
		if Q_strcasecmp(Cmd_Argv(1), "none") == 0 {
			Cvar_Set(&r_skyname, "")
		} else {
			Cvar_Set(&r_skyname, Cmd_Argv(1))
		}
	default:
		Com_Printf("Usage: %s <skybox>\n", Cmd_Argv(0))
	}
}

// A sky texture is 256 * 128, with the right side being a masked overlay
func R_InitSky(texturedata []byte) {
	var trans [128 * 128]uint32

	src := texturedata

	if solidskytexture == 0 {
		solidskytexture = texture_extension_number
		texture_extension_number++
	}
	if alphaskytexture == 0 {
		alphaskytexture = texture_extension_number
		texture_extension_number++
	}

	// make an average value for the back to avoid a fringe on the top level
	r, g, b := 0, 0, 0
	for i := 0; i < 128; i++ {
		for j := 0; j < 128; j++ {
			p := int(src[i*256+j+128])
			rgba := d_8to24table[p]
			trans[(i*128)+j] = rgba
			r += int(rgba & 0xff)
			g += int((rgba >> 8) & 0xff)
			b += int((rgba >> 16) & 0xff)
		}
	}

	transpix := uint32(r/(128*128)) | uint32(g/(128*128))<<8 | uint32(b/(128*128))<<16

	transBytes := unsafe.Slice((*byte)(unsafe.Pointer(&trans[0])), 128*128*4)
	tex := GPU_CreateTextureRGBA(transBytes, 128, 128, 0)
	GPU_Texture_Set(solidskytexture, tex, 128, 128, GPU_TEXPREF_LINEAR)

	for i := 0; i < 128; i++ {
		for j := 0; j < 128; j++ {
			p := int(src[i*256+j])
			if p != 0 {
				trans[(i*128)+j] = d_8to24table[p]
			} else {
				trans[(i*128)+j] = transpix
			}
		}
	}

	tex = GPU_CreateTextureRGBA(transBytes, 128, 128, 0)
	GPU_Texture_Set(alphaskytexture, tex, 128, 128, GPU_TEXPREF_LINEAR)

	sky_initialised = true

	R_SkyNameChanged(r_skyname.string)
}

func R_DrawSkyChain() {
	var firstindex uint32

	if skychain == nil {
		return
	}

	if r_fastsky.value != 0 || cl.worldmodel.bspversion == HL_BSPVERSION {
		col := StringToRGB(r_skycolor.string)
		for fa := skychain; fa != nil; fa = fa.texturechain {
			EmitFastPolyColoured(fa, col[:])
		}
	} else if current_vb != nil {
		speedscale := float32(cl.time * 8)
		speedscale -= float32(int(speedscale) &^ 127)
		speedscale2 := float32(cl.time * 16)
		speedscale2 -= float32(int(speedscale2) &^ 127)

		alphatex := GPU_Texture_Lookup(alphaskytexture, nil)

		for fa := skychain; fa != nil; fa = fa.texturechain {
			if fa.fastpolys == nil {
				continue
			}
			n := Fan_AllocIndices(fa.fastpolyfirstindex, fa.numedges, &firstindex)
			if n == 0 {
				continue
			}
			b := Scene_AddBatch(SCENE_PIPE_SKY, solidskytexture, alphatex, current_vb.buf, firstindex, n, current_mvp)
			if b != nil {
				b.params[0] = r_origin[0]
				b.params[1] = r_origin[1]
				b.params[2] = r_origin[2]
				b.params[3] = speedscale
				b.params[4] = speedscale2
			}
		}
	}

	skychain = nil
	skychain_tail = &skychain
}

// ---- skybox ----

var skyclip = [6]vec3_t{
	{1, 1, 0},
	{1, -1, 0},
	{0, -1, 1},
	{0, 1, 1},
	{1, 0, 1},
	{-1, 0, 1},
}

// 1 = s, 2 = t, 3 = 2048
var st_to_vec = [6][3]int{
	{3, -1, 2},
	{-3, 1, 2},

	{1, 3, 2},
	{-1, -3, 2},

	{-2, -1, 3}, // 0 degrees yaw, look straight up
	{2, -1, -3}, // look straight down
}

// s = [0]/[2], t = [1]/[2]
var vec_to_st = [6][3]int{
	{-2, 3, 1},
	{2, 3, -1},

	{1, 3, 2},
	{-1, 3, -2},

	{-2, -1, 3},
	{-2, 1, -3},
}

var skymins, skymaxs [2][6]float32

func DrawSkyPolygon(nump int, vecs []float32) {
	var v, av vec3_t
	var s, t, dv float32
	var axis int

	// decide which face it maps to
	for i := 0; i < nump; i++ {
		v[0] += vecs[i*3+0]
		v[1] += vecs[i*3+1]
		v[2] += vecs[i*3+2]
	}
	av[0] = float32(math.Abs(float64(v[0])))
	av[1] = float32(math.Abs(float64(v[1])))
	av[2] = float32(math.Abs(float64(v[2])))
	if av[0] > av[1] && av[0] > av[2] {
		axis = 0
		if v[0] < 0 {
			axis = 1
		}
	} else if av[1] > av[2] && av[1] > av[0] {
		axis = 2
		if v[1] < 0 {
			axis = 3
		}
	} else {
		axis = 4
		if v[2] < 0 {
			axis = 5
		}
	}

	// project new texture coords
	for i := 0; i < nump; i++ {
		vv := vecs[i*3:]
		j := vec_to_st[axis][2]
		if j > 0 {
			dv = vv[j-1]
		} else {
			dv = -vv[-j-1]
		}

		j = vec_to_st[axis][0]
		if j < 0 {
			s = -vv[-j-1] / dv
		} else {
			s = vv[j-1] / dv
		}
		j = vec_to_st[axis][1]
		if j < 0 {
			t = -vv[-j-1] / dv
		} else {
			t = vv[j-1] / dv
		}

		if s < skymins[0][axis] {
			skymins[0][axis] = s
		}
		if t < skymins[1][axis] {
			skymins[1][axis] = t
		}
		if s > skymaxs[0][axis] {
			skymaxs[0][axis] = s
		}
		if t > skymaxs[1][axis] {
			skymaxs[1][axis] = t
		}
	}
}

const MAX_CLIP_VERTS = 64

// C keeps these on the stack; Go would zero them on every call, which is most
// of the sky cost, so each recursion stage owns one set instead
type skyClipScratch struct {
	dists [MAX_CLIP_VERTS]float32
	sides [MAX_CLIP_VERTS]int
	newv  [2][MAX_CLIP_VERTS * 3]float32
}

var sky_clip_scratch [6]skyClipScratch

// vecs is a flat xyz list with room for one wrap-around vertex past nump
func ClipSkyPolygon(nump int, vecs []float32, stage int) {
	var newc [2]int
	var i int

	if nump > MAX_CLIP_VERTS-2 {
		Sys_Error("ClipSkyPolygon: nump > MAX_CLIP_VERTS - 2")
	}

	if stage == 6 {
		// fully clipped, so draw it
		DrawSkyPolygon(nump, vecs)
		return
	}

	scratch := &sky_clip_scratch[stage]
	dists := &scratch.dists
	sides := &scratch.sides
	newv := &scratch.newv

	front, back := false, false
	norm := skyclip[stage]
	for i = 0; i < nump; i++ {
		v := vec3_t{vecs[i*3+0], vecs[i*3+1], vecs[i*3+2]}
		d := DotProduct(v, norm)
		if float64(d) > ON_EPSILON {
			front = true
			sides[i] = SIDE_FRONT
		} else if float64(d) < -ON_EPSILON {
			back = true
			sides[i] = SIDE_BACK
		} else {
			sides[i] = SIDE_ON
		}
		dists[i] = d
	}

	if !front || !back {
		// not clipped
		ClipSkyPolygon(nump, vecs, stage+1)
		return
	}

	// clip it
	sides[i] = sides[0]
	dists[i] = dists[0]
	copy(vecs[i*3:i*3+3], vecs[0:3])
	newc[0], newc[1] = 0, 0

	for i = 0; i < nump; i++ {
		v := vecs[i*3:]
		switch sides[i] {
		case SIDE_FRONT:
			copy(newv[0][newc[0]*3:newc[0]*3+3], v[0:3])
			newc[0]++
		case SIDE_BACK:
			copy(newv[1][newc[1]*3:newc[1]*3+3], v[0:3])
			newc[1]++
		case SIDE_ON:
			copy(newv[0][newc[0]*3:newc[0]*3+3], v[0:3])
			newc[0]++
			copy(newv[1][newc[1]*3:newc[1]*3+3], v[0:3])
			newc[1]++
		}

		if sides[i] == SIDE_ON || sides[i+1] == SIDE_ON || sides[i+1] == sides[i] {
			continue
		}

		d := dists[i] / (dists[i] - dists[i+1])
		for j := 0; j < 3; j++ {
			e := v[j] + d*(v[j+3]-v[j])
			newv[0][newc[0]*3+j] = e
			newv[1][newc[1]*3+j] = e
		}
		newc[0]++
		newc[1]++
	}

	// continue
	ClipSkyPolygon(newc[0], newv[0][:], stage+1)
	ClipSkyPolygon(newc[1], newv[1][:], stage+1)
}

func R_AddSkyBoxSurface(fa *msurface_t) {
	var verts [MAX_CLIP_VERTS * 3]float32

	// calculate vertex values for sky box
	for p := fa.warppolys; p != nil; p = p.next {
		for i := 0; i < p.numverts; i++ {
			verts[i*3+0] = p.verts[i][0] - r_origin[0]
			verts[i*3+1] = p.verts[i][1] - r_origin[1]
			verts[i*3+2] = p.verts[i][2] - r_origin[2]
		}
		ClipSkyPolygon(p.numverts, verts[:], 0)
	}
}

func R_ClearSkyBox() {
	for i := 0; i < 6; i++ {
		skymins[0][i], skymins[1][i] = 9999, 9999
		skymaxs[0][i], skymaxs[1][i] = -9999, -9999
	}
}

func MakeSkyVec(s, t float32, axis int, out *scene_vert_t) {
	var v, b vec3_t

	farclip := max(int(r_farclip.value), 4096)
	b[0] = s * float32(farclip>>1)
	b[1] = t * float32(farclip>>1)
	b[2] = float32(farclip >> 1)

	for j := 0; j < 3; j++ {
		k := st_to_vec[axis][j]
		if k < 0 {
			v[j] = -b[-k-1]
		} else {
			v[j] = b[k-1]
		}
		v[j] += r_origin[j]
	}

	// avoid bilerp seam
	s = (s + 1) * 0.5
	t = (t + 1) * 0.5

	s = float32(bound(1.0/512, float64(s), 511.0/512))
	t = float32(bound(1.0/512, float64(t), 511.0/512))

	t = 1 - t
	out.pos[0] = v[0]
	out.pos[1] = v[1]
	out.pos[2] = v[2]
	out.st[0] = s
	out.st[1] = t
	out.lm[0], out.lm[1] = 0, 0
	out.rgba[0], out.rgba[1], out.rgba[2], out.rgba[3] = 255, 255, 255, 255
}

var skytexorder = [6]int{0, 2, 1, 3, 4, 5}

func R_DrawSkyBox() {
	var firstvert, firstindex uint32

	if skychain == nil {
		return
	}

	R_ClearSkyBox()
	for fa := skychain; fa != nil; fa = fa.texturechain {
		R_AddSkyBoxSurface(fa)
	}

	for i := 0; i < 6; i++ {
		if skymins[0][i] >= skymaxs[0][i] || skymins[1][i] >= skymaxs[1][i] {
			continue
		}

		v := Scene_AllocVerts(4, &firstvert)
		if v == nil {
			break
		}
		idx := Scene_AllocIndices(6, &firstindex)
		if idx == nil {
			break
		}

		MakeSkyVec(skymins[0][i], skymins[1][i], i, &v[0])
		MakeSkyVec(skymins[0][i], skymaxs[1][i], i, &v[1])
		MakeSkyVec(skymaxs[0][i], skymaxs[1][i], i, &v[2])
		MakeSkyVec(skymaxs[0][i], skymins[1][i], i, &v[3])

		idx[0] = firstvert
		idx[1] = firstvert + 1
		idx[2] = firstvert + 2
		idx[3] = firstvert
		idx[4] = firstvert + 2
		idx[5] = firstvert + 3

		Scene_AddBatch(SCENE_PIPE_TEX, skyboxtextures+skytexorder[i], nil, GPU_GetDynamicSceneVB(), firstindex, 6, current_mvp)
	}

	// z fill so world geometry occludes the sky quads
	if current_vb != nil {
		for fa := skychain; fa != nil; fa = fa.texturechain {
			if fa.fastpolys == nil {
				continue
			}
			n := Fan_AllocIndices(fa.fastpolyfirstindex, fa.numedges, &firstindex)
			if n != 0 {
				Scene_AddBatch(SCENE_PIPE_DEPTHFILL, GPU_Texture_White(), nil, current_vb.buf, firstindex, n, current_mvp)
			}
		}
	}

	skychain = nil
	skychain_tail = &skychain
}

// ---- brush models and world ----

func R_DrawBrushModel(e *entity_t) {
	var mins, maxs vec3_t
	var rotated bool
	var entmatrix [16]float32

	currententity = e
	currenttexture = -1

	clmodel := e.model

	if e.angles[0] != 0 || e.angles[1] != 0 || e.angles[2] != 0 {
		rotated = true
		if R_CullSphere(e.origin, clmodel.radius) {
			return
		}
	} else {
		rotated = false
		VectorAdd(e.origin, clmodel.mins, &mins)
		VectorAdd(e.origin, clmodel.maxs, &maxs)
		if R_CullBox(mins, maxs) {
			return
		}
	}

	VectorSubtract(r_refdef.vieworg, e.origin, &modelorg)
	if rotated {
		var forward, right, up vec3_t

		temp := modelorg
		AngleVectors(e.angles, &forward, &right, &up)
		modelorg[0] = DotProduct(temp, forward)
		modelorg[1] = -DotProduct(temp, right)
		modelorg[2] = DotProduct(temp, up)
	}

	// calculate dynamic lighting for bmodel if it's not an instanced model
	if clmodel.firstmodelsurface != 0 {
		for li := uint(0); li < MAX_DLIGHTS/32; li++ {
			if cl_dlight_active[li] != 0 {
				for lj := uint(0); lj < 32; lj++ {
					if (cl_dlight_active[li]&(1<<lj)) != 0 && li*32+lj < MAX_DLIGHTS {
						k := int(li*32 + lj)
						// C shifts a 32-bit int by k, x86 masks the count to 5 bits
						if gl_flashblend.value == 0 || (cl_dlights[k].bubble != 0 && gl_flashblend.value != 2) {
							R_MarkLights(clmodel, &cl_dlights[k], 1<<(k&31), uint32(clmodel.hulls[0].firstclipnode))
						}
					}
				}
			}
		}
	}

	// entity transform, same order as the glTranslatef/glRotatef sequence
	Mat4_Identity(entmatrix[:])
	Mat4_Translate(entmatrix[:], e.origin[0], e.origin[1], e.origin[2])
	Mat4_RotateZ(entmatrix[:], e.angles[1])
	Mat4_RotateY(entmatrix[:], e.angles[0])
	Mat4_RotateX(entmatrix[:], e.angles[2])
	Mat4_Multiply(r_viewproj[:], entmatrix[:], brush_mvp[:])
	current_mvp = brush_mvp[:]
	current_vb = World_FindVB(clmodel)

	R_ClearTextureChains(clmodel)

	for i := 0; i < clmodel.nummodelsurfaces; i++ {
		psurf := &clmodel.surfaces[clmodel.firstmodelsurface+i]

		// find which side of the node we are on
		pplane := psurf.plane
		dot := PlaneDiff(modelorg, pplane)

		flags := clmodel.surfflags[clmodel.firstmodelsurface+i]

		// draw the water surfaces now, and setup sky/normal chains
		if (flags&SURF_PLANEBACK != 0 && float64(dot) < -BACKFACE_EPSILON) ||
			(flags&SURF_PLANEBACK == 0 && float64(dot) > BACKFACE_EPSILON) {
			if flags&SURF_DRAWSKY != 0 {
				chainSurfB2F(psurf, &skychain)
			} else if flags&SURF_DRAWTURB != 0 {
				EmitWaterPolys(clmodel, psurf)
			} else if flags&SURF_DRAWALPHA != 0 {
				chainSurfB2F(psurf, &alphachain)
			} else if r_drawflat_enable.value == 1 && psurf.is_drawflat != 0 {
				chainSurfB2F(psurf, &drawflatchain)
			} else {
				underwater := 0
				if flags&SURF_UNDERWATER != 0 {
					underwater = 1
				}
				chainSurfB2F(psurf, &psurf.texinfo.texture.texturechain[underwater])
			}
		}
	}

	// draw the textures chains for the model
	R_RenderAllDynamicLightmaps(clmodel)
	R_UpdateFlatColours(clmodel)
	DrawTextureChains(clmodel)
	R_DrawFlat(clmodel)
	R_DrawSkyChain()
	R_DrawAlphaChain()

	current_mvp = r_viewproj[:]
	if cl.worldmodel != nil {
		current_vb = World_FindVB(cl.worldmodel)
	} else {
		current_vb = nil
	}
}

func R_RecursiveWorldNode(model *model_t, nodenum uint32, clipflags int) {
	var pleaf *mleaf_t
	var node *mnode_t
	var visframe int
	var minmaxs *[6]float32

	if nodenum >= uint32(model.numnodes) {
		leafnum := nodenum - uint32(model.numnodes)
		if model.leafsolid[leafnum/32]&(1<<(leafnum%32)) != 0 {
			return // solid
		}
		pleaf = &model.leafs[leafnum]
		visframe = pleaf.visframe
		minmaxs = &pleaf.minmaxs
	} else {
		node = &model.nodes[nodenum]
		visframe = node.visframe
		minmaxs = &node.minmaxs
	}

	if visframe != r_visframecount {
		return
	}

	if clipflags != 0 {
		for c := 0; c < 4; c++ {
			if clipflags&(1<<c) == 0 {
				continue // don't need to clip against it
			}

			clipped := BOX_ON_PLANE_SIDE(vec3_t{minmaxs[0], minmaxs[1], minmaxs[2]}, vec3_t{minmaxs[3], minmaxs[4], minmaxs[5]}, &frustum[c])
			if clipped == 2 {
				return
			} else if clipped == 1 {
				clipflags &^= 1 << c // node is entirely on screen
			}
		}
	}

	// if a leaf node, draw stuff
	if pleaf != nil {
		mark := pleaf.firstmarksurfacenum
		c := pleaf.nummarksurfaces

		if c != 0 {
			for {
				surfnum := model.marksurfaces[mark]
				cl.worldmodel.surfvisible[surfnum/32] |= 1 << (surfnum % 32)
				mark++
				c--
				if c == 0 {
					break
				}
			}
		}

		// deal with model fragments in this leaf
		if pleaf.efrags != nil {
			R_StoreEfrags(&pleaf.efrags)
		}
	} else {
		// node is just a decision point, so go down the apropriate sides
		// find which side of the node we are on
		plane := &model.planes[node.planenum]
		dot := PlaneDiff(modelorg, plane)
		side := 0
		if dot < 0 {
			side = 1
		}

		// recurse down the children, front side first
		R_RecursiveWorldNode(model, node.childrennum[side], clipflags)

		// draw stuff
		c := int(node.numsurfaces)

		if c != 0 {
			surfnum := node.firstsurface

			if float64(dot) < -BACKFACE_EPSILON {
				side = SURF_PLANEBACK
			} else if float64(dot) > BACKFACE_EPSILON {
				side = 0
			}

			for ; c != 0; c, surfnum = c-1, surfnum+1 {
				surf := &cl.worldmodel.surfaces[surfnum]

				if cl.worldmodel.surfvisible[surfnum/32]&(1<<(surfnum%32)) == 0 {
					continue
				}

				flags := cl.worldmodel.surfflags[surfnum]

				if (dot < 0) != (flags&SURF_PLANEBACK != 0) {
					continue // wrong side
				}

				// add surf to the right chain
				if flags&SURF_DRAWSKY != 0 {
					chainSurfF2B(surf, &skychain_tail)
				} else if flags&SURF_DRAWTURB != 0 {
					chainSurfF2B(surf, &waterchain_tail)
				} else if flags&SURF_DRAWALPHA != 0 {
					chainSurfB2F(surf, &alphachain)
				} else if r_drawflat_enable.value == 1 && surf.is_drawflat != 0 {
					chainSurfF2B(surf, &drawflatchain_tail)
				} else {
					underwater := 0
					if flags&SURF_UNDERWATER != 0 {
						underwater = 1
					}
					chainSurfF2B(surf, &surf.texinfo.texture.texturechain_tail[underwater])
				}
			}
		}

		// recurse down the back side
		R_RecursiveWorldNode(model, node.childrennum[b2i(side == 0)], clipflags)
	}
}

var r_worldent entity_t

func R_DrawWorld() {
	r_worldent = entity_t{}
	r_worldent.model = cl.worldmodel

	R_ClearTextureChains(cl.worldmodel)

	modelorg = r_refdef.vieworg

	currententity = &r_worldent
	currenttexture = -1
	current_mvp = r_viewproj[:]
	current_vb = World_FindVB(cl.worldmodel)

	// set up texture chains for the world
	for i := range cl.worldmodel.surfvisible {
		cl.worldmodel.surfvisible[i] = 0
	}

	R_RecursiveWorldNode(cl.worldmodel, 0, 15)

	// draw the world sky
	if r_skyboxloaded {
		R_DrawSkyBox()
	} else {
		R_DrawSkyChain()
	}

	R_DrawEntitiesOnList(&cl_firstpassents)

	// draw the world
	R_RenderAllDynamicLightmaps(cl.worldmodel)
	R_UpdateFlatColours(cl.worldmodel)
	DrawTextureChains(cl.worldmodel)
	R_DrawFlat(cl.worldmodel)

	// draw the world alpha textures
	R_DrawAlphaChain()
}

func R_MarkLeaves() {
	var solid [MAX_MAP_LEAFS / 8]byte
	var vis []byte

	if r_novis.value == 0 && r_oldviewleaf == r_viewleaf &&
		r_oldviewleaf2 == r_viewleaf2 { // watervis hack
		return
	}

	r_visframecount++
	r_oldviewleaf = r_viewleaf

	if r_novis.value != 0 {
		vis = solid[:]
		for i := 0; i < (cl.worldmodel.numleafs+7)>>3; i++ {
			solid[i] = 0xff
		}
	} else {
		vis = Mod_LeafPVS(r_viewleaf, cl.worldmodel)

		if r_viewleaf2 != nil {
			// merge visibility data for two leafs
			count := (cl.worldmodel.numleafs + 7) >> 3
			copy(solid[:count], vis)
			src := Mod_LeafPVS(r_viewleaf2, cl.worldmodel)
			for i := 0; i < count; i++ {
				solid[i] |= src[i]
			}
			vis = solid[:]
		}
	}

	for i := 0; i < cl.worldmodel.numleafs; i++ {
		if vis[i>>3]&(1<<(i&7)) != 0 {
			// a leaf first, then its parent nodes up to the root
			leaf := &cl.worldmodel.leafs[i+1]
			if leaf.visframe == r_visframecount {
				continue
			}
			leaf.visframe = r_visframecount
			if leaf.parentnum == 0xffffffff {
				continue
			}
			nodenum := leaf.parentnum
			for {
				node := &cl.worldmodel.nodes[nodenum]
				if node.visframe == r_visframecount {
					break
				}
				node.visframe = r_visframecount
				if node.parentnum == 0xffffffff {
					break
				}
				nodenum = node.parentnum
			}
		}
	}
}

// ---- load-time lightmap and geometry building ----

// returns a texture number and the position inside it
func AllocBlock(w, h int, x, y *int) int {
	if w < 1 || w > BLOCK_WIDTH || h < 1 || h > BLOCK_HEIGHT {
		Sys_Error("AllocBlock: Bad dimensions")
	}

	for texnum := 0; texnum < MAX_LIGHTMAPS; texnum++ {
		best := BLOCK_HEIGHT + 1

		for i := 0; i < BLOCK_WIDTH-w; i++ {
			best2 := 0

			var j int
			for j = i; j < i+w; j++ {
				if allocated[texnum][j] >= best {
					i = j
					break
				}
				if allocated[texnum][j] > best2 {
					best2 = allocated[texnum][j]
				}
			}
			if j == i+w {
				// this is a valid spot
				*x = i
				best = best2
				*y = best
			}
		}

		if best+h > BLOCK_HEIGHT {
			continue
		}

		for i := 0; i < w; i++ {
			allocated[texnum][*x+i] = best + h
		}

		return texnum
	}

	Sys_Error("AllocBlock: full")
	return 0
}

func BuildSurfaceDisplayList(model *model_t, fa *msurface_t) {
	// reconstruct the polygon
	lnumverts := fa.numedges

	// draw texture
	poly := &glpoly_t{numverts: lnumverts, verts: make([][VERTEXSIZE]float32, lnumverts)}
	fa.polys = poly

	for i := 0; i < lnumverts; i++ {
		lindex := model.surfedges[fa.firstedge+i]

		var vec vec3_t
		if lindex > 0 {
			vec = model.vertexes[model.edges[lindex].v[0]].position
		} else {
			vec = model.vertexes[model.edges[-lindex].v[1]].position
		}
		s := DotProduct(vec, texVec3(&fa.texinfo.vecs[0])) + fa.texinfo.vecs[0][3]
		s /= float32(fa.texinfo.texture.width)

		t := DotProduct(vec, texVec3(&fa.texinfo.vecs[1])) + fa.texinfo.vecs[1][3]
		t /= float32(fa.texinfo.texture.height)

		poly.verts[i][0] = vec[0]
		poly.verts[i][1] = vec[1]
		poly.verts[i][2] = vec[2]
		poly.verts[i][3] = s
		poly.verts[i][4] = t

		// lightmap texture coordinates
		s = DotProduct(vec, texVec3(&fa.texinfo.vecs[0])) + fa.texinfo.vecs[0][3]
		s -= float32(fa.texturemins[0])
		s += float32(fa.light_s * 16)
		s += 8
		s /= BLOCK_WIDTH * 16

		t = DotProduct(vec, texVec3(&fa.texinfo.vecs[1])) + fa.texinfo.vecs[1][3]
		t -= float32(fa.texturemins[1])
		t += float32(fa.light_t * 16)
		t += 8
		t /= BLOCK_HEIGHT * 16

		poly.verts[i][5] = s
		poly.verts[i][6] = t

		// detail texture coordinates
		s = DotProduct(vec, texVec3(&fa.texinfo.vecs[0])) + fa.texinfo.vecs[0][3]
		s /= 128
		t = DotProduct(vec, texVec3(&fa.texinfo.vecs[1])) + fa.texinfo.vecs[1][3]
		t /= 128
		poly.verts[i][7] = s
		poly.verts[i][8] = t
	}
}

func R_CreateSurfaceLightmap(surf *msurface_t) {
	smax := (int(surf.extents[0]) >> 4) + 1
	tmax := (int(surf.extents[1]) >> 4) + 1

	if smax > BLOCK_WIDTH {
		Host_Error("R_CreateSurfaceLightmap: smax = %d > BLOCK_WIDTH", smax)
	}
	if tmax > BLOCK_HEIGHT {
		Host_Error("R_CreateSurfaceLightmap: tmax = %d > BLOCK_HEIGHT", tmax)
	}
	if smax*tmax > MAX_LIGHTMAP_SIZE {
		Host_Error("R_CreateSurfaceLightmap: smax * tmax = %d > MAX_LIGHTMAP_SIZE", smax*tmax)
	}

	surf.lightmaptexturenum = AllocBlock(smax, tmax, &surf.light_s, &surf.light_t)
	base := surf.lightmaptexturenum * BLOCK_WIDTH * BLOCK_HEIGHT * 4
	base += (surf.light_t*BLOCK_WIDTH + surf.light_s) * 4
	R_BuildLightMap(surf, lightmaps[base:], BLOCK_WIDTH*4, 0)
}

// one interleaved static buffer per model: lit poly verts first, then turb/sky fastpoly verts
func World_BuildModelBuffer(m *model_t) {
	litverts := uint32(0)
	warpverts := uint32(0)
	for i := 0; i < m.numsurfaces; i++ {
		if m.surfaces[i].polys != nil {
			litverts += uint32(m.surfaces[i].polys.numverts)
		}
		if m.surfaces[i].fastpolys != nil {
			warpverts += uint32(m.surfaces[i].numedges)
		}
	}
	total := litverts + warpverts
	if total == 0 {
		return
	}
	// sanity guard for corrupt data; big community maps exceed 150k legitimately
	if total > (1 << 22) {
		Com_Printf("Model \"%s\" has %d verts, not building a vertex buffer\n", m.name, total)
		return
	}
	if num_worldvbs >= MAX_MODELS {
		return
	}

	verts := make([]scene_vert_t, total)

	m.num_vertices = litverts
	if litverts != 0 {
		// cl_main.c drawflat writes vertcolours through the surfaces
		m.vertcoords = make([]float32, 3*litverts)
		m.vertcolours = make([]float32, 3*litverts)
	}

	vert := uint32(0)
	for i := 0; i < m.numsurfaces; i++ {
		poly := m.surfaces[i].polys
		if poly == nil {
			continue
		}
		poly.firstindex = int(vert)
		for j := 0; j < poly.numverts; j++ {
			v := &verts[vert]
			v.pos[0] = poly.verts[j][0]
			v.pos[1] = poly.verts[j][1]
			v.pos[2] = poly.verts[j][2]
			v.st[0] = poly.verts[j][3]
			v.st[1] = poly.verts[j][4]
			v.lm[0] = poly.verts[j][5]
			v.lm[1] = poly.verts[j][6]
			v.rgba[0], v.rgba[1], v.rgba[2], v.rgba[3] = 255, 255, 255, 255
			if m.vertcoords != nil {
				m.vertcoords[3*vert+0] = v.pos[0]
				m.vertcoords[3*vert+1] = v.pos[1]
				m.vertcoords[3*vert+2] = v.pos[2]
			}
			if m.vertcolours != nil {
				m.vertcolours[3*vert+0] = 1
				m.vertcolours[3*vert+1] = 1
				m.vertcolours[3*vert+2] = 1
			}
			vert++
		}
	}

	for i := 0; i < m.numsurfaces; i++ {
		surf := &m.surfaces[i]
		if surf.fastpolys == nil {
			continue
		}
		surf.fastpolyfirstindex = vert
		for j := 0; j < surf.numedges; j++ {
			v := &verts[vert]
			v.pos[0] = surf.fastpolys[j*3+0]
			v.pos[1] = surf.fastpolys[j*3+1]
			v.pos[2] = surf.fastpolys[j*3+2]
			v.st[0] = surf.shadertexcoords[j*2+0]
			v.st[1] = surf.shadertexcoords[j*2+1]
			v.lm[0], v.lm[1] = 0, 0
			v.rgba[0], v.rgba[1], v.rgba[2], v.rgba[3] = 255, 255, 255, 255
			vert++
		}
	}

	buf := GPU_CreateStaticVertexBuffer(verts)
	if buf == nil {
		return
	}

	vb := &worldvbs[num_worldvbs]
	num_worldvbs++
	vb.surfaces = surfacesKey(m)
	vb.buf = buf
	vb.verts = verts
	vb.numverts = total
	vb.litverts = litverts
}

// Builds the lightmap data and vertex buffers for all brush models
func R_BuildLightmaps() {
	allocated = [MAX_LIGHTMAPS][BLOCK_WIDTH]int{}

	r_framecount = 1 // no dlightcache

	World_Shutdown()

	for j := 1; j < MAX_MODELS; j++ {
		m := cl.model_precache[j]
		if m == nil {
			break
		}
		if strings.Contains(m.name, "*") {
			continue // inline submodels share the parent's buffer
		}
		if m.type_ != mod_brush {
			continue
		}
		for i := 0; i < m.numsurfaces; i++ {
			if m.surfflags[i]&(SURF_DRAWTURB|SURF_DRAWSKY) != 0 {
				continue
			}
			if m.surfaces[i].texinfo.flags&TEX_SPECIAL != 0 {
				continue
			}
			R_CreateSurfaceLightmap(&m.surfaces[i])
			BuildSurfaceDisplayList(m, &m.surfaces[i])
		}
		World_BuildModelBuffer(m)
	}

	// create pages for the used slots, first upload sends whole pages
	for i := 0; i < MAX_LIGHTMAPS; i++ {
		if allocated[i][0] == 0 {
			break // no more used
		}
		lightmap_pages[i] = GPU_CreateDynamicTexture(BLOCK_WIDTH, BLOCK_HEIGHT)
		lightmap_modified[i] = true
		lightmap_rectchange[i].l = 0
		lightmap_rectchange[i].t = 0
		lightmap_rectchange[i].w = BLOCK_WIDTH
		lightmap_rectchange[i].h = BLOCK_HEIGHT
	}

	GPU_SetSceneUploader(World_UploadLightmaps)
}

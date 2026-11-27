// Copyright (C) 1996-1997 Id Software, Inc.
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
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"strings"
)

// r_model.c, r_model.h and bspfile.h -- model loading and caching

// entity effects
const (
	EF_BRIGHTFIELD = 1
	EF_MUZZLEFLASH = 2
	EF_BRIGHTLIGHT = 4
	EF_DIMLIGHT    = 8
	EF_FLAG1       = 16
	EF_FLAG2       = 32
	EF_BLUE        = 64
	EF_RED         = 128
)

// model flags, from the alias model header
const (
	EF_ROCKET  = 1
	EF_GRENADE = 2
	EF_GIB     = 4
	EF_ROTATE  = 8
	EF_TRACER  = 16
	EF_ZOMGIB  = 32
	EF_TRACER2 = 64
	EF_TRACER3 = 128
)

// bspfile.h
const (
	MAX_MAP_HULLS = 4
	MAX_MAP_LEAFS = 32767

	Q1_BSPVERSION    = 29
	HL_BSPVERSION    = 30
	Q1_BSPVERSION2   = 'B' + 'S'<<8 + 'P'<<16 + '2'<<24
	Q1_BSPVERSION29a = '2' + 'P'<<8 + 'S'<<16 + 'B'<<24

	LUMP_ENTITIES     = 0
	LUMP_PLANES       = 1
	LUMP_TEXTURES     = 2
	LUMP_VERTEXES     = 3
	LUMP_VISIBILITY   = 4
	LUMP_NODES        = 5
	LUMP_TEXINFO      = 6
	LUMP_FACES        = 7
	LUMP_LIGHTING     = 8
	LUMP_CLIPNODES    = 9
	LUMP_LEAFS        = 10
	LUMP_MARKSURFACES = 11
	LUMP_EDGES        = 12
	LUMP_SURFEDGES    = 13
	LUMP_MODELS       = 14
	HEADER_LUMPS      = 15

	PLANE_X    = 0
	PLANE_Y    = 1
	PLANE_Z    = 2
	PLANE_ANYX = 3
	PLANE_ANYY = 4
	PLANE_ANYZ = 5

	CONTENTS_EMPTY = -1
	CONTENTS_SOLID = -2
	CONTENTS_WATER = -3
	CONTENTS_SLIME = -4
	CONTENTS_LAVA  = -5
	CONTENTS_SKY   = -6

	AMBIENT_WATER = 0
	AMBIENT_SKY   = 1
	AMBIENT_SLIME = 2
	AMBIENT_LAVA  = 3
	NUM_AMBIENTS  = 4

	MAXLIGHTMAPS = 4
	MIPLEVELS    = 4

	TEX_SPECIAL = 1 // sky or slime, no lightmap or 256 subdivision
)

// on-disk sizes of the lump records
const (
	sizeof_lump_t          = 8
	sizeof_dheader_t       = 4 + HEADER_LUMPS*sizeof_lump_t
	sizeof_dmodel_t        = 64
	sizeof_dplane_t        = 20
	sizeof_dnode_t         = 24
	sizeof_dnode_bsp2_t    = 44
	sizeof_dclipnode_t     = 8
	sizeof_dclipnode29a_t  = 12
	sizeof_dleaf_t         = 28
	sizeof_dleaf_bsp2_t    = 44
	sizeof_dface_t         = 20
	sizeof_dface29a_t      = 28
	sizeof_bspx_header_t   = 8
	sizeof_bspx_lump_t     = 32
	sizeof_dmarksurface    = 2
	sizeof_dmarksurface29a = 4
)

type lump_t struct {
	fileofs, filelen int32
}

type dheader_t struct {
	version int32
	lumps   [HEADER_LUMPS]lump_t
}

type dmodel_t struct {
	mins, maxs [3]float32
	origin     [3]float32
	headnode   [MAX_MAP_HULLS]int32
	visleafs   int32 // not including the solid leaf 0
	firstface  int32
	numfaces   int32
}

type bspx_lump_t struct {
	lumpname string
	fileofs  int32
	filelen  int32
}

type bspx_header_t struct {
	numlumps int32
	lumps    []bspx_lump_t
}

// modelgen.h, spritegn.h
const (
	IDPOLYHEADER   = 'O'<<24 + 'P'<<16 + 'D'<<8 + 'I'
	IDSPRITEHEADER = 'P'<<24 + 'S'<<16 + 'D'<<8 + 'I'
)

type synctype_t int

const (
	ST_SYNC synctype_t = iota
	ST_RAND
)

// r_model.h
const (
	SIDE_FRONT = 0
	SIDE_BACK  = 1
	SIDE_ON    = 2
)

const (
	SURF_PLANEBACK  = 1 << 1
	SURF_DRAWSKY    = 1 << 2
	SURF_DRAWTURB   = 1 << 3
	SURF_DRAWTILED  = 1 << 4
	SURF_UNDERWATER = 1 << 6
	SURF_DRAWALPHA  = 1 << 7
)

type mplane_t struct {
	normal   vec3_t
	dist     float32
	type_    byte // for texture axis selection and fast side tests
	signbits byte // signx + signy<<1 + signz<<1
}

type mvertex_t struct {
	position vec3_t
}

// pixels holds the mip data of the built-in checkerboard texture, where
// C puts it right after the struct; offsets index into it
type texture_t struct {
	name              string
	width, height     uint32
	gl_texturenum     int
	fb_texturenum     int // index of fullbright mask or 0
	texturechain      [2]*msurface_t
	texturechain_tail [2]**msurface_t
	anim_total        int // total tenths in sequence ( 0 = no)
	anim_min          int // time for this frame min <=time< max
	anim_max          int
	anim_next         *texture_t // in the animation sequence
	alternate_anims   *texture_t // bmodels in frame 1 use these
	offsets           [MIPLEVELS]uint32
	colour            uint32
	isLumaTexture     int
	pixels            []byte
}

type medge_t struct {
	v [2]uint32
}

type mtexinfo_t struct {
	vecs    [2][4]float32
	texture *texture_t
	flags   int
}

const WARPVERTEXSIZE = 5 // xyz s1t1
const VERTEXSIZE = 9     // xyz s1t1 s2t2 s3t3

type glwarppoly struct {
	next     *glwarppoly
	numverts int
	verts    [][WARPVERTEXSIZE]float32
}

type glpoly_t struct {
	fb_chain       *glpoly_t // next fb poly in chain
	luma_chain     *glpoly_t // next luma poly in chain
	caustics_chain *glpoly_t // next caustic poly in chain
	detail_chain   *glpoly_t // next detail poly in chain
	numverts       int
	firstindex     int
	verts          [][VERTEXSIZE]float32
}

type msurface_t struct {
	plane     *mplane_t
	firstedge int // look up in model->surfedges[], negative numbers
	numedges  int // are backwards edges

	texturemins [2]int16
	extents     [2]int16

	is_drawflat int
	color       [3]float32

	light_s, light_t int // gl lightmap coordinates

	warppolys          *glwarppoly
	polys              *glpoly_t // multiple if warped
	fastpolys          []float32
	shadertexcoords    []float32
	fastpolyfirstindex uint32
	texturechain       *msurface_t

	texinfo *mtexinfo_t

	dlightframe        int
	dlightbits         int
	lightmaptexturenum int
	styles             [MAXLIGHTMAPS]byte
	cached_light       [MAXLIGHTMAPS]int // values currently used in lightmap
	cached_dlight      bool              // true if dynamic light in cache
	samples            []byte            // [numstyles*surfsize], into lightdata
}

// Node children and leafs share one index space: a child number at or
// past numnodes is a leaf (NODENUM_TO_NODE in C).
type mnode_t struct {
	visframe     int        // node needs to be traversed if current
	minmaxs      [6]float32 // for bounding box culling
	parentnum    uint32
	planenum     uint32
	childrennum  [2]uint32
	firstsurface uint32
	numsurfaces  uint32
}

type mleaf_t struct {
	visframe            int        // node needs to be traversed if current
	minmaxs             [6]float32 // for bounding box culling
	parentnum           uint32
	contents            int16 // wil be a negative contents number
	compressed_vis      []byte
	efrags              *efrag_t
	firstmarksurfacenum uint32
	nummarksurfaces     int
	ambient_sound_level [NUM_AMBIENTS]byte
}

// In-memory clipnode; BSP29 short children are sign-extended to int at load.
type mclipnode_t struct {
	planenum int
	children [2]int
}

// clipnodes and planes are the model's whole arrays, indexed by node
// number like the C pointers
type hull_t struct {
	clipnodes     []mclipnode_t
	planes        []mplane_t
	firstclipnode int
	lastclipnode  int
	clip_mins     vec3_t
	clip_maxs     vec3_t
}

type modtype_t int

const (
	mod_brush modtype_t = iota
	mod_sprite
	mod_alias
)

type modhint_t int

const (
	MOD_NORMAL modhint_t = iota
	MOD_PLAYER
	MOD_EYES
	MOD_FLAME
	MOD_THUNDERBOLT
	MOD_BACKPACK
)

type model_t struct {
	next     *model_t
	name     string
	needload bool // bmodels and sprites don't cache normally
	modhint  modhint_t
	type_    modtype_t

	numframes int
	synctype  synctype_t

	flags int

	// volume occupied by the model graphics
	mins, maxs vec3_t
	radius     float32

	// brush model
	firstmodelsurface, nummodelsurfaces int

	numsubmodels int
	submodels    []model_t

	numplanes int
	planes    []mplane_t

	numleafs int // number of visible leafs, not counting 0
	leafs    []mleaf_t

	numvertexes int
	vertexes    []mvertex_t

	numedges int
	edges    []medge_t

	numnodes int
	nodes    []mnode_t

	numtexinfo int
	texinfo    []mtexinfo_t

	numsurfaces int
	surfaces    []msurface_t

	numsurfedges int
	surfedges    []int

	numclipnodes int
	clipnodes    []mclipnode_t

	nummarksurfaces int
	marksurfaces    []uint32

	hulls [MAX_MAP_HULLS]hull_t

	numtextures int
	textures    []*texture_t

	visdata   []byte
	lightdata []byte
	entities  []byte

	checksum  uint32
	checksum2 uint32

	bspversion   int
	isworldmodel bool

	surfvisible []uint32
	surfflags   []byte
	leafsolid   []uint32

	num_vertices uint32

	surface_colours_dirty bool

	vertcoords  []float32
	vertcolours []float32

	// additional model data
	extradata any
}

var mod_novis [MAX_MAP_LEAFS / 8]byte

var firstmodel *model_t

// Caches the data if needed
func Mod_Extradata(mod *model_t) any {
	if mod.extradata != nil {
		return mod.extradata
	}

	Mod_LoadModel(mod, true)

	return mod.extradata
}

func Mod_PointInLeaf(p vec3_t, model *model_t) *mleaf_t {
	if model == nil || model.nodes == nil {
		Sys_Error("Mod_PointInLeaf: bad model")
	}

	nodenum := uint32(0)
	for {
		if nodenum >= uint32(model.numnodes) {
			return &model.leafs[nodenum-uint32(model.numnodes)]
		}
		node := &model.nodes[nodenum]
		plane := &model.planes[node.planenum]
		d := PlaneDiff(p, plane)
		if d > 0 {
			nodenum = node.childrennum[0]
		} else {
			nodenum = node.childrennum[1]
		}
	}
}

var mod_decompressed [MAX_MAP_LEAFS / 8]byte

func Mod_DecompressVis(in []byte, model *model_t) []byte {
	row := (model.numleafs + 7) >> 3
	out := 0

	if in == nil { // no vis info, so make all visible
		for row > 0 {
			mod_decompressed[out] = 0xff
			out++
			row--
		}
		return mod_decompressed[:]
	}

	for {
		if in[0] != 0 {
			mod_decompressed[out] = in[0]
			out++
			in = in[1:]
		} else {
			c := int(in[1])
			in = in[2:]
			for c > 0 {
				mod_decompressed[out] = 0
				out++
				c--
			}
		}
		if out >= row {
			break
		}
	}

	return mod_decompressed[:]
}

func Mod_LeafPVS(leaf *mleaf_t, model *model_t) []byte {
	if len(model.leafs) > 0 && leaf == &model.leafs[0] {
		return mod_novis[:]
	}
	return Mod_DecompressVis(leaf.compressed_vis, model)
}

// The per-type free functions are memory management and have no Go
// counterpart.
func Mod_ClearAll() {
	firstmodel = nil

	WAD3_ClearTextureWads()
}

func Mod_FindName(name string) *model_t {
	var searchname string

	p := strings.IndexByte(name, '*')
	if p >= 0 {
		searchname = name[:p]
	} else {
		searchname = name
	}

	if searchname == "" {
		Sys_Error("Mod_ForName: NULL name")
	}

	// search the currently loaded models
	mod := firstmodel
	for mod != nil {
		if mod.name == searchname {
			break
		}
		mod = mod.next
	}

	if p >= 0 {
		if mod == nil {
			Sys_Error("Mod_FindName: Submodel for non-existant model %s requested\n", searchname)
		}

		submodel := atoi(name[p+1:])
		if !(submodel > 0 && submodel < mod.numsubmodels) {
			Sys_Error("Mod_FindName: Requested submodel %d out of range\n", submodel)
		}

		mod = &mod.submodels[submodel-1]
	} else if mod == nil {
		mod = &model_t{}
		mod.name = name
		mod.needload = true

		mod.next = firstmodel
		firstmodel = mod
	}

	return mod
}

// Loads a model into the cache
func Mod_LoadModel(mod *model_t, crash bool) *model_t {
	if !mod.needload {
		if mod.type_ == mod_alias {
			if mod.extradata != nil {
				return mod
			}
		} else {
			return mod // not cached at all
		}
	}

	// load the file
	buf := FS_LoadMallocFile(mod.name)
	if buf == nil {
		if crash {
			Host_Error("Mod_LoadModel: %s not found", mod.name)
		}
		return nil
	}

	FMod_CheckModel(mod.name, buf, com_filesize)

	// call the apropriate loader
	mod.needload = false

	var ident uint32
	if len(buf) >= 4 {
		ident = binary.LittleEndian.Uint32(buf)
	}

	switch ident {
	case IDPOLYHEADER:
		Mod_LoadAliasModel(mod, buf)
	case IDSPRITEHEADER:
		Mod_LoadSpriteModel(mod, buf)
	default:
		Mod_LoadBrushModel(mod, buf)
	}

	return mod
}

// Loads in a model for the given name
func Mod_ForName(name string, crash bool) *model_t {
	mod := Mod_FindName(name)
	return Mod_LoadModel(mod, crash)
}

/*
==============================================================================
BRUSHMODEL LOADING
==============================================================================
*/

var mod_base []byte

func lumpData(l *lump_t) []byte {
	return mod_base[l.fileofs : l.fileofs+l.filelen]
}

func readHeader(b []byte) dheader_t {
	var h dheader_t

	h.version = int32(binary.LittleEndian.Uint32(b))
	for i := 0; i < HEADER_LUMPS; i++ {
		h.lumps[i].fileofs = int32(binary.LittleEndian.Uint32(b[4+i*8:]))
		h.lumps[i].filelen = int32(binary.LittleEndian.Uint32(b[8+i*8:]))
	}
	return h
}

func leFloat(b []byte) float32 { return math.Float32frombits(binary.LittleEndian.Uint32(b)) }
func leLong(b []byte) int32    { return int32(binary.LittleEndian.Uint32(b)) }
func leShort(b []byte) int16   { return int16(binary.LittleEndian.Uint16(b)) }

func ISTURBTEX(model *model_t, name string) bool {
	return (model.bspversion == Q1_BSPVERSION && at(name, 0) == '*') ||
		(model.bspversion == HL_BSPVERSION && at(name, 0) == '!')
}

func ISSKYTEX(name string) bool {
	return at(name, 0) == 's' && at(name, 1) == 'k' && at(name, 2) == 'y'
}

// '{' prefix = fence-style texture (palette 255 = transparent), all Q1 BSPs
func ISALPHATEX(model *model_t, name string) bool {
	return at(name, 0) == '{'
}

func Mod_LoadExternalTexture(model *model_t, tx *texture_t, mode int) int {
	if model.bspversion == HL_BSPVERSION {
		return 0
	}

	if model.isworldmodel {
		if gl_externalTextures_world.value == 0 {
			return 0
		}
	} else {
		if gl_externalTextures_bmodels.value == 0 {
			return 0
		}
	}

	name := tx.name
	mapname := TP_MapName()
	groupname := TP_GetMapGroupName(mapname, nil)

	if model.isworldmodel {
		if tx.gl_texturenum = R_LoadTextureImage(va("textures/%s/%s", mapname, name), name, 0, 0, mode); tx.gl_texturenum != 0 {
			if !ISTURBTEX(model, name) {
				tx.fb_texturenum = R_LoadTextureImage(va("textures/%s/%s_luma", mapname, name), va("@fb_%s", name), 0, 0, mode|TEX_LUMA)
			}
		} else {
			if groupname != "" {
				if tx.gl_texturenum = R_LoadTextureImage(va("textures/%s/%s", groupname, name), name, 0, 0, mode); tx.gl_texturenum != 0 {
					if !ISTURBTEX(model, name) {
						tx.fb_texturenum = R_LoadTextureImage(va("textures/%s/%s_luma", groupname, name), va("@fb_%s", name), 0, 0, mode|TEX_LUMA)
					}
				}
			}
		}
	} else {
		if tx.gl_texturenum = R_LoadTextureImage(va("textures/bmodels/%s", name), name, 0, 0, mode); tx.gl_texturenum != 0 {
			if !ISTURBTEX(model, name) {
				tx.fb_texturenum = R_LoadTextureImage(va("textures/bmodels/%s_luma", name), va("@fb_%s", name), 0, 0, mode|TEX_LUMA)
			}
		}
	}

	if tx.gl_texturenum == 0 {
		if tx.gl_texturenum = R_LoadTextureImage(va("textures/%s", name), name, 0, 0, mode); tx.gl_texturenum != 0 {
			if !ISTURBTEX(model, name) {
				tx.fb_texturenum = R_LoadTextureImage(va("textures/%s_luma", name), va("@fb_%s", name), 0, 0, mode|TEX_LUMA)
			}
		}
	}

	if tx.fb_texturenum != 0 {
		tx.isLumaTexture = 1
	}

	return tx.gl_texturenum
}

// a width by height checkerboard of 16 pixel squares, scaled down with the
// mip level
func mod_checkerboard(width, height, step int) []byte {
	d := make([]byte, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if (b2i(x&step != 0) ^ b2i(y&step != 0)) != 0 {
				d[y*width+x] = 0x0e
			} else {
				d[y*width+x] = 0
			}
		}
	}
	return d
}

// miptex_t: name[16], width, height, offsets[4]
const sizeof_miptex_t = 40

func Mod_LoadTextures(model *model_t, l *lump_t) {
	var anims, altanims [10]*texture_t

	if l.filelen == 0 {
		model.textures = nil
		return
	}

	m := lumpData(l)
	nummiptex := int(leLong(m))

	if nummiptex < 0 || nummiptex > 4096 {
		Host_Error("Mod_LoadTextures: bogus nummiptex %d in %s", nummiptex, model.name)
	}

	model.numtextures = nummiptex

	var txblock []texture_t
	if nummiptex != 0 {
		model.textures = make([]*texture_t, nummiptex)
		txblock = make([]texture_t, nummiptex)
	}

	brighten_flag := 0
	if lightmode == 2 {
		brighten_flag = TEX_BRIGHTEN
	}

	for i := 0; i < nummiptex; i++ {
		var offsets [MIPLEVELS]uint32

		dataofs := leLong(m[4+i*4:])
		if dataofs == -1 {
			continue
		}

		nodata := 0
		if uint32(dataofs)&(1<<31) != 0 {
			nodata = 1
			dataofs = int32(uint32(dataofs) &^ (1 << 31))
		}

		mt := m[dataofs:]
		tx := &txblock[i]
		model.textures[i] = tx

		mtname := cstr(mt[:16])
		tx.name = mtname
		if tx.name == "" {
			tx.name = strlcpy(fmt.Sprintf("unnamed%d", i), 16)
			Com_DPrintf("Warning: unnamed texture in %s, renaming to %.16s\n", model.name, tx.name)
		}

		tx.width = uint32(leLong(mt[16:]))
		tx.height = uint32(leLong(mt[20:]))

		if tx.width&15 != 0 || tx.height&15 != 0 {
			Host_Error("Mod_LoadTextures: Texture %.16s is not 16 aligned", mtname)
		}

		if nodata == 0 {
			for j := 0; j < MIPLEVELS; j++ {
				offsets[j] = uint32(leLong(mt[24+j*4:]))
			}
		}

		// HACK HACK HACK
		if mtname == "shot1sid" && tx.width == 32 && tx.height == 32 && CRC_Block(mt[sizeof_miptex_t:sizeof_miptex_t+32*32]) == 65393 {
			// This texture in b_shell1.bsp has some of the first 32 pixels painted white.
			// They are invisible in software, but look really ugly in GL. So we just copy
			// 32 pixels from the bottom to make it look nice.
			copy(mt[sizeof_miptex_t:sizeof_miptex_t+32], mt[sizeof_miptex_t+32*31:sizeof_miptex_t+32*32])
		}

		if nodata == 0 && offsets[0] != 0 {
			pixel := mt[offsets[0]+((tx.height*tx.width)>>1)]
			tx.colour = 255<<24 + d_8to24table[pixel]&0xffffff
		}

		if model.isworldmodel && model.bspversion != HL_BSPVERSION && ISSKYTEX(tx.name) {
			if nodata != 0 {
				R_InitSky(mod_checkerboard(int(tx.width), int(tx.height), 16))
			} else {
				R_InitSky(mt[offsets[0]:])
			}

			continue
		}

		noscale_flag := 0
		if (gl_scaleModelTextures.value == 0 && !model.isworldmodel) ||
			(gl_scaleTurbTextures.value == 0 && ISTURBTEX(model, tx.name)) {
			noscale_flag = TEX_NOSCALE
		}
		texmode := TEX_MIPMAP | noscale_flag
		mipTexLevel := 0
		if noscale_flag == 0 {
			mipTexLevel = int(gl_miptexLevel.value)
		}

		if Mod_LoadExternalTexture(model, tx, texmode) != 0 {
			continue
		}

		if model.bspversion == HL_BSPVERSION {
			if data := WAD3_LoadTexture(mt); data != nil {
				com_netpath = ""
				alpha_flag := 0
				if ISALPHATEX(model, tx.name) {
					alpha_flag = TEX_ALPHA
				}
				tx.gl_texturenum = R_LoadTexturePixels(data, tx.name, int(tx.width), int(tx.height), texmode|alpha_flag)
				continue
			}
		}

		var texname string
		var width, height int
		var data []byte

		if nodata == 0 && offsets[0] != 0 {
			texname = tx.name
			width = int(tx.width) >> mipTexLevel
			height = int(tx.height) >> mipTexLevel
			data = mt[offsets[mipTexLevel]:]
		} else {
			// C points at the checkerboard mip and then draws a fresh one over it
			texname = r_notexture_mip.name
			width = 32 >> mipTexLevel
			height = 32 >> mipTexLevel
			data = mod_checkerboard(width, height, 16/(1<<mipTexLevel))
		}
		alpha_flag := 0
		if ISALPHATEX(model, tx.name) {
			alpha_flag = TEX_ALPHA
		}
		tx.gl_texturenum = R_LoadTexture(texname, width, height, data, texmode|brighten_flag|alpha_flag, 1)
		if !ISTURBTEX(model, tx.name) && Img_HasFullbrights(data, width*height) {
			tx.fb_texturenum = R_LoadTexture(va("@fb_%s", texname), width, height, data, texmode|TEX_FULLBRIGHT, 1)
		}
	}

	// sequence the animations
	for i := 0; i < nummiptex; i++ {
		tx := model.textures[i]
		if tx == nil || at(tx.name, 0) != '+' {
			continue
		}
		if tx.anim_next != nil {
			continue // already sequenced
		}

		// find the number of frames in the animation
		anims = [10]*texture_t{}
		altanims = [10]*texture_t{}

		max := int(at(tx.name, 1))
		altmax := 0
		if max >= 'a' && max <= 'z' {
			max -= 'a' - 'A'
		}
		if max >= '0' && max <= '9' {
			max -= '0'
			altmax = 0
			anims[max] = tx
			max++
		} else if max >= 'A' && max <= 'J' {
			altmax = max - 'A'
			max = 0
			altanims[altmax] = tx
			altmax++
		} else {
			Host_Error("Mod_LoadTextures: Bad animating texture %.16s", tx.name)
		}

		for j := i + 1; j < nummiptex; j++ {
			tx2 := model.textures[j]
			if tx2 == nil || at(tx2.name, 0) != '+' {
				continue
			}
			if tx2.name[2:] != tx.name[2:] {
				continue
			}

			num := int(at(tx2.name, 1))
			if num >= 'a' && num <= 'z' {
				num -= 'a' - 'A'
			}
			if num >= '0' && num <= '9' {
				num -= '0'
				anims[num] = tx2
				if num+1 > max {
					max = num + 1
				}
			} else if num >= 'A' && num <= 'J' {
				num = num - 'A'
				altanims[num] = tx2
				if num+1 > altmax {
					altmax = num + 1
				}
			} else {
				Host_Error("Mod_LoadTextures: Bad animating texture %.16s", tx.name)
			}
		}

		const ANIM_CYCLE = 2
		// link them all together
		for j := 0; j < max; j++ {
			tx2 := anims[j]
			if tx2 == nil {
				Host_Error("Mod_LoadTextures: Missing frame %d of %.16s", j, tx.name)
			}
			tx2.anim_total = max * ANIM_CYCLE
			tx2.anim_min = j * ANIM_CYCLE
			tx2.anim_max = (j + 1) * ANIM_CYCLE
			tx2.anim_next = anims[(j+1)%max]
			if altmax != 0 {
				tx2.alternate_anims = altanims[0]
			}
		}
		for j := 0; j < altmax; j++ {
			tx2 := altanims[j]
			if tx2 == nil {
				Host_Error("Mod_LoadTextures: Missing frame %d of %.16s", j, tx.name)
			}
			tx2.anim_total = altmax * ANIM_CYCLE
			tx2.anim_min = j * ANIM_CYCLE
			tx2.anim_max = (j + 1) * ANIM_CYCLE
			tx2.anim_next = altanims[(j+1)%altmax]
			if max != 0 {
				tx2.alternate_anims = anims[0]
			}
		}
	}
}

func LoadColoredLighting(name string, litfilename *string, litlength *int) []byte {
	var data []byte
	var system bool

	if gl_loadlitfiles.value == 0 {
		return nil
	}

	mapname := TP_MapName()
	groupname := TP_GetMapGroupName(mapname, &system)

	if name != va("maps/%s.bsp", mapname) {
		return nil
	}

	*litfilename = va("maps/lits/%s.lit", mapname)
	f, flen := FS_FOpenFile(*litfilename)

	if flen == -1 {
		*litfilename = va("maps/%s.lit", mapname)
		f, flen = FS_FOpenFile(*litfilename)
	}

	if flen == -1 {
		*litfilename = va("lits/%s.lit", mapname)
		f, flen = FS_FOpenFile(*litfilename)
	}

	if flen == -1 && groupname != "" && !system {
		*litfilename = va("maps/%s.lit", groupname)
		f, flen = FS_FOpenFile(*litfilename)
	}

	if flen == -1 && groupname != "" && !system {
		*litfilename = va("lits/%s.lit", groupname)
		f, flen = FS_FOpenFile(*litfilename)
	}

	if flen != -1 {
		var buf [8]byte

		if _, err := io.ReadFull(f, buf[:]); flen <= 8 || err != nil || string(buf[:4]) != "QLIT" {
			Com_Printf("Corrupt .lit file (%s)...ignoring\n", COM_SkipPath(*litfilename))
		} else if lit_ver := leLong(buf[4:]); lit_ver != 1 {
			Com_Printf("Unknown .lit file version (v%d)\n", lit_ver)
		} else {
			*litlength = flen - 8

			data = make([]byte, flen-8)

			if _, err := io.ReadFull(f, data); err != nil {
				data = nil
			}
		}

		f.Close()
	}

	return data
}

func Mod_LoadLighting(model *model_t, l *lump_t, bspx *bspx_header_t) {
	var litfilename string
	var litlength int
	var source string

	model.lightdata = nil
	if l.filelen == 0 {
		return
	}

	if model.bspversion == HL_BSPVERSION {
		model.lightdata = make([]byte, l.filelen)
		copy(model.lightdata, lumpData(l))
		return
	}

	// check for a .lit file (highest priority for backwards compat)
	data := LoadColoredLighting(model.name, &litfilename, &litlength)
	if data != nil && int(l.filelen)*3 != litlength {
		Com_Printf("Warning: .lit file (%s) has incorrect size\n", COM_SkipPath(litfilename))
		data = nil
	}
	if data != nil {
		source = ".lit"
	} else {
		// check for inline RGBLIGHTING in BSPX
		var bspxsize int
		bspxdata := Mod_BSPX_FindLump(bspx, "RGBLIGHTING", &bspxsize, mod_base)
		if bspxdata != nil && bspxsize == int(l.filelen)*3 {
			data = make([]byte, bspxsize)
			copy(data, bspxdata)
			source = "BSPX RGBLIGHTING"
		}
	}

	if data != nil {
		if developer.value != 0 || cl_warncmd.value != 0 {
			Com_Printf("Static coloured lighting loaded (%s)\n", source)
		}

		model.lightdata = data

		in := lumpData(l)
		out := model.lightdata
		for i := 0; i < int(l.filelen); i++ {
			b := max(int(out[3*i]), max(int(out[3*i+1]), int(out[3*i+2])))

			if b == 0 {
				out[3*i] = in[i]
				out[3*i+1] = in[i]
				out[3*i+2] = in[i]
			} else {
				r := float32(in[i]) / float32(b)
				out[3*i+0] = byte(int(r * float32(out[3*i+0])))
				out[3*i+1] = byte(int(r * float32(out[3*i+1])))
				out[3*i+2] = byte(int(r * float32(out[3*i+2])))
			}
		}

		return
	}

	// no colored lighting available, expand the white lighting data to gray RGB
	model.lightdata = make([]byte, int(l.filelen)*3)

	in := lumpData(l)
	out := model.lightdata
	for i := 0; i < int(l.filelen); i++ {
		d := in[i]
		out[3*i] = d
		out[3*i+1] = d
		out[3*i+2] = d
	}
}

func Mod_LoadVertexes(model *model_t, l *lump_t) {
	if l.filelen%12 != 0 {
		Host_Error("Mod_LoadVertexes: funny lump size in %s", model.name)
	}
	count := int(l.filelen) / 12
	in := lumpData(l)
	out := make([]mvertex_t, count)

	model.vertexes = out
	model.numvertexes = count

	for i := 0; i < count; i++ {
		out[i].position[0] = leFloat(in[i*12:])
		out[i].position[1] = leFloat(in[i*12+4:])
		out[i].position[2] = leFloat(in[i*12+8:])
	}
}

func Mod_LoadEdges(model *model_t, l *lump_t) {
	if l.filelen%4 != 0 {
		Host_Error("Mod_LoadEdges: funny lump size in %s", model.name)
	}
	count := int(l.filelen) / 4
	in := lumpData(l)
	out := make([]medge_t, count+1)

	model.edges = out
	model.numedges = count

	for i := 0; i < count; i++ {
		out[i].v[0] = uint32(uint16(leShort(in[i*4:])))
		out[i].v[1] = uint32(uint16(leShort(in[i*4+2:])))
	}
}

func Mod_LoadEdgesBSP2(model *model_t, l *lump_t) {
	if l.filelen%8 != 0 {
		Host_Error("Mod_LoadEdgesBSP2: funny lump size in %s", model.name)
	}
	count := int(l.filelen) / 8
	in := lumpData(l)
	out := make([]medge_t, count+1)

	model.edges = out
	model.numedges = count

	for i := 0; i < count; i++ {
		out[i].v[0] = uint32(leLong(in[i*8:]))
		out[i].v[1] = uint32(leLong(in[i*8+4:]))
	}
}

const sizeof_texinfo_t = 40

func Mod_LoadTexinfo(model *model_t, l *lump_t) {
	if l.filelen%sizeof_texinfo_t != 0 {
		Host_Error("Mod_LoadTexinfo: funny lump size in %s", model.name)
	}
	count := int(l.filelen) / sizeof_texinfo_t
	in := lumpData(l)
	out := make([]mtexinfo_t, count)

	model.texinfo = out
	model.numtexinfo = count

	for i := 0; i < count; i++ {
		b := in[i*sizeof_texinfo_t:]
		o := &out[i]
		for j := 0; j < 2; j++ {
			for k := 0; k < 4; k++ {
				o.vecs[j][k] = leFloat(b[(j*4+k)*4:])
			}
		}

		miptex := int(leLong(b[32:]))
		o.flags = int(leLong(b[36:]))

		if model.textures == nil {
			o.texture = r_notexture_mip // checkerboard texture
			o.flags = 0
		} else {
			if miptex >= model.numtextures {
				Host_Error("Mod_LoadTexinfo: miptex >= model->numtextures")
			}
			o.texture = model.textures[miptex]
			if o.texture == nil {
				o.texture = r_notexture_mip // texture not found
				o.flags = 0
			}
		}
	}
}

// Fills in s->texturemins[] and s->extents[]
func CalcSurfaceExtents(model *model_t, s *msurface_t) {
	var mins, maxs [2]float32
	var bmins, bmaxs [2]int
	var v *mvertex_t

	mins[0], mins[1] = 999999, 999999
	maxs[0], maxs[1] = -99999, -99999

	tex := s.texinfo

	for i := 0; i < s.numedges; i++ {
		e := model.surfedges[s.firstedge+i]
		if e < 0 {
			if -e >= model.numedges {
				Host_Error("CalcSurfaceExtents: Bad surface edge")
			}
		} else if e >= model.numedges {
			Host_Error("CalcSurfaceExtents: Bad surface edge")
		}

		if e >= 0 {
			v = &model.vertexes[model.edges[e].v[0]]
		} else {
			v = &model.vertexes[model.edges[-e].v[1]]
		}

		for j := 0; j < 2; j++ {
			val := v.position[0]*tex.vecs[j][0] +
				v.position[1]*tex.vecs[j][1] +
				v.position[2]*tex.vecs[j][2] +
				tex.vecs[j][3]
			if val < mins[j] {
				mins[j] = val
			}
			if val > maxs[j] {
				maxs[j] = val
			}
		}
	}

	for i := 0; i < 2; i++ {
		bmins[i] = int(math.Floor(float64(mins[i] / 16)))
		bmaxs[i] = int(math.Ceil(float64(maxs[i] / 16)))

		s.texturemins[i] = int16(bmins[i] * 16)
		s.extents[i] = int16((bmaxs[i] - bmins[i]) * 16)
		if tex.flags&TEX_SPECIAL == 0 && s.extents[i] > 512 /* 256 */ {
			Host_Error("CalcSurfaceExtents: Bad surface extents")
		}
	}
}

// the drawing flags shared by both face loaders
func mod_surfaceFlags(model *model_t, out *msurface_t, flags byte) byte {
	// set the drawing flags flag
	// sky, turb and alpha should be mutually exclusive

	if ISSKYTEX(out.texinfo.texture.name) { // sky
		flags |= SURF_DRAWSKY | SURF_DRAWTILED
		R_SubdivideSurface(model, out) // cut up polygon for warps
		return flags
	}

	if ISTURBTEX(model, out.texinfo.texture.name) { // turbulent
		flags |= SURF_DRAWTURB | SURF_DRAWTILED
		for i := 0; i < 2; i++ {
			out.extents[i] = 16384
			out.texturemins[i] = -8192
		}
		R_SubdivideSurface(model, out) // cut up polygon for warps
		return flags
	}

	if ISALPHATEX(model, out.texinfo.texture.name) {
		flags |= SURF_DRAWALPHA
	}

	return flags
}

func mod_lightSamples(model *model_t, lightofs int32) []byte {
	if lightofs == -1 || model.lightdata == nil {
		return nil
	}
	if model.bspversion == HL_BSPVERSION {
		return model.lightdata[lightofs:]
	}
	return model.lightdata[lightofs*3:]
}

func Mod_LoadFaces(model *model_t, l *lump_t) {
	if l.filelen%sizeof_dface_t != 0 {
		Host_Error("Mod_LoadFaces: funny lump size in %s", model.name)
	}
	count := int(l.filelen) / sizeof_dface_t

	if count > 65535 {
		Sys_Error("Mod_LoadFaces: count > 65535")
	}

	in := lumpData(l)
	out := make([]msurface_t, count)
	model.surfvisible = make([]uint32, (count+31)/32)
	model.surfflags = make([]byte, count)

	model.surfaces = out
	model.numsurfaces = count

	for surfnum := 0; surfnum < count; surfnum++ {
		b := in[surfnum*sizeof_dface_t:]
		o := &out[surfnum]
		var flags byte

		o.firstedge = int(leLong(b[4:]))
		o.numedges = int(leShort(b[8:]))

		planenum := int(leShort(b))
		side := leShort(b[2:])
		if side != 0 {
			flags |= SURF_PLANEBACK
		}

		o.plane = &model.planes[planenum]

		o.texinfo = &model.texinfo[leShort(b[10:])]

		CalcSurfaceExtents(model, o)

		// lighting info
		for i := 0; i < MAXLIGHTMAPS; i++ {
			o.styles[i] = b[12+i]
		}
		o.samples = mod_lightSamples(model, leLong(b[16:]))

		model.surfflags[surfnum] = mod_surfaceFlags(model, o, flags)
	}
}

func Mod_LoadFacesBSP2(model *model_t, l *lump_t) {
	if l.filelen%sizeof_dface29a_t != 0 {
		Host_Error("Mod_LoadFacesBSP2: funny lump size in %s", model.name)
	}
	count := int(l.filelen) / sizeof_dface29a_t

	in := lumpData(l)
	out := make([]msurface_t, count)
	model.surfvisible = make([]uint32, (count+31)/32)
	model.surfflags = make([]byte, count)

	model.surfaces = out
	model.numsurfaces = count

	for surfnum := 0; surfnum < count; surfnum++ {
		b := in[surfnum*sizeof_dface29a_t:]
		o := &out[surfnum]
		var flags byte

		o.firstedge = int(leLong(b[8:]))
		o.numedges = int(leLong(b[12:]))

		planenum := int(leLong(b))
		side := leLong(b[4:])
		if side != 0 {
			flags |= SURF_PLANEBACK
		}

		o.plane = &model.planes[planenum]

		o.texinfo = &model.texinfo[leLong(b[16:])]

		CalcSurfaceExtents(model, o)

		for i := 0; i < MAXLIGHTMAPS; i++ {
			o.styles[i] = b[20+i]
		}
		o.samples = mod_lightSamples(model, leLong(b[24:]))

		model.surfflags[surfnum] = mod_surfaceFlags(model, o, flags)
	}
}

func Mod_LoadSurfedges(model *model_t, l *lump_t) {
	if l.filelen%4 != 0 {
		Host_Error("Mod_LoadSurfedges: funny lump size in %s", model.name)
	}
	count := int(l.filelen) / 4
	in := lumpData(l)
	out := make([]int, count)

	model.surfedges = out
	model.numsurfedges = count

	for i := 0; i < count; i++ {
		out[i] = int(leLong(in[i*4:]))
	}
}

func Mod_LoadVisibility(model *model_t, l *lump_t) {
	if l.filelen == 0 {
		model.visdata = nil
		return
	}

	model.visdata = make([]byte, l.filelen)
	copy(model.visdata, lumpData(l))
}

func Mod_ParseWadsFromEntityLump(data string) {
	var ok bool

	WAD3_ClearTextureWads()

	if data == "" {
		return
	}
	if data, ok = COM_Parse(data); !ok {
		return
	}

	if at(com_token, 0) != '{' {
		return // error
	}

	for {
		if data, ok = COM_Parse(data); !ok {
			return // error
		}

		if at(com_token, 0) == '}' {
			break // end of worldspawn
		}

		key := com_token
		if at(key, 0) == '_' {
			key = key[1:]
		}
		key = strlcpy(key, 1024)

		for len(key) > 0 && key[len(key)-1] == ' ' { // remove trailing spaces
			key = key[:len(key)-1]
		}

		if data, ok = COM_Parse(data); !ok {
			return // error
		}

		value := strlcpy(com_token, 1024)

		if key == "sky" || key == "skyname" {
			Cvar_Set(&r_skyname, value)
		}

		if key == "wad" {
			j := 0
			i := 0
			for i = 0; i < len(value); i++ {
				if value[i] != ';' && value[i] != '\\' && value[i] != '/' && value[i] != ':' {
					break
				}
			}
			if at(value, i) == 0 {
				continue
			}
			for ; i < 1024; i++ {
				// ignore path - the \\ check is for HalfLife... stupid windoze 'programmers'...
				c := at(value, i)
				if c == '\\' || c == '/' || c == ':' {
					j = i + 1
				} else if c == ';' || c == 0 {
					if j < i {
						WAD3_LoadTextureWadFile(value[j:i])
					}
					j = i + 1
					if c == 0 {
						break
					}
				}
			}
		}
	}
}

func Mod_LoadEntities(model *model_t, l *lump_t) {
	if l.filelen == 0 {
		model.entities = nil
		return
	}
	model.entities = make([]byte, l.filelen)
	copy(model.entities, lumpData(l))
	if model.bspversion == HL_BSPVERSION && !dedicated {
		Mod_ParseWadsFromEntityLump(cstr(model.entities))
	}
}

func Mod_LoadSubmodels(model *model_t, l *lump_t) []dmodel_t {
	if l.filelen%sizeof_dmodel_t != 0 {
		Host_Error("Mod_LoadSubmodels: funny lump size in %s", model.name)
	}
	count := int(l.filelen) / sizeof_dmodel_t
	if count > MAX_MODELS {
		Host_Error("Mod_LoadSubmodels : count > MAX_MODELS")
	}

	model.numsubmodels = count

	if count == 0 {
		return nil
	}

	in := lumpData(l)
	out := make([]dmodel_t, count)

	for i := 0; i < count; i++ {
		b := in[i*sizeof_dmodel_t:]
		o := &out[i]
		for j := 0; j < 3; j++ { // spread the mins / maxs by a pixel
			o.mins[j] = leFloat(b[j*4:]) - 1
			o.maxs[j] = leFloat(b[12+j*4:]) + 1
			o.origin[j] = leFloat(b[24+j*4:])
		}
		for j := 0; j < MAX_MAP_HULLS; j++ {
			o.headnode[j] = leLong(b[36+j*4:])
		}
		o.visleafs = leLong(b[52:])
		o.firstface = leLong(b[56:])
		o.numfaces = leLong(b[60:])
	}

	return out
}

func Mod_SetParent(model *model_t, nodenum uint32, parentnum uint32) {
	if nodenum >= uint32(model.numnodes) {
		model.leafs[nodenum-uint32(model.numnodes)].parentnum = parentnum
		return
	}

	node := &model.nodes[nodenum]
	node.parentnum = parentnum

	Mod_SetParent(model, node.childrennum[0], nodenum)
	Mod_SetParent(model, node.childrennum[1], nodenum)
}

func Mod_LoadNodes(model *model_t, l *lump_t) {
	if l.filelen%sizeof_dnode_t != 0 {
		Host_Error("Mod_LoadNodes: funny lump size in %s", model.name)
	}
	count := int(l.filelen) / sizeof_dnode_t
	in := lumpData(l)
	out := make([]mnode_t, count)

	model.nodes = out
	model.numnodes = count

	for i := 0; i < count; i++ {
		b := in[i*sizeof_dnode_t:]
		o := &out[i]
		for j := 0; j < 3; j++ {
			o.minmaxs[j] = float32(leShort(b[8+j*2:]))
			o.minmaxs[3+j] = float32(leShort(b[14+j*2:]))
		}

		o.planenum = uint32(leLong(b))

		o.firstsurface = uint32(uint16(leShort(b[20:])))
		o.numsurfaces = uint32(uint16(leShort(b[22:])))

		for j := 0; j < 2; j++ {
			p := int(leShort(b[4+j*2:]))
			if p >= 0 {
				o.childrennum[j] = uint32(p)
			} else {
				o.childrennum[j] = uint32(model.numnodes + (-1 - p))
			}
		}
	}

	Mod_SetParent(model, 0, 0xffffffff) // sets nodes and leafs
}

func Mod_LoadNodesBSP2(model *model_t, l *lump_t) {
	if l.filelen%sizeof_dnode_bsp2_t != 0 {
		Host_Error("Mod_LoadNodesBSP2: funny lump size in %s", model.name)
	}
	count := int(l.filelen) / sizeof_dnode_bsp2_t
	in := lumpData(l)
	out := make([]mnode_t, count)

	model.nodes = out
	model.numnodes = count

	for i := 0; i < count; i++ {
		b := in[i*sizeof_dnode_bsp2_t:]
		o := &out[i]
		for j := 0; j < 3; j++ {
			o.minmaxs[j] = leFloat(b[12+j*4:])
			o.minmaxs[3+j] = leFloat(b[24+j*4:])
		}

		o.planenum = uint32(leLong(b))

		o.firstsurface = uint32(leLong(b[36:]))
		o.numsurfaces = uint32(leLong(b[40:]))

		for j := 0; j < 2; j++ {
			p := int(leLong(b[4+j*4:]))
			if p >= 0 {
				o.childrennum[j] = uint32(p)
			} else {
				o.childrennum[j] = uint32(model.numnodes + (-1 - p))
			}
		}
	}

	Mod_SetParent(model, 0, 0xffffffff) // sets nodes and leafs
}

func Mod_LoadLeafs(model *model_t, l *lump_t) {
	if l.filelen%sizeof_dleaf_t != 0 {
		Host_Error("Mod_LoadLeafs: funny lump size in %s", model.name)
	}
	count := int(l.filelen) / sizeof_dleaf_t
	in := lumpData(l)
	out := make([]mleaf_t, count)
	model.leafsolid = make([]uint32, (count+31)/32)

	model.leafs = out
	model.numleafs = count
	for i := 0; i < count; i++ {
		b := in[i*sizeof_dleaf_t:]
		o := &out[i]
		for j := 0; j < 3; j++ {
			o.minmaxs[j] = float32(leShort(b[8+j*2:]))
			o.minmaxs[3+j] = float32(leShort(b[14+j*2:]))
		}

		p := leLong(b)
		o.contents = int16(p)

		if p == CONTENTS_SOLID {
			model.leafsolid[i/32] |= 1 << (i % 32)
		}

		o.firstmarksurfacenum = uint32(uint16(leShort(b[20:])))
		o.nummarksurfaces = int(uint16(leShort(b[22:])))

		p = leLong(b[4:])
		if p == -1 || int(p) >= len(model.visdata) {
			o.compressed_vis = nil
		} else {
			o.compressed_vis = model.visdata[p:]
		}
		o.efrags = nil

		for j := 0; j < 4; j++ {
			o.ambient_sound_level[j] = b[24+j]
		}

		if !dedicated && o.contents != CONTENTS_EMPTY {
			for j := 0; j < o.nummarksurfaces; j++ {
				model.surfflags[model.marksurfaces[o.firstmarksurfacenum+uint32(j)]] |= SURF_UNDERWATER
			}
		}
	}
}

func Mod_LoadLeafsBSP2(model *model_t, l *lump_t) {
	if l.filelen%sizeof_dleaf_bsp2_t != 0 {
		Host_Error("Mod_LoadLeafsBSP2: funny lump size in %s", model.name)
	}
	count := int(l.filelen) / sizeof_dleaf_bsp2_t
	in := lumpData(l)
	out := make([]mleaf_t, count)
	model.leafsolid = make([]uint32, (count+31)/32)

	model.leafs = out
	model.numleafs = count
	for i := 0; i < count; i++ {
		b := in[i*sizeof_dleaf_bsp2_t:]
		o := &out[i]
		for j := 0; j < 3; j++ {
			o.minmaxs[j] = leFloat(b[8+j*4:])
			o.minmaxs[3+j] = leFloat(b[20+j*4:])
		}

		p := leLong(b)
		o.contents = int16(p)

		if p == CONTENTS_SOLID {
			model.leafsolid[i/32] |= 1 << (i % 32)
		}

		o.firstmarksurfacenum = uint32(leLong(b[32:]))
		o.nummarksurfaces = int(leLong(b[36:]))

		p = leLong(b[4:])
		if p == -1 || int(p) >= len(model.visdata) {
			o.compressed_vis = nil
		} else {
			o.compressed_vis = model.visdata[p:]
		}
		o.efrags = nil

		for j := 0; j < 4; j++ {
			o.ambient_sound_level[j] = b[40+j]
		}

		if !dedicated && o.contents != CONTENTS_EMPTY {
			for j := 0; j < o.nummarksurfaces; j++ {
				model.surfflags[model.marksurfaces[o.firstmarksurfacenum+uint32(j)]] |= SURF_UNDERWATER
			}
		}
	}
}

func mod_setHull(hull *hull_t, model *model_t, count int, mins, maxs vec3_t) {
	hull.clipnodes = model.clipnodes
	hull.firstclipnode = 0
	hull.lastclipnode = count - 1
	hull.planes = model.planes
	hull.clip_mins = mins
	hull.clip_maxs = maxs
}

func Mod_LoadClipnodes(model *model_t, l *lump_t) {
	if l.filelen%sizeof_dclipnode_t != 0 {
		Host_Error("Mod_LoadClipnodes: funny lump size in %s", model.name)
	}
	count := int(l.filelen) / sizeof_dclipnode_t
	in := lumpData(l)
	out := make([]mclipnode_t, count)

	model.clipnodes = out
	model.numclipnodes = count

	if model.bspversion == HL_BSPVERSION {
		mod_setHull(&model.hulls[1], model, count, vec3_t{-16, -16, -36}, vec3_t{16, 16, 36})
		mod_setHull(&model.hulls[2], model, count, vec3_t{-32, -32, -32}, vec3_t{32, 32, 32})
		mod_setHull(&model.hulls[3], model, count, vec3_t{-16, -16, -18}, vec3_t{16, 16, 18})
	} else {
		mod_setHull(&model.hulls[1], model, count, vec3_t{-16, -16, -24}, vec3_t{16, 16, 32})
		mod_setHull(&model.hulls[2], model, count, vec3_t{-32, -32, -24}, vec3_t{32, 32, 64})
	}

	for i := 0; i < count; i++ {
		b := in[i*sizeof_dclipnode_t:]
		out[i].planenum = int(leLong(b))
		out[i].children[0] = int(leShort(b[4:]))
		out[i].children[1] = int(leShort(b[6:]))
	}
}

func Mod_LoadClipnodesBSP2(model *model_t, l *lump_t) {
	if l.filelen%sizeof_dclipnode29a_t != 0 {
		Host_Error("Mod_LoadClipnodesBSP2: funny lump size in %s", model.name)
	}
	count := int(l.filelen) / sizeof_dclipnode29a_t
	in := lumpData(l)
	out := make([]mclipnode_t, count)

	model.clipnodes = out
	model.numclipnodes = count

	// BSP2 has no HL variant; hull dimensions follow vanilla Q1 values.
	mod_setHull(&model.hulls[1], model, count, vec3_t{-16, -16, -24}, vec3_t{16, 16, 32})
	mod_setHull(&model.hulls[2], model, count, vec3_t{-32, -32, -24}, vec3_t{32, 32, 64})

	for i := 0; i < count; i++ {
		b := in[i*sizeof_dclipnode29a_t:]
		out[i].planenum = int(leLong(b))
		out[i].children[0] = int(leLong(b[4:]))
		out[i].children[1] = int(leLong(b[8:]))
	}
}

// Deplicate the drawing hull structure as a clipping hull
func Mod_MakeHull0(model *model_t) {
	hull := &model.hulls[0]

	count := model.numnodes
	out := make([]mclipnode_t, count)

	hull.clipnodes = out
	hull.firstclipnode = 0
	hull.lastclipnode = count - 1
	hull.planes = model.planes

	for i := 0; i < count; i++ {
		in := &model.nodes[i]
		out[i].planenum = int(in.planenum)
		for j := 0; j < 2; j++ {
			if in.childrennum[j] >= uint32(model.numnodes) {
				out[i].children[j] = int(model.leafs[in.childrennum[j]-uint32(model.numnodes)].contents)
			} else {
				out[i].children[j] = int(in.childrennum[j])
			}
		}
	}
}

func Mod_LoadMarksurfaces(model *model_t, l *lump_t) {
	if l.filelen%sizeof_dmarksurface != 0 {
		Host_Error("Mod_LoadMarksurfaces: funny lump size in %s", model.name)
	}
	count := int(l.filelen) / sizeof_dmarksurface
	in := lumpData(l)
	out := make([]uint32, count)

	model.marksurfaces = out
	model.nummarksurfaces = count

	for i := 0; i < count; i++ {
		j := int(uint16(leShort(in[i*2:])))
		if j >= model.numsurfaces {
			Host_Error("Mod_LoadMarksurfaces: bad surface number")
		}
		out[i] = uint32(j)
	}
}

func Mod_LoadMarksurfacesBSP2(model *model_t, l *lump_t) {
	if l.filelen%sizeof_dmarksurface29a != 0 {
		Host_Error("Mod_LoadMarksurfacesBSP2: funny lump size in %s", model.name)
	}
	count := int(l.filelen) / sizeof_dmarksurface29a
	in := lumpData(l)
	out := make([]uint32, count)

	model.marksurfaces = out
	model.nummarksurfaces = count

	for i := 0; i < count; i++ {
		j := uint32(leLong(in[i*4:]))
		if int(j) >= model.numsurfaces {
			Host_Error("Mod_LoadMarksurfacesBSP2: bad surface number")
		}
		out[i] = j
	}
}

func Mod_LoadPlanes(model *model_t, l *lump_t) {
	if l.filelen%sizeof_dplane_t != 0 {
		Host_Error("Mod_LoadPlanes: funny lump size in %s", model.name)
	}
	count := int(l.filelen) / sizeof_dplane_t

	if count > 65535 {
		Sys_Error("Mod_LoadPlanes: count > 65535")
	}

	in := lumpData(l)
	out := make([]mplane_t, count)

	model.planes = out
	model.numplanes = count

	for i := 0; i < count; i++ {
		b := in[i*sizeof_dplane_t:]
		o := &out[i]
		bits := 0
		for j := 0; j < 3; j++ {
			o.normal[j] = leFloat(b[j*4:])
			if o.normal[j] < 0 {
				bits |= 1 << j
			}
		}

		o.dist = leFloat(b[12:])
		o.type_ = byte(leLong(b[16:]))
		o.signbits = byte(bits)
	}
}

func RadiusFromBounds(mins, maxs vec3_t) float32 {
	var corner vec3_t

	for i := 0; i < 3; i++ {
		if math.Abs(float64(mins[i])) > math.Abs(float64(maxs[i])) {
			corner[i] = float32(math.Abs(float64(mins[i])))
		} else {
			corner[i] = float32(math.Abs(float64(maxs[i])))
		}
	}

	return VectorLength(corner)
}

// Returns the data of a named BSPX lump (or nil).
func Mod_BSPX_FindLump(bspx *bspx_header_t, name string, plumpsize *int, mod_base []byte) []byte {
	if bspx == nil {
		return nil
	}

	for i := range bspx.lumps {
		lump := &bspx.lumps[i]
		if lump.lumpname == name {
			if plumpsize != nil {
				*plumpsize = int(lump.filelen)
			}
			return mod_base[lump.fileofs : lump.fileofs+lump.filelen]
		}
	}

	return nil
}

// Locate BSPX header past last BSP lump
func Mod_LoadBSPX(filesize int, mod_base []byte) *bspx_header_t {
	header := readHeader(mod_base)

	xofs := 0
	for i := 0; i < HEADER_LUMPS; i++ {
		end := int(header.lumps[i].fileofs + header.lumps[i].filelen)
		if end > xofs {
			xofs = end
		}
	}

	if xofs+sizeof_bspx_header_t > filesize {
		return nil
	}

	if string(mod_base[xofs:xofs+4]) != "BSPX" {
		return nil
	}

	xheader := &bspx_header_t{}
	xheader.numlumps = leLong(mod_base[xofs+4:])
	if xheader.numlumps < 0 || xofs+sizeof_bspx_header_t+int(xheader.numlumps)*sizeof_bspx_lump_t > filesize {
		return nil
	}

	xheader.lumps = make([]bspx_lump_t, xheader.numlumps)
	for i := range xheader.lumps {
		b := mod_base[xofs+sizeof_bspx_header_t+i*sizeof_bspx_lump_t:]
		lump := &xheader.lumps[i]
		lump.lumpname = cstr(b[:24])
		lump.fileofs = leLong(b[24:])
		lump.filelen = leLong(b[28:])
		if lump.fileofs < 0 || lump.filelen < 0 || uint32(lump.fileofs+lump.filelen) > uint32(filesize) {
			return nil
		}
	}

	return xheader
}

func Mod_LoadBrushModel(mod *model_t, buffer []byte) {
	mod.type_ = mod_brush

	header := readHeader(buffer)

	mod.bspversion = int(header.version)

	if mod.bspversion != Q1_BSPVERSION && mod.bspversion != HL_BSPVERSION && mod.bspversion != Q1_BSPVERSION2 {
		Host_Error("Mod_LoadBrushModel: %s has wrong version number (%d should be %d (Quake), %d (HalfLife) or %d (BSP2))", mod.name, mod.bspversion, Q1_BSPVERSION, HL_BSPVERSION, Q1_BSPVERSION2)
	}

	mod.isworldmodel = mod.name == va("maps/%s.bsp", mapname.string)

	mod_base = buffer

	bspx := Mod_LoadBSPX(com_filesize, mod_base)
	if bspx != nil {
		Com_DPrintf("BSPX header in %s: %d lumps\n", mod.name, bspx.numlumps)
		for j := range bspx.lumps {
			Com_DPrintf("  [%d] %s (%d bytes)\n", j, bspx.lumps[j].lumpname, bspx.lumps[j].filelen)
		}
	}

	if mod.isworldmodel {
		var bspformat string
		if mod.bspversion == Q1_BSPVERSION {
			bspformat = "BSP29"
		} else if mod.bspversion == HL_BSPVERSION {
			bspformat = "BSP30 (HalfLife)"
		} else if mod.bspversion == Q1_BSPVERSION2 {
			bspformat = "BSP2"
		} else if mod.bspversion == Q1_BSPVERSION29a {
			bspformat = "2PSB"
		} else {
			bspformat = "unknown"
		}

		if bspx != nil {
			s := "s"
			if bspx.numlumps == 1 {
				s = ""
			}
			Com_Printf("%s: %s + BSPX (%d lump%s)\n", mod.name, bspformat, bspx.numlumps, s)
		} else {
			Com_Printf("%s: %s\n", mod.name, bspformat)
		}
	}

	// checksum all of the map, except for entities
	mod.checksum = 0
	mod.checksum2 = 0

	for i := 0; i < HEADER_LUMPS; i++ {
		if i == LUMP_ENTITIES {
			continue
		}

		checksumvalue := Com_BlockChecksum(lumpData(&header.lumps[i]), int(header.lumps[i].filelen))

		mod.checksum ^= checksumvalue

		if i == LUMP_VISIBILITY || i == LUMP_LEAFS || i == LUMP_NODES {
			continue
		}

		mod.checksum2 ^= checksumvalue
	}

	// load into heap

	Mod_LoadEntities(mod, &header.lumps[LUMP_ENTITIES])
	if !dedicated {
		Mod_LoadVertexes(mod, &header.lumps[LUMP_VERTEXES])
		if mod.bspversion == Q1_BSPVERSION2 {
			Mod_LoadEdgesBSP2(mod, &header.lumps[LUMP_EDGES])
		} else {
			Mod_LoadEdges(mod, &header.lumps[LUMP_EDGES])
		}
		Mod_LoadSurfedges(mod, &header.lumps[LUMP_SURFEDGES])
		Mod_LoadTextures(mod, &header.lumps[LUMP_TEXTURES])
		Mod_LoadLighting(mod, &header.lumps[LUMP_LIGHTING], bspx)
	}
	Mod_LoadPlanes(mod, &header.lumps[LUMP_PLANES])
	if !dedicated {
		Mod_LoadTexinfo(mod, &header.lumps[LUMP_TEXINFO])
		if mod.bspversion == Q1_BSPVERSION2 {
			Mod_LoadFacesBSP2(mod, &header.lumps[LUMP_FACES])
			Mod_LoadMarksurfacesBSP2(mod, &header.lumps[LUMP_MARKSURFACES])
		} else {
			Mod_LoadFaces(mod, &header.lumps[LUMP_FACES])
			Mod_LoadMarksurfaces(mod, &header.lumps[LUMP_MARKSURFACES])
		}
	}
	Mod_LoadVisibility(mod, &header.lumps[LUMP_VISIBILITY])
	if mod.bspversion == Q1_BSPVERSION2 {
		Mod_LoadLeafsBSP2(mod, &header.lumps[LUMP_LEAFS])
		Mod_LoadNodesBSP2(mod, &header.lumps[LUMP_NODES])
		Mod_LoadClipnodesBSP2(mod, &header.lumps[LUMP_CLIPNODES])
	} else {
		Mod_LoadLeafs(mod, &header.lumps[LUMP_LEAFS])
		Mod_LoadNodes(mod, &header.lumps[LUMP_NODES])
		Mod_LoadClipnodes(mod, &header.lumps[LUMP_CLIPNODES])
	}
	subdmodels := Mod_LoadSubmodels(mod, &header.lumps[LUMP_MODELS])

	Mod_MakeHull0(mod)

	mod.numframes = 2 // regular and alternate animation

	if mod.numsubmodels != 0 {
		if mod.numsubmodels > 1 {
			mod.submodels = make([]model_t, mod.numsubmodels-1)
		} else {
			mod.submodels = nil
		}

		mainmodel := mod

		// set up the submodels (FIXME: this is confusing)
		for i := 0; i < mod.numsubmodels; i++ {
			bm := &subdmodels[i]

			mod.hulls[0].firstclipnode = int(bm.headnode[0])
			for j := 1; j < MAX_MAP_HULLS; j++ {
				mod.hulls[j].firstclipnode = int(bm.headnode[j])
				mod.hulls[j].lastclipnode = mod.numclipnodes - 1
			}

			mod.firstmodelsurface = int(bm.firstface)
			mod.nummodelsurfaces = int(bm.numfaces)

			mod.maxs = bm.maxs
			mod.mins = bm.mins

			mod.radius = RadiusFromBounds(mod.mins, mod.maxs)

			mod.numleafs = int(bm.visleafs)

			if i < mod.numsubmodels-1 {
				// duplicate the basic information

				nextmodel := &mod.submodels[i]
				*nextmodel = *mod
				nextmodel.name = fmt.Sprintf("%s*%d", mainmodel.name, i+1)
				mod = nextmodel
			}
		}
	}
}

/*
==============================================================================
ALIAS MODELS
==============================================================================
*/

// modelgen.h
const (
	ALIAS_VERSION = 6
	ALIAS_ONSEAM  = 0x0020

	ALIAS_SINGLE = 0
	ALIAS_GROUP  = 1

	ALIAS_SKIN_SINGLE = 0
	ALIAS_SKIN_GROUP  = 1

	DT_FACES_FRONT = 0x0010

	sizeof_mdl_t          = 84
	sizeof_stvert_t       = 12
	sizeof_dtriangle_t    = 16
	sizeof_trivertx_t     = 4
	sizeof_daliasframe_t  = 24
	sizeof_daliasgroup_t  = 12
	sizeof_daliasinterval = 4
)

// r_local.h
const (
	MAX_LBM_HEIGHT        = 480
	ALIAS_BASE_SIZE_RATIO = 1.0 / 11.0
)

type trivertx_t struct {
	v                [3]byte
	lightnormalindex byte
}

type stvert_t struct {
	onseam int
	s      int
	t      int
}

type mtriangle_t struct {
	facesfront int
	vertindex  [3]int
}

type maliasframedesc_t struct {
	firstpose int
	numposes  int
	interval  float32
	bboxmin   vec3_t
	bboxmax   vec3_t
	radius    float32
	name      string
}

const MAX_SKINS = 32

type aliashdr_t struct {
	scale          vec3_t
	scale_origin   vec3_t
	boundingradius float32
	eyeposition    vec3_t
	numskins       int
	skinwidth      int
	skinheight     int
	numverts       int
	numtris        int
	numframes      int
	size           float32

	// vertex array stuff, filled by R_MakeAliasModelDisplayLists
	totalverts    uint16
	collisions    uint32
	collisionmap  []uint16
	indices       []uint16
	indexmin      uint16
	indexmax      uint16
	texcoords     []float32
	realposeverts []trivertx_t // numverts * numposes

	numposes int

	gl_texturenum [MAX_SKINS][4]int
	fb_texturenum [MAX_SKINS][4]int
	frames        []maliasframedesc_t
}

const (
	MAXALIASVERTS  = 2048
	MAXALIASFRAMES = 256
	MAXALIASTRIS   = 2048
)

var stverts [MAXALIASVERTS]stvert_t
var triangles [MAXALIASTRIS]mtriangle_t

// a pose is a single set of vertexes.  a frame may be
// an animating sequence of poses; each entry is the file data from the
// pose's first trivertx_t on
var poseverts [MAXALIASFRAMES][]byte
var posenum int

func Mod_LoadAliasFrame(pheader *aliashdr_t, pin []byte, frame *maliasframedesc_t) []byte {
	frame.name = cstr(pin[8:24])
	frame.firstpose = posenum
	frame.numposes = 1

	for i := 0; i < 3; i++ {
		// these are byte values, so we don't have to worry about endianness
		frame.bboxmin[i] = float32(pin[i])*pheader.scale[i] + pheader.scale_origin[i]
		frame.bboxmax[i] = float32(pin[4+i])*pheader.scale[i] + pheader.scale_origin[i]
	}
	frame.radius = RadiusFromBounds(frame.bboxmin, frame.bboxmax)

	pinframe := pin[sizeof_daliasframe_t:]

	poseverts[posenum] = pinframe
	posenum++

	return pinframe[pheader.numverts*sizeof_trivertx_t:]
}

func Mod_LoadAliasGroup(pheader *aliashdr_t, pin []byte, frame *maliasframedesc_t) []byte {
	numframes := int(leLong(pin))

	frame.firstpose = posenum
	frame.numposes = numframes

	for i := 0; i < 3; i++ {
		// these are byte values, so we don't have to worry about endianness
		frame.bboxmin[i] = float32(pin[4+i])*pheader.scale[i] + pheader.scale_origin[i]
		frame.bboxmax[i] = float32(pin[8+i])*pheader.scale[i] + pheader.scale_origin[i]
	}
	frame.radius = RadiusFromBounds(frame.bboxmin, frame.bboxmax)

	pin_intervals := pin[sizeof_daliasgroup_t:]

	frame.interval = leFloat(pin_intervals)

	ptemp := pin_intervals[numframes*sizeof_daliasinterval:]

	for i := 0; i < numframes; i++ {
		poseverts[posenum] = ptemp[sizeof_daliasframe_t:]
		posenum++

		ptemp = ptemp[sizeof_daliasframe_t+pheader.numverts*sizeof_trivertx_t:]
	}

	return ptemp
}

//=========================================================

type floodfill_t struct {
	x, y int16
}

// must be a power of 2
const FLOODFILL_FIFO_SIZE = 0x1000
const FLOODFILL_FIFO_MASK = FLOODFILL_FIFO_SIZE - 1

// Fill background pixels so mipmapping doesn't have haloes - Ed
func Mod_FloodFillSkin(skin []byte, skinwidth, skinheight int) {
	fillcolor := skin[0] // assume this is the pixel to fill
	var fifo [FLOODFILL_FIFO_SIZE]floodfill_t
	inpt, outpt := 0, 0
	filledcolor := -1

	if filledcolor == -1 {
		filledcolor = 0
		// attempt to find opaque black
		for i := 0; i < 256; i++ {
			if d_8to24table[i] == 255<<0 { // alpha 1.0
				filledcolor = i
				break
			}
		}
	}

	// can't fill to filled color or to transparent color (used as visited marker)
	if int(fillcolor) == filledcolor || fillcolor == 255 {
		return
	}

	fifo[inpt].x, fifo[inpt].y = 0, 0
	inpt = (inpt + 1) & FLOODFILL_FIFO_MASK

	for outpt != inpt {
		x, y := int(fifo[outpt].x), int(fifo[outpt].y)
		fdc := filledcolor
		pos := x + skinwidth*y

		outpt = (outpt + 1) & FLOODFILL_FIFO_MASK

		step := func(off, dx, dy int) {
			if skin[pos+off] == fillcolor {
				skin[pos+off] = 255
				fifo[inpt].x, fifo[inpt].y = int16(x+dx), int16(y+dy)
				inpt = (inpt + 1) & FLOODFILL_FIFO_MASK
			} else if skin[pos+off] != 255 {
				fdc = int(skin[pos+off])
			}
		}

		if x > 0 {
			step(-1, -1, 0)
		}
		if x < skinwidth-1 {
			step(1, 1, 0)
		}
		if y > 0 {
			step(-skinwidth, 0, -1)
		}
		if y < skinheight-1 {
			step(skinwidth, 0, 1)
		}
		skin[x+skinwidth*y] = byte(fdc)
	}
}

func Mod_LoadExternalSkin(model *model_t, identifier string, fb_texnum *int) int {
	if model.modhint == MOD_EYES || model.modhint == MOD_BACKPACK {
		*fb_texnum = 0
		return 0
	}

	texmode := TEX_MIPMAP
	if gl_scaleModelTextures.value == 0 {
		texmode |= TEX_NOSCALE
	}

	loadpath := strlcpy(fmt.Sprintf("textures/models/%s", identifier), 64)
	texnum := R_LoadTextureImage(loadpath, identifier, 0, 0, texmode)

	if texnum == 0 {
		loadpath = strlcpy(fmt.Sprintf("textures/%s", identifier), 64)
		texnum = R_LoadTextureImage(loadpath, identifier, 0, 0, texmode)
	}

	*fb_texnum = 0
	return texnum
}

func Img_HasFullbrights(pixels []byte, size int) bool {
	for i := 0; i < size; i++ {
		if pixels[i] >= 224 {
			return true
		}
	}

	return false
}

func Mod_LoadAllSkins(model *model_t, pheader *aliashdr_t, numskins int, pskintype []byte) []byte {
	var gl_texnum, fb_texnum int

	// the C code takes this once, so every flood fill works on the first skin
	skin := pskintype[4:]

	if numskins < 1 || numskins > MAX_SKINS {
		Host_Error("Mod_LoadAllSkins: Invalid # of skins: %d\n", numskins)
	}

	s := pheader.skinwidth * pheader.skinheight

	basename := COM_CopyAndStripExtension(COM_SkipPath(model.name), 64)

	texmode := TEX_MIPMAP
	if gl_scaleModelTextures.value == 0 && !model.isworldmodel {
		texmode |= TEX_NOSCALE
	}

	for i := 0; i < numskins; i++ {
		if leLong(pskintype) == ALIAS_SKIN_SINGLE {
			Mod_FloodFillSkin(skin, pheader.skinwidth, pheader.skinheight)

			// save 8 bit texels for the player model to remap
			if model.modhint == MOD_PLAYER {
				Skin_SetDefault(pskintype[4:4+s], uint(pheader.skinwidth), uint(pheader.skinheight))
			}

			identifier := strlcpy(fmt.Sprintf("%s_%d", basename, i), 64)

			gl_texnum, fb_texnum = 0, 0
			if gl_texnum = Mod_LoadExternalSkin(model, identifier, &fb_texnum); gl_texnum == 0 {
				gl_texnum = R_LoadTexture(identifier, pheader.skinwidth, pheader.skinheight, pskintype[4:4+s], texmode, 1)

				if Img_HasFullbrights(pskintype[4:], pheader.skinwidth*pheader.skinheight) {
					fb_texnum = R_LoadTexture(va("@fb_%s", identifier), pheader.skinwidth, pheader.skinheight, pskintype[4:4+s], texmode|TEX_FULLBRIGHT, 1)
				}
			}

			for k := 0; k < 4; k++ {
				pheader.gl_texturenum[i][k] = gl_texnum
				pheader.fb_texturenum[i][k] = fb_texnum
			}

			pskintype = pskintype[4+s:]
		} else {
			// animating skin group.  yuck.
			pskintype = pskintype[4:]
			groupskins := int(leLong(pskintype))
			pinskinintervals := pskintype[4:]

			pskintype = pinskinintervals[groupskins*4:]

			j := 0
			for j = 0; j < groupskins; j++ {
				Mod_FloodFillSkin(skin, pheader.skinwidth, pheader.skinheight)

				identifier := strlcpy(fmt.Sprintf("%s_%d_%d", basename, i, j), 64)

				gl_texnum, fb_texnum = 0, 0
				if gl_texnum = Mod_LoadExternalSkin(model, identifier, &fb_texnum); gl_texnum == 0 {
					gl_texnum = R_LoadTexture(identifier, pheader.skinwidth, pheader.skinheight, pskintype[:s], texmode, 1)

					if Img_HasFullbrights(pskintype, pheader.skinwidth*pheader.skinheight) {
						fb_texnum = R_LoadTexture(va("@fb_%s", identifier), pheader.skinwidth, pheader.skinheight, pskintype[:s], texmode|TEX_FULLBRIGHT, 1)
					}
				}

				pheader.gl_texturenum[i][j&3] = gl_texnum
				pheader.fb_texturenum[i][j&3] = fb_texnum

				pskintype = pskintype[s:]
			}

			for k := j; j < 4; j++ {
				pheader.gl_texturenum[i][j&3] = pheader.gl_texturenum[i][j-k]
				pheader.fb_texturenum[i][j&3] = pheader.fb_texturenum[i][j-k]
			}
		}
	}

	return pskintype
}

//=========================================================================

func Mod_LoadAliasModel(mod *model_t, buffer []byte) {
	// some models are special
	if mod.name == "progs/player.mdl" {
		mod.modhint = MOD_PLAYER
	} else if mod.name == "progs/eyes.mdl" {
		mod.modhint = MOD_EYES
	} else if mod.name == "progs/flame.mdl" || mod.name == "progs/flame2.mdl" {
		mod.modhint = MOD_FLAME
	} else if mod.name == "progs/bolt.mdl" || mod.name == "progs/bolt2.mdl" || mod.name == "progs/bolt3.mdl" {
		mod.modhint = MOD_THUNDERBOLT
	} else if mod.name == "progs/backpack.mdl" {
		mod.modhint = MOD_BACKPACK
	} else {
		mod.modhint = MOD_NORMAL
	}

	if mod.modhint == MOD_PLAYER || mod.modhint == MOD_EYES {
		name := emodel_name
		if mod.modhint == MOD_PLAYER {
			name = pmodel_name
		}

		crc := CRC_Block(buffer[:com_filesize])
		st := fmt.Sprintf("%d", int(crc))
		cls.userinfo = Info_SetValueForKey(cls.userinfo, name, st, MAX_INFO_STRING)

		if cls.state >= ca_connected {
			if cls.netqw != nil {
				st = fmt.Sprintf("%csetinfo %s %d", clc_stringcmd, name, int(crc))

				if len(st) < 128 {
					NetQW_AppendReliableBuffer(cls.netqw, append([]byte(st), 0))
				}
			}
		}
	}

	pinmodel := buffer

	version := int(leLong(pinmodel[4:]))

	if version != ALIAS_VERSION {
		Host_Error("Mod_LoadAliasModel: %s has wrong version number (%d should be %d)\n", mod.name, version, ALIAS_VERSION)
		return
	}

	numframes := int(leLong(pinmodel[68:]))
	if numframes < 1 || numframes > 65535 {
		Host_Error("Mod_LoadAliasModel: Invalid number of alias frames")
	}

	// allocate space for a working header, plus all the data except the frames, skin and group info
	pheader := &aliashdr_t{frames: make([]maliasframedesc_t, numframes)}

	mod.flags = int(leLong(pinmodel[76:]))

	// endian-adjust and copy the data, starting with the alias model header
	pheader.boundingradius = leFloat(pinmodel[32:])
	pheader.numskins = int(leLong(pinmodel[48:]))
	pheader.skinwidth = int(leLong(pinmodel[52:]))
	pheader.skinheight = int(leLong(pinmodel[56:]))

	if pheader.skinheight > MAX_LBM_HEIGHT {
		Host_Error("Mod_LoadAliasModel: model %s has a skin taller than %d", mod.name, MAX_LBM_HEIGHT)
	}

	pheader.numverts = int(leLong(pinmodel[60:]))

	if pheader.numverts <= 0 {
		Host_Error("Mod_LoadAliasModel: model %s has no vertices", mod.name)
	}

	if pheader.numverts > MAXALIASVERTS {
		Host_Error("Mod_LoadAliasModel: model %s has too many vertices", mod.name)
	}

	pheader.numtris = int(leLong(pinmodel[64:]))

	if pheader.numtris <= 0 {
		Host_Error("Mod_LoadAliasModel: model %s has no triangles", mod.name)
	}

	pheader.numframes = numframes

	pheader.size = float32(float64(leFloat(pinmodel[80:])) * ALIAS_BASE_SIZE_RATIO)
	mod.synctype = synctype_t(leLong(pinmodel[72:]))
	mod.numframes = pheader.numframes

	for i := 0; i < 3; i++ {
		pheader.scale[i] = leFloat(pinmodel[8+i*4:])
		pheader.scale_origin[i] = leFloat(pinmodel[20+i*4:])
		pheader.eyeposition[i] = leFloat(pinmodel[36+i*4:])
	}

	// load the skins
	pskintype := pinmodel[sizeof_mdl_t:]
	pskintype = Mod_LoadAllSkins(mod, pheader, pheader.numskins, pskintype)

	// load base s and t vertices
	pinstverts := pskintype

	for i := 0; i < pheader.numverts; i++ {
		b := pinstverts[i*sizeof_stvert_t:]
		stverts[i].onseam = int(leLong(b))
		stverts[i].s = int(leLong(b[4:]))
		stverts[i].t = int(leLong(b[8:]))
	}

	// load triangle lists
	pintriangles := pinstverts[pheader.numverts*sizeof_stvert_t:]

	for i := 0; i < pheader.numtris; i++ {
		b := pintriangles[i*sizeof_dtriangle_t:]
		triangles[i].facesfront = int(leLong(b))

		for j := 0; j < 3; j++ {
			triangles[i].vertindex[j] = int(leLong(b[4+j*4:]))
		}
	}

	// load the frames
	posenum = 0
	pframetype := pintriangles[pheader.numtris*sizeof_dtriangle_t:]

	mod.mins = vec3_t{255, 255, 255}
	mod.maxs = vec3_t{0, 0, 0}

	for i := 0; i < numframes; i++ {
		frametype := leLong(pframetype)

		if frametype == ALIAS_SINGLE {
			pframetype = Mod_LoadAliasFrame(pheader, pframetype[4:], &pheader.frames[i])
		} else {
			pframetype = Mod_LoadAliasGroup(pheader, pframetype[4:], &pheader.frames[i])
		}

		for j := 0; j < 3; j++ {
			mod.mins[j] = min(mod.mins[j], pheader.frames[i].bboxmin[j])
			mod.maxs[j] = max(mod.maxs[j], pheader.frames[i].bboxmax[j])
		}
	}

	mod.radius = RadiusFromBounds(mod.mins, mod.maxs)

	pheader.numposes = posenum

	mod.type_ = mod_alias

	// build the draw lists
	R_MakeAliasModelDisplayLists(mod, pheader)

	mod.extradata = pheader
}

/*
==============================================================================
SPRITE MODELS
==============================================================================
*/

// spritegn.h
const (
	SPRITE_VERSION = 1

	SPR_VP_PARALLEL_UPRIGHT  = 0
	SPR_FACING_UPRIGHT       = 1
	SPR_VP_PARALLEL          = 2
	SPR_ORIENTED             = 3
	SPR_VP_PARALLEL_ORIENTED = 4

	sizeof_dsprite_t         = 36
	sizeof_dspriteframe_t    = 16
	sizeof_dspritegroup_t    = 4
	sizeof_dspriteinterval_t = 4
)

type spriteframetype_t int

const (
	SPR_SINGLE spriteframetype_t = iota
	SPR_GROUP
)

type mspriteframe_t struct {
	width         int
	height        int
	up, down      float32
	left, right   float32
	gl_texturenum int
}

type mspritegroup_t struct {
	numframes int
	intervals []float32
	frames    []*mspriteframe_t
}

// C keeps one pointer and casts it by type; here a group has its own field.
type mspriteframedesc_t struct {
	type_    spriteframetype_t
	frameptr *mspriteframe_t
	group    *mspritegroup_t
}

type msprite_t struct {
	type_      int
	maxwidth   int
	maxheight  int
	numframes  int
	beamlength float32 // remove?
	frames     []mspriteframedesc_t
}

func Mod_LoadExternalSpriteSkin(model *model_t, identifier string, framenum int) int {
	texmode := TEX_MIPMAP | TEX_ALPHA
	if gl_scaleModelTextures.value == 0 && !model.isworldmodel {
		texmode |= TEX_NOSCALE
	}

	loadpath := strlcpy(fmt.Sprintf("textures/sprites/%s", identifier), 64)
	texnum := R_LoadTextureImage(loadpath, identifier, 0, 0, texmode)

	if texnum == 0 {
		loadpath = strlcpy(fmt.Sprintf("textures/%s", identifier), 64)
		texnum = R_LoadTextureImage(loadpath, identifier, 0, 0, texmode)
	}

	return texnum
}

func Mod_LoadSpriteFrame(model *model_t, pin []byte, ppframe **mspriteframe_t, framenum int) []byte {
	var origin [2]int

	texmode := TEX_MIPMAP | TEX_ALPHA
	if gl_scaleModelTextures.value == 0 && !model.isworldmodel {
		texmode |= TEX_NOSCALE
	}

	basename := COM_CopyAndStripExtension(COM_SkipPath(model.name), 64)

	width := int(leLong(pin[8:]))
	height := int(leLong(pin[12:]))
	size := width * height

	pspriteframe := &mspriteframe_t{}

	*ppframe = pspriteframe

	pspriteframe.width = width
	pspriteframe.height = height
	origin[0] = int(leLong(pin))
	origin[1] = int(leLong(pin[4:]))

	pspriteframe.up = float32(origin[1])
	pspriteframe.down = float32(origin[1] - height)
	pspriteframe.left = float32(origin[0])
	pspriteframe.right = float32(width + origin[0])

	identifier := strlcpy(fmt.Sprintf("sprites/%s_%d", basename, framenum), 64)
	texnum := Mod_LoadExternalSpriteSkin(model, identifier, framenum)
	if texnum == 0 {
		texnum = R_LoadTexture(identifier, width, height, pin[sizeof_dspriteframe_t:sizeof_dspriteframe_t+size], texmode, 1)
	}

	pspriteframe.gl_texturenum = texnum

	return pin[sizeof_dspriteframe_t+size:]
}

func Mod_LoadSpriteGroup(model *model_t, pin []byte, pgroup **mspritegroup_t, framenum int) []byte {
	numframes := int(leLong(pin))
	if numframes < 1 || numframes >= 65536 {
		Host_Error("Invalid number of sprite frames")
	}

	pspritegroup := &mspritegroup_t{frames: make([]*mspriteframe_t, numframes)}

	pspritegroup.numframes = numframes

	*pgroup = pspritegroup

	pin_intervals := pin[sizeof_dspritegroup_t:]

	poutintervals := make([]float32, numframes)

	pspritegroup.intervals = poutintervals

	for i := 0; i < numframes; i++ {
		poutintervals[i] = leFloat(pin_intervals[i*sizeof_dspriteinterval_t:])
		if poutintervals[i] <= 0.0 {
			Host_Error("Mod_LoadSpriteGroup: interval <= 0")
		}
	}

	ptemp := pin_intervals[numframes*sizeof_dspriteinterval_t:]

	for i := 0; i < numframes; i++ {
		ptemp = Mod_LoadSpriteFrame(model, ptemp, &pspritegroup.frames[i], framenum*100+i)
	}

	return ptemp
}

func Mod_LoadSpriteModel(mod *model_t, buffer []byte) {
	pin := buffer

	version := int(leLong(pin[4:]))

	if version != SPRITE_VERSION {
		Host_Error("Mod_LoadSpriteModel: %s has wrong version number (%d should be %d)", mod.name, version, SPRITE_VERSION)
		return
	}

	numframes := int(leLong(pin[24:]))

	psprite := &msprite_t{}
	if numframes >= 1 {
		psprite.frames = make([]mspriteframedesc_t, numframes)
	}

	mod.extradata = psprite

	psprite.type_ = int(leLong(pin[8:]))
	psprite.maxwidth = int(leLong(pin[16:]))
	psprite.maxheight = int(leLong(pin[20:]))
	psprite.beamlength = leFloat(pin[28:])
	mod.synctype = synctype_t(leLong(pin[32:]))
	psprite.numframes = numframes

	mod.mins[0] = float32(-psprite.maxwidth / 2)
	mod.mins[1] = mod.mins[0]
	mod.maxs[0] = float32(psprite.maxwidth / 2)
	mod.maxs[1] = mod.maxs[0]
	mod.mins[2] = float32(-psprite.maxheight / 2)
	mod.maxs[2] = float32(psprite.maxheight / 2)

	// load the frames
	if numframes < 1 {
		Host_Error("Mod_LoadSpriteModel: Invalid # of frames: %d\n", numframes)
	}

	mod.numframes = numframes

	pframetype := pin[sizeof_dsprite_t:]

	for i := 0; i < numframes; i++ {
		frametype := spriteframetype_t(leLong(pframetype))
		psprite.frames[i].type_ = frametype

		if frametype == SPR_SINGLE {
			pframetype = Mod_LoadSpriteFrame(mod, pframetype[4:], &psprite.frames[i].frameptr, i)
		} else {
			pframetype = Mod_LoadSpriteGroup(mod, pframetype[4:], &psprite.frames[i].group, i)
		}
	}

	mod.type_ = mod_sprite
}

func Mod_Init() {
	for i := range mod_novis {
		mod_novis[i] = 0xff
	}
}

func Mod_Shutdown() {
	Mod_ClearAll()
}

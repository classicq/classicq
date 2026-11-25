package engine

// gpu_alias.c

import (
	"unsafe"

	"github.com/classicq/classicq/internal/sdl"
)

const (
	NUMVERTEXNORMALS = 162
	SHADEDOT_QUANT   = 64
)

var shadedots = &r_avertexnormal_dots[0]

var r_framelerp float32
var r_lerpdistance float32
var shadelight, ambientlight float32
var full_light bool

// viewmodel batches squash depth to 0..0.3 so the gun doesn't poke into walls
var r_viewmodelpass bool

func Mat4_Scale(m []float32, x, y, z float32) {
	for i := 0; i < 4; i++ {
		m[i] *= x
		m[4+i] *= y
		m[8+i] *= z
	}
}

// ---- sprites ----

func R_GetSpriteFrame(e *entity_t) *mspriteframe_t {
	psprite := e.model.extradata.(*msprite_t)
	frame := e.frame

	if frame >= psprite.numframes || frame < 0 {
		Com_Printf("R_GetSpriteFrame: no such frame %d\n", frame)
		frame = 0
	}

	if psprite.frames[frame].type_ == SPR_SINGLE {
		return psprite.frames[frame].frameptr
	}

	pspritegroup := psprite.frames[frame].group
	pintervals := pspritegroup.intervals
	numframes := pspritegroup.numframes
	fullinterval := pintervals[numframes-1]

	time := float32(cl.time)

	// intervals are all positive, so no division by 0
	targettime := time - float32(int(time/fullinterval))*fullinterval

	i := 0
	for ; i < numframes-1; i++ {
		if pintervals[i] > targettime {
			break
		}
	}

	return pspritegroup.frames[i]
}

func R_DrawSpriteModel(e *entity_t) {
	var right, up vec3_t
	var pointup, pointdown vec3_t
	var firstvert, firstindex uint32

	currententity = e

	frame := R_GetSpriteFrame(e)
	psprite := e.model.extradata.(*msprite_t)

	if psprite.type_ == SPR_ORIENTED {
		// bullet marks on walls
		AngleVectors(e.angles, nil, &right, &up)
	} else if psprite.type_ == SPR_FACING_UPRIGHT {
		up = vec3_t{0, 0, 1}
		right[0] = e.origin[1] - r_origin[1]
		right[1] = -(e.origin[0] - r_origin[0])
		right[2] = 0
		VectorNormalizeFast(&right)
	} else if psprite.type_ == SPR_VP_PARALLEL_UPRIGHT {
		up = vec3_t{0, 0, 1}
		right = vright
	} else {
		// normal sprite
		up = vup
		right = vright
	}

	v := Scene_AllocVerts(4, &firstvert)
	if v == nil {
		return
	}
	idx := Scene_AllocIndices(6, &firstindex)
	if idx == nil {
		return
	}

	VectorMA(e.origin, frame.down, up, &pointdown)
	VectorMA(e.origin, frame.up, up, &pointup)

	VectorMA(pointdown, frame.left, right, (*vec3_t)(&v[0].pos))
	v[0].st = [2]float32{0, 1}

	VectorMA(pointup, frame.left, right, (*vec3_t)(&v[1].pos))
	v[1].st = [2]float32{0, 0}

	VectorMA(pointup, frame.right, right, (*vec3_t)(&v[2].pos))
	v[2].st = [2]float32{1, 0}

	VectorMA(pointdown, frame.right, right, (*vec3_t)(&v[3].pos))
	v[3].st = [2]float32{1, 1}

	for i := 0; i < 4; i++ {
		v[i].lm = [2]float32{}
		v[i].rgba = [4]uint8{255, 255, 255, 255}
	}

	idx[0] = firstvert
	idx[1] = firstvert + 1
	idx[2] = firstvert + 2
	idx[3] = firstvert
	idx[4] = firstvert + 2
	idx[5] = firstvert + 3

	Scene_AddBatch(SCENE_PIPE_TEX_ALPHATEST, frame.gl_texturenum, nil, GPU_GetDynamicSceneVB(), firstindex, 6, r_viewproj[:])
}

// ---- alias models ----

func R_AliasSetupFullLight(model *model_t) {
	if model.modhint == MOD_THUNDERBOLT ||
		model.modhint == MOD_FLAME ||
		(model.modhint == MOD_PLAYER && bound(0, r_fullbrightSkins.value, cl.fbskins) != 0) {
		full_light = true
	} else {
		full_light = false
	}
}

func R_AliasSetupLighting(ent *entity_t) {
	var dist vec3_t

	clmodel := ent.model

	// make thunderbolt and torches full light
	if clmodel.modhint == MOD_THUNDERBOLT {
		ambientlight = 60 + 150*bound(0, gl_shaftlight.value, 1)
		shadelight = 0
		return
	} else if clmodel.modhint == MOD_FLAME {
		ambientlight = 255
		shadelight = 0
		return
	}

	ambientlight = float32(R_LightPoint(ent.origin))
	shadelight = ambientlight

	for i := uint(0); i < MAX_DLIGHTS/32; i++ {
		if cl_dlight_active[i] != 0 {
			for j := uint(0); j < 32; j++ {
				if cl_dlight_active[i]&(1<<j) != 0 && i*32+j < MAX_DLIGHTS {
					lnum := int(i*32 + j)

					VectorSubtract(ent.origin, cl_dlights[lnum].origin, &dist)
					add := cl_dlights[lnum].radius - VectorLength(dist)

					if add > 0 {
						ambientlight += add
					}
				}
			}
		}
	}

	// clamp lighting so it doesn't overbright as much
	if ambientlight > 128 {
		ambientlight = 128
	}
	if ambientlight+shadelight > 192 {
		shadelight = 192 - ambientlight
	}

	// always give the gun some light
	if ent.flags&RF_WEAPONMODEL != 0 && ambientlight < 24 {
		ambientlight, shadelight = 24, 24
	}

	// never allow players to go totally black
	if clmodel.modhint == MOD_PLAYER {
		if ambientlight < 8 {
			ambientlight, shadelight = 8, 8
		}

		fbskins := bound(0, r_fullbrightSkins.value, cl.fbskins)
		if fbskins != 0 {
			ambientlight = max(ambientlight, 8+fbskins*120)
			shadelight = max(shadelight, 8+fbskins*120)
		}
	}

	minlight := float32(cl.minlight)

	if ambientlight < minlight {
		ambientlight, shadelight = minlight, minlight
	}
}

// VectorL2Compare on packed byte vertices: the squared distance is an int in C
func trivertxL2Compare(v, w [3]byte, m float32) bool {
	d0 := int(v[0]) - int(w[0])
	d1 := int(v[1]) - int(w[1])
	d2 := int(v[2]) - int(w[2])
	return float32(d0*d0+d1*d1+d2*d2) < m*m
}

// FloatInterpolate on shade table bytes
func shadeInterpolate(f1 byte, frac float32, f2 byte) float32 {
	return float32(f1) + frac*float32(int(f2)-int(f1))
}

func R_DrawAliasModel(ent *entity_t) {
	var mins, maxs vec3_t
	var firstvert, firstindex uint32
	var entmatrix, mvp [16]float32

	currententity = ent

	clmodel := ent.model
	paliashdr := Mod_Extradata(ent.model).(*aliashdr_t)

	if ent.frame >= paliashdr.numframes || ent.frame < 0 {
		Com_DPrintf("R_DrawAliasModel: no such frame %d\n", ent.frame)
		ent.frame = 0
	}
	if ent.oldframe >= paliashdr.numframes || ent.oldframe < 0 {
		Com_DPrintf("R_DrawAliasModel: no such oldframe %d\n", ent.oldframe)
		ent.oldframe = 0
	}

	frame := &paliashdr.frames[ent.frame]
	oldframe := &paliashdr.frames[ent.oldframe]

	if r_lerpframes.value == 0 || ent.framelerp < 0 || ent.oldframe == ent.frame {
		r_framelerp = 1.0
	} else {
		r_framelerp = min(ent.framelerp, 1)
	}

	// culling
	if ent.flags&RF_WEAPONMODEL == 0 {
		if ent.angles[0] != 0 || ent.angles[1] != 0 || ent.angles[2] != 0 {
			if R_CullSphere(ent.origin, max(oldframe.radius, frame.radius)) {
				return
			}
		} else {
			if r_framelerp == 1 {
				VectorAdd(ent.origin, frame.bboxmin, &mins)
				VectorAdd(ent.origin, frame.bboxmax, &maxs)
			} else {
				for i := 0; i < 3; i++ {
					mins[i] = ent.origin[i] + min(oldframe.bboxmin[i], frame.bboxmin[i])
					maxs[i] = ent.origin[i] + max(oldframe.bboxmax[i], frame.bboxmax[i])
				}
			}
			if R_CullBox(mins, maxs) {
				return
			}
		}
	}

	R_AliasSetupFullLight(clmodel)
	R_AliasSetupLighting(ent)

	shadedots = &r_avertexnormal_dots[int(float64(ent.angles[1])*(SHADEDOT_QUANT/360.0))&(SHADEDOT_QUANT-1)]

	c_alias_polys += paliashdr.numtris

	// entity transform, same order as the glTranslatef/glRotatef sequence
	Mat4_Identity(entmatrix[:])
	Mat4_Translate(entmatrix[:], ent.origin[0], ent.origin[1], ent.origin[2])
	Mat4_RotateZ(entmatrix[:], ent.angles[1])
	Mat4_RotateY(entmatrix[:], -ent.angles[0])
	Mat4_RotateX(entmatrix[:], ent.angles[2])

	if clmodel.modhint == MOD_EYES {
		// double size of eyes, since they are really hard to see in gl
		Mat4_Translate(entmatrix[:], paliashdr.scale_origin[0], paliashdr.scale_origin[1], paliashdr.scale_origin[2]-30)
		Mat4_Scale(entmatrix[:], paliashdr.scale[0]*2, paliashdr.scale[1]*2, paliashdr.scale[2]*2)
	} else if ent.flags&RF_WEAPONMODEL != 0 {
		scale := float32(0.5 + float64(bound(0, r_viewmodelsize.value, 1)/2))
		Mat4_Translate(entmatrix[:], paliashdr.scale_origin[0], paliashdr.scale_origin[1], paliashdr.scale_origin[2])
		Mat4_Scale(entmatrix[:], paliashdr.scale[0]*scale, paliashdr.scale[1], paliashdr.scale[2])
	} else {
		Mat4_Translate(entmatrix[:], paliashdr.scale_origin[0], paliashdr.scale_origin[1], paliashdr.scale_origin[2])
		Mat4_Scale(entmatrix[:], paliashdr.scale[0], paliashdr.scale[1], paliashdr.scale[2])
	}

	Mat4_Multiply(r_viewproj[:], entmatrix[:], mvp[:])

	anim := int(cl.time*10) & 3
	skinnum := ent.skinnum
	if skinnum >= paliashdr.numskins || skinnum < 0 {
		Com_DPrintf("R_DrawAliasModel: no such skin # %d\n", skinnum)
		skinnum = 0
	}

	texture := paliashdr.gl_texturenum[skinnum][anim]
	fb_texture := paliashdr.fb_texturenum[skinnum][anim]

	var modelalpha float32 = 1
	if ent.flags&RF_WEAPONMODEL != 0 {
		modelalpha = bound(0, cl_drawgun.value, 1)
	}

	if ent.scoreboard != nil {
		// C takes the player index from the pointer difference
		for i := 0; i < MAX_CLIENTS; i++ {
			if ent.scoreboard != &cl.players[i] {
				continue
			}
			skinimp := Skin_GetTranslation(cl.players[i].skin, uint(cl.players[i].topcolor), uint(cl.players[i].bottomcolor))
			if skinimp != nil {
				texture = skinimp.texid
				fb_texture = skinimp.fbtexid
			}
			break
		}
	}

	if full_light || gl_fb_models.value == 0 {
		fb_texture = 0
	}

	// pose selection
	pose1 := oldframe.firstpose
	if oldframe.numposes > 1 {
		pose1 += int(cl.time/float64(oldframe.interval)) % oldframe.numposes
	}

	pose2 := frame.firstpose
	if frame.numposes > 1 {
		pose2 += int(cl.time/float64(frame.interval)) % frame.numposes
	}

	numverts := paliashdr.numverts
	numindices := uint32(paliashdr.numtris * 3)

	v := Scene_AllocVerts(uint32(paliashdr.totalverts), &firstvert)
	if v == nil {
		return
	}
	idx := Scene_AllocIndices(numindices, &firstindex)
	if idx == nil {
		return
	}

	verts1 := paliashdr.realposeverts[pose1*numverts:]
	verts2 := paliashdr.realposeverts[pose2*numverts:]

	alphabyte := byte(bound(0, modelalpha*255, 255))

	for i := 0; i < numverts; i++ {
		lerpfrac := r_framelerp
		if ent.flags&RF_LIMITLERP != 0 {
			if trivertxL2Compare(verts1[i].v, verts2[i].v, r_lerpdistance) {
				lerpfrac = r_framelerp
			} else {
				lerpfrac = 1
			}
		}

		for k := 0; k < 3; k++ {
			v[i].pos[k] = float32(verts1[i].v[k]) + lerpfrac*float32(int(verts2[i].v[k])-int(verts1[i].v[k]))
		}

		l := float32(float64(shadeInterpolate(shadedots[verts1[i].lightnormalindex], lerpfrac, shadedots[verts2[i].lightnormalindex])) / 127.0)
		l = l*shadelight + ambientlight
		l = min(l, 255)
		l = max(l, 0)
		lc := byte(l)

		v[i].st[0] = paliashdr.texcoords[i*2+0]
		v[i].st[1] = paliashdr.texcoords[i*2+1]
		v[i].lm = [2]float32{}
		v[i].rgba = [4]uint8{lc, lc, lc, alphabyte}
	}

	// duplicated verts for the seam, texcoords carry the +skinwidth/2 shift
	for i := 0; i < int(paliashdr.collisions); i++ {
		v[numverts+i] = v[paliashdr.collisionmap[i]]
		v[numverts+i].st[0] = paliashdr.texcoords[(numverts+i)*2+0]
		v[numverts+i].st[1] = paliashdr.texcoords[(numverts+i)*2+1]
	}

	for i := 0; i < int(numindices); i++ {
		idx[i] = firstvert + uint32(paliashdr.indices[i])
	}

	var fbtex *sdl.GPUTexture
	if fb_texture != 0 {
		fbtex = GPU_Texture_Lookup(fb_texture, nil)
	}
	var b *scene_batch_t
	if fbtex != nil {
		b = Scene_AddBatch(SCENE_PIPE_ALIAS_FB, texture, fbtex, GPU_GetDynamicSceneVB(), firstindex, numindices, mvp[:])
	} else {
		b = Scene_AddBatch(SCENE_PIPE_TEX_BLEND, texture, nil, GPU_GetDynamicSceneVB(), firstindex, numindices, mvp[:])
	}

	if b != nil && r_viewmodelpass {
		b.depth_min = 0.0
		b.depth_max = 0.3
	}
}

// ---- viewmodel ----

var r_viewmodel_gun entity_t

func R_DrawViewModel() {
	if r_drawentities.value == 0 || cl.viewent.current.modelindex == 0 {
		return
	}

	gun := &r_viewmodel_gun
	*gun = entity_t{}
	cent := &cl.viewent
	currententity = gun

	// skip on stale weapon modelindex during map-change
	if cent.current.modelindex >= MAX_MODELS {
		return
	}
	gun.model = cl.model_precache[cent.current.modelindex]
	if gun.model == nil {
		return
	}

	gun.origin = cent.current.origin
	gun.angles = cent.current.angles
	gun.flags = RF_WEAPONMODEL | RF_NOSHADOW
	if r_lerpmuzzlehack.value != 0 {
		if cent.current.modelindex != cl_modelindices[mi_vaxe] &&
			cent.current.modelindex != cl_modelindices[mi_vbio] &&
			cent.current.modelindex != cl_modelindices[mi_vgrap] &&
			cent.current.modelindex != cl_modelindices[mi_vknife] &&
			cent.current.modelindex != cl_modelindices[mi_vknife2] &&
			cent.current.modelindex != cl_modelindices[mi_vmedi] &&
			cent.current.modelindex != cl_modelindices[mi_vspan] {
			gun.flags |= RF_LIMITLERP
			r_lerpdistance = 135
		}
	}

	gun.frame = cent.current.frame
	if cent.frametime >= 0 && cent.frametime <= cl.time {
		gun.oldframe = cent.oldframe
		gun.framelerp = float32((cl.time - cent.frametime) * 10)
	} else {
		gun.oldframe = gun.frame
		gun.framelerp = -1
	}

	r_viewmodelpass = true
	R_DrawAliasModel(gun)
	r_viewmodelpass = false
}

// ---- player skin textures via the GPU texture table ----

// r_skinimp.h
type SkinImp struct {
	texid   int
	fbtexid int
}

func u32Bytes(s []uint32) []byte {
	if len(s) == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(&s[0])), len(s)*4)
}

func SkinImp_CreateSolidColour(colours []float32) *SkinImp {
	var tex [4]byte

	tex[0] = byte(colours[0] * 255)
	tex[1] = byte(colours[1] * 255)
	tex[2] = byte(colours[2] * 255)
	tex[3] = 255

	skinimp := &SkinImp{}
	skinimp.texid = texture_extension_number
	texture_extension_number++
	skinimp.fbtexid = 0
	GPU_Texture_Set(skinimp.texid, GPU_CreateTextureRGBA(tex[:], 1, 1, 0), 1, 1, GPU_PrefsForTexturemode())

	return skinimp
}

func SkinImp_CreateTexturePaletted(data []byte, width, height, modulo uint) *SkinImp {
	skinmem := make([]uint32, width*height)

	dofullbright := false
	src, dst := 0, 0
	for y := uint(0); y < height; y++ {
		for x := uint(0); x < width; x++ {
			if data[src+int(x)] >= 224 {
				dofullbright = true
			}
			skinmem[dst+int(x)] = d_8to24table[data[src+int(x)]]
		}

		src += int(modulo)
		dst += int(width)
	}

	skinimp := &SkinImp{}
	skinimp.texid = texture_extension_number
	texture_extension_number++
	skinimp.fbtexid = 0
	GPU_Texture_Set(skinimp.texid, GPU_CreateTextureRGBA(u32Bytes(skinmem), uint32(width), uint32(height), 0), uint32(width), uint32(height), GPU_PrefsForTexturemode())

	if dofullbright {
		src, dst = 0, 0
		for y := uint(0); y < height; y++ {
			for x := uint(0); x < width; x++ {
				if data[src+int(x)] < 224 {
					skinmem[dst+int(x)] = 0
				}
			}

			src += int(modulo)
			dst += int(width)
		}

		skinimp.fbtexid = texture_extension_number
		texture_extension_number++
		GPU_Texture_Set(skinimp.fbtexid, GPU_CreateTextureRGBA(u32Bytes(skinmem), uint32(width), uint32(height), 0), uint32(width), uint32(height), GPU_PrefsForTexturemode())
	}

	return skinimp
}

func SkinImp_CreateTextureTruecolour(data []byte, width, height uint) *SkinImp {
	skinimp := &SkinImp{}
	skinimp.texid = texture_extension_number
	texture_extension_number++
	skinimp.fbtexid = 0
	GPU_Texture_Set(skinimp.texid, GPU_CreateTextureRGBA(data, uint32(width), uint32(height), 0), uint32(width), uint32(height), GPU_PrefsForTexturemode())

	return skinimp
}

func SkinImp_Destroy(skinimp *SkinImp) {
	GPU_Texture_Set(skinimp.texid, nil, 0, 0, 0)
	if skinimp.fbtexid != 0 {
		GPU_Texture_Set(skinimp.fbtexid, nil, 0, 0, 0)
	}
}

package engine

// gpu_part.c

import "math"

// GL blend factor values, kept as opaque type tags in the QMB tables
const (
	GL_ZERO                = 0
	GL_ONE                 = 1
	GL_ONE_MINUS_SRC_COLOR = 0x0301
	GL_SRC_ALPHA           = 0x0302
	GL_ONE_MINUS_SRC_ALPHA = 0x0303
)

// ---- run batcher, merges contiguous (pipe, texture) emits into one batch ----

var run_pipe = -1
var run_tex int
var run_firstindex uint32
var run_numindices uint32

func Run_Flush() {
	if run_numindices != 0 {
		Scene_AddBatch(run_pipe, run_tex, nil, GPU_GetDynamicSceneVB(), run_firstindex, run_numindices, r_viewproj[:])
	}
	run_numindices = 0
	run_pipe = -1
}

func Run_Select(pipe, tex int) {
	if pipe != run_pipe || tex != run_tex {
		Run_Flush()
		run_pipe = pipe
		run_tex = tex
	}
}

func Run_Indices(count uint32) []uint32 {
	var firstindex uint32

	idx := Scene_AllocIndices(count, &firstindex)
	if idx == nil {
		return nil
	}
	if run_numindices == 0 {
		run_firstindex = firstindex
	}
	run_numindices += count
	return idx
}

// untextured triangle fan, center + ring
func Run_EmitFan(center vec3_t, ccol, ecol [4]byte, edges []vec3_t, numedges int) {
	var firstvert uint32

	v := Scene_AllocVerts(uint32(1+numedges), &firstvert)
	if v == nil {
		return
	}
	idx := Run_Indices(uint32(3 * (numedges - 1)))
	if idx == nil {
		return
	}

	v[0].pos = center
	v[0].rgba = ccol
	for j := 0; j < numedges; j++ {
		v[1+j].pos = edges[j]
		v[1+j].rgba = ecol
	}
	for j := 0; j < 1+numedges; j++ {
		v[j].st = [2]float32{}
		v[j].lm = [2]float32{}
	}
	for j := 0; j < numedges-1; j++ {
		idx[j*3+0] = firstvert
		idx[j*3+1] = firstvert + 1 + uint32(j)
		idx[j*3+2] = firstvert + 2 + uint32(j)
	}
}

// texrect = {s1, t1, s2, t2}; uv order matches the GL quad
func Run_EmitQuad(corners []vec3_t, texrect []float32, col [4]byte) {
	var firstvert uint32

	v := Scene_AllocVerts(4, &firstvert)
	if v == nil {
		return
	}
	idx := Run_Indices(6)
	if idx == nil {
		return
	}

	for j := 0; j < 4; j++ {
		v[j].pos = corners[j]
		v[j].rgba = col
		v[j].lm = [2]float32{}
	}
	v[0].st = [2]float32{texrect[0], texrect[3]}
	v[1].st = [2]float32{texrect[0], texrect[1]}
	v[2].st = [2]float32{texrect[2], texrect[1]}
	v[3].st = [2]float32{texrect[2], texrect[3]}

	idx[0] = firstvert
	idx[1] = firstvert + 1
	idx[2] = firstvert + 2
	idx[3] = firstvert
	idx[4] = firstvert + 2
	idx[5] = firstvert + 3
}

// ---- classic particles (gl_part.c) ----

var r_partscale float32
var part_up, part_right vec3_t
var part_firstvert uint32
var part_numverts uint32
var part_full bool

func R_DrawParticleInit() {
}

func R_DrawParticleBegin() {
	r_partscale = float32(0.004 * math.Tan(float64(r_refdef.fov_x)*(float64(math.Pi)/180)*0.5))

	VectorScale(vup, 1.5, &part_up)
	VectorScale(vright, 1.5, &part_right)

	part_numverts = 0
	part_full = false
}

func R_DrawParticle(p *particle_t) {
	var firstvert uint32

	if part_full {
		return
	}

	v := Scene_AllocVerts(3, &firstvert)
	if v == nil {
		part_full = true
		return
	}
	if part_numverts == 0 {
		part_firstvert = firstvert
	}
	part_numverts += 3

	// hack a scale up to keep particles from disapearing
	dist := (p.org[0]-r_origin[0])*vpn[0] + (p.org[1]-r_origin[1])*vpn[1] + (p.org[2]-r_origin[2])*vpn[2]
	scale := 1 + dist*r_partscale

	c := d_8to24table[int(p.color)]
	at := [3]byte{byte(c), byte(c >> 8), byte(c >> 16)}
	var theAlpha byte
	if p.type_ == pt_fire {
		theAlpha = byte(255 * (6 - p.ramp) / 6)
	} else {
		theAlpha = 255
	}

	v[0].pos = p.org
	v[0].st = [2]float32{0, 0}

	VectorMA(p.org, scale, part_up, (*vec3_t)(&v[1].pos))
	v[1].st = [2]float32{1, 0}

	VectorMA(p.org, scale, part_right, (*vec3_t)(&v[2].pos))
	v[2].st = [2]float32{0, 1}

	for j := 0; j < 3; j++ {
		v[j].lm = [2]float32{}
		v[j].rgba = [4]uint8{at[0], at[1], at[2], theAlpha}
	}
}

func R_DrawParticleEnd() {
	var firstindex uint32

	if part_numverts == 0 {
		return
	}

	idx := Scene_AllocIndices(part_numverts, &firstindex)
	if idx == nil {
		return
	}
	for i := uint32(0); i < part_numverts; i++ {
		idx[i] = part_firstvert + i
	}

	Scene_AddBatch(SCENE_PIPE_TEX_BLEND, particletexture, nil, GPU_GetDynamicSceneVB(), firstindex, part_numverts, r_viewproj[:])
}

// ---- QMB particles (gl_rpart.c) ----

// the classic limits in r_part.go keep the C names
const (
	qmb_default_num_particles  = 4096
	qmb_absolute_min_particles = 256
	qmb_absolute_max_particles = 32768
)

type col_t [4]byte

type part_type_t int

const (
	p_spark part_type_t = iota
	p_smoke
	p_fire
	p_bubble
	p_lavasplash
	p_gunblast
	p_chunk
	p_shockwave
	p_inferno_flame
	p_inferno_trail
	p_sparkray
	p_staticbubble
	p_trailpart
	p_dpsmoke
	p_dpfire
	p_teleflare
	p_blood1
	p_blood2
	p_blood3
	num_particletypes
)

type part_move_t int

const (
	pm_static part_move_t = iota
	pm_normal
	pm_bounce
	pm_die
	pm_nophysics
	pm_float
)

type part_tex_t int

const (
	ptex_none part_tex_t = iota
	ptex_smoke
	ptex_bubble
	ptex_generic
	ptex_dpsmoke
	ptex_lava
	ptex_blueflare
	ptex_blood1
	ptex_blood2
	ptex_blood3
	num_particletextures
)

type part_draw_t int

const (
	pd_spark part_draw_t = iota
	pd_sparkray
	pd_billboard
	pd_billboard_vel
)

type qmb_particle_t struct {
	next        *qmb_particle_t
	org, endorg vec3_t
	color       col_t
	growth      float32
	vel         vec3_t
	rotangle    float32
	rotspeed    float32
	size        float32
	start       float32
	die         float32
	hit         byte
	texindex    byte
	bounces     byte
}

type particle_type_t struct {
	start      *qmb_particle_t
	id         part_type_t
	drawtype   part_draw_t
	SrcBlend   int
	DstBlend   int
	texture    part_tex_t
	startalpha float32
	grav       float32
	accel      float32
	move       part_move_t
	custom     float32
}

const MAX_PTEX_COMPONENTS = 8

type particle_texture_t struct {
	texnum     int
	components int
	coords     [MAX_PTEX_COMPONENTS][4]float32
}

var sint = [7]float32{0.000000, 0.781832, 0.974928, 0.433884, -0.433884, -0.974928, -0.781832}
var cost = [7]float32{1.000000, 0.623490, -0.222521, -0.900969, -0.900969, -0.222521, 0.623490}

var qmb_particles []qmb_particle_t
var qmb_free_particles *qmb_particle_t
var particle_types [num_particletypes]particle_type_t
var particle_type_index [num_particletypes]int
var particle_textures [num_particletextures]particle_texture_t

var qmb_numparticles int
var zerodir = vec3_t{22, 22, 22}
var particle_time float32
var trail_stop vec3_t

var qmb_initialized = false

var gl_clipparticles = cvar_t{name: "gl_clipparticles", string: "1"}
var gl_bounceparticles = cvar_t{name: "gl_bounceparticles", string: "1"}

func TruePointContents(p vec3_t) int {
	return PM_HullPointContents(&cl.worldmodel.hulls[0], 0, p)
}

func ISUNDERWATER(x int) bool {
	return x == CONTENTS_WATER || x == CONTENTS_SLIME || x == CONTENTS_LAVA
}

func TraceLineN(start, end vec3_t, impact, normal *vec3_t) bool {
	var trace pmtrace_t

	trace.fraction = 1
	if PM_RecursiveHullCheck(&cl.worldmodel.hulls[0], 0, 0, 1, start, end, &trace) {
		return false
	}
	*impact = trace.endpos
	if normal != nil {
		*normal = trace.plane.normal
	}

	return true
}

var color_for_particle col_t

func ColorForParticle(type_ part_type_t) *col_t {
	color := &color_for_particle

	switch type_ {
	case p_spark:
		color[0] = byte(224 + (rand() & 31))
		color[1] = byte(100 + (rand() & 31))
		color[2] = 0
	case p_smoke:
		color[0], color[1], color[2] = 255, 255, 255
	case p_fire:
		color[0], color[1], color[2] = 255, 142, 62
	case p_bubble, p_staticbubble:
		c := byte(192 + (rand() & 63))
		color[0], color[1], color[2] = c, c, c
	case p_teleflare, p_lavasplash:
		c := byte(128 + (rand() & 127))
		color[0], color[1], color[2] = c, c, c
	case p_gunblast:
		color[0] = byte(224 + (rand() & 31))
		color[1] = byte(170 + (rand() & 31))
		color[2] = 0
	case p_chunk:
		c := byte(32 + (rand() & 127))
		color[0], color[1], color[2] = c, c, c
	case p_shockwave:
		c := byte(64 + (rand() & 31))
		color[0], color[1], color[2] = c, c, c
	case p_inferno_flame, p_inferno_trail:
		color[0], color[1], color[2] = 255, 77, 13
	case p_sparkray:
		color[0], color[1], color[2] = 255, 102, 25
	case p_dpsmoke:
		c := byte(48 + (((rand() & 0xFF) * 48) >> 8))
		color[0], color[1], color[2] = c, c, c
	case p_dpfire:
		lambda := rand() & 0xFF
		color[0] = byte(160 + ((lambda * 48) >> 8))
		color[1] = byte(16 + ((lambda * 148) >> 8))
		color[2] = byte(16 + ((lambda * 16) >> 8))
	case p_blood1, p_blood2:
		c := byte(180 + (rand() & 63))
		color[0], color[1], color[2] = c, c, c
	case p_blood3:
		color[0] = byte(100 + (rand() & 31))
		color[1], color[2] = 0, 0
	default:
		Sys_Error("ColorForParticle: unexpected type")
	}
	return color
}

func ADD_PARTICLE_TEXTURE(ptex part_tex_t, texnum, texindex, components, s1, t1, s2, t2 int) {
	particle_textures[ptex].texnum = texnum
	particle_textures[ptex].components = components
	particle_textures[ptex].coords[texindex][0] = float32(float64(s1+1) / 256.0)
	particle_textures[ptex].coords[texindex][1] = float32(float64(t1+1) / 256.0)
	particle_textures[ptex].coords[texindex][2] = float32(float64(s2-1) / 256.0)
	particle_textures[ptex].coords[texindex][3] = float32(float64(t2-1) / 256.0)
}

func R_Particles_CvarInit() {
	Cvar_SetCurrentGroup(CVAR_GROUP_PARTICLES)
	Cvar_Register(&gl_clipparticles)
	Cvar_Register(&gl_bounceparticles)

	Cvar_ResetCurrentGroup()
}

// C writes -0 into the sparkray gravity
var negzero = float32(math.Copysign(0, -1))

func R_Particles_TextureInit() {
	count := 0

	particlefont := R_LoadTextureImage("textures/particles/particlefont", "qmb:particlefont", 256, 256, TEX_ALPHA|TEX_COMPLAIN)
	if particlefont == 0 {
		qmb_initialized = false
		return
	}

	qmb_initialized = true

	ADD_PARTICLE_TEXTURE(ptex_none, 0, 0, 1, 0, 0, 0, 0)
	ADD_PARTICLE_TEXTURE(ptex_blood1, particlefont, 0, 1, 0, 0, 64, 64)
	ADD_PARTICLE_TEXTURE(ptex_blood2, particlefont, 0, 1, 64, 0, 128, 64)
	ADD_PARTICLE_TEXTURE(ptex_lava, particlefont, 0, 1, 128, 0, 192, 64)
	ADD_PARTICLE_TEXTURE(ptex_blueflare, particlefont, 0, 1, 192, 0, 256, 64)
	ADD_PARTICLE_TEXTURE(ptex_generic, particlefont, 0, 1, 0, 96, 96, 192)
	ADD_PARTICLE_TEXTURE(ptex_smoke, particlefont, 0, 1, 96, 96, 192, 192)
	ADD_PARTICLE_TEXTURE(ptex_blood3, particlefont, 0, 1, 192, 96, 256, 160)
	ADD_PARTICLE_TEXTURE(ptex_bubble, particlefont, 0, 1, 192, 160, 224, 192)
	for i := 0; i < 8; i++ {
		ADD_PARTICLE_TEXTURE(ptex_dpsmoke, particlefont, i, 8, i*32, 64, (i+1)*32, 96)
	}

	add := func(id part_type_t, drawtype part_draw_t, SrcBlend, DstBlend int, texture part_tex_t, startalpha, grav, accel float32, move part_move_t, custom float32) {
		pt := &particle_types[count]
		pt.id = id
		pt.drawtype = drawtype
		pt.SrcBlend = SrcBlend
		pt.DstBlend = DstBlend
		pt.texture = texture
		pt.startalpha = startalpha
		pt.grav = float32(9.8 * float64(grav))
		pt.accel = accel
		pt.move = move
		pt.custom = custom
		particle_type_index[id] = count
		count++
	}

	add(p_spark, pd_spark, GL_SRC_ALPHA, GL_ONE, ptex_none, 255, -32, 0, pm_bounce, 1.3)
	add(p_sparkray, pd_sparkray, GL_SRC_ALPHA, GL_ONE, ptex_none, 255, negzero, 0, pm_nophysics, 0)
	add(p_gunblast, pd_spark, GL_SRC_ALPHA, GL_ONE, ptex_none, 255, -16, 0, pm_bounce, 1.3)

	add(p_fire, pd_billboard, GL_SRC_ALPHA, GL_ONE, ptex_generic, 204, 0, -2.95, pm_die, 0)
	add(p_chunk, pd_billboard, GL_SRC_ALPHA, GL_ONE, ptex_generic, 255, -16, 0, pm_bounce, 1.475)
	add(p_shockwave, pd_billboard, GL_SRC_ALPHA, GL_ONE, ptex_generic, 255, 0, -4.85, pm_nophysics, 0)
	add(p_inferno_flame, pd_billboard, GL_SRC_ALPHA, GL_ONE, ptex_generic, 153, 0, 0, pm_static, 0)
	add(p_inferno_trail, pd_billboard, GL_SRC_ALPHA, GL_ONE, ptex_generic, 204, 0, 0, pm_die, 0)
	add(p_trailpart, pd_billboard, GL_SRC_ALPHA, GL_ONE, ptex_generic, 230, 0, 0, pm_static, 0)
	add(p_smoke, pd_billboard, GL_SRC_ALPHA, GL_ONE, ptex_smoke, 140, 3, 0, pm_normal, 0)
	add(p_dpfire, pd_billboard, GL_SRC_ALPHA, GL_ONE, ptex_dpsmoke, 144, 0, 0, pm_die, 0)
	add(p_dpsmoke, pd_billboard, GL_SRC_ALPHA, GL_ONE, ptex_dpsmoke, 85, 3, 0, pm_die, 0)

	add(p_teleflare, pd_billboard, GL_ONE, GL_ONE, ptex_blueflare, 255, 0, 0, pm_die, 0)

	add(p_blood1, pd_billboard, GL_ZERO, GL_ONE_MINUS_SRC_COLOR, ptex_blood1, 255, -20, 0, pm_die, 0)
	add(p_blood2, pd_billboard_vel, GL_ZERO, GL_ONE_MINUS_SRC_COLOR, ptex_blood2, 255, -25, 0, pm_die, 0.018)

	add(p_lavasplash, pd_billboard, GL_SRC_ALPHA, GL_ONE_MINUS_SRC_ALPHA, ptex_lava, 170, 0, 0, pm_nophysics, 0)
	add(p_blood3, pd_billboard, GL_SRC_ALPHA, GL_ONE_MINUS_SRC_ALPHA, ptex_blood3, 255, -20, 0, pm_normal, 0)
	add(p_bubble, pd_billboard, GL_SRC_ALPHA, GL_ONE_MINUS_SRC_ALPHA, ptex_bubble, 204, 8, 0, pm_float, 0)
	add(p_staticbubble, pd_billboard, GL_SRC_ALPHA, GL_ONE_MINUS_SRC_ALPHA, ptex_bubble, 204, 0, 0, pm_static, 0)
}

func QMB_InitParticles() int {
	if i := COM_CheckParm("-particles"); i != 0 && i+1 < com_argc {
		qmb_numparticles = Q_atoi(com_argv[i+1])
		qmb_numparticles = bound(qmb_absolute_min_particles, qmb_numparticles, qmb_absolute_max_particles)
	} else {
		qmb_numparticles = qmb_default_num_particles
	}

	qmb_particles = make([]qmb_particle_t, qmb_numparticles)
	return 1
}

func QMB_ShutdownParticles() {
	qmb_particles = nil
}

func QMB_ClearParticles() {
	if !qmb_initialized {
		return
	}

	clear(qmb_particles)
	qmb_free_particles = &qmb_particles[0]

	for i := 0; i+1 < qmb_numparticles; i++ {
		qmb_particles[i].next = &qmb_particles[i+1]
	}
	qmb_particles[qmb_numparticles-1].next = nil

	for i := range particle_types {
		particle_types[i].start = nil
	}
}

func QMB_UpdateParticles() {
	var oldorg, stop, normal vec3_t

	grav := float32(float64(movevars.gravity) / 800.0)

	for i := 0; i < int(num_particletypes); i++ {
		pt := &particle_types[i]

		// C aliases pt->start as the previous particle's next; a link pointer does the same
		link := &pt.start
		nextp := pt.start
		for p := nextp; p != nil; p = nextp {
			nextp = p.next

			if p.die <= particle_time {
				*link = p.next
				p.next = qmb_free_particles
				qmb_free_particles = p

				continue
			} else {
				link = &p.next
			}

			if particle_time < p.start {
				continue
			}

			p.size = float32(float64(p.size) + float64(p.growth)*cls.frametime)

			if p.size <= 0 {
				p.die = 0
				continue
			}

			p.color[3] = byte(pt.startalpha * ((p.die - particle_time) / (p.die - p.start)))

			p.rotangle = float32(float64(p.rotangle) + float64(p.rotspeed)*cls.frametime)

			if p.hit != 0 {
				continue
			}

			p.vel[2] = float32(float64(p.vel[2]) + float64(pt.grav*grav)*cls.frametime)

			VectorScale(p.vel, float32(1+float64(pt.accel)*cls.frametime), &p.vel)

			switch pt.move {
			case pm_static:
			case pm_normal:
				oldorg = p.org
				VectorMA(p.org, float32(cls.frametime), p.vel, &p.org)
				if CONTENTS_SOLID == TruePointContents(p.org) {
					p.hit = 1
					p.org = oldorg
					p.vel = vec3_t{}
				}
			case pm_float:
				VectorMA(p.org, float32(cls.frametime), p.vel, &p.org)
				p.org[2] += p.size + 1
				contents := TruePointContents(p.org)
				if !ISUNDERWATER(contents) {
					p.die = 0
				}
				p.org[2] -= p.size + 1
			case pm_nophysics:
				VectorMA(p.org, float32(cls.frametime), p.vel, &p.org)
			case pm_die:
				VectorMA(p.org, float32(cls.frametime), p.vel, &p.org)
				if CONTENTS_SOLID == TruePointContents(p.org) {
					p.die = 0
				}
			case pm_bounce:
				if gl_bounceparticles.value == 0 || p.bounces != 0 {
					VectorMA(p.org, float32(cls.frametime), p.vel, &p.org)
					if CONTENTS_SOLID == TruePointContents(p.org) {
						p.die = 0
					}
				} else {
					oldorg = p.org
					VectorMA(p.org, float32(cls.frametime), p.vel, &p.org)
					if CONTENTS_SOLID == TruePointContents(p.org) {
						if TraceLineN(oldorg, p.org, &stop, &normal) {
							p.org = stop
							bounce := -pt.custom * DotProduct(p.vel, normal)
							VectorMA(p.vel, bounce, normal, &p.vel)
							p.bounces++
						}
					}
				}
			default:
				Sys_Error("QMB_UpdateParticles: unexpected pt->move")
			}
		}
	}
}

func QMB_PipeForBlend(SrcBlend, DstBlend int) int {
	if SrcBlend == GL_ONE && DstBlend == GL_ONE {
		return SCENE_PIPE_ADD_NODEPTHWRITE
	}
	if SrcBlend == GL_ZERO && DstBlend == GL_ONE_MINUS_SRC_COLOR {
		return SCENE_PIPE_INVSRCCOLOR_NODEPTHWRITE
	}
	if SrcBlend == GL_SRC_ALPHA && DstBlend == GL_ONE_MINUS_SRC_ALPHA {
		return SCENE_PIPE_TEX_BLEND_NODEPTHWRITE
	}
	return SCENE_PIPE_ADDALPHA_NODEPTHWRITE
}

func QWB_DrawBillboardParticle(ptex *particle_texture_t, p *qmb_particle_t, coord []vec3_t) {
	var newcoord [4]vec3_t

	for j := 0; j < 4; j++ {
		VectorMA(p.org, p.size, coord[j], &newcoord[j])
	}

	Run_EmitQuad(newcoord[:], ptex.coords[p.texindex][:], p.color)
}

func QWB_DrawBillboardParticleRotate(ptex *particle_texture_t, p *qmb_particle_t, coord []vec3_t, vpn vec3_t) {
	var rotated vec3_t
	var newcoord [4]vec3_t

	for j := 0; j < 4; j++ {
		RotatePointAroundVector(&rotated, vpn, coord[j], p.rotangle)
		VectorMA(p.org, p.size, rotated, &newcoord[j])
	}

	Run_EmitQuad(newcoord[:], ptex.coords[p.texindex][:], p.color)
}

func QMB_DrawParticles() {
	var v, neworg vec3_t
	var billboard [4]vec3_t
	var edges [8]vec3_t
	var edgecolor col_t

	if !qmb_initialized {
		return
	}

	particle_time = float32(cl.time)

	if cl.paused == 0 {
		QMB_UpdateParticles()
	}

	VectorAdd(vup, vright, &billboard[2])
	VectorSubtract(vright, vup, &billboard[3])
	VectorNegate(billboard[2], &billboard[0])
	VectorNegate(billboard[3], &billboard[1])

	for i := 0; i < int(num_particletypes); i++ {
		pt := &particle_types[i]
		if pt.start == nil {
			continue
		}

		pipe := QMB_PipeForBlend(pt.SrcBlend, pt.DstBlend)

		switch pt.drawtype {
		case pd_spark:
			Run_Select(pipe, GPU_Texture_White())
			for p := pt.start; p != nil; p = p.next {
				if particle_time < p.start || particle_time >= p.die {
					continue
				}

				edgecolor[0] = p.color[0] >> 1
				edgecolor[1] = p.color[1] >> 1
				edgecolor[2] = p.color[2] >> 1
				edgecolor[3] = 0
				for j := 7; j >= 0; j-- {
					for k := 0; k < 3; k++ {
						v[k] = p.org[k] - p.vel[k]/8 + vright[k]*cost[j%7]*p.size + vup[k]*sint[j%7]*p.size
					}
					edges[7-j] = v
				}
				Run_EmitFan(p.org, p.color, edgecolor, edges[:], 8)
			}
		case pd_sparkray:
			Run_Select(pipe, GPU_Texture_White())
			for p := pt.start; p != nil; p = p.next {
				if particle_time < p.start || particle_time >= p.die {
					continue
				}

				if !TraceLineN(p.endorg, p.org, &neworg, nil) {
					neworg = p.org
				}

				edgecolor[0] = p.color[0] >> 1
				edgecolor[1] = p.color[1] >> 1
				edgecolor[2] = p.color[2] >> 1
				edgecolor[3] = 0
				for j := 7; j >= 0; j-- {
					for k := 0; k < 3; k++ {
						v[k] = neworg[k] + vright[k]*cost[j%7]*p.size + vup[k]*sint[j%7]*p.size
					}
					edges[7-j] = v
				}
				Run_EmitFan(p.endorg, p.color, edgecolor, edges[:], 8)
			}
		case pd_billboard:
			ptex := &particle_textures[pt.texture]
			Run_Select(pipe, ptex.texnum)
			drawncount := 0
			for p := pt.start; p != nil; p = p.next {
				if particle_time < p.start || particle_time >= p.die {
					continue
				}

				if gl_clipparticles.value != 0 {
					if drawncount >= 3 && VectorSupCompare(p.org, r_origin, 30) {
						continue
					}
					drawncount++
				}
				QWB_DrawBillboardParticle(ptex, p, billboard[:])
			}
		case pd_billboard_vel:
			ptex := &particle_textures[pt.texture]
			Run_Select(pipe, ptex.texnum)
			for p := pt.start; p != nil; p = p.next {
				if particle_time < p.start || particle_time >= p.die {
					continue
				}

				if p.rotspeed != 0 {
					QWB_DrawBillboardParticleRotate(ptex, p, billboard[:], vpn)
				} else {
					QWB_DrawBillboardParticle(ptex, p, billboard[:])
				}
			}
		default:
			Sys_Error("QMB_DrawParticles: unexpected drawtype")
		}
	}

	Run_Flush()
}

// INIT_NEW_PARTICLE; the colour copy takes three components like VectorCopy
func INIT_NEW_PARTICLE(pt *particle_type_t, color *col_t, size, time float32) *qmb_particle_t {
	p := qmb_free_particles
	qmb_free_particles = p.next
	p.next = pt.start
	pt.start = p
	p.size = size
	p.hit = 0
	p.start = float32(cl.time)
	p.die = p.start + time
	p.growth = 0
	p.rotspeed = 0
	p.texindex = byte(rand() % particle_textures[pt.texture].components)
	p.bounces = 0
	p.color[0], p.color[1], p.color[2] = color[0], color[1], color[2]
	return p
}

func AddParticle(type_ part_type_t, org vec3_t, count int, size, time float32, col *col_t, dir vec3_t) {
	if !qmb_initialized {
		Sys_Error("QMB particle added without initialization")
	}

	if size <= 0 {
		Sys_Error("AddParticle: size <= 0")
	}

	if time <= 0 {
		Sys_Error("AddParticle: time <= 0")
	}

	if type_ < 0 || type_ >= num_particletypes {
		Sys_Error("AddParticle: Invalid type (%d)", type_)
	}

	pt := &particle_types[particle_type_index[type_]]

	for i := 0; i < count && qmb_free_particles != nil; i++ {
		color := col
		if color == nil {
			color = ColorForParticle(type_)
		}
		p := INIT_NEW_PARTICLE(pt, color, size, time)

		switch type_ {
		case p_spark:
			p.size = 1.175
			p.org = org
			tempSize := size * 2
			p.vel[0] = float32((rand() % int(tempSize)) - (int(tempSize) / 2))
			p.vel[1] = float32((rand() % int(tempSize)) - (int(tempSize) / 2))
			p.vel[2] = float32((rand() % int(tempSize)) - (int(tempSize) / 3))
		case p_smoke:
			for j := 0; j < 3; j++ {
				p.org[j] = float32(float64(org[j]) + float64((rand()&31)-16)/2.0)
			}
			for j := 0; j < 3; j++ {
				p.vel[j] = float32(float64((rand()%10)-5) / 20.0)
			}
		case p_fire:
			p.org = org
			for j := 0; j < 3; j++ {
				p.vel[j] = float32(float64((rand()%160)-80) * (float64(size) / 25.0))
			}
		case p_bubble:
			p.start = float32(float64(p.start) + float64(rand()&15)/36.0)
			p.org[0] = org[0] + float32((rand()&31)-16)
			p.org[1] = org[1] + float32((rand()&31)-16)
			p.org[2] = org[2] + float32((rand()&63)-32)
			p.vel = vec3_t{}
		case p_lavasplash:
			p.org = org
			p.vel = dir
		case p_gunblast:
			p.size = 1
			p.org = org
			p.vel[0] = float32((rand() & 127) - 64)
			p.vel[1] = float32((rand() & 127) - 64)
			p.vel[2] = float32((rand() & 127) - 10)
		case p_chunk:
			p.org = org
			p.vel[0] = float32((rand() % 40) - 20)
			p.vel[1] = float32((rand() % 40) - 20)
			p.vel[2] = float32((rand() % 40) - 5)
		case p_shockwave:
			p.org = org
			p.vel = dir
		case p_inferno_trail:
			for j := 0; j < 3; j++ {
				p.org[j] = org[j] + float32(rand()&15) - 8
			}
			for j := 0; j < 3; j++ {
				p.vel[j] = float32((rand() & 3) - 2)
			}
			p.growth = -1.5
		case p_inferno_flame:
			p.org = org
			p.vel = vec3_t{}
			p.growth = -30
		case p_sparkray:
			p.endorg = org
			p.org = dir
			for j := 0; j < 3; j++ {
				p.vel[j] = float32((rand() & 127) - 64)
			}
			p.growth = -16
		case p_staticbubble:
			p.org = org
			p.vel = vec3_t{}
		case p_teleflare:
			p.org = org
			p.vel = dir
			p.growth = 1.75
		case p_blood1, p_blood2:
			for j := 0; j < 3; j++ {
				p.org[j] = org[j] + float32(rand()&15) - 8
			}
			for j := 0; j < 3; j++ {
				p.vel[j] = float32((rand() & 63) - 32)
			}
		case p_blood3:
			p.size = float32(float64(size*float32(rand()%20)) / 5.0)
			p.org = org
			for j := 0; j < 3; j++ {
				p.vel[j] = float32((rand() % 40) - 20)
			}
		default:
			Sys_Error("AddParticle: unexpected type")
		}
	}
}

func AddParticleTrail(type_ part_type_t, start, end vec3_t, size, time float32, col *col_t) {
	var delta vec3_t

	if !qmb_initialized {
		Sys_Error("QMB particle added without initialization")
	}

	if size <= 0 {
		Sys_Error("AddParticleTrail: size <= 0")
	}

	if time <= 0 {
		Sys_Error("AddParticleTrail: time <= 0")
	}

	if type_ < 0 || type_ >= num_particletypes {
		Sys_Error("AddParticle: Invalid type (%d)", type_)
	}

	pt := &particle_types[particle_type_index[type_]]

	point := start
	VectorSubtract(end, start, &delta)
	length := VectorLength(delta)
	if length != 0 {
		var count float32
		switch type_ {
		case p_trailpart:
			count = float32(float64(length) / 1.1)
		case p_blood3:
			count = length / 8
		case p_smoke:
			count = float32(float64(length) / 3.8)
		case p_dpsmoke:
			count = float32(float64(length) / 2.5)
		case p_dpfire:
			count = float32(float64(length) / 2.8)
		default:
			Sys_Error("AddParticleTrail: unexpected type")
		}

		num_particles := int(count)
		if num_particles != 0 {
			VectorScale(delta, float32(1.0/float64(num_particles)), &delta)

			for i := 0; i < num_particles && qmb_free_particles != nil; i++ {
				color := col
				if color == nil {
					color = ColorForParticle(type_)
				}
				p := INIT_NEW_PARTICLE(pt, color, size, time)

				switch type_ {
				case p_trailpart:
					p.org = point
					p.vel = vec3_t{}
					p.growth = -size / time
				case p_blood3:
					p.org = point
					for j := 0; j < 3; j++ {
						p.org[j] = float32(float64(p.org[j]) + float64((rand()&15)-8)/8.0)
					}
					for j := 0; j < 3; j++ {
						p.vel[j] = float32(float64((rand()&15)-8) / 2.0)
					}
					p.size = float32(float64(size*float32(rand()%20)) / 10.0)
					p.growth = 6
				case p_smoke:
					p.org = point
					for j := 0; j < 3; j++ {
						p.org[j] = float32(float64(p.org[j]) + float64((rand()&7)-4)/8.0)
					}
					p.vel[0], p.vel[1] = 0, 0
					p.vel[2] = float32(rand() & 3)
					p.growth = 4.5
					p.rotspeed = float32((rand() & 63) + 96)
				case p_dpsmoke:
					p.org = point
					for j := 0; j < 3; j++ {
						p.vel[j] = float32((rand() % 10) - 5)
					}
					p.growth = 3
					p.rotspeed = float32((rand() & 63) + 96)
				case p_dpfire:
					p.org = point
					for j := 0; j < 3; j++ {
						p.vel[j] = float32((rand() % 40) - 20)
					}
				default:
					Sys_Error("AddParticleTrail: unexpected type")
				}

				VectorAdd(point, delta, &point)
			}
		}
	}
	trail_stop = point
}

const ONE_FRAME_ONLY = 0.0001

func QMB_ParticleExplosion(org vec3_t) {
	contents := TruePointContents(org)
	if ISUNDERWATER(contents) {
		AddParticle(p_fire, org, 12, 14, 0.8, nil, zerodir)
		AddParticle(p_bubble, org, 6, 3.0, 2.5, nil, zerodir)
		AddParticle(p_bubble, org, 4, 2.35, 2.5, nil, zerodir)
		if r_explosiontype.value != 1 {
			AddParticle(p_spark, org, 50, 100, 0.75, nil, zerodir)
			AddParticle(p_spark, org, 25, 60, 0.75, nil, zerodir)
		}
	} else {
		AddParticle(p_fire, org, 16, 18, 1, nil, zerodir)
		if r_explosiontype.value != 1 {
			AddParticle(p_spark, org, 50, 250, 0.925, nil, zerodir)
			AddParticle(p_spark, org, 25, 150, 0.925, nil, zerodir)
		}
	}
}

func QMB_RunParticleEffect(org, dir vec3_t, col, count int) {
	var color col_t
	var neworg vec3_t

	if col == 73 {
		bloodcount := Q_rint(float64(count) / 28.0)
		bloodcount = bound(2, bloodcount, 8)
		for i := 0; i < bloodcount; i++ {
			AddParticle(p_blood1, org, 1, 4.5, float32(2+float64(rand()&15)/16.0), nil, zerodir)
			AddParticle(p_blood2, org, 1, 4.5, float32(2+float64(rand()&15)/16.0), nil, zerodir)
		}
		return
	} else if col == 225 {
		color[0], color[1], color[2] = 0, 0, 0
		scale := 1
		if count > 130 {
			scale = 3
		} else if count > 20 {
			scale = 2
		}
		scale = int(float64(scale) * 0.758)
		for i := 0; i < (count >> 1); i++ {
			neworg[0] = org[0] + float32(scale*((rand()&15)-8))
			neworg[1] = org[1] + float32(scale*((rand()&15)-8))
			neworg[2] = org[2] + float32(scale*((rand()&15)-8))
			color[0] = byte(50 + (rand() & 63))
			AddParticle(p_blood3, org, 1, 1, 2.3, nil, zerodir)
		}
		return
	} else if col == 20 && count == 30 {
		color[0], color[2], color[1] = 51, 51, 255
		AddParticle(p_chunk, org, 1, 1, 0.75, &color, zerodir)
		AddParticle(p_spark, org, 12, 75, 0.4, &color, zerodir)
		return
	} else if col == 226 && count == 20 {
		color[0], color[1], color[2] = 230, 204, 26
		AddParticle(p_chunk, org, 1, 1, 0.75, &color, zerodir)
		AddParticle(p_spark, org, 12, 75, 0.4, &color, zerodir)
		return
	}

	switch count {
	case 10:
		AddParticle(p_spark, org, 4, 70, 0.9, nil, zerodir)
	case 20:
		AddParticle(p_spark, org, 8, 85, 0.9, nil, zerodir)
	case 30:
		AddParticle(p_chunk, org, 10, 1, 4, nil, zerodir)
		AddParticle(p_spark, org, 8, 105, 0.9, nil, zerodir)
	default:
		// C stores 2.274, 1.516 and 0.758 into an int scale; the spark values it also sets are never read
		var scale, blastcount, blastsize, chunkcount int
		var blasttime float32
		if count > 130 {
			scale = 2
			blastcount, blastsize, blasttime = 50, 50, 0.4
			chunkcount = 14
		} else if count > 20 {
			scale = 1
			blastcount, blastsize, blasttime = 30, 35, 0.25
			chunkcount = 7
		} else {
			scale = 0
			blastcount, blastsize, blasttime = 15, 5, 0.15
			chunkcount = 3
		}
		particlecount := max(1, count>>1)
		AddParticle(p_gunblast, org, blastcount, float32(blastsize), blasttime, nil, zerodir)
		for i := 0; i < particlecount; i++ {
			neworg[0] = org[0] + float32(scale*((rand()&15)-8))
			neworg[1] = org[1] + float32(scale*((rand()&15)-8))
			neworg[2] = org[2] + float32(scale*((rand()&15)-8))
			AddParticle(p_smoke, neworg, 1, 4, float32(float64(float32(0.825))+float64((rand()%10)-5)/40.0), nil, zerodir)
			if i%(particlecount/chunkcount) == 0 {
				AddParticle(p_chunk, neworg, 1, 0.75, 3.75, nil, zerodir)
			}
		}
	}
}

func QMB_ParticleTrail(start, end vec3_t, trail_origin *vec3_t, type_ trail_type_t) {
	var color col_t

	switch type_ {
	case GRENADE_TRAIL:
		AddParticleTrail(p_smoke, start, end, 1.45, 0.825, nil)
	case BLOOD_TRAIL, BIG_BLOOD_TRAIL:
		size := float32(2.4)
		if type_ == BLOOD_TRAIL {
			size = 1.35
		}
		AddParticleTrail(p_blood3, start, end, size, 2, nil)
	case TRACER1_TRAIL:
		color[0], color[1], color[2] = 0, 124, 0
		AddParticleTrail(p_trailpart, start, end, 3.75, 0.5, &color)
	case TRACER2_TRAIL:
		color[0], color[1], color[2] = 255, 77, 0
		AddParticleTrail(p_trailpart, start, end, 3.75, 0.5, &color)
	case VOOR_TRAIL:
		color[0], color[1], color[2] = 77, 0, 255
		AddParticleTrail(p_trailpart, start, end, 3.75, 0.5, &color)
	case ALT_ROCKET_TRAIL:
		AddParticleTrail(p_dpfire, start, end, 3, 0.26, nil)
		AddParticleTrail(p_dpsmoke, start, end, 3, 0.825, nil)
	default: // ROCKET_TRAIL
		color[0], color[1], color[2] = 255, 56, 9
		AddParticleTrail(p_trailpart, start, end, 6.2, 0.31, &color)
		AddParticleTrail(p_smoke, start, end, 1.8, 0.825, nil)
	}

	*trail_origin = trail_stop
}

func QMB_BlobExplosion(org vec3_t) {
	var color col_t
	var neworg, vel vec3_t

	color[0], color[1], color[2] = 60, 100, 240
	AddParticle(p_spark, org, 44, 250, 1.15, &color, zerodir)

	color[0], color[1], color[2] = 90, 47, 207
	AddParticle(p_fire, org, 15, 30, 1.4, &color, zerodir)

	vel[2] = 0
	for theta := float32(0); float64(theta) < 2*float64(math.Pi); theta = float32(float64(theta) + 2*float64(math.Pi)/70) {
		color[0] = byte(60 + (rand() & 15))
		color[1] = byte(65 + (rand() & 15))
		color[2] = byte(200 + (rand() & 15))

		vel[0] = float32(math.Cos(float64(theta)) * 125)
		vel[1] = float32(math.Sin(float64(theta)) * 125)
		neworg[0] = float32(float64(org[0]) + math.Cos(float64(theta))*6)
		neworg[1] = float32(float64(org[1]) + math.Sin(float64(theta))*6)
		neworg[2] = org[2] + 0 - 10
		AddParticle(p_shockwave, neworg, 1, 4, 0.8, &color, vel)
		neworg[2] = org[2] + 0 + 10
		AddParticle(p_shockwave, neworg, 1, 4, 0.8, &color, vel)

		vel[0] = float32(float64(vel[0]) * 1.15)
		vel[1] = float32(float64(vel[1]) * 1.15)
		neworg[0] = float32(float64(org[0]) + math.Cos(float64(theta))*13)
		neworg[1] = float32(float64(org[1]) + math.Sin(float64(theta))*13)
		neworg[2] = org[2] + 0
		AddParticle(p_shockwave, neworg, 1, 6, 1.0, &color, vel)
	}
}

func QMB_LavaSplash(org vec3_t) {
	var dir, neworg vec3_t

	for i := -16; i < 16; i++ {
		for j := -16; j < 16; j++ {
			dir[0] = float32(j*8 + (rand() & 7))
			dir[1] = float32(i*8 + (rand() & 7))
			dir[2] = 256

			neworg[0] = org[0] + dir[0]
			neworg[1] = org[1] + dir[1]
			neworg[2] = org[2] + float32(rand()&63)

			VectorNormalizeFast(&dir)
			vel := float32(50 + (rand() & 63))
			VectorScale(dir, vel, &dir)

			AddParticle(p_lavasplash, neworg, 1, 4.5, float32(2.6+float64(rand()&31)*0.02), nil, dir)
		}
	}
}

func QMB_TeleportSplash(org vec3_t) {
	var neworg, angle vec3_t
	var color col_t

	for i := -12; i <= 12; i += 6 {
		for j := -12; j <= 12; j += 6 {
			for k := -24; k <= 32; k += 8 {
				neworg[0] = org[0] + float32(i) + float32(rand()&3) - 1
				neworg[1] = org[1] + float32(j) + float32(rand()&3) - 1
				neworg[2] = org[2] + float32(k) + float32(rand()&3) - 1
				angle[0] = float32((rand() & 15) - 7)
				angle[1] = float32((rand() & 15) - 7)
				angle[2] = float32((rand() % 160) - 80)
				AddParticle(p_teleflare, neworg, 1, 1.8, float32(0.30+float64(rand()&7)*0.02), nil, angle)
			}
		}
	}

	color[0], color[1], color[2] = 140, 140, 255
	angle = vec3_t{}
	for i := 0; i < 5; i++ {
		angle[2] = 0
		for j := 0; j < 5; j++ {
			AngleVectors(angle, nil, nil, &neworg)
			VectorMA(org, 70, neworg, &neworg)
			AddParticle(p_sparkray, org, 1, float32(6+(i&3)), 5, &color, neworg)
			angle[2] += 360 / 5
		}
		angle[0] += 180 / 5
	}
}

func QMB_DetpackExplosion(org vec3_t) {
	var neworg vec3_t
	angle := vec3_t{0, 0, 0}

	contents := TruePointContents(org)
	if ISUNDERWATER(contents) {
		AddParticle(p_bubble, org, 8, 2.8, 2.5, nil, zerodir)
		AddParticle(p_bubble, org, 6, 2.2, 2.5, nil, zerodir)
		AddParticle(p_fire, org, 10, 25, 0.75, ColorForParticle(p_inferno_flame), zerodir)
	} else {
		AddParticle(p_fire, org, 14, 33, 0.75, ColorForParticle(p_inferno_flame), zerodir)
	}

	for i := 0; i < 5; i++ {
		angle[2] = 0
		for j := 0; j < 5; j++ {
			AngleVectors(angle, nil, nil, &neworg)
			VectorMA(org, 90, neworg, &neworg)
			AddParticle(p_sparkray, org, 1, float32(9+(i&3)), 0.75, nil, neworg)
			angle[2] += 360 / 5
		}
		angle[0] += 180 / 5
	}

	angle[2] = 0
	for theta := float32(0); float64(theta) < 2*float64(math.Pi); theta = float32(float64(theta) + 2*float64(math.Pi)/90) {
		angle[0] = float32(math.Cos(float64(theta)) * 350)
		angle[1] = float32(math.Sin(float64(theta)) * 350)
		AddParticle(p_shockwave, org, 1, 14, 0.75, nil, angle)
	}
}

func QMB_InfernoFlame(org vec3_t) {
	if cls.frametime != 0 {
		AddParticle(p_inferno_flame, org, 1, 30, float32(13.125*cls.frametime), nil, zerodir)
		AddParticle(p_inferno_trail, org, 2, 1.75, float32(45.0*cls.frametime), nil, zerodir)
		AddParticle(p_inferno_trail, org, 2, 1.0, float32(52.5*cls.frametime), nil, zerodir)
	}
}

func QMB_StaticBubble(ent *entity_t) {
	size := float32(2.9)
	if ent.frame == 1 {
		size = 1.85
	}
	AddParticle(p_staticbubble, ent.origin, 1, size, ONE_FRAME_ONLY, nil, zerodir)
}

// ---- flashblend dlight bubbles (gl_rlight.c) ----

var bubble_sintable, bubble_costable [17]float32
var bubble_initialized bool

func R_InitBubble() {
	for i := 16; i >= 0; i-- {
		a := float32(float64(i) / 16.0 * float64(math.Pi) * 2)
		bubble_sintable[16-i] = float32(math.Sin(float64(a)))
		bubble_costable[16-i] = float32(math.Cos(float64(a)))
	}

	bubble_initialized = true
}

var bubblecolor = [NUM_DLIGHTTYPES][4]float32{
	{0.2, 0.1, 0.05},   // dimlight or brightlight (lt_default)
	{0.2, 0.1, 0.05},   // muzzleflash
	{0.2, 0.1, 0.05},   // explosion
	{0.2, 0.1, 0.05},   // rocket
	{0.5, 0.05, 0.05},  // red
	{0.05, 0.05, 0.3},  // blue
	{0.5, 0.05, 0.4},   // red + blue
	{0.05, 0.45, 0.05}, // green
	{0.5, 0.5, 0.5},    // white
}

func R_RenderDlight(light *dlight_t) {
	var v, v_right, v_up, center vec3_t
	var edges [17]vec3_t
	var ccol, ecol [4]byte

	// don't draw our own powerup glow and muzzleflashes
	if light.key == (cl.viewplayernum+1) ||
		light.key == -(cl.viewplayernum+1) { // muzzleflash keys are negative
		return
	}

	rad := float32(float64(light.radius) * 0.35)
	VectorSubtract(light.origin, r_origin, &v)
	length := VectorNormalize(&v)

	if length < rad {
		// view is inside the dlight
		V_AddLightBlend(1, 0.5, 0, float32(float64(light.radius)*0.0003))
		return
	}

	VectorVectors(v, &v_right, &v_up)

	if length-rad > 8 {
		VectorScale(v, rad, &v)
	} else {
		// make sure the light bubble will not be clipped by near z clip plane
		VectorScale(v, length-8, &v)
	}

	VectorSubtract(light.origin, v, &center)

	ccol[0] = byte(bubblecolor[light.type_][0] * 255)
	ccol[1] = byte(bubblecolor[light.type_][1] * 255)
	ccol[2] = byte(bubblecolor[light.type_][2] * 255)
	ccol[3] = 255
	ecol = [4]byte{0, 0, 0, 255}

	for i := 16; i >= 0; i-- {
		for j := 0; j < 3; j++ {
			edges[16-i][j] = light.origin[j] + (v_right[j]*bubble_costable[16-i]+v_up[j]*bubble_sintable[16-i])*rad
		}
	}

	Run_Select(SCENE_PIPE_ADD_NODEPTHWRITE, GPU_Texture_White())
	Run_EmitFan(center, ccol, ecol, edges[:], 17)
}

func R_RenderDlights() {
	if gl_flashblend.value == 0 {
		return
	}

	r_dlightframecount = r_framecount + 1 // because the count hasn't advanced yet for this frame

	if !bubble_initialized {
		R_InitBubble()
	}

	for i := uint(0); i < MAX_DLIGHTS/32; i++ {
		if cl_dlight_active[i] != 0 {
			for j := uint(0); j < 32; j++ {
				if cl_dlight_active[i]&(1<<j) != 0 && i*32+j < MAX_DLIGHTS {
					l := &cl_dlights[i*32+j]

					if l.bubble != 0 && int(gl_flashblend.value) != 2 {
						// C shifts an int by up to 63; x86 masks the count to 5 bits
						R_MarkLights(cl.worldmodel, l, 1<<((i*32+j)&31), 0)
					} else {
						R_RenderDlight(l)
					}
				}
			}
		}
	}

	Run_Flush()
}

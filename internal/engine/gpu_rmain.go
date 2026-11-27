// Copyright (C) 1996-1997 Id Software, Inc.
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

import "math"

// gpu_rmain.c: 3D frame setup for the SDL_GPU renderer; records scene work, no GPU calls here

var r_viewproj [16]float32

var r_dlightframecount int

// ---- matrix helpers, column-major GL layout ----

func Mat4_Identity(m []float32) {
	for i := range m[:16] {
		m[i] = 0
	}
	m[0], m[5], m[10], m[15] = 1, 1, 1, 1
}

func Mat4_Multiply(a, b, out []float32) {
	var tmp [16]float32

	for col := 0; col < 4; col++ {
		for row := 0; row < 4; row++ {
			tmp[col*4+row] = a[0*4+row]*b[col*4+0] +
				a[1*4+row]*b[col*4+1] +
				a[2*4+row]*b[col*4+2] +
				a[3*4+row]*b[col*4+3]
		}
	}

	copy(out[:16], tmp[:])
}

func Mat4_Translate(m []float32, x, y, z float32) {
	m[12] += m[0]*x + m[4]*y + m[8]*z
	m[13] += m[1]*x + m[5]*y + m[9]*z
	m[14] += m[2]*x + m[6]*y + m[10]*z
	m[15] += m[3]*x + m[7]*y + m[11]*z
}

// Rotate* multiply in place like glRotatef, m = m * R

func Mat4_RotateX(m []float32, deg float32) {
	c := float32(math.Cos(float64(deg) * (float64(math.Pi) / 180.0)))
	s := float32(math.Sin(float64(deg) * (float64(math.Pi) / 180.0)))

	for i := 0; i < 4; i++ {
		t := m[4+i]
		m[4+i] = t*c + m[8+i]*s
		m[8+i] = m[8+i]*c - t*s
	}
}

func Mat4_RotateY(m []float32, deg float32) {
	c := float32(math.Cos(float64(deg) * (float64(math.Pi) / 180.0)))
	s := float32(math.Sin(float64(deg) * (float64(math.Pi) / 180.0)))

	for i := 0; i < 4; i++ {
		t := m[i]
		m[i] = t*c - m[8+i]*s
		m[8+i] = t*s + m[8+i]*c
	}
}

func Mat4_RotateZ(m []float32, deg float32) {
	c := float32(math.Cos(float64(deg) * (float64(math.Pi) / 180.0)))
	s := float32(math.Sin(float64(deg) * (float64(math.Pi) / 180.0)))

	for i := 0; i < 4; i++ {
		t := m[i]
		m[i] = t*c + m[4+i]*s
		m[4+i] = m[4+i]*c - t*s
	}
}

// ---- culling ----

// Returns true if the box is completely outside the frustum
func R_CullBox(mins, maxs vec3_t) bool {
	for i := 0; i < 4; i++ {
		if BOX_ON_PLANE_SIDE(mins, maxs, &frustum[i]) == 2 {
			return true
		}
	}
	return false
}

// Returns true if the sphere is completely outside the frustum
func R_CullSphere(centre vec3_t, radius float32) bool {
	for i := 0; i < 4; i++ {
		if PlaneDiff(centre, &frustum[i]) <= -radius {
			return true
		}
	}

	return false
}

func SignbitsForPlane(out *mplane_t) int {
	// for fast box on planeside test
	bits := 0
	for j := 0; j < 3; j++ {
		if out.normal[j] < 0 {
			bits |= 1 << j
		}
	}
	return bits
}

func R_SetFrustum() {
	// rotate VPN right by FOV_X/2 degrees
	RotatePointAroundVector(&frustum[0].normal, vup, vpn, -(90 - r_refdef.fov_x/2))
	// rotate VPN left by FOV_X/2 degrees
	RotatePointAroundVector(&frustum[1].normal, vup, vpn, 90-r_refdef.fov_x/2)
	// rotate VPN up by FOV_Y/2 degrees
	RotatePointAroundVector(&frustum[2].normal, vright, vpn, 90-r_refdef.fov_y/2)
	// rotate VPN down by FOV_Y/2 degrees
	RotatePointAroundVector(&frustum[3].normal, vright, vpn, -(90 - r_refdef.fov_y/2))

	for i := 0; i < 4; i++ {
		frustum[i].type_ = PLANE_ANYZ
		frustum[i].dist = DotProduct(r_origin, frustum[i].normal)
		frustum[i].signbits = byte(SignbitsForPlane(&frustum[i]))
	}
}

// ---- lights ----

func R_AnimateLight() {
	// light animations : 'm' is normal light, 'a' is no light, 'z' is double bright
	i := int(cl.time * 10)
	for j := 0; j < MAX_LIGHTSTYLES; j++ {
		if cl_lightstyle[j].length == 0 {
			d_lightstylevalue[j] = 256
			continue
		}

		k := i % cl_lightstyle[j].length
		k = int(cl_lightstyle[j].map_[k]) - 'a'
		k = k * 22
		d_lightstylevalue[j] = k
	}
}

func R_MarkLights(model *model_t, light *dlight_t, bit int, nodenum uint32) {
	if nodenum >= uint32(model.numnodes) {
		return
	}

	node := &model.nodes[nodenum]

	splitplane := &model.planes[node.planenum]
	dist := PlaneDiff(light.origin, splitplane)

	if dist > light.radius {
		R_MarkLights(model, light, bit, node.childrennum[0])
		return
	}
	if dist < -light.radius {
		R_MarkLights(model, light, bit, node.childrennum[1])
		return
	}

	dlightframecount := r_dlightframecount

	// mark the polygons
	for i := uint32(0); i < node.numsurfaces; i++ {
		surf := &cl.worldmodel.surfaces[node.firstsurface+i]
		if surf.dlightframe != dlightframecount {
			surf.dlightbits = 0
			surf.dlightframe = dlightframecount
		}
		surf.dlightbits |= bit
	}

	R_MarkLights(model, light, bit, node.childrennum[0])
	R_MarkLights(model, light, bit, node.childrennum[1])
}

func R_PushDlights() {
	if gl_flashblend.value != 0 {
		return
	}

	r_dlightframecount = r_framecount + 1 // because the count hasn't advanced yet for this frame

	for i := uint(0); i < MAX_DLIGHTS/32; i++ {
		if cl_dlight_active[i] != 0 {
			for j := uint(0); j < 32; j++ {
				if (cl_dlight_active[i]&(1<<j)) != 0 && i*32+j < MAX_DLIGHTS {
					l := &cl_dlights[i*32+j]

					// C shifts a 32-bit int by up to 63 here, x86 masks the count to 5 bits
					R_MarkLights(cl.worldmodel, l, 1<<((i*32+j)&31), 0)
				}
			}
		}
	}
}

var lightcolor vec3_t

func texVec3(v *[4]float32) vec3_t {
	return vec3_t{v[0], v[1], v[2]}
}

func RecursiveLightPoint(model *model_t, color *vec3_t, nodenum uint32, start, end vec3_t) bool {
	var node *mnode_t
	var plane *mplane_t
	var front, back float32
	var mid vec3_t

	for {
		if nodenum >= uint32(model.numnodes) {
			return false // didn't hit anything
		}

		node = &model.nodes[nodenum]

		plane = &model.planes[node.planenum]

		// calculate mid point
		if plane.type_ < 3 {
			front = start[plane.type_] - plane.dist
			back = end[plane.type_] - plane.dist
		} else {
			front = DotProduct(start, plane.normal) - plane.dist
			back = DotProduct(end, plane.normal) - plane.dist
		}
		// optimized recursion
		if (back < 0) != (front < 0) {
			break
		}
		nodenum = node.childrennum[b2i(front < 0)]
	}

	frac := front / (front - back)
	mid[0] = start[0] + (end[0]-start[0])*frac
	mid[1] = start[1] + (end[1]-start[1])*frac
	mid[2] = start[2] + (end[2]-start[2])*frac

	// go down front side
	if RecursiveLightPoint(model, color, node.childrennum[b2i(front < 0)], start, mid) {
		return true // hit something
	}

	// check for impact on this node
	for i := uint32(0); i < node.numsurfaces; i++ {
		surf := &cl.worldmodel.surfaces[node.firstsurface+i]
		if cl.worldmodel.surfflags[node.firstsurface+i]&SURF_DRAWTILED != 0 {
			continue // no lightmaps
		}
		ds := int(DotProduct(mid, texVec3(&surf.texinfo.vecs[0])) + surf.texinfo.vecs[0][3])
		dt := int(DotProduct(mid, texVec3(&surf.texinfo.vecs[1])) + surf.texinfo.vecs[1][3])
		if ds < int(surf.texturemins[0]) || dt < int(surf.texturemins[1]) {
			continue
		}

		ds -= int(surf.texturemins[0])
		dt -= int(surf.texturemins[1])

		if ds > int(surf.extents[0]) || dt > int(surf.extents[1]) {
			continue
		}

		if surf.samples != nil {
			// enhanced to interpolate lighting
			var r00, g00, b00, r01, g01, b01, r10, g10, b10, r11, g11, b11 int
			dsfrac := ds & 15
			dtfrac := dt & 15
			line3 := ((int(surf.extents[0]) >> 4) + 1) * 3
			lightmap := ((dt>>4)*((int(surf.extents[0])>>4)+1) + (ds >> 4)) * 3 // LordHavoc: *3 for color

			for maps := 0; maps < MAXLIGHTMAPS && surf.styles[maps] != 255; maps++ {
				lm := surf.samples[lightmap:]
				scale := float32(float64(d_lightstylevalue[surf.styles[maps]]) * 1.0 / 256.0)
				r00 = int(float32(r00) + float32(lm[0])*scale)
				g00 = int(float32(g00) + float32(lm[1])*scale)
				b00 = int(float32(b00) + float32(lm[2])*scale)

				r01 = int(float32(r01) + float32(lm[3])*scale)
				g01 = int(float32(g01) + float32(lm[4])*scale)
				b01 = int(float32(b01) + float32(lm[5])*scale)

				r10 = int(float32(r10) + float32(lm[line3+0])*scale)
				g10 = int(float32(g10) + float32(lm[line3+1])*scale)
				b10 = int(float32(b10) + float32(lm[line3+2])*scale)

				r11 = int(float32(r11) + float32(lm[line3+3])*scale)
				g11 = int(float32(g11) + float32(lm[line3+4])*scale)
				b11 = int(float32(b11) + float32(lm[line3+5])*scale)

				lightmap += ((int(surf.extents[0]) >> 4) + 1) * ((int(surf.extents[1]) >> 4) + 1) * 3 // LordHavoc: *3 for colored lighting
			}
			color[0] += float32((((((((r11 - r10) * dsfrac) >> 4) + r10) - ((((r01 - r00) * dsfrac) >> 4) + r00)) * dtfrac) >> 4) +
				((((r01 - r00) * dsfrac) >> 4) + r00))
			color[1] += float32((((((((g11 - g10) * dsfrac) >> 4) + g10) - ((((g01 - g00) * dsfrac) >> 4) + g00)) * dtfrac) >> 4) +
				((((g01 - g00) * dsfrac) >> 4) + g00))
			color[2] += float32((((((((b11 - b10) * dsfrac) >> 4) + b10) - ((((b01 - b00) * dsfrac) >> 4) + b00)) * dtfrac) >> 4) +
				((((b01 - b00) * dsfrac) >> 4) + b00))
		}
		return true // success
	}
	// go down back side
	return RecursiveLightPoint(model, color, node.childrennum[b2i(front >= 0)], mid, end)
}

func R_LightPoint(p vec3_t) int {
	var end vec3_t

	if cl.worldmodel.lightdata == nil {
		return 255
	}

	end[0] = p[0]
	end[1] = p[1]
	end[2] = p[2] - 2048

	lightcolor[0], lightcolor[1], lightcolor[2] = 0, 0, 0
	RecursiveLightPoint(cl.worldmodel, &lightcolor, 0, p, end)
	return int(float64(lightcolor[0]+lightcolor[1]+lightcolor[2]) / 3.0)
}

// ---- frame setup ----

func R_SetupFrame() {
	var testorigin vec3_t

	// don't allow cheats in multiplayer
	r_fullbright.value = 0
	r_lightmap.value = 0

	R_AnimateLight()

	r_framecount++

	// build the transformation matrix for the given view angles
	r_origin = r_refdef.vieworg
	AngleVectors(r_refdef.viewangles, &vpn, &vright, &vup)

	// current viewleaf
	r_oldviewleaf = r_viewleaf
	r_oldviewleaf2 = r_viewleaf2

	r_viewleaf = Mod_PointInLeaf(r_origin, cl.worldmodel)
	r_viewleaf2 = nil

	// check above and below so crossing solid water doesn't draw wrong
	if r_viewleaf.contents <= CONTENTS_WATER && r_viewleaf.contents >= CONTENTS_LAVA {
		// look up a bit
		testorigin = r_origin
		testorigin[2] += 10
		leaf := Mod_PointInLeaf(testorigin, cl.worldmodel)
		if leaf.contents == CONTENTS_EMPTY {
			r_viewleaf2 = leaf
		}
	} else if r_viewleaf.contents == CONTENTS_EMPTY {
		// look down a bit
		testorigin = r_origin
		testorigin[2] -= 10
		leaf := Mod_PointInLeaf(testorigin, cl.worldmodel)
		if leaf.contents <= CONTENTS_WATER && leaf.contents >= CONTENTS_LAVA {
			r_viewleaf2 = leaf
		}
	}

	V_SetContentsColor(int(r_viewleaf.contents))
	V_CalcBlend()

	r_cache_thrash = false

	c_brush_polys = 0
	c_alias_polys = 0
}

// projection and view matrices, replaces R_SetupGL
func R_SetupScene() {
	var proj, glproj, view [16]float32
	var vp [4]int

	// viewport, top-left origin for SDL_GPU
	x := r_refdef.vrect.x * glwidth / int(vid.conwidth)
	x2 := (r_refdef.vrect.x + r_refdef.vrect.width) * glwidth / int(vid.conwidth)
	y := r_refdef.vrect.y * glheight / int(vid.conheight)
	y2 := (r_refdef.vrect.y + r_refdef.vrect.height) * glheight / int(vid.conheight)
	w := x2 - x
	h := y2 - y

	Scene_SetViewport(float32(x), float32(y), float32(w), float32(h))

	aspect := float32(r_refdef.vrect.width) / float32(r_refdef.vrect.height)
	farclip := max(int(r_farclip.value), 4096)
	znear := float32(4)
	zfar := float32(farclip)
	f := float32(1 / math.Tan(float64(r_refdef.fov_y)*math.Pi/360.0))

	// projection, clip-space depth 0..1
	proj[0] = f / aspect
	proj[5] = f
	proj[10] = zfar / (znear - zfar)
	proj[11] = -1
	proj[14] = znear * zfar / (znear - zfar)

	// view, rotation rows vright / vup / -vpn
	Mat4_Identity(view[:])
	view[0], view[4], view[8] = vright[0], vright[1], vright[2]
	view[1], view[5], view[9] = vup[0], vup[1], vup[2]
	view[2], view[6], view[10] = -vpn[0], -vpn[1], -vpn[2]
	view[12] = -DotProduct(vright, r_refdef.vieworg)
	view[13] = -DotProduct(vup, r_refdef.vieworg)
	view[14] = DotProduct(vpn, r_refdef.vieworg)

	Mat4_Multiply(proj[:], view[:], r_viewproj[:])

	// GL-convention projection and bottom-left viewport for qglProject
	glproj = proj
	glproj[10] = -(zfar + znear) / (zfar - znear)
	glproj[14] = -2 * zfar * znear / (zfar - znear)

	vp[0] = x
	vp[1] = glheight - (y + h)
	vp[2] = w
	vp[3] = h
	GPU_SetSceneMatrices(view[:], glproj[:], vp[:])
}

// ---- entities ----

func R_DrawEntitiesOnList(vislist *visentlist_t) {
	if r_drawentities.value == 0 || vislist.count == 0 {
		return
	}

	for i := 0; i < vislist.count; i++ {
		currententity = &vislist.list[i]
		switch currententity.model.type_ {
		case mod_brush:
			R_DrawBrushModel(currententity)
		case mod_alias:
			R_DrawAliasModel(currententity)
		case mod_sprite:
			R_DrawSpriteModel(currententity)
		}
	}
}

// ---- screen blends, post shader handles both ----

func R_PolyBlend() {
}

func R_BrightenScreen() {
}

// ---- frame entry ----

func R_RenderView() {
	var time1, time2 float64

	if r_worldentity.model == nil || cl.worldmodel == nil {
		Sys_Error("R_RenderView: NULL worldmodel")
	}

	if r_speeds.value != 0 {
		time1 = Sys_DoubleTime()
		c_brush_polys = 0
		c_alias_polys = 0
	}

	// no R_Clear, the scene pass clears its targets

	R_SetupFrame()

	R_SetFrustum()

	R_SetupScene()

	R_MarkLeaves() // done here so we know if we're in water

	R_DrawWorld() // adds static entities to the list

	R_DrawEntitiesOnList(&cl_visents)
	R_DrawEntitiesOnList(&cl_alphaents)

	R_DrawWaterSurfaces()

	R_RenderDlights()
	R_DrawParticles()

	R_DrawViewModel()

	if r_speeds.value != 0 {
		time2 = Sys_DoubleTime()
		Com_Printf("%3d ms  %4d wpoly %4d epoly\n", int((time2-time1)*1000), c_brush_polys, c_alias_polys)
	}
}

// ---- lifecycle ----

var lightmode int // set to gl_lightmode on mapchange

var r_notexture_mip *texture_t

func R_InitTextures() int {
	// create a simple checkerboard texture for the default
	r_notexture_mip = &texture_t{pixels: make([]byte, 16*16+8*8+4*4+2*2)}
	r_notexture_mip.name = "notexture"
	r_notexture_mip.width = 16
	r_notexture_mip.height = 16
	r_notexture_mip.offsets[0] = 0
	r_notexture_mip.offsets[1] = r_notexture_mip.offsets[0] + 16*16
	r_notexture_mip.offsets[2] = r_notexture_mip.offsets[1] + 8*8
	r_notexture_mip.offsets[3] = r_notexture_mip.offsets[2] + 4*4

	for m := 0; m < 4; m++ {
		dest := r_notexture_mip.offsets[m]
		for y := 0; y < (16 >> m); y++ {
			for x := 0; x < (16 >> m); x++ {
				if (y < (8 >> m)) != (x < (8 >> m)) {
					r_notexture_mip.pixels[dest] = 0
				} else {
					r_notexture_mip.pixels[dest] = 0x0e
				}
				dest++
			}
		}
	}

	return 1
}

func R_ShutdownTextures() {
	r_notexture_mip = nil
}

func R_Init() int {
	R_Texture_Init()

	if R_InitTextures() == 0 {
		return 0
	}

	if R_InitParticles() == 0 {
		R_ShutdownTextures()
		return 0
	}

	return 1
}

func R_InitGL() {
	Classic_LoadParticleTextures()
	R_InitOtherTextures()
}

func R_TimeRefresh_f() {
	if cls.state != ca_active {
		return
	}

	if !(cl.spectator != 0 || cls.demoplayback || cl.standby) && !Ruleset_AllowTimeRefresh() {
		Com_Printf("Timerefresh's disabled during match\n")
		return
	}

	start := float32(Sys_DoubleTime())
	for i := 0; i < 128; i++ {
		r_refdef.viewangles[1] = float32(float64(i) * (360.0 / 128.0))
		SCR_UpdateScreen()
	}

	stop := float32(Sys_DoubleTime())
	time := stop - start
	Com_Printf("%f seconds (%f fps)\n", time, 128/time)
}

func R_Shutdown() {
	World_Shutdown()
	R_ShutdownParticles()
	R_ShutdownTextures()
	R_Texture_Shutdown()
}

func R_PreMapLoad() {
	if !dedicated {
		if gl_lightmode.value == 0 {
			lightmode = 0
		} else {
			lightmode = 2
		}
	}
}

func R_NewMap() {
	for i := 0; i < 256; i++ {
		d_lightstylevalue[i] = 264 // normal light value
	}

	r_worldentity = entity_t{}
	r_worldentity.model = cl.worldmodel

	// clear out efrags in case the level hasn't been reloaded
	for i := 0; i < cl.worldmodel.numleafs; i++ {
		cl.worldmodel.leafs[i].efrags = nil
	}

	r_viewleaf = nil
	R_ClearParticles()

	R_BuildLightmaps()

	for i := 0; i < cl.worldmodel.numtextures; i++ {
		if cl.worldmodel.textures[i] == nil {
			continue
		}
		for waterline := 0; waterline < 2; waterline++ {
			cl.worldmodel.textures[i].texturechain[waterline] = nil
			cl.worldmodel.textures[i].texturechain_tail[waterline] = &cl.worldmodel.textures[i].texturechain[waterline]
		}
	}
}

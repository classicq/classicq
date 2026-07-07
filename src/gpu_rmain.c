/*
Copyright (C) 1996-1997 Id Software, Inc.
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

// 3D frame setup for the SDL_GPU renderer; records scene work, no GPU calls here

#include <stdlib.h>
#include <string.h>
#include <math.h>

#include "quakedef.h"
#include "gl_local.h"
#include "sound.h"
#include "screen.h"
#include "ruleset.h"
#include "gpu_local.h"
#include "gpu_render.h"

void R_MarkLeaves(void);	// gpu_world.c

float r_viewproj[16];

int r_dlightframecount;

// ---- matrix helpers, column-major GL layout ----

void Mat4_Identity(float *m)
{
	memset(m, 0, 16 * sizeof(*m));
	m[0] = m[5] = m[10] = m[15] = 1;
}

void Mat4_Multiply(const float *a, const float *b, float *out)
{
	float tmp[16];
	int col, row;

	for (col = 0; col < 4; col++)
	{
		for (row = 0; row < 4; row++)
		{
			tmp[col*4+row] = a[0*4+row] * b[col*4+0]
			               + a[1*4+row] * b[col*4+1]
			               + a[2*4+row] * b[col*4+2]
			               + a[3*4+row] * b[col*4+3];
		}
	}

	memcpy(out, tmp, sizeof(tmp));
}

void Mat4_Translate(float *m, float x, float y, float z)
{
	m[12] += m[0]*x + m[4]*y + m[8]*z;
	m[13] += m[1]*x + m[5]*y + m[9]*z;
	m[14] += m[2]*x + m[6]*y + m[10]*z;
	m[15] += m[3]*x + m[7]*y + m[11]*z;
}

// Rotate* multiply in place like glRotatef, m = m * R

void Mat4_RotateX(float *m, float deg)
{
	float c = cos(deg * (M_PI / 180.0));
	float s = sin(deg * (M_PI / 180.0));
	float t;
	int i;

	for (i = 0; i < 4; i++)
	{
		t = m[4+i];
		m[4+i] = t*c + m[8+i]*s;
		m[8+i] = m[8+i]*c - t*s;
	}
}

void Mat4_RotateY(float *m, float deg)
{
	float c = cos(deg * (M_PI / 180.0));
	float s = sin(deg * (M_PI / 180.0));
	float t;
	int i;

	for (i = 0; i < 4; i++)
	{
		t = m[i];
		m[i] = t*c - m[8+i]*s;
		m[8+i] = t*s + m[8+i]*c;
	}
}

void Mat4_RotateZ(float *m, float deg)
{
	float c = cos(deg * (M_PI / 180.0));
	float s = sin(deg * (M_PI / 180.0));
	float t;
	int i;

	for (i = 0; i < 4; i++)
	{
		t = m[i];
		m[i] = t*c + m[4+i]*s;
		m[4+i] = m[4+i]*c - t*s;
	}
}

// ---- culling ----

//Returns true if the box is completely outside the frustum
qboolean R_CullBox(vec3_t mins, vec3_t maxs)
{
	int i;

	for (i = 0; i < 4; i++)
	{
		if (BOX_ON_PLANE_SIDE(mins, maxs, &frustum[i]) == 2)
			return true;
	}
	return false;
}

//Returns true if the sphere is completely outside the frustum
qboolean R_CullSphere(vec3_t centre, float radius)
{
	int i;
	mplane_t *p;

	for (i = 0, p = frustum; i < 4; i++, p++)
	{
		if (PlaneDiff(centre, p) <= -radius)
			return true;
	}

	return false;
}

static int SignbitsForPlane(mplane_t *out)
{
	int bits, j;

	// for fast box on planeside test
	bits = 0;
	for (j = 0; j < 3; j++)
	{
		if (out->normal[j] < 0)
			bits |= 1 << j;
	}
	return bits;
}

static void R_SetFrustum(void)
{
	int i;

	// rotate VPN right by FOV_X/2 degrees
	RotatePointAroundVector(frustum[0].normal, vup, vpn, -(90 - r_refdef.fov_x / 2));
	// rotate VPN left by FOV_X/2 degrees
	RotatePointAroundVector(frustum[1].normal, vup, vpn, 90 - r_refdef.fov_x / 2);
	// rotate VPN up by FOV_Y/2 degrees
	RotatePointAroundVector(frustum[2].normal, vright, vpn, 90 - r_refdef.fov_y / 2);
	// rotate VPN down by FOV_Y/2 degrees
	RotatePointAroundVector(frustum[3].normal, vright, vpn, -(90 - r_refdef.fov_y / 2));

	for (i = 0; i < 4; i++)
	{
		frustum[i].type = PLANE_ANYZ;
		frustum[i].dist = DotProduct(r_origin, frustum[i].normal);
		frustum[i].signbits = SignbitsForPlane(&frustum[i]);
	}
}

// ---- lights ----

void R_AnimateLight(void)
{
	int i, j, k;

	// light animations : 'm' is normal light, 'a' is no light, 'z' is double bright
	i = (int) (cl.time * 10);
	for (j = 0; j < MAX_LIGHTSTYLES; j++)
	{
		if (!cl_lightstyle[j].length)
		{
			d_lightstylevalue[j] = 256;
			continue;
		}

		k = i % cl_lightstyle[j].length;
		k = cl_lightstyle[j].map[k] - 'a';
		k = k * 22;
		d_lightstylevalue[j] = k;
	}
}

void R_MarkLights(model_t *model, dlight_t *light, int bit, unsigned int nodenum)
{
	mnode_t *node;
	mplane_t *splitplane;
	float dist;
	msurface_t *surf;
	int i;
	unsigned dlightframecount;

	if (nodenum >= model->numnodes)
		return;

	node = model->nodes + nodenum;

	splitplane = model->planes + node->planenum;
	dist = PlaneDiff(light->origin, splitplane);

	if (dist > light->radius)
	{
		R_MarkLights(model, light, bit, node->childrennum[0]);
		return;
	}
	if (dist < -light->radius)
	{
		R_MarkLights(model, light, bit, node->childrennum[1]);
		return;
	}

	dlightframecount = r_dlightframecount;

	// mark the polygons
	surf = cl.worldmodel->surfaces + node->firstsurface;
	for (i = 0; i < node->numsurfaces; i++, surf++)
	{
		if (surf->dlightframe != dlightframecount)
		{
			surf->dlightbits = 0;
			surf->dlightframe = dlightframecount;
		}
		surf->dlightbits |= bit;
	}

	R_MarkLights(model, light, bit, node->childrennum[0]);
	R_MarkLights(model, light, bit, node->childrennum[1]);
}

void R_PushDlights(void)
{
	unsigned int i;
	unsigned int j;
	dlight_t *l;

	if (gl_flashblend.value)
		return;

	r_dlightframecount = r_framecount + 1;	// because the count hasn't advanced yet for this frame

	for (i = 0; i < MAX_DLIGHTS/32; i++)
	{
		if (cl_dlight_active[i])
		{
			for (j = 0; j < 32; j++)
			{
				if ((cl_dlight_active[i] & (1<<j)) && i*32+j < MAX_DLIGHTS)
				{
					l = cl_dlights + i*32 + j;

					R_MarkLights(cl.worldmodel, l, 1<<(i*32 + j), 0);
				}
			}
		}
	}
}

static mplane_t *lightplane;
static vec3_t lightspot;
static vec3_t lightcolor;

static int RecursiveLightPoint(model_t *model, vec3_t color, unsigned int nodenum, vec3_t start, vec3_t end)
{
	mnode_t *node;
	mplane_t *plane;
	float front, back, frac;
	vec3_t mid;

loc0:
	if (nodenum >= model->numnodes)
		return false;		// didn't hit anything

	node = model->nodes + nodenum;

	plane = model->planes + node->planenum;

	// calculate mid point
	if (plane->type < 3)
	{
		front = start[plane->type] - plane->dist;
		back = end[plane->type] - plane->dist;
	}
	else
	{
		front = DotProduct(start, plane->normal) - plane->dist;
		back = DotProduct(end, plane->normal) - plane->dist;
	}
	// optimized recursion
	if ((back < 0) == (front < 0))
	{
		nodenum = node->childrennum[front < 0];
		goto loc0;
	}

	frac = front / (front-back);
	mid[0] = start[0] + (end[0] - start[0]) * frac;
	mid[1] = start[1] + (end[1] - start[1]) * frac;
	mid[2] = start[2] + (end[2] - start[2]) * frac;

	// go down front side
	if (RecursiveLightPoint(model, color, node->childrennum[front < 0], start, mid))
	{
		return true;	// hit something
	}
	else
	{
		int i, ds, dt;
		msurface_t *surf;
		// check for impact on this node
		VectorCopy (mid, lightspot);
		lightplane = plane;
		surf = cl.worldmodel->surfaces + node->firstsurface;
		for (i = 0; i < node->numsurfaces; i++, surf++)
		{
			if (cl.worldmodel->surfflags[node->firstsurface + i] & SURF_DRAWTILED)
				continue;	// no lightmaps
			ds = (int) ((float) DotProduct (mid, surf->texinfo->vecs[0]) + surf->texinfo->vecs[0][3]);
			dt = (int) ((float) DotProduct (mid, surf->texinfo->vecs[1]) + surf->texinfo->vecs[1][3]);
			if (ds < surf->texturemins[0] || dt < surf->texturemins[1])
				continue;

			ds -= surf->texturemins[0];
			dt -= surf->texturemins[1];

			if (ds > surf->extents[0] || dt > surf->extents[1])
				continue;

			if (surf->samples)
			{
				//enhanced to interpolate lighting
				byte *lightmap;
				int maps, line3, dsfrac = ds & 15, dtfrac = dt & 15, r00 = 0, g00 = 0, b00 = 0, r01 = 0, g01 = 0, b01 = 0, r10 = 0, g10 = 0, b10 = 0, r11 = 0, g11 = 0, b11 = 0;
				float scale;
				line3 = ((surf->extents[0] >> 4) + 1) * 3;
				lightmap = surf->samples + ((dt >> 4) * ((surf->extents[0] >> 4) + 1) + (ds >> 4)) * 3; // LordHavoc: *3 for color

				for (maps = 0; maps < MAXLIGHTMAPS && surf->styles[maps] != 255; maps++)
				{
					scale = (float) d_lightstylevalue[surf->styles[maps]] * 1.0 / 256.0;
					r00 += (float) lightmap[0] * scale;
					g00 += (float) lightmap[1] * scale;
					b00 += (float) lightmap[2] * scale;

					r01 += (float) lightmap[3] * scale;
					g01 += (float) lightmap[4] * scale;
					b01 += (float) lightmap[5] * scale;

					r10 += (float) lightmap[line3 + 0] * scale;
					g10 += (float) lightmap[line3 + 1] * scale;
					b10 += (float) lightmap[line3 + 2] * scale;

					r11 += (float) lightmap[line3 + 3] * scale;
					g11 += (float) lightmap[line3 + 4] * scale;
					b11 += (float) lightmap[line3 + 5] * scale;

					lightmap += ((surf->extents[0] >> 4) + 1) * ((surf->extents[1] >> 4) + 1) * 3; // LordHavoc: *3 for colored lighting
				}
				color[0] += (float) ((int) ((((((((r11 - r10) * dsfrac) >> 4) + r10)
					- ((((r01 - r00) * dsfrac) >> 4) + r00)) * dtfrac) >> 4)
					+ ((((r01 - r00) * dsfrac) >> 4) + r00)));
				color[1] += (float) ((int) ((((((((g11 - g10) * dsfrac) >> 4) + g10)
					- ((((g01 - g00) * dsfrac) >> 4) + g00)) * dtfrac) >> 4)
					+ ((((g01 - g00) * dsfrac) >> 4) + g00)));
				color[2] += (float) ((int) ((((((((b11 - b10) * dsfrac) >> 4) + b10)
					- ((((b01 - b00) * dsfrac) >> 4) + b00)) * dtfrac) >> 4)
					+ ((((b01 - b00) * dsfrac) >> 4) + b00)));
			}
			return true; // success
		}
		// go down back side
		return RecursiveLightPoint(model, color, node->childrennum[front >= 0], mid, end);
	}
}

int R_LightPoint(vec3_t p)
{
	vec3_t end;

	if (!cl.worldmodel->lightdata)
		return 255;

	end[0] = p[0];
	end[1] = p[1];
	end[2] = p[2] - 2048;

	lightcolor[0] = lightcolor[1] = lightcolor[2] = 0;
	RecursiveLightPoint(cl.worldmodel, lightcolor, 0, p, end);
	return (lightcolor[0] + lightcolor[1] + lightcolor[2]) / 3.0;
}

// ---- frame setup ----

static void R_SetupFrame(void)
{
	vec3_t testorigin;
	mleaf_t *leaf;

	// don't allow cheats in multiplayer
	r_fullbright.value = 0;
	r_lightmap.value = 0;

	R_AnimateLight();

	r_framecount++;

	// build the transformation matrix for the given view angles
	VectorCopy(r_refdef.vieworg, r_origin);
	AngleVectors(r_refdef.viewangles, vpn, vright, vup);

	// current viewleaf
	r_oldviewleaf = r_viewleaf;
	r_oldviewleaf2 = r_viewleaf2;

	r_viewleaf = Mod_PointInLeaf(r_origin, cl.worldmodel);
	r_viewleaf2 = NULL;

	// check above and below so crossing solid water doesn't draw wrong
	if (r_viewleaf->contents <= CONTENTS_WATER && r_viewleaf->contents >= CONTENTS_LAVA)
	{
		// look up a bit
		VectorCopy(r_origin, testorigin);
		testorigin[2] += 10;
		leaf = Mod_PointInLeaf(testorigin, cl.worldmodel);
		if (leaf->contents == CONTENTS_EMPTY)
			r_viewleaf2 = leaf;
	}
	else if (r_viewleaf->contents == CONTENTS_EMPTY)
	{
		// look down a bit
		VectorCopy(r_origin, testorigin);
		testorigin[2] -= 10;
		leaf = Mod_PointInLeaf(testorigin, cl.worldmodel);
		if (leaf->contents <= CONTENTS_WATER && leaf->contents >= CONTENTS_LAVA)
			r_viewleaf2 = leaf;
	}

	V_SetContentsColor(r_viewleaf->contents);
	V_CalcBlend();

	r_cache_thrash = false;

	c_brush_polys = 0;
	c_alias_polys = 0;
}

// projection and view matrices, replaces R_SetupGL
static void R_SetupScene(void)
{
	float proj[16], glproj[16], view[16];
	int vp[4];
	int x, x2, y, y2, w, h, farclip;
	float f, aspect, znear, zfar;

	// viewport, top-left origin for SDL_GPU
	x = r_refdef.vrect.x * glwidth / vid.conwidth;
	x2 = (r_refdef.vrect.x + r_refdef.vrect.width) * glwidth / vid.conwidth;
	y = r_refdef.vrect.y * glheight / vid.conheight;
	y2 = (r_refdef.vrect.y + r_refdef.vrect.height) * glheight / vid.conheight;
	w = x2 - x;
	h = y2 - y;

	Scene_SetViewport(x, y, w, h);

	aspect = (float)r_refdef.vrect.width / r_refdef.vrect.height;
	farclip = max((int) r_farclip.value, 4096);
	znear = 4;
	zfar = farclip;
	f = 1 / tan(r_refdef.fov_y * M_PI / 360.0);

	// projection, clip-space depth 0..1
	memset(proj, 0, sizeof(proj));
	proj[0] = f / aspect;
	proj[5] = f;
	proj[10] = zfar / (znear - zfar);
	proj[11] = -1;
	proj[14] = znear * zfar / (znear - zfar);

	// view, rotation rows vright / vup / -vpn
	Mat4_Identity(view);
	view[0] = vright[0]; view[4] = vright[1]; view[8] = vright[2];
	view[1] = vup[0];    view[5] = vup[1];    view[9] = vup[2];
	view[2] = -vpn[0];   view[6] = -vpn[1];   view[10] = -vpn[2];
	view[12] = -DotProduct(vright, r_refdef.vieworg);
	view[13] = -DotProduct(vup, r_refdef.vieworg);
	view[14] = DotProduct(vpn, r_refdef.vieworg);

	Mat4_Multiply(proj, view, r_viewproj);

	// GL-convention projection and bottom-left viewport for qglProject
	memcpy(glproj, proj, sizeof(glproj));
	glproj[10] = -(zfar + znear) / (zfar - znear);
	glproj[14] = -2 * zfar * znear / (zfar - znear);

	vp[0] = x;
	vp[1] = glheight - (y + h);
	vp[2] = w;
	vp[3] = h;
	GPU_SetSceneMatrices(view, glproj, vp);
}

// ---- entities ----

void R_DrawEntitiesOnList(visentlist_t *vislist)
{
	int i;

	if (!r_drawentities.value || !vislist->count)
		return;

	for (i = 0; i < vislist->count; i++)
	{
		currententity = &vislist->list[i];
		switch (currententity->model->type)
		{
			case mod_brush:
				R_DrawBrushModel(currententity);
				break;
			case mod_alias:
				R_DrawAliasModel(currententity);
				break;
			case mod_sprite:
				R_DrawSpriteModel(currententity);
				break;
		}
	}
}

// ---- screen blends, post shader handles both ----

void R_PolyBlend(void)
{
}

void R_BrightenScreen(void)
{
}

// ---- frame entry ----

void R_RenderView(void)
{
	double time1 = 0, time2;

	if (!r_worldentity.model || !cl.worldmodel)
		Sys_Error("R_RenderView: NULL worldmodel");

	if (r_speeds.value)
	{
		time1 = Sys_DoubleTime();
		c_brush_polys = 0;
		c_alias_polys = 0;
	}

	// no R_Clear, the scene pass clears its targets

	R_SetupFrame();

	R_SetFrustum();

	R_SetupScene();

	R_MarkLeaves();	// done here so we know if we're in water

	R_DrawWorld();		// adds static entities to the list

	S_ExtraUpdate();	// don't let sound get messed up if going slow

	R_DrawEntitiesOnList(&cl_visents);
	R_DrawEntitiesOnList(&cl_alphaents);

	R_DrawWaterSurfaces();

	R_RenderDlights();
	R_DrawParticles();

	R_DrawViewModel();

	if (r_speeds.value)
	{
		time2 = Sys_DoubleTime();
		Com_Printf("%3i ms  %4i wpoly %4i epoly\n", (int)((time2 - time1) * 1000), c_brush_polys, c_alias_polys);
	}
}

// ---- lifecycle ----

int R_InitTextures(void)
{
	int x, y, m;
	byte *dest;

	// create a simple checkerboard texture for the default
	r_notexture_mip = malloc(sizeof(texture_t) + 16 * 16 + 8 * 8 + 4 * 4 + 2 * 2);
	if (r_notexture_mip)
	{
		strcpy(r_notexture_mip->name, "notexture");
		r_notexture_mip->width = r_notexture_mip->height = 16;
		r_notexture_mip->offsets[0] = sizeof(texture_t);
		r_notexture_mip->offsets[1] = r_notexture_mip->offsets[0] + 16 * 16;
		r_notexture_mip->offsets[2] = r_notexture_mip->offsets[1] + 8 * 8;
		r_notexture_mip->offsets[3] = r_notexture_mip->offsets[2] + 4 * 4;

		for (m = 0; m < 4; m++)
		{
			dest = (byte *) r_notexture_mip + r_notexture_mip->offsets[m];
			for (y = 0; y < (16 >> m); y++)
			{
				for (x = 0; x < (16 >> m); x++)
				{
					if ((y < (8 >> m)) ^ (x < (8 >> m)))
						*dest++ = 0;
					else
						*dest++ = 0x0e;
				}
			}
		}

		return 1;
	}

	return 0;
}

void R_ShutdownTextures(void)
{
	free(r_notexture_mip);
	r_notexture_mip = NULL;
}

int R_Init(void)
{
	GL_Texture_Init();

	if (!R_InitTextures())
		return 0;

	if (!R_InitParticles())
	{
		R_ShutdownTextures();
		return 0;
	}

	return 1;
}

void R_InitGL(void)
{
	Classic_LoadParticleTextures();
	R_InitOtherTextures();
}

void R_TimeRefresh_f(void)
{
	int i;
	float start, stop, time;

	if (cls.state != ca_active)
		return;

	if (!(cl.spectator || cls.demoplayback || cl.standby) && !Ruleset_AllowTimeRefresh())
	{
		Com_Printf("Timerefresh's disabled during match\n");
		return;
	}

	start = Sys_DoubleTime();
	for (i = 0; i < 128; i++)
	{
		r_refdef.viewangles[1] = i * (360.0 / 128.0);
		SCR_UpdateScreen();
	}

	stop = Sys_DoubleTime();
	time = stop - start;
	Com_Printf("%f seconds (%f fps)\n", time, 128 / time);
}

void R_Shutdown(void)
{
	World_Shutdown();
	R_ShutdownParticles();
	R_ShutdownTextures();
	GL_Texture_Shutdown();
}

void R_PreMapLoad(void)
{
	if (!dedicated)
		lightmode = gl_lightmode.value == 0 ? 0 : 2;
}

void R_NewMap(void)
{
	int i, waterline;

	for (i = 0; i < 256; i++)
		d_lightstylevalue[i] = 264;		// normal light value

	memset(&r_worldentity, 0, sizeof(r_worldentity));
	r_worldentity.model = cl.worldmodel;

	// clear out efrags in case the level hasn't been reloaded
	for (i = 0; i < cl.worldmodel->numleafs; i++)
		cl.worldmodel->leafs[i].efrags = NULL;

	r_viewleaf = NULL;
	R_ClearParticles();

	GL_BuildLightmaps();

	for (i = 0; i < cl.worldmodel->numtextures; i++)
	{
		if (!cl.worldmodel->textures[i])
			continue;
		for (waterline = 0; waterline < 2; waterline++)
		{
			cl.worldmodel->textures[i]->texturechain[waterline] = NULL;
			cl.worldmodel->textures[i]->texturechain_tail[waterline] = &cl.worldmodel->textures[i]->texturechain[waterline];
		}
	}
}

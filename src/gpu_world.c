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

// world, brush model, lightmap, sky and water rendering for the SDL_GPU path
// ported from gl_rsurf.c and gl_warp.c

#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <math.h>

#include "quakedef.h"
#include "r_local.h"
#include "gpu_local.h"
#include "gpu_render.h"
#include "utils.h"

#define BLOCK_WIDTH		128
#define BLOCK_HEIGHT	128

#define MAX_LIGHTMAP_SIZE	(32 * 32)
#define MAX_LIGHTMAPS		512

extern cvar_t r_drawflat_enable;
extern cvar_t r_fastturb;
extern cvar_t gl_colorlights;

static unsigned int blocklights[MAX_LIGHTMAP_SIZE * 3];

typedef struct glRect_s
{
	unsigned char l, t, w, h;
} glRect_t;

static qboolean lightmap_modified[MAX_LIGHTMAPS];
static glRect_t lightmap_rectchange[MAX_LIGHTMAPS];
static SDL_GPUTexture *lightmap_pages[MAX_LIGHTMAPS];
static SDL_GPUTransferBuffer *lightmap_transfer;

static int allocated[MAX_LIGHTMAPS][BLOCK_WIDTH];

// CPU copy, RGBA so uploads match the page format
static byte lightmaps[4 * MAX_LIGHTMAPS * BLOCK_WIDTH * BLOCK_HEIGHT];

static msurface_t *skychain = NULL;
static msurface_t **skychain_tail = &skychain;

static msurface_t *waterchain = NULL;
static msurface_t **waterchain_tail = &waterchain;

static msurface_t *alphachain = NULL;
static msurface_t **alphachain_tail = &alphachain;

static msurface_t *drawflatchain = NULL;
static msurface_t **drawflatchain_tail = &drawflatchain;

#define CHAIN_SURF_F2B(surf, chain_tail)		\
	{											\
		*(chain_tail) = (surf);					\
		(chain_tail) = &(surf)->texturechain;	\
		(surf)->texturechain = NULL;			\
	}

#define CHAIN_SURF_B2F(surf, chain) 			\
	{											\
		(surf)->texturechain = (chain);			\
		(chain) = (surf);						\
	}

static glpoly_t *fullbright_polys[MAX_GLTEXTURES];
static unsigned int fullbright_polys_used[(MAX_GLTEXTURES+31)/32];

static glpoly_t *luma_polys[MAX_GLTEXTURES];
static unsigned int luma_polys_used[(MAX_GLTEXTURES+31)/32];

static qboolean drawfullbrights = false, drawlumas = false;

// ---- per-model static vertex buffers ----

typedef struct worldvb_s
{
	msurface_t *surfaces;		// key, shared with inline '*' submodels
	SDL_GPUBuffer *buf;
	scene_vert_t *verts;		// CPU copy kept for drawflat colour rebuilds
	unsigned int numverts;
	unsigned int litverts;
} worldvb_t;

static worldvb_t worldvbs[MAX_MODELS];
static int num_worldvbs;

static worldvb_t *World_FindVB(model_t *model)
{
	int i;

	for (i = 0; i < num_worldvbs; i++)
	{
		if (worldvbs[i].surfaces == model->surfaces)
			return &worldvbs[i];
	}

	return NULL;
}

// mvp and vertex buffer of the entity whose chains are being drawn
static float brush_mvp[16];
static const float *current_mvp = r_viewproj;
static worldvb_t *current_vb;

// drops all world GPU objects; also runs on vid_restart while the old device is alive
void World_Shutdown(void)
{
	int i;

	for (i = 0; i < num_worldvbs; i++)
	{
		GPU_ReleaseBuffer(worldvbs[i].buf);
		free(worldvbs[i].verts);
	}
	memset(worldvbs, 0, sizeof(worldvbs));
	num_worldvbs = 0;
	current_vb = NULL;

	for (i = 0; i < MAX_LIGHTMAPS; i++)
	{
		if (lightmap_pages[i])
		{
			GPU_ReleaseTexture(lightmap_pages[i]);
			lightmap_pages[i] = NULL;
		}
		lightmap_modified[i] = false;
	}
}

// triangle fan over a contiguous vertex range, returns index count
static unsigned int Fan_AllocIndices(unsigned int firstvert, int numverts, unsigned int *firstindex)
{
	unsigned int *idx;
	unsigned int k, n;

	if (numverts < 3)
		return 0;

	n = (numverts - 2) * 3;
	idx = Scene_AllocIndices(n, firstindex);
	if (!idx)
		return 0;

	for (k = 0; k < (unsigned int)numverts - 2; k++)
	{
		idx[k*3+0] = firstvert;
		idx[k*3+1] = firstvert + k + 1;
		idx[k*3+2] = firstvert + k + 2;
	}

	return n;
}

// streamed flat-coloured fan from a surface's fastpolys (fastsky, fastturb)
static void EmitFastPolyColoured(msurface_t *fa, const byte *rgb)
{
	scene_vert_t *v;
	unsigned int firstvert, firstindex, n;
	int i;

	if (!fa->fastpolys || fa->numedges < 3)
		return;

	v = Scene_AllocVerts(fa->numedges, &firstvert);
	if (!v)
		return;

	for (i = 0; i < fa->numedges; i++, v++)
	{
		v->pos[0] = fa->fastpolys[i*3+0];
		v->pos[1] = fa->fastpolys[i*3+1];
		v->pos[2] = fa->fastpolys[i*3+2];
		v->st[0] = v->st[1] = 0;
		v->lm[0] = v->lm[1] = 0;
		v->rgba[0] = rgb[0];
		v->rgba[1] = rgb[1];
		v->rgba[2] = rgb[2];
		v->rgba[3] = 255;
	}

	n = Fan_AllocIndices(firstvert, fa->numedges, &firstindex);
	if (n)
		Scene_AddBatch(SCENE_PIPE_TEX, GPU_Texture_White(), NULL, GPU_GetDynamicSceneVB(), firstindex, n, current_mvp);
}

// ---- dynamic lights ----

typedef struct dlightinfo_s
{
	int local[2];
	int rad;
	int minlight;	// rad - minlight
	int type;
} dlightinfo_t;

static dlightinfo_t dlightlist[MAX_DLIGHTS];

static int R_BuildDlightList(msurface_t *surf)
{
	float dist;
	vec3_t impact;
	mtexinfo_t *tex;
	int lnum, i, smax, tmax, irad, iminlight, local[2], tdmin, sdmin, distmin;
	dlightinfo_t *light;
	unsigned int dlightbits;
	int numdlights;

	numdlights = 0;

	smax = (surf->extents[0]>>4)+1;
	tmax = (surf->extents[1]>>4)+1;
	tex = surf->texinfo;

	dlightbits = surf->dlightbits;

	for (lnum = 0; lnum < MAX_DLIGHTS && dlightbits; lnum++)
	{
		if ( !(dlightbits & (1 << lnum) ) )
			continue;		// not lit by this light

		dlightbits &= ~(1<<lnum);

		dist = PlaneDiff(cl_dlights[lnum].origin, surf->plane);
		irad = (cl_dlights[lnum].radius - fabs(dist)) * 256;
		iminlight = cl_dlights[lnum].minlight * 256;
		if (irad < iminlight)
			continue;

		iminlight = irad - iminlight;

		for (i = 0; i < 3; i++)
			impact[i] = cl_dlights[lnum].origin[i] - surf->plane->normal[i] * dist;

		local[0] = DotProduct (impact, tex->vecs[0]) + tex->vecs[0][3] - surf->texturemins[0];
		local[1] = DotProduct (impact, tex->vecs[1]) + tex->vecs[1][3] - surf->texturemins[1];

		// check if this dlight will touch the surface
		if (local[1] > 0)
		{
			tdmin = local[1] - (tmax << 4);
			if (tdmin < 0)
				tdmin = 0;
		}
		else
		{
			tdmin = -local[1];
		}

		if (local[0] > 0)
		{
			sdmin = local[0] - (smax << 4);
			if (sdmin < 0)
				sdmin = 0;
		}
		else
		{
			sdmin = -local[0];
		}

		if (sdmin > tdmin)
			distmin = (sdmin << 8) + (tdmin << 7);
		else
			distmin = (tdmin << 8) + (sdmin << 7);

		if (distmin < iminlight)
		{
			// save dlight info
			light = &dlightlist[numdlights];
			light->minlight = iminlight;
			light->rad = irad;
			light->local[0] = local[0];
			light->local[1] = local[1];
			light->type = cl_dlights[lnum].type;
			numdlights++;
		}
	}

	return numdlights;
}

static const int dlightcolor[NUM_DLIGHTTYPES][3] =
{
	{ 100, 90, 80 },	// dimlight or brightlight
	{ 100, 50, 10 },	// muzzleflash
	{ 100, 50, 10 },	// explosion
	{ 90, 60, 7 },		// rocket
	{ 128, 0, 0 },		// red
	{ 0, 0, 128 },		// blue
	{ 128, 0, 128 },	// red + blue
	{ 0, 128, 0 },		// green
	{ 128, 128, 128},	// white
};

//R_BuildDlightList must be called first!
static void R_AddDynamicLights(msurface_t *surf, int numdlights)
{
	int i, smax, tmax, s, t, sd, td, _sd, _td, irad, idist, iminlight, color[3], tmp;
	dlightinfo_t *light;
	unsigned int *dest;

	smax = (surf->extents[0]>>4)+1;
	tmax = (surf->extents[1]>>4)+1;

	for (i = 0, light = dlightlist; i < numdlights; i++, light++)
	{
		if (gl_colorlights.value)
		{
			VectorCopy(dlightcolor[light->type], color);
		}
		else
		{
			VectorSet(color, 128, 128, 128);
		}

		irad = light->rad;
		iminlight = light->minlight;

		_td = light->local[1];
		dest = blocklights;
		for (t = 0; t < tmax; t++)
		{
			td = _td;
			if (td < 0)	td = -td;
			_td -= 16;
			_sd = light->local[0];

			for (s = 0; s < smax; s++)
			{
				sd = _sd < 0 ? -_sd : _sd;
				_sd -= 16;
				if (sd > td)
					idist = (sd << 8) + (td << 7);
				else
					idist = (td << 8) + (sd << 7);

				if (idist < iminlight)
				{
					tmp = (irad - idist) >> 7;
					dest[0] += tmp * color[0];
					dest[1] += tmp * color[1];
					dest[2] += tmp * color[2];
				}
				dest += 3;
			}
		}
	}
}

static void AddAllLightMaps(byte *lightmap, msurface_t *surf, int blocksize)
{
	int maps;
	int i;
	unsigned scale;
	unsigned int *bl;

	// add all the lightmaps
	if (lightmap)
	{
		for (maps = 0; maps < MAXLIGHTMAPS && surf->styles[maps] != 255; maps++)
		{
			scale = d_lightstylevalue[surf->styles[maps]];
			surf->cached_light[maps] = scale;	// 8.8 fraction
			bl = blocklights;
			for (i = 0; i < blocksize; i++)
				*bl++ += lightmap[i] * scale;
			lightmap += blocksize;		// skip to next lightmap
		}
	}
}

static void lightmapstore_mode2(int stride, int smax, int tmax, byte *dest)
{
	unsigned int *bl;
	int i;
	int j;
	int t;

	bl = blocklights;

	for (i = 0; i < tmax; i++, dest += stride)
	{
		for (j = smax; j; j--)
		{
			t = bl[0]; t = (t >> 8) + (t >> 9); if (t > 255) t = 255;
			dest[0] = t;
			t = bl[1]; t = (t >> 8) + (t >> 9); if (t > 255) t = 255;
			dest[1] = t;
			t = bl[2]; t = (t >> 8) + (t >> 9); if (t > 255) t = 255;
			dest[2] = t;
			dest[3] = 255;
			bl += 3;
			dest += 4;
		}
	}
}

static void lightmapstore_mode0(int stride, int smax, int tmax, byte *dest)
{
	unsigned int *bl;
	int i;
	int j;
	int t;

	bl = blocklights;

	for (i = 0; i < tmax; i++, dest += stride)
	{
		for (j = smax; j; j--)
		{
			t = bl[0]; t = t >> 7; if (t > 255) t = 255;
			dest[0] = t;
			t = bl[1]; t = t >> 7; if (t > 255) t = 255;
			dest[1] = t;
			t = bl[2]; t = t >> 7; if (t > 255) t = 255;
			dest[2] = t;
			dest[3] = 255;
			bl += 3;
			dest += 4;
		}
	}
}

static void StoreLightMap(int stride, int smax, int tmax, byte *dest)
{
	if (lightmode == 2)
	{
		lightmapstore_mode2(stride, smax, tmax, dest);
	}
	else
	{
		lightmapstore_mode0(stride, smax, tmax, dest);
	}
}

//Combine and scale multiple lightmaps into the 8.8 format in blocklights
static void R_BuildLightMap(msurface_t *surf, byte *dest, int stride, int numdlights)
{
	int smax, tmax, size, i, blocksize;
	byte *lightmap;

	surf->cached_dlight = !!numdlights;

	smax = (surf->extents[0] >> 4) + 1;
	tmax = (surf->extents[1] >> 4) + 1;
	size = smax * tmax;
	stride -= smax * 4;
	blocksize = size * 3;
	lightmap = surf->samples;

	// set to full bright if no light data
	if (!cl.worldmodel->lightdata)
	{
		for (i = 0; i < blocksize; i++)
			blocklights[i] = 255 << 8;
		goto store;
	}

	// clear to no light
	memset (blocklights, 0, blocksize * sizeof(*blocklights));

	AddAllLightMaps(lightmap, surf, blocksize);

	// add all the dynamic lights
	if (numdlights)
		R_AddDynamicLights(surf, numdlights);

	// bound and shift
store:
	StoreLightMap(stride, smax, tmax, dest);
}

//Returns the proper texture for a given time and base texture
static texture_t *R_TextureAnimation(texture_t *base)
{
	int relative, count;

	if (currententity->frame)
	{
		if (base->alternate_anims)
			base = base->alternate_anims;
	}

	if (!base->anim_total)
		return base;

	relative = (int) (cl.time * 10) % base->anim_total;

	count = 0;
	while (base->anim_min > relative || base->anim_max <= relative)
	{
		base = base->anim_next;
		if (!base)
			Host_Error ("R_TextureAnimation: broken cycle");
		if (++count > 100)
			Host_Error ("R_TextureAnimation: infinite cycle");
	}

	return base;
}

static void R_RenderDynamicLightmaps(msurface_t *fa)
{
	byte *base;
	int maps, smax, tmax;
	glRect_t *theRect;
	qboolean lightstyle_modified = false;
	int numdlights;

	c_brush_polys++;

	if (!r_dynamic.value && !fa->cached_dlight)
		return;

	// check for lightmap modification
	for (maps = 0; maps < MAXLIGHTMAPS && fa->styles[maps] != 255; maps++)
	{
		if (d_lightstylevalue[fa->styles[maps]] != fa->cached_light[maps])
		{
			lightstyle_modified = true;
			break;
		}
	}

	if (r_dynamic.value)
	{
		if (fa->dlightframe == r_framecount)
			numdlights = R_BuildDlightList (fa);
		else
			numdlights = 0;

		if (numdlights == 0 && !fa->cached_dlight && !lightstyle_modified)
			return;
	}
	else
		numdlights = 0;

	lightmap_modified[fa->lightmaptexturenum] = true;
	theRect = &lightmap_rectchange[fa->lightmaptexturenum];
	if (fa->light_t < theRect->t)
	{
		if (theRect->h)
			theRect->h += theRect->t - fa->light_t;
		theRect->t = fa->light_t;
	}
	if (fa->light_s < theRect->l)
	{
		if (theRect->w)
			theRect->w += theRect->l - fa->light_s;
		theRect->l = fa->light_s;
	}
	smax = (fa->extents[0] >> 4) + 1;
	tmax = (fa->extents[1] >> 4) + 1;
	if (theRect->w + theRect->l < fa->light_s + smax)
		theRect->w = fa->light_s - theRect->l + smax;
	if (theRect->h + theRect->t < fa->light_t + tmax)
		theRect->h = fa->light_t - theRect->t + tmax;
	base = lightmaps + fa->lightmaptexturenum * BLOCK_WIDTH * BLOCK_HEIGHT * 4;
	base += (fa->light_t * BLOCK_WIDTH + fa->light_s) * 4;
	R_BuildLightMap(fa, base, BLOCK_WIDTH * 4, numdlights);
}

// uploads happen later in World_UploadLightmaps, only the CPU copy is touched here
static void R_RenderAllDynamicLightmaps(model_t *model)
{
	msurface_t *s;
	int waterline;
	int i;

	for (i = 0; i < model->numtextures; i++)
	{
		if (!model->textures[i] || (!model->textures[i]->texturechain[0] && !model->textures[i]->texturechain[1]))
			continue;

		for (waterline = 0; waterline < 2; waterline++)
		{
			for (s = model->textures[i]->texturechain[waterline]; s; s = s->texturechain)
				R_RenderDynamicLightmaps(s);
		}
	}

	for (s = drawflatchain; s; s = s->texturechain)
		R_RenderDynamicLightmaps(s);
}

// copy-pass hook, sends dirty page regions to the GPU
void World_UploadLightmaps(SDL_GPUCopyPass *copy)
{
	SDL_GPUDevice *device = GPU_GetDevice();
	SDL_GPUTextureTransferInfo transfer;
	SDL_GPUTextureRegion region;
	glRect_t *rect;
	byte *mapped;
	int i;

	if (!device || !copy)
		return;

	for (i = 0; i < MAX_LIGHTMAPS; i++)
	{
		if (!lightmap_pages[i] || !lightmap_modified[i])
			continue;

		rect = &lightmap_rectchange[i];
		if (!rect->h)
			continue;

		if (!lightmap_transfer)
		{
			SDL_GPUTransferBufferCreateInfo tci;

			memset(&tci, 0, sizeof(tci));
			tci.usage = SDL_GPU_TRANSFERBUFFERUSAGE_UPLOAD;
			tci.size = BLOCK_WIDTH * BLOCK_HEIGHT * 4;
			lightmap_transfer = SDL_CreateGPUTransferBuffer(device, &tci);
			if (!lightmap_transfer)
				return;
		}

		// full-width rows of the dirty rect, buffer cycled per page
		mapped = SDL_MapGPUTransferBuffer(device, lightmap_transfer, true);
		if (!mapped)
			return;
		memcpy(mapped, lightmaps + ((i * BLOCK_HEIGHT) + rect->t) * BLOCK_WIDTH * 4, rect->h * BLOCK_WIDTH * 4);
		SDL_UnmapGPUTransferBuffer(device, lightmap_transfer);

		memset(&transfer, 0, sizeof(transfer));
		transfer.transfer_buffer = lightmap_transfer;
		transfer.pixels_per_row = BLOCK_WIDTH;

		memset(&region, 0, sizeof(region));
		region.texture = lightmap_pages[i];
		region.y = rect->t;
		region.w = BLOCK_WIDTH;
		region.h = rect->h;
		region.d = 1;

		SDL_UploadToGPUTexture(copy, &transfer, &region, false);

		lightmap_modified[i] = false;
		rect->l = BLOCK_WIDTH;
		rect->t = BLOCK_HEIGHT;
		rect->w = 0;
		rect->h = 0;
	}
}

// ---- fullbright and luma passes ----

static void R_RenderPolyBuckets(glpoly_t **polys, unsigned int *used, int pipe, glpoly_t *(*next)(glpoly_t *))
{
	unsigned int i, j, n;
	unsigned int runfirst, runcount, firstindex;
	glpoly_t *p;

	for (j = 0; j < (MAX_GLTEXTURES+31)/32; j++)
	{
		if (!used[j])
			continue;

		for (i = 0; i < 32; i++)
		{
			if (!(used[j] & (1<<i)))
				continue;

			runcount = 0;
			runfirst = 0;
			for (p = polys[j*32+i]; p; p = next(p))
			{
				n = Fan_AllocIndices(p->firstindex, p->numverts, &firstindex);
				if (!n)
					continue;
				if (!runcount)
					runfirst = firstindex;
				runcount += n;
			}

			if (runcount)
				Scene_AddBatch(pipe, j*32+i, NULL, current_vb->buf, runfirst, runcount, current_mvp);
		}

		used[j] = 0;
	}
}

static glpoly_t *next_fb(glpoly_t *p) { return p->fb_chain; }
static glpoly_t *next_luma(glpoly_t *p) { return p->luma_chain; }

static void R_RenderFullbrights(void)
{
	if (!drawfullbrights)
		return;

	R_RenderPolyBuckets(fullbright_polys, fullbright_polys_used, SCENE_PIPE_TEX_ALPHATEST_NODEPTHWRITE, next_fb);
	drawfullbrights = false;
}

static void R_RenderLumas(void)
{
	if (!drawlumas)
		return;

	R_RenderPolyBuckets(luma_polys, luma_polys_used, SCENE_PIPE_ADD_NODEPTHWRITE, next_luma);
	drawlumas = false;
}

// ---- caustics and detail decal passes ----

static glpoly_t *caustics_polys;
static glpoly_t *detail_polys;

#define TURBSINSIZE 128
#define TURBSCALE ((float) TURBSINSIZE / (2 * M_PI))

static const byte turbsin[TURBSINSIZE] =
{
	127, 133, 139, 146, 152, 158, 164, 170, 176, 182, 187, 193, 198, 203, 208, 213,
		217, 221, 226, 229, 233, 236, 239, 242, 245, 247, 249, 251, 252, 253, 254, 254,
		255, 254, 254, 253, 252, 251, 249, 247, 245, 242, 239, 236, 233, 229, 226, 221,
		217, 213, 208, 203, 198, 193, 187, 182, 176, 170, 164, 158, 152, 146, 139, 133,
		127, 121, 115, 108, 102, 96, 90, 84, 78, 72, 67, 61, 56, 51, 46, 41,
		37, 33, 28, 25, 21, 18, 15, 12, 9, 7, 5, 3, 2, 1, 0, 0,
		0, 0, 0, 1, 2, 3, 5, 7, 9, 12, 15, 18, 21, 25, 28, 33,
		37, 41, 46, 51, 56, 61, 67, 72, 78, 84, 90, 96, 102, 108, 115, 121,
};

__inline static float SINTABLE_APPROX(float time)
{
	float sinlerpf, lerptime, lerp;
	int sinlerp1, sinlerp2;

	sinlerpf = time * TURBSCALE;
	sinlerp1 = floor(sinlerpf);
	sinlerp2 = sinlerp1 + 1;
	lerptime = sinlerpf - sinlerp1;

	lerp =	turbsin[sinlerp1 & (TURBSINSIZE - 1)] * (1 - lerptime) +
		turbsin[sinlerp2 & (TURBSINSIZE - 1)] * lerptime;
	return -8 + 16 * lerp / 255.0;
}

static void CalcCausticTexCoords(float *v, float *s, float *t)
{
	float os, ot;

	os = v[3];
	ot = v[4];

	*s = os + SINTABLE_APPROX(0.465 * (cl.time + ot));
	*s *= -3 * (0.5 / 64);

	*t = ot + SINTABLE_APPROX(0.465 * (cl.time + os));
	*t *= -3 * (0.5 / 64);
}

// emits one decal batch over the chained polys with per-frame CPU texcoords
static void EmitDecalPolys(glpoly_t *chain, int is_caustics, int texnum)
{
	glpoly_t *p;
	scene_vert_t *out;
	unsigned int *idx;
	unsigned int firstvert, firstindex, batchfirst = 0, total = 0;
	int i, j;
	float *v, s, t;

	for (p = chain; p; p = is_caustics ? p->caustics_chain : p->detail_chain)
	{
		if (p->numverts < 3)
			continue;

		out = Scene_AllocVerts(p->numverts, &firstvert);
		idx = Scene_AllocIndices((p->numverts - 2) * 3, &firstindex);
		if (!out || !idx)
			break;

		for (i = 0, v = p->verts[0]; i < p->numverts; i++, v += VERTEXSIZE)
		{
			if (is_caustics)
			{
				CalcCausticTexCoords(v, &s, &t);
			}
			else
			{
				s = v[7] * 18;
				t = v[8] * 18;
			}

			out[i].pos[0] = v[0];
			out[i].pos[1] = v[1];
			out[i].pos[2] = v[2];
			out[i].st[0] = s;
			out[i].st[1] = t;
			out[i].lm[0] = 0;
			out[i].lm[1] = 0;
			out[i].rgba[0] = 255;
			out[i].rgba[1] = 255;
			out[i].rgba[2] = 255;
			out[i].rgba[3] = 255;
		}

		for (j = 0; j < p->numverts - 2; j++)
		{
			idx[j * 3 + 0] = firstvert;
			idx[j * 3 + 1] = firstvert + j + 1;
			idx[j * 3 + 2] = firstvert + j + 2;
		}

		if (!total)
			batchfirst = firstindex;
		total += (p->numverts - 2) * 3;
	}

	if (total)
		Scene_AddBatch(SCENE_PIPE_MOD_NODEPTHWRITE, texnum, NULL,
			GPU_GetDynamicSceneVB(), batchfirst, total, current_mvp);
}

void EmitCausticsPolys(void)
{
	if (underwatertexture && caustics_polys)
		EmitDecalPolys(caustics_polys, 1, underwatertexture);
	caustics_polys = NULL;
}

static void EmitDetailPolys(void)
{
	if (detailtexture && detail_polys)
		EmitDecalPolys(detail_polys, 0, detailtexture);
	detail_polys = NULL;
}

void R_InitOtherTextures(void)
{
	static const int flags = TEX_MIPMAP | TEX_ALPHA | TEX_COMPLAIN;

	underwatertexture = R_LoadTextureImage ("textures/water_caustic", NULL, 0, 0,  flags );
	detailtexture = R_LoadTextureImage("textures/detail", NULL, 256, 256, flags);
}

// ---- texture chains ----

#define CHAIN_RESET(chain)			\
{								\
	chain = NULL;				\
	chain##_tail = &chain;		\
}

static void R_ClearTextureChains(model_t *clmodel)
{
	int i;
	texture_t *texture;

	memset (fullbright_polys_used, 0, sizeof(fullbright_polys_used));
	memset (luma_polys_used, 0, sizeof(luma_polys_used));

	for (i = 0; i < clmodel->numtextures; i++)
	{
		if ((texture = clmodel->textures[i]))
		{
			texture->texturechain[0] = NULL;
			texture->texturechain[1] = NULL;
			texture->texturechain_tail[0] = &texture->texturechain[0];
			texture->texturechain_tail[1] = &texture->texturechain[1];
		}
	}

	if (r_notexture_mip)
	{
		r_notexture_mip->texturechain[0] = NULL;
		r_notexture_mip->texturechain_tail[0] = &r_notexture_mip->texturechain[0];
		r_notexture_mip->texturechain[1] = NULL;
		r_notexture_mip->texturechain_tail[1] = &r_notexture_mip->texturechain[1];
	}

	CHAIN_RESET(skychain);
	if (clmodel == cl.worldmodel)
		CHAIN_RESET(waterchain);
	CHAIN_RESET(alphachain);
	CHAIN_RESET(drawflatchain);
}

static void DrawTextureChains(model_t *model)
{
	int waterline, i;
	msurface_t *s;
	texture_t *t;
	qboolean drawLumasGlowing, draw_fbs, draw_caustics, draw_details;
	unsigned int runfirst, runcount, firstindex, n;
	int runpage;

	drawLumasGlowing = (com_serveractive || cl.allow_lumas) && gl_fb_bmodels.value;
	draw_caustics = underwatertexture && gl_caustics.value;
	draw_details = detailtexture && gl_detail.value;

	if (!current_vb)
		return;

	for (i = 0; i < model->numtextures; i++)
	{
		if (!model->textures[i] || (!model->textures[i]->texturechain[0] && !model->textures[i]->texturechain[1]))
			continue;

		t = R_TextureAnimation (model->textures[i]);
		draw_fbs = t->isLumaTexture || gl_fb_bmodels.value;

		runcount = 0;
		runfirst = 0;
		runpage = -1;

		for (waterline = 0; waterline < 2; waterline++)
		{
			for (s = model->textures[i]->texturechain[waterline]; s; s = s->texturechain)
			{
				if (!s->polys)
					continue;

				// batch breaks on lightmap page change
				if (runcount && s->lightmaptexturenum != runpage)
				{
					Scene_AddBatch(SCENE_PIPE_WORLD, t->gl_texturenum, lightmap_pages[runpage],
						current_vb->buf, runfirst, runcount, current_mvp);
					runcount = 0;
				}

				n = Fan_AllocIndices(s->polys->firstindex, s->polys->numverts, &firstindex);
				if (n)
				{
					if (!runcount)
					{
						runfirst = firstindex;
						runpage = s->lightmaptexturenum;
					}
					runcount += n;
				}

				if (waterline && draw_caustics)
				{
					s->polys->caustics_chain = caustics_polys;
					caustics_polys = s->polys;
				}
				if (!waterline && draw_details)
				{
					s->polys->detail_chain = detail_polys;
					detail_polys = s->polys;
				}

				if (t->fb_texturenum > 0 && t->fb_texturenum < MAX_GLTEXTURES && draw_fbs)
				{
					if (t->isLumaTexture)
					{
						if ((luma_polys_used[t->fb_texturenum/32]&((1<<(t->fb_texturenum%32)))))
						{
							s->polys->luma_chain = luma_polys[t->fb_texturenum];
						}
						else
						{
							s->polys->luma_chain = 0;
						}

						luma_polys[t->fb_texturenum] = s->polys;

						if (!(luma_polys_used[t->fb_texturenum/32]&((1<<(t->fb_texturenum%32)))))
						{
							luma_polys_used[t->fb_texturenum/32] |= (1<<(t->fb_texturenum%32));
							drawlumas = true;
						}
					}
					else
					{
						if ((fullbright_polys_used[t->fb_texturenum/32])&(1<<(t->fb_texturenum%32)))
						{
							s->polys->fb_chain = fullbright_polys[t->fb_texturenum];
						}
						else
						{
							s->polys->fb_chain = 0;
						}

						fullbright_polys[t->fb_texturenum] = s->polys;

						if (!((fullbright_polys_used[t->fb_texturenum/32])&(1<<(t->fb_texturenum%32))))
						{
							fullbright_polys_used[t->fb_texturenum/32] |= (1<<(t->fb_texturenum%32));
							drawfullbrights = true;
						}
					}
				}
			}
		}

		if (runcount)
			Scene_AddBatch(SCENE_PIPE_WORLD, t->gl_texturenum, lightmap_pages[runpage],
				current_vb->buf, runfirst, runcount, current_mvp);
	}

	if (drawLumasGlowing)
	{
		R_RenderFullbrights();
		R_RenderLumas();
	}
	else
	{
		R_RenderLumas();
		R_RenderFullbrights();
	}

	EmitCausticsPolys();
	EmitDetailPolys();
}

static void R_UpdateFlatColours(model_t *model)
{
	worldvb_t *vb;
	unsigned int i;
	SDL_GPUBuffer *newbuf;

	if (!model->surface_colours_dirty || !model->vertcolours)
		return;

	vb = World_FindVB(model);
	if (!vb || !vb->verts)
		return;

	for (i = 0; i < vb->litverts; i++)
	{
		vb->verts[i].rgba[0] = (unsigned char)(bound(0, model->vertcolours[i*3+0], 1) * 255);
		vb->verts[i].rgba[1] = (unsigned char)(bound(0, model->vertcolours[i*3+1], 1) * 255);
		vb->verts[i].rgba[2] = (unsigned char)(bound(0, model->vertcolours[i*3+2], 1) * 255);
		vb->verts[i].rgba[3] = 255;
	}

	// rare, a full rebuild keeps the buffer static
	newbuf = GPU_CreateStaticVertexBuffer(vb->verts, vb->numverts);
	if (newbuf)
	{
		GPU_ReleaseBuffer(vb->buf);
		vb->buf = newbuf;
	}

	model->surface_colours_dirty = 0;
}

static void R_DrawFlat(model_t *model)
{
	msurface_t *s;
	unsigned int runfirst, runcount, firstindex, n;
	int runpage;

	if (r_drawflat_enable.value == 0)
		return;

	if (!current_vb)
		return;

	runcount = 0;
	runfirst = 0;
	runpage = -1;

	for (s = drawflatchain; s; s = s->texturechain)
	{
		if (!s->polys)
			continue;

		if (runcount && s->lightmaptexturenum != runpage)
		{
			Scene_AddBatch(SCENE_PIPE_WORLD, GPU_Texture_White(), lightmap_pages[runpage],
				current_vb->buf, runfirst, runcount, current_mvp);
			runcount = 0;
		}

		n = Fan_AllocIndices(s->polys->firstindex, s->polys->numverts, &firstindex);
		if (n)
		{
			if (!runcount)
			{
				runfirst = firstindex;
				runpage = s->lightmaptexturenum;
			}
			runcount += n;
		}
	}

	if (runcount)
		Scene_AddBatch(SCENE_PIPE_WORLD, GPU_Texture_White(), lightmap_pages[runpage],
			current_vb->buf, runfirst, runcount, current_mvp);
}

//draws transparent textures for HL world and nonworld models
static void R_DrawAlphaChain(void)
{
	msurface_t *s;
	texture_t *t;
	unsigned int firstindex, n;

	if (!alphachain)
		return;

	// back to front, one batch per surface preserves the order
	for (s = alphachain; s; s = s->texturechain)
	{
		t = s->texinfo->texture;
		R_RenderDynamicLightmaps (s);

		if (!s->polys || !current_vb)
			continue;

		n = Fan_AllocIndices(s->polys->firstindex, s->polys->numverts, &firstindex);
		if (n)
			Scene_AddBatch(SCENE_PIPE_WORLD_ALPHATEST, t->gl_texturenum, lightmap_pages[s->lightmaptexturenum],
				current_vb->buf, firstindex, n, current_mvp);
	}

	alphachain = NULL;
}

// ---- water ----

//Emits a water batch from the pre-built fastpoly range, shader does the warp
void EmitWaterPolys(model_t *model, msurface_t *fa)
{
	worldvb_t *vb;
	unsigned int firstindex, n;
	scene_batch_t *b;

	if (r_fastturb.value)
	{
		EmitFastPolyColoured(fa, (byte *) &fa->texinfo->texture->colour);
		return;
	}

	vb = World_FindVB(model);
	if (!vb || !fa->fastpolys)
		return;

	n = Fan_AllocIndices(fa->fastpolyfirstindex, fa->numedges, &firstindex);
	if (!n)
		return;

	b = Scene_AddBatch(SCENE_PIPE_WATER, fa->texinfo->texture->gl_texturenum, NULL, vb->buf, firstindex, n, current_mvp);
	if (b)
		b->params[0] = cl.time * (20.0 / 64.0);
}

void R_DrawWaterSurfaces(void)
{
	msurface_t *s;

	if (!waterchain)
		return;

	// always opaque, r_wateralpha is not honoured by the SDL_GPU path
	current_mvp = r_viewproj;

	for (s = waterchain; s; s = s->texturechain)
		EmitWaterPolys(cl.worldmodel, s);

	waterchain = NULL;
	waterchain_tail = &waterchain;
}

// ---- surface subdivision (load time, called from gl_model.c) ----

static void BoundPoly(int numverts, float *verts, vec3_t mins, vec3_t maxs)
{
	int i, j;
	float *v;

	mins[0] = mins[1] = mins[2] = 9999;
	maxs[0] = maxs[1] = maxs[2] = -9999;
	v = verts;
	for (i=0 ; i<numverts ; i++)
	{
		for (j = 0; j < 3; j++, v++)
		{
			if (*v < mins[j])
				mins[j] = *v;
			if (*v > maxs[j])
				maxs[j] = *v;
		}
	}
}

static void SubdividePolygon(msurface_t *warpface, int numverts, float *verts)
{
	int i, j, k, f, b;
	vec3_t mins, maxs, front[64], back[64];
	float m, *v, dist[64], frac, s, t;
	struct glwarppoly *poly;
	float subdivide_size;

	if (numverts > 60)
		Sys_Error ("numverts = %i", numverts);

	subdivide_size = max(1, gl_subdivide_size.value);
	BoundPoly (numverts, verts, mins, maxs);

	for (i = 0; i < 3; i++)
	{
		m = (mins[i] + maxs[i]) * 0.5;
		m = subdivide_size * floor (m / subdivide_size + 0.5);
		if (maxs[i] - m < 8)
			continue;
		if (m - mins[i] < 8)
			continue;

		// cut it
		v = verts + i;
		for (j = 0; j < numverts; j++, v += 3)
			dist[j] = *v - m;

		// wrap cases
		dist[j] = dist[0];
		v -= i;
		VectorCopy (verts, v);

		f = b = 0;
		v = verts;
		for (j = 0; j < numverts; j++, v += 3)
		{
			if (dist[j] >= 0)
			{
				VectorCopy (v, front[f]);
				f++;
			}
			if (dist[j] <= 0)
			{
				VectorCopy (v, back[b]);
				b++;
			}
			if (dist[j] == 0 || dist[j + 1] == 0)
				continue;
			if ( (dist[j] > 0) != (dist[j + 1] > 0) )
			{
				// clip point
				frac = dist[j] / (dist[j] - dist[j + 1]);
				for (k = 0; k < 3; k++)
					front[f][k] = back[b][k] = v[k] + frac * (v[3 + k] - v[k]);
				f++;
				b++;
			}
		}
		SubdividePolygon(warpface, f, front[0]);
		SubdividePolygon(warpface, b, back[0]);
		return;
	}

	poly = malloc(sizeof(struct glwarppoly) + (numverts - 4) * WARPVERTEXSIZE * sizeof(float));
	if (poly == 0)
		Sys_Error("SubdividePolygon: Out of memory\n");
	poly->next = warpface->warppolys;
	warpface->warppolys = poly;
	poly->numverts = numverts;
	for (i = 0; i < numverts; i++, verts += 3)
	{
		VectorCopy (verts, poly->verts[i]);
		s = DotProduct (verts, warpface->texinfo->vecs[0]);
		t = DotProduct (verts, warpface->texinfo->vecs[1]);
		poly->verts[i][3] = s;
		poly->verts[i][4] = t;
	}
}

// Breaks a polygon up along axial 64 unit boundaries so that turbulent and sky warps can be done reasonably.
void R_SubdivideSurface(model_t *model, msurface_t *fa)
{
	vec3_t verts[64];
	int i, lindex;
	float *vec;
	float s;
	float t;

	/* Build simple verts for fastturb/fastsky */
	fa->fastpolys = malloc(fa->numedges * 3 * sizeof(*fa->fastpolys));
	if (fa->fastpolys == 0)
		Sys_Error("R_SubdivideSurface: Out of memory\n");

	fa->shadertexcoords = malloc(fa->numedges * 3 * sizeof(*fa->shadertexcoords));
	if (fa->shadertexcoords == 0)
		Sys_Error("R_SubdivideSurface: Out of memory\n");

	// convert edges back to a normal polygon
	for (i = 0; i < fa->numedges && i < 64; i++)
	{
		lindex = model->surfedges[fa->firstedge + i];

		if (lindex > 0)
			vec = model->vertexes[model->edges[lindex].v[0]].position;
		else
			vec = model->vertexes[model->edges[-lindex].v[1]].position;
		VectorCopy (vec, verts[i]);

		fa->fastpolys[i*3+0] = vec[0];
		fa->fastpolys[i*3+1] = vec[1];
		fa->fastpolys[i*3+2] = vec[2];

		s = DotProduct(vec, fa->texinfo->vecs[0]);
		t = DotProduct(vec, fa->texinfo->vecs[1]);
		fa->shadertexcoords[i*2+0] = s / 64.0;
		fa->shadertexcoords[i*2+1] = t / 64.0;
	}

	SubdividePolygon(fa, fa->numedges, verts[0]);
}

// ---- sky ----

static char sky_initialised;
static int solidskytexture, alphaskytexture;

static char *skybox_ext[6] = {"rt", "bk", "lf", "ft", "up", "dn"};

int R_SetSky(char *skyname)
{
	int i, error = 0;
	byte *data[6] = {NULL, NULL, NULL, NULL, NULL, NULL};
	unsigned int imagewidth, imageheight;

	for (i = 0; i < 6; i++)
	{
		if (
		    !(data[i] = R_LoadImagePixels (va("env/%s%s", skyname, skybox_ext[i]), 0, 0, &imagewidth, &imageheight, 0)) &&
		    !(data[i] = R_LoadImagePixels (va("gfx/env/%s%s", skyname, skybox_ext[i]), 0, 0, &imagewidth, &imageheight, 0)) &&
		    !(data[i] = R_LoadImagePixels (va("env/%s_%s", skyname, skybox_ext[i]), 0, 0, &imagewidth, &imageheight, 0)) &&
		    !(data[i] = R_LoadImagePixels (va("gfx/env/%s_%s", skyname, skybox_ext[i]), 0, 0, &imagewidth, &imageheight, 0))
		   )
		{
			Com_Printf ("Couldn't load skybox \"%s\"\n", skyname);
			error = 1;

			goto cleanup;
		}
	}

	// the GL renderer reserved these at startup, here they are claimed on first use
	if (!skyboxtextures)
	{
		skyboxtextures = texture_extension_number;
		texture_extension_number += 6;
	}

	for (i = 0; i < 6; i++)
	{
		R_Bind (skyboxtextures + i);
		R_Upload32 ((unsigned int *) data[i], imagewidth, imageheight, TEX_NOCOMPRESS);
		GPU_Texture_SetPrefs(skyboxtextures + i, GPU_TEXPREF_LINEAR | GPU_TEXPREF_CLAMP);
	}

	r_skyboxloaded = true;

cleanup:
	for (i = 0; i < 6; i++)
	{
		if (data[i])
			free(data[i]);
		else
			break;
	}

	return error;
}

static void R_SkyNameChanged(char *skyname)
{
	if (!sky_initialised)
		return;

	if (!skyname[0])
	{
		r_skyboxloaded = false;
		return;
	}

	R_SetSky(skyname);
}

static qboolean OnChange_r_skyname(cvar_t *v, char *skyname)
{
	if (!sky_initialised)
		return false;

	if (!skyname[0])
	{
		r_skyboxloaded = false;
		return false;
	}

	return R_SetSky(skyname);
}

cvar_t r_skyname = { "r_skyname", "purple_chaos", 0, OnChange_r_skyname };

void R_LoadSky_f(void)
{
	switch (Cmd_Argc())
	{
		case 1:
			if (r_skyboxloaded)
				Com_Printf("Current skybox is \"%s\"\n", r_skyname.string);
			else
				Com_Printf("No skybox has been set\n");
			break;
		case 2:
			if (!Q_strcasecmp(Cmd_Argv(1), "none"))
				Cvar_Set(&r_skyname, "");
			else
				Cvar_Set(&r_skyname, Cmd_Argv(1));
			break;
		default:
			Com_Printf("Usage: %s <skybox>\n", Cmd_Argv(0));
	}
}

//A sky texture is 256 * 128, with the right side being a masked overlay
void R_InitSky(void *texturedata)
{
	int i, j, p, r, g, b;
	byte *src;
	unsigned trans[128 * 128], transpix, *rgba;
	SDL_GPUTexture *tex;

	src = texturedata;

	if (!solidskytexture)
		solidskytexture = texture_extension_number++;
	if (!alphaskytexture)
		alphaskytexture = texture_extension_number++;

	// make an average value for the back to avoid a fringe on the top level
	r = g = b = 0;
	for (i = 0; i < 128; i++)
	{
		for (j = 0; j < 128; j++)
		{
			p = src[i * 256 + j + 128];
			rgba = &d_8to24table[p];
			trans[(i * 128) + j] = *rgba;
			r += ((byte *) rgba)[0];
			g += ((byte *) rgba)[1];
			b += ((byte *) rgba)[2];
		}
	}

	((byte *) &transpix)[0] = r / (128 * 128);
	((byte *) &transpix)[1] = g / (128 * 128);
	((byte *) &transpix)[2] = b / (128 * 128);
	((byte *) &transpix)[3] = 0;

	tex = GPU_CreateTextureRGBA((const unsigned char *) trans, 128, 128, 0);
	GPU_Texture_Set(solidskytexture, tex, 128, 128, GPU_TEXPREF_LINEAR);

	for (i = 0; i < 128; i++)
	{
		for (j = 0; j < 128; j++)
		{
			p = src[i * 256 + j];
			trans[(i * 128) + j] = p ? d_8to24table[p] : transpix;
		}
	}

	tex = GPU_CreateTextureRGBA((const unsigned char *) trans, 128, 128, 0);
	GPU_Texture_Set(alphaskytexture, tex, 128, 128, GPU_TEXPREF_LINEAR);

	sky_initialised = 1;

	R_SkyNameChanged(r_skyname.string);
}

void R_DrawSkyChain(void)
{
	msurface_t *fa;
	byte *col;
	scene_batch_t *b;
	SDL_GPUTexture *alphatex;
	unsigned int firstindex, n;
	float speedscale, speedscale2;

	if (!skychain)
		return;

	if (r_fastsky.value || cl.worldmodel->bspversion == HL_BSPVERSION)
	{
		col = StringToRGB(r_skycolor.string);

		for (fa = skychain; fa; fa = fa->texturechain)
			EmitFastPolyColoured(fa, col);
	}
	else if (current_vb)
	{
		speedscale = cl.time * 8;
		speedscale -= (int) speedscale & ~127;
		speedscale2 = cl.time * 16;
		speedscale2 -= (int) speedscale2 & ~127;

		alphatex = GPU_Texture_Lookup(alphaskytexture, NULL);

		for (fa = skychain; fa; fa = fa->texturechain)
		{
			if (!fa->fastpolys)
				continue;

			n = Fan_AllocIndices(fa->fastpolyfirstindex, fa->numedges, &firstindex);
			if (!n)
				continue;

			b = Scene_AddBatch(SCENE_PIPE_SKY, solidskytexture, alphatex, current_vb->buf, firstindex, n, current_mvp);
			if (b)
			{
				b->params[0] = r_origin[0];
				b->params[1] = r_origin[1];
				b->params[2] = r_origin[2];
				b->params[3] = speedscale;
				b->params[4] = speedscale2;
			}
		}
	}

	skychain = NULL;
	skychain_tail = &skychain;
}

// ---- skybox ----

static vec3_t skyclip[6] =
{
	{1,1,0},
	{1,-1,0},
	{0,-1,1},
	{0,1,1},
	{1,0,1},
	{-1,0,1}
};

// 1 = s, 2 = t, 3 = 2048
static int st_to_vec[6][3] =
{
	{3,-1,2},
	{-3,1,2},

	{1,3,2},
	{-1,-3,2},

	{-2,-1,3},		// 0 degrees yaw, look straight up
	{2,-1,-3}		// look straight down
};

// s = [0]/[2], t = [1]/[2]
static int vec_to_st[6][3] =
{
	{-2,3,1},
	{2,3,-1},

	{1,3,2},
	{-1,3,-2},

	{-2,-1,3},
	{-2,1,-3}
};

static float skymins[2][6], skymaxs[2][6];

static void DrawSkyPolygon(int nump, vec3_t vecs)
{
	int i,j, axis;
	vec3_t v, av;
	float s, t, dv, *vp;

	// decide which face it maps to
	VectorClear (v);
	for (i = 0, vp = vecs; i < nump; i++, vp += 3)
		VectorAdd (vp, v, v);

	av[0] = fabs(v[0]);
	av[1] = fabs(v[1]);
	av[2] = fabs(v[2]);
	if (av[0] > av[1] && av[0] > av[2])
		axis = (v[0] < 0) ? 1 : 0;
	else if (av[1] > av[2] && av[1] > av[0])
		axis = (v[1] < 0) ? 3 : 2;
	else
		axis = (v[2] < 0) ? 5 : 4;

	// project new texture coords
	for (i = 0; i < nump; i++, vecs += 3)
	{
		j = vec_to_st[axis][2];
		dv = (j > 0) ? vecs[j - 1] : -vecs[-j - 1];

		j = vec_to_st[axis][0];
		s = (j < 0) ? -vecs[-j -1] / dv : vecs[j-1] / dv;

		j = vec_to_st[axis][1];
		t = (j < 0) ? -vecs[-j -1] / dv : vecs[j-1] / dv;

		if (s < skymins[0][axis])
			skymins[0][axis] = s;
		if (t < skymins[1][axis])
			skymins[1][axis] = t;
		if (s > skymaxs[0][axis])
			skymaxs[0][axis] = s;
		if (t > skymaxs[1][axis])
			skymaxs[1][axis] = t;
	}
}

#define	MAX_CLIP_VERTS	64
static void ClipSkyPolygon(int nump, vec3_t vecs, int stage)
{
	float *norm, *v, d, e, dists[MAX_CLIP_VERTS];
	qboolean front, back;
	int sides[MAX_CLIP_VERTS], newc[2], i, j;
	vec3_t newv[2][MAX_CLIP_VERTS];

	if (nump > MAX_CLIP_VERTS - 2)
		Sys_Error ("ClipSkyPolygon: nump > MAX_CLIP_VERTS - 2");
	if (stage == 6)
	{
		// fully clipped, so draw it
		DrawSkyPolygon (nump, vecs);
		return;
	}

	front = back = false;
	norm = skyclip[stage];
	for (i = 0, v = vecs; i < nump; i++, v += 3)
	{
		d = DotProduct (v, norm);
		if (d > ON_EPSILON)
		{
			front = true;
			sides[i] = SIDE_FRONT;
		}
		else if (d < -ON_EPSILON)
		{
			back = true;
			sides[i] = SIDE_BACK;
		}
		else
		{
			sides[i] = SIDE_ON;
		}
		dists[i] = d;
	}

	if (!front || !back)
	{
		// not clipped
		ClipSkyPolygon (nump, vecs, stage + 1);
		return;
	}

	// clip it
	sides[i] = sides[0];
	dists[i] = dists[0];
	VectorCopy (vecs, (vecs + (i * 3)));
	newc[0] = newc[1] = 0;

	for (i = 0, v = vecs; i < nump; i++, v += 3)
	{
		switch (sides[i])
		{
		case SIDE_FRONT:
			VectorCopy (v, newv[0][newc[0]]);
			newc[0]++;
			break;
		case SIDE_BACK:
			VectorCopy (v, newv[1][newc[1]]);
			newc[1]++;
			break;
		case SIDE_ON:
			VectorCopy (v, newv[0][newc[0]]);
			newc[0]++;
			VectorCopy (v, newv[1][newc[1]]);
			newc[1]++;
			break;
		}

		if (sides[i] == SIDE_ON || sides[i + 1] == SIDE_ON || sides[i + 1] == sides[i])
			continue;

		d = dists[i] / (dists[i] - dists[i+1]);
		for (j = 0; j < 3; j++)
		{
			e = v[j] + d * (v[j + 3] - v[j]);
			newv[0][newc[0]][j] = e;
			newv[1][newc[1]][j] = e;
		}
		newc[0]++;
		newc[1]++;
	}

	// continue
	ClipSkyPolygon (newc[0], newv[0][0], stage + 1);
	ClipSkyPolygon (newc[1], newv[1][0], stage + 1);
}

static void R_AddSkyBoxSurface(msurface_t *fa)
{
	int i;
	vec3_t verts[MAX_CLIP_VERTS];
	struct glwarppoly *p;

	// calculate vertex values for sky box
	for (p = fa->warppolys; p; p = p->next)
	{
		for (i = 0; i < p->numverts; i++)
			VectorSubtract (p->verts[i], r_origin, verts[i]);
		ClipSkyPolygon (p->numverts, verts[0], 0);
	}
}

static void R_ClearSkyBox(void)
{
	int i;

	for (i = 0; i < 6; i++)
	{
		skymins[0][i] = skymins[1][i] = 9999;
		skymaxs[0][i] = skymaxs[1][i] = -9999;
	}
}

static void MakeSkyVec(float s, float t, int axis, scene_vert_t *out)
{
	vec3_t v, b;
	int j, k, farclip;

	farclip = max((int) r_farclip.value, 4096);
	b[0] = s * (farclip >> 1);
	b[1] = t * (farclip >> 1);
	b[2] = (farclip >> 1);

	for (j = 0; j < 3; j++)
	{
		k = st_to_vec[axis][j];
		v[j] = (k < 0) ? -b[-k - 1] : b[k - 1];
		v[j] += r_origin[j];
	}

	// avoid bilerp seam
	s = (s + 1) * 0.5;
	t = (t + 1) * 0.5;

	s = bound(1.0 / 512, s, 511.0 / 512);
	t = bound(1.0 / 512, t, 511.0 / 512);

	t = 1.0 - t;

	out->pos[0] = v[0];
	out->pos[1] = v[1];
	out->pos[2] = v[2];
	out->st[0] = s;
	out->st[1] = t;
	out->lm[0] = out->lm[1] = 0;
	out->rgba[0] = out->rgba[1] = out->rgba[2] = out->rgba[3] = 255;
}

static int skytexorder[6] = {0, 2, 1, 3, 4, 5};
void R_DrawSkyBox(void)
{
	int i;
	msurface_t *fa;
	scene_vert_t *v;
	unsigned int *idx;
	unsigned int firstvert, firstindex, n;

	if (!skychain)
		return;

	R_ClearSkyBox();
	for (fa = skychain; fa; fa = fa->texturechain)
		R_AddSkyBoxSurface (fa);

	for (i = 0; i < 6; i++)
	{
		if (skymins[0][i] >= skymaxs[0][i] || skymins[1][i] >= skymaxs[1][i])
			continue;

		v = Scene_AllocVerts(4, &firstvert);
		if (!v)
			break;
		idx = Scene_AllocIndices(6, &firstindex);
		if (!idx)
			break;

		MakeSkyVec (skymins[0][i], skymins[1][i], i, &v[0]);
		MakeSkyVec (skymins[0][i], skymaxs[1][i], i, &v[1]);
		MakeSkyVec (skymaxs[0][i], skymaxs[1][i], i, &v[2]);
		MakeSkyVec (skymaxs[0][i], skymins[1][i], i, &v[3]);

		idx[0] = firstvert;
		idx[1] = firstvert + 1;
		idx[2] = firstvert + 2;
		idx[3] = firstvert;
		idx[4] = firstvert + 2;
		idx[5] = firstvert + 3;

		Scene_AddBatch(SCENE_PIPE_TEX, skyboxtextures + skytexorder[i], NULL, GPU_GetDynamicSceneVB(), firstindex, 6, current_mvp);
	}

	// z fill so world geometry occludes the sky quads
	if (current_vb)
	{
		for (fa = skychain; fa; fa = fa->texturechain)
		{
			if (!fa->fastpolys)
				continue;
			n = Fan_AllocIndices(fa->fastpolyfirstindex, fa->numedges, &firstindex);
			if (n)
				Scene_AddBatch(SCENE_PIPE_DEPTHFILL, GPU_Texture_White(), NULL, current_vb->buf, firstindex, n, current_mvp);
		}
	}

	skychain = NULL;
	skychain_tail = &skychain;
}

// ---- brush models and world ----

void R_DrawBrushModel(entity_t *e)
{
	int i, k;
	unsigned int li;
	unsigned int lj;
	vec3_t mins, maxs;
	msurface_t *psurf;
	float dot;
	mplane_t *pplane;
	model_t *clmodel;
	qboolean rotated;
	unsigned char flags;
	float entmatrix[16];

	currententity = e;
	currenttexture = -1;

	clmodel = e->model;

	if (e->angles[0] || e->angles[1] || e->angles[2])
	{
		rotated = true;
		if (R_CullSphere (e->origin, clmodel->radius))
			return;
	}
	else
	{
		rotated = false;
		VectorAdd (e->origin, clmodel->mins, mins);
		VectorAdd (e->origin, clmodel->maxs, maxs);

		if (R_CullBox (mins, maxs))
			return;
	}

	VectorSubtract (r_refdef.vieworg, e->origin, modelorg);
	if (rotated)
	{
		vec3_t	temp;
		vec3_t	forward, right, up;

		VectorCopy (modelorg, temp);
		AngleVectors (e->angles, forward, right, up);
		modelorg[0] = DotProduct (temp, forward);
		modelorg[1] = -DotProduct (temp, right);
		modelorg[2] = DotProduct (temp, up);
	}

	psurf = &clmodel->surfaces[clmodel->firstmodelsurface];

	// calculate dynamic lighting for bmodel if it's not an instanced model
	if (clmodel->firstmodelsurface)
	{
		for(li=0;li<MAX_DLIGHTS/32;li++)
		{
			if (cl_dlight_active[li])
			{
				for(lj=0;lj<32;lj++)
				{
					if ((cl_dlight_active[li]&(1<<lj)) && li*32+lj < MAX_DLIGHTS)
					{
						k = li*32 + lj;

						/* This will fail for k >= 32 */
						if (!gl_flashblend.value || (cl_dlights[k].bubble && gl_flashblend.value != 2))
							R_MarkLights(clmodel, &cl_dlights[k], 1 << k, clmodel->hulls[0].firstclipnode);
					}
				}
			}
		}
	}

	// entity transform, same order as the glTranslatef/glRotatef sequence
	Mat4_Identity(entmatrix);
	Mat4_Translate(entmatrix, e->origin[0], e->origin[1], e->origin[2]);
	Mat4_RotateZ(entmatrix, e->angles[1]);
	Mat4_RotateY(entmatrix, e->angles[0]);
	Mat4_RotateX(entmatrix, e->angles[2]);
	Mat4_Multiply(r_viewproj, entmatrix, brush_mvp);

	current_mvp = brush_mvp;
	current_vb = World_FindVB(clmodel);

	R_ClearTextureChains(clmodel);

	for (i = 0; i < clmodel->nummodelsurfaces; i++, psurf++)
	{
		// find which side of the node we are on
		pplane = psurf->plane;
		dot = PlaneDiff(modelorg, pplane);

		flags = clmodel->surfflags[clmodel->firstmodelsurface + i];

		//draw the water surfaces now, and setup sky/normal chains
		if (((flags & SURF_PLANEBACK) && (dot < -BACKFACE_EPSILON))
		 || (!(flags & SURF_PLANEBACK) && (dot > BACKFACE_EPSILON)))
		{
			if (flags & SURF_DRAWSKY)
			{
				CHAIN_SURF_B2F(psurf, skychain);
			}
			else if (flags & SURF_DRAWTURB)
			{
				EmitWaterPolys(clmodel, psurf);
			}
			else if (flags & SURF_DRAWALPHA)
			{
				CHAIN_SURF_B2F(psurf, alphachain);
			}
			else if (r_drawflat_enable.value == 1 && psurf->is_drawflat)
			{
				CHAIN_SURF_B2F(psurf, drawflatchain);
			}
			else
			{
				int underwater = (flags & SURF_UNDERWATER) ? 1 : 0;
				CHAIN_SURF_B2F(psurf, psurf->texinfo->texture->texturechain[underwater]);
			}
		}
	}

	//draw the textures chains for the model
	R_RenderAllDynamicLightmaps(clmodel);
	R_UpdateFlatColours(clmodel);
	DrawTextureChains(clmodel);
	R_DrawFlat(clmodel);
	R_DrawSkyChain();
	R_DrawAlphaChain ();

	current_mvp = r_viewproj;
	current_vb = cl.worldmodel ? World_FindVB(cl.worldmodel) : NULL;
}

static void R_RecursiveWorldNode(model_t *model, unsigned int nodenum, int clipflags)
{
	mnode_t *node;
	int c, side, clipped, underwater;
	mplane_t *plane, *clipplane;
	msurface_t *surf;
	unsigned int surfnum;
	unsigned int *mark;
	unsigned char flags;
	mleaf_t *pleaf;
	float dot;
	unsigned int leafnum;
	int isleaf;

	if (nodenum >= model->numnodes)
	{
		isleaf = 1;
		leafnum = nodenum - model->numnodes;

		if ((model->leafsolid[leafnum/32] & (1<<(leafnum%32))))
			return; // solid

		node = (mnode_t *)(model->leafs + leafnum);
	}
	else
	{
		isleaf = 0;
		node = model->nodes + nodenum;
	}

	if (node->visframe != r_visframecount)
		return;

	if (clipflags)
	{
		for (c = 0, clipplane = frustum; c < 4; c++, clipplane++)
		{
			if (!(clipflags & (1 << c)))
				continue;	// don't need to clip against it

			clipped = BOX_ON_PLANE_SIDE (node->minmaxs, node->minmaxs + 3, clipplane);
			if (clipped == 2)
				return;
			else if (clipped == 1)
				clipflags &= ~(1<<c);	// node is entirely on screen
		}
	}

	// if a leaf node, draw stuff
	if (isleaf)
	{
		pleaf = (mleaf_t *)node;

		mark = model->marksurfaces + pleaf->firstmarksurfacenum;
		c = pleaf->nummarksurfaces;

		if (c)
		{
			do
			{
				surfnum = *mark;
				cl.worldmodel->surfvisible[surfnum/32] |= (1<<(surfnum%32));
				mark++;
			} while(--c);
		}

		// deal with model fragments in this leaf
		if (pleaf->efrags)
			R_StoreEfrags(&pleaf->efrags);
	}
	else
	{
		// node is just a decision point, so go down the apropriate sides

		// find which side of the node we are on
		plane = model->planes + node->planenum;

		dot = PlaneDiff(modelorg, plane);
		side = (dot >= 0) ? 0 : 1;

		// recurse down the children, front side first
		R_RecursiveWorldNode(model, node->childrennum[side], clipflags);

		// draw stuff
		c = node->numsurfaces;

		if (c)
		{
			surf = cl.worldmodel->surfaces + node->firstsurface;
			surfnum = node->firstsurface;

			if (dot < -BACKFACE_EPSILON)
				side = SURF_PLANEBACK;
			else if (dot > BACKFACE_EPSILON)
				side = 0;

			for ( ; c; c--, surf++, surfnum++)
			{
				if (!(cl.worldmodel->surfvisible[surfnum/32]&(1<<(surfnum%32))))
					continue;

				flags = cl.worldmodel->surfflags[surfnum];

				if ((dot < 0) ^ !!(flags & SURF_PLANEBACK))
					continue;		// wrong side

				// add surf to the right chain
				if (flags & SURF_DRAWSKY)
				{
					CHAIN_SURF_F2B(surf, skychain_tail);
				}
				else if (flags & SURF_DRAWTURB)
				{
					CHAIN_SURF_F2B(surf, waterchain_tail);
				}
				else if (flags & SURF_DRAWALPHA)
				{
					CHAIN_SURF_B2F(surf, alphachain);
				}
				else if (r_drawflat_enable.value == 1 && surf->is_drawflat)
				{
					CHAIN_SURF_F2B(surf, drawflatchain_tail);
				}
				else
				{
					underwater = (flags & SURF_UNDERWATER) ? 1 : 0;
					CHAIN_SURF_F2B(surf, surf->texinfo->texture->texturechain_tail[underwater]);
				}
			}
		}

		// recurse down the back side
		R_RecursiveWorldNode(model, node->childrennum[!side], clipflags);
	}
}

void R_DrawWorld(void)
{
	static entity_t ent;

	memset (&ent, 0, sizeof(ent));
	ent.model = cl.worldmodel;

	R_ClearTextureChains(cl.worldmodel);

	VectorCopy (r_refdef.vieworg, modelorg);

	currententity = &ent;
	currenttexture = -1;

	current_mvp = r_viewproj;
	current_vb = World_FindVB(cl.worldmodel);

	//set up texture chains for the world
	memset(cl.worldmodel->surfvisible, 0, ((cl.worldmodel->numsurfaces+31)/32)*sizeof(*cl.worldmodel->surfvisible));
	R_RecursiveWorldNode(cl.worldmodel, 0, 15);

	//draw the world sky
	if (r_skyboxloaded)
		R_DrawSkyBox ();
	else
		R_DrawSkyChain ();

	R_DrawEntitiesOnList (&cl_firstpassents);

	//draw the world
	R_RenderAllDynamicLightmaps(cl.worldmodel);
	R_UpdateFlatColours(cl.worldmodel);
	DrawTextureChains(cl.worldmodel);
	R_DrawFlat(cl.worldmodel);

	//draw the world alpha textures
	R_DrawAlphaChain ();
}

void R_MarkLeaves(void)
{
	byte *vis;
	mnode_t *node;
	int i;
	byte solid[MAX_MAP_LEAFS/8];

	if (!r_novis.value && r_oldviewleaf == r_viewleaf
		&& r_oldviewleaf2 == r_viewleaf2)	// watervis hack
		return;

	r_visframecount++;
	r_oldviewleaf = r_viewleaf;

	if (r_novis.value)
	{
		vis = solid;
		memset (solid, 0xff, (cl.worldmodel->numleafs + 7) >> 3);
	}
	else
	{
		vis = Mod_LeafPVS (r_viewleaf, cl.worldmodel);

		if (r_viewleaf2)
		{
			int			i, count;
			unsigned	*src, *dest;

			// merge visibility data for two leafs
			count = (cl.worldmodel->numleafs + 7) >> 3;
			memcpy (solid, vis, count);
			src = (unsigned *) Mod_LeafPVS (r_viewleaf2, cl.worldmodel);
			dest = (unsigned *) solid;
			count = (count + 3) >> 2;
			for (i = 0; i < count; i++)
				*dest++ |= *src++;
			vis = solid;
		}
	}

	for (i = 0; i < cl.worldmodel->numleafs; i++)
	{
		if (vis[i >> 3] & (1 << (i & 7)))
		{
			node = (mnode_t *)&cl.worldmodel->leafs[i + 1];
			while(1)
			{
				if (node->visframe == r_visframecount)
					break;
				node->visframe = r_visframecount;
				if (node->parentnum == 0xffffffff)
					break;
				node = NODENUM_TO_NODE(cl.worldmodel, node->parentnum);
			}
		}
	}
}

// ---- load-time lightmap and geometry building ----

// returns a texture number and the position inside it
static int AllocBlock(int w, int h, int *x, int *y)
{
	int i, j, best, best2, texnum;

	if (w < 1 || w > BLOCK_WIDTH || h < 1 || h > BLOCK_HEIGHT)
		Sys_Error ("AllocBlock: Bad dimensions");

	for (texnum = 0; texnum < MAX_LIGHTMAPS; texnum++)
	{
		best = BLOCK_HEIGHT + 1;

		for (i = 0; i < BLOCK_WIDTH - w; i++)
		{
			best2 = 0;

			for (j = i; j < i + w; j++)
			{
				if (allocated[texnum][j] >= best)
				{
					i = j;
					break;
				}
				if (allocated[texnum][j] > best2)
					best2 = allocated[texnum][j];
			}
			if (j == i + w)
			{
				// this is a valid spot
				*x = i;
				*y = best = best2;
			}
		}

		if (best + h > BLOCK_HEIGHT)
			continue;

		for (i = 0; i < w; i++)
			allocated[texnum][*x + i] = best + h;

		return texnum;
	}

	Sys_Error ("AllocBlock: full");
	return 0;
}

static void BuildSurfaceDisplayList(model_t *model, msurface_t *fa)
{
	int i, lindex, lnumverts;
	medge_t *pedges, *r_pedge;
	float *vec, s, t;
	glpoly_t *poly;
	mvertex_t *vertbase;

	vertbase = model->vertexes;

	// reconstruct the polygon
	pedges = model->edges;
	lnumverts = fa->numedges;

	// draw texture
	poly = malloc(sizeof(glpoly_t) + (lnumverts - 4) * VERTEXSIZE*sizeof(float));
	if (poly == 0)
		Sys_Error("BuildSurfaceDisplayList: Out of memory\n");
	fa->polys = poly;
	poly->numverts = lnumverts;

	for (i = 0; i < lnumverts; i++)
	{
		lindex = model->surfedges[fa->firstedge + i];

		if (lindex > 0)
		{
			r_pedge = &pedges[lindex];
			vec = vertbase[r_pedge->v[0]].position;
		}
		else
		{
			r_pedge = &pedges[-lindex];
			vec = vertbase[r_pedge->v[1]].position;
		}
		s = DotProduct (vec, fa->texinfo->vecs[0]) + fa->texinfo->vecs[0][3];
		s /= fa->texinfo->texture->width;

		t = DotProduct (vec, fa->texinfo->vecs[1]) + fa->texinfo->vecs[1][3];
		t /= fa->texinfo->texture->height;

		VectorCopy (vec, poly->verts[i]);
		poly->verts[i][3] = s;
		poly->verts[i][4] = t;

		// lightmap texture coordinates
		s = DotProduct (vec, fa->texinfo->vecs[0]) + fa->texinfo->vecs[0][3];
		s -= fa->texturemins[0];
		s += fa->light_s * 16;
		s += 8;
		s /= BLOCK_WIDTH*16;

		t = DotProduct (vec, fa->texinfo->vecs[1]) + fa->texinfo->vecs[1][3];
		t -= fa->texturemins[1];
		t += fa->light_t * 16;
		t += 8;
		t /= BLOCK_HEIGHT * 16;

		poly->verts[i][5] = s;
		poly->verts[i][6] = t;

		// detail texture coordinates
		s = DotProduct (vec, fa->texinfo->vecs[0]) + fa->texinfo->vecs[0][3];
		s /= 128;
		t = DotProduct (vec, fa->texinfo->vecs[1]) + fa->texinfo->vecs[1][3];
		t /= 128;
		poly->verts[i][7] = s;
		poly->verts[i][8] = t;
	}
}

static void R_CreateSurfaceLightmap(msurface_t *surf)
{
	int smax, tmax;
	byte *base;

	smax = (surf->extents[0] >> 4) + 1;
	tmax = (surf->extents[1] >> 4) + 1;

	if (smax > BLOCK_WIDTH)
		Host_Error("R_CreateSurfaceLightmap: smax = %d > BLOCK_WIDTH", smax);
	if (tmax > BLOCK_HEIGHT)
		Host_Error("R_CreateSurfaceLightmap: tmax = %d > BLOCK_HEIGHT", tmax);
	if (smax * tmax > MAX_LIGHTMAP_SIZE)
		Host_Error("R_CreateSurfaceLightmap: smax * tmax = %d > MAX_LIGHTMAP_SIZE", smax * tmax);

	surf->lightmaptexturenum = AllocBlock (smax, tmax, &surf->light_s, &surf->light_t);
	base = lightmaps + surf->lightmaptexturenum * BLOCK_WIDTH * BLOCK_HEIGHT * 4;
	base += (surf->light_t * BLOCK_WIDTH + surf->light_s) * 4;
	R_BuildLightMap(surf, base, BLOCK_WIDTH * 4, 0);
}

// one interleaved static buffer per model: lit poly verts first, then turb/sky fastpoly verts
static void World_BuildModelBuffer(model_t *m)
{
	unsigned int litverts, warpverts, total, vert;
	int i, j;
	glpoly_t *poly;
	msurface_t *surf;
	scene_vert_t *verts, *v;
	SDL_GPUBuffer *buf;
	worldvb_t *vb;

	litverts = 0;
	warpverts = 0;
	for (i = 0; i < m->numsurfaces; i++)
	{
		if (m->surfaces[i].polys)
			litverts += m->surfaces[i].polys->numverts;
		if (m->surfaces[i].fastpolys)
			warpverts += m->surfaces[i].numedges;
	}

	total = litverts + warpverts;
	if (!total)
		return;

	// sanity guard for corrupt data; big community maps exceed 150k legitimately
	if (total > (1 << 22))
	{
		Com_Printf("Model \"%s\" has %u verts, not building a vertex buffer\n", m->name, total);
		return;
	}

	if (num_worldvbs >= MAX_MODELS)
		return;

	verts = malloc(total * sizeof(*verts));
	if (!verts)
		Sys_Error("World_BuildModelBuffer: Out of memory\n");

	m->num_vertices = litverts;
	if (litverts)
	{
		// cl_main.c drawflat writes vertcolours through the surfaces
		m->vertcoords = malloc(sizeof(*m->vertcoords) * 3 * litverts);
		m->vertcolours = malloc(sizeof(*m->vertcolours) * 3 * litverts);
	}

	vert = 0;
	for (i = 0; i < m->numsurfaces; i++)
	{
		poly = m->surfaces[i].polys;
		if (!poly)
			continue;

		poly->firstindex = vert;
		for (j = 0; j < poly->numverts; j++, vert++)
		{
			v = &verts[vert];
			v->pos[0] = poly->verts[j][0];
			v->pos[1] = poly->verts[j][1];
			v->pos[2] = poly->verts[j][2];
			v->st[0] = poly->verts[j][3];
			v->st[1] = poly->verts[j][4];
			v->lm[0] = poly->verts[j][5];
			v->lm[1] = poly->verts[j][6];
			v->rgba[0] = v->rgba[1] = v->rgba[2] = v->rgba[3] = 255;

			if (m->vertcoords)
			{
				m->vertcoords[3*vert+0] = v->pos[0];
				m->vertcoords[3*vert+1] = v->pos[1];
				m->vertcoords[3*vert+2] = v->pos[2];
			}
			if (m->vertcolours)
			{
				m->vertcolours[3*vert+0] = 1;
				m->vertcolours[3*vert+1] = 1;
				m->vertcolours[3*vert+2] = 1;
			}
		}
	}

	for (i = 0; i < m->numsurfaces; i++)
	{
		surf = &m->surfaces[i];
		if (!surf->fastpolys)
			continue;

		surf->fastpolyfirstindex = vert;
		for (j = 0; j < surf->numedges; j++, vert++)
		{
			v = &verts[vert];
			v->pos[0] = surf->fastpolys[j*3+0];
			v->pos[1] = surf->fastpolys[j*3+1];
			v->pos[2] = surf->fastpolys[j*3+2];
			v->st[0] = surf->shadertexcoords[j*2+0];
			v->st[1] = surf->shadertexcoords[j*2+1];
			v->lm[0] = v->lm[1] = 0;
			v->rgba[0] = v->rgba[1] = v->rgba[2] = v->rgba[3] = 255;
		}
	}

	buf = GPU_CreateStaticVertexBuffer(verts, total);
	if (!buf)
	{
		free(verts);
		return;
	}

	vb = &worldvbs[num_worldvbs++];
	vb->surfaces = m->surfaces;
	vb->buf = buf;
	vb->verts = verts;
	vb->numverts = total;
	vb->litverts = litverts;
}

//Builds the lightmap data and vertex buffers for all brush models
void R_BuildLightmaps(void)
{
	int i, j;
	model_t	*m;

	memset (allocated, 0, sizeof(allocated));

	r_framecount = 1;		// no dlightcache

	World_Shutdown();

	for (j = 1; j < MAX_MODELS; j++)
	{
		if (!(m = cl.model_precache[j]))
			break;
		if (strchr(m->name, '*'))
			continue;	// inline submodels share the parent's buffer
		if (m->type != mod_brush)
			continue;

		for (i = 0; i < m->numsurfaces; i++)
		{
			if (m->surfflags[i] & (SURF_DRAWTURB | SURF_DRAWSKY))
				continue;
			if (m->surfaces[i].texinfo->flags & TEX_SPECIAL)
				continue;
			R_CreateSurfaceLightmap(m->surfaces + i);
			BuildSurfaceDisplayList(m, m->surfaces + i);
		}

		World_BuildModelBuffer(m);
	}

	// create pages for the used slots, first upload sends whole pages
	for (i = 0; i < MAX_LIGHTMAPS; i++)
	{
		if (!allocated[i][0])
			break;		// no more used
		lightmap_pages[i] = GPU_CreateDynamicTexture(BLOCK_WIDTH, BLOCK_HEIGHT);
		lightmap_modified[i] = true;
		lightmap_rectchange[i].l = 0;
		lightmap_rectchange[i].t = 0;
		lightmap_rectchange[i].w = BLOCK_WIDTH;
		lightmap_rectchange[i].h = BLOCK_HEIGHT;
	}

	GPU_SetSceneUploader(World_UploadLightmaps);
}

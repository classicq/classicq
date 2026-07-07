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

// alias model, sprite and viewmodel rendering for the SDL_GPU path
// ported from gl_rmain.c

#include <stdlib.h>
#include <string.h>
#include <math.h>

#include "quakedef.h"
#include "r_local.h"
#include "r_skinimp.h"
#include "skinimp.h"
#include "skin.h"
#include "gpu_local.h"
#include "gpu_render.h"

extern cvar_t r_lerpframes, r_lerpmuzzlehack, gl_shaftlight;

#define NUMVERTEXNORMALS 162
#define SHADEDOT_QUANT 64

static byte r_avertexnormal_dots[SHADEDOT_QUANT][NUMVERTEXNORMALS] =
#include "anorm_dots.h"
;

static byte *shadedots = r_avertexnormal_dots[0];

static float r_framelerp;
static float r_lerpdistance;
static float shadelight, ambientlight;
static qboolean full_light;

// viewmodel batches squash depth to 0..0.3 so the gun doesn't poke into walls
static qboolean r_viewmodelpass;

static void Mat4_Scale(float *m, float x, float y, float z)
{
	int i;

	for (i = 0; i < 4; i++)
	{
		m[i] *= x;
		m[4+i] *= y;
		m[8+i] *= z;
	}
}

// ---- sprites ----

static mspriteframe_t *R_GetSpriteFrame(entity_t *e)
{
	msprite_t *psprite;
	mspritegroup_t *pspritegroup;
	mspriteframe_t *pspriteframe;
	int i, numframes, frame;
	float *pintervals, fullinterval, targettime, time;

	psprite = e->model->extradata;
	frame = e->frame;

	if (frame >= psprite->numframes || frame < 0)
	{
		Com_Printf("R_GetSpriteFrame: no such frame %d\n", frame);
		frame = 0;
	}

	if (psprite->frames[frame].type == SPR_SINGLE)
	{
		pspriteframe = psprite->frames[frame].frameptr;
	}
	else
	{
		pspritegroup = (mspritegroup_t *)psprite->frames[frame].frameptr;
		pintervals = pspritegroup->intervals;
		numframes = pspritegroup->numframes;
		fullinterval = pintervals[numframes-1];

		time = cl.time;

		// intervals are all positive, so no division by 0
		targettime = time - ((int)(time / fullinterval)) * fullinterval;

		for (i = 0; i < (numframes - 1); i++)
		{
			if (pintervals[i] > targettime)
				break;
		}

		pspriteframe = pspritegroup->frames[i];
	}

	return pspriteframe;
}

void R_DrawSpriteModel(entity_t *e)
{
	vec3_t right, up;
	vec3_t pointup, pointdown;
	mspriteframe_t *frame;
	msprite_t *psprite;
	scene_vert_t *v;
	unsigned int *idx;
	unsigned int firstvert, firstindex;
	int i;

	currententity = e;

	frame = R_GetSpriteFrame(e);
	psprite = e->model->extradata;

	if (psprite->type == SPR_ORIENTED)
	{
		// bullet marks on walls
		AngleVectors(e->angles, NULL, right, up);
	}
	else if (psprite->type == SPR_FACING_UPRIGHT)
	{
		VectorSet(up, 0, 0, 1);
		right[0] = e->origin[1] - r_origin[1];
		right[1] = -(e->origin[0] - r_origin[0]);
		right[2] = 0;
		VectorNormalizeFast(right);
	}
	else if (psprite->type == SPR_VP_PARALLEL_UPRIGHT)
	{
		VectorSet(up, 0, 0, 1);
		VectorCopy(vright, right);
	}
	else
	{
		// normal sprite
		VectorCopy(vup, up);
		VectorCopy(vright, right);
	}

	v = Scene_AllocVerts(4, &firstvert);
	if (!v)
		return;
	idx = Scene_AllocIndices(6, &firstindex);
	if (!idx)
		return;

	VectorMA(e->origin, frame->down, up, pointdown);
	VectorMA(e->origin, frame->up, up, pointup);

	VectorMA(pointdown, frame->left, right, v[0].pos);
	v[0].st[0] = 0; v[0].st[1] = 1;

	VectorMA(pointup, frame->left, right, v[1].pos);
	v[1].st[0] = 0; v[1].st[1] = 0;

	VectorMA(pointup, frame->right, right, v[2].pos);
	v[2].st[0] = 1; v[2].st[1] = 0;

	VectorMA(pointdown, frame->right, right, v[3].pos);
	v[3].st[0] = 1; v[3].st[1] = 1;

	for (i = 0; i < 4; i++)
	{
		v[i].lm[0] = v[i].lm[1] = 0;
		v[i].rgba[0] = v[i].rgba[1] = v[i].rgba[2] = v[i].rgba[3] = 255;
	}

	idx[0] = firstvert;
	idx[1] = firstvert + 1;
	idx[2] = firstvert + 2;
	idx[3] = firstvert;
	idx[4] = firstvert + 2;
	idx[5] = firstvert + 3;

	Scene_AddBatch(SCENE_PIPE_TEX_ALPHATEST, frame->gl_texturenum, NULL, GPU_GetDynamicSceneVB(), firstindex, 6, r_viewproj);
}

// ---- alias models ----

static void R_AliasSetupFullLight(model_t *model)
{
	if (model->modhint == MOD_THUNDERBOLT
	 || model->modhint == MOD_FLAME
	 || (model->modhint == MOD_PLAYER && bound(0, r_fullbrightSkins.value, cl.fbskins)))
	{
		full_light = true;
	}
	else
	{
		full_light = false;
	}
}

static void R_AliasSetupLighting(entity_t *ent)
{
	int minlight, lnum;
	float add, fbskins;
	unsigned int i;
	unsigned int j;
	vec3_t dist;
	model_t *clmodel;

	clmodel = ent->model;

	// make thunderbolt and torches full light
	if (clmodel->modhint == MOD_THUNDERBOLT)
	{
		ambientlight = 60 + 150 * bound(0, gl_shaftlight.value, 1);
		shadelight = 0;
		return;
	}
	else if (clmodel->modhint == MOD_FLAME)
	{
		ambientlight = 255;
		shadelight = 0;
		return;
	}

	ambientlight = shadelight = R_LightPoint(ent->origin);

	for (i = 0; i < MAX_DLIGHTS/32; i++)
	{
		if (cl_dlight_active[i])
		{
			for (j = 0; j < 32; j++)
			{
				if ((cl_dlight_active[i] & (1<<j)) && i*32+j < MAX_DLIGHTS)
				{
					lnum = i*32 + j;

					VectorSubtract(ent->origin, cl_dlights[lnum].origin, dist);
					add = cl_dlights[lnum].radius - VectorLength(dist);

					if (add > 0)
						ambientlight += add;
				}
			}
		}
	}

	// clamp lighting so it doesn't overbright as much
	if (ambientlight > 128)
		ambientlight = 128;
	if (ambientlight + shadelight > 192)
		shadelight = 192 - ambientlight;

	// always give the gun some light
	if ((ent->flags & RF_WEAPONMODEL) && ambientlight < 24)
		ambientlight = shadelight = 24;

	// never allow players to go totally black
	if (clmodel->modhint == MOD_PLAYER)
	{
		if (ambientlight < 8)
			ambientlight = shadelight = 8;

		fbskins = bound(0, r_fullbrightSkins.value, cl.fbskins);
		if (fbskins)
		{
			ambientlight = max(ambientlight, 8 + fbskins * 120);
			shadelight = max(shadelight, 8 + fbskins * 120);
		}
	}

	minlight = cl.minlight;

	if (ambientlight < minlight)
		ambientlight = shadelight = minlight;
}

void R_DrawAliasModel(entity_t *ent)
{
	int i, anim, skinnum, texture, fb_texture;
	int pose1, pose2;
	float scale, lerpfrac, l, modelalpha;
	unsigned char lc, alphabyte;
	vec3_t mins, maxs;
	aliashdr_t *paliashdr;
	model_t *clmodel;
	maliasframedesc_t *oldframe, *frame;
	trivertx_t *verts1, *verts2;
	scene_vert_t *v;
	unsigned int *idx;
	unsigned int firstvert, firstindex, numverts, numindices;
	SDL_GPUTexture *fbtex;
	scene_batch_t *b;
	float entmatrix[16], mvp[16];
	extern cvar_t r_viewmodelsize, cl_drawgun;

	currententity = ent;

	clmodel = ent->model;
	paliashdr = (aliashdr_t *)Mod_Extradata(ent->model);

	if (ent->frame >= paliashdr->numframes || ent->frame < 0)
	{
		Com_DPrintf("R_DrawAliasModel: no such frame %d\n", ent->frame);
		ent->frame = 0;
	}
	if (ent->oldframe >= paliashdr->numframes || ent->oldframe < 0)
	{
		Com_DPrintf("R_DrawAliasModel: no such oldframe %d\n", ent->oldframe);
		ent->oldframe = 0;
	}

	frame = &paliashdr->frames[ent->frame];
	oldframe = &paliashdr->frames[ent->oldframe];

	if (!r_lerpframes.value || ent->framelerp < 0 || ent->oldframe == ent->frame)
		r_framelerp = 1.0;
	else
		r_framelerp = min(ent->framelerp, 1);

	// culling
	if (!(ent->flags & RF_WEAPONMODEL))
	{
		if (ent->angles[0] || ent->angles[1] || ent->angles[2])
		{
			if (R_CullSphere(ent->origin, max(oldframe->radius, frame->radius)))
				return;
		}
		else
		{
			if (r_framelerp == 1)
			{
				VectorAdd(ent->origin, frame->bboxmin, mins);
				VectorAdd(ent->origin, frame->bboxmax, maxs);
			}
			else
			{
				for (i = 0; i < 3; i++)
				{
					mins[i] = ent->origin[i] + min(oldframe->bboxmin[i], frame->bboxmin[i]);
					maxs[i] = ent->origin[i] + max(oldframe->bboxmax[i], frame->bboxmax[i]);
				}
			}
			if (R_CullBox(mins, maxs))
				return;
		}
	}

	R_AliasSetupFullLight(clmodel);
	R_AliasSetupLighting(ent);

	shadedots = r_avertexnormal_dots[((int)(ent->angles[1] * (SHADEDOT_QUANT / 360.0))) & (SHADEDOT_QUANT - 1)];

	c_alias_polys += paliashdr->numtris;

	// entity transform, same order as the glTranslatef/glRotatef sequence
	Mat4_Identity(entmatrix);
	Mat4_Translate(entmatrix, ent->origin[0], ent->origin[1], ent->origin[2]);
	Mat4_RotateZ(entmatrix, ent->angles[1]);
	Mat4_RotateY(entmatrix, -ent->angles[0]);
	Mat4_RotateX(entmatrix, ent->angles[2]);

	if (clmodel->modhint == MOD_EYES)
	{
		// double size of eyes, since they are really hard to see in gl
		Mat4_Translate(entmatrix, paliashdr->scale_origin[0], paliashdr->scale_origin[1], paliashdr->scale_origin[2] - 30);
		Mat4_Scale(entmatrix, paliashdr->scale[0] * 2, paliashdr->scale[1] * 2, paliashdr->scale[2] * 2);
	}
	else if (ent->flags & RF_WEAPONMODEL)
	{
		scale = 0.5 + bound(0, r_viewmodelsize.value, 1) / 2;
		Mat4_Translate(entmatrix, paliashdr->scale_origin[0], paliashdr->scale_origin[1], paliashdr->scale_origin[2]);
		Mat4_Scale(entmatrix, paliashdr->scale[0] * scale, paliashdr->scale[1], paliashdr->scale[2]);
	}
	else
	{
		Mat4_Translate(entmatrix, paliashdr->scale_origin[0], paliashdr->scale_origin[1], paliashdr->scale_origin[2]);
		Mat4_Scale(entmatrix, paliashdr->scale[0], paliashdr->scale[1], paliashdr->scale[2]);
	}

	Mat4_Multiply(r_viewproj, entmatrix, mvp);

	anim = (int)(cl.time * 10) & 3;
	skinnum = ent->skinnum;
	if (skinnum >= paliashdr->numskins || skinnum < 0)
	{
		Com_DPrintf("R_DrawAliasModel: no such skin # %d\n", skinnum);
		skinnum = 0;
	}

	texture = paliashdr->gl_texturenum[skinnum][anim];
	fb_texture = paliashdr->fb_texturenum[skinnum][anim];

	modelalpha = (ent->flags & RF_WEAPONMODEL) ? bound(0, cl_drawgun.value, 1) : 1;

	if (ent->scoreboard)
	{
		i = ent->scoreboard - cl.players;
		if (i >= 0 && i < MAX_CLIENTS)
		{
			struct SkinImp *skinimp;

			skinimp = Skin_GetTranslation(cl.players[i].skin, cl.players[i].topcolor, cl.players[i].bottomcolor);
			if (skinimp)
			{
				texture = skinimp->texid;
				fb_texture = skinimp->fbtexid;
			}
		}
	}

	if (full_light || !gl_fb_models.value)
		fb_texture = 0;

	// pose selection
	pose1 = oldframe->firstpose;
	if (oldframe->numposes > 1)
		pose1 += (int)(cl.time / oldframe->interval) % oldframe->numposes;

	pose2 = frame->firstpose;
	if (frame->numposes > 1)
		pose2 += (int)(cl.time / frame->interval) % frame->numposes;

	numverts = paliashdr->numverts;
	numindices = paliashdr->numtris * 3;

	v = Scene_AllocVerts(paliashdr->totalverts, &firstvert);
	if (!v)
		return;
	idx = Scene_AllocIndices(numindices, &firstindex);
	if (!idx)
		return;

	verts1 = paliashdr->realposeverts + pose1 * numverts;
	verts2 = paliashdr->realposeverts + pose2 * numverts;

	alphabyte = bound(0, modelalpha * 255, 255);

	for (i = 0; i < (int)numverts; i++)
	{
		lerpfrac = r_framelerp;
		if (ent->flags & RF_LIMITLERP)
			lerpfrac = VectorL2Compare(verts1[i].v, verts2[i].v, r_lerpdistance) ? r_framelerp : 1;

		VectorInterpolate(verts1[i].v, lerpfrac, verts2[i].v, v[i].pos);

		l = FloatInterpolate(shadedots[verts1[i].lightnormalindex], lerpfrac, shadedots[verts2[i].lightnormalindex]) / 127.0;
		l = l * shadelight + ambientlight;
		l = min(l, 255);
		l = max(l, 0);
		lc = l;

		v[i].st[0] = paliashdr->texcoords[i*2 + 0];
		v[i].st[1] = paliashdr->texcoords[i*2 + 1];
		v[i].lm[0] = v[i].lm[1] = 0;
		v[i].rgba[0] = v[i].rgba[1] = v[i].rgba[2] = lc;
		v[i].rgba[3] = alphabyte;
	}

	// duplicated verts for the seam, texcoords carry the +skinwidth/2 shift
	for (i = 0; i < (int)paliashdr->collisions; i++)
	{
		v[numverts + i] = v[paliashdr->collisionmap[i]];
		v[numverts + i].st[0] = paliashdr->texcoords[(numverts + i)*2 + 0];
		v[numverts + i].st[1] = paliashdr->texcoords[(numverts + i)*2 + 1];
	}

	for (i = 0; i < (int)numindices; i++)
		idx[i] = firstvert + paliashdr->indices[i];

	fbtex = fb_texture ? GPU_Texture_Lookup(fb_texture, NULL) : NULL;
	if (fbtex)
		b = Scene_AddBatch(SCENE_PIPE_ALIAS_FB, texture, fbtex, GPU_GetDynamicSceneVB(), firstindex, numindices, mvp);
	else
		b = Scene_AddBatch(SCENE_PIPE_TEX_BLEND, texture, NULL, GPU_GetDynamicSceneVB(), firstindex, numindices, mvp);

	if (b && r_viewmodelpass)
	{
		b->depth_min = 0.0f;
		b->depth_max = 0.3f;
	}
}

// ---- viewmodel ----

void R_DrawViewModel(void)
{
	centity_t *cent;
	static entity_t gun;

	if (!r_drawentities.value || !cl.viewent.current.modelindex)
		return;

	memset(&gun, 0, sizeof(gun));
	cent = &cl.viewent;
	currententity = &gun;

	// skip on stale weapon modelindex during map-change
	if (cent->current.modelindex >= MAX_MODELS
	 || !(gun.model = cl.model_precache[cent->current.modelindex]))
		return;

	VectorCopy(cent->current.origin, gun.origin);
	VectorCopy(cent->current.angles, gun.angles);
	gun.flags = RF_WEAPONMODEL | RF_NOSHADOW;
	if (r_lerpmuzzlehack.value)
	{
		if (cent->current.modelindex != cl_modelindices[mi_vaxe]
		 && cent->current.modelindex != cl_modelindices[mi_vbio]
		 && cent->current.modelindex != cl_modelindices[mi_vgrap]
		 && cent->current.modelindex != cl_modelindices[mi_vknife]
		 && cent->current.modelindex != cl_modelindices[mi_vknife2]
		 && cent->current.modelindex != cl_modelindices[mi_vmedi]
		 && cent->current.modelindex != cl_modelindices[mi_vspan])
		{
			gun.flags |= RF_LIMITLERP;
			r_lerpdistance = 135;
		}
	}

	gun.frame = cent->current.frame;
	if (cent->frametime >= 0 && cent->frametime <= cl.time)
	{
		gun.oldframe = cent->oldframe;
		gun.framelerp = (cl.time - cent->frametime) * 10;
	}
	else
	{
		gun.oldframe = gun.frame;
		gun.framelerp = -1;
	}

	r_viewmodelpass = true;
	R_DrawAliasModel(&gun);
	r_viewmodelpass = false;
}

// ---- player skin textures via the GPU texture table ----

struct SkinImp *SkinImp_CreateSolidColour(float *colours)
{
	struct SkinImp *skinimp;
	unsigned char tex[4];

	skinimp = malloc(sizeof(*skinimp));
	if (!skinimp)
		return 0;

	tex[0] = colours[0] * 255;
	tex[1] = colours[1] * 255;
	tex[2] = colours[2] * 255;
	tex[3] = 255;

	skinimp->texid = texture_extension_number++;
	skinimp->fbtexid = 0;
	GPU_Texture_Set(skinimp->texid, GPU_CreateTextureRGBA(tex, 1, 1, 0), 1, 1, GPU_TEXPREF_LINEAR);

	return skinimp;
}

struct SkinImp *SkinImp_CreateTexturePaletted(void *data, unsigned int width, unsigned int height, unsigned int modulo)
{
	struct SkinImp *skinimp;
	unsigned int *skinmem;
	unsigned char *src;
	unsigned int *dst;
	unsigned int x;
	unsigned int y;
	unsigned int dofullbright;

	skinimp = malloc(sizeof(*skinimp));
	skinmem = malloc(width * height * 4);
	if (!skinimp || !skinmem)
	{
		free(skinmem);
		free(skinimp);
		return 0;
	}

	dofullbright = 0;
	src = data;
	dst = skinmem;
	for (y = 0; y < height; y++)
	{
		for (x = 0; x < width; x++)
		{
			if (src[x] >= 224)
				dofullbright = 1;
			dst[x] = d_8to24table[src[x]];
		}

		src += modulo;
		dst += width;
	}

	skinimp->texid = texture_extension_number++;
	skinimp->fbtexid = 0;
	GPU_Texture_Set(skinimp->texid, GPU_CreateTextureRGBA((unsigned char *)skinmem, width, height, 0), width, height, GPU_TEXPREF_LINEAR);

	if (dofullbright)
	{
		src = data;
		dst = skinmem;
		for (y = 0; y < height; y++)
		{
			for (x = 0; x < width; x++)
			{
				if (src[x] < 224)
					dst[x] = 0;
			}

			src += modulo;
			dst += width;
		}

		skinimp->fbtexid = texture_extension_number++;
		GPU_Texture_Set(skinimp->fbtexid, GPU_CreateTextureRGBA((unsigned char *)skinmem, width, height, 0), width, height, GPU_TEXPREF_LINEAR);
	}

	free(skinmem);

	return skinimp;
}

struct SkinImp *SkinImp_CreateTextureTruecolour(void *data, unsigned int width, unsigned int height)
{
	struct SkinImp *skinimp;

	skinimp = malloc(sizeof(*skinimp));
	if (!skinimp)
		return 0;

	skinimp->texid = texture_extension_number++;
	skinimp->fbtexid = 0;
	GPU_Texture_Set(skinimp->texid, GPU_CreateTextureRGBA(data, width, height, 0), width, height, GPU_TEXPREF_LINEAR);

	return skinimp;
}

void SkinImp_Destroy(struct SkinImp *skinimp)
{
	GPU_Texture_Set(skinimp->texid, NULL, 0, 0, 0);
	if (skinimp->fbtexid)
		GPU_Texture_Set(skinimp->fbtexid, NULL, 0, 0, 0);
	free(skinimp);
}

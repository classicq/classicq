/*
Copyright (C) 1996-1997 Id Software, Inc.

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
// gl_local.h -- private refresh defs

#include "gl_texture.h"
#include "gl_model.h"

#include "render.h"
#include "protocol.h"
#include "client.h"

extern int glwidth, glheight;

#define ALIAS_BASE_SIZE_RATIO		(1.0 / 11.0)
					// normalizing factor so player model works out to about
					//  1 pixel per triangle
#define	MAX_LBM_HEIGHT		480

#define SKYSHIFT		7
#define	SKYSIZE			(1 << SKYSHIFT)
#define SKYMASK			(SKYSIZE - 1)

#define BACKFACE_EPSILON	0.01

#ifdef FOD_BIGENDIAN
#define COLOURMASK_RGBA 0xffffff00
#else
#define COLOURMASK_RGBA 0x00ffffff
#endif


void R_InitGL(void);
void R_TimeRefresh_f (void);

//====================================================


int QMB_InitParticles(void);
void QMB_ShutdownParticles();
void QMB_ClearParticles(void);
void QMB_DrawParticles(void);

void QMB_RunParticleEffect(const vec3_t org, const vec3_t dir, int color, int count);
void QMB_ParticleTrail (vec3_t start, vec3_t end, vec3_t *, trail_type_t type);
void QMB_BlobExplosion (vec3_t org);
void QMB_ParticleExplosion (vec3_t org);
void QMB_LavaSplash (vec3_t org);
void QMB_TeleportSplash (vec3_t org);

void QMB_DetpackExplosion (vec3_t org);

void QMB_InfernoFlame (vec3_t org);
void QMB_StaticBubble (entity_t *ent);

extern qboolean qmb_initialized;

void GL_Particles_CvarInit(void);
void GL_Particles_TextureInit(void);

void Classic_LoadParticleTextures(void);

//====================================================

extern	entity_t	r_worldentity;
extern	qboolean	r_cache_thrash;		// compatability
extern	vec3_t		modelorg, r_entorigin;
extern	entity_t	*currententity;
extern	int			r_visframecount;
extern	int			r_framecount;
extern	mplane_t	frustum[4];
extern	int			c_brush_polys, c_alias_polys;

// view origin
extern	vec3_t	vup;
extern	vec3_t	vpn;
extern	vec3_t	vright;
extern	vec3_t	r_origin;

// screen size info
extern	refdef_t	r_refdef;
extern	mleaf_t		*r_viewleaf, *r_oldviewleaf;
extern	mleaf_t		*r_viewleaf2, *r_oldviewleaf2;	// for watervis hack
extern	texture_t	*r_notexture_mip;
extern	int			d_lightstylevalue[256];	// 8.8 fraction of base light value

extern	int	particletexture;
extern	int	netgraphtexture;
extern	int	playertextures;
extern	int	playerfbtextures[MAX_CLIENTS];
extern	int	skyboxtextures;
extern	int underwatertexture, detailtexture;

#include "gl_cvars.h"

extern	int		lightmode;		// set to gl_lightmode on mapchange

// gpu_world.c
void GL_SubdivideSurface(model_t *model, msurface_t *fa);
void EmitCausticsPolys (void);
void R_DrawSkyChain (void);
void R_LoadSky_f(void);
void R_DrawSkyBox (void);
extern qboolean	r_skyboxloaded;

// gpu_draw2d.c
void GL_Set2D (void);
void Draw_SizeChanged(void);
void R_NetGraph (void);

// gpu_rmain.c
qboolean R_CullBox (vec3_t mins, vec3_t maxs);
qboolean R_CullSphere (vec3_t centre, float radius);
void R_PolyBlend (void);
void R_BrightenScreen (void);
void R_DrawEntitiesOnList (visentlist_t *vislist);
void R_InitOtherTextures(void);

// gpu_rmain.c dlights
void R_MarkLights(model_t *model, dlight_t *light, int bit, unsigned int nodenum);
void R_AnimateLight (void);
void R_RenderDlights (void);
int R_LightPoint (vec3_t p);

// gl_refrag.c
void R_StoreEfrags (efrag_t **ppefrag);

// gl_mesh.c
void GL_MakeAliasModelDisplayLists (model_t *m, aliashdr_t *hdr);

// gpu_world.c world drawing
void R_DrawBrushModel (entity_t *e);
void R_DrawWorld (void);
void R_DrawWaterSurfaces (void);
void GL_BuildLightmaps (void);

extern qboolean gl_fbo;

void Check_Gamma (unsigned char *pal);
void VID_SetPalette (unsigned char *palette);
void GL_CvarInit(void);
void GL_Init (void);

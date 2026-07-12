/*
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

#ifndef GPU_LOCAL_H
#define GPU_LOCAL_H

#include <SDL3/SDL.h>

int GPU_Init(SDL_Window *window, int vsync);
void GPU_Shutdown(void);
void GPU_ShutdownAll(void);
void GPU_SetVsync(int vsync);

void GPU_BeginFrame(unsigned int width, unsigned int height);
void GPU_EndFrame(void);

SDL_GPUDevice *GPU_GetDevice(void);

// rgb, 3 bytes/px, rows bottom-up (glReadPixels convention)
int GPU_ReadPixels(unsigned char *rgb, unsigned int width, unsigned int height);

// column-major modelview/projection + viewport of the last 3D scene; 0 if none yet
int GPU_GetSceneMatrices(float *modelview, float *projection, int *viewport);

// post pass parameters, latched per frame by R_PostProcess_Draw
void GPU_SetPostParams(float gamma, float contrast, const float blend[4]);
float GPU_CrtCompGamma(void);

// ---- textures (gpu_texture.c) ----

#define GPU_TEXPREF_LINEAR 1
#define GPU_TEXPREF_CLAMP  2

void GPU_Texture_InitTable(void);
void GPU_Texture_ShutdownTable(void);
void GPU_Texture_Set(int texnum, SDL_GPUTexture *tex, unsigned int width, unsigned int height, int prefs);
void GPU_Texture_SetPrefs(int texnum, int prefs);
int GPU_PrefsForTexturemode(void);
SDL_GPUTexture *GPU_Texture_Lookup(int texnum, int *prefs);
int GPU_Texture_White(void);
SDL_GPUTexture *GPU_CreateTextureRGBA(const unsigned char *rgba, unsigned int width, unsigned int height, int mipmap);
// per-frame refresh (netgraph); reuses the texture when dimensions match
void GPU_UpdateTextureRGBA(int texnum, const unsigned char *rgba, unsigned int width, unsigned int height, int prefs);

// ---- 3D scene (recorded during R_RenderView, executed in GPU_EndFrame) ----

typedef struct scene_vert_s
{
	float pos[3];
	float st[2];
	float lm[2];
	unsigned char rgba[4];
} scene_vert_t;

enum
{
	SCENE_PIPE_WORLD,               // tex * lightmap, opaque
	SCENE_PIPE_WORLD_ALPHATEST,     // fence/alpha chain
	SCENE_PIPE_TEX,                 // tex * color, opaque (skybox, flat fills)
	SCENE_PIPE_TEX_ALPHATEST_NODEPTHWRITE,  // fullbright second pass
	SCENE_PIPE_ADD_NODEPTHWRITE,    // luma pass, blend one/one
	SCENE_PIPE_SKY,                 // two scrolling layers
	SCENE_PIPE_DEPTHFILL,           // skybox z fill, no color writes
	SCENE_PIPE_WATER,               // fs warp
	SCENE_PIPE_TEX_BLEND,           // alias models, alpha blend (opaque at a=255)
	SCENE_PIPE_TEX_ALPHATEST,       // sprites, depth write on
	SCENE_PIPE_ALIAS_FB,            // alias base + fullbright decal, 2 textures
	SCENE_PIPE_TEX_BLEND_NODEPTHWRITE,      // QMB lavasplash/blood3/bubble
	SCENE_PIPE_ADDALPHA_NODEPTHWRITE,       // QMB sparks/fire/smoke, srcalpha/one
	SCENE_PIPE_INVSRCCOLOR_NODEPTHWRITE,    // QMB blood1/blood2, zero/1-srccolor
	SCENE_PIPE_MOD_NODEPTHWRITE,            // caustics/detail decal, dstcolor/srccolor
	SCENE_PIPE_COUNT
};

#define SCENE_MAX_PARAMS 8

typedef struct scene_batch_s
{
	int pipe;
	int texnum;                 // main texture via texture table
	SDL_GPUTexture *tex2;       // lightmap page or sky alpha layer, owned by caller
	SDL_GPUBuffer *vbuf;        // static vertex buffer the indices refer to
	unsigned int firstindex;
	unsigned int numindices;
	float depth_min;            // viewport depth range, viewmodel squashes to 0..0.3
	float depth_max;
	float mvp[16];
	float params[SCENE_MAX_PARAMS];  // sky: origin xyz + speed1, speed2; water: cltime
} scene_batch_t;

#define GPU_SCENE_MAX_INDICES (1 << 20)
#define GPU_SCENE_MAX_BATCHES 8192

SDL_GPUBuffer *GPU_CreateStaticVertexBuffer(const scene_vert_t *verts, unsigned int count);
void GPU_ReleaseBuffer(SDL_GPUBuffer *buf);
SDL_GPUTexture *GPU_CreateDynamicTexture(unsigned int width, unsigned int height);
void GPU_ReleaseTexture(SDL_GPUTexture *tex);

#define GPU_SCENE_MAX_DYNVERTS 65536

void Scene_FrameReset(void);
unsigned int *Scene_AllocIndices(unsigned int count, unsigned int *firstindex);
// per-frame streamed vertices (skybox, sprites); batches use GPU_GetDynamicSceneVB()
scene_vert_t *Scene_AllocVerts(unsigned int count, unsigned int *firstvert);
SDL_GPUBuffer *GPU_GetDynamicSceneVB(void);
scene_batch_t *Scene_AddBatch(int pipe, int texnum, SDL_GPUTexture *tex2, SDL_GPUBuffer *vbuf,
	unsigned int firstindex, unsigned int numindices, const float *mvp);
void Scene_SetViewport(float x, float y, float w, float h);

// copy-pass hook for dirty lightmap uploads, runs right before the scene pass
void GPU_SetSceneUploader(void (*fn)(SDL_GPUCopyPass *copy));

// latched for autoID readback
void GPU_SetSceneMatrices(const float *modelview, const float *projection, const int *viewport);

// ---- 2D batcher (gpu_draw2d.c) ----

#define GPU_UI_MAX_VERTS 65536

typedef struct ui_batch_s
{
	int texnum;
	int alphatest;
	unsigned int firstvert;
	unsigned int numverts;
	float ortho_w;
	float ortho_h;
} ui_batch_t;

typedef struct ui_vert_s
{
	float x, y;
	float u, v;
	unsigned char rgba[4];
} ui_vert_t;

void Draw2D_FrameReset(void);
void Draw2D_Quad(int texnum, int alphatest,
	float x0, float y0, float x1, float y1,
	float s0, float t0, float s1, float t1,
	const unsigned char rgba[4]);
const ui_vert_t *Draw2D_GetVerts(unsigned int *numverts);
const ui_batch_t *Draw2D_GetBatches(unsigned int *numbatches);

#endif

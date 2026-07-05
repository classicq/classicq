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
void GPU_SetVsync(int vsync);

void GPU_BeginFrame(unsigned int width, unsigned int height);
void GPU_EndFrame(void);

SDL_GPUDevice *GPU_GetDevice(void);
SDL_GPUCommandBuffer *GPU_GetCommandBuffer(void);
SDL_GPUTexture *GPU_GetSceneColor(void);
SDL_GPUTexture *GPU_GetSceneDepth(void);
SDL_GPUTextureFormat GPU_GetSceneDepthFormat(void);

// rgb, 3 bytes/px, rows bottom-up (glReadPixels convention)
int GPU_ReadPixels(unsigned char *rgb, unsigned int width, unsigned int height);

// column-major modelview/projection + viewport of the last 3D scene; 0 if none yet
int GPU_GetSceneMatrices(float *modelview, float *projection, int *viewport);

#endif

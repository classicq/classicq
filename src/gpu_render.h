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

// internal contract between gpu_rmain.c and gpu_world.c

#ifndef GPU_RENDER_H
#define GPU_RENDER_H

// column-major 4x4, GL layout (translation in m[12..14])
void Mat4_Identity(float *m);
void Mat4_Multiply(const float *a, const float *b, float *out);
void Mat4_Translate(float *m, float x, float y, float z);
void Mat4_RotateZ(float *m, float deg);
void Mat4_RotateY(float *m, float deg);
void Mat4_RotateX(float *m, float deg);

// viewproj for the current scene, depth 0..1 clip; set by R_SetupGL port
extern float r_viewproj[16];

// lightmap uploads hook target, implemented in gpu_world.c
struct SDL_GPUCopyPass;
void World_UploadLightmaps(struct SDL_GPUCopyPass *copy);

#endif

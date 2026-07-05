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

#include <string.h>

#include "quakedef.h"
#include "gpu_local.h"

#define MAX_BATCHES 4096

static ui_vert_t verts[GPU_UI_MAX_VERTS];
static unsigned int numverts;

static ui_batch_t batches[MAX_BATCHES];
static unsigned int numbatches;

static int warned_overflow;

void Draw2D_FrameReset(void)
{
	numverts = 0;
	numbatches = 0;
	warned_overflow = 0;
}

void Draw2D_Quad(int texnum, int alphatest,
	float x0, float y0, float x1, float y1,
	float s0, float t0, float s1, float t1,
	const unsigned char rgba[4])
{
	ui_batch_t *b;
	ui_vert_t *v;
	float ow = (float)vid.conwidth;
	float oh = (float)vid.conheight;

	if (numverts + 6 > GPU_UI_MAX_VERTS || numbatches >= MAX_BATCHES)
	{
		if (!warned_overflow)
		{
			Com_Printf("Draw2D: batch overflow\n");
			warned_overflow = 1;
		}
		return;
	}

	b = numbatches ? &batches[numbatches - 1] : NULL;
	if (!b || b->texnum != texnum || b->alphatest != alphatest
		|| b->ortho_w != ow || b->ortho_h != oh)
	{
		b = &batches[numbatches++];
		b->texnum = texnum;
		b->alphatest = alphatest;
		b->firstvert = numverts;
		b->numverts = 0;
		b->ortho_w = ow;
		b->ortho_h = oh;
	}

	v = &verts[numverts];

	v[0].x = x0; v[0].y = y0; v[0].u = s0; v[0].v = t0;
	v[1].x = x1; v[1].y = y0; v[1].u = s1; v[1].v = t0;
	v[2].x = x1; v[2].y = y1; v[2].u = s1; v[2].v = t1;
	v[3].x = x0; v[3].y = y0; v[3].u = s0; v[3].v = t0;
	v[4].x = x1; v[4].y = y1; v[4].u = s1; v[4].v = t1;
	v[5].x = x0; v[5].y = y1; v[5].u = s0; v[5].v = t1;

	memcpy(v[0].rgba, rgba, 4);
	memcpy(v[1].rgba, rgba, 4);
	memcpy(v[2].rgba, rgba, 4);
	memcpy(v[3].rgba, rgba, 4);
	memcpy(v[4].rgba, rgba, 4);
	memcpy(v[5].rgba, rgba, 4);

	numverts += 6;
	b->numverts += 6;
}

const ui_vert_t *Draw2D_GetVerts(unsigned int *out_numverts)
{
	*out_numverts = numverts;
	return verts;
}

const ui_batch_t *Draw2D_GetBatches(unsigned int *out_numbatches)
{
	*out_numbatches = numbatches;
	return batches;
}

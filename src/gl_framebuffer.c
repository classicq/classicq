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

#include "quakedef.h"
#include "gl_local.h"
#include "gl_framebuffer.h"

static GLuint fb_id;
static GLuint fb_color_tex;
static GLuint fb_depth_rb;
static int fb_width;
static int fb_height;
static qboolean fb_inited;

static void GL_FBO_Destroy(void)
{
	if (fb_id)
	{
		qglDeleteFramebuffers(1, &fb_id);
		fb_id = 0;
	}
	if (fb_depth_rb)
	{
		qglDeleteRenderbuffers(1, &fb_depth_rb);
		fb_depth_rb = 0;
	}
	if (fb_color_tex)
	{
		// fb_color_tex came from texture_extension_number, not glGenTextures
		glDeleteTextures(1, &fb_color_tex);
		fb_color_tex = 0;
	}
	fb_width = 0;
	fb_height = 0;
	fb_inited = false;
}

qboolean GL_FBO_Init(int width, int height)
{
	extern int texture_extension_number;
	GLenum status;

	if (!gl_fbo)
		return false;

	if (width <= 0 || height <= 0)
		return false;

	if (fb_inited && fb_width == width && fb_height == height)
		return true;

	GL_FBO_Destroy();

	// reserve name through texture_extension_number, not glGenTextures
	fb_color_tex = texture_extension_number++;
	glBindTexture(GL_TEXTURE_2D, fb_color_tex);
	glTexImage2D(GL_TEXTURE_2D, 0, GL_RGBA8, width, height, 0, GL_RGBA, GL_UNSIGNED_BYTE, 0);
	glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MIN_FILTER, GL_NEAREST);
	glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MAG_FILTER, GL_NEAREST);
	glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_WRAP_S, GL_CLAMP_TO_EDGE);
	glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_WRAP_T, GL_CLAMP_TO_EDGE);

	qglGenRenderbuffers(1, &fb_depth_rb);
	qglBindRenderbuffer(GL_RENDERBUFFER, fb_depth_rb);
	qglRenderbufferStorage(GL_RENDERBUFFER, GL_DEPTH_COMPONENT24, width, height);

	qglGenFramebuffers(1, &fb_id);
	qglBindFramebuffer(GL_FRAMEBUFFER, fb_id);
	qglFramebufferTexture2D(GL_FRAMEBUFFER, GL_COLOR_ATTACHMENT0, GL_TEXTURE_2D, fb_color_tex, 0);
	qglFramebufferRenderbuffer(GL_FRAMEBUFFER, GL_DEPTH_ATTACHMENT, GL_RENDERBUFFER, fb_depth_rb);

	status = qglCheckFramebufferStatus(GL_FRAMEBUFFER);
	qglBindFramebuffer(GL_FRAMEBUFFER, 0);
	qglBindRenderbuffer(GL_RENDERBUFFER, 0);

	if (status != GL_FRAMEBUFFER_COMPLETE)
	{
		Com_Printf("GL_FBO_Init: framebuffer incomplete (0x%x)\n", status);
		GL_FBO_Destroy();
		return false;
	}

	fb_width = width;
	fb_height = height;
	fb_inited = true;
	return true;
}

void GL_FBO_Shutdown(void)
{
	GL_FBO_Destroy();
}

qboolean GL_FBO_Resize(int width, int height)
{
	return GL_FBO_Init(width, height);
}

void GL_FBO_Bind(void)
{
	if (fb_inited)
		qglBindFramebuffer(GL_FRAMEBUFFER, fb_id);
}

void GL_FBO_Unbind(void)
{
	if (fb_inited)
		qglBindFramebuffer(GL_FRAMEBUFFER, 0);
}

unsigned int GL_FBO_GetColorTexture(void)
{
	return fb_color_tex;
}

unsigned int GL_FBO_GetID(void)
{
	return fb_id;
}

int GL_FBO_GetWidth(void)
{
	return fb_width;
}

int GL_FBO_GetHeight(void)
{
	return fb_height;
}

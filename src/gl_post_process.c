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
#include "gl_shader.h"
#include "gl_framebuffer.h"
#include "gl_post_process.h"

static int pp_program;
static int u_base;
static int u_gamma;
static int u_contrast;
static int u_blend;

static const char *pp_fragment_src =
	"#version 120\n"
	"uniform sampler2D base;\n"
	"uniform float gamma;\n"
	"uniform float contrast;\n"
	"uniform vec4 v_blend;\n"
	"void main(void)\n"
	"{\n"
	"    vec3 c = texture2D(base, gl_TexCoord[0].st).rgb;\n"
	"    c = mix(c, v_blend.rgb, v_blend.a);\n"
	"    c *= contrast;\n"
	"    c = pow(max(c, vec3(0.0)), vec3(gamma));\n"
	"    gl_FragColor = vec4(c, 1.0);\n"
	"}\n";

qboolean GL_PostProcess_Init(void)
{
	if (!gl_fs)
		return false;

	if (pp_program)
		return true;

	pp_program = GL_SetupShaderProgram(0, 0, 0, pp_fragment_src);
	if (!pp_program)
		return false;

	u_base     = qglGetUniformLocation(pp_program, "base");
	u_gamma    = qglGetUniformLocation(pp_program, "gamma");
	u_contrast = qglGetUniformLocation(pp_program, "contrast");
	u_blend    = qglGetUniformLocation(pp_program, "v_blend");

	return true;
}

void GL_PostProcess_Shutdown(void)
{
	if (pp_program)
	{
		qglDeleteProgram(pp_program);
		pp_program = 0;
	}
}

qboolean GL_PostProcess_IsReady(void)
{
	return pp_program != 0;
}

void GL_PostProcess_Draw(unsigned int color_tex, float gamma, float contrast, const float blend[4])
{
	GLint prev_active_tex = GL_TEXTURE0;

	if (!pp_program)
		return;

	glGetIntegerv(GL_ACTIVE_TEXTURE, &prev_active_tex);

	glPushAttrib(GL_ALL_ATTRIB_BITS);
	glPushClientAttrib(GL_CLIENT_ALL_ATTRIB_BITS);

	glViewport(0, 0, GL_FBO_GetWidth(), GL_FBO_GetHeight());
	glDisable(GL_DEPTH_TEST);
	glDepthMask(GL_FALSE);
	glDisable(GL_BLEND);
	glDisable(GL_ALPHA_TEST);
	glDisable(GL_CULL_FACE);
	glDisable(GL_SCISSOR_TEST);

	glActiveTexture(GL_TEXTURE0);
	glBindTexture(GL_TEXTURE_2D, color_tex);
	glEnable(GL_TEXTURE_2D);
	glColor4f(1.0f, 1.0f, 1.0f, 1.0f);

	qglUseProgram(pp_program);
	qglUniform1i(u_base, 0);
	qglUniform1f(u_gamma, gamma);
	qglUniform1f(u_contrast, contrast);
	qglUniform4f(u_blend, blend[0], blend[1], blend[2], blend[3]);

	glMatrixMode(GL_TEXTURE);
	glPushMatrix();
	glLoadIdentity();
	glMatrixMode(GL_PROJECTION);
	glPushMatrix();
	glLoadIdentity();
	glMatrixMode(GL_MODELVIEW);
	glPushMatrix();
	glLoadIdentity();

	glBegin(GL_QUADS);
	glTexCoord2f(0.0f, 0.0f); glVertex2f(-1.0f, -1.0f);
	glTexCoord2f(1.0f, 0.0f); glVertex2f( 1.0f, -1.0f);
	glTexCoord2f(1.0f, 1.0f); glVertex2f( 1.0f,  1.0f);
	glTexCoord2f(0.0f, 1.0f); glVertex2f(-1.0f,  1.0f);
	glEnd();

	glMatrixMode(GL_TEXTURE);
	glPopMatrix();
	glMatrixMode(GL_PROJECTION);
	glPopMatrix();
	glMatrixMode(GL_MODELVIEW);
	glPopMatrix();

	qglUseProgram(0);

	glPopClientAttrib();
	glPopAttrib();

	glActiveTexture(prev_active_tex);
}

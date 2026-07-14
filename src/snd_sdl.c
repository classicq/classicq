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

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include <SDL3/SDL.h>

#include "quakedef.h"
#include "sound.h"

static cvar_t s_desiredsamples = {"s_desiredsamples", "512", CVAR_ARCHIVE};

static SDL_AudioStream *g_stream;

static void sdl_audio_callback(void *userdata, SDL_AudioStream *stream, int additional_amount, int total_amount)
{
	struct SoundCard *sc = userdata;
	static short buf[4096];
	int framebytes = sc->channels * (sc->samplebits / 8);
	int frames = (additional_amount + framebytes - 1) / framebytes;
	int maxframes = sizeof(buf) / framebytes;

	(void)total_amount;

	while (frames > 0)
	{
		int n = frames < maxframes ? frames : maxframes;
		S_MixAudio(buf, n);
		SDL_PutAudioStreamData(stream, buf, n * framebytes);
		frames -= n;
	}
}

static void sdl_audio_shutdown(struct SoundCard *sc)
{
	if (g_stream)
	{
		SDL_DestroyAudioStream(g_stream);
		g_stream = NULL;
	}
	SDL_QuitSubSystem(SDL_INIT_AUDIO);
}

static qboolean sdl_audio_init(struct SoundCard *sc, int rate, int channels, int bits)
{
	SDL_AudioSpec spec, devspec;
	int devframes = 0;

	if (bits != 16)
	{
		Com_Printf("SDL audio: only 16-bit samples supported (got %d)\n", bits);
		return false;
	}

	if (s_desiredsamples.value > 0)
	{
		char tmp[16];
		snprintf(tmp, sizeof(tmp), "%d", (int)s_desiredsamples.value);
		SDL_SetHint(SDL_HINT_AUDIO_DEVICE_SAMPLE_FRAMES, tmp);
	}
	else
		SDL_ResetHint(SDL_HINT_AUDIO_DEVICE_SAMPLE_FRAMES);

	if (!SDL_InitSubSystem(SDL_INIT_AUDIO))
	{
		Com_Printf("SDL_InitSubSystem(AUDIO) failed: %s\n", SDL_GetError());
		return false;
	}

	// match device rate to avoid resampling, like SDL2 ALLOW_FREQUENCY_CHANGE
	if (SDL_GetAudioDeviceFormat(SDL_AUDIO_DEVICE_DEFAULT_PLAYBACK, &devspec, NULL))
	{
		if (devspec.freq > 0)
			rate = devspec.freq;
		if (devspec.channels == 1)
			channels = 1;
	}

	memset(&spec, 0, sizeof(spec));
	spec.freq = rate;
	spec.format = SDL_AUDIO_S16;
	spec.channels = channels;

	sc->channels = channels;
	sc->samplebits = 16;
	sc->speed = rate;
	sc->Shutdown = sdl_audio_shutdown;

	g_stream = SDL_OpenAudioDeviceStream(SDL_AUDIO_DEVICE_DEFAULT_PLAYBACK, &spec, sdl_audio_callback, sc);
	if (!g_stream)
	{
		Com_Printf("SDL_OpenAudioDeviceStream failed: %s\n", SDL_GetError());
		SDL_QuitSubSystem(SDL_INIT_AUDIO);
		return false;
	}

	SDL_GetAudioDeviceFormat(SDL_GetAudioStreamDevice(g_stream), &devspec, &devframes);

	SDL_ResumeAudioStreamDevice(g_stream);

	Com_Printf("SDL audio: %d Hz, %d ch, %d-sample fragments\n",
		rate, channels, devframes);

	return true;
}

static void sdl_audio_cvarinit(void)
{
	Cvar_SetCurrentGroup(CVAR_GROUP_SOUND);
	Cvar_Register(&s_desiredsamples);
	Cvar_ResetCurrentGroup();
}

SoundInitFunc SNDSDL_Init = sdl_audio_init;
SoundCvarInitFunc SNDSDL_CvarInit = sdl_audio_cvarinit;

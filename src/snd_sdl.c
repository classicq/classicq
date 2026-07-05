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

#include <stdlib.h>
#include <string.h>

#include <SDL3/SDL.h>

#include "quakedef.h"
#include "sound.h"

static SDL_AudioStream *g_stream;
static int g_buffer_bytes;

static void sdl_audio_callback(void *userdata, SDL_AudioStream *stream, int additional_amount, int total_amount)
{
	struct SoundCard *sc = userdata;
	int pos = sc->samplepos * (sc->samplebits / 8);
	int len = additional_amount;

	(void)total_amount;

	while (len > 0)
	{
		int chunk = g_buffer_bytes - pos;
		if (chunk > len)
			chunk = len;
		SDL_PutAudioStreamData(stream, (Uint8 *)sc->buffer + pos, chunk);
		pos = (pos + chunk) % g_buffer_bytes;
		len -= chunk;
	}

	sc->samplepos = pos / (sc->samplebits / 8);
}

static int sdl_audio_get_dma_pos(struct SoundCard *sc)
{
	return sc->samplepos;
}

static void sdl_audio_submit(struct SoundCard *sc, unsigned int count)
{
}

static void *sdl_audio_lock(struct SoundCard *sc)
{
	SDL_LockAudioStream(g_stream);
	return sc->buffer;
}

static void sdl_audio_unlock(struct SoundCard *sc)
{
	SDL_UnlockAudioStream(g_stream);
}

static void sdl_audio_shutdown(struct SoundCard *sc)
{
	if (g_stream)
	{
		SDL_DestroyAudioStream(g_stream);
		g_stream = NULL;
	}
	if (sc->buffer)
	{
		free(sc->buffer);
		sc->buffer = NULL;
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

	if (!SDL_InitSubSystem(SDL_INIT_AUDIO))
	{
		Com_Printf("SDL_InitSubSystem(AUDIO) failed: %s\n", SDL_GetError());
		return false;
	}

	// match device rate to avoid resampling, like SDL2 ALLOW_FREQUENCY_CHANGE
	if (SDL_GetAudioDeviceFormat(SDL_AUDIO_DEVICE_DEFAULT_PLAYBACK, &devspec, &devframes))
	{
		if (devspec.freq > 0)
			rate = devspec.freq;
		if (devspec.channels == 1)
			channels = 1;
	}
	if (devframes < 1024 || devframes > 4096)
		devframes = 1024;

	memset(&spec, 0, sizeof(spec));
	spec.freq = rate;
	spec.format = SDL_AUDIO_S16;
	spec.channels = channels;

	g_stream = SDL_OpenAudioDeviceStream(SDL_AUDIO_DEVICE_DEFAULT_PLAYBACK, &spec, sdl_audio_callback, sc);
	if (!g_stream)
	{
		Com_Printf("SDL_OpenAudioDeviceStream failed: %s\n", SDL_GetError());
		SDL_QuitSubSystem(SDL_INIT_AUDIO);
		return false;
	}

	g_buffer_bytes = devframes * channels * 2 * 8;
	sc->buffer = malloc(g_buffer_bytes);
	if (!sc->buffer)
	{
		Com_Printf("SDL audio: out of memory for %d-byte buffer\n", g_buffer_bytes);
		SDL_DestroyAudioStream(g_stream);
		g_stream = NULL;
		SDL_QuitSubSystem(SDL_INIT_AUDIO);
		return false;
	}
	memset(sc->buffer, 0, g_buffer_bytes);

	sc->channels = channels;
	sc->samples = g_buffer_bytes / 2;
	sc->samplepos = 0;
	sc->samplebits = 16;
	sc->speed = rate;

	sc->GetDMAPos = sdl_audio_get_dma_pos;
	sc->Submit = sdl_audio_submit;
	sc->Lock = sdl_audio_lock;
	sc->Unlock = sdl_audio_unlock;
	sc->Shutdown = sdl_audio_shutdown;

	SDL_ResumeAudioStreamDevice(g_stream);

	Com_Printf("SDL audio: %d Hz, %d ch, %d-sample fragments\n",
		rate, channels, devframes);

	return true;
}

static void sdl_audio_cvarinit(void)
{
}

SoundInitFunc SNDSDL_Init = sdl_audio_init;
SoundCvarInitFunc SNDSDL_CvarInit = sdl_audio_cvarinit;

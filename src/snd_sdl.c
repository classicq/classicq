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

#define SDL_MAIN_HANDLED 1
#include <SDL.h>

#include "quakedef.h"
#include "sound.h"

static SDL_AudioDeviceID g_dev;
static int g_buffer_bytes;

static void sdl_audio_callback(void *userdata, Uint8 *stream, int len)
{
	struct SoundCard *sc = userdata;
	int pos = sc->samplepos * (sc->samplebits / 8);

	if (pos + len <= g_buffer_bytes)
	{
		memcpy(stream, (Uint8 *)sc->buffer + pos, len);
	}
	else
	{
		int first = g_buffer_bytes - pos;
		memcpy(stream, (Uint8 *)sc->buffer + pos, first);
		memcpy(stream + first, sc->buffer, len - first);
	}

	sc->samplepos = ((pos + len) % g_buffer_bytes) / (sc->samplebits / 8);
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
	SDL_LockAudioDevice(g_dev);
	return sc->buffer;
}

static void sdl_audio_unlock(struct SoundCard *sc)
{
	SDL_UnlockAudioDevice(g_dev);
}

static void sdl_audio_shutdown(struct SoundCard *sc)
{
	if (g_dev)
	{
		SDL_CloseAudioDevice(g_dev);
		g_dev = 0;
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
	SDL_AudioSpec desired, obtained;

	if (bits != 16)
	{
		Com_Printf("SDL audio: only 16-bit samples supported (got %d)\n", bits);
		return false;
	}

	if (SDL_InitSubSystem(SDL_INIT_AUDIO) != 0)
	{
		Com_Printf("SDL_InitSubSystem(AUDIO) failed: %s\n", SDL_GetError());
		return false;
	}

	memset(&desired, 0, sizeof(desired));
	desired.freq = rate;
	desired.format = AUDIO_S16SYS;
	desired.channels = channels;
	desired.samples = 1024;
	desired.callback = sdl_audio_callback;
	desired.userdata = sc;

	g_dev = SDL_OpenAudioDevice(NULL, 0, &desired, &obtained,
		SDL_AUDIO_ALLOW_FREQUENCY_CHANGE | SDL_AUDIO_ALLOW_CHANNELS_CHANGE);
	if (g_dev == 0)
	{
		Com_Printf("SDL_OpenAudioDevice failed: %s\n", SDL_GetError());
		SDL_QuitSubSystem(SDL_INIT_AUDIO);
		return false;
	}

	if (obtained.format != AUDIO_S16SYS)
	{
		Com_Printf("SDL audio: unexpected format 0x%x\n", obtained.format);
		SDL_CloseAudioDevice(g_dev);
		g_dev = 0;
		SDL_QuitSubSystem(SDL_INIT_AUDIO);
		return false;
	}

	g_buffer_bytes = obtained.samples * obtained.channels * 2 * 8;
	sc->buffer = malloc(g_buffer_bytes);
	if (!sc->buffer)
	{
		Com_Printf("SDL audio: out of memory for %d-byte buffer\n", g_buffer_bytes);
		SDL_CloseAudioDevice(g_dev);
		g_dev = 0;
		SDL_QuitSubSystem(SDL_INIT_AUDIO);
		return false;
	}
	memset(sc->buffer, 0, g_buffer_bytes);

	sc->channels = obtained.channels;
	sc->samples = g_buffer_bytes / 2;
	sc->samplepos = 0;
	sc->samplebits = 16;
	sc->speed = obtained.freq;

	sc->GetDMAPos = sdl_audio_get_dma_pos;
	sc->Submit = sdl_audio_submit;
	sc->Lock = sdl_audio_lock;
	sc->Unlock = sdl_audio_unlock;
	sc->Shutdown = sdl_audio_shutdown;

	SDL_PauseAudioDevice(g_dev, 0);

	Com_Printf("SDL audio: %d Hz, %d ch, %d-sample fragments\n",
		obtained.freq, obtained.channels, obtained.samples);

	return true;
}

static void sdl_audio_cvarinit(void)
{
}

SoundInitFunc SNDSDL_Init = sdl_audio_init;
SoundCvarInitFunc SNDSDL_CvarInit = sdl_audio_cvarinit;

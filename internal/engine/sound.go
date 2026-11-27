// Copyright (C) 1996-1997 Id Software, Inc.
//
// This program is free software; you can redistribute it and/or
// modify it under the terms of the GNU General Public License
// as published by the Free Software Foundation; either version 2
// of the License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.
//
// See the GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program; if not, write to the Free Software
// Foundation, Inc., 59 Temple Place - Suite 330, Boston, MA  02111-1307, USA.

package engine

// sound.h -- client sound i/o functions

type portable_samplepair_t struct {
	left  int32
	right int32
}

type sfxcache_t struct {
	length    int
	loopstart int
	speed     int
	width     int
	stereo    int
	data      []byte // variable sized
}

type sfx_t struct {
	name     string
	sfxcache *sfxcache_t
}

type channel_t struct {
	sfx        *sfx_t  // sfx number
	leftvol    int     // 0-255 volume
	rightvol   int     // 0-255 volume
	end        int     // end time in global paintsamples
	pos        int     // sample position in sfx
	entnum     int     // to allow overriding a specific sound
	entchannel int     //
	origin     vec3_t  // origin of sound effect
	dist_mult  float32 // distance multiplier (attenuation/clipK)
	master_vol int     // 0-255 master volume
}

type wavinfo_t struct {
	rate      int
	width     int
	channels  int
	loopstart int
	samples   int
	dataofs   int // chunk starts this many bytes from file start
}

// driverprivate is unused by the SDL driver and left out
type SoundCard struct {
	Shutdown func(*SoundCard)
	Update   func(*SoundCard) // once per frame, feeds the device

	channels   int
	samplebits int
	speed      int
	buffer     []byte // current mix destination, set by S_MixAudio

	drivername string
}

type SoundCvarInitFunc func()
type SoundInitFunc func(sc *SoundCard, rate, channels, bits int) bool

// ====================================================================
// User-setable variables
// ====================================================================

const (
	MAX_CHANNELS         = 128
	MAX_DYNAMIC_CHANNELS = 32
)

// 0 to MAX_DYNAMIC_CHANNELS-1	= normal entity sounds
// MAX_DYNAMIC_CHANNELS to MAX_DYNAMIC_CHANNELS + NUM_AMBIENTS -1 = water, etc
// MAX_DYNAMIC_CHANNELS + NUM_AMBIENTS to total_channels = static sounds

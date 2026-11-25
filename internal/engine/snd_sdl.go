package engine

// snd_sdl.c

import (
	"fmt"
	"unsafe"

	"github.com/classicq/classicq/internal/sdl"
)

var s_desiredsamples = cvar_t{name: "s_desiredsamples", string: "512", flags: CVAR_ARCHIVE}

// seconds of mixed audio kept queued ahead of the device, never under two fragments
var s_mixahead = cvar_t{name: "s_mixahead", string: "0.03", flags: CVAR_ARCHIVE}

var g_stream *sdl.AudioStream

var sdl_audio_buf [4096 * 2]byte // short buf[4096]

var sdl_audio_fragment int // device period in sample frames

// The main thread tops the stream up once per frame instead of mixing in
// an SDL callback: no Go code runs on a foreign thread and the mixer needs
// no lock. The price is s_mixahead of latency, and a gap when a frame
// takes longer than that.
func sdl_audio_update(sc *SoundCard) {
	framebytes := sc.channels * (sc.samplebits / 8)
	ahead := max(int(float64(sc.speed)*float64(s_mixahead.value)), 2*sdl_audio_fragment)
	frames := ahead - int(sdl.GetAudioStreamQueued(g_stream))/framebytes
	maxframes := len(sdl_audio_buf) / framebytes

	for frames > 0 {
		n := min(frames, maxframes)
		S_MixAudio(sdl_audio_buf[:], n)
		sdl.PutAudioStreamData(g_stream, unsafe.Pointer(&sdl_audio_buf[0]), int32(n*framebytes))
		frames -= n
	}
}

func sdl_audio_shutdown(sc *SoundCard) {
	if g_stream != nil {
		sdl.DestroyAudioStream(g_stream)
		g_stream = nil
	}
	sdl.QuitSubSystem(sdl.INIT_AUDIO)
}

func sdl_audio_init(sc *SoundCard, rate, channels, bits int) bool {
	var spec, devspec sdl.AudioSpec
	var devframes int32

	if bits != 16 {
		Com_Printf("SDL audio: only 16-bit samples supported (got %d)\n", bits)
		return false
	}

	if s_desiredsamples.value > 0 {
		sdl.SetHint(sdl.HINT_AUDIO_DEVICE_SAMPLE_FRAMES, fmt.Sprintf("%d", int(s_desiredsamples.value)))
	} else {
		sdl.ResetHint(sdl.HINT_AUDIO_DEVICE_SAMPLE_FRAMES)
	}

	if !sdl.InitSubSystem(sdl.INIT_AUDIO) {
		Com_Printf("SDL_InitSubSystem(AUDIO) failed: %s\n", sdl.GetError())
		return false
	}

	// match device rate to avoid resampling, like SDL2 ALLOW_FREQUENCY_CHANGE
	if sdl.GetAudioDeviceFormat(sdl.AUDIO_DEVICE_DEFAULT_PLAYBACK, &devspec, nil) {
		if devspec.Freq > 0 {
			rate = int(devspec.Freq)
		}
		if devspec.Channels == 1 {
			channels = 1
		}
	}

	spec = sdl.AudioSpec{}
	spec.Freq = int32(rate)
	spec.Format = sdl.AUDIO_S16
	spec.Channels = int32(channels)

	sc.channels = channels
	sc.samplebits = 16
	sc.speed = rate
	sc.Shutdown = sdl_audio_shutdown
	sc.Update = sdl_audio_update

	g_stream = sdl.OpenAudioDeviceStream(sdl.AUDIO_DEVICE_DEFAULT_PLAYBACK, &spec, 0, nil)
	if g_stream == nil {
		Com_Printf("SDL_OpenAudioDeviceStream failed: %s\n", sdl.GetError())
		sdl.QuitSubSystem(sdl.INIT_AUDIO)
		return false
	}

	sdl.GetAudioDeviceFormat(sdl.GetAudioStreamDevice(g_stream), &devspec, &devframes)
	sdl_audio_fragment = max(int(devframes), 128) // 0 when the query fails

	sdl.ResumeAudioStreamDevice(g_stream)

	Com_Printf("SDL audio: %d Hz, %d ch, %d-sample fragments\n",
		rate, channels, devframes)

	return true
}

func sdl_audio_cvarinit() {
	Cvar_SetCurrentGroup(CVAR_GROUP_SOUND)
	Cvar_Register(&s_desiredsamples)
	Cvar_Register(&s_mixahead)
	Cvar_ResetCurrentGroup()
}

func init() {
	SNDSDL_Init = sdl_audio_init
	SNDSDL_CvarInit = sdl_audio_cvarinit
}

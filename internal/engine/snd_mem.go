package engine

// snd_mem.c: sound caching

import "fmt"

func ResampleSfx(sc *sfxcache_t, inrate, inwidth int, data []byte) {
	var outcount int
	var srcsample int
	var stepscale float32
	var i int
	var sample, samplefrac, fracstep int

	stepscale = float32(inrate) / float32(soundcard.speed) // this is usually 0.5, 1, or 2

	outcount = int(float32(sc.length) / stepscale)
	sc.length = outcount
	if sc.loopstart != -1 {
		sc.loopstart = int(float32(sc.loopstart) / stepscale)
	}

	sc.speed = soundcard.speed
	if s_loadas8bit.value != 0 {
		sc.width = 1
	} else {
		sc.width = inwidth
	}
	sc.stereo = 0

	// resample / decimate to the current source rate

	if stepscale == 1 && inwidth == 1 && sc.width == 1 {
		// fast special case
		for i = 0; i < outcount; i++ {
			sc.data[i] = byte(int(data[i]) - 128)
		}
	} else {
		// general case
		samplefrac = 0
		fracstep = int(stepscale * 256)
		for i = 0; i < outcount; i++ {
			srcsample = samplefrac >> 8
			samplefrac += fracstep
			if inwidth == 2 {
				sample = int(int16(uint16(data[2*srcsample]) | uint16(data[2*srcsample+1])<<8))
			} else {
				sample = (int(data[srcsample]) - 128) << 8
			}
			if sc.width == 2 {
				sc.data[2*i] = byte(sample)
				sc.data[2*i+1] = byte(sample >> 8)
			} else {
				sc.data[i] = byte(sample >> 8)
			}
		}
	}
}

//=============================================================================

func S_LoadSound(s *sfx_t) *sfxcache_t {
	var namebuffer string
	var data []byte
	var info wavinfo_t
	var len_ int
	var stepscale float32
	var sc *sfxcache_t

	if soundcard == nil {
		return nil
	}

	// see if still in memory
	if s.sfxcache != nil {
		return s.sfxcache
	}

	// load it in
	namebuffer = fmt.Sprintf("sound/%s", s.name)

	data = FS_LoadMallocFile(namebuffer)

	sc = nil

	if data == nil {
		Com_Printf("Couldn't load %s\n", namebuffer)
	} else {
		FMod_CheckModel(namebuffer, data, com_filesize)

		info = GetWavinfo(s.name, data, com_filesize)
		if info.channels != 1 {
			Com_Printf("%s is a stereo sample\n", s.name)
		} else {
			stepscale = float32(info.rate) / float32(soundcard.speed)
			len_ = int(float32(info.samples) / stepscale)

			len_ = len_ * info.width * info.channels

			sc = &sfxcache_t{data: make([]byte, len_)}
			sc.length = info.samples
			sc.loopstart = info.loopstart
			sc.speed = info.rate
			sc.width = info.width
			sc.stereo = info.channels

			ResampleSfx(sc, sc.speed, sc.width, data[info.dataofs:])

			// publish only when fully initialized, the mixer reads this from the audio thread
			S_LockMixer()
			s.sfxcache = sc
			S_UnlockMixer()
		}
	}

	return sc
}

/*
===============================================================================

WAV loading

===============================================================================
*/

// C walks the file with raw pointers; these are offsets into iff_wav, with
// -1 standing for NULL. Reads past the end give zeros where C reads garbage.
var (
	iff_wav    []byte
	data_p     int
	iff_end    int
	last_chunk int
	iff_data   int
)

func wavByte(i int) byte {
	if i >= 0 && i < len(iff_wav) {
		return iff_wav[i]
	}
	return 0
}

func wavTag(i int, name string) bool {
	if i < 0 || i+4 > len(iff_wav) {
		return false
	}
	return string(iff_wav[i:i+4]) == name
}

func GetLittleShort() int16 {
	val := int16(uint16(wavByte(data_p)) | uint16(wavByte(data_p+1))<<8)
	data_p += 2
	return val
}

func GetLittleLong() int {
	val := int(int32(uint32(wavByte(data_p)) | uint32(wavByte(data_p+1))<<8 | uint32(wavByte(data_p+2))<<16 | uint32(wavByte(data_p+3))<<24))
	data_p += 4
	return val
}

func FindNextChunk(name string) {
	var iff_chunk_len int
	var dataleft uint

	for {
		data_p = last_chunk
		dataleft = uint(iff_end - data_p)

		if dataleft < 8 { // didn't find the chunk
			data_p = -1
			return
		}

		data_p += 4
		iff_chunk_len = GetLittleLong()
		dataleft -= 8
		if iff_chunk_len < 0 || uint(iff_chunk_len) > dataleft {
			data_p = -1
			return
		}
		dataleft -= uint(iff_chunk_len)
		data_p -= 8
		last_chunk = data_p + 8 + iff_chunk_len
		if iff_chunk_len&1 != 0 && dataleft != 0 {
			last_chunk++
		}
		if wavTag(data_p, name) {
			return
		}
	}
}

func FindChunk(name string) {
	last_chunk = iff_data
	FindNextChunk(name)
}

func GetWavinfo(name string, wav []byte, wavlength int) wavinfo_t {
	var info wavinfo_t
	var i int
	var format int
	var samples int

	if wav == nil {
		return info
	}

	iff_wav = wav
	iff_data = 0
	iff_end = wavlength

	// find "RIFF" chunk
	FindChunk("RIFF")
	if !(data_p >= 0 && wavTag(data_p+8, "WAVE")) {
		Com_Printf("Missing RIFF/WAVE chunks\n")
		return info
	}

	// get "fmt " chunk
	iff_data = data_p + 12

	FindChunk("fmt ")
	if data_p < 0 {
		Com_Printf("Missing fmt chunk\n")
		return info
	}

	data_p += 8
	format = int(GetLittleShort())
	if format != 1 {
		Com_Printf("Microsoft PCM format only\n")
		return info
	}

	info.channels = int(GetLittleShort())
	info.rate = GetLittleLong()
	data_p += 4 + 2
	info.width = int(GetLittleShort()) / 8

	// get cue chunk
	FindChunk("cue ")
	if data_p >= 0 {
		data_p += 32
		info.loopstart = GetLittleLong()

		// if the next chunk is a LIST chunk, look for a cue length marker
		FindNextChunk("LIST")
		if data_p >= 0 {
			if wavTag(data_p+28, "mark") { // this is not a proper parse, but it works with cooledit...
				data_p += 24
				i = GetLittleLong() // samples in loop
				info.samples = info.loopstart + i
			}
		}
	} else {
		info.loopstart = -1
	}

	// find data chunk
	FindChunk("data")
	if data_p < 0 {
		Com_Printf("Missing data chunk\n")
		return info
	}

	data_p += 4
	samples = GetLittleLong() / info.width

	if info.samples != 0 {
		if samples < info.samples {
			Sys_Error("Sound %s has a bad loop length", name)
		}
	} else {
		info.samples = samples
	}

	info.dataofs = data_p

	return info
}

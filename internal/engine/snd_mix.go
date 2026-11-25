package engine

// snd_mix.c -- portable code to mix sounds for snd_dma.c

import (
	"encoding/binary"
	"unsafe"
)

const PAINTBUFFER_SIZE = 512

var paintbuffer [PAINTBUFFER_SIZE]portable_samplepair_t
var snd_scaletable [32][256]int32
var snd_p []int32 // paintbuffer seen as a flat array, as C casts it
var snd_linear_count int
var snd_vol int32
var snd_out []byte // 16-bit little endian samples
var snd_mixbase int

func Snd_WriteLinearBlastStereo16() {
	var i int
	var val int32

	for i = 0; i < snd_linear_count; i += 2 {
		val = (snd_p[i] * snd_vol) >> 8
		binary.LittleEndian.PutUint16(snd_out[2*i:], uint16(bound(-32768, val, 32767)))
		val = (snd_p[i+1] * snd_vol) >> 8
		binary.LittleEndian.PutUint16(snd_out[2*(i+1):], uint16(bound(-32768, val, 32767)))
	}
}

func Snd_WriteLinearBlastStereo16_SwapStereo() {
	var i int
	var val int32

	for i = 0; i < snd_linear_count; i += 2 {
		val = (snd_p[i+1] * snd_vol) >> 8
		binary.LittleEndian.PutUint16(snd_out[2*i:], uint16(bound(-32768, val, 32767)))
		val = (snd_p[i] * snd_vol) >> 8
		binary.LittleEndian.PutUint16(snd_out[2*(i+1):], uint16(bound(-32768, val, 32767)))
	}
}

func S_TransferStereo16(endtime int) {
	snd_vol = int32(s_volume.value * 256)

	snd_p = unsafe.Slice(&paintbuffer[0].left, 2*PAINTBUFFER_SIZE)
	snd_out = soundcard.buffer[((paintedtime-snd_mixbase)<<1)*2:]
	snd_linear_count = (endtime - paintedtime) << 1

	if s_swapstereo.value != 0 {
		Snd_WriteLinearBlastStereo16_SwapStereo()
	} else {
		Snd_WriteLinearBlastStereo16()
	}
}

func S_TransferPaintBuffer(endtime int) {
	var out_idx int
	var count int
	var p []int32
	var pi int
	var step int
	var val int32
	var snd_vol int32

	if soundcard.samplebits == 16 && soundcard.channels == 2 {
		S_TransferStereo16(endtime)
		return
	}

	p = unsafe.Slice(&paintbuffer[0].left, 2*PAINTBUFFER_SIZE)
	count = (endtime - paintedtime) * soundcard.channels
	out_idx = (paintedtime - snd_mixbase) * soundcard.channels
	step = 3 - soundcard.channels
	snd_vol = int32(s_volume.value * 256)

	if soundcard.samplebits == 16 {
		out := soundcard.buffer
		for ; count > 0; count-- {
			val = (p[pi] * snd_vol) >> 8
			pi += step
			if val > 0x7fff {
				val = 0x7fff
			} else if val < -0x8000 {
				val = -0x8000
			}
			binary.LittleEndian.PutUint16(out[2*out_idx:], uint16(val))
			out_idx++
		}
	} else if soundcard.samplebits == 8 {
		out := soundcard.buffer
		for ; count > 0; count-- {
			val = (p[pi] * snd_vol) >> 8
			pi += step
			if val > 0x7fff {
				val = 0x7fff
			} else if val < -0x8000 {
				val = -0x8000
			}
			out[out_idx] = byte((val >> 8) + 128)
			out_idx++
		}
	}
}

/*
===============================================================================

CHANNEL MIXING

===============================================================================
*/

func S_PaintChannels(endtime int) {
	var i int
	var end int
	var ch *channel_t
	var sc *sfxcache_t
	var ltime, count int

	for paintedtime < endtime {
		// if paintbuffer is smaller than DMA buffer
		end = endtime
		if endtime-paintedtime > PAINTBUFFER_SIZE {
			end = paintedtime + PAINTBUFFER_SIZE
		}

		// clear the paint buffer
		clear(paintbuffer[:end-paintedtime])

		// paint in the channels.
		for i = 0; i < total_channels; i++ {
			ch = &channels[i]
			if ch.sfx == nil {
				continue
			}
			if ch.leftvol == 0 && ch.rightvol == 0 {
				continue
			}
			// no loading from the audio thread, skip channels without a cache
			sc = ch.sfx.sfxcache
			if sc == nil {
				ch.sfx = nil
				continue
			}

			ltime = paintedtime

			for ltime < end { // paint up to end
				if ch.end < end {
					count = ch.end - ltime
				} else {
					count = end - ltime
				}

				if count > 0 {
					if sc.width == 1 {
						SND_PaintChannelFrom8(ch, sc, count)
					} else {
						SND_PaintChannelFrom16(ch, sc, count)
					}

					ltime += count
				}

				// if at end of loop, restart
				if ltime >= ch.end {
					if sc.loopstart >= 0 {
						ch.pos = sc.loopstart
						ch.end = ltime + sc.length - ch.pos
					} else { // channel just stopped
						ch.sfx = nil
						break
					}
				}
			}
		}

		// transfer out according to DMA format
		S_TransferPaintBuffer(end)
		paintedtime = end
	}
}

func SND_InitScaletable() {
	var i, j int

	for i = 0; i < 32; i++ {
		for j = 0; j < 256; j++ {
			snd_scaletable[i][j] = int32(int8(j)) * int32(i) * 8
		}
	}
}

func SND_PaintChannelFrom8(ch *channel_t, sc *sfxcache_t, count int) {
	var data int
	var lscale, rscale *[256]int32
	var sfx []byte
	var i int

	if ch.leftvol > 255 {
		ch.leftvol = 255
	}
	if ch.rightvol > 255 {
		ch.rightvol = 255
	}

	lscale = &snd_scaletable[ch.leftvol>>3]
	rscale = &snd_scaletable[ch.rightvol>>3]
	sfx = sc.data[ch.pos:]

	for i = 0; i < count; i++ {
		data = int(sfx[i])
		paintbuffer[i].left += lscale[data]
		paintbuffer[i].right += rscale[data]
	}

	ch.pos += count
}

func SND_PaintChannelFrom16(ch *channel_t, sc *sfxcache_t, count int) {
	var data int32
	var left, right int32
	var leftvol, rightvol int32
	var sfx []byte
	var i int

	leftvol = int32(ch.leftvol)
	rightvol = int32(ch.rightvol)
	sfx = sc.data[2*ch.pos:]

	for i = 0; i < count; i++ {
		data = int32(int16(uint16(sfx[2*i]) | uint16(sfx[2*i+1])<<8))
		left = (data * leftvol) >> 8
		right = (data * rightvol) >> 8
		paintbuffer[i].left += left
		paintbuffer[i].right += right
	}

	ch.pos += count
}

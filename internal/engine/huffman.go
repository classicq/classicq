// Copyright (C) 2006-2007 Mark Olsen
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

// huffmantable.h
type huffenctable_s struct {
	len  byte
	code uint16
}

type huffdectable_s struct {
	value byte
	len   byte
}

type hufftables struct {
	huffenctable *[256]huffenctable_s
	huffdectable *[2048]huffdectable_s
}

type HuffContext = hufftables

var hufftables_q3 = hufftables{&huffenctable_q3, &huffdectable_q3}

// huffman.c

func huffencbyte(huffenctable *[256]huffenctable_s, inbyte byte, outbuf []byte, length *uint) {
	l := *length
	o := l / 8
	bitsleft := byte(8 - (l % 8))
	if bitsleft == 8 {
		outbuf[o] = 0
	}

	size := huffenctable[inbyte].len
	code := huffenctable[inbyte].code

	for size != 0 {
		bitstocopy := size
		if bitsleft < size {
			bitstocopy = bitsleft
		}
		// incompressible input runs past the end before the caller falls
		// back to a raw copy; C has slack in its buffers, Go needs the check
		if o < uint(len(outbuf)) {
			outbuf[o] |= bitswaptable[code>>(16-bitsleft)]
		}
		code <<= bitstocopy
		size -= bitstocopy
		o++
		if o < uint(len(outbuf)) {
			outbuf[o] = 0
		}
		l += uint(bitstocopy)
		bitsleft = 8
	}

	*length = l
}

func huffdecbyte(huffdectable *[2048]huffdectable_s, inbuf []byte, length *uint) byte {
	l := *length
	o := l / 8

	index := uint16(bitswaptable[inbuf[o]]) << ((l % 8) + 8)
	index |= uint16(bitswaptable[inbuf[o+1]]) << (l % 8)
	index |= uint16(bitswaptable[inbuf[o+2]]) >> (8 - (l % 8))
	index >>= 16 - 11

	*length += uint(huffdectable[index].len)

	return huffdectable[index].value
}

func Huff_Init(tablecrc uint32) *HuffContext {
	// The first value is from FTE with broken CRC generation
	if tablecrc == 0x5ed5c4e4 || tablecrc == 0x286f2e8d {
		return &hufftables_q3
	}
	return nil
}

// Returns the compressed length written to outbuf, or 0 when outbuf is not
// larger than inbuf.
func Huff_CompressPacket(huffcontext *HuffContext, inbuf []byte, outbuf []byte) uint {
	ht := huffcontext
	inbuflen := uint(len(inbuf))
	outbuflen := uint(len(outbuf))

	if !(outbuflen > inbuflen) {
		return 0
	}

	buffer := outbuf[1:]

	var outlen uint
	for i := uint(0); i < inbuflen && outlen/8 < inbuflen; i++ {
		huffencbyte(ht.huffenctable, inbuf[i], buffer, &outlen)
	}

	if outlen/8 >= inbuflen {
		copy(buffer, inbuf)
		outbuf[0] = 0x80
		return inbuflen + 1
	}

	// Wasting 1 byte, but we must be compatible...
	outbuf[0] = byte(8 - (outlen % 8))

	outlen += 8
	outlen /= 8

	return outlen + 1
}

func Huff_DecompressPacket(huffcontext *HuffContext, inbuf []byte, outbuf []byte) uint {
	ht := huffcontext
	inbuflen := uint(len(inbuf))

	if uint(len(outbuf)) < inbuflen {
		return 0
	}

	if inbuf[0] == 0x80 {
		copy(outbuf, inbuf[1:])
		return inbuflen - 1
	}

	// the decoder peeks up to two bytes past the data, as C does in its
	// larger scratch buffer
	var encmsg [MAX_MSGLEN + 1 + 4]byte
	copy(encmsg[:], inbuf[1:])

	bits := (inbuflen-1)*8 - uint(inbuf[0])

	var outlen uint
	i := 0
	for outlen < bits && i < MAX_MSGLEN && i < len(outbuf) {
		outbuf[i] = huffdecbyte(ht.huffdectable, encmsg[:], &outlen)
		i++
	}

	return uint(i)
}

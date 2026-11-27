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

import (
	"encoding/binary"
	"math"
)

/********************************** WRITING **********************************/

func MSG_WriteChar(sb *sizebuf_t, c int) {
	buf := SZ_GetSpace(sb, 1)
	buf[0] = byte(c)
}

func MSG_WriteByte(sb *sizebuf_t, c int) {
	buf := SZ_GetSpace(sb, 1)
	buf[0] = byte(c)
}

func MSG_WriteShort(sb *sizebuf_t, c int) {
	buf := SZ_GetSpace(sb, 2)
	buf[0] = byte(c & 0xff)
	buf[1] = byte(c >> 8)
}

func MSG_WriteLong(sb *sizebuf_t, c int) {
	buf := SZ_GetSpace(sb, 4)
	buf[0] = byte(c & 0xff)
	buf[1] = byte((c >> 8) & 0xff)
	buf[2] = byte((c >> 16) & 0xff)
	buf[3] = byte(c >> 24)
}

func MSG_WriteFloat(sb *sizebuf_t, f float32) {
	binary.LittleEndian.PutUint32(SZ_GetSpace(sb, 4), math.Float32bits(f))
}

func MSG_WriteString(sb *sizebuf_t, s string) {
	buf := SZ_GetSpace(sb, len(s)+1)
	copy(buf, s)
	buf[len(s)] = 0
}

func MSG_WriteCoord(sb *sizebuf_t, f float32) {
	MSG_WriteShort(sb, int(f*8))
}

func MSG_WriteAngle(sb *sizebuf_t, f float32) {
	MSG_WriteByte(sb, Q_rint(float64(f)*256.0/360.0)&255)
}

func MSG_WriteAngle16(sb *sizebuf_t, f float32) {
	MSG_WriteShort(sb, Q_rint(float64(f)*65536.0/360.0)&65535)
}

func MSG_WriteDeltaUsercmd(buf *sizebuf_t, from, cmd *usercmd_t) {
	// send the movement message
	bits := 0
	if cmd.angles[0] != from.angles[0] {
		bits |= CM_ANGLE1
	}
	if cmd.angles[1] != from.angles[1] {
		bits |= CM_ANGLE2
	}
	if cmd.angles[2] != from.angles[2] {
		bits |= CM_ANGLE3
	}
	if cmd.forwardmove != from.forwardmove {
		bits |= CM_FORWARD
	}
	if cmd.sidemove != from.sidemove {
		bits |= CM_SIDE
	}
	if cmd.upmove != from.upmove {
		bits |= CM_UP
	}
	if cmd.buttons != from.buttons {
		bits |= CM_BUTTONS
	}
	if cmd.impulse != from.impulse {
		bits |= CM_IMPULSE
	}

	MSG_WriteByte(buf, bits)

	if bits&CM_ANGLE1 != 0 {
		MSG_WriteAngle16(buf, cmd.angles[0])
	}
	if bits&CM_ANGLE2 != 0 {
		MSG_WriteAngle16(buf, cmd.angles[1])
	}
	if bits&CM_ANGLE3 != 0 {
		MSG_WriteAngle16(buf, cmd.angles[2])
	}

	if bits&CM_FORWARD != 0 {
		MSG_WriteShort(buf, int(cmd.forwardmove))
	}
	if bits&CM_SIDE != 0 {
		MSG_WriteShort(buf, int(cmd.sidemove))
	}
	if bits&CM_UP != 0 {
		MSG_WriteShort(buf, int(cmd.upmove))
	}

	if bits&CM_BUTTONS != 0 {
		MSG_WriteByte(buf, int(cmd.buttons))
	}
	if bits&CM_IMPULSE != 0 {
		MSG_WriteByte(buf, int(cmd.impulse))
	}
	MSG_WriteByte(buf, int(cmd.msec))
}

// Writes part of a packetentities message.
// Can delta from either a baseline or a previous packet_entity
func MSG_WriteDeltaEntity(from, to *entity_state_t, msg *sizebuf_t, force bool) {
	if to.number <= 0 && to.number >= 512 {
		Sys_Error("MSG_WriteDeltaEntity: Number out of range")
	}

	// send an update
	bits := 0

	for i := 0; i < 3; i++ {
		miss := to.origin[i] - from.origin[i]
		if float64(miss) < -0.1 || float64(miss) > 0.1 {
			bits |= U_ORIGIN1 << i
		}
	}

	if to.angles[0] != from.angles[0] {
		bits |= U_ANGLE1
	}
	if to.angles[1] != from.angles[1] {
		bits |= U_ANGLE2
	}
	if to.angles[2] != from.angles[2] {
		bits |= U_ANGLE3
	}

	if to.colormap != from.colormap {
		bits |= U_COLORMAP
	}
	if to.skinnum != from.skinnum {
		bits |= U_SKIN
	}
	if to.frame != from.frame {
		bits |= U_FRAME
	}
	if to.effects != from.effects {
		bits |= U_EFFECTS
	}
	if to.modelindex != from.modelindex {
		bits |= U_MODEL
	}

	if bits&511 != 0 {
		bits |= U_MOREBITS
	}

	if to.flags&U_SOLID != 0 {
		bits |= U_SOLID
	}

	// write the message
	if bits == 0 && !force {
		return // nothing to send!
	}

	i := to.number | (bits &^ 511)
	MSG_WriteShort(msg, i)

	if bits&U_MOREBITS != 0 {
		MSG_WriteByte(msg, bits&255)
	}
	if bits&U_MODEL != 0 {
		MSG_WriteByte(msg, to.modelindex)
	}
	if bits&U_FRAME != 0 {
		MSG_WriteByte(msg, to.frame)
	}
	if bits&U_COLORMAP != 0 {
		MSG_WriteByte(msg, to.colormap)
	}
	if bits&U_SKIN != 0 {
		MSG_WriteByte(msg, to.skinnum)
	}
	if bits&U_EFFECTS != 0 {
		MSG_WriteByte(msg, to.effects)
	}
	if bits&U_ORIGIN1 != 0 {
		MSG_WriteCoord(msg, to.origin[0])
	}
	if bits&U_ANGLE1 != 0 {
		MSG_WriteAngle(msg, to.angles[0])
	}
	if bits&U_ORIGIN2 != 0 {
		MSG_WriteCoord(msg, to.origin[1])
	}
	if bits&U_ANGLE2 != 0 {
		MSG_WriteAngle(msg, to.angles[1])
	}
	if bits&U_ORIGIN3 != 0 {
		MSG_WriteCoord(msg, to.origin[2])
	}
	if bits&U_ANGLE3 != 0 {
		MSG_WriteAngle(msg, to.angles[2])
	}
}

// Writes a delta update of a packet_entities_t to the message.
func MSG_EmitPacketEntities(from *packet_entities_t, delta_sequence int, to *packet_entities_t, msg *sizebuf_t,
	GetBaseline func(number int) *entity_state_t) {
	var oldmax int

	// this is the frame that we are going to delta update from
	if from != nil {
		oldmax = from.num_entities
		MSG_WriteByte(msg, svc_deltapacketentities)
		MSG_WriteByte(msg, delta_sequence)
	} else {
		oldmax = 0 // no delta update
		MSG_WriteByte(msg, svc_packetentities)
	}

	newindex := 0
	oldindex := 0
	for newindex < to.num_entities || oldindex < oldmax {
		newnum := 9999
		if newindex < to.num_entities {
			newnum = to.entities[newindex].number
		}
		oldnum := 9999
		if oldindex < oldmax {
			oldnum = from.entities[oldindex].number
		}

		if newnum == oldnum {
			// delta update from old position
			MSG_WriteDeltaEntity(&from.entities[oldindex], &to.entities[newindex], msg, false)
			oldindex++
			newindex++
			continue
		}

		if newnum < oldnum {
			// this is a new entity, send it from the baseline
			MSG_WriteDeltaEntity(GetBaseline(newnum), &to.entities[newindex], msg, true)
			newindex++
			continue
		}

		if newnum > oldnum {
			// the old entity isn't present in the new message
			MSG_WriteShort(msg, oldnum|U_REMOVE)
			oldindex++
			continue
		}
	}

	MSG_WriteShort(msg, 0) // end of packetentities
}

/********************************** READING **********************************/

var msg_readcount int
var msg_badread bool // set if a read goes beyond end of message
var msg_message *sizebuf_t

func MSG_BeginReading(message *sizebuf_t) {
	msg_readcount = 0
	msg_badread = false
	msg_message = message
}

func MSG_GetReadCount() int {
	return msg_readcount
}

func MSG_ReadData(destination []byte) bool {
	size := len(destination)
	if msg_readcount+size <= msg_message.cursize {
		copy(destination, msg_message.data[msg_readcount:])
		msg_readcount += size
		return true
	}
	return false
}

// returns -1 and sets msg_badread if no more characters are available
func MSG_ReadChar() int {
	if msg_readcount+1 > msg_message.cursize {
		msg_badread = true
		return -1
	}

	c := int(int8(msg_message.data[msg_readcount]))
	msg_readcount++

	return c
}

func MSG_ReadByte() int {
	if msg_readcount+1 > msg_message.cursize {
		msg_badread = true
		return -1
	}

	c := int(msg_message.data[msg_readcount])
	msg_readcount++

	return c
}

func MSG_ReadShort() int {
	if msg_readcount+2 > msg_message.cursize {
		msg_badread = true
		return -1
	}

	c := int(int16(binary.LittleEndian.Uint16(msg_message.data[msg_readcount:])))
	msg_readcount += 2

	return c
}

func MSG_ReadLong() int {
	if msg_readcount+4 > msg_message.cursize {
		msg_badread = true
		return -1
	}

	c := int(int32(binary.LittleEndian.Uint32(msg_message.data[msg_readcount:])))
	msg_readcount += 4

	return c
}

// C does not check the size here and reads whatever follows in the buffer.
func MSG_ReadFloat() float32 {
	var b [4]byte
	for i := range b {
		if msg_readcount+i < len(msg_message.data) {
			b[i] = msg_message.data[msg_readcount+i]
		}
	}
	msg_readcount += 4
	return math.Float32frombits(binary.LittleEndian.Uint32(b[:]))
}

func MSG_ReadString() string {
	str := make([]byte, 0, 64)
	for len(str) < 2048-1 {
		c := MSG_ReadByte()
		if c == -1 || c == 0 { // msg_badread or end of string
			break
		}
		str = append(str, byte(c))
	}
	return string(str)
}

func MSG_ReadStringLine() string {
	str := make([]byte, 0, 64)
	for len(str) < 2048-1 {
		c := MSG_ReadByte()
		if c == 255 {
			continue
		}
		if c == -1 || c == 0 || c == '\n' {
			break
		}
		str = append(str, byte(c))
	}
	return string(str)
}

func MSG_ReadCoord() float32 {
	if cls.ftexsupported&FTEX_FLOATCOORDS != 0 {
		return MSG_ReadFloat()
	}
	return float32(float64(MSG_ReadShort()) * (1.0 / 8))
}

func MSG_ReadAngle() float32 {
	if cls.ftexsupported&FTEX_FLOATCOORDS != 0 {
		return MSG_ReadAngle16()
	}
	return float32(float64(MSG_ReadChar()) * (360.0 / 256))
}

func MSG_ReadAngle16() float32 {
	return float32(float64(MSG_ReadShort()) * (360.0 / 65536))
}

const CM_MSEC = 1 << 7 // same as CM_ANGLE2

func MSG_ReadDeltaUsercmd(from, move *usercmd_t, protoversion int) {
	*move = *from

	bits := MSG_ReadByte()

	if protoversion == 26 {
		// read current angles
		if bits&CM_ANGLE1 != 0 {
			move.angles[0] = MSG_ReadAngle16()
		}
		move.angles[1] = MSG_ReadAngle16() // always sent
		if bits&CM_ANGLE3 != 0 {
			move.angles[2] = MSG_ReadAngle16()
		}

		// read movement
		if bits&CM_FORWARD != 0 {
			move.forwardmove = int16(MSG_ReadChar() << 3)
		}
		if bits&CM_SIDE != 0 {
			move.sidemove = int16(MSG_ReadChar() << 3)
		}
		if bits&CM_UP != 0 {
			move.upmove = int16(MSG_ReadChar() << 3)
		}
	} else {
		// read current angles
		if bits&CM_ANGLE1 != 0 {
			move.angles[0] = MSG_ReadAngle16()
		}
		if bits&CM_ANGLE2 != 0 {
			move.angles[1] = MSG_ReadAngle16()
		}
		if bits&CM_ANGLE3 != 0 {
			move.angles[2] = MSG_ReadAngle16()
		}

		// read movement
		if bits&CM_FORWARD != 0 {
			move.forwardmove = int16(MSG_ReadShort())
		}
		if bits&CM_SIDE != 0 {
			move.sidemove = int16(MSG_ReadShort())
		}
		if bits&CM_UP != 0 {
			move.upmove = int16(MSG_ReadShort())
		}
	}

	// read buttons
	if bits&CM_BUTTONS != 0 {
		move.buttons = byte(MSG_ReadByte())
	}
	if bits&CM_IMPULSE != 0 {
		move.impulse = byte(MSG_ReadByte())
	}

	// read time to run command
	if protoversion == 26 {
		if bits&CM_MSEC != 0 {
			move.msec = byte(MSG_ReadByte())
		}
	} else {
		move.msec = byte(MSG_ReadByte()) // always sent
	}
}

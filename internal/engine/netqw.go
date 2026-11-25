package engine

import (
	"encoding/binary"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
)

// netqw.c: the connection to the server runs on its own thread, which
// sends the movement frames at a fixed rate, keeps the reliable stream
// going and hands received packets to the client.

const PLPACKETHISTORYCOUNT = 200

const CLAMPTOINTEGERMS = 1

var zerocmd usercmd_t

const (
	state_uninitialised int32 = iota
	state_sendchallenge
	state_sendconnection
	state_connected
)

type ReliableBuffer struct {
	next *ReliableBuffer
	data []byte
}

// unreliable buffer for fire-and-forget commands (no ack, no retransmit)
type UnreliableBuffer struct {
	next *UnreliableBuffer
	data []byte
}

type NetPacket struct {
	clientnext   *NetPacket
	internalnext *NetPacket
	delayuntil   uint64
	usecount     uint
	data         []byte
}

type SendPacket struct {
	// Like above, but for sent packets
	next       *SendPacket
	delayuntil uint64
	data       []byte
}

type netqwFrame struct {
	cmd            usercmd_t
	senttime       float64
	delta_sequence int
}

type NetQW struct {
	error_     int
	qport      uint16
	ftex       uint32
	challenge  int
	resendtime uint64
	packetloss byte
	hoststring string
	userinfo   string
	addr       netaddr
	done       chan struct{} // the thread
	mutex      sync.Mutex
	netdata    *SysNetData
	socket     *SysSocket

	// QW connection state info
	current_outgoing_sequence_number uint32
	last_incoming_sequence_number    uint32

	reliable_buffers_sent    uint32
	reliable_sequence_number uint32

	outgoing_reliable_xor uint32
	incoming_reliable_xor uint32

	replyreceived [PLPACKETHISTORYCOUNT]byte

	huffcontext *HuffContext
	huffcrc     uint32

	// packet scratch space, only touched by the net thread
	sendbuf     [1500]byte
	compressbuf [1500]byte

	internalnetpackethead *NetPacket
	internalnetpackettail *NetPacket
	sendpackethead        *SendPacket
	sendpackettail        *SendPacket

	// Shared between threads
	quit                 atomic.Int32
	state                atomic.Int32
	microsecondsperframe uint32
	lastmovesendtime     uint64
	movetimecounter      uint64
	send_tmove           int
	movement_locked      int
	tmove                [12]byte // 6 bytes of shorts or 3 floats
	forwardspeed         float32
	sidespeed            float32
	upspeed              float32
	oncebuttons          byte
	currentbuttons       byte
	priorityimpulse      byte
	normalimpulse        byte
	lag                  uint32
	lag_ezcheat          int
	reliablebufferhead   *ReliableBuffer
	reliablebuffertail   *ReliableBuffer
	unreliablebufferhead *UnreliableBuffer
	unreliablebuffertail *UnreliableBuffer
	clientnetpackethead  *NetPacket
	clientnetpackettail  *NetPacket
	lastserverpackettime uint64

	frames                            [UPDATE_BACKUP]netqwFrame
	lastsentframe                     uint32
	framestosend                      uint32
	lastcopiedframe                   uint32
	framestocopy                      uint32
	last_received_entity_update_frame int
}

/*****/

func WriteAngle16(b []byte, f float32) {
	v := uint16(Q_rint(float64(f)*65536.0/360.0) & 65535)

	b[0] = byte(v)
	b[1] = byte(v >> 8)
}

func WriteDeltaUsercmd(buf []byte, from, cmd *usercmd_t) uint {
	i := uint(0)

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

	buf[i] = byte(bits)
	i++

	if bits&CM_ANGLE1 != 0 {
		WriteAngle16(buf[i:], cmd.angles[0])
		i += 2
	}
	if bits&CM_ANGLE2 != 0 {
		WriteAngle16(buf[i:], cmd.angles[1])
		i += 2
	}
	if bits&CM_ANGLE3 != 0 {
		WriteAngle16(buf[i:], cmd.angles[2])
		i += 2
	}

	if bits&CM_FORWARD != 0 {
		buf[i] = byte(cmd.forwardmove)
		buf[i+1] = byte(cmd.forwardmove >> 8)
		i += 2
	}
	if bits&CM_SIDE != 0 {
		buf[i] = byte(cmd.sidemove)
		buf[i+1] = byte(cmd.sidemove >> 8)
		i += 2
	}
	if bits&CM_UP != 0 {
		buf[i] = byte(cmd.upmove)
		buf[i+1] = byte(cmd.upmove >> 8)
		i += 2
	}

	if bits&CM_BUTTONS != 0 {
		buf[i] = cmd.buttons
		i++
	}
	if bits&CM_IMPULSE != 0 {
		buf[i] = cmd.impulse
		i++
	}

	buf[i] = cmd.msec
	i++

	return i
}

/*****/

func NetQW_GenerateFrames(netqw *NetQW) {
	var hej usercmd_t

	curtime := Sys_IntTime()

	if netqw.state.Load() != state_connected {
		return
	}

	netqw.mutex.Lock()

	for netqw.lastmovesendtime+uint64(netqw.microsecondsperframe) <= curtime {
		netqw.lastmovesendtime += uint64(netqw.microsecondsperframe)
		netqw.movetimecounter += uint64(netqw.microsecondsperframe)

		hej = usercmd_t{}

		hej.forwardmove = int16(netqw.forwardspeed)
		hej.sidemove = int16(netqw.sidespeed)
		hej.upmove = int16(netqw.upspeed)

		hej.buttons = netqw.oncebuttons | netqw.currentbuttons
		netqw.oncebuttons = 0

		if netqw.priorityimpulse != 0 {
			hej.impulse = netqw.priorityimpulse
			netqw.priorityimpulse = 0
		} else if netqw.normalimpulse != 0 {
			hej.impulse = netqw.normalimpulse
			netqw.normalimpulse = 0
		}

		for i := 0; i < 3; i++ {
			hej.angles[i] = float32(float64(Q_rint(float64(hej.angles[i])*65536.0/360.0)&65535) * (360.0 / 65536.0))
		}

		hej.msec = byte(netqw.movetimecounter / 1000)
		netqw.movetimecounter %= 1000
		Mouse_GetViewAngles(&hej.angles)

		framenum := (netqw.lastsentframe + netqw.framestosend + 1) & UPDATE_MASK

		if netqw.movement_locked != 0 {
			hej.forwardmove = 0
			hej.sidemove = 0
			hej.upmove = 0
		}

		netqw.frames[framenum].cmd = hej
		netqw.frames[framenum].senttime = Sys_DoubleTime()
		netqw.frames[framenum].delta_sequence = -1

		if netqw.last_received_entity_update_frame != -1 && netqw.current_outgoing_sequence_number-uint32(netqw.last_received_entity_update_frame) < UPDATE_BACKUP {
			netqw.frames[netqw.current_outgoing_sequence_number&UPDATE_MASK].delta_sequence = netqw.last_received_entity_update_frame
		}

		netqw.framestosend++
		netqw.framestocopy++
	}

	netqw.mutex.Unlock()
}

func NetQW_Thread_Init(netqw *NetQW) int {
	netqw.netdata = Sys_Net_Init()
	if netqw.netdata != nil {
		b := NET_StringToAdr(netqw.netdata, netqw.hoststring, &netqw.addr)
		if !b {
			Sys_Printf("Unable to parse/resolve hoststring\n")
		} else {
			netqw.socket = Sys_Net_CreateSocket(netqw.netdata, netqw.addr.type_)
			if netqw.socket != nil {
				netqw.resendtime = 0
				netqw.state.Store(state_sendchallenge)
				netqw.current_outgoing_sequence_number = 1
				netqw.last_incoming_sequence_number = 0
				netqw.incoming_reliable_xor = 0
				netqw.outgoing_reliable_xor = 0

				return 1
			}
		}

		Sys_Net_Shutdown(netqw.netdata)
	}

	return 0
}

func NetQW_Thread_Deinit(netqw *NetQW) {
	Sys_Net_DeleteSocket(netqw.netdata, netqw.socket)
	Sys_Net_Shutdown(netqw.netdata)
}

func NetQW_Thread_QueuePacket(netqw *NetQW, buf []byte) {
	sendpacket := &SendPacket{data: append([]byte(nil), buf...)}
	if netqw.lag_ezcheat != 0 {
		sendpacket.delayuntil = Sys_IntTime() + uint64(netqw.lag)
	} else {
		sendpacket.delayuntil = Sys_IntTime() + uint64(netqw.lag/2)
	}

	if netqw.sendpackettail != nil {
		netqw.sendpackettail.next = sendpacket
		netqw.sendpackettail = sendpacket
	} else {
		netqw.sendpackethead = sendpacket
		netqw.sendpackettail = sendpacket
	}
}

func NetQW_Thread_SendPacket(netqw *NetQW, buf []byte, to *netaddr, force int) {
	compressedbuf := &netqw.compressbuf

	buflen := len(buf)
	if buflen >= len(compressedbuf) {
		return
	}

	if netqw.huffcontext != nil {
		copy(compressedbuf[:10], buf[:10])
		i := Huff_CompressPacket(netqw.huffcontext, buf[10:buflen], compressedbuf[10:])
		if i == 0 {
			Com_Printf("Huffman coding failed\n")
		} else {
			if netqw.lag != 0 && force == 0 {
				NetQW_Thread_QueuePacket(netqw, compressedbuf[:i+10])
			} else {
				Sys_Net_Send(netqw.netdata, netqw.socket, compressedbuf[:i+10], &netqw.addr)
			}
		}
	} else {
		if netqw.lag != 0 && force == 0 {
			NetQW_Thread_QueuePacket(netqw, buf)
		} else {
			Sys_Net_Send(netqw.netdata, netqw.socket, buf, &netqw.addr)
		}
	}
}

func NetQW_Thread_Disconnect(netqw *NetQW) {
	var disconnectmessage [10 + 6]byte

	if netqw.state.Load() == state_connected {
		for i := 0; i < 3; i++ {
			binary.LittleEndian.PutUint32(disconnectmessage[0:], netqw.current_outgoing_sequence_number)
			binary.LittleEndian.PutUint32(disconnectmessage[4:], netqw.last_incoming_sequence_number)

			disconnectmessage[8] = byte(netqw.qport)
			disconnectmessage[9] = byte(netqw.qport >> 8)

			disconnectmessage[10] = clc_stringcmd
			copy(disconnectmessage[11:], "drop\x00")

			NetQW_Thread_SendPacket(netqw, disconnectmessage[:16], &netqw.addr, 1)

			netqw.current_outgoing_sequence_number++
		}
	}
}

func NetQW_Thread_HandleReceivedPacket(netqw *NetQW, netpacket *NetPacket) {
	buf := netpacket.data

	sequence_number := uint32(buf[0])
	sequence_number |= uint32(buf[1]) << 8
	sequence_number |= uint32(buf[2]) << 16
	sequence_number |= uint32(buf[3]&0x7f) << 24

	netqw.last_incoming_sequence_number = sequence_number

	if buf[3]&0x80 != 0 {
		netqw.outgoing_reliable_xor ^= 1
	}

	if sequence_number+PLPACKETHISTORYCOUNT > netqw.current_outgoing_sequence_number {
		netqw.replyreceived[sequence_number%PLPACKETHISTORYCOUNT] = 1

		lostpackets := uint32(0)
		j := netqw.current_outgoing_sequence_number
		if j > PLPACKETHISTORYCOUNT {
			j = PLPACKETHISTORYCOUNT
		}

		j -= netqw.current_outgoing_sequence_number - sequence_number + 1

		for i := uint32(0); i < j; i++ {
			if netqw.replyreceived[(sequence_number-i)%PLPACKETHISTORYCOUNT] == 0 {
				lostpackets++
			}
		}

		// svc_chokecount is not taken into account
		netqw.packetloss = byte(((lostpackets * 100) + (PLPACKETHISTORYCOUNT - 1)) / PLPACKETHISTORYCOUNT)
	}

	if netqw.reliable_buffers_sent != 0 && sequence_number >= netqw.reliable_sequence_number {
		// The server should have seen our reliable packet by now...

		if (netqw.incoming_reliable_xor == 0) != (buf[7]&0x80 == 0) {
			// ... but it hasn't :/

			netqw.reliable_buffers_sent = 0
			netqw.incoming_reliable_xor ^= 1
		} else {
			// And it has! Great, free up the buffers then.

			for i := uint32(0); i < netqw.reliable_buffers_sent; i++ {
				reliablebuffer := netqw.reliablebufferhead
				netqw.reliablebufferhead = reliablebuffer.next
			}

			if netqw.reliablebufferhead == nil {
				netqw.reliablebuffertail = nil
			}

			netqw.reliable_buffers_sent = 0
		}
	}
}

func netqwQueueClientPacket(netqw *NetQW, netpacket *NetPacket) {
	netqw.mutex.Lock()
	if netqw.clientnetpackettail != nil {
		netqw.clientnetpackettail.clientnext = netpacket
		netqw.clientnetpackettail = netpacket
	} else {
		netqw.clientnetpackethead = netpacket
		netqw.clientnetpackettail = netpacket
	}
	netqw.mutex.Unlock()
}

func NetQW_Thread_DoReceive(netqw *NetQW) {
	var addr netaddr
	var buf [1500]byte
	var compressedbuf [1500]byte
	var r int

	for {
		if netqw.state.Load() == state_connected && netqw.huffcontext != nil {
			r = Sys_Net_Receive(netqw.netdata, netqw.socket, compressedbuf[:len(compressedbuf)-1], &addr)

			// OOB packets (0xffffffff prefix) skip decompression
			if r >= 4 && compressedbuf[0] == 0xff && compressedbuf[1] == 0xff && compressedbuf[2] == 0xff && compressedbuf[3] == 0xff {
				if r > len(buf) {
					r = len(buf)
				}
				copy(buf[:r], compressedbuf[:r])
			} else {
				copy(buf[:8], compressedbuf[:8])
				if r > 8 {
					r = int(Huff_DecompressPacket(netqw.huffcontext, compressedbuf[8:r], buf[8:len(buf)-1]))
					r += 8
				}
			}
		} else {
			r = Sys_Net_Receive(netqw.netdata, netqw.socket, buf[:len(buf)-1], &addr)
		}

		if r > 0 {
			if !NET_CompareAdr(&addr, &netqw.addr) {
				continue
			}

			netqw.lastserverpackettime = Sys_IntTime()

			if netqw.state.Load() == state_sendchallenge {
				if r < 5 || buf[0] != 255 || buf[1] != 255 || buf[2] != 255 || buf[3] != 255 || buf[4] != S2C_CHALLENGE {
					continue
				}

				len_ := uint(r - 5)

				p := 5
				i := uint(0)
				for buf[p] != 0 && len_ != 0 {
					p++
					i++
					len_--
				}

				if i < 16 {
					// reset wishlist; OR-in per server FTEX advertisement
					ftex_wishlist := netqw.ftex
					netqw.ftex = 0

					netqw.challenge = atoi(string(buf[5 : 5+i]))

					if len_ != 0 {
						p++
						len_--

						for len_ >= 8 {
							extension := uint32(buf[p+3])<<24 | uint32(buf[p+2])<<16 | uint32(buf[p+1])<<8 | uint32(buf[p])
							value := uint32(buf[p+7])<<24 | uint32(buf[p+6])<<16 | uint32(buf[p+5])<<8 | uint32(buf[p+4])

							if extension == QW_PROTOEXT_HUFF {
								Com_Printf("Server supports Huffman compression\n")

								netqw.huffcontext = Huff_Init(value)
								if netqw.huffcontext == nil {
									Com_Printf("Unable to initialise Huffman coding\n")
								} else {
									netqw.huffcrc = value
								}
							} else if extension == QW_PROTOEXT_FTEX {
								netqw.ftex |= ftex_wishlist & value
							} else if extension == QW_PROTOEXT_FTE2 || extension == QW_PROTOEXT_MVD1 {
							} else {
								Com_Printf("Unknown protocol extension: %08x\n", extension)
							}

							p += 8
							len_ -= 8
						}
					}

					netqw.state.Store(state_sendconnection)
					netqw.resendtime = 0
				}
			} else if netqw.state.Load() == state_sendconnection {
				if r < 5 || buf[0] != 255 || buf[1] != 255 || buf[2] != 255 || buf[3] != 255 {
					continue
				}

				if buf[4] == A2C_PRINT {
					len_ := r - 5
					p := 5
					for buf[p] != 0 && len_ != 0 {
						p++
						len_--
					}

					buf[p] = 0

					Com_Printf("Connectionless message: %s\n", string(buf[5:p]))
				} else if buf[4] == S2C_CONNECTION {
					Com_Printf("Connected.\n")

					buf[0] = clc_stringcmd
					copy(buf[1:], "new\x00")
					NetQW_AppendReliableBuffer(netqw, buf[:5])

					netqw.lastmovesendtime = Sys_IntTime() - uint64(netqw.microsecondsperframe)
					netqw.movetimecounter = 0
					netqw.state.Store(state_connected)
				} else {
					Sys_Printf("Unknown packet from server: %d\n", buf[4])
				}
			} else {
				if r < 4 {
					continue
				}

				// OOB packets (0xffffffff prefix) bypass sequenced channel
				if buf[0] == 0xff && buf[1] == 0xff && buf[2] == 0xff && buf[3] == 0xff {
					netpacket := &NetPacket{usecount: 1, data: append([]byte(nil), buf[:r]...)}
					netqwQueueClientPacket(netqw, netpacket)
					continue
				}

				if r < 8 {
					continue
				}

				sequence_number := uint32(buf[0])
				sequence_number |= uint32(buf[1]) << 8
				sequence_number |= uint32(buf[2]) << 16
				sequence_number |= uint32(buf[3]&0x7f) << 24

				if sequence_number <= netqw.last_incoming_sequence_number {
					continue
				}

				sequence_number = uint32(buf[4])
				sequence_number |= uint32(buf[5]) << 8
				sequence_number |= uint32(buf[6]) << 16
				sequence_number |= uint32(buf[7]&0x7f) << 24

				if sequence_number >= netqw.current_outgoing_sequence_number {
					continue
				}

				netpacket := &NetPacket{usecount: 1, data: append([]byte(nil), buf[:r]...)}
				if netqw.lag != 0 && netqw.lag_ezcheat == 0 {
					netpacket.delayuntil = Sys_IntTime() + uint64(netqw.lag/2)
				} else {
					netpacket.delayuntil = 0
				}

				if netpacket.delayuntil == 0 {
					NetQW_Thread_HandleReceivedPacket(netqw, netpacket)
				} else {
					if netqw.internalnetpackettail != nil {
						netqw.internalnetpackettail.internalnext = netpacket
						netqw.internalnetpackettail = netpacket
					} else {
						netqw.internalnetpackethead = netpacket
						netqw.internalnetpackettail = netpacket
					}

					netpacket.usecount++
				}

				netqwQueueClientPacket(netqw, netpacket)
			}
		}

		if r <= 0 {
			break
		}
	}
}

func NetQW_Thread_DoSend(netqw *NetQW) {
	const getchallenge = "\xff\xff\xff\xff" + "getchallenge\n"
	buf := &netqw.sendbuf

	curtime := Sys_IntTime()

	// Just to avoid catastrophy with buggy Sys_IntTime() implementations
	if netqw.lastmovesendtime+1000000 < curtime {
		netqw.lastmovesendtime = curtime
	}

	if netqw.state.Load() == state_connected {
		NetQW_GenerateFrames(netqw)

		for {
			if netqw.framestosend == 0 {
				break
			}

			i := uint(0)

			binary.LittleEndian.PutUint32(buf[0:], netqw.current_outgoing_sequence_number)
			binary.LittleEndian.PutUint32(buf[4:], netqw.last_incoming_sequence_number)
			i = 8

			buf[7] |= byte(netqw.outgoing_reliable_xor << 7)

			buf[i] = byte(netqw.qport)
			buf[i+1] = byte(netqw.qport >> 8)
			i += 2

			buf[i] = clc_move
			buf[i+1] = 0
			buf[i+2] = netqw.packetloss
			i += 3

			i += WriteDeltaUsercmd(buf[i:], &zerocmd, &netqw.frames[(netqw.current_outgoing_sequence_number-2)&UPDATE_MASK].cmd)
			i += WriteDeltaUsercmd(buf[i:], &netqw.frames[(netqw.current_outgoing_sequence_number-2)&UPDATE_MASK].cmd, &netqw.frames[(netqw.current_outgoing_sequence_number-1)&UPDATE_MASK].cmd)
			i += WriteDeltaUsercmd(buf[i:], &netqw.frames[(netqw.current_outgoing_sequence_number-1)&UPDATE_MASK].cmd, &netqw.frames[netqw.current_outgoing_sequence_number&UPDATE_MASK].cmd)

			buf[11] = COM_BlockSequenceCRCByte(buf[12:], int(i-12), int(netqw.current_outgoing_sequence_number))

			netqw.mutex.Lock()
			if netqw.frames[netqw.current_outgoing_sequence_number&UPDATE_MASK].delta_sequence != -1 {
				buf[i] = clc_delta
				buf[i+1] = byte(netqw.frames[netqw.current_outgoing_sequence_number&UPDATE_MASK].delta_sequence)
				i += 2
			}

			netqw.mutex.Unlock()

			if netqw.send_tmove != 0 {
				buf[i] = clc_tmove
				i++
				if cls.ftexsupported&FTEX_FLOATCOORDS != 0 {
					copy(buf[i:], netqw.tmove[:12])
					i += 12
				} else {
					copy(buf[i:], netqw.tmove[:6])
					i += 6
				}
				netqw.send_tmove = 0
			}

			if netqw.reliable_buffers_sent == 0 {
				// Now append as many reliable buffers as we can fit
				netqw.mutex.Lock()

				j := uint32(0)
				reliablebuffer := netqw.reliablebufferhead
				for reliablebuffer != nil && i+uint(len(reliablebuffer.data)) < 1450 {
					copy(buf[i:], reliablebuffer.data)
					i += uint(len(reliablebuffer.data))
					reliablebuffer = reliablebuffer.next
					j++
				}

				netqw.mutex.Unlock()

				if j != 0 {
					netqw.reliable_buffers_sent = j
					netqw.reliable_sequence_number = netqw.current_outgoing_sequence_number
					netqw.incoming_reliable_xor ^= 1

					buf[3] |= 0x80
				}
			}

			// Drain unreliable buffer (no ack gate)
			{
				netqw.mutex.Lock()

				for ub := netqw.unreliablebufferhead; ub != nil && i+uint(len(ub.data)) < 1450; ub = netqw.unreliablebufferhead {
					copy(buf[i:], ub.data)
					i += uint(len(ub.data))
					netqw.unreliablebufferhead = ub.next
				}

				if netqw.unreliablebufferhead == nil {
					netqw.unreliablebuffertail = nil
				}

				netqw.mutex.Unlock()
			}

			NetQW_Thread_SendPacket(netqw, buf[:i], &netqw.addr, 0)

			netqw.replyreceived[netqw.current_outgoing_sequence_number%PLPACKETHISTORYCOUNT] = 0

			netqw.mutex.Lock()
			netqw.lastsentframe++
			netqw.framestosend--
			netqw.current_outgoing_sequence_number++
			netqw.mutex.Unlock()
		}
	} else {
		if netqw.resendtime <= curtime {
			if netqw.state.Load() == state_sendchallenge {
				Sys_Net_Send(netqw.netdata, netqw.socket, []byte(getchallenge), &netqw.addr)
			} else {
				s := fmt.Sprintf("\xff\xff\xff\xff"+"connect %d %d %d \"%s\"\n", PROTOCOL_VERSION, netqw.qport, netqw.challenge, netqw.userinfo)

				if netqw.huffcontext != nil {
					s += fmt.Sprintf("0x%08x 0x%08x\n", QW_PROTOEXT_HUFF, netqw.huffcrc)
				}

				if netqw.ftex != 0 {
					s += fmt.Sprintf("0x%08x 0x%08x\n", QW_PROTOEXT_FTEX, netqw.ftex)
				}

				if len(s) < len(buf) {
					Sys_Net_Send(netqw.netdata, netqw.socket, []byte(s), &netqw.addr)
				}
			}

			netqw.resendtime = curtime + 3000000
		}
	}
}

func NetQW_Thread(netqw *NetQW) {
	defer close(netqw.done)

	r := NetQW_Thread_Init(netqw)
	if r == 0 {
		netqw.error_ = 1
	} else {
		for netqw.quit.Load() == 0 {
			var waittime uint

			curtime := Sys_IntTime()

			if netqw.state.Load() == state_connected {
				netqw.mutex.Lock()

				if netqw.lastmovesendtime+uint64(netqw.microsecondsperframe) > curtime {
					waittime = uint((netqw.lastmovesendtime + uint64(netqw.microsecondsperframe)) - curtime)
				} else {
					waittime = 0
				}

				netqw.mutex.Unlock()
			} else {
				if netqw.resendtime > curtime {
					waittime = uint(netqw.resendtime - curtime)
				} else {
					waittime = 0
				}
			}

			// Mmm, sanity checks...
			if netpacket := netqw.internalnetpackethead; netpacket != nil && netpacket.delayuntil > curtime+1000000 {
				for ; netpacket != nil; netpacket = netpacket.internalnext {
					netpacket.delayuntil = curtime
				}
			}

			for netpacket := netqw.internalnetpackethead; netpacket != nil && netpacket.delayuntil <= curtime; netpacket = netqw.internalnetpackethead {
				NetQW_Thread_HandleReceivedPacket(netqw, netpacket)

				netqw.internalnetpackethead = netpacket.internalnext
				if netqw.internalnetpackethead == nil {
					netqw.internalnetpackettail = nil
				}

				netqw.mutex.Lock()
				netpacket.usecount--
				netqw.mutex.Unlock()
			}

			if sendpacket := netqw.sendpackethead; sendpacket != nil && sendpacket.delayuntil > curtime+1000000 {
				for ; sendpacket != nil; sendpacket = sendpacket.next {
					sendpacket.delayuntil = curtime
				}
			}

			for sendpacket := netqw.sendpackethead; sendpacket != nil && sendpacket.delayuntil <= curtime; sendpacket = netqw.sendpackethead {
				Sys_Net_Send(netqw.netdata, netqw.socket, sendpacket.data, &netqw.addr)

				netqw.sendpackethead = sendpacket.next
				if netqw.sendpackethead == nil {
					netqw.sendpackettail = nil
				}
			}

			if netpacket := netqw.internalnetpackethead; netpacket != nil {
				if uint(netpacket.delayuntil-curtime) < waittime {
					waittime = uint(netpacket.delayuntil - curtime)
				}
			}

			if sendpacket := netqw.sendpackethead; sendpacket != nil {
				if uint(sendpacket.delayuntil-curtime) < waittime {
					waittime = uint(sendpacket.delayuntil - curtime)
				}
			}

			if waittime != 0 {
				// Because we don't quit from signalling, but polling, and
				// the latency would be too high otherwise
				if waittime > 50000 {
					waittime = 50000
				}

				Sys_Net_Wait(netqw.netdata, netqw.socket, waittime)
			}

			NetQW_Thread_DoReceive(netqw)
			NetQW_Thread_DoSend(netqw)
		}

		NetQW_Thread_Disconnect(netqw)

		NetQW_Thread_Deinit(netqw)
	}
}

// C falls back to running the connection on the main thread when it cannot
// start one; a goroutine always starts.
func NetQW_Create(hoststring, userinfo string, qport uint16, ftex uint32) *NetQW {
	netqw := &NetQW{}

	netqw.qport = qport
	netqw.ftex = ftex
	netqw.state.Store(state_uninitialised)
	netqw.last_received_entity_update_frame = -1

	NetQW_SetFPS(netqw, 72)

	netqw.hoststring = hoststring
	netqw.userinfo = userinfo

	netqw.done = make(chan struct{})
	go NetQW_Thread(netqw)

	return netqw
}

func NetQW_Delete(netqw *NetQW) {
	// Yup, should use proper signalling
	netqw.quit.Store(1)

	<-netqw.done

	netqw.reliablebufferhead = nil
	netqw.unreliablebufferhead = nil
	netqw.clientnetpackethead = nil
	netqw.internalnetpackethead = nil
}

func NetQW_Receive(netqw *NetQW) {
}

func NetQW_Send(netqw *NetQW) {
}

func NetQW_SetFPS(netqw *NetQW, fps uint) {
	if fps != 0 {
		netqw.mutex.Lock()

		netqw.microsecondsperframe = uint32(1000000 / fps)
		if netqw.microsecondsperframe < 1000 {
			netqw.microsecondsperframe = 1000
		}

		if CLAMPTOINTEGERMS != 0 {
			netqw.microsecondsperframe += 999
			netqw.microsecondsperframe -= netqw.microsecondsperframe % 1000
		}

		netqw.mutex.Unlock()
	}
}

func NetQW_GetFrameTime(netqw *NetQW) uint64 {
	return uint64(netqw.microsecondsperframe)
}

func NetQW_AppendReliableBuffer(netqw *NetQW, buffer []byte) int {
	reliablebuffer := &ReliableBuffer{data: append([]byte(nil), buffer...)}

	netqw.mutex.Lock()

	if netqw.reliablebuffertail != nil {
		netqw.reliablebuffertail.next = reliablebuffer
		netqw.reliablebuffertail = reliablebuffer
	} else {
		netqw.reliablebufferhead = reliablebuffer
		netqw.reliablebuffertail = reliablebuffer
	}

	netqw.mutex.Unlock()

	return 1
}

func NetQW_AppendUnreliableBuffer(netqw *NetQW, buffer []byte) int {
	ub := &UnreliableBuffer{data: append([]byte(nil), buffer...)}

	netqw.mutex.Lock()

	if netqw.unreliablebuffertail != nil {
		netqw.unreliablebuffertail.next = ub
		netqw.unreliablebuffertail = ub
	} else {
		netqw.unreliablebufferhead = ub
		netqw.unreliablebuffertail = ub
	}

	netqw.mutex.Unlock()

	return 1
}

func NetQW_GetPacketLength(netqw *NetQW) uint {
	var ret uint

	netqw.mutex.Lock()

	if netqw.clientnetpackethead != nil && netqw.clientnetpackethead.delayuntil <= Sys_IntTime() {
		ret = uint(len(netqw.clientnetpackethead.data))
	} else {
		ret = 0
	}

	netqw.mutex.Unlock()

	return ret
}

func NetQW_GetPacketData(netqw *NetQW) []byte {
	var ret []byte

	netqw.mutex.Lock()

	if netqw.clientnetpackethead != nil && netqw.clientnetpackethead.delayuntil <= Sys_IntTime() {
		ret = netqw.clientnetpackethead.data
	} else {
		ret = nil
	}

	netqw.mutex.Unlock()

	return ret
}

func NetQW_FreePacket(netqw *NetQW) {
	netqw.mutex.Lock()

	netpacket := netqw.clientnetpackethead
	netqw.clientnetpackethead = netpacket.clientnext
	netpacket.usecount--
	if netqw.clientnetpackethead == nil {
		netqw.clientnetpackettail = nil
	}

	netqw.mutex.Unlock()
}

func NetQW_CopyFrames(netqw *NetQW, frames *[UPDATE_BACKUP]frame_t, newseqnr *int, startframe, numframes *uint) {
	netqw.mutex.Lock()

	framecount := uint(0)
	if startframe != nil {
		*startframe = uint((netqw.lastcopiedframe + 1) & UPDATE_MASK)
	}

	for netqw.framestocopy != 0 {
		netqw.lastcopiedframe = (netqw.lastcopiedframe + 1) & UPDATE_MASK
		frames[netqw.lastcopiedframe].cmd = netqw.frames[netqw.lastcopiedframe].cmd
		frames[netqw.lastcopiedframe].senttime = netqw.frames[netqw.lastcopiedframe].senttime
		frames[netqw.lastcopiedframe].delta_sequence = netqw.frames[netqw.lastcopiedframe].delta_sequence

		netqw.framestocopy--

		framecount++
	}

	if newseqnr != nil {
		*newseqnr = int(netqw.current_outgoing_sequence_number + netqw.framestosend)
	}
	if numframes != nil {
		*numframes = framecount
	}

	netqw.mutex.Unlock()
}

func NetQW_SetDeltaPoint(netqw *NetQW, delta_sequence_number int) {
	netqw.mutex.Lock()

	netqw.last_received_entity_update_frame = delta_sequence_number

	netqw.mutex.Unlock()
}

func NetQW_SetTeleport(netqw *NetQW, position vec3_t) {
	netqw.mutex.Lock()

	netqw.send_tmove = 1

	if cls.ftexsupported&FTEX_FLOATCOORDS != 0 {
		binary.LittleEndian.PutUint32(netqw.tmove[0:], math.Float32bits(LittleFloat(position[0])))
		binary.LittleEndian.PutUint32(netqw.tmove[4:], math.Float32bits(LittleFloat(position[1])))
		binary.LittleEndian.PutUint32(netqw.tmove[8:], math.Float32bits(LittleFloat(position[2])))
	} else {
		temp := uint32(int32(position[0] * 8))
		netqw.tmove[0] = byte(temp)
		netqw.tmove[1] = byte(temp >> 8)

		temp = uint32(int32(position[1] * 8))
		netqw.tmove[2] = byte(temp)
		netqw.tmove[3] = byte(temp >> 8)

		temp = uint32(int32(position[2] * 8))
		netqw.tmove[4] = byte(temp)
		netqw.tmove[5] = byte(temp >> 8)
	}

	netqw.mutex.Unlock()
}

func NetQW_LockMovement(netqw *NetQW) {
	netqw.movement_locked = 1
}

func NetQW_UnlockMovement(netqw *NetQW) {
	netqw.movement_locked = 0
}

func NetQW_SetLag(netqw *NetQW, microseconds uint) {
	if microseconds > 500000 {
		microseconds = 500000
	}

	netqw.lag = uint32(microseconds)
}

func NetQW_SetLagEzcheat(netqw *NetQW, enabled int) {
	netqw.lag_ezcheat = enabled
}

func NetQW_GetTimeSinceLastPacketFromServer(netqw *NetQW) uint64 {
	return Sys_IntTime() - netqw.lastserverpackettime
}

func NetQW_GetExtensions(netqw *NetQW, ftex *uint32) bool {
	if netqw.state.Load() < state_sendconnection {
		return false
	}

	*ftex = netqw.ftex

	return true
}

func NetQW_SetForwardSpeed(netqw *NetQW, value float32) {
	netqw.forwardspeed = value
}

func NetQW_SetSideSpeed(netqw *NetQW, value float32) {
	netqw.sidespeed = value
}

func NetQW_SetUpSpeed(netqw *NetQW, value float32) {
	netqw.upspeed = value
}

func NetQW_ButtonDown(netqw *NetQW, button, impulse int) uint {
	if button < 0 || button > 2 {
		return 0
	}

	netqw.mutex.Lock()
	if impulse != 0 {
		netqw.priorityimpulse = byte(impulse)
	}

	netqw.oncebuttons |= 1 << button

	ret := uint(netqw.current_outgoing_sequence_number)

	netqw.currentbuttons |= 1 << button

	netqw.mutex.Unlock()

	return ret
}

func NetQW_ButtonUp(netqw *NetQW, button int) {
	if button < 0 || button > 2 {
		return
	}

	netqw.currentbuttons &^= 1 << button
}

func NetQW_SetImpulse(netqw *NetQW, impulse int) {
	netqw.mutex.Lock()

	netqw.normalimpulse = byte(impulse)

	netqw.mutex.Unlock()
}

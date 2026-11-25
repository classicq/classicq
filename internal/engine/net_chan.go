package engine

import "encoding/binary"

// sys_net.h

type netaddrtype int

const (
	NA_LOOPBACK netaddrtype = iota // Only used internally
	NA_IPV4
	NA_IPV6
	NA_NUMTYPES // Not a real type
)

type netaddr_ipv4 struct {
	address [4]byte
	port    uint16
}

type netaddr_ipv6 struct {
	address [16]byte
	port    uint16
}

// The C union holds one of the two; here both are present and type_ says
// which one counts.
type netaddr struct {
	type_ netaddrtype
	ipv4  netaddr_ipv4
	ipv6  netaddr_ipv6
}

// net.h

type netsrc int

const (
	NS_CLIENT netsrc = iota
	NS_SERVER
)

// net_chan.h

const (
	OLD_AVG    = 0.99 // total = oldtotal*OLD_AVG + new*(1-OLD_AVG)
	MAX_LATENT = 32
)

type netchan_t struct {
	fatal_error bool

	sock netsrc

	dropped int // between last packet and previous

	last_received float32 // for timeouts

	// the statistics are cleared at each client begin, because
	// the server connecting process gives a bogus picture of the data
	frame_latency float32 // rolling average
	frame_rate    float32

	drop_count int // dropped packets, cleared each level
	good_count int // cleared each level

	remote_address netaddr
	qport          int

	// bandwidth estimator
	cleartime float64 // if curtime > nc->cleartime, free to go
	rate      float64 // seconds / byte

	// sequencing variables
	incoming_sequence              int
	incoming_acknowledged          int
	incoming_reliable_acknowledged int // single bit

	incoming_reliable_sequence int // single bit, maintained local

	outgoing_sequence      int
	reliable_sequence      int // single bit
	last_reliable_sequence int // sequence number of last send

	// reliable staging and holding areas
	message     sizebuf_t // writing buffer to send to server
	message_buf [MAX_MSGLEN]byte

	reliable_length int
	reliable_buf    [MAX_MSGLEN]byte // unacked reliable message

	// time and size data to calculate bandwidth
	outgoing_size [MAX_LATENT]int
	outgoing_time [MAX_LATENT]float64

	// Anything below here is not cleared by Netchan_Setup
	huffcontext *HuffContext
}

// net_chan.c

const PACKET_HEADER = 8

/*
packet header
-------------
31	sequence
1	does this message contain a reliable payload
31	acknowledge sequence
1	acknowledge receipt of even/odd message
16  qport

The remote connection never knows if it missed a reliable message, the
local side detects that it has been dropped by seeing a sequence acknowledge
higher than the last reliable sequence, but without the correct even/odd
bit for the reliable set.

If the sender notices that a reliable message has been dropped, it will be
retransmitted. It will not be retransmitted again until a message after
the retransmit has been acknowledged and the reliable still failed to get there.

if the sequence number is -1, the packet should be handled without a netcon

The qport field is a workaround for bad address translating routers that
sometimes remap the client's source port on a packet during gameplay.
*/

var showpackets = cvar_t{name: "showpackets", string: "0"}
var showdrop = cvar_t{name: "showdrop", string: "0"}
var qport = cvar_t{name: "qport", string: "0"}

func Huff_CompressPacketSizebuf(huffcontext *HuffContext, msg *sizebuf_t, offset int) {
	var buf [MAX_MSGLEN + 1]byte

	if offset > msg.cursize {
		return
	}

	newlen := int(Huff_CompressPacket(huffcontext, msg.data[offset:msg.cursize], buf[:]))
	if newlen+offset > msg.maxsize {
		return
	}

	copy(msg.data[offset:], buf[:newlen])
	msg.cursize = newlen + offset
}

func Huff_DecompressPacketSizebuf(huffcontext *HuffContext, msg *sizebuf_t, offset int) {
	var buf [MAX_MSGLEN + 1]byte

	if msg.cursize > len(buf) || msg.cursize < offset+1 {
		return
	}

	n := copy(buf[:], msg.data[offset:msg.cursize])
	msg.cursize = int(Huff_DecompressPacket(huffcontext, buf[:n], msg.data[offset:msg.maxsize])) + offset
}

func Netchan_CvarInit() {
	var b [2]byte
	Sys_RandomBytes(b[:])
	port := binary.LittleEndian.Uint16(b[:])

	Cvar_SetCurrentGroup(CVAR_GROUP_SCREEN)
	Cvar_Register(&showpackets)
	Cvar_Register(&showdrop)
	Cvar_SetCurrentGroup(CVAR_GROUP_NO_GROUP)
	Cvar_Register(&qport)
	Cvar_ResetCurrentGroup()

	Cvar_SetValue(&qport, float32(port))
}

// Sends an out-of-band datagram
func Netchan_OutOfBand(sock netsrc, adr netaddr, data []byte) {
	var send sizebuf_t
	var send_buf [MAX_MSGLEN + PACKET_HEADER]byte

	// write the packet header
	SZ_Init(&send, send_buf[:], len(send_buf))

	MSG_WriteLong(&send, -1) // -1 sequence means out of band
	SZ_Write(&send, data)

	// send the datagram
	// zoid, no input in demo playback mode
	if !cls.demoplayback {
		NET_SendPacket(sock, send.data[:send.cursize], &adr)
	}
}

// Sends a text message in an out-of-band datagram
func Netchan_OutOfBandPrint(sock netsrc, adr netaddr, format string, a ...any) {
	str := strlcpy(va(format, a...), 8192)
	Netchan_OutOfBand(sock, adr, []byte(str))
}

// called to open a channel to a remote system
func Netchan_Setup(sock netsrc, chann *netchan_t, adr netaddr, qport int) {
	huffcontext := chann.huffcontext
	*chann = netchan_t{}
	chann.huffcontext = huffcontext

	chann.sock = sock
	chann.remote_address = adr
	chann.qport = qport
	chann.last_received = float32(curtime)

	SZ_Init(&chann.message, chann.message_buf[:], len(chann.message_buf))
	chann.message.allowoverflow = true

	chann.rate = 1.0 / 2500
}

const MAX_BACKUP = 200

// Returns true if the bandwidth choke isn't active
func Netchan_CanPacket(chann *netchan_t) bool {
	if chann.remote_address.type_ == NA_LOOPBACK {
		return true // unlimited bandwidth for local client
	}

	if chann.cleartime < curtime+MAX_BACKUP*chann.rate {
		return true
	}
	return false
}

// Returns true if the bandwidth choke isn't
func Netchan_CanReliable(chann *netchan_t) bool {
	if chann.reliable_length != 0 {
		return false // waiting for ack
	}
	return Netchan_CanPacket(chann)
}

var netchan_last_error_time float64

// tries to send an unreliable message to a connection, and handles the
// transmition / retransmition of the reliable messages.
// A 0 length will still generate a packet and deal with the reliable messages.
func Netchan_Transmit(chann *netchan_t, data []byte) {
	var send sizebuf_t
	var send_buf [MAX_MSGLEN + PACKET_HEADER]byte

	if chann.message.overflowed { // check for message overflow
		current_time := Sys_DoubleTime()

		chann.fatal_error = true // FIXME: THIS DOES NOTHING

		if netchan_last_error_time-current_time > 5 || developer.value != 0 {
			Com_Printf("%s:Outgoing message overflow\n", NET_AdrToString(&chann.remote_address))
			netchan_last_error_time = current_time
		}
		return
	}

	// if the remote side dropped the last reliable message, resend it
	send_reliable := false

	if chann.incoming_acknowledged > chann.last_reliable_sequence && chann.incoming_reliable_acknowledged != chann.reliable_sequence {
		send_reliable = true
	}

	// if the reliable transmit buffer is empty, copy the current message out
	if chann.reliable_length == 0 && chann.message.cursize != 0 {
		copy(chann.reliable_buf[:], chann.message_buf[:chann.message.cursize])
		chann.reliable_length = chann.message.cursize
		chann.message.cursize = 0
		chann.reliable_sequence ^= 1
		send_reliable = true
	}

	// write the packet header
	SZ_Init(&send, send_buf[:], len(send_buf))

	w1 := uint32(chann.outgoing_sequence) | uint32(b2i(send_reliable))<<31
	w2 := uint32(chann.incoming_sequence) | uint32(chann.incoming_reliable_sequence)<<31

	chann.outgoing_sequence++

	MSG_WriteLong(&send, int(int32(w1)))
	MSG_WriteLong(&send, int(int32(w2)))

	// send the qport if we are a client
	if chann.sock == NS_CLIENT {
		MSG_WriteShort(&send, chann.qport)
	}

	// copy the reliable message to the packet first
	if send_reliable {
		SZ_Write(&send, chann.reliable_buf[:chann.reliable_length])
		chann.last_reliable_sequence = chann.outgoing_sequence
	}

	// add the unreliable part if space is available
	if send.maxsize-send.cursize >= len(data) {
		SZ_Write(&send, data)
	}

	// send the datagram
	i := chann.outgoing_sequence & (MAX_LATENT - 1)
	chann.outgoing_size[i] = send.cursize
	chann.outgoing_time[i] = curtime

	// zoid, no input in demo playback mode
	if !cls.demoplayback {
		if cls.netchan.huffcontext != nil {
			offset := 8
			if chann.sock == NS_CLIENT {
				offset = 10
			}
			Huff_CompressPacketSizebuf(chann.huffcontext, &send, offset)
		}
		NET_SendPacket(chann.sock, send.data[:send.cursize], &chann.remote_address)
	}

	if chann.cleartime < curtime {
		chann.cleartime = curtime + float64(send.cursize)*chann.rate
	} else {
		chann.cleartime += float64(send.cursize) * chann.rate
	}

	if showpackets.value != 0 {
		Com_Printf("--> s=%d(%d) a=%d(%d) %d\n",
			chann.outgoing_sequence,
			b2i(send_reliable),
			chann.incoming_sequence,
			chann.incoming_reliable_sequence,
			send.cursize)
	}
}

// called when the current net_message is from remote_address
// modifies net_message so that it points to the packet payload
func Netchan_Process(chann *netchan_t, message *sizebuf_t) bool {
	// get sequence numbers
	MSG_BeginReading(message)
	sequence := uint32(MSG_ReadLong())
	sequence_ack := uint32(MSG_ReadLong())

	// read the qport if we are a server
	if chann.sock == NS_SERVER {
		MSG_ReadShort()
	}

	reliable_message := sequence >> 31
	reliable_ack := sequence_ack >> 31

	sequence &^= 1 << 31
	sequence_ack &^= 1 << 31

	if showpackets.value != 0 {
		Com_Printf("<-- s=%d(%d) a=%d(%d) %d\n",
			sequence,
			reliable_message,
			sequence_ack,
			reliable_ack,
			message.cursize)
	}

	// discard stale or duplicated packets
	if sequence <= uint32(chann.incoming_sequence) {
		if showdrop.value != 0 {
			Com_Printf("%s:Out of order packet %d at %d\n",
				NET_AdrToString(&chann.remote_address),
				sequence,
				chann.incoming_sequence)
		}
		return false
	}

	// dropped packets don't keep the message from being used
	chann.dropped = int(int32(sequence - uint32(chann.incoming_sequence+1)))
	if chann.dropped > 0 {
		chann.drop_count += 1

		if showdrop.value != 0 {
			Com_Printf("%s:Dropped %d packets at %d\n",
				NET_AdrToString(&chann.remote_address),
				chann.dropped,
				sequence)
		}
	}

	// if the current outgoing reliable message has been acknowledged
	// clear the buffer to make way for the next
	if reliable_ack == uint32(chann.reliable_sequence) {
		chann.reliable_length = 0 // it has been received
	}

	// if this message contains a reliable message, bump incoming_reliable_sequence
	chann.incoming_sequence = int(sequence)
	chann.incoming_acknowledged = int(sequence_ack)
	chann.incoming_reliable_acknowledged = int(reliable_ack)
	if reliable_message != 0 {
		chann.incoming_reliable_sequence ^= 1
	}

	// the message can now be read from the current message pointer
	// update statistics counters
	chann.frame_latency = float32(float64(chann.frame_latency)*OLD_AVG +
		float64(uint32(chann.outgoing_sequence)-sequence_ack)*(1.0-OLD_AVG))
	chann.frame_rate = float32(float64(chann.frame_rate)*OLD_AVG +
		(curtime-float64(chann.last_received))*(1.0-OLD_AVG))
	chann.good_count += 1

	chann.last_received = float32(curtime)

	if chann.huffcontext != nil {
		offset := 8
		if chann.sock == NS_SERVER {
			offset = 10
		}
		Huff_DecompressPacketSizebuf(chann.huffcontext, message, offset)
	}

	return true
}

package engine

import (
	"fmt"
	"strings"
)

const INADDR_LOOPBACK = 0x7f000001

const MAX_LOOPBACK = 4 // must be a power of two

type loopmsg struct {
	data    [MAX_MSGLEN * 2]byte
	datalen int
}

type loopback struct {
	msgs      [MAX_LOOPBACK]loopmsg
	get, send uint32
}

type NetData struct {
	sysnetdata *SysNetData
	loopbacks  [2]loopback
	sockets    [2]*SysSocket
}

var netdata *NetData

//=============================================================================

func NET_CompareBaseAdr(a, b *netaddr) bool {
	if a.type_ != b.type_ {
		return false
	}

	if a.type_ == NA_LOOPBACK {
		return true
	} else if a.type_ == NA_IPV4 {
		if a.ipv4.address == b.ipv4.address {
			return true
		}
	} else if a.type_ == NA_IPV6 {
		if a.ipv6.address == b.ipv6.address {
			return true
		}
	}

	return false
}

func NET_CompareAdr(a, b *netaddr) bool {
	if a.type_ != b.type_ {
		return false
	}

	if a.type_ == NA_LOOPBACK {
		return true
	} else if a.type_ == NA_IPV4 {
		if a.ipv4 == b.ipv4 {
			return true
		}
	} else if a.type_ == NA_IPV6 {
		if a.ipv6 == b.ipv6 {
			return true
		}
	}

	return false
}

func NET_IsLocalAddress(a *netaddr) bool {
	if a.type_ != NA_IPV4 {
		return false
	}

	if a.ipv4.address[0] == (INADDR_LOOPBACK>>24)&0xff &&
		a.ipv4.address[1] == (INADDR_LOOPBACK>>16)&0xff &&
		a.ipv4.address[2] == (INADDR_LOOPBACK>>8)&0xff &&
		a.ipv4.address[3] == (INADDR_LOOPBACK>>0)&0xff {
		return true
	}

	return false
}

// The output never gets near the 64 byte buffer of C, so the size juggling
// is left out.
func NET_IPv6ToString(a *netaddr, addport bool) string {
	addr := &a.ipv6.address
	biggestzeroblockstart := -1
	biggestzeroblocksize := 0

	for i := 0; i < len(addr); i += 2 {
		if addr[i] == 0 && addr[i+1] == 0 {
			j := 2
			for ; j+i < len(addr); j += 2 {
				if addr[i+j] != 0 || addr[i+j+1] != 0 {
					break
				}
			}

			if j > biggestzeroblocksize {
				biggestzeroblocksize = j
				biggestzeroblockstart = i
			}
		}
	}

	var s strings.Builder
	s.WriteByte('[')

	for i := 0; i < len(addr); i += 2 {
		if i == biggestzeroblockstart {
			s.WriteByte(':')
			i += biggestzeroblocksize - 2
		} else {
			if i != 0 {
				s.WriteByte(':')
			}
			fmt.Fprintf(&s, "%x", int(addr[i])<<8|int(addr[i+1]))
		}
	}

	s.WriteByte(']')

	if addport {
		fmt.Fprintf(&s, ":%d", a.ipv6.port)
	}

	return s.String()
}

func NET_AdrToString(a *netaddr) string {
	if a.type_ == NA_LOOPBACK {
		return "loopback"
	} else if a.type_ == NA_IPV4 {
		return fmt.Sprintf("%d.%d.%d.%d:%d", a.ipv4.address[0], a.ipv4.address[1], a.ipv4.address[2], a.ipv4.address[3], a.ipv4.port)
	} else if a.type_ == NA_IPV6 {
		return NET_IPv6ToString(a, true)
	}
	return ""
}

func NET_BaseAdrToString(a *netaddr) string {
	if a.type_ == NA_LOOPBACK {
		return "loopback"
	} else if a.type_ == NA_IPV4 {
		return fmt.Sprintf("%d.%d.%d.%d", a.ipv4.address[0], a.ipv4.address[1], a.ipv4.address[2], a.ipv4.address[3])
	} else if a.type_ == NA_IPV6 {
		return NET_IPv6ToString(a, false)
	}
	return ""
}

// Returns the number of digits consumed, 0 when the value exceeds maxvalue.
func NET_StringDecToUInt(s string, maxvalue uint32) (int, uint32) {
	n := 0
	var value uint32
	for at(s, n) >= '0' && at(s, n) <= '9' && value <= maxvalue {
		value *= 10
		value += uint32(s[n] - '0')
		n++
	}

	if value > maxvalue {
		return 0, value
	}

	return n, value
}

func NET_StringToIPv6(s string, a *netaddr) int {
	return 0
}

func NET_StringToIPv4(s string, a *netaddr) int {
	pos := 0
	for i := 0; i < 4; i++ {
		r, value := NET_StringDecToUInt(s[pos:], 255)
		if r == 0 {
			return 0
		}

		pos += r

		if i == 3 {
			if at(s, pos) != 0 && at(s, pos) != ':' {
				return 0
			}
		} else {
			if at(s, pos) != '.' {
				return 0
			}
			pos++
		}

		a.ipv4.address[i] = byte(value)
	}

	a.type_ = NA_IPV4

	return pos
}

func NET_StringToNetAddrNoPort(sysnetdata *SysNetData, s string, a *netaddr) int {
	if sysnetdata == nil {
		sysnetdata = netdata.sysnetdata
	}

	if s == "local" {
		a.type_ = NA_LOOPBACK
		return 5
	}

	if n := NET_StringToIPv6(s, a); n != 0 {
		return n
	}

	if n := NET_StringToIPv4(s, a); n != 0 {
		return n
	}

	endofhostname := strings.IndexByte(s, ':')
	if endofhostname < 0 {
		endofhostname = len(s)
	}

	if !Sys_Net_ResolveName(sysnetdata, s[:endofhostname], a) {
		endofhostname = 0
	}

	return endofhostname
}

func NET_StringToAdr(sysnetdata *SysNetData, s string, a *netaddr) bool {
	if sysnetdata == nil {
		sysnetdata = netdata.sysnetdata
	}

	*a = netaddr{}

	n := NET_StringToNetAddrNoPort(sysnetdata, s, a)
	if n == 0 {
		return false
	}

	s = s[n:]

	var port uint32
	if at(s, 0) == ':' {
		s = s[1:]
		var n int
		n, port = NET_StringDecToUInt(s, 65535)
		if n == 0 {
			return false
		}
		s = s[n:]
	} else {
		port = 27500
	}

	if s != "" {
		return false
	}

	if a.type_ == NA_IPV4 {
		a.ipv4.port = uint16(port)
	} else if a.type_ == NA_IPV6 {
		a.ipv6.port = uint16(port)
	} else if port != 27500 {
		return false
	}

	return true
}

// The second result is false where C returns NULL.
func NET_GetHostnameForAddress(addr *netaddr) (string, bool) {
	return Sys_Net_ResolveAddress(netdata.sysnetdata, addr)
}

/*
=============================================================================
LOOPBACK BUFFERS FOR LOCAL PLAYER
=============================================================================
*/

func NET_GetLoopPacket(sock netsrc, message *sizebuf_t, from *netaddr) bool {
	loop := &netdata.loopbacks[sock]

	if loop.send-loop.get > MAX_LOOPBACK {
		loop.get = loop.send - MAX_LOOPBACK
	}

	if int32(loop.send-loop.get) <= 0 {
		return false
	}

	i := loop.get & (MAX_LOOPBACK - 1)
	loop.get++

	copy(message.data, loop.msgs[i].data[:loop.msgs[i].datalen])
	message.cursize = loop.msgs[i].datalen
	*from = netaddr{type_: NA_LOOPBACK}
	return true
}

func NET_SendLoopPacket(sock netsrc, data []byte) {
	loop := &netdata.loopbacks[sock^1]

	i := loop.send & (MAX_LOOPBACK - 1)
	loop.send++

	if len(data) > len(loop.msgs[i].data) {
		Sys_Error("NET_SendLoopPacket: length > MAX_UDP_PACKET")
	}

	copy(loop.msgs[i].data[:], data)
	loop.msgs[i].datalen = len(data)
}

//=============================================================================

func NET_ClearLoopback() {
	if netdata == nil {
		Sys_Error("NET_ClearLoopback() called before net init\n")
	}

	netdata.loopbacks[0].send = 0
	netdata.loopbacks[0].get = 0
	netdata.loopbacks[1].send = 0
	netdata.loopbacks[1].get = 0
}

func NET_GetPacket(sock netsrc, message *sizebuf_t, from *netaddr) bool {
	if NET_GetLoopPacket(sock, message, from) {
		return true
	}

	net_socket := netdata.sockets[sock]
	if net_socket == nil {
		return false
	}

	ret := Sys_Net_Receive(netdata.sysnetdata, net_socket, message.data[:message.maxsize], from)
	if ret <= 0 {
		if ret < 0 {
			Com_Printf("NET_GetPacket: Network error.\n")
		}
		return false
	}

	message.cursize = ret

	return true
}

//=============================================================================

func NET_SendPacket(sock netsrc, data []byte, to *netaddr) {
	if to.type_ == NA_LOOPBACK {
		NET_SendLoopPacket(sock, data)
		return
	}

	net_socket := netdata.sockets[sock]
	if net_socket == nil {
		return
	}

	ret := Sys_Net_Send(netdata.sysnetdata, net_socket, data, to)
	if ret < 0 {
		Com_Printf("NET_SendPacket: Network error.\n")
	}
}

//=============================================================================

func NET_OpenSocket(socknum netsrc, type_ netaddrtype) bool {
	if netdata.sockets[socknum] != nil {
		Sys_Net_DeleteSocket(netdata.sysnetdata, netdata.sockets[socknum])
	}

	netdata.sockets[socknum] = Sys_Net_CreateSocket(netdata.sysnetdata, type_)

	return netdata.sockets[socknum] != nil
}

func NET_ServerConfig(enable bool) {
	if netdata == nil {
		return
	}

	if enable {
		if netdata.sockets[NS_SERVER] == nil {
			port := 0
			i := COM_CheckParm("-port")
			if i != 0 && i < com_argc {
				port = atoi(com_argv[i+1])
			}
			if port == 0 {
				port = PORT_SERVER
			}

			netdata.sockets[NS_SERVER] = Sys_Net_CreateSocket(netdata.sysnetdata, NA_IPV4)
			if netdata.sockets[NS_SERVER] == nil {
				Com_Printf("WARNING: Couldn't allocate server socket.\n")
			} else {
				if !Sys_Net_Bind(netdata.sysnetdata, netdata.sockets[NS_SERVER], uint16(port)) {
					Com_Printf("WARNING: Unable to bind server socket to port.\n")
				} else {
					return
				}

				Sys_Net_DeleteSocket(netdata.sysnetdata, netdata.sockets[NS_SERVER])
				netdata.sockets[NS_SERVER] = nil
			}
		}
	} else {
		if netdata.sockets[NS_SERVER] != nil {
			Sys_Net_DeleteSocket(netdata.sysnetdata, netdata.sockets[NS_SERVER])
			netdata.sockets[NS_SERVER] = nil
		}
	}
}

// Sleeps msec or until the server socket is ready; disabled in C as well.
func NET_Sleep(msec int) {
}

func NET_Init() {
	netdata = &NetData{}
	netdata.sysnetdata = Sys_Net_Init()
	if netdata.sysnetdata == nil {
		Com_Printf("Failed to initialise networking\n")
	}
}

func NET_Shutdown() {
	if netdata == nil {
		return
	}

	if netdata.sockets[NS_CLIENT] != nil {
		Sys_Net_DeleteSocket(netdata.sysnetdata, netdata.sockets[NS_CLIENT])
	}

	NET_ServerConfig(false)

	Sys_Net_Shutdown(netdata.sysnetdata)

	netdata = nil
}

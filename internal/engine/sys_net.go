package engine

import (
	"errors"
	"io"
	"net"
	"runtime"
	"strings"
	"sync"
	"time"
)

// sys_net.h on top of the Go net package. net_win32.c and net_posix.c use
// nonblocking BSD sockets and select; here every UDP socket has a reader
// goroutine that feeds a channel, Sys_Net_Receive takes from the channel
// without blocking and Sys_Net_Wait waits on it.

type SysNetData struct{}

type sysPacket struct {
	data []byte // nil marks a receive error, C returns -1 then
	from netaddr
}

const sysPacketBuf = 2048

// packet buffers recycled by Sys_Net_Receive, bigger packets get their own
var sysPacketPool = sync.Pool{New: func() any { return new([sysPacketBuf]byte) }}

type SysSocket struct {
	conn       *net.UDPConn
	domain     netaddrtype
	packets    chan sysPacket
	closed     chan struct{}
	pending    *sysPacket // taken by Sys_Net_Wait, handed out by the next receive
	pendingbuf sysPacket
	timer      *time.Timer // reused by Sys_Net_Wait
}

type SysTCPSocket struct {
	conn    net.Conn
	timeout time.Duration // SO_RCVTIMEO and SO_SNDTIMEO of C
}

func Sys_Net_Init() *SysNetData {
	return &SysNetData{}
}

func Sys_Net_Shutdown(netdata *SysNetData) {
}

func sysNetFromIP(ip net.IP, port int, address *netaddr) {
	*address = netaddr{}
	if ip4 := ip.To4(); ip4 != nil {
		address.type_ = NA_IPV4
		copy(address.ipv4.address[:], ip4)
		address.ipv4.port = uint16(port)
		return
	}
	address.type_ = NA_IPV6
	copy(address.ipv6.address[:], ip.To16())
	address.ipv6.port = uint16(port)
}

func sysNetToUDPAddr(address *netaddr) *net.UDPAddr {
	if address.type_ == NA_IPV4 {
		return &net.UDPAddr{IP: net.IP(address.ipv4.address[:]), Port: int(address.ipv4.port)}
	}
	return &net.UDPAddr{IP: net.IP(address.ipv6.address[:]), Port: int(address.ipv6.port)}
}

// the first address getaddrinfo returns, whatever its family
func Sys_Net_ResolveName(netdata *SysNetData, name string, address *netaddr) bool {
	ips, err := net.LookupIP(name)
	if err != nil || len(ips) == 0 {
		return false
	}

	sysNetFromIP(ips[0], 0, address)
	return true
}

// The second result is false where C returns false. net_win32.c has the
// getnameinfo call commented out, so Windows never resolves.
func Sys_Net_ResolveAddress(netdata *SysNetData, address *netaddr) (string, bool) {
	if runtime.GOOS == "windows" {
		return "", false
	}

	var ip net.IP
	if address.type_ == NA_IPV4 {
		ip = net.IP(address.ipv4.address[:])
	} else if address.type_ == NA_IPV6 {
		ip = net.IP(address.ipv6.address[:])
	} else {
		return "", false
	}

	names, err := net.LookupAddr(ip.String())
	if err != nil || len(names) == 0 {
		return "", false
	}

	return strings.TrimSuffix(names[0], "."), true
}

func sysSocketReader(s *SysSocket, conn *net.UDPConn) {
	buf := make([]byte, 65536)
	for {
		n, from, err := conn.ReadFromUDP(buf)
		var p sysPacket
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			// recvfrom failed, for instance after an ICMP port unreachable
		} else {
			if n <= sysPacketBuf {
				p.data = sysPacketPool.Get().(*[sysPacketBuf]byte)[:n]
			} else {
				p.data = make([]byte, n)
			}
			copy(p.data, buf[:n])
			sysNetFromIP(from.IP, from.Port, &p.from)
		}

		select {
		case s.packets <- p:
		case <-s.closed:
			return
		}
	}
}

func sysSocketListen(s *SysSocket, port int) bool {
	network := "udp4"
	if s.domain == NA_IPV6 {
		network = "udp6"
	}

	conn, err := net.ListenUDP(network, &net.UDPAddr{Port: port})
	if err != nil {
		return false
	}

	s.conn = conn
	s.packets = make(chan sysPacket, 256)
	s.closed = make(chan struct{})
	s.pending = nil
	go sysSocketReader(s, conn)

	return true
}

func sysSocketClose(s *SysSocket) {
	if s.conn != nil {
		close(s.closed)
		s.conn.Close()
		s.conn = nil
	}
}

func Sys_Net_CreateSocket(netdata *SysNetData, addrtype netaddrtype) *SysSocket {
	if addrtype != NA_IPV4 && addrtype != NA_IPV6 {
		return nil
	}

	s := &SysSocket{domain: addrtype}
	if !sysSocketListen(s, 0) {
		return nil
	}

	return s
}

func Sys_Net_DeleteSocket(netdata *SysNetData, socket *SysSocket) {
	sysSocketClose(socket)
}

// C binds the existing socket; a Go socket is bound when it is opened, so
// it is opened again on the port.
func Sys_Net_Bind(netdata *SysNetData, socket *SysSocket, port uint16) bool {
	sysSocketClose(socket)
	return sysSocketListen(socket, int(port))
}

func Sys_Net_Send(netdata *SysNetData, socket *SysSocket, data []byte, address *netaddr) int {
	if socket.conn == nil || address == nil {
		return -1
	}

	r, err := socket.conn.WriteToUDP(data, sysNetToUDPAddr(address))
	if err != nil {
		return -1
	}

	return r
}

func Sys_Net_Receive(netdata *SysNetData, socket *SysSocket, data []byte, address *netaddr) int {
	var p sysPacket

	if socket.pending != nil {
		p = *socket.pending
		socket.pending = nil
	} else if socket.packets == nil {
		return -1
	} else {
		select {
		case p = <-socket.packets:
		default:
			return 0
		}
	}

	if p.data == nil {
		return -1
	}

	if address != nil {
		*address = p.from
	}

	r := copy(data, p.data)
	if cap(p.data) == sysPacketBuf {
		sysPacketPool.Put((*[sysPacketBuf]byte)(p.data[:sysPacketBuf]))
	}

	return r
}

func Sys_Net_Wait(netdata *SysNetData, socket *SysSocket, timeout_us uint) {
	if socket.pending != nil || socket.packets == nil {
		return
	}

	d := time.Duration(timeout_us) * time.Microsecond
	if socket.timer == nil {
		socket.timer = time.NewTimer(d)
	} else {
		socket.timer.Reset(d)
	}

	select {
	case p := <-socket.packets:
		socket.timer.Stop()
		socket.pendingbuf = p
		socket.pending = &socket.pendingbuf
	case <-socket.timer.C:
	}
}

func Sys_Net_TCPConnect(netdata *SysNetData, address *netaddr, timeout_us uint) *SysTCPSocket {
	if address.type_ != NA_IPV4 && address.type_ != NA_IPV6 {
		return nil
	}

	timeout := time.Duration(timeout_us) * time.Microsecond
	conn, err := net.DialTimeout("tcp", sysNetToUDPAddr(address).String(), timeout)
	if err != nil {
		return nil
	}

	return &SysTCPSocket{conn: conn, timeout: timeout}
}

func Sys_Net_TCPClose(netdata *SysNetData, socket *SysTCPSocket) {
	socket.conn.Close()
}

func Sys_Net_TCPSend(netdata *SysNetData, socket *SysTCPSocket, data []byte) int {
	sent := 0
	for sent < len(data) {
		socket.conn.SetWriteDeadline(time.Now().Add(socket.timeout))
		r, err := socket.conn.Write(data[sent:])
		if err != nil || r <= 0 {
			return -1
		}

		sent += r
	}

	return sent
}

// 0 at the end of the stream, -1 on an error or timeout, like recv
func Sys_Net_TCPReceive(netdata *SysNetData, socket *SysTCPSocket, data []byte) int {
	socket.conn.SetReadDeadline(time.Now().Add(socket.timeout))
	r, err := socket.conn.Read(data)
	if err != nil {
		if r > 0 {
			return r
		}
		if errors.Is(err, io.EOF) {
			return 0
		}
		return -1
	}
	return r
}

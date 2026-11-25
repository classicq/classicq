package engine

import (
	"bytes"
	"strings"
	"sync"
	"sync/atomic"
)

// serverscanner.c: asks the master servers for the server list, then
// queries and pings every server from its own thread. The thread is a
// goroutine, the mutex a sync.Mutex; the fields C marks volatile are atomic,
// the others are shared without locks as in C.

type ServerScannerStatus int32

const (
	SSS_SCANNING ServerScannerStatus = iota
	SSS_PINGING
	SSS_IDLE
	SSS_ERROR
)

type QWServerStatus int

const (
	QWSS_WAITING QWServerStatus = iota
	QWSS_REQUESTSENT
	QWSS_DONE
	QWSS_FAILED
)

// The string pointers are nil where C has null pointers.
type QWPlayer struct {
	name        *string
	team        *string
	frags       uint32
	time        uint32
	ping        uint32
	topcolor    uint32
	bottomcolor uint32
}

type QWSpectator struct {
	name *string
	team *string
	time uint32
	ping uint32
}

type QWServer struct {
	addr          netaddr
	status        QWServerStatus
	pingtime      uint32  // In microseconds
	hostname      *string // Note, this is the _QW_ text 'hostname'
	gamedir       *string
	maxclients    uint32
	maxspectators uint32
	teamplay      uint32
	map_          *string
	players       []QWPlayer
	numplayers    uint32
	spectators    []QWSpectator
	numspectators uint32
}

const MAXCONCURRENTSCANS = 64
const MAXCONCURRENTPINGS = 32
const PINGINTERVAL = 5000
const QWSERVERTIMEOUT = 750000
const HTTPFALLBACKDELAY = 500000
const HTTPFETCHDEADLINE = 5000000
const MAXHTTPADDRS = 4096

type masterserver struct {
	hostname string
	addr     netaddr
}

type qwserverpriv struct {
	next               *qwserverpriv
	nextscaninprogress *qwserverpriv
	nextscanwaiting    *qwserverpriv
	nextpinginprogress *qwserverpriv
	nextpingwaiting    *qwserverpriv
	packetsendtime     uint64
	pub                QWServer
}

type ServerScanner struct {
	netdata                    *SysNetData
	sockets                    [NA_NUMTYPES]*SysSocket
	thread                     chan struct{} // closed when the thread has ended
	mutex                      sync.Mutex
	initialstuffdone           bool
	starttime                  uint64
	masterservers              []masterserver
	qwservers                  *qwserverpriv
	qwserversscaninprogress    *qwserverpriv
	qwserversscanwaiting       *qwserverpriv
	qwserverspinginprogress    *qwserverpriv
	qwserverspingwaiting       *qwserverpriv
	quit                       atomic.Uint32
	nummasterservers           uint
	numvalidmasterservers      uint
	numqwservers               uint
	numqwserversscaninprogress uint
	numqwserverspinginprogress uint
	lastpingtime               uint64
	status                     atomic.Int32
	updated                    bool

	httpsources   string // "" is C's null pointer
	httpthread    chan struct{}
	httpaddrs     []netaddr
	numhttpaddrs  uint
	httpstarted   bool
	httpdone      bool
	httpchecked   bool
	masterreplied bool

	debug_t0                uint64
	debug_first_master_done bool
	debug_first_status_done bool
	debug_idle_done         bool
}

func (s *ServerScanner) getStatus() ServerScannerStatus {
	return ServerScannerStatus(s.status.Load())
}

func (s *ServerScanner) setStatus(status ServerScannerStatus) {
	s.status.Store(int32(status))
}

func debug_elapsed_ms(s *ServerScanner) uint64 {
	return (Sys_IntTime() - s.debug_t0) / 1000
}

func ServerScanner_Thread_Init(serverscanner *ServerScanner) bool {
	serverscanner.netdata = Sys_Net_Init()

	return serverscanner.netdata != nil
}

func ServerScanner_Thread_Deinit(serverscanner *ServerScanner) {
	Sys_Net_Shutdown(serverscanner.netdata)
}

func ServerScanner_Thread_LookUpMasters(serverscanner *ServerScanner) {
	for i := range serverscanner.masterservers {
		if !NET_StringToAdr(serverscanner.netdata, serverscanner.masterservers[i].hostname, &serverscanner.masterservers[i].addr) {
			serverscanner.numvalidmasterservers--
		}
	}
}

func ServerScanner_Thread_OpenSockets(serverscanner *ServerScanner) {
	for i := range serverscanner.masterservers {
		addrtype := serverscanner.masterservers[i].addr.type_

		if addrtype == NA_LOOPBACK {
			continue
		}

		if serverscanner.sockets[addrtype] == nil {
			serverscanner.sockets[addrtype] = Sys_Net_CreateSocket(serverscanner.netdata, addrtype)
		}

		if serverscanner.sockets[addrtype] == nil {
			serverscanner.numvalidmasterservers--
		}
	}
}

func ServerScanner_Thread_CloseSockets(serverscanner *ServerScanner) {
	for i := range serverscanner.sockets {
		if serverscanner.sockets[i] != nil {
			Sys_Net_DeleteSocket(serverscanner.netdata, serverscanner.sockets[i])
		}
	}
}

func ServerScanner_Thread_QueryMasters(serverscanner *ServerScanner) {
	querystring := []byte("c\n\x00")

	for i := range serverscanner.masterservers {
		if serverscanner.sockets[serverscanner.masterservers[i].addr.type_] != nil {
			Sys_Net_Send(serverscanner.netdata, serverscanner.sockets[serverscanner.masterservers[i].addr.type_], querystring, &serverscanner.masterservers[i].addr)
		}
	}
}

// Drop TeamFortress, QWFWD proxies, QTV streamers
func is_unwanted_server(server *QWServer) bool {
	if server.gamedir != nil && Q_strcasecmp(*server.gamedir, "fortress") == 0 {
		return true
	}
	if server.hostname != nil {
		if Util_strcasestr(*server.hostname, "qwfwd") >= 0 {
			return true
		}
		if Util_strcasestr(*server.hostname, "qtv") >= 0 {
			return true
		}
	}
	return false
}

// The reply is parsed in place like C does with its NUL terminators; the
// buffer ends where the packet ends.
func ServerScanner_Thread_ParseQWServerReply(serverscanner *ServerScanner, qwserver *qwserverpriv, data []byte) {
	var name, team [32]*string
	var spectator [32]bool
	var frags, time_ [32]int32
	var ping, topcolour, bottomcolour [32]uint32

	qwserver.pub.status = QWSS_FAILED

	if len(data) < 2 || data[0] != 'n' {
		return
	}

	data = data[1:]

	p := bytes.IndexByte(data, '\n')
	if p < 0 {
		return
	}

	if p2 := bytes.IndexByte(data, 0); p2 >= 0 && p2 < p {
		return
	}

	data[p] = 0

	p = 1
	for cbufAt(data, p) != 0 {
		key := p

		p = cbufChr(data, key, '\\')
		if p < 0 {
			break
		}

		data[p] = 0
		value := p + 1

		p = cbufChr(data, value, '\\')
		if p >= 0 {
			data[p] = 0
			p++
		} else {
			p = cbufEnd(data, value)
		}

		k := cbufStr(data, key)
		v := cbufStr(data, value)

		k = strings.TrimPrefix(k, "*")
		switch k {
		case "maxclients":
			qwserver.pub.maxclients = uint32(strtoul(v, 0))
		case "maxspectators":
			qwserver.pub.maxspectators = uint32(strtoul(v, 0))
		case "teamplay":
			qwserver.pub.teamplay = uint32(strtoul(v, 0))
		case "map":
			s := v
			qwserver.pub.map_ = &s
		case "hostname":
			s := v
			qwserver.pub.hostname = &s
		case "gamedir":
			s := v
			qwserver.pub.gamedir = &s
		}
	}

	if p < 0 {
		return
	}

	if p >= len(data) {
		return
	}

	data = data[p+1:]

	if is_unwanted_server(&qwserver.pub) {
		qwserver.pub.map_ = nil
		qwserver.pub.hostname = nil
		qwserver.pub.gamedir = nil
		return
	}

	qwserver.pub.status = QWSS_DONE

	numplayers := uint32(0)
	numspectators := uint32(0)

	i := 0
	for {
		if numplayers+numspectators >= 32 {
			break
		}

		p = bytes.IndexByte(data, '\n')
		if p < 0 {
			break
		}

		if p2 := bytes.IndexByte(data, 0); p2 >= 0 && p2 < p {
			break
		}

		data[p] = 0

		p2 := cbufChr(data, 0, ' ')
		if p2 < 0 {
			break
		}

		p2++
		frags[i] = int32(atoi(cbufStr(data, p2)))

		p2 = cbufChr(data, p2, ' ')
		if p2 < 0 {
			break
		}

		p2++
		time_[i] = int32(atoi(cbufStr(data, p2)))

		p2 = cbufChr(data, p2, ' ')
		if p2 < 0 {
			break
		}

		p2++
		ping[i] = uint32(int32(atoi(cbufStr(data, p2))))

		p2 = cbufChr(data, p2, ' ')
		if p2 < 0 {
			break
		}

		p2++
		if cbufAt(data, p2) != '"' {
			break
		}

		p2++
		nameat := p2

		p2 = cbufChr(data, p2, '"')
		if p2 < 0 {
			break
		}

		data[p2] = 0
		p2++
		if cbufAt(data, p2) != ' ' {
			break
		}

		p2++

		if cbufAt(data, p2) != '"' {
			break
		}

		p2++

		p2 = cbufChr(data, p2, '"')
		if p2 < 0 {
			break
		}

		p2++
		if cbufAt(data, p2) != 0 {
			p2++
		}

		topcolour[i] = uint32(int32(atoi(cbufStr(data, p2))))

		p2 = cbufChr(data, p2, ' ')
		if p2 < 0 {
			break
		}

		p2++
		bottomcolour[i] = uint32(int32(atoi(cbufStr(data, p2))))

		p2 = cbufChr(data, p2, ' ')
		if p2 >= 0 {
			p2++
		}

		if p2 >= 0 && cbufAt(data, p2) == '"' {
			p2++
			teamat := p2

			p2 = cbufChr(data, p2, '"')
			if p2 < 0 {
				break
			}

			data[p2] = 0
			t := cbufStr(data, teamat)
			team[i] = &t
		} else {
			team[i] = nil
		}

		n := cbufStr(data, nameat)
		if strings.HasPrefix(n, "\\s\\") {
			numspectators++
			spectator[i] = true
			time_[i] = -time_[i]
			n = n[3:]
		} else {
			numplayers++
			spectator[i] = false
		}
		name[i] = &n

		data = data[p+1:]

		i++
	}

	qwplayers := make([]QWPlayer, 0, numplayers)
	qwspectators := make([]QWSpectator, 0, numspectators)

	for i := uint32(0); i < numplayers+numspectators; i++ {
		if spectator[i] {
			qwspectators = append(qwspectators, QWSpectator{
				name: name[i],
				team: team[i],
				time: uint32(time_[i]),
				ping: ping[i],
			})
		} else {
			qwplayers = append(qwplayers, QWPlayer{
				name:        name[i],
				team:        team[i],
				frags:       uint32(frags[i]),
				time:        uint32(time_[i]),
				ping:        ping[i],
				topcolor:    topcolour[i],
				bottomcolor: bottomcolour[i],
			})
		}
	}

	qwserver.pub.players = qwplayers
	qwserver.pub.numplayers = numplayers
	qwserver.pub.spectators = qwspectators
	qwserver.pub.numspectators = numspectators
}

func ServerScanner_Thread_SendQWRequest(serverscanner *ServerScanner, qwserver *qwserverpriv) {
	querystring := []byte("\xff\xff\xff\xffstatus 23\n\x00")

	qwserver.packetsendtime = Sys_IntTime()
	Sys_Net_Send(serverscanner.netdata, serverscanner.sockets[NA_IPV4], querystring, &qwserver.pub.addr)
	if qwserver.pub.status != QWSS_REQUESTSENT {
		qwserver.pub.status = QWSS_REQUESTSENT
		serverscanner.numqwserversscaninprogress++
		qwserver.nextscaninprogress = serverscanner.qwserversscaninprogress
		serverscanner.qwserversscaninprogress = qwserver
	}
}

func ServerScanner_Thread_SendQWPingRequest(serverscanner *ServerScanner, qwserver *qwserverpriv) {
	querystring := []byte("\xff\xff\xff\xffk\x00")

	curtime := Sys_IntTime()

	qwserver.packetsendtime = curtime
	Sys_Net_Send(serverscanner.netdata, serverscanner.sockets[NA_IPV4], querystring, &qwserver.pub.addr)

	serverscanner.numqwserverspinginprogress++
	qwserver.nextpinginprogress = serverscanner.qwserverspinginprogress
	serverscanner.qwserverspinginprogress = qwserver

	serverscanner.lastpingtime = curtime
}

/* Caller must hold the mutex */
func ServerScanner_AddServer(serverscanner *ServerScanner, newaddr *netaddr) *qwserverpriv {
	if serverscanner.numqwservers > 10000 {
		return nil
	}

	// a linear scan, as in C
	for qwserver := serverscanner.qwservers; qwserver != nil; qwserver = qwserver.next {
		if NET_CompareAdr(newaddr, &qwserver.pub.addr) {
			return nil
		}
	}

	qwserver := &qwserverpriv{}

	qwserver.pub.addr = *newaddr
	qwserver.pub.status = QWSS_WAITING

	qwserver.next = serverscanner.qwservers
	serverscanner.qwservers = qwserver
	serverscanner.numqwservers++

	return qwserver
}

func ServerScanner_HttpThread(serverscanner *ServerScanner) {
	deadline := Sys_IntTime() + HTTPFETCHDEADLINE

	numaddrs := uint(0)
	addrs := make([]netaddr, MAXHTTPADDRS)

	p := serverscanner.httpsources
	for len(p) > 0 && serverscanner.quit.Load() == 0 && numaddrs < MAXHTTPADDRS && Sys_IntTime() < deadline {
		p2 := strings.IndexByte(p, ' ')
		if p2 < 0 {
			p2 = len(p)
		}

		if p2 != 0 && p2 < 512 {
			url := p[:p2]

			urladdrs := HTTPList_Fetch(serverscanner.netdata, url, addrs[numaddrs:], &serverscanner.quit, deadline)
			numaddrs += urladdrs

			if sb_debug.value != 0 {
				Com_Printf("SB: t=%dms    http source %s: %d addresses\n", debug_elapsed_ms(serverscanner), url, urladdrs)
			}
		}

		p = p[p2:]
		if len(p) > 0 {
			p = p[1:]
		}
	}

	serverscanner.mutex.Lock()

	serverscanner.httpaddrs = addrs
	serverscanner.numhttpaddrs = numaddrs
	serverscanner.httpdone = true

	serverscanner.mutex.Unlock()
}

func ServerScanner_Thread_HandlePacket(serverscanner *ServerScanner, data []byte, addr *netaddr) {
	var newaddr netaddr

	if len(data) > 0 && data[0] == 'l' {
		var prevqwserver *qwserverpriv
		for qwserver := serverscanner.qwserverspinginprogress; qwserver != nil; qwserver = qwserver.nextpinginprogress {
			if NET_CompareAdr(&qwserver.pub.addr, addr) {
				serverscanner.mutex.Lock()

				qwserver.pub.pingtime = uint32(Sys_IntTime() - qwserver.packetsendtime)

				serverscanner.updated = true

				serverscanner.mutex.Unlock()

				if prevqwserver != nil {
					prevqwserver.nextpinginprogress = qwserver.nextpinginprogress
				} else {
					serverscanner.qwserverspinginprogress = qwserver.nextpinginprogress
				}

				qwserver.nextpinginprogress = nil
				serverscanner.numqwserverspinginprogress--

				return
			}

			prevqwserver = qwserver
		}

		return
	}

	if len(data) < 4 || data[0] != 255 || data[1] != 255 || data[2] != 255 || data[3] != 255 {
		return
	}

	data = data[4:]

	for i := range serverscanner.masterservers {
		if NET_CompareAdr(&serverscanner.masterservers[i].addr, addr) {
			if len(data) < 2 || data[0] != 'd' || data[1] != '\n' {
				return
			}

			data = data[2:]

			if len(data)%6 != 0 {
				return
			}

			if !serverscanner.debug_first_master_done && sb_debug.value != 0 {
				Com_Printf("SB: t=%dms    first master reply (%d records, %d known)\n",
					debug_elapsed_ms(serverscanner), len(data)/6, serverscanner.numqwservers)
				serverscanner.debug_first_master_done = true
			}

			serverscanner.mutex.Lock()

			serverscanner.masterreplied = true

			for i := 0; i < len(data); i += 6 {
				newaddr = netaddr{}
				newaddr.type_ = NA_IPV4
				newaddr.ipv4.address[0] = data[i+0]
				newaddr.ipv4.address[1] = data[i+1]
				newaddr.ipv4.address[2] = data[i+2]
				newaddr.ipv4.address[3] = data[i+3]
				newaddr.ipv4.port = uint16(data[i+4])<<8 | uint16(data[i+5])

				qwserver := ServerScanner_AddServer(serverscanner, &newaddr)
				if qwserver != nil {
					if serverscanner.numqwserversscaninprogress < MAXCONCURRENTSCANS {
						ServerScanner_Thread_SendQWRequest(serverscanner, qwserver)
					} else {
						qwserver.nextscanwaiting = serverscanner.qwserversscanwaiting
						serverscanner.qwserversscanwaiting = qwserver
					}
				}
			}

			serverscanner.updated = true

			serverscanner.mutex.Unlock()

			return
		}
	}

	var prevqwserver *qwserverpriv
	for qwserver := serverscanner.qwserversscaninprogress; qwserver != nil; qwserver = qwserver.nextscaninprogress {
		if NET_CompareAdr(&qwserver.pub.addr, addr) && qwserver.pub.status == QWSS_REQUESTSENT {
			if !serverscanner.debug_first_status_done && sb_debug.value != 0 {
				Com_Printf("SB: t=%dms    first status reply\n", debug_elapsed_ms(serverscanner))
				serverscanner.debug_first_status_done = true
			}

			serverscanner.mutex.Lock()

			serverscanner.updated = true

			ServerScanner_Thread_ParseQWServerReply(serverscanner, qwserver, data)

			serverscanner.mutex.Unlock()

			if prevqwserver != nil {
				prevqwserver.nextscaninprogress = qwserver.nextscaninprogress
			} else {
				serverscanner.qwserversscaninprogress = qwserver.nextscaninprogress
			}

			qwserver.nextscaninprogress = nil
			serverscanner.numqwserversscaninprogress--

			qwserver.nextpingwaiting = serverscanner.qwserverspingwaiting
			serverscanner.qwserverspingwaiting = qwserver

			return
		}

		prevqwserver = qwserver
	}
}

func ServerScanner_Thread_CheckTimeout(serverscanner *ServerScanner) {
	curtime := Sys_IntTime()

	var prevqwserver *qwserverpriv
	for qwserver := serverscanner.qwserversscaninprogress; qwserver != nil; qwserver = qwserver.nextscaninprogress {
		if qwserver.packetsendtime+QWSERVERTIMEOUT <= curtime {
			qwserver.pub.status = QWSS_FAILED

			if prevqwserver != nil {
				prevqwserver.nextscaninprogress = qwserver.nextscaninprogress
			} else {
				serverscanner.qwserversscaninprogress = qwserver.nextscaninprogress
			}

			serverscanner.numqwserversscaninprogress--
		} else {
			prevqwserver = qwserver
		}
	}

	prevqwserver = nil
	for qwserver := serverscanner.qwserverspinginprogress; qwserver != nil; qwserver = qwserver.nextpinginprogress {
		if qwserver.packetsendtime+QWSERVERTIMEOUT <= curtime {
			qwserver.pub.pingtime = 999999
			qwserver.pub.status = QWSS_FAILED

			if prevqwserver != nil {
				prevqwserver.nextpinginprogress = qwserver.nextpinginprogress
			} else {
				serverscanner.qwserverspinginprogress = qwserver.nextpinginprogress
			}

			serverscanner.numqwserverspinginprogress--
		} else {
			prevqwserver = qwserver
		}
	}
}

func ServerScanner_DoStuffInternal(serverscanner *ServerScanner) uint {
	var buf [65536]byte
	var addr netaddr

	if serverscanner.getStatus() == SSS_ERROR || serverscanner.getStatus() == SSS_IDLE {
		return 50000
	}

	if !serverscanner.initialstuffdone {
		if !ServerScanner_Thread_Init(serverscanner) {
			serverscanner.setStatus(SSS_ERROR)
			return 50000
		}

		ServerScanner_Thread_LookUpMasters(serverscanner)
		ServerScanner_Thread_OpenSockets(serverscanner)

		if serverscanner.httpsources != "" && serverscanner.sockets[NA_IPV4] == nil {
			serverscanner.sockets[NA_IPV4] = Sys_Net_CreateSocket(serverscanner.netdata, NA_IPV4)
		}

		if serverscanner.numvalidmasterservers == 0 && serverscanner.sockets[NA_IPV4] == nil {
			serverscanner.setStatus(SSS_ERROR)
			return 50000
		}

		serverscanner.starttime = Sys_IntTime()
		serverscanner.debug_t0 = serverscanner.starttime

		if sb_debug.value != 0 {
			Com_Printf("SB: t=0ms       scanner querying %d master(s)\n", serverscanner.numvalidmasterservers)
		}

		ServerScanner_Thread_QueryMasters(serverscanner)

		serverscanner.initialstuffdone = true
	}

	for i := 1; i < int(NA_NUMTYPES); i++ {
		if serverscanner.sockets[i] == nil {
			continue
		}

		for {
			r := Sys_Net_Receive(serverscanner.netdata, serverscanner.sockets[i], buf[:], &addr)
			if r <= 0 {
				break
			}
			ServerScanner_Thread_HandlePacket(serverscanner, buf[:r], &addr)
		}
	}

	serverscanner.mutex.Lock()

	ServerScanner_Thread_CheckTimeout(serverscanner)

	if serverscanner.httpsources != "" && !serverscanner.httpstarted && !serverscanner.masterreplied &&
		Sys_IntTime() >= serverscanner.starttime+HTTPFALLBACKDELAY {
		serverscanner.httpstarted = true
		serverscanner.httpthread = make(chan struct{})
		go func() {
			ServerScanner_HttpThread(serverscanner)
			close(serverscanner.httpthread)
		}()
		if sb_debug.value != 0 {
			Com_Printf("SB: t=%dms    no master reply, trying http fallback\n", debug_elapsed_ms(serverscanner))
		}
	}

	if serverscanner.httpstarted && !serverscanner.httpchecked && serverscanner.httpdone {
		serverscanner.httpchecked = true

		if serverscanner.numqwservers == 0 && serverscanner.numhttpaddrs != 0 {
			for i := uint(0); i < serverscanner.numhttpaddrs; i++ {
				qwserver := ServerScanner_AddServer(serverscanner, &serverscanner.httpaddrs[i])
				if qwserver != nil {
					qwserver.nextscanwaiting = serverscanner.qwserversscanwaiting
					serverscanner.qwserversscanwaiting = qwserver
				}
			}

			serverscanner.updated = true

			if sb_debug.value != 0 {
				Com_Printf("SB: t=%dms    http fallback added %d servers\n", debug_elapsed_ms(serverscanner), serverscanner.numqwservers)
			}
		}

		serverscanner.httpaddrs = nil
	}

	if serverscanner.getStatus() == SSS_SCANNING {
		if serverscanner.qwserversscanwaiting == nil && serverscanner.qwserversscaninprogress == nil &&
			!(serverscanner.httpstarted && !serverscanner.httpchecked && serverscanner.numqwservers == 0) &&
			(serverscanner.numqwservers > 0 || Sys_IntTime() > serverscanner.starttime+2000000) {
			serverscanner.setStatus(SSS_PINGING)
		}
	}

	if serverscanner.getStatus() == SSS_PINGING {
		if serverscanner.qwserverspingwaiting == nil && serverscanner.qwserverspinginprogress == nil {
			if serverscanner.qwserversscanwaiting != nil {
				serverscanner.setStatus(SSS_SCANNING)
			} else {
				serverscanner.setStatus(SSS_IDLE)
				if !serverscanner.debug_idle_done && sb_debug.value != 0 {
					alive := uint(0)
					dead := uint(0)
					for p := serverscanner.qwservers; p != nil; p = p.next {
						if p.pub.status == QWSS_DONE {
							alive++
						} else if p.pub.status == QWSS_FAILED {
							dead++
						}
					}
					Com_Printf("SB: t=%dms   scanner idle (%d known, %d alive, %d dead)\n",
						debug_elapsed_ms(serverscanner), serverscanner.numqwservers, alive, dead)
					serverscanner.debug_idle_done = true
				}
			}
		}
	}

	curtime := Sys_IntTime()

	for serverscanner.getStatus() == SSS_SCANNING && serverscanner.numqwserversscaninprogress < MAXCONCURRENTSCANS && serverscanner.qwserversscanwaiting != nil {
		qwserver := serverscanner.qwserversscanwaiting
		ServerScanner_Thread_SendQWRequest(serverscanner, qwserver)
		serverscanner.qwserversscanwaiting = qwserver.nextscanwaiting
		qwserver.nextscanwaiting = nil
	}

	for serverscanner.getStatus() == SSS_PINGING && serverscanner.numqwserverspinginprogress < MAXCONCURRENTPINGS && serverscanner.qwserverspingwaiting != nil && serverscanner.lastpingtime+PINGINTERVAL <= curtime {
		qwserver := serverscanner.qwserverspingwaiting
		ServerScanner_Thread_SendQWPingRequest(serverscanner, qwserver)
		serverscanner.qwserverspingwaiting = qwserver.nextpingwaiting
		qwserver.nextpingwaiting = nil
	}

	timeout := uint(50000)
	if serverscanner.getStatus() == SSS_SCANNING {
		for qwserver := serverscanner.qwserversscaninprogress; qwserver != nil; qwserver = qwserver.nextscaninprogress {
			if qwserver.packetsendtime+QWSERVERTIMEOUT >= curtime && uint(qwserver.packetsendtime+QWSERVERTIMEOUT-curtime) < timeout {
				timeout = uint(qwserver.packetsendtime + QWSERVERTIMEOUT - curtime)
			}
		}
	} else if serverscanner.getStatus() == SSS_PINGING {
		for qwserver := serverscanner.qwserverspinginprogress; qwserver != nil; qwserver = qwserver.nextpinginprogress {
			if qwserver.packetsendtime+QWSERVERTIMEOUT >= curtime && uint(qwserver.packetsendtime+QWSERVERTIMEOUT-curtime) < timeout {
				timeout = uint(qwserver.packetsendtime + QWSERVERTIMEOUT - curtime)
			}
		}

		if serverscanner.qwserverspingwaiting != nil && serverscanner.lastpingtime+PINGINTERVAL >= curtime {
			if uint(serverscanner.lastpingtime+PINGINTERVAL-curtime) < timeout {
				timeout = uint(serverscanner.lastpingtime + PINGINTERVAL - curtime)
			}
		}
	}

	serverscanner.mutex.Unlock()

	return timeout
}

func ServerScanner_Thread(serverscanner *ServerScanner) {
	for serverscanner.quit.Load() == 0 {
		timeout := ServerScanner_DoStuffInternal(serverscanner)

		if serverscanner.sockets[NA_IPV4] != nil {
			Sys_Net_Wait(serverscanner.netdata, serverscanner.sockets[NA_IPV4], timeout)
		} else {
			Sys_MicroSleep(timeout)
		}
	}

	if serverscanner.httpthread != nil {
		<-serverscanner.httpthread
		serverscanner.httpthread = nil
	}

	ServerScanner_Thread_CloseSockets(serverscanner)

	ServerScanner_Thread_Deinit(serverscanner)
}

func ServerScanner_Create(masters, httpsources string) *ServerScanner {
	nummasterservers := uint(0)
	p := masters
	for len(p) > 0 {
		p2 := strings.IndexByte(p, ' ')
		if p2 < 0 {
			p2 = len(p)
		}

		if p2 != 0 {
			nummasterservers++
		}

		p = p[p2:]
		if len(p) > 0 {
			p = p[1:]
		}
	}

	if nummasterservers == 0 {
		return nil
	}

	if nummasterservers > 1000 {
		return nil
	}

	serverscanner := &ServerScanner{}

	serverscanner.httpsources = httpsources

	serverscanner.masterservers = make([]masterserver, nummasterservers)

	p = masters
	for i := uint(0); i < nummasterservers; {
		p2 := strings.IndexByte(p, ' ')
		if p2 < 0 {
			p2 = len(p)
		}

		if p2 != 0 {
			serverscanner.masterservers[i].hostname = p[:p2]
			i++
		}

		p = p[p2:]
		if len(p) > 0 {
			p = p[1:]
		}
	}

	serverscanner.setStatus(SSS_SCANNING)
	serverscanner.nummasterservers = nummasterservers
	serverscanner.numvalidmasterservers = nummasterservers

	serverscanner.thread = make(chan struct{})
	go func() {
		ServerScanner_Thread(serverscanner)
		close(serverscanner.thread)
	}()

	return serverscanner
}

func ServerScanner_Delete(serverscanner *ServerScanner) {
	serverscanner.quit.Store(1)

	if serverscanner.thread != nil {
		<-serverscanner.thread
	} else {
		if serverscanner.httpthread != nil {
			<-serverscanner.httpthread
		}

		ServerScanner_Thread_CloseSockets(serverscanner)
	}
}

func ServerScanner_DoStuff(serverscanner *ServerScanner) {
	if serverscanner.thread == nil {
		ServerScanner_DoStuffInternal(serverscanner)
	}
}

func ServerScanner_DataUpdated(serverscanner *ServerScanner) bool {
	serverscanner.mutex.Lock()

	ret := serverscanner.updated

	serverscanner.mutex.Unlock()

	return ret
}

// numservers is left alone where C does not allocate the array
func ServerScanner_GetServers(serverscanner *ServerScanner, numservers *uint) []*QWServer {
	var servers []*QWServer

	serverscanner.mutex.Lock()

	if serverscanner.numqwservers <= 10000 {
		servers = make([]*QWServer, 0, serverscanner.numqwservers)

		qwserver := serverscanner.qwservers
		for i := uint(0); i < serverscanner.numqwservers; i++ {
			if qwserver.pub.status == QWSS_DONE && qwserver.pub.hostname != nil && qwserver.pub.map_ != nil {
				servers = append(servers, &qwserver.pub)
			}

			qwserver = qwserver.next
		}

		*numservers = uint(len(servers))

		serverscanner.updated = false
	}

	serverscanner.mutex.Unlock()

	return servers
}

func ServerScanner_FreeServers(serverscanner *ServerScanner, servers []*QWServer) {
}

func ServerScanner_RescanServer(serverscanner *ServerScanner, server *QWServer) {
	if serverscanner == nil || server == nil {
		return
	}

	serverscanner.mutex.Lock()

	priv := serverscanner.qwservers
	for ; priv != nil; priv = priv.next {
		if &priv.pub == server {
			break
		}
	}

	if priv != nil && priv.pub.status == QWSS_DONE {
		priv.pub.status = QWSS_WAITING
		priv.nextscanwaiting = serverscanner.qwserversscanwaiting
		serverscanner.qwserversscanwaiting = priv
		if serverscanner.getStatus() == SSS_IDLE || serverscanner.getStatus() == SSS_PINGING {
			serverscanner.setStatus(SSS_SCANNING)
		}
		serverscanner.updated = true
	}

	serverscanner.mutex.Unlock()
}

func ServerScanner_RescanAll(serverscanner *ServerScanner) {
	if serverscanner == nil {
		return
	}

	serverscanner.mutex.Lock()

	for priv := serverscanner.qwservers; priv != nil; priv = priv.next {
		if priv.pub.status == QWSS_DONE {
			priv.pub.status = QWSS_WAITING
			priv.nextscanwaiting = serverscanner.qwserversscanwaiting
			serverscanner.qwserversscanwaiting = priv
		}
	}

	if serverscanner.getStatus() == SSS_IDLE || serverscanner.getStatus() == SSS_PINGING {
		serverscanner.setStatus(SSS_SCANNING)
	}
	serverscanner.updated = true

	serverscanner.mutex.Unlock()
}

func ServerScanner_GetStatus(serverscanner *ServerScanner) ServerScannerStatus {
	return serverscanner.getStatus()
}

// Copyright (C) 2026 classicQ
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
	"strings"
	"sync/atomic"
)

// httplist.c: server list fallback fetched over plain HTTP.

const HTTPLIST_TIMEOUT = 1500000
const HTTPLIST_MAXRESPONSE = 256 * 1024

// Numeric only, never resolves
func HTTPList_ParseIPv4(token string, addr *netaddr) bool {
	*addr = netaddr{}

	p := 0
	digit := func() bool { return p < len(token) && token[p] >= '0' && token[p] <= '9' }

	for i := 0; i < 4; i++ {
		if !digit() {
			return false
		}

		value := uint(0)
		for digit() {
			value = value*10 + uint(token[p]-'0')
			if value > 255 {
				return false
			}
			p++
		}

		addr.ipv4.address[i] = byte(value)

		if i < 3 {
			if p >= len(token) || token[p] != '.' {
				return false
			}
			p++
		}
	}

	if p >= len(token) || token[p] != ':' {
		return false
	}

	p++
	if !digit() {
		return false
	}

	value := uint(0)
	for digit() {
		value = value*10 + uint(token[p]-'0')
		if value > 65535 {
			return false
		}
		p++
	}

	if p != len(token) || value == 0 {
		return false
	}

	addr.type_ = NA_IPV4
	addr.ipv4.port = uint16(value)

	return true
}

// Accepts any body containing quoted or line separated "a.b.c.d:port" entries
func HTTPList_ParseBody(body string, addrs []netaddr) uint {
	var addr netaddr
	count := uint(0)
	maxaddrs := uint(len(addrs))

	p := 0
	for p < len(body) && count < maxaddrs {
		for p < len(body) && !(body[p] >= '0' && body[p] <= '9') {
			p++
		}

		start := p
		for p < len(body) && ((body[p] >= '0' && body[p] <= '9') || body[p] == '.' || body[p] == ':') {
			p++
		}

		if l := p - start; l >= 9 && l < 24 {
			if HTTPList_ParseIPv4(body[start:p], &addr) {
				i := uint(0)
				for ; i < count; i++ {
					if NET_CompareAdr(&addrs[i], &addr) {
						break
					}
				}

				if i == count {
					addrs[count] = addr
					count++
				}
			}
		}
	}

	return count
}

func HTTPList_Fetch(netdata *SysNetData, url string, addrs []netaddr, quit *atomic.Uint32, deadline uint64) uint {
	var hostaddr netaddr

	if !strings.HasPrefix(url, "http://") {
		return 0
	}

	url = url[7:]

	path := "/"
	hostlen := len(url)
	if i := strings.IndexByte(url, '/'); i >= 0 {
		hostlen = i
		if i+1 < len(url) {
			path = url[i:]
		}
	}

	if hostlen == 0 || hostlen >= 256 {
		return 0
	}

	host := url[:hostlen]

	var resolvehost string
	if strings.IndexByte(host, ':') >= 0 {
		resolvehost = snprintf(264, "%s", host)
	} else {
		resolvehost = snprintf(264, "%s:80", host)
	}

	if quit.Load() != 0 || Sys_IntTime() >= deadline {
		return 0
	}

	if !NET_StringToAdr(netdata, resolvehost, &hostaddr) {
		return 0
	}

	if quit.Load() != 0 || Sys_IntTime() >= deadline {
		return 0
	}

	s := Sys_Net_TCPConnect(netdata, &hostaddr, HTTPLIST_TIMEOUT)
	if s == nil {
		return 0
	}

	count := uint(0)

	request := snprintf(512, "GET %s HTTP/1.0\r\nHost: %s\r\nUser-Agent: classicQ\r\nConnection: close\r\n\r\n", path, host)

	if Sys_Net_TCPSend(netdata, s, []byte(request)) > 0 {
		response := make([]byte, HTTPLIST_MAXRESPONSE+1)
		total := 0
		closed := false
		for total < HTTPLIST_MAXRESPONSE && quit.Load() == 0 && Sys_IntTime() < deadline {
			r := Sys_Net_TCPReceive(netdata, s, response[total:HTTPLIST_MAXRESPONSE])
			if r <= 0 {
				if r == 0 {
					closed = true
				}
				break
			}

			total += r
		}

		// the C string ends at the first NUL
		text := string(response[:total])
		if i := strings.IndexByte(text, 0); i >= 0 {
			text = text[:i]
		}

		if total > 12 && strings.HasPrefix(text, "HTTP/1.") {
			sp := strings.IndexByte(text, ' ')
			bodyat := strings.Index(text, "\r\n\r\n")
			if sp >= 0 && strings.HasPrefix(text[sp+1:], "200") && bodyat >= 0 {
				headers := text[:bodyat]
				body := text[bodyat+4:]

				havecontentlength := false
				contentlength := uint64(0)
				if i := Util_strcasestr(headers, "\ncontent-length:"); i >= 0 {
					havecontentlength = true
					contentlength = strtoul(headers[i+16:], 10)
				}

				complete := closed
				if havecontentlength {
					complete = uint64(total-(bodyat+4)) == contentlength
				}
				if complete {
					count = HTTPList_ParseBody(body, addrs)
				}
			}
		}
	}

	Sys_Net_TCPClose(netdata, s)

	return count
}

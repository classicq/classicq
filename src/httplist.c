/*
Copyright (C) 2026 classicQ

This program is free software; you can redistribute it and/or
modify it under the terms of the GNU General Public License
as published by the Free Software Foundation; either version 2
of the License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.

See the GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program; if not, write to the Free Software
Foundation, Inc., 59 Temple Place - Suite 330, Boston, MA  02111-1307, USA.
*/

#include <stdlib.h>
#include <string.h>
#include <stdio.h>

#include "sys_net.h"
#include "net.h"
#include "utils.h"
#include "httplist.h"

unsigned long long Sys_IntTime(void);

#define HTTPLIST_TIMEOUT 1500000
#define HTTPLIST_MAXRESPONSE (256*1024)

/* Numeric only, never resolves */
static int HTTPList_ParseIPv4(const char *token, struct netaddr *addr)
{
	unsigned int value;
	unsigned int i;
	const char *p;

	memset(addr, 0, sizeof(*addr));

	p = token;
	for(i=0;i<4;i++)
	{
		if (*p < '0' || *p > '9')
			return 0;

		value = 0;
		while(*p >= '0' && *p <= '9')
		{
			value = value * 10 + (*p - '0');
			if (value > 255)
				return 0;

			p++;
		}

		addr->addr.ipv4.address[i] = value;

		if (i < 3)
		{
			if (*p != '.')
				return 0;

			p++;
		}
	}

	if (*p != ':')
		return 0;

	p++;
	if (*p < '0' || *p > '9')
		return 0;

	value = 0;
	while(*p >= '0' && *p <= '9')
	{
		value = value * 10 + (*p - '0');
		if (value > 65535)
			return 0;

		p++;
	}

	if (*p != 0 || value == 0)
		return 0;

	addr->type = NA_IPV4;
	addr->addr.ipv4.port = value;

	return 1;
}

/* Accepts any body containing quoted or line separated "a.b.c.d:port" entries */
static unsigned int HTTPList_ParseBody(const char *body, struct netaddr *addrs, unsigned int maxaddrs)
{
	struct netaddr addr;
	char token[24];
	const char *p;
	const char *start;
	unsigned int len;
	unsigned int i;
	unsigned int count;

	count = 0;

	p = body;
	while(*p && count < maxaddrs)
	{
		while(*p && !(*p >= '0' && *p <= '9'))
			p++;

		start = p;
		while((*p >= '0' && *p <= '9') || *p == '.' || *p == ':')
			p++;

		len = p - start;
		if (len >= 9 && len < sizeof(token))
		{
			memcpy(token, start, len);
			token[len] = 0;

			if (HTTPList_ParseIPv4(token, &addr))
			{
				for(i=0;i<count;i++)
				{
					if (NET_CompareAdr(&addrs[i], &addr))
						break;
				}

				if (i == count)
					addrs[count++] = addr;
			}
		}
	}

	return count;
}

unsigned int HTTPList_Fetch(struct SysNetData *netdata, const char *url, struct netaddr *addrs, unsigned int maxaddrs, const volatile unsigned int *quit, unsigned long long deadline)
{
	struct SysTCPSocket *s;
	struct netaddr hostaddr;
	char host[256];
	char resolvehost[264];
	char request[512];
	char *response;
	char *body;
	char *headers;
	char *p;
	const char *path;
	unsigned int hostlen;
	unsigned int total;
	unsigned int count;
	unsigned int closed;
	unsigned long contentlength;
	int havecontentlength;
	int r;

	if (strncmp(url, "http://", 7) != 0)
		return 0;

	url += 7;

	path = strchr(url, '/');
	hostlen = path ? (unsigned int)(path - url) : strlen(url);
	if (path == 0 || *path == 0)
		path = "/";

	if (hostlen == 0 || hostlen >= sizeof(host))
		return 0;

	memcpy(host, url, hostlen);
	host[hostlen] = 0;

	if (strchr(host, ':'))
		snprintf(resolvehost, sizeof(resolvehost), "%s", host);
	else
		snprintf(resolvehost, sizeof(resolvehost), "%s:80", host);

	if (*quit || Sys_IntTime() >= deadline)
		return 0;

	if (!NET_StringToAdr(netdata, resolvehost, &hostaddr))
		return 0;

	if (*quit || Sys_IntTime() >= deadline)
		return 0;

	s = Sys_Net_TCPConnect(netdata, &hostaddr, HTTPLIST_TIMEOUT);
	if (s == 0)
		return 0;

	count = 0;

	snprintf(request, sizeof(request), "GET %s HTTP/1.0\r\nHost: %s\r\nUser-Agent: classicQ\r\nConnection: close\r\n\r\n", path, host);

	if (Sys_Net_TCPSend(netdata, s, request, strlen(request)) > 0)
	{
		response = malloc(HTTPLIST_MAXRESPONSE + 1);
		if (response)
		{
			total = 0;
			closed = 0;
			while(total < HTTPLIST_MAXRESPONSE && !*quit && Sys_IntTime() < deadline)
			{
				r = Sys_Net_TCPReceive(netdata, s, response + total, HTTPLIST_MAXRESPONSE - total);
				if (r <= 0)
				{
					if (r == 0)
						closed = 1;

					break;
				}

				total += r;
			}

			response[total] = 0;

			if (total > 12 && strncmp(response, "HTTP/1.", 7) == 0)
			{
				p = strchr(response, ' ');
				body = strstr(response, "\r\n\r\n");
				if (p && strncmp(p + 1, "200", 3) == 0 && body)
				{
					*body = 0;
					headers = response;
					body += 4;

					havecontentlength = 0;
					contentlength = 0;
					p = Util_strcasestr(headers, "\ncontent-length:");
					if (p)
					{
						havecontentlength = 1;
						contentlength = strtoul(p + 16, 0, 10);
					}

					if (havecontentlength ? (total - (body - response) == contentlength) : closed)
						count = HTTPList_ParseBody(body, addrs, maxaddrs);
				}
			}

			free(response);
		}
	}

	Sys_Net_TCPClose(netdata, s);

	return count;
}

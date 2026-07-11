#ifndef HTTPLIST_H
#define HTTPLIST_H

#include "sys_net.h"

unsigned int HTTPList_Fetch(struct SysNetData *netdata, const char *url, struct netaddr *addrs, unsigned int maxaddrs, const volatile unsigned int *quit, unsigned long long deadline);

#endif

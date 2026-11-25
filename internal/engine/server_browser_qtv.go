package engine

import (
	"sync"
	"sync/atomic"
)

// server_browser_qtv.c: asks the QTV lookup service for the stream of a
// server. The thread is a goroutine, Sys_Thread_DeleteThread is waiting on
// the channel.

type QTVR_Status int32

const (
	QTVR_WAITING_FOR_REPLY QTVR_Status = iota
	QTVR_REPLY_RECIEVED
	QTVR_ERROR
)

type qtvr_t struct {
	thread chan struct{}
	mutex  sync.Mutex

	status atomic.Int32

	retval  string
	request string
	server  string

	quit atomic.Int32
}

func (q *qtvr_t) getStatus() QTVR_Status {
	return QTVR_Status(q.status.Load())
}

// the reply, with false where C returns a null pointer
func QTVR_Get_Retval(qtvr *qtvr_t) (string, bool) {
	ret := ""
	ok := false

	qtvr.mutex.Lock()
	if qtvr.getStatus() == QTVR_REPLY_RECIEVED {
		ret = qtvr.retval
		ok = true
	}
	qtvr.mutex.Unlock()

	return ret, ok
}

func QTVR_Waiting(qtvr *qtvr_t) bool {
	return qtvr.getStatus() == QTVR_WAITING_FOR_REPLY
}

func QTVR_Thread(qtvr *qtvr_t) {
	var buf [128]byte
	var serveraddr, addr netaddr

	netdata := Sys_Net_Init()

	if netdata == nil {
		Com_Printf("QTVR_Thread: Sys_Net_Init failed.\n")
	} else {
		if !NET_StringToAdr(netdata, qtvr.server, &serveraddr) {
			Com_Printf("QTVR_Thread: Unable to parse/resolve hoststring.\n")
		} else {
			socket := Sys_Net_CreateSocket(netdata, serveraddr.type_)

			if socket == nil {
				Com_Printf("QTVR_Create: could not create socket.\n")
			} else {
				Sys_Net_Send(netdata, socket, []byte(qtvr.request), &serveraddr)

				for qtvr.quit.Load() == 0 {
					Sys_Net_Wait(netdata, socket, 50000)

					if r := Sys_Net_Receive(netdata, socket, buf[:len(buf)-1], &addr); r > 0 {
						qtvr.mutex.Lock()
						qtvr.retval = cbufStr(buf[:r], 0)
						qtvr.status.Store(int32(QTVR_REPLY_RECIEVED))
						qtvr.mutex.Unlock()

						break
					}
				}

				Sys_Net_DeleteSocket(netdata, socket)
			}
		}

		Sys_Net_Shutdown(netdata)
	}

	if qtvr.getStatus() != QTVR_REPLY_RECIEVED {
		qtvr.status.Store(int32(QTVR_ERROR))
	}
}

func QTVR_Create(server, request string) *qtvr_t {
	qtvr := &qtvr_t{server: server, request: request}

	qtvr.status.Store(int32(QTVR_WAITING_FOR_REPLY))

	qtvr.thread = make(chan struct{})
	go func() {
		QTVR_Thread(qtvr)
		close(qtvr.thread)
	}()

	return qtvr
}

func QTVR_Destroy(qtvr *qtvr_t) {
	qtvr.quit.Store(1)
	<-qtvr.thread
}

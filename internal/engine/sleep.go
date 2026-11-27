// Copyright (C) 2007 Mark Olsen
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
	"math"
	"runtime"
)

// sleep.c: frame limiter sleep that measures how coarse the system sleep is
// and spins for the remainder

var sleep_granularity uint

const NUMSAMPLES = 50

// abs(sleeptime - avg) of C: the unsigned difference is truncated to int
// before abs
func sleepDist(t uint64, avg uint) uint {
	d := int32(uint32(t - uint64(avg)))
	if d < 0 {
		d = -d
	}
	return uint(d)
}

func get_sleep_granularity() int {
	var sleeptime [NUMSAMPLES]uint64
	var avg uint
	var stddev float64

	samples := uint(NUMSAMPLES)

	for i := 0; i < NUMSAMPLES; i++ {
		Sys_MicroSleep(1)
		basetime := Sys_IntTime()
		Sys_MicroSleep(1)
		sleeptime[i] = Sys_IntTime() - basetime
	}

	for {
		var accum uint64
		for i := uint(0); i < samples; i++ {
			accum += sleeptime[i]
		}

		avg = uint(accum / uint64(samples))

		stddev = 0
		for i := uint(0); i < samples; i++ {
			tmpf := float64(sleeptime[i]) - float64(avg)
			tmpf *= tmpf
			stddev += tmpf
		}
		stddev = math.Sqrt(stddev / float64(samples))

		maxdist := uint(0)
		for i := uint(0); i < samples; i++ {
			if sleepDist(sleeptime[i], avg) > maxdist {
				maxdist = sleepDist(sleeptime[i], avg)
			}
		}

		for i := uint(0); i < samples; i++ {
			if sleepDist(sleeptime[i], avg) == maxdist {
				copy(sleeptime[i:samples-1], sleeptime[i+1:samples])
				samples--
				break
			}
		}

		if !(stddev > float64(avg)*0.01 && avg > 500) {
			break
		}
	}

	if samples < NUMSAMPLES/5 {
		// system timing too unstable, assume 8ms granularity
		avg = 8000
	}

	return int(avg)
}

func Sleep_Init() {
	sleep_granularity = uint(get_sleep_granularity())

	Com_Printf("System sleep granularity: %d us\n", sleep_granularity)

	if runtime.GOOS == "linux" {
		sleep_granularity += sleep_granularity
	}

	sleep_granularity += sleep_granularity * 2 / 100
}

func Sleep_Sleep(sleeptime uint) {
	if Dump_Active() {
		return
	}

	endtime := Sys_IntTime()

	endtime += uint64(sleeptime)

	if sleeptime >= sleep_granularity {
		real_sleep_time := sleeptime - sleep_granularity

		Sys_MicroSleep(real_sleep_time)
	}

	for Sys_IntTime() < endtime {
	}
}

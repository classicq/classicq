// Copyright (C) 2026 classicQ contributors
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

// The UCRT rand(), whose exact sequence matters.

const RAND_MAX = 0x7fff

var rand_seed uint32 = 1

// UCRT generator: seed * 214013 + 2531011, upper bits out.
func srand(seed uint32) {
	rand_seed = seed
}

func rand() int {
	rand_seed = rand_seed*214013 + 2531011
	return int((rand_seed >> 16) & 0x7fff)
}

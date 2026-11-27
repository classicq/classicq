// Copyright (C) 1996-1997 Id Software, Inc.
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

// r_mesh.c

func MakeCollisionMap(hdr *aliashdr_t, collisionrevmap_ret *[]uint16, backside_ret *[]byte) {
	collisionmap := make([]uint16, hdr.numverts)
	collisionrevmap := make([]uint16, hdr.numverts)
	backside := make([]byte, hdr.numverts)

	collisions := uint32(0)

	for i := range backside {
		backside[i] = 3
	}

	for tri := 0; tri < hdr.numtris; tri++ {
		for i := 0; i < 3; i++ {
			vert := triangles[tri].vertindex[i]
			var new_ byte
			if triangles[tri].facesfront == 0 && stverts[vert].onseam != 0 {
				new_ = 1
			} else {
				new_ = 0
			}

			if backside[vert] == 3 {
				backside[vert] = new_
			} else if backside[vert] != 2 && backside[vert] != new_ {
				collisionmap[collisions] = uint16(vert)
				collisionrevmap[vert] = uint16(hdr.numverts + int(collisions))
				backside[vert] = 2
				collisions++
			}
		}
	}

	hdr.totalverts = uint16(hdr.numverts + int(collisions))
	hdr.collisions = collisions
	hdr.collisionmap = make([]uint16, collisions)
	copy(hdr.collisionmap, collisionmap[:collisions])

	*collisionrevmap_ret = collisionrevmap
	*backside_ret = backside
}

func MakeIndices(hdr *aliashdr_t, collisionrevmap []uint16, backside []byte) {
	indices := make([]uint16, hdr.numtris*3)

	min_ := uint16(65535)
	max_ := uint16(0)
	for tri := 0; tri < hdr.numtris; tri++ {
		for i := 0; i < 3; i++ {
			vert := uint16(triangles[tri].vertindex[i])

			if triangles[tri].facesfront == 0 && stverts[vert].onseam != 0 && backside[vert] != 1 {
				vert = collisionrevmap[vert]
			}

			indices[tri*3+i] = vert

			if vert > max_ {
				max_ = vert
			}
			if vert < min_ {
				min_ = vert
			}
		}
	}

	hdr.indices = indices
	hdr.indexmin = min_
	hdr.indexmax = max_
}

func MakeTextureCoordinates(hdr *aliashdr_t, backside []byte) {
	texcoords := make([]float32, int(hdr.totalverts)*2)

	vert := 0
	for ; vert < hdr.numverts; vert++ {
		s := float32(stverts[vert].s)
		t := float32(stverts[vert].t)
		if backside[vert] == 1 {
			s += float32(hdr.skinwidth / 2)
		}

		texcoords[vert*2+0] = float32((float64(s) + 0.5) / float64(hdr.skinwidth))
		texcoords[vert*2+1] = float32((float64(t) + 0.5) / float64(hdr.skinheight))
	}

	for ; vert < int(hdr.totalverts); vert++ {
		s := float32(stverts[hdr.collisionmap[vert-hdr.numverts]].s)
		t := float32(stverts[hdr.collisionmap[vert-hdr.numverts]].t)
		s += float32(hdr.skinwidth / 2)

		texcoords[vert*2+0] = float32((float64(s) + 0.5) / float64(hdr.skinwidth))
		texcoords[vert*2+1] = float32((float64(t) + 0.5) / float64(hdr.skinheight))
	}

	hdr.texcoords = texcoords
}

func R_MakeAliasModelDisplayLists(m *model_t, hdr *aliashdr_t) {
	var collisionrevmap []uint16
	var backside []byte

	paliashdr := hdr

	MakeCollisionMap(hdr, &collisionrevmap, &backside)
	MakeIndices(hdr, collisionrevmap, backside)
	MakeTextureCoordinates(hdr, backside)

	// poseverts still point at the raw file bytes, 4 per vertex
	paliashdr.realposeverts = make([]trivertx_t, paliashdr.numverts*paliashdr.numposes)
	for i := 0; i < paliashdr.numposes; i++ {
		src := poseverts[i]
		dst := paliashdr.realposeverts[i*paliashdr.numverts:]
		for j := 0; j < paliashdr.numverts; j++ {
			dst[j] = trivertx_t{v: [3]byte{src[j*4], src[j*4+1], src[j*4+2]}, lightnormalindex: src[j*4+3]}
		}
	}
}

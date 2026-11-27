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

var box_hull hull_t
var box_clipnodes [6]mclipnode_t
var box_planes [6]mplane_t

// Set up the planes and clipnodes so that the six floats of a bounding box
// can just be stored out and get a proper hull_t structure.
func PM_InitBoxHull() {
	box_hull.clipnodes = box_clipnodes[:]
	box_hull.planes = box_planes[:]
	box_hull.firstclipnode = 0
	box_hull.lastclipnode = 5

	for i := 0; i < 6; i++ {
		box_clipnodes[i].planenum = i

		side := i & 1

		box_clipnodes[i].children[side] = CONTENTS_EMPTY
		if i != 5 {
			box_clipnodes[i].children[side^1] = i + 1
		} else {
			box_clipnodes[i].children[side^1] = CONTENTS_SOLID
		}

		box_planes[i].type_ = byte(i >> 1)
		box_planes[i].normal[i>>1] = 1
	}
}

// To keep everything totally uniform, bounding boxes are turned into small
// BSP trees instead of being compared directly.
func PM_HullForBox(mins, maxs vec3_t) *hull_t {
	box_planes[0].dist = maxs[0]
	box_planes[1].dist = mins[0]
	box_planes[2].dist = maxs[1]
	box_planes[3].dist = mins[1]
	box_planes[4].dist = maxs[2]
	box_planes[5].dist = mins[2]

	return &box_hull
}

func PM_HullPointContents(hull *hull_t, num int, p vec3_t) int {
	for num >= 0 {
		if num < hull.firstclipnode || num > hull.lastclipnode {
			Sys_Error("PM_HullPointContents: bad node number")
		}

		node := &hull.clipnodes[num]
		plane := &hull.planes[node.planenum]

		d := PlaneDiff(p, plane)
		if d < 0 {
			num = node.children[1]
		} else {
			num = node.children[0]
		}
	}

	return num
}

func PM_PointContents(p vec3_t) int {
	hull := &pmove.physents[0].model.hulls[0]

	num := hull.firstclipnode

	for num >= 0 {
		if num < hull.firstclipnode || num > hull.lastclipnode {
			Sys_Error("PM_HullPointContents: bad node number")
		}

		node := &hull.clipnodes[num]
		plane := &hull.planes[node.planenum]

		d := PlaneDiff(p, plane)
		if d < 0 {
			num = node.children[1]
		} else {
			num = node.children[0]
		}
	}

	return num
}

/*
===============================================================================
LINE TESTING IN HULLS
===============================================================================
*/

// 1/32 epsilon to keep floating point happy
const DIST_EPSILON = 0.03125

func PM_RecursiveHullCheck(hull *hull_t, num int, p1f, p2f float32, p1, p2 vec3_t, trace *pmtrace_t) bool {
	var t1, t2, frac, midf float32
	var mid vec3_t

	// check for empty
	if num < 0 {
		if num != CONTENTS_SOLID {
			trace.allsolid = false
			if num == CONTENTS_EMPTY {
				trace.inopen = true
			} else {
				trace.inwater = true
			}
		} else {
			trace.startsolid = true
		}
		return true // empty
	}

	if num < hull.firstclipnode || num > hull.lastclipnode {
		Sys_Error("PM_RecursiveHullCheck: bad node number")
	}

	// find the point distances
	node := &hull.clipnodes[num]
	plane := &hull.planes[node.planenum]

	if plane.type_ < 3 {
		t1 = p1[plane.type_] - plane.dist
		t2 = p2[plane.type_] - plane.dist
	} else {
		t1 = DotProduct(plane.normal, p1) - plane.dist
		t2 = DotProduct(plane.normal, p2) - plane.dist
	}

	if t1 >= 0 && t2 >= 0 {
		return PM_RecursiveHullCheck(hull, node.children[0], p1f, p2f, p1, p2, trace)
	}
	if t1 < 0 && t2 < 0 {
		return PM_RecursiveHullCheck(hull, node.children[1], p1f, p2f, p1, p2, trace)
	}

	// put the crosspoint DIST_EPSILON pixels on the near side
	if t1 < 0 {
		frac = float32((float64(t1) + DIST_EPSILON) / float64(t1-t2))
	} else {
		frac = float32((float64(t1) - DIST_EPSILON) / float64(t1-t2))
	}
	frac = bound(0, frac, 1)

	midf = p1f + (p2f-p1f)*frac
	for i := 0; i < 3; i++ {
		mid[i] = p1[i] + frac*(p2[i]-p1[i])
	}

	side := b2i(t1 < 0)

	// move up to the node
	if !PM_RecursiveHullCheck(hull, node.children[side], p1f, midf, p1, mid, trace) {
		return false
	}

	if PM_HullPointContents(hull, node.children[side^1], mid) != CONTENTS_SOLID { // go past the node
		return PM_RecursiveHullCheck(hull, node.children[side^1], midf, p2f, mid, p2, trace)
	}

	if trace.allsolid {
		return false // never got out of the solid area
	}

	// the other side of the node is solid, this is the impact point
	if side == 0 {
		trace.plane.normal = plane.normal
		trace.plane.dist = plane.dist
	} else {
		VectorNegate(plane.normal, &trace.plane.normal)
		trace.plane.dist = -plane.dist
	}

	trace.draw_plane = plane

	for PM_HullPointContents(hull, hull.firstclipnode, mid) == CONTENTS_SOLID {
		// shouldn't really happen, but does occasionally
		frac = float32(float64(frac) - 0.1)
		if frac < 0 {
			trace.fraction = midf
			trace.endpos = mid
			Com_DPrintf("backup past 0\n")
			return false
		}
		midf = p1f + (p2f-p1f)*frac
		for i := 0; i < 3; i++ {
			mid[i] = p1[i] + frac*(p2[i]-p1[i])
		}
	}

	trace.fraction = midf
	trace.endpos = mid

	return false
}

// Returns false if the given player position is not valid (in solid)
func PM_TestPlayerPosition(pos vec3_t) bool {
	var mins, maxs, pos_l, offset vec3_t
	var hull *hull_t

	for i := 0; i < pmove.numphysent; i++ {
		pe := &pmove.physents[i]
		// get the clipping hull
		if pe.model != nil {
			hull = &pmove.physents[i].model.hulls[1]
			VectorSubtract(hull.clip_mins, player_mins, &offset)
			VectorAdd(offset, pe.origin, &offset)
			VectorSubtract(pos, offset, &pos_l)
		} else {
			VectorSubtract(pe.mins, player_maxs, &mins)
			VectorSubtract(pe.maxs, player_mins, &maxs)
			hull = PM_HullForBox(mins, maxs)
			VectorSubtract(pos, pe.origin, &pos_l)
		}

		if PM_HullPointContents(hull, hull.firstclipnode, pos_l) == CONTENTS_SOLID {
			return false
		}
	}

	return true
}

func PM_PlayerTrace(start, end vec3_t) pmtrace_t {
	var trace, total pmtrace_t
	var offset, start_l, end_l, mins, maxs vec3_t
	var hull *hull_t

	// fill in a default trace
	total = pmtrace_t{}
	total.fraction = 1
	total.ent = -1
	total.endpos = end

	for i := 0; i < pmove.numphysent; i++ {
		pe := &pmove.physents[i]
		// get the clipping hull
		if pe.model != nil {
			hull = &pmove.physents[i].model.hulls[1]
			VectorSubtract(hull.clip_mins, player_mins, &offset)
			VectorAdd(offset, pe.origin, &offset)
		} else {
			VectorSubtract(pe.mins, player_maxs, &mins)
			VectorSubtract(pe.maxs, player_mins, &maxs)
			hull = PM_HullForBox(mins, maxs)
			offset = pe.origin
		}

		VectorSubtract(start, offset, &start_l)
		VectorSubtract(end, offset, &end_l)

		// fill in a default trace
		trace = pmtrace_t{}
		trace.fraction = 1
		trace.allsolid = true
		trace.endpos = end

		// trace a line through the appropriate clipping hull
		PM_RecursiveHullCheck(hull, hull.firstclipnode, 0, 1, start_l, end_l, &trace)

		if trace.allsolid {
			trace.startsolid = true
		}
		if trace.startsolid {
			trace.fraction = 0
		}

		// did we clip the move?
		if trace.fraction < total.fraction {
			// fix trace up by the offset
			VectorAdd(trace.endpos, offset, &trace.endpos)
			total = trace
			total.ent = i
		}
	}

	return total
}

// FIXME: merge with PM_PlayerTrace (PM_Move?)
func PM_TraceLine(start, end vec3_t) pmtrace_t {
	var trace, total pmtrace_t
	var offset, start_l, end_l vec3_t
	var hull *hull_t

	// fill in a default trace
	total = pmtrace_t{}
	total.fraction = 1
	total.ent = -1
	total.endpos = end

	for i := 0; i < pmove.numphysent; i++ {
		pe := &pmove.physents[i]
		// get the clipping hull
		if pe.model != nil {
			hull = &pmove.physents[i].model.hulls[0]
		} else {
			hull = PM_HullForBox(pe.mins, pe.maxs)
		}

		offset = pe.origin

		VectorSubtract(start, offset, &start_l)
		VectorSubtract(end, offset, &end_l)

		// fill in a default trace
		trace = pmtrace_t{}
		trace.fraction = 1
		trace.allsolid = true
		trace.endpos = end

		// trace a line through the apropriate clipping hull
		PM_RecursiveHullCheck(hull, hull.firstclipnode, 0, 1, start_l, end_l, &trace)

		if trace.allsolid {
			trace.startsolid = true
		}
		if trace.startsolid {
			trace.fraction = 0
		}

		// did we clip the move?
		if trace.fraction < total.fraction {
			// fix trace up by the offset
			VectorAdd(trace.endpos, offset, &trace.endpos)
			total = trace
			total.ent = i
		}
	}

	return total
}

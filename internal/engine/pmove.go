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

import "math"

// pmove.h

type pmplane_t struct {
	normal vec3_t
	dist   float32
}

type pmtrace_t struct {
	allsolid   bool // if true, plane is not valid
	startsolid bool // if true, the initial point was in a solid area
	inopen     bool
	inwater    bool
	fraction   float32   // time completed, 1.0 = didn't hit anything
	endpos     vec3_t    // final position
	plane      pmplane_t // surface normal at impact
	ent        int       // entity the surface is on
	draw_plane *mplane_t
}

const MAX_PHYSENTS = 64

type physent_t struct {
	origin     vec3_t
	model      *model_t // only for bsp models
	mins, maxs vec3_t   // only for non-bsp models
	info       int      // for client or server to identify
}

type pmtype_t int

const (
	PM_NORMAL        pmtype_t = iota // normal ground movement
	PM_OLD_SPECTATOR                 // fly, no clip to world (QW bug)
	PM_SPECTATOR                     // fly, no clip to world
	PM_DEAD                          // no acceleration
	PM_FLY                           // fly, bump into walls
)

type playermove_t struct {
	// player state
	origin        vec3_t
	angles        vec3_t
	velocity      vec3_t
	jump_held     bool
	jump_msec     int // msec since last jump
	waterjumptime float32
	pm_type       pmtype_t

	// world state
	numphysent int
	physents   [MAX_PHYSENTS]physent_t // 0 should be the world

	// input
	cmd usercmd_t

	// results
	numtouch   int
	touchindex [MAX_PHYSENTS]int
	onground   bool
	groundent  int // index in physents array, only valid when onground is true
	waterlevel int
	watertype  int
}

type movevars_t struct {
	gravity           float32
	stopspeed         float32
	maxspeed          float32
	spectatormaxspeed float32
	accelerate        float32
	airaccelerate     float32
	wateraccelerate   float32
	friction          float32
	waterfriction     float32
	entgravity        float32
	bunnyspeedcap     float32
	ktjump            float32
	slidefix          int
	airstep           int
}

// pmove.c

var movevars movevars_t
var pmove playermove_t

var pm_frametime float32

var pm_forward, pm_right, pm_up vec3_t

var player_mins = vec3_t{-16, -16, -24}
var player_maxs = vec3_t{16, 16, 32}

const (
	STEPSIZE        = 18
	MIN_STEP_NORMAL = 0.7 // roughly 45 degrees
	STOP_EPSILON    = 0.1

	pm_flyfriction = 4

	BLOCKED_FLOOR = 1
	BLOCKED_STEP  = 2
	BLOCKED_OTHER = 4
	BLOCKED_ANY   = 7
)

func PM_Init() {
	PM_InitBoxHull()
}

// Slide off of the impacting object
// returns the blocked flags (1 = floor, 2 = step / wall)
func PM_ClipVelocity(in, normal vec3_t, out *vec3_t, overbounce float32) int {
	blocked := 0
	if normal[2] > 0 {
		blocked |= BLOCKED_FLOOR // floor
	} else if normal[2] == 0 {
		blocked |= BLOCKED_STEP // step
	}

	backoff := DotProduct(in, normal) * overbounce

	for i := 0; i < 3; i++ {
		change := normal[i] * backoff
		out[i] = in[i] - change
		if float64(out[i]) > -STOP_EPSILON && float64(out[i]) < STOP_EPSILON {
			out[i] = 0
		}
	}

	return blocked
}

const MAX_CLIP_PLANES = 5

// The basic solid body movement clip that slides along multiple planes
func PM_SlideMove() int {
	var i, j int
	var dir, primal_velocity, original_velocity, end vec3_t
	var planes [MAX_CLIP_PLANES]vec3_t
	var trace pmtrace_t

	numbumps := 4

	blocked := 0
	original_velocity = pmove.velocity
	primal_velocity = pmove.velocity
	numplanes := 0

	time_left := pm_frametime

	for bumpcount := 0; bumpcount < numbumps; bumpcount++ {
		VectorMA(pmove.origin, time_left, pmove.velocity, &end)
		trace = PM_PlayerTrace(pmove.origin, end)

		if trace.startsolid || trace.allsolid {
			// entity is trapped in another solid
			pmove.velocity = vec3_t{}
			return 3
		}

		if trace.fraction > 0 {
			// actually covered some distance
			pmove.origin = trace.endpos
			numplanes = 0
		}

		if trace.fraction == 1 {
			break // moved the entire distance
		}

		// save entity for contact
		if pmove.numtouch < MAX_PHYSENTS {
			pmove.touchindex[pmove.numtouch] = trace.ent
			pmove.numtouch++
		}

		if float64(trace.plane.normal[2]) >= MIN_STEP_NORMAL {
			blocked |= BLOCKED_FLOOR
		} else if trace.plane.normal[2] == 0 {
			blocked |= BLOCKED_STEP
		} else {
			blocked |= BLOCKED_OTHER
		}

		time_left -= time_left * trace.fraction

		// cliped to another plane
		if numplanes >= MAX_CLIP_PLANES {
			// this shouldn't really happen
			pmove.velocity = vec3_t{}
			break
		}

		planes[numplanes] = trace.plane.normal
		numplanes++

		// modify original_velocity so it parallels all of the clip planes
		for i = 0; i < numplanes; i++ {
			PM_ClipVelocity(original_velocity, planes[i], &pmove.velocity, 1)
			for j = 0; j < numplanes; j++ {
				if j != i {
					if DotProduct(pmove.velocity, planes[j]) < 0 {
						break // not ok
					}
				}
			}
			if j == numplanes {
				break
			}
		}

		if i != numplanes {
			// go along this plane
		} else {
			// go along the crease
			if numplanes != 2 {
				pmove.velocity = vec3_t{}
				break
			}
			CrossProduct(planes[0], planes[1], &dir)
			d := DotProduct(dir, pmove.velocity)
			VectorScale(dir, d, &pmove.velocity)
		}

		// if velocity is against the original velocity, stop dead to avoid tiny occilations in sloping corners
		if DotProduct(pmove.velocity, primal_velocity) <= 0 {
			pmove.velocity = vec3_t{}
			break
		}
	}

	if pmove.waterjumptime != 0 {
		pmove.velocity = primal_velocity
	}
	return blocked
}

// Each intersection will try to step over the obstruction instead of sliding along it.
func PM_StepSlideMove(in_air bool) int {
	var dest vec3_t
	var trace pmtrace_t
	var original, originalvel, down, up, downvel vec3_t
	var downdist, updist, stepsize float32

	// try sliding forward both on ground and up 16 pixels
	// take the move that goes farthest
	original = pmove.origin
	originalvel = pmove.velocity

	blocked := PM_SlideMove()

	if blocked == 0 {
		return blocked // moved the entire distance
	}

	if in_air {
		// don't let us step up unless it's indeed a step we bumped in
		// (that is, there's solid ground below)
		if blocked&BLOCKED_STEP == 0 {
			return blocked
		}

		var org vec3_t
		if originalvel[2] < 0 {
			org = pmove.origin
		} else {
			org = original
		}
		dest = org
		dest[2] -= STEPSIZE
		trace = PM_PlayerTrace(org, dest)
		if trace.fraction == 1 || float64(trace.plane.normal[2]) < MIN_STEP_NORMAL {
			return blocked
		}

		// adjust stepsize, otherwise it would be possible to walk up
		// a step higher than STEPSIZE
		stepsize = STEPSIZE - (org[2] - trace.endpos[2])
	} else {
		stepsize = STEPSIZE
	}

	down = pmove.origin
	downvel = pmove.velocity

	pmove.origin = original
	pmove.velocity = originalvel

	// move up a stair height
	dest = pmove.origin
	dest[2] += stepsize
	trace = PM_PlayerTrace(pmove.origin, dest)
	if !trace.startsolid && !trace.allsolid {
		pmove.origin = trace.endpos
	}

	if in_air && originalvel[2] < 0 {
		pmove.velocity[2] = 0
	}

	PM_SlideMove()

	// press down the stepheight
	dest = pmove.origin
	dest[2] -= stepsize
	trace = PM_PlayerTrace(pmove.origin, dest)
	usedown := false
	if trace.fraction != 1 && float64(trace.plane.normal[2]) < MIN_STEP_NORMAL {
		usedown = true
	} else {
		if !trace.startsolid && !trace.allsolid {
			pmove.origin = trace.endpos
		}

		if pmove.origin[2] < original[2] {
			usedown = true
		} else {
			up = pmove.origin

			// decide which one went farther
			downdist = (down[0]-original[0])*(down[0]-original[0]) + (down[1]-original[1])*(down[1]-original[1])
			updist = (up[0]-original[0])*(up[0]-original[0]) + (up[1]-original[1])*(up[1]-original[1])

			if downdist >= updist {
				usedown = true
			}
		}
	}

	if usedown {
		pmove.origin = down
		pmove.velocity = downvel
		return blocked
	}

	// copy z value from slide move
	pmove.velocity[2] = downvel[2]

	if !pmove.onground && pmove.waterlevel < 2 && blocked&BLOCKED_STEP != 0 {
		// airstep: 16-unit step kills 16% horizontal velocity
		scale := float32(1 - 0.01*float64(pmove.origin[2]-original[2]))
		pmove.velocity[0] *= scale
		pmove.velocity[1] *= scale
	}

	return blocked
}

// Handles both ground friction and water friction
func PM_Friction() {
	var drop float32
	var start, stop vec3_t

	if pmove.waterjumptime != 0 {
		return
	}

	speed := VectorLength(pmove.velocity)
	if speed < 1 {
		pmove.velocity[0] = 0
		pmove.velocity[1] = 0
		if pmove.pm_type == PM_FLY {
			pmove.velocity[2] = 0
		}
		return
	}

	if pmove.waterlevel >= 2 {
		// apply water friction, even if in fly mode
		drop = speed * movevars.waterfriction * float32(pmove.waterlevel) * pm_frametime
	} else if pmove.pm_type == PM_FLY {
		// apply flymode friction
		drop = speed * pm_flyfriction * pm_frametime
	} else if pmove.onground {
		// apply ground friction
		friction := movevars.friction

		// if the leading edge is over a dropoff, increase friction
		if pmove.onground {
			start[0] = pmove.origin[0] + pmove.velocity[0]/speed*16
			stop[0] = start[0]
			start[1] = pmove.origin[1] + pmove.velocity[1]/speed*16
			stop[1] = start[1]
			start[2] = pmove.origin[2] + player_mins[2]
			stop[2] = start[2] - 34

			trace := PM_PlayerTrace(start, stop)

			if trace.fraction == 1 {
				friction *= 2
			}
		}

		var control float32
		if speed < movevars.stopspeed {
			control = movevars.stopspeed
		} else {
			control = speed
		}
		drop = control * friction * pm_frametime
	} else {
		return // in air, no friction
	}

	// scale the velocity
	newspeed := speed - drop
	newspeed = max(newspeed, 0)
	newspeed /= speed

	VectorScale(pmove.velocity, newspeed, &pmove.velocity)
}

func PM_Accelerate(wishdir vec3_t, wishspeed, accel float32) {
	if pmove.pm_type == PM_DEAD {
		return
	}
	if pmove.waterjumptime != 0 {
		return
	}

	currentspeed := DotProduct(pmove.velocity, wishdir)
	addspeed := wishspeed - currentspeed
	if addspeed <= 0 {
		return
	}
	accelspeed := accel * pm_frametime * wishspeed
	if accelspeed > addspeed {
		accelspeed = addspeed
	}

	VectorMA(pmove.velocity, accelspeed, wishdir, &pmove.velocity)
}

func PM_AirAccelerate(wishdir vec3_t, wishspeed, accel float32) {
	var originalspeed float32
	wishspd := wishspeed

	if pmove.pm_type == PM_DEAD {
		return
	}
	if pmove.waterjumptime != 0 {
		return
	}

	if movevars.bunnyspeedcap > 0 {
		originalspeed = float32(math.Sqrt(float64(pmove.velocity[0]*pmove.velocity[0] + pmove.velocity[1]*pmove.velocity[1])))
	}

	wishspd = min(wishspd, 30)
	currentspeed := DotProduct(pmove.velocity, wishdir)
	addspeed := wishspd - currentspeed
	if addspeed <= 0 {
		return
	}
	accelspeed := accel * wishspeed * pm_frametime
	accelspeed = min(accelspeed, addspeed)

	VectorMA(pmove.velocity, accelspeed, wishdir, &pmove.velocity)

	if movevars.bunnyspeedcap > 0 {
		newspeed := float32(math.Sqrt(float64(pmove.velocity[0]*pmove.velocity[0] + pmove.velocity[1]*pmove.velocity[1])))
		if newspeed > originalspeed {
			speedcap := movevars.maxspeed * movevars.bunnyspeedcap
			if newspeed > speedcap {
				if originalspeed < speedcap {
					originalspeed = speedcap
				}
				pmove.velocity[0] *= originalspeed / newspeed
				pmove.velocity[1] *= originalspeed / newspeed
			}
		}
	}
}

func PM_WaterMove() {
	var wishvel, wishdir vec3_t

	// user intentions
	for i := 0; i < 3; i++ {
		wishvel[i] = pm_forward[i]*float32(pmove.cmd.forwardmove) + pm_right[i]*float32(pmove.cmd.sidemove)
	}

	if pmove.pm_type != PM_FLY && pmove.cmd.forwardmove == 0 && pmove.cmd.sidemove == 0 && pmove.cmd.upmove == 0 {
		wishvel[2] -= 60 // drift towards bottom
	} else {
		wishvel[2] += float32(pmove.cmd.upmove)
	}

	wishdir = wishvel
	wishspeed := VectorNormalize(&wishdir)

	if wishspeed > movevars.maxspeed {
		VectorScale(wishvel, movevars.maxspeed/wishspeed, &wishvel)
		wishspeed = movevars.maxspeed
	}
	wishspeed = float32(float64(wishspeed) * 0.7)

	// water acceleration
	PM_Accelerate(wishdir, wishspeed, movevars.wateraccelerate)

	PM_StepSlideMove(false)
}

func PM_FlyMove() {
	var wishvel, wishdir vec3_t

	for i := 0; i < 3; i++ {
		wishvel[i] = pm_forward[i]*float32(pmove.cmd.forwardmove) + pm_right[i]*float32(pmove.cmd.sidemove)
	}

	wishvel[2] += float32(pmove.cmd.upmove)

	wishdir = wishvel
	wishspeed := VectorNormalize(&wishdir)

	if wishspeed > movevars.maxspeed {
		VectorScale(wishvel, movevars.maxspeed/wishspeed, &wishvel)
		wishspeed = movevars.maxspeed
	}

	PM_Accelerate(wishdir, wishspeed, movevars.accelerate)

	PM_StepSlideMove(false)
}

func PM_AirMove() {
	var wishvel, wishdir vec3_t

	fmove := float32(pmove.cmd.forwardmove)
	smove := float32(pmove.cmd.sidemove)

	pm_forward[2] = 0
	pm_right[2] = 0
	VectorNormalize(&pm_forward)
	VectorNormalize(&pm_right)

	for i := 0; i < 2; i++ {
		wishvel[i] = pm_forward[i]*fmove + pm_right[i]*smove
	}
	wishvel[2] = 0

	wishdir = wishvel
	wishspeed := VectorNormalize(&wishdir)

	// clamp to server defined max speed
	if wishspeed > movevars.maxspeed {
		VectorScale(wishvel, movevars.maxspeed/wishspeed, &wishvel)
		wishspeed = movevars.maxspeed
	}

	if pmove.onground {
		if pmove.velocity[2] > 0 || movevars.slidefix == 0 {
			pmove.velocity[2] = 0
		}
		PM_Accelerate(wishdir, wishspeed, movevars.accelerate)
		pmove.velocity[2] -= movevars.entgravity * movevars.gravity * pm_frametime

		if movevars.slidefix == 0 {
			pmove.velocity[2] = 0
		}

		if pmove.velocity[0] == 0 && pmove.velocity[1] == 0 {
			pmove.velocity[2] = 0
			return
		}

		PM_StepSlideMove(false)
	} else {
		// not on ground, so little effect on velocity
		PM_AirAccelerate(wishdir, wishspeed, movevars.accelerate)

		// add gravity
		pmove.velocity[2] -= movevars.entgravity * movevars.gravity * pm_frametime

		if movevars.airstep != 0 {
			PM_StepSlideMove(true)
		} else {
			PM_SlideMove()
		}
	}
}

var groundplane pmplane_t

func PM_CategorizePosition() {
	var point vec3_t
	var trace pmtrace_t

	// if the player hull point one unit down is solid, the player is on ground

	// see if standing on something solid
	point[0] = pmove.origin[0]
	point[1] = pmove.origin[1]
	point[2] = pmove.origin[2] - 1
	if pmove.velocity[2] > 180 {
		pmove.onground = false
	} else {
		trace = PM_PlayerTrace(pmove.origin, point)
		if trace.fraction == 1 || float64(trace.plane.normal[2]) < MIN_STEP_NORMAL {
			pmove.onground = false
		} else {
			pmove.onground = true
			pmove.groundent = trace.ent
			groundplane = trace.plane
			pmove.waterjumptime = 0
		}

		// standing on an entity other than the world
		if trace.ent > 0 {
			if pmove.numtouch < MAX_PHYSENTS {
				pmove.touchindex[pmove.numtouch] = trace.ent
				pmove.numtouch++
			}
		}
	}

	// get waterlevel
	pmove.waterlevel = 0
	pmove.watertype = CONTENTS_EMPTY

	point[2] = pmove.origin[2] + player_mins[2] + 1
	cont := PM_PointContents(point)

	if cont <= CONTENTS_WATER {
		pmove.watertype = cont
		pmove.waterlevel = 1
		point[2] = float32(float64(pmove.origin[2]) + float64(player_mins[2]+player_maxs[2])*0.5)
		cont = PM_PointContents(point)
		if cont <= CONTENTS_WATER {
			pmove.waterlevel = 2
			point[2] = pmove.origin[2] + 22
			cont = PM_PointContents(point)
			if cont <= CONTENTS_WATER {
				pmove.waterlevel = 3
			}
		}
	}

	// snap to ground unless in fly mode or underwater
	if pmove.onground && pmove.pm_type != PM_FLY && pmove.waterlevel < 2 {
		if !trace.startsolid && !trace.allsolid {
			pmove.origin = trace.endpos
		}
	}
}

func PM_CheckJump() {
	if pmove.pm_type == PM_FLY {
		return
	}

	if pmove.pm_type == PM_DEAD {
		pmove.jump_held = true // don't jump on respawn
		return
	}

	if pmove.cmd.buttons&BUTTON_JUMP == 0 {
		pmove.jump_held = false
		return
	}

	if pmove.waterjumptime != 0 {
		return
	}

	if pmove.waterlevel >= 2 {
		// swimming, not jumping
		pmove.onground = false

		if pmove.watertype == CONTENTS_WATER {
			pmove.velocity[2] = 100
		} else if pmove.watertype == CONTENTS_SLIME {
			pmove.velocity[2] = 80
		} else {
			pmove.velocity[2] = 50
		}
		return
	}

	if !pmove.onground {
		return // in air, so no effect
	}

	if pmove.jump_held && pmove.jump_msec == 0 {
		return // don't pogo stick
	}

	// check for jump bug
	// groundplane normal was set in the call to PM_CategorizePosition
	if pmove.velocity[2] < 0 && float64(DotProduct(pmove.velocity, groundplane.normal)) < -0.1 {
		// pmove.velocity is pointing into the ground, clip it
		PM_ClipVelocity(pmove.velocity, groundplane.normal, &pmove.velocity, 1)
	}

	pmove.onground = false
	pmove.velocity[2] += 270

	if movevars.ktjump > 0 {
		if movevars.ktjump > 1 {
			movevars.ktjump = 1
		}
		if pmove.velocity[2] < 270 {
			pmove.velocity[2] = pmove.velocity[2]*(1-movevars.ktjump) + 270*movevars.ktjump
		}
	}

	pmove.jump_held = true // don't jump again until released

	pmove.jump_msec = int(pmove.cmd.msec)
}

func PM_CheckWaterJump() {
	var spot vec3_t
	var flatforward vec3_t

	if pmove.waterjumptime != 0 {
		return
	}

	// don't hop out if we just jumped in
	if pmove.velocity[2] < -180 {
		return
	}

	// see if near an edge
	flatforward[0] = pm_forward[0]
	flatforward[1] = pm_forward[1]
	flatforward[2] = 0
	VectorNormalize(&flatforward)

	VectorMA(pmove.origin, 24, flatforward, &spot)
	spot[2] += 8
	cont := PM_PointContents(spot)
	if cont != CONTENTS_SOLID {
		return
	}
	spot[2] += 24
	cont = PM_PointContents(spot)
	if cont != CONTENTS_EMPTY {
		return
	}
	// jump out of water
	VectorScale(flatforward, 50, &pmove.velocity)
	pmove.velocity[2] = 310
	pmove.waterjumptime = 2 // safety net
	pmove.jump_held = true  // don't jump again until released
}

var pm_nudge_sign = [3]int{0, -1, 1}

// If pmove.origin is in a solid position, try nudging slightly on all axis to allow for the cut precision of the net coordinates
func PM_NudgePosition() {
	base := pmove.origin

	for i := 0; i < 3; i++ {
		pmove.origin[i] = float32(float64(int(pmove.origin[i]*8)) * 0.125)
	}

	for z := 0; z <= 2; z++ {
		for y := 0; y <= 2; y++ {
			for x := 0; x <= 2; x++ {
				pmove.origin[0] = float32(float64(base[0]) + float64(pm_nudge_sign[x])*0.125)
				pmove.origin[1] = float32(float64(base[1]) + float64(pm_nudge_sign[y])*0.125)
				pmove.origin[2] = float32(float64(base[2]) + float64(pm_nudge_sign[z])*0.125)
				if PM_TestPlayerPosition(pmove.origin) {
					return
				}
			}
		}
	}
	pmove.origin = base
}

func PM_SpectatorMove() {
	var wishvel, wishdir vec3_t

	// friction
	speed := VectorLength(pmove.velocity)
	if speed < 1 {
		pmove.velocity = vec3_t{}
	} else {
		friction := float32(float64(movevars.friction) * 1.5) // extra friction
		var control float32
		if speed < movevars.stopspeed {
			control = movevars.stopspeed
		} else {
			control = speed
		}
		drop := control * friction * pm_frametime

		// scale the velocity
		newspeed := speed - drop
		if newspeed < 0 {
			newspeed = 0
		}
		newspeed /= speed

		VectorScale(pmove.velocity, newspeed, &pmove.velocity)
	}

	// accelerate
	fmove := float32(pmove.cmd.forwardmove)
	smove := float32(pmove.cmd.sidemove)

	VectorNormalize(&pm_forward)
	VectorNormalize(&pm_right)

	for i := 0; i < 3; i++ {
		wishvel[i] = pm_forward[i]*fmove + pm_right[i]*smove
	}
	wishvel[2] += float32(pmove.cmd.upmove)

	wishdir = wishvel
	wishspeed := VectorNormalize(&wishdir)

	// clamp to server defined max speed
	if wishspeed > movevars.spectatormaxspeed {
		VectorScale(wishvel, movevars.spectatormaxspeed/wishspeed, &wishvel)
		wishspeed = movevars.spectatormaxspeed
	}

	currentspeed := DotProduct(pmove.velocity, wishdir)
	addspeed := wishspeed - currentspeed

	// Buggy QW spectator mode, kept for compatibility
	if pmove.pm_type == PM_OLD_SPECTATOR {
		if addspeed <= 0 {
			return
		}
	}

	if addspeed > 0 {
		accelspeed := movevars.accelerate * pm_frametime * wishspeed
		accelspeed = min(accelspeed, addspeed)
		VectorMA(pmove.velocity, accelspeed, wishdir, &pmove.velocity)
	}

	// move
	VectorMA(pmove.origin, pm_frametime, pmove.velocity, &pmove.origin)
}

// Returns with origin, angles, and velocity modified in place.
// Numtouch and touchindex[] will be set if any of the physents were contacted during the move.
func PM_PlayerMove() {
	pm_frametime = float32(float64(pmove.cmd.msec) * 0.001)
	pmove.numtouch = 0

	// take angles directly from command
	pmove.angles = pmove.cmd.angles
	AngleVectors(pmove.angles, &pm_forward, &pm_right, &pm_up)

	if pmove.pm_type == PM_SPECTATOR || pmove.pm_type == PM_OLD_SPECTATOR {
		PM_SpectatorMove()
		pmove.onground = false
		return
	}

	PM_NudgePosition()

	// set onground, watertype, and waterlevel
	PM_CategorizePosition()

	if pmove.waterlevel == 2 && pmove.pm_type != PM_FLY {
		PM_CheckWaterJump()
	}

	if pmove.velocity[2] < 0 || pmove.pm_type == PM_DEAD {
		pmove.waterjumptime = 0
	}

	if pmove.waterjumptime != 0 {
		pmove.waterjumptime -= pm_frametime
		if pmove.waterjumptime < 0 {
			pmove.waterjumptime = 0
		}
	}

	if pmove.jump_msec != 0 {
		pmove.jump_msec += int(pmove.cmd.msec)
		if pmove.jump_msec > 50 {
			pmove.jump_msec = 0
		}
	}

	PM_CheckJump()

	PM_Friction()

	if pmove.waterlevel >= 2 {
		PM_WaterMove()
	} else if pmove.pm_type == PM_FLY {
		PM_FlyMove()
	} else {
		PM_AirMove()
	}

	// set onground, watertype, and waterlevel for final spot
	PM_CategorizePosition()

	// this is to make sure landing sound is not played twice
	// and falling damage is calculated correctly
	if pmove.onground && pmove.velocity[2] < -300 && float64(DotProduct(pmove.velocity, groundplane.normal)) < -0.1 {
		PM_ClipVelocity(pmove.velocity, groundplane.normal, &pmove.velocity, 1)
	}
}

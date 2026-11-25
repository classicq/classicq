package engine

import "math"

var cl_nopred = cvar_t{name: "cl_nopred", string: "0"}
var cl_pushlatency = cvar_t{name: "pushlatency", string: "-999", OnChange: OnChange_cl_pushlatency}

func OnChange_cl_pushlatency(cvar *cvar_t, value string) bool {
	newvalue := Q_atof(value)
	newvalue = min(newvalue, 0)
	Cvar_SetValue(cvar, newvalue)
	return true
}

func CL_PredictUsercmd(from, to *player_state_t, u *usercmd_t) {
	// split up very long moves
	if u.msec > 50 {
		var temp player_state_t

		split := *u
		split.msec /= 2

		CL_PredictUsercmd(from, &temp, &split)
		CL_PredictUsercmd(&temp, to, &split)
		return
	}

	pmove.origin = from.origin
	pmove.angles = u.angles
	pmove.velocity = from.velocity

	if cl.z_ext&Z_EXT_PM_TYPE != 0 {
		pmove.jump_msec = 0
	} else {
		pmove.jump_msec = from.jump_msec
	}
	pmove.jump_held = from.jump_held
	pmove.waterjumptime = from.waterjumptime
	pmove.pm_type = pmtype_t(from.pm_type)
	pmove.onground = from.onground

	pmove.cmd = *u

	movevars.entgravity = cl.entgravity
	movevars.maxspeed = cl.maxspeed
	movevars.bunnyspeedcap = cl.bunnyspeedcap

	PM_PlayerMove()

	to.waterjumptime = pmove.waterjumptime
	to.jump_held = pmove.jump_held
	to.jump_msec = pmove.jump_msec
	pmove.jump_msec = 0

	to.origin = pmove.origin
	to.viewangles = pmove.angles
	to.velocity = pmove.velocity
	to.onground = pmove.onground

	to.weaponframe = from.weaponframe
	to.pm_type = from.pm_type
}

// Used when cl_nopred is 1 to determine whether we are on ground, otherwise stepup smoothing code produces ugly jump physics
func CL_CategorizePosition() {
	if cl.spectator != 0 && cl.playernum == cl.viewplayernum {
		cl.onground = false // in air
		return
	}
	pmove.velocity = vec3_t{}
	pmove.origin = cl.simorg
	pmove.numtouch = 0
	PM_CategorizePosition()
	cl.onground = pmove.onground
}

var (
	crouch_oldorigin   vec3_t
	crouch_oldz        float32
	crouch_extracrouch float32
	crouch_crouchspeed float32 = 100
)

// Smooth out stair step ups.
// Called before CL_EmitEntities so that the player's lightning model origin is updated properly
func CL_CalcCrouch() {
	teleported := !VectorL2Compare(cl.simorg, crouch_oldorigin, 48)
	crouch_oldorigin = cl.simorg

	if teleported {
		// possibly teleported or respawned
		crouch_oldz = cl.simorg[2]
		crouch_extracrouch = 0
		crouch_crouchspeed = 100
		cl.crouch = 0
		crouch_oldorigin = cl.simorg
		return
	}

	if cl.onground && cl.simorg[2]-crouch_oldz > 0 {
		if cl.simorg[2]-crouch_oldz > 20 {
			// if on steep stairs, increase speed
			if crouch_crouchspeed < 160 {
				crouch_extracrouch = float32(float64(cl.simorg[2]-crouch_oldz) - cls.frametime*200 - 15)
				crouch_extracrouch = min(crouch_extracrouch, 5)
			}
			crouch_crouchspeed = 160
		}

		crouch_oldz = float32(float64(crouch_oldz) + cls.frametime*float64(crouch_crouchspeed))
		if crouch_oldz > cl.simorg[2] {
			crouch_oldz = cl.simorg[2]
		}

		if cl.simorg[2]-crouch_oldz > 15+crouch_extracrouch {
			crouch_oldz = cl.simorg[2] - 15 - crouch_extracrouch
		}
		crouch_extracrouch = float32(float64(crouch_extracrouch) - cls.frametime*200)
		crouch_extracrouch = max(crouch_extracrouch, 0)

		cl.crouch = crouch_oldz - cl.simorg[2]
	} else {
		// in air or moving down
		crouch_oldz = cl.simorg[2]
		cl.crouch = float32(float64(cl.crouch) + cls.frametime*150)
		if cl.crouch > 0 {
			cl.crouch = 0
		}
		crouch_crouchspeed = 100
		crouch_extracrouch = 0
	}
}

// INTERPOLATEDPHYSICS is always on, as in the NETQW build.

var (
	lerp_last_seq    int
	lerp_times       [3]float64
	lerp_origin      [3]vec3_t
	lerp_nolerp      [2]bool
	lerp_nolerp_next bool
	lerp_latency     float64 = 0.01
)

func CL_LerpMove() {
	var from, to int

	seq := cls.netchan.outgoing_sequence
	physframe := seq != lerp_last_seq

	if seq < lerp_last_seq || seq-lerp_last_seq > UPDATE_BACKUP/2 {
		lerp_last_seq = seq
		lerp_times[0] = 0
		lerp_times[1] = 0
		lerp_times[2] = 0
		lerp_latency = 0.01
		lerp_nolerp[0] = false
		lerp_nolerp[1] = false
		lerp_nolerp_next = false
		return
	}

	if physframe {
		lerp_last_seq = seq

		lerp_times[2] = lerp_times[1]
		lerp_times[1] = lerp_times[0]
		lerp_times[0] = cl.frames[(seq-1)&UPDATE_MASK].senttime

		lerp_origin[2] = lerp_origin[1]
		lerp_origin[1] = lerp_origin[0]
		lerp_origin[0] = cl.simorg

		lerp_nolerp[1] = lerp_nolerp[0]
		lerp_nolerp[0] = lerp_nolerp_next
		lerp_nolerp_next = false

		for i := 0; i < 3; i++ {
			if math.Abs(float64(lerp_origin[0][i]-lerp_origin[1][i])) > 100 {
				lerp_nolerp[0] = true
				break
			}
		}
	}

	if lerp_times[2] <= 0 {
		return
	}

	simtime := cls.realtime - lerp_latency

	if simtime > lerp_times[0] {
		lerp_latency = cls.realtime - lerp_times[0]
	} else if simtime < lerp_times[2] {
		lerp_latency = cls.realtime - lerp_times[2]
	} else if physframe {
		lerp_latency -= min((lerp_times[0]-lerp_times[1])*0.005, 0.001)
	}

	if simtime > lerp_times[1] {
		from = 1
		to = 0
	} else {
		from = 2
		to = 1
	}

	if lerp_nolerp[to] {
		return
	}

	span := lerp_times[to] - lerp_times[from]
	if span <= 0 {
		return
	}

	for i := 0; i < 3; i++ {
		cl.simorg[i] = float32(float64(lerp_origin[from][i]) +
			float64(lerp_origin[to][i]-lerp_origin[from][i])*bound(0.0, (simtime-lerp_times[from])/span, 1.0))
	}
}

func cl_predictMoveOut() {
	CL_CalcCrouch()
	cl.waterlevel = pmove.waterlevel
}

func CL_PredictMove() {
	if cl.paused != 0 {
		return
	}

	if cl.intermission != 0 {
		cl.crouch = 0
		return
	}

	if cl.validsequence == 0 {
		return
	}

	if cls.netchan.outgoing_sequence-cl.validsequence >= UPDATE_BACKUP-1 {
		return
	}

	cl.simangles = cl.viewangles

	// this is the last valid frame received from the server
	to := &cl.frames[cl.validsequence&UPDATE_MASK]

	// FIXME...
	if cls.demoplayback && cl.spectator != 0 && cl.viewplayernum != cl.playernum {
		cl.simvel = to.playerstate[cl.viewplayernum].velocity
		cl.simorg = to.playerstate[cl.viewplayernum].origin
		cl.simangles = to.playerstate[cl.viewplayernum].viewangles
		CL_CategorizePosition()
		cl_predictMoveOut()
		return
	}

	if cl_nopred.value != 0 && !cls.mvdplayback {
		cl.simvel = to.playerstate[cl.playernum].velocity
		cl.simorg = to.playerstate[cl.playernum].origin
		CL_CategorizePosition()
		cl_predictMoveOut()
		return
	}

	oldphysent := pmove.numphysent
	CL_SetSolidPlayers(cl.playernum)

	// run frames until to->senttime >= playertime

	if cl.validsequence >= 0 {
		for i := cl.validsequence; i < cls.netchan.outgoing_sequence-1; i++ {
			from := &cl.frames[i&UPDATE_MASK]
			to = &cl.frames[(i+1)&UPDATE_MASK]
			CL_PredictUsercmd(&from.playerstate[cl.playernum], &to.playerstate[cl.playernum], &to.cmd)
		}
	}

	to = &cl.frames[(cls.netchan.outgoing_sequence-1)&UPDATE_MASK]
	pmove.numphysent = oldphysent
	cl.simorg = to.playerstate[cl.playernum].origin
	cl.simvel = to.playerstate[cl.playernum].velocity
	cl.onground = pmove.onground
	CL_LerpMove()
	cl_predictMoveOut()
}

func CL_CvarInitPrediction() {
	Cvar_SetCurrentGroup(CVAR_GROUP_NETWORK)
	Cvar_Register(&cl_nopred)
	Cvar_Register(&cl_pushlatency)

	Cvar_ResetCurrentGroup()
}

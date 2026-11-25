package engine

import (
	"fmt"
	"math"
)

// ZOID: Player camera tracking in Spectator mode
//
// This takes over player controls for spectator automatic camera.
// Player moves as a spectator, but the camera tracks and enemy player

var desired_position vec3_t // where the camera wants to be
var cam_locked = false
var oldbuttons int

var cl_chasecam = cvar_t{name: "cl_chasecam", string: "1"} // "through the eyes" view

var cam_lastviewtime float64

var spec_track = 0 // player# of who we are tracking
var autocam = CAM_NONE
var ideal_track = 0
var last_lock float32

func vectoangles(vec vec3_t, ang *vec3_t) {
	var yaw, pitch float32

	if vec[1] == 0 && vec[0] == 0 {
		yaw = 0
		if vec[2] > 0 {
			pitch = 90
		} else {
			pitch = 270
		}
	} else {
		yaw = float32(int(math.Atan2(float64(vec[1]), float64(vec[0])) * 180 / M_PI))
		if yaw < 0 {
			yaw += 360
		}

		forward := float32(math.Sqrt(float64(vec[0]*vec[0] + vec[1]*vec[1])))
		pitch = float32(int(math.Atan2(float64(vec[2]), float64(forward)) * 180 / M_PI))
		if pitch < 0 {
			pitch += 360
		}
	}

	ang[0] = pitch
	ang[1] = yaw
	ang[2] = 0
}

func Cam_SetViewPlayer() {
	if cl.spectator != 0 && autocam != 0 && cam_locked && cl_chasecam.value != 0 {
		cl.viewplayernum = spec_track
	} else {
		cl.viewplayernum = cl.playernum
	}
}

// returns true if weapon model should be drawn in camera mode
func Cam_DrawViewModel() bool {
	if cl.spectator == 0 {
		return true
	}

	if autocam != 0 && cam_locked && cl_chasecam.value != 0 {
		return true
	}
	return false
}

// returns true if we should draw this player, we don't if we are chase camming
func Cam_DrawPlayer(playernum int) bool {
	if cl.spectator != 0 && autocam != 0 && cam_locked && cl_chasecam.value != 0 &&
		spec_track == playernum {
		return false
	}
	return true
}

func Cam_Unlock() {
	if autocam == 0 {
		return
	}

	if cls.netqw != nil {
		st := fmt.Sprintf("%cptrack", clc_stringcmd)
		if len(st) < 40 {
			NetQW_AppendReliableBuffer(cls.netqw, append([]byte(st), 0))
		}
		NetQW_UnlockMovement(cls.netqw)
	}

	autocam = CAM_NONE
	cam_locked = false
	Sbar_Changed()
}

func Cam_Lock(playernum int) {
	if cls.mvdplayback {
		cl.stats = cl.players[playernum].stats
		ideal_track = playernum
	}
	last_lock = float32(cls.realtime)

	if cls.netqw != nil {
		st := fmt.Sprintf("%cptrack %d", clc_stringcmd, playernum)
		if len(st) < 40 {
			NetQW_AppendReliableBuffer(cls.netqw, append([]byte(st), 0))
		}
		NetQW_LockMovement(cls.netqw)
	}

	spec_track = playernum
	cam_locked = false
	Sbar_Changed()
}

func Cam_DoTrace(vec1, vec2 vec3_t) pmtrace_t {
	pmove.origin = vec1
	return PM_PlayerTrace(pmove.origin, vec2)
}

// Returns distance or 9999 if invalid for some reason; vec comes back as
// the end position of the trace.
func Cam_TryFlyby(self, player *player_state_t, vec *vec3_t, checkvis bool) float32 {
	var v vec3_t

	vectoangles(*vec, &v)
	pmove.angles = v
	VectorNormalizeFast(vec)
	VectorMA(player.origin, 800, *vec, &v)
	// v is endpos
	// fake a player move
	trace := Cam_DoTrace(player.origin, v)
	if trace.inwater {
		return 9999
	}
	*vec = trace.endpos
	VectorSubtract(trace.endpos, player.origin, &v)
	length := VectorLength(v)
	if length < 32 || length > 800 {
		return 9999
	}
	if checkvis {
		VectorSubtract(trace.endpos, self.origin, &v)
		length = VectorLength(v)

		trace = Cam_DoTrace(self.origin, *vec)
		if trace.fraction != 1 || trace.inwater {
			return 9999
		}
	}
	return length
}

// Is player visible?
func Cam_IsVisible(player *player_state_t, vec vec3_t) bool {
	var v vec3_t

	trace := Cam_DoTrace(player.origin, vec)
	if trace.fraction != 1 || trace.inwater {
		return false
	}
	// check distance, don't let the player get too far away or too close
	VectorSubtract(player.origin, vec, &v)
	return v[0]*v[0]+v[1]*v[1]+v[2]*v[2] >= 256
}

func InitFlyby(self, player *player_state_t, checkvis bool) bool {
	var vec, vec2 vec3_t
	var forward, right, up vec3_t

	vec = player.viewangles
	vec[0] = 0
	AngleVectors(vec, &forward, &right, &up)

	var max float32 = 1000

	try := func() {
		if f := Cam_TryFlyby(self, player, &vec2, checkvis); f < max {
			max = f
			vec = vec2
		}
	}

	VectorAdd(forward, up, &vec2)
	VectorAdd(vec2, right, &vec2)
	try()

	VectorAdd(forward, up, &vec2)
	VectorSubtract(vec2, right, &vec2)
	try()

	VectorAdd(forward, right, &vec2)
	try()

	VectorSubtract(forward, right, &vec2)
	try()

	VectorAdd(forward, up, &vec2)
	try()

	VectorSubtract(forward, up, &vec2)
	try()

	VectorAdd(up, right, &vec2)
	VectorSubtract(vec2, forward, &vec2)
	try()

	VectorSubtract(up, right, &vec2)
	VectorSubtract(vec2, forward, &vec2)
	try()

	// invert
	VectorSubtract(vec3_origin, forward, &vec2)
	try()

	vec2 = forward
	try()

	// invert
	VectorSubtract(vec3_origin, right, &vec2)
	try()

	vec2 = right
	try()

	// ack, can't find him
	if max >= 1000 {
		return false
	}
	cam_locked = true
	desired_position = vec
	return true
}

// Take over the user controls and track a player.
// We find a nice position to watch the player and move there
func Cam_Track(cmd *usercmd_t) {
	var vec vec3_t

	if cl.spectator == 0 {
		return
	}

	if autocam == 0 || cls.state != ca_active {
		return
	}

	if cam_locked && (cl.players[spec_track].name == "" || cl.players[spec_track].spectator != 0) {
		cam_locked = false
		Cam_Unlock()
		return
	}

	frame := &cl.frames[cl.validsequence&UPDATE_MASK]

	if autocam != 0 && cls.mvdplayback {
		if ideal_track != spec_track && cls.realtime-float64(last_lock) > 0.1 &&
			frame.playerstate[ideal_track].messagenum == cl.parsecount {
			Cam_Lock(ideal_track)
		}

		if frame.playerstate[spec_track].messagenum != cl.parsecount {
			i := 0
			for ; i < MAX_CLIENTS; i++ {
				if frame.playerstate[i].messagenum == cl.parsecount {
					break
				}
			}
			if i < MAX_CLIENTS {
				Cam_Lock(i)
			}
		}
	}

	player := &frame.playerstate[spec_track]
	self := &frame.playerstate[cl.playernum]

	if !cam_locked || !Cam_IsVisible(player, desired_position) {
		if !cam_locked || cls.realtime-cam_lastviewtime > 0.1 {
			if !InitFlyby(self, player, true) {
				InitFlyby(self, player, false)
			}
			cam_lastviewtime = cls.realtime
		}
	} else {
		cam_lastviewtime = cls.realtime
	}

	// couldn't track for some reason
	if !cam_locked || autocam == 0 {
		return
	}

	do_tmove := false
	if cl_chasecam.value != 0 {
		cmd.forwardmove = 0
		cmd.sidemove = 0
		cmd.upmove = 0

		cl.viewangles = player.viewangles
		desired_position = player.origin
		if desired_position != self.origin {
			do_tmove = true
			// move there locally immediately
			self.origin = desired_position
		}
	} else {
		// Ok, move to our desired position and set our angles to view
		// the player
		VectorSubtract(desired_position, self.origin, &vec)
		cmd.forwardmove = 0
		cmd.sidemove = 0
		cmd.upmove = 0
		if VectorLength(vec) > 16 {
			// close enough?
			do_tmove = true
		}
		// move there locally immediately
		self.origin = desired_position

		VectorSubtract(player.origin, desired_position, &vec)
		vectoangles(vec, &cl.viewangles)
		cl.viewangles[0] = -cl.viewangles[0]
		if cl.viewangles[0] < 0 {
			cl.viewangles[0] += 360
		}
	}

	Mouse_SetViewAngles(cl.viewangles)

	if do_tmove && cls.netqw != nil {
		NetQW_SetTeleport(cls.netqw, desired_position)
	}
}

func Cam_FinishMove(cmd *usercmd_t) {
	if cls.state != ca_active {
		return
	}

	if cl.spectator == 0 { // only in spectator mode
		return
	}

	if cmd.buttons&BUTTON_ATTACK != 0 {
		if oldbuttons&BUTTON_ATTACK == 0 {
			oldbuttons |= BUTTON_ATTACK
			autocam++

			if autocam > CAM_TRACK {
				Cam_Unlock()
				cmd.angles = cl.viewangles
				return
			}
		} else {
			return
		}
	} else {
		oldbuttons &^= BUTTON_ATTACK
		if autocam == 0 {
			return
		}
	}

	if cam_locked {
		if cmd.buttons&BUTTON_JUMP != 0 && oldbuttons&BUTTON_JUMP != 0 {
			return // don't pogo stick
		}

		if cmd.buttons&BUTTON_JUMP == 0 {
			oldbuttons &^= BUTTON_JUMP
			return
		}
		oldbuttons |= BUTTON_JUMP // don't jump again until released
	}

	var end int
	if cam_locked && autocam != 0 {
		end = (ideal_track + 1) % MAX_CLIENTS
	} else {
		end = ideal_track
	}
	i := end
	for {
		s := &cl.players[i]
		if s.name != "" && s.spectator == 0 {
			Cam_Lock(i)
			ideal_track = i
			return
		}
		i = (i + 1) % MAX_CLIENTS
		if i == end {
			break
		}
	}
	// stay on same guy?
	i = ideal_track
	s := &cl.players[i]
	if s.name != "" && s.spectator == 0 {
		Cam_Lock(i)
		return
	}
	Com_Printf("No target found ...\n")
	autocam = 0
	cam_locked = false
}

func Cam_Reset() {
	autocam = CAM_NONE
	spec_track = 0
	ideal_track = 0
}

var cam_lastlocktime float32

// Fixes spectator chasecam demos
func Cam_TryLock() {
	if cl.validsequence == 0 {
		return
	}

	if autocam == 0 {
		cam_lastlocktime = 0
	}

	old_autocam := autocam
	old_spec_track := spec_track

	state := &cl.frames[cl.validsequence&UPDATE_MASK].playerstate
	for i := 0; i < MAX_CLIENTS; i++ {
		if cl.players[i].name == "" || cl.players[i].spectator != 0 ||
			state[i].messagenum != cl.parsecount {
			continue
		}

		if math.Abs(float64(state[i].command.angles[0]-cl.viewangles[0])) < 2 && math.Abs(float64(state[i].command.angles[1]-cl.viewangles[1])) < 2 {
			j := 0
			for ; j < 3; j++ {
				if math.Abs(float64(state[i].origin[j]-state[cl.playernum].origin[j])) > 200 {
					break // too far
				}
			}
			if j < 3 {
				continue
			}
			autocam = CAM_TRACK
			spec_track = i
			cam_locked = true
			cam_lastlocktime = float32(cls.realtime)
			break
		}
	}

	if cls.realtime-float64(cam_lastlocktime) > 0.3 {
		// Couldn't lock to any player for 0.3 seconds, so assume the
		// spectator switched to free spectator mode
		autocam = CAM_NONE
		spec_track = 0
		cam_locked = false
	}

	if autocam != old_autocam || spec_track != old_spec_track {
		Sbar_Changed()
	}
}

func CL_CvarInitCam() {
	Cvar_SetCurrentGroup(CVAR_GROUP_SPECTATOR)
	Cvar_Register(&cl_chasecam)
	Cvar_ResetCurrentGroup()

	Cmd_AddCommand("track", CL_Track_f)
}

func CL_Track_f() {
	if cls.state < ca_connected {
		Com_Printf("You must be connected to track\n")
		return
	}

	if cl.spectator == 0 {
		Com_Printf("You can only track in spectator mode\n")
		return
	}

	if Cmd_Argc() != 2 {
		Com_Printf("Usage: %s <name | userid>\n", Cmd_Argv(0))
		return
	}

	arg := Cmd_Argv(1)
	slot := Player_GetSlot(arg)
	if slot == PLAYER_NAME_NOMATCH {
		Com_Printf("%s : no such player %s\n", Cmd_Argv(0), arg)
		return
	} else if slot == PLAYER_ID_NOMATCH {
		Com_Printf("%s : no player with userid %d\n", Cmd_Argv(0), Q_atoi(arg))
		return
	} else if slot < 0 || slot >= MAX_CLIENTS { // PLAYER_NUM_MISMATCH covered by this
		Com_Printf("%s : no such player\n", Cmd_Argv(0))
		return
	}

	if cl.players[slot].spectator != 0 {
		Com_Printf("You cannot track a spectator\n")
	} else if Cam_TrackNum() != slot {
		autocam = CAM_TRACK
		Cam_Lock(slot)
		ideal_track = slot
		cam_locked = true
	}
}

func Cam_TrackNum() int {
	if autocam == 0 {
		return -1
	}
	return spec_track
}

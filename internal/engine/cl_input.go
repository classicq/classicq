package engine

// cl_input.c -- builds an intended movement command to send to the server

// input.h
type kbutton_t struct {
	down  [2]int // key nums holding it down
	state int    // low bit is down state
}

var cl_weaponfire = cvar_t{name: "cl_weaponfire", string: "1"}
var cl_weaponswitch = cvar_t{name: "cl_weaponswitch", string: "2 1"}
var cl_preselectedweapon int

var cl_nodelta = cvar_t{name: "cl_nodelta", string: "0"}
var cl_c2spps = cvar_t{name: "cl_c2spps", string: "0"}
var cl_c2sImpulseBackup = cvar_t{name: "cl_c2sImpulseBackup", string: "3"}
var cl_smartjump = cvar_t{name: "cl_smartjump", string: "1"}

var cl_upspeed = cvar_t{name: "cl_upspeed", string: "400"}
var cl_forwardspeed = cvar_t{name: "cl_forwardspeed", string: "400", flags: CVAR_ARCHIVE}
var cl_backspeed = cvar_t{name: "cl_backspeed", string: "400", flags: CVAR_ARCHIVE}
var cl_sidespeed = cvar_t{name: "cl_sidespeed", string: "400", flags: CVAR_ARCHIVE}

/*
===============================================================================
KEY BUTTONS

Continuous button event tracking is complicated by the fact that two different
input sources (say, mouse button 1 and the control key) can both press the
same button, but the button should only be released when both of the
pressing key have been released.

When a key event issues a button command (+forward, +attack, etc), it appends
its key number as a parameter to the command so it can be matched up with
the release.

state bit 0 is the current state of the key
state bit 1 is edge triggered on the up to down transition
===============================================================================
*/

var in_forward, in_back kbutton_t
var in_moveleft, in_moveright kbutton_t
var in_use, in_jump, in_attack kbutton_t
var in_up, in_down kbutton_t

var in_impulse int
var in_otherimpulse int

func checkmovementruleset() bool {
	// Ugliest hack in the world
	if Ruleset_AllowMovementScripts() {
		return true
	}

	if atoi(Cmd_Argv(2)) != movementkey {
		Com_Printf("Movement scripts disallowed by ruleset\n")
		return false
	}

	return true
}

func KeyDown(b *kbutton_t) {
	var k int

	c := Cmd_Argv(1)
	if c != "" {
		k = atoi(c)
	} else {
		k = -1 // typed manually at the console for continuous down
	}

	if k == b.down[0] || k == b.down[1] {
		return // repeating key
	}

	if b.down[0] == 0 {
		b.down[0] = k
	} else if b.down[1] == 0 {
		b.down[1] = k
	} else {
		Com_Printf("Three keys down for a button!\n")
		return
	}

	if b.state&1 != 0 {
		return // still down
	}
	b.state |= 1 + 2 // down + impulse down
}

func KeyUp(b *kbutton_t) {
	var k int

	c := Cmd_Argv(1)
	if c != "" {
		k = atoi(c)
	} else {
		// typed manually at the console, assume for unsticking, so clear all
		b.down[0] = 0
		b.down[1] = 0
		b.state &^= 1
		return
	}

	if b.down[0] == k {
		b.down[0] = 0
	} else if b.down[1] == k {
		b.down[1] = 0
	} else {
		return // key up without coresponding down (menu pass through)
	}
	if b.down[0] != 0 || b.down[1] != 0 {
		return // some other key is still holding it down
	}

	if b.state&1 == 0 {
		return // still up (this should not happen)
	}
	b.state &^= 1 // now up
}

// Returns 1 if a key is pressed, 0 otherwise.
func CL_KeyState(key *kbutton_t) float32 {
	val := float32(b2i(key.state != 0))
	key.state &= 1 // clear impulses
	return val
}

func UpdateNetQWForwardSpeed() {
	if cls.netqw != nil {
		speed := cl_forwardspeed.value*CL_KeyState(&in_forward) - cl_backspeed.value*CL_KeyState(&in_back)
		NetQW_SetForwardSpeed(cls.netqw, speed)
	}
}

func UpdateNetQWSideSpeed() {
	if cls.netqw != nil {
		speed := cl_sidespeed.value*CL_KeyState(&in_moveright) - cl_sidespeed.value*CL_KeyState(&in_moveleft)
		NetQW_SetSideSpeed(cls.netqw, speed)
	}
}

func UpdateNetQWUpSpeed() {
	if cls.netqw != nil {
		speed := cl_upspeed.value*CL_KeyState(&in_up) - cl_upspeed.value*CL_KeyState(&in_down)
		NetQW_SetUpSpeed(cls.netqw, speed)
	}
}

func IN_UpDown()   { KeyDown(&in_up); UpdateNetQWUpSpeed() }
func IN_UpUp()     { KeyUp(&in_up); UpdateNetQWUpSpeed() }
func IN_DownDown() { KeyDown(&in_down); UpdateNetQWUpSpeed() }
func IN_DownUp()   { KeyUp(&in_down); UpdateNetQWUpSpeed() }

func IN_ForwardDown() {
	if checkmovementruleset() {
		KeyDown(&in_forward)
		UpdateNetQWForwardSpeed()
	}
}

func IN_ForwardUp() {
	if checkmovementruleset() {
		KeyUp(&in_forward)
		UpdateNetQWForwardSpeed()
	}
}

func IN_BackDown() {
	if checkmovementruleset() {
		KeyDown(&in_back)
		UpdateNetQWForwardSpeed()
	}
}

func IN_BackUp() {
	if checkmovementruleset() {
		KeyUp(&in_back)
		UpdateNetQWForwardSpeed()
	}
}

func IN_MoveleftDown() {
	if checkmovementruleset() {
		KeyDown(&in_moveleft)
		UpdateNetQWSideSpeed()
	}
}

func IN_MoveleftUp() {
	if checkmovementruleset() {
		KeyUp(&in_moveleft)
		UpdateNetQWSideSpeed()
	}
}

func IN_MoverightDown() {
	if checkmovementruleset() {
		KeyDown(&in_moveright)
		UpdateNetQWSideSpeed()
	}
}

func IN_MoverightUp() {
	if checkmovementruleset() {
		KeyUp(&in_moveright)
		UpdateNetQWSideSpeed()
	}
}

var idleweaponlist [8]byte

var weaponindex = [8]uint16{
	IT_AXE,
	IT_SHOTGUN,
	IT_SUPER_SHOTGUN,
	IT_NAILGUN,
	IT_SUPER_NAILGUN,
	IT_GRENADE_LAUNCHER,
	IT_ROCKET_LAUNCHER,
	IT_LIGHTNING,
}

var ammoindex = [8]struct {
	statindex byte
	minamount byte
}{
	{STAT_SHELLS, 0}, // hah
	{STAT_SHELLS, 1},
	{STAT_SHELLS, 2},
	{STAT_NAILS, 1},
	{STAT_NAILS, 2},
	{STAT_ROCKETS, 1},
	{STAT_ROCKETS, 1},
	{STAT_CELLS, 1},
}

func CheckWeaponAvailable(index int) bool {
	if index < 1 || index > 8 {
		return false
	}

	index--

	if cl.stats[STAT_ITEMS]&int(weaponindex[index]) != 0 && cl.stats[ammoindex[index].statindex] >= int(ammoindex[index].minamount) {
		return true
	}

	return false
}

func WeaponFallback() {
	weapon := 0
	for i := 0; i < len(idleweaponlist) && idleweaponlist[i] != 0; i++ {
		if CheckWeaponAvailable(int(idleweaponlist[i])) {
			weapon = int(idleweaponlist[i])
			break
		}
	}

	if weapon != 0 {
		if cls.netqw != nil {
			NetQW_SetImpulse(cls.netqw, weapon)
		} else {
			in_otherimpulse = weapon
		}
	}
}

var attack_outgoing_sequence uint

func IN_AttackDown() {
	if cl_preselectedweapon != 0 {
		in_impulse = cl_preselectedweapon
	}

	KeyDown(&in_attack)

	if cls.netqw != nil {
		attack_outgoing_sequence = NetQW_ButtonDown(cls.netqw, 0, cl_preselectedweapon)
	} else {
		attack_outgoing_sequence = uint(cls.netchan.outgoing_sequence)
	}
}

func IN_AttackUp() {
	KeyUp(&in_attack)

	if cl_preselectedweapon != 0 {
		WeaponFallback()
	}

	if cls.netqw != nil {
		NetQW_ButtonUp(cls.netqw, 0)
	}
}

func IN_UseDown() {
	KeyDown(&in_use)

	if cls.netqw != nil {
		NetQW_ButtonDown(cls.netqw, 2, 0)
	}
}

func IN_UseUp() {
	KeyUp(&in_use)

	if cls.netqw != nil {
		NetQW_ButtonUp(cls.netqw, 2)
	}
}

func IN_WeaponDown() {
	if Cmd_Argc() < 2 {
		Com_Printf("Usage: +weapon <weapon preference list>\n")
		return
	}

	idleindex := 0
	weapon := 0

	for index, max := 1, Cmd_Argc(); index < max; index++ {
		num := Q_atoi(Cmd_Argv(index))
		if num < 0 {
			if idleindex < len(idleweaponlist) {
				idleweaponlist[idleindex] = byte(-num)
				idleindex++
			}
		} else {
			if weapon == 0 {
				if CheckWeaponAvailable(num) {
					weapon = num
				}
			}
		}
	}

	if weapon == 0 && (cl_weaponfire.value == 0 || (cl.stats[STAT_HEALTH] > 0 && cl.intermission == 0)) {
		return
	}

	if idleindex == 0 {
		Cmd_TokenizeString(cl_weaponswitch.string)
		for index, max := 0, Cmd_Argc(); index < max; index++ {
			num := Q_atoi(Cmd_Argv(index))
			if idleindex < len(idleweaponlist) {
				idleweaponlist[idleindex] = byte(num)
				idleindex++
			}
		}
	}

	if idleindex < len(idleweaponlist) {
		idleweaponlist[idleindex] = 0
	}

	if cl_weaponfire.value != 0 {
		KeyDown(&in_attack)
		if cls.netqw != nil {
			NetQW_ButtonDown(cls.netqw, 0, weapon)
		} else {
			in_impulse = weapon
		}
	} else {
		cl_preselectedweapon = weapon
	}
}

func IN_WeaponUp() {
	if cl_weaponfire.value == 0 {
		return
	}

	KeyUp(&in_attack)

	if cls.netqw != nil {
		NetQW_ButtonUp(cls.netqw, 0)
	}

	WeaponFallback()
}

func IN_JumpDown() {
	condition := cls.state == ca_active && cl_smartjump.value != 0

	if condition && cl.stats[STAT_HEALTH] > 0 &&
		!cls.demoplayback && cl.spectator == 0 &&
		cl.waterlevel >= 2 &&
		(!cl.teamfortress || in_forward.state&1 == 0) {
		IN_UpDown()
	} else if condition && cl.spectator != 0 && Cam_TrackNum() == -1 {
		IN_UpDown()
	} else {
		if cls.netqw != nil {
			NetQW_ButtonDown(cls.netqw, 1, 0)
		}
		KeyDown(&in_jump)
	}
}

func IN_JumpUp() {
	if cl_smartjump.value != 0 {
		IN_UpUp()
	}

	KeyUp(&in_jump)

	if cls.netqw != nil {
		NetQW_ButtonUp(cls.netqw, 1)
	}
}

// Tonik -->
func IN_Impulse() {
	best := Q_atoi(Cmd_Argv(1))

	if best >= 1 && best <= 8 {
		cl_preselectedweapon = 0
	}

	if Cmd_Argc() > 2 {
		best = 0
		for i := Cmd_Argc() - 1; i > 0; i-- {
			imp := Q_atoi(Cmd_Argv(i))
			if CheckWeaponAvailable(imp) {
				best = imp
			}
		}
	}

	if best != 0 {
		if cls.netqw != nil {
			NetQW_SetImpulse(cls.netqw, best)
		} else {
			in_impulse = best
		}
	}
}

// <-- Tonik

//==========================================================================

func CL_Rotate_f() {
	var angles vec3_t

	if Cmd_Argc() != 2 {
		Com_Printf("Usage: %s <degrees>\n", Cmd_Argv(0))
		return
	}

	if cl.fpd&FPD_LIMIT_YAW != 0 || !Ruleset_AllowRJScripts() {
		return
	}

	angles[PITCH] = 0
	angles[ROLL] = 0
	angles[YAW] = float32(atof(Cmd_Argv(1)))

	Mouse_AddViewAngles(angles)
}

// Send the intended movement message to the server
func CL_BaseMove(cmd *usercmd_t) {
	*cmd = usercmd_t{}

	cmd.angles = cl.viewangles

	cmd.sidemove = int16(float32(cmd.sidemove) + cl_sidespeed.value*CL_KeyState(&in_moveright))
	cmd.sidemove = int16(float32(cmd.sidemove) - cl_sidespeed.value*CL_KeyState(&in_moveleft))

	cmd.upmove = int16(float32(cmd.upmove) + cl_upspeed.value*CL_KeyState(&in_up))
	cmd.upmove = int16(float32(cmd.upmove) - cl_upspeed.value*CL_KeyState(&in_down))

	cmd.forwardmove = int16(float32(cmd.forwardmove) + cl_forwardspeed.value*CL_KeyState(&in_forward))
	cmd.forwardmove = int16(float32(cmd.forwardmove) - cl_backspeed.value*CL_KeyState(&in_back))
}

func MakeShort(i int) int16 {
	if i < -32768 {
		i = -32768
	}
	if i > 32767 {
		i = 32767
	}
	return int16(i)
}

var cl_extramsec float64

func CL_FinishMove(cmd *usercmd_t) {
	var frametime float32

	if Movie_IsCapturing() && movie_steadycam.value != 0 {
		if movie_fps.value > 0 {
			frametime = float32(1.0 / float64(movie_fps.value))
		} else {
			frametime = float32(1 / 30.0)
		}
	} else {
		frametime = float32(cls.trueframetime)
	}

	// figure button bits
	if in_attack.state&3 != 0 {
		cmd.buttons |= 1
	}
	in_attack.state &^= 2

	if in_jump.state&3 != 0 {
		cmd.buttons |= 2
	}
	in_jump.state &^= 2

	if in_use.state&3 != 0 {
		cmd.buttons |= 4
	}
	in_use.state &^= 2

	// send milliseconds of time to apply the move
	cl_extramsec += float64(frametime * 1000)
	ms := int(cl_extramsec)
	cl_extramsec -= float64(ms)
	if ms > 250 {
		ms = 100 // time was unreasonable
	}
	cmd.msec = byte(ms)

	cmd.angles = cl.viewangles

	if in_impulse != 0 {
		cmd.impulse = byte(in_impulse)
		in_impulse = 0
	} else if in_otherimpulse != 0 {
		cmd.impulse = byte(in_otherimpulse)
		in_otherimpulse = 0
	}

	// chop down so no extra bits are kept that the server wouldn't get
	cmd.forwardmove = MakeShort(int(cmd.forwardmove))
	cmd.sidemove = MakeShort(int(cmd.sidemove))
	cmd.upmove = MakeShort(int(cmd.upmove))

	for i := 0; i < 3; i++ {
		cmd.angles[i] = float32(float64(Q_rint(float64(cmd.angles[i])*65536.0/360.0)&65535) * (360.0 / 65536.0))
	}
}

func CL_CvarInitInput() {
	Cmd_AddCommand("+moveup", IN_UpDown)
	Cmd_AddCommand("-moveup", IN_UpUp)
	Cmd_AddCommand("+movedown", IN_DownDown)
	Cmd_AddCommand("-movedown", IN_DownUp)
	Cmd_AddCommand("+forward", IN_ForwardDown)
	Cmd_AddCommand("-forward", IN_ForwardUp)
	Cmd_AddCommand("+back", IN_BackDown)
	Cmd_AddCommand("-back", IN_BackUp)
	Cmd_AddCommand("+moveleft", IN_MoveleftDown)
	Cmd_AddCommand("-moveleft", IN_MoveleftUp)
	Cmd_AddCommand("+moveright", IN_MoverightDown)
	Cmd_AddCommand("-moveright", IN_MoverightUp)
	Cmd_AddCommand("+attack", IN_AttackDown)
	Cmd_AddCommand("-attack", IN_AttackUp)
	Cmd_AddCommand("+use", IN_UseDown)
	Cmd_AddCommand("-use", IN_UseUp)
	Cmd_AddCommand("+jump", IN_JumpDown)
	Cmd_AddCommand("-jump", IN_JumpUp)
	Cmd_AddCommand("impulse", IN_Impulse)
	Cmd_AddCommand("+weapon", IN_WeaponDown)
	Cmd_AddCommand("-weapon", IN_WeaponUp)
	Cmd_AddCommand("rotate", CL_Rotate_f)

	Cvar_SetCurrentGroup(CVAR_GROUP_INPUT_KEYBOARD)
	Cvar_Register(&cl_weaponswitch)
	Cvar_Register(&cl_weaponfire)
	Cvar_Register(&cl_smartjump)
	Cvar_Register(&cl_upspeed)
	Cvar_Register(&cl_forwardspeed)
	Cvar_Register(&cl_backspeed)
	Cvar_Register(&cl_sidespeed)

	Cvar_SetCurrentGroup(CVAR_GROUP_NETWORK)
	Cvar_Register(&cl_nodelta)
	Cvar_Register(&cl_c2sImpulseBackup)
	Cvar_Register(&cl_c2spps)

	Cvar_ResetCurrentGroup()
}

package engine

import (
	"math"
	"unsafe"
)

// cl_view.c

var cl_rollspeed = cvar_t{name: "cl_rollspeed", string: "200"}
var cl_rollangle = cvar_t{name: "cl_rollangle", string: "0"}

var v_kicktime = cvar_t{name: "v_kicktime", string: "0"}
var v_kickroll = cvar_t{name: "v_kickroll", string: "0.6"}
var v_kickpitch = cvar_t{name: "v_kickpitch", string: "0.6"}

var v_viewheight = cvar_t{name: "v_viewheight", string: "0"}

var gl_hwblend = cvar_t{name: "gl_hwblend", string: "1"}

var v_gamma = cvar_t{name: "gl_gamma", string: "0.85", flags: CVAR_ARCHIVE}
var v_contrast = cvar_t{name: "gl_contrast", string: "1.4", flags: CVAR_ARCHIVE}

var (
	v_iyaw_cycle   = cvar_t{name: "v_iyaw_cycle", string: "2", OnChange: Change_v_idle}
	v_iroll_cycle  = cvar_t{name: "v_iroll_cycle", string: "0.5", OnChange: Change_v_idle}
	v_ipitch_cycle = cvar_t{name: "v_ipitch_cycle", string: "1", OnChange: Change_v_idle}
	v_iyaw_level   = cvar_t{name: "v_iyaw_level", string: "0.3", OnChange: Change_v_idle}
	v_iroll_level  = cvar_t{name: "v_iroll_level", string: "0.1", OnChange: Change_v_idle}
	v_ipitch_level = cvar_t{name: "v_ipitch_level", string: "0.3", OnChange: Change_v_idle}
	v_idlescale    = cvar_t{name: "v_idlescale", string: "0", OnChange: Change_v_idle}
)

var v_contentblend = cvar_t{name: "v_contentblend", string: "0.1"}
var v_damagecshift = cvar_t{name: "v_damagecshift", string: "0"}
var v_quadcshift = cvar_t{name: "v_quadcshift", string: "0.3"}
var v_suitcshift = cvar_t{name: "v_suitcshift", string: "0.3"}
var v_ringcshift = cvar_t{name: "v_ringcshift", string: "0.3"}
var v_pentcshift = cvar_t{name: "v_pentcshift", string: "0.3"}

var v_bonusflash = cvar_t{name: "cl_bonusflash", string: "0"}

var v_dmg_time, v_dmg_roll, v_dmg_pitch float32

var cshift_empty = cshift_t{destcolor: [3]int{130, 80, 50}}

func V_CalcRoll(angles, velocity vec3_t) float32 {
	var right vec3_t

	AngleVectors(angles, nil, &right, nil)
	side := DotProduct(velocity, right)
	var sign float32 = 1
	if side < 0 {
		sign = -1
	}
	side = float32(math.Abs(float64(side)))

	if side < cl_rollspeed.value {
		side = side * cl_rollangle.value / cl_rollspeed.value
	} else {
		side = cl_rollangle.value
	}

	if side > 45 {
		side = 45
	}

	return side * sign
}

func V_ParseDamage() {
	var from, forward, right vec3_t

	armor := MSG_ReadByte()
	blood := MSG_ReadByte()
	for i := 0; i < 3; i++ {
		from[i] = MSG_ReadCoord()
	}

	count := float32(float64(blood)*0.5 + float64(armor)*0.5)
	if count < 10 {
		count = 10
	}

	cl.faceanimtime = float32(cl.time + 0.2) // but sbar face into pain frame

	dmg := &cl.cshifts[CSHIFT_DAMAGE]
	dmg.percent = int(float32(dmg.percent) + 3*count)
	if dmg.percent < 0 {
		dmg.percent = 0
	}
	if dmg.percent > 150 {
		dmg.percent = 150
	}

	fraction := v_damagecshift.value
	if fraction < 0 {
		fraction = 0
	}
	if fraction > 1 {
		fraction = 1
	}
	dmg.percent = int(float32(dmg.percent) * fraction)

	if armor > blood {
		dmg.destcolor[0] = 200
		dmg.destcolor[1] = 100
		dmg.destcolor[2] = 100
	} else if armor != 0 {
		dmg.destcolor[0] = 220
		dmg.destcolor[1] = 50
		dmg.destcolor[2] = 50
	} else {
		dmg.destcolor[0] = 255
		dmg.destcolor[1] = 0
		dmg.destcolor[2] = 0
	}

	// calculate view angle kicks
	VectorSubtract(from, cl.simorg, &from)
	VectorNormalizeFast(&from)

	AngleVectors(cl.simangles, &forward, &right, nil)

	side := DotProduct(from, right)
	v_dmg_roll = count * side * v_kickroll.value

	side = DotProduct(from, forward)
	v_dmg_pitch = count * side * v_kickpitch.value

	v_dmg_time = v_kicktime.value
}

var crosshair = cvar_t{name: "crosshair", string: "2", flags: CVAR_ARCHIVE}
var crosshaircolor = cvar_t{name: "crosshaircolor", string: "79", flags: CVAR_ARCHIVE}
var crosshairsize = cvar_t{name: "crosshairsize", string: "0.4"}
var cl_crossx = cvar_t{name: "cl_crossx", string: "0", flags: CVAR_ARCHIVE}
var cl_crossy = cvar_t{name: "cl_crossy", string: "0", flags: CVAR_ARCHIVE}

// the hook recomputes the crosshair from these cvars
func init() {
	for _, v := range []*cvar_t{&crosshair, &crosshaircolor, &crosshairsize, &cl_crossx, &cl_crossy} {
		v.PostChangeCallback = PostChange_crosshairstuff
	}
}

var gl_cshiftpercent = cvar_t{name: "gl_cshiftpercent", string: "100"}
var v_blend [4]float32 // rgba 0.0 - 1.0
var vid_hwgammacontrol = cvar_t{name: "vid_hwgammacontrol", string: "0", flags: CVAR_ARCHIVE}
var ramps [3][256]uint16

func V_SoftGammaActive() bool {
	// SDL3 has no gamma ramps, shader gamma is the only path
	return gl_fbo && R_PostProcess_IsReady()
}

var cl_bob = cvar_t{name: "cl_bob", string: "0"}
var cl_bobcycle = cvar_t{name: "cl_bobcycle", string: "0"}
var cl_bobup = cvar_t{name: "cl_bobup", string: "0"}
var v_gunkick = cvar_t{name: "v_gunkick", string: "0"}
var cl_drawgun = cvar_t{name: "r_drawviewmodel", string: "1"}
var r_viewmodelsize = cvar_t{name: "r_viewmodelSize", string: "0.65"}
var v_dlightcshift = cvar_t{name: "v_dlightcshift", string: "1"}

var old_gamma float32
var old_contrast float32

func V_cshift_f() {
	// don't allow cheating in TF
	if cls.state >= ca_connected && cl.teamfortress && cbuf_current != cbuf_svc {
		return
	}

	cshift_empty.destcolor[0] = atoi(Cmd_Argv(1))
	cshift_empty.destcolor[1] = atoi(Cmd_Argv(2))
	cshift_empty.destcolor[2] = atoi(Cmd_Argv(3))
	cshift_empty.percent = atoi(Cmd_Argv(4))
}

func V_BonusFlash_f() {
	if v_bonusflash.value == 0 && cbuf_current == cbuf_svc {
		return
	}

	cl.cshifts[CSHIFT_BONUS].destcolor[0] = 215
	cl.cshifts[CSHIFT_BONUS].destcolor[1] = 186
	cl.cshifts[CSHIFT_BONUS].destcolor[2] = 69
	cl.cshifts[CSHIFT_BONUS].percent = 50
}

// gfx/rgbmap: a palette, then a bit count, box counts and the colours of
// each box
var rgbmapcounts []byte
var rgbmapoffsets []uint32
var rgbmapcolours []byte

func setuprgbmap(rgbmap []byte, size uint) {
	if size >= 768+1 {
		uc := rgbmap[768:]

		numbits := uint(uc[0])
		uc = uc[1:]

		numboxes := uint((1 << numbits) * (1 << numbits) * (1 << numbits) * 2)

		if size >= 768+1+numboxes {
			counts := uint(0)
			for i := uint(0); i < numboxes; i++ {
				counts += uint(uc[i])
			}

			if size >= 768+1+numboxes+counts {
				rgbmapcounts = make([]byte, numboxes)
				rgbmapoffsets = make([]uint32, numboxes)
				rgbmapcolours = make([]byte, counts)

				counts = 0

				for i := uint(0); i < numboxes; i++ {
					rgbmapoffsets[i] = uint32(counts)
					counts += uint(uc[i])
				}

				copy(rgbmapcounts, rgbmap[768+1:768+1+numboxes])
				copy(rgbmapcolours, rgbmap[768+1+numboxes:768+1+numboxes+counts])
			}
		}
	}
}

func V_Init() {
	old_gamma = 42.42
	old_contrast = 42.42

	rgbmap := FS_LoadMallocFile("gfx/rgbmap")
	if rgbmap != nil {
		setuprgbmap(rgbmap, uint(com_filesize))
	}
}

func V_CvarInit() {
	Cmd_AddCommand("v_cshift", V_cshift_f)
	Cmd_AddCommand("bf", V_BonusFlash_f)

	Cvar_SetCurrentGroup(CVAR_GROUP_VIEW)
	Cvar_Register(&cl_rollspeed)
	Cvar_Register(&cl_rollangle)

	Cvar_Register(&v_idlescale)
	Cvar_Register(&v_iyaw_cycle)
	Cvar_Register(&v_iroll_cycle)
	Cvar_Register(&v_ipitch_cycle)
	Cvar_Register(&v_iyaw_level)
	Cvar_Register(&v_iroll_level)
	Cvar_Register(&v_ipitch_level)

	Cvar_SetCurrentGroup(CVAR_GROUP_CROSSHAIR)
	Cvar_Register(&crosshaircolor)
	Cvar_Register(&crosshair)
	Cvar_Register(&crosshairsize)
	Cvar_Register(&cl_crossx)
	Cvar_Register(&cl_crossy)

	Cvar_SetCurrentGroup(CVAR_GROUP_VIEWMODEL)
	Cvar_Register(&cl_bob)
	Cvar_Register(&cl_bobcycle)
	Cvar_Register(&cl_bobup)

	Cvar_Register(&cl_drawgun)
	Cvar_Register(&r_viewmodelsize)

	Cvar_SetCurrentGroup(CVAR_GROUP_VIEW)
	Cvar_Register(&v_kicktime)
	Cvar_Register(&v_kickroll)
	Cvar_Register(&v_kickpitch)
	Cvar_Register(&v_gunkick)

	Cvar_Register(&v_viewheight)

	Cvar_SetCurrentGroup(CVAR_GROUP_BLEND)

	Cvar_Register(&v_bonusflash)
	Cvar_Register(&v_contentblend)
	Cvar_Register(&v_damagecshift)
	Cvar_Register(&v_quadcshift)
	Cvar_Register(&v_suitcshift)
	Cvar_Register(&v_ringcshift)
	Cvar_Register(&v_pentcshift)

	Cvar_Register(&v_dlightcshift)
	Cvar_Register(&gl_cshiftpercent)
	Cvar_Register(&gl_hwblend)

	Cvar_SetCurrentGroup(CVAR_GROUP_SCREEN)
	Cvar_Register(&v_gamma)
	Cvar_Register(&v_contrast)
	Cvar_Register(&vid_hwgammacontrol)

	Cvar_ResetCurrentGroup()

	Cmd_AddLegacyCommand("gamma", v_gamma.name)
	Cmd_AddLegacyCommand("contrast", v_contrast.name)
}

var v_bobtime float64
var v_bob float32

func V_CalcBob() float32 {
	if cl.spectator != 0 {
		return 0
	}

	if !cl.onground {
		return v_bob // just use old value
	}

	if cl_bobcycle.value <= 0 {
		return 0
	}

	v_bobtime += cls.frametime
	cycle := float32(v_bobtime - float64(int(v_bobtime/float64(cl_bobcycle.value)))*float64(cl_bobcycle.value))
	cycle /= cl_bobcycle.value
	if cycle < cl_bobup.value {
		cycle = float32(math.Pi * float64(cycle) / float64(cl_bobup.value))
	} else {
		cycle = float32(math.Pi + math.Pi*float64(cycle-cl_bobup.value)/(1.0-float64(cl_bobup.value)))
	}

	// bob is proportional to simulated velocity in the xy plane
	// (don't count Z, or jumping messes it up)
	v_bob = float32(math.Sqrt(float64(cl.simvel[0]*cl.simvel[0]+cl.simvel[1]*cl.simvel[1])) * float64(cl_bob.value))
	v_bob = float32(float64(v_bob)*0.3 + float64(v_bob)*0.7*math.Sin(float64(cycle)))
	v_bob = bound(float32(-7), v_bob, 4)
	return v_bob
}

var cshift_water = cshift_t{destcolor: [3]int{130, 80, 50}, percent: 128}
var cshift_slime = cshift_t{destcolor: [3]int{0, 25, 5}, percent: 150}
var cshift_lava = cshift_t{destcolor: [3]int{255, 80, 0}, percent: 150}

func V_SetContentsColor(contents int) {
	if v_contentblend.value == 0 {
		cl.cshifts[CSHIFT_CONTENTS] = cshift_empty
		cl.cshifts[CSHIFT_CONTENTS].percent *= 100
		return
	}

	switch contents {
	case CONTENTS_EMPTY:
		cl.cshifts[CSHIFT_CONTENTS] = cshift_empty
	case CONTENTS_LAVA:
		cl.cshifts[CSHIFT_CONTENTS] = cshift_lava
	case CONTENTS_SOLID, CONTENTS_SLIME:
		cl.cshifts[CSHIFT_CONTENTS] = cshift_slime
	default:
		cl.cshifts[CSHIFT_CONTENTS] = cshift_water
	}

	if v_contentblend.value > 0 && v_contentblend.value < 1 && contents != CONTENTS_EMPTY {
		cl.cshifts[CSHIFT_CONTENTS].percent = int(float32(cl.cshifts[CSHIFT_CONTENTS].percent) * v_contentblend.value)
	}

	if gl_polyblend.value == 0 && !cl.teamfortress {
		cl.cshifts[CSHIFT_CONTENTS].percent = 0
	} else {
		// ignore gl_cshiftpercent on custom cshifts (set with v_cshift
		// command) to avoid cheating in TF
		if contents != CONTENTS_EMPTY {
			if gl_polyblend.value == 0 {
				cl.cshifts[CSHIFT_CONTENTS].percent = 0
			} else {
				cl.cshifts[CSHIFT_CONTENTS].percent = int(float32(cl.cshifts[CSHIFT_CONTENTS].percent) * gl_cshiftpercent.value)
			}
		} else {
			cl.cshifts[CSHIFT_CONTENTS].percent *= 100
		}
	}
}

func V_CalcPowerupCshift() {
	var fraction float32

	if cl.stats[STAT_ITEMS]&IT_QUAD != 0 {
		cl.cshifts[CSHIFT_POWERUP].destcolor[0] = 0
		cl.cshifts[CSHIFT_POWERUP].destcolor[1] = 0
		cl.cshifts[CSHIFT_POWERUP].destcolor[2] = 255
		fraction = bound(float32(0), v_quadcshift.value, 1)
		cl.cshifts[CSHIFT_POWERUP].percent = int(30 * fraction)
	} else if cl.stats[STAT_ITEMS]&IT_SUIT != 0 {
		cl.cshifts[CSHIFT_POWERUP].destcolor[0] = 0
		cl.cshifts[CSHIFT_POWERUP].destcolor[1] = 255
		cl.cshifts[CSHIFT_POWERUP].destcolor[2] = 0
		fraction = bound(float32(0), v_suitcshift.value, 1)
		cl.cshifts[CSHIFT_POWERUP].percent = int(20 * fraction)
	} else if cl.stats[STAT_ITEMS]&IT_INVISIBILITY != 0 {
		cl.cshifts[CSHIFT_POWERUP].destcolor[0] = 100
		cl.cshifts[CSHIFT_POWERUP].destcolor[1] = 100
		cl.cshifts[CSHIFT_POWERUP].destcolor[2] = 100
		fraction = bound(float32(0), v_ringcshift.value, 1)
		cl.cshifts[CSHIFT_POWERUP].percent = int(100 * fraction)
	} else if cl.stats[STAT_ITEMS]&IT_INVULNERABILITY != 0 {
		cl.cshifts[CSHIFT_POWERUP].destcolor[0] = 255
		cl.cshifts[CSHIFT_POWERUP].destcolor[1] = 255
		cl.cshifts[CSHIFT_POWERUP].destcolor[2] = 0
		fraction = bound(float32(0), v_pentcshift.value, 1)
		cl.cshifts[CSHIFT_POWERUP].percent = int(30 * fraction)
	} else {
		cl.cshifts[CSHIFT_POWERUP].percent = 0
	}
}

func V_CalcBlend() {
	var r, g, b, a, a2 float32

	if cls.state != ca_active {
		cl.cshifts[CSHIFT_CONTENTS] = cshift_empty
		cl.cshifts[CSHIFT_POWERUP].percent = 0
	} else {
		V_CalcPowerupCshift()
	}

	// drop the damage value
	cl.cshifts[CSHIFT_DAMAGE].percent = int(float64(cl.cshifts[CSHIFT_DAMAGE].percent) - cls.frametime*150)
	if cl.cshifts[CSHIFT_DAMAGE].percent <= 0 {
		cl.cshifts[CSHIFT_DAMAGE].percent = 0
	}

	// drop the bonus value
	cl.cshifts[CSHIFT_BONUS].percent = int(float64(cl.cshifts[CSHIFT_BONUS].percent) - cls.frametime*100)
	if cl.cshifts[CSHIFT_BONUS].percent <= 0 {
		cl.cshifts[CSHIFT_BONUS].percent = 0
	}

	for j := 0; j < NUM_CSHIFTS; j++ {
		if (gl_cshiftpercent.value == 0 || gl_polyblend.value == 0) && j != CSHIFT_CONTENTS {
			continue
		}

		if j == CSHIFT_CONTENTS {
			a2 = float32(float64(cl.cshifts[j].percent) / 100.0 / 255.0)
		} else {
			a2 = float32(float64(float32(cl.cshifts[j].percent)*gl_cshiftpercent.value) / 100.0 / 255.0)
		}

		if a2 == 0 {
			continue
		}
		a = a + a2*(1-a)

		a2 = a2 / a
		r = r*(1-a2) + float32(cl.cshifts[j].destcolor[0])*a2
		g = g*(1-a2) + float32(cl.cshifts[j].destcolor[1])*a2
		b = b*(1-a2) + float32(cl.cshifts[j].destcolor[2])*a2
	}

	v_blend[0] = float32(float64(r) / 255.0)
	v_blend[1] = float32(float64(g) / 255.0)
	v_blend[2] = float32(float64(b) / 255.0)
	v_blend[3] = a
	v_blend[3] = bound(float32(0), v_blend[3], 1)
}

func V_AddLightBlend(r, g, b, a2 float32) {
	if gl_polyblend.value == 0 || gl_cshiftpercent.value == 0 || v_dlightcshift.value == 0 {
		return
	}

	a2 = float32(float64(a2*bound(float32(0), v_dlightcshift.value, 1)*gl_cshiftpercent.value) / 100.0)

	a := v_blend[3] + a2*(1-v_blend[3])
	v_blend[3] = a

	if a == 0 {
		return
	}

	a2 = a2 / a

	v_blend[0] = v_blend[0]*(1-a2) + r*a2
	v_blend[1] = v_blend[1]*(1-a2) + g*a2
	v_blend[2] = v_blend[2]*(1-a2) + b*a2
}

var (
	v_prev_blend      [4]float32
	v_old_hwblend     float32
	v_last_ramps      [3][256]uint16
	v_have_last_ramps bool
)

func rampsSlice(r *[3][256]uint16) []uint16 {
	return unsafe.Slice(&r[0][0], 3*256)
}

func V_UpdatePalette(force_update bool) {
	var rgb [3]float32

	if V_SoftGammaActive() {
		return
	}

	new_ := false

	for i := 0; i < 4; i++ {
		if v_blend[i] != v_prev_blend[i] {
			new_ = true
			v_prev_blend[i] = v_blend[i]
		}
	}

	current_gamma := float32(bound(0.3, float64(v_gamma.value), 3))
	if current_gamma != old_gamma {
		old_gamma = current_gamma
		new_ = true
	}

	current_contrast := bound(float32(1), v_contrast.value, 3)
	if current_contrast != old_contrast {
		old_contrast = current_contrast
		new_ = true
	}

	if gl_hwblend.value != v_old_hwblend {
		new_ = true
		v_old_hwblend = gl_hwblend.value
	}

	if !new_ && !force_update {
		if v_have_last_ramps && VID_HWGammaSupported() {
			VID_SetDeviceGammaRamp(rampsSlice(&v_last_ramps))
		}
		return
	}

	a := v_blend[3]

	if !VID_HWGammaSupported() || gl_hwblend.value == 0 || cl.teamfortress {
		a = 0
	}

	rgb[0] = 255 * v_blend[0] * a
	rgb[1] = 255 * v_blend[1] * a
	rgb[2] = 255 * v_blend[2] * a

	a = 1 - a

	if vid_gamma != 1.0 {
		current_contrast = float32(math.Pow(float64(current_contrast), float64(vid_gamma)))
		current_gamma = current_gamma / vid_gamma
	}

	if VID_HWGammaSupported() {
		for i := 0; i < 256; i++ {
			for j := 0; j < 3; j++ {
				// apply blend and contrast
				c := int((float32(i)*a + rgb[j]) * current_contrast)
				if c > 255 {
					c = 255
				}
				// apply gamma
				c = int(255*math.Pow((float64(c)+0.5)/255.5, float64(current_gamma)) + 0.5)
				c = bound(0, c, 255)
				ramps[j][i] = uint16(c << 8)
			}
		}
		v_last_ramps = ramps
		v_have_last_ramps = true
		VID_SetDeviceGammaRamp(rampsSlice(&ramps))
	}
}

func v_palDistance(r, g, b int, p uint) float64 {
	return math.Sqrt(math.Pow(float64(r-int(host_basepal[p*3+0])), 2) + math.Pow(float64(g-int(host_basepal[p*3+1])), 2) + math.Pow(float64(b-int(host_basepal[p*3+2])), 2))
}

func V_LookUpColourInternal_RGBMap(r, g, b int) byte {
	bestdistance := float64(256 * 256 * 256)
	bestfit := uint(0)

	box := uint(((r >> 5) << 6) + ((g >> 5) << 3) + ((b >> 5) << 0))

	count := uint(rgbmapcounts[box])
	offset := uint(rgbmapoffsets[box])

	for i := uint(0); i < count; i++ {
		p := uint(rgbmapcolours[offset+i])
		distance := v_palDistance(r, g, b, p)
		if distance < bestdistance {
			bestfit = p
			bestdistance = distance
		}
	}

	return byte(bestfit)
}

func V_LookUpColourInternal_RGBMap_NoFullBright(r, g, b int) byte {
	bestdistance := float64(256 * 256 * 256)
	bestfit := uint(0)

	box := uint(((r >> 5) << 6) + ((g >> 5) << 3) + ((b >> 5) << 0))

	box += 8 * 8 * 8

	count := uint(rgbmapcounts[box])
	offset := uint(rgbmapoffsets[box])

	for i := uint(0); i < count; i++ {
		p := uint(rgbmapcolours[offset+i])
		distance := v_palDistance(r, g, b, p)
		if distance < bestdistance {
			bestfit = p
			bestdistance = distance
		}
	}

	return byte(bestfit)
}

func V_LookUpColourInternal_Brute(r, g, b int, maxindex uint) byte {
	bestdistance := float64(256 * 256 * 256)
	bestfit := uint(0)

	for i := uint(0); i < maxindex; i++ {
		distance := v_palDistance(r, g, b, i)
		if distance < bestdistance {
			bestfit = i
			bestdistance = distance
		}
	}

	return byte(bestfit)
}

func V_LookUpColourInt(r, g, b uint) byte {
	if rgbmapcounts != nil {
		return V_LookUpColourInternal_RGBMap(int(r), int(g), int(b))
	}
	return V_LookUpColourInternal_Brute(int(r), int(g), int(b), 256)
}

func V_LookUpColourIntNoFullbright(r, g, b uint) byte {
	if rgbmapcounts != nil {
		return V_LookUpColourInternal_RGBMap_NoFullBright(int(r), int(g), int(b))
	}
	return V_LookUpColourInternal_Brute(int(r), int(g), int(b), 224)
}

func V_LookUpColour(r, g, b float32) byte {
	r = bound(float32(0), r, 1)
	g = bound(float32(0), g, 1)
	b = bound(float32(0), b, 1)

	if rgbmapcounts != nil {
		return V_LookUpColourInternal_RGBMap(int(r*255), int(g*255), int(b*255))
	}
	return V_LookUpColourInternal_Brute(int(r*255), int(g*255), int(b*255), 256)
}

func V_LookUpColourNoFullbright(r, g, b float32) byte {
	r = bound(float32(0), r, 1)
	g = bound(float32(0), g, 1)
	b = bound(float32(0), b, 1)

	if rgbmapcounts != nil {
		return V_LookUpColourInternal_RGBMap_NoFullBright(int(r*255), int(g*255), int(b*255))
	}
	return V_LookUpColourInternal_Brute(int(r*255), int(g*255), int(b*255), 224)
}

func V_BoundOffsets() {
	// absolutely bound refresh relative to entity clipping hull
	// so the view can never be inside a solid wall

	if r_refdef.vieworg[0] < cl.simorg[0]-14 {
		r_refdef.vieworg[0] = cl.simorg[0] - 14
	} else if r_refdef.vieworg[0] > cl.simorg[0]+14 {
		r_refdef.vieworg[0] = cl.simorg[0] + 14
	}

	if r_refdef.vieworg[1] < cl.simorg[1]-14 {
		r_refdef.vieworg[1] = cl.simorg[1] - 14
	} else if r_refdef.vieworg[1] > cl.simorg[1]+14 {
		r_refdef.vieworg[1] = cl.simorg[1] + 14
	}

	if r_refdef.vieworg[2] < cl.simorg[2]-22 {
		r_refdef.vieworg[2] = cl.simorg[2] - 22
	} else if r_refdef.vieworg[2] > cl.simorg[2]+30 {
		r_refdef.vieworg[2] = cl.simorg[2] + 30
	}
}

// Idle swaying
func V_AddIdle() {
	if v_idlescale.value == 0 {
		return
	}

	r_refdef.viewangles[ROLL] = float32(float64(r_refdef.viewangles[ROLL]) + float64(v_idlescale.value)*math.Sin(cl.time*float64(v_iroll_cycle.value))*float64(v_iroll_level.value))
	r_refdef.viewangles[PITCH] = float32(float64(r_refdef.viewangles[PITCH]) + float64(v_idlescale.value)*math.Sin(cl.time*float64(v_ipitch_cycle.value))*float64(v_ipitch_level.value))
	r_refdef.viewangles[YAW] = float32(float64(r_refdef.viewangles[YAW]) + float64(v_idlescale.value)*math.Sin(cl.time*float64(v_iyaw_cycle.value))*float64(v_iyaw_level.value))
}

// Roll is induced by movement and damage
func V_CalcViewRoll() {
	side := V_CalcRoll(cl.simangles, cl.simvel)
	adjspeed := float32(20 * bound(2, math.Abs(float64(cl_rollangle.value)), 45))
	if side > cl.rollangle {
		cl.rollangle = float32(float64(cl.rollangle) + cls.frametime*float64(adjspeed))
		if cl.rollangle > side {
			cl.rollangle = side
		}
	} else if side < cl.rollangle {
		cl.rollangle = float32(float64(cl.rollangle) - cls.frametime*float64(adjspeed))
		if cl.rollangle < side {
			cl.rollangle = side
		}
	}
	r_refdef.viewangles[ROLL] += cl.rollangle

	if v_dmg_time > 0 {
		r_refdef.viewangles[ROLL] += v_dmg_time / v_kicktime.value * v_dmg_roll
		r_refdef.viewangles[PITCH] += v_dmg_time / v_kicktime.value * v_dmg_pitch
		v_dmg_time = float32(float64(v_dmg_time) - cls.frametime)
	}
}

func V_AddViewWeapon(bob float32) {
	var forward vec3_t
	var weapon int

	cent := &cl.viewent
	TP_ParseWeaponModel(cl.model_precache[cl.stats[STAT_WEAPON]])

	if cl_drawgun.value == 0 || (cl_drawgun.value == 2 && scr_fov.value > 90) ||
		(view_message.flags&(PF_GIB|PF_DEAD)) != 0 ||
		cl.stats[STAT_ITEMS]&IT_INVISIBILITY != 0 || cl.stats[STAT_HEALTH] <= 0 || !Cam_DrawViewModel() {
		cent.current.modelindex = 0 // no model
		return
	}

	// angles
	cent.current.angles[YAW] = r_refdef.viewangles[YAW]
	cent.current.angles[PITCH] = -r_refdef.viewangles[PITCH]
	cent.current.angles[ROLL] = r_refdef.viewangles[ROLL]
	// origin
	AngleVectors(r_refdef.viewangles, &forward, nil, nil)
	cent.current.origin = r_refdef.vieworg
	VectorMA(cent.current.origin, float32(float64(bob)*0.4), forward, &cent.current.origin)
	// fudge position around to keep amount of weapon visible roughly equal with different FOV
	if scr_viewsize.value == 110 {
		cent.current.origin[2] += 1
	} else if scr_viewsize.value == 100 {
		cent.current.origin[2] += 2
	} else if scr_viewsize.value == 90 {
		cent.current.origin[2] += 1
	} else if scr_viewsize.value == 80 {
		cent.current.origin[2] += 0.5
	}

	if cl_preselectedweapon != 0 && cl_preselectedweapon >= 1 && cl_preselectedweapon <= 8 && cl.weapon_to_model_index[cl_preselectedweapon-1] != 0 {
		weapon = cl.weapon_to_model_index[cl_preselectedweapon-1]
	} else {
		weapon = cl.stats[STAT_WEAPON]
	}

	if cent.current.modelindex != weapon {
		cl.viewent.frametime = -1
	} else {
		if cent.current.frame != view_message.weaponframe {
			cent.frametime = cl.time
			cent.oldframe = cent.current.frame
		}
	}

	cent.current.modelindex = weapon
	cent.current.frame = view_message.weaponframe
}

func V_CalcIntermissionRefdef() {
	r_refdef.vieworg = cl.simorg
	r_refdef.viewangles = cl.simangles

	// we don't draw weapon in intermission
	cl.viewent.current.modelindex = 0

	// always idle in intermission
	old := v_idlescale.value
	v_idlescale.value = 1
	V_AddIdle()
	v_idlescale.value = old
}

func V_CalcRefdef() {
	var forward vec3_t
	var height_adjustment float32

	if v_viewheight.value != 0 {
		height_adjustment = bound(float32(-7), v_viewheight.value, 4)
	} else {
		height_adjustment = V_CalcBob()
	}

	// set up the refresh position
	r_refdef.vieworg = cl.simorg

	// never let it sit exactly on a node line, because a water plane can
	// dissapear when viewed with the eye exactly on it.
	// the server protocol only specifies to 1/8 pixel, so add 1/16 in each axis
	r_refdef.vieworg[0] = float32(float64(r_refdef.vieworg[0]) + 1.0/16)
	r_refdef.vieworg[1] = float32(float64(r_refdef.vieworg[1]) + 1.0/16)
	r_refdef.vieworg[2] = float32(float64(r_refdef.vieworg[2]) + 1.0/16)

	// add view height
	if view_message.flags&PF_GIB != 0 {
		r_refdef.vieworg[2] += 8 // gib view height
	} else if view_message.flags&PF_DEAD != 0 && cl.stats[STAT_HEALTH] <= 0 {
		r_refdef.vieworg[2] -= 16 // corpse view height
	} else {
		r_refdef.vieworg[2] += 22 // normal view height

		r_refdef.vieworg[2] += height_adjustment

		// smooth out stair step ups
		r_refdef.vieworg[2] += cl.crouch
	}

	// set up refresh view angles
	r_refdef.viewangles = cl.simangles
	V_CalcViewRoll()
	V_AddIdle()

	if v_gunkick.value != 0 {
		// add weapon kick offset
		AngleVectors(r_refdef.viewangles, &forward, nil, nil)
		VectorMA(r_refdef.vieworg, cl.punchangle, forward, &r_refdef.vieworg)

		// add weapon kick angle
		r_refdef.viewangles[PITCH] = float32(float64(r_refdef.viewangles[PITCH]) + float64(cl.punchangle)*0.5)
	}

	if view_message.flags&PF_DEAD != 0 && cl.stats[STAT_HEALTH] <= 0 {
		r_refdef.viewangles[ROLL] = 80 // dead view angle
	}

	V_AddViewWeapon(height_adjustment)
}

func DropPunchAngle() {
	if cl.ideal_punchangle < cl.punchangle {
		if cl.ideal_punchangle == -2 { // small kick
			cl.punchangle = float32(float64(cl.punchangle) - 20*cls.frametime)
		} else { // big kick
			cl.punchangle = float32(float64(cl.punchangle) - 40*cls.frametime)
		}

		if cl.punchangle < cl.ideal_punchangle {
			cl.punchangle = cl.ideal_punchangle
			cl.ideal_punchangle = 0
		}
	} else {
		cl.punchangle = float32(float64(cl.punchangle) + 20*cls.frametime)
		if cl.punchangle > 0 {
			cl.punchangle = 0
		}
	}
}

// The player's clipping box goes from (-16 -16 -24) to (16 16 32) from
// the entity origin, so any view position inside that will be valid
var view_frame *frame_t
var view_message *player_state_t

func V_RenderView() {
	cl.simangles[ROLL] = 0 // FIXME @@@

	if cls.state != ca_active {
		V_CalcBlend()
		return
	}

	view_frame = &cl.frames[cl.validsequence&UPDATE_MASK]
	view_message = &view_frame.playerstate[cl.viewplayernum]

	DropPunchAngle()
	if cl.intermission != 0 { // intermission / finale rendering
		V_CalcIntermissionRefdef()
	} else {
		V_CalcRefdef()
	}

	R_PushDlights()
	R_RenderView()
}

func V_Shutdown() {
	rgbmapcounts = nil
	rgbmapoffsets = nil
	rgbmapcolours = nil
}

func Change_v_idle(v *cvar_t, value string) bool {
	// Don't allow cheating in TF
	return cl.teamfortress && cls.state >= ca_connected && cbuf_current != cbuf_svc
}

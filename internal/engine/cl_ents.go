package engine

import "math"

var nullentitystate entity_state_t

func ISDEAD(i int) bool {
	return i >= 41 && i <= 102
}

type predicted_player struct {
	flags    int
	active   bool
	origin   vec3_t // predicted origin
	predict  bool
	oldo     vec3_t
	olda     vec3_t
	oldv     vec3_t
	oldstate *player_state_t
}

var predicted_players [MAX_CLIENTS]predicted_player

var cl_modelnames [cl_num_modelindices]string
var cl_modelindices [cl_num_modelindices]int // model numbers, typed as the enum in C

func CL_InitEnts() bool {
	cl_modelnames = [cl_num_modelindices]string{}
	cl_modelnames[mi_spike] = "progs/spike.mdl"
	cl_modelnames[mi_player] = "progs/player.mdl"
	cl_modelnames[mi_flag] = "progs/flag.mdl"
	cl_modelnames[mi_tf_flag] = "progs/tf_flag.mdl"
	cl_modelnames[mi_tf_stan] = "progs/tf_stan.mdl"
	cl_modelnames[mi_explod1] = "progs/s_explod.spr"
	cl_modelnames[mi_explod2] = "progs/s_expl.spr"
	cl_modelnames[mi_h_player] = "progs/h_player.mdl"
	cl_modelnames[mi_gib1] = "progs/gib1.mdl"
	cl_modelnames[mi_gib2] = "progs/gib2.mdl"
	cl_modelnames[mi_gib3] = "progs/gib3.mdl"
	cl_modelnames[mi_rocket] = "progs/missile.mdl"
	cl_modelnames[mi_grenade] = "progs/grenade.mdl"
	cl_modelnames[mi_bubble] = "progs/s_bubble.spr"
	cl_modelnames[mi_vaxe] = "progs/v_axe.mdl"
	cl_modelnames[mi_vbio] = "progs/v_bio.mdl"
	cl_modelnames[mi_vgrap] = "progs/v_grap.mdl"
	cl_modelnames[mi_vknife] = "progs/v_knife.mdl"
	cl_modelnames[mi_vknife2] = "progs/v_knife2.mdl"
	cl_modelnames[mi_vmedi] = "progs/v_medi.mdl"
	cl_modelnames[mi_vspan] = "progs/v_span.mdl"

	for i := 0; i < int(cl_num_modelindices); i++ {
		if cl_modelnames[i] == "" {
			Sys_Error("cl_modelnames[%d] not initialized", i)
		}
	}

	cl_firstpassents.max = 64
	cl_firstpassents.alpha = 0

	cl_visents.max = 256
	cl_visents.alpha = 0

	cl_alphaents.max = 64
	cl_alphaents.alpha = 1

	cl_firstpassents.list = make([]entity_t, cl_firstpassents.max)
	cl_visents.list = make([]entity_t, cl_visents.max)
	cl_alphaents.list = make([]entity_t, cl_alphaents.max)

	CL_ClearScene()

	return true
}

func CL_ShutdownEnts() {
	cl_firstpassents.list = nil
	cl_visents.list = nil
	cl_alphaents.list = nil
}

func CL_ClearScene() {
	cl_firstpassents.count = 0
	cl_visents.count = 0
	cl_alphaents.count = 0
}

func CL_AddEntity(ent *entity_t) {
	var vislist *visentlist_t

	if ent.model.type_ == mod_sprite {
		vislist = &cl_alphaents
	} else if ent.model.modhint == MOD_PLAYER || ent.model.modhint == MOD_EYES {
		vislist = &cl_firstpassents
		ent.flags |= RF_NOSHADOW
	} else {
		vislist = &cl_visents
	}

	if vislist.count < vislist.max {
		vislist.list[vislist.count] = *ent
		vislist.count++
	}
}

func dlightColor(f float32, def dlighttype_t, random bool) dlighttype_t {
	colors := [int(NUM_DLIGHTTYPES) - 4]dlighttype_t{lt_red, lt_blue, lt_redblue, lt_green, lt_white}

	switch {
	case int(f) == 1:
		return lt_red
	case int(f) == 2:
		return lt_blue
	case int(f) == 3:
		return lt_redblue
	case int(f) == 4:
		return lt_green
	case int(f) == 5:
		return lt_white
	case int(f) == int(NUM_DLIGHTTYPES)-3 && random:
		return colors[rand()%(int(NUM_DLIGHTTYPES)-4)]
	}
	return def
}

func CL_AllocDlight(key int) *dlight_t {
	// first look for an exact key match
	if key != 0 {
		for i := 0; i < MAX_DLIGHTS; i++ {
			dl := &cl_dlights[i]
			if dl.key == key {
				*dl = dlight_t{}
				dl.key = key
				cl_dlight_active[i/32] |= 1 << uint(i%32)
				return dl
			}
		}
	}

	// then look for anything else
	for i := 0; i < MAX_DLIGHTS/32; i++ {
		if cl_dlight_active[i] != 0xffffffff {
			for j := 0; j < 32; j++ {
				if cl_dlight_active[i]&(1<<uint(j)) == 0 && i*32+j < MAX_DLIGHTS {
					dl := &cl_dlights[i*32+j]
					*dl = dlight_t{}
					dl.key = key
					cl_dlight_active[i] |= 1 << uint(j)
					return dl
				}
			}
		}
	}

	dl := &cl_dlights[0]
	*dl = dlight_t{}
	dl.key = key
	cl_dlight_active[0] |= 1
	return dl
}

func CL_NewDlight(key int, origin vec3_t, radius float32, time float32, type_ int, bubble int) {
	dl := CL_AllocDlight(key)
	dl.origin = origin
	dl.radius = radius
	dl.die = float32(cl.time + float64(time))
	dl.type_ = type_
	dl.bubble = bubble
}

func CL_DecayLights() {
	if cls.state < ca_active {
		return
	}

	for i := 0; i < MAX_DLIGHTS/32; i++ {
		if cl_dlight_active[i] != 0 {
			for j := 0; j < 32; j++ {
				if cl_dlight_active[i]&(1<<uint(j)) != 0 && i*32+j < MAX_DLIGHTS {
					dl := &cl_dlights[i*32+j]
					dl.radius = float32(float64(dl.radius) - cls.frametime*float64(dl.decay))
					if dl.radius < 0 {
						dl.radius = 0
					}
					if float64(dl.die) < cl.time || dl.radius == 0 {
						cl_dlight_active[i] &^= 1 << uint(j)
						continue
					}
				}
			}
		}
	}
}

func CL_SetupPacketEntity(number int, state *entity_state_t, changed bool) {
	cent := &cl_entities[number]

	if cl.oldvalidsequence == 0 || cl.oldvalidsequence != cent.sequence ||
		state.modelindex != cent.current.modelindex ||
		!VectorL2Compare(state.origin, cent.current.origin, 200) {
		cent.startlerp = cl.time
		cent.deltalerp = -1
		cent.frametime = -1
		cent.oldsequence = 0
		cent.flags = 0
	} else {
		cent.oldsequence = cent.sequence

		if state.frame != cent.current.frame {
			cent.frametime = cl.time
			cent.oldframe = cent.current.frame
		}

		if changed {
			if !VectorCompare(state.origin, cent.current.origin) || !VectorCompare(state.angles, cent.current.angles) {
				cent.old_origin = cent.current.origin
				cent.old_angles = cent.current.angles
				if cls.mvdplayback {
					cent.deltalerp = float64(nextdemotime - olddemotime)
					cent.startlerp = cls.demotime
				} else {
					cent.deltalerp = cl.time - cent.startlerp
					cent.startlerp = cl.time
				}
			}
		}
	}

	cent.current = *state
	cent.sequence = cl.validsequence
}

// Can go from either a baseline or a previous packet_entity
func CL_ParseDelta(from, to *entity_state_t, bits int) {
	var morebits uint16

	// set everything to the state we are delta'ing from
	*to = *from

	to.number = bits & 511
	bits &^= 511

	if bits&U_MOREBITS != 0 { // read in the low order bits
		i := MSG_ReadByte()
		bits |= i
	}

	if bits&FTE_U_EVENMORE != 0 {
		morebits = uint16(MSG_ReadByte())
		if morebits&FTE_U2_YETMORE != 0 {
			morebits |= uint16(MSG_ReadByte() << 8)
		}
		if morebits&(FTE_U2_UNUSED1|FTE_U2_DRAWFLAGS|FTE_U2_ABSLIGHT|FTE_U2_DPFLAGS|FTE_U2_TAGINFO|FTE_U2_LIGHT|FTE_U2_EFFECT16|FTE_U2_FARMORE) != 0 {
			Host_Error("Unsupported FTE delta morebits 0x%02x\n", morebits)
		}
	} else {
		morebits = 0
	}

	if morebits&FTE_U2_ENTITYDBL != 0 {
		if cls.ftexsupported&FTEX_ENTITYDBL == 0 {
			Host_Error("Got unexpected FTE_U2_ENTITYDBL")
		}
		to.number += 512
	}

	if morebits&FTE_U2_ENTITYDBL2 != 0 {
		if cls.ftexsupported&FTEX_ENTITYDBL2 == 0 {
			Host_Error("Got unexpected FTE_U2_ENTITYDBL2")
		}
		to.number += 1024
	}

	to.flags = bits

	if bits&U_MODEL != 0 {
		to.modelindex = MSG_ReadByte()
	}

	if morebits&FTE_U2_MODELDBL != 0 {
		if cls.ftexsupported&FTEX_MODELDOUBLE == 0 {
			Host_Error("Got unexpected FTE_U2_MODELDBL bit")
		}
		if bits&U_MODEL != 0 {
			to.modelindex += 256
		} else {
			to.modelindex = MSG_ReadShort()
		}
	}

	if bits&U_FRAME != 0 {
		to.frame = MSG_ReadByte()
	}

	if bits&U_COLORMAP != 0 {
		to.colormap = MSG_ReadByte()
	}

	if bits&U_SKIN != 0 {
		to.skinnum = MSG_ReadByte()
	}

	if bits&U_EFFECTS != 0 {
		to.effects = MSG_ReadByte()
	}

	if bits&U_ORIGIN1 != 0 {
		to.origin[0] = MSG_ReadCoord()
	}

	if bits&U_ANGLE1 != 0 {
		to.angles[0] = MSG_ReadAngle()
	}

	if bits&U_ORIGIN2 != 0 {
		to.origin[1] = MSG_ReadCoord()
	}

	if bits&U_ANGLE2 != 0 {
		to.angles[1] = MSG_ReadAngle()
	}

	if bits&U_ORIGIN3 != 0 {
		to.origin[2] = MSG_ReadCoord()
	}

	if bits&U_ANGLE3 != 0 {
		to.angles[2] = MSG_ReadAngle()
	}

	if bits&U_SOLID != 0 {
		// FIXME
	}

	if morebits&FTE_U2_SCALE != 0 {
		if cls.ftexsupported&FTEX_SCALE == 0 {
			Host_Error("Got unexpected FTE_U2_SCALE")
		}
		MSG_ReadByte()
	}

	if morebits&FTE_U2_TRANS != 0 {
		if cls.ftexsupported&FTEX_TRANS == 0 {
			Host_Error("Got unexpected FTE_U2_TRANS")
		}
		MSG_ReadByte()
	}

	if morebits&FTE_U2_FATNESS != 0 {
		if cls.ftexsupported&FTEX_FATNESS == 0 {
			Host_Error("Got unexpected FTE_U2_FATNESS")
		}
		MSG_ReadByte()
	}

	if morebits&FTE_U2_COLOURMOD != 0 {
		if cls.ftexsupported&FTEX_COLOURMOD == 0 {
			Host_Error("Got unexpected FTE_U2_COLOURMOD")
		}
		MSG_ReadByte()
		MSG_ReadByte()
		MSG_ReadByte()
	}

	if to.modelindex >= MAX_MODELS {
		Host_Error("Model index %d out of range", to.modelindex)
	}
}

func FlushEntityPacket() {
	var olde, newe entity_state_t

	Com_DPrintf("FlushEntityPacket\n")

	cl.frames[cls.netchan.incoming_sequence&UPDATE_MASK].invalid = true

	// read it all, but ignore it
	for {
		word := int(uint16(MSG_ReadShort()))
		if msg_badread { // something didn't parse right...
			Host_Error("msg_badread in packetentities")
			return
		}

		if word == 0 {
			break // done
		}

		CL_ParseDelta(&olde, &newe, word)
	}
}

// An svc_packetentities has just been parsed, deal with the rest of the data stream.
func CL_ParsePacketEntities(delta bool) {
	var oldp *packet_entities_t
	var dummy packet_entities_t
	var full bool

	newpacket := cls.netchan.incoming_sequence & UPDATE_MASK
	newp := &cl.frames[newpacket].packet_entities
	cl.frames[newpacket].invalid = false

	if delta {
		from := byte(MSG_ReadByte())

		oldpacket := cl.frames[newpacket].delta_sequence
		if cls.mvdplayback {
			oldpacket = cls.netchan.incoming_sequence - 1
			from = byte(oldpacket)
		}

		if cls.netchan.outgoing_sequence-cls.netchan.incoming_sequence >= UPDATE_BACKUP-1 {
			// there are no valid frames left, so drop it
			FlushEntityPacket()
			cl.validsequence = 0
			return
		}

		if int(from&UPDATE_MASK) != oldpacket&UPDATE_MASK {
			Com_DPrintf("WARNING: from mismatch\n")
			FlushEntityPacket()
			cl.validsequence = 0
			return
		}

		if cls.netchan.outgoing_sequence-oldpacket >= UPDATE_BACKUP-1 {
			// we can't use this, it is too old
			FlushEntityPacket()
			// don't clear cl.validsequence, so that frames can still be rendered;
			// it is possible that a fresh packet will be received before
			// (outgoing_sequence - incoming_sequence) exceeds UPDATE_BACKUP - 1
			return
		}

		oldp = &cl.frames[oldpacket&UPDATE_MASK].packet_entities
		full = false
	} else {
		// this is a full update that we can start delta compressing from now
		oldp = &dummy
		dummy.num_entities = 0
		full = true
	}

	cl.oldvalidsequence = cl.validsequence
	cl.validsequence = cls.netchan.incoming_sequence
	deltasequence := uint32(cl.validsequence)

	oldindex := 0
	newindex := 0
	newp.num_entities = 0

	tooMany := func() bool {
		return newindex >= MAX_MVD_PACKET_ENTITIES || (newindex >= MAX_PACKET_ENTITIES && !cls.mvdplayback)
	}

	for {
		word := int(uint16(MSG_ReadShort()))
		if msg_badread {
			// something didn't parse right...
			Host_Error("msg_badread in packetentities")
			return
		}

		if word == 0 {
			for oldindex < oldp.num_entities {
				// copy all the rest of the entities from the old packet
				if tooMany() {
					Host_Error("CL_ParsePacketEntities: newindex == MAX_PACKET_ENTITIES")
				}
				newp.entities[newindex] = oldp.entities[oldindex]
				CL_SetupPacketEntity(newp.entities[newindex].number, &newp.entities[newindex], false)
				newindex++
				oldindex++
			}
			break
		}

		newnum := word & 511
		oldnum := 9999
		if oldindex < oldp.num_entities {
			oldnum = oldp.entities[oldindex].number
		}

		for newnum > oldnum {
			if full {
				Com_Printf("WARNING: oldcopy on full update")
				FlushEntityPacket()
				cl.validsequence = 0 // can't render a frame
				return
			}

			// copy one of the old entities over to the new packet unchanged
			if tooMany() {
				Host_Error("CL_ParsePacketEntities: newindex == MAX_PACKET_ENTITIES")
			}
			newp.entities[newindex] = oldp.entities[oldindex]
			CL_SetupPacketEntity(oldnum, &newp.entities[newindex], word > 511)
			newindex++
			oldindex++
			oldnum = 9999
			if oldindex < oldp.num_entities {
				oldnum = oldp.entities[oldindex].number
			}
		}

		if newnum < oldnum {
			// new from baseline
			if word&U_REMOVE != 0 {
				if full {
					Com_Printf("WARNING: U_REMOVE on full update\n")
					FlushEntityPacket()
					cl.validsequence = 0 // can't render a frame
					return
				}
				continue
			}

			if tooMany() {
				Host_Error("CL_ParsePacketEntities: newindex == MAX_PACKET_ENTITIES")
			}
			CL_ParseDelta(&cl_entities[newnum].baseline, &newp.entities[newindex], word)
			CL_SetupPacketEntity(newnum, &newp.entities[newindex], word > 511)
			newindex++
			continue
		}

		if newnum == oldnum {
			// delta from previous
			if full {
				cl.validsequence = 0
				deltasequence = 0
				Com_Printf("WARNING: delta on full update")
			}
			if word&U_REMOVE != 0 {
				oldindex++
				continue
			}
			CL_ParseDelta(&oldp.entities[oldindex], &newp.entities[newindex], word)
			CL_SetupPacketEntity(newnum, &newp.entities[newindex], word > 511)
			newindex++
			oldindex++
		}
	}

	newp.num_entities = newindex

	if cl_nodelta.value != 0 {
		deltasequence = 0
	}
	if cls.netqw != nil {
		if deltasequence != 0 {
			NetQW_SetDeltaPoint(cls.netqw, int(deltasequence))
		} else {
			NetQW_SetDeltaPoint(cls.netqw, -1)
		}
	}

	if cls.state == ca_onserver { // we can now render a frame
		CL_MakeActive()
	}
}

func CL_LinkPacketEntities() {
	var ent entity_t

	pack := &cl.frames[cl.validsequence&UPDATE_MASK].packet_entities

	autorotate := float32(anglemod(100 * cl.time))

	for pnum := 0; pnum < pack.num_entities; pnum++ {
		state := &pack.entities[pnum]
		cent := &cl_entities[state.number]

		// control powerup glow for bots
		if state.modelindex != cl_modelindices[mi_player] || r_powerupglow.value != 0 {
			flicker := 0
			if r_lightflicker.value != 0 {
				flicker = rand() & 31
			}
			// spawn light flashes, even ones coming from invisible objects
			if state.effects&(EF_BLUE|EF_RED) == (EF_BLUE | EF_RED) {
				CL_NewDlight(state.number, state.origin, float32(200+flicker), 0.1, int(lt_redblue), 0)
			} else if state.effects&EF_BLUE != 0 {
				CL_NewDlight(state.number, state.origin, float32(200+flicker), 0.1, int(lt_blue), 0)
			} else if state.effects&EF_RED != 0 {
				CL_NewDlight(state.number, state.origin, float32(200+flicker), 0.1, int(lt_red), 0)
			} else if state.effects&EF_BRIGHTLIGHT != 0 {
				tmp := state.origin
				tmp[2] += 16
				CL_NewDlight(state.number, tmp, float32(400+flicker), 0.1, int(lt_default), 0)
			} else if state.effects&EF_DIMLIGHT != 0 {
				var dimlightcolor dlighttype_t
				if cl.teamfortress && (state.modelindex == cl_modelindices[mi_tf_flag] || state.modelindex == cl_modelindices[mi_tf_stan]) {
					dimlightcolor = dlightColor(r_flagcolor.value, lt_default, false)
				} else if state.modelindex == cl_modelindices[mi_flag] {
					dimlightcolor = dlightColor(r_flagcolor.value, lt_default, false)
				} else {
					dimlightcolor = lt_default
				}
				CL_NewDlight(state.number, state.origin, float32(200+flicker), 0.1, int(dimlightcolor), 0)
			}
		}

		if state.modelindex == 0 { // if set to invisible, skip
			continue
		}

		if state.modelindex == cl_modelindices[mi_player] {
			i := state.frame
			if cl_deadbodyfilter.value == 2 {
				if ISDEAD(i) {
					continue
				}
			} else if cl_deadbodyfilter.value != 0 {
				if i == 49 || i == 60 || i == 69 || i == 84 || i == 93 || i == 102 {
					continue
				}
			}
		}

		if cl_gibfilter.value != 0 && (state.modelindex == cl_modelindices[mi_h_player] ||
			state.modelindex == cl_modelindices[mi_gib1] || state.modelindex == cl_modelindices[mi_gib2] || state.modelindex == cl_modelindices[mi_gib3]) {
			continue
		}

		var model *model_t
		if state.modelindex < 0 || state.modelindex >= MAX_MODELS {
			Host_Error("CL_LinkPacketEntities: bad modelindex")
		}
		if model = cl.model_precache[state.modelindex]; model == nil {
			Host_Error("CL_LinkPacketEntities: bad modelindex")
		}

		ent.model = model
		if state.modelindex == cl_modelindices[mi_rocket] {
			if cl_rocket2grenade.value != 0 && cl_modelindices[mi_grenade] != -1 {
				ent.model = cl.model_precache[cl_modelindices[mi_grenade]]
			}
		}
		ent.skinnum = state.skinnum

		ent.frame = state.frame
		if cent.frametime >= 0 && cent.frametime <= cl.time {
			ent.oldframe = cent.oldframe
			ent.framelerp = float32((cl.time - cent.frametime) * 10)
		} else {
			ent.oldframe = ent.frame
			ent.framelerp = -1
		}

		// The skin system works differently these days, so entity colour
		// maps aren't supported anymore.
		if state.colormap >= 1 && state.colormap <= MAX_CLIENTS && ent.model.modhint == MOD_PLAYER {
			ent.scoreboard = &cl.players[state.colormap-1]
		} else {
			ent.scoreboard = nil
		}

		var lerp float32
		if (cl_nolerp.value != 0 && !cls.mvdplayback) || cent.deltalerp <= 0 {
			lerp = -1
			ent.origin = cent.current.origin
		} else {
			time := cl.time
			if cls.mvdplayback {
				time = cls.demotime
			}
			lerp = float32((time - cent.startlerp) / cent.deltalerp)
			lerp = min(lerp, 1)
			VectorInterpolate(cent.old_origin, lerp, cent.current.origin, &ent.origin)
		}

		if ent.model.flags&EF_ROTATE != 0 {
			ent.angles[0] = 0
			ent.angles[2] = 0
			ent.angles[1] = autorotate
			if cl_model_bobbing.value != 0 {
				ent.origin[2] = float32(float64(ent.origin[2]) + math.Sin(float64(autorotate/90)*M_PI)*5 + 5)
			}
		} else {
			if lerp != -1 {
				AngleInterpolate(cent.old_angles, lerp, cent.current.angles, &ent.angles)
			} else {
				ent.angles = cent.current.angles
			}
		}

		if qmb_initialized {
			if state.modelindex == cl_modelindices[mi_explod1] || state.modelindex == cl_modelindices[mi_explod2] {
				if gl_part_inferno.value != 0 {
					QMB_InfernoFlame(ent.origin)
					continue
				}
			}

			if state.modelindex == cl_modelindices[mi_bubble] {
				QMB_StaticBubble(&ent)
				continue
			}
		}

		// add trails
		if model.flags&^EF_ROTATE != 0 {
			var old_origin vec3_t
			if cent.flags&CENT_TRAILDRAWN == 0 || !VectorL2Compare(cent.trail_origin, ent.origin, 140) {
				old_origin = ent.origin // not present last frame or too far away
				cent.flags |= CENT_TRAILDRAWN
			} else {
				old_origin = cent.trail_origin
			}

			if model.flags&EF_ROCKET != 0 {
				if r_rockettrail.value != 0 {
					switch r_rockettrail.value {
					case 2:
						R_ParticleTrail(old_origin, ent.origin, &cent.trail_origin, GRENADE_TRAIL)
					case 4:
						R_ParticleTrail(old_origin, ent.origin, &cent.trail_origin, BLOOD_TRAIL)
					case 5:
						R_ParticleTrail(old_origin, ent.origin, &cent.trail_origin, BIG_BLOOD_TRAIL)
					case 6:
						R_ParticleTrail(old_origin, ent.origin, &cent.trail_origin, TRACER1_TRAIL)
					case 7:
						R_ParticleTrail(old_origin, ent.origin, &cent.trail_origin, TRACER2_TRAIL)
					case 8:
						R_ParticleTrail(old_origin, ent.origin, &cent.trail_origin, VOOR_TRAIL)
					case 3:
						R_ParticleTrail(old_origin, ent.origin, &cent.trail_origin, ALT_ROCKET_TRAIL)
					default:
						R_ParticleTrail(old_origin, ent.origin, &cent.trail_origin, ROCKET_TRAIL)
					}
				} else {
					cent.trail_origin = ent.origin
				}

				if r_rocketlight.value != 0 {
					rocketlightcolor := dlightColor(r_rocketlightcolor.value, lt_rocket, false)
					rocketlightsize := int(100 * (1 + bound(float32(0), r_rocketlight.value, float32(1))))
					CL_NewDlight(state.number, ent.origin, float32(rocketlightsize), 0.1, int(rocketlightcolor), 1)
				}
			} else if model.flags&EF_GRENADE != 0 {
				if r_grenadetrail.value != 0 {
					R_ParticleTrail(old_origin, ent.origin, &cent.trail_origin, GRENADE_TRAIL)
				} else {
					cent.trail_origin = ent.origin
				}
			} else if model.flags&EF_GIB != 0 {
				R_ParticleTrail(old_origin, ent.origin, &cent.trail_origin, BLOOD_TRAIL)
			} else if model.flags&EF_ZOMGIB != 0 {
				R_ParticleTrail(old_origin, ent.origin, &cent.trail_origin, BIG_BLOOD_TRAIL)
			} else if model.flags&EF_TRACER != 0 {
				R_ParticleTrail(old_origin, ent.origin, &cent.trail_origin, TRACER1_TRAIL)
			} else if model.flags&EF_TRACER2 != 0 {
				R_ParticleTrail(old_origin, ent.origin, &cent.trail_origin, TRACER2_TRAIL)
			} else if model.flags&EF_TRACER3 != 0 {
				R_ParticleTrail(old_origin, ent.origin, &cent.trail_origin, VOOR_TRAIL)
			}
		}

		cent.lerp_origin = ent.origin
		CL_AddEntity(&ent)
	}
}

type projectile_t struct {
	modelindex int
	origin     vec3_t
	angles     vec3_t
	num        int
}

var cl_projectiles [MAX_PROJECTILES]projectile_t
var cl_num_projectiles int

var cl_oldprojectiles [MAX_PROJECTILES]projectile_t
var cl_num_oldprojectiles int

var cl_projectiles_parsecount = 0

func CL_ClearProjectiles() {
	if cls.mvdplayback {
		if cl_projectiles_parsecount == cl.parsecount {
			return
		}
		cl_projectiles_parsecount = cl.parsecount

		cl.int_projectiles = [MAX_PROJECTILES]interpolate_t{}
		cl_num_oldprojectiles = cl_num_projectiles
		cl_oldprojectiles = cl_projectiles
	}

	cl_num_projectiles = 0
}

// Nails are passed as efficient temporary entities
func CL_ParseProjectiles(indexed bool) {
	var bits [6]int

	c := MSG_ReadByte()

	for i := 0; i < c; i++ {
		num := 0
		if indexed {
			num = MSG_ReadByte()
		}

		for j := 0; j < 6; j++ {
			bits[j] = MSG_ReadByte()
		}

		if cl_num_projectiles == MAX_PROJECTILES {
			continue
		}

		pr := &cl_projectiles[cl_num_projectiles]
		int_projectile := &cl.int_projectiles[cl_num_projectiles]
		cl_num_projectiles++

		pr.modelindex = cl_modelindices[mi_spike]
		pr.origin[0] = float32(((bits[0] + ((bits[1] & 15) << 8)) << 1) - 4096)
		pr.origin[1] = float32((((bits[1] >> 4) + (bits[2] << 4)) << 1) - 4096)
		pr.origin[2] = float32(((bits[3] + ((bits[4] & 15) << 8)) << 1) - 4096)
		pr.angles[0] = float32(360 * (bits[4] >> 4) / 16)
		pr.angles[1] = float32(360 * bits[5] / 256)

		pr.num = num
		if pr.num == 0 {
			continue
		}

		for j := 0; j < cl_num_oldprojectiles; j++ {
			if cl_oldprojectiles[j].num == num {
				int_projectile.interpolate = true
				int_projectile.oldindex = j
				int_projectile.origin = pr.origin
				break
			}
		}
	}
}

func CL_LinkProjectiles() {
	var ent entity_t

	for i := 0; i < cl_num_projectiles; i++ {
		pr := &cl_projectiles[i]
		if pr.modelindex < 1 {
			continue
		}
		ent.model = cl.model_precache[pr.modelindex]
		ent.origin = pr.origin
		ent.angles = pr.angles
		CL_AddEntity(&ent)
	}
}

func SetupPlayerEntity(num int, state *player_state_t) {
	cent := &cl_entities[num]

	if cl.oldparsecount == 0 || cl.oldparsecount != cent.sequence ||
		state.modelindex != cent.current.modelindex ||
		!VectorL2Compare(state.origin, cent.current.origin, 200) {
		cent.startlerp = cl.time
		cent.deltalerp = -1
		cent.frametime = -1
	} else {
		if state.frame != cent.current.frame {
			cent.frametime = cl.time
			cent.oldframe = cent.current.frame
		}

		if !VectorCompare(state.origin, cent.current.origin) || !VectorCompare(state.viewangles, cent.current.angles) {
			cent.old_origin = cent.current.origin
			cent.old_angles = cent.current.angles
			if cls.mvdplayback {
				cent.deltalerp = float64(nextdemotime - olddemotime)
				cent.startlerp = cls.demotime
			} else {
				cent.deltalerp = cl.time - cent.startlerp
				cent.startlerp = cl.time
			}
		}
	}

	cent.current.origin = state.origin
	cent.current.angles = state.viewangles
	cent.current.frame = state.frame
	cent.sequence = state.messagenum
	cent.current.modelindex = state.modelindex
}

var oldplayerstates [MAX_CLIENTS]player_state_t

func CL_ParsePlayerinfo() {
	var dummy player_state_t

	num := MSG_ReadByte()
	if num >= MAX_CLIENTS {
		Host_Error("CL_ParsePlayerinfo: num >= MAX_CLIENTS")
	}

	info := &cl.players[num]
	state := &cl.frames[parsecountmod].playerstate[num]
	cent := &cl_entities[num+1]

	if cls.mvdplayback {
		var prevstate *player_state_t
		if cl.parsecount == 0 || cent.sequence > cl.parsecount || cl.parsecount-cent.sequence >= UPDATE_BACKUP-1 {
			prevstate = &dummy
		} else {
			prevstate = &cl.frames[cent.sequence&UPDATE_MASK].playerstate[num]
		}

		*state = *prevstate

		if cls.findtrack && info.stats[STAT_HEALTH] > 0 {
			autocam = CAM_TRACK
			Cam_Lock(num)
			ideal_track = num
			cls.findtrack = false
		}

		flags := MSG_ReadShort()
		state.flags = MVD_TranslateFlags(flags)

		state.messagenum = cl.parsecount
		state.command.msec = 0

		state.frame = MSG_ReadByte()

		state.state_time = parsecounttime
		state.command.msec = 0

		for i := 0; i < 3; i++ {
			if flags&(DF_ORIGIN<<uint(i)) != 0 {
				state.origin[i] = MSG_ReadCoord()
			}
		}

		for i := 0; i < 3; i++ {
			if flags&(DF_ANGLES<<uint(i)) != 0 {
				state.command.angles[i] = MSG_ReadAngle16()
			}
		}

		if flags&DF_MODEL != 0 {
			state.modelindex = MSG_ReadByte()
		}

		if flags&DF_SKINNUM != 0 {
			state.skinnum = MSG_ReadByte()
		}

		if flags&DF_EFFECTS != 0 {
			state.effects = MSG_ReadByte()
		}

		if flags&DF_WEAPONFRAME != 0 {
			state.weaponframe = MSG_ReadByte()
		}
	} else {
		flags := int(uint16(MSG_ReadShort()))
		if cls.ftexsupported&FTEX_TRANS != 0 && flags&PF_EXTRA_PFS != 0 {
			flags |= MSG_ReadByte() << 16
		}
		state.flags = flags

		state.messagenum = cl.parsecount
		state.origin[0] = MSG_ReadCoord()
		state.origin[1] = MSG_ReadCoord()
		state.origin[2] = MSG_ReadCoord()

		state.frame = MSG_ReadByte()

		// the other player's last move was likely some time before the
		// packet was sent out, so accurately track the exact time it was
		// valid at
		if flags&PF_MSEC != 0 {
			msec := MSG_ReadByte()
			state.state_time = parsecounttime - float64(msec)*0.001
		} else {
			state.state_time = parsecounttime
		}

		if flags&PF_COMMAND != 0 {
			MSG_ReadDeltaUsercmd(&nullcmd, &state.command, cl.protoversion)
		}

		for i := 0; i < 3; i++ {
			if flags&(PF_VELOCITY1<<uint(i)) != 0 {
				state.velocity[i] = float32(MSG_ReadShort())
			} else {
				state.velocity[i] = 0
			}
		}

		if flags&PF_MODEL != 0 {
			state.modelindex = MSG_ReadByte()
		} else {
			state.modelindex = cl_modelindices[mi_player]
		}

		if flags&PF_SKINNUM != 0 {
			state.skinnum = MSG_ReadByte()
		} else {
			state.skinnum = 0
		}

		if flags&PF_EFFECTS != 0 {
			state.effects = MSG_ReadByte()
		} else {
			state.effects = 0
		}

		if flags&PF_WEAPONFRAME != 0 {
			state.weaponframe = MSG_ReadByte()
		} else {
			state.weaponframe = 0
		}

		if cls.ftexsupported&FTEX_TRANS != 0 && flags&PF_TRANS_Z != 0 {
			MSG_ReadByte()
		}

		guess_pm_type := true
		if cl.z_ext&Z_EXT_PM_TYPE != 0 {
			guess_pm_type = false
			pm_code := (flags >> PF_PMC_SHIFT) & PF_PMC_MASK
			if pm_code == PMC_NORMAL || pm_code == PMC_NORMAL_JUMP_HELD {
				if flags&PF_DEAD != 0 {
					state.pm_type = int(PM_DEAD)
				} else {
					state.pm_type = int(PM_NORMAL)
					state.jump_held = pm_code == PMC_NORMAL_JUMP_HELD
				}
			} else if pm_code == PMC_OLD_SPECTATOR {
				state.pm_type = int(PM_OLD_SPECTATOR)
			} else if cl.z_ext&Z_EXT_PM_TYPE_NEW != 0 && pm_code == PMC_SPECTATOR {
				state.pm_type = int(PM_SPECTATOR)
			} else if cl.z_ext&Z_EXT_PM_TYPE_NEW != 0 && pm_code == PMC_FLY {
				state.pm_type = int(PM_FLY)
			} else {
				// future extension?
				guess_pm_type = true
			}
		}
		if guess_pm_type {
			if cl.players[num].spectator != 0 {
				state.pm_type = int(PM_OLD_SPECTATOR)
			} else if flags&PF_DEAD != 0 {
				state.pm_type = int(PM_DEAD)
			} else {
				state.pm_type = int(PM_NORMAL)
			}
		}
	}

	state.viewangles = state.command.angles

	TP_ParsePlayerInfo(&oldplayerstates[num], state, info)

	oldplayerstates[num] = *state

	SetupPlayerEntity(num+1, state)
}

// Called when the CTF flags are set
func CL_AddFlagModels(ent *entity_t, team int) {
	var v_forward, v_right vec3_t
	var newent entity_t

	if cl_modelindices[mi_flag] == -1 {
		return
	}

	var f float32 = 14
	if ent.frame >= 29 && ent.frame <= 40 {
		if ent.frame >= 29 && ent.frame <= 34 { // axpain
			switch ent.frame {
			case 29:
				f = f + 2
			case 30:
				f = f + 8
			case 31:
				f = f + 12
			case 32:
				f = f + 11
			case 33:
				f = f + 10
			case 34:
				f = f + 4
			}
		} else if ent.frame >= 35 && ent.frame <= 40 { // pain
			switch ent.frame {
			case 35:
				f = f + 2
			case 36:
				f = f + 10
			case 37:
				f = f + 10
			case 38:
				f = f + 8
			case 39:
				f = f + 4
			case 40:
				f = f + 2
			}
		}
	} else if ent.frame >= 103 && ent.frame <= 118 {
		if ent.frame >= 103 && ent.frame <= 104 {
			f = f + 6 // nailattack
		} else if ent.frame >= 105 && ent.frame <= 106 {
			f = f + 6 // light
		} else if ent.frame >= 107 && ent.frame <= 112 {
			f = f + 7 // rocketattack
		} else if ent.frame >= 112 && ent.frame <= 118 {
			f = f + 7 // shotattack
		}
	}

	newent.model = cl.model_precache[cl_modelindices[mi_flag]]
	newent.skinnum = team

	AngleVectors(ent.angles, &v_forward, &v_right, nil)
	v_forward[2] = -v_forward[2] // reverse z component
	for i := 0; i < 3; i++ {
		newent.origin[i] = ent.origin[i] - f*v_forward[i] + 22*v_right[i]
	}
	newent.origin[2] -= 16

	newent.angles = ent.angles
	newent.angles[2] -= 45

	CL_AddEntity(&newent)
}

// Create visible entities in the correct position for all current players
func CL_LinkPlayers() {
	var ent entity_t
	var exact player_state_t

	playertime := cls.realtime - float64(cls.latency) + 0.02
	if playertime > cls.realtime {
		playertime = cls.realtime
	}

	frame := &cl.frames[cl.parsecount&UPDATE_MASK]

	for j := 0; j < MAX_CLIENTS; j++ {
		info := &cl.players[j]
		state := &frame.playerstate[j]
		cent := &cl_entities[j+1]

		if state.messagenum != cl.parsecount {
			continue // not present this frame
		}

		// spawn light flashes, even ones coming from invisible objects
		if r_powerupglow.value != 0 && !(r_powerupglow.value == 2 && j == cl.viewplayernum) {
			org := state.origin
			if j == cl.playernum {
				org = cl.simorg
			}
			flicker := 0
			if r_lightflicker.value != 0 {
				flicker = rand() & 31
			}
			if state.effects&(EF_BLUE|EF_RED) == (EF_BLUE | EF_RED) {
				CL_NewDlight(j+1, org, float32(200+flicker), 0.1, int(lt_redblue), 0)
			} else if state.effects&EF_BLUE != 0 {
				CL_NewDlight(j+1, org, float32(200+flicker), 0.1, int(lt_blue), 0)
			} else if state.effects&EF_RED != 0 {
				CL_NewDlight(j+1, org, float32(200+flicker), 0.1, int(lt_red), 0)
			} else if state.effects&EF_BRIGHTLIGHT != 0 {
				tmp := org
				tmp[2] += 16
				CL_NewDlight(j+1, tmp, float32(400+flicker), 0.1, int(lt_default), 0)
			} else if state.effects&EF_DIMLIGHT != 0 {
				var dimlightcolor dlighttype_t
				if cl.teamfortress {
					dimlightcolor = dlightColor(r_flagcolor.value, lt_default, false)
				} else if state.effects&(EF_FLAG1|EF_FLAG2) != 0 {
					dimlightcolor = dlightColor(r_flagcolor.value, lt_default, false)
				} else {
					dimlightcolor = lt_default
				}
				CL_NewDlight(j+1, org, float32(200+flicker), 0.1, int(dimlightcolor), 0)
			}
		}

		// the player object never gets added
		if j == cl.playernum || !Cam_DrawPlayer(j) {
			continue
		}

		if state.modelindex == 0 {
			continue
		}

		if state.modelindex == cl_modelindices[mi_player] {
			i := state.frame
			if cl_deadbodyfilter.value == 2 {
				if ISDEAD(i) {
					continue
				}
			} else if cl_deadbodyfilter.value != 0 {
				if i == 49 || i == 60 || i == 69 || i == 84 || i == 93 || i == 102 {
					continue
				}
			}
		}

		if ent.model = cl.model_precache[state.modelindex]; ent.model == nil {
			Host_Error("CL_LinkPlayers: bad modelindex")
		}
		ent.skinnum = state.skinnum
		if state.modelindex == cl_modelindices[mi_player] {
			ent.scoreboard = info
		} else {
			ent.scoreboard = nil
		}

		ent.frame = state.frame
		if cent.frametime >= 0 && cent.frametime <= cl.time {
			ent.oldframe = cent.oldframe
			ent.framelerp = float32((cl.time - cent.frametime) * 10)
		} else {
			ent.oldframe = ent.frame
			ent.framelerp = -1
		}

		// angles
		ent.angles[PITCH] = -state.viewangles[PITCH] / 3
		ent.angles[YAW] = state.viewangles[YAW]
		ent.angles[ROLL] = 0
		ent.angles[ROLL] = 4 * V_CalcRoll(ent.angles, state.velocity)

		// only predict half the move to minimize overruns
		msec := int(500 * (playertime - state.state_time))
		if msec <= 0 || cl_predictPlayers.value == 0 || cls.mvdplayback {
			ent.origin = state.origin
		} else {
			// predict players movement
			if msec > 255 {
				msec = 255
			}
			state.command.msec = byte(msec)

			oldphysent := pmove.numphysent
			CL_SetSolidPlayers(j)
			CL_PredictUsercmd(state, &exact, &state.command)
			pmove.numphysent = oldphysent
			ent.origin = exact.origin
		}

		if state.effects&(EF_FLAG1|EF_FLAG2) != 0 {
			CL_AddFlagModels(&ent, b2i(state.effects&EF_FLAG2 != 0))
		}

		cent.lerp_origin = ent.origin
		CL_AddEntity(&ent)
	}
}

// Builds all the pmove physents for the current frame
func CL_SetSolidEntities() {
	pmove.physents[0].model = cl.worldmodel
	pmove.physents[0].origin = vec3_t{}
	pmove.physents[0].info = 0
	pmove.numphysent = 1

	frame := &cl.frames[parsecountmod]
	pak := &frame.packet_entities

	for i := 0; i < pak.num_entities; i++ {
		state := &pak.entities[i]

		if pmove.numphysent == MAX_PHYSENTS {
			break
		}

		if state.modelindex == 0 {
			continue
		}
		if cl.model_precache[state.modelindex] == nil {
			continue
		}
		if cl.model_precache[state.modelindex].hulls[1].firstclipnode != 0 {
			pmove.physents[pmove.numphysent].model = cl.model_precache[state.modelindex]
			pmove.physents[pmove.numphysent].origin = state.origin
			pmove.numphysent++
		}
	}
}

// Calculate the new position of players, without other player clipping
// We do this to set up real player prediction.
// Players are predicted twice, first without clipping other players,
// then with clipping against them.
// This sets up the first phase.
func CL_SetUpPlayerPrediction(dopred bool) {
	var exact player_state_t

	playertime := cls.realtime - float64(cls.latency) + 0.02
	if playertime > cls.realtime {
		playertime = cls.realtime
	}

	frame := &cl.frames[cl.parsecount&UPDATE_MASK]

	for j := 0; j < MAX_CLIENTS; j++ {
		pplayer := &predicted_players[j]
		state := &frame.playerstate[j]

		pplayer.active = false

		if state.messagenum != cl.parsecount {
			continue // not present this frame
		}

		if state.modelindex == 0 {
			continue
		}

		pplayer.active = true
		pplayer.flags = state.flags

		// note that the local player is special, since he moves locally
		// we use his last predicted postition
		if j == cl.playernum {
			pplayer.origin = cl.frames[cls.netchan.outgoing_sequence&UPDATE_MASK].playerstate[cl.playernum].origin
		} else {
			// only predict half the move to minimize overruns
			msec := int(500 * (playertime - state.state_time))
			if msec <= 0 || cl_predictPlayers.value == 0 || !dopred || cls.mvdplayback {
				pplayer.origin = state.origin
			} else {
				// predict players movement
				if msec > 255 {
					msec = 255
				}
				state.command.msec = byte(msec)

				CL_PredictUsercmd(state, &exact, &state.command)
				pplayer.origin = exact.origin
			}
		}
	}
}

// Builds all the pmove physents for the current frame.
// Note that CL_SetUpPlayerPrediction() must be called first!
// pmove must be setup with world and solid entity hulls before calling
// (via CL_PredictMove)
func CL_SetSolidPlayers(playernum int) {
	if cl_solidPlayers.value == 0 {
		return
	}

	for j := 0; j < MAX_CLIENTS; j++ {
		pplayer := &predicted_players[j]

		if pmove.numphysent == MAX_PHYSENTS {
			break
		}

		if !pplayer.active {
			continue // not present this frame
		}

		// the player object never gets added
		if j == playernum {
			continue
		}

		if pplayer.flags&PF_DEAD != 0 {
			continue // dead players aren't solid
		}

		pent := &pmove.physents[pmove.numphysent]
		pent.model = nil
		pent.origin = pplayer.origin
		pent.mins = player_mins
		pent.maxs = player_maxs
		pmove.numphysent++
	}
}

// Builds the visedicts array for cl.time
// Made up of: clients, packet_entities, nails, and tents
func CL_EmitEntities() {
	if cls.state != ca_active {
		return
	}
	if cl.validsequence == 0 {
		return
	}

	CL_ClearScene()

	CL_LinkPlayers()
	CL_LinkPacketEntities()
	CL_LinkProjectiles()
	CL_UpdateTEnts()
}

var mvd_fixangle int

func MVD_TranslateFlags(src int) int {
	dst := 0

	if src&DF_EFFECTS != 0 {
		dst |= PF_EFFECTS
	}
	if src&DF_SKINNUM != 0 {
		dst |= PF_SKINNUM
	}
	if src&DF_DEAD != 0 {
		dst |= PF_DEAD
	}
	if src&DF_GIB != 0 {
		dst |= PF_GIB
	}
	if src&DF_WEAPONFRAME != 0 {
		dst |= PF_WEAPONFRAME
	}
	if src&DF_MODEL != 0 {
		dst |= PF_MODEL
	}

	return dst
}

func MVD_AdjustAngle(current, ideal, fraction float32) float32 {
	move := ideal - current
	if move >= 180 {
		move -= 360
	} else if move <= -180 {
		move += 360
	}
	return current + fraction*move
}

func MVD_InitInterpolation() {
	var dist vec3_t

	if cl.validsequence == 0 {
		return
	}

	if nextdemotime <= olddemotime {
		return
	}

	frame := &cl.frames[cl.parsecount&UPDATE_MASK]
	oldframe := &cl.frames[cl.oldparsecount&UPDATE_MASK]

	// clients
	for i := 0; i < MAX_CLIENTS; i++ {
		pplayer := &predicted_players[i]
		state := &frame.playerstate[i]
		oldstate := &oldframe.playerstate[i]

		if pplayer.predict {
			oldstate.origin = pplayer.oldo
			oldstate.command.angles = pplayer.olda
			oldstate.velocity = pplayer.oldv
		}

		pplayer.predict = false

		tracknum := Cam_TrackNum()
		if mvd_fixangle&(1<<uint(i)) != 0 {
			if i == tracknum {
				state.command.angles = cl.viewangles
				state.viewangles = cl.viewangles
			}

			// no angle interpolation
			oldstate.command.angles = state.command.angles

			mvd_fixangle &^= 1 << uint(i)
		}

		// we dont interpolate ourself if we are spectating
		if i == cl.playernum && cl.spectator != 0 {
			continue
		}

		state.velocity = vec3_t{}

		if state.messagenum != cl.parsecount {
			continue // not present this frame
		}

		if oldstate.messagenum != cl.oldparsecount || oldstate.messagenum == 0 {
			continue // not present last frame
		}

		if !ISDEAD(state.frame) && ISDEAD(oldstate.frame) {
			continue
		}

		VectorSubtract(state.origin, oldstate.origin, &dist)
		if DotProduct(dist, dist) > 22500 {
			continue
		}

		VectorScale(dist, 1/(nextdemotime-olddemotime), &pplayer.oldv)
		pplayer.oldo = state.origin
		pplayer.olda = state.command.angles

		pplayer.oldstate = oldstate
		pplayer.predict = true
	}

	// nails
	for i := 0; i < cl_num_projectiles; i++ {
		if !cl.int_projectiles[i].interpolate {
			continue
		}

		cl_projectiles[i].origin = cl.int_projectiles[i].origin
	}
}

var mvd_interpolate_old float32

func MVD_Interpolate() {
	self := &cl.frames[cl.parsecount&UPDATE_MASK].playerstate[cl.playernum]
	oldself := &cl.frames[(cls.netchan.outgoing_sequence-1)&UPDATE_MASK].playerstate[cl.playernum]

	self.messagenum = cl.parsecount

	self.origin = oldself.origin
	self.velocity = oldself.velocity
	self.viewangles = oldself.viewangles

	if mvd_interpolate_old != nextdemotime {
		mvd_interpolate_old = nextdemotime
		MVD_InitInterpolation()
	}

	CL_ParseClientdata()

	cls.netchan.outgoing_sequence = cl.parsecount + 1

	if cl.validsequence == 0 {
		return
	}

	if nextdemotime <= olddemotime {
		return
	}

	frame := &cl.frames[cl.parsecount&UPDATE_MASK]
	oldframe := &cl.frames[cl.oldparsecount&UPDATE_MASK]

	f := float32(bound(0.0, (cls.demotime-float64(olddemotime))/float64(nextdemotime-olddemotime), 1.0))

	// interpolate nails
	for i := 0; i < cl_num_projectiles; i++ {
		if !cl.int_projectiles[i].interpolate {
			continue
		}

		old := &cl_oldprojectiles[cl.int_projectiles[i].oldindex]
		for j := 0; j < 3; j++ {
			cl_projectiles[i].origin[j] = old.origin[j] + f*(cl.int_projectiles[i].origin[j]-old.origin[j])
		}
	}

	// interpolate clients
	for i := 0; i < MAX_CLIENTS; i++ {
		pplayer := &predicted_players[i]
		state := &frame.playerstate[i]
		oldstate := &oldframe.playerstate[i]

		if pplayer.predict {
			for j := 0; j < 3; j++ {
				state.viewangles[j] = MVD_AdjustAngle(oldstate.command.angles[j], pplayer.olda[j], f)
				state.origin[j] = oldstate.origin[j] + f*(pplayer.oldo[j]-oldstate.origin[j])
				state.velocity[j] = oldstate.velocity[j] + f*(pplayer.oldv[j]-oldstate.velocity[j])
			}
		}
	}
}

func CL_ClearPredict() {
	predicted_players = [MAX_CLIENTS]predicted_player{}
	mvd_fixangle = 0
}

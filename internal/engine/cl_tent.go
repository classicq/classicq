package engine

import "math"

// cl_tent.c -- client side temporary entities

const MAX_BEAMS = 32

type beam_t struct {
	entity     int
	model      *model_t
	endtime    float32
	start, end vec3_t
}

var cl_beams [MAX_BEAMS]beam_t

var playerbeam_end vec3_t

const MAX_EXPLOSIONS = 32

type explosion_t struct {
	prev, next *explosion_t
	origin     vec3_t
	start      float32
	model      *model_t
}

// cl_free_explosions = linear linked list of free explosions
var cl_explosions [MAX_EXPLOSIONS]explosion_t
var cl_explosions_headnode explosion_t
var cl_free_explosions *explosion_t

var cl_explo_mod, cl_bolt1_mod, cl_bolt2_mod, cl_bolt3_mod *model_t

var cl_sfx_wizhit, cl_sfx_knighthit, cl_sfx_tink1, cl_sfx_ric1, cl_sfx_ric2, cl_sfx_ric3, cl_sfx_r_exp3 *sfx_t

func CL_InitTEnts() {
	cl_sfx_wizhit = S_PrecacheSound("wizard/hit.wav")
	cl_sfx_knighthit = S_PrecacheSound("hknight/hit.wav")
	cl_sfx_tink1 = S_PrecacheSound("weapons/tink1.wav")
	cl_sfx_ric1 = S_PrecacheSound("weapons/ric1.wav")
	cl_sfx_ric2 = S_PrecacheSound("weapons/ric2.wav")
	cl_sfx_ric3 = S_PrecacheSound("weapons/ric3.wav")
	cl_sfx_r_exp3 = S_PrecacheSound("weapons/r_exp3.wav")
}

func CL_ClearTEnts() {
	cl_explo_mod = nil
	cl_bolt1_mod = nil
	cl_bolt2_mod = nil
	cl_bolt3_mod = nil

	cl_beams = [MAX_BEAMS]beam_t{}
	cl_explosions = [MAX_EXPLOSIONS]explosion_t{}

	// link explosions
	cl_free_explosions = &cl_explosions[0]
	cl_explosions_headnode.next = &cl_explosions_headnode
	cl_explosions_headnode.prev = &cl_explosions_headnode

	for i := 0; i < MAX_EXPLOSIONS-1; i++ {
		cl_explosions[i].next = &cl_explosions[i+1]
	}
}

func CL_AllocExplosion() *explosion_t {
	var ex *explosion_t

	if cl_free_explosions != nil {
		// take a free explosion if possible
		ex = cl_free_explosions
		cl_free_explosions = ex.next
	} else {
		// grab the oldest one otherwise
		ex = cl_explosions_headnode.prev
		ex.prev.next = ex.next
		ex.next.prev = ex.prev
	}

	// put the explosion at the start of the list
	ex.prev = &cl_explosions_headnode
	ex.next = cl_explosions_headnode.next
	ex.next.prev = ex
	ex.prev.next = ex

	return ex
}

func CL_FreeExplosion(ex *explosion_t) {
	// remove from linked active list
	ex.prev.next = ex.next
	ex.next.prev = ex.prev

	// insert into front linked free list
	ex.next = cl_free_explosions
	cl_free_explosions = ex
}

func CL_ParseBeam(m *model_t) {
	var start, end vec3_t

	ent := MSG_ReadShort()

	start[0] = MSG_ReadCoord()
	start[1] = MSG_ReadCoord()
	start[2] = MSG_ReadCoord()

	end[0] = MSG_ReadCoord()
	end[1] = MSG_ReadCoord()
	end[2] = MSG_ReadCoord()

	if ent == cl.viewplayernum+1 {
		playerbeam_end = end // for cl_trueLightning
	}

	// override any beam with the same entity
	for i := 0; i < MAX_BEAMS; i++ {
		b := &cl_beams[i]
		if b.entity == ent {
			b.model = m
			b.endtime = float32(cl.time + 0.2)
			b.start = start
			b.end = end
			return
		}
	}

	// find a free beam
	for i := 0; i < MAX_BEAMS; i++ {
		b := &cl_beams[i]
		if b.model == nil || float64(b.endtime) < cl.time {
			b.entity = ent
			b.model = m
			b.endtime = float32(cl.time + 0.2)
			b.start = start
			b.end = end
			return
		}
	}
	Com_DPrintf("beam list overflow!\n")
}

func CL_ExplosionSprite(pos vec3_t) {
	ex := CL_AllocExplosion()
	ex.origin = pos
	ex.start = float32(cl.time)
	if cl_explo_mod == nil {
		cl_explo_mod = Mod_ForName("progs/s_explod.spr", true)
	}
	ex.model = cl_explo_mod
}

func cl_readTEntPos() vec3_t {
	var pos vec3_t
	pos[0] = MSG_ReadCoord()
	pos[1] = MSG_ReadCoord()
	pos[2] = MSG_ReadCoord()
	return pos
}

func cl_spikeSound(pos vec3_t) {
	if rand()%5 != 0 {
		S_StartSound(-1, 0, cl_sfx_tink1, pos, 1, 1)
	} else {
		rnd := rand() & 3
		if rnd == 1 {
			S_StartSound(-1, 0, cl_sfx_ric1, pos, 1, 1)
		} else if rnd == 2 {
			S_StartSound(-1, 0, cl_sfx_ric2, pos, 1, 1)
		} else {
			S_StartSound(-1, 0, cl_sfx_ric3, pos, 1, 1)
		}
	}
}

func CL_ParseTEnt() {
	type_ := MSG_ReadByte()
	switch type_ {
	case TE_WIZSPIKE: // spike hitting wall
		pos := cl_readTEntPos()
		R_RunParticleEffect(pos, vec3_origin, 20, 30)
		S_StartSound(-1, 0, cl_sfx_wizhit, pos, 1, 1)

	case TE_KNIGHTSPIKE: // spike hitting wall
		pos := cl_readTEntPos()
		R_RunParticleEffect(pos, vec3_origin, 226, 20)
		S_StartSound(-1, 0, cl_sfx_knighthit, pos, 1, 1)

	case TE_SPIKE: // spike hitting wall
		pos := cl_readTEntPos()
		R_RunParticleEffect(pos, vec3_origin, 0, 10)
		cl_spikeSound(pos)

	case TE_SUPERSPIKE: // super spike hitting wall
		pos := cl_readTEntPos()
		R_RunParticleEffect(pos, vec3_origin, 0, 20)
		cl_spikeSound(pos)

	case TE_EXPLOSION: // rocket explosion
		pos := cl_readTEntPos()
		if r_explosiontype.value == 2 {
			R_TeleportSplash(pos) // teleport splash
		} else if r_explosiontype.value == 3 {
			R_RunParticleEffect(pos, vec3_origin, 225, 50) // lightning blood
		} else if r_explosiontype.value == 4 {
			R_RunParticleEffect(pos, vec3_origin, 73, 20*32) // big blood
		} else if r_explosiontype.value == 5 {
			R_RunParticleEffect(pos, vec3_origin, 0, 20*14) // dbl gunshot
		} else if r_explosiontype.value == 6 {
			R_BlobExplosion(pos) // blob explosion
		} else if r_explosiontype.value == 7 && qmb_initialized && gl_part_explosions.value != 0 {
			QMB_DetpackExplosion(pos) // detpack explosion
		} else { // sprite and particles
			R_ParticleExplosion(pos) // normal explosion
		}

		if r_explosionlight.value != 0 {
			dl := CL_AllocDlight(0)
			dl.origin = pos
			dl.radius = 150 + 200*bound(float32(0), r_explosionlight.value, float32(1))
			dl.die = float32(cl.time + 0.5)
			dl.decay = 300
			dl.type_ = int(dlightColor(r_explosionlightcolor.value, lt_explosion, true))
		}
		S_StartSound(-1, 0, cl_sfx_r_exp3, pos, 1, 1)

	case TE_TAREXPLOSION: // tarbaby explosion
		pos := cl_readTEntPos()
		R_BlobExplosion(pos)
		S_StartSound(-1, 0, cl_sfx_r_exp3, pos, 1, 1)

	case TE_LIGHTNING1: // lightning bolts
		if cl_bolt1_mod == nil {
			cl_bolt1_mod = Mod_ForName("progs/bolt.mdl", true)
		}
		CL_ParseBeam(cl_bolt1_mod)

	case TE_LIGHTNING2: // lightning bolts
		if cl_bolt2_mod == nil {
			cl_bolt2_mod = Mod_ForName("progs/bolt2.mdl", true)
		}
		CL_ParseBeam(cl_bolt2_mod)

	case TE_LIGHTNING3: // lightning bolts
		if cl_bolt3_mod == nil {
			cl_bolt3_mod = Mod_ForName("progs/bolt3.mdl", true)
		}
		CL_ParseBeam(cl_bolt3_mod)

	case TE_LAVASPLASH:
		pos := cl_readTEntPos()
		R_LavaSplash(pos)

	case TE_TELEPORT:
		pos := cl_readTEntPos()
		R_TeleportSplash(pos)

	case TE_GUNSHOT: // bullet hitting wall
		cnt := MSG_ReadByte()
		pos := cl_readTEntPos()
		R_RunParticleEffect(pos, vec3_origin, 0, 20*cnt)

	case TE_BLOOD: // bullets hitting body
		cnt := MSG_ReadByte()
		pos := cl_readTEntPos()
		R_RunParticleEffect(pos, vec3_origin, 73, 20*cnt)

	case TE_LIGHTNINGBLOOD: // lightning hitting body
		pos := cl_readTEntPos()
		R_RunParticleEffect(pos, vec3_origin, 225, 50)

	default:
		Host_Error("CL_ParseTEnt: bad type")
	}
}

func CL_UpdateBeams() {
	var ent entity_t
	var dist, org vec3_t
	var yaw, pitch float32

	truelightning := bound(float32(0), cl_trueLightning.value, cl.truelightning)

	// update lightning
	for i := 0; i < MAX_BEAMS; i++ {
		b := &cl_beams[i]
		if b.model == nil || float64(b.endtime) < cl.time {
			continue
		}

		// if coming from the player, update the start position
		if b.entity == cl.viewplayernum+1 {
			b.start = cl.simorg
			b.start[2] += cl.crouch + bound(float32(-7), v_viewheight.value, float32(4))
			if cl_trueLightning.value != 0 {
				var forward, v, org, ang vec3_t
				var destangles vec3_t

				if cl_trueLightning.value == 2 {
					frametime := NetQW_GetFrameTime(cls.netqw)
					if uint(cls.netchan.outgoing_sequence) > attack_outgoing_sequence {
						mytime := uint64(uint(cls.netchan.outgoing_sequence-1)-attack_outgoing_sequence) * frametime
						mytime = mytime - (mytime % 100000) + (frametime - 1)
						myseqnum := attack_outgoing_sequence + uint(mytime/frametime)
						destangles = cl.frames[myseqnum&UPDATE_MASK].cmd.angles
					} else {
						destangles = cl.frames[(cls.netchan.outgoing_sequence-1)&UPDATE_MASK].cmd.angles
					}
				} else {
					destangles = cl.simangles
				}

				VectorSubtract(playerbeam_end, cl.simorg, &v)
				v[2] -= 22 // adjust for view height
				vectoangles(v, &ang)

				// lerp pitch
				ang[0] = -ang[0]
				if ang[0] < -180 {
					ang[0] += 360
				}
				ang[0] += (destangles[0] - ang[0]) * truelightning

				// lerp yaw
				delta := destangles[1] - ang[1]
				if delta > 180 {
					delta -= 360
				}
				if delta < -180 {
					delta += 360
				}
				ang[1] += delta * truelightning
				ang[2] = 0

				AngleVectors(ang, &forward, nil, nil)
				VectorScale(forward, 600, &forward)
				org = cl.simorg
				org[2] += 16
				VectorAdd(org, forward, &b.end)

				trace := PM_TraceLine(org, b.end)
				if trace.fraction < 1 {
					b.end = trace.endpos
				}
			}
		}

		// calculate pitch and yaw
		VectorSubtract(b.end, b.start, &dist)

		if dist[1] == 0 && dist[0] == 0 {
			yaw = 0
			if dist[2] > 0 {
				pitch = 90
			} else {
				pitch = 270
			}
		} else {
			yaw = float32(math.Atan2(float64(dist[1]), float64(dist[0])) * 180 / M_PI)
			if yaw < 0 {
				yaw += 360
			}

			forward := float32(math.Sqrt(float64(dist[0]*dist[0] + dist[1]*dist[1])))
			pitch = float32(math.Atan2(float64(dist[2]), float64(forward)) * 180 / M_PI)
			if pitch < 0 {
				pitch += 360
			}
		}

		// add new entities for the lightning
		org = b.start
		d := VectorNormalize(&dist)
		VectorScale(dist, 30, &dist)
		for ; d > 0; d -= 30 {
			ent.origin = org
			ent.model = b.model
			ent.angles[0] = pitch
			ent.angles[1] = yaw
			ent.angles[2] = float32(rand() % 360)

			CL_AddEntity(&ent)

			VectorAdd(org, dist, &org)
		}
	}
}

func CL_UpdateExplosions() {
	var ent entity_t

	hnode := &cl_explosions_headnode
	for ex := hnode.next; ex != hnode; {
		next := ex.next
		f := int(10 * (cl.time - float64(ex.start)))
		if f >= ex.model.numframes {
			CL_FreeExplosion(ex)
			ex = next
			continue
		}

		ent.origin = ex.origin
		ent.model = ex.model
		ent.frame = f

		CL_AddEntity(&ent)
		ex = next
	}
}

func CL_UpdateTEnts() {
	CL_UpdateBeams()
	CL_UpdateExplosions()
}

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

// r_part.c

const (
	DEFAULT_NUM_PARTICLES  = 2048
	ABSOLUTE_MIN_PARTICLES = 512
	ABSOLUTE_MAX_PARTICLES = 8192
)

// particles.h
type ptype_t int

const (
	pt_static ptype_t = iota
	pt_grav
	pt_slowgrav
	pt_fire
	pt_explode
	pt_explode2
	pt_blob
	pt_blob2
)

type particle_t struct {
	org   vec3_t
	color float32
	next  *particle_t
	vel   vec3_t
	ramp  float32
	die   float32
	type_ ptype_t
}

var ramp1 = [8]int{0x6f, 0x6d, 0x6b, 0x69, 0x67, 0x65, 0x63, 0x61}
var ramp2 = [8]int{0x6f, 0x6e, 0x6d, 0x6c, 0x6b, 0x6a, 0x68, 0x66}
var ramp3 = [8]int{0x6d, 0x6b, 6, 5, 4, 3}

var particles []particle_t
var active_particles, free_particles *particle_t

var r_numparticles int

func Classic_LoadParticleTextures() {
	var data [32 * 32]uint32

	particletexture = texture_extension_number
	texture_extension_number++
	R_Bind(particletexture)

	// clear to transparent white
	for i := 0; i < 32*32; i++ {
		data[i] = COLOURMASK_RGBA
	}

	// draw a square in the top left corner
	for x := 0; x < 16; x++ {
		for y := 0; y < 16; y++ {
			data[y*32+x] = 0xFFFFFFFF // solid white
		}
	}

	R_Upload32(u32Bytes(data[:]), 32, 32, TEX_MIPMAP|TEX_ALPHA|TEX_NOSCALE)
}

func Classic_InitParticles() int {
	if i := COM_CheckParm("-particles"); i != 0 && i+1 < com_argc {
		r_numparticles = Q_atoi(com_argv[i+1])
		r_numparticles = bound(ABSOLUTE_MIN_PARTICLES, r_numparticles, ABSOLUTE_MAX_PARTICLES)
	} else {
		r_numparticles = DEFAULT_NUM_PARTICLES
	}

	particles = make([]particle_t, r_numparticles)
	return 1
}

func Classic_ShutdownParticles() {
	particles = nil
}

func Classic_ClearParticles() {
	free_particles = &particles[0]
	active_particles = nil

	for i := 0; i < r_numparticles-1; i++ {
		particles[i].next = &particles[i+1]
	}
	particles[r_numparticles-1].next = nil
}

func Classic_ParticleExplosion(org vec3_t) {
	CL_ExplosionSprite(org)

	if r_explosiontype.value == 1 {
		return
	}

	for i := 0; i < 1024; i++ {
		if free_particles == nil {
			return
		}
		p := free_particles
		free_particles = p.next
		p.next = active_particles
		active_particles = p

		p.die = float32(cl.time + 5)
		p.color = float32(ramp1[0])
		p.ramp = float32(rand() & 3)
		if i&1 != 0 {
			p.type_ = pt_explode
			for j := 0; j < 3; j++ {
				p.org[j] = org[j] + float32((rand()%32)-16)
				p.vel[j] = float32((rand() % 512) - 256)
			}
		} else {
			p.type_ = pt_explode2
			for j := 0; j < 3; j++ {
				p.org[j] = org[j] + float32((rand()%32)-16)
				p.vel[j] = float32((rand() % 512) - 256)
			}
		}
	}
}

func Classic_BlobExplosion(org vec3_t) {
	for i := 0; i < 1024; i++ {
		if free_particles == nil {
			return
		}
		p := free_particles
		free_particles = p.next
		p.next = active_particles
		active_particles = p

		p.die = float32(cl.time + 1 + float64(rand()&8)*0.05)

		if i&1 != 0 {
			p.type_ = pt_blob
			p.color = float32(66 + rand()%6)
			for j := 0; j < 3; j++ {
				p.org[j] = org[j] + float32((rand()%32)-16)
				p.vel[j] = float32((rand() % 512) - 256)
			}
		} else {
			p.type_ = pt_blob2
			p.color = float32(150 + rand()%6)
			for j := 0; j < 3; j++ {
				p.org[j] = org[j] + float32((rand()%32)-16)
				p.vel[j] = float32((rand() % 512) - 256)
			}
		}
	}
}

func Classic_RunParticleEffect(org, dir vec3_t, color, count int) {
	scale := 1
	if count > 130 {
		scale = 3
	} else if count > 20 {
		scale = 2
	}

	for i := 0; i < count; i++ {
		if free_particles == nil {
			return
		}
		p := free_particles
		free_particles = p.next
		p.next = active_particles
		active_particles = p

		p.die = float32(cl.time + 0.1*float64(rand()%5))
		p.color = float32((color &^ 7) + (rand() & 7))
		p.type_ = pt_grav
		for j := 0; j < 3; j++ {
			p.org[j] = org[j] + float32(scale*((rand()&15)-8))
			p.vel[j] = dir[j] * 15
		}
	}
}

func Classic_LavaSplash(org vec3_t) {
	var dir vec3_t

	for i := -16; i < 16; i++ {
		for j := -16; j < 16; j++ {
			for k := 0; k < 1; k++ {
				if free_particles == nil {
					return
				}
				p := free_particles
				free_particles = p.next
				p.next = active_particles
				active_particles = p

				p.die = float32(cl.time + 2 + float64(rand()&31)*0.02)
				p.color = float32(224 + (rand() & 7))
				p.type_ = pt_grav

				dir[0] = float32(j*8 + (rand() & 7))
				dir[1] = float32(i*8 + (rand() & 7))
				dir[2] = 256

				p.org[0] = org[0] + dir[0]
				p.org[1] = org[1] + dir[1]
				p.org[2] = org[2] + float32(rand()&63)

				VectorNormalizeFast(&dir)
				vel := float32(50 + (rand() & 63))
				VectorScale(dir, vel, &p.vel)
			}
		}
	}
}

func Classic_TeleportSplash(org vec3_t) {
	var dir vec3_t

	for i := -16; i < 16; i += 4 {
		for j := -16; j < 16; j += 4 {
			for k := -24; k < 32; k += 4 {
				if free_particles == nil {
					return
				}
				p := free_particles
				free_particles = p.next
				p.next = active_particles
				active_particles = p

				p.die = float32(cl.time + 0.2 + float64(rand()&7)*0.02)
				p.color = float32(7 + (rand() & 7))
				p.type_ = pt_grav

				dir[0] = float32(j * 8)
				dir[1] = float32(i * 8)
				dir[2] = float32(k * 8)

				p.org[0] = org[0] + float32(i) + float32(rand()&3)
				p.org[1] = org[1] + float32(j) + float32(rand()&3)
				p.org[2] = org[2] + float32(k) + float32(rand()&3)

				VectorNormalizeFast(&dir)
				vel := float32(50 + (rand() & 63))
				VectorScale(dir, vel, &p.vel)
			}
		}
	}
}

var classic_tracercount int

func Classic_ParticleTrail(start, end vec3_t, trail_origin *vec3_t, type_ trail_type_t) {
	var delta, dir vec3_t

	point := start
	VectorSubtract(end, start, &delta)
	len_ := VectorLength(delta)
	if len_ != 0 {
		VectorScale(delta, 1/len_, &dir) // unit vector in direction of trail

		switch type_ {
		case ALT_ROCKET_TRAIL:
			len_ = float32(float64(len_) / 1.5)
		case BLOOD_TRAIL:
			len_ /= 6
		default:
			len_ /= 3
		}

		num_particles := int(len_)
		if num_particles != 0 {
			VectorScale(delta, float32(1.0/float64(num_particles)), &delta)

			for i := 0; i < num_particles && free_particles != nil; i++ {
				p := free_particles
				free_particles = p.next
				p.next = active_particles
				active_particles = p

				p.vel = vec3_t{}
				p.die = float32(cl.time + 2)

				switch type_ {
				case GRENADE_TRAIL:
					p.ramp = float32((rand() & 3) + 2)
					p.color = float32(ramp3[int(p.ramp)])
					p.type_ = pt_fire
					for j := 0; j < 3; j++ {
						p.org[j] = point[j] + float32((rand()%6)-3)
					}
				case BLOOD_TRAIL:
					p.type_ = pt_slowgrav
					p.color = float32(67 + (rand() & 3))
					for j := 0; j < 3; j++ {
						p.org[j] = point[j] + float32((rand()%6)-3)
					}
				case BIG_BLOOD_TRAIL:
					p.type_ = pt_slowgrav
					p.color = float32(67 + (rand() & 3))
					for j := 0; j < 3; j++ {
						p.org[j] = point[j] + float32((rand()%6)-3)
					}
				case TRACER1_TRAIL, TRACER2_TRAIL:
					p.die = float32(cl.time + 0.5)
					p.type_ = pt_static
					if type_ == TRACER1_TRAIL {
						p.color = float32(52 + ((classic_tracercount & 4) << 1))
					} else {
						p.color = float32(230 + ((classic_tracercount & 4) << 1))
					}

					classic_tracercount++

					p.org = point
					if classic_tracercount&1 != 0 {
						p.vel[0] = 90 * dir[1]
						p.vel[1] = 90 * -dir[0]
					} else {
						p.vel[0] = 90 * -dir[1]
						p.vel[1] = 90 * dir[0]
					}
				case VOOR_TRAIL:
					p.color = float32(9*16 + 8 + (rand() & 3))
					p.type_ = pt_static
					p.die = float32(cl.time + 0.3)
					for j := 0; j < 3; j++ {
						p.org[j] = point[j] + float32((rand()&15)-8)
					}
				case ALT_ROCKET_TRAIL:
					p.ramp = float32(rand() & 3)
					p.color = float32(ramp3[int(p.ramp)])
					p.type_ = pt_fire
					for j := 0; j < 3; j++ {
						p.org[j] = point[j] + float32((rand()%6)-3)
					}
				default: // ROCKET_TRAIL
					p.ramp = float32(rand() & 3)
					p.color = float32(ramp3[int(p.ramp)])
					p.type_ = pt_fire
					for j := 0; j < 3; j++ {
						p.org[j] = point[j] + float32((rand()%6)-3)
					}
				}
				VectorAdd(point, delta, &point)
			}
		}
	}
	*trail_origin = point
}

func Classic_DrawParticles() {
	if active_particles == nil {
		return
	}

	R_DrawParticleBegin()

	frametime := float32(cls.frametime)
	if cl.paused != 0 {
		frametime = 0
	}
	time3 := frametime * 15
	time2 := frametime * 10 // 15;
	time1 := frametime * 5
	grav := float32(float64(frametime*800) * 0.05)
	dvel := 4 * frametime

	for {
		kill := active_particles
		if kill != nil && float64(kill.die) < cl.time {
			active_particles = kill.next
			kill.next = free_particles
			free_particles = kill
			continue
		}
		break
	}

	for p := active_particles; p != nil; p = p.next {
		for {
			kill := p.next
			if kill != nil && float64(kill.die) < cl.time {
				p.next = kill.next
				kill.next = free_particles
				free_particles = kill
				continue
			}
			break
		}

		R_DrawParticle(p)

		p.org[0] += p.vel[0] * frametime
		p.org[1] += p.vel[1] * frametime
		p.org[2] += p.vel[2] * frametime

		switch p.type_ {
		case pt_static:

		case pt_fire:
			p.ramp += time1
			if p.ramp >= 6 {
				p.die = -1
			} else {
				p.color = float32(ramp3[int(p.ramp)])
			}
			p.vel[2] += grav

		case pt_explode:
			p.ramp += time2
			if p.ramp >= 8 {
				p.die = -1
			} else {
				p.color = float32(ramp1[int(p.ramp)])
			}
			for i := 0; i < 3; i++ {
				p.vel[i] += p.vel[i] * dvel
			}
			p.vel[2] -= grav * 30

		case pt_explode2:
			p.ramp += time3
			if p.ramp >= 8 {
				p.die = -1
			} else {
				p.color = float32(ramp2[int(p.ramp)])
			}
			for i := 0; i < 3; i++ {
				p.vel[i] -= p.vel[i] * frametime
			}
			p.vel[2] -= grav * 30

		case pt_blob:
			for i := 0; i < 3; i++ {
				p.vel[i] += p.vel[i] * dvel
			}
			p.vel[2] -= grav

		case pt_blob2:
			for i := 0; i < 2; i++ {
				p.vel[i] -= p.vel[i] * dvel
			}
			p.vel[2] -= grav

		case pt_slowgrav, pt_grav:
			p.vel[2] -= grav
		}
	}

	R_DrawParticleEnd()
}

func R_InitParticles() int {
	if Classic_InitParticles() != 0 {
		QMB_InitParticles()

		R_DrawParticleInit()

		return 1
	}

	return 0
}

func R_ShutdownParticles() {
	QMB_ShutdownParticles()
	Classic_ShutdownParticles()
}

func R_ClearParticles() {
	Classic_ClearParticles()
	QMB_ClearParticles()
}

func R_DrawParticles() {
	Classic_DrawParticles()
	QMB_DrawParticles()
}

func R_RunParticleEffect(org, dir vec3_t, color, count int) {
	if color == 73 || color == 225 {
		if qmb_initialized && gl_part_blood.value != 0 {
			QMB_RunParticleEffect(org, dir, color, count)
		} else {
			Classic_RunParticleEffect(org, dir, color, count)
		}
		return
	}

	switch count {
	case 10, 20, 30:
		if qmb_initialized && gl_part_spikes.value != 0 {
			QMB_RunParticleEffect(org, dir, color, count)
		} else {
			Classic_RunParticleEffect(org, dir, color, count)
		}
	default:
		if qmb_initialized && gl_part_gunshots.value != 0 {
			QMB_RunParticleEffect(org, dir, color, count)
		} else {
			Classic_RunParticleEffect(org, dir, color, count)
		}
	}
}

func R_ParticleTrail(start, end vec3_t, trail_origin *vec3_t, type_ trail_type_t) {
	if qmb_initialized && gl_part_trails.value != 0 {
		QMB_ParticleTrail(start, end, trail_origin, type_)
	} else {
		Classic_ParticleTrail(start, end, trail_origin, type_)
	}
}

func R_ParticleExplosion(org vec3_t) {
	if qmb_initialized && gl_part_explosions.value != 0 {
		QMB_ParticleExplosion(org)
	} else {
		Classic_ParticleExplosion(org)
	}
}

func R_BlobExplosion(org vec3_t) {
	if qmb_initialized && gl_part_blobs.value != 0 {
		QMB_BlobExplosion(org)
	} else {
		Classic_BlobExplosion(org)
	}
}

func R_LavaSplash(org vec3_t) {
	if qmb_initialized && gl_part_lavasplash.value != 0 {
		QMB_LavaSplash(org)
	} else {
		Classic_LavaSplash(org)
	}
}

func R_TeleportSplash(org vec3_t) {
	if qmb_initialized && gl_part_telesplash.value != 0 {
		QMB_TeleportSplash(org)
	} else {
		Classic_TeleportSplash(org)
	}
}

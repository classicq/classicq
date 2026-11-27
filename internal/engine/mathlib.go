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

// qtypes.h
type (
	vec_t  = float32
	vec3_t [3]float32
)

const (
	PITCH = 0 // up / down
	YAW   = 1 // left / right
	ROLL  = 2 // fall over
)

const M_PI = 3.14159265358979323846

// The C macros take floats and compute in double, hence float64 here.
func DEG2RAD(a float64) float64 { return (a * M_PI) / 180.0 }
func RAD2DEG(a float64) float64 { return (a * 180.0) / M_PI }

const NANMASK = 255 << 23

func IS_NAN(x float32) bool {
	return math.Float32bits(x)&NANMASK == NANMASK
}

func Q_rint(x float64) int {
	if x > 0 {
		return int(x + 0.5)
	}
	return int(x - 0.5)
}

func DotProduct(x, y vec3_t) float32 {
	return x[0]*y[0] + x[1]*y[1] + x[2]*y[2]
}

func VectorSubtract(a, b vec3_t, c *vec3_t) {
	c[0] = a[0] - b[0]
	c[1] = a[1] - b[1]
	c[2] = a[2] - b[2]
}

func VectorAdd(a, b vec3_t, c *vec3_t) {
	c[0] = a[0] + b[0]
	c[1] = a[1] + b[1]
	c[2] = a[2] + b[2]
}

func VectorNegate(a vec3_t, b *vec3_t) {
	b[0] = -a[0]
	b[1] = -a[1]
	b[2] = -a[2]
}

func CrossProduct(v1, v2 vec3_t, x *vec3_t) {
	x[0] = v1[1]*v2[2] - v1[2]*v2[1]
	x[1] = v1[2]*v2[0] - v1[0]*v2[2]
	x[2] = v1[0]*v2[1] - v1[1]*v2[0]
}

func VectorSupCompare(v, w vec3_t, m float32) bool {
	return v[0]-w[0] > -m && v[0]-w[0] < m &&
		v[1]-w[1] > -m && v[1]-w[1] < m &&
		v[2]-w[2] > -m && v[2]-w[2] < m
}

func VectorL2Compare(v, w vec3_t, m float32) bool {
	return (v[0]-w[0])*(v[0]-w[0])+
		(v[1]-w[1])*(v[1]-w[1])+
		(v[2]-w[2])*(v[2]-w[2]) < m*m
}

func VectorCompare(v, w vec3_t) bool {
	return v[0] == w[0] && v[1] == w[1] && v[2] == w[2]
}

func VectorMA(a vec3_t, f float32, b vec3_t, c *vec3_t) {
	c[0] = a[0] + f*b[0]
	c[1] = a[1] + f*b[1]
	c[2] = a[2] + f*b[2]
}

func VectorScale(in vec3_t, scale float32, out *vec3_t) {
	out[0] = in[0] * scale
	out[1] = in[1] * scale
	out[2] = in[2] * scale
}

func anglemod(a float64) float64 {
	return (360.0 / 65536) * float64(int(a*(65536/360.0))&65535)
}

func VectorNormalizeFast(v *vec3_t) {
	f1 := DotProduct(*v, *v)
	if f1 != 0 {
		f2 := 0.5 * f1
		i := int32(math.Float32bits(f1))
		i = 0x5f375a86 - (i >> 1)
		f1 = math.Float32frombits(uint32(i))
		f1 = f1 * (1.5 - f2*f1*f1)
		VectorScale(*v, f1, v)
	}
}

func BOX_ON_PLANE_SIDE(emins, emaxs vec3_t, p *mplane_t) int {
	if p.type_ < 3 {
		if p.dist <= emins[p.type_] {
			return 1
		}
		if p.dist >= emaxs[p.type_] {
			return 2
		}
		return 3
	}
	return BoxOnPlaneSide(emins, emaxs, p)
}

func VectorInterpolate(v1 vec3_t, frac float32, v2 vec3_t, v *vec3_t) {
	v[0] = v1[0] + frac*(v2[0]-v1[0])
	v[1] = v1[1] + frac*(v2[1]-v1[1])
	v[2] = v1[2] + frac*(v2[2]-v1[2])
}

func AngleInterpolate(v1 vec3_t, frac float32, v2 vec3_t, v *vec3_t) {
	var d vec3_t
	VectorSubtract(v2, v1, &d)

	for i := 0; i < 3; i++ {
		if d[i] > 180 {
			d[i] -= 360
		} else if d[i] < -180 {
			d[i] += 360
		}
	}

	v[0] = v1[0] + frac*d[0]
	v[1] = v1[1] + frac*d[1]
	v[2] = v1[2] + frac*d[2]
}

func FloatInterpolate(f1, frac, f2 float32) float32 {
	return f1 + frac*(f2-f1)
}

func PlaneDist(point vec3_t, plane *mplane_t) float32 {
	if plane.type_ < 3 {
		return point[plane.type_]
	}
	return DotProduct(point, plane.normal)
}

func PlaneDiff(point vec3_t, plane *mplane_t) float32 {
	if plane.type_ < 3 {
		return point[plane.type_] - plane.dist
	}
	return DotProduct(point, plane.normal) - plane.dist
}

// mathlib.c

var vec3_origin = vec3_t{0, 0, 0}

func ProjectPointOnPlane(dst *vec3_t, p, normal vec3_t) {
	var n vec3_t

	inv_denom := 1.0 / DotProduct(normal, normal)
	d := DotProduct(normal, p) * inv_denom
	VectorScale(normal, inv_denom, &n)
	VectorMA(p, -d, n, dst)
}

func PerpendicularVector(dst *vec3_t, src vec3_t) {
	if src[0] == 0 {
		*dst = vec3_t{1, 0, 0}
	} else if src[1] == 0 {
		*dst = vec3_t{0, 1, 0}
	} else if src[2] == 0 {
		*dst = vec3_t{0, 0, 1}
	} else {
		*dst = vec3_t{-src[1], src[0], 0}
		VectorNormalizeFast(dst)
	}
}

func VectorVectors(forward vec3_t, right, up *vec3_t) {
	PerpendicularVector(right, forward)
	CrossProduct(*right, forward, up)
}

func RotatePointAroundVector(dst *vec3_t, dir, point vec3_t, degrees float32) {
	var vr, vu vec3_t

	angle := float32(DEG2RAD(float64(degrees)))
	c := float32(math.Cos(float64(angle)))
	s := float32(math.Sin(float64(angle)))
	vf := dir
	VectorVectors(vf, &vr, &vu)

	t0 := vr[0]*c + vu[0]*-s
	t1 := vr[0]*s + vu[0]*c
	dst[0] = (t0*vr[0]+t1*vu[0]+vf[0]*vf[0])*point[0] +
		(t0*vr[1]+t1*vu[1]+vf[0]*vf[1])*point[1] +
		(t0*vr[2]+t1*vu[2]+vf[0]*vf[2])*point[2]

	t0 = vr[1]*c + vu[1]*-s
	t1 = vr[1]*s + vu[1]*c
	dst[1] = (t0*vr[0]+t1*vu[0]+vf[1]*vf[0])*point[0] +
		(t0*vr[1]+t1*vu[1]+vf[1]*vf[1])*point[1] +
		(t0*vr[2]+t1*vu[2]+vf[1]*vf[2])*point[2]

	t0 = vr[2]*c + vu[2]*-s
	t1 = vr[2]*s + vu[2]*c
	dst[2] = (t0*vr[0]+t1*vu[0]+vf[2]*vf[0])*point[0] +
		(t0*vr[1]+t1*vu[1]+vf[2]*vf[1])*point[1] +
		(t0*vr[2]+t1*vu[2]+vf[2]*vf[2])*point[2]
}

func BOPS_Error() {
	Sys_Error("BoxOnPlaneSide:  Bad signbits")
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Returns 1, 2, or 1 + 2
func BoxOnPlaneSide(emins, emaxs vec3_t, p *mplane_t) int {
	n := p.normal
	switch p.signbits {
	default:
		fallthrough
	case 0:
		return b2i(n[0]*emaxs[0]+n[1]*emaxs[1]+n[2]*emaxs[2] >= p.dist) |
			b2i(n[0]*emins[0]+n[1]*emins[1]+n[2]*emins[2] < p.dist)<<1
	case 1:
		return b2i(n[0]*emins[0]+n[1]*emaxs[1]+n[2]*emaxs[2] >= p.dist) |
			b2i(n[0]*emaxs[0]+n[1]*emins[1]+n[2]*emins[2] < p.dist)<<1
	case 2:
		return b2i(n[0]*emaxs[0]+n[1]*emins[1]+n[2]*emaxs[2] >= p.dist) |
			b2i(n[0]*emins[0]+n[1]*emaxs[1]+n[2]*emins[2] < p.dist)<<1
	case 3:
		return b2i(n[0]*emins[0]+n[1]*emins[1]+n[2]*emaxs[2] >= p.dist) |
			b2i(n[0]*emaxs[0]+n[1]*emaxs[1]+n[2]*emins[2] < p.dist)<<1
	case 4:
		return b2i(n[0]*emaxs[0]+n[1]*emaxs[1]+n[2]*emins[2] >= p.dist) |
			b2i(n[0]*emins[0]+n[1]*emins[1]+n[2]*emaxs[2] < p.dist)<<1
	case 5:
		return b2i(n[0]*emins[0]+n[1]*emaxs[1]+n[2]*emins[2] >= p.dist) |
			b2i(n[0]*emaxs[0]+n[1]*emins[1]+n[2]*emaxs[2] < p.dist)<<1
	case 6:
		return b2i(n[0]*emaxs[0]+n[1]*emins[1]+n[2]*emins[2] >= p.dist) |
			b2i(n[0]*emins[0]+n[1]*emaxs[1]+n[2]*emaxs[2] < p.dist)<<1
	case 7:
		return b2i(n[0]*emins[0]+n[1]*emins[1]+n[2]*emins[2] >= p.dist) |
			b2i(n[0]*emaxs[0]+n[1]*emaxs[1]+n[2]*emaxs[2] < p.dist)<<1
	}
}

// Any of forward, right and up may be nil.
func AngleVectors(angles vec3_t, forward, right, up *vec3_t) {
	var angle, sr, sp, sy, cr, cp, cy, temp float32

	if angles[YAW] != 0 {
		angle = float32(DEG2RAD(float64(angles[YAW])))
		sy = float32(math.Sin(float64(angle)))
		cy = float32(math.Cos(float64(angle)))
	} else {
		sy = 0
		cy = 1
	}

	if angles[PITCH] != 0 {
		angle = float32(DEG2RAD(float64(angles[PITCH])))
		sp = float32(math.Sin(float64(angle)))
		cp = float32(math.Cos(float64(angle)))
	} else {
		sp = 0
		cp = 1
	}

	if forward != nil {
		forward[0] = cp * cy
		forward[1] = cp * sy
		forward[2] = -sp
	}

	if right != nil || up != nil {
		if angles[ROLL] != 0 {
			angle = float32(DEG2RAD(float64(angles[ROLL])))
			sr = float32(math.Sin(float64(angle)))
			cr = float32(math.Cos(float64(angle)))

			if right != nil {
				temp = sr * sp
				right[0] = -1*temp*cy + cr*sy
				right[1] = -1*temp*sy - cr*cy
				right[2] = -1 * sr * cp
			}

			if up != nil {
				temp = cr * sp
				up[0] = (temp*cy + sr*sy)
				up[1] = (temp*sy - sr*cy)
				up[2] = cr * cp
			}
		} else {
			if right != nil {
				right[0] = sy
				right[1] = -cy
				right[2] = 0
			}

			if up != nil {
				up[0] = sp * cy
				up[1] = sp * sy
				up[2] = cp
			}
		}
	}
}

func VectorLength(v vec3_t) vec_t {
	length := v[0]*v[0] + v[1]*v[1] + v[2]*v[2]
	return float32(math.Sqrt(float64(length)))
}

func VectorNormalize(v *vec3_t) float32 {
	length := v[0]*v[0] + v[1]*v[1] + v[2]*v[2]
	length = float32(math.Sqrt(float64(length)))

	if length != 0 {
		VectorScale(*v, 1/length, v)
	}

	return length
}

func R_ConcatRotations(in1, in2 [3][3]float32, out *[3][3]float32) {
	out[0][0] = in1[0][0]*in2[0][0] + in1[0][1]*in2[1][0] + in1[0][2]*in2[2][0]
	out[0][1] = in1[0][0]*in2[0][1] + in1[0][1]*in2[1][1] + in1[0][2]*in2[2][1]
	out[0][2] = in1[0][0]*in2[0][2] + in1[0][1]*in2[1][2] + in1[0][2]*in2[2][2]
	out[1][0] = in1[1][0]*in2[0][0] + in1[1][1]*in2[1][0] + in1[1][2]*in2[2][0]
	out[1][1] = in1[1][0]*in2[0][1] + in1[1][1]*in2[1][1] + in1[1][2]*in2[2][1]
	out[1][2] = in1[1][0]*in2[0][2] + in1[1][1]*in2[1][2] + in1[1][2]*in2[2][2]
	out[2][0] = in1[2][0]*in2[0][0] + in1[2][1]*in2[1][0] + in1[2][2]*in2[2][0]
	out[2][1] = in1[2][0]*in2[0][1] + in1[2][1]*in2[1][1] + in1[2][2]*in2[2][1]
	out[2][2] = in1[2][0]*in2[0][2] + in1[2][1]*in2[1][2] + in1[2][2]*in2[2][2]
}

func R_ConcatTransforms(in1, in2 [3][4]float32, out *[3][4]float32) {
	out[0][0] = in1[0][0]*in2[0][0] + in1[0][1]*in2[1][0] + in1[0][2]*in2[2][0]
	out[0][1] = in1[0][0]*in2[0][1] + in1[0][1]*in2[1][1] + in1[0][2]*in2[2][1]
	out[0][2] = in1[0][0]*in2[0][2] + in1[0][1]*in2[1][2] + in1[0][2]*in2[2][2]
	out[0][3] = in1[0][0]*in2[0][3] + in1[0][1]*in2[1][3] + in1[0][2]*in2[2][3] + in1[0][3]
	out[1][0] = in1[1][0]*in2[0][0] + in1[1][1]*in2[1][0] + in1[1][2]*in2[2][0]
	out[1][1] = in1[1][0]*in2[0][1] + in1[1][1]*in2[1][1] + in1[1][2]*in2[2][1]
	out[1][2] = in1[1][0]*in2[0][2] + in1[1][1]*in2[1][2] + in1[1][2]*in2[2][2]
	out[1][3] = in1[1][0]*in2[0][3] + in1[1][1]*in2[1][3] + in1[1][2]*in2[2][3] + in1[1][3]
	out[2][0] = in1[2][0]*in2[0][0] + in1[2][1]*in2[1][0] + in1[2][2]*in2[2][0]
	out[2][1] = in1[2][0]*in2[0][1] + in1[2][1]*in2[1][1] + in1[2][2]*in2[2][1]
	out[2][2] = in1[2][0]*in2[0][2] + in1[2][1]*in2[1][2] + in1[2][2]*in2[2][2]
	out[2][3] = in1[2][0]*in2[0][3] + in1[2][1]*in2[1][3] + in1[2][2]*in2[2][3] + in1[2][3]
}

// Returns mathematically correct (floor-based) quotient and remainder for
// numer and denom, both of which should contain no fractional part. The
// quotient must fit in 32 bits.
func FloorDivMod(numer, denom float64, quotient, rem *int) {
	var q, r int
	var x float64

	if denom <= 0.0 {
		Sys_Error("FloorDivMod: bad denominator %v", denom)
	}

	if numer >= 0.0 {
		x = math.Floor(numer / denom)
		q = int(x)
		r = int(math.Floor(numer - (x * denom)))
	} else {
		// perform operations with positive values, and fix mod to make floor-based
		x = math.Floor(-numer / denom)
		q = -int(x)
		r = int(math.Floor(-numer - (x * denom)))
		if r != 0 {
			q--
			r = int(denom) - r
		}
	}

	*quotient = q
	*rem = r
}

func GreatestCommonDivisor(i1, i2 int) int {
	if i1 > i2 {
		if i2 == 0 {
			return i1
		}
		return GreatestCommonDivisor(i2, i1%i2)
	}
	if i1 == 0 {
		return i2
	}
	return GreatestCommonDivisor(i1, i2%i1)
}

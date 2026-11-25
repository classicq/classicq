package engine

// mdfour.c: MD4, used for the map checksums sent to the server.

type mdfour struct {
	A, B, C, D uint32
	totalN     uint32
}

func mdfour_F(X, Y, Z uint32) uint32 { return (X & Y) | (^X & Z) }
func mdfour_G(X, Y, Z uint32) uint32 { return (X & Y) | (X & Z) | (Y & Z) }
func mdfour_H(X, Y, Z uint32) uint32 { return X ^ Y ^ Z }
func lshift(x uint32, s uint) uint32 { return (x << s) | (x >> (32 - s)) }

// this applies md4 to 64 byte chunks
func mdfour64(m *mdfour, X *[16]uint32) {
	A, B, C, D := m.A, m.B, m.C, m.D
	AA, BB, CC, DD := A, B, C, D

	r1 := func(a, b, c, d uint32, k int, s uint) uint32 { return lshift(a+mdfour_F(b, c, d)+X[k], s) }
	r2 := func(a, b, c, d uint32, k int, s uint) uint32 { return lshift(a+mdfour_G(b, c, d)+X[k]+0x5A827999, s) }
	r3 := func(a, b, c, d uint32, k int, s uint) uint32 { return lshift(a+mdfour_H(b, c, d)+X[k]+0x6ED9EBA1, s) }

	A = r1(A, B, C, D, 0, 3)
	D = r1(D, A, B, C, 1, 7)
	C = r1(C, D, A, B, 2, 11)
	B = r1(B, C, D, A, 3, 19)
	A = r1(A, B, C, D, 4, 3)
	D = r1(D, A, B, C, 5, 7)
	C = r1(C, D, A, B, 6, 11)
	B = r1(B, C, D, A, 7, 19)
	A = r1(A, B, C, D, 8, 3)
	D = r1(D, A, B, C, 9, 7)
	C = r1(C, D, A, B, 10, 11)
	B = r1(B, C, D, A, 11, 19)
	A = r1(A, B, C, D, 12, 3)
	D = r1(D, A, B, C, 13, 7)
	C = r1(C, D, A, B, 14, 11)
	B = r1(B, C, D, A, 15, 19)

	A = r2(A, B, C, D, 0, 3)
	D = r2(D, A, B, C, 4, 5)
	C = r2(C, D, A, B, 8, 9)
	B = r2(B, C, D, A, 12, 13)
	A = r2(A, B, C, D, 1, 3)
	D = r2(D, A, B, C, 5, 5)
	C = r2(C, D, A, B, 9, 9)
	B = r2(B, C, D, A, 13, 13)
	A = r2(A, B, C, D, 2, 3)
	D = r2(D, A, B, C, 6, 5)
	C = r2(C, D, A, B, 10, 9)
	B = r2(B, C, D, A, 14, 13)
	A = r2(A, B, C, D, 3, 3)
	D = r2(D, A, B, C, 7, 5)
	C = r2(C, D, A, B, 11, 9)
	B = r2(B, C, D, A, 15, 13)

	A = r3(A, B, C, D, 0, 3)
	D = r3(D, A, B, C, 8, 9)
	C = r3(C, D, A, B, 4, 11)
	B = r3(B, C, D, A, 12, 15)
	A = r3(A, B, C, D, 2, 3)
	D = r3(D, A, B, C, 10, 9)
	C = r3(C, D, A, B, 6, 11)
	B = r3(B, C, D, A, 14, 15)
	A = r3(A, B, C, D, 1, 3)
	D = r3(D, A, B, C, 9, 9)
	C = r3(C, D, A, B, 5, 11)
	B = r3(B, C, D, A, 13, 15)
	A = r3(A, B, C, D, 3, 3)
	D = r3(D, A, B, C, 11, 9)
	C = r3(C, D, A, B, 7, 11)
	B = r3(B, C, D, A, 15, 15)

	A += AA
	B += BB
	C += CC
	D += DD

	m.A, m.B, m.C, m.D = A, B, C, D
}

func copy64(M *[16]uint32, in []byte) {
	for i := 0; i < 16; i++ {
		M[i] = uint32(in[i*4+3])<<24 | uint32(in[i*4+2])<<16 | uint32(in[i*4+1])<<8 | uint32(in[i*4+0])
	}
}

func copy4(out []byte, x uint32) {
	out[0] = byte(x)
	out[1] = byte(x >> 8)
	out[2] = byte(x >> 16)
	out[3] = byte(x >> 24)
}

func mdfour_begin(md *mdfour) {
	md.A = 0x67452301
	md.B = 0xefcdab89
	md.C = 0x98badcfe
	md.D = 0x10325476
	md.totalN = 0
}

func mdfour_tail(m *mdfour, in []byte, n int) {
	var buf [128]byte
	var M [16]uint32

	m.totalN += uint32(n)
	b := m.totalN * 8

	copy(buf[:], in[:n])
	buf[n] = 0x80

	if n <= 55 {
		copy4(buf[56:], b)
		copy64(&M, buf[:])
		mdfour64(m, &M)
	} else {
		copy4(buf[120:], b)
		copy64(&M, buf[:])
		mdfour64(m, &M)
		copy64(&M, buf[64:])
		mdfour64(m, &M)
	}
}

func mdfour_update(md *mdfour, in []byte, n int) {
	var M [16]uint32

	for n >= 64 {
		copy64(&M, in)
		mdfour64(md, &M)
		in = in[64:]
		n -= 64
		md.totalN += 64
	}
	mdfour_tail(md, in, n)
}

func mdfour_result(md *mdfour, out []byte) {
	copy4(out, md.A)
	copy4(out[4:], md.B)
	copy4(out[8:], md.C)
	copy4(out[12:], md.D)
}

func mdfour_sum(out []byte, in []byte, n int) {
	var md mdfour

	mdfour_begin(&md)
	mdfour_update(&md, in, n)
	mdfour_result(&md, out)
}

func Com_BlockChecksum(buffer []byte, length int) uint32 {
	var digest [16]byte

	mdfour_sum(digest[:], buffer, length)
	d := func(i int) uint32 {
		return uint32(digest[i]) | uint32(digest[i+1])<<8 | uint32(digest[i+2])<<16 | uint32(digest[i+3])<<24
	}
	return d(0) ^ d(4) ^ d(8) ^ d(12)
}

func Com_BlockFullChecksum(buffer []byte, length int, outbuf []byte) {
	mdfour_sum(outbuf, buffer, length)
}

package engine

import (
	"cmp"
	"fmt"
	"strconv"
	"unsafe"
)

// Helpers that reproduce C string semantics where they affect results.

// at reads s[i], or 0 past the end, like a NUL-terminated C string.
func at(s string, i int) byte {
	if i < len(s) {
		return s[i]
	}
	return 0
}

// cstr reads b as a NUL-terminated C string.
func cstr(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

// strlcpy truncates src to fit a buffer of size bytes including the NUL.
func strlcpy(src string, size int) string {
	if size <= 0 {
		return ""
	}
	if len(src) >= size {
		return src[:size-1]
	}
	return src
}

// strlcat appends src to dst within a buffer of size bytes including the NUL.
func strlcat(dst, src string, size int) string {
	if len(dst) >= size {
		return dst
	}
	room := size - 1 - len(dst)
	if len(src) > room {
		src = src[:room]
	}
	return dst + src
}

func isdigit(c byte) bool {
	return c >= '0' && c <= '9'
}

// atoi parses the longest decimal prefix like C's atoi.
func atoi(s string) int {
	i := 0
	for i < len(s) && (s[i] == ' ' || (s[i] >= '\t' && s[i] <= '\r')) {
		i++
	}
	neg := false
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		neg = s[i] == '-'
		i++
	}
	var v int32
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		v = v*10 + int32(s[i]-'0')
		i++
	}
	if neg {
		v = -v
	}
	return int(v)
}

// atof parses the longest decimal prefix like C's atof; no inf, nan or hex.
func atof(s string) float64 {
	i := 0
	for i < len(s) && (s[i] == ' ' || (s[i] >= '\t' && s[i] <= '\r')) {
		i++
	}
	start := i
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	digits := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
		digits++
	}
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
			digits++
		}
	}
	if digits == 0 {
		return 0
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		j := i + 1
		if j < len(s) && (s[j] == '+' || s[j] == '-') {
			j++
		}
		if j < len(s) && s[j] >= '0' && s[j] <= '9' {
			for j < len(s) && s[j] >= '0' && s[j] <= '9' {
				j++
			}
			i = j
		}
	}
	f, _ := strconv.ParseFloat(s[start:i], 64)
	return f
}

// bound is the bound() macro from common.h, quirks included.
func bound[T cmp.Ordered](a, b, c T) T {
	if a >= c {
		return a
	}
	if b < a {
		return a
	}
	if b > c {
		return c
	}
	return b
}

func isalnum(c byte) bool {
	return isdigit(c) || (c|32 >= 'a' && c|32 <= 'z')
}

func isspace(c byte) bool {
	return c == ' ' || (c >= '\t' && c <= '\r')
}

func tolower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + 'a' - 'A'
	}
	return c
}

// snprintf into a C buffer of size bytes: the result is cut to size-1
func snprintf(size int, format string, a ...any) string {
	s := fmt.Sprintf(format, a...)
	if len(s) > size-1 {
		s = s[:size-1]
	}
	return s
}

// scratchf is snprintf into one reused buffer: the result lives until the
// next call, so use it right away. scratchfb hands out the bytes.
var scratch_buf []byte

func scratchfb(size int, format string, a ...any) []byte {
	scratch_buf = fmt.Appendf(scratch_buf[:0], format, a...)
	if len(scratch_buf) > size-1 {
		scratch_buf = scratch_buf[:size-1]
	}
	return scratch_buf
}

func scratchf(size int, format string, a ...any) string {
	b := scratchfb(size, format, a...)
	return unsafe.String(unsafe.SliceData(b), len(b))
}

// strncmp(a, b, n) == 0
func strneq(a, b string, n int) bool {
	if len(a) > n {
		a = a[:n]
	}
	if len(b) > n {
		b = b[:n]
	}
	return a == b
}

// Q_strncpyz(dst, s + p, n + 1): up to n bytes of s from offset p
func cstrn(s string, p, n int) string {
	if p >= len(s) {
		return ""
	}
	return strlcpy(s[p:], n+1)
}

// strtoul: leading whitespace, optional sign, base 0 picks 8, 10 or 16 from
// the prefix, stops at the first non-digit
func strtoul(s string, base int) uint64 {
	i := 0
	for i < len(s) && isspace(s[i]) {
		i++
	}
	neg := false
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		neg = s[i] == '-'
		i++
	}
	if (base == 0 || base == 16) && i+1 < len(s) && s[i] == '0' && (s[i+1] == 'x' || s[i+1] == 'X') {
		base = 16
		i += 2
	} else if base == 0 && i < len(s) && s[i] == '0' {
		base = 8
	} else if base == 0 {
		base = 10
	}
	v := uint64(0)
	for ; i < len(s); i++ {
		c := int(s[i])
		var d int
		switch {
		case c >= '0' && c <= '9':
			d = c - '0'
		case c >= 'a' && c <= 'z':
			d = c - 'a' + 10
		case c >= 'A' && c <= 'Z':
			d = c - 'A' + 10
		default:
			d = base
		}
		if d >= base {
			break
		}
		v = v*uint64(base) + uint64(d)
	}
	if neg {
		v = -v
	}
	return v
}

// C strings living in a byte buffer: a string runs from an index to the
// next NUL or the end of the buffer.
func cbufEnd(b []byte, i int) int {
	for i < len(b) && b[i] != 0 {
		i++
	}
	return i
}

func cbufStr(b []byte, i int) string {
	if i >= len(b) {
		return ""
	}
	return string(b[i:cbufEnd(b, i)])
}

// strchr, -1 when c is not before the terminator
func cbufChr(b []byte, i int, c byte) int {
	for ; i < len(b) && b[i] != 0; i++ {
		if b[i] == c {
			return i
		}
	}
	return -1
}

func cbufAt(b []byte, i int) byte {
	if i < 0 || i >= len(b) {
		return 0
	}
	return b[i]
}

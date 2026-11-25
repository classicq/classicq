package engine

// char is signed on x86, so bytes with the high bit set
// compare as negative numbers.
func mytolower(c byte) int8 {
	if c >= 'A' && c <= 'Z' {
		return int8(c + ('a' - 'A'))
	}
	return int8(c)
}

func Q_strcasecmp(s1, s2 string) int {
	var c1, c2 int8
	i := 0
	for {
		c1 = mytolower(at(s1, i))
		c2 = mytolower(at(s2, i))
		i++
		if !(c1 != 0 && c1 == c2) {
			break
		}
	}
	return int(c1) - int(c2)
}

func Q_strncasecmp(s1, s2 string, n uint) int {
	var c1, c2 int8 = 1, 1
	i := 0
	for n > 0 && c1 != 0 && c1 == c2 {
		n--
		c1 = mytolower(at(s1, i))
		c2 = mytolower(at(s2, i))
		i++
	}
	return int(c1) - int(c2)
}

package engine

// The UCRT rand(), whose exact sequence matters.

const RAND_MAX = 0x7fff

var rand_seed uint32 = 1

// UCRT generator: seed * 214013 + 2531011, upper bits out.
func srand(seed uint32) {
	rand_seed = seed
}

func rand() int {
	rand_seed = rand_seed*214013 + 2531011
	return int((rand_seed >> 16) & 0x7fff)
}

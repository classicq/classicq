package engine

var readablechars = [256]byte{
	'.', '_', '_', '_', '_', '.', '_', '_', '_', '_', 10, '_', 10, '>', '.', '.',
	'[', ']', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '.', '_', '_', '_',
}

func ReadableChars_Init() {
	for i := 32; i < 127; i++ {
		readablechars[i] = byte(i)
		readablechars[128+i] = byte(i)
	}
	readablechars[127] = '_'
	readablechars[128+127] = '_'
	for i := 0; i < 32; i++ {
		readablechars[128+i] = readablechars[i]
	}
	readablechars[128] = '_'
	readablechars[10+128] = '_'
	readablechars[12+128] = '_'
}

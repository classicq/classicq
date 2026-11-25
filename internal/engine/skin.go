package engine

var baseskin = cvar_t{name: "baseskin", string: "base"}
var noskins = cvar_t{name: "noskins", string: "0"}

var defaultskin []byte

type SkinSourceType int

const (
	SKINSOURCE_SOLIDCOLOUR SkinSourceType = iota
	SKINSOURCE_TEXTURE_PALETTED
	SKINSOURCE_TEXTURE_PALETTED_TRANSLATED
	SKINSOURCE_TEXTURE_TRUECOLOUR
)

// the C union holds either the colour or the texture; type_ says which
type SkinSource struct {
	next *SkinSource

	type_ SkinSourceType

	translations *SkinTranslation

	skinname string

	colours [3]float32

	width  uint
	height uint
	data   []byte
}

type SkinTranslation struct {
	next *SkinTranslation

	topcolour    uint
	bottomcolour uint

	skinimp *SkinImp
}

var skinsources *SkinSource

func Skin_SetupTexture(source *SkinSource) {
	width := source.width
	height := source.height

	if width > 296 {
		width = 296
	}

	if height > 194 {
		height = 194
	}

	source.type_ = SKINSOURCE_TEXTURE_PALETTED

	p := 0
	for y := uint(0); y < height; y++ {
		for x := uint(0); x < width; x++ {
			c := source.data[p+int(x)]
			if (c >= 16 && c < 32) || (c >= 96 && c < 112) {
				source.type_ = SKINSOURCE_TEXTURE_PALETTED_TRANSLATED
				break
			}
		}

		if source.type_ == SKINSOURCE_TEXTURE_PALETTED_TRANSLATED {
			break
		}

		p += int(source.width)
	}
}

func Skin_GetSource(skinname string) *SkinSource {
	if skinname == "" || noskins.value == 1 {
		skinname = baseskin.string
	}

	source := skinsources
	for source != nil {
		if source.skinname == skinname {
			return source
		}

		source = source.next
	}

	source = &SkinSource{}
	source.skinname = skinname

	ok := true

	if ParseColourDescription(skinname, &source.colours) {
		source.type_ = SKINSOURCE_SOLIDCOLOUR
	} else if source.data = nil; skinname != "" && func() bool {
		source.data = Image_LoadPNG(nil, va("skins/%s.png", skinname), 0, 0, &source.width, &source.height)
		return source.data != nil
	}() {
		source.type_ = SKINSOURCE_TEXTURE_TRUECOLOUR
	} else if skinname != "" && func() bool {
		source.data = Image_LoadPCX(nil, va("skins/%s.pcx", skinname), 0, 0, &source.width, &source.height)
		return source.data != nil
	}() {
		Skin_SetupTexture(source)
	} else if source.data = Image_LoadPCX(nil, "skins/base.pcx", 0, 0, &source.width, &source.height); source.data != nil {
		Skin_SetupTexture(source)
	} else if skinname == "base" {
		if defaultskin != nil {
			source.data = make([]byte, 296*194)
			copy(source.data, defaultskin)
			source.width = 296
			source.height = 194
			Skin_SetupTexture(source)
		} else {
			source.type_ = SKINSOURCE_SOLIDCOLOUR
			source.colours[0] = 1
			source.colours[1] = 0
			source.colours[2] = 1
		}
	} else {
		ok = false
	}

	if ok {
		source.next = skinsources
		skinsources = source

		return source
	}

	if skinname != "base" {
		return Skin_GetSource("base")
	}

	return nil
}

func Skin_DeleteSource(source *SkinSource) {
	next := source.translations
	for next != nil {
		translation := next
		next = translation.next

		SkinImp_Destroy(translation.skinimp)
	}

	if source == skinsources {
		skinsources = source.next
	} else {
		s := skinsources
		for s != nil {
			if s.next == source {
				s.next = source.next
				break
			}

			s = s.next
		}
	}
}

func Skin_GetTranslation(skinname string, topcolour, bottomcolour uint) *SkinImp {
	var skinimp *SkinImp

	source := Skin_GetSource(skinname)
	if source == nil {
		return nil
	}

	if topcolour > 13 {
		topcolour = 13
	}

	if bottomcolour > 13 {
		bottomcolour = 13
	}

	translation := source.translations
	for translation != nil {
		if (translation.topcolour == topcolour && translation.bottomcolour == bottomcolour) ||
			source.type_ == SKINSOURCE_SOLIDCOLOUR ||
			source.type_ == SKINSOURCE_TEXTURE_PALETTED {
			return translation.skinimp
		}

		translation = translation.next
	}

	translation = &SkinTranslation{}

	if source.type_ == SKINSOURCE_SOLIDCOLOUR {
		skinimp = SkinImp_CreateSolidColour(source.colours[:])
	} else if source.type_ == SKINSOURCE_TEXTURE_PALETTED {
		skinimp = SkinImp_CreateTexturePaletted(source.data, 296, 194, source.width)
	} else if source.type_ == SKINSOURCE_TEXTURE_PALETTED_TRANSLATED {
		var table [256]byte

		translated := make([]byte, 296*194)

		top := topcolour * 16
		bottom := bottomcolour * 16

		for i := uint(0); i < 16; i++ {
			table[i] = byte(i)
		}

		if top < 128 {
			for i := uint(0); i < 16; i++ {
				table[i+16] = byte(top + i)
			}
		} else {
			for i := uint(0); i < 16; i++ {
				table[i+16] = byte(top + 15 - i)
			}
		}

		for i := 32; i < 96; i++ {
			table[i] = byte(i)
		}

		if bottom < 128 {
			for i := uint(0); i < 16; i++ {
				table[i+96] = byte(bottom + i)
			}
		} else {
			for i := uint(0); i < 16; i++ {
				table[i+96] = byte(bottom + 15 - i)
			}
		}

		for i := 112; i < 256; i++ {
			table[i] = byte(i)
		}

		src := 0
		dst := 0

		width := source.width
		if width > 296 {
			width = 296
		}

		height := source.height
		if height > 194 {
			height = 194
		}

		y := uint(0)
		for y = 0; y < height; y++ {
			x := uint(0)
			for x = 0; x < width; x++ {
				translated[dst+int(x)] = table[source.data[src+int(x)]]
			}

			for ; x < 296; x++ {
				translated[dst+int(x)] = 0
			}

			src += int(source.width)
			dst += 296
		}

		if y != 194 {
			for i := dst; i < dst+int(194-y)*296; i++ {
				translated[i] = 0
			}
		}

		skinimp = SkinImp_CreateTexturePaletted(translated, 296, 194, 296)
	} else if source.type_ == SKINSOURCE_TEXTURE_TRUECOLOUR {
		skinimp = SkinImp_CreateTextureTruecolour(source.data, source.width, source.height)
	}

	if skinimp != nil {
		translation.topcolour = topcolour
		translation.bottomcolour = bottomcolour
		translation.skinimp = skinimp

		translation.next = source.translations
		source.translations = translation

		return skinimp
	}

	return nil
}

func Skin_SetDefault(data []byte, width, height uint) {
	if width < 296 || height < 194 {
		return
	}

	if defaultskin == nil {
		defaultskin = make([]byte, 296*194)
	}

	src := 0
	dst := 0

	for y := 0; y < 194; y++ {
		copy(defaultskin[dst:dst+296], data[src:src+296])

		src += int(width)
		dst += 296
	}
}

func Skin_FreeAll() {
	for skinsources != nil {
		Skin_DeleteSource(skinsources)
	}
}

func Skin_CvarInit() {
	Cvar_SetCurrentGroup(CVAR_GROUP_SKIN)
	Cvar_Register(&baseskin)
	Cvar_Register(&noskins)
	Cvar_ResetCurrentGroup()
}

func Skin_Init() {
}

func Skin_Shutdown() {
	Skin_FreeAll()

	defaultskin = nil
}

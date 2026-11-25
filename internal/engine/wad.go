package engine

import (
	"io"
	"os"
	"sort"
)

// wad.h
const (
	CMP_NONE = 0
	CMP_LZSS = 1

	TYP_NONE    = 0
	TYP_LABEL   = 1
	TYP_LUMPY   = 64 // 64 + grab command number
	TYP_PALETTE = 64
	TYP_QTEX    = 65
	TYP_QPIC    = 66
	TYP_SOUND   = 67
	TYP_MIPTEX  = 68

	sizeof_wadinfo_t  = 12
	sizeof_lumpinfo_t = 32
)

type lumpinfo_t struct {
	filepos     int32
	disksize    int32
	size        int32 // uncompressed
	type_       byte
	compression byte
	pad1, pad2  byte
	name        string // 16 bytes in the file, must be null terminated
}

// wad.c

var wad_numlumps int
var wad_lumps []lumpinfo_t
var wad_base []byte

// Lowercases name and pads with spaces and a terminating 0 to the length
// of lumpinfo_t->name. Used so lumpname lookups can proceed rapidly by
// comparing 4 chars at a time.
func W_CleanupName(in string) string {
	var out []byte

	for i := 0; i < 16; i++ {
		c := at(in, i)
		if c == 0 {
			break
		}
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		out = append(out, c)
	}
	return string(out)
}

func readLumpinfo(b []byte) lumpinfo_t {
	return lumpinfo_t{
		filepos:     leLong(b),
		disksize:    leLong(b[4:]),
		size:        leLong(b[8:]),
		type_:       b[12],
		compression: b[13],
		pad1:        b[14],
		pad2:        b[15],
		name:        string(b[16:32]),
	}
}

func W_LoadWadFile(filename string) {
	wad_base = FS_LoadMallocFile(filename)
	if wad_base == nil {
		Sys_Error("W_LoadWadFile: couldn't load %s", filename)
	}

	if com_filesize > 512*1024*1024 {
		Sys_Error("Wad file %s is too large (%d bytes)\n", filename, com_filesize)
	}

	if len(wad_base) < sizeof_wadinfo_t || string(wad_base[:4]) != "WAD2" {
		Sys_Error("Wad file %s doesn't have WAD2 id\n", filename)
	}

	wad_numlumps = int(leLong(wad_base[4:]))
	infotableofs := int(leLong(wad_base[8:]))

	if wad_numlumps >= 65536 {
		Sys_Error("Wad file %s contains too many lumps (%d)\n", filename, wad_numlumps)
	}

	if infotableofs >= 512*1024*1024 {
		Sys_Error("Wad file %s has invalid info table offset (%d)\n", filename, infotableofs)
	}

	if infotableofs >= com_filesize || infotableofs+wad_numlumps*sizeof_lumpinfo_t > com_filesize {
		Sys_Error("Was file %s corrupt. Info table out of range.\n", filename)
	}

	wad_lumps = make([]lumpinfo_t, wad_numlumps)
	for i := 0; i < wad_numlumps; i++ {
		wad_lumps[i] = readLumpinfo(wad_base[infotableofs+i*sizeof_lumpinfo_t:])
		wad_lumps[i].name = W_CleanupName(wad_lumps[i].name)
	}

	sort.Slice(wad_lumps, func(a, b int) bool { return wad_lumps[a].filepos < wad_lumps[b].filepos })

	for i := range wad_lumps {
		lump_p := &wad_lumps[i]

		// W_CleanupName stops at the first NUL, so a full 16 byte name is
		// the unterminated case
		if len(lump_p.name) == 16 {
			Sys_Error("Was file %s corrupt. Unterminated lump name.\n", filename)
		}

		if lump_p.size <= 0 || lump_p.size >= 512*1024*1024 {
			Sys_Error("Wad file %s contains lump with invalid size (%d)\n", filename, lump_p.size)
		}

		// Now find the largest possible size of the lump...
		var lump_end int
		if i+1 != wad_numlumps {
			lump_end = int(wad_lumps[i+1].filepos)
		} else {
			lump_end = com_filesize
		}

		// If the lump info table is somewhere inside the computed max size, crop it to the beginning of the lump info table
		if infotableofs >= int(lump_p.filepos) && infotableofs <= lump_end {
			lump_end = infotableofs
		}

		if lump_end-int(lump_p.filepos) < int(lump_p.size) {
			lump_p.size = int32(lump_end - int(lump_p.filepos))
		}

		// Now verify that the lump is inside the file and not inside the lump info table
		if lump_p.filepos < 0 || int(lump_p.filepos) >= com_filesize {
			Sys_Error("Wad file %s contains lump with invalid offset (%d)\n", filename, lump_p.filepos)
		}

		if int(lump_p.filepos+lump_p.size) >= com_filesize {
			Sys_Error("Wad file %s contains out of bounds lump\n", filename)
		}

		tableend := infotableofs + wad_numlumps*sizeof_lumpinfo_t
		if !((int(lump_p.filepos) <= infotableofs && int(lump_p.filepos+lump_p.size) <= infotableofs) ||
			(int(lump_p.filepos) >= tableend && int(lump_p.filepos+lump_p.size) >= tableend)) {
			Sys_Error("Wad file %s contains lump and info table overlap\n", filename)
		}

		// SwapPic on TYP_QPIC lumps is a no-op on little endian
	}
}

func W_UnloadWadFile() {
	wad_base = nil
	wad_lumps = nil
	wad_numlumps = 0
}

func W_GetLumpinfo(name string) *lumpinfo_t {
	clean := W_CleanupName(name)

	for i := range wad_lumps {
		if clean == wad_lumps[i].name {
			return &wad_lumps[i]
		}
	}

	Sys_Error("W_GetLumpinfo: %s not found", name)
	return nil
}

// Returns the lump data, which runs on to the end of the wad.
func W_GetLumpName(name string) []byte {
	lump := W_GetLumpinfo(name)

	if name == "conchars" {
		if CRC_Block(wad_base[lump.filepos:lump.filepos+lump.size]) == 798 {
			W_InsertOcranaLeds(wad_base[lump.filepos:])
		}
	}

	return wad_base[lump.filepos:]
}

var ocrana_leds = [4][8][8]byte{
	/* green */
	{
		{0x00, 0x38, 0x3b, 0x3b, 0x3b, 0x3b, 0x35, 0x00},
		{0x38, 0x3b, 0x3d, 0x3f, 0x3f, 0x3d, 0x38, 0x35},
		{0x3b, 0x3d, 0xfe, 0x3f, 0x3f, 0x3f, 0x3b, 0x35},
		{0x3b, 0x3f, 0x3f, 0x3f, 0x3f, 0x3f, 0x3b, 0x35},
		{0x3b, 0x3f, 0x3f, 0x3f, 0x3f, 0x3d, 0x3b, 0x35},
		{0x3b, 0x3d, 0x3f, 0x3f, 0x3d, 0x3b, 0x38, 0x35},
		{0x35, 0x38, 0x3b, 0x3b, 0x3b, 0x38, 0x35, 0x35},
		{0x00, 0x35, 0x35, 0x35, 0x35, 0x35, 0x35, 0x00},
	},
	/* red */
	{
		{0x00, 0xf8, 0xf9, 0xf9, 0xf9, 0xf9, 0x4c, 0x00},
		{0xf8, 0xf9, 0xfa, 0xfb, 0xfb, 0xfa, 0xf8, 0x4c},
		{0xf9, 0xfa, 0xfe, 0xfb, 0xfb, 0xfb, 0xf9, 0x4c},
		{0xf9, 0xfb, 0xfb, 0xfb, 0xfb, 0xfb, 0xf9, 0x4c},
		{0xf9, 0xfb, 0xfb, 0xfb, 0xfb, 0xfa, 0xf9, 0x4c},
		{0xf9, 0xfa, 0xfb, 0xfb, 0xfa, 0xf9, 0xf8, 0x4c},
		{0x4c, 0xf8, 0xf9, 0xf9, 0xf9, 0xf8, 0x4c, 0x4c},
		{0x00, 0x4c, 0x4c, 0x4c, 0x4c, 0x4c, 0x4c, 0x00},
	},
	/* yellow */
	{
		{0x00, 0xc8, 0xc5, 0xc5, 0xc5, 0xc5, 0xcb, 0x00},
		{0xc8, 0xc5, 0xc2, 0x6f, 0x6f, 0xc2, 0xc8, 0xcb},
		{0xc5, 0xc2, 0xfe, 0x6f, 0x6f, 0x6f, 0xc5, 0xcb},
		{0xc5, 0x6f, 0x6f, 0x6f, 0x6f, 0x6f, 0xc5, 0xcb},
		{0xc5, 0x6f, 0x6f, 0x6f, 0x6f, 0xc2, 0xc5, 0xcb},
		{0xc5, 0xc2, 0x6f, 0x6f, 0xc2, 0xc5, 0xc8, 0xcb},
		{0xcb, 0xc8, 0xc5, 0xc5, 0xc5, 0xc8, 0xcb, 0xcb},
		{0x00, 0xcb, 0xcb, 0xcb, 0xcb, 0xcb, 0xcb, 0x00},
	},
	/* blue */
	{
		{0x00, 0xd8, 0xd5, 0xd5, 0xd5, 0xd5, 0xdc, 0x00},
		{0xd8, 0xd5, 0xd2, 0xd0, 0xd0, 0xd2, 0xd8, 0xdc},
		{0xd5, 0xd2, 0xfe, 0xd0, 0xd0, 0xd0, 0xd5, 0xdc},
		{0xd5, 0xd0, 0xd0, 0xd0, 0xd0, 0xd0, 0xd5, 0xdc},
		{0xd5, 0xd0, 0xd0, 0xd0, 0xd0, 0xd2, 0xd5, 0xdc},
		{0xd5, 0xd2, 0xd0, 0xd0, 0xd2, 0xd5, 0xd8, 0xdc},
		{0xdc, 0xd8, 0xd5, 0xd5, 0xd5, 0xd8, 0xdc, 0xdc},
		{0x00, 0xdc, 0xdc, 0xdc, 0xdc, 0xdc, 0xdc, 0x00},
	},
}

func W_InsertOcranaLeds(data []byte) {
	for i := 0; i < 4; i++ {
		leddata := (0x80 >> 4 << 10) + (0x06 << 3) + (i << 3)
		for y := 0; y < 8; y++ {
			for x := 0; x < 8; x++ {
				data[leddata] = ocrana_leds[i][y][x]
				leddata++
			}
			leddata += 128 - 8
		}
	}
}

/*
=============================================================================
WAD3 Texture Loading for BSP 3.0 Support
=============================================================================
*/

const TEXWAD_MAXIMAGES = 16384

type texwadlump_t struct {
	name     string
	file     *os.File
	position int
	size     int
}

var texwadlump []texwadlump_t

func WAD3_ClearTextureWads() {
	for i := range texwadlump {
		if texwadlump[i].file == nil {
			continue
		}

		for j := i + 1; j < len(texwadlump); j++ {
			if texwadlump[j].file == texwadlump[i].file {
				texwadlump[j].file = nil
			}
		}

		texwadlump[i].file.Close()
	}

	texwadlump = nil
}

func WAD3_LoadTextureWadFile(filename string) {
	var file *os.File
	var flen int

	if file, flen = FS_FOpenFile(va("textures/wad3/%s", filename)); flen == -1 {
		if file, flen = FS_FOpenFile(va("textures/halflife/%s", filename)); flen == -1 {
			if file, flen = FS_FOpenFile(va("textures/%s", filename)); flen == -1 {
				if file, flen = FS_FOpenFile(filename); flen == -1 {
					Host_Error("Couldn't load halflife wad \"%s\"\n", filename)
				}
			}
		}
	}

	var header [sizeof_wadinfo_t]byte
	if _, err := io.ReadFull(file, header[:]); err != nil {
		Com_Printf("WAD3_LoadTextureWadFile: unable to read wad header")
		file.Close()
		return
	}

	if string(header[:4]) != "WAD3" {
		Com_Printf("WAD3_LoadTextureWadFile: Wad file %s doesn't have WAD3 id\n", filename)
		file.Close()
		return
	}

	numlumps := int(leLong(header[4:]))
	if numlumps < 1 || numlumps > TEXWAD_MAXIMAGES {
		Com_Printf("WAD3_LoadTextureWadFile: invalid number of lumps (%d)\n", numlumps)
		file.Close()
		return
	}

	if len(texwadlump)+numlumps > TEXWAD_MAXIMAGES {
		Com_Printf("WAD3_LoadTextureWadFile: Too many lumps loaded\n")
		file.Close()
		return
	}

	// the seek is absolute like in C, so a wad inside a pak reads wrong
	infotableofs := int(leLong(header[8:]))
	if _, err := file.Seek(int64(infotableofs), io.SeekStart); err != nil {
		Com_Printf("WAD3_LoadTextureWadFile: unable to seek to lump table")
		file.Close()
		return
	}

	lumps := make([]byte, sizeof_lumpinfo_t*numlumps)
	if _, err := io.ReadFull(file, lumps); err != nil {
		Com_Printf("WAD3_LoadTextureWadFile: unable to read lump table")
		file.Close()
		return
	}

	for i := 0; i < numlumps; i++ {
		lump_p := readLumpinfo(lumps[i*sizeof_lumpinfo_t:])
		lump_p.name = W_CleanupName(lump_p.name)
		j := 0
		for j = 0; j < len(texwadlump); j++ {
			if lump_p.name == texwadlump[j].name {
				break
			}
		}
		if j == len(texwadlump) {
			texwadlump = append(texwadlump, texwadlump_t{name: strlcpy(lump_p.name, MAX_QPATH)})
		}
		texwadlump[j].file = file
		texwadlump[j].position = int(lump_p.filepos)
		texwadlump[j].size = int(lump_p.disksize)
	}
}

// converts paletted to rgba
func ConvertWad3ToRGBA(tex []byte) []byte {
	offset := int(leLong(tex[24:]))
	if offset == 0 {
		Sys_Error("ConvertWad3ToRGBA: tex->offsets[0] == 0")
	}

	width := int(leLong(tex[16:]))
	height := int(leLong(tex[20:]))
	image_size := width * height
	in := tex[offset:]
	data := make([]byte, image_size*4)
	pal := in[((image_size*85)>>6)+2:]
	for i := 0; i < image_size; i++ {
		p := int(in[i])
		if tex[0] == '{' && p == 255 {
			data[i*4+0] = 0
			data[i*4+1] = 0
			data[i*4+2] = 0
			data[i*4+3] = 0
		} else {
			p *= 3
			data[i*4+0] = pal[p]
			data[i*4+1] = pal[p+1]
			data[i*4+2] = pal[p+2]
			data[i*4+3] = 255
		}
	}
	return data
}

// mt is the miptex_t from the map; the pixels come from it or from one of
// the loaded texture wads
func WAD3_LoadTexture(mt []byte) []byte {
	if leLong(mt[24:]) != 0 {
		return ConvertWad3ToRGBA(mt)
	}

	texname := W_CleanupName(cstr(mt[:16]))
	for i := range texwadlump {
		if texname != texwadlump[i].name {
			continue
		}

		file := texwadlump[i].file
		if _, err := file.Seek(int64(texwadlump[i].position), io.SeekStart); err != nil {
			Com_Printf("WAD3_LoadTexture: corrupt WAD3 file")
			return nil
		}

		tex := make([]byte, texwadlump[i].size)
		if _, err := io.ReadFull(file, tex); err != nil || len(tex) < sizeof_miptex_t {
			Com_Printf("WAD3_LoadTexture: corrupt WAD3 file")
			return nil
		}

		if leLong(tex[16:]) == leLong(mt[16:]) && leLong(tex[20:]) == leLong(mt[20:]) {
			return ConvertWad3ToRGBA(tex)
		}
		return nil
	}

	return nil
}

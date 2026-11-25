package engine

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// gpu_draw2d.c: the 2D batcher and the picture layer ported from gl_draw.c

const MAX_BATCHES = 4096

var draw2d_verts [GPU_UI_MAX_VERTS]ui_vert_t
var draw2d_numverts uint32

var draw2d_batches [MAX_BATCHES]ui_batch_t
var draw2d_numbatches uint32

var warned_overflow int

func Draw2D_FrameReset() {
	draw2d_numverts = 0
	draw2d_numbatches = 0
	warned_overflow = 0
}

func Draw2D_Quad(texnum, alphatest int,
	x0, y0, x1, y1 float32,
	s0, t0, s1, t1 float32,
	rgba [4]byte) {
	var b *ui_batch_t
	ow := float32(vid.conwidth)
	oh := float32(vid.conheight)

	if draw2d_numverts+6 > GPU_UI_MAX_VERTS || draw2d_numbatches >= MAX_BATCHES {
		if warned_overflow == 0 {
			Com_Printf("Draw2D: batch overflow\n")
			warned_overflow = 1
		}
		return
	}

	if draw2d_numbatches != 0 {
		b = &draw2d_batches[draw2d_numbatches-1]
	}
	if b == nil || b.texnum != texnum || b.alphatest != alphatest ||
		b.ortho_w != ow || b.ortho_h != oh {
		b = &draw2d_batches[draw2d_numbatches]
		draw2d_numbatches++
		b.texnum = texnum
		b.alphatest = alphatest
		b.firstvert = draw2d_numverts
		b.numverts = 0
		b.ortho_w = ow
		b.ortho_h = oh
	}

	v := draw2d_verts[draw2d_numverts : draw2d_numverts+6]

	v[0] = ui_vert_t{x: x0, y: y0, u: s0, v: t0, rgba: rgba}
	v[1] = ui_vert_t{x: x1, y: y0, u: s1, v: t0, rgba: rgba}
	v[2] = ui_vert_t{x: x1, y: y1, u: s1, v: t1, rgba: rgba}
	v[3] = ui_vert_t{x: x0, y: y0, u: s0, v: t0, rgba: rgba}
	v[4] = ui_vert_t{x: x1, y: y1, u: s1, v: t1, rgba: rgba}
	v[5] = ui_vert_t{x: x0, y: y1, u: s0, v: t1, rgba: rgba}

	draw2d_numverts += 6
	b.numverts += 6
}

func Draw2D_GetVerts() []ui_vert_t {
	return draw2d_verts[:draw2d_numverts]
}

func Draw2D_GetBatches() []ui_batch_t {
	return draw2d_batches[:draw2d_numbatches]
}

// ---- 2D drawing layer, ported from gl_draw.c ----

// draw.h
const (
	DRAW_LOADPICTURE_NOFALLBACK = iota
	DRAW_LOADPICTURE_DUMMYFALLBACK
)

type Picture struct {
	texnum        int
	width         uint
	height        uint
	glwidthscale  float32
	glheightscale float32

	texcoords [4 * 2]float32
}

var drawgl_inited byte

var colour_white = [4]byte{255, 255, 255, 255}

var gl_crosshairimage = cvar_t{name: "crosshairimage", string: ""}
var gl_consolefont = cvar_t{name: "gl_consolefont", string: "classicq"}
var gl_crosshairalpha = cvar_t{name: "crosshairalpha", string: "1"}
var gl_smoothfont = cvar_t{name: "gl_smoothfont", string: "0"}

// the callbacks reach back to these cvars, so the hooks go in here rather
// than in the literals
func init() {
	gl_crosshairimage.OnChange = OnChange_gl_crosshairimage
	gl_consolefont.OnChange = OnChange_gl_consolefont
	gl_crosshairalpha.PostChangeCallback = PostChange_crosshairstuff
	gl_smoothfont.OnChange = OnChange_gl_smoothfont
}

var draw_chars []byte // 8*8 graphic characters

var char_texture int

const NUMCROSSHAIRS = 6

var crosshairtextures [NUMCROSSHAIRS]int
var crosshairtexture_txt int
var crosshairpic *Picture

var customcrosshairdata [64]byte

const (
	CROSSHAIR_NONE  = 0
	CROSSHAIR_TXT   = 1
	CROSSHAIR_IMAGE = 2
)

var customcrosshair_loaded = CROSSHAIR_NONE

var crosshairdata = [NUMCROSSHAIRS][64]byte{
	{
		0xff, 0xff, 0xff, 0xfe, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xfe, 0xff, 0xff, 0xff, 0xff,
		0xfe, 0xff, 0xfe, 0xff, 0xfe, 0xff, 0xfe, 0xff,
		0xff, 0xff, 0xff, 0xfe, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xfe, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	},

	{
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xfe, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xfe, 0xfe, 0xfe, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xfe, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	},

	{
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xfe, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	},

	{
		0xfe, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xfe,
		0xff, 0xfe, 0xff, 0xff, 0xff, 0xff, 0xfe, 0xff,
		0xff, 0xff, 0xfe, 0xff, 0xff, 0xfe, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xfe, 0xff, 0xff, 0xfe, 0xff, 0xff,
		0xff, 0xfe, 0xff, 0xff, 0xff, 0xff, 0xfe, 0xff,
		0xfe, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xfe,
	},

	{
		0xff, 0xff, 0xfe, 0xfe, 0xfe, 0xff, 0xff, 0xff,
		0xff, 0xfe, 0xff, 0xfe, 0xff, 0xfe, 0xff, 0xff,
		0xfe, 0xfe, 0xff, 0xfe, 0xff, 0xfe, 0xfe, 0xff,
		0xfe, 0xfe, 0xfe, 0xfe, 0xfe, 0xfe, 0xfe, 0xff,
		0xfe, 0xff, 0xfe, 0xfe, 0xfe, 0xff, 0xfe, 0xff,
		0xff, 0xfe, 0xff, 0xff, 0xff, 0xfe, 0xff, 0xff,
		0xff, 0xff, 0xfe, 0xfe, 0xfe, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	},

	{
		0xff, 0xff, 0xfe, 0xfe, 0xfe, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xfe, 0xff, 0xff, 0xff, 0xff, 0xff, 0xfe, 0xff,
		0xfe, 0xff, 0xff, 0xfe, 0xff, 0xff, 0xfe, 0xff,
		0xfe, 0xff, 0xff, 0xff, 0xff, 0xff, 0xfe, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xfe, 0xfe, 0xfe, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	},
}

func SetupFontSmoothing() {
	if char_texture == 0 {
		return
	}

	prefs := 0
	if gl_smoothfont.value != 0 {
		prefs = GPU_TEXPREF_LINEAR
	}
	GPU_Texture_SetPrefs(char_texture, prefs)
}

func OnChange_gl_smoothfont(v *cvar_t, value string) bool {
	if drawgl_inited != 0 {
		v.value = Q_atof(value)

		SetupFontSmoothing()
	}

	return false
}

func Crosshair_LoadImage(s string) {
	if crosshairpic != nil {
		Draw_FreePicture(crosshairpic)
		crosshairpic = nil
	}

	customcrosshair_loaded &^= CROSSHAIR_IMAGE

	if s != "" {
		pic := Draw_LoadPicture(va("crosshairs/%s", s), DRAW_LOADPICTURE_NOFALLBACK)
		if pic != nil {
			// force NEAREST, keep clamp
			GPU_Texture_SetPrefs(pic.texnum, GPU_TEXPREF_CLAMP)
			crosshairpic = pic
			customcrosshair_loaded |= CROSSHAIR_IMAGE
		} else {
			Com_Printf("Couldn't load image %s\n", s)
		}
	}

	Draw_RecalcCrosshair()
}

func OnChange_gl_crosshairimage(v *cvar_t, s string) bool {
	if drawgl_inited != 0 {
		Crosshair_LoadImage(s)
	}

	return false
}

func customCrosshair_Init() {
	i := 0

	customcrosshair_loaded = CROSSHAIR_NONE

	Crosshair_LoadImage(gl_crosshairimage.string)

	f, flen := FS_FOpenFile("crosshairs/crosshair.txt")
	if flen == -1 {
		return
	}

	data := make([]byte, flen)
	n, _ := io.ReadFull(f, data)
	f.Close()
	data = data[:n]

	for p := 0; i < 64; p++ {
		if p >= len(data) {
			Com_Printf("Invalid format in crosshair.txt (Need 64 X's and O's)\n")
			return
		}
		c := data[p]
		if isspace(c) {
			continue
		}
		if tolower(c) != 'x' && tolower(c) != 'o' {
			Com_Printf("Invalid format in crosshair.txt (Only X's and O's and whitespace permitted)\n")
			return
		}
		if c == 'x' || c == 'X' {
			customcrosshairdata[i] = 0xfe
		} else {
			customcrosshairdata[i] = 0xff
		}
		i++
	}
	crosshairtexture_txt = R_LoadTexture("", 8, 8, customcrosshairdata[:], TEX_ALPHA, 1)
	GPU_Texture_SetPrefs(crosshairtexture_txt, 0)
	customcrosshair_loaded |= CROSSHAIR_TXT
}

func Draw_SizeChanged() {
	Draw_RecalcCrosshair()
}

func Draw_LoadCharset(name string) int {
	if Q_strcasecmp(name, "original") == 0 {
		var buf [128 * 256]byte

		// blank rows between character rows keep LINEAR from bleeding
		for i := range buf {
			buf[i] = 255
		}
		src := 0
		dest := 0
		for i := 0; i < 16; i++ {
			copy(buf[dest:dest+128*8], draw_chars[src:src+128*8])
			src += 128 * 8
			dest += 128 * 8 * 2
		}
		char_texture = R_LoadTexture("pic:charset", 128, 256, buf[:], TEX_ALPHA, 1)
	} else if texnum := R_LoadCharsetImage(va("textures/charsets/%s", name), "pic:charset"); texnum != 0 {
		char_texture = texnum
	} else {
		Com_Printf("Couldn't load charset \"%s\"\n", name)
		return 1
	}

	SetupFontSmoothing()

	return 0
}

func OnChange_gl_consolefont(v *cvar_t, value string) bool {
	if drawgl_inited != 0 {
		return Draw_LoadCharset(value) != 0
	}

	return false
}

func Draw_LoadCharset_f() {
	switch Cmd_Argc() {
	case 1:
		Com_Printf("Current charset is \"%s\"\n", gl_consolefont.string)
	case 2:
		Cvar_Set(&gl_consolefont, Cmd_Argv(1))
	default:
		Com_Printf("Usage: %s <charset>\n", Cmd_Argv(0))
	}
}

func Draw_InitCharset() {
	draw_chars = W_GetLumpName("conchars")
	for i := 0; i < 256*64; i++ {
		if draw_chars[i] == 0 {
			draw_chars[i] = 255
		}
	}

	Draw_LoadCharset(gl_consolefont.string)

	if char_texture == 0 {
		Cvar_Set(&gl_consolefont, "original")
		Draw_LoadCharset(gl_consolefont.string)
	}

	if char_texture == 0 {
		Sys_Error("Draw_InitCharset: Couldn't load charset")
	}
}

func DrawImp_CvarInit() {
	Cmd_AddCommand("loadcharset", Draw_LoadCharset_f)

	Cvar_SetCurrentGroup(CVAR_GROUP_CONSOLE)
	Cvar_Register(&gl_smoothfont)
	Cvar_Register(&gl_consolefont)

	Cvar_SetCurrentGroup(CVAR_GROUP_CROSSHAIR)
	Cvar_Register(&gl_crosshairimage)
	Cvar_Register(&gl_crosshairalpha)

	Cvar_ResetCurrentGroup()

	R_Texture_CvarInit()
}

var dummypicture *Picture

var dummyfallbackdata = []byte{
	0xff, 0xff, 0xff, 0xff,
	0x00, 0x00, 0x00, 0xff,
	0x00, 0x00, 0x00, 0xff,
	0xff, 0xff, 0xff, 0xff,
}

func Draw_CreateDummyPicture() {
	dummypicture = &Picture{}
	dummypicture.texnum = texture_extension_number
	texture_extension_number++

	dummypicture.width = 2
	dummypicture.height = 2

	dummypicture.glwidthscale = 1
	dummypicture.glheightscale = 1

	dummypicture.texcoords = [8]float32{0, 0, 1, 0, 1, 1, 0, 1}

	tex := GPU_CreateTextureRGBA(dummyfallbackdata, 2, 2, 0)
	GPU_Texture_Set(dummypicture.texnum, tex, 2, 2, GPU_TEXPREF_CLAMP)
}

func Draw_DeleteDummyPicture() {
	dummypicture = nil
}

func DrawImp_Init() {
	Draw_InitCharset()

	for i := 0; i < NUMCROSSHAIRS; i++ {
		crosshairtextures[i] = R_LoadTexture("", 8, 8, crosshairdata[i][:], TEX_ALPHA, 1)
		GPU_Texture_SetPrefs(crosshairtextures[i], 0)
	}
	customCrosshair_Init()

	Draw_RecalcCrosshair()

	Draw_CreateDummyPicture()

	drawgl_inited = 1
}

func DrawImp_Shutdown() {
	if crosshairpic != nil {
		Draw_FreePicture(crosshairpic)
		crosshairpic = nil
	}

	Draw_DeleteDummyPicture()

	drawgl_inited = 0
}

var textrenderingenabled int
var colouredtextrendering int

var fontcolour = [4]byte{255, 255, 255, 255}

func DrawImp_SetTextColor(r, g, b int) {
	fontcolour[0] = byte(r | (r << 4))
	fontcolour[1] = byte(g | (g << 4))
	fontcolour[2] = byte(b | (b << 4))
}

// Draws one 8*8 graphics character with 0 being transparent.
// It can be clipped to the top of the screen to allow the console to be smoothly scrolled off.
func DrawImp_Character(x, y int, num byte) {
	frow := float32(float64(num>>4) * 0.0625)
	fcol := float32(float64(num&15) * 0.0625)

	colour := colour_white
	if colouredtextrendering != 0 {
		colour = fontcolour
	}
	Draw2D_Quad(char_texture, 1,
		float32(x), float32(y), float32(x+8), float32(y+8),
		fcol, frow, fcol+0.0625, frow+0.03125,
		colour)
}

func Draw_BeginTextRendering() {
	textrenderingenabled++
}

func Draw_EndTextRendering() {
	textrenderingenabled--

	if textrenderingenabled == 0 {
		colouredtextrendering = 0
	}
}

func Draw_BeginColoredTextRendering() {
	if textrenderingenabled == 0 {
		colouredtextrendering = 1
	}

	textrenderingenabled++
}

func Draw_EndColoredTextRendering() {
	Draw_EndTextRendering()
	fontcolour = [4]byte{255, 255, 255, 255}
}

var crosshairtexnum int
var crosshair_x0, crosshair_y0, crosshair_x1, crosshair_y1 float32
var crosshaircolour [4]byte

func Draw_RecalcCrosshair() {
	var ofs1, ofs2 float32

	x := float32(scr_vrect.x+scr_vrect.width/2) + cl_crossx.value
	y := float32(scr_vrect.y+scr_vrect.height/2) + cl_crossy.value

	c := StringToRGB(crosshaircolor.string)

	crosshaircolour[0] = c[0]
	crosshaircolour[1] = c[1]
	crosshaircolour[2] = c[2]
	crosshaircolour[3] = byte(bound(float32(0), gl_crosshairalpha.value, float32(1)) * 255)

	if customcrosshair_loaded&CROSSHAIR_IMAGE != 0 {
		crosshairtexnum = crosshairpic.texnum
		ofs1 = 4 - 4.0/16
		ofs2 = 4 + 4.0/16
	} else {
		if crosshair.value >= 2 {
			// C indexes the table unchecked; Draw_Crosshair only uses it in range
			if idx := int(crosshair.value) - 2; idx < NUMCROSSHAIRS {
				crosshairtexnum = crosshairtextures[idx]
			} else {
				crosshairtexnum = 0
			}
		} else {
			crosshairtexnum = crosshairtexture_txt
		}
		ofs1 = 3.5
		ofs2 = 4.5
	}
	ofs1 *= float32(vid.conwidth/320) * bound(float32(0), crosshairsize.value, float32(20))
	ofs2 *= float32(vid.conwidth/320) * bound(float32(0), crosshairsize.value, float32(20))

	crosshair_x0 = x - ofs1
	crosshair_y0 = y - ofs1
	crosshair_x1 = x + ofs2
	crosshair_y1 = y + ofs2
}

func PostChange_crosshairstuff(v *cvar_t) {
	Draw_RecalcCrosshair()
}

func Draw_Crosshair() {
	if (crosshair.value >= 2 && crosshair.value <= NUMCROSSHAIRS+1) ||
		(customcrosshair_loaded&CROSSHAIR_TXT != 0 && crosshair.value == 1) ||
		customcrosshair_loaded&CROSSHAIR_IMAGE != 0 {
		if gl_crosshairalpha.value == 0 {
			return
		}

		Draw2D_Quad(crosshairtexnum, 0,
			crosshair_x0, crosshair_y0, crosshair_x1, crosshair_y1,
			0, 0, 1, 1,
			crosshaircolour)
	} else if crosshair.value != 0 {
		Draw_Character(int(float32(scr_vrect.x+scr_vrect.width/2-4)+cl_crossx.value),
			int(float32(scr_vrect.y+scr_vrect.height/2-4)+cl_crossy.value), '+')
	}
}

func Draw_AlphaFillRGB(x, y, w, h int, r, g, b, alpha float32) {
	var col [4]byte

	alpha = bound(float32(0), alpha, float32(1))

	if alpha == 0 {
		return
	}

	col[0] = byte(r * 255)
	col[1] = byte(g * 255)
	col[2] = byte(b * 255)
	col[3] = byte(alpha * 255)

	Draw2D_Quad(GPU_Texture_White(), 0, float32(x), float32(y), float32(x+w), float32(y+h), 0, 0, 1, 1, col)
}

func Draw_AlphaFill(x, y, w, h int, c int, alpha float32) {
	r := float32(float64(host_basepal[c*3+0]) / 255.0)
	g := float32(float64(host_basepal[c*3+1]) / 255.0)
	b := float32(float64(host_basepal[c*3+2]) / 255.0)

	Draw_AlphaFillRGB(x, y, w, h, r, g, b, alpha)
}

func Draw_Fill(x, y, w, h int, c int) {
	Draw_AlphaFill(x, y, w, h, c, 1)
}

//=============================================================================

func Draw_FadeScreen() {
	var col [4]byte

	alpha := bound(float32(0), scr_menualpha.value, float32(1))
	if alpha == 0 {
		return
	}

	col[3] = byte(alpha * 255)

	Draw2D_Quad(GPU_Texture_White(), 0, 0, 0, float32(vid.conwidth), float32(vid.conheight), 0, 0, 1, 1, col)

	Sbar_Changed()
}

//=============================================================================

// a wad picture: int width, short height, then the pixels
func Draw_LoadWadPicture(name string, rwidth, rheight *uint) []byte {
	data := W_GetLumpName(name)

	if data != nil { /* Always true, Quake sucks. */
		width := uint(leLong(data))
		height := uint(leShort(data[4:]))

		if width < 32768 && height < 32768 {
			newdata := make([]byte, width*height)
			copy(newdata, data[8:8+width*height])
			*rwidth = width
			*rheight = height
			return newdata
		}
	}

	return nil
}

func Draw_LoadLmpPicture(fh *os.File, rwidth, rheight *uint) []byte {
	var header [8]byte

	if _, err := io.ReadFull(fh, header[:]); err == nil {
		width := uint(leLong(header[:]))
		height := uint(leLong(header[4:]))

		if width < 32768 && height < 32768 {
			size := width * height

			data := make([]byte, size)
			if _, err := io.ReadFull(fh, data); err == nil {
				*rwidth = width
				*rheight = height

				return data
			}
		}
	}

	return nil
}

func Draw_8to32(source []byte, width, height uint) []byte {
	if width == 0 || height == 0 || width >= 32768 || height >= 32768 {
		return nil
	}

	n := width * height
	dst := make([]byte, n*4)
	for i := uint(0); i < n; i++ {
		v := d_8to24table[source[i]]
		dst[i*4+0] = byte(v)
		dst[i*4+1] = byte(v >> 8)
		dst[i*4+2] = byte(v >> 16)
		dst[i*4+3] = byte(v >> 24)
	}

	return dst
}

var (
	menuplyr_pic          Picture
	menuplyr_src          [8192]byte
	menuplyr_src_w        uint
	menuplyr_src_h        uint
	menuplyr_src_state    int /* 0 = untried, 1 = loaded, -1 = missing */
	menuplyr_top_cache    = -1
	menuplyr_bottom_cache = -1
)

func Draw_BuildTranslatedMenuplyr(top, bottom int) *Picture {
	var trans [256]byte
	var rgba [8192 * 4]byte

	if menuplyr_src_state == 0 {
		var w, h uint

		if fh, flen := FS_FOpenFile("gfx/menuplyr.lmp"); flen != -1 {
			data := Draw_LoadLmpPicture(fh, &w, &h)
			fh.Close()

			if data != nil && w > 0 && h > 0 && w*h <= uint(len(menuplyr_src)) {
				copy(menuplyr_src[:], data[:w*h])
				menuplyr_src_w = w
				menuplyr_src_h = h
				menuplyr_src_state = 1
			} else {
				menuplyr_src_state = -1
			}
		} else {
			menuplyr_src_state = -1
		}
	}

	if menuplyr_src_state < 0 {
		return nil
	}

	pic := &menuplyr_pic

	// rebuild texture if the GPU device was torn down by vid_restart
	if pic.texnum != 0 && GPU_Texture_Lookup(pic.texnum, nil) == nil {
		pic.texnum = 0
		menuplyr_top_cache = -1
		menuplyr_bottom_cache = -1
	}

	if pic.texnum != 0 && top == menuplyr_top_cache && bottom == menuplyr_bottom_cache {
		return pic
	}

	for i := 0; i < 256; i++ {
		trans[i] = byte(i)
	}

	if top < 128 {
		for i := 0; i < 16; i++ {
			trans[16+i] = byte(top + i)
		}
	} else {
		// id palette skin ranges run backwards; mirror for slider direction
		for i := 0; i < 16; i++ {
			trans[16+i] = byte(top + 15 - i)
		}
	}

	if bottom < 128 {
		for i := 0; i < 16; i++ {
			trans[96+i] = byte(bottom + i)
		}
	} else {
		for i := 0; i < 16; i++ {
			trans[96+i] = byte(bottom + 15 - i)
		}
	}

	n := menuplyr_src_w * menuplyr_src_h
	for i := uint(0); i < n; i++ {
		t := trans[menuplyr_src[i]]

		if t == 255 {
			rgba[i*4+0] = 0
			rgba[i*4+1] = 0
			rgba[i*4+2] = 0
			rgba[i*4+3] = 0
		} else {
			rgba[i*4+0] = host_basepal[int(t)*3+0]
			rgba[i*4+1] = host_basepal[int(t)*3+1]
			rgba[i*4+2] = host_basepal[int(t)*3+2]
			rgba[i*4+3] = 255
		}
	}

	if pic.texnum == 0 {
		pic.texnum = texture_extension_number
		texture_extension_number++
	}

	tex := GPU_CreateTextureRGBA(rgba[:n*4], uint32(menuplyr_src_w), uint32(menuplyr_src_h), 0)
	GPU_Texture_Set(pic.texnum, tex, uint32(menuplyr_src_w), uint32(menuplyr_src_h), GPU_TEXPREF_CLAMP)

	pic.width = menuplyr_src_w
	pic.height = menuplyr_src_h
	pic.glwidthscale = 1.0
	pic.glheightscale = 1.0
	pic.texcoords = [8]float32{0, 0, 1, 0, 1, 1, 0, 1}

	menuplyr_top_cache = top
	menuplyr_bottom_cache = bottom

	return pic
}

func Draw_LoadPicture(name string, fallback int) *Picture {
	var picture *Picture
	var width, height uint
	var data, newdata []byte

	if strings.HasPrefix(name, "wad:") {
		picture = Draw_LoadPicture("textures/wad/"+name[4:], DRAW_LOADPICTURE_NOFALLBACK)
		if picture != nil {
			return picture
		}
		picture = Draw_LoadPicture("gfx/"+name[4:], DRAW_LOADPICTURE_NOFALLBACK)
		if picture != nil {
			return picture
		}
		data = Draw_LoadWadPicture(name[4:], &width, &height)
	} else {
		namelen := len(name)

		newname := COM_CopyAndStripExtension(name, namelen+1)

		newdata = Image_LoadTGA(nil, newname+".tga", 0, 0, &width, &height)
		if newdata == nil {
			newdata = Image_LoadPNG(nil, newname+".png", 0, 0, &width, &height)
		}

		if newdata == nil {
			data = Image_LoadPCX(nil, newname+".pcx", 0, 0, &width, &height)
		}

		if newdata == nil && data == nil {
			if namelen > 4 && name[namelen-4:] == ".lmp" {
				if fh, flen := FS_FOpenFile(name); flen != -1 {
					data = Draw_LoadLmpPicture(fh, &width, &height)

					fh.Close()
				}
			}
		}
	}

	if data != nil {
		newdata = Draw_8to32(data, width, height)
	}

	if newdata != nil {
		picture = &Picture{}
		picture.texnum = texture_extension_number
		texture_extension_number++
		picture.width = width
		picture.height = height
		picture.glwidthscale = 1
		picture.glheightscale = 1

		tex := GPU_CreateTextureRGBA(newdata, uint32(width), uint32(height), 0)
		GPU_Texture_Set(picture.texnum, tex, uint32(width), uint32(height), GPU_TEXPREF_LINEAR|GPU_TEXPREF_CLAMP)

		picture.texcoords = [8]float32{0, 0, 1, 0, 1, 1, 0, 1}
	}

	if picture != nil {
		return picture
	}

	if fallback == DRAW_LOADPICTURE_DUMMYFALLBACK {
		return dummypicture
	}

	return nil
}

func Draw_FreePicture(picture *Picture) {
	if picture != nil && picture != dummypicture {
		GPU_Texture_Set(picture.texnum, nil, 0, 0, 0)
	}
}

func Draw_GetPictureWidth(picture *Picture) uint {
	return picture.width
}

func Draw_GetPictureHeight(picture *Picture) uint {
	return picture.height
}

func Draw_DrawPicture(picture *Picture, x, y int, width, height uint) {
	Draw2D_Quad(picture.texnum, 1,
		float32(x), float32(y), float32(x+int(width)), float32(y+int(height)),
		picture.texcoords[0], picture.texcoords[1],
		picture.texcoords[4], picture.texcoords[5],
		colour_white)
}

func Draw_DrawPictureModulated(picture *Picture, x, y int, width, height uint, r, g, b, alpha float32) {
	var col [4]byte

	col[0] = byte(bound(float32(0), r, float32(1)) * 255)
	col[1] = byte(bound(float32(0), g, float32(1)) * 255)
	col[2] = byte(bound(float32(0), b, float32(1)) * 255)
	col[3] = byte(bound(float32(0), alpha, float32(1)) * 255)

	Draw2D_Quad(picture.texnum, 0,
		float32(x), float32(y), float32(x+int(width)), float32(y+int(height)),
		picture.texcoords[0], picture.texcoords[1],
		picture.texcoords[4], picture.texcoords[5],
		col)
}

func Draw_DrawSubPicture(picture *Picture, sx, sy, swidth, sheight float32, x, y int, width, height uint) {
	Draw2D_Quad(picture.texnum, 1,
		float32(x), float32(y), float32(x+int(width)), float32(y+int(height)),
		sx*picture.glwidthscale, sy*picture.glheightscale,
		(sx+swidth)*picture.glwidthscale, (sy+sheight)*picture.glheightscale,
		colour_white)
}

// ---- netgraph, ported from gl_ngraph.c ----

const NET_GRAPHHEIGHT = 32

var ngraph_texels [NET_GRAPHHEIGHT][NET_TIMINGS]byte

func R_LineGraph(x, h int) {
	var color int

	s := NET_GRAPHHEIGHT

	if h == 10000 {
		color = 0x6f // yellow
	} else if h == 9999 {
		color = 0x4f // red
	} else if h == 9998 {
		color = 0xd0 // blue
	} else {
		color = 0xfe // white
	}

	h = min(h, s)

	i := 0
	for i = 0; i < h; i++ {
		if i&1 != 0 {
			ngraph_texels[NET_GRAPHHEIGHT-i-1][x] = 0xff
		} else {
			ngraph_texels[NET_GRAPHHEIGHT-i-1][x] = byte(color)
		}
	}

	for ; i < s; i++ {
		ngraph_texels[NET_GRAPHHEIGHT-i-1][x] = 0xff
	}
}

var ngraph_pixels [NET_GRAPHHEIGHT * NET_TIMINGS * 4]byte

func R_NetGraph() {
	lost := CL_CalcNet()
	for a := 0; a < NET_TIMINGS; a++ {
		i := (cls.netchan.outgoing_sequence - a) & NET_TIMINGSMASK
		R_LineGraph(NET_TIMINGS-1-a, packet_latency[i])
	}

	for y := 0; y < NET_GRAPHHEIGHT; y++ {
		for x := 0; x < NET_TIMINGS; x++ {
			v := d_8to24table[ngraph_texels[y][x]]
			p := (y*NET_TIMINGS + x) * 4
			ngraph_pixels[p+0] = byte(v)
			ngraph_pixels[p+1] = byte(v >> 8)
			ngraph_pixels[p+2] = byte(v >> 16)
			ngraph_pixels[p+3] = byte(v >> 24)
		}
	}

	x := 0
	y := int(vid.conheight) - sb_lines - 24 - NET_GRAPHHEIGHT - 1

	if r_netgraph.value != 2 && r_netgraph.value != 3 {
		Draw_TextBox(x, y, NET_TIMINGS/8, NET_GRAPHHEIGHT/8+1)
	}

	if r_netgraph.value != 3 {
		st := fmt.Sprintf("%3d%% packet loss", lost)
		Draw_String(8, y+8, st)
	}

	x = 8
	y += 16

	if netgraphtexture == 0 {
		netgraphtexture = texture_extension_number
		texture_extension_number++
	}

	GPU_UpdateTextureRGBA(netgraphtexture, ngraph_pixels[:],
		NET_TIMINGS, NET_GRAPHHEIGHT, GPU_TEXPREF_LINEAR)

	Draw2D_Quad(netgraphtexture, 0,
		float32(x), float32(y), float32(x+NET_TIMINGS), float32(y+NET_GRAPHHEIGHT),
		0, 0, 1, 1, colour_white)
}

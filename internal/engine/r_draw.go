package engine

var scr_conalpha = cvar_t{name: "scr_conalpha", string: "0.8"}
var scr_menualpha = cvar_t{name: "scr_menualpha", string: "0.7"}

var conbackpic *Picture

var backtilepic *Picture

var box_tlpic *Picture
var box_mlpic *Picture
var box_blpic *Picture
var box_tmpic *Picture
var box_mmpic *Picture
var box_mm2pic *Picture
var box_bmpic *Picture
var box_trpic *Picture
var box_mrpic *Picture
var box_brpic *Picture

//=============================================================================
/* Support Routines */

func Draw_CvarInit() {
	Cvar_SetCurrentGroup(CVAR_GROUP_CONSOLE)
	Cvar_Register(&scr_conalpha)

	Cvar_SetCurrentGroup(CVAR_GROUP_SCREEN)
	Cvar_Register(&scr_menualpha)

	Cvar_ResetCurrentGroup()

	DrawImp_CvarInit()
}

func Draw_Init() int {
	DrawImp_Init()

	conbackpic = Draw_LoadPicture("gfx/conback.lmp", DRAW_LOADPICTURE_NOFALLBACK)
	if conbackpic != nil {
		backtilepic = Draw_LoadPicture("wad:backtile", DRAW_LOADPICTURE_DUMMYFALLBACK)

		box_tlpic = Draw_LoadPicture("gfx/box_tl.lmp", DRAW_LOADPICTURE_DUMMYFALLBACK)
		box_mlpic = Draw_LoadPicture("gfx/box_ml.lmp", DRAW_LOADPICTURE_DUMMYFALLBACK)
		box_blpic = Draw_LoadPicture("gfx/box_bl.lmp", DRAW_LOADPICTURE_DUMMYFALLBACK)
		box_tmpic = Draw_LoadPicture("gfx/box_tm.lmp", DRAW_LOADPICTURE_DUMMYFALLBACK)
		box_mmpic = Draw_LoadPicture("gfx/box_mm.lmp", DRAW_LOADPICTURE_DUMMYFALLBACK)
		box_mm2pic = Draw_LoadPicture("gfx/box_mm2.lmp", DRAW_LOADPICTURE_DUMMYFALLBACK)
		box_bmpic = Draw_LoadPicture("gfx/box_bm.lmp", DRAW_LOADPICTURE_DUMMYFALLBACK)
		box_trpic = Draw_LoadPicture("gfx/box_tr.lmp", DRAW_LOADPICTURE_DUMMYFALLBACK)
		box_mrpic = Draw_LoadPicture("gfx/box_mr.lmp", DRAW_LOADPICTURE_DUMMYFALLBACK)
		box_brpic = Draw_LoadPicture("gfx/box_br.lmp", DRAW_LOADPICTURE_DUMMYFALLBACK)

		return 1
	}

	DrawImp_Shutdown()

	return 0
}

func Draw_Shutdown() {
	Draw_FreePicture(box_tlpic)
	Draw_FreePicture(box_mlpic)
	Draw_FreePicture(box_blpic)
	Draw_FreePicture(box_tmpic)
	Draw_FreePicture(box_mmpic)
	Draw_FreePicture(box_mm2pic)
	Draw_FreePicture(box_bmpic)
	Draw_FreePicture(box_trpic)
	Draw_FreePicture(box_mrpic)
	Draw_FreePicture(box_brpic)

	Draw_FreePicture(backtilepic)

	Draw_FreePicture(conbackpic)

	DrawImp_Shutdown()
}

func Draw_Character(x, y int, num byte) {
	if y <= -8 {
		return
	}

	if num == 32 || num == (32|128) {
		return
	}

	DrawImp_Character(x, y, num)
}

func Draw_String(x, y int, str string) {
	if str == "" {
		return
	}

	Draw_BeginTextRendering()

	for i := 0; i < len(str); i++ {
		Draw_Character(x, y, str[i])
		x += 8
	}

	Draw_EndTextRendering()
}

func Draw_String_Length(x, y int, str string, length int) {
	if length == 0 {
		return
	}

	Draw_BeginTextRendering()

	for i := 0; length > 0; i, length = i+1, length-1 {
		Draw_Character(x, y, at(str, i))
		x += 8
	}

	Draw_EndTextRendering()
}

func Draw_Alt_String(x, y int, str string) {
	if str == "" {
		return
	}

	Draw_BeginTextRendering()

	for i := 0; i < len(str); i++ {
		num := str[i]
		num |= 0x80
		Draw_Character(x, y, num)
		x += 8
	}

	Draw_EndTextRendering()
}

func HexToInt(c byte) int {
	if isdigit(c) {
		return int(c - '0')
	} else if c >= 'a' && c <= 'f' {
		return 10 + int(c-'a')
	} else if c >= 'A' && c <= 'F' {
		return 10 + int(c-'A')
	} else {
		return -1
	}
}

func Draw_ColoredString(x, y int, text string, red int) {
	Draw_BeginColoredTextRendering()

	for i := 0; i < len(text); i++ {
		if text[i] == '&' {
			if at(text, i+1) == 'c' && at(text, i+2) != 0 && at(text, i+3) != 0 && at(text, i+4) != 0 {
				r := HexToInt(text[i+2])
				g := HexToInt(text[i+3])
				b := HexToInt(text[i+4])
				if r >= 0 && g >= 0 && b >= 0 {
					if scr_coloredText.value != 0 {
						DrawImp_SetTextColor(r, g, b)
					}
					i += 4
					continue
				}
			} else if at(text, i+1) == 'r' {
				if scr_coloredText.value != 0 {
					DrawImp_SetTextColor(15, 15, 15)
				}
				i += 1
				continue
			}
		}

		num := int(text[i]) & 255
		if scr_coloredText.value == 0 && red != 0 {
			num |= 128
		}

		Draw_Character(x, y, byte(num))
		x += 8
	}

	Draw_EndColoredTextRendering()
}

func Draw_ColoredString_Length(x, y int, text string, red int, length int, startcolour uint16) {
	Draw_BeginColoredTextRendering()

	DrawImp_SetTextColor(int(startcolour>>8)&15, int(startcolour>>4)&15, int(startcolour)&15)

	i := 0
	for length != 0 {
		if at(text, i) == '&' {
			if at(text, i+1) == 'c' && at(text, i+2) != 0 && at(text, i+3) != 0 && at(text, i+4) != 0 {
				r := HexToInt(at(text, i+2))
				g := HexToInt(at(text, i+3))
				b := HexToInt(at(text, i+4))
				if r >= 0 && g >= 0 && b >= 0 {
					if scr_coloredText.value != 0 {
						DrawImp_SetTextColor(r, g, b)
					}
					i += 5
					length -= 5
					continue
				}
			} else if at(text, i+1) == 'r' {
				if scr_coloredText.value != 0 {
					DrawImp_SetTextColor(15, 15, 15)
				}
				i += 2
				length -= 2
				continue
			}
		}

		num := int(at(text, i)) & 255
		if scr_coloredText.value == 0 && red != 0 {
			num |= 128
		}

		Draw_Character(x, y, byte(num))
		x += 8

		i++
		length--
	}

	Draw_EndColoredTextRendering()
}

func Draw_TextBox(x, y int, width, lines int) {
	// draw left side
	cx := x
	cy := y
	Draw_DrawPicture(box_tlpic, cx, cy, 8, 8)
	for n := 0; n < lines; n++ {
		cy += 8
		Draw_DrawPicture(box_mlpic, cx, cy, 8, 8)
	}
	Draw_DrawPicture(box_blpic, cx, cy+8, 8, 8)

	// draw middle
	cx += 8
	for width > 0 {
		cy = y
		Draw_DrawPicture(box_tmpic, cx, cy, 16, 8)
		Draw_DrawPicture(box_mmpic, cx, cy+8, 16, 8)
		cy += 16
		for n := 1; n < lines; n++ {
			Draw_DrawPicture(box_mm2pic, cx, cy, 16, 8)
			cy += 8
		}
		Draw_DrawPicture(box_bmpic, cx, cy, 16, 8)
		width -= 2
		cx += 16
	}

	// draw right side
	cy = y
	Draw_DrawPicture(box_trpic, cx, cy, 8, 8)
	for n := 0; n < lines; n++ {
		cy += 8
		Draw_DrawPicture(box_mrpic, cx, cy, 8, 8)
	}
	Draw_DrawPicture(box_brpic, cx, cy+8, 8, 8)
}

func Draw_ConsoleBackground(lines int) {
	if SCR_NEED_CONSOLE_BACKGROUND() {
		Draw_DrawPicture(conbackpic, 0, lines-int(vid.conheight), uint(vid.conwidth), uint(vid.conheight))
	} else {
		Draw_DrawPictureModulated(conbackpic, 0, lines-int(vid.conheight), uint(vid.conwidth), uint(vid.conheight), 1, 1, 1, scr_conalpha.value)
	}

	ver := "classicQ v" + CLASSICQ_VERSION
	Draw_Alt_String(int(vid.conwidth)-len(ver)*8-8, lines-10, ver)
}

// This repeats a 64*64 tile graphic to fill the screen around a sized down refresh window.
func Draw_TileClear(x, y int, width, height int) {
	yoffset := y % 64
	dsty := y
	dstheight := height
	tileheight := 64 - yoffset
	for dstheight != 0 {
		if tileheight > dstheight {
			tileheight = dstheight
		}

		xoffset := x % 64
		dstx := x
		dstwidth := width
		tilewidth := 64 - xoffset
		for dstwidth != 0 {
			if tilewidth > dstwidth {
				tilewidth = dstwidth
			}

			Draw_DrawSubPicture(backtilepic, float32(float64(xoffset)/64), float32(float64(yoffset)/64),
				float32(float64(tilewidth)/64), float32(float64(tileheight)/64), dstx, dsty, uint(tilewidth), uint(tileheight))

			dstx += tilewidth
			dstwidth -= tilewidth
			xoffset = 0
			tilewidth = 64
		}

		dsty += tileheight
		dstheight -= tileheight
		yoffset = 0
		tileheight = 64
	}
}

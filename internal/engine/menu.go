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

// menu.c -- the framework and the first half of the screens; the rest (single
// player, multiplayer, demos, setup, M_Draw and M_Keydown) follows in menu_screens.go

import "fmt"

const NUM_HELP_PAGES = 6

var qplaquepic *Picture
var ttl_mainpic *Picture
var mainmenupic *Picture
var menudotpic [6]*Picture
var p_optionpic *Picture
var ttl_cstmpic *Picture
var vidmodespic *Picture
var helppic [NUM_HELP_PAGES]*Picture
var ttl_sglpic *Picture
var sp_menupic *Picture
var p_loadpic *Picture
var p_savepic *Picture
var p_multipic *Picture
var bigboxpic *Picture
var menuplyrpic *Picture

type menustates int

const (
	m_none menustates = iota
	m_main
	m_singleplayer
	m_load
	m_save
	m_multiplayer
	m_setup
	m_options
	m_video
	m_video_verify
	m_keys
	m_help
	m_quit
	m_gameoptions
	m_fps
	m_demos
	m_teamcomms
)

var m_state menustates

var multiplayermenu *Menu
var optionsmenu *Menu
var fpsmenu *Menu

var optionsmenu_usemouse *MenuItem

var m_entersound bool /* play after drawing a frame, so caching won't disrupt the sound */
var m_recursiveDraw bool
var m_topmenu menustates /* set if a submenu was entered via a menu_* command */

//=============================================================================
/* Support Routines */

var scr_scaleMenu = cvar_t{name: "scr_scaleMenu", string: "0"}
var menuwidth = 320
var menuheight = 240

var scr_centerMenu = cvar_t{name: "scr_centerMenu", string: "1"}
var m_yofs = 0

func M_DrawCharacter(cx, line int, num int) {
	Draw_Character(cx+((menuwidth-320)>>1), line+m_yofs, byte(num))
}

func M_Print(cx, cy int, str string) {
	Draw_Alt_String(cx+((menuwidth-320)>>1), cy+m_yofs, str)
}

func M_PrintWhite(cx, cy int, str string) {
	Draw_String(cx+((menuwidth-320)>>1), cy+m_yofs, str)
}

func M_DrawPic(pic *Picture, x, y int, width, height int) {
	Draw_DrawPicture(pic, x+((menuwidth-320)>>1), y+m_yofs, uint(width), uint(height))
}

func M_DrawFill(x, y int, width, height int, color int) {
	Draw_Fill(x+((menuwidth-320)>>1), y+m_yofs, width, height, color)
}

func M_DrawTextBox(x, y int, width, lines int) {
	Draw_TextBox(x+((menuwidth-320)>>1), y+m_yofs, width, lines)
}

//=============================================================================

func M_ToggleMenu_f() {
	m_entersound = true

	if key_dest == key_menu {
		if m_state != m_main {
			M_Menu_Main_f()
			return
		}
		key_dest = key_game
		m_state = m_none
		VID_SetMouseGrab(1)
		return
	} else {
		VID_SetMouseGrab(0)
		M_Menu_Main_f()
	}
}

func M_EnterMenu(state menustates) {
	if key_dest != key_menu {
		m_topmenu = state
		Con_ClearNotify()
		// hide the console
		scr_conlines = 0
		scr_con_current = 0
	} else {
		m_topmenu = m_none
	}

	key_dest = key_menu
	m_state = state
	m_entersound = true
}

func M_LeaveMenu(parent menustates) {
	if m_topmenu == m_state {
		m_state = m_none
		key_dest = key_game
		VID_SetMouseGrab(1)
	} else {
		m_state = parent
		m_entersound = true
	}
}

/* Some less ugly menu crapola */

// C embeds MenuItem at the start of each item type and casts the pointer
// back; sub holds the enclosing item instead. The delete hook has no job here.
type MenuItem struct {
	next         *MenuItem
	label        string
	labellen     uint
	whitelabel   int
	isselectable int
	hidden       int

	selectindex int

	x uint
	y uint

	draw      func(menuitem *MenuItem, extra_x, extra_width uint)
	handlekey func(menuitem *MenuItem, key int)

	sub any
}

type MenuItemButton struct {
	menuitem MenuItem

	callback func()
}

type MenuItemCvarRange struct {
	menuitem MenuItem

	minvalue float32
	maxvalue float32
	step     float32
	cvar     *cvar_t
}

type MenuItemCvarBoolean struct {
	menuitem MenuItem

	cvar   *cvar_t
	invert int
}

type MenuItemCvarPosNegBoolean struct {
	menuitem MenuItem

	cvar *cvar_t
}

type MenuItemCvarMultiSelect struct {
	menuitem MenuItem

	cvar      *cvar_t
	values    []string
	numvalues int
}

type MenuItemCvarValuePreset struct {
	menuitem MenuItem

	cvar      *cvar_t
	values    []int
	numvalues uint
}

type Menu struct {
	error int

	items               *MenuItem
	selectableitemarray []*MenuItem
	numselectableitems  uint

	cursor_right int
	prevmenu     menustates

	cursor   uint
	cursor_x uint
	cursor_y uint

	extra_x     uint
	extra_width uint
}

func Menu_Create(cursor_right int, prevmenu menustates) *Menu {
	menu := &Menu{}
	menu.items = nil
	menu.selectableitemarray = nil
	menu.numselectableitems = 0
	menu.error = 0
	menu.cursor = 0

	menu.cursor_right = cursor_right
	menu.prevmenu = prevmenu

	return menu
}

func Menu_Delete(menu *Menu) {
	if menu == nil {
		return
	}

	menu.items = nil
	menu.selectableitemarray = nil
}

func Menu_Layout(menu *Menu) {
	var menuitem *MenuItem
	var numselectableitems uint
	var labelwidth uint
	var labelstart uint
	var y uint
	var i uint

	if menu == nil {
		return
	}

	labelwidth = 0
	numselectableitems = 0
	menuitem = menu.items
	for menuitem != nil {
		if menuitem.hidden == 0 {
			if menuitem.labellen > labelwidth {
				labelwidth = menuitem.labellen
			}

			if menuitem.isselectable != 0 {
				numselectableitems++
			}
		}

		menuitem = menuitem.next
	}

	menu.numselectableitems = numselectableitems

	menu.selectableitemarray = make([]*MenuItem, numselectableitems)

	labelstart = 56
	y = 32
	i = 0

	if menu.cursor_right != 0 {
		menu.cursor_x = labelstart + labelwidth*8 + 8
	} else {
		menu.cursor_x = labelstart + 8
		labelstart += 8 * 3
	}

	menuitem = menu.items
	for menuitem != nil {
		if menuitem.hidden == 0 {
			if menuitem.isselectable != 0 {
				menuitem.selectindex = int(i)
				menu.selectableitemarray[i] = menuitem
				i++
			}

			if menu.cursor_right != 0 {
				menuitem.x = labelstart + (labelwidth-menuitem.labellen)*8
			} else {
				menuitem.x = labelstart
			}
			menuitem.y = y

			y += 8
		}

		menuitem = menuitem.next
	}

	menu.cursor_y = 32

	menu.extra_x = menu.cursor_x + 16
	menu.extra_width = 320 - menu.extra_x - 8
}

func Menu_Draw(menu *Menu) {
	var menuitem *MenuItem

	if menu == nil || menu.error != 0 {
		return
	}

	menuitem = menu.items
	for menuitem != nil {
		if menuitem.hidden == 0 {
			if menuitem.whitelabel != 0 {
				M_PrintWhite(int(menuitem.x), int(menuitem.y), menuitem.label)
			} else {
				M_Print(int(menuitem.x), int(menuitem.y), menuitem.label)
			}

			menuitem.draw(menuitem, menu.extra_x, menu.extra_width)
		}

		menuitem = menuitem.next
	}

	M_DrawCharacter(int(menu.cursor_x), int(menu.selectableitemarray[menu.cursor].y), 12+(int(curtime*4)&1))
}

func Menu_ShowItem(menu *Menu, menuitem *MenuItem) {
	if menu == nil || menuitem == nil {
		return
	}

	menuitem.hidden = 0

	Menu_Layout(menu)

	if menuitem.isselectable != 0 && uint(menuitem.selectindex) <= menu.cursor {
		menu.cursor++
	}
}

func Menu_HideItem(menu *Menu, menuitem *MenuItem) {
	if menu == nil || menuitem == nil {
		return
	}

	menuitem.hidden = 1

	if menuitem.isselectable != 0 && menu.cursor != 0 && uint(menuitem.selectindex) <= menu.cursor {
		menu.cursor--
	}

	Menu_Layout(menu)
}

func Menu_HandleKey(menu *Menu, key int) {
	switch key {
	case int(K_BACKSPACE), int(K_ESCAPE):
		if key == int(K_BACKSPACE) {
			m_topmenu = m_none
		}
		if menu != nil {
			M_LeaveMenu(menu.prevmenu)
		} else {
			M_LeaveMenu(m_main)
		}
	}

	if menu == nil || menu.error != 0 {
		return
	}

	switch key {
	case int(K_HOME), int(K_PGUP):
		menu.cursor = 0

	case int(K_END), int(K_PGDN):
		menu.cursor = menu.numselectableitems - 1

	case int(K_UPARROW):
		if menu.cursor == 0 {
			menu.cursor = menu.numselectableitems - 1
		} else {
			menu.cursor--
		}

	case int(K_DOWNARROW):
		if menu.cursor == menu.numselectableitems-1 {
			menu.cursor = 0
		} else {
			menu.cursor++
		}

	default:
		menu.selectableitemarray[menu.cursor].handlekey(menu.selectableitemarray[menu.cursor], key)

		return
	}

	S_LocalSound("misc/menu1.wav")
}

func MenuItem_SetLabel(menuitem *MenuItem, label string, white int) int {
	menuitem.labellen = uint(len(label))
	menuitem.whitelabel = white
	menuitem.label = label

	return 1
}

func Menu_AddItem(menu *Menu, menuitem *MenuItem) {
	var mi *MenuItem

	if menuitem == nil {
		menu.error = 1
		return
	}

	menuitem.next = nil

	if menu.items != nil {
		mi = menu.items
		for mi.next != nil {
			mi = mi.next
		}

		mi.next = menuitem
	} else {
		menu.items = menuitem
	}
}

func MenuItemButton_HandleKey(menuitem *MenuItem, key int) {
	menuitembutton := menuitem.sub.(*MenuItemButton)

	if key == int(K_ENTER) {
		S_LocalSound("misc/menu2.wav")
		menuitembutton.callback()
	}
}

func MenuItemButton_Draw(menuitem *MenuItem, extra_x, extra_width uint) {
}

func MenuItemButton_Create(label string, white int, callback func()) *MenuItem {
	menuitembutton := &MenuItemButton{}
	if MenuItem_SetLabel(&menuitembutton.menuitem, label, white) != 0 {
		menuitembutton.menuitem.draw = MenuItemButton_Draw
		menuitembutton.menuitem.handlekey = MenuItemButton_HandleKey
		menuitembutton.menuitem.isselectable = 1
		menuitembutton.menuitem.hidden = 0
		menuitembutton.menuitem.sub = menuitembutton

		menuitembutton.callback = callback

		return &menuitembutton.menuitem
	}

	return nil
}

func MenuItemCvarRange_Draw(menuitem *MenuItem, extra_x, extra_width uint) {
	var i int
	var position float32

	menuitemcvarrange := menuitem.sub.(*MenuItemCvarRange)

	position = (bound(menuitemcvarrange.minvalue, menuitemcvarrange.cvar.value, menuitemcvarrange.maxvalue) - menuitemcvarrange.minvalue) / (menuitemcvarrange.maxvalue - menuitemcvarrange.minvalue)
	if menuitemcvarrange.step < 0 {
		position = 1 - position
	}

	M_DrawCharacter(int(extra_x), int(menuitemcvarrange.menuitem.y), 128)
	for i = 0; uint(i) < (extra_width/8)-2; i++ {
		M_DrawCharacter(int(extra_x)+8+i*8, int(menuitemcvarrange.menuitem.y), 129)
	}
	M_DrawCharacter(int(extra_x)+8+i*8, int(menuitemcvarrange.menuitem.y), 130)
	M_DrawCharacter(int(float32(extra_x+8)+(position*float32((i-1)*8))), int(menuitemcvarrange.menuitem.y), 131)
}

func MenuItemCvarRange_HandleKey(menuitem *MenuItem, key int) {
	menuitemcvarrange := menuitem.sub.(*MenuItemCvarRange)

	switch key {
	case int(K_ENTER), int(K_RIGHTARROW):
		Cvar_SetValue(menuitemcvarrange.cvar, bound(menuitemcvarrange.minvalue, menuitemcvarrange.cvar.value+menuitemcvarrange.step, menuitemcvarrange.maxvalue))

	case int(K_LEFTARROW):
		Cvar_SetValue(menuitemcvarrange.cvar, bound(menuitemcvarrange.minvalue, menuitemcvarrange.cvar.value-menuitemcvarrange.step, menuitemcvarrange.maxvalue))

	default:
		return
	}

	S_LocalSound("misc/menu3.wav")
}

func MenuItemCvarRange_Create(label string, minvalue, maxvalue, step float32, cvar *cvar_t) *MenuItem {
	menuitemcvarrange := &MenuItemCvarRange{}
	if MenuItem_SetLabel(&menuitemcvarrange.menuitem, label, 0) != 0 {
		menuitemcvarrange.menuitem.draw = MenuItemCvarRange_Draw
		menuitemcvarrange.menuitem.handlekey = MenuItemCvarRange_HandleKey
		menuitemcvarrange.menuitem.isselectable = 1
		menuitemcvarrange.menuitem.hidden = 0
		menuitemcvarrange.menuitem.sub = menuitemcvarrange

		menuitemcvarrange.minvalue = minvalue
		menuitemcvarrange.maxvalue = maxvalue
		menuitemcvarrange.step = step
		menuitemcvarrange.cvar = cvar

		return &menuitemcvarrange.menuitem
	}

	return nil
}

func MenuItemCvarBoolean_Draw(menuitem *MenuItem, extra_x, extra_width uint) {
	menuitemcvarboolean := menuitem.sub.(*MenuItemCvarBoolean)

	if (b2i(menuitemcvarboolean.cvar.value != 0) ^ menuitemcvarboolean.invert) != 0 {
		M_Print(int(extra_x+8), int(menuitem.y), "on")
	} else {
		M_Print(int(extra_x+8), int(menuitem.y), "off")
	}
}

func MenuItemCvarBoolean_HandleKey(menuitem *MenuItem, key int) {
	menuitemcvarboolean := menuitem.sub.(*MenuItemCvarBoolean)

	switch key {
	case int(K_ENTER), int(K_RIGHTARROW), int(K_LEFTARROW):
		S_LocalSound("misc/menu3.wav")
		Cvar_SetValue(menuitemcvarboolean.cvar, float32(b2i(menuitemcvarboolean.cvar.value == 0)))
	}
}

func MenuItemCvarBoolean_Create(label string, cvar *cvar_t, invert int) *MenuItem {
	menuitemcvarboolean := &MenuItemCvarBoolean{}
	if MenuItem_SetLabel(&menuitemcvarboolean.menuitem, label, 0) != 0 {
		menuitemcvarboolean.menuitem.draw = MenuItemCvarBoolean_Draw
		menuitemcvarboolean.menuitem.handlekey = MenuItemCvarBoolean_HandleKey
		menuitemcvarboolean.menuitem.isselectable = 1
		menuitemcvarboolean.menuitem.hidden = 0
		menuitemcvarboolean.menuitem.sub = menuitemcvarboolean

		menuitemcvarboolean.cvar = cvar
		menuitemcvarboolean.invert = invert

		return &menuitemcvarboolean.menuitem
	}

	return nil
}

func MenuItemCvarPosNegBoolean_Draw(menuitem *MenuItem, extra_x, extra_width uint) {
	menuitemcvarposnegboolean := menuitem.sub.(*MenuItemCvarPosNegBoolean)

	if menuitemcvarposnegboolean.cvar.value < 0 {
		M_Print(int(extra_x+8), int(menuitem.y), "on")
	} else {
		M_Print(int(extra_x+8), int(menuitem.y), "off")
	}
}

func MenuItemCvarPosNegBoolean_HandleKey(menuitem *MenuItem, key int) {
	menuitemcvarposnegboolean := menuitem.sub.(*MenuItemCvarPosNegBoolean)

	switch key {
	case int(K_ENTER), int(K_RIGHTARROW), int(K_LEFTARROW):
		S_LocalSound("misc/menu3.wav")
		Cvar_SetValue(menuitemcvarposnegboolean.cvar, -menuitemcvarposnegboolean.cvar.value)
	}
}

func MenuItemCvarPosNegBoolean_Create(label string, cvar *cvar_t) *MenuItem {
	menuitemcvarposnegboolean := &MenuItemCvarPosNegBoolean{}
	if MenuItem_SetLabel(&menuitemcvarposnegboolean.menuitem, label, 0) != 0 {
		menuitemcvarposnegboolean.menuitem.draw = MenuItemCvarPosNegBoolean_Draw
		menuitemcvarposnegboolean.menuitem.handlekey = MenuItemCvarPosNegBoolean_HandleKey
		menuitemcvarposnegboolean.menuitem.isselectable = 1
		menuitemcvarposnegboolean.menuitem.hidden = 0
		menuitemcvarposnegboolean.menuitem.sub = menuitemcvarposnegboolean

		menuitemcvarposnegboolean.cvar = cvar

		return &menuitemcvarposnegboolean.menuitem
	}

	return nil
}

func MenuItemCvarMultiSelect_Draw(menuitem *MenuItem, extra_x, extra_width uint) {
	var value int

	menuitemcvarmultiselect := menuitem.sub.(*MenuItemCvarMultiSelect)

	value = int(menuitemcvarmultiselect.cvar.value)
	if value < 0 || value >= menuitemcvarmultiselect.numvalues {
		value = 0
	}

	M_Print(int(extra_x+8), int(menuitem.y), menuitemcvarmultiselect.values[value])
}

func MenuItemCvarMultiSelect_HandleKey(menuitem *MenuItem, key int) {
	var value int

	menuitemcvarmultiselect := menuitem.sub.(*MenuItemCvarMultiSelect)

	value = int(menuitemcvarmultiselect.cvar.value)

	switch key {
	case int(K_ENTER), int(K_RIGHTARROW):
		S_LocalSound("misc/menu2.wav")
		value++
		if value >= menuitemcvarmultiselect.numvalues {
			value = 0
		}

		Cvar_SetValue(menuitemcvarmultiselect.cvar, float32(value))

	case int(K_LEFTARROW):
		S_LocalSound("misc/menu2.wav")
		value--
		if value < 0 {
			value = menuitemcvarmultiselect.numvalues - 1
		}

		Cvar_SetValue(menuitemcvarmultiselect.cvar, float32(value))
	}
}

func MenuItemCvarMultiSelect_Create(label string, cvar *cvar_t, values []string) *MenuItem {
	menuitemcvarmultiselect := &MenuItemCvarMultiSelect{}
	if MenuItem_SetLabel(&menuitemcvarmultiselect.menuitem, label, 0) != 0 {
		menuitemcvarmultiselect.menuitem.draw = MenuItemCvarMultiSelect_Draw
		menuitemcvarmultiselect.menuitem.handlekey = MenuItemCvarMultiSelect_HandleKey
		menuitemcvarmultiselect.menuitem.isselectable = 1
		menuitemcvarmultiselect.menuitem.hidden = 0
		menuitemcvarmultiselect.menuitem.sub = menuitemcvarmultiselect

		menuitemcvarmultiselect.cvar = cvar
		menuitemcvarmultiselect.values = values

		menuitemcvarmultiselect.numvalues = len(values)

		return &menuitemcvarmultiselect.menuitem
	}

	return nil
}

func MenuItemCvarValuePreset_Draw(menuitem *MenuItem, extra_x, extra_width uint) {
	var v, min_, max_, i int
	var slider_chars int
	var position float32
	var buf string

	m := menuitem.sub.(*MenuItemCvarValuePreset)
	v = int(m.cvar.value)
	min_ = m.values[0]
	max_ = m.values[m.numvalues-1]

	slider_chars = int(extra_width/8) - 5
	if slider_chars < 5 {
		slider_chars = 5
	}

	position = float32(bound(min_, v, max_)-min_) / float32(max_-min_)

	M_DrawCharacter(int(extra_x), int(menuitem.y), 128)
	for i = 0; i < slider_chars-2; i++ {
		M_DrawCharacter(int(extra_x)+8+i*8, int(menuitem.y), 129)
	}
	M_DrawCharacter(int(extra_x)+8+i*8, int(menuitem.y), 130)
	M_DrawCharacter(int(float32(extra_x+8)+(position*float32((slider_chars-3)*8))), int(menuitem.y), 131)

	buf = snprintf(16, "%d", v)
	M_Print(int(extra_x)+(slider_chars+1)*8, int(menuitem.y), buf)
}

func MenuItemCvarValuePreset_HandleKey(menuitem *MenuItem, key int) {
	var v, new_v int
	var i uint

	m := menuitem.sub.(*MenuItemCvarValuePreset)
	v = int(m.cvar.value)
	new_v = v

	switch key {
	case int(K_ENTER), int(K_RIGHTARROW):
		new_v = m.values[m.numvalues-1]
		for i = 0; i < m.numvalues; i++ {
			if m.values[i] > v {
				new_v = m.values[i]
				break
			}
		}

	case int(K_LEFTARROW):
		new_v = m.values[0]
		for i = m.numvalues; i > 0; i-- {
			if m.values[i-1] < v {
				new_v = m.values[i-1]
				break
			}
		}

	default:
		return
	}

	if new_v != v {
		Cvar_SetValue(m.cvar, float32(new_v))
		S_LocalSound("misc/menu3.wav")
	}
}

func MenuItemCvarValuePreset_Create(label string, cvar *cvar_t, values []int, numvalues uint) *MenuItem {
	m := &MenuItemCvarValuePreset{}
	if MenuItem_SetLabel(&m.menuitem, label, 0) != 0 {
		m.menuitem.draw = MenuItemCvarValuePreset_Draw
		m.menuitem.handlekey = MenuItemCvarValuePreset_HandleKey
		m.menuitem.isselectable = 1
		m.menuitem.hidden = 0
		m.menuitem.sub = m

		m.cvar = cvar
		m.values = values
		m.numvalues = numvalues

		return &m.menuitem
	}

	return nil
}

func MenuItemSpacer_Create() *MenuItem {
	menuitem := &MenuItem{}
	if MenuItem_SetLabel(menuitem, "", 0) != 0 {
		menuitem.draw = MenuItemButton_Draw
		menuitem.isselectable = 0
		menuitem.hidden = 0

		return menuitem
	}

	return nil
}

//=============================================================================
/* MAIN MENU */

var m_main_cursor int

const MAIN_ITEMS = 4

func M_Menu_Main_f() {
	M_EnterMenu(m_main)
}

func M_Main_Draw() {
	var f int

	M_DrawPic(qplaquepic, 16, 4, 32, 144)
	M_DrawPic(ttl_mainpic, (320-96)/2, 4, 96, 24)
	M_DrawPic(mainmenupic, 72, 32, 240, 112)

	f = int(curtime*10) % 6
	M_DrawPic(menudotpic[f], 54, 32+m_main_cursor*20, 16, 24)
}

func M_Main_Key(key int) {
	switch key {
	case int(K_ESCAPE):
		key_dest = key_game
		m_state = m_none
		VID_SetMouseGrab(1)

	case int(K_UPARROW):
		S_LocalSound("misc/menu1.wav")
		m_main_cursor--
		if m_main_cursor < 0 {
			m_main_cursor = MAIN_ITEMS - 1
		}

	case int(K_DOWNARROW):
		S_LocalSound("misc/menu1.wav")
		m_main_cursor++
		if m_main_cursor >= MAIN_ITEMS {
			m_main_cursor = 0
		}

	case int(K_HOME), int(K_PGUP):
		S_LocalSound("misc/menu1.wav")
		m_main_cursor = 0

	case int(K_END), int(K_PGDN):
		S_LocalSound("misc/menu1.wav")
		m_main_cursor = MAIN_ITEMS - 1

	case int(K_ENTER):
		m_entersound = true

		switch m_main_cursor {
		case 0:
			M_Menu_SinglePlayer_f()

		case 1:
			M_Menu_MultiPlayer_f()

		case 2:
			M_Menu_Options_f()

		case 3:
			M_Menu_Quit_f()
		}
	}
}

//=============================================================================
/* OPTIONS MENU */

func M_Menu_Options_f() {
	M_EnterMenu(m_options)
}

func M_Options_GoToConsole() {
	m_state = m_none
	key_dest = key_console
}

func M_Options_ResetToDefaults() {
	Cbuf_AddText("exec default.cfg\n")
}

func M_Options_SaveConfiguration() {
	Cbuf_AddText("cfg_save default.cfg\n")
}

var m_options_fs = 2

func M_Options_Draw() {
	var newfs int

	M_DrawPic(qplaquepic, 16, 4, 32, 144)
	M_DrawPic(p_optionpic, (320-144)/2, 4, 144, 24)

	newfs = b2i(VID_GetFullscreen())
	if newfs != m_options_fs {
		if newfs != 0 {
			Menu_HideItem(optionsmenu, optionsmenu_usemouse)
		} else {
			Menu_ShowItem(optionsmenu, optionsmenu_usemouse)
		}

		m_options_fs = newfs
	}

	Menu_Draw(optionsmenu)
}

//=============================================================================
/* KEYS MENU */

// an empty command is the NULL separator row of C
var bindnames = [][2]string{
	{"+forward", "move forward"},
	{"+back", "move back"},
	{"+moveleft", "move left"},
	{"+moveright", "move right"},
	{"+jump", "jump"},
	{"", ""},
	{"+weapon_rocket", "rocket launcher"},
	{"+weapon_thunderbolt", "thunderbolt"},
	{"+weapon_grenade", "grenade launcher"},
	{"", ""},
	{"+weapon_axe", "axe"},
	{"+weapon_shotgun", "shotgun"},
	{"+weapon_nailgun", "nailgun"},
	{"", ""},
	{"+moveup", "swim up"},
	{"+movedown", "swim down"},
	{"", ""},
	{"+weapon_hook", "CTF grapple hook"},
	{"tossflag", "CTF toss the flag"},
	{"tossrune", "CTF toss the rune"},
	{"", ""},
	{"impulse 12", "noob: prev weapon"},
	{"impulse 10", "noob: next weapon"},
	{"+attack", "noob: attack"},
	{"", ""},
	{"+use", "use"},
	{"messagemode", "say"},
}

var NUMCOMMANDS = len(bindnames)

var keys_cursor int
var bind_grab bool

func M_Menu_Keys_f() {
	M_EnterMenu(m_keys)
}

func M_FindKeysForCommand(command string, twokeys *[2]int) {
	var count, j int
	var b *string

	twokeys[0] = -1
	twokeys[1] = -1
	count = 0

	for j = 0; j < 256; j++ {
		b = keybindings[j]
		if b == nil {
			continue
		}
		if *b == command {
			if count != 0 {
				if j == twokeys[0]+1 && (twokeys[0] == int(K_LCTRL) || twokeys[0] == int(K_LSHIFT) || twokeys[0] == int(K_LALT)) {

					twokeys[0]--
					continue
				}
			}
			twokeys[count] = j
			count++
			if count == 2 {

				if Key_IsLeftRightSameBind(twokeys[1]) {
					twokeys[1]++
				}
				break
			}
		}
	}
}

func M_UnbindCommand(command string) {
	var j int
	var b *string

	for j = 0; j < 256; j++ {
		b = keybindings[j]
		if b == nil {
			continue
		}
		if *b == command {
			Key_Unbind(j)
		}
	}
}

// the keys and team comms screens share their drawing and key handling
func menuBindingsDraw(names [][2]string, cursor int, grab bool) {
	var x, y, i int
	var keys [2]int
	var name string

	M_DrawPic(ttl_cstmpic, (320-184)/2, 4, 184, 24)

	if grab {
		M_Print(12, 32, "Press a key or button for this action")
	} else {
		M_Print(18, 32, "Enter to change, del to clear")
	}

	// search for known bindings
	for i = 0; i < len(names); i++ {
		y = 48 + 8*i

		if names[i][0] == "" {
			continue
		}

		M_Print(16, y, names[i][1])

		M_FindKeysForCommand(names[i][0], &keys)

		if keys[0] == -1 {
			M_Print(172, y, "???")
		} else {
			name = Key_KeynumToString(keys[0])
			M_Print(172, y, name)
			x = len(name) * 8
			if keys[1] != -1 {
				M_Print(172+x+8, y, "or")
				M_Print(172+x+32, y, Key_KeynumToString(keys[1]))
			}
		}
	}

	if grab {
		M_DrawCharacter(158, 48+cursor*8, '=')
	} else {
		M_DrawCharacter(158, 48+cursor*8, 12+(int(curtime*4)&1))
	}
}

func menuBindingsKey(names [][2]string, cursor *int, grab *bool, k int) {
	var keys [2]int

	if *grab {
		// defining a key
		S_LocalSound("misc/menu1.wav")
		if k == int(K_ESCAPE) {
			*grab = false
		} else if k != '`' {
			Key_SetBinding(k, names[*cursor][0])
		}

		*grab = false
		return
	}

	switch k {
	case int(K_BACKSPACE), int(K_ESCAPE):
		if k == int(K_BACKSPACE) {
			m_topmenu = m_none
		}
		M_LeaveMenu(m_options)

	case int(K_LEFTARROW), int(K_UPARROW):
		S_LocalSound("misc/menu1.wav")
		for {
			*cursor--
			if *cursor < 0 {
				*cursor = len(names) - 1
			}
			if names[*cursor][0] != "" {
				break
			}
		}

	case int(K_DOWNARROW), int(K_RIGHTARROW):
		S_LocalSound("misc/menu1.wav")
		for {
			*cursor++
			if *cursor >= len(names) {
				*cursor = 0
			}
			if names[*cursor][0] != "" {
				break
			}
		}

	case int(K_HOME), int(K_PGUP):
		S_LocalSound("misc/menu1.wav")
		*cursor = 0
		for names[*cursor][0] == "" {
			*cursor++
		}

	case int(K_END), int(K_PGDN):
		S_LocalSound("misc/menu1.wav")
		*cursor = len(names) - 1
		for names[*cursor][0] == "" {
			*cursor--
		}

	case int(K_ENTER): // go into bind mode
		M_FindKeysForCommand(names[*cursor][0], &keys)
		S_LocalSound("misc/menu2.wav")
		if keys[1] != -1 {
			M_UnbindCommand(names[*cursor][0])
		}
		*grab = true

	case int(K_DEL): // delete bindings
		S_LocalSound("misc/menu2.wav")
		M_UnbindCommand(names[*cursor][0])
	}
}

func M_Keys_Draw() {
	menuBindingsDraw(bindnames, keys_cursor, bind_grab)
}

func M_Keys_Key(k int) {
	menuBindingsKey(bindnames, &keys_cursor, &bind_grab, k)
}

//=============================================================================
/* TEAM COMMUNICATION MENU */

var teamcomms_bindnames = [][2]string{
	{"tp_msgreport", "report"},
	{"tp_msgpoint", "point at location"},
	{"tp_msgtook", "took item"},
	{"tp_msgreplace", "replace"},
	{"", ""},
	{"tp_msgquaddead", "quad dead"},
	{"tp_msglost", "lost location"},
	{"tp_msgneed", "need item"},
	{"tp_msgkillme", "kill me"},
	{"", ""},
	{"tp_msggetquad", "get quad"},
	{"tp_msggetpent", "get pent"},
	{"tp_msgenemypwr", "enemy quad"},
	{"tp_msgteampwr", "team power"},
	{"", ""},
	{"tp_msgyesok", "yes/ok"},
	{"tp_msgnocancel", "no/cancel"},
	{"tp_msghelp", "help"},
	{"tp_msgcoming", "coming"},
	{"tp_msgwaiting", "waiting"},
	{"tp_msgsafe", "safe"},
	{"tp_msgslipped", "enemy slipped"},
	{"tp_msgtrick", "trick"},
	{"tp_msgutake", "you take"},
	{"", ""},
	{"messagemode2", "say (team)"},
}

var NUMTEAMCOMMS = len(teamcomms_bindnames)

var teamcomms_cursor int
var teamcomms_grab bool

func M_Menu_TeamComms_f() {
	M_EnterMenu(m_teamcomms)
}

func M_TeamComms_Draw() {
	menuBindingsDraw(teamcomms_bindnames, teamcomms_cursor, teamcomms_grab)
}

func M_TeamComms_Key(k int) {
	menuBindingsKey(teamcomms_bindnames, &teamcomms_cursor, &teamcomms_grab, k)
}

//=============================================================================
/* FPS SETTINGS MENU */

func M_Menu_Fps_f() {
	M_EnterMenu(m_fps)
}

var explosiontypevalues = []string{
	"fire + sparks",
	"fire only",
	"teleport",
	"blood",
	"big blood",
	"dbl gunshot",
	"blob effect",
	"big explosion",
}

var muzzleflashvalues = []string{
	"off",
	"on",
	"own off",
}

var deadbodyfiltervalues = []string{
	"off",
	"on (normal)",
	"on (instant)",
}

var rocket2grenadevalues = []string{
	"normal",
	"grenade",
}

var rockettrailvalues = []string{
	"off",
	"normal",
	"grenade",
	"alt normal",
	"slight blood",
	"big blood",
	"tracer 1",
	"tracer 2",
}

var powerupglowvalues = []string{
	"off",
	"on",
	"own off",
}

func M_Fps_FastMode() {
	Cvar_SetValue(&r_explosiontype, 1)
	Cvar_SetValue(&r_explosionlight, 0)
	Cvar_SetValue(&cl_muzzleflash, 0)
	Cvar_SetValue(&cl_gibfilter, 1)
	Cvar_SetValue(&cl_deadbodyfilter, 1)
	Cvar_SetValue(&r_rocketlight, 0)
	Cvar_SetValue(&r_powerupglow, 0)
	Cvar_SetValue(&r_drawflame, 0)
	Cvar_SetValue(&r_fastsky, 1)
	Cvar_SetValue(&r_rockettrail, 1)
	Cvar_SetValue(&v_damagecshift, 0)
	Cvar_SetValue(&v_bonusflash, 0)
	Cvar_SetValue(&gl_flashblend, 1)
	Cvar_SetValue(&r_dynamic, 0)
	Cvar_SetValue(&gl_part_explosions, 0)
	Cvar_SetValue(&gl_part_trails, 0)
	Cvar_SetValue(&gl_part_spikes, 0)
	Cvar_SetValue(&gl_part_gunshots, 0)
	Cvar_SetValue(&gl_part_blood, 0)
	Cvar_SetValue(&gl_part_telesplash, 0)
	Cvar_SetValue(&gl_part_blobs, 0)
	Cvar_SetValue(&gl_part_lavasplash, 0)
	Cvar_SetValue(&gl_part_inferno, 0)
}

func M_Fps_HighQuality() {
	Cvar_SetValue(&r_explosiontype, 0)
	Cvar_SetValue(&r_explosionlight, 1)
	Cvar_SetValue(&cl_muzzleflash, 1)
	Cvar_SetValue(&cl_gibfilter, 0)
	Cvar_SetValue(&cl_deadbodyfilter, 0)
	Cvar_SetValue(&r_rocketlight, 1)
	Cvar_SetValue(&r_powerupglow, 2)
	Cvar_SetValue(&r_drawflame, 1)
	Cvar_SetValue(&r_fastsky, 0)
	Cvar_SetValue(&r_rockettrail, 1)
	Cvar_SetValue(&v_damagecshift, 1)
	Cvar_SetValue(&v_bonusflash, 1)
	Cvar_SetValue(&gl_flashblend, 0)
	Cvar_SetValue(&r_dynamic, 1)
	Cvar_SetValue(&gl_part_explosions, 1)
	Cvar_SetValue(&gl_part_trails, 1)
	Cvar_SetValue(&gl_part_spikes, 1)
	Cvar_SetValue(&gl_part_gunshots, 1)
	Cvar_SetValue(&gl_part_blood, 1)
	Cvar_SetValue(&gl_part_telesplash, 1)
	Cvar_SetValue(&gl_part_blobs, 1)
	Cvar_SetValue(&gl_part_lavasplash, 1)
	Cvar_SetValue(&gl_part_inferno, 1)
}

func M_Fps_Draw() {
	M_DrawPic(qplaquepic, 16, 4, 32, 144)
	M_DrawPic(ttl_cstmpic, (320-184)/2, 4, 184, 24)

	Menu_Draw(fpsmenu)
}

//=============================================================================
/* VIDEO MENU */

var video_verify_oldcvar_width string
var video_verify_oldcvar_height string
var video_verify_oldcvar_fullscreen string
var video_verify_oldcvar_mode string

var video_verify_oldactive_width uint
var video_verify_oldactive_height uint
var video_verify_oldactive_fullscreen bool
var video_verify_oldactive_mode string

var video_selectedmodeline string // "" is C's null pointer

var video_verify_fail_time float64

func M_Menu_Video_Verify_Revert() {
	Cvar_SetValue(&vid_width, float32(video_verify_oldactive_width))
	Cvar_SetValue(&vid_height, float32(video_verify_oldactive_height))
	Cvar_SetValue(&vid_fullscreen, float32(b2i(video_verify_oldactive_fullscreen)))
	Cvar_Set(&vid_mode, video_verify_oldactive_mode)
	VID_Restart()
	Cvar_Set(&vid_width, video_verify_oldcvar_width)
	Cvar_Set(&vid_height, video_verify_oldcvar_height)
	Cvar_Set(&vid_fullscreen, video_verify_oldcvar_fullscreen)
	Cvar_Set(&vid_mode, video_verify_oldcvar_mode)
}

func M_Menu_Video_Verify_Cleanup() {
	video_verify_oldcvar_width = ""
	video_verify_oldcvar_height = ""
	video_verify_oldcvar_fullscreen = ""
	video_verify_oldcvar_mode = ""
	video_verify_oldactive_mode = ""
}

func M_Menu_Video_Verify_f() {
	video_verify_oldcvar_width = vid_width.string
	video_verify_oldcvar_height = vid_height.string
	video_verify_oldcvar_fullscreen = vid_fullscreen.string
	video_verify_oldcvar_mode = vid_mode.string

	video_verify_oldactive_width = VID_GetWidth()
	video_verify_oldactive_height = VID_GetHeight()
	video_verify_oldactive_fullscreen = VID_GetFullscreen()
	video_verify_oldactive_mode = VID_GetMode()

	Cvar_Set(&vid_mode, video_selectedmodeline)
	Cvar_SetValue(&vid_fullscreen, 1)
	VID_Restart()

	video_verify_fail_time = curtime + 15

	M_EnterMenu(m_video_verify)
}

func M_Video_Verify_Draw() {
	var timeremaining int
	var buf string
	var middle uint

	if scr_centerMenu.value != 0 {
		middle = 100
	} else {
		middle = 36 + 8
	}

	M_Print(160-(32*4), int(middle)-36, "Testing your selected video mode")
	M_Print(160-(38*4), int(middle)-28, "If you can see this text, then press y")
	M_Print(160-(36*4), int(middle)-20, "to verify that this video mode works")

	timeremaining = int(video_verify_fail_time - curtime)
	if timeremaining < 0 {
		timeremaining = 0
	}

	buf = snprintf(40, "Reverting in %d seconds", timeremaining)

	M_PrintWhite(160-(len(buf)*4), int(middle)-4, buf)

	M_Print(160-(35*4), int(middle)+12, "Press escape or n to cancel setting")
	M_Print(160-(15*4), int(middle)+20, "this video mode")

}

func M_Video_Verify_Key(key int) {
	switch key {
	case int(K_BACKSPACE), int(K_ESCAPE), 'n':
		M_Menu_Video_Verify_Revert()
		M_Menu_Video_Verify_Cleanup()
		M_LeaveMenu(m_video)

	case 'y':
		M_Menu_Video_Verify_Cleanup()
		M_LeaveMenu(m_video)
	}
}

var windowedresolutions = [...]struct {
	width  uint
	height uint
}{
	{320, 200},
	{320, 240},
	{512, 384},
	{640, 400},
	{640, 480},
	{800, 600},
	{1024, 768},
	{1280, 800},
	{1280, 1024},
	{1440, 900},
	{1600, 1200},
	{1680, 1050},
	{1920, 1080},
	{2560, 1440},
}

var windowedresolutionnames = [...]string{
	"320x200",
	"320x240",
	"512x384",
	"640x400",
	"640x480",
	"800x600",
	"1024x768",
	"1280x800",
	"1280x1024",
	"1440x900",
	"1600x1200",
	"1680x1050",
	"1920x1080",
	"2560x1440",
}

const NUMWINDOWEDRESOLUTIONS = uint(len(windowedresolutions))

var video_fullscreenmodecursor uint
var video_fullscreenmodelistbegin uint
var video_windowmodecursor uint
var video_windowmodelistbegin uint
var video_typenum uint

func M_Menu_Video_f() {
	M_EnterMenu(m_video)

	if vid_fullscreen.value != 0 {
		if vid_mode.string != "" {
			video_typenum = 2
		} else {
			video_typenum = 1
		}
	} else {
		video_typenum = 0
	}

	video_fullscreenmodelistbegin = 0
	video_windowmodelistbegin = 0
}

var video_displaytypes = [...]string{
	"Windowed",
	"Fullscreen, native desktop",
	"Fullscreen, custom mode",
}

func M_Video_Draw() {
	var vidmodes []string
	var t string
	var modestring string
	var i uint
	var j uint
	var maxwidth uint
	var bottom uint
	var modelines uint
	var curmode string

	if scr_centerMenu.value != 0 {
		bottom = 200
	} else {
		bottom = uint(menuheight)
	}

	M_DrawPic(vidmodespic, (320-216)/2, 4, 216, 24)

	curmode = VID_GetMode()

	M_Print(160-(6*8)-4, 36, "Current mode:")
	if VID_GetFullscreen() {
		modestring = fmt.Sprintf("Fullscreen, %dx%d", VID_GetWidth(), VID_GetHeight())

		if curmode != "" {
			t = Sys_Video_GetModeDescription(curmode)
			if t != "" {
				modestring = strlcpy(t, 40)
			}
		}
	} else {
		modestring = fmt.Sprintf("Windowed, %dx%d", VID_GetWidth(), VID_GetHeight())
	}

	M_Print(160-(len(modestring)*4), 44, modestring)

	M_Print(160-(6*8)-4, 60, "Display type:")
	M_Print(160-((len(video_displaytypes[video_typenum])*8)/2), 68, video_displaytypes[video_typenum])

	modelines = (bottom - 32 - 92) / 8

	if video_typenum == 0 {
		if video_windowmodecursor >= NUMWINDOWEDRESOLUTIONS {
			video_windowmodecursor = NUMWINDOWEDRESOLUTIONS - 1
		}

		if video_windowmodecursor < video_windowmodelistbegin {
			video_windowmodelistbegin = video_windowmodecursor
		} else if video_windowmodecursor >= modelines+video_windowmodelistbegin {
			video_windowmodelistbegin = video_windowmodecursor - modelines + 1
		}

		maxwidth = 0 /* 16 */
		for i = 0; i < NUMWINDOWEDRESOLUTIONS; i++ {
			if uint(len(windowedresolutionnames[i])) > maxwidth {
				maxwidth = uint(len(windowedresolutionnames[i]))
			}
		}

		M_Print(160-((16*8)/2), 84, "Available modes:")
		for i, j = video_windowmodelistbegin, 0; i < NUMWINDOWEDRESOLUTIONS && j < modelines; i, j = i+1, j+1 {
			if !VID_GetFullscreen() && VID_GetWidth() == windowedresolutions[i].width && VID_GetHeight() == windowedresolutions[i].height {
				M_PrintWhite(160-int((maxwidth*8)/2), 92+int(j)*8, windowedresolutionnames[i])
			} else {
				M_Print(160-int((maxwidth*8)/2), 92+int(j)*8, windowedresolutionnames[i])
			}
		}

		M_DrawCharacter(160-int((maxwidth*8)/2)-16, 92+int(video_windowmodecursor-video_windowmodelistbegin)*8, 12+(int(curtime*4)&1))
	} else if video_typenum == 2 {
		vidmodes = Sys_Video_GetModeList()
		if vidmodes != nil {
			maxwidth = 16
			for i = 0; i < uint(len(vidmodes)); i++ {
				t = Sys_Video_GetModeDescription(vidmodes[i])
				if t != "" {
					if uint(len(t)) > maxwidth {
						maxwidth = uint(len(t))
					}
				}
			}

			if video_fullscreenmodecursor >= i {
				if i > 0 {
					video_fullscreenmodecursor = i - 1
				} else {
					video_fullscreenmodecursor = 0
				}
			}

			if video_fullscreenmodecursor < video_fullscreenmodelistbegin {
				video_fullscreenmodelistbegin = video_fullscreenmodecursor
			} else if video_fullscreenmodecursor >= modelines+video_fullscreenmodelistbegin {
				video_fullscreenmodelistbegin = video_fullscreenmodecursor - modelines + 1
			}

			if maxwidth > uint((menuwidth/8)-4) {
				maxwidth = uint((menuwidth / 8) - 4)
			}

			M_Print(160-int((maxwidth*8)/2), 84, "Available modes:")
			for i, j = video_fullscreenmodelistbegin, 0; i < uint(len(vidmodes)) && j < modelines; i, j = i+1, j+1 {
				t = Sys_Video_GetModeDescription(vidmodes[i])
				if t != "" {
					if curmode == vidmodes[i] {
						M_PrintWhite(160-int((maxwidth*8)/2), 92+int(j)*8, t)
					} else {
						M_Print(160-int((maxwidth*8)/2), 92+int(j)*8, t)
					}
				} else {
					M_Print(160-int((maxwidth*8)/2), 92+int(j)*8, "Unknown")
				}
			}

			video_selectedmodeline = vidmodes[video_fullscreenmodecursor]

			Sys_Video_FreeModeList(vidmodes)

			if video_fullscreenmodecursor < video_fullscreenmodelistbegin {
				video_fullscreenmodelistbegin = video_fullscreenmodecursor
			} else if video_fullscreenmodecursor >= modelines+video_fullscreenmodelistbegin {
				video_fullscreenmodelistbegin = video_fullscreenmodecursor - modelines + 1
			}

			M_DrawCharacter(160-int((maxwidth*8)/2)-16, 92+int(video_fullscreenmodecursor-video_fullscreenmodelistbegin)*8, 12+(int(curtime*4)&1))
		}
	}

	M_Print(160-(37*4), int(bottom)-24, "Left/right arrows change display type")
	M_Print(160-(35*4), int(bottom)-16, "Up/down arrows scroll the mode list")
	M_Print(160-(20*4), int(bottom)-8, "Enter selects a mode")
}

func M_Video_Key(key int) {
	switch key {
	case int(K_BACKSPACE), int(K_ESCAPE):
		if key == int(K_BACKSPACE) {
			m_topmenu = m_none
		}
		video_selectedmodeline = ""
		M_LeaveMenu(m_options)

	case int(K_UPARROW):
		S_LocalSound("misc/menu1.wav")

		if video_typenum == 0 {
			if video_windowmodecursor != 0 {
				video_windowmodecursor--
			}
		} else if video_typenum == 2 {
			if video_fullscreenmodecursor != 0 {
				video_fullscreenmodecursor--
			}
		}
	case int(K_DOWNARROW):
		S_LocalSound("misc/menu1.wav")

		if video_typenum == 0 && video_windowmodecursor < NUMWINDOWEDRESOLUTIONS-1 {
			video_windowmodecursor++
		} else if video_typenum == 2 {
			video_fullscreenmodecursor++
		}

	case int(K_LEFTARROW):
		S_LocalSound("misc/menu1.wav")

		if video_typenum != 0 {
			video_typenum--
		} else {
			video_typenum = 1
		}

	case int(K_RIGHTARROW):
		S_LocalSound("misc/menu1.wav")

		video_typenum++
		if video_typenum > 1 {
			video_typenum = 0
		}

	case int(K_ENTER):
		m_entersound = true

		if video_typenum == 0 {
			if !(!VID_GetFullscreen() && VID_GetWidth() == windowedresolutions[video_windowmodecursor].width && VID_GetHeight() == windowedresolutions[video_windowmodecursor].height) {
				Cvar_SetValue(&vid_width, float32(windowedresolutions[video_windowmodecursor].width))
				Cvar_SetValue(&vid_height, float32(windowedresolutions[video_windowmodecursor].height))
				Cvar_SetValue(&vid_fullscreen, 0)
				Cbuf_AddText("vid_restart\n")
			}
		} else if video_typenum == 1 {
			Cvar_SetValue(&vid_fullscreen, 1)
			Cvar_Set(&vid_mode, "")
			Cbuf_AddText("vid_restart\n")
		} else if video_typenum == 2 && video_selectedmodeline != "" {
			M_Menu_Video_Verify_f()
		}
	}
}

//=============================================================================
/* HELP MENU */

var help_page int

func M_Menu_Help_f() {
	M_EnterMenu(m_help)
	help_page = 0
}

func M_Help_Draw() {
	M_DrawPic(helppic[help_page], 0, 0, 320, 200)
}

func M_Help_Key(key int) {
	switch key {
	case int(K_BACKSPACE), int(K_ESCAPE):
		if key == int(K_BACKSPACE) {
			m_topmenu = m_none
		}
		M_LeaveMenu(m_main)

	case int(K_UPARROW), int(K_RIGHTARROW):
		m_entersound = true
		help_page++
		if help_page >= NUM_HELP_PAGES {
			help_page = 0
		}

	case int(K_DOWNARROW), int(K_LEFTARROW):
		m_entersound = true
		help_page--
		if help_page < 0 {
			help_page = NUM_HELP_PAGES - 1
		}
	}

}

//=============================================================================
/* QUIT MENU */

var m_quit_prevstate menustates
var wasInMenus bool

func M_Menu_Quit_f() {
	if cl_confirmquit.value != 0 {
		if m_state == m_quit {
			return
		}
		wasInMenus = (key_dest == key_menu)
		m_quit_prevstate = m_state
		rand() // keeps the rand() sequence of C, which picks a quit message here
		M_EnterMenu(m_quit)
	} else {
		Host_Quit()
	}
}

func M_Quit_Key(key int) {
	switch key {
	case int(K_ESCAPE), 'n', 'N':
		if wasInMenus {
			m_state = m_quit_prevstate
			m_entersound = true
		} else {
			key_dest = key_game
			m_state = m_none
			VID_SetMouseGrab(1)
		}

	case int(K_ENTER), 'Y', 'y':
		key_dest = key_console
		Host_Quit()

	default:
	}
}

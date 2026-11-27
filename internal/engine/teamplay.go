// Copyright (C) 2000-2003       Anton Gavrilov, A Nourai
// Copyright (C) 2005-2012 Mark Olsen
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

// teamplay.c, first half: triggers, macros, macro and fun char parsing, skin
// and colour forcing, loc files. The rest is in teamplay_items.go.

import (
	"fmt"
	"strings"
	"time"
)

var cl_parseSay = cvar_t{name: "cl_parseSay", string: "1"}
var cl_parseFunChars = cvar_t{name: "cl_parseFunChars", string: "1"}
var tp_triggers = cvar_t{name: "tp_triggers", string: "1"}
var tp_msgtriggers = cvar_t{name: "tp_msgtriggers", string: "1"}
var tp_forceTriggers = cvar_t{name: "tp_forceTriggers", string: "0"}
var cl_nofake = cvar_t{name: "cl_nofake", string: "0"}
var tp_loadlocs = cvar_t{name: "tp_loadlocs", string: "1"}
var name_short = cvar_t{name: "name_short", string: "unn", flags: CVAR_ARCHIVE}

var cl_teamskin = cvar_t{name: "teamskin", string: ""}
var cl_enemyskin = cvar_t{name: "enemyskin", string: ""}
var cl_teamquadskin = cvar_t{name: "teamquadskin", string: ""}
var cl_enemyquadskin = cvar_t{name: "enemyquadskin", string: ""}
var cl_teampentskin = cvar_t{name: "teampentskin", string: ""}
var cl_enemypentskin = cvar_t{name: "enemypentskin", string: ""}
var cl_teambothskin = cvar_t{name: "teambothskin", string: ""}
var cl_enemybothskin = cvar_t{name: "enemybothskin", string: ""}

// the callback reads these cvars back, which Go rejects as an initialisation cycle in the literals
func init() {
	for _, v := range []*cvar_t{&cl_teamskin, &cl_enemyskin, &cl_teamquadskin, &cl_enemyquadskin, &cl_teampentskin, &cl_enemypentskin, &cl_teambothskin, &cl_enemybothskin} {
		v.PostChangeCallback = OnChangeSkinForcing
	}
}

var tp_soundtrigger = cvar_t{name: "tp_soundtrigger", string: "~"}

var tp_name_axe = cvar_t{name: "tp_name_axe", string: "axe"}
var tp_name_sg = cvar_t{name: "tp_name_sg", string: "sg"}
var tp_name_ssg = cvar_t{name: "tp_name_ssg", string: "ssg"}
var tp_name_ng = cvar_t{name: "tp_name_ng", string: "ng"}
var tp_name_sng = cvar_t{name: "tp_name_sng", string: "sng"}
var tp_name_gl = cvar_t{name: "tp_name_gl", string: "gl"}
var tp_name_rl = cvar_t{name: "tp_name_rl", string: "rl"}
var tp_name_lg = cvar_t{name: "tp_name_lg", string: "lg"}
var tp_name_armortype_ra = cvar_t{name: "tp_name_armortype_ra", string: "r"}
var tp_name_armortype_ya = cvar_t{name: "tp_name_armortype_ya", string: "y"}
var tp_name_armortype_ga = cvar_t{name: "tp_name_armortype_ga", string: "g"}
var tp_name_ra = cvar_t{name: "tp_name_ra", string: "ra"}
var tp_name_ya = cvar_t{name: "tp_name_ya", string: "ya"}
var tp_name_ga = cvar_t{name: "tp_name_ga", string: "ga"}
var tp_name_quad = cvar_t{name: "tp_name_quad", string: "quad"}
var tp_name_pent = cvar_t{name: "tp_name_pent", string: "pent"}
var tp_name_ring = cvar_t{name: "tp_name_ring", string: "ring"}
var tp_name_suit = cvar_t{name: "tp_name_suit", string: "suit"}
var tp_name_shells = cvar_t{name: "tp_name_shells", string: "shells"}
var tp_name_nails = cvar_t{name: "tp_name_nails", string: "nails"}
var tp_name_rockets = cvar_t{name: "tp_name_rockets", string: "rockets"}
var tp_name_cells = cvar_t{name: "tp_name_cells", string: "cells"}
var tp_name_mh = cvar_t{name: "tp_name_mh", string: "mega"}
var tp_name_health = cvar_t{name: "tp_name_health", string: "health"}
var tp_name_armor = cvar_t{name: "tp_name_armor", string: "armor"}
var tp_name_weapon = cvar_t{name: "tp_name_weapon", string: "weapon"}
var tp_name_backpack = cvar_t{name: "tp_name_backpack", string: "pack"}
var tp_name_flag = cvar_t{name: "tp_name_flag", string: "flag"}
var tp_name_sentry = cvar_t{name: "tp_name_sentry", string: "sentry gun"}
var tp_name_disp = cvar_t{name: "tp_name_disp", string: "dispenser"}
var tp_name_rune1 = cvar_t{name: "tp_name_rune1", string: "resistance rune"}
var tp_name_rune2 = cvar_t{name: "tp_name_rune2", string: "strength rune"}
var tp_name_rune3 = cvar_t{name: "tp_name_rune3", string: "haste rune"}
var tp_name_rune4 = cvar_t{name: "tp_name_rune4", string: "regeneration rune"}
var tp_name_teammate = cvar_t{name: "tp_name_teammate", string: ""}
var tp_name_enemy = cvar_t{name: "tp_name_enemy", string: "enemy"}
var tp_name_eyes = cvar_t{name: "tp_name_eyes", string: "eyes"}
var tp_name_quaded = cvar_t{name: "tp_name_quaded", string: "quaded"}
var tp_name_pented = cvar_t{name: "tp_name_pented", string: "pented"}
var tp_name_nothing = cvar_t{name: "tp_name_nothing", string: "nothing"}
var tp_name_someplace = cvar_t{name: "tp_name_someplace", string: "someplace"}
var tp_name_at = cvar_t{name: "tp_name_at", string: "at"}
var tp_name_none = cvar_t{name: "tp_name_none", string: ""}
var tp_name_separator = cvar_t{name: "tp_name_separator", string: "/"}
var tp_weapon_order = cvar_t{name: "tp_weapon_order", string: "78654321"}

var tp_name_status_green = cvar_t{name: "tp_name_status_green", string: "$G"}
var tp_name_status_yellow = cvar_t{name: "tp_name_status_yellow", string: "$Y"}
var tp_name_status_red = cvar_t{name: "tp_name_status_red", string: "$R"}
var tp_name_status_blue = cvar_t{name: "tp_name_status_blue", string: "$B"}

var tp_need_ra = cvar_t{name: "tp_need_ra", string: "50"}
var tp_need_ya = cvar_t{name: "tp_need_ya", string: "50"}
var tp_need_ga = cvar_t{name: "tp_need_ga", string: "50"}
var tp_need_health = cvar_t{name: "tp_need_health", string: "50"}
var tp_need_weapon = cvar_t{name: "tp_need_weapon", string: "35687"}
var tp_need_rl = cvar_t{name: "tp_need_rl", string: "1"}
var tp_need_rockets = cvar_t{name: "tp_need_rockets", string: "5"}
var tp_need_cells = cvar_t{name: "tp_need_cells", string: "20"}
var tp_need_nails = cvar_t{name: "tp_need_nails", string: "40"}
var tp_need_shells = cvar_t{name: "tp_need_shells", string: "10"}

var suppress bool

const (
	POINT_TYPE_ITEM     = 1
	POINT_TYPE_POWERUP  = 2
	POINT_TYPE_TEAMMATE = 3
	POINT_TYPE_ENEMY    = 4
)

const TP_TOOK_EXPIRE_TIME = 15
const TP_POINT_EXPIRE_TIME = TP_TOOK_EXPIRE_TIME

const MAX_LOC_NAME = 64

// this structure is cleared after entering a new map
type tvars_t struct {
	health            int
	items             int
	olditems          int
	activeweapon      int
	stat_framecounts  [MAX_CL_STATS]int
	deathtrigger_time float64
	lastdeathloc      string // MAX_LOC_NAME
	tookname          string // 32
	tookloc           string // MAX_LOC_NAME
	tooktime          float64
	pointtime         float64 // cls.realtime for which pointitem & pointloc are valid
	pointname         string  // 32
	pointloc          string  // MAX_LOC_NAME
	pointtype         int
	nearestitemloc    string // MAX_LOC_NAME
	lastreportedloc   string // MAX_LOC_NAME
	lastdrop_time     float64
	lastdroploc       string // MAX_LOC_NAME
	lasttrigger_match string // 256

	numenemies         int
	numfriendlies      int
	last_numenemies    int
	last_numfriendlies int

	enemy_powerups      int
	enemy_powerups_time float64
}

var vars tvars_t

/********************************** TRIGGERS **********************************/

type f_trigger_t struct {
	name     string
	teamplay bool
}

var f_triggers = []f_trigger_t{
	{"f_newmap", false},
	{"f_spawn", false},
	{"f_mapend", false},
	{"f_reloadstart", false},
	{"f_reloadend", false},

	{"f_weaponchange", false},

	{"f_took", true},
	{"f_respawn", true},
	{"f_death", true},
	{"f_flagdeath", true},
}

var num_f_triggers = len(f_triggers)

func TP_ExecTrigger(trigger string) {
	var i, j, numteammates int
	var alias *cmd_alias_t

	if tp_triggers.value == 0 || cls.demoplayback || cl.spectator != 0 {
		return
	}

	for i = 0; i < num_f_triggers; i++ {
		if f_triggers[i].name == trigger {
			break
		}
	}
	if i == num_f_triggers {
		Sys_Error("Unknown f_trigger \"%s\"", trigger)
	}

	if f_triggers[i].teamplay && tp_forceTriggers.value == 0 {
		if cl.teamplay == 0 {
			return
		}

		numteammates = 0
		for j = 0; j < MAX_CLIENTS; j++ {
			if cl.players[j].name != "" && cl.players[j].spectator == 0 && j != cl.playernum {
				if cl.players[j].team == cl.players[cl.playernum].team {
					numteammates++
				}
			}
		}

		if numteammates == 0 {
			return
		}
	}

	if alias = Cmd_FindAlias(trigger); alias != nil {
		if Ruleset_AllowFTrigger(trigger) {
			Cbuf_AddTextEx(cbuf_main, va("%s\n", alias.value))
		}
	}
}

/*********************************** MACROS ***********************************/

const MAX_MACRO_VALUE = 256

var macro_buf = ""

// the "separator, then the name" step the item macros repeat
func macroAppend(name string) {
	if macro_buf != "" {
		macro_buf = strlcat(macro_buf, tp_name_separator.string, MAX_MACRO_VALUE)
	}
	macro_buf = strlcat(macro_buf, name, MAX_MACRO_VALUE)
}

func Macro_Quote_f() string {
	return "\""
}

func Macro_Latency() string {
	macro_buf = snprintf(MAX_MACRO_VALUE, "%d", Q_rint(float64(cls.latency*1000)))
	return macro_buf
}

func Macro_Health() string {
	macro_buf = snprintf(MAX_MACRO_VALUE, "%d", cl.stats[STAT_HEALTH])
	return macro_buf
}

func Macro_Armor() string {
	macro_buf = snprintf(MAX_MACRO_VALUE, "%d", cl.stats[STAT_ARMOR])
	return macro_buf
}

func Macro_Shells() string {
	macro_buf = snprintf(MAX_MACRO_VALUE, "%d", cl.stats[STAT_SHELLS])
	return macro_buf
}

func Macro_Nails() string {
	macro_buf = snprintf(MAX_MACRO_VALUE, "%d", cl.stats[STAT_NAILS])
	return macro_buf
}

func Macro_Rockets() string {
	macro_buf = snprintf(MAX_MACRO_VALUE, "%d", cl.stats[STAT_ROCKETS])
	return macro_buf
}

func Macro_Cells() string {
	macro_buf = snprintf(MAX_MACRO_VALUE, "%d", cl.stats[STAT_CELLS])
	return macro_buf
}

func Macro_Ammo() string {
	macro_buf = snprintf(MAX_MACRO_VALUE, "%d", cl.stats[STAT_AMMO])
	return macro_buf
}

func Weapon_NumToString(num int) string {
	switch num {
	case IT_AXE:
		return tp_name_axe.string
	case IT_SHOTGUN:
		return tp_name_sg.string
	case IT_SUPER_SHOTGUN:
		return tp_name_ssg.string
	case IT_NAILGUN:
		return tp_name_ng.string
	case IT_SUPER_NAILGUN:
		return tp_name_sng.string
	case IT_GRENADE_LAUNCHER:
		return tp_name_gl.string
	case IT_ROCKET_LAUNCHER:
		return tp_name_rl.string
	case IT_LIGHTNING:
		return tp_name_lg.string
	default:
		return tp_name_none.string
	}
}

func Macro_Weapon() string {
	return Weapon_NumToString(cl.stats[STAT_ACTIVEWEAPON])
}

func Macro_WeaponAndAmmo() string {
	weapon := Macro_Weapon()
	macro_buf = snprintf(MAX_MACRO_VALUE, "%s:%s", weapon, Macro_Ammo())
	return macro_buf
}

func Macro_WeaponNum() string {
	switch cl.stats[STAT_ACTIVEWEAPON] {

	case IT_AXE:
		return "1"
	case IT_SHOTGUN:
		return "2"
	case IT_SUPER_SHOTGUN:
		return "3"
	case IT_NAILGUN:
		return "4"
	case IT_SUPER_NAILGUN:
		return "5"
	case IT_GRENADE_LAUNCHER:
		return "6"
	case IT_ROCKET_LAUNCHER:
		return "7"
	case IT_LIGHTNING:
		return "8"
	default:
		return "0"
	}
}

func BestWeapon() int {
	var i int
	t := []string{tp_weapon_order.string, "78654321"}

	for _, s := range t {
		for i = 0; i < len(s); i++ {
			switch s[i] {
			case '1':
				if cl.stats[STAT_ITEMS]&IT_AXE != 0 {
					return IT_AXE
				}
			case '2':
				if cl.stats[STAT_ITEMS]&IT_SHOTGUN != 0 {
					return IT_SHOTGUN
				}
			case '3':
				if cl.stats[STAT_ITEMS]&IT_SUPER_SHOTGUN != 0 {
					return IT_SUPER_SHOTGUN
				}
			case '4':
				if cl.stats[STAT_ITEMS]&IT_NAILGUN != 0 {
					return IT_NAILGUN
				}
			case '5':
				if cl.stats[STAT_ITEMS]&IT_SUPER_NAILGUN != 0 {
					return IT_SUPER_NAILGUN
				}
			case '6':
				if cl.stats[STAT_ITEMS]&IT_GRENADE_LAUNCHER != 0 {
					return IT_GRENADE_LAUNCHER
				}
			case '7':
				if cl.stats[STAT_ITEMS]&IT_ROCKET_LAUNCHER != 0 {
					return IT_ROCKET_LAUNCHER
				}
			case '8':
				if cl.stats[STAT_ITEMS]&IT_LIGHTNING != 0 {
					return IT_LIGHTNING
				}
			}
		}
	}
	return 0
}

func Macro_BestWeapon() string {
	switch BestWeapon() {
	case IT_AXE:
		return tp_name_axe.string
	case IT_SHOTGUN:
		return tp_name_sg.string
	case IT_SUPER_SHOTGUN:
		return tp_name_ssg.string
	case IT_NAILGUN:
		return tp_name_ng.string
	case IT_SUPER_NAILGUN:
		return tp_name_sng.string
	case IT_GRENADE_LAUNCHER:
		return tp_name_gl.string
	case IT_ROCKET_LAUNCHER:
		return tp_name_rl.string
	case IT_LIGHTNING:
		return tp_name_lg.string
	default:
		return tp_name_none.string
	}
}

func Macro_BestAmmo() string {
	switch BestWeapon() {
	case IT_SHOTGUN, IT_SUPER_SHOTGUN:
		macro_buf = snprintf(MAX_MACRO_VALUE, "%d", cl.stats[STAT_SHELLS])
		return macro_buf

	case IT_NAILGUN, IT_SUPER_NAILGUN:
		macro_buf = snprintf(MAX_MACRO_VALUE, "%d", cl.stats[STAT_NAILS])
		return macro_buf

	case IT_GRENADE_LAUNCHER, IT_ROCKET_LAUNCHER:
		macro_buf = snprintf(MAX_MACRO_VALUE, "%d", cl.stats[STAT_ROCKETS])
		return macro_buf

	case IT_LIGHTNING:
		macro_buf = snprintf(MAX_MACRO_VALUE, "%d", cl.stats[STAT_CELLS])
		return macro_buf

	default:
		return "0"
	}
}

// needed for %b parsing
func Macro_BestWeaponAndAmmo() string {
	weapon := Macro_BestWeapon()
	macro_buf = snprintf(MAX_MACRO_VALUE, "%s:%s", weapon, Macro_BestAmmo())
	return macro_buf
}

func Macro_ArmorType() string {
	if cl.stats[STAT_ITEMS]&IT_ARMOR1 != 0 {
		return tp_name_armortype_ga.string
	} else if cl.stats[STAT_ITEMS]&IT_ARMOR2 != 0 {
		return tp_name_armortype_ya.string
	} else if cl.stats[STAT_ITEMS]&IT_ARMOR3 != 0 {
		return tp_name_armortype_ra.string
	} else {
		return tp_name_none.string
	}
}

func Macro_Powerups() string {
	var effects int

	macro_buf = ""

	if cl.stats[STAT_ITEMS]&IT_QUAD != 0 {
		macro_buf = strlcpy(tp_name_quad.string, MAX_MACRO_VALUE)
	}

	if cl.stats[STAT_ITEMS]&IT_INVULNERABILITY != 0 {
		macroAppend(tp_name_pent.string)
	}

	if cl.stats[STAT_ITEMS]&IT_INVISIBILITY != 0 {
		macroAppend(tp_name_ring.string)
	}

	effects = cl.frames[cl.parsecount&UPDATE_MASK].playerstate[cl.playernum].effects
	if effects&(EF_FLAG1|EF_FLAG2) != 0 /* CTF */ ||
		(cl.teamfortress && cl.stats[STAT_ITEMS]&(IT_KEY1|IT_KEY2) != 0) /* TF */ {
		macroAppend(tp_name_flag.string)
	}

	if macro_buf == "" {
		macro_buf = strlcpy(tp_name_none.string, MAX_MACRO_VALUE)
	}

	return macro_buf
}

func Macro_Location() string {
	vars.lastreportedloc = strlcpy(TP_LocationName(cl.simorg), MAX_LOC_NAME)
	return vars.lastreportedloc
}

func Macro_LastDeath() string {
	if vars.deathtrigger_time != 0 {
		return vars.lastdeathloc
	}
	return tp_name_someplace.string
}

func Macro_Last_Location() string {
	if vars.deathtrigger_time != 0 && cls.realtime-vars.deathtrigger_time <= 5 {
		vars.lastreportedloc = strlcpy(vars.lastdeathloc, MAX_LOC_NAME)
	} else {
		vars.lastreportedloc = strlcpy(TP_LocationName(cl.simorg), MAX_LOC_NAME)
	}
	return vars.lastreportedloc
}

func Macro_LastReportedLoc() string {
	if vars.lastreportedloc == "" {
		return tp_name_someplace.string
	}
	return vars.lastreportedloc
}

func Macro_Time() string {
	macro_buf = time.Now().Format("15:04")
	return macro_buf
}

func Macro_Date() string {
	macro_buf = time.Now().Format("02.01.06")
	return macro_buf
}

// returns the last item picked up
func Macro_Took() string {
	if vars.tooktime == 0 || cls.realtime > vars.tooktime+TP_TOOK_EXPIRE_TIME {
		macro_buf = strlcpy(tp_name_nothing.string, MAX_MACRO_VALUE)
	} else {
		macro_buf = strlcpy(vars.tookname, MAX_MACRO_VALUE)
	}
	return macro_buf
}

// returns location of the last item picked up
func Macro_TookLoc() string {
	if vars.tooktime == 0 || cls.realtime > vars.tooktime+TP_TOOK_EXPIRE_TIME {
		macro_buf = strlcpy(tp_name_someplace.string, MAX_MACRO_VALUE)
	} else {
		macro_buf = strlcpy(vars.tookloc, MAX_MACRO_VALUE)
	}
	return macro_buf
}

// %i macro - last item picked up in "name at location" style
func Macro_TookAtLoc() string {
	if vars.tooktime == 0 || cls.realtime > vars.tooktime+TP_TOOK_EXPIRE_TIME {
		macro_buf = strlcpy(tp_name_nothing.string, MAX_MACRO_VALUE)
	} else {
		macro_buf = strlcpy(va("%s %s %s", vars.tookname, tp_name_at.string, vars.tookloc), MAX_MACRO_VALUE)
	}
	return macro_buf
}

// pointing calculations are CPU expensive, so the results are cached
// in vars.pointname & vars.pointloc
func Macro_PointName() string {
	TP_FindPoint()
	return vars.pointname
}

func Macro_PointLocation() string {
	TP_FindPoint()
	if vars.pointloc != "" {
		return vars.pointloc
	}
	return Macro_Location()
}

func Macro_LastPointAtLoc() string {
	if vars.pointtime == 0 || cls.realtime-vars.pointtime > TP_POINT_EXPIRE_TIME {
		macro_buf = strlcpy(tp_name_nothing.string, MAX_MACRO_VALUE)
	} else {
		loc := vars.pointloc
		if loc == "" {
			loc = Macro_Location()
		}
		macro_buf = snprintf(MAX_MACRO_VALUE, "%s %s %s", vars.pointname, tp_name_at.string, loc)
	}
	return macro_buf
}

func Macro_PointNameAtLocation() string {
	TP_FindPoint()
	return Macro_LastPointAtLoc()
}

func Macro_Weapons() string {
	macro_buf = ""

	if cl.stats[STAT_ITEMS]&IT_LIGHTNING != 0 {
		macro_buf = strlcpy(tp_name_lg.string, MAX_MACRO_VALUE)
	}

	if cl.stats[STAT_ITEMS]&IT_ROCKET_LAUNCHER != 0 {
		macroAppend(tp_name_rl.string)
	}
	if cl.stats[STAT_ITEMS]&IT_GRENADE_LAUNCHER != 0 {
		macroAppend(tp_name_gl.string)
	}
	if cl.stats[STAT_ITEMS]&IT_SUPER_NAILGUN != 0 {
		macroAppend(tp_name_sng.string)
	}
	if cl.stats[STAT_ITEMS]&IT_NAILGUN != 0 {
		macroAppend(tp_name_ng.string)
	}
	if cl.stats[STAT_ITEMS]&IT_SUPER_SHOTGUN != 0 {
		macroAppend(tp_name_ssg.string)
	}
	if cl.stats[STAT_ITEMS]&IT_SHOTGUN != 0 {
		macroAppend(tp_name_sg.string)
	}
	if cl.stats[STAT_ITEMS]&IT_AXE != 0 {
		macroAppend(tp_name_axe.string)
	}

	if macro_buf == "" {
		macro_buf = strlcpy(tp_name_none.string, MAX_MACRO_VALUE)
	}

	return macro_buf
}

func Skin_To_TFSkin(myskin string) string {
	if !cl.teamfortress || cl.spectator != 0 || Q_strncasecmp(myskin, "tf_", 3) != 0 {
		macro_buf = strlcpy(myskin, MAX_MACRO_VALUE)
	} else {
		if Q_strcasecmp(myskin, "tf_demo") == 0 {
			macro_buf = strlcpy("demoman", MAX_MACRO_VALUE)
		} else if Q_strcasecmp(myskin, "tf_eng") == 0 {
			macro_buf = strlcpy("engineer", MAX_MACRO_VALUE)
		} else if Q_strcasecmp(myskin, "tf_snipe") == 0 {
			macro_buf = strlcpy("sniper", MAX_MACRO_VALUE)
		} else if Q_strcasecmp(myskin, "tf_sold") == 0 {
			macro_buf = strlcpy("soldier", MAX_MACRO_VALUE)
		} else {
			macro_buf = strlcpy(myskin[3:], MAX_MACRO_VALUE)
		}
	}
	return macro_buf
}

func Macro_TF_Skin() string {
	return Skin_To_TFSkin(Info_ValueForKey(cl.players[cl.playernum].userinfo, "skin"))
}

func Macro_LastDrop() string {
	if vars.lastdrop_time != 0 {
		return vars.lastdroploc
	} else {
		return tp_name_someplace.string
	}
}

func Macro_LastTrigger_Match() string {
	return vars.lasttrigger_match
}

func Macro_LastDropTime() string {
	if vars.lastdrop_time != 0 {
		macro_buf = snprintf(32, "%d", int(cls.realtime-vars.lastdrop_time))
	} else {
		macro_buf = snprintf(32, "%d", -1)
	}
	return macro_buf
}

func Macro_Need() string {
	var i, weapon int
	var needammo string
	var haveammo bool

	macro_buf = ""

	// check armor
	if (cl.stats[STAT_ITEMS]&IT_ARMOR1 != 0 && float32(cl.stats[STAT_ARMOR]) < tp_need_ga.value) ||
		(cl.stats[STAT_ITEMS]&IT_ARMOR2 != 0 && float32(cl.stats[STAT_ARMOR]) < tp_need_ya.value) ||
		(cl.stats[STAT_ITEMS]&IT_ARMOR3 != 0 && float32(cl.stats[STAT_ARMOR]) < tp_need_ra.value) ||
		(cl.stats[STAT_ITEMS]&(IT_ARMOR1|IT_ARMOR2|IT_ARMOR3) == 0 &&
			(tp_need_ga.value != 0 || tp_need_ya.value != 0 || tp_need_ra.value != 0)) {
		macro_buf = strlcpy(tp_name_armor.string, MAX_MACRO_VALUE)
	}

	// check health
	if tp_need_health.value != 0 && float32(cl.stats[STAT_HEALTH]) < tp_need_health.value {
		macroAppend(tp_name_health.string)
	}

	if cl.teamfortress {
		if float32(cl.stats[STAT_ROCKETS]) < tp_need_rockets.value {
			macroAppend(tp_name_rockets.string)
		}
		if float32(cl.stats[STAT_SHELLS]) < tp_need_shells.value {
			macroAppend(tp_name_shells.string)
		}
		if float32(cl.stats[STAT_NAILS]) < tp_need_nails.value {
			macroAppend(tp_name_nails.string)
		}
		if float32(cl.stats[STAT_CELLS]) < tp_need_cells.value {
			macroAppend(tp_name_cells.string)
		}
		goto done
	}
	// check weapon
	weapon = 0
	for i = len(tp_need_weapon.string) - 1; i >= 0; i-- {
		switch tp_need_weapon.string[i] {
		case '2':
			if cl.stats[STAT_ITEMS]&IT_SHOTGUN != 0 {
				weapon = 2
			}
		case '3':
			if cl.stats[STAT_ITEMS]&IT_SUPER_SHOTGUN != 0 {
				weapon = 3
			}
		case '4':
			if cl.stats[STAT_ITEMS]&IT_NAILGUN != 0 {
				weapon = 4
			}
		case '5':
			if cl.stats[STAT_ITEMS]&IT_SUPER_NAILGUN != 0 {
				weapon = 5
			}
		case '6':
			if cl.stats[STAT_ITEMS]&IT_GRENADE_LAUNCHER != 0 {
				weapon = 6
			}
		case '7':
			if cl.stats[STAT_ITEMS]&IT_ROCKET_LAUNCHER != 0 {
				weapon = 7
			}
		case '8':
			if cl.stats[STAT_ITEMS]&IT_LIGHTNING != 0 {
				weapon = 8
			}
		}
		if weapon != 0 {
			break
		}
	}

	if weapon == 0 {
		macroAppend(tp_name_weapon.string)
	} else {
		if tp_need_rl.value != 0 && cl.stats[STAT_ITEMS]&IT_ROCKET_LAUNCHER == 0 {
			macroAppend(tp_name_rl.string)
		}

		switch weapon {
		case 2, 3:
			if float32(cl.stats[STAT_SHELLS]) < tp_need_shells.value {
				needammo = tp_name_shells.string
				haveammo = true
			}
		case 4, 5:
			if float32(cl.stats[STAT_NAILS]) < tp_need_nails.value {
				needammo = tp_name_nails.string
				haveammo = true
			}
		case 6, 7:
			if float32(cl.stats[STAT_ROCKETS]) < tp_need_rockets.value {
				needammo = tp_name_rockets.string
				haveammo = true
			}
		case 8:
			if float32(cl.stats[STAT_CELLS]) < tp_need_cells.value {
				needammo = tp_name_cells.string
				haveammo = true
			}
		}

		if haveammo {
			macroAppend(needammo)
		}
	}

done:
	if macro_buf == "" {
		macro_buf = strlcpy(tp_name_nothing.string, MAX_MACRO_VALUE)
	}

	return macro_buf
}

func Macro_Point_LED() string {
	TP_FindPoint()

	if vars.pointtype == POINT_TYPE_ENEMY {
		return tp_name_status_red.string
	} else if vars.pointtype == POINT_TYPE_TEAMMATE {
		return tp_name_status_green.string
	} else if vars.pointtype == POINT_TYPE_POWERUP {
		return tp_name_status_yellow.string
	} else { // POINT_TYPE_ITEM
		return tp_name_status_blue.string
	}
}

func Macro_MyStatus_LED() string {
	var count int
	var save_need_rl float32
	var s string
	var save_separator string
	const separator = "/"

	save_need_rl = tp_need_rl.value
	save_separator = tp_name_separator.string
	tp_need_rl.value = 0
	tp_name_separator.string = separator
	s = Macro_Need()
	tp_need_rl.value = save_need_rl
	tp_name_separator.string = save_separator

	if s == tp_name_nothing.string {
		count = 0
	} else {
		count = 1
		for i := 0; i < len(s); i++ {
			if s[i] == separator[0] {
				count++
			}
		}
	}

	if count == 0 {
		macro_buf = snprintf(MAX_MACRO_VALUE, "%s", tp_name_status_green.string)
	} else if count <= 1 {
		macro_buf = snprintf(MAX_MACRO_VALUE, "%s", tp_name_status_yellow.string)
	} else {
		macro_buf = snprintf(MAX_MACRO_VALUE, "%s", tp_name_status_red.string)
	}

	return macro_buf
}

func Macro_EnemyStatus_LED() string {
	CountNearbyPlayers(false)
	if vars.numenemies == 0 {
		macro_buf = snprintf(MAX_MACRO_VALUE, "\xffl%s\xff", tp_name_status_green.string)
	} else if vars.numenemies <= vars.numfriendlies {
		macro_buf = snprintf(MAX_MACRO_VALUE, "\xffl%s\xff", tp_name_status_yellow.string)
	} else {
		macro_buf = snprintf(MAX_MACRO_VALUE, "\xffl%s\xff", tp_name_status_red.string)
	}

	suppress = true
	return macro_buf
}

const (
	TP_PENT = 1
	TP_QUAD = 2
	TP_RING = 4
)

func Macro_LastSeenPowerup() string {
	if vars.enemy_powerups_time == 0 || cls.realtime-vars.enemy_powerups_time > 5 {
		macro_buf = strlcpy(tp_name_quad.string, MAX_MACRO_VALUE)
	} else {
		macro_buf = ""
		if vars.enemy_powerups&TP_QUAD != 0 {
			macro_buf = strlcat(macro_buf, tp_name_quad.string, MAX_MACRO_VALUE)
		}
		if vars.enemy_powerups&TP_PENT != 0 {
			macroAppend(tp_name_pent.string)
		}
		if vars.enemy_powerups&TP_RING != 0 {
			macroAppend(tp_name_ring.string)
		}
	}
	return macro_buf
}

// C trims the "\x7f!<player>\n" marker in place; the result goes back through buf
func TP_SuppressMessage(buf *string) bool {
	var len_ int

	if len_ = len(*buf); len_ < 4 {
		return false
	}

	s := (*buf)[len_-4:]

	if s[0] == 0x7F && s[1] == '!' && s[3] == '\n' {
		*buf = (*buf)[:len_-4] + "\n"
		return !cls.demoplayback && cl.spectator == 0 && int(s[2])-'A' == cl.playernum
	}

	return false
}

// strips the quotes and the hidden-text markers from buf and prints the
// visible part locally
func TP_PrintHiddenMessage(buf *string) {
	var team, hide bool
	var dest []byte
	var length, offset, flags int
	var name string

	b := *buf
	if length = len(b); length == 0 {
		return
	}

	team = Q_strcasecmp("say_team", Cmd_Argv(0)) == 0

	if length >= 2 && b[0] == '"' && b[length-1] == '"' {
		b = b[1 : length-1]
	}

	var out []byte
	for i := 0; i < len(b); i++ {
		c := b[i]
		if c == 0xff {
			hide = !hide
			if hide {
				if at(b, i+1) == 'z' {
					dest = append(dest, 'x')
				} else {
					dest = append(dest, 139)
				}
				i++
			}
		} else {
			if !hide {
				dest = append(dest, c)
			}
			out = append(out, c)
		}
	}
	*buf = string(out)

	if cls.demoplayback {
		return
	}

	name = Info_ValueForKey(cl.players[cl.playernum].userinfo, "name")
	if len(name) >= 32 {
		name = name[:31]
	}

	var msg string
	if team {
		msg = snprintf(4096, "(%s): %s\n", name, TP_ParseFunChars(string(dest), true))
	} else {
		msg = snprintf(4096, "%s: %s\n", name, TP_ParseFunChars(string(dest), true))
	}

	flags = TP_CategorizeMessage(msg, &offset)

	if flags == 2 {
		tail, ok := TP_FilterMessage(msg[offset:])
		msg = msg[:offset] + tail
		if !ok {
			return
		}
	}

	if cl_chatsound.value != 0 {
		S_LocalSound("misc/talk.wav")
	}

	if cl_nofake.value == 1 || (cl_nofake.value == 2 && flags != 2) {
		m := []byte(msg)
		for i := range m {
			if m[i] == 0x0D || (m[i] == 0x0A && i+1 < len(m)) {
				m[i] = ' '
			}
		}
		msg = string(m)
	}

	Com_Printf("%s", TP_ParseWhiteText(msg, team, offset))
}

var countnearby_lastframecount = -1

func CountNearbyPlayers(dead bool) {
	var i int
	var state *player_state_t
	var info *player_info_t

	if cls.framecount == countnearby_lastframecount {
		return
	}
	countnearby_lastframecount = cls.framecount

	vars.numenemies = 0
	vars.numfriendlies = 0

	if cl.spectator == 0 && !dead {
		vars.numfriendlies++
	}

	if cl.oldparsecount == 0 || cl.parsecount == 0 || cls.state < ca_active {
		return
	}

	for i = 0; i < MAX_CLIENTS; i++ {
		state = &cl.frames[cl.oldparsecount&UPDATE_MASK].playerstate[i]
		info = &cl.players[i]
		if i != cl.playernum && state.messagenum == cl.oldparsecount && info.spectator == 0 && !ISDEAD(state.frame) {
			if cl.teamplay != 0 && info.team == TP_PlayerTeam() {
				vars.numfriendlies++
			} else {
				vars.numenemies++
			}
		}
	}
}

func Macro_CountNearbyEnemyPlayers() string {
	CountNearbyPlayers(false)
	macro_buf = fmt.Sprintf("\xffz%d\xff", vars.numenemies)
	suppress = true
	return macro_buf
}

func Macro_Count_Last_NearbyEnemyPlayers() string {
	if vars.deathtrigger_time != 0 && cls.realtime-vars.deathtrigger_time <= 5 {
		macro_buf = fmt.Sprintf("\xffz%d\xff", vars.last_numenemies)
	} else {
		CountNearbyPlayers(false)
		macro_buf = fmt.Sprintf("\xffz%d\xff", vars.numenemies)
	}
	suppress = true
	return macro_buf
}

func Macro_CountNearbyFriendlyPlayers() string {
	CountNearbyPlayers(false)
	macro_buf = fmt.Sprintf("\xffz%d\xff", vars.numfriendlies)
	suppress = true
	return macro_buf
}

func Macro_Count_Last_NearbyFriendlyPlayers() string {
	if vars.deathtrigger_time != 0 && cls.realtime-vars.deathtrigger_time <= 5 {
		macro_buf = fmt.Sprintf("\xffz%d\xff", vars.last_numfriendlies)
	} else {
		CountNearbyPlayers(false)
		macro_buf = fmt.Sprintf("\xffz%d\xff", vars.numfriendlies)
	}
	suppress = true
	return macro_buf
}

// Note: longer macro names like "armortype" must be defined
// _before_ the shorter ones like "armor" to be parsed properly
func TP_AddMacros() {
	teamplay := true /* Yeah, I should fix this up some day */

	Cmd_AddMacro("qt", Macro_Quote_f)
	Cmd_AddMacro("latency", Macro_Latency)
	Cmd_AddMacro("time", Macro_Time)
	Cmd_AddMacro("date", Macro_Date)

	Cmd_AddMacroEx("health", Macro_Health, teamplay)
	Cmd_AddMacroEx("armortype", Macro_ArmorType, teamplay)
	Cmd_AddMacroEx("armor", Macro_Armor, teamplay)

	Cmd_AddMacroEx("shells", Macro_Shells, teamplay)
	Cmd_AddMacroEx("nails", Macro_Nails, teamplay)
	Cmd_AddMacroEx("rockets", Macro_Rockets, teamplay)
	Cmd_AddMacroEx("cells", Macro_Cells, teamplay)

	Cmd_AddMacro("weaponnum", Macro_WeaponNum)
	Cmd_AddMacroEx("weapons", Macro_Weapons, teamplay)
	Cmd_AddMacro("weapon", Macro_Weapon)

	Cmd_AddMacroEx("ammo", Macro_Ammo, teamplay)

	Cmd_AddMacroEx("bestweapon", Macro_BestWeapon, teamplay)
	Cmd_AddMacroEx("bestammo", Macro_BestAmmo, teamplay)

	Cmd_AddMacroEx("powerups", Macro_Powerups, teamplay)

	Cmd_AddMacroEx("location", Macro_Location, teamplay)
	Cmd_AddMacroEx("deathloc", Macro_LastDeath, teamplay)

	Cmd_AddMacroEx("tookatloc", Macro_TookAtLoc, teamplay)
	Cmd_AddMacroEx("tookloc", Macro_TookLoc, teamplay)
	Cmd_AddMacroEx("took", Macro_Took, teamplay)

	Cmd_AddMacroEx("pointatloc", Macro_PointNameAtLocation, teamplay)
	Cmd_AddMacroEx("pointloc", Macro_PointLocation, teamplay)
	Cmd_AddMacroEx("point", Macro_PointName, teamplay)

	Cmd_AddMacroEx("need", Macro_Need, teamplay)

	Cmd_AddMacroEx("droploc", Macro_LastDrop, teamplay)
	Cmd_AddMacroEx("droptime", Macro_LastDropTime, teamplay)

	Cmd_AddMacro("tf_skin", Macro_TF_Skin)

	Cmd_AddMacro("triggermatch", Macro_LastTrigger_Match)

	Cmd_AddMacroEx("ledpoint", Macro_Point_LED, teamplay)
	Cmd_AddMacroEx("ledstatus", Macro_MyStatus_LED, teamplay)
}

/********************** MACRO/FUNCHAR/WHITE TEXT PARSING **********************/

const MAX_MACRO_STRING = 2048

func TP_ParseWhiteText(s string, team bool, offset int) string {
	var out []byte

	parsewhite := cl_parseWhiteText.value == 1 || (cl_parseWhiteText.value == 2 && team)

	for p := 0; p < len(s); p++ {
		if parsewhite && s[p] == '{' && p >= offset {
			if p1 := strings.IndexByte(s[p+1:], '}'); p1 != -1 {
				p1 += p + 1
				out = append(out, s[p+1:p1]...)
				p = p1
				continue
			}
		}
		if s[p] != 10 && s[p] != 13 && !(p == 0 && (s[p] == 1 || s[p] == 2)) {
			out = append(out, s[p]|128) // convert to red
		} else {
			out = append(out, s[p])
		}
	}
	return string(out)
}

// Parses %a-like expressions
func TP_ParseMacroString(s string) string {
	var buf []byte
	var macro_string string
	var mbuf string

	if cl_parseSay.value == 0 {
		return s
	}

	suppress = false

	p := 0
	for p < len(s) && len(buf) < MAX_MACRO_STRING-1 {
		// check %[P], etc
		if s[p] == '%' && at(s, p+1) == '[' && at(s, p+2) != 0 && at(s, p+3) == ']' {
			switch s[p+2] {
			case 'a':
				macro_string = Macro_ArmorType()
				if macro_string == tp_name_none.string {
					macro_string = "a"
				}
				if cl.stats[STAT_ARMOR] < 30 {
					mbuf = snprintf(MAX_MACRO_VALUE, "\x10%s:%d\x11", macro_string, cl.stats[STAT_ARMOR])
				} else {
					mbuf = snprintf(MAX_MACRO_VALUE, "%s:%d", macro_string, cl.stats[STAT_ARMOR])
				}
				macro_string = mbuf

			case 'h':
				if cl.stats[STAT_HEALTH] >= 50 {
					macro_buf = snprintf(MAX_MACRO_VALUE, "%d", cl.stats[STAT_HEALTH])
				} else {
					macro_buf = snprintf(MAX_MACRO_VALUE, "\x10%d\x11", cl.stats[STAT_HEALTH])
				}
				macro_string = macro_buf

			case 'p', 'P':
				macro_string = Macro_Powerups()
				if macro_string != tp_name_none.string {
					mbuf = snprintf(MAX_MACRO_VALUE, "\x10%s\x11", macro_string)
				} else {
					mbuf = ""
				}
				macro_string = mbuf

			default:
				buf = append(buf, s[p])
				p++
				continue
			}
			if len(buf)+len(macro_string) >= MAX_MACRO_STRING-1 {
				Sys_Error("TP_ParseMacroString: macro string length > MAX_MACRO_STRING)")
			}
			buf = append(buf, macro_string...)
			p += 4 // skip %[<char>]
			continue
		}

		// check %a, etc
		if s[p] == '%' {
			switch at(s, p+1) {
			case 'a':
				macro_string = Macro_Armor()
			case 'A':
				macro_string = Macro_ArmorType()
			case 'b':
				macro_string = Macro_BestWeaponAndAmmo()
			case 'c':
				macro_string = Macro_Cells()
			case 'd':
				macro_string = Macro_LastDeath()
			case 'h':
				macro_string = Macro_Health()
			case 'i':
				macro_string = Macro_TookAtLoc()
			case 'j':
				macro_string = Macro_LastPointAtLoc()
			case 'k':
				macro_string = Macro_LastTookOrPointed()
			case 'l':
				macro_string = Macro_Location()
			case 'L':
				macro_string = Macro_Last_Location()
			case 'm':
				macro_string = Macro_LastTookOrPointed()

			case 'o':
				macro_string = Macro_CountNearbyFriendlyPlayers()
			case 'e':
				macro_string = Macro_CountNearbyEnemyPlayers()
			case 'O':
				macro_string = Macro_Count_Last_NearbyFriendlyPlayers()
			case 'E':
				macro_string = Macro_Count_Last_NearbyEnemyPlayers()

			case 'P', 'p':
				macro_string = Macro_Powerups()
			case 'q':
				macro_string = Macro_LastSeenPowerup()
			case 'r':
				macro_string = Macro_LastReportedLoc()
			case 's':
				macro_string = Macro_EnemyStatus_LED()
			case 'S':
				macro_string = Macro_TF_Skin()
			case 't':
				macro_string = Macro_PointNameAtLocation()
			case 'u':
				macro_string = Macro_Need()
			case 'w':
				macro_string = Macro_WeaponAndAmmo()
			case 'x':
				macro_string = Macro_PointName()
			case 'X':
				macro_string = Macro_Took()
			case 'y':
				macro_string = Macro_PointLocation()
			case 'Y':
				macro_string = Macro_TookLoc()
			case '%':
				macro_string = "%"
			default:
				buf = append(buf, s[p])
				p++
				continue
			}
			if len(buf)+len(macro_string) >= MAX_MACRO_STRING-1 {
				Sys_Error("TP_ParseMacroString: macro string length > MAX_MACRO_STRING)")
			}
			buf = append(buf, macro_string...)
			p += 2 // skip % and letter
			continue
		}
		buf = append(buf, s[p])
		p++
	}

	if suppress {
		quotes := false

		str := string(buf)
		TP_PrintHiddenMessage(&str)
		if len(str) > 0 && str[len(str)-1] == '"' {
			str = str[:len(str)-1]
			quotes = true
		}
		str += string([]byte{0x7f, '!', byte('A' + cl.playernum)})
		if quotes {
			str += "\""
		}
		return str
	}

	return string(buf)
}

// Doesn't check for overflows, so strlen(s) should be < MAX_MACRO_STRING
func TP_ParseFunChars(s string, chat bool) string {
	var out []byte
	var c byte

	if cl_parseFunChars.value == 0 {
		return s
	}

	for p := 0; p < len(s) && len(out) < MAX_MACRO_STRING; {
		if s[p] == '$' && at(s, p+1) == 'x' {
			var i int
			// check for $x10, $x8a, etc
			ok := true
			c = tolower(at(s, p+2))
			if c >= '0' && c <= '9' {
				i = int(c-'0') << 4
			} else if c >= 'a' && c <= 'f' {
				i = int(c-'a'+10) << 4
			} else {
				ok = false
			}
			if ok {
				c = tolower(at(s, p+3))
				if c >= '0' && c <= '9' {
					i += int(c - '0')
				} else if c >= 'a' && c <= 'f' {
					i += int(c - 'a' + 10)
				} else {
					ok = false
				}
			}
			if ok {
				if i == 0 {
					i = ' '
				}
				out = append(out, byte(i))
				p += 4
				continue
			}
			// goto skip
		} else {
			if s[p] == '$' && at(s, p+1) != 0 {
				c = 0
				switch s[p+1] {
				case '\\':
					c = 0x0D
				case ':':
					c = 0x0A
				case '[':
					c = 0x10
				case ']':
					c = 0x11
				case 'G':
					c = 0x86
				case 'R':
					c = 0x87
				case 'Y':
					c = 0x88
				case 'B':
					c = 0x89
				case '(':
					c = 0x80
				case '=':
					c = 0x81
				case ')':
					c = 0x82
				case 'a':
					c = 0x83
				case '<':
					c = 0x1d
				case '-':
					c = 0x1e
				case '>':
					c = 0x1f
				case ',':
					c = 0x1c
				case '.':
					c = 0x9c
				case 'b':
					c = 0x8b
				case 'c', 'd':
					c = 0x8d
				case '$':
					c = '$'
				case '^':
					c = '^'
				}
				if s[p+1] >= '0' && s[p+1] <= '9' {
					c = s[p+1] - '0' + 0x12
				}
				if c != 0 {
					out = append(out, c)
					p += 2
					continue
				}
			}
			if !chat && s[p] == '^' && at(s, p+1) != 0 && s[p+1] != ' ' {
				out = append(out, s[p+1]|128)
				p += 2
				continue
			}
		}
		// skip:
		out = append(out, s[p])
		p++
	}

	return string(out)
}

/************************* SKIN FORCING & REFRESHING *************************/

var ourlastplayernum = -1

func TP_CalculateSkinForPlayer(slot uint) {
	var ourplayernum int
	var isteam bool
	var skin string
	var state *player_state_t
	var effects uint

	skin = Info_ValueForKey(cl.players[slot].userinfo, "skin")

	if cl.spectator != 0 {
		ourplayernum = Cam_TrackNum()
	} else {
		ourplayernum = cl.playernum
	}

	if !cl.teamfortress && cl.fpd&FPD_NO_FORCE_SKIN == 0 {
		if ourplayernum == -1 {
			ourplayernum = ourlastplayernum
		} else {
			ourlastplayernum = ourplayernum
		}

		if ourplayernum == -1 {
			isteam = false
		} else {
			isteam = cl.teamplay != 0 && cl.players[ourplayernum].team == cl.players[slot].team
		}

		if isteam && cl_teamskin.string != "" {
			skin = cl_teamskin.string
		} else if !isteam && cl_enemyskin.string != "" {
			skin = cl_enemyskin.string
		}

		state = &cl.frames[cl.parsecount&UPDATE_MASK].playerstate[slot]
		if state.messagenum == cl.parsecount {
			effects = uint(state.effects)

			if effects&(EF_BLUE|EF_RED) == (EF_BLUE | EF_RED) {
				if isteam && cl_teambothskin.string != "" {
					skin = cl_teambothskin.string
				} else if !isteam && cl_enemybothskin.string != "" {
					skin = cl_enemybothskin.string
				}
			} else if effects&EF_BLUE != 0 {
				if isteam && cl_teamquadskin.string != "" {
					skin = cl_teamquadskin.string
				} else if !isteam && cl_enemyquadskin.string != "" {
					skin = cl_enemyquadskin.string
				}
			} else if effects&EF_RED != 0 {
				if isteam && cl_teampentskin.string != "" {
					skin = cl_teampentskin.string
				} else if !isteam && cl_enemypentskin.string != "" {
					skin = cl_enemypentskin.string
				}
			}
		}
	}

	cl.players[slot].skin = strlcpy(skin, 64)
}

func TP_CalculateColoursForPlayer(slot uint) {
	var ourplayernum int
	var isteam bool

	if cl.spectator != 0 {
		ourplayernum = Cam_TrackNum()
	} else {
		ourplayernum = cl.playernum
	}

	if ourplayernum == -1 {
		ourplayernum = ourlastplayernum
	} else {
		ourlastplayernum = ourplayernum
	}

	if ourplayernum == -1 {
		isteam = false
	} else {
		isteam = cl.teamplay != 0 && cl.players[ourplayernum].team == cl.players[slot].team
	}

	if isteam {
		if cl_teamtopcolor != -1 {
			cl.players[slot].topcolor = byte(cl_teamtopcolor)
			cl.players[slot].bottomcolor = byte(cl_teambottomcolor)
		} else {
			cl.players[slot].topcolor = cl.players[slot].real_topcolor
			cl.players[slot].bottomcolor = cl.players[slot].real_bottomcolor
		}
	} else {
		if cl_enemytopcolor != -1 {
			cl.players[slot].topcolor = byte(cl_enemytopcolor)
			cl.players[slot].bottomcolor = byte(cl_enemybottomcolor)
		} else {
			cl.players[slot].topcolor = cl.players[slot].real_topcolor
			cl.players[slot].bottomcolor = cl.players[slot].real_bottomcolor
		}
	}
}

func TP_RecalculateSkins() {
	var i uint

	for i = 0; i < MAX_CLIENTS; i++ {
		if cl.players[i].name != "" && cl.players[i].spectator == 0 {
			TP_CalculateSkinForPlayer(i)
		}
	}
}

func TP_RecalculateColours() {
	var i uint

	for i = 0; i < MAX_CLIENTS; i++ {
		if cl.players[i].name != "" && cl.players[i].spectator == 0 {
			TP_CalculateColoursForPlayer(i)
		}
	}
}

func OnChangeSkinForcing(v *cvar_t) {
	TP_RecalculateSkins()
}

func TP_Frame() {
	var num int

	if cl.spectator != 0 {
		num = Cam_TrackNum()
		if num != -1 && num != ourlastplayernum {
			TP_RecalculateSkins()
			TP_RecalculateColours()
		}
	}
}

var cl_teamtopcolor = -1
var cl_teambottomcolor int
var cl_enemytopcolor = -1
var cl_enemybottomcolor int

func TP_ColorForcing(topcolor, bottomcolor *int) {
	var top, bottom int

	if Cmd_Argc() == 1 {
		if *topcolor < 0 {
			Com_Printf("\"%s\" is \"off\"\n", Cmd_Argv(0))
		} else {
			Com_Printf("\"%s\" is \"%d %d\"\n", Cmd_Argv(0), *topcolor, *bottomcolor)
		}
		return
	}

	if Q_strcasecmp(Cmd_Argv(1), "off") == 0 {
		*topcolor = -1

		TP_RecalculateColours()
		return
	}

	if Cmd_Argc() == 2 {
		top = atoi(Cmd_Argv(1))
		bottom = top
	} else {
		top = atoi(Cmd_Argv(1))
		bottom = atoi(Cmd_Argv(2))
	}

	top &= 15
	top = min(13, top)
	bottom &= 15
	bottom = min(13, bottom)

	*topcolor = top
	*bottomcolor = bottom

	TP_RecalculateColours()
}

func TP_TeamColor_f() {
	TP_ColorForcing(&cl_teamtopcolor, &cl_teambottomcolor)
}

func TP_EnemyColor_f() {
	TP_ColorForcing(&cl_enemytopcolor, &cl_enemybottomcolor)
}

/********************************* .LOC FILES *********************************/

type locdata_t struct {
	coord vec3_t
	name  string
	next  *locdata_t
}

var locdata *locdata_t

func TP_ClearLocs() {
	locdata = nil
}

func TP_AddLocNode(coord vec3_t, name string) {
	var newnode, node *locdata_t

	newnode = &locdata_t{}
	newnode.name = name
	newnode.next = nil
	newnode.coord = coord

	if locdata == nil {
		locdata = newnode
		return
	}

	for node = locdata; node.next != nil; node = node.next {
	}

	node.next = newnode
}

func locSkipBlanks(buf []byte, p *int) {
	for at2(buf, *p) == ' ' || at2(buf, *p) == '\t' || at2(buf, *p) == '\r' {
		*p++
	}
}

func locSkipToEOL(buf []byte, p *int) {
	for at2(buf, *p) != '\n' && at2(buf, *p) != 0 {
		*p++
	}
	if at2(buf, *p) == '\n' {
		*p++
	}
}

// at for a byte buffer: 0 past the end, like the NUL FS_LoadFile appends in C
func at2(b []byte, i int) byte {
	if i < len(b) {
		return b[i]
	}
	return 0
}

func TP_LoadLocFile(path string, quiet bool) bool {
	var buf []byte
	var p int
	var locname string
	var location []byte
	var i, n, sign, line, loc_numentries int
	var overflow bool
	var coord vec3_t

	if path == "" {
		return false
	}

	locname = "locs/"
	if len(path)+len(locname)+2+4 > MAX_OSPATH {
		Com_Printf("TP_LoadLocFile: path name > MAX_OSPATH\n")
		return false
	}
	locname = strlcat(locname, path, MAX_OSPATH)
	locname = COM_DefaultExtension(locname, ".loc", MAX_OSPATH)

	if buf = FS_LoadMallocFile(locname); buf == nil {
		if !quiet {
			Com_Printf("Could not load %s\n", locname)
		}

		return false
	}

	TP_ClearLocs()
	loc_numentries = 0

	// Parse the whole file now
	p = 0
	line = 0

	for {
		line++
		locSkipBlanks(buf, &p)

		if at2(buf, p) == 0 {
			break
		} else if buf[p] == '\n' {
			p++
			continue
		} else if buf[p] == '/' && at2(buf, p+1) == '/' {
			locSkipToEOL(buf, &p)
			continue
		}

		// parse three ints
		for i = 0; i < 3; i++ {
			n = 0
			sign = 1
		parse:
			for {
				if at2(buf, p) == 0 {
					break
				}

				c := buf[p]
				p++
				switch c {
				case ' ', '\t':
					break parse
				case '-':
					if n != 0 {
						Com_Printf("Locfile error (line %d): unexpected '-'\n", line)
						locSkipToEOL(buf, &p)
						continue
					}
					sign = -1
				case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
					n = n*10 + int(c-'0')
				default: // including eol or eof
					Com_Printf("Locfile error (line %d): couldn't parse coords\n", line)
					locSkipToEOL(buf, &p)
					continue
				}
			}
			n *= sign
			coord[i] = float32(float64(n) / 8.0)

			locSkipBlanks(buf, &p)
		}

		if at2(buf, p) == 0 {
			break
		}

		// parse location name
		overflow = false
		location = location[:0]
	name:
		for {
			switch c := at2(buf, p); c {
			case '\r':
				p++
			case '\n', 0:
				TP_AddLocNode(coord, string(location))
				loc_numentries++
				if c == '\n' {
					p++
				}

				break name
			default:
				if len(location) < MAX_LOC_NAME-1 {
					location = append(location, c)
				} else if !overflow {
					overflow = true
					Com_Printf("Locfile warning (line %d): truncating loc name to %d chars\n", line, MAX_LOC_NAME-1)
				}
				p++
			}
		}
	}

	if loc_numentries != 0 {
		if !quiet {
			Com_Printf("Loaded locfile \"%s\" (%d loc points)\n", COM_SkipPath(locname), loc_numentries)
		}
	} else {
		TP_ClearLocs()
		if !quiet {
			Com_Printf("Locfile \"%s\" was empty\n", COM_SkipPath(locname))
		}
	}

	return true
}

func TP_LoadLocFile_f() {
	if Cmd_Argc() != 2 {
		Com_Printf("loadloc <filename> : load a loc file\n")
		return
	}
	TP_LoadLocFile(Cmd_Argv(1), false)
}

type locmacro_t struct {
	macro string
	val   string
}

var locmacros = []locmacro_t{
	{"ssg", "ssg"},
	{"ng", "ng"},
	{"sng", "sng"},
	{"gl", "gl"},
	{"rl", "rl"},
	{"lg", "lg"},
	{"separator", "-"},
	{"ga", "ga"},
	{"ya", "ya"},
	{"ra", "ra"},
	{"quad", "quad"},
	{"pent", "pent"},
	{"ring", "ring"},
	{"suit", "suit"},
	{"mh", "mega"},
}

var NUM_LOCMACROS = len(locmacros)

var tp_locationname_recursive bool

func TP_LocationName(location vec3_t) string {
	var in string
	var ii int
	var out []byte
	var value string
	var i int
	var dist, mindist float32
	var vec vec3_t
	var node, best *locdata_t
	var cvar *cvar_t

	if locdata == nil || cls.state != ca_active {
		return tp_name_someplace.string
	}

	if tp_locationname_recursive {
		return ""
	}

	best = nil
	mindist = 0

	for node = locdata; node != nil; node = node.next {
		VectorSubtract(location, node.coord, &vec)
		dist = vec[0]*vec[0] + vec[1]*vec[1] + vec[2]*vec[2]
		if best == nil || dist < mindist {
			best = node
			mindist = dist
		}
	}

	in = best.name
	for ii < len(in) && len(out) < MAX_LOC_NAME-1 {
		if Q_strncasecmp(in[ii:], "$loc_name_", 10) == 0 {
			ii += 10
			for i = 0; i < NUM_LOCMACROS; i++ {
				if Q_strncasecmp(in[ii:], locmacros[i].macro, uint(len(locmacros[i].macro))) == 0 {
					if cvar = Cvar_FindVar(va("loc_name_%s", locmacros[i].macro)); cvar != nil {
						value = cvar.string
					} else {
						value = locmacros[i].val
					}
					if len(out) >= MAX_LOC_NAME-len(value)-1 {
						goto done_locmacros
					}
					out = append(out, value...)
					ii += len(locmacros[i].macro)
					break
				}
			}
			if i == NUM_LOCMACROS {
				if len(out) >= MAX_LOC_NAME-10-1 {
					goto done_locmacros
				}
				out = append(out, "$loc_name_"...)
			}
		} else {
			out = append(out, in[ii])
			ii++
		}
	}
done_locmacros:

	tp_locationname_recursive = true
	buf := Cmd_ExpandString(string(out), 1024)
	tp_locationname_recursive = false

	return buf
}

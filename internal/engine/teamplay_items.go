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

// teamplay.c, second half: message triggers, match info, message
// categorising, item flags and tracking, pointing, stat and sound triggers,
// message filters, init.

import (
	"fmt"
	"io"
	"strings"
)

const (
	FPD_NO_FORCE_SKIN  = 256
	FPD_NO_FORCE_COLOR = 512
	FPD_LIMIT_PITCH    = 1 << 14
	FPD_LIMIT_YAW      = 1 << 15
)

/****************************** MESSAGE TRIGGERS ******************************/

type msg_trigger_t struct {
	name   string // 32
	string string // 64
	level  int
	next   *msg_trigger_t
}

var msg_triggers *msg_trigger_t

func TP_ResetAllTriggers() {
	msg_triggers = nil
}

func TP_DumpTriggers(f io.Writer) {
	var t *msg_trigger_t

	for t = msg_triggers; t != nil; t = t.next {
		if t.level == PRINT_HIGH {
			fmt.Fprintf(f, "msg_trigger  %s \"%s\"\n", t.name, t.string)
		} else {
			level := byte('0' + t.level)
			if t.level == 4 {
				level = 't'
			}
			fmt.Fprintf(f, "msg_trigger  %s \"%s\" -l %c\n", t.name, t.string, level)
		}
	}
}

func TP_FindTrigger(name string) *msg_trigger_t {
	var t *msg_trigger_t

	for t = msg_triggers; t != nil; t = t.next {
		if t.name == name {
			return t
		}
	}

	return nil
}

func TP_MsgTrigger_f() {
	var c int
	var name string
	var trig *msg_trigger_t

	c = Cmd_Argc()

	if c > 5 {
		Com_Printf("msg_trigger <trigger name> \"string\" [-l <level>]\n")
		return
	}

	if c == 1 {
		if msg_triggers == nil {
			Com_Printf("no triggers defined\n")
		} else {
			for trig = msg_triggers; trig != nil; trig = trig.next {
				Com_Printf("%s : \"%s\"\n", trig.name, trig.string)
			}
		}
		return
	}

	name = Cmd_Argv(1)
	if len(name) > 31 {
		Com_Printf("trigger name too long\n")
		return
	}

	if c == 2 {
		trig = TP_FindTrigger(name)
		if trig != nil {
			Com_Printf("%s: \"%s\"\n", trig.name, trig.string)
		} else {
			Com_Printf("trigger \"%s\" not found\n", name)
		}
		return
	}

	if c >= 3 {
		if len(Cmd_Argv(2)) > 63 {
			Com_Printf("trigger string too long\n")
			return
		}

		if trig = TP_FindTrigger(name); trig == nil {
			// allocate new trigger
			trig = &msg_trigger_t{}
			trig.next = msg_triggers
			msg_triggers = trig
			trig.name = name
			trig.level = PRINT_HIGH
		}

		trig.string = strlcpy(Cmd_Argv(2), 64)
		if c == 5 && Q_strcasecmp(Cmd_Argv(3), "-l") == 0 {
			if Cmd_Argv(4) == "t" {
				trig.level = 4
			} else {
				trig.level = Q_atoi(Cmd_Argv(4))
				if uint(trig.level) > PRINT_CHAT {
					trig.level = PRINT_HIGH
				}
			}
		}
	}
}

// the source has U+FFFD where the original high-bit Quake characters were
func TP_IsFlagMessage(message string) bool {
	if strings.Contains(message, " has your key!") ||
		strings.Contains(message, " has taken your Key") ||
		strings.Contains(message, " has your flag") ||
		strings.Contains(message, " took your flag!") ||
		strings.Contains(message, " ���� ���� flag!") ||
		strings.Contains(message, " ��� ���� ����") || strings.Contains(message, " ��� ����� ����") ||
		strings.Contains(message, " took the blue flag") || strings.Contains(message, " took the red flag") ||
		strings.Contains(message, " Has the Red Flag") || strings.Contains(message, " Has the Blue Flag") {
		return true
	}

	return false
}

func TP_SearchForMsgTriggers(s string, level int) {
	var t *msg_trigger_t

	if cls.demoplayback {
		return
	}
	if tp_msgtriggers.value == 0 || !Ruleset_AllowMsgTriggers() {
		return
	}

	for t = msg_triggers; t != nil; t = t.next {
		if (t.level == level || (t.level == 3 && level == 4)) && t.string != "" && strings.Contains(s, t.string) {
			if level == PRINT_CHAT && (strings.Contains(s, "f_version") || strings.Contains(s, "f_system") ||
				strings.Contains(s, "f_server") || strings.Contains(s, "f_speed") || strings.Contains(s, "f_modified")) {
				continue // don't let llamas fake proxy replies
			}

			if cl.teamfortress && level == PRINT_HIGH && TP_IsFlagMessage(s) {
				continue
			}

			if str, ok := Cmd_AliasString(t.name); ok {
				vars.lasttrigger_match = strlcpy(s, 256)
				Cbuf_AddTextEx(cbuf_safe, str)
			} else {
				Com_Printf("trigger \"%s\" has no matching alias\n", t.name)
			}
		}
	}
}

/************************* BASIC MATCH INFO FUNCTIONS *************************/

func TP_PlayerName() string {
	return Info_ValueForKey(cl.players[cl.playernum].userinfo, "name")
}

func TP_PlayerTeam() string {
	return cl.players[cl.playernum].team
}

func TP_CountPlayers() int {
	var i int
	count := 0

	for i = 0; i < MAX_CLIENTS; i++ {
		if cl.players[i].name != "" && cl.players[i].spectator == 0 {
			count++
		}
	}
	return count
}

func TP_MapName() string {
	return mapname.string
}

func TP_GetMapGroupName(mapname string, system *bool) string {
	return MT_GetMapGroupName(mapname, system)
}

/****************************** PUBLIC FUNCTIONS ******************************/

var tp_last_map string

func TP_NewMap() {
	var groupname, mapname_ string
	var system bool

	vars = tvars_t{}
	TP_FindModelNumbers()

	mapname_ = TP_MapName()
	if mapname_ != tp_last_map { // map name has changed
		TP_ClearLocs() // clear loc file
		if tp_loadlocs.value != 0 && cl.deathmatch != 0 && !cls.demoplayback {
			if !TP_LoadLocFile(va("%s.loc", mapname_), true) {
				if groupname = TP_GetMapGroupName(mapname_, &system); groupname != "" && !system {
					TP_LoadLocFile(va("%s.loc", groupname), true)
				}
			}
			tp_last_map = strlcpy(mapname_, MAX_QPATH)
		} else {
			tp_last_map = ""
		}
	}
	TP_ExecTrigger("f_newmap")
	Ignore_ResetFloodList()
}

/*
returns a combination of these values:
0 -- unknown (probably generated by the server)
1 -- normal
2 -- team message
4 -- spectator
8 -- spec team message
Note that sometimes we can't be sure who really sent the message,  e.g. when there's a
player "unnamed" in your team and "(unnamed)" in the enemy team. The result will be 3 (1+2)
*/
func TP_CategorizeMessage(s string, offset *int) int {
	var i, msglen, len_, flags, tracknum int
	var player *player_info_t
	var name, team string

	tracknum = -1
	if cl.spectator != 0 {
		tracknum = Cam_TrackNum()
	}
	if cl.spectator != 0 && tracknum != -1 {
		team = cl.players[tracknum].team
	} else if cl.spectator == 0 {
		team = cl.players[cl.playernum].team
	} else {
		team = ""
	}

	flags = 0
	*offset = 0
	if msglen = len(s); msglen == 0 {
		return 0
	}

	for i = 0; i < MAX_CLIENTS; i++ {
		player = &cl.players[i]
		if player.name == "" {
			continue
		}
		name = Info_ValueForKey(player.userinfo, "name")
		len_ = bound(0, len(name), 31)
		// check messagemode1
		if len_+2 <= msglen && s[len_] == ':' && s[len_+1] == ' ' && name[:len_] == s[:len_] {
			if player.spectator != 0 {
				flags |= 4
			} else {
				flags |= 1
			}
			*offset = len_ + 2
		} else if s[0] == '(' && len_+4 <= msglen && s[len_+1:len_+4] == "): " && name[:len_] == s[1:1+len_] &&
			(cl.spectator == 0 || tracknum != -1) { // check messagemode2
			// no team messages in teamplay 0, except for our own
			if i == cl.playernum || (cl.teamplay != 0 && team == player.team) {
				flags |= 2
			}
			*offset = len_ + 4
		} else if cl.spectator != 0 && strings.HasPrefix(s, "[SPEC] ") && player.spectator != 0 &&
			len_+9 <= msglen && s[len_+7] == ':' && s[len_+8] == ' ' && name[:len_] == s[7:7+len_] { // check spec mm2
			flags |= 8
			*offset = len_ + 9
		}
	}
	return flags
}

/****************************** POINTING & TOOK ******************************/

// symbolic names used in tp_took, tp_pickup, tp_point commands
var pknames = []string{"quad", "pent", "ring", "suit", "ra", "ya", "ga",
	"mh", "health", "lg", "rl", "gl", "sng", "ng", "ssg", "pack",
	"cells", "rockets", "nails", "shells", "flag",
	"teammate", "enemy", "eyes", "sentry", "disp", "runes"}

const (
	it_quad     = 1 << 0
	it_pent     = 1 << 1
	it_ring     = 1 << 2
	it_suit     = 1 << 3
	it_ra       = 1 << 4
	it_ya       = 1 << 5
	it_ga       = 1 << 6
	it_mh       = 1 << 7
	it_health   = 1 << 8
	it_lg       = 1 << 9
	it_rl       = 1 << 10
	it_gl       = 1 << 11
	it_sng      = 1 << 12
	it_ng       = 1 << 13
	it_ssg      = 1 << 14
	it_pack     = 1 << 15
	it_cells    = 1 << 16
	it_rockets  = 1 << 17
	it_nails    = 1 << 18
	it_shells   = 1 << 19
	it_flag     = 1 << 20
	it_teammate = 1 << 21
	it_enemy    = 1 << 22
	it_eyes     = 1 << 23
	it_sentry   = 1 << 24
	it_disp     = 1 << 25
	it_runes    = 1 << 26

	NUM_ITEMFLAGS = 27

	it_powerups = it_quad | it_pent | it_ring
	it_weapons  = it_lg | it_rl | it_gl | it_sng | it_ng | it_ssg
	it_armor    = it_ra | it_ya | it_ga
	it_ammo     = it_cells | it_rockets | it_nails | it_shells
	it_players  = it_teammate | it_enemy | it_eyes

	default_pkflags = it_powerups | it_suit | it_armor | it_weapons | it_mh |
		it_rockets | it_pack | it_flag

	default_tookflags = it_powerups | it_ra | it_ya | it_lg | it_rl | it_mh | it_flag

	default_pointflags = it_powerups | it_suit | it_armor | it_mh |
		it_lg | it_rl | it_gl | it_sng | it_rockets | it_pack | it_flag | it_players
)

var pkflags = default_pkflags
var tookflags = default_tookflags
var pointflags = default_pointflags

func DumpFlagCommand(f io.Writer, name string, flags, default_flags int) {
	var i int
	all_flags := (1 << NUM_ITEMFLAGS) - 1

	fmt.Fprintf(f, "%s ", name)

	if flags == 0 {
		fmt.Fprintf(f, "none\n")
		return
	}
	if flags == all_flags {
		fmt.Fprintf(f, "all\n")
		return
	} else if flags == default_flags {
		fmt.Fprintf(f, "default\n")
		return
	}

	if flags&it_powerups == it_powerups {
		fmt.Fprintf(f, "powerups ")
		flags &^= it_powerups
	}
	if flags&it_weapons == it_weapons {
		fmt.Fprintf(f, "weapons ")
		flags &^= it_weapons
	}
	if flags&it_armor == it_armor {
		fmt.Fprintf(f, "armor ")
		flags &^= it_armor
	}
	if flags&it_ammo == it_ammo {
		fmt.Fprintf(f, "ammo ")
		flags &^= it_ammo
	}
	if flags&it_players == it_players {
		fmt.Fprintf(f, "players ")
		flags &^= it_players
	}
	for i = 0; i < NUM_ITEMFLAGS; i++ {
		if flags&(1<<i) != 0 {
			fmt.Fprintf(f, "%s ", pknames[i])
		}
	}
	fmt.Fprintf(f, "\n")
}

func DumpFlagCommands(f io.Writer) {
	DumpFlagCommand(f, "tp_pickup   ", pkflags, default_pkflags)
	DumpFlagCommand(f, "tp_took     ", tookflags, default_tookflags)
	DumpFlagCommand(f, "tp_point    ", pointflags, default_pointflags)
}

func FlagCommand(flags *int, defaultflags int) {
	var i, j, c, flag int
	var p, str string
	removeflag := false

	c = Cmd_Argc()
	if c == 1 {
		if *flags == 0 {
			str = "none"
		}
		for i = 0; i < NUM_ITEMFLAGS; i++ {
			if *flags&(1<<i) != 0 {
				if str != "" {
					str = strlcat(str, " ", 255)
				}
				str = strlcat(str, pknames[i], 255)
			}
		}
		Com_Printf("%s\n", str)
		return
	}

	if c == 2 && Q_strcasecmp(Cmd_Argv(1), "none") == 0 {
		*flags = 0
		return
	}

	if at(Cmd_Argv(1), 0) != '+' && at(Cmd_Argv(1), 0) != '-' {
		*flags = 0
	}

	for i = 1; i < c; i++ {
		p = Cmd_Argv(i)
		if at(p, 0) == '+' {
			removeflag = false
			p = p[1:]
		} else if at(p, 0) == '-' {
			removeflag = true
			p = p[1:]
		}

		flag = 0
		for j = 0; j < NUM_ITEMFLAGS; j++ {
			if Q_strncasecmp(p, pknames[j], 3) == 0 {
				flag = 1 << j
				break
			}
		}

		if flag == 0 {
			if Q_strcasecmp(p, "armor") == 0 {
				flag = it_armor
			} else if Q_strcasecmp(p, "weapons") == 0 {
				flag = it_weapons
			} else if Q_strcasecmp(p, "powerups") == 0 {
				flag = it_powerups
			} else if Q_strcasecmp(p, "ammo") == 0 {
				flag = it_ammo
			} else if Q_strcasecmp(p, "players") == 0 {
				flag = it_players
			} else if Q_strcasecmp(p, "default") == 0 {
				flag = defaultflags
			} else if Q_strcasecmp(p, "all") == 0 {
				flag = (1 << NUM_ITEMFLAGS) - 1
			}
		}

		if flags != &pointflags {
			flag &^= it_sentry | it_disp | it_players
		}

		if removeflag {
			*flags &^= flag
		} else {
			*flags |= flag
		}
	}
}

func TP_Took_f() {
	FlagCommand(&tookflags, default_tookflags)
}

func TP_Pickup_f() {
	FlagCommand(&pkflags, default_pkflags)
}

func TP_Point_f() {
	FlagCommand(&pointflags, default_pointflags)
}

type item_t struct {
	itemflag  int
	cvar      *cvar_t
	modelname string
	offset    vec3_t  // offset of model graphics center
	radius    float32 // model graphics radius
	flags     int     // TODO: "NOPICKUP" (disp), "TEAMENEMY" (flag, disp)
}

var tp_items = []item_t{
	{it_quad, &tp_name_quad, "progs/quaddama.mdl", vec3_t{0, 0, 24}, 25, 0},
	{it_pent, &tp_name_pent, "progs/invulner.mdl", vec3_t{0, 0, 22}, 25, 0},
	{it_ring, &tp_name_ring, "progs/invisibl.mdl", vec3_t{0, 0, 16}, 12, 0},
	{it_suit, &tp_name_suit, "progs/suit.mdl", vec3_t{0, 0, 24}, 20, 0},
	{it_lg, &tp_name_lg, "progs/g_light.mdl", vec3_t{0, 0, 30}, 20, 0},
	{it_rl, &tp_name_rl, "progs/g_rock2.mdl", vec3_t{0, 0, 30}, 20, 0},
	{it_gl, &tp_name_gl, "progs/g_rock.mdl", vec3_t{0, 0, 30}, 20, 0},
	{it_sng, &tp_name_sng, "progs/g_nail2.mdl", vec3_t{0, 0, 30}, 20, 0},
	{it_ng, &tp_name_ng, "progs/g_nail.mdl", vec3_t{0, 0, 30}, 20, 0},
	{it_ssg, &tp_name_ssg, "progs/g_shot.mdl", vec3_t{0, 0, 30}, 20, 0},
	{it_cells, &tp_name_cells, "maps/b_batt0.bsp", vec3_t{16, 16, 24}, 18, 0},
	{it_cells, &tp_name_cells, "maps/b_batt1.bsp", vec3_t{16, 16, 24}, 18, 0},
	{it_rockets, &tp_name_rockets, "maps/b_rock0.bsp", vec3_t{8, 8, 20}, 18, 0},
	{it_rockets, &tp_name_rockets, "maps/b_rock1.bsp", vec3_t{16, 8, 20}, 18, 0},
	{it_nails, &tp_name_nails, "maps/b_nail0.bsp", vec3_t{16, 16, 10}, 18, 0},
	{it_nails, &tp_name_nails, "maps/b_nail1.bsp", vec3_t{16, 16, 10}, 18, 0},
	{it_shells, &tp_name_shells, "maps/b_shell0.bsp", vec3_t{16, 16, 10}, 18, 0},
	{it_shells, &tp_name_shells, "maps/b_shell1.bsp", vec3_t{16, 16, 10}, 18, 0},
	{it_health, &tp_name_health, "maps/b_bh10.bsp", vec3_t{16, 16, 8}, 18, 0},
	{it_health, &tp_name_health, "maps/b_bh25.bsp", vec3_t{16, 16, 8}, 18, 0},
	{it_mh, &tp_name_mh, "maps/b_bh100.bsp", vec3_t{16, 16, 14}, 20, 0},
	{it_pack, &tp_name_backpack, "progs/backpack.mdl", vec3_t{0, 0, 18}, 18, 0},
	{it_flag, &tp_name_flag, "progs/tf_flag.mdl", vec3_t{0, 0, 14}, 25, 0},
	{it_flag, &tp_name_flag, "progs/tf_stan.mdl", vec3_t{0, 0, 45}, 40, 0},
	{it_flag, &tp_name_flag, "progs/w_g_key.mdl", vec3_t{0, 0, 20}, 18, 0},
	{it_flag, &tp_name_flag, "progs/w_s_key.mdl", vec3_t{0, 0, 20}, 18, 0},
	{it_flag, &tp_name_flag, "progs/m_g_key.mdl", vec3_t{0, 0, 20}, 18, 0},
	{it_flag, &tp_name_flag, "progs/m_s_key.mdl", vec3_t{0, 0, 20}, 18, 0},
	{it_flag, &tp_name_flag, "progs/b_s_key.mdl", vec3_t{0, 0, 20}, 18, 0},
	{it_flag, &tp_name_flag, "progs/b_g_key.mdl", vec3_t{0, 0, 20}, 18, 0},
	{it_flag, &tp_name_flag, "progs/flag.mdl", vec3_t{0, 0, 14}, 25, 0},
	{it_runes, &tp_name_rune1, "progs/end1.mdl", vec3_t{0, 0, 20}, 18, 0},
	{it_runes, &tp_name_rune2, "progs/end2.mdl", vec3_t{0, 0, 20}, 18, 0},
	{it_runes, &tp_name_rune3, "progs/end3.mdl", vec3_t{0, 0, 20}, 18, 0},
	{it_runes, &tp_name_rune4, "progs/end4.mdl", vec3_t{0, 0, 20}, 18, 0},
	{it_ra | it_ya | it_ga, nil, "progs/armor.mdl", vec3_t{0, 0, 24}, 22, 0},
	{it_sentry, &tp_name_sentry, "progs/turrgun.mdl", vec3_t{0, 0, 23}, 25, 0},
	{it_disp, &tp_name_disp, "progs/disp.mdl", vec3_t{0, 0, 24}, 25, 0},
}

var NUMITEMS = len(tp_items)

var model2item [MAX_MODELS]*item_t

func TP_FindModelNumbers() {
	var i, j int
	var s string

	for i = 0; i < MAX_MODELS; i++ {
		model2item[i] = nil
		s = cl.model_name[i]
		if s == "" {
			continue
		}
		for j = 0; j < NUMITEMS; j++ {
			if s == tp_items[j].modelname {
				model2item[i] = &tp_items[j]
			}
		}
	}
}

// on success, result is non-zero
// on failure, result is zero
// for armors, returns skinnum+1 on success
func FindNearestItem(flags int, pitem **item_t) int {
	var frame *frame_t
	var pak *packet_entities_t
	var ent, bestent *entity_state_t
	var i int
	var bestdist, dist float32
	var org, v vec3_t
	var item *item_t

	org = cl.frames[cl.validsequence&UPDATE_MASK].playerstate[cl.playernum].origin

	// look in previous frame
	frame = &cl.frames[cl.oldvalidsequence&UPDATE_MASK]
	pak = &frame.packet_entities
	bestdist = 100 * 100
	*pitem = nil
	for i = 0; i < pak.num_entities; i++ {
		ent = &pak.entities[i]
		item = model2item[ent.modelindex]
		if item == nil || item.itemflag&flags == 0 {
			continue
		}

		VectorSubtract(ent.origin, org, &v)
		VectorAdd(v, item.offset, &v)
		dist = DotProduct(v, v)

		if dist <= bestdist {
			bestdist = dist
			bestent = ent
			*pitem = item
		}
	}

	if bestent != nil {
		vars.nearestitemloc = strlcpy(TP_LocationName(bestent.origin), MAX_LOC_NAME)
	} else {
		vars.nearestitemloc = ""
	}

	if bestent != nil && (*pitem).itemflag == it_armor {
		return bestent.skinnum + 1 // 1 = green, 2 = yellow, 3 = red
	}

	if bestent != nil {
		return bestent.modelindex
	}
	return 0
}

func Macro_LastTookOrPointed() string {
	if vars.tooktime != 0 && vars.tooktime > vars.pointtime && cls.realtime-vars.tooktime < 5 {
		return Macro_TookAtLoc()
	} else if vars.pointtime != 0 && vars.tooktime <= vars.pointtime && cls.realtime-vars.pointtime < 5 {
		return Macro_LastPointAtLoc()
	}

	macro_buf = snprintf(MAX_MACRO_VALUE, "%s %s %s", tp_name_nothing.string, tp_name_at.string, tp_name_someplace.string)

	return macro_buf
}

func CheckTrigger() bool {
	var i, count int
	var player *player_info_t
	var myteam string

	if cl.spectator != 0 {
		return false
	}

	if tp_forceTriggers.value != 0 {
		return true
	}

	if cl.teamplay == 0 {
		return false
	}

	count = 0
	myteam = cl.players[cl.playernum].team
	for i = 0; i < MAX_CLIENTS; i++ {
		player = &cl.players[i]
		if player.name != "" && player.spectator == 0 && i != cl.playernum && player.team == myteam {
			count++
		}
	}

	return count != 0
}

func ExecTookTrigger(s string, flag int, org vec3_t) {
	var pkflags_dmm, tookflags_dmm int

	pkflags_dmm = pkflags
	tookflags_dmm = tookflags

	if !cl.teamfortress && cl.deathmatch >= 1 && cl.deathmatch <= 4 {
		if cl.deathmatch == 4 {
			pkflags_dmm &^= it_ammo | it_weapons
			tookflags_dmm &^= it_ammo | it_weapons
		}
	}
	if (pkflags_dmm|tookflags_dmm)&flag == 0 {
		return
	}

	vars.tooktime = cls.realtime
	vars.tookname = strlcpy(s, 32)
	vars.tookloc = strlcpy(TP_LocationName(org), MAX_LOC_NAME)

	if tookflags_dmm&flag != 0 && CheckTrigger() {
		TP_ExecTrigger("f_took")
	}
}

// info - cl.players
func playerSlot(info *player_info_t) int {
	for i := range cl.players {
		if &cl.players[i] == info {
			return i
		}
	}
	return -1
}

func TP_ParsePlayerInfo(oldstate, state *player_state_t, info *player_info_t) {
	if state.effects&(EF_BLUE|EF_RED) != oldstate.effects&(EF_BLUE|EF_RED) {
		TP_CalculateSkinForPlayer(uint(playerSlot(info)))
	}

	if cl.spectator == 0 && cl.teamplay != 0 && info.team != TP_PlayerTeam() {
		eyes := state.modelindex != 0 && cl.model_precache[state.modelindex] != nil && cl.model_precache[state.modelindex].modhint == MOD_EYES

		if state.effects&(EF_BLUE|EF_RED) != 0 || eyes {
			vars.enemy_powerups = 0
			vars.enemy_powerups_time = cls.realtime

			if state.effects&EF_BLUE != 0 {
				vars.enemy_powerups |= TP_QUAD
			}
			if state.effects&EF_RED != 0 {
				vars.enemy_powerups |= TP_PENT
			}
			if eyes {
				vars.enemy_powerups |= TP_RING
			}
		}
	}
	if cl.spectator == 0 && !cl.teamfortress && playerSlot(info) == cl.playernum {
		if state.effects&(EF_FLAG1|EF_FLAG2) != 0 && oldstate.effects&(EF_FLAG1|EF_FLAG2) == 0 {
			ExecTookTrigger(tp_name_flag.string, it_flag, cl.frames[cl.validsequence&UPDATE_MASK].playerstate[cl.playernum].origin)
		} else if state.effects&(EF_FLAG1|EF_FLAG2) == 0 && oldstate.effects&(EF_FLAG1|EF_FLAG2) != 0 {
			vars.lastdrop_time = cls.realtime
			vars.lastdroploc = Macro_Location()
		}
	}
}

func TP_CheckPickupSound(s string, org vec3_t) {
	var item *item_t

	if cl.spectator != 0 {
		return
	}

	switch {
	case s == "items/damage.wav":
		ExecTookTrigger(tp_name_quad.string, it_quad, org)
		return
	case s == "items/protect.wav":
		ExecTookTrigger(tp_name_pent.string, it_pent, org)
		return
	case s == "items/inv1.wav":
		ExecTookTrigger(tp_name_ring.string, it_ring, org)
		return
	case s == "items/suit.wav":
		ExecTookTrigger(tp_name_suit.string, it_suit, org)
		return
	case s == "items/health1.wav" || s == "items/r_item1.wav":
		ExecTookTrigger(tp_name_health.string, it_health, org)
		return
	case s == "items/r_item2.wav":
		ExecTookTrigger(tp_name_mh.string, it_mh, org)
		return
	}

	// more:
	if cl.validsequence == 0 || cl.oldvalidsequence == 0 {
		return
	}

	// weapons
	if s == "weapons/pkup.wav" {
		if FindNearestItem(it_weapons, &item) != 0 {
			ExecTookTrigger(item.cvar.string, item.itemflag, org)
		} else if vars.stat_framecounts[STAT_ITEMS] == cls.framecount {
			if vars.items&^vars.olditems&IT_LIGHTNING != 0 {
				ExecTookTrigger(tp_name_lg.string, it_lg, cl.simorg)
			} else if vars.items&^vars.olditems&IT_ROCKET_LAUNCHER != 0 {
				ExecTookTrigger(tp_name_rl.string, it_rl, cl.simorg)
			} else if vars.items&^vars.olditems&IT_GRENADE_LAUNCHER != 0 {
				ExecTookTrigger(tp_name_gl.string, it_gl, cl.simorg)
			} else if vars.items&^vars.olditems&IT_SUPER_NAILGUN != 0 {
				ExecTookTrigger(tp_name_sng.string, it_sng, cl.simorg)
			} else if vars.items&^vars.olditems&IT_NAILGUN != 0 {
				ExecTookTrigger(tp_name_ng.string, it_ng, cl.simorg)
			} else if vars.items&^vars.olditems&IT_SUPER_SHOTGUN != 0 {
				ExecTookTrigger(tp_name_ssg.string, it_ssg, cl.simorg)
			}
		}
		return
	}

	// armor
	if s == "items/armor1.wav" {
		var armor_updated bool
		var armortype int

		armor_updated = (vars.stat_framecounts[STAT_ARMOR] == cls.framecount)
		armortype = FindNearestItem(it_armor, &item)
		if armortype == 1 || (armortype == 0 && armor_updated && cl.stats[STAT_ARMOR] == 100) {
			ExecTookTrigger(tp_name_ga.string, it_ga, org)
		} else if armortype == 2 || (armortype == 0 && armor_updated && cl.stats[STAT_ARMOR] == 150) {
			ExecTookTrigger(tp_name_ya.string, it_ya, org)
		} else if armortype == 3 || (armortype == 0 && armor_updated && cl.stats[STAT_ARMOR] == 200) {
			ExecTookTrigger(tp_name_ra.string, it_ra, org)
		}
		return
	}

	// backpack, ammo or runes
	if s == "weapons/lock4.wav" {
		if FindNearestItem(it_ammo|it_pack|it_runes, &item) == 0 {
			return
		}
		ExecTookTrigger(item.cvar.string, item.itemflag, org)
	}
}

type item_vis_t struct {
	vieworg vec3_t
	forward vec3_t
	right   vec3_t
	up      vec3_t
	entorg  vec3_t
	radius  float32
	dir     vec3_t
	dist    float32
}

func TP_IsItemVisible(visitem *item_vis_t) bool {
	var end, v vec3_t

	if visitem.dist <= visitem.radius {
		return true
	}

	VectorNegate(visitem.dir, &v)
	VectorNormalizeFast(&v)
	VectorMA(visitem.entorg, visitem.radius, v, &end)
	trace := PM_TraceLine(visitem.vieworg, end)
	if trace.fraction == 1 {
		return true
	}

	VectorMA(visitem.entorg, visitem.radius, visitem.right, &end)
	VectorSubtract(visitem.vieworg, end, &v)
	VectorNormalizeFast(&v)
	VectorMA(end, visitem.radius, v, &end)
	trace = PM_TraceLine(visitem.vieworg, end)
	if trace.fraction == 1 {
		return true
	}

	VectorMA(visitem.entorg, -visitem.radius, visitem.right, &end)
	VectorSubtract(visitem.vieworg, end, &v)
	VectorNormalizeFast(&v)
	VectorMA(end, visitem.radius, v, &end)
	trace = PM_TraceLine(visitem.vieworg, end)
	if trace.fraction == 1 {
		return true
	}

	VectorMA(visitem.entorg, visitem.radius, visitem.up, &end)
	VectorSubtract(visitem.vieworg, end, &v)
	VectorNormalizeFast(&v)
	VectorMA(end, visitem.radius, v, &end)
	trace = PM_TraceLine(visitem.vieworg, end)
	if trace.fraction == 1 {
		return true
	}

	// use half the radius, otherwise it's possible to see through floor in some places
	VectorMA(visitem.entorg, -visitem.radius/2, visitem.up, &end)
	VectorSubtract(visitem.vieworg, end, &v)
	VectorNormalizeFast(&v)
	VectorMA(end, visitem.radius, v, &end)
	trace = PM_TraceLine(visitem.vieworg, end)
	return trace.fraction == 1
}

func TP_RankPoint(visitem *item_vis_t) float32 {
	var v2, v3 vec3_t
	var miss float32

	if visitem.dist < 10 {
		return -1
	}

	VectorScale(visitem.forward, visitem.dist, &v2)
	VectorSubtract(v2, visitem.dir, &v3)
	miss = VectorLength(v3)
	if miss > 300 {
		return -1
	}
	if float64(miss) > float64(visitem.dist)*1.7 {
		return -1 // over 60 degrees off
	}

	if visitem.dist < 3000.0/8.0 {
		return float32(float64(miss) * (float64(visitem.dist)*8.0*float64(float32(0.0002)) + float64(float32(0.3))))
	}
	return miss
}

func TP_FindPoint() {
	var pak *packet_entities_t
	var ent *entity_state_t
	var i, j, pointflags_dmm int
	var best, rank float32 = -1, 0
	var bestent *entity_state_t
	var ang vec3_t
	var item, bestitem *item_t
	var state, beststate *player_state_t
	var info, bestinfo *player_info_t
	var visitem item_vis_t

	if vars.pointtime == cls.realtime {
		return
	}

	nothing := func() {
		vars.pointname = strlcpy(tp_name_nothing.string, 32)
		vars.pointloc = ""
		vars.pointtype = POINT_TYPE_ITEM
	}

	if cl.validsequence == 0 {
		nothing()
		vars.pointtime = cls.realtime
		return
	}

	ang[0] = cl.viewangles[0]
	ang[1] = cl.viewangles[1]
	ang[2] = 0
	AngleVectors(ang, &visitem.forward, &visitem.right, &visitem.up)
	visitem.vieworg = cl.simorg
	var viewheight float32
	if v_viewheight.value != 0 {
		viewheight = bound(-7, v_viewheight.value, 4)
	}
	visitem.vieworg[2] += 22 + viewheight

	pointflags_dmm = pointflags
	if !cl.teamfortress && cl.deathmatch >= 1 && cl.deathmatch <= 4 {
		if cl.deathmatch == 4 {
			pointflags_dmm &^= it_ammo
		}
		if cl.deathmatch != 1 {
			pointflags_dmm &^= it_weapons
		}
	}

	pak = &cl.frames[cl.validsequence&UPDATE_MASK].packet_entities
	for i = 0; i < pak.num_entities; i++ {
		ent = &pak.entities[i]
		item = model2item[ent.modelindex]
		if item == nil || item.itemflag&pointflags_dmm == 0 {
			continue
		}
		// special check for armors; the C switch falls through, so green needs
		// all three flags and yellow the last two
		if item.itemflag == (it_ra | it_ya | it_ga) {
			if ent.skinnum == 0 && pointflags_dmm&it_ga == 0 {
				continue
			}
			if (ent.skinnum == 0 || ent.skinnum == 1) && pointflags_dmm&it_ya == 0 {
				continue
			}
			if pointflags_dmm&it_ra == 0 {
				continue
			}
		}

		VectorAdd(ent.origin, item.offset, &visitem.entorg)
		VectorSubtract(visitem.entorg, visitem.vieworg, &visitem.dir)
		visitem.dist = DotProduct(visitem.dir, visitem.forward)
		if ent.effects&(EF_BLUE|EF_RED|EF_DIMLIGHT|EF_BRIGHTLIGHT) != 0 {
			visitem.radius = 200
		} else {
			visitem.radius = item.radius
		}

		if rank = TP_RankPoint(&visitem); rank < 0 {
			continue
		}

		// check if we can actually see the object
		if (rank < best || best < 0) && TP_IsItemVisible(&visitem) {
			best = rank
			bestent = ent
			bestitem = item
		}
	}

	for j = 0; j < MAX_CLIENTS; j++ {
		state = &cl.frames[cl.parsecount&UPDATE_MASK].playerstate[j]
		info = &cl.players[j]
		if state.messagenum != cl.parsecount || j == cl.playernum || info.spectator != 0 {
			continue
		}

		if (state.modelindex == cl_modelindices[mi_player] && ISDEAD(state.frame)) ||
			state.modelindex == cl_modelindices[mi_h_player] {
			continue
		}

		visitem.entorg = state.origin
		visitem.entorg[2] += 30
		VectorSubtract(visitem.entorg, visitem.vieworg, &visitem.dir)
		visitem.dist = DotProduct(visitem.dir, visitem.forward)
		if state.effects&(EF_BLUE|EF_RED|EF_DIMLIGHT|EF_BRIGHTLIGHT) != 0 {
			visitem.radius = 200
		} else {
			visitem.radius = 27
		}

		if rank = TP_RankPoint(&visitem); rank < 0 {
			continue
		}

		// check if we can actually see the object
		if (rank < best || best < 0) && TP_IsItemVisible(&visitem) {
			if TP_ISEYESMODEL(state.modelindex) {
				if pointflags_dmm&it_eyes == 0 {
					continue
				}
			} else {
				if cl.teamplay != 0 && info.team == TP_PlayerTeam() {
					if pointflags_dmm&it_teammate == 0 {
						continue
					}
				} else {
					if pointflags_dmm&it_enemy == 0 {
						continue
					}
				}
			}

			best = rank
			bestinfo = info
			beststate = state
		}
	}

	if best >= 0 && bestinfo != nil {
		var teammate, eyes bool
		var name, buf string

		eyes = beststate.modelindex != 0 && cl.model_precache[beststate.modelindex] != nil &&
			cl.model_precache[beststate.modelindex].modhint == MOD_EYES
		if cl.teamfortress {
			teammate = Utils_TF_ColorToTeam(int(bestinfo.real_bottomcolor)) == TP_PlayerTeam()

			if eyes {
				name = tp_name_eyes.string // duck on 2night2
			} else if cl.spectator != 0 {
				name = bestinfo.name
			} else if teammate {
				if tp_name_teammate.string != "" {
					name = tp_name_teammate.string
				} else {
					name = "teammate"
				}
			} else {
				name = tp_name_enemy.string
			}

			if !eyes {
				sep := ""
				if name != "" {
					sep = " "
				}
				name = va("%s%s%s", name, sep, Skin_To_TFSkin(Info_ValueForKey(bestinfo.userinfo, "skin")))
			}
		} else {
			teammate = (cl.teamplay != 0 && bestinfo.team == TP_PlayerTeam())

			if eyes {
				name = tp_name_eyes.string
			} else if cl.spectator != 0 || (teammate && tp_name_teammate.string == "") {
				name = bestinfo.name
			} else if teammate {
				name = tp_name_teammate.string
			} else {
				name = tp_name_enemy.string
			}
		}
		if beststate.effects&EF_BLUE != 0 {
			buf = strlcat(buf, tp_name_quaded.string, 256)
		}
		if beststate.effects&EF_RED != 0 {
			sep := ""
			if buf != "" {
				sep = " "
			}
			buf = strlcat(buf, va("%s%s", sep, tp_name_pented.string), 256)
		}
		sep := ""
		if buf != "" {
			sep = " "
		}
		buf = strlcat(buf, va("%s%s", sep, name), 256)
		vars.pointname = strlcpy(buf, 32)
		vars.pointloc = strlcpy(TP_LocationName(beststate.origin), MAX_LOC_NAME)

		if teammate && !eyes {
			vars.pointtype = POINT_TYPE_TEAMMATE
		} else {
			vars.pointtype = POINT_TYPE_ENEMY
		}
	} else if best >= 0 {
		var p string

		if bestitem.cvar == nil {
			// armors are special
			switch bestent.skinnum {
			case 0:
				p = tp_name_ga.string
			case 1:
				p = tp_name_ya.string
			default:
				p = tp_name_ra.string
			}
		} else {
			p = bestitem.cvar.string
		}

		if bestitem.itemflag&(it_powerups|it_flag) != 0 {
			vars.pointtype = POINT_TYPE_POWERUP
		} else {
			vars.pointtype = POINT_TYPE_ITEM
		}
		vars.pointname = strlcpy(p, 32)
		vars.pointloc = strlcpy(TP_LocationName(bestent.origin), MAX_LOC_NAME)
	} else {
		nothing()
	}
	vars.pointtime = cls.realtime
}

func TP_ISEYESMODEL(x int) bool {
	return x != 0 && cl.model_precache[x] != nil && cl.model_precache[x].modhint == MOD_EYES
}

var tp_last_weaponmodel *model_t

func TP_ParseWeaponModel(model *model_t) {
	if cl.teamfortress && (cl.spectator == 0 || Cam_TrackNum() != -1) {
		if model != nil && tp_last_weaponmodel == nil {
			TP_ExecTrigger("f_reloadend")
		} else if model == nil && tp_last_weaponmodel != nil {
			TP_ExecTrigger("f_reloadstart")
		}
	}
	tp_last_weaponmodel = model
}

func TP_StatChanged(stat, value int) {
	effects := cl.frames[cl.parsecount&UPDATE_MASK].playerstate[cl.playernum].effects

	switch stat {
	case STAT_HEALTH:
		if value > 0 {
			if vars.health <= 0 {
				if cl.teamfortress {
					cshift_empty = cshift_t{}
				}
				if CheckTrigger() {
					TP_ExecTrigger("f_respawn")
				}
			}
			vars.health = value
			return
		}
		if vars.health > 0 { // We have just died
			vars.deathtrigger_time = cls.realtime
			vars.lastdeathloc = Macro_Location()

			CountNearbyPlayers(true)
			vars.last_numenemies = vars.numenemies
			vars.last_numfriendlies = vars.numfriendlies

			if CheckTrigger() {
				if cl.teamfortress && cl.stats[STAT_ITEMS]&(IT_KEY1|IT_KEY2) != 0 {
					TP_ExecTrigger("f_flagdeath")
				} else if effects&(EF_FLAG1|EF_FLAG2) != 0 {
					TP_ExecTrigger("f_flagdeath")
				} else {
					TP_ExecTrigger("f_death")
				}
			}
		}
		vars.health = value
	case STAT_ITEMS:
		if value&^vars.items&(IT_KEY1|IT_KEY2) != 0 {
			if cl.teamfortress && cl.spectator == 0 {
				ExecTookTrigger(tp_name_flag.string, it_flag, cl.frames[cl.validsequence&UPDATE_MASK].playerstate[cl.playernum].origin)
			}
		}
		if cl.spectator == 0 && cl.teamfortress && ^value&vars.items&(IT_KEY1|IT_KEY2) != 0 {
			vars.lastdrop_time = cls.realtime
			vars.lastdroploc = Macro_Location()
		}
		vars.olditems = vars.items
		vars.items = value
	case STAT_ACTIVEWEAPON:
		if cl.stats[STAT_ACTIVEWEAPON] != vars.activeweapon {
			TP_ExecTrigger("f_weaponchange")
			vars.activeweapon = cl.stats[STAT_ACTIVEWEAPON]
		}
	}
	vars.stat_framecounts[stat] = cls.framecount
}

/******************************* SOUND TRIGGERS *******************************/

// Find and execute sound triggers. A sound trigger must be terminated by either a CR or LF.
// Returns true if a sound was found and played. The trigger is cut out of the
// message, which comes back as the first result.
func TP_CheckSoundTrigger(str string) (string, bool) {
	var i, j, start, length int
	var soundname string

	if tp_soundtrigger.string == "" {
		return str, false
	}

	b := []byte(str)

	for i = len(b) - 1; i > 0; i-- {
		if b[i] != 0x0A && b[i] != 0x0D {
			continue
		}

		for j = i - 1; j >= 0; j-- {
			// quick check for chars that cannot be used
			// as sound triggers but might be part of a file name
			if isalnum(b[j]) {
				continue // file name or chat
			}

			if strings.IndexByte(tp_soundtrigger.string, b[j]) != -1 {
				// this might be a sound trigger

				start = j + 1
				length = i - start

				if length == 0 {
					break
				}
				if length >= MAX_QPATH {
					break
				}

				soundname = string(b[start : start+length])
				if strings.Contains(soundname, "..") {
					break // no thank you
				}

				// clean up the message
				b = append(b[:j], b[i:]...)

				if !snd_initialized {
					return string(b), false
				}

				soundname = COM_DefaultExtension(soundname, ".wav", MAX_OSPATH)

				// make sure we have it on disk (FIXME)
				if FS_FileExists(va("sound/%s", soundname)) == 0 {
					return string(b), false
				}

				// now play the sound
				S_LocalSound(soundname)
				return string(b), true
			}
			if b[j] == '\\' {
				b[j] = '/'
			}
			if b[j] <= ' ' || strings.IndexByte("\"&'*,:;<>?\\|\x7f", b[j]) != -1 {
				break // we don't allow these in a file name
			}
		}
	}

	return string(b), false
}

/****************************** MESSAGE FILTERS ******************************/

const MAX_FILTER_LENGTH = 4

var filter_strings [8]string
var num_filters = 0

// returns false if the message shouldn't be printed.
// Matching filters are stripped from the message, which comes back as the first result
func TP_FilterMessage(s string) (string, bool) {
	var i, j, len_, maxlen int

	if num_filters == 0 {
		return s, true
	}

	len_ = len(s)
	if len_ < 2 || s[len_-1] != '\n' || s[len_-2] == '#' {
		return s, true
	}

	maxlen = MAX_FILTER_LENGTH + 1
	for i = len_ - 2; i >= 0 && maxlen > 0; i, maxlen = i-1, maxlen-1 {
		if s[i] == ' ' {
			return s, true
		}
		if s[i] == '#' {
			break
		}
	}
	if i < 0 || maxlen == 0 {
		return s, true // no filter at all
	}

	for j = 0; j < num_filters; j++ {
		if Q_strcasecmp(s[i+1:len_-1], filter_strings[j]) == 0 {
			// strip the filter from message
			if i != 0 && s[i-1] == ' ' {
				// there's a space just before the filter, remove it
				// so that soundtriggers like ^blah #att work
				return s[:i-1] + "\n", true
			}
			return s[:i] + "\n", true
		}
	}

	return s, false // this message is not for us, don't print it
}

func TP_MsgFilter_f() {
	var c, i int
	var s string

	if c = Cmd_Argc(); c == 1 {
		if num_filters == 0 {
			Com_Printf("No filters defined\n")
			return
		}
		for i = 0; i < num_filters; i++ {
			sep := ""
			if i != 0 {
				sep = " "
			}
			Com_Printf("%s#%s", sep, filter_strings[i])
		}
		Com_Printf("\n")
		return
	}

	if c == 2 && (Cmd_Argv(1) == "" || Q_strcasecmp(Cmd_Argv(1), "clear") == 0) {
		num_filters = 0
		return
	}

	num_filters = 0
	for i = 1; i < c; i++ {
		s = Cmd_Argv(i)
		if at(s, 0) != '#' {
			Com_Printf("A filter must start with \"#\"\n")
			return
		}
		if strings.IndexByte(s[1:], ' ') != -1 {
			Com_Printf("A filter may not contain spaces\n")
			return
		}
		filter_strings[num_filters] = strlcpy(s[1:], MAX_FILTER_LENGTH+1)
		num_filters++
		if num_filters >= 8 {
			break
		}
	}
}

func TP_DumpMsgFilters(f io.Writer) {
	var i int

	fmt.Fprintf(f, "filter       ")
	if num_filters == 0 {
		fmt.Fprintf(f, "clear")
	}

	for i = 0; i < num_filters; i++ {
		fmt.Fprintf(f, "#%s ", filter_strings[i])
	}

	fmt.Fprintf(f, "\n")
}

/************************************ INIT ************************************/

func TP_CvarInit() {
	TP_AddMacros()

	Cvar_SetCurrentGroup(CVAR_GROUP_CHAT)
	Cvar_Register(&cl_parseFunChars)
	Cvar_Register(&cl_parseSay)
	Cvar_Register(&cl_nofake)

	Cvar_SetCurrentGroup(CVAR_GROUP_SKIN)
	Cvar_Register(&cl_enemybothskin)
	Cvar_Register(&cl_teambothskin)
	Cvar_Register(&cl_enemypentskin)
	Cvar_Register(&cl_teampentskin)
	Cvar_Register(&cl_enemyquadskin)
	Cvar_Register(&cl_teamquadskin)
	Cvar_Register(&cl_enemyskin)
	Cvar_Register(&cl_teamskin)

	Cvar_SetCurrentGroup(CVAR_GROUP_COMMUNICATION)
	Cvar_Register(&tp_triggers)
	Cvar_Register(&tp_msgtriggers)
	Cvar_Register(&tp_forceTriggers)
	Cvar_Register(&tp_loadlocs)
	Cvar_Register(&tp_soundtrigger)
	Cvar_Register(&tp_weapon_order)
	Cvar_Register(&name_short)

	Cvar_SetCurrentGroup(CVAR_GROUP_ITEM_NAMES)
	Cvar_Register(&tp_name_separator)
	Cvar_Register(&tp_name_none)
	Cvar_Register(&tp_name_nothing)
	Cvar_Register(&tp_name_at)
	Cvar_Register(&tp_name_someplace)

	Cvar_Register(&tp_name_status_blue)
	Cvar_Register(&tp_name_status_red)
	Cvar_Register(&tp_name_status_yellow)
	Cvar_Register(&tp_name_status_green)

	Cvar_Register(&tp_name_pented)
	Cvar_Register(&tp_name_quaded)
	Cvar_Register(&tp_name_eyes)
	Cvar_Register(&tp_name_enemy)
	Cvar_Register(&tp_name_teammate)
	Cvar_Register(&tp_name_disp)
	Cvar_Register(&tp_name_sentry)
	Cvar_Register(&tp_name_rune4)
	Cvar_Register(&tp_name_rune3)
	Cvar_Register(&tp_name_rune2)
	Cvar_Register(&tp_name_rune1)
	Cvar_Register(&tp_name_flag)
	Cvar_Register(&tp_name_backpack)
	Cvar_Register(&tp_name_weapon)
	Cvar_Register(&tp_name_armor)
	Cvar_Register(&tp_name_health)
	Cvar_Register(&tp_name_mh)
	Cvar_Register(&tp_name_suit)
	Cvar_Register(&tp_name_ring)
	Cvar_Register(&tp_name_pent)
	Cvar_Register(&tp_name_quad)
	Cvar_Register(&tp_name_cells)
	Cvar_Register(&tp_name_rockets)
	Cvar_Register(&tp_name_nails)
	Cvar_Register(&tp_name_shells)
	Cvar_Register(&tp_name_armortype_ra)
	Cvar_Register(&tp_name_armortype_ya)
	Cvar_Register(&tp_name_armortype_ga)
	Cvar_Register(&tp_name_ra)
	Cvar_Register(&tp_name_ya)
	Cvar_Register(&tp_name_ga)
	Cvar_Register(&tp_name_lg)
	Cvar_Register(&tp_name_rl)
	Cvar_Register(&tp_name_gl)
	Cvar_Register(&tp_name_sng)
	Cvar_Register(&tp_name_ng)
	Cvar_Register(&tp_name_ssg)
	Cvar_Register(&tp_name_sg)
	Cvar_Register(&tp_name_axe)

	Cvar_SetCurrentGroup(CVAR_GROUP_ITEM_NEED)
	Cvar_Register(&tp_need_health)
	Cvar_Register(&tp_need_cells)
	Cvar_Register(&tp_need_rockets)
	Cvar_Register(&tp_need_nails)
	Cvar_Register(&tp_need_shells)
	Cvar_Register(&tp_need_ra)
	Cvar_Register(&tp_need_ya)
	Cvar_Register(&tp_need_ga)
	Cvar_Register(&tp_need_weapon)
	Cvar_Register(&tp_need_rl)

	Cvar_ResetCurrentGroup()

	Cmd_AddCommand("loadloc", TP_LoadLocFile_f)
	Cmd_AddCommand("filter", TP_MsgFilter_f)
	Cmd_AddCommand("msg_trigger", TP_MsgTrigger_f)
	Cmd_AddCommand("teamcolor", TP_TeamColor_f)
	Cmd_AddCommand("enemycolor", TP_EnemyColor_f)
	Cmd_AddCommand("tp_took", TP_Took_f)
	Cmd_AddCommand("tp_pickup", TP_Pickup_f)
	Cmd_AddCommand("tp_point", TP_Point_f)
}

func TP_Shutdown() {
	TP_ClearLocs()
}

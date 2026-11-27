// Copyright (C) 2003       A Nourai
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
// See the included (GNU.txt) GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program; if not, write to the Free Software
// Foundation, Inc., 59 Temple Place - Suite 330, Boston, MA  02111-1307, USA.

package engine

import (
	"sort"
	"strings"
)

// cl_fragmsgs.c: fragfile parsing and the per-player frag statistics that
// feed the scoreboard

var (
	cl_parsefrags    = cvar_t{name: "cl_parseFrags", string: "0"}
	cl_loadFragfiles = cvar_t{name: "cl_loadFragfiles", string: "0"}
)

const FRAGFILE_VERSION = "1.00"

const (
	MAX_WEAPON_CLASSES   = 64
	MAX_FRAG_DEFINITIONS = 512
	MAX_FRAGMSG_LENGTH   = 256
)

type msgtype_t int

const (
	mt_fragged msgtype_t = iota
	mt_frags
	mt_tkills
	mt_tkilled

	mt_death
	mt_suicide
	mt_frag
	mt_tkill
	mt_flagtouch
	mt_flagdrop
	mt_flagcap
)

type wclass_t struct {
	keyword   string
	name      string
	shortname string
}

type fragmsg_t struct {
	msg1         string
	msg2         string
	hasmsg2      bool // msg2 != NULL
	type_        msgtype_t
	wclass_index int
}

type fragdef_t struct {
	active     bool
	flagalerts bool

	version int
	gamedir string

	title       string
	description string
	author      string
	email       string
	webpage     string

	fragmsgs     []*fragmsg_t
	msgdata      [MAX_FRAG_DEFINITIONS]fragmsg_t
	num_fragmsgs int
}

var (
	fragdefs     fragdef_t
	wclasses     [MAX_WEAPON_CLASSES]wclass_t
	num_wclasses int

	fragmsg1_indexes [26]int
)

func MYISLOWER(c int) bool {
	return c >= 'a' && c <= 'z'
}

// first non-space character of msg1
func fragmsgKey(s string) int {
	i := 0
	for i < len(s) && isspace(s[i]&127) {
		i++
	}
	return int(tolower(at(s, i) & 127))
}

func Compare_FragMsg(f1, f2 *fragmsg_t) int {
	s1 := f1.msg1
	s2 := f2.msg1

	a := fragmsgKey(s1)
	b := fragmsgKey(s2)

	if MYISLOWER(a) && MYISLOWER(b) {
		if a != b {
			return a - b
		}
		return -1 * Q_strcasecmp(s1, s2)
	}
	if MYISLOWER(a) {
		return 1
	}
	if MYISLOWER(b) {
		return -1
	}
	if a != b {
		return a - b
	}
	return -1 * Q_strcasecmp(s1, s2)
}

func Build_FragMsg_Indices() {
	j := -1

	for i := 0; i < fragdefs.num_fragmsgs; i++ {
		c := fragmsgKey(fragdefs.fragmsgs[i].msg1)
		if !MYISLOWER(c) {
			continue
		}

		if c == 'a'+j {
			continue
		}
		for j++; j < c-'a'; j++ {
			fragmsg1_indexes[j] = -1
		}

		fragmsg1_indexes[j] = i
	}

	for j++; j <= 'z'-'a'; j++ {
		fragmsg1_indexes[j] = -1
	}
}

func InitFragDefs() {
	fragdefs = fragdef_t{}
	wclasses = [MAX_WEAPON_CLASSES]wclass_t{}

	wclasses[0].name = "Unknown"
	num_wclasses = 1
}

func LoadFragFile(filename string, quiet bool) {
	var c, i int
	var msgtype msgtype_t
	var token string
	gotversion, warned_flagmsg_overflow := false, false

	InitFragDefs()

	fragfilename := COM_ForceExtension(strlcpy(filename, MAX_OSPATH), ".dat")
	buffer := FS_LoadMallocFile(fragfilename)
	if buffer == nil {
		if !quiet {
			Com_Printf("Couldn't load fragfile \"%s\"\n", fragfilename)
		}
		return
	}

	checkargs := func(line, x int) bool {
		if c != x {
			Com_Printf("Fragfile warning (line %d): %d tokens expected\n", line, x)
			return false
		}
		return true
	}
	checkargs2 := func(line, x, y int) bool {
		if c != x && c != y {
			Com_Printf("Fragfile warning (line %d): %d or %d tokens expected\n", line, x, y)
			return false
		}
		return true
	}
	checkVersionDefined := func() bool {
		if !gotversion {
			Com_Printf("Fragfile error: #FRAGFILE VERSION must be defined at the head of the file\n")
		}
		return gotversion
	}

	aborted := false
nextline:
	for lineidx, text := range strings.Split(string(buffer), "\n") {
		line := lineidx + 1
		Cmd_TokenizeString(text)

		if c = Cmd_Argc(); c == 0 {
			continue
		}

		if Q_strcasecmp(Cmd_Argv(0), "#FRAGFILE") == 0 {
			if !checkargs(line, 3) {
				continue
			}
			if Q_strcasecmp(Cmd_Argv(1), "VERSION") == 0 {
				if gotversion {
					Com_Printf("Fragfile error (line %d): VERSION redefined\n", line)
					aborted = true
					break
				}
				token = Cmd_Argv(2)
				if Q_strcasecmp(token, FRAGFILE_VERSION) == 0 {
					fragdefs.version = int(100 * Q_atof(token))
					gotversion = true
				} else {
					Com_Printf("Error: fragfile version (\"%s\") unsupported \n", token)
					aborted = true
					break
				}
			} else if Q_strcasecmp(Cmd_Argv(1), "GAMEDIR") == 0 {
				if !checkVersionDefined() {
					aborted = true
					break
				}
				if Q_strcasecmp(Cmd_Argv(2), "ANY") == 0 {
					fragdefs.gamedir = ""
				} else {
					fragdefs.gamedir = Cmd_Argv(2)
				}
			} else {
				if !checkVersionDefined() {
					aborted = true
					break
				}
				Com_Printf("Fragfile warning (line %d): unexpected token \"%s\"\n", line, Cmd_Argv(1))
				continue
			}
		} else if Q_strcasecmp(Cmd_Argv(0), "#META") == 0 {
			if !checkVersionDefined() {
				aborted = true
				break
			}
			if !checkargs(line, 3) {
				continue
			}
			if Q_strcasecmp(Cmd_Argv(1), "TITLE") == 0 {
				fragdefs.title = Cmd_Argv(2)
			} else if Q_strcasecmp(Cmd_Argv(1), "AUTHOR") == 0 {
				fragdefs.author = Cmd_Argv(2)
			} else if Q_strcasecmp(Cmd_Argv(1), "DESCRIPTION") == 0 {
				fragdefs.description = Cmd_Argv(2)
			} else if Q_strcasecmp(Cmd_Argv(1), "EMAIL") == 0 {
				fragdefs.email = Cmd_Argv(2)
			} else if Q_strcasecmp(Cmd_Argv(1), "WEBPAGE") == 0 {
				fragdefs.webpage = Cmd_Argv(2)
			} else {
				Com_Printf("Fragfile warning (line %d): unexpected token \"%s\"\n", line, Cmd_Argv(1))
				continue
			}
		} else if Q_strcasecmp(Cmd_Argv(0), "#DEFINE") == 0 {
			if !checkVersionDefined() {
				aborted = true
				break
			}
			if Q_strcasecmp(Cmd_Argv(1), "WEAPON_CLASS") == 0 || Q_strcasecmp(Cmd_Argv(1), "WC") == 0 {
				if !checkargs2(line, 4, 5) {
					continue
				}
				token = Cmd_Argv(2)
				if Q_strcasecmp(token, "NOWEAPON") == 0 || Q_strcasecmp(token, "NONE") == 0 || Q_strcasecmp(token, "NULL") == 0 {
					Com_Printf("Fragfile warning (line %d): \"%s\" is a reserved keyword\n", line, token)
					continue
				}
				for i = 1; i < num_wclasses; i++ {
					if Q_strcasecmp(token, wclasses[i].keyword) == 0 {
						Com_Printf("Fragfile warning (line %d): WEAPON_CLASS \"%s\" already defined\n", line, wclasses[i].keyword)
						continue nextline
					}
				}
				if num_wclasses == MAX_WEAPON_CLASSES {
					Com_Printf("Fragfile warning (line %d): only %d WEAPON_CLASS's may be #DEFINE'd\n", line, MAX_WEAPON_CLASSES)
					continue
				}
				wclasses[num_wclasses].keyword = token
				wclasses[num_wclasses].name = Cmd_Argv(3)
				// C overwrites the name here and never fills shortname
				if c == 5 {
					wclasses[num_wclasses].name = Cmd_Argv(4)
				}
				num_wclasses++
			} else if Q_strcasecmp(Cmd_Argv(1), "OBITUARY") == 0 || Q_strcasecmp(Cmd_Argv(1), "OBIT") == 0 {
				if Q_strcasecmp(Cmd_Argv(2), "PLAYER_DEATH") == 0 {
					if !checkargs(line, 5) {
						continue
					}
					msgtype = mt_death
				} else if Q_strcasecmp(Cmd_Argv(2), "PLAYER_SUICIDE") == 0 {
					if !checkargs(line, 5) {
						continue
					}
					msgtype = mt_suicide
				} else if Q_strcasecmp(Cmd_Argv(2), "X_FRAGS_UNKNOWN") == 0 {
					if !checkargs(line, 5) {
						continue
					}
					msgtype = mt_frag
				} else if Q_strcasecmp(Cmd_Argv(2), "X_TEAMKILLS_UNKNOWN") == 0 {
					if !checkargs(line, 5) {
						continue
					}
					msgtype = mt_tkill
				} else if Q_strcasecmp(Cmd_Argv(2), "X_FRAGS_Y") == 0 {
					if !checkargs2(line, 5, 6) {
						continue
					}
					msgtype = mt_frags
				} else if Q_strcasecmp(Cmd_Argv(2), "X_FRAGGED_BY_Y") == 0 {
					if !checkargs2(line, 5, 6) {
						continue
					}
					msgtype = mt_fragged
				} else if Q_strcasecmp(Cmd_Argv(2), "X_TEAMKILLS_Y") == 0 {
					if !checkargs2(line, 5, 6) {
						continue
					}
					msgtype = mt_tkills
				} else if Q_strcasecmp(Cmd_Argv(2), "X_TEAMKILLED_BY_Y") == 0 {
					if !checkargs2(line, 5, 6) {
						continue
					}
					msgtype = mt_tkilled
				} else {
					Com_Printf("Fragfile warning (line %d): unexpected token \"%s\"\n", line, Cmd_Argv(2))
					continue
				}

				if fragdefs.num_fragmsgs == MAX_FRAG_DEFINITIONS {
					if !warned_flagmsg_overflow {
						Com_Printf("Fragfile warning (line %d): only %d OBITUARY's and FLAG_ALERT's's may be #DEFINE'd\n",
							line, MAX_FRAG_DEFINITIONS)
					}
					warned_flagmsg_overflow = true
					continue
				}

				if len(Cmd_Argv(4)) >= MAX_FRAGMSG_LENGTH || (c == 6 && len(Cmd_Argv(5)) >= MAX_FRAGMSG_LENGTH) {
					Com_Printf("Fragfile warning (line %d): Message token cannot exceed %d characters\n", line, MAX_FRAGMSG_LENGTH-1)
					continue
				}

				token = Cmd_Argv(3)
				if Q_strcasecmp(token, "NOWEAPON") == 0 || Q_strcasecmp(token, "NONE") == 0 || Q_strcasecmp(token, "NULL") == 0 {
					fragdefs.msgdata[fragdefs.num_fragmsgs].wclass_index = 0
				} else {
					for i = 1; i < num_wclasses; i++ {
						if Q_strcasecmp(token, wclasses[i].keyword) == 0 {
							fragdefs.msgdata[fragdefs.num_fragmsgs].wclass_index = i
							break
						}
					}
					if i == num_wclasses {
						Com_Printf("Fragfile warning (line %d): \"%s\" is not a defined WEAPON_CLASS\n", line, token)
						continue
					}
				}

				fragdefs.msgdata[fragdefs.num_fragmsgs].type_ = msgtype
				fragdefs.msgdata[fragdefs.num_fragmsgs].msg1 = Cmd_Argv(4)
				fragdefs.msgdata[fragdefs.num_fragmsgs].msg2 = ""
				fragdefs.msgdata[fragdefs.num_fragmsgs].hasmsg2 = c == 6
				if c == 6 {
					fragdefs.msgdata[fragdefs.num_fragmsgs].msg2 = Cmd_Argv(5)
				}
				fragdefs.num_fragmsgs++
			} else if Q_strcasecmp(Cmd_Argv(1), "FLAG_ALERT") == 0 || Q_strcasecmp(Cmd_Argv(1), "FLAG_MSG") == 0 {
				if !checkargs(line, 4) {
					continue
				}
				if Q_strcasecmp(Cmd_Argv(2), "X_TOUCHES_FLAG") == 0 ||
					Q_strcasecmp(Cmd_Argv(2), "X_GETS_FLAG") == 0 ||
					Q_strcasecmp(Cmd_Argv(2), "X_TAKES_FLAG") == 0 {
					msgtype = mt_flagtouch
				} else if Q_strcasecmp(Cmd_Argv(2), "X_DROPS_FLAG") == 0 ||
					Q_strcasecmp(Cmd_Argv(2), "X_FUMBLES_FLAG") == 0 ||
					Q_strcasecmp(Cmd_Argv(2), "X_LOSES_FLAG") == 0 {
					msgtype = mt_flagdrop
				} else if Q_strcasecmp(Cmd_Argv(2), "X_CAPTURES_FLAG") == 0 ||
					Q_strcasecmp(Cmd_Argv(2), "X_CAPS_FLAG") == 0 ||
					Q_strcasecmp(Cmd_Argv(2), "X_SCORES") == 0 {
					msgtype = mt_flagcap
				} else {
					Com_Printf("Fragfile warning (line/ %d): unexpected token \"%s\"\n", line, Cmd_Argv(2))
					continue
				}

				if fragdefs.num_fragmsgs == MAX_FRAG_DEFINITIONS {
					if !warned_flagmsg_overflow {
						Com_Printf("Fragfile warning (line %d): only %d OBITUARY's and FLAG_ALERT's's may be #DEFINE'd\n",
							line, MAX_FRAG_DEFINITIONS)
					}
					warned_flagmsg_overflow = true
					continue
				}
				fragdefs.msgdata[fragdefs.num_fragmsgs].type_ = msgtype
				fragdefs.msgdata[fragdefs.num_fragmsgs].msg1 = Cmd_Argv(3)
				fragdefs.msgdata[fragdefs.num_fragmsgs].msg2 = ""
				fragdefs.msgdata[fragdefs.num_fragmsgs].hasmsg2 = false
				fragdefs.num_fragmsgs++

				fragdefs.flagalerts = true
			}
		} else {
			if !checkVersionDefined() {
				aborted = true
				break
			}
			Com_Printf("Fragfile warning (line %d): unexpected token \"%s\"\n", line, Cmd_Argv(0))
			continue
		}
	}

	if aborted {
		return
	}

	if fragdefs.num_fragmsgs != 0 {
		fragdefs.fragmsgs = make([]*fragmsg_t, fragdefs.num_fragmsgs)
		for i = 0; i < fragdefs.num_fragmsgs; i++ {
			fragdefs.fragmsgs[i] = &fragdefs.msgdata[i]
		}
		sort.SliceStable(fragdefs.fragmsgs, func(a, b int) bool { return Compare_FragMsg(fragdefs.fragmsgs[a], fragdefs.fragmsgs[b]) < 0 })
		Build_FragMsg_Indices()

		fragdefs.active = true
		if !quiet {
			Com_Printf("Loaded fragfile \"%s\" (%d frag defs)\n", fragfilename, fragdefs.num_fragmsgs)
		}
	} else {
		if !quiet {
			Com_Printf("Fragfile \"%s\" was empty\n", fragfilename)
		}
	}
}

func Load_FragFile_f() {
	switch Cmd_Argc() {
	case 2:
		LoadFragFile(Cmd_Argv(1), false)
	default:
		Com_Printf("Usage: %s <filename>\n", Cmd_Argv(0))
	}
}

type fragstats_t struct {
	deaths     [MAX_CLIENTS]int
	teamdeaths [MAX_CLIENTS]int
	kills      [MAX_CLIENTS]int
	teamkills  [MAX_CLIENTS]int
	wdeaths    [MAX_WEAPON_CLASSES]int
	wkills     [MAX_WEAPON_CLASSES]int

	totaldeaths    int
	totalfrags     int
	totalteamkills int
	totalsuicides  int

	touches  int
	fumbles  int
	captures int
}

var (
	fragstats                                 [MAX_CLIENTS]fragstats_t
	flag_dropped, flag_touched, flag_captured bool
)

func Stats_ParsePrintLine(s string) {
	var start_search, end_Search, i, j, k, p1len, msg1len, msg2len, p2len, killer, victim int
	var fragmsg *fragmsg_t
	var player1, player2 *player_info_t

	for i = 0; i < MAX_CLIENTS; i++ {
		player1 = &cl.players[i]
		if player1.name == "" || player1.spectator != 0 {
			continue
		}
		name1 := Info_ValueForKey(player1.userinfo, "name")
		p1len = min(len(name1), 31)
		if strneq(s, name1, p1len) {
			t := p1len
			for t < len(s) && isspace(s[t]&127) {
				t++
			}
			k = int(tolower(at(s, t) & 127))
			if MYISLOWER(k) {
				start_search = fragmsg1_indexes[k-'a']
				if k == 'z' {
					end_Search = fragdefs.num_fragmsgs
				} else {
					end_Search = fragmsg1_indexes[k-'a'+1]
				}
			} else {
				start_search = 0
				end_Search = fragmsg1_indexes[0]
			}
			if start_search == -1 {
				continue
			}
			if end_Search == -1 {
				end_Search = fragdefs.num_fragmsgs
			}

			for j = start_search; j < end_Search; j++ {
				fragmsg = fragdefs.fragmsgs[j]
				msg1len = len(fragmsg.msg1)
				if strneq(s[p1len:], fragmsg.msg1, msg1len) {
					if fragmsg.type_ == mt_fragged || fragmsg.type_ == mt_frags ||
						fragmsg.type_ == mt_tkills || fragmsg.type_ == mt_tkilled {
						for k = 0; k < MAX_CLIENTS; k++ {
							start := p1len + msg1len
							player2 = &cl.players[k]
							if player2.name == "" || player2.spectator != 0 {
								continue
							}
							name2 := Info_ValueForKey(player2.userinfo, "name")
							p2len = min(len(name2), 31)
							if strneq(s[start:], name2, p2len) {
								if fragmsg.hasmsg2 {
									start = p1len + msg1len + p2len
									if start >= len(s) {
										continue
									}

									msg2len = len(fragmsg.msg2)
									if strneq(s[start:], fragmsg.msg2, msg2len) {
										goto foundmatch
									}
								} else {
									goto foundmatch
								}
							}
						}
					} else {
						goto foundmatch
					}
				}
			}
		}
	}

	return

foundmatch:
	i = playerSlot(player1)
	if player2 != nil {
		j = playerSlot(player2)
	}

	switch fragmsg.type_ {
	case mt_death:
		fragstats[i].totaldeaths++
		fragstats[i].wdeaths[fragmsg.wclass_index]++

	case mt_suicide:
		fragstats[i].totalsuicides++
		fragstats[i].totaldeaths++

	case mt_fragged, mt_frags:
		if fragmsg.type_ == mt_fragged {
			killer, victim = j, i
		} else {
			killer, victim = i, j
		}

		fragstats[killer].kills[victim]++
		fragstats[killer].totalfrags++
		fragstats[killer].wkills[fragmsg.wclass_index]++

		fragstats[victim].deaths[killer]++
		fragstats[victim].totaldeaths++
		fragstats[victim].wdeaths[fragmsg.wclass_index]++

	case mt_frag:
		fragstats[i].totalfrags++
		fragstats[i].wkills[fragmsg.wclass_index]++

	case mt_tkilled, mt_tkills:
		if fragmsg.type_ == mt_tkilled {
			killer, victim = j, i
		} else {
			killer, victim = i, j
		}

		fragstats[killer].teamkills[victim]++
		fragstats[killer].totalteamkills++

		fragstats[victim].teamdeaths[killer]++
		fragstats[victim].totaldeaths++

	case mt_tkill:
		fragstats[i].totalteamkills++

	case mt_flagtouch:
		fragstats[i].touches++
		flag_touched = true

	case mt_flagdrop:
		fragstats[i].fumbles++
		flag_dropped = true

	case mt_flagcap:
		fragstats[i].captures++
		flag_captured = true
	}
}

func Stats_ParsePrint(s string, level int) {
	if !Stats_IsActive() {
		return
	}

	if level != PRINT_MEDIUM && level != PRINT_HIGH {
		return
	}

	// each line keeps its newline; the tail after the last one is parsed too
	start := 0
	for {
		end := strings.IndexByte(s[start:], '\n')
		if end == -1 {
			Stats_ParsePrintLine(s[start:])
			break
		}
		end += start + 1
		Stats_ParsePrintLine(s[start:end])
		start = end
	}
}

func Stats_Reset() {
	fragstats = [MAX_CLIENTS]fragstats_t{}
	flag_touched, flag_dropped, flag_captured = false, false, false
}

var stats_last_gamedir string

func Stats_NewMap() {
	if stats_last_gamedir == "" || Q_strcasecmp(stats_last_gamedir, cls.gamedirfile) != 0 {
		if cl_loadFragfiles.value != 0 {
			stats_last_gamedir = strlcpy(cls.gamedirfile, MAX_OSPATH)
			LoadFragFile("fragfile", true)
		} else {
			InitFragDefs()
		}
	}

	Stats_Reset()
}

func Stats_EnterSlot(num int) {
	if num < 0 || num >= MAX_CLIENTS {
		Sys_Error("Stats_EnterSlot: num < 0 || num >= MAX_CLIENTS")
	}

	fragstats[num] = fragstats_t{}

	for i := 0; i < MAX_CLIENTS; i++ {
		if cl.players[i].name == "" || cl.players[i].spectator != 0 {
			continue
		}
		if i == num {
			continue
		}

		fragstats[i].teamdeaths[num], fragstats[i].deaths[num] = 0, 0
		fragstats[i].teamkills[num], fragstats[i].kills[num] = 0, 0
	}
}

func Stats_GetBasicStats(num int, playerstats []int) {
	if num < 0 || num >= MAX_CLIENTS {
		Sys_Error("Stats_EnterSlot: num < 0 || num >= MAX_CLIENTS")
	}

	playerstats[0] = fragstats[num].totalfrags
	playerstats[1] = fragstats[num].totaldeaths
	playerstats[2] = fragstats[num].totalteamkills
	playerstats[3] = fragstats[num].totalsuicides
}

func Stats_GetWeaponKills(num, count int, weaponstats []int) int {
	if num < 0 || num >= MAX_CLIENTS {
		Sys_Error("Stats_EnterSlot: num < 0 || num >= MAX_CLIENTS")
	}

	if count < 0 || count > num_wclasses {
		return 1
	}

	for i := 0; i < count; i++ {
		weaponstats[i] = fragstats[num].wkills[i]
	}

	return 0
}

func Stats_GetWeaponKill(num int, weapon string) int {
	if num < 0 || num >= MAX_CLIENTS {
		return 0
	}

	for i := 0; i < num_wclasses; i++ {
		if wclasses[i].shortname != "" {
			if wclasses[i].shortname == weapon {
				return fragstats[num].wkills[i]
			}
		} else {
			if wclasses[i].name == weapon {
				return fragstats[num].wkills[i]
			}
		}
	}
	return 0
}

func Stats_GetFlagStats(num int, playerstats []int) {
	if num < 0 || num >= MAX_CLIENTS {
		Sys_Error("Stats_EnterSlot: num < 0 || num >= MAX_CLIENTS")
	}

	if !flag_touched && flag_dropped && flag_captured {
		playerstats[0] = fragstats[num].fumbles + fragstats[num].captures
	} else {
		playerstats[0] = fragstats[num].touches
	}

	playerstats[1] = fragstats[num].fumbles
	playerstats[2] = fragstats[num].captures
}

func Stats_IsActive() bool {
	return fragdefs.active && cl_parsefrags.value != 0
}

func Stats_IsFlagsParsed() bool {
	return Stats_IsActive() && fragdefs.flagalerts
}

func Stats_CvarInit() {
	Cvar_Register(&cl_parsefrags)
	Cvar_Register(&cl_loadFragfiles)
	Cmd_AddCommand("loadFragfile", Load_FragFile_f)
}

func Stats_Init() {
	InitFragDefs()
}

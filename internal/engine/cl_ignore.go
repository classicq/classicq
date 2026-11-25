package engine

// cl_ignore.c: per-player, per-team, spectator and flood ignoring of chat

// ignore.h
const (
	NO_IGNORE_NO_ADD = 0
	NO_IGNORE_ADD    = 1
	IGNORE_NO_ADD    = 2
)

const (
	MAX_TEAMIGNORELIST = 4
	FLOODLIST_SIZE     = 10
)

var (
	ignore_spec           = cvar_t{name: "ignore_spec", string: "0"}
	ignore_mode           = cvar_t{name: "ignore_mode", string: "0"}
	ignore_flood_duration = cvar_t{name: "ignore_flood_duration", string: "4"}
	ignore_flood          = cvar_t{name: "ignore_flood", string: "0"}
	ignore_opponents      = cvar_t{name: "ignore_opponents", string: "0"}
)

var ignoreteamlist [MAX_TEAMIGNORELIST]string // 16 chars each

type flood_t struct {
	data string // 2047 chars
	time float32
}

var (
	floodlist  [FLOODLIST_SIZE]flood_t
	floodindex int
)

// C pushes each name at the head of a list, so the replay runs newest first
var ignored_names []string

func IsIgnored(slot int) bool {
	return cl.players[slot].ignored
}

func Display_Ignorelist() {
	Com_Printf("\x02" + "User Ignore List:\n")
	tp := TablePrint_Begin(1)
	for i := 0; i < MAX_CLIENTS; i++ {
		if cl.players[i].name != "" && cl.players[i].ignored {
			TablePrint_AddItem(tp, cl.players[i].name)
		}
	}
	TablePrint_End(tp)

	Com_Printf("\x02" + "Team Ignore List:\n")
	tp = TablePrint_Begin(1)
	for i := 0; i < MAX_TEAMIGNORELIST && ignoreteamlist[i] != ""; i++ {
		TablePrint_AddItem(tp, ignoreteamlist[i])
	}
	TablePrint_End(tp)

	if ignore_opponents.value != 0 {
		Com_Printf("\x02" + "Opponents are Ignored\n")
	}

	if ignore_spec.value == 2 || (ignore_spec.value == 1 && cl.spectator == 0) {
		Com_Printf("\x02" + "Spectators are Ignored\n")
	}
	Com_Printf("\n")
}

func Ignorelist_Add(slot int) bool {
	if IsIgnored(slot) {
		return false
	}

	cl.players[slot].ignored = true
	return true
}

func Ignorelist_Del(slot int) bool {
	if !cl.players[slot].ignored {
		return false
	}

	cl.players[slot].ignored = false
	return true
}

func Ignore_f() {
	c := Cmd_Argc()
	if c == 1 {
		Display_Ignorelist()
		return
	} else if c != 2 {
		Com_Printf("Usage: %s [userid | name]\n", Cmd_Argv(0))
		return
	}

	slot := Player_StringtoSlot(Cmd_Argv(1))
	if slot == PLAYER_ID_NOMATCH {
		Com_Printf("%s : no player with userid %d\n", Cmd_Argv(0), Q_atoi(Cmd_Argv(1)))
	} else if slot == PLAYER_NAME_NOMATCH {
		Com_Printf("%s : no player with name %s\n", Cmd_Argv(0), Cmd_Argv(1))
	} else {
		if Ignorelist_Add(slot) {
			Com_Printf("Added user %s to ignore list\n", cl.players[slot].name)
		} else {
			Com_Printf("User %s is already ignored\n", cl.players[slot].name)
		}
	}
}

func IgnoreList_f() {
	if Cmd_Argc() != 1 {
		Com_Printf("%s : no arguments expected\n", Cmd_Argv(0))
	} else {
		Display_Ignorelist()
	}
}

func Ignore_ID_f() {
	c := Cmd_Argc()
	if c == 1 {
		Display_Ignorelist()
		return
	} else if c != 2 {
		Com_Printf("Usage: %s [userid]\n", Cmd_Argv(0))
		return
	}
	arg := Cmd_Argv(1)
	for i := 0; i < len(arg); i++ {
		if !isdigit(arg[i]) {
			Com_Printf("Usage: %s [userid]\n", Cmd_Argv(0))
			return
		}
	}
	userid := Q_atoi(arg)
	slot := Player_IdtoSlot(userid)
	if slot == PLAYER_ID_NOMATCH {
		Com_Printf("%s : no player with userid %d\n", Cmd_Argv(0), userid)
		return
	}
	if Ignorelist_Add(slot) {
		Com_Printf("Added user %s to ignore list\n", cl.players[slot].name)
	} else {
		Com_Printf("User %s is already ignored\n", cl.players[slot].name)
	}
}

func Unignore_f() {
	c := Cmd_Argc()
	if c == 1 {
		Display_Ignorelist()
		return
	} else if c != 2 {
		Com_Printf("Usage: %s [userid | name]\n", Cmd_Argv(0))
		return
	}

	slot := Player_StringtoSlot(Cmd_Argv(1))
	if slot == PLAYER_ID_NOMATCH {
		Com_Printf("%s : no player with userid %d\n", Cmd_Argv(0), Q_atoi(Cmd_Argv(1)))
	} else if slot == PLAYER_NAME_NOMATCH {
		Com_Printf("%s : no player with name %s\n", Cmd_Argv(0), Cmd_Argv(1))
	} else {
		if Ignorelist_Del(slot) {
			Com_Printf("Removed user %s from ignore list\n", cl.players[slot].name)
		} else {
			Com_Printf("User %s is not being ignored\n", cl.players[slot].name)
		}
	}
}

func Unignore_ID_f() {
	c := Cmd_Argc()
	if c == 1 {
		Display_Ignorelist()
		return
	} else if c != 2 {
		Com_Printf("Usage: %s [userid]\n", Cmd_Argv(0))
		return
	}
	arg := Cmd_Argv(1)
	for i := 0; i < len(arg); i++ {
		if !isdigit(arg[i]) {
			Com_Printf("Usage: %s [userid]\n", Cmd_Argv(0))
			return
		}
	}
	userid := Q_atoi(arg)
	slot := Player_IdtoSlot(userid)
	if slot == PLAYER_ID_NOMATCH {
		Com_Printf("%s : no player with userid %d\n", Cmd_Argv(0), userid)
		return
	}
	if Ignorelist_Del(slot) {
		Com_Printf("Removed user %s from ignore list\n", cl.players[slot].name)
	} else {
		Com_Printf("User %s is not being ignored\n", cl.players[slot].name)
	}
}

func Ignoreteam_f() {
	c := Cmd_Argc()
	if c == 1 {
		Display_Ignorelist()
		return
	} else if c != 2 {
		Com_Printf("Usage: %s [team]\n", Cmd_Argv(0))
		return
	}
	arg := Cmd_Argv(1)
	for i := 0; i < MAX_CLIENTS; i++ {
		if cl.players[i].name != "" && cl.players[i].spectator == 0 && arg == cl.players[i].team {
			j := 0
			for ; j < MAX_TEAMIGNORELIST && ignoreteamlist[j] != ""; j++ {
				if strneq(arg, ignoreteamlist[j], 16) {
					Com_Printf("Team %s is already ignored\n", arg)
					return
				}
			}

			if j == MAX_TEAMIGNORELIST {
				Com_Printf("You cannot ignore more than %d teams\n", MAX_TEAMIGNORELIST)
			}

			// C writes past the array when the list is full
			if j < MAX_TEAMIGNORELIST {
				ignoreteamlist[j] = strlcpy(arg, 17)
			}

			if j+1 < MAX_TEAMIGNORELIST {
				ignoreteamlist[j+1] = ""
			}

			Com_Printf("Added team %s to ignore list\n", arg)

			return
		}
	}

	Com_Printf("%s : no team with name %s\n", Cmd_Argv(0), arg)
}

func Unignoreteam_f() {
	c := Cmd_Argc()
	if c == 1 {
		Display_Ignorelist()
		return
	} else if c != 2 {
		Com_Printf("Usage: %s [team]\n", Cmd_Argv(0))
		return
	}

	arg := Cmd_Argv(1)
	for i := 0; i < MAX_TEAMIGNORELIST && ignoreteamlist[i] != ""; i++ {
		if strneq(arg, ignoreteamlist[i], 16) {
			j := i
			for ; j < MAX_TEAMIGNORELIST && ignoreteamlist[j] != ""; j++ {
			}

			j--
			if j > i {
				ignoreteamlist[i] = strlcpy(ignoreteamlist[j], 17)
			}

			ignoreteamlist[j] = ""
			Com_Printf("Removed team %s from ignore list\n", arg)

			return
		}
	}

	Com_Printf("Team %s is not being ignored\n", arg)
}

func UnignoreAll_f() {
	if Cmd_Argc() != 1 {
		Com_Printf("%s : no arguments expected\n", Cmd_Argv(0))
		return
	}

	for i := 0; i < MAX_CLIENTS; i++ {
		cl.players[i].ignored = false
	}

	Com_Printf("User ignore list cleared\n")
}

func UnignoreteamAll_f() {
	if Cmd_Argc() != 1 {
		Com_Printf("%s : no arguments expected\n", Cmd_Argv(0))
		return
	}
	ignoreteamlist[0] = ""
	Com_Printf("Team ignore list cleared\n")
}

func Ignore_Check_Flood(s string, flags, offset int) int {
	if !((ignore_flood.value == 1 && (flags == 1 || flags == 4)) ||
		(ignore_flood.value == 2 && flags != 0)) {
		return NO_IGNORE_NO_ADD
	}

	var p, q int
	if flags == 1 || flags == 4 {
		p = 0
		q = offset - 3
	} else if flags == 2 {
		p = 1
		q = offset - 4
	} else if flags == 8 {
		p = 7
		q = offset - 3
	} else {
		return NO_IGNORE_NO_ADD
	}

	length := bound(0, q-p+1, MAX_INFO_STRING-1)

	name := cstrn(s, p, length)
	if !cls.demoplayback && name == Player_MyName() {
		return NO_IGNORE_NO_ADD
	}
	for i := 0; i < FLOODLIST_SIZE; i++ {
		if floodlist[i].data != "" && strneq(floodlist[i].data, s, 2047) && cls.realtime-float64(floodlist[i].time) < float64(ignore_flood_duration.value) {
			return IGNORE_NO_ADD
		}
	}

	return NO_IGNORE_ADD
}

func Ignore_Flood_Add(s string) {
	floodlist[floodindex].data = strlcpy(s, 2048)
	floodlist[floodindex].time = float32(cls.realtime)
	floodindex++
	if floodindex == FLOODLIST_SIZE {
		floodindex = 0
	}
}

func Ignore_Message(s string, flags, offset int) bool {
	if ignore_mode.value == 0 && flags&2 != 0 {
		return false
	}

	if ignore_spec.value == 2 && (flags == 4 || (flags == 8 && ignore_mode.value != 0)) {
		return true
	} else if ignore_spec.value == 1 && flags == 4 && cl.spectator == 0 {
		return true
	}

	var p, q int
	if flags == 1 || flags == 4 {
		p = 0
		q = offset - 3
	} else if flags == 2 {
		p = 1
		q = offset - 4
	} else if flags == 8 {
		p = 7
		q = offset - 3
	} else {
		return false
	}

	length := bound(0, q-p+1, 31)
	name := cstrn(s, p, length)

	slot := Player_NametoSlot(name)
	if slot == PLAYER_NAME_NOMATCH {
		return false
	}

	if cl.players[slot].ignored {
		return true
	}

	if ignore_opponents.value != 0 && flags == 1 && cl.spectator == 0 && slot != cl.playernum &&
		(cl.teamplay == 0 || cl.players[slot].team != cl.players[cl.playernum].team) {
		return true
	}

	if cl.teamplay == 0 {
		return false
	}

	if cl.players[slot].spectator != 0 || Player_MyName() == name {
		return false
	}

	for i := 0; i < MAX_TEAMIGNORELIST && ignoreteamlist[i] != ""; i++ {
		if strneq(cl.players[slot].team, ignoreteamlist[i], 16) {
			return true
		}
	}

	return false
}

func Ignore_ResetFloodList() {
	for i := 0; i < FLOODLIST_SIZE; i++ {
		floodlist[i].data = ""
	}

	floodindex = 0
}

func Ignore_CvarInit() {
	Cvar_SetCurrentGroup(CVAR_GROUP_CHAT)
	Cvar_Register(&ignore_flood_duration)
	Cvar_Register(&ignore_flood)
	Cvar_Register(&ignore_spec)
	Cvar_Register(&ignore_mode)
	Cvar_Register(&ignore_opponents)

	Cvar_ResetCurrentGroup()

	Cmd_AddCommand("ignore", Ignore_f)
	Cmd_AddCommand("ignorelist", IgnoreList_f)
	Cmd_AddCommand("unignore", Unignore_f)
	Cmd_AddCommand("ignore_team", Ignoreteam_f)
	Cmd_AddCommand("unignore_team", Unignoreteam_f)
	Cmd_AddCommand("unignoreAll", UnignoreAll_f)
	Cmd_AddCommand("unignoreAll_team", UnignoreteamAll_f)
	Cmd_AddCommand("unignore_id", Unignore_ID_f)
	Cmd_AddCommand("ignore_id", Ignore_ID_f)
}

func Ignore_Init() {
	for i := 0; i < MAX_TEAMIGNORELIST; i++ {
		ignoreteamlist[i] = ""
	}

	Ignore_ResetFloodList()
}

func Ignore_PreNewMap() {
	for i := 0; i < MAX_CLIENTS; i++ {
		if cl.players[i].ignored {
			ignored_names = append(ignored_names, cl.players[i].name)
		}
	}
}

func Ignore_PostNewMap() {
	for i := len(ignored_names) - 1; i >= 0; i-- {
		slot := Player_StringtoSlot(ignored_names[i])

		if slot != PLAYER_ID_NOMATCH && slot != PLAYER_NAME_NOMATCH {
			Ignorelist_Add(slot)
		}
	}

	ignored_names = nil
}

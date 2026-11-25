package engine

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// match_tools.c: match naming macros, automatic demo/log/screenshot per
// match and the map groups

const MAX_STATIC_STRING = 1024

var (
	match_format_solo = cvar_t{name: "match_format_solo", string: "solo/%n - [%M]"}
	match_format_coop = cvar_t{name: "match_format_coop", string: "coop/%n - [%C_player_coop] - [%M]"}
	match_format_race = cvar_t{name: "match_format_race", string: "race/%n - [race] - [%M]"}

	match_format_duel      = cvar_t{name: "match_format_duel", string: "duel/%n - %p%v%e - [dmm%D] - [%M]"}
	match_format_ffa       = cvar_t{name: "match_format_ffa", string: "ffa/%n - [%C_player_ffa] - [%M]"}
	match_format_2on2      = cvar_t{name: "match_format_2on2", string: "2on2/%n - [%k%v%l] - [%M]"}
	match_format_3on3      = cvar_t{name: "match_format_3on3", string: "tdm/%n - [%Oon%E_%t%v%e] - [%M]"}
	match_format_4on4      = cvar_t{name: "match_format_4on4", string: "tdm/%n - [%Oon%E_%t%v%e] - [%M]"}
	match_format_tdm       = cvar_t{name: "match_format_tdm", string: "tdm/%n - [%Oon%E_%t%v%e] - [%M]"}
	match_format_multiteam = cvar_t{name: "match_format_multiteam", string: "tdm/%n - [%a_%b] - [%M]"}

	match_format_arena      = cvar_t{name: "match_format_arena", string: "arena/%n - %p%v%e - [%F_frags] - [%M]"}
	match_format_tf_duel    = cvar_t{name: "match_format_tf_duel", string: "tfduel/%n - %p%v%e [%M]"}
	match_format_tf_clanwar = cvar_t{name: "match_format_tf_clanwar", string: "tfwar/%n - [%Oon%E_%t%v%e] - [%M]"}

	match_name_and    = cvar_t{name: "match_name_and", string: "_&_"}
	match_name_versus = cvar_t{name: "match_name_versus", string: "_vs_"}
	match_name_on     = cvar_t{name: "match_name_on", string: "on"}
	match_name_nick   = cvar_t{name: "match_name_nick", string: ""}
	match_name_spec   = cvar_t{name: "match_name_spec", string: "(SPEC)"}
)

func MT_CleanString(str string, allow_spaces_and_slashes bool) string {
	badchars := " \\/?*:<>\"|"

	disallowed := badchars
	if allow_spaces_and_slashes {
		disallowed = badchars[3:]
	}

	cleanchar := func(c byte) byte {
		r := readablechars[c]
		if r < ' ' || strings.IndexByte(disallowed, r) >= 0 {
			return '_'
		}
		return r
	}

	out := make([]byte, 0, len(str))
	in := 0
	for in < len(str) && len(out) < MAX_STATIC_STRING-1 {
		c := str[in]
		in++
		d := cleanchar(c)
		out = append(out, d)
		if d == '_' {
			for in < len(str) && cleanchar(str[in]) == '_' {
				in++
			}
		}
	}

	return Util_Process_Filename(string(out))
}

func MT_PlayerName() string {
	return TP_PlayerName()
}

func MT_PlayerTeam() string {
	return TP_PlayerTeam()
}

func MT_EnemyName() string {
	myname := MT_PlayerName()

	for i := 0; i < MAX_CLIENTS; i++ {
		if cl.players[i].name != "" && cl.players[i].spectator == 0 {
			name := Info_ValueForKey(cl.players[i].userinfo, "name")
			if name != myname {
				return name
			}
		}
	}

	return ""
}

func MT_EnemyTeam() string {
	myteam := MT_PlayerTeam()

	for i := 0; i < MAX_CLIENTS; i++ {
		if cl.players[i].name != "" && cl.players[i].spectator == 0 {
			team := cl.players[i].team
			if team != "" && team != myteam {
				return team
			}
		}
	}

	return ""
}

func MT_CountPlayers() int {
	return TP_CountPlayers()
}

func MT_CountTeamMembers(team string) int {
	count := 0

	for i := 0; i < MAX_CLIENTS; i++ {
		if cl.players[i].name == "" || cl.players[i].spectator != 0 {
			continue
		}
		if cl.players[i].team == team {
			count++
		}
	}

	return count
}

func MT_NameAndClean_TeamMembers(team string) string {
	namebuf := ""

	for i := 0; i < MAX_CLIENTS; i++ {
		if cl.players[i].name == "" || cl.players[i].spectator != 0 {
			continue
		}

		if cl.players[i].team == team {
			if namebuf != "" {
				namebuf = strlcat(namebuf, match_name_and.string, MAX_STATIC_STRING)
			}

			namebuf = strlcat(namebuf, MT_CleanString(cl.players[i].name, false), MAX_STATIC_STRING)
		}
	}

	return namebuf
}

func MT_MapName() string {
	return strlcpy(TP_MapName(), MAX_OSPATH)
}

func MT_GetPlayerNames() (name1, name2 string) {
	var s1, s2 *string

	for i := 0; i < MAX_CLIENTS; i++ {
		if cl.players[i].name == "" || cl.players[i].spectator != 0 {
			continue
		}

		if s1 == nil {
			s1 = &cl.players[i].name
		} else {
			s2 = &cl.players[i].name
			break
		}
	}

	if s1 != nil {
		name1 = *s1
	}

	if s2 != nil {
		name2 = *s2
	}
	return
}

func MT_GetTeamNames(teams *[MAX_CLIENTS]string, max int) int {
	count := 0

	// C clears only sizeof(pointer) bytes here and leaves the rest of the
	// caller's array as it was
	*teams = [MAX_CLIENTS]string{}

	for i := 0; i < MAX_CLIENTS; i++ {
		if cl.players[i].name == "" || cl.players[i].spectator != 0 {
			continue
		}

		j := 0
		for ; j < i; j++ {
			if cl.players[j].name == "" || cl.players[j].spectator != 0 {
				continue
			}

			if cl.players[i].team == cl.players[j].team {
				break
			}
		}
		if j == i {
			teams[count] = cl.players[i].team
			count++
		}
		if count == max {
			break
		}
	}
	return count
}

func MT_Serverinfo_Race() string {
	return strlcpy(Info_ValueForKey(cl.serverinfo, "race"), MAX_OSPATH)
}

type matchtype_t int

const (
	mt_duel matchtype_t = iota
	mt_ffa
	mt_2on2
	mt_3on3
	mt_4on4
	mt_tdm
	mt_multiteam
	mt_arena
	mt_tf_duel
	mt_tf_clanwar
	mt_solo
	mt_coop
	mt_race
	mt_empty
	mt_unknown_gamedir
	mt_unknown
	mt_numtypes
)

type matchcvar_t struct {
	matchtype matchtype_t
	nickname  string
	format    *cvar_t
	autosshot bool
}

var matchcvars = [mt_numtypes]matchcvar_t{
	{mt_duel, "duel", &match_format_duel, true},
	{mt_ffa, "ffa", &match_format_ffa, true},
	{mt_2on2, "2on2", &match_format_2on2, true},
	{mt_3on3, "3on3", &match_format_3on3, true},
	{mt_4on4, "4on4", &match_format_4on4, true},
	{mt_tdm, "tdm", &match_format_tdm, true},
	{mt_multiteam, "tdm", &match_format_multiteam, true},
	{mt_arena, "arena", &match_format_arena, true},
	{mt_tf_duel, "tfduel", &match_format_tf_duel, true},
	{mt_tf_clanwar, "tfclanwar", &match_format_tf_clanwar, true},
	{mt_solo, "solo", &match_format_solo, false},
	{mt_coop, "coop", &match_format_coop, false},
	{mt_race, "race", &match_format_race, false},
	{mt_empty, "empty", nil, false},
	{mt_unknown_gamedir, "unknown", nil, false},
	{mt_unknown, "unknown", nil, false},
}

type matchinfo_t struct {
	spectator       bool
	myname          string
	player1         string
	player2         string
	team1           string
	team2           string
	team1count      int
	team2count      int
	numteams        int
	team1names      string
	team2names      string
	multiteamnames  string
	multiteamcounts string
	maxteamsize     int
	numplayers      int
	timelimit       int
	fraglimit       int
	teamplay        int
	deathmatch      int
	mapname         string
	gamedir         string
	matchtype       matchtype_t
	day             string
	month           string
	year            string
	bigyear         string
	hour            string
	minute          string
	second          string
}

func MT_GetMatchType(matchinfo *matchinfo_t) matchtype_t {
	if matchinfo.numplayers < 1 {
		return mt_empty
	}

	if Q_strcasecmp(matchinfo.gamedir, "qw") == 0 && MT_Serverinfo_Race() == matchinfo.mapname {
		return mt_race
	}

	if matchinfo.numplayers < 2 {
		return mt_solo
	}

	if cl.teamfortress {
		if matchinfo.numplayers == 2 {
			return mt_tf_duel
		}
		if matchinfo.numteams < 2 {
			return mt_unknown
		}
		return mt_tf_clanwar
	}

	if strings.Contains(matchinfo.gamedir, "arena") {
		return mt_arena
	}

	if matchinfo.deathmatch == 0 {
		return mt_coop
	}

	if matchinfo.numplayers == 2 {
		if matchinfo.teamplay == 0 || matchinfo.numteams == 2 {
			return mt_duel
		}
		return mt_unknown
	}

	if matchinfo.teamplay == 0 || matchinfo.maxteamsize <= 1 {
		return mt_ffa
	}

	if matchinfo.numteams != 2 {
		if matchinfo.numteams > 2 {
			return mt_multiteam
		}
		return mt_unknown
	}

	switch matchinfo.maxteamsize {
	case 2:
		return mt_2on2
	case 3:
		return mt_3on3
	case 4:
		return mt_4on4
	default:
		return mt_tdm
	}
}

var mt_matchinfo matchinfo_t

func MT_GetMatchInfo() *matchinfo_t {
	var teamnames [MAX_CLIENTS]string

	mt_matchinfo = matchinfo_t{}
	matchinfo := &mt_matchinfo
	numteams := MT_GetTeamNames(&teamnames, MAX_CLIENTS)

	matchinfo.spectator = cl.spectator != 0
	matchinfo.myname = strlcpy(MT_PlayerName(), MAX_INFO_STRING)

	if cl.spectator != 0 {
		matchinfo.player1, matchinfo.player2 = MT_GetPlayerNames()
		matchinfo.team1 = strlcpy(teamnames[0], MAX_INFO_STRING)
		matchinfo.team2 = strlcpy(teamnames[1], MAX_INFO_STRING)
	} else {
		matchinfo.player1 = strlcpy(MT_PlayerName(), MAX_INFO_STRING)
		matchinfo.player2 = strlcpy(MT_EnemyName(), MAX_INFO_STRING)
		matchinfo.team1 = strlcpy(MT_PlayerTeam(), MAX_INFO_STRING)
		matchinfo.team2 = strlcpy(MT_EnemyTeam(), MAX_INFO_STRING)
	}

	matchinfo.team1count = MT_CountTeamMembers(matchinfo.team1)
	matchinfo.team2count = MT_CountTeamMembers(matchinfo.team2)
	matchinfo.numteams = numteams
	matchinfo.team1names = strlcpy(MT_NameAndClean_TeamMembers(matchinfo.team1), MAX_STATIC_STRING)
	matchinfo.team2names = strlcpy(MT_NameAndClean_TeamMembers(matchinfo.team2), MAX_STATIC_STRING)

	matchinfo.myname = strlcpy(MT_CleanString(matchinfo.myname, false), MAX_INFO_STRING)
	matchinfo.player1 = strlcpy(MT_CleanString(matchinfo.player1, false), MAX_INFO_STRING)
	matchinfo.player2 = strlcpy(MT_CleanString(matchinfo.player2, false), MAX_INFO_STRING)
	matchinfo.team1 = strlcpy(MT_CleanString(matchinfo.team1, false), MAX_INFO_STRING)
	matchinfo.team2 = strlcpy(MT_CleanString(matchinfo.team2, false), MAX_INFO_STRING)

	for i := 0; i < numteams; i++ {
		matchinfo.multiteamnames = strlcat(matchinfo.multiteamnames, MT_CleanString(teamnames[i], false), MAX_STATIC_STRING)
		if i < numteams-1 {
			matchinfo.multiteamnames = strlcat(matchinfo.multiteamnames, match_name_versus.string, MAX_STATIC_STRING)
		}
	}

	maxteamsize := 0
	for i := 0; i < numteams; i++ {
		teamsize := MT_CountTeamMembers(teamnames[i])
		if teamnames[i] != "" {
			maxteamsize = max(maxteamsize, teamsize)
		}
		matchinfo.multiteamcounts = strlcat(matchinfo.multiteamcounts, va("%d", teamsize), 128)
		if i < numteams-1 {
			matchinfo.multiteamcounts = strlcat(matchinfo.multiteamcounts, match_name_on.string, 128)
		}
	}
	matchinfo.maxteamsize = maxteamsize

	matchinfo.numplayers = MT_CountPlayers()

	matchinfo.timelimit = Q_atoi(Info_ValueForKey(cl.serverinfo, "timelimit"))
	matchinfo.fraglimit = Q_atoi(Info_ValueForKey(cl.serverinfo, "fraglimit"))
	matchinfo.teamplay = Q_atoi(Info_ValueForKey(cl.serverinfo, "teamplay"))
	matchinfo.deathmatch = cl.deathmatch

	matchinfo.mapname = strlcpy(MT_MapName(), MAX_OSPATH)
	matchinfo.gamedir = strlcpy(cls.gamedirfile, MAX_OSPATH)

	matchinfo.matchtype = MT_GetMatchType(matchinfo)

	// strftime with %d %m %y %Y %H %M %S
	now := time.Now()
	matchinfo.day = now.Format("02")
	matchinfo.month = now.Format("01")
	matchinfo.year = now.Format("06")
	matchinfo.bigyear = now.Format("2006")
	matchinfo.hour = now.Format("15")
	matchinfo.minute = now.Format("04")
	matchinfo.second = now.Format("05")

	return matchinfo
}

func MT_Macrolist_f() {
	if Cmd_Argc() != 1 {
		Com_Printf("%s: no arguments expected\n", Cmd_Argv(0))
	}

	Com_Printf("\x02The following macros can be used to name your matches\n")
	Com_Printf("\x02The square brackets apply when spectating a match\n\n")
	Com_Printf("\x02%%n")
	Com_Printf(" - your nick [followed by match_name_spec]\n")
	Com_Printf("\x02%%p")
	Com_Printf(" - your name [player1's name]\n")
	Com_Printf("\x02%%t")
	Com_Printf(" - your team [team1's name]\n")
	Com_Printf("\x02%%e")
	Com_Printf(" - enemy nick in duels, enemy team in tp [player2/team2]\n")

	Com_Printf("\x02%%k")
	Com_Printf(" - names of players on your team [team1]\n")
	Com_Printf("\x02%%l")
	Com_Printf(" - names of players on enemy team [team2]\n")

	Com_Printf("\x02%%O")
	Com_Printf(" - number of teammates [number on team1]\n")
	Com_Printf("\x02%%E")
	Com_Printf(" - number of enemies [number on team2]\n")
	Com_Printf("\x02%%C")
	Com_Printf(" - number of players on the server\n")

	Com_Printf("\x02%%a")
	Com_Printf(" - team counts separated by match_name_on (eg 4on3on4)\n")
	Com_Printf("\x02%%b")
	Com_Printf(" - team names separated by match_name_versus\n")
	Com_Printf("\x02%%v")
	Com_Printf(" - shortcut for $match_name_versus\n")

	Com_Printf("\x02%%T")
	Com_Printf(" - timelimit on the server\n")
	Com_Printf("\x02%%F")
	Com_Printf(" - fraglimit n the server\n")
	Com_Printf("\x02%%p")
	Com_Printf(" - teamplay setting on the server\n")
	Com_Printf("\x02%%D")
	Com_Printf(" - deathmatch mode on the server\n")

	Com_Printf("\x02%%M")
	Com_Printf(" - mapname\n")
	Com_Printf("\x02%%G")
	Com_Printf(" - gamedir (eg. qw, fortress, arena, etc)\n")

	Com_Printf("\x02%%d")
	Com_Printf(" - day\n")
	Com_Printf("\x02%%m")
	Com_Printf(" - month\n")
	Com_Printf("\x02%%H")
	Com_Printf(" - hour\n")
	Com_Printf("\x02%%Q")
	Com_Printf(" - minute\n")
	Com_Printf("\x02%%S")
	Com_Printf(" - second\n")
	Com_Printf("\x02%%y")
	Com_Printf(" - year (without century)\n")
	Com_Printf("\x02%%Y")
	Com_Printf(" - year (with century)\n")
}

func MT_ParseFormat(format string, matchinfo *matchinfo_t) string {
	buf := make([]byte, 0, MAX_STATIC_STRING)
	in := 0

	for in < len(format) && len(buf) < MAX_STATIC_STRING-1 {
		c := format[in]
		in++
		if c == '%' && in < len(format) {
			var temp string
			c = format[in]
			in++
			switch c {
			case '%':
				temp = "%"
			case 'n':
				if match_name_nick.string != "" {
					temp = match_name_nick.string
				} else {
					temp = matchinfo.myname
				}
				if cl.spectator != 0 {
					temp = va("%s%s", temp, match_name_spec.string)
				}
			case 'p':
				temp = matchinfo.player1
			case 't':
				temp = matchinfo.team1
			case 'e':
				if matchinfo.numplayers == 2 {
					temp = matchinfo.player2
				} else {
					temp = matchinfo.team2
				}
			case 'k':
				temp = matchinfo.team1names
			case 'l':
				temp = matchinfo.team2names
			case 'O':
				if matchinfo.teamplay != 0 {
					temp = va("%d", matchinfo.team1count)
				} else {
					temp = "1"
				}
			case 'E':
				if matchinfo.teamplay != 0 {
					temp = va("%d", matchinfo.team2count)
				} else {
					temp = va("%d", matchinfo.numplayers-1)
				}
			case 'C':
				temp = va("%d", matchinfo.numplayers)
			case 'a':
				temp = matchinfo.multiteamcounts
			case 'b':
				temp = matchinfo.multiteamnames
			case 'v':
				temp = match_name_versus.string
			case 'T':
				temp = va("%d", matchinfo.timelimit)
			case 'F':
				temp = va("%d", matchinfo.fraglimit)
			case 'N':
				temp = va("%d", matchinfo.teamplay)
			case 'D':
				temp = va("%d", matchinfo.deathmatch)
			case 'M':
				temp = matchinfo.mapname
			case 'G':
				temp = matchinfo.gamedir
			case 'd':
				temp = matchinfo.day
			case 'm':
				temp = matchinfo.month
			case 'y':
				temp = matchinfo.year
			case 'Y':
				temp = matchinfo.bigyear
			case 'H':
				temp = matchinfo.hour
			case 'Q':
				temp = matchinfo.minute
			case 'S':
				temp = matchinfo.second
			default:
				temp = va("%%%c", c)
			}
			buf = append(buf, strlcpy(temp, MAX_STATIC_STRING-len(buf))...)
		} else {
			buf = append(buf, c)
		}
	}

	return string(buf)
}

func MT_NameForMatchInfo(matchinfo *matchinfo_t) string {
	var format string

	switch matchinfo.matchtype {
	case mt_empty:
		format = "%n - [%M]"
	case mt_unknown_gamedir:
		format = "%n - [gamedir - %G, %C players] - [%M]"
	case mt_unknown:
		format = "%n - [Unknown Game, gamedir - %G, %C players] - [%M]"
	default:
		if matchinfo.matchtype >= 0 && matchinfo.matchtype < mt_numtypes {
			if matchcvars[matchinfo.matchtype].format == nil {
				Sys_Error("MT_NameForMatchInfo: NULL format")
			}
			format = matchcvars[matchinfo.matchtype].format.string
		} else {
			Sys_Error("Macro_Matchdesc : Unknown match type %d", matchinfo.matchtype)
		}
	}

	return MT_CleanString(MT_ParseFormat(format, matchinfo), true)
}

func Macro_MatchName() string {
	if cls.state < ca_active {
		return "No match in progress"
	}
	return MT_NameForMatchInfo(MT_GetMatchInfo())
}

func MT_MatchName() string {
	return strlcpy(Macro_MatchName(), MAX_STATIC_STRING)
}

const MT_SCOREBOARD_SHOWIME = 4

var (
	match_auto_record     = cvar_t{name: "match_auto_record", string: "0"}
	match_auto_logconsole = cvar_t{name: "match_auto_logconsole", string: "0"}
	match_auto_sshot      = cvar_t{name: "match_auto_sshot", string: "0"}
	match_auto_minlength  = cvar_t{name: "match_auto_minlength", string: "30"}
	match_auto_spectating = cvar_t{name: "match_auto_spectating", string: "0"}
)

type mt_matchstate_t struct {
	standby      bool
	intermission int
	status       int
	starttime    float32
	endtime      float32
	matchname    string
	matchtype    matchtype_t
}

var matchstate mt_matchstate_t

func MT_Delayed_EndMatch() {
	matchstate.endtime = 0

	Log_AutoLogging_StopMatch()
	CL_AutoRecord_StopMatch()
}

func MT_EndMatch() {
	MT_TakeScreenshot()

	if matchstate.status == 0 {
		return
	}

	matchstate.starttime = 0
	matchstate.status = 0
	matchstate.endtime = float32(cls.realtime)
}

func MT_CancelMatch() {
	if matchstate.endtime != 0 {
		MT_Delayed_EndMatch()
	}

	if matchstate.status == 0 {
		return
	}

	matchstate.starttime = 0
	matchstate.status = 0
	matchstate.endtime = 0

	Log_AutoLogging_CancelMatch()
	CL_AutoRecord_CancelMatch()
}

func MT_StartMatch() {
	if matchstate.endtime != 0 {
		MT_Delayed_EndMatch()
	}

	if matchstate.status != 0 {
		MT_CancelMatch()
	}

	matchstate.status = 1
	matchstate.starttime = float32(cls.realtime)
	matchstate.endtime = 0

	if cls.state < ca_active {
		matchstate.matchtype = mt_empty
		matchstate.matchname = strlcpy("No match in progress", 2*MAX_OSPATH)
	} else {
		matchinfo := MT_GetMatchInfo()
		matchstate.matchtype = matchinfo.matchtype
		matchstate.matchname = strlcpy(MT_NameForMatchInfo(matchinfo), 2*MAX_OSPATH)
	}

	CL_AutoRecord_StartMatch(matchstate.matchname)
	Log_AutoLogging_StartMatch(matchstate.matchname)

	Stats_Reset()
}

func MT_ClearClientState() {
	matchstate = mt_matchstate_t{}
}

func MT_Frame() {
	if matchstate.endtime != 0 && cls.realtime >= float64(matchstate.endtime+MT_SCOREBOARD_SHOWIME) {
		MT_Delayed_EndMatch()
	}

	if cls.state != ca_active || cls.demoplayback {
		return
	}

	if matchstate.standby && !cl.standby && cl.intermission == 0 {
		if cl.spectator == 0 || match_auto_spectating.value != 0 {
			MT_StartMatch()
		}
	}

	if matchstate.intermission == 0 && cl.intermission != 0 {
		MT_EndMatch()
	} else if matchstate.status != 0 && !matchstate.standby && cl.standby {
		MT_CancelMatch()
	}

	matchstate.standby = cl.standby
	matchstate.intermission = cl.intermission
}

func MT_NewMap() {
	MT_CancelMatch()
	MT_ClearClientState()
}

func MT_Disconnect() {
	MT_CancelMatch()
	MT_ClearClientState()
}

var mt_tempdir string

func MT_TempDirectory() string {
	if mt_tempdir == "" {
		mt_tempdir = snprintf(MAX_OSPATH*2, "%s/classicq/temp", com_basedir)
	}

	return mt_tempdir
}

func MT_SaveMatch_f() {
	demo_status := CL_AutoRecord_Status()
	log_status := Log_AutoLogging_Status()

	if (demo_status&2) != 0 || (log_status&2) != 0 {
		Com_Printf("\x02Saving match...\n")
		CL_AutoRecord_SaveMatch()
		Log_AutoLogging_SaveMatch()
	} else {
		if (demo_status&1) != 0 || (log_status&1) != 0 {
			if (demo_status&1) != 0 && (log_status&1) != 0 {
				Com_Printf("Auto demo recording and console logging still in progress\n")
			} else if (demo_status & 1) != 0 {
				Com_Printf("Auto demo recording still in progress\n")
			} else if (log_status & 1) != 0 {
				Com_Printf("Auto console logging still in progress\n")
			}
		} else {
			Com_Printf("Nothing to save!\n")
		}
	}
}

func MT_Match_ForceStart_f() {
	switch Cmd_Argc() {
	case 1:
		if cls.state != ca_active || cls.demoplayback {
			Com_Printf("You must be connected to a server before using \"%s\"\n", Cmd_Argv(0))
			return
		}
		MT_StartMatch()
	default:
		Com_Printf("%s : no arguments expected\n", Cmd_Argv(0))
	}
}

func MT_TakeScreenshot() {
	if match_auto_sshot.value == 0 || (cl.spectator != 0 && match_auto_spectating.value == 0) {
		return
	}

	// don't bother screen-shotting solo games etc
	if !matchcvars[matchstate.matchtype].autosshot {
		return
	}

	// make sure there are actually some frags on the board, and somebody besides us
	have_opponent := false
	for i := 0; i < MAX_CLIENTS; i++ {
		if cl.players[i].name == "" || cl.players[i].spectator != 0 {
			continue
		}
		if i != cl.playernum {
			have_opponent = true
		}
	}
	if !have_opponent {
		Com_Printf("Auto screenshot cancelled\n")
		return
	}

	if matchstate.status == 0 {
		matchinfo := MT_GetMatchInfo()
		matchstate.matchtype = matchinfo.matchtype
		matchstate.matchname = strlcpy(MT_NameForMatchInfo(matchinfo), 2*MAX_OSPATH)
	}
	SCR_AutoScreenshot(matchstate.matchname)
}

const MAX_GROUP_MEMBERS = 36

type mapgroup_t struct {
	groupname  string
	members    [MAX_GROUP_MEMBERS]string
	next, prev *mapgroup_t
	system     bool
	nummembers int
}

var (
	mapgroups            *mapgroup_t
	last_system_mapgroup *mapgroup_t
	mapgroups_init       = false
)

func FIRSTUSERGROUP() *mapgroup_t {
	if last_system_mapgroup != nil {
		return last_system_mapgroup.next
	}
	return mapgroups
}

func GetGroupWithName(groupname string) *mapgroup_t {
	for node := mapgroups; node != nil; node = node.next {
		if Q_strcasecmp(node.groupname, groupname) == 0 {
			return node
		}
	}
	return nil
}

func GetGroupWithMember(member string) *mapgroup_t {
	for node := mapgroups; node != nil; node = node.next {
		for j := 0; j < node.nummembers; j++ {
			if Q_strcasecmp(node.members[j], member) == 0 {
				return node
			}
		}
	}
	return nil
}

func DeleteMapGroup(group *mapgroup_t) {
	if group == nil {
		return
	}

	if group.prev != nil {
		group.prev.next = group.next
	}
	if group.next != nil {
		group.next.prev = group.prev
	}

	if group == mapgroups {
		mapgroups = mapgroups.next
		if group == last_system_mapgroup {
			last_system_mapgroup = nil
		}
	} else if group == last_system_mapgroup {
		last_system_mapgroup = last_system_mapgroup.prev
	}
}

func ResetGroupMembers(group *mapgroup_t) {
	if group == nil {
		return
	}

	for i := 0; i < group.nummembers; i++ {
		group.members[i] = ""
	}

	group.nummembers = 0
}

func DeleteGroupMember(group *mapgroup_t, member string) {
	if group == nil {
		return
	}

	i := 0
	for ; i < group.nummembers; i++ {
		if Q_strcasecmp(member, group.members[i]) == 0 {
			break
		}
	}

	if i == group.nummembers {
		return
	}

	if i < group.nummembers-1 {
		copy(group.members[i:], group.members[i+1:group.nummembers])
	}

	group.nummembers--
}

func AddGroupMember(group *mapgroup_t, member string) {
	if group == nil || group.nummembers == MAX_GROUP_MEMBERS {
		return
	}

	for i := 0; i < group.nummembers; i++ {
		if Q_strcasecmp(member, group.members[i]) == 0 {
			return
		}
	}

	group.members[group.nummembers] = strlcpy(member, MAX_QPATH)
	group.nummembers++
}

func MT_MapGroup_f() {
	removeflag := false

	c := Cmd_Argc()
	if c == 1 {
		if FIRSTUSERGROUP() == nil {
			Com_Printf("No map groups defined\n")
		} else {
			for node := FIRSTUSERGROUP(); node != nil; node = node.next {
				Com_Printf("\x02%s: ", node.groupname)
				for j := 0; j < node.nummembers; j++ {
					Com_Printf("%s ", node.members[j])
				}
				Com_Printf("\n")
			}
		}
		return
	}

	groupname := Cmd_Argv(1)

	if c == 2 && Q_strcasecmp(groupname, "clear") == 0 {
		var tempnode *mapgroup_t
		for node := FIRSTUSERGROUP(); node != nil; node = tempnode {
			tempnode = node.next
			DeleteMapGroup(node)
		}
		return
	}

	if !Util_Is_Valid_Filename(groupname) {
		Com_Printf("Error: %s is not a valid map group name\n", groupname)
		return
	}

	group := GetGroupWithName(groupname)

	if c == 2 {
		if group == nil {
			Com_Printf("No map group named \"%s\"\n", groupname)
		} else {
			Com_Printf("\x02%s: ", groupname)
			for j := 0; j < group.nummembers; j++ {
				Com_Printf("%s ", group.members[j])
			}
			Com_Printf("\n")
		}
		return
	}

	if group != nil && group.system {
		Com_Printf("Cannot modify system group \"%s\"\n", groupname)
		return
	}

	if c == 3 && Q_strcasecmp(Cmd_Argv(2), "clear") == 0 {
		if group == nil {
			Com_Printf("\"%s\" is not a map group name\n", groupname)
		} else {
			DeleteMapGroup(group)
		}
		return
	}

	if group == nil {
		group = &mapgroup_t{}
		group.groupname = strlcpy(groupname, MAX_QPATH)
		group.system = !mapgroups_init
		if mapgroups != nil {
			tempnode := mapgroups
			for ; tempnode.next != nil; tempnode = tempnode.next {
			}
			tempnode.next = group
			group.prev = tempnode
		} else {
			mapgroups = group
		}
	} else {
		member := Cmd_Argv(2)
		if at(member, 0) != '+' && at(member, 0) != '-' {
			ResetGroupMembers(group)
		}
	}

	for i := 2; i < c; i++ {
		member := Cmd_Argv(i)
		if at(member, 0) == '+' {
			removeflag = false
			member = member[1:]
		} else if at(member, 0) == '-' {
			removeflag = true
			member = member[1:]
		}

		if !removeflag {
			if tempnode := GetGroupWithMember(member); tempnode != nil && tempnode != group {
				if cl_warncmd.value != 0 || developer.value != 0 {
					Com_Printf("Warning: \"%s\" is already a member of group \"%s\"...ignoring\n", member, tempnode.groupname)
				}
				continue
			}
		}

		if removeflag {
			DeleteGroupMember(group, member)
		} else {
			AddGroupMember(group, member)
		}
	}

	if group.nummembers == 0 {
		DeleteMapGroup(group)
	}
}

func MT_AddMapGroups() {
	Cmd_TokenizeString("mapgroup exmy start e1m1 e1m2 e1m3 e1m4 e1m5 e1m6 e1m7 e1m8 e2m1 e2m2 e2m3 e2m4 e2m5 e2m6 e2m7 e3m1 e3m2 e3m3 e3m4 e3m5 e3m6 e3m7 e4m1 e4m2 e4m3 e4m4 e4m5 e4m6 e4m7 e4m8")
	MT_MapGroup_f()

	tempnode := mapgroups
	for ; tempnode.next != nil; tempnode = tempnode.next {
	}
	last_system_mapgroup = tempnode

	mapgroups_init = true
}

// "" stands for the NULL of C
func MT_GetMapGroupName(mapname string, system *bool) string {
	group := GetGroupWithMember(mapname)
	if group != nil && mapname != group.groupname {
		if system != nil {
			*system = group.system
		}
		return group.groupname
	}
	return ""
}

func DumpMapGroups(f io.Writer) {
	if FIRSTUSERGROUP() == nil {
		fmt.Fprintf(f, "mapgroup clear\n")
		return
	}
	for node := FIRSTUSERGROUP(); node != nil; node = node.next {
		fmt.Fprintf(f, "mapgroup %s ", node.groupname)
		for j := 0; j < node.nummembers; j++ {
			fmt.Fprintf(f, "%s ", node.members[j])
		}
		fmt.Fprintf(f, "\n")
	}
}

func Macro_MatchType() string {
	if cls.state < ca_active {
		return "No match in progress"
	}

	matchinfo := MT_GetMatchInfo()

	return matchcvars[matchinfo.matchtype].nickname
}

func MT_CvarInit() {
	Cmd_AddMacro("matchname", Macro_MatchName)
	Cmd_AddMacro("matchtype", Macro_MatchType)

	Cmd_AddCommand("match_format_macrolist", MT_Macrolist_f)
	Cmd_AddCommand("match_forcestart", MT_Match_ForceStart_f)
	Cmd_AddCommand("match_save", MT_SaveMatch_f)
	Cmd_AddCommand("mapgroup", MT_MapGroup_f)

	Cvar_SetCurrentGroup(CVAR_GROUP_MATCH_TOOLS)
	Cvar_Register(&match_format_solo)
	Cvar_Register(&match_format_coop)
	Cvar_Register(&match_format_race)
	Cvar_Register(&match_format_duel)
	Cvar_Register(&match_format_ffa)
	Cvar_Register(&match_format_2on2)
	Cvar_Register(&match_format_3on3)
	Cvar_Register(&match_format_4on4)
	Cvar_Register(&match_format_tdm)
	Cvar_Register(&match_format_multiteam)
	Cvar_Register(&match_format_arena)
	Cvar_Register(&match_format_tf_duel)
	Cvar_Register(&match_format_tf_clanwar)

	Cvar_Register(&match_name_and)
	Cvar_Register(&match_name_versus)
	Cvar_Register(&match_name_on)

	Cvar_Register(&match_name_nick)
	Cvar_Register(&match_name_spec)

	Cvar_Register(&match_auto_record)
	Cvar_Register(&match_auto_logconsole)
	Cvar_Register(&match_auto_sshot)
	Cvar_Register(&match_auto_minlength)
	Cvar_Register(&match_auto_spectating)

	Cvar_ResetCurrentGroup()
}

func MT_Init() {
	Sys_IO_Create_Directory(va("%s/classicq/temp", com_basedir))

	MT_ClearClientState()

	MT_AddMapGroups()
}

package engine

// sbar.c -- status bar code

import (
	"fmt"
	"math"
	"strings"
	"unsafe"
)

var sb_inited int

var sb_updates int // if >= vid.numpages, no update needed

const STAT_MINUS = 10 // num frame for '-' stats digit
var sb_nums [2][11]*Picture
var sb_colon, sb_slash *Picture
var sb_ibar *Picture
var sb_sbar *Picture
var sb_scorebar *Picture

var sb_weapons [7][8]*Picture // 0 is active, 1 is owned, 2-5 are flashes
var sb_ammo [4]*Picture
var sb_sigil [4]*Picture
var sb_armor [3]*Picture
var sb_armor_invuln *Picture
var sb_items [32]*Picture

var sb_faces [5][2]*Picture // 0 is dead, 1-4 are alive
// 0 is static, 1 is temporary animation
var sb_face_invis *Picture
var sb_face_quad *Picture
var sb_face_invuln *Picture
var sb_face_invis_invuln *Picture

var rankingpic *Picture

var completepic *Picture
var interpic *Picture
var finalepic *Picture

var sb_showscores bool
var sb_showteamscores bool

var sb_lines int // scan lines to draw

var sbar_xofs int

const SBAR_HEIGHT = 24

var scr_centerSbar = cvar_t{name: "scr_centerSbar", string: "1"}

var scr_compactHud = cvar_t{name: "scr_compactHud", string: "0"}
var scr_compactHudAlign = cvar_t{name: "scr_compactHudAlign", string: "0"}

var scr_drawHFrags = cvar_t{name: "scr_drawHFrags", string: "1"}
var scr_drawVFrags = cvar_t{name: "scr_drawVFrags", string: "1"}

var scr_scoreboard_teamsort = cvar_t{name: "scr_scoreboard_teamsort", string: "1"}
var scr_scoreboard_forcecolors = cvar_t{name: "scr_scoreboard_forcecolors", string: "0"}
var scr_scoreboard_showfrags = cvar_t{name: "scr_scoreboard_showfrags", string: "1"}
var scr_scoreboard_drawtitle = cvar_t{name: "scr_scoreboard_drawtitle", string: "1"}
var scr_scoreboard_borderless = cvar_t{name: "scr_scoreboard_borderless", string: "1"}
var scr_scoreboard_titleseperator = cvar_t{name: "scr_scoreboard_titleseperator", string: "1"}

var scr_scoreboard_fillalpha = cvar_t{name: "scr_scoreboard_fillalpha", string: "0"}
var scr_scoreboard_fillcolored = cvar_t{name: "scr_scoreboard_fillcolored", string: "0"}

/********************************** CONTROL **********************************/

// Tab key down
func Sbar_ShowTeamScores() {
	if sb_showteamscores {
		return
	}

	sb_showteamscores = true
	sb_updates = 0
}

// Tab key up
func Sbar_DontShowTeamScores() {
	sb_showteamscores = false
	sb_updates = 0
}

// Tab key down
func Sbar_ShowScores() {
	if sb_showscores {
		return
	}

	sb_showscores = true
	sb_updates = 0
}

// Tab key up
func Sbar_DontShowScores() {
	sb_showscores = false
	sb_updates = 0
}

func Sbar_Changed() {
	sb_updates = 0 // update next frame
}

/************************************ INIT ************************************/

func Sbar_CvarInit() {
	Cvar_SetCurrentGroup(CVAR_GROUP_SBAR)
	Cvar_Register(&scr_centerSbar)

	Cvar_Register(&scr_compactHud)
	Cvar_Register(&scr_compactHudAlign)

	Cvar_Register(&scr_drawHFrags)
	Cvar_Register(&scr_drawVFrags)
	Cvar_Register(&scr_scoreboard_teamsort)
	Cvar_Register(&scr_scoreboard_forcecolors)
	Cvar_Register(&scr_scoreboard_showfrags)
	Cvar_Register(&scr_scoreboard_drawtitle)
	Cvar_Register(&scr_scoreboard_borderless)
	Cvar_Register(&scr_scoreboard_titleseperator)
	Cvar_Register(&scr_scoreboard_fillalpha)
	Cvar_Register(&scr_scoreboard_fillcolored)

	Cvar_ResetCurrentGroup()

	Cmd_AddCommand("+showscores", Sbar_ShowScores)
	Cmd_AddCommand("-showscores", Sbar_DontShowScores)

	Cmd_AddCommand("+showteamscores", Sbar_ShowTeamScores)
	Cmd_AddCommand("-showteamscores", Sbar_DontShowTeamScores)
}

func Sbar_Init() {
	var i int

	for i = 0; i < 10; i++ {
		sb_nums[0][i] = Draw_LoadPicture(va("wad:num_%d", i), DRAW_LOADPICTURE_DUMMYFALLBACK)
		sb_nums[1][i] = Draw_LoadPicture(va("wad:anum_%d", i), DRAW_LOADPICTURE_DUMMYFALLBACK)
	}

	sb_nums[0][10] = Draw_LoadPicture("wad:num_minus", DRAW_LOADPICTURE_DUMMYFALLBACK)
	sb_nums[1][10] = Draw_LoadPicture("wad:anum_minus", DRAW_LOADPICTURE_DUMMYFALLBACK)

	sb_colon = Draw_LoadPicture("wad:num_colon", DRAW_LOADPICTURE_DUMMYFALLBACK)
	sb_slash = Draw_LoadPicture("wad:num_slash", DRAW_LOADPICTURE_DUMMYFALLBACK)

	sb_weapons[0][0] = Draw_LoadPicture("wad:inv_shotgun", DRAW_LOADPICTURE_DUMMYFALLBACK)
	sb_weapons[0][1] = Draw_LoadPicture("wad:inv_sshotgun", DRAW_LOADPICTURE_DUMMYFALLBACK)
	sb_weapons[0][2] = Draw_LoadPicture("wad:inv_nailgun", DRAW_LOADPICTURE_DUMMYFALLBACK)
	sb_weapons[0][3] = Draw_LoadPicture("wad:inv_snailgun", DRAW_LOADPICTURE_DUMMYFALLBACK)
	sb_weapons[0][4] = Draw_LoadPicture("wad:inv_rlaunch", DRAW_LOADPICTURE_DUMMYFALLBACK)
	sb_weapons[0][5] = Draw_LoadPicture("wad:inv_srlaunch", DRAW_LOADPICTURE_DUMMYFALLBACK)
	sb_weapons[0][6] = Draw_LoadPicture("wad:inv_lightng", DRAW_LOADPICTURE_DUMMYFALLBACK)

	sb_weapons[1][0] = Draw_LoadPicture("wad:inv2_shotgun", DRAW_LOADPICTURE_DUMMYFALLBACK)
	sb_weapons[1][1] = Draw_LoadPicture("wad:inv2_sshotgun", DRAW_LOADPICTURE_DUMMYFALLBACK)
	sb_weapons[1][2] = Draw_LoadPicture("wad:inv2_nailgun", DRAW_LOADPICTURE_DUMMYFALLBACK)
	sb_weapons[1][3] = Draw_LoadPicture("wad:inv2_snailgun", DRAW_LOADPICTURE_DUMMYFALLBACK)
	sb_weapons[1][4] = Draw_LoadPicture("wad:inv2_rlaunch", DRAW_LOADPICTURE_DUMMYFALLBACK)
	sb_weapons[1][5] = Draw_LoadPicture("wad:inv2_srlaunch", DRAW_LOADPICTURE_DUMMYFALLBACK)
	sb_weapons[1][6] = Draw_LoadPicture("wad:inv2_lightng", DRAW_LOADPICTURE_DUMMYFALLBACK)

	for i = 0; i < 5; i++ {
		sb_weapons[2+i][0] = Draw_LoadPicture(va("wad:inva%d_shotgun", i+1), DRAW_LOADPICTURE_DUMMYFALLBACK)
		sb_weapons[2+i][1] = Draw_LoadPicture(va("wad:inva%d_sshotgun", i+1), DRAW_LOADPICTURE_DUMMYFALLBACK)
		sb_weapons[2+i][2] = Draw_LoadPicture(va("wad:inva%d_nailgun", i+1), DRAW_LOADPICTURE_DUMMYFALLBACK)
		sb_weapons[2+i][3] = Draw_LoadPicture(va("wad:inva%d_snailgun", i+1), DRAW_LOADPICTURE_DUMMYFALLBACK)
		sb_weapons[2+i][4] = Draw_LoadPicture(va("wad:inva%d_rlaunch", i+1), DRAW_LOADPICTURE_DUMMYFALLBACK)
		sb_weapons[2+i][5] = Draw_LoadPicture(va("wad:inva%d_srlaunch", i+1), DRAW_LOADPICTURE_DUMMYFALLBACK)
		sb_weapons[2+i][6] = Draw_LoadPicture(va("wad:inva%d_lightng", i+1), DRAW_LOADPICTURE_DUMMYFALLBACK)
	}

	sb_ammo[0] = Draw_LoadPicture("wad:sb_shells", DRAW_LOADPICTURE_DUMMYFALLBACK)
	sb_ammo[1] = Draw_LoadPicture("wad:sb_nails", DRAW_LOADPICTURE_DUMMYFALLBACK)
	sb_ammo[2] = Draw_LoadPicture("wad:sb_rocket", DRAW_LOADPICTURE_DUMMYFALLBACK)
	sb_ammo[3] = Draw_LoadPicture("wad:sb_cells", DRAW_LOADPICTURE_DUMMYFALLBACK)

	sb_armor[0] = Draw_LoadPicture("wad:sb_armor1", DRAW_LOADPICTURE_DUMMYFALLBACK)
	sb_armor[1] = Draw_LoadPicture("wad:sb_armor2", DRAW_LOADPICTURE_DUMMYFALLBACK)
	sb_armor[2] = Draw_LoadPicture("wad:sb_armor3", DRAW_LOADPICTURE_DUMMYFALLBACK)
	sb_armor_invuln = Draw_LoadPicture("wad:disc", DRAW_LOADPICTURE_DUMMYFALLBACK)

	sb_items[0] = Draw_LoadPicture("wad:sb_key1", DRAW_LOADPICTURE_DUMMYFALLBACK)
	sb_items[1] = Draw_LoadPicture("wad:sb_key2", DRAW_LOADPICTURE_DUMMYFALLBACK)
	sb_items[2] = Draw_LoadPicture("wad:sb_invis", DRAW_LOADPICTURE_DUMMYFALLBACK)
	sb_items[3] = Draw_LoadPicture("wad:sb_invuln", DRAW_LOADPICTURE_DUMMYFALLBACK)
	sb_items[4] = Draw_LoadPicture("wad:sb_suit", DRAW_LOADPICTURE_DUMMYFALLBACK)
	sb_items[5] = Draw_LoadPicture("wad:sb_quad", DRAW_LOADPICTURE_DUMMYFALLBACK)

	sb_sigil[0] = Draw_LoadPicture("wad:sb_sigil1", DRAW_LOADPICTURE_DUMMYFALLBACK)
	sb_sigil[1] = Draw_LoadPicture("wad:sb_sigil2", DRAW_LOADPICTURE_DUMMYFALLBACK)
	sb_sigil[2] = Draw_LoadPicture("wad:sb_sigil3", DRAW_LOADPICTURE_DUMMYFALLBACK)
	sb_sigil[3] = Draw_LoadPicture("wad:sb_sigil4", DRAW_LOADPICTURE_DUMMYFALLBACK)

	sb_faces[4][0] = Draw_LoadPicture("wad:face1", DRAW_LOADPICTURE_DUMMYFALLBACK)
	sb_faces[4][1] = Draw_LoadPicture("wad:face_p1", DRAW_LOADPICTURE_DUMMYFALLBACK)
	sb_faces[3][0] = Draw_LoadPicture("wad:face2", DRAW_LOADPICTURE_DUMMYFALLBACK)
	sb_faces[3][1] = Draw_LoadPicture("wad:face_p2", DRAW_LOADPICTURE_DUMMYFALLBACK)
	sb_faces[2][0] = Draw_LoadPicture("wad:face3", DRAW_LOADPICTURE_DUMMYFALLBACK)
	sb_faces[2][1] = Draw_LoadPicture("wad:face_p3", DRAW_LOADPICTURE_DUMMYFALLBACK)
	sb_faces[1][0] = Draw_LoadPicture("wad:face4", DRAW_LOADPICTURE_DUMMYFALLBACK)
	sb_faces[1][1] = Draw_LoadPicture("wad:face_p4", DRAW_LOADPICTURE_DUMMYFALLBACK)
	sb_faces[0][0] = Draw_LoadPicture("wad:face5", DRAW_LOADPICTURE_DUMMYFALLBACK)
	sb_faces[0][1] = Draw_LoadPicture("wad:face_p5", DRAW_LOADPICTURE_DUMMYFALLBACK)

	sb_face_invis = Draw_LoadPicture("wad:face_invis", DRAW_LOADPICTURE_DUMMYFALLBACK)
	sb_face_invuln = Draw_LoadPicture("wad:face_invul2", DRAW_LOADPICTURE_DUMMYFALLBACK)
	sb_face_invis_invuln = Draw_LoadPicture("wad:face_inv2", DRAW_LOADPICTURE_DUMMYFALLBACK)
	sb_face_quad = Draw_LoadPicture("wad:face_quad", DRAW_LOADPICTURE_DUMMYFALLBACK)

	sb_sbar = Draw_LoadPicture("wad:sbar", DRAW_LOADPICTURE_DUMMYFALLBACK)
	sb_ibar = Draw_LoadPicture("wad:ibar", DRAW_LOADPICTURE_DUMMYFALLBACK)
	sb_scorebar = Draw_LoadPicture("wad:scorebar", DRAW_LOADPICTURE_DUMMYFALLBACK)

	rankingpic = Draw_LoadPicture("gfx/ranking.lmp", DRAW_LOADPICTURE_DUMMYFALLBACK)

	sb_inited = 1
}

func Sbar_Shutdown() {
	var i, j int

	if sb_inited == 0 {
		return
	}

	sb_inited = 0

	if finalepic != nil {
		Draw_FreePicture(finalepic)
		finalepic = nil
	}

	if completepic != nil {
		Draw_FreePicture(completepic)
		completepic = nil
	}

	if interpic != nil {
		Draw_FreePicture(interpic)
		interpic = nil
	}

	Draw_FreePicture(rankingpic)

	for i = 0; i < 11; i++ {
		Draw_FreePicture(sb_nums[0][i])
		Draw_FreePicture(sb_nums[1][i])
	}

	Draw_FreePicture(sb_colon)
	Draw_FreePicture(sb_slash)

	for i = 0; i < 7; i++ {
		for j = 0; j < 7; j++ {
			Draw_FreePicture(sb_weapons[i][j])
		}
	}

	for i = 0; i < 4; i++ {
		Draw_FreePicture(sb_ammo[i])
	}

	Draw_FreePicture(sb_armor_invuln)

	for i = 0; i < 3; i++ {
		Draw_FreePicture(sb_armor[i])
	}
	for i = 0; i < 6; i++ {
		Draw_FreePicture(sb_items[i])
	}

	for i = 0; i < 4; i++ {
		Draw_FreePicture(sb_sigil[i])
	}

	for i = 0; i < 5; i++ {
		Draw_FreePicture(sb_faces[i][0])
		Draw_FreePicture(sb_faces[i][1])
	}

	Draw_FreePicture(sb_face_invis)
	Draw_FreePicture(sb_face_invuln)
	Draw_FreePicture(sb_face_invis_invuln)
	Draw_FreePicture(sb_face_quad)

	Draw_FreePicture(sb_sbar)
	Draw_FreePicture(sb_ibar)
	Draw_FreePicture(sb_scorebar)
}

/****************************** SUPPORT ROUTINES ******************************/

// drawing routines are relative to the status bar location

func Sbar_DrawPic(x, y int, pic *Picture, width, height int) {
	Draw_DrawPicture(pic, x+sbar_xofs, y+(int(vid.conheight)-SBAR_HEIGHT), uint(width), uint(height))
}

// JACK: Draws a portion of the picture in the status bar.
func Sbar_DrawSubPicNormalised(x, y, width, height int, pic *Picture, srcx, srcy, srcwidth, srcheight float32) {
	Draw_DrawSubPicture(pic, srcx, srcy, srcwidth, srcheight, x, y+(int(vid.conheight)-SBAR_HEIGHT), uint(width), uint(height))
}

func Sbar_DrawTransPic(x, y int, pic *Picture, width, height int) {
	Draw_DrawPicture(pic, x+sbar_xofs, y+(int(vid.conheight)-SBAR_HEIGHT), uint(width), uint(height))
}

// Draws one solid graphics character
func Sbar_DrawCharacter(x, y, num int) {
	Draw_Character(x+4+sbar_xofs, y+int(vid.conheight)-SBAR_HEIGHT, byte(num))
}

func Sbar_DrawString(x, y int, str string) {
	Draw_String(x+sbar_xofs, y+int(vid.conheight)-SBAR_HEIGHT, str)
}

func Sbar_DrawAltString(x, y int, str string) {
	Draw_Alt_String(x+sbar_xofs, y+int(vid.conheight)-SBAR_HEIGHT, str)
}

// static buffer, the result lives until the next call
var sbar_itoa_str [12]byte

func Sbar_itoa(num int) string {
	str := sbar_itoa_str[:0]
	var pow10, dig int

	if num < 0 {
		str = append(str, '-')
		num = -num
	}

	for pow10 = 10; num >= pow10; pow10 *= 10 {
	}

	for {
		pow10 /= 10
		dig = num / pow10
		str = append(str, byte('0'+dig))
		num -= dig * pow10
		if pow10 == 1 {
			break
		}
	}

	return unsafe.String(&str[0], len(str))
}

func Sbar_DrawNum(x, y, num, digits, color int) {
	var str, ptr string
	var l, frame int

	str = Sbar_itoa(num)
	l = len(str)
	ptr = str
	if l > digits {
		ptr = ptr[l-digits:]
	}
	if l < digits {
		x += (digits - l) * 24
	}

	for i := 0; i < len(ptr); i++ {
		if ptr[i] == '-' {
			frame = STAT_MINUS
		} else {
			frame = int(ptr[i] - '0')
		}

		Sbar_DrawTransPic(x, y, sb_nums[color][frame], 24, 24)
		x += 24
	}
}

func Sbar_ColorForMap(m int) int {
	m = bound(0, m, 13)

	return 16*m + 8
}

// scr_scoreboard_forcecolors picks between the forced and the real colours
func sbarPlayerColors(s *player_info_t) (top, bottom int) {
	if scr_scoreboard_forcecolors.value != 0 {
		return int(s.topcolor), int(s.bottomcolor)
	}
	return int(s.real_topcolor), int(s.real_bottomcolor)
}

/********************************* FRAG SORT *********************************/

var fragsort [MAX_CLIENTS]int
var scoreboardlines int

const SCR_TEAM_T_MAXTEAMSIZE = 16 + 1

type team_t struct {
	team                  string // SCR_TEAM_T_MAXTEAMSIZE
	frags                 int
	players               int
	plow, phigh, ptotal   int
	topcolor, bottomcolor int
	myteam                bool
}

var teams [MAX_CLIENTS]team_t

var teamsort [MAX_CLIENTS]int
var scoreboardteams int

func Sbar_PlayerNum() int {
	mynum := -1

	if cl.spectator != 0 {
		mynum = Cam_TrackNum()
	}
	if cl.spectator == 0 || mynum == -1 {
		mynum = cl.playernum
	}

	return mynum
}

func Sbar_IsSpectator(mynum int) bool {
	if mynum == cl.playernum {
		return cl.spectator != 0
	}
	return cl.players[mynum].spectator != 0
}

var sbar_sortfrags_lastframecount = 0

func Sbar_SortFrags(spec bool) {
	var i, j, k int

	if !spec && sbar_sortfrags_lastframecount != 0 && sbar_sortfrags_lastframecount == cls.framecount {
		return
	}

	if spec {
		sbar_sortfrags_lastframecount = 0
	} else {
		sbar_sortfrags_lastframecount = cls.framecount
	}

	// sort by frags
	scoreboardlines = 0
	for i = 0; i < MAX_CLIENTS; i++ {
		if cl.players[i].name != "" && (spec || cl.players[i].spectator == 0) {
			fragsort[scoreboardlines] = i
			scoreboardlines++
			if cl.players[i].spectator != 0 {
				cl.players[i].frags = -999
			}
		}
	}

	for i = 0; i < scoreboardlines; i++ {
		for j = 0; j < scoreboardlines-1-i; j++ {
			if cl.players[fragsort[j]].frags < cl.players[fragsort[j+1]].frags {
				k = fragsort[j]
				fragsort[j] = fragsort[j+1]
				fragsort[j+1] = k
			}
		}
	}
}

var sbar_sortteams_lastframecount = 0

func Sbar_SortTeams() {
	var i, j, k, mynum int
	var playertoteam [MAX_CLIENTS]int
	var s *player_info_t
	var t string

	if cl.teamplay == 0 {
		return
	}

	if sbar_sortteams_lastframecount != 0 && sbar_sortteams_lastframecount == cls.framecount {
		return
	}
	sbar_sortteams_lastframecount = cls.framecount

	scoreboardteams = 0

	for i = 0; i < MAX_CLIENTS; i++ {
		playertoteam[i] = -1
	}

	// sort the teams
	teams = [MAX_CLIENTS]team_t{}
	for i = 0; i < MAX_CLIENTS; i++ {
		teams[i].plow = 999
	}

	mynum = Sbar_PlayerNum()

	for i = 0; i < MAX_CLIENTS; i++ {
		s = &cl.players[i]
		if s.name == "" || s.spectator != 0 {
			continue
		}

		// find his team in the list
		t = strlcpy(s.team, SCR_TEAM_T_MAXTEAMSIZE)
		if t == "" {
			continue // not on team
		}

		for j = 0; j < scoreboardteams; j++ {
			if teams[j].team == t {
				playertoteam[i] = j

				teams[j].frags += s.frags
				teams[j].players++
				if !cl.teamfortress && i == mynum {
					teams[j].topcolor, teams[j].bottomcolor = sbarPlayerColors(s)
				}
				break
			}
		}
		if j == scoreboardteams { // must add him
			j = scoreboardteams
			scoreboardteams++
			playertoteam[i] = j

			teams[j].frags = s.frags
			teams[j].players = 1
			if cl.teamfortress {
				teams[j].topcolor = Utils_TF_TeamToColor(t)
				teams[j].bottomcolor = teams[j].topcolor
			} else {
				teams[j].topcolor, teams[j].bottomcolor = sbarPlayerColors(s)
			}
			teams[j].team = strlcpy(t, SCR_TEAM_T_MAXTEAMSIZE)

			// strncmp over sizeof(t) - 1 characters
			if !Sbar_IsSpectator(mynum) && strlcpy(cl.players[mynum].team, SCR_TEAM_T_MAXTEAMSIZE) == t {
				teams[j].myteam = true
			} else {
				teams[j].myteam = false
			}
		}
		teams[j].plow = min(s.ping, teams[j].plow)
		teams[j].phigh = max(s.ping, teams[j].phigh)
		teams[j].ptotal += s.ping
	}

	// prepare for sort
	for i = 0; i < scoreboardteams; i++ {
		teamsort[i] = i
	}

	// good 'ol bubble sort
	for i = 0; i < scoreboardteams-1; i++ {
		for j = i + 1; j < scoreboardteams; j++ {
			if teams[teamsort[i]].frags < teams[teamsort[j]].frags {
				k = teamsort[i]
				teamsort[i] = teamsort[j]
				teamsort[j] = k
			}
		}
	}

	if cl.teamfortress {
		for i = 0; i < scoreboardteams; i++ {
			frags := float32(teams[i].frags) / float32(teams[i].players)

			if frags != float32(int(frags)) {
				return
			}

			for j = 0; j < MAX_CLIENTS; j++ {
				s = &cl.players[j]
				if s.name == "" || s.spectator != 0 || playertoteam[j] != i {
					continue
				}

				if float32(s.frags) != frags {
					return
				}
			}
		}

		for i = 0; i < scoreboardteams; i++ {
			teams[i].frags /= teams[i].players
		}
	}
}

func Sbar_SortTeamsAndFrags_Compare(a, b int) int {
	var team_comp, frag_comp int
	var team_one, team_two string
	var p1, p2 *player_info_t

	p1 = &cl.players[a]
	p2 = &cl.players[b]

	if p1.spectator != 0 {
		if p2.spectator != 0 {
			return Q_strcasecmp(p1.name, p2.name)
		}
		return 1
	} else if p2.spectator != 0 {
		return -1
	}

	if cl.teamfortress {
		team_one = Utils_TF_ColorToTeam(int(p1.real_bottomcolor))
		team_two = Utils_TF_ColorToTeam(int(p2.real_bottomcolor))
	} else {
		team_one = p1.team
		team_two = p2.team
	}

	if team_comp = strings.Compare(team_one, team_two); team_comp != 0 {
		var i int
		var t1, t2 *team_t

		for i = 0; i < scoreboardteams; i++ {
			if team_one == teams[i].team {
				t1 = &teams[i]
				break
			}
		}
		for i = 0; i < scoreboardteams; i++ {
			if team_two == teams[i].team {
				t2 = &teams[i]
				break
			}
		}
		if t1 != nil && t2 != nil && t1.frags != t2.frags {
			return t2.frags - t1.frags
		}
		return team_comp
	} else {
		if frag_comp = p2.frags - p1.frags; frag_comp != 0 {
			return frag_comp
		}
		return Q_strcasecmp(p1.name, p2.name)
	}
}

func Sbar_SortTeamsAndFrags(specs bool) {
	var i, j, k int

	real_teamplay := cl.teamplay != 0 && (TP_CountPlayers() > 2)

	if !real_teamplay || scr_scoreboard_teamsort.value == 0 {
		Sbar_SortFrags(specs)
		return
	}

	scoreboardlines = 0
	for i = 0; i < MAX_CLIENTS; i++ {
		if cl.players[i].name != "" && (specs || cl.players[i].spectator == 0) {
			fragsort[scoreboardlines] = i
			scoreboardlines++
			if cl.players[i].spectator != 0 {
				cl.players[i].frags = -999
			}
		}
	}

	Sbar_SortTeams()

	for i = 0; i < scoreboardlines; i++ {
		for j = 0; j < scoreboardlines-1-i; j++ {
			if Sbar_SortTeamsAndFrags_Compare(fragsort[j], fragsort[j+1]) > 0 {
				k = fragsort[j]
				fragsort[j] = fragsort[j+1]
				fragsort[j+1] = k
			}
		}
	}
}

/********************************* INVENTORY *********************************/

func Sbar_DrawInventory() {
	var i, flashon int
	var num string
	var time float32
	var headsup, hudswap bool

	headsup = !(cl_sbar.value != 0 || scr_viewsize.value < 100)
	hudswap = cl_hudswap.value != 0 // Get that nasty float out :)

	if !headsup {
		Sbar_DrawPic(0, -24, sb_ibar, 320, 24)
	}

	// weapons
	for i = 0; i < 7; i++ {
		if cl.stats[STAT_ITEMS]&(IT_SHOTGUN<<i) != 0 {
			time = cl.item_gettime[i]
			flashon = int((cl.time - float64(time)) * 10)
			if flashon < 0 {
				flashon = 0
			}
			if flashon >= 10 {
				if cl_preselectedweapon == i+2 || (cl_preselectedweapon == 0 && cl.stats[STAT_ACTIVEWEAPON] == (IT_SHOTGUN<<i)) {
					flashon = 1
				} else {
					flashon = 0
				}
			} else {
				flashon = (flashon % 5) + 2
			}

			if headsup {
				if i != 0 || vid.conheight > 200 {
					x := int(vid.conwidth) - 32
					if hudswap {
						x = 0
					}
					if i == 6 {
						Sbar_DrawSubPicNormalised(x, -92-(7-i)*16, 24, 16, sb_weapons[flashon][i], 0, 0, 0.5, 1)
					} else {
						Sbar_DrawPic(x-sbar_xofs, -92-(7-i)*16, sb_weapons[flashon][i], 24, 16)
					}
				}
			} else {
				w := 24
				if i == 6 {
					w = 48
				}
				Sbar_DrawPic(i*24-sbar_xofs, -16, sb_weapons[flashon][i], w, 16)
			}

			if flashon > 1 {
				sb_updates = 0 // force update to remove flash
			}
		}
	}

	// ammo counts
	for i = 0; i < 4; i++ {
		b := scratchfb(6, "%3d", cl.stats[STAT_SHELLS+i])
		if b[0] != ' ' {
			b[0] = 18 + b[0] - '0'
		}
		if b[1] != ' ' {
			b[1] = 18 + b[1] - '0'
		}
		if b[2] != ' ' {
			b[2] = 18 + b[2] - '0'
		}
		num = unsafe.String(&b[0], len(b))

		if headsup {
			x := int(vid.conwidth) - 42
			if hudswap {
				x = 0
			}
			Sbar_DrawSubPicNormalised(x, -24-(4-i)*11, 42, 11, sb_ibar, float32(float64(3+(i*48))/320), 0, float32(float64(42.0)/320), float32(float64(11.0)/24))
			x = int(vid.conwidth) - 35
			if hudswap {
				x = 7
			}
			Draw_String(x, int(vid.conheight)-SBAR_HEIGHT-24-(4-i)*11, num)
		} else {
			Sbar_DrawString((6*i+1)*8-2+4, -24, num)
		}
	}

	flashon = 0
	// items
	for i = 0; i < 6; i++ {
		if cl.stats[STAT_ITEMS]&(1<<(17+i)) != 0 {
			time = cl.item_gettime[17+i]
			if time != 0 && float64(time) > cl.time-2 && flashon != 0 {
				// flash frame
				sb_updates = 0
			} else {
				Sbar_DrawPic(192+i*16, -16, sb_items[i], 16, 16)
			}

			if time != 0 && float64(time) > cl.time-2 {
				sb_updates = 0
			}
		}
	}

	// sigils
	for i = 0; i < 4; i++ {
		if cl.stats[STAT_ITEMS]&(1<<(28+i)) != 0 {
			time = cl.item_gettime[28+i]
			if time != 0 && float64(time) > cl.time-2 && flashon != 0 {
				// flash frame
				sb_updates = 0
			} else {
				Sbar_DrawPic(320-32+i*8, -16, sb_sigil[i], 8, 16)
			}

			if time != 0 && float64(time) > cl.time-2 {
				sb_updates = 0
			}
		}
	}
}

/************************************ HUD ************************************/

func Sbar_DrawFrags_DrawCell(x, y, topcolor, bottomcolor, frags int, brackets bool) {
	var num string

	// draw background
	Draw_Fill(sbar_xofs+x*8+10, y, 28, 4, Sbar_ColorForMap(topcolor))
	Draw_Fill(sbar_xofs+x*8+10, y+4, 28, 3, Sbar_ColorForMap(bottomcolor))

	// draw number
	if brackets {
		num = scratchf(6, "%c%3d%c", 16, frags, 17)

		Sbar_DrawString(x*8+4, -24, num)
		Sbar_DrawCharacter(x*8+0, -24, 16)
	} else {
		num = scratchf(6, "%3d", frags)

		Sbar_DrawString((x+1)*8+4, -24, num)
	}
}

func Sbar_DrawFrags() {
	var i, k, l, top, bottom, x, y, mynum int
	myteam := -1
	var s *player_info_t
	var tm *team_t
	drawn_self, drawn_self_team := false, false

	if scr_drawHFrags.value == 0 {
		return
	}

	x = 23
	y = int(vid.conheight) - SBAR_HEIGHT - 23

	mynum = Sbar_PlayerNum()

	if cl.teamplay == 0 || scr_drawHFrags.value == 1 {

		Sbar_SortFrags(false)
		l = min(scoreboardlines, 4)

		for i = 0; i < l; i++ {
			if i+1 == l && !drawn_self && !Sbar_IsSpectator(mynum) {
				k = mynum
			} else {
				k = fragsort[i]
			}

			s = &cl.players[k]

			if s.name == "" || s.spectator != 0 {
				continue
			}

			top, bottom = sbarPlayerColors(s)
			top = bound(0, top, 13)
			bottom = bound(0, bottom, 13)

			Sbar_DrawFrags_DrawCell(x, y, top, bottom, s.frags, k == mynum)

			if k == mynum {
				drawn_self = true
			}

			x += 4
		}
	} else {
		Sbar_SortTeams()

		for i = 0; i < scoreboardteams; i++ {
			if teams[i].myteam {
				myteam = i
				break
			}
		}

		l = min(scoreboardteams, 4)
		for i = 0; i < l; i++ {
			if myteam != -1 && i+1 == l && !drawn_self_team {
				k = myteam
			} else {
				k = teamsort[i]
			}

			tm = &teams[k]
			Sbar_DrawFrags_DrawCell(x, y, tm.topcolor, tm.bottomcolor, tm.frags, tm.myteam)

			if k == myteam {
				drawn_self_team = true
			}

			x += 4
		}

		if scr_drawHFrags.value != 3 && l <= 2 && !Sbar_IsSpectator(mynum) {

			x += 4 * (3 - l)

			s = &cl.players[mynum]

			top, bottom = sbarPlayerColors(s)
			top = bound(0, top, 13)
			bottom = bound(0, bottom, 13)

			Sbar_DrawFrags_DrawCell(x, y, top, bottom, s.frags, true)
		}
	}
}

func Sbar_DrawFace() {
	var f, anim int

	if (cl.stats[STAT_ITEMS] & (IT_INVISIBILITY | IT_INVULNERABILITY)) == (IT_INVISIBILITY | IT_INVULNERABILITY) {
		Sbar_DrawPic(112, 0, sb_face_invis_invuln, 24, 24)
		return
	}
	if cl.stats[STAT_ITEMS]&IT_QUAD != 0 {
		Sbar_DrawPic(112, 0, sb_face_quad, 24, 24)
		return
	}
	if cl.stats[STAT_ITEMS]&IT_INVISIBILITY != 0 {
		Sbar_DrawPic(112, 0, sb_face_invis, 24, 24)
		return
	}
	if cl.stats[STAT_ITEMS]&IT_INVULNERABILITY != 0 {
		Sbar_DrawPic(112, 0, sb_face_invuln, 24, 24)
		return
	}

	f = cl.stats[STAT_HEALTH] / 20
	f = bound(0, f, 4)

	if cl.time <= float64(cl.faceanimtime) {
		anim = 1
		sb_updates = 0 // make sure the anim gets drawn over
	} else {
		anim = 0
	}
	Sbar_DrawPic(112, 0, sb_faces[f][anim], 24, 24)
}

func Sbar_DrawNormal() {
	if cl_sbar.value != 0 || scr_viewsize.value < 100 {
		Sbar_DrawPic(0, 0, sb_sbar, 320, 24)
	}

	// armor
	if cl.stats[STAT_ITEMS]&IT_INVULNERABILITY != 0 {
		Sbar_DrawNum(24, 0, 666, 3, 1)
		Sbar_DrawPic(0, 0, sb_armor_invuln, 24, 24)
	} else {
		Sbar_DrawNum(24, 0, cl.stats[STAT_ARMOR], 3, b2i(cl.stats[STAT_ARMOR] <= 25))
		if cl.stats[STAT_ITEMS]&IT_ARMOR3 != 0 {
			Sbar_DrawPic(0, 0, sb_armor[2], 24, 24)
		} else if cl.stats[STAT_ITEMS]&IT_ARMOR2 != 0 {
			Sbar_DrawPic(0, 0, sb_armor[1], 24, 24)
		} else if cl.stats[STAT_ITEMS]&IT_ARMOR1 != 0 {
			Sbar_DrawPic(0, 0, sb_armor[0], 24, 24)
		}
	}

	// face
	Sbar_DrawFace()

	// health
	Sbar_DrawNum(136, 0, cl.stats[STAT_HEALTH], 3, b2i(cl.stats[STAT_HEALTH] <= 25))

	// ammo icon
	if cl.stats[STAT_ITEMS]&IT_SHELLS != 0 {
		Sbar_DrawPic(224, 0, sb_ammo[0], 24, 24)
	} else if cl.stats[STAT_ITEMS]&IT_NAILS != 0 {
		Sbar_DrawPic(224, 0, sb_ammo[1], 24, 24)
	} else if cl.stats[STAT_ITEMS]&IT_ROCKETS != 0 {
		Sbar_DrawPic(224, 0, sb_ammo[2], 24, 24)
	} else if cl.stats[STAT_ITEMS]&IT_CELLS != 0 {
		Sbar_DrawPic(224, 0, sb_ammo[3], 24, 24)
	}

	Sbar_DrawNum(248, 0, cl.stats[STAT_AMMO], 3, b2i(cl.stats[STAT_AMMO] <= 10))
}

var sbar_compact_weapons = [7]string{"sg", "bs", "ng", "sn", "gl", "rl", "lg"}

func Sbar_DrawCompact() {
	var i, align, old_sbar_xofs int
	var str string

	if cl_sbar.value != 0 || scr_viewsize.value < 100 {
		Sbar_DrawPic(0, 0, sb_sbar, 320, 24)
	}

	old_sbar_xofs = sbar_xofs
	if scr_centerSbar.value != 0 {
		sbar_xofs = int((vid.conwidth - 306) >> 1)
	} else {
		sbar_xofs = 0
	}

	if cl.stats[STAT_ITEMS]&IT_INVULNERABILITY != 0 {
		Sbar_DrawNum(2, 0, 666, 3, 1)
	} else {
		Sbar_DrawNum(2, 0, cl.stats[STAT_ARMOR], 3, b2i(cl.stats[STAT_ARMOR] <= 25))
	}

	Sbar_DrawNum(86, 0, cl.stats[STAT_HEALTH], 3, b2i(cl.stats[STAT_HEALTH] <= 25))

	Draw_BeginTextRendering()

	align = b2i(scr_compactHudAlign.value != 0)
	for i = 0; i < 4; i++ {
		str = scratchf(4, "%d", cl.stats[STAT_SHELLS+i])
		if cl.stats[STAT_SHELLS+i] < 5 {
			Sbar_DrawAltString(align*8*(3-len(str))+174+32*i, 3, str)
		} else {
			Sbar_DrawString(align*8*(3-len(str))+174+32*i, 3, str)
		}
	}
	for i = 0; i < 7; i++ {
		if cl.stats[STAT_ITEMS]&(IT_SHOTGUN<<i) != 0 {
			if cl_preselectedweapon == i+2 {
				Sbar_DrawAltString(166+5*4*i, 14, sbar_compact_weapons[i])
			} else if cl_preselectedweapon == 0 && cl.stats[STAT_ACTIVEWEAPON] == (IT_SHOTGUN<<i) {
				Sbar_DrawAltString(166+5*4*i, 14, sbar_compact_weapons[i])
			} else {
				Sbar_DrawString(166+5*4*i, 14, sbar_compact_weapons[i])
			}
		}
	}
	sbar_xofs = old_sbar_xofs

	Draw_EndTextRendering()
}

func Sbar_DrawCompact_TF() {
	var i, align, old_sbar_xofs int
	var str string

	if cl_sbar.value != 0 || scr_viewsize.value < 100 {
		Sbar_DrawPic(0, 0, sb_sbar, 320, 24)
	}

	old_sbar_xofs = sbar_xofs
	if scr_centerSbar.value != 0 {
		sbar_xofs = int((vid.conwidth - 222) >> 1)
	} else {
		sbar_xofs = 0
	}

	align = b2i(scr_compactHudAlign.value != 0)
	if cl.stats[STAT_ITEMS]&IT_INVULNERABILITY != 0 {
		Sbar_DrawNum(2, 0, 666, 3, 1)
	} else {
		Sbar_DrawNum(2, 0, cl.stats[STAT_ARMOR], 3, b2i(cl.stats[STAT_ARMOR] <= 25))
	}
	Sbar_DrawNum(86, 0, cl.stats[STAT_HEALTH], 3, b2i(cl.stats[STAT_HEALTH] <= 25))
	for i = 0; i < 4; i++ {
		str = scratchf(4, "%d", cl.stats[STAT_SHELLS+i])
		y := 3
		if i >= 2 {
			y = 14
		}
		if cl.stats[STAT_SHELLS+i] < 5 {
			Sbar_DrawAltString(align*8*(3-len(str))+166+32*(i%2), y, str)
		} else {
			Sbar_DrawString(align*8*(3-len(str))+166+32*(i%2), y, str)
		}
	}
	sbar_xofs = old_sbar_xofs
}

func Sbar_DrawCompact_Bare() {
	var old_sbar_xofs int

	if cl_sbar.value != 0 || scr_viewsize.value < 100 {
		Sbar_DrawPic(0, 0, sb_sbar, 320, 24)
	}

	old_sbar_xofs = sbar_xofs
	if scr_centerSbar.value != 0 {
		sbar_xofs = int((vid.conwidth - 158) >> 1)
	} else {
		sbar_xofs = 0
	}

	if cl.stats[STAT_ITEMS]&IT_INVULNERABILITY != 0 {
		Sbar_DrawNum(2, 0, 666, 3, 1)
	} else {
		Sbar_DrawNum(2, 0, cl.stats[STAT_ARMOR], 3, b2i(cl.stats[STAT_ARMOR] <= 25))
	}
	Sbar_DrawNum(86, 0, cl.stats[STAT_HEALTH], 3, b2i(cl.stats[STAT_HEALTH] <= 25))
	sbar_xofs = old_sbar_xofs
}

/******************************** SCOREBOARD ********************************/

func Sbar_SoloScoreboard() {
	var str string
	var time float64
	var minutes, seconds, tens, units int

	sb_updates = 0 // because time display changes every second

	Sbar_DrawPic(0, 0, sb_scorebar, 320, 24)

	if cl.gametype == GAME_COOP {
		str = fmt.Sprintf("Monsters:%3d /%3d", cl.stats[STAT_MONSTERS], cl.stats[STAT_TOTALMONSTERS])
		Sbar_DrawString(8, 4, str)

		str = fmt.Sprintf("Secrets :%3d /%3d", cl.stats[STAT_SECRETS], cl.stats[STAT_TOTALSECRETS])
		Sbar_DrawString(8, 12, str)
	}

	// time
	if cl.servertime_works {
		time = cl.servertime // good, we know real time spent on level
	} else {
		if cls.demoplayback {
			time = cls.demotime
		} else {
			time = cls.realtime
		}
		if cl.gametype == GAME_COOP {
			time -= float64(cl.players[cl.playernum].entertime)
		}
	}

	minutes = int(time / 60)
	seconds = int(time - float64(60*minutes))
	tens = seconds / 10
	units = seconds - 10*tens
	str = fmt.Sprintf("Time :%3d:%d%d", minutes, tens, units)
	Sbar_DrawString(184, 4, str)

	if cl.gametype == GAME_COOP {
		// draw level name
		l := len(cl.levelname)
		if l < 22 && !strings.Contains(cl.levelname, "\n") {
			Sbar_DrawString(232-l*4, 12, cl.levelname)
		}
	}
}

func SCOREBOARD_LASTROW() int { return int(vid.conheight) - 34 }

const (
	RANK_WIDTH_BASICSTATS = 11 * 8
	RANK_WIDTH_TEAMSTATS  = 4 * 8
	RANK_WIDTH_TCHSTATS   = 5 * 8
	RANK_WIDTH_CAPSTATS   = 5 * 8

	RANK_WIDTH_DM   = -8 + 168 + (MAX_SCOREBOARDNAME * 8)
	RANK_WIDTH_TEAM = -8 + 208 + (MAX_SCOREBOARDNAME * 8)
)

func SCOREBOARD_ALPHA() float64        { return 0.5 * float64(bound(0, scr_scoreboard_fillalpha.value, 1)) }
func SCOREBOARD_HEADINGALPHA() float32 { return bound(0, scr_scoreboard_fillalpha.value, 1) }

// the scoreboard name buffer is MAX_SCOREBOARDNAME, shortened further by leftover pixels
func sbarScoreboardName(s *player_info_t, leftover int) string {
	name := strlcpy(s.name, MAX_SCOREBOARDNAME)
	if leftover > 0 {
		truncate := (leftover / 8) + b2i(leftover%8 != 0)

		truncate = bound(0, truncate, MAX_SCOREBOARDNAME-1)
		if len(name) > MAX_SCOREBOARDNAME-1-truncate {
			name = name[:MAX_SCOREBOARDNAME-1-truncate]
		}
	}
	return name
}

func Sbar_DeathmatchOverlay() {
	var stats_team, stats_touches, stats_caps int
	var playerstats [7]int
	var scoreboardsize, colors_thickness, statswidth, stats_xoffset int
	var i, k, top, bottom, x, y, xofs, total, minutes, p int
	skip := 10
	var rank_width, leftover, startx, tempx, mynum int
	var num, scorerow, team, name string
	var s *player_info_t
	var start int
	var topy int
	var midy int

	if cl.teamplay != 0 && (sb_showteamscores || cl.intermission != 0) {
		start = Sbar_TeamOverlay()
	} else {
		start = 0
	}

	if cls.state == ca_active && !cls.demoplayback {
		// request new ping times every two seconds
		if cls.realtime-cl.last_ping_request >= 2 {
			cl.last_ping_request = cls.realtime

			NetQW_AppendReliableBuffer(cls.netqw, append([]byte{clc_stringcmd}, "pings\x00"...))
		}
	}

	mynum = Sbar_PlayerNum()

	if cl.teamplay != 0 {
		rank_width = RANK_WIDTH_TEAM
	} else {
		rank_width = RANK_WIDTH_DM
	}

	statswidth = 0
	stats_team, stats_touches, stats_caps = 0, 0, 0
	if scr_scoreboard_showfrags.value != 0 && Stats_IsActive() {
		if rank_width+statswidth+RANK_WIDTH_BASICSTATS < int(vid.conwidth)-16 {
			statswidth += RANK_WIDTH_BASICSTATS
			if cl.teamplay != 0 {
				if rank_width+statswidth+RANK_WIDTH_TEAMSTATS < int(vid.conwidth)-16 {
					if (cl.teamfortress || cl.teamplay != 3) &&
						(!cl.teamfortress || (cl.teamplay&21) != 21) {
						statswidth += RANK_WIDTH_TEAMSTATS
						stats_team++
					}
				}
				if Stats_IsFlagsParsed() {
					if rank_width+statswidth+RANK_WIDTH_TCHSTATS < int(vid.conwidth)-16 {
						statswidth += RANK_WIDTH_TCHSTATS
						stats_touches++
					}
					if rank_width+statswidth+RANK_WIDTH_CAPSTATS < int(vid.conwidth)-16 {
						statswidth += RANK_WIDTH_CAPSTATS
						stats_caps++
					}
				}
			}
		}
	}

	rank_width += statswidth

	leftover = rank_width
	rank_width = bound(0, rank_width, int(vid.conwidth)-16)
	leftover = max(0, leftover-rank_width)
	xofs = (int(vid.conwidth) - rank_width) >> 1

	if start == 0 {
		if scr_scoreboard_drawtitle.value != 0 {
			Draw_DrawPicture(rankingpic, xofs+(rank_width-168)/2, 0, 168, 24)
			start = 36
		} else {
			start = 12
		}
	}

	y = start

	topy = y - 9

	headingheight := 9
	if scr_scoreboard_titleseperator.value != 0 {
		headingheight += 8
	}
	Draw_AlphaFill(xofs, y-8, rank_width, headingheight, 1, SCOREBOARD_HEADINGALPHA()) // Draw heading row

	if cl.teamplay != 0 {
		Draw_String(xofs+1, y-8, " ping pl time frags team name")
	} else {
		Draw_String(xofs+1, y-8, " ping pl time frags name")
	}

	if statswidth != 0 {
		temp := ""

		if stats_team != 0 {
			temp += "kills tks dths"
		} else {
			temp += "kills dths"
		}
		if stats_touches != 0 {
			temp += " tchs"
		}
		if stats_caps != 0 {
			temp += " caps"
		}

		if cl.teamplay != 0 {
			stats_xoffset = 41 * 8
		} else {
			stats_xoffset = 36 * 8
		}
		Draw_String(xofs+1+stats_xoffset, y-8, temp)
	}

	if scr_scoreboard_titleseperator.value != 0 {
		if cl.teamplay != 0 {
			Draw_String(xofs+1, y, " \x1d\x1e\x1e\x1f \x1d\x1f \x1d\x1e\x1e\x1f \x1d\x1e\x1e\x1e\x1f \x1d\x1e\x1e\x1f \x1d\x1e\x1e\x1e\x1e\x1e\x1e\x1e\x1e\x1e\x1e\x1e\x1e\x1e\x1f")
		} else {
			Draw_String(xofs+1, y, " \x1d\x1e\x1e\x1f \x1d\x1f \x1d\x1e\x1e\x1f \x1d\x1e\x1e\x1e\x1f \x1d\x1e\x1e\x1e\x1e\x1e\x1e\x1e\x1e\x1e\x1e\x1e\x1e\x1e\x1f")
		}

		y += 8
	}

	midy = y + 1

	y += 2 // dont go over the black border, move the rest down

	startx = xofs + 8
	x = startx

	tempx = xofs + rank_width - MAX_SCOREBOARDNAME*8 - (48 + statswidth) + leftover

	Sbar_SortTeamsAndFrags(true) // scores

	scoreboardsize = 0
	for i = 0; i < scoreboardlines; i++ {
		k = fragsort[i]
		s = &cl.players[k]

		if s.name != "" {
			scoreboardsize++
		}
	}

	if y+(scoreboardsize-1)*skip > SCOREBOARD_LASTROW() {
		colors_thickness = 3
		skip = 8
	} else {
		colors_thickness = 4
	}

	for i = 0; i < scoreboardlines && y <= SCOREBOARD_LASTROW(); i++ {
		k = fragsort[i]
		s = &cl.players[k]

		if s.name == "" {
			continue
		}

		// render the main background transparencies behind players row
		top, bottom = sbarPlayerColors(s)

		if cl.teamplay == 0 ||
			s.spectator != 0 ||
			scr_scoreboard_fillcolored.value == 0 ||
			(scr_scoreboard_fillcolored.value == 2 && scr_scoreboard_teamsort.value == 0) {
			Draw_AlphaFill(xofs, y, rank_width, skip, 2, float32(SCOREBOARD_ALPHA()))
		} else {
			if k == mynum {
				var alpha float32
				alpha = float32(1.7 * SCOREBOARD_ALPHA())
				alpha = float32(math.Min(float64(alpha), 0.75))
				Draw_AlphaFill(xofs, y, rank_width, skip, Sbar_ColorForMap(bottom), alpha)
			} else {
				Draw_AlphaFill(xofs, y, rank_width, skip, Sbar_ColorForMap(bottom), float32(SCOREBOARD_ALPHA()))
			}
		}

		// draw ping
		p = s.ping
		if p < 0 || p > 999 {
			p = 999
		}

		num = scratchf(12, "&cAAD%4d", p)
		Draw_ColoredString(x, y, num, 0)
		x += 32 // move it forward, ready to print next column

		// draw pl
		p = int(s.pl)

		if p > 25 {
			num = scratchf(12, "&cD33%3d", p)
			Draw_ColoredString(x, y, num, 1)
		} else {
			num = scratchf(12, "&cDB0%3d", p)
			Draw_ColoredString(x, y, num, 0)
		}

		x += 32

		if s.spectator != 0 {
			Draw_ColoredString(x, y, "&cF20s&cF50p&cF80e&c883c&cA85t&c668a&c55At&c33Bo&c22Dr", 0)

			if cl.teamplay != 0 { // move across to print the name
				x += 128
			} else {
				x += 88
			}

			name = sbarScoreboardName(s, leftover)

			Draw_String(x, y, name) // draw name

			y += skip
			x = startx
		} else {
			// draw time
			var t float64
			if cl.intermission != 0 {
				t = float64(cl.completed_time)
			} else if cls.demoplayback {
				t = cls.demotime
			} else {
				t = cls.realtime
			}
			total = int(t - float64(s.entertime))
			minutes = total / 60

			// print the shirt/pants colour bars
			cx := tempx
			if cl.teamplay != 0 {
				cx = tempx - 40
			}
			Draw_Fill(cx, y+4-colors_thickness, 40, colors_thickness, Sbar_ColorForMap(top))
			Draw_Fill(cx, y+4, 40, 4, Sbar_ColorForMap(bottom))

			// name
			name = sbarScoreboardName(s, leftover)

			bl, br := byte(' '), byte(' ')
			if k == mynum {
				bl, br = 16, 17
			}

			// team
			if cl.teamplay != 0 {
				team = strlcpy(s.team, 5)
				scorerow = scratchf(64, " %3d %c%3d%c %-4s %s", minutes, bl, s.frags, br, team, name)
			} else {
				scorerow = scratchf(64, " %3d %c%3d%c %s", minutes, bl, s.frags, br, name)
			}

			Draw_String(x, y, scorerow)

			if statswidth != 0 {
				Stats_GetBasicStats(k, playerstats[:])
				if stats_touches != 0 || stats_caps != 0 {
					Stats_GetFlagStats(k, playerstats[4:])
				}

				scorerow = ""

				if stats_team != 0 {
					scorerow = scratchf(64, " &c0B4%3d  &cF60%3d &cF00%3d ", playerstats[0], playerstats[2], playerstats[1])
				} else {
					scorerow = scratchf(64, " &c0B4%3d  &cF00%3d ", playerstats[0], playerstats[1])
				}

				if stats_touches != 0 {
					scorerow += va("  &cFD0%2d ", playerstats[4])
				}
				if stats_caps != 0 {
					scorerow += va("  &c24F%2d ", playerstats[6])
				}

				Draw_ColoredString(x+stats_xoffset-9*8, y, scorerow, 0)
			}

			y += skip
			x = startx
		}
	}

	if scr_scoreboard_borderless.value == 0 {
		Draw_Fill(xofs, topy, rank_width, 1, 0) // Border - Top
		Draw_Fill(xofs, midy, rank_width, 1, 0) // Border - Top (under header)
		Draw_Fill(xofs, y-1, rank_width, 1, 0)  // Border - Bottom

		Draw_Fill(xofs-1, topy, 1, y-topy, 0)              // Border - Left
		Draw_Fill(xofs-1+rank_width+1, topy, 1, y-topy, 0) // Border - Right
	}
}

func Sbar_TeamOverlay() int {
	var i, k, x, y, xofs, plow, phigh, pavg, rank_width int
	skip := 10
	var num, team string
	var tm *team_t
	var topy int

	if cl.teamplay != 0 {
		rank_width = RANK_WIDTH_TEAM
	} else {
		rank_width = RANK_WIDTH_DM
	}
	rank_width = bound(0, rank_width, int(vid.conwidth)-16)

	xofs = (int(vid.conwidth) - rank_width) >> 1

	if scr_scoreboard_drawtitle.value != 0 {
		Draw_DrawPicture(rankingpic, xofs+(rank_width-168)/2, 0, 168, 24)
		y = 26
	} else {
		y = 2
	}

	topy = y + 1

	Draw_AlphaFill(xofs, y+2, rank_width, skip, 1, SCOREBOARD_HEADINGALPHA()) // draw heading row
	y += 2                                                                    // dont go over the black border, move the rest down

	x = int(float64(xofs) + float64(rank_width)*0.1 + 8)

	Draw_String(x, y, "low/avg/high team total players")

	y += 10

	Sbar_SortTeams() // sort the teams

	for i = 0; i < scoreboardteams && y <= SCOREBOARD_LASTROW(); i++ {
		k = teamsort[i]
		tm = &teams[k]

		Draw_AlphaFill(xofs, y, rank_width, skip, 2, float32(SCOREBOARD_ALPHA()))

		// draw pings
		plow = tm.plow
		if plow < 0 || plow > 999 {
			plow = 999
		}

		phigh = tm.phigh
		if phigh < 0 || phigh > 999 {
			phigh = 999
		}

		if tm.players != 0 {
			pavg = tm.ptotal / tm.players
		} else {
			pavg = 999
		}
		if pavg < 0 || pavg > 999 {
			pavg = 999
		}

		num = scratchf(12, "%3d/%3d/%3d", plow, pavg, phigh)
		Draw_String(x, y, num)

		// draw team
		team = strlcpy(tm.team, 5)
		Draw_String(x+104, y, team)

		// draw total
		num = scratchf(12, "%5d", tm.frags)
		Draw_String(x+104+40, y, num)

		// draw players
		num = scratchf(12, "%5d", tm.players)
		Draw_String(x+104+88, y, num)

		if tm.myteam {
			Draw_Character(x+104-8, y, 16)
			Draw_Character(x+104+32, y, 17)
		}

		y += skip
	}

	if scr_scoreboard_borderless.value == 0 {
		Draw_Fill(xofs, topy, rank_width, 1, 0)    // Border - Top
		Draw_Fill(xofs, topy+10, rank_width, 1, 0) // Border - Top (under header)
		Draw_Fill(xofs, y-1, rank_width, 1, 0)     // Border - Bottom

		Draw_Fill(xofs-1, topy, 1, y-topy, 0)              // Border - Left
		Draw_Fill(xofs-1+rank_width+1, topy, 1, y-topy, 0) // Border - Right
	}

	y += 14

	return y
}

func Sbar_MiniDeathmatchOverlay() {
	var i, k, top, bottom, x, y, mynum, numlines int
	var num, name, team string
	var s *player_info_t
	var tm *team_t

	if scr_drawVFrags.value == 0 {
		return
	}

	if sb_lines == 0 {
		return // not enuff room
	}

	mynum = Sbar_PlayerNum()

	x = 324

	// C jumps straight to the team list here
	if !(vid.conwidth < 640 && cl.teamplay != 0 && scr_drawVFrags.value == 2) {
		// scores
		Sbar_SortFrags(false)

		if scoreboardlines == 0 {
			return // no one there?
		}

		// draw the text
		y = int(vid.conheight) - sb_lines - 1
		numlines = sb_lines / 8
		if numlines < 3 {
			return // not enough room
		}

		// find us
		for i = 0; i < scoreboardlines; i++ {
			if fragsort[i] == mynum {
				break
			}
		}

		if i == scoreboardlines { // we're not there, we are probably a spectator, just display top
			i = 0
		} else { // figure out start
			i = i - numlines/2
		}

		i = bound(0, i, scoreboardlines-numlines)

		for ; i < scoreboardlines && y < int(vid.conheight)-8+1; i++ {
			k = fragsort[i]
			s = &cl.players[k]

			if s.name == "" {
				continue
			}

			top, bottom = sbarPlayerColors(s)

			Draw_Fill(x, y+1, 40, 3, Sbar_ColorForMap(top))
			Draw_Fill(x, y+4, 40, 4, Sbar_ColorForMap(bottom))

			// draw number
			bl, br := byte(' '), byte(' ')
			if k == mynum {
				bl, br = 16, 17
			}
			num = scratchf(6+1, "%c%3d%c", bl, s.frags, br)

			Draw_String(x, y, num)

			// team
			if cl.teamplay != 0 {
				team = strlcpy(s.team, 4+1)
				Draw_String(x+48, y, team)
			}

			// draw name
			name = strlcpy(s.name, 16+1)
			if cl.teamplay != 0 {
				Draw_String(x+48+40, y, name)
			} else {
				Draw_String(x+48, y, name)
			}

			y += 8
		}

		// draw teams if room
		if vid.conwidth < 640 || cl.teamplay == 0 {
			return
		}

		Sbar_SortTeams()
		if scoreboardteams == 0 {
			return
		}

		// draw seperator
		x += 208
		for y = int(vid.conheight) - sb_lines; y < int(vid.conheight)-6; y += 2 {
			Draw_Character(x, y, 14)
		}

		x += 16
	}

	// drawteams:
	Sbar_SortTeams()

	y = int(vid.conheight) - sb_lines
	for i = 0; i < scoreboardteams && y < int(vid.conheight)-8+1; i++ {
		k = teamsort[i]
		tm = &teams[k]

		// draw name
		team = strlcpy(tm.team, 4+1)
		Draw_String(x, y, team)

		// draw total
		Draw_Fill(x+40, y+1, 48, 3, Sbar_ColorForMap(tm.topcolor))
		Draw_Fill(x+40, y+4, 48, 4, Sbar_ColorForMap(tm.bottomcolor))
		num = scratchf(6+1, "%4d", tm.frags)
		Draw_Character(x+40+8, y, num[0])
		Draw_Character(x+40+16, y, num[1])
		Draw_Character(x+40+24, y, num[2])
		Draw_Character(x+40+32, y, num[3])

		if tm.myteam {
			Draw_Character(x+40, y, 16)
			Draw_Character(x+40+40, y, 17)
		}

		y += 8
	}
}

func Sbar_IntermissionNumber(x, y, num, digits, color int, rightadjustify bool) {
	var str, ptr string
	var l, frame int

	str = Sbar_itoa(num)
	l = len(str)
	ptr = str
	if l > digits {
		ptr = ptr[l-digits:]
	}
	if rightadjustify && l < digits {
		x += (digits - l) * 24
	}

	for i := 0; i < len(ptr); i++ {
		if ptr[i] == '-' {
			frame = STAT_MINUS
		} else {
			frame = int(ptr[i] - '0')
		}
		Draw_DrawPicture(sb_nums[color][frame], x, y, 24, 24)
		x += 24
	}
}

func Sbar_IntermissionOverlay() {
	var time float32
	var dig, num, xofs int

	if cl.gametype == GAME_DEATHMATCH {
		Sbar_DeathmatchOverlay()

		return
	}

	xofs = int((vid.conwidth - 320) >> 1)

	if completepic == nil {
		completepic = Draw_LoadPicture("gfx/complete.lmp", DRAW_LOADPICTURE_DUMMYFALLBACK)
	}

	Draw_DrawPicture(completepic, xofs+64, 24, 192, 24)

	if interpic == nil {
		interpic = Draw_LoadPicture("gfx/inter.lmp", DRAW_LOADPICTURE_DUMMYFALLBACK)
	}

	Draw_DrawPicture(interpic, xofs, 56, 160, 144)

	if cl.servertime_works {
		time = float32(cl.solo_completed_time) // we know the exact time
	} else {
		time = float32(cl.completed_time) - cl.players[cl.playernum].entertime // use an estimate
	}

	// time
	dig = int(time / 60)
	dig = max(dig, 0)
	Sbar_IntermissionNumber(xofs+152, 64, dig, 3, 0, true)
	num = int(time - float32(dig*60))
	Draw_DrawPicture(sb_colon, xofs+224, 64, 24, 24)
	Draw_DrawPicture(sb_nums[0][num/10], xofs+248, 64, 24, 24)
	Draw_DrawPicture(sb_nums[0][num%10], xofs+272, 64, 24, 24)

	// secrets
	Sbar_IntermissionNumber(xofs+152, 104, cl.stats[STAT_SECRETS], 3, 0, true)
	Draw_DrawPicture(sb_slash, xofs+224, 104, 24, 24)
	Sbar_IntermissionNumber(xofs+248, 104, cl.stats[STAT_TOTALSECRETS], 3, 0, false)

	// monsters
	Sbar_IntermissionNumber(xofs+152, 144, cl.stats[STAT_MONSTERS], 3, 0, true)
	Draw_DrawPicture(sb_slash, xofs+224, 144, 24, 24)
	Sbar_IntermissionNumber(xofs+248, 144, cl.stats[STAT_TOTALMONSTERS], 3, 0, false)
}

func Sbar_FinaleOverlay() {
	if finalepic == nil {
		finalepic = Draw_LoadPicture("gfx/finale.lmp", DRAW_LOADPICTURE_DUMMYFALLBACK)
	}

	Draw_DrawPicture(finalepic, int((vid.conwidth-288)/2), 16, 288, 24)
}

/********************************* INTERFACE *********************************/

// the main area, drawn the same way for players and for tracked spectators
func sbarDrawMainArea() {
	if sb_showscores || sb_showteamscores || cl.stats[STAT_HEALTH] <= 0 {
		Sbar_SoloScoreboard()
	} else if scr_compactHud.value == 3 {
		Sbar_DrawCompact_Bare()
	} else if scr_compactHud.value == 2 {
		Sbar_DrawCompact_TF()
	} else if scr_compactHud.value == 1 {
		Sbar_DrawCompact()
	} else {
		Sbar_DrawNormal()
	}
}

func Sbar_Draw() {
	var headsup bool
	var st string

	headsup = !(cl_sbar.value != 0 || scr_viewsize.value < 100)
	if sb_updates >= vid.numpages && !headsup {
		return
	}

	sb_updates++

	if scr_centerSbar.value != 0 {
		sbar_xofs = int((vid.conwidth - 320) >> 1)
	} else {
		sbar_xofs = 0
	}

	// top line
	if sb_lines > 24 {
		if cl.spectator == 0 || autocam == CAM_TRACK {
			Sbar_DrawInventory()
		}

		if cl.gametype == GAME_DEATHMATCH && (!headsup || vid.conwidth < 512) {
			Sbar_DrawFrags()
		}
	}

	// main area
	if sb_lines > 0 {
		if cl.spectator != 0 {
			if autocam != CAM_TRACK {
				Sbar_DrawPic(0, 0, sb_scorebar, 320, 24)
				Sbar_DrawString(160-7*8, 4, "SPECTATOR MODE")
				Sbar_DrawString(160-14*8+4, 12, "Press [ATTACK] for AutoCamera")
			} else {
				sbarDrawMainArea()

				st = fmt.Sprintf("Tracking %-.13s", cl.players[spec_track].name)
				if !cls.demoplayback {
					st = strlcat(st, ", [JUMP] for next", 64)
				}

				Sbar_DrawString(0, -8, st)
			}
		} else {
			sbarDrawMainArea()
		}
	}

	// main screen deathmatch rankings
	// if we're dead show team scores in team games
	if (cl.stats[STAT_HEALTH] <= 0 && cl.spectator == 0) || sb_showscores || sb_showteamscores {
		Sbar_DeathmatchOverlay()
	}

	if sb_showscores || sb_showteamscores || cl.stats[STAT_HEALTH] <= 0 {
		sb_updates = 0
	}

	// clear unused areas in GL
	if vid.conwidth > 320 && !headsup {
		if scr_centerSbar.value != 0 { // left
			Draw_TileClear(0, int(vid.conheight)-sb_lines, sbar_xofs, sb_lines)
		}

		Draw_TileClear(320+sbar_xofs, int(vid.conheight)-sb_lines, int(vid.conwidth)-(320+sbar_xofs), sb_lines) // right
	}
	if !headsup && cl.spectator != 0 && autocam != CAM_TRACK && sb_lines > SBAR_HEIGHT {
		Draw_TileClear(sbar_xofs, int(vid.conheight)-sb_lines, 320, sb_lines-SBAR_HEIGHT)
	}

	if vid.conwidth >= 512 && sb_lines > 0 && cl.gametype == GAME_DEATHMATCH && scr_centerSbar.value == 0 {
		Sbar_MiniDeathmatchOverlay()
	}
}

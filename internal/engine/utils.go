package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unsafe"
)

// utils.c -- string and filename helpers

// utils.h
const (
	PLAYER_ID_NOMATCH   = -1
	PLAYER_NAME_NOMATCH = -2
	PLAYER_NUM_NOMATCH  = -3
)

// Finds the first free _NNN suffix for filename with any of the extensions,
// -1 when none is free. C only knows '/' as the start of an absolute path,
// so on Windows it prepends com_basedir to an already absolute path and
// never finds the earlier saves.
func Util_Extend_Filename(filename string, ext []string) int {
	maxextlen := 0
	for _, e := range ext {
		if len(e) > maxextlen {
			maxextlen = len(e)
		}
	}

	var base string
	if at(filename, 0) == '/' || filepath.IsAbs(filename) {
		base = filename
	} else {
		base = com_basedir + "/" + filename
	}

	if len(base)+5+maxextlen >= 1024 {
		return -1
	}

	for i := 0; i < 1000; i++ {
		taken := false
		for _, e := range ext {
			f, err := os.Open(fmt.Sprintf("%s_%03d.%s", base, i, e))
			if err == nil {
				f.Close()
				taken = true
				break
			}
		}
		if !taken {
			return i
		}
	}
	return -1
}

func Util_Is_Valid_Filename(s string) bool {
	const badchars = "?*:<>\""

	if s == "" {
		return false
	}

	if strings.Contains(s, "../") || strings.Contains(s, "..\\") {
		return false
	}

	for i := 0; i < len(s); i++ {
		if s[i] < 32 || s[i] >= 127 || strings.IndexByte(badchars, s[i]) >= 0 {
			return false
		}
	}
	return true
}

func Util_Invalid_Filename_Msg(s string) string {
	return strlcpy(fmt.Sprintf("%s is not a valid filename (?*:<>\" are illegal characters)\n", s), 192)
}

// Backslashes become slashes and a leading slash is dropped; C does it in
// place, Go returns the result.
func Util_Process_Filename(s string) string {
	s = strings.ReplaceAll(s, "\\", "/")
	if at(s, 0) == '/' {
		s = s[1:]
	}
	return s
}

func ishex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func HEXTOINT(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return 0
}

func is_valid_color_info(s string, i int) bool {
	return ishex(at(s, i)) && ishex(at(s, i+1)) && ishex(at(s, i+2))
}

// Visible length of a string with &cRGB and &r colour codes.
func Colored_String_Length(s string) int {
	count := 0
	i := 0
	for at(s, i) != 0 {
		if s[i] == '&' && at(s, i+1) == 'c' && is_valid_color_info(s, i+2) {
			i += 5
		} else if s[i] == '&' && at(s, i+1) == 'r' {
			i += 2
		} else {
			i++
			count++
		}
	}
	return count
}

// Byte offset after maxlen visible characters; lastcolour, when not nil,
// receives the colour in effect there.
func Colored_String_Offset(s string, maxlen uint, lastcolour *uint16) int {
	count := uint(0)
	i := 0
	var colour uint16 = 0x0fff

	for at(s, i) != 0 && count != maxlen {
		if s[i] == '&' && at(s, i+1) == 'c' && is_valid_color_info(s, i+2) {
			if lastcolour != nil {
				colour = uint16(HEXTOINT(s[i+2])<<8 | HEXTOINT(s[i+3])<<4 | HEXTOINT(s[i+4]))
			}
			i += 5
		} else if s[i] == '&' && at(s, i+1) == 'r' {
			i += 2
		} else {
			i++
			count++
		}
	}

	if lastcolour != nil {
		*lastcolour = colour
	}

	return i
}

// Parses "rgb:r,g,b" (case insensitive, up to the first space) into rgb.
func ParseColourDescription(source string, rgb *[3]float32) bool {
	var desc []byte

	for i := 0; i < len(source) && source[i] != ' ' && i+1 < 64; i++ {
		if source[i] >= 'A' && source[i] <= 'Z' {
			desc = append(desc, 'a'+(source[i]-'A'))
		} else {
			desc = append(desc, source[i])
		}
	}

	if !(len(desc) >= 4 && desc[0] == 'r' && desc[1] == 'g' && desc[2] == 'b' && desc[3] == ':') {
		return false
	}
	rest := string(desc[4:])

	// strtod must consume each field whole
	parse := func(s string) (float32, bool) {
		f, err := strconv.ParseFloat(s, 64)
		return float32(f), err == nil
	}

	p := strings.IndexByte(rest, ',')
	if p <= 0 {
		return false
	}
	v, ok := parse(rest[:p])
	rgb[0] = v
	if !ok {
		return false
	}
	rest = rest[p+1:]

	p = strings.IndexByte(rest, ',')
	if p <= 0 {
		return false
	}
	v, ok = parse(rest[:p])
	rgb[1] = v
	if !ok {
		return false
	}
	rest = rest[p+1:]

	if rest == "" {
		return false
	}
	v, ok = parse(rest)
	rgb[2] = v
	return ok
}

// A colour as "r g b" or as a palette index, with full alpha.
func StringToRGB(s string) [4]byte {
	var rgb [4]byte

	Cmd_TokenizeString(s)
	if Cmd_Argc() == 3 {
		rgb[0] = byte(Q_atoi(Cmd_Argv(0)))
		rgb[1] = byte(Q_atoi(Cmd_Argv(1)))
		rgb[2] = byte(Q_atoi(Cmd_Argv(2)))
	} else {
		col := d_8to24table[byte(Q_atoi(s))]
		rgb[0] = byte(col)
		rgb[1] = byte(col >> 8)
		rgb[2] = byte(col >> 16)
	}
	rgb[3] = 255
	return rgb
}

// the HUD asks every frame, the result lives until the next call
var hour_str [8]byte

func SecondsToHourString(print_time int) string {
	tens_hours := (print_time / 36000) % 10
	hours := (print_time / 3600) % 10
	tens_minutes := (print_time / 600) % 6
	minutes := (print_time / 60) % 10
	tens_seconds := (print_time / 10) % 6
	seconds := print_time % 10
	b := strconv.AppendInt(hour_str[:0], int64(tens_hours), 10)
	b = strconv.AppendInt(b, int64(hours), 10)
	b = append(b, ':')
	b = strconv.AppendInt(b, int64(tens_minutes), 10)
	b = strconv.AppendInt(b, int64(minutes), 10)
	b = append(b, ':')
	b = strconv.AppendInt(b, int64(tens_seconds), 10)
	b = strconv.AppendInt(b, int64(seconds), 10)
	return unsafe.String(&b[0], len(b))
}

func Utils_TF_ColorToTeam_Failsafe(color int) string {
	var i, j int
	var teamcounts [MAX_CLIENTS]int // 8 in C, which overflows past eight teams
	numteamsseen := 0
	best := -1
	var teams [MAX_CLIENTS]string

	for i = 0; i < MAX_CLIENTS; i++ {
		if cl.players[i].name == "" || cl.players[i].spectator != 0 {
			continue
		}
		if int(cl.players[i].real_bottomcolor) != color {
			continue
		}
		for j = 0; j < numteamsseen; j++ {
			if cl.players[i].team == teams[j] {
				break
			}
		}
		if j == numteamsseen {
			teams[numteamsseen] = cl.players[i].team
			teamcounts[numteamsseen] = 1
			numteamsseen++
		} else {
			teamcounts[j]++
		}
	}
	for i = 0; i < numteamsseen; i++ {
		if best == -1 || teamcounts[i] > teamcounts[best] {
			best = i
		}
	}
	if best == -1 {
		return ""
	}
	return teams[best]
}

func Utils_TF_ColorToTeam(color int) string {
	var s string

	switch color {
	case 13:
		if s = Info_ValueForKey(cl.serverinfo, "team1"); s != "" {
			return s
		}
		if s = Info_ValueForKey(cl.serverinfo, "t1"); s != "" {
			return s
		}
	case 4:
		if s = Info_ValueForKey(cl.serverinfo, "team2"); s != "" {
			return s
		}
		if s = Info_ValueForKey(cl.serverinfo, "t2"); s != "" {
			return s
		}
	case 12:
		if s = Info_ValueForKey(cl.serverinfo, "team3"); s != "" {
			return s
		}
		if s = Info_ValueForKey(cl.serverinfo, "t3"); s != "" {
			return s
		}
	case 11:
		if s = Info_ValueForKey(cl.serverinfo, "team4"); s != "" {
			return s
		}
		if s = Info_ValueForKey(cl.serverinfo, "t4"); s != "" {
			return s
		}
	}

	return Utils_TF_ColorToTeam_Failsafe(color)
}

func Utils_TF_TeamToColor(team string) int {
	if Q_strcasecmp(team, Utils_TF_ColorToTeam(13)) == 0 {
		return 13
	}
	if Q_strcasecmp(team, Utils_TF_ColorToTeam(4)) == 0 {
		return 4
	}
	if Q_strcasecmp(team, Utils_TF_ColorToTeam(12)) == 0 {
		return 12
	}
	if Q_strcasecmp(team, Utils_TF_ColorToTeam(11)) == 0 {
		return 11
	}
	return 0
}

/************************************* Player Utils *************************************/

// qsort comparator over player pointers; the pointer difference is the slot
// difference
func Player_Compare(p1, p2 int) int {
	player1 := &cl.players[p1]
	player2 := &cl.players[p2]

	if player1.spectator != 0 {
		if player2.spectator != 0 {
			return p1 - p2
		}
		return 1
	}
	if player2.spectator != 0 {
		return -1
	}

	if cl.teamplay != 0 {
		if team_comp := strings.Compare(player1.team, player2.team); team_comp != 0 {
			return team_comp
		}
	}

	return p1 - p2
}

func Player_NumtoSlot(num int) int {
	players := make([]int, 0, MAX_CLIENTS)
	for i := 0; i < MAX_CLIENTS; i++ {
		if cl.players[i].name != "" {
			players = append(players, i)
		}
	}

	sort.Slice(players, func(a, b int) bool { return Player_Compare(players[a], players[b]) < 0 })

	if num < 1 || num > len(players) {
		return PLAYER_NUM_NOMATCH
	}

	return players[num-1]
}

func Player_IdtoSlot(id int) int {
	for j := 0; j < MAX_CLIENTS; j++ {
		if cl.players[j].name != "" && cl.players[j].userid == id {
			return j
		}
	}
	return -1
}

func Player_StringtoSlot(arg string) int {
	for i := 0; i < MAX_CLIENTS; i++ {
		if cl.players[i].name != "" && strneq(arg, cl.players[i].name, MAX_SCOREBOARDNAME-1) {
			return i
		}
	}

	if arg == "" {
		return PLAYER_NAME_NOMATCH
	}

	for i := 0; i < len(arg); i++ {
		if !isdigit(arg[i]) {
			return PLAYER_NAME_NOMATCH
		}
	}
	if slot := Player_IdtoSlot(Q_atoi(arg)); slot >= 0 {
		return slot
	}
	return PLAYER_ID_NOMATCH
}

func Player_NametoSlot(name string) int {
	for i := 0; i < MAX_CLIENTS; i++ {
		if cl.players[i].name != "" && strneq(Info_ValueForKey(cl.players[i].userinfo, "name"), name, 31) {
			return i
		}
	}
	return PLAYER_NAME_NOMATCH
}

func Player_SlottoId(slot int) int {
	if slot >= 0 && slot < MAX_CLIENTS && cl.players[slot].name != "" {
		return cl.players[slot].userid
	}
	return -1
}

func Player_MyName() string {
	if cls.demoplayback {
		return Info_ValueForKey(cls.userinfo, "name")
	}
	return Info_ValueForKey(cl.players[cl.playernum].userinfo, "name")
}

func Player_GetSlot(arg string) int {
	response := Player_StringtoSlot(arg)
	if response >= 0 || response == PLAYER_ID_NOMATCH {
		return response
	}

	if at(arg, 0) != '#' {
		return response
	}

	for i := 1; i < len(arg); i++ {
		if !isdigit(arg[i]) {
			return PLAYER_NAME_NOMATCH
		}
	}

	if response = Player_NumtoSlot(Q_atoi(arg[1:])); response >= 0 {
		return response
	}

	return PLAYER_NUM_NOMATCH
}

func Util_F_Match(_msg, f_request string) bool {
	var offset int

	flags := TP_CategorizeMessage(_msg, &offset)

	if flags != 1 && flags != 4 {
		return false
	}

	b := []byte(_msg[offset:])
	for i := range b {
		b[i] &^= 128
	}
	s := string(b)

	if !strings.HasPrefix(s, f_request) {
		return false
	}
	status := 0
	for i := len(f_request); i < len(s); i++ {
		if isdigit(s[i]) && status <= 1 {
			status = 1
		} else if isspace(s[i]) {
			if status == 1 {
				status = 2
			}
		} else {
			return false
		}
	}
	return true
}

// Util_strcasestr returns the index of little in big, ignoring ASCII case,
// or -1. C returns a pointer into big.
func Util_strcasestr(big, little string) int {
	if little == "" {
		return 0
	}

	up := func(c byte) byte {
		if c >= 'a' && c <= 'z' {
			return c - 32
		}
		return c
	}

	for pos := 0; pos < len(big); pos++ {
		if up(big[pos]) == up(little[0]) {
			i, j := pos+1, 1
			for i < len(big) && j < len(little) && up(big[i]) == up(little[j]) {
				i++
				j++
			}
			if j == len(little) {
				return pos
			}
		}
	}

	return -1
}

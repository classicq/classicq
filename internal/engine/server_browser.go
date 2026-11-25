package engine

import (
	"fmt"
	"io"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unsafe"
)

// server_browser.c: the server browser screen over the server scanner.

func Color_For_Map(m int) int {
	m = bound(0, m, 13)
	return 16*m + 8
}

type sb_friend struct {
	next, prev *sb_friend
	name       string
}

var serverscanner *ServerScanner

var friends *sb_friend

var qtv_connect_pending bool
var qtv_connect_time float64

var (
	sb_masterserver        = cvar_t{name: "sb_masterserver", string: "master.quakeservers.net:27000 qwmaster.fodquake.net:27000 master.quakeworld.nu:27000"}
	sb_masterserver_http   = cvar_t{name: "sb_masterserver_http", string: "http://hubapi.quakeworld.nu/v2/masters/master.quakeservers.net:27000 http://hubapi.quakeworld.nu/v2/masters/qwmaster.fodquake.net:27000 http://hubapi.quakeworld.nu/v2/masters/master.quakeworld.nu:27000"}
	sb_player_drawing      = cvar_t{name: "sb_player_drawing", string: "1"}
	sb_refresh_on_activate = cvar_t{name: "sb_refresh_on_activate", string: "0"}
)

var sb_last_rescan_time float64
var sb_ctrlr_count_in_session int
var sb_full_refresh_pending bool
var sb_debug = cvar_t{name: "sb_debug", string: "0"}
var sb_bg_color = cvar_t{name: "sb_bg_color", string: "112"}

var (
	sb_color_bg                    = cvar_t{name: "sb_color_bg", string: "1"}
	sb_color_bg_free               = cvar_t{name: "sb_color_bg_free", string: "51"}
	sb_color_bg_full               = cvar_t{name: "sb_color_bg_full", string: "70"}
	sb_color_bg_empty              = cvar_t{name: "sb_color_bg_empty", string: "112"}
	sb_color_bg_specable           = cvar_t{name: "sb_color_bg_specable", string: "83"}
	sb_qtv_proxy                   = cvar_t{name: "sb_qtv_proxy", string: "qtv.fodquake.net:27599"}
	sb_qtv_lookup                  = cvar_t{name: "sb_qtv_lookup", string: "qtv.fodquake.net:12000"}
	sb_qtv_connect_timeout         = cvar_t{name: "sb_qtv_connect_timeout", string: "2"}
	sb_highlight_sort_column       = cvar_t{name: "sb_highlight_sort_column", string: "1"}
	sb_highlight_sort_column_color = cvar_t{name: "sb_highlight_sort_column_color", string: "70"}
	sb_highlight_sort_column_alpha = cvar_t{name: "sb_highlight_sort_column_alpha", string: "0.5"}
)

var qtvr *qtvr_t

var sb_open bool
var sb_default_settings = true
var sb_qw_server []*QWServer
var sb_qw_server_count uint

var sb_help_prev = -1
var sb_active_help_window int

const SB_HELP_WINDOWS = 4

var sb_check_serverscanner bool

// general display
var sb_active_window int
var sb_selected_filter int
var sb_status_bar string

// server display
var sb_server_insert bool
var sb_server_count_width = 1

// filter display
var sb_filter_insert bool
var sb_filter_delete bool

// inserting filter
var sb_filter_insert_selected_key int
var sb_filter_insert_selected_type int
var sb_filter_insert_value [512]byte
var sb_filter_insert_value_position int
var sb_filter_insert_selected_box int

// player filter
var sb_player_filter bool
var sb_player_filter_entry [512]byte
var sb_player_filter_entry_position int
var sb_player_filter_blink_time float64

// text filter
var sb_text_filter bool
var sb_text_filter_entry [512]byte
var sb_text_filter_entry_position int
var sb_text_filter_blink_time float64

var sb_server_insert_selected_box int
var sb_server_insert_ip [512]byte
var sb_server_insert_ip_position int
var sb_server_insert_port [512]byte
var sb_server_insert_port_position int

const SB_MAX_TABS = 10
const SB_SERVER = 0
const SB_FILTER = 1
const SB_HELP = 2

var current_selected_server *QWServer

var friend_name_max_len int

type column_type int

const (
	SBCT_PLAYERS column_type = iota
	SBCT_MAP
	SBCT_HOSTNAME
	SBCT_PING
)

const SB_SORT_MAX = 4

type tab_column_type struct {
	length int
	type_  column_type
}

type tab_t struct {
	prev, next *tab_t

	name                      string
	max_filter_keyword_length int
	server_count              int
	max_hostname_length       int
	sb_position               int
	changed                   bool
	sort                      int
	server_index              []int  // nil where C has no array
	player_filter             string // "" is C's null pointer
	text_filter               string
	filters                   []*filter
	friends                   bool
	friend_links              []*sb_friend

	columns      int
	column_types []tab_column_type
}

var tab_first *tab_t
var tab_last *tab_t
var tab_active *tab_t

var column_names = []string{
	"players",
	"map",
	"hostname",
	"ping",
}

type filter struct {
	key     int
	keyword string
	type_   int
	value   string
	fvalue  float32
}

type filter_type_t struct {
	type_            int // 0 - float, 1 - string
	name             string
	description      string
	compare_function func(server *QWServer, filter *filter) bool
}

var filter_num_operators = []string{
	"==",
	"<=",
	">=",
}

var filter_char_operators = []string{
	"isin",
	"!isin",
}

func num_check(sv, fv float32, type_ int) bool {
	if type_ == 0 {
		if sv == fv {
			return true
		}
	} else if type_ == 1 {
		if sv <= fv {
			return true
		}
	} else if type_ == 2 {
		if sv >= fv {
			return true
		}
	}
	return false
}

func char_check(sv, fv string, type_ int) bool {
	if type_ == 0 {
		return strings.Contains(fv, sv)
	} else if type_ == 1 {
		return strings.Contains(sv, fv)
	}

	return false
}

func filter_player_check(server *QWServer, filter *filter) bool {
	return num_check(float32(server.numplayers), filter.fvalue, filter.type_)
}

func filter_map_check(server *QWServer, filter *filter) bool {
	if server.map_ != nil {
		return char_check(*server.map_, filter.value, filter.type_)
	}
	return false
}

func filter_hostname_check(server *QWServer, filter *filter) bool {
	if server.hostname != nil {
		return char_check(*server.hostname, filter.value, filter.type_)
	}
	return false
}

func filter_teamplay_check(server *QWServer, filter *filter) bool {
	return num_check(float32(server.teamplay), filter.fvalue, filter.type_)
}

func filter_max_clients_check(server *QWServer, filter *filter) bool {
	return num_check(float32(server.maxclients), filter.fvalue, filter.type_)
}

func filter_ping_check(server *QWServer, filter *filter) bool {
	return num_check(float32(server.pingtime/1000), filter.fvalue, filter.type_)
}

const SB_FILTER_TYPE_MAX = 6

var filter_types = [SB_FILTER_TYPE_MAX]filter_type_t{
	{0, "players", "amount of players on the servers", filter_player_check},
	{1, "map", "mapname", filter_map_check},
	{1, "hostname", "hostname", filter_hostname_check},
	{0, "teamplay", "teamplay", filter_teamplay_check},
	{0, "max_clients", "max clients allowed on server", filter_max_clients_check},
	{0, "ping", "ping to the server", filter_ping_check},
}

var old_keydest keydest_t

func SB_Proxy_Connect(server *QWServer) {
	if qtvr != nil {
		QTVR_Destroy(qtvr)
	}

	qtvr = QTVR_Create(sb_qtv_lookup.string, NET_AdrToString(&server.addr))
	if qtvr == nil {
		Com_Printf("QTV Retriever not running.\n")
		return
	}

	Com_Printf("Retrieving proxy for %s\n", NET_AdrToString(&server.addr))
	qtv_connect_pending = true
	qtv_connect_time = cls.realtime
}

func Filter_Type_String(type_ int) string {
	switch type_ {
	case 0:
		return "=="
	case 1:
		return ">="
	case 2:
		return "<="
	case 3:
		return "isin"
	case 4:
		return "!isin"
	}
	return ""
}

func sb_add_tab(name string) *tab_t {
	tab := &tab_t{}

	tab.column_types = make([]tab_column_type, 4)

	tab.name = name

	if tab_first == nil {
		tab_first = tab
	}

	if tab_last == nil {
		tab_last = tab
	} else {
		tab_last.next = tab
		tab.prev = tab_last
		tab_last = tab_last.next
	}

	if tab_active == nil {
		tab_active = tab
	}

	if name == "friends" {
		tab.friends = true
	}

	tab.column_types[0].type_ = SBCT_PING
	tab.column_types[1].type_ = SBCT_PLAYERS
	tab.column_types[2].type_ = SBCT_MAP
	tab.column_types[3].type_ = SBCT_HOSTNAME

	tab.columns = 4

	tab.sort = 1

	return tab
}

func sb_del_tab(tab *tab_t) {
	if tab.next == nil && tab.prev == nil {
		tab_first, tab_last, tab_active = nil, nil, nil
	} else if tab.next != nil && tab.prev == nil {
		tab_first = tab.next
		tab.next.prev = nil
		if tab == tab_active {
			tab_active = tab.next
		}
	} else if tab.next == nil && tab.prev != nil {
		tab.prev.next = nil
		tab_last = tab.prev
		if tab == tab_active {
			tab_active = tab.prev
		}
	} else {
		tab.prev.next = tab.next
		tab.next.prev = tab.prev
		if tab == tab_active {
			tab_active = tab.next
		}
	}
}

func sb_del_tab_by_name(name string) {
	for tab := tab_first; tab != nil; tab = tab.next {
		if tab.name == name {
			sb_del_tab(tab)
			return
		}
	}

	Com_Printf("tab \"%s\" not found.\n", name)
}

func SB_Set_Statusbar(format string, a ...any) {
	sb_status_bar = snprintf(512, format, a...)
}

func SB_Refresh() {
	sb_full_refresh_pending = true

	if serverscanner != nil {
		ServerScanner_FreeServers(serverscanner, sb_qw_server)
		ServerScanner_Delete(serverscanner)
	}
	serverscanner = nil
	sb_qw_server = nil
	current_selected_server = nil
	sb_qw_server_count = 0
	serverscanner = ServerScanner_Create(sb_masterserver.string, sb_masterserver_http.string)
	if serverscanner == nil {
		SB_Set_Statusbar("error creating server scanner!")
	}

	for tab := tab_first; tab != nil; tab = tab.next {
		tab.sb_position = 0
		tab.changed = false
	}
}

// str is the whole C buffer, the text ends at its first NUL
func handle_textbox(key int, str []byte, position *int) {
	size := len(str)
	length := cbufEnd(str, 0)

	if key == int(K_LEFTARROW) {
		*position -= 1
		if *position < 0 {
			*position = 0
		}
		return
	}

	if key == int(K_RIGHTARROW) {
		*position += 1
		if *position > length {
			*position = length
		}
		return
	}

	if key == int(K_DEL) {
		i := *position
		if *position >= length-1 {
			return
		}
		for i+1 < size && str[i] != 0 {
			str[i] = str[i+1]
			i++
		}
		return
	}

	if key == int(K_BACKSPACE) {
		i := *position - 1
		if *position < 1 {
			return
		}
		// C reads one byte past the buffer here, the last byte is cleared instead
		for i < size-1 {
			str[i] = str[i+1]
			i++
		}
		str[size-1] = 0
		*position -= 1
		return
	}

	if key < 32 || key > 127 {
		return
	}

	if length >= size {
		return
	}
	i := length
	for i > *position {
		str[i] = str[i-1]
		i--
	}

	str[*position] = byte(key)
	*position += 1
}

func Check_Server_Against_Filter(tab *tab_t, server *QWServer) bool {
	for _, fe := range tab.filters {
		if !filter_types[fe.key].compare_function(server, fe) {
			return false
		}
	}
	return true
}

func remove_colors(str string, size int) string {
	new_string := make([]byte, 0, size)

	ptr := 0
	x := 0

	for ptr < len(str) && str[ptr] != 0 && x < size {
		if str[ptr] == '&' {
			if x+1 < size {
				if ptr+1 < len(str) && str[ptr+1] == 'c' {
					if x+5 >= size {
						break
					}
					ptr += 5
					x += 5
				}
			} else {
				break
			}
		}
		new_string = append(new_string, readablechars[at(str, ptr)])

		ptr++
		x++
	}

	return string(new_string)
}

func check_player_name(name string, server *QWServer) bool {
	player := name

	if server.numplayers == 0 && server.numspectators == 0 {
		return false
	}

	for i := uint32(0); i < server.numplayers; i++ {
		if server.players[i].name != nil {
			player_uncolored := remove_colors(*server.players[i].name, len(*server.players[i].name))
			if Util_strcasestr(player_uncolored, player) >= 0 {
				return true
			}
		}
	}

	for i := uint32(0); i < server.numspectators; i++ {
		if server.spectators[i].name != nil {
			player_uncolored := remove_colors(*server.spectators[i].name, len(*server.spectators[i].name))
			if Util_strcasestr(player_uncolored, player) >= 0 {
				return true
			}
		}
	}
	return false
}

func check_player(tab *tab_t, server *QWServer) bool {
	return check_player_name(tab.player_filter, server)
}

func check_text_hm(text string, server *QWServer) bool {
	if server.hostname != nil {
		if Util_strcasestr(*server.hostname, text) >= 0 {
			return true
		}
	}
	if server.map_ != nil {
		if Util_strcasestr(*server.map_, text) >= 0 {
			return true
		}
	}
	if server.gamedir != nil {
		if Util_strcasestr(*server.gamedir, text) >= 0 {
			return true
		}
	}
	return false
}

func check_text(tab *tab_t, server *QWServer) bool {
	return check_text_hm(tab.text_filter, server)
}

// the comparators take indices into sb_qw_server

func activity_then_ping_compare(a, b int) int {
	x := sb_qw_server[a]
	y := sb_qw_server[b]

	x_empty, y_empty := 1, 1
	if x.numplayers > 0 {
		x_empty = 0
	}
	if y.numplayers > 0 {
		y_empty = 0
	}

	if x_empty != y_empty {
		return x_empty - y_empty
	}

	ping_cmp := int(int32(x.pingtime/1000)) - int(int32(y.pingtime/1000))
	if ping_cmp != 0 {
		return ping_cmp
	}

	return Q_strcasecmp(*x.hostname, *y.hostname)
}

func sort_tab(tab *tab_t) {
	idx := tab.server_index[:tab.server_count]
	sort.SliceStable(idx, func(i, j int) bool {
		return activity_then_ping_compare(idx[i], idx[j]) < 0
	})
}

func stubby(tab *tab_t, server *QWServer) bool {
	return true
}

func update_friends_tab(tab *tab_t) {
	tab.max_hostname_length = 0

	count := 0
	for s := friends; s != nil; s = s.next {
		for x := uint(0); x < sb_qw_server_count; x++ {
			if check_player_name(s.name, sb_qw_server[x]) {
				count++
			}
		}
	}

	tab.server_index = nil
	tab.friend_links = nil

	if count > 10000 || count <= 0 {
		tab.server_count = 0
		tab.sb_position = 0
		return
	}

	tab.server_index = make([]int, count)
	tab.friend_links = make([]*sb_friend, count)

	tab.server_count = count

	i := 0
	for s := friends; s != nil; s = s.next {
		for x := uint(0); x < sb_qw_server_count; x++ {
			if check_player_name(s.name, sb_qw_server[x]) {
				tab.friend_links[i] = s
				tab.server_index[i] = int(x)
				i++
			}
		}
	}
}

func update_tab(tab *tab_t) {
	var cf func(tab *tab_t, server *QWServer) bool

	tab.max_hostname_length = 0

	if tab.friends {
		update_friends_tab(tab)
		return
	}

	if len(tab.filters) != 0 && tab.player_filter == "" {
		cf = Check_Server_Against_Filter
	} else if tab.player_filter != "" {
		cf = check_player
	} else if tab.text_filter != "" {
		cf = check_text
	} else {
		cf = stubby
	}

	tab.server_index = nil

	count := 0
	for x := uint(0); x < sb_qw_server_count; x++ {
		if cf(tab, sb_qw_server[x]) {
			count++
		}
	}

	if count > 10000 || count <= 0 {
		tab.server_count = 0
		tab.sb_position = 0
		return
	}

	tab.server_index = make([]int, count)

	tab.server_count = count

	i := 0
	for x := uint(0); x < sb_qw_server_count; x++ {
		if cf(tab, sb_qw_server[x]) {
			tab.server_index[i] = int(x)
			i++
		}
	}

	if tab.sb_position >= tab.server_count {
		tab.sb_position = tab.server_count - 1
	}
	if tab.sb_position < 0 {
		tab.sb_position = 0
	}

	sort_tab(tab)

	for x := 0; x < tab.columns; x++ {
		tab.column_types[x].length = 0
	}

	// indexes sb_qw_server directly rather than through server_index, as in C
	for i := 0; i < tab.server_count; i++ {
		for x := 0; x < tab.columns; x++ {
			switch tab.column_types[x].type_ {
			case SBCT_PING:
				temp := sbDigits(int32(sb_qw_server[i].pingtime / 1000))
				if temp < 4 {
					temp = 4
				}
				if temp > tab.column_types[x].length {
					tab.column_types[x].length = temp
				}
			case SBCT_PLAYERS:
				tab.column_types[x].length = 7
			case SBCT_MAP:
				if sb_qw_server[i].map_ != nil {
					temp := len(*sb_qw_server[i].map_)
					if temp > tab.column_types[x].length {
						tab.column_types[x].length = temp
					}
				}
			case SBCT_HOSTNAME:
				if sb_qw_server[i].hostname != nil {
					temp := len(*sb_qw_server[i].hostname)
					if temp > tab.column_types[x].length {
						tab.column_types[x].length = temp
					}
				}
			}
		}
	}

	if !tab.changed {
		return
	}

	if tab == tab_active {
		if current_selected_server != nil {
			i := 0
			for ; i < tab.server_count; i++ {
				if current_selected_server == sb_qw_server[tab.server_index[i]] {
					break
				}
			}
			if i == tab.server_count {
				return
			}
			tab.sb_position = i
		} else {
			i := tab.server_index[tab.sb_position]
			current_selected_server = sb_qw_server[i]
		}
	}
}

func SB_Update_Tabs() {
	for tab := tab_first; tab != nil; tab = tab.next {
		update_tab(tab)
	}
}

func SB_Help_Handler(key int) {
	if key == int(K_ESCAPE) {
		sb_active_window = sb_help_prev
		return
	}

	if key == int(K_RIGHTARROW) {
		sb_active_help_window++
		if sb_active_help_window >= SB_HELP_WINDOWS {
			sb_active_help_window = 0
		}
		return
	}

	if key == int(K_LEFTARROW) {
		sb_active_help_window--
		if sb_active_help_window <= 0 {
			sb_active_help_window = SB_HELP_WINDOWS - 1
		}
		return
	}
}

func SB_Close() {
	key_dest = old_keydest
	sb_open = false
}

// The server under the cursor; nil where C would read past the list.
func sb_selected_server(tab *tab_t) *QWServer {
	i := tab.sb_position
	if tab.server_index != nil {
		if i < 0 || i >= len(tab.server_index) {
			return nil
		}
		i = tab.server_index[i]
	}
	if i < 0 || i >= len(sb_qw_server) {
		return nil
	}
	return sb_qw_server[i]
}

func SB_Key(key int) {
	if key >= int(K_F1) && key <= int(K_F12) {
		kb := keybindings[key]
		if kb != nil {
			if len(*kb) > 0 && (*kb)[0] == '+' { // button commands add keynum as a parm
				Cbuf_AddText(snprintf(1024, "%s %d\n", *kb, key))
				keyactive[key] = true
			} else {
				Cbuf_AddText(*kb)
				Cbuf_AddText("\n")
			}
		}

		return
	}

	if sb_active_window == SB_HELP {
		SB_Help_Handler(key)
		return
	}

	if key == 'h' && keydown[K_CTRL] {
		sb_help_prev = sb_active_window
		sb_active_window = SB_HELP
		return
	}

	if sb_active_window == SB_FILTER {
		if sb_filter_delete {
			if key == 'y' {
				SB_Filter_Delete_Filter()
				SB_Update_Tabs()
			}
			sb_filter_delete = false
			return
		}

		if sb_filter_insert {
			SB_Filter_Insert_Handler(key)
			return
		}

		if key == int(K_DOWNARROW) {
			sb_selected_filter++
			if sb_selected_filter >= len(tab_active.filters) {
				sb_selected_filter = 0
			}
			return
		}

		if key == int(K_UPARROW) {
			sb_selected_filter--
			if sb_selected_filter < 0 {
				sb_selected_filter = len(tab_active.filters)
			}
			return
		}

		if key == int(K_INS) {
			sb_filter_insert = true
			return
		}

		if key == int(K_ENTER) {
			return
		}

		if key == int(K_DEL) {
			sb_filter_delete = true
			return
		}
	}

	if sb_active_window == SB_SERVER {
		tab := tab_active

		if sb_player_filter {
			tab.changed = true
			update := false
			if key == int(K_ESCAPE) {
				sb_player_filter = false
				return
			}

			if key == int(K_ENTER) {
				sb_player_filter = false
			}

			handle_textbox(key, sb_player_filter_entry[:], &sb_player_filter_entry_position)
			if tab.player_filter != "" {
				tab.player_filter = ""
				update = true
			}
			if entry := cbufStr(sb_player_filter_entry[:], 0); len(entry) > 0 {
				tab.player_filter = entry
				update_tab(tab)
			}
			if update {
				update_tab(tab)
			}

			if key != int(K_UPARROW) && key != int(K_DOWNARROW) && key != int(K_ENTER) {
				return
			}
		}

		if sb_text_filter {
			tab.changed = true
			update := false
			if key == int(K_ESCAPE) {
				sb_text_filter = false
				return
			}

			if key == int(K_ENTER) {
				sb_text_filter = false
			}

			handle_textbox(key, sb_text_filter_entry[:], &sb_text_filter_entry_position)
			if tab.text_filter != "" {
				tab.text_filter = ""
				update = true
			}
			if entry := cbufStr(sb_text_filter_entry[:], 0); len(entry) > 0 {
				tab.text_filter = entry
				update_tab(tab)
			}
			if update {
				update_tab(tab)
			}

			if key != int(K_UPARROW) && key != int(K_DOWNARROW) && key != int(K_ENTER) {
				return
			}
		}

		if sb_server_insert {
			SB_Server_Insert_Handler(key)
			return
		}

		if key == int(K_INS) {
			if tab_active.friends {
				return
			}
			sb_server_insert = true
		}

		if key == int(K_UPARROW) {
			if sb_qw_server == nil {
				return
			}

			if keydown[K_SHIFT] {
				tab.sb_position -= 10
			} else if keydown[K_CTRL] {
				tab.sb_position -= 20
			} else {
				tab.sb_position--
			}

			if tab.sb_position < 0 {
				tab.sb_position = 0
			}

			if tab.server_count == 0 || tab.sb_position > tab.server_count {
				current_selected_server = nil
				return
			}

			i := tab.server_index[tab.sb_position]
			current_selected_server = sb_qw_server[i]
			tab.changed = true
			return
		}

		if key == int(K_DOWNARROW) {
			if sb_qw_server == nil {
				return
			}

			if keydown[K_SHIFT] {
				tab.sb_position += 10
			} else if keydown[K_CTRL] {
				tab.sb_position += 20
			} else {
				tab.sb_position++
			}
			if tab.sb_position >= tab.server_count {
				tab.sb_position = tab.server_count - 1
			}

			if tab.server_count == 0 || tab.sb_position > tab.server_count {
				current_selected_server = nil
				return
			}

			i := tab.server_index[tab.sb_position]
			current_selected_server = sb_qw_server[i]
			tab.changed = true
			return
		}

		if key == int(K_ENTER) {
			if sb_qw_server == nil {
				return
			}

			server := sb_selected_server(tab)
			if server == nil {
				return
			}

			if keydown[K_CTRL] {
				Cbuf_AddText("spectator 1\n")
				Cbuf_AddText(va("connect %s\n", NET_AdrToString(&server.addr)))
				SB_Close()
				return
			}

			if keydown[K_SHIFT] {
				SB_Proxy_Connect(server)
				SB_Close()
				return
			}

			if server.maxclients == server.numplayers {
				Cbuf_AddText("spectator 1\n")
			} else {
				Cbuf_AddText("spectator 0\n")
			}

			Cbuf_AddText(va("connect %s\n", NET_AdrToString(&server.addr)))
			SB_Close()
			return
		}

		if key == 'f' && keydown[K_CTRL] {
			sb_player_filter = true
			return
		}

		if key == '/' {
			sb_text_filter = true
			return
		}

		if key == 'r' {
			if keydown[K_CTRL] {
				if sb_ctrlr_count_in_session == 0 && serverscanner != nil {
					ServerScanner_RescanAll(serverscanner)
				} else {
					SB_Refresh()
				}
				sb_ctrlr_count_in_session++
				return
			}

			if cls.realtime-sb_last_rescan_time < 0.5 {
				return
			}

			if sb_qw_server != nil {
				if server := sb_selected_server(tab); server != nil {
					ServerScanner_RescanServer(serverscanner, server)
					sb_last_rescan_time = cls.realtime
				}
			}
		}
	}

	if key == int(K_ESCAPE) {
		SB_Close()
		return
	}
}

// SB_Server_Add is commented out in C

func SB_Add_Filter_To_Tab(tab *tab_t, key, type_ int, value string) {
	if tab == nil {
		return
	}

	if len(value) == 0 {
		return
	}

	f := &filter{}

	f.key = key

	f.keyword = filter_types[key].name

	f.type_ = type_

	f.value = value

	if filter_types[key].type_ == 0 {
		f.fvalue = float32(atof(value))
	}

	tab.filters = append(tab.filters, f)
	if i := len(f.keyword); tab.max_filter_keyword_length < i {
		tab.max_filter_keyword_length = i
	}
}

func sb_default_tabs() {
	sb_add_tab("all")
	tab := sb_add_tab("duel")
	SB_Add_Filter_To_Tab(tab, 0, 2, "1")
	SB_Add_Filter_To_Tab(tab, 4, 0, "2")
	tab = sb_add_tab("2on2")
	SB_Add_Filter_To_Tab(tab, 0, 2, "1")
	SB_Add_Filter_To_Tab(tab, 4, 0, "4")
	tab = sb_add_tab("4on4")
	SB_Add_Filter_To_Tab(tab, 0, 2, "1")
	SB_Add_Filter_To_Tab(tab, 4, 0, "8")
	sb_add_tab("friends")
}

func SB_Activate_f() {
	if sb_open {
		return
	}

	old_keydest = key_dest
	key_dest = key_serverbrowser
	sb_open = true
	sb_ctrlr_count_in_session = 0

	if tab_first == nil {
		sb_default_tabs()
		tab_active = tab_first
	}

	if sb_refresh_on_activate.value == 1 || serverscanner == nil {
		SB_Refresh()
	}
	SB_Update_Tabs()
}

func check_selected_type(type_ int) int {
	var max int

	if filter_types[sb_filter_insert_selected_key].type_ == 0 {
		max = 3
	} else {
		max = 2
	}

	if type_ < 0 {
		type_ = max - 1
	}
	if type_ >= max {
		type_ = 0
	}

	return type_
}

func SB_Filter_Insert_Handler(key int) {
	if key == int(K_ENTER) {
		SB_Add_Filter_To_Tab(tab_active, sb_filter_insert_selected_key, sb_filter_insert_selected_type, cbufStr(sb_filter_insert_value[:], 0))
		sb_filter_insert = false
		sb_filter_insert_value_position = 0
		update_tab(tab_active)
		sb_default_settings = false
		return
	}

	if key == int(K_TAB) {
		sb_filter_insert_selected_box++
		if sb_filter_insert_selected_box > 2 {
			sb_filter_insert_selected_box = 0
		}

		return
	}

	if key == int(K_ESCAPE) {
		sb_filter_insert = false
		return
	}

	if sb_filter_insert_selected_box == 1 {
		if key == int(K_DOWNARROW) {
			sb_filter_insert_selected_type--
		} else if key == int(K_UPARROW) {
			sb_filter_insert_selected_type++
		}

		sb_filter_insert_selected_type = check_selected_type(sb_filter_insert_selected_type)
		return
	}

	if sb_filter_insert_selected_box == 2 {
		handle_textbox(key, sb_filter_insert_value[:], &sb_filter_insert_value_position)
		return
	}

	if sb_filter_insert_selected_box == 0 {
		if key == int(K_DOWNARROW) {
			sb_filter_insert_selected_key--
		} else if key == int(K_UPARROW) {
			sb_filter_insert_selected_key++
		}

		if sb_filter_insert_selected_key < 0 {
			sb_filter_insert_selected_key = SB_FILTER_TYPE_MAX - 1
		}
		if sb_filter_insert_selected_key >= SB_FILTER_TYPE_MAX {
			sb_filter_insert_selected_key = 0
		}

		sb_filter_insert_selected_type = check_selected_type(sb_filter_insert_selected_type)
		return
	}
}

func SB_Filter_Delete_Filter() {
	if sb_selected_filter < 0 || sb_selected_filter >= len(tab_active.filters) {
		return
	}
	tab_active.filters = append(tab_active.filters[:sb_selected_filter], tab_active.filters[sb_selected_filter+1:]...)
}

func SB_Server_Insert_Handler(key int) {
	if key == int(K_TAB) {
		sb_server_insert_selected_box++
		if sb_server_insert_selected_box > 1 {
			sb_server_insert_selected_box = 0
		}
		return
	}

	if key == int(K_ESCAPE) {
		sb_server_insert = false
		return
	}

	if key == int(K_ENTER) {
		// SB_Server_Add does nothing
		sb_server_insert = false
		return
	}

	if sb_server_insert_selected_box == 0 {
		handle_textbox(key, sb_server_insert_ip[:], &sb_server_insert_ip_position)
		return
	}

	if sb_server_insert_selected_box == 1 {
		handle_textbox(key, sb_server_insert_port[:], &sb_server_insert_port_position)
		return
	}
}

func SB_Draw_Tabs() {
	width := int(vid.conwidth) / 8

	x := len(tab_active.name)
	if x%2 != 0 {
		x++
	}

	l := x
	x = x/2 + 1

	Draw_ColoredString((width/2-x)*8, 0, va("&cf44<%-*s>", l, tab_active.name), 1)
	l = width/2 - x - 1
	r := width/2 + x + 1

	i := l
	for tab := tab_active.prev; tab != nil; tab = tab.prev {
		x = len(tab.name)
		i -= x
		Draw_String(i*8, 0, tab.name)
		i--
	}

	i = r
	for tab := tab_active.next; tab != nil; tab = tab.next {
		x = len(tab.name)
		Draw_String(i*8, 0, tab.name)
		i += x
		i++
	}
}

func SB_Draw_Background() {
	Draw_Fill(0, 0, int(vid.conwidth), int(vid.conheight), int(sb_bg_color.value))
}

func SB_Draw_Filter_Insert() {
	i := 0
	Draw_String(8, 24, filter_types[sb_filter_insert_selected_key].name)
	Draw_String(0, 24+sb_filter_insert_selected_box*8, ">")
	Draw_String(8, 48+8*i, "press arrow up/down to switch trough available options")
	i++
	Draw_String(8, 48+8*i, "tab to switch trough the entry fields")
	i++
	Draw_String(8, 48+8*i, "esc to leave")
	i++

	if sb_filter_insert_selected_box == 0 {
		Draw_String(8, 48+8*i, filter_types[sb_filter_insert_selected_key].description)
		i++
	}

	if sb_filter_insert_selected_box == 1 {
		// both branches show the numeric operators in C
		Draw_String(8, 48+8*i, filter_num_operators[sb_filter_insert_selected_type])
	}

	if filter_types[sb_filter_insert_selected_key].type_ == 0 {
		Draw_String(8, 32, filter_num_operators[sb_filter_insert_selected_type])
	} else {
		Draw_String(8, 32, filter_char_operators[sb_filter_insert_selected_type])
	}

	Draw_String(8, 40, cbufStr(sb_filter_insert_value[:], 0))
	if sb_filter_insert_selected_box == 2 {
		Draw_String(8+8*sb_filter_insert_value_position, 40, "_")
	}
}

func SB_Draw_Filter() {
	if sb_filter_insert {
		SB_Draw_Filter_Insert()
		return
	}

	Draw_String(8, 16, "Filter:")
	for i, filter := range tab_active.filters {
		if i == sb_selected_filter {
			Draw_String(0, 24+i*8, ">")
		}
		Draw_String(8, 24+i*8, va("%*s %5s %s", tab_active.max_filter_keyword_length, filter.keyword, Filter_Type_String(filter.type_), filter.value))
	}
}

func SB_Draw_Server_Insert() {
	Draw_String(8, 24, va("ip: %s", cbufStr(sb_server_insert_ip[:], 0)))
	if sb_server_insert_selected_box == 0 {
		Draw_String(8+8*(sb_server_insert_ip_position+3), 24, "_")
	}

	Draw_String(8, 32, va("port: %s", cbufStr(sb_server_insert_port[:], 0)))
	if sb_server_insert_selected_box == 1 {
		Draw_String(8+8*(sb_server_insert_port_position+6), 32, "_")
	}
}

func sort_players_team(x, y *QWPlayer) int {
	if x.team != nil && y.team != nil {
		if i := strings.Compare(*x.team, *y.team); i == 0 {
			return int(int32(y.frags - x.frags))
		} else {
			return i
		}
	}

	return int(int32(y.frags - x.frags))
}

func sort_players(x, y *QWPlayer) int {
	return int(int32(y.frags - x.frags))
}

// k += snprintf(string + k, sizeof(string) - k, ...) on a 512 byte buffer that
// C keeps on the stack; here it lives between frames so drawing allocates nothing.
var sbRow []byte

var sb_sorted_players []*QWPlayer // kept between frames

func sbRowString() string {
	if len(sbRow) == 0 {
		return ""
	}
	return unsafe.String(&sbRow[0], len(sbRow))
}

// "%-*.*s " with width as both the precision and the field width
func sbRowStr(k *int, s string, width int) {
	if len(s) > width {
		s = s[:width]
	}
	*k += width + 1
	sbRow = append(sbRow, s...)
	for n := len(s); n < width; n++ {
		sbRow = append(sbRow, ' ')
	}
	sbRow = append(sbRow, ' ')
	sbRow = sbRow[:min(len(sbRow), 511)]
}

// "%*d" followed by sep
func sbRowInt(k *int, v int32, width int, sep byte) {
	*k += max(sbDigits(v), width) + 1
	sbRow = append(appendPadInt(sbRow, int64(v), width), sep)
	sbRow = sbRow[:min(len(sbRow), 511)]
}

func sbDigits(v int32) int {
	var tmp [12]byte
	return len(strconv.AppendInt(tmp[:0], int64(v), 10))
}

// "%*d": v right-justified in width, appended to b
func appendPadInt(b []byte, v int64, width int) []byte {
	var tmp [20]byte
	d := strconv.AppendInt(tmp[:0], v, 10)
	for n := len(d); n < width; n++ {
		b = append(b, ' ')
	}
	return append(b, d...)
}

func SB_Draw_Server() {
	var k, i, x, y, z int
	var offset int
	var hostname, map_ string
	var server *QWServer

	if sb_server_insert {
		SB_Draw_Server_Insert()
		return
	}

	if sb_qw_server_count == 0 {
		return
	}

	tab := tab_active
	if tab == nil {
		return
	}
	player_space := 0

	// Header
	var str string
	sbRow = sbRow[:0]
	k = 0
	for x = 0; x < tab.columns; x++ {
		sbRowStr(&k, column_names[tab.column_types[x].type_], tab.column_types[x].length)

		if k >= 512 {
			break
		}
	}
	Draw_ColoredString(8, 16, sbRowString(), 0)

	if sb_player_filter || sb_text_filter {
		kind := "player search"
		if sb_text_filter {
			kind = "text search"
		}
		str = snprintf(512, "Server: %*d/%*d - %s: ", sb_server_count_width, tab.sb_position+1, sb_server_count_width, tab.server_count, kind)
		i = len(str)
		Draw_String(8, 8, str)
		x = 1
		if tab.player_filter != "" || tab.text_filter != "" {
			s := tab.player_filter
			if tab.text_filter != "" {
				s = tab.text_filter
			}
			x = len(s)
			if x == 0 {
				x = 1
			} else {
				x += 1
			}

			if sb_player_filter || sb_text_filter {
				Draw_Fill(8+i*8, 8, x*8, 8, 55)
			}
			Draw_String(8+i*8, 8, s)
		} else {
			if sb_player_filter || sb_text_filter {
				Draw_Fill(8+i*8, 8, x*8, 8, 55)
			}
		}

		if sb_player_filter && !sb_text_filter {
			if sb_player_filter_blink_time < cls.realtime {
				Draw_Character(8+i*8+sb_player_filter_entry_position*8, 8, 11)
			}

			if sb_player_filter_blink_time+float64(float32(0.2)) < cls.realtime {
				sb_player_filter_blink_time = cls.realtime + float64(float32(0.2))
			}
		}

		if sb_text_filter {
			if sb_text_filter_blink_time < cls.realtime {
				Draw_Character(8+i*8+sb_text_filter_entry_position*8, 8, 11)
			}

			if sb_text_filter_blink_time+float64(float32(0.2)) < cls.realtime {
				sb_text_filter_blink_time = cls.realtime + float64(float32(0.2))
			}
		}
	} else {
		Draw_String(8, 8, scratchf(1024, "Server: %*d/%*d ", sb_server_count_width, tab.sb_position+1, sb_server_count_width, tab.server_count))
	}

	if tab.server_count == 0 {
		return
	}

	line_space := int(vid.conheight)/8 - 4
	if sb_player_drawing.value == 1 {
		if tab.server_index == nil {
			z = tab.sb_position
		} else {
			z = tab.server_index[tab.sb_position]
		}

		if sb_qw_server[z].numplayers != 0 || sb_qw_server[z].numspectators != 0 {
			player_space = int(sb_qw_server[z].numspectators + sb_qw_server[z].numplayers + 1)
		}
		z = 0
	}

	x = 0
	if tab.server_count > line_space {
		x = int(float32(tab.sb_position) - float32(line_space)/2.0)
		if x < 0 {
			x = 0
			offset = tab.sb_position
		} else if x > tab.server_count-line_space {
			x = tab.server_count - line_space
			offset = line_space - (tab.server_count - tab.sb_position)
		} else {
			offset = tab.sb_position - x
		}
	} else {
		offset = tab.sb_position
	}

	offset += z
	y = x + 1

	for i = 0; x < tab.server_count && i < line_space; i, x = i+1, x+1 {
		if tab.server_index == nil {
			z = x
		} else {
			z = tab.server_index[x]
		}
		server = sb_qw_server[z]

		if server.hostname == nil {
			hostname = NET_AdrToString(&server.addr)
		} else {
			hostname = *server.hostname
		}

		if server.map_ == nil {
			map_ = " "
		} else {
			map_ = *server.map_
		}

		if sb_color_bg.value == 1 {
			if server.maxclients == server.numplayers {
				if server.maxspectators == server.numspectators {
					Draw_Fill(0, 24+i*8, int(vid.conwidth), 8, int(sb_color_bg_full.value))
				} else {
					Draw_Fill(0, 24+i*8, int(vid.conwidth), 8, int(sb_color_bg_specable.value))
				}
			} else if server.maxclients > server.numplayers && server.numplayers > 0 {
				Draw_Fill(0, 24+i*8, int(vid.conwidth), 8, int(sb_color_bg_free.value))
			} else if server.numplayers == 0 {
				Draw_Fill(0, 24+i*8, int(vid.conwidth), 8, int(sb_color_bg_empty.value))
			}
		}

		if server == current_selected_server {
			Draw_Fill(0, 24+i*8, int(vid.conwidth), 9, 171)
		}

		sbRow = sbRow[:0]

		k = 0
		for tab_type := 0; tab_type < tab.columns; tab_type++ {
			length := tab.column_types[tab_type].length
			switch tab.column_types[tab_type].type_ {
			case SBCT_PING:
				sbRowInt(&k, int32(server.pingtime/1000), length, ' ')
			case SBCT_PLAYERS:
				sbRowInt(&k, int32(server.numplayers), 3, '/')
				sbRowInt(&k, int32(server.maxclients), 3, ' ')
			case SBCT_MAP:
				sbRowStr(&k, map_, length)
			case SBCT_HOSTNAME:
				sbRowStr(&k, hostname, length)
			}

			if k >= 512 {
				break
			}
		}

		Draw_String(8, 24+i*8, sbRowString())

		y++
	}
	Draw_String(0, 24+offset*8, ">")

	if sb_qw_server_count == 0 {
		return
	}

	if tab.sb_position < 0 || tab.sb_position >= tab.server_count {
		return
	}

	if tab.server_index != nil {
		x = tab.sb_position
		x = tab.server_index[x]
		if x > int(sb_qw_server_count) {
			return
		}
		server = sb_qw_server[x]
	} else {
		if tab.sb_position > int(sb_qw_server_count) {
			return
		}
		server = sb_qw_server[tab.sb_position]
	}

	if server == nil {
		return
	}

	if sb_player_drawing.value == 1 {
		if server.numplayers == 0 && server.numspectators == 0 {
			return
		}
		y = (line_space - player_space) + 3
		if y < offset+3 {
			y = offset + 1 + 3
		}

		Draw_Fill(0, y*8, int(vid.conwidth), player_space*8, 113)
		Draw_Fill(0, y*8, int(vid.conwidth), 8, 20)
		Draw_String(0, y*8, "ping time frags team")
		y++

		sorted_players := sb_sorted_players[:0]
		for i := range int(server.numplayers) {
			sorted_players = append(sorted_players, &server.players[i])
		}
		sb_sorted_players = sorted_players

		if server.teamplay > 0 {
			slices.SortStableFunc(sorted_players, sort_players_team)
		} else {
			slices.SortStableFunc(sorted_players, sort_players)
		}

		for _, player := range sorted_players {
			if player.name != nil {
				Draw_Fill((5+5)*8, y*8, 5*8, 4, Color_For_Map(int(int32(player.topcolor))))
				Draw_Fill((5+5)*8, y*8+4, 5*8, 4, Color_For_Map(int(int32(player.bottomcolor))))
				team := " "
				if player.team != nil {
					team = *player.team
				}
				Draw_ColoredString(0, y*8, scratchf(1024, "%4d %4d %4d  %4.4s&cFFF  %s", int32(player.ping), int32(player.time), int32(player.frags), team, *player.name), 0)
				y++
			}
		}

		for i := range server.spectators {
			spectator := &server.spectators[i]
			if spectator.name != nil {
				Draw_ColoredString(0, y*8, scratchf(1024, "%4d %4d &cF20s&cF50p&cF80e&c883c&cA85t&c668a&c55At&c33Bo&c22Dr&cFFF   %s", int32(-spectator.ping), int32(-spectator.time), *spectator.name), 0)
				y++
			}
		}
	}
}

func SB_Draw_Status_Bar() {
	var hint string

	if sb_ctrlr_count_in_session == 0 {
		hint = "Ctrl+R: refresh   R: update selected"
	} else {
		hint = "Ctrl+R: full rescan   R: update selected"
	}

	line := scratchf(512, "%-24s%s", sb_status_bar, hint)
	Draw_String(0, int(vid.conheight)-8, line)
}

func SB_Draw_Help() {
	i := 0

	lines := func(y0 int, texts ...string) {
		for _, text := range texts {
			Draw_String(8, y0+8*i, text)
			i++
		}
	}

	lines(0, "press arrow left/right to switch help screens", "esc to quit")
	if sb_active_help_window == 0 {
		lines(8, "general controls:",
			" 1->0, arrow left/right -  to switch tabs",
			" arrow up/down - to scroll, use shift/ctrl to modify jump length",
			" esc - will quit submenus, browser")
	} else if sb_active_help_window == 1 {
		lines(8, "server screen:",
			" ctrl + r - rescan all server",
			" r - rescan selected server",
			" enter - to join as player",
			" ctrl + enter - to join as spectator",
			" shift + enter - to join via qtv",
			" ctrl + f - start player search",
			" tab - to switch through sort mode")
	} else if sb_active_help_window == 2 {
		lines(8, "filter screen:",
			" insert - to add filter")
	} else if sb_active_help_window == 3 {
		lines(8, "insert filter screen:",
			" tab - to switch through entry boxes",
			" arrow up/down - to select compare type")
	}
}

func SB_Frame() {
	if qtv_connect_pending {
		if !QTVR_Waiting(qtvr) {
			proxy_stream, ok := QTVR_Get_Retval(qtvr)
			qtv_connect_pending = false
			if ok {
				Cbuf_AddText(va("alias f_qtv \"say .qtv %s\"; connect %s; echo qtv will buffer for 10 seconds please be patient", proxy_stream, sb_qtv_proxy.string))
			} else {
				Com_Printf("Sorry could not get a qtv reply.\n")
			}

			QTVR_Destroy(qtvr)
			qtvr = nil
		}

		// C destroys the retriever a second time when the reply and the
		// timeout fall in the same frame
		if qtvr != nil && float64(sb_qtv_connect_timeout.value)+qtv_connect_time < cls.realtime {
			Com_Printf("Sorry qtv lookup timed out.\n")
			qtv_connect_pending = false
			QTVR_Destroy(qtvr)
			qtvr = nil
		}
	}

	if key_dest != key_serverbrowser {
		sb_open = false
	}

	if !sb_open {
		return
	}

	if serverscanner != nil {
		ServerScanner_DoStuff(serverscanner)
	}

	if serverscanner != nil && sb_check_serverscanner {
		sss := ServerScanner_GetStatus(serverscanner)
		if sss == SSS_IDLE {
			SB_Set_Statusbar("All done.")
			sb_check_serverscanner = false
			if sb_full_refresh_pending && tab_active != nil && tab_active.server_count > 0 && tab_active.server_index != nil {
				tab_active.sb_position = 0
				current_selected_server = sb_qw_server[tab_active.server_index[0]]
				tab_active.changed = true
			}
			sb_full_refresh_pending = false
		} else if sss == SSS_SCANNING {
			SB_Set_Statusbar("Scanning servers...")
		} else if sss == SSS_PINGING {
			SB_Set_Statusbar("Pinging servers...")
		} else if sss == SSS_ERROR {
			SB_Set_Statusbar("Server scanner error.")
		}
	}

	if serverscanner != nil {
		if ServerScanner_DataUpdated(serverscanner) {
			if sb_qw_server != nil {
				ServerScanner_FreeServers(serverscanner, sb_qw_server)
			}
			sb_qw_server = ServerScanner_GetServers(serverscanner, &sb_qw_server_count)
			sb_server_count_width = 1
			for x := sb_qw_server_count / 10; x != 0; x /= 10 {
				sb_server_count_width++
			}
			SB_Update_Tabs()
		}
	}

	if !sb_open {
		return
	}

	SB_Draw_Background()
	if sb_active_window == SB_SERVER && serverscanner != nil {
		SB_Draw_Server()
	}
	if sb_active_window == SB_FILTER {
		SB_Draw_Filter()
	}
	if sb_active_window == SB_HELP {
		SB_Draw_Help()
	}
	SB_Draw_Status_Bar()
	sb_check_serverscanner = true
}

func SB_Quit() {
	if serverscanner != nil {
		ServerScanner_FreeServers(serverscanner, sb_qw_server)
		ServerScanner_Delete(serverscanner)
		serverscanner = nil
	}

	if qtvr != nil {
		QTVR_Destroy(qtvr)
		qtvr = nil
	}

	for friends != nil {
		friends = friends.next
	}

	for tab_first != nil {
		sb_del_tab(tab_first)
	}
}

func SB_List_Tabs_f() {
	for tab := tab_first; tab != nil; tab = tab.next {
		Com_Printf("%s\n", tab.name)
	}
}

func SB_Add_Tab_f() {
	if Cmd_Argc() != 2 {
		Com_Printf("Usage: %s [tab name]\n", Cmd_Argv(0))
	}

	sb_default_settings = false
	sb_add_tab(Cmd_Argv(1))
}

func SB_Del_Tab_f() {
	if Cmd_Argc() != 2 {
		Com_Printf("Usage: %s [tab name]\n", Cmd_Argv(0))
	}

	sb_del_tab_by_name(Cmd_Argv(1))
}

func SB_Add_Filter_f() {
	if Cmd_Argc() != 5 {
		Com_Printf("Usage: %s [tab name] [filter name] [filter operator] [value]\n", Cmd_Argv(0))
	}

	tab := tab_first
	for tab != nil {
		if tab.name == Cmd_Argv(1) {
			break
		}
		tab = tab.next
	}

	if tab == nil {
		Com_Printf("%s: tab %s not found.\n", Cmd_Argv(0), Cmd_Argv(1))
		return
	}

	filter := -1

	for i := 0; i < SB_FILTER_TYPE_MAX; i++ {
		if filter_types[i].name == Cmd_Argv(2) {
			filter = i
			break
		}
	}

	if filter == -1 {
		Com_Printf("%s: filter type %s not found.\n", Cmd_Argv(0), Cmd_Argv(2))
		Com_Printf("use sb_list_filter_types to get a list of available filters.\n")
		return
	}

	operator := -1

	var operators []string
	if filter_types[filter].type_ == 0 {
		operators = filter_num_operators
	} else {
		operators = filter_char_operators
	}

	for i := range operators {
		if operators[i] == Cmd_Argv(3) {
			operator = i
			break
		}
	}

	if operator == -1 {
		Com_Printf("%s: filter operator %s not found.\n", Cmd_Argv(0), Cmd_Argv(3))
		Com_Printf("use sb_list_filter_types to get a list of available filters and operators.\n")
		return
	}

	sb_default_settings = false

	SB_Add_Filter_To_Tab(tab, filter, operator, Cmd_Argv(4))
}

func SB_Write_Config() {
	file := va("%s/qw/%s.cfg", com_basedir, Cmd_Argv(1))

	f, err := os.Create(file)
	if err != nil {
		return
	}

	Dump_SB_Config(f)

	f.Close()
}

func SB_Set_Clipboard_f() {
	if Cmd_Argc() < 2 {
		Com_Printf("Usage: %s [clip bord text]\n", Cmd_Argv(0))
	}

	buf := ""
	for i := 1; i < Cmd_Argc(); i++ {
		buf = strlcat(buf, Cmd_Argv(i), 512)
		buf = strlcat(buf, " ", 512)
	}

	b := []byte(buf)
	for i := range b {
		b[i] = readablechars[b[i]] & 127
	}

	VID_SetClipboardText(string(b))
}

func SB_Update_Friend_Name_Length() {
	if friends == nil {
		friend_name_max_len = 0
	} else {
		for s := friends; s != nil; s = s.next {
			if i := len(s.name); i > friend_name_max_len {
				friend_name_max_len = i
			}
		}
	}
}

func SB_Add_Friend(name string) int {
	var s *sb_friend

	if friends != nil {
		s = friends
		for s != nil {
			if s.name == name {
				Com_Printf("A friend with the name \"%s\" already exists.\n", name)
				return 1
			}
			if s.next != nil {
				s = s.next
			} else {
				break
			}
		}

		s.next = &sb_friend{}

		s.next.prev = s

		s = s.next
	} else {
		s = &sb_friend{}
		friends = s
	}

	s.name = name
	SB_Update_Friend_Name_Length()
	return 0
}

func SB_Remove_Friend(name string) {
	if friends == nil {
		Com_Printf("no friends in the friend list\n")
		return
	}

	for s := friends; s != nil; s = s.next {
		if s.name == name {
			if s == friends {
				if s.next != nil {
					s.next.prev = nil
				}
				friends = s.next
			} else {
				if s.next != nil {
					s.next.prev = s.prev
				}
				if s.prev != nil {
					s.prev.next = s.next
				}
			}
			SB_Update_Friend_Name_Length()
			return
		}
	}
}

func SB_Add_Friend_f() {
	if Cmd_Argc() != 2 {
		Com_Printf("Usage: %s friend\n", Cmd_Argv(0))
		return
	}

	SB_Add_Friend(Cmd_Argv(1))
}

func SB_Remove_Friend_f() {
	if Cmd_Argc() != 2 {
		Com_Printf("Usage: %s friend\n", Cmd_Argv(0))
		return
	}

	SB_Remove_Friend(Cmd_Argv(1))
}

func SB_Init() {
	SB_Set_Statusbar("Loading...")
}

func Get_Tab_By_Name(name string) *tab_t {
	for tab := tab_first; tab != nil; tab = tab.next {
		if tab.name == name {
			return tab
		}
	}
	return nil
}

func SB_Tab_Layout_f() {
	if Cmd_Argc() < 3 {
		Com_Printf("Usage: %s tab_name [ping players map hostname]\n", Cmd_Argv(0))
		return
	}

	tab := Get_Tab_By_Name(Cmd_Argv(1))
	if tab == nil {
		Com_Printf("could not find a tab with the name \"%s\"\n", Cmd_Argv(1))
		return
	}

	for i := 2; i < Cmd_Argc(); i++ {
		if Cmd_Argv(i) != "ping" &&
			Cmd_Argv(i) != "players" &&
			Cmd_Argv(i) != "map" &&
			Cmd_Argv(i) != "hostname" {
			Com_Printf("%s is not a valid column type, valid types: ping players map hostname\n", Cmd_Argv(i))
			return
		}
	}

	c := Cmd_Argc() - 2
	tab.columns = c

	tab.column_types = make([]tab_column_type, c)

	for i := 0; i < c; i++ {
		switch Cmd_Argv(2 + i) {
		case "ping":
			tab.column_types[i].type_ = SBCT_PING
		case "players":
			tab.column_types[i].type_ = SBCT_PLAYERS
		case "map":
			tab.column_types[i].type_ = SBCT_MAP
		case "hostname":
			tab.column_types[i].type_ = SBCT_HOSTNAME
		}
	}
	update_tab(tab)
}

func SB_CvarInit() {
	Cmd_AddCommand("sb_activate", SB_Activate_f)
	Cmd_AddCommand("sb_list", SB_List_Tabs_f)
	Cmd_AddCommand("sb_tablayout", SB_Tab_Layout_f)
	Cmd_AddCommand("sb_add_tab", SB_Add_Tab_f)
	Cmd_AddCommand("sb_del_tab", SB_Del_Tab_f)
	Cmd_AddCommand("sb_add_filter", SB_Add_Filter_f)
	Cmd_AddCommand("sb_write_config", SB_Write_Config)
	Cmd_AddCommand("sb_set_clipboard", SB_Set_Clipboard_f)
	Cmd_AddCommand("sb_add_friend", SB_Add_Friend_f)
	Cmd_AddCommand("sb_remove_friend", SB_Remove_Friend_f)

	SB_AddMacros()

	Cvar_Register(&sb_masterserver)
	Cvar_Register(&sb_masterserver_http)
	Cvar_Register(&sb_player_drawing)
	Cvar_Register(&sb_refresh_on_activate)
	Cvar_Register(&sb_debug)
	Cvar_Register(&sb_bg_color)
	Cvar_Register(&sb_color_bg)
	Cvar_Register(&sb_color_bg_empty)
	Cvar_Register(&sb_color_bg_free)
	Cvar_Register(&sb_color_bg_specable)
	Cvar_Register(&sb_color_bg_full)
	Cvar_Register(&sb_qtv_proxy)
	Cvar_Register(&sb_qtv_lookup)
	Cvar_Register(&sb_qtv_connect_timeout)
	Cvar_Register(&sb_highlight_sort_column)
	Cvar_Register(&sb_highlight_sort_column_color)
	Cvar_Register(&sb_highlight_sort_column_alpha)
}

func Dump_SB_Config(f io.Writer) {
	if f == nil {
		return
	}

	if sb_default_settings {
		return
	}

	for tab := tab_first; tab != nil; tab = tab.next {
		fmt.Fprintf(f, "sb_add_tab %s\n", tab.name)

		if tab.name != "friends" {
			for _, filter := range tab.filters {
				if filter_types[filter.key].type_ == 0 {
					fmt.Fprintf(f, "sb_add_filter %s %s %s %f\n", tab.name, filter_types[filter.key].name, filter_num_operators[filter.type_], filter.fvalue)
				} else {
					fmt.Fprintf(f, "sb_add_filter %s %s %s %s\n", tab.name, filter_types[filter.key].name, filter_char_operators[filter.type_], filter.value)
				}
			}
		}

		fmt.Fprintf(f, "sb_tablayout %s", tab.name)
		for i := 0; i < tab.columns; i++ {
			fmt.Fprintf(f, " %s", column_names[tab.column_types[i].type_])
		}
		fmt.Fprintf(f, "\n")
	}
}

func SB_Macro_Ip() string {
	if current_selected_server != nil {
		return NET_AdrToString(&current_selected_server.addr)
	}
	return "none"
}

func SB_Macro_Hostname() string {
	if current_selected_server != nil {
		if current_selected_server.hostname != nil {
			return *current_selected_server.hostname
		}
		return "none"
	}
	return "none"
}

func SB_Macro_Map() string {
	if current_selected_server != nil {
		if current_selected_server.map_ != nil {
			return *current_selected_server.map_
		}
		return "none"
	}
	return "none"
}

func SB_Macro_Player() string {
	if current_selected_server != nil {
		return snprintf(512, "%d", int32(current_selected_server.numplayers))
	}
	return snprintf(512, "%d", -1)
}

func SB_Macro_Max_Player() string {
	if current_selected_server != nil {
		return snprintf(512, "%d", int32(current_selected_server.maxclients))
	}
	return snprintf(512, "%d", -1)
}

func SB_Macro_Ping() string {
	if current_selected_server != nil {
		return snprintf(512, "%d", int32(current_selected_server.pingtime/1000))
	}
	return snprintf(512, "%d", -1)
}

func SB_Macro_Player_Names() string {
	if current_selected_server != nil {
		if current_selected_server.numplayers > 0 {
			buf := ""
			for i := uint32(0); i < current_selected_server.numplayers; i++ {
				player := &current_selected_server.players[i]
				if i != 0 {
					buf = strlcat(buf, " - ", 512)
				}
				if player.name != nil {
					buf = strlcat(buf, *player.name, 512)
				}
			}
			return buf
		}
		return "none"
	}
	return "none"
}

func SB_AddMacros() {
	Cmd_AddMacro("sb_player_names", SB_Macro_Player_Names)
	Cmd_AddMacro("sb_max_player", SB_Macro_Max_Player)
	Cmd_AddMacro("sb_hostname", SB_Macro_Hostname)
	Cmd_AddMacro("sb_player", SB_Macro_Player)
	Cmd_AddMacro("sb_ping", SB_Macro_Ping)
	Cmd_AddMacro("sb_map", SB_Macro_Map)
	Cmd_AddMacro("sb_ip", SB_Macro_Ip)
}

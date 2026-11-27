// Copyright (C) 2001-2002		  A	Nourai
// Copyright (C) 2005-2007, 2009 Mark Olsen
//
// This program is	free software; you can redistribute	it and/or
// modify it under	the	terms of the GNU General Public	License
// as published by	the	Free Software Foundation; either version 2
// of the License,	or (at your	option)	any	later version.
//
// This program is	distributed	in the hope	that it	will be	useful,
// but	WITHOUT	ANY	WARRANTY; without even the implied warranty	of
// MERCHANTABILITY	or FITNESS FOR A PARTICULAR	PURPOSE.
//
// See	the	included (GNU.txt) GNU General Public License for more details.
//
// You	should have	received a copy	of the GNU General Public License
// along with this	program; if	not, write to the Free Software
// Foundation,	Inc., 59 Temple	Place -	Suite 330, Boston, MA  02111-1307, USA.

package engine

// config_manager.c -- cfg_save, cfg_load and cfg_reset

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
)

var cfg_save_unchanged = cvar_t{name: "cfg_save_unchanged", string: "0"}
var cfg_save_userinfo = cvar_t{name: "cfg_save_userinfo", string: "2"}
var cfg_legacy_exec = cvar_t{name: "cfg_legacy_exec", string: "1"}
var cfg_legacy_write = cvar_t{name: "cfg_legacy_write", string: "0"}
var cfg_save_cvars = cvar_t{name: "cfg_save_cvars", string: "1"}
var cfg_save_aliases = cvar_t{name: "cfg_save_aliases", string: "1"}
var cfg_save_cmds = cvar_t{name: "cfg_save_cmds", string: "1"}
var cfg_save_binds = cvar_t{name: "cfg_save_binds", string: "1"}
var cfg_save_sysinfo = cvar_t{name: "cfg_save_sysinfo", string: "0"}
var cfg_save_cmdline = cvar_t{name: "cfg_save_cmdline", string: "1"}
var cfg_backup = cvar_t{name: "cfg_backup", string: "0"}

/************************************ DUMP FUNCTIONS ************************************/

const BIND_ALIGN_COL = 20

// a binding that is unset prints as C's "(null)"
func bindingText(i int) string {
	if keybindings[i] == nil {
		return "(null)"
	}
	return *keybindings[i]
}

func DumpBindings(f io.Writer) {
	var i, leftright int
	var str string
	var stringlen int
	printed := false

	for i = 0; i < 256; i++ {
		leftright = b2i(Key_IsLeftRightSameBind(i))
		if keybindings[i] != nil || leftright != 0 {
			printed = true
			str = Key_KeynumToString(i)
			stringlen = BIND_ALIGN_COL - len(str) - 6
			if i == ';' {
				fmt.Fprintf(f, "bind  \";\"%*s\"%s\"\n", stringlen, "", bindingText(i))
			} else {
				idx := i
				if leftright != 0 {
					idx = i + 1
				}
				fmt.Fprintf(f, "bind  %s%*s\"%s\"\n", str, stringlen, "", bindingText(idx))
			}
			if leftright != 0 {
				i += 2
			}
		}
	}

	if !printed {
		fmt.Fprintf(f, "//no bindings\n")
	}
}

const CONFIG_MAX_COL = 60

func cvarSkipsConfig(v *cvar_t) bool {
	return v.flags&(CVAR_USER_CREATED|CVAR_ROM|CVAR_INIT) != 0 ||
		(v.group != nil && (v.group.name == CVAR_GROUP_NO_GROUP || v.group.name == CVAR_GROUP_SERVERINFO))
}

func cvarIsUserinfoKey(v *cvar_t) bool {
	return v.name == "team" || v.name == "skin" ||
		v.name == "spectator" || v.name == "name" ||
		v.name == "topcolor" || v.name == "bottomcolor"
}

func sortCvars(vars []*cvar_t) {
	sort.Slice(vars, func(a, b int) bool { return Cvar_CvarCompare(vars[a], vars[b]) < 0 })
}

func DumpVariables(f io.Writer) {
	var v *cvar_t
	var sorted_vars [256]*cvar_t
	var group *cvar_group_t
	var count, i, col_size int
	skip_userinfo := false

	col_size = 0
	for v = cvar_vars; v != nil; v = v.next {
		if !cvarSkipsConfig(v) {
			col_size = max(col_size, len(v.name))
		}
	}
	col_size = min(col_size+2, CONFIG_MAX_COL)

	if cfg_save_unchanged.value != 0 {
		fmt.Fprintf(f, "//All variables (even those with default values) are listed below.\n")
		fmt.Fprintf(f, "//You can use \"cfg_save_unchanged 0\" to save only changed variables.\n\n")
	} else {
		fmt.Fprintf(f, "//Only variables with non-default values are listed below.\n")
		fmt.Fprintf(f, "//You can use \"cfg_save_unchanged 1\" to save all variables.\n\n")
	}

	for group = cvar_groups; group != nil; group = group.next {
		if group.name == CVAR_GROUP_NO_GROUP ||
			group.name == CVAR_GROUP_SERVERINFO ||
			(cfg_save_userinfo.value == 0 && group.name == CVAR_GROUP_USERINFO) {
			continue
		}

		skip_userinfo = cfg_save_userinfo.value == 1 && group.name == CVAR_GROUP_USERINFO

		for count, v = 0, group.head; v != nil && count < 256; v = v.next_in_group {
			if skip_userinfo && cvarIsUserinfoKey(v) {
				continue
			}

			if v.flags&(CVAR_USER_CREATED|CVAR_ROM|CVAR_INIT) == 0 {
				if cfg_save_unchanged.value != 0 || v.string != v.defaultvalue {
					sorted_vars[count] = v
					count++
				}
			}
		}

		if count == 0 {
			continue
		}

		if group.name != CVAR_GROUP_ITEM_NAMES &&
			group.name != CVAR_GROUP_ITEM_NEED &&
			group.name != CVAR_GROUP_USERINFO &&
			group.name != CVAR_GROUP_SKIN {
			sortCvars(sorted_vars[:count])
		}

		fmt.Fprintf(f, "//%s\n", group.name)
		for i = 0; i < count; i++ {
			v = sorted_vars[i]
			if cfg_save_unchanged.value != 0 || v.string != v.defaultvalue {
				fmt.Fprintf(f, "%s%*s\"%s\"\n", v.name, col_size-len(v.name), "", v.string)
			}
		}
		fmt.Fprintf(f, "\n")
	}

	for count, v = 0, cvar_vars; v != nil && count < 256; v = v.next {
		if v.group == nil && v.flags&(CVAR_USER_CREATED|CVAR_ROM|CVAR_INIT) == 0 {
			if cfg_save_unchanged.value != 0 || v.string != v.defaultvalue {
				sorted_vars[count] = v
				count++
			}
		}
	}

	if count != 0 {
		sortCvars(sorted_vars[:count])

		fmt.Fprintf(f, "//Unsorted Variables\n")
		for i = 0; i < count; i++ {
			v = sorted_vars[i]
			fmt.Fprintf(f, "%s%*s\"%s\"\n", v.name, col_size-len(v.name), "", v.string)
		}
		fmt.Fprintf(f, "\n")
	}

	col_size = col_size - 2
	for count, v = 0, cvar_vars; v != nil && count < 256; v = v.next {
		if v.flags&CVAR_USER_CREATED != 0 {
			sorted_vars[count] = v
			count++
			col_size = max(col_size, len(v.name))
		}
	}

	if count == 0 {
		return
	}

	col_size = min(col_size+2, CONFIG_MAX_COL)

	sortCvars(sorted_vars[:count])

	fmt.Fprintf(f, "//User Created Variables\n")
	for i = 0; i < count; i++ {
		v = sorted_vars[i]
		set := "set "
		if v.flags&CVAR_USER_ARCHIVE != 0 {
			set = "seta"
		}
		fmt.Fprintf(f, "%s %s%*s\"%s\"\n", set, v.name, col_size-len(v.name)-5, "", v.string)
	}
}

const MAX_ALIGN_COL = 60

func dumpAlias(f io.Writer, a *cmd_alias_t, maxlen int) {
	fmt.Fprintf(f, "alias %s%*s\"%s\"\n", a.name, maxlen+3-len(a.name), "", a.value)
}

func DumpAliases(f io.Writer) {
	var maxlen, i, j, count, lonely_count, minus_index, minus_count int
	var b, a *cmd_alias_t
	var sorted_aliases, lonely_pluses [2048]*cmd_alias_t
	var partner, printed bool

	count = 0
	maxlen = 0
	for a = cmd_alias; count < len(sorted_aliases) && a != nil; a = a.next {
		if a.flags&(ALIAS_SERVER|ALIAS_TEMP) == 0 {
			maxlen = max(maxlen, len(a.name))
			count++
		}
	}

	if count == 0 {
		fmt.Fprintf(f, "//no aliases\n")
		return
	}

	for i, a = 0, cmd_alias; i < count; a = a.next {
		if a.flags&(ALIAS_SERVER|ALIAS_TEMP) == 0 {
			sorted_aliases[i] = a
			i++
		}
	}

	sort.Slice(sorted_aliases[:count], func(x, y int) bool { return Cmd_AliasCompare(sorted_aliases[x], sorted_aliases[y]) < 0 })

	minus_index = -1
	minus_count = 0
	for i = 0; i < count; i++ {
		a = sorted_aliases[i]
		if at(a.name, 0) == '-' {
			if minus_index == -1 {
				minus_index = i
			}
			minus_count++
		} else if at(a.name, 0) != '+' {
			break
		}
	}

	printed = false
	lonely_count = 0
	for i = 0; i < count; i++ {
		a = sorted_aliases[i]
		if at(a.name, 0) != '+' {
			break
		}

		partner = false
		for j = minus_index; j >= 0 && j < minus_index+minus_count; j++ {
			b = sorted_aliases[j]
			if Q_strcasecmp(b.name[1:], a.name[1:]) == 0 {
				dumpAlias(f, a, maxlen)
				dumpAlias(f, b, maxlen)
				printed = true
				partner = true
				break
			}
		}

		if !partner {
			lonely_pluses[lonely_count] = a
			lonely_count++
		}
	}

	for i = 0; i < lonely_count; i++ {
		a = lonely_pluses[i]
		dumpAlias(f, a, maxlen)
		printed = true
	}

	for i = minus_index; i >= 0 && i < minus_index+minus_count; i++ {
		a = sorted_aliases[i]
		partner = false
		for j = 0; j < minus_index; j++ {
			b = sorted_aliases[j]
			if Q_strcasecmp(b.name[1:], a.name[1:]) == 0 {
				partner = true
				break
			}
		}

		if !partner {
			dumpAlias(f, a, maxlen)
			printed = true
		}
	}

	if minus_index == -1 {
		i = 0
	} else {
		i = minus_index + minus_count
	}
	for ; i < count; i++ {
		a = sorted_aliases[i]
		if minus_index != -1 || at(a.name, 0) != '+' {
			if printed {
				fmt.Fprintf(f, "\n")
			}
			printed = false
			dumpAlias(f, a, maxlen)
		}
	}
}

func DumpPlusCommand(f io.Writer, b *kbutton_t, name string) {
	if b.state&1 != 0 && b.down[0] < 0 {
		fmt.Fprintf(f, "+%s\n", name)
	} else {
		fmt.Fprintf(f, "-%s\n", name)
	}
}

func DumpPlusCommands(f io.Writer) {
	DumpPlusCommand(f, &in_up, "moveup")
	DumpPlusCommand(f, &in_down, "movedown")
	DumpPlusCommand(f, &in_forward, "forward")
	DumpPlusCommand(f, &in_back, "back")
	DumpPlusCommand(f, &in_moveleft, "moveleft")
	DumpPlusCommand(f, &in_moveright, "moveright")
	DumpPlusCommand(f, &in_attack, "attack")
	DumpPlusCommand(f, &in_use, "use")
	DumpPlusCommand(f, &in_jump, "jump")
	if sb_showscores {
		fmt.Fprintf(f, "+showscores\n")
	} else {
		fmt.Fprintf(f, "-showscores\n")
	}
	if sb_showteamscores {
		fmt.Fprintf(f, "+showteamscores\n")
	} else {
		fmt.Fprintf(f, "-showteamscores\n")
	}
}

func DumpColorForcing(f io.Writer, name string, topcolor, bottomcolor int) {
	if topcolor < 0 {
		if cfg_save_unchanged.value != 0 {
			fmt.Fprintf(f, "%s%*soff\n", name, 13-len(name), "")
		}
	} else {
		if topcolor != bottomcolor {
			fmt.Fprintf(f, "%s%*s%d %d\n", name, 13-len(name), "", topcolor, bottomcolor)
		} else {
			fmt.Fprintf(f, "%s%*s%d\n", name, 13-len(name), "", topcolor)
		}
	}
}

func DumpTeamplay(f io.Writer) {
	DumpColorForcing(f, "teamcolor", cl_teamtopcolor, cl_teambottomcolor)
	DumpColorForcing(f, "enemycolor", cl_enemytopcolor, cl_enemybottomcolor)
	fmt.Fprintf(f, "\n")
	DumpFlagCommands(f)
	fmt.Fprintf(f, "\n")
	TP_DumpMsgFilters(f)
	fmt.Fprintf(f, "\n")
	TP_DumpTriggers(f)
}

func DumpMisc(f io.Writer) {
	DumpMapGroups(f)
	fmt.Fprintf(f, "\n")

	if cl.teamfortress {
		if Q_strcasecmp(Info_ValueForKey(cls.userinfo, "ec"), "on") == 0 ||
			Q_strcasecmp(Info_ValueForKey(cls.userinfo, "exec_class"), "on") == 0 {
			fmt.Fprintf(f, "setinfo ec on\n")
		}
		if Q_strcasecmp(Info_ValueForKey(cls.userinfo, "em"), "on") == 0 ||
			Q_strcasecmp(Info_ValueForKey(cls.userinfo, "exec_map"), "on") == 0 {
			fmt.Fprintf(f, "setinfo em on\n")
		}
	}
}

func DumpCmdLine(f io.Writer) {
	fmt.Fprintf(f, "// %s\n", cl_cmdline.string)
}

/************************************ RESET FUNCTIONS ************************************/

func ResetVariables(cvar_flags int, userinfo bool) {
	var v *cvar_t
	check_userinfos := false

	if userinfo {
		if cfg_save_userinfo.value == 0 {
			cvar_flags |= CVAR_USERINFO
		} else if userinfo && cfg_save_userinfo.value == 1 {
			check_userinfos = true
		}
	}

	for v = cvar_vars; v != nil; v = v.next {
		if !(v.flags&(cvar_flags|CVAR_ROM|CVAR_INIT|CVAR_USER_CREATED) != 0 ||
			(v.group != nil && v.group.name == CVAR_GROUP_NO_GROUP)) {
			if check_userinfos && cvarIsUserinfoKey(v) {
				continue
			}
			Cvar_ResetVar(v)
		}
	}
}

func DeleteUserAliases() {
	var a *cmd_alias_t

	for a = cmd_alias; a != nil; a = a.next {
		if a.flags&(ALIAS_SERVER|ALIAS_TEMP) == 0 {
			Cmd_DeleteAlias(a.name)
		}
	}
}

func DeleteUserVariables() {
	var v *cvar_t

	for v = cvar_vars; v != nil; v = v.next {
		if v.flags&CVAR_USER_CREATED != 0 {
			Cvar_Delete(v.name)
		}
	}
}

func ResetPlusCommands() {
	Cbuf_AddText("-moveup;-movedown\n")
	Cbuf_AddText("-forward;-back\n")
	Cbuf_AddText("-moveleft;-moveright\n")
	Cbuf_AddText("-attack\n")
	Cbuf_AddText("-use\n")
	Cbuf_AddText("-jump\n")
	Cbuf_AddText("-showscores\n")
	Cbuf_AddText("-showteamscores\n")
}

func ResetTeamplayCommands() {
	Cbuf_AddText("enemycolor off\nteamcolor	off\n")
	Cbuf_AddText("filter clear\n")
	TP_ResetAllTriggers()
	Cbuf_AddText("tp_took default\ntp_pickup default\ntp_point default\n")
}

func ResetMiscCommands() {
	Cbuf_AddText("mapgroup clear\n")
	cls.userinfo = Info_RemoveKey(cls.userinfo, "ec")
	cls.userinfo = Info_RemoveKey(cls.userinfo, "exec_class")
	cls.userinfo = Info_RemoveKey(cls.userinfo, "em")
	cls.userinfo = Info_RemoveKey(cls.userinfo, "exec_map")
}

/************************************ PRINTING FUNCTIONS ************************************/

const CONFIG_WIDTH = 100

func Config_PrintBorder(f io.Writer) {
	buf := make([]byte, CONFIG_WIDTH)
	for i := range buf {
		buf[i] = '/'
	}
	fmt.Fprintf(f, "%s\n", buf)
}

func Config_PrintLine(f io.Writer, title string, width int) {
	buf := make([]byte, CONFIG_WIDTH)
	var title_len, i int

	width = bound(1, width, CONFIG_WIDTH<<3)
	for i = 0; i < width; i++ {
		buf[i] = '/'
		buf[CONFIG_WIDTH-1-i] = '/'
	}
	for i = width; i < CONFIG_WIDTH-width; i++ {
		buf[i] = ' '
	}

	if len(title) > CONFIG_WIDTH-(2*width+4) {
		title = "Config_PrintLine : TITLE TOO BIG"
	}
	title_len = len(title)
	copy(buf[width+((CONFIG_WIDTH-title_len-2*width)>>1):], title)
	fmt.Fprintf(f, "%s\n", buf)
}

func Config_PrintHeading(f io.Writer, title string) {
	Config_PrintBorder(f)
	Config_PrintLine(f, "", 2)
	Config_PrintLine(f, title, 2)
	Config_PrintLine(f, "", 2)
	Config_PrintBorder(f)
	fmt.Fprintf(f, "\n\n")
}

func Config_PrintPreamble(f io.Writer) {
	Config_PrintBorder(f)
	Config_PrintBorder(f)
	Config_PrintLine(f, "", 3)
	Config_PrintLine(f, "", 3)
	Config_PrintLine(f, "C L A S S I C Q   C O N F I G U R A T I O N", 3)
	Config_PrintLine(f, "", 3)
	Config_PrintLine(f, "https://classicq.github.io/", 3)
	Config_PrintLine(f, "", 3)
	Config_PrintLine(f, "", 3)
	Config_PrintBorder(f)
	Config_PrintBorder(f)
	fmt.Fprintf(f, "\n\n\n")
}

/************************************ MAIN FUCTIONS	************************************/

func ResetConfigs(resetall bool) {
	ResetVariables(CVAR_SERVERINFO, !resetall)

	DeleteUserAliases()

	DeleteUserVariables()

	Cbuf_AddText("unbindall\n")

	ResetPlusCommands()
	ResetTeamplayCommands()
	ResetMiscCommands()

	Cbuf_AddText("cl_warncmd 0\n")
	Cbuf_AddText("exec default.cfg\n")
	if FS_FileExists("autoexec.cfg") != 0 {
		Cbuf_AddText("exec autoexec.cfg\n")
	}
	Cbuf_AddText("cl_warncmd 1\n")
}

func DumpConfig(name string) {
	var fh *os.File
	var err error
	var outfile string
	newlines := "\n\n\n\n"

	outfile = va("%s/classicq/configs/%s", com_basedir, name)

	if fh, err = os.Create(outfile); err != nil {
		FS_CreatePath(outfile)
		if fh, err = os.Create(outfile); err != nil {
			Com_ErrorPrintf("Couldn't write \"%s\".\n", name)
			return
		}
	}
	f := bufio.NewWriter(fh)

	Config_PrintPreamble(f)

	if cfg_save_cmdline.value != 0 {
		Config_PrintHeading(f, "C O M M A N D   L I N E")
		DumpCmdLine(f)
		fmt.Fprint(f, newlines)
	}

	if cfg_save_cvars.value != 0 {
		Config_PrintHeading(f, "V A R I A B L E S")
		DumpVariables(f)
		fmt.Fprint(f, newlines)
	}

	if cfg_save_aliases.value != 0 {
		Config_PrintHeading(f, "A L I A S E S")
		DumpAliases(f)
		fmt.Fprint(f, newlines)
	}

	if cfg_save_cmds.value != 0 {
		Config_PrintHeading(f, "T E A M P L A Y   C O M M A N D S")
		DumpTeamplay(f)
		fmt.Fprint(f, newlines)

		Config_PrintHeading(f, "M I S C E L L A N E O U S   C O M M A N D S")
		DumpMisc(f)
		fmt.Fprint(f, newlines)

		Config_PrintHeading(f, "P L U S   C O M M A N D S")
		DumpPlusCommands(f)
		fmt.Fprint(f, newlines)
	}

	if cfg_save_binds.value != 0 {
		Config_PrintHeading(f, "K E Y   B I N D I N G S")
		DumpBindings(f)
	}

	f.Flush()
	fh.Close()
}

/************************************ API ************************************/

var saveconfig_call_count = 0

func SaveConfig_f() {
	var filename, filename_ext, backupname_ext string
	var namebuf string

	if Cmd_Argc() > 2 {
		Com_Printf("Usage: %s <filename>\n", Cmd_Argv(0))
		return
	}

	if Cmd_Argc() == 2 {
		namebuf = strlcpy(COM_SkipPath(Cmd_Argv(1)), MAX_OSPATH)
		namebuf = COM_ForceExtension(namebuf, ".cfg")
		filename = namebuf
		saveconfig_call_count = 0
	} else {
		if saveconfig_call_count == 0 {
			Com_Printf("please call %s again to save your current setup as default\n", Cmd_Argv(0))
			saveconfig_call_count++
			return
		}
		filename = "default.cfg"
		saveconfig_call_count = 0
	}

	if cfg_backup.value != 0 {
		filename_ext = va("%s/classicq/configs/%s", com_basedir, filename)
		if fh, err := os.Open(filename_ext); err == nil {
			fh.Close()
			backupname_ext = filename_ext + ".bak"
			if fh, err := os.Open(backupname_ext); err == nil {
				fh.Close()
				os.Remove(backupname_ext)
			}
			Sys_rename(filename_ext, backupname_ext)
		}
	}

	DumpConfig(filename)
	Com_Printf("Saving configuration to %s\n", filename)
}

func ResetConfigs_f() {
	if Cmd_Argc() != 1 {
		Com_Printf("Usage: %s \n", Cmd_Argv(0))
		return
	}

	Com_Printf("Resetting configuration to default state...\n")
	ResetConfigs(true)
}

func LoadConfig_f() {
	var filename string
	var namebuf string

	if Cmd_Argc() != 2 {
		Com_Printf("Usage: %s <filename>\n", Cmd_Argv(0))
		return
	}

	namebuf = strlcpy(COM_SkipPath(Cmd_Argv(1)), MAX_OSPATH)
	namebuf = COM_ForceExtension(namebuf, ".cfg")
	filename = namebuf

	fh, err := os.Open(va("%s/classicq/configs/%s", com_basedir, filename))
	if err != nil {
		Com_Printf("Couldn't load config %s\n", filename)
		return
	}
	fh.Close()

	Con_Suppress()
	ResetConfigs(false)
	Con_Unsuppress()

	Com_Printf("Loading config %s ...\n", filename)

	Cbuf_AddText("cl_warncmd 0\n")
	Cbuf_AddText(va("exec configs/%s\n", filename))
	Cbuf_AddText("cl_warncmd 1\n")
}

func ConfigManager_CvarInit() {
	Cmd_AddCommand("cfg_save", SaveConfig_f)
	Cmd_AddCommand("cfg_load", LoadConfig_f)
	Cmd_AddCommand("cfg_reset", ResetConfigs_f)

	Cvar_SetCurrentGroup(CVAR_GROUP_CONFIG)
	Cvar_Register(&cfg_save_unchanged)
	Cvar_Register(&cfg_save_userinfo)
	Cvar_Register(&cfg_legacy_exec)
	Cvar_Register(&cfg_legacy_write)
	Cvar_Register(&cfg_save_cvars)
	Cvar_Register(&cfg_save_aliases)
	Cvar_Register(&cfg_save_cmds)
	Cvar_Register(&cfg_save_binds)
	Cvar_Register(&cfg_save_cmdline)
	Cvar_Register(&cfg_save_sysinfo)
	Cvar_Register(&cfg_backup)

	Cvar_ResetCurrentGroup()
}

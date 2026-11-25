package engine

import (
	"fmt"
	"io"
	"slices"
	"strings"
)

// cmd.h -- Command buffer and command execution

const MAXCMDBUF = 1 << 18

type cbuf_t struct {
	text_buf    [MAXCMDBUF]byte
	text_start  int
	text_end    int
	wait        bool
	runAwayLoop int
}

type xcommand_t = func()

type cmd_function_t struct {
	hash_next *cmd_function_t
	next      *cmd_function_t
	name      string
	function  xcommand_t
}

const MAX_ALIAS_NAME = 32

const (
	ALIAS_ARCHIVE = 1
	ALIAS_SERVER  = 2
	ALIAS_TEMP    = 4
)

type cmd_alias_t struct {
	hash_next *cmd_alias_t
	next      *cmd_alias_t
	name      string
	value     string
	flags     int
}

// cmd.c

var can_execute_functions bool

var cl_warncmd = cvar_t{name: "cl_warncmd", string: "0"}

var (
	cbuf_main            *cbuf_t
	cbuf_svc             *cbuf_t
	cbuf_safe            *cbuf_t
	cbuf_formatted_comms *cbuf_t
	cbuf_cmdsave         *cbuf_t
	cbuf_current         *cbuf_t
)

//=============================================================================

// Causes execution of the remainder of the command buffer to be delayed until
// next frame. This allows commands like:
// bind g "impulse 5 ; +attack ; wait ; -attack ; impulse 2"
func Cmd_Wait_f() {
	if cbuf_current != nil {
		cbuf_current.wait = true
	}
}

/*
=============================================================================
						COMMAND BUFFER
=============================================================================
*/

func Cbuf_AddText(text string) {
	Cbuf_AddTextEx(cbuf_main, text)
}

func Cbuf_InsertText(text string) {
	Cbuf_InsertTextEx(cbuf_main, text)
}

func Cbuf_Execute() {
	// drain startup commands stranded by a wait during the one-shot replay
	if can_execute_functions {
		Cbuf_ExecuteEx(cbuf_cmdsave)
	}
	Cbuf_ExecuteEx(cbuf_main)
	Cbuf_ExecuteEx(cbuf_safe)
	Cbuf_ExecuteEx(cbuf_formatted_comms)
}

func newCbuf() *cbuf_t {
	return &cbuf_t{text_start: MAXCMDBUF >> 1, text_end: MAXCMDBUF >> 1}
}

func Cbuf_Init() {
	cbuf_main = newCbuf()
	cbuf_svc = newCbuf()
	cbuf_safe = newCbuf()
	cbuf_formatted_comms = newCbuf()
	cbuf_cmdsave = newCbuf()
}

func Cbuf_Shutdown() {
	cbuf_main = nil
	cbuf_svc = nil
	cbuf_safe = nil
	cbuf_formatted_comms = nil
	cbuf_cmdsave = nil
}

// Adds command text at the end of the buffer
func Cbuf_AddTextEx(cbuf *cbuf_t, text string) {
	n := len(text)

	if cbuf.text_end+n <= MAXCMDBUF {
		copy(cbuf.text_buf[cbuf.text_end:], text)
		cbuf.text_end += n
		return
	}

	new_bufsize := cbuf.text_end - cbuf.text_start + n
	if new_bufsize > MAXCMDBUF {
		Com_Printf("Cbuf_AddText: overflow\n")
		return
	}

	// Calculate optimal position of text in buffer
	new_start := (MAXCMDBUF - new_bufsize) >> 1

	copy(cbuf.text_buf[new_start:], cbuf.text_buf[cbuf.text_start:cbuf.text_end])
	copy(cbuf.text_buf[new_start+cbuf.text_end-cbuf.text_start:], text)
	cbuf.text_start = new_start
	cbuf.text_end = cbuf.text_start + new_bufsize
}

// Adds command text at the beginning of the buffer
func Cbuf_InsertTextEx(cbuf *cbuf_t, text string) {
	n := len(text)

	if n <= cbuf.text_start {
		copy(cbuf.text_buf[cbuf.text_start-n:], text)
		cbuf.text_start -= n
		return
	}

	new_bufsize := cbuf.text_end - cbuf.text_start + n
	if new_bufsize > MAXCMDBUF {
		Com_Printf("Cbuf_InsertText: overflow\n")
		return
	}

	// Calculate optimal position of text in buffer
	new_start := (MAXCMDBUF - new_bufsize) >> 1

	copy(cbuf.text_buf[new_start+n:], cbuf.text_buf[cbuf.text_start:cbuf.text_end])
	copy(cbuf.text_buf[new_start:], text)
	cbuf.text_start = new_start
	cbuf.text_end = cbuf.text_start + new_bufsize
}

const MAX_RUNAWAYLOOP = 1000

func Cbuf_ExecuteEx(cbuf *cbuf_t) {
	for cbuf.text_end > cbuf.text_start {
		// find a \n or ; line break
		text := cbuf.text_buf[cbuf.text_start:cbuf.text_end]
		cursize := len(text)

		comment, quotes := false, false
		i := 0
		for ; i < cursize; i++ {
			if text[i] == '\n' {
				break
			}
			if text[i] == '"' {
				quotes = !quotes
				continue
			}
			if comment || quotes {
				continue
			}
			if text[i] == '/' && i+1 < cursize && text[i+1] == '/' {
				comment = true
			} else if text[i] == ';' {
				break
			}
		}

		// don't execute lines without ending \n; this fixes problems with
		// partially stuffed aliases not being executed properly
		if cbuf == cbuf_svc && i == cursize {
			break
		}

		// Copy text to line, skipping carriage return chars
		j := min(i, 1400-1)
		line := make([]byte, 0, j)
		for k := 0; k < j; k++ {
			if text[k] != '\r' {
				line = append(line, text[k])
			}
		}

		// delete the text from the command buffer and move remaining commands down
		// This is necessary because commands (exec, alias) can insert data at the
		// beginning of the text buffer
		if i == cursize {
			cbuf.text_start = MAXCMDBUF >> 1
			cbuf.text_end = MAXCMDBUF >> 1
		} else {
			i++
			cbuf.text_start += i
		}

		cursize = cbuf.text_end - cbuf.text_start
		Cmd_ExecuteStringEx(cbuf, string(line)) // execute the command line

		if cbuf.text_end-cbuf.text_start > cursize {
			cbuf.runAwayLoop++
		}

		if cbuf.runAwayLoop > MAX_RUNAWAYLOOP {
			Com_Printf("\x02" + "A recursive alias has caused an infinite loop.")
			Com_Printf("\x02" + " Clearing execution buffer to prevent lockup.\n")
			cbuf.text_start = MAXCMDBUF >> 1
			cbuf.text_end = MAXCMDBUF >> 1
			cbuf.runAwayLoop = 0
		}

		if cbuf.wait {
			// skip out while text still remains in buffer, leaving it for next frame
			cbuf.wait = false
			cbuf.runAwayLoop += Q_rint(0.5 * cls.frametime * MAX_RUNAWAYLOOP)
			return
		}
	}

	cbuf.runAwayLoop = 0
}

/*
==============================================================================
						SCRIPT COMMANDS
==============================================================================
*/

func Cmd_ParseLegacyCmdLineCmds() {
	for i := 1; i < com_argc; i++ {
		if com_argv[i] == "-width" && i+1 < com_argc {
			Cvar_Set(&vid_width, com_argv[i+1])
		} else if com_argv[i] == "-height" && i+1 < com_argc {
			Cvar_Set(&vid_height, com_argv[i+1])
		} else if com_argv[i] == "-conwidth" && i+1 < com_argc {
			Cvar_Set(&vid_conwidth, com_argv[i+1])
		} else if com_argv[i] == "-conheight" && i+1 < com_argc {
			Cvar_Set(&vid_conheight, com_argv[i+1])
		} else if com_argv[i] == "-fullscreen" {
			Cvar_Set(&vid_fullscreen, "1")
		} else if com_argv[i] == "-window" {
			Cvar_Set(&vid_fullscreen, "0")
		} else if com_argv[i] == "-nosound" {
			Cvar_Set(&s_nosound, "1")
		}
	}
}

/*
Adds command line parameters as script statements
Commands lead with a +, and continue until a - or another +
quake +prog jctest.qp +cmd amlev1
quake -nosound +cmd amlev1
*/
func Cmd_StuffCmds_f() {
	// build the combined string to parse from
	if com_argc <= 1 {
		return
	}
	text := strings.Join(com_argv[1:com_argc], " ")

	// pull out the commands
	i := 0
	for at(text, i) != 0 {
		if text[i] == '+' {
			var token []byte
			for i++; at(text, i) != 0 && (text[i] != ' ' || (at(text, i+1) != '-' && at(text, i+1) != '+')); i++ {
				token = append(token, text[i])
			}
			token = append(token, '\n')
			Cbuf_AddText(string(token))
		} else if text[i] == '-' {
			for i++; at(text, i) != 0 && text[i] != ' '; i++ {
			}
		} else {
			i++
		}
	}
}

func Cmd_Exec_f() {
	if Cmd_Argc() != 2 {
		Com_Printf("exec <filename> : execute a script file\n")
		return
	}

	name := strlcpy(Cmd_Argv(1), MAX_OSPATH-4)
	f := FS_LoadMallocFile(name)
	if f == nil {
		p := COM_SkipPath(name)
		if !strings.Contains(p, ".") {
			// no extension, so try the default (.cfg)
			name += ".cfg"
			f = FS_LoadMallocFile(name)
		}
		if f == nil {
			Com_Printf("couldn't exec %s\n", Cmd_Argv(1))
			return
		}
	}
	if cl_warncmd.value != 0 || developer.value != 0 {
		Com_Printf("execing %s\n", name)
	}

	if cbuf_current == cbuf_svc {
		Cbuf_AddText(cstr(f))
		Cbuf_AddText("\n")
	} else {
		Cbuf_InsertText("\n")
		Cbuf_InsertText(cstr(f))
	}
}

// Just prints the rest of the line to the console
func Cmd_Echo_f() {
	for i := 1; i < Cmd_Argc(); i++ {
		Com_Printf("%s ", Cmd_Argv(i))
	}
	Com_Printf("\n")
}

/*
=============================================================================
								ALIASES
=============================================================================
*/

var cmd_alias_hash [32]*cmd_alias_t
var cmd_alias *cmd_alias_t

func Cmd_FindAlias(name string) *cmd_alias_t {
	key := Com_HashKey(name)
	for alias := cmd_alias_hash[key]; alias != nil; alias = alias.hash_next {
		if Q_strcasecmp(name, alias.name) == 0 {
			return alias
		}
	}
	return nil
}

// The second result is false where C returns NULL.
func Cmd_AliasString(name string) (string, bool) {
	key := Com_HashKey(name)
	for alias := cmd_alias_hash[key]; alias != nil; alias = alias.hash_next {
		if Q_strcasecmp(name, alias.name) == 0 {
			return alias.value, true
		}
	}
	return "", false
}

func Cmd_Viewalias_f() {
	if Cmd_Argc() < 2 {
		Com_Printf("viewalias <aliasname> : view body of alias\n")
		return
	}

	alias := Cmd_FindAlias(Cmd_Argv(1))
	if alias != nil {
		Com_Printf("%s : \"%s\"\n", Cmd_Argv(1), alias.value)
	} else {
		Com_Printf("No such alias: %s\n", Cmd_Argv(1))
	}
}

func Cmd_AliasCompare(a1, a2 *cmd_alias_t) int {
	if at(a1.name, 0) == '+' {
		if at(a2.name, 0) == '+' {
			return Q_strcasecmp(a1.name[1:], a2.name[1:])
		}
		return -1
	} else if at(a1.name, 0) == '-' {
		if at(a2.name, 0) == '+' {
			return 1
		} else if at(a2.name, 0) == '-' {
			return Q_strcasecmp(a1.name[1:], a2.name[1:])
		}
		return -1
	} else if at(a2.name, 0) == '+' || at(a2.name, 0) == '-' {
		return 1
	}
	return Q_strcasecmp(a1.name, a2.name)
}

const MAX_SORTED_ALIASES = 512

func Cmd_AliasList_f() {
	var sorted_aliases []*cmd_alias_t
	for a := cmd_alias; a != nil && len(sorted_aliases) < MAX_SORTED_ALIASES; a = a.next {
		sorted_aliases = append(sorted_aliases, a)
	}
	slices.SortFunc(sorted_aliases, Cmd_AliasCompare)

	for _, a := range sorted_aliases {
		Com_Printf("\x02%s :", a.name)
		Com_Printf(" %s\n\n", a.value)
	}
	Com_Printf("----------\n%d aliases\n", len(sorted_aliases))
}

// Creates a new command that executes a command string (possibly ; separated)
func Cmd_Alias_f() {
	c := Cmd_Argc()
	if c == 1 {
		Com_Printf("%s <name> <command> : create or modify an alias\n", Cmd_Argv(0))
		Com_Printf("aliaslist : list all aliases\n")
		return
	}

	s := Cmd_Argv(1)
	if len(s) >= MAX_ALIAS_NAME {
		Com_Printf("Alias name is too long\n")
		return
	}

	key := Com_HashKey(s)

	// if the alias already exists, reuse it
	var a *cmd_alias_t
	for a = cmd_alias_hash[key]; a != nil; a = a.hash_next {
		if Q_strcasecmp(a.name, s) == 0 {
			break
		}
	}

	if a == nil {
		a = &cmd_alias_t{}
		a.next = cmd_alias
		cmd_alias = a
		a.hash_next = cmd_alias_hash[key]
		cmd_alias_hash[key] = a
	}
	a.name = s

	a.flags = 0
	if Q_strcasecmp(Cmd_Argv(0), "aliasa") == 0 {
		a.flags |= ALIAS_ARCHIVE
	}
	if cbuf_current == cbuf_svc {
		a.flags |= ALIAS_SERVER
	}
	if Q_strcasecmp(Cmd_Argv(0), "tempalias") == 0 {
		a.flags |= ALIAS_TEMP
	}

	// copy the rest of the command line
	a.value = Cmd_MakeArgs(2)
}

func Cmd_DeleteAlias(name string) bool {
	key := Com_HashKey(name)

	var prev *cmd_alias_t
	var a *cmd_alias_t
	for a = cmd_alias_hash[key]; a != nil; a = a.hash_next {
		if Q_strcasecmp(a.name, name) == 0 {
			// unlink from hash
			if prev != nil {
				prev.hash_next = a.hash_next
			} else {
				cmd_alias_hash[key] = a.hash_next
			}
			break
		}
		prev = a
	}

	if a == nil {
		return false // not found
	}

	prev = nil
	for a = cmd_alias; a != nil; a = a.next {
		if Q_strcasecmp(a.name, name) == 0 {
			// unlink from alias list
			if prev != nil {
				prev.next = a.next
			} else {
				cmd_alias = a.next
			}
			return true
		}
		prev = a
	}

	Sys_Error("Cmd_DeleteAlias: alias list broken")
	return false
}

func Cmd_UnAlias_f() {
	if Cmd_Argc() != 2 {
		Com_Printf("unalias <alias>: erase an existing alias\n")
		return
	}

	s := Cmd_Argv(1)
	if len(s) >= MAX_ALIAS_NAME {
		Com_Printf("Alias name is too long\n")
		return
	}

	Cmd_DeleteAlias(s)
}

// remove all aliases
func Cmd_UnAliasAll_f() {
	cmd_alias = nil

	// clear hash
	for i := range cmd_alias_hash {
		cmd_alias_hash[i] = nil
	}
}

func DeleteServerAliases() {
	for a := cmd_alias; a != nil; a = a.next {
		if a.flags&ALIAS_SERVER != 0 {
			Cmd_DeleteAlias(a.name)
		}
	}
}

func Cmd_WriteAliases(f io.Writer) {
	for a := cmd_alias; a != nil; a = a.next {
		if a.flags&ALIAS_ARCHIVE != 0 {
			fmt.Fprintf(f, "aliasa %s \"%s\"\n", a.name, a.value)
		}
	}
}

/*
=============================================================================
					LEGACY COMMANDS
=============================================================================
*/

type legacycmd_t struct {
	oldname, newname string
	next             *legacycmd_t
}

var legacycmds *legacycmd_t

func Cmd_AddLegacyCommand(oldname, newname string) {
	cmd := &legacycmd_t{oldname: oldname, newname: newname}
	cmd.next = legacycmds
	legacycmds = cmd
}

func Cmd_IsLegacyCommand(oldname string) bool {
	for cmd := legacycmds; cmd != nil; cmd = cmd.next {
		if Q_strcasecmp(cmd.oldname, oldname) == 0 {
			return true
		}
	}
	return false
}

var cmd_legacy_recursive = false

func Cmd_LegacyCommand() bool {
	var cmd *legacycmd_t
	for cmd = legacycmds; cmd != nil; cmd = cmd.next {
		if Q_strcasecmp(cmd.oldname, Cmd_Argv(0)) == 0 {
			break
		}
	}
	if cmd == nil {
		return false
	}

	if cmd.newname == "" {
		return true // just ignore this command
	}

	// build new command string
	text := strlcpy(cmd.newname+" "+Cmd_Args(), 1024)

	if cmd_legacy_recursive {
		Sys_Error("Cmd_LegacyCommand: Called recursively")
	}
	cmd_legacy_recursive = true
	Cmd_ExecuteString(text)
	cmd_legacy_recursive = false
	return true
}

/*
=============================================================================
					COMMAND EXECUTION
=============================================================================
*/

const MAX_ARGS = 80

var (
	cmd_argc int
	cmd_argv [MAX_ARGS]string
	cmd_args string
)

var cmd_hash_array [32]*cmd_function_t
var cmd_functions *cmd_function_t // possible commands to execute

func Cmd_Argc() int {
	return cmd_argc
}

func Cmd_Argv(arg int) string {
	if arg >= cmd_argc {
		return ""
	}
	return cmd_argv[arg]
}

// Returns a single string containing argv(1) to argv(argc() - 1)
func Cmd_Args() string {
	return cmd_args
}

// Returns a single string containing argv(start) to argv(argc() - 1)
// Unlike Cmd_Args, shrinks spaces between argvs
func Cmd_MakeArgs(start int) string {
	text := ""
	c := Cmd_Argc()
	for i := start; i < c; i++ {
		if i > start {
			text = strlcat(text, " ", 1024)
		}
		text = strlcat(text, Cmd_Argv(i), 1024)
	}
	return text
}

// Parses the given string into command line tokens.
func Cmd_TokenizeString(text string) {
	idx := 0
	cmd_argc = 0
	cmd_args = ""

	i := 0
	for {
		// skip whitespace
		for at(text, i) == ' ' || at(text, i) == '\t' || at(text, i) == '\r' {
			i++
		}

		if at(text, i) == '\n' {
			// a newline separates commands in the buffer
			break
		}

		if at(text, i) == 0 {
			return
		}

		if cmd_argc == 1 {
			cmd_args = text[i:]
		}

		rest, ok := COM_Parse(text[i:])
		if !ok {
			return
		}
		i = len(text) - len(rest)

		if cmd_argc < MAX_ARGS {
			if idx+len(com_token) >= 1024 {
				break
			}
			cmd_argv[cmd_argc] = com_token
			idx += len(com_token) + 1
			cmd_argc++
		}
	}
}

func Cmd_AddCommand(cmd_name string, function xcommand_t) {
	if cvarsregged != 0 {
		fmt.Printf("Command \"%s\" registered too late\n", cmd_name)
	}

	// fail if the command is a variable name
	if Cvar_FindVar(cmd_name) != nil {
		Com_Printf("Cmd_AddCommand: %s already defined as a var\n", cmd_name)
		return
	}

	key := Com_HashKey(cmd_name)

	// fail if the command already exists
	for cmd := cmd_hash_array[key]; cmd != nil; cmd = cmd.hash_next {
		if Q_strcasecmp(cmd_name, cmd.name) == 0 {
			Com_Printf("Cmd_AddCommand: %s already defined\n", cmd_name)
			return
		}
	}

	cmd := &cmd_function_t{name: cmd_name, function: function}
	cmd.next = cmd_functions
	cmd_functions = cmd
	cmd.hash_next = cmd_hash_array[key]
	cmd_hash_array[key] = cmd
}

func Cmd_Exists(cmd_name string) bool {
	key := Com_HashKey(cmd_name)
	for cmd := cmd_hash_array[key]; cmd != nil; cmd = cmd.hash_next {
		if Q_strcasecmp(cmd_name, cmd.name) == 0 {
			return true
		}
	}
	return false
}

func Cmd_FindCommand(cmd_name string) *cmd_function_t {
	key := Com_HashKey(cmd_name)
	for cmd := cmd_hash_array[key]; cmd != nil; cmd = cmd.hash_next {
		if Q_strcasecmp(cmd_name, cmd.name) == 0 {
			return cmd
		}
	}
	return nil
}

// Returns "" where C returns NULL.
func Cmd_CompleteCommand(partial string) string {
	n := len(partial)
	if n == 0 {
		return ""
	}

	// check for exact match
	for cmd := cmd_functions; cmd != nil; cmd = cmd.next {
		if Q_strcasecmp(partial, cmd.name) == 0 {
			return cmd.name
		}
	}
	for alias := cmd_alias; alias != nil; alias = alias.next {
		if Q_strcasecmp(partial, alias.name) == 0 {
			return alias.name
		}
	}

	// check for partial match
	for cmd := cmd_functions; cmd != nil; cmd = cmd.next {
		if Q_strncasecmp(partial, cmd.name, uint(n)) == 0 {
			return cmd.name
		}
	}
	for alias := cmd_alias; alias != nil; alias = alias.next {
		if Q_strncasecmp(partial, alias.name, uint(n)) == 0 {
			return alias.name
		}
	}

	return ""
}

func Cmd_CompleteCountPossible(partial string) int {
	n := len(partial)
	if n == 0 {
		return 0
	}
	c := 0
	for cmd := cmd_functions; cmd != nil; cmd = cmd.next {
		if Q_strncasecmp(partial, cmd.name, uint(n)) == 0 {
			c++
		}
	}
	return c
}

func Cmd_AliasCompleteCountPossible(partial string) int {
	n := len(partial)
	if n == 0 {
		return 0
	}
	c := 0
	for alias := cmd_alias; alias != nil; alias = alias.next {
		if Q_strncasecmp(partial, alias.name, uint(n)) == 0 {
			c++
		}
	}
	return c
}

func Cmd_CmdList_f() {
	var sorted_cmds []*cmd_function_t
	for cmd := cmd_functions; cmd != nil; cmd = cmd.next {
		sorted_cmds = append(sorted_cmds, cmd)
	}
	slices.SortFunc(sorted_cmds, func(a, b *cmd_function_t) int { return strings.Compare(a.name, b.name) })

	for _, cmd := range sorted_cmds {
		Com_Printf("%s\n", cmd.name)
	}
	Com_Printf("------------\n%d commands\n", len(sorted_cmds))
}

const MAX_MACROS = 64

type macro_command_t struct {
	name     string
	function func() string
	teamplay bool
}

var macro_commands [MAX_MACROS]macro_command_t
var macro_count = 0

func Cmd_AddMacroEx(s string, f func() string, teamplay bool) {
	if macro_count == MAX_MACROS {
		Sys_Error("Cmd_AddMacro: macro_count == MAX_MACROS")
	}
	macro_commands[macro_count].name = strlcpy(s, 32)
	macro_commands[macro_count].function = f
	macro_commands[macro_count].teamplay = teamplay
	macro_count++
}

func Cmd_AddMacro(s string, f func() string) {
	Cmd_AddMacroEx(s, f, false)
}

// The second result is false where C returns NULL.
func Cmd_MacroString(s string, macro_length *int) (string, bool) {
	for i := 0; i < macro_count; i++ {
		macro := &macro_commands[i]
		if Q_strncasecmp(s, macro.name, uint(len(macro.name))) == 0 {
			if cbuf_current == cbuf_main && macro.teamplay {
				cbuf_current = cbuf_formatted_comms
			}
			*macro_length = len(macro.name)
			return macro.function(), true
		}
	}
	*macro_length = 0
	return "", false
}

var cmd_sorted_macros []*macro_command_t

func Cmd_MacroList_f() {
	if macro_count == 0 {
		Com_Printf("No macros!")
		return
	}

	if cmd_sorted_macros == nil {
		for i := 0; i < macro_count; i++ {
			cmd_sorted_macros = append(cmd_sorted_macros, &macro_commands[i])
		}
		slices.SortFunc(cmd_sorted_macros, func(a, b *macro_command_t) int { return strings.Compare(a.name, b.name) })
	}

	for i := 0; i < macro_count; i++ {
		Com_Printf("$%s\n", cmd_sorted_macros[i].name)
	}
	Com_Printf("---------\n%d macros\n", macro_count)
}

// Expands all $cvar expressions to cvar values and all $macro expressions
func Cmd_ExpandString(data string, maxlen int) string {
	dest := make([]byte, 0, len(data))
	quotes := 0

	i := 0
	for {
		c := at(data, i)
		if c == 0 {
			break
		}

		if c == '"' {
			quotes++
		}

		if c == '$' && quotes&1 == 0 {
			i++

			// Copy the text after '$' to a temporary buffer and look for the
			// longest cvar match in the process
			buf := make([]byte, 0, 255)
			var bestvar *cvar_t
			for {
				c = at(data, i)
				if !(c > 32 && len(buf) < 255-1) {
					break
				}
				if c == '$' {
					break
				}
				i++
				buf = append(buf, c)
				if v := Cvar_FindVar(string(buf)); v != nil {
					bestvar = v
				}
			}

			var macro_length int
			str, have := Cmd_MacroString(string(buf), &macro_length)
			name_length := macro_length
			if bestvar != nil && (!have || len(bestvar.name) > macro_length) {
				str = bestvar.string
				have = true
				name_length = len(bestvar.name)
			}

			if have {
				// check buffer size
				if len(dest)+len(str)+len(buf)-name_length >= maxlen-1 {
					break
				}
				dest = append(dest, str...)
				dest = append(dest, buf[name_length:]...)
			} else {
				// no matching cvar or macro
				dest = append(dest, '$')
				if len(dest)+len(buf) >= maxlen-1 {
					break
				}
				dest = append(dest, buf...)
			}
		} else {
			dest = append(dest, c)
			i++
			if len(dest) >= maxlen-1 {
				break
			}
		}
	}

	return string(dest)
}

var msgtrigger_commands = []string{
	"play", "playvol", "stopsound", "set", "echo", "say", "say_team",
	"alias", "unalias", "msg_trigger", "inc", "bind", "unbind", "record",
	"easyrecord", "stop", "if", "wait", "log", "match_forcestart",
}

var formatted_comms_commands = []string{
	"if", "wait", "echo", "say", "say_team",
	"tp_point", "tp_pickup", "tp_took",
}

func cmdInList(list []string, name string) bool {
	for _, s := range list {
		if Q_strcasecmp(name, s) == 0 {
			return true
		}
	}
	return false
}

// A complete command line has been parsed, so try to execute it
func Cmd_ExecuteStringEx(context *cbuf_t, text string) {
	oldcontext := cbuf_current
	cbuf_current = context
	defer func() { cbuf_current = oldcontext }()

	buf := Cmd_ExpandString(text, 2048)
	Cmd_TokenizeString(buf)

	if Cmd_Argc() == 0 {
		return // no tokens
	}

	if cbuf_current == cbuf_svc {
		if CL_CheckServerCommand() {
			return
		}
	}

	// check functions
	if cmd := Cmd_FindCommand(cmd_argv[0]); cmd != nil {
		if can_execute_functions || cmd_argv[0] == "cfg_load" || cmd_argv[0] == "exec" || cmd_argv[0] == "alias" {
			if cbuf_current == cbuf_safe {
				if !cmdInList(msgtrigger_commands, cmd_argv[0]) {
					Com_Printf("\"%s\" cannot be used in message triggers\n", cmd_argv[0])
					return
				}
			} else if cbuf_current == cbuf_formatted_comms {
				if !cmdInList(formatted_comms_commands, cmd_argv[0]) {
					Com_Printf("\"%s\" cannot be used in combination with teamplay $macros\n", cmd_argv[0])
					return
				}
			}

			if cmd.function != nil {
				cmd.function()
			} else {
				Cmd_ForwardToServer()
			}
			return
		}

		Cbuf_AddTextEx(cbuf_cmdsave, text)
		Cbuf_AddTextEx(cbuf_cmdsave, "\n")
		return
	}

	// some bright guy decided to use "skill" as a mod command in Custom TF, sigh
	if !(Cmd_Argv(0) == "skill" && cmd_argc == 1 && Cmd_FindAlias("skill") != nil) {
		// check cvars
		if Cvar_FindVar(Cmd_Argv(0)) != nil {
			if cbuf_current == cbuf_formatted_comms {
				Com_Printf("\"%s\" cannot be used in combination with teamplay $macros\n", cmd_argv[0])
				return
			}
			if Cvar_Command() {
				return
			}
		}
	}

	// check aliases
	if a := Cmd_FindAlias(cmd_argv[0]); a != nil {
		if cbuf_current == cbuf_svc {
			Cbuf_AddText(a.value)
			Cbuf_AddText("\n")
		} else {
			inserttarget := cbuf_current
			if inserttarget == nil {
				inserttarget = cbuf_main
			}

			Cbuf_InsertTextEx(inserttarget, "\n")

			// if the alias value is a command or cvar and
			// the alias is called with parameters, add them
			if Cmd_Argc() > 1 && !strings.ContainsAny(a.value, " \t") &&
				(Cvar_FindVar(a.value) != nil || (Cmd_FindCommand(a.value) != nil && at(a.value, 0) != '+' && at(a.value, 0) != '-')) {
				Cbuf_InsertTextEx(inserttarget, Cmd_Args())
				Cbuf_InsertTextEx(inserttarget, " ")
			}
			Cbuf_InsertTextEx(inserttarget, a.value)
		}
		return
	}

	if Cmd_LegacyCommand() {
		return
	}

	if cbuf_current != cbuf_svc {
		if cl_warncmd.value != 0 || developer.value != 0 {
			Com_Printf("Unknown command \"%s\"\n", Cmd_Argv(0))
		}
	}
}

func Cmd_ExecuteString(text string) {
	Cmd_ExecuteStringEx(nil, text)
}

func is_numeric(c string) bool {
	c0, c1 := at(c, 0), at(c, 1)
	return (c0 >= '0' && c0 <= '9') ||
		((c0 == '-' || c0 == '+') && (c1 == '.' || (c1 >= '0' && c1 <= '9'))) ||
		(c0 == '.' && (c1 >= '0' && c1 <= '9'))
}

func Cmd_If_f() {
	c := Cmd_Argc()
	if c < 5 {
		Com_Printf("Usage: if <expr1> <op> <expr2> <command> [else <command>]\n")
		return
	}

	var result bool
	op := Cmd_Argv(2)
	if op == "==" || op == "=" || op == "!=" || op == "<>" {
		if is_numeric(Cmd_Argv(1)) && is_numeric(Cmd_Argv(3)) {
			result = Q_atof(Cmd_Argv(1)) == Q_atof(Cmd_Argv(3))
		} else {
			result = Cmd_Argv(1) == Cmd_Argv(3)
		}

		if op[0] != '=' {
			result = !result
		}
	} else if op == ">" {
		result = Q_atof(Cmd_Argv(1)) > Q_atof(Cmd_Argv(3))
	} else if op == "<" {
		result = Q_atof(Cmd_Argv(1)) < Q_atof(Cmd_Argv(3))
	} else if op == ">=" {
		result = Q_atof(Cmd_Argv(1)) >= Q_atof(Cmd_Argv(3))
	} else if op == "<=" {
		result = Q_atof(Cmd_Argv(1)) <= Q_atof(Cmd_Argv(3))
	} else if op == "isin" {
		result = strings.Contains(Cmd_Argv(3), Cmd_Argv(1))
	} else if op == "!isin" {
		result = !strings.Contains(Cmd_Argv(3), Cmd_Argv(1))
	} else {
		Com_Printf("unknown operator: %s\n", op)
		Com_Printf("valid operators are ==, =, !=, <>, >, <, >=, <=, isin, !isin\n")
		return
	}

	buf := ""
	if result {
		for i := 4; i < c; i++ {
			if i == 4 && Q_strcasecmp(Cmd_Argv(i), "then") == 0 {
				continue
			}
			if Q_strcasecmp(Cmd_Argv(i), "else") == 0 {
				break
			}
			if buf != "" {
				buf = strlcat(buf, " ", 1024)
			}
			buf = strlcat(buf, Cmd_Argv(i), 1024)
		}
	} else {
		i := 4
		for ; i < c; i++ {
			if Q_strcasecmp(Cmd_Argv(i), "else") == 0 {
				break
			}
		}
		if i == c {
			return
		}
		for i++; i < c; i++ {
			if buf != "" {
				buf = strlcat(buf, " ", 1024)
			}
			buf = strlcat(buf, Cmd_Argv(i), 1024)
		}
	}

	buf = strlcat(buf, "\n", 1024)
	if cbuf_current != nil {
		Cbuf_InsertTextEx(cbuf_current, buf)
	} else {
		Cbuf_InsertTextEx(cbuf_main, buf)
	}
}

// Returns the position (1 to argc - 1) in the command's argument list where
// the given parameter apears, or 0 if not present
func Cmd_CheckParm(parm string) int {
	c := Cmd_Argc()
	for i := 1; i < c; i++ {
		if Q_strcasecmp(parm, Cmd_Argv(i)) == 0 {
			return i
		}
	}
	return 0
}

func Cmd_EnableFunctionExecution() {
	can_execute_functions = true
	Cbuf_ExecuteEx(cbuf_cmdsave)
}

func Cmd_Init() {
	// register our commands
	Cmd_AddCommand("exec", Cmd_Exec_f)
	Cmd_AddCommand("echo", Cmd_Echo_f)
	Cmd_AddCommand("aliaslist", Cmd_AliasList_f)
	Cmd_AddCommand("alias", Cmd_Alias_f)
	Cmd_AddCommand("tempalias", Cmd_Alias_f)
	Cmd_AddCommand("viewalias", Cmd_Viewalias_f)
	Cmd_AddCommand("unaliasall", Cmd_UnAliasAll_f)
	Cmd_AddCommand("unalias", Cmd_UnAlias_f)
	Cmd_AddCommand("wait", Cmd_Wait_f)
	Cmd_AddCommand("cmdlist", Cmd_CmdList_f)
	Cmd_AddCommand("if", Cmd_If_f)
	Cmd_AddCommand("macrolist", Cmd_MacroList_f)
}

func Cmd_Shutdown() {
	cmd_functions = nil
}

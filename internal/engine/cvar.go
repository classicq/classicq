package engine

import (
	"fmt"
	"io"
	"slices"
	"strings"
)

// cvar flags
const (
	CVAR_ARCHIVE      = 1
	CVAR_USERINFO     = 2 // mirrored to userinfo
	CVAR_SERVERINFO   = 4 // mirrored to serverinfo
	CVAR_ROM          = 8 // read only
	CVAR_INIT         = 16
	CVAR_USER_CREATED = 32 // created by a set command
	CVAR_USER_ARCHIVE = 64 // created by a seta command
	CVAR_CHANGED      = 128
)

type cvar_t struct {
	name               string
	string             string
	flags              int
	OnChange           func(v *cvar_t, value string) bool
	PostChangeCallback func(v *cvar_t)
	value              float32
	defaultvalue       string
	group              *cvar_group_t
	next_in_group      *cvar_t
	hash_next          *cvar_t
	next               *cvar_t
}

type cvar_group_t struct {
	name  string
	count int
	head  *cvar_t
	next  *cvar_group_t
}

// cvar_groups.h
const (
	CVAR_GROUP_NO_GROUP           = "#No Group#"
	CVAR_GROUP_SERVER_PERMISSIONS = "Serverside Permissions"
	CVAR_GROUP_SERVER_PHYSICS     = "Serverside Physics"
	CVAR_GROUP_SERVERINFO         = "Serverinfo Keys"
	CVAR_GROUP_SERVER_MAIN        = "Server Settings"
	CVAR_GROUP_ITEM_NEED          = "Item Need Amounts"
	CVAR_GROUP_ITEM_NAMES         = "Item Names"
	CVAR_GROUP_SPECTATOR          = "Spectator Tracking"
	CVAR_GROUP_SCREENSHOTS        = "Screenshot Settings"
	CVAR_GROUP_DEMO               = "Demo Handling"
	CVAR_GROUP_MATCH_TOOLS        = "Match Tools"
	CVAR_GROUP_VIEW               = "View Settings"
	CVAR_GROUP_BLEND              = "Screen & Powerup Blends"
	CVAR_GROUP_SCREEN             = "Screen Settings"
	CVAR_GROUP_CROSSHAIR          = "Crosshair Settings"
	CVAR_GROUP_SBAR               = "Status Bar and Scoreboard"
	CVAR_GROUP_NETWORK            = "Network Settings"
	CVAR_GROUP_INPUT_MISC         = "Input - Misc"
	CVAR_GROUP_INPUT_JOY          = "Input - Joystick"
	CVAR_GROUP_INPUT_MOUSE        = "Input - Mouse"
	CVAR_GROUP_INPUT_KEYBOARD     = "Input - Keyboard"
	CVAR_GROUP_MP3                = "MP3 Settings"
	CVAR_GROUP_SOUND              = "Sound Settings"
	CVAR_GROUP_VIDEO              = "Video Settings"
	CVAR_GROUP_SYSTEM_SETTINGS    = "System Settings"
	CVAR_GROUP_OPENGL             = "OpenGL Rendering"
	CVAR_GROUP_TEXTURES           = "Texture Settings"
	CVAR_GROUP_VIEWMODEL          = "Weapon View Model Settings"
	CVAR_GROUP_TURB               = "Water and Sky Settings"
	CVAR_GROUP_LIGHTING           = "Lighting"
	CVAR_GROUP_PARTICLES          = "Particle Effects"
	CVAR_GROUP_EYECANDY           = "FPS and EyeCandy Settings"
	CVAR_GROUP_CHAT               = "Chat Settings"
	CVAR_GROUP_CONSOLE            = "Console Settings"
	CVAR_GROUP_SKIN               = "Skin Settings"
	CVAR_GROUP_COMMUNICATION      = "Teamplay Communications"
	CVAR_GROUP_USERINFO           = "Player Settings"
	CVAR_GROUP_CONFIG             = "Config Management"
)

var cvar_groups_list = []string{
	CVAR_GROUP_NO_GROUP,
	CVAR_GROUP_SERVER_PERMISSIONS,
	CVAR_GROUP_SERVER_PHYSICS,
	CVAR_GROUP_SERVERINFO,
	CVAR_GROUP_SERVER_MAIN,
	CVAR_GROUP_ITEM_NEED,
	CVAR_GROUP_ITEM_NAMES,
	CVAR_GROUP_SPECTATOR,
	CVAR_GROUP_SCREENSHOTS,
	CVAR_GROUP_DEMO,
	CVAR_GROUP_MATCH_TOOLS,
	CVAR_GROUP_VIEW,
	CVAR_GROUP_BLEND,
	CVAR_GROUP_SCREEN,
	CVAR_GROUP_CROSSHAIR,
	CVAR_GROUP_SBAR,
	CVAR_GROUP_NETWORK,
	CVAR_GROUP_INPUT_MISC,
	CVAR_GROUP_INPUT_JOY,
	CVAR_GROUP_INPUT_MOUSE,
	CVAR_GROUP_INPUT_KEYBOARD,
	CVAR_GROUP_MP3,
	CVAR_GROUP_SOUND,
	CVAR_GROUP_VIDEO,
	CVAR_GROUP_SYSTEM_SETTINGS,
	CVAR_GROUP_OPENGL,
	CVAR_GROUP_TEXTURES,
	CVAR_GROUP_VIEWMODEL,
	CVAR_GROUP_TURB,
	CVAR_GROUP_LIGHTING,
	CVAR_GROUP_PARTICLES,
	CVAR_GROUP_EYECANDY,
	CVAR_GROUP_CHAT,
	CVAR_GROUP_CONSOLE,
	CVAR_GROUP_SKIN,
	CVAR_GROUP_COMMUNICATION,
	CVAR_GROUP_USERINFO,
	CVAR_GROUP_CONFIG,
}

// cvar.c -- dynamic variable tracking

var cvar_hash [32]*cvar_t
var cvar_vars *cvar_t

var cvar_viewdefault = cvar_t{name: "cvar_viewdefault", string: "1"}

func Cvar_FindVar(var_name string) *cvar_t {
	key := Com_HashKey(var_name)
	for v := cvar_hash[key]; v != nil; v = v.hash_next {
		if Q_strcasecmp(var_name, v.name) == 0 {
			return v
		}
	}
	return nil
}

func Cvar_ResetVar(v *cvar_t) {
	if v.string != v.defaultvalue {
		Cvar_Set(v, v.defaultvalue)
	}
	v.flags &^= CVAR_CHANGED
}

func Cvar_Reset_f() {
	if Cmd_Argc() != 2 {
		Com_Printf("Usage: %s <variable>\n", Cmd_Argv(0))
		return
	}
	s := Cmd_Argv(1)
	if v := Cvar_FindVar(s); v != nil {
		Cvar_ResetVar(v)
	} else {
		Com_Printf("%s : No variable with name %s\n", Cmd_Argv(0), s)
	}
}

// "%f" with trailing zeros and a bare dot stripped, as cvar.c does by hand.
func cvarFloatString(value float32) string {
	val := fmt.Sprintf("%f", float64(value))
	i := len(val) - 1
	for i > 0 && val[i] == '0' {
		i--
	}
	if val[i] == '.' {
		return val[:i]
	}
	return val[:i+1]
}

func Cvar_SetDefault(v *cvar_t, value float32) {
	val := cvarFloatString(value)
	v.defaultvalue = val
	if v.flags&CVAR_CHANGED == 0 {
		Cvar_Set(v, val)
		v.flags &^= CVAR_CHANGED
	}
}

func Cvar_VariableValue(var_name string) float32 {
	v := Cvar_FindVar(var_name)
	if v == nil {
		return 0
	}
	return Q_atof(v.string)
}

func Cvar_VariableString(var_name string) string {
	v := Cvar_FindVar(var_name)
	if v == nil {
		return ""
	}
	return v.string
}

// Returns "" where C returns NULL.
func Cvar_CompleteVariable(partial string) string {
	n := len(partial)
	if n == 0 {
		return ""
	}

	// check exact match
	for cvar := cvar_vars; cvar != nil; cvar = cvar.next {
		if Q_strcasecmp(partial, cvar.name) == 0 {
			return cvar.name
		}
	}

	// check partial match
	for cvar := cvar_vars; cvar != nil; cvar = cvar.next {
		if Q_strncasecmp(partial, cvar.name, uint(n)) == 0 {
			return cvar.name
		}
	}

	return ""
}

func Cvar_CompleteCountPossible(partial string) int {
	n := len(partial)
	if n == 0 {
		return 0
	}
	c := 0
	for cvar := cvar_vars; cvar != nil; cvar = cvar.next {
		if Q_strncasecmp(partial, cvar.name, uint(n)) == 0 {
			c++
		}
	}
	return c
}

var cvar_changing = false

func Cvar_Set(v *cvar_t, value string) {
	if v == nil {
		return
	}

	if v.flags&CVAR_ROM != 0 {
		Com_Printf("\"%s\" is write protected\n", v.name)
		return
	}

	if v.flags&CVAR_INIT != 0 && host_initialized {
		if cl_warncmd.value != 0 || developer.value != 0 {
			Com_Printf("\"%s\" can only be changed with \"+set %s %s\" on the command line.\n", v.name, v.name, value)
		}
		return
	}

	floatvalue := Q_atof(value)

	if !Ruleset_ValidateCvarChange(v, value, floatvalue) {
		Com_Printf("Ruleset disallows setting \"%s\" variable to \"%s\".\n", v.name, value)
		return
	}

	v.flags |= CVAR_CHANGED

	if v.OnChange != nil && !cvar_changing {
		cvar_changing = true
		if v.OnChange(v, value) {
			cvar_changing = false
			return
		}
		cvar_changing = false
	}

	v.string = value
	v.value = floatvalue

	if v.flags&CVAR_USERINFO != 0 {
		CL_UserinfoChanged(v.name, v.string)
	}

	if v.PostChangeCallback != nil {
		v.PostChangeCallback(v)
	}
}

func Cvar_ForceSet(v *cvar_t, value string) {
	if v == nil {
		return
	}
	saved_flags := v.flags
	v.flags &^= CVAR_ROM
	Cvar_Set(v, value)
	v.flags = saved_flags
}

func Cvar_SetValue(v *cvar_t, value float32) {
	Cvar_Set(v, cvarFloatString(value))
}

func Cvar_GetFlags(v *cvar_t) int {
	return v.flags
}

func Cvar_SetFlags(v *cvar_t, flags int) {
	v.flags = flags
}

var cvar_current_group *cvar_group_t
var cvar_groups *cvar_group_t

var cvar_groups_initialised = false

func Cvar_AddGroup(name string) *cvar_group_t {
	if !cvar_groups_initialised {
		cvar_groups_initialised = true
		for _, s := range cvar_groups_list {
			Cvar_AddGroup(s)
		}
	}

	for g := cvar_groups; g != nil; g = g.next {
		if Q_strcasecmp(g.name, name) == 0 {
			return g
		}
	}

	newgroup := &cvar_group_t{name: strlcpy(name, 65)}
	newgroup.next = cvar_groups
	cvar_groups = newgroup
	return newgroup
}

func Cvar_SetCurrentGroup(name string) {
	for g := cvar_groups; g != nil; g = g.next {
		if g.name == name {
			cvar_current_group = g
			return
		}
	}
	cvar_current_group = Cvar_AddGroup(name)
}

func Cvar_ResetCurrentGroup() {
	cvar_current_group = nil
}

func Cvar_AddCvarToGroup(v *cvar_t) {
	v.group = cvar_current_group
	if v.group != nil {
		v.next_in_group = v.group.head
		v.group.head = v
		v.group.count++
	} else {
		v.next_in_group = nil
	}
}

// Adds a freestanding variable to the variable list.
// If the variable already exists, the value will not be set.
// The flags will be or'ed in if the variable exists.
func Cvar_Register(v *cvar_t) {
	if cvarsregged != 0 {
		fmt.Printf("Cvar \"%s\" registered too late\n", v.name)
	}

	// first check to see if it has already been defined
	old := Cvar_FindVar(v.name)
	if old != nil && old.flags&CVAR_USER_CREATED == 0 {
		Com_Printf("Can't register variable %s, already defined\n", v.name)
		return
	}

	// check for overlap with a command
	if Cmd_Exists(v.name) {
		Com_Printf("Cvar_Register: %s is a command\n", v.name)
		return
	}

	v.defaultvalue = v.string
	if old != nil {
		v.flags |= old.flags &^ CVAR_USER_CREATED
		s := strlcpy(old.string, 512)
		Cvar_Delete(old.name)
		if v.flags&CVAR_ROM == 0 {
			v.string = s
		}
	}

	v.value = Q_atof(v.string)

	// link the variable in
	key := Com_HashKey(v.name)
	v.hash_next = cvar_hash[key]
	cvar_hash[key] = v
	v.next = cvar_vars
	cvar_vars = v

	Cvar_AddCvarToGroup(v)

	if v.flags&CVAR_USERINFO != 0 {
		CL_UserinfoChanged(v.name, v.string)
	}
}

func Cvar_Command() bool {
	// check variables
	v := Cvar_FindVar(Cmd_Argv(0))
	if v == nil {
		return false
	}

	if Cmd_Argc() == 1 {
		if cvar_viewdefault.value != 0 {
			Com_Printf("%s : default value is \"%s\"\n", v.name, v.defaultvalue)
			Com_Printf("%*s current value is \"%s\"\n", len(v.name)+2, "", v.string)
		} else {
			Com_Printf("\"%s\" is \"%s\"\n", v.name, v.string)
		}
	} else {
		Cvar_Set(v, Cmd_MakeArgs(1))
	}

	return true
}

// Writes lines containing "set variable value" for all variables with the
// archive flag set to true.
func Cvar_WriteVariables(f io.Writer) {
	// write builtin cvars in a QW compatible way
	for v := cvar_vars; v != nil; v = v.next {
		if v.flags&CVAR_ARCHIVE != 0 {
			fmt.Fprintf(f, "%s \"%s\"\n", v.name, v.string)
		}
	}

	// write everything else
	for v := cvar_vars; v != nil; v = v.next {
		if v.flags&CVAR_USER_ARCHIVE != 0 {
			fmt.Fprintf(f, "seta %s \"%s\"\n", v.name, v.string)
		}
	}
}

func Cvar_Toggle_f() {
	if Cmd_Argc() != 2 {
		Com_Printf("toggle <cvar> : toggle a cvar on/off\n")
		return
	}

	v := Cvar_FindVar(Cmd_Argv(1))
	if v == nil {
		Com_Printf("Unknown variable \"%s\"\n", Cmd_Argv(1))
		return
	}

	if v.value != 0 {
		Cvar_Set(v, "0")
	} else {
		Cvar_Set(v, "1")
	}
}

func Cvar_CvarList_f() {
	var sorted_cvars []*cvar_t
	for v := cvar_vars; v != nil; v = v.next {
		sorted_cvars = append(sorted_cvars, v)
	}
	slices.SortFunc(sorted_cvars, func(a, b *cvar_t) int { return strings.Compare(a.name, b.name) })

	flag := func(set bool, c byte) byte {
		if set {
			return c
		}
		return ' '
	}
	for _, v := range sorted_cvars {
		Com_Printf("%c%c%c %s\n",
			flag(v.flags&(CVAR_ARCHIVE|CVAR_USER_ARCHIVE) != 0, '*'),
			flag(v.flags&CVAR_USERINFO != 0, 'u'),
			flag(v.flags&CVAR_SERVERINFO != 0, 's'),
			v.name)
	}
	Com_Printf("-------------\n%d variables\n", len(sorted_cvars))
}

func Cvar_Create(name, str string, cvarflags int) *cvar_t {
	if v := Cvar_FindVar(name); v != nil {
		return v
	}

	// Cvar doesn't exist, so we create it
	v := &cvar_t{}
	v.next = cvar_vars
	cvar_vars = v

	key := Com_HashKey(name)
	v.hash_next = cvar_hash[key]
	cvar_hash[key] = v

	v.name = name
	v.string = str
	v.defaultvalue = str
	v.flags = cvarflags
	v.value = Q_atof(v.string)

	return v
}

// returns true if the cvar was found (and deleted)
func Cvar_Delete(name string) bool {
	key := Com_HashKey(name)

	var prev *cvar_t
	var v *cvar_t
	for v = cvar_hash[key]; v != nil; v = v.hash_next {
		if Q_strcasecmp(v.name, name) == 0 {
			// unlink from hash
			if prev != nil {
				prev.hash_next = v.hash_next
			} else {
				cvar_hash[key] = v.hash_next
			}
			break
		}
		prev = v
	}

	if v == nil {
		return false
	}

	prev = nil
	for v = cvar_vars; v != nil; v = v.next {
		if Q_strcasecmp(v.name, name) == 0 {
			// unlink from cvar list
			if prev != nil {
				prev.next = v.next
			} else {
				cvar_vars = v.next
			}
			return true
		}
		prev = v
	}

	Sys_Error("Cvar list broken")
	return false
}

var cvar_seta = false

func Cvar_Set_f() {
	if Cmd_Argc() != 3 {
		Com_Printf("Usage: %s <cvar> <value>\n", Cmd_Argv(0))
		return
	}

	var_name := Cmd_Argv(1)
	v := Cvar_FindVar(var_name)

	if v != nil {
		Cvar_Set(v, Cmd_Argv(2))
	} else {
		if Cmd_Exists(var_name) {
			Com_Printf("\"%s\" is a command\n", var_name)
			return
		}
		v = Cvar_Create(var_name, Cmd_Argv(2), CVAR_USER_CREATED|CVAR_CHANGED)
	}

	if cvar_seta {
		v.flags |= CVAR_USER_ARCHIVE
	}
}

func Cvar_Seta_f() {
	cvar_seta = true
	Cvar_Set_f()
	cvar_seta = false
}

func Cvar_Inc_f() {
	c := Cmd_Argc()
	if c != 2 && c != 3 {
		Com_Printf("inc <cvar> [value]\n")
		return
	}

	v := Cvar_FindVar(Cmd_Argv(1))
	if v == nil {
		Com_Printf("Unknown variable \"%s\"\n", Cmd_Argv(1))
		return
	}

	var delta float32 = 1
	if c == 3 {
		delta = float32(atof(Cmd_Argv(2)))
	}
	Cvar_SetValue(v, v.value+delta)
}

func Cvar_Init() {
	Cmd_AddCommand("cvarlist", Cvar_CvarList_f)
	Cmd_AddCommand("toggle", Cvar_Toggle_f)
	Cmd_AddCommand("set", Cvar_Set_f)
	Cmd_AddCommand("inc", Cvar_Inc_f)
	Cmd_AddCommand("cvar_reset", Cvar_Reset_f)

	Cvar_SetCurrentGroup(CVAR_GROUP_CONSOLE)
	Cvar_Register(&cvar_viewdefault)
	Cvar_ResetCurrentGroup()
}

func Cvar_Shutdown() {
	cvar_groups = nil
}

func Cvar_CvarCompare(v1, v2 *cvar_t) int {
	return strings.Compare(v1.name, v2.name)
}

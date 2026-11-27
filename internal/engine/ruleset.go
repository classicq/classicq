// Copyright (C) 2009 Mark Olsen
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

type ruleset_s struct {
	name                 string
	Init                 func()
	AllowRJScripts       func() bool
	AllowTimeRefresh     func() bool
	AllowPacketCmd       func() bool
	ValidateCvarChange   func(cvar *cvar_t, newstringvalue string, newfloatvalue float32) bool
	AllowFTrigger        func(triggername string) bool
	AllowMsgTriggers     func() bool
	AllowMovementScripts func() bool
}

/* --- Default ruleset */

func Ruleset_Default_Init() {
}

func Ruleset_Default_AllowRJScripts() bool {
	return true
}

func Ruleset_Default_AllowTimeRefresh() bool {
	return true
}

func Ruleset_Default_AllowPacketCmd() bool {
	return true
}

func Ruleset_Default_ValidateCvarChange(cvar *cvar_t, newstringvalue string, newfloatvalue float32) bool {
	return true
}

func Ruleset_Default_AllowFTrigger(triggername string) bool {
	return true
}

func Ruleset_Default_AllowMsgTriggers() bool {
	return true
}

func Ruleset_Default_AllowMovementScripts() bool {
	return true
}

var ruleset_default = ruleset_s{
	"default",
	Ruleset_Default_Init,
	Ruleset_Default_AllowRJScripts,
	Ruleset_Default_AllowTimeRefresh,
	Ruleset_Default_AllowPacketCmd,
	Ruleset_Default_ValidateCvarChange,
	Ruleset_Default_AllowFTrigger,
	Ruleset_Default_AllowMsgTriggers,
	Ruleset_Default_AllowMovementScripts,
}

/* --- EQL ruleset */

func Ruleset_EQL_Init() {
	Cvar_Set(&cl_imitate_client, "none")
	Cvar_Set(&cl_imitate_os, "none")
}

func Ruleset_EQL_AllowRJScripts() bool {
	return false
}

func Ruleset_EQL_AllowTimeRefresh() bool {
	return false
}

func Ruleset_EQL_AllowPacketCmd() bool {
	return false
}

func Ruleset_EQL_ValidateCvarChange(cvar *cvar_t, newstringvalue string, newfloatvalue float32) bool {
	if cvar.name == "r_aliasstats" ||
		cvar.name == "cl_imitate_client" ||
		cvar.name == "cl_imitate_os" {
		return false
	}
	return true
}

func Ruleset_EQL_AllowFTrigger(triggername string) bool {
	if triggername == "f_took" ||
		triggername == "f_respawn" ||
		triggername == "f_death" ||
		triggername == "f_flagdeath" {
		return false
	}
	return true
}

func Ruleset_EQL_AllowMsgTriggers() bool {
	return false
}

func Ruleset_EQL_AllowMovementScripts() bool {
	return false
}

var ruleset_eql = ruleset_s{
	"eql",
	Ruleset_EQL_Init,
	Ruleset_EQL_AllowRJScripts,
	Ruleset_EQL_AllowTimeRefresh,
	Ruleset_EQL_AllowPacketCmd,
	Ruleset_EQL_ValidateCvarChange,
	Ruleset_EQL_AllowFTrigger,
	Ruleset_EQL_AllowMsgTriggers,
	Ruleset_EQL_AllowMovementScripts,
}

/* --- */

var ruleset = &ruleset_default

func Ruleset_GetName() string {
	return ruleset.name
}

func Ruleset_AllowRJScripts() bool {
	return ruleset.AllowRJScripts()
}

func Ruleset_AllowTimeRefresh() bool {
	return ruleset.AllowTimeRefresh()
}

func Ruleset_AllowPacketCmd() bool {
	return ruleset.AllowPacketCmd()
}

func Ruleset_ValidateCvarChange(cvar *cvar_t, newstringvalue string, newfloatvalue float32) bool {
	return ruleset.ValidateCvarChange(cvar, newstringvalue, newfloatvalue)
}

func Ruleset_AllowFTrigger(triggername string) bool {
	return ruleset.AllowFTrigger(triggername)
}

func Ruleset_AllowMsgTriggers() bool {
	return ruleset.AllowMsgTriggers()
}

func Ruleset_AllowMovementScripts() bool {
	return ruleset.AllowMovementScripts()
}

/* --- */

var rulesets = []*ruleset_s{
	&ruleset_default,
	&ruleset_eql,
}

func cl_ruleset_callback(v *cvar_t, str string) bool {
	for _, r := range rulesets {
		if r.name == str {
			return false
		}
	}
	Com_Printf("Invalid ruleset name. Available rulesets are:\n")
	for _, r := range rulesets {
		Com_Printf("%s\n", r.name)
	}
	return true
}

var cl_ruleset = cvar_t{name: "cl_ruleset", string: "eql", OnChange: cl_ruleset_callback}

func Ruleset_Activate() {
	ruleset = &ruleset_default
	for _, r := range rulesets {
		if r.name == cl_ruleset.string {
			r.Init()
			ruleset = r
			break
		}
	}
}

func Ruleset_CvarInit() {
	Cvar_Register(&cl_ruleset)
}

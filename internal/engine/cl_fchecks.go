package engine

// cl_fchecks.c: answers to the f_version, f_skins, f_modified, f_server and
// f_ruleset requests in chat

var f_ruleset_reply_time, f_mod_reply_time, f_version_reply_time, f_skins_reply_time, f_server_reply_time float32

func FChecks_VersionResponse() {
	Cbuf_AddText(va("say %s %s:"+QW_ARCH+"\n", fversion_clientnames[imitatedclientnum], fversion_osnames[imitatedosnum]))
}

func FChecks_FServerResponse() {
	var adr netaddr

	if !NET_StringToAdr(nil, cls.servername, &adr) {
		return
	}

	if adr.type_ == NA_IPV4 && adr.ipv4.port == 0 {
		adr.ipv4.port = uint16(BigShort(PORT_SERVER))
	} else if adr.type_ == NA_IPV6 && adr.ipv6.port == 0 {
		adr.ipv6.port = uint16(BigShort(PORT_SERVER))
	}

	Cbuf_AddText(va("say classicQ f_server response: %s\n", NET_AdrToString(&adr)))
}

func FChecks_SkinsResponse(fbskins float32) {
	Cbuf_AddText(va("say All skins %d%% fullbright\n", int(fbskins*100)))
}

func FChecks_VersionRequest(s string) bool {
	if cl.spectator != 0 || (f_version_reply_time != 0 && cls.realtime-float64(f_version_reply_time) < 20) {
		return false
	}

	if Util_F_Match(s, "f_version") || Util_F_Match(s, "fuh_version") {
		FChecks_VersionResponse()
		f_version_reply_time = float32(cls.realtime)
		return true
	}
	return false
}

func FChecks_SkinRequest(s string) bool {
	fbskins := bound(0, r_fullbrightSkins.value, cl.fbskins)
	if cl.spectator != 0 || fbskins == 0 || (f_skins_reply_time != 0 && cls.realtime-float64(f_skins_reply_time) < 20) {
		return false
	}

	if Util_F_Match(s, "f_skins") {
		FChecks_SkinsResponse(fbskins)
		f_skins_reply_time = float32(cls.realtime)
		return true
	}
	return false
}

func FChecks_CheckFModRequest(s string) bool {
	if cl.spectator != 0 || (f_mod_reply_time != 0 && cls.realtime-float64(f_mod_reply_time) < 20) {
		return false
	}

	if Util_F_Match(s, "f_modified") {
		FMod_Response()
		f_mod_reply_time = float32(cls.realtime)
		return true
	}
	return false
}

func FChecks_CheckFServerRequest(s string) bool {
	var adr netaddr

	if cl.spectator != 0 || (f_server_reply_time != 0 && cls.realtime-float64(f_server_reply_time) < 20) {
		return false
	}

	if Util_F_Match(s, "f_server") && NET_StringToAdr(nil, cls.servername, &adr) {
		FChecks_FServerResponse()
		f_server_reply_time = float32(cls.realtime)
		return true
	}
	return false
}

func FChecks_CheckFRulesetRequest(s string) bool {
	if cl.spectator != 0 || (f_ruleset_reply_time != 0 && cls.realtime-float64(f_ruleset_reply_time) < 20) {
		return false
	}

	if Util_F_Match(s, "f_ruleset") {
		Cbuf_AddText(va("say classicQ Ruleset: %s\n", Ruleset_GetName()))
		f_ruleset_reply_time = float32(cls.realtime)
		return true
	}
	return false
}

func FChecks_CheckRequest(s string) {
	FChecks_VersionRequest(s)
	FChecks_SkinRequest(s)
	FChecks_CheckFModRequest(s)
	FChecks_CheckFServerRequest(s)
	FChecks_CheckFRulesetRequest(s)
}

func FChecks_CvarInit() {
	Cmd_AddCommand("fuh_version", FChecks_VersionResponse)
	Cmd_AddCommand("f_server", FChecks_FServerResponse)
}

func FChecks_Init() {
}

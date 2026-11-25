package engine

// vid_mode_null.c: no display mode list on this platform

func Sys_Video_GetModeList() []string {
	return nil
}

func Sys_Video_FreeModeList(displaymodes []string) {
}

func Sys_Video_GetModeDescription(mode string) string {
	return ""
}

func Sys_Video_FreeModeDescription(modedescription string) {
}

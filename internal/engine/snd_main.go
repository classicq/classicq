package engine

// snd_dma.c -- main control for any streaming sound output device

var soundcard *SoundCard

// =======================================================================
// Internal sound data & structures
// =======================================================================

var channels [MAX_CHANNELS]channel_t
var total_channels int

var snd_blocked = 0
var snd_initialized = false

var snd_ambient = true

var listener_origin vec3_t
var listener_right vec3_t
var sound_nominal_clip_dist float32 = 1000.0

var paintedtime int // sample PAIRS

const MAX_SFX = 512

var known_sfx []sfx_t // [MAX_SFX]
var num_sfx int

var ambient_sfx [NUM_AMBIENTS]*sfx_t

var sound_started = 0

// The driver mixes on the main thread (sdl_audio_update), so there is
// nothing to lock; the calls mark where a threaded mixer would.
func S_LockMixer() {}

func S_UnlockMixer() {}

var bgmvolume = cvar_t{name: "bgmvolume", string: "1", flags: CVAR_ARCHIVE}
var s_volume = cvar_t{name: "volume", string: "0.2", flags: CVAR_ARCHIVE}
var s_nosound = cvar_t{name: "s_nosound", string: "0"}
var s_precache = cvar_t{name: "s_precache", string: "1"}
var s_loadas8bit = cvar_t{name: "s_loadas8bit", string: "0"}
var s_khz = cvar_t{name: "s_khz", string: "11"}
var s_ambientlevel = cvar_t{name: "s_ambientlevel", string: "1"}
var s_ambientfade = cvar_t{name: "s_ambientfade", string: "100"}
var s_show = cvar_t{name: "s_show", string: "0"}
var s_swapstereo = cvar_t{name: "s_swapstereo", string: "0"}
var s_driver = cvar_t{name: "s_driver", string: "auto"}

type SoundDriver struct {
	name     string
	init     *SoundInitFunc
	cvarinit *SoundCvarInitFunc
}

var SNDSDL_CvarInit SoundCvarInitFunc
var SNDSDL_Init SoundInitFunc

var sounddrivers = [...]SoundDriver{
	{"SDL", &SNDSDL_Init, &SNDSDL_CvarInit},
}

const NUMSOUNDDRIVERS = len(sounddrivers)

// ====================================================================
// User-setable variables
// ====================================================================

func S_SoundInfo_f() {
	if sound_started == 0 || soundcard == nil {
		Com_Printf("sound system not started\n")
		return
	}
	Com_Printf("%s driver\n", soundcard.drivername)
	Com_Printf("%5d stereo\n", soundcard.channels-1)
	Com_Printf("%5d samplebits\n", soundcard.samplebits)
	Com_Printf("%5d speed\n", soundcard.speed)
	Com_Printf("%5d total_channels\n", total_channels)
}

func S_InitDriver() {
	rc := false
	var i int
	var rate uint

	if !snd_initialized {
		return
	}

	// -nodisplay runs without SDL (see vid_sdl.go): no driver
	if vid_nodisplay {
		Com_ErrorPrintf("Unable to initialise sound output.\n")
		sound_started = 0
		return
	}

	paintedtime = 0

	if s_khz.value == 44 {
		rate = 44100
	} else if s_khz.value == 22 {
		rate = 22050
	} else {
		rate = 11025
	}

	soundcard = &SoundCard{}
	for i = 0; i < NUMSOUNDDRIVERS; i++ {
		if *sounddrivers[i].init != nil {
			if Q_strcasecmp(s_driver.string, "auto") != 0 {
				if Q_strcasecmp(sounddrivers[i].name, s_driver.string) != 0 {
					continue
				}
			}

			*soundcard = SoundCard{}
			rc = (*sounddrivers[i].init)(soundcard, int(rate), 2, 16)
			if rc {
				soundcard.drivername = sounddrivers[i].name
				break
			}
		}
	}

	if i == NUMSOUNDDRIVERS {
		soundcard = nil
	}

	if !rc {
		Com_ErrorPrintf("Unable to initialise sound output.\n")
		if Q_strcasecmp(s_driver.string, "auto") != 0 {
			Com_Printf("WARNING: You have specified a custom s_driver which may be the cause of failure. Try setting it to \"auto\"\n")
		}
		soundcard = nil
		sound_started = 0
		return
	}

	S_LockMixer()
	sound_started = 1
	S_UnlockMixer()
}

func S_ShutdownDriver() {
	if sound_started == 0 {
		return
	}

	S_LockMixer()
	sound_started = 0
	S_UnlockMixer()

	soundcard.Shutdown(soundcard)

	soundcard = nil
}

func SND_Restart_f() {
	var i int

	S_StopAllSounds(true)
	S_ShutdownDriver()

	for i = 0; i < num_sfx; i++ {
		known_sfx[i].sfxcache = nil
	}

	if s_nosound.value == 0 {
		S_InitDriver()

		for i = 0; i < num_sfx; i++ {
			S_LoadSound(&known_sfx[i])
		}

		S_StopAllSounds(true)
	}
}

func S_CvarInit() {
	var i int

	Cvar_SetCurrentGroup(CVAR_GROUP_SOUND)
	Cvar_Register(&bgmvolume)
	Cvar_Register(&s_volume)
	Cvar_Register(&s_nosound)
	Cvar_Register(&s_precache)
	Cvar_Register(&s_loadas8bit)
	Cvar_Register(&s_khz)
	Cvar_Register(&s_ambientlevel)
	Cvar_Register(&s_ambientfade)
	Cvar_Register(&s_show)
	Cvar_Register(&s_swapstereo)
	Cvar_Register(&s_driver)

	Cvar_ResetCurrentGroup()

	// compatibility with old configs
	Cmd_AddLegacyCommand("volume", "s_volume")
	Cmd_AddLegacyCommand("nosound", "s_nosound")
	Cmd_AddLegacyCommand("precache", "s_precache")
	Cmd_AddLegacyCommand("loadas8bit", "s_loadas8bit")
	Cmd_AddLegacyCommand("ambient_level", "s_ambientlevel")
	Cmd_AddLegacyCommand("ambient_fade", "s_ambientfade")
	Cmd_AddLegacyCommand("snd_show", "s_show")

	Cmd_AddCommand("snd_restart", SND_Restart_f)
	Cmd_AddCommand("play", S_Play_f)
	Cmd_AddCommand("playvol", S_PlayVol_f)
	Cmd_AddCommand("stopsound", S_StopAllSounds_f)
	Cmd_AddCommand("soundlist", S_SoundList_f)
	Cmd_AddCommand("soundinfo", S_SoundInfo_f)

	for i = 0; i < NUMSOUNDDRIVERS; i++ {
		if *sounddrivers[i].cvarinit != nil {
			(*sounddrivers[i].cvarinit)()
		}
	}

	snd_initialized = true
}

func S_Init() {
	if s_nosound.value == 0 {
		S_InitDriver()
	}

	SND_InitScaletable()

	known_sfx = make([]sfx_t, MAX_SFX)

	num_sfx = 0

	ambient_sfx[AMBIENT_WATER] = S_PrecacheSound("ambience/water1.wav")
	ambient_sfx[AMBIENT_SKY] = S_PrecacheSound("ambience/wind2.wav")

	S_StopAllSounds(true)
}

// =======================================================================
// Shutdown sound engine
// =======================================================================

func S_Shutdown() {
	S_ShutdownDriver()
	known_sfx = nil
}

// =======================================================================
// Load a sound
// =======================================================================

func S_FindName(name string) *sfx_t {
	var i int
	var sfx *sfx_t

	if len(name) >= MAX_QPATH {
		Sys_Error("Sound name too long: %s", name)
	}

	// see if already loaded
	for i = 0; i < num_sfx; i++ {
		if known_sfx[i].name == name {
			return &known_sfx[i]
		}
	}

	if num_sfx == MAX_SFX {
		Sys_Error("S_FindName: out of sfx_t")
	}

	sfx = &known_sfx[i]
	sfx.name = name

	num_sfx++

	return sfx
}

func S_PrecacheSound(name string) *sfx_t {
	sfx := S_FindName(name)

	// cache it in
	if s_precache.value != 0 {
		S_LoadSound(sfx)
	}

	return sfx
}

//=============================================================================

func SND_PickChannel(entnum, entchannel int) *channel_t {
	var ch_idx, first_to_die, life_left int

	// Check for replacement sound, or find the best one to replace
	first_to_die = -1
	life_left = 0x7fffffff
	for ch_idx = NUM_AMBIENTS; ch_idx < NUM_AMBIENTS+MAX_DYNAMIC_CHANNELS; ch_idx++ {
		if entchannel != 0 && // channel 0 never overrides
			channels[ch_idx].entnum == entnum &&
			(channels[ch_idx].entchannel == entchannel || entchannel == -1) { // always override sound from same entity
			first_to_die = ch_idx
			break
		}

		// don't let monster sounds override player sounds
		if channels[ch_idx].entnum == cl.playernum+1 && entnum != cl.playernum+1 && channels[ch_idx].sfx != nil {
			continue
		}

		if channels[ch_idx].end-paintedtime < life_left {
			life_left = channels[ch_idx].end - paintedtime
			first_to_die = ch_idx
		}
	}

	if first_to_die == -1 {
		return nil
	}

	if channels[first_to_die].sfx != nil {
		channels[first_to_die].sfx = nil
	}

	return &channels[first_to_die]
}

func SND_Spatialize(ch *channel_t) {
	var dot, dist, lscale, rscale, scale float32
	var source_vec vec3_t

	// anything coming from the view entity will always be full volume
	if ch.entnum == cl.playernum+1 {
		ch.leftvol = ch.master_vol
		ch.rightvol = ch.master_vol
		return
	}

	// calculate stereo seperation and distance attenuation

	VectorSubtract(ch.origin, listener_origin, &source_vec)

	dist = VectorNormalize(&source_vec) * ch.dist_mult
	dot = DotProduct(listener_right, source_vec)

	if soundcard.channels == 1 {
		rscale = 1.0
		lscale = 1.0
	} else {
		rscale = float32(1.0 + float64(dot))
		lscale = float32(1.0 - float64(dot))
	}

	// add in distance effect
	scale = float32((1.0 - float64(dist)) * float64(rscale))
	ch.rightvol = int(float32(ch.master_vol) * scale)
	if ch.rightvol < 0 {
		ch.rightvol = 0
	}

	scale = float32((1.0 - float64(dist)) * float64(lscale))
	ch.leftvol = int(float32(ch.master_vol) * scale)
	if ch.leftvol < 0 {
		ch.leftvol = 0
	}
}

// =======================================================================
// Start a sound effect
// =======================================================================

func S_StartSound(entnum, entchannel int, sfx *sfx_t, origin vec3_t, fvol, attenuation float32) {
	var target_chan, check *channel_t
	var sc *sfxcache_t
	var vol, ch_idx, skip int

	if sound_started == 0 {
		return
	}

	if sfx == nil {
		return
	}

	if s_nosound.value != 0 {
		return
	}

	vol = int(fvol * 255)

	S_LockMixer()

	// pick a channel to play on
	target_chan = SND_PickChannel(entnum, entchannel)
	if target_chan == nil {
		S_UnlockMixer()
		return
	}

	// spatialize
	*target_chan = channel_t{}
	target_chan.origin = origin
	target_chan.dist_mult = attenuation / sound_nominal_clip_dist
	target_chan.master_vol = vol
	target_chan.entnum = entnum
	target_chan.entchannel = entchannel
	SND_Spatialize(target_chan)

	if target_chan.leftvol == 0 && target_chan.rightvol == 0 {
		S_UnlockMixer()
		return // not audible at all
	}

	// new channel
	sc = S_LoadSound(sfx)
	if sc == nil {
		target_chan.sfx = nil
		S_UnlockMixer()
		return // couldn't load the sound's data
	}

	target_chan.sfx = sfx
	target_chan.pos = 0
	target_chan.end = paintedtime + sc.length

	// if an identical sound has also been started this frame, offset the pos
	// a bit to keep it from just making the first one louder
	for ch_idx = NUM_AMBIENTS; ch_idx < NUM_AMBIENTS+MAX_DYNAMIC_CHANNELS; ch_idx++ {
		check = &channels[ch_idx]
		if check == target_chan {
			continue
		}
		if check.sfx == sfx && check.pos == 0 {
			skip = rand() % int(0.1*float64(soundcard.speed))
			if skip >= target_chan.end {
				skip = target_chan.end - 1
			}
			target_chan.pos += skip
			target_chan.end -= skip
			break
		}
	}

	S_UnlockMixer()
}

func S_StopSound(entnum, entchannel int) {
	var i int

	S_LockMixer()

	for i = 0; i < MAX_DYNAMIC_CHANNELS; i++ {
		if channels[i].entnum == entnum && channels[i].entchannel == entchannel {
			channels[i].end = 0
			channels[i].sfx = nil
			break
		}
	}

	S_UnlockMixer()
}

func S_StopAllSounds(clear bool) {
	S_LockMixer()

	total_channels = MAX_DYNAMIC_CHANNELS + NUM_AMBIENTS // no statics

	channels = [MAX_CHANNELS]channel_t{}

	S_UnlockMixer()
}

func S_StopAllSounds_f() {
	S_StopAllSounds(true)
}

func S_StaticSound(sfx *sfx_t, origin vec3_t, vol, attenuation float32) {
	var ss *channel_t
	var sc *sfxcache_t

	if sfx == nil {
		return
	}

	S_LockMixer()

	if total_channels == MAX_CHANNELS {
		Com_Printf("total_channels == MAX_CHANNELS\n")
		S_UnlockMixer()
		return
	}

	ss = &channels[total_channels]
	total_channels++

	sc = S_LoadSound(sfx)
	if sc == nil {
		S_UnlockMixer()
		return
	}

	if sc.loopstart == -1 {
		Com_Printf("Sound %s not looped\n", sfx.name)
		S_UnlockMixer()
		return
	}

	ss.sfx = sfx
	ss.origin = origin
	ss.master_vol = int(vol)
	ss.dist_mult = (attenuation / 64) / sound_nominal_clip_dist
	ss.end = paintedtime + sc.length

	SND_Spatialize(ss)

	S_UnlockMixer()
}

//=============================================================================

func S_UpdateAmbientSounds() {
	var l *mleaf_t
	var vol float32
	var ambient_channel int
	var chan_ *channel_t

	if !snd_ambient {
		return
	}

	// calc ambient sound levels
	if cl.worldmodel == nil {
		return
	}

	l = Mod_PointInLeaf(listener_origin, cl.worldmodel)
	if l == nil || s_ambientlevel.value == 0 {
		for ambient_channel = 0; ambient_channel < NUM_AMBIENTS; ambient_channel++ {
			channels[ambient_channel].sfx = nil
		}
		return
	}

	for ambient_channel = 0; ambient_channel < NUM_AMBIENTS; ambient_channel++ {
		chan_ = &channels[ambient_channel]
		chan_.sfx = ambient_sfx[ambient_channel]

		vol = s_ambientlevel.value * float32(l.ambient_sound_level[ambient_channel])
		if vol < 8 {
			vol = 0
		}

		// don't adjust volume too fast
		if float32(chan_.master_vol) < vol {
			chan_.master_vol = int(float64(chan_.master_vol) + cls.frametime*float64(s_ambientfade.value))
			if float32(chan_.master_vol) > vol {
				chan_.master_vol = int(vol)
			}
		} else if float32(chan_.master_vol) > vol {
			chan_.master_vol = int(float64(chan_.master_vol) - cls.frametime*float64(s_ambientfade.value))
			if float32(chan_.master_vol) < vol {
				chan_.master_vol = int(vol)
			}
		}

		chan_.leftvol = chan_.master_vol
		chan_.rightvol = chan_.master_vol
	}
}

// Called once each time through the main loop
func S_Update(origin, forward, right, up vec3_t) {
	var i, j, total int
	var ch, combine *channel_t

	if sound_started == 0 || snd_blocked > 0 {
		return
	}

	S_LockMixer()

	listener_origin = origin
	listener_right = right

	// update general area ambient sound sources
	S_UpdateAmbientSounds()

	combine = nil

	// update spatialization for static and dynamic sounds
	for i = NUM_AMBIENTS; i < total_channels; i++ {
		ch = &channels[i]
		if ch.sfx == nil {
			continue
		}
		SND_Spatialize(ch) // respatialize channel
		if ch.leftvol == 0 && ch.rightvol == 0 {
			continue
		}

		// try to combine static sounds with a previous channel of the same
		// sound effect so we don't mix five torches every frame

		if i >= MAX_DYNAMIC_CHANNELS+NUM_AMBIENTS {
			// see if it can just use the last one
			if combine != nil && combine.sfx == ch.sfx {
				combine.leftvol += ch.leftvol
				combine.rightvol += ch.rightvol
				ch.leftvol = 0
				ch.rightvol = 0
				continue
			}
			// search for one
			for j = MAX_DYNAMIC_CHANNELS + NUM_AMBIENTS; j < i; j++ {
				if channels[j].sfx == ch.sfx {
					break
				}
			}
			combine = &channels[j]

			if j == total_channels {
				combine = nil
			} else {
				if combine != ch {
					combine.leftvol += ch.leftvol
					combine.rightvol += ch.rightvol
					ch.leftvol = 0
					ch.rightvol = 0
				}
				continue
			}
		}
	}

	// debugging output
	if s_show.value != 0 {
		total = 0
		for i = 0; i < total_channels; i++ {
			ch = &channels[i]
			if ch.sfx != nil && (ch.leftvol != 0 || ch.rightvol != 0) {
				total++
			}
		}

		Com_Printf("----(%d)----\n", total)
	}

	S_UnlockMixer()

	if soundcard.Update != nil {
		soundcard.Update(soundcard)
	}
}

// called by the driver from S_Update, once per frame
func S_MixAudio(buf []byte, frames int) {
	S_LockMixer()

	if sound_started == 0 || snd_blocked > 0 {
		clear(buf[:frames*soundcard.channels*(soundcard.samplebits/8)])
		S_UnlockMixer()
		return
	}

	if paintedtime > 0x40000000 {
		// time to chop things off to avoid 32 bit limits
		total_channels = MAX_DYNAMIC_CHANNELS + NUM_AMBIENTS
		channels = [MAX_CHANNELS]channel_t{}
		paintedtime = 0
	}

	soundcard.buffer = buf
	snd_mixbase = paintedtime
	S_PaintChannels(paintedtime + frames)

	S_UnlockMixer()
}

/*
===============================================================================
console functions
===============================================================================
*/

var s_play_hash = 345

func S_Play_f() {
	var i int
	var name string
	var sfx *sfx_t

	for i = 1; i < Cmd_Argc(); i++ {
		name = Cmd_Argv(i)
		if len(name) > 255 {
			name = name[:255]
		}
		name = COM_DefaultExtension(name, ".wav", 256)
		sfx = S_PrecacheSound(name)
		S_StartSound(s_play_hash, 0, sfx, listener_origin, 1.0, 0.0)
		s_play_hash++
	}
}

var s_playvol_hash = 543

func S_PlayVol_f() {
	var i int
	var vol float32
	var name string
	var sfx *sfx_t

	for i = 1; i+1 < Cmd_Argc(); i += 2 {
		name = Cmd_Argv(i)
		if len(name) > 255 {
			name = name[:255]
		}
		name = COM_DefaultExtension(name, ".wav", 256)
		sfx = S_PrecacheSound(name)
		vol = Q_atof(Cmd_Argv(i + 1))
		S_StartSound(s_playvol_hash, 0, sfx, listener_origin, vol, 0.0)
		s_playvol_hash++
	}
}

func S_SoundList_f() {
	var i, size, total int
	var sfx *sfx_t
	var sc *sfxcache_t

	total = 0
	for i = 0; i < num_sfx; i++ {
		sfx = &known_sfx[i]
		sc = sfx.sfxcache
		if sc == nil {
			continue
		}
		size = sc.length * sc.width * (sc.stereo + 1)
		total += size
		if sc.loopstart >= 0 {
			Com_Printf("L")
		} else {
			Com_Printf(" ")
		}
		Com_Printf("(%2db) %6d : %s\n", sc.width*8, size, sfx.name)
	}
	Com_Printf("Total resident: %d\n", total)
}

func S_LocalSound(sound string) {
	var sfx *sfx_t

	if s_nosound.value != 0 {
		return
	}
	if sound_started == 0 {
		return
	}

	sfx = S_PrecacheSound(sound)
	if sfx == nil {
		Com_Printf("S_LocalSound: can't cache %s\n", sound)
		return
	}
	S_StartSound(cl.playernum+1, -1, sfx, vec3_origin, 1, 0)
}

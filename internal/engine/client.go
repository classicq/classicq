package engine

import "os"

// client.h

var cl_net_from netaddr
var cl_net_message sizebuf_t

const MAX_PROJECTILES = 32

// player_state_t is the information needed by a player entity
// to do move prediction and to generate a drawable entity
type player_state_t struct {
	messagenum int // all players won't be updated each frame

	state_time float64   // not the same as the packet time, because player commands come asyncronously
	command    usercmd_t // last command for prediction

	origin     vec3_t
	viewangles vec3_t // only for demos, not from server
	velocity   vec3_t

	weaponframe int

	modelindex int
	frame      int
	skinnum    int
	effects    int

	flags int // dead, gib, etc

	pm_type       int
	waterjumptime float32
	onground      bool
	jump_held     bool
	jump_msec     int // fix bunny-hop flickering
}

const MAX_SCOREBOARDNAME = 16

type player_info_t struct {
	userid int

	// scoreboard information
	name      string
	entertime float32
	frags     int
	ping      int
	pl        byte
	spectator byte

	// skin information
	topcolor         byte
	bottomcolor      byte
	real_topcolor    byte
	real_bottomcolor byte

	ignored bool // for ignore

	stats [MAX_CL_STATS]int

	userinfo string
	team     string
	skin     string
}

type interpolate_t struct {
	interpolate bool
	origin      vec3_t
	oldindex    int
}

type frame_t struct {
	// generated on client side
	cmd            usercmd_t // cmd that generated the frame
	senttime       float64   // time cmd was sent off
	delta_sequence int       // sequence number to delta from, -1 = full update

	// received from server
	receivedtime    float64                     // time message was received, or -1
	playerstate     [MAX_CLIENTS]player_state_t // message received that reflects performing the usercmd
	packet_entities packet_entities_t
	invalid         bool // true if the packet_entities delta was invalid
}

type centity_t struct {
	baseline     entity_state_t
	current      entity_state_t
	lerp_origin  vec3_t
	trail_origin vec3_t
	flags        int
	old_origin   vec3_t
	old_angles   vec3_t
	oldframe     int
	sequence     int
	oldsequence  int
	startlerp    float64
	deltalerp    float64
	frametime    float64
}

const CENT_TRAILDRAWN = 1

type cshift_t struct {
	destcolor [3]int
	percent   int // 0-256
}

const (
	CSHIFT_CONTENTS = 0
	CSHIFT_DAMAGE   = 1
	CSHIFT_BONUS    = 2
	CSHIFT_POWERUP  = 3
	NUM_CSHIFTS     = 4
)

const MAX_DLIGHTS = 64

type dlighttype_t int

const (
	lt_default dlighttype_t = iota
	lt_muzzleflash
	lt_explosion
	lt_rocket
	lt_red
	lt_blue
	lt_redblue
	lt_green
	lt_white
	NUM_DLIGHTTYPES
)

type dlight_t struct {
	key      int // so entities can reuse same entry
	origin   vec3_t
	radius   float32
	die      float32 // stop lighting after this time
	decay    float32 // drop this each second
	minlight float32 // don't add when contributing less
	type_    int
	bubble   int // non zero means no flashblend bubble
}

type lightstyle_t struct {
	length int
	map_   string
}

type static_entity struct {
	next *static_entity
	ent  entity_t
}

const (
	MAX_EFRAGS   = 8192
	MAX_DEMOS    = 8
	MAX_DEMONAME = 16
)

type cactive_t int

const (
	ca_disconnected cactive_t = iota // full screen console with no connection
	ca_demostart                     // starting up a demo
	ca_connected                     // netchan_t established, waiting for svc_serverdata
	ca_onserver                      // processing data lists, donwloading, etc
	ca_active                        // everything is in, so frames can be rendered
)

type dltype_t int // download type

const (
	dl_none dltype_t = iota
	dl_model
	dl_sound
	dl_single
)

// the clientPersistent_t structure is persistent through an arbitrary number
// of server connections
type clientPersistent_t struct {
	// connection information
	state cactive_t

	framecount     int     // incremented every frame, never reset
	realtime       float64 // scaled by cl_demospeed
	demotime       float64 // scaled by cl_demospeed, reset when starting a demo
	demotimeoffset float64 // demotime-demotimeoffset=gametime
	trueframetime  float64 // time since last frame
	frametime      float64 // time since last frame, scaled by cl_demospeed
	framedev       float64

	// Let's clear the air with unpolluted, although more, time variables
	realactualfirstdemotimestamp float64 // the first timestamp present in the demo file. -1 before the first frame has been parsed.
	realactualdemotime           float64 // the time in the demo we have most recently read from. 0 is the first frame of the demo.

	// network stuff
	netchan    netchan_t
	servername string // name of server from original connect
	server_adr netaddr

	netqw *NetQW

	// private userinfo for sending to masterless servers
	userinfo string

	// on a local server these may differ from com_gamedirfile and com_gamedir
	gamedirfile string
	gamedir     string

	download          *os.File // file transfer from server
	downloadtempname  string
	downloadname      string
	downloadnumber    int
	downloadtype      dltype_t
	downloadpercent   int
	downloadrate      int // KB/s, average since downloadstarttime
	downloadstarttime float64

	// demo recording info must be here, because record is started before
	// entering a map (and clearing clientState_t)
	demorecording    bool
	demoplayback     bool
	timedemo         bool
	demomessage_data [MAX_MSGLEN * 2]byte
	demomessage      sizebuf_t

	latency float32 // rolling average

	mvdplayback bool // playing mvd
	lastto      int
	lasttype    int
	findtrack   bool

	ftexsupported          uint32
	fte2supported          uint32
	mvdprotocolextensions1 uint32
	extensionsqueried      bool
}

var cls clientPersistent_t

// cl.paused flags
const (
	PAUSED_SERVER = 1
	PAUSED_DEMO   = 2
)

// the clientState_t structure is wiped completely at every server signon
type clientState_t struct {
	servercount int // server identification for prespawns

	serverinfo string

	// some important serverinfo keys are mirrored here:
	deathmatch   int
	teamplay     int
	gametype     int  // GAME_COOP or GAME_DEATHMATCH
	teamfortress bool // true if gamedir is "fortress"
	fpd          int  // FAQ proxy flags
	z_ext        int  // ZQuake protocol extensions flags
	maxfps       float32

	parsecount       int // server message counter
	oldparsecount    int
	validsequence    int // this is the sequence number of the last good packetentity_t we got. If this is 0, we can't render a frame yet
	oldvalidsequence int

	spectator int

	last_ping_request float64 // while showing scoreboard

	// sentcmds[cl.netchan.outgoing_sequence & UPDATE_MASK] = cmd
	frames [UPDATE_BACKUP]frame_t

	// information for local display
	stats        [MAX_CL_STATS]int // health, etc
	item_gettime [32]float32       // cl.time of acquiring item, for blinking
	faceanimtime float32           // use anim frame if cl.time < this

	cshifts [NUM_CSHIFTS]cshift_t // color shifts for damage, powerups and content types

	// the client maintains its own idea of view angles, which are
	// sent to the server each frame. And only reset at level change
	// and teleport times
	viewangles vec3_t

	// the client simulates or interpolates movement to get these values
	time             float64 // this is the time value that the client is rendering at. always <= realtime
	servertime       float64
	servertime_works bool
	gametime         float64
	simorg           vec3_t
	simvel           vec3_t
	simangles        vec3_t

	// pitch drifting vars

	onground bool
	crouch   float32 // local amount for smoothing stepups

	paused int // a combination of PAUSED_SERVER and PAUSED_DEMO flags

	ideal_punchangle float32 // temporary view kick from weapon firing
	punchangle       float32 // drifts towards ideal_punchangle
	rollangle        float32 // smooth out rollangle changes when strafing

	intermission        int // don't change view angle, full screen, etc
	completed_time      int // latched from time at intermission start
	solo_completed_time int // to draw on intermission screen

	// information that is static for the entire time connected to a server
	model_name     [MAX_MODELS]string
	sound_name     [MAX_SOUNDS]string
	model_precache [MAX_MODELS]*model_t
	sound_precache [MAX_SOUNDS]*sfx_t

	levelname     string // for display on solo scoreboard
	playernum     int
	viewplayernum int // either playernum or spec_track (in chase camera mode)

	// refresh related state
	worldmodel   *model_t // cl_entitites[0].model
	free_efrags  *efrag_t
	first_static *static_entity
	last_static  *static_entity

	cdtrack int // cd audio

	viewent centity_t // weapon model

	// all player information
	players [MAX_CLIENTS]player_info_t

	// sprint buffer
	sprint_level int
	sprint_buf   string

	// localized movement vars
	entgravity    float32
	maxspeed      float32
	bunnyspeedcap float32

	waterlevel int

	whensaid     [10]float64 // For floodprots
	whensaidhead int         // Head value for floodprots

	standby   bool
	countdown bool

	fbskins       float32
	truelightning float32
	minlight      int
	watervis      bool
	allow_lumas   bool

	int_projectiles [MAX_PROJECTILES]interpolate_t

	protoversion int

	weapon_to_model_index [8]int
}

var cl clientState_t

type visentlist_t struct {
	list  []entity_t
	count int
	max   int
	alpha int
}

var cl_firstpassents, cl_visents, cl_alphaents visentlist_t

const CL_MAX_EDICTS = 2048

var cl_entities [CL_MAX_EDICTS]centity_t
var cl_efrags [MAX_EFRAGS]efrag_t
var cl_lightstyle [MAX_LIGHTSTYLES]lightstyle_t
var cl_dlights [MAX_DLIGHTS]dlight_t
var cl_dlight_active [MAX_DLIGHTS / 32]uint32

var host_basepal []byte
var host_colormap []byte

// cl_parse.c
const (
	NET_TIMINGS     = 256
	NET_TIMINGSMASK = 255
)

// cl_ents.c
type cl_modelindex_t int

const (
	mi_spike cl_modelindex_t = iota
	mi_player
	mi_flag
	mi_tf_flag
	mi_tf_stan
	mi_explod1
	mi_explod2
	mi_h_player
	mi_gib1
	mi_gib2
	mi_gib3
	mi_rocket
	mi_grenade
	mi_bubble
	mi_vaxe
	mi_vbio
	mi_vgrap
	mi_vknife
	mi_vknife2
	mi_vmedi
	mi_vspan
	cl_num_modelindices
)

// cl_cam.c
const (
	CAM_NONE  = 0
	CAM_TRACK = 1
)

const (
	RSSHOT_WIDTH  = 320
	RSSHOT_HEIGHT = 200
)

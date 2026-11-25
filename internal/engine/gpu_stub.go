package engine

import "math"

// gpu_stub.c

var d_8to24table [256]uint32
var d_8to24table2 [256]uint32
var vid_gamma float32 = 1.0
var vid_gamma_table [256]byte

func Check_Gamma(pal []byte) {
	var palette [768]byte

	if i := COM_CheckParm("-gamma"); i != 0 && i+1 < com_argc {
		vid_gamma = float32(bound(0.3, float64(Q_atof(com_argv[i+1])), 1))
		Cvar_SetDefault(&v_gamma, vid_gamma)
	} else {
		vid_gamma = 1
	}

	if vid_gamma != 1 {
		for i := 0; i < 256; i++ {
			inf := float32(255*math.Pow((float64(i)+0.5)/255.5, float64(vid_gamma)) + 0.5)
			if inf > 255 {
				inf = 255
			}

			vid_gamma_table[i] = byte(inf)
		}
	} else {
		for i := 0; i < 256; i++ {
			vid_gamma_table[i] = byte(i)
		}
	}

	for i := 0; i < 768; i++ {
		palette[i] = vid_gamma_table[pal[i]]
	}

	copy(pal, palette[:])
}

func VID_SetPalette(palette []byte) {
	// 8 8 8 encoding
	pal := palette
	for i := 0; i < 256; i++ {
		r := uint32(pal[0])
		g := uint32(pal[1])
		b := uint32(pal[2])
		pal = pal[3:]

		d_8to24table[i] = (255 << 24) + (b << 16) + (g << 8) + (r << 0)
	}
	d_8to24table[255] = 0 // 255 is transparent

	// Tonik: create a brighter palette for bmodel textures
	pal = palette
	for i := 0; i < 256; i++ {
		r := uint32(float64(pal[0]) * (2.0 / 1.5))
		if r > 255 {
			r = 255
		}
		g := uint32(float64(pal[1]) * (2.0 / 1.5))
		if g > 255 {
			g = 255
		}
		b := uint32(float64(pal[2]) * (2.0 / 1.5))
		if b > 255 {
			b = 255
		}
		pal = pal[3:]
		d_8to24table2[i] = (255 << 24) + (b << 16) + (g << 8) + (r << 0)
	}

	d_8to24table2[255] = 0 // 255 is transparent
}

// shared globals
var texture_extension_number = 1
var currenttexture = -1
var gl_max_size_default = 2048

var gl_max_size = cvar_t{name: "gl_max_size", string: "512"}

var r_netgraph = cvar_t{name: "r_netgraph", string: "0"}

var netgraphtexture int

var r_cache_thrash bool
var frustum [4]mplane_t
var gl_fbo = true

var gl_clear = cvar_t{name: "gl_clear", string: "1"}

func R_Set2D() {}

func Draw_SetSize(width, height uint) {}

// ---- FBO / post-process shims, the real pass lives in gpu_vid.c ----

func R_FBO_Init(width, height int) bool {
	return true
}

func R_FBO_Bind()   {}
func R_FBO_Unbind() {}

func R_FBO_GetColorTexture() uint {
	return 0
}

func R_PostProcess_IsReady() bool {
	return true
}

func R_PostProcess_Draw(color_tex uint, gamma, contrast float32, blend []float32) {
	GPU_SetPostParams(gamma, contrast, blend)
}

func R_PostProcess_CrtGamma() float32 {
	return GPU_CrtCompGamma()
}

var r_refdef refdef_t

// some cvars are registered only so old configs load without warnings
var (
	r_drawentities              = cvar_t{name: "r_drawentities", string: "1"}
	r_lerpframes                = cvar_t{name: "r_lerpframes", string: "1"}
	r_lerpmuzzlehack            = cvar_t{name: "r_lerpmuzzlehack", string: "1"}
	r_drawflame                 = cvar_t{name: "r_drawflame", string: "1"}
	r_speeds                    = cvar_t{name: "r_speeds", string: "0"}
	r_fullbright                = cvar_t{name: "r_fullbright", string: "1"}
	r_lightmap                  = cvar_t{name: "r_lightmap", string: "1"}
	gl_shaftlight               = cvar_t{name: "gl_shaftlight", string: "1"}
	r_wateralpha                = cvar_t{name: "r_wateralpha", string: "1"}
	r_fastturb                  = cvar_t{name: "r_fastturb", string: "0"}
	r_dynamic                   = cvar_t{name: "r_dynamic", string: "1"}
	r_novis                     = cvar_t{name: "r_novis", string: "0"}
	r_fullbrightSkins           = cvar_t{name: "r_fullbrightSkins", string: "0"}
	r_fastsky                   = cvar_t{name: "r_fastsky", string: "0"}
	r_skycolor                  = cvar_t{name: "r_skycolor", string: "4"}
	gl_colorlights              = cvar_t{name: "gl_colorlights", string: "1"}
	r_farclip                   = cvar_t{name: "r_farclip", string: "8192"}
	gl_detail                   = cvar_t{name: "gl_detail", string: "0"}
	gl_caustics                 = cvar_t{name: "gl_caustics", string: "1"}
	gl_subdivide_size           = cvar_t{name: "gl_subdivide_size", string: "128", flags: CVAR_ARCHIVE}
	gl_clearColor               = cvar_t{name: "gl_clearColor", string: "0 0 0"}
	gl_cull                     = cvar_t{name: "gl_cull", string: "1"}
	gl_smoothmodels             = cvar_t{name: "gl_smoothmodels", string: "0"}
	gl_polyblend                = cvar_t{name: "gl_polyblend", string: "1"}
	gl_flashblend               = cvar_t{name: "gl_flashblend", string: "0"}
	gl_playermip                = cvar_t{name: "gl_playermip", string: "0"}
	gl_finish                   = cvar_t{name: "gl_finish", string: "0"}
	gl_fb_bmodels               = cvar_t{name: "gl_fb_bmodels", string: "1"}
	gl_fb_models                = cvar_t{name: "gl_fb_models", string: "1"}
	gl_lightmode                = cvar_t{name: "gl_lightmode", string: "1"}
	gl_loadlitfiles             = cvar_t{name: "gl_loadlitfiles", string: "1"}
	gl_water_program            = cvar_t{name: "gl_water_program", string: "1"}
	gl_part_explosions          = cvar_t{name: "gl_part_explosions", string: "1"}
	gl_part_trails              = cvar_t{name: "gl_part_trails", string: "0"}
	gl_part_spikes              = cvar_t{name: "gl_part_spikes", string: "0"}
	gl_part_gunshots            = cvar_t{name: "gl_part_gunshots", string: "2"}
	gl_part_blood               = cvar_t{name: "gl_part_blood", string: "0"}
	gl_part_telesplash          = cvar_t{name: "gl_part_telesplash", string: "0"}
	gl_part_blobs               = cvar_t{name: "gl_part_blobs", string: "0"}
	gl_part_lavasplash          = cvar_t{name: "gl_part_lavasplash", string: "1"}
	gl_part_inferno             = cvar_t{name: "gl_part_inferno", string: "1"}
	gl_miptexLevel              = cvar_t{name: "gl_miptexLevel", string: "0"}
	gl_scaleModelTextures       = cvar_t{name: "gl_scaleModelTextures", string: "0"}
	gl_scaleTurbTextures        = cvar_t{name: "gl_scaleTurbTextures", string: "1"}
	gl_externalTextures_world   = cvar_t{name: "gl_externalTextures_world", string: "1"}
	gl_externalTextures_bmodels = cvar_t{name: "gl_externalTextures_bmodels", string: "1"}
)

var stub_cvars = []*cvar_t{
	&r_drawentities, &r_lerpframes, &r_lerpmuzzlehack, &r_drawflame, &r_speeds,
	&r_fullbright, &r_lightmap, &gl_shaftlight, &r_wateralpha, &r_fastturb,
	&r_dynamic, &r_novis, &r_netgraph, &r_fullbrightSkins, &r_fastsky,
	&r_skycolor, &gl_colorlights, &r_farclip, &gl_detail, &gl_caustics,
	&gl_subdivide_size, &gl_clear, &gl_clearColor, &gl_cull, &gl_smoothmodels,
	&gl_polyblend, &gl_flashblend, &gl_playermip, &gl_finish, &gl_fb_bmodels,
	&gl_fb_models, &gl_lightmode, &gl_loadlitfiles, &r_skyname, &gl_water_program,
	&gl_part_explosions, &gl_part_trails, &gl_part_spikes, &gl_part_gunshots,
	&gl_part_blood, &gl_part_telesplash, &gl_part_blobs, &gl_part_lavasplash,
	&gl_part_inferno, &gl_max_size,
	&gl_miptexLevel, &gl_scaleModelTextures, &gl_scaleTurbTextures,
	&gl_externalTextures_world, &gl_externalTextures_bmodels,
	&r_crt_phosphor, &r_crt_dotbloom, &vid_framesinflight,
}

func R_CvarInit() {
	for _, v := range stub_cvars {
		Cvar_Register(v)
	}

	R_Particles_CvarInit()

	Cmd_AddCommand("loadsky", R_LoadSky_f)
	Cmd_AddCommand("timerefresh", R_TimeRefresh_f)
}

var r_origin, vpn, vright, vup vec3_t
var r_worldentity entity_t
var d_lightstylevalue [256]int
var r_visframecount, r_framecount int
var c_brush_polys, c_alias_polys int
var modelorg vec3_t
var currententity *entity_t
var r_viewleaf, r_oldviewleaf *mleaf_t
var r_viewleaf2, r_oldviewleaf2 *mleaf_t // for watervis hack
var r_skyboxloaded bool

var particletexture, skyboxtextures int
var underwatertexture, detailtexture int

func R_CommonCvarInit() {}

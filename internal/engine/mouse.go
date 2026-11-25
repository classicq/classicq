package engine

import (
	"math"
	"sync"
)

var sensitivity = cvar_t{name: "sensitivity", string: "4", flags: CVAR_ARCHIVE}
var m_pitch = cvar_t{name: "m_pitch", string: "0.022", flags: CVAR_ARCHIVE}
var m_yaw = cvar_t{name: "m_yaw", string: "0.022"}
var m_accel = cvar_t{name: "m_accel", string: "0"}
var m_filter = cvar_t{name: "m_filter", string: "0"}

// The callback compares against these cvars, which Go rejects as an
// initialization cycle in the literals above.
func init() {
	for _, v := range []*cvar_t{&sensitivity, &m_pitch, &m_yaw, &m_accel, &m_filter} {
		v.OnChange = mouse_cvar_callback
	}
}

type Mouse struct {
	mutex      sync.Mutex
	viewangles vec3_t
	oldmousex  int
	oldmousey  int

	// cvar shadowing
	sensitivity float32
	m_pitch     float32
	m_yaw       float32
	m_accel     float32
	m_filter    float32
}

var mouse_global *Mouse

func mouse_cvar_callback(v *cvar_t, value string) bool {
	Cvar_Set(v, value)

	if mouse_global != nil {
		mouse_global.mutex.Lock()

		switch v {
		case &sensitivity:
			mouse_global.sensitivity = sensitivity.value
		case &m_pitch:
			mouse_global.m_pitch = m_pitch.value
		case &m_yaw:
			mouse_global.m_yaw = m_yaw.value
		case &m_accel:
			mouse_global.m_accel = m_accel.value
		case &m_filter:
			mouse_global.m_filter = m_filter.value
		}

		mouse_global.mutex.Unlock()
	}

	return true
}

func Mouse_UpdateValues(mouse *Mouse) {
	var mx, my float64
	var mousex, mousey int
	var filterfrac float32
	var mousespeed float32

	VID_GetMouseMovement(&mousex, &mousey)

	if mouse.m_filter != 0 {
		filterfrac = float32(float64(bound(float32(0), mouse.m_filter, float32(1))) / 2.0)
		mx = float64(float32(mousex)*(1-filterfrac) + float32(mouse.oldmousex)*filterfrac)
		my = float64(float32(mousey)*(1-filterfrac) + float32(mouse.oldmousey)*filterfrac)
	} else {
		mx = float64(mousex)
		my = float64(mousey)
	}

	mouse.oldmousex = mousex
	mouse.oldmousey = mousey

	if mouse.m_accel != 0 {
		mousespeed = float32(math.Sqrt(float64(mousex*mousex + mousey*mousey)))
		mx *= float64(mousespeed*mouse.m_accel + mouse.sensitivity)
		my *= float64(mousespeed*mouse.m_accel + mouse.sensitivity)
	} else {
		mx *= float64(mouse.sensitivity)
		my *= float64(mouse.sensitivity)
	}

	mouse.viewangles[YAW] = float32(float64(mouse.viewangles[YAW]) - float64(mouse.m_yaw)*mx)

	if mouse.viewangles[YAW] < 0 {
		mouse.viewangles[YAW] = float32(360 - math.Mod(float64(-mouse.viewangles[YAW]), 360))
	} else if mouse.viewangles[YAW] >= 360 {
		mouse.viewangles[YAW] = float32(math.Mod(float64(mouse.viewangles[YAW]), 360))
	}

	mouse.viewangles[PITCH] = float32(float64(mouse.viewangles[PITCH]) + float64(mouse.m_pitch)*my)
	mouse.viewangles[PITCH] = bound(float32(-70), mouse.viewangles[PITCH], float32(80))
}

func Mouse_GetViewAngles(viewangles *vec3_t) {
	mouse_global.mutex.Lock()

	Mouse_UpdateValues(mouse_global)

	*viewangles = mouse_global.viewangles

	mouse_global.mutex.Unlock()
}

func Mouse_SetViewAngles(viewangles vec3_t) {
	mouse_global.mutex.Lock()

	mouse_global.viewangles = viewangles

	mouse_global.mutex.Unlock()
}

func Mouse_AddViewAngles(viewangles vec3_t) {
	mouse_global.mutex.Lock()

	VectorAdd(viewangles, mouse_global.viewangles, &mouse_global.viewangles)

	mouse_global.mutex.Unlock()
}

func Mouse_Init() bool {
	mouse_global = &Mouse{}

	mouse_global.mutex.Lock()

	mouse_global.sensitivity = sensitivity.value
	mouse_global.m_pitch = m_pitch.value
	mouse_global.m_yaw = m_yaw.value
	mouse_global.m_accel = m_accel.value
	mouse_global.m_filter = m_filter.value

	mouse_global.mutex.Unlock()

	return true
}

func Mouse_Shutdown() {
	mouse_global = nil
}

func Mouse_CvarInit() {
	Cvar_SetCurrentGroup(CVAR_GROUP_INPUT_MOUSE)
	Cvar_Register(&sensitivity)
	Cvar_Register(&m_pitch)
	Cvar_Register(&m_yaw)
	Cvar_Register(&m_accel)
	Cvar_Register(&m_filter)
}

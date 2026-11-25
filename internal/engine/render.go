package engine

// render.h -- public interface to refresh functions

const (
	TOP_RANGE    = 16 // soldier uniform colors
	BOTTOM_RANGE = 96
)

type efrag_t struct {
	leaf     *mleaf_t
	leafnext *efrag_t
	entity   *entity_t
	entnext  *efrag_t
}

const (
	RF_WEAPONMODEL = 1
	RF_NOSHADOW    = 2
	RF_LIMITLERP   = 4
)

type entity_t struct {
	origin     vec3_t
	angles     vec3_t
	model      *model_t       // nil = no model
	skinnum    int            // for Alias models
	scoreboard *player_info_t // identify player
	flags      int
	oldframe   int
	frame      int
	framelerp  float32
	efrag      *efrag_t // linked list of efrags (FIXME)
	visframe   int      // last frame this entity was found in an active leaf. only used for static objects
	topnode    uint32   // for bmodels, first world node that splits bmodel (index, leafs past numnodes), 0xffffffff if not split
}

type refdef_t struct {
	vrect        vrect_t // subwindow in video for refresh
	vieworg      vec3_t
	viewangles   vec3_t
	fov_x, fov_y float32
}

type trail_type_t int

const (
	ROCKET_TRAIL trail_type_t = iota
	GRENADE_TRAIL
	ALT_ROCKET_TRAIL
	BLOOD_TRAIL
	BIG_BLOOD_TRAIL
	TRACER1_TRAIL
	TRACER2_TRAIL
	VOOR_TRAIL
)

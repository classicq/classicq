
typedef enum
{
	pt_static, pt_grav, pt_slowgrav, pt_fire, pt_explode, pt_explode2, pt_blob, pt_blob2
} ptype_t;

typedef struct particle_s
{
	vec3_t		org;
	float		color;
	struct particle_s	*next;
	vec3_t		vel;
	float		ramp;
	float		die;
	ptype_t		type;
} particle_t;

void R_DrawParticleInit(void);
void R_DrawParticleBegin(void);
void R_DrawParticleEnd(void);
void R_DrawParticle(particle_t *p);


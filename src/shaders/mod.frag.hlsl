Texture2D tex : register(t0, space2);
SamplerState smp : register(s0, space2);

struct Input
{
    float4 color : TEXCOORD0;
    float2 st : TEXCOORD1;
    float2 lm : TEXCOORD2;
};

// GL_DECAL first stage: mix incoming color with texture by texture alpha;
// paired with DST_COLOR/SRC_COLOR blend for caustics and detail passes
float4 main(Input input) : SV_Target
{
    float4 c = tex.Sample(smp, input.st);
    return float4(lerp(input.color.rgb, c.rgb, c.a), 1.0);
}

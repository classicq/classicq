Texture2D solid_tex : register(t0, space2);
SamplerState solid_smp : register(s0, space2);
Texture2D alpha_tex : register(t1, space2);
SamplerState alpha_smp : register(s1, space2);

struct Input
{
    float4 color : TEXCOORD0;
    float2 st : TEXCOORD1;
    float2 lm : TEXCOORD2;
};

// two scrolling layers, alpha layer decals over solid
float4 main(Input input) : SV_Target
{
    float3 solid = solid_tex.Sample(solid_smp, input.st).rgb;
    float4 alpha = alpha_tex.Sample(alpha_smp, input.lm);
    return float4(lerp(solid, alpha.rgb, alpha.a), 1.0);
}

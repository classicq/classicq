Texture2D tex : register(t0, space2);
SamplerState tex_smp : register(s0, space2);
Texture2D lightmap : register(t1, space2);
SamplerState lm_smp : register(s1, space2);

struct Input
{
    float4 color : TEXCOORD0;
    float2 st : TEXCOORD1;
    float2 lm : TEXCOORD2;
};

float4 main(Input input) : SV_Target
{
    float4 t = tex.Sample(tex_smp, input.st);
    // GL_GREATER 0.666
    if (t.a * input.color.a <= 0.666)
        discard;
    float3 l = lightmap.Sample(lm_smp, input.lm).rgb;
    return float4(t.rgb * l * input.color.rgb, t.a * input.color.a);
}

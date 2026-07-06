Texture2D tex : register(t0, space2);
SamplerState smp : register(s0, space2);
Texture2D fbtex : register(t1, space2);
SamplerState fbsmp : register(s1, space2);

struct Input
{
    float4 color : TEXCOORD0;
    float2 st : TEXCOORD1;
    float2 lm : TEXCOORD2;
};

// base modulated by vertex light, fullbright texels decal on top
float4 main(Input input) : SV_Target
{
    float4 c = tex.Sample(smp, input.st) * input.color;
    float4 fb = fbtex.Sample(fbsmp, input.st);
    c.rgb = lerp(c.rgb, fb.rgb, fb.a);
    return c;
}

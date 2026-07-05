cbuffer UBO : register(b0, space3)
{
    float cltime;
    float3 pad;
};

Texture2D tex : register(t0, space2);
SamplerState smp : register(s0, space2);

struct Input
{
    float4 color : TEXCOORD0;
    float2 st : TEXCOORD1;
    float2 lm : TEXCOORD2;
};

// port of the GLSL warp shader from gl_warp.c
float4 main(Input input) : SV_Target
{
    const float pi = 3.14159265358979323846;
    float os = input.st.x;
    float ot = input.st.y;
    float s = os + ((8.0 / 64.0) + sin((ot + cltime) * pi) * (8.0 / 64.0));
    float t = ot + ((8.0 / 64.0) + sin((os + cltime) * pi) * (8.0 / 64.0));
    return tex.Sample(smp, float2(s, t)) * input.color;
}

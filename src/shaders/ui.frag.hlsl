Texture2D tex : register(t0, space2);
SamplerState smp : register(s0, space2);

struct Input
{
    float4 color : TEXCOORD0;
    float2 uv : TEXCOORD1;
};

float4 main(Input input) : SV_Target
{
    return tex.Sample(smp, input.uv) * input.color;
}

Texture2D tex : register(t0, space2);
SamplerState smp : register(s0, space2);

struct Input
{
    float4 color : TEXCOORD0;
    float2 st : TEXCOORD1;
    float2 lm : TEXCOORD2;
};

float4 main(Input input) : SV_Target
{
    return tex.Sample(smp, input.st) * input.color;
}

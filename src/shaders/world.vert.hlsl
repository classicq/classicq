cbuffer UBO : register(b0, space1)
{
    float4x4 mvp;
};

struct Input
{
    float3 pos : TEXCOORD0;
    float2 st : TEXCOORD1;
    float2 lm : TEXCOORD2;
    float4 color : TEXCOORD3;
};

struct Output
{
    float4 color : TEXCOORD0;
    float2 st : TEXCOORD1;
    float2 lm : TEXCOORD2;
    float4 pos : SV_Position;
};

Output main(Input input)
{
    Output output;
    output.pos = mul(mvp, float4(input.pos, 1.0));
    output.st = input.st;
    output.lm = input.lm;
    output.color = input.color;
    return output;
}

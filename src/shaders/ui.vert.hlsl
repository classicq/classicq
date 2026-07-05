cbuffer UBO : register(b0, space1)
{
    float4x4 ortho;
};

struct Input
{
    float2 pos : TEXCOORD0;
    float2 uv : TEXCOORD1;
    float4 color : TEXCOORD2;
};

struct Output
{
    float4 color : TEXCOORD0;
    float2 uv : TEXCOORD1;
    float4 pos : SV_Position;
};

Output main(Input input)
{
    Output output;
    output.pos = mul(ortho, float4(input.pos, 0.0, 1.0));
    output.uv = input.uv;
    output.color = input.color;
    return output;
}

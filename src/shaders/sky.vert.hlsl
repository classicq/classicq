cbuffer UBO : register(b0, space1)
{
    float4x4 mvp;
    float4 origin_speed1;   // xyz view origin, w layer1 scroll
    float4 speed2;          // x layer2 scroll
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

// EmitSkyPolys: dir z tripled, projected to sphere, layers scroll
Output main(Input input)
{
    Output output;
    float3 dir = input.pos - origin_speed1.xyz;
    dir.z *= 3.0;

    float len = 6.0 * 63.0 / length(dir);
    dir.x *= len;
    dir.y *= len;

    output.st = float2((origin_speed1.w + dir.x) * (1.0 / 128.0),
                       (origin_speed1.w + dir.y) * (1.0 / 128.0));
    output.lm = float2((speed2.x + dir.x) * (1.0 / 128.0),
                       (speed2.x + dir.y) * (1.0 / 128.0));

    output.pos = mul(mvp, float4(input.pos, 1.0));
    output.color = input.color;
    return output;
}

struct Output
{
    float2 uv : TEXCOORD0;
    float4 pos : SV_Position;
};

Output main(uint id : SV_VertexID)
{
    Output output;
    float2 uv = float2((id << 1) & 2, id & 2);
    output.uv = uv;
    output.pos = float4(uv * float2(2.0, -2.0) + float2(-1.0, 1.0), 0.0, 1.0);
    return output;
}

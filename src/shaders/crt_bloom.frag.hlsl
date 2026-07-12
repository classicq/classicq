// Themaister's dot 'n bloom shader (public domain), HLSL port

cbuffer UBO : register(b0, space3)
{
    float2 res;
    float2 pad;
};

Texture2D tex : register(t0, space2);
SamplerState smp : register(s0, space2);

static const float gamma = 1.2;
static const float shine = 0.25;
static const float blend = 0.12;

float dist(float2 coord, float2 source)
{
    float2 delta = coord - source;
    return sqrt(dot(delta, delta));
}

float color_bloom(float3 color)
{
    const float3 gray_coeff = float3(0.30, 0.59, 0.11);
    float bright = dot(color, gray_coeff);
    return lerp(1.0 + shine, 1.0 - shine, bright);
}

float3 lookup(float2 pixel_no, float offset_x, float offset_y, float2 coord)
{
    float2 offset = float2(offset_x, offset_y);
    float3 color = tex.Sample(smp, coord).rgb;
    float delta = dist(frac(pixel_no), offset + float2(0.5, 0.5));
    return color * exp(-gamma * delta * color_bloom(color));
}

float4 main(float2 uv : TEXCOORD0) : SV_Target
{
    float dx = 1.0 / res.x;
    float dy = 1.0 / res.y;
    float2 pixel_no = uv * res;

    float3 mid_color = lookup(pixel_no, 0.0, 0.0, uv);
    float3 color = float3(0.0, 0.0, 0.0);
    color += lookup(pixel_no, -1.0, -1.0, uv + float2(-dx, -dy));
    color += lookup(pixel_no,  0.0, -1.0, uv + float2(0.0, -dy));
    color += lookup(pixel_no,  1.0, -1.0, uv + float2(dx, -dy));
    color += lookup(pixel_no, -1.0,  0.0, uv + float2(-dx, 0.0));
    color += mid_color;
    color += lookup(pixel_no,  1.0,  0.0, uv + float2(dx, 0.0));
    color += lookup(pixel_no, -1.0,  1.0, uv + float2(-dx, dy));
    color += lookup(pixel_no,  0.0,  1.0, uv + float2(0.0, dy));
    color += lookup(pixel_no,  1.0,  1.0, uv + float2(dx, dy));
    float3 out_color = lerp(1.2 * mid_color, color, blend);

    return float4(out_color, 1.0);
}

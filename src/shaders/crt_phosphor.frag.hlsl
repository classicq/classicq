// Themaister's phosphor shader (public domain), HLSL port

cbuffer UBO : register(b0, space3)
{
    float2 res;
    float2 pad;
};

Texture2D tex : register(t0, space2);
SamplerState smp : register(s0, space2);

float3 to_focus(float pixel)
{
    pixel = fmod(pixel + 3.0, 3.0);
    if (pixel >= 2.0)
        return float3(pixel - 2.0, 0.0, 3.0 - pixel);
    else if (pixel >= 1.0)
        return float3(0.0, 2.0 - pixel, pixel - 1.0);
    else
        return float3(1.0 - pixel, pixel, 0.0);
}

float4 main(float2 uv : TEXCOORD0) : SV_Target
{
    float y = frac(uv.y * res.y);
    float intensity = exp(-0.15 * y);

    float2 one_x = float2(1.0 / (3.0 * res.x), 0.0);

    float3 color = tex.Sample(smp, uv - 0.0 * one_x).rgb;
    float3 color_prev = tex.Sample(smp, uv - 1.0 * one_x).rgb;
    float3 color_prev_prev = tex.Sample(smp, uv - 2.0 * one_x).rgb;

    float pixel_x = 3.0 * uv.x * res.x;

    float3 focus = to_focus(pixel_x - 0.0);
    float3 focus_prev = to_focus(pixel_x - 1.0);
    float3 focus_prev_prev = to_focus(pixel_x - 2.0);

    float3 result =
        1.2 * color * focus +
        0.9 * color_prev * focus_prev +
        0.45 * color_prev_prev * focus_prev_prev;

    result = 2.3 * pow(max(result, 0.0), 1.4);

    return float4(intensity * result, 1.0);
}

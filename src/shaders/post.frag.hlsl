cbuffer UBO : register(b0, space3)
{
    float4 blend;
    float gamma;
    float contrast;
    float2 pad;
};

Texture2D scene : register(t0, space2);
SamplerState smp : register(s0, space2);

// port of gl_post_process.c fragment shader
float4 main(float2 uv : TEXCOORD0) : SV_Target
{
    float3 c = scene.Sample(smp, uv).rgb;
    c = lerp(c, blend.rgb, blend.a);
    c *= contrast;
    c = pow(max(c, 0.0), gamma);
    return float4(c, 1.0);
}

# Shaders

Sources are the .hlsl files. The build embeds the precompiled blobs from
compiled/ (via the generated shaders_gen.h); blobs are committed because
neither the build nor CI compiles HLSL.

Recompiling needs shadercross from
https://github.com/libsdl-org/SDL_shadercross (current blobs built with
3.0.0). Per file, from this directory:

    shadercross x.frag.hlsl -o compiled/x.frag.spv    # also .dxil and .msl

All three formats must exist or the build breaks. `zig build shaders`
recompiles everything at once - a different shadercross version rewrites
every blob, so don't use it just to add one shader.

SDL_GPU register conventions: fragment textures/samplers tN/sN in space2,
cbuffer b0 in space3; vertex cbuffer b0 in space1. Entry point: main.
New shaders also need a load_shader + pipeline hookup in gpu_vid.c.

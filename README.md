![Windows](https://img.shields.io/badge/Windows-0078D6?logo=windows&logoColor=white) ![macOS](https://img.shields.io/badge/macOS-000000?logo=apple&logoColor=white) ![Linux](https://img.shields.io/badge/Linux-FCC624?logo=linux&logoColor=black) ![SDL3](https://img.shields.io/badge/SDL-3-1E4E79) ![Vulkan](https://img.shields.io/badge/Vulkan-A41E22?logo=vulkan&logoColor=white) ![Direct3D 12](https://img.shields.io/badge/Direct3D_12-107C10) ![Metal](https://img.shields.io/badge/Metal-8A8A8E?logo=apple&logoColor=white)

# classicQ

A QuakeWorld client for the **30th anniversary of Quake** (June 22, 2026).

![classicq-screenshot.webp](assets/icons/classicq-screenshot.webp)

## Download

You'll need `pak0.pak` and `pak1.pak` from a licensed copy of Quake. Available on [Steam](https://store.steampowered.com/app/2310/Quake/), [GOG](https://www.gog.com/game/quake_the_offering), or that dusty CD from 1996.

With those in hand, grab the classicQ binary from [the releases page](https://github.com/classicq/classicq/releases) and drop the `.pak` files into `id1/`.

## Build from source

1. Install [Zig](https://ziglang.org/download/).
2. Clone the repo.
3. Run `zig build`.

Binary lands in `assets/` next to the configs. Run from there.

SDL3 is built from source as part of the build. No system dev packages needed, on any platform.

Rendering runs on SDL_GPU: Vulkan or Direct3D 12 on Windows, Vulkan on Linux, Metal on macOS.

## Credits

- **Quake**
  - id Software
- **ZQuake**
  - Tonik
- **FuhQuake**
  - fuh
- **Fodquake**
  - bigfoot
  - Tuna
  - Jogi
- **classicQ**
  - mg

### Tip of the hat

**[ezQuake](https://ezquake.com/)** - reference for modern QuakeWorld compatibility and optimizations. For feature-rich gameplay with modern graphical options, ezQuake is *the* recommended client for most users.

The entire [QuakeWorld community](https://quake.world/) co-created the clients above through code, maps, mods, and graphics. If you feel you've been left out, please create an issue and you'll be added to the credits.


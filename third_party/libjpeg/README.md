# third_party/libjpeg

Vendored Zig build script for [libjpeg](https://www.ijg.org/) (IJG, v9f).

`build.zig` and `build.zig.zon` here are a fork of
[timsurber/libjpeg](https://github.com/timsurber/libjpeg) with two small
patches for Zig 0.16: `addConfigHeader` and `addCSourceFiles` calls moved
from `Compile` to `Compile.root_module`. Both upstream changes are MIT
licensed. See `LICENSE` for the original notice.

The libjpeg C sources themselves are not vendored - Zig fetches
`jpegsrc.v9f.tar.gz` from `https://www.ijg.org/files/` at build time, per
`build.zig.zon`. Those sources are released under the IJG license (see
the `README` file inside the tarball under "LEGAL ISSUES"). Any binary
distribution that links them must satisfy that license.

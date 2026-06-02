# Releasing classicQ

## Versioning

Major versions are tied to Quake anniversaries:

- `v3.x.y` - 30th anniversary (2026-)
- `v4.x.y` - 40th anniversary (2036)
- `v5.x.y` - 50th anniversary (2046)
- ...
- `v10.x.y` - 100th anniversary (2096)

Within a major:

- `v3.0.0` - stable release
- `v3.0.0-rc1` - prerelease, auto-flagged on GitHub
- `v3.0.1` - patch (bug fixes, small tweaks, security fixes)
- `v3.1.0` - minor (new features, refactors, sometimes removals)

## Procedure

1. Bump version:
   - `src/version.h` - `CLASSICQ_VERSION`
   - `assets/macos/Info.plist` - `CFBundleVersion` and `CFBundleShortVersionString`
2. Commit, PR.
3. Signed tag + push:
   ```
   git tag -sS v3.0.1 -m "classicQ v3.0.1"
   git push origin v3.0.1
   ```
4. CI builds and uploads release.

#!/usr/bin/env bash
# Assembles classicQ-<version>-<platform>.zip for every platform from assets/
# and the build artifacts downloaded into artifacts/<binary name>/.
set -euo pipefail

VER=$1

rm -rf assets-base
mkdir -p assets-base
cp -r assets/. assets-base/
rm -f assets-base/classicq-windows-amd64.exe assets-base/classicq-linux-amd64 assets-base/classicq-macos-arm64
rm -f assets-base/classicq-go-* assets-base/classicq.pdb assets-base/SDL3.dll assets-base/libSDL3*
rm -rf assets-base/classicq-macos-arm64.app
cp LICENSE README.md assets-base/

# Windows
BIN="classicq-windows-amd64.exe"
STAGE="classicQ-${VER}-windows-amd64"
rm -rf "$STAGE"
mkdir -p "$STAGE"
cp -r assets-base/. "$STAGE/"
cp "artifacts/${BIN}/${BIN}" "artifacts/${BIN}/SDL3.dll" "$STAGE/"
zip -qr "${STAGE}.zip" "$STAGE"

# Linux
BIN="classicq-linux-amd64"
STAGE="classicQ-${VER}-linux-amd64"
rm -rf "$STAGE"
mkdir -p "$STAGE"
cp -r assets-base/. "$STAGE/"
cp "artifacts/${BIN}/${BIN}" "artifacts/${BIN}/libSDL3.so.0" "$STAGE/"
chmod +x "$STAGE/${BIN}"
zip -qr "${STAGE}.zip" "$STAGE"

# macOS, the .app bundle keeps its layout
STAGE="classicQ-${VER}-macos-arm64"
APP="$STAGE/classicq-macos-arm64.app"
rm -rf "$STAGE"
mkdir -p "$STAGE" "$APP"
cp -r assets-base/. "$STAGE/"
cp -r artifacts/classicq-macos-arm64/. "$APP/"
chmod +x "$APP/Contents/MacOS/classicq-macos-arm64"
zip -qry "${STAGE}.zip" "$STAGE"

ls -l classicQ-*.zip

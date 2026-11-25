// Package classicq embeds the compiled shaders.
package classicq

import "embed"

// Compiled shader blobs.
//
//go:embed src/shaders/compiled/*.spv src/shaders/compiled/*.dxil src/shaders/compiled/*.msl
var Shaders embed.FS

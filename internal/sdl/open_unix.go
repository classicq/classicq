//go:build linux || darwin

package sdl

import (
	"runtime"

	"github.com/ebitengine/purego"
)

var libName = map[string]string{"linux": "libSDL3.so.0", "darwin": "libSDL3.dylib"}[runtime.GOOS]

func open(path string) (uintptr, error) {
	return purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_GLOBAL)
}

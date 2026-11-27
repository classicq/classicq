// Copyright (C) 2026 classicQ contributors
//
// This program is free software; you can redistribute it and/or
// modify it under the terms of the GNU General Public License
// as published by the Free Software Foundation; either version 2
// of the License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.
//
// See the GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program; if not, write to the Free Software
// Foundation, Inc., 59 Temple Place - Suite 330, Boston, MA  02111-1307, USA.

// Package sdl is a thin purego binding to the SDL3 functions classicQ uses.
// Names follow the C API without the SDL_ prefix; all calls must happen on
// the main thread.
package sdl

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ebitengine/purego"
)

// SDL_VERSIONNUM(3, 4, 0): the oldest runtime the struct layouts were checked against.
const minVersion = 3004000

// Load binds every function to the shared library at path.
func Load(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	lib, err := open(abs)
	if err != nil {
		return fmt.Errorf("sdl: load %s: %w", abs, err)
	}
	purego.RegisterLibFunc(&GetVersion, lib, "SDL_GetVersion")
	if v := GetVersion(); v < minVersion {
		return fmt.Errorf("sdl: %s is SDL %d.%d.%d, need 3.4 or newer", abs, v/1000000, v/1000%1000, v%1000)
	}
	register(lib)
	return nil
}

// DefaultPath is the library next to the executable, or CLASSICQ_SDL3_PATH when set.
func DefaultPath() (string, error) {
	if p := os.Getenv("CLASSICQ_SDL3_PATH"); p != "" {
		return p, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(exe), libName), nil
}

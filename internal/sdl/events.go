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

package sdl

import "unsafe"

// Event mirrors the SDL_Event union: 128 bytes, 8-byte aligned, read through typed views.
type Event struct {
	_   [0]uint64
	raw [128]byte
}

func (e *Event) Type() EventType           { return *(*EventType)(unsafe.Pointer(e)) }
func (e *Event) Common() *CommonEvent      { return (*CommonEvent)(unsafe.Pointer(e)) }
func (e *Event) Window() *WindowEvent      { return (*WindowEvent)(unsafe.Pointer(e)) }
func (e *Event) Key() *KeyboardEvent       { return (*KeyboardEvent)(unsafe.Pointer(e)) }
func (e *Event) Motion() *MouseMotionEvent { return (*MouseMotionEvent)(unsafe.Pointer(e)) }
func (e *Event) Button() *MouseButtonEvent { return (*MouseButtonEvent)(unsafe.Pointer(e)) }
func (e *Event) Wheel() *MouseWheelEvent   { return (*MouseWheelEvent)(unsafe.Pointer(e)) }
func (e *Event) Quit() *QuitEvent          { return (*QuitEvent)(unsafe.Pointer(e)) }

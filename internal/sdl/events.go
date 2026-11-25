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

// SDL 3.4.12 bindings, generated from the headers and kept by hand since.

package sdl

import (
	"github.com/ebitengine/purego"
	"unsafe"
)

// C names without the SDL_ prefix; signatures mirror the C prototypes.
var (
	ClaimWindowForGPUDevice               func(device *GPUDevice, window *Window) bool
	CreateGPUBuffer                       func(device *GPUDevice, createinfo *GPUBufferCreateInfo) *GPUBuffer
	CreateGPUDevice                       func(formatFlags GPUShaderFormat, debugMode bool, name *byte) *GPUDevice
	CreateGPUGraphicsPipeline             func(device *GPUDevice, createinfo *GPUGraphicsPipelineCreateInfo) *GPUGraphicsPipeline
	CreateGPUSampler                      func(device *GPUDevice, createinfo *GPUSamplerCreateInfo) *GPUSampler
	CreateGPUShader                       func(device *GPUDevice, createinfo *GPUShaderCreateInfo) *GPUShader
	CreateGPUTexture                      func(device *GPUDevice, createinfo *GPUTextureCreateInfo) *GPUTexture
	CreateGPUTransferBuffer               func(device *GPUDevice, createinfo *GPUTransferBufferCreateInfo) *GPUTransferBuffer
	CreateSurfaceFrom                     func(width int32, height int32, format PixelFormat, pixels unsafe.Pointer, pitch int32) *Surface
	CreateWindow                          func(title string, w int32, h int32, flags WindowFlags) *Window
	DestroyAudioStream                    func(stream *AudioStream)
	DestroyGPUDevice                      func(device *GPUDevice)
	DestroySurface                        func(surface *Surface)
	DestroyWindow                         func(window *Window)
	DownloadFromGPUTexture                func(copyPass *GPUCopyPass, source *GPUTextureRegion, destination *GPUTextureTransferInfo)
	GPUTextureSupportsFormat              func(device *GPUDevice, format GPUTextureFormat, type_ GPUTextureType, usage GPUTextureUsageFlags) bool
	GenerateMipmapsForGPUTexture          func(commandBuffer *GPUCommandBuffer, texture *GPUTexture)
	GetAudioDeviceFormat                  func(devid AudioDeviceID, spec *AudioSpec, sampleFrames *int32) bool
	GetAudioStreamDevice                  func(stream *AudioStream) AudioDeviceID
	GetAudioStreamQueued                  func(stream *AudioStream) int32
	GetClipboardText                      func() *byte
	GetCurrentDisplayMode                 func(displayID DisplayID) *DisplayMode
	GetError                              func() string
	GetGPUDeviceDriver                    func(device *GPUDevice) string
	GetGPUShaderFormats                   func(device *GPUDevice) GPUShaderFormat
	GetGPUSwapchainTextureFormat          func(device *GPUDevice, window *Window) GPUTextureFormat
	GetPrimaryDisplay                     func() DisplayID
	GetVersion                            func() int32
	GetWindowDisplayScale                 func(window *Window) float32
	GetWindowPixelDensity                 func(window *Window) float32
	GetWindowSizeInPixels                 func(window *Window, w *int32, h *int32) bool
	Init                                  func(flags InitFlags) bool
	InitSubSystem                         func(flags InitFlags) bool
	OpenAudioDeviceStream                 func(devid AudioDeviceID, spec *AudioSpec, callback uintptr, userdata unsafe.Pointer) *AudioStream
	PutAudioStreamData                    func(stream *AudioStream, buf unsafe.Pointer, len_ int32) bool
	QuitSubSystem                         func(flags InitFlags)
	ReleaseGPUBuffer                      func(device *GPUDevice, buffer *GPUBuffer)
	ReleaseGPUFence                       func(device *GPUDevice, fence *GPUFence)
	ReleaseGPUGraphicsPipeline            func(device *GPUDevice, graphicsPipeline *GPUGraphicsPipeline)
	ReleaseGPUSampler                     func(device *GPUDevice, sampler *GPUSampler)
	ReleaseGPUShader                      func(device *GPUDevice, shader *GPUShader)
	ReleaseGPUTexture                     func(device *GPUDevice, texture *GPUTexture)
	ReleaseGPUTransferBuffer              func(device *GPUDevice, transferBuffer *GPUTransferBuffer)
	ReleaseWindowFromGPUDevice            func(device *GPUDevice, window *Window)
	ResetHint                             func(name string) bool
	ResumeAudioStreamDevice               func(stream *AudioStream) bool
	SetClipboardText                      func(text string) bool
	SetGPUAllowedFramesInFlight           func(device *GPUDevice, allowedFramesInFlight uint32) bool
	SetGPUSwapchainParameters             func(device *GPUDevice, window *Window, swapchainComposition GPUSwapchainComposition, presentMode GPUPresentMode) bool
	SetHint                               func(name string, value string) bool
	SetMainReady                          func()
	SetWindowIcon                         func(window *Window, icon *Surface) bool
	SetWindowPosition                     func(window *Window, x int32, y int32) bool
	SetWindowRelativeMouseMode            func(window *Window, enabled bool) bool
	SetWindowTitle                        func(window *Window, title string) bool
	ShowSimpleMessageBox                  func(flags MessageBoxFlags, title string, message string, window *Window) bool
	ShowWindow                            func(window *Window) bool
	SubmitGPUCommandBufferAndAcquireFence func(commandBuffer *GPUCommandBuffer) *GPUFence
	WaitForGPUFences                      func(device *GPUDevice, waitAll bool, fences **GPUFence, numFences uint32) bool
	WaitForGPUIdle                        func(device *GPUDevice) bool
	WasInit                               func(flags InitFlags) InitFlags
	WindowSupportsGPUPresentMode          func(device *GPUDevice, window *Window, presentMode GPUPresentMode) bool
	Free                                  func(mem unsafe.Pointer)
)

func register(lib uintptr) {
	purego.RegisterLibFunc(&ClaimWindowForGPUDevice, lib, "SDL_ClaimWindowForGPUDevice")
	purego.RegisterLibFunc(&CreateGPUBuffer, lib, "SDL_CreateGPUBuffer")
	purego.RegisterLibFunc(&CreateGPUDevice, lib, "SDL_CreateGPUDevice")
	purego.RegisterLibFunc(&CreateGPUGraphicsPipeline, lib, "SDL_CreateGPUGraphicsPipeline")
	purego.RegisterLibFunc(&CreateGPUSampler, lib, "SDL_CreateGPUSampler")
	purego.RegisterLibFunc(&CreateGPUShader, lib, "SDL_CreateGPUShader")
	purego.RegisterLibFunc(&CreateGPUTexture, lib, "SDL_CreateGPUTexture")
	purego.RegisterLibFunc(&CreateGPUTransferBuffer, lib, "SDL_CreateGPUTransferBuffer")
	purego.RegisterLibFunc(&CreateSurfaceFrom, lib, "SDL_CreateSurfaceFrom")
	purego.RegisterLibFunc(&CreateWindow, lib, "SDL_CreateWindow")
	purego.RegisterLibFunc(&DestroyAudioStream, lib, "SDL_DestroyAudioStream")
	purego.RegisterLibFunc(&DestroyGPUDevice, lib, "SDL_DestroyGPUDevice")
	purego.RegisterLibFunc(&DestroySurface, lib, "SDL_DestroySurface")
	purego.RegisterLibFunc(&DestroyWindow, lib, "SDL_DestroyWindow")
	purego.RegisterLibFunc(&DownloadFromGPUTexture, lib, "SDL_DownloadFromGPUTexture")
	purego.RegisterLibFunc(&GPUTextureSupportsFormat, lib, "SDL_GPUTextureSupportsFormat")
	purego.RegisterLibFunc(&GenerateMipmapsForGPUTexture, lib, "SDL_GenerateMipmapsForGPUTexture")
	purego.RegisterLibFunc(&GetAudioDeviceFormat, lib, "SDL_GetAudioDeviceFormat")
	purego.RegisterLibFunc(&GetAudioStreamDevice, lib, "SDL_GetAudioStreamDevice")
	purego.RegisterLibFunc(&GetAudioStreamQueued, lib, "SDL_GetAudioStreamQueued")
	purego.RegisterLibFunc(&GetClipboardText, lib, "SDL_GetClipboardText")
	purego.RegisterLibFunc(&GetCurrentDisplayMode, lib, "SDL_GetCurrentDisplayMode")
	purego.RegisterLibFunc(&GetError, lib, "SDL_GetError")
	purego.RegisterLibFunc(&GetGPUDeviceDriver, lib, "SDL_GetGPUDeviceDriver")
	purego.RegisterLibFunc(&GetGPUShaderFormats, lib, "SDL_GetGPUShaderFormats")
	purego.RegisterLibFunc(&GetGPUSwapchainTextureFormat, lib, "SDL_GetGPUSwapchainTextureFormat")
	purego.RegisterLibFunc(&GetPrimaryDisplay, lib, "SDL_GetPrimaryDisplay")
	purego.RegisterLibFunc(&GetVersion, lib, "SDL_GetVersion")
	purego.RegisterLibFunc(&GetWindowDisplayScale, lib, "SDL_GetWindowDisplayScale")
	purego.RegisterLibFunc(&GetWindowPixelDensity, lib, "SDL_GetWindowPixelDensity")
	purego.RegisterLibFunc(&GetWindowSizeInPixels, lib, "SDL_GetWindowSizeInPixels")
	purego.RegisterLibFunc(&Init, lib, "SDL_Init")
	purego.RegisterLibFunc(&InitSubSystem, lib, "SDL_InitSubSystem")
	purego.RegisterLibFunc(&OpenAudioDeviceStream, lib, "SDL_OpenAudioDeviceStream")
	purego.RegisterLibFunc(&PutAudioStreamData, lib, "SDL_PutAudioStreamData")
	purego.RegisterLibFunc(&QuitSubSystem, lib, "SDL_QuitSubSystem")
	purego.RegisterLibFunc(&ReleaseGPUBuffer, lib, "SDL_ReleaseGPUBuffer")
	purego.RegisterLibFunc(&ReleaseGPUFence, lib, "SDL_ReleaseGPUFence")
	purego.RegisterLibFunc(&ReleaseGPUGraphicsPipeline, lib, "SDL_ReleaseGPUGraphicsPipeline")
	purego.RegisterLibFunc(&ReleaseGPUSampler, lib, "SDL_ReleaseGPUSampler")
	purego.RegisterLibFunc(&ReleaseGPUShader, lib, "SDL_ReleaseGPUShader")
	purego.RegisterLibFunc(&ReleaseGPUTexture, lib, "SDL_ReleaseGPUTexture")
	purego.RegisterLibFunc(&ReleaseGPUTransferBuffer, lib, "SDL_ReleaseGPUTransferBuffer")
	purego.RegisterLibFunc(&ReleaseWindowFromGPUDevice, lib, "SDL_ReleaseWindowFromGPUDevice")
	purego.RegisterLibFunc(&ResetHint, lib, "SDL_ResetHint")
	purego.RegisterLibFunc(&ResumeAudioStreamDevice, lib, "SDL_ResumeAudioStreamDevice")
	purego.RegisterLibFunc(&SetClipboardText, lib, "SDL_SetClipboardText")
	purego.RegisterLibFunc(&SetGPUAllowedFramesInFlight, lib, "SDL_SetGPUAllowedFramesInFlight")
	purego.RegisterLibFunc(&SetGPUSwapchainParameters, lib, "SDL_SetGPUSwapchainParameters")
	purego.RegisterLibFunc(&SetHint, lib, "SDL_SetHint")
	purego.RegisterLibFunc(&SetMainReady, lib, "SDL_SetMainReady")
	purego.RegisterLibFunc(&SetWindowIcon, lib, "SDL_SetWindowIcon")
	purego.RegisterLibFunc(&SetWindowPosition, lib, "SDL_SetWindowPosition")
	purego.RegisterLibFunc(&SetWindowRelativeMouseMode, lib, "SDL_SetWindowRelativeMouseMode")
	purego.RegisterLibFunc(&SetWindowTitle, lib, "SDL_SetWindowTitle")
	purego.RegisterLibFunc(&ShowSimpleMessageBox, lib, "SDL_ShowSimpleMessageBox")
	purego.RegisterLibFunc(&ShowWindow, lib, "SDL_ShowWindow")
	purego.RegisterLibFunc(&SubmitGPUCommandBufferAndAcquireFence, lib, "SDL_SubmitGPUCommandBufferAndAcquireFence")
	purego.RegisterLibFunc(&WaitForGPUFences, lib, "SDL_WaitForGPUFences")
	purego.RegisterLibFunc(&WaitForGPUIdle, lib, "SDL_WaitForGPUIdle")
	purego.RegisterLibFunc(&WasInit, lib, "SDL_WasInit")
	purego.RegisterLibFunc(&WindowSupportsGPUPresentMode, lib, "SDL_WindowSupportsGPUPresentMode")
	purego.RegisterLibFunc(&Free, lib, "SDL_free")
	registerFast(lib)
}

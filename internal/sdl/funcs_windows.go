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

import (
	"syscall"
	"unsafe"
)

// The per-frame GPU calls go through syscall.SyscallN. purego.RegisterLibFunc
// dispatches through reflection and forces pointer arguments onto the heap,
// which at a thousand calls per frame kept the garbage collector busy.

var (
	procAcquireGPUCommandBuffer           uintptr
	procBeginGPUCopyPass                  uintptr
	procBeginGPURenderPass                uintptr
	procBindGPUFragmentSamplers           uintptr
	procBindGPUGraphicsPipeline           uintptr
	procBindGPUIndexBuffer                uintptr
	procBindGPUVertexBuffers              uintptr
	procDrawGPUIndexedPrimitives          uintptr
	procDrawGPUPrimitives                 uintptr
	procEndGPUCopyPass                    uintptr
	procEndGPURenderPass                  uintptr
	procMapGPUTransferBuffer              uintptr
	procPollEvent                         uintptr
	procPushGPUFragmentUniformData        uintptr
	procPushGPUVertexUniformData          uintptr
	procSetGPUViewport                    uintptr
	procSubmitGPUCommandBuffer            uintptr
	procUnmapGPUTransferBuffer            uintptr
	procUploadToGPUBuffer                 uintptr
	procUploadToGPUTexture                uintptr
	procWaitAndAcquireGPUSwapchainTexture uintptr
)

func registerFast(lib uintptr) {
	procAcquireGPUCommandBuffer = proc(lib, "SDL_AcquireGPUCommandBuffer")
	procBeginGPUCopyPass = proc(lib, "SDL_BeginGPUCopyPass")
	procBeginGPURenderPass = proc(lib, "SDL_BeginGPURenderPass")
	procBindGPUFragmentSamplers = proc(lib, "SDL_BindGPUFragmentSamplers")
	procBindGPUGraphicsPipeline = proc(lib, "SDL_BindGPUGraphicsPipeline")
	procBindGPUIndexBuffer = proc(lib, "SDL_BindGPUIndexBuffer")
	procBindGPUVertexBuffers = proc(lib, "SDL_BindGPUVertexBuffers")
	procDrawGPUIndexedPrimitives = proc(lib, "SDL_DrawGPUIndexedPrimitives")
	procDrawGPUPrimitives = proc(lib, "SDL_DrawGPUPrimitives")
	procEndGPUCopyPass = proc(lib, "SDL_EndGPUCopyPass")
	procEndGPURenderPass = proc(lib, "SDL_EndGPURenderPass")
	procMapGPUTransferBuffer = proc(lib, "SDL_MapGPUTransferBuffer")
	procPollEvent = proc(lib, "SDL_PollEvent")
	procPushGPUFragmentUniformData = proc(lib, "SDL_PushGPUFragmentUniformData")
	procPushGPUVertexUniformData = proc(lib, "SDL_PushGPUVertexUniformData")
	procSetGPUViewport = proc(lib, "SDL_SetGPUViewport")
	procSubmitGPUCommandBuffer = proc(lib, "SDL_SubmitGPUCommandBuffer")
	procUnmapGPUTransferBuffer = proc(lib, "SDL_UnmapGPUTransferBuffer")
	procUploadToGPUBuffer = proc(lib, "SDL_UploadToGPUBuffer")
	procUploadToGPUTexture = proc(lib, "SDL_UploadToGPUTexture")
	procWaitAndAcquireGPUSwapchainTexture = proc(lib, "SDL_WaitAndAcquireGPUSwapchainTexture")
}

func proc(lib uintptr, name string) uintptr {
	addr, err := syscall.GetProcAddress(syscall.Handle(lib), name)
	if err != nil {
		panic("sdl: " + name + ": " + err.Error())
	}
	return addr
}

func b2u(b bool) uintptr {
	if b {
		return 1
	}
	return 0
}

// Foreign pointers come back in a uintptr; reinterpreting the word avoids
// the integer-to-pointer conversion vet warns about.
func ptr(r uintptr) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&r))
}

func AcquireGPUCommandBuffer(device *GPUDevice) *GPUCommandBuffer {
	r, _, _ := syscall.SyscallN(procAcquireGPUCommandBuffer, uintptr(unsafe.Pointer(device)))
	return (*GPUCommandBuffer)(ptr(r))
}

func BeginGPUCopyPass(commandBuffer *GPUCommandBuffer) *GPUCopyPass {
	r, _, _ := syscall.SyscallN(procBeginGPUCopyPass, uintptr(unsafe.Pointer(commandBuffer)))
	return (*GPUCopyPass)(ptr(r))
}

func BeginGPURenderPass(commandBuffer *GPUCommandBuffer, colorTargetInfos *GPUColorTargetInfo, numColorTargets uint32, depthStencilTargetInfo *GPUDepthStencilTargetInfo) *GPURenderPass {
	r, _, _ := syscall.SyscallN(procBeginGPURenderPass, uintptr(unsafe.Pointer(commandBuffer)), uintptr(unsafe.Pointer(colorTargetInfos)), uintptr(numColorTargets), uintptr(unsafe.Pointer(depthStencilTargetInfo)))
	return (*GPURenderPass)(ptr(r))
}

func BindGPUFragmentSamplers(renderPass *GPURenderPass, firstSlot uint32, textureSamplerBindings *GPUTextureSamplerBinding, numBindings uint32) {
	syscall.SyscallN(procBindGPUFragmentSamplers, uintptr(unsafe.Pointer(renderPass)), uintptr(firstSlot), uintptr(unsafe.Pointer(textureSamplerBindings)), uintptr(numBindings))
}

func BindGPUGraphicsPipeline(renderPass *GPURenderPass, graphicsPipeline *GPUGraphicsPipeline) {
	syscall.SyscallN(procBindGPUGraphicsPipeline, uintptr(unsafe.Pointer(renderPass)), uintptr(unsafe.Pointer(graphicsPipeline)))
}

func BindGPUIndexBuffer(renderPass *GPURenderPass, binding *GPUBufferBinding, indexElementSize GPUIndexElementSize) {
	syscall.SyscallN(procBindGPUIndexBuffer, uintptr(unsafe.Pointer(renderPass)), uintptr(unsafe.Pointer(binding)), uintptr(indexElementSize))
}

func BindGPUVertexBuffers(renderPass *GPURenderPass, firstSlot uint32, bindings *GPUBufferBinding, numBindings uint32) {
	syscall.SyscallN(procBindGPUVertexBuffers, uintptr(unsafe.Pointer(renderPass)), uintptr(firstSlot), uintptr(unsafe.Pointer(bindings)), uintptr(numBindings))
}

func DrawGPUIndexedPrimitives(renderPass *GPURenderPass, numIndices uint32, numInstances uint32, firstIndex uint32, vertexOffset int32, firstInstance uint32) {
	syscall.SyscallN(procDrawGPUIndexedPrimitives, uintptr(unsafe.Pointer(renderPass)), uintptr(numIndices), uintptr(numInstances), uintptr(firstIndex), uintptr(vertexOffset), uintptr(firstInstance))
}

func DrawGPUPrimitives(renderPass *GPURenderPass, numVertices uint32, numInstances uint32, firstVertex uint32, firstInstance uint32) {
	syscall.SyscallN(procDrawGPUPrimitives, uintptr(unsafe.Pointer(renderPass)), uintptr(numVertices), uintptr(numInstances), uintptr(firstVertex), uintptr(firstInstance))
}

func EndGPUCopyPass(copyPass *GPUCopyPass) {
	syscall.SyscallN(procEndGPUCopyPass, uintptr(unsafe.Pointer(copyPass)))
}

func EndGPURenderPass(renderPass *GPURenderPass) {
	syscall.SyscallN(procEndGPURenderPass, uintptr(unsafe.Pointer(renderPass)))
}

func MapGPUTransferBuffer(device *GPUDevice, transferBuffer *GPUTransferBuffer, cycle bool) unsafe.Pointer {
	r, _, _ := syscall.SyscallN(procMapGPUTransferBuffer, uintptr(unsafe.Pointer(device)), uintptr(unsafe.Pointer(transferBuffer)), b2u(cycle))
	return ptr(r)
}

func PollEvent(event *Event) bool {
	r, _, _ := syscall.SyscallN(procPollEvent, uintptr(unsafe.Pointer(event)))
	return byte(r) != 0
}

func PushGPUFragmentUniformData(commandBuffer *GPUCommandBuffer, slotIndex uint32, data unsafe.Pointer, length uint32) {
	syscall.SyscallN(procPushGPUFragmentUniformData, uintptr(unsafe.Pointer(commandBuffer)), uintptr(slotIndex), uintptr(data), uintptr(length))
}

func PushGPUVertexUniformData(commandBuffer *GPUCommandBuffer, slotIndex uint32, data unsafe.Pointer, length uint32) {
	syscall.SyscallN(procPushGPUVertexUniformData, uintptr(unsafe.Pointer(commandBuffer)), uintptr(slotIndex), uintptr(data), uintptr(length))
}

func SetGPUViewport(renderPass *GPURenderPass, viewport *GPUViewport) {
	syscall.SyscallN(procSetGPUViewport, uintptr(unsafe.Pointer(renderPass)), uintptr(unsafe.Pointer(viewport)))
}

func SubmitGPUCommandBuffer(commandBuffer *GPUCommandBuffer) bool {
	r, _, _ := syscall.SyscallN(procSubmitGPUCommandBuffer, uintptr(unsafe.Pointer(commandBuffer)))
	return byte(r) != 0
}

func UnmapGPUTransferBuffer(device *GPUDevice, transferBuffer *GPUTransferBuffer) {
	syscall.SyscallN(procUnmapGPUTransferBuffer, uintptr(unsafe.Pointer(device)), uintptr(unsafe.Pointer(transferBuffer)))
}

func UploadToGPUBuffer(copyPass *GPUCopyPass, source *GPUTransferBufferLocation, destination *GPUBufferRegion, cycle bool) {
	syscall.SyscallN(procUploadToGPUBuffer, uintptr(unsafe.Pointer(copyPass)), uintptr(unsafe.Pointer(source)), uintptr(unsafe.Pointer(destination)), b2u(cycle))
}

func UploadToGPUTexture(copyPass *GPUCopyPass, source *GPUTextureTransferInfo, destination *GPUTextureRegion, cycle bool) {
	syscall.SyscallN(procUploadToGPUTexture, uintptr(unsafe.Pointer(copyPass)), uintptr(unsafe.Pointer(source)), uintptr(unsafe.Pointer(destination)), b2u(cycle))
}

func WaitAndAcquireGPUSwapchainTexture(commandBuffer *GPUCommandBuffer, window *Window, swapchainTexture **GPUTexture, swapchainTextureWidth *uint32, swapchainTextureHeight *uint32) bool {
	r, _, _ := syscall.SyscallN(procWaitAndAcquireGPUSwapchainTexture, uintptr(unsafe.Pointer(commandBuffer)), uintptr(unsafe.Pointer(window)), uintptr(unsafe.Pointer(swapchainTexture)), uintptr(unsafe.Pointer(swapchainTextureWidth)), uintptr(unsafe.Pointer(swapchainTextureHeight)))
	return byte(r) != 0
}

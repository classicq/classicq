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

//go:build !windows

package sdl

import (
	"unsafe"

	"github.com/ebitengine/purego"
)

// Per-frame GPU calls; funcs_windows.go binds them without purego.

var (
	AcquireGPUCommandBuffer           func(device *GPUDevice) *GPUCommandBuffer
	BeginGPUCopyPass                  func(commandBuffer *GPUCommandBuffer) *GPUCopyPass
	BeginGPURenderPass                func(commandBuffer *GPUCommandBuffer, colorTargetInfos *GPUColorTargetInfo, numColorTargets uint32, depthStencilTargetInfo *GPUDepthStencilTargetInfo) *GPURenderPass
	BindGPUFragmentSamplers           func(renderPass *GPURenderPass, firstSlot uint32, textureSamplerBindings *GPUTextureSamplerBinding, numBindings uint32)
	BindGPUGraphicsPipeline           func(renderPass *GPURenderPass, graphicsPipeline *GPUGraphicsPipeline)
	BindGPUIndexBuffer                func(renderPass *GPURenderPass, binding *GPUBufferBinding, indexElementSize GPUIndexElementSize)
	BindGPUVertexBuffers              func(renderPass *GPURenderPass, firstSlot uint32, bindings *GPUBufferBinding, numBindings uint32)
	DrawGPUIndexedPrimitives          func(renderPass *GPURenderPass, numIndices uint32, numInstances uint32, firstIndex uint32, vertexOffset int32, firstInstance uint32)
	DrawGPUPrimitives                 func(renderPass *GPURenderPass, numVertices uint32, numInstances uint32, firstVertex uint32, firstInstance uint32)
	EndGPUCopyPass                    func(copyPass *GPUCopyPass)
	EndGPURenderPass                  func(renderPass *GPURenderPass)
	MapGPUTransferBuffer              func(device *GPUDevice, transferBuffer *GPUTransferBuffer, cycle bool) unsafe.Pointer
	PollEvent                         func(event *Event) bool
	PushGPUFragmentUniformData        func(commandBuffer *GPUCommandBuffer, slotIndex uint32, data unsafe.Pointer, length uint32)
	PushGPUVertexUniformData          func(commandBuffer *GPUCommandBuffer, slotIndex uint32, data unsafe.Pointer, length uint32)
	SetGPUViewport                    func(renderPass *GPURenderPass, viewport *GPUViewport)
	SubmitGPUCommandBuffer            func(commandBuffer *GPUCommandBuffer) bool
	UnmapGPUTransferBuffer            func(device *GPUDevice, transferBuffer *GPUTransferBuffer)
	UploadToGPUBuffer                 func(copyPass *GPUCopyPass, source *GPUTransferBufferLocation, destination *GPUBufferRegion, cycle bool)
	UploadToGPUTexture                func(copyPass *GPUCopyPass, source *GPUTextureTransferInfo, destination *GPUTextureRegion, cycle bool)
	WaitAndAcquireGPUSwapchainTexture func(commandBuffer *GPUCommandBuffer, window *Window, swapchainTexture **GPUTexture, swapchainTextureWidth *uint32, swapchainTextureHeight *uint32) bool
)

func registerFast(lib uintptr) {
	purego.RegisterLibFunc(&AcquireGPUCommandBuffer, lib, "SDL_AcquireGPUCommandBuffer")
	purego.RegisterLibFunc(&BeginGPUCopyPass, lib, "SDL_BeginGPUCopyPass")
	purego.RegisterLibFunc(&BeginGPURenderPass, lib, "SDL_BeginGPURenderPass")
	purego.RegisterLibFunc(&BindGPUFragmentSamplers, lib, "SDL_BindGPUFragmentSamplers")
	purego.RegisterLibFunc(&BindGPUGraphicsPipeline, lib, "SDL_BindGPUGraphicsPipeline")
	purego.RegisterLibFunc(&BindGPUIndexBuffer, lib, "SDL_BindGPUIndexBuffer")
	purego.RegisterLibFunc(&BindGPUVertexBuffers, lib, "SDL_BindGPUVertexBuffers")
	purego.RegisterLibFunc(&DrawGPUIndexedPrimitives, lib, "SDL_DrawGPUIndexedPrimitives")
	purego.RegisterLibFunc(&DrawGPUPrimitives, lib, "SDL_DrawGPUPrimitives")
	purego.RegisterLibFunc(&EndGPUCopyPass, lib, "SDL_EndGPUCopyPass")
	purego.RegisterLibFunc(&EndGPURenderPass, lib, "SDL_EndGPURenderPass")
	purego.RegisterLibFunc(&MapGPUTransferBuffer, lib, "SDL_MapGPUTransferBuffer")
	purego.RegisterLibFunc(&PollEvent, lib, "SDL_PollEvent")
	purego.RegisterLibFunc(&PushGPUFragmentUniformData, lib, "SDL_PushGPUFragmentUniformData")
	purego.RegisterLibFunc(&PushGPUVertexUniformData, lib, "SDL_PushGPUVertexUniformData")
	purego.RegisterLibFunc(&SetGPUViewport, lib, "SDL_SetGPUViewport")
	purego.RegisterLibFunc(&SubmitGPUCommandBuffer, lib, "SDL_SubmitGPUCommandBuffer")
	purego.RegisterLibFunc(&UnmapGPUTransferBuffer, lib, "SDL_UnmapGPUTransferBuffer")
	purego.RegisterLibFunc(&UploadToGPUBuffer, lib, "SDL_UploadToGPUBuffer")
	purego.RegisterLibFunc(&UploadToGPUTexture, lib, "SDL_UploadToGPUTexture")
	purego.RegisterLibFunc(&WaitAndAcquireGPUSwapchainTexture, lib, "SDL_WaitAndAcquireGPUSwapchainTexture")
}

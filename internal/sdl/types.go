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

// SDL 3.4.12 bindings, generated from the headers and kept by hand since.

package sdl

// Opaque SDL objects, only ever handled through pointers.
type (
	AudioStream         struct{ _ [0]byte }
	DisplayModeData     struct{ _ [0]byte }
	GPUBuffer           struct{ _ [0]byte }
	GPUCommandBuffer    struct{ _ [0]byte }
	GPUCopyPass         struct{ _ [0]byte }
	GPUDevice           struct{ _ [0]byte }
	GPUFence            struct{ _ [0]byte }
	GPUGraphicsPipeline struct{ _ [0]byte }
	GPURenderPass       struct{ _ [0]byte }
	GPUSampler          struct{ _ [0]byte }
	GPUShader           struct{ _ [0]byte }
	GPUTexture          struct{ _ [0]byte }
	GPUTransferBuffer   struct{ _ [0]byte }
	Surface             struct{ _ [0]byte }
	Window              struct{ _ [0]byte }
)

type (
	AudioDeviceID           uint32
	DisplayID               uint32
	GPUBufferUsageFlags     uint32
	GPUColorComponentFlags  uint8
	GPUShaderFormat         uint32
	GPUTextureUsageFlags    uint32
	InitFlags               uint32
	KeyboardID              uint32
	Keycode                 uint32
	Keymod                  uint16
	MessageBoxFlags         uint32
	MouseButtonFlags        uint32
	MouseID                 uint32
	PropertiesID            uint32
	WindowFlags             uint64
	WindowID                uint32
	AudioFormat             uint32
	EventType               uint32
	GPUBlendFactor          uint32
	GPUBlendOp              uint32
	GPUCompareOp            uint32
	GPUCullMode             uint32
	GPUFillMode             uint32
	GPUFilter               uint32
	GPUFrontFace            uint32
	GPUIndexElementSize     uint32
	GPULoadOp               uint32
	GPUPresentMode          uint32
	GPUPrimitiveType        uint32
	GPUSampleCount          uint32
	GPUSamplerAddressMode   uint32
	GPUSamplerMipmapMode    uint32
	GPUShaderStage          uint32
	GPUStencilOp            uint32
	GPUStoreOp              uint32
	GPUSwapchainComposition uint32
	GPUTextureFormat        uint32
	GPUTextureType          uint32
	GPUTransferBufferUsage  uint32
	GPUVertexElementFormat  uint32
	GPUVertexInputRate      uint32
	MouseWheelDirection     uint32
	PixelFormat             uint32
	Scancode                uint32
)

type GPUColorTargetInfo struct {
	Texture             *GPUTexture
	MipLevel            uint32
	LayerOrDepthPlane   uint32
	ClearColor          FColor
	LoadOp              GPULoadOp
	StoreOp             GPUStoreOp
	ResolveTexture      *GPUTexture
	ResolveMipLevel     uint32
	ResolveLayer        uint32
	Cycle               bool
	CycleResolveTexture bool
	Padding1            uint8
	Padding2            uint8
}

type FColor struct {
	R float32
	G float32
	B float32
	A float32
}

type GPUDepthStencilTargetInfo struct {
	Texture        *GPUTexture
	ClearDepth     float32
	LoadOp         GPULoadOp
	StoreOp        GPUStoreOp
	StencilLoadOp  GPULoadOp
	StencilStoreOp GPUStoreOp
	Cycle          bool
	ClearStencil   uint8
	MipLevel       uint8
	Layer          uint8
}

type GPUTextureSamplerBinding struct {
	Texture *GPUTexture
	Sampler *GPUSampler
}

type GPUBufferBinding struct {
	Buffer *GPUBuffer
	Offset uint32
}

type GPUBufferCreateInfo struct {
	Usage GPUBufferUsageFlags
	Size  uint32
	Props PropertiesID
}

type GPUGraphicsPipelineCreateInfo struct {
	VertexShader      *GPUShader
	FragmentShader    *GPUShader
	VertexInputState  GPUVertexInputState
	PrimitiveType     GPUPrimitiveType
	RasterizerState   GPURasterizerState
	MultisampleState  GPUMultisampleState
	DepthStencilState GPUDepthStencilState
	TargetInfo        GPUGraphicsPipelineTargetInfo
	Props             PropertiesID
}

type GPUVertexInputState struct {
	VertexBufferDescriptions *GPUVertexBufferDescription
	NumVertexBuffers         uint32
	VertexAttributes         *GPUVertexAttribute
	NumVertexAttributes      uint32
}

type GPUVertexBufferDescription struct {
	Slot             uint32
	Pitch            uint32
	InputRate        GPUVertexInputRate
	InstanceStepRate uint32
}

type GPUVertexAttribute struct {
	Location   uint32
	BufferSlot uint32
	Format     GPUVertexElementFormat
	Offset     uint32
}

type GPURasterizerState struct {
	FillMode                GPUFillMode
	CullMode                GPUCullMode
	FrontFace               GPUFrontFace
	DepthBiasConstantFactor float32
	DepthBiasClamp          float32
	DepthBiasSlopeFactor    float32
	EnableDepthBias         bool
	EnableDepthClip         bool
	Padding1                uint8
	Padding2                uint8
}

type GPUMultisampleState struct {
	SampleCount           GPUSampleCount
	SampleMask            uint32
	EnableMask            bool
	EnableAlphaToCoverage bool
	Padding2              uint8
	Padding3              uint8
}

type GPUDepthStencilState struct {
	CompareOp         GPUCompareOp
	BackStencilState  GPUStencilOpState
	FrontStencilState GPUStencilOpState
	CompareMask       uint8
	WriteMask         uint8
	EnableDepthTest   bool
	EnableDepthWrite  bool
	EnableStencilTest bool
	Padding1          uint8
	Padding2          uint8
	Padding3          uint8
}

type GPUStencilOpState struct {
	FailOp      GPUStencilOp
	PassOp      GPUStencilOp
	DepthFailOp GPUStencilOp
	CompareOp   GPUCompareOp
}

type GPUGraphicsPipelineTargetInfo struct {
	ColorTargetDescriptions *GPUColorTargetDescription
	NumColorTargets         uint32
	DepthStencilFormat      GPUTextureFormat
	HasDepthStencilTarget   bool
	Padding1                uint8
	Padding2                uint8
	Padding3                uint8
}

type GPUColorTargetDescription struct {
	Format     GPUTextureFormat
	BlendState GPUColorTargetBlendState
}

type GPUColorTargetBlendState struct {
	SrcColorBlendfactor  GPUBlendFactor
	DstColorBlendfactor  GPUBlendFactor
	ColorBlendOp         GPUBlendOp
	SrcAlphaBlendfactor  GPUBlendFactor
	DstAlphaBlendfactor  GPUBlendFactor
	AlphaBlendOp         GPUBlendOp
	ColorWriteMask       GPUColorComponentFlags
	EnableBlend          bool
	EnableColorWriteMask bool
	Padding1             uint8
	Padding2             uint8
}

type GPUSamplerCreateInfo struct {
	MinFilter        GPUFilter
	MagFilter        GPUFilter
	MipmapMode       GPUSamplerMipmapMode
	AddressModeU     GPUSamplerAddressMode
	AddressModeV     GPUSamplerAddressMode
	AddressModeW     GPUSamplerAddressMode
	MipLodBias       float32
	MaxAnisotropy    float32
	CompareOp        GPUCompareOp
	MinLod           float32
	MaxLod           float32
	EnableAnisotropy bool
	EnableCompare    bool
	Padding1         uint8
	Padding2         uint8
	Props            PropertiesID
}

type GPUShaderCreateInfo struct {
	CodeSize           uintptr
	Code               *uint8
	Entrypoint         *byte
	Format             GPUShaderFormat
	Stage              GPUShaderStage
	NumSamplers        uint32
	NumStorageTextures uint32
	NumStorageBuffers  uint32
	NumUniformBuffers  uint32
	Props              PropertiesID
}

type GPUTextureCreateInfo struct {
	Type              GPUTextureType
	Format            GPUTextureFormat
	Usage             GPUTextureUsageFlags
	Width             uint32
	Height            uint32
	LayerCountOrDepth uint32
	NumLevels         uint32
	SampleCount       GPUSampleCount
	Props             PropertiesID
}

type GPUTransferBufferCreateInfo struct {
	Usage GPUTransferBufferUsage
	Size  uint32
	Props PropertiesID
}

type GPUTextureRegion struct {
	Texture  *GPUTexture
	MipLevel uint32
	Layer    uint32
	X        uint32
	Y        uint32
	Z        uint32
	W        uint32
	H        uint32
	D        uint32
}

type GPUTextureTransferInfo struct {
	TransferBuffer *GPUTransferBuffer
	Offset         uint32
	PixelsPerRow   uint32
	RowsPerLayer   uint32
}

type AudioSpec struct {
	Format   AudioFormat
	Channels int32
	Freq     int32
}

type DisplayMode struct {
	DisplayID              DisplayID
	Format                 PixelFormat
	W                      int32
	H                      int32
	PixelDensity           float32
	RefreshRate            float32
	RefreshRateNumerator   int32
	RefreshRateDenominator int32
	Internal               *DisplayModeData
}

type GPUViewport struct {
	X        float32
	Y        float32
	W        float32
	H        float32
	MinDepth float32
	MaxDepth float32
}

type GPUTransferBufferLocation struct {
	TransferBuffer *GPUTransferBuffer
	Offset         uint32
}

type GPUBufferRegion struct {
	Buffer *GPUBuffer
	Offset uint32
	Size   uint32
}

type CommonEvent struct {
	Type      uint32
	Reserved  uint32
	Timestamp uint64
}

type WindowEvent struct {
	Type      EventType
	Reserved  uint32
	Timestamp uint64
	WindowID  WindowID
	Data1     int32
	Data2     int32
}

type KeyboardEvent struct {
	Type      EventType
	Reserved  uint32
	Timestamp uint64
	WindowID  WindowID
	Which     KeyboardID
	Scancode  Scancode
	Key       Keycode
	Mod       Keymod
	Raw       uint16
	Down      bool
	Repeat    bool
}

type MouseMotionEvent struct {
	Type      EventType
	Reserved  uint32
	Timestamp uint64
	WindowID  WindowID
	Which     MouseID
	State     MouseButtonFlags
	X         float32
	Y         float32
	Xrel      float32
	Yrel      float32
}

type MouseButtonEvent struct {
	Type      EventType
	Reserved  uint32
	Timestamp uint64
	WindowID  WindowID
	Which     MouseID
	Button    uint8
	Down      bool
	Clicks    uint8
	Padding   uint8
	X         float32
	Y         float32
}

type MouseWheelEvent struct {
	Type      EventType
	Reserved  uint32
	Timestamp uint64
	WindowID  WindowID
	Which     MouseID
	X         float32
	Y         float32
	Direction MouseWheelDirection
	MouseX    float32
	MouseY    float32
	IntegerX  int32
	IntegerY  int32
}

type QuitEvent struct {
	Type      EventType
	Reserved  uint32
	Timestamp uint64
}

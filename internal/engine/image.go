package engine

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
)

// image.c. PNG goes through image/png instead of a dynamically loaded
// libpng; JPEG (USE_JPEG=0 in the C build) is left out.

const IMAGE_MAX_DIMENSIONS = 4096

var image_png_compression_level = cvar_t{name: "image_png_compression_level", string: "1"}

/***************************** IMAGE RESAMPLING ******************************/

func Image_Resample32LerpLine(in []byte, out []byte, inwidth, outwidth int) {
	ip, op := 0, 0
	oldx := 0

	fstep := int(float32(inwidth) * 65536.0 / float32(outwidth))
	endx := inwidth - 1
	for j, f := 0, 0; j < outwidth; j, f = j+1, f+fstep {
		xi := f >> 16
		if xi != oldx {
			ip += (xi - oldx) * 4
			oldx = xi
		}
		if xi < endx {
			lerp := f & 0xFFFF
			out[op+0] = byte((((int(in[ip+4]) - int(in[ip+0])) * lerp) >> 16) + int(in[ip+0]))
			out[op+1] = byte((((int(in[ip+5]) - int(in[ip+1])) * lerp) >> 16) + int(in[ip+1]))
			out[op+2] = byte((((int(in[ip+6]) - int(in[ip+2])) * lerp) >> 16) + int(in[ip+2]))
			out[op+3] = byte((((int(in[ip+7]) - int(in[ip+3])) * lerp) >> 16) + int(in[ip+3]))
		} else {
			out[op+0] = in[ip+0]
			out[op+1] = in[ip+1]
			out[op+2] = in[ip+2]
			out[op+3] = in[ip+3]
		}
		op += 4
	}
}

func Image_Resample24LerpLine(in []byte, out []byte, inwidth, outwidth int) {
	ip, op := 0, 0
	oldx := 0

	fstep := int(float32(inwidth) * 65536.0 / float32(outwidth))
	endx := inwidth - 1
	for j, f := 0, 0; j < outwidth; j, f = j+1, f+fstep {
		xi := f >> 16
		if xi != oldx {
			ip += (xi - oldx) * 3
			oldx = xi
		}
		if xi < endx {
			lerp := f & 0xFFFF
			out[op+0] = byte((((int(in[ip+3]) - int(in[ip+0])) * lerp) >> 16) + int(in[ip+0]))
			out[op+1] = byte((((int(in[ip+4]) - int(in[ip+1])) * lerp) >> 16) + int(in[ip+1]))
			out[op+2] = byte((((int(in[ip+5]) - int(in[ip+2])) * lerp) >> 16) + int(in[ip+2]))
		} else {
			out[op+0] = in[ip+0]
			out[op+1] = in[ip+1]
			out[op+2] = in[ip+2]
		}
		op += 3
	}
}

// Vertical pass shared by both pixel sizes: the C code unrolls the per
// byte lerp four pixels at a time, which comes to the same bytes.
func image_resampleRows(indata []byte, inwidth, inheight int, outdata []byte, outwidth, outheight, bpp int,
	lerpLine func(in, out []byte, inwidth, outwidth int)) {
	inrowbytes := inwidth * bpp
	outrowbytes := outwidth * bpp
	endy := inheight - 1

	op := 0
	fstep := int(float32(inheight) * 65536.0 / float32(outheight))

	row1 := make([]byte, outrowbytes)
	row2 := make([]byte, outrowbytes)
	inrow := 0
	oldy := 0
	lerpLine(indata[inrow:], row1, inwidth, outwidth)
	lerpLine(indata[inrow+inrowbytes:], row2, inwidth, outwidth)
	for i, f := 0, 0; i < outheight; i, f = i+1, f+fstep {
		yi := f >> 16
		if yi < endy {
			lerp := f & 0xFFFF
			if yi != oldy {
				inrow = inrowbytes * yi
				if yi == oldy+1 {
					copy(row1, row2)
				} else {
					lerpLine(indata[inrow:], row1, inwidth, outwidth)
				}
				lerpLine(indata[inrow+inrowbytes:], row2, inwidth, outwidth)
				oldy = yi
			}
			for k := 0; k < outrowbytes; k++ {
				r := int(row1[k])
				outdata[op+k] = byte((((int(row2[k]) - r) * lerp) >> 16) + r)
			}
			op += outrowbytes
		} else {
			if yi != oldy {
				inrow = inrowbytes * yi
				if yi == oldy+1 {
					copy(row1, row2)
				} else {
					lerpLine(indata[inrow:], row1, inwidth, outwidth)
				}
				oldy = yi
			}
			copy(outdata[op:op+outrowbytes], row1)
			op += outrowbytes
		}
	}
}

func Image_Resample32(indata []byte, inwidth, inheight int, outdata []byte, outwidth, outheight, quality int) {
	if quality != 0 {
		image_resampleRows(indata, inwidth, inheight, outdata, outwidth, outheight, 4, Image_Resample32LerpLine)
		return
	}

	op := 0
	fracstep := uint32(inwidth) * 0x10000 / uint32(outwidth)
	for i := 0; i < outheight; i++ {
		inrow := inwidth * 4 * (i * inheight / outheight)
		frac := fracstep >> 1
		for j := 0; j < outwidth; j++ {
			p := inrow + int(frac>>16)*4
			copy(outdata[op:op+4], indata[p:p+4])
			frac += fracstep
			op += 4
		}
	}
}

func Image_Resample24(indata []byte, inwidth, inheight int, outdata []byte, outwidth, outheight, quality int) {
	if quality != 0 {
		image_resampleRows(indata, inwidth, inheight, outdata, outwidth, outheight, 3, Image_Resample24LerpLine)
		return
	}

	op := 0
	fracstep := uint32(inwidth) * 0x10000 / uint32(outwidth)
	for i := 0; i < outheight; i++ {
		inrow := inwidth * 3 * (i * inheight / outheight)
		frac := fracstep >> 1
		for j := 0; j < outwidth; j++ {
			p := inrow + int(frac>>16)*3
			copy(outdata[op:op+3], indata[p:p+3])
			frac += fracstep
			op += 3
		}
	}
}

func Image_Resample(indata []byte, inwidth, inheight int, outdata []byte, outwidth, outheight, bpp, quality int) {
	if bpp == 4 {
		Image_Resample32(indata, inwidth, inheight, outdata, outwidth, outheight, quality)
	} else if bpp == 3 {
		Image_Resample24(indata, inwidth, inheight, outdata, outwidth, outheight, quality)
	} else {
		Sys_Error("Image_Resample: unsupported bpp (%d)", bpp)
	}
}

func Image_MipReduce(in []byte, out []byte, width, height *int, bpp int) {
	ip, op := 0, 0

	nextrow := *width * bpp

	if *width > 1 {
		*width >>= 1
		if *height > 1 {
			*height >>= 1
			if bpp == 4 {
				for y := 0; y < *height; y++ {
					for x := 0; x < *width; x++ {
						out[op+0] = byte((int(in[ip+0]) + int(in[ip+4]) + int(in[ip+nextrow]) + int(in[ip+nextrow+4])) >> 2)
						out[op+1] = byte((int(in[ip+1]) + int(in[ip+5]) + int(in[ip+nextrow+1]) + int(in[ip+nextrow+5])) >> 2)
						out[op+2] = byte((int(in[ip+2]) + int(in[ip+6]) + int(in[ip+nextrow+2]) + int(in[ip+nextrow+6])) >> 2)
						out[op+3] = byte((int(in[ip+3]) + int(in[ip+7]) + int(in[ip+nextrow+3]) + int(in[ip+nextrow+7])) >> 2)
						op += 4
						ip += 8
					}
					ip += nextrow
				}
			} else if bpp == 3 {
				for y := 0; y < *height; y++ {
					for x := 0; x < *width; x++ {
						out[op+0] = byte((int(in[ip+0]) + int(in[ip+3]) + int(in[ip+nextrow]) + int(in[ip+nextrow+3])) >> 2)
						out[op+1] = byte((int(in[ip+1]) + int(in[ip+4]) + int(in[ip+nextrow+1]) + int(in[ip+nextrow+4])) >> 2)
						out[op+2] = byte((int(in[ip+2]) + int(in[ip+5]) + int(in[ip+nextrow+2]) + int(in[ip+nextrow+5])) >> 2)
						op += 3
						ip += 6
					}
					ip += nextrow
				}
			} else {
				Sys_Error("Image_MipReduce: unsupported bpp (%d)", bpp)
			}
		} else {
			if bpp == 4 {
				for y := 0; y < *height; y++ {
					for x := 0; x < *width; x++ {
						out[op+0] = byte((int(in[ip+0]) + int(in[ip+4])) >> 1)
						out[op+1] = byte((int(in[ip+1]) + int(in[ip+5])) >> 1)
						out[op+2] = byte((int(in[ip+2]) + int(in[ip+6])) >> 1)
						out[op+3] = byte((int(in[ip+3]) + int(in[ip+7])) >> 1)
						op += 4
						ip += 8
					}
				}
			} else if bpp == 3 {
				for y := 0; y < *height; y++ {
					for x := 0; x < *width; x++ {
						out[op+0] = byte((int(in[ip+0]) + int(in[ip+3])) >> 1)
						out[op+1] = byte((int(in[ip+1]) + int(in[ip+4])) >> 1)
						out[op+2] = byte((int(in[ip+2]) + int(in[ip+5])) >> 1)
						op += 3
						ip += 6
					}
				}
			} else {
				Sys_Error("Image_MipReduce: unsupported bpp (%d)", bpp)
			}
		}
	} else if *height > 1 {
		*height >>= 1
		if bpp == 4 {
			for y := 0; y < *height; y++ {
				for x := 0; x < *width; x++ {
					out[op+0] = byte((int(in[ip+0]) + int(in[ip+nextrow])) >> 1)
					out[op+1] = byte((int(in[ip+1]) + int(in[ip+nextrow+1])) >> 1)
					out[op+2] = byte((int(in[ip+2]) + int(in[ip+nextrow+2])) >> 1)
					out[op+3] = byte((int(in[ip+3]) + int(in[ip+nextrow+3])) >> 1)
					op += 4
					ip += 4
				}
				ip += nextrow
			}
		} else if bpp == 3 {
			for y := 0; y < *height; y++ {
				for x := 0; x < *width; x++ {
					out[op+0] = byte((int(in[ip+0]) + int(in[ip+nextrow])) >> 1)
					out[op+1] = byte((int(in[ip+1]) + int(in[ip+nextrow+1])) >> 1)
					out[op+2] = byte((int(in[ip+2]) + int(in[ip+nextrow+2])) >> 1)
					op += 3
					ip += 3
				}
				ip += nextrow
			}
		} else {
			Sys_Error("Image_MipReduce: unsupported bpp (%d)", bpp)
		}
	} else {
		Sys_Error("Image_MipReduce: Input texture has dimensions %dx%d", *width, *height)
	}
}

/************************************ PNG ************************************/

// Reads the whole file the way the C loaders do: the open file is
// positioned at the start of the entry and com_filesize is its length.
func image_readFile(fin *os.File, filename string) []byte {
	if fin == nil {
		var flen int
		if fin, flen = FS_FOpenFile(filename); flen == -1 {
			return nil
		}
	}
	buf := make([]byte, com_filesize)
	n, _ := io.ReadFull(fin, buf)
	fin.Close()
	return buf[:n]
}

var png_signature = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}

func Image_LoadPNG(fin *os.File, filename string, matchwidth, matchheight int, imagewidth, imageheight *uint) []byte {
	buf := image_readFile(fin, filename)
	if buf == nil {
		return nil
	}

	if !bytes.HasPrefix(buf, png_signature) {
		Com_DPrintf("Invalid PNG image %s\n", COM_SkipPath(filename))
		return nil
	}

	cfg, err := png.DecodeConfig(bytes.NewReader(buf))
	if err != nil {
		return nil
	}
	width, height := cfg.Width, cfg.Height

	if width > IMAGE_MAX_DIMENSIONS || height > IMAGE_MAX_DIMENSIONS {
		Com_DPrintf("PNG image %s exceeds maximum supported dimensions\n", COM_SkipPath(filename))
		return nil
	}

	if (matchwidth != 0 && width != matchwidth) || (matchheight != 0 && height != matchheight) {
		return nil
	}

	img, err := png.Decode(bytes.NewReader(buf))
	if err != nil {
		return nil
	}

	// libpng is set up to deliver 8 bit RGBA with straight alpha whatever
	// the file holds
	data := make([]byte, width*height*4)
	switch im := img.(type) {
	case *image.NRGBA:
		for y := 0; y < height; y++ {
			copy(data[y*width*4:(y+1)*width*4], im.Pix[y*im.Stride:y*im.Stride+width*4])
		}
	case *image.RGBA: // opaque truecolor, so premultiplied makes no difference
		for y := 0; y < height; y++ {
			copy(data[y*width*4:(y+1)*width*4], im.Pix[y*im.Stride:y*im.Stride+width*4])
		}
	default:
		b := img.Bounds()
		i := 0
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
				data[i+0] = c.R
				data[i+1] = c.G
				data[i+2] = c.B
				data[i+3] = c.A
				i += 4
			}
		}
	}

	*imagewidth = uint(width)
	*imageheight = uint(height)
	return data
}

// zlib levels 0..9 onto the four the standard encoder knows
func image_pngLevel(compression int) png.CompressionLevel {
	switch c := bound(0, compression, 9); {
	case c == 0:
		return png.NoCompression
	case c <= 3:
		return png.BestSpeed
	case c <= 6:
		return png.DefaultCompression
	default:
		return png.BestCompression
	}
}

func image_createFile(filename string) *os.File {
	name := va("%s/%s", com_basedir, filename)
	fp, err := os.Create(name)
	if err != nil {
		FS_CreatePath(name)
		if fp, err = os.Create(name); err != nil {
			return nil
		}
	}
	return fp
}

// A negative width means the rows run bottom to top, as the screenshot
// code hands them over. C then gets a pointer to the last row and walks
// backwards; here pixels is always the start of the buffer.
func Image_WritePNG(filename string, compression int, pixels []byte, width, height int) bool {
	const bpp = 3

	flipped := width < 0
	if width < 0 {
		width = -width
	}

	fp := image_createFile(filename)
	if fp == nil {
		return false
	}
	defer fp.Close()

	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for i := 0; i < height; i++ {
		row := pixels[i*width*bpp:]
		if flipped {
			row = pixels[(height-1-i)*width*bpp:]
		}
		for x := 0; x < width; x++ {
			img.Pix[i*img.Stride+x*4+0] = row[x*3+0]
			img.Pix[i*img.Stride+x*4+1] = row[x*3+1]
			img.Pix[i*img.Stride+x*4+2] = row[x*3+2]
			img.Pix[i*img.Stride+x*4+3] = 255
		}
	}

	enc := png.Encoder{CompressionLevel: image_pngLevel(compression)}
	return enc.Encode(fp, image_opaque{img}) == nil
}

// Drops the alpha channel so the encoder writes an RGB file like libpng
// does for PNG_COLOR_TYPE_RGB.
type image_opaque struct{ *image.NRGBA }

func (im image_opaque) Opaque() bool { return true }

func Image_WritePNGPLTE(filename string, compression int, pixels []byte, width, height int, palette []byte) bool {
	rowbytes := width

	fp := image_createFile(filename)
	if fp == nil {
		return false
	}
	defer fp.Close()

	pal := make(color.Palette, 256)
	for i := range pal {
		pal[i] = color.NRGBA{palette[i*3], palette[i*3+1], palette[i*3+2], 255}
	}
	img := image.NewPaletted(image.Rect(0, 0, width, height), pal)
	for i := 0; i < height; i++ {
		copy(img.Pix[i*img.Stride:i*img.Stride+width], pixels[i*rowbytes:i*rowbytes+width])
	}

	enc := png.Encoder{CompressionLevel: image_pngLevel(compression)}
	return enc.Encode(fp, img) == nil
}

/************************************ TGA ************************************/

// Definitions for image types
const (
	TGA_MAPPED     = 1  // Uncompressed, color-mapped images
	TGA_MAPPED_RLE = 9  // Runlength encoded color-mapped images
	TGA_RGB        = 2  // Uncompressed, RGB images
	TGA_RGB_RLE    = 10 // Runlength encoded RGB images
	TGA_MONO       = 3  // Uncompressed, black and white images
	TGA_MONO_RLE   = 11 // Compressed, black and white images
)

// Custom definitions to simplify code
const (
	MYTGA_MAPPED = 80
	MYTGA_RGB15  = 81
	MYTGA_RGB24  = 82
	MYTGA_RGB32  = 83
	MYTGA_MONO8  = 84
	MYTGA_MONO16 = 85
)

type TGAHeader_t struct {
	idLength, colormapType, imageType byte
	colormapIndex, colormapLength     uint16
	colormapSize                      byte
	xOrigin, yOrigin, width, height   uint16
	pixelSize, attributes             byte
}

func TGA_upsample15(dest []byte, src []byte, alpha bool) {
	dest[2] = (src[0] & 0x1F) << 3
	dest[1] = (((src[1] & 0x03) << 3) + ((src[0] & 0xE0) >> 5)) << 3
	dest[0] = ((src[1] & 0x7C) >> 2) << 3
	if alpha && src[1]&0x80 == 0 {
		dest[3] = 0
	} else {
		dest[3] = 255
	}
}

func TGA_upsample24(dest []byte, src []byte) {
	dest[2] = src[0]
	dest[1] = src[1]
	dest[0] = src[2]
	dest[3] = 255
}

func TGA_upsample32(dest []byte, src []byte) {
	dest[2] = src[0]
	dest[1] = src[1]
	dest[0] = src[2]
	dest[3] = src[3]
}

func BuffLittleShort(buffer []byte) uint16 {
	return uint16(buffer[1])<<8 | uint16(buffer[0])
}

func Image_LoadTGA(fin *os.File, filename string, matchwidth, matchheight int, imagewidth, imageheight *uint) []byte {
	var header TGAHeader_t
	var rgba [4]byte
	var palette [256 * 4]byte
	var mytype int

	fileBuffer := image_readFile(fin, filename)
	if fileBuffer == nil {
		return nil
	}

	tgaError := func(msg string) []byte {
		if msg != "" {
			Com_DPrintf(msg, COM_SkipPath(filename))
		}
		return nil
	}

	if len(fileBuffer) < 19 {
		return tgaError("")
	}

	header.idLength = fileBuffer[0]
	header.colormapType = fileBuffer[1]
	header.imageType = fileBuffer[2]

	header.colormapIndex = BuffLittleShort(fileBuffer[3:])
	header.colormapLength = BuffLittleShort(fileBuffer[5:])
	header.colormapSize = fileBuffer[7]
	header.xOrigin = BuffLittleShort(fileBuffer[8:])
	header.yOrigin = BuffLittleShort(fileBuffer[10:])
	header.width = BuffLittleShort(fileBuffer[12:])
	header.height = BuffLittleShort(fileBuffer[14:])
	width := int(header.width)
	height := int(header.height)
	header.pixelSize = fileBuffer[16]
	header.attributes = fileBuffer[17]

	if width > IMAGE_MAX_DIMENSIONS || height > IMAGE_MAX_DIMENSIONS || width <= 0 || height <= 0 {
		return tgaError("")
	}
	if (matchwidth != 0 && width != matchwidth) || (matchheight != 0 && height != matchheight) {
		return tgaError("")
	}

	bpp := (int(header.pixelSize) + 7) >> 3
	alphabits := int(header.attributes & 0x0F)
	compressed := header.imageType&0x08 != 0

	in := 18 + int(header.idLength)
	enddata := len(fileBuffer)

	// error check the image type's pixel size
	if header.imageType == TGA_RGB || header.imageType == TGA_RGB_RLE {
		if !(header.pixelSize == 15 || header.pixelSize == 16 || header.pixelSize == 24 || header.pixelSize == 32) {
			return tgaError("Unsupported TGA image %s: Bad pixel size for RGB image\n")
		}
		if header.pixelSize == 24 {
			mytype = MYTGA_RGB24
		} else if header.pixelSize == 32 {
			mytype = MYTGA_RGB32
		} else {
			mytype = MYTGA_RGB15
		}
	} else if header.imageType == TGA_MAPPED || header.imageType == TGA_MAPPED_RLE {
		if header.pixelSize != 8 {
			return tgaError("Unsupported TGA image %s: Bad pixel size for color-mapped image.\n")
		}
		if !(header.colormapSize == 15 || header.colormapSize == 16 || header.colormapSize == 24 || header.colormapSize == 32) {
			return tgaError("Unsupported TGA image %s: Bad colormap size.\n")
		}
		if header.colormapType != 1 || int(header.colormapLength)*4 > len(palette) {
			return tgaError("Unsupported TGA image %s: Bad colormap type and/or length for color-mapped image.\n")
		}

		// read in the palette
		if header.colormapSize == 15 || header.colormapSize == 16 {
			for i := 0; i < int(header.colormapLength); i, in = i+1, in+2 {
				TGA_upsample15(palette[i*4:], fileBuffer[in:], alphabits == 1)
			}
		} else if header.colormapSize == 24 {
			for i := 0; i < int(header.colormapLength); i, in = i+1, in+3 {
				TGA_upsample24(palette[i*4:], fileBuffer[in:])
			}
		} else if header.colormapSize == 32 {
			for i := 0; i < int(header.colormapLength); i, in = i+1, in+4 {
				TGA_upsample32(palette[i*4:], fileBuffer[in:])
			}
		}
		mytype = MYTGA_MAPPED
	} else if header.imageType == TGA_MONO || header.imageType == TGA_MONO_RLE {
		if !(header.pixelSize == 8 || (header.pixelSize == 16 && alphabits == 8)) {
			return tgaError("Unsupported TGA image %s: Bad pixel size for grayscale image.\n")
		}
		if header.pixelSize == 8 {
			mytype = MYTGA_MONO8
		} else {
			mytype = MYTGA_MONO16
		}
	} else {
		return tgaError("Unsupported TGA image %s: Bad image type.\n")
	}

	if header.attributes&0x10 != 0 {
		return tgaError("Unsupported TGA image %s: Pixel data spans right to left.\n")
	}

	data := make([]byte, width*height*4)

	// if bit 5 of attributes isn't set, the image has been stored from bottom to top
	var out, row_inc int
	if header.attributes&0x20 != 0 {
		out = 0
		row_inc = 0
	} else {
		out = (height - 1) * width * 4
		row_inc = -width * 4 * 2
	}

	x, y := 0, 0
	rgba = [4]byte{255, 255, 255, 255}

	for y < height {
		// decoder is mostly the same whether it's compressed or not
		readpixelcount := 0x7FFFFFFF
		runlen := 0x7FFFFFFF
		if compressed && in < enddata {
			runlen = int(fileBuffer[in])
			in++
			// high bit indicates this is an RLE compressed run
			if runlen&0x80 != 0 {
				readpixelcount = 1
			}
			runlen = 1 + (runlen & 0x7F)
		}

		for runlen != 0 && y < height {
			runlen--
			if readpixelcount > 0 {
				readpixelcount--
				rgba = [4]byte{255, 255, 255, 255}

				if in+bpp <= enddata {
					switch mytype {
					case MYTGA_MAPPED:
						for i := 0; i < 4; i++ {
							rgba[i] = palette[int(fileBuffer[in])*4+i]
						}
					case MYTGA_RGB15:
						TGA_upsample15(rgba[:], fileBuffer[in:], alphabits == 1)
					case MYTGA_RGB24:
						TGA_upsample24(rgba[:], fileBuffer[in:])
					case MYTGA_RGB32:
						TGA_upsample32(rgba[:], fileBuffer[in:])
					case MYTGA_MONO8:
						rgba[0] = fileBuffer[in]
						rgba[1] = fileBuffer[in]
						rgba[2] = fileBuffer[in]
					case MYTGA_MONO16:
						rgba[0] = fileBuffer[in]
						rgba[1] = fileBuffer[in]
						rgba[2] = fileBuffer[in]
						rgba[3] = fileBuffer[in+1]
					}
					in += bpp
				}
			}
			copy(data[out:out+4], rgba[:])
			out += 4
			x++
			if x == width {
				// end of line, advance to next
				x = 0
				y++
				out += row_inc
			}
		}
	}

	*imagewidth = uint(width)
	*imageheight = uint(height)

	return data
}

func Image_WriteTGA(filename string, pixels []byte, width, height int) bool {
	size := width * height * 3
	buffer := make([]byte, size+18)
	buffer[2] = 2 // uncompressed type
	buffer[12] = byte(width & 255)
	buffer[13] = byte(width >> 8)
	buffer[14] = byte(height & 255)
	buffer[15] = byte(height >> 8)
	buffer[16] = 24

	copy(buffer[18:], pixels[:size])

	return FS_WriteFile(filename, buffer)
}

/************************************ PCX ************************************/

// pcx_t: manufacturer, version, encoding, bits_per_pixel, xmin, ymin,
// xmax, ymax, hres, vres, palette[48], reserved, color_planes,
// bytes_per_line, palette_type, filler[58], then the data
const (
	pcx_xmax           = 8
	pcx_ymax           = 10
	pcx_bytes_per_line = 66
	pcx_palette_type   = 68
	pcx_data           = 128
)

func Image_LoadPCX(fin *os.File, filename string, matchwidth, matchheight int, imagewidth, imageheight *uint) []byte {
	if fin == nil {
		var flen int
		if fin, flen = FS_FOpenFile(filename); flen == -1 {
			return nil
		}
	}

	pcxbuf := make([]byte, com_filesize)
	if _, err := io.ReadFull(fin, pcxbuf); err != nil {
		Com_DPrintf("Image_LoadPCX: fread() failed on %s\n", COM_SkipPath(filename))
		fin.Close()
		return nil
	}
	fin.Close()

	if len(pcxbuf) < pcx_data {
		Com_DPrintf("Invalid PCX image %s\n", COM_SkipPath(filename))
		return nil
	}

	pix := pcx_data

	if pcxbuf[0] != 0x0a || pcxbuf[1] != 5 || pcxbuf[2] != 1 || pcxbuf[3] != 8 {
		Com_DPrintf("Invalid PCX image %s\n", COM_SkipPath(filename))
		return nil
	}

	width := int(leShort(pcxbuf[pcx_xmax:])) + 1
	height := int(leShort(pcxbuf[pcx_ymax:])) + 1

	if width > IMAGE_MAX_DIMENSIONS || height > IMAGE_MAX_DIMENSIONS {
		Com_DPrintf("PCX image %s exceeds maximum supported dimensions\n", COM_SkipPath(filename))
		return nil
	}

	if (matchwidth != 0 && width != matchwidth) || (matchheight != 0 && height != matchheight) {
		return nil
	}

	data := make([]byte, width*height)
	out := 0

	// C checks the position after the read, so the byte past the end is a
	// zero here instead of whatever follows the buffer
	next := func() int {
		b := 0
		if pix < len(pcxbuf) {
			b = int(pcxbuf[pix])
		}
		pix++
		return b
	}

	for y := 0; y < height; y, out = y+1, out+width {
		for x := 0; x < width; {
			if pix > com_filesize {
				Com_DPrintf("Malformed PCX image %s\n", COM_SkipPath(filename))
				return nil
			}

			dataByte := next()
			var runLength int

			if dataByte&0xC0 == 0xC0 {
				runLength = dataByte & 0x3F
				if pix > com_filesize {
					Com_DPrintf("Malformed PCX image %s\n", COM_SkipPath(filename))
					return nil
				}
				dataByte = next()
			} else {
				runLength = 1
			}

			if runLength+x > width+1 {
				Com_DPrintf("Malformed PCX image %s\n", COM_SkipPath(filename))
				return nil
			}

			for ; runLength > 0; runLength-- {
				if out+x < len(data) {
					data[out+x] = byte(dataByte)
				}
				x++
			}
		}
	}

	if pix > com_filesize {
		Com_DPrintf("Malformed PCX image %s\n", COM_SkipPath(filename))
		return nil
	}

	*imagewidth = uint(width)
	*imageheight = uint(height)
	return data
}

func Image_WritePCX(filename string, data []byte, width, height int, palette []byte) bool {
	pcx := make([]byte, 0, width*height*2+1000)

	hdr := make([]byte, pcx_data)
	hdr[0] = 0x0a
	hdr[1] = 5
	hdr[2] = 1
	hdr[3] = 8
	hdr[pcx_xmax] = byte(width - 1)
	hdr[pcx_xmax+1] = byte((width - 1) >> 8)
	hdr[pcx_ymax] = byte(height - 1)
	hdr[pcx_ymax+1] = byte((height - 1) >> 8)
	hdr[12] = byte(width)
	hdr[13] = byte(width >> 8)
	hdr[14] = byte(height)
	hdr[15] = byte(height >> 8)
	hdr[65] = 1
	hdr[pcx_bytes_per_line] = byte(width)
	hdr[pcx_bytes_per_line+1] = byte(width >> 8)
	hdr[pcx_palette_type] = 1
	pcx = append(pcx, hdr...)

	dp := 0
	for i := 0; i < height; i++ {
		for j := 0; j < width; j++ {
			if data[dp]&0xc0 != 0xc0 {
				pcx = append(pcx, data[dp])
			} else {
				pcx = append(pcx, 0xc1, data[dp])
			}
			dp++
		}
	}

	pcx = append(pcx, 0x0c)
	pcx = append(pcx, palette[:768]...)

	return FS_WriteFile(filename, pcx)
}

/*********************************** INIT ************************************/

func Image_CvarInit() {
	Cvar_SetCurrentGroup(CVAR_GROUP_SCREENSHOTS)

	Cvar_Register(&image_png_compression_level)

	Cvar_ResetCurrentGroup()
}

// image/png needs no loading, the module is simply registered
func Image_Init() {
	QLib_RegisterModule(qlib_libpng, func() {})
}

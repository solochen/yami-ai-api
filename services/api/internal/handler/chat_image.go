package handler

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
)

const (
	// chatImageMaxSide is the longest edge kept for chat vision attachments.
	chatImageMaxSide = 1600
	// chatImageMaxBytes is the payload budget after re-encoding.
	chatImageMaxBytes = 1_500_000
)

// shrinkChatImage downscales oversized chat attachments before they are inlined
// as base64 into the upstream request. A phone photo of 8-10MB becomes a few
// hundred KB, which keeps vision providers from stalling on huge payloads.
// It returns the original bytes when decoding fails or the image is already small.
func shrinkChatImage(data []byte, mime string) ([]byte, string) {
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return data, mime
	}
	bounds := img.Bounds()
	if len(data) <= chatImageMaxBytes && maxIntValue(bounds.Dx(), bounds.Dy()) <= chatImageMaxSide {
		return data, mime
	}
	scaled := scaleImageBox(img, chatImageMaxSide)
	if format == "png" || format == "gif" {
		var pngBuf bytes.Buffer
		if err := png.Encode(&pngBuf, scaled); err == nil && pngBuf.Len() < len(data) {
			return pngBuf.Bytes(), "image/png"
		}
	}
	flat := flattenWhite(scaled)
	for _, quality := range []int{85, 70, 55} {
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, flat, &jpeg.Options{Quality: quality}); err != nil {
			continue
		}
		if buf.Len() <= chatImageMaxBytes || quality == 55 {
			return buf.Bytes(), "image/jpeg"
		}
	}
	return data, mime
}

// scaleImageBox averages source pixels into the target pixel (box filter).
func scaleImageBox(src image.Image, maxSide int) image.Image {
	bounds := src.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	longest := maxIntValue(width, height)
	if longest <= maxSide || width <= 0 || height <= 0 {
		return src
	}
	scale := float64(maxSide) / float64(longest)
	targetW := int(float64(width)*scale + 0.5)
	targetH := int(float64(height)*scale + 0.5)
	if targetW < 1 {
		targetW = 1
	}
	if targetH < 1 {
		targetH = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
	for y := 0; y < targetH; y++ {
		y0 := bounds.Min.Y + y*height/targetH
		y1 := bounds.Min.Y + (y+1)*height/targetH
		if y1 <= y0 {
			y1 = y0 + 1
		}
		for x := 0; x < targetW; x++ {
			x0 := bounds.Min.X + x*width/targetW
			x1 := bounds.Min.X + (x+1)*width/targetW
			if x1 <= x0 {
				x1 = x0 + 1
			}
			var r, g, b, a, count uint32
			for sy := y0; sy < y1; sy++ {
				for sx := x0; sx < x1; sx++ {
					cr, cg, cb, ca := src.At(sx, sy).RGBA()
					r += cr
					g += cg
					b += cb
					a += ca
					count++
				}
			}
			if count == 0 {
				continue
			}
			dst.SetRGBA(x, y, color.RGBA{
				R: uint8((r / count) >> 8),
				G: uint8((g / count) >> 8),
				B: uint8((b / count) >> 8),
				A: uint8((a / count) >> 8),
			})
		}
	}
	return dst
}

func flattenWhite(src image.Image) image.Image {
	dst := image.NewRGBA(src.Bounds())
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(dst, dst.Bounds(), src, src.Bounds().Min, draw.Over)
	return dst
}

func maxIntValue(a, b int) int {
	if a > b {
		return a
	}
	return b
}

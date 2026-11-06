package handler

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"math/rand"
	"testing"
)

func TestShrinkChatImageReducesLargePhoto(t *testing.T) {
	const width, height = 3200, 2400
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	rng := rand.New(rand.NewSource(7))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetRGBA(x, y, color.RGBA{
				R: uint8(rng.Intn(256)),
				G: uint8((x * 255) / width),
				B: uint8((y * 255) / height),
				A: 255,
			})
		}
	}
	var original bytes.Buffer
	if err := jpeg.Encode(&original, img, &jpeg.Options{Quality: 98}); err != nil {
		t.Fatalf("encode fixture: %v", err)
	}
	if original.Len() <= chatImageMaxBytes {
		t.Fatalf("fixture too small to be meaningful: %d bytes", original.Len())
	}

	shrunk, mime := shrinkChatImage(original.Bytes(), "image/jpeg")
	if mime != "image/jpeg" {
		t.Fatalf("unexpected mime %q", mime)
	}
	if len(shrunk) > chatImageMaxBytes {
		t.Fatalf("shrunk image still %d bytes", len(shrunk))
	}
	decoded, _, err := image.Decode(bytes.NewReader(shrunk))
	if err != nil {
		t.Fatalf("decode shrunk image: %v", err)
	}
	bounds := decoded.Bounds()
	if bounds.Dx() > chatImageMaxSide || bounds.Dy() > chatImageMaxSide {
		t.Fatalf("shrunk image still %dx%d", bounds.Dx(), bounds.Dy())
	}
	if bounds.Dx() == width && bounds.Dy() == height {
		t.Fatalf("image was not resized")
	}
}

func TestShrinkChatImageKeepsSmallImages(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 320, 200))
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 20, G: 40, B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatalf("encode: %v", err)
	}
	original := buf.Bytes()
	out, mime := shrinkChatImage(original, "image/jpeg")
	if mime != "image/jpeg" || !bytes.Equal(out, original) {
		t.Fatalf("small image should be returned untouched")
	}
}

package main

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/jpeg"
	"strings"
	"testing"
)

// 实测事故：8 张手机原图 62.6MB，base64 后请求体 83.5MB，上游网关收不下导致
// 客户端等到超时且拿不到任何响应。参考图必须在发送前缩图压缩。
func TestLargeReferenceIsDownscaledBeforeUpload(t *testing.T) {
	// 构造一张高噪点的 3000x3000 JPEG，模拟手机原图的体积。
	im := image.NewRGBA(image.Rect(0, 0, 3000, 3000))
	seed := uint32(1)
	for y := 0; y < 3000; y++ {
		for x := 0; x < 3000; x++ {
			seed = seed*1664525 + 1013904223
			im.Set(x, y, color.RGBA{uint8(seed >> 24), uint8(seed >> 16), uint8(seed >> 8), 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, im, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	original := buf.Bytes()
	if len(original) < 1<<20 {
		t.Fatalf("测试素材过小，无法验证压缩: %d", len(original))
	}

	got := referenceDataURL(original, "image/jpeg")
	if !strings.HasPrefix(got, "data:image/jpeg;base64,") {
		t.Fatalf("未转为 data URL: %s", got[:40])
	}
	payload := len(got) - len("data:image/jpeg;base64,")
	raw, err := base64.StdEncoding.DecodeString(got[len("data:image/jpeg;base64,"):])
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width > referenceImageMaxEdge || cfg.Height > referenceImageMaxEdge {
		t.Fatalf("未缩图: %dx%d", cfg.Width, cfg.Height)
	}
	if payload > len(original)/2 {
		t.Fatalf("压缩无效: %d -> %d", len(original), payload)
	}
}

// 小图不应被无谓重编码（避免画质损失）。
func TestSmallReferenceStaysUntouched(t *testing.T) {
	im := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			im.Set(x, y, color.RGBA{200, 30, 30, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, im, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	want := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
	if got := referenceDataURL(buf.Bytes(), "image/jpeg"); got != want {
		t.Fatal("小图被重新编码")
	}
}

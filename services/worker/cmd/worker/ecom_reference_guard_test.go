package main

import (
	"context"



	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/starai/worker/internal/storage"
)

// 复现线上事故：上传素材存为 /media/<bucket>/<key> 相对地址后，
// worker 读不到原图却静默继续，模型只能凭文字自由发挥，把商品画成完全无关的品类。
func TestRelativeMediaReferenceResolvesThroughObjectStore(t *testing.T) {
	root := t.TempDir()
	key := "assets/7/ast_1/IMG_3641 拷贝.jpg"
	if err := os.MkdirAll(filepath.Join(root, "assets/7/ast_1"), 0o755); err != nil {
		t.Fatal(err)
	}
	// 1x1 PNG
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89")
	if err := os.WriteFile(filepath.Join(root, key), png, 0o644); err != nil {
		t.Fatal(err)
	}
	client, err := storage.NewLocal(root, "/media")
	if err != nil {
		t.Fatal(err)
	}
	previous := objectStore
	objectStore = client
	defer func() { objectStore = previous }()

	got, diag := normalizeReferenceImageDiag(context.Background(), "/media/"+key)
	if diag != "" || !strings.HasPrefix(got, "data:image/png;base64,") {
		t.Fatalf("相对素材地址未读成 data URL: got=%q diag=%s", got, diag)
	}
}

func TestUnreadableReferencesFailInsteadOfDegrading(t *testing.T) {
	client, err := storage.NewLocal(t.TempDir(), "/media")
	if err != nil {
		t.Fatal(err)
	}
	previous := objectStore
	objectStore = client
	defer func() { objectStore = previous }()

	// 有参考图但一张都读不出来 → 必须报错，不能退化成纯文字生图。
	if _, err := normalizeReferenceImagesChecked(context.Background(),
		[]interface{}{"/media/starai-works/missing-a.jpg", "/media/starai-works/missing-b.jpg"}); err == nil {
		t.Fatal("全部参考图不可读时仍继续生成")
	}
	// 相对地址读不出来时必须丢弃，不能把上游无法解析的地址透传出去。
	if out := normalizeReferenceImages(context.Background(), []interface{}{"/media/starai-works/missing.jpg"}); len(referenceImageSources(out)) != 0 {
		t.Fatalf("不可读的相对地址被透传: %v", out)
	}
	// 未声明参考图时不应报错。
	if _, err := normalizeReferenceImagesChecked(context.Background(), []interface{}{}); err != nil {
		t.Fatalf("无参考图时不应失败: %v", err)
	}
	// 已格式化的 data URL 直接透传。
	ref := "data:image/png;base64,aGVsbG8="
	out, err := normalizeReferenceImagesChecked(context.Background(), []interface{}{ref})
	if err != nil || len(referenceImageSources(out)) != 1 {
		t.Fatalf("data URL 被误判: %v %v", out, err)
	}
}

// 尺寸图/接口图/参数表的文字就是交付物，不能被默认的「禁止添加文字」策略压制。
func TestTextModulesGetTextPreservingPolicy(t *testing.T) {
	prompt := "画面任务：生成电商详情页规格参数表图。模块数据：型号：CX10P"

	plain := applyImageGenerationPolicies(prompt, map[string]interface{}{"language": "简体中文"})
	if !strings.Contains(plain, "Do not add captions, titles, marketing copy, subtitles, watermarks, size codes") {
		t.Fatal("普通模块应保留默认禁文字策略")
	}

	textModule := applyImageGenerationPolicies(prompt, map[string]interface{}{"language": "简体中文", "needs_text": true})
	if strings.Contains(textModule, "size codes or surrounding text") {
		t.Fatal("文字型模块不应被禁止渲染尺寸/文字")
	}
	if !strings.Contains(textModule, "required content") || !strings.Contains(textModule, "verbatim") {
		t.Fatalf("文字型模块缺少逐字渲染策略: %s", textModule)
	}
	if !strings.Contains(textModule, "型号：CX10P") {
		t.Fatal("模块资料不应丢失")
	}
}


package main

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/starai/worker/internal/storage"
)

// fakeMediaStore 只实现 loadMediaBytes 关心的两个方法。
type fakeMediaStore struct {
	objectKey string
	data      []byte
	err       error
	readKey   string
}

func (f *fakeMediaStore) Upload(context.Context, string, string, io.Reader, int64) (string, error) {
	return "", errors.New("not implemented")
}

func (f *fakeMediaStore) ReadAll(_ context.Context, objectName string, maxBytes int64) ([]byte, error) {
	f.readKey = objectName
	if f.err != nil {
		return nil, f.err
	}
	if int64(len(f.data)) > maxBytes {
		return nil, errors.New("object exceeds size limit")
	}
	return f.data, nil
}

func (f *fakeMediaStore) ObjectKeyFromURL(rawURL string) string {
	if objectName, ok := strings.CutPrefix(strings.TrimSpace(rawURL), "/media/"); ok {
		return objectName
	}
	return ""
}

// 详情排版会把上游成片落成 /media 相对地址后再下载拼长图；
// 相对地址没有协议头，必须走对象存储，否则详情排版整批失败。
func TestLoadMediaBytesReadsRelativeMediaFromObjectStore(t *testing.T) {
	previous := objectStore
	t.Cleanup(func() { objectStore = previous })

	png := []byte("\x89PNG\r\n\x1a\n")
	fake := &fakeMediaStore{data: png}
	objectStore = storage.Store(fake)

	data, contentType, err := loadMediaBytes(context.Background(), "/media/starai-works/works/image/task_1_wf01/img.png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != string(png) {
		t.Fatalf("unexpected bytes: %q", data)
	}
	if fake.readKey != "starai-works/works/image/task_1_wf01/img.png" {
		t.Fatalf("unexpected object key: %q", fake.readKey)
	}
	if !strings.HasPrefix(contentType, "image/") {
		t.Fatalf("unexpected content type: %q", contentType)
	}
}

// 无法反查对象 key 时不能静默返回空内容，必须报错，避免产出 0 字节成片。
func TestLoadMediaBytesRelativeWithoutStoreFails(t *testing.T) {
	previous := objectStore
	t.Cleanup(func() { objectStore = previous })
	objectStore = nil

	if _, _, err := loadMediaBytes(context.Background(), "/media/starai-works/works/image/x.png"); err == nil {
		t.Fatal("expected error for relative media path without object store")
	}
}

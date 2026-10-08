package service

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"testing"
)

type galleryQualityMemoryStore struct {
	publicURL string
	data      []byte
	readKey   string
}

func (s *galleryQualityMemoryStore) Upload(context.Context, string, string, io.Reader, int64) (string, error) {
	return "", nil
}
func (s *galleryQualityMemoryStore) ReadAll(_ context.Context, objectName string, _ int64) ([]byte, error) {
	s.readKey = objectName
	return s.data, nil
}
func (s *galleryQualityMemoryStore) Delete(context.Context, string) error { return nil }
func (s *galleryQualityMemoryStore) ObjectKeyFromURL(rawURL string) string {
	if rawURL == s.publicURL+"/starai-works/works/image/result.png" {
		return "works/image/result.png"
	}
	return ""
}
func (s *galleryQualityMemoryStore) PublicURL(objectName string) string {
	return s.publicURL + "/starai-works/" + objectName
}

func TestRewriteGalleryAssetURL(t *testing.T) {
	tests := []struct {
		name         string
		raw          string
		publicBase   string
		internalBase string
		want         string
	}{
		{
			name:         "rewrites configured local upload",
			raw:          "http://localhost:18080/uploads-local/works/image/a.png?download=1",
			publicBase:   "http://localhost:18080/uploads-local",
			internalBase: "http://api:8080/uploads-local",
			want:         "http://api:8080/uploads-local/works/image/a.png?download=1",
		},
		{
			name:         "preserves unrelated host",
			raw:          "https://cdn.example.com/uploads-local/a.png",
			publicBase:   "http://localhost:18080/uploads-local",
			internalBase: "http://api:8080/uploads-local",
			want:         "https://cdn.example.com/uploads-local/a.png",
		},
		{
			name:         "preserves sibling path",
			raw:          "http://localhost:18080/uploads-local-backup/a.png",
			publicBase:   "http://localhost:18080/uploads-local",
			internalBase: "http://api:8080/uploads-local",
			want:         "http://localhost:18080/uploads-local-backup/a.png",
		},
		{
			name:         "preserves URL without internal base",
			raw:          "http://localhost:18080/uploads-local/a.png",
			publicBase:   "http://localhost:18080/uploads-local",
			internalBase: "",
			want:         "http://localhost:18080/uploads-local/a.png",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := rewriteGalleryAssetURL(test.raw, test.publicBase, test.internalBase); got != test.want {
				t.Fatalf("rewriteGalleryAssetURL() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestCheckGalleryImageAssetUsesStoreForFirstPartyURL(t *testing.T) {
	var body bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 2, 3))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := png.Encode(&body, img); err != nil {
		t.Fatal(err)
	}
	store := &galleryQualityMemoryStore{publicURL: "http://localhost:9000", data: body.Bytes()}
	rawURL := store.PublicURL("works/image/result.png")
	checked, err := checkGalleryImageAsset(context.Background(), &http.Client{}, store, rawURL, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if store.readKey != "works/image/result.png" {
		t.Fatalf("storage key = %q", store.readKey)
	}
	if checked["status"] != "pass" || checked["width"] != 2 || checked["height"] != 3 || checked["format"] != "png" {
		t.Fatalf("unexpected image check: %#v", checked)
	}
	if checked["url"] != rawURL || checked["content_hash"] == "" {
		t.Fatalf("public URL or content hash missing: %#v", checked)
	}
}

func TestDuplicateStatusIsNotPollutedByUnrelatedWarnings(t *testing.T) {
	tests := []struct {
		name     string
		warnings []string
		blockers []string
		want     string
	}{
		{name: "no duplicate findings despite unrelated global warning", want: "pass"},
		{name: "duplicate prompt warning", warnings: []string{"duplicate_prompt"}, want: "warning"},
		{name: "duplicate content blocker", blockers: []string{"duplicate_content"}, want: "fail"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := galleryDuplicateCheckStatus(test.warnings, test.blockers); got != test.want {
				t.Fatalf("galleryDuplicateCheckStatus() = %q, want %q", got, test.want)
			}
		})
	}
}

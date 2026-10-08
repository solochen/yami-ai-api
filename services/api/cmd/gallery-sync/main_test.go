package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCaseContentHashIsStableAcrossTaxonomyOrder(t *testing.T) {
	left := referenceCase{Title: "Example", Image: "/images/example.jpg", Prompt: "prompt", Category: "Category", Styles: []string{"Poster", "UI"}, Scenes: []string{"Tech", "Commerce"}}
	right := referenceCase{Title: left.Title, Image: left.Image, Prompt: left.Prompt, Category: left.Category, Styles: []string{"UI", "Poster"}, Scenes: []string{"Commerce", "Tech"}}
	leftHash, err := caseContentHash(left)
	if err != nil {
		t.Fatal(err)
	}
	rightHash, err := caseContentHash(right)
	if err != nil {
		t.Fatal(err)
	}
	if leftHash != rightHash {
		t.Fatalf("hash changed when taxonomy order changed: %s != %s", leftHash, rightHash)
	}
}

func TestRawIDAcceptsNumberAndString(t *testing.T) {
	for _, raw := range []string{`544`, `"544"`} {
		got, err := rawID(json.RawMessage(raw))
		if err != nil || got != "544" {
			t.Fatalf("rawID(%s) = %q, %v", raw, got, err)
		}
	}
}

func TestValidateManifestRejectsUnknownTaxonomy(t *testing.T) {
	data := manifest{
		TotalCases: 1,
		Categories: []string{"Category"},
		Styles:     []string{"Style"},
		Scenes:     []string{"Scene"},
		Cases: []referenceCase{{
			ID: json.RawMessage(`1`), Title: "Title", Image: "/image.jpg", Prompt: "Prompt", Category: "Category",
			Styles: []string{"Unknown"}, Scenes: []string{"Scene"},
		}},
	}
	if err := validateManifest(data); err == nil || !strings.Contains(err.Error(), "unknown style") {
		t.Fatalf("validateManifest error = %v, want unknown style", err)
	}
}

func TestResolveReferenceAssetURLKeepsManifestDataDirectory(t *testing.T) {
	got, err := resolveReferenceAssetURL("/images/case544.jpg", "https://raw.githubusercontent.com/freestylefly/awesome-gpt-image-2/main/data/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	want := "https://raw.githubusercontent.com/freestylefly/awesome-gpt-image-2/main/data/images/case544.jpg"
	if got != want {
		t.Fatalf("resolved URL = %q, want %q", got, want)
	}
}

func TestDownloadReferenceAssetValidatesContentTypeAndMagic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("not a png"))
	}))
	defer server.Close()
	_, _, _, err := downloadReferenceAsset(context.Background(), server.Client(), server.URL)
	if err == nil || !strings.Contains(err.Error(), "mismatched image type") {
		t.Fatalf("downloadReferenceAsset error = %v, want a type validation error", err)
	}
}

func TestDownloadReferenceAssetAcceptsPNG(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png; charset=binary")
		_, _ = w.Write([]byte("\x89PNG\r\n\x1a\n"))
	}))
	defer server.Close()
	data, contentType, extension, err := downloadReferenceAsset(context.Background(), server.Client(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 8 || contentType != "image/png" || extension != ".png" {
		t.Fatalf("downloaded asset = %d bytes, %q, %q", len(data), contentType, extension)
	}
}

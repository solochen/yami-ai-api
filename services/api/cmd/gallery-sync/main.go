package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/starai/api/internal/config"
	"github.com/starai/api/internal/storage"
)

const (
	defaultSourceKey       = "github:freestylefly/awesome-gpt-image-2"
	defaultRepositoryURL   = "https://github.com/freestylefly/awesome-gpt-image-2"
	defaultManifestURL     = "https://raw.githubusercontent.com/freestylefly/awesome-gpt-image-2/main/data/cases.json"
	defaultLicenseName     = "MIT"
	defaultLicenseURL      = "https://raw.githubusercontent.com/freestylefly/awesome-gpt-image-2/main/LICENSE"
	defaultAttribution     = "freestylefly/awesome-gpt-image-2"
	defaultRequestTimeout  = 45 * time.Second
	defaultAssetTimeout    = 30 * time.Second
	defaultMirrorTimeout   = 30 * time.Minute
	maxMirrorAssetBytes    = 15 << 20
	defaultSourceAssetFlag = "unverified"
)

type manifest struct {
	TotalCases int             `json:"totalCases"`
	Categories []string        `json:"categories"`
	Styles     []string        `json:"styles"`
	Scenes     []string        `json:"scenes"`
	Cases      []referenceCase `json:"cases"`
}

type referenceCase struct {
	ID            json.RawMessage `json:"id"`
	Title         string          `json:"title"`
	Image         string          `json:"image"`
	ImageAlt      string          `json:"imageAlt"`
	SourceLabel   string          `json:"sourceLabel"`
	SourceURL     string          `json:"sourceUrl"`
	Prompt        string          `json:"prompt"`
	PromptPreview string          `json:"promptPreview"`
	Category      string          `json:"category"`
	Styles        []string        `json:"styles"`
	Scenes        []string        `json:"scenes"`
	Featured      bool            `json:"featured"`
	GithubURL     string          `json:"githubUrl"`
}

type sourceConfig struct {
	Key         string
	Repository  string
	ManifestURL string
	LicenseName string
	LicenseURL  string
	Attribution string
}

type syncStats struct {
	SourceKey    string
	Cases        int
	ManifestHash string
}

func main() {
	_ = godotenv.Load("../../.env.local", "../../.env", ".env.local", ".env")
	var (
		manifestURL  = flag.String("manifest-url", getenv("GALLERY_REFERENCE_MANIFEST_URL", defaultManifestURL), "reference manifest URL")
		sourceKey    = flag.String("source-key", getenv("GALLERY_REFERENCE_SOURCE_KEY", defaultSourceKey), "stable source key")
		repository   = flag.String("repository-url", getenv("GALLERY_REFERENCE_REPOSITORY_URL", defaultRepositoryURL), "source repository URL")
		licenseName  = flag.String("license-name", getenv("GALLERY_REFERENCE_LICENSE_NAME", defaultLicenseName), "repository license name")
		licenseURL   = flag.String("license-url", getenv("GALLERY_REFERENCE_LICENSE_URL", defaultLicenseURL), "license evidence URL")
		attribution  = flag.String("attribution", getenv("GALLERY_REFERENCE_ATTRIBUTION", defaultAttribution), "required source attribution")
		dryRun       = flag.Bool("dry-run", false, "validate and hash the manifest without writing to the database")
		mirrorAssets = flag.Bool("mirror-assets", false, "download and store source images without changing their license status")
	)
	flag.Parse()

	if strings.TrimSpace(*manifestURL) == "" || strings.TrimSpace(*sourceKey) == "" {
		log.Fatal("manifest-url and source-key are required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()
	raw, data, err := fetchManifest(ctx, *manifestURL)
	if err != nil {
		log.Fatalf("fetch reference manifest: %v", err)
	}
	if err := validateManifest(data); err != nil {
		log.Fatalf("validate reference manifest: %v", err)
	}

	stats := syncStats{SourceKey: *sourceKey, Cases: len(data.Cases), ManifestHash: hashBytes(raw)}
	if *dryRun {
		fmt.Printf("validated source=%s cases=%d manifest_hash=%s asset_license_status=%s\n", stats.SourceKey, stats.Cases, stats.ManifestHash, defaultSourceAssetFlag)
		return
	}

	dbURL := getenv("DATABASE_URL", "postgres://starai:starai@localhost:5432/starai?sslmode=disable")
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer pool.Close()

	config := sourceConfig{
		Key:         strings.TrimSpace(*sourceKey),
		Repository:  strings.TrimRight(strings.TrimSpace(*repository), "/"),
		ManifestURL: strings.TrimSpace(*manifestURL),
		LicenseName: strings.TrimSpace(*licenseName),
		LicenseURL:  strings.TrimSpace(*licenseURL),
		Attribution: strings.TrimSpace(*attribution),
	}
	if err := validateSourceConfig(config); err != nil {
		log.Fatalf("validate source config: %v", err)
	}
	if err := syncReferenceCases(ctx, pool, config, data, stats.ManifestHash); err != nil {
		log.Fatalf("sync reference cases: %v", err)
	}
	fmt.Printf("synced source=%s cases=%d manifest_hash=%s asset_license_status=%s\n", stats.SourceKey, stats.Cases, stats.ManifestHash, defaultSourceAssetFlag)
	if *mirrorAssets {
		mirrorCtx, mirrorCancel := context.WithTimeout(context.Background(), defaultMirrorTimeout)
		defer mirrorCancel()
		store, err := newReferenceStorage(mirrorCtx, pool)
		if err != nil {
			log.Fatalf("configure reference asset storage: %v", err)
		}
		mirrorStats, mirrorErr := mirrorReferenceAssets(mirrorCtx, pool, store, config.ManifestURL, config.Key)
		fmt.Printf("mirrored source=%s uploaded=%d skipped=%d failed=%d\n", config.Key, mirrorStats.Uploaded, mirrorStats.Skipped, mirrorStats.Failed)
		if mirrorErr != nil {
			log.Fatal(mirrorErr)
		}
	}
}

type mirrorStats struct {
	Uploaded int
	Skipped  int
	Failed   int
}

type referenceAssetRow struct {
	ID           int64
	SourceItemID string
	SourceURL    string
	ContentHash  string
}

func newReferenceStorage(ctx context.Context, pool *pgxpool.Pool) (storage.Store, error) {
	cfg := config.Load()
	storageCfg := storage.LoadConfig(ctx, pool, storage.Config{
		Provider:  "minio",
		Endpoint:  cfg.MinioEndpoint,
		AccessKey: cfg.MinioAccessKey,
		SecretKey: cfg.MinioSecretKey,
		Bucket:    cfg.MinioBucket,
		PublicURL: cfg.MinioPublicURL,
		UseSSL:    cfg.MinioUseSSL,
	})
	if strings.EqualFold(strings.TrimSpace(storageCfg.Provider), "local") {
		publicURL := strings.TrimRight(strings.TrimSpace(storageCfg.PublicURL), "/")
		if publicURL == "" {
			publicURL = strings.TrimRight(strings.TrimSpace(cfg.LocalStoragePublicURL), "/")
		}
		if publicURL == "" {
			publicURL = "http://localhost:" + getenv("API_PORT", "8080") + "/uploads-local"
		}
		return storage.NewLocal("", publicURL)
	}
	return storage.New(storageCfg.Endpoint, storageCfg.AccessKey, storageCfg.SecretKey, storageCfg.Bucket, storageCfg.PublicURL, storageCfg.UseSSL)
}

func mirrorReferenceAssets(ctx context.Context, pool *pgxpool.Pool, store storage.Store, manifestURL, sourceKey string) (mirrorStats, error) {
	rows, err := pool.Query(ctx, `
		SELECT c.id, c.source_item_id, c.image_source_url, c.content_hash
		FROM gallery_reference_cases c
		JOIN gallery_reference_sources s ON s.id=c.source_id
		WHERE s.source_key=$1 AND c.is_active=true AND c.image_asset_url IS NULL
		ORDER BY c.id ASC`, sourceKey)
	if err != nil {
		return mirrorStats{}, err
	}
	defer rows.Close()

	client := &http.Client{
		Timeout: defaultAssetTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			if req.URL == nil || !isHTTPURL(req.URL) {
				return errors.New("redirect target must use http or https")
			}
			return nil
		},
	}
	var stats mirrorStats
	for rows.Next() {
		var item referenceAssetRow
		if err := rows.Scan(&item.ID, &item.SourceItemID, &item.SourceURL, &item.ContentHash); err != nil {
			return stats, err
		}
		assetURL, err := resolveReferenceAssetURL(item.SourceURL, manifestURL)
		if err != nil {
			stats.Failed++
			log.Printf("mirror case %s: resolve image URL: %v", item.SourceItemID, err)
			continue
		}
		data, contentType, extension, err := downloadReferenceAsset(ctx, client, assetURL)
		if err != nil {
			stats.Failed++
			log.Printf("mirror case %s: %v", item.SourceItemID, err)
			continue
		}
		objectName := fmt.Sprintf("gallery/reference/%s/%s-%s%s", safeObjectSegment(sourceKey), safeObjectSegment(item.SourceItemID), item.ContentHash[:minInt(16, len(item.ContentHash))], extension)
		storedURL, err := store.Upload(ctx, objectName, contentType, bytes.NewReader(data), int64(len(data)))
		if err != nil {
			stats.Failed++
			log.Printf("mirror case %s: upload: %v", item.SourceItemID, err)
			continue
		}
		result, err := pool.Exec(ctx, `
			UPDATE gallery_reference_cases
			SET image_asset_url=$1, updated_at=now()
			WHERE id=$2 AND content_hash=$3 AND is_active=true AND image_asset_url IS NULL`, storedURL, item.ID, item.ContentHash)
		if err != nil {
			_ = store.Delete(ctx, objectName)
			return stats, err
		}
		if result.RowsAffected() == 0 {
			stats.Skipped++
			_ = store.Delete(ctx, objectName)
			continue
		}
		stats.Uploaded++
	}
	if err := rows.Err(); err != nil {
		return stats, err
	}
	if stats.Failed > 0 {
		return stats, fmt.Errorf("mirror completed with %d failed assets", stats.Failed)
	}
	return stats, nil
}

func resolveReferenceAssetURL(raw, manifestURL string) (string, error) {
	base, err := url.Parse(strings.TrimSpace(manifestURL))
	if err != nil || !isHTTPURL(base) {
		return "", errors.New("manifest URL must use http or https")
	}
	ref, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || strings.TrimSpace(raw) == "" {
		return "", errors.New("image URL is invalid")
	}
	if ref.IsAbs() {
		if !isHTTPURL(ref) {
			return "", errors.New("image URL must use http or https")
		}
		return ref.String(), nil
	}
	if strings.HasPrefix(ref.Path, "/") {
		ref.Path = path.Join(path.Dir(base.Path), strings.TrimPrefix(ref.Path, "/"))
		ref.RawPath = ""
		ref.Scheme = base.Scheme
		ref.Host = base.Host
		ref.Fragment = ""
		return ref.String(), nil
	}
	resolved := base.ResolveReference(ref)
	if !isHTTPURL(resolved) {
		return "", errors.New("resolved image URL must use http or https")
	}
	return resolved.String(), nil
}

func downloadReferenceAsset(ctx context.Context, client *http.Client, rawURL string) ([]byte, string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", "", err
	}
	req.Header.Set("Accept", "image/png,image/jpeg,image/webp,image/gif")
	req.Header.Set("User-Agent", "YiHangAI-gallery-sync/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, "", "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > maxMirrorAssetBytes {
		return nil, "", "", fmt.Errorf("image exceeds %d bytes", maxMirrorAssetBytes)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxMirrorAssetBytes+1))
	if err != nil {
		return nil, "", "", err
	}
	if len(data) == 0 || len(data) > maxMirrorAssetBytes {
		return nil, "", "", fmt.Errorf("image exceeds %d bytes or is empty", maxMirrorAssetBytes)
	}
	contentType, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		return nil, "", "", errors.New("image Content-Type is invalid")
	}
	detectedType, extension, ok := detectImageType(data)
	if !ok || contentType != detectedType {
		return nil, "", "", fmt.Errorf("unsupported or mismatched image type: header=%s", contentType)
	}
	return data, detectedType, extension, nil
}

func detectImageType(data []byte) (string, string, bool) {
	switch {
	case len(data) >= 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n":
		return "image/png", ".png", true
	case len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff:
		return "image/jpeg", ".jpg", true
	case len(data) >= 6 && (string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a"):
		return "image/gif", ".gif", true
	case len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return "image/webp", ".webp", true
	default:
		return "", "", false
	}
}

func isHTTPURL(value *url.URL) bool {
	return value != nil && (strings.EqualFold(value.Scheme, "http") || strings.EqualFold(value.Scheme, "https")) && value.Host != ""
}

func safeObjectSegment(value string) string {
	value = strings.TrimSpace(value)
	var builder strings.Builder
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '-' || char == '_' || char == '.' {
			builder.WriteRune(char)
		} else {
			builder.WriteByte('-')
		}
	}
	value = strings.Trim(builder.String(), ".-")
	if value == "" {
		return "source"
	}
	return value
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func fetchManifest(ctx context.Context, manifestURL string) ([]byte, manifest, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return nil, manifest{}, err
	}
	req.Header.Set("Accept", "application/json")
	client := &http.Client{Timeout: defaultRequestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, manifest{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, manifest{}, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 25<<20))
	if err != nil {
		return nil, manifest{}, err
	}
	var data manifest
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, manifest{}, err
	}
	return raw, data, nil
}

func validateSourceConfig(config sourceConfig) error {
	for name, value := range map[string]string{
		"source key":     config.Key,
		"repository URL": config.Repository,
		"manifest URL":   config.ManifestURL,
		"license name":   config.LicenseName,
		"license URL":    config.LicenseURL,
		"attribution":    config.Attribution,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", name)
		}
	}
	return nil
}

func validateManifest(data manifest) error {
	if len(data.Cases) == 0 {
		return errors.New("cases is empty")
	}
	if data.TotalCases > 0 && data.TotalCases != len(data.Cases) {
		return fmt.Errorf("totalCases=%d but cases=%d", data.TotalCases, len(data.Cases))
	}
	categorySet := stringSet(data.Categories)
	styleSet := stringSet(data.Styles)
	sceneSet := stringSet(data.Scenes)
	seenIDs := map[string]struct{}{}
	for index, item := range data.Cases {
		id, err := rawID(item.ID)
		if err != nil {
			return fmt.Errorf("cases[%d].id: %w", index, err)
		}
		if _, exists := seenIDs[id]; exists {
			return fmt.Errorf("duplicate case id %q", id)
		}
		seenIDs[id] = struct{}{}
		if strings.TrimSpace(item.Title) == "" || strings.TrimSpace(item.Prompt) == "" || strings.TrimSpace(item.Image) == "" || strings.TrimSpace(item.Category) == "" {
			return fmt.Errorf("case %q has a missing required field", id)
		}
		if _, ok := categorySet[item.Category]; !ok {
			return fmt.Errorf("case %q uses unknown category %q", id, item.Category)
		}
		for _, style := range item.Styles {
			if _, ok := styleSet[style]; !ok {
				return fmt.Errorf("case %q uses unknown style %q", id, style)
			}
		}
		for _, scene := range item.Scenes {
			if _, ok := sceneSet[scene]; !ok {
				return fmt.Errorf("case %q uses unknown scene %q", id, scene)
			}
		}
	}
	return nil
}

func syncReferenceCases(ctx context.Context, pool *pgxpool.Pool, config sourceConfig, data manifest, manifestHash string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, config.Key); err != nil {
		return err
	}

	var sourceID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO gallery_reference_sources (
			source_key, repository_url, manifest_url, license_name, license_url, attribution,
			sync_status, manifest_hash, case_count, last_sync_error, last_synced_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,'running',$7,$8,NULL,NULL,now())
		ON CONFLICT (source_key) DO UPDATE SET
			repository_url=EXCLUDED.repository_url,
			manifest_url=EXCLUDED.manifest_url,
			license_name=EXCLUDED.license_name,
			license_url=EXCLUDED.license_url,
			attribution=EXCLUDED.attribution,
			sync_status='running',
			manifest_hash=EXCLUDED.manifest_hash,
			case_count=EXCLUDED.case_count,
			last_sync_error=NULL,
			updated_at=now()
		RETURNING id`, config.Key, config.Repository, config.ManifestURL, config.LicenseName, config.LicenseURL, config.Attribution, manifestHash, len(data.Cases)).Scan(&sourceID)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE gallery_reference_cases SET is_active=false, updated_at=now() WHERE source_id=$1`, sourceID); err != nil {
		return err
	}

	for _, item := range data.Cases {
		id, _ := rawID(item.ID)
		styles, _ := json.Marshal(sortedCopy(item.Styles))
		scenes, _ := json.Marshal(sortedCopy(item.Scenes))
		contentHash, err := caseContentHash(item)
		if err != nil {
			return fmt.Errorf("hash case %s: %w", id, err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO gallery_reference_cases (
				source_id, source_item_id, content_hash, title, image_source_url, image_alt,
				source_url, github_url, prompt, prompt_preview, category, styles, scenes,
				featured, asset_license_status, status, is_active, last_seen_at, updated_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12::jsonb,$13::jsonb,$14,'unverified','pending_license',true,now(),now())
			ON CONFLICT (source_id, source_item_id) DO UPDATE SET
				content_hash=EXCLUDED.content_hash,
				title=EXCLUDED.title,
				image_source_url=EXCLUDED.image_source_url,
				image_alt=EXCLUDED.image_alt,
				source_url=EXCLUDED.source_url,
				github_url=EXCLUDED.github_url,
				prompt=EXCLUDED.prompt,
				prompt_preview=EXCLUDED.prompt_preview,
				category=EXCLUDED.category,
				styles=EXCLUDED.styles,
				scenes=EXCLUDED.scenes,
				featured=EXCLUDED.featured,
				image_asset_url=CASE WHEN gallery_reference_cases.content_hash = EXCLUDED.content_hash THEN gallery_reference_cases.image_asset_url ELSE NULL END,
				asset_license_status=CASE WHEN gallery_reference_cases.content_hash = EXCLUDED.content_hash THEN gallery_reference_cases.asset_license_status ELSE 'unverified' END,
				status=CASE WHEN gallery_reference_cases.content_hash = EXCLUDED.content_hash THEN gallery_reference_cases.status ELSE 'pending_license' END,
				is_active=true,
				last_seen_at=now(),
				updated_at=now()`,
			sourceID, id, contentHash, item.Title, item.Image, item.ImageAlt, item.SourceURL, item.GithubURL,
			item.Prompt, item.PromptPreview, item.Category, string(styles), string(scenes), item.Featured); err != nil {
			return fmt.Errorf("upsert case %s: %w", id, err)
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE gallery_reference_sources SET sync_status='succeeded', last_sync_error=NULL, last_synced_at=now(), updated_at=now() WHERE id=$1`, sourceID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func caseContentHash(item referenceCase) (string, error) {
	canonical := struct {
		Title    string   `json:"title"`
		Image    string   `json:"image"`
		Prompt   string   `json:"prompt"`
		Category string   `json:"category"`
		Styles   []string `json:"styles"`
		Scenes   []string `json:"scenes"`
	}{
		Title: item.Title, Image: item.Image, Prompt: item.Prompt, Category: item.Category,
		Styles: sortedCopy(item.Styles), Scenes: sortedCopy(item.Scenes),
	}
	raw, err := json.Marshal(canonical)
	if err != nil {
		return "", err
	}
	return hashBytes(raw), nil
}

func rawID(raw json.RawMessage) (string, error) {
	value := strings.TrimSpace(string(raw))
	if value == "" || value == "null" {
		return "", errors.New("id is required")
	}
	if value[0] == '"' {
		var decoded string
		if err := json.Unmarshal(raw, &decoded); err != nil {
			return "", err
		}
		if strings.TrimSpace(decoded) == "" {
			return "", errors.New("id is empty")
		}
		return decoded, nil
	}
	return value, nil
}

func stringSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}

func sortedCopy(values []string) []string {
	copyValues := append([]string(nil), values...)
	sort.Strings(copyValues)
	return copyValues
}

func hashBytes(raw []byte) string {
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func getenv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

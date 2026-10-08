package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type GalleryReferenceAdminItem struct {
	ID                 int64    `json:"id"`
	SourceID           int64    `json:"source_id"`
	SourceItemID       string   `json:"source_item_id"`
	SourceKey          string   `json:"source_key"`
	Title              string   `json:"title"`
	Prompt             string   `json:"prompt"`
	PromptPreview      string   `json:"prompt_preview,omitempty"`
	Category           string   `json:"category"`
	Styles             []string `json:"styles"`
	Scenes             []string `json:"scenes"`
	ImageSourceURL     string   `json:"image_source_url"`
	ImageAssetURL      *string  `json:"image_asset_url,omitempty"`
	ImageAlt           *string  `json:"image_alt,omitempty"`
	SourceURL          *string  `json:"source_url,omitempty"`
	GithubURL          *string  `json:"github_url,omitempty"`
	Featured           bool     `json:"featured"`
	AssetLicenseStatus string   `json:"asset_license_status"`
	Status             string   `json:"status"`
	IsActive           bool     `json:"is_active"`
	LicenseName        string   `json:"license_name"`
	LicenseURL         string   `json:"license_url"`
	Attribution        string   `json:"attribution"`
	RepositoryURL      string   `json:"repository_url"`
	SyncStatus         string   `json:"sync_status"`
	ManifestURL        string   `json:"manifest_url"`
	ContentHash        string   `json:"content_hash"`
	FirstSeenAt        string   `json:"first_seen_at"`
	LastSeenAt         string   `json:"last_seen_at"`
	UpdatedAt          string   `json:"updated_at"`
}

type GalleryReferenceAdminFilters struct {
	Status        string
	LicenseStatus string
	SourceKey     string
	Category      string
	Query         string
	Active        string
}

type GalleryReferenceUpdateInput struct {
	Title              *string   `json:"title"`
	Prompt             *string   `json:"prompt"`
	PromptPreview      *string   `json:"prompt_preview"`
	Category           *string   `json:"category"`
	Styles             *[]string `json:"styles"`
	Scenes             *[]string `json:"scenes"`
	ImageAlt           *string   `json:"image_alt"`
	SourceURL          *string   `json:"source_url"`
	GithubURL          *string   `json:"github_url"`
	Status             *string   `json:"status"`
	AssetLicenseStatus *string   `json:"asset_license_status"`
	Featured           *bool     `json:"featured"`
}

type GalleryReferenceBatchResult struct {
	Action  string  `json:"action"`
	Updated int     `json:"updated"`
	IDs     []int64 `json:"ids"`
}

type GalleryReferenceImportCase struct {
	ID             json.RawMessage `json:"id"`
	SourceItemID   string          `json:"source_item_id"`
	Title          string          `json:"title"`
	Image          string          `json:"image"`
	ImageSourceURL string          `json:"image_source_url"`
	ImageAlt       string          `json:"imageAlt"`
	SourceURL      string          `json:"sourceUrl"`
	GithubURL      string          `json:"githubUrl"`
	Prompt         string          `json:"prompt"`
	PromptPreview  string          `json:"promptPreview"`
	Category       string          `json:"category"`
	Styles         []string        `json:"styles"`
	Scenes         []string        `json:"scenes"`
	Featured       bool            `json:"featured"`
}

type GalleryReferenceImportInput struct {
	SourceKey     string
	RepositoryURL string
	ManifestURL   string
	LicenseName   string
	LicenseURL    string
	Attribution   string
	Cases         []GalleryReferenceImportCase
}

type GalleryReferenceCoverageFacet struct {
	Value          string `json:"value"`
	Total          int    `json:"total"`
	Mirrored       int    `json:"mirrored"`
	Published      int    `json:"published"`
	PendingLicense int    `json:"pending_license"`
	Hidden         int    `json:"hidden"`
	Gap            int    `json:"gap"`
}

type GalleryReferenceCoverageSource struct {
	SourceKey      string `json:"source_key"`
	RepositoryURL  string `json:"repository_url"`
	LicenseName    string `json:"license_name"`
	LicenseURL     string `json:"license_url"`
	Attribution    string `json:"attribution"`
	SyncStatus     string `json:"sync_status"`
	ActiveCases    int    `json:"active_cases"`
	MirroredCases  int    `json:"mirrored_cases"`
	PublishedCases int    `json:"published_cases"`
}

type GalleryReferenceCoverage struct {
	TargetPerFacet  int                              `json:"target_per_facet"`
	TotalActive     int                              `json:"total_active"`
	Mirrored        int                              `json:"mirrored"`
	Unmirrored      int                              `json:"unmirrored"`
	Published       int                              `json:"published"`
	PendingLicense  int                              `json:"pending_license"`
	Hidden          int                              `json:"hidden"`
	RejectedLicense int                              `json:"rejected_license"`
	Sources         []GalleryReferenceCoverageSource `json:"sources"`
	Categories      []GalleryReferenceCoverageFacet  `json:"categories"`
	Styles          []GalleryReferenceCoverageFacet  `json:"styles"`
	Scenes          []GalleryReferenceCoverageFacet  `json:"scenes"`
}

type galleryReferenceCoverageCounter struct {
	total, mirrored, published, pending, hidden int
}

type galleryReferenceScanner interface {
	Scan(dest ...any) error
}

const galleryReferenceAdminSelect = `
	SELECT c.id, c.source_id, c.source_item_id, s.source_key, c.title, c.prompt,
	       c.prompt_preview, c.category, c.styles, c.scenes, c.image_source_url,
	       c.image_asset_url, c.image_alt, c.source_url, c.github_url, c.featured,
	       c.asset_license_status, c.status, c.is_active, s.license_name,
	       s.license_url, s.attribution, s.repository_url, s.sync_status,
	       s.manifest_url, c.content_hash, c.first_seen_at, c.last_seen_at, c.updated_at
	FROM gallery_reference_cases c
	JOIN gallery_reference_sources s ON s.id=c.source_id`

func scanGalleryReferenceAdmin(scanner galleryReferenceScanner) (*GalleryReferenceAdminItem, error) {
	item := &GalleryReferenceAdminItem{}
	var stylesRaw, scenesRaw []byte
	var firstSeen, lastSeen, updated time.Time
	if err := scanner.Scan(
		&item.ID, &item.SourceID, &item.SourceItemID, &item.SourceKey, &item.Title, &item.Prompt,
		&item.PromptPreview, &item.Category, &stylesRaw, &scenesRaw, &item.ImageSourceURL,
		&item.ImageAssetURL, &item.ImageAlt, &item.SourceURL, &item.GithubURL, &item.Featured,
		&item.AssetLicenseStatus, &item.Status, &item.IsActive, &item.LicenseName,
		&item.LicenseURL, &item.Attribution, &item.RepositoryURL, &item.SyncStatus,
		&item.ManifestURL, &item.ContentHash, &firstSeen, &lastSeen, &updated,
	); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(stylesRaw, &item.Styles); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(scenesRaw, &item.Scenes); err != nil {
		return nil, err
	}
	if item.Styles == nil {
		item.Styles = []string{}
	}
	if item.Scenes == nil {
		item.Scenes = []string{}
	}
	item.FirstSeenAt = firstSeen.Format(time.RFC3339)
	item.LastSeenAt = lastSeen.Format(time.RFC3339)
	item.UpdatedAt = updated.Format(time.RFC3339)
	return item, nil
}

func (s *GalleryService) ListAdminReferences(ctx context.Context, filters GalleryReferenceAdminFilters, page, pageSize int) ([]GalleryReferenceAdminItem, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 30
	}
	where, args, err := galleryReferenceWhere(filters)
	if err != nil {
		return nil, 0, err
	}
	var total int
	if err := s.db.QueryRow(ctx, "SELECT COUNT(*) FROM gallery_reference_cases c JOIN gallery_reference_sources s ON s.id=c.source_id "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	listArgs := append([]interface{}{}, args...)
	listArgs = append(listArgs, pageSize, (page-1)*pageSize)
	query := galleryReferenceAdminSelect + " WHERE " + strings.TrimPrefix(where, " WHERE ") + fmt.Sprintf(" ORDER BY c.updated_at DESC, c.id DESC LIMIT $%d OFFSET $%d", len(listArgs)-1, len(listArgs))
	rows, err := s.db.Query(ctx, query, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]GalleryReferenceAdminItem, 0)
	for rows.Next() {
		item, err := scanGalleryReferenceAdmin(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func galleryReferenceWhere(filters GalleryReferenceAdminFilters) (string, []interface{}, error) {
	where := " WHERE 1=1"
	args := []interface{}{}
	add := func(condition string, value interface{}) {
		args = append(args, value)
		where += fmt.Sprintf(" AND "+condition, len(args))
	}
	if value := strings.TrimSpace(filters.Status); value != "" {
		if !validGalleryReferenceStatus(value) {
			return "", nil, errors.New("无效的参考案例状态")
		}
		add("c.status=$%d", value)
	}
	if value := strings.TrimSpace(filters.LicenseStatus); value != "" {
		if !validGalleryReferenceLicenseStatus(value) {
			return "", nil, errors.New("无效的许可证状态")
		}
		add("c.asset_license_status=$%d", value)
	}
	if value := strings.TrimSpace(filters.SourceKey); value != "" {
		add("s.source_key=$%d", value)
	}
	if value := strings.TrimSpace(filters.Category); value != "" {
		add("c.category=$%d", value)
	}
	if value := strings.TrimSpace(filters.Query); value != "" {
		args = append(args, "%"+strings.ToLower(value)+"%")
		placeholder := len(args)
		where += fmt.Sprintf(" AND (LOWER(c.title) LIKE $%d OR LOWER(c.prompt) LIKE $%d OR LOWER(c.source_item_id) LIKE $%d OR LOWER(s.source_key) LIKE $%d)", placeholder, placeholder, placeholder, placeholder)
	}
	switch value := strings.TrimSpace(filters.Active); value {
	case "true":
		where += " AND c.is_active=true"
	case "false":
		where += " AND c.is_active=false"
	case "":
	default:
		return "", nil, errors.New("无效的 active 筛选值")
	}
	return where, args, nil
}

func (s *GalleryService) GetAdminReference(ctx context.Context, id int64) (*GalleryReferenceAdminItem, error) {
	return scanGalleryReferenceAdmin(s.db.QueryRow(ctx, galleryReferenceAdminSelect+" WHERE c.id=$1", id))
}

func (s *GalleryService) UpdateAdminReference(ctx context.Context, id int64, input GalleryReferenceUpdateInput) (*GalleryReferenceAdminItem, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var currentStatus, currentLicense string
	var active bool
	var assetURL *string
	if err := tx.QueryRow(ctx, `SELECT status, asset_license_status, is_active, image_asset_url FROM gallery_reference_cases WHERE id=$1 FOR UPDATE`, id).Scan(&currentStatus, &currentLicense, &active, &assetURL); err != nil {
		return nil, err
	}
	status := currentStatus
	license := currentLicense
	if input.Status != nil {
		status = strings.TrimSpace(*input.Status)
		if !validGalleryReferenceStatus(status) {
			return nil, errors.New("无效的参考案例状态")
		}
	}
	if input.AssetLicenseStatus != nil {
		license = strings.TrimSpace(*input.AssetLicenseStatus)
		if !validGalleryReferenceLicenseStatus(license) {
			return nil, errors.New("无效的许可证状态")
		}
	}
	if license == "rejected" && status == "published" {
		status = "hidden"
	}
	if status == "published" && (license != "verified" || !active || assetURL == nil || strings.TrimSpace(*assetURL) == "") {
		return nil, errors.New("案例必须已验证许可证、处于有效状态并完成资源镜像后才能发布")
	}
	if input.Title != nil && strings.TrimSpace(*input.Title) == "" {
		return nil, errors.New("标题不能为空")
	}
	if input.Prompt != nil && strings.TrimSpace(*input.Prompt) == "" {
		return nil, errors.New("Prompt 不能为空")
	}
	if input.Category != nil && strings.TrimSpace(*input.Category) == "" {
		return nil, errors.New("分类不能为空")
	}
	var stylesJSON, scenesJSON interface{}
	if input.Styles != nil {
		values := normalizeReferenceLabels(*input.Styles)
		raw, _ := json.Marshal(values)
		stylesJSON = string(raw)
	}
	if input.Scenes != nil {
		values := normalizeReferenceLabels(*input.Scenes)
		raw, _ := json.Marshal(values)
		scenesJSON = string(raw)
	}
	_, err = tx.Exec(ctx, `
		UPDATE gallery_reference_cases SET
			title=COALESCE($1,title), prompt=COALESCE($2,prompt), prompt_preview=COALESCE($3,prompt_preview),
			category=COALESCE($4,category), styles=COALESCE($5::jsonb,styles), scenes=COALESCE($6::jsonb,scenes),
			image_alt=COALESCE($7,image_alt), source_url=COALESCE($8,source_url), github_url=COALESCE($9,github_url),
			status=$10, asset_license_status=$11, featured=COALESCE($12,featured), updated_at=now()
		WHERE id=$13`, nullableString(input.Title), nullableString(input.Prompt), nullableString(input.PromptPreview), nullableString(input.Category), stylesJSON, scenesJSON, nullableString(input.ImageAlt), nullableString(input.SourceURL), nullableString(input.GithubURL), status, license, input.Featured, id)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetAdminReference(ctx, id)
}

func (s *GalleryService) BatchUpdateAdminReferences(ctx context.Context, ids []int64, action string) (*GalleryReferenceBatchResult, error) {
	if len(ids) == 0 || len(ids) > 200 {
		return nil, errors.New("一次最多操作 200 个案例")
	}
	unique := make([]int64, 0, len(ids))
	seen := map[int64]struct{}{}
	for _, id := range ids {
		if id <= 0 {
			return nil, errors.New("案例 ID 无效")
		}
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			unique = append(unique, id)
		}
	}
	validActions := map[string]bool{"verify_license": true, "reject_license": true, "publish": true, "hide": true, "feature": true, "unfeature": true}
	if !validActions[action] {
		return nil, errors.New("不支持的批量操作")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT id, status, asset_license_status, is_active, image_asset_url FROM gallery_reference_cases WHERE id=ANY($1) FOR UPDATE`, unique)
	if err != nil {
		return nil, err
	}
	type state struct {
		id              int64
		status, license string
		active          bool
		asset           *string
	}
	states := map[int64]state{}
	for rows.Next() {
		var value state
		if err := rows.Scan(&value.id, &value.status, &value.license, &value.active, &value.asset); err != nil {
			rows.Close()
			return nil, err
		}
		states[value.id] = value
	}
	rows.Close()
	if len(states) != len(unique) {
		return nil, errors.New("部分案例不存在或已被删除")
	}
	for _, id := range unique {
		value := states[id]
		switch action {
		case "publish":
			if value.license != "verified" || !value.active || value.asset == nil || strings.TrimSpace(*value.asset) == "" {
				return nil, fmt.Errorf("案例 %d 未满足发布条件：需要验证许可证并完成资源镜像", id)
			}
			if _, err := tx.Exec(ctx, `UPDATE gallery_reference_cases SET status='published', updated_at=now() WHERE id=$1`, id); err != nil {
				return nil, err
			}
		case "hide":
			if _, err := tx.Exec(ctx, `UPDATE gallery_reference_cases SET status='hidden', updated_at=now() WHERE id=$1`, id); err != nil {
				return nil, err
			}
		case "verify_license":
			if _, err := tx.Exec(ctx, `UPDATE gallery_reference_cases SET asset_license_status='verified', updated_at=now() WHERE id=$1`, id); err != nil {
				return nil, err
			}
		case "reject_license":
			if _, err := tx.Exec(ctx, `UPDATE gallery_reference_cases SET asset_license_status='rejected', status=CASE WHEN status='published' THEN 'hidden' ELSE status END, updated_at=now() WHERE id=$1`, id); err != nil {
				return nil, err
			}
		case "feature":
			if _, err := tx.Exec(ctx, `UPDATE gallery_reference_cases SET featured=true, updated_at=now() WHERE id=$1`, id); err != nil {
				return nil, err
			}
		case "unfeature":
			if _, err := tx.Exec(ctx, `UPDATE gallery_reference_cases SET featured=false, updated_at=now() WHERE id=$1`, id); err != nil {
				return nil, err
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &GalleryReferenceBatchResult{Action: action, Updated: len(unique), IDs: unique}, nil
}

func (s *GalleryService) ImportAdminReferences(ctx context.Context, input GalleryReferenceImportInput) (int, error) {
	input.SourceKey = strings.TrimSpace(input.SourceKey)
	if input.SourceKey == "" {
		return 0, errors.New("source_key 不能为空")
	}
	if len(input.Cases) == 0 || len(input.Cases) > 5000 {
		return 0, errors.New("导入案例数量必须在 1 到 5000 之间")
	}
	seen := map[string]struct{}{}
	for index := range input.Cases {
		item := &input.Cases[index]
		if strings.TrimSpace(item.SourceItemID) == "" {
			if len(item.ID) == 0 {
				return 0, fmt.Errorf("cases[%d] 缺少 source_item_id", index)
			}
			item.SourceItemID = strings.Trim(strings.TrimSpace(string(item.ID)), "\"")
		}
		item.SourceItemID = strings.TrimSpace(item.SourceItemID)
		if item.Title == "" || item.Prompt == "" || item.Image == "" || item.Category == "" {
			return 0, fmt.Errorf("cases[%d] 缺少 title、prompt、image 或 category", index)
		}
		if _, ok := seen[item.SourceItemID]; ok {
			return 0, fmt.Errorf("重复的 source_item_id: %s", item.SourceItemID)
		}
		seen[item.SourceItemID] = struct{}{}
		item.Styles = normalizeReferenceLabels(item.Styles)
		item.Scenes = normalizeReferenceLabels(item.Scenes)
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, input.SourceKey); err != nil {
		return 0, err
	}
	var sourceID int64
	err = tx.QueryRow(ctx, `SELECT id FROM gallery_reference_sources WHERE source_key=$1 FOR UPDATE`, input.SourceKey).Scan(&sourceID)
	if errors.Is(err, pgx.ErrNoRows) {
		if strings.TrimSpace(input.RepositoryURL) == "" || strings.TrimSpace(input.ManifestURL) == "" || strings.TrimSpace(input.LicenseName) == "" || strings.TrimSpace(input.LicenseURL) == "" || strings.TrimSpace(input.Attribution) == "" {
			return 0, errors.New("新来源必须提供 repository_url、manifest_url、license_name、license_url 和 attribution")
		}
		err = tx.QueryRow(ctx, `INSERT INTO gallery_reference_sources (source_key, repository_url, manifest_url, license_name, license_url, attribution, sync_status, case_count, updated_at) VALUES ($1,$2,$3,$4,$5,$6,'succeeded',$7,now()) RETURNING id`, input.SourceKey, strings.TrimRight(input.RepositoryURL, "/"), input.ManifestURL, input.LicenseName, input.LicenseURL, input.Attribution, len(input.Cases)).Scan(&sourceID)
	} else if err == nil {
		_, err = tx.Exec(ctx, `UPDATE gallery_reference_sources SET repository_url=COALESCE(NULLIF($1,''),repository_url), manifest_url=COALESCE(NULLIF($2,''),manifest_url), license_name=COALESCE(NULLIF($3,''),license_name), license_url=COALESCE(NULLIF($4,''),license_url), attribution=COALESCE(NULLIF($5,''),attribution), case_count=$6, sync_status='succeeded', last_sync_error=NULL, updated_at=now() WHERE id=$7`, strings.TrimRight(strings.TrimSpace(input.RepositoryURL), "/"), strings.TrimSpace(input.ManifestURL), strings.TrimSpace(input.LicenseName), strings.TrimSpace(input.LicenseURL), strings.TrimSpace(input.Attribution), len(input.Cases), sourceID)
	} else {
		return 0, err
	}
	if err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `UPDATE gallery_reference_cases SET is_active=false, updated_at=now() WHERE source_id=$1`, sourceID); err != nil {
		return 0, err
	}
	for _, item := range input.Cases {
		styles, _ := json.Marshal(item.Styles)
		scenes, _ := json.Marshal(item.Scenes)
		hash := galleryReferenceContentHash(item)
		if _, err := tx.Exec(ctx, `
			INSERT INTO gallery_reference_cases (source_id, source_item_id, content_hash, title, image_source_url, image_alt, source_url, github_url, prompt, prompt_preview, category, styles, scenes, featured, asset_license_status, status, is_active, last_seen_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,NULLIF($6,''),NULLIF($7,''),NULLIF($8,''),$9,NULLIF($10,''),$11,$12::jsonb,$13::jsonb,$14,'unverified','pending_license',true,now(),now())
			ON CONFLICT (source_id, source_item_id) DO UPDATE SET
				content_hash=EXCLUDED.content_hash, title=EXCLUDED.title, image_source_url=EXCLUDED.image_source_url, image_alt=EXCLUDED.image_alt, source_url=EXCLUDED.source_url, github_url=EXCLUDED.github_url, prompt=EXCLUDED.prompt, prompt_preview=EXCLUDED.prompt_preview, category=EXCLUDED.category, styles=EXCLUDED.styles, scenes=EXCLUDED.scenes, featured=EXCLUDED.featured,
				image_asset_url=CASE WHEN gallery_reference_cases.content_hash=EXCLUDED.content_hash THEN gallery_reference_cases.image_asset_url ELSE NULL END,
				asset_license_status=CASE WHEN gallery_reference_cases.content_hash=EXCLUDED.content_hash THEN gallery_reference_cases.asset_license_status ELSE 'unverified' END,
				status=CASE WHEN gallery_reference_cases.content_hash=EXCLUDED.content_hash THEN gallery_reference_cases.status ELSE 'pending_license' END,
				is_active=true, last_seen_at=now(), updated_at=now()`, sourceID, item.SourceItemID, hash, item.Title, firstNonEmpty(item.ImageSourceURL, item.Image), item.ImageAlt, item.SourceURL, item.GithubURL, item.Prompt, item.PromptPreview, item.Category, string(styles), string(scenes), item.Featured); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return len(input.Cases), nil
}

func (s *GalleryService) GalleryReferenceCoverage(ctx context.Context, target int) (*GalleryReferenceCoverage, error) {
	if target < 1 || target > 100 {
		target = 8
	}
	rows, err := s.db.Query(ctx, galleryReferenceAdminSelect+" WHERE c.is_active=true ORDER BY c.id ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	categories := map[string]*galleryReferenceCoverageCounter{}
	styles := map[string]*galleryReferenceCoverageCounter{}
	scenes := map[string]*galleryReferenceCoverageCounter{}
	sources := map[string]*GalleryReferenceCoverageSource{}
	result := &GalleryReferenceCoverage{TargetPerFacet: target, Sources: []GalleryReferenceCoverageSource{}, Categories: []GalleryReferenceCoverageFacet{}, Styles: []GalleryReferenceCoverageFacet{}, Scenes: []GalleryReferenceCoverageFacet{}}
	for rows.Next() {
		item, err := scanGalleryReferenceAdmin(rows)
		if err != nil {
			return nil, err
		}
		result.TotalActive++
		mirrored := item.ImageAssetURL != nil && strings.TrimSpace(*item.ImageAssetURL) != ""
		if mirrored {
			result.Mirrored++
		} else {
			result.Unmirrored++
		}
		switch item.Status {
		case "published":
			result.Published++
		case "pending_license":
			result.PendingLicense++
		case "hidden":
			result.Hidden++
		}
		if item.AssetLicenseStatus == "rejected" {
			result.RejectedLicense++
		}
		if source := sources[item.SourceKey]; source == nil {
			sources[item.SourceKey] = &GalleryReferenceCoverageSource{SourceKey: item.SourceKey, RepositoryURL: item.RepositoryURL, LicenseName: item.LicenseName, LicenseURL: item.LicenseURL, Attribution: item.Attribution, SyncStatus: item.SyncStatus, ActiveCases: 1, MirroredCases: boolInt(mirrored), PublishedCases: boolInt(item.Status == "published")}
		} else {
			source.ActiveCases++
			source.MirroredCases += boolInt(mirrored)
			source.PublishedCases += boolInt(item.Status == "published")
		}
		incrementCoverageCounter(categories, item.Category, item.Status, mirrored)
		for _, value := range uniqueReferenceLabels(item.Styles) {
			incrementCoverageCounter(styles, value, item.Status, mirrored)
		}
		for _, value := range uniqueReferenceLabels(item.Scenes) {
			incrementCoverageCounter(scenes, value, item.Status, mirrored)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, source := range sources {
		result.Sources = append(result.Sources, *source)
	}
	sort.Slice(result.Sources, func(i, j int) bool { return result.Sources[i].SourceKey < result.Sources[j].SourceKey })
	result.Categories = coverageFacets(categories, target)
	result.Styles = coverageFacets(styles, target)
	result.Scenes = coverageFacets(scenes, target)
	return result, nil
}

func incrementCoverageCounter(values map[string]*galleryReferenceCoverageCounter, label, status string, mirrored bool) {
	value := values[label]
	if value == nil {
		value = &galleryReferenceCoverageCounter{}
		values[label] = value
	}
	value.total++
	if mirrored {
		value.mirrored++
	}
	switch status {
	case "published":
		value.published++
	case "pending_license":
		value.pending++
	case "hidden":
		value.hidden++
	}
}

func coverageFacets(values map[string]*galleryReferenceCoverageCounter, target int) []GalleryReferenceCoverageFacet {
	labels := make([]string, 0, len(values))
	for label := range values {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	items := make([]GalleryReferenceCoverageFacet, 0, len(labels))
	for _, label := range labels {
		value := values[label]
		gap := target - value.published
		if gap < 0 {
			gap = 0
		}
		items = append(items, GalleryReferenceCoverageFacet{Value: label, Total: value.total, Mirrored: value.mirrored, Published: value.published, PendingLicense: value.pending, Hidden: value.hidden, Gap: gap})
	}
	return items
}

func validGalleryReferenceStatus(value string) bool {
	return value == "pending_license" || value == "published" || value == "hidden"
}

func validGalleryReferenceLicenseStatus(value string) bool {
	return value == "unverified" || value == "verified" || value == "rejected"
}

func nullableString(value *string) interface{} {
	if value == nil {
		return nil
	}
	return strings.TrimSpace(*value)
}

func normalizeReferenceLabels(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func uniqueReferenceLabels(values []string) []string {
	return normalizeReferenceLabels(values)
}

func galleryReferenceContentHash(item GalleryReferenceImportCase) string {
	canonical := struct {
		Title    string   `json:"title"`
		Image    string   `json:"image"`
		Prompt   string   `json:"prompt"`
		Category string   `json:"category"`
		Styles   []string `json:"styles"`
		Scenes   []string `json:"scenes"`
	}{item.Title, firstNonEmpty(item.ImageSourceURL, item.Image), item.Prompt, item.Category, item.Styles, item.Scenes}
	raw, _ := json.Marshal(canonical)
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

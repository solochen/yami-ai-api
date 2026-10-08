package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/starai/api/internal/storage"
	"github.com/starai/api/internal/util"
)

const galleryQualityMaxImageBytes = 50 << 20

type GalleryQualityReviewService struct {
	db                      *pgxpool.Pool
	httpClient              *http.Client
	storage                 storage.Store
	localStoragePublicURL   string
	localStorageInternalURL string
}

func NewGalleryQualityReviewService(db *pgxpool.Pool, store storage.Store, storageURLs ...string) *GalleryQualityReviewService {
	var publicURL, internalURL string
	if len(storageURLs) > 0 {
		publicURL = strings.TrimRight(strings.TrimSpace(storageURLs[0]), "/")
	}
	if len(storageURLs) > 1 {
		internalURL = strings.TrimRight(strings.TrimSpace(storageURLs[1]), "/")
	}
	return &GalleryQualityReviewService{
		db:                      db,
		httpClient:              &http.Client{Timeout: 45 * time.Second},
		storage:                 store,
		localStoragePublicURL:   publicURL,
		localStorageInternalURL: internalURL,
	}
}

type GalleryQualityReviewAdminItem struct {
	ID                int64                  `json:"id"`
	PublicID          string                 `json:"public_id"`
	TaskID            int64                  `json:"task_id"`
	TaskPublicID      string                 `json:"task_public_id"`
	SpecID            int64                  `json:"spec_id"`
	SpecPublicID      string                 `json:"spec_public_id"`
	SpecTitle         string                 `json:"spec_title"`
	Prompt            string                 `json:"prompt"`
	Category          string                 `json:"category"`
	Styles            []string               `json:"styles"`
	Scenes            []string               `json:"scenes"`
	TaskStatus        string                 `json:"task_status"`
	ModelCode         string                 `json:"model_code"`
	OutputAssetURL    string                 `json:"output_asset_url"`
	Status            string                 `json:"status"`
	ReviewVersion     int                    `json:"review_version"`
	AssetCheck        map[string]interface{} `json:"asset_check"`
	MetadataCheck     map[string]interface{} `json:"metadata_check"`
	OCRCheck          map[string]interface{} `json:"ocr_check"`
	SemanticCheck     map[string]interface{} `json:"semantic_check"`
	RiskCheck         map[string]interface{} `json:"risk_check"`
	DuplicateCheck    map[string]interface{} `json:"duplicate_check"`
	CheckSummary      map[string]interface{} `json:"check_summary"`
	OutputContentHash string                 `json:"output_content_hash,omitempty"`
	PromptHash        string                 `json:"prompt_hash,omitempty"`
	ReviewerID        *int64                 `json:"reviewer_id,omitempty"`
	ReviewNotes       string                 `json:"review_notes"`
	RejectionReason   string                 `json:"rejection_reason"`
	ReviewedAt        string                 `json:"reviewed_at,omitempty"`
	GalleryItemID     *int64                 `json:"gallery_item_id,omitempty"`
	PublishedAt       string                 `json:"published_at,omitempty"`
	CreatedAt         string                 `json:"created_at"`
	UpdatedAt         string                 `json:"updated_at"`
}

type GalleryQualityReviewDecisionInput struct {
	ReviewNotes     string `json:"review_notes"`
	RejectionReason string `json:"rejection_reason"`
}

const galleryQualityReviewSelect = `
	SELECT r.id, r.public_id, r.task_id, t.public_id, r.spec_id, s.public_id, s.title,
	       s.prompt, s.category, s.styles, s.scenes, t.status, t.model_code,
	       t.output_asset_url, r.status, r.review_version, r.asset_check,
	       r.metadata_check, r.ocr_check, r.semantic_check, r.risk_check, r.duplicate_check,
	       r.check_summary, r.output_content_hash, r.prompt_hash, r.reviewer_id,
	       r.review_notes, r.rejection_reason, r.reviewed_at, r.gallery_item_id,
	       r.published_at, r.created_at, r.updated_at
	FROM gallery_quality_reviews r
	JOIN gallery_generation_tasks t ON t.id=r.task_id
	JOIN gallery_case_specs s ON s.id=r.spec_id
`

func (s *GalleryQualityReviewService) EnsurePendingReviewRecords(ctx context.Context) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO gallery_quality_reviews (public_id, task_id, spec_id)
		SELECT 'g_review_' || md5(t.id::text || clock_timestamp()::text), t.id, t.spec_id
		FROM gallery_generation_tasks t
		WHERE t.status='pending_review'
		ON CONFLICT (task_id) DO NOTHING`)
	return err
}

func (s *GalleryQualityReviewService) List(ctx context.Context, status string, page, pageSize int) ([]GalleryQualityReviewAdminItem, int, error) {
	if err := s.EnsurePendingReviewRecords(ctx); err != nil {
		return nil, 0, err
	}
	page, pageSize = normalizeGalleryPage(page, pageSize)
	args := []interface{}{}
	where := " WHERE 1=1"
	if value := strings.TrimSpace(status); value != "" {
		if !validGalleryQualityReviewStatus(value) {
			return nil, 0, errors.New("无效的质量审核状态")
		}
		args = append(args, value)
		where += fmt.Sprintf(" AND r.status=$%d", len(args))
	}
	var total int
	if err := s.db.QueryRow(ctx, "SELECT COUNT(*) FROM gallery_quality_reviews r"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, pageSize, (page-1)*pageSize)
	query := galleryQualityReviewSelect + where + fmt.Sprintf(" ORDER BY CASE r.status WHEN 'auto_failed' THEN 1 WHEN 'awaiting_manual_review' THEN 2 WHEN 'approved' THEN 3 WHEN 'published' THEN 4 ELSE 5 END, r.updated_at DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]GalleryQualityReviewAdminItem, 0)
	for rows.Next() {
		item, err := scanGalleryQualityReview(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *item)
	}
	return items, total, rows.Err()
}

func (s *GalleryQualityReviewService) Get(ctx context.Context, id int64) (*GalleryQualityReviewAdminItem, error) {
	if id <= 0 {
		return nil, errors.New("质量审核记录不存在")
	}
	return scanGalleryQualityReview(s.db.QueryRow(ctx, galleryQualityReviewSelect+" WHERE r.id=$1", id))
}

func (s *GalleryQualityReviewService) RunAutoCheck(ctx context.Context, id int64) (*GalleryQualityReviewAdminItem, error) {
	if id <= 0 {
		return nil, errors.New("质量审核记录不存在")
	}
	var taskID int64
	var taskStatus, assetURL, prompt, specStatus, riskReviewStatus string
	var expectedRaw, riskFlagsRaw, candidateModelsRaw []byte
	if err := s.db.QueryRow(ctx, `
		SELECT r.task_id, t.status, t.output_asset_url, s.prompt, s.status,
		       s.expected_text, s.risk_flags, s.risk_review_status, s.candidate_models
		FROM gallery_quality_reviews r
		JOIN gallery_generation_tasks t ON t.id=r.task_id
		JOIN gallery_case_specs s ON s.id=r.spec_id
		WHERE r.id=$1`, id).Scan(&taskID, &taskStatus, &assetURL, &prompt, &specStatus, &expectedRaw, &riskFlagsRaw, &riskReviewStatus, &candidateModelsRaw); err != nil {
		return nil, err
	}
	if _, err := s.db.Exec(ctx, `UPDATE gallery_quality_reviews SET status='auto_checking',updated_at=now() WHERE id=$1 AND status NOT IN ('published','rejected')`, id); err != nil {
		return nil, err
	}

	assetCheck := map[string]interface{}{"status": "fail", "url": strings.TrimSpace(assetURL)}
	metadataCheck := map[string]interface{}{"status": "pass"}
	ocrCheck := map[string]interface{}{}
	semanticCheck := map[string]interface{}{"status": "unavailable", "available": false, "matched": false, "reason": "当前部署未配置视觉语义审核模型，需人工核对图片与 Prompt/标签的一致性"}
	riskCheck := map[string]interface{}{}
	duplicateCheck := map[string]interface{}{}
	warnings := []string{}
	blockers := []string{}

	contentHash := ""
	if strings.TrimSpace(assetURL) == "" {
		assetCheck["reason"] = "生成任务没有输出图片 URL"
		blockers = append(blockers, "asset_missing")
	} else if checked, err := checkGalleryImageAsset(ctx, s.httpClient, s.storage, assetURL, s.localStoragePublicURL, s.localStorageInternalURL); err != nil {
		assetCheck["reason"] = err.Error()
		blockers = append(blockers, "asset_unavailable")
	} else {
		assetCheck = checked
		assetCheck["url"] = assetURL
		contentHash = checked["content_hash"].(string)
	}

	if taskStatus != "pending_review" && taskStatus != "succeeded" {
		metadataCheck["status"] = "fail"
		metadataCheck["reason"] = "生成任务尚未进入可审核状态"
		blockers = append(blockers, "task_not_reviewable")
	}
	if specStatus != "ready" {
		metadataCheck["status"] = "fail"
		metadataCheck["reason"] = "案例规格不是 ready 状态"
		blockers = append(blockers, "spec_not_ready")
	}
	var candidates []GalleryCaseSpecModelCandidate
	if err := json.Unmarshal(candidateModelsRaw, &candidates); err != nil {
		metadataCheck["status"] = "fail"
		metadataCheck["reason"] = "候选模型 JSON 无效"
		blockers = append(blockers, "candidate_models_invalid")
	}
	var taskModelCode string
	if err := s.db.QueryRow(ctx, `SELECT model_code FROM gallery_generation_tasks WHERE id=$1`, taskID).Scan(&taskModelCode); err != nil {
		return nil, err
	}
	if len(candidates) == 0 || !containsGalleryModel(candidates, taskModelCode) {
		metadataCheck["status"] = "fail"
		metadataCheck["reason"] = "生成模型不在案例规格候选模型中"
		blockers = append(blockers, "model_not_declared")
	}
	metadataCheck["task_status"] = taskStatus
	metadataCheck["spec_status"] = specStatus
	metadataCheck["model_code"] = taskModelCode

	var expectedText []string
	_ = json.Unmarshal(expectedRaw, &expectedText)
	if len(expectedText) == 0 {
		ocrCheck = map[string]interface{}{"status": "not_required", "available": false, "matched": true}
	} else {
		ocrCheck = map[string]interface{}{"status": "unavailable", "available": false, "matched": false, "expected_text": expectedText, "reason": "当前部署未配置 OCR 引擎，需人工复核图片文字"}
		warnings = append(warnings, "ocr_unavailable")
	}
	warnings = append(warnings, "semantic_check_unavailable")

	var riskFlags []string
	_ = json.Unmarshal(riskFlagsRaw, &riskFlags)
	switch {
	case strings.EqualFold(riskReviewStatus, "rejected"):
		riskCheck = map[string]interface{}{"status": "fail", "risk_flags": riskFlags, "risk_review_status": riskReviewStatus}
		blockers = append(blockers, "risk_rejected")
	case len(riskFlags) > 0 && !strings.EqualFold(riskReviewStatus, "approved"):
		riskCheck = map[string]interface{}{"status": "manual_required", "risk_flags": riskFlags, "risk_review_status": riskReviewStatus}
		warnings = append(warnings, "risk_manual_review")
	default:
		riskCheck = map[string]interface{}{"status": "pass", "risk_flags": riskFlags, "risk_review_status": riskReviewStatus}
	}

	promptHash := hashString(prompt)
	duplicateCheck, duplicateWarnings, duplicateBlockers, err := s.findGalleryDuplicates(ctx, id, taskID, assetURL, contentHash, promptHash)
	if err != nil {
		return nil, err
	}
	warnings = append(warnings, duplicateWarnings...)
	blockers = append(blockers, duplicateBlockers...)
	duplicateCheck["status"] = galleryDuplicateCheckStatus(duplicateWarnings, duplicateBlockers)

	overall := "pass"
	if len(blockers) > 0 {
		overall = "fail"
	}
	checkSummary := map[string]interface{}{"overall": overall, "blockers": blockers, "warnings": warnings, "checked_at": time.Now().UTC().Format(time.RFC3339)}
	status := "awaiting_manual_review"
	if overall == "fail" {
		status = "auto_failed"
	}
	if _, err := s.db.Exec(ctx, `
		UPDATE gallery_quality_reviews
		SET status=$2, review_version=review_version+1, asset_check=$3, metadata_check=$4,
		    ocr_check=$5, semantic_check=$6, risk_check=$7, duplicate_check=$8, check_summary=$9,
		    output_content_hash=$10, prompt_hash=$11, updated_at=now()
		WHERE id=$1 AND status NOT IN ('published','rejected')`, id, status, mustJSON(assetCheck), mustJSON(metadataCheck), mustJSON(ocrCheck), mustJSON(semanticCheck), mustJSON(riskCheck), mustJSON(duplicateCheck), mustJSON(checkSummary), contentHash, promptHash); err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

func (s *GalleryQualityReviewService) Decide(ctx context.Context, id, adminID int64, approved bool, input GalleryQualityReviewDecisionInput) (*GalleryQualityReviewAdminItem, error) {
	if id <= 0 || adminID <= 0 {
		return nil, errors.New("质量审核参数无效")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var status string
	var summaryRaw []byte
	if err := tx.QueryRow(ctx, `SELECT status,check_summary FROM gallery_quality_reviews WHERE id=$1 FOR UPDATE`, id).Scan(&status, &summaryRaw); err != nil {
		return nil, err
	}
	if approved {
		if status == "approved" || status == "published" {
			if err := tx.Commit(ctx); err != nil {
				return nil, err
			}
			return s.Get(ctx, id)
		}
		if status != "awaiting_manual_review" {
			return nil, fmt.Errorf("当前状态 %s 不允许审核通过，请先完成自动检查", status)
		}
		var summary map[string]interface{}
		_ = json.Unmarshal(summaryRaw, &summary)
		if summary["overall"] == "fail" {
			return nil, errors.New("自动检查未通过，不能直接审核通过")
		}
		_, err = tx.Exec(ctx, `UPDATE gallery_quality_reviews SET status='approved',reviewer_id=$2,review_notes=$3,rejection_reason='',reviewed_at=now(),updated_at=now() WHERE id=$1`, id, adminID, strings.TrimSpace(input.ReviewNotes))
	} else {
		if status == "rejected" {
			if err := tx.Commit(ctx); err != nil {
				return nil, err
			}
			return s.Get(ctx, id)
		}
		if status == "published" {
			return nil, errors.New("已发布的案例不能拒绝")
		}
		if strings.TrimSpace(input.RejectionReason) == "" {
			return nil, errors.New("拒绝时必须填写原因")
		}
		_, err = tx.Exec(ctx, `UPDATE gallery_quality_reviews SET status='rejected',reviewer_id=$2,review_notes=$3,rejection_reason=$4,reviewed_at=now(),updated_at=now() WHERE id=$1`, id, adminID, strings.TrimSpace(input.ReviewNotes), strings.TrimSpace(input.RejectionReason))
	}
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

func (s *GalleryQualityReviewService) Publish(ctx context.Context, id int64) (*GalleryQualityReviewAdminItem, error) {
	if id <= 0 {
		return nil, errors.New("质量审核记录不存在")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var taskID, specID int64
	var status, taskStatus, modelCode, title, prompt, imageURL, mediaType, category string
	var stylesRaw, scenesRaw []byte
	if err := tx.QueryRow(ctx, `
		SELECT r.task_id,r.spec_id,r.status,t.status,t.model_code,s.title,s.prompt,t.output_asset_url,
		       s.output_media_type,s.category,s.styles,s.scenes
		FROM gallery_quality_reviews r
		JOIN gallery_generation_tasks t ON t.id=r.task_id
		JOIN gallery_case_specs s ON s.id=r.spec_id
		WHERE r.id=$1 FOR UPDATE`, id).Scan(&taskID, &specID, &status, &taskStatus, &modelCode, &title, &prompt, &imageURL, &mediaType, &category, &stylesRaw, &scenesRaw); err != nil {
		return nil, err
	}
	if status == "published" {
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return s.Get(ctx, id)
	}
	if status != "approved" {
		return nil, fmt.Errorf("当前状态 %s 不允许发布", status)
	}
	if strings.TrimSpace(imageURL) == "" || (taskStatus != "pending_review" && taskStatus != "succeeded") {
		return nil, errors.New("生成结果已失效，不能发布")
	}
	tags := []string{strings.TrimSpace(category)}
	var styles, scenes []string
	_ = json.Unmarshal(stylesRaw, &styles)
	_ = json.Unmarshal(scenesRaw, &scenes)
	tags = appendUniqueStrings(tags, styles...)
	tags = appendUniqueStrings(tags, scenes...)
	tags = compactStrings(tags)
	tagsJSON := mustJSON(tags)
	var galleryID int64
	var galleryPublicID string
	err = tx.QueryRow(ctx, `SELECT id,public_id FROM gallery_items WHERE generation_task_id=$1 FOR UPDATE`, taskID).Scan(&galleryID, &galleryPublicID)
	if errors.Is(err, pgx.ErrNoRows) {
		galleryPublicID = util.NewPublicID("gal")
		err = tx.QueryRow(ctx, `
			INSERT INTO gallery_items (public_id,generation_task_id,model_code,title,prompt,cover_url,type,tags,status,is_featured)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'approved',false)
			RETURNING id`, galleryPublicID, taskID, modelCode, title, prompt, imageURL, mediaTypeOrImage(mediaType), tagsJSON).Scan(&galleryID)
	} else if err == nil {
		_, err = tx.Exec(ctx, `UPDATE gallery_items SET model_code=$2,title=$3,prompt=$4,cover_url=$5,type=$6,tags=$7,status='approved' WHERE id=$1`, galleryID, modelCode, title, prompt, imageURL, mediaTypeOrImage(mediaType), tagsJSON)
	}
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `UPDATE gallery_quality_reviews SET status='published',gallery_item_id=$2,published_at=COALESCE(published_at,now()),updated_at=now() WHERE id=$1`, id, galleryID); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `UPDATE gallery_generation_tasks SET status='succeeded',updated_at=now() WHERE id=$1 AND status='pending_review'`, taskID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

func scanGalleryQualityReview(row interface{ Scan(...interface{}) error }) (*GalleryQualityReviewAdminItem, error) {
	var item GalleryQualityReviewAdminItem
	var stylesRaw, scenesRaw, assetRaw, metadataRaw, ocrRaw, semanticRaw, riskRaw, duplicateRaw, summaryRaw []byte
	var reviewedAt, publishedAt, createdAt, updatedAt *time.Time
	if err := row.Scan(&item.ID, &item.PublicID, &item.TaskID, &item.TaskPublicID, &item.SpecID, &item.SpecPublicID, &item.SpecTitle, &item.Prompt, &item.Category, &stylesRaw, &scenesRaw, &item.TaskStatus, &item.ModelCode, &item.OutputAssetURL, &item.Status, &item.ReviewVersion, &assetRaw, &metadataRaw, &ocrRaw, &semanticRaw, &riskRaw, &duplicateRaw, &summaryRaw, &item.OutputContentHash, &item.PromptHash, &item.ReviewerID, &item.ReviewNotes, &item.RejectionReason, &reviewedAt, &item.GalleryItemID, &publishedAt, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	_ = json.Unmarshal(stylesRaw, &item.Styles)
	_ = json.Unmarshal(scenesRaw, &item.Scenes)
	item.AssetCheck = parseJSONMapBytes(assetRaw)
	item.MetadataCheck = parseJSONMapBytes(metadataRaw)
	item.OCRCheck = parseJSONMapBytes(ocrRaw)
	item.SemanticCheck = parseJSONMapBytes(semanticRaw)
	item.RiskCheck = parseJSONMapBytes(riskRaw)
	item.DuplicateCheck = parseJSONMapBytes(duplicateRaw)
	item.CheckSummary = parseJSONMapBytes(summaryRaw)
	item.ReviewedAt = formatOptionalTime(reviewedAt)
	item.PublishedAt = formatOptionalTime(publishedAt)
	item.CreatedAt = formatOptionalTime(createdAt)
	item.UpdatedAt = formatOptionalTime(updatedAt)
	if item.Styles == nil {
		item.Styles = []string{}
	}
	if item.Scenes == nil {
		item.Scenes = []string{}
	}
	return &item, nil
}

func checkGalleryImage(ctx context.Context, client *http.Client, rawURL string) (map[string]interface{}, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, errors.New("图片 URL 不是有效的 HTTP(S) 地址")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("图片请求失败：%w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("图片请求返回 HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, galleryQualityMaxImageBytes+1))
	if err != nil {
		return nil, fmt.Errorf("读取图片失败：%w", err)
	}
	return inspectGalleryImage(rawURL, body, resp.Header.Get("Content-Type"))
}

// checkGalleryImageAsset reads first-party assets through the configured store.
// Public browser URLs can point at localhost or a CDN that API containers cannot
// reach, while the storage client remains the authoritative internal read path.
func checkGalleryImageAsset(ctx context.Context, client *http.Client, store storage.Store, rawURL, publicBase, internalBase string) (map[string]interface{}, error) {
	if store != nil {
		if objectKey := strings.TrimSpace(store.ObjectKeyFromURL(rawURL)); objectKey != "" {
			body, err := store.ReadAll(ctx, objectKey, galleryQualityMaxImageBytes)
			if err != nil {
				return nil, fmt.Errorf("读取站点存储图片失败：%w", err)
			}
			return inspectGalleryImage(rawURL, body, "")
		}
	}
	return checkGalleryImage(ctx, client, rewriteGalleryAssetURL(rawURL, publicBase, internalBase))
}

func inspectGalleryImage(rawURL string, body []byte, responseContentType string) (map[string]interface{}, error) {
	if len(body) == 0 {
		return nil, errors.New("图片内容为空")
	}
	if len(body) > galleryQualityMaxImageBytes {
		return nil, errors.New("图片超过 50 MB 限制")
	}
	detected := http.DetectContentType(body)
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(responseContentType, ";")[0]))
	if !strings.HasPrefix(contentType, "image/") {
		contentType = detected
	}
	if !strings.HasPrefix(contentType, "image/") {
		return nil, errors.New("响应 MIME 不是图片")
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil || config.Width <= 0 || config.Height <= 0 {
		return nil, errors.New("图片无法解码")
	}
	sum := sha256.Sum256(body)
	return map[string]interface{}{
		"status":       "pass",
		"url":          rawURL,
		"content_type": contentType,
		"format":       format,
		"size_bytes":   len(body),
		"width":        config.Width,
		"height":       config.Height,
		"content_hash": hex.EncodeToString(sum[:]),
	}, nil
}

// rewriteGalleryAssetURL lets API containers read their own local uploads when
// the public URL resolves to the host machine rather than the Compose network.
// URLs outside the configured public base are intentionally left untouched.
func rewriteGalleryAssetURL(rawURL, publicBase, internalBase string) string {
	rawURL = strings.TrimSpace(rawURL)
	publicBase = strings.TrimRight(strings.TrimSpace(publicBase), "/")
	internalBase = strings.TrimRight(strings.TrimSpace(internalBase), "/")
	if rawURL == "" || publicBase == "" || internalBase == "" || publicBase == internalBase {
		return rawURL
	}
	raw, err := url.Parse(rawURL)
	if err != nil || raw.Scheme == "" || raw.Host == "" {
		return rawURL
	}
	public, err := url.Parse(publicBase)
	if err != nil || public.Scheme == "" || public.Host == "" {
		return rawURL
	}
	if !strings.EqualFold(raw.Scheme, public.Scheme) || !strings.EqualFold(raw.Host, public.Host) {
		return rawURL
	}
	basePath := strings.TrimRight(public.Path, "/")
	if basePath == "" {
		basePath = "/"
	}
	if raw.Path != basePath && !strings.HasPrefix(raw.Path, strings.TrimRight(basePath, "/")+"/") {
		return rawURL
	}
	relativePath := strings.TrimPrefix(raw.Path, strings.TrimRight(basePath, "/"))
	if relativePath == "" {
		relativePath = "/"
	}
	internal, err := url.Parse(internalBase)
	if err != nil || internal.Scheme == "" || internal.Host == "" {
		return rawURL
	}
	internal.Path = strings.TrimRight(internal.Path, "/") + relativePath
	internal.RawQuery = raw.RawQuery
	internal.Fragment = raw.Fragment
	return internal.String()
}

func (s *GalleryQualityReviewService) findGalleryDuplicates(ctx context.Context, reviewID, taskID int64, assetURL, contentHash, promptHash string) (map[string]interface{}, []string, []string, error) {
	result := map[string]interface{}{"content_hash": contentHash, "prompt_hash": promptHash, "same_content": []interface{}{}, "same_prompt": []interface{}{}, "same_url": []interface{}{}}
	warnings, blockers := []string{}, []string{}
	if contentHash != "" {
		rows, err := s.db.Query(ctx, `SELECT public_id,task_id,status FROM gallery_quality_reviews WHERE output_content_hash=$1 AND task_id<>$2 AND status NOT IN ('rejected') LIMIT 5`, contentHash, taskID)
		if err != nil {
			return nil, nil, nil, err
		}
		matches := []map[string]interface{}{}
		for rows.Next() {
			var publicID, status string
			var matchedTaskID int64
			if err := rows.Scan(&publicID, &matchedTaskID, &status); err != nil {
				rows.Close()
				return nil, nil, nil, err
			}
			matches = append(matches, map[string]interface{}{"public_id": publicID, "task_id": matchedTaskID, "status": status})
		}
		rows.Close()
		result["same_content"] = matches
		if len(matches) > 0 {
			blockers = append(blockers, "duplicate_content")
		}
	}
	if promptHash != "" {
		rows, err := s.db.Query(ctx, `SELECT public_id,task_id,status FROM gallery_quality_reviews WHERE prompt_hash=$1 AND task_id<>$2 AND status NOT IN ('rejected') LIMIT 5`, promptHash, taskID)
		if err != nil {
			return nil, nil, nil, err
		}
		matches := []map[string]interface{}{}
		for rows.Next() {
			var publicID, status string
			var matchedTaskID int64
			if err := rows.Scan(&publicID, &matchedTaskID, &status); err != nil {
				rows.Close()
				return nil, nil, nil, err
			}
			matches = append(matches, map[string]interface{}{"public_id": publicID, "task_id": matchedTaskID, "status": status})
		}
		rows.Close()
		result["same_prompt"] = matches
		if len(matches) > 0 {
			warnings = append(warnings, "duplicate_prompt")
		}
	}
	if strings.TrimSpace(assetURL) != "" {
		rows, err := s.db.Query(ctx, `SELECT public_id,status FROM gallery_items WHERE cover_url=$1 AND status IN ('approved','pending') LIMIT 5`, assetURL)
		if err != nil {
			return nil, nil, nil, err
		}
		matches := []map[string]interface{}{}
		for rows.Next() {
			var publicID, status string
			if err := rows.Scan(&publicID, &status); err != nil {
				rows.Close()
				return nil, nil, nil, err
			}
			matches = append(matches, map[string]interface{}{"public_id": publicID, "status": status})
		}
		rows.Close()
		result["same_url"] = matches
		if len(matches) > 0 {
			blockers = append(blockers, "duplicate_public_url")
		}
	}
	result["review_id"] = reviewID
	return result, warnings, blockers, nil
}

func containsGalleryModel(candidates []GalleryCaseSpecModelCandidate, code string) bool {
	for _, candidate := range candidates {
		if strings.EqualFold(strings.TrimSpace(candidate.Code), strings.TrimSpace(code)) {
			return true
		}
	}
	return false
}

func galleryDuplicateCheckStatus(warnings, blockers []string) string {
	if len(blockers) > 0 {
		return "fail"
	}
	if len(warnings) > 0 {
		return "warning"
	}
	return "pass"
}

func hashString(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func mustJSON(value interface{}) []byte {
	data, _ := json.Marshal(value)
	return data
}

func parseJSONMapBytes(raw []byte) map[string]interface{} {
	value := map[string]interface{}{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &value)
	}
	return value
}

func formatOptionalTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format(time.RFC3339)
}

func mediaTypeOrImage(value string) string {
	if strings.TrimSpace(value) == "" {
		return "image"
	}
	return value
}

func appendUniqueStrings(values []string, additions ...string) []string {
	seen := map[string]bool{}
	for _, value := range values {
		seen[strings.TrimSpace(value)] = true
	}
	for _, value := range additions {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			values = append(values, value)
			seen[value] = true
		}
	}
	return values
}

func compactStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			result = append(result, value)
			seen[value] = true
		}
	}
	return result
}

func validGalleryQualityReviewStatus(value string) bool {
	switch value {
	case "pending_review", "auto_checking", "auto_failed", "awaiting_manual_review", "approved", "rejected", "published":
		return true
	default:
		return false
	}
}

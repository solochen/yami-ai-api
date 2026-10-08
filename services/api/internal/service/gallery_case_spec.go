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

type GalleryGapTaskAdminItem struct {
	ID               int64                  `json:"id"`
	PublicID         string                 `json:"public_id"`
	Fingerprint      string                 `json:"fingerprint"`
	SourceReportHash string                 `json:"source_report_hash"`
	SourceSnapshotAt *string                `json:"source_snapshot_at,omitempty"`
	DimensionType    string                 `json:"dimension_type"`
	Category         string                 `json:"category"`
	Style            string                 `json:"style"`
	Scene            string                 `json:"scene"`
	PromptLanguage   string                 `json:"prompt_language"`
	TargetCount      int                    `json:"target_count"`
	CurrentCount     int                    `json:"current_count"`
	GapCount         int                    `json:"gap_count"`
	Priority         string                 `json:"priority"`
	Title            string                 `json:"title"`
	Rationale        string                 `json:"rationale"`
	RiskFlags        []string               `json:"risk_flags"`
	SourceContext    map[string]interface{} `json:"source_context"`
	Status           string                 `json:"status"`
	SpecCount        int                    `json:"spec_count"`
	ReadySpecCount   int                    `json:"ready_spec_count"`
	CreatedAt        string                 `json:"created_at"`
	UpdatedAt        string                 `json:"updated_at"`
}

type GalleryCaseSpecModelCandidate struct {
	Code      string `json:"code"`
	Role      string `json:"role"`
	Rationale string `json:"rationale"`
}

type GalleryCaseSpecAdminItem struct {
	ID                     int64                           `json:"id"`
	PublicID               string                          `json:"public_id"`
	GapTaskID              int64                           `json:"gap_task_id"`
	GapTaskPublicID        string                          `json:"gap_task_public_id"`
	GapTaskTitle           string                          `json:"gap_task_title"`
	Version                int                             `json:"version"`
	Status                 string                          `json:"status"`
	Title                  string                          `json:"title"`
	Purpose                string                          `json:"purpose"`
	Prompt                 string                          `json:"prompt"`
	NegativePrompt         string                          `json:"negative_prompt"`
	PromptLanguage         string                          `json:"prompt_language"`
	ImageTextLanguages     []string                        `json:"image_text_languages"`
	ExpectedText           []string                        `json:"expected_text"`
	Category               string                          `json:"category"`
	Styles                 []string                        `json:"styles"`
	Scenes                 []string                        `json:"scenes"`
	AspectRatio            string                          `json:"aspect_ratio"`
	OutputMediaType        string                          `json:"output_media_type"`
	CandidateModels        []GalleryCaseSpecModelCandidate `json:"candidate_models"`
	AcceptanceCriteria     []string                        `json:"acceptance_criteria"`
	SelectionReason        string                          `json:"selection_reason"`
	RiskFlags              []string                        `json:"risk_flags"`
	RiskNotes              string                          `json:"risk_notes"`
	RiskReviewStatus       string                          `json:"risk_review_status"`
	GeneratorType          string                          `json:"generator_type"`
	GeneratorModelCode     string                          `json:"generator_model_code"`
	GeneratorPromptVersion string                          `json:"generator_prompt_version"`
	GenerationRequestID    string                          `json:"generation_request_id"`
	GenerationInput        map[string]interface{}          `json:"generation_input"`
	GenerationOutput       map[string]interface{}          `json:"generation_output"`
	EstimatedCost          float64                         `json:"estimated_cost"`
	ActualCost             float64                         `json:"actual_cost"`
	InputTokens            int                             `json:"input_tokens"`
	OutputTokens           int                             `json:"output_tokens"`
	ValidationErrors       []string                        `json:"validation_errors"`
	ContentHash            string                          `json:"content_hash"`
	CreatedAt              string                          `json:"created_at"`
	UpdatedAt              string                          `json:"updated_at"`
}

type GalleryGapTaskInput struct {
	PublicID         string                 `json:"public_id"`
	SourceReportHash string                 `json:"source_report_hash"`
	SourceSnapshotAt string                 `json:"source_snapshot_at"`
	DimensionType    string                 `json:"dimension_type"`
	Category         string                 `json:"category"`
	Style            string                 `json:"style"`
	Scene            string                 `json:"scene"`
	PromptLanguage   string                 `json:"prompt_language"`
	TargetCount      int                    `json:"target_count"`
	CurrentCount     int                    `json:"current_count"`
	GapCount         int                    `json:"gap_count"`
	Priority         string                 `json:"priority"`
	Title            string                 `json:"title"`
	Rationale        string                 `json:"rationale"`
	RiskFlags        []string               `json:"risk_flags"`
	SourceContext    map[string]interface{} `json:"source_context"`
}

type GalleryCaseSpecInput struct {
	PublicID               string                          `json:"public_id"`
	GapTaskPublicID        string                          `json:"gap_task_public_id"`
	Title                  string                          `json:"title"`
	Purpose                string                          `json:"purpose"`
	Prompt                 string                          `json:"prompt"`
	NegativePrompt         string                          `json:"negative_prompt"`
	PromptLanguage         string                          `json:"prompt_language"`
	ImageTextLanguages     []string                        `json:"image_text_languages"`
	ExpectedText           []string                        `json:"expected_text"`
	Category               string                          `json:"category"`
	Styles                 []string                        `json:"styles"`
	Scenes                 []string                        `json:"scenes"`
	AspectRatio            string                          `json:"aspect_ratio"`
	OutputMediaType        string                          `json:"output_media_type"`
	CandidateModels        []GalleryCaseSpecModelCandidate `json:"candidate_models"`
	AcceptanceCriteria     []string                        `json:"acceptance_criteria"`
	SelectionReason        string                          `json:"selection_reason"`
	RiskFlags              []string                        `json:"risk_flags"`
	RiskNotes              string                          `json:"risk_notes"`
	RiskReviewStatus       string                          `json:"risk_review_status"`
	Status                 string                          `json:"status"`
	GeneratorType          string                          `json:"generator_type"`
	GeneratorModelCode     string                          `json:"generator_model_code"`
	GeneratorPromptVersion string                          `json:"generator_prompt_version"`
	GenerationRequestID    string                          `json:"generation_request_id"`
	GenerationInput        map[string]interface{}          `json:"generation_input"`
	GenerationOutput       map[string]interface{}          `json:"generation_output"`
	EstimatedCost          float64                         `json:"estimated_cost"`
	ActualCost             float64                         `json:"actual_cost"`
	InputTokens            int                             `json:"input_tokens"`
	OutputTokens           int                             `json:"output_tokens"`
}

type GalleryCaseSpecUpdateInput struct {
	Title              *string                          `json:"title"`
	Purpose            *string                          `json:"purpose"`
	Prompt             *string                          `json:"prompt"`
	NegativePrompt     *string                          `json:"negative_prompt"`
	PromptLanguage     *string                          `json:"prompt_language"`
	ImageTextLanguages *[]string                        `json:"image_text_languages"`
	ExpectedText       *[]string                        `json:"expected_text"`
	Category           *string                          `json:"category"`
	Styles             *[]string                        `json:"styles"`
	Scenes             *[]string                        `json:"scenes"`
	AspectRatio        *string                          `json:"aspect_ratio"`
	CandidateModels    *[]GalleryCaseSpecModelCandidate `json:"candidate_models"`
	AcceptanceCriteria *[]string                        `json:"acceptance_criteria"`
	SelectionReason    *string                          `json:"selection_reason"`
	RiskNotes          *string                          `json:"risk_notes"`
	RiskReviewStatus   *string                          `json:"risk_review_status"`
	Status             *string                          `json:"status"`
}

const galleryGapTaskSelect = `
	SELECT g.id, g.public_id, g.fingerprint, g.source_report_hash, g.source_snapshot_at,
	       g.dimension_type, g.category, g.style, g.scene, g.prompt_language,
	       g.target_count, g.current_count, g.gap_count, g.priority, g.title,
	       g.rationale, g.risk_flags, g.source_context, g.status,
	       COUNT(s.id), COUNT(s.id) FILTER (WHERE s.status='ready'),
	       g.created_at, g.updated_at
	FROM gallery_gap_tasks g
	LEFT JOIN gallery_case_specs s ON s.gap_task_id=g.id
`

const galleryCaseSpecSelect = `
	SELECT s.id, s.public_id, s.gap_task_id, g.public_id, g.title, s.version, s.status,
	       s.title, s.purpose, s.prompt, s.negative_prompt, s.prompt_language,
	       s.image_text_languages, s.expected_text, s.category, s.styles, s.scenes,
	       s.aspect_ratio, s.output_media_type, s.candidate_models, s.acceptance_criteria,
	       s.selection_reason, s.risk_flags, s.risk_notes, s.risk_review_status,
	       s.generator_type, s.generator_model_code, s.generator_prompt_version,
	       s.generation_request_id, s.generation_input, s.generation_output,
	       s.estimated_cost, s.actual_cost, s.input_tokens, s.output_tokens,
	       s.validation_errors, s.content_hash, s.created_at, s.updated_at
	FROM gallery_case_specs s
	JOIN gallery_gap_tasks g ON g.id=s.gap_task_id
`

func (s *GalleryService) ListGalleryGapTasks(ctx context.Context, status string, page, pageSize int) ([]GalleryGapTaskAdminItem, int, error) {
	page, pageSize = normalizeGalleryPage(page, pageSize)
	args := []interface{}{}
	where := " WHERE 1=1"
	if value := strings.TrimSpace(status); value != "" {
		if !validGalleryGapStatus(value) {
			return nil, 0, errors.New("无效的缺口任务状态")
		}
		args = append(args, value)
		where += fmt.Sprintf(" AND g.status=$%d", len(args))
	}
	var total int
	if err := s.db.QueryRow(ctx, "SELECT COUNT(*) FROM gallery_gap_tasks g"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, pageSize, (page-1)*pageSize)
	query := galleryGapTaskSelect + where + " GROUP BY g.id ORDER BY CASE g.priority WHEN 'critical' THEN 4 WHEN 'high' THEN 3 WHEN 'medium' THEN 2 ELSE 1 END DESC, g.gap_count DESC, g.updated_at DESC LIMIT $" + fmt.Sprint(len(args)-1) + " OFFSET $" + fmt.Sprint(len(args))
	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]GalleryGapTaskAdminItem, 0)
	for rows.Next() {
		item, err := scanGalleryGapTask(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *item)
	}
	return items, total, rows.Err()
}

func (s *GalleryService) ListGalleryCaseSpecs(ctx context.Context, status, gapTaskPublicID string, page, pageSize int) ([]GalleryCaseSpecAdminItem, int, error) {
	page, pageSize = normalizeGalleryPage(page, pageSize)
	args := []interface{}{}
	where := " WHERE 1=1"
	if value := strings.TrimSpace(status); value != "" {
		if !validGalleryCaseSpecStatus(value) {
			return nil, 0, errors.New("无效的案例规格状态")
		}
		args = append(args, value)
		where += fmt.Sprintf(" AND s.status=$%d", len(args))
	}
	if value := strings.TrimSpace(gapTaskPublicID); value != "" {
		args = append(args, value)
		where += fmt.Sprintf(" AND g.public_id=$%d", len(args))
	}
	var total int
	if err := s.db.QueryRow(ctx, "SELECT COUNT(*) FROM gallery_case_specs s JOIN gallery_gap_tasks g ON g.id=s.gap_task_id"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, pageSize, (page-1)*pageSize)
	query := galleryCaseSpecSelect + where + " ORDER BY s.updated_at DESC, s.id DESC LIMIT $" + fmt.Sprint(len(args)-1) + " OFFSET $" + fmt.Sprint(len(args))
	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]GalleryCaseSpecAdminItem, 0)
	for rows.Next() {
		item, err := scanGalleryCaseSpec(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *item)
	}
	return items, total, rows.Err()
}

func (s *GalleryService) GetGalleryCaseSpec(ctx context.Context, id int64) (*GalleryCaseSpecAdminItem, error) {
	return scanGalleryCaseSpec(s.db.QueryRow(ctx, galleryCaseSpecSelect+" WHERE s.id=$1", id))
}

func (s *GalleryService) ImportGalleryCaseSpecs(ctx context.Context, tasks []GalleryGapTaskInput, specs []GalleryCaseSpecInput, adminID int64) ([]GalleryCaseSpecAdminItem, error) {
	if len(tasks) == 0 && len(specs) == 0 {
		return nil, errors.New("缺口任务和案例规格不能同时为空")
	}
	if len(tasks) > 200 || len(specs) > 500 {
		return nil, errors.New("单次最多导入 200 个缺口任务和 500 个案例规格")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	taskIDs := map[string]int64{}
	for _, input := range tasks {
		id, publicID, err := upsertGalleryGapTask(ctx, tx, input, adminID)
		if err != nil {
			return nil, err
		}
		taskIDs[publicID] = id
	}
	for _, input := range specs {
		taskID, ok := taskIDs[input.GapTaskPublicID]
		if !ok {
			if err := tx.QueryRow(ctx, "SELECT id FROM gallery_gap_tasks WHERE public_id=$1", input.GapTaskPublicID).Scan(&taskID); err != nil {
				return nil, fmt.Errorf("缺口任务 %s 不存在", input.GapTaskPublicID)
			}
		}
		normalized, validationErrors, err := normalizeGalleryCaseSpecInput(input)
		if err != nil {
			return nil, err
		}
		if normalized.Status == "ready" && len(validationErrors) > 0 {
			return nil, fmt.Errorf("规格 %s 不能标记为 ready：%s", normalized.Title, strings.Join(validationErrors, "；"))
		}
		if normalized.Status == "ready" && len(normalized.RiskFlags) > 0 && normalized.RiskReviewStatus != "approved" {
			return nil, fmt.Errorf("规格 %s 存在未完成的风险审核", normalized.Title)
		}
		if _, err := upsertGalleryCaseSpec(ctx, tx, taskID, normalized, validationErrors, adminID); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, "UPDATE gallery_gap_tasks SET status=CASE WHEN status='open' THEN 'in_progress' ELSE status END, updated_by=$1, updated_at=now() WHERE id=$2", adminID, taskID); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	items, _, err := s.ListGalleryCaseSpecs(ctx, "", "", 1, 500)
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (s *GalleryService) UpdateGalleryCaseSpec(ctx context.Context, id, adminID int64, input GalleryCaseSpecUpdateInput) (*GalleryCaseSpecAdminItem, error) {
	current, err := s.GetGalleryCaseSpec(ctx, id)
	if err != nil {
		return nil, err
	}
	merged := GalleryCaseSpecInput{
		PublicID: current.PublicID, GapTaskPublicID: current.GapTaskPublicID, Title: current.Title, Purpose: current.Purpose,
		Prompt: current.Prompt, NegativePrompt: current.NegativePrompt, PromptLanguage: current.PromptLanguage,
		ImageTextLanguages: current.ImageTextLanguages, ExpectedText: current.ExpectedText, Category: current.Category,
		Styles: current.Styles, Scenes: current.Scenes, AspectRatio: current.AspectRatio, OutputMediaType: current.OutputMediaType,
		CandidateModels: current.CandidateModels, AcceptanceCriteria: current.AcceptanceCriteria, SelectionReason: current.SelectionReason,
		RiskFlags: current.RiskFlags, RiskNotes: current.RiskNotes, RiskReviewStatus: current.RiskReviewStatus,
		Status: current.Status, GeneratorType: current.GeneratorType, GeneratorModelCode: current.GeneratorModelCode,
		GeneratorPromptVersion: current.GeneratorPromptVersion, GenerationRequestID: current.GenerationRequestID,
		GenerationInput: current.GenerationInput, GenerationOutput: current.GenerationOutput, EstimatedCost: current.EstimatedCost,
		ActualCost: current.ActualCost, InputTokens: current.InputTokens, OutputTokens: current.OutputTokens,
	}
	applyGalleryCaseSpecUpdate(&merged, input)
	normalized, validationErrors, err := normalizeGalleryCaseSpecInput(merged)
	if err != nil {
		return nil, err
	}
	if normalized.Status == "ready" && len(validationErrors) > 0 {
		return nil, fmt.Errorf("规格不能标记为 ready：%s", strings.Join(validationErrors, "；"))
	}
	if normalized.Status == "ready" && len(normalized.RiskFlags) > 0 && normalized.RiskReviewStatus != "approved" {
		return nil, errors.New("规格存在风险标记，完成风险审核并标记为 approved 后才能进入 ready")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	nextVersion := current.Version + 1
	if err := updateGalleryCaseSpec(ctx, tx, id, nextVersion, normalized, validationErrors, adminID); err != nil {
		return nil, err
	}
	if err := insertGalleryCaseSpecVersion(ctx, tx, id, nextVersion, normalized, validationErrors, adminID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetGalleryCaseSpec(ctx, id)
}

func (s *GalleryService) ListGalleryCaseSpecVersions(ctx context.Context, id int64) ([]map[string]interface{}, error) {
	rows, err := s.db.Query(ctx, "SELECT version, change_type, snapshot, changed_by, created_at FROM gallery_case_spec_versions WHERE spec_id=$1 ORDER BY version DESC", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]map[string]interface{}, 0)
	for rows.Next() {
		var version int
		var changeType string
		var snapshot []byte
		var changedBy *int64
		var createdAt time.Time
		if err := rows.Scan(&version, &changeType, &snapshot, &changedBy, &createdAt); err != nil {
			return nil, err
		}
		value := map[string]interface{}{}
		if err := json.Unmarshal(snapshot, &value); err != nil {
			return nil, err
		}
		value["version"], value["change_type"], value["changed_by"], value["created_at"] = version, changeType, changedBy, createdAt.Format(time.RFC3339)
		items = append(items, value)
	}
	return items, rows.Err()
}

func (s *GalleryService) SetGalleryGapTaskStatus(ctx context.Context, id int64, status string, adminID int64) error {
	status = strings.TrimSpace(status)
	if !validGalleryGapStatus(status) {
		return errors.New("无效的缺口任务状态")
	}
	result, err := s.db.Exec(ctx, "UPDATE gallery_gap_tasks SET status=$1, updated_by=$2, updated_at=now() WHERE id=$3", status, adminID, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func upsertGalleryGapTask(ctx context.Context, tx pgx.Tx, input GalleryGapTaskInput, adminID int64) (int64, string, error) {
	input, err := normalizeGalleryGapTaskInput(input)
	if err != nil {
		return 0, "", err
	}
	fingerprint := galleryGapTaskFingerprint(input)
	publicID := strings.TrimSpace(input.PublicID)
	if publicID == "" {
		publicID = "gap_" + fingerprint[:20]
	}
	risk, _ := json.Marshal(uniqueStrings(input.RiskFlags))
	sourceContext, _ := json.Marshal(input.SourceContext)
	var snapshot interface{}
	if strings.TrimSpace(input.SourceSnapshotAt) != "" {
		parsed, parseErr := time.Parse(time.RFC3339, input.SourceSnapshotAt)
		if parseErr != nil {
			return 0, "", errors.New("source_snapshot_at 必须是 RFC3339 时间")
		}
		snapshot = parsed
	}
	var id int64
	err = tx.QueryRow(ctx, `
		INSERT INTO gallery_gap_tasks
		(public_id,fingerprint,source_report_hash,source_snapshot_at,dimension_type,category,style,scene,prompt_language,
		 target_count,current_count,gap_count,priority,title,rationale,risk_flags,source_context,created_by,updated_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)
		ON CONFLICT (fingerprint) DO UPDATE SET
			source_report_hash=EXCLUDED.source_report_hash,source_snapshot_at=EXCLUDED.source_snapshot_at,
			target_count=EXCLUDED.target_count,current_count=EXCLUDED.current_count,gap_count=EXCLUDED.gap_count,
			priority=EXCLUDED.priority,title=EXCLUDED.title,rationale=EXCLUDED.rationale,
			risk_flags=EXCLUDED.risk_flags,source_context=EXCLUDED.source_context,
			updated_by=EXCLUDED.updated_by,updated_at=now()
		RETURNING id,public_id
	`, publicID, fingerprint, input.SourceReportHash, snapshot, input.DimensionType, input.Category, input.Style,
		input.Scene, input.PromptLanguage, input.TargetCount, input.CurrentCount, input.GapCount, input.Priority,
		input.Title, input.Rationale, risk, sourceContext, adminID, adminID).Scan(&id, &publicID)
	if err != nil {
		return 0, "", err
	}
	return id, publicID, nil
}

func upsertGalleryCaseSpec(ctx context.Context, tx pgx.Tx, taskID int64, input GalleryCaseSpecInput, validationErrors []string, adminID int64) (int64, error) {
	publicID := strings.TrimSpace(input.PublicID)
	if publicID == "" {
		publicID = "spec_" + galleryCaseSpecHash(input)[:20]
	}
	input = normalizeGalleryCaseSpecDefaults(input)
	args := galleryCaseSpecArgs(publicID, taskID, input, validationErrors, adminID)
	var id int64
	var version int
	err := tx.QueryRow(ctx, `
		INSERT INTO gallery_case_specs
		(public_id,gap_task_id,status,title,purpose,prompt,negative_prompt,prompt_language,image_text_languages,expected_text,
		 category,styles,scenes,aspect_ratio,output_media_type,candidate_models,acceptance_criteria,selection_reason,
		 risk_flags,risk_notes,risk_review_status,generator_type,generator_model_code,generator_prompt_version,
		 generation_request_id,generation_input,generation_output,estimated_cost,actual_cost,input_tokens,output_tokens,
		 validation_errors,content_hash,created_by,updated_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31,$32,$33,$34,$35)
		ON CONFLICT (public_id) DO UPDATE SET
			gap_task_id=EXCLUDED.gap_task_id,status=EXCLUDED.status,title=EXCLUDED.title,purpose=EXCLUDED.purpose,
			prompt=EXCLUDED.prompt,negative_prompt=EXCLUDED.negative_prompt,prompt_language=EXCLUDED.prompt_language,
			image_text_languages=EXCLUDED.image_text_languages,expected_text=EXCLUDED.expected_text,category=EXCLUDED.category,
			styles=EXCLUDED.styles,scenes=EXCLUDED.scenes,aspect_ratio=EXCLUDED.aspect_ratio,
			output_media_type=EXCLUDED.output_media_type,candidate_models=EXCLUDED.candidate_models,
			acceptance_criteria=EXCLUDED.acceptance_criteria,selection_reason=EXCLUDED.selection_reason,
			risk_flags=EXCLUDED.risk_flags,risk_notes=EXCLUDED.risk_notes,risk_review_status=EXCLUDED.risk_review_status,
			generator_type=EXCLUDED.generator_type,generator_model_code=EXCLUDED.generator_model_code,
			generator_prompt_version=EXCLUDED.generator_prompt_version,generation_request_id=EXCLUDED.generation_request_id,
			generation_input=EXCLUDED.generation_input,generation_output=EXCLUDED.generation_output,
			estimated_cost=EXCLUDED.estimated_cost,actual_cost=EXCLUDED.actual_cost,input_tokens=EXCLUDED.input_tokens,
			output_tokens=EXCLUDED.output_tokens,validation_errors=EXCLUDED.validation_errors,content_hash=EXCLUDED.content_hash,
			version=CASE WHEN gallery_case_specs.content_hash=EXCLUDED.content_hash
				AND gallery_case_specs.status=EXCLUDED.status
				AND gallery_case_specs.risk_flags=EXCLUDED.risk_flags
				AND gallery_case_specs.risk_notes=EXCLUDED.risk_notes
				AND gallery_case_specs.risk_review_status=EXCLUDED.risk_review_status
				THEN gallery_case_specs.version ELSE gallery_case_specs.version+1 END,
			updated_by=EXCLUDED.updated_by,updated_at=now()
		RETURNING id,version
	`, args...).Scan(&id, &version)
	if err != nil {
		return 0, err
	}
	if err := insertGalleryCaseSpecVersion(ctx, tx, id, version, input, validationErrors, adminID); err != nil {
		return 0, err
	}
	return id, nil
}

func updateGalleryCaseSpec(ctx context.Context, tx pgx.Tx, id int64, version int, input GalleryCaseSpecInput, validationErrors []string, adminID int64) error {
	args := galleryCaseSpecArgs("", 0, input, validationErrors, adminID)
	// Drop the insert-only public_id and gap_task_id, but keep updated_by.
	// The UPDATE statement has 34 placeholders: version, 31 spec fields,
	// updated_by, and id.
	args = append(args[2:34], id)
	_, err := tx.Exec(ctx, `
		UPDATE gallery_case_specs SET
		version=$1,status=$2,title=$3,purpose=$4,prompt=$5,negative_prompt=$6,prompt_language=$7,
		image_text_languages=$8,expected_text=$9,category=$10,styles=$11,scenes=$12,aspect_ratio=$13,
		output_media_type=$14,candidate_models=$15,acceptance_criteria=$16,selection_reason=$17,
		risk_flags=$18,risk_notes=$19,risk_review_status=$20,generator_type=$21,generator_model_code=$22,
		generator_prompt_version=$23,generation_request_id=$24,generation_input=$25,generation_output=$26,
		estimated_cost=$27,actual_cost=$28,input_tokens=$29,output_tokens=$30,validation_errors=$31,
		content_hash=$32,updated_by=$33,updated_at=now() WHERE id=$34
	`, append([]interface{}{version}, args...)...)
	return err
}

func insertGalleryCaseSpecVersion(ctx context.Context, tx pgx.Tx, id int64, version int, input GalleryCaseSpecInput, validationErrors []string, adminID int64) error {
	snapshot := map[string]interface{}{}
	raw, _ := json.Marshal(input)
	_ = json.Unmarshal(raw, &snapshot)
	snapshot["version"], snapshot["validation_errors"] = version, validationErrors
	snapshotRaw, _ := json.Marshal(snapshot)
	changeType := "edited"
	if version == 1 {
		changeType = "created"
	}
	if input.GeneratorType != "manual" && input.GeneratorType != "" {
		changeType = "generated"
	}
	if input.Status == "ready" {
		changeType = "ready"
	}
	_, err := tx.Exec(ctx, "INSERT INTO gallery_case_spec_versions (spec_id,version,change_type,snapshot,changed_by) VALUES ($1,$2,$3,$4,$5) ON CONFLICT (spec_id,version) DO UPDATE SET snapshot=EXCLUDED.snapshot,change_type=EXCLUDED.change_type,changed_by=EXCLUDED.changed_by", id, version, changeType, snapshotRaw, adminID)
	return err
}

func galleryCaseSpecArgs(publicID string, taskID int64, input GalleryCaseSpecInput, validationErrors []string, adminID int64) []interface{} {
	imageText, _ := json.Marshal(input.ImageTextLanguages)
	expectedText, _ := json.Marshal(input.ExpectedText)
	styles, _ := json.Marshal(input.Styles)
	scenes, _ := json.Marshal(input.Scenes)
	candidates, _ := json.Marshal(input.CandidateModels)
	criteria, _ := json.Marshal(input.AcceptanceCriteria)
	risks, _ := json.Marshal(input.RiskFlags)
	inputJSON, _ := json.Marshal(input.GenerationInput)
	outputJSON, _ := json.Marshal(input.GenerationOutput)
	errorsJSON, _ := json.Marshal(validationErrors)
	return []interface{}{publicID, taskID, input.Status, input.Title, input.Purpose, input.Prompt, input.NegativePrompt, input.PromptLanguage, imageText, expectedText, input.Category, styles, scenes, input.AspectRatio, input.OutputMediaType, candidates, criteria, input.SelectionReason, risks, input.RiskNotes, input.RiskReviewStatus, input.GeneratorType, input.GeneratorModelCode, input.GeneratorPromptVersion, input.GenerationRequestID, inputJSON, outputJSON, input.EstimatedCost, input.ActualCost, input.InputTokens, input.OutputTokens, errorsJSON, galleryCaseSpecHash(input), adminID, adminID}
}

func normalizeGalleryGapTaskInput(input GalleryGapTaskInput) (GalleryGapTaskInput, error) {
	input.SourceReportHash = strings.TrimSpace(input.SourceReportHash)
	if input.SourceReportHash == "" {
		input.SourceReportHash = "manual"
	}
	input.DimensionType = strings.TrimSpace(input.DimensionType)
	if !validGalleryGapDimension(input.DimensionType) {
		return input, errors.New("缺口任务 dimension_type 无效")
	}
	input.Category, input.Style, input.Scene, input.PromptLanguage = strings.TrimSpace(input.Category), strings.TrimSpace(input.Style), strings.TrimSpace(input.Scene), strings.TrimSpace(input.PromptLanguage)
	if input.TargetCount <= 0 {
		return input, errors.New("缺口任务 target_count 必须大于 0")
	}
	if input.CurrentCount < 0 || input.GapCount < 0 {
		return input, errors.New("缺口任务数量不能为负数")
	}
	if input.GapCount == 0 {
		input.GapCount = maxInt(input.TargetCount-input.CurrentCount, 0)
	}
	input.Priority = strings.ToLower(strings.TrimSpace(input.Priority))
	if input.Priority == "" {
		input.Priority = "medium"
	}
	if !validGalleryGapPriority(input.Priority) {
		return input, errors.New("缺口任务 priority 无效")
	}
	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" {
		input.Title = galleryGapTaskTitle(input)
	}
	if input.SourceContext == nil {
		input.SourceContext = map[string]interface{}{}
	}
	input.RiskFlags = uniqueStrings(input.RiskFlags)
	return input, nil
}

func normalizeGalleryCaseSpecDefaults(input GalleryCaseSpecInput) GalleryCaseSpecInput {
	input.Title, input.Purpose, input.Prompt, input.NegativePrompt = strings.TrimSpace(input.Title), strings.TrimSpace(input.Purpose), strings.TrimSpace(input.Prompt), strings.TrimSpace(input.NegativePrompt)
	input.PromptLanguage, input.Category, input.AspectRatio = strings.TrimSpace(input.PromptLanguage), strings.TrimSpace(input.Category), strings.TrimSpace(input.AspectRatio)
	input.OutputMediaType, input.Status, input.GeneratorType = strings.TrimSpace(input.OutputMediaType), strings.TrimSpace(input.Status), strings.TrimSpace(input.GeneratorType)
	if input.OutputMediaType == "" {
		input.OutputMediaType = "image"
	}
	if input.Status == "" {
		input.Status = "draft"
	}
	if input.GeneratorType == "" {
		input.GeneratorType = "manual"
	}
	input.Styles, input.Scenes = uniqueStrings(input.Styles), uniqueStrings(input.Scenes)
	input.ImageTextLanguages, input.ExpectedText = uniqueStrings(input.ImageTextLanguages), uniqueStrings(input.ExpectedText)
	input.AcceptanceCriteria, input.RiskFlags = uniqueStrings(input.AcceptanceCriteria), uniqueStrings(append(input.RiskFlags, detectGalleryRiskFlags(input)...))
	if input.RiskReviewStatus == "" || (len(input.RiskFlags) > 0 && input.RiskReviewStatus == "not_required") {
		input.RiskReviewStatus = riskReviewStatus(input.RiskFlags)
	}
	return input
}

func normalizeGalleryCaseSpecInput(input GalleryCaseSpecInput) (GalleryCaseSpecInput, []string, error) {
	input = normalizeGalleryCaseSpecDefaults(input)
	if !validGalleryCaseSpecStatus(input.Status) {
		return input, nil, errors.New("案例规格 status 无效")
	}
	if !validGalleryRiskReviewStatus(input.RiskReviewStatus) {
		return input, nil, errors.New("案例规格 risk_review_status 无效")
	}
	return input, validateGalleryCaseSpec(input), nil
}

func validateGalleryCaseSpec(input GalleryCaseSpecInput) []string {
	items := []string{}
	if input.Title == "" {
		items = append(items, "缺少标题")
	}
	if input.Purpose == "" {
		items = append(items, "缺少用途说明")
	}
	if input.Prompt == "" {
		items = append(items, "缺少 Prompt")
	}
	if input.PromptLanguage == "" {
		items = append(items, "缺少 Prompt 语言")
	}
	if input.Category == "" {
		items = append(items, "缺少分类")
	}
	if len(input.Styles) == 0 {
		items = append(items, "至少需要一个风格标签")
	}
	if len(input.Scenes) == 0 {
		items = append(items, "至少需要一个场景标签")
	}
	if !validAspectRatio(input.AspectRatio) {
		items = append(items, "画幅比例无效")
	}
	if input.OutputMediaType != "image" {
		items = append(items, "当前只允许 image 媒体类型")
	}
	if len(input.CandidateModels) == 0 {
		items = append(items, "至少需要一个候选图片模型")
	}
	if len(input.AcceptanceCriteria) < 3 {
		items = append(items, "至少需要 3 条验收标准")
	}
	if len(input.RiskFlags) > 0 && input.RiskReviewStatus != "approved" {
		items = append(items, "风险标记未完成人工审核")
	}
	if input.RiskReviewStatus == "rejected" {
		items = append(items, "风险审核已拒绝")
	}
	return uniqueStrings(items)
}

func applyGalleryCaseSpecUpdate(target *GalleryCaseSpecInput, input GalleryCaseSpecUpdateInput) {
	if input.Title != nil {
		target.Title = *input.Title
	}
	if input.Purpose != nil {
		target.Purpose = *input.Purpose
	}
	if input.Prompt != nil {
		target.Prompt = *input.Prompt
	}
	if input.NegativePrompt != nil {
		target.NegativePrompt = *input.NegativePrompt
	}
	if input.PromptLanguage != nil {
		target.PromptLanguage = *input.PromptLanguage
	}
	if input.ImageTextLanguages != nil {
		target.ImageTextLanguages = *input.ImageTextLanguages
	}
	if input.ExpectedText != nil {
		target.ExpectedText = *input.ExpectedText
	}
	if input.Category != nil {
		target.Category = *input.Category
	}
	if input.Styles != nil {
		target.Styles = *input.Styles
	}
	if input.Scenes != nil {
		target.Scenes = *input.Scenes
	}
	if input.AspectRatio != nil {
		target.AspectRatio = *input.AspectRatio
	}
	if input.CandidateModels != nil {
		target.CandidateModels = *input.CandidateModels
	}
	if input.AcceptanceCriteria != nil {
		target.AcceptanceCriteria = *input.AcceptanceCriteria
	}
	if input.SelectionReason != nil {
		target.SelectionReason = *input.SelectionReason
	}
	if input.RiskNotes != nil {
		target.RiskNotes = *input.RiskNotes
	}
	if input.RiskReviewStatus != nil {
		target.RiskReviewStatus = *input.RiskReviewStatus
	}
	if input.Status != nil {
		target.Status = *input.Status
	}
}

func detectGalleryRiskFlags(input GalleryCaseSpecInput) []string {
	// Negative prompts commonly contain words such as "logo" or "public figures"
	// as prohibited content. They should not turn an otherwise safe concept into
	// a false-positive risk review task.
	text := strings.ToLower(strings.Join([]string{input.Title, input.Purpose, input.Prompt, strings.Join(input.ExpectedText, " ")}, "\n"))
	rules := map[string][]string{
		"brand_or_trademark":  {"品牌", "商标", "logo", "nike", "apple", "tesla", "openai", "可口可乐"},
		"public_figure":       {"名人", "公众人物", "明星", "elon musk", "donald trump", "sam altman", "马斯克", "特朗普"},
		"copyright_character": {"版权角色", "pokemon", "disney", "marvel", "mickey", "哈利波特", "原神"},
		"real_institution":    {"真实机构", "真实学校", "真实医院", "真实大学", "university", "hospital", "government"},
		"sensitive_topic":     {"政治", "选举", "战争", "暴力", "血腥", "未成年人", "医疗", "election", "war", "medical"},
	}
	flags := []string{}
	for flag, words := range rules {
		for _, word := range words {
			if galleryRiskSignalPresent(text, word) {
				flags = append(flags, flag)
				break
			}
		}
	}
	sort.Strings(flags)
	return uniqueStrings(flags)
}

func galleryRiskSignalPresent(text, word string) bool {
	start := 0
	for start < len(text) {
		index := strings.Index(text[start:], word)
		if index < 0 {
			return false
		}
		index += start
		// Use a generous UTF-8 byte window so short Chinese negations such as
		// "不出现真实品牌" are not truncated before the marker.
		prefixStart := index - 48
		if prefixStart < 0 {
			prefixStart = 0
		}
		prefix := text[prefixStart:index]
		negative := []string{"不", "无", "避免", "不要", "禁止", "不使用", "不出现", "non-", "without", "avoid", "no ", "not "}
		blocked := false
		for _, marker := range negative {
			if strings.Contains(prefix, marker) {
				blocked = true
				break
			}
		}
		if !blocked {
			return true
		}
		start = index + len(word)
	}
	return false
}

func galleryCaseSpecHash(input GalleryCaseSpecInput) string {
	raw, _ := json.Marshal([]interface{}{input.Title, input.Purpose, input.Prompt, input.NegativePrompt, input.PromptLanguage, input.ImageTextLanguages, input.ExpectedText, input.Category, input.Styles, input.Scenes, input.AspectRatio, input.CandidateModels, input.AcceptanceCriteria})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func galleryGapTaskFingerprint(input GalleryGapTaskInput) string {
	raw, _ := json.Marshal([]string{input.SourceReportHash, input.DimensionType, input.Category, input.Style, input.Scene, input.PromptLanguage})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func galleryGapTaskTitle(input GalleryGapTaskInput) string {
	values := []string{}
	for _, value := range []string{input.Category, input.Style, input.Scene, input.PromptLanguage} {
		if strings.TrimSpace(value) != "" {
			values = append(values, value)
		}
	}
	if len(values) == 0 {
		return "灵感广场案例缺口"
	}
	return strings.Join(values, " + ") + " 案例缺口"
}

func riskReviewStatus(flags []string) string {
	if len(uniqueStrings(flags)) > 0 {
		return "pending"
	}
	return "not_required"
}
func validGalleryGapDimension(value string) bool {
	switch value {
	case "category", "style", "scene", "category_scene", "category_style", "scene_language", "custom":
		return true
	}
	return false
}
func validGalleryGapPriority(value string) bool {
	return value == "low" || value == "medium" || value == "high" || value == "critical"
}
func validGalleryGapStatus(value string) bool {
	return value == "open" || value == "in_progress" || value == "fulfilled" || value == "dismissed"
}
func validGalleryCaseSpecStatus(value string) bool {
	return value == "draft" || value == "ready" || value == "archived"
}
func validGalleryRiskReviewStatus(value string) bool {
	return value == "not_required" || value == "pending" || value == "approved" || value == "rejected"
}
func validAspectRatio(value string) bool {
	switch strings.TrimSpace(value) {
	case "1:1", "4:5", "3:4", "4:3", "16:9", "9:16", "21:9", "auto":
		return true
	}
	return false
}
func normalizeGalleryPage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 30
	}
	return page, pageSize
}
func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}

func decodeStringArray(raw []byte) []string {
	var values []string
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &values)
	}
	return uniqueStrings(values)
}
func decodeObject(raw []byte) map[string]interface{} {
	value := map[string]interface{}{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &value)
	}
	return value
}
func decodeCandidateModels(raw []byte) []GalleryCaseSpecModelCandidate {
	var values []GalleryCaseSpecModelCandidate
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &values)
	}
	return values
}

func scanGalleryGapTask(scanner interface{ Scan(...any) error }) (*GalleryGapTaskAdminItem, error) {
	item := &GalleryGapTaskAdminItem{}
	var snapshot *time.Time
	var riskRaw, contextRaw []byte
	var created, updated time.Time
	if err := scanner.Scan(&item.ID, &item.PublicID, &item.Fingerprint, &item.SourceReportHash, &snapshot, &item.DimensionType, &item.Category, &item.Style, &item.Scene, &item.PromptLanguage, &item.TargetCount, &item.CurrentCount, &item.GapCount, &item.Priority, &item.Title, &item.Rationale, &riskRaw, &contextRaw, &item.Status, &item.SpecCount, &item.ReadySpecCount, &created, &updated); err != nil {
		return nil, err
	}
	item.RiskFlags, item.SourceContext, item.CreatedAt, item.UpdatedAt = decodeStringArray(riskRaw), decodeObject(contextRaw), created.Format(time.RFC3339), updated.Format(time.RFC3339)
	if snapshot != nil {
		value := snapshot.Format(time.RFC3339)
		item.SourceSnapshotAt = &value
	}
	return item, nil
}

func scanGalleryCaseSpec(scanner interface{ Scan(...any) error }) (*GalleryCaseSpecAdminItem, error) {
	item := &GalleryCaseSpecAdminItem{}
	var imageTextRaw, expectedRaw, stylesRaw, scenesRaw, candidatesRaw, criteriaRaw, risksRaw, inputRaw, outputRaw, errorsRaw []byte
	var created, updated time.Time
	if err := scanner.Scan(&item.ID, &item.PublicID, &item.GapTaskID, &item.GapTaskPublicID, &item.GapTaskTitle, &item.Version, &item.Status, &item.Title, &item.Purpose, &item.Prompt, &item.NegativePrompt, &item.PromptLanguage, &imageTextRaw, &expectedRaw, &item.Category, &stylesRaw, &scenesRaw, &item.AspectRatio, &item.OutputMediaType, &candidatesRaw, &criteriaRaw, &item.SelectionReason, &risksRaw, &item.RiskNotes, &item.RiskReviewStatus, &item.GeneratorType, &item.GeneratorModelCode, &item.GeneratorPromptVersion, &item.GenerationRequestID, &inputRaw, &outputRaw, &item.EstimatedCost, &item.ActualCost, &item.InputTokens, &item.OutputTokens, &errorsRaw, &item.ContentHash, &created, &updated); err != nil {
		return nil, err
	}
	item.ImageTextLanguages, item.ExpectedText, item.Styles, item.Scenes = decodeStringArray(imageTextRaw), decodeStringArray(expectedRaw), decodeStringArray(stylesRaw), decodeStringArray(scenesRaw)
	item.CandidateModels, item.AcceptanceCriteria, item.RiskFlags = decodeCandidateModels(candidatesRaw), decodeStringArray(criteriaRaw), decodeStringArray(risksRaw)
	item.GenerationInput, item.GenerationOutput, item.ValidationErrors = decodeObject(inputRaw), decodeObject(outputRaw), decodeStringArray(errorsRaw)
	item.CreatedAt, item.UpdatedAt = created.Format(time.RFC3339), updated.Format(time.RFC3339)
	return item, nil
}

var _ = pgx.ErrNoRows

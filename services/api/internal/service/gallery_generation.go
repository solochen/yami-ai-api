package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/starai/api/internal/queue"
)

const galleryGenerationCostRuleVersion = "model.price_rule.v1"
const galleryGenerationActiveTaskStatuses = "'queued','submitting','processing','pending_review'"

var ErrGalleryGenerationSpecActive = errors.New("案例规格已有活跃生成任务")

type GalleryGenerationService struct {
	db     *pgxpool.Pool
	models *ModelService
	queue  *asynq.Client
}

func NewGalleryGenerationService(db *pgxpool.Pool, models *ModelService, client *asynq.Client) *GalleryGenerationService {
	return &GalleryGenerationService{db: db, models: models, queue: client}
}

type GalleryGenerationBatchAdminItem struct {
	ID              int64   `json:"id"`
	PublicID        string  `json:"public_id"`
	Status          string  `json:"status"`
	RequestedBy     *int64  `json:"requested_by,omitempty"`
	SpecCount       int     `json:"spec_count"`
	TaskCount       int     `json:"task_count"`
	CompletedCount  int     `json:"completed_count"`
	FailedCount     int     `json:"failed_count"`
	CancelledCount  int     `json:"cancelled_count"`
	ActiveCount     int     `json:"active_count"`
	MaxConcurrency  int     `json:"max_concurrency"`
	MaxRetries      int     `json:"max_retries"`
	BudgetLimit     float64 `json:"budget_limit"`
	EstimatedCost   float64 `json:"estimated_cost"`
	ActualCost      float64 `json:"actual_cost"`
	Currency        string  `json:"currency"`
	CostRuleVersion string  `json:"cost_rule_version"`
	ErrorMessage    string  `json:"error_message,omitempty"`
	CreatedAt       string  `json:"created_at"`
	StartedAt       string  `json:"started_at,omitempty"`
	FinishedAt      string  `json:"finished_at,omitempty"`
	CancelledAt     string  `json:"cancelled_at,omitempty"`
	UpdatedAt       string  `json:"updated_at"`
}

type GalleryGenerationTaskAdminItem struct {
	ID                 int64                  `json:"id"`
	PublicID           string                 `json:"public_id"`
	BatchID            int64                  `json:"batch_id"`
	BatchPublicID      string                 `json:"batch_public_id"`
	SpecID             int64                  `json:"spec_id"`
	SpecPublicID       string                 `json:"spec_public_id"`
	SpecTitle          string                 `json:"spec_title"`
	Attempt            int                    `json:"attempt"`
	Status             string                 `json:"status"`
	ModelCode          string                 `json:"model_code"`
	ProviderModel      string                 `json:"provider_model"`
	RequestPayload     map[string]interface{} `json:"request_payload"`
	RequestID          string                 `json:"request_id"`
	UpstreamTaskID     string                 `json:"upstream_task_id,omitempty"`
	Output             map[string]interface{} `json:"output"`
	OutputAssetURL     string                 `json:"output_asset_url,omitempty"`
	OutputThumbnailURL string                 `json:"output_thumbnail_url,omitempty"`
	EstimatedCost      float64                `json:"estimated_cost"`
	ActualCost         float64                `json:"actual_cost"`
	ProviderCost       float64                `json:"provider_cost"`
	RetryCount         int                    `json:"retry_count"`
	MaxRetries         int                    `json:"max_retries"`
	ErrorCode          string                 `json:"error_code,omitempty"`
	ErrorMessage       string                 `json:"error_message,omitempty"`
	StartedAt          string                 `json:"started_at,omitempty"`
	FinishedAt         string                 `json:"finished_at,omitempty"`
	CreatedAt          string                 `json:"created_at"`
	UpdatedAt          string                 `json:"updated_at"`
}

type CreateGalleryGenerationBatchInput struct {
	SpecIDs        []int64 `json:"spec_ids"`
	MaxConcurrency int     `json:"max_concurrency"`
	MaxRetries     int     `json:"max_retries"`
	BudgetLimit    float64 `json:"budget_limit"`
}

type GalleryGenerationOccupiedSpecAdminItem struct {
	SpecID        int64  `json:"spec_id"`
	SpecPublicID  string `json:"spec_public_id"`
	SpecTitle     string `json:"spec_title"`
	TaskStatus    string `json:"task_status"`
	BatchID       int64  `json:"batch_id"`
	BatchPublicID string `json:"batch_public_id"`
	BatchStatus   string `json:"batch_status"`
}

type GalleryGenerationEligibilityAdminItem struct {
	ReadySpecCount     int                                      `json:"ready_spec_count"`
	AvailableSpecCount int                                      `json:"available_spec_count"`
	OccupiedSpecCount  int                                      `json:"occupied_spec_count"`
	OccupiedSpecs      []GalleryGenerationOccupiedSpecAdminItem `json:"occupied_specs"`
}

type galleryGenerationSpecRow struct {
	id              int64
	publicID        string
	title           string
	prompt          string
	negativePrompt  string
	aspectRatio     string
	candidatesRaw   []byte
	generationInput []byte
}

func (s *GalleryGenerationService) ListBatches(ctx context.Context, status string, page, pageSize int) ([]GalleryGenerationBatchAdminItem, int, error) {
	page, pageSize = normalizeGalleryPage(page, pageSize)
	status = strings.TrimSpace(status)
	args := []interface{}{}
	where := " WHERE 1=1"
	if status != "" {
		if !validGalleryGenerationBatchStatus(status) {
			return nil, 0, errors.New("无效的生成批次状态")
		}
		args = append(args, status)
		where += " AND status=$1"
	}
	var total int
	if err := s.db.QueryRow(ctx, "SELECT COUNT(*) FROM gallery_generation_batches"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := s.db.Query(ctx, `SELECT id,public_id,status,requested_by,spec_count,task_count,completed_count,failed_count,cancelled_count,active_count,max_concurrency,max_retries,budget_limit,estimated_cost,actual_cost,currency,cost_rule_version,COALESCE(error_message,''),created_at,started_at,finished_at,cancelled_at,updated_at FROM gallery_generation_batches`+where+` ORDER BY created_at DESC,id DESC LIMIT $`+fmt.Sprint(len(args)-1)+` OFFSET $`+fmt.Sprint(len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]GalleryGenerationBatchAdminItem, 0)
	for rows.Next() {
		item, err := scanGalleryGenerationBatch(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *item)
	}
	return items, total, rows.Err()
}

func (s *GalleryGenerationService) GetBatch(ctx context.Context, id int64) (*GalleryGenerationBatchAdminItem, error) {
	return scanGalleryGenerationBatch(s.db.QueryRow(ctx, `SELECT id,public_id,status,requested_by,spec_count,task_count,completed_count,failed_count,cancelled_count,active_count,max_concurrency,max_retries,budget_limit,estimated_cost,actual_cost,currency,cost_rule_version,COALESCE(error_message,''),created_at,started_at,finished_at,cancelled_at,updated_at FROM gallery_generation_batches WHERE id=$1`, id))
}

func (s *GalleryGenerationService) ListTasks(ctx context.Context, batchID int64, page, pageSize int) ([]GalleryGenerationTaskAdminItem, int, error) {
	page, pageSize = normalizeGalleryPage(page, pageSize)
	var total int
	if err := s.db.QueryRow(ctx, `SELECT COUNT(*) FROM gallery_generation_tasks WHERE batch_id=$1`, batchID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.Query(ctx, `SELECT t.id,t.public_id,t.batch_id,b.public_id,t.spec_id,s.public_id,s.title,t.attempt,t.status,t.model_code,t.provider_model,t.request_payload,t.request_id,t.upstream_task_id,t.output,t.output_asset_url,t.output_thumbnail_url,t.estimated_cost,t.actual_cost,t.provider_cost,t.retry_count,t.max_retries,COALESCE(t.error_code,''),COALESCE(t.error_message,''),t.started_at,t.finished_at,t.created_at,t.updated_at FROM gallery_generation_tasks t JOIN gallery_generation_batches b ON b.id=t.batch_id JOIN gallery_case_specs s ON s.id=t.spec_id WHERE t.batch_id=$1 ORDER BY t.created_at ASC,t.id ASC LIMIT $2 OFFSET $3`, batchID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]GalleryGenerationTaskAdminItem, 0)
	for rows.Next() {
		item, err := scanGalleryGenerationTask(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *item)
	}
	return items, total, rows.Err()
}

func (s *GalleryGenerationService) GetEligibility(ctx context.Context) (*GalleryGenerationEligibilityAdminItem, error) {
	rows, err := s.db.Query(ctx, `
		SELECT s.id,s.public_id,s.title,t.status,b.id,b.public_id,b.status
		FROM gallery_case_specs s
		LEFT JOIN gallery_generation_tasks t
		  ON t.spec_id=s.id AND t.status IN (`+galleryGenerationActiveTaskStatuses+`)
		LEFT JOIN gallery_generation_batches b ON b.id=t.batch_id
		WHERE s.status='ready'
		ORDER BY s.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := &GalleryGenerationEligibilityAdminItem{OccupiedSpecs: []GalleryGenerationOccupiedSpecAdminItem{}}
	for rows.Next() {
		result.ReadySpecCount++
		var specID int64
		var specPublicID, specTitle string
		var taskStatus, batchPublicID, batchStatus *string
		var batchID *int64
		if err := rows.Scan(&specID, &specPublicID, &specTitle, &taskStatus, &batchID, &batchPublicID, &batchStatus); err != nil {
			return nil, err
		}
		if taskStatus == nil {
			result.AvailableSpecCount++
			continue
		}
		result.OccupiedSpecs = append(result.OccupiedSpecs, GalleryGenerationOccupiedSpecAdminItem{
			SpecID: specID, SpecPublicID: specPublicID, SpecTitle: specTitle, TaskStatus: *taskStatus,
			BatchID: valueOrZero(batchID), BatchPublicID: valueOrEmpty(batchPublicID), BatchStatus: valueOrEmpty(batchStatus),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	result.OccupiedSpecCount = len(result.OccupiedSpecs)
	return result, nil
}

func (s *GalleryGenerationService) CreateBatch(ctx context.Context, adminID int64, input CreateGalleryGenerationBatchInput) (*GalleryGenerationBatchAdminItem, error) {
	if input.MaxConcurrency == 0 {
		input.MaxConcurrency = 2
	}
	if input.MaxRetries == 0 {
		input.MaxRetries = 2
	}
	if input.MaxConcurrency < 1 || input.MaxConcurrency > 20 {
		return nil, errors.New("max_concurrency 必须在 1 到 20 之间")
	}
	if input.MaxRetries < 0 || input.MaxRetries > 5 {
		return nil, errors.New("max_retries 必须在 0 到 5 之间")
	}
	if input.BudgetLimit < 0 {
		return nil, errors.New("budget_limit 不能为负数")
	}
	if len(input.SpecIDs) > 500 {
		return nil, errors.New("单批次最多选择 500 条规格")
	}

	specs, err := s.loadReadySpecs(ctx, input.SpecIDs)
	if err != nil {
		return nil, err
	}
	if len(specs) == 0 {
		return nil, s.noAvailableSpecsError(ctx, input.SpecIDs)
	}

	type preparedTask struct {
		specID, modelID                     int64
		modelCode, providerModel, requestID string
		payload                             map[string]interface{}
		cost                                float64
	}
	prepared := make([]preparedTask, 0, len(specs))
	var estimated float64
	for _, spec := range specs {
		var candidates []GalleryCaseSpecModelCandidate
		if err := json.Unmarshal(spec.candidatesRaw, &candidates); err != nil {
			return nil, fmt.Errorf("规格 %s 候选模型 JSON 无效", spec.publicID)
		}
		var selected *ModelFull
		for _, candidate := range candidates {
			code := strings.TrimSpace(candidate.Code)
			if code == "" {
				continue
			}
			model, modelErr := s.models.GetFullByCode(ctx, code)
			if modelErr != nil || !model.IsEnabled || model.Category != "image" || model.RequestMode != "images" {
				continue
			}
			selected = model
			break
		}
		if selected == nil {
			return nil, fmt.Errorf("规格 %s 没有可用的启用图片模型或线路", spec.publicID)
		}
		params := cloneJSONMap(selected.DefaultParams)
		var generated map[string]interface{}
		if len(spec.generationInput) > 0 {
			if err := json.Unmarshal(spec.generationInput, &generated); err != nil {
				return nil, fmt.Errorf("规格 %s generation_input JSON 无效", spec.publicID)
			}
			for key, value := range generated {
				params[key] = value
			}
		}
		params["prompt"] = spec.prompt
		if spec.negativePrompt != "" {
			params["negative_prompt"] = spec.negativePrompt
		}
		if spec.aspectRatio != "" {
			params["aspect_ratio"] = spec.aspectRatio
		}
		if _, ok := params["n"]; !ok {
			params["n"] = 1
		}
		cost := s.models.EstimateCost(selected, params, 0, 0)
		requestID := "gallery_" + uuid.NewString()
		prepared = append(prepared, preparedTask{specID: spec.id, modelID: selected.ID, modelCode: selected.Code, providerModel: selected.NewAPIModel, requestID: requestID, payload: params, cost: cost})
		estimated += cost
	}
	if input.BudgetLimit > 0 && estimated > input.BudgetLimit+0.00000001 {
		return nil, fmt.Errorf("批次预估算力 %.6f 超出预算上限 %.6f", estimated, input.BudgetLimit)
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	publicID := "g_batch_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	var batchID int64
	if err := tx.QueryRow(ctx, `INSERT INTO gallery_generation_batches (public_id,status,requested_by,spec_count,task_count,max_concurrency,max_retries,budget_limit,estimated_cost,cost_rule_version) VALUES ($1,'draft',$2,$3,$3,$4,$5,$6,$7,$8) RETURNING id`, publicID, adminID, len(prepared), input.MaxConcurrency, input.MaxRetries, input.BudgetLimit, estimated, galleryGenerationCostRuleVersion).Scan(&batchID); err != nil {
		return nil, err
	}
	for _, task := range prepared {
		payload, _ := json.Marshal(task.payload)
		taskPublicID := "g_task_" + strings.ReplaceAll(uuid.NewString(), "-", "")
		if _, err := tx.Exec(ctx, `INSERT INTO gallery_generation_tasks (public_id,batch_id,spec_id,model_id,model_code,provider_model,request_payload,request_id,estimated_cost,max_retries) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, taskPublicID, batchID, task.specID, task.modelID, task.modelCode, task.providerModel, payload, task.requestID, task.cost, input.MaxRetries); err != nil {
			return nil, galleryGenerationTaskInsertError(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetBatch(ctx, batchID)
}

func (s *GalleryGenerationService) StartBatch(ctx context.Context, id int64) (*GalleryGenerationBatchAdminItem, error) {
	if s.queue == nil {
		return nil, errors.New("生成队列未配置")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var status string
	if err := tx.QueryRow(ctx, `SELECT status FROM gallery_generation_batches WHERE id=$1 FOR UPDATE`, id).Scan(&status); err != nil {
		return nil, err
	}
	if status != "draft" && status != "queued" && status != "running" {
		return nil, fmt.Errorf("批次当前状态 %s 不允许启动", status)
	}
	if _, err := tx.Exec(ctx, `UPDATE gallery_generation_batches SET status='queued',error_message=NULL,updated_at=now(),started_at=COALESCE(started_at,now()) WHERE id=$1`, id); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT id,public_id FROM gallery_generation_tasks WHERE batch_id=$1 AND status='queued' ORDER BY id`, id)
	if err != nil {
		return nil, err
	}
	tasks := make([]queue.GalleryGenerationTaskPayload, 0)
	for rows.Next() {
		var payload queue.GalleryGenerationTaskPayload
		if err := rows.Scan(&payload.TaskID, &payload.PublicID); err != nil {
			rows.Close()
			return nil, err
		}
		tasks = append(tasks, payload)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	for _, payload := range tasks {
		if err := queue.EnqueueGalleryGenerationTask(s.queue, payload); err != nil {
			_, _ = s.db.Exec(ctx, `UPDATE gallery_generation_batches SET status='failed',error_message=$2,finished_at=now(),updated_at=now() WHERE id=$1 AND status IN ('queued','running')`, id, "队列投递失败："+err.Error())
			return nil, err
		}
	}
	return s.GetBatch(ctx, id)
}

func (s *GalleryGenerationService) CancelBatch(ctx context.Context, id int64) (*GalleryGenerationBatchAdminItem, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var status string
	if err := tx.QueryRow(ctx, `SELECT status FROM gallery_generation_batches WHERE id=$1 FOR UPDATE`, id).Scan(&status); err != nil {
		return nil, err
	}
	if status == "completed" || status == "partial_failed" || status == "failed" || status == "cancelled" {
		return nil, fmt.Errorf("批次当前状态 %s 不允许取消", status)
	}
	if _, err := tx.Exec(ctx, `UPDATE gallery_generation_tasks SET status='cancelled',finished_at=now(),updated_at=now(),error_code='CANCELLED',error_message='管理员取消批次' WHERE batch_id=$1 AND status IN ('queued','submitting','processing')`, id); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE gallery_generation_batches SET status='cancelled',cancelled_at=now(),finished_at=now(),updated_at=now() WHERE id=$1`, id); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetBatch(ctx, id)
}

func (s *GalleryGenerationService) RetryTask(ctx context.Context, id int64) (*GalleryGenerationTaskAdminItem, error) {
	if s.queue == nil {
		return nil, errors.New("生成队列未配置")
	}
	var payload queue.GalleryGenerationTaskPayload
	var status string
	if err := s.db.QueryRow(ctx, `UPDATE gallery_generation_tasks SET status='queued',attempt=attempt+1,retry_count=retry_count+1,request_id=$2,error_code=NULL,error_message=NULL,started_at=NULL,finished_at=NULL,updated_at=now() WHERE id=$1 AND status IN ('failed','expired','unknown') AND retry_count < max_retries RETURNING id,public_id,status`, id, "gallery_"+uuid.NewString()).Scan(&payload.TaskID, &payload.PublicID, &status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("任务当前不可重试，或已达到最大重试次数")
		}
		return nil, err
	}
	if _, err := s.db.Exec(ctx, `UPDATE gallery_generation_batches SET status='queued',error_message=NULL,finished_at=NULL,updated_at=now() WHERE id=(SELECT batch_id FROM gallery_generation_tasks WHERE id=$1) AND status IN ('failed','partial_failed','cancelled')`, id); err != nil {
		return nil, err
	}
	if err := queue.EnqueueGalleryGenerationTask(s.queue, payload); err != nil {
		return nil, err
	}
	return s.GetTask(ctx, id)
}

func (s *GalleryGenerationService) GetTask(ctx context.Context, id int64) (*GalleryGenerationTaskAdminItem, error) {
	return scanGalleryGenerationTask(s.db.QueryRow(ctx, `SELECT t.id,t.public_id,t.batch_id,b.public_id,t.spec_id,s.public_id,s.title,t.attempt,t.status,t.model_code,t.provider_model,t.request_payload,t.request_id,t.upstream_task_id,t.output,t.output_asset_url,t.output_thumbnail_url,t.estimated_cost,t.actual_cost,t.provider_cost,t.retry_count,t.max_retries,COALESCE(t.error_code,''),COALESCE(t.error_message,''),t.started_at,t.finished_at,t.created_at,t.updated_at FROM gallery_generation_tasks t JOIN gallery_generation_batches b ON b.id=t.batch_id JOIN gallery_case_specs s ON s.id=t.spec_id WHERE t.id=$1`, id))
}

func (s *GalleryGenerationService) loadReadySpecs(ctx context.Context, ids []int64) ([]galleryGenerationSpecRow, error) {
	query := `SELECT s.id,s.public_id,s.title,s.prompt,s.negative_prompt,s.aspect_ratio,s.candidate_models,s.generation_input
		FROM gallery_case_specs s
		WHERE s.status='ready'
		AND NOT EXISTS (
			SELECT 1 FROM gallery_generation_tasks t
			WHERE t.spec_id=s.id AND t.status IN (` + galleryGenerationActiveTaskStatuses + `)
		)`
	args := []interface{}{}
	if len(ids) > 0 {
		placeholders := make([]string, len(ids))
		for i, id := range ids {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
			args = append(args, id)
		}
		query += " AND s.id IN (" + strings.Join(placeholders, ",") + ")"
	}
	query += " ORDER BY s.id"
	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]galleryGenerationSpecRow, 0)
	for rows.Next() {
		var item galleryGenerationSpecRow
		if err := rows.Scan(&item.id, &item.publicID, &item.title, &item.prompt, &item.negativePrompt, &item.aspectRatio, &item.candidatesRaw, &item.generationInput); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *GalleryGenerationService) noAvailableSpecsError(ctx context.Context, ids []int64) error {
	query := `SELECT COALESCE(string_agg(DISTINCT b.public_id, ', ' ORDER BY b.public_id), '')
		FROM gallery_case_specs s
		JOIN gallery_generation_tasks t ON t.spec_id=s.id AND t.status IN (` + galleryGenerationActiveTaskStatuses + `)
		JOIN gallery_generation_batches b ON b.id=t.batch_id
		WHERE s.status='ready'`
	args := []interface{}{}
	if len(ids) > 0 {
		placeholders := make([]string, len(ids))
		for i, id := range ids {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
			args = append(args, id)
		}
		query += " AND s.id IN (" + strings.Join(placeholders, ",") + ")"
	}
	var batches string
	if err := s.db.QueryRow(ctx, query, args...).Scan(&batches); err == nil && batches != "" {
		return fmt.Errorf("%w：这些 ready 规格正在批次 %s 中排队、处理或等待审核，请继续处理现有批次", ErrGalleryGenerationSpecActive, batches)
	}
	return errors.New("没有可创建任务的 ready 规格，请先完成规格审核并将状态设为 ready")
}

func galleryGenerationTaskInsertError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "uq_gallery_generation_active_spec" {
		return fmt.Errorf("%w：另一个请求刚刚为同一规格创建了任务，请刷新批次列表后继续现有任务", ErrGalleryGenerationSpecActive)
	}
	return fmt.Errorf("创建生成任务失败：%w", err)
}

func valueOrZero(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func cloneJSONMap(input map[string]interface{}) map[string]interface{} {
	output := make(map[string]interface{}, len(input)+4)
	for key, value := range input {
		output[key] = value
	}
	return output
}

func validGalleryGenerationBatchStatus(value string) bool {
	switch value {
	case "draft", "queued", "running", "completed", "partial_failed", "failed", "cancelled":
		return true
	default:
		return false
	}
}

func scanGalleryGenerationBatch(scanner interface{ Scan(...any) error }) (*GalleryGenerationBatchAdminItem, error) {
	var item GalleryGenerationBatchAdminItem
	var created, updated time.Time
	var started, finished, cancelled *time.Time
	if err := scanner.Scan(&item.ID, &item.PublicID, &item.Status, &item.RequestedBy, &item.SpecCount, &item.TaskCount, &item.CompletedCount, &item.FailedCount, &item.CancelledCount, &item.ActiveCount, &item.MaxConcurrency, &item.MaxRetries, &item.BudgetLimit, &item.EstimatedCost, &item.ActualCost, &item.Currency, &item.CostRuleVersion, &item.ErrorMessage, &created, &started, &finished, &cancelled, &updated); err != nil {
		return nil, err
	}
	item.CreatedAt, item.UpdatedAt = created.Format(time.RFC3339), updated.Format(time.RFC3339)
	if started != nil {
		item.StartedAt = started.Format(time.RFC3339)
	}
	if finished != nil {
		item.FinishedAt = finished.Format(time.RFC3339)
	}
	if cancelled != nil {
		item.CancelledAt = cancelled.Format(time.RFC3339)
	}
	return &item, nil
}

func scanGalleryGenerationTask(scanner interface{ Scan(...any) error }) (*GalleryGenerationTaskAdminItem, error) {
	var item GalleryGenerationTaskAdminItem
	var payloadRaw, outputRaw []byte
	var created, updated time.Time
	var started, finished *time.Time
	if err := scanner.Scan(&item.ID, &item.PublicID, &item.BatchID, &item.BatchPublicID, &item.SpecID, &item.SpecPublicID, &item.SpecTitle, &item.Attempt, &item.Status, &item.ModelCode, &item.ProviderModel, &payloadRaw, &item.RequestID, &item.UpstreamTaskID, &outputRaw, &item.OutputAssetURL, &item.OutputThumbnailURL, &item.EstimatedCost, &item.ActualCost, &item.ProviderCost, &item.RetryCount, &item.MaxRetries, &item.ErrorCode, &item.ErrorMessage, &started, &finished, &created, &updated); err != nil {
		return nil, err
	}
	item.RequestPayload, item.Output = map[string]interface{}{}, map[string]interface{}{}
	_ = json.Unmarshal(payloadRaw, &item.RequestPayload)
	_ = json.Unmarshal(outputRaw, &item.Output)
	item.CreatedAt, item.UpdatedAt = created.Format(time.RFC3339), updated.Format(time.RFC3339)
	if started != nil {
		item.StartedAt = started.Format(time.RFC3339)
	}
	if finished != nil {
		item.FinishedAt = finished.Format(time.RFC3339)
	}
	return &item, nil
}

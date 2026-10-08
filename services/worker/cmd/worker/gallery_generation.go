package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// processGalleryGenerationTask deliberately keeps the gallery ledger separate
// from tasks/works/wallets. It may produce a durable object, but the only
// terminal success state it writes is pending_review.
func processGalleryGenerationTask(ctx context.Context, pool *pgxpool.Pool, baseURL, token string, payload GalleryGenerationTaskPayload) error {
	if payload.TaskID <= 0 || strings.TrimSpace(payload.PublicID) == "" {
		return errors.New("gallery generation payload is incomplete")
	}

	lockConn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer lockConn.Release()
	var locked bool
	if err := lockConn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, -payload.TaskID).Scan(&locked); err != nil {
		return err
	}
	if !locked {
		return nil
	}
	defer func() { _, _ = lockConn.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, -payload.TaskID) }()

	var batchID, modelID int64
	var status, modelCode string
	var requestPayloadRaw []byte
	var estimated float64
	var maxRetries, retryCount int
	var batchStatus string
	var requestMode, category, endpoint, upstreamModel string
	var extraRaw, runtimeRaw []byte
	if err := pool.QueryRow(ctx, `SELECT t.batch_id,t.status,t.model_id,t.model_code,t.request_payload,t.estimated_cost,t.max_retries,t.retry_count,b.status,m.request_mode,m.category,m.new_api_model,m.new_api_endpoint,m.new_api_extra_params,m.runtime_rule FROM gallery_generation_tasks t JOIN gallery_generation_batches b ON b.id=t.batch_id JOIN models m ON m.id=t.model_id WHERE t.id=$1 AND t.public_id=$2`, payload.TaskID, payload.PublicID).
		Scan(&batchID, &status, &modelID, &modelCode, &requestPayloadRaw, &estimated, &maxRetries, &retryCount, &batchStatus, &requestMode, &category, &upstreamModel, &endpoint, &extraRaw, &runtimeRaw); err != nil {
		return err
	}
	if status != "queued" && status != "submitting" && status != "processing" {
		return nil
	}
	if batchStatus == "cancelled" {
		_, _ = pool.Exec(ctx, `UPDATE gallery_generation_tasks SET status='cancelled',error_code='CANCELLED',error_message='批次已取消',finished_at=now(),updated_at=now() WHERE id=$1 AND status IN ('queued','submitting','processing')`, payload.TaskID)
		return nil
	}

	claimed := false
	if err := pool.QueryRow(ctx, `UPDATE gallery_generation_batches SET active_count=active_count+1,status='running',started_at=COALESCE(started_at,now()),updated_at=now() WHERE id=$1 AND status IN ('queued','running') AND active_count < max_concurrency RETURNING id`, batchID).Scan(&batchID); err != nil {
		// Asynq will retry this task after another gallery slot is released.
		return &RetryableError{Code: "GALLERY_CONCURRENCY", Message: "gallery batch concurrency limit reached"}
	}
	claimed = true
	defer func() {
		if claimed {
			_, _ = pool.Exec(context.Background(), `UPDATE gallery_generation_batches SET active_count=GREATEST(active_count-1,0),updated_at=now() WHERE id=$1`, batchID)
			refreshGalleryBatchProgress(context.Background(), pool, batchID)
		}
	}()

	if _, err := pool.Exec(ctx, `UPDATE gallery_generation_tasks SET status='processing',started_at=COALESCE(started_at,now()),updated_at=now() WHERE id=$1 AND status IN ('queued','submitting','processing')`, payload.TaskID); err != nil {
		return err
	}
	input := map[string]interface{}{}
	if err := json.Unmarshal(requestPayloadRaw, &input); err != nil {
		return failGalleryGenerationTask(ctx, pool, payload.TaskID, "INVALID_PAYLOAD", "生成参数 JSON 无效", false)
	}
	requestMode = normalizeWorkerMediaRequestMode(requestMode, category)
	if requestMode != "images" && requestMode != "image" {
		return failGalleryGenerationTask(ctx, pool, payload.TaskID, "MODEL_NOT_IMAGE", "批量图库生成只允许图片模型", false)
	}

	legacyExtra := parseJSONMap(extraRaw)
	legacyRuntime := parseJSONMap(runtimeRaw)
	routes, err := loadWorkerModelRoutes(ctx, pool, modelID, baseURL, token, upstreamModel, endpoint, legacyExtra, legacyRuntime)
	if err != nil {
		return failGalleryGenerationTask(ctx, pool, payload.TaskID, "MODEL_ROUTE_ERROR", "无法加载模型线路："+err.Error(), isTransientError("MODEL_ROUTE_ERROR", 0, err.Error()))
	}
	p := ImageTaskPayload{TaskNo: payload.PublicID, ModelID: modelID, ModelCode: modelCode, Input: input}
	var selected workerGenerationAttemptResult
	var lastErr error
	attempt := 0
	poolEnabled := len(routes) > 1
	selectedOK := false
	for _, route := range routes {
		if poolEnabled && !acquireWorkerRouteProbe(ctx, pool, route) {
			continue
		}
		for retry := 0; retry <= route.MaxRetries; retry++ {
			if attempt >= maxWorkerRouteAttempts {
				break
			}
			attempt++
			started := time.Now()
			candidate, callErr := executeWorkerGenerationAttempt(ctx, pool, p, route, false, false, true, stringAny(input["prompt"]))
			latency := int(time.Since(started).Milliseconds())
			if callErr == nil && candidate.StatusCode < 400 && len(candidate.ResultData) == 0 {
				candidate.ResultData, candidate.UpstreamTaskID = parseUpstreamMedia(candidate.ResponseBody)
				if len(candidate.ResultData) == 0 && candidate.UpstreamTaskID == "" {
					callErr = errors.New("upstream returned no usable result")
				}
			}
			if callErr == nil && candidate.StatusCode < 400 {
				selected, selectedOK = candidate, true
				markWorkerRouteSuccess(ctx, pool, route.ID)
				logWorkerRouteAttempt(ctx, pool, payload.PublicID, modelID, route.ID, attempt, "success", candidate.StatusCode, latency)
				break
			}
			lastErr = callErr
			if callErr == nil {
				lastErr = fmt.Errorf("upstream HTTP %d: %s", candidate.StatusCode, upstreamErrorMessage(candidate.ResponseBody))
			}
			if callErr == nil && !workerStatusCanFailover(candidate.StatusCode) {
				logWorkerRouteAttempt(ctx, pool, payload.PublicID, modelID, route.ID, attempt, "rejected", candidate.StatusCode, latency)
				return failGalleryGenerationTask(ctx, pool, payload.TaskID, "MODEL_PROVIDER_ERROR", upstreamErrorMessage(candidate.ResponseBody), false)
			}
			markWorkerRouteFailure(ctx, pool, route.ID, poolEnabled)
			logWorkerRouteAttempt(ctx, pool, payload.PublicID, modelID, route.ID, attempt, "failed", candidate.StatusCode, latency)
			if retry < route.MaxRetries && workerShouldRetrySameRoute(callErr, candidate.StatusCode, poolEnabled) && waitWorkerRouteRetry(ctx, retry) {
				continue
			}
			break
		}
		if selectedOK {
			break
		}
	}
	if !selectedOK {
		message := "所有可用线路均调用失败"
		if lastErr != nil {
			message += "：" + humanizeUpstreamFailure(lastErr.Error())
		}
		transient := isTransientError("MODEL_PROVIDER_ERROR", 0, message)
		return failGalleryGenerationTask(ctx, pool, payload.TaskID, "MODEL_PROVIDER_ERROR", message, transient && retryCount < maxRetries)
	}

	resultData, upstreamID := selected.ResultData, selected.UpstreamTaskID
	if len(resultData) == 0 {
		resultData, upstreamID = parseUpstreamMedia(selected.ResponseBody)
		if len(resultData) == 0 && upstreamID != "" {
			pollCfg := parsePollConfig(selected.RuntimeRule, selected.Endpoint)
			if path := upstreamPollPath(selected.ResponseBody, selected.Connection.BaseURL); path != "" {
				pollCfg.Path = path
			}
			resultData, _, err = pollUpstreamTask(ctx, pool, selected.Connection, pollCfg, upstreamID, payload.PublicID)
			if err != nil {
				return failGalleryGenerationTask(ctx, pool, payload.TaskID, "MODEL_PROVIDER_ERROR", err.Error(), isTransientError("MODEL_PROVIDER_ERROR", 0, err.Error()) && retryCount < maxRetries)
			}
		}
	}
	if len(resultData) == 0 {
		return failGalleryGenerationTask(ctx, pool, payload.TaskID, "EMPTY_OUTPUT", "生成完成但未返回可用图片", false)
	}

	usage := upstreamUsageFromBody(selected.ResponseBody)
	billingInput := inputWithActualUpstreamUsage(input, usage)
	actualCost := estimated
	if usage.hasAny() {
		actualCost = estimateModelCostByIDWorker(ctx, pool, modelID, billingInput, usage.PromptTokens, usage.OutputTokens, 0, 0)
	}
	providerCost := workerRouteProviderCost(selected.Route, billingInput, usage.PromptTokens, usage.OutputTokens, 0, 0)
	if usd := upstreamUSDCost(selected.ResponseBody); usd > 0 {
		if rate := modelUSDToComputeRate(ctx, pool, modelID); rate > 0 {
			actualCost = usd * rate
			providerCost = actualCost
		} else {
			providerCost = usd
		}
	}
	images := make([]map[string]string, 0, len(resultData))
	for idx, item := range resultData {
		url := normalizeImageResultURL(item.URL, item.B64JSON)
		if item.B64JSON != "" {
			stored, storeErr := storeBase64MediaResult(ctx, payload.PublicID, idx+1, item.B64JSON, item.MimeType, "image")
			if storeErr != nil {
				log.Printf("Gallery task %s store base64 image #%d failed: %v", payload.PublicID, idx+1, storeErr)
			} else if stored != "" {
				url = stored
			}
		}
		if url != "" && item.B64JSON == "" {
			stored, storeErr := persistGeneratedMedia(ctx, selected.Connection, url, payload.PublicID, fmt.Sprintf("image_%d", idx+1), "image", 50<<20)
			if storeErr != nil {
				return failGalleryGenerationTask(ctx, pool, payload.TaskID, "MEDIA_PERSIST_FAILED", "生成成功但图片转存失败："+storeErr.Error(), false)
			}
			if stored != "" {
				url = stored
			}
		}
		if url != "" {
			images = append(images, map[string]string{"url": url})
		}
	}
	if len(images) == 0 {
		return failGalleryGenerationTask(ctx, pool, payload.TaskID, "EMPTY_OUTPUT", "生成结果无法保存为图片", false)
	}
	output := map[string]interface{}{"image_url": images[0]["url"], "images": images, "upstream_task_id": upstreamID, "review_status": "pending_review"}
	outputRaw, _ := json.Marshal(output)
	_, err = pool.Exec(ctx, `UPDATE gallery_generation_tasks SET status='pending_review',attempt=attempt,upstream_task_id=$2,output=$3,output_asset_url=$4,output_thumbnail_url=$4,actual_cost=$5,provider_cost=$6,error_code=NULL,error_message=NULL,finished_at=now(),updated_at=now() WHERE id=$1 AND status='processing'`, payload.TaskID, upstreamID, outputRaw, images[0]["url"], actualCost, providerCost)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO gallery_quality_reviews (public_id,task_id,spec_id)
		SELECT 'g_review_' || md5(t.id::text || clock_timestamp()::text), t.id, t.spec_id
		FROM gallery_generation_tasks t
		WHERE t.id=$1
		ON CONFLICT (task_id) DO NOTHING`, payload.TaskID)
	if err != nil {
		return err
	}
	log.Printf("Gallery generation task %s completed and is pending review", payload.PublicID)
	return nil
}

func failGalleryGenerationTask(ctx context.Context, pool *pgxpool.Pool, taskID int64, code, message string, retry bool) error {
	if retry && !isFinalAsynqAttempt(ctx) {
		_, _ = pool.Exec(ctx, `UPDATE gallery_generation_tasks SET status='queued',retry_count=retry_count+1,error_code=$2,error_message=$3,updated_at=now() WHERE id=$1 AND status IN ('queued','submitting','processing')`, taskID, code, message)
		return &RetryableError{Code: code, Message: message}
	}
	_, err := pool.Exec(ctx, `UPDATE gallery_generation_tasks SET status='failed',error_code=$2,error_message=$3,finished_at=now(),updated_at=now() WHERE id=$1 AND status IN ('queued','submitting','processing')`, taskID, code, message)
	return err
}

func parseJSONMap(raw []byte) map[string]interface{} {
	value := map[string]interface{}{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &value)
	}
	return value
}

func refreshGalleryBatchProgress(ctx context.Context, pool *pgxpool.Pool, batchID int64) {
	_, _ = pool.Exec(ctx, `WITH counts AS (
		SELECT batch_id,
			COUNT(*) FILTER (WHERE status IN ('pending_review','succeeded')) AS completed_count,
			COUNT(*) FILTER (WHERE status IN ('failed','expired','unknown')) AS failed_count,
			COUNT(*) FILTER (WHERE status='cancelled') AS cancelled_count,
			COUNT(*) FILTER (WHERE status IN ('queued','submitting','processing')) AS active_tasks,
			COALESCE(SUM(actual_cost),0) AS actual_cost,
			COUNT(*) AS task_count
		FROM gallery_generation_tasks WHERE batch_id=$1 GROUP BY batch_id
	)
	UPDATE gallery_generation_batches b SET
		completed_count=c.completed_count,failed_count=c.failed_count,cancelled_count=c.cancelled_count,actual_cost=c.actual_cost,updated_at=now(),
		status=CASE
			WHEN c.completed_count+c.failed_count+c.cancelled_count=c.task_count AND c.failed_count=0 AND c.cancelled_count=0 THEN 'completed'
			WHEN c.completed_count+c.failed_count+c.cancelled_count=c.task_count AND c.completed_count>0 THEN 'partial_failed'
			WHEN c.completed_count+c.failed_count+c.cancelled_count=c.task_count AND c.cancelled_count=c.task_count THEN 'cancelled'
			WHEN c.completed_count+c.failed_count+c.cancelled_count=c.task_count AND c.failed_count>0 THEN 'failed'
			WHEN b.status IN ('queued','running') THEN 'running'
			ELSE b.status
		END,
		finished_at=CASE WHEN c.completed_count+c.failed_count+c.cancelled_count=c.task_count THEN COALESCE(b.finished_at,now()) ELSE NULL END
	FROM counts c WHERE b.id=c.batch_id`, batchID)
}

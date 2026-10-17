package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/starai/api/internal/billing"
	"github.com/starai/api/internal/queue"
	"github.com/starai/api/internal/runtime"
)

// 电商图片工厂：输入校验/计价、items 级重入（重生成/精修/继续）、反推与 ZIP 导出。
// 提示词与模块目录的权威定义在 worker 侧 ecom_factory_catalog.json（有对拍测试守护），
// API 侧只做结构性校验，不重复模板。

const (
	ecomFactoryMaxPlanItems  = 24
	ecomFactoryMaxPerModule  = 3
	ecomFactoryMaxAssets     = 14
	ecomFactoryMaxBatchCount = 200
)

var ecomFactoryModuleIDs = map[string]bool{
	"main-square": true, "white-bg": true, "scene-hero": true, "detail-cover": true,
	"texture-close": true, "selling-grid": true, "compare": true, "seed-cover": true,
	"lifestyle": true, "live-sticker": true, "style-clone": true, "color-series": true,
	"shop-banner": true, "reference-batch": true,
}

func normalizeEcomFactoryInputs(runtimeCfg, inputs map[string]interface{}) map[string]interface{} {
	if inputs == nil {
		inputs = map[string]interface{}{}
	}
	defaultStr := func(key, fallback string) {
		if strings.TrimSpace(stringValue(inputs[key])) == "" {
			inputs[key] = fallback
		}
	}
	defaultStr("mode", "suite")
	defaultStr("platform", "淘宝/天猫")
	defaultStr("language", "简体中文")
	defaultStr("visualStyle", "高端商业摄影")
	defaultStr("resolution", firstAgentString(stringValue(runtimeCfg["default_resolution"]), "1K"))
	defaultStr("selectedSuite", "launch")
	if stringValue(inputs["mode"]) != "batch" && intFromAgentAny(inputs["countPerModule"]) <= 0 {
		inputs["countPerModule"] = 1
	}
	if _, ok := inputs["copyEnabled"].(bool); !ok {
		inputs["copyEnabled"] = false
	}
	inputs["count"], inputs["n"] = 1, 1
	return inputs
}

func validateEcomFactoryRequest(ctx context.Context, userID int64, runtimeCfg, inputs map[string]interface{}) error {
	mode := stringValue(inputs["mode"])
	assets := agentAnySlice(inputs["assets"])
	if len(assets) > ecomFactoryMaxAssets {
		return errors.New("上传图片数量超出限制（商品图 8 张 + 风格参考 6 张）")
	}
	hasAssetText := func() bool {
		for _, raw := range assets {
			if m, _ := raw.(map[string]interface{}); m != nil && strings.TrimSpace(stringValue(m["url"])) != "" {
				return true
			}
		}
		return false
	}()
	switch mode {
	case "batch":
		if strings.TrimSpace(stringValue(inputs["batchPrompt"])) == "" {
			return errors.New("请先填写批量生成提示词")
		}
		if n := intFromAgentAny(inputs["batchCount"]); n < 1 || n > ecomFactoryMaxBatchCount {
			return errors.New("批量张数需在 1-200 之间")
		}
	case "quick":
		if strings.TrimSpace(stringValue(inputs["quickPrompt"])) == "" {
			return errors.New("请输入或反推出提示词")
		}
	default:
		modules := agentStringSliceValue(inputs["selectedModuleIds"])
		if len(modules) == 0 {
			if suite := strings.TrimSpace(stringValue(inputs["selectedSuite"])); suite != "" {
				modules = ecomFactorySuiteModules(suite)
			}
		}
		if len(modules) == 0 {
			return errors.New("请先选择生成模块")
		}
		for _, id := range modules {
			if !ecomFactoryModuleIDs[id] {
				return fmt.Errorf("未知图片类别：%s", id)
			}
		}
		if !hasAssetText &&
			strings.TrimSpace(stringValue(inputs["productName"])) == "" &&
			strings.TrimSpace(stringValue(inputs["sellingPoints"])) == "" {
			return errors.New("请先补充商品图、商品信息或选择生成模块")
		}
	}
	resolution := strings.ToUpper(strings.TrimSpace(stringValue(inputs["resolution"])))
	if resolution != "" {
		supported := agentStringSliceValue(runtimeCfg["supported_resolutions"])
		if len(supported) > 0 {
			found := false
			for _, r := range supported {
				if strings.EqualFold(r, resolution) {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("模型暂不支持 %s 清晰度", resolution)
			}
		}
	}
	if _, err := ecomFactoryPlanCount(inputs); err != nil {
		return err
	}
	return nil
}

func ecomFactorySuiteModules(suite string) []string {
	switch suite {
	case "detail":
		return []string{"detail-cover", "texture-close", "selling-grid", "compare", "scene-hero"}
	case "social":
		return []string{"seed-cover", "lifestyle", "live-sticker", "scene-hero"}
	case "clone":
		return []string{"style-clone", "color-series", "main-square", "shop-banner"}
	case "batch":
		return []string{"reference-batch"}
	case "launch", "":
		return []string{"main-square", "white-bg", "scene-hero", "detail-cover", "seed-cover"}
	}
	return nil
}

// ecomFactoryPlanCount 与 worker 计划构建保持一致的张数口径（≤24）。
func ecomFactoryPlanCount(inputs map[string]interface{}) (int, error) {
	switch stringValue(inputs["mode"]) {
	case "batch":
		n := intFromAgentAny(inputs["batchCount"])
		if n < 1 {
			n = 5
		}
		if n > ecomFactoryMaxPlanItems {
			n = ecomFactoryMaxPlanItems
		}
		return n, nil
	case "quick":
		return 1, nil
	}
	modules := agentStringSliceValue(inputs["selectedModuleIds"])
	if len(modules) == 0 {
		modules = ecomFactorySuiteModules(stringValue(inputs["selectedSuite"]))
	}
	if len(modules) == 0 {
		return 0, errors.New("请先选择生成模块")
	}
	count := intFromAgentAny(inputs["countPerModule"])
	if count < 1 {
		count = 1
	}
	if count > ecomFactoryMaxPerModule {
		count = ecomFactoryMaxPerModule
	}
	total := len(modules) * count
	if total > ecomFactoryMaxPlanItems {
		total = ecomFactoryMaxPlanItems
	}
	return total, nil
}

func estimateEcomFactoryCost(ctx context.Context, s *AgentService, runtimeCfg, inputs map[string]interface{}) float64 {
	code := strings.TrimSpace(stringValue(runtimeCfg["generation_model_code"]))
	if code == "" {
		return 0
	}
	n, err := ecomFactoryPlanCount(inputs)
	if err != nil || n <= 0 {
		return 0
	}
	params := map[string]interface{}{
		"n": 1, "count": 1,
		"image_size": strings.ToUpper(firstAgentString(stringValue(inputs["resolution"]), "1K")),
	}
	return float64(n) * s.estimateModelCostByCode(ctx, code, params, 0, 0)
}

// ---------- items 级重入：regenerate / refine / continue ----------

type EcomItemActionRequest struct {
	Action      string `json:"action"` // regenerate refine continue
	ItemID      string `json:"itemId"`
	Prompt      string `json:"prompt"`
	Instruction string `json:"instruction"`
}

func (s *AgentService) EcomFactoryItemAction(ctx context.Context, userID int64, publicID string, req EcomItemActionRequest) error {
	var projectID int64
	var status string
	var inputsRaw, outputsRaw, runtimeRaw []byte
	err := s.db.QueryRow(ctx,
		`SELECT p.id, p.status, p.inputs, p.outputs, w.runtime_config
		 FROM workflow_projects p JOIN workflow_definitions w ON w.id=p.workflow_id
		 WHERE p.public_id=$1 AND p.user_id=$2 AND w.runtime_config->>'agent_mode'='ecom_factory'`, publicID, userID).
		Scan(&projectID, &status, &inputsRaw, &outputsRaw, &runtimeRaw)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("电商图片工厂项目不存在")
		}
		return err
	}
	if status == "running" || status == "pending" {
		return errors.New("项目正在生成中，请等待完成后再操作")
	}
	inputs, outputs, runtimeCfg := map[string]interface{}{}, map[string]interface{}{}, map[string]interface{}{}
	_ = json.Unmarshal(inputsRaw, &inputs)
	_ = json.Unmarshal(outputsRaw, &outputs)
	_ = json.Unmarshal(runtimeRaw, &runtimeCfg)
	state := ecomFactoryStateFromOutputs(outputs)
	items := ecomFactoryStateItems(state)
	if len(items) == 0 {
		return errors.New("未找到可重跑的生成项")
	}
	itemIDs := []string{}
	switch req.Action {
	case "continue":
		for _, item := range items {
			if item["status"] != "success" && stringValue(item["id"]) != "" {
				itemIDs = append(itemIDs, stringValue(item["id"]))
			}
		}
		if len(itemIDs) == 0 {
			return errors.New("没有需要重跑的生成项")
		}
	case "regenerate", "refine":
		if strings.TrimSpace(req.ItemID) == "" {
			return errors.New("缺少生成项 ID")
		}
		var found map[string]interface{}
		for _, item := range items {
			if stringValue(item["id"]) == req.ItemID {
				found = item
				break
			}
		}
		if found == nil {
			return errors.New("生成项不存在")
		}
		if req.Action == "refine" && strings.TrimSpace(stringValue(found["imageUrl"])) == "" {
			return errors.New("当前卡片还没有可精修的图片")
		}
		itemIDs = []string{req.ItemID}
	default:
		return errors.New("不支持的操作")
	}
	resolution := strings.ToUpper(firstAgentString(stringValue(state["resolution"]), stringValue(inputs["resolution"]), "1K"))
	params := map[string]interface{}{"n": 1, "count": 1, "image_size": resolution}
	perImage := s.estimateModelCostByCode(ctx, stringValue(runtimeCfg["generation_model_code"]), params, 0, 0)
	estimated := float64(len(itemIDs)) * perImage
	if estimated <= 0 {
		estimated = 0.01 * float64(len(itemIDs))
	}
	scope := map[string]interface{}{"action": req.Action, "itemIds": toAgentAnySlice(itemIDs)}
	if strings.TrimSpace(req.Prompt) != "" {
		scope["prompt"] = strings.TrimSpace(req.Prompt)
	}
	if req.Action == "refine" {
		instruction := strings.TrimSpace(req.Instruction)
		if instruction != "" {
			scope["instruction"] = instruction
		}
		for _, item := range items {
			if stringValue(item["id"]) == req.ItemID {
				if url := strings.TrimSpace(stringValue(item["imageUrl"])); url != "" {
					scope["reference"] = url
				}
				break
			}
		}
	}
	inputs["itemScope"] = scope
	inputs["_execution_budget"] = estimated + float64(agentStateSpent(outputs))
	if err := s.billing.FreezeWithFinalize(ctx, userID, estimated, "workflow", publicID, func(tx pgx.Tx) error {
		var currentStatus string
		if err := tx.QueryRow(ctx, `SELECT status FROM workflow_projects WHERE id=$1 FOR UPDATE`, projectID).Scan(&currentStatus); err != nil {
			return err
		}
		if currentStatus != status {
			return errors.New("项目状态已变化，请刷新后重试")
		}
		var locked bool
		if err := tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock($1)`, projectID).Scan(&locked); err != nil {
			return err
		}
		if !locked {
			return errors.New("项目正在处理中，请稍后重试")
		}
		result, err := tx.Exec(ctx, `UPDATE workflow_projects SET status='pending', inputs=$2, error_message=NULL, finished_at=NULL, updated_at=now() WHERE id=$1`, projectID, mustAgentJSON(inputs))
		if err != nil {
			return err
		}
		if result.RowsAffected() != 1 {
			return errors.New("项目状态已变化，请刷新后重试")
		}
		return nil
	}); err != nil {
		if errors.Is(err, billing.ErrInsufficientBalance) {
			return errors.New(billing.InsufficientBalanceMsg)
		}
		return err
	}
	if err := queue.EnqueueWorkflowTask(s.queue, queue.WorkflowTaskPayload{ProjectID: projectID, UserID: userID}); err != nil {
		cleanupErr := s.billing.UnfreezeWithFinalize(ctx, userID, estimated, "workflow", publicID, func(tx pgx.Tx) error {
			_, updateErr := tx.Exec(ctx, `UPDATE workflow_projects SET status=$2, error_message='重跑入队失败', finished_at=now(), updated_at=now() WHERE id=$1`, projectID, status)
			return updateErr
		})
		if cleanupErr != nil {
			return fmt.Errorf("重跑入队失败: %v；回滚冻结额度失败: %w", err, cleanupErr)
		}
		return err
	}
	return nil
}

func ecomFactoryStateFromOutputs(outputs map[string]interface{}) map[string]interface{} {
	state, _ := outputs["ecom_factory"].(map[string]interface{})
	return state
}

func ecomFactoryStateItems(state map[string]interface{}) []map[string]interface{} {
	items := []map[string]interface{}{}
	if state == nil {
		return items
	}
	if raw, ok := state["items"].([]interface{}); ok {
		for _, v := range raw {
			if m, _ := v.(map[string]interface{}); m != nil {
				items = append(items, m)
			}
		}
	}
	return items
}

func agentStateSpent(outputs map[string]interface{}) float64 {
	return floatValue(outputs["incurred_cost"])
}

// ---------- 反推提示词（免费，视觉模型直调） ----------

// resolveEcomReverseModel 返回可用的视觉反推模型：优先请求指定，其次 ecom_factory
// 定义绑定的 analysis_model_code，最后回退到任意启用的 glm chat 线路。
// ResolveEcomReverseModel 由 AgentService 持有 db 连接，统一解析反推模型。
func (s *AgentService) ResolveEcomReverseModel(ctx context.Context, requested string) (string, error) {
	return s.resolveEcomReverseModel(ctx, requested)
}

func (s *AgentService) resolveEcomReverseModel(ctx context.Context, requested string) (string, error) {
	if strings.TrimSpace(requested) != "" {
		return strings.TrimSpace(requested), nil
	}
	var bound string
	err := s.db.QueryRow(ctx, `SELECT w.runtime_config->>'analysis_model_code'
		FROM workflow_definitions w WHERE w.code='ecom_factory' AND w.is_enabled`).Scan(&bound)
	if err == nil && strings.TrimSpace(bound) != "" {
		return bound, nil
	}
	fallback := ""
	err = s.db.QueryRow(ctx, `SELECT m.code FROM models m
		WHERE m.is_enabled=true AND m.category='chat' AND (m.new_api_model ILIKE '%glm%' OR m.code ILIKE '%glm%')
		ORDER BY m.sort_order NULLS LAST, m.id LIMIT 1`).Scan(&fallback)
	if err != nil || fallback == "" {
		return "", errors.New("反推模型未配置，请联系管理员")
	}
	return fallback, nil
}

const ecomFactoryReversePrompt = "请把这张图片反推成一段可直接用于电商商品图 AI 生成的中文提示词，覆盖：商品主体与品类、材质质感、构图与机位、光影与氛围、背景与场景、色调与风格。只输出一段可直接使用的提示词，不要解释、不要分点、不要加引号。"

func (s *ChatService) EcomFactoryReversePrompt(ctx context.Context, modelCode, imageURL string) (string, error) {
	if strings.TrimSpace(modelCode) == "" {
		return "", errors.New("反推模型未配置")
	}
	model, err := s.models.GetFullByCode(ctx, modelCode)
	if err != nil {
		return "", fmt.Errorf("反推模型 %q 不可用: %w", modelCode, err)
	}
	routes, err := s.modelRuntimeRoutes(ctx, model)
	if err != nil {
		return "", err
	}
	messages := []map[string]interface{}{{
		"role": "user",
		"content": []map[string]interface{}{
			{"type": "image_url", "image_url": map[string]interface{}{"url": imageURL}},
			{"type": "text", "text": ecomFactoryReversePrompt},
		},
	}}
	var lastErr error
	for i := range routes {
		route := routes[i]
		name := route.UpstreamModel
		if name == "" {
			name = model.NewAPIModel
		}
		if name == "" {
			name = model.Code
		}
		resp, callErr := s.runtime.ChatCompletionWithConfig(ctx, route.Endpoint,
			runtime.ChatRequest{Model: name, Messages: messages},
			route.RequestExtraForRequest(model, ""))
		if callErr == nil {
			if len(resp.Choices) > 0 && strings.TrimSpace(resp.Choices[0].Message.Content) != "" {
				return strings.TrimSpace(resp.Choices[0].Message.Content), nil
			}
			lastErr = errors.New("反推模型未返回内容")
			continue
		}
		lastErr = callErr
	}
	if lastErr == nil {
		lastErr = errors.New("反推模型没有可用线路")
	}
	return "", lastErr
}

// ---------- ZIP 导出 ----------

var ecomFactoryFileSanitizer = regexp.MustCompile(`[\\/:*?"<>|]`)

func (s *AgentService) EcomFactoryExportZip(ctx context.Context, userID int64, publicID string) (*bytes.Buffer, string, error) {
	var status string
	var outputsRaw []byte
	err := s.db.QueryRow(ctx,
		`SELECT p.status, p.outputs FROM workflow_projects p
		 JOIN workflow_definitions w ON w.id=p.workflow_id
		 WHERE p.public_id=$1 AND p.user_id=$2 AND w.runtime_config->>'agent_mode'='ecom_factory'`, publicID, userID).
		Scan(&status, &outputsRaw)
	if err != nil {
		return nil, "", errors.New("电商图片工厂项目不存在")
	}
	outputs := map[string]interface{}{}
	_ = json.Unmarshal(outputsRaw, &outputs)
	state := ecomFactoryStateFromOutputs(outputs)
	if state == nil {
		return nil, "", errors.New("项目还没有生成结果")
	}
	items := ecomFactoryStateItems(state)
	type zipEntry struct{ name, key string }
	entries := []zipEntry{}
	productName := strings.TrimSpace(stringValue(state["productName"]))
	index := 0
	for _, item := range items {
		if item["status"] != "success" {
			continue
		}
		url := strings.TrimSpace(stringValue(item["imageUrl"]))
		if url == "" {
			continue
		}
		index++
		label := sanitizeEcomFileName(stringValue(item["label"]))
		if key := s.storage.ObjectKeyFromURL(url); key != "" {
			entries = append(entries, zipEntry{fmt.Sprintf("%s-%02d.jpg", label, index), key})
		}
	}
	if len(entries) == 0 {
		return nil, "", errors.New("没有可下载的生成结果")
	}
	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	for _, entry := range entries {
		data, readErr := s.storage.ReadAll(ctx, entry.key, 64<<20)
		if readErr != nil {
			continue
		}
		w, zipErr := zw.Create(entry.name)
		if zipErr != nil {
			return nil, "", zipErr
		}
		if _, err := w.Write(data); err != nil {
			return nil, "", err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, "", err
	}
	zipName := sanitizeEcomFileName(firstAgentString(productName, "电商图片工厂")) + "-电商套图.zip"
	return buf, zipName, nil
}

func sanitizeEcomFileName(name string) string {
	name = ecomFactoryFileSanitizer.ReplaceAllString(strings.TrimSpace(name), "_")
	name = regexp.MustCompile(`\s+`).ReplaceAllString(name, "-")
	if len(name) > 48 {
		name = name[:48]
	}
	if name == "" {
		name = "image"
	}
	return name
}

// ---------- 共享小工具 ----------

func agentAnySlice(v interface{}) []interface{} {
	out, _ := v.([]interface{})
	return out
}

func agentStringSliceValue(v interface{}) []string {
	out := []string{}
	if raw, ok := v.([]interface{}); ok {
		for _, item := range raw {
			if s := stringValue(item); s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

func toAgentAnySlice(items []string) []interface{} {
	out := make([]interface{}, 0, len(items))
	for _, item := range items {
		out = append(out, item)
	}
	return out
}

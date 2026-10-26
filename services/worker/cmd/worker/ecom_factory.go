package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// 电商图片工厂：上传商品图/参考图 + 表单 → 套装/模块计划 → 并发生图。
// 提示词模板与模块目录来自 ecom_factory_catalog.json（同源下发前端）。

type ecomItem struct {
	ID       string `json:"id"`
	ModuleID string `json:"moduleId"`
	Label    string `json:"label"`
	Ratio    string `json:"ratio"`
	Prompt   string `json:"prompt"`
	Status   string `json:"status"` // queued generating success failed stopped
	ImageURL string `json:"imageUrl,omitempty"`
	TaskNo   string `json:"taskNo,omitempty"`
	Error    string `json:"error,omitempty"`
	Progress string `json:"progress"`
	Attempts int    `json:"attempts"`
}

type ecomFactoryState struct {
	Mode        string     `json:"mode"` // suite batch quick
	Suite       string     `json:"suite"`
	Platform    string     `json:"platform"`
	Resolution  string     `json:"resolution"`
	ProductName string     `json:"productName"`
	BatchStatus string     `json:"batchStatus"` // running completed failed stopped
	Items       []ecomItem `json:"items"`
	Total       int        `json:"total"`
	Success     int        `json:"success"`
	Failed      int        `json:"failed"`
}

type ecomItemScope struct {
	Action      string   `json:"action"`      // regenerate refine continue
	ItemIDs     []string `json:"itemIds"`     // regenerate/refine 指定项
	Prompt      string   `json:"prompt"`      // regenerate 提示词覆盖
	Reference   string   `json:"reference"`   // refine 以指定图为基准（默认该项已生成图）
	Instruction string   `json:"instruction"` // refine 调整要求
}

func processEcomFactoryWorkflow(ctx context.Context, pool *pgxpool.Pool, baseURL, token string, p WorkflowTaskPayload, publicID string, reserved float64, inputs, runtime map[string]interface{}) error {
	catalog, err := loadEcomCatalog()
	if err != nil {
		return failWorkflow(ctx, pool, p, publicID, reserved, err.Error())
	}
	outputs := loadWorkflowOutputs(ctx, pool, p.ProjectID)
	budget := floatAny(inputs["_execution_budget"])

	scope, hasScope := ecomParseItemScope(inputs)
	state, planErr := ecomPrepareState(catalog, outputs, inputs, scope, hasScope)
	if planErr != "" {
		return finishEcomFactory(ctx, pool, p, publicID, reserved, outputs, state, planErr)
	}
	modelCode := firstNonEmpty(stringAny(runtime["generation_model_code"]), "gpt-image-2")
	resolution := firstNonEmpty(state.Resolution, "1K")
	references := ecomReferenceURLs(inputs, scope, hasScope)
	maxRefs := catalog.Limits.DefaultMaxRefs
	if state.Mode == "quick" {
		maxRefs = catalog.Limits.QuickMaxReferences
	}
	if len(references) > maxRefs {
		references = references[:maxRefs]
	}
	concurrency := catalog.Limits.MaxConcurrency
	if c := intAny(runtime["max_concurrency"]); c > 0 && c < concurrency {
		concurrency = c
	}

	// 并发池：每项独立生图，单项失败不阻塞其他项；取消后未开始项置 stopped。
	var mu sync.Mutex
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	saveState := func(mutate func(*ecomFactoryState)) {
		mu.Lock()
		defer mu.Unlock()
		mutate(state)
		ecomPersistState(ctx, pool, p.ProjectID, outputs, state)
	}
	for idx := range state.Items {
		item := &state.Items[idx]
		if item.Status != "queued" {
			continue
		}
		wg.Add(1)
		go func(item *ecomItem) {
			defer wg.Done()
			if !ecomProjectRunning(ctx, pool, p.ProjectID) {
				saveState(func(s *ecomFactoryState) { ecomMarkItem(s, item, "stopped", "", "", "已停止") })
				return
			}
			sem <- struct{}{}
			defer func() { <-sem }()
			saveState(func(s *ecomFactoryState) { ecomMarkItem(s, item, "generating", "", "", "提交生成") })
			imageURL, taskNo, genErr := ecomFactoryGenerate(ctx, pool, baseURL, token, p, publicID, modelCode, item, references, resolution, budget)
			saveState(func(s *ecomFactoryState) {
				if genErr != nil {
					// 先写真实上游错误再标记状态，避免被「生成失败」兜底文案覆盖。
					item.Error = genErr.Error()
					ecomMarkItem(s, item, "failed", "", taskNo, "失败")
				} else {
					ecomMarkItem(s, item, "success", imageURL, taskNo, "完成")
				}
			})
		}(item)
	}
	wg.Wait()
	return finishEcomFactory(ctx, pool, p, publicID, reserved, outputs, state, "")
}

// ecomPrepareState 新建计划或从既有 outputs 恢复并按 scope 过滤重跑项。
func ecomPrepareState(catalog *ecomCatalog, outputs map[string]interface{}, inputs map[string]interface{}, scope ecomItemScope, hasScope bool) (*ecomFactoryState, string) {
	if hasScope {
		saved := ecomLoadState(outputs)
		if saved == nil || len(saved.Items) == 0 {
			return nil, "未找到可重跑的生成项"
		}
		target := map[string]bool{}
		switch scope.Action {
		case "continue":
			for i := range saved.Items {
				if saved.Items[i].Status != "success" {
					target[saved.Items[i].ID] = true
				}
			}
		case "regenerate", "refine":
			for _, id := range scope.ItemIDs {
				target[id] = true
			}
		}
		if len(target) == 0 {
			return nil, "没有需要重跑的生成项"
		}
		cat := catalog
		for i := range saved.Items {
			item := &saved.Items[i]
			if !target[item.ID] {
				continue
			}
			item.Status, item.Error, item.ImageURL, item.Progress = "queued", "", "", "排队中"
			item.Attempts++
			switch scope.Action {
			case "regenerate":
				if strings.TrimSpace(scope.Prompt) != "" {
					item.Prompt = strings.TrimSpace(scope.Prompt)
				}
			case "refine":
				instruction := strings.TrimSpace(scope.Instruction)
				if instruction == "" {
					instruction = cat.Templates.Refine.DefaultInstruction
				}
				item.Prompt = cat.Templates.Refine.Prefix + instruction
			}
		}
		saved.Mode = firstNonEmpty(saved.Mode, "suite")
		saved.BatchStatus = "running"
		ecomRecalcState(saved)
		return saved, ""
	}
	return ecomBuildState(catalog, inputs)
}

func ecomBuildState(catalog *ecomCatalog, inputs map[string]interface{}) (*ecomFactoryState, string) {
	form := ecomFormFromInputs(inputs)
	mode := firstNonEmpty(stringAny(inputs["mode"]), "suite")
	state := &ecomFactoryState{
		Mode:        mode,
		Suite:       form.Suite,
		Platform:    form.Platform,
		Resolution:  form.Resolution,
		ProductName: strings.TrimSpace(form.ProductName),
		BatchStatus: "running",
	}
	items := []ecomItem{}
	switch mode {
	case "batch":
		count := intAny(inputs["batchCount"])
		if count < 1 {
			count = 5
		}
		if count > catalog.Limits.MaxPlanItems {
			count = catalog.Limits.MaxPlanItems
		}
		module := catalog.Modules[len(catalog.Modules)-1]
		for i := 1; i <= count; i++ {
			items = append(items, ecomItem{
				ID: fmt.Sprintf("batch-%03d", i), ModuleID: module.ID,
				Label: fmt.Sprintf("参考批量 %03d", i), Ratio: firstNonEmpty(form.Ratio, module.DefaultRatio),
				Prompt: renderEcomBatchPrompt(catalog, form, i), Status: "queued", Progress: "排队中",
			})
		}
	case "quick":
		prompt := strings.TrimSpace(stringAny(inputs["quickPrompt"]))
		if prompt == "" {
			return nil, "请输入或反推出提示词"
		}
		moduleID := firstNonEmpty(peekFirstModuleID(inputs), "main-square")
		module, _ := catalog.ModuleByID(moduleID)
		items = append(items, ecomItem{
			ID: "quick-1", ModuleID: module.ID, Label: "快速生图",
			Ratio: firstNonEmpty(form.Ratio, "1:1"), Prompt: prompt, Status: "queued", Progress: "排队中",
		})
	default:
		modules := ecomResolveModules(catalog, inputs)
		if len(modules) == 0 {
			return nil, "请先选择生成模块"
		}
		count := intAny(inputs["countPerModule"])
		if count < 1 {
			count = 1
		}
		if count > catalog.Limits.MaxCountPerModule {
			count = catalog.Limits.MaxCountPerModule
		}
		for _, module := range modules {
			for v := 1; v <= count; v++ {
				label := module.Label
				if count > 1 {
					label = fmt.Sprintf("%s %d", module.Label, v)
				}
				items = append(items, ecomItem{
					ID: fmt.Sprintf("%s-%d", module.ID, v), ModuleID: module.ID, Label: label,
					Ratio:  firstNonEmpty(form.Ratio, module.DefaultRatio),
					Prompt: renderEcomNormalPrompt(catalog, form, module, v),
					Status: "queued", Progress: "排队中",
				})
			}
		}
		if len(items) > catalog.Limits.MaxPlanItems {
			items = items[:catalog.Limits.MaxPlanItems]
		}
	}
	state.Items = items
	state.Total = len(items)
	ecomRecalcState(state)
	return state, ""
}

func ecomResolveModules(catalog *ecomCatalog, inputs map[string]interface{}) []ecomModule {
	ids := []string{}
	if raw, ok := inputs["selectedModuleIds"].([]interface{}); ok {
		for _, v := range raw {
			if id := stringAny(v); id != "" {
				ids = append(ids, id)
			}
		}
	}
	if len(ids) == 0 {
		if suite, ok := catalog.SuiteByID(firstNonEmpty(stringAny(inputs["selectedSuite"]), "launch")); ok {
			ids = suite.ModuleIds
		}
	}
	modules := []ecomModule{}
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		if module, ok := catalog.ModuleByID(id); ok {
			modules = append(modules, module)
			seen[id] = true
		}
	}
	return modules
}

func peekFirstModuleID(inputs map[string]interface{}) string {
	if raw, ok := inputs["selectedModuleIds"].([]interface{}); ok && len(raw) > 0 {
		return stringAny(raw[0])
	}
	return ""
}

// ---------- 表单 ----------

type ecomForm struct {
	ProductName, SellingPoints, Audience, Scenario              string
	Platform, Language, VisualStyle, CustomStyle                string
	Suite, Ratio, Resolution, ExtraPrompt                       string
	CopyEnabled, HasAssets                                      bool
	BatchPrompt, BatchStrategy, BatchSimilarity, BatchVariation string
	BatchKeepKeys, BatchChangeKeys                              []string
}

func ecomFormFromInputs(inputs map[string]interface{}) ecomForm {
	str := func(k string) string { return strings.TrimSpace(stringAny(inputs[k])) }
	boolv := func(k string) bool { return boolAny(inputs[k]) }
	f := ecomForm{
		ProductName: str("productName"), SellingPoints: str("sellingPoints"),
		Audience: str("audience"), Scenario: str("scenario"),
		Platform: str("platform"), Language: str("language"),
		VisualStyle: str("visualStyle"), CustomStyle: str("customStyle"),
		Suite: str("selectedSuite"), Ratio: str("ratio"), Resolution: str("resolution"),
		ExtraPrompt: str("extraPrompt"), CopyEnabled: boolv("copyEnabled"),
		BatchPrompt: str("batchPrompt"), BatchStrategy: str("batchStrategy"),
		BatchSimilarity: str("batchSimilarity"), BatchVariation: str("batchVariation"),
	}
	f.HasAssets = len(ecomReferenceURLs(inputs, ecomItemScope{}, false)) > 0
	f.BatchKeepKeys = ecomStringSlice(inputs, "batchKeepKeys")
	f.BatchChangeKeys = ecomStringSlice(inputs, "batchChangeKeys")
	return f
}

func ecomStringSlice(inputs map[string]interface{}, key string) []string {
	out := []string{}
	if raw, ok := inputs[key].([]interface{}); ok {
		for _, v := range raw {
			if s := stringAny(v); s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

// ecomReferenceURLs 汇总商品图 + 风格参考 URL；refine scope 时以基准图优先。
func ecomReferenceURLs(inputs map[string]interface{}, scope ecomItemScope, hasScope bool) []string {
	if hasScope && scope.Action == "refine" {
		if ref := strings.TrimSpace(scope.Reference); ref != "" {
			return []string{ref}
		}
		return []string{}
	}
	urls := []string{}
	if raw, ok := inputs["assets"].([]interface{}); ok {
		for _, v := range raw {
			m, _ := v.(map[string]interface{})
			if m == nil {
				continue
			}
			if url := strings.TrimSpace(stringAny(m["url"])); url != "" {
				urls = append(urls, url)
			}
		}
	}
	return urls
}

// ---------- 提示词渲染 ----------

// renderEcomTemplate 数据驱动行模板：when 条件求值 + {{path||fallback}} 替换。
func renderEcomTemplate(tpl ecomLineTemplate, vars map[string]interface{}) string {
	lines := []string{}
	for _, line := range tpl.Lines {
		if line.When != "" && !ecomWhen(line.When, vars) {
			continue
		}
		text := ecomInterpolate(line.Text, vars)
		if strings.TrimSpace(text) != "" {
			lines = append(lines, text)
		}
	}
	return strings.Join(lines, "\n")
}

func ecomWhen(name string, vars map[string]interface{}) bool {
	v, _ := vars[name].(bool)
	return v
}

func ecomInterpolate(text string, vars map[string]interface{}) string {
	var b strings.Builder
	for i := 0; i < len(text); {
		start := strings.Index(text[i:], "{{")
		if start < 0 {
			b.WriteString(text[i:])
			break
		}
		b.WriteString(text[i : i+start])
		rest := text[i+start:]
		end := strings.Index(rest, "}}")
		if end < 0 {
			b.WriteString(rest)
			break
		}
		expr := strings.TrimSpace(rest[2:end])
		fallback := ""
		if idx := strings.Index(expr, "||"); idx >= 0 {
			fallback = strings.TrimSpace(expr[idx+2:])
			expr = strings.TrimSpace(expr[:idx])
		}
		value := ecomLookupPath(vars, expr)
		if value == "" {
			value = fallback
		}
		b.WriteString(value)
		i += start + end + 2
	}
	return b.String()
}

func ecomLookupPath(vars map[string]interface{}, path string) string {
	parts := strings.Split(path, ".")
	var current interface{} = vars
	for _, part := range parts {
		m, ok := current.(map[string]interface{})
		if !ok {
			return ""
		}
		current = m[part]
	}
	switch v := current.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case int:
		return fmt.Sprintf("%d", v)
	default:
		return ""
	}
}

func ecomPromptVars(form ecomForm) map[string]interface{} {
	visualStyle := form.VisualStyle
	if visualStyle == "自定义" {
		visualStyle = form.CustomStyle
	}
	copyAllowed := form.Language != "无文字" && form.CopyEnabled
	return map[string]interface{}{
		"productName": form.ProductName, "sellingPoints": form.SellingPoints,
		"audience": form.Audience, "scenario": form.Scenario,
		"platform": form.Platform, "language": form.Language,
		"visualStyle": visualStyle, "extraPrompt": form.ExtraPrompt,
		"copyAllowed": copyAllowed, "copyForbidden": !copyAllowed,
		"hasAssets": form.HasAssets, "noAssets": !form.HasAssets,
		"hasExtra":         strings.TrimSpace(form.ExtraPrompt) != "",
		"hasProductName":   form.ProductName != "",
		"hasSellingPoints": form.SellingPoints != "",
		"hasAudience":      form.Audience != "",
		"hasScenario":      form.Scenario != "",
	}
}

func renderEcomNormalPrompt(catalog *ecomCatalog, form ecomForm, module ecomModule, variant int) string {
	vars := ecomPromptVars(form)
	vars["module"] = map[string]interface{}{
		"channel": module.Channel, "label": module.Label,
		"prompt": module.Prompt, "goal": module.Goal,
	}
	vars["variantIndex"] = variant
	return renderEcomTemplate(catalog.Templates.Normal, vars)
}

func renderEcomBatchPrompt(catalog *ecomCatalog, form ecomForm, variant int) string {
	vars := ecomPromptVars(form)
	vars["batch"] = map[string]interface{}{
		"strategy": form.BatchStrategy, "similarity": form.BatchSimilarity,
		"variation": form.BatchVariation, "prompt": form.BatchPrompt,
		"keepLabels":   ecomKeyLabels(catalog, form.BatchKeepKeys, true),
		"changeLabels": ecomKeyLabels(catalog, form.BatchChangeKeys, false),
	}
	vars["variantIndex"] = variant
	return renderEcomTemplate(catalog.Templates.Batch, vars)
}

func ecomKeyLabels(catalog *ecomCatalog, keys []string, keep bool) string {
	labels := []string{}
	for _, key := range keys {
		list := catalog.Batch.ChangeKeys
		if keep {
			list = catalog.Batch.KeepKeys
		}
		for _, k := range list {
			if k.Key == key {
				labels = append(labels, k.Label)
			}
		}
	}
	return strings.Join(labels, "、")
}

// ---------- 生图 ----------

func ecomFactoryGenerate(ctx context.Context, pool *pgxpool.Pool, baseURL, token string, p WorkflowTaskPayload, publicID, modelCode string, item *ecomItem, references []string, resolution string, budget float64) (string, string, error) {
	catalog, _ := loadEcomCatalog()
	var modelID int64
	var defaultsRaw, runtimeRaw []byte
	if err := pool.QueryRow(ctx, `SELECT id,default_params,runtime_rule FROM models WHERE code=$1 AND is_enabled=true AND request_mode='images'`, modelCode).
		Scan(&modelID, &defaultsRaw, &runtimeRaw); err != nil {
		return "", "", fmt.Errorf("未配置可用图片生成模型")
	}
	defaults, runtime := map[string]interface{}{}, map[string]interface{}{}
	_ = json.Unmarshal(defaultsRaw, &defaults)
	_ = json.Unmarshal(runtimeRaw, &runtime)
	taskInput := map[string]interface{}{"prompt": item.Prompt}
	applyAgentModelDefaults(taskInput, defaults, runtime, "image")
	taskInput["count"], taskInput["n"] = 1, 1
	taskInput["size"] = catalog.resolveEcomSize(item.Ratio, resolution)
	taskInput["_workflow_project"] = publicID
	taskInput["_skip_billing"] = true
	if len(references) > 0 {
		taskInput["reference_images"] = references
	}
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%d", publicID, item.ID, item.Attempts)))
	taskNo := fmt.Sprintf("ecomf_%x", hash[:12])
	var state string
	err := pool.QueryRow(ctx, `SELECT status FROM tasks WHERE task_no=$1`, taskNo).Scan(&state)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return "", taskNo, err
		}
		forecast := estimateModelCostByIDWorker(ctx, pool, modelID, taskInput, 0, 0, 0, 0)
		if budget > 0 {
			spent, err := productSpent(ctx, pool, p.ProjectID)
			if err == nil && !productBudgetAllows(spent, forecast, budget) {
				return "", taskNo, fmt.Errorf("剩余预算不足以生成下一张图片")
			}
		}
		if _, err := pool.Exec(ctx, `INSERT INTO tasks(task_no,user_id,model_id,type,status,input,estimated_cost,workflow_project_id) VALUES($1,$2,$3,'image','pending',$4,$5,$6)`, taskNo, p.UserID, modelID, mustJSON(taskInput), forecast, p.ProjectID); err != nil {
			return "", taskNo, err
		}
		state = "pending"
	} else if err != nil {
		return "", taskNo, err
	}
	if state == "running" {
		return "", taskNo, fmt.Errorf("上次生图状态尚未确定，已停止重复提交，请稍后重试")
	}
	if state == "pending" {
		var projectState string
		if err := pool.QueryRow(ctx, `SELECT status FROM workflow_projects WHERE id=$1`, p.ProjectID).Scan(&projectState); err != nil {
			return "", taskNo, err
		}
		if projectState != "running" {
			return "", taskNo, fmt.Errorf("工作流已停止，未提交生成")
		}
		if err := processImageTask(ctx, pool, baseURL, token, ImageTaskPayload{
			TaskNo: taskNo, UserID: p.UserID, ModelID: modelID, ModelCode: modelCode,
			WorkflowProjectID: p.ProjectID, Input: taskInput,
		}); err != nil {
			return "", taskNo, err
		}
	}
	result := loadAgentMediaTask(ctx, pool, taskNo)
	nodeKey := taskNo
	var exists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM workflow_node_runs WHERE project_id=$1 AND node_id=$2)`, p.ProjectID, nodeKey).Scan(&exists); err != nil {
		return "", taskNo, err
	}
	if !exists {
		id := insertWorkflowNodeRun(ctx, pool, p.ProjectID, nodeKey, "电商图片生成", "image", map[string]interface{}{"task_no": taskNo, "module": item.ModuleID}, item.Attempts)
		if id <= 0 {
			return "", taskNo, fmt.Errorf("无法记录图片成本")
		}
		updateNodeRunSuccess(ctx, pool, id, result, floatAny(result["actual_cost"]), 0)
	}
	if stringAny(result["status"]) != "succeeded" {
		return "", taskNo, fmt.Errorf("图片生成失败：%s", firstNonEmpty(stringAny(result["error_message"]), "上游返回失败"))
	}
	return ecomImageURLFromTask(result), taskNo, nil
}

func ecomImageURLFromTask(result map[string]interface{}) string {
	if url := stringAny(result["image_url"]); url != "" {
		return url
	}
	output, _ := result["output"].(map[string]interface{})
	if output == nil {
		return ""
	}
	for _, key := range []string{"image_url", "url", "b64_json"} {
		if v := stringAny(output[key]); v != "" && key != "b64_json" {
			return v
		}
	}
	if raw, ok := output["images"].([]interface{}); ok && len(raw) > 0 {
		if m, _ := raw[0].(map[string]interface{}); m != nil {
			if url := stringAny(m["url"]); url != "" {
				return url
			}
		}
	}
	return ""
}

// ---------- 状态与结算 ----------

func ecomMarkItem(state *ecomFactoryState, item *ecomItem, status, imageURL, taskNo, progress string) {
	item.Status = status
	if imageURL != "" {
		item.ImageURL = imageURL
	}
	if taskNo != "" {
		item.TaskNo = taskNo
	}
	if progress != "" {
		item.Progress = progress
	}
	if status == "failed" && item.Error == "" {
		item.Error = "生成失败"
	}
}

func ecomRecalcState(state *ecomFactoryState) {
	success, failed, running := 0, 0, 0
	for _, item := range state.Items {
		switch item.Status {
		case "success":
			success++
		case "failed":
			failed++
		case "queued", "generating":
			running++
		}
	}
	state.Total, state.Success, state.Failed = len(state.Items), success, failed
	switch {
	case running > 0:
		state.BatchStatus = "running"
	case failed > 0:
		state.BatchStatus = "failed"
	default:
		state.BatchStatus = "completed"
	}
}

func ecomPersistState(ctx context.Context, pool *pgxpool.Pool, projectID int64, outputs map[string]interface{}, state *ecomFactoryState) {
	ecomRecalcState(state)
	outputs["ecom_factory"] = state
	outputs["current_step"] = "ecom_factory"
	if log := ecomProcessLog(outputs, state); log != nil {
		outputs["ecom_factory_process_log"] = log
	}
	saveWorkflowOutputs(ctx, pool, projectID, outputs)
}

func ecomProcessLog(outputs map[string]interface{}, state *ecomFactoryState) []map[string]interface{} {
	records := []map[string]interface{}{}
	_ = json.Unmarshal(mustJSON(outputs["ecom_factory_process_log"]), &records)
	now := time.Now().UTC().Format(time.RFC3339)
	entry := map[string]interface{}{
		"id": "ecom_factory_summary", "stage": "ecom_factory", "status": state.BatchStatus,
		"message":    fmt.Sprintf("%d 成功 / %d 失败 / %d 进行中", state.Success, state.Failed, state.Total-state.Success-state.Failed),
		"updated_at": now,
	}
	replaced := false
	for i, record := range records {
		if stringAny(record["id"]) == "ecom_factory_summary" {
			if created := stringAny(record["created_at"]); created != "" {
				entry["created_at"] = created
			}
			records[i], replaced = entry, true
			break
		}
	}
	if !replaced {
		entry["created_at"] = now
		records = append(records, entry)
	}
	return records
}

func ecomLoadState(outputs map[string]interface{}) *ecomFactoryState {
	raw, ok := outputs["ecom_factory"]
	if !ok {
		return nil
	}
	state := &ecomFactoryState{}
	if err := json.Unmarshal(mustJSON(raw), state); err != nil || len(state.Items) == 0 {
		return nil
	}
	return state
}

func ecomParseItemScope(inputs map[string]interface{}) (ecomItemScope, bool) {
	raw, ok := inputs["itemScope"].(map[string]interface{})
	if !ok {
		return ecomItemScope{}, false
	}
	scope := ecomItemScope{
		Action: stringAny(raw["action"]), Prompt: stringAny(raw["prompt"]),
		Reference: stringAny(raw["reference"]), Instruction: stringAny(raw["instruction"]),
	}
	if ids, ok := raw["itemIds"].([]interface{}); ok {
		for _, v := range ids {
			if s := stringAny(v); s != "" {
				scope.ItemIDs = append(scope.ItemIDs, s)
			}
		}
	}
	if scope.Action == "" {
		return ecomItemScope{}, false
	}
	return scope, true
}

func ecomProjectRunning(ctx context.Context, pool *pgxpool.Pool, projectID int64) bool {
	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM workflow_projects WHERE id=$1`, projectID).Scan(&status); err != nil {
		return false
	}
	return status == "running"
}

func finishEcomFactory(ctx context.Context, pool *pgxpool.Pool, p WorkflowTaskPayload, publicID string, reserved float64, outputs map[string]interface{}, state *ecomFactoryState, message string) error {
	if state != nil {
		ecomRecalcState(state)
		if !ecomProjectRunning(ctx, pool, p.ProjectID) {
			for i := range state.Items {
				if state.Items[i].Status == "queued" || state.Items[i].Status == "generating" {
					state.Items[i].Status, state.Items[i].Progress = "stopped", "已停止"
				}
			}
			ecomRecalcState(state)
		}
		outputs["ecom_factory"] = state
	}
	chargeResult, err := workflowSettlementCharge(ctx, pool, p.ProjectID)
	if err != nil {
		return failWorkflowWithoutCost(ctx, pool, p, publicID, reserved, err.Error())
	}
	actual := chargeResult.UserCharge
	outputs["incurred_cost"], outputs["cost"] = chargeResult.ProviderCost, actual
	status := "succeeded"
	quality := "passed"
	if message != "" {
		status, quality = "failed", "needs_review"
	} else if state != nil && state.Failed > 0 {
		quality = "needs_review"
	}
	outputs["quality_status"] = quality
	return chargeWorkflowWithSettlement(ctx, pool, p.UserID, publicID, p.ProjectID, reserved, incrementalWorkflowCharge(ctx, pool, p.ProjectID, actual), chargeResult, "电商图片工厂", func(tx pgx.Tx) error {
		if state != nil {
			for index, item := range state.Items {
				if item.ImageURL == "" {
					continue
				}
				// works.public_id 为 varchar(32)：用项目内序号而非完整 item ID 保证长度。
				workID := fmt.Sprintf("work_%s_%d", publicID, index)
				if _, err := tx.Exec(ctx, `INSERT INTO works(public_id,user_id,type,title,thumbnail_url,metadata) VALUES($1,$2,'image',$3,$4,$5) ON CONFLICT(public_id) DO NOTHING`,
					workID, p.UserID, firstNonEmpty(state.ProductName, item.Label), item.ImageURL,
					mustJSON(map[string]interface{}{"image_url": item.ImageURL, "module_id": item.ModuleID, "item_id": item.ID, "workflow_project": publicID})); err != nil {
					return err
				}
			}
		}
		tag, err := tx.Exec(ctx, `UPDATE workflow_projects SET status=CASE WHEN status='canceling' THEN 'canceled' ELSE $1 END, outputs=$2,actual_cost=$3,error_message=NULLIF($4,''),finished_at=now(),updated_at=now() WHERE id=$5 AND status IN ('running','canceling')`,
			status, mustJSON(outputs), actual, message, p.ProjectID)
		if err == nil && tag.RowsAffected() != 1 {
			return fmt.Errorf("工作流状态已改变")
		}
		return err
	})
}

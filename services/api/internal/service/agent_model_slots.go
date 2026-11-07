package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/starai/modelcontract"
)

type AgentModelSlotDTO struct {
	modelcontract.Slot
	ModelName           string                `json:"model_name"`
	Compatible          bool                  `json:"compatible"`
	Issue               string                `json:"issue,omitempty"`
	Candidates          []AgentModelCandidate `json:"candidates"`
	SourceLabel         string                `json:"source_label"`
	SharedWith          []string              `json:"shared_with"`
	ConfiguredModelCode string                `json:"configured_model_code"`
	Inherited           bool                  `json:"is_inherited"`
}

type AgentModelCandidate struct {
	Code string `json:"code"`
	Name string `json:"display_name"`
}
type AgentModelBindingsDTO struct {
	Code            string              `json:"code"`
	Name            string              `json:"name"`
	Enabled         bool                `json:"is_enabled"`
	SystemWorkspace bool                `json:"system_workspace"`
	Slots           []AgentModelSlotDTO `json:"slots"`
	Note            string              `json:"note,omitempty"`
}

func contractDefinition(w WorkflowDTO) modelcontract.Definition {
	d := modelcontract.Definition{Code: w.Code, Category: w.Category, Runtime: w.RuntimeConfig}
	for _, n := range w.Nodes {
		d.Nodes = append(d.Nodes, modelcontract.Node{ID: n.ID, Type: n.Type, Name: n.Name, ModelCode: n.ModelCode})
	}
	return d
}

func (s *AgentService) validateModelBindings(ctx context.Context, w WorkflowDTO, inputs map[string]interface{}) error {
	models, err := modelcontract.LoadModels(ctx, s.db)
	if err != nil {
		return err
	}
	d := contractDefinition(w)
	if inputs != nil {
		d = modelcontract.Effective(d, inputs)
	}
	return modelcontract.ValidateDefinition(d, models)
}

func (s *AgentService) validateSuppliedAgentModels(ctx context.Context, in AgentUpsertInput) error {
	runtime := modelcontract.Merge(in.RuntimeConfig, nil)
	if _, ok := runtime["agent_mode"]; !ok && in.AgentMode != "" {
		runtime["agent_mode"] = in.AgentMode
	}
	if _, ok := runtime["generation_type"]; !ok && in.GenerationType != "" {
		runtime["generation_type"] = in.GenerationType
	}
	for key, code := range map[string]string{"analysis_model_code": in.AnalysisModelCode, "generation_model_code": in.GenerationModelCode} {
		if code != "" {
			runtime[key] = code
		}
	}
	d := contractDefinition(WorkflowDTO{Code: in.Code, Category: in.Category, RuntimeConfig: runtime, Nodes: in.Nodes})
	// Only submitted slots are checked here; normalization fills required slots
	// from legacy top-level fields before the complete definition is checked.
	models, err := modelcontract.LoadModels(ctx, s.db)
	if err != nil {
		return err
	}
	for _, slot := range modelcontract.Slots(d) {
		if slot.ModelCode == "" {
			continue
		}
		m, ok := modelcontract.Find(models, slot.ModelCode)
		if !ok {
			return fmt.Errorf("%s：模型不存在：%s", slot.Label, slot.ModelCode)
		}
		if err := modelcontract.Validate(slot, m); err != nil {
			return err
		}
	}
	return nil
}

func (s *AgentService) validateModelOverrides(ctx context.Context, w WorkflowDTO, inputs map[string]interface{}) error {
	provided := false
	for key := range inputs {
		if strings.HasSuffix(key, "model_code") || strings.HasSuffix(key, "model_codes") {
			provided = true
			break
		}
	}
	if !provided {
		return nil
	}
	models, err := modelcontract.LoadModels(ctx, s.db)
	if err != nil {
		return err
	}
	d := contractDefinition(w)
	for key, raw := range modelcontract.Overrides(d, inputs) {
		slot, known := modelcontract.FieldSlot(d, key)
		if !known {
			continue
		}
		codes := []string{modelcontract.String(raw)}
		if key == "dialogue_model_codes" {
			codes = modelcontract.Strings(raw)
		}
		for _, code := range codes {
			if code == "" {
				continue
			}
			m, ok := modelcontract.Find(models, code)
			if !ok {
				return fmt.Errorf("%s：模型不存在：%s", slot.Label, code)
			}
			if err := modelcontract.Validate(slot, m); err != nil {
				return err
			}
		}
	}
	return nil
}

// aliasGroup joins only fields consumed as the same logical slot. Custom nodes
// remain independent unless a built-in workflow explicitly mirrors them.
func aliasGroup(d modelcontract.Definition, slot modelcontract.Slot) string {
	id := strings.TrimPrefix(slot.ID, "runtime.")
	mode := modelcontract.Mode(d)
	if strings.HasPrefix(slot.ID, "node.") {
		n := modelcontract.Node{ID: strings.TrimPrefix(slot.ID, "node.")}
		if key := modelcontract.NodeRuntimeField(d, n); key != "" {
			return key
		}
		return slot.ID
	}
	if mode == "comic_drama" {
		if id == "generation_model_code" {
			return "video_model_code"
		}
		if id == "dialogue_model_codes.0" {
			return "analysis_model_code"
		}
	}
	if mode == "creative_chat" && id == "generation_model_code" {
		return "image_model_code"
	}
	return id
}

func bindingSlots(d modelcontract.Definition) []modelcontract.Slot {
	all := modelcontract.Slots(d)
	out := []modelcontract.Slot{}
	indexes := map[string]int{}
	for _, slot := range all {
		group := aliasGroup(d, slot)
		if i, exists := indexes[group]; exists {
			out[i].Sources = append(out[i].Sources, slot.Sources...)
			for _, r := range slot.Required {
				if !stringInModelSlice(out[i].Required, r) {
					out[i].Required = append(out[i].Required, r)
				}
			}
			if slot.ID == "runtime."+group || out[i].ModelCode == "" {
				out[i].ModelCode = slot.ModelCode
			}
		} else {
			if !strings.HasPrefix(group, "node.") && strings.HasPrefix(slot.ID, "node.") {
				// Missing runtime fields still need to be created when saving a
				// descriptive node, or the specialized consumer would ignore it.
				slot.Sources = append([]string{"runtime_config." + group}, slot.Sources...)
			}
			slot.ID = group
			indexes[group] = len(out)
			out = append(out, slot)
		}
	}
	return out
}

func stringInModelSlice(items []string, v string) bool {
	for _, s := range items {
		if s == v {
			return true
		}
	}
	return false
}

func buildAgentModelBindings(w WorkflowDTO, models []modelcontract.Model) AgentModelBindingsDTO {
	d := contractDefinition(w)
	item := AgentModelBindingsDTO{Code: w.Code, Name: w.Name, Enabled: w.IsEnabled, SystemWorkspace: w.RuntimeConfig["system_workspace"] == true, Slots: []AgentModelSlotDTO{}}
	if modelcontract.Mode(d) == "creative_chat" || modelcontract.Mode(d) == "infinite_canvas" {
		item.SystemWorkspace = true
	}
	for _, slot := range bindingSlots(d) {
		row := AgentModelSlotDTO{Slot: slot, ConfiguredModelCode: slot.ModelCode, Candidates: []AgentModelCandidate{}, SharedWith: []string{}, SourceLabel: "智能体配置"}
		if strings.HasPrefix(slot.ID, "node.") {
			row.SourceLabel = "工作流节点"
		}
		if slot.ModelCode == "" {
			row.Issue = "尚未固定模型"
		} else if m, ok := modelcontract.Find(models, slot.ModelCode); ok {
			row.ModelName = m.Name
			if err := modelcontract.Validate(slot, m); err != nil {
				row.Issue = err.Error()
			} else {
				row.Compatible = true
			}
		} else {
			row.Issue = "模型不存在"
		}
		// Detect stale mirrors, rather than concealing which model is executed.
		for _, raw := range modelcontract.Slots(d) {
			if aliasGroup(d, raw) != slot.ID || raw.ModelCode == "" {
				continue
			}
			if raw.ModelCode != slot.ModelCode {
				row.Issue = "同一槽位的配置来源不一致，保存后将同步更新"
				row.Compatible = false
			}
		}
		for _, m := range models {
			if modelcontract.Validate(slot, m) == nil {
				row.Candidates = append(row.Candidates, AgentModelCandidate{Code: m.Code, Name: m.Name})
			}
		}
		item.Slots = append(item.Slots, row)
	}
	if len(item.Slots) == 0 {
		item.Note = "该智能体没有可配置的模型槽位；使用专用服务或由任务节点自行选择模型"
	}
	if modelcontract.Mode(d) == "infinite_canvas" {
		item.Note = "未固定的槽位由画布按任务选择模型；设置后作为新任务的默认值，已有节点保留自己的选择"
	}
	if modelcontract.Mode(d) == "realtime_avatar" {
		item.Note = "实时数字人使用 Vidu 实时服务；配置来源为系统设置中的 vidu_* 参数，未使用模型库槽位"
	}
	if modelcontract.Mode(d) == "product_image_extract" {
		item.Note = "使用视频帧提取与图片处理完成商品图片提取，无模型调用"
	}
	return item
}

func (s *AgentService) ListAgentModelBindings(ctx context.Context) ([]AgentModelBindingsDTO, error) {
	workflows, err := s.List(ctx, true)
	if err != nil {
		return nil, err
	}
	models, err := modelcontract.LoadModels(ctx, s.db)
	if err != nil {
		return nil, err
	}
	items := []AgentModelBindingsDTO{}
	for _, w := range workflows {
		items = append(items, buildAgentModelBindings(w, models))
	}
	// Prompt packs provide a code default when the DB slot is empty.
	for i, w := range workflows {
		if pack, ok := PromptPackByCode(w.Code); ok {
			for j := range items[i].Slots {
				row := &items[i].Slots[j]
				if row.ID == "chat_model_code" && row.ModelCode == "" {
					applyInheritedBinding(row, pack.DefaultChatModelCode, "内置提示词包默认模型", "prompt_pack."+w.Code+".default_chat_model_code", models)
				}
			}
		}
	}
	// Canvas enhancement inherits its own analysis model before the general one.
	for i, w := range workflows {
		if modelcontract.Mode(contractDefinition(w)) != "infinite_canvas" {
			continue
		}
		for j := range items[i].Slots {
			row := &items[i].Slots[j]
			if row.ID != "prompt_enhance_model_code" || row.ModelCode != "" {
				continue
			}
			if code := stringValue(w.RuntimeConfig["analysis_model_code"]); code != "" {
				applyInheritedBinding(row, code, "继承当前智能体的分析模型（保存后独立配置）", "runtime_config.analysis_model_code", models)
				continue
			}
			for _, general := range workflows {
				if general.Code != "general_creative_agent" {
					continue
				}
				code := stringValue(general.RuntimeConfig["analysis_model_code"])
				row.ModelCode = code
				row.SourceLabel = "继承通用创作的分析模型（保存后独立配置）"
				row.Sources = []string{"general_creative_agent.runtime_config.analysis_model_code"}
				row.SharedWith = []string{"general_creative_agent"}
				row.Inherited = true
				row.Compatible = false
				if m, ok := modelcontract.Find(models, code); ok {
					row.ModelName = m.Name
					if err := modelcontract.Validate(row.Slot, m); err == nil {
						row.Compatible = true
						row.Issue = ""
					} else {
						row.Issue = err.Error()
					}
				}
			}
		}
	}
	for i := range items {
		if items[i].Code != "general_creative_agent" {
			continue
		}
		for j := range items[i].Slots {
			if items[i].Slots[j].ID != "analysis_model_code" {
				continue
			}
			for _, other := range items {
				for _, row := range other.Slots {
					if row.Inherited && stringInModelSlice(row.SharedWith, "general_creative_agent") {
						items[i].Slots[j].SharedWith = append(items[i].Slots[j].SharedWith, other.Code)
					}
				}
			}
		}
	}
	return items, nil
}

func applyInheritedBinding(row *AgentModelSlotDTO, code, label, source string, models []modelcontract.Model) {
	row.ModelCode = code
	row.SourceLabel = label
	row.Sources = []string{source}
	row.Inherited = true
	row.Compatible = false
	if m, ok := modelcontract.Find(models, code); ok {
		row.ModelName = m.Name
		if err := modelcontract.Validate(row.Slot, m); err == nil {
			row.Compatible = true
			row.Issue = ""
		} else {
			row.Issue = err.Error()
		}
	} else {
		row.Issue = "继承的模型不存在"
	}
}

type AgentModelSlotUpdate struct {
	ModelCode         string  `json:"model_code"`
	ExpectedModelCode *string `json:"expected_model_code"`
}

// UpdateAgentModelSlot locks only the definition and changes model references;
// prompts, prices, policies, custom node fields and other slots remain intact.
func (s *AgentService) UpdateAgentModelSlot(ctx context.Context, code, slotID string, in AgentModelSlotUpdate) (map[string]interface{}, error) {
	if strings.TrimSpace(in.ModelCode) == "" {
		return nil, errors.New("请选择模型")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var runtimeRaw, nodesRaw []byte
	var category string
	if err = tx.QueryRow(ctx, `SELECT runtime_config,nodes,category FROM workflow_definitions WHERE code=$1 FOR UPDATE`, code).Scan(&runtimeRaw, &nodesRaw, &category); err != nil {
		return nil, err
	}
	d := modelcontract.Definition{Code: code, Category: category}
	if err = json.Unmarshal(runtimeRaw, &d.Runtime); err != nil {
		return nil, err
	}
	if d.Runtime == nil {
		d.Runtime = map[string]interface{}{}
	}
	if err = json.Unmarshal(nodesRaw, &d.Nodes); err != nil {
		return nil, err
	}
	var chosen *modelcontract.Slot
	for _, slot := range bindingSlots(d) {
		if slot.ID == slotID {
			copy := slot
			chosen = &copy
			break
		}
	}
	if chosen == nil {
		return nil, errors.New("模型槽位不存在")
	}
	if in.ExpectedModelCode != nil && *in.ExpectedModelCode != chosen.ModelCode {
		return nil, errors.New("模型配置已被其他管理员更新，请刷新后重试")
	}
	models, err := modelcontract.LoadModels(ctx, tx)
	if err != nil {
		return nil, err
	}
	m, ok := modelcontract.Find(models, strings.TrimSpace(in.ModelCode))
	if !ok {
		return nil, errors.New("模型不存在")
	}
	if err = modelcontract.Validate(*chosen, m); err != nil {
		return nil, err
	}
	var nodes []map[string]interface{}
	if err = json.Unmarshal(nodesRaw, &nodes); err != nil {
		return nil, err
	}
	sources := append([]string{}, chosen.Sources...)
	// Include absent mirrors that the runtime normalizers can recreate later.
	if modelcontract.Mode(d) == "comic_drama" {
		switch slotID {
		case "video_model_code":
			sources = append(sources, "runtime_config.generation_model_code")
		case "analysis_model_code":
			sources = append(sources, "runtime_config.dialogue_model_codes.0")
		}
	}
	for _, source := range sources {
		parts := strings.Split(source, ".")
		if len(parts) >= 2 && parts[0] == "runtime_config" {
			if parts[1] == "dialogue_model_codes" {
				codes := modelcontract.Strings(d.Runtime[parts[1]])
				var idx int
				if len(parts) != 3 {
					return nil, errors.New("槽位来源无效")
				}
				if _, err = fmt.Sscanf(parts[2], "%d", &idx); err != nil || idx < 0 || idx > len(codes) {
					return nil, errors.New("槽位来源无效")
				}
				if idx == len(codes) {
					codes = append(codes, m.Code)
				} else {
					codes[idx] = m.Code
				}
				d.Runtime[parts[1]] = codes
			} else {
				d.Runtime[parts[1]] = m.Code
			}
		} else if strings.HasPrefix(source, "nodes.") && strings.HasSuffix(source, ".model_code") {
			nodeID := strings.TrimSuffix(strings.TrimPrefix(source, "nodes."), ".model_code")
			for _, node := range nodes {
				if stringValue(node["id"]) == nodeID {
					node["model_code"] = m.Code
				}
			}
		}
	}
	runtimeRaw, err = json.Marshal(d.Runtime)
	if err != nil {
		return nil, err
	}
	nodesRaw, err = json.Marshal(nodes)
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `UPDATE workflow_definitions SET runtime_config=$1,nodes=$2,updated_at=now() WHERE code=$3`, runtimeRaw, nodesRaw, code); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return map[string]interface{}{"slot_id": slotID, "before": chosen.ModelCode, "after": m.Code, "sources": sources}, nil
}

// ValidateProjectModels also runs before retries, covering user-provided model
// overrides before they can mutate a failed project's saved checkpoints.
func (s *AgentService) ValidateProjectModels(ctx context.Context, userID int64, publicID string, overrides map[string]string) error {
	if len(overrides) == 0 {
		return nil
	}
	var code string
	var runtimeRaw []byte
	if err := s.db.QueryRow(ctx, `SELECT COALESCE(to_jsonb(w)->>'code',''),w.runtime_config FROM workflow_projects p JOIN workflow_definitions w ON w.id=p.workflow_id WHERE p.public_id=$1 AND p.user_id=$2`, publicID, userID).Scan(&code, &runtimeRaw); err != nil {
		return err
	}
	runtime := map[string]interface{}{}
	if err := json.Unmarshal(runtimeRaw, &runtime); err != nil {
		return err
	}
	w := WorkflowDTO{Code: code, RuntimeConfig: runtime}
	if code == "" && stringValue(runtime["agent_mode"]) == "comic_drama" {
		w.Code = "ai_comic_drama"
	}
	provided := map[string]interface{}{}
	for k, v := range overrides {
		provided[k] = v
	}
	return s.validateModelOverrides(ctx, w, provided)
}

// Retain the compile-time guarantee that transactions support capability reads.
var _ modelcontract.Queryer = (pgx.Tx)(nil)

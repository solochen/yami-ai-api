// Package modelcontract defines the server-owned requirements of agent model slots.
// Admin candidates, writes, fallbacks and worker execution all use this package.
package modelcontract

import (
	"fmt"
	"strings"
)

type Node struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Name      string `json:"name"`
	ModelCode string `json:"model_code"`
}

type Definition struct {
	Code     string
	Category string
	Runtime  map[string]interface{}
	Nodes    []Node
}

type Slot struct {
	ID        string   `json:"id"`
	Label     string   `json:"label"`
	Purpose   string   `json:"purpose"`
	Required  []string `json:"required_capabilities"`
	Sources   []string `json:"sources"`
	ModelCode string   `json:"model_code"`
	Optional  bool     `json:"optional"`
}

type Route struct {
	Enabled  bool                   `json:"is_enabled"`
	Rule     map[string]interface{} `json:"runtime_rule"`
	Endpoint string                 `json:"endpoint"`
}

type Model struct {
	ID       int64                  `json:"id"`
	Code     string                 `json:"code"`
	Alias    string                 `json:"public_alias"`
	Name     string                 `json:"display_name"`
	Category string                 `json:"category"`
	Mode     string                 `json:"request_mode"`
	Enabled  bool                   `json:"is_enabled"`
	Rule     map[string]interface{} `json:"runtime_rule"`
	Schema   map[string]interface{} `json:"input_schema"`
	Endpoint string                 `json:"new_api_endpoint"`
	Routes   []Route                `json:"routes"`
}

var Labels = map[string]string{
	"chat": "文本生成", "vision": "图片理解", "video_analysis": "视频理解",
	"image": "图片生成", "reference_images": "参考图输入", "multi_reference": "多参考图输入",
	"video": "视频生成", "image_to_video": "首帧或参考图输入", "first_frame": "首帧输入",
	"first_last": "首尾帧输入", "speech": "语音合成 TTS", "music": "音乐生成",
	"video_upscale": "视频超分", "video_redraw": "视频重绘", "subtitle_remove": "字幕移除",
	"mask_edit": "蒙版编辑", "embedding": "文本向量化", "unsupported_node_type": "已支持的节点类型",
}

func Record(v interface{}) map[string]interface{} { m, _ := v.(map[string]interface{}); return m }
func String(v interface{}) string                 { s, _ := v.(string); return strings.TrimSpace(s) }
func Strings(v interface{}) []string {
	switch values := v.(type) {
	case []string:
		return values
	case []interface{}:
		out := []string{}
		for _, v := range values {
			out = append(out, String(v))
		}
		return out
	case string:
		if String(values) != "" {
			return []string{String(values)}
		}
	}
	return nil
}
func number(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	}
	return 0
}

// Explicit false beats legacy aliases. A route override also beats aliases on
// the model, even when the route and model use different capability names.
func flag(rule map[string]interface{}, keys ...string) (bool, bool) {
	caps := Record(rule["capabilities"])
	for _, key := range keys {
		if value, ok := caps[key].(bool); ok {
			return value, true
		}
	}
	return false, false
}
func capability(modelRule, routeRule map[string]interface{}, keys ...string) (bool, bool) {
	if value, ok := flag(routeRule, keys...); ok {
		return value, true
	}
	return flag(modelRule, keys...)
}
func Merge(a, b map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		if child, ok := v.(map[string]interface{}); ok {
			out[k] = Merge(Record(out[k]), child)
		} else {
			out[k] = v
		}
	}
	return out
}

func modes(schema map[string]interface{}) []string {
	props := Record(schema["properties"])
	out := []string{}
	for _, key := range []string{"generation_mode", "mode"} {
		out = append(out, Strings(Record(props[key])["enum"])...)
	}
	return out
}
func contains(items []string, values ...string) bool {
	for _, v := range items {
		for _, want := range values {
			if v == want {
				return true
			}
		}
	}
	return false
}
func hasField(schema map[string]interface{}, keys ...string) bool {
	props := Record(schema["properties"])
	for _, k := range keys {
		if _, ok := props[k]; ok {
			return true
		}
	}
	return false
}

func supports(m Model, route Route, required string) bool {
	rule := Merge(m.Rule, route.Rule)
	image, video, up := Record(rule["image"]), Record(rule["video"]), Record(rule["upstream"])
	endpoint := strings.ToLower(route.Endpoint)
	if endpoint == "" {
		endpoint = strings.ToLower(m.Endpoint)
	}
	switch required {
	case "chat":
		return m.Category == "chat" && (m.Mode == "chat_completions" || m.Mode == "responses")
	case "vision":
		v, _ := capability(m.Rule, route.Rule, "vision", "image_input", "multimodal")
		return v
	case "video_analysis":
		v, _ := capability(m.Rule, route.Rule, "video_analysis", "video_input", "video_understanding")
		return v
	case "image":
		return m.Category == "image" && (m.Mode == "images" || m.Mode == "image")
	case "video":
		return m.Category == "video" && m.Mode == "video"
	case "reference_images", "multi_reference":
		if v, ok := capability(m.Rule, route.Rule, "reference_images", "image_reference"); ok && !v {
			return false
		}
		max := number(image["max_reference_images"])
		if _, declared := image["max_reference_images"]; declared && max <= 0 {
			return false
		}
		if required == "multi_reference" {
			return max >= 2
		}
		if v, ok := capability(m.Rule, route.Rule, "reference_images", "image_reference"); ok {
			return v
		}
		return max > 0 || hasField(m.Schema, "reference_images", "image", "images", "image_url")
	case "first_frame", "first_last", "image_to_video":
		if v, ok := capability(m.Rule, route.Rule, required); ok {
			return v
		}
		profile := String(video["upload_profile"])
		values := modes(m.Schema)
		first := contains(values, "image", "first_frame", "first_last") || hasField(m.Schema, "first_frame", "first_frame_url") || contains([]string{profile}, "first_last", "first_last_frame", "seedance_2", "minimax_h3", "aliyun_happyhorse_first_frame", "single_ref")
		ref := contains(values, "reference") || hasField(m.Schema, "reference_images") || number(video["max_reference_images"]) > 0
		if len(values) > 0 {
			first = contains(values, "image", "first_frame", "first_last")
			ref = contains(values, "reference")
		}
		if required == "first_last" {
			if len(values) > 0 {
				return contains(values, "first_last")
			}
			return contains(values, "first_last") || hasField(m.Schema, "first_frame") && hasField(m.Schema, "last_frame") || contains([]string{profile}, "first_last", "first_last_frame", "seedance_2", "minimax_h3")
		}
		if required == "first_frame" {
			return first
		}
		return first || ref
	case "speech", "music":
		if m.Category != "audio" || m.Mode != "audio" {
			return false
		}
		keys := []string{"speech_synthesis", "tts"}
		if required == "music" {
			keys = []string{"music_generation"}
		}
		if v, ok := capability(m.Rule, route.Rule, keys...); ok {
			return v
		}
		if required == "speech" {
			return strings.HasSuffix(endpoint, "/audio/speech") || strings.HasSuffix(endpoint, "/t2a_v2") || String(up["adapter"]) == "openrouter_audio" && String(Record(rule["audio"])["output_kind"]) == "speech"
		}
		return strings.HasSuffix(endpoint, "/audio/music") || strings.HasSuffix(endpoint, "/music_generation") || String(Record(rule["audio"])["output_kind"]) == "music" || String(up["adapter"]) == "openrouter_music"
	case "mask_edit":
		if v, ok := capability(m.Rule, route.Rule, "mask_edit"); ok && !v {
			return false
		}
		return String(up["adapter"]) == "openai_images"
	case "embedding":
		return m.Mode == "embeddings" || m.Category == "embedding"
	default:
		v, _ := capability(m.Rule, route.Rule, required)
		return v
	}
}

func Validate(slot Slot, m Model) error {
	if !m.Enabled {
		return fmt.Errorf("%s：模型不存在或已停用：%s", slot.Label, m.Code)
	}
	if contains([]string{m.Code}, "chat_demo_v1", "image_fast_v1", "video_demo_v1") {
		return fmt.Errorf("%s：演示模型不可用于生产", slot.Label)
	}
	routes := m.Routes
	if len(routes) == 0 {
		routes = []Route{{Enabled: true}}
	}
	active := 0
	for _, route := range routes {
		if !route.Enabled {
			continue
		}
		active++
		for _, required := range slot.Required {
			if !supports(m, route, required) {
				return fmt.Errorf("%s：模型 %s 缺少%s能力，或启用线路不兼容", slot.Label, m.Code, Labels[required])
			}
		}
	}
	if active == 0 {
		return fmt.Errorf("%s：模型 %s 没有启用的线路", slot.Label, m.Code)
	}
	return nil
}

// Mode pins built-in workflows to their actual consumer, so changing agent_mode
// or a node's client-supplied type cannot weaken the built-in requirements.
func Mode(d Definition) string {
	builtins := map[string]string{"viral_video_breakdown": "viral_video_breakdown", "ai_comic_drama": "comic_drama", "general_creative_agent": "creative_chat", "product_refine": "product_refine", "ecom_factory": "ecom_factory", "ai_photo_studio": "photo_studio", "ai_virtual_tryon": "virtual_try_on", "ai_novel_workshop": "novel_workshop", "video_upscale": "video_upscale", "video_redraw": "video_redraw", "subtitle_remove": "subtitle_remove", "ecommerce_video": "ecommerce_video", "video_creation": "infinite_canvas", "video_creation_v2": "infinite_canvas", "infinite_canvas": "infinite_canvas", "one_click_viral_remake": "infinite_canvas", "viral_remake": "infinite_canvas", "video_remake": "infinite_canvas"}
	if mode := builtins[d.Code]; mode != "" {
		return mode
	}
	return String(d.Runtime["agent_mode"])
}

var fields = []string{"analysis_model_code", "chat_model_code", "fallback_model_code", "quality_model_code", "prompt_enhance_model_code", "dialogue_model_codes", "generation_model_code", "image_model_code", "video_model_code", "speech_model_code", "narration_model_code", "audio_model_code", "music_model_code", "embedding_model_code"}

func FieldSlot(d Definition, key string) (Slot, bool) {
	mode := Mode(d)
	slot := Slot{ID: "runtime." + key, Sources: []string{"runtime_config." + key}, Optional: true}
	switch key {
	case "analysis_model_code", "chat_model_code", "fallback_model_code", "quality_model_code", "prompt_enhance_model_code", "dialogue_model_codes":
		slot.Required = []string{"chat"}
		slot.Label = map[string]string{"analysis_model_code": "分析与规划", "chat_model_code": "智能体对话", "fallback_model_code": "备用分析模型", "quality_model_code": "视觉验收", "prompt_enhance_model_code": "提示词优化", "dialogue_model_codes": "剧本与对白模型"}[key]
		slot.Purpose = map[string]string{"analysis_model_code": "理解需求、分析输入素材并规划后续步骤", "chat_model_code": "根据智能体提示词进行多轮问答和内容写作", "fallback_model_code": "主分析模型不可用时承担相同分析职责", "quality_model_code": "检查生成图片的内容、质量与参考素材一致性", "prompt_enhance_model_code": "优化提交给生成模型的提示词", "dialogue_model_codes": "按顺序用于剧本规划、对白创作及失败回退"}[key]
		if key == "quality_model_code" || (key == "analysis_model_code" || key == "fallback_model_code") && (mode == "photo_studio" || mode == "product_refine" || mode == "ecom_factory" || mode == "ecommerce_video" || mode == "infinite_canvas") {
			slot.Required = append(slot.Required, "vision")
		}
		if mode == "viral_video_breakdown" && (key == "analysis_model_code" || key == "fallback_model_code") {
			slot.Label = "爆款结构分析"
			slot.Purpose = "理解原视频与抽取的关键帧，拆解镜头、节奏和爆款结构"
			slot.Required = []string{"chat", "vision", "video_analysis"}
			slot.Optional = false
		}
		if mode == "novel_workshop" && key == "analysis_model_code" {
			slot.Label = "故事策划"
			slot.Purpose = "根据创意生成故事设定、人物关系、分卷和章节大纲"
		}
		if mode == "photo_studio" && key == "analysis_model_code" {
			slot.Label = "写真造型设计"
			slot.Purpose = "理解本人照片与风格要求，设计造型、场景和拍摄方案"
		}
		if contains([]string{d.Code}, "one_click_viral_remake", "viral_remake", "video_remake") && (key == "analysis_model_code" || key == "fallback_model_code") {
			slot.Required = []string{"chat", "vision", "video_analysis"}
			slot.Label = "爆款结构分析"
			slot.Purpose = "理解输入视频与关键帧，拆解并规划复刻内容"
		}
	case "image_model_code":
		slot.Label = "关键帧与参考图生成"
		slot.Purpose = "基于角色、场景和商品参考图生成关键帧与图片素材"
		slot.Required = []string{"image", "reference_images"}
	case "video_model_code":
		slot.Label = "视频片段"
		slot.Purpose = "以关键帧或参考图生成视频片段"
		slot.Required = []string{"video", "image_to_video"}
	case "speech_model_code", "narration_model_code", "audio_model_code":
		slot.Label = "配音"
		slot.Purpose = "将对白、旁白文字合成为语音"
		slot.Required = []string{"speech"}
	case "music_model_code":
		slot.Label = "配乐"
		slot.Purpose = "根据创作要求生成背景音乐或歌曲"
		slot.Required = []string{"music"}
	case "embedding_model_code":
		slot.Label = "知识库向量模型"
		slot.Purpose = "将资料与问题编码为向量，用于知识检索"
		slot.Required = []string{"embedding"}
	case "generation_model_code":
		switch {
		case mode == "viral_video_breakdown" || mode == "novel_workshop":
			slot.Required = []string{"chat"}
			slot.Label = "文本生成"
			slot.Purpose = "整理分析结果或生成章节正文"
			if mode == "novel_workshop" {
				slot.Label = "章节创作、润色与归档"
				slot.Purpose = "生成章节正文、润色审校并更新设定档案；这些阶段共用同一模型"
			}
		case contains([]string{mode}, "video_upscale", "video_redraw", "subtitle_remove"):
			slot.Required = []string{"video", mode}
			slot.Label = Labels[mode]
			slot.Purpose = "对输入视频执行" + Labels[mode]
		case mode == "comic_drama" || mode == "ecommerce_video" || String(d.Runtime["generation_type"]) == "video":
			slot.Required = []string{"video", "image_to_video"}
			slot.Label = "视频片段"
			slot.Purpose = "以生成图片为首帧或参考图生成视频"
		case mode == "creative_chat":
			slot.Required = []string{"image", "reference_images"}
			slot.Label = "图片生成"
			slot.Purpose = "通用创作图片模型的旧配置镜像，与图片槽位同步更新"
		default:
			slot.Required = []string{"image"}
			slot.Label = "图片生成"
			slot.Purpose = "根据创作方案生成图片"
		}
		if mode == "product_refine" {
			slot.Required = []string{"image", "reference_images", "mask_edit"}
			slot.Label = "商品编辑"
			slot.Purpose = "根据商品参考图与蒙版进行局部编辑"
		}
		if mode == "virtual_try_on" {
			slot.Required = []string{"image", "multi_reference"}
			slot.Purpose = "同时使用人物图和服装图生成试穿图片"
		}
		if mode == "photo_studio" || mode == "ecom_factory" || mode == "ecommerce_video" {
			if contains(slot.Required, "image") {
				slot.Required = append(slot.Required, "reference_images")
			}
		}
	default:
		return Slot{}, false
	}
	if contains(slot.Required, "video") && mode == "ecommerce_video" {
		slot.Required = []string{"video", "first_frame"}
	}
	if contains(slot.Required, "video") && Record(d.Runtime["input_capabilities"])["support_first_last_frame"] == true {
		slot.Required = append(slot.Required, "first_last")
	}
	if mode == "comic_drama" && contains([]string{"analysis_model_code", "image_model_code", "video_model_code"}, key) {
		slot.Optional = false
		if key == "analysis_model_code" && len(Strings(d.Runtime["dialogue_model_codes"])) > 0 {
			slot.Optional = true
		}
	}
	return slot, true
}

// NodeRuntimeField identifies descriptive nodes that mirror a runtime binding.
// Custom workflow nodes keep their own bindings even when their IDs coincide.
func NodeRuntimeField(d Definition, n Node) string {
	switch Mode(d) {
	case "comic_drama":
		return map[string]string{"comic_plan": "analysis_model_code", "keyframes": "image_model_code", "video_segments": "video_model_code", "narrations": "narration_model_code"}[n.ID]
	case "simple_pipeline":
		return map[string]string{"analysis": "analysis_model_code", "analyze": "analysis_model_code", "generate": "generation_model_code", "poster": "generation_model_code"}[n.ID]
	case "photo_studio":
		return map[string]string{"styling": "analysis_model_code", "generate": "generation_model_code"}[n.ID]
	case "novel_workshop":
		return map[string]string{"planning": "analysis_model_code", "writing": "generation_model_code", "polishing": "generation_model_code", "archiving": "generation_model_code"}[n.ID]
	case "virtual_try_on":
		if n.ID == "try_on" {
			return "generation_model_code"
		}
	case "video_upscale", "video_redraw", "subtitle_remove":
		if contains([]string{"upscale", "redraw", "remove_subtitle", "subtitle_remove"}, n.ID) {
			return "generation_model_code"
		}
	case "creative_chat":
		return map[string]string{"analysis": "analysis_model_code", "generate": "image_model_code"}[n.ID]
	case "ecommerce_video":
		return map[string]string{"poster": "image_model_code", "video": "generation_model_code"}[n.ID]
	}
	return ""
}

func NodeSlot(d Definition, n Node) Slot {
	key := NodeRuntimeField(d, n)
	mirror := key != ""
	if key == "" && Mode(d) == "ecommerce_video" && n.ID == "copy" {
		key = "analysis_model_code"
	}
	if key == "" {
		switch n.Type {
		case "llm", "chat":
			key = "chat_model_code"
		case "image":
			key = "generation_model_code"
		case "video":
			key = "video_model_code"
		case "audio":
			key = "narration_model_code"
		}
	}
	slot, ok := FieldSlot(d, key)
	if !ok {
		slot = Slot{Required: []string{"unsupported_node_type"}, Purpose: "未知节点类型，需要先补充服务端能力规则"}
	}
	if !mirror && n.Type == "image" && key == "generation_model_code" && !contains(slot.Required, "image") {
		slot.Required = []string{"image"}
	}
	slot.ID = "node." + n.ID
	slot.Sources = []string{"nodes." + n.ID + ".model_code"}
	slot.ModelCode = n.ModelCode
	if !mirror {
		slot.Optional = false
	}
	if n.Name != "" {
		slot.Label = n.Name
	}
	return slot
}

func Slots(d Definition) []Slot {
	out := []Slot{}
	expected := ExpectedFields(d)
	for _, key := range fields {
		raw, exists := d.Runtime[key]
		if !exists && !contains(expected, key) {
			continue
		}
		slot, _ := FieldSlot(d, key)
		if key == "dialogue_model_codes" {
			for i, code := range Strings(raw) {
				s := slot
				s.ID = fmt.Sprintf("runtime.%s.%d", key, i)
				s.Sources = []string{fmt.Sprintf("runtime_config.%s.%d", key, i)}
				s.Label = fmt.Sprintf("剧本与对白模型 %d", i+1)
				s.ModelCode = code
				out = append(out, s)
			}
			continue
		}
		slot.ModelCode = String(raw)
		out = append(out, slot)
	}
	for _, n := range d.Nodes {
		if Mode(d) == "comic_drama" && n.ID == "compose" {
			continue // Local ffmpeg composition has no model call.
		}
		if n.ModelCode != "" || contains([]string{"llm", "chat", "image", "video", "audio"}, n.Type) || NodeRuntimeField(d, n) != "" {
			out = append(out, NodeSlot(d, n))
		}
	}
	return out
}

func ExpectedFields(d Definition) []string {
	switch Mode(d) {
	case "creative_chat":
		return []string{"analysis_model_code", "fallback_model_code", "image_model_code", "video_model_code", "speech_model_code", "music_model_code"}
	case "infinite_canvas":
		return []string{"analysis_model_code", "quality_model_code", "prompt_enhance_model_code", "image_model_code", "video_model_code", "audio_model_code"}
	case "comic_drama":
		return []string{"analysis_model_code", "image_model_code", "video_model_code", "narration_model_code"}
	case "viral_video_breakdown":
		return []string{"analysis_model_code"}
	case "photo_studio", "ecom_factory", "product_refine", "novel_workshop", "simple_pipeline":
		return []string{"analysis_model_code", "generation_model_code"}
	case "virtual_try_on", "video_upscale", "video_redraw", "subtitle_remove":
		return []string{"generation_model_code"}
	case "prompt_chat":
		return []string{"chat_model_code"}
	case "renovation_budget":
		return []string{"chat_model_code", "embedding_model_code"}
	}
	return nil
}

// Overrides resolves legacy request aliases used by specialized consumers.
func Overrides(d Definition, inputs map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	for _, key := range fields {
		if v, ok := inputs[key]; ok {
			out[key] = v
		}
	}
	if code := String(inputs["model_code"]); code != "" {
		switch Mode(d) {
		case "photo_studio", "virtual_try_on":
			out["generation_model_code"] = code
		case "novel_workshop":
			out["analysis_model_code"], out["generation_model_code"] = code, code
		}
	}
	if code := String(inputs["dialogue_model_code"]); code != "" && Mode(d) == "comic_drama" {
		out["dialogue_model_codes"] = []string{code}
	}
	return out
}

// Effective returns every configured default/backup and request override. The
// worker uses this too, so changing a model after submission cannot bypass it.
func Effective(d Definition, inputs map[string]interface{}) Definition {
	d.Runtime = Merge(d.Runtime, nil)
	for key, v := range Overrides(d, inputs) {
		d.Runtime[key] = v
	}
	if Mode(d) == "comic_drama" {
		if String(d.Runtime["video_model_code"]) == "" {
			d.Runtime["video_model_code"] = String(d.Runtime["generation_model_code"])
		}
		if code := String(inputs["video_model_code"]); code != "" {
			d.Runtime["generation_model_code"] = code
		}
	}
	// Specialized consumers use runtime fields; nodes are descriptive mirrors.
	if !contains([]string{Mode(d)}, "custom_nodes", "ecommerce_video", "") {
		d.Nodes = nil
	}
	return d
}

func Find(models []Model, code string) (Model, bool) {
	for _, m := range models {
		if m.Code == code || m.Alias != "" && m.Alias == code {
			return m, true
		}
	}
	return Model{}, false
}

func ValidateDefinition(d Definition, models []Model) error {
	if configured := String(d.Runtime["agent_mode"]); configured != "" && Mode(d) != configured && Mode(d) != "ecommerce_video" {
		return fmt.Errorf("内置智能体 %s 的运行模式必须为 %s", d.Code, Mode(d))
	}
	for _, key := range fields {
		if raw, ok := d.Runtime[key]; ok && raw != nil {
			if key == "dialogue_model_codes" {
				switch values := raw.(type) {
				case []string:
				case []interface{}:
					for _, v := range values {
						if _, ok := v.(string); !ok {
							return fmt.Errorf("%s必须是模型编码数组", key)
						}
					}
				default:
					return fmt.Errorf("%s必须是模型编码数组", key)
				}
			} else if _, ok := raw.(string); !ok {
				return fmt.Errorf("%s必须是模型编码字符串", key)
			}
		}
	}
	for _, slot := range Slots(d) {
		if slot.ModelCode == "" {
			if slot.Optional {
				continue
			}
			return fmt.Errorf("%s未配置模型", slot.Label)
		}
		m, ok := Find(models, slot.ModelCode)
		if !ok {
			return fmt.Errorf("%s：模型不存在：%s", slot.Label, slot.ModelCode)
		}
		if err := Validate(slot, m); err != nil {
			return err
		}
	}
	return nil
}

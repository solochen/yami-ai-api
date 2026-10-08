package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RenovationService 装修预算助理 Agent：材料询价 / 工程算量 / 知识库检索 / 报告生成。
type RenovationService struct {
	db  *pgxpool.Pool
	chat *ChatService
	kb  *RenovationKB
}

func NewRenovationService(db *pgxpool.Pool, chat *ChatService) *RenovationService {
	s := &RenovationService{db: db, chat: chat}
	s.kb = NewRenovationKB(db, chat)
	return s
}

// ============ 工具 Schema（OpenAI function calling） ============

func RenovationToolSchemas() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "query_material_price",
				"description": "查询装修材料的市场价格。支持瓷砖、地板、乳胶漆、壁纸、室内门、辅料等。任何涉及材料价格/费用的问题都必须先调用本工具查询，禁止凭记忆报价。",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"material": map[string]interface{}{
							"type":        "string",
							"description": "材料名称关键词，如：抛釉砖、强化复合地板、乳胶漆面漆、壁纸、烤漆门",
						},
						"category": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"tile", "floor", "paint", "wallpaper", "door", "auxiliary"},
							"description": "材料大类（可选）。tile=瓷砖 floor=地板 paint=乳胶漆 wallpaper=壁纸 door=门窗 auxiliary=辅料",
						},
						"brand_tier": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"budget", "mainstream", "premium"},
							"description": "预算档位（可选）。budget=经济 mainstream=主流 premium=高端",
						},
					},
					"required": []string{"material"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "calc_material_quantity",
				"description": "工程算量：根据房间尺寸计算材料用量。瓷砖返回片数、地板返回平方米、乳胶漆返回 5L 桶数、壁纸返回卷数。任何面积/用量/数量计算都必须调用本工具，禁止心算。",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"material_type": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"tile", "floor", "paint", "wallpaper"},
							"description": "材料类型。tile=瓷砖(按片) floor=地板(按平米) paint=乳胶漆(按5L桶) wallpaper=壁纸(按卷)",
						},
						"room_name": map[string]interface{}{
							"type":        "string",
							"description": "房间名称，如：客厅、主卧",
						},
						"room_length": map[string]interface{}{
							"type":        "number",
							"description": "房间长度（米），必须是具体数字",
						},
						"room_width": map[string]interface{}{
							"type":        "number",
							"description": "房间宽度（米），必须是具体数字",
						},
						"room_height": map[string]interface{}{
							"type":        "number",
							"description": "房间层高（米），默认 2.7",
						},
						"include_ceiling": map[string]interface{}{
							"type":        "boolean",
							"description": "乳胶漆是否刷顶面，默认 true",
						},
						"coats": map[string]interface{}{
							"type":        "integer",
							"description": "乳胶漆面漆遍数，默认 2",
						},
						"piece_area": map[string]interface{}{
							"type":        "number",
							"description": "瓷砖单片面积（平方米）。600x600 砖为 0.36，800x800 砖为 0.64，300x600 砖为 0.18。material_type=tile 时必填",
						},
						"loss_rate": map[string]interface{}{
							"type":        "number",
							"description": "损耗率（0-0.3），瓷砖/地板默认 0.05，壁纸默认 0.12",
						},
					},
					"required": []string{"material_type", "room_length", "room_width"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "search_knowledge",
				"description": "检索装修软装知识库，内容涵盖：装修风格（现代简约/北欧/奶油风/原木风/新中式/轻奢等）、空间设计技巧（小户型/采光/动线/各房间要点）、材料选购指南、预算分配与避坑。凡是风格特点、搭配建议、选购知识类问题都应先检索再回答。",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]interface{}{
							"type":        "string",
							"description": "检索问题或关键词，如：奶油风适合小户型吗、乳胶漆环保等级、全屋预算怎么分配",
						},
						"top_k": map[string]interface{}{
							"type":        "integer",
							"description": "返回条数，默认 3，最多 5",
						},
					},
					"required": []string{"query"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "generate_report",
				"description": "把最终的装修预算方案整理成 Markdown 报告并交付给用户下载。当一轮对话中所有询价和算量完成后，必须调用本工具输出结构化报告。报告需包含：需求回顾、各房间材料用量与费用明细、合计、风格建议。",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"title": map[string]interface{}{
							"type":        "string",
							"description": "报告标题，如：89平三室两厅全屋装修预算报告",
						},
						"markdown_content": map[string]interface{}{
							"type":        "string",
							"description": "完整的 Markdown 格式报告正文，使用表格呈现明细",
						},
					},
					"required": []string{"title", "markdown_content"},
				},
			},
		},
	}
}

// ============ 工具执行入口 ============

// ExecuteRenovationTool 执行一个工具调用，返回 tool 消息的内容。
// 业务性错误（参数缺失等）以 {"error": ...} 返回给模型自愈；基础设施错误返回 err。
// embedModel 为本次运行的 embedding 模型 code（用于知识库检索，可为空走关键词模式）。
func (s *RenovationService) ExecuteRenovationTool(ctx context.Context, name, arguments, embedModel string) (string, error) {
	var args map[string]interface{}
	if arguments != "" {
		if err := json.Unmarshal([]byte(arguments), &args); err != nil {
			return toolErrorf("参数不是合法 JSON：%v", err), nil
		}
	}
	switch name {
	case "query_material_price":
		return s.execQueryMaterialPrice(ctx, args)
	case "calc_material_quantity":
		return s.execCalcQuantity(args)
	case "search_knowledge":
		return s.execSearchKnowledge(ctx, args, embedModel)
	case "generate_report":
		return s.execGenerateReport(args)
	default:
		return toolErrorf("未知工具：%s", name), nil
	}
}

func toolErrorf(format string, a ...interface{}) string {
	b, _ := json.Marshal(map[string]interface{}{"error": fmt.Sprintf(format, a...)})
	return string(b)
}

func argString(args map[string]interface{}, key string) string {
	if v, ok := args[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func argFloat(args map[string]interface{}, key string, fallback float64) float64 {
	if v, ok := args[key].(float64); ok {
		return v
	}
	return fallback
}

func argInt(args map[string]interface{}, key string, fallback int) int {
	if v, ok := args[key].(float64); ok {
		return int(v)
	}
	return fallback
}

// ============ query_material_price ============

var renovationCategoryAliases = map[string]string{
	"瓷砖": "tile", "地砖": "tile", "墙砖": "tile", "砖": "tile", "tile": "tile",
	"地板": "floor", "木地板": "floor", "floor": "floor",
	"乳胶漆": "paint", "漆": "paint", "油漆": "paint", "paint": "paint",
	"壁纸": "wallpaper", "墙纸": "wallpaper", "wallpaper": "wallpaper",
	"门": "door", "室内门": "door", "房门": "door", "door": "door",
	"辅料": "auxiliary", "auxiliary": "auxiliary",
}

func (s *RenovationService) execQueryMaterialPrice(ctx context.Context, args map[string]interface{}) (string, error) {
	material := argString(args, "material")
	category := argString(args, "category")
	brandTier := argString(args, "brand_tier")
	if material == "" && category == "" {
		return toolErrorf("material 或 category 至少提供一个"), nil
	}
	if alias, ok := renovationCategoryAliases[material]; ok && category == "" {
		category = alias
		material = ""
	}
	if alias, ok := renovationCategoryAliases[category]; ok {
		category = alias
	}

	sql := `SELECT name, category, spec, unit, price, brand_tier, note
		FROM renovation_materials
		WHERE is_active
		  AND ($1 = '' OR name ILIKE '%' || $1 || '%')
		  AND ($2 = '' OR category = $2)
		  AND ($3 = '' OR brand_tier = $3)
		ORDER BY CASE brand_tier WHEN 'budget' THEN 1 WHEN 'mainstream' THEN 2 ELSE 3 END, price
		LIMIT 8`
	rows, err := s.db.Query(ctx, sql, material, category, brandTier)
	if err != nil {
		return "", fmt.Errorf("查询材料价格失败: %w", err)
	}
	defer rows.Close()

	type item struct {
		Name      string  `json:"name"`
		Category  string  `json:"category"`
		Spec      string  `json:"spec"`
		Unit      string  `json:"unit"`
		Price     float64 `json:"price"`
		BrandTier string  `json:"brand_tier"`
		Note      string  `json:"note,omitempty"`
	}
	items := []item{}
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.Name, &it.Category, &it.Spec, &it.Unit, &it.Price, &it.BrandTier, &it.Note); err != nil {
			return "", fmt.Errorf("读取材料价格失败: %w", err)
		}
		items = append(items, it)
	}
	rows.Close()
	// 名称未命中时，按别名关键词回退到类别检索（如"乳胶漆面漆"→paint 类）
	if len(items) == 0 && material != "" {
		for key, alias := range renovationCategoryAliases {
			if key != material && strings.Contains(material, key) {
				fallback, err := s.db.Query(ctx, sql, "", alias, brandTier)
				if err != nil {
					return "", fmt.Errorf("查询材料价格失败: %w", err)
				}
				for fallback.Next() {
					var it item
					if err := fallback.Scan(&it.Name, &it.Category, &it.Spec, &it.Unit, &it.Price, &it.BrandTier, &it.Note); err == nil {
						items = append(items, it)
					}
				}
				fallback.Close()
				break
			}
		}
	}
	if len(items) == 0 {
		return toolErrorf("没有找到匹配的材料，请尝试更宽泛的关键词，或换用 category 参数（tile/floor/paint/wallpaper/door/auxiliary）"), nil
	}
	b, _ := json.Marshal(map[string]interface{}{"count": len(items), "items": items})
	return string(b), nil
}

// ============ calc_material_quantity ============

func (s *RenovationService) execCalcQuantity(args map[string]interface{}) (string, error) {
	materialType := argString(args, "material_type")
	roomName := argString(args, "room_name")
	length := argFloat(args, "room_length", 0)
	width := argFloat(args, "room_width", 0)
	height := argFloat(args, "room_height", 2.7)
	coats := argInt(args, "coats", 2)
	pieceArea := argFloat(args, "piece_area", 0)
	lossRate := argFloat(args, "loss_rate", 0)
	includeCeiling := true
	if v, ok := args["include_ceiling"].(bool); ok {
		includeCeiling = v
	}

	if roomName == "" {
		roomName = "房间"
	}
	if length <= 0 || width <= 0 || length > 100 || width > 100 {
		return toolErrorf("room_length / room_width 必须是 0-100 之间的数字（单位：米），当前为 %v x %v", length, width), nil
	}

	floorArea := length * width
	result := map[string]interface{}{
		"room":       roomName,
		"floor_area": round2(floorArea),
	}

	switch materialType {
	case "tile":
		if pieceArea <= 0 || pieceArea > 4 {
			return toolErrorf("material_type=tile 时必须提供 piece_area（单片面积平方米）：600x600=0.36，800x800=0.64，300x600=0.18，750x1500=1.125"), nil
		}
		if lossRate == 0 {
			lossRate = 0.05
		}
		pieces := int(math.Ceil(floorArea * (1 + lossRate) / pieceArea))
		result["quantity"] = pieces
		result["unit"] = "片"
		result["formula"] = fmt.Sprintf("ceil(%.2f㎡ x (1+%.0f%%) / %.3f㎡每片)", floorArea, lossRate*100, pieceArea)
	case "floor":
		if lossRate == 0 {
			lossRate = 0.05
		}
		qty := int(math.Ceil(floorArea * (1 + lossRate)))
		result["quantity"] = qty
		result["unit"] = "平方米"
		result["formula"] = fmt.Sprintf("ceil(%.2f㎡ x (1+%.0f%%))", floorArea, lossRate*100)
	case "paint":
		if coats <= 0 || coats > 4 {
			coats = 2
		}
		wallArea := (length + width) * 2 * height * 0.85 // 0.85 = 扣除门窗洞口
		area := wallArea
		if includeCeiling {
			area += floorArea
		}
		liters := area * float64(coats) * 0.115 // 每平米每遍约 0.115L
		buckets := int(math.Ceil(liters / 5))
		result["wall_area"] = round2(wallArea)
		result["paint_area"] = round2(area)
		result["coats"] = coats
		result["liters"] = round2(liters)
		result["quantity"] = buckets
		result["unit"] = "桶(5L)"
		result["formula"] = fmt.Sprintf("((%.0f+%.0f)x2x%.1fx0.85%s) x %d遍 x 0.115L/㎡ ÷ 5L", length, width, height, ceilingMark(includeCeiling, floorArea), coats)
		result["note"] = "底漆只需 1 遍，用量约为面漆一半，需单独计算"
	case "wallpaper":
		if lossRate == 0 {
			lossRate = 0.12
		}
		wallArea := (length + width) * 2 * height * 0.85
		rolls := int(math.Ceil(wallArea * (1 + lossRate) / 5.3)) // 每卷 0.53x10m ≈ 5.3㎡
		result["wall_area"] = round2(wallArea)
		result["quantity"] = rolls
		result["unit"] = "卷"
		result["formula"] = fmt.Sprintf("ceil((%.0f+%.0f)x2x%.1fx0.85 x (1+%.0f%%) / 5.3㎡每卷)", length, width, height, lossRate*100)
	default:
		return toolErrorf("material_type 必须是 tile/floor/paint/wallpaper 之一，当前为 %q", materialType), nil
	}

	b, _ := json.Marshal(result)
	return string(b), nil
}

func ceilingMark(include bool, floorArea float64) string {
	if include {
		return fmt.Sprintf("+顶%.2f", floorArea)
	}
	return ""
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// ============ search_knowledge ============

func (s *RenovationService) execSearchKnowledge(ctx context.Context, args map[string]interface{}, embedModel string) (string, error) {
	query := argString(args, "query")
	if query == "" {
		return toolErrorf("query 不能为空"), nil
	}
	topK := argInt(args, "top_k", 3)
	if topK <= 0 || topK > 5 {
		topK = 3
	}
	results, err := s.kb.Search(ctx, query, embedModel, topK)
	if err != nil {
		return "", fmt.Errorf("知识库检索失败: %w", err)
	}
	if len(results) == 0 {
		return toolErrorf("知识库没有相关内容，请基于常识回答并说明可能不够专业"), nil
	}
	b, _ := json.Marshal(map[string]interface{}{"count": len(results), "results": results})
	return string(b), nil
}

// ============ generate_report ============

func (s *RenovationService) execGenerateReport(args map[string]interface{}) (string, error) {
	title := argString(args, "title")
	content := argString(args, "markdown_content")
	if title == "" || len(content) < 50 {
		return toolErrorf("title 必填；markdown_content 必须是完整的报告正文（至少 50 字），包含明细表格"), nil
	}
	filename := fmt.Sprintf("装修预算报告_%s.md", time.Now().Format("20060102_150405"))
	preview := []rune(content)
	if len(preview) > 120 {
		preview = preview[:120]
	}
	b, _ := json.Marshal(map[string]interface{}{
		"filename":   filename,
		"title":      title,
		"size_chars": len([]rune(content)),
		"preview":    string(preview) + "…",
		"delivered":  true,
	})
	return string(b), nil
}

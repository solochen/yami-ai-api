package handler

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/starai/api/internal/runtime"
	"github.com/starai/api/internal/service"
	"github.com/starai/api/internal/util"
)

func (h *Handler) RewriteViralBreakdown(c *gin.Context) {
	var req struct {
		Kind string `json:"kind"`
	}
	if c.ShouldBindJSON(&req) != nil {
		util.BadRequest(c, "改写参数错误")
		return
	}
	instruction, ok := breakdownRewriteInstruction(req.Kind)
	if !ok {
		util.BadRequest(c, "改写类型无效")
		return
	}
	userID := c.GetInt64("user_id")
	project, err := h.agents.GetProject(c.Request.Context(), userID, c.Param("id"))
	if err != nil || project == nil || project.WorkflowCode != "viral_video_breakdown" {
		util.NotFound(c, "项目不存在")
		return
	}
	script := ""
	if project.Outputs != nil {
		script, _ = project.Outputs["script_markdown"].(string)
	}
	script = strings.TrimSpace(script)
	if project.Status != "succeeded" || script == "" {
		util.BadRequest(c, "剧本完成后才能改写")
		return
	}
	definition, err := h.agents.Get(c.Request.Context(), "viral_video_breakdown")
	if err != nil || definition == nil {
		util.BadRequest(c, "拆解智能体不存在")
		return
	}
	modelCode, _ := definition.RuntimeConfig["analysis_model_code"].(string)
	modelCode = strings.TrimSpace(modelCode)
	if modelCode == "" {
		util.BadRequest(c, "请先在后台配置分析模型")
		return
	}
	result, err := h.chat.Completion(c.Request.Context(), userID, service.CompletionInput{
		ModelCode: modelCode,
		Messages: []runtime.ChatMessage{{
			Role:    "user",
			Content: instruction + "\n\n" + script,
		}},
		Ephemeral: true,
		Params:    map[string]interface{}{"temperature": 0.2},
	})
	if err != nil {
		util.BadRequest(c, "改写失败，原稿未改动")
		return
	}
	if err = h.agents.SaveBreakdownRewrite(c.Request.Context(), userID, project.PublicID, req.Kind, result.Content); err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	updated, err := h.agents.GetProject(c.Request.Context(), userID, project.PublicID)
	if err != nil {
		util.InternalError(c, "改写已保存，刷新后查看")
		return
	}
	util.OK(c, updated)
}

func breakdownRewriteInstruction(kind string) (string, bool) {
	switch kind {
	case "shorten":
		return "把下面的剧本缩写到大约一半长度。保留人物、场景、时间单元和关键动作，不要新增剧情。", true
	case "oral":
		return "把下面的剧本改成可以口播的中文讲稿。保留原有事实，不要新增剧情。", true
	case "storyboard":
		return "把下面的剧本整理成分镜表，使用 Markdown 表格，列包括时间、景别、动作、台词。不要新增剧情。", true
	case "english":
		return "把下面的剧本翻译成英文，保留镜头结构和人物称呼，不要新增剧情。", true
	default:
		return "", false
	}
}

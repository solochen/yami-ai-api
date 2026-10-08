package handler

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/starai/api/internal/service"
	"github.com/starai/api/internal/util"
)

func (h *Handler) AdminListGalleryGapTasks(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "30"))
	items, total, err := h.gallery.ListGalleryGapTasks(c.Request.Context(), c.Query("status"), page, pageSize)
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	util.OK(c, map[string]interface{}{"items": items, "total": total, "page": page, "page_size": pageSize})
}

func (h *Handler) AdminListGalleryCaseSpecs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "30"))
	items, total, err := h.gallery.ListGalleryCaseSpecs(c.Request.Context(), c.Query("status"), c.Query("gap_task"), page, pageSize)
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	util.OK(c, map[string]interface{}{"items": items, "total": total, "page": page, "page_size": pageSize})
}

func (h *Handler) AdminGetGalleryCaseSpec(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		util.NotFound(c, "案例规格不存在")
		return
	}
	item, err := h.gallery.GetGalleryCaseSpec(c.Request.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		util.NotFound(c, "案例规格不存在")
		return
	}
	if err != nil {
		util.InternalError(c, err.Error())
		return
	}
	util.OK(c, item)
}

func (h *Handler) AdminListGalleryCaseSpecVersions(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		util.NotFound(c, "案例规格不存在")
		return
	}
	items, err := h.gallery.ListGalleryCaseSpecVersions(c.Request.Context(), id)
	if err != nil {
		util.InternalError(c, err.Error())
		return
	}
	util.OK(c, items)
}

func (h *Handler) AdminImportGalleryCaseSpecs(c *gin.Context) {
	var req struct {
		Tasks []service.GalleryGapTaskInput  `json:"tasks"`
		Specs []service.GalleryCaseSpecInput `json:"specs"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "案例规格 JSON 格式错误")
		return
	}
	items, err := h.gallery.ImportGalleryCaseSpecs(c.Request.Context(), req.Tasks, req.Specs, c.GetInt64("admin_id"))
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	h.admin.LogOperation(c.Request.Context(), c.GetInt64("admin_id"), "import_gallery_case_specs", "gallery_case_spec", "batch", map[string]interface{}{"tasks": len(req.Tasks), "specs": len(req.Specs)})
	util.OK(c, map[string]interface{}{"items": items, "task_count": len(req.Tasks), "spec_count": len(req.Specs)})
}

func (h *Handler) AdminUpdateGalleryCaseSpec(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		util.NotFound(c, "案例规格不存在")
		return
	}
	var req service.GalleryCaseSpecUpdateInput
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "案例规格参数错误")
		return
	}
	item, err := h.gallery.UpdateGalleryCaseSpec(c.Request.Context(), id, c.GetInt64("admin_id"), req)
	if errors.Is(err, pgx.ErrNoRows) {
		util.NotFound(c, "案例规格不存在")
		return
	}
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	h.admin.LogOperation(c.Request.Context(), c.GetInt64("admin_id"), "update_gallery_case_spec", "gallery_case_spec", strconv.FormatInt(id, 10), map[string]interface{}{"status": req.Status})
	util.OK(c, item)
}

func (h *Handler) AdminSetGalleryGapTaskStatus(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		util.NotFound(c, "缺口任务不存在")
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Status) == "" {
		util.BadRequest(c, "缺口任务状态参数错误")
		return
	}
	if err := h.gallery.SetGalleryGapTaskStatus(c.Request.Context(), id, req.Status, c.GetInt64("admin_id")); err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	h.admin.LogOperation(c.Request.Context(), c.GetInt64("admin_id"), "update_gallery_gap_task", "gallery_gap_task", strconv.FormatInt(id, 10), map[string]interface{}{"status": req.Status})
	util.OK(c, map[string]interface{}{"id": id, "status": req.Status})
}

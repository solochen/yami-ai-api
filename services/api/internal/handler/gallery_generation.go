package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/starai/api/internal/service"
	"github.com/starai/api/internal/util"
)

func (h *Handler) AdminGetGalleryGenerationEligibility(c *gin.Context) {
	item, err := h.galleryGen.GetEligibility(c.Request.Context())
	if err != nil {
		util.InternalError(c, err.Error())
		return
	}
	util.OK(c, item)
}

func (h *Handler) AdminListGalleryGenerationBatches(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	items, total, err := h.galleryGen.ListBatches(c.Request.Context(), c.Query("status"), page, pageSize)
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	util.OK(c, map[string]interface{}{"items": items, "total": total, "page": page, "page_size": pageSize})
}

func (h *Handler) AdminGetGalleryGenerationBatch(c *gin.Context) {
	id, ok := galleryGenerationID(c)
	if !ok {
		util.NotFound(c, "生成批次不存在")
		return
	}
	item, err := h.galleryGen.GetBatch(c.Request.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		util.NotFound(c, "生成批次不存在")
		return
	}
	if err != nil {
		util.InternalError(c, err.Error())
		return
	}
	util.OK(c, item)
}

func (h *Handler) AdminListGalleryGenerationTasks(c *gin.Context) {
	id, ok := galleryGenerationID(c)
	if !ok {
		util.NotFound(c, "生成批次不存在")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "100"))
	items, total, err := h.galleryGen.ListTasks(c.Request.Context(), id, page, pageSize)
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	util.OK(c, map[string]interface{}{"items": items, "total": total, "page": page, "page_size": pageSize})
}

func (h *Handler) AdminCreateGalleryGenerationBatch(c *gin.Context) {
	var req service.CreateGalleryGenerationBatchInput
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "生成批次参数错误")
		return
	}
	item, err := h.galleryGen.CreateBatch(c.Request.Context(), c.GetInt64("admin_id"), req)
	if err != nil {
		if errors.Is(err, service.ErrGalleryGenerationSpecActive) {
			util.Fail(c, http.StatusConflict, http.StatusConflict, err.Error())
			return
		}
		util.BadRequest(c, err.Error())
		return
	}
	h.admin.LogOperation(c.Request.Context(), c.GetInt64("admin_id"), "create_gallery_generation_batch", "gallery_generation_batch", item.PublicID, map[string]interface{}{"spec_count": item.SpecCount, "estimated_cost": item.EstimatedCost})
	util.Created(c, item)
}

func (h *Handler) AdminStartGalleryGenerationBatch(c *gin.Context) {
	id, ok := galleryGenerationID(c)
	if !ok {
		util.NotFound(c, "生成批次不存在")
		return
	}
	item, err := h.galleryGen.StartBatch(c.Request.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		util.NotFound(c, "生成批次不存在")
		return
	}
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	h.admin.LogOperation(c.Request.Context(), c.GetInt64("admin_id"), "start_gallery_generation_batch", "gallery_generation_batch", strconv.FormatInt(id, 10), nil)
	util.OK(c, item)
}

func (h *Handler) AdminCancelGalleryGenerationBatch(c *gin.Context) {
	id, ok := galleryGenerationID(c)
	if !ok {
		util.NotFound(c, "生成批次不存在")
		return
	}
	item, err := h.galleryGen.CancelBatch(c.Request.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		util.NotFound(c, "生成批次不存在")
		return
	}
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	h.admin.LogOperation(c.Request.Context(), c.GetInt64("admin_id"), "cancel_gallery_generation_batch", "gallery_generation_batch", strconv.FormatInt(id, 10), nil)
	util.OK(c, item)
}

func (h *Handler) AdminRetryGalleryGenerationTask(c *gin.Context) {
	id, ok := galleryGenerationID(c)
	if !ok {
		util.NotFound(c, "生成任务不存在")
		return
	}
	item, err := h.galleryGen.RetryTask(c.Request.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		util.NotFound(c, "生成任务不存在")
		return
	}
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	h.admin.LogOperation(c.Request.Context(), c.GetInt64("admin_id"), "retry_gallery_generation_task", "gallery_generation_task", strconv.FormatInt(id, 10), nil)
	util.OK(c, item)
}

func galleryGenerationID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	return id, err == nil && id > 0
}

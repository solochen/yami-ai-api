package handler

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/starai/api/internal/service"
	"github.com/starai/api/internal/util"
)

func (h *Handler) AdminListGalleryQualityReviews(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	items, total, err := h.galleryReview.List(c.Request.Context(), c.Query("status"), page, pageSize)
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	util.OK(c, map[string]interface{}{"items": items, "total": total, "page": page, "page_size": pageSize})
}

func (h *Handler) AdminGetGalleryQualityReview(c *gin.Context) {
	id, ok := galleryQualityReviewID(c)
	if !ok {
		util.NotFound(c, "质量审核记录不存在")
		return
	}
	item, err := h.galleryReview.Get(c.Request.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		util.NotFound(c, "质量审核记录不存在")
		return
	}
	if err != nil {
		util.InternalError(c, err.Error())
		return
	}
	util.OK(c, item)
}

func (h *Handler) AdminAutoCheckGalleryQualityReview(c *gin.Context) {
	id, ok := galleryQualityReviewID(c)
	if !ok {
		util.NotFound(c, "质量审核记录不存在")
		return
	}
	item, err := h.galleryReview.RunAutoCheck(c.Request.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		util.NotFound(c, "质量审核记录不存在")
		return
	}
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	h.admin.LogOperation(c.Request.Context(), c.GetInt64("admin_id"), "auto_check_gallery_quality_review", "gallery_quality_review", strconv.FormatInt(id, 10), map[string]interface{}{"status": item.Status, "summary": item.CheckSummary})
	util.OK(c, item)
}

func (h *Handler) AdminApproveGalleryQualityReview(c *gin.Context) {
	h.adminDecideGalleryQualityReview(c, true)
}

func (h *Handler) AdminRejectGalleryQualityReview(c *gin.Context) {
	h.adminDecideGalleryQualityReview(c, false)
}

func (h *Handler) adminDecideGalleryQualityReview(c *gin.Context, approved bool) {
	id, ok := galleryQualityReviewID(c)
	if !ok {
		util.NotFound(c, "质量审核记录不存在")
		return
	}
	var req service.GalleryQualityReviewDecisionInput
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			util.BadRequest(c, "审核参数错误")
			return
		}
	}
	item, err := h.galleryReview.Decide(c.Request.Context(), id, c.GetInt64("admin_id"), approved, req)
	if errors.Is(err, pgx.ErrNoRows) {
		util.NotFound(c, "质量审核记录不存在")
		return
	}
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	action := "reject_gallery_quality_review"
	if approved {
		action = "approve_gallery_quality_review"
	}
	h.admin.LogOperation(c.Request.Context(), c.GetInt64("admin_id"), action, "gallery_quality_review", strconv.FormatInt(id, 10), map[string]interface{}{"status": item.Status})
	util.OK(c, item)
}

func (h *Handler) AdminPublishGalleryQualityReview(c *gin.Context) {
	id, ok := galleryQualityReviewID(c)
	if !ok {
		util.NotFound(c, "质量审核记录不存在")
		return
	}
	item, err := h.galleryReview.Publish(c.Request.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		util.NotFound(c, "质量审核记录不存在")
		return
	}
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	h.admin.LogOperation(c.Request.Context(), c.GetInt64("admin_id"), "publish_gallery_quality_review", "gallery_quality_review", strconv.FormatInt(id, 10), map[string]interface{}{"status": item.Status, "gallery_item_id": item.GalleryItemID})
	util.OK(c, item)
}

func galleryQualityReviewID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	return id, err == nil && id > 0
}

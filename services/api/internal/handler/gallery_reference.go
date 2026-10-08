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

func (h *Handler) AdminListGalleryReferences(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "30"))
	items, total, err := h.gallery.ListAdminReferences(c.Request.Context(), service.GalleryReferenceAdminFilters{
		Status:        c.Query("status"),
		LicenseStatus: c.Query("license_status"),
		SourceKey:     c.Query("source_key"),
		Category:      c.Query("category"),
		Query:         c.Query("q"),
		Active:        c.Query("active"),
	}, page, pageSize)
	if err != nil {
		if strings.Contains(err.Error(), "无效") {
			util.BadRequest(c, err.Error())
			return
		}
		util.InternalError(c, err.Error())
		return
	}
	util.OK(c, map[string]interface{}{"items": items, "total": total, "page": page, "page_size": pageSize})
}

func (h *Handler) AdminGetGalleryReference(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		util.NotFound(c, "参考案例不存在")
		return
	}
	item, err := h.gallery.GetAdminReference(c.Request.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		util.NotFound(c, "参考案例不存在")
		return
	}
	if err != nil {
		util.InternalError(c, err.Error())
		return
	}
	util.OK(c, item)
}

func (h *Handler) AdminUpdateGalleryReference(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		util.NotFound(c, "参考案例不存在")
		return
	}
	var req service.GalleryReferenceUpdateInput
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "参考案例参数错误")
		return
	}
	item, err := h.gallery.UpdateAdminReference(c.Request.Context(), id, req)
	if errors.Is(err, pgx.ErrNoRows) {
		util.NotFound(c, "参考案例不存在")
		return
	}
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	h.admin.LogOperation(c.Request.Context(), c.GetInt64("admin_id"), "update_gallery_reference", "gallery_reference", c.Param("id"), map[string]interface{}{
		"status": req.Status, "asset_license_status": req.AssetLicenseStatus, "featured": req.Featured,
	})
	util.OK(c, item)
}

func (h *Handler) AdminBatchGalleryReferences(c *gin.Context) {
	var req struct {
		IDs    []int64 `json:"ids"`
		Action string  `json:"action"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "批量操作参数错误")
		return
	}
	result, err := h.gallery.BatchUpdateAdminReferences(c.Request.Context(), req.IDs, strings.TrimSpace(req.Action))
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	h.admin.LogOperation(c.Request.Context(), c.GetInt64("admin_id"), "batch_gallery_reference", "gallery_reference", "batch", map[string]interface{}{
		"action": result.Action, "count": result.Updated, "ids": result.IDs,
	})
	util.OK(c, result)
}

func (h *Handler) AdminImportGalleryReferences(c *gin.Context) {
	var req struct {
		SourceKey     string                               `json:"source_key"`
		RepositoryURL string                               `json:"repository_url"`
		ManifestURL   string                               `json:"manifest_url"`
		LicenseName   string                               `json:"license_name"`
		LicenseURL    string                               `json:"license_url"`
		Attribution   string                               `json:"attribution"`
		Cases         []service.GalleryReferenceImportCase `json:"cases"`
		Manifest      *struct {
			Cases []service.GalleryReferenceImportCase `json:"cases"`
		} `json:"manifest"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "导入 JSON 格式错误")
		return
	}
	if len(req.Cases) == 0 && req.Manifest != nil {
		req.Cases = req.Manifest.Cases
	}
	count, err := h.gallery.ImportAdminReferences(c.Request.Context(), service.GalleryReferenceImportInput{
		SourceKey: req.SourceKey, RepositoryURL: req.RepositoryURL, ManifestURL: req.ManifestURL,
		LicenseName: req.LicenseName, LicenseURL: req.LicenseURL, Attribution: req.Attribution, Cases: req.Cases,
	})
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	h.admin.LogOperation(c.Request.Context(), c.GetInt64("admin_id"), "import_gallery_references", "gallery_reference", req.SourceKey, map[string]interface{}{"count": count})
	util.OK(c, map[string]interface{}{"imported": count, "source_key": req.SourceKey})
}

func (h *Handler) AdminGalleryReferenceCoverage(c *gin.Context) {
	target, _ := strconv.Atoi(c.DefaultQuery("target", "8"))
	coverage, err := h.gallery.GalleryReferenceCoverage(c.Request.Context(), target)
	if err != nil {
		util.InternalError(c, err.Error())
		return
	}
	util.OK(c, coverage)
}

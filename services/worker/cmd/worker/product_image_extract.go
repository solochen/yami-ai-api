package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/starai/worker/internal/jdproduct"
)

func processProductImageExtractWorkflow(ctx context.Context, pool *pgxpool.Pool, p WorkflowTaskPayload, publicID string, estimated float64, inputs map[string]interface{}) error {
	pageURL := strings.TrimSpace(stringAny(inputs["product_url"]))
	sku := strings.TrimSpace(firstNonEmpty(stringAny(inputs["sku_id"]), stringAny(inputs["product_id"])))
	platform := strings.TrimSpace(stringAny(inputs["platform"]))
	if pageURL == "" || sku == "" {
		return failWorkflow(ctx, pool, p, publicID, estimated, "请输入京东商品链接")
	}
	if platform != "" && platform != "jd" {
		return failWorkflow(ctx, pool, p, publicID, estimated, "仅支持京东商品链接")
	}
	outputs := loadWorkflowOutputs(ctx, pool, p.ProjectID)
	outputs["current_step"] = "fetch"
	outputs["sku_id"] = sku
	outputs["platform"] = "jd"
	outputs["source_url"] = pageURL
	saveWorkflowOutputs(ctx, pool, p.ProjectID, outputs)

	nodeRunID := insertWorkflowNodeRun(ctx, pool, p.ProjectID, "extract", "提取商品图片", "tool", map[string]interface{}{
		"product_url": pageURL,
		"sku_id":      sku,
		"platform":    "jd",
	}, 0)
	started := time.Now()
	product, err := jdproduct.Load(ctx, sku)
	if err != nil || len(product.Images) == 0 {
		message := productExtractError(err)
		pool.Exec(ctx, `UPDATE workflow_node_runs SET status='failed', error=$1, duration_ms=$2 WHERE id=$3`, message, time.Since(started).Milliseconds(), nodeRunID)
		return failWorkflow(ctx, pool, p, publicID, estimated, message)
	}
	if strings.TrimSpace(product.SKU) == "" {
		product.SKU = sku
	}
	if strings.TrimSpace(product.Title) == "" {
		product.Title = "京东商品 " + sku
	}
	if objectStore == nil {
		message := "对象存储未配置，无法保存商品图片"
		pool.Exec(ctx, `UPDATE workflow_node_runs SET status='failed', error=$1, duration_ms=$2 WHERE id=$3`, message, time.Since(started).Milliseconds(), nodeRunID)
		return failWorkflow(ctx, pool, p, publicID, estimated, message)
	}
	outputs["current_step"] = "downloading"
	outputs["title"] = product.Title
	outputs["sale_specs"] = product.SaleSpecs
	outputs["attributes"] = product.Attributes
	outputs["progress"] = map[string]int{"done": 0, "total": len(product.Images)}
	saveWorkflowOutputs(ctx, pool, p.ProjectID, outputs)

	downloaded, err := jdproduct.Download(ctx, product.Images)
	if err != nil {
		message := productExtractError(err)
		pool.Exec(ctx, `UPDATE workflow_node_runs SET status='failed', error=$1, duration_ms=$2 WHERE id=$3`, message, time.Since(started).Milliseconds(), nodeRunID)
		return failWorkflow(ctx, pool, p, publicID, estimated, message)
	}
	stored, zipURL := storeProductImages(ctx, pool, p.UserID, publicID, product.Title, downloaded)
	if len(stored) == 0 {
		message := jdproduct.ErrPageUnavailable.Error()
		pool.Exec(ctx, `UPDATE workflow_node_runs SET status='failed', error=$1, duration_ms=$2 WHERE id=$3`, message, time.Since(started).Milliseconds(), nodeRunID)
		return failWorkflow(ctx, pool, p, publicID, estimated, message)
	}
	outputs["current_step"] = "result"
	outputs["sku_id"] = product.SKU
	outputs["title"] = product.Title
	outputs["sale_specs"] = product.SaleSpecs
	outputs["attributes"] = product.Attributes
	outputs["images"] = stored
	outputs["image_count"] = len(stored)
	outputs["zip_url"] = zipURL
	outputs["progress"] = map[string]int{"done": len(stored), "total": len(product.Images)}
	saveWorkflowOutputs(ctx, pool, p.ProjectID, outputs)
	updateNodeRunSuccess(ctx, pool, nodeRunID, map[string]interface{}{
		"title":       product.Title,
		"image_count": len(stored),
		"sku_id":      product.SKU,
	}, 0, int(time.Since(started).Milliseconds()))
	return completeSimpleAgentWorkflow(ctx, pool, p, publicID, estimated, outputs)
}

func productExtractError(err error) string {
	if err == nil {
		return jdproduct.ErrPageUnavailable.Error()
	}
	if errors.Is(err, jdproduct.ErrBrowserMissing) || errors.Is(err, jdproduct.ErrPageUnavailable) {
		return err.Error()
	}
	return jdproduct.ErrPageUnavailable.Error()
}

func storeProductImages(ctx context.Context, pool *pgxpool.Pool, userID int64, publicID, title string, images []jdproduct.DownloadedImage) ([]map[string]interface{}, string) {
	bucket := strings.TrimSpace(os.Getenv("MINIO_BUCKET"))
	if bucket == "" {
		bucket = "starai-works"
	}
	stored := make([]map[string]interface{}, 0, len(images))
	zipBuffer := &bytes.Buffer{}
	zipper := zip.NewWriter(zipBuffer)
	usedNames := map[string]int{}
	for _, image := range images {
		ext := imageExt(image.ContentType)
		label := "详情图"
		if image.Role == "gallery" {
			label = "主图"
		}
		base := fmt.Sprintf("%s-%02d%s", label, image.Index, ext)
		if image.Spec != "" {
			base = fmt.Sprintf("%s-%02d-%s%s", label, image.Index, sanitizeFileName(image.Spec), ext)
		}
		name := uniqueFileName(usedNames, base)
		assetID := newExtractID("ast")
		objectName := fmt.Sprintf("assets/%d/%s/%s", userID, assetID, name)
		assetURL, err := objectStore.Upload(ctx, objectName, image.ContentType, bytes.NewReader(image.Body), int64(len(image.Body)))
		if err != nil {
			continue
		}
		mime := image.ContentType
		displayName := strings.TrimSuffix(name, ext)
		_, _ = pool.Exec(ctx, `
			INSERT INTO assets (public_id, user_id, bucket, object_key, name, description, kind, asset_type, mime_type, size_bytes, tags)
			VALUES ($1,$2,$3,$4,$5,$6,'image','prop',$7,$8,$9)`,
			assetID, userID, bucket, objectName, displayName, title, mime, len(image.Body), mustJSON([]string{"京东商品", label}))
		if entry, zipErr := zipper.Create(name); zipErr == nil {
			_, _ = entry.Write(image.Body)
		}
		stored = append(stored, map[string]interface{}{
			"role":       image.Role,
			"spec":       image.Spec,
			"index":      image.Index,
			"source_url": image.SourceURL,
			"asset_url":  assetURL,
			"asset_id":   assetID,
			"name":       displayName,
		})
	}
	if err := zipper.Close(); err != nil || len(stored) == 0 || zipBuffer.Len() == 0 {
		return stored, ""
	}
	zipID := newExtractID("zip")
	zipName := fmt.Sprintf("works/product-extract/%s/%s.zip", publicID, zipID)
	zipURL, err := objectStore.Upload(ctx, zipName, "application/zip", bytes.NewReader(zipBuffer.Bytes()), int64(zipBuffer.Len()))
	if err != nil {
		return stored, ""
	}
	return stored, zipURL
}

func imageExt(contentType string) string {
	switch {
	case strings.Contains(contentType, "png"):
		return ".png"
	case strings.Contains(contentType, "webp"):
		return ".webp"
	case strings.Contains(contentType, "gif"):
		return ".gif"
	case strings.Contains(contentType, "avif"):
		return ".avif"
	default:
		return ".jpg"
	}
}

func sanitizeFileName(value string) string {
	value = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`/\?%*:|"<>`, r) || r < 32 {
			return '-'
		}
		return r
	}, strings.TrimSpace(value))
	if len([]rune(value)) > 24 {
		value = string([]rune(value)[:24])
	}
	if value == "" {
		return "规格"
	}
	return value
}

func uniqueFileName(used map[string]int, name string) string {
	if used[name] == 0 {
		used[name] = 1
		return name
	}
	used[name]++
	dot := strings.LastIndex(name, ".")
	if dot <= 0 {
		return fmt.Sprintf("%s-%d", name, used[name])
	}
	return fmt.Sprintf("%s-%d%s", name[:dot], used[name], name[dot:])
}

func newExtractID(prefix string) string {
	buf := make([]byte, 6)
	_, _ = rand.Read(buf)
	return prefix + "_" + hex.EncodeToString(buf)
}

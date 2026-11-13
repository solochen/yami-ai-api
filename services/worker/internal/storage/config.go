package storage

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
	Provider  string
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	PublicURL string
	UseSSL    bool

	// Region 用于 S3 V4 签名（例如 ap-hongkong）。留空时 minio-go 会调用
	// GetBucketLocation 自动探测，COS 上该调用需要额外的桶级权限。
	Region string
	// PublicURLStyle 决定公开地址是否带 bucket 前缀：
	// "bucket"（默认）→ <域名>/<bucket>/<key>；"object" → <域名>/<key>。
	PublicURLStyle string
	// Private 为 true 时不写入“公有读”桶策略，适配私有读桶（如 COS 私有桶）。
	Private bool
	// SkipBucketInit 为 true 时不检查/创建桶，适用于在控制台建桶的对象存储。
	SkipBucketInit bool
	// BucketLookup 控制 S3 请求风格："auto"（默认）、"path"、"dns"（虚拟主机风格）。
	// COS 默认走 path；若返回 SignatureDoesNotMatch 可尝试 "dns"。
	BucketLookup string
}

func LoadConfig(ctx context.Context, db *pgxpool.Pool, fallback Config) Config {
	if db == nil {
		return fallback
	}
	rows, err := db.Query(ctx, `SELECT key, value FROM system_configs WHERE key = ANY($1)`, []string{
		"storage_provider",
		"storage_endpoint",
		"storage_access_key",
		"storage_secret_key",
		"storage_bucket",
		"storage_public_url",
		"storage_use_ssl",
	})
	if err != nil {
		return fallback
	}
	defer rows.Close()
	cfg := fallback
	for rows.Next() {
		var key string
		var raw []byte
		if err := rows.Scan(&key, &raw); err != nil {
			continue
		}
		var v interface{}
		if err := json.Unmarshal(raw, &v); err != nil {
			continue
		}
		switch key {
		case "storage_provider":
			if s := stringValue(v); s != "" {
				cfg.Provider = s
			}
		case "storage_endpoint":
			if s := stringValue(v); s != "" {
				cfg.Endpoint = normalizeEndpoint(s)
			}
		case "storage_access_key":
			if s := stringValue(v); s != "" {
				cfg.AccessKey = s
			}
		case "storage_secret_key":
			if s := stringValue(v); s != "" {
				cfg.SecretKey = s
			}
		case "storage_bucket":
			if s := stringValue(v); s != "" {
				cfg.Bucket = s
			}
		case "storage_public_url":
			cfg.PublicURL = strings.TrimRight(stringValue(v), "/")
		case "storage_use_ssl":
			if b, ok := v.(bool); ok {
				cfg.UseSSL = b
			}
		}
	}
	return cfg
}

func stringValue(v interface{}) string {
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

func normalizeEndpoint(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	return strings.TrimRight(s, "/")
}

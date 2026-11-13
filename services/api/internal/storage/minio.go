package storage

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// 公开访问地址的拼接方式。
//
//   - URLStyleBucket：<公开域名>/<bucket>/<key>。MinIO、Cloudflare R2 等
//     “直连桶”的部署沿用该格式，保持历史数据兼容。
//   - URLStyleObject：<公开域名>/<key>。腾讯云 COS 的自定义 CDN 加速域名、
//     自定义源站域名都直接映射到对象，路径里不能再带 bucket 名。
const (
	URLStyleBucket = "bucket"
	URLStyleObject = "object"
)

// Client wraps a MinIO/S3-compatible object store for user uploads.
type Client struct {
	mc        *minio.Client
	bucket    string
	publicURL string
	urlStyle  string
	lookup    minio.BucketLookupType
}

// New connects to MinIO/S3-compatible storage.
//
// SkipBucketInit 为 true 时不调用 BucketExists / MakeBucket，适用于 COS 这类
// 在控制台建桶、且子账号没有桶级权限的场景；Private 为 true 时不再写入
// “公有读”桶策略（COS 私有桶不需要，也没有该接口权限）。
func New(cfg Config) (*Client, error) {
	style := strings.ToLower(strings.TrimSpace(cfg.PublicURLStyle))
	if style != URLStyleObject {
		style = URLStyleBucket
	}

	lookup := bucketLookupType(cfg.BucketLookup, cfg.Endpoint)
	mc, err := minio.New(normalizeEndpoint(cfg.Endpoint), &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		// 显式指定地域可以跳过 GetBucketLocation（COS 上该接口需要额外的桶级权限）。
		Region:       strings.TrimSpace(cfg.Region),
		BucketLookup: lookup,
	})
	if err != nil {
		return nil, err
	}

	if !cfg.SkipBucketInit {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		exists, err := mc.BucketExists(ctx, cfg.Bucket)
		if err != nil {
			return nil, err
		}
		if !exists {
			if err := mc.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{Region: strings.TrimSpace(cfg.Region)}); err != nil {
				return nil, err
			}
		}
	}

	if !cfg.Private {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		policy := fmt.Sprintf(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["*"]},"Action":["s3:GetObject"],"Resource":["arn:aws:s3:::%s/*"]}]}`, cfg.Bucket)
		_ = mc.SetBucketPolicy(ctx, cfg.Bucket, policy)
	}

	publicURL := strings.TrimRight(strings.TrimSpace(cfg.PublicURL), "/")
	if publicURL == "" {
		scheme := "http"
		if cfg.UseSSL {
			scheme = "https"
		}
		publicURL = fmt.Sprintf("%s://%s", scheme, normalizeEndpoint(cfg.Endpoint))
	}

	return &Client{mc: mc, bucket: cfg.Bucket, publicURL: publicURL, urlStyle: style, lookup: lookup}, nil
}

// Upload stores an object and returns its public URL.
func (c *Client) Upload(ctx context.Context, objectName, contentType string, r io.Reader, size int64) (string, error) {
	_, err := c.mc.PutObject(ctx, c.bucket, objectName, r, size, minio.PutObjectOptions{
		ContentType:        contentType,
		ContentDisposition: "inline",
		CacheControl:       "public, max-age=86400, stale-while-revalidate=604800",
	})
	if err != nil {
		return "", err
	}
	return c.PublicURL(objectName), nil
}

// ReadAll reads an object from storage, bounded by maxBytes.
func (c *Client) ReadAll(ctx context.Context, objectName string, maxBytes int64) ([]byte, error) {
	obj, err := c.mc.GetObject(ctx, c.bucket, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer obj.Close()
	return io.ReadAll(io.LimitReader(obj, maxBytes+1))
}

// Delete removes an object from storage.
func (c *Client) Delete(ctx context.Context, objectName string) error {
	return c.mc.RemoveObject(ctx, c.bucket, objectName, minio.RemoveObjectOptions{})
}

// ObjectKeyFromURL 从公开地址反查对象 key。
//
// 同时兼容三种历史/当前形态：
//   - <公开域名>/<bucket>/<key>（MinIO 直连、旧的 /media 代理绝对地址）
//   - <公开域名>/<key>（COS 自定义 CDN / 源站域名）
//   - 相对路径 /media/<bucket>/<key> 与 <bucket>/<key>
func (c *Client) ObjectKeyFromURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	publicBase, _ := url.Parse(strings.TrimRight(c.publicURL, "/"))
	basePath := ""
	if publicBase != nil {
		basePath = strings.Trim(strings.TrimPrefix(publicBase.Path, "/"), "/")
	}
	hostMatches := publicBase != nil && publicBase.Host != "" && strings.EqualFold(u.Host, publicBase.Host)

	path := strings.TrimPrefix(u.Path, "/")
	// 相对媒体路径（/media/<bucket>/<key>，由 web/admin 代理到对象存储）也接受。
	// 只对相对地址剥离 media/ 前缀，避免把真实 key 以 media/ 开头的对象截断。
	if u.Host == "" && strings.HasPrefix(path, "media/") {
		path = strings.TrimPrefix(path, "media/")
	}

	if u.Host != "" && !hostMatches {
		// 异源绝对地址一律拒绝：外部 CDN/图床地址不能被当成站内对象去读取或删除。
		return ""
	}

	if basePath != "" {
		path = strings.TrimPrefix(path, basePath+"/")
	}
	path = strings.TrimPrefix(path, c.bucket+"/")
	return strings.TrimPrefix(path, "/")
}

// PublicURL 返回对象的公开访问地址；CDN 形态下不带 bucket 前缀。
func (c *Client) PublicURL(objectName string) string {
	return strings.TrimRight(c.publicURL, "/") + "/" + encodeObjectPath(c.objectPath(objectName))
}

// URLStyle 返回当前的公开地址拼接方式，便于启动日志与自检命令输出。
func (c *Client) URLStyle() string {
	return c.urlStyle
}

// LookupStyle 返回实际生效的 S3 请求风格（path / dns / auto）。
func (c *Client) LookupStyle() string {
	switch c.lookup {
	case minio.BucketLookupDNS:
		return "dns"
	case minio.BucketLookupPath:
		return "path"
	default:
		return "auto"
	}
}

func (c *Client) objectPath(objectName string) string {
	key := strings.TrimLeft(strings.ReplaceAll(objectName, "\\", "/"), "/")
	if c.urlStyle == URLStyleObject {
		return key
	}
	return c.bucket + "/" + key
}

func bucketLookupType(value, endpoint string) minio.BucketLookupType {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "path", "path-style":
		return minio.BucketLookupPath
	case "dns", "virtual", "virtual-host", "virtual-hosted":
		return minio.BucketLookupDNS
	}
	// 腾讯云 COS 不接受 path-style（会返回
	// "must be addressed using COS virtual-styled domain"），
	// *.myqcloud.com 默认走虚拟主机风格 <bucket>.cos.<region>.myqcloud.com。
	if isTencentCOSEndpoint(endpoint) {
		return minio.BucketLookupDNS
	}
	return minio.BucketLookupAuto
}

func isTencentCOSEndpoint(endpoint string) bool {
	host := strings.ToLower(normalizeEndpoint(endpoint))
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return host == "myqcloud.com" || strings.HasSuffix(host, ".myqcloud.com")
}

func encodeObjectPath(path string) string {
	parts := strings.Split(path, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}

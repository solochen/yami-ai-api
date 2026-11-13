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

func (c *Client) Upload(ctx context.Context, objectName, contentType string, r io.Reader, size int64) (string, error) {
	_, err := c.mc.PutObject(ctx, c.bucket, objectName, r, size, minio.PutObjectOptions{
		ContentType:        contentType,
		ContentDisposition: "inline",
	})
	if err != nil {
		return "", err
	}
	return c.PublicURL(objectName), nil
}

func (c *Client) ReadAll(ctx context.Context, objectName string, maxBytes int64) ([]byte, error) {
	object, err := c.mc.GetObject(ctx, c.bucket, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer object.Close()
	data, err := io.ReadAll(io.LimitReader(object, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("object exceeds size limit")
	}
	return data, nil
}

// ObjectKeyFromURL 从公开地址反查对象 key。
//
// 同时兼容三种历史/当前形态：
//   - <公开域名>/<bucket>/<key>（MinIO 直连、旧的 /media 代理绝对地址）
//   - <公开域名>/<key>（COS 自定义 CDN / 源站域名）
//   - 相对路径 /media/<bucket>/<key> 与 <bucket>/<key>
func (c *Client) ObjectKeyFromURL(rawURL string) string {
	raw := strings.TrimSpace(rawURL)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	base, _ := url.Parse(strings.TrimRight(c.publicURL, "/"))
	basePath := ""
	if base != nil {
		basePath = strings.Trim(strings.TrimPrefix(base.Path, "/"), "/")
	}
	hostMatches := base != nil && base.Host != "" && strings.EqualFold(u.Host, base.Host)

	path := strings.TrimPrefix(u.Path, "/")
	// 相对媒体地址（/media/<bucket>/<key>）同样要能反查对象 key，
	// 否则上传素材在 worker 侧读取失败，参考图会整批丢失。
	if strings.HasPrefix(path, "media/") {
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
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		return ""
	}
	return path
}

// PublicURL 返回对象的公开访问地址；CDN 形态下不带 bucket 前缀。
func (c *Client) PublicURL(objectName string) string {
	return strings.TrimRight(c.publicURL, "/") + "/" + encodeObjectPath(c.objectPath(objectName))
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

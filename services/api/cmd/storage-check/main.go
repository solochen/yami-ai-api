// Command storage-check 在不启动整套服务的情况下验证对象存储配置：
// 连接、签名、上传、CDN 公网读取、删除。用于 COS + 自定义 CDN 域名上线前的自检。
//
//	docker compose -f infra/docker/docker-compose.prod.yml --env-file .env.production run --rm api /storage-check
//	go run ./cmd/storage-check
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/starai/api/internal/config"
	"github.com/starai/api/internal/storage"
)

// 1x1 透明 PNG，用来做最小化上传探测。
var probePNG, _ = base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==")

func main() {
	_ = godotenv.Load("../../.env.local", "../../.env", ".env.local", ".env")

	keep := flag.Bool("keep", false, "保留探测对象，便于在浏览器/控制台复查")
	noDB := flag.Bool("no-db", false, "忽略数据库 system_configs，只用环境变量（切换存储前预演）")
	timeout := flag.Duration("timeout", 60*time.Second, "整体超时时间")
	flag.Parse()

	if err := run(*keep, *noDB, *timeout); err != nil {
		fmt.Fprintf(os.Stderr, "\n❌ 自检失败：%v\n", err)
		os.Exit(1)
	}
}

func run(keep, noDB bool, timeout time.Duration) error {
	cfg := config.Load()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	storageCfg := storage.Config{
		Provider:       "minio",
		Endpoint:       cfg.MinioEndpoint,
		AccessKey:      cfg.MinioAccessKey,
		SecretKey:      cfg.MinioSecretKey,
		Bucket:         cfg.MinioBucket,
		PublicURL:      cfg.MinioPublicURL,
		UseSSL:         cfg.MinioUseSSL,
		Region:         cfg.MinioRegion,
		PublicURLStyle: cfg.MinioPublicURLStyle,
		Private:        cfg.MinioBucketPrivate,
		SkipBucketInit: cfg.MinioSkipBucketInit,
		BucketLookup:   cfg.MinioBucketLookup,
	}
	// 后台“系统配置 > 存储”里的值会覆盖环境变量，自检必须使用同一份数据。
	if noDB {
		fmt.Println("· 已指定 -no-db，只使用环境变量")
	} else if pool, err := pgxpool.New(ctx, cfg.DatabaseURL); err == nil {
		defer pool.Close()
		pingCtx, cancelPing := context.WithTimeout(ctx, 5*time.Second)
		pingErr := pool.Ping(pingCtx)
		cancelPing()
		if pingErr != nil {
			fmt.Printf("· 数据库不可用，仅使用环境变量：%v\n", pingErr)
		} else {
			storageCfg = storage.LoadConfig(ctx, pool, storageCfg)
			fmt.Println("· 已读取数据库 system_configs（存储配置以数据库为准）")
		}
	} else {
		fmt.Printf("· 未连接数据库，仅使用环境变量：%v\n", err)
	}

	fmt.Println("\n=== 当前存储配置 ===")
	fmt.Printf("  Endpoint        : %s\n", storageCfg.Endpoint)
	fmt.Printf("  Region          : %s\n", orDash(storageCfg.Region))
	fmt.Printf("  Bucket          : %s\n", storageCfg.Bucket)
	fmt.Printf("  UseSSL          : %v\n", storageCfg.UseSSL)
	fmt.Printf("  PublicURL       : %s\n", storageCfg.PublicURL)
	fmt.Printf("  PublicURLStyle  : %s（object = 自定义 CDN/源站域名，不带 bucket 前缀）\n", orDash(storageCfg.PublicURLStyle))
	fmt.Printf("  Private         : %v\n", storageCfg.Private)
	fmt.Printf("  SkipBucketInit  : %v\n", storageCfg.SkipBucketInit)
	fmt.Printf("  BucketLookup    : %s\n", orDash(storageCfg.BucketLookup))
	fmt.Printf("  AccessKey       : %s\n", mask(storageCfg.AccessKey))

	client, err := storage.New(storageCfg)
	if err != nil {
		return fmt.Errorf("初始化存储客户端失败（检查 Endpoint/密钥/Bucket/Region）：%w", err)
	}
	fmt.Println("\n✅ 存储客户端初始化成功")
	fmt.Printf("   地址风格 = %s（bucket = 带桶前缀，object = CDN/源站域名）\n", client.URLStyle())
	fmt.Printf("   请求风格 = %s（COS 域名会自动使用 dns/虚拟主机风格）\n", client.LookupStyle())

	objectName := fmt.Sprintf("healthcheck/storage-check-%d.png", time.Now().UnixNano())
	publicURL, err := client.Upload(ctx, objectName, "image/png", bytes.NewReader(probePNG), int64(len(probePNG)))
	if err != nil {
		return fmt.Errorf("上传失败（检查密钥权限、Bucket 是否存在、Region 是否正确）：%w", err)
	}
	fmt.Printf("✅ 上传成功\n   object = %s\n   url    = %s\n", objectName, publicURL)

	data, err := client.ReadAll(ctx, objectName, 1<<20)
	if err != nil {
		return fmt.Errorf("读回对象失败（检查 GetObject 权限）：%w", err)
	}
	if !bytes.Equal(data, probePNG) {
		return fmt.Errorf("读回内容与上传内容不一致（%d vs %d 字节）", len(data), len(probePNG))
	}
	fmt.Println("✅ 服务端读回一致（API/Worker 读写链路可用）")

	if err := checkPublicURL(ctx, publicURL); err != nil {
		fmt.Printf("⚠️  公网地址暂不可用：%v\n", err)
		fmt.Println("   上传/读取已经可用；公网访问请检查：CDN 域名 CNAME、回源鉴权、HTTPS 证书、对象是否已同步到 COS。")
	} else {
		fmt.Println("✅ 公网地址可访问（浏览器可以直接加载）")
	}

	if keep {
		fmt.Printf("\n已保留探测对象：%s\n", publicURL)
		return nil
	}
	if err := client.Delete(ctx, objectName); err != nil {
		return fmt.Errorf("删除探测对象失败（检查 DeleteObject 权限）：%w", err)
	}
	fmt.Println("✅ 删除成功，自检结束")
	return nil
}

func checkPublicURL(ctx context.Context, rawURL string) error {
	if strings.HasPrefix(rawURL, "/") {
		return fmt.Errorf("公开地址是相对路径 %q，后端无法直接探测；请在浏览器里访问站点确认", rawURL)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Cache-Control", "no-cache")
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<10))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HEAD %s 返回 %d", rawURL, resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "" && !strings.HasPrefix(ct, "image/") {
		return fmt.Errorf("Content-Type 为 %q，期望 image/*", ct)
	}
	return nil
}

func mask(v string) string {
	if len(v) <= 6 {
		return "***"
	}
	return v[:4] + "***" + v[len(v)-2:]
}

func orDash(v string) string {
	if strings.TrimSpace(v) == "" {
		return "(空)"
	}
	return v
}

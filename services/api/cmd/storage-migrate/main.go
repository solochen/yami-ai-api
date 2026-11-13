// Command storage-migrate 把源对象存储里的对象按原 key 复制到目标对象存储。
//
// 设计目标：
//   - 只列源桶（源端通常有完整权限），目标端只做 HeadObject/PutObject，
//     不依赖目标的 ListBucket 权限（COS 子账号通常没有）。
//   - 支持 dry-run、并发、跳过已存在同大小对象、独立校验。
//   - 不改变 key，迁移后数据库只需替换 URL 前缀。
//
// 典型用法（在 api 容器里，DST 默认取 MINIO_* 环境变量）：
//
//	SRC_ENDPOINT=minio:9000 SRC_ACCESS_KEY=... SRC_SECRET_KEY=... SRC_BUCKET=starai-works \
//	/storage-migrate -dry-run
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type endpoint struct {
	name      string
	endpoint  string
	accessKey string
	secretKey string
	bucket    string
	region    string
	lookup    string
	secure    bool
}

type objectJob struct {
	key         string
	size        int64
	contentType string
}

func main() {
	prefix := flag.String("prefix", "", "只迁移指定 key 前缀")
	dryRun := flag.Bool("dry-run", false, "只列出待迁移对象，不写入目标")
	verify := flag.Bool("verify", false, "只校验目标是否已有同大小对象，不写入")
	concurrency := flag.Int("concurrency", 8, "并发数")
	skipExisting := flag.Bool("skip-existing", true, "目标已存在同大小对象时跳过")
	timeout := flag.Duration("timeout", 30*time.Minute, "整体超时时间")
	flag.Parse()

	src := endpoint{
		name:      "src",
		endpoint:  env("SRC_ENDPOINT", ""),
		accessKey: env("SRC_ACCESS_KEY", ""),
		secretKey: env("SRC_SECRET_KEY", ""),
		bucket:    env("SRC_BUCKET", ""),
		region:    env("SRC_REGION", ""),
		lookup:    env("SRC_LOOKUP", "auto"),
		secure:    envBool("SRC_USE_SSL", false),
	}
	dst := endpoint{
		name:      "dst",
		endpoint:  env("DST_ENDPOINT", os.Getenv("MINIO_ENDPOINT")),
		accessKey: env("DST_ACCESS_KEY", os.Getenv("MINIO_ACCESS_KEY")),
		secretKey: env("DST_SECRET_KEY", os.Getenv("MINIO_SECRET_KEY")),
		bucket:    env("DST_BUCKET", os.Getenv("MINIO_BUCKET")),
		region:    env("DST_REGION", os.Getenv("MINIO_REGION")),
		lookup:    env("DST_LOOKUP", os.Getenv("MINIO_BUCKET_LOOKUP")),
		secure:    envBool("DST_USE_SSL", os.Getenv("MINIO_USE_SSL") == "true"),
	}
	for _, ep := range []endpoint{src, dst} {
		if ep.endpoint == "" || ep.accessKey == "" || ep.secretKey == "" || ep.bucket == "" {
			log.Fatalf("%s 配置不完整：需要 %s_ENDPOINT / %s_ACCESS_KEY / %s_SECRET_KEY / %s_BUCKET",
				ep.name, strings.ToUpper(ep.name), strings.ToUpper(ep.name), strings.ToUpper(ep.name), strings.ToUpper(ep.name))
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	srcClient, err := newClient(src)
	if err != nil {
		log.Fatalf("源存储初始化失败: %v", err)
	}
	dstClient, err := newClient(dst)
	if err != nil {
		log.Fatalf("目标存储初始化失败: %v", err)
	}

	log.Printf("源  : %s bucket=%s ssl=%v lookup=%s", src.endpoint, src.bucket, src.secure, src.lookup)
	log.Printf("目标: %s bucket=%s ssl=%v lookup=%s", dst.endpoint, dst.bucket, dst.secure, dst.lookup)

	jobs, totalBytes, err := listObjects(ctx, srcClient, src.bucket, *prefix)
	if err != nil {
		log.Fatalf("列举源对象失败: %v", err)
	}
	log.Printf("源对象数=%d 总大小=%.1f MB", len(jobs), float64(totalBytes)/1024/1024)

	if *dryRun {
		for i, job := range jobs {
			if i < 20 {
				log.Printf("  %s (%d bytes, %s)", job.key, job.size, job.contentType)
			}
		}
		if len(jobs) > 20 {
			log.Printf("  ... 其余 %d 个对象省略", len(jobs)-20)
		}
		log.Printf("DRY RUN：以上对象将被复制到 %s/%s", dst.bucket, dst.endpoint)
		return
	}

	if *verify {
		missing, mismatched, err := verifyObjects(ctx, dstClient, dst.bucket, jobs, *concurrency)
		if err != nil {
			log.Fatalf("校验失败: %v", err)
		}
		log.Printf("校验完成：缺失=%d 大小不一致=%d 正常=%d", missing, mismatched, len(jobs)-missing-mismatched)
		if missing > 0 || mismatched > 0 {
			os.Exit(1)
		}
		return
	}

	var copied, skipped, failed int64
	var copiedBytes int64
	var failures []string
	var failMu sync.Mutex
	sem := make(chan struct{}, *concurrency)
	var wg sync.WaitGroup

	start := time.Now()
	for _, job := range jobs {
		wg.Add(1)
		sem <- struct{}{}
		go func(job objectJob) {
			defer wg.Done()
			defer func() { <-sem }()

			if *skipExisting {
				if info, err := dstClient.StatObject(ctx, dst.bucket, job.key, minio.StatObjectOptions{}); err == nil && info.Size == job.size {
					atomic.AddInt64(&skipped, 1)
					return
				}
			}

			// ListObjects 不一定返回 Content-Type，用 StatObject 拿准确值；
			// 拿不到时按扩展名兜底，避免 COS 里对象变成 application/octet-stream。
			contentType := job.contentType
			size := job.size
			if srcInfo, statErr := srcClient.StatObject(ctx, src.bucket, job.key, minio.StatObjectOptions{}); statErr == nil {
				if srcInfo.ContentType != "" {
					contentType = srcInfo.ContentType
				}
				if srcInfo.Size > 0 {
					size = srcInfo.Size
				}
			}
			if contentType == "" {
				contentType = mime.TypeByExtension(strings.ToLower(filepath.Ext(job.key)))
			}
			if contentType == "" {
				contentType = "application/octet-stream"
			}

			obj, err := srcClient.GetObject(ctx, src.bucket, job.key, minio.GetObjectOptions{})
			if err != nil {
				recordFailure(&failures, &failMu, job.key, err)
				atomic.AddInt64(&failed, 1)
				return
			}
			defer obj.Close()

			info, err := dstClient.PutObject(ctx, dst.bucket, job.key, obj, size, minio.PutObjectOptions{
				ContentType: contentType,
			})
			if err != nil {
				recordFailure(&failures, &failMu, job.key, err)
				atomic.AddInt64(&failed, 1)
				return
			}
			atomic.AddInt64(&copied, 1)
			atomic.AddInt64(&copiedBytes, info.Size)
			if done := atomic.LoadInt64(&copied) + atomic.LoadInt64(&skipped); done%50 == 0 {
				log.Printf("进度：已复制=%d 跳过=%d 失败=%d", atomic.LoadInt64(&copied), atomic.LoadInt64(&skipped), atomic.LoadInt64(&failed))
			}
		}(job)
	}
	wg.Wait()

	log.Printf("迁移完成：复制=%d 跳过=%d 失败=%d 写入=%.1f MB 耗时=%s",
		copied, skipped, failed, float64(copiedBytes)/1024/1024, time.Since(start).Round(time.Second))
	for i, msg := range failures {
		if i >= 20 {
			log.Printf("  ... 其余 %d 个失败省略", len(failures)-20)
			break
		}
		log.Printf("  失败：%s", msg)
	}
	if failed > 0 {
		os.Exit(1)
	}
}

func newClient(ep endpoint) (*minio.Client, error) {
	endpoint := strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(ep.endpoint), "https://"), "http://")
	endpoint = strings.TrimRight(endpoint, "/")
	return minio.New(endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(ep.accessKey, ep.secretKey, ""),
		Secure:       ep.secure,
		Region:       strings.TrimSpace(ep.region),
		BucketLookup: lookupType(ep.lookup, endpoint),
	})
}

func lookupType(value, endpoint string) minio.BucketLookupType {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "path", "path-style":
		return minio.BucketLookupPath
	case "dns", "virtual", "virtual-host", "virtual-hosted":
		return minio.BucketLookupDNS
	}
	if strings.HasSuffix(strings.ToLower(endpoint), ".myqcloud.com") {
		return minio.BucketLookupDNS
	}
	return minio.BucketLookupAuto
}

func listObjects(ctx context.Context, client *minio.Client, bucket, prefix string) ([]objectJob, int64, error) {
	var jobs []objectJob
	var total int64
	for obj := range client.ListObjects(ctx, bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		if obj.Err != nil {
			return nil, 0, obj.Err
		}
		jobs = append(jobs, objectJob{key: obj.Key, size: obj.Size, contentType: obj.ContentType})
		total += obj.Size
	}
	return jobs, total, nil
}

func verifyObjects(ctx context.Context, client *minio.Client, bucket string, jobs []objectJob, concurrency int) (int, int, error) {
	var missing, mismatched int64
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for _, job := range jobs {
		wg.Add(1)
		sem <- struct{}{}
		go func(job objectJob) {
			defer wg.Done()
			defer func() { <-sem }()
			info, err := client.StatObject(ctx, bucket, job.key, minio.StatObjectOptions{})
			if err != nil {
				atomic.AddInt64(&missing, 1)
				return
			}
			if info.Size != job.size {
				atomic.AddInt64(&mismatched, 1)
			}
		}(job)
	}
	wg.Wait()
	return int(missing), int(mismatched), nil
}

func recordFailure(failures *[]string, mu *sync.Mutex, key string, err error) {
	mu.Lock()
	defer mu.Unlock()
	if len(*failures) < 100 {
		*failures = append(*failures, fmt.Sprintf("%s: %v", key, err))
	}
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if v == "" {
		return fallback
	}
	return v == "true" || v == "1" || v == "yes"
}

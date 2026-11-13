package config

import (
	"os"
	"strconv"
)

type Config struct {
	DatabaseURL             string
	RedisURL                string
	JWTSecret               string
	AdminJWT                string
	ModelRouteCipherKey     string
	APIPort                 string
	AppEnv                  string
	BaseURL                 string
	LocalStoragePublicURL   string
	LocalStorageInternalURL string
	TrustedProxies          string

	MinioEndpoint  string
	MinioAccessKey string
	MinioSecretKey string
	MinioBucket    string
	MinioUseSSL    bool
	MinioPublicURL string
	// MinioRegion 用于 S3 V4 签名；COS 建议显式填写 ap-hongkong 等地域。
	MinioRegion string
	// MinioPublicURLStyle 为 object 时公开地址不带 bucket 前缀（COS + 自定义 CDN 域名）。
	MinioPublicURLStyle string
	MinioBucketPrivate  bool
	MinioSkipBucketInit bool
	// MinioBucketLookup 控制 S3 请求风格：auto（默认）/ path / dns（虚拟主机风格）。
	MinioBucketLookup string

	NewAPIBaseURL          string
	NewAPIToken            string
	NewAPITimeoutSec       int
	NewAPIStreamTimeoutSec int
}

func Load() *Config {
	return &Config{
		DatabaseURL:             getEnv("DATABASE_URL", "postgres://starai:starai@localhost:5432/starai?sslmode=disable"),
		RedisURL:                getEnv("REDIS_URL", "redis://localhost:6379/0"),
		JWTSecret:               getEnv("JWT_SECRET", "dev-jwt-secret-starai"),
		AdminJWT:                getEnv("ADMIN_JWT_SECRET", "dev-admin-jwt-secret"),
		ModelRouteCipherKey:     getEnv("MODEL_ROUTE_CIPHER_KEY", getEnv("ADMIN_JWT_SECRET", "dev-admin-jwt-secret")),
		APIPort:                 getEnv("API_PORT", "8080"),
		AppEnv:                  getEnv("APP_ENV", "development"),
		BaseURL:                 getEnv("BASE_URL", ""),
		LocalStoragePublicURL:   getEnv("LOCAL_STORAGE_PUBLIC_URL", ""),
		LocalStorageInternalURL: getEnv("LOCAL_STORAGE_INTERNAL_URL", ""),
		TrustedProxies:          getEnv("TRUSTED_PROXIES", "127.0.0.1,::1,172.16.0.0/12"),
		MinioEndpoint:           getEnv("MINIO_ENDPOINT", "localhost:9000"),
		MinioAccessKey:          getEnv("MINIO_ACCESS_KEY", "starai"),
		MinioSecretKey:          getEnv("MINIO_SECRET_KEY", "starai123"),
		MinioBucket:             getEnv("MINIO_BUCKET", "starai-works"),
		MinioUseSSL:             getEnv("MINIO_USE_SSL", "false") == "true",
		MinioPublicURL:          getEnv("MINIO_PUBLIC_URL", "/media"),
		MinioRegion:             getEnv("MINIO_REGION", ""),
		MinioPublicURLStyle:     getEnv("MINIO_PUBLIC_URL_STYLE", "bucket"),
		MinioBucketPrivate:      getEnv("MINIO_BUCKET_PRIVATE", "false") == "true",
		MinioSkipBucketInit:     getEnv("MINIO_SKIP_BUCKET_INIT", "false") == "true",
		MinioBucketLookup:       getEnv("MINIO_BUCKET_LOOKUP", "auto"),
		NewAPIBaseURL:           getEnv("NEW_API_BASE_URL", "http://localhost:3002"),
		NewAPIToken:             getEnv("NEW_API_TOKEN", "sk-platform-internal-token"),
		NewAPITimeoutSec:        getEnvInt("NEW_API_TIMEOUT_SECONDS", 300),
		NewAPIStreamTimeoutSec:  getEnvInt("NEW_API_STREAM_TIMEOUT_SECONDS", 600),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

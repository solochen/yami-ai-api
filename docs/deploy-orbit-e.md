# Orbit-E AI 生产部署手册（腾讯云 HK · 124.156.112.244）

2026-10-17 首次部署完成。本文档记录完整部署架构、同步步骤与日常运维入口。

## 1. 部署架构

| 域名 | 路由 | 上游 |
|---|---|---|
| https://www.orbit-e.com | `/*` | web 容器 172.28.0.30:3000 |
| https://www.orbit-e.com | `/api/*`（priority 200） | api 容器 172.28.0.31:8080 |
| https://www.orbit-e.com | `/minio-store/*`（priority 200，proxy-rewrite 去前缀） | minio 172.28.0.33:9000 |
| https://orbit-e.com | 同 www（裸域同一套规则） | 同上 |
| https://admin.orbit-e.com | `/*` + `/api/*`（**DNS 记录未加，待补**） | admin 172.28.0.32:3001 / api |

- 入口：APISIX standalone（`sub2api-apisix` 容器，80/443），路由文件 `/home/ubuntu/sub2api-prod/apisix/apisix.base.yaml`
- 证书：Let's Encrypt（certbot 宿主机 standalone :8888 + ACME 路由放行），deploy hook 自动续期
- 项目目录：`/home/ubuntu/yard-api/yami-ai-api`（git clone github.com/solochen/yami-ai-api）
- compose：`infra/docker/docker-compose.prod.yml` + `infra/docker/docker-compose.prod.override.yml`（服务器专用，不入库），`.env.production` 在仓库根目录（600 权限）
- 容器：starai-{postgres,redis,minio,searxng,mock-new-api,api,worker,web,admin}，项目名 `starai`

## 2. 与本地一致性的关键配置

- **`.env.production`**：JWT/ADMIN_JWT/MODEL_ROUTE_CIPHER_KEY/NEW_API_TOKEN/MINIO 密钥 与本地 dev 容器同值（`model_routes.api_key` 以 MODEL_ROUTE_CIPHER_KEY 加密，必须一致才能解密）；仅基础设施地址改为容器服务名（postgres/redis/minio）
- **MinIO**：桶 `starai-works`（318MB 全量同步，版本 RELEASE.2025-09-07T16-13-09Z 与本地一致，镜像由 GitHub Release 二进制在服务器自建 `orbit-minio:2025-09-07`——quay.io/dl.min.io 在服务器网络均被挡）
- **MINIO_PUBLIC_URL**：`https://www.orbit-e.com/minio-store`（路径型，走 APISIX）；DB 中历史 URL 已全量改写（20 列，`http://localhost:9000` → 该前缀）
- **site_base_url**：已改为 `https://www.orbit-e.com`
- 数据库：本地 pg_dump 全量恢复（67 表，迁移基线 148），schema_migrations 同步，无需再跑 migrate
- Redis 不需要同步（队列为瞬态）

## 3. 日常操作（服务器）

```bash
cd /home/ubuntu/yard-api/yami-ai-api
export CMP="docker compose -f infra/docker/docker-compose.prod.yml -f infra/docker/docker-compose.prod.override.yml --env-file .env.production --profile search --profile preprod"
$CMP ps                          # 状态
$CMP logs -f api worker           # 日志
$CMP up -d                        # 启动全部
$CMP build api worker web admin   # 升级构建（COMPOSE_PARALLEL_LIMIT=1，4C8G 建议）
```

升级发布：本地/`git pull` → `$CMP build` → `$CMP up -d`。

改 APISIX 路由：编辑 `/home/ubuntu/sub2api-prod/apisix/apisix.base.yaml` → `sudo bash /home/ubuntu/sub2api-prod/update-ssls.sh` → `docker restart sub2api-apisix`。

**注意**：APISIX 路由级不允许 `retries` 字段（会导致整条路由加载失败被丢弃）；retries/timeout 写在 upstream 级。

## 4. admin.orbit-e.com 待办（DNS 加好后的两步）

1. DNS 服务商给 `admin.orbit-e.com` 加 A 记录 → 124.156.112.244（ACME 路由已在 hosts 放行）
2. 服务器执行：

```bash
sudo certbot certonly --standalone --http-01-port 8888 --http-01-address 0.0.0.0 \
  --key-type ecdsa --cert-name admin.orbit-e.com -d admin.orbit-e.com \
  --non-interactive --agree-tos
sudo bash /etc/letsencrypt/renewal-hooks/deploy/yardai-apisix.sh   # 复制证书+重载
```

admin 路由/上游已就位（路由 20/21，上游 12/11），证书就位后即生效。

## 5. 重建/重同步手册（从本地恢复）

```bash
# 数据库（本地导出 → 服务器恢复）
docker exec docker-postgres-1 pg_dump -U starai -Fc starai > /tmp/starai.dump
scp /tmp/starai.dump yardai:/tmp/
ssh yardai 'cd /home/ubuntu/yard-api/yami-ai-api && docker compose -f infra/docker/docker-compose.prod.yml --env-file .env.production exec -T postgres psql -U starai -d postgres -c "DROP DATABASE IF EXISTS starai WITH (FORCE);" -c "CREATE DATABASE starai OWNER starai;"'
ssh yardai 'cd /home/ubuntu/yard-api/yami-ai-api && docker compose -f infra/docker/docker-compose.prod.yml --env-file .env.production exec -T postgres pg_restore -U starai -d starai --no-owner < /tmp/starai.dump'

# MinIO（同版本数据卷直拷）
docker run --rm -v docker_minio_data:/data -v /tmp:/backup alpine tar czf /tmp/minio-data.tar.gz -C /data .
scp /tmp/minio-data.tar.gz yardai:/tmp/
ssh yardai 'docker run --rm -v starai_minio_data:/data -v /tmp:/backup alpine tar xzf /backup/minio-data.tar.gz -C /data && docker restart starai-minio-1'

# URL 改写（新入库数据若含 localhost:9000，见仓库 /tmp/rewrite-localhost.sql 模式：
# 对 public 全部 text/varchar/jsonb/json 列 replace('http://localhost:9000', 'https://www.orbit-e.com/minio-store')）
```

## 6. 已知边界

- **服务器出口公网带宽约 3Mbps（小水管）**：APISIX → 客户端是全链路瓶颈（容器间 40ms 拉 1.4MB，出公网 4.5s；跨境访问未压缩大 JSON 曾达 70s 导致灵感广场渲染为空）。
  已在 APISIX 网关层开启 gzip（`apisix/config.yaml` http_configuration_snippet：`gzip on` + `gzip_types application/json text/css application/javascript image/svg+xml`；`text/event-stream` 不在列表，**SSE 流式不受影响**）。
  实测 `/api/gallery/references` 1.4MB → 411KB，跨境端到端 70s → **0.97s**。后续若仍慢：升级服务器带宽，或应用层给该接口瘦身（分页/裁剪字段）。
- **OpenRouter 生图模型区域限制**：该 HK 出口 IP 调 `openai/gpt-image-2`、`google/gemini-*-image` 均返回 `403 not available in your region`（本地同现象）。**已验证可用**：`seedream-5-0-lite`（生成成功且新图 URL 自动为公共域名）、`qwen-image-3`（慢但在跑）。聊天模型（z-ai/glm-5.3-flash）正常。
  若需切换 ecom_factory 生图模型：
  `UPDATE workflow_definitions SET runtime_config=jsonb_set(runtime_config,'{generation_model_code}','"seedream-5-0-lite"'), updated_at=now() WHERE code='ecom_factory';`
- BigModel 线路 key 已过期（401），OpenRouter 线路正常（api key 加密存于 model_routes）
- mock-new-api 仅内网（preprod profile），不暴露公网
- `minio/minio` 不在 Docker Hub、quay.io 被墙：镜像 `orbit-minio:2025-09-07` 由 GitHub Release 二进制构建（alpine:3.20 基底），已在服务器本地

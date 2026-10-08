# 灵感广场阶段 9：测试、上线与运营手册

本手册对应 `docs/inspiration-gallery-content-development-plan.md` 的阶段 9，覆盖参考案例、案例规格、批量生成、质量审核和明确发布。它保留阶段 7 的发布闸门：生成成功只进入 `pending_review`，人工通过后还必须单独点击“发布”。

## 1. 上线前检查

在仓库根目录执行：

```bash
node scripts/verify-migrations.js
(cd services/api && go test ./...)
(cd services/worker && go test ./...)
pnpm --filter @starai/admin lint
pnpm --filter @starai/admin build
pnpm --filter @starai/web lint
pnpm --filter @starai/web build
git diff --check
```

迁移校验按完整文件名和 checksum 工作。130、131、132 的重复数字属于历史兼容例外；新迁移仍不得复用未登记的数字。不要编辑已应用的历史 `.up.sql`，应新增迁移。

不要让同一应用目录中的 `next dev` 和 `next build` 并行读写 `.next`。生产构建后如需继续本地开发，必须停止旧开发进程、移走或清理对应应用的 `.next`，再重新启动开发服务；验收时除 HTTP 状态外还要检查页面截图和浏览器控制台，CSS 资源 404 时页面 HTML 仍可能返回 200。

## 2. 隔离预生产验证

Mac/Linux 使用 Bash 版脚本：

```bash
pnpm preprod:verify
```

脚本使用独立 Compose 项目 `starai-preprod`、独立数据库名和端口，依次执行：

1. PostgreSQL、Redis、mock gateway 健康检查。
2. API、Worker、Admin、Web 镜像构建。
3. 迁移执行两次，验证幂等。
4. 在隔离数据库内执行完整 `migrate down`，再执行 `migrate up`，验证回滚恢复。
5. API `/health`、`/metrics`、Gallery 三个公开接口。
6. Admin Gallery 未登录时返回 401/403，登录页、图库页返回 200。
7. Web `/app/gallery` 返回 200。
8. `scripts/integration-test.sh` 的用户登录、Admin 登录、模型、支付保护、Gallery 公共和后台契约、聊天 smoke 测试；临时验证码配置在退出时恢复。
9. `scripts/gallery-e2e.sh` 的缺口与 `ready` 规格导入、批次创建、Worker mock 生图和持久化、自动质检、人工审核、明确发布、重复发布幂等和公共 Gallery 查询。

保留环境排查时：

```bash
pnpm preprod:verify -- --keep-running
```

验证后访问 `http://localhost:13000`、`http://localhost:13001`、`http://localhost:18080`。脚本默认会删除隔离卷；只有需要复现问题时才使用 `--keep-running` 或 `--preserve-data`。

Windows 仍可使用 `scripts/preprod-verify.ps1`；Mac/Linux 优先使用 Bash 版，避免依赖 PowerShell。

2026-10-02 本地验收已完整通过以上流程，隔离容器、网络、数据库卷和临时上传目录在结束后自动删除。端到端回归还验证了未审核发布返回 400、空拒绝原因返回 400、公开资源 URL 在 API 容器内通过内部地址完成质检且不会泄漏内部 URL。该结果证明当前仓库快照可完成本地预生产门禁，不替代部署目标机器上的同命令复验。

`LOCAL_STORAGE_INTERNAL_URL` 只用于 API 在容器网络内回访本地上传资源。浏览器和数据库仍保存 `LOCAL_STORAGE_PUBLIC_URL`；如果公开地址在容器内本来就可访问，可以留空。隔离预生产配置为：

```dotenv
LOCAL_STORAGE_PUBLIC_URL=http://localhost:18080/uploads-local
LOCAL_STORAGE_INTERNAL_URL=http://api:8080/uploads-local
```

`scripts/gallery-e2e.sh` 会创建并发布一条带唯一后缀的测试案例，只应在可销毁的隔离环境中运行。生产验收使用真实、人工批准的小批次，不直接执行该脚本。

## 3. Gallery 运营闭环

每个缺口按以下顺序处理，任何一步失败都可以停留在当前状态并重试：

1. **缺口**：在 `/admin/gallery/specs` 查看缺口任务和覆盖率，确认分类、风格、场景、Prompt 语言和风险边界。
2. **规格**：导入或编辑案例规格。后端会校验必填字段、候选模型、风险审核和 `ready` 状态，非法规格不会进入生成。
3. **生成**：在 `/admin/gallery/generation` 按预算和并发创建批次。失败任务可以单独重试，活动任务唯一约束防止重复生成。
4. **质量审核**：在 `/admin/gallery/quality-reviews` 执行自动检查并人工查看图片、Prompt、标签、尺寸、资源地址和风险。OCR/语义检查不可用时必须人工核对，不得把 `manual_required` 当成自动通过。
5. **审核决定**：通过进入 `approved`；拒绝必须填写原因。
6. **发布**：只有人工通过后才可明确发布。重复发布是幂等的，不应创建多个公共案例。
7. **公共验证**：生成案例检查 `GET /api/gallery`，同步参考案例检查 `GET /api/gallery/references`，并最终检查 Web `/app/gallery`。未发布、未授权或无自有资源的案例不得出现。

参考案例同步使用：

```bash
pnpm gallery:sync -dry-run
pnpm gallery:sync
```

同步只更新服务端快照，默认保留 `pending_license`，不会绕过许可证审核，也不会自动发布。

## 4. 备份与恢复

生产发布前先做完整备份，并在另一环境验证：

```bash
ENV_FILE=.env.production bash scripts/export-full-backup.sh
bash scripts/verify-full-backup.sh backups/full/starai-full-backup-<timestamp>.tar.gz
```

备份包含数据库、上传资源、生产环境文件和 checksum，必须放在私有存储，不得提交 Git。恢复是有破坏性的，必须显式确认：

```bash
CONFIRM_RESTORE=1 bash scripts/import-full-backup.sh backups/full/starai-full-backup-<timestamp>.tar.gz
```

恢复后重新执行迁移、健康检查和 Gallery 公共接口检查。不要对当前开发数据库运行 `migrate down`；回滚验证只允许使用隔离预生产数据库。

## 5. 发布与回滚

推荐顺序：备份 -> 校验配置 -> 构建镜像 -> 运行一次 `migrate up` -> 启动 API/Worker -> 健康检查 -> 启动 Web/Admin -> 登录和 Gallery smoke。数据库迁移是向前兼容优先；应用回滚不自动回滚数据库迁移。

应用异常时优先：

```bash
docker compose --env-file .env.production -f infra/docker/docker-compose.prod.yml ps
docker compose --env-file .env.production -f infra/docker/docker-compose.prod.yml logs --tail=200 api worker web admin
AUTO_ROLLBACK=1 bash scripts/deploy-prod.sh
```

只有确认数据库损坏或灾难恢复时，才在维护窗口使用已验证的完整备份恢复。参考案例发布异常时，先停止发布操作并保留审核记录，不要直接删除来源快照或公共表历史。

## 6. 监控、告警与成本

最低监控项：

- API `/health` 非 200，或 `worker_status` 非 `ok`。
- `/metrics` 中 HTTP 5xx、请求延迟、队列失败和上游错误持续升高。
- Gallery 批次 `failed`、任务重试耗尽、`pending_review` 长时间堆积。
- 质量审核 `rejected`、重复内容拦截、许可证待核验数量异常增长。
- PostgreSQL、Redis、对象存储容量和备份 checksum 失败。

成本边界：批次创建时重新读取启用图片模型并使用 `ModelService.EstimateCost` 计算 `estimated_cost`，同时记录 `cost_rule_version`；`budget_limit` 在服务端强制执行。真实 OpenRouter API Key 由管理员在模型线路后台填写，不能写进规格、日志或前端。上线前应按日设置预算上限、失败重试上限和人工发布配额，并在账单侧核对上游实际成本。

## 7. 已知边界

- 当前 OCR 和视觉语义检查没有部署真实模型，相关结果会明确标记为 `unavailable/manual_required`。
- 参考案例当前可能为 0 条公开结果，这是许可证审核或明确发布闸门的真实状态，不等于接口故障。
- Web/Admin 构建仍可能输出仓库既有的 `<img>` 优化和 Hook 依赖 warning；本阶段以无 error、生产构建成功和运行态页面可访问为通过标准。

# 「一键同步上游分组账号」后台功能 · 开发计划

> 目标：把 CodexAuv / PackyCode 这类上游的「分组 → Key → 项目账号」接入过程，从"AI/人工逐步操作"
> 变成后台里点几下就能完成的**可重复、幂等、可审计**的功能。

---

## 1. 范围（做什么 / 不做什么）

**做**
- 在后台配置「同步源」：上游类型 + 站点地址 + 凭证 + 登录账号
- 一键预览：拉取站点分组，与项目现有账号做 diff，列出「已导入 / 将新建 / 不可用」
- 一键执行：为缺失分组创建 Key（创建即加密保存），按规范建/更新上游账号，回填分组倍率
- 连通性校验：逐个账号请求 `/v1/models`（或按协议发最小请求），输出通过率
- 运行记录：每次同步的明细（建了几个 Key、更新几个账号、失败原因）
- 凭证失效检测：401/403 时提示"需重新授权"

**不做（默认）**
- 不删除站点上任何 Key
- 不删除项目里已有账号
- 不自动改绑模型线路（提供开关，默认关闭）
- 不做浏览器自动化（不依赖 CDP/登录态，改用用户粘贴的凭证）

---

## 2. 用户操作流程

```
上游账号管理页
  └─ [同步上游] ──► 选择同步源（或新建）
        └─ [预览] ──► 表格：分组 | 倍率 | 状态（已导入/将新建/不可用） | 说明
              └─ [执行] ──► 进度条 ──► 结果报告（成功/失败明细）
                    └─ 可选：[把新账号绑定到缺失线路]（需二次确认）
```

**凭证获取**：页面提供一段可复制的浏览器控制台脚本，用户登录站点后执行，自动打印所需凭证：

- CodexAuv：`localStorage.auth_token`
- PackyCode：`document.cookie` 中的会话 + `user.id`（`New-Api-User` 头）

---

## 3. 数据模型

```sql
-- 同步源
CREATE TABLE upstream_sync_sources (
  id            bigserial PRIMARY KEY,
  provider      varchar(64)  NOT NULL,        -- codexauv / packycode
  adapter       varchar(32)  NOT NULL,        -- codexauv / new_api
  name          varchar(128) NOT NULL,
  site_base_url text         NOT NULL,
  credential    text         NOT NULL,        -- 加密存储
  credential_hint varchar(64) NOT NULL DEFAULT '', -- 脱敏提示，如 sk-*** / cookie
  login_account varchar(190) NOT NULL DEFAULT '',
  config        jsonb        NOT NULL DEFAULT '{}'::jsonb, -- 额外参数：user_id、分组过滤等
  is_enabled    boolean      NOT NULL DEFAULT true,
  last_sync_at  timestamptz,
  last_status   varchar(16),
  last_error    text,
  created_at, updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (provider, site_base_url)
);

-- 同步运行记录
CREATE TABLE upstream_sync_runs (
  id           bigserial PRIMARY KEY,
  source_id    bigint NOT NULL REFERENCES upstream_sync_sources(id),
  status       varchar(16) NOT NULL,   -- running / success / partial / failed
  groups_total int NOT NULL DEFAULT 0,
  keys_created int NOT NULL DEFAULT 0,
  accounts_created int NOT NULL DEFAULT 0,
  accounts_updated int NOT NULL DEFAULT 0,
  verify_passed    int NOT NULL DEFAULT 0,
  verify_failed    int NOT NULL DEFAULT 0,
  started_at   timestamptz NOT NULL DEFAULT now(),
  finished_at  timestamptz,
  summary      text
);

-- 运行明细
CREATE TABLE upstream_sync_run_items (
  id        bigserial PRIMARY KEY,
  run_id    bigint NOT NULL REFERENCES upstream_sync_runs(id) ON DELETE CASCADE,
  group_name varchar(128) NOT NULL,
  rate      numeric(12,6) NOT NULL DEFAULT 0,
  action    varchar(16) NOT NULL,   -- existed / created_key / created_account / updated / skipped / failed
  message   text,
  account_id bigint
);
```

复用现有：`upstream_accounts`（key 加密存储）、`upstream_group_rates`、`upstream_events`。

---

## 4. 后端设计

### 4.1 适配器接口

```go
type UpstreamSyncGroup struct {
    Name     string
    Rate     float64
    Meta     map[string]any // 厂商、描述、可用模型数
}

type UpstreamSyncKey struct {
    Name  string
    Key   string // 完整 key（创建后立即加密保存）
    Group string
}

type UpstreamAdapter interface {
    Name() string
    FetchGroups(ctx context.Context, src SyncSource) ([]UpstreamSyncGroup, error)
    FetchKeys(ctx context.Context, src SyncSource) ([]UpstreamSyncKey, error)
    CreateKey(ctx context.Context, src SyncSource, group UpstreamSyncGroup) (*UpstreamSyncKey, error)
    VerifyKey(ctx context.Context, src SyncSource, key, group string) (bool, int, error)
}
```

- `codexauv`：`GET /api/v1/groups/available`、`POST /api/v1/keys`、`GET /v1/models`
- `new_api`（PackyCode 等）：`GET /api/user/self/groups`、`POST /api/token/`、`GET /v1/models`，
  自动带 `New-Api-User` 头（从 `config.user_id` 取）

### 4.2 同步引擎（幂等）

```
1. 拉站点分组 + 现有令牌
2. 拉项目账号（provider 维度）
3. 逐分组决策：
   - 已有同名 Key（按分组匹配）→ 复用
   - 无 Key 且允许建 → 创建（记录完整 key）
   - 有账号 → 更新 key/名称/登录账号；无账号 → 新建
4. 回填 upstream_group_rates
5. 校验：/v1/models（或协议化请求）
6. 写 run/item 明细 + 审计日志
```

**失败保护**：Key 创建成功后必须先落库（加密）再继续；中途失败时 run 标记 `partial`，支持重跑（幂等）。

### 4.3 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| GET/POST | `/admin/api/upstream-sync/sources` | 同步源列表 / 新建 |
| PATCH/DELETE | `/admin/api/upstream-sync/sources/:id` | 更新 / 删除 |
| POST | `/admin/api/upstream-sync/sources/:id/preview` | 预览 diff（不写库） |
| POST | `/admin/api/upstream-sync/sources/:id/run` | 执行同步 |
| GET | `/admin/api/upstream-sync/runs?source_id=` | 运行历史 |
| GET | `/admin/api/upstream-sync/runs/:id` | 运行明细 |

全部 `super_admin` 权限，凭证字段只写不读（返回脱敏）。

---

## 5. 前端页面

1. **上游账号管理**页新增按钮「同步上游」→ 弹窗
2. 弹窗结构：
   - 同步源选择（下拉）+ 「新建同步源」
   - 新建源表单：上游类型 / 站点地址 / 凭证（粘贴）/ 登录账号 / 高级（user_id、分组过滤）
   - 「获取凭证」帮助面板（可复制的控制台脚本）
   - 预览结果表：分组 / 倍率 / 状态 / 说明
   - 执行按钮（二次确认）+ 进度
   - 结果卡片：新建 Key 数、新增账号数、更新数、校验通过率、失败项
3. 可选：同步源管理列表（最近同步时间、状态、重新授权）

---

## 6. 实施计划

| 阶段 | 内容 | 产出 | 工作量 |
|---|---|---|---|
| P0 | 方案确认（范围/凭证方式/是否允许改绑） | 确认单 | 0.5 天 |
| P1 | 数据模型 + 适配器框架 + CodexAuv 适配器 | 迁移 212/213 + adapter 单测 | 1 天 |
| P2 | PackyCode（new-api）适配器 + 凭证提取脚本 | adapter | 0.5 天 |
| P3 | 同步引擎（diff/建 Key/建账号/回填倍率/校验/幂等） | service | 1 天 |
| P4 | 后端接口 + 审计 + 权限 | handler/route | 0.5 天 |
| P5 | 后台页面（同步源管理 + 预览 + 执行 + 结果） | 页面 | 1.5 天 |
| P6 | 联调验收（真实账号、幂等重跑、凭证失效、限流） | 测试报告 + 文档 | 1 天 |

合计 **约 6 天**（单人）。

---

## 7. 验收标准

- [ ] 同一分组重复同步：不重复创建 Key/账号（幂等）
- [ ] 新建 Key 后账号可连通（`/v1/models` 200）
- [ ] 分组倍率写入 `upstream_group_rates` 且账号 `rate_multiplier` 正确
- [ ] 凭证失效时给出「需重新授权」而不是笼统报错
- [ ] 全程不删除站点 Key；不删除项目账号
- [ ] 每次运行有明细记录 + 审计日志
- [ ] 预览不写库（dry-run 真实可用）

---

## 8. 风险与对策

| 风险 | 对策 |
|---|---|
| 站点接口变更 | 适配器隔离，只改一个文件 |
| 凭证过期（JWT/会话） | 明确提示 + 一键更新入口 |
| 创建 Key 时被限流 | 串行 + 300ms 间隔 + 失败重试 2 次 |
| 完整 Key 只返回一次 | 创建成功立即加密入库，失败则记录并提示重跑 |
| Cloudflare/人机验证 | 不使用浏览器自动化，走用户粘贴的凭证 |
| 误建大量 Key | 默认 dry-run + 明确展示"将创建 N 个" |

---

## 9. 后续扩展

- 新上游适配器（七牛、aiboli、其他 new-api 站）
- 定时巡检：每天对比站点分组变化（新增/下线）并提醒
- 账号级用量与失败率统计（已有基础）
- 可选：同步后自动把新账号接入"线路批量管理"待选池

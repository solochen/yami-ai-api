# 上游账号接入工作流（SOP）

> 用途：把某个上游（CodexAuv / PackyCode / 其他中转站）的「分组 + API Key」批量接入本项目，
> 建成上游账号并绑定模型线路。本文既是**给 AI 的提示词模板**，也是**人工操作手册**。

---

## 一、前置条件（缺一不可）

1. **浏览器**：Chrome 以 `--remote-debugging-port=9222` 运行，且已登录目标上游后台
2. **项目后台**：本机 API 可访问（默认 `http://127.0.0.1:8080`），并持有 super_admin 会话
3. **明确目标账号**：确认登录的账号 ID/邮箱，写入所有新建账号的 `login_account` 字段

> ⚠️ 若登录账号与预期不符，**先停止**，让用户切换账号后再继续。

---

## 二、每个上游的接入参数（实测）

### CodexAuv（https://www.codexauv.com）

| 项 | 值 |
|---|---|
| 登录态 | `localStorage.auth_token`（JWT） |
| 分组列表 | `GET /api/v1/groups/available` → `data[].{id,name,rate_multiplier}` |
| Key 列表 | `GET /api/v1/keys?page=1&page_size=200` → `data.items[].{id,key,name,group.name}` |
| 创建 Key | `POST /api/v1/keys`，body `{"name":"<分组名>","group_id":<id>}` → 返回**完整 key** |
| 鉴权 | `Authorization: Bearer <auth_token>` |
| 特点 | 建 Key 后 Key 完整值**只在创建时和列表接口**能拿到；列表接口可直接取回全量 key |
| 兼容性 | 用 `/v1/models` + 实际调用双重校验；图片模型需单独分组（如 `Image-1k专线`） |

### PackyCode（https://www.packyapi.ai）

| 项 | 值 |
|---|---|
| 登录态 | Cookie 会话（`credentials: include`）+ **必须带请求头 `New-Api-User: <用户ID>`** |
| 用户信息 | `GET /api/user/self` |
| 分组列表 | `GET /api/user/self/groups` → `{分组名: {ratio, vendors, desc}}` |
| Key 列表 | `GET /api/token/?p=1&size=100` |
| 创建 Key | `POST /api/token/`，body `{"name":"<分组名>","group":"<分组名>","unlimited_quota":true,"expired_time":-1,"remain_quota":0,"model_limits_enabled":false,"model_limits":"","allow_ips":""}` → 返回**完整 key** |
| 模型清单 | `GET /v1/models`（Bearer key） |
| 协议差异 | Claude 模型必须走 `POST /v1/messages` + `x-api-key` + `anthropic-version: 2023-06-01`；`cc` 分组仅允许 Claude CLI（普通 API 返回 403） |

> 其他 new-api 系中转站可套用 PackyCode 的模板（`New-Api-User` 头 + `/api/token/`）。

---

## 三、项目侧接口与约定

| 用途 | 接口 |
|---|---|
| 账号列表 | `GET /admin/api/upstream-accounts?provider=<p>` |
| 新建账号 | `POST /admin/api/upstream-accounts` `{provider,name,api_key,group_name,login_account}` |
| 更新账号 | `PATCH /admin/api/upstream-accounts/:id` |
| 测试账号 | `POST /admin/api/upstream-accounts/:id/test` |
| 分组能力 | `GET /admin/api/upstream-groups/capabilities?provider=<p>` |
| 线路列表 | `GET /admin/api/model-routes/list?provider=<p>&page_size=200` |
| 批量绑定 | `POST /admin/api/model-routes/batch-bind` `{route_ids:[], account_id, force}` |
| 单条绑定 | `POST /admin/api/model-routes/:routeId/bind` `{account_id, force}` |
| 事件列表 | `GET /admin/api/upstream-events?limit=50` |

**命名规范**：`<上游展示名> · <分组名>-<倍率>x-<登录账号>`，例如
`PackyCode · aws-q-0.3x-oidc_45529`。

**倍率来源优先级**：`upstream_group_rates`（权威） > `upstream_price_current.billing_multiplier`。

---

## 四、执行步骤

1. **确认账号**：读取站点用户信息，记录 `登录账号`（ID/邮箱/@用户名）
2. **拉分组**：取全部分组（名称、倍率、可用模型）
3. **对比存量**：拉项目现有账号，得到「已覆盖分组」与「缺失分组」
4. **建缺失 Key**：一个分组一个 Key，名称 = 分组名；**创建时立即保存完整 key 到临时文件**（不要只依赖页面展示）
5. **回填分组倍率**：确保 `upstream_group_rates` 有该上游全部分组倍率
6. **导入账号**：对每个分组
   - 已有同分组账号 → 更新其 `api_key`、`name`、`login_account`
   - 缺失 → 新建
7. **校验连通性**：对每个账号调用 `/v1/models`（或按协议发最小请求），记录结果
8. **绑定线路**（可选，需用户确认）：
   - 按模型 → 分组兼容性（优先 `/v1/models` 实际清单，其次价格矩阵）
   - 同倍率下选最便宜可用分组；claude 类注意协议
   - 用 `batch-bind`（必要时 `force=true`）执行并记录变更
9. **输出报告**：表格列出账号、分组、倍率、Key 掩码、连通性、承载线路数、失败项与建议

---

## 五、决策规则（避免踩坑）

| 场景 | 规则 |
|---|---|
| 同上游同分组已有账号 | 更新而非新建，避免重复 |
| 分组在新账号中已不存在（如 `gemini-officially`） | 不新建；提示用户改绑或禁用线路 |
| 模型不在目标分组 `/v1/models` 清单 | 视为不兼容，禁止绑定（除非 `force`） |
| `protocol_not_supported` | 不是账号故障；检查协议（Claude → `/v1/messages`） |
| `403 only accessible via official Claude CLI` | 该分组仅限 CLI 客户端，普通 API 不可用，建议禁用 |
| 图片/视频模型 | 需专门分组；验证时用对应端点（`/v1/images/generations` 等） |
| 探测/告警 | 阈值：401/402/403 或额度不足 → 熔断 30 分钟；其它累计 3 次 → 10 分钟 |

---

## 六、安全约束（必须遵守）

1. **不删除**站点上已有 Key，除非用户明确要求
2. **不改**生产环境（香港/新加坡），除非用户明确要求
3. 完整 Key **不落公开文件**；临时文件用完即删
4. 批量改绑线路前，**先预览影响条数**并让用户确认
5. 操作前后各记录一次快照（分组、账号、线路绑定）

---

## 七、AI 提示词模板（可直接复制使用）

```text
你是我的上游账号接入助手。请按 《docs/workflows/upstream-account-onboarding.md》 的 SOP，
帮我接入一个新的上游账号。

参数：
- 上游名称：<CodexAuv / PackyCode / 其他>
- 站点地址：<https://...>
- 登录账号（用于回填）：<82907183 / oidc_45529 / ...>
- 操作范围：<只建 Key 和账号 / 同时改绑线路>
- 是否允许删除站点 Key：<否>
- 是否允许改绑现有线路：<否 / 是（需预览确认）>

要求：
1. 先确认浏览器已登录且账号正确，若不是则停下让我切换
2. 拉取分组列表并与项目现有账号对比，输出「已有 / 缺失」清单
3. 只为缺失分组创建 Key，创建时立即保存完整 key
4. 按命名规范 <上游> · <分组>-<倍率>x-<登录账号> 导入项目
5. 每个账号做连通性校验，输出结果表
6. 若有分组不可用（协议/权限/模型缺失），单独列出并给建议
7. 全程不删除站点 Key、不改生产环境，改绑线路前先让我确认

输出格式：
- 站点侧：分组数、新建 Key 数、失败项
- 项目侧：新建/更新账号数、连通性通过率
- 待决策项：不兼容分组、缺失模型、协议限制
```

---

## 八、脚本化建议（下一步）

把本 SOP 实现为 `scripts/upstream-onboard/`：

```
node scripts/upstream-onboard/index.mjs \
  --provider packycode \
  --site https://www.packyapi.ai \
  --login oidc_45529 \
  --api http://127.0.0.1:8080 \
  --dry-run        # 先 diff，不写入
  --apply          # 实际创建/导入
```

内部复用本文的接口与规则，做到**幂等**（重复执行不会重复建 Key/账号），
并可把结果写入 `reports/upstream-onboard-<provider>-<date>.md`。

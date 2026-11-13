# 腾讯云 COS 私有桶 + 自定义 CDN 域名接入手册（yami-ai-api）

> 适用场景：项目部署在香港服务器，灵感广场 / 作品 / 上传素材的图片体积大、直连源站慢，
> 已创建 **私有读 + 私有写** 的 COS 存储桶，希望：
>
> 1. 用户访问图片走 CDN，不再回源到香港服务器；
> 2. 项目里所有上传功能（用户上传、资产管理、AI 生成结果、灵感广场参考图）直接落到 COS；
> 3. 上传后返回的地址浏览器能直接打开，并且长期有效。
>
> 本文按 **从 0 到 1** 的顺序写，每一步都可以单独验收。文中示例值在表 0-1。

## 表 0-1 本文使用的示例值

| 项 | 示例值 | 说明 |
| --- | --- | --- |
| 站点主域名 | `www.orbit-e.com` | 现有前台域名 |
| 图片 CDN 域名 | `img.orbit-e.com` | **新建**子域名，用户实际访问图片用 |
| 自定义源站域名（可选） | `cos-origin.orbit-e.com` | 给第三方 CDN 回源 / 运维直连用 |
| COS 存储桶 | `yami-media-1250000000` | 名字里的 `1250000000` 是你的 APPID |
| 存储桶地域 | `ap-hongkong` | 香港 |
| COS 默认域名 | `yami-media-1250000000.cos.ap-hongkong.myqcloud.com` | 控制台可查 |
| 项目目录 | `/home/ubuntu/yard-api/yami-ai-api` | 以你服务器实际路径为准 |

---

## 0. 先填对三个值：桶名 / APPID / 地域

后面所有配置都围绕这三个值，填错的症状见 0.2。

| 占位符 | 是什么 | 去哪看 | 示例 |
| --- | --- | --- | --- |
| 桶名（bucket） | **完整**桶名，含 APPID 后缀 | COS 控制台 → 存储桶列表 → 「存储桶名称」列；或桶概览 → 基本信息 → 名称 | `yami-media-1250000000` |
| APPID | 主账号的 10 位 APPID，等于桶名最后一段 | 桶名 `xxx-1250000000` 的后缀；或 [账号信息](https://console.cloud.tencent.com/developer) → APPID | `1250000000` |
| 地域（region） | 桶所在地域的 API 代码 | 桶概览 → 「所属地域」；或默认域名里 `cos.<region>.myqcloud.com` 的中间段 | `ap-hongkong` |

拆解示例：

```text
yami-media-1250000000
└──┬─────┘ └───┬───┘
 桶名前缀     APPID（CAM 资源里写成 uid/1250000000）

yami-media-1250000000.cos.ap-hongkong.myqcloud.com
                              └───┬─────┘
                                地域 = ap-hongkong
```

**最容易搞错的 5 件事：**

1. **桶名要带 APPID**。控制台新建桶时你只输入 `yami-media`，系统自动补成
   `yami-media-1250000000`；API / SDK / 环境变量里必须用**全名**，只写 `yami-media`
   会报 `NoSuchBucket`。
2. **APPID ≠ UIN**。CAM 资源里 COS 用 **`uid/<APPID>`**（10 位数字），
   不是 `uin/<账号ID>`。填成 UIN 会 `AccessDenied`。
3. **`MINIO_ENDPOINT` 不带桶名**：`cos.ap-hongkong.myqcloud.com`。
   写成 `yami-media-1250000000.cos.ap-hongkong.myqcloud.com` 会签名/路径错误。
4. **地域填代码不填中文**：控制台显示「中国香港」，配置里写 `ap-hongkong`。
5. **COS 只接受虚拟主机风格**：请求必须打到 `<bucket>.cos.<region>.myqcloud.com`；
   path-style（`cos.<region>.myqcloud.com/<bucket>/...`）会报
   `must be addressed using COS virtual-styled domain`。
   本仓库对 `*.myqcloud.com` 已自动切换（见 2.1）；旧镜像需显式设置 `MINIO_BUCKET_LOOKUP=dns`。

**一条命令把 CAM 资源拼出来**（避免手滑）：

```bash
BUCKET=yami-media-1250000000
REGION=ap-hongkong
APPID="${BUCKET##*-}"
echo "qcs::cos:${REGION}:uid/${APPID}:${BUCKET}/*"
# 输出：qcs::cos:ap-hongkong:uid/1250000000:yami-media-1250000000/*
```

**三处配置分别怎么填：**

| 配置位置 | 填什么 |
| --- | --- |
| `.env.production` 的 `MINIO_BUCKET` | 完整桶名 `yami-media-1250000000` |
| `.env.production` 的 `MINIO_REGION` | `ap-hongkong` |
| `.env.production` 的 `MINIO_ENDPOINT` | `cos.ap-hongkong.myqcloud.com`（不带桶名） |
| CAM 策略的 `resource` | `qcs::cos:ap-hongkong:uid/1250000000:yami-media-1250000000/*` |
| `MINIO_PUBLIC_URL` | CDN 域名 `https://img.orbit-e.com`（**不要**带桶名） |

**30 秒确认桶名和地域没写错：**

```bash
curl -sI "https://yami-media-1250000000.cos.ap-hongkong.myqcloud.com/healthcheck/probe.txt" | head -1
# 403 Forbidden  → 桶名 + 地域都正确（私有桶拒绝匿名访问）
# 404 Not Found  → 桶名或地域写错了
# 301 Moved      → 地域写错了（桶在别的地域）
```

### 0.2 填错的症状对照

| 症状 | 大概率填错 |
| --- | --- |
| `NoSuchBucket` | `MINIO_BUCKET` 少了 APPID 后缀，或桶名拼错 |
| `AccessDenied`（控制台能看到对象） | CAM 资源里 APPID 写成了 UIN，或 APPID 与桶后缀不一致 |
| `SignatureDoesNotMatch` | `MINIO_REGION` 与桶地域不一致，或 Endpoint 带了桶名 |
| `PermanentRedirect` / 301 | Endpoint 的地域写错（跨地域访问） |
| 上传成功但图片 404 | `MINIO_PUBLIC_URL` 没配成 CDN 域名，或 CDN 没配好 |

---

## 目录

- [0. 先填对三个值：桶名 / APPID / 地域](#0-先填对三个值桶名--appid--地域)
- [1. 先搞清两种域名：选哪个？](#1-先搞清两种域名选哪个)
- [2. 目标架构与项目改造点](#2-目标架构与项目改造点)
- [3. 前置条件与费用](#3-前置条件与费用)
- [4. 第一步：确认私有桶](#4-第一步确认私有桶)
- [5. 第二步：子账号与最小权限](#5-第二步子账号与最小权限)
- [6. 第三步：配置自定义 CDN 加速域名（核心）](#6-第三步配置自定义-cdn-加速域名核心)
- [7. 第四步：配置自定义源站域名（可选）](#7-第四步配置自定义源站域名可选)
- [8. 第五步：改项目配置并自检](#8-第五步改项目配置并自检)
- [9. 第六步：存量图片迁移](#9-第六步存量图片迁移)
- [10. 上传链路：为什么传完就在 COS 里](#10-上传链路为什么传完就在-cos-里)
- [11. 让灵感广场真正变快：缩略图](#11-让灵感广场真正变快缩略图)
- [12. 验收清单](#12-验收清单)
- [13. 回滚方案](#13-回滚方案)
- [14. 常见问题](#14-常见问题)

---

## 1. 先搞清两种域名：选哪个？

COS 控制台的 **域名与传输管理** 下有两个入口，名字很像，作用完全不同：

| | 自定义 CDN 加速域名 | 自定义源站域名 |
| --- | --- | --- |
| 本质 | 域名 → 腾讯云 CDN → COS | 域名 → 直接 CNAME 到 COS 默认域名 |
| 有没有缓存 | **有**，静态图片可缓存几十天 | 无，每次都回源 |
| 能不能加速 | 能（按加速地域就近分发） | 不能，只是给桶换个域名 |
| 私有桶怎么访问 | 配置 **CDN 服务授权 + 回源鉴权**，CDN 用服务身份读私有桶 | 必须每次带 **COS 签名**（预签名 URL），否则 403 |
| 浏览器直接打开 | 可以（本方案关闭 CDN 鉴权时） | 只有带签名的地址可以 |
| 备案要求 | 中国境内/全球加速需备案；**中国境外加速不需要** | 官方文档要求域名已备案（控制台会校验） |
| 本项目用不用 | **用这个**，作为 `MINIO_PUBLIC_URL` | 可选，给第三方 CDN 做源站或运维直连 |

**结论：**

- 用户侧统一走 **`img.orbit-e.com`（自定义 CDN 加速域名）**；
- **自定义源站域名不是必须的**。只有当你想把 COS 挂到别的 CDN（Cloudflare 等）后面，
  或者想要一个稳定的"源站地址"给自己用时才配；
- 同一个子域名通常只能二选一（同时作为 CDN 加速域名和自定义源站域名），两个都要就分配两个子域名。

> 备案提醒：你的服务器在香港，如果域名没有 ICP 备案，**加速地域只能选"中国境外"**。
> 选"中国境内 / 全球加速"会被要求备案，未备案无法添加。中国境外 CDN 对中国大陆用户
> 仍是跨境访问，但至少省掉了"每次回源香港源站"，并带来了边缘缓存和并发能力。

---

## 2. 目标架构与项目改造点

```text
                         ┌──────────────────────────────┐
  浏览器 <img>/<video> ──▶│ img.orbit-e.com（腾讯云 CDN）│
                         └──────────────┬───────────────┘
                                        │ 回源（CDN 服务身份 + 回源鉴权）
                                        ▼
                         ┌──────────────────────────────┐
                         │ COS 私有桶（ap-hongkong）     │
                         └──────────────▲───────────────┘
                                        │ S3 API（PutObject/GetObject/Delete）
                    ┌───────────────────┴───────────────────┐
                    │  api / worker 容器（香港 CVM）         │
                    │  上传、读取、删除、生成结果转存        │
                    └───────────────────────────────────────┘
```

数据库里存的是 **CDN 地址**（`https://img.orbit-e.com/<key>`），浏览器直接访问 CDN；
API/Worker 始终用 S3 API + 密钥访问 COS，不依赖 CDN 回源。

### 2.1 为了支持"私有桶 + 自定义域名"，代码做了哪些改造

| 改造点 | 原因 | 对应配置 |
| --- | --- | --- |
| 公开地址可去掉 bucket 前缀 | CDN/源站域名直接映射对象，`/bucket/key` 会 404 | `MINIO_PUBLIC_URL_STYLE=object` |
| 显式指定 S3 签名地域 | 不指定时 minio-go 会调 `GetBucketLocation`，COS 子账号通常没有该权限 | `MINIO_REGION=ap-hongkong` |
| 不写"公有读"桶策略 | 原逻辑启动时会 `SetBucketPolicy` 把桶设为公有读，与私有桶冲突 | `MINIO_BUCKET_PRIVATE=true` |
| 跳过启动时建桶检查 | 子账号通常无桶级权限；桶已在控制台建好 | `MINIO_SKIP_BUCKET_INIT=true` |
| COS 强制虚拟主机风格 | path-style 会被 COS 拒绝（`virtual-styled domain`） | 代码自动识别 `*.myqcloud.com`；旧镜像设 `MINIO_BUCKET_LOOKUP=dns` |
| 对象 key 反查兼容两种 URL | 灵感广场质检、ZIP 导出要靠 URL 反查 key | 无需配置，代码已兼容 |
| 新增 `storage-check` 自检命令 | 上线前验证密钥、签名、上传、CDN 可达 | 见 8.4 |

> 以上开关默认值都与改造前一致（`bucket` / 公有读 / 检查桶），不配置时旧部署行为不变。

---

## 3. 前置条件与费用

### 3.1 前置条件

- [ ] 腾讯云账号已实名，COS 与 CDN 均已开通（COS 默认开通；CDN 首次进控制台同意开通协议）
- [ ] 一个可解析的域名（`orbit-e.com` 在腾讯云 DNSPod 或任意 DNS 服务商）
- [ ] 若使用"中国境内/全球加速"：域名已完成 ICP 备案
- [ ] 服务器可以访问 `cos.ap-hongkong.myqcloud.com:443`
- [ ] 本地/服务器有 `dig`、`curl`、`docker compose`

### 3.2 会产生哪些费用

| 费用项 | 触发时机 | 备注 |
| --- | --- | --- |
| COS 存储容量 | 图片存进桶 | 香港地域按 GB/月 |
| COS 请求次数 | 上传/读取/删除 | 很低 |
| **CDN 回源流量** | CDN 未命中缓存，回 COS 取数据 | 香港/境外单价高于境内 |
| **CDN 流量** | 用户从 CDN 节点下载 | 主要成本，建议买流量包 |
| CDN HTTPS 请求 | HTTPS 访问 | 按万次请求 |
| 数据万象图片处理 | 访问带 `imageMogr2` 的地址 | 按处理次数 + 流量，量小 |

省钱要点：**缓存时间拉长、开启智能压缩、列表页用缩略图**，命中率上去后回源流量会非常低。

---

## 4. 第一步：确认私有桶

控制台：<https://console.cloud.tencent.com/cos5> → 存储桶列表 → 你的桶。

逐项确认：

- [ ] **地域**：`ap-hongkong`（和服务器同地域，回源/上传走内网更快）
- [ ] **访问权限**：`私有读写`（不要改成公有读，本方案靠 CDN 回源鉴权读私有桶）
- [ ] **静态网站**：关闭（不需要）
- [ ] 记下**存储桶名称全称**：`<bucket>-<APPID>`
- [ ] 记下**默认访问域名**：`<bucket>-<APPID>.cos.ap-hongkong.myqcloud.com`

可选但建议：

- **生命周期**：给 `healthcheck/` 前缀加一条"7 天后删除"，自检残留会自动清理；
- **版本控制**：图片类桶一般不需要，开了会增加存储成本；
- **存储桶标签**：`app=yami-ai-api`，方便账单归集。

> 本项目**不会**在启动时创建桶，也不会修改桶策略（`MINIO_SKIP_BUCKET_INIT=true`、
> `MINIO_BUCKET_PRIVATE=true`），所有桶级配置都以控制台为准。

---

## 5. 第二步：子账号与最小权限

目标：拿到一对**只能读写这一个桶**的 `SecretId` / `SecretKey`，并且不开放控制台登录。

官方文档：[新建子用户](https://cloud.tencent.com/document/product/598/13674)、
[子账号访问密钥管理](https://cloud.tencent.com/document/product/598/37140)、
[授权子账号访问 COS](https://cloud.tencent.com/document/product/436/11714)。

> 前置：用**主账号**（或拥有 CAM 管理权限的管理员子账号）操作；过程中会要求主账号做一次
> 短信/微信身份验证。未实名认证的主账号 24 小时内最多创建 10 个子账号。

### 5.1 建议顺序

**先建策略 → 再建子用户 → 关联策略 → 建密钥。** 这样建用户时可以直接勾选策略，不用来回切页面。

### 5.2 创建子用户

1. 打开 [访问管理 CAM 控制台](https://console.cloud.tencent.com/cam) → 左侧 **用户 → 用户列表** → **新建用户**。
2. 选择 **自定义创建**（不要用"快速创建"，快速创建默认给 `AdministratorAccess`）。
3. 用户类型选 **可访问资源并接收消息** → 下一步。
4. 设置用户信息：
   - **用户名**：`cos-yami-media-rw`（带用途，方便审计）
   - **邮箱**：填一个能收信的邮箱（用于通知/绑定微信）；不需要通知时留空即可
   - **访问方式**：**只勾"编程访问"**，不要勾"腾讯云控制台访问"
     —— 不开放控制台登录，少一个被撞库的入口；日常看桶用主账号
5. 下一步 → 主账号身份验证（短信 / 微信扫码）。
6. 设置用户权限：策略已建好就勾选 5.3 的自定义策略；没建好就跳过，创建完再关联。
7. 用户标签（可选）：`app=yami-ai-api`。
8. 完成。

### 5.3 创建自定义策略（最小权限）

1. CAM 控制台 → **策略** → **新建自定义策略**。
2. 选择 **按策略语法创建**（或"按策略生成器创建"后切到 JSON）→ 模板选 **空白模板** → 下一步。
3. 策略名称：`yamiai-cos-yami-media-rw`；描述：`yami-ai-api 读写 yami-media 桶`。
4. 策略内容粘贴（三个值怎么找见 [第 0 节](#0-先填对三个值桶名--appid--地域)）：

```json
{
  "version": "2.0",
  "statement": [
    {
      "effect": "allow",
      "action": [
        "cos:GetObject",
        "cos:HeadObject",
        "cos:PutObject",
        "cos:DeleteObject",
        "cos:InitiateMultipartUpload",
        "cos:UploadPart",
        "cos:CompleteMultipartUpload",
        "cos:AbortMultipartUpload",
        "cos:ListParts",
        "cos:ListMultipartUploads"
      ],
      "resource": [
        "qcs::cos:ap-hongkong:uid/1250000000:yami-media-1250000000/*"
      ]
    }
  ]
}
```

5. 完成。

**四个必须注意的点：**

- `action` 用 **`服务:接口名`** 格式，即 `cos:GetObject`。
  COS 文档里出现的 `name/cos:GetObject` 是「存储桶策略 / 临时密钥」的写法，
  **CAM 自定义策略不要带 `name/` 前缀**，否则权限不生效或保存报语法错误。
- `resource` 的账号段用 **`uid/<APPID>`**（APPID = 桶名后缀那串数字，不是 UIN），
  地域段必须是桶的地域 `ap-hongkong`。
- 不要图省事写 `"resource": "*"` 或 `"action": ["cos:*"]`，那等于把这台服务器上的密钥
  变成整个账号对象存储的万能钥匙。
- 各权限的用途：`GetObject`（读 + 图片处理）、`HeadObject`（探测）、`PutObject`（上传）、
  `DeleteObject`（删作品）、`Multipart*`（>16MB 的大图/视频分片上传）；
  不需要 `GetBucket` / `HeadBucket`，因为项目启动时跳过了桶检查
  （若把 `MINIO_SKIP_BUCKET_INIT` 改成 `false`，必须补上这两个桶级权限）。

### 5.4 关联策略到子用户

两种入口效果一样：

- 用户列表 → 找到 `cos-yami-media-rw` → 操作列 **授权** → 策略类型选 **自定义策略** →
  勾选 `yamiai-cos-yami-media-rw` → 确定；
- 或 策略列表 → 找到策略 → **关联用户/组/角色** → 勾选该用户 → 确定。

> 没有关联策略的子用户 = 没有任何权限，调用会直接 403。

### 5.5 创建 API 密钥

1. 用户列表 → 点用户名进入详情 → **API 密钥** 页签 → **新建密钥**。
2. 弹窗显示 `SecretId` 和 `SecretKey`。**SecretKey 只显示这一次**
   （2023-11-30 起腾讯云关闭了 SecretKey 查询），立刻复制到密码管理器。
3. 一个子用户最多两把密钥，建议留一把备用方便轮换。

写入服务器 `.env.production`（权限 `600`，不要提交 git）：

```dotenv
MINIO_ACCESS_KEY=<SecretId>
MINIO_SECRET_KEY=<SecretKey>
```

### 5.6 验证

用项目自带的 `storage-check`，并加 `-no-db` 忽略数据库里的旧存储配置：

```bash
CMP="docker compose -f infra/docker/docker-compose.prod.yml --env-file .env.production"
$CMP run --rm \
  -e MINIO_ENDPOINT=cos.ap-hongkong.myqcloud.com \
  -e MINIO_REGION=ap-hongkong \
  -e MINIO_ACCESS_KEY=<SecretId> \
  -e MINIO_SECRET_KEY=<SecretKey> \
  -e MINIO_BUCKET=yami-media-1250000000 \
  -e MINIO_PUBLIC_URL=https://img.orbit-e.com \
  -e MINIO_PUBLIC_URL_STYLE=object \
  -e MINIO_BUCKET_PRIVATE=true \
  -e MINIO_SKIP_BUCKET_INIT=true \
  -e MINIO_USE_SSL=true \
  -e MINIO_BUCKET_LOOKUP=dns \
  api /storage-check -no-db
```

期望：初始化成功 → 上传成功 → 服务端读回一致 →（公网地址 ✅ 或 ⚠️）→ 删除成功。

也可以在 CAM 控制台 → 用户详情 → API 密钥 → **更多访问记录** 查看最近的调用记录。

### 5.7 可选加固

**IP 白名单（推荐）**：让密钥只能从你的服务器出口 IP 使用。

先确认出口 IP：

```bash
curl -s https://ifconfig.me
```

然后在策略 statement 里加 `condition`：

```json
"condition": {
  "ip_equal": {
    "qcs:ip": ["124.156.112.244/32"]
  }
}
```

注意：换服务器、加机器、走代理出口时都要同步改，否则 API 会 403；不确定就先别加。

**密钥轮换**：新建第二把密钥 → 更新 `.env.production` → 重启 api/worker → 观察 24 小时 →
在 CAM 删除旧密钥。SecretKey 泄露时这是唯一正确的处理方式（不能"查询/修改"，只能新建 + 删除）。

### 5.8 常见错误

| 现象 | 原因 |
| --- | --- |
| 403 AccessDenied，但控制台里看着都配了 | 策略没关联到子用户，或 `resource` 里 APPID/桶名写错 |
| 403 且日志提示 `SignatureDoesNotMatch` | 密钥含空格/换行；或 `MINIO_REGION` 与桶地域不一致 |
| 小图能传，传视频/大图 403 | 少了 `InitiateMultipartUpload` / `UploadPart` / `CompleteMultipartUpload` |
| 启动日志 `object storage unavailable` | `MINIO_SKIP_BUCKET_INIT=false` 但子账号没有 `HeadBucket` 权限 |
| 策略保存报语法错误 | `action` 写成了 `name/cos:GetObject`，或 JSON 里带了注释/中文标点 |
| SecretKey 找不到了 | 无法找回，只能新建一把并删除旧密钥 |

> 再次提醒：**不要给子账号 `QcloudCOSFullAccess` 或 `AdministratorAccess`**，
> 那等于把整个账号下所有桶（包括其他业务）都交给这台服务器上的密钥。

---

## 6. 第三步：配置自定义 CDN 加速域名（核心）

官方文档：[开启自定义 CDN 加速域名](https://cloud.tencent.com/document/product/436/36637)、
[CDN 加速配置](https://cloud.tencent.com/document/product/436/18670)、
[CDN 加速概述](https://cloud.tencent.com/document/product/436/18669)。

### 6.1 添加域名

路径：存储桶 → **域名与传输管理** → **自定义 CDN 加速域名** → **添加域名**。

| 配置项 | 填什么 | 说明 |
| --- | --- | --- |
| 域名 | `img.orbit-e.com` | 已备案的话可直接用；未备案只能选中国境外加速 |
| 加速地域 | `中国境外`（未备案）/ `中国境内`或`全球加速`（已备案） | **决定是否需要备案** |
| 源站类型 | `默认源站` | 不要选静态网站源站 |
| 回源鉴权 | **开启** | 私有桶必须开，否则 CDN 拿不到任何数据 |

如果控制台提示需要 **添加 CDN 服务授权**，先点授权：这会把"腾讯云 CDN 服务身份"
加入该桶的访问策略（授予 `GetObject` / `HeadObject` / `OptionsObject`），
这是私有桶能被 CDN 回源读取的前提。

> ⚠️ 安全模型说明：
> - **回源鉴权 = 开**：CDN 能读私有桶，源站不暴露；
> - **CDN 鉴权（URL 签名）= 先关**：灵感广场是公开内容，地址必须长期有效、可直接分享；
>   开启后所有图片 URL 都要带时间戳签名，前端、数据库、CDN 缓存都要跟着改。
> - 效果等同于官方说的"源站保护"模式：**COS 域名私有 + CDN 域名公开**。
>   如果以后有付费/私密图，单独走带签名的 COS 预签名 URL，不要和公开图混用同一个域名。

创建后，列表里会显示一条形如 `img.orbit-e.com.cdn.dnsv1.com` 的 **CNAME 值**（以控制台为准）。

### 6.2 配置 DNS 解析

在 DNS 服务商（DNSPod / Cloudflare / 阿里云解析）添加：

```text
记录类型：CNAME
主机记录：img
记录值：  img.orbit-e.com.cdn.dnsv1.com   ← 填控制台给出的值
TTL：     600（Cloudflare 选 Auto 即可）
```

#### Cloudflare 特别注意

`orbit-e.com` 的 NS 在 Cloudflare 时，最容易犯的错误是把记录开成**橙色云朵（Proxied）**。
这个域名必须交给腾讯云 CDN 直接对外，Cloudflare 只能做 DNS，不能代理：

| 字段 | 填什么 |
| --- | --- |
| Type | `CNAME` |
| Name | `img` |
| Target | 腾讯云控制台给出的 CNAME 值（不要带 `https://`、不要带结尾的点） |
| Proxy status | **DNS only（灰色云朵）** ← 关键 |
| TTL | Auto |

操作：Cloudflare → 选择 `orbit-e.com` → 左侧 **DNS → Records** → **Add record** →
按上表填写 → **Save**。

为什么必须灰色云朵：

- 橙色云朵会让 Cloudflare 先终止 TLS 再回源到腾讯云 CDN，多一跳，且腾讯云的
  域名归属校验/证书签发可能失败；
- 腾讯云 CDN 看到的都是 Cloudflare 节点 IP，回源鉴权、缓存命中、日志都会失真；
- 常见症状是 `522` / `525` / `526`，或者浏览器证书显示 Cloudflare 的而不是腾讯云的。

如果已经加成橙色云朵：点该记录的 **Edit** → Proxy status 切到 **DNS only** → Save。

验证：

```bash
dig +noall +answer img.orbit-e.com
# 期望：CNAME 指向 ...cdn.dnsv1.com，随后是 CDN 节点 IP

dig @8.8.8.8 +short img.orbit-e.com
# 期望不再是空（NXDOMAIN）

curl -I https://img.orbit-e.com/            # 证书配好后应返回 4xx（而不是连接失败）
```

> 域名刚添加时状态是"部署中"，一般 5–30 分钟变"已上线"。CNAME 没生效前访问会失败，属正常。
> Cloudflare 的 DNS 通常几十秒内生效，但公共 DNS 缓存可能要几分钟。

### 6.3 配置 HTTPS 证书

路径：CDN 控制台 → 域名管理 → 找到 `img.orbit-e.com` → **HTTPS 配置**。

#### 先搞清楚「HTTPS 服务」收不收费

- **HTTPS 服务开关本身按请求数计费**：每个账号每月有 **300 万次** HTTPS 请求免费额度，
  超出部分 **0.05 元/万次**（后付费）。300 万次以内实际费用为 0。
- **证书本身可以免费**（腾讯云免费 DV 证书），收费的是超出额度的 HTTPS 请求数。
- 如果把「HTTPS 服务」关掉，HTTPS 请求会被拒绝并返回 **514**，且不产生 HTTPS 费用。
  本项目必须保持开启，否则浏览器加载图片会失败。
- 官方计费说明：[CDN 增值服务计费](https://cloud.tencent.com/document/product/228/75563)。

#### 为什么 CDN 控制台里看不到「免费证书」

CDN 的「配置证书」弹窗只列出 **SSL 控制台里已经存在的证书**（腾讯云托管证书）。
免费证书要先在 SSL 控制台申请，签发后才会出现在这个列表里。

#### 申请免费 DV 证书（90 天）

1. 打开 [SSL 证书控制台](https://console.cloud.tencent.com/ssl) → **我的证书** → **申请免费证书**。
2. 证书类型选免费 DV，域名填 **`img.orbit-e.com`**（单域名，免费证书不支持通配符）。
3. 验证方式选 **DNS 验证**（推荐，域名还没解析到 CDN 时也能过）：
   - 腾讯云会给一条 TXT 记录（主机记录形如 `_dnsauth.img`，记录值是一长串字符）；
   - 到 Cloudflare → DNS → Add record：Type `TXT`、Name 填腾讯云给的主机记录、
     Content 填记录值、TTL Auto；
   - 回腾讯云点提交，等待签发（通常几分钟到几小时，官方说一个工作日内）。
4. 签发后回到 CDN 控制台 → `img.orbit-e.com` → **HTTPS 配置** → **配置证书** →
   证书来源选 **腾讯云托管证书** → 选中刚签发的证书 → 确定。
5. 证书部署约 30 分钟内生效；生效后再打开 **HTTP/2** 和 **强制跳转 HTTPS**。

> ⚠️ 免费证书有效期只有 **90 天，且不支持续费**，到期要重新申请并重新绑定。
> 正式对外运营建议换成一年期付费 DV 证书（控制台「升级」或购买页），
> 或者用 certbot 申请 Let's Encrypt 证书后以「自有证书」方式上传并做自动续期。
> 免费证书如果用，务必在到期前 10 天设提醒。

验证：

```bash
curl -I https://img.orbit-e.com/
# 期望：HTTP/2 200 或 403/404（说明 TLS 握手成功），不是 SSL 错误

openssl s_client -connect img.orbit-e.com:443 -servername img.orbit-e.com </dev/null 2>/dev/null \
  | openssl x509 -noout -subject -dates
# 期望 subject 里有 img.orbit-e.com，日期未过期
```

### 6.4 缓存配置（决定命中率和成本）

CDN 控制台 → 域名管理 → `img.orbit-e.com` → **缓存配置**。

建议规则：

| 类型 | 内容 | 缓存时间 |
| --- | --- | --- |
| 文件类型 | `jpg;jpeg;png;webp;avif;gif;svg` | 30 天 |
| 文件类型 | `mp4;webm;mov` | 7 天 |
| 文件类型 | `json;js;css;html` | 1 小时（本项目基本不用 CDN 发这些） |
| 全部文件（兜底） | `*` | 1 天 |

**两个必须注意的点：**

1. **不要开启"忽略参数"**（或"忽略 URL 参数"）。缩略图靠 `?imageMogr2/...` 区分，
   忽略参数会导致不同尺寸的图互相覆盖缓存。
2. **删除/覆盖图片后 CDN 不会自动失效**。项目删除作品时只删 COS 对象；
   如需立刻生效，去 CDN 控制台 **刷新缓存**（按 URL 或目录），
   或者给桶配置"CDN 缓存自动刷新"（函数计算，按需）。

#### 回源配置里唯一要改的：分片回源

CDN 控制台 → 域名 → **回源配置**，其他项保持默认即可，但 **分片回源要关掉**。

页面自己就写了提示：

> 若您的资源都是静态小文件，或域名源站为 COS 源站且已使用数据处理类功能
> （例如：图片处理），不建议开启分片回源，开启后会影响回源。

本项目要用 `imageMogr2` 生成缩略图（数据万象图片处理），
**分片回源（Range 回源）会和图片处理冲突**，表现为部分缩略图回源失败或返回原图。
所以：分片回源 → 全部文件 → **修改 → 关闭**。

其余回源项：

| 配置 | 建议 | 说明 |
| --- | --- | --- |
| 合并回源 | 保持关闭 | 高并发下减少回源请求，图片场景收益有限，可不开 |
| 回源 HTTP 请求头 | 保持关闭 | 默认规则不影响 COS 源站 |
| 回源跟随 301/302 | 保持开启 | COS 跨地域重定向时能跟随 |
| 回源超时 | 默认 5s/10s；大图多可改 10s/30s | 原图较大时避免误超时 |
| 回源 URL 重写 | 必须留空 | CDN 路径直接对应对象 key，重写会导致 404 |
| 回源 SNI | 保持关闭 | COS 默认域名不需要 |

### 6.5 配置 CORS（必做，否则画布导出会失败）

本项目有多处会把图片画进 `<canvas>` 再导出（作品页封面、参考版导出、视频封面抽帧），
代码里设置了 `img.crossOrigin = "anonymous"`。如果 CDN 不返回 CORS 头，
画布会被"污染"，导出直接报错。

1. COS 控制台 → 桶 → **安全管理** → **跨域访问 CORS 设置** → 添加规则：

```text
来源 Origin：  https://www.orbit-e.com
              https://admin.orbit-e.com
              http://localhost:3000        （本地开发可选）
操作 Methods：GET, HEAD
Allow-Headers：*
Expose-Headers：ETag, Content-Length, Content-Range, Content-Type
超时 Max-Age：86400
```

2. CDN 控制台 → 域名 → **高级配置 → HTTP 响应头配置**（不同版本入口名可能叫
   "跨域配置"/"响应头"）追加：

```text
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, HEAD, OPTIONS
```

> 想收紧安全就把 `*` 换成站点域名；`crossOrigin="anonymous"` 不带 Cookie，
> 用 `*` 也能正常工作。

验证：

```bash
curl -sI -H "Origin: https://www.orbit-e.com" https://img.orbit-e.com/<任意已存在对象> \
  | grep -i access-control
```

### 6.6 压缩与图片自适应（可选，但强烈建议）

- **CDN 智能压缩**：控制台默认会创建 gzip 规则，对 `.js/.html/.css/.json` 生效，图片无感，保持开启即可。
- **图片自适应压缩**：COS 控制台 → 域名列表 → 个性化配置 → 一键开启。
  浏览器支持 WebP/AVIF 时自动下发压缩格式。**会产生数据万象费用**，按量很小。
- 若开启了它，本文第 11 节的 `imageMogr2` 缩略图依然有效，两者不冲突。

### 6.7 这一节的验收

- [ ] 域名状态"已上线"
- [ ] `dig +short img.orbit-e.com` 有结果
- [ ] `curl -I https://img.orbit-e.com/` TLS 正常
- [ ] CORS 头存在
- [ ] 回源鉴权 = 开启，CDN 鉴权 = 关闭（当前阶段）

---

## 7. 第四步：配置自定义源站域名（可选）

官方文档：[开启自定义源站域名](https://cloud.tencent.com/document/product/436/36638)。

### 7.1 什么时候需要

- 你要用**第三方 CDN**（Cloudflare 等）回源到 COS，需要一个固定源站域名；
- 你想给运维/内部工具一个稳定域名直连桶（带签名访问）；
- 你不想依赖 COS 默认域名（换桶时域名不变）。

**不需要**的情况：只用腾讯云 CDN + 本项目上传下载 —— 跳过本节即可。

### 7.2 配置步骤

路径：存储桶 → **域名与传输管理** → **自定义源站域名** → **添加域名**。

| 配置项 | 填什么 |
| --- | --- |
| 域名 | `cos-origin.orbit-e.com` |
| 源站类型 | 默认源站 |
| 强制 HTTPS | 开启 |

然后在 DNS 添加：

```text
记录类型：CNAME
主机记录：cos-origin
记录值：  yami-media-1250000000.cos.ap-hongkong.myqcloud.com
```

证书：在该域名上"绑定证书"（COS 支持上传自有证书或托管证书）。

### 7.3 私有桶的限制（重要）

- 通过自定义源站域名访问私有桶对象，**必须带 COS 签名**，否则 403；
  浏览器直接打开 `https://cos-origin.orbit-e.com/xxx.png` 会失败。
- 本项目目前**没有**"生成 COS 预签名 URL"的接口，所以：
  - 前台/后台的图片地址**不要**指向自定义源站域名；
  - 用户侧统一用 `img.orbit-e.com`（CDN 加速域名）。
- 如果拿它给第三方 CDN 回源：该 CDN 必须能对每个回源请求签名（例如 Cloudflare Worker
  用 COS 密钥计算签名），否则就只能把桶改成公有读 —— 不推荐。

### 7.4 验收

- [ ] `dig +short cos-origin.orbit-e.com` 指向 COS 默认域名
- [ ] 带签名的 URL 能下载（可选）
- [ ] 未签名 URL 返回 403（符合私有桶预期）

---

## 8. 第五步：改项目配置并自检

### 8.1 修改 `.env.production`

```dotenv
# ---- 对象存储：腾讯云 COS 私有桶 + 自定义 CDN 加速域名 ----
MINIO_ENDPOINT=cos.ap-hongkong.myqcloud.com
MINIO_REGION=ap-hongkong
MINIO_ACCESS_KEY=<子账号 SecretId>
MINIO_SECRET_KEY=<子账号 SecretKey>
MINIO_BUCKET=yami-media-1250000000
MINIO_PUBLIC_URL=https://img.orbit-e.com
MINIO_USE_SSL=true
# 关键：CDN 域名直接映射对象，URL 不带 bucket 前缀
MINIO_PUBLIC_URL_STYLE=object
# 关键：私有桶不写公有读策略，启动时也不检查/创建桶
MINIO_BUCKET_PRIVATE=true
MINIO_SKIP_BUCKET_INIT=true
# COS 必须用虚拟主机风格；新代码会按 *.myqcloud.com 自动识别，
# 旧镜像或自定义 endpoint 时显式写 dns
MINIO_BUCKET_LOOKUP=dns

# ---- 前端：让灵感广场列表使用 CDN 缩略图（构建时内联）----
NEXT_PUBLIC_MEDIA_CDN_HOSTS=img.orbit-e.com
```

> `MINIO_ENDPOINT` 不要带 `https://`。同地域 CVM 访问该域名通常会被解析到内网 IP
> （用 `dig cos.ap-hongkong.myqcloud.com` 确认），上传不占公网带宽。

### 8.2 检查数据库里的存储配置（很容易踩坑）

后台「系统配置 → 存储」的值保存在 `system_configs` 表，**优先级高于环境变量**。
如果之前在里面填过 MinIO 地址，改 `.env.production` 不会生效。

先查：

```bash
docker compose -f infra/docker/docker-compose.prod.yml --env-file .env.production \
  exec -T postgres psql -U starai -d starai -c \
  "SELECT key, left(value::text, 80) FROM system_configs WHERE key LIKE 'storage_%' ORDER BY key;"
```

两种处理方式，**二选一**：

**方式 A（推荐）：删掉数据库覆盖项，让环境变量成为唯一来源**

```bash
docker compose -f infra/docker/docker-compose.prod.yml --env-file .env.production \
  exec -T postgres psql -U starai -d starai -c \
  "DELETE FROM system_configs WHERE key IN
   ('storage_provider','storage_endpoint','storage_access_key','storage_secret_key',
    'storage_bucket','storage_public_url','storage_use_ssl');"
```

**方式 B：在后台把这几项改成 COS 的值**

`Endpoint=cos.ap-hongkong.myqcloud.com`、`Bucket=yami-media-1250000000`、
`Access Key/Secret Key`、`公开访问域名=https://img.orbit-e.com`、`使用 HTTPS=是`。
注意：`MINIO_PUBLIC_URL_STYLE` / `MINIO_REGION` / `MINIO_BUCKET_PRIVATE` /
`MINIO_SKIP_BUCKET_INIT` 仍然只能写在 `.env.production` 里。

### 8.3 重新构建并启动

> ⚠️ Orbit-E 这台服务器还有 `infra/docker/docker-compose.prod.override.yml`
> （接入 APISIX 网关网络 + 固定容器 IP）。**所有 compose 命令都必须同时带两个 `-f`**，
> 否则重建后的 api/web 会脱离 `172.28.x` 网络，APISIX 直接返回 502。
> 仓库里的 `scripts/deploy-prod.sh` 已改成自动识别 override，用脚本发布最稳。

```bash
cd /home/ubuntu/yard-api/yami-ai-api
CMP="docker compose -f infra/docker/docker-compose.prod.yml \
  -f infra/docker/docker-compose.prod.override.yml --env-file .env.production"

# web 的 NEXT_PUBLIC_* 是构建期内联的，必须重建
$CMP build api worker web
$CMP up -d api worker web

# 看启动日志有没有存储告警（不应出现 "object storage unavailable"）
$CMP logs --tail=50 api worker | grep -i -E "storage|minio|对象存储" || true
```

> `admin` 只有提示文案变化，不重建也能用。

### 8.4 用 `storage-check` 自检（本仓库新增）

它会在不启动业务的情况下依次验证：连接 → 签名 → 上传 → 服务端读回 → 公网访问 → 删除。

**切换前预演（忽略数据库配置，只用命令行传入的变量）**：

```bash
docker compose -f infra/docker/docker-compose.prod.yml --env-file .env.production \
  run --rm \
  -e MINIO_ENDPOINT=cos.ap-hongkong.myqcloud.com \
  -e MINIO_REGION=ap-hongkong \
  -e MINIO_ACCESS_KEY=<SecretId> \
  -e MINIO_SECRET_KEY=<SecretKey> \
  -e MINIO_BUCKET=yami-media-1250000000 \
  -e MINIO_PUBLIC_URL=https://img.orbit-e.com \
  -e MINIO_PUBLIC_URL_STYLE=object \
  -e MINIO_BUCKET_PRIVATE=true \
  -e MINIO_SKIP_BUCKET_INIT=true \
  -e MINIO_USE_SSL=true \
  -e MINIO_BUCKET_LOOKUP=dns \
  api /storage-check -no-db
```

期望输出：

```text
✅ 存储客户端初始化成功
✅ 上传成功
   url = https://img.orbit-e.com/healthcheck/storage-check-....png
✅ 服务端读回一致（API/Worker 读写链路可用）
✅ 公网地址可访问（浏览器可以直接加载）
✅ 删除成功，自检结束
```

如果公网那一行是 ⚠️，但上传/读回都 ✅，说明 COS 侧已经通了，问题只在 CDN
（CNAME、证书、回源鉴权），按 14 节排查。

**切换后复检**（用容器真实环境 + 数据库配置）：

```bash
docker compose -f infra/docker/docker-compose.prod.yml --env-file .env.production \
  exec api /storage-check
```

### 8.5 页面级验收

1. 打开灵感广场，Network 里图片地址应为 `https://img.orbit-e.com/...`；
2. 列表页图片 URL 带 `?imageMogr2/...`（说明缩略图生效，见第 11 节）；
3. 上传一张图（作品页 / 素材管理），返回值是 CDN 地址，刷新后仍能显示；
4. 打开一张图 → 详情 → 仍是原图（不带 imageMogr2）；
5. 画布导出/参考版导出不报错（验证 CORS）。

---

## 9. 第六步：存量图片迁移

新上传的图会自动进 COS，但**历史图片还在旧的 MinIO / 本地目录**，而数据库里的 URL 也还是旧地址。
顺序是：**先同步对象 → 再改 URL → 最后切换流量**。

### 9.1 同步对象

#### 方式 A：内置 `storage-migrate`（推荐）

仓库自带迁移命令，已打进 api 镜像。它**只列源桶**，目标端只做 `HeadObject/PutObject`，
不依赖 COS 子账号的 ListBucket 权限，私有桶也能用；支持 dry-run、并发、跳过已存在、独立校验。

```bash
CMP="docker compose -f infra/docker/docker-compose.prod.yml \
  -f infra/docker/docker-compose.prod.override.yml --env-file .env.production"

# 源端（旧 MinIO）显式传参；目标端默认读容器里的 MINIO_*（COS）
$CMP run --rm \
  -e SRC_ENDPOINT=minio:9000 \
  -e SRC_ACCESS_KEY=<旧 MinIO AK> \
  -e SRC_SECRET_KEY=<旧 MinIO SK> \
  -e SRC_BUCKET=starai-works \
  -e SRC_USE_SSL=false \
  -e SRC_LOOKUP=path \
  api /storage-migrate -dry-run

# 确认清单后正式复制
$CMP run --rm <同上 -e 参数> api /storage-migrate -concurrency 8

# 校验目标对象是否齐全（缺失/大小不一致会返回非 0）
$CMP run --rm <同上 -e 参数> api /storage-migrate -verify
```

说明：

- `docker compose run` 不会重启业务容器，可以在线迁移；
- 源端是 MinIO 时用 `SRC_LOOKUP=path`；COS 域名会按 `*.myqcloud.com` 自动用虚拟主机风格；
- 迁移保持原 key 不变，所以数据库只需要替换 URL 前缀（9.2）；
- 迁移工具会读取源对象的 `Content-Type`，避免 COS 里图片变成 `application/octet-stream`。

#### 方式 B：rclone（备选）

先确认旧存储的可访问地址：宿主机 `127.0.0.1:9000`，或容器网络里的 `minio:9000`。

```bash
# 查 compose 网络名
docker network ls | grep -i starai
```

在宿主机用 rclone（推荐用容器跑，不污染宿主机）：

```bash
# 1) 旧 MinIO
docker run --rm -it --network starai_default \
  -v ~/.config/rclone:/config/rclone \
  rclone/rclone config create old s3 \
    provider=Minio endpoint=http://minio:9000 \
    access_key_id=<旧MinIO AK> secret_access_key=<旧MinIO SK> \
    force_path_style=true

# 2) 新 COS
docker run --rm -it --network starai_default \
  -v ~/.config/rclone:/config/rclone \
  rclone/rclone config create cos s3 \
    provider=Other endpoint=https://cos.ap-hongkong.myqcloud.com \
    access_key_id=<SecretId> secret_access_key=<SecretKey> \
    region=ap-hongkong force_path_style=true

# 3) 全量复制（桶名替换成实际值）
docker run --rm -it --network starai_default \
  -v ~/.config/rclone:/config/rclone \
  rclone/rclone copy old:starai-works cos:yami-media-1250000000 \
    --transfers 8 --checkers 16 --progress

# 4) 校验（数量 + 大小）
docker run --rm -it --network starai_default \
  -v ~/.config/rclone:/config/rclone \
  rclone/rclone check old:starai-works cos:yami-media-1250000000 --size-only
```

> 旧数据如果在本地目录（`LOCAL_STORAGE_DIR`），直接用 `rclone copy /path/to/uploads cos:bucket`
> 把目录内容按原 key 结构传上去即可（key 就是相对路径）。

### 9.2 改写数据库里的 URL

先确认旧地址长什么样（可能是相对路径，也可能是完整域名）：

```bash
docker compose -f infra/docker/docker-compose.prod.yml --env-file .env.production \
  exec -T postgres psql -U starai -d starai -c \
  "SELECT thumbnail_url FROM works WHERE thumbnail_url LIKE '%minio%' OR thumbnail_url LIKE '/media/%' LIMIT 5;"
```

然后编辑 `scripts/rewrite-media-urls.sql` 顶部的三个变量：

```sql
v_old_prefix text := 'https://www.orbit-e.com/minio-store/starai-works/';
v_new_prefix text := 'https://img.orbit-e.com/';
v_dry_run    boolean := true;
```

- 旧地址是相对路径时，`v_old_prefix` 用 `/media/starai-works/`；
- 保持 `v_dry_run = true` 先跑，确认 NOTICE 里的命中行数合理（不会误伤）；
- 再改成 `false` 执行。

执行：

```bash
docker compose -f infra/docker/docker-compose.prod.yml --env-file .env.production \
  cp scripts/rewrite-media-urls.sql postgres:/tmp/rewrite-media-urls.sql

docker compose -f infra/docker/docker-compose.prod.yml --env-file .env.production \
  exec -T postgres psql -U starai -d starai -v ON_ERROR_STOP=1 \
  -f /tmp/rewrite-media-urls.sql
```

脚本会遍历 `public` 下所有 text/varchar/jsonb/json 列，打印每个命中列的
"表.列: 行数"，最后给出总行数；重复执行是幂等的。

> 必须先 `pg_dump` 备份，或确认能接受回滚到备份点。

### 9.3 切换与观察

1. 完成第 8 节的 env + 重启；
2. 打开灵感广场，确认图片 200 且来自 `img.orbit-e.com`；
3. 观察 24 小时：API 日志无存储告警、CDN 命中率上升、COS 无异常 4xx；
4. 旧 MinIO **先不要删**，保留 1–2 周作为回滚副本。

---

## 10. 上传链路：为什么传完就在 COS 里

项目里所有上传统一走服务端（API/Worker），不存在"传到服务器本地"的分支：

| 入口 | 代码位置 | 上限 | 落库 |
| --- | --- | --- | --- |
| 作品/聊天图片 `POST /api/upload` | `internal/handler/handler.go` `Upload` | 10 MB | 只返回 URL |
| 素材库 `POST /api/assets/upload` | `UploadAsset` | 20 MB | `assets` 表存 bucket + object_key |
| URL 导入 `POST /api/assets/import-url` | `ImportAssetURL` | — | 先下载再上传 |
| AI 生成结果转存 | worker `persistGeneratedMedia` | 按任务 | works/metadata |
| 灵感广场参考图镜像 | `cmd/gallery-sync` | 50 MB | `gallery_reference_cases.image_asset_url` |
| 后台图片导入 `POST /api/admin/upload` | `AdminUpload` | 10 MB | 只返回 URL |

上传过程：

```text
浏览器 ──multipart──▶ API ──PutObject(S3 API + 密钥)──▶ COS
                        │
                        └─ 返回 https://img.orbit-e.com/<key>
```

所以：

- 用户上传后，**对象直接落在 COS**，服务器只做一次中转（不落盘、不长期保存）；
- 返回的地址就是 CDN 地址，浏览器立刻可访问；
- 删除作品时，`DeleteWithStorage` 用同一个 storage client 删 COS 对象；
- 大文件（>16MB）自动走分片上传，需要第 5 节里的 Multipart 权限。

**关于"直传 COS"**：如果你以后想彻底省掉服务器中转，可以让后端签发预签名 PUT URL，
浏览器直传 COS。代价是要处理 CORS、上传鉴权、失败重试、孤儿对象清理，
当前流量规模不建议做；本方案的瓶颈已经是"下载慢"，不是"上传慢"。

---

## 11. 让灵感广场真正变快：缩略图

只上 CDN 不够：灵感广场列表以前直接加载原图，单张常见 2–8 MB。
本次改造后：

- 列表卡片：`https://img.orbit-e.com/xxx.png?imageMogr2/thumbnail/800x/format/webp/quality/82/ignore-error/1`
- 详情弹窗：仍加载原图；
- 只有命中 `NEXT_PUBLIC_MEDIA_CDN_HOSTS` 的地址才会加参数，其他图床（GitHub raw 等）行为不变。

### 11.1 前置：开通数据万象图片处理

首次访问带 `imageMogr2` 的地址时，COS 会**自动开通并把数据万象绑定到该桶**
（[官方说明](https://cloud.tencent.com/document/product/460/36540)），按处理次数计费。
建议先手动验证一张：

```bash
curl -sI "https://img.orbit-e.com/<任意图片key>?imageMogr2/thumbnail/480x/format/webp/quality/82" \
  | grep -i -E "content-type|content-length|x-cos"
# 期望 content-type: image/webp，content-length 明显小于原图
```

如果返回原图大小或 4xx：

- 确认桶已绑定数据万象（COS 控制台 → 数据处理 → 图片处理）；
- 确认子账号有 `cos:GetObject`（处理走下载时处理，用的是 GetObject 权限）；
- 确认 CDN **没有开启"忽略参数"**。

### 11.2 前端配置

`.env.production`：

```dotenv
NEXT_PUBLIC_MEDIA_CDN_HOSTS=img.orbit-e.com
```

重新构建 web（`NEXT_PUBLIC_*` 是构建期内联）：

```bash
docker compose -f infra/docker/docker-compose.prod.yml --env-file .env.production \
  build web && docker compose ... up -d web
```

### 11.3 缓存与成本

- 同一张图最多 7 档宽度（320/480/640/750/960/1280/1600），避免缓存碎片；
- CDN 缓存 30 天后，缩略图基本不回源；
- 想进一步降本：把列表宽度调小（`GalleryPanel.tsx` 里 `mediaThumbnailURL(..., 800)`），
  或把 `quality` 从 82 调到 78。

---

## 12. 验收清单

复制到工单/备忘录逐条勾：

**存储与权限**

- [ ] `storage-check` 全部 ✅（上传、读回、公网、删除）
- [ ] COS 桶仍是"私有读写"
- [ ] 子账号只有目标桶的读写权限，无 `*` 通配
- [ ] `system_configs` 没有残留的旧存储配置

**CDN**

- [ ] `dig img.orbit-e.com` 解析到 CDN
- [ ] HTTPS 证书有效、强制 HTTPS 已开
- [ ] 回源鉴权开启、CDN 鉴权关闭（当前阶段）
- [ ] 缓存规则已配、未开启"忽略参数"
- [ ] CORS 头存在，画布导出正常

**业务**

- [ ] 灵感广场列表图片全部来自 `img.orbit-e.com`，且列表请求带 `imageMogr2`
- [ ] 详情页加载原图
- [ ] 上传图片 → 返回值是 CDN 地址 → 刷新可见
- [ ] AI 生成任务完成后，结果图能显示（worker 转存成功）
- [ ] 删除作品后，COS 对象被删、CDN 刷新后 404
- [ ] 视频可拖动进度条（Range 正常）

**性能**

- [ ] 灵感广场首屏 LCP 明显下降（Network 里单图 < 200 KB）
- [ ] CDN 缓存命中率 > 90%（CDN 控制台 → 统计数据）
- [ ] 源站（COS）回源流量平稳

---

## 13. 回滚方案

按"影响面从小到大"：

1. **只回滚 URL 形态**：把 `MINIO_PUBLIC_URL_STYLE` 改回 `bucket`，
   同时 `MINIO_PUBLIC_URL` 改成带桶前缀的地址 —— 只影响新上传，历史 URL 不变。
2. **回滚到旧存储**：`.env.production` 恢复旧 `MINIO_*`，重启 api/worker；
   数据库里已改写的 CDN URL 会失效 —— 需要用备份恢复，或者把旧对象也同步到 CDN 域名下。
   **所以第 9 节的数据库备份是回滚的前提。**
3. **CDN 出问题**：在 COS 控制台把 `img.orbit-e.com` 状态改为"下线"，
   同时把 `MINIO_PUBLIC_URL` 指向 COS 默认域名（私有桶下浏览器打不开，只适合临时排查）。
4. **域名被 DDoS/刷流量**：CDN 控制台开启 IP 访问限频、带宽封顶、URL 鉴权；
   紧急时可下线域名。

---

## 14. 常见问题

### Q1. 图片 403

先看响应头，能直接区分是哪一层拒绝的：

| 特征 | 含义 |
| --- | --- |
| `server: tencent-cos` + 响应体 `<Code>AccessDenied</Code>` | **CDN 回源被 COS 拒绝**，即 CDN 没有带上私有桶的合法身份 |
| `server: tencent-cos` + `<Code>NoSuchKey</Code>` | 鉴权已通，但对象不存在（检查 key / 迁移） |
| `server: nws` 或 CDN 自己的错误页 | CDN 层的问题（域名/回源地址/端口） |
| `x-cache-lookup: Cache Miss` | 是实时回源，不是缓存命中；说明问题在源站请求 |

> ⚠️ **不要用 `curl -I https://img.orbit-e.com/` 判断是否修好**：
> 根路径 `/` 在 COS 里是 `GetBucket`（列桶）操作，而 CDN 服务授权只给了
> `GetObject / HeadObject / OptionsObject`，没有 `GetBucket`，所以 `/` 永远返回 403。
> 用真实对象地址，或直接跑 `storage-check`。

如果确认是 `AccessDenied`，按顺序处理：

1. COS 控制台 → 桶 → **域名与传输管理 → 自定义 CDN 加速域名** →
   找到域名 → **鉴权** 列，确认「回源鉴权」是**已开启**；
2. 如果页面顶部出现红色提示 **「当前存在域名开启了回源鉴权，但该存储桶未开启
   CDN 服务授权，点击添加 CDN 服务授权」**，点它完成 CAM 服务角色授权
   （需要主账号或管理员）；只开回源鉴权但不授权，CDN 依然是匿名回源，仍然 403；
3. 也可以从 CDN 控制台 → 域名 → **回源配置** 检查：源站类型必须是 **COS 源站**
   （不是普通域名），并开启回源鉴权；
4. 如果 CDN 的源站类型是「域名」，说明域名不是按私有桶方式接入的，
   最稳的是删掉后在 COS 控制台重新添加；
5. 桶策略被手动改坏（删掉了 CDN 服务身份的 `GetObject`）时，重新点一次服务授权。

> 修复后 `curl -I https://img.orbit-e.com/` 会从 **403** 变成 **404**（根路径没有对象，
> 属正常）；请用真实对象 URL 或 `storage-check` 做最终验证。
> 如果修完仍是 403，去 CDN 控制台**刷新缓存**（可能缓存了之前的 403）。

### Q2. 图片 404

- `MINIO_PUBLIC_URL_STYLE` 是否漏配？CDN 域名下 `/bucket/key` 会 404，必须是 `/key`；
- 数据库里还是旧地址（第 9.2 节没执行）；
- 迁移时 key 前缀变了（rclone 同步要保持原 key 结构）。

### Q3. 浏览器报 CORS / 画布导出失败

- COS 桶 CORS 是否配了 `GET, HEAD` 且 Origin 包含站点域名；
- CDN 是否把 CORS 响应头透传/追加；
- 用 `curl -I -H "Origin: https://www.orbit-e.com"` 确认响应里有 `access-control-allow-origin`。

### Q4. 上传成功但返回的地址是旧域名

- `system_configs.storage_public_url` 还在覆盖 env（8.2）；
- 容器没重启（env 只在启动时读）；
- 改的是 `.env` 而不是 `.env.production`。

### Q5. 启动日志出现 `object storage unavailable, falling back to local uploads`

说明 `storage.New` 失败，API 会**静默降级到本地上传**（很危险，表现是"图片传到服务器本地了"）。常见原因：

- `MINIO_SKIP_BUCKET_INIT=false` 但子账号没有 `HeadBucket` 权限；
- `MINIO_REGION` 与桶地域不一致（签名失败 `SignatureDoesNotMatch`）；
- Endpoint 写成了 `https://cos...`（代码会自动去掉协议，但建议不写）。

### Q6. `SignatureDoesNotMatch` / `AccessDenied`

- Region 必须是 `ap-hongkong`（桶在哪就填哪）；
- SecretId/SecretKey 是否有多余空格；
- 子账号策略的 resource 里 APPID 是否写对（`uid/1250000000`）；
- 系统时间偏差超过 15 分钟也会导致签名失败（`date` 检查一下）；
- 如果报的是 `The bucket you are attempting to access must be addressed using COS
  virtual-styled domain`，说明请求走了 path-style。新代码会对 `*.myqcloud.com` 自动改用
  虚拟主机风格；旧镜像在 `.env.production` 加 `MINIO_BUCKET_LOOKUP=dns` 后重启即可。

### Q7. CDN 缓存不更新

- 默认缓存 30 天，删除/覆盖后必须**刷新缓存**；
- 可以给桶配"CDN 缓存自动刷新"（函数计算），或在发布脚本里调用 CDN 刷新 API。

### Q8. 境内用户还是慢

- 未备案 → 只能用中国境外节点，跨境延迟无法消除；
- 想真正解决，需要给域名做 ICP 备案，然后把加速地域改成"中国境内"或"全球加速"；
- 备案前可先优化图片体积（第 11 节），收益往往比换节点更直接。

### Q9. 费用突然变高

- 检查是否被刷：CDN 控制台看 TOP URL/IP，开启带宽封顶和 URL 鉴权；
- 检查缓存命中率：命中率低会导致大量回源流量；
- 检查是否有超大原图被列表页直接加载（确认 `NEXT_PUBLIC_MEDIA_CDN_HOSTS` 已生效）。

---

## 附录 A：命令速查

```bash
CMP="docker compose -f infra/docker/docker-compose.prod.yml --env-file .env.production"

# 状态 / 日志
$CMP ps
$CMP logs --tail=100 api worker

# 存储自检
$CMP exec api /storage-check
$CMP run --rm api /storage-check -no-db

# 查数据库存储配置
$CMP exec -T postgres psql -U starai -d starai -c \
  "SELECT key, value FROM system_configs WHERE key LIKE 'storage_%';"

# 查媒体地址分布
$CMP exec -T postgres psql -U starai -d starai -c \
  "SELECT count(*) FROM works WHERE thumbnail_url LIKE 'https://img.orbit-e.com/%';"

# CDN 验证
dig +short img.orbit-e.com
curl -sI https://img.orbit-e.com/<key> | head -20
curl -sI -H "Origin: https://www.orbit-e.com" https://img.orbit-e.com/<key> | grep -i access-control
```

## 附录 B：相关代码位置

| 内容 | 路径 |
| --- | --- |
| 存储客户端（API） | `services/api/internal/storage/minio.go` |
| 存储配置与 DB 覆盖 | `services/api/internal/storage/config.go` |
| 环境变量定义 | `services/api/internal/config/config.go` |
| 启动与降级逻辑 | `services/api/cmd/api/main.go` |
| 存储自检命令 | `services/api/cmd/storage-check/main.go` |
| 存储客户端（Worker） | `services/worker/internal/storage/minio.go` |
| 上传入口 | `services/api/internal/handler/handler.go`（`Upload` / `UploadAsset`） |
| 灵感广场 | `services/api/internal/service/gallery.go`、`apps/web/src/components/workbench/GalleryPanel.tsx` |
| 缩略图工具 | `apps/web/src/lib/media.ts` |
| URL 批量改写 | `scripts/rewrite-media-urls.sql` |

## 附录 C：官方文档

- [开启自定义 CDN 加速域名](https://cloud.tencent.com/document/product/436/36637)
- [CDN 加速配置](https://cloud.tencent.com/document/product/436/18670)
- [CDN 加速概述（权限矩阵）](https://cloud.tencent.com/document/product/436/18669)
- [开启自定义源站域名](https://cloud.tencent.com/document/product/436/36638)
- [CDN 鉴权配置](https://cloud.tencent.com/document/product/228/41622)
- [数据万象图片缩放 imageMogr2](https://cloud.tencent.com/document/product/460/36540)

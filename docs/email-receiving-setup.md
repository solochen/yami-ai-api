# 接收邮件配置手册（hr@orbit-e.com · Cloudflare Email Routing）

2026-10-18 编写。目标：让 `hr@orbit-e.com` 这类地址能收到外部来信，信件自动转发到你的常用邮箱。

## 0. 先搞清楚边界

**腾讯云邮件推送（SES）只能发信，不能收信。** 它没有收信产品，控制台里也配不出来。
我们之前给 `mail.orbit-e.com` 加的那条 MX（指向 `mxbiz1.qq.com`）只是让腾讯云能收到**退信通知**，
不是一个邮箱——别人往那儿发信不会有人收到。

本手册用 **Cloudflare Email Routing**：免费、DNS 已在 Cloudflare、5 分钟配完。
代价是它**只转发不存储**，也不能用 `hr@orbit-e.com` 发信（发信见第 6 节）。

## 1. 动手前的 DNS 基线

| 域名 | 现状 | 说明 |
|---|---|---|
| `orbit-e.com`（根域名） | 无 MX、无 SPF、无 DMARC | **本次要动的是这里**，目前完全干净，不会有冲突 |
| `mail.orbit-e.com` | MX → `mxbiz1.qq.com`，SPF → `qcloudmail.com` | **只用于发信，本次一律不要动** |

根域名干净是这次配置的最大便利：Cloudflare 自动写入的 MX / SPF / DKIM 不会覆盖或冲突任何现有记录。

**两条硬性禁令：**

1. **不要把收信邮箱建在 `mail.orbit-e.com` 上**（例如 `hr@mail.orbit-e.com`）。那个子域名的 MX 已被腾讯云 SES 占用做退信接收，再指向邮件服务商会冲突，且会破坏现有验证码发信。
2. **根域名最终只能有一条 SPF 记录。** Cloudflare 会写入 `v=spf1 include:_spf.mx.cloudflare.net ~all`。将来若给根域名加其他发信渠道（例如把根域名也加到腾讯云 SES），必须把 include 合并进同一条，**不能新增第二条**——多条 SPF 会导致 SPF 整体失效（permerror），发信会立刻开始进垃圾箱。

## 2. 操作步骤

> 界面路径按 2026 年当前 Cloudflare 控制台。若菜单名有出入，认准 **Email Routing** 这个入口即可。

### 步骤 1：启用 Email Routing 并写入 DNS 记录

| | |
|---|---|
| **位置** | Cloudflare Dashboard → 选中 `orbit-e.com` → 左侧 **Compute → Email Service → Email Routing** |
| **操作** | 点 **Onboard Domain**（接入域名）→ 选择 `orbit-e.com` → 展开 DNS 记录预览确认 → 点 **Done** |
| **预期** | 页面提示已启用；根域名被写入 3 条 MX（`route1/2/3.mx.cloudflare.net`，优先级 10/20/30）、1 条 SPF TXT、1 条 DKIM TXT |
| **看哪里** | 点开 DNS 记录预览时，**确认这些记录都挂在 `orbit-e.com` 根域名上，且列表里没有 `mail` 子域名** |

如果这一步弹出「检测到已有 MX 记录」的提示，先停下——说明根域名已经被人加过 MX，把那条记录内容发我确认后再继续。

### 步骤 2：添加目标邮箱并完成验证

| | |
|---|---|
| **位置** | Email Routing → **Destination Addresses**（目标地址）标签页 |
| **操作** | 在 **Destination addresses** 下方的输入框填你的常用邮箱（如 QQ 邮箱）→ 提交 |
| **预期** | 目标邮箱会收到一封 Cloudflare 的验证邮件 |
| **操作** | 去该邮箱打开邮件，点 **Verify email address** |
| **预期** | 回到 Cloudflare 页面，该地址状态变为已验证 |

**关键点**：目标地址**未验证之前，指向它的路由规则会一直处于禁用状态**，邮件不会转发。这一步漏了会表现为「配置都对但收不到信」。

### 步骤 3：创建 hr@ 的路由规则

| | |
|---|---|
| **位置** | Email Routing → **Routing Rules**（路由规则）标签页 |
| **操作** | 点 **Create routing rule**（创建路由规则） |
| **填 Email pattern** | 本地部分填 `hr`，域名选 `orbit-e.com` → 即 `hr@orbit-e.com` |
| **填 Action** | 下拉选 **Send to an email** |
| **填 Destination** | 选步骤 2 里已验证的邮箱 |
| **操作** | 点 **Save** |

### 步骤 4：开启 Catch-all（建议）

| | |
|---|---|
| **位置** | 同 Routing Rules 标签页 |
| **操作** | 打开 **Catch-all rule** 开关使其变为 **Active** → Action 选 **Send to an email** → Destination 选你的邮箱 → **Save** |
| **作用** | 把发到 `orbit-e.com` 的**所有**地址（含拼错的、以及 `contact@`、`support@` 等你没建规则的）都转发到同一个邮箱 |

这一步是「以后不用再回来加地址」的关键。不开启的话，只有显式建过规则的地址能收到信。

### 步骤 5：实测

1. 用一个**不是**目标邮箱的账号（比如另一个 QQ 号或朋友的邮箱），往 `hr@orbit-e.com` 发一封测试邮件
2. 去目标邮箱查收
3. **没收到就先翻垃圾箱**——转发信被目标邮箱判为垃圾邮件是这一步最常见的「失败」

## 3. 配置后自查

```bash
# MX 应返回 3 条 route*.mx.cloudflare.net
dig +short MX orbit-e.com

# SPF 应包含 include:_spf.mx.cloudflare.net
dig +short TXT orbit-e.com

# 发信用的子域名应保持不变，仍是 mxbiz1.qq.com / qcloudmail.com
dig +short MX mail.orbit-e.com
dig +short TXT mail.orbit-e.com
```

最后两条是回归检查：**如果 `mail.orbit-e.com` 的记录变了，说明动错了地方**，验证码发信会受影响。

## 4. 平台限制（够用，但先知道）

| 项目 | 限制 |
|---|---|
| 目标地址数量 | 每账号最多 200 个，跨域名共用 |
| 路由规则数量 | 每个域名最多 200 条 |
| 单封邮件大小上限 | 25 MiB，超出直接拒收 |
| 前置条件 | 必须使用 Cloudflare DNS（我们是） |
| DNS 生效时间 | 官方称最长 24 小时，实测通常几分钟 |

## 5. 常见坑

- **规则建了但收不到**：九成是目标地址没验证（规则处于 Disabled）或信件进了目标邮箱的垃圾箱。
- **同一个 pattern 建了多条规则**：只有列表里**最靠前**的那条生效，其余静默失效。
- **删除目标地址**：所有以它为目标的规则会被**自动禁用**，需要重新指定目标。
- **不要用 `hr@mail.orbit-e.com`**：与腾讯云 SES 的退信 MX 冲突。
- **根域名 SPF 只能一条**：见第 1 节的禁令 2。

## 6. 后续：如果还需要「用 hr@orbit-e.com 发信」

Email Routing 只负责收。要能以 `hr@` 身份回信，两条路：

**A. 换成真正的企业邮箱**（收发一体，推荐给需要正式对外回信的场景）

| 方案 | 免费额度 | 付费 |
|---|---|---|
| 腾讯企业邮（exmail.qq.com，独立产品，不在腾讯云控制台） | 免费版 1G 邮箱容量 + 2G 中转站 | 专业版 950 元/年起 |
| Zoho Mail | 1 个域名、最多 5 用户、每人 5G | 按用户订阅；**免费版不含 IMAP/POP/ActiveSync**，只能网页版和 App |

**B. 保留 Cloudflare 收信 + 用腾讯云 SES 发信**（免费，但维护两套）

1. 保持第 2 节的收信配置
2. 到腾讯云 SES 控制台**再添加一个发信域名 `orbit-e.com`**（根域名），完成 SPF/DKIM 验证
3. 之后即可用 `hr@orbit-e.com` 作为发件人发信
4. **务必把根域名 SPF 合并成一条**：

```
v=spf1 include:_spf.mx.cloudflare.net include:qcloudmail.com ~all
```

## 7. 与备案的关系

**不需要 ICP 备案。** `orbit-e.com` 的 A 记录指向香港（124.156.112.244），
腾讯云 SES 文档明确：「如果域名仅用于发送邮件，不强制备案通过。如果域名的 A 记录指向大陆服务器，则需要备案。」收信同理。

## 8. 建议顺手补一条 DMARC

配好收信后，根域名目前仍无 DMARC，别人可以伪造 `@orbit-e.com` 发钓鱼邮件。建议在 Cloudflare DNS 加一条 TXT：

| 字段 | 值 |
|---|---|
| Type | `TXT` |
| Name | `_dmarc` |
| Content | `v=DMARC1; p=none; rua=mailto:hr@orbit-e.com` |

`p=none` 表示只观察不拦截，安全无副作用，可以先用一段时间看报表再收紧。

# RelayPulse 中转商 / 模型价格调研（2026-10-30）

> 数据源：relaypulse.top `/api/status`（44 家中转商、63 条监测）+ 各站公开的 New API `/api/pricing`、`/api/status` 接口。
> 价格口径：全部折算为 **人民币 / 百万 token**（图像为 ¥/张，视频为 ¥/秒）。换算公式见第 3 节。

## 0. 结论先行

1. **RelayPulse 是「Claude Code / Codex / Gemini 编程套餐中转」监测站，不是通用 API 比价站。** 44 家里有 14 家公开了按量计价的 New API 接口（linoroute 仅 15 天样本，主表统计 13 家）；其余 30 家是订阅制/自建站，无法自动取价（其中 YunDou、oozkzi 可用率 100%，autocodex 98%，但没有公开按量价）。
2. **可用率最高且公开按量价的 4 家**：PackyCode 96.3%、yomiapi 95.0%、ClaudeCN 94.2%、cheapai 92.5%。前两家 + ClaudeCN 属于「可长期合作」档；cheapai 只有 59 天，先小额试。
3. **Claude/GPT 的极低价全部来自 Kiro/Codex 订阅号池**（官方价的 2%~15%），不是官方 API，存在降智、封号、随时断供风险，**不建议作为对外售卖的主线路**。
4. **真正能改善毛利的是这 4 类**：`glm-5-3-flash` 最低 ¥0.12/M（当前 ¥0.8）、`deepseek-v4-1-flash-1017` ¥0.3/M（当前 ¥1.0）、`qwen3-8-flash` ¥0.4/M（当前 ¥0.8）、`wan-3-0` 720p ¥0.312/s（当前 ¥0.6/s）；`qwen3-8-max-0902` 同型号最低 ¥6/M（官方 ¥12）。注意：relay 的 `glm-5.3` 是标准版，**没有 `glm-5-3-prime`**，不能拿来替换 prime。
5. **汇率**：13 家里 12 家是「1 元 = 1 美元额度」（充值倍率 1），只有 yomiapi 是 6.95 元/美元；modelflare 是美元站。所有表已按人民币折算，不需要再乘 7。

## 1. 中转商总览（按 30 天可用率排序）

> 另有 4 家 `/api/pricing` 需要登录（ByteCatCode、die-ge、lucoo、tokenshop），以及 27 家完全自建/订阅制站点（ICodeEasy、TopRouterCN、autocodex、YunDou 等）无法自动取价。RelayPulse 的可用率监测的是 Claude Code / Codex / Gemini 三种编程端点，**不等于这些站按量 API 的可用率**。

| 中转商 | 站点 | 30d可用率 | P10 | 平均延迟 | 在榜天数 | 公开按量价 | 充值倍率 | 模型数 | 分组数 | 备注 |
|---|---|---:|---:|---:|---:|---|---:|---:|---:|---|
| PackyCode | https://www.packyapi.ai | 96.31% | 88.5% | 2790ms | 340 | ✅ | 1 元/$ | 78 | 24 | 340 天在榜；bailian/glm/deepseek 官转分组齐全，综合最稳 |
| yomiapi | https://yomiapi.com | 95.04% | 84.6% | 2362ms | 77 | ✅ | 6.95 元/$ | 46 | 14 | 77 天；唯一按 6.95 汇率计价，价格≈官方 CNY |
| ClaudeCN | https://claudecn.ai | 94.22% | 84.6% | 2496ms | 326 | ✅ | 1 元/$ | 266 | 10 | 326 天在榜；国产模型 0.9 分组标称 99.9% SLA，正价组=官方价 |
| cheapai | https://cheapai.info | 92.48% | 66.7% | 2752ms | 59 | ✅ | 1 元/$ | 38 | 15 | 59 天新站；价格最低但多处标注「缓存低/不保证 24h」 |
| 91token | https://91token.com | 90.15% | 63.3% | 2947ms | 109 | ✅ | 1 元/$ | 68 | 17 | 国产特惠 0.2 折；Claude/GPT 走 Kiro/Plus 号池 |
| lighthub | https://ly.ai3322.site | 86.54% | 37.5% | 3751ms | 77 | ✅ | 1 元/$ | 37 | 12 | GLM/DS 分组便宜；福利分组有 0 元模型 |
| LinkAPI | https://linkapi.ai | 86.06% | 56.5% | 3552ms | 324 | ✅ | 1 元/$ | 158 | 27 | 324 天在榜；分组多但可用率仅 86% |
| ikunapi | https://api.monkey-tools.cn | 84.83% | 36.8% | 3363ms | 151 | ✅ | 1 元/$ | 56 | 32 | Claude Kiro 史低；GPT 有「国模映射」需警惕 |
| dayway | https://dayway.site | 78.36% | 28.6% | 4914ms | 76 | ✅ | 1 元/$ | 9 | 4 | 仅 9 个模型，可用率 78% |
| nodexi | https://www.nodexi.dev | 77.88% | 35.0% | 4858ms | 83 | ✅ | 1 元/$ | 46 | 9 | 可用率 78%，价格中等 |
| modelflare | https://origin.modelflare.dev | 71.05% | 27.3% | 5703ms | 123 | ✅ | 美元站×7.2 | 70 | 21 | 美元站，award 分组疑似补贴/赠送，不适合长期商用 |
| ikuncode | https://api.ikuncode.cc | 63.85% | 25.0% | 4522ms | 342 | ✅ | 1 元/$ | 38 | 17 | 342 天老站但可用率仅 64%，低价伴随高故障 |
| wanmoapi | https://www.wanmoapi.com | 52.29% | 2.1% | 6528ms | 136 | ✅ | 1 元/$ | 47 | 10 | 52% 可用率，最便宜，仅适合离线批处理 |
| YunDou | https://yundou.ai | 100.0% | 100.0% | 2565ms | 175 | ❌ | - | - | - | 最好成绩，但未公开按量价 |
| oozkzi | https://oozkzi.cn | 100.0% | 100.0% | 3016ms | 53 | ❌ | - | - | - | 100% 但仅 53 天，未公开按量价 |
| autocodex | https://vip.auto-code.net | 98.08% | 84.6% | 2945ms | 194 | ❌ | - | - | - | 未公开按量价 |
| ark（火山方舟） | https://www.volcengine.com/L/avzVZehszoA | 97.16% | 88.5% | 1821ms | 86 | ❌ | - | - | - | 官方 Coding Plan，不是中转 |
| wawazz | https://wawazz.xyz | 95.39% | 71.4% | 2853ms | 115 | ❌ | - | - | - | 未公开按量价 |

> 「收入高」RelayPulse 并不公开营收数据，任何声称能直接排序收入的都是猜的。这里用 **在榜天数（经营时长）+ 30 天可用率 + 是否有公开报价接口** 作为代理指标。

## 2. 13 家公开报价站的费率结构

> 注：linoroute（linoroute.com，15 天、可用率 83.7%）也公开了 333 个模型的报价，因样本太短未并入主表；它的「自部署」分组 DeepSeek V4.1 Flash ¥0.198/0.792、GLM-5.3-Flash ¥0.099/0.33、Qwen3.8-Max ¥1.32/3.96，价格带与 lighthub 接近。

| 中转商 | 显示币种 | 充值倍率（¥/$额度） | 显示汇率 | 分组数 | 计价方式 |
|---|---|---:|---:|---:|---|
| PackyCode | USD | 1 元/$ | 7 | 24 | 按 token / 按次（图像视频） |
| yomiapi | USD | 6.95 元/$ | 6.95 | 14 | 按 token / 按次（图像视频） |
| ClaudeCN | USD | 1 元/$ | 7.3 | 10 | 按 token / 按次（图像视频） |
| cheapai | USD | 1 元/$ | 6.7 | 15 | 按 token / 按次（图像视频） |
| 91token | CNY | 1 元/$ | 1 | 17 | 按 token / 按次（图像视频） |
| lighthub | CNY | 1 元/$ | 1 | 12 | 按 token / 按次（图像视频） |
| LinkAPI | CNY | 1 元/$ | 1 | 27 | 按 token / 按次（图像视频） |
| ikunapi | CNY | 1 元/$ | 1 | 32 | 按 token / 按次（图像视频） |
| dayway | USD | 1 元/$ | 1 | 4 | 按 token / 按次（图像视频） |
| nodexi | USD | 1 元/$ | 1 | 9 | 按 token / 按次（图像视频） |
| modelflare | USD | 美元站×7.2 | 1 | 21 | 按 token / 按次（图像视频） |
| ikuncode | CNY | 1 元/$ | 1 | 17 | 按 token / 按次（图像视频） |
| wanmoapi | CNY | 1 元/$ | 1 | 10 | 按 token / 按次（图像视频） |

**怎么读**：
- 充值倍率 1 表示「充 1 元得到 1 美元额度」，所以站内显示的 `$3/M` 实际就是 ¥3/M，比官方 ¥21.6 便宜 7 倍——这是这类站的核心商业模式（靠 Claude/GPT 订阅号池或国产模型渠道折扣）。
- 分组倍率才是真正的折扣开关。同一个模型在不同分组可能差 20 倍：例如 ClaudeCN 的 `Claude Code` 组 1.0 vs `正价Claude` 组 7.0；7.0 就是「官方美元价 × 7」的人民币正价。
- 下文所有价格都取「该模型可用的、倍率最低的非 0 分组」，并保留分组名，方便你判断是官方还是逆向渠道。

## 3. 换算公式与校验

```
按 token：  ¥/M = model_ratio × 2 × 分组倍率 × 充值倍率
            （若带 billing_expr，则直接取 expr 里的 p/c/cr 系数，单位已是 USD/M）
按次：      ¥/次 = model_price × 分组倍率 × 充值倍率
按秒：      ¥/秒 = expr 里 seconds 的单价 × 分组倍率 × 充值倍率
modelflare：美元站，最后再 × 7.2 折算人民币
```

校验样例：ClaudeCN 的 `gpt-4o-mini` model_ratio=0.075、分组 `正价Openai`=7、充值倍率=1 → 0.075×2×7 = **¥1.05/M 输入**，与 OpenAI 官方人民币价 ¥1.1 基本一致，说明公式和汇率语义正确。

## 4. 重点模型比价（便宜 → 贵）

> 只列「按 token」计费，已排除 0 元福利组。`vs 当前成本` = 该报价输入价 ÷ 我们项目当前成本；`—` 表示项目里没有对应模型或没有可比价格。

### 4.1 国产文本

| 模型 | 中转商 | 使用分组 | 输入 ¥/M | 输出 ¥/M | 缓存读 ¥/M | 缓存写 ¥/M | vs 当前成本 |
|---|---|---|---:|---:|---:|---:|---:|
| DeepSeek V4.1 Flash（-1017） | cheapai | 国产模型-全网最低(缓存低慎用) | 0.1 | 0.4 | 0.002 | — | 10% |
| DeepSeek V4.1 Flash（-1017） | ikunapi | 88_线路2_神州模型_DeepSeek | 0.3 | 1.2 | 0.006 | — | 30% |
| DeepSeek V4.1 Flash（-1017） | 91token | 国产模型 特惠渠道 | 0.4 | 1.6 | 0.008 | — | 40% |
| DeepSeek V4.1 Flash（-1017） | ClaudeCN | 国产模型 | 0.9 | 3.6 | 0.018 | — | 90% |
| DeepSeek V4.1 Flash（无日期别名） | cheapai | 国产模型-全网最低(缓存低慎用) | 0.1 | 0.4 | 0.002 | — | 10% |
| DeepSeek V4.1 Flash（无日期别名） | modelflare | deepseek-award | 0.1219 | 0.4877 | 0.002439 | — | 12% |
| DeepSeek V4.1 Flash（无日期别名） | 91token | 国产模型 特惠渠道 | 0.2 | 0.8 | 0.004 | — | 20% |
| DeepSeek V4.1 Flash（无日期别名） | ikunapi | 88_线路2_神州模型_DeepSeek | 0.3 | 1.2 | 0.006 | — | 30% |
| DeepSeek V4.1 Flash（无日期别名） | ikuncode | deepseek-cheap | 0.35 | 1.4 | 0.007 | — | 35% |
| DeepSeek V4.1 Flash（无日期别名） | PackyCode | deepseek-sale | 0.5 | 2 | 0.01 | — | 50% |
| DeepSeek V4 Pro（-0813） | cheapai | 国产模型-全网最低(缓存低慎用) | 0.45 | 1.35 | 0.015 | — | 10% |
| DeepSeek V4 Pro（-0813） | 91token | 国产模型 特惠渠道 | 1.8 | 5.4 | 0.06 | — | 40% |
| DeepSeek V4 Pro（-0813） | wanmoapi | sale | 2.25 | 6.75 | 0.225 | — | 50% |
| DeepSeek V4 Pro（-0813） | ClaudeCN | 国产模型 | 4.05 | 12.15 | 0.135 | — | 90% |
| DeepSeek V4 Pro（别名） | ikunapi | 88_线路2_神州模型_DeepSeek | 0.3 | 1.2 | 0.006 | — | 7% |
| DeepSeek V4 Pro（别名） | modelflare | deepseek-award | 0.486 | 1.458 | 0.0162 | — | 11% |
| DeepSeek V4 Pro（别名） | 91token | 国产模型 特惠渠道 | 0.9 | 2.7 | 0.03 | — | 20% |
| DeepSeek V4 Pro（别名） | ikuncode | deepseek-cheap | 1.575 | 4.725 | 0.0525 | — | 35% |
| DeepSeek V4 Pro（别名） | PackyCode | deepseek-sale | 2.25 | 6.75 | 0.07493 | — | 50% |
| DeepSeek V4 Pro（别名） | yomiapi | DeepSeek 优选线路 | 2.752 | 8.257 | 0.09174 | — | 61% |
| GLM-5.3（标准版） | modelflare | glm-award | 0.432 | 1.512 | 0.108 | — | 5% |
| GLM-5.3（标准版） | cheapai | 国产模型-全网最低(缓存低慎用) | 0.8 | 2.8 | 0.2 | — | 10% |
| GLM-5.3（标准版） | lighthub | GLM专属分组 | 1.2 | 4.2 | 0.3 | — | 15% |
| GLM-5.3（标准版） | LinkAPI | Temp | 3.2 | 11.2 | 0.8 | — | 40% |
| GLM-5.3（标准版） | 91token | 国产模型 官转渠道 | 4 | 14 | 1 | — | 50% |
| GLM-5.3（标准版） | wanmoapi | sale | 4 | 14 | 1 | — | 50% |
| GLM-5.3 Flash | modelflare | glm-award | 0.0432 | 0.1512 | 0.01242 | — | 5% |
| GLM-5.3 Flash | lighthub | GLM专属分组 | 0.12 | 0.42 | 0.0345 | — | 15% |
| GLM-5.3 Flash | LinkAPI | Temp | 0.16 | 0.56 | 0.046 | — | 20% |
| GLM-5.3 Flash | ikunapi | 88_线路1_神州模型_GLM | 0.24 | 0.84 | 0.069 | — | 30% |
| GLM-5.3 Flash | 91token | 国产模型 官转渠道 | 0.4 | 1.4 | 0.115 | — | 50% |
| GLM-5.3 Flash | yomiapi | 智谱 国模折扣 | 0.4795 | 1.668 | 0.1168 | — | 60% |
| GLM-5.3 FlashX | 91token | 国产模型 官转渠道 | 1 | 3.5 | 0.285 | — | 667% |
| GLM-5.3 FlashX | ikuncode | glm | 1.4 | 4.9 | 0.399 | — | 933% |
| GLM-5.3 FlashX | ClaudeCN | 国产模型 | 1.8 | 6.3 | 0.513 | — | 1200% |
| GLM-5.3 FlashX | PackyCode | glm-sale | 2 | 7 | 0.57 | — | 1333% |
| Qwen3.8 Max（-0902） | PackyCode | bailian | 6 | 18 | 0.75 | — | 50% |
| Qwen3.8 Max（-0902） | ClaudeCN | 国产模型 | 10.8 | 32.4 | 1.35 | — | 90% |
| Qwen3.8 Max（别名） | modelflare | qwen-stable | 1.481 | 4.443 | 0.1851 | 1.851 | 12% |
| Qwen3.8 Max（别名） | cheapai | 国产模型-低价高缓存 | 2.4 | 7.2 | 0.48 | 3 | 20% |
| Qwen3.8 Max（别名） | PackyCode | bailian | 6 | 18 | 0.75 | — | 50% |
| Qwen3.8 Max（别名） | wanmoapi | bailian | 6 | 18 | 0.75 | — | 50% |
| Qwen3.8 Max（别名） | 91token | 国产模型 官转渠道 | 7.5 | 22.5 | 0.9375 | 9.375 | 62% |
| Qwen3.8 Max（别名） | ClaudeCN | 国产模型 | 10.8 | 32.4 | 1.35 | — | 90% |
| Qwen3.8 Flash | PackyCode | bailian | 0.4 | 1.35 | 0.05 | — | 50% |
| Qwen3.8 Flash | 91token | 国产模型 官转渠道 | 0.5625 | 1.762 | 0.06 | 0.75 | 70% |
| Kimi K3 | modelflare | kimi-award | 1.08 | 5.4 | 0.108 | — | — |
| Kimi K3 | cheapai | 国产模型-全网最低(缓存低慎用) | 2 | 10 | 0.2 | — | — |
| Kimi K3 | ikuncode | kimi-cheap | 6 | 30 | 0.6 | — | — |
| Kimi K3 | 91token | 国产模型 官转渠道 | 10 | 50 | 1 | — | — |
| Kimi K3 | PackyCode | kimi-sale | 10 | 50 | 1 | — | — |
| Kimi K3 | wanmoapi | sale | 10 | 50 | 1 | — | — |

> 表中「无日期别名 / 别名」行可能对应不同的上游版本（例如 91token 的 `deepseek-v4-pro` 与 `deepseek-v4-pro-0813` 价差 2 倍）。接入前务必确认对方渠道实际映射的 `upstream_model`，不要只看模型名。

### 4.2 海外文本

| 模型 | 中转商 | 使用分组 | 输入 ¥/M | 输出 ¥/M | 缓存读 ¥/M | 缓存写 ¥/M | vs 当前成本 |
|---|---|---|---:|---:|---:|---:|---:|
| Claude Sonnet 5.5 | modelflare | claude-tmp-cheap | 0.1296 | 0.648 | 0.01296 | 0.162 | 1% |
| Claude Sonnet 5.5 | cheapai | claude-kiro-撕破底价 | 0.23 | 1.15 | 0.023 | 0.2875 | 2% |
| Claude Sonnet 5.5 | lighthub | claude kiro分组 | 0.24 | 1.2 | 0.024 | 0.3 | 2% |
| Claude Sonnet 5.5 | ikunapi | 🏀_Claude_标准分组 | 0.3 | 1.5 | 0.03 | 0.375 | 2% |
| Claude Sonnet 5.5 | wanmoapi | claude_kiro | 0.4 | 2 | 0.02 | 0.5 | 3% |
| Claude Sonnet 5.5 | 91token | Claude 特惠 | 0.6 | 3 | 0.03 | 0.75 | 4% |
| Claude Sonnet 5 | modelflare | claude-tmp-cheap | 0.1296 | 0.648 | 0.01296 | 0.162 | 1% |
| Claude Sonnet 5 | cheapai | claude-kiro-撕破底价 | 0.23 | 1.15 | 0.023 | 0.2875 | 2% |
| Claude Sonnet 5 | lighthub | claude kiro分组 | 0.24 | 1.2 | 0.024 | 0.36 | 2% |
| Claude Sonnet 5 | ikunapi | Kiro_史低 | 0.33 | 1.65 | 0.033 | 0.4125 | 2% |
| Claude Sonnet 5 | wanmoapi | claude_kiro | 0.4 | 2 | 0.04 | 0.5 | 3% |
| Claude Sonnet 5 | 91token | Claude 特惠 | 0.6 | 3 | 0.06 | 0.75 | 4% |
| Claude Opus 5.5 | modelflare | claude-tmp-cheap | 0.2592 | 1.296 | 0.01296 | 0.324 | 1% |
| Claude Opus 5.5 | ikunapi | Kiro_史低 | 0.44 | 2.2 | 0.022 | 0.55 | 2% |
| Claude Opus 5.5 | cheapai | claude-kiro-撕破底价 | 0.46 | 2.3 | 0.046 | 0.575 | 2% |
| Claude Opus 5.5 | lighthub | claude kiro分组 | 0.48 | 2.4 | 0.024 | 0.6 | 2% |
| Claude Opus 5.5 | wanmoapi | claude_kiro | 0.8 | 4 | 0.04 | 1 | 3% |
| Claude Opus 5.5 | 91token | Claude 特惠 | 1.2 | 6 | 0.06 | 1.5 | 4% |
| GPT-5.6 Luna | cheapai | 【低价·高可用】 | 0.04 | 0.24 | 0.04 | 0.05 | 3% |
| GPT-5.6 Luna | lighthub | GPT稳定分组 | 0.078 | 0.468 | 0.0078 | 0.0975 | 6% |
| GPT-5.6 Luna | LinkAPI | codex | 0.08 | 0.36 | 0.008 | 0.1 | 6% |
| GPT-5.6 Luna | ClaudeCN | CodeX | 0.1 | 0.6 | 0.01 | — | 7% |
| GPT-5.6 Luna | ikunapi | GPT-国模映射 | 0.2 | 0.9 | 0.02 | 0.25 | 15% |
| GPT-5.6 Luna | nodexi | gpt-stable | 0.5 | 3 | 0.05 | — | 37% |
| GPT-5.6 Terra | modelflare | openai-award | 0.0648 | 0.3888 | 0.00648 | 0.081 | — |
| GPT-5.6 Terra | cheapai | cheap-gpt-特惠 | 0.16 | 0.96 | 0.016 | 0.2 | — |
| GPT-5.6 Terra | ikunapi | GPT-Plus | 0.28 | 1.26 | 0.028 | 0.35 | — |
| GPT-5.6 Terra | lighthub | GPT稳定分组 | 0.52 | 2.34 | 0.052 | 0.65 | — |
| GPT-5.6 Terra | 91token | GPT Plus | 0.8 | 3.6 | 0.08 | 1 | — |
| GPT-5.6 Terra | LinkAPI | codex | 0.8 | 3.6 | 0.08 | 1 | — |
| GPT-5.6 Sol | modelflare | openai-award | 0.162 | 0.972 | 0.0162 | 0.2025 | 1% |
| GPT-5.6 Sol | cheapai | cheap-gpt-特惠 | 0.4 | 2.4 | 0.04 | 0.5 | 1% |
| GPT-5.6 Sol | ikunapi | GPT-Plus | 0.7 | 3.15 | 0.07 | 0.875 | 3% |
| GPT-5.6 Sol | PackyCode | codex-sale | 1.2 | 6 | 0.12 | — | 4% |
| GPT-5.6 Sol | lighthub | GPT稳定分组 | 1.3 | 5.85 | 0.13 | 1.625 | 5% |
| GPT-5.6 Sol | LinkAPI | codex | 1.6 | 6 | 0.16 | 2 | 6% |
| Gemini 3.8 Flash | modelflare | gemini-stable | 0.243 | 1.215 | 0.0243 | 0.162 | 4% |
| Gemini 3.8 Flash | LinkAPI | geminicheap | 0.375 | 1.875 | 0.0375 | — | 7% |
| Gemini 3.8 Flash | 91token | Gemini 特惠 | 0.45 | 2.25 | 0.045 | — | 8% |
| Gemini 3.8 Flash | ikuncode | gemini | 0.525 | 2.625 | 0.0525 | — | 9% |
| Gemini 3.8 Flash | yomiapi | Gemini特价线路 | 0.7298 | 3.649 | 0.07297 | — | 13% |
| Gemini 3.8 Flash | PackyCode | gemini-slb | 2.25 | 11.25 | 0.225 | — | 41% |
| Grok 4.7 | modelflare | grok-award | 0.0216 | 0.0648 | 0.0054 | — | — |
| Grok 4.7 | lighthub | grok分组 | 0.26 | 0.78 | 0.065 | — | — |
| Grok 4.7 | LinkAPI | GrokHeavy | 0.5 | 1.5 | 0.125 | — | — |
| Grok 4.7 | wanmoapi | grok_sale | 0.5 | 1.5 | 0.125 | — | — |
| Grok 4.7 | PackyCode | grok-sale | 0.6 | 1.8 | 0.15 | — | — |
| Grok 4.7 | ikuncode | grok-heavy-test | 0.6 | 1.8 | 0.15 | — | — |

### 4.3 图像（¥/张）

| 模型 | 中转商 | 使用分组 | ¥/张 | 实际型号 |
|---|---|---|---:|---|
| gpt-image-2.5 | ikunapi | 99_线路4_gpt生图 | 0.025 | gpt-image-2.5-sunburst |
| gpt-image-2.5 | PackyCode | image | 0.0294 | gpt-image-2.5-flare |
| gpt-image-2.5 | 91token | GPT image生图 超分4K | 0.048 | gpt-image-2.5-sunburst |
| gpt-image-2.5 | wanmoapi | default | 0.05 | gpt-image-2.5-sunburst |
| gpt-image-2.5 | yomiapi | 生图专用分组 | 0.0904 | gpt-image-2.5 |
| gpt-image-2 | ikunapi | 99_线路4_gpt生图 | 0.025 | gpt-image-2 |
| gpt-image-2 | 91token | GPT image生图 原生4K | 0.04 | gpt-image-2-1k |
| gpt-image-2 | LinkAPI | once | 0.10 | gpt-image-2-c |
| gpt-image-2 | yomiapi | 生图专用分组 | 0.139 | gpt-image-2-pro |
| gpt-image-2 | dayway | GPT-生图 | 0.25 | gpt-image-2 |
| seedream-5.0-pro | ClaudeCN | 国产模型 | 0.27 | seedream-image-v5.0-pro |
| seedream-5.0-lite | ClaudeCN | 国产模型 | 0.198 | seedream-image-v5.0-lite |
| seedream-5.0-lite | modelflare | Media | 0.2391 | seedream-5-0-lite |
| gemini-3-pro-image | ikunapi | 99_线路6_banana生图 | 0.072 | gemini-3-pro-image |
| gemini-3-pro-image | 91token | Gemini 4K生图专区 | 0.15 | gemini-3-pro-image |
| gemini-3-pro-image | LinkAPI | once | 0.18 | gemini-3-pro-image-c |
| gemini-3-pro-image | yomiapi | 生图专用分组 | 0.3475 | gemini-3-pro-image-preview |
| grok-imagine-image-2.0 | PackyCode | grok-officially | 0.10 | grok-imagine-image-2.0 |
| grok-imagine-image-2.0 | yomiapi | 生图专用分组 | 0.2293 | grok-imagine-image-quality |

### 4.4 视频（¥/秒或 ¥/5 秒，按各站计费表达式折算）

| 模型/档位 | 中转商 | 使用分组 | 折算价 | 我们当前 | 降幅 | 说明 |
|---|---|---|---:|---:|---:|---|
| Seedance 2.0 · 720p | 91token | Seedance_特惠 0.8 | ¥0.795/s | ¥0.99/s | -20% | 按 108000 token/5s 折算 |
| Seedance 2.0 · 720p | ClaudeCN | 国产模型 0.9 | ¥0.894/s | ¥0.99/s | -10% | 官方挂牌价 ×0.9 |
| Seedance 2.0 Fast · 720p | 91token | Seedance 0.83 | ¥0.663/s | ¥0.80/s | -17% | 同上 |
| Seedance 2.0 Mini · 720p | 91token | Seedance 0.83 | ¥0.412/s | ¥0.50/s | -18% | 同上 |
| Seedance 2.5 · 720p | 91token | Seedance 0.8 | ¥1.208/s | ¥1.51/s | -20% | 按 70 USD/M token 折算 |
| Wan 3.0 · 480p | 91token | 阿里万相 0.52 | ¥0.156/s | ¥0.30/s | -48% | 按秒计价 |
| Wan 3.0 · 720p | 91token | 阿里万相 0.52 | ¥0.312/s | ¥0.60/s | -48% | 按秒计价 |
| Wan 3.0 · 1080p | 91token | 阿里万相 0.52 | ¥0.624/s | ¥1.20/s | -48% | 按秒计价 |
| Wan 3.0 Prime · 720p | 91token | 阿里万相 0.52 | ¥0.468/s | ¥0.90/s | -48% | 按秒计价 |

## 5. 各站「最便宜文本模型」定位（看一家店的价格带）

| 中转商 | 最便宜 3 个文本模型（输入/输出 ¥/M） | 说明 |
|---|---|---|
| 91token | Gemini 2.5 Flash Lite 0.06/0.24；Gemini 3.1 Flash Lite 0.15/0.9；Gemini 3.5 Flash Lite 0.18/1.5 | Gemini 特惠组最便宜，Claude/GPT 靠号池 |
| ClaudeCN | GPT-6 Luna 0.05/0.25；GPT-5.6 Luna 0.1/0.6；MiniMax M3.1 Flash 0.123/0.493 | CodeX 组 0.5 折，国产组 0.9 折最稳 |
| LinkAPI | text-embedding-3-small 0.01；GPT-6 Luna 0.06/0.225；Gemini 3.8 Flash 0.375/1.875 | 分组最细，但需逐模型选组 |
| PackyCode | Qwen3.5 Flash 0.1/1.0；Qwen3.7 Flash 0.1/0.4；Jev 0.126 | 百炼官方组 0.5，国产最规范 |
| cheapai | GPT-5.6 Luna 0.04/0.24；DeepSeek V4.1 Flash 0.1/0.4；GLM-5.3 0.8/2.8 | 全场最低价，但多处注明不稳定 |
| dayway | GPT-5.6 Terra 0.92/4.14；GPT-6 Sol 0.92/3.45 | 只有 9 个模型，价格不便宜 |
| ikunapi | Codex Auto Review 0.014；Claude Haiku 5.5 0.075/0.375；DeepSeek V4.1 Flash 0.3/1.2 | Claude Kiro 史低，GPT 有国模映射 |
| ikuncode | Claude Haiku 5.5 0.35；DeepSeek V4.1 Flash 0.35/1.4；GPT-6.1 Terra 0.5 | 老站但可用率仅 64% |
| lighthub | GPT-5.6 Luna 0.078/0.468；GLM-5.3 Flash 0.12/0.42；GPT-6 Luna 0.13/0.455 | 国模组 0.15 折，性价比最高档 |
| modelflare | Claude Haiku 5.5 0.006；GPT-6 Luna 0.008；Grok 4.7 0.022 | award 组疑似补贴，别当长期价 |
| nodexi | GPT-6 Luna 0.1/0.375；Gemini 3.1/3.5 Flash Lite 0.3/1.2 | 价格中等，可用率 78% |
| wanmoapi | Qwen3.5 Flash 0.1/1.0；GPT-6 Luna 0.15/0.75；Claude Haiku 5.5 0.2/1.0 | 最便宜但可用率 52% |
| yomiapi | GLM-5.3 Flash 0.48/1.668；DeepSeek Flash 0.625/2.502；Gemini 3.7 Flash 0.73/3.649 | 6.95 汇率，价格≈官方，胜在稳定 |

## 6. 结合当前项目：换哪条线路最赚钱

> 这一节严格按**我们库里的 model code**取同型号报价，避免把「GLM-5.3 标准版」当成「GLM-5.3 Prime」、把「Sonnet 5.5」当成「Sonnet 5」。

| 我们的 model code | 当前成本 | 同型号最低报价（分组） | 可信渠道价 | 零售价 | 换线后毛利率 | 结论 |
|---|---:|---|---:|---:|---:|---|
| `deepseek-v4-pro-0813` | ¥4.5/13.5 | ¥0.45/1.35（cheapai 国模全网最低） | ¥1.8/5.4（91token 特惠） | 9/27 | 80%~95% | ✅ 可换，先用 91token 灰度 |
| `deepseek-v4-1-flash-1017` | ¥1.0/4.0 | ¥0.1/0.4（cheapai） | ¥0.3/1.2（ikunapi 神州） | 2/8 | 85%~95% | ✅ 可换，ikunapi 优先于 cheapai |
| `glm-5-3-flash` | ¥0.8/2.8 | ¥0.12/0.42（lighthub GLM 专属） | ¥0.24/0.84（ikunapi 神州） | 0.8/2.8 | 70%~85% | ✅ 最确定的一笔，先换这条 |
| `glm-5-3-flashx` | ¥0.15/0.5 | ¥1.0/3.5（91token 官转） | — | 2/7 | 负 | ❌ 现有 CodexAuv 已是最低，不动 |
| `glm-5-3-prime` | ¥2.8/8.8 | **中转商没有 prime 档** | — | 16/56 | — | ❌ relay 的 `glm-5.3` 是标准版，不能替代 prime |
| `qwen3-8-max-0902` | ¥12/36 | ¥2.4/7.2（cheapai，非 -0902） | ¥6/18（PackyCode bailian，同型号） | 12/36 | 50%~80% | ⚠️ 先试 PackyCode，cheapai 需验型号 |
| `qwen3-8-flash` | ¥0.8/2.7 | ¥0.4/1.35（PackyCode bailian） | 同左 | 0.8/2.7 | 50% | ✅ 可换，官方渠道 |
| `wan-3-0` 视频 720p | ¥0.6/s | ¥0.312/s（91token 阿里万相） | 同左 | 0.6/s | 48% | ✅ 可换，量大时收益明显 |
| `seedance-2-0` 视频 720p | ¥0.99/s | ¥0.795/s（91token Seedance_特惠） | ¥0.894/s（ClaudeCN 国产） | 0.99/s | 10%~20% | ⚠️ 收益薄，优先 ClaudeCN 稳 |
| `seedream-5-0-pro` | ¥0.30/张 | ¥0.27/张（ClaudeCN 国产） | 同左 | 0.30/张 | 10% | ⚠️ 只省 3 分钱，可暂不换 |
| `seedream-5-0-lite` | ¥0.22/张 | ¥0.198/张（ClaudeCN 国产） | 同左 | 0.22/张 | 10% | ⚠️ 同上 |
| `claude-sonnet-5` | 官方 ¥13.4/67 | ¥0.24/1.2（lighthub kiro） | 正价组 ¥14/70（ClaudeCN 正价Claude） | 13.4/67 | 号池 98% / 正价 0% | ❌ 号池不可商用；正价无套利 |
| `gpt-5-6-luna` | ¥1.34/8.04 | ¥0.04/0.24（cheapai 低价高可用） | 正价组≈官方 | 1.34/8.04 | 号池 97% / 正价 0% | ❌ 同上 |

**读法**：
- 只有 **`glm-5-3-flash`、`qwen3-8-flash`、`wan-3-0`、`deepseek-*`** 是「同型号 + 有可信渠道」的真机会。
- `glm-5-3-prime` 在 13 家里**没有任何一家卖**（relay 列表里的 `glm-5.3` 是标准版，官方 ¥8/28），不要用它去替换 prime，否则就是降档卖。
- Claude/GPT 的「同型号最低价」全是号池；正价组（×7 汇率）和我们的零售价基本持平，没有套利空间。

### 6.1 报价可信度分层（比价格更重要）

| 层级 | 相对官方成本 | 典型分组 | 判断 | 用法 |
|---|---|---|---|---|
| A · 官转/官方渠道 | 30%~100% | PackyCode `bailian` 0.5、ClaudeCN `国产模型` 0.9、yomiapi `DeepSeek 优选` 0.6、91token `官转渠道` 0.5、ikunapi `神州模型` 0.3、lighthub `国模分组` 0.25 | 价格仍在上游成本线附近，可信度最高 | 可直接做主线路 |
| B · 促销/补贴/批量 | 15%~30% | lighthub `GLM专属` 0.15、cheapai `国模低价` 0.2、91token `特惠渠道` 0.2、modelflare `*-award` | 低于上游零售成本，通常是批量/免费额度/补贴，随时可能取消 | 灰度 10% 流量，跑 7 天 |
| C · 订阅号池/逆向 | 2%~15% | cheapai `claude-kiro-*`、lighthub `claude kiro`、ikunapi `Kiro_史低`、modelflare `claude-tmp-cheap` | 拿 Claude Max / Codex Pro / Kiro 订阅账号转 API，封号即断 | 只做内部/非 SLA 场景 |

> 关键提醒：第 6 节的毛利率是**账面毛利**。报价低于官方成本 50% 的（DeepSeek 0.3、GLM-Flash 0.12、Qwen 2.4、Wan 0.156）本质上都在 B/C 层，必须完成 7.2 的同题对拍再切流量，否则省下的成本会变成用户投诉和退款。

> 结论：**优先换 `glm-5-3-flash`（lighthub）、`qwen3-8-flash`（PackyCode）、`wan-3-0`（91token）、DeepSeek 两条（91token/ikunapi）**；`glm-5-3-prime` 没有替代品；Claude/GPT 的 2 折价不要碰。

## 7. 风险与验证协议

### 7.1 四类风险（按严重度）
1. **模型映射/降智**：`ikunapi` 的 `GPT-国模映射` 分组直接把 GPT 请求映射到 GLM-5.3；`cheapai` 的 `国产模型-全网最低` 明确写缓存低。价格低于官方 50% 的，先假设它「不是同一个东西」。
2. **订阅号池/逆向**：Kiro、Claude Max、Codex Pro 号池（cheapai 0.115、lighthub 0.12、ikunapi 0.11）是拿编程订阅账号转 API，随时封号、限速、降智，不适合对外 SLA 售卖。
3. **资金风险**：这 13 家全是预充值。新站（cheapai 59 天、linoroute 15 天、yomiapi 77 天）建议小额多次，不要年付。
4. **合规**：转售 Claude/OpenAI/Gemini 需厂商授权；国内公众运营境外模型本身有法律风险（我们此前的结论不变）。国产模型（DeepSeek/GLM/Qwen/火山）走原厂或授权聚合更安全。

### 7.2 接入前必须做的 3 个对拍（预算 < ¥50）
1. **同题对拍**：准备 20 条固定 prompt（代码/长文/工具调用各 1/3），同时打旧线路和新线路，比较输出长度、token 用量、延迟、缓存命中率。若新线路 token 数明显偏低但输出变短，就是降智。
2. **缓存验证**：连续两次相同长上下文请求，看 `prompt_tokens_details.cached_tokens` 是否真的命中；`cheapai` 的低价组明确说缓存低，长上下文场景可能反而更贵。
3. **小流量灰度**：在 `model_routes` 新增 priority=5 的线路、weight=10，跑 3 天；用 `/admin` 的成本报表对比 `cost_rule` 与账单，确认没有隐藏倍率（尤其按次计费的图像/视频）。

## 8. 附件

- 全量比价 CSV（917 行，按输入价升序）：`docs/research/relaypulse-prices-2026-10-30.csv`
- 字段：`url, provider, model, billing, cny_in, cny_out, cny_cache, cny_cw, cny_call, group, gratio, recharge, ref_max, discount, expr, endpoint`
- `ref_max` = 该站同模型最高分组价（近似官方价参考），`discount` = 最低价/最高价，可用来判断这家店的渠道折扣力度。

---
生成时间：2026-10-30 · 数据为公开接口实时抓取，价格随时变动，接入前请以站内「模型广场」登录后价格为准。
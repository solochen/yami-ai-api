#!/usr/bin/env python3
"""用真实账单反推 Vidu 实时数字人的实际积分费率。

背景：Vidu 官方价目表（platform.vidu.cn/docs/pricing.md）写 S2 实时版
1.5 积分/秒，但限时套餐页的容量估算用的是 1 积分/秒（3154 积分 → 53 分钟），
两者矛盾。营销文案和文档都可能滞后，账单不会。

用法：
    VIDU_API_KEY=vda_xxx python3 scripts/vidu-live-rate-check.py
    VIDU_API_KEY=vda_xxx VIDU_HOST=api.vidu.cn python3 scripts/vidu-live-rate-check.py --pages 3

只读：只调用 GET /live/v1/lives 列表接口，不会创建或结束任何会话。
Key 从环境变量读取，不落盘、不打印。
"""

from __future__ import annotations

import argparse
import json
import os
import statistics
import sys
import urllib.error
import urllib.request

# 积分单价（元）。标准价与限时 7 折套餐价。
CREDIT_PRICE_STANDARD = 0.03125
CREDIT_PRICE_PROMO = 0.021875

# 文档声明的两种费率（积分/秒）。
RATE_REALTIME = 1.5
RATE_COMPONENT = 1.0


def fetch_lives(host: str, key: str, page: int, size: int) -> dict:
    url = f"https://{host}/live/v1/lives?pager.page={page}&pager.pagesz={size}"
    request = urllib.request.Request(url, headers={"Authorization": f"Token {key}"})
    with urllib.request.urlopen(request, timeout=30) as response:
        return json.loads(response.read().decode("utf-8"))


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--host", default=os.environ.get("VIDU_HOST", "api.vidu.cn"))
    parser.add_argument("--pages", type=int, default=2, help="翻多少页（每页 50 条）")
    parser.add_argument("--promo", action="store_true", help="已购限时 7 折套餐，按 0.021875 元/积分折算")
    args = parser.parse_args()

    key = os.environ.get("VIDU_API_KEY", "").strip()
    if not key:
        print("缺少 VIDU_API_KEY 环境变量。后台「系统配置 → Vidu API Key」里可以取到。")
        return 2

    rows: list[tuple[str, str, int, float]] = []
    for page in range(max(1, args.pages)):
        try:
            payload = fetch_lives(args.host, key, page, 50)
        except urllib.error.HTTPError as err:
            print(f"请求失败：HTTP {err.code} {err.reason}")
            return 1
        except Exception as err:  # noqa: BLE001 - 诊断脚本，直接暴露原因
            print(f"请求失败：{err}")
            return 1
        for live in payload.get("lives") or []:
            seconds = int(live.get("billed_seconds") or 0)
            credits = float(live.get("credits_cost") or 0)
            if seconds > 0 and credits > 0:
                rows.append((str(live.get("id")), str(live.get("call_mode")), seconds, credits))

    if not rows:
        print("没有找到已计费的会话。先跑一通实时数字人通话（>30 秒）再执行本脚本。")
        return 0

    print(f"{'live_id':<22}{'mode':<8}{'秒':>7}{'积分':>10}{'积分/秒':>9}{'标准价元/分':>13}{'7折元/分':>11}")
    print("-" * 82)
    ratios: list[float] = []
    for live_id, mode, seconds, credits in rows[:40]:
        per_second = credits / seconds
        ratios.append(per_second)
        print(
            f"{live_id:<22}{mode:<8}{seconds:>7}{credits:>10.1f}{per_second:>9.3f}"
            f"{per_second * 60 * CREDIT_PRICE_STANDARD:>13.3f}"
            f"{per_second * 60 * CREDIT_PRICE_PROMO:>11.3f}"
        )

    median = statistics.median(ratios)
    print("-" * 82)
    print(f"会话数 {len(rows)}，积分/秒 中位数 {median:.3f}")

    if abs(median - RATE_REALTIME) < abs(median - RATE_COMPONENT):
        verdict, rate = "实时版 1.5 积分/秒", RATE_REALTIME
    else:
        verdict, rate = "组件版/非实时版 1 积分/秒", RATE_COMPONENT
    print(f"结论：实际扣费接近「{verdict}」，套餐页的容量估算{'偏乐观' if rate < RATE_REALTIME else '一致'}。")
    print(f"成本折算：标准价 {rate * 60 * CREDIT_PRICE_STANDARD:.4f} 元/分钟；"
          f"7 折套餐 {rate * 60 * CREDIT_PRICE_PROMO:.4f} 元/分钟。")
    print("\n把这个结果填回后台：")
    print(f"  vidu_text_credit_rate        = {CREDIT_PRICE_PROMO if args.promo else CREDIT_PRICE_STANDARD}")
    per_minute = rate * 60 * (CREDIT_PRICE_PROMO if args.promo else CREDIT_PRICE_STANDARD)
    print(f"  vidu_voice_price_per_minute  = {per_minute:.6f}")
    print(f"  vidu_video_price_per_minute  = {per_minute:.6f}")
    return 0


if __name__ == "__main__":
    sys.exit(main())

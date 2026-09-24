#!/usr/bin/env python3
"""Create the two OpenRouter DeepSeek chat models currently in use.

API Key defaults to a placeholder so you can replace it later in 模型管理.

Usage:
  python3 scripts/batch-create-openrouter-deepseek-models.py --dry-run
  python3 scripts/batch-create-openrouter-deepseek-models.py --prune \\
    --admin-email admin@starai.local --admin-password admin123 \\
    --api-key sk-or-v1-replace-me
"""

from __future__ import annotations

import argparse
import json
import ssl
import sys
import urllib.error
import urllib.request
from typing import Any

# Only the two snapshot chat models currently needed.
MODELS: list[dict[str, Any]] = [
    {
        "id": "deepseek/deepseek-v4-pro-0813",
        "code": "deepseek-v4-pro-0813",
        "name": "DeepSeek V4 Pro 0813",
        "tags": ["对话", "DeepSeek", "推理"],
        "description": "DeepSeek V4 Pro 0813，适合复杂对话与推理；支持思考开关。",
        "think": "thinking_type",
        "vision": False,
        "sort": 10,
    },
    {
        "id": "deepseek/deepseek-v4-flash-0731",
        "code": "deepseek-v4-flash-0731",
        "name": "DeepSeek V4 Flash 0731",
        "tags": ["对话", "DeepSeek", "推理"],
        "description": "DeepSeek V4 Flash 0731，速度快；支持思考开关。",
        "think": "thinking_type",
        "vision": False,
        "sort": 20,
    },
]

# Previously imported DeepSeek codes that are no longer wanted. --prune removes
# only these, and never deletes chat_demo_v1 or other unrelated models.
PRUNE_CODES = {
    "deepseek-v4-1-flash",
    "deepseek-v4-pro",
    "deepseek-v4-flash",
    "deepseek-v4-flash-vision-exp",
    "deepseek-v3-2",
    "deepseek-v3-2-exp",
    "deepseek-v3-1-terminus",
    "deepseek-chat-v3-1",
    "deepseek-chat-v3-0324",
    "deepseek-chat",
    "deepseek-r1",
    "deepseek-r1-0528",
    "deepseek-r1-distill-llama-70b",
}


def runtime_rule(item: dict[str, Any]) -> dict[str, Any]:
    caps: dict[str, Any] = {"web_search": False, "deep_think": bool(item["think"])}
    if item["vision"]:
        caps["vision"] = True
        caps["image_input"] = True
    rule: dict[str, Any] = {"capabilities": caps}
    if item["think"]:
        rule["reasoning"] = {"mode": item["think"]}
    return rule


def payload_for(item: dict[str, Any], args: argparse.Namespace) -> dict[str, Any]:
    return {
        "code": item["code"],
        "display_name": item["name"],
        "icon_url": "",
        "new_api_model": item["id"],
        "new_api_endpoint": "/v1/chat/completions",
        "request_mode": "chat_completions",
        "category": "chat",
        "description": item["description"],
        "tags": item["tags"],
        "input_schema": {},
        "default_params": {},
        "new_api_extra_params": {
            "connection": {
                "provider": "openrouter",
                "protocol": "openai_compatible",
                "base_url": args.base_url.rstrip("/"),
                "api_key": args.api_key,
                "auth_type": "bearer",
                "api_key_header": "Authorization",
                "models_endpoint": "/v1/models",
            }
        },
        "price_rule": {
            "billing_type": "per_token",
            "currency": "¥",
            "input_price_per_m": 2,
            "output_price_per_m": 8,
            "cache_read_price_per_m": 0.2,
        },
        "runtime_rule": runtime_rule(item),
        "is_enabled": True,
        "sort_order": item["sort"],
    }


def request_json(url: str, method: str, token: str | None, body: dict[str, Any] | None = None) -> Any:
    data = None if body is None else json.dumps(body).encode("utf-8")
    headers = {"Content-Type": "application/json", "Accept": "application/json"}
    if token:
        headers["Authorization"] = "Bearer " + token
    req = urllib.request.Request(url, data=data, headers=headers, method=method)
    ctx = ssl.create_default_context()
    try:
        with urllib.request.urlopen(req, context=ctx, timeout=30) as resp:
            raw = resp.read().decode("utf-8")
    except urllib.error.HTTPError as exc:
        raw = exc.read().decode("utf-8", errors="replace")
        try:
            parsed = json.loads(raw)
            message = parsed.get("message") or raw
        except json.JSONDecodeError:
            message = raw or str(exc)
        raise RuntimeError(f"HTTP {exc.code}: {message}") from exc
    parsed = json.loads(raw) if raw else {}
    if parsed.get("code") not in (0, None):
        raise RuntimeError(parsed.get("message") or raw)
    return parsed.get("data", parsed)


def login(args: argparse.Namespace) -> str:
    data = request_json(
        args.api_url.rstrip("/") + "/admin/api/login",
        "POST",
        None,
        {"email": args.admin_email, "password": args.admin_password},
    )
    token = (data or {}).get("token")
    if not token:
        raise RuntimeError("登录成功但未返回 token")
    return token


def existing_models(args: argparse.Namespace, token: str) -> dict[str, int]:
    items = request_json(args.api_url.rstrip("/") + "/admin/api/models", "GET", token) or []
    return {str(item.get("code") or ""): int(item["id"]) for item in items if item.get("code") and item.get("id")}


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Batch create OpenRouter DeepSeek chat models")
    parser.add_argument("--api-url", default="http://localhost:8080")
    parser.add_argument("--admin-email", default="admin@starai.local")
    parser.add_argument("--admin-password", default="admin123")
    parser.add_argument("--api-key", default="sk-or-v1-replace-me")
    parser.add_argument("--base-url", default="https://openrouter.ai/api/v1")
    parser.add_argument("--dry-run", action="store_true", help="Print payloads without creating")
    parser.add_argument("--prune", action="store_true", help="Delete previously imported extra DeepSeek models")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    print(f"将创建 {len(MODELS)} 个 DeepSeek 聊天模型（OpenRouter）")
    if args.dry_run:
        for item in MODELS:
            print(f"  [dry-run] {item['code']:36} {item['id']}")
        if args.prune:
            for code in sorted(PRUNE_CODES):
                print(f"  [dry-run] prune {code}")
        return 0

    token = login(args)
    known = existing_models(args, token)
    pruned = prune_failed = 0
    if args.prune:
        for code in sorted(PRUNE_CODES):
            model_id = known.get(code)
            if not model_id:
                continue
            try:
                request_json(args.api_url.rstrip("/") + f"/admin/api/models/{model_id}", "DELETE", token)
                print(f"  prune {code}")
                known.pop(code, None)
                pruned += 1
            except Exception as exc:  # noqa: BLE001 — report each row and continue
                print(f"  fail  prune {code}: {exc}", file=sys.stderr)
                prune_failed += 1

    created = skipped = failed = 0
    for item in MODELS:
        if item["code"] in known:
            print(f"  skip  {item['code']}（编码已存在）")
            skipped += 1
            continue
        try:
            request_json(
                args.api_url.rstrip("/") + "/admin/api/models",
                "POST",
                token,
                payload_for(item, args),
            )
            print(f"  ok    {item['code']:36} {item['id']}")
            created += 1
            known[item["code"]] = 0
        except Exception as exc:  # noqa: BLE001 — report each row and continue
            print(f"  fail  {item['code']}: {exc}", file=sys.stderr)
            failed += 1
    print(f"完成：创建 {created}，跳过 {skipped}，清理 {pruned}，失败 {failed + prune_failed}")
    if created:
        print("请到管理后台 → 模型管理，把占位 API Key 改成真实的 OpenRouter Key。")
    return 1 if failed or prune_failed else 0


if __name__ == "__main__":
    raise SystemExit(main())

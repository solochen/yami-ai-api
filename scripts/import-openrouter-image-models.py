#!/usr/bin/env python3
"""Add the selected OpenRouter image models to the model catalog.

Capabilities and USD prices are read from
https://openrouter.ai/api/v1/images/models and each model's endpoints
document. User-facing 算力 uses the platform rate of 7.2 per USD
(1 算力 ≈ 1 RMB). The OpenRouter key is copied from an existing route
and is never printed.
"""

from __future__ import annotations

import json
import subprocess
import urllib.request
from decimal import Decimal, ROUND_HALF_UP

SOURCE_CODE = "deepseek-v4-pro-0813"
CATALOG = "https://openrouter.ai/api/v1/images/models"
USD_TO_COMPUTE = Decimal("7.2")
UI_RATIOS = {
    "1:1", "16:9", "9:16", "3:2", "2:3", "4:3", "3:4", "5:4", "4:5",
    "7:3", "3:7", "21:9", "9:21", "2:1", "1:2", "3:1", "1:3", "4:1", "1:4",
}

# Chinese copy stays inside the official model description.
COPY = {
    "bytedance-seed/seedream-5-0-pro": "ByteDance Seed 的图像生成与编辑模型，适合需要精确编辑、真实场景和自然画面的商业视觉制作。",
    "bytedance-seed/seedream-5-0-lite": "ByteDance Seed 的图像生成模型，适合需要联网检索、复杂提示词理解和参考图的专业视觉创作。",
    "bytedance-seed/seedream-4.5": "ByteDance 自研图像生成模型，相对 Seedream 4.0 加强了编辑一致性，能更好保留主体细节。",
    "openai/gpt-image-2.5-sunburst": "OpenAI GPT Image 2.5 系列里偏精度的图像生成与编辑型号，适合细节要求高的创作。",
    "openai/gpt-image-2.5-flare": "OpenAI GPT Image 2.5 系列里偏速度的图像生成与编辑型号，适合大批量日常出图。",
    "openai/gpt-image-2": "OpenAI 的图像生成模型，通过图片接口支持高保真生成和编辑。",
    "google/gemini-3-pro-image": "Google 基于 Gemini 3 Pro 的图像生成与编辑模型，加强了多模态推理和现实场景理解。",
    "google/gemini-2.5-flash-image": "Google 的图像生成模型，具备上下文理解，可用于生成和编辑图片。",
    "x-ai/grok-imagine-image-2.0": "xAI 的图像生成与编辑模型，可根据文字提示出图，也可按参考图编辑。",
    "qwen/qwen-image-3": "通义的统一图像生成与编辑模型，支持精细文字和约 10 像素级细节。",
    "qwen/qwen-image-3-pro": "通义的图像生成与编辑模型，支持精细文字和约 10 像素级细节，并带有更丰富的世界知识。",
}

# Official output prices, in USD, checked against the endpoints document before insert.
# Seedream 5.0 Pro's unscoped image price is 1K; high_resolution is its 2K rate.
# Grok prices are quality × resolution. Gemini 2.5 uses the standard Google AI Studio rate.
PRICES = {
    "bytedance-seed/seedream-5-0-pro": {"per_image": {"1K": "0.045", "2K": "0.09"}, "expect": ["0.045", "0.09"]},
    "bytedance-seed/seedream-5-0-lite": {"per_image": {"2K": "0.035", "4K": "0.035"}, "expect": ["0.035"]},
    "bytedance-seed/seedream-4.5": {"per_image": {"1K": "0.04", "2K": "0.04", "4K": "0.04"}, "expect": ["0.04"]},
    "x-ai/grok-imagine-image-2.0": {
        "per_variant": {"low_1k": "0.04", "low_2k": "0.06", "medium_1k": "0.06", "medium_2k": "0.08"},
        "expect": ["0.04", "0.06", "0.08"],
    },
    "qwen/qwen-image-3": {"per_image": {"1K": "0.03", "2K": "0.03"}, "expect": ["0.03"]},
    "qwen/qwen-image-3-pro": {"per_image": {"1K": "0.04", "2K": "0.075"}, "expect": ["0.04", "0.075"]},
    "openai/gpt-image-2.5-sunburst": {"input_token": "0.000008", "output_token": "0.00003", "estimated_output_tokens": 8000},
    "openai/gpt-image-2.5-flare": {"input_token": "0.000008", "output_token": "0.00003", "estimated_output_tokens": 8000},
    "openai/gpt-image-2": {"input_token": "0.000008", "output_token": "0.00003", "estimated_output_tokens": 8000},
    "google/gemini-3-pro-image": {"input_token": "0.000002", "output_token": "0.00012", "estimated_output_tokens": 2000},
    "google/gemini-2.5-flash-image": {"input_token": "0.0000003", "output_token": "0.00003", "estimated_output_tokens": 4000, "provider_slug": "google-ai-studio"},
}


def fetch_json(url: str) -> dict:
    request = urllib.request.Request(url, headers={"Accept": "application/json", "User-Agent": "starai-model-import"})
    with urllib.request.urlopen(request, timeout=60) as response:
        return json.load(response)


def money(value: str | Decimal) -> float:
    amount = (Decimal(value) * USD_TO_COMPUTE).quantize(Decimal("0.000001"), rounding=ROUND_HALF_UP)
    return float(amount)


def sql_text(value: str) -> str:
    return "'" + value.replace("'", "''") + "'"


def sql_json(value: object) -> str:
    return "'" + json.dumps(value, ensure_ascii=False).replace("'", "''") + "'::jsonb"


def listed_costs(endpoint: dict) -> set[str]:
    found = set()
    for line in endpoint.get("pricing") or []:
        cost = line.get("cost_usd")
        if cost is None:
            continue
        found.add(format(Decimal(str(cost)).normalize(), "f"))
    return found


def require_costs(model_id: str, endpoints: list[dict], expected: list[str], provider_slug: str | None = None) -> None:
    chosen = endpoints
    if provider_slug:
        chosen = [item for item in endpoints if item.get("provider_slug") == provider_slug]
        if not chosen:
            raise SystemExit(f"{model_id} missing provider {provider_slug}")
    found: set[str] = set()
    for item in chosen:
        found |= listed_costs(item)
    missing = [item for item in expected if item not in found and format(Decimal(item).normalize(), "f") not in found]
    if missing:
        raise SystemExit(f"{model_id} official price changed, missing {missing}, saw {sorted(found)}")


def quality_schema(values: list[str]) -> dict:
    return {
        "type": "object",
        "properties": {
            "quality": {
                "type": "string",
                "title": "清晰度",
                "enum": values,
                "default": "auto" if "auto" in values else values[0],
                "x-order": 1,
                "x-widget": "option_menu",
            }
        },
    }


def main() -> None:
    catalog = {item["id"]: item for item in fetch_json(CATALOG).get("data", [])}
    missing = [model_id for model_id in COPY if model_id not in catalog]
    if missing:
        raise SystemExit("OpenRouter image catalog missing: " + ", ".join(missing))

    statements = ["BEGIN;"]
    for index, model_id in enumerate(COPY):
        official = catalog[model_id]
        params = official.get("supported_parameters") or {}
        endpoints = fetch_json("https://openrouter.ai" + official["endpoints"]).get("endpoints") or []
        price = PRICES[model_id]
        require_costs(model_id, endpoints, price.get("expect") or [price["input_token"], price["output_token"]], price.get("provider_slug"))

        ratios = [item for item in (params.get("aspect_ratio") or {}).get("values") or [] if item in UI_RATIOS]
        tiers = [item for item in (params.get("resolution") or {}).get("values") or [] if item in {"1K", "2K", "4K"}]
        count_max = int((params.get("n") or {}).get("max") or 1)
        ref_max = int((params.get("input_references") or {}).get("max") or 0)
        qualities = list((params.get("quality") or {}).get("values") or [])
        code = model_id.split("/", 1)[1].replace(".", "-")
        name = official["name"].split(": ", 1)[-1]
        image_rule: dict = {
            "max_reference_images": ref_max,
            "supported_ratios": ratios,
            "allow_auto_ratio": "auto" in ((params.get("aspect_ratio") or {}).get("values") or []),
            "count_max": count_max,
            "count_options": list(range(1, min(4, count_max) + 1)),
        }
        if tiers:
            image_rule["supported_size_tiers"] = tiers
        runtime = {
            "capabilities": {"vision": True, "image_input": ref_max > 0},
            "upstream": {"adapter": "openrouter_image"},
            "image": image_rule,
        }
        defaults: dict = {"aspect_ratio": "1:1", "n": 1}
        if tiers:
            defaults["image_size"] = tiers[0]
        if qualities:
            defaults["quality"] = "auto" if "auto" in qualities else qualities[0]
        if "per_image" in price or "per_variant" in price:
            by_size = price.get("per_image") or {}
            user_price: dict = {
                "billing_type": "per_image",
                "currency": "¥",
                "usd_to_compute": float(USD_TO_COMPUTE),
                "unit_price": money(next(iter(by_size.values())) if by_size else next(iter(price["per_variant"].values()))),
            }
            if by_size:
                user_price["unit_price_by_size"] = {tier: money(amount) for tier, amount in by_size.items()}
                user_price["unit_price"] = money(by_size[tiers[0]] if tiers and tiers[0] in by_size else next(iter(by_size.values())))
            if price.get("per_variant"):
                user_price["unit_price_by_variant"] = {key: money(amount) for key, amount in price["per_variant"].items()}
                user_price["unit_price"] = money(price["per_variant"]["low_1k"])
            source_usd = by_size.get(tiers[0]) if tiers and tiers and tiers[0] in by_size else (next(iter(by_size.values())) if by_size else price["per_variant"]["low_1k"])
            cost_rule = {"billing_type": "per_image", "unit_cost": float(Decimal(source_usd))}
            if by_size:
                cost_rule["unit_cost_by_size"] = {tier: float(Decimal(amount)) for tier, amount in by_size.items()}
            if price.get("per_variant"):
                cost_rule["unit_cost_by_variant"] = {key: float(Decimal(amount)) for key, amount in price["per_variant"].items()}
        else:
            user_price = {
                "billing_type": "per_token",
                "currency": "¥",
                "usd_to_compute": float(USD_TO_COMPUTE),
                "input_price_per_m": money(Decimal(price["input_token"]) * Decimal(1_000_000)),
                "output_price_per_m": money(Decimal(price["output_token"]) * Decimal(1_000_000)),
                "estimated_output_tokens": price["estimated_output_tokens"],
            }
            cost_rule = {
                "billing_type": "per_token",
                "input_cost_per_m": float(Decimal(price["input_token"]) * Decimal(1_000_000)),
                "output_cost_per_m": float(Decimal(price["output_token"]) * Decimal(1_000_000)),
            }
        vendor = "ByteDance" if model_id.startswith("bytedance") else "xAI" if model_id.startswith("x-ai") else model_id.split("/", 1)[0]
        tags = ["图片", vendor, name.split()[0]]
        schema = quality_schema(qualities) if qualities else {}
        statements.append(f"""
INSERT INTO models (
  code, display_name, new_api_model, new_api_endpoint, request_mode, category,
  description, tags, input_schema, default_params, new_api_extra_params, price_rule,
  runtime_rule, is_enabled, sort_order
)
SELECT
  {sql_text(code)}, {sql_text(name)}, {sql_text(model_id)}, '/images', 'images', 'image',
  {sql_text(COPY[model_id])}, {sql_json(tags)}, {sql_json(schema)}, {sql_json(defaults)}, src.new_api_extra_params,
  {sql_json(user_price)}, {sql_json(runtime)}, true, {3 + index}
FROM models src
WHERE src.code = '{SOURCE_CODE}'
ON CONFLICT (code) DO UPDATE SET
  display_name = EXCLUDED.display_name,
  new_api_model = EXCLUDED.new_api_model,
  new_api_endpoint = EXCLUDED.new_api_endpoint,
  request_mode = EXCLUDED.request_mode,
  category = EXCLUDED.category,
  description = EXCLUDED.description,
  tags = EXCLUDED.tags,
  input_schema = EXCLUDED.input_schema,
  default_params = EXCLUDED.default_params,
  price_rule = EXCLUDED.price_rule,
  runtime_rule = EXCLUDED.runtime_rule,
  is_enabled = true,
  sort_order = EXCLUDED.sort_order,
  updated_at = now();
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url, api_key,
  auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule, priority, weight,
  timeout_seconds, max_retries, is_enabled
)
SELECT m.id, 'OpenRouter', src.provider, src.protocol, {sql_text(model_id)}, '/images', src.base_url, src.api_key,
  src.auth_type, src.api_key_header, src.headers, '{{}}'::jsonb, '{{}}'::jsonb, {sql_json(cost_rule)}, src.priority, src.weight,
  600, 0, true
FROM models m
JOIN model_routes src ON src.model_id = (SELECT id FROM models WHERE code = '{SOURCE_CODE}') AND src.is_enabled = true
WHERE m.code = {sql_text(code)}
  AND NOT EXISTS (
    SELECT 1 FROM model_routes existing WHERE existing.model_id = m.id AND existing.upstream_model = {sql_text(model_id)}
  );
UPDATE model_routes SET endpoint='/images', cost_rule={sql_json(cost_rule)}, timeout_seconds=600, max_retries=0, is_enabled=true
WHERE model_id = (SELECT id FROM models WHERE code = {sql_text(code)}) AND upstream_model = {sql_text(model_id)};
""")
    statements.append("COMMIT;")
    subprocess.run(
        ["docker", "exec", "-i", "docker-postgres-1", "psql", "-U", "starai", "-d", "starai", "-v", "ON_ERROR_STOP=1"],
        input="\n".join(statements).encode(),
        check=True,
    )
    print(f"imported {len(COPY)} image models")


if __name__ == "__main__":
    main()

#!/usr/bin/env python3
"""Add OpenRouter text/image-to-video models to the model catalog.

Capabilities and USD rates come from
https://openrouter.ai/api/v1/videos/models. Models that require an existing
video or a voice track are skipped, because the model workspace cannot supply
those inputs. User-facing 算力 uses 7.2 per USD. The final charge follows
OpenRouter usage.cost when the job completes. The API key is copied from an
existing route and is never printed.
"""

from __future__ import annotations

import json
import subprocess
import urllib.request
from decimal import Decimal, ROUND_HALF_UP

SOURCE_CODE = "deepseek-v4-pro-0813"
CATALOG = "https://openrouter.ai/api/v1/videos/models"
USD_TO_COMPUTE = Decimal("7.2")
RES_ALIASES = {
    "480p": ["480p"],
    "720p": ["720p"],
    "768p": ["768p"],
    "1080p": ["1080p"],
    "1k": ["1k"],
    "2k": ["2k"],
    "4k": ["4k"],
}

COPY = {
    "minimax/hailuo-3-max": "MiniMax 基于 H3 继续训练的视频模型，面向更快的文生视频。",
    "alibaba/wan-3.0-prime": "阿里 Wan 3.0 的快速版本，支持文字出视频，也可用首帧图生视频。",
    "alibaba/wan-3.0": "阿里的视频生成模型，支持文生视频、图生视频和参考图引导。",
    "bytedance/seedance-2.0-mini": "ByteDance Seedance 2.0 Mini，支持文生视频、首尾帧图生视频和多参考图。",
    "bytedance/seedance-2.5": "ByteDance Seedance 2.5，适合较长叙事、多模态参考、视频编辑和续写。",
    "black-forest-labs/flux-3-video": "Black Forest Labs 的视频模型，支持文生视频、首尾关键帧和视频续写。",
    "minimax/hailuo-3": "MiniMax 的轻量视频模型，支持按指令做多模态编辑和受控生成。",
    "runway/gen-4.5": "Runway 的视频模型，支持文生视频和图生视频，强调电影感运动。",
    "x-ai/grok-imagine-video-1.5": "xAI 的视频模型，可用文字生成，也可用一张起始图引导画面。",
    "alibaba/happyhorse-1.1": "阿里的短视频模型，可用文字、一张起始图或一组参考图生成。",
    "alibaba/happyhorse-1.0": "阿里的短视频模型，可用文字、一张起始图或一组参考图生成。",
    "x-ai/grok-imagine-video": "xAI 的快速视频模型，支持文字、图片和参考条件，时长 1 到 15 秒。",
    "kwaivgi/kling-v3.0-pro": "可灵 v3.0 Pro，画质高于标准档，支持文生视频和图生视频，可控制首尾帧。",
    "kwaivgi/kling-v3.0-std": "可灵 v3.0 标准档，支持文生视频和图生视频，可控制首尾帧。",
    "google/veo-3.1-fast": "Google 速度与画质折中的视频模型，可由文字或图片生成，并带同步声音。",
    "google/veo-3.1-lite": "Google 成本更低的视频模型，适合大批量和快速试片，可由文字或图片生成。",
    "kwaivgi/kling-video-o1": "可灵 Video O1，支持文字和图片输入并输出视频。",
    "minimax/hailuo-2.3": "MiniMax Hailuo 2.3，接受文字和参考图，支持文生视频和图生视频。",
    "bytedance/seedance-2.0": "ByteDance Seedance 2.0，支持文生视频、首尾帧图生视频和多模态参考。",
    "bytedance/seedance-2.0-fast": "ByteDance Seedance 2.0 的快速版本，支持文生视频、首尾帧和多模态参考。",
    "alibaba/wan-2.7": "阿里 Wan 2.7，支持文生视频、首尾帧图生视频和多参考图。",
    "alibaba/wan-2.6": "阿里 Wan 2.6，可由文字、图片和参考素材生成视频。",
    "bytedance/seedance-1-5-pro": "ByteDance Seedance 1.5 Pro，可同时生成视频和配套声音。",
    "openai/sora-2-pro": "OpenAI 的视频模型，强调物理运动、同步声音和镜头间的场景延续。",
    "google/veo-3.1": "Google 面向成片画质的视频模型，可由文字或图片生成高清视频。",
}


def fetch_json(url: str) -> dict:
    request = urllib.request.Request(url, headers={"Accept": "application/json", "User-Agent": "starai-model-import"})
    with urllib.request.urlopen(request, timeout=60) as response:
        return json.load(response)


def money(value: Decimal) -> float:
    return float(value.quantize(Decimal("0.000001"), rounding=ROUND_HALF_UP))


def sql_text(value: str) -> str:
    return "'" + value.replace("'", "''") + "'"


def sql_json(value: object) -> str:
    return "'" + json.dumps(value, ensure_ascii=False).replace("'", "''") + "'::jsonb"


def matches_resolution(key: str, resolution: str) -> bool:
    lowered = key.lower()
    return any(alias in lowered for alias in RES_ALIASES.get(resolution.lower(), [resolution.lower()]))


def is_time_rate(key: str) -> bool:
    lowered = key.lower()
    return "video_tokens" not in lowered and ("duration" in lowered or "per_second" in lowered or "second" in lowered)


def usd_amount(key: str, value: str) -> Decimal:
    amount = Decimal(str(value))
    if "cents" in key.lower():
        return amount / Decimal(100)
    return amount


def token_rate(skus: dict, resolution: str) -> Decimal | None:
    generic = None
    for key, value in skus.items():
        lowered = key.lower()
        if "video_tokens" not in lowered or "video_input" in lowered or "without_audio" in lowered:
            continue
        if matches_resolution(key, resolution):
            return Decimal(str(value))
        if lowered == "video_tokens":
            generic = Decimal(str(value))
    return generic


def pick_size(sizes: list[str], aspect: str, resolution: str) -> tuple[int, int] | None:
    if not sizes:
        return None
    width_ratio, height_ratio = (Decimal(part) for part in aspect.split(":"))
    target = width_ratio / height_ratio
    short_side = {"480p": 480, "720p": 720, "768p": 768, "1080p": 1080, "1k": 1080, "2k": 1440, "4k": 2160}.get(resolution.lower())
    best = None
    best_score = None
    for raw in sizes:
        width, height = (int(part) for part in raw.lower().split("x"))
        ratio_gap = abs((Decimal(width) / Decimal(height)) - target)
        short_gap = 0 if short_side is None else abs(min(width, height) - short_side)
        score = ratio_gap * 1000 + Decimal(short_gap)
        if best_score is None or score < best_score:
            best = (width, height)
            best_score = score
    return best


def rate_for(skus: dict, resolution: str, sizes: list[str], aspect: str) -> Decimal | None:
    ranked = []
    for key, value in skus.items():
        if not is_time_rate(key) or "video_input" in key or "without_audio" in key or "continuation" in key:
            continue
        if not matches_resolution(key, resolution):
            continue
        rank = 0 if "with_audio" in key else 1
        ranked.append((rank, usd_amount(key, value)))
    if ranked:
        ranked.sort(key=lambda item: item[0])
        return ranked[0][1]
    generic = []
    for key, value in skus.items():
        if not is_time_rate(key) or "video_input" in key or "without_audio" in key or "continuation" in key:
            continue
        if any(alias in key.lower() for aliases in RES_ALIASES.values() for alias in aliases):
            continue
        rank = 0 if "with_audio" in key else 1
        generic.append((rank, usd_amount(key, value)))
    if generic:
        generic.sort(key=lambda item: item[0])
        return generic[0][1]
    rate = token_rate(skus, resolution)
    size = pick_size(sizes, aspect, resolution)
    if rate is None or size is None:
        return None
    width, height = size
    tokens = Decimal(width * height * 24) / Decimal(1024)
    return tokens * rate


def prefer(values: list, preferred: list):
    for item in preferred:
        if item in values:
            return item
    return values[0]


def main() -> None:
    catalog = fetch_json(CATALOG).get("data") or []
    selected = []
    skipped = []
    for model in catalog:
        if model["id"] not in COPY:
            if not model.get("supported_durations"):
                skipped.append(model["id"])
            else:
                raise SystemExit("missing Chinese copy for " + model["id"])
            continue
        if not model.get("supported_durations"):
            skipped.append(model["id"])
            continue
        selected.append(model)
    missing = [model_id for model_id in COPY if model_id not in {item["id"] for item in catalog}]
    if missing:
        raise SystemExit("OpenRouter video catalog missing: " + ", ".join(missing))

    statements = ["BEGIN;"]
    for index, model in enumerate(selected):
        model_id = model["id"]
        durations = [int(item) for item in model["supported_durations"]]
        resolutions = [str(item) for item in model.get("supported_resolutions") or []]
        ratios = [str(item) for item in model.get("supported_aspect_ratios") or []]
        sizes = [str(item) for item in model.get("supported_sizes") or []]
        frames = [str(item) for item in model.get("supported_frame_images") or []]
        if not resolutions or not ratios:
            raise SystemExit(f"{model_id} has no resolution or aspect ratio")
        default_duration = prefer(durations, [5, 4, 6, 8])
        default_resolution = prefer(resolutions, ["720p", "768p", "1080p", "480p", "2K", "1K", "4K"])
        default_ratio = prefer(ratios, ["16:9", "1:1", "9:16"])
        by_resolution = {}
        for resolution in resolutions:
            rate = rate_for(model.get("pricing_skus") or {}, resolution, sizes, default_ratio)
            if rate is None or rate <= 0 or rate > Decimal(20):
                raise SystemExit(f"{model_id} {resolution} price {rate} is missing or implausible")
            by_resolution[resolution] = money(rate * USD_TO_COMPUTE)
        unit = by_resolution[default_resolution]
        code = model_id.split("/", 1)[1].replace(".", "-")
        name = model["name"].split(": ", 1)[-1]
        schema = {
            "type": "object",
            "properties": {
                "duration": {
                    "type": "integer",
                    "title": "时长",
                    "enum": durations,
                    "default": default_duration,
                    "x-order": 1,
                    "x-widget": "option_menu",
                    "enumLabels": {str(item): f"{item} 秒" for item in durations},
                },
                "resolution": {
                    "type": "string",
                    "title": "分辨率",
                    "enum": resolutions,
                    "default": default_resolution,
                    "x-order": 2,
                    "x-widget": "option_menu",
                },
                "aspect_ratio": {
                    "type": "string",
                    "title": "画幅",
                    "enum": ratios,
                    "default": default_ratio,
                    "x-order": 3,
                    "x-widget": "option_menu",
                },
            },
        }
        runtime = {
            "capabilities": {},
            "upstream": {
                "adapter": "openrouter_video",
                "async": True,
                "poll_path": "/videos/{id}",
                "poll_interval_sec": 10,
                "poll_timeout_sec": 900,
            },
            "video": {
                "upload_profile": "single_ref",
                "prompt_required": True,
                "prompt_hint": "可直接用文字生成视频。上传一张参考图时，会作为起始画面。",
                "show_channel": False,
                "max_reference_images": 1,
                "min_reference_images": 0,
                "supported_frame_images": frames,
            },
        }
        defaults = {"duration": default_duration, "resolution": default_resolution, "aspect_ratio": default_ratio}
        user_price = {
            "billing_type": "per_second",
            "currency": "¥",
            "usd_to_compute": float(USD_TO_COMPUTE),
            "unit_price": unit,
            "unit_price_by_resolution": by_resolution,
        }
        cost_rule = {
            "billing_type": "per_second",
            "unit_cost": float((Decimal(str(unit)) / USD_TO_COMPUTE).quantize(Decimal("0.000001"))),
            "unit_cost_by_resolution": {key: float((Decimal(str(value)) / USD_TO_COMPUTE).quantize(Decimal("0.000001"))) for key, value in by_resolution.items()},
        }
        vendor = model_id.split("/", 1)[0]
        print(f"{code:28} {default_resolution:6} {unit} 算力/秒")
        statements.append(f"""
INSERT INTO models (
  code, display_name, new_api_model, new_api_endpoint, request_mode, category,
  description, tags, input_schema, default_params, new_api_extra_params, price_rule,
  runtime_rule, is_enabled, sort_order
)
SELECT
  {sql_text(code)}, {sql_text(name)}, {sql_text(model_id)}, '/videos', 'video', 'video',
  {sql_text(COPY[model_id])}, {sql_json(["视频", vendor, name.split()[0]])}, {sql_json(schema)}, {sql_json(defaults)}, src.new_api_extra_params,
  {sql_json(user_price)}, {sql_json(runtime)}, true, {14 + index}
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
SELECT m.id, 'OpenRouter', src.provider, src.protocol, {sql_text(model_id)}, '/videos', src.base_url, src.api_key,
  src.auth_type, src.api_key_header, src.headers, '{{}}'::jsonb, '{{}}'::jsonb, {sql_json(cost_rule)}, src.priority, src.weight,
  180, 0, true
FROM models m
JOIN model_routes src ON src.model_id = (SELECT id FROM models WHERE code = '{SOURCE_CODE}') AND src.is_enabled = true
WHERE m.code = {sql_text(code)}
  AND NOT EXISTS (
    SELECT 1 FROM model_routes existing WHERE existing.model_id = m.id AND existing.upstream_model = {sql_text(model_id)}
  );
UPDATE model_routes SET endpoint='/videos', cost_rule={sql_json(cost_rule)}, timeout_seconds=180, max_retries=0, is_enabled=true
WHERE model_id = (SELECT id FROM models WHERE code = {sql_text(code)}) AND upstream_model = {sql_text(model_id)};
""")
    statements.append("COMMIT;")
    subprocess.run(
        ["docker", "exec", "-i", "docker-postgres-1", "psql", "-U", "starai", "-d", "starai", "-v", "ON_ERROR_STOP=1"],
        input="\n".join(statements).encode(),
        check=True,
    )
    print(f"imported {len(selected)} video models; skipped {', '.join(skipped)}")


if __name__ == "__main__":
    main()

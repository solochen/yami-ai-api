#!/usr/bin/env python3
"""Import selected OpenRouter chat models using the public models catalog.

Prices, modalities, and reasoning settings come from
https://openrouter.ai/api/v1/models. Display copy is a Chinese summary of
that official description only.

The OpenRouter connection is copied from an existing model so the stored key
is reused and never printed.
"""

from __future__ import annotations

import json
import subprocess
import urllib.request
from decimal import Decimal

SOURCE_CODE = "deepseek-v4-pro-0813"
CATALOG = "https://openrouter.ai/api/v1/models"

# Chinese summaries stay inside the official OpenRouter description.
COPY = {
    "openai/gpt-6-sol": ("GPT-6 Sol", "OpenAI GPT-6 系列中位于旗舰 Astra 之下、快速档 Luna 之上的高性价比型号，面向要求较高的专业任务。"),
    "openai/gpt-6-luna": ("GPT-6 Luna", "OpenAI GPT-6 系列中位于 Sol 之下的快速、低成本型号，适合高并发、对延迟敏感的对话、分类和轻量智能体任务。"),
    "openai/gpt-6-astra": ("GPT-6 Astra", "OpenAI 面向端到端重任务的旗舰型号，适合高级分析、软件工程、深度研究、科学工作和文档创作，长程任务是其强项。"),
    "openai/gpt-5.6-luna": ("GPT-5.6 Luna", "OpenAI GPT-5.6 系列中的快速、低成本型号，适合高并发、对延迟敏感的对话、分类和轻量智能体流程。"),
    "openai/gpt-5.6-sol": ("GPT-5.6 Sol", "OpenAI GPT-5.6 系列旗舰，适合复杂推理、编程和智能体流程，尤其擅长命令行和多步编程任务。"),
    "google/gemini-3.8-flash": ("Gemini 3.8 Flash", "Google 最强的 Flash 型号，相对 3.7 Flash 在软件工程、智能体任务和多步推理上有明显提升。"),
    "google/gemini-3.7-flash": ("Gemini 3.7 Flash", "Google 的多模态型号，面向快速智能体流程、编程和复杂多步推理，强调响应速度和可靠的多步任务。"),
    "anthropic/claude-opus-5.5": ("Claude Opus 5.5", "Anthropic 旗舰，面向高要求推理、编程和长程智能体工作，接替 Claude Opus 5，尤其擅长大型代码库中的多步修改。"),
    "anthropic/claude-fable-5.1": ("Claude Fable 5.1", "相对 Claude Fable 5 全面提升，增益最大的是智能体编程、长时间运行的智能体流程和知识工作，包括长代码重构、前端和视觉相关任务。"),
    "anthropic/claude-sonnet-5": ("Claude Sonnet 5", "Anthropic 最强的 Sonnet 档型号，在编程、智能体和专业工作上达到前沿水平，支持可选择强度的自适应思考。"),
    "anthropic/claude-fable-5": ("Claude Fable 5", "Anthropic 的 Mythos 级型号，面向自主知识工作和编程，支持文本、图片和文件输入、文本输出，以及推理。"),
    "anthropic/claude-opus-4.8": ("Claude Opus 4.8", "Anthropic Opus 家族中能力最强的公开型号，支持文本、图片和文件输入、文本输出、推理，以及 100 万 token 上下文。"),
    "qwen/qwen3.8-max-prime": ("Qwen3.8 Max Prime", "阿里 Qwen3.8 Max 的高吞吐变体，作为单独型号提供且价格更高，接受文本、图片和视频输入。"),
    "qwen/qwen3.8-omni-flash": ("Qwen3.8 Omni Flash", "阿里全模态推理型号，首个围绕智能体能力、原生理解音视频的 Qwen，适合音视频分析与摘要。"),
    "qwen/qwen3.8-max-0902": ("Qwen3.8 Max 0902", "Qwen3.8 Max 的 0902 快照，2.4 万亿参数混合专家模型，接受文本、图片和视频输入并输出文本。"),
    "qwen/qwen3.8-flash": ("Qwen3.8 Flash", "阿里多模态推理型号，适合编程辅助、智能体流程、视觉理解、文档与代码库分析、桌面交互、图表分析和长视频分析。"),
    "qwen/qwen3.8-27b": ("Qwen3.8 27B", "Qwen 的开源权重稠密视觉语言模型，适合编程、专业流程、研究、多模态交互和长时间智能体任务，思考方式可调整。"),
    "z-ai/glm-5.3-prime": ("GLM 5.3 Prime", "Z.ai GLM-5.3 的高速变体，继承其全部能力，并通过推理加速把输出吞吐提高到约 1.5 到 2 倍；文本输入输出，100 万 token 上下文。"),
    "z-ai/glm-5.3-flashx": ("GLM 5.3 FlashX", "Z.ai GLM-5.3-Flash 的高速变体，原生多模态，推理速度最高约 200 token/s，采用混合稀疏与线性注意力。"),
    "z-ai/glm-5.3-flash": ("GLM 5.3 Flash", "Z.ai 的原生多模态型号，适合高效编程和长程智能体任务；混合稀疏与线性注意力用于保持长上下文准确性。"),
    "z-ai/glm-5.3": ("GLM 5.3", "Z.ai 的大规模推理型号，面向复杂软件工程和长程智能体任务，文本输入输出，100 万 token 上下文。"),
}


def per_million(value: str | None) -> float | None:
    if not value:
        return None
    amount = (Decimal(value) * Decimal(1_000_000)).normalize()
    return float(amount)


def reasoning_rule(model: dict) -> dict:
    info = model.get("reasoning") or {}
    efforts = [str(item) for item in info.get("supported_efforts") or []]
    mandatory = bool(info.get("mandatory"))
    default_enabled = info.get("default_enabled")
    if default_enabled is None:
        default_enabled = mandatory
    params = set(model.get("supported_parameters") or [])
    rule: dict = {"default_enabled": bool(default_enabled)}
    if "temperature" not in params:
        rule["omit_temperature"] = True
    if efforts and "reasoning_effort" in params:
        off = "none" if "none" in efforts and not mandatory else efforts[-1]
        on = "high" if "high" in efforts else efforts[0]
        rule.update({"mode": "reasoning_effort", "off_effort": off, "on_effort": on})
        return rule
    rule.update({
        "mode": "custom",
        "can_disable": not mandatory,
        "on_params": {"reasoning": {"enabled": True}},
        "off_params": {"reasoning": {"enabled": False}},
    })
    return rule


def capabilities(model: dict) -> dict:
    inputs = set((model.get("architecture") or {}).get("input_modalities") or [])
    return {
        "web_search": False,
        "deep_think": True,
        "vision": "image" in inputs,
        "image_input": "image" in inputs,
        "video_analysis": "video" in inputs,
        "audio_analysis": "audio" in inputs,
    }


def sql_text(value: str) -> str:
    return "'" + value.replace("'", "''") + "'"


def sql_json(value: object) -> str:
    return "'" + json.dumps(value, ensure_ascii=False).replace("'", "''") + "'::jsonb"


def main() -> None:
    request = urllib.request.Request(CATALOG, headers={"Accept": "application/json", "User-Agent": "starai-model-import"})
    with urllib.request.urlopen(request, timeout=60) as response:
        catalog = {item["id"]: item for item in json.load(response).get("data", [])}
    missing = [model_id for model_id in COPY if model_id not in catalog]
    if missing:
        raise SystemExit("OpenRouter catalog missing: " + ", ".join(missing))

    statements = ["BEGIN;"]
    for index, model_id in enumerate(COPY):
        official = catalog[model_id]
        name, description = COPY[model_id]
        code = model_id.split("/", 1)[1].replace(".", "-")
        vendor = model_id.split("/", 1)[0]
        pricing = official.get("pricing") or {}
        price = {
            "billing_type": "per_token",
            "currency": "$",
            "input_price_per_m": per_million(pricing.get("prompt")),
            "output_price_per_m": per_million(pricing.get("completion")),
        }
        cache_read = per_million(pricing.get("input_cache_read"))
        cache_write = per_million(pricing.get("input_cache_write"))
        if cache_read:
            price["cache_read_price_per_m"] = cache_read
        if cache_write:
            price["cache_write_price_per_m"] = cache_write
        runtime = {"capabilities": capabilities(official), "reasoning": reasoning_rule(official)}
        tags = ["对话", vendor]
        statements.append(f"""
INSERT INTO models (
  code, display_name, new_api_model, new_api_endpoint, request_mode, category,
  description, tags, input_schema, default_params, new_api_extra_params, price_rule,
  runtime_rule, is_enabled, sort_order
)
SELECT
  {sql_text(code)}, {sql_text(name)}, {sql_text(model_id)}, '/v1/chat/completions', 'chat_completions', 'chat',
  {sql_text(description)}, {sql_json(tags)}, '{{}}'::jsonb, '{{}}'::jsonb, src.new_api_extra_params,
  {sql_json(price)}, {sql_json(runtime)}, true, {200 + index}
FROM models src
WHERE src.code = '{SOURCE_CODE}'
ON CONFLICT (code) DO UPDATE SET
  display_name = EXCLUDED.display_name,
  new_api_model = EXCLUDED.new_api_model,
  description = EXCLUDED.description,
  tags = EXCLUDED.tags,
  price_rule = EXCLUDED.price_rule,
  runtime_rule = EXCLUDED.runtime_rule,
  is_enabled = true,
  updated_at = now();
INSERT INTO model_routes (
  model_id, route_name, provider, protocol, upstream_model, endpoint, base_url, api_key,
  auth_type, api_key_header, headers, extra_params, runtime_rule, cost_rule, priority, weight,
  timeout_seconds, max_retries, is_enabled
)
SELECT m.id, 'OpenRouter', src.provider, src.protocol, {sql_text(model_id)}, src.endpoint, src.base_url, src.api_key,
  src.auth_type, src.api_key_header, src.headers, src.extra_params, '{{}}'::jsonb, '{{}}'::jsonb, src.priority, src.weight,
  src.timeout_seconds, src.max_retries, true
FROM models m
JOIN model_routes src ON src.model_id = (SELECT id FROM models WHERE code = '{SOURCE_CODE}')
WHERE m.code = {sql_text(code)}
  AND NOT EXISTS (
    SELECT 1 FROM model_routes existing WHERE existing.model_id = m.id AND existing.upstream_model = {sql_text(model_id)}
  );
""")
    statements.append("COMMIT;")
    sql = "\n".join(statements)
    subprocess.run(
        ["docker", "exec", "-i", "docker-postgres-1", "psql", "-U", "starai", "-d", "starai", "-v", "ON_ERROR_STOP=1"],
        input=sql.encode(),
        check=True,
    )
    print(f"imported {len(COPY)} chat models")


if __name__ == "__main__":
    main()

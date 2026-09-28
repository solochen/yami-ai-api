#!/usr/bin/env python3
"""Expand OpenRouter image and video models to the capabilities their APIs publish.

Image options come from GET /api/v1/images/models. Video input modes come from
GET /api/v1/videos/models plus the documented reference contract: Seedance 2
honors image, video, and audio references; MiniMax H3's model page accepts the
same three, with MiniMax's published 9/3/3 limits. Other video models get
first and last frames only when the catalog lists them.
"""

from __future__ import annotations

import json
import subprocess
import urllib.request

IMAGE_CATALOG = "https://openrouter.ai/api/v1/images/models"
VIDEO_CATALOG = "https://openrouter.ai/api/v1/videos/models"
OMNI = {
    "minimax/hailuo-3": (9, 3, 3),
    "bytedance/seedance-2.0": (9, 3, 3),
    "bytedance/seedance-2.0-fast": (9, 3, 3),
    "bytedance/seedance-2.0-mini": (9, 3, 3),
    "bytedance/seedance-2.5": (9, 3, 3),
}
FRAME_ONLY_OMNI_UI = {"minimax/hailuo-3-max"}


def fetch_json(url: str) -> dict:
    request = urllib.request.Request(url, headers={"Accept": "application/json", "User-Agent": "starai-model-import"})
    with urllib.request.urlopen(request, timeout=60) as response:
        return json.load(response)


def sql_json(value: object) -> str:
    return "'" + json.dumps(value, ensure_ascii=False).replace("'", "''") + "'::jsonb"


def sql_text(value: str) -> str:
    return "'" + value.replace("'", "''") + "'"


def enum_field(title: str, values: list, default, order: int, labels: dict | None = None, widget: str = "option_menu") -> dict:
    field = {
        "type": "string" if not values or isinstance(values[0], str) else "integer",
        "title": title,
        "enum": values,
        "default": default,
        "x-order": order,
        "x-widget": widget,
    }
    if labels:
        field["enumLabels"] = labels
    return field


def main() -> None:
    images = {item["id"]: item for item in fetch_json(IMAGE_CATALOG).get("data", [])}
    videos = {item["id"]: item for item in fetch_json(VIDEO_CATALOG).get("data", [])}
    statements = ["BEGIN;"]

    for model_id, model in images.items():
        code = model_id.split("/", 1)[1].replace(".", "-")
        params = model.get("supported_parameters") or {}
        properties = {}
        order = 1
        for key, title in (("quality", "清晰度"), ("background", "背景"), ("output_format", "输出格式")):
            spec = params.get(key) or {}
            values = [str(item) for item in spec.get("values") or []]
            if spec.get("type") == "enum" and values:
                default = "auto" if "auto" in values else values[0]
                properties[key] = enum_field(title, values, default, order)
                order += 1
        if not properties:
            continue
        statements.append(f"""
UPDATE models
SET input_schema = jsonb_set(
      CASE WHEN jsonb_typeof(input_schema->'properties') = 'object' THEN input_schema ELSE '{{"type":"object","properties":{{}}}}'::jsonb END,
      '{{properties}}',
      COALESCE(input_schema->'properties', '{{}}'::jsonb) || {sql_json(properties)}
    ),
    updated_at = now()
WHERE new_api_model = {sql_text(model_id)}
  AND runtime_rule->'upstream'->>'adapter' = 'openrouter_image';
""")
        print(f"image {code}: {', '.join(properties)}")

    for model_id, model in videos.items():
        code = model_id.split("/", 1)[1].replace(".", "-")
        frames = [str(item) for item in model.get("supported_frame_images") or []]
        durations = [int(item) for item in model.get("supported_durations") or []]
        if not durations:
            continue
        has_audio = model.get("generate_audio") is True
        if model_id in OMNI:
            image_max, video_max, audio_max = OMNI[model_id]
            profile = "minimax_h3"
            modes = ["text", "first_frame", "last_frame", "first_last", "reference"]
            hint = (
                "描述你想要的画面、运动和氛围。首尾帧模式上传 1～2 张图；参考生模式可上传参考图（1～9 张）/ 参考视频 / 参考音频任意组合；什么都不传就是文生视频。参考图计费说明：前 5 张免费，从第 6 张起每张加收 ⚡{{reference_image_surcharge}}/张（仅超出免费张数的部分收费，与本次生成费用一并结算）"
                if model_id == "minimax/hailuo-3"
                else "可文字生成，也可上传首帧、尾帧，或同时使用参考图、参考视频和参考音频。参考音频不能单独使用，所有参考合计不超过 12 个。"
            )
        elif model_id in FRAME_ONLY_OMNI_UI or ({"first_frame", "last_frame"} <= set(frames) and image_max_fallback(model_id) == 0):
            image_max, video_max, audio_max = 0, 0, 0
            profile = "minimax_h3"
            modes = ["text", "first_frame", "last_frame", "first_last"]
            hint = "可直接文字生成，也可上传首帧、尾帧或同时上传首尾帧。"
        elif {"first_frame", "last_frame"} <= set(frames):
            image_max, video_max, audio_max = 0, 0, 0
            profile = "frame_pair"
            modes = []
            hint = "可直接文字生成。需要控制起止画面时，上传首帧和尾帧。"
        elif "first_frame" in frames:
            image_max, video_max, audio_max = 1, 0, 0
            profile = "single_ref"
            modes = []
            hint = "可直接文字生成。上传的一张参考图会作为起始画面。"
        else:
            image_max, video_max, audio_max = 0, 0, 0
            profile = "single_ref"
            modes = []
            hint = "可直接文字生成视频。"
        video_rule = {
            "upload_profile": profile,
            "prompt_required": True,
            "prompt_hint": hint,
            "show_channel": False,
            "max_reference_images": image_max,
            "min_reference_images": 0,
            "max_total_images": 12 if profile == "minimax_h3" and image_max else 2,
            "count_toward_total": True,
            "mode_param": "generation_mode",
            "supported_frame_images": frames,
            "frames": {"first": {"key": "first_frame", "max": 1}, "last": {"key": "last_frame", "max": 1 if "last_frame" in frames else 0}},
            "reference_images": {"key": "reference_images", "max": image_max},
            "reference_videos": {"key": "reference_videos", "max": video_max},
            "reference_audios": {"key": "reference_audios", "max": audio_max},
        }
        if model_id == "minimax/hailuo-3":
            video_rule["reference_image_surcharge_usd"] = 0.04
        properties = {}
        if modes:
            properties["generation_mode"] = {
                "type": "string",
                "title": "生成模式",
                "enum": modes,
                "default": "text",
                "enumLabels": {
                    "text": "文生视频",
                    "first_frame": "首帧",
                    "last_frame": "尾帧",
                    "first_last": "首尾帧",
                    "reference": "全能参考",
                },
                "x-order": 0,
                "x-widget": "option_menu",
                "x-highlight": True,
            }
        if has_audio:
            properties["generate_audio"] = {
                "type": "boolean",
                "title": "生成声音",
                "default": True,
                "x-order": 4,
                "x-widget": "boolean_toggle",
            }
        statements.append(f"""
UPDATE models
SET runtime_rule = jsonb_set(
      jsonb_set(runtime_rule, '{{video}}', {sql_json(video_rule)}, true),
      '{{upstream,adapter}}',
      '"openrouter_video"'::jsonb,
      true
    ),
    input_schema = jsonb_set(
      CASE WHEN jsonb_typeof(input_schema->'properties') = 'object' THEN input_schema ELSE '{{"type":"object","properties":{{}}}}'::jsonb END,
      '{{properties}}',
      COALESCE(input_schema->'properties', '{{}}'::jsonb) || {sql_json(properties)}
    ),
    default_params = default_params || {sql_json({"generation_mode": "text"} if modes else {})} || {sql_json({"generate_audio": True} if has_audio else {})},
    updated_at = now()
WHERE new_api_model = {sql_text(model_id)}
  AND runtime_rule->'upstream'->>'adapter' = 'openrouter_video';
""")
        print(f"video {code}: {profile} images={image_max} videos={video_max} audios={audio_max} audio_out={has_audio}")

    statements.append("COMMIT;")
    subprocess.run(
        ["docker", "exec", "-i", "docker-postgres-1", "psql", "-U", "starai", "-d", "starai", "-v", "ON_ERROR_STOP=1"],
        input="\n".join(statements).encode(),
        check=True,
    )


def image_max_fallback(model_id: str) -> int:
    return 0


if __name__ == "__main__":
    main()

export type SectionId = "start" | "chat" | "image" | "video" | "audio" | "models" | "task" | "events";
export type ChatProtocol = "openai" | "anthropic" | "gemini";
export type MediaSection = "chat" | "image" | "video" | "audio";

export interface ApiDocRecord {
  slug: string;
  title?: string;
  summary?: string;
  model_code: string;
  model_name?: string;
  model_description?: string;
  category?: string;
  request_mode?: string;
  public_code?: string;
  content?: {
    request_example?: unknown;
    features?: unknown;
  };
}

export interface PublicModel {
  code: string;
  display_name?: string;
  category?: string;
  description?: string | null;
}

export interface DocModel {
  key: string;
  name: string;
  modelId: string;
  description: string;
  section: MediaSection;
  example: Record<string, unknown>;
  tags: string[];
}

export interface FieldHelp {
  name: string;
  type: string;
  required: string;
  help: string;
}

const IMAGE_PROMPT = "一只白瓷杯放在浅色桌面上，柔和日光，真实摄影。";
const VIDEO_PROMPT = "镜头缓慢推近一只白瓷杯，浅色桌面，柔和日光。";
const SPEECH = "你好，今天想为你读一段简短的欢迎词。";
const LYRICS = "[Verse]\n夜风轻轻掠过窗前\n[Chorus]\n让星光照亮明天";
const CHAT_TEXT = "用三句话介绍你能做什么。";

const GENERIC_TAGS = new Set(["统一 API Key", "平台模型编码", "标准 JSON 响应"]);

const FIELD_HELP: Record<string, Omit<FieldHelp, "name">> = {
  model: { type: "string", required: "必填", help: "模型 ID。使用本页列出的 ID，不使用展示名称。" },
  messages: { type: "array", required: "必填", help: "消息数组。元素包含 role（system、user、assistant）和 content。" },
  stream: { type: "boolean", required: "可选", help: "默认 false，返回一次性 JSON，正文位于 content。true 时响应为 text/event-stream。" },
  max_tokens: { type: "integer", required: "可选", help: "最大输出 token 数。Anthropic Messages 中为必填。" },
  temperature: { type: "number", required: "可选", help: "采样温度。省略时使用模型默认值。" },
  tools: { type: "array", required: "可选", help: "工具定义。仅在函数调用时传入。" },
  contents: { type: "array", required: "必填", help: "Gemini contents。元素包含 role 与 parts，文本位于 parts[].text。" },
  prompt: { type: "string", required: "条件必填", help: "生成提示词。仅首帧、无文本模板的视频模型可以省略。" },
  input: { type: "string", required: "条件必填", help: "TTS 文本。音乐模型中作为歌词；纯风格描述使用 music_prompt。" },
  n: { type: "integer", required: "可选", help: "生成数量，默认 1。" },
  count: { type: "integer", required: "可选", help: "生成数量，默认 1。与 n 等价。" },
  size: { type: "string", required: "可选", help: "像素尺寸，例如 1024x1024。取值以所选模型为准。" },
  aspect_ratio: { type: "string", required: "可选", help: "画面比例，例如 1:1、16:9、9:16。" },
  image_size: { type: "string", required: "可选", help: "清晰度档位，例如 1K、2K。与 size 同时出现时保持示例中的组合。" },
  quality: { type: "string", required: "可选", help: "生成质量。取值以所选模型为准。" },
  duration: { type: "integer", required: "可选", help: "视频时长，单位秒。范围以所选模型为准。" },
  resolution: { type: "string", required: "可选", help: "分辨率，例如 720p、1080p。枚举以所选模型为准。" },
  generation_mode: { type: "string", required: "可选", help: "生成模式。text 为文生；首帧或参考图模式需同时提供对应媒体 URL。" },
  image: { type: "string | string[]", required: "可选", help: "参考图的公网 URL。本地文件使用 POST /v1/images/edits。" },
  first_frame: { type: "string", required: "可选", help: "首帧图片的公网 URL。" },
  last_frame: { type: "string", required: "可选", help: "尾帧图片的公网 URL。" },
  reference_images: { type: "string[]", required: "可选", help: "参考图公网 URL 列表。" },
  reference_videos: { type: "string[]", required: "可选", help: "参考视频公网 URL 列表。" },
  reference_audios: { type: "string[]", required: "可选", help: "参考音频公网 URL 列表。" },
  music_prompt: { type: "string", required: "可选", help: "曲风、情绪与配器描述。" },
  lyrics: { type: "string", required: "可选", help: "歌词。是否可替代 input 取决于所选模型。" },
  voice: { type: "string", required: "可选", help: "音色 ID。省略时使用模型默认音色。" },
  format: { type: "string", required: "可选", help: "音频封装格式，例如 mp3。" },
  sample_rate: { type: "integer", required: "可选", help: "采样率。取值以所选模型为准。" },
  output_format: { type: "string", required: "可选", help: "上游返回载体。平台仍在任务结果中返回可访问的音频 URL，请保留示例值。" },
  model_version: { type: "string", required: "可选", help: "模型要求附带的版本参数，不是展示名称。请保留示例值。" },
  is_instrumental: { type: "boolean", required: "可选", help: "true 表示生成纯音乐。" },
  generate_audio: { type: "boolean", required: "可选", help: "是否为视频同时生成音轨。" },
};

export const SECTIONS: Array<{ id: SectionId; label: string; hint: string }> = [
  { id: "start", label: "概述", hint: "Base URL、鉴权与模型标识" },
  { id: "chat", label: "文本生成", hint: "Chat Completions 与原生协议" },
  { id: "image", label: "图像生成", hint: "POST /v1/images/generations" },
  { id: "video", label: "视频生成", hint: "POST /v1/video/generations" },
  { id: "audio", label: "音频生成", hint: "POST /v1/audio/speech" },
  { id: "models", label: "模型列表", hint: "GET /v1/models" },
  { id: "task", label: "查询任务", hint: "GET /v1/tasks/{task_no}" },
  { id: "events", label: "任务事件", hint: "GET /v1/tasks/{task_no}/events" },
];

export function sectionVisible(id: SectionId, operations?: Record<string, boolean> | null) {
  if (!operations) return true;
  if (id === "start") return true;
  if (id === "chat") return operations.chat !== false;
  if (id === "image") return operations.image !== false;
  if (id === "video") return operations.video !== false;
  if (id === "audio") return operations.audio !== false;
  return operations.platform !== false;
}

export function collectModels(docs: ApiDocRecord[], publics: PublicModel[]): DocModel[] {
  const models: DocModel[] = [];
  const seen = new Set<string>();
  const publicCodeByName = uniquePublicCodeByName(publics);
  for (const doc of docs) {
    if (isCollaboration(doc.category, doc.model_code, doc.public_code)) continue;
    const section = sectionFromMode(doc.request_mode);
    if (!section) continue;
    const modelId = callableModelId(doc, publicCodeByName);
    if (!modelId || seen.has(modelId) || seen.has(doc.model_code)) continue;
    seen.add(modelId);
    seen.add(doc.model_code);
    models.push({
      key: doc.slug || modelId,
      name: firstText(doc.model_name, doc.title, modelId),
      modelId,
      description: firstText(doc.model_description, doc.summary),
      section,
      example: exampleFor(doc, section, modelId),
      tags: cleanTags(doc.content?.features),
    });
  }
  for (const model of publics) {
    if (isCollaboration(model.category, model.code)) continue;
    const section = sectionFromCategory(model.category);
    if (!section || !model.code || seen.has(model.code)) continue;
    seen.add(model.code);
    models.push({
      key: "public:" + model.code,
      name: firstText(model.display_name, model.code),
      modelId: model.code,
      description: model.description || "",
      section,
      example: defaultExample(section, model.code),
      tags: [],
    });
  }
  return models.sort((a, b) => a.name.localeCompare(b.name, "zh-CN"));
}

export function modelsFor(models: DocModel[], section: SectionId) {
  const target: MediaSection | "" = section === "start" ? "chat" : section === "chat" || section === "image" || section === "video" || section === "audio" ? section : "";
  if (!target) return [];
  return models.filter((model) => model.section === target);
}

export function endpointFor(section: SectionId, protocol: ChatProtocol, modelId: string, taskNo: string) {
  const task = encodeURIComponent(taskNo.trim() || "{task_no}");
  if (section === "image") return { method: "POST" as const, path: "/v1/images/generations" };
  if (section === "video") return { method: "POST" as const, path: "/v1/video/generations" };
  if (section === "audio") return { method: "POST" as const, path: "/v1/audio/speech" };
  if (section === "models") return { method: "GET" as const, path: "/v1/models" };
  if (section === "task") return { method: "GET" as const, path: "/v1/tasks/" + task };
  if (section === "events") return { method: "GET" as const, path: "/v1/tasks/" + task + "/events" };
  if (protocol === "anthropic") return { method: "POST" as const, path: "/v1/messages" };
  if (protocol === "gemini") return { method: "POST" as const, path: "/v1beta/models/" + encodeURIComponent(modelId || "{model}") + ":generateContent" };
  return { method: "POST" as const, path: "/v1/chat/completions" };
}

export function requestBody(section: SectionId, protocol: ChatProtocol, model: DocModel | undefined) {
  if (!model || section === "models" || section === "task" || section === "events") return undefined;
  if (section === "chat" || section === "start") {
    if (protocol === "anthropic") {
      return { model: model.modelId, max_tokens: 1024, messages: [{ role: "user", content: CHAT_TEXT }] };
    }
    if (protocol === "gemini") {
      return { contents: [{ role: "user", parts: [{ text: CHAT_TEXT }] }] };
    }
    return { model: model.modelId, messages: [{ role: "user", content: CHAT_TEXT }], stream: false };
  }
  return model.example;
}

export function requestFields(section: SectionId, protocol: ChatProtocol, body: Record<string, unknown> | undefined): FieldHelp[] {
  if (section === "models" || section === "task" || section === "events") return [];
  if ((section === "chat" || section === "start") && protocol === "gemini") {
    return [
      helpFor("contents"),
      { name: "model", type: "string", required: "必填", help: "路径参数。取值为模型 ID，位于 /v1beta/models/{model}:generateContent。" },
    ];
  }
  if ((section === "chat" || section === "start") && protocol === "anthropic") {
    return ["model", "max_tokens", "messages"].map((name) => name === "max_tokens"
      ? { ...helpFor(name), required: "必填" }
      : helpFor(name));
  }
  const names = body ? Object.keys(body) : [];
  const ordered = ["model", "messages", "prompt", "input", "stream", ...names.filter((name) => !["model", "messages", "prompt", "input", "stream"].includes(name))];
  return ordered.filter((name, index) => ordered.indexOf(name) === index && (body ? name in body : false)).map(helpFor);
}

export function responseExample(section: SectionId, protocol: ChatProtocol, modelId: string) {
  if (section === "models") {
    return { object: "list", data: [{ id: modelId || "模型 ID", object: "model", owned_by: "orbit-e" }] };
  }
  if (section === "task") {
    return {
      task_no: "task_xxx",
      type: "image",
      status: "succeeded",
      poll_url: "/v1/tasks/task_xxx",
      output: { image_url: "https://example.com/result.png", images: [{ url: "https://example.com/result.png" }] },
    };
  }
  if (section === "events") {
    return {
      items: [
        { event_type: "created", payload: {}, created_at: "2026-01-01T00:00:00Z" },
        { event_type: "progress", payload: { status: "processing", progress: 50 }, created_at: "2026-01-01T00:00:05Z" },
        { event_type: "completed", payload: {}, created_at: "2026-01-01T00:00:20Z" },
      ],
    };
  }
  if (section === "image" || section === "video" || section === "audio") {
    const type = section === "image" ? "image" : section;
    return {
      task_no: "task_xxx",
      type,
      status: "pending",
      model_code: modelId,
      estimated_cost: 1,
      created_at: "2026-01-01T00:00:00Z",
      poll_url: "/v1/tasks/task_xxx",
    };
  }
  if (protocol === "anthropic") {
    return {
      id: "msg_xxx",
      type: "message",
      role: "assistant",
      model: modelId,
      content: [{ type: "text", text: "这里是完整回答。" }],
      stop_reason: "end_turn",
      usage: { input_tokens: 12, output_tokens: 8 },
    };
  }
  if (protocol === "gemini") {
    return {
      candidates: [{ content: { role: "model", parts: [{ text: "这里是完整回答。" }] }, finishReason: "STOP", index: 0 }],
      modelVersion: modelId,
    };
  }
  return {
    request_id: "req_xxx",
    conversation_id: "conv_xxx",
    content: "这里是完整回答。",
    usage: { prompt_tokens: 12, completion_tokens: 8, total_tokens: 20 },
    cost: 0.01,
  };
}

export function succeededExample(section: SectionId) {
  if (section === "image") return { task_no: "task_xxx", type: "image", status: "succeeded", output: { image_url: "https://example.com/result.png" } };
  if (section === "video") return { task_no: "task_xxx", type: "video", status: "succeeded", output: { video_url: "https://example.com/result.mp4" } };
  if (section === "audio") return { task_no: "task_xxx", type: "audio", status: "succeeded", output: { audio_url: "https://example.com/result.mp3" } };
  return null;
}

export function authHeaders(protocol: ChatProtocol) {
  if (protocol === "anthropic") return [["x-api-key", "<API_KEY>"], ["anthropic-version", "2023-06-01"]];
  if (protocol === "gemini") return [["x-goog-api-key", "<API_KEY>"]];
  return [["Authorization", "Bearer <API_KEY>"]];
}

export function curlExample(base: string, method: "GET" | "POST", path: string, protocol: ChatProtocol, body?: unknown) {
  const root = base.replace(/\/+$/, "");
  const headers = authHeaders(protocol).map(([name, value]) => {
    if (name === "Authorization") return "Authorization: Bearer <API_KEY>";
    if (name === "x-api-key" || name === "x-goog-api-key") return name + ": <API_KEY>";
    return name + ": " + value;
  });
  const lines = ["curl " + root + path + " \\"];
  headers.forEach((header) => lines.push('  -H "' + header + '" \\'));
  if (method === "POST") {
    lines.push('  -H "Content-Type: application/json" \\');
    lines.push("  -d '" + JSON.stringify(body ?? {}) + "'");
  } else {
    lines[lines.length - 1] = lines[lines.length - 1].replace(/ \\$/, "");
  }
  return lines.join("\n");
}

export function streamSample(protocol: ChatProtocol) {
  if (protocol === "anthropic") {
    return "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"这里\"}}";
  }
  if (protocol === "gemini") {
    return "data: {\"candidates\":[{\"content\":{\"role\":\"model\",\"parts\":[{\"text\":\"这里\"}]}}]}";
  }
  return "data: {\"id\":\"chatcmpl_xxx\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"这里\"},\"finish_reason\":null}]}\n\ndata: [DONE]";
}

export function outcomeCopy(section: SectionId, protocol: ChatProtocol) {
  if (section === "start") return "接口共用同一 Base URL 与 API Key。model 传模型 ID。文本接口的非流式响应读取 content；图像、视频和音频接口创建异步任务。";
  if (section === "chat" && protocol === "anthropic") return "Anthropic Messages。文本位于 content[] 中 type 为 text 的 text。stream 为 true 时返回 SSE。";
  if (section === "chat" && protocol === "gemini") return "Gemini generateContent。模型 ID 为路径参数。文本位于 candidates[0].content.parts[].text。";
  if (section === "chat") return "OpenAI 兼容的 Chat Completions。stream 为 false 时正文在 content，响应不含 choices。stream 为 true 时返回 SSE。";
  if (section === "image") return "创建图像任务。HTTP 200 只表示任务已受理。完成后从 GET /v1/tasks/{task_no} 的 output.image_url 读取结果。";
  if (section === "video") return "创建视频任务。HTTP 200 只表示任务已受理。完成后从 output.video_url 读取结果。";
  if (section === "audio") return "创建语音或音乐任务。完成后从 output.audio_url 读取结果。";
  if (section === "models") return "返回当前 API Key 可调用的模型。data[].id 即请求中的 model，以及 Gemini 路径中的 {model}。";
  if (section === "task") return "按 task_no 查询任务。status 为 succeeded 时读取 output；failed 时读取 error_message。响应中的 model_code 用于对账，后续请求仍传模型 ID。";
  return "按时间返回任务事件。进度位于 event_type 为 progress 的 payload.progress。最终文件以查询任务接口的 output 为准。";
}

export const ERRORS = [
  { status: "401", code: "authentication_error", help: "缺少 API Key，或 Key 无效、已删除。" },
  { status: "400", code: "invalid_request_error", help: "参数无效。model 不存在或未启用时，code 为 model_not_found。" },
  { status: "402", code: "insufficient_balance", help: "账户余额不足。API Key 仍然有效。" },
  { status: "429", code: "429", help: "超过账号频率限制。响应体为 { code, message }，不是 error 对象。" },
  { status: "502", code: "upstream_error", help: "上游模型调用失败。" },
];

export function pretty(value: unknown) {
  return JSON.stringify(value ?? {}, null, 2);
}

function helpFor(name: string): FieldHelp {
  return { name, ...(FIELD_HELP[name] || { type: "any", required: "可选", help: "所选模型的专用参数。未确认取值范围时保留示例中的值。" }) };
}

function callableModelId(doc: ApiDocRecord, publicCodeByName: Map<string, string>) {
  const published = (doc.public_code || "").trim();
  if (published) return published;
  const named = publicCodeByName.get((doc.model_name || "").trim());
  return named || (doc.model_code || "").trim();
}

function uniquePublicCodeByName(models: PublicModel[]) {
  const counts = new Map<string, number>();
  const codes = new Map<string, string>();
  for (const model of models) {
    const name = (model.display_name || "").trim();
    const code = (model.code || "").trim();
    if (!name || !code || isCollaboration(model.category, code)) continue;
    counts.set(name, (counts.get(name) || 0) + 1);
    codes.set(name, code);
  }
  for (const [name, count] of counts) {
    if (count !== 1) codes.delete(name);
  }
  return codes;
}

function isCollaboration(category?: string, ...codes: Array<string | undefined>) {
  if (category === "multi_collab") return true;
  return codes.some((code) => (code || "").trim() === "multi_collab_chat");
}

function sectionFromMode(mode?: string): MediaSection | "" {
  if (mode === "chat_completions" || mode === "responses") return "chat";
  if (mode === "images") return "image";
  if (mode === "video") return "video";
  if (mode === "audio") return "audio";
  return "";
}

function sectionFromCategory(category?: string): MediaSection | "" {
  if (category === "chat") return "chat";
  if (category === "image") return "image";
  if (category === "video") return "video";
  if (category === "audio") return "audio";
  return "";
}

function exampleFor(doc: ApiDocRecord, section: MediaSection, modelId: string) {
  if (section === "chat") return defaultExample(section, modelId);
  const raw = doc.content?.request_example;
  const example = raw && typeof raw === "object" && !Array.isArray(raw) ? { ...(raw as Record<string, unknown>) } : defaultExample(section, modelId);
  example.model = modelId;
  if (typeof example.prompt === "string") example.prompt = section === "video" ? VIDEO_PROMPT : IMAGE_PROMPT;
  if (typeof example.input === "string") {
    const musical = "music_prompt" in example || "lyrics" in example || "is_instrumental" in example || /\[(verse|chorus)\]/i.test(String(example.input));
    example.input = musical ? LYRICS : SPEECH;
  }
  delete example.stream;
  return example;
}

function defaultExample(section: MediaSection, modelId: string): Record<string, unknown> {
  if (section === "image") return { model: modelId, prompt: IMAGE_PROMPT, n: 1, size: "1024x1024" };
  if (section === "video") return { model: modelId, prompt: VIDEO_PROMPT, duration: 5 };
  if (section === "audio") return { model: modelId, input: SPEECH };
  return { model: modelId, messages: [{ role: "user", content: CHAT_TEXT }], stream: false };
}

function cleanTags(value: unknown) {
  const list = Array.isArray(value) ? value : [];
  return list.map((item) => String(item)).filter((item) => item && !GENERIC_TAGS.has(item)).slice(0, 4);
}

function firstText(...values: Array<string | null | undefined>) {
  for (const value of values) {
    const text = (value || "").trim();
    if (text) return text;
  }
  return "";
}

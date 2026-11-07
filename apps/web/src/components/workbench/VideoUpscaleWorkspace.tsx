"use client";

import { pollAsync } from "@/lib/pollAsync";

import { useEffect, useMemo, useRef, useState } from "react";
import { ArrowUp, CheckCircle2, Download, Film, FolderOpen, History, Image as ImageIcon, Loader2, RefreshCw, Sparkles, Upload, X } from "lucide-react";
import { api, listAssets, uploadAsset } from "@/lib/api";
import { useI18n } from "@/i18n/I18nProvider";
import { AGENT_THEMES } from "./categoryMeta";
import { AgentLanding } from "./AgentLanding";
import { MediaMenuOption, MediaOptionMenu } from "./MediaOptionMenu";

type WorkflowLike = {
  code: string;
  name: string;
  description?: string;
  icon?: string;
  display_config?: {
    help?: string;
    theme?: string;
    hero_tags?: string[];
    feature_tags?: string[];
    steps?: Array<{ icon?: string; title: string; subtitle?: string; tags?: string[] }>;
  };
  runtime_config?: {
    agent_mode?: string;
    preset_code?: string;
    supported_resolutions?: string[];
    default_target_resolution?: string;
    preserve_audio?: boolean;
    default_enhancement_mode?: string;
    max_input_duration_sec?: number;
    max_input_size_mb?: number;
    default_style_strength?: number;
    preserve_motion?: boolean;
    preserve_identity?: boolean;
    default_subtitle_mode?: string;
    default_subtitle_region?: string;
    protect_watermark?: boolean;
  };
};

type SourceVideo = { url: string; name: string; public_id?: string; size_bytes?: number; duration?: number };
type StyleReference = { url: string; name: string; public_id?: string };
type MediaTask = { task_no: string; status: string; progress?: number; output?: Record<string, any>; error_message?: string };
type Project = {
  public_id: string;
  status: string;
  inputs?: Record<string, any>;
  estimated_cost?: number;
  actual_cost?: number;
  error_message?: string;
  outputs?: Record<string, any>;
  media_tasks?: MediaTask[];
};
type HistoryItem = { public_id: string; workflow_name?: string; status: string; created_at: string };
type AssetItem = { public_id: string; url: string; name?: string; size_bytes?: number; metadata?: Record<string, any> };
type UtilityFeature = { icon?: string; title: string; subtitle?: string; tags?: string[] };

function outputURL(task?: MediaTask) {
  const out = task?.output || {};
  return String(out.video_url || out.url || (Array.isArray(out.videos) && out.videos[0]?.url) || "");
}

function subtitleResultLabel(mode: string | undefined, ts: (source: string) => string) {
  if (mode === "soft_track") return ts("已去掉单独的字幕轨");
  if (mode === "soft_and_hard") return ts("已移除字幕轨并修复画面硬字幕");
  return ts("AI 硬字幕修复");
}

function SubtitleLimitNotice({ ts }: { ts: (source: string) => string }) {
  return (
    <div className="rounded-2xl bg-amber-50 px-3 py-3 text-amber-950 dark:bg-amber-400/10 dark:text-amber-50">
      <div className="text-sm font-semibold">{ts("只能去掉单独的字幕轨")}</div>
      <p className="mt-1 text-xs leading-5 text-amber-900/80 dark:text-amber-50/80">{ts("这个功能只能去掉可以单独开关的字幕，画面本身不会改。字如果是直接做进画面里的，比如短视频底部那一行，这里去不掉。")}</p>
    </div>
  );
}

function assetDurationSeconds(asset: AssetItem) {
  const meta = asset.metadata || {};
  const raw = meta.duration_seconds ?? meta.duration_sec ?? meta.duration;
  const value = Number(raw || 0);
  return Number.isFinite(value) && value > 0 ? value : 0;
}

function formatBytes(value = 0) {
  if (!value) return "";
  if (value < 1024 * 1024) return `${Math.max(1, Math.round(value / 1024))} KB`;
  return `${(value / 1024 / 1024).toFixed(value > 100 * 1024 * 1024 ? 0 : 1)} MB`;
}

function readVideoDuration(file: File) {
  return new Promise<number>((resolve) => {
    const url = URL.createObjectURL(file);
    const video = document.createElement("video");
    video.preload = "metadata";
    video.onloadedmetadata = () => {
      const duration = Number.isFinite(video.duration) ? video.duration : 0;
      URL.revokeObjectURL(url);
      resolve(duration);
    };
    video.onerror = () => {
      URL.revokeObjectURL(url);
      resolve(0);
    };
    video.src = url;
  });
}

function LegacyUtilityLanding({
  icon,
  name,
  description,
  label,
  heroTags,
  features,
  activeIndex,
  onSelect,
}: {
  icon: string;
  name: string;
  description: string;
  label: string;
  heroTags: string[];
  features: UtilityFeature[];
  activeIndex: number;
  onSelect: (index: number) => void;
}) {
  const { ts } = useI18n();
  const active = features[Math.min(activeIndex, features.length - 1)] || features[0];
  const activeTags = active?.tags?.length ? active.tags : heroTags.slice(0, 4);
  return (
    <div className="flex min-h-0 flex-1 flex-col justify-center overflow-y-auto overscroll-contain py-2">
      <div className="shrink-0 text-center">
        <div className="mb-1.5 inline-flex items-center gap-2 rounded-full border border-white/60 bg-primary/10 px-3 py-1 text-[11px] font-semibold text-primary backdrop-blur dark:border-white/10 dark:bg-primary/10 dark:text-primary sm:px-4 sm:text-xs">
          <span className="h-1.5 w-1.5 rounded-full bg-primary" />
          {label}
        </div>
        <div className="flex items-center justify-center gap-3">
          <div className="flex h-9 w-9 items-center justify-center rounded-2xl bg-primary/10 text-xl shadow-sm sm:h-11 sm:w-11 sm:text-2xl">{icon}</div>
          <h1 title={name} className="max-w-[min(78vw,900px)] truncate text-xl font-black text-gray-900 dark:text-white sm:text-3xl">{name}</h1>
        </div>
        {description && <p className="mx-auto mt-2 max-w-2xl px-3 text-xs leading-5 text-gray-500 dark:text-gray-300 sm:text-sm">{description}</p>}
        <div className="mt-2 flex flex-wrap justify-center gap-1.5 sm:mt-3 sm:gap-2">
          {heroTags.map((tag, index) => (
            <span key={`${tag}-${index}`} title={tag} className="max-w-[180px] truncate rounded-full border border-gray-200 bg-white/55 px-2.5 py-0.5 text-[11px] text-gray-500 backdrop-blur dark:border-white/10 dark:bg-white/5 dark:text-gray-300 sm:px-3 sm:py-1 sm:text-xs">
              {tag}
            </span>
          ))}
        </div>
      </div>

      <div className="agent-showcase mx-auto mt-4 w-full max-w-[980px] shrink-0 items-center sm:mt-5">
        <div className="agent-feature-list mx-auto w-full max-w-[300px] gap-2.5">
          {features.slice(0, 4).map((item, index) => {
            const selected = activeIndex === index;
            return (
              <button
                key={`${item.title}-${index}`}
                type="button"
                onClick={() => onSelect(index)}
                className={`group w-full min-w-0 overflow-hidden rounded-2xl border p-3.5 text-left backdrop-blur transition duration-300 hover:-translate-y-0.5 hover:shadow-lg ${
                  selected
                    ? "border-primary/30 bg-white/80 shadow-md shadow-primary/20 dark:border-primary/30 dark:bg-white/10"
                    : "border-gray-200 bg-white/55 hover:border-primary/30 hover:bg-white/70 dark:border-white/10 dark:bg-transparent dark:hover:bg-primary/5"
                }`}
              >
                <div className="flex items-center gap-3">
                  <div className={`flex h-9 w-9 shrink-0 items-center justify-center rounded-xl text-lg ${selected ? "bg-primary/10" : "bg-gray-500/10 text-gray-400 dark:bg-transparent"}`}>{item.icon || "•"}</div>
                  <div className="min-w-0">
                    <div title={item.title} className="truncate text-sm font-bold text-gray-900 dark:text-white">{item.title}</div>
                    {item.subtitle && <div title={item.subtitle} className="mt-0.5 truncate text-xs text-gray-400">{item.subtitle}</div>}
                  </div>
                  {selected && <span className="ml-auto text-primary">›</span>}
                </div>
              </button>
            );
          })}
        </div>

        <div className="agent-feature-card group mx-auto flex min-h-[270px] w-full max-w-[640px] flex-col justify-center overflow-y-auto rounded-3xl border border-primary/30 bg-white/65 p-5 shadow-xl shadow-primary/20 backdrop-blur-xl transition dark:border-primary/30 dark:bg-transparent dark:shadow-black/30 sm:min-h-[300px] sm:p-6">
          <div className="mb-4 flex items-center justify-between gap-3">
            <span className="rounded-xl bg-primary/10 px-3 py-2 text-sm font-black text-primary dark:text-primary">{String(Math.min(activeIndex + 1, features.length)).padStart(2, "0")}</span>
            <span className="rounded-full border border-primary/30 bg-primary/10 px-3 py-1 text-xs font-semibold text-primary dark:border-primary/25 dark:bg-primary/10 dark:text-primary">{ts("支持视频处理链路")}</span>
          </div>
          <div className="flex items-start gap-4">
            <div className="flex h-14 w-14 shrink-0 items-center justify-center rounded-2xl bg-primary/10 text-2xl">{active?.icon || icon}</div>
            <div className="min-w-0">
              <h2 className="line-clamp-2 text-xl font-black text-gray-900 dark:text-white sm:text-2xl">{active?.title}</h2>
              <p className="mt-3 line-clamp-3 text-xs leading-6 text-gray-500 dark:text-gray-300 sm:text-sm">{active?.subtitle}</p>
              <div className="mt-4 flex flex-wrap gap-2">
                {activeTags.map((tag, index) => <span key={`${tag}-${index}`} className="max-w-[170px] truncate rounded-lg bg-primary/10 px-2.5 py-1 text-xs font-semibold text-primary dark:text-primary">{tag}</span>)}
              </div>
            </div>
          </div>
          <div className="mt-5 flex justify-center gap-2">
            {features.slice(0, 4).map((feature, index) => (
              <button key={`${feature.title}-dot-${index}`} type="button" onClick={() => onSelect(index)} aria-label={feature.title} className={`${index === activeIndex ? "h-3 w-9 bg-primary/100 shadow-md shadow-primary/20" : "h-3 w-3 bg-gray-300/70 hover:bg-primary dark:bg-white/20"} rounded-full transition-all`} />
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
void LegacyUtilityLanding;

export function VideoUpscaleWorkspace({ workflow }: { workflow: WorkflowLike }) {
  const { t, td, ts } = useI18n();
  const config = workflow.runtime_config || {};
  const utilityMode = config.agent_mode || config.preset_code || "video_upscale";
  const isUpscale = utilityMode === "video_upscale";
  const isRedraw = utilityMode === "video_redraw";
  const isSubtitle = utilityMode === "subtitle_remove";
  const resolutions = useMemo(() => {
    const allowed = new Set(["720P", "1K", "2K"]);
    const values = (config.supported_resolutions || []).filter((item) => allowed.has(item));
    return values.length ? values : ["720P", "1K", "2K"];
  }, [config.supported_resolutions]);
  const [source, setSource] = useState<SourceVideo | null>(null);
  const [resolution, setResolution] = useState(config.default_target_resolution || resolutions[0]);
  const [preserveAudio, setPreserveAudio] = useState(config.preserve_audio !== false);
  const [mode, setMode] = useState(config.default_enhancement_mode || "balanced");
  const [prompt, setPrompt] = useState("");
  const [styleReference, setStyleReference] = useState<StyleReference | null>(null);
  const [styleStrength, setStyleStrength] = useState(Number(config.default_style_strength ?? 0.65));
  const [preserveMotion, setPreserveMotion] = useState(config.preserve_motion !== false);
  const [preserveIdentity, setPreserveIdentity] = useState(config.preserve_identity !== false);
  const [uploading, setUploading] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [project, setProject] = useState<Project | null>(null);
  const [error, setError] = useState("");
  const [assetOpen, setAssetOpen] = useState(false);
  const [assets, setAssets] = useState<AssetItem[]>([]);
  const [styleAssetOpen, setStyleAssetOpen] = useState(false);
  const [styleAssets, setStyleAssets] = useState<AssetItem[]>([]);
  const [historyOpen, setHistoryOpen] = useState(false);
  const [history, setHistory] = useState<HistoryItem[]>([]);
  const [activeFeature, setActiveFeature] = useState(0);
  const pollRef = useRef<(() => void) | null>(null);
  const pollScopeRef = useRef<string | null>(null);
  const activeProjectKey = `video_utility_active_project:${workflow.code}`;
  const display = workflow.display_config || {};
  const fallbackFeatures: UtilityFeature[] = isRedraw
    ? [
        { icon: "🎬", title: ts("上传源视频"), subtitle: ts("上传文件或从资产库引用已有视频"), tags: [ts("格式校验"), ts("时长检查")] },
        { icon: "🎨", title: ts("选择目标画风"), subtitle: ts("使用文字或参考图定义转绘风格"), tags: [ts("风格参考"), ts("强度控制")] },
        { icon: "🪄", title: ts("AI 视频转绘"), subtitle: ts("保持人物、动作和镜头连续性完成风格迁移"), tags: [ts("人物一致"), ts("动作保持")] },
        { icon: "📥", title: ts("预览与下载"), subtitle: ts("查看处理进度并下载完整成片"), tags: [ts("结果预览"), ts("实际计费")] },
      ]
    : isSubtitle
      ? [
          { icon: "🎬", title: ts("上传源视频"), subtitle: ts("上传文件或从资产库引用已有视频"), tags: [ts("格式校验"), ts("时长检查")] },
          { icon: "🧹", title: ts("去掉字幕轨"), subtitle: ts("只移除可以单独开关的那一层字幕"), tags: [ts("不改画面"), ts("保留原音")] },
          { icon: "🚫", title: ts("印在画面上的去不掉"), subtitle: ts("直接做进画面里的字，这里处理不了"), tags: [ts("短视频字幕"), ts("烧进画面")] },
          { icon: "📥", title: ts("预览与下载"), subtitle: ts("处理完成后查看并下载视频"), tags: [ts("结果预览"), ts("画面不变")] },
        ]
      : [
          { icon: "🎬", title: ts("上传源视频"), subtitle: ts("上传文件或从资产库引用已有视频"), tags: [ts("格式校验"), ts("时长检查")] },
          { icon: "✨", title: ts("AI 超分增强"), subtitle: ts("降噪、去压缩瑕疵并恢复自然细节"), tags: [ts("细节恢复"), ts("智能降噪")] },
          { icon: "🖥️", title: ts("选择目标清晰度"), subtitle: ts("按需要输出 720P、1K 或 2K 视频"), tags: ["720P", "1K", "2K"] },
          { icon: "📥", title: ts("高清结果"), subtitle: ts("在线预览并下载高清成片"), tags: [ts("保留原音"), ts("实际计费")] },
        ];
  const configuredFeatures = (display.steps || []).map((item, index) => ({
    ...item,
    title: td(`agent.${workflow.code}.step.${index}.title`, item.title),
    subtitle: item.subtitle ? td(`agent.${workflow.code}.step.${index}.subtitle`, item.subtitle) : "",
    tags: item.tags?.map((tag) => td(`agent.${workflow.code}.step.${index}.tag.${tag}`, tag)),
  }));
  const features = fallbackFeatures.map(
    (fallback) => configuredFeatures.find((item) => item.title === fallback.title) || fallback
  );
  const heroTags = (isSubtitle
    ? [ts("只去字幕轨"), ts("不改画面"), ts("印在画面上的去不掉")]
    : (display.hero_tags?.length ? display.hero_tags : fallbackFeatures.flatMap((item) => item.tags || []).slice(0, 4))
  ).map((tag) => td(`agent.${workflow.code}.heroTag.${tag}`, tag));

  useEffect(() => {
    if (!resolutions.includes(resolution)) setResolution(resolutions[0]);
  }, [resolution, resolutions]);
  useEffect(() => {
    pollScopeRef.current = workflow.code;
    return () => {
      pollScopeRef.current = null;
      pollRef.current?.();
    };
  }, [workflow.code]);
  useEffect(() => {
    if (project) return;
    const timer = window.setInterval(() => {
      if (!document.hidden) setActiveFeature((value) => (value + 1) % Math.max(1, features.length));
    }, 3600);
    return () => window.clearInterval(timer);
  }, [features.length, project]);
  useEffect(() => {
    const id = window.localStorage.getItem(activeProjectKey);
    if (!id) return;
    api<Project>(`/api/agent-projects/${id}`)
      .then((saved) => {
        setProject(saved);
        const savedInputs = saved.inputs || {};
        const savedURL = String(savedInputs.video_url || savedInputs.source_video_url || "");
        if (savedURL) {
          setSource({
            url: savedURL,
            name: String(savedInputs.source_name || "源视频"),
            public_id: savedInputs.source_asset_id ? String(savedInputs.source_asset_id) : undefined,
            size_bytes: Number(savedInputs.size_bytes || 0) || undefined,
            duration: Number(savedInputs.duration_sec || savedInputs.duration_seconds || 0) || undefined,
          });
        }
        if (typeof savedInputs.preserve_audio === "boolean") setPreserveAudio(savedInputs.preserve_audio);
        if (!isSubtitle && typeof savedInputs.prompt === "string") setPrompt(savedInputs.prompt);
        if (["pending", "running"].includes(saved.status)) startPolling(saved.public_id);
      })
      .catch(() => window.localStorage.removeItem(activeProjectKey));
    // Restore only when opening this workflow; polling manages subsequent updates.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeProjectKey]);

  const mediaTasks = project?.media_tasks?.length ? project.media_tasks : ((project?.outputs?.media_tasks || []) as MediaTask[]);
  const latestTask = mediaTasks[mediaTasks.length - 1];
  const resultURL = [...mediaTasks].reverse().map(outputURL).find(Boolean) || "";
  const progress = project?.status === "succeeded" || project?.status === "failed"
    ? 100
    : Math.max(5, Number(latestTask?.progress || (project?.status === "running" ? 20 : project?.status === "pending" ? 5 : 0)));
  const busy = project?.status === "pending" || project?.status === "running";

  const startPolling = (id: string) => {
    if (pollScopeRef.current !== workflow.code) return;
    pollRef.current?.();
    pollRef.current = pollAsync(async (signal) => {
      try {
        const next = await api<Project>(`/api/agent-projects/${id}`, { signal });
        if (signal.aborted) return;
        setProject(next);
        if (["succeeded", "failed", "canceled"].includes(next.status)) {
          pollRef.current?.();
        }
      } catch {
        // Keep the currently visible progress while a transient request is retried.
      }
    }, 1600);
  };

  const selectFile = async (file?: File | null) => {
    if (!file) return;
    setError("");
    const maxMB = Math.max(1, Number(config.max_input_size_mb || 500));
    if (!file.type.startsWith("video/")) {
      setError(ts("请选择视频文件"));
      return;
    }
    if (file.size > maxMB * 1024 * 1024) {
      setError(td("videoUpscale.maxSize", "Video must not exceed {size} MB", { size: maxMB }));
      return;
    }
    const duration = await readVideoDuration(file);
    const maxDuration = Math.max(1, Number(config.max_input_duration_sec || 300));
    if (duration > maxDuration) {
      setError(td("videoUpscale.maxDuration", "Video duration must not exceed {seconds} seconds", { seconds: maxDuration }));
      return;
    }
    setUploading(true);
    try {
      const asset = await uploadAsset(file, { name: file.name, kind: "video", asset_type: `${utilityMode}_source` });
      setSource({ url: asset.url, name: asset.name || file.name, public_id: asset.public_id, size_bytes: asset.size_bytes || file.size, duration });
      setProject(null);
      window.localStorage.removeItem(activeProjectKey);
    } catch (err) {
      setError(err instanceof Error ? err.message : ts("视频上传失败"));
    } finally {
      setUploading(false);
    }
  };

  const selectStyleFile = async (file?: File | null) => {
    if (!file) return;
    if (!file.type.startsWith("image/")) {
      setError(ts("请选择图片文件作为风格参考"));
      return;
    }
    setUploading(true);
    setError("");
    try {
      const asset = await uploadAsset(file, { name: file.name, kind: "image", asset_type: "video_redraw_style_reference" });
      setStyleReference({ url: asset.url, name: asset.name || file.name, public_id: asset.public_id });
    } catch (err) {
      setError(err instanceof Error ? err.message : ts("风格参考图上传失败"));
    } finally {
      setUploading(false);
    }
  };

  const openStyleAssets = async () => {
    setStyleAssetOpen(true);
    try {
      const result = await listAssets({ kind: "image", page: 1, page_size: 60 });
      setStyleAssets((result.items || []) as AssetItem[]);
    } catch (err) {
      setStyleAssets([]);
      setError(err instanceof Error ? err.message : ts("图片资产库加载失败"));
    }
  };

  const openAssets = async () => {
    setAssetOpen(true);
    setError("");
    try {
      const result = await listAssets({ kind: "video", page: 1, page_size: 60 });
      setAssets((result.items || []) as AssetItem[]);
    } catch (err) {
      setAssets([]);
      setError(err instanceof Error ? err.message : ts("资产库加载失败"));
    }
  };

  const run = async () => {
    if (!source || submitting || busy) {
      if (!source) setError(ts("请先上传或从资产库选择源视频"));
      return;
    }
    setSubmitting(true);
    setError("");
    try {
      const inputs: Record<string, any> = {
        prompt: prompt.trim(),
        video_url: source.url,
        source_video_url: source.url,
        reference_videos: [source.url],
        source_asset_id: source.public_id,
        preserve_audio: preserveAudio,
        count: 1,
        n: 1,
        _mode: "auto",
      };
      if (isUpscale) {
        inputs.target_resolution = resolution;
        inputs.enhancement_mode = mode;
      } else if (isRedraw) {
        inputs.style_strength = styleStrength;
        inputs.preserve_motion = preserveMotion;
        inputs.preserve_identity = preserveIdentity;
        if (styleReference) {
          inputs.style_reference_url = styleReference.url;
          inputs.style_reference_asset_id = styleReference.public_id;
          inputs.reference_images = [styleReference.url];
        }
      } else {
        inputs.subtitle_mode = "soft_track";
        inputs.prompt = "";
        inputs.source_name = source.name;
        if (source.size_bytes) inputs.size_bytes = source.size_bytes;
        if (source.duration && source.duration > 0) inputs.duration_sec = source.duration;
      }
      const next = await api<Project>(`/api/agents/${workflow.code}/projects`, {
        method: "POST",
        body: JSON.stringify({ inputs }),
      });
      setProject(next);
      window.localStorage.setItem(activeProjectKey, next.public_id);
      startPolling(next.public_id);
    } catch (err) {
      setError(err instanceof Error ? err.message : ts("任务启动失败"));
    } finally {
      setSubmitting(false);
    }
  };

  const retry = async () => {
    if (!project) return;
    setError("");
    try {
      await api(`/api/agent-projects/${project.public_id}/retry`, { method: "POST" });
      startPolling(project.public_id);
    } catch (err) {
      setError(err instanceof Error ? err.message : ts("重试失败"));
    }
  };

  const cancel = async () => {
    if (!project || project.status !== "pending") return;
    setError("");
    try {
      await api(`/api/agent-projects/${project.public_id}/cancel`, { method: "POST" });
      const next = await api<Project>(`/api/agent-projects/${project.public_id}`);
      setProject(next);
      pollRef.current?.();
    } catch (err) {
      setError(err instanceof Error ? err.message : ts("取消任务失败"));
    }
  };

  const openHistory = async () => {
    const next = !historyOpen;
    setHistoryOpen(next);
    if (!next) return;
    try {
      const result = await api<{ items: HistoryItem[] }>(`/api/agent-projects?workflow_code=${encodeURIComponent(workflow.code)}&page=1&page_size=30`);
      setHistory(result.items || []);
    } catch {
      setHistory([]);
    }
  };

  return (
    <div className="mcdl-workspace mcdl-workspace--video relative flex min-h-0 flex-1 flex-col overflow-hidden text-gray-900 dark:text-white">
      <div className="mcdl-workspace-atmosphere pointer-events-none absolute inset-0" />
      <div className="relative z-10 shrink-0 px-4 py-3 sm:px-6">
        <div className="flex items-center gap-2">
          <button type="button" onClick={() => { setProject(null); setSource(null); setPrompt(""); setError(""); window.localStorage.removeItem(activeProjectKey); }} className="flex h-9 items-center gap-1.5 rounded-xl bg-primary px-3 text-sm font-semibold text-dark"><Sparkles size={15} />{t("common.newTask")}</button>
          <button type="button" onClick={openHistory} className="flex h-9 items-center gap-1.5 rounded-xl border border-gray-100 bg-white px-3 text-sm text-gray-600 transition hover:bg-gray-50 dark:border-white/10 dark:bg-white/5 dark:text-gray-300 dark:hover:bg-white/10"><History size={15} />{t("common.history")}</button>
        </div>
      </div>

      <div className={`relative z-10 flex min-h-0 flex-1 flex-col px-3 sm:px-5 lg:px-8 ${project ? "overflow-y-auto py-4" : "overflow-y-auto overscroll-contain lg:overflow-hidden"}`}>
      <div className="mx-auto flex min-h-0 w-full max-w-7xl flex-1 flex-col">
        {!project && (
          <AgentLanding
            workflowCode={workflow.code}
            workflowIcon={workflow.icon || (isRedraw ? "🪄" : isSubtitle ? "🧹" : "✨")}
            workflowName={td(`agent.${workflow.code}.name`, workflow.name)}
            workflowDescription={isSubtitle ? ts("只能去掉单独的字幕轨。已经印在画面上的字幕去不掉。") : td(`agent.${workflow.code}.description`, workflow.description || "")}
            heroTags={heroTags}
            features={features}
            activeIndex={activeFeature}
            onSelect={setActiveFeature}
            theme={AGENT_THEMES[display.theme || ""] || AGENT_THEMES.comic}
            generationType="video"
            compactOnMobile
          />
        )}

        {!project && (
          <div className="relative z-10 shrink-0 pb-3 pt-3 sm:pb-5 sm:pt-2">
          <section className="soft-input mx-auto w-full max-w-[1040px] overflow-hidden">
            <div className="border-b border-gray-50 px-3 py-2 dark:border-white/10 sm:px-4">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <div className="flex flex-wrap items-center gap-2">
                  <button type="button" onClick={openAssets} className="flex h-9 items-center gap-2 rounded-xl border border-gray-100 bg-gray-50 px-3 text-xs font-medium text-gray-600 transition hover:bg-white dark:border-white/10 dark:bg-white/5 dark:text-gray-300 dark:hover:bg-white/10"><FolderOpen size={15} />{t("upscale.asset")}</button>
                  {isRedraw && (
                    <>
                      <label className="flex h-9 cursor-pointer items-center gap-2 rounded-xl border border-gray-100 bg-gray-50 px-3 text-xs font-medium text-gray-600 transition hover:bg-white dark:border-white/10 dark:bg-white/5 dark:text-gray-300 dark:hover:bg-white/10">
                        <ImageIcon size={15} />{ts("上传风格图")}
                        <input type="file" accept="image/*" className="hidden" onChange={(event) => { void selectStyleFile(event.target.files?.[0]); event.target.value = ""; }} />
                      </label>
                      <button type="button" onClick={openStyleAssets} className="flex h-9 items-center gap-2 rounded-xl border border-gray-100 bg-gray-50 px-3 text-xs font-medium text-gray-600 transition hover:bg-white dark:border-white/10 dark:bg-white/5 dark:text-gray-300 dark:hover:bg-white/10"><FolderOpen size={15} />{ts("风格资产")}</button>
                    </>
                  )}
                </div>
                <div className="text-[11px] text-gray-400">{t("upscale.limit", { size: config.max_input_size_mb || 500, duration: config.max_input_duration_sec || 300 })}</div>
              </div>
            </div>

            <div className="flex min-h-[104px] flex-col gap-2 px-3 py-3 sm:flex-row sm:gap-3 sm:px-4">
              <div className="flex w-full shrink-0 items-stretch gap-2 sm:w-auto">
                {source ? (
                  <div className="group/source relative min-h-[96px] w-full overflow-hidden rounded-2xl border border-gray-100 bg-black dark:border-white/10 sm:min-h-0 sm:w-40">
                    <video src={source.url} muted preload="metadata" className="h-full min-h-[88px] w-full object-cover" />
                    <div className="absolute inset-x-0 bottom-0 bg-black/65 px-2 py-1.5 text-[10px] text-white">
                      <div className="truncate">{source.name}</div>
                      <div className="mt-0.5 text-white/60">{source.duration ? `${source.duration.toFixed(1)}s` : ""}{source.duration && source.size_bytes ? " · " : ""}{formatBytes(source.size_bytes)}</div>
                    </div>
                    <button type="button" onClick={() => { setSource(null); window.localStorage.removeItem(activeProjectKey); }} className="absolute right-1.5 top-1.5 flex h-6 w-6 items-center justify-center rounded-full bg-black/65 text-white opacity-80 transition hover:opacity-100"><X size={13} /></button>
                  </div>
                ) : (
                  <label className="flex min-h-[96px] w-full cursor-pointer flex-col items-center justify-center rounded-2xl border border-dashed border-gray-200 bg-white/70 px-3 text-center transition hover:border-primary/50 hover:bg-primary/5 dark:border-white/10 dark:bg-white/5 dark:hover:bg-primary/10 sm:min-h-0 sm:w-32 sm:px-0">
                    {uploading ? <Loader2 size={21} className="animate-spin text-primary" /> : <Upload size={21} className="text-gray-400" />}
                    <span className="mt-2 text-xs text-gray-400 sm:text-[10px]">{uploading ? t("upscale.uploading") : t("upscale.upload")}</span>
                    <input type="file" accept="video/mp4,video/webm,video/quicktime,video/x-matroska" disabled={uploading} className="hidden" onChange={(event) => { void selectFile(event.target.files?.[0]); event.target.value = ""; }} />
                  </label>
                )}
                {isRedraw && styleReference && (
                  <div className="group/style relative min-h-[96px] w-24 shrink-0 overflow-hidden rounded-2xl border border-violet-100 bg-gray-100 dark:border-violet-400/20 dark:bg-white/5 sm:min-h-0 sm:w-20">
                    {/* eslint-disable-next-line @next/next/no-img-element */}
                    <img loading="lazy" decoding="async" src={styleReference.url} alt={styleReference.name} className="h-full min-h-[88px] w-full object-cover" />
                    <div className="absolute inset-x-0 bottom-0 truncate bg-black/65 px-1.5 py-1 text-center text-[9px] text-white">{ts("风格参考")}</div>
                    <button type="button" onClick={() => setStyleReference(null)} className="absolute right-1 top-1 flex h-5 w-5 items-center justify-center rounded-full bg-black/65 text-white"><X size={11} /></button>
                  </div>
                )}
              </div>
              {isSubtitle ? (
                <div className="min-w-0 flex-1 py-1"><SubtitleLimitNotice ts={ts} /></div>
              ) : (
                <textarea
                  value={prompt}
                  onChange={(event) => setPrompt(event.target.value)}
                  rows={3}
                  placeholder={isRedraw ? ts("描述目标画风，例如：日系动漫、厚涂插画、赛博朋克电影感") : t("upscale.prompt")}
                  className="min-h-[72px] min-w-0 flex-1 resize-none bg-transparent px-1 py-1 text-sm leading-relaxed text-gray-700 outline-none placeholder:text-gray-400 dark:text-gray-100 dark:placeholder:text-gray-500 sm:min-h-[88px]"
                />
              )}
            </div>

            <div className="flex items-end gap-2 border-t border-gray-50 px-3 py-3 dark:border-white/10 sm:items-center sm:px-4">
              <div className="flex min-w-0 flex-1 flex-wrap items-center gap-2 overflow-visible sm:flex-nowrap sm:overflow-x-auto sm:pb-1">
                {isUpscale && (
                  <>
                    <MediaOptionMenu icon={<Film size={14} />} activeLabel={resolution} title={t("upscale.resolution")} subtitle={ts("选择输出视频清晰度")}>
                      {(close) => <div className="space-y-1.5">{resolutions.map((item) => <MediaMenuOption key={item} selected={resolution === item} onClick={() => { setResolution(item); close(); }}>{item}</MediaMenuOption>)}</div>}
                    </MediaOptionMenu>
                    <MediaOptionMenu icon={<Sparkles size={14} />} activeLabel={mode === "detail" ? t("upscale.detail") : mode === "denoise" ? t("upscale.denoise") : t("upscale.balanced")} title={t("upscale.mode")} subtitle={ts("选择视频增强处理方式")}>
                      {(close) => <div className="space-y-1.5">{[
                        { value: "balanced", label: t("upscale.balanced") },
                        { value: "detail", label: t("upscale.detail") },
                        { value: "denoise", label: t("upscale.denoise") },
                      ].map((item) => <MediaMenuOption key={item.value} selected={mode === item.value} onClick={() => { setMode(item.value); close(); }}>{item.label}</MediaMenuOption>)}</div>}
                    </MediaOptionMenu>
                  </>
                )}
                {isRedraw && (
                  <>
                    <MediaOptionMenu icon={<Sparkles size={14} />} activeLabel={ts(`风格 ${Math.round(styleStrength * 100)}%`)} title={ts("风格强度")} subtitle={ts("调整转绘风格对原视频的影响程度")}>
                      {(close) => <div className="space-y-1.5">{[0.4, 0.65, 0.8, 1].map((value) => <MediaMenuOption key={value} selected={styleStrength === value} onClick={() => { setStyleStrength(value); close(); }}>{ts(`风格 ${Math.round(value * 100)}%`)}</MediaMenuOption>)}</div>}
                    </MediaOptionMenu>
                    <button type="button" onClick={() => setPreserveMotion((value) => !value)} className={`h-9 shrink-0 rounded-xl border px-3 text-xs font-medium transition ${preserveMotion ? "border-primary/30 bg-primary/10 text-primary dark:border-primary/30 dark:bg-primary/10 dark:text-primary" : "border-gray-100 bg-gray-50 text-gray-500 hover:border-primary/30 dark:border-white/10 dark:bg-white/5 dark:text-gray-300"}`}>{ts("动作保持")}：{preserveMotion ? "ON" : "OFF"}</button>
                    <button type="button" onClick={() => setPreserveIdentity((value) => !value)} className={`h-9 shrink-0 rounded-xl border px-3 text-xs font-medium transition ${preserveIdentity ? "border-primary/30 bg-primary/10 text-primary dark:border-primary/30 dark:bg-primary/10 dark:text-primary" : "border-gray-100 bg-gray-50 text-gray-500 hover:border-primary/30 dark:border-white/10 dark:bg-white/5 dark:text-gray-300"}`}>{ts("人物保持")}：{preserveIdentity ? "ON" : "OFF"}</button>
                  </>
                )}
                <button type="button" onClick={() => setPreserveAudio((value) => !value)} className={`h-9 shrink-0 rounded-xl border px-3 text-xs font-medium transition ${preserveAudio ? "border-primary/30 bg-primary/10 text-primary dark:border-primary/30 dark:bg-primary/10 dark:text-primary" : "border-gray-100 bg-gray-50 text-gray-500 hover:border-primary/30 dark:border-white/10 dark:bg-white/5 dark:text-gray-300"}`}>{t("upscale.preserveAudio")}：{preserveAudio ? "ON" : "OFF"}</button>
              </div>
              <button type="button" onClick={run} disabled={submitting || uploading || busy || !source} className="flex h-12 w-12 shrink-0 items-center justify-center rounded-full bg-secondary text-white shadow-md transition hover:bg-secondary/90 disabled:cursor-not-allowed disabled:opacity-40">
                {submitting ? <Loader2 size={20} className="animate-spin" /> : <ArrowUp size={20} />}
              </button>
            </div>
          </section>
          {error && <div className="mx-auto mt-2 w-full max-w-[1040px] px-1 text-xs leading-5 text-red-500">{error}</div>}
          </div>
        )}

        {project && <div className="grid flex-1 gap-4 lg:grid-cols-[minmax(0,1.1fr)_minmax(360px,.9fr)]">
          <section className="rounded-3xl border border-primary/30 bg-white/90 p-4 shadow-sm backdrop-blur dark:border-primary/30 dark:bg-[#111827]/90 sm:p-5">
            <div className="mb-4 flex items-center justify-between">
              <div className="flex items-center gap-2 font-semibold"><Film size={18} className="text-primary" />{t("upscale.source")}</div>
              {source && <button type="button" onClick={() => { setSource(null); setProject(null); window.localStorage.removeItem(activeProjectKey); }} className="flex h-8 w-8 items-center justify-center rounded-lg text-gray-400 hover:bg-gray-100 dark:hover:bg-white/10"><X size={15} /></button>}
            </div>
            {source ? (
              <div className="overflow-hidden rounded-2xl border border-gray-100 bg-black dark:border-white/10">
                <video src={source.url} controls preload="metadata" className="aspect-video w-full object-contain" />
                <div className="flex flex-wrap items-center justify-between gap-2 bg-white px-3 py-2 text-xs dark:bg-white/5">
                  <span className="max-w-[70%] truncate font-medium">{source.name}</span>
                  <span className="text-gray-400">{source.duration ? `${source.duration.toFixed(1)}s` : ""}{source.duration && source.size_bytes ? " · " : ""}{formatBytes(source.size_bytes)}</span>
                </div>
              </div>
            ) : (
              <label className="flex min-h-[200px] cursor-pointer flex-col items-center justify-center rounded-2xl border-2 border-dashed border-primary/30 bg-primary/10 px-5 text-center transition hover:border-primary/30 hover:bg-primary/10 dark:border-primary/30 dark:bg-primary/5 dark:hover:bg-primary/10 sm:min-h-[300px]">
                {uploading ? <Loader2 size={36} className="animate-spin text-primary" /> : <Upload size={36} className="text-primary" />}
                <div className="mt-4 font-semibold">{uploading ? t("upscale.uploading") : t("upscale.upload")}</div>
                <div className="mt-2 text-xs leading-5 text-gray-400">{t("upscale.limit", { size: config.max_input_size_mb || 500, duration: config.max_input_duration_sec || 300 })}</div>
                <input type="file" accept="video/mp4,video/webm,video/quicktime,video/x-matroska" disabled={uploading} className="hidden" onChange={(event) => { void selectFile(event.target.files?.[0]); event.target.value = ""; }} />
              </label>
            )}
            <button type="button" onClick={openAssets} className="mt-3 flex h-11 w-full items-center justify-center gap-2 rounded-xl border border-violet-200 bg-violet-50 text-sm font-medium text-violet-700 dark:border-violet-400/20 dark:bg-violet-400/10 dark:text-violet-200"><FolderOpen size={16} />{t("upscale.asset")}</button>
          </section>

          <section className="flex flex-col rounded-3xl border border-gray-100 bg-white/90 p-4 shadow-sm backdrop-blur dark:border-white/10 dark:bg-[#111827]/90 sm:p-5">
            <div className="font-semibold">{isRedraw ? ts("转绘设置") : isSubtitle ? ts("去字幕设置") : t("upscale.settings")}</div>
            {isUpscale && <div className="mt-5">
              <div className="mb-2 text-xs text-gray-400">{t("upscale.resolution")}</div>
              <div className="grid grid-cols-3 gap-2">
                {resolutions.map((item) => <button key={item} type="button" onClick={() => setResolution(item)} className={`h-12 rounded-xl border text-sm font-semibold transition ${resolution === item ? "border-primary/30 bg-primary/10 text-primary shadow-sm dark:bg-primary/15 dark:text-primary" : "border-gray-100 bg-gray-50 text-gray-500 dark:border-white/10 dark:bg-white/5 dark:text-gray-300"}`}>{item}</button>)}
              </div>
            </div>}
            {isUpscale && <div className="mt-4">
              <div className="mb-2 text-xs text-gray-400">{t("upscale.mode")}</div>
              <div className="grid grid-cols-3 gap-2">
                {[["balanced", t("upscale.balanced")], ["detail", t("upscale.detail")], ["denoise", t("upscale.denoise")]].map(([value, label]) => <button key={value} type="button" onClick={() => setMode(value)} className={`h-10 rounded-xl border text-xs font-medium ${mode === value ? "border-primary/30 bg-primary/10 text-primary dark:bg-primary/15 dark:text-primary" : "border-gray-100 text-gray-500 dark:border-white/10 dark:text-gray-300"}`}>{label}</button>)}
              </div>
            </div>}
            {isRedraw && (
              <>
                <div className="mt-5">
                  <div className="mb-2 flex items-center justify-between text-xs text-gray-400"><span>{ts("风格强度")}</span><span>{Math.round(styleStrength * 100)}%</span></div>
                  <input type="range" min={0} max={1} step={0.05} value={styleStrength} onChange={(event) => setStyleStrength(Number(event.target.value))} className="w-full accent-violet-500" />
                </div>
                <div className="mt-4 rounded-2xl border border-violet-100 bg-violet-50/40 p-3 dark:border-violet-400/15 dark:bg-violet-400/5">
                  <div className="mb-2 flex items-center justify-between"><span className="flex items-center gap-2 text-sm font-medium"><ImageIcon size={15} />{ts("风格参考图（可选）")}</span>{styleReference && <button type="button" onClick={() => setStyleReference(null)} className="text-gray-400"><X size={14} /></button>}</div>
                  {styleReference ? <div className="flex items-center gap-3"><img loading="lazy" decoding="async" src={styleReference.url} alt="" className="h-16 w-16 rounded-xl object-cover" /><span className="min-w-0 flex-1 truncate text-xs">{styleReference.name}</span></div> : <div className="grid grid-cols-2 gap-2">
                    <label className="flex h-10 cursor-pointer items-center justify-center gap-2 rounded-xl border border-violet-200 bg-white text-xs text-violet-700 dark:bg-white/5"><Upload size={14} />{ts("上传图片")}<input type="file" accept="image/*" className="hidden" onChange={(event) => { void selectStyleFile(event.target.files?.[0]); event.target.value = ""; }} /></label>
                      <button type="button" onClick={openStyleAssets} className="flex h-10 items-center justify-center gap-2 rounded-xl border border-violet-200 bg-white text-xs text-violet-700 dark:bg-white/5"><FolderOpen size={14} />{ts("资产库")}</button>
                  </div>}
                </div>
                <div className="mt-4 grid grid-cols-2 gap-2 text-xs">
                  <label className="flex items-center gap-2 rounded-xl border border-gray-100 px-3 py-2 dark:border-white/10"><input type="checkbox" checked={preserveMotion} onChange={(e) => setPreserveMotion(e.target.checked)} />{ts("保留动作")}</label>
                  <label className="flex items-center gap-2 rounded-xl border border-gray-100 px-3 py-2 dark:border-white/10"><input type="checkbox" checked={preserveIdentity} onChange={(e) => setPreserveIdentity(e.target.checked)} />{ts("保留人物")}</label>
                </div>
              </>
            )}
            {isSubtitle && <div className="mt-4"><SubtitleLimitNotice ts={ts} /></div>}
            <label className="mt-4 flex cursor-pointer items-center justify-between rounded-xl border border-gray-100 px-3 py-3 text-sm dark:border-white/10">
              <span><span className="font-medium">{t("upscale.preserveAudio")}</span><span className="mt-0.5 block text-[11px] text-gray-400">{t("upscale.preserveAudioDesc")}</span></span>
              <input type="checkbox" checked={preserveAudio} onChange={(event) => setPreserveAudio(event.target.checked)} className="h-4 w-4 accent-primary" />
            </label>
            {!isSubtitle && <textarea value={prompt} onChange={(event) => setPrompt(event.target.value)} rows={2} placeholder={isRedraw ? ts("描述目标画风，例如：日系动漫、厚涂插画、赛博朋克电影感") : t("upscale.prompt")} className="mt-4 min-h-[64px] resize-none rounded-xl border border-gray-100 bg-gray-50 px-3 py-2 text-sm outline-none focus:border-primary/30 dark:border-white/10 dark:bg-white/5" />}

            {project && (
              <div className="mt-4 rounded-2xl border border-gray-100 bg-gray-50 p-3 dark:border-white/10 dark:bg-white/5">
                <div className="flex items-center justify-between gap-3 text-xs">
                  <span className="font-medium">{project.status === "succeeded" ? t("upscale.completed") : project.status === "failed" ? t("upscale.failed") : t("upscale.processing")}</span>
                  <span className="text-primary">{Math.round(progress)}%</span>
                </div>
                <div className="mt-2 h-2 overflow-hidden rounded-full bg-gray-200 dark:bg-white/10"><div className={`h-full rounded-full transition-all ${project.status === "failed" ? "bg-red-500" : "bg-primary/100"}`} style={{ width: `${progress}%` }} /></div>
                {project.status === "failed" && <div className="mt-2 text-xs leading-5 text-red-500">{project.error_message || latestTask?.error_message || "处理失败"}</div>}
                {project.status === "succeeded" && <div className="mt-2 flex flex-wrap items-center justify-between gap-2 text-[11px] text-gray-400"><span>{isUpscale ? `${t("upscale.target")}：${project.outputs?.target_resolution || resolution}` : isSubtitle ? `处理路径：${subtitleResultLabel(String(project.outputs?.subtitle_mode || ""), ts)}` : `风格强度：${Math.round(styleStrength * 100)}%`}</span><span>{t("upscale.actualCost", { value: Number(project.actual_cost || 0).toFixed(2) })}</span></div>}
              </div>
            )}
            {error && <div className="mt-3 text-xs leading-5 text-red-500">{error}</div>}
            <div className="mt-auto flex gap-2">
              {project?.status === "pending" && <button type="button" onClick={cancel} className="flex h-12 items-center justify-center gap-2 rounded-xl border border-red-200 px-4 text-sm font-medium text-red-500 hover:bg-red-50 dark:border-red-400/20 dark:hover:bg-red-500/10"><X size={16} />{t("upscale.cancel")}</button>}
              <button type="button" onClick={project?.status === "failed" ? retry : run} disabled={submitting || uploading || busy || !source} className="flex h-12 flex-1 items-center justify-center gap-2 rounded-xl bg-primary text-sm font-semibold text-dark shadow-lg shadow-primary/20 transition hover:brightness-105 disabled:cursor-not-allowed disabled:opacity-40">
                {submitting || busy ? <Loader2 size={17} className="animate-spin" /> : project?.status === "failed" ? <RefreshCw size={17} /> : <ArrowUp size={17} />}
                {project?.status === "failed" ? t("upscale.retry") : busy ? t("upscale.processing") : t("upscale.start")}
              </button>
            </div>
          </section>
        </div>}

        {resultURL && (
          <section className="mt-4 rounded-3xl border border-emerald-100 bg-white/95 p-4 shadow-sm dark:border-emerald-400/15 dark:bg-[#111827]/95 sm:p-5">
            <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
              <div className="flex items-center gap-2 font-semibold"><CheckCircle2 size={18} className="text-emerald-500" />{isRedraw ? ts("转绘结果") : isSubtitle ? ts("无字幕视频") : `${t("upscale.result")} · ${project?.outputs?.target_resolution || resolution}`}</div>
              <a href={resultURL} target="_blank" rel="noreferrer" download className="flex h-9 items-center gap-2 rounded-xl bg-gray-900 px-3 text-xs font-medium text-white dark:bg-white dark:text-gray-950"><Download size={15} />{t("upscale.download")}</a>
            </div>
            <video src={resultURL} controls preload="metadata" className="mx-auto max-h-[58vh] w-full rounded-2xl bg-black object-contain" />
          </section>
        )}
      </div>
      </div>

      {assetOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" onClick={() => setAssetOpen(false)}>
          <div className="w-full max-w-3xl rounded-2xl bg-white p-4 shadow-2xl dark:border dark:border-white/10 dark:bg-gray-900" onClick={(event) => event.stopPropagation()}>
            <div className="flex items-center justify-between"><div><div className="font-semibold">{t("upscale.selectAsset")}</div><div className="mt-1 text-xs text-gray-400">{t("upscale.assetOnly")}</div></div><button onClick={() => setAssetOpen(false)} className="flex h-9 w-9 items-center justify-center rounded-xl bg-gray-100 dark:bg-white/10"><X size={16} /></button></div>
            <div className="mt-4 grid max-h-[60vh] grid-cols-2 gap-3 overflow-y-auto pr-1 sm:grid-cols-3">
              {assets.map((asset) => <button key={asset.public_id} type="button" onClick={() => {
                const maxBytes = Math.max(1, Number(config.max_input_size_mb || 500)) * 1024 * 1024;
                if (asset.size_bytes && asset.size_bytes > maxBytes) {
                  setError(td("videoUpscale.maxSize", "Video must not exceed {size} MB", { size: config.max_input_size_mb || 500 }));
                  setAssetOpen(false);
                  return;
                }
                const duration = assetDurationSeconds(asset);
                const maxDuration = Math.max(1, Number(config.max_input_duration_sec || 300));
                if (isSubtitle && duration > maxDuration) {
                  setError(td("videoUpscale.maxDuration", "Video duration must not exceed {seconds} seconds", { seconds: maxDuration }));
                  setAssetOpen(false);
                  return;
                }
                setSource({ url: asset.url, name: asset.name || "视频资产", public_id: asset.public_id, size_bytes: asset.size_bytes, duration });
                setProject(null);
                window.localStorage.removeItem(activeProjectKey);
                setAssetOpen(false);
              }} className="overflow-hidden rounded-xl border border-gray-100 bg-gray-50 text-left transition hover:border-primary/30 dark:border-white/10 dark:bg-white/5">
                <video src={asset.url} preload="metadata" muted className="aspect-video w-full bg-black object-cover" />
                <div className="truncate px-3 py-2 text-xs font-medium">{asset.name || "视频资产"}</div>
              </button>)}
              {assets.length === 0 && <div className="col-span-full py-16 text-center text-sm text-gray-400">{t("upscale.noAssets")}</div>}
            </div>
          </div>
        </div>
      )}

      {styleAssetOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" onClick={() => setStyleAssetOpen(false)}>
          <div className="w-full max-w-3xl rounded-2xl bg-white p-4 shadow-2xl dark:border dark:border-white/10 dark:bg-gray-900" onClick={(event) => event.stopPropagation()}>
                <div className="flex items-center justify-between"><div><div className="font-semibold">{ts("选择风格参考图")}</div><div className="mt-1 text-xs text-gray-400">{ts("只显示当前账号可访问的图片资产")}</div></div><button onClick={() => setStyleAssetOpen(false)} className="flex h-9 w-9 items-center justify-center rounded-xl bg-gray-100 dark:bg-white/10"><X size={16} /></button></div>
            <div className="mt-4 grid max-h-[60vh] grid-cols-2 gap-3 overflow-y-auto pr-1 sm:grid-cols-4">
              {styleAssets.map((asset) => <button key={asset.public_id} type="button" onClick={() => { setStyleReference({ url: asset.url, name: asset.name || "风格参考图", public_id: asset.public_id }); setStyleAssetOpen(false); }} className="overflow-hidden rounded-xl border border-gray-100 bg-gray-50 text-left hover:border-violet-300 dark:border-white/10 dark:bg-white/5">
                <img loading="lazy" decoding="async" src={asset.url} alt="" className="aspect-square w-full object-cover" />
                <div className="truncate px-3 py-2 text-xs font-medium">{asset.name || "图片资产"}</div>
              </button>)}
              {styleAssets.length === 0 && <div className="col-span-full py-16 text-center text-sm text-gray-400">{ts("暂无可用图片资产")}</div>}
            </div>
          </div>
        </div>
      )}

      {historyOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" onClick={() => setHistoryOpen(false)}>
          <div className="w-full max-w-lg rounded-2xl bg-white p-4 shadow-2xl dark:border dark:border-white/10 dark:bg-gray-900" onClick={(event) => event.stopPropagation()}>
            <div className="flex items-center justify-between"><div className="font-semibold">{t("upscale.historyTitle")}</div><button onClick={() => setHistoryOpen(false)} className="flex h-9 w-9 items-center justify-center rounded-xl bg-gray-100 dark:bg-white/10"><X size={16} /></button></div>
            <div className="mt-4 max-h-[60vh] space-y-2 overflow-y-auto">
              {history.map((item) => <button key={item.public_id} type="button" onClick={async () => { const next = await api<Project>(`/api/agent-projects/${item.public_id}`); setProject(next); window.localStorage.setItem(activeProjectKey, next.public_id); if (["pending", "running"].includes(next.status)) startPolling(next.public_id); setHistoryOpen(false); }} className="flex w-full items-center justify-between gap-3 rounded-xl border border-gray-100 px-3 py-3 text-left hover:border-primary/30 dark:border-white/10">
                <span className="min-w-0"><span className="block truncate text-sm font-medium">{item.workflow_name || workflow.name}</span><span className="mt-1 block text-[11px] text-gray-400">{new Date(item.created_at).toLocaleString()}</span></span>
                <span className="shrink-0 rounded-full bg-gray-100 px-2 py-1 text-[10px] dark:bg-white/10">{item.status}</span>
              </button>)}
              {history.length === 0 && <div className="py-14 text-center text-sm text-gray-400">{t("upscale.noHistory")}</div>}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

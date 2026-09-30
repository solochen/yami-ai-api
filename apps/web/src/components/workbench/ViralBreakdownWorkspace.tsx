"use client";

import { useEffect, useRef, useState } from "react";
import { AlertTriangle, CheckCircle2, Copy, History, Loader2, RefreshCw, Upload } from "lucide-react";
import { api, uploadAsset } from "@/lib/api";
import { pollAsync } from "@/lib/pollAsync";
import { useI18n } from "@/i18n/I18nProvider";

type WorkflowLike = {
  code: string;
  name: string;
  description?: string;
  price_rule?: { unit_price?: number };
};

type Keyframe = { t_ms?: number; url?: string; label?: string };

type Project = {
  public_id: string;
  status: string;
  error_message?: string;
  inputs?: { share_text?: string; video_url?: string; source_url?: string };
  outputs?: {
    current_step?: string;
    progress_message?: string;
    progress_percent?: number;
    stages?: Record<string, string>;
    script_markdown?: string;
    production_markdown?: string;
    completion_status?: "running" | "complete" | "degraded" | "failed";
    worker_version?: string;
    notes?: string[];
    quality?: {
      status?: string;
      coverage_percent?: number;
      max_gap_sec?: number;
      issues?: string[];
      segment_total?: number;
      segment_done?: number;
    };
    segment_checkpoint_summary?: { total?: number; done?: number; failed?: number; coverage_percent?: number };
    keyframes?: Keyframe[];
    frame_total?: number;
    frame_kept?: number;
    rewrites?: Record<string, string>;
    duration_ms?: number;
  };
};

type HistoryItem = { public_id: string; status: string; created_at: string };

const STEPS = [
  { id: "understand", label: "理解" },
  { id: "subtitle", label: "字幕" },
  { id: "frames", label: "拆帧" },
  { id: "verify", label: "印证" },
  { id: "script", label: "剧本" },
];

const REWRITES = [
  { id: "shorten", label: "缩写 50%" },
  { id: "oral", label: "口语稿" },
  { id: "storyboard", label: "分镜表" },
  { id: "english", label: "英文版" },
];

export function ViralBreakdownWorkspace({ workflow }: { workflow: WorkflowLike }) {
  const { ts } = useI18n();
  const [mode, setMode] = useState<"link" | "upload">("link");
  const [shareText, setShareText] = useState("");
  const [videoURL, setVideoURL] = useState("");
  const [videoName, setVideoName] = useState("");
  const [project, setProject] = useState<Project | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [rewriting, setRewriting] = useState("");
  const [rewriteView, setRewriteView] = useState("");
  const [resultView, setResultView] = useState<"source" | "production">("source");
  const [retrying, setRetrying] = useState(false);
  const [error, setError] = useState("");
  const [copied, setCopied] = useState(false);
  const [historyOpen, setHistoryOpen] = useState(false);
  const [history, setHistory] = useState<HistoryItem[]>([]);
  const fileRef = useRef<HTMLInputElement>(null);
  const pollRef = useRef<(() => void) | null>(null);
  const activeKey = `viral_breakdown_active:${workflow.code}`;
  const outputs = project?.outputs || {};
  const busy = submitting || retrying || project?.status === "pending" || project?.status === "running";
  const rate = Number(workflow.price_rule?.unit_price || 0);
  const operationalNotes = outputs.notes || [];
  const legacyDegraded = project?.status === "succeeded" && !outputs.completion_status && (operationalNotes.some((note) => /未完成|超时|deadline exceeded/i.test(note)) || /【冲突】[\s\S]*(未完成|超时|deadline exceeded)/i.test(outputs.script_markdown || ""));
  const degraded = outputs.completion_status === "degraded" || outputs.completion_status === "failed" || legacyDegraded;
  const complete = project?.status === "succeeded" && !degraded;
  const sourceMarkdown = rewriteView && outputs.rewrites?.[rewriteView] ? outputs.rewrites[rewriteView] : outputs.script_markdown || "";
  const markdown = resultView === "production" ? outputs.production_markdown || "" : sourceMarkdown;
  const keyframes = outputs.keyframes || [];

  const startPolling = (id: string) => {
    pollRef.current?.();
    pollRef.current = pollAsync(async (signal) => {
      const next = await api<Project>(`/api/agent-projects/${id}`, { signal });
      if (signal.aborted) return;
      setProject(next);
      if (next.status === "succeeded" && next.outputs?.production_markdown) setResultView("production");
      if (["succeeded", "failed", "canceled"].includes(next.status)) pollRef.current?.();
    }, 1600);
  };

  useEffect(() => () => pollRef.current?.(), []);

  useEffect(() => {
    const id = window.localStorage.getItem(activeKey);
    if (!id) return;
    api<Project>(`/api/agent-projects/${id}`)
      .then((saved) => {
        setProject(saved);
        setResultView(saved.outputs?.production_markdown ? "production" : "source");
        setShareText(saved.inputs?.share_text || "");
        if (saved.inputs?.video_url && !saved.inputs.share_text) {
          setMode("upload");
          setVideoURL(saved.inputs.video_url);
        }
        if (["pending", "running"].includes(saved.status)) startPolling(saved.public_id);
      })
      .catch(() => window.localStorage.removeItem(activeKey));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeKey]);

  const submit = async (nextVideoURL = videoURL) => {
    const text = shareText.trim();
    if (mode === "link" && !text) {
      setError(ts("请粘贴抖音分享文本或视频链接"));
      return;
    }
    if (mode === "upload" && !nextVideoURL) {
      setError(ts("请先上传视频"));
      return;
    }
    setSubmitting(true);
    setError("");
    setRewriteView("");
    setResultView("source");
    try {
      const inputs = mode === "upload" ? { video_url: nextVideoURL } : { share_text: text };
      const next = await api<Project>(`/api/agents/${workflow.code}/projects`, {
        method: "POST",
        body: JSON.stringify({ inputs }),
      });
      setProject(next);
      window.localStorage.setItem(activeKey, next.public_id);
      startPolling(next.public_id);
    } catch (err) {
      setError(err instanceof Error ? err.message : ts("任务启动失败"));
    } finally {
      setSubmitting(false);
    }
  };

  const onFile = async (file?: File) => {
    if (!file) return;
    setUploading(true);
    setError("");
    try {
      const asset = await uploadAsset(file, { name: file.name, kind: "video", asset_type: "prop" });
      setVideoURL(asset.url);
      setVideoName(file.name);
      setMode("upload");
      await submit(asset.url);
    } catch (err) {
      setError(err instanceof Error ? err.message : ts("视频上传失败"));
    } finally {
      setUploading(false);
    }
  };

  const loadHistory = async () => {
    setHistoryOpen(true);
    try {
      const result = await api<{ items: HistoryItem[] }>(`/api/agent-projects?workflow_code=${encodeURIComponent(workflow.code)}&page=1&page_size=100`);
      setHistory(result.items || []);
    } catch (err) {
      setError(err instanceof Error ? err.message : ts("历史记录加载失败"));
    }
  };

  const openHistory = async (id: string) => {
    try {
      const saved = await api<Project>(`/api/agent-projects/${id}`);
      setProject(saved);
      setHistoryOpen(false);
      setRewriteView("");
      setResultView(saved.outputs?.production_markdown ? "production" : "source");
      window.localStorage.setItem(activeKey, saved.public_id);
      if (["pending", "running"].includes(saved.status)) startPolling(saved.public_id);
    } catch (err) {
      setError(err instanceof Error ? err.message : ts("历史记录加载失败"));
    }
  };

  const rewrite = async (kind: string) => {
    if (!project || !complete) return;
    setError("");
    setRewriting(kind);
    setResultView("source");
    try {
      const next = await api<Project>(`/api/agent-projects/${project.public_id}/breakdown-rewrite`, {
        method: "POST",
        body: JSON.stringify({ kind }),
      });
      setProject(next);
      setRewriteView(kind);
      setRewriting("");
    } catch (err) {
      setRewriting("");
      setError(err instanceof Error ? err.message : ts("改写失败，原稿未改动"));
    }
  };

  const retryIncomplete = async () => {
    if (!project || busy) return;
    setRetrying(true);
    setError("");
    setRewriteView("");
    setResultView("source");
    try {
      if (project.status === "failed") {
        await api(`/api/agent-projects/${project.public_id}/retry`, { method: "POST" });
        setProject({ ...project, status: "pending", error_message: undefined });
        startPolling(project.public_id);
        return;
      }
      const savedInputs = project.inputs || {};
      const inputs: { share_text?: string; video_url?: string } = savedInputs.share_text
        ? { share_text: savedInputs.share_text }
        : { video_url: savedInputs.video_url || savedInputs.source_url };
      if (!inputs.share_text && !inputs.video_url) throw new Error(ts("原视频地址已丢失，请重新上传"));
      const next = await api<Project>(`/api/agents/${workflow.code}/projects`, {
        method: "POST",
        body: JSON.stringify({ inputs }),
      });
      setProject(next);
      window.localStorage.setItem(activeKey, next.public_id);
      startPolling(next.public_id);
    } catch (err) {
      setError(err instanceof Error ? err.message : ts("重试失败"));
    } finally {
      setRetrying(false);
    }
  };

  const copyMarkdown = async () => {
    if (!markdown) return;
    await navigator.clipboard.writeText(markdown);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1200);
  };

  const reset = () => {
    pollRef.current?.();
    setProject(null);
    setError("");
    setRewriting("");
    setRewriteView("");
    setResultView("source");
    setShareText("");
    setVideoURL("");
    setVideoName("");
    window.localStorage.removeItem(activeKey);
  };

  const percent = Math.max(0, Math.min(100, Number(outputs.progress_percent || 0)));
  const message = busy ? outputs.progress_message || ts("正在解析视频来源...") : project?.status === "failed" ? project.error_message || outputs.progress_message || "" : complete ? ts("完整拆解与可生成镜头包已生成") : degraded ? ts("拆解未完整，可从检查点继续重试") : "";

  return (
    <div className="relative flex min-h-0 flex-1 flex-col overflow-hidden bg-[#07110e] text-emerald-50">
      <header className="border-b border-white/10 px-4 py-3 lg:pr-72">
        <div className="flex items-start justify-between gap-4">
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <div className="truncate text-sm font-semibold">{workflow.name}</div>
              {complete && <span className="inline-flex shrink-0 items-center gap-1 rounded-full bg-emerald-400/15 px-2 py-0.5 text-[10px] text-emerald-200"><CheckCircle2 size={11} />{ts("完整")}</span>}
              {degraded && <span className="inline-flex shrink-0 items-center gap-1 rounded-full bg-amber-400/15 px-2 py-0.5 text-[10px] text-amber-200"><AlertTriangle size={11} />{ts("未完整")}</span>}
            </div>
            <div className="truncate text-xs text-emerald-100/50">{workflow.description}</div>
          </div>
          <div className="flex shrink-0 items-center gap-2">
            <button type="button" onClick={() => void loadHistory()} className="inline-flex items-center gap-1 rounded-full border border-white/10 px-3 py-1.5 text-xs"><History size={14} />{ts("历史")}</button>
            <button type="button" onClick={reset} className="rounded-full border border-white/10 px-3 py-1.5 text-xs">{ts("清空")}</button>
          </div>
        </div>
        <div className="mt-3">
          <div className="mb-1 truncate text-xs text-emerald-200/80">{message || ts("等待分析")}</div>
          <div className="h-1.5 overflow-hidden rounded-full bg-white/10">
            <div className="h-full rounded-full bg-emerald-400 transition-all" style={{ width: `${project ? percent : 0}%` }} />
          </div>
        </div>
      </header>

      <div className="grid min-h-0 flex-1 grid-cols-1 lg:grid-cols-[280px_minmax(0,1fr)_280px]">
        <aside className="flex min-h-0 flex-col gap-3 border-white/10 p-4 lg:border-r">
          <div className="text-xs text-emerald-100/60">{rate > 0 ? `${rate} ${ts("算力/秒")}` : ts("按视频秒数计费")}</div>
          <div className="grid grid-cols-2 gap-2">
            <button type="button" onClick={() => setMode("link")} className={`rounded-xl px-3 py-2 text-xs ${mode === "link" ? "bg-emerald-400 text-emerald-950" : "bg-white/5"}`}>{ts("抖音链接")}</button>
            <button type="button" onClick={() => setMode("upload")} className={`rounded-xl px-3 py-2 text-xs ${mode === "upload" ? "bg-emerald-400 text-emerald-950" : "bg-white/5"}`}>{ts("本地上传")}</button>
          </div>
          {mode === "link" ? (
            <>
              <textarea value={shareText} onChange={(event) => setShareText(event.target.value)} placeholder={ts("粘贴抖音分享文本或视频链接")} className="min-h-28 rounded-xl border border-white/10 bg-black/20 px-3 py-2 text-sm outline-none" />
              <div className="flex gap-2">
                <button type="button" disabled={busy} onClick={() => void submit()} className="flex-1 rounded-xl bg-emerald-400 px-3 py-2 text-sm font-semibold text-emerald-950 disabled:opacity-50">{busy ? ts("处理中...") : ts("开始分析")}</button>
                <button type="button" onClick={reset} className="rounded-xl border border-white/10 px-3 py-2 text-sm">{ts("清空")}</button>
              </div>
            </>
          ) : (
            <>
              <button type="button" disabled={busy || uploading} onClick={() => fileRef.current?.click()} className="flex items-center justify-center gap-2 rounded-xl border border-dashed border-white/15 px-3 py-8 text-sm disabled:opacity-50"><Upload size={16} />{uploading ? ts("正在上传") : videoName || ts("选择视频")}</button>
              <input ref={fileRef} type="file" accept="video/*" hidden onChange={(event) => void onFile(event.target.files?.[0])} />
            </>
          )}
          {error && <p className="text-xs leading-5 text-red-300">{error}</p>}
          {degraded && project && (
            <div className="rounded-xl border border-amber-300/20 bg-amber-300/10 p-3 text-xs text-amber-100">
              <p className="leading-5">{project.error_message || operationalNotes[operationalNotes.length - 1] || ts("存在未完成片段，当前内容仅作预览。")}</p>
              <button type="button" disabled={busy} onClick={() => void retryIncomplete()} className="mt-2 inline-flex w-full items-center justify-center gap-1 rounded-lg bg-amber-300 px-3 py-2 font-semibold text-amber-950 disabled:opacity-50">
                {retrying ? <Loader2 size={13} className="animate-spin" /> : <RefreshCw size={13} />}
                {project.status === "failed" ? ts("仅重试失败片段") : ts("重新完整分析")}
              </button>
            </div>
          )}
          <div className="mt-auto grid grid-cols-5 gap-1">
            {STEPS.map((step, index) => {
              const state = outputs.stages?.[step.id] || "pending";
              return <div key={step.id} className={`rounded-lg px-1 py-2 text-center text-[10px] ${state === "done" ? "bg-emerald-400/20 text-emerald-200" : state === "running" ? "bg-emerald-400 text-emerald-950" : "bg-white/5 text-emerald-100/40"}`}>{index + 1} {step.label}</div>;
            })}
          </div>
        </aside>

        <section className="flex min-h-0 flex-col border-white/10 p-4 lg:border-r">
          <div className="mb-3 flex items-center justify-between gap-2">
            <div className="flex items-center gap-2">
              <button type="button" onClick={() => setResultView("source")} className={`rounded-lg px-2.5 py-1 text-xs ${resultView === "source" ? "bg-emerald-400 text-emerald-950" : "bg-white/5 text-emerald-100/70"}`}>{ts("原片剧本")}</button>
              <button type="button" disabled={!outputs.production_markdown} onClick={() => setResultView("production")} className={`rounded-lg px-2.5 py-1 text-xs disabled:opacity-30 ${resultView === "production" ? "bg-emerald-400 text-emerald-950" : "bg-white/5 text-emerald-100/70"}`}>{ts("视频生成包")}</button>
            </div>
            <button type="button" onClick={() => void copyMarkdown()} disabled={!markdown} className="inline-flex items-center gap-1 rounded-lg border border-white/10 px-2 py-1 text-xs disabled:opacity-40"><Copy size={12} />{copied ? ts("已复制") : ts("复制 Markdown")}</button>
          </div>
          {(outputs.quality || outputs.segment_checkpoint_summary) && (
            <div className={`mb-3 grid grid-cols-2 gap-2 rounded-xl border px-3 py-2 text-[11px] sm:grid-cols-4 ${degraded ? "border-amber-300/20 bg-amber-300/10 text-amber-100" : "border-emerald-300/15 bg-emerald-300/5 text-emerald-100/70"}`}>
              <span>{ts("状态")}：{degraded ? ts("未通过") : ts("已通过")}</span>
              <span>{ts("时间线覆盖")}：{Number(outputs.quality?.coverage_percent || 0).toFixed(1)}%</span>
              <span>{ts("视频片段")}：{outputs.quality?.segment_done ?? outputs.segment_checkpoint_summary?.done ?? 0}/{outputs.quality?.segment_total ?? outputs.segment_checkpoint_summary?.total ?? 0}</span>
              <span>{ts("版本")}：{outputs.worker_version || "-"}</span>
            </div>
          )}
          <div className="min-h-0 flex-1 overflow-y-auto rounded-2xl border border-white/10 bg-black/20 p-4 text-sm leading-6 whitespace-pre-wrap">
            {markdown || <span className="text-emerald-100/40">{ts("完整分析后生成可编辑剧本与视频生成包。")}</span>}
          </div>
          {complete && resultView === "source" && (
            <div className="mt-3 flex flex-wrap gap-2">
              {REWRITES.map((item) => (
                <button key={item.id} type="button" disabled={Boolean(rewriting)} onClick={() => void rewrite(item.id)} className={`rounded-full border px-3 py-1.5 text-xs ${rewriteView === item.id ? "border-emerald-300 bg-emerald-400 text-emerald-950" : "border-white/10"}`}>
                  {rewriting === item.id ? ts("改写中") : ts(item.label)}
                </button>
              ))}
              {rewriteView && <button type="button" onClick={() => setRewriteView("")} className="rounded-full border border-white/10 px-3 py-1.5 text-xs">{ts("查看原稿")}</button>}
            </div>
          )}
        </section>

        <aside className="flex min-h-0 flex-col p-4">
          <div className="mb-3 flex items-center justify-between text-sm">
            <span className="font-semibold">{ts("关键帧参考")}</span>
            {keyframes.length > 0 && <span className="rounded-full bg-white/10 px-2 py-0.5 text-[10px]">{keyframes.length}{ts("帧")}</span>}
          </div>
          <div className="min-h-0 flex-1 space-y-3 overflow-y-auto">
            {keyframes.length === 0 ? <p className="pt-16 text-center text-xs text-emerald-100/40">{ts("分析完成后，关键帧将显示在这里。")}</p> : keyframes.map((frame) => (
              <figure key={`${frame.url}-${frame.t_ms}`} className="overflow-hidden rounded-xl border border-white/10">
                {frame.url && <img src={frame.url} alt={frame.label || ""} className="aspect-video w-full object-cover" />}
                <figcaption className="px-2 py-1 text-[10px] text-emerald-100/70">{frame.label}</figcaption>
              </figure>
            ))}
          </div>
          {busy && <div className="mt-3 flex items-center gap-2 text-xs text-emerald-100/70"><Loader2 size={14} className="animate-spin" />{outputs.progress_message}</div>}
        </aside>
      </div>

      {historyOpen && (
        <div className="absolute inset-0 z-20 flex justify-end bg-black/40" onClick={() => setHistoryOpen(false)}>
          <div className="h-full w-full max-w-sm overflow-y-auto bg-[#0c1713] p-4" onClick={(event) => event.stopPropagation()}>
            <div className="mb-3 text-sm font-semibold">{ts("历史记录")}</div>
            {history.length === 0 ? <p className="text-xs text-emerald-100/40">{ts("还没有记录")}</p> : history.map((item) => (
              <button key={item.public_id} type="button" onClick={() => void openHistory(item.public_id)} className="mb-2 block w-full rounded-xl border border-white/10 px-3 py-2 text-left text-xs">
                <div>{item.created_at}</div>
                <div className="text-emerald-100/50">{item.status}</div>
              </button>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

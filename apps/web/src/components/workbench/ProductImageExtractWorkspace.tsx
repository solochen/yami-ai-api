"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { Download, History, Image as ImageIcon, Loader2, RefreshCw } from "lucide-react";
import { api } from "@/lib/api";
import { pollAsync } from "@/lib/pollAsync";
import { useI18n } from "@/i18n/I18nProvider";
import { AgentLanding } from "./AgentLanding";
import { AGENT_THEMES } from "./categoryMeta";

type WorkflowLike = {
  code: string;
  name: string;
  description?: string;
  icon?: string;
  display_config?: {
    theme?: string;
    hero_tags?: string[];
    steps?: Array<{ icon?: string; title: string; subtitle?: string; tags?: string[] }>;
  };
  price_rule?: { billing_type?: string; unit_price?: number };
};

type ExtractedImage = {
  role?: string;
  spec?: string;
  index?: number;
  asset_url?: string;
  name?: string;
};

type Project = {
  public_id: string;
  status: string;
  error_message?: string;
  inputs?: { product_url?: string };
  outputs?: {
    title?: string;
    sku_id?: string;
    source_url?: string;
    current_step?: string;
    zip_url?: string;
    image_count?: number;
    sale_specs?: Array<{ name?: string; values?: string[] }>;
    attributes?: Array<{ name?: string; value?: string }>;
    images?: ExtractedImage[];
    progress?: { done?: number; total?: number };
  };
};

type HistoryItem = { public_id: string; status: string; created_at: string };

export function ProductImageExtractWorkspace({ workflow }: { workflow: WorkflowLike }) {
  const { t, td, ts } = useI18n();
  const [url, setUrl] = useState("");
  const [project, setProject] = useState<Project | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");
  const [historyOpen, setHistoryOpen] = useState(false);
  const [history, setHistory] = useState<HistoryItem[]>([]);
  const [activeFeature, setActiveFeature] = useState(0);
  const [preview, setPreview] = useState<ExtractedImage | null>(null);
  const pollRef = useRef<(() => void) | null>(null);
  const pollScopeRef = useRef<string | null>(null);
  const activeKey = `product_extract_active:${workflow.code}`;
  const display = workflow.display_config || {};
  const theme = AGENT_THEMES[display.theme || ""] || AGENT_THEMES.amber;
  const features = useMemo(() => (display.steps || []).map((item, index) => ({
    ...item,
    title: td(`agent.${workflow.code}.step.${index}.title`, item.title),
    subtitle: item.subtitle ? td(`agent.${workflow.code}.step.${index}.subtitle`, item.subtitle) : "",
    tags: item.tags?.map((tag) => td(`agent.${workflow.code}.step.${index}.tag.${tag}`, tag)),
  })), [display.steps, td, workflow.code]);
  const outputs = project?.outputs || {};
  const images = outputs.images || [];
  const gallery = images.filter((item) => item.role !== "detail" && item.asset_url);
  const details = images.filter((item) => item.role === "detail" && item.asset_url);
  const busy = project?.status === "pending" || project?.status === "running";
  const workflowName = td(`agent.${workflow.code}.name`, workflow.name);
  const workflowDescription = td(`agent.${workflow.code}.description`, workflow.description || "");
  const extractPrice = Number(workflow.price_rule?.unit_price || 0);

  useEffect(() => {
    if (!preview) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") setPreview(null);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [preview]);

  const startPolling = (id: string) => {
    if (pollScopeRef.current !== workflow.code) return;
    pollRef.current?.();
    pollRef.current = pollAsync(async (signal) => {
      const next = await api<Project>(`/api/agent-projects/${id}`, { signal });
      if (signal.aborted) return;
      setProject(next);
      if (["succeeded", "failed", "canceled"].includes(next.status)) pollRef.current?.();
    }, 1600);
  };

  useEffect(() => {
    pollScopeRef.current = workflow.code;
    return () => {
      pollScopeRef.current = null;
      pollRef.current?.();
    };
  }, [workflow.code]);

  useEffect(() => {
    if (project || features.length < 2) return;
    const timer = window.setInterval(() => {
      if (!document.hidden) setActiveFeature((value) => (value + 1) % features.length);
    }, 3600);
    return () => window.clearInterval(timer);
  }, [features.length, project]);

  useEffect(() => {
    const id = window.localStorage.getItem(activeKey);
    if (!id) return;
    api<Project>(`/api/agent-projects/${id}`)
      .then((saved) => {
        setProject(saved);
        setUrl(saved.inputs?.product_url || saved.outputs?.source_url || "");
        if (["pending", "running"].includes(saved.status)) startPolling(saved.public_id);
      })
      .catch(() => window.localStorage.removeItem(activeKey));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeKey]);

  const submit = async () => {
    const productURL = url.trim();
    if (!productURL) {
      setError(ts("请输入京东商品链接"));
      return;
    }
    setSubmitting(true);
    setError("");
    try {
      const next = await api<Project>(`/api/agents/${workflow.code}/projects`, {
        method: "POST",
        body: JSON.stringify({ inputs: { product_url: productURL } }),
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

  const loadHistory = async () => {
    setHistoryOpen(true);
    try {
      const result = await api<{ items: HistoryItem[] }>(`/api/agent-projects?workflow_code=${encodeURIComponent(workflow.code)}&page=1&page_size=20`);
      setHistory(result.items || []);
    } catch (err) {
      setError(err instanceof Error ? err.message : ts("历史记录加载失败"));
    }
  };

  const progressText = busy
    ? outputs.current_step === "downloading"
      ? ts("正在保存图片")
      : ts("正在读取商品页")
    : "";

  return (
    <div className="relative flex min-h-0 flex-1 flex-col overflow-hidden bg-[#fff8ef] text-gray-900 dark:bg-[#120d08] dark:text-white">
      <div className="flex items-center justify-between gap-3 px-4 py-3 sm:px-6 lg:pr-56">
        <div className="min-w-0">
          <div className="truncate text-sm font-semibold">{workflowName}</div>
          <div className="truncate text-xs text-gray-500">{workflowDescription}</div>
        </div>
        <div className="flex shrink-0 gap-2">
          <button type="button" onClick={() => { setProject(null); setError(""); }} className="rounded-xl border border-gray-200 bg-white px-3 py-2 text-xs font-semibold dark:border-white/10 dark:bg-white/5">{ts("新任务")}</button>
          <button type="button" onClick={() => void loadHistory()} className="inline-flex items-center gap-1 rounded-xl border border-gray-200 bg-white px-3 py-2 text-xs font-semibold dark:border-white/10 dark:bg-white/5"><History size={14} />{ts("历史")}</button>
        </div>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto px-4 pb-28 sm:px-6">
        {!project ? (
          <AgentLanding
            workflowIcon={workflow.icon || "🛍️"}
            workflowName={workflowName}
            workflowDescription={workflowDescription}
            heroTags={(display.hero_tags || []).map((tag) => td(`agent.${workflow.code}.tag.${tag}`, tag))}
            features={features}
            activeIndex={activeFeature}
            onSelect={setActiveFeature}
            theme={theme}
            generationType="image"
          />
        ) : (
          <div className="mx-auto flex w-full max-w-5xl flex-col gap-5 py-2">
            {busy && (
              <div className="flex items-center gap-3 rounded-2xl border border-amber-200 bg-white/80 px-4 py-3 text-sm dark:border-amber-400/20 dark:bg-white/5">
                <Loader2 size={16} className="animate-spin text-amber-500" />
                <span>{progressText}</span>
                {outputs.progress?.total ? <span className="text-gray-400">{outputs.progress.done || 0}/{outputs.progress.total}</span> : null}
              </div>
            )}
            {project.status === "failed" && <p className="rounded-2xl bg-red-50 px-4 py-3 text-sm text-red-600 dark:bg-red-500/10 dark:text-red-300">{project.error_message || ts("提取失败")}</p>}
            {project.status === "succeeded" && (
              <>
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div>
                    <h2 className="text-lg font-bold">{outputs.title || ts("商品图片")}</h2>
                    <p className="mt-1 text-xs text-gray-500">{ts("商品编号")} {outputs.sku_id} · {ts("已保存")} {outputs.image_count || images.length} {ts("张到资产库")}</p>
                  </div>
                  {outputs.zip_url && <a href={outputs.zip_url} className="inline-flex items-center gap-1 rounded-xl bg-gray-900 px-3 py-2 text-xs font-semibold text-white dark:bg-white dark:text-gray-900"><Download size={14} />{ts("打包下载")}</a>}
                </div>
                <SpecBlock title={ts("产品规格")} specs={outputs.sale_specs || []} attributes={outputs.attributes || []} emptyText={ts("这个商品没有读到规格参数")} />
                <ImageBlock title={ts("产品图片")} images={gallery} emptyText={ts("没有读到主图")} onPreview={setPreview} />
                <ImageBlock title={ts("详情图片")} images={details} emptyText={ts("没有读到详情图")} onPreview={setPreview} />
              </>
            )}
          </div>
        )}
      </div>

      <form
        onSubmit={(event) => { event.preventDefault(); void submit(); }}
        className="absolute inset-x-0 bottom-0 border-t border-amber-100 bg-white/90 p-3 backdrop-blur dark:border-white/10 dark:bg-[#120d08]/90 sm:p-4"
      >
        <div className="mx-auto flex max-w-5xl items-center gap-2">
          <input
            value={url}
            onChange={(event) => setUrl(event.target.value)}
            placeholder="https://item.jd.com/10041037225218.html"
            className="h-11 min-w-0 flex-1 rounded-2xl border border-gray-200 bg-white px-4 text-sm outline-none focus:border-amber-400 dark:border-white/10 dark:bg-white/5"
          />
          <button type="submit" disabled={submitting || busy} className="inline-flex h-11 items-center gap-1 rounded-2xl bg-amber-500 px-4 text-sm font-semibold text-white disabled:opacity-50">
            {submitting || busy ? <Loader2 size={16} className="animate-spin" /> : <ImageIcon size={16} />}
            {busy ? ts("提取中") : extractPrice > 0 ? `${ts("开始提取")} · ${extractPrice} ${ts("算力")}` : ts("开始提取")}
          </button>
          {project?.status === "failed" && (
            <button type="button" onClick={() => void submit()} className="inline-flex h-11 items-center gap-1 rounded-2xl border border-gray-200 px-3 text-sm dark:border-white/10"><RefreshCw size={14} />{ts("重试")}</button>
          )}
        </div>
        {error && <p className="mx-auto mt-2 max-w-5xl text-xs text-red-500">{error}</p>}
      </form>

      {preview?.asset_url && (
        <div className="absolute inset-0 z-30 flex items-center justify-center bg-black/80 p-4" onClick={() => setPreview(null)} role="dialog" aria-modal="true">
          <button type="button" onClick={() => setPreview(null)} className="absolute right-4 top-4 rounded-full bg-white/15 px-3 py-1 text-sm text-white">{ts("关闭")}</button>
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img src={preview.asset_url} alt={preview.name || ts("商品图片")} className="max-h-full max-w-full object-contain" onClick={(event) => event.stopPropagation()} />
        </div>
      )}

      {historyOpen && (
        <div className="absolute inset-0 z-20 flex justify-end bg-black/20" onClick={() => setHistoryOpen(false)}>
          <div className="h-full w-full max-w-sm overflow-y-auto bg-white p-4 dark:bg-[#1b140e]" onClick={(event) => event.stopPropagation()}>
            <div className="mb-3 text-sm font-semibold">{ts("历史记录")}</div>
            <div className="space-y-2">
              {history.map((item) => (
                <button
                  key={item.public_id}
                  type="button"
                  onClick={async () => {
                    const next = await api<Project>(`/api/agent-projects/${item.public_id}`);
                    setProject(next);
                    setUrl(next.inputs?.product_url || next.outputs?.source_url || "");
                    window.localStorage.setItem(activeKey, next.public_id);
                    if (["pending", "running"].includes(next.status)) startPolling(next.public_id);
                    setHistoryOpen(false);
                  }}
                  className="flex w-full items-center justify-between rounded-xl border border-gray-100 px-3 py-3 text-left text-xs dark:border-white/10"
                >
                  <span>{new Date(item.created_at).toLocaleString()}</span>
                  <span>{t(statusKey(item.status))}</span>
                </button>
              ))}
              {history.length === 0 && <p className="text-xs text-gray-400">{ts("还没有提取记录")}</p>}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function statusKey(status: string) {
  if (status === "succeeded") return "status.succeeded";
  if (status === "failed") return "status.failed";
  if (status === "running") return "status.running";
  return "status.pending";
}

function specText(specs: Array<{ name?: string; values?: string[] }>, attributes: Array<{ name?: string; value?: string }>) {
  const lines: string[] = [];
  for (const spec of specs) {
    const values = (spec.values || []).filter(Boolean);
    if (spec.name && values.length) lines.push(`${spec.name}：${values.join("、")}`);
  }
  for (const item of attributes) {
    if (item.name && item.value) lines.push(`${item.name}：${item.value}`);
  }
  return lines.join("\n");
}

function SpecBlock({ title, specs, attributes, emptyText }: { title: string; specs: Array<{ name?: string; values?: string[] }>; attributes: Array<{ name?: string; value?: string }>; emptyText: string }) {
  const { ts } = useI18n();
  const [copied, setCopied] = useState(false);
  const text = specText(specs, attributes);
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1500);
    } catch {
      setCopied(false);
    }
  };
  return (
    <section className="rounded-3xl border border-amber-100 bg-white/80 p-4 dark:border-white/10 dark:bg-white/5">
      <div className="mb-3 flex items-center justify-between gap-3">
        <h3 className="text-sm font-bold">{title}</h3>
        {text && <button type="button" onClick={() => void copy()} className="rounded-lg bg-amber-500/10 px-2.5 py-1 text-xs font-semibold text-amber-700 dark:text-amber-200">{copied ? ts("已复制") : ts("复制")}</button>}
      </div>
      {text ? (
        <textarea
          readOnly
          value={text}
          onFocus={(event) => event.currentTarget.select()}
          rows={Math.min(12, Math.max(4, text.split("\n").length))}
          className="w-full resize-y rounded-2xl border border-gray-200 bg-gray-50 px-3 py-2 text-sm leading-6 text-gray-800 outline-none dark:border-white/10 dark:bg-black/20 dark:text-gray-100"
        />
      ) : <p className="text-xs text-gray-400">{emptyText}</p>}
    </section>
  );
}

function ImageBlock({ title, images, emptyText, onPreview }: { title: string; images: ExtractedImage[]; emptyText: string; onPreview: (image: ExtractedImage) => void }) {
  const { ts } = useI18n();
  return (
    <section>
      <h3 className="mb-3 text-sm font-bold">{title}</h3>
      {images.length === 0 ? <p className="text-xs text-gray-400">{emptyText}</p> : (
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
          {images.map((image) => (
            <figure key={`${image.role}-${image.index}-${image.asset_url}`} className="overflow-hidden rounded-2xl border border-gray-100 bg-white dark:border-white/10 dark:bg-white/5">
              <button type="button" onClick={() => onPreview(image)} className="block w-full" aria-label={ts("预览大图")}>
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img src={image.asset_url} alt={image.name || title} className="aspect-square w-full object-contain bg-white" />
              </button>
              <figcaption className="flex items-center justify-between gap-2 px-2 py-2 text-[11px] text-gray-500">
                <span className="truncate">{image.spec || image.name}</span>
                <a href={image.asset_url} target="_blank" rel="noreferrer" className="shrink-0 text-amber-600">{ts("打开")}</a>
              </figcaption>
            </figure>
          ))}
        </div>
      )}
    </section>
  );
}

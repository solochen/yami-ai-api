"use client";

import { useCallback, useEffect, useState } from "react";
import { AlertTriangle, Ban, Check, Clock3, Eye, Image as ImageIcon, Loader2, Play, RefreshCw, RotateCcw, Sparkles, X } from "lucide-react";
import { adminApi } from "@/lib/api";

type BatchStatus = "draft" | "queued" | "running" | "completed" | "partial_failed" | "failed" | "cancelled";
type TaskStatus = "queued" | "submitting" | "processing" | "pending_review" | "succeeded" | "failed" | "cancelled" | "expired" | "unknown";

interface Batch {
  id: number;
  public_id: string;
  status: BatchStatus;
  spec_count: number;
  task_count: number;
  completed_count: number;
  failed_count: number;
  cancelled_count: number;
  active_count: number;
  max_concurrency: number;
  max_retries: number;
  budget_limit: number;
  estimated_cost: number;
  actual_cost: number;
  currency: string;
  cost_rule_version: string;
  error_message?: string;
  created_at: string;
  started_at?: string;
  finished_at?: string;
}

interface GenerationTask {
  id: number;
  public_id: string;
  spec_title: string;
  status: TaskStatus;
  attempt: number;
  model_code: string;
  provider_model: string;
  output_asset_url?: string;
  estimated_cost: number;
  actual_cost: number;
  provider_cost: number;
  retry_count: number;
  max_retries: number;
  error_code?: string;
  error_message?: string;
  created_at: string;
  finished_at?: string;
}

interface PageResult<T> {
  items: T[];
  total: number;
}

interface OccupiedSpec {
  spec_id: number;
  spec_public_id: string;
  spec_title: string;
  task_status: TaskStatus;
  batch_id: number;
  batch_public_id: string;
  batch_status: BatchStatus;
}

interface GenerationEligibility {
  ready_spec_count: number;
  available_spec_count: number;
  occupied_spec_count: number;
  occupied_specs: OccupiedSpec[];
}

const statusLabels: Record<BatchStatus | TaskStatus, string> = {
  draft: "草稿",
  queued: "排队中",
  running: "生成中",
  completed: "待审核",
  partial_failed: "部分失败",
  failed: "失败",
  cancelled: "已取消",
  submitting: "提交中",
  processing: "处理中",
  pending_review: "待审核",
  succeeded: "已完成",
  expired: "已过期",
  unknown: "未知",
};

function statusClass(status: BatchStatus | TaskStatus) {
  if (status === "completed" || status === "pending_review" || status === "succeeded") return "bg-emerald-50 text-emerald-700";
  if (status === "running" || status === "queued" || status === "processing" || status === "submitting" || status === "draft") return "bg-amber-50 text-amber-700";
  if (status === "failed" || status === "partial_failed" || status === "expired" || status === "unknown") return "bg-red-50 text-red-700";
  return "bg-gray-100 text-gray-600";
}

function dateValue(value?: string) {
  return value ? new Date(value).toLocaleString("zh-CN", { dateStyle: "short", timeStyle: "short" }) : "-";
}

function numberValue(value: number) {
  return Number(value || 0).toFixed(4);
}

export default function GalleryGenerationPage() {
  const [batches, setBatches] = useState<Batch[]>([]);
  const [status, setStatus] = useState("");
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [detail, setDetail] = useState<Batch | null>(null);
  const [tasks, setTasks] = useState<GenerationTask[]>([]);
  const [tasksLoading, setTasksLoading] = useState(false);
  const [eligibility, setEligibility] = useState<GenerationEligibility | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const query = status ? `?status=${encodeURIComponent(status)}` : "";
      const [result, eligibilityResult] = await Promise.all([
        adminApi<PageResult<Batch>>(`/gallery/generation-batches${query}`),
        adminApi<GenerationEligibility>("/gallery/generation-batches/eligibility"),
      ]);
      setBatches(result.items || []);
      setEligibility(eligibilityResult);
    } catch (err) {
      setError(err instanceof Error ? err.message : "生成批次加载失败");
    } finally {
      setLoading(false);
    }
  }, [status]);

  useEffect(() => { void load(); }, [load]);

  const openDetail = async (batch: Batch) => {
    setDetail(batch);
    setTasksLoading(true);
    try {
      const [full, taskResult] = await Promise.all([
        adminApi<Batch>(`/gallery/generation-batches/${batch.id}`),
        adminApi<PageResult<GenerationTask>>(`/gallery/generation-batches/${batch.id}/tasks?page_size=100`),
      ]);
      setDetail(full);
      setTasks(taskResult.items || []);
    } catch (err) {
      setError(err instanceof Error ? err.message : "生成批次详情加载失败");
    } finally {
      setTasksLoading(false);
    }
  };

  const createBatch = async () => {
    if (!eligibility?.available_spec_count) {
      setError(eligibility?.occupied_spec_count
        ? "当前 ready 规格都已有活跃任务，请继续处理现有批次，不要重复创建。"
        : "当前没有可创建任务的 ready 规格，请先到案例规格工作台完成审核。");
      return;
    }
    setBusy(true);
    setError("");
    setNotice("");
    try {
      const created = await adminApi<Batch>("/gallery/generation-batches", { method: "POST", body: JSON.stringify({}) });
      setNotice(`批次 ${created.public_id} 已创建，共 ${created.spec_count} 条规格，请确认后启动`);
      await load();
      await openDetail(created);
    } catch (err) {
      setError(err instanceof Error ? err.message : "创建批次失败");
      await load();
    } finally {
      setBusy(false);
    }
  };

  const batchAction = async (batch: Batch, action: "start" | "cancel") => {
    const confirmed = window.confirm(action === "start"
      ? `确认启动批次 ${batch.public_id}？\n\n预计消耗 ${numberValue(batch.estimated_cost)} 算力，启动后将调用上游图片模型。`
      : `确认取消批次 ${batch.public_id}？\n\n未完成的生成任务将不会继续执行。`);
    if (!confirmed) return;
    setBusy(true);
    setError("");
    try {
      const updated = await adminApi<Batch>(`/gallery/generation-batches/${batch.id}/${action}`, { method: "POST" });
      setNotice(action === "start" ? "批次已进入队列，Worker 将按并发上限处理" : "批次已取消，未完成任务不会继续生成");
      await load();
      await openDetail(updated);
    } catch (err) {
      setError(err instanceof Error ? err.message : "批次操作失败");
    } finally {
      setBusy(false);
    }
  };

  const retryTask = async (task: GenerationTask) => {
    setBusy(true);
    setError("");
    try {
      await adminApi(`/gallery/generation-tasks/${task.id}/retry`, { method: "POST" });
      setNotice(`任务“${task.spec_title}”已重新排队`);
      if (detail) await openDetail(detail);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "任务重试失败");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="w-full space-y-6">
      <header className="flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between">
        <div>
          <div className="mb-2 flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.18em] text-gray-400"><Sparkles size={14} />Inspiration Gallery / Stage 6</div>
          <h1 className="text-2xl font-bold tracking-tight text-gray-950">批量媒体生成</h1>
          <p className="mt-1 max-w-3xl text-sm leading-6 text-gray-500">从已审核的 ready 规格创建可追踪的生成批次。成功图片只会进入待审核，不会自动写入灵感广场或公开发布。</p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <select aria-label="批次状态" value={status} onChange={(event) => setStatus(event.target.value)} className="rounded-xl border border-gray-200 bg-white px-3 py-2.5 text-sm text-gray-700"><option value="">全部状态</option>{(["draft", "queued", "running", "completed", "partial_failed", "failed", "cancelled"] as BatchStatus[]).map((value) => <option key={value} value={value}>{statusLabels[value]}</option>)}</select>
          <button type="button" onClick={() => void load()} className="inline-flex items-center gap-2 rounded-xl border border-gray-200 bg-white px-4 py-2.5 text-sm font-medium text-gray-700 hover:bg-gray-50" title="刷新批次"><RefreshCw size={16} />刷新</button>
          <button type="button" onClick={() => void createBatch()} disabled={busy || !eligibility?.available_spec_count} className="inline-flex items-center gap-2 rounded-xl bg-gray-950 px-4 py-2.5 text-sm font-semibold text-white shadow-sm hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-50" title={eligibility?.available_spec_count ? `为 ${eligibility.available_spec_count} 条可用规格创建批次` : "没有可创建任务的规格"}><Play size={16} />创建批次{eligibility ? ` (${eligibility.available_spec_count})` : ""}</button>
        </div>
      </header>

      <section className="flex items-start gap-3 rounded-2xl bg-amber-50 px-5 py-4 text-sm text-amber-800"><Clock3 size={18} className="mt-0.5 shrink-0" /><div><div className="font-semibold">生产保护已启用</div><div className="mt-1 leading-6">创建时服务端会重新读取 ready 规格、排除已有活跃任务的规格、校验模型线路并锁定预估成本。{eligibility && <> 当前 ready {eligibility.ready_spec_count} 条，可创建 {eligibility.available_spec_count} 条，已占用 {eligibility.occupied_spec_count} 条。</>}</div>{eligibility && eligibility.occupied_spec_count > 0 && <div className="mt-1 text-xs leading-5 text-amber-700">已占用规格正在批次 {Array.from(new Set(eligibility.occupied_specs.map((item) => item.batch_public_id))).join("、")} 中排队、处理或等待审核；请在现有批次和质量审核中继续处理。</div>}</div></section>
      {error && <div role="alert" className="flex items-start gap-3 rounded-xl border border-red-100 bg-red-50 px-4 py-3 text-sm text-red-700"><AlertTriangle size={17} className="mt-0.5 shrink-0" /><span>{error}</span><button type="button" onClick={() => setError("")} className="ml-auto" title="关闭提示"><X size={16} /></button></div>}
      {notice && <div role="status" className="flex items-start gap-3 rounded-xl border border-emerald-100 bg-emerald-50 px-4 py-3 text-sm text-emerald-700"><Check size={17} className="mt-0.5 shrink-0" /><span>{notice}</span><button type="button" onClick={() => setNotice("")} className="ml-auto" title="关闭提示"><X size={16} /></button></div>}

      {loading ? <div className="flex min-h-48 items-center justify-center rounded-2xl bg-white text-sm text-gray-400"><Loader2 className="mr-2 animate-spin" size={18} />加载批次</div> : batches.length === 0 ? <div className="flex min-h-56 flex-col items-center justify-center rounded-2xl bg-white text-center"><ImageIcon size={28} className="text-gray-300" /><div className="mt-3 text-sm font-semibold text-gray-700">暂无生成批次</div><div className="mt-1 text-xs text-gray-400">先在“案例规格”中完成校验，并将至少一条规格设为 ready。</div></div> : <section className="overflow-hidden rounded-2xl bg-white shadow-sm"><div className="overflow-x-auto"><table className="min-w-[980px] w-full text-left text-sm"><thead className="bg-gray-50 text-xs text-gray-400"><tr><th className="px-5 py-3 font-medium">批次</th><th className="px-5 py-3 font-medium">状态</th><th className="px-5 py-3 font-medium">进度</th><th className="px-5 py-3 font-medium">预估 / 实际算力</th><th className="px-5 py-3 font-medium">并发 / 重试</th><th className="px-5 py-3 font-medium">创建时间</th><th className="px-5 py-3 font-medium">操作</th></tr></thead><tbody className="divide-y divide-gray-100">{batches.map((batch) => <tr key={batch.id} className="hover:bg-gray-50/70"><td className="px-5 py-4"><button type="button" className="text-left" onClick={() => void openDetail(batch)}><div className="font-semibold text-gray-900">{batch.public_id}</div><div className="mt-1 text-xs text-gray-400">{batch.spec_count} 条规格 · 规则 {batch.cost_rule_version}</div></button></td><td className="px-5 py-4"><span className={`rounded-full px-2 py-1 text-xs font-medium ${statusClass(batch.status)}`}>{statusLabels[batch.status]}</span></td><td className="px-5 py-4"><div className="font-medium text-gray-700">{batch.completed_count + batch.failed_count + batch.cancelled_count} / {batch.task_count}</div><div className="mt-1 text-xs text-gray-400">待审核 {batch.completed_count} · 失败 {batch.failed_count}</div></td><td className="px-5 py-4"><div className="font-mono text-xs text-gray-700">{numberValue(batch.estimated_cost)} / {numberValue(batch.actual_cost)}</div><div className="mt-1 text-xs text-gray-400">{batch.currency}</div></td><td className="px-5 py-4 text-xs text-gray-600">{batch.max_concurrency} / {batch.max_retries}</td><td className="px-5 py-4 text-xs text-gray-500">{dateValue(batch.created_at)}</td><td className="px-5 py-4"><div className="flex items-center gap-2"><button type="button" onClick={() => void openDetail(batch)} className="rounded-lg p-2 text-gray-500 hover:bg-gray-100 hover:text-gray-900" title="查看详情"><Eye size={16} /></button>{batch.status === "draft" && <button type="button" onClick={() => void batchAction(batch, "start")} disabled={busy} className="rounded-lg p-2 text-emerald-600 hover:bg-emerald-50" title="启动批次"><Play size={16} /></button>}{["draft", "queued", "running"].includes(batch.status) && <button type="button" onClick={() => void batchAction(batch, "cancel")} disabled={busy} className="rounded-lg p-2 text-red-500 hover:bg-red-50" title="取消批次"><Ban size={16} /></button>}</div></td></tr>)}</tbody></table></div></section>}

      {detail && <div className="fixed inset-0 z-50 flex items-end justify-center bg-gray-950/30 p-0 sm:items-center sm:p-6" role="dialog" aria-modal="true" aria-label="生成批次详情"><div className="max-h-[92vh] w-full overflow-y-auto rounded-t-2xl bg-white p-5 shadow-2xl sm:max-w-5xl sm:rounded-2xl"><div className="flex items-start justify-between gap-4"><div><div className="text-xs font-semibold uppercase tracking-[0.16em] text-gray-400">Generation Batch</div><h2 className="mt-1 text-xl font-bold text-gray-950">{detail.public_id}</h2><div className="mt-2 flex flex-wrap items-center gap-2"><span className={`rounded-full px-2 py-1 text-xs font-medium ${statusClass(detail.status)}`}>{statusLabels[detail.status]}</span><span className="text-xs text-gray-400">预算 {numberValue(detail.budget_limit)} · 预估 {numberValue(detail.estimated_cost)} · 实际 {numberValue(detail.actual_cost)}</span></div></div><button type="button" onClick={() => setDetail(null)} className="rounded-lg p-2 text-gray-500 hover:bg-gray-100" title="关闭详情"><X size={18} /></button></div><div className="mt-5 flex items-center justify-between rounded-xl bg-gray-50 px-4 py-3 text-xs text-gray-500"><span>完成 {detail.completed_count} · 失败 {detail.failed_count} · 取消 {detail.cancelled_count} · 活跃 {detail.active_count}</span><span>创建于 {dateValue(detail.created_at)}</span></div>{detail.error_message && <div className="mt-4 rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700">{detail.error_message}</div>}<div className="mt-5 overflow-x-auto">{tasksLoading ? <div className="py-12 text-center text-sm text-gray-400"><Loader2 className="mx-auto mb-2 animate-spin" size={20} />加载任务</div> : tasks.length === 0 ? <div className="py-12 text-center text-sm text-gray-400">暂无任务</div> : <table className="min-w-[900px] w-full text-left text-sm"><thead className="text-xs text-gray-400"><tr><th className="px-3 py-3 font-medium">规格</th><th className="px-3 py-3 font-medium">状态</th><th className="px-3 py-3 font-medium">模型</th><th className="px-3 py-3 font-medium">成本</th><th className="px-3 py-3 font-medium">结果</th><th className="px-3 py-3 font-medium">操作</th></tr></thead><tbody className="divide-y divide-gray-100">{tasks.map((task) => <tr key={task.id}><td className="max-w-[300px] px-3 py-3"><div className="font-medium text-gray-800">{task.spec_title}</div><div className="mt-1 text-xs text-gray-400">尝试 {task.attempt} · {task.public_id}</div></td><td className="px-3 py-3"><span className={`rounded-full px-2 py-1 text-xs font-medium ${statusClass(task.status)}`}>{statusLabels[task.status]}</span>{task.error_message && <div className="mt-1 max-w-[240px] text-xs text-red-500">{task.error_message}</div>}</td><td className="px-3 py-3 text-xs text-gray-600"><div>{task.model_code}</div><div className="mt-1 text-gray-400">{task.provider_model}</div></td><td className="px-3 py-3 font-mono text-xs text-gray-600">{numberValue(task.estimated_cost)} / {numberValue(task.actual_cost)}</td><td className="px-3 py-3">{task.output_asset_url ? <a href={task.output_asset_url} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1 text-xs text-gray-700 hover:underline"><ImageIcon size={14} />查看图片</a> : <span className="text-xs text-gray-400">-</span>}</td><td className="px-3 py-3">{["failed", "expired", "unknown"].includes(task.status) && <button type="button" onClick={() => void retryTask(task)} disabled={busy} className="inline-flex items-center gap-1 rounded-lg px-2 py-1.5 text-xs text-gray-600 hover:bg-gray-100" title="重试"><RotateCcw size={14} />重试</button>}</td></tr>)}</tbody></table>}</div><div className="mt-5 flex flex-wrap items-center justify-end gap-2"><button type="button" onClick={() => setDetail(null)} className="rounded-xl border border-gray-200 px-4 py-2.5 text-sm text-gray-700 hover:bg-gray-50">关闭</button>{["draft", "queued", "running"].includes(detail.status) && <button type="button" onClick={() => void batchAction(detail, "cancel")} disabled={busy} className="inline-flex items-center gap-2 rounded-xl border border-red-200 px-4 py-2.5 text-sm font-medium text-red-600 hover:bg-red-50 disabled:cursor-not-allowed disabled:opacity-50"><Ban size={16} />取消批次</button>}{detail.status === "draft" && <button type="button" onClick={() => void batchAction(detail, "start")} disabled={busy} className="inline-flex items-center gap-2 rounded-xl bg-emerald-600 px-4 py-2.5 text-sm font-semibold text-white hover:bg-emerald-700 disabled:cursor-not-allowed disabled:opacity-50"><Play size={16} />确认并启动</button>}</div></div></div>}
    </div>
  );
}

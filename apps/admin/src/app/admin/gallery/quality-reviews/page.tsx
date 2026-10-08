"use client";

import { useCallback, useEffect, useState } from "react";
import { AlertTriangle, Check, ClipboardCheck, Eye, Image as ImageIcon, Loader2, RefreshCw, Send, X } from "lucide-react";
import { adminApi } from "@/lib/api";

type ReviewStatus = "pending_review" | "auto_checking" | "auto_failed" | "awaiting_manual_review" | "approved" | "rejected" | "published";

interface Review {
  id: number;
  public_id: string;
  task_id: number;
  task_public_id: string;
  spec_id: number;
  spec_public_id: string;
  spec_title: string;
  prompt: string;
  category: string;
  styles: string[];
  scenes: string[];
  task_status: string;
  model_code: string;
  output_asset_url: string;
  status: ReviewStatus;
  review_version: number;
  asset_check: Record<string, unknown>;
  metadata_check: Record<string, unknown>;
  ocr_check: Record<string, unknown>;
  semantic_check: Record<string, unknown>;
  risk_check: Record<string, unknown>;
  duplicate_check: Record<string, unknown>;
  check_summary: { overall?: string; blockers?: string[]; warnings?: string[] };
  review_notes: string;
  rejection_reason: string;
  updated_at: string;
}

interface PageResult {
  items: Review[];
  total: number;
}

const statuses: Array<{ value: "" | ReviewStatus; label: string }> = [
  { value: "", label: "全部" },
  { value: "pending_review", label: "待自动检查" },
  { value: "auto_failed", label: "自动未通过" },
  { value: "awaiting_manual_review", label: "待人工审核" },
  { value: "approved", label: "已审核待发布" },
  { value: "published", label: "已发布" },
  { value: "rejected", label: "已拒绝" },
];

const statusLabels: Record<ReviewStatus, string> = {
  pending_review: "待自动检查",
  auto_checking: "自动检查中",
  auto_failed: "自动未通过",
  awaiting_manual_review: "待人工审核",
  approved: "已审核待发布",
  rejected: "已拒绝",
  published: "已发布",
};

function statusClass(status: ReviewStatus) {
  if (status === "published") return "bg-emerald-100 text-emerald-700";
  if (status === "approved" || status === "awaiting_manual_review") return "bg-blue-50 text-blue-700";
  if (status === "auto_failed" || status === "rejected") return "bg-red-50 text-red-700";
  return "bg-amber-50 text-amber-700";
}

function dateValue(value: string) {
  return value ? new Date(value).toLocaleString("zh-CN", { dateStyle: "short", timeStyle: "short" }) : "-";
}

function resultTone(value: unknown) {
  if (value === "pass" || value === "not_required") return "text-emerald-600";
  if (value === "fail") return "text-red-600";
  return "text-amber-600";
}

const checkStatusLabels: Record<string, string> = {
  pass: "通过",
  fail: "未通过",
  warning: "需留意",
  unavailable: "需人工核对",
  not_required: "无需检查",
  manual_required: "需人工审核",
};

export default function GalleryQualityReviewsPage() {
  const [items, setItems] = useState<Review[]>([]);
  const [status, setStatus] = useState<"" | ReviewStatus>("awaiting_manual_review");
  const [detail, setDetail] = useState<Review | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const query = status ? `?status=${encodeURIComponent(status)}&page_size=100` : "?page_size=100";
      const result = await adminApi<PageResult>(`/gallery/quality-reviews${query}`);
      setItems(result.items || []);
    } catch (err) {
      setError(err instanceof Error ? err.message : "质量审核列表加载失败");
    } finally {
      setLoading(false);
    }
  }, [status]);

  useEffect(() => { void load(); }, [load]);

  const openDetail = async (item: Review) => {
    try {
      setDetail(await adminApi<Review>(`/gallery/quality-reviews/${item.id}`));
    } catch (err) {
      setError(err instanceof Error ? err.message : "审核详情加载失败");
    }
  };

  const action = async (item: Review, type: "auto-check" | "approve" | "reject" | "publish") => {
    const rejectionReason = type === "reject" ? window.prompt("请输入拒绝原因：") : "";
    if (type === "reject" && !rejectionReason?.trim()) return;
    let body: Record<string, string> = {};
    if (type === "reject") body = { rejection_reason: (rejectionReason || "").trim() };
    setBusy(true);
    setError("");
    setNotice("");
    try {
      const updated = await adminApi<Review>(`/gallery/quality-reviews/${item.id}/${type}`, { method: "POST", body: JSON.stringify(body) });
      setDetail(updated);
      setNotice(type === "auto-check" ? "自动检查已完成" : type === "approve" ? "审核已通过，请确认是否发布" : type === "publish" ? "案例已发布到灵感广场" : "审核已拒绝");
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "审核操作失败");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="w-full space-y-6">
      <header className="flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between">
        <div>
          <div className="mb-2 flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.18em] text-gray-400"><ClipboardCheck size={14} />Inspiration Gallery / Stage 7</div>
          <h1 className="text-2xl font-bold tracking-tight text-gray-950">质量审核与发布</h1>
          <p className="mt-1 max-w-3xl text-sm leading-6 text-gray-500">生成结果必须经过资源、规格、重复和风险检查，再由管理员审核并明确发布。自动检查不会直接公开案例。</p>
        </div>
        <div className="flex items-center gap-2">
          <select aria-label="审核状态" value={status} onChange={(event) => setStatus(event.target.value as "" | ReviewStatus)} className="rounded-xl border border-gray-200 bg-white px-3 py-2.5 text-sm text-gray-700">
            {statuses.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}
          </select>
          <button type="button" onClick={() => void load()} className="inline-flex items-center gap-2 rounded-xl border border-gray-200 bg-white px-4 py-2.5 text-sm font-medium text-gray-700 hover:bg-gray-50" title="刷新审核列表"><RefreshCw size={16} />刷新</button>
        </div>
      </header>

      {error && <div role="alert" className="flex items-start gap-3 rounded-xl border border-red-100 bg-red-50 px-4 py-3 text-sm text-red-700"><AlertTriangle size={17} className="mt-0.5 shrink-0" /><span>{error}</span><button type="button" onClick={() => setError("")} className="ml-auto" title="关闭提示"><X size={16} /></button></div>}
      {notice && <div role="status" className="flex items-start gap-3 rounded-xl border border-emerald-100 bg-emerald-50 px-4 py-3 text-sm text-emerald-700"><Check size={17} className="mt-0.5 shrink-0" /><span>{notice}</span><button type="button" onClick={() => setNotice("")} className="ml-auto" title="关闭提示"><X size={16} /></button></div>}

      {loading ? <div className="flex min-h-48 items-center justify-center rounded-2xl bg-white text-sm text-gray-400"><Loader2 className="mr-2 animate-spin" size={18} />加载审核记录</div> : items.length === 0 ? <div className="flex min-h-56 flex-col items-center justify-center rounded-2xl bg-white text-center"><ClipboardCheck size={28} className="text-gray-300" /><div className="mt-3 text-sm font-semibold text-gray-700">当前没有待处理审核</div><div className="mt-1 text-xs text-gray-400">生成任务完成后会自动进入这里，审核通过后还需单独发布。</div></div> : <section className="overflow-hidden rounded-2xl bg-white shadow-sm"><div className="overflow-x-auto"><table className="min-w-[1040px] w-full text-left text-sm"><thead className="bg-gray-50 text-xs text-gray-400"><tr><th className="px-5 py-3 font-medium">案例</th><th className="px-5 py-3 font-medium">状态</th><th className="px-5 py-3 font-medium">模型 / 任务</th><th className="px-5 py-3 font-medium">检查摘要</th><th className="px-5 py-3 font-medium">更新时间</th><th className="px-5 py-3 font-medium">操作</th></tr></thead><tbody className="divide-y divide-gray-100">{items.map((item) => { const summary = item.check_summary || {}; return <tr key={item.id} className="hover:bg-gray-50/70"><td className="max-w-[360px] px-5 py-4"><button type="button" onClick={() => void openDetail(item)} className="text-left"><div className="font-semibold text-gray-900">{item.spec_title || "未命名案例"}</div><div className="mt-1 line-clamp-2 text-xs text-gray-400">{item.prompt}</div><div className="mt-2 flex flex-wrap gap-1">{[item.category, ...(item.styles || []).slice(0, 2)].filter(Boolean).map((tag) => <span key={tag} className="rounded-full bg-gray-100 px-2 py-0.5 text-[10px] text-gray-500">{tag}</span>)}</div></button></td><td className="px-5 py-4"><span className={`rounded-full px-2 py-1 text-xs font-medium ${statusClass(item.status)}`}>{statusLabels[item.status]}</span></td><td className="px-5 py-4 text-xs text-gray-600"><div>{item.model_code}</div><div className="mt-1 text-gray-400">{item.task_public_id}</div></td><td className="px-5 py-4 text-xs"><div className={summary.overall === "pass" ? "text-emerald-600" : "text-red-600"}>{summary.overall === "pass" ? "基础检查通过" : summary.overall === "fail" ? "存在阻断项" : "尚未完成"}</div><div className="mt-1 text-gray-400">阻断 {summary.blockers?.length || 0} · 警告 {summary.warnings?.length || 0}</div></td><td className="px-5 py-4 text-xs text-gray-500">{dateValue(item.updated_at)}</td><td className="px-5 py-4"><div className="flex items-center gap-1"><button type="button" onClick={() => void openDetail(item)} className="rounded-lg p-2 text-gray-500 hover:bg-gray-100" title="查看详情"><Eye size={16} /></button>{(item.status === "pending_review" || item.status === "auto_failed") && <button type="button" onClick={() => void action(item, "auto-check")} disabled={busy} className="rounded-lg p-2 text-blue-600 hover:bg-blue-50" title="运行自动检查"><ClipboardCheck size={16} /></button>}{item.status === "awaiting_manual_review" && <button type="button" onClick={() => void action(item, "approve")} disabled={busy} className="rounded-lg p-2 text-emerald-600 hover:bg-emerald-50" title="审核通过"><Check size={16} /></button>}{item.status === "approved" && <button type="button" onClick={() => void action(item, "publish")} disabled={busy} className="rounded-lg p-2 text-blue-600 hover:bg-blue-50" title="发布到灵感广场"><Send size={16} /></button>}</div></td></tr>; })}</tbody></table></div></section>}

      {detail && <div className="fixed inset-0 z-50 flex items-end justify-center bg-gray-950/30 p-0 sm:items-center sm:p-6" role="dialog" aria-modal="true" aria-label="质量审核详情"><div className="max-h-[94vh] w-full overflow-y-auto rounded-t-2xl bg-white p-5 shadow-2xl sm:max-w-5xl sm:rounded-2xl"><div className="flex items-start justify-between gap-4"><div><div className="text-xs font-semibold uppercase tracking-[0.16em] text-gray-400">Quality Review</div><h2 className="mt-1 text-xl font-bold text-gray-950">{detail.spec_title || detail.public_id}</h2><div className="mt-2 flex flex-wrap items-center gap-2"><span className={`rounded-full px-2 py-1 text-xs font-medium ${statusClass(detail.status)}`}>{statusLabels[detail.status]}</span><span className="text-xs text-gray-400">{detail.model_code} · 版本 {detail.review_version}</span></div></div><button type="button" onClick={() => setDetail(null)} className="rounded-lg p-2 text-gray-500 hover:bg-gray-100" title="关闭详情"><X size={18} /></button></div><div className="mt-5 grid gap-5 lg:grid-cols-[280px_1fr]"><div>{detail.output_asset_url ? <a href={detail.output_asset_url} target="_blank" rel="noreferrer" className="block overflow-hidden rounded-xl bg-gray-100"><img src={detail.output_asset_url} alt={detail.spec_title || "生成结果"} className="aspect-square w-full object-cover" /></a> : <div className="flex aspect-square items-center justify-center rounded-xl bg-gray-50 text-sm text-gray-400"><ImageIcon size={22} /></div>}<div className="mt-3 text-xs text-gray-400">任务 {detail.task_public_id}</div></div><div><div className="rounded-xl bg-gray-50 p-4 text-sm leading-6 text-gray-700"><div className="mb-1 text-xs font-semibold text-gray-400">生成提示词</div>{detail.prompt}</div><div className="mt-4 grid gap-3 sm:grid-cols-2">{[["资源", detail.asset_check], ["规格", detail.metadata_check], ["OCR", detail.ocr_check], ["语义一致性", detail.semantic_check], ["风险", detail.risk_check], ["重复", detail.duplicate_check]].map(([label, value]) => { const record = value as Record<string, unknown>; const checkStatus = String(record.status || ""); return <div key={label as string} className="rounded-xl border border-gray-100 p-3"><div className="flex items-center justify-between text-xs font-semibold text-gray-500"><span>{label as string}</span><span className={resultTone(checkStatus)}>{checkStatusLabels[checkStatus] || "未检查"}</span></div><pre className="mt-2 max-h-28 overflow-auto whitespace-pre-wrap break-words text-[10px] leading-4 text-gray-400">{JSON.stringify(record, null, 2)}</pre></div>; })}</div></div></div><div className="mt-5 rounded-xl bg-gray-50 p-4 text-xs leading-5 text-gray-600"><div className="font-semibold text-gray-700">检查摘要</div><div className="mt-2">阻断项：{detail.check_summary?.blockers?.join("、") || "无"}</div><div>警告：{detail.check_summary?.warnings?.join("、") || "无"}</div>{detail.rejection_reason && <div className="mt-2 text-red-600">拒绝原因：{detail.rejection_reason}</div>}</div>{detail.status === "awaiting_manual_review" && <div className="mt-4 rounded-xl border border-blue-100 bg-blue-50 px-4 py-3 text-xs leading-5 text-blue-800"><div className="font-semibold">下一步：人工核对画面</div><div className="mt-1">确认图片与生成提示词、分类和标签一致，且没有明显变形、侵权元素或不可接受的瑕疵。符合要求后点击“审核通过”；不符合则点击“拒绝”并填写原因。</div></div>}{detail.status === "approved" && <div className="mt-4 rounded-xl border border-emerald-100 bg-emerald-50 px-4 py-3 text-xs leading-5 text-emerald-800"><div className="font-semibold">下一步：确认发布</div><div className="mt-1">审核通过不会自动公开。确认标题、图片和标签都适合前端展示后，再点击“发布到灵感广场”。</div></div>}<div className="mt-5 flex flex-wrap justify-end gap-2"><button type="button" onClick={() => void action(detail, "auto-check")} disabled={busy || detail.status === "published"} className="inline-flex items-center gap-2 rounded-xl border border-gray-200 px-4 py-2 text-sm text-gray-700 hover:bg-gray-50 disabled:opacity-50"><ClipboardCheck size={16} />重新检查</button>{detail.status === "awaiting_manual_review" && <><button type="button" onClick={() => void action(detail, "reject")} disabled={busy} className="inline-flex items-center gap-2 rounded-xl border border-red-100 px-4 py-2 text-sm text-red-600 hover:bg-red-50"><X size={16} />拒绝</button><button type="button" onClick={() => void action(detail, "approve")} disabled={busy} className="inline-flex items-center gap-2 rounded-xl bg-emerald-600 px-4 py-2 text-sm font-medium text-white hover:bg-emerald-700"><Check size={16} />审核通过</button></>}{detail.status === "approved" && <button type="button" onClick={() => void action(detail, "publish")} disabled={busy} className="inline-flex items-center gap-2 rounded-xl bg-gray-950 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800"><Send size={16} />发布到灵感广场</button>}<button type="button" onClick={() => setDetail(null)} className="rounded-xl border border-gray-200 px-4 py-2 text-sm text-gray-700 hover:bg-gray-50">关闭</button></div></div></div>}
    </div>
  );
}

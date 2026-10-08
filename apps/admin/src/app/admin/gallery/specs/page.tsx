"use client";

import { ChangeEvent, useCallback, useEffect, useMemo, useState } from "react";
import {
  AlertTriangle,
  Archive,
  Check,
  CheckCircle2,
  ChevronRight,
  Clock3,
  FileJson,
  History,
  ListChecks,
  Loader2,
  Pencil,
  RefreshCw,
  Save,
  ShieldAlert,
  Sparkles,
  Target,
  Upload,
  X,
} from "lucide-react";
import { adminApi } from "@/lib/api";
import { AdminPagination } from "@/components/AdminPagination";

type Status = "draft" | "ready" | "archived";
type GapStatus = "open" | "in_progress" | "fulfilled" | "dismissed";

interface ModelCandidate {
  code: string;
  role: string;
  rationale: string;
}

interface GapTask {
  id: number;
  public_id: string;
  dimension_type: string;
  category: string;
  style: string;
  scene: string;
  prompt_language: string;
  target_count: number;
  current_count: number;
  gap_count: number;
  priority: string;
  title: string;
  rationale: string;
  risk_flags: string[];
  status: GapStatus;
  spec_count: number;
  ready_spec_count: number;
  updated_at: string;
}

interface CaseSpec {
  id: number;
  public_id: string;
  gap_task_id: number;
  gap_task_public_id: string;
  gap_task_title: string;
  version: number;
  status: Status;
  title: string;
  purpose: string;
  prompt: string;
  negative_prompt: string;
  prompt_language: string;
  image_text_languages: string[];
  expected_text: string[];
  category: string;
  styles: string[];
  scenes: string[];
  aspect_ratio: string;
  output_media_type: string;
  candidate_models: ModelCandidate[];
  acceptance_criteria: string[];
  selection_reason: string;
  risk_flags: string[];
  risk_notes: string;
  risk_review_status: "not_required" | "pending" | "approved" | "rejected";
  generator_type: string;
  generator_model_code: string;
  generator_prompt_version: string;
  estimated_cost: number;
  actual_cost: number;
  validation_errors: string[];
  content_hash: string;
  created_at: string;
  updated_at: string;
}

interface VersionRecord {
  version: number;
  change_type: string;
  changed_by?: number | null;
  created_at: string;
  snapshot?: Record<string, unknown>;
}

interface ListResponse<T> {
  items: T[];
  total: number;
  page: number;
  page_size: number;
}

interface EditorState {
  title: string;
  purpose: string;
  prompt: string;
  negative_prompt: string;
  prompt_language: string;
  category: string;
  aspect_ratio: string;
  selection_reason: string;
  risk_notes: string;
  risk_review_status: CaseSpec["risk_review_status"];
  status: Status;
}

const PAGE_SIZE = 12;
const STATUS_OPTIONS: Array<{ value: Status | ""; label: string }> = [
  { value: "", label: "全部状态" },
  { value: "draft", label: "草稿" },
  { value: "ready", label: "可生成" },
  { value: "archived", label: "已归档" },
];
const GAP_STATUS_OPTIONS: Array<{ value: GapStatus; label: string }> = [
  { value: "open", label: "待处理" },
  { value: "in_progress", label: "进行中" },
  { value: "fulfilled", label: "已完成" },
  { value: "dismissed", label: "已忽略" },
];

function listValue(values: string[]) {
  return values.join("\n");
}

function parseList(value: string) {
  return Array.from(new Set(value.split(/[\n,，]/).map((item) => item.trim()).filter(Boolean)));
}

function dateValue(value?: string) {
  if (!value) return "-";
  return new Date(value).toLocaleString("zh-CN", { dateStyle: "short", timeStyle: "short" });
}

function statusLabel(status: Status) {
  return STATUS_OPTIONS.find((item) => item.value === status)?.label || status;
}

function statusClass(status: Status) {
  if (status === "ready") return "bg-emerald-50 text-emerald-700";
  if (status === "archived") return "bg-gray-100 text-gray-500";
  return "bg-amber-50 text-amber-700";
}

function priorityClass(priority: string) {
  if (priority === "critical") return "bg-red-50 text-red-700";
  if (priority === "high") return "bg-orange-50 text-orange-700";
  return "bg-gray-100 text-gray-600";
}

function fieldClass(extra = "") {
  return `mt-1 w-full rounded-xl border border-gray-200 bg-white px-3 py-2.5 text-sm text-gray-900 outline-none transition focus:border-gray-900 focus:ring-2 focus:ring-gray-900/10 ${extra}`;
}

export default function GalleryCaseSpecsPage() {
  const [view, setView] = useState<"tasks" | "specs">("tasks");
  const [tasks, setTasks] = useState<GapTask[]>([]);
  const [specs, setSpecs] = useState<CaseSpec[]>([]);
  const [taskStatus, setTaskStatus] = useState("");
  const [specStatus, setSpecStatus] = useState("");
  const [gapTaskFilter, setGapTaskFilter] = useState("");
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [editor, setEditor] = useState<EditorState | null>(null);
  const [detail, setDetail] = useState<CaseSpec | null>(null);
  const [versions, setVersions] = useState<VersionRecord[]>([]);
  const [stylesText, setStylesText] = useState("");
  const [scenesText, setScenesText] = useState("");
  const [imageTextLanguagesText, setImageTextLanguagesText] = useState("");
  const [expectedTextText, setExpectedTextText] = useState("");
  const [acceptanceCriteriaText, setAcceptanceCriteriaText] = useState("");
  const [candidateModelsText, setCandidateModelsText] = useState("");
  const [detailLoading, setDetailLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [importing, setImporting] = useState(false);
  const [importOpen, setImportOpen] = useState(false);
  const [importPayload, setImportPayload] = useState<{ tasks?: unknown[]; specs?: unknown[] } | null>(null);
  const [importName, setImportName] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const taskParams = new URLSearchParams({ page: "1", page_size: "100" });
      if (taskStatus) taskParams.set("status", taskStatus);
      const specParams = new URLSearchParams({ page: String(page), page_size: String(PAGE_SIZE) });
      if (specStatus) specParams.set("status", specStatus);
      if (gapTaskFilter) specParams.set("gap_task", gapTaskFilter);
      const [taskResult, specResult] = await Promise.all([
        adminApi<ListResponse<GapTask>>(`/gallery/gap-tasks?${taskParams.toString()}`),
        adminApi<ListResponse<CaseSpec>>(`/gallery/case-specs?${specParams.toString()}`),
      ]);
      setTasks(taskResult.items || []);
      setSpecs(specResult.items || []);
      setTotal(specResult.total || 0);
    } catch (err) {
      setError(err instanceof Error ? err.message : "案例规格加载失败");
    } finally {
      setLoading(false);
    }
  }, [gapTaskFilter, page, specStatus, taskStatus]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    setPage(1);
  }, [gapTaskFilter, specStatus]);

  const summary = useMemo(() => ({
    taskCount: tasks.length,
    openTasks: tasks.filter((task) => task.status === "open" || task.status === "in_progress").length,
    specCount: total,
    readyCount: specs.filter((spec) => spec.status === "ready").length,
  }), [specs, tasks, total]);

  const refresh = () => {
    setNotice("");
    void load();
  };

  const openSpec = async (item: CaseSpec) => {
    setDetailLoading(true);
    setError("");
    try {
      const [full, history] = await Promise.all([
        adminApi<CaseSpec>(`/gallery/case-specs/${item.id}`),
        adminApi<VersionRecord[]>(`/gallery/case-specs/${item.id}/versions`),
      ]);
      setDetail(full);
      setEditor({
        title: full.title,
        purpose: full.purpose,
        prompt: full.prompt,
        negative_prompt: full.negative_prompt,
        prompt_language: full.prompt_language,
        category: full.category,
        aspect_ratio: full.aspect_ratio,
        selection_reason: full.selection_reason,
        risk_notes: full.risk_notes,
        risk_review_status: full.risk_review_status,
        status: full.status,
      });
      setStylesText(listValue(full.styles));
      setScenesText(listValue(full.scenes));
      setImageTextLanguagesText(listValue(full.image_text_languages));
      setExpectedTextText(listValue(full.expected_text));
      setAcceptanceCriteriaText(listValue(full.acceptance_criteria));
      setCandidateModelsText(JSON.stringify(full.candidate_models, null, 2));
      setVersions(history || []);
    } catch (err) {
      setError(err instanceof Error ? err.message : "规格详情加载失败");
    } finally {
      setDetailLoading(false);
    }
  };

  const closeDetail = () => {
    setDetail(null);
    setEditor(null);
    setVersions([]);
  };

  const saveSpec = async () => {
    if (!detail || !editor) return;
    let candidateModels: ModelCandidate[];
    try {
      const parsed = JSON.parse(candidateModelsText || "[]");
      if (!Array.isArray(parsed)) throw new Error("候选模型必须是 JSON 数组");
      candidateModels = parsed;
    } catch (err) {
      setError(err instanceof Error ? err.message : "候选模型 JSON 无效");
      return;
    }
    setSaving(true);
    setError("");
    try {
      const updated = await adminApi<CaseSpec>(`/gallery/case-specs/${detail.id}`, {
        method: "PATCH",
        body: JSON.stringify({
          ...editor,
          image_text_languages: parseList(imageTextLanguagesText),
          expected_text: parseList(expectedTextText),
          styles: parseList(stylesText),
          scenes: parseList(scenesText),
          candidate_models: candidateModels,
          acceptance_criteria: parseList(acceptanceCriteriaText),
        }),
      });
      setDetail(updated);
      const history = await adminApi<VersionRecord[]>(`/gallery/case-specs/${detail.id}/versions`);
      setVersions(history || []);
      setNotice(`“${updated.title}”已保存，第 ${updated.version} 版`);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "规格保存失败");
    } finally {
      setSaving(false);
    }
  };

  const updateTaskStatus = async (task: GapTask, status: GapStatus) => {
    try {
      await adminApi(`/gallery/gap-tasks/${task.id}/status`, { method: "PATCH", body: JSON.stringify({ status }) });
      setNotice(`缺口任务“${task.title}”已更新为${GAP_STATUS_OPTIONS.find((item) => item.value === status)?.label}`);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "缺口任务更新失败");
    }
  };

  const handleImportFile = async (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file) return;
    try {
      const parsed = JSON.parse(await file.text());
      if (!parsed || typeof parsed !== "object" || (!Array.isArray(parsed.tasks) && !Array.isArray(parsed.specs))) {
        throw new Error("文件需要包含 tasks 或 specs 数组");
      }
      setImportPayload(parsed);
      setImportName(file.name);
      setError("");
    } catch (err) {
      setImportPayload(null);
      setImportName("");
      setError(err instanceof Error ? err.message : "导入文件不是有效的规格 JSON");
    }
    event.target.value = "";
  };

  const importSpecs = async () => {
    if (!importPayload) return;
    setImporting(true);
    setError("");
    try {
      await adminApi("/gallery/case-specs/import", {
        method: "POST",
        body: JSON.stringify({ tasks: importPayload.tasks || [], specs: importPayload.specs || [] }),
      });
      setImportOpen(false);
      setImportPayload(null);
      setImportName("");
      setNotice("规格已导入，所有新规格仍保持 draft，不会生成或公开图片");
      setView("specs");
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "规格导入失败");
    } finally {
      setImporting(false);
    }
  };

  return (
    <div className="w-full space-y-6">
      <header className="flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between">
        <div>
          <div className="mb-2 flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.18em] text-gray-400">
            <Sparkles size={14} />
            Inspiration Gallery / Stage 5
          </div>
          <h1 className="text-2xl font-bold tracking-tight text-gray-950">案例规格工作台</h1>
          <p className="mt-1 max-w-3xl text-sm leading-6 text-gray-500">把覆盖率缺口整理成可审核、可追踪、可进入下一阶段的案例规格。当前页面只管理规格，不调用图片模型，也不会发布公共案例。</p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <button type="button" onClick={refresh} className="inline-flex items-center gap-2 rounded-xl border border-gray-200 bg-white px-4 py-2.5 text-sm font-medium text-gray-700 hover:bg-gray-50" title="刷新数据">
            <RefreshCw size={16} />
            刷新
          </button>
          <button type="button" onClick={() => setImportOpen(true)} className="inline-flex items-center gap-2 rounded-xl bg-gray-950 px-4 py-2.5 text-sm font-semibold text-white shadow-sm hover:bg-gray-800">
            <Upload size={16} />
            导入规格 JSON
          </button>
        </div>
      </header>

      <section className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        {[
          ["缺口任务", summary.taskCount, Target],
          ["待处理缺口", summary.openTasks, Clock3],
          ["规格总数", summary.specCount, FileJson],
          ["可生成规格", summary.readyCount, CheckCircle2],
        ].map(([label, value, Icon]) => {
          const MetricIcon = Icon as typeof Target;
          return <div key={String(label)} className="rounded-2xl border border-gray-200 bg-white px-4 py-4 shadow-sm"><MetricIcon size={17} className="text-gray-400" /><div className="mt-3 text-2xl font-semibold text-gray-950">{String(value)}</div><div className="mt-1 text-xs text-gray-500">{String(label)}</div></div>;
        })}
      </section>

      <div className="flex flex-wrap items-center gap-2 border-b border-gray-200">
        {[
          ["tasks", "缺口任务", Target],
          ["specs", "案例规格", ListChecks],
        ].map(([value, label, Icon]) => {
          const TabIcon = Icon as typeof Target;
          const active = view === value;
          return <button key={String(value)} type="button" onClick={() => setView(value as "tasks" | "specs")} className={`inline-flex items-center gap-2 border-b-2 px-3 py-3 text-sm font-semibold transition ${active ? "border-gray-950 text-gray-950" : "border-transparent text-gray-400 hover:text-gray-700"}`}><TabIcon size={16} />{String(label)}</button>;
        })}
      </div>

      {error && <div role="alert" className="flex items-start gap-3 rounded-xl border border-red-100 bg-red-50 px-4 py-3 text-sm text-red-700"><AlertTriangle size={17} className="mt-0.5 shrink-0" /><span>{error}</span><button type="button" onClick={() => setError("")} className="ml-auto text-red-500" title="关闭提示"><X size={16} /></button></div>}
      {notice && <div role="status" className="flex items-start gap-3 rounded-xl border border-emerald-100 bg-emerald-50 px-4 py-3 text-sm text-emerald-700"><Check size={17} className="mt-0.5 shrink-0" /><span>{notice}</span><button type="button" onClick={() => setNotice("")} className="ml-auto text-emerald-600" title="关闭提示"><X size={16} /></button></div>}

      {view === "tasks" ? (
        <section className="overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-sm">
          <div className="flex flex-col gap-3 border-b border-gray-100 px-5 py-4 md:flex-row md:items-center md:justify-between">
            <div><h2 className="font-semibold text-gray-950">覆盖率缺口任务</h2><p className="mt-1 text-xs text-gray-400">优先处理高价值标签组合，再由规格审核决定是否进入媒体生成。</p></div>
            <select aria-label="缺口任务状态" value={taskStatus} onChange={(event) => setTaskStatus(event.target.value)} className="rounded-xl border border-gray-200 bg-white px-3 py-2 text-sm text-gray-700"><option value="">全部状态</option>{GAP_STATUS_OPTIONS.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}</select>
          </div>
          {loading ? <LoadingState /> : tasks.length === 0 ? <EmptyState label="暂无缺口任务，请先导入阶段 5 规格 JSON。" /> : <div className="overflow-x-auto"><table className="min-w-[900px] w-full text-left text-sm"><thead className="bg-gray-50 text-xs text-gray-400"><tr><th className="px-5 py-3 font-medium">任务</th><th className="px-5 py-3 font-medium">维度</th><th className="px-5 py-3 font-medium">缺口</th><th className="px-5 py-3 font-medium">规格进度</th><th className="px-5 py-3 font-medium">优先级</th><th className="px-5 py-3 font-medium">状态</th><th className="px-5 py-3 font-medium">更新时间</th></tr></thead><tbody className="divide-y divide-gray-100">{tasks.map((task) => <tr key={task.id} className="hover:bg-gray-50/70"><td className="max-w-[330px] px-5 py-4"><div className="font-semibold text-gray-900">{task.title}</div><div className="mt-1 line-clamp-2 text-xs leading-5 text-gray-400">{task.rationale}</div><div className="mt-2 flex flex-wrap gap-1.5">{[task.category, task.style, task.scene, task.prompt_language].filter(Boolean).map((tag) => <span key={tag} className="rounded-full bg-gray-100 px-2 py-0.5 text-[10px] text-gray-500">{tag}</span>)}</div></td><td className="px-5 py-4 text-xs text-gray-600">{task.dimension_type}</td><td className="px-5 py-4"><div className="font-semibold text-gray-900">{task.gap_count}</div><div className="text-xs text-gray-400">目标 {task.target_count} / 当前 {task.current_count}</div></td><td className="px-5 py-4"><div className="font-medium text-gray-700">{task.spec_count} 条</div><div className="text-xs text-gray-400">ready {task.ready_spec_count} 条</div></td><td className="px-5 py-4"><span className={`rounded-full px-2 py-1 text-xs font-medium ${priorityClass(task.priority)}`}>{task.priority}</span></td><td className="px-5 py-4"><select aria-label={`${task.title} 状态`} value={task.status} onChange={(event) => void updateTaskStatus(task, event.target.value as GapStatus)} className="rounded-lg border border-gray-200 bg-white px-2 py-1.5 text-xs text-gray-700">{GAP_STATUS_OPTIONS.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}</select></td><td className="px-5 py-4 text-xs text-gray-400">{dateValue(task.updated_at)}</td></tr>)}</tbody></table></div>}
        </section>
      ) : (
        <section className="overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-sm">
          <div className="flex flex-col gap-3 border-b border-gray-100 px-5 py-4 xl:flex-row xl:items-center xl:justify-between">
            <div><h2 className="font-semibold text-gray-950">案例规格列表</h2><p className="mt-1 text-xs text-gray-400">draft 是待审核草稿，ready 仅表示允许阶段 6 创建生成任务。</p></div>
            <div className="flex flex-wrap gap-2"><select aria-label="案例规格状态" value={specStatus} onChange={(event) => setSpecStatus(event.target.value)} className="rounded-xl border border-gray-200 bg-white px-3 py-2 text-sm text-gray-700">{STATUS_OPTIONS.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}</select><select aria-label="按缺口任务筛选" value={gapTaskFilter} onChange={(event) => setGapTaskFilter(event.target.value)} className="max-w-[260px] rounded-xl border border-gray-200 bg-white px-3 py-2 text-sm text-gray-700"><option value="">全部缺口任务</option>{tasks.map((task) => <option key={task.public_id} value={task.public_id}>{task.title}</option>)}</select></div>
          </div>
          {loading ? <LoadingState /> : specs.length === 0 ? <EmptyState label="暂无案例规格，请从右上角导入草稿 JSON。" /> : <div className="overflow-x-auto"><table className="min-w-[980px] w-full text-left text-sm"><thead className="bg-gray-50 text-xs text-gray-400"><tr><th className="px-5 py-3 font-medium">规格</th><th className="px-5 py-3 font-medium">分类 / 风格 / 场景</th><th className="px-5 py-3 font-medium">候选模型</th><th className="px-5 py-3 font-medium">风险</th><th className="px-5 py-3 font-medium">状态</th><th className="px-5 py-3 font-medium">版本</th><th className="px-5 py-3 font-medium"></th></tr></thead><tbody className="divide-y divide-gray-100">{specs.map((spec) => <tr key={spec.id} className="hover:bg-gray-50/70"><td className="max-w-[350px] px-5 py-4"><div className="flex items-start gap-3"><div className="mt-0.5 rounded-lg bg-gray-100 p-2 text-gray-500"><FileJson size={16} /></div><div><div className="font-semibold text-gray-900">{spec.title}</div><div className="mt-1 line-clamp-2 text-xs leading-5 text-gray-400">{spec.purpose}</div><div className="mt-2 text-[10px] text-gray-400">{spec.public_id}</div></div></div></td><td className="px-5 py-4"><div className="text-xs font-medium text-gray-700">{spec.category}</div><div className="mt-1 flex flex-wrap gap-1">{[...spec.styles, ...spec.scenes].map((tag) => <span key={tag} className="rounded-full bg-gray-100 px-2 py-0.5 text-[10px] text-gray-500">{tag}</span>)}</div></td><td className="px-5 py-4"><div className="flex flex-wrap gap-1.5">{spec.candidate_models.map((model) => <span key={model.code} className="rounded-full border border-gray-200 px-2 py-1 text-[10px] text-gray-600">{model.code}</span>)}</div></td><td className="px-5 py-4">{spec.risk_flags.length > 0 ? <span className="inline-flex items-center gap-1 rounded-full bg-red-50 px-2 py-1 text-xs text-red-700"><ShieldAlert size={13} />{spec.risk_flags.length} 项</span> : <span className="text-xs text-emerald-600">无标记</span>}</td><td className="px-5 py-4"><span className={`rounded-full px-2 py-1 text-xs font-medium ${statusClass(spec.status)}`}>{statusLabel(spec.status)}</span>{spec.risk_review_status === "pending" && <div className="mt-1 text-[10px] text-amber-600">待风险审核</div>}</td><td className="px-5 py-4 text-xs text-gray-500">v{spec.version}</td><td className="px-5 py-4 text-right"><button type="button" onClick={() => void openSpec(spec)} className="inline-flex items-center gap-1.5 rounded-lg border border-gray-200 px-3 py-2 text-xs font-medium text-gray-700 hover:bg-gray-50"><Pencil size={14} />编辑<ChevronRight size={14} /></button></td></tr>)}</tbody></table></div>}
          <AdminPagination page={page} total={total} pageSize={PAGE_SIZE} onPageChange={setPage} />
        </section>
      )}

      {detailLoading && <div className="fixed inset-0 z-40 flex items-center justify-center bg-gray-950/20"><div className="rounded-2xl bg-white px-5 py-4 text-sm text-gray-600 shadow-xl"><Loader2 className="mr-2 inline animate-spin" size={16} />正在加载规格详情</div></div>}
      {detail && editor && <div className="fixed inset-0 z-50 bg-gray-950/25" onMouseDown={(event) => { if (event.target === event.currentTarget) closeDetail(); }}><aside className="ml-auto flex h-full w-full max-w-3xl flex-col overflow-hidden bg-[#f7f8fb] shadow-2xl" aria-label="案例规格编辑"><div className="flex items-start justify-between border-b border-gray-200 bg-white px-5 py-4"><div><div className="flex items-center gap-2 text-xs text-gray-400"><FileJson size={14} />{detail.public_id}</div><h2 className="mt-1 text-lg font-bold text-gray-950">编辑案例规格</h2><p className="mt-1 text-xs text-gray-400">来源任务：{detail.gap_task_title}</p></div><button type="button" onClick={closeDetail} className="rounded-lg p-2 text-gray-400 hover:bg-gray-100 hover:text-gray-700" title="关闭编辑"><X size={19} /></button></div><div className="flex-1 overflow-y-auto p-5"><div className="mb-5 rounded-xl border border-amber-100 bg-amber-50 px-4 py-3 text-xs leading-5 text-amber-800"><strong>阶段边界：</strong>保存规格不会调用图片模型。只有人工审核通过并标记为 ready，下一阶段才允许创建媒体生成任务。</div><div className="space-y-5"><section className="rounded-2xl border border-gray-200 bg-white p-4"><div className="mb-3 flex items-center gap-2 font-semibold text-gray-900"><Pencil size={16} />内容与 Prompt</div><div className="space-y-3"><label className="block text-xs font-medium text-gray-600">标题<input value={editor.title} onChange={(event) => setEditor({ ...editor, title: event.target.value })} className={fieldClass()} /></label><label className="block text-xs font-medium text-gray-600">用途说明<textarea value={editor.purpose} onChange={(event) => setEditor({ ...editor, purpose: event.target.value })} rows={3} className={fieldClass("resize-y")} /></label><label className="block text-xs font-medium text-gray-600">Prompt<textarea value={editor.prompt} onChange={(event) => setEditor({ ...editor, prompt: event.target.value })} rows={7} className={fieldClass("resize-y font-mono text-xs leading-5")} /></label><label className="block text-xs font-medium text-gray-600">Negative Prompt<textarea value={editor.negative_prompt} onChange={(event) => setEditor({ ...editor, negative_prompt: event.target.value })} rows={3} className={fieldClass("resize-y font-mono text-xs leading-5")} /></label></div></section><section className="rounded-2xl border border-gray-200 bg-white p-4"><div className="mb-3 flex items-center gap-2 font-semibold text-gray-900"><ListChecks size={16} />标签与输出</div><div className="grid gap-3 sm:grid-cols-2"><label className="block text-xs font-medium text-gray-600">Prompt 语言<input value={editor.prompt_language} onChange={(event) => setEditor({ ...editor, prompt_language: event.target.value })} className={fieldClass()} /></label><label className="block text-xs font-medium text-gray-600">分类<input value={editor.category} onChange={(event) => setEditor({ ...editor, category: event.target.value })} className={fieldClass()} /></label><label className="block text-xs font-medium text-gray-600">画幅<select value={editor.aspect_ratio} onChange={(event) => setEditor({ ...editor, aspect_ratio: event.target.value })} className={fieldClass()}>{["1:1", "4:5", "3:4", "4:3", "16:9", "9:16", "21:9", "auto"].map((value) => <option key={value}>{value}</option>)}</select></label><label className="block text-xs font-medium text-gray-600">图片文字语言<input value={imageTextLanguagesText} onChange={(event) => setImageTextLanguagesText(event.target.value)} placeholder="每行一项" className={fieldClass()} /></label></div><label className="mt-3 block text-xs font-medium text-gray-600">风格标签<textarea value={stylesText} onChange={(event) => setStylesText(event.target.value)} rows={2} className={fieldClass("resize-y")} /></label><label className="mt-3 block text-xs font-medium text-gray-600">场景标签<textarea value={scenesText} onChange={(event) => setScenesText(event.target.value)} rows={2} className={fieldClass("resize-y")} /></label><label className="mt-3 block text-xs font-medium text-gray-600">期望出现的文字<textarea value={expectedTextText} onChange={(event) => setExpectedTextText(event.target.value)} rows={2} className={fieldClass("resize-y")} /></label></section><section className="rounded-2xl border border-gray-200 bg-white p-4"><div className="mb-3 flex items-center gap-2 font-semibold text-gray-900"><Target size={16} />生成与验收约束</div><label className="block text-xs font-medium text-gray-600">候选模型 JSON<textarea value={candidateModelsText} onChange={(event) => setCandidateModelsText(event.target.value)} rows={6} className={fieldClass("resize-y font-mono text-xs leading-5")} spellCheck={false} /></label><label className="mt-3 block text-xs font-medium text-gray-600">验收标准<textarea value={acceptanceCriteriaText} onChange={(event) => setAcceptanceCriteriaText(event.target.value)} rows={7} className={fieldClass("resize-y")} /></label><label className="mt-3 block text-xs font-medium text-gray-600">筛选理由<textarea value={editor.selection_reason} onChange={(event) => setEditor({ ...editor, selection_reason: event.target.value })} rows={3} className={fieldClass("resize-y")} /></label></section><section className="rounded-2xl border border-gray-200 bg-white p-4"><div className="mb-3 flex items-center gap-2 font-semibold text-gray-900"><ShieldAlert size={16} />风险与状态</div>{detail.risk_flags.length > 0 ? <div className="mb-3 rounded-xl border border-red-100 bg-red-50 px-3 py-3 text-xs leading-5 text-red-700"><div className="font-semibold">自动识别风险：{detail.risk_flags.join("、")}</div><div className="mt-1">未完成风险审核时，后端会拒绝 ready。</div></div> : <div className="mb-3 rounded-xl border border-emerald-100 bg-emerald-50 px-3 py-3 text-xs text-emerald-700">当前没有自动识别风险标记。</div>}<label className="block text-xs font-medium text-gray-600">风险审核状态<select value={editor.risk_review_status} onChange={(event) => setEditor({ ...editor, risk_review_status: event.target.value as EditorState["risk_review_status"] })} className={fieldClass()}><option value="not_required">无需审核</option><option value="pending">待审核</option><option value="approved">已批准</option><option value="rejected">已拒绝</option></select></label><label className="mt-3 block text-xs font-medium text-gray-600">风险审核备注<textarea value={editor.risk_notes} onChange={(event) => setEditor({ ...editor, risk_notes: event.target.value })} rows={3} className={fieldClass("resize-y")} /></label><label className="mt-3 block text-xs font-medium text-gray-600">规格状态<select value={editor.status} onChange={(event) => setEditor({ ...editor, status: event.target.value as Status })} className={fieldClass()}>{STATUS_OPTIONS.filter((item) => item.value).map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}</select></label>{detail.validation_errors.length > 0 && <div className="mt-3 rounded-xl border border-orange-100 bg-orange-50 px-3 py-3 text-xs leading-5 text-orange-800"><div className="font-semibold">当前校验提示</div><ul className="mt-1 list-disc pl-4">{detail.validation_errors.map((item) => <li key={item}>{item}</li>)}</ul></div>}</section><section className="rounded-2xl border border-gray-200 bg-white p-4"><div className="mb-3 flex items-center gap-2 font-semibold text-gray-900"><History size={16} />版本历史</div>{versions.length === 0 ? <div className="text-xs text-gray-400">暂无版本记录</div> : <div className="space-y-2">{versions.map((item) => <div key={`${item.version}-${item.change_type}`} className="flex items-center justify-between rounded-xl bg-gray-50 px-3 py-2.5 text-xs"><div><span className="font-semibold text-gray-700">v{item.version}</span><span className="ml-2 text-gray-500">{item.change_type}</span></div><span className="text-gray-400">{dateValue(item.created_at)}</span></div>)}</div>}</section></div></div><div className="flex flex-wrap items-center justify-between gap-3 border-t border-gray-200 bg-white px-5 py-4"><div className="text-xs text-gray-400">最后更新：{dateValue(detail.updated_at)}</div><div className="flex items-center gap-2"><button type="button" onClick={closeDetail} className="rounded-xl border border-gray-200 px-4 py-2.5 text-sm text-gray-600 hover:bg-gray-50">取消</button><button type="button" disabled={saving} onClick={() => void saveSpec()} className="inline-flex items-center gap-2 rounded-xl bg-gray-950 px-5 py-2.5 text-sm font-semibold text-white hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-50"><Save size={16} />{saving ? "保存中..." : "保存规格"}</button></div></div></aside></div>}

      {importOpen && <div className="fixed inset-0 z-50 flex items-center justify-center bg-gray-950/30 p-4" onMouseDown={(event) => { if (event.target === event.currentTarget) setImportOpen(false); }}><div className="w-full max-w-xl rounded-2xl bg-white p-5 shadow-2xl"><div className="flex items-start justify-between"><div><div className="flex items-center gap-2 text-gray-950"><FileJson size={18} /><h2 className="font-semibold">导入案例规格 JSON</h2></div><p className="mt-2 text-xs leading-5 text-gray-500">支持阶段 5 脚本生成的完整 JSON，也支持只包含 tasks/specs 的裁剪文件。导入使用幂等 public_id，新规格保持 draft。</p></div><button type="button" onClick={() => setImportOpen(false)} className="rounded-lg p-2 text-gray-400 hover:bg-gray-100" title="关闭导入"><X size={18} /></button></div><label className="mt-5 flex cursor-pointer flex-col items-center justify-center rounded-xl border border-dashed border-gray-300 bg-gray-50 px-5 py-8 text-center hover:border-gray-500"><Upload size={24} className="text-gray-400" /><span className="mt-2 text-sm font-medium text-gray-700">选择 JSON 文件</span><span className="mt-1 text-xs text-gray-400">{importName || "尚未选择文件"}</span><input type="file" accept="application/json,.json" onChange={handleImportFile} className="sr-only" /></label>{importPayload && <div className="mt-4 grid grid-cols-2 gap-3"><div className="rounded-xl bg-gray-50 px-3 py-3 text-sm text-gray-700"><span className="block text-xs text-gray-400">缺口任务</span><strong>{importPayload.tasks?.length || 0}</strong> 条</div><div className="rounded-xl bg-gray-50 px-3 py-3 text-sm text-gray-700"><span className="block text-xs text-gray-400">案例规格</span><strong>{importPayload.specs?.length || 0}</strong> 条</div></div>}<div className="mt-5 flex justify-end gap-2"><button type="button" onClick={() => setImportOpen(false)} className="rounded-xl border border-gray-200 px-4 py-2.5 text-sm text-gray-600 hover:bg-gray-50">取消</button><button type="button" disabled={!importPayload || importing} onClick={() => void importSpecs()} className="inline-flex items-center gap-2 rounded-xl bg-gray-950 px-5 py-2.5 text-sm font-semibold text-white hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-50"><Upload size={16} />{importing ? "导入中..." : "开始导入"}</button></div></div></div>}
    </div>
  );
}

function LoadingState() {
  return <div className="flex min-h-48 items-center justify-center gap-2 text-sm text-gray-400"><Loader2 size={17} className="animate-spin" />正在加载</div>;
}

function EmptyState({ label }: { label: string }) {
  return <div className="flex min-h-48 flex-col items-center justify-center gap-2 px-5 text-center text-sm text-gray-400"><Archive size={20} />{label}</div>;
}

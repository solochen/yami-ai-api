"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Loader2, Plus, X, CheckCircle2, AlertCircle, Square, Copy, Download, Wand2, RefreshCw, ImagePlus, Package, History, Trash2 } from "lucide-react";
import { api, apiBlob, uploadAsset } from "@/lib/api";
import { useI18n } from "@/i18n/I18nProvider";
import {
  ECOM_BATCH, ECOM_LANGUAGES, ECOM_LIMITS, ECOM_PLATFORMS, ECOM_RATIOS, ECOM_RESOLUTIONS,
  ECOM_SUITES, ECOM_VISUAL_STYLES, ECOM_MODULES, ECOM_STATUS_LABELS, ECOM_BATCH_STATUS_LABELS,
  buildPlan, buildRefinePrompt, moduleById, suiteById, type EcomFormState,
} from "@/lib/ecomFactory";

type Project = { public_id: string; status: string; inputs: Record<string, any>; outputs: Record<string, any>; estimated_cost?: number; actual_cost?: number; error_message?: string | null };
type AssetItem = { id: string; role: "product" | "reference"; name: string; previewUrl: string; uploadedUrl: string; status: "local" | "uploading" | "ready" | "failed" };
type ResultItem = { id: string; moduleId: string; label: string; ratio: string; prompt: string; status: string; imageUrl?: string; error?: string; progress?: string; attempts?: number };
type FactoryState = { mode?: string; suite?: string; platform?: string; resolution?: string; productName?: string; batchStatus?: string; items?: ResultItem[]; total?: number; success?: number; failed?: number };

const field = "w-full rounded-xl border border-gray-200 bg-gray-50/80 px-3 py-2.5 text-sm text-gray-900 focus:outline-none focus:ring-2 focus:ring-primary dark:border-white/10 dark:bg-white/5 dark:text-gray-100";
const selectField = "w-full rounded-xl border border-gray-200 bg-white px-3 py-2.5 text-sm text-gray-700 outline-none focus:border-primary focus:ring-2 focus:ring-primary/20 dark:border-white/10 dark:bg-gray-800 dark:text-gray-100 [color-scheme:light] dark:[color-scheme:dark]";
const sectionTitle = "text-xs font-semibold uppercase tracking-wide text-gray-400 dark:text-gray-500";

export function EcomFactoryWorkspace({ workflowCode, workflowName, project, onProjectChange, onSubmitted, onStop, onNewTask, onLoadHistory }: {
  workflowCode: string;
  workflowName: string;
  project: Project | null;
  onProjectChange: (p: Project) => void;
  onSubmitted: (publicId: string) => void;
  onStop: () => void | Promise<void>;
  onNewTask: () => void;
  onLoadHistory: (id: string) => void | Promise<void>;
}) {
  const { t } = useI18n();
  const [productAssets, setProductAssets] = useState<AssetItem[]>([]);
  const [referenceAssets, setReferenceAssets] = useState<AssetItem[]>([]);
  const [productName, setProductName] = useState("");
  const [sellingPoints, setSellingPoints] = useState("");
  const [audience, setAudience] = useState("");
  const [scenario, setScenario] = useState("");
  const [platform, setPlatform] = useState<string>(ECOM_PLATFORMS[0]);
  const [language, setLanguage] = useState<string>(ECOM_LANGUAGES[0]);
  const [ratio, setRatio] = useState<string>(ECOM_RATIOS[0]);
  const [resolution, setResolution] = useState<string>(ECOM_RESOLUTIONS[0]);
  const [countPerModule, setCountPerModule] = useState(1);
  const [suiteId, setSuiteId] = useState("launch");
  const [selectedModuleIds, setSelectedModuleIds] = useState<string[]>(ECOM_SUITES[0].moduleIds);
  const [visualStyle, setVisualStyle] = useState<string>(ECOM_VISUAL_STYLES[0]);
  const [customStyle, setCustomStyle] = useState("");
  const [copyEnabled, setCopyEnabled] = useState(true);
  const [extraPrompt, setExtraPrompt] = useState("");
  const [batchPrompt, setBatchPrompt] = useState("");
  const [batchCount, setBatchCount] = useState(20);
  const [batchStrategy, setBatchStrategy] = useState<string>(ECOM_BATCH.strategies[0]);
  const [batchSimilarity, setBatchSimilarity] = useState<string>(ECOM_BATCH.similarities[1]);
  const [batchVariation, setBatchVariation] = useState<string>(ECOM_BATCH.variations[1]);
  const [batchKeepKeys, setBatchKeepKeys] = useState<string[]>([...ECOM_BATCH.defaultKeepKeys]);
  const [batchChangeKeys, setBatchChangeKeys] = useState<string[]>([...ECOM_BATCH.defaultChangeKeys]);
  const [quickPrompt, setQuickPrompt] = useState("");
  const [reverseBusy, setReverseBusy] = useState(false);
  const [quickBusy, setQuickBusy] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [toast, setToast] = useState("");
  const [cardEditor, setCardEditor] = useState<{ id: string; mode: "regen" | "refine"; text: string } | null>(null);
  const [history, setHistory] = useState<{ public_id: string; title?: string; status: string; created_at?: string; outputs?: Record<string, any>; estimated_cost?: number }[]>([]);
  const [restoredId, setRestoredId] = useState("");
  const productInputRef = useRef<HTMLInputElement>(null);
  const referenceInputRef = useRef<HTMLInputElement>(null);
  const reverseInputRef = useRef<HTMLInputElement>(null);

  const mode = suiteId === "batch" ? "batch" : "suite";
  const running = project?.status === "pending" || project?.status === "running";
  const state: FactoryState = project?.outputs?.ecom_factory || {};
  const items: ResultItem[] = state.items || [];
  const successCount = items.filter(i => i.status === "success").length;
  const failedCount = items.filter(i => i.status === "failed").length;
  const hasUnfinished = items.length > 0 && successCount < items.length;

  const form: EcomFormState = useMemo(() => ({
    productName, sellingPoints, audience, scenario, platform, language,
    visualStyle, customStyle, extraPrompt, copyEnabled,
    hasAssets: productAssets.length + referenceAssets.length > 0,
    batchPrompt, batchStrategy, batchSimilarity, batchVariation, batchKeepKeys, batchChangeKeys,
  }), [productName, sellingPoints, audience, scenario, platform, language, visualStyle, customStyle, extraPrompt, copyEnabled, productAssets.length, referenceAssets.length, batchPrompt, batchStrategy, batchSimilarity, batchVariation, batchKeepKeys, batchChangeKeys]);

  const plan = useMemo(() => buildPlan(form, mode, {
    selectedModuleIds, selectedSuite: suiteId, ratio, countPerModule, batchCount,
  }), [form, mode, selectedModuleIds, suiteId, ratio, countPerModule, batchCount]);

  const canSubmit = !running && !busy && plan.length > 0 && (
    mode === "batch" ? batchPrompt.trim().length > 0
      : productAssets.length > 0 || productName.trim() !== "" || sellingPoints.trim() !== ""
  );

  const showToast = useCallback((text: string) => {
    setToast(text);
    window.setTimeout(() => setToast(""), 1800);
  }, []);

  const loadHistoryList = useCallback(() => {
    api<{ items: typeof history }>(`/api/agent-projects?workflow_code=${encodeURIComponent(workflowCode)}&page=1&page_size=20`)
      .then(r => setHistory(r.items || [])).catch(() => setHistory([]));
  }, [workflowCode]);

  useEffect(() => { loadHistoryList(); }, [loadHistoryList]);

  // 历史恢复：切换 project 时从 inputs 快照回填表单，从 outputs 回填结果
  useEffect(() => {
    if (!project || !project.inputs || restoredId === project.public_id) return;
    const inputs = project.inputs;
    setRestoredId(project.public_id);
    setProductName(str(inputs.productName));
    setSellingPoints(str(inputs.sellingPoints));
    setAudience(str(inputs.audience));
    setScenario(str(inputs.scenario));
    if (str(inputs.platform)) setPlatform(str(inputs.platform));
    if (str(inputs.language)) setLanguage(str(inputs.language));
    if (str(inputs.visualStyle)) setVisualStyle(str(inputs.visualStyle));
    setCustomStyle(str(inputs.customStyle));
    setCopyEnabled(inputs.copyEnabled === true);
    setExtraPrompt(str(inputs.extraPrompt));
    if (str(inputs.selectedSuite) && suiteById(str(inputs.selectedSuite))) setSuiteId(str(inputs.selectedSuite));
    if (Array.isArray(inputs.selectedModuleIds) && inputs.selectedModuleIds.length) setSelectedModuleIds(inputs.selectedModuleIds.map(String));
    if (str(inputs.ratio) && (ECOM_RATIOS as readonly string[]).includes(str(inputs.ratio))) setRatio(str(inputs.ratio));
    if (str(inputs.resolution) && (ECOM_RESOLUTIONS as readonly string[]).includes(str(inputs.resolution))) setResolution(str(inputs.resolution));
    if (num(inputs.countPerModule) > 0) setCountPerModule(Math.min(num(inputs.countPerModule), 3));
    setBatchPrompt(str(inputs.batchPrompt));
    if (num(inputs.batchCount) > 0) setBatchCount(num(inputs.batchCount));
    if (str(inputs.batchStrategy)) setBatchStrategy(str(inputs.batchStrategy));
    if (str(inputs.batchSimilarity)) setBatchSimilarity(str(inputs.batchSimilarity));
    if (str(inputs.batchVariation)) setBatchVariation(str(inputs.batchVariation));
    if (Array.isArray(inputs.batchKeepKeys)) setBatchKeepKeys(inputs.batchKeepKeys.map(String));
    if (Array.isArray(inputs.batchChangeKeys)) setBatchChangeKeys(inputs.batchChangeKeys.map(String));
    const assets = (Array.isArray(inputs.assets) ? inputs.assets : []) as { role?: string; name?: string; url?: string }[];
    const restored: AssetItem[] = assets.filter(a => a.url).map((a, i) => ({
      id: `hist-${i}`, role: a.role === "reference" ? "reference" : "product",
      name: a.name || "history-image", previewUrl: a.url!, uploadedUrl: a.url!, status: "ready",
    }));
    setProductAssets(restored.filter(a => a.role === "product"));
    setReferenceAssets(restored.filter(a => a.role === "reference"));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [project?.public_id]);

  const pickFiles = async (files: FileList | null, role: "product" | "reference") => {
    if (!files?.length) return;
    const limit = role === "product" ? ECOM_LIMITS.maxProductImages : ECOM_LIMITS.maxReferenceImages;
    const list = role === "product" ? productAssets : referenceAssets;
    const room = Math.max(0, limit - list.length);
    const picked = Array.from(files).filter(f => f.type.startsWith("image/")).slice(0, room);
    if (!picked.length) {
      setError(`最多上传 ${limit} 张${role === "product" ? "商品图" : "参考图"}`);
      return;
    }
    setError("");
    const staged: AssetItem[] = picked.map((f, i) => ({
      id: `${role}-${Date.now().toString(36)}-${i}`, role, name: f.name || "image",
      previewUrl: URL.createObjectURL(f), uploadedUrl: "", status: "local",
    }));
    const append = (prev: AssetItem[]) => [...prev, ...staged];
    if (role === "product") setProductAssets(append); else setReferenceAssets(append);
    for (const item of staged) {
      const update = (prev: AssetItem[]) => prev.map(a => a.id === item.id ? { ...a, status: "uploading" as const } : a);
      if (role === "product") setProductAssets(update); else setReferenceAssets(update);
      try {
        const asset = await uploadAsset(picked[staged.indexOf(item)], { name: item.name, kind: "image", asset_type: "scene" });
        const done = (prev: AssetItem[]) => prev.map(a => a.id === item.id ? { ...a, uploadedUrl: asset.url, status: "ready" as const } : a);
        if (role === "product") setProductAssets(done); else setReferenceAssets(done);
      } catch (e) {
        const fail = (prev: AssetItem[]) => prev.map(a => a.id === item.id ? { ...a, status: "failed" as const } : a);
        if (role === "product") setProductAssets(fail); else setReferenceAssets(fail);
        setError(e instanceof Error ? e.message : t("agent.uploadFailed"));
      }
    }
  };

  const removeAsset = (id: string) => {
    setProductAssets(prev => prev.filter(a => a.id !== id));
    setReferenceAssets(prev => prev.filter(a => a.id !== id));
  };

  const selectSuite = (id: string) => {
    setSuiteId(id);
    const suite = suiteById(id);
    if (suite) setSelectedModuleIds([...suite.moduleIds]);
  };

  const toggleModule = (id: string) => {
    setSelectedModuleIds(prev => prev.includes(id) ? prev.filter(x => x !== id) : [...prev, id]);
  };

  const buildPayload = (submitMode: "suite" | "batch" | "quick") => ({
    mode: submitMode,
    productName: productName.trim(), sellingPoints: sellingPoints.trim(),
    audience: audience.trim(), scenario: scenario.trim(),
    platform, language, visualStyle, customStyle: customStyle.trim(),
    extraPrompt: extraPrompt.trim(), copyEnabled,
    selectedSuite: suiteId, selectedModuleIds,
    ratio, resolution, countPerModule,
    batchPrompt: batchPrompt.trim(), batchCount, batchStrategy, batchSimilarity, batchVariation,
    batchKeepKeys, batchChangeKeys,
    quickPrompt: quickPrompt.trim(),
    assets: [...productAssets, ...referenceAssets]
      .filter(a => a.uploadedUrl)
      .map(a => ({ id: a.id, role: a.role, name: a.name, url: a.uploadedUrl, status: a.status })),
  });

  const submit = async () => {
    if (!canSubmit) {
      setError(mode === "batch" ? "请先填写批量生成提示词" : "请先补充商品图、商品信息或选择生成模块");
      return;
    }
    setBusy(true); setError("");
    try {
      const p = await api<Project>(`/api/agents/${workflowCode}/projects`, {
        method: "POST", body: JSON.stringify({ inputs: buildPayload(mode === "batch" ? "batch" : "suite") }),
      });
      onProjectChange(p);
      onSubmitted(p.public_id);
      loadHistoryList();
    } catch (e) {
      setError(e instanceof Error ? e.message : "生成任务启动失败");
    } finally {
      setBusy(false);
    }
  };

  const itemAction = async (action: "regenerate" | "refine" | "continue", itemId?: string, text?: string) => {
    if (!project || running || busy) return;
    setBusy(true); setError(""); setCardEditor(null);
    try {
      await api(`/api/agent-projects/${project.public_id}/ecom-items`, {
        method: "POST",
        body: JSON.stringify({
          action, itemId,
          prompt: action === "regenerate" ? text : undefined,
          instruction: action === "refine" ? text : undefined,
        }),
      });
      const p = await api<Project>(`/api/agent-projects/${project.public_id}`);
      onProjectChange(p);
      onSubmitted(p.public_id);
    } catch (e) {
      setError(e instanceof Error ? e.message : "操作失败");
    } finally {
      setBusy(false);
    }
  };

  const reverseImage = async (file?: File | null) => {
    if (!file) return;
    setReverseBusy(true); setError("");
    try {
      const asset = await uploadAsset(file, { name: file.name, kind: "image", asset_type: "scene" });
      const res = await api<{ prompt: string }>("/api/agents/ecom-factory/reverse", {
        method: "POST", body: JSON.stringify({ image_url: asset.url }),
      });
      setQuickPrompt(res.prompt);
    } catch (e) {
      setError(e instanceof Error ? e.message : "反推失败");
    } finally {
      setReverseBusy(false);
      if (reverseInputRef.current) reverseInputRef.current.value = "";
    }
  };

  const quickGenerate = async () => {
    if (!quickPrompt.trim()) { setError("请输入或反推出提示词"); return; }
    setQuickBusy(true); setError("");
    try {
      const p = await api<Project>(`/api/agents/${workflowCode}/projects`, {
        method: "POST", body: JSON.stringify({ inputs: { ...buildPayload("quick"), quickPrompt: quickPrompt.trim() } }),
      });
      onProjectChange(p);
      onSubmitted(p.public_id);
      loadHistoryList();
    } catch (e) {
      setError(e instanceof Error ? e.message : "快速生图失败");
    } finally {
      setQuickBusy(false);
    }
  };

  const copyText = async (text: string, label = "已复制") => {
    try { await navigator.clipboard.writeText(text); showToast(label); }
    catch { showToast("复制失败"); }
  };

  const downloadImage = async (item: ResultItem) => {
    if (!item.imageUrl) return;
    try {
      const blob = await apiBlob(item.imageUrl.startsWith("http") ? item.imageUrl : item.imageUrl);
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `${(productName.trim() || "电商图片")}-${item.label}.jpg`.replace(/[\\/:*?"<>|]/g, "_");
      a.click();
      URL.revokeObjectURL(url);
    } catch {
      window.open(item.imageUrl, "_blank", "noopener");
    }
  };

  const exportZip = async () => {
    if (!project || successCount === 0) return;
    try {
      const blob = await apiBlob(`/api/agent-projects/${project.public_id}/export`);
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `${(productName.trim() || "电商图片工厂")}-电商套图.zip`;
      a.click();
      URL.revokeObjectURL(url);
    } catch (e) {
      setError(e instanceof Error ? e.message : "导出失败");
    }
  };

  const headerSummary = items.length
    ? `${successCount} 成功 / ${failedCount} 失败 / ${items.length - successCount - failedCount} 进行中`
    : plan.length ? "等待生成" : "请选择生成模块";

  const planRatioLabel = (moduleId: string, planRatio: string) => planRatio || moduleById(moduleId)?.defaultRatio || "1:1";

  return (
    <div className="relative flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-4 pb-6 pt-5 sm:px-6 lg:px-8">
      <input ref={productInputRef} type="file" accept="image/*" multiple className="hidden" onChange={e => { void pickFiles(e.target.files, "product"); e.currentTarget.value = ""; }} />
      <input ref={referenceInputRef} type="file" accept="image/*" multiple className="hidden" onChange={e => { void pickFiles(e.target.files, "reference"); e.currentTarget.value = ""; }} />
      <input ref={reverseInputRef} type="file" accept="image/*" className="hidden" onChange={e => { void reverseImage(e.target.files?.[0]); e.currentTarget.value = ""; }} />

      <header className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.2em] text-primary">Ecommerce Image Lab</p>
          <h2 className="mt-1 text-xl font-bold text-gray-900 dark:text-gray-50">{workflowName}</h2>
          <span className="text-sm text-gray-500 dark:text-gray-400">商品图 · 详情页 · 种草图 · 风格复刻 · 反推提示词</span>
        </div>
        <div className="flex items-center gap-2">
          {hasUnfinished && !running && (
            <button type="button" onClick={() => void itemAction("continue")} disabled={busy}
              className="rounded-xl border border-gray-200 px-3.5 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50 dark:border-white/10 dark:text-gray-200 dark:hover:bg-white/5">继续未完成</button>
          )}
          <button type="button" onClick={() => void submit()} disabled={!canSubmit}
            className="rounded-xl bg-primary px-4 py-2 text-sm font-semibold text-white shadow-sm hover:bg-primary/90 disabled:opacity-50">
            {running || busy ? "生成中..." : `生成 ${plan.length} 张`}
          </button>
          {running && (
            <button type="button" onClick={() => void onStop()}
              className="flex items-center gap-1.5 rounded-xl border border-red-200 px-3.5 py-2 text-sm font-medium text-red-600 hover:bg-red-50 dark:border-red-500/30 dark:hover:bg-red-500/10">
              <Square className="h-3.5 w-3.5" />停止队列
            </button>
          )}
          {!running && project && (
            <button type="button" onClick={onNewTask}
              className="rounded-xl border border-gray-200 px-3.5 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 dark:border-white/10 dark:text-gray-200 dark:hover:bg-white/5">新建任务</button>
          )}
        </div>
      </header>

      {(error || project?.error_message) && (
        <div className="flex items-start gap-2 rounded-xl border border-red-200 bg-red-50/80 px-3.5 py-2.5 text-sm text-red-700 dark:border-red-500/30 dark:bg-red-500/10 dark:text-red-300">
          <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
          <span>{error || project?.error_message}</span>
          <button className="ml-auto" onClick={() => setError("")}><X className="h-4 w-4" /></button>
        </div>
      )}
      {toast && (
        <div className="pointer-events-none fixed right-6 top-24 z-50 rounded-xl bg-gray-900/90 px-4 py-2 text-sm text-white shadow-lg dark:bg-white/90 dark:text-gray-900">{toast}</div>
      )}

      <div className="grid min-h-0 flex-1 grid-cols-1 gap-4 xl:grid-cols-[300px_minmax(0,1fr)_280px]">
        {/* 左栏：资产与商品信息 */}
        <aside className="flex flex-col gap-4">
          <section className="rounded-2xl border border-gray-200 bg-white/70 p-4 dark:border-white/10 dark:bg-white/5">
            <div className="mb-2 flex items-center justify-between">
              <strong className="text-sm font-semibold text-gray-900 dark:text-gray-100">{mode === "batch" ? "主参考图" : "商品资产"}</strong>
              <small className="text-xs text-gray-400">{productAssets.length}/{ECOM_LIMITS.maxProductImages}</small>
            </div>
            <button type="button" onClick={() => productInputRef.current?.click()} disabled={running}
              className="flex w-full flex-col items-center gap-1 rounded-xl border border-dashed border-gray-300 py-5 text-gray-500 transition hover:border-primary hover:text-primary disabled:opacity-50 dark:border-white/15 dark:text-gray-400">
              <ImagePlus className="h-5 w-5" />
              <b className="text-sm font-medium">{mode === "batch" ? "上传主参考图" : "上传商品图"}</b>
              <span className="text-xs">{mode === "batch" ? "主体、构图、原始画面" : "主体图、细节图、包装图"}</span>
            </button>
            {productAssets.length > 0 && (
              <div className="mt-3 grid grid-cols-2 gap-2">
                {productAssets.map(a => <AssetCard key={a.id} asset={a} disabled={running} onRemove={() => removeAsset(a.id)} />)}
              </div>
            )}
          </section>
          <section className="rounded-2xl border border-gray-200 bg-white/70 p-4 dark:border-white/10 dark:bg-white/5">
            <div className="mb-2 flex items-center justify-between">
              <strong className="text-sm font-semibold text-gray-900 dark:text-gray-100">{mode === "batch" ? "辅助参考图" : "风格参考"}</strong>
              <small className="text-xs text-gray-400">{referenceAssets.length}/{ECOM_LIMITS.maxReferenceImages}</small>
            </div>
            <button type="button" onClick={() => referenceInputRef.current?.click()} disabled={running}
              className="flex w-full flex-col items-center gap-1 rounded-xl border border-dashed border-gray-300 py-5 text-gray-500 transition hover:border-primary hover:text-primary disabled:opacity-50 dark:border-white/15 dark:text-gray-400">
              <ImagePlus className="h-5 w-5" />
              <b className="text-sm font-medium">{mode === "batch" ? "上传辅助参考图" : "上传参考图"}</b>
              <span className="text-xs">{mode === "batch" ? "风格、背景、材质、局部元素" : "构图、光影、氛围、爆款样式"}</span>
            </button>
            {referenceAssets.length > 0 && (
              <div className="mt-3 grid grid-cols-2 gap-2">
                {referenceAssets.map(a => <AssetCard key={a.id} asset={a} disabled={running} onRemove={() => removeAsset(a.id)} />)}
              </div>
            )}
          </section>
          <section className="flex flex-col gap-3 rounded-2xl border border-gray-200 bg-white/70 p-4 dark:border-white/10 dark:bg-white/5">
            <div className={sectionTitle}>商品信息</div>
            <label className="flex flex-col gap-1.5 text-xs text-gray-500 dark:text-gray-400">
              商品名称
              <input className={field} placeholder="例：便携咖啡机" value={productName} onChange={e => setProductName(e.target.value)} />
            </label>
            <label className="flex flex-col gap-1.5 text-xs text-gray-500 dark:text-gray-400">
              核心卖点
              <textarea className={field} rows={4} placeholder="材质、功能、痛点、优惠、规格" value={sellingPoints} onChange={e => setSellingPoints(e.target.value)} />
            </label>
            <label className="flex flex-col gap-1.5 text-xs text-gray-500 dark:text-gray-400">
              目标人群
              <input className={field} placeholder="例：办公室白领、露营用户" value={audience} onChange={e => setAudience(e.target.value)} />
            </label>
            <label className="flex flex-col gap-1.5 text-xs text-gray-500 dark:text-gray-400">
              使用场景
              <input className={field} placeholder="例：清晨厨房、户外露营、直播间" value={scenario} onChange={e => setScenario(e.target.value)} />
            </label>
          </section>
        </aside>

        {/* 中栏：配置 + 计划 + 结果 */}
        <main className="flex min-w-0 flex-col gap-4">
          <section className="grid grid-cols-2 gap-3 rounded-2xl border border-gray-200 bg-white/70 p-4 sm:grid-cols-3 dark:border-white/10 dark:bg-white/5">
            <label className="flex flex-col gap-1.5 text-xs text-gray-500 dark:text-gray-400">平台
              <select className={selectField} value={platform} onChange={e => setPlatform(e.target.value)}>
                {ECOM_PLATFORMS.map(p => <option key={p} value={p}>{p}</option>)}
              </select>
            </label>
            <label className="flex flex-col gap-1.5 text-xs text-gray-500 dark:text-gray-400">比例
              <select className={selectField} value={ratio} onChange={e => setRatio(e.target.value)}>
                {ECOM_RATIOS.map(r => <option key={r} value={r}>{r}</option>)}
              </select>
            </label>
            <label className="flex flex-col gap-1.5 text-xs text-gray-500 dark:text-gray-400">清晰度
              <select className={selectField} value={resolution} onChange={e => setResolution(e.target.value)}>
                {ECOM_RESOLUTIONS.map(r => <option key={r} value={r}>{r}</option>)}
              </select>
            </label>
            {mode === "batch" ? (
              <label className="flex flex-col gap-1.5 text-xs text-gray-500 dark:text-gray-400">批量张数
                <input type="number" min={1} max={ECOM_LIMITS.maxBatchCount} className={field} value={batchCount} onChange={e => setBatchCount(Math.max(1, Math.min(ECOM_LIMITS.maxBatchCount, Number(e.target.value) || 1)))} />
              </label>
            ) : (
              <label className="flex flex-col gap-1.5 text-xs text-gray-500 dark:text-gray-400">每类张数
                <input type="number" min={1} max={3} className={field} value={countPerModule} onChange={e => setCountPerModule(Math.max(1, Math.min(3, Number(e.target.value) || 1)))} />
              </label>
            )}
            <label className="flex flex-col gap-1.5 text-xs text-gray-500 dark:text-gray-400">语言
              <select className={selectField} value={language} onChange={e => setLanguage(e.target.value)}>
                {ECOM_LANGUAGES.map(l => <option key={l} value={l}>{l}</option>)}
              </select>
            </label>
          </section>

          <section className="flex flex-col gap-2.5 rounded-2xl border border-gray-200 bg-white/70 p-4 dark:border-white/10 dark:bg-white/5">
            <div className={sectionTitle}>套装</div>
            <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3">
              {ECOM_SUITES.map(suite => (
                <button key={suite.id} type="button" onClick={() => selectSuite(suite.id)}
                  className={`rounded-xl border px-3 py-2.5 text-left transition ${suiteId === suite.id ? "border-primary bg-primary/5 ring-1 ring-primary/30" : "border-gray-200 hover:border-primary/50 dark:border-white/10"}`}>
                  <strong className="block text-sm font-semibold text-gray-900 dark:text-gray-100">{suite.label}</strong>
                  <span className="mt-0.5 block text-xs leading-snug text-gray-500 dark:text-gray-400">{suite.description}</span>
                </button>
              ))}
            </div>
          </section>

          {mode === "batch" ? (
            <section className="flex flex-col gap-3 rounded-2xl border border-gray-200 bg-white/70 p-4 dark:border-white/10 dark:bg-white/5">
              <div className={sectionTitle}>参考图批量指令 · {plan.length} 张计划</div>
              <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
                <label className="flex flex-col gap-1.5 text-xs text-gray-500 dark:text-gray-400">生成方式
                  <select className={selectField} value={batchStrategy} onChange={e => setBatchStrategy(e.target.value)}>
                    {ECOM_BATCH.strategies.map(s => <option key={s} value={s}>{s}</option>)}
                  </select>
                </label>
                <label className="flex flex-col gap-1.5 text-xs text-gray-500 dark:text-gray-400">参考相似度
                  <select className={selectField} value={batchSimilarity} onChange={e => setBatchSimilarity(e.target.value)}>
                    {ECOM_BATCH.similarities.map(s => <option key={s} value={s}>{s}</option>)}
                  </select>
                </label>
                <label className="flex flex-col gap-1.5 text-xs text-gray-500 dark:text-gray-400">变化强度
                  <select className={selectField} value={batchVariation} onChange={e => setBatchVariation(e.target.value)}>
                    {ECOM_BATCH.variations.map(s => <option key={s} value={s}>{s}</option>)}
                  </select>
                </label>
              </div>
              <label className="flex flex-col gap-1.5 text-xs text-gray-500 dark:text-gray-400">用户提示词（必填）
                <textarea className={field} rows={3} placeholder="例：保持商品主体不变，把杯身图案换成夏日水果插画" value={batchPrompt} onChange={e => setBatchPrompt(e.target.value)} />
              </label>
              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <KeyToggle title="必须保持" options={ECOM_BATCH.keepKeys} selected={batchKeepKeys} onChange={setBatchKeepKeys} />
                <KeyToggle title="允许改变" options={ECOM_BATCH.changeKeys} selected={batchChangeKeys} onChange={setBatchChangeKeys} />
              </div>
              <div className="flex flex-wrap items-center gap-2">
                <span className="text-xs text-gray-500 dark:text-gray-400">快速数量</span>
                {ECOM_BATCH.countOptions.map(n => (
                  <button key={n} type="button" onClick={() => setBatchCount(n)}
                    className={`rounded-lg border px-2.5 py-1 text-xs ${batchCount === n ? "border-primary bg-primary/10 text-primary" : "border-gray-200 text-gray-600 hover:border-primary/50 dark:border-white/10 dark:text-gray-300"}`}>{n}</button>
                ))}
              </div>
            </section>
          ) : (
            <section className="flex flex-col gap-2.5 rounded-2xl border border-gray-200 bg-white/70 p-4 dark:border-white/10 dark:bg-white/5">
              <div className={sectionTitle}>图片类别</div>
              <div className="flex flex-wrap gap-2">
                {ECOM_MODULES.filter(m => m.id !== "reference-batch").map(m => {
                  const active = selectedModuleIds.includes(m.id);
                  return (
                    <button key={m.id} type="button" onClick={() => toggleModule(m.id)}
                      className={`flex flex-col rounded-xl border px-3 py-2 text-left transition ${active ? "border-transparent text-white shadow-sm" : "border-gray-200 text-gray-700 hover:border-primary/50 dark:border-white/10 dark:text-gray-200"}`}
                      style={active ? { backgroundColor: m.accent } : undefined}>
                      <span className="text-sm font-semibold">{m.shortLabel}</span>
                      <span className={`text-[11px] ${active ? "text-white/85" : "text-gray-400"}`}>{m.channel}</span>
                    </button>
                  );
                })}
              </div>
            </section>
          )}

          <section className="flex flex-col gap-3 rounded-2xl border border-gray-200 bg-white/70 p-4 dark:border-white/10 dark:bg-white/5">
            <div className="flex items-center justify-between">
              <label className="flex flex-1 flex-col gap-1.5 text-xs text-gray-500 dark:text-gray-400">视觉风格
                <select className={`${selectField} max-w-xs`} value={visualStyle} onChange={e => setVisualStyle(e.target.value)}>
                  {ECOM_VISUAL_STYLES.map(s => <option key={s} value={s}>{s}</option>)}
                </select>
              </label>
              <label className="mt-5 flex cursor-pointer items-center gap-2 text-xs text-gray-600 dark:text-gray-300">
                <input type="checkbox" checked={copyEnabled} onChange={e => setCopyEnabled(e.target.checked)} className="h-4 w-4 accent-[var(--primary,#ff8711)]" />
                允许生成少量运营文案
              </label>
            </div>
            {visualStyle === "自定义" && (
              <input className={field} placeholder="描述自定义视觉风格" value={customStyle} onChange={e => setCustomStyle(e.target.value)} />
            )}
            <label className="flex flex-col gap-1.5 text-xs text-gray-500 dark:text-gray-400">补充要求
              <textarea className={field} rows={2} placeholder="例：背景保持浅色；不要改变瓶身标签；更偏夏季清爽" value={extraPrompt} onChange={e => setExtraPrompt(e.target.value)} />
            </label>
          </section>

          <section className="rounded-2xl border border-gray-200 bg-white/70 p-4 dark:border-white/10 dark:bg-white/5">
            <div className="mb-3 flex items-center justify-between">
              <strong className="text-sm font-semibold text-gray-900 dark:text-gray-100">生成计划</strong>
              <span className="text-xs text-gray-500 dark:text-gray-400">{running || items.length ? headerSummary : `${plan.length} 张`}</span>
            </div>
            <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3">
              {(items.length ? items : plan.map(p => ({ ...p, status: "pending", progress: "" }))).map((item, idx) => {
                const planPrompt = plan[idx]?.prompt || item.prompt || "";
                const mod = moduleById(item.moduleId);
                return (
                  <article key={item.id} className="rounded-xl border border-gray-200 p-3 dark:border-white/10">
                    <div className="flex items-start justify-between gap-2">
                      <div>
                        <strong className="block text-sm font-semibold text-gray-900 dark:text-gray-100">{item.label}</strong>
                        <span className="text-[11px] text-gray-400">{mod?.channel || ""} · {planRatioLabel(item.moduleId, item.ratio)}</span>
                      </div>
                      <button type="button" title="复制提示词" onClick={() => void copyText(item.prompt || planPrompt, "已复制提示词")}
                        className="rounded-lg border border-gray-200 p-1.5 text-gray-500 hover:border-primary hover:text-primary dark:border-white/10 dark:text-gray-300">
                        <Copy className="h-3.5 w-3.5" />
                      </button>
                    </div>
                  </article>
                );
              })}
            </div>
          </section>

          <section className="rounded-2xl border border-gray-200 bg-white/70 p-4 dark:border-white/10 dark:bg-white/5">
            <div className="mb-3 flex items-center justify-between">
              <strong className="text-sm font-semibold text-gray-900 dark:text-gray-100">出图结果</strong>
              <span className="text-xs text-gray-500 dark:text-gray-400">{successCount} 张可下载</span>
            </div>
            {items.length === 0 ? (
              <p className="rounded-xl border border-dashed border-gray-300 py-8 text-center text-sm text-gray-400 dark:border-white/10">
                上传商品图或填写商品信息后，选择套装并开始生成。
              </p>
            ) : (
              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
                {items.map(item => (
                  <ResultCard key={item.id} item={item} running={running} busy={busy}
                    editor={cardEditor?.id === item.id ? cardEditor : null}
                    onEditorOpen={mode2 => setCardEditor({ id: item.id, mode: mode2, text: mode2 === "regen" ? item.prompt : "" })}
                    onEditorClose={() => setCardEditor(null)}
                    onEditorSubmit={text => void itemAction(cardEditor?.mode === "refine" ? "refine" : "regenerate", item.id, text)}
                    onRegenerate={() => void itemAction("regenerate", item.id)}
                    onDownload={() => void downloadImage(item)}
                  />
                ))}
              </div>
            )}
          </section>
        </main>

        {/* 右栏：快速生图 / 导出 / 历史 */}
        <aside className="flex flex-col gap-4">
          <section className="rounded-2xl border border-gray-200 bg-white/70 p-4 dark:border-white/10 dark:bg-white/5">
            <div className={sectionTitle}>提示词快速生图</div>
            <textarea className={`${field} mt-2`} rows={5} placeholder="直接输入提示词生成一张；或点下方「上传图片反推」，用图片自动生成提示词后再生成。" value={quickPrompt} onChange={e => setQuickPrompt(e.target.value)} />
            <div className="mt-2 flex gap-2">
              <button type="button" onClick={() => reverseInputRef.current?.click()} disabled={reverseBusy}
                className="flex flex-1 items-center justify-center gap-1.5 rounded-xl border border-gray-200 px-3 py-2 text-xs font-medium text-gray-600 hover:border-primary hover:text-primary disabled:opacity-50 dark:border-white/10 dark:text-gray-300">
                {reverseBusy ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Wand2 className="h-3.5 w-3.5" />}上传图片反推
              </button>
              <button type="button" onClick={() => void quickGenerate()} disabled={quickBusy || running || !quickPrompt.trim()}
                className="flex-1 rounded-xl bg-primary px-3 py-2 text-xs font-semibold text-white hover:bg-primary/90 disabled:opacity-50">
                {quickBusy ? "生成中..." : "生成"}
              </button>
            </div>
          </section>

          <section className="rounded-2xl border border-gray-200 bg-white/70 p-4 dark:border-white/10 dark:bg-white/5">
            <div className={sectionTitle}>导出与流转</div>
            <p className="mt-2 text-xs text-gray-400">{successCount} 张</p>
            <div className="mt-2 flex flex-col gap-2">
              <button type="button" onClick={() => void exportZip()} disabled={successCount === 0}
                className="flex items-center justify-center gap-1.5 rounded-xl border border-gray-200 px-3 py-2 text-xs font-medium text-gray-600 hover:border-primary hover:text-primary disabled:opacity-50 dark:border-white/10 dark:text-gray-300">
                <Package className="h-3.5 w-3.5" />打包下载
              </button>
              <button type="button" onClick={() => window.dispatchEvent(new Event("starai:open-works"))} disabled={successCount === 0}
                className="flex items-center justify-center gap-1.5 rounded-xl border border-gray-200 px-3 py-2 text-xs font-medium text-gray-600 hover:border-primary hover:text-primary disabled:opacity-50 dark:border-white/10 dark:text-gray-300">
                <Download className="h-3.5 w-3.5" />查看作品库
              </button>
            </div>
          </section>

          <section className="flex-1 rounded-2xl border border-gray-200 bg-white/70 p-4 dark:border-white/10 dark:bg-white/5">
            <div className="mb-2 flex items-center justify-between">
              <div className={sectionTitle}>历史记录</div>
              <button type="button" onClick={loadHistoryList} className="text-gray-400 hover:text-primary" title="刷新"><RefreshCw className="h-3.5 w-3.5" /></button>
            </div>
            <div className="flex flex-col gap-2">
              {history.length === 0 && <p className="py-4 text-center text-xs text-gray-400">暂无历史任务</p>}
              {history.map(h => {
                const s = h.outputs?.ecom_factory || {};
                const total = s.total || 0;
                const ok = s.success || 0;
                return (
                  <button key={h.public_id} type="button" onClick={() => void onLoadHistory(h.public_id)}
                    className={`rounded-xl border px-3 py-2.5 text-left transition ${project?.public_id === h.public_id ? "border-primary bg-primary/5" : "border-gray-200 hover:border-primary/50 dark:border-white/10"}`}>
                    <strong className="block text-sm font-semibold text-gray-900 dark:text-gray-100">{h.title || "电商图片工厂"}</strong>
                    <span className="mt-0.5 block text-[11px] text-gray-400">
                      {(suiteById(s.suite)?.label) || "电商套图"} · {ok}/{total} 张完成 · {ECOM_BATCH_STATUS_LABELS[s.batchStatus as string] || statusText(h.status)} · {s.resolution || ""}
                    </span>
                  </button>
                );
              })}
            </div>
          </section>

          <section className="grid grid-cols-3 gap-2">
            <StatCard label="参考图" value={String(productAssets.length + referenceAssets.length)} />
            <StatCard label="计划" value={String(items.length || plan.length)} />
            <StatCard label="完成" value={String(successCount)} />
          </section>
        </aside>
      </div>
    </div>
  );
}

function str(v: unknown): string { return typeof v === "string" ? v : ""; }
function num(v: unknown): number { const n = typeof v === "number" ? v : Number(v); return Number.isFinite(n) ? n : 0; }
function statusText(status: string): string {
  return ({ pending: "排队中", running: "进行中", succeeded: "已完成", failed: "有失败", canceled: "已停止" } as Record<string, string>)[status] || status;
}

function AssetCard({ asset, disabled, onRemove }: { asset: AssetItem; disabled?: boolean; onRemove: () => void }) {
  return (
    <article className="flex flex-col gap-1.5 rounded-xl border border-gray-200 p-2 dark:border-white/10">
      <div className="relative aspect-square overflow-hidden rounded-lg bg-gray-100 dark:bg-white/5">
        {/* eslint-disable-next-line @next/next/no-img-element */}
        <img src={asset.previewUrl} alt={asset.name} className="h-full w-full object-cover" />
      </div>
      <div className="flex items-center justify-between gap-1">
        <span className={`text-[10px] ${asset.status === "ready" ? "text-emerald-600 dark:text-emerald-400" : asset.status === "failed" ? "text-red-500" : "text-gray-400"}`}>
          {asset.status === "ready" ? "已上传" : asset.status === "uploading" ? "上传中" : asset.status === "failed" ? "失败" : "本地"}
        </span>
        <button type="button" disabled={disabled} onClick={onRemove} className="text-gray-400 hover:text-red-500 disabled:opacity-40"><X className="h-3.5 w-3.5" /></button>
      </div>
    </article>
  );
}

function KeyToggle({ title, options, selected, onChange }: { title: string; options: { key: string; label: string }[]; selected: string[]; onChange: (keys: string[]) => void }) {
  return (
    <div className="flex flex-col gap-1.5">
      <span className="text-xs text-gray-500 dark:text-gray-400">{title}</span>
      <div className="flex flex-wrap gap-1.5">
        {options.map(opt => {
          const active = selected.includes(opt.key);
          return (
            <button key={opt.key} type="button" onClick={() => onChange(active ? selected.filter(k => k !== opt.key) : [...selected, opt.key])}
              className={`rounded-lg border px-2 py-1 text-xs ${active ? "border-primary bg-primary/10 text-primary" : "border-gray-200 text-gray-600 hover:border-primary/50 dark:border-white/10 dark:text-gray-300"}`}>{opt.label}</button>
          );
        })}
      </div>
    </div>
  );
}

function StatCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-2xl border border-gray-200 bg-white/70 p-3 text-center dark:border-white/10 dark:bg-white/5">
      <div className="text-[10px] text-gray-400">{label}</div>
      <strong className="text-lg font-bold text-gray-900 dark:text-gray-100">{value}</strong>
    </div>
  );
}

function ResultCard({ item, running, busy, editor, onEditorOpen, onEditorClose, onEditorSubmit, onRegenerate, onDownload }: {
  item: ResultItem; running?: boolean; busy?: boolean;
  editor: { mode: "regen" | "refine"; text: string } | null;
  onEditorOpen: (mode: "regen" | "refine") => void;
  onEditorClose: () => void;
  onEditorSubmit: (text: string) => void;
  onRegenerate: () => void;
  onDownload: () => void;
}) {
  const [text, setText] = useState(editor?.text ?? "");
  useEffect(() => { setText(editor?.text ?? ""); }, [editor]);
  return (
    <article className="flex flex-col gap-2 rounded-xl border border-gray-200 p-3 dark:border-white/10">
      <div className="relative aspect-square overflow-hidden rounded-lg bg-gray-100 dark:bg-white/5">
        {item.imageUrl ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img src={item.imageUrl} alt={item.label} className="h-full w-full object-cover" />
        ) : (
          <div className="flex h-full w-full flex-col items-center justify-center gap-1.5 text-gray-400">
            {item.status === "generating" ? <Loader2 className="h-5 w-5 animate-spin text-primary" /> : item.status === "failed" ? <AlertCircle className="h-5 w-5 text-red-400" /> : item.status === "success" ? <CheckCircle2 className="h-5 w-5 text-emerald-500" /> : null}
            <span className="text-xs">{item.progress || ECOM_STATUS_LABELS[item.status] || item.status}</span>
          </div>
        )}
      </div>
      <div className="flex items-center justify-between gap-1">
        <strong className="text-sm font-semibold text-gray-900 dark:text-gray-100">{item.label}</strong>
        <span className={`text-[11px] ${item.status === "success" ? "text-emerald-600 dark:text-emerald-400" : item.status === "failed" ? "text-red-500" : "text-gray-400"}`}>
          {ECOM_STATUS_LABELS[item.status] || item.status}
        </span>
      </div>
      {item.error && <p className="line-clamp-2 text-[11px] text-red-500">{item.error}</p>}
      {editor && (
        <div className="flex flex-col gap-1.5">
          <textarea className={field} rows={editor.mode === "regen" ? 5 : 2} value={text}
            placeholder={editor.mode === "refine" ? "例：背景换成浅灰色，商品角度微调" : "编辑提示词"}
            onChange={e => setText(e.target.value)} />
          <div className="flex gap-1.5">
            <button type="button" disabled={busy} onClick={() => onEditorSubmit(text)}
              className="flex-1 rounded-lg bg-primary px-2 py-1.5 text-xs font-semibold text-white hover:bg-primary/90 disabled:opacity-50">
              {editor.mode === "refine" ? "开始精修" : "重新生成"}
            </button>
            <button type="button" onClick={onEditorClose} className="rounded-lg border border-gray-200 px-2.5 py-1.5 text-xs text-gray-500 dark:border-white/10">取消</button>
          </div>
        </div>
      )}
      {!editor && (
        <div className="flex gap-1.5">
          <button type="button" title="重新生成" disabled={running || busy} onClick={onRegenerate}
            className="flex-1 rounded-lg border border-gray-200 px-2 py-1.5 text-xs text-gray-600 hover:border-primary hover:text-primary disabled:opacity-50 dark:border-white/10 dark:text-gray-300">
            <span className="flex items-center justify-center gap-1"><RefreshCw className="h-3 w-3" />重新生成</span>
          </button>
          <button type="button" title="精修" disabled={running || busy || !item.imageUrl} onClick={() => onEditorOpen("refine")}
            className="flex-1 rounded-lg border border-gray-200 px-2 py-1.5 text-xs text-gray-600 hover:border-primary hover:text-primary disabled:opacity-50 dark:border-white/10 dark:text-gray-300">
            <span className="flex items-center justify-center gap-1"><Wand2 className="h-3 w-3" />精修</span>
          </button>
          <button type="button" title="下载" disabled={!item.imageUrl} onClick={onDownload}
            className="flex-1 rounded-lg border border-gray-200 px-2 py-1.5 text-xs text-gray-600 hover:border-primary hover:text-primary disabled:opacity-50 dark:border-white/10 dark:text-gray-300">
            <span className="flex items-center justify-center gap-1"><Download className="h-3 w-3" />下载</span>
          </button>
        </div>
      )}
    </article>
  );
}

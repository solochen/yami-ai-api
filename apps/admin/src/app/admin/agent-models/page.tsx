"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { AlertCircle, Check, Loader2, RefreshCw, Search } from "lucide-react";
import { adminApi } from "@/lib/api";

type Candidate = { code: string; display_name: string };
type Slot = {
  id: string; label: string; purpose: string; model_code: string; model_name: string;
  required_capabilities: string[]; sources: string[]; source_label: string;
  compatible: boolean; issue?: string; candidates: Candidate[]; shared_with: string[];
  configured_model_code: string; is_inherited: boolean;
};
type Agent = { code: string; name: string; is_enabled: boolean; system_workspace: boolean; slots: Slot[]; note?: string };
type Overview = { items: Agent[]; capability_labels: Record<string, string> };

function ModelSlot({ agent, slot, labels, reload }: { agent: Agent; slot: Slot; labels: Record<string, string>; reload: () => Promise<void> }) {
  const [selected, setSelected] = useState(slot.model_code);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [saved, setSaved] = useState(false);
  useEffect(() => { setSelected(slot.model_code); }, [slot.model_code]);
  const selectable = slot.candidates.some(model => model.code === selected);
  const inherited = slot.is_inherited;

  const save = async () => {
    setSaving(true); setError(""); setSaved(false);
    try {
      await adminApi(`/agents/${encodeURIComponent(agent.code)}/models/${encodeURIComponent(slot.id)}`, {
        method: "PUT",
        body: JSON.stringify({ model_code: selected, expected_model_code: slot.configured_model_code }),
      });
      await reload(); setSaved(true);
    } catch (err) { setError(err instanceof Error ? err.message : "保存失败，请重试"); }
    finally { setSaving(false); }
  };

  return <div className="rounded-2xl bg-gray-50/80 p-4 sm:p-5">
    <div className="grid min-w-0 gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(18rem,0.9fr)]">
      <div className="min-w-0">
        <div className="flex flex-wrap items-center gap-2">
          <h3 className="font-semibold text-gray-900">{slot.label}</h3>
          <span className={`rounded-full px-2 py-0.5 text-xs ${slot.compatible ? "bg-emerald-50 text-emerald-700" : "bg-amber-50 text-amber-800"}`}>{slot.compatible ? "能力兼容" : slot.model_code ? "需要检查" : "未固定"}</span>
        </div>
        <p className="mt-2 text-sm leading-6 text-gray-600">{slot.purpose}</p>
        <div className="mt-3 flex flex-wrap gap-1.5" aria-label="槽位必需能力">{slot.required_capabilities.map(key => <span key={key} className="rounded-lg bg-white px-2 py-1 text-xs text-gray-600">{labels[key] || key}</span>)}</div>
        <details className="mt-3 text-xs text-gray-500">
          <summary className="cursor-pointer rounded focus-visible:outline focus-visible:outline-2 focus-visible:outline-indigo-500">{slot.source_label}{slot.sources.length > 1 ? ` · ${slot.sources.length} 处引用同步更新` : ""}</summary>
          <ul className="mt-2 space-y-1">{slot.sources.map(source => <li key={source} className="break-all font-mono">{source}</li>)}</ul>
          {inherited && <p className="mt-2">在此保存只设置当前智能体的默认值。</p>}
          {!inherited && slot.shared_with.length > 0 && <p className="mt-2">共享影响：{slot.shared_with.join("、")} 的提示词增强将跟随此模型。</p>}
        </details>
      </div>
      <div className="min-w-0">
        <p className="mb-2 break-all text-xs text-gray-500">当前：{slot.model_name || slot.model_code || "由任务自动选择 / 尚未配置"}{slot.model_name && <span className="ml-1 font-mono">({slot.model_code})</span>}</p>
        <label htmlFor={`${agent.code}-${slot.id}`} className="sr-only">{agent.name} · {slot.label}替换模型</label>
        <div className="flex flex-col gap-2 sm:flex-row">
          <select id={`${agent.code}-${slot.id}`} value={selected} disabled={saving || !slot.candidates.length} onChange={event => { setSelected(event.target.value); setSaved(false); setError(""); }} className="min-w-0 flex-1 rounded-xl border border-gray-200 bg-white px-3 py-2.5 text-sm text-gray-800 focus:border-indigo-400 focus:outline-none focus:ring-2 focus:ring-indigo-100 disabled:bg-gray-100">
            {!slot.candidates.some(model => model.code === slot.model_code) && <option disabled value={slot.model_code}>{slot.model_code ? `${slot.compatible ? "当前" : "当前不可用"}：${slot.model_code}` : "请选择兼容模型"}</option>}
            {slot.candidates.map(model => <option key={model.code} value={model.code}>{model.display_name} · {model.code}</option>)}
          </select>
          <button type="button" onClick={() => void save()} disabled={saving || !selectable || selected === slot.model_code && !slot.issue && !inherited} className="inline-flex shrink-0 items-center justify-center gap-1.5 rounded-xl bg-indigo-600 px-4 py-2.5 text-sm font-semibold text-white hover:bg-indigo-700 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-indigo-500 disabled:cursor-not-allowed disabled:opacity-40">{saving ? <Loader2 size={15} className="animate-spin motion-reduce:animate-none" /> : saved ? <Check size={15} /> : null}{saving ? "保存中" : saved ? "已保存" : "保存"}</button>
        </div>
        {slot.issue && <p className="mt-2 text-xs leading-5 text-amber-800">{slot.issue}</p>}
        {!slot.candidates.length && <p className="mt-2 text-xs leading-5 text-gray-500">暂无兼容模型，请先到<Link href="/admin/models" className="text-indigo-600 underline">模型管理</Link>检查模型能力和线路配置。</p>}
        <p role="status" aria-live="polite" className={`mt-2 text-xs ${error ? "text-red-600" : "text-emerald-700"}`}>{error || (saved ? "已保存，后续任务将使用此配置。" : "")}</p>
      </div>
    </div>
  </div>;
}

export default function AgentModelsPage() {
  const [data, setData] = useState<Overview>({ items: [], capability_labels: {} });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [query, setQuery] = useState("");
  const [filter, setFilter] = useState("all");
  const load = useCallback(async () => { const result = await adminApi<Overview>("/agent-models"); setData(result); setError(""); }, []);
  const refresh = useCallback(async () => { setLoading(true); try { await load(); } catch (err) { setError(err instanceof Error ? err.message : "加载失败"); } finally { setLoading(false); } }, [load]);
  useEffect(() => { void refresh(); }, [refresh]);
  const items = useMemo(() => data.items.filter(agent => {
    const text = `${agent.name} ${agent.code} ${agent.slots.map(slot => `${slot.label} ${slot.purpose} ${slot.model_code} ${slot.model_name}`).join(" ")}`.toLowerCase();
    return text.includes(query.trim().toLowerCase()) && (filter === "all" || filter === "enabled" && agent.is_enabled || filter === "disabled" && !agent.is_enabled || filter === "issues" && agent.slots.some(slot => !slot.compatible));
  }), [data.items, query, filter]);
  const issueCount = data.items.reduce((count, agent) => count + agent.slots.filter(slot => !slot.compatible).length, 0);

  return <div className="space-y-6">
    <div className="flex flex-wrap items-start justify-between gap-4">
      <div><h1 className="text-2xl font-bold text-gray-950">智能体模型配置</h1><p className="mt-2 text-sm leading-6 text-gray-500">查看每个模型的职责，按槽位所需能力更换模型。包含已停用智能体和系统工作区。</p></div>
      <button onClick={() => void refresh()} disabled={loading} className="inline-flex items-center gap-2 rounded-xl bg-white px-4 py-2.5 text-sm font-medium text-gray-700 shadow-sm focus-visible:outline focus-visible:outline-2 focus-visible:outline-indigo-500 disabled:opacity-50"><RefreshCw size={16} className={loading ? "animate-spin motion-reduce:animate-none" : ""} />刷新</button>
    </div>
    <div className="flex flex-wrap items-center gap-3 rounded-2xl bg-white p-4 shadow-sm">
      <label className="relative w-full min-w-0 sm:w-auto sm:flex-1"><span className="sr-only">搜索智能体、用途或模型</span><Search size={17} className="absolute left-3 top-3 text-gray-400" /><input value={query} onChange={event => setQuery(event.target.value)} placeholder="搜索智能体、用途或模型" className="w-full min-w-0 rounded-xl border border-gray-200 py-2.5 pl-10 pr-3 text-sm focus:border-indigo-400 focus:outline-none focus:ring-2 focus:ring-indigo-100" /></label>
      <label><span className="sr-only">筛选状态</span><select value={filter} onChange={event => setFilter(event.target.value)} className="rounded-xl border border-gray-200 bg-white px-3 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-100"><option value="all">全部智能体</option><option value="enabled">已启用</option><option value="disabled">已停用</option><option value="issues">有待检查槽位</option></select></label>
      <p className="text-xs text-gray-500">{data.items.length} 个智能体 · {issueCount} 个槽位待检查或未固定</p>
    </div>
    {error && <div role="alert" className="flex items-center gap-2 rounded-2xl bg-red-50 p-4 text-sm text-red-700"><AlertCircle size={18} />{error}<button className="ml-auto underline" onClick={() => void refresh()}>重试</button></div>}
    {loading && !data.items.length ? <div role="status" className="py-20 text-center text-sm text-gray-500">正在加载智能体模型配置…</div> : !items.length && !error ? <div className="rounded-2xl bg-white py-16 text-center text-sm text-gray-500">没有匹配的智能体。</div> : items.map(agent => <section key={agent.code} className="min-w-0 rounded-2xl bg-white p-4 shadow-sm sm:p-6" aria-label={agent.name}>
      <div className="mb-4 flex flex-wrap items-center gap-2"><h2 className="text-lg font-semibold text-gray-950">{agent.name}</h2><span className={`rounded-full px-2.5 py-1 text-xs ${agent.is_enabled ? "bg-emerald-50 text-emerald-700" : "bg-gray-100 text-gray-500"}`}>{agent.is_enabled ? "已启用" : "已停用"}</span>{agent.system_workspace && <span className="rounded-full bg-indigo-50 px-2.5 py-1 text-xs text-indigo-700">系统工作区</span>}<code className="min-w-0 break-all text-xs text-gray-400">{agent.code}</code></div>
      {agent.note && <p className="mb-4 text-xs leading-6 text-gray-500">{agent.note}</p>}
      <div className="space-y-3">{agent.slots.map(slot => <ModelSlot key={slot.id} agent={agent} slot={slot} labels={data.capability_labels} reload={load} />)}</div>
    </section>)}
  </div>;
}

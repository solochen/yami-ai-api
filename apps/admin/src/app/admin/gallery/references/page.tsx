"use client";

import { ChangeEvent, useCallback, useEffect, useMemo, useState } from "react";
import { Check, ExternalLink, FileJson, Image as ImageIcon, RefreshCw, Search, ShieldCheck, UploadCloud, X } from "lucide-react";
import { adminApi } from "@/lib/api";
import { AdminPagination } from "@/components/AdminPagination";

interface ReferenceItem {
  id: number;
  source_item_id: string;
  source_key: string;
  title: string;
  prompt: string;
  prompt_preview?: string;
  category: string;
  styles: string[];
  scenes: string[];
  image_source_url: string;
  image_asset_url?: string;
  image_alt?: string;
  source_url?: string;
  github_url?: string;
  featured: boolean;
  asset_license_status: "unverified" | "verified" | "rejected";
  status: "pending_license" | "published" | "hidden";
  is_active: boolean;
  license_name: string;
  license_url: string;
  attribution: string;
  repository_url: string;
  sync_status: string;
  content_hash: string;
  updated_at: string;
}

interface CoverageFacet {
  value: string;
  total: number;
  mirrored: number;
  published: number;
  pending_license: number;
  hidden: number;
  gap: number;
}

interface Coverage {
  target_per_facet: number;
  total_active: number;
  mirrored: number;
  unmirrored: number;
  published: number;
  pending_license: number;
  hidden: number;
  rejected_license: number;
  categories: CoverageFacet[];
  styles: CoverageFacet[];
  scenes: CoverageFacet[];
}

const PAGE_SIZE = 24;

const statusLabels: Record<ReferenceItem["status"], string> = {
  pending_license: "待许可证审核",
  published: "已发布",
  hidden: "已下线",
};

const licenseLabels: Record<ReferenceItem["asset_license_status"], string> = {
  unverified: "未验证",
  verified: "已验证",
  rejected: "已拒绝",
};

const batchActions = [
  { value: "verify_license", label: "验证许可证", icon: ShieldCheck },
  { value: "reject_license", label: "拒绝许可证", icon: X },
  { value: "publish", label: "批量发布", icon: Check },
  { value: "hide", label: "批量下线", icon: X },
  { value: "feature", label: "设为精选", icon: Check },
  { value: "unfeature", label: "取消精选", icon: X },
] as const;

function statusClass(status: ReferenceItem["status"]) {
  if (status === "published") return "bg-emerald-50 text-emerald-700";
  if (status === "hidden") return "bg-gray-100 text-gray-600";
  return "bg-amber-50 text-amber-700";
}

function licenseClass(status: ReferenceItem["asset_license_status"]) {
  if (status === "verified") return "bg-emerald-50 text-emerald-700";
  if (status === "rejected") return "bg-red-50 text-red-600";
  return "bg-amber-50 text-amber-700";
}

function shortDate(value?: string) {
  if (!value) return "-";
  return new Date(value).toLocaleString("zh-CN", { dateStyle: "short", timeStyle: "short" });
}

export default function ReferenceGalleryAdminPage() {
  const [items, setItems] = useState<ReferenceItem[]>([]);
  const [coverage, setCoverage] = useState<Coverage | null>(null);
  const [status, setStatus] = useState("");
  const [licenseStatus, setLicenseStatus] = useState("");
  const [query, setQuery] = useState("");
  const [queryDraft, setQueryDraft] = useState("");
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [detail, setDetail] = useState<ReferenceItem | null>(null);
  const [saving, setSaving] = useState(false);
  const [importing, setImporting] = useState(false);
  const [importPayload, setImportPayload] = useState<unknown>(null);
  const [importName, setImportName] = useState("");
  const [sourceKey, setSourceKey] = useState("github:freestylefly/awesome-gpt-image-2");
  const [repositoryURL, setRepositoryURL] = useState("https://github.com/freestylefly/awesome-gpt-image-2");
  const [manifestURL, setManifestURL] = useState("https://raw.githubusercontent.com/freestylefly/awesome-gpt-image-2/main/data/cases.json");
  const [licenseName, setLicenseName] = useState("MIT");
  const [licenseURL, setLicenseURL] = useState("https://raw.githubusercontent.com/freestylefly/awesome-gpt-image-2/main/LICENSE");
  const [attribution, setAttribution] = useState("freestylefly/awesome-gpt-image-2");

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    const params = new URLSearchParams({ page: String(page), page_size: String(PAGE_SIZE), active: "true" });
    if (status) params.set("status", status);
    if (licenseStatus) params.set("license_status", licenseStatus);
    if (query) params.set("q", query);
    try {
      const result = await adminApi<{ items: ReferenceItem[]; total: number }>(`/gallery/references?${params.toString()}`);
      setItems(result.items || []);
      setTotal(result.total || 0);
    } catch (err) {
      setError(err instanceof Error ? err.message : "参考案例加载失败");
    } finally {
      setLoading(false);
    }
  }, [licenseStatus, page, query, status]);

  const loadCoverage = useCallback(async () => {
    try {
      setCoverage(await adminApi<Coverage>("/gallery/references/coverage?target=8"));
    } catch (err) {
      setError(err instanceof Error ? err.message : "覆盖率加载失败");
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    void loadCoverage();
  }, [loadCoverage]);

  const selectedOnPage = useMemo(() => items.length > 0 && items.every((item) => selected.has(item.id)), [items, selected]);

  const refresh = () => {
    setSelected(new Set());
    void load();
    void loadCoverage();
  };

  const toggleAll = () => {
    setSelected((current) => {
      const next = new Set(current);
      if (selectedOnPage) items.forEach((item) => next.delete(item.id));
      else items.forEach((item) => next.add(item.id));
      return next;
    });
  };

  const runBatch = async (action: string) => {
    if (selected.size === 0) return;
    const actionLabel = batchActions.find((item) => item.value === action)?.label || action;
    if (!window.confirm(`确认对 ${selected.size} 个案例执行“${actionLabel}”吗？`)) return;
    try {
      await adminApi("/gallery/references/batch", { method: "POST", body: JSON.stringify({ ids: Array.from(selected), action }) });
      refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "批量操作失败");
    }
  };

  const openDetail = (item: ReferenceItem) => setDetail({ ...item });

  const saveDetail = async () => {
    if (!detail) return;
    setSaving(true);
    try {
      const result = await adminApi<ReferenceItem>(`/gallery/references/${detail.id}`, {
        method: "PATCH",
        body: JSON.stringify({
          title: detail.title,
          prompt: detail.prompt,
          category: detail.category,
          styles: detail.styles,
          scenes: detail.scenes,
          status: detail.status,
          asset_license_status: detail.asset_license_status,
          featured: detail.featured,
        }),
      });
      setDetail(result);
      refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "保存失败");
    } finally {
      setSaving(false);
    }
  };

  const handleImportFile = async (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file) return;
    try {
      setImportPayload(JSON.parse(await file.text()));
      setImportName(file.name);
      setError("");
    } catch {
      setImportPayload(null);
      setImportName("");
      setError("导入文件不是有效 JSON");
    }
    event.target.value = "";
  };

  const importJSON = async () => {
    if (!importPayload) return;
    setImporting(true);
    try {
      await adminApi("/gallery/references/import", {
        method: "POST",
        body: JSON.stringify({
          source_key: sourceKey,
          repository_url: repositoryURL,
          manifest_url: manifestURL,
          license_name: licenseName,
          license_url: licenseURL,
          attribution,
          manifest: importPayload,
        }),
      });
      setImportPayload(null);
      setImportName("");
      refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "导入失败");
    } finally {
      setImporting(false);
    }
  };

  const coverageGroups: Array<[string, CoverageFacet[] | undefined]> = [
    ["分类", coverage?.categories],
    ["风格", coverage?.styles],
    ["场景", coverage?.scenes],
  ];

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-950">参考案例运营</h1>
          <p className="mt-1 text-sm text-gray-500">管理来源快照、许可证审核和灵感广场公开内容。未完成许可证审核的案例不会公开。</p>
        </div>
        <button type="button" onClick={refresh} className="inline-flex h-10 items-center justify-center gap-2 rounded-xl border border-gray-200 bg-white px-4 text-sm font-medium text-gray-700 hover:bg-gray-50">
          <RefreshCw size={16} />刷新数据
        </button>
      </div>

      {error ? <div className="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">{error}</div> : null}

      <section className="grid grid-cols-2 gap-3 md:grid-cols-4 xl:grid-cols-8">
        {[
          ["有效案例", coverage?.total_active ?? 0, ""],
          ["已镜像", coverage?.mirrored ?? 0, "text-emerald-700"],
          ["未镜像", coverage?.unmirrored ?? 0, "text-amber-700"],
          ["已发布", coverage?.published ?? 0, "text-emerald-700"],
          ["待审核", coverage?.pending_license ?? 0, "text-amber-700"],
          ["已下线", coverage?.hidden ?? 0, "text-gray-600"],
          ["许可证拒绝", coverage?.rejected_license ?? 0, "text-red-600"],
          ["覆盖目标", coverage?.target_per_facet ?? 8, ""],
        ].map(([label, value, color]) => (
          <div key={String(label)} className="rounded-2xl border border-gray-200 bg-white p-4 shadow-sm shadow-gray-950/5">
            <div className="text-xs text-gray-500">{label}</div>
            <div className={`mt-2 text-2xl font-bold ${color}`}>{Number(value).toLocaleString("zh-CN")}</div>
          </div>
        ))}
      </section>

      <section className="grid gap-4 xl:grid-cols-[1fr_360px]">
        <div className="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm shadow-gray-950/5">
          <div className="mb-4 flex items-center justify-between gap-3">
            <div>
              <h2 className="font-semibold text-gray-950">覆盖率缺口</h2>
              <p className="mt-1 text-xs text-gray-400">按已发布案例统计，每个标签目标 {coverage?.target_per_facet ?? 8} 个。</p>
            </div>
            <span className="text-xs text-gray-400">分类 / 风格 / 场景</span>
          </div>
          <div className="grid gap-5 lg:grid-cols-3">
            {coverageGroups.map(([label, values]) => (
              <div key={String(label)}>
                <div className="mb-2 text-sm font-medium text-gray-700">{label}</div>
                <div className="max-h-52 space-y-2 overflow-y-auto pr-1">
                  {values?.map((facet) => (
                    <div key={facet.value} className="flex items-center justify-between gap-2 text-xs">
                      <span className="min-w-0 truncate text-gray-600" title={facet.value}>{facet.value}</span>
                      <span className={facet.gap > 0 ? "shrink-0 font-semibold text-amber-700" : "shrink-0 text-emerald-700"}>{facet.published}/{coverage?.target_per_facet ?? 8}</span>
                    </div>
                  ))}
                </div>
              </div>
            ))}
          </div>
        </div>

        <details className="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm shadow-gray-950/5">
          <summary className="cursor-pointer list-none font-semibold text-gray-950">导入来源快照</summary>
          <div className="mt-4 space-y-3">
            <label className="block text-xs text-gray-500">JSON 文件<input type="file" accept="application/json,.json" onChange={handleImportFile} className="mt-1 block w-full text-xs" /></label>
            {importName ? <div className="flex items-center gap-2 text-xs text-emerald-700"><FileJson size={14} />{importName}</div> : null}
            <input value={sourceKey} onChange={(event) => setSourceKey(event.target.value)} placeholder="source_key" className="h-9 w-full rounded-lg border border-gray-200 px-3 text-xs" />
            <input value={repositoryURL} onChange={(event) => setRepositoryURL(event.target.value)} placeholder="repository_url" className="h-9 w-full rounded-lg border border-gray-200 px-3 text-xs" />
            <input value={manifestURL} onChange={(event) => setManifestURL(event.target.value)} placeholder="manifest_url" className="h-9 w-full rounded-lg border border-gray-200 px-3 text-xs" />
            <div className="grid grid-cols-2 gap-2"><input value={licenseName} onChange={(event) => setLicenseName(event.target.value)} placeholder="许可证" className="h-9 rounded-lg border border-gray-200 px-3 text-xs" /><input value={attribution} onChange={(event) => setAttribution(event.target.value)} placeholder="署名" className="h-9 rounded-lg border border-gray-200 px-3 text-xs" /></div>
            <input value={licenseURL} onChange={(event) => setLicenseURL(event.target.value)} placeholder="license_url" className="h-9 w-full rounded-lg border border-gray-200 px-3 text-xs" />
            <button type="button" disabled={!importPayload || importing} onClick={importJSON} className="inline-flex h-9 w-full items-center justify-center gap-2 rounded-lg bg-gray-950 text-xs font-semibold text-white disabled:opacity-40"><UploadCloud size={14} />{importing ? "导入中..." : "导入并进入待审核"}</button>
            <p className="text-[11px] leading-5 text-gray-400">导入只更新本地快照并置为待审核，不会自动镜像资源或发布。</p>
          </div>
        </details>
      </section>

      <section className="rounded-2xl border border-gray-200 bg-white shadow-sm shadow-gray-950/5">
        <div className="flex flex-col gap-3 border-b border-gray-100 p-4 lg:flex-row lg:items-center">
          <div className="relative min-w-0 flex-1"><Search size={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" /><input value={queryDraft} onChange={(event) => setQueryDraft(event.target.value)} onKeyDown={(event) => { if (event.key === "Enter") { setPage(1); setQuery(queryDraft.trim()); } }} placeholder="搜索标题、Prompt、来源 ID..." className="h-10 w-full rounded-xl border border-gray-200 pl-9 pr-3 text-sm outline-none focus:border-gray-950" /></div>
          <select value={status} onChange={(event) => { setPage(1); setStatus(event.target.value); }} className="h-10 rounded-xl border border-gray-200 px-3 text-sm text-gray-600"><option value="">全部状态</option><option value="pending_license">待许可证审核</option><option value="published">已发布</option><option value="hidden">已下线</option></select>
          <select value={licenseStatus} onChange={(event) => { setPage(1); setLicenseStatus(event.target.value); }} className="h-10 rounded-xl border border-gray-200 px-3 text-sm text-gray-600"><option value="">全部许可证</option><option value="unverified">未验证</option><option value="verified">已验证</option><option value="rejected">已拒绝</option></select>
          <button type="button" onClick={() => { setPage(1); setQuery(queryDraft.trim()); }} className="inline-flex h-10 items-center justify-center gap-2 rounded-xl bg-gray-950 px-4 text-sm font-semibold text-white"><Search size={15} />搜索</button>
        </div>
        <div className="flex flex-wrap items-center gap-2 border-b border-gray-100 px-4 py-3">
          <label className="inline-flex items-center gap-2 text-xs text-gray-500"><input type="checkbox" checked={selectedOnPage} onChange={toggleAll} />全选本页</label>
          <span className="text-xs text-gray-400">已选 {selected.size} 条</span>
          {batchActions.map((action) => { const Icon = action.icon; return <button key={action.value} type="button" disabled={selected.size === 0} onClick={() => void runBatch(action.value)} className="inline-flex h-8 items-center gap-1.5 rounded-lg border border-gray-200 px-2.5 text-xs text-gray-600 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-40"><Icon size={13} />{action.label}</button>; })}
        </div>
        {loading ? <div className="py-20 text-center text-sm text-gray-400">加载中...</div> : items.length === 0 ? <div className="py-20 text-center text-sm text-gray-400">暂无匹配的参考案例</div> : (
          <div className="divide-y divide-gray-100">
            {items.map((item) => (
              <div key={item.id} className="flex flex-col gap-3 p-4 lg:flex-row lg:items-center">
                <input type="checkbox" checked={selected.has(item.id)} onChange={() => setSelected((current) => { const next = new Set(current); if (next.has(item.id)) next.delete(item.id); else next.add(item.id); return next; })} />
                {item.image_asset_url ? <img src={item.image_asset_url} alt={item.image_alt || item.title} className="h-16 w-16 shrink-0 rounded-xl bg-gray-100 object-cover" /> : <div className="grid h-16 w-16 shrink-0 place-items-center rounded-xl bg-gray-100 text-gray-400"><ImageIcon size={20} /></div>}
                <div className="min-w-0 flex-1"><div className="flex flex-wrap items-center gap-2"><button type="button" onClick={() => openDetail(item)} className="truncate text-left text-sm font-semibold text-gray-900 hover:text-gray-950 hover:underline">{item.title}</button><span className={`rounded-full px-2 py-0.5 text-[10px] ${statusClass(item.status)}`}>{statusLabels[item.status]}</span><span className={`rounded-full px-2 py-0.5 text-[10px] ${licenseClass(item.asset_license_status)}`}>{licenseLabels[item.asset_license_status]}</span>{item.featured ? <span className="rounded-full bg-orange-50 px-2 py-0.5 text-[10px] text-orange-700">精选</span> : null}</div><div className="mt-1 line-clamp-1 text-xs text-gray-500">{item.category} · {item.source_key} · #{item.source_item_id}</div><div className="mt-1 line-clamp-1 text-xs text-gray-400">{item.prompt_preview || item.prompt}</div></div>
                <div className="flex shrink-0 items-center gap-2 text-xs"><span className="text-gray-400">{shortDate(item.updated_at)}</span>{item.source_url ? <a href={item.source_url} target="_blank" rel="noreferrer" title="打开来源" className="text-gray-400 hover:text-gray-950"><ExternalLink size={15} /></a> : null}<button type="button" onClick={() => openDetail(item)} className="rounded-lg border border-gray-200 px-3 py-1.5 font-medium text-gray-600 hover:bg-gray-50">审核详情</button></div>
              </div>
            ))}
          </div>
        )}
        <div className="px-4 pb-4"><AdminPagination page={page} total={total} pageSize={PAGE_SIZE} onPageChange={setPage} /></div>
      </section>

      {detail ? <ReferenceDetail item={detail} saving={saving} onChange={setDetail} onClose={() => setDetail(null)} onSave={() => void saveDetail()} /> : null}
    </div>
  );
}

function ReferenceDetail({ item, saving, onChange, onClose, onSave }: { item: ReferenceItem; saving: boolean; onChange: (item: ReferenceItem) => void; onClose: () => void; onSave: () => void }) {
  return <div className="fixed inset-0 z-50 flex items-center justify-center bg-gray-950/50 p-4" onMouseDown={(event) => { if (event.currentTarget === event.target) onClose(); }}>
    <div className="max-h-[92vh] w-full max-w-5xl overflow-y-auto rounded-2xl bg-white shadow-2xl">
      <div className="flex items-center justify-between border-b border-gray-100 px-5 py-4"><div><h2 className="font-semibold text-gray-950">参考案例审核详情</h2><p className="mt-1 text-xs text-gray-400">#{item.source_item_id} · {item.source_key}</p></div><button type="button" onClick={onClose} className="grid h-9 w-9 place-items-center rounded-lg text-gray-500 hover:bg-gray-100"><X size={18} /></button></div>
      <div className="grid gap-5 p-5 lg:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]">
        <div>{item.image_asset_url ? <img src={item.image_asset_url} alt={item.image_alt || item.title} className="max-h-[62vh] w-full rounded-xl bg-gray-100 object-contain" /> : <div className="grid aspect-[4/5] place-items-center rounded-xl bg-gray-100 text-sm text-gray-400">尚未镜像资源</div>}<div className="mt-3 flex flex-wrap gap-2 text-xs"><span className={`rounded-full px-2 py-1 ${licenseClass(item.asset_license_status)}`}>{licenseLabels[item.asset_license_status]}</span><span className={`rounded-full px-2 py-1 ${statusClass(item.status)}`}>{statusLabels[item.status]}</span>{item.source_url ? <a href={item.source_url} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1 rounded-full border border-gray-200 px-2 py-1 text-gray-600">来源 <ExternalLink size={12} /></a> : null}{item.github_url ? <a href={item.github_url} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1 rounded-full border border-gray-200 px-2 py-1 text-gray-600">GitHub <ExternalLink size={12} /></a> : null}</div></div>
        <div className="space-y-4"><label className="block text-xs font-medium text-gray-500">标题<input value={item.title} onChange={(event) => onChange({ ...item, title: event.target.value })} className="mt-1 h-10 w-full rounded-lg border border-gray-200 px-3 text-sm text-gray-900" /></label><label className="block text-xs font-medium text-gray-500">分类<input value={item.category} onChange={(event) => onChange({ ...item, category: event.target.value })} className="mt-1 h-10 w-full rounded-lg border border-gray-200 px-3 text-sm text-gray-900" /></label><div className="grid grid-cols-2 gap-3"><label className="block text-xs font-medium text-gray-500">内容状态<select value={item.status} onChange={(event) => onChange({ ...item, status: event.target.value as ReferenceItem["status"] })} className="mt-1 h-10 w-full rounded-lg border border-gray-200 px-3 text-sm text-gray-700"><option value="pending_license">待许可证审核</option><option value="published">已发布</option><option value="hidden">已下线</option></select></label><label className="block text-xs font-medium text-gray-500">许可证<select value={item.asset_license_status} onChange={(event) => onChange({ ...item, asset_license_status: event.target.value as ReferenceItem["asset_license_status"] })} className="mt-1 h-10 w-full rounded-lg border border-gray-200 px-3 text-sm text-gray-700"><option value="unverified">未验证</option><option value="verified">已验证</option><option value="rejected">已拒绝</option></select></label></div><label className="inline-flex items-center gap-2 text-sm text-gray-700"><input type="checkbox" checked={item.featured} onChange={(event) => onChange({ ...item, featured: event.target.checked })} />设为精选</label><label className="block text-xs font-medium text-gray-500">Prompt<textarea value={item.prompt} onChange={(event) => onChange({ ...item, prompt: event.target.value })} rows={12} className="mt-1 w-full rounded-lg border border-gray-200 p-3 text-sm leading-6 text-gray-900" /></label><div className="grid grid-cols-2 gap-3"><label className="block text-xs font-medium text-gray-500">风格（逗号分隔）<input value={item.styles.join(", ")} onChange={(event) => onChange({ ...item, styles: event.target.value.split(",").map((value) => value.trim()).filter(Boolean) })} className="mt-1 h-10 w-full rounded-lg border border-gray-200 px-3 text-sm" /></label><label className="block text-xs font-medium text-gray-500">场景（逗号分隔）<input value={item.scenes.join(", ")} onChange={(event) => onChange({ ...item, scenes: event.target.value.split(",").map((value) => value.trim()).filter(Boolean) })} className="mt-1 h-10 w-full rounded-lg border border-gray-200 px-3 text-sm" /></label></div><div className="rounded-xl bg-gray-50 p-3 text-xs leading-5 text-gray-500">许可证：{item.license_name} · {item.attribution}<br />内容 hash：{item.content_hash}<br />同步状态：{item.sync_status} · 最近更新：{shortDate(item.updated_at)}</div><div className="flex justify-end gap-2"><button type="button" onClick={onClose} className="h-10 rounded-lg border border-gray-200 px-4 text-sm text-gray-600">取消</button><button type="button" disabled={saving} onClick={onSave} className="h-10 rounded-lg bg-gray-950 px-5 text-sm font-semibold text-white disabled:opacity-50">{saving ? "保存中..." : "保存审核结果"}</button></div></div>
      </div>
    </div>
  </div>;
}

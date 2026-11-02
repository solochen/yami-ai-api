"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  AlertTriangle,
  ArrowDown,
  ArrowRight,
  ArrowUp,
  Check,
  ChevronsUpDown,
  Database,
  History,
  RefreshCw,
  TrendingUp,
  X,
} from "lucide-react";
import { adminApi } from "@/lib/api";
import { AdminPagination } from "@/components/AdminPagination";

type BillingType = "per_token" | "per_image" | "per_second" | "per_request" | string;

interface PriceCost {
  billing_type: BillingType;
  input: number;
  output: number;
  cache_read: number;
  cache_write: number;
  unit: number;
}

interface RouteBrief {
  route_id: number;
  route_name: string;
  provider: string;
  upstream_model: string;
  price_group: string;
  priority: number;
  health_status: string;
  cost: PriceCost;
  cost_source: string;
  observed_at?: string;
  is_stale: boolean;
  user_cost?: PriceCost;
}

interface PotentialPrice {
  provider: string;
  upstream_model: string;
  group_name: string;
  cost: PriceCost;
  observed_at: string;
}

interface Usage {
  input_tokens: number;
  output_tokens: number;
  cache_read_tokens: number;
  cache_write_tokens: number;
  image_count: number;
  video_seconds: number;
  request_count: number;
  provider_cost: number;
}

interface AnalysisRow {
  model_id: number;
  code: string;
  display_name: string;
  primary: RouteBrief;
  cheapest: RouteBrief;
  potential?: PotentialPrice;
  gap_percent: number;
  daily_saving: number;
  monthly_saving: number;
  usage: Usage;
  route_count: number;
  has_user_price: boolean;
  monthly_revenue: number;
  monthly_cost: number;
  monthly_profit: number;
  status: string;
  margin_percent?: number;
  backups: RouteBrief[];
}

interface Provider {
  provider: string;
  display_name: string;
  source_type: string;
  price_endpoint: string;
  currency: string;
  recharge_rate: number;
  usd_cny_rate: number;
  is_enabled: boolean;
  last_sync_at?: string;
  last_status: string;
  consecutive_failures: number;
}

interface SyncRun {
  id: number;
  trigger: string;
  status: string;
  provider_total: number;
  provider_succeeded: number;
  provider_failed: number;
  items_total: number;
  items_matched: number;
  error_summary: string;
  started_at: string;
  finished_at?: string;
  providers?: SyncProviderRun[];
}

interface SyncProviderRun {
  id: number;
  provider: string;
  status: string;
  http_status: number;
  items: number;
  latency_ms: number;
  attempts: number;
  error: string;
}

interface CatalogRow {
  provider: string;
  upstream_model: string;
  group_name: string;
  billing_type: string;
  source_currency: string;
  source_unit: string;
  input_price: number;
  output_price: number;
  unit_price: number;
  billing_multiplier: number;
  fx_rate: number;
  cny_input_per_m: number;
  cny_output_per_m: number;
  cny_unit_price: number;
  source_type: string;
  is_trusted: boolean;
  observed_at: string;
  is_stale: boolean;
  model_id?: number;
  model_code?: string;
  model_name?: string;
}

interface MappingRow {
  provider: string;
  upstream_model: string;
  model_id?: number;
  model_code: string;
  model_name: string;
  match_method: string;
  confidence: number;
  is_confirmed: boolean;
}

interface ModelOption {
  id: number;
  code: string;
  display_name: string;
}

interface SwitchLog {
  id: number;
  model_id: number;
  model_code?: string;
  from_route_id?: number;
  to_route_id: number;
  from_route_name?: string;
  to_route_name?: string;
  reason: string;
  estimated_daily_saving: number;
  estimated_monthly_saving: number;
  created_at: string;
  rolled_back_at?: string;
}

const PAGE_SIZE = 20;
const CATALOG_PAGE_SIZE = 50;

type SortKey = "model" | "cost" | "gap" | "usage" | "saving" | "profit" | "status";

const STATUS_RANK: Record<string, number> = {
  loss: 0,
  low_margin: 1,
  stale: 2,
  not_lowest: 3,
  missing_price: 4,
  ok: 5,
};

function sortCost(row: AnalysisRow) {
  const c = row.primary.cost;
  return c.billing_type === "per_token" ? c.input + c.output : c.unit;
}

function sortUsage(row: AnalysisRow) {
  if (row.primary.cost.billing_type === "per_token") return row.usage.input_tokens + row.usage.output_tokens;
  if (row.primary.cost.billing_type === "per_image") return row.usage.image_count;
  if (row.primary.cost.billing_type === "per_second") return row.usage.video_seconds;
  return row.usage.request_count;
}

function money(v?: number, digits = 3) {
  if (v === undefined || v === null || Number.isNaN(v)) return "—";
  const fixed = Number(v).toFixed(digits).replace(/\.?0+$/, "");
  return `¥${fixed || "0"}`;
}

function pct(v?: number) {
  if (v === undefined || v === null || Number.isNaN(v)) return "—";
  const sign = v > 0 ? "+" : "";
  return `${sign}${v.toFixed(1)}%`;
}

function costText(c: PriceCost) {
  if (c.billing_type === "per_token") return `${money(c.input)} / ${money(c.output)}`;
  return `${money(c.unit)}/${c.billing_type === "per_image" ? "张" : c.billing_type === "per_second" ? "秒" : "次"}`;
}

function sourceLabel(source: string) {
  if (source.startsWith("hub:")) return `网关真实成本 · ${source.slice(4)}`;
  if (source === "sync") return "同步价";
  if (source === "config") return "配置价";
  return source;
}

function statusBadge(status: string) {
  const map: Record<string, { label: string; cls: string }> = {
    ok: { label: "最优", cls: "bg-emerald-50 text-emerald-700 border-emerald-200" },
    not_lowest: { label: "可优化", cls: "bg-amber-50 text-amber-700 border-amber-200" },
    stale: { label: "价格过期", cls: "bg-orange-50 text-orange-700 border-orange-200" },
    loss: { label: "亏损风险", cls: "bg-red-50 text-red-700 border-red-200" },
    missing_price: { label: "缺少价格", cls: "bg-gray-100 text-gray-500 border-gray-200" },
    low_margin: { label: "低毛利", cls: "bg-orange-50 text-orange-700 border-orange-200" },
  };
  const item = map[status] || { label: status, cls: "bg-gray-50 text-gray-600 border-gray-200" };
  return <span className={`inline-flex items-center rounded-full border px-2 py-0.5 text-[11px] font-medium ${item.cls}`}>{item.label}</span>;
}

function providerStatus(status: string) {
  if (status === "success") return <span className="text-emerald-600">正常</span>;
  if (status === "partial") return <span className="text-amber-600">部分失败</span>;
  if (status === "failed") return <span className="text-red-600">失败</span>;
  if (status === "running") return <span className="text-blue-600">运行中</span>;
  return <span className="text-gray-400">未同步</span>;
}

function providerCardStatus(p: Provider) {
  if (!p.is_enabled) return <span className="text-gray-400">已停用</span>;
  if (p.last_status === "never") {
    if (p.source_type === "manual" || p.source_type === "official_page") {
      return <span className="text-gray-400" title="该厂商暂无稳定公开价格接口，使用线路 cost_rule 中的配置价">手工维护</span>;
    }
    return <span className="text-gray-400">未同步</span>;
  }
  return providerStatus(p.last_status);
}

function providerSourceLabel(sourceType: string) {
  const map: Record<string, string> = {
    new_api: "New API 价格接口",
    new_api_hub: "网关真实上游",
    openrouter: "OpenRouter 价格接口",
    codexauv_page: "CodexAuv 模型广场",
    official_page: "官方价格页（手工）",
    manual: "手工维护",
  };
  return map[sourceType] || sourceType;
}

export default function UpstreamPricesPage() {
  const [tab, setTab] = useState<"analysis" | "overview" | "catalog" | "sync" | "mappings" | "switches">("analysis");
  const [rows, setRows] = useState<AnalysisRow[]>([]);
  const [providers, setProviders] = useState<Provider[]>([]);
  const [runs, setRuns] = useState<SyncRun[]>([]);
  const [catalog, setCatalog] = useState<CatalogRow[]>([]);
  const [catalogTotal, setCatalogTotal] = useState(0);
  const [catalogPage, setCatalogPage] = useState(1);
  const [catalogProvider, setCatalogProvider] = useState("");
  const [catalogQuery, setCatalogQuery] = useState("");
  const [catalogTrusted, setCatalogTrusted] = useState(true);
  const [mappings, setMappings] = useState<MappingRow[]>([]);
  const [models, setModels] = useState<ModelOption[]>([]);
  const [switches, setSwitches] = useState<SwitchLog[]>([]);
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState("all");
  const [providerFilter, setProviderFilter] = useState("all");
  const [sortKey, setSortKey] = useState<SortKey>("saving");
  const [sortDir, setSortDir] = useState<"asc" | "desc">("desc");
  const [loading, setLoading] = useState(true);
  const [syncing, setSyncing] = useState(false);
  const [syncRun, setSyncRun] = useState<SyncRun | null>(null);
  const [error, setError] = useState("");
  const [promote, setPromote] = useState<{
    row: AnalysisRow;
    target: RouteBrief;
    daily: number;
    monthly: number;
  } | null>(null);
  const [promoteConfirm, setPromoteConfirm] = useState(false);
  const [promoteBusy, setPromoteBusy] = useState(false);
  const [applyingCost, setApplyingCost] = useState(false);

  const loadAnalysis = useCallback(async () => {
    const res = await adminApi<{ items: AnalysisRow[] }>("/upstream-prices/analysis");
    setRows(res.items || []);
  }, []);

  const loadProviders = useCallback(async () => {
    const res = await adminApi<Provider[]>("/upstream-prices/providers");
    setProviders(res || []);
  }, []);

  const loadRuns = useCallback(async () => {
    const res = await adminApi<{ items: SyncRun[] }>("/upstream-prices/sync-runs?page=1&page_size=20");
    setRuns(res.items || []);
  }, []);

  const loadModels = useCallback(async () => {
    const res = await adminApi<ModelOption[]>("/models");
    setModels(res || []);
  }, []);

  const loadCatalog = useCallback(async () => {
    const sp = new URLSearchParams({
      page: String(catalogPage),
      page_size: String(CATALOG_PAGE_SIZE),
      trusted: catalogTrusted ? "1" : "0",
    });
    if (catalogProvider) sp.set("provider", catalogProvider);
    if (catalogQuery.trim()) sp.set("q", catalogQuery.trim());
    const res = await adminApi<{ items: CatalogRow[]; total: number }>(`/upstream-prices/catalog?${sp.toString()}`);
    setCatalog(res.items || []);
    setCatalogTotal(res.total || 0);
  }, [catalogPage, catalogProvider, catalogQuery, catalogTrusted]);

  const loadMappings = useCallback(async () => {
    const res = await adminApi<{ items: MappingRow[] }>("/upstream-model-mappings?unmatched=1");
    setMappings(res.items || []);
  }, []);

  const loadSwitches = useCallback(async () => {
    const res = await adminApi<{ items: SwitchLog[] }>("/model-route-switches?page=1&page_size=50");
    setSwitches(res.items || []);
  }, []);

  const loadAll = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      await Promise.all([loadAnalysis(), loadProviders(), loadRuns(), loadModels()]);
    } catch (err) {
      setError(err instanceof Error ? err.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }, [loadAnalysis, loadModels, loadProviders, loadRuns]);

  useEffect(() => {
    loadAll();
  }, [loadAll]);

  useEffect(() => {
    if (tab === "catalog") loadCatalog().catch((err) => setError(err instanceof Error ? err.message : "目录加载失败"));
  }, [tab, loadCatalog]);

  useEffect(() => {
    if (tab === "mappings") loadMappings().catch((err) => setError(err instanceof Error ? err.message : "映射加载失败"));
    if (tab === "switches") loadSwitches().catch((err) => setError(err instanceof Error ? err.message : "切换记录加载失败"));
  }, [tab, loadMappings, loadSwitches]);

  const pollSync = useCallback(
    async (runId: number) => {
      for (let i = 0; i < 120; i += 1) {
        try {
          const run = await adminApi<SyncRun>(`/upstream-prices/sync-runs/${runId}`);
          setSyncRun(run);
          if (run.status !== "running") {
            setSyncing(false);
            await Promise.all([loadAnalysis(), loadProviders(), loadRuns()]);
            if (tab === "catalog") await loadCatalog();
            return;
          }
        } catch (err) {
          // 批次刚创建时可能有一瞬间查不到，继续轮询，不立刻中断。
          if (i > 4) throw err;
        }
        await new Promise((resolve) => setTimeout(resolve, 2000));
      }
      setSyncing(false);
      setError("同步超时，请到同步记录查看结果");
    },
    [loadAnalysis, loadCatalog, loadProviders, loadRuns, tab],
  );

  const startSync = async () => {
    if (syncing) return;
    setSyncing(true);
    setError("");
    try {
      const res = await adminApi<{ run_id: number }>("/upstream-prices/sync", {
        method: "POST",
        body: JSON.stringify({}),
      });
      await pollSync(res.run_id);
    } catch (err) {
      setSyncing(false);
      setError(err instanceof Error ? err.message : "同步失败");
    }
  };

  const openPromote = async (row: AnalysisRow, target: RouteBrief) => {
    if (target.route_id === row.primary.route_id) return;
    try {
      const res = await adminApi<{ target: RouteBrief; daily_saving: number; monthly_saving: number }>(
        `/models/${row.model_id}/routes/${target.route_id}/promote-preview`,
      );
      setPromote({ row, target: res.target || target, daily: res.daily_saving, monthly: res.monthly_saving });
      setPromoteConfirm(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : "无法预览切换");
    }
  };

  const confirmPromote = async () => {
    if (!promote || !promoteConfirm || promoteBusy) return;
    setPromoteBusy(true);
    try {
      await adminApi(`/models/${promote.row.model_id}/routes/${promote.target.route_id}/promote`, {
        method: "POST",
        body: JSON.stringify({ confirm: true, reason: "manual" }),
      });
      setPromote(null);
      await loadAnalysis();
    } catch (err) {
      setError(err instanceof Error ? err.message : "切换失败");
    } finally {
      setPromoteBusy(false);
    }
  };

  const applyRealCost = async (row: AnalysisRow) => {
    if (applyingCost) return;
    if (!confirm(`确认用网关真实成本更新「${row.display_name || row.code}」当前主线路的 cost_rule？`)) return;
    setApplyingCost(true);
    try {
      await adminApi("/upstream-prices/apply", {
        method: "POST",
        body: JSON.stringify({ model_id: row.model_id, route_id: row.primary.route_id }),
      });
      await loadAnalysis();
    } catch (err) {
      setError(err instanceof Error ? err.message : "应用真实成本失败");
    } finally {
      setApplyingCost(false);
    }
  };

  const rollback = async (id: number) => {
    if (!confirm("确认回滚这次切换？会恢复切换前的线路优先级。")) return;
    try {
      await adminApi(`/model-route-switches/${id}/rollback`, { method: "POST", body: JSON.stringify({}) });
      await Promise.all([loadAnalysis(), loadSwitches()]);
    } catch (err) {
      setError(err instanceof Error ? err.message : "回滚失败");
    }
  };

  const updateMapping = async (row: MappingRow, modelID: number) => {
    if (!modelID) return;
    try {
      await adminApi("/upstream-model-mappings", {
        method: "PATCH",
        body: JSON.stringify({ provider: row.provider, upstream_model: row.upstream_model, model_id: modelID }),
      });
      await loadMappings();
    } catch (err) {
      setError(err instanceof Error ? err.message : "映射更新失败");
    }
  };

  const filteredRows = useMemo(() => {
    return rows.filter((r) => {
      if (search.trim()) {
        const q = search.trim().toLowerCase();
        if (!r.code.toLowerCase().includes(q) && !r.display_name.toLowerCase().includes(q)) return false;
      }
      if (statusFilter !== "all" && r.status !== statusFilter) return false;
      if (providerFilter !== "all" && r.primary.provider !== providerFilter && !r.backups.some((b) => b.provider === providerFilter)) return false;
      return true;
    });
  }, [rows, search, statusFilter, providerFilter]);

  const sortedRows = useMemo(() => {
    const list = [...filteredRows];
    const dir = sortDir === "asc" ? 1 : -1;
    list.sort((a, b) => {
      let diff = 0;
      if (sortKey === "model") {
        diff = `${a.code} ${a.display_name}`.localeCompare(`${b.code} ${b.display_name}`);
      } else if (sortKey === "cost") {
        diff = sortCost(a) - sortCost(b);
      } else if (sortKey === "gap") {
        diff = a.gap_percent - b.gap_percent;
      } else if (sortKey === "usage") {
        diff = sortUsage(a) - sortUsage(b);
      } else if (sortKey === "saving") {
        diff = a.monthly_saving - b.monthly_saving;
      } else if (sortKey === "profit") {
        diff = a.monthly_profit - b.monthly_profit;
      } else if (sortKey === "status") {
        diff = (STATUS_RANK[a.status] ?? 99) - (STATUS_RANK[b.status] ?? 99);
      }
      if (diff === 0) diff = a.code.localeCompare(b.code);
      return diff * dir;
    });
    return list;
  }, [filteredRows, sortDir, sortKey]);

  const toggleSort = (key: SortKey) => {
    if (sortKey === key) {
      setSortDir((prev) => (prev === "asc" ? "desc" : "asc"));
    } else {
      setSortKey(key);
      setSortDir(key === "model" ? "asc" : "desc");
    }
  };

  const sortTh = (label: string, key: SortKey) => (
    <th className="px-4 py-3">
      <button
        type="button"
        onClick={() => toggleSort(key)}
        className={`inline-flex items-center gap-1 ${sortKey === key ? "text-gray-900" : "text-gray-500"} hover:text-gray-900`}
        title={`按${label}排序`}
      >
        {label}
        {sortKey === key ? (
          sortDir === "asc" ? <ArrowUp className="h-3 w-3" /> : <ArrowDown className="h-3 w-3" />
        ) : (
          <ChevronsUpDown className="h-3 w-3 text-gray-300" />
        )}
      </button>
    </th>
  );

  const stats = useMemo(() => {
    const monthly = rows.reduce((sum, r) => sum + Math.max(0, r.monthly_saving), 0);
    return {
      total: rows.length,
      optimizable: rows.filter((r) => r.status === "not_lowest" || r.status === "loss").length,
      monthly,
      profit: rows.reduce((sum, r) => sum + (r.has_user_price ? r.monthly_profit : 0), 0),
      loss: rows.filter((r) => r.status === "loss" || r.status === "low_margin").length,
    };
  }, [rows]);

  const lastSync = useMemo(() => {
    const list = providers.filter((p) => p.last_sync_at).map((p) => p.last_sync_at as string).sort();
    return list.length ? list[list.length - 1] : "";
  }, [providers]);

  const providerOptions = useMemo(() => Array.from(new Set(rows.flatMap((r) => [r.primary.provider, ...r.backups.map((b) => b.provider)]))), [rows]);
  const stale = !lastSync || Date.now() - new Date(lastSync).getTime() > 24 * 60 * 60 * 1000;

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">上游价格监控</h1>
          <p className="mt-1 text-sm text-gray-500">
            按线路实际分组成本排序；价格只在点击「同步」时刷新。
            {lastSync ? ` 最近同步：${new Date(lastSync).toLocaleString("zh-CN")}` : " 尚未同步"}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={startSync}
            disabled={syncing}
            className="inline-flex items-center gap-2 rounded-xl bg-gray-950 px-4 py-2 text-sm font-semibold text-white disabled:opacity-50"
          >
            <RefreshCw className={`h-4 w-4 ${syncing ? "animate-spin" : ""}`} />
            {syncing ? "同步中…" : "同步"}
          </button>
        </div>
      </div>

      {error && (
        <div className="flex items-start gap-3 rounded-2xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
          <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
          <span className="flex-1">{error}</span>
          <button type="button" onClick={() => setError("")} className="text-red-400 hover:text-red-600">
            <X className="h-4 w-4" />
          </button>
        </div>
      )}

      {stale && (
        <div className="flex items-start gap-3 rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800">
          <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
          <span>价格已超过 24 小时未同步，当前排序可能过期。建议先点右上角「同步」再切换线路。</span>
        </div>
      )}

      {syncRun && (
        <div className="rounded-2xl border bg-white p-4">
          <div className="mb-3 flex items-center justify-between">
            <div className="flex items-center gap-2 text-sm font-semibold text-gray-900">
              <RefreshCw className={`h-4 w-4 ${syncRun.status === "running" ? "animate-spin text-blue-600" : "text-gray-400"}`} />
              同步批次 #{syncRun.id} · {syncRun.status}
            </div>
            <span className="text-xs text-gray-400">
              {syncRun.provider_succeeded}/{syncRun.provider_total} 厂商成功 · {syncRun.items_total} 条
            </span>
          </div>
          <div className="grid gap-2 md:grid-cols-2 xl:grid-cols-3">
            {(syncRun.providers || []).map((p) => (
              <div key={p.id} className="flex items-center justify-between rounded-xl border border-gray-100 bg-gray-50 px-3 py-2 text-xs">
                <span className="font-medium text-gray-700">{p.provider}</span>
                <span className="flex items-center gap-2">
                  {providerStatus(p.status)}
                  <span className="text-gray-400">{p.items} 条 · {p.latency_ms}ms</span>
                </span>
              </div>
            ))}
          </div>
        </div>
      )}

      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-5">
        {[
          { label: "监控模型", value: stats.total, suffix: "个" },
          { label: "可优化", value: stats.optimizable, suffix: "个" },
          { label: "换线可省", value: money(stats.monthly, 2), suffix: "" },
          { label: "近30天毛利", value: money(stats.profit, 2), suffix: "" },
          { label: "亏损/低毛利", value: stats.loss, suffix: "个" },
        ].map((card) => (
          <div key={card.label} className="rounded-2xl border bg-white p-4">
            <div className="text-xs text-gray-500">{card.label}</div>
            <div className="mt-1 text-xl font-bold text-gray-900">
              {card.value}
              <span className="ml-1 text-xs font-normal text-gray-400">{card.suffix}</span>
            </div>
          </div>
        ))}
      </div>

      <div className="flex flex-wrap gap-2 border-b border-gray-200 pb-2">
        {[
          { key: "analysis", label: "选路分析", icon: TrendingUp },
          { key: "overview", label: "主备总览", icon: Database },
          { key: "catalog", label: "价格目录", icon: Database },
          { key: "sync", label: "同步记录", icon: RefreshCw },
          { key: "mappings", label: "映射管理", icon: Check },
          { key: "switches", label: "切换记录", icon: History },
        ].map((item) => {
          const Icon = item.icon;
          return (
            <button
              key={item.key}
              type="button"
              onClick={() => setTab(item.key as typeof tab)}
              className={`inline-flex items-center gap-2 rounded-xl px-3 py-2 text-sm font-medium ${
                tab === item.key ? "bg-gray-950 text-white" : "text-gray-600 hover:bg-gray-100"
              }`}
            >
              <Icon className="h-4 w-4" />
              {item.label}
            </button>
          );
        })}
      </div>

      {(tab === "analysis" || tab === "overview") && (
        <div className="space-y-3">
          <div className="flex flex-col gap-2 md:flex-row md:items-center">
            <input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="搜索模型 code / 名称"
              className="w-full rounded-xl border px-3 py-2 text-sm md:w-72"
            />
            <select value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)} className="rounded-xl border px-3 py-2 text-sm">
              <option value="all">全部状态</option>
              <option value="not_lowest">可优化</option>
              <option value="loss">亏损</option>
              <option value="low_margin">低毛利</option>
              <option value="stale">价格过期</option>
              <option value="ok">最优</option>
            </select>
            <select value={providerFilter} onChange={(e) => setProviderFilter(e.target.value)} className="rounded-xl border px-3 py-2 text-sm">
              <option value="all">全部厂商</option>
              {providerOptions.map((p) => (
                <option key={p} value={p}>{p}</option>
              ))}
            </select>
          </div>

          {tab === "analysis" ? (
            <div className="overflow-x-auto rounded-2xl border bg-white">
              <table className="min-w-full text-sm">
                <thead className="bg-gray-50 text-left text-xs text-gray-500">
                  <tr>
                    {sortTh("模型", "model")}
                    <th className="px-4 py-3">当前主线路</th>
                    {sortTh("当前成本", "cost")}
                    <th className="px-4 py-3">最低同型线路</th>
                    {sortTh("差额", "gap")}
                    {sortTh("近30天用量", "usage")}
                    {sortTh("预计月省（换线）", "saving")}
                    {sortTh("盈利状况", "profit")}
                    {sortTh("状态", "status")}
                    <th className="px-4 py-3 text-right">操作</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-100">
                  {sortedRows.map((row) => (
                    <tr key={row.model_id} className="align-top hover:bg-gray-50/60">
                      <td className="px-4 py-3">
                        <div className="font-medium text-gray-900">{row.display_name || row.code}</div>
                        <div className="mt-0.5 font-mono text-[11px] text-gray-400">{row.code}</div>
                      </td>
                      <td className="px-4 py-3">
                        <div className="font-medium text-gray-800">{row.primary.provider}</div>
                        <div className="text-xs text-gray-500">{row.primary.upstream_model}{row.primary.price_group ? ` · ${row.primary.price_group}` : ""}</div>
                        <div className="mt-0.5 text-[11px] text-gray-400">priority {row.primary.priority}</div>
                      </td>
                      <td className="px-4 py-3 font-mono text-xs text-gray-700">
                        {costText(row.primary.cost)}
                        <div className="mt-0.5 font-sans text-[10px] font-normal text-gray-400">{sourceLabel(row.primary.cost_source)}</div>
                      </td>
                      <td className="px-4 py-3">
                        <div className="font-medium text-gray-800">{row.cheapest.provider}</div>
                        <div className="text-xs text-gray-500">{row.cheapest.price_group || row.cheapest.upstream_model}</div>
                        <div className="mt-0.5 font-mono text-[11px] text-emerald-600">{costText(row.cheapest.cost)}</div>
                      </td>
                      <td className={`px-4 py-3 font-mono text-xs ${row.gap_percent > 0.5 ? "text-red-600" : "text-gray-400"}`}>{pct(row.gap_percent)}</td>
                      <td className="px-4 py-3 text-xs text-gray-500">
                        {row.primary.cost.billing_type === "per_token"
                          ? `${((row.usage.input_tokens + row.usage.output_tokens) / 1000000).toFixed(1)}M tokens`
                          : row.primary.cost.billing_type === "per_image"
                            ? `${row.usage.image_count} 张`
                            : `${row.usage.video_seconds.toFixed(1)} 秒`}
                      </td>
                      <td className="px-4 py-3 font-mono text-xs text-gray-700">
                        {money(row.monthly_saving, 2)}
                        <div className="mt-0.5 font-sans text-[10px] font-normal text-gray-400">换线后</div>
                      </td>
                      <td className="px-4 py-3">
                        {row.has_user_price && sortUsage(row) > 0 ? (
                          <div>
                            <div className={`font-mono text-xs font-medium ${row.monthly_profit >= 0 ? "text-emerald-600" : "text-red-600"}`}>
                              {money(row.monthly_profit, 2)}
                            </div>
                            <div className="mt-0.5 text-[10px] text-gray-400">
                              毛利率 {row.margin_percent === undefined ? "—" : pct(row.margin_percent)}
                            </div>
                          </div>
                        ) : (
                          <span className="text-xs text-gray-400">—</span>
                        )}
                      </td>
                      <td className="px-4 py-3">{statusBadge(row.status)}</td>
                      <td className="px-4 py-3 text-right">
                        <div className="flex flex-col items-end gap-1.5">
                          {row.primary.provider === "new_api" && row.primary.cost_source.startsWith("hub:") && (
                            <button
                              type="button"
                              disabled={applyingCost}
                              onClick={() => applyRealCost(row)}
                              className="rounded-lg border border-emerald-200 bg-emerald-50 px-2.5 py-1.5 text-xs font-medium text-emerald-700 hover:bg-emerald-100 disabled:opacity-50"
                            >
                              应用真实成本
                            </button>
                          )}
                          {row.cheapest.route_id !== row.primary.route_id ? (
                            <button
                              type="button"
                              onClick={() => openPromote(row, row.cheapest)}
                              className="inline-flex items-center gap-1 rounded-lg border border-gray-200 px-2.5 py-1.5 text-xs font-medium text-gray-700 hover:bg-gray-50"
                            >
                              设为主线路 <ArrowRight className="h-3 w-3" />
                            </button>
                          ) : (
                            <span className="text-xs text-gray-400">已是最优</span>
                          )}
                        </div>
                      </td>
                    </tr>
                  ))}
                  {sortedRows.length === 0 && (
                    <tr><td colSpan={10} className="px-4 py-10 text-center text-sm text-gray-400">没有匹配的模型</td></tr>
                  )}
                </tbody>
              </table>
            </div>
          ) : (
            <div className="grid gap-3 xl:grid-cols-2">
              {filteredRows.map((row) => (
                <div key={row.model_id} className="rounded-2xl border bg-white p-4">
                  <div className="flex items-start justify-between gap-3">
                    <div>
                      <div className="font-semibold text-gray-900">{row.display_name || row.code}</div>
                      <div className="font-mono text-[11px] text-gray-400">{row.code}</div>
                    </div>
                    {statusBadge(row.status)}
                  </div>
                  <div className="mt-3 rounded-xl border border-gray-100 bg-gray-50 p-3">
                    <div className="mb-1 text-[11px] font-semibold uppercase tracking-wide text-gray-400">主线路</div>
                    <div className="flex items-center justify-between gap-3">
                      <div>
                        <div className="text-sm font-medium text-gray-800">{row.primary.provider} · {row.primary.route_name}</div>
                        <div className="text-xs text-gray-500">{row.primary.upstream_model}{row.primary.price_group ? ` · ${row.primary.price_group}` : ""}</div>
                      </div>
                      <div className="text-right font-mono text-xs text-gray-700">
                        {costText(row.primary.cost)}
                        <div className="mt-0.5 font-sans text-[10px] font-normal text-gray-400">
                          {row.primary.observed_at ? new Date(row.primary.observed_at).toLocaleString("zh-CN") : "配置价"}
                          {row.primary.is_stale ? " · 过期" : ""}
                        </div>
                      </div>
                    </div>
                  </div>
                  <div className="mt-2 space-y-1.5">
                    {row.backups.map((b) => (
                      <div key={b.route_id} className="flex items-center justify-between gap-3 rounded-lg px-1 py-1 text-xs">
                        <div>
                          <span className="font-medium text-gray-700">{b.provider}</span>
                          <span className="ml-2 text-gray-400">{b.upstream_model}{b.price_group ? ` · ${b.price_group}` : ""}</span>
                        </div>
                        <div className="flex items-center gap-2 font-mono text-gray-500">
                          {costText(b.cost)}
                          <button
                            type="button"
                            onClick={() => openPromote(row, b)}
                            className="rounded-md border px-1.5 py-0.5 text-[11px] text-gray-600 hover:bg-gray-50"
                          >
                            设为主
                          </button>
                        </div>
                      </div>
                    ))}
                    {row.backups.length === 0 && <div className="text-xs text-gray-400">没有备用线路</div>}
                  </div>
                  {row.potential && (
                    <div className="mt-3 rounded-xl border border-dashed border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800">
                      潜在更低价：{row.potential.provider} · {row.potential.group_name} · {costText(row.potential.cost)}
                      <span className="ml-2 text-amber-600">需换 Key 或新建线路，暂不可一键切换</span>
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {tab === "catalog" && (
        <div className="space-y-3">
          <div className="flex flex-col gap-2 md:flex-row md:items-center">
            <input value={catalogQuery} onChange={(e) => { setCatalogQuery(e.target.value); setCatalogPage(1); }} placeholder="搜索上游模型 / 内部模型" className="w-full rounded-xl border px-3 py-2 text-sm md:w-72" />
            <select value={catalogProvider} onChange={(e) => { setCatalogProvider(e.target.value); setCatalogPage(1); }} className="rounded-xl border px-3 py-2 text-sm">
              <option value="">全部厂商</option>
              {providers.map((p) => <option key={p.provider} value={p.provider}>{p.display_name || p.provider}</option>)}
            </select>
            <label className="flex items-center gap-2 text-sm text-gray-600">
              <input type="checkbox" checked={catalogTrusted} onChange={(e) => { setCatalogTrusted(e.target.checked); setCatalogPage(1); }} />
              只看可信分组
            </label>
          </div>
          <div className="overflow-x-auto rounded-2xl border bg-white">
            <table className="min-w-full text-sm">
              <thead className="bg-gray-50 text-left text-xs text-gray-500">
                <tr>
                  <th className="px-4 py-3">厂商 / 分组</th>
                  <th className="px-4 py-3">上游模型</th>
                  <th className="px-4 py-3">内部模型</th>
                  <th className="px-4 py-3">类型</th>
                  <th className="px-4 py-3">原始价</th>
                  <th className="px-4 py-3">人民币价</th>
                  <th className="px-4 py-3">倍率</th>
                  <th className="px-4 py-3">来源</th>
                  <th className="px-4 py-3">时间</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100">
                {catalog.map((r) => (
                  <tr key={`${r.provider}-${r.upstream_model}-${r.group_name}`} className="hover:bg-gray-50/60">
                    <td className="px-4 py-3">
                      <div className="font-medium text-gray-800">{r.provider}</div>
                      <div className="text-[11px] text-gray-400">{r.group_name || "default"}{r.is_trusted ? " · 可信" : " · 高风险"}</div>
                    </td>
                    <td className="px-4 py-3 font-mono text-xs text-gray-600">{r.upstream_model}</td>
                    <td className="px-4 py-3 text-xs text-gray-500">{r.model_code || "未匹配"}</td>
                    <td className="px-4 py-3 text-xs text-gray-500">{r.billing_type}</td>
                    <td className="px-4 py-3 font-mono text-xs text-gray-500">{r.billing_type === "per_token" ? `${r.input_price}/${r.output_price}` : r.unit_price} {r.source_currency}</td>
                    <td className="px-4 py-3 font-mono text-xs text-gray-800">{r.billing_type === "per_token" ? `${money(r.cny_input_per_m)} / ${money(r.cny_output_per_m)}` : money(r.cny_unit_price)}</td>
                    <td className="px-4 py-3 text-xs text-gray-500">{r.billing_multiplier} × {r.fx_rate}</td>
                    <td className="px-4 py-3 text-xs text-gray-500">{sourceLabel(r.source_type)}{r.is_stale ? " · 过期" : ""}</td>
                    <td className="px-4 py-3 text-xs text-gray-400">{new Date(r.observed_at).toLocaleString("zh-CN")}</td>
                  </tr>
                ))}
                {catalog.length === 0 && <tr><td colSpan={9} className="px-4 py-10 text-center text-sm text-gray-400">暂无价格，点击右上角「同步」</td></tr>}
              </tbody>
            </table>
          </div>
          <AdminPagination page={catalogPage} total={catalogTotal} pageSize={CATALOG_PAGE_SIZE} onPageChange={setCatalogPage} />
        </div>
      )}

      {tab === "sync" && (
        <div className="space-y-3">
          <div className="rounded-2xl border border-blue-200 bg-blue-50 px-4 py-3 text-xs leading-5 text-blue-700">
            自动同步当前支持 <b>PackyCode（New API）</b>、<b>OpenRouter</b> 和 <b>自建 New API 网关</b>（通过 upstream-inspector 读取真实上游渠道）；百炼、火山、BigModel、TokenHub 暂无公开价格接口，继续使用现有线路成本配置。
          </div>
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
            {providers.map((p) => (
              <div key={p.provider} className="rounded-2xl border bg-white p-4 text-sm">
                <div className="flex items-center justify-between">
                  <div className="font-semibold text-gray-900">{p.display_name || p.provider}</div>
                  {providerCardStatus(p)}
                </div>
                <div className="mt-2 space-y-1 text-xs text-gray-500">
                  <div>来源：{providerSourceLabel(p.source_type)} · {p.currency}</div>
                  <div>
                    充值倍率：{p.recharge_rate}
                    {p.currency === "USD" ? ` · 美元汇率：${p.usd_cny_rate}` : ""}
                  </div>
                  <div>最近同步：{p.last_sync_at ? new Date(p.last_sync_at).toLocaleString("zh-CN") : "—"}</div>
                  <div>连续失败：{p.consecutive_failures}</div>
                </div>
              </div>
            ))}
          </div>
          <div className="overflow-x-auto rounded-2xl border bg-white">
            <table className="min-w-full text-sm">
              <thead className="bg-gray-50 text-left text-xs text-gray-500">
                <tr>
                  <th className="px-4 py-3">批次</th>
                  <th className="px-4 py-3">状态</th>
                  <th className="px-4 py-3">厂商</th>
                  <th className="px-4 py-3">条数</th>
                  <th className="px-4 py-3">开始时间</th>
                  <th className="px-4 py-3">错误</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100">
                {runs.map((r) => (
                  <tr key={r.id}>
                    <td className="px-4 py-3 font-mono text-xs">#{r.id}</td>
                    <td className="px-4 py-3 text-xs">{providerStatus(r.status)}</td>
                    <td className="px-4 py-3 text-xs">{r.provider_succeeded}/{r.provider_total} 成功</td>
                    <td className="px-4 py-3 text-xs">{r.items_total}</td>
                    <td className="px-4 py-3 text-xs text-gray-400">{new Date(r.started_at).toLocaleString("zh-CN")}</td>
                    <td className="px-4 py-3 max-w-[420px] truncate text-xs text-red-500" title={r.error_summary}>{r.error_summary || "—"}</td>
                  </tr>
                ))}
                {runs.length === 0 && <tr><td colSpan={6} className="px-4 py-10 text-center text-sm text-gray-400">还没有同步记录</td></tr>}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {tab === "mappings" && (
        <div className="overflow-x-auto rounded-2xl border bg-white">
          <table className="min-w-full text-sm">
            <thead className="bg-gray-50 text-left text-xs text-gray-500">
              <tr>
                <th className="px-4 py-3">厂商</th>
                <th className="px-4 py-3">上游模型</th>
                <th className="px-4 py-3">当前映射</th>
                <th className="px-4 py-3">选择内部模型</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {mappings.map((m) => (
                <tr key={`${m.provider}-${m.upstream_model}`}>
                  <td className="px-4 py-3 text-xs font-medium">{m.provider}</td>
                  <td className="px-4 py-3 font-mono text-xs text-gray-600">{m.upstream_model}</td>
                  <td className="px-4 py-3 text-xs text-gray-500">{m.model_code || "未匹配"}{m.is_confirmed ? " · 已确认" : ""}</td>
                  <td className="px-4 py-3">
                    <select
                      defaultValue=""
                      onChange={(e) => updateMapping(m, Number(e.target.value))}
                      className="rounded-lg border px-2 py-1.5 text-xs"
                    >
                      <option value="">选择模型…</option>
                      {models.map((opt) => <option key={opt.id} value={opt.id}>{opt.code} · {opt.display_name}</option>)}
                    </select>
                  </td>
                </tr>
              ))}
              {mappings.length === 0 && <tr><td colSpan={4} className="px-4 py-10 text-center text-sm text-gray-400">没有未匹配的模型</td></tr>}
            </tbody>
          </table>
        </div>
      )}

      {tab === "switches" && (
        <div className="overflow-x-auto rounded-2xl border bg-white">
          <table className="min-w-full text-sm">
            <thead className="bg-gray-50 text-left text-xs text-gray-500">
              <tr>
                <th className="px-4 py-3">时间</th>
                <th className="px-4 py-3">模型</th>
                <th className="px-4 py-3">切换</th>
                <th className="px-4 py-3">预计月省</th>
                <th className="px-4 py-3">状态</th>
                <th className="px-4 py-3 text-right">操作</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {switches.map((s) => (
                <tr key={s.id}>
                  <td className="px-4 py-3 text-xs text-gray-400">{new Date(s.created_at).toLocaleString("zh-CN")}</td>
                  <td className="px-4 py-3 text-xs font-mono">{s.model_code || s.model_id}</td>
                  <td className="px-4 py-3 text-xs text-gray-600">{s.from_route_name || "—"} <ArrowRight className="inline h-3 w-3" /> {s.to_route_name}</td>
                  <td className="px-4 py-3 font-mono text-xs">{money(s.estimated_monthly_saving, 2)}</td>
                  <td className="px-4 py-3 text-xs">{s.rolled_back_at ? <span className="text-gray-400">已回滚</span> : <span className="text-emerald-600">生效中</span>}</td>
                  <td className="px-4 py-3 text-right">
                    {!s.rolled_back_at && s.reason !== "rollback" && (
                      <button type="button" onClick={() => rollback(s.id)} className="rounded-lg border px-2.5 py-1.5 text-xs text-gray-700 hover:bg-gray-50">回滚</button>
                    )}
                  </td>
                </tr>
              ))}
              {switches.length === 0 && <tr><td colSpan={6} className="px-4 py-10 text-center text-sm text-gray-400">没有切换记录</td></tr>}
            </tbody>
          </table>
        </div>
      )}

      {promote && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4" onClick={() => setPromote(null)}>
          <div className="w-full max-w-2xl rounded-2xl bg-white p-5 shadow-2xl" onClick={(e) => e.stopPropagation()}>
            <div className="flex items-start justify-between">
              <div>
                <h2 className="text-lg font-semibold text-gray-900">切换主线路</h2>
                <p className="mt-1 text-xs text-gray-500">{promote.row.display_name || promote.row.code}</p>
              </div>
              <button type="button" onClick={() => setPromote(null)} className="text-gray-400 hover:text-gray-600"><X className="h-5 w-5" /></button>
            </div>
            <div className="mt-4 overflow-hidden rounded-xl border">
              <table className="min-w-full text-sm">
                <thead className="bg-gray-50 text-left text-xs text-gray-500">
                  <tr><th className="px-3 py-2">项</th><th className="px-3 py-2">切换前</th><th className="px-3 py-2">切换后</th></tr>
                </thead>
                <tbody className="divide-y">
                  <tr><td className="px-3 py-2 text-gray-500">主线路</td><td className="px-3 py-2">{promote.row.primary.provider} · {promote.row.primary.route_name}</td><td className="px-3 py-2 font-medium">{promote.target.provider} · {promote.target.route_name}</td></tr>
                  <tr><td className="px-3 py-2 text-gray-500">成本</td><td className="px-3 py-2 font-mono text-xs">{costText(promote.row.primary.cost)}</td><td className="px-3 py-2 font-mono text-xs text-emerald-600">{costText(promote.target.cost)}</td></tr>
                  <tr><td className="px-3 py-2 text-gray-500">预计日节省</td><td className="px-3 py-2">—</td><td className="px-3 py-2 font-mono text-xs">{money(promote.daily, 2)}</td></tr>
                  <tr><td className="px-3 py-2 text-gray-500">预计月节省</td><td className="px-3 py-2">—</td><td className="px-3 py-2 font-mono text-xs">{money(promote.monthly, 2)}</td></tr>
                </tbody>
              </table>
            </div>
            <label className="mt-4 flex items-start gap-2 text-sm text-gray-700">
              <input type="checkbox" checked={promoteConfirm} onChange={(e) => setPromoteConfirm(e.target.checked)} className="mt-0.5" />
              <span>我确认立即切换线上流量；切换后会用最新同步价更新该线路成本规则。</span>
            </label>
            <div className="mt-5 flex justify-end gap-2">
              <button type="button" onClick={() => setPromote(null)} className="rounded-xl border px-4 py-2 text-sm text-gray-600">取消</button>
              <button type="button" disabled={!promoteConfirm || promoteBusy} onClick={confirmPromote} className="rounded-xl bg-gray-950 px-4 py-2 text-sm font-semibold text-white disabled:opacity-50">
                {promoteBusy ? "切换中…" : "确认切换"}
              </button>
            </div>
          </div>
        </div>
      )}

      {loading && <div className="text-center text-sm text-gray-400">加载中…</div>}
    </div>
  );
}

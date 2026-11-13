"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { adminApi } from "@/lib/api";

interface RouteItem {
  route_id: number;
  model_id: number;
  model_code: string;
  model_name: string;
  category: string;
  route_name: string;
  provider: string;
  upstream_model: string;
  price_group: string;
  priority: number;
  is_enabled: boolean;
  health_status: string;
  account_id?: number | null;
  account_name?: string;
  account_group?: string;
  login_account?: string;
}

interface GroupCapability {
  group_name: string;
  model_count: number;
  categories: Record<string, number>;
  models: string[];
}

interface AccountItem {
  id: number;
  provider: string;
  name: string;
  group_name: string;
  rate_multiplier?: number;
  login_account?: string;
  is_enabled: boolean;
}

const PAGE_SIZE = 50;

export default function ModelRoutesBulkPage() {
  const [items, setItems] = useState<RouteItem[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [accounts, setAccounts] = useState<AccountItem[]>([]);
  const [providerFilter, setProviderFilter] = useState("");
  const [accountFilter, setAccountFilter] = useState("");
  const [enabledFilter, setEnabledFilter] = useState("");
  const [keyword, setKeyword] = useState("");
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [targetAccount, setTargetAccount] = useState<number | "">("");
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const [modelSwitch, setModelSwitch] = useState<{ modelId: number; modelCode: string; modelName: string; provider: string; accountId: number | "" } | null>(null);
  const [capabilities, setCapabilities] = useState<Record<string, Record<string, GroupCapability>>>({});
  const [showIncompatible, setShowIncompatible] = useState(false);

  const load = useCallback(async () => {
    const params = new URLSearchParams({ page: String(page), page_size: String(PAGE_SIZE) });
    if (providerFilter) params.set("provider", providerFilter);
    if (accountFilter) params.set("account_id", accountFilter);
    if (enabledFilter) params.set("enabled", enabledFilter);
    if (keyword.trim()) params.set("q", keyword.trim());
    const data = await adminApi<{ items: RouteItem[]; total: number }>(`/model-routes/list?${params.toString()}`);
    setItems(Array.isArray(data?.items) ? data.items : []);
    setTotal(data?.total ?? 0);
  }, [page, providerFilter, accountFilter, enabledFilter, keyword]);

  useEffect(() => { void load(); }, [load]);
  useEffect(() => {
    adminApi<AccountItem[]>("/upstream-accounts").then((list) => setAccounts(Array.isArray(list) ? list : [])).catch(() => setAccounts([]));
  }, []);

  const providers = useMemo(() => Array.from(new Set(accounts.map((a) => a.provider))).sort(), [accounts]);

  const loadCapabilities = useCallback(async (provider: string) => {
    try {
      const list = await adminApi<GroupCapability[]>(`/upstream-groups/capabilities?provider=${encodeURIComponent(provider)}`);
      const map: Record<string, GroupCapability> = {};
      for (const item of Array.isArray(list) ? list : []) map[item.group_name] = item;
      setCapabilities((prev) => ({ ...prev, [provider]: map }));
    } catch {
      setCapabilities((prev) => ({ ...prev, [provider]: {} }));
    }
  }, []);
  useEffect(() => {
    const list = providerFilter ? [providerFilter] : providers;
    for (const provider of list) void loadCapabilities(provider);
  }, [providerFilter, providers, loadCapabilities]);

  // 账号展示名：名称里若已包含分组/登录账号则不重复追加
  const accountDisplay = (a: AccountItem) => {
    let label = a.name || "";
    if (a.group_name && !label.includes(a.group_name)) label += ` · ${a.group_name}`;
    if (a.login_account && !label.includes(a.login_account)) label += ` · ${a.login_account}`;
    if (a.rate_multiplier && a.rate_multiplier > 0 && !label.includes(`${a.rate_multiplier}x`)) label += ` · ${a.rate_multiplier}x`;
    return label;
  };

  const accountsByProvider = useMemo(() => {
    const map: Record<string, AccountItem[]> = {};
    for (const account of accounts) (map[account.provider] ||= []).push(account);
    return map;
  }, [accounts]);

  // 账号分组是否支持该线路的上游模型
  const accountCompatible = (provider: string, upstreamModel: string, account: AccountItem) => {
    if (!account.group_name) return "unknown";
    const caps = capabilities[provider];
    if (!caps) return "unknown";
    const group = caps[account.group_name];
    if (!group) return "unknown";
    const target = upstreamModel.toLowerCase();
    return group.models.some((m) => m.toLowerCase() === target) ? "compatible" : "incompatible";
  };

  const selectedProviders = useMemo(() => {
    const set = new Set<string>();
    for (const item of items) if (selected.has(item.route_id)) set.add(item.provider);
    return Array.from(set);
  }, [items, selected]);

  const targetOptions = useMemo(() => {
    if (selectedProviders.length === 1) return accountsByProvider[selectedProviders[0]] || [];
    return [];
  }, [selectedProviders, accountsByProvider]);

  const toggle = (id: number) => {
    const next = new Set(selected);
    if (next.has(id)) next.delete(id); else next.add(id);
    setSelected(next);
  };

  const toggleAll = () => {
    if (selected.size === items.length) setSelected(new Set());
    else setSelected(new Set(items.map((item) => item.route_id)));
  };

  const bindSingle = async (item: RouteItem, accountId: number) => {
    if (!accountId) return;
    try {
      setBusy(true);
      setMessage("");
      await adminApi(`/model-routes/${item.route_id}/bind`, { method: "POST", body: JSON.stringify({ account_id: accountId }) });
      await load();
      setMessage(`已把 ${item.model_code} 切到账号：${accounts.find((a) => a.id === accountId)?.name || accountId}`);
    } catch (e) {
      setMessage(e instanceof Error ? e.message : "切换失败");
    } finally {
      setBusy(false);
    }
  };

  const bindBatch = async () => {
    if (!targetAccount) { setMessage("请选择目标账号"); return; }
    if (selectedProviders.length !== 1) { setMessage("所选线路跨了多个上游，请先按上游筛选"); return; }
    try {
      setBusy(true);
      const result = await adminApi<{ affected: number }>("/model-routes/batch-bind", {
        method: "POST",
        body: JSON.stringify({ route_ids: Array.from(selected), account_id: targetAccount }),
      });
      setMessage(`已切换 ${result?.affected ?? 0} 条线路`);
      setSelected(new Set());
      setTargetAccount("");
      await load();
    } catch (e) {
      setMessage(e instanceof Error ? e.message : "批量绑定失败");
    } finally {
      setBusy(false);
    }
  };

  const bindModel = async () => {
    if (!modelSwitch || !modelSwitch.accountId) { setMessage("请选择目标账号"); return; }
    try {
      setBusy(true);
      const result = await adminApi<{ affected: number }>("/model-routes/batch-bind", {
        method: "POST",
        body: JSON.stringify({ model_id: modelSwitch.modelId, account_id: modelSwitch.accountId }),
      });
      setMessage(`模型 ${modelSwitch.modelCode} 已整体切换 ${result?.affected ?? 0} 条线路`);
      setModelSwitch(null);
      await load();
    } catch (e) {
      setMessage(e instanceof Error ? e.message : "整体切换失败");
    } finally {
      setBusy(false);
    }
  };

  const pages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  const [costPreview, setCostPreview] = useState<any | null>(null);
  const [costBusy, setCostBusy] = useState(false);

  const previewCosts = async () => {
    try {
      setCostBusy(true);
      const res = await adminApi<any>("/model-routes/recalibrate-costs", {
        method: "POST",
        body: JSON.stringify({ provider: providerFilter, dry_run: true }),
      });
      setCostPreview(res);
    } catch (e) {
      setMessage(e instanceof Error ? e.message : "校准预览失败");
    } finally {
      setCostBusy(false);
    }
  };

  const applyCosts = async () => {
    if (!confirm("将按「官方价 × 分组倍率」覆盖可计算的线路成本，确认继续？")) return;
    try {
      setCostBusy(true);
      const res = await adminApi<any>("/model-routes/recalibrate-costs", {
        method: "POST",
        body: JSON.stringify({ provider: providerFilter, dry_run: false }),
      });
      setCostPreview(null);
      setMessage(`已校准线路成本，共 ${res.updatable ?? 0} 条`);
      await load();
    } catch (e) {
      setMessage(e instanceof Error ? e.message : "校准失败");
    } finally {
      setCostBusy(false);
    }
  };

  return (
    <div className="space-y-4 p-6">
      <div>
        <h1 className="text-xl font-semibold text-gray-900">线路批量管理</h1>
        <p className="mt-1 text-xs text-gray-500">一屏查看并切换所有模型的线路绑定账号；支持单条即时切换、勾选批量、按模型整体切换。</p>
      </div>

      {message && <div className="rounded-lg bg-blue-50 px-3 py-2 text-xs text-blue-700">{message}</div>}

      <div className="flex flex-wrap items-center gap-2">
        <select value={providerFilter} onChange={(e) => { setProviderFilter(e.target.value); setPage(1); }} className="rounded-lg border p-2 text-sm">
          <option value="">全部上游</option>
          {providers.map((p) => <option key={p} value={p}>{p}</option>)}
        </select>
        <select value={accountFilter} onChange={(e) => { setAccountFilter(e.target.value); setPage(1); }} className="rounded-lg border p-2 text-sm">
          <option value="">全部账号</option>
          <option value="unbound">未绑定账号</option>
          {accounts.filter((a) => !providerFilter || a.provider === providerFilter).map((a) => (
            <option key={a.id} value={String(a.id)}>{a.name}</option>
          ))}
        </select>
        <select value={enabledFilter} onChange={(e) => { setEnabledFilter(e.target.value); setPage(1); }} className="rounded-lg border p-2 text-sm">
          <option value="">全部状态</option>
          <option value="enabled">启用</option>
          <option value="disabled">停用</option>
        </select>
        <input value={keyword} onChange={(e) => { setKeyword(e.target.value); setPage(1); }} placeholder="搜索模型 / 线路 / 上游模型" className="w-64 rounded-lg border p-2 text-sm" />
        <label className="flex items-center gap-1 text-xs text-gray-600">
          <input type="checkbox" checked={showIncompatible} onChange={(e) => setShowIncompatible(e.target.checked)} />
          显示不兼容账号
        </label>
        <button type="button" disabled={costBusy} onClick={previewCosts} className="rounded-lg border px-3 py-2 text-xs text-gray-700 hover:bg-gray-50">
          校准线路成本
        </button>
        <span className="text-xs text-gray-400">共 {total} 条</span>
      </div>

      {costPreview && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/30 p-4" onClick={() => setCostPreview(null)}>
          <div className="max-h-[85vh] w-full max-w-4xl overflow-auto rounded-2xl bg-white p-5 shadow-xl" onClick={(e) => e.stopPropagation()}>
            <div className="flex items-start justify-between gap-4">
              <div>
                <h2 className="text-lg font-semibold text-gray-900">校准线路成本</h2>
                <p className="mt-1 text-xs text-gray-500">
                  按「官方价 × 线路绑定分组倍率」重算成本；共 {costPreview.total ?? 0} 条线路，可更新 {costPreview.updatable ?? 0} 条，其余因缺分组/倍率/官方价跳过。
                </p>
              </div>
              <div className="flex gap-2">
                <button type="button" disabled={costBusy} onClick={applyCosts} className="rounded-lg bg-gray-900 px-3 py-1.5 text-sm text-white disabled:opacity-50">应用</button>
                <button type="button" onClick={() => setCostPreview(null)} className="rounded-lg border px-3 py-1.5 text-sm">关闭</button>
              </div>
            </div>
            <table className="mt-3 w-full text-sm">
              <thead className="bg-gray-50 text-xs text-gray-500">
                <tr>
                  <th className="px-3 py-2 text-left">模型</th>
                  <th className="px-3 py-2 text-left">线路</th>
                  <th className="px-3 py-2 text-left">分组</th>
                  <th className="px-3 py-2 text-right">倍率</th>
                  <th className="px-3 py-2 text-right">原成本（入/出）</th>
                  <th className="px-3 py-2 text-right">新成本（入/出）</th>
                  <th className="px-3 py-2 text-left">说明</th>
                </tr>
              </thead>
              <tbody className="divide-y">
                {(costPreview.items ?? []).map((item: any) => (
                  <tr key={item.route_id} className={item.new_cost_rule ? "" : "text-gray-400"}>
                    <td className="px-3 py-2 font-mono text-xs">{item.model_code}</td>
                    <td className="px-3 py-2 text-xs">{item.route_name}</td>
                    <td className="px-3 py-2 text-xs">{item.group_name || "—"}</td>
                    <td className="px-3 py-2 text-right text-xs">{item.group_rate > 0 ? `${item.group_rate}x` : "—"}</td>
                    <td className="px-3 py-2 text-right text-xs">{(item.old_cost_rule?.input_cost_per_m ?? "—")}/{(item.old_cost_rule?.output_cost_per_m ?? "—")}</td>
                    <td className="px-3 py-2 text-right text-xs">{item.new_cost_rule ? `${item.new_cost_rule.input_cost_per_m}/${item.new_cost_rule.output_cost_per_m}` : "—"}</td>
                    <td className="px-3 py-2 text-xs">{item.reason || "将更新"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {selected.size > 0 && (
        <div className="flex flex-wrap items-center gap-3 rounded-xl border border-blue-200 bg-blue-50 px-4 py-3">
          <span className="text-sm text-blue-800">已选 {selected.size} 条</span>
          <select
            value={targetAccount}
            onChange={(e) => setTargetAccount(Number(e.target.value) || "")}
            className="rounded-lg border p-2 text-sm"
          >
            <option value="">选择目标账号</option>
            {targetOptions.map((a) => (
              <option key={a.id} value={a.id}>{accountDisplay(a)}</option>
            ))}
          </select>
          {selectedProviders.length > 1 && <span className="text-xs text-amber-600">所选线路跨多个上游，请先按上游筛选</span>}
          <button type="button" disabled={busy || !targetAccount || selectedProviders.length !== 1} onClick={bindBatch} className="rounded-lg bg-gray-900 px-3 py-2 text-sm text-white disabled:opacity-50">批量绑定</button>
          <button type="button" onClick={() => { setSelected(new Set()); setTargetAccount(""); }} className="text-xs text-gray-500">清空选择</button>
        </div>
      )}

      <div className="overflow-hidden rounded-2xl border bg-white">
        <table className="w-full text-sm">
          <thead className="bg-gray-50 text-gray-500">
            <tr>
              <th className="w-10 px-3 py-3"><input type="checkbox" checked={items.length > 0 && selected.size === items.length} onChange={toggleAll} /></th>
              <th className="px-3 py-3 text-left">模型</th>
              <th className="px-3 py-3 text-left">线路</th>
              <th className="px-3 py-3 text-left">上游模型</th>
              <th className="px-3 py-3 text-left">当前账号</th>
              <th className="px-3 py-3 text-left">分组</th>
              <th className="px-3 py-3 text-left">优先级</th>
              <th className="px-3 py-3 text-left">状态</th>
              <th className="px-3 py-3 text-left">健康</th>
            </tr>
          </thead>
          <tbody className="divide-y">
            {items.map((item) => (
              <tr key={item.route_id} className={selected.has(item.route_id) ? "bg-blue-50/40" : ""}>
                <td className="px-3 py-2"><input type="checkbox" checked={selected.has(item.route_id)} onChange={() => toggle(item.route_id)} /></td>
                <td className="px-3 py-2">
                  <div className="font-mono text-xs text-gray-800">{item.model_code}</div>
                  <div className="text-[11px] text-gray-400">{item.model_name}</div>
                  <button
                    type="button"
                    onClick={() => setModelSwitch({ modelId: item.model_id, modelCode: item.model_code, modelName: item.model_name, provider: item.provider, accountId: "" })}
                    className="mt-1 text-[11px] text-purple-600 hover:underline"
                  >
                    按模型整体切换
                  </button>
                </td>
                <td className="px-3 py-2 text-xs">{item.route_name}</td>
                <td className="px-3 py-2 font-mono text-xs">{item.upstream_model}</td>
                <td className="px-3 py-2">
                  <select
                    value={item.account_id ?? ""}
                    disabled={busy}
                    onChange={(e) => bindSingle(item, Number(e.target.value))}
                    className="w-[220px] rounded-lg border p-1.5 text-xs"
                  >
                    <option value="">（未绑定）</option>
                    {(accountsByProvider[item.provider] || [])
                      .filter((a) => showIncompatible || accountCompatible(item.provider, item.upstream_model, a) !== "incompatible")
                      .map((a) => {
                        const compat = accountCompatible(item.provider, item.upstream_model, a);
                        const mark = compat === "compatible" ? "" : compat === "incompatible" ? " ⚠️不兼容" : " ⚠️未知分组";
                        return (
                          <option key={a.id} value={a.id}>{accountDisplay(a)}{mark}</option>
                        );
                      })}
                  </select>
                </td>
                <td className="px-3 py-2 text-xs">{item.price_group || <span className="text-gray-400">—</span>}</td>
                <td className="px-3 py-2 text-xs">{item.priority}</td>
                <td className="px-3 py-2 text-xs"><span className={item.is_enabled ? "text-green-600" : "text-gray-400"}>{item.is_enabled ? "启用" : "停用"}</span></td>
                <td className="px-3 py-2 text-xs"><span className={item.health_status === "healthy" ? "text-green-600" : "text-amber-600"}>{item.health_status || "-"}</span></td>
              </tr>
            ))}
            {items.length === 0 && (
              <tr><td colSpan={9} className="p-8 text-center text-gray-400">没有匹配的线路</td></tr>
            )}
          </tbody>
        </table>
      </div>

      <div className="flex items-center justify-end gap-2 text-sm">
        <button type="button" disabled={page <= 1} onClick={() => setPage(page - 1)} className="rounded-lg border px-3 py-1.5 disabled:opacity-40">上一页</button>
        <span className="text-xs text-gray-500">{page} / {pages}</span>
        <button type="button" disabled={page >= pages} onClick={() => setPage(page + 1)} className="rounded-lg border px-3 py-1.5 disabled:opacity-40">下一页</button>
      </div>

      {modelSwitch && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/30 p-4">
          <div className="w-full max-w-md rounded-2xl bg-white p-5 shadow-xl">
            <h2 className="text-base font-semibold text-gray-900">按模型整体切换</h2>
            <p className="mt-1 text-xs text-gray-500">
              模型 {modelSwitch.modelCode}（{modelSwitch.modelName}）的该上游全部线路一起切到目标账号。
            </p>
            <label className="mt-4 block text-xs text-gray-600">目标账号
              <select
                value={modelSwitch.accountId}
                onChange={(e) => setModelSwitch({ ...modelSwitch, accountId: Number(e.target.value) || "" })}
                className="mt-1 w-full rounded-lg border p-2 text-sm"
              >
                <option value="">请选择</option>
                {accounts.filter((a) => !modelSwitch.provider || a.provider === modelSwitch.provider).map((a) => (
                  <option key={a.id} value={a.id}>{accountDisplay(a)}</option>
                ))}
              </select>
            </label>
            <div className="mt-4 flex justify-end gap-2">
              <button type="button" onClick={() => setModelSwitch(null)} className="rounded-lg border px-3 py-2 text-sm">取消</button>
              <button type="button" disabled={busy || !modelSwitch.accountId} onClick={bindModel} className="rounded-lg bg-gray-900 px-3 py-2 text-sm text-white disabled:opacity-50">确认切换</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

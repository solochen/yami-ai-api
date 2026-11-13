"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { adminApi } from "@/lib/api";

interface UpstreamAccount {
  id: number;
  provider: string;
  name: string;
  api_key: string;
  group_name: string;
  rate_multiplier: number;
  login_account: string;
  is_enabled: boolean;
  notes?: string;
  last_test_at?: string;
  last_test_ok?: boolean;
  last_test_error?: string;
  route_count: number;
  created_at: string;
  updated_at: string;
}

interface GroupCapability {
  group_name: string;
  model_count: number;
  categories: Record<string, number>;
  models: string[];
}

interface BulkImportResult {
  line: number;
  name: string;
  status: string;
  message?: string;
}

interface AccountRouteItem {
  route_id: number;
  model_id: number;
  model_code: string;
  model_name: string;
  route_name: string;
  provider: string;
  upstream_model: string;
  price_group: string;
  priority: number;
  is_enabled: boolean;
  health_status: string;
}

interface LoginStat {
  provider: string;
  login_account: string;
  key_count: number;
  route_count: number;
  attempts: number;
  failures: number;
  failure_rate: number;
  last_failure_at?: string;
}

interface TestResult {
  ok: boolean;
  status_code?: number;
  url?: string;
  models?: number;
  message?: string;
}

// provider 代码 → 展示名
const PROVIDER_LABELS: Record<string, string> = {
  codexauv: "CodexAuv",
  packycode: "PackyCode",
  qiniu: "七牛云",
  openrouter: "OpenRouter",
  new_api: "新加坡中转（hub.yardai.cc）",
  bailian: "阿里百炼",
  tokenhub: "腾讯云",
  aiboli: "aiboli",
  sync: "Sync",
};
const providerLabel = (p: string) => PROVIDER_LABELS[p] ?? p;

const PROVIDERS = ["codexauv", "packycode", "qiniu", "openrouter", "new_api", "bailian", "tokenhub", "aiboli", "sync"];

const emptyForm = {
  provider: "codexauv",
  name: "",
  api_key: "",
  group_name: "",
  login_account: "",
  is_enabled: true,
  notes: "",
};

export default function UpstreamAccountsPage() {
  const [accounts, setAccounts] = useState<UpstreamAccount[]>([]);
  const [groups, setGroups] = useState<Record<string, string[]>>({});
  const [providerFilter, setProviderFilter] = useState("");
  const [form, setForm] = useState({ ...emptyForm });
  const [editingId, setEditingId] = useState<number | null>(null);
  const [showForm, setShowForm] = useState(false);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const [testing, setTesting] = useState<number | null>(null);
  const [testResults, setTestResults] = useState<Record<number, TestResult>>({});
  const [loginStats, setLoginStats] = useState<LoginStat[]>([]);
  const [statsDays, setStatsDays] = useState(7);
  const [capabilityView, setCapabilityView] = useState<{ provider: string; group: string; capability?: GroupCapability } | null>(null);
  const [importOpen, setImportOpen] = useState(false);
  const [importProvider, setImportProvider] = useState("codexauv");
  const [importContent, setImportContent] = useState("");
  const [importResults, setImportResults] = useState<BulkImportResult[]>([]);
  const [importBusy, setImportBusy] = useState(false);
  const [routesView, setRoutesView] = useState<{ account: UpstreamAccount; items: AccountRouteItem[] } | null>(null);
  const [routesLoading, setRoutesLoading] = useState(false);
  const [migrateFrom, setMigrateFrom] = useState<UpstreamAccount | null>(null);
  const [migrateTo, setMigrateTo] = useState<number | null>(null);

  const load = useCallback(async () => {
    const items = await adminApi<UpstreamAccount[]>("/upstream-accounts");
    setAccounts(Array.isArray(items) ? items : []);
  }, []);

  useEffect(() => { void load(); }, [load]);
  useEffect(() => {
    adminApi<Record<string, string[]>>("/upstream-prices/groups").then(setGroups).catch(() => setGroups({}));
  }, []);
  useEffect(() => {
    adminApi<LoginStat[]>(`/upstream-accounts/stats?days=${statsDays}`).then((items) => setLoginStats(Array.isArray(items) ? items : [])).catch(() => setLoginStats([]));
  }, [statsDays, accounts.length]);

  const filtered = useMemo(
    () => (providerFilter ? accounts.filter((a) => a.provider === providerFilter) : accounts),
    [accounts, providerFilter],
  );

  const providersInUse = useMemo(() => Array.from(new Set(accounts.map((a) => a.provider))).sort(), [accounts]);

  const providerOptions = useMemo(
    () => Array.from(new Set([...PROVIDERS, ...providersInUse])),
    [providersInUse],
  );

  const openCreate = () => {
    setEditingId(null);
    setForm({ ...emptyForm });
    setShowForm(true);
    setMessage("");
  };

  const openEdit = (account: UpstreamAccount) => {
    setEditingId(account.id);
    setForm({
      provider: account.provider,
      name: account.name,
      api_key: "",
      group_name: account.group_name,
      login_account: account.login_account || "",
      is_enabled: account.is_enabled,
      notes: account.notes || "",
    });
    setShowForm(true);
    setMessage("");
  };

  const save = async () => {
    if (!form.provider.trim()) { setMessage("请填写上游"); return; }
    if (!editingId && !form.api_key.trim()) { setMessage("请填写 API Key"); return; }
    try {
      setBusy(true);
      setMessage("");
      if (editingId) {
        await adminApi(`/upstream-accounts/${editingId}`, { method: "PATCH", body: JSON.stringify(form) });
      } else {
        await adminApi("/upstream-accounts", { method: "POST", body: JSON.stringify(form) });
      }
      setShowForm(false);
      await load();
    } catch (e) {
      setMessage(e instanceof Error ? e.message : "保存失败");
    } finally {
      setBusy(false);
    }
  };

  const toggleEnabled = async (account: UpstreamAccount) => {
    try {
      setBusy(true);
      await adminApi(`/upstream-accounts/${account.id}`, {
        method: "PATCH",
        body: JSON.stringify({ provider: account.provider, name: account.name, group_name: account.group_name, login_account: account.login_account || "", is_enabled: !account.is_enabled, notes: account.notes || "" }),
      });
      await load();
    } finally {
      setBusy(false);
    }
  };

  const remove = async (account: UpstreamAccount) => {
    if (!confirm(`确认删除账号「${account.name}」？`)) return;
    try {
      setBusy(true);
      await adminApi(`/upstream-accounts/${account.id}`, { method: "DELETE" });
      await load();
    } catch (e) {
      alert(e instanceof Error ? e.message : "删除失败");
    } finally {
      setBusy(false);
    }
  };

  const test = async (account: UpstreamAccount) => {
    try {
      setTesting(account.id);
      const result = await adminApi<TestResult>(`/upstream-accounts/${account.id}/test`, { method: "POST", body: "{}" });
      setTestResults((prev) => ({ ...prev, [account.id]: result }));
      await load();
    } catch (e) {
      setTestResults((prev) => ({ ...prev, [account.id]: { ok: false, message: e instanceof Error ? e.message : "测试失败" } }));
    } finally {
      setTesting(null);
    }
  };

  const openCapability = async (provider: string, group: string) => {
    setCapabilityView({ provider, group });
    try {
      const list = await adminApi<GroupCapability[]>(`/upstream-groups/capabilities?provider=${encodeURIComponent(provider)}`);
      const found = (Array.isArray(list) ? list : []).find((item) => item.group_name === group);
      setCapabilityView({ provider, group, capability: found });
    } catch {
      setCapabilityView({ provider, group });
    }
  };

  const runImport = async () => {
    try {
      setImportBusy(true);
      const result = await adminApi<{ items: BulkImportResult[] }>("/upstream-accounts/bulk-import", {
        method: "POST",
        body: JSON.stringify({ provider: importProvider, content: importContent }),
      });
      setImportResults(Array.isArray(result?.items) ? result.items : []);
      await load();
    } catch (e) {
      setImportResults([{ line: 0, name: "", status: "failed", message: e instanceof Error ? e.message : "导入失败" }]);
    } finally {
      setImportBusy(false);
    }
  };

  const openRoutes = async (account: UpstreamAccount) => {
    setRoutesLoading(true);
    setRoutesView({ account, items: [] });
    try {
      const items = await adminApi<AccountRouteItem[]>(`/upstream-accounts/${account.id}/routes`);
      setRoutesView({ account, items: Array.isArray(items) ? items : [] });
    } catch {
      setRoutesView({ account, items: [] });
    } finally {
      setRoutesLoading(false);
    }
  };

  const openMigrate = (account: UpstreamAccount) => {
    setMigrateFrom(account);
    setMigrateTo(null);
    setMessage("");
  };

  const migrateTargets = useMemo(
    () => accounts.filter((a) => migrateFrom && a.id !== migrateFrom.id && a.provider === migrateFrom.provider),
    [accounts, migrateFrom],
  );

  const runMigrate = async () => {
    if (!migrateFrom || !migrateTo) { setMessage("请选择目标账号"); return; }
    const target = accounts.find((a) => a.id === migrateTo);
    if (!confirm(`确认把「${migrateFrom.name}」下的 ${migrateFrom.route_count} 条线路迁移到「${target?.name}」？`)) return;
    try {
      setBusy(true);
      const result = await adminApi<{ affected: number }>(`/upstream-accounts/${migrateFrom.id}/migrate`, {
        method: "POST",
        body: JSON.stringify({ to_id: migrateTo }),
      });
      setMigrateFrom(null);
      setMessage(`已迁移 ${result?.affected ?? 0} 条线路`);
      await load();
    } catch (e) {
      setMessage(e instanceof Error ? e.message : "迁移失败");
    } finally {
      setBusy(false);
    }
  };

  const groupOptions = groups[form.provider] || [];

  return (
    <div className="space-y-4 p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold text-gray-900">上游账号管理</h1>
          <p className="mt-1 text-xs text-gray-500">一个 API Key 只属于一个分组；线路绑定账号后，Key 与分组一起管理，可一键批量迁移。</p>
        </div>
        <div className="flex items-center gap-2">
          <select value={providerFilter} onChange={(e) => setProviderFilter(e.target.value)} className="rounded-lg border p-2 text-sm">
            <option value="">全部上游</option>
            {providersInUse.map((p) => <option key={p} value={p}>{providerLabel(p)}</option>)}
          </select>
          <button type="button" onClick={() => { setImportOpen(true); setImportResults([]); setImportContent(""); }} className="rounded-lg border px-3 py-2 text-sm">批量导入</button>
          <button type="button" onClick={openCreate} className="rounded-lg bg-gray-900 px-3 py-2 text-sm text-white">新增账号</button>
        </div>
      </div>

      {message && <div className="rounded-lg bg-blue-50 px-3 py-2 text-xs text-blue-700">{message}</div>}

      <div className="rounded-2xl border bg-white">
        <div className="flex items-center justify-between border-b px-4 py-3">
          <div>
            <div className="text-sm font-semibold text-gray-900">按登录账号汇总</div>
            <div className="mt-0.5 text-[11px] text-gray-400">同一上游开多个登录账号分流时，看这个表判断是否需要加号或换号</div>
          </div>
          <select value={statsDays} onChange={(e) => setStatsDays(Number(e.target.value))} className="rounded-lg border p-2 text-xs">
            <option value={7}>近 7 天</option>
            <option value={30}>近 30 天</option>
          </select>
        </div>
        <table className="w-full text-sm">
          <thead className="bg-gray-50 text-gray-500">
            <tr>
              <th className="px-4 py-2 text-left">上游</th>
              <th className="px-4 py-2 text-left">登录账号</th>
              <th className="px-4 py-2 text-left">Key 数</th>
              <th className="px-4 py-2 text-left">线路数</th>
              <th className="px-4 py-2 text-left">调用次数</th>
              <th className="px-4 py-2 text-left">失败率</th>
              <th className="px-4 py-2 text-left">最近失败</th>
            </tr>
          </thead>
          <tbody className="divide-y">
            {loginStats.map((stat) => (
              <tr key={`${stat.provider}-${stat.login_account}`}>
                <td className="px-4 py-2 text-xs" title={stat.provider}>{providerLabel(stat.provider)}</td>
                <td className="px-4 py-2 text-xs">{stat.login_account || <span className="text-gray-400">未登记</span>}</td>
                <td className="px-4 py-2">{stat.key_count}</td>
                <td className="px-4 py-2">{stat.route_count}</td>
                <td className="px-4 py-2">{stat.attempts}</td>
                <td className="px-4 py-2">
                  <span className={stat.failure_rate > 10 ? "text-red-600" : stat.failure_rate > 0 ? "text-amber-600" : "text-green-600"}>
                    {stat.attempts > 0 ? `${stat.failure_rate.toFixed(1)}%` : "—"}
                  </span>
                </td>
                <td className="px-4 py-2 text-xs text-gray-500">{stat.last_failure_at ? new Date(stat.last_failure_at).toLocaleString() : "—"}</td>
              </tr>
            ))}
            {loginStats.length === 0 && (
              <tr><td colSpan={7} className="p-6 text-center text-gray-400">暂无数据</td></tr>
            )}
          </tbody>
        </table>
      </div>

      <div className="overflow-hidden rounded-2xl border bg-white">
        <table className="w-full text-sm">
          <thead className="bg-gray-50 text-gray-500">
            <tr>
              <th className="px-4 py-3 text-left">上游</th>
              <th className="px-4 py-3 text-left">账号名</th>
              <th className="px-4 py-3 text-left">登录账号</th>
              <th className="px-4 py-3 text-left">分组</th>
              <th className="px-4 py-3 text-left">倍率</th>
              <th className="px-4 py-3 text-left">API Key</th>
              <th className="px-4 py-3 text-left">绑定线路</th>
              <th className="px-4 py-3 text-left">最近测试</th>
              <th className="px-4 py-3 text-left">状态</th>
              <th className="px-4 py-3 text-left">操作</th>
            </tr>
          </thead>
          <tbody className="divide-y">
            {filtered.map((account) => {
              const result = testResults[account.id];
              const testOK = result ? result.ok : account.last_test_ok;
              return (
                <tr key={account.id}>
                  <td className="px-4 py-3 text-xs" title={account.provider}>
                    <div>{providerLabel(account.provider)}</div>
                    {PROVIDER_LABELS[account.provider] && <div className="text-[10px] text-gray-400">{account.provider}</div>}
                  </td>
                  <td className="px-4 py-3">{account.name}</td>
                  <td className="px-4 py-3 text-xs">{account.login_account || <span className="text-gray-400">—</span>}</td>
                  <td className="px-4 py-3 text-xs">
                    {account.group_name ? (
                      <button type="button" onClick={() => openCapability(account.provider, account.group_name)} className="text-blue-600 hover:underline" title="查看该分组支持的模型">
                        {account.group_name}
                      </button>
                    ) : (
                      <span className="text-gray-400">未绑定</span>
                    )}
                  </td>
                  <td className="px-4 py-3 text-xs">{account.rate_multiplier > 0 ? `${account.rate_multiplier}x` : <span className="text-gray-400">—</span>}</td>
                  <td className="px-4 py-3 font-mono text-xs text-gray-500">{account.api_key}</td>
                  <td className="px-4 py-3">
                    {account.route_count > 0 ? (
                      <button type="button" onClick={() => openRoutes(account)} className="text-blue-600 hover:underline" title="查看绑定线路明细">
                        {account.route_count} 条
                      </button>
                    ) : (
                      <span className="text-gray-400">0 条</span>
                    )}
                  </td>
                  <td className="px-4 py-3 text-xs">
                    {testOK === undefined ? <span className="text-gray-400">未测试</span> : testOK ? <span className="text-green-600">正常</span> : <span className="text-red-500">失败</span>}
                    {testOK === true && <div className="mt-0.5 text-[10px] text-gray-400">{result?.models ? `返回 ${result.models} 个模型` : "连通正常"}</div>}
                    {testOK === false && (result?.message || account.last_test_error) && (
                      <div className="mt-0.5 max-w-[220px] truncate text-[10px] text-red-400" title={result?.message || account.last_test_error}>
                        {result?.message || account.last_test_error}
                      </div>
                    )}
                  </td>
                  <td className="px-4 py-3"><span className={account.is_enabled ? "text-green-600" : "text-gray-400"}>{account.is_enabled ? "启用" : "禁用"}</span></td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-3 whitespace-nowrap">
                      <button type="button" disabled={busy || testing === account.id} onClick={() => test(account)} className="text-blue-600 hover:underline disabled:text-gray-300">{testing === account.id ? "测试中" : "测试"}</button>
                      <button type="button" disabled={busy} onClick={() => openEdit(account)} className="text-gray-700 hover:underline">编辑</button>
                      <button type="button" disabled={busy || account.route_count === 0} onClick={() => openMigrate(account)} className="text-purple-600 hover:underline disabled:text-gray-300">批量迁移</button>
                      <button type="button" disabled={busy} onClick={() => toggleEnabled(account)} className="text-amber-600 hover:underline">{account.is_enabled ? "禁用" : "启用"}</button>
                      <button type="button" disabled={busy} onClick={() => remove(account)} className="text-red-500 hover:underline">删除</button>
                    </div>
                  </td>
                </tr>
              );
            })}
            {filtered.length === 0 && (
              <tr><td colSpan={10} className="p-8 text-center text-gray-400">还没有账号。点右上角「新增账号」创建。</td></tr>
            )}
          </tbody>
        </table>
      </div>

      {showForm && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/30 p-4">
          <div className="w-full max-w-lg rounded-2xl bg-white p-5 shadow-xl">
            <h2 className="text-base font-semibold text-gray-900">{editingId ? "编辑账号" : "新增账号"}</h2>
            <div className="mt-4 grid grid-cols-2 gap-3">
              <label className="text-xs text-gray-600">上游
                <select value={form.provider} onChange={(e) => setForm({ ...form, provider: e.target.value, group_name: "" })} className="mt-1 w-full rounded-lg border p-2 text-sm">
                  {providerOptions.map((p) => <option key={p} value={p}>{p}</option>)}
                </select>
              </label>
              <label className="text-xs text-gray-600">账号名
                <input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} className="mt-1 w-full rounded-lg border p-2 text-sm" placeholder="如 Azure 主号" />
              </label>
              <label className="col-span-2 text-xs text-gray-600">API Key{editingId && <span className="ml-1 text-gray-400">（留空表示不修改）</span>}
                <input value={form.api_key} onChange={(e) => setForm({ ...form, api_key: e.target.value })} className="mt-1 w-full rounded-lg border p-2 font-mono text-sm" placeholder={editingId ? "••••••（不修改）" : "sk-..."} />
              </label>
              <label className="col-span-2 text-xs text-gray-600">上游登录账号<span className="ml-1 text-gray-400">（该 Key 对应的上游登录邮箱/用户名，用于同上游多账号分流）</span>
                <input value={form.login_account} onChange={(e) => setForm({ ...form, login_account: e.target.value })} className="mt-1 w-full rounded-lg border p-2 text-sm" placeholder="如 user@example.com" />
              </label>
              <label className="col-span-2 text-xs text-gray-600">所属分组<span className="ml-1 text-gray-400">（该 Key 在上游的计费分组）</span>
                <select value={form.group_name} onChange={(e) => setForm({ ...form, group_name: e.target.value })} className="mt-1 w-full rounded-lg border bg-white p-2 text-sm">
                  <option value="">未绑定 / 不适用</option>
                  {groupOptions.map((g) => <option key={g} value={g}>{g}</option>)}
                  {form.group_name && !groupOptions.includes(form.group_name) && <option value={form.group_name}>{form.group_name}（同步目录中未找到）</option>}
                </select>
              </label>
              <label className="col-span-2 text-xs text-gray-600">备注
                <input value={form.notes} onChange={(e) => setForm({ ...form, notes: e.target.value })} className="mt-1 w-full rounded-lg border p-2 text-sm" />
              </label>
              <label className="col-span-2 flex items-center gap-2 text-xs text-gray-600">
                <input type="checkbox" checked={form.is_enabled} onChange={(e) => setForm({ ...form, is_enabled: e.target.checked })} /> 启用
              </label>
            </div>
            {message && <div className="mt-3 rounded-lg bg-red-50 px-3 py-2 text-xs text-red-600">{message}</div>}
            <div className="mt-4 flex justify-end gap-2">
              <button type="button" onClick={() => setShowForm(false)} className="rounded-lg border px-3 py-2 text-sm">取消</button>
              <button type="button" disabled={busy} onClick={save} className="rounded-lg bg-gray-900 px-3 py-2 text-sm text-white disabled:opacity-50">保存</button>
            </div>
          </div>
        </div>
      )}

      {capabilityView && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/30 p-4">
          <div className="w-full max-w-2xl rounded-2xl bg-white p-5 shadow-xl">
            <div className="flex items-start justify-between">
              <div>
                <h2 className="text-base font-semibold text-gray-900">分组能力</h2>
                <p className="mt-1 text-xs text-gray-500">{capabilityView.provider} · {capabilityView.group}</p>
              </div>
              <button type="button" onClick={() => setCapabilityView(null)} className="text-sm text-gray-400 hover:text-gray-600">关闭</button>
            </div>
            {capabilityView.capability ? (
              <>
                <div className="mt-3 flex flex-wrap gap-3 text-xs">
                  <span className="rounded-lg bg-gray-100 px-2 py-1">共 {capabilityView.capability.model_count} 个模型</span>
                  {Object.entries(capabilityView.capability.categories).map(([cat, count]) => (
                    <span key={cat} className="rounded-lg bg-sky-50 px-2 py-1 text-sky-700">{cat}：{count}</span>
                  ))}
                </div>
                <div className="mt-3 max-h-[50vh] overflow-auto rounded-xl border p-3 text-xs text-gray-700">
                  <div className="flex flex-wrap gap-1.5">
                    {capabilityView.capability.models.map((model) => (
                      <span key={model} className="rounded bg-gray-50 px-1.5 py-0.5 font-mono">{model}</span>
                    ))}
                  </div>
                </div>
              </>
            ) : (
              <div className="mt-4 text-xs text-gray-400">加载中…（若长时间无数据，说明该分组尚未同步到价格矩阵）</div>
            )}
          </div>
        </div>
      )}

      {importOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/30 p-4">
          <div className="w-full max-w-2xl rounded-2xl bg-white p-5 shadow-xl">
            <div className="flex items-start justify-between">
              <div>
                <h2 className="text-base font-semibold text-gray-900">批量导入账号</h2>
                <p className="mt-1 text-xs text-gray-500">每行一个：名称, API Key, 分组, 登录账号（逗号分隔，后两项可留空）</p>
              </div>
              <button type="button" onClick={() => setImportOpen(false)} className="text-sm text-gray-400 hover:text-gray-600">关闭</button>
            </div>
            <div className="mt-3 grid grid-cols-3 gap-3">
              <label className="text-xs text-gray-600">上游
                <select value={importProvider} onChange={(e) => setImportProvider(e.target.value)} className="mt-1 w-full rounded-lg border p-2 text-sm">
                  {providerOptions.map((p) => <option key={p} value={p}>{p}</option>)}
                </select>
              </label>
            </div>
            <textarea
              value={importContent}
              onChange={(e) => setImportContent(e.target.value)}
              rows={7}
              placeholder={"Azure 主号, sk-xxxx, GPT-Azure, user@a.com\nTeam 特惠, sk-yyyy, Codex-team特惠"}
              className="mt-3 w-full rounded-lg border p-2 font-mono text-xs"
            />
            {importResults.length > 0 && (
              <div className="mt-3 max-h-40 overflow-auto rounded-lg border p-2 text-xs">
                {importResults.map((r, i) => (
                  <div key={`${r.line}-${i}`} className="flex gap-2 py-0.5">
                    <span className="text-gray-400">#{r.line}</span>
                    <span className="w-32 truncate">{r.name || "-"}</span>
                    <span className={r.status === "created" ? "text-green-600" : r.status === "skipped" ? "text-amber-600" : "text-red-500"}>
                      {r.status === "created" ? "已创建" : r.status === "skipped" ? "已跳过" : "失败"}
                    </span>
                    <span className="truncate text-gray-500">{r.message || ""}</span>
                  </div>
                ))}
              </div>
            )}
            <div className="mt-4 flex justify-end gap-2">
              <button type="button" onClick={() => setImportOpen(false)} className="rounded-lg border px-3 py-2 text-sm">关闭</button>
              <button type="button" disabled={importBusy || !importContent.trim()} onClick={runImport} className="rounded-lg bg-gray-900 px-3 py-2 text-sm text-white disabled:opacity-50">{importBusy ? "导入中…" : "开始导入"}</button>
            </div>
          </div>
        </div>
      )}

      {routesView && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/30 p-4">
          <div className="w-full max-w-4xl rounded-2xl bg-white p-5 shadow-xl">
            <div className="flex items-start justify-between">
              <div>
                <h2 className="text-base font-semibold text-gray-900">绑定线路明细</h2>
                <p className="mt-1 text-xs text-gray-500">
                  {routesView.account.name} · {routesView.account.provider}
                  {routesView.account.group_name ? ` · ${routesView.account.group_name}` : ""}
                  {routesView.account.login_account ? ` · ${routesView.account.login_account}` : ""}
                  {` · 共 ${routesView.items.length} 条`}
                </p>
              </div>
              <button type="button" onClick={() => setRoutesView(null)} className="text-sm text-gray-400 hover:text-gray-600">关闭</button>
            </div>
            <div className="mt-3 max-h-[60vh] overflow-auto rounded-xl border">
              <table className="w-full text-sm">
                <thead className="sticky top-0 bg-gray-50 text-gray-500">
                  <tr>
                    <th className="px-4 py-2 text-left">模型</th>
                    <th className="px-4 py-2 text-left">线路名</th>
                    <th className="px-4 py-2 text-left">上游模型</th>
                    <th className="px-4 py-2 text-left">分组</th>
                    <th className="px-4 py-2 text-left">优先级</th>
                    <th className="px-4 py-2 text-left">状态</th>
                    <th className="px-4 py-2 text-left">健康</th>
                  </tr>
                </thead>
                <tbody className="divide-y">
                  {routesView.items.map((item) => (
                    <tr key={item.route_id}>
                      <td className="px-4 py-2">
                        <div className="font-mono text-xs text-gray-800">{item.model_code}</div>
                        <div className="text-[11px] text-gray-400">{item.model_name}</div>
                      </td>
                      <td className="px-4 py-2 text-xs">{item.route_name}</td>
                      <td className="px-4 py-2 font-mono text-xs">{item.upstream_model}</td>
                      <td className="px-4 py-2 text-xs">{item.price_group || <span className="text-gray-400">—</span>}</td>
                      <td className="px-4 py-2 text-xs">{item.priority}</td>
                      <td className="px-4 py-2 text-xs">
                        <span className={item.is_enabled ? "text-green-600" : "text-gray-400"}>{item.is_enabled ? "启用" : "停用"}</span>
                      </td>
                      <td className="px-4 py-2 text-xs">
                        <span className={item.health_status === "healthy" ? "text-green-600" : "text-amber-600"}>{item.health_status || "-"}</span>
                      </td>
                    </tr>
                  ))}
                  {routesLoading && (
                    <tr><td colSpan={7} className="p-6 text-center text-gray-400">加载中…</td></tr>
                  )}
                  {!routesLoading && routesView.items.length === 0 && (
                    <tr><td colSpan={7} className="p-6 text-center text-gray-400">该账号暂无绑定线路</td></tr>
                  )}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}

      {migrateFrom && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/30 p-4">
          <div className="w-full max-w-lg rounded-2xl bg-white p-5 shadow-xl">
            <h2 className="text-base font-semibold text-gray-900">批量迁移线路</h2>
            <p className="mt-1 text-xs text-gray-500">
              把「{migrateFrom.name}」（{migrateFrom.provider} · {migrateFrom.group_name || "未绑定分组"}）下
              <b> {migrateFrom.route_count} 条线路</b> 一次性切到目标账号，API Key 与价格分组会一起替换。
            </p>
            <label className="mt-4 block text-xs text-gray-600">目标账号
              <select value={migrateTo ?? ""} onChange={(e) => setMigrateTo(Number(e.target.value) || null)} className="mt-1 w-full rounded-lg border p-2 text-sm">
                <option value="">请选择</option>
                {migrateTargets.map((a) => <option key={a.id} value={a.id}>{a.name}（{a.group_name || "未绑定分组"}）</option>)}
              </select>
            </label>
            {migrateTargets.length === 0 && <div className="mt-2 text-xs text-amber-600">该上游没有其它账号，请先新增目标账号。</div>}
            {message && <div className="mt-3 rounded-lg bg-red-50 px-3 py-2 text-xs text-red-600">{message}</div>}
            <div className="mt-4 flex justify-end gap-2">
              <button type="button" onClick={() => setMigrateFrom(null)} className="rounded-lg border px-3 py-2 text-sm">取消</button>
              <button type="button" disabled={busy || !migrateTo} onClick={runMigrate} className="rounded-lg bg-gray-900 px-3 py-2 text-sm text-white disabled:opacity-50">确认迁移</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

"use client";

import { useCallback, useEffect, useState } from "react";
import { adminApi } from "@/lib/api";

interface SyncSource {
  id: number;
  provider: string;
  adapter: string;
  name: string;
  site_base_url: string;
  credential_hint: string;
  login_account: string;
  config: Record<string, unknown>;
  is_enabled: boolean;
  last_sync_at?: string;
  last_status?: string;
  last_error?: string;
}

interface PreviewItem {
  group: string;
  rate: number;
  has_key: boolean;
  has_account: boolean;
  action: string;
  account_name?: string;
}

interface PreviewData {
  groups: number;
  keys_existing: number;
  keys_to_create: number;
  accounts_existing: number;
  accounts_to_create: number;
  items: PreviewItem[];
}

interface RunResult {
  run_id: number;
  groups_total: number;
  keys_created: number;
  keys_reused: number;
  accounts_created: number;
  accounts_updated: number;
  routes_bound: number;
  verify_passed: number;
  verify_failed: number;
}

const PROVIDERS = ["codexauv", "packycode", "qiniu", "aiboli"];
const emptyForm = {
  provider: "codexauv",
  adapter: "codexauv",
  name: "",
  site_base_url: "https://www.codexauv.com",
  credential: "",
  login_account: "",
  user_id: "",
  is_enabled: true,
};

const ACTION_LABELS: Record<string, string> = {
  reuse: "已存在（复用）",
  create_key: "将创建 Key + 账号",
  create_account: "将创建账号",
  rebuild: "重建 Key",
};

interface CredHelp {
  title: string;
  steps: string[];
  script: string;
  note?: string;
}

const CRED_HELP: Record<string, CredHelp> = {
  codexauv: {
    title: "获取 CodexAuv 的 auth_token",
    steps: [
      "登录 https://www.codexauv.com，确认右上角是目标账号（如 82907183）",
      "按 ⌘+⌥+I（Mac）或 F12（Windows）打开开发者工具；也可右键页面 → 检查",
      "在面板顶部切到 Console / 控制台",
      "首次粘贴会被 Chrome 拦截：在输入框里手动输入 allow pasting 并回车（只需一次）",
      "粘贴下面的命令并回车；返回 undefined 即为已复制到剪贴板",
      "粘贴到记事本确认以 eyJ 开头，再粘到上方「凭证」输入框",
    ],
    script: "copy(localStorage.getItem('auth_token'))",
    note: "token 有效期约 30 天；同步提示 401 时重新获取一次。若 copy 不可用，执行 console.log(localStorage.getItem('auth_token')) 后手动复制输出。",
  },
  new_api: {
    title: "获取 new-api 站点（PackyCode 等）的 Cookie 与 user_id",
    steps: [
      "登录站点（如 https://www.packyapi.ai）",
      "按 ⌘+⌥+I（Mac）或 F12（Windows）打开开发者工具",
      "切到 Application / 应用 → 左侧 Storage → Cookies → 选择站点域名",
      "找到 cf_clearance 和 session 两行，分别复制它们的 Value",
      "把两段拼成：cf_clearance=<值>; session=<值>，粘到上方「凭证」输入框",
      "在 Console 执行 JSON.parse(localStorage.getItem('user')).id 获取 user_id（如 45529），填入「用户 ID」",
    ],
    script: "JSON.parse(localStorage.getItem('user')).id",
    note: "Cookie 为 HttpOnly，document.cookie 读不到，只能按上面从 DevTools 复制。会话过期后需重新复制。",
  },
};
CRED_HELP.packycode = CRED_HELP.new_api;

function CredHelpPanel({ help }: { help: CredHelp }) {
  const [copied, setCopied] = useState(false);
  const copyScript = async () => {
    try {
      await navigator.clipboard.writeText(help.script);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      setCopied(false);
    }
  };
  return (
    <div className="rounded-lg bg-gray-50 p-3 text-[11px] leading-5 text-gray-600">
      <div className="text-xs font-semibold text-gray-800">{help.title}</div>
      <ol className="mt-1 list-decimal space-y-0.5 pl-4">
        {help.steps.map((step) => <li key={step}>{step}</li>)}
      </ol>
      <div className="mt-2 flex items-center gap-2">
        <pre className="flex-1 overflow-auto rounded bg-white p-2 font-mono text-[10px]">{help.script}</pre>
        <button type="button" onClick={copyScript} className="rounded border bg-white px-2 py-1 text-[10px] text-gray-700">
          {copied ? "已复制" : "复制命令"}
        </button>
      </div>
      {help.note && <div className="mt-1.5 text-[10px] text-amber-700">{help.note}</div>}
    </div>
  );
}

export default function UpstreamSyncPage() {
  const [sources, setSources] = useState<SyncSource[]>([]);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const [showForm, setShowForm] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [form, setForm] = useState({ ...emptyForm });
  const [activeSource, setActiveSource] = useState<SyncSource | null>(null);
  const [rebuild, setRebuild] = useState(false);
  const [bindUnbound, setBindUnbound] = useState(false);
  const [preview, setPreview] = useState<PreviewData | null>(null);
  const [result, setResult] = useState<RunResult | null>(null);
  const [runs, setRuns] = useState<Record<string, unknown>[]>([]);

  const load = useCallback(async () => {
    const items = await adminApi<SyncSource[]>("/upstream-sync/sources").catch(() => []);
    setSources(Array.isArray(items) ? items : []);
    const runItems = await adminApi<Record<string, unknown>[]>("/upstream-sync/runs?limit=10").catch(() => []);
    setRuns(Array.isArray(runItems) ? runItems : []);
  }, []);
  useEffect(() => { void load(); }, [load]);

  const openCreate = () => {
    setEditingId(null);
    setForm({ ...emptyForm });
    setShowForm(true);
    setMessage("");
  };
  const openEdit = (src: SyncSource) => {
    setEditingId(src.id);
    setForm({
      provider: src.provider,
      adapter: src.adapter,
      name: src.name,
      site_base_url: src.site_base_url,
      credential: "",
      login_account: src.login_account,
      user_id: String((src.config || {}).user_id || ""),
      is_enabled: src.is_enabled,
    });
    setShowForm(true);
    setMessage("");
  };

  const save = async () => {
    try {
      setBusy(true);
      setMessage("");
      const payload = {
        provider: form.provider,
        adapter: form.adapter,
        name: form.name,
        site_base_url: form.site_base_url,
        credential: form.credential,
        login_account: form.login_account,
        config: form.user_id ? { user_id: form.user_id } : {},
        is_enabled: form.is_enabled,
      };
      if (editingId) await adminApi(`/upstream-sync/sources/${editingId}`, { method: "PATCH", body: JSON.stringify(payload) });
      else await adminApi("/upstream-sync/sources", { method: "POST", body: JSON.stringify(payload) });
      setShowForm(false);
      await load();
    } catch (e) {
      setMessage(e instanceof Error ? e.message : "保存失败");
    } finally {
      setBusy(false);
    }
  };

  const remove = async (src: SyncSource) => {
    if (!confirm(`删除同步源「${src.name}」？`)) return;
    await adminApi(`/upstream-sync/sources/${src.id}`, { method: "DELETE" }).catch(() => null);
    await load();
  };

  const doPreview = async (src: SyncSource) => {
    try {
      setBusy(true); setMessage(""); setResult(null);
      setActiveSource(src);
      const data = await adminApi<PreviewData>(`/upstream-sync/sources/${src.id}/preview`, { method: "POST", body: JSON.stringify({ rebuild }) });
      setPreview(data);
    } catch (e) {
      setMessage(e instanceof Error ? e.message : "预览失败");
    } finally { setBusy(false); }
  };

  const doRun = async () => {
    if (!activeSource) return;
    const summary = `将创建 Key ${preview?.keys_to_create ?? "?"} 个、账号 ${preview?.accounts_to_create ?? "?"} 个${rebuild ? "（重建全部）" : ""}${bindUnbound ? "，并绑定未绑定线路" : ""}`;
    if (!confirm(`确认执行同步？\n${summary}`)) return;
    try {
      setBusy(true); setMessage("");
      const data = await adminApi<RunResult>(`/upstream-sync/sources/${activeSource.id}/run`, {
        method: "POST", body: JSON.stringify({ rebuild, bind_unbound: bindUnbound }),
      });
      setResult(data);
      await load();
    } catch (e) {
      setMessage(e instanceof Error ? e.message : "执行失败");
    } finally { setBusy(false); }
  };

  const help = CRED_HELP[form.adapter] || CRED_HELP.new_api || CRED_HELP.codexauv;

  return (
    <div className="space-y-4 p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold text-gray-900">上游同步</h1>
          <p className="mt-1 text-xs text-gray-500">从上游站点拉取分组，自动创建缺失的 Key 与上游账号，回填倍率并校验连通性。</p>
        </div>
        <button type="button" onClick={openCreate} className="rounded-lg bg-gray-900 px-3 py-2 text-sm text-white">新增同步源</button>
      </div>

      {message && <div className="rounded-lg bg-amber-50 px-3 py-2 text-xs text-amber-700">{message}</div>}

      <div className="overflow-hidden rounded-2xl border bg-white">
        <table className="w-full text-sm">
          <thead className="bg-gray-50 text-gray-500">
            <tr>
              <th className="px-4 py-3 text-left">同步源</th>
              <th className="px-4 py-3 text-left">上游 / 适配器</th>
              <th className="px-4 py-3 text-left">登录账号</th>
              <th className="px-4 py-3 text-left">凭证</th>
              <th className="px-4 py-3 text-left">最近同步</th>
              <th className="px-4 py-3 text-left">操作</th>
            </tr>
          </thead>
          <tbody className="divide-y">
            {sources.map((src) => (
              <tr key={src.id}>
                <td className="px-4 py-3">
                  <div>{src.name}</div>
                  <div className="text-[11px] text-gray-400">{src.site_base_url}</div>
                </td>
                <td className="px-4 py-3 text-xs">{src.provider} / {src.adapter}</td>
                <td className="px-4 py-3 text-xs">{src.login_account || "—"}</td>
                <td className="px-4 py-3 font-mono text-xs text-gray-500">{src.credential_hint || "—"}</td>
                <td className="px-4 py-3 text-xs">
                  {src.last_sync_at ? new Date(src.last_sync_at).toLocaleString() : "未同步"}
                  {src.last_status && <span className={src.last_status === "success" ? "ml-1 text-green-600" : "ml-1 text-amber-600"}>({src.last_status})</span>}
                  {src.last_error && <div className="mt-0.5 max-w-[200px] truncate text-[10px] text-red-400" title={src.last_error}>{src.last_error}</div>}
                </td>
                <td className="px-4 py-3">
                  <div className="flex items-center gap-3 whitespace-nowrap text-xs">
                    <button type="button" disabled={busy} onClick={() => doPreview(src)} className="text-blue-600 hover:underline">预览</button>
                    <button type="button" disabled={busy} onClick={() => { setActiveSource(src); setPreview(null); setResult(null); }} className="text-gray-700 hover:underline">设置</button>
                    <button type="button" onClick={() => openEdit(src)} className="text-gray-700 hover:underline">编辑</button>
                    <button type="button" onClick={() => remove(src)} className="text-red-500 hover:underline">删除</button>
                  </div>
                </td>
              </tr>
            ))}
            {sources.length === 0 && (
              <tr><td colSpan={6} className="p-8 text-center text-gray-400">还没有同步源。点右上角「新增同步源」，把上游站点和凭证填进来。</td></tr>
            )}
          </tbody>
        </table>
      </div>

      {activeSource && (
        <div className="rounded-2xl border bg-white p-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div>
              <div className="text-sm font-semibold text-gray-900">{activeSource.name} · 同步预览</div>
              <div className="mt-0.5 text-[11px] text-gray-400">
                {preview ? `分组 ${preview.groups} 个 · 已有 Key ${preview.keys_existing} · 待创建 Key ${preview.keys_to_create} · 已有账号 ${preview.accounts_existing} · 待建账号 ${preview.accounts_to_create}` : "点「预览」查看差异"}
              </div>
            </div>
            <div className="flex flex-wrap items-center gap-3 text-xs text-gray-600">
              <label className="flex items-center gap-1"><input type="checkbox" checked={rebuild} onChange={(e) => setRebuild(e.target.checked)} /> 重建全部（重新创建 Key）</label>
              <label className="flex items-center gap-1"><input type="checkbox" checked={bindUnbound} onChange={(e) => setBindUnbound(e.target.checked)} /> 同步后绑定未绑定线路</label>
              <button type="button" disabled={busy} onClick={() => doPreview(activeSource)} className="rounded-lg border px-3 py-1.5">预览</button>
              <button type="button" disabled={busy || !preview} onClick={doRun} className="rounded-lg bg-gray-900 px-3 py-1.5 text-white disabled:opacity-50">执行同步</button>
              <button type="button" onClick={() => { setActiveSource(null); setPreview(null); setResult(null); }} className="text-gray-500">关闭</button>
            </div>
          </div>

          {result && (
            <div className="mt-3 rounded-xl bg-green-50 px-3 py-2 text-xs text-green-700">
              执行完成：新建 Key {result.keys_created} · 复用 {result.keys_reused} · 新建账号 {result.accounts_created} · 更新 {result.accounts_updated}
              {result.routes_bound > 0 && ` · 绑定线路 ${result.routes_bound}`} · 校验通过 {result.verify_passed}/{result.verify_passed + result.verify_failed}
            </div>
          )}

          {preview && (
            <div className="mt-3 max-h-[50vh] overflow-auto rounded-xl border">
              <table className="w-full text-sm">
                <thead className="sticky top-0 bg-gray-50 text-gray-500">
                  <tr>
                    <th className="px-3 py-2 text-left">分组</th>
                    <th className="px-3 py-2 text-left">倍率</th>
                    <th className="px-3 py-2 text-left">动作</th>
                    <th className="px-3 py-2 text-left">现有账号</th>
                  </tr>
                </thead>
                <tbody className="divide-y">
                  {preview.items.map((item) => (
                    <tr key={item.group}>
                      <td className="px-3 py-2 font-mono text-xs">{item.group}</td>
                      <td className="px-3 py-2 text-xs">{item.rate > 0 ? `${item.rate}x` : "—"}</td>
                      <td className="px-3 py-2 text-xs">{ACTION_LABELS[item.action] || item.action}</td>
                      <td className="px-3 py-2 text-xs text-gray-500">{item.account_name || "—"}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}

      <div className="overflow-hidden rounded-2xl border bg-white">
        <div className="border-b px-4 py-3 text-sm font-semibold text-gray-900">最近同步记录</div>
        <table className="w-full text-sm">
          <thead className="bg-gray-50 text-gray-500">
            <tr>
              <th className="px-4 py-2 text-left">时间</th>
              <th className="px-4 py-2 text-left">状态</th>
              <th className="px-4 py-2 text-left">分组</th>
              <th className="px-4 py-2 text-left">新建 Key</th>
              <th className="px-4 py-2 text-left">新建/更新账号</th>
              <th className="px-4 py-2 text-left">校验</th>
            </tr>
          </thead>
          <tbody className="divide-y">
            {runs.map((run) => (
              <tr key={String(run.id)}>
                <td className="px-4 py-2 text-xs text-gray-500">{run.started_at ? new Date(String(run.started_at)).toLocaleString() : "—"}</td>
                <td className="px-4 py-2 text-xs">{String(run.status)}</td>
                <td className="px-4 py-2 text-xs">{String(run.groups_total)}</td>
                <td className="px-4 py-2 text-xs">{String(run.keys_created)}</td>
                <td className="px-4 py-2 text-xs">{String(run.accounts_created)} / {String(run.accounts_updated)}</td>
                <td className="px-4 py-2 text-xs">{String(run.verify_passed)} / {Number(run.verify_passed) + Number(run.verify_failed)}</td>
              </tr>
            ))}
            {runs.length === 0 && (<tr><td colSpan={6} className="p-6 text-center text-gray-400">暂无记录</td></tr>)}
          </tbody>
        </table>
      </div>

      {showForm && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/30 p-4">
          <div className="w-full max-w-2xl rounded-2xl bg-white p-5 shadow-xl">
            <h2 className="text-base font-semibold text-gray-900">{editingId ? "编辑同步源" : "新增同步源"}</h2>
            <div className="mt-4 grid grid-cols-2 gap-3">
              <label className="text-xs text-gray-600">上游
                <select value={form.provider} onChange={(e) => {
                  const provider = e.target.value;
                  const adapter = provider === "codexauv" ? "codexauv" : "new_api";
                  const site = provider === "codexauv" ? "https://www.codexauv.com" : provider === "packycode" ? "https://www.packyapi.ai" : form.site_base_url;
                  setForm({ ...form, provider, adapter, site_base_url: site });
                }} className="mt-1 w-full rounded-lg border p-2 text-sm">
                  {PROVIDERS.map((p) => <option key={p} value={p}>{p}</option>)}
                </select>
              </label>
              <label className="text-xs text-gray-600">适配器
                <select value={form.adapter} onChange={(e) => setForm({ ...form, adapter: e.target.value })} className="mt-1 w-full rounded-lg border p-2 text-sm">
                  <option value="codexauv">codexauv</option>
                  <option value="new_api">new_api（PackyCode 等）</option>
                </select>
              </label>
              <label className="text-xs text-gray-600">名称
                <input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} className="mt-1 w-full rounded-lg border p-2 text-sm" placeholder="如 CodexAuv 主账号" />
              </label>
              <label className="text-xs text-gray-600">登录账号（写入账号名）
                <input value={form.login_account} onChange={(e) => setForm({ ...form, login_account: e.target.value })} className="mt-1 w-full rounded-lg border p-2 text-sm" placeholder="82907183 / oidc_45529" />
              </label>
              <label className="col-span-2 text-xs text-gray-600">站点地址
                <input value={form.site_base_url} onChange={(e) => setForm({ ...form, site_base_url: e.target.value })} className="mt-1 w-full rounded-lg border p-2 font-mono text-sm" />
              </label>
              {form.adapter === "new_api" && (
                <label className="col-span-2 text-xs text-gray-600">用户 ID（New-Api-User 头）
                  <input value={form.user_id} onChange={(e) => setForm({ ...form, user_id: e.target.value })} className="mt-1 w-full rounded-lg border p-2 font-mono text-sm" placeholder="45529" />
                </label>
              )}
              <label className="col-span-2 text-xs text-gray-600">凭证{editingId && <span className="ml-1 text-gray-400">（留空不修改）</span>}
                <textarea value={form.credential} onChange={(e) => setForm({ ...form, credential: e.target.value })} rows={4} className="mt-1 w-full rounded-lg border p-2 font-mono text-xs" placeholder={form.adapter === "codexauv" ? "JWT（localStorage.auth_token）" : "cf_clearance=...; session=..."} />
              </label>
              <div className="col-span-2">
                <CredHelpPanel help={help} />
              </div>
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
    </div>
  );
}

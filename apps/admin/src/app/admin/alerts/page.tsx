"use client";

import { useCallback, useEffect, useState } from "react";
import { adminApi } from "@/lib/api";

interface AlertChannel {
  id: number;
  name: string;
  type: string;
  webhook_url: string;
  secret: string;
  is_enabled: boolean;
  notify_events: string;
}

interface UpstreamEvent {
  id: number;
  event_type: string;
  provider: string;
  account_id: number;
  model_code: string;
  reason: string;
  status_code: number;
  notified: boolean;
  created_at: string;
}

const EVENT_LABELS: Record<string, string> = {
  route_failover: "线路切换",
  account_down: "账号不可用",
  all_routes_down: "全部线路失败",
  manual_migrate: "手动批量迁移",
};
const EVENT_OPTIONS = [
  { value: "route_failover", label: "线路切换" },
  { value: "account_down", label: "账号不可用" },
  { value: "all_routes_down", label: "全部线路失败" },
  { value: "manual_migrate", label: "手动批量迁移" },
];

const emptyForm = {
  name: "",
  type: "dingtalk",
  webhook_url: "",
  secret: "",
  is_enabled: true,
  notify_events: "route_failover,account_down,all_routes_down,manual_migrate",
};

export default function AlertsPage() {
  const [channels, setChannels] = useState<AlertChannel[]>([]);
  const [events, setEvents] = useState<UpstreamEvent[]>([]);
  const [form, setForm] = useState({ ...emptyForm });
  const [editingId, setEditingId] = useState<number | null>(null);
  const [showForm, setShowForm] = useState(false);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");

  const load = useCallback(async () => {
    const [channelItems, eventItems] = await Promise.all([
      adminApi<AlertChannel[]>("/alert-channels").catch(() => []),
      adminApi<UpstreamEvent[]>("/upstream-events?limit=50").catch(() => []),
    ]);
    setChannels(Array.isArray(channelItems) ? channelItems : []);
    setEvents(Array.isArray(eventItems) ? eventItems : []);
  }, []);
  useEffect(() => { void load(); }, [load]);

  const openCreate = () => { setEditingId(null); setForm({ ...emptyForm }); setShowForm(true); setMessage(""); };
  const openEdit = (c: AlertChannel) => {
    setEditingId(c.id);
    setForm({ name: c.name, type: c.type, webhook_url: c.webhook_url, secret: "", is_enabled: c.is_enabled, notify_events: c.notify_events });
    setShowForm(true);
    setMessage("");
  };

  const save = async () => {
    try {
      setBusy(true);
      setMessage("");
      if (editingId) {
        await adminApi(`/alert-channels/${editingId}`, { method: "PATCH", body: JSON.stringify(form) });
      } else {
        await adminApi("/alert-channels", { method: "POST", body: JSON.stringify(form) });
      }
      setShowForm(false);
      await load();
    } catch (e) {
      setMessage(e instanceof Error ? e.message : "保存失败");
    } finally {
      setBusy(false);
    }
  };

  const remove = async (c: AlertChannel) => {
    if (!confirm(`删除告警通道「${c.name}」？`)) return;
    await adminApi(`/alert-channels/${c.id}`, { method: "DELETE" }).catch(() => null);
    await load();
  };

  const test = async (c: AlertChannel) => {
    try {
      setBusy(true);
      await adminApi(`/alert-channels/${c.id}/test`, { method: "POST", body: "{}" });
      setMessage(`测试消息已发送到「${c.name}」`);
    } catch (e) {
      setMessage(e instanceof Error ? `发送失败：${e.message}` : "发送失败");
    } finally {
      setBusy(false);
    }
  };

  const toggleEvent = (value: string) => {
    const current = form.notify_events ? form.notify_events.split(",").filter(Boolean) : [];
    const next = current.includes(value) ? current.filter((v) => v !== value) : [...current, value];
    setForm({ ...form, notify_events: next.join(",") });
  };

  return (
    <div className="space-y-4 p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold text-gray-900">上游告警</h1>
          <p className="mt-1 text-xs text-gray-500">线路切换、账号不可用、全部线路失败时，推送到钉钉/飞书机器人。</p>
        </div>
        <button type="button" onClick={openCreate} className="rounded-lg bg-gray-900 px-3 py-2 text-sm text-white">新增通道</button>
      </div>

      {message && <div className="rounded-lg bg-blue-50 px-3 py-2 text-xs text-blue-700">{message}</div>}

      <div className="overflow-hidden rounded-2xl border bg-white">
        <table className="w-full text-sm">
          <thead className="bg-gray-50 text-gray-500">
            <tr>
              <th className="px-4 py-3 text-left">名称</th>
              <th className="px-4 py-3 text-left">类型</th>
              <th className="px-4 py-3 text-left">Webhook</th>
              <th className="px-4 py-3 text-left">订阅事件</th>
              <th className="px-4 py-3 text-left">状态</th>
              <th className="px-4 py-3 text-left">操作</th>
            </tr>
          </thead>
          <tbody className="divide-y">
            {channels.map((c) => (
              <tr key={c.id}>
                <td className="px-4 py-3">{c.name}</td>
                <td className="px-4 py-3 text-xs">{c.type === "dingtalk" ? "钉钉机器人" : "飞书机器人"}</td>
                <td className="px-4 py-3 max-w-[260px] truncate text-xs text-gray-500" title={c.webhook_url}>{c.webhook_url}</td>
                <td className="px-4 py-3 text-xs">{c.notify_events.split(",").filter(Boolean).map((e) => EVENT_LABELS[e] || e).join("、")}</td>
                <td className="px-4 py-3"><span className={c.is_enabled ? "text-green-600" : "text-gray-400"}>{c.is_enabled ? "启用" : "禁用"}</span></td>
                <td className="px-4 py-3">
                  <div className="flex items-center gap-3 whitespace-nowrap">
                    <button type="button" disabled={busy} onClick={() => test(c)} className="text-blue-600 hover:underline">测试</button>
                    <button type="button" onClick={() => openEdit(c)} className="text-gray-700 hover:underline">编辑</button>
                    <button type="button" onClick={() => remove(c)} className="text-red-500 hover:underline">删除</button>
                  </div>
                </td>
              </tr>
            ))}
            {channels.length === 0 && (
              <tr><td colSpan={6} className="p-8 text-center text-gray-400">还没有告警通道。点右上角「新增通道」配置钉钉/飞书机器人。</td></tr>
            )}
          </tbody>
        </table>
      </div>

      <div className="overflow-hidden rounded-2xl border bg-white">
        <div className="border-b px-4 py-3">
          <div className="text-sm font-semibold text-gray-900">最近事件</div>
          <div className="mt-0.5 text-[11px] text-gray-400">线路切换 / 账号不可用 / 全部线路失败（最近 50 条）</div>
        </div>
        <table className="w-full text-sm">
          <thead className="bg-gray-50 text-gray-500">
            <tr>
              <th className="px-4 py-2 text-left">时间</th>
              <th className="px-4 py-2 text-left">事件</th>
              <th className="px-4 py-2 text-left">模型</th>
              <th className="px-4 py-2 text-left">上游</th>
              <th className="px-4 py-2 text-left">原因</th>
              <th className="px-4 py-2 text-left">已通知</th>
            </tr>
          </thead>
          <tbody className="divide-y">
            {events.map((e) => (
              <tr key={e.id}>
                <td className="px-4 py-2 text-xs text-gray-500">{new Date(e.created_at).toLocaleString()}</td>
                <td className="px-4 py-2 text-xs">{EVENT_LABELS[e.event_type] || e.event_type}</td>
                <td className="px-4 py-2 font-mono text-xs">{e.model_code || "—"}</td>
                <td className="px-4 py-2 text-xs">{e.provider || "—"}</td>
                <td className="px-4 py-2 max-w-[380px] truncate text-xs text-gray-500" title={e.reason}>{e.reason || "—"}</td>
                <td className="px-4 py-2 text-xs">{e.notified ? "是" : "否"}</td>
              </tr>
            ))}
            {events.length === 0 && (
              <tr><td colSpan={6} className="p-6 text-center text-gray-400">暂无事件</td></tr>
            )}
          </tbody>
        </table>
      </div>

      {showForm && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/30 p-4">
          <div className="w-full max-w-lg rounded-2xl bg-white p-5 shadow-xl">
            <h2 className="text-base font-semibold text-gray-900">{editingId ? "编辑告警通道" : "新增告警通道"}</h2>
            <div className="mt-4 grid grid-cols-2 gap-3">
              <label className="text-xs text-gray-600">名称
                <input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} className="mt-1 w-full rounded-lg border p-2 text-sm" placeholder="如 运维群" />
              </label>
              <label className="text-xs text-gray-600">类型
                <select value={form.type} onChange={(e) => setForm({ ...form, type: e.target.value })} className="mt-1 w-full rounded-lg border bg-white p-2 text-sm">
                  <option value="dingtalk">钉钉机器人</option>
                  <option value="feishu">飞书机器人</option>
                </select>
              </label>
              <label className="col-span-2 text-xs text-gray-600">Webhook 地址
                <input value={form.webhook_url} onChange={(e) => setForm({ ...form, webhook_url: e.target.value })} className="mt-1 w-full rounded-lg border p-2 font-mono text-sm" placeholder="https://oapi.dingtalk.com/robot/send?access_token=..." />
              </label>
              <label className="col-span-2 text-xs text-gray-600">加签 Secret{editingId && <span className="ml-1 text-gray-400">（留空表示不修改）</span>}
                <input value={form.secret} onChange={(e) => setForm({ ...form, secret: e.target.value })} className="mt-1 w-full rounded-lg border p-2 font-mono text-sm" placeholder="SEC..." />
              </label>
              <div className="col-span-2 text-xs text-gray-600">订阅事件
                <div className="mt-1 flex flex-wrap gap-3">
                  {EVENT_OPTIONS.map((opt) => (
                    <label key={opt.value} className="flex items-center gap-1 text-xs text-gray-700">
                      <input type="checkbox" checked={form.notify_events.split(",").includes(opt.value)} onChange={() => toggleEvent(opt.value)} />
                      {opt.label}
                    </label>
                  ))}
                </div>
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

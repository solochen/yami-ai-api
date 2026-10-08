"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { Calculator, ChevronDown, ChevronRight, Download, History, Loader2, Plus, Send, Wrench } from "lucide-react";
import { API_URL, api, legacyAuthHeaders } from "@/lib/api";
import { readEventStream } from "@/lib/eventStream";
import { useI18n } from "@/i18n/I18nProvider";
import { AgentIcon } from "./AgentIcon";
import { AgentRichText } from "./AgentRichText";

type WorkflowLike = {
  code?: string;
  name?: string;
  description?: string;
  icon?: string;
  display_config?: { toolbox?: { subtitle?: string } } & Record<string, unknown>;
  runtime_config?: Record<string, unknown>;
};

type ToolTrace = {
  round: number;
  name: string;
  arguments?: unknown;
  result?: unknown;
  ok: boolean;
};

type ReportFile = { title?: string; filename?: string; content?: string };

type Message = {
  role: "user" | "assistant";
  content: string;
  tools?: ToolTrace[];
  report?: ReportFile;
};

type HistoryItem = { public_id: string; title?: string | null; updated_at: string };

const HOT_PROMPTS = [
  "客厅 5.2米×3.8米铺 800x800 瓷砖，25 元左右一片，帮我算量和总价",
  "主卧 4.2×3.5 米铺强化复合地板，墙面刷乳胶漆，出个预算报告",
  "奶油风适合小户型吗？和原木风比怎么选",
  "89 平三室两厅，全屋预算应该怎么分配？",
];

const TOOL_LABELS: Record<string, string> = {
  query_material_price: "材料询价",
  calc_material_quantity: "工程算量",
  search_knowledge: "知识库检索",
  generate_report: "生成报告",
};

function prettyJSON(value: unknown): string {
  if (value === undefined || value === null) return "";
  if (typeof value === "string") {
    try {
      return JSON.stringify(JSON.parse(value), null, 2);
    } catch {
      return value;
    }
  }
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

function ToolTraceCard({ trace }: { trace: ToolTrace }) {
  const [open, setOpen] = useState(false);
  const failed = trace.ok && trace.result && typeof trace.result === "object" && "error" in (trace.result as Record<string, unknown>);
  const resultText = prettyJSON(trace.result);
  return (
    <div className="overflow-hidden rounded-xl border border-gray-200 bg-gray-50/70 text-xs dark:border-white/10 dark:bg-white/5">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="flex w-full items-center gap-2 px-3 py-2 text-left"
      >
        {open ? <ChevronDown size={13} className="shrink-0 text-gray-400" /> : <ChevronRight size={13} className="shrink-0 text-gray-400" />}
        <Wrench size={13} className={failed ? "shrink-0 text-amber-500" : "shrink-0 text-emerald-500"} />
        <span className="font-medium text-gray-700 dark:text-gray-200">{TOOL_LABELS[trace.name] || trace.name}</span>
        <span className="min-w-0 flex-1 truncate text-gray-400">第 {trace.round} 轮 · {prettyJSON(trace.arguments).replace(/\s+/g, " ").slice(0, 60)}</span>
        <span className={failed ? "shrink-0 text-amber-500" : "shrink-0 text-emerald-500"}>{failed ? "已反馈重试" : "成功"}</span>
      </button>
      {open ? (
        <div className="grid gap-2 border-t border-gray-200 px-3 py-2 dark:border-white/10 md:grid-cols-2">
          <div>
            <div className="mb-1 text-[11px] font-medium text-gray-400">参数</div>
            <pre className="max-h-48 overflow-auto whitespace-pre-wrap break-all rounded-lg bg-white p-2 text-[11px] text-gray-600 dark:bg-black/20 dark:text-gray-300">{prettyJSON(trace.arguments) || "-"}</pre>
          </div>
          <div>
            <div className="mb-1 text-[11px] font-medium text-gray-400">结果</div>
            <pre className="max-h-48 overflow-auto whitespace-pre-wrap break-all rounded-lg bg-white p-2 text-[11px] text-gray-600 dark:bg-black/20 dark:text-gray-300">{resultText || "-"}</pre>
          </div>
        </div>
      ) : null}
    </div>
  );
}

function downloadReport(report: ReportFile) {
  if (!report.content) return;
  const blob = new Blob([report.content], { type: "text/markdown;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = report.filename || "装修预算报告.md";
  anchor.click();
  URL.revokeObjectURL(url);
}

export function RenovationBudgetWorkspace({ workflow }: { workflow: WorkflowLike }) {
  const { ts } = useI18n();
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState("");
  const [busy, setBusy] = useState(false);
  const [stage, setStage] = useState("");
  const [conversationId, setConversationId] = useState<string>("");
  const [error, setError] = useState("");
  const [history, setHistory] = useState<HistoryItem[]>([]);
  const [showHistory, setShowHistory] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const stickRef = useRef(true);
  const abortRef = useRef<AbortController | null>(null);

  const loadHistory = useCallback(async () => {
    try {
      const data = await api<HistoryItem[]>("/api/chat/conversations?scope=renovation_budget");
      setHistory(Array.isArray(data) ? data : []);
    } catch {
      setHistory([]);
    }
  }, []);

  useEffect(() => {
    void loadHistory();
  }, [loadHistory]);

  useEffect(() => {
    if (stickRef.current) messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages, stage]);

  const startNewChat = useCallback(() => {
    abortRef.current?.abort();
    setMessages([]);
    setConversationId("");
    setError("");
    setStage("");
    setInput("");
  }, []);

  const openConversation = useCallback(async (id: string) => {
    abortRef.current?.abort();
    try {
      const data = await api<{ messages?: Array<{ role: string; content: string }> }>(`/api/chat/conversations/${id}?limit=200`);
      const restored: Message[] = (data.messages || [])
        .filter((m) => m.role === "user" || m.role === "assistant")
        .map((m) => ({ role: m.role as "user" | "assistant", content: m.content }));
      setMessages(restored);
      setConversationId(id);
      setError("");
      setShowHistory(false);
      stickRef.current = true;
    } catch (e) {
      setError(e instanceof Error ? e.message : ts("会话加载失败"));
    }
  }, [ts]);

  const send = useCallback(async (text?: string) => {
    const message = (text ?? input).trim();
    if (!message || busy) return;
    setInput("");
    setError("");
    setBusy(true);
    setStage(ts("正在思考…"));
    stickRef.current = true;
    const assistant: Message = { role: "assistant", content: "", tools: [] };
    setMessages((prev) => [...prev, { role: "user", content: message }, assistant]);

    const controller = new AbortController();
    abortRef.current = controller;

    const patchAssistant = (patch: Partial<Message> | ((m: Message) => Message)) => {
      setMessages((prev) => {
        const next = [...prev];
        const index = next.length - 1;
        if (index >= 0 && next[index].role === "assistant") {
          next[index] = typeof patch === "function" ? patch(next[index]) : { ...next[index], ...patch };
        }
        return next;
      });
    };

    // 打字机缓冲：每 40ms 批量吐出 delta
    let pendingText = "";
    let flushTimer: ReturnType<typeof setTimeout> | undefined;
    const flush = () => {
      if (flushTimer !== undefined) clearTimeout(flushTimer);
      flushTimer = undefined;
      if (pendingText) {
        const chunk = pendingText;
        pendingText = "";
        patchAssistant((m) => ({ ...m, content: m.content + chunk }));
      }
    };

    try {
      const locale = typeof window === "undefined" ? "zh-CN" : localStorage.getItem("site_locale") || "zh-CN";
      const response = await fetch(`${API_URL}/api/renovation-budget/chat`, {
        method: "POST",
        credentials: "include",
        signal: controller.signal,
        headers: {
          "Content-Type": "application/json",
          "Accept-Language": locale,
          "X-Locale": locale,
          ...legacyAuthHeaders(),
        },
        body: JSON.stringify({ conversation_id: conversationId || undefined, message }),
      });
      const contentType = response.headers.get("content-type") || "";
      if (!response.ok || !contentType.includes("text/event-stream")) {
        const body = await response.json().catch(() => null);
        throw new Error(body?.message || `请求失败（HTTP ${response.status}）`);
      }
      if (!response.body) throw new Error(ts("当前浏览器不支持流式输出"));

      for await (const { event, data: raw } of readEventStream(response.body)) {
        if (raw === "[DONE]") continue;
        let data: Record<string, unknown>;
        try {
          data = JSON.parse(raw) as Record<string, unknown>;
        } catch {
          continue;
        }
        if (event === "meta") {
          if (typeof data.conversation_id === "string" && data.conversation_id) setConversationId(data.conversation_id);
        } else if (event === "status") {
          setStage(String(data.message || ""));
        } else if (event === "tool_result") {
          flush();
          const trace: ToolTrace = {
            round: Number(data.round || 0),
            name: String(data.name || ""),
            arguments: data.arguments,
            result: data.result,
            ok: data.ok !== false,
          };
          patchAssistant((m) => ({ ...m, tools: [...(m.tools || []), trace] }));
        } else if (event === "delta") {
          pendingText += String(data.text || "");
          if (flushTimer === undefined) flushTimer = setTimeout(flush, 40);
        } else if (event === "error") {
          throw new Error(String(data.message || ts("生成失败")));
        } else if (event === "done") {
          flush();
          const report = (data.report as ReportFile | undefined) || undefined;
          patchAssistant((m) => ({
            ...m,
            content: typeof data.content === "string" && data.content ? data.content : m.content,
            report,
          }));
          if (report?.content) {
            setStage("");
            patchAssistant((m) => m);
          }
        }
      }
      flush();
      void loadHistory();
    } catch (e) {
      flush();
      if (e instanceof DOMException && e.name === "AbortError") {
        // 用户主动停止
      } else {
        setError(e instanceof Error ? e.message : ts("生成失败"));
      }
    } finally {
      if (flushTimer !== undefined) clearTimeout(flushTimer);
      setBusy(false);
      setStage("");
      abortRef.current = null;
    }
  }, [busy, conversationId, input, loadHistory, ts]);

  const agentName = workflow.name || ts("装修预算助理");
  const subtitle = (workflow.display_config?.toolbox?.subtitle as string | undefined) || ts("询价、算量、出预算报告一步到位");
  const chatModelCode = (workflow.runtime_config?.chat_model_code as string | undefined) || "";

  return (
    <div className="flex h-full min-h-0 flex-col">
      <header className="flex items-center gap-3 border-b border-gray-200/70 px-4 py-3 dark:border-white/10 md:px-6">
        <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-primary/10">
          <Calculator size={18} className="text-primary" />
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <h2 className="truncate text-sm font-semibold text-gray-900 dark:text-gray-100">{agentName}</h2>
            {chatModelCode ? (
              <span className="hidden shrink-0 rounded-full bg-gray-100 px-2 py-0.5 text-[10px] text-gray-500 dark:bg-white/10 dark:text-gray-400 sm:inline">{chatModelCode}</span>
            ) : null}
          </div>
          <p className="truncate text-xs text-gray-400">{subtitle}</p>
        </div>
        <button
          type="button"
          onClick={() => { setShowHistory((v) => !v); void loadHistory(); }}
          className="inline-flex h-8 items-center gap-1 rounded-lg border border-gray-200 px-2.5 text-xs text-gray-500 transition hover:bg-gray-50 dark:border-white/10 dark:text-gray-400 dark:hover:bg-white/5"
        >
          <History size={13} />{ts("历史")}
        </button>
        <button
          type="button"
          onClick={startNewChat}
          className="inline-flex h-8 items-center gap-1 rounded-lg bg-primary px-2.5 text-xs font-medium text-white transition hover:opacity-90"
        >
          <Plus size={13} />{ts("新会话")}
        </button>
      </header>

      {showHistory ? (
        <div className="max-h-56 overflow-auto border-b border-gray-200/70 bg-gray-50/60 px-4 py-2 dark:border-white/10 dark:bg-white/[0.03]">
          {history.length === 0 ? (
            <div className="py-4 text-center text-xs text-gray-400">{ts("暂无历史会话")}</div>
          ) : (
            <ul className="mx-auto max-w-5xl divide-y divide-gray-200/70 dark:divide-white/5">
              {history.map((item) => (
                <li key={item.public_id}>
                  <button
                    type="button"
                    onClick={() => void openConversation(item.public_id)}
                    className={`flex w-full items-center gap-2 px-2 py-2 text-left text-xs transition hover:bg-white dark:hover:bg-white/5 ${conversationId === item.public_id ? "text-primary" : "text-gray-600 dark:text-gray-300"}`}
                  >
                    <AgentIcon code={workflow.code} value={workflow.icon} className="h-4 w-4 shrink-0" />
                    <span className="min-w-0 flex-1 truncate">{item.title || ts("装修预算助理")}</span>
                    <span className="shrink-0 text-[10px] text-gray-400">{new Date(item.updated_at).toLocaleString()}</span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      ) : null}

      <div
        className="min-h-0 flex-1 overflow-y-auto px-4 py-6 md:px-6"
        onScroll={(e) => {
          const el = e.currentTarget;
          stickRef.current = el.scrollHeight - el.scrollTop - el.clientHeight < 80;
        }}
      >
        <div className="mx-auto max-w-5xl space-y-4">
          {messages.length === 0 ? (
            <div className="soft-card mx-auto max-w-2xl p-6 text-center">
              <div className="mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-2xl bg-primary/10">
                <Calculator size={22} className="text-primary" />
              </div>
              <h3 className="text-base font-semibold text-gray-900 dark:text-gray-100">{agentName}</h3>
              <p className="mt-1 text-xs text-gray-400">
                {ts("描述房间尺寸和材料偏好，我会自动完成询价、算量并生成可下载的预算报告。")}
              </p>
              <div className="mt-4 grid gap-2 text-left sm:grid-cols-2">
                {HOT_PROMPTS.map((prompt) => (
                  <button
                    key={prompt}
                    type="button"
                    onClick={() => void send(prompt)}
                    className="rounded-xl border border-gray-200 px-3 py-2 text-xs text-gray-600 transition hover:border-primary/40 hover:text-primary dark:border-white/10 dark:text-gray-300"
                  >
                    {prompt}
                  </button>
                ))}
              </div>
            </div>
          ) : null}

          {messages.map((message, index) =>
            message.role === "user" ? (
              <div key={index} className="flex justify-end">
                <div className="max-w-[85%] rounded-2xl rounded-br-md bg-primary px-4 py-2.5 text-sm text-white shadow-sm">{message.content}</div>
              </div>
            ) : (
              <div key={index} className="flex justify-start">
                <div className="w-full max-w-[92%] space-y-2">
                  {message.tools && message.tools.length > 0 ? (
                    <div className="space-y-1.5">
                      {message.tools.map((trace, i) => <ToolTraceCard key={i} trace={trace} />)}
                    </div>
                  ) : null}
                  {message.content ? (
                    <div className="soft-card px-4 py-3 text-sm leading-relaxed text-gray-700 dark:text-gray-200">
                      <AgentRichText content={message.content} />
                    </div>
                  ) : null}
                  {message.report ? (
                    <button
                      type="button"
                      onClick={() => downloadReport(message.report as ReportFile)}
                      className="inline-flex items-center gap-2 rounded-xl border border-primary/30 bg-primary/5 px-3 py-2 text-xs font-medium text-primary transition hover:bg-primary/10"
                    >
                      <Download size={13} />
                      {ts("下载预算报告")} · {message.report.filename}
                    </button>
                  ) : null}
                </div>
              </div>
            )
          )}

          {busy ? (
            <div className="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
              <Loader2 size={14} className="animate-spin text-primary" />
              <span role="status" aria-live="polite">{stage || ts("正在处理…")}</span>
            </div>
          ) : null}

          {error ? (
            <div className="rounded-xl border border-red-200 bg-red-50 px-3 py-2 text-xs text-red-500 dark:border-red-500/20 dark:bg-red-500/10">{error}</div>
          ) : null}
          <div ref={messagesEndRef} />
        </div>
      </div>

      <footer className="border-t border-gray-200/70 px-4 py-3 dark:border-white/10 md:px-6">
        <form
          className="mx-auto flex max-w-5xl items-end gap-2"
          onSubmit={(e) => {
            e.preventDefault();
            void send();
          }}
        >
          <textarea
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter" && !e.shiftKey && !e.nativeEvent.isComposing) {
                e.preventDefault();
                void send();
              }
            }}
            rows={1}
            placeholder={ts("输入房间尺寸和材料需求，例如：主卧 4.2×3.5 米铺 98 元/㎡ 强化地板要多少钱")}
            className="soft-input max-h-32 min-h-[44px] flex-1 resize-y px-3 py-2.5 text-sm"
          />
          <button
            type="submit"
            disabled={busy || !input.trim()}
            className="inline-flex h-[44px] w-[44px] shrink-0 items-center justify-center rounded-xl bg-primary text-white transition hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-40"
          >
            {busy ? <Loader2 size={16} className="animate-spin" /> : <Send size={16} />}
          </button>
        </form>
        <p className="mx-auto mt-1.5 max-w-5xl text-center text-[10px] text-gray-400">
          {ts("材料价格为参考库数据，实际以市场报价为准；算量结果已含常规损耗。")}
        </p>
      </footer>
    </div>
  );
}
